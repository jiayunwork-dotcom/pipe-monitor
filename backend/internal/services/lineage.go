package services

import (
	"errors"
	"fmt"
	"pipe-monitor/internal/models"
	"pipe-monitor/internal/utils"
	"time"

	"gorm.io/gorm"
)

type LineageService struct {
	db *gorm.DB
}

func NewLineageService(db *gorm.DB) *LineageService {
	return &LineageService{db: db}
}

type AddLineageEdgeReq struct {
	TenantID             uint
	PipelineID           uint
	UserID               uint
	IPAddress            string
	UpstreamType         models.LineageNodeType
	UpstreamPipelineID   *uint
	UpstreamExternal     string
	DownstreamType       models.LineageNodeType
	DownstreamPipelineID *uint
	DownstreamExternal   string
	DependencyType       models.LineageDependencyType
	Description          string
}

func (s *LineageService) AddEdge(req *AddLineageEdgeReq) (*models.LineageEdge, error) {
	if err := s.validateEdge(req); err != nil {
		return nil, err
	}

	if cyclePath, err := s.DetectCycle(req); err != nil {
		return nil, fmt.Errorf("检测到循环依赖: %s", cyclePath)
	}

	var upstreamEdge *models.LineageEdge
	err := s.db.Transaction(func(tx *gorm.DB) error {
		upstreamEdge = &models.LineageEdge{
			TenantID:             req.TenantID,
			PipelineID:           req.PipelineID,
			UpstreamType:         req.UpstreamType,
			UpstreamPipelineID:   req.UpstreamPipelineID,
			UpstreamExternal:     req.UpstreamExternal,
			DownstreamType:       req.DownstreamType,
			DownstreamPipelineID: req.DownstreamPipelineID,
			DownstreamExternal:   req.DownstreamExternal,
			DependencyType:       req.DependencyType,
			EdgeDirection:        "upstream",
			Description:          req.Description,
			CreatedBy:            req.UserID,
		}

		if err := tx.Create(upstreamEdge).Error; err != nil {
			return err
		}

		downstreamPipelineID := req.PipelineID
		if req.DownstreamType == models.LineageNodePipeline && req.DownstreamPipelineID != nil {
			downstreamPipelineID = *req.DownstreamPipelineID
		}

		if downstreamPipelineID != req.PipelineID && req.DownstreamType == models.LineageNodePipeline {
			downstreamEdge := &models.LineageEdge{
				TenantID:             req.TenantID,
				PipelineID:           downstreamPipelineID,
				UpstreamType:         req.UpstreamType,
				UpstreamPipelineID:   req.UpstreamPipelineID,
				UpstreamExternal:     req.UpstreamExternal,
				DownstreamType:       req.DownstreamType,
				DownstreamPipelineID: req.DownstreamPipelineID,
				DownstreamExternal:   req.DownstreamExternal,
				DependencyType:       req.DependencyType,
				EdgeDirection:        "downstream",
				Description:          req.Description,
				CreatedBy:            req.UserID,
			}
			if err := tx.Create(downstreamEdge).Error; err != nil {
				return err
			}
		}

		edgeInfo := map[string]interface{}{
			"id":                   upstreamEdge.ID,
			"upstreamType":         upstreamEdge.UpstreamType,
			"upstreamPipelineId":   upstreamEdge.UpstreamPipelineID,
			"upstreamExternal":     upstreamEdge.UpstreamExternal,
			"downstreamType":       upstreamEdge.DownstreamType,
			"downstreamPipelineId": upstreamEdge.DownstreamPipelineID,
			"downstreamExternal":   upstreamEdge.DownstreamExternal,
			"dependencyType":       upstreamEdge.DependencyType,
			"description":          upstreamEdge.Description,
		}

		auditLog := &models.LineageAuditLog{
			TenantID:   req.TenantID,
			PipelineID: req.PipelineID,
			UserID:     req.UserID,
			ActionType: "add",
			EdgeID:     &upstreamEdge.ID,
			EdgeInfo:   string(utils.JSONString(utils.ToJSON(edgeInfo))),
			IPAddress:  req.IPAddress,
		}

		return tx.Create(auditLog).Error
	})

	if err != nil {
		return nil, err
	}

	return upstreamEdge, nil
}

