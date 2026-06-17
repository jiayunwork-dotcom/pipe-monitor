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

	var mainEdge *models.LineageEdge
	err := s.db.Transaction(func(tx *gorm.DB) error {
		currentDir := s.getCurrentEdgeDirection(req)

		mainEdge = &models.LineageEdge{
			TenantID:             req.TenantID,
			PipelineID:           req.PipelineID,
			UpstreamType:         req.UpstreamType,
			UpstreamPipelineID:   req.UpstreamPipelineID,
			UpstreamExternal:     req.UpstreamExternal,
			DownstreamType:       req.DownstreamType,
			DownstreamPipelineID: req.DownstreamPipelineID,
			DownstreamExternal:   req.DownstreamExternal,
			DependencyType:       req.DependencyType,
			EdgeDirection:        currentDir,
			Description:          req.Description,
			CreatedBy:            req.UserID,
		}

		if err := tx.Create(mainEdge).Error; err != nil {
			return err
		}

		peerPipeID, hasPeer := s.getPeerPipelineID(req)
		if hasPeer {
			peerDir := "upstream"
			if currentDir == "upstream" {
				peerDir = "downstream"
			}
			peerEdge := &models.LineageEdge{
				TenantID:             req.TenantID,
				PipelineID:           peerPipeID,
				UpstreamType:         req.UpstreamType,
				UpstreamPipelineID:   req.UpstreamPipelineID,
				UpstreamExternal:     req.UpstreamExternal,
				DownstreamType:       req.DownstreamType,
				DownstreamPipelineID: req.DownstreamPipelineID,
				DownstreamExternal:   req.DownstreamExternal,
				DependencyType:       req.DependencyType,
				EdgeDirection:        peerDir,
				Description:          req.Description,
				CreatedBy:            req.UserID,
			}
			if err := tx.Create(peerEdge).Error; err != nil {
				return err
			}
		}

		edgeInfo := map[string]interface{}{
			"id":                   mainEdge.ID,
			"upstreamType":         mainEdge.UpstreamType,
			"upstreamPipelineId":   mainEdge.UpstreamPipelineID,
			"upstreamExternal":     mainEdge.UpstreamExternal,
			"downstreamType":       mainEdge.DownstreamType,
			"downstreamPipelineId": mainEdge.DownstreamPipelineID,
			"downstreamExternal":   mainEdge.DownstreamExternal,
			"dependencyType":       mainEdge.DependencyType,
			"description":          mainEdge.Description,
		}

		auditLog := &models.LineageAuditLog{
			TenantID:   req.TenantID,
			PipelineID: req.PipelineID,
			UserID:     req.UserID,
			ActionType: "add",
			EdgeID:     &mainEdge.ID,
			EdgeInfo:   utils.JSONString(utils.ToJSON(edgeInfo)),
			IPAddress:  req.IPAddress,
		}

		return tx.Create(auditLog).Error
	})

	if err != nil {
		return nil, err
	}

	return mainEdge, nil
}

func (s *LineageService) getCurrentEdgeDirection(req *AddLineageEdgeReq) string {
	if req.DownstreamType == models.LineageNodePipeline &&
		req.DownstreamPipelineID != nil &&
		*req.DownstreamPipelineID == req.PipelineID {
		return "upstream"
	}
	if req.UpstreamType == models.LineageNodePipeline &&
		req.UpstreamPipelineID != nil &&
		*req.UpstreamPipelineID == req.PipelineID {
		return "downstream"
	}
	if req.UpstreamType == models.LineageNodeExternal {
		return "upstream"
	}
	if req.DownstreamType == models.LineageNodeExternal {
		return "downstream"
	}
	return "upstream"
}

