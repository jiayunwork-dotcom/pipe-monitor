package models

import (
	"time"
)

type RunStatus string

const (
	RunPending   RunStatus = "pending"
	RunRunning   RunStatus = "running"
	RunSuccess   RunStatus = "success"
	RunFailed    RunStatus = "failed"
	RunTimeout   RunStatus = "timeout"
	RunCancelled RunStatus = "cancelled"
	RunSkipped   RunStatus = "skipped"
)

type SLAResult string

const (
	SLAUnknown   SLAResult = "unknown"
	SLAAchieved  SLAResult = "achieved"
	SLARunning   SLAResult = "running"
	SLAPredicted SLAResult = "predicted_breach"
	SLABreached  SLAResult = "breached"
)

type PipelineRun struct {
	ID              uint           `gorm:"primaryKey" json:"id"`
	TenantID        uint           `gorm:"not null;index" json:"tenantId"`
	PipelineID      uint           `gorm:"not null;index:idx_run_pipeline" json:"pipelineId"`
	Pipeline        Pipeline       `gorm:"foreignKey:PipelineID" json:"pipeline,omitempty"`
	RunID           string         `gorm:"size:100;uniqueIndex" json:"runId"`
	TriggerType     string         `gorm:"size:50;default:scheduled" json:"triggerType"`
	TriggeredBy     string         `gorm:"size:200" json:"triggeredBy"`
	ScheduledStart  *time.Time     `json:"scheduledStart"`
	ActualStart     *time.Time     `gorm:"index:idx_run_actual_start" json:"actualStart"`
	ActualEnd       *time.Time     `json:"actualEnd"`
	DurationSec     int            `json:"durationSec"`
	Status          RunStatus      `gorm:"size:20;not null;default:pending;index:idx_run_status" json:"status"`
	HealthStatus    string         `gorm:"size:20;default:gray" json:"healthStatus"`
	AttemptCount    int            `gorm:"default:1" json:"attemptCount"`
	MaxAttempts     int            `gorm:"default:3" json:"maxAttempts"`
	SlaResult       SLAResult      `gorm:"size:20;default:unknown;index:idx_run_sla" json:"slaResult"`
	SlaBreachReason string         `gorm:"type:text" json:"slaBreachReason"`
	RootCauseType   string         `gorm:"size:50" json:"rootCauseType"`
	RootCauseDetail string         `gorm:"type:text" json:"rootCauseDetail"`
	DataVolume      int64          `json:"dataVolume"`
	ErrorMessage    string         `gorm:"type:text" json:"errorMessage"`
	ErrorStack      string         `gorm:"type:text" json:"errorStack"`
	LogsURL         string         `gorm:"size:500" json:"logsUrl"`
	ExternalURL     string         `gorm:"size:500" json:"externalUrl"`
	CreatedAt       time.Time      `json:"createdAt"`
	UpdatedAt       time.Time      `json:"updatedAt"`
}