func (s *LineageService) validateEdge(req *AddLineageEdgeReq) error {
	if req.UpstreamType == models.LineageNodePipeline {
		if req.UpstreamPipelineID == nil || *req.UpstreamPipelineID == 0 {
			return errors.New("管道类型上游必须指定管道ID")
		}
		if *req.UpstreamPipelineID == req.PipelineID {
			return errors.New("不能将自己作为上游")
		}
	} else {
		if req.UpstreamExternal == "" {
			return errors.New("外部数据源必须填写名称")
		}
	}

	if req.DownstreamType == models.LineageNodePipeline {
		if req.DownstreamPipelineID == nil || *req.DownstreamPipelineID == 0 {
			return errors.New("管道类型下游必须指定管道ID")
		}
	} else {
		if req.DownstreamExternal == "" {
			return errors.New("外部数据集必须填写名称")
		}
	}

	var count int64
	query := s.db.Model(&models.LineageEdge{}).
		Where("tenant_id = ? AND upstream_type = ? AND downstream_type = ?",
			req.TenantID, req.UpstreamType, req.DownstreamType)

	if req.UpstreamType == models.LineageNodePipeline {
		query = query.Where("upstream_pipeline_id = ?", req.UpstreamPipelineID)
	} else {
		query = query.Where("upstream_external = ?", req.UpstreamExternal)
	}

	if req.DownstreamType == models.LineageNodePipeline {
		query = query.Where("downstream_pipeline_id = ?", req.DownstreamPipelineID)
	} else {
		query = query.Where("downstream_external = ?", req.DownstreamExternal)
	}

	if err := query.Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New("该血缘关系已存在")
	}

	return nil
}

type CycleDetectionRequest struct {
	PipelineID           uint
	UpstreamType         models.LineageNodeType
	UpstreamPipelineID   *uint
	UpstreamExternal     string
	DownstreamType       models.LineageNodeType
	DownstreamPipelineID *uint
	DownstreamExternal   string
}

func (s *LineageService) DetectCycle(req *AddLineageEdgeReq) (string, error) {
	if req.UpstreamType != models.LineageNodePipeline || req.DownstreamType != models.LineageNodePipeline {
		return "", nil
	}

	visited := make(map[uint]bool)
	recStack := make(map[uint]bool)
	path := make([]uint, 0)

	cyclePath := s.dfsFindCycle(*req.DownstreamPipelineID, *req.UpstreamPipelineID, visited, recStack, path)
	if cyclePath != "" {
		return cyclePath, errors.New("cycle detected")
	}

	return "", nil
}

func (s *LineageService) dfsFindCycle(start, target uint, visited, recStack map[uint]bool, path []uint) string {
	visited[start] = true
	recStack[start] = true
	path = append(path, start)

	if start == target {
		path = append(path, path[0])
		return s.formatCyclePath(path)
	}

	var edges []models.LineageEdge
	if err := s.db.Where("pipeline_id = ? AND edge_direction = ? AND downstream_type = ? AND downstream_pipeline_id = ?",
		start, "upstream", models.LineageNodePipeline, start).Find(&edges).Error; err != nil {
		return ""
	}

	for _, edge := range edges {
		if edge.UpstreamType != models.LineageNodePipeline || edge.UpstreamPipelineID == nil {
			continue
		}
		next := *edge.UpstreamPipelineID
		if !visited[next] {
			if cycle := s.dfsFindCycle(next, target, visited, recStack, path); cycle != "" {
				return cycle
			}
		} else if recStack[next] {
			cyclePath := make([]uint, 0)
			found := false
			for _, p := range path {
				if p == next {
					found = true
				}
				if found {
					cyclePath = append(cyclePath, p)
				}
			}
			cyclePath = append(cyclePath, next)
			return s.formatCyclePath(cyclePath)
		}
	}

	recStack[start] = false
	path = path[:len(path)-1]
	return ""
}

