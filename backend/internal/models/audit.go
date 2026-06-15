package models

import (
	"time"
)

type AuditLog struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	TenantID     uint      `gorm:"not null;index" json:"tenantId"`
	UserID       *uint     `gorm:"index" json:"userId"`
	User         *User     `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Action       string    `gorm:"size:100;not null;index" json:"action"`
	ResourceType string    `gorm:"size:50;not null" json:"resourceType"`
	ResourceID   uint      `json:"resourceId"`
	ResourceName string    `gorm:"size:500" json:"resourceName"`
	OldValue     string    `gorm:"type:json" json:"oldValue"`
	NewValue     string    `gorm:"type:json" json:"newValue"`
	IPAddress    string    `gorm:"size:50" json:"ipAddress"`
	UserAgent    string    `gorm:"size:500" json:"userAgent"`
	CreatedAt    time.Time `gorm:"index" json:"createdAt"`
}

type Holiday struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	TenantID  uint      `gorm:"not null;index:idx_holiday_tenant" json:"tenantId"`
	Date      time.Time `gorm:"uniqueIndex:idx_holiday_date;type:date" json:"date"`
	Name      string    `gorm:"size:200" json:"name"`
	IsHoliday bool      `gorm:"default:true;index:idx_holiday_type" json:"isHoliday"`
	CreatedAt time.Time `json:"createdAt"`
}

type ApiToken struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	TenantID    uint      `gorm:"not null;index" json:"tenantId"`
	UserID      uint      `gorm:"not null" json:"userId"`
	Token       string    `gorm:"size:200;uniqueIndex;not null" json:"-"`
	Name        string    `gorm:"size:200;not null" json:"name"`
	Scopes      string    `gorm:"type:json" json:"scopes"`
	LastUsedAt  *time.Time `json:"lastUsedAt"`
	ExpiresAt   *time.Time `json:"expiresAt"`
	CreatedAt   time.Time `json:"createdAt"`
}
