package routes

import (
	"pipe-monitor/internal/middleware"
	"pipe-monitor/internal/models"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func setupTenantRoutes(r fiber.Router, db *gorm.DB) {
	tenants := r.Group("/tenants", middleware.SuperAdminRequired())

	tenants.Get("", func(c *fiber.Ctx) error {
		var list []models.Tenant
		db.Order("name").Find(&list)
		return c.JSON(fiber.Map{"data": list})
	})

	tenants.Post("", func(c *fiber.Ctx) error {
		var tenant models.Tenant
		if err := c.BodyParser(&tenant); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "参数错误"})
		}
		if err := db.Create(&tenant).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"data": tenant})
	})

	tenants.Put("/:id", func(c *fiber.Ctx) error {
		id := parseUintID(c, "id")
		var updates map[string]interface{}
		if err := c.BodyParser(&updates); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "参数错误"})
		}
		db.Model(&models.Tenant{}).Where("id = ?", id).Updates(updates)
		return c.JSON(fiber.Map{"message": "已更新"})
	})
}

func setupUserRoutes(r fiber.Router, db *gorm.DB) {
	users := r.Group("/users")

	users.Get("", func(c *fiber.Ctx) error {
		auth := middleware.GetAuthCtx(c)
		isSuper := auth.Role == "super_admin"
		var list []models.User
		q := db.Model(&models.User{}).Select("id, tenant_id, username, email, full_name, phone, role, status, created_at")
		if !isSuper {
			q = q.Where("tenant_id = ?", auth.TenantID)
		}
		team := c.Query("team")
		_ = team
		q.Order("username").Find(&list)
		return c.JSON(fiber.Map{"data": list})
	})

	users.Get("/me", func(c *fiber.Ctx) error {
		auth := middleware.GetAuthCtx(c)
		var user models.User
		db.Preload("Tenant").First(&user, auth.UserID)
		return c.JSON(fiber.Map{"data": fiber.Map{
			"id":                  user.ID,
			"tenantId":            user.TenantID,
			"tenantName":          user.Tenant.Name,
			"tenantDisplayName":   user.Tenant.DisplayName,
			"username":            user.Username,
			"email":               user.Email,
			"fullName":            user.FullName,
			"phone":               user.Phone,
			"role":                user.Role,
			"status":              user.Status,
			"lastLoginAt":         user.LastLoginAt,
		}})
	})

	users.Post("", middleware.AdminRequired(), func(c *fiber.Ctx) error {
		auth := middleware.GetAuthCtx(c)
		var req struct {
			Username string             `json:"username"`
			Email    string             `json:"email"`
			Password string             `json:"password"`
			FullName string             `json:"fullName"`
			Phone    string             `json:"phone"`
			Role     models.UserRole    `json:"role"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "参数错误"})
		}
		return c.Status(501).JSON(fiber.Map{"error": "暂未实现创建用户"})
	})
}