func (s *LineageService) formatCyclePath(ids []uint) string {
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		var pipe models.Pipeline
		if err := s.db.Select("id, name, code").Where("id = ?", id).First(&pipe).Error; err == nil {
			names = append(names, fmt.Sprintf("%s(%d)", pipe.Name, pipe.ID))
		} else {
			names = append(names, fmt.Sprintf("ID:%d", id))
		}
	}
	result := ""
	for i, name := range names {
		if i > 0 {
			result += " → "
		}
		result += name
	}
	return result
}

func (s *LineageService) RemoveEdge(tenantID, edgeID, userID uint, ipAddress string) error {
	var edge models.LineageEdge
	if err := s.db.Where("id = ? AND tenant_id = ?", edgeID, tenantID).First(&edge).Error; err != nil {
		return errors.New("血缘边不存在")
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		edgeInfo := map[string]interface{}{
			"id":                   edge.ID,
			"upstreamType":         edge.UpstreamType,
			"upstreamPipelineId":   edge.UpstreamPipelineID,
			"upstreamExternal":     edge.UpstreamExternal,
			"downstreamType":       edge.DownstreamType,
			"downstreamPipelineId": edge.DownstreamPipelineID,
			"downstreamExternal":   edge.DownstreamExternal,
			"dependencyType":       edge.DependencyType,
			"description":          edge.Description,
		}

		auditLog := &models.LineageAuditLog{
			TenantID:   tenantID,
			PipelineID: edge.PipelineID,
			UserID:     userID,
			ActionType: "remove",
			EdgeID:     &edge.ID,
			EdgeInfo:   string(utils.JSONString(utils.ToJSON(edgeInfo))),
			IPAddress:  ipAddress,
		}

		if err := tx.Create(auditLog).Error; err != nil {
			return err
		}

		query := tx.Where("tenant_id = ? AND upstream_type = ? AND downstream_type = ?",
			tenantID, edge.UpstreamType, edge.DownstreamType)

		if edge.UpstreamType == models.LineageNodePipeline {
			query = query.Where("upstream_pipeline_id = ?", edge.UpstreamPipelineID)
		} else {
			query = query.Where("upstream_external = ?", edge.UpstreamExternal)
		}

		if edge.DownstreamType == models.LineageNodePipeline {
			query = query.Where("downstream_pipeline_id = ?", edge.DownstreamPipelineID)
		} else {
			query = query.Where("downstream_external = ?", edge.DownstreamExternal)
		}

		return query.Delete(&models.LineageEdge{}).Error
	})
}

func (s *LineageService) GetPipelineLineage(pipelineID uint) (map[string]interface{}, error) {
	var upstreams []models.LineageEdge
	if err := s.db.Preload("UpstreamPipeline").
		Where("pipeline_id = ? AND edge_direction = ? AND downstream_type = ? AND downstream_pipeline_id = ?",
			pipelineID, "upstream", models.LineageNodePipeline, pipelineID).
		Find(&upstreams).Error; err != nil {
		return nil, err
	}

	var downstreams []models.LineageEdge
	if err := s.db.Preload("DownstreamPipeline").
		Where("pipeline_id = ? AND edge_direction = ? AND upstream_type = ? AND upstream_pipeline_id = ?",
			pipelineID, "downstream", models.LineageNodePipeline, pipelineID).
		Find(&downstreams).Error; err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"upstreams":   upstreams,
		"downstreams": downstreams,
	}, nil
}

func (s *LineageService) GetAuditLogs(pipelineID uint, actionType string, page, pageSize int) (map[string]interface{}, error) {
	query := s.db.Preload("User").Where("pipeline_id = ?", pipelineID)
	if actionType != "" && actionType != "all" {
		query = query.Where("action_type = ?", actionType)
	}

	var total int64
	if err := query.Model(&models.LineageAuditLog{}).Count(&total).Error; err != nil {
		return nil, err
	}

	var logs []models.LineageAuditLog
	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&logs).Error; err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"total": total,
		"page":  page,
		"size":  pageSize,
		"data":  logs,
	}, nil
}

