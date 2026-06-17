package services

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/smtp"
	"pipe-monitor/internal/config"
	"pipe-monitor/internal/models"
	"pipe-monitor/internal/utils"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"

	redisClient "pipe-monitor/internal/redis"
)

type AlertService struct {
	cfg  *config.Config
	db   *gorm.DB
	rdb  *redisClient.Client
	wsFn func(tenantID uint, msgType string, payload interface{})
}

func NewAlertService(cfg *config.Config, db *gorm.DB, rdb *redisClient.Client) *AlertService {
	return &AlertService{cfg: cfg, db: db, rdb: rdb}
}

func (a *AlertService) SetWSPublisher(fn func(tenantID uint, msgType string, payload interface{})) {
	a.wsFn = fn
}

func (a *AlertService) ProcessRunStatus(pipe *models.Pipeline, run *models.PipelineRun) {
	var rules []models.AlertRule
	a.db.Where("(pipeline_id = ? OR pipeline_id IS NULL) AND enabled = ?", pipe.ID, true).Find(&rules)

	for _, rule := range rules {
		switch rule.RuleType {
		case models.AlertConsecutiveFail:
			a.processConsecutiveFail(pipe, run, &rule)
		case models.AlertDurationP95:
			a.processDurationP95(pipe, run, &rule)
		case models.AlertPipelineDown:
			if run.Status == models.RunFailed || run.Status == models.RunTimeout {
				a.triggerAlert(pipe, run, &rule, fmt.Sprintf("%s 运行失败", pipe.Name), run.ErrorMessage)
			}
		}
	}
}

func (a *AlertService) processConsecutiveFail(pipe *models.Pipeline, run *models.PipelineRun, rule *models.AlertRule) {
	if run.Status != models.RunFailed && run.Status != models.RunTimeout {
		return
	}

	var recent []models.PipelineRun
	a.db.Where("pipeline_id = ?", pipe.ID).Order("created_at DESC").Limit(rule.ConsecutiveFailN).Find(&recent)

	n := rule.ConsecutiveFailN
	if n <= 0 {
		n = 3
	}
	failCount := 0
	for _, r := range recent {
		if r.Status == models.RunFailed || r.Status == models.RunTimeout {
			failCount++
		} else {
			break
		}
	}
	if failCount >= n {
		a.triggerAlert(pipe, run, rule,
			fmt.Sprintf("%s 连续失败%d次", pipe.Name, failCount),
			fmt.Sprintf("最近%d次运行中失败%d次，最后错误: %s", len(recent), failCount, run.ErrorMessage))
	}
}

func (a *AlertService) processDurationP95(pipe *models.Pipeline, run *models.PipelineRun, rule *models.AlertRule) {
	if run.DurationSec <= 0 {
		return
	}
	_, p95 := (&SLAEngine{db: a.db}).GetHistoryStats(pipe.ID, 7)
	if p95 <= 0 {
		return
	}
	multi := rule.DurationP95Multi
	if multi <= 0 {
		multi = 2.0
	}
	threshold := int(float64(p95) * multi)
	if run.DurationSec > threshold {
		a.triggerAlert(pipe, run, rule,
			fmt.Sprintf("%s 运行耗时异常", pipe.Name),
			fmt.Sprintf("本次耗时%d秒，超过历史P95(%d秒)的%.1f倍阈值(%d秒)",
				run.DurationSec, p95, multi, threshold))
	}
}

func (a *AlertService) ProcessSLAImminent(pipe *models.Pipeline, run *models.PipelineRun, rule *models.SLARule, secondsToDeadline int) {
	var alertRules []models.AlertRule
	a.db.Where("pipeline_id = ? AND rule_type = ? AND enabled = ?", pipe.ID, models.AlertSLAImminent, true).Find(&alertRules)
	if len(alertRules) == 0 {
		alertRules = append(alertRules, models.AlertRule{
			ID:               0,
			TenantID:         pipe.TenantID,
			PipelineID:       &pipe.ID,
			RuleType:         models.AlertSLAImminent,
			Severity:         models.AlertWarning,
			Channels:         rule.AlertChannels,
			SuppressWindowMin: 120,
		})
	}
	for _, r := range alertRules {
		ar := r
		a.triggerAlert(pipe, run, &ar,
			fmt.Sprintf("%s SLA即将违约", pipe.Name),
			fmt.Sprintf("距离截止时间还有%d分钟，SLA规则: %s，预计无法按时完成",
				secondsToDeadline/60, rule.Name))
	}
}

