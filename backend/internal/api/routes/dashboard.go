package routes

import (
	"pipe-monitor/internal/middleware"
	"pipe-monitor/internal/services"
	"github.com/gofiber/fiber/v2"
)

func setupDashboardRoutes(r fiber.Router, runService *services.RunService) {
	dash := r.Group("/dashboard")

	dash.Get("/overview", func(c *fiber.Ctx) error {
		auth := middleware.GetAuthCtx(c)
		isSuper := auth.Role == "super_admin"
		stats, err := runService.GetDashboardStats(auth.TenantID, isSuper)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"data": stats})
	})
}