type LineageGraphNode struct {
	ID               string      `json:"id"`
	Type             string      `json:"type"`
	PipelineID       *uint       `json:"pipelineId,omitempty"`
	Name             string      `json:"name"`
	Code             string      `json:"code,omitempty"`
	Team             string      `json:"team,omitempty"`
	LastRunStatus    string      `json:"lastRunStatus,omitempty"`
	LastRunTime      *time.Time  `json:"lastRunTime,omitempty"`
	SLAStatus        string      `json:"slaStatus,omitempty"`
	HasAlert         bool        `json:"hasAlert,omitempty"`
	Level            int         `json:"level"`
	Direction        string      `json:"direction"`
}

type LineageGraphEdge struct {
	ID             uint   `json:"id"`
	Source         string `json:"source"`
	Target         string `json:"target"`
	DependencyType string `json:"dependencyType"`
	Description    string `json:"description"`
}

type LineageGraphResult struct {
	Nodes []LineageGraphNode `json:"nodes"`
	Edges []LineageGraphEdge `json:"edges"`
	CenterPipelineID uint `json:"centerPipelineId"`
}

func (s *LineageService) BuildLineageGraph(pipelineID uint, tenantID uint, isSuper bool, maxDepth int) (*LineageGraphResult, error) {
	if maxDepth <= 0 || maxDepth > 5 {
		maxDepth = 5
	}

	var centerPipe models.Pipeline
	if err := s.db.Where("id = ?", pipelineID).First(&centerPipe).Error; err != nil {
		return nil, errors.New("管道不存在")
	}

	nodeMap := make(map[string]*LineageGraphNode)
	edgeMap := make(map[string]*LineageGraphEdge)

	centerNodeID := fmt.Sprintf("pipe_%d", centerPipe.ID)
	lastRunStatus, lastRunTime, slaStatus, hasAlert := s.getPipelineStatusInfo(centerPipe.ID)
	nodeMap[centerNodeID] = &LineageGraphNode{
		ID:            centerNodeID,
		Type:          "pipeline",
		PipelineID:    &centerPipe.ID,
		Name:          centerPipe.Name,
		Code:          centerPipe.Code,
		Team:          centerPipe.Team,
		LastRunStatus: lastRunStatus,
		LastRunTime:   lastRunTime,
		SLAStatus:     slaStatus,
		HasAlert:      hasAlert,
		Level:         0,
		Direction:     "center",
	}

	s.bfsCollectUpstream(centerPipe.ID, maxDepth, nodeMap, edgeMap)
	s.bfsCollectDownstream(centerPipe.ID, maxDepth, nodeMap, edgeMap)

	nodes := make([]LineageGraphNode, 0, len(nodeMap))
	for _, node := range nodeMap {
		nodes = append(nodes, *node)
	}

	edges := make([]LineageGraphEdge, 0, len(edgeMap))
	for _, edge := range edgeMap {
		edges = append(edges, *edge)
	}

	return &LineageGraphResult{
		Nodes:            nodes,
		Edges:            edges,
		CenterPipelineID: pipelineID,
	}, nil
}

