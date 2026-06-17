package routes

import (
	"pipe-monitor/internal/middleware"
	"pipe-monitor/internal/models"
	"pipe-monitor/internal/services"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func setupLineageRoutes(r fiber.Router, lineageSvc *services.LineageService) {
	lineage := r.Group("/lineage")
	adminOnly := middleware.AdminRequired()

	lineage.Post("/pipelines/:id/edges", adminOnly, func(c *fiber.Ctx) error {
		auth := middleware.GetAuthCtx(c)
		pipelineID := parseUintID(c, "id")

		var req struct {
			UpstreamType         string `json:"upstreamType"`
			UpstreamPipelineID   *uint  `json:"upstreamPipelineId"`
			UpstreamExternal     string `json:"upstreamExternal"`
			DownstreamType       string `json:"downstreamType"`
			DownstreamPipelineID *uint  `json:"downstreamPipelineId"`
			DownstreamExternal   string `json:"downstreamExternal"`
			DependencyType       string `json:"dependencyType"`
			Description          string `json:"description"`
		}

		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "参数错误: " + err.Error()})
		}

		ip := c.IP()
		xfwd := c.Get("X-Forwarded-For")
		if xfwd != "" {
			ips := strings.Split(xfwd, ",")
			if len(ips) > 0 {
				ip = strings.TrimSpace(ips[0])
			}
		}

		if req.DownstreamType == "" {
			req.DownstreamType = "pipeline"
			req.DownstreamPipelineID = &pipelineID
		}
		if req.UpstreamType == "" {
			req.UpstreamType = "pipeline"
		}
		if req.DependencyType == "" {
			req.DependencyType = "hard"
		}

		addReq := &services.AddLineageEdgeReq{
			TenantID:             auth.TenantID,
			PipelineID:           pipelineID,
			UserID:               auth.UserID,
			IPAddress:            ip,
			UpstreamType:         models.LineageNodeType(req.UpstreamType),
			UpstreamPipelineID:   req.UpstreamPipelineID,
			UpstreamExternal:     req.UpstreamExternal,
			DownstreamType:       models.LineageNodeType(req.DownstreamType),
			DownstreamPipelineID: req.DownstreamPipelineID,
			DownstreamExternal:   req.DownstreamExternal,
			DependencyType:       models.LineageDependencyType(req.DependencyType),
			Description:          req.Description,
		}

		edge, err := lineageSvc.AddEdge(addReq)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}

		return c.JSON(fiber.Map{"data": edge})
	})

	lineage.Delete("/pipelines/:id/edges/:edgeId", adminOnly, func(c *fiber.Ctx) error {
		auth := middleware.GetAuthCtx(c)
		edgeID := parseUintID(c, "edgeId")

		ip := c.IP()
		xfwd := c.Get("X-Forwarded-For")
		if xfwd != "" {
			ips := strings.Split(xfwd, ",")
			if len(ips) > 0 {
				ip = strings.TrimSpace(ips[0])
			}
		}

		if err := lineageSvc.RemoveEdge(auth.TenantID, edgeID, auth.UserID, ip); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}

		return c.JSON(fiber.Map{"success": true})
	})

	lineage.Get("/pipelines/:id", func(c *fiber.Ctx) error {
		pipelineID := parseUintID(c, "id")
		result, err := lineageSvc.GetPipelineLineage(pipelineID)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"data": result})
	})

	lineage.Get("/pipelines/:id/audit", func(c *fiber.Ctx) error {
		pipelineID := parseUintID(c, "id")
		actionType := c.Query("actionType", "all")
		page := parseIntParam(c, "page", 1)
		pageSize := parseIntParam(c, "pageSize", 20)

		result, err := lineageSvc.GetAuditLogs(pipelineID, actionType, page, pageSize)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"data": result})
	})

	lineage.Get("/graph", func(c *fiber.Ctx) error {
		auth := middleware.GetAuthCtx(c)
		isSuper := auth.Role == "super_admin"
		pipelineID := parseUintParam(c, "pipelineId")
		if pipelineID == 0 {
			return c.Status(400).JSON(fiber.Map{"error": "请指定管道ID"})
		}
		maxDepth := parseIntParam(c, "maxDepth", 5)
		if maxDepth > 5 {
			maxDepth = 5
		}

		graph, err := lineageSvc.BuildLineageGraph(pipelineID, auth.TenantID, isSuper, maxDepth)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"data": graph})
	})

	lineage.Post("/impact-analysis", func(c *fiber.Ctx) error {
		var req struct {
			NodeID string `json:"nodeId"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "参数错误"})
		}
		if req.NodeID == "" {
			return c.Status(400).JSON(fiber.Map{"error": "请指定节点ID"})
		}

		result, err := lineageSvc.ImpactAnalysis(req.NodeID)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"data": result})
	})

	lineage.Post("/pipelines/:id/check-cycle", adminOnly, func(c *fiber.Ctx) error {
		auth := middleware.GetAuthCtx(c)
		pipelineID := parseUintID(c, "id")

		var req struct {
			UpstreamType         string `json:"upstreamType"`
			UpstreamPipelineID   *uint  `json:"upstreamPipelineId"`
			UpstreamExternal     string `json:"upstreamExternal"`
			DownstreamType       string `json:"downstreamType"`
			DownstreamPipelineID *uint  `json:"downstreamPipelineId"`
			DownstreamExternal   string `json:"downstreamExternal"`
		}

		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "参数错误: " + err.Error()})
		}

		if req.DownstreamType == "" {
			req.DownstreamType = "pipeline"
			req.DownstreamPipelineID = &pipelineID
		}

		checkReq := &services.AddLineageEdgeReq{
			TenantID:             auth.TenantID,
			PipelineID:           pipelineID,
			UserID:               auth.UserID,
			IPAddress:            c.IP(),
			UpstreamType:         models.LineageNodeType(req.UpstreamType),
			UpstreamPipelineID:   req.UpstreamPipelineID,
			UpstreamExternal:     req.UpstreamExternal,
			DownstreamType:       models.LineageNodeType(req.DownstreamType),
			DownstreamPipelineID: req.DownstreamPipelineID,
			DownstreamExternal:   req.DownstreamExternal,
		}

		cyclePath, err := lineageSvc.DetectCycle(checkReq)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"hasCycle": true, "cyclePath": cyclePath, "error": cyclePath})
		}
		return c.JSON(fiber.Map{"hasCycle": false})
	})
}
