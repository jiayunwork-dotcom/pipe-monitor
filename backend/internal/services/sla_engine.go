package services

import (
	"pipe-monitor/internal/config"
	"pipe-monitor/internal/models"
	"pipe-monitor/internal/utils"
	"time"

	"gorm.io/gorm"

	redisClient "pipe-monitor/internal/redis"
)

type SLAEngine struct {
	db           *gorm.DB
	rdb          *redisClient.Client
	alertService *AlertService
}

func NewSLAEngine(db *gorm.DB, rdb *redisClient.Client, alertService *AlertService) *SLAEngine {
	return &SLAEngine{db: db, rdb: rdb, alertService: alertService}
}

func (e *SLAEngine) StartScheduler() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		e.EvaluateAllRunning()
		e.GenerateMonthlyReportIfNeeded()
	}
}

func (e *SLAEngine) EvaluateAllRunning() {
	var runningRuns []models.PipelineRun
	e.db.Where("status = ?", models.RunRunning).Find(&runningRuns)
	for _, run := range runningRuns {
		e.EvaluateRun(run.PipelineID, run.ID)
	}
}

func (e *SLAEngine) EvaluateRun(pipelineID, runID uint) {
	var run models.PipelineRun
	if err := e.db.Preload("Pipeline").First(&run, runID).Error; err != nil {
		return
	}

	var rules []models.SLARule
	e.db.Where("pipeline_id = ? AND enabled = ?", pipelineID, true).Find(&rules)

	now := time.Now()
	overallSLA := models.SLAAchieved
	hasRunning := false

	for _, rule := range rules {
		result := models.SLAUnknown
		actualVal := 0.0
		threshold := 0.0
		breachSec := 0

		if !e.matchDateType(rule.DateType, now) {
			continue
		}

		switch rule.RuleType {
		case models.SLAFinishByTime:
			deadline, err := utils.GetTodayDeadline(now, rule.FinishDeadlineTime)
			if err != nil {
				continue
			}
			threshold = float64(rule.MaxDurationSec)
			if run.Status == models.RunSuccess && run.ActualEnd != nil {
				if run.ActualEnd.Before(deadline) {
					result = models.SLAAchieved
				} else {
					result = models.SLABreached
					breachSec = int(run.ActualEnd.Sub(deadline).Seconds())
					actualVal = float64(breachSec)
				}
			} else if run.Status == models.RunRunning {
				hasRunning = true
				p50, p95 := e.GetHistoryStats(pipelineID, 7)
				expectedEnd := now.Add(time.Duration(p50) * time.Second)
				if run.ActualStart != nil {
					elapsed := int(now.Sub(*run.ActualStart).Seconds())
					remaining := p50 - elapsed
					if remaining <= 0 {
						remaining = p95 - elapsed
					}
					expectedEnd = now.Add(time.Duration(remaining) * time.Second)
				}
				actualVal = expectedEnd.Sub(deadline).Seconds()
				if expectedEnd.After(deadline) {
					result = models.SLAPredicted
					if rule.WarnThresholdSec > 0 {
						secToDeadline := int(deadline.Sub(now).Seconds())
						if secToDeadline <= rule.WarnThresholdSec {
							if e.alertService != nil {
								go e.alertService.ProcessSLAImminent(&run.Pipeline, &run, &rule, secToDeadline)
							}
						}
					}
				} else {
					result = models.SLARunning
				}
			} else {
				result = models.SLAUnknown
			}

		case models.SLAMaxDuration:
			threshold = float64(rule.MaxDurationSec)
			if run.DurationSec > 0 {
				actualVal = float64(run.DurationSec)
				if run.DurationSec > rule.MaxDurationSec {
					result = models.SLABreached
					breachSec = run.DurationSec - rule.MaxDurationSec
				} else if run.Status == models.RunSuccess || run.Status == models.RunFailed {
					result = models.SLAAchieved
				} else {
					result = models.SLARunning
					hasRunning = true
				}
			} else if run.Status == models.RunRunning && run.ActualStart != nil {
				elapsed := int(now.Sub(*run.ActualStart).Seconds())
				actualVal = float64(elapsed)
				if elapsed > rule.MaxDurationSec {
					result = models.SLABreached
					breachSec = elapsed - rule.MaxDurationSec
				} else {
					result = models.SLARunning
					hasRunning = true
				}
			}

		case models.SLAMaxConsecFail:
			failN := e.CountConsecutiveFailures(pipelineID, run.ID)
			threshold = float64(rule.MaxConsecutiveFail)
			actualVal = float64(failN)
			if run.Status == models.RunFailed && failN >= rule.MaxConsecutiveFail {
				result = models.SLABreached
			} else {
				result = models.SLAAchieved
			}
		}

		if result != models.SLAUnknown {
			eval := models.SLAEvaluation{
				TenantID:       run.TenantID,
				RunID:          runID,
				RuleID:         rule.ID,
				Result:         result,
				ActualValue:    actualVal,
				ThresholdValue: threshold,
				BreachSec:      breachSec,
				EvaluatedAt:    now,
			}
			if result == models.SLAPredicted {
				t := now
				eval.PredictedAt = &t
			}
			e.db.Create(&eval)
		}

		if result == models.SLABreached {
			overallSLA = models.SLABreached
			run.SlaBreachReason = rule.Name
			if e.alertService != nil {
				go e.alertService.ProcessSLABreach(&run.Pipeline, &run, &rule, breachSec)
			}
		} else if result == models.SLAPredicted && overallSLA != models.SLABreached {
			overallSLA = models.SLAPredicted
		} else if hasRunning && result == models.SLARunning && overallSLA == models.SLAAchieved {
			overallSLA = models.SLARunning
		}
	}

	run.SlaResult = overallSLA
	e.db.Save(&run)
}

