package services

import (
	"errors"
	"pipe-monitor/internal/models"
	"pipe-monitor/internal/utils"
	"time"

	"gorm.io/gorm"

	redisClient "pipe-monitor/internal/redis"
)

type RunService struct {
	db           *gorm.DB
	rdb          *redisClient.Client
	slaEngine    *SLAEngine
	alertService *AlertService
	wsBroadcast  func(tenantID uint, msgType string, payload interface{})
}

func NewRunService(db *gorm.DB, rdb *redisClient.Client, alertService *AlertService) *RunService {
	return &RunService{
		db:           db,
		rdb:          rdb,
		alertService: alertService,
	}
}

func (s *RunService) SetWSPublisher(fn func(tenantID uint, msgType string, payload interface{})) {
	s.wsBroadcast = fn
}

func (s *RunService) SetSLAEngine(engine *SLAEngine) {
	s.slaEngine = engine
}

type ReportRunReq struct {
	RunID           string      `json:"runId"`
	PipelineCode    string      `json:"pipelineCode"`
	PipelineID      uint        `json:"pipelineId"`
	TenantID        uint        `json:"tenantId"`
	Token           string      `json:"token"`
	Status          string      `json:"status"`
	TriggerType     string      `json:"triggerType"`
	TriggeredBy     string      `json:"triggeredBy"`
	ActualStart     *time.Time  `json:"actualStart"`
	ActualEnd       *time.Time  `json:"actualEnd"`
	DurationSec     int         `json:"durationSec"`
	ErrorMessage    string      `json:"errorMessage"`
	DataVolume      int64       `json:"dataVolume"`
	LogsURL         string      `json:"logsUrl"`
	ExternalURL     string      `json:"externalUrl"`
}

func (s *RunService) ReportRun(req *ReportRunReq) (*models.PipelineRun, error) {
	var pipe models.Pipeline
	q := s.db.Where("code = ?", req.PipelineCode)
	if req.PipelineID > 0 {
		q = s.db.Where("id = ? OR code = ?", req.PipelineID, req.PipelineCode)
	}
	if err := q.First(&pipe).Error; err != nil {
		return nil, errors.New("管道不存在")
	}

	if pipe.WebhookToken != "" && req.Token != pipe.WebhookToken {
	}

	if req.TenantID == 0 {
		req.TenantID = pipe.TenantID
	}

	var run models.PipelineRun
	isNew := false
	err := s.db.Where("run_id = ?", req.RunID).First(&run).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			isNew = true
			run = models.PipelineRun{
				TenantID:    req.TenantID,
				PipelineID:  pipe.ID,
				RunID:       req.RunID,
				TriggerType: req.TriggerType,
				TriggeredBy: req.TriggeredBy,
				Status:      models.RunPending,
				AttemptCount: 1,
				MaxAttempts:  3,
			}
		} else {
			return nil, err
		}
	}

	if req.ActualStart != nil {
		run.ActualStart = req.ActualStart
	}
	if req.ActualEnd != nil {
		run.ActualEnd = req.ActualEnd
	}
	if req.DurationSec > 0 {
		run.DurationSec = req.DurationSec
	}
	if req.Status != "" {
		run.Status = models.RunStatus(req.Status)
	}
	if req.ErrorMessage != "" {
		run.ErrorMessage = req.ErrorMessage
	}
	if req.DataVolume > 0 {
		run.DataVolume = req.DataVolume
	}
	if req.LogsURL != "" {
		run.LogsURL = req.LogsURL
	}
	if req.ExternalURL != "" {
		run.ExternalURL = req.ExternalURL
	}

	if run.Status == models.RunSuccess || run.Status == models.RunFailed ||
		run.Status == models.RunTimeout || run.Status == models.RunCancelled {
		if run.ActualStart != nil && run.ActualEnd != nil && run.DurationSec == 0 {
			run.DurationSec = int(run.ActualEnd.Sub(*run.ActualStart).Seconds())
		}
	}

	run.HealthStatus = s.computeHealth(run.Status)

	if isNew {
		if err := s.db.Create(&run).Error; err != nil {
			return nil, err
		}
	} else {
		if err := s.db.Save(&run).Error; err != nil {
			return nil, err
		}
	}

	if s.slaEngine != nil {
		s.slaEngine.EvaluateRun(pipe.ID, run.ID)
	}

	if s.alertService != nil {
		go s.alertService.ProcessRunStatus(&pipe, &run)
	}

	if s.wsBroadcast != nil {
		s.wsBroadcast(pipe.TenantID, "run_status_change", map[string]interface{}{
			"pipelineId": pipe.ID,
			"pipelineCode": pipe.Code,
			"run": map[string]interface{}{
				"id":          run.ID,
				"runId":       run.RunID,
				"status":      run.Status,
				"health":      run.HealthStatus,
				"actualStart": run.ActualStart,
				"actualEnd":   run.ActualEnd,
				"durationSec": run.DurationSec,
			},
		})
		s.wsBroadcast(pipe.TenantID, "pipeline_status_change", map[string]interface{}{
			"pipelineId":   pipe.ID,
			"pipelineCode": pipe.Code,
			"health":       run.HealthStatus,
			"lastRunStatus": run.Status,
			"lastRunAt":    run.ActualStart,
		})
	}

	return &run, nil
}

