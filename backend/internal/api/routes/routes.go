package routes

import (
	"pipe-monitor/internal/config"
	"pipe-monitor/internal/middleware"
	"pipe-monitor/internal/services"
	"pipe-monitor/internal/websocket"

	wsFiber "github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"

	redisClient "pipe-monitor/internal/redis"
)

func SetupRouter(
	cfg *config.Config,
	db *gorm.DB,
	rdb *redisClient.Client,
	wsHub *websocket.Hub,
	alertService *services.AlertService,
	slaEngine *services.SLAEngine,
) *fiber.App {
	app := fiber.New(fiber.Config{
		BodyLimit:    50 * 1024 * 1024,
		ReadTimeout:  300,
		WriteTimeout: 300,
	})

	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${status} - ${latency} ${method} ${path}\n",
	}))
	app.Use(cors.New(cors.Config{
		AllowOriginsFunc: func(origin string) bool {
			return true
		},
		AllowMethods:     "GET,POST,PUT,DELETE,PATCH,OPTIONS",
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization,X-Webhook-Token",
		AllowCredentials: true,
		MaxAge:           3600,
	}))

	wsPublisher := func(tenantID uint, msgType string, payload interface{}) {
		wsHub.BroadcastToTenant(tenantID, msgType, payload)
	}
	alertService.SetWSPublisher(wsPublisher)

	pipelineService := services.NewPipelineService(db)
	runService := services.NewRunService(db, rdb, alertService)
	runService.SetWSPublisher(wsPublisher)
	runService.SetSLAEngine(slaEngine)
	onCallService := services.NewOnCallService(db)

	api := app.Group("/api")
	api.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "pipe-monitor"})
	})

	setupAuthRoutes(api, cfg, db)

	authMW := middleware.AuthMiddleware(cfg, db)
	apiV1 := api.Group("/v1", authMW)
	setupDashboardRoutes(apiV1, runService)
	setupPipelineRoutes(apiV1, pipelineService, runService)
	setupRunRoutes(apiV1, runService)
	setupSLARoutes(apiV1, slaEngine, db)
	setupAlertRoutes(apiV1, alertService, db)
	setupOnCallRoutes(apiV1, onCallService, db)
	setupTenantRoutes(apiV1, db)
	setupUserRoutes(apiV1, db)

	webhookGroup := app.Group("/webhook")
	setupWebhookRoutes(webhookGroup, runService, db)

	app.Use("/ws", func(c *fiber.Ctx) error {
		if wsFiber.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	app.Get("/ws", wsFiber.New(func(conn *wsFiber.Conn) {
		tokenStr := conn.Query("token")
		var tenantID uint
		var userID uint
		if tokenStr != "" {
			claims, err := parseToken(cfg, tokenStr)
			if err == nil {
				tenantID = claims.TenantID
				userID = claims.UserID
			}
		}
		client := &websocket.Client{
			ID:       services.GenerateWSID(),
			TenantID: tenantID,
			UserID:   userID,
			Conn:     conn,
			Hub:      wsHub,
			Send:     make(chan []byte, 256),
		}
		wsHub.Register <- client
		defer func() { wsHub.Unregister <- client }()
		go client.WritePump()
		client.ReadPump()
	}))

	return app
}

func parseToken(cfg *config.Config, tokenStr string) (*middleware.Claims, error) {
	claims := &middleware.Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(cfg.JWT.Secret), nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, jwt.ErrSignatureInvalid
	}
	return claims, nil
}