func (e *SLAEngine) matchDateType(dt models.DateType, t time.Time) bool {
	switch dt {
	case models.DateWorkday:
		return utils.IsWorkday(t) && !e.isHoliday(t)
	case models.DateHoliday:
		return !utils.IsWorkday(t) || e.isHoliday(t)
	case models.DateSpecial:
		return e.isSpecialDate(t)
	default:
		return true
	}
}

func (e *SLAEngine) isHoliday(t time.Time) bool {
	date := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	var cnt int64
	e.db.Model(&models.Holiday{}).Where("date = ? AND is_holiday = ?", date, true).Count(&cnt)
	return cnt > 0
}

func (e *SLAEngine) isSpecialDate(t time.Time) bool {
	date := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	var cnt int64
	e.db.Model(&models.Holiday{}).Where("date = ?", date).Count(&cnt)
	return cnt > 0
}

func (e *SLAEngine) GetHistoryStats(pipelineID uint, days int) (p50, p95 int) {
	since := time.Now().AddDate(0, 0, -days)
	var runs []models.PipelineRun
	e.db.Model(&models.PipelineRun{}).
		Where("pipeline_id = ? AND status = ? AND duration_sec > 0 AND created_at >= ?",
			pipelineID, models.RunSuccess, since).
		Pluck("duration_sec", &runs)

	durations := make([]int, 0, len(runs))
	for _, r := range runs {
		durations = append(durations, r.DurationSec)
	}
	return utils.PercentileInt(durations, 50), utils.PercentileInt(durations, 95)
}

func (e *SLAEngine) CountConsecutiveFailures(pipelineID, beforeRunID uint) int {
	var runs []models.PipelineRun
	e.db.Where("pipeline_id = ? AND id <= ?", pipelineID, beforeRunID).
		Order("created_at DESC").Limit(10).Find(&runs)

	count := 0
	for _, r := range runs {
		if r.Status == models.RunFailed || r.Status == models.RunTimeout {
			count++
		} else {
			break
		}
	}
	return count
}