func (s *RunService) computeHealth(status models.RunStatus) string {
	switch status {
	case models.RunSuccess:
		return "green"
	case models.RunRunning:
		return "blue"
	case models.RunFailed, models.RunTimeout:
		return "red"
	case models.RunPending:
		return "gray"
	case models.RunSkipped, models.RunCancelled:
		return "yellow"
	default:
		return "gray"
	}
}

type PaginatedRuns struct {
	Total    int64                `json:"total"`
	Page     int                  `json:"page"`
	PageSize int                  `json:"pageSize"`
	Data     []models.PipelineRun `json:"data"`
}

func (s *RunService) ListRuns(tenantID uint, isSuper bool, pipelineID uint, status string, days int, page, pageSize int) (*PaginatedRuns, error) {
	var runs []models.PipelineRun
	q := s.db.Model(&models.PipelineRun{})
	if !isSuper {
		q = q.Where("tenant_id = ?", tenantID)
	}
	if pipelineID > 0 {
		q = q.Where("pipeline_id = ?", pipelineID)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if days > 0 {
		since := time.Now().AddDate(0, 0, -days)
		q = q.Where("created_at >= ?", since)
	}
	q = q.Preload("Pipeline").Order("created_at DESC")

	var count int64
	q.Count(&count)
	offset := (page - 1) * pageSize
	q.Offset(offset).Limit(pageSize).Find(&runs)

	return &PaginatedRuns{
		Total:    count,
		Page:     page,
		PageSize: pageSize,
		Data:     runs,
	}, nil
}

func (s *RunService) GetDashboardStats(tenantID uint, isSuper bool) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	pipesQ := s.db.Model(&models.Pipeline{})
	if !isSuper {
		pipesQ = pipesQ.Where("tenant_id = ?", tenantID)
	}
	var totalPipes int64
	pipesQ.Count(&totalPipes)
	result["totalPipelines"] = totalPipes

	todayStart := time.Now().Truncate(24 * time.Hour)
	runsQ := s.db.Model(&models.PipelineRun{}).Where("created_at >= ?", todayStart)
	if !isSuper {
		runsQ = runsQ.Where("tenant_id = ?", tenantID)
	}

	var todayTotal int64
	runsQ.Count(&todayTotal)
	result["todayRuns"] = todayTotal

	var successCount int64
	s.db.Model(&models.PipelineRun{}).Where("created_at >= ? AND status = ?", todayStart, models.RunSuccess).
		Scopes(s.tenantScope(tenantID, isSuper)).Count(&successCount)
	result["todaySuccess"] = successCount

	var failedCount int64
	s.db.Model(&models.PipelineRun{}).Where("created_at >= ? AND status IN ?", todayStart, []string{string(models.RunFailed), string(models.RunTimeout)}).
		Scopes(s.tenantScope(tenantID, isSuper)).Count(&failedCount)
	result["todayFailed"] = failedCount

	var runningCount int64
	s.db.Model(&models.PipelineRun{}).Where("status = ?", models.RunRunning).
		Scopes(s.tenantScope(tenantID, isSuper)).Count(&runningCount)
	result["runningNow"] = runningCount

	var slaBreachCount int64
	s.db.Model(&models.PipelineRun{}).Where("created_at >= ? AND sla_result = ?", todayStart, models.SLABreached).
		Scopes(s.tenantScope(tenantID, isSuper)).Count(&slaBreachCount)
	result["todaySLABreach"] = slaBreachCount

	openAlertQ := s.db.Model(&models.AlertEvent{}).Where("status IN ?", []string{string(models.AlertTriggered), string(models.AlertAcknowledged)})
	if !isSuper {
		openAlertQ = openAlertQ.Where("tenant_id = ?", tenantID)
	}
	var openAlertCount int64
	openAlertQ.Count(&openAlertCount)
	result["openAlerts"] = openAlertCount

	type pipeHealthItem struct {
		PipelineID uint   `json:"pipelineId"`
		Code       string `json:"code"`
		Name       string `json:"name"`
		Team       string `json:"team"`
		DataDomain string `json:"dataDomain"`
		LastRunID  uint   `json:"lastRunId"`
		LastStatus string `json:"lastStatus"`
		Health     string `json:"health"`
		SLA        string `json:"sla"`
		LastRunAt  *time.Time `json:"lastRunAt"`
	}
	var pipes []models.Pipeline
	if !isSuper {
		s.db.Where("tenant_id = ? AND status = ?", tenantID, models.PipelineActive).Find(&pipes)
	} else {
		s.db.Where("status = ?", models.PipelineActive).Find(&pipes)
	}

	pipelineList := make([]pipeHealthItem, 0)
	for _, p := range pipes {
		var lastRun models.PipelineRun
		s.db.Where("pipeline_id = ?", p.ID).Order("created_at DESC").First(&lastRun)
		item := pipeHealthItem{
			PipelineID: p.ID,
			Code:       p.Code,
			Name:       p.Name,
			Team:       p.Team,
			DataDomain: p.DataDomain,
			LastStatus: string(lastRun.Status),
			Health:     s.computeHealth(lastRun.Status),
			SLA:        string(lastRun.SlaResult),
			LastRunAt:  lastRun.ActualStart,
			LastRunID:  lastRun.ID,
		}
		pipelineList = append(pipelineList, item)
	}
	result["pipelines"] = pipelineList
	_ = utils.UintToStr

	return result, nil
}