func (s *LineageService) getPeerPipelineID(req *AddLineageEdgeReq) (uint, bool) {
	if req.UpstreamType == models.LineageNodePipeline &&
		req.UpstreamPipelineID != nil &&
		*req.UpstreamPipelineID != req.PipelineID {
		return *req.UpstreamPipelineID, true
	}
	if req.DownstreamType == models.LineageNodePipeline &&
		req.DownstreamPipelineID != nil &&
		*req.DownstreamPipelineID != req.PipelineID {
		return *req.DownstreamPipelineID, true
	}
	return 0, false
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
			EdgeInfo:   utils.JSONString(utils.ToJSON(edgeInfo)),
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

type BatchImportItem struct {
	UpstreamCode   string `json:"upstreamCode"`
	DownstreamCode string `json:"downstreamCode"`
	DependencyType string `json:"dependencyType"`
	Description    string `json:"description"`
}

type BatchImportResultItem struct {
	RowIndex       int    `json:"rowIndex"`
	UpstreamCode   string `json:"upstreamCode"`
	DownstreamCode string `json:"downstreamCode"`
	Success        bool   `json:"success"`
	Reason         string `json:"reason,omitempty"`
	EdgeID         *uint  `json:"edgeId,omitempty"`
}

type edgeCandidate struct {
	RowIndex             int
	UpstreamPipelineID   uint
	DownstreamPipelineID uint
	DependencyType       models.LineageDependencyType
	Description          string
	UpstreamCode         string
	DownstreamCode       string
}

type BatchImportResult struct {
	TotalCount   int                    `json:"totalCount"`
	SuccessCount int                    `json:"successCount"`
	FailCount    int                    `json:"failCount"`
	HasCycle     bool                   `json:"hasCycle"`
	CyclePath    string                 `json:"cyclePath,omitempty"`
	Items        []BatchImportResultItem `json:"items"`
}

func (s *LineageService) BatchImport(tenantID, userID uint, ipAddress string, items []BatchImportItem) (*BatchImportResult, error) {
	result := &BatchImportResult{
		TotalCount: len(items),
		Items:      make([]BatchImportResultItem, 0, len(items)),
	}

	pipeCodeToID := make(map[string]uint)
	var allPipes []models.Pipeline
	if err := s.db.Select("id, code").Where("tenant_id = ?", tenantID).Find(&allPipes).Error; err != nil {
		return nil, err
	}
	for _, p := range allPipes {
		pipeCodeToID[p.Code] = p.ID
	}

	var validEdges []edgeCandidate

	for i, item := range items {
		rowResult := BatchImportResultItem{
			RowIndex:       i + 1,
			UpstreamCode:   item.UpstreamCode,
			DownstreamCode: item.DownstreamCode,
		}

		if item.UpstreamCode == "" || item.DownstreamCode == "" {
			rowResult.Success = false
			rowResult.Reason = "上游或下游管道编码不能为空"
			result.Items = append(result.Items, rowResult)
			continue
		}

		upstreamID, upOK := pipeCodeToID[item.UpstreamCode]
		downstreamID, downOK := pipeCodeToID[item.DownstreamCode]

		if !upOK {
			rowResult.Success = false
			rowResult.Reason = fmt.Sprintf("上游管道编码 %s 不存在", item.UpstreamCode)
			result.Items = append(result.Items, rowResult)
			continue
		}
		if !downOK {
			rowResult.Success = false
			rowResult.Reason = fmt.Sprintf("下游管道编码 %s 不存在", item.DownstreamCode)
			result.Items = append(result.Items, rowResult)
			continue
		}

		if upstreamID == downstreamID {
			rowResult.Success = false
			rowResult.Reason = "上游和下游不能是同一管道"
			result.Items = append(result.Items, rowResult)
			continue
		}

		depType := models.LineageDepHard
		if item.DependencyType == "soft" || item.DependencyType == "弱依赖" {
			depType = models.LineageDepSoft
		}

		validEdges = append(validEdges, edgeCandidate{
			RowIndex:         i + 1,
			UpstreamPipelineID: upstreamID,
			DownstreamPipelineID: downstreamID,
			DependencyType:   depType,
			Description:      item.Description,
			UpstreamCode:     item.UpstreamCode,
			DownstreamCode:   item.DownstreamCode,
		})
	}

	if len(validEdges) == 0 {
		return result, nil
	}

	if cyclePath, hasCycle := s.detectBatchCycle(tenantID, validEdges); hasCycle {
		result.HasCycle = true
		result.CyclePath = cyclePath
		for _, e := range validEdges {
			result.Items = append(result.Items, BatchImportResultItem{
				RowIndex:       e.RowIndex,
				UpstreamCode:   e.UpstreamCode,
				DownstreamCode: e.DownstreamCode,
				Success:        false,
				Reason:         "检测到循环依赖，批量导入已全部拒绝",
			})
		}
		return result, nil
	}

	for _, e := range validEdges {
		addReq := &AddLineageEdgeReq{
			TenantID:             tenantID,
			PipelineID:           e.DownstreamPipelineID,
			UserID:               userID,
			IPAddress:            ipAddress,
			UpstreamType:         models.LineageNodePipeline,
			UpstreamPipelineID:   &e.UpstreamPipelineID,
			DownstreamType:       models.LineageNodePipeline,
			DownstreamPipelineID: &e.DownstreamPipelineID,
			DependencyType:       e.DependencyType,
			Description:          e.Description,
		}

		edge, err := s.AddEdge(addReq)
		rowResult := BatchImportResultItem{
			RowIndex:       e.RowIndex,
			UpstreamCode:   e.UpstreamCode,
			DownstreamCode: e.DownstreamCode,
		}
		if err != nil {
			rowResult.Success = false
			rowResult.Reason = err.Error()
		} else {
			rowResult.Success = true
			rowResult.EdgeID = &edge.ID
			result.SuccessCount++
		}
		result.Items = append(result.Items, rowResult)
	}

	result.FailCount = len(result.Items) - result.SuccessCount
	return result, nil
}

func (s *LineageService) detectBatchCycle(tenantID uint, edges []edgeCandidate) (string, bool) {
	graph := make(map[uint][]uint)
	nodeSet := make(map[uint]bool)

	var existingEdges []models.LineageEdge
	s.db.Where("tenant_id = ? AND upstream_type = ? AND downstream_type = ?",
		tenantID, models.LineageNodePipeline, models.LineageNodePipeline).
		Find(&existingEdges)

	for _, e := range existingEdges {
		if e.UpstreamPipelineID != nil && e.DownstreamPipelineID != nil {
			graph[*e.DownstreamPipelineID] = append(graph[*e.DownstreamPipelineID], *e.UpstreamPipelineID)
			nodeSet[*e.UpstreamPipelineID] = true
			nodeSet[*e.DownstreamPipelineID] = true
		}
	}

	for _, e := range edges {
		graph[e.DownstreamPipelineID] = append(graph[e.DownstreamPipelineID], e.UpstreamPipelineID)
		nodeSet[e.UpstreamPipelineID] = true
		nodeSet[e.DownstreamPipelineID] = true
	}

	visited := make(map[uint]bool)
	recStack := make(map[uint]bool)
	path := make([]uint, 0)

	for node := range nodeSet {
		if !visited[node] {
			if cycle := s.dfsFindCycleInGraph(graph, node, visited, recStack, path); cycle != "" {
				return cycle, true
			}
		}
	}
	return "", false
}

func (s *LineageService) dfsFindCycleInGraph(graph map[uint][]uint, start uint, visited, recStack map[uint]bool, path []uint) string {
	visited[start] = true
	recStack[start] = true
	path = append(path, start)

	for _, next := range graph[start] {
		if !visited[next] {
			if cycle := s.dfsFindCycleInGraph(graph, next, visited, recStack, path); cycle != "" {
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

type SnapshotEdgeData struct {
	ID                 uint   `json:"id"`
	UpstreamType       string `json:"upstreamType"`
	UpstreamPipelineID *uint  `json:"upstreamPipelineId,omitempty"`
	UpstreamExternal   string `json:"upstreamExternal,omitempty"`
	UpstreamName       string `json:"upstreamName,omitempty"`
	UpstreamCode       string `json:"upstreamCode,omitempty"`
	DownstreamType     string `json:"downstreamType"`
	DownstreamPipelineID *uint `json:"downstreamPipelineId,omitempty"`
	DownstreamExternal string `json:"downstreamExternal,omitempty"`
	DownstreamName     string `json:"downstreamName,omitempty"`
	DownstreamCode     string `json:"downstreamCode,omitempty"`
	DependencyType     string `json:"dependencyType"`
	Description        string `json:"description"`
}

type CreateSnapshotReq struct {
	TenantID    uint
	UserID      uint
	Name        string
	Description string
}

func (s *LineageService) CreateSnapshot(req *CreateSnapshotReq) (*models.LineageSnapshot, error) {
	var edges []models.LineageEdge
	if err := s.db.Preload("UpstreamPipeline").Preload("DownstreamPipeline").
		Where("tenant_id = ?", req.TenantID).
		Find(&edges).Error; err != nil {
		return nil, err
	}

	edgeData := make([]SnapshotEdgeData, 0, len(edges))
	seen := make(map[string]bool)
	for _, e := range edges {
		key := fmt.Sprintf("%s_%v_%s_%s_%v_%s",
			e.UpstreamType, e.UpstreamPipelineID, e.UpstreamExternal,
			e.DownstreamType, e.DownstreamPipelineID, e.DownstreamExternal)
		if seen[key] {
			continue
		}
		seen[key] = true

		upName, upCode := "", ""
		if e.UpstreamPipeline != nil {
			upName = e.UpstreamPipeline.Name
			upCode = e.UpstreamPipeline.Code
		}
		downName, downCode := "", ""
		if e.DownstreamPipeline != nil {
			downName = e.DownstreamPipeline.Name
			downCode = e.DownstreamPipeline.Code
		}

		edgeData = append(edgeData, SnapshotEdgeData{
			ID:                   e.ID,
			UpstreamType:         string(e.UpstreamType),
			UpstreamPipelineID:   e.UpstreamPipelineID,
			UpstreamExternal:     e.UpstreamExternal,
			UpstreamName:         upName,
			UpstreamCode:         upCode,
			DownstreamType:       string(e.DownstreamType),
			DownstreamPipelineID: e.DownstreamPipelineID,
			DownstreamExternal:   e.DownstreamExternal,
			DownstreamName:       downName,
			DownstreamCode:       downCode,
			DependencyType:       string(e.DependencyType),
			Description:          e.Description,
		})
	}

	snapshot := &models.LineageSnapshot{
		TenantID:     req.TenantID,
		Name:         req.Name,
		Description:  req.Description,
		SnapshotData: utils.JSONString(utils.ToJSON(edgeData)),
		CreatedBy:    req.UserID,
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(snapshot).Error; err != nil {
			return err
		}

		var total int64
		tx.Model(&models.LineageSnapshot{}).Where("tenant_id = ?", req.TenantID).Count(&total)
		if total > 10 {
			var oldSnapshots []models.LineageSnapshot
			tx.Where("tenant_id = ?", req.TenantID).
				Order("created_at ASC").
				Limit(int(total - 10)).
				Find(&oldSnapshots)
			for _, ss := range oldSnapshots {
				tx.Delete(&ss)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return snapshot, nil
}

func (s *LineageService) ListSnapshots(tenantID uint) ([]models.LineageSnapshot, error) {
	var snapshots []models.LineageSnapshot
	if err := s.db.Preload("User").
		Where("tenant_id = ?", tenantID).
		Order("created_at DESC").
		Find(&snapshots).Error; err != nil {
		return nil, err
	}
	return snapshots, nil
}

func (s *LineageService) DeleteSnapshot(tenantID, snapshotID, userID uint) error {
	result := s.db.Where("id = ? AND tenant_id = ?", snapshotID, tenantID).
		Delete(&models.LineageSnapshot{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("快照不存在")
	}
	return nil
}

type SnapshotDiffItem struct {
	ChangeType     string `json:"changeType"`
	UpstreamName   string `json:"upstreamName"`
	UpstreamCode   string `json:"upstreamCode"`
	DownstreamName string `json:"downstreamName"`
	DownstreamCode string `json:"downstreamCode"`
	DependencyType string `json:"dependencyType"`
	Description    string `json:"description"`
}

type SnapshotDiffResult struct {
	Added   []SnapshotDiffItem `json:"added"`
	Removed []SnapshotDiffItem `json:"removed"`
}

func (s *LineageService) CompareSnapshots(tenantID uint, snapshotID1, snapshotID2 uint) (*SnapshotDiffResult, error) {
	var ss1, ss2 models.LineageSnapshot
	if err := s.db.Where("id = ? AND tenant_id = ?", snapshotID1, tenantID).First(&ss1).Error; err != nil {
		return nil, errors.New("快照1不存在")
	}
	if err := s.db.Where("id = ? AND tenant_id = ?", snapshotID2, tenantID).First(&ss2).Error; err != nil {
		return nil, errors.New("快照2不存在")
	}

	var edges1, edges2 []SnapshotEdgeData
	utils.FromJSON(string(ss1.SnapshotData), &edges1)
	utils.FromJSON(string(ss2.SnapshotData), &edges2)

	makeKey := func(e SnapshotEdgeData) string {
		return fmt.Sprintf("%s_%v_%s_%s_%v_%s",
			e.UpstreamType, e.UpstreamPipelineID, e.UpstreamExternal,
			e.DownstreamType, e.DownstreamPipelineID, e.DownstreamExternal)
	}

	map1 := make(map[string]SnapshotEdgeData)
	for _, e := range edges1 {
		map1[makeKey(e)] = e
	}

	map2 := make(map[string]SnapshotEdgeData)
	for _, e := range edges2 {
		map2[makeKey(e)] = e
	}

	result := &SnapshotDiffResult{
		Added:   make([]SnapshotDiffItem, 0),
		Removed: make([]SnapshotDiffItem, 0),
	}

	for k, e := range map2 {
		if _, exists := map1[k]; !exists {
			result.Added = append(result.Added, SnapshotDiffItem{
				ChangeType:     "added",
				UpstreamName:   e.UpstreamName,
				UpstreamCode:   e.UpstreamCode,
				DownstreamName: e.DownstreamName,
				DownstreamCode: e.DownstreamCode,
				DependencyType: e.DependencyType,
				Description:    e.Description,
			})
		}
	}

	for k, e := range map1 {
		if _, exists := map2[k]; !exists {
			result.Removed = append(result.Removed, SnapshotDiffItem{
				ChangeType:     "removed",
				UpstreamName:   e.UpstreamName,
				UpstreamCode:   e.UpstreamCode,
				DownstreamName: e.DownstreamName,
				DownstreamCode: e.DownstreamCode,
				DependencyType: e.DependencyType,
				Description:    e.Description,
			})
		}
	}

	return result, nil
}

type HealthScoreResult struct {
	Score    int      `json:"score"`
	Details  []string `json:"details"`
	Level    string   `json:"level"`
}

func (s *LineageService) CalculateHealthScore(pipelineID uint) (*HealthScoreResult, error) {
	score := 100
	var details []string

	var upstreamEdges []models.LineageEdge
	s.db.Preload("UpstreamPipeline").
		Where("pipeline_id = ? AND edge_direction = ? AND downstream_type = ? AND downstream_pipeline_id = ?",
			pipelineID, "upstream", models.LineageNodePipeline, pipelineID).
		Find(&upstreamEdges)

	var downstreamEdges []models.LineageEdge
	s.db.Preload("DownstreamPipeline").
		Where("pipeline_id = ? AND edge_direction = ? AND upstream_type = ? AND upstream_pipeline_id = ?",
			pipelineID, "downstream", models.LineageNodePipeline, pipelineID).
		Find(&downstreamEdges)

	if len(upstreamEdges) == 0 {
		score -= 20
		details = append(details, "无上游依赖声明: -20分")
	}

	if len(downstreamEdges) == 0 {
		score -= 10
		details = append(details, "无下游产出声明: -10分")
	}

	maxDepth := s.calculateUpstreamDepth(pipelineID, 0, make(map[uint]bool))
	if maxDepth > 3 {
		score -= 10
		details = append(details, fmt.Sprintf("存在超过3层的间接依赖链(当前%d层): -10分", maxDepth))
	}

	sevenDaysAgo := time.Now().AddDate(0, 0, -7)
	for _, edge := range upstreamEdges {
		if edge.UpstreamType == models.LineageNodePipeline && edge.UpstreamPipelineID != nil {
			var failCount int64
			s.db.Model(&models.PipelineRun{}).
				Where("pipeline_id = ? AND status IN ? AND created_at >= ?",
					*edge.UpstreamPipelineID,
					[]models.RunStatus{models.RunFailed, models.RunTimeout, models.RunCancelled},
					sevenDaysAgo).
				Count(&failCount)
			if failCount > 0 {
				score -= 15
				pipeName := fmt.Sprintf("ID:%d", *edge.UpstreamPipelineID)
				if edge.UpstreamPipeline != nil {
					pipeName = edge.UpstreamPipeline.Name
				}
				details = append(details, fmt.Sprintf("上游管道[%s]最近7天内运行失败: -15分", pipeName))
			}
		}
	}

	for _, edge := range upstreamEdges {
		if edge.DependencyType == models.LineageDepSoft &&
			edge.UpstreamType == models.LineageNodePipeline &&
			edge.UpstreamPipelineID != nil {
			var recentRuns []models.PipelineRun
			s.db.Where("pipeline_id = ?", *edge.UpstreamPipelineID).
				Order("created_at DESC").
				Limit(3).
				Find(&recentRuns)
			if len(recentRuns) == 3 {
				allFailed := true
				for _, r := range recentRuns {
					if r.Status == models.RunSuccess {
						allFailed = false
						break
					}
				}
				if allFailed {
					score -= 25
					pipeName := fmt.Sprintf("ID:%d", *edge.UpstreamPipelineID)
					if edge.UpstreamPipeline != nil {
						pipeName = edge.UpstreamPipeline.Name
					}
					details = append(details, fmt.Sprintf("弱依赖上游管道[%s]连续3次运行失败: -25分", pipeName))
				}
			}
		}
	}

	if len(details) == 0 && maxDepth <= 2 {
		score = 100
		details = append(details, "上游全部运行正常且依赖层级不超过2层: 满分")
	}

	if score < 0 {
		score = 0
	}

	level := "excellent"
	if score < 60 {
		level = "poor"
	} else if score < 80 {
		level = "fair"
	} else if score < 90 {
		level = "good"
	}

	return &HealthScoreResult{
		Score:   score,
		Details: details,
		Level:   level,
	}, nil
}

func (s *LineageService) calculateUpstreamDepth(pipelineID uint, currentDepth int, visited map[uint]bool) int {
	if visited[pipelineID] {
		return currentDepth
	}
	visited[pipelineID] = true

	var edges []models.LineageEdge
	s.db.Where("pipeline_id = ? AND edge_direction = ? AND downstream_type = ? AND downstream_pipeline_id = ?",
		pipelineID, "upstream", models.LineageNodePipeline, pipelineID).Find(&edges)

	if len(edges) == 0 {
		return currentDepth
	}

	maxChildDepth := currentDepth
	for _, edge := range edges {
		if edge.UpstreamType == models.LineageNodePipeline && edge.UpstreamPipelineID != nil {
			childDepth := s.calculateUpstreamDepth(*edge.UpstreamPipelineID, currentDepth+1, visited)
			if childDepth > maxChildDepth {
				maxChildDepth = childDepth
			}
		}
	}
	return maxChildDepth
}