func (a *AlertService) ProcessSLABreach(pipe *models.Pipeline, run *models.PipelineRun, rule *models.SLARule, breachSec int) {
	var alertRules []models.AlertRule
	a.db.Where("pipeline_id = ? AND rule_type = ? AND enabled = ?", pipe.ID, models.AlertSLABreached, true).Find(&alertRules)
	if len(alertRules) == 0 {
		ch := rule.AlertChannels
		if ch == "" {
			ch = `["feishu","email"]`
		}
		alertRules = append(alertRules, models.AlertRule{
			ID:               0,
			TenantID:         pipe.TenantID,
			PipelineID:       &pipe.ID,
			RuleType:         models.AlertSLABreached,
			Severity:         models.AlertSeverity(rule.AlertSeverity),
			Channels:         ch,
			SuppressWindowMin: 120,
		})
	}
	for _, r := range alertRules {
		ar := r
		a.triggerAlert(pipe, run, &ar,
			fmt.Sprintf("%s SLA违约", pipe.Name),
			fmt.Sprintf("SLA规则: %s，违约%d秒。本次运行详情: 状态=%s, 耗时=%d秒",
				rule.Name, breachSec, run.Status, run.DurationSec))
	}
}

func (a *AlertService) calcFingerprint(pipe *models.Pipeline, rule *models.AlertRule) string {
	raw := fmt.Sprintf("%d|%d|%s|%s", pipe.ID, rule.ID, rule.RuleType, rule.Severity)
	sum := md5.Sum([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func (a *AlertService) triggerAlert(pipe *models.Pipeline, run *models.PipelineRun, rule *models.AlertRule, title, message string) {
	fp := a.calcFingerprint(pipe, rule)

	now := time.Now()
	var existing models.AlertEvent
	err := a.db.Where("fingerprint = ? AND status IN ?", fp, []string{
		string(models.AlertTriggered),
		string(models.AlertAcknowledged),
		string(models.AlertSuppressed),
	}).First(&existing).Error

	if err == nil {
		existing.NotifyCount++
		existing.UpdatedAt = now
		if existing.Status == models.AlertSuppressed &&
			(existing.SuppressedUntil == nil || existing.SuppressedUntil.Before(now)) {
			existing.Status = models.AlertTriggered
			existing.FirstTriggeredAt = now
			a.db.Save(&existing)
			a.notifyChannels(&existing, rule, title, message)
		} else {
			a.db.Save(&existing)
		}
		return
	}

	tenantID := pipe.TenantID
	if rule.TenantID > 0 {
		tenantID = rule.TenantID
	}
	event := models.AlertEvent{
		TenantID:         tenantID,
		RuleID:           rule.ID,
		PipelineID:       &pipe.ID,
		RunID:            &run.ID,
		Fingerprint:      fp,
		Severity:         rule.Severity,
		Status:           models.AlertTriggered,
		Title:            title,
		Message:          message,
		Detail:           utils.ToJSON(map[string]interface{}{"pipelineCode": pipe.Code, "runId": run.RunID, "rule": rule.Name}),
		TriggeredAt:      now,
		FirstTriggeredAt: now,
		NotifyCount:      1,
	}

	if rule.SuppressWindowMin > 0 {
		sup := now.Add(time.Duration(rule.SuppressWindowMin) * time.Minute)
		event.SuppressedUntil = &sup
	}

	if rule.NotifyOnCall {
		occ := a.getCurrentOnCallUser(pipe.ID, tenantID)
		if occ != nil {
			note := fmt.Sprintf("已路由到值班人ID:%d", occ.UserID)
			event.Message = message + " " + note
		}
	}

	a.db.Create(&event)

	a.notifyChannels(&event, rule, title, message)

	if a.wsFn != nil {
		inSilent := a.isInSilentPeriod(rule, now)
		a.wsFn(tenantID, "new_alert", map[string]interface{}{
			"id":         event.ID,
			"title":      title,
			"severity":   event.Severity,
			"status":     event.Status,
			"pipelineId": pipe.ID,
			"pipelineName": pipe.Name,
			"triggeredAt": now,
			"isSilent":   inSilent,
		})
	}
}

func (a *AlertService) getCurrentOnCallUser(pipelineID, tenantID uint) *models.OnCallAssignment {
	now := time.Now()
	var occ models.OnCallAssignment
	err := a.db.Where("(pipeline_id = ? OR pipeline_id IS NULL) AND tenant_id = ? AND start_date <= ? AND end_date >= ?",
		pipelineID, tenantID, now, now).Order("is_backup ASC, start_date DESC").First(&occ).Error
	if err != nil {
		return nil
	}
	return &occ
}

func (a *AlertService) notifyChannels(event *models.AlertEvent, rule *models.AlertRule, title, message string, skipSilentCheck ...bool) {
	skip := len(skipSilentCheck) > 0 && skipSilentCheck[0]
	if !skip && a.isInSilentPeriod(rule, time.Now()) {
		return
	}
	var channels []string
	json.Unmarshal([]byte(rule.Channels), &channels)

	notified := make([]string, 0)
	for _, ch := range channels {
		chType := models.AlertChannelType(ch)
		recipient := ""
		sent := false
		var notifyErr error
		switch chType {
		case models.AlertWebhookFeishu:
			notifyErr = a.sendFeishu(title, message)
			sent = notifyErr == nil
			recipient = a.cfg.Alert.FeishuWebhook
		case models.AlertWebhookDingTalk:
			notifyErr = a.sendDingTalk(title, message)
			sent = notifyErr == nil
			recipient = a.cfg.Alert.DingTalkWebhook
		case models.AlertWebhookSlack:
			notifyErr = a.sendSlack(title, message)
			sent = notifyErr == nil
			recipient = a.cfg.Alert.SlackWebhook
		case models.AlertWebhookCustom:
			if rule.CustomWebhookURL != "" {
				notifyErr = a.sendCustomWebhook(rule.CustomWebhookURL, title, message)
				sent = notifyErr == nil
				recipient = rule.CustomWebhookURL
			}
		case models.AlertEmail:
			var recipients []string
			json.Unmarshal([]byte(rule.EmailRecipients), &recipients)
			occ := a.getCurrentOnCallUser(*event.PipelineID, event.TenantID)
			if occ != nil {
				var u models.User
				if a.db.First(&u, occ.UserID).Error == nil {
					recipients = append(recipients, u.Email)
				}
			}
			if len(recipients) > 0 {
				notifyErr = a.sendEmail(recipients, title, message)
				sent = notifyErr == nil
				recipient = strings.Join(recipients, ",")
			}
		}
		status := "sent"
		errStr := ""
		if notifyErr != nil {
			status = "failed"
			errStr = notifyErr.Error()
		}
		if sent || notifyErr != nil {
			notified = append(notified, ch)
			notif := models.AlertNotification{
				AlertID:   event.ID,
				Channel:   chType,
				Recipient: recipient,
				Status:    status,
				Error:     errStr,
				SentAt:    time.Now(),
			}
			a.db.Create(&notif)
		}
	}
	if len(notified) > 0 {
		event.ChannelsNotified = utils.ToJSON(notified)
		a.db.Save(event)
	}
}

func (a *AlertService) sendFeishu(title, msg string) error {
	if a.cfg.Alert.FeishuWebhook == "" {
		return nil
	}
	payload := map[string]interface{}{
		"msg_type": "interactive",
		"card": map[string]interface{}{
			"config": map[string]interface{}{"wide_screen_mode": true},
			"header": map[string]interface{}{
				"title":    map[string]string{"tag": "plain_text", "content": title},
				"template": "red",
			},
			"elements": []interface{}{
				map[string]interface{}{
					"tag":  "div",
					"text": map[string]string{"tag": "lark_md", "content": msg},
				},
			},
		},
	}
	return a.httpPost(a.cfg.Alert.FeishuWebhook, payload)
}

func (a *AlertService) sendDingTalk(title, msg string) error {
	if a.cfg.Alert.DingTalkWebhook == "" {
		return nil
	}
	payload := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": title,
			"text":  "### " + title + "\n\n" + msg,
		},
	}
	return a.httpPost(a.cfg.Alert.DingTalkWebhook, payload)
}

func (a *AlertService) sendSlack(title, msg string) error {
	if a.cfg.Alert.SlackWebhook == "" {
		return nil
	}
	payload := map[string]interface{}{
		"attachments": []interface{}{
			map[string]interface{}{
				"color": "danger",
				"title": title,
				"text":  msg,
				"ts":    time.Now().Unix(),
			},
		},
	}
	return a.httpPost(a.cfg.Alert.SlackWebhook, payload)
}

func (a *AlertService) sendCustomWebhook(url, title, msg string) error {
	payload := map[string]string{
		"title":   title,
		"message": msg,
		"time":    time.Now().Format(time.RFC3339),
	}
	return a.httpPost(url, payload)
}

func (a *AlertService) httpPost(url string, payload interface{}) error {
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

func (a *AlertService) sendEmail(recipients []string, subject, body string) error {
	if a.cfg.Alert.SMTPHost == "" {
		return nil
	}
	msg := "From: " + a.cfg.Alert.SMTPFrom + "\r\n" +
		"To: " + strings.Join(recipients, ",") + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n\r\n" +
		body
	auth := smtp.PlainAuth("", a.cfg.Alert.SMTPUser, a.cfg.Alert.SMTPPassword, a.cfg.Alert.SMTPHost)
	addr := fmt.Sprintf("%s:%d", a.cfg.Alert.SMTPHost, a.cfg.Alert.SMTPPort)
	return smtp.SendMail(addr, auth, a.cfg.Alert.SMTPFrom, recipients, []byte(msg))
}

func (a *AlertService) Acknowledge(alertID uint, userID uint, note string) error {
	now := time.Now()
	return a.db.Model(&models.AlertEvent{}).Where("id = ?", alertID).Updates(map[string]interface{}{
		"status":             models.AlertAcknowledged,
		"acknowledged_at":    now,
		"acknowledged_by_id": userID,
		"ack_note":           note,
		"updated_at":         now,
	}).Error
}

func (a *AlertService) Resolve(alertID uint, userID uint, note string) error {
	now := time.Now()
	return a.db.Model(&models.AlertEvent{}).Where("id = ?", alertID).Updates(map[string]interface{}{
		"status":           models.AlertResolved,
		"resolved_at":      now,
		"resolved_by_id":   userID,
		"resolve_note":     note,
		"updated_at":       now,
	}).Error
}

type PaginatedAlerts struct {
	Total    int64               `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"pageSize"`
	Data     []models.AlertEvent `json:"data"`
}

func (a *AlertService) List(tenantID uint, isSuper bool, status, severity string, days, page, pageSize int) (*PaginatedAlerts, error) {
	var alerts []models.AlertEvent
	q := a.db.Model(&models.AlertEvent{})
	if !isSuper {
		q = q.Where("tenant_id = ?", tenantID)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if severity != "" {
		q = q.Where("severity = ?", severity)
	}
	if days > 0 {
		since := time.Now().AddDate(0, 0, -days)
		q = q.Where("triggered_at >= ?", since)
	}
	q = q.Preload("Pipeline").Preload("AcknowledgedBy").Preload("ResolvedBy").Order("triggered_at DESC")

	var total int64
	q.Count(&total)
	offset := (page - 1) * pageSize
	q.Offset(offset).Limit(pageSize).Find(&alerts)
	return &PaginatedAlerts{
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		Data:     alerts,
	}, nil
}

func (a *AlertService) GetEscalations(alertID uint) ([]models.AlertEscalation, error) {
	var escalations []models.AlertEscalation
	err := a.db.Where("alert_id = ?", alertID).Order("triggered_at DESC").Find(&escalations).Error
	return escalations, err
}

func (a *AlertService) getOnCallLeader(pipelineID, tenantID uint) *models.User {
	var assignments []models.OnCallAssignment
	a.db.Where("(pipeline_id = ? OR pipeline_id IS NULL) AND tenant_id = ? AND start_date <= ? AND end_date >= ?",
		pipelineID, tenantID, time.Now(), time.Now()).Find(&assignments)

	if len(assignments) == 0 {
		return nil
	}

	groupIDs := make(map[uint]bool)
	for _, asn := range assignments {
		groupIDs[asn.GroupID] = true
	}

	var groups []models.OnCallGroup
	a.db.Where("id IN ?", keysToSlice(groupIDs)).Preload("Leader").Find(&groups)

	for _, g := range groups {
		if g.LeaderID != nil && g.Leader != nil {
			return g.Leader
		}
	}

	var admin models.User
	err := a.db.Where("tenant_id = ? AND role = ? AND status = ?", tenantID, models.RoleAdmin, "active").First(&admin).Error
	if err != nil {
		return nil
	}
	return &admin
}

func keysToSlice[K comparable, V any](m map[K]V) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func (a *AlertService) escalateAlert(event *models.AlertEvent, rule *models.AlertRule) error {
	now := time.Now()
	fromSev := event.Severity
	toSev := rule.EscalationToSeverity
	if toSev == "" {
		toSev = models.AlertCritical
	}

	if fromSev == toSev {
		return nil
	}

	tx := a.db.Begin()

	event.Severity = toSev
	event.UpdatedAt = now
	if err := tx.Save(event).Error; err != nil {
		tx.Rollback()
		return err
	}

	escalation := models.AlertEscalation{
		AlertID:      event.ID,
		TenantID:     event.TenantID,
		FromSeverity: fromSev,
		ToSeverity:   toSev,
		Reason:       fmt.Sprintf("触发后%d分钟未认领自动升级", rule.EscalationAfterMin),
		TriggeredBy:  "system",
		TriggeredAt:  now,
		CreatedAt:    now,
	}

	leader := a.getOnCallLeader(*event.PipelineID, event.TenantID)
	if leader != nil {
		escalation.NotifiedLeader = true
		escalation.TriggeredByID = &leader.ID
	}

	if err := tx.Create(&escalation).Error; err != nil {
		tx.Rollback()
		return err
	}

	tx.Commit()

	title := fmt.Sprintf("[告警升级] %s", event.Title)
	msg := fmt.Sprintf("告警已从%s升级为%s，原因：%s\n\n原告警内容：%s",
		severityText(fromSev), severityText(toSev), escalation.Reason, event.Message)

	a.notifyChannels(event, rule, title, msg, true)

	if leader != nil && leader.Email != "" {
		a.sendEmail([]string{leader.Email}, title, msg)
	}

	if a.wsFn != nil {
		a.wsFn(event.TenantID, "alert_escalated", map[string]interface{}{
			"id":           event.ID,
			"fromSeverity": fromSev,
			"toSeverity":   toSev,
			"escalationId": escalation.ID,
		})
	}

	return nil
}

func severityText(sev models.AlertSeverity) string {
	switch sev {
	case models.AlertInfo:
		return "提示"
	case models.AlertWarning:
		return "警告"
	case models.AlertCritical:
		return "严重"
	default:
		return string(sev)
	}
}

func (a *AlertService) CheckAndEscalate() {
	now := time.Now()
	var rules []models.AlertRule
	a.db.Where("escalation_enabled = ? AND enabled = ?", true, true).Find(&rules)

	ruleMap := make(map[uint]*models.AlertRule)
	for i := range rules {
		ruleMap[rules[i].ID] = &rules[i]
	}

	var events []models.AlertEvent
	a.db.Where("status = ? AND triggered_at <= ?",
		models.AlertTriggered, now.Add(-15*time.Minute)).
		Preload("Rule").
		Find(&events)

	for i := range events {
		event := &events[i]
		rule := event.Rule
		if rule == nil {
			if r, ok := ruleMap[event.RuleID]; ok {
				rule = r
			}
		}
		if rule == nil || !rule.EscalationEnabled {
			continue
		}

		escalateAfter := time.Duration(rule.EscalationAfterMin) * time.Minute
		if escalateAfter <= 0 {
			escalateAfter = 15 * time.Minute
		}

		if now.Sub(event.TriggeredAt) < escalateAfter {
			continue
		}

		var existingEsc int64
		a.db.Model(&models.AlertEscalation{}).Where("alert_id = ?", event.ID).Count(&existingEsc)
		if existingEsc > 0 {
			continue
		}

		go a.escalateAlert(event, rule)
	}
}

func (a *AlertService) isInSilentPeriod(rule *models.AlertRule, t time.Time) bool {
	if !rule.SilentEnabled {
		return false
	}
	startStr := rule.SilentStart
	endStr := rule.SilentEnd
	if startStr == "" || endStr == "" {
		return false
	}

	startTime := parseTimeOfDay(startStr)
	endTime := parseTimeOfDay(endStr)
	current := time.Date(0, 1, 1, t.Hour(), t.Minute(), t.Second(), 0, time.Local)

	if startTime.Before(endTime) {
		return !current.Before(startTime) && !current.After(endTime)
	} else {
		return !current.Before(startTime) || !current.After(endTime)
	}
}

func parseTimeOfDay(s string) time.Time {
	parts := strings.Split(s, ":")
	if len(parts) < 2 {
		return time.Date(0, 1, 1, 0, 0, 0, 0, time.Local)
	}
	h := 0
	m := 0
	fmt.Sscanf(parts[0], "%d", &h)
	fmt.Sscanf(parts[1], "%d", &m)
	return time.Date(0, 1, 1, h, m, 0, 0, time.Local)
}

func (a *AlertService) getAlertsForSilentSummary(ruleID uint, since time.Time) []models.AlertEvent {
	var alerts []models.AlertEvent
	a.db.Where("rule_id = ? AND triggered_at >= ? AND status IN ?",
		ruleID, since,
		[]string{string(models.AlertTriggered), string(models.AlertAcknowledged)}).
		Order("triggered_at DESC").Find(&alerts)
	return alerts
}

func (a *AlertService) CheckSilentPeriodEnd() {
	now := time.Now()
	var rules []models.AlertRule
	a.db.Where("silent_enabled = ? AND silent_summary_enabled = ? AND enabled = ?",
		true, true, true).Find(&rules)

	for i := range rules {
		rule := &rules[i]
		if !a.justExitedSilentPeriod(rule, now) {
			continue
		}

		startTime := parseTimeOfDay(rule.SilentStart)
		silentStartToday := time.Date(now.Year(), now.Month(), now.Day(),
			startTime.Hour(), startTime.Minute(), 0, 0, now.Location())

		alerts := a.getAlertsForSilentSummary(rule.ID, silentStartToday)
		if len(alerts) == 0 {
			continue
		}

		title := fmt.Sprintf("[静默期汇总] %d条未恢复告警", len(alerts))
		var msgBuilder strings.Builder
		msgBuilder.WriteString(fmt.Sprintf("静默期(%s-%s)内共有%d条未恢复告警：\n\n",
			rule.SilentStart, rule.SilentEnd, len(alerts)))
		for i, alert := range alerts {
			if i >= 10 {
				msgBuilder.WriteString(fmt.Sprintf("... 还有%d条更多\n", len(alerts)-10))
				break
			}
			msgBuilder.WriteString(fmt.Sprintf("[%s] %s\n", severityText(alert.Severity), alert.Title))
		}

		var channels []string
		json.Unmarshal([]byte(rule.Channels), &channels)
		for _, ch := range channels {
			switch models.AlertChannelType(ch) {
			case models.AlertWebhookFeishu:
				a.sendFeishu(title, msgBuilder.String())
			case models.AlertWebhookDingTalk:
				a.sendDingTalk(title, msgBuilder.String())
			case models.AlertWebhookSlack:
				a.sendSlack(title, msgBuilder.String())
			}
		}
	}
}

func (a *AlertService) justExitedSilentPeriod(rule *models.AlertRule, now time.Time) bool {
	endTime := parseTimeOfDay(rule.SilentEnd)
	endToday := time.Date(now.Year(), now.Month(), now.Day(),
		endTime.Hour(), endTime.Minute(), 0, 0, now.Location())

	diff := now.Sub(endToday)
	return diff >= 0 && diff <= 2*time.Minute
}

func (a *AlertService) StartScheduler() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		a.CheckAndEscalate()
		a.CheckSilentPeriodEnd()
	}
}

type AlertTrendData struct {
	Dates       []string `json:"dates"`
	InfoCounts  []int    `json:"infoCounts"`
	WarnCounts  []int    `json:"warnCounts"`
	CritCounts  []int    `json:"critCounts"`
	TotalCounts []int    `json:"totalCounts"`
}

type AlertWeekComparison struct {
	ThisWeekCount  int     `json:"thisWeekCount"`
	LastWeekCount  int     `json:"lastWeekCount"`
	ChangePercent  float64 `json:"changePercent"`
	IsIncrease     bool    `json:"isIncrease"`
}

type AlertTrendResponse struct {
	Trend      AlertTrendData      `json:"trend"`
	Comparison AlertWeekComparison `json:"comparison"`
}

func (a *AlertService) GetTrendStats(tenantID uint, isSuper bool) (*AlertTrendResponse, error) {
	now := time.Now()
	loc := now.Location()

	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	dates := make([]string, 7)
	infoCounts := make([]int, 7)
	warnCounts := make([]int, 7)
	critCounts := make([]int, 7)
	totalCounts := make([]int, 7)

	for i := 6; i >= 0; i-- {
		day := today.AddDate(0, 0, -i)
		dates[6-i] = day.Format("01-02")
	}

	type dailyStat struct {
		Date     string
		Severity string
		Count    int
	}

	startDate := today.AddDate(0, 0, -6)
	endDate := today.AddDate(0, 0, 1)

	var results []dailyStat
	q := a.db.Model(&models.AlertEvent{}).
		Select("DATE(triggered_at) as date, severity, COUNT(*) as count").
		Where("triggered_at >= ? AND triggered_at < ?", startDate, endDate)
	if !isSuper {
		q = q.Where("tenant_id = ?", tenantID)
	}
	q.Group("date, severity").Order("date").Scan(&results)

	for _, r := range results {
		t, err := time.Parse("2006-01-02", r.Date)
		if err != nil {
			continue
		}
		dateStr := t.Format("01-02")
		idx := -1
		for i, d := range dates {
			if d == dateStr {
				idx = i
				break
			}
		}
		if idx < 0 {
			continue
		}
		switch r.Severity {
		case string(models.AlertInfo):
			infoCounts[idx] = r.Count
		case string(models.AlertWarning):
			warnCounts[idx] = r.Count
		case string(models.AlertCritical):
			critCounts[idx] = r.Count
		}
		totalCounts[idx] += r.Count
	}

	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	thisWeekStart := today.AddDate(0, 0, -(weekday - 1))
	lastWeekStart := thisWeekStart.AddDate(0, 0, -7)
	lastWeekEnd := thisWeekStart

	var thisWeekCount int64
	q2 := a.db.Model(&models.AlertEvent{}).
		Where("triggered_at >= ? AND triggered_at < ?", thisWeekStart, endDate)
	if !isSuper {
		q2 = q2.Where("tenant_id = ?", tenantID)
	}
	q2.Count(&thisWeekCount)

	var lastWeekCount int64
	q3 := a.db.Model(&models.AlertEvent{}).
		Where("triggered_at >= ? AND triggered_at < ?", lastWeekStart, lastWeekEnd)
	if !isSuper {
		q3 = q3.Where("tenant_id = ?", tenantID)
	}
	q3.Count(&lastWeekCount)

	changePercent := 0.0
	isIncrease := false
	if lastWeekCount > 0 {
		changePercent = float64(thisWeekCount-lastWeekCount) / float64(lastWeekCount) * 100
		isIncrease = thisWeekCount > lastWeekCount
	} else if thisWeekCount > 0 {
		changePercent = 100.0
		isIncrease = true
	}

	return &AlertTrendResponse{
		Trend: AlertTrendData{
			Dates:       dates,
			InfoCounts:  infoCounts,
			WarnCounts:  warnCounts,
			CritCounts:  critCounts,
			TotalCounts: totalCounts,
		},
		Comparison: AlertWeekComparison{
			ThisWeekCount: int(thisWeekCount),
			LastWeekCount: int(lastWeekCount),
			ChangePercent: changePercent,
			IsIncrease:    isIncrease,
		},
	}, nil
}

type AlertAggregateGroup struct {
	GroupKey    string              `json:"groupKey"`
	PipelineID  *uint               `json:"pipelineId"`
	RuleID      uint                `json:"ruleId"`
	RuleType    string              `json:"ruleType"`
	Severity    string              `json:"severity"`
	Count       int                 `json:"count"`
	LatestAlert *models.AlertEvent  `json:"latestAlert"`
	Alerts      []models.AlertEvent `json:"alerts"`
}

type PaginatedAlertGroups struct {
	Total    int64                `json:"total"`
	Page     int                  `json:"page"`
	PageSize int                  `json:"pageSize"`
	Data     []AlertAggregateGroup `json:"data"`
}

func (a *AlertService) ListAggregated(tenantID uint, isSuper bool, status, severity string, days, page, pageSize int) (*PaginatedAlertGroups, error) {
	now := time.Now()
	window := 30 * time.Minute

	var allAlerts []models.AlertEvent
	q := a.db.Model(&models.AlertEvent{})
	if !isSuper {
		q = q.Where("tenant_id = ?", tenantID)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if severity != "" {
		q = q.Where("severity = ?", severity)
	}
	if days > 0 {
		since := now.AddDate(0, 0, -days)
		q = q.Where("triggered_at >= ?", since)
	}
	q.Preload("Pipeline").Preload("AcknowledgedBy").Preload("ResolvedBy").Preload("Rule").
		Order("triggered_at DESC").Find(&allAlerts)

	ruleIDMap := make(map[uint]string)
	for i := range allAlerts {
		alert := &allAlerts[i]
		if alert.Rule != nil {
			ruleIDMap[alert.RuleID] = string(alert.Rule.RuleType)
		}
	}

	if len(ruleIDMap) > 0 {
		ruleIDs := make([]uint, 0, len(ruleIDMap))
		for id, t := range ruleIDMap {
			if t == "" {
				ruleIDs = append(ruleIDs, id)
			}
		}
		if len(ruleIDs) > 0 {
			var rules []models.AlertRule
			a.db.Where("id IN ?", ruleIDs).Find(&rules)
			for _, r := range rules {
				ruleIDMap[r.ID] = string(r.RuleType)
			}
		}
	}

	type rawGroup struct {
		Alerts []models.AlertEvent
	}
	groups := make(map[string]*rawGroup)
	groupList := make([]string, 0)

	for i := range allAlerts {
		alert := allAlerts[i]
		var pid uint
		if alert.PipelineID != nil {
			pid = *alert.PipelineID
		}

		ruleType := ruleIDMap[alert.RuleID]
		if ruleType == "" {
			ruleType = "unknown"
		}

		key := fmt.Sprintf("%d|%d|%s|%s", pid, alert.RuleID, ruleType, alert.Severity)

		group, exists := groups[key]
		if !exists {
			group = &rawGroup{
				Alerts: make([]models.AlertEvent, 0),
			}
			groups[key] = group
			groupList = append(groupList, key)
		}

		group.Alerts = append(group.Alerts, alert)
	}

	shouldAggregate := func(alerts []models.AlertEvent) bool {
		if len(alerts) < 3 {
			return false
		}
		sorted := make([]time.Time, len(alerts))
		for i, a := range alerts {
			sorted[i] = a.TriggeredAt
		}
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].Before(sorted[j]) })

		for i := 0; i <= len(sorted)-3; i++ {
			if sorted[i+2].Sub(sorted[i]) <= window {
				return true
			}
		}
		return false
	}

	resultGroups := make([]AlertAggregateGroup, 0)
	for _, key := range groupList {
		raw := groups[key]

		if !shouldAggregate(raw.Alerts) {
			for _, alert := range raw.Alerts {
				rt := ruleIDMap[alert.RuleID]
				if rt == "" {
					rt = "unknown"
				}
				singleGroup := AlertAggregateGroup{
					GroupKey:    fmt.Sprintf("single-%d", alert.ID),
					PipelineID:  alert.PipelineID,
					RuleID:      alert.RuleID,
					RuleType:    rt,
					Severity:    string(alert.Severity),
					Count:       1,
					LatestAlert: &alert,
					Alerts:      []models.AlertEvent{alert},
				}
				resultGroups = append(resultGroups, singleGroup)
			}
		} else {
			sort.Slice(raw.Alerts, func(i, j int) bool {
				return raw.Alerts[i].TriggeredAt.After(raw.Alerts[j].TriggeredAt)
			})
			latest := &raw.Alerts[0]
			rt := ruleIDMap[latest.RuleID]
			if rt == "" {
				rt = "unknown"
			}
			group := AlertAggregateGroup{
				GroupKey:    key,
				PipelineID:  latest.PipelineID,
				RuleID:      latest.RuleID,
				RuleType:    rt,
				Severity:    string(latest.Severity),
				Count:       len(raw.Alerts),
				LatestAlert: latest,
				Alerts:      raw.Alerts,
			}
			resultGroups = append(resultGroups, group)
		}
	}

	total := int64(len(resultGroups))
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > len(resultGroups) {
		start = len(resultGroups)
	}
	if end > len(resultGroups) {
		end = len(resultGroups)
	}
	pagedData := resultGroups[start:end]

	return &PaginatedAlertGroups{
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		Data:     pagedData,
	}, nil
}

func (a *AlertService) BatchAcknowledge(alertIDs []uint, userID uint, note string) (int, error) {
	if len(alertIDs) == 0 {
		return 0, nil
	}
	now := time.Now()
	result := a.db.Model(&models.AlertEvent{}).
		Where("id IN ? AND status = ?", alertIDs, models.AlertTriggered).
		Updates(map[string]interface{}{
			"status":             models.AlertAcknowledged,
			"acknowledged_at":    now,
			"acknowledged_by_id": userID,
			"ack_note":           note,
			"updated_at":         now,
		})
	return int(result.RowsAffected), result.Error
}

func (a *AlertService) BatchResolve(alertIDs []uint, userID uint, note string) (int, error) {
	if len(alertIDs) == 0 {
		return 0, nil
	}
	now := time.Now()
	result := a.db.Model(&models.AlertEvent{}).
		Where("id IN ? AND status IN ?", alertIDs, []string{string(models.AlertTriggered), string(models.AlertAcknowledged)}).
		Updates(map[string]interface{}{
			"status":         models.AlertResolved,
			"resolved_at":    now,
			"resolved_by_id": userID,
			"resolve_note":   note,
			"updated_at":     now,
		})
	return int(result.RowsAffected), result.Error
}
