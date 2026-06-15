package routes

import (
	"pipe-monitor/internal/middleware"
	"pipe-monitor/internal/models"
	"pipe-monitor/internal/config"
	"errors"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func setupAuthRoutes(api fiber.Router, cfg *config.Config, db *gorm.DB) {
	auth := api.Group("/auth")

	auth.Post("/login", func(c *fiber.Ctx) error {
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "无效的请求"})
		}

		var user models.User
		if err := db.Where("username = ? OR email = ?", req.Username, req.Username).
			Preload("Tenant").First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "用户名或密码错误"})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		if user.Status != "active" {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "账户已被禁用"})
		}

		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "用户名或密码错误"})
		}

		now := time.Now()
		user.LastLoginAt = &now
		db.Save(&user)

		token, err := middleware.GenerateToken(cfg, &user)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "生成令牌失败"})
		}

		return c.JSON(fiber.Map{
			"token": token,
			"user": fiber.Map{
				"id":          user.ID,
				"tenantId":    user.TenantID,
				"tenantName":  user.Tenant.Name,
				"tenantDisplayName": user.Tenant.DisplayName,
				"username":    user.Username,
				"email":       user.Email,
				"fullName":    user.FullName,
				"role":        user.Role,
			},
		})
	})

	auth.Post("/logout", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "已登出"})
	})
}

func parseIntParam(c *fiber.Ctx, key string, def int) int {
	v := c.Query(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func parseUintParam(c *fiber.Ctx, key string) uint {
	v := c.Query(key)
	n, _ := strconv.ParseUint(v, 10, 64)
	return uint(n)
}

func parseUintID(c *fiber.Ctx, key string) uint {
	v := c.Params(key)
	n, _ := strconv.ParseUint(v, 10, 64)
	return uint(n)
}
