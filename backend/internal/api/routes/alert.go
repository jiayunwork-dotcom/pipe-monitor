package routes

import (
	"pipe-monitor/internal/middleware"
	"pipe-monitor/internal/models"
	"pipe-monitor/internal/services"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func setupAlertRoutes(r fiber.Router, svc *services.AlertService, db *gorm.DB) {
	alerts := r.Group("/alerts")
	adminOnly := middleware.AdminRequired()

	alerts.Get("", func(c *fiber.Ctx) error {
		auth := middleware.GetAuthCtx(c)
		isSuper := auth.Role == "super_admin"
		status := c.Query("status")
		severity := c.Query("severity")
		days := parseIntParam(c, "days", 30)
		page := parseIntParam(c, "page", 1)
		size := parseIntParam(c, "pageSize", 20)
		result, err := svc.List(auth.TenantID, isSuper, status, severity, days, page, size)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(result)
	})

	alerts.Post("/:id/acknowledge", func(c *fiber.Ctx) error {
		auth := middleware.GetAuthCtx(c)
		id := parseUintID(c, "id")
		var req struct {
			Note string `json:"note"`
		}
		c.BodyParser(&req)
		if err := svc.Acknowledge(id, auth.UserID, req.Note); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"message": "已认领"})
	})

	alerts.Post("/:id/resolve", func(c *fiber.Ctx) error {
		auth := middleware.GetAuthCtx(c)
		id := parseUintID(c, "id")
		var req struct {
			Note string `json:"note"`
		}
		c.BodyParser(&req)
		if err := svc.Resolve(id, auth.UserID, req.Note); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"message": "已关闭"})
	})

	alerts.Get("/rules", func(c *fiber.Ctx) error {
		auth := middleware.GetAuthCtx(c)
		isSuper := auth.Role == "super_admin"
		pid := parseUintParam(c, "pipelineId")
		var rules []models.AlertRule
		q := db.Model(&models.AlertRule{})
		if !isSuper {
			q = q.Where("tenant_id = ?", auth.TenantID)
		}
		if pid > 0 {
			q = q.Where("pipeline_id = ? OR pipeline_id IS NULL", pid)
		}
		q.Order("created_at DESC").Find(&rules)
		return c.JSON(fiber.Map{"data": rules})
	})

	alerts.Post("/rules", adminOnly, func(c *fiber.Ctx) error {
		auth := middleware.GetAuthCtx(c)
		var rule models.AlertRule
		if err := c.BodyParser(&rule); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "参数错误: " + err.Error()})
		}
		rule.TenantID = auth.TenantID
		if err := db.Create(&rule).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"data": rule})
	})

	alerts.Put("/rules/:id", adminOnly, func(c *fiber.Ctx) error {
		id := parseUintID(c, "id")
		var updates map[string]interface{}
		if err := c.BodyParser(&updates); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "参数错误"})
		}
		db.Model(&models.AlertRule{}).Where("id = ?", id).Updates(updates)
		return c.JSON(fiber.Map{"message": "已更新"})
	})

	alerts.Delete("/rules/:id", adminOnly, func(c *fiber.Ctx) error {
		id := parseUintID(c, "id")
		db.Delete(&models.AlertRule{}, id)
		return c.JSON(fiber.Map{"message": "已删除"})
	})

	alerts.Get("/:id/notifications", func(c *fiber.Ctx) error {
		id := parseUintID(c, "id")
		var notifs []models.AlertNotification
		db.Where("alert_id = ?", id).Order("sent_at DESC").Find(&notifs)
		return c.JSON(fiber.Map{"data": notifs})
	})
}