func (s *RunService) tenantScope(tenantID uint, isSuper bool) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if isSuper {
			return db
		}
		return db.Where("tenant_id = ?", tenantID)
	}
}

type RunHistoryGanttItem struct {
	ID          uint       `json:"id"`
	RunID       string     `json:"runId"`
	Status      string     `json:"status"`
	Start       time.Time  `json:"start"`
	End         *time.Time `json:"end"`
	DurationSec int        `json:"durationSec"`
	Health      string     `json:"health"`
	SLA         string     `json:"sla"`
	Day         string     `json:"day"`
}

func (s *RunService) GetRunHistory(pipelineID uint, days int) ([]RunHistoryGanttItem, error) {
	since := time.Now().AddDate(0, 0, -days)
	var runs []models.PipelineRun
	if err := s.db.Where("pipeline_id = ? AND created_at >= ?", pipelineID, since).
		Order("created_at DESC").Limit(300).Find(&runs).Error; err != nil {
		return nil, err
	}

	result := make([]RunHistoryGanttItem, 0, len(runs))
	for _, r := range runs {
		start := r.CreatedAt
		if r.ActualStart != nil {
			start = *r.ActualStart
		}
		day := start.Format("2006-01-02")
		result = append(result, RunHistoryGanttItem{
			ID:          r.ID,
			RunID:       r.RunID,
			Status:      string(r.Status),
			Start:       start,
			End:         r.ActualEnd,
			DurationSec: r.DurationSec,
			Health:      s.computeHealth(r.Status),
			SLA:         string(r.SlaResult),
			Day:         day,
		})
	}
	return result, nil
}
