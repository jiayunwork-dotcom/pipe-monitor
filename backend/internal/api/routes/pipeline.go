package routes

import (
	"pipe-monitor/internal/middleware"
	"pipe-monitor/internal/models"
	"pipe-monitor/internal/services"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func setupPipelineRoutes(r fiber.Router, pipeSvc *services.PipelineService, runSvc *services.RunService) {
	pipes := r.Group("/pipelines")
	adminOnly := middleware.AdminRequired()

	pipes.Get("", func(c *fiber.Ctx) error {
		auth := middleware.GetAuthCtx(c)
		isSuper := auth.Role == "super_admin"
		team := c.Query("team")
		domain := c.Query("dataDomain")
		freq := c.Query("freq")
		status := c.Query("status")
		page := parseIntParam(c, "page", 1)
		size := parseIntParam(c, "pageSize", 20)
		result, err := pipeSvc.List(auth.TenantID, isSuper, team, domain, freq, status, page, size)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(result)
	})

	pipes.Get("/:id", func(c *fiber.Ctx) error {
		auth := middleware.GetAuthCtx(c)
		isSuper := auth.Role == "super_admin"
		id := parseUintID(c, "id")
		pipe, err := pipeSvc.GetByID(auth.TenantID, isSuper, id)
		if err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "管道不存在"})
		}
		deps, _ := pipeSvc.GetDependencies(id)
		down, _ := pipeSvc.GetDownstream(id)
		return c.JSON(fiber.Map{
			"pipeline":     pipe,
			"dependencies": deps,
			"downstream":   down,
		})
	})

	pipes.Post("", adminOnly, func(c *fiber.Ctx) error {
		auth := middleware.GetAuthCtx(c)
		var req struct {
			Name           string              `json:"name"`
			Code           string              `json:"code"`
			Description    string              `json:"description"`
			DataDomain     string              `json:"dataDomain"`
			SourceID       *uint               `json:"sourceId"`
			SourceDetail   string              `json:"sourceDetail"`
			TargetID       *uint               `json:"targetId"`
			TargetDetail   string              `json:"targetDetail"`
			ScheduleFreq   models.ScheduleFreq `json:"scheduleFreq"`
			CronExpression string              `json:"cronExpression"`
			OwnerID        uint                `json:"ownerId"`
			Team           string              `json:"team"`
			Status         models.PipelineStatus `json:"status"`
			Tags           []string            `json:"tags"`
			ExpectedRunSec int                 `json:"expectedRunSec"`
			UpstreamIDs    []uint              `json:"upstreamIds"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "参数错误: " + err.Error()})
		}
		tenantID := auth.TenantID
		createReq := &services.CreatePipelineReq{
			TenantID:       tenantID,
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
			Tags:           req.Tags,
			ExpectedRunSec: req.ExpectedRunSec,
			UpstreamIDs:    req.UpstreamIDs,
		}
		pipe, err := pipeSvc.Create(createReq)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"data": pipe})
	})

	pipes.Put("/:id", adminOnly, func(c *fiber.Ctx) error {
		auth := middleware.GetAuthCtx(c)
		_ = auth
		id := parseUintID(c, "id")
		var updates map[string]interface{}
		if err := c.BodyParser(&updates); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "参数错误"})
		}
		q := (&gorm.DB{}).Model(&models.Pipeline{}).Where("id = ?", id)
		_ = q
		return c.Status(501).JSON(fiber.Map{"error": "暂未实现"})
	})

	pipes.Get("/:id/runs/history", func(c *fiber.Ctx) error {
		id := parseUintID(c, "id")
		days := parseIntParam(c, "days", 30)
		history, err := runSvc.GetRunHistory(id, days)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"data": history})
	})

	pipes.Get("/dag/graph", func(c *fiber.Ctx) error {
		auth := middleware.GetAuthCtx(c)
		isSuper := auth.Role == "super_admin"
		startID := parseUintParam(c, "startPipelineId")
		up := c.Query("includeUpstream") == "true" || startID == 0
		down := c.Query("includeDownstream") == "true" || startID == 0
		depth := parseIntParam(c, "maxDepth", 0)
		dag, err := pipeSvc.BuildDAG(auth.TenantID, isSuper, startID, up, down, depth)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"data": dag})
	})

	pipes.Get("/:id/affected", func(c *fiber.Ctx) error {
		auth := middleware.GetAuthCtx(c)
		isSuper := auth.Role == "super_admin"
		id := parseUintID(c, "id")
		affected, err := pipeSvc.GetAffectedPipelines(id, auth.TenantID, isSuper)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"data": affected})
	})

	pipes.Get("/dag/critical-path", func(c *fiber.Ctx) error {
		auth := middleware.GetAuthCtx(c)
		isSuper := auth.Role == "super_admin"
		result, err := pipeSvc.CriticalPath(auth.TenantID, isSuper)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"data": result})
	})

	pipes.Post("/:id/check-cycle", adminOnly, func(c *fiber.Ctx) error {
		id := parseUintID(c, "id")
		err := pipeSvc.CheckCyclicDependency(id)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"hasCycle": true, "error": err.Error()})
		}
		return c.JSON(fiber.Map{"hasCycle": false})
	})
}
