package models

import (
	"time"
)

type DateType string

const (
	DateWorkday    DateType = "workday"
	DateHoliday    DateType = "holiday"
	DateSpecial    DateType = "special"
	DateAny        DateType = "any"
)

type SLARuleType string

const (
	SLAFinishByTime   SLARuleType = "finish_by_time"
	SLAMaxDuration    SLARuleType = "max_duration"
	SLAMaxDelay       SLARuleType = "max_delay"
	SLAMaxConsecFail  SLARuleType = "max_consecutive_fail"
	SLAMinSuccessRate SLARuleType = "min_success_rate"
)

type SLARule struct {
	ID                 uint       `gorm:"primaryKey" json:"id"`
	TenantID           uint       `gorm:"not null;index" json:"tenantId"`
	PipelineID         uint       `gorm:"not null;index:idx_sla_pipeline" json:"pipelineId"`
	Pipeline           Pipeline   `gorm:"foreignKey:PipelineID" json:"-"`
	Name               string     `gorm:"size:200;not null" json:"name"`
	RuleType           SLARuleType `gorm:"size:50;not null" json:"ruleType"`
	DateType           DateType   `gorm:"size:20;not null;default:any;index:idx_sla_date" json:"dateType"`
	FinishDeadlineTime string     `gorm:"size:10" json:"finishDeadlineTime"`
	MaxDurationSec     int        `json:"maxDurationSec"`
	MaxDelaySec        int        `json:"maxDelaySec"`
	MaxConsecutiveFail int        `json:"maxConsecutiveFail"`
	MinSuccessRate     float64    `json:"minSuccessRate"`
	SuccessRateWindow  int        `gorm:"default:30" json:"successRateWindow"`
	WarnThresholdSec   int        `json:"warnThresholdSec"`
	AlertChannels      string     `gorm:"type:json;default:'[]'" json:"alertChannels"`
	AlertSeverity      string     `gorm:"size:20;default:warning" json:"alertSeverity"`
	Enabled            bool       `gorm:"default:true" json:"enabled"`
	Description        string     `gorm:"type:text" json:"description"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
}

type SLAEvaluation struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	TenantID       uint       `gorm:"not null;index" json:"tenantId"`
	RunID          uint       `gorm:"not null;index:idx_sla_eval_run" json:"runId"`
	Run            PipelineRun `gorm:"foreignKey:RunID" json:"-"`
	RuleID         uint       `gorm:"not null;index:idx_sla_eval_rule" json:"ruleId"`
	Rule           SLARule    `gorm:"foreignKey:RuleID" json:"rule,omitempty"`
	Result         SLAResult  `gorm:"size:20;not null" json:"result"`
	ActualValue    float64    `json:"actualValue"`
	ThresholdValue float64    `json:"thresholdValue"`
	BreachSec      int        `json:"breachSec"`
	EvaluatedAt    time.Time  `json:"evaluatedAt"`
	PredictedAt    *time.Time `json:"predictedAt"`
	CreatedAt      time.Time  `json:"createdAt"`
}

type SLAMonthlyReport struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	TenantID        uint      `gorm:"not null;index" json:"tenantId"`
	PipelineID      uint      `gorm:"not null;index:idx_sla_rep_pipe" json:"pipelineId"`
	ReportMonth     string    `gorm:"size:7;not null;index:idx_sla_rep_month" json:"reportMonth"`
	TotalRuns       int       `json:"totalRuns"`
	SuccessCount    int       `json:"successCount"`
	FailedCount     int       `json:"failedCount"`
	TimeoutCount    int       `json:"timeoutCount"`
	BreachCount     int       `json:"breachCount"`
	AchievementRate float64   `json:"achievementRate"`
	AvgDurationSec  int       `json:"avgDurationSec"`
	P50DurationSec  int       `json:"p50DurationSec"`
	P95DurationSec  int       `json:"p95DurationSec"`
	MaxDurationSec  int       `json:"maxDurationSec"`
	AvgDelaySec     int       `json:"avgDelaySec"`
	MaxDelaySec     int       `json:"maxDelaySec"`
	RootCauseSummary string   `gorm:"type:text" json:"rootCauseSummary"`
	CreatedAt       time.Time `json:"createdAt"`
}
