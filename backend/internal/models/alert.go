package models

import (
	"time"
)

type AlertSeverity string

const (
	AlertInfo     AlertSeverity = "info"
	AlertWarning  AlertSeverity = "warning"
	AlertCritical AlertSeverity = "critical"
)

type AlertStatus string

const (
	AlertTriggered AlertStatus = "triggered"
	AlertAcknowledged AlertStatus = "acknowledged"
	AlertResolved  AlertStatus = "resolved"
	AlertClosed    AlertStatus = "closed"
	AlertSuppressed AlertStatus = "suppressed"
)

type AlertRuleType string

const (
	AlertConsecutiveFail   AlertRuleType = "consecutive_fail"
	AlertDurationP95       AlertRuleType = "duration_over_p95"
	AlertSLAImminent       AlertRuleType = "sla_imminent"
	AlertSLABreached       AlertRuleType = "sla_breached"
	AlertDataDelay         AlertRuleType = "data_delay"
	AlertPipelineDown      AlertRuleType = "pipeline_down"
	AlertCustomCondition   AlertRuleType = "custom"
)

type AlertChannelType string

const (
	AlertWebhookFeishu   AlertChannelType = "feishu"
	AlertWebhookDingTalk AlertChannelType = "dingtalk"
	AlertWebhookSlack    AlertChannelType = "slack"
	AlertWebhookCustom   AlertChannelType = "custom_webhook"
	AlertEmail           AlertChannelType = "email"
	AlertSMS             AlertChannelType = "sms"
)

type AlertRule struct {
	ID               uint            `gorm:"primaryKey" json:"id"`
	TenantID         uint            `gorm:"not null;index" json:"tenantId"`
	PipelineID       *uint           `gorm:"index:idx_alert_rule_pipe" json:"pipelineId"`
	Name             string          `gorm:"size:200;not null" json:"name"`
	RuleType         AlertRuleType   `gorm:"size:50;not null" json:"ruleType"`
	Conditions       string          `gorm:"type:json" json:"conditions"`
	ConsecutiveFailN int             `gorm:"default:3" json:"consecutiveFailN"`
	DurationP95Multi float64         `gorm:"default:2.0" json:"durationP95Multi"`
	SlaWarnMinBefore int             `gorm:"default:15" json:"slaWarnMinBefore"`
	DataDelaySec     int             `gorm:"default:1800" json:"dataDelaySec"`
	Severity         AlertSeverity   `gorm:"size:20;not null;default:warning" json:"severity"`
	Channels         string          `gorm:"type:json;not null" json:"channels"`
	CustomWebhookURL string          `gorm:"size:500" json:"customWebhookUrl"`
	EmailRecipients  string          `gorm:"type:json" json:"emailRecipients"`
	NotifyOnCall     bool            `gorm:"default:true" json:"notifyOnCall"`
	SuppressWindowMin int            `gorm:"default:60" json:"suppressWindowMin"`
	Enabled          bool            `gorm:"default:true" json:"enabled"`
	Description      string          `gorm:"type:text" json:"description"`
	EscalationEnabled bool           `gorm:"default:false" json:"escalationEnabled"`
	EscalationAfterMin int           `gorm:"default:15" json:"escalationAfterMin"`
	EscalationToSeverity AlertSeverity `gorm:"size:20;default:critical" json:"escalationToSeverity"`
	SilentEnabled    bool            `gorm:"default:false" json:"silentEnabled"`
	SilentStart      string          `gorm:"size:10;default:02:00" json:"silentStart"`
	SilentEnd        string          `gorm:"size:10;default:06:00" json:"silentEnd"`
	SilentSummaryEnabled bool        `gorm:"default:true" json:"silentSummaryEnabled"`
	CreatedAt        time.Time       `json:"createdAt"`
	UpdatedAt        time.Time       `json:"updatedAt"`
}