func (s *LineageService) bfsCollectUpstream(startID uint, maxDepth int, nodeMap map[string]*LineageGraphNode, edgeMap map[string]*LineageGraphEdge) {
	type queueItem struct {
		id    uint
		depth int
	}
	queue := []queueItem{{startID, 0}}
	visited := make(map[uint]bool)
	visited[startID] = true

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		if cur.depth >= maxDepth {
			continue
		}

		var edges []models.LineageEdge
		s.db.Where("pipeline_id = ? AND edge_direction = ? AND downstream_type = ? AND downstream_pipeline_id = ?",
			cur.id, "upstream", models.LineageNodePipeline, cur.id).Find(&edges)

		for _, edge := range edges {
			sourceID := ""
			sourceName := ""

			if edge.UpstreamType == models.LineageNodePipeline && edge.UpstreamPipelineID != nil {
				sourceID = fmt.Sprintf("pipe_%d", *edge.UpstreamPipelineID)
				var upPipe models.Pipeline
				if err := s.db.Where("id = ?", *edge.UpstreamPipelineID).First(&upPipe).Error; err == nil {
					sourceName = upPipe.Name
					if _, exists := nodeMap[sourceID]; !exists {
						lastRunStatus, lastRunTime, slaStatus, hasAlert := s.getPipelineStatusInfo(upPipe.ID)
						nodeMap[sourceID] = &LineageGraphNode{
							ID:            sourceID,
							Type:          "pipeline",
							PipelineID:    &upPipe.ID,
							Name:          upPipe.Name,
							Code:          upPipe.Code,
							Team:          upPipe.Team,
							LastRunStatus: lastRunStatus,
							LastRunTime:   lastRunTime,
							SLAStatus:     slaStatus,
							HasAlert:      hasAlert,
							Level:         -(cur.depth + 1),
							Direction:     "upstream",
						}
					}
				}
			} else if edge.UpstreamType == models.LineageNodeExternal {
				sourceID = fmt.Sprintf("ext_up_%s", edge.UpstreamExternal)
				sourceName = edge.UpstreamExternal
				if _, exists := nodeMap[sourceID]; !exists {
					nodeMap[sourceID] = &LineageGraphNode{
						ID:        sourceID,
						Type:      "external",
						Name:      sourceName,
						Level:     -(cur.depth + 1),
						Direction: "upstream",
					}
				}
			}

			if sourceID != "" {
				targetID := fmt.Sprintf("pipe_%d", cur.id)
				edgeKey := fmt.Sprintf("%s_%s", sourceID, targetID)
				if _, exists := edgeMap[edgeKey]; !exists {
					edgeMap[edgeKey] = &LineageGraphEdge{
						ID:             edge.ID,
						Source:         sourceID,
						Target:         targetID,
						DependencyType: string(edge.DependencyType),
						Description:    edge.Description,
					}
				}
			}

			if edge.UpstreamType == models.LineageNodePipeline && edge.UpstreamPipelineID != nil {
				if !visited[*edge.UpstreamPipelineID] {
					visited[*edge.UpstreamPipelineID] = true
					queue = append(queue, queueItem{*edge.UpstreamPipelineID, cur.depth + 1})
				}
			}
		}
	}
}

func (s *LineageService) bfsCollectDownstream(startID uint, maxDepth int, nodeMap map[string]*LineageGraphNode, edgeMap map[string]*LineageGraphEdge) {
	type queueItem struct {
		id    uint
		depth int
	}
	queue := []queueItem{{startID, 0}}
	visited := make(map[uint]bool)
	visited[startID] = true

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		if cur.depth >= maxDepth {
			continue
		}

		var edges []models.LineageEdge
		s.db.Where("pipeline_id = ? AND edge_direction = ? AND upstream_type = ? AND upstream_pipeline_id = ?",
			cur.id, "downstream", models.LineageNodePipeline, cur.id).Find(&edges)

		for _, edge := range edges {
			targetID := ""
			targetName := ""

			if edge.DownstreamType == models.LineageNodePipeline && edge.DownstreamPipelineID != nil {
				targetID = fmt.Sprintf("pipe_%d", *edge.DownstreamPipelineID)
				var downPipe models.Pipeline
				if err := s.db.Where("id = ?", *edge.DownstreamPipelineID).First(&downPipe).Error; err == nil {
					targetName = downPipe.Name
					if _, exists := nodeMap[targetID]; !exists {
						lastRunStatus, lastRunTime, slaStatus, hasAlert := s.getPipelineStatusInfo(downPipe.ID)
						nodeMap[targetID] = &LineageGraphNode{
							ID:            targetID,
							Type:          "pipeline",
							PipelineID:    &downPipe.ID,
							Name:          downPipe.Name,
							Code:          downPipe.Code,
							Team:          downPipe.Team,
							LastRunStatus: lastRunStatus,
							LastRunTime:   lastRunTime,
							SLAStatus:     slaStatus,
							HasAlert:      hasAlert,
							Level:         cur.depth + 1,
							Direction:     "downstream",
						}
					}
				}
			} else if edge.DownstreamType == models.LineageNodeExternal {
				targetID = fmt.Sprintf("ext_down_%s", edge.DownstreamExternal)
				targetName = edge.DownstreamExternal
				if _, exists := nodeMap[targetID]; !exists {
					nodeMap[targetID] = &LineageGraphNode{
						ID:        targetID,
						Type:      "external",
						Name:      targetName,
						Level:     cur.depth + 1,
						Direction: "downstream",
					}
				}
			}

			if targetID != "" {
				sourceID := fmt.Sprintf("pipe_%d", cur.id)
				edgeKey := fmt.Sprintf("%s_%s", sourceID, targetID)
				if _, exists := edgeMap[edgeKey]; !exists {
					edgeMap[edgeKey] = &LineageGraphEdge{
						ID:             edge.ID,
						Source:         sourceID,
						Target:         targetID,
						DependencyType: string(edge.DependencyType),
						Description:    edge.Description,
					}
				}
			}

			if edge.DownstreamType == models.LineageNodePipeline && edge.DownstreamPipelineID != nil {
				if !visited[*edge.DownstreamPipelineID] {
					visited[*edge.DownstreamPipelineID] = true
					queue = append(queue, queueItem{*edge.DownstreamPipelineID, cur.depth + 1})
				}
			}
		}
	}
}

