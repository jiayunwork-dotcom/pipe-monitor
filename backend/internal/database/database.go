package database

import (
	"fmt"
	"pipe-monitor/internal/config"
	"pipe-monitor/internal/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Init(cfg *config.Config) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=Asia/Shanghai",
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.Name,
		cfg.Database.SSLMode,
	)

	gormCfg := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	}

	db, err := gorm.Open(postgres.Open(dsn), gormCfg)
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)

	return db, nil
}

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.Tenant{},
		&models.User{},
		&models.DataSource{},
		&models.Pipeline{},
		&models.PipelineDependency{},
		&models.PipelineRun{},
		&models.SLARule{},
		&models.SLAEvaluation{},
		&models.SLAMonthlyReport{},
		&models.AlertRule{},
		&models.AlertEvent{},
		&models.AlertNotification{},
		&models.OnCallGroup{},
		&models.OnCallAssignment{},
		&models.HandoverSummary{},
		&models.AuditLog{},
		&models.Holiday{},
		&models.ApiToken{},
	)
}
