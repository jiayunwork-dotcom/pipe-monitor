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
		a.wsFn(tenantID, "new_alert", map[string]interface{}{
			"id":         event.ID,
			"title":      title,
			"severity":   event.Severity,
			"status":     event.Status,
			"pipelineId": pipe.ID,
			"pipelineName": pipe.Name,
			"triggeredAt": now,
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

func (a *AlertService) notifyChannels(event *models.AlertEvent, rule *models.AlertRule, title, message string) {
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
