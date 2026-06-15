package services

import (
	"errors"
	"fmt"
	"pipe-monitor/internal/models"
	"pipe-monitor/internal/utils"

	"gorm.io/gorm"
)

type PipelineService struct {
	db *gorm.DB
}

func NewPipelineService(db *gorm.DB) *PipelineService {
	return &PipelineService{db: db}
}

type CreatePipelineReq struct {
	TenantID       uint
	Name           string
	Code           string
	Description    string
	DataDomain     string
	SourceID       *uint
	SourceDetail   string
	TargetID       *uint
	TargetDetail   string
	ScheduleFreq   models.ScheduleFreq
	CronExpression string
	OwnerID        uint
	Team           string
	Status         models.PipelineStatus
	Tags           []string
	ExpectedRunSec int
	UpstreamIDs    []uint
}

func (s *PipelineService) Create(req *CreatePipelineReq) (*models.Pipeline, error) {
	var existing models.Pipeline
	if err := s.db.Where("code = ?", req.Code).First(&existing).Error; err == nil {
		return nil, errors.New("管道编码已存在")
	}

	pipe := &models.Pipeline{
		TenantID:       req.TenantID,
		Name:           req.Name,
		Code:           req.Code,
		Description:    req.Description,
		DataDomain:     req.DataDomain,
		SourceID:       req.SourceID,
		SourceDetail:   req.SourceDetail,
		TargetID:       req.TargetID,
		TargetDetail:   req.TargetDetail,
		ScheduleFreq:   req.ScheduleFreq,
		CronExpression: req.CronExpression,
		OwnerID:        req.OwnerID,
		Team:           req.Team,
		Status:         req.Status,
		Tags:           utils.ToJSON(req.Tags),
		WebhookToken:   utils.GenerateToken(32),
		ExpectedRunSec: req.ExpectedRunSec,
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(pipe).Error; err != nil {
			return err
		}
		for _, upID := range req.UpstreamIDs {
			dep := &models.PipelineDependency{
				TenantID:       req.TenantID,
				PipelineID:     pipe.ID,
				UpstreamID:     upID,
				DependencyType: "hard",
			}
			if err := tx.Create(dep).Error; err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	if len(req.UpstreamIDs) > 0 {
		if err := s.CheckCyclicDependency(pipe.ID); err != nil {
			return nil, fmt.Errorf("循环依赖校验失败: %v", err)
		}
	}

	return pipe, nil
}

func (s *PipelineService) CheckCyclicDependency(pipelineID uint) error {
	visited := make(map[uint]bool)
	recStack := make(map[uint]bool)
	return s.dfsHasCycle(pipelineID, visited, recStack)
}

func (s *PipelineService) dfsHasCycle(u uint, visited, recStack map[uint]bool) error {
	visited[u] = true
	recStack[u] = true

	var deps []models.PipelineDependency
	if err := s.db.Where("pipeline_id = ?", u).Find(&deps).Error; err != nil {
		return err
	}

	for _, dep := range deps {
		v := dep.UpstreamID
		if !visited[v] {
			if err := s.dfsHasCycle(v, visited, recStack); err != nil {
				return err
			}
		} else if recStack[v] {
			return fmt.Errorf("发现循环依赖: 管道 %d -> %d", u, v)
		}
	}

	recStack[u] = false
	return nil
}

type PaginatedPipelines struct {
	Total    int64              `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"pageSize"`
	Data     []models.Pipeline  `json:"data"`
}

func (s *PipelineService) List(tenantID uint, isSuper bool, team, dataDomain, freq, status string, page, pageSize int) (*PaginatedPipelines, error) {
	var pipes []models.Pipeline
	q := s.db.Model(&models.Pipeline{})
	if !isSuper {
		q = q.Where("tenant_id = ?", tenantID)
	}
	if team != "" {
		q = q.Where("team = ?", team)
	}
	if dataDomain != "" {
		q = q.Where("data_domain = ?", dataDomain)
	}
	if freq != "" {
		q = q.Where("schedule_freq = ?", freq)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	q = q.Preload("Owner").Order("created_at DESC")

	var count int64
	q.Count(&count)
	offset := (page - 1) * pageSize
	q.Offset(offset).Limit(pageSize).Find(&pipes)

	return &PaginatedPipelines{
		Total:    count,
		Page:     page,
		PageSize: pageSize,
		Data:     pipes,
	}, nil
}

func (s *PipelineService) GetByID(tenantID uint, isSuper bool, id uint) (*models.Pipeline, error) {
	var pipe models.Pipeline
	q := s.db.Model(&models.Pipeline{})
	if !isSuper {
		q = q.Where("tenant_id = ?", tenantID)
	}
	if err := q.Preload("Owner").Preload("Source").Preload("Target").
		Where("pipelines.id = ?", id).First(&pipe).Error; err != nil {
		return nil, err
	}
	return &pipe, nil
}

func (s *PipelineService) GetDependencies(pipelineID uint) ([]models.PipelineDependency, error) {
	var deps []models.PipelineDependency
	if err := s.db.Preload("Upstream").Where("pipeline_id = ?", pipelineID).Find(&deps).Error; err != nil {
		return nil, err
	}
	return deps, nil
}

func (s *PipelineService) GetDownstream(pipelineID uint) ([]models.PipelineDependency, error) {
	var deps []models.PipelineDependency
	if err := s.db.Preload("Pipeline").Where("upstream_id = ?", pipelineID).Find(&deps).Error; err != nil {
		return nil, err
	}
	return deps, nil
}

type DAGNode struct {
	ID       uint     `json:"id"`
	Code     string   `json:"code"`
	Name     string   `json:"name"`
	Level    int      `json:"level"`
	Status   string   `json:"status"`
	Health   string   `json:"health"`
	Children []uint   `json:"children"`
	Parents  []uint   `json:"parents"`
}

type DAGResult struct {
	Nodes    map[uint]*DAGNode `json:"nodes"`
	TopoSort []uint            `json:"topoSort"`
	Levels   map[int][]uint    `json:"levels"`
}

func (s *PipelineService) BuildDAG(tenantID uint, isSuper bool, startPipelineID uint, includeUpstream, includeDownstream bool, maxDepth int) (*DAGResult, error) {
	var allPipes []models.Pipeline
	q := s.db.Model(&models.Pipeline{})
	if !isSuper {
		q = q.Where("tenant_id = ?", tenantID)
	}
	if err := q.Find(&allPipes).Error; err != nil {
		return nil, err
	}

	pipeMap := make(map[uint]*models.Pipeline)
	for i := range allPipes {
		pipeMap[allPipes[i].ID] = &allPipes[i]
	}

	var allDeps []models.PipelineDependency
	depQuery := s.db
	if !isSuper {
		depQuery = depQuery.Where("tenant_id = ?", tenantID)
	}
	if err := depQuery.Find(&allDeps).Error; err != nil {
		return nil, err
	}

	children := make(map[uint][]uint)
	parents := make(map[uint][]uint)
	for _, d := range allDeps {
		children[d.UpstreamID] = append(children[d.UpstreamID], d.PipelineID)
		parents[d.PipelineID] = append(parents[d.PipelineID], d.UpstreamID)
	}

	nodes := make(map[uint]*DAGNode)
	if startPipelineID > 0 {
		visited := make(map[uint]bool)
		s.bfsCollect(startPipelineID, maxDepth, includeUpstream, includeDownstream, visited, pipeMap, children, parents, nodes)
	} else {
		for id, p := range pipeMap {
			nodes[id] = &DAGNode{
				ID:       id,
				Code:     p.Code,
				Name:     p.Name,
				Status:   string(p.Status),
				Children: children[id],
				Parents:  parents[id],
			}
		}
	}

	levels := make(map[int][]uint)
	inDegree := make(map[uint]int)
	for id, node := range nodes {
		degree := 0
		for _, pid := range node.Parents {
			if _, ok := nodes[pid]; ok {
				degree++
			}
		}
		inDegree[id] = degree
	}

	queue := make([]uint, 0)
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	topo := make([]uint, 0)
	currentLevel := 0
	for len(queue) > 0 {
		sz := len(queue)
		levels[currentLevel] = append(levels[currentLevel], queue...)
		for i := 0; i < sz; i++ {
			u := queue[0]
			queue = queue[1:]
			topo = append(topo, u)
			if n, ok := nodes[u]; ok {
				n.Level = currentLevel
			}
			for _, v := range children[u] {
				if _, ok := nodes[v]; !ok {
					continue
				}
				inDegree[v]--
				if inDegree[v] == 0 {
					queue = append(queue, v)
				}
			}
		}
		currentLevel++
	}

	return &DAGResult{Nodes: nodes, TopoSort: topo, Levels: levels}, nil
}

func (s *PipelineService) bfsCollect(start uint, maxDepth int, up, down bool, visited map[uint]bool, pipeMap map[uint]*models.Pipeline, children, parents map[uint][]uint, nodes map[uint]*DAGNode) {
	type item struct {
		id    uint
		depth int
	}
	q := []item{{start, 0}}
	visited[start] = true

	for len(q) > 0 {
		cur := q[0]
		q = q[1:]

		p, ok := pipeMap[cur.id]
		if !ok {
			continue
		}
		nodes[cur.id] = &DAGNode{
			ID:       cur.id,
			Code:     p.Code,
			Name:     p.Name,
			Status:   string(p.Status),
			Children: children[cur.id],
			Parents:  parents[cur.id],
		}

		if maxDepth == 0 || cur.depth < maxDepth {
			if down {
				for _, cid := range children[cur.id] {
					if !visited[cid] {
						visited[cid] = true
						q = append(q, item{cid, cur.depth + 1})
					}
				}
			}
			if up {
				for _, pid := range parents[cur.id] {
					if !visited[pid] {
						visited[pid] = true
						q = append(q, item{pid, cur.depth + 1})
					}
				}
			}
		}
	}
}

type AffectedResult struct {
	PipelineID  uint   `json:"pipelineId"`
	Name        string `json:"name"`
	Code        string `json:"code"`
	Depth       int    `json:"depth"`
	TotalImpact int    `json:"totalImpact"`
}

func (s *PipelineService) GetAffectedPipelines(pipelineID uint, tenantID uint, isSuper bool) ([]AffectedResult, error) {
	result := make([]AffectedResult, 0)
	visited := make(map[uint]bool)
	type qitem struct {
		id    uint
		depth int
	}
	var deps []models.PipelineDependency
	q := s.db.Where("upstream_id = ?", pipelineID)
	if !isSuper {
		q = q.Where("tenant_id = ?", tenantID)
	}
	if err := q.Find(&deps).Error; err != nil {
		return nil, err
	}
	queue := make([]qitem, 0)
	for _, d := range deps {
		queue = append(queue, qitem{d.PipelineID, 1})
		visited[d.PipelineID] = true
	}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		var p models.Pipeline
		if err := s.db.Select("id, name, code").First(&p, cur.id).Error; err != nil {
			continue
		}
		result = append(result, AffectedResult{
			PipelineID: p.ID,
			Name:       p.Name,
			Code:       p.Code,
			Depth:      cur.depth,
		})

		var next []models.PipelineDependency
		q2 := s.db.Where("upstream_id = ?", cur.id)
		if !isSuper {
			q2 = q2.Where("tenant_id = ?", tenantID)
		}
		if err := q2.Find(&next).Error; err != nil {
			continue
		}
		for _, d := range next {
			if !visited[d.PipelineID] {
				visited[d.PipelineID] = true
				queue = append(queue, qitem{d.PipelineID, cur.depth + 1})
			}
		}
	}

	for i := range result {
		var cnt int64
		s.db.Model(&models.PipelineDependency{}).Where("upstream_id = ?", result[i].PipelineID).Count(&cnt)
		result[i].TotalImpact = int(cnt)
	}
	return result, nil
}

type CriticalPathResult struct {
	Path        []uint  `json:"path"`
	PathDetails []string `json:"pathDetails"`
	TotalDuration int   `json:"totalDuration"`
}

func (s *PipelineService) CriticalPath(tenantID uint, isSuper bool) (*CriticalPathResult, error) {
	dag, err := s.BuildDAG(tenantID, isSuper, 0, true, true, 0)
	if err != nil {
		return nil, err
	}

	durations := make(map[uint]int)
	for id, _ := range dag.Nodes {
		var p models.Pipeline
		if err := s.db.Select("expected_run_sec").First(&p, id).Error; err == nil {
			durations[id] = p.ExpectedRunSec
			if durations[id] == 0 {
				durations[id] = 300
			}
		} else {
			durations[id] = 300
		}
	}

	dp := make(map[uint]int)
	prev := make(map[uint]uint)
	var endNode uint
	maxEnd := 0

	for _, u := range dag.TopoSort {
		if _, ok := dag.Nodes[u]; !ok {
			continue
		}
		if dp[u] == 0 {
			dp[u] = durations[u]
		}
		for _, v := range dag.Nodes[u].Children {
			if _, ok := dag.Nodes[v]; !ok {
				continue
			}
			if dp[v] < dp[u]+durations[v] {
				dp[v] = dp[u] + durations[v]
				prev[v] = u
			}
		}
		if dp[u] > maxEnd {
			maxEnd = dp[u]
			endNode = u
		}
	}

	path := make([]uint, 0)
	details := make([]string, 0)
	cur := endNode
	for cur != 0 {
		path = append([]uint{cur}, path...)
		if n, ok := dag.Nodes[cur]; ok {
			details = append([]string{n.Code}, details...)
		}
		cur = prev[cur]
	}

	return &CriticalPathResult{
		Path:          path,
		PathDetails:   details,
		TotalDuration: maxEnd,
	}, nil
}
