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

type LineageNodeType string

const (
	LineageNodePipeline LineageNodeType = "pipeline"
	LineageNodeExternal LineageNodeType = "external"
)

type LineageDependencyType string

const (
	LineageDepHard LineageDependencyType = "hard"
	LineageDepSoft LineageDependencyType = "soft"
)

type LineageEdge struct {
	ID                 uint                  `gorm:"primaryKey" json:"id"`
	TenantID           uint                  `gorm:"not null;index:idx_lineage_tenant" json:"tenantId"`
	PipelineID         uint                  `gorm:"not null;index:idx_lineage_pipe" json:"pipelineId"`
	Pipeline           Pipeline              `gorm:"foreignKey:PipelineID" json:"-"`
	UpstreamType       LineageNodeType       `gorm:"size:20;not null;default:pipeline" json:"upstreamType"`
	UpstreamPipelineID *uint                 `gorm:"index:idx_lineage_upstream_pipe" json:"upstreamPipelineId"`
	UpstreamPipeline   *Pipeline             `gorm:"foreignKey:UpstreamPipelineID" json:"upstreamPipeline,omitempty"`
	UpstreamExternal   string                `gorm:"size:200" json:"upstreamExternal"`
	DownstreamType     LineageNodeType       `gorm:"size:20;not null;default:pipeline" json:"downstreamType"`
	DownstreamPipelineID *uint               `gorm:"index:idx_lineage_downstream_pipe" json:"downstreamPipelineId"`
	DownstreamPipeline *Pipeline             `gorm:"foreignKey:DownstreamPipelineID" json:"downstreamPipeline,omitempty"`
	DownstreamExternal string                `gorm:"size:200" json:"downstreamExternal"`
	DependencyType     LineageDependencyType `gorm:"size:20;default:hard" json:"dependencyType"`
	EdgeDirection      string                `gorm:"size:10;not null;default:upstream" json:"edgeDirection"`
	Description        string                `gorm:"size:500" json:"description"`
	CreatedAt          time.Time             `json:"createdAt"`
	CreatedBy          uint                  `gorm:"not null" json:"createdBy"`
}

type LineageAuditLog struct {
	ID           uint              `gorm:"primaryKey" json:"id"`
	TenantID     uint              `gorm:"not null;index:idx_lineage_audit_tenant" json:"tenantId"`
	PipelineID   uint              `gorm:"not null;index:idx_lineage_audit_pipe" json:"pipelineId"`
	Pipeline     Pipeline          `gorm:"foreignKey:PipelineID" json:"-"`
	UserID       uint              `gorm:"not null" json:"userId"`
	User         User              `gorm:"foreignKey:UserID" json:"user,omitempty"`
	ActionType   string            `gorm:"size:20;not null;index:idx_lineage_audit_action" json:"actionType"`
	EdgeID       *uint             `json:"edgeId"`
	EdgeInfo     utils.JSONString  `gorm:"type:json" json:"edgeInfo"`
	ChangeDetail utils.JSONString  `gorm:"type:json" json:"changeDetail"`
	IPAddress    string            `gorm:"size:50" json:"ipAddress"`
	CreatedAt    time.Time         `gorm:"index:idx_lineage_audit_time" json:"createdAt"`
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

type LineageSnapshot struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	TenantID    uint           `gorm:"not null;index:idx_snapshot_tenant" json:"tenantId"`
	Name        string         `gorm:"size:200;not null" json:"name"`
	Description string         `gorm:"size:500" json:"description"`
	SnapshotData utils.JSONString `gorm:"type:json" json:"snapshotData"`
	CreatedBy   uint           `gorm:"not null" json:"createdBy"`
	User        *User          `gorm:"foreignKey:CreatedBy" json:"user,omitempty"`
	CreatedAt   time.Time      `json:"createdAt"`
}