func (s *LineageService) getPipelineStatusInfo(pipelineID uint) (string, *time.Time, string, bool) {
	var lastRun models.PipelineRun
	s.db.Where("pipeline_id = ?", pipelineID).Order("created_at DESC").Limit(1).Find(&lastRun)

	lastRunStatus := "gray"
	var lastRunTime *time.Time
	if lastRun.ID > 0 {
		lastRunTime = lastRun.ActualEnd
		if lastRunTime == nil {
			lastRunTime = lastRun.ActualStart
		}
		switch lastRun.Status {
		case models.RunSuccess:
			lastRunStatus = "success"
		case models.RunFailed, models.RunTimeout, models.RunCancelled:
			lastRunStatus = "failed"
		case models.RunRunning:
			lastRunStatus = "running"
		}
	}

	var latestEval models.SLAEvaluation
	s.db.Joins("JOIN sla_rules ON sla_evaluations.rule_id = sla_rules.id").
		Where("sla_rules.pipeline_id = ?", pipelineID).
		Order("sla_evaluations.created_at DESC").
		Limit(1).Find(&latestEval)

	slaStatus := "unknown"
	if latestEval.ID > 0 {
		slaStatus = string(latestEval.Result)
	}

	var alertCount int64
	s.db.Model(&models.AlertEvent{}).
		Where("pipeline_id = ? AND status IN ?", pipelineID, []string{"open", "acknowledged"}).
		Count(&alertCount)
	hasAlert := alertCount > 0

	return lastRunStatus, lastRunTime, slaStatus, hasAlert
}

type ImpactAnalysisItem struct {
	PipelineID     uint      `json:"pipelineId"`
	Name           string    `json:"name"`
	Code           string    `json:"code"`
	Team           string    `json:"team"`
	SLAStatus      string    `json:"slaStatus"`
	Depth          int       `json:"depth"`
	ExpectedImpact int       `json:"expectedImpactSec"`
	HasAlert       bool      `json:"hasAlert"`
	AlertCount     int64     `json:"alertCount"`
}

type ImpactAnalysisResult struct {
	TotalAffected int                  `json:"totalAffected"`
	TotalImpactSec int                 `json:"totalImpactSec"`
	AffectedPipelines []ImpactAnalysisItem `json:"affectedPipelines"`
}

