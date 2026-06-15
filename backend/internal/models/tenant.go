package models

import (
	"time"
)

type Tenant struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"size:100;not null;uniqueIndex" json:"name"`
	DisplayName string    `gorm:"size:200" json:"displayName"`
	Description string    `gorm:"type:text" json:"description"`
	Status      string    `gorm:"size:20;default:active" json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type UserRole string

const (
	RoleSuperAdmin UserRole = "super_admin"
	RoleAdmin      UserRole = "admin"
	RoleMember     UserRole = "member"
)

type User struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	TenantID     uint       `gorm:"not null;index" json:"tenantId"`
	Tenant       Tenant     `gorm:"foreignKey:TenantID" json:"-"`
	Username     string     `gorm:"size:100;not null;uniqueIndex" json:"username"`
	Email        string     `gorm:"size:200;uniqueIndex" json:"email"`
	PasswordHash string     `gorm:"size:500;not null" json:"-"`
	FullName     string     `gorm:"size:200" json:"fullName"`
	Phone        string     `gorm:"size:50" json:"phone"`
	Role         UserRole   `gorm:"size:20;not null;default:member" json:"role"`
	Status       string     `gorm:"size:20;default:active" json:"status"`
	LastLoginAt  *time.Time `json:"lastLoginAt"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}
