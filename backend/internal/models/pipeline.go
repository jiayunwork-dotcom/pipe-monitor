package models

import (
	"pipe-monitor/internal/utils"
	"time"
)

type DataSource struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	TenantID    uint      `gorm:"not null;index" json:"tenantId"`
	Name        string    `gorm:"size:200;not null" json:"name"`
	Type        string    `gorm:"size:50;not null" json:"type"`
	Host        string    `gorm:"size:500" json:"host"`
	Port        int       `json:"port"`
	Database    string    `gorm:"size:200" json:"database"`
	Table       string    `gorm:"size:200" json:"table"`
	Description string    `gorm:"type:text" json:"description"`
	Config      utils.JSONString `gorm:"type:json" json:"config"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type ScheduleFreq string

const (
	ScheduleHourly   ScheduleFreq = "hourly"
	ScheduleDaily    ScheduleFreq = "daily"
	ScheduleWeekly   ScheduleFreq = "weekly"
	ScheduleMonthly  ScheduleFreq = "monthly"
	ScheduleCustom   ScheduleFreq = "custom"
)

type PipelineStatus string

const (
	PipelineActive   PipelineStatus = "active"
	PipelinePaused   PipelineStatus = "paused"
	PipelineArchived PipelineStatus = "archived"
)

type Pipeline struct {
	ID              uint           `gorm:"primaryKey" json:"id"`
	TenantID        uint           `gorm:"not null;index:idx_pipeline_tenant" json:"tenantId"`
	Name            string         `gorm:"size:200;not null" json:"name"`
	Code            string         `gorm:"size:100;not null;uniqueIndex" json:"code"`
	Description     string         `gorm:"type:text" json:"description"`
	DataDomain      string         `gorm:"size:100;index:idx_pipeline_domain" json:"dataDomain"`
	SourceID        *uint          `json:"sourceId"`
	Source          *DataSource    `gorm:"foreignKey:SourceID" json:"source,omitempty"`
	SourceDetail    string         `gorm:"size:500" json:"sourceDetail"`
	TargetID        *uint          `json:"targetId"`
	Target          *DataSource    `gorm:"foreignKey:TargetID" json:"target,omitempty"`
	TargetDetail    string         `gorm:"size:500" json:"targetDetail"`
	ScheduleFreq    ScheduleFreq   `gorm:"size:20;not null;default:daily;index:idx_pipeline_freq" json:"scheduleFreq"`
	CronExpression  string         `gorm:"size:100" json:"cronExpression"`
	OwnerID         uint           `gorm:"not null" json:"ownerId"`
	Owner           User           `gorm:"foreignKey:OwnerID" json:"owner,omitempty"`
	Team            string         `gorm:"size:100;index:idx_pipeline_team" json:"team"`
	Status          PipelineStatus `gorm:"size:20;not null;default:active;index:idx_pipeline_status" json:"status"`
	Tags            utils.JSONString `gorm:"type:json" json:"tags"`
	WebhookToken    string         `gorm:"size:100" json:"-"`
	ExpectedRunSec  int            `gorm:"default:0" json:"expectedRunSec"`
	CreatedAt       time.Time      `json:"createdAt"`
	UpdatedAt       time.Time      `json:"updatedAt"`
}

type PipelineDependency struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	TenantID      uint      `gorm:"not null;index" json:"tenantId"`
	PipelineID    uint      `gorm:"not null;index:idx_dep_pipe" json:"pipelineId"`
	Pipeline      Pipeline  `gorm:"foreignKey:PipelineID" json:"-"`
	UpstreamID    uint      `gorm:"not null;index:idx_dep_upstream" json:"upstreamId"`
	Upstream      Pipeline  `gorm:"foreignKey:UpstreamID" json:"upstream,omitempty"`
	DependencyType string   `gorm:"size:50;default:hard" json:"dependencyType"`
	TimeOffsetSec int       `gorm:"default:0" json:"timeOffsetSec"`
	CreatedAt     time.Time `json:"createdAt"`
}
