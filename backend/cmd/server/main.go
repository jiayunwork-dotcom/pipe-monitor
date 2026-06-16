package main

import (
	"log"
	"pipe-monitor/internal/api/routes"
	"pipe-monitor/internal/config"
	"pipe-monitor/internal/database"
	"pipe-monitor/internal/redis"
	"pipe-monitor/internal/services"
	"pipe-monitor/internal/websocket"
)

func main() {
	cfg := config.Load()

	log.Println("Initializing database connection...")
	db, err := database.Init(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	log.Println("Initializing Redis connection...")
	rdb, err := redis.Init(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize Redis: %v", err)
	}

	if err := database.AutoMigrate(db); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	if err := database.Seed(db); err != nil {
		log.Fatalf("Failed to seed data: %v", err)
	}

	wsHub := websocket.NewHub(rdb)
	go wsHub.Run()

	alertService := services.NewAlertService(cfg, db, rdb)
	slaEngine := services.NewSLAEngine(db, rdb, alertService)
	go slaEngine.StartScheduler()
	go alertService.StartScheduler()

	app := routes.SetupRouter(cfg, db, rdb, wsHub, alertService, slaEngine)

	log.Printf("Server starting on port %s in %s mode", cfg.Server.Port, cfg.Server.Env)
	if err := app.Listen(":" + cfg.Server.Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
