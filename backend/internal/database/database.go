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
		Logger:                                   logger.Default.LogMode(logger.Info),
		DisableForeignKeyConstraintWhenMigrating: true,
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
	if err := db.AutoMigrate(
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
	); err != nil {
		return err
	}

	fkStatements := []string{
		`ALTER TABLE IF EXISTS users ADD CONSTRAINT IF NOT EXISTS fk_users_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE`,
		`ALTER TABLE IF EXISTS pipelines ADD CONSTRAINT IF NOT EXISTS fk_pipelines_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE`,
		`ALTER TABLE IF EXISTS pipelines ADD CONSTRAINT IF NOT EXISTS fk_pipelines_owner FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE RESTRICT`,
		`ALTER TABLE IF EXISTS pipelines ADD CONSTRAINT IF NOT EXISTS fk_pipelines_source FOREIGN KEY (source_id) REFERENCES data_sources(id) ON DELETE SET NULL`,
		`ALTER TABLE IF EXISTS pipelines ADD CONSTRAINT IF NOT EXISTS fk_pipelines_target FOREIGN KEY (target_id) REFERENCES data_sources(id) ON DELETE SET NULL`,
		`ALTER TABLE IF EXISTS pipeline_dependencies ADD CONSTRAINT IF NOT EXISTS fk_deps_pipe FOREIGN KEY (pipeline_id) REFERENCES pipelines(id) ON DELETE CASCADE`,
		`ALTER TABLE IF EXISTS pipeline_dependencies ADD CONSTRAINT IF NOT EXISTS fk_deps_upstream FOREIGN KEY (upstream_id) REFERENCES pipelines(id) ON DELETE CASCADE`,
		`ALTER TABLE IF EXISTS pipeline_runs ADD CONSTRAINT IF NOT EXISTS fk_runs_pipe FOREIGN KEY (pipeline_id) REFERENCES pipelines(id) ON DELETE CASCADE`,
		`ALTER TABLE IF EXISTS pipeline_runs ADD CONSTRAINT IF NOT EXISTS fk_runs_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE`,
		`ALTER TABLE IF EXISTS sla_rules ADD CONSTRAINT IF NOT EXISTS fk_sla_rules_pipe FOREIGN KEY (pipeline_id) REFERENCES pipelines(id) ON DELETE CASCADE`,
		`ALTER TABLE IF EXISTS sla_evaluations ADD CONSTRAINT IF NOT EXISTS fk_sla_eval_run FOREIGN KEY (run_id) REFERENCES pipeline_runs(id) ON DELETE CASCADE`,
		`ALTER TABLE IF EXISTS sla_evaluations ADD CONSTRAINT IF NOT EXISTS fk_sla_eval_rule FOREIGN KEY (rule_id) REFERENCES sla_rules(id) ON DELETE CASCADE`,
		`ALTER TABLE IF EXISTS sla_monthly_reports ADD CONSTRAINT IF NOT EXISTS fk_sla_rep_pipe FOREIGN KEY (pipeline_id) REFERENCES pipelines(id) ON DELETE CASCADE`,
		`ALTER TABLE IF EXISTS alert_events ADD CONSTRAINT IF NOT EXISTS fk_alert_ev_rule FOREIGN KEY (rule_id) REFERENCES alert_rules(id) ON DELETE SET NULL`,
		`ALTER TABLE IF EXISTS alert_events ADD CONSTRAINT IF NOT EXISTS fk_alert_ev_pipe FOREIGN KEY (pipeline_id) REFERENCES pipelines(id) ON DELETE SET NULL`,
		`ALTER TABLE IF EXISTS alert_events ADD CONSTRAINT IF NOT EXISTS fk_alert_ev_ack FOREIGN KEY (acknowledged_by_id) REFERENCES users(id) ON DELETE SET NULL`,
		`ALTER TABLE IF EXISTS alert_events ADD CONSTRAINT IF NOT EXISTS fk_alert_ev_res FOREIGN KEY (resolved_by_id) REFERENCES users(id) ON DELETE SET NULL`,
		`ALTER TABLE IF EXISTS alert_notifications ADD CONSTRAINT IF NOT EXISTS fk_alert_notif_alert FOREIGN KEY (alert_id) REFERENCES alert_events(id) ON DELETE CASCADE`,
		`ALTER TABLE IF EXISTS on_call_assignments ADD CONSTRAINT IF NOT EXISTS fk_occ_group FOREIGN KEY (group_id) REFERENCES on_call_groups(id) ON DELETE CASCADE`,
		`ALTER TABLE IF EXISTS on_call_assignments ADD CONSTRAINT IF NOT EXISTS fk_occ_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE`,
		`ALTER TABLE IF EXISTS on_call_assignments ADD CONSTRAINT IF NOT EXISTS fk_occ_pipe FOREIGN KEY (pipeline_id) REFERENCES pipelines(id) ON DELETE SET NULL`,
		`ALTER TABLE IF EXISTS handover_summaries ADD CONSTRAINT IF NOT EXISTS fk_handover_from FOREIGN KEY (from_user_id) REFERENCES users(id) ON DELETE RESTRICT`,
		`ALTER TABLE IF EXISTS handover_summaries ADD CONSTRAINT IF NOT EXISTS fk_handover_to FOREIGN KEY (to_user_id) REFERENCES users(id) ON DELETE RESTRICT`,
		`ALTER TABLE IF EXISTS audit_logs ADD CONSTRAINT IF NOT EXISTS fk_audit_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL`,
	}

	for _, sql := range fkStatements {
		if err := db.Exec(sql).Error; err != nil {
			return err
		}
	}

	return nil
}