type AlertEvent struct {
	ID                 uint           `gorm:"primaryKey" json:"id"`
	TenantID           uint           `gorm:"not null;index" json:"tenantId"`
	RuleID             uint           `gorm:"index:idx_alert_ev_rule" json:"ruleId"`
	Rule               *AlertRule     `gorm:"foreignKey:RuleID" json:"rule,omitempty"`
	PipelineID         *uint          `gorm:"index:idx_alert_ev_pipe" json:"pipelineId"`
	Pipeline           *Pipeline      `gorm:"foreignKey:PipelineID" json:"pipeline,omitempty"`
	RunID              *uint          `json:"runId"`
	Fingerprint        string         `gorm:"size:100;index:idx_alert_fp" json:"fingerprint"`
	Severity           AlertSeverity  `gorm:"size:20;not null" json:"severity"`
	Status             AlertStatus    `gorm:"size:20;not null;default:triggered;index:idx_alert_status" json:"status"`
	Title              string         `gorm:"size:500;not null" json:"title"`
	Message            string         `gorm:"type:text" json:"message"`
	Detail             string         `gorm:"type:json" json:"detail"`
	TriggeredAt        time.Time      `gorm:"index:idx_alert_trigger" json:"triggeredAt"`
	FirstTriggeredAt   time.Time      `json:"firstTriggeredAt"`
	AcknowledgedAt     *time.Time     `json:"acknowledgedAt"`
	AcknowledgedByID   *uint          `json:"acknowledgedById"`
	AcknowledgedBy     *User          `gorm:"foreignKey:AcknowledgedByID" json:"acknowledgedBy,omitempty"`
	AckNote            string         `gorm:"type:text" json:"ackNote"`
	ResolvedAt         *time.Time     `json:"resolvedAt"`
	ResolvedByID       *uint          `json:"resolvedById"`
	ResolvedBy         *User          `gorm:"foreignKey:ResolvedByID" json:"resolvedBy,omitempty"`
	ResolveNote        string         `gorm:"type:text" json:"resolveNote"`
	SuppressedUntil    *time.Time     `json:"suppressedUntil"`
	ChannelsNotified   string         `gorm:"type:json" json:"channelsNotified"`
	NotifyCount        int            `gorm:"default:1" json:"notifyCount"`
	CreatedAt          time.Time      `json:"createdAt"`
	UpdatedAt          time.Time      `json:"updatedAt"`
}

type AlertNotification struct {
	ID        uint            `gorm:"primaryKey" json:"id"`
	AlertID   uint            `gorm:"not null;index" json:"alertId"`
	Alert     AlertEvent      `gorm:"foreignKey:AlertID" json:"-"`
	Channel   AlertChannelType `gorm:"size:30;not null" json:"channel"`
	Recipient string          `gorm:"size:500" json:"recipient"`
	Status    string          `gorm:"size:20;default:sent" json:"status"`
	Error     string          `gorm:"type:text" json:"error"`
	SentAt    time.Time       `json:"sentAt"`
}

type AlertEscalation struct {
	ID             uint           `gorm:"primaryKey" json:"id"`
	AlertID        uint           `gorm:"not null;index" json:"alertId"`
	Alert          AlertEvent     `gorm:"foreignKey:AlertID" json:"-"`
	TenantID       uint           `gorm:"not null;index" json:"tenantId"`
	FromSeverity   AlertSeverity  `gorm:"size:20;not null" json:"fromSeverity"`
	ToSeverity     AlertSeverity  `gorm:"size:20;not null" json:"toSeverity"`
	Reason         string         `gorm:"size:200" json:"reason"`
	TriggeredBy    string         `gorm:"size:50;not null" json:"triggeredBy"`
	TriggeredByID  *uint          `json:"triggeredById"`
	TriggeredAt    time.Time      `gorm:"index" json:"triggeredAt"`
	NotifiedLeader bool           `gorm:"default:false" json:"notifiedLeader"`
	CreatedAt      time.Time      `json:"createdAt"`
}
