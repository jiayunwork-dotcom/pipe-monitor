package routes

import (
	"pipe-monitor/internal/middleware"
	"pipe-monitor/internal/models"
	"pipe-monitor/internal/services"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func setupSLARoutes(r fiber.Router, engine *services.SLAEngine, db *gorm.DB) {
	sla := r.Group("/sla")
	adminOnly := middleware.AdminRequired()

	sla.Get("/rules", func(c *fiber.Ctx) error {
		auth := middleware.GetAuthCtx(c)
		isSuper := auth.Role == "super_admin"
		pid := parseUintParam(c, "pipelineId")
		var rules []models.SLARule
		q := db.Model(&models.SLARule{})
		if !isSuper {
			q = q.Where("tenant_id = ?", auth.TenantID)
		}
		if pid > 0 {
			q = q.Where("pipeline_id = ?", pid)
		}
		q.Preload("Pipeline").Find(&rules)
		return c.JSON(fiber.Map{"data": rules})
	})

	sla.Post("/rules", adminOnly, func(c *fiber.Ctx) error {
		auth := middleware.GetAuthCtx(c)
		var rule models.SLARule
		if err := c.BodyParser(&rule); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "参数错误"})
		}
		rule.TenantID = auth.TenantID
		if err := db.Create(&rule).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"data": rule})
	})

	sla.Put("/rules/:id", adminOnly, func(c *fiber.Ctx) error {
		id := parseUintID(c, "id")
		var updates map[string]interface{}
		if err := c.BodyParser(&updates); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "参数错误"})
		}
		if err := db.Model(&models.SLARule{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"message": "已更新"})
	})

	sla.Delete("/rules/:id", adminOnly, func(c *fiber.Ctx) error {
		id := parseUintID(c, "id")
		db.Delete(&models.SLARule{}, id)
		return c.JSON(fiber.Map{"message": "已删除"})
	})

	sla.Get("/evaluations", func(c *fiber.Ctx) error {
		auth := middleware.GetAuthCtx(c)
		isSuper := auth.Role == "super_admin"
		runID := parseUintParam(c, "runId")
		var evals []models.SLAEvaluation
		q := db.Model(&models.SLAEvaluation{})
		if !isSuper {
			q = q.Where("tenant_id = ?", auth.TenantID)
		}
		if runID > 0 {
			q = q.Where("run_id = ?", runID)
		}
		q.Preload("Rule").Order("evaluated_at DESC").Limit(100).Find(&evals)
		return c.JSON(fiber.Map{"data": evals})
	})

	sla.Get("/stats", func(c *fiber.Ctx) error {
		auth := middleware.GetAuthCtx(c)
		isSuper := auth.Role == "super_admin"
		pid := parseUintParam(c, "pipelineId")
		days := parseIntParam(c, "days", 30)
		stats, err := engine.GetStats(auth.TenantID, isSuper, pid, days)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"data": stats})
	})

	sla.Get("/monthly-reports", func(c *fiber.Ctx) error {
		auth := middleware.GetAuthCtx(c)
		isSuper := auth.Role == "super_admin"
		pid := parseUintParam(c, "pipelineId")
		month := c.Query("month")
		reports, err := engine.GetMonthlyReports(auth.TenantID, isSuper, pid, month)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"data": reports})
	})

	sla.Post("/evaluate/:runId", adminOnly, func(c *fiber.Ctx) error {
		pid := parseUintParam(c, "pipelineId")
		rid := parseUintID(c, "runId")
		engine.EvaluateRun(pid, rid)
		return c.JSON(fiber.Map{"message": "评估已触发"})
	})
}
