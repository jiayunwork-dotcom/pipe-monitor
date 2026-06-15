package middleware

import (
	"errors"
	"fmt"
	"pipe-monitor/internal/config"
	"pipe-monitor/internal/models"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

type Claims struct {
	UserID   uint           `json:"userId"`
	TenantID uint           `json:"tenantId"`
	Username string         `json:"username"`
	Role     models.UserRole `json:"role"`
	jwt.RegisteredClaims
}

type AuthContext struct {
	UserID   uint
	TenantID uint
	Username string
	Role     models.UserRole
}

func GetAuthCtx(c *fiber.Ctx) *AuthContext {
	user, ok := c.Locals("auth").(*AuthContext)
	if !ok {
		return nil
	}
	return user
}

func GenerateToken(cfg *config.Config, user *models.User) (string, error) {
	expirationTime := time.Now().Add(time.Duration(cfg.JWT.ExpireHour) * time.Hour)
	claims := &Claims{
		UserID:   user.ID,
		TenantID: user.TenantID,
		Username: user.Username,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "pipe-monitor",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.JWT.Secret))
}

func AuthMiddleware(cfg *config.Config, db *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Missing authorization header",
			})
		}

		const prefix = "Bearer "
		if len(authHeader) <= len(prefix) || authHeader[:len(prefix)] != prefix {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid authorization header format",
			})
		}
		tokenStr := authHeader[len(prefix):]

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(cfg.JWT.Secret), nil
		})

		if err != nil || !token.Valid {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid or expired token",
			})
		}

		var user models.User
		if err := db.First(&user, claims.UserID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "User not found"})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Database error"})
		}

		if user.Status != "active" {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "User account is disabled"})
		}

		authCtx := &AuthContext{
			UserID:   user.ID,
			TenantID: user.TenantID,
			Username: user.Username,
			Role:     user.Role,
		}
		c.Locals("auth", authCtx)

		return c.Next()
	}
}

func AdminRequired() fiber.Handler {
	return func(c *fiber.Ctx) error {
		auth := GetAuthCtx(c)
		if auth == nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
		}
		if auth.Role != models.RoleAdmin && auth.Role != models.RoleSuperAdmin {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Admin role required"})
		}
		return c.Next()
	}
}

func SuperAdminRequired() fiber.Handler {
	return func(c *fiber.Ctx) error {
		auth := GetAuthCtx(c)
		if auth == nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
		}
		if auth.Role != models.RoleSuperAdmin {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Super admin role required"})
		}
		return c.Next()
	}
}
