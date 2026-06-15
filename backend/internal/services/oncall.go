package services

import (
	"encoding/json"
	"pipe-monitor/internal/models"
	"time"

	"gorm.io/gorm"
)

type OnCallService struct {
	db *gorm.DB
}

func NewOnCallService(db *gorm.DB) *OnCallService {
	return &OnCallService{db: db}
}

type CreateGroupReq struct {
	TenantID     uint
	Name         string
	Description  string
	RotationMode models.RotationMode
	Timezone     string
	StartDate    time.Time
	MemberIDs    []uint
}

func (s *OnCallService) CreateGroup(req *CreateGroupReq) (*models.OnCallGroup, error) {
	if req.Timezone == "" {
		req.Timezone = "Asia/Shanghai"
	}
	group := &models.OnCallGroup{
		TenantID:     req.TenantID,
		Name:         req.Name,
		Description:  req.Description,
		RotationMode: req.RotationMode,
		Timezone:     req.Timezone,
		StartDate:    req.StartDate,
		Members:      toJSONStr(req.MemberIDs),
	}
	if err := s.db.Create(group).Error; err != nil {
		return nil, err
	}
	if err := s.generateAssignments(group); err != nil {
		return nil, err
	}
	return group, nil
}

func (s *OnCallService) generateAssignments(group *models.OnCallGroup) error {
	var memberIDs []uint
	json.Unmarshal([]byte(group.Members), &memberIDs)
	if len(memberIDs) == 0 {
		return nil
	}

	loc, _ := time.LoadLocation(group.Timezone)
	start := group.StartDate.In(loc)
	idx := 0

	nextYear := start.AddDate(1, 0, 0)
	cycleDate := start
	for cycleDate.Before(nextYear) {
		var end time.Time
		var days int
		switch group.RotationMode {
		case models.RotationDaily:
			days = 1
		default:
			days = 7
		}
		end = cycleDate.AddDate(0, 0, days)

		uid := memberIDs[idx%len(memberIDs)]
		occ := models.OnCallAssignment{
			TenantID:  group.TenantID,
			GroupID:   group.ID,
			UserID:    uid,
			StartDate: cycleDate,
			EndDate:   end,
			ShiftType: "primary",
		}
		s.db.Create(&occ)

		if idx+1 < len(memberIDs) {
			bid := memberIDs[(idx+1)%len(memberIDs)]
			backup := models.OnCallAssignment{
				TenantID:  group.TenantID,
				GroupID:   group.ID,
				UserID:    bid,
				StartDate: cycleDate,
				EndDate:   end,
				ShiftType: "backup",
				IsBackup:  true,
			}
			s.db.Create(&backup)
		}

		cycleDate = end
		idx++
	}
	return nil
}

func (s *OnCallService) GetGroups(tenantID uint, isSuper bool) ([]models.OnCallGroup, error) {
	var groups []models.OnCallGroup
	q := s.db
	if !isSuper {
		q = q.Where("tenant_id = ?", tenantID)
	}
	err := q.Order("created_at DESC").Find(&groups).Error
	return groups, err
}

func (s *OnCallService) GetCurrentAssignment(groupID uint, pipelineID *uint) (*models.OnCallAssignment, error) {
	now := time.Now()
	var occ models.OnCallAssignment
	q := s.db.Where("group_id = ? AND start_date <= ? AND end_date >= ?", groupID, now, now)
	if pipelineID != nil {
		q = q.Where("(pipeline_id = ? OR pipeline_id IS NULL)", *pipelineID)
	}
	q = q.Preload("User")
	err := q.Order("is_backup ASC").First(&occ).Error
	return &occ, err
}

func (s *OnCallService) GetAssignments(groupID uint, days int) ([]models.OnCallAssignment, error) {
	var occs []models.OnCallAssignment
	since := time.Now().AddDate(0, 0, -days)
	until := time.Now().AddDate(0, 0, days)
	err := s.db.Preload("User").
		Where("group_id = ? AND start_date >= ? AND end_date <= ?", groupID, since, until).
		Order("start_date ASC").Find(&occs).Error
	return occs, err
}

type HandoverData struct {
	OpenAlerts       []models.AlertEvent `json:"openAlerts"`
	PipelineHealth   map[string]int      `json:"pipelineHealth"`
	RecentFailures   []models.PipelineRun `json:"recentFailures"`
	TrendChanges     map[string]string   `json:"trendChanges"`
}

func (s *OnCallService) CreateHandover(groupID, fromUserID, toUserID uint, notes string) (*models.HandoverSummary, error) {
	var group models.OnCallGroup
	if err := s.db.First(&group, groupID).Error; err != nil {
		return nil, err
	}

	now := time.Now()
	var cur models.OnCallAssignment
	s.db.Where("group_id = ? AND user_id = ? AND start_date <= ? AND end_date >= ?",
		groupID, fromUserID, now, now).Order("is_backup ASC").First(&cur)

	var next models.OnCallAssignment
	s.db.Where("group_id = ? AND user_id = ? AND start_date >= ?", groupID, toUserID, now).
		Order("start_date ASC").First(&next)

	var openAlerts []models.AlertEvent
	s.db.Where("tenant_id = ? AND status IN ?", group.TenantID,
		[]string{string(models.AlertTriggered), string(models.AlertAcknowledged)}).
		Order("triggered_at DESC").Find(&openAlerts)

	var recentFailures []models.PipelineRun
	since := now.AddDate(0, 0, -7)
	s.db.Where("tenant_id = ? AND status IN ? AND created_at >= ?", group.TenantID,
		[]string{string(models.RunFailed), string(models.RunTimeout)}, since).
		Preload("Pipeline").Order("created_at DESC").Limit(20).Find(&recentFailures)

	health := make(map[string]int)
	statuses := []string{"green", "yellow", "red", "gray"}
	for _, st := range statuses {
		var cnt int64
		subQ := s.db.Table("pipeline_runs pr").
			Select("DISTINCT ON (pipeline_id) pipeline_id, health_status").
			Where("tenant_id = ?", group.TenantID).
			Order("pipeline_id, created_at DESC")
		s.db.Table("(?) AS sub", subQ).Where("health_status = ?", st).Count(&cnt)
		health[st] = int(cnt)
	}

	trend := make(map[string]string)
	trend["thisWeekFails"] = "需计算"
	trend["lastWeekFails"] = "需计算"

	handover := &models.HandoverSummary{
		TenantID:              group.TenantID,
		GroupID:               groupID,
		FromUserID:            fromUserID,
		ToUserID:              toUserID,
		HandoverTime:          now,
		StartDate:             cur.StartDate,
		EndDate:               next.EndDate,
		OpenAlertsCount:       len(openAlerts),
		OpenAlerts:            toJSONStr(openAlerts),
		PipelineHealthSummary: toJSONStr(health),
		RecentTrendChanges:    toJSONStr(trend),
		Notes:                 notes,
	}
	if err := s.db.Create(handover).Error; err != nil {
		return nil, err
	}
	return handover, nil
}

func (s *OnCallService) GetHandovers(groupID uint, limit int) ([]models.HandoverSummary, error) {
	var list []models.HandoverSummary
	err := s.db.Preload("FromUser").Preload("ToUser").
		Where("group_id = ?", groupID).
		Order("handover_time DESC").Limit(limit).Find(&list).Error
	return list, err
}

func toJSONStr(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}
