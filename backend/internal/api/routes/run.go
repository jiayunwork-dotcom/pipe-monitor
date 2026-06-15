package routes

import (
	"pipe-monitor/internal/middleware"
	"pipe-monitor/internal/services"
	"github.com/gofiber/fiber/v2"
)

func setupRunRoutes(r fiber.Router, svc *services.RunService) {
	runs := r.Group("/runs")

	runs.Get("", func(c *fiber.Ctx) error {
		auth := middleware.GetAuthCtx(c)
		isSuper := auth.Role == "super_admin"
		pid := parseUintParam(c, "pipelineId")
		status := c.Query("status")
		days := parseIntParam(c, "days", 7)
		page := parseIntParam(c, "page", 1)
		size := parseIntParam(c, "pageSize", 20)
		result, err := svc.ListRuns(auth.TenantID, isSuper, pid, status, days, page, size)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(result)
	})

	runs.Post("/report", func(c *fiber.Ctx) error {
		auth := middleware.GetAuthCtx(c)
		var req services.ReportRunReq
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "参数错误: " + err.Error()})
		}
		if req.TenantID == 0 {
			req.TenantID = auth.TenantID
		}
		run, err := svc.ReportRun(&req)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"data": run})
	})
}