func (e *SLAEngine) GenerateMonthlyReportIfNeeded() {
	now := time.Now()
	firstOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	monthKey := firstOfMonth.Format("2006-01")

	var pipelines []models.Pipeline
	e.db.Find(&pipelines)
	for _, p := range pipelines {
		var existing models.SLAMonthlyReport
		err := e.db.Where("pipeline_id = ? AND report_month = ?", p.ID, monthKey).First(&existing).Error
		if err == nil {
			continue
		}

		nextMonth := firstOfMonth.AddDate(0, 1, 0)
		var runs []models.PipelineRun
		e.db.Where("pipeline_id = ? AND created_at >= ? AND created_at < ?", p.ID, firstOfMonth, nextMonth).Find(&runs)

		report := models.SLAMonthlyReport{
			TenantID:    p.TenantID,
			PipelineID:  p.ID,
			ReportMonth: monthKey,
			TotalRuns:   len(runs),
		}
		durations := make([]int, 0)
		delays := make([]int, 0)
		for _, r := range runs {
			switch r.Status {
			case models.RunSuccess:
				report.SuccessCount++
			case models.RunFailed:
				report.FailedCount++
			case models.RunTimeout:
				report.TimeoutCount++
			}
			if r.SlaResult == models.SLABreached {
				report.BreachCount++
			}
			if r.DurationSec > 0 {
				durations = append(durations, r.DurationSec)
			}
		}
		if report.TotalRuns > 0 {
			report.AchievementRate = utils.RoundFloat(float64(report.TotalRuns-report.BreachCount)/float64(report.TotalRuns)*100, 2)
		}
		report.AvgDurationSec = utils.AverageInt(durations)
		report.P50DurationSec = utils.PercentileInt(durations, 50)
		report.P95DurationSec = utils.PercentileInt(durations, 95)
		report.MaxDurationSec = utils.MaxInt(durations)
		report.AvgDelaySec = utils.AverageInt(delays)
		report.MaxDelaySec = utils.MaxInt(delays)

		e.db.Create(&report)
	}
}

func (e *SLAEngine) GetMonthlyReports(tenantID uint, isSuper bool, pipelineID uint, month string) ([]models.SLAMonthlyReport, error) {
	var reports []models.SLAMonthlyReport
	q := e.db.Model(&models.SLAMonthlyReport{})
	if !isSuper {
		q = q.Where("tenant_id = ?", tenantID)
	}
	if pipelineID > 0 {
		q = q.Where("pipeline_id = ?", pipelineID)
	}
	if month != "" {
		q = q.Where("report_month = ?", month)
	}
	q = q.Preload("Pipeline").Order("report_month DESC")
	err := q.Find(&reports).Error
	return reports, err
}

type SLAStats struct {
	PipelineID      uint    `json:"pipelineId"`
	PipelineName    string  `json:"pipelineName"`
	TotalRuns       int     `json:"totalRuns"`
	SuccessCount    int     `json:"successCount"`
	BreachCount     int     `json:"breachCount"`
	AchievementRate float64 `json:"achievementRate"`
	P50Sec          int     `json:"p50Sec"`
	P95Sec          int     `json:"p95Sec"`
	AvgSec          int     `json:"avgSec"`
}

func (e *SLAEngine) GetStats(tenantID uint, isSuper bool, pipelineID uint, days int) (*SLAStats, error) {
	since := time.Now().AddDate(0, 0, -days)
	var runs []models.PipelineRun
	q := e.db.Where("created_at >= ?", since)
	if !isSuper {
		q = q.Where("tenant_id = ?", tenantID)
	}
	if pipelineID > 0 {
		q = q.Where("pipeline_id = ?", pipelineID)
	}
	q.Find(&runs)

	stats := &SLAStats{PipelineID: pipelineID}
	stats.TotalRuns = len(runs)
	durations := make([]int, 0)
	for _, r := range runs {
		if r.Status == models.RunSuccess {
			stats.SuccessCount++
		}
		if r.SlaResult == models.SLABreached {
			stats.BreachCount++
		}
		if r.DurationSec > 0 {
			durations = append(durations, r.DurationSec)
		}
	}
	if stats.TotalRuns > 0 {
		stats.AchievementRate = utils.RoundFloat(float64(stats.TotalRuns-stats.BreachCount)/float64(stats.TotalRuns)*100, 2)
	}
	stats.P50Sec = utils.PercentileInt(durations, 50)
	stats.P95Sec = utils.PercentileInt(durations, 95)
	stats.AvgSec = utils.AverageInt(durations)

	var p models.Pipeline
	if pipelineID > 0 {
		e.db.Select("name").First(&p, pipelineID)
		stats.PipelineName = p.Name
	}
	return stats, nil
}

var _ = config.Config{}