func (s *LineageService) ImpactAnalysis(nodeID string) (*ImpactAnalysisResult, error) {
	var pipelineID uint
	if len(nodeID) > 5 && nodeID[:5] == "pipe_" {
		fmt.Sscanf(nodeID, "pipe_%d", &pipelineID)
	} else {
		return nil, errors.New("仅支持对管道节点进行影响分析")
	}

	visited := make(map[uint]bool)
	type queueItem struct {
		id    uint
		depth int
	}
	queue := []queueItem{{pipelineID, 0}}
	visited[pipelineID] = true

	pipelineDurations := make(map[uint]int)
	affectedItems := make([]ImpactAnalysisItem, 0)

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		if cur.id != pipelineID {
			var pipe models.Pipeline
			if err := s.db.Where("id = ?", cur.id).First(&pipe).Error; err != nil {
				continue
			}

			avgDuration := pipe.ExpectedRunSec
			if avgDuration <= 0 {
				var runs []models.PipelineRun
				s.db.Where("pipeline_id = ? AND status = ?", cur.id, models.RunSuccess).
					Order("created_at DESC").Limit(10).Find(&runs)
				if len(runs) > 0 {
					total := 0
					for _, r := range runs {
						total += r.DurationSec
					}
					avgDuration = total / len(runs)
				} else {
					avgDuration = 300
				}
			}
			pipelineDurations[cur.id] = avgDuration

			_, _, slaStatus, hasAlert := s.getPipelineStatusInfo(cur.id)

			var alertCount int64
			s.db.Model(&models.AlertEvent{}).
				Where("pipeline_id = ? AND status IN ?", cur.id, []string{"open", "acknowledged"}).
				Count(&alertCount)

			affectedItems = append(affectedItems, ImpactAnalysisItem{
				PipelineID:     pipe.ID,
				Name:           pipe.Name,
				Code:           pipe.Code,
				Team:           pipe.Team,
				SLAStatus:      slaStatus,
				Depth:          cur.depth,
				ExpectedImpact: s.calculateCumulativeImpact(cur.id, pipelineID, pipelineDurations),
				HasAlert:       hasAlert,
				AlertCount:     alertCount,
			})
		}

		var edges []models.LineageEdge
		s.db.Where("pipeline_id = ? AND edge_direction = ? AND upstream_type = ? AND upstream_pipeline_id = ?",
			cur.id, "downstream", models.LineageNodePipeline, cur.id).Find(&edges)

		for _, edge := range edges {
			if edge.DownstreamType == models.LineageNodePipeline && edge.DownstreamPipelineID != nil {
				nextID := *edge.DownstreamPipelineID
				if !visited[nextID] {
					visited[nextID] = true
					queue = append(queue, queueItem{nextID, cur.depth + 1})
				}
			}
		}
	}

	totalImpact := 0
	for _, item := range affectedItems {
		totalImpact += item.ExpectedImpact
	}

	return &ImpactAnalysisResult{
		TotalAffected:     len(affectedItems),
		TotalImpactSec:    totalImpact,
		AffectedPipelines: affectedItems,
	}, nil
}

func (s *LineageService) calculateCumulativeImpact(targetID, startID uint, durations map[uint]int) int {
	maxPath := 0

	var dfs func(current uint, visited map[uint]bool, currentSum int)
	dfs = func(current uint, visited map[uint]bool, currentSum int) {
		if current == targetID {
			if currentSum > maxPath {
				maxPath = currentSum
			}
			return
		}

		var edges []models.LineageEdge
		s.db.Where("pipeline_id = ? AND edge_direction = ? AND upstream_type = ? AND upstream_pipeline_id = ?",
			current, "downstream", models.LineageNodePipeline, current).Find(&edges)

		for _, edge := range edges {
			if edge.DownstreamType == models.LineageNodePipeline && edge.DownstreamPipelineID != nil {
				nextID := *edge.DownstreamPipelineID
				if !visited[nextID] {
					visited[nextID] = true
					duration := durations[nextID]
					if duration <= 0 {
						duration = 300
					}
					dfs(nextID, visited, currentSum+duration)
					visited[nextID] = false
				}
			}
		}
	}

	visited := make(map[uint]bool)
	visited[startID] = true
	dfs(startID, visited, 0)

	return maxPath
}
