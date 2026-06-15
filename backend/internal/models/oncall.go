package models

import (
	"time"
)

type RotationMode string

const (
	RotationWeekly  RotationMode = "weekly"
	RotationDaily   RotationMode = "daily"
	RotationCustom  RotationMode = "custom"
)

type OnCallGroup struct {
	ID           uint         `gorm:"primaryKey" json:"id"`
	TenantID     uint         `gorm:"not null;index" json:"tenantId"`
	Name         string       `gorm:"size:200;not null" json:"name"`
	Description  string       `gorm:"type:text" json:"description"`
	RotationMode RotationMode `gorm:"size:20;not null;default:weekly" json:"rotationMode"`
	Timezone     string       `gorm:"size:50;default:Asia/Shanghai" json:"timezone"`
	StartDate    time.Time    `json:"startDate"`
	Members      string       `gorm:"type:json;not null" json:"members"`
	CreatedAt    time.Time    `json:"createdAt"`
	UpdatedAt    time.Time    `json:"updatedAt"`
}

type OnCallAssignment struct {
	ID          uint        `gorm:"primaryKey" json:"id"`
	TenantID    uint        `gorm:"not null;index" json:"tenantId"`
	GroupID     uint        `gorm:"not null;index:idx_occ_group" json:"groupId"`
	Group       OnCallGroup `gorm:"foreignKey:GroupID" json:"-"`
	PipelineID  *uint       `gorm:"index:idx_occ_pipe" json:"pipelineId"`
	UserID      uint        `gorm:"not null;index:idx_occ_user" json:"userId"`
	User        User        `gorm:"foreignKey:UserID" json:"user,omitempty"`
	StartDate   time.Time   `gorm:"index:idx_occ_date" json:"startDate"`
	EndDate     time.Time   `json:"endDate"`
	ShiftType   string      `gorm:"size:20;default:primary" json:"shiftType"`
	IsBackup    bool        `gorm:"default:false" json:"isBackup"`
	HandoverNote string     `gorm:"type:text" json:"handoverNote"`
	CreatedAt   time.Time   `json:"createdAt"`
}

type HandoverSummary struct {
	ID                    uint      `gorm:"primaryKey" json:"id"`
	TenantID              uint      `gorm:"not null;index" json:"tenantId"`
	GroupID               uint      `gorm:"not null;index" json:"groupId"`
	FromUserID            uint      `gorm:"not null" json:"fromUserId"`
	FromUser              User      `gorm:"foreignKey:FromUserID" json:"-"`
	ToUserID              uint      `gorm:"not null" json:"toUserId"`
	ToUser                User      `gorm:"foreignKey:ToUserID" json:"-"`
	HandoverTime          time.Time `json:"handoverTime"`
	StartDate             time.Time `json:"startDate"`
	EndDate               time.Time `json:"endDate"`
	OpenAlertsCount       int       `json:"openAlertsCount"`
	OpenAlerts            string    `gorm:"type:json" json:"openAlerts"`
	PipelineHealthSummary string    `gorm:"type:json" json:"pipelineHealthSummary"`
	RecentTrendChanges    string    `gorm:"type:json" json:"recentTrendChanges"`
	Notes                 string    `gorm:"type:text" json:"notes"`
	CreatedAt             time.Time `json:"createdAt"`
}
