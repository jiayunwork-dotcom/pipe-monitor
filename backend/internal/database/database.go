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

	fkDefinitions := []struct {
		table      string
		constraint string
		column     string
		refTable   string
		refColumn  string
		onDelete   string
	}{
		{"users", "fk_users_tenant", "tenant_id", "tenants", "id", "CASCADE"},
		{"pipelines", "fk_pipelines_tenant", "tenant_id", "tenants", "id", "CASCADE"},
		{"pipelines", "fk_pipelines_owner", "owner_id", "users", "id", "RESTRICT"},
		{"pipelines", "fk_pipelines_source", "source_id", "data_sources", "id", "SET NULL"},
		{"pipelines", "fk_pipelines_target", "target_id", "data_sources", "id", "SET NULL"},
		{"pipeline_dependencies", "fk_deps_pipe", "pipeline_id", "pipelines", "id", "CASCADE"},
		{"pipeline_dependencies", "fk_deps_upstream", "upstream_id", "pipelines", "id", "CASCADE"},
		{"pipeline_runs", "fk_runs_pipe", "pipeline_id", "pipelines", "id", "CASCADE"},
		{"sla_rules", "fk_sla_rules_pipe", "pipeline_id", "pipelines", "id", "CASCADE"},
		{"sla_evaluations", "fk_sla_eval_run", "run_id", "pipeline_runs", "id", "CASCADE"},
		{"sla_evaluations", "fk_sla_eval_rule", "rule_id", "sla_rules", "id", "CASCADE"},
		{"sla_monthly_reports", "fk_sla_rep_pipe", "pipeline_id", "pipelines", "id", "CASCADE"},
		{"alert_events", "fk_alert_ev_rule", "rule_id", "alert_rules", "id", "SET NULL"},
		{"alert_events", "fk_alert_ev_pipe", "pipeline_id", "pipelines", "id", "SET NULL"},
		{"alert_events", "fk_alert_ev_ack", "acknowledged_by_id", "users", "id", "SET NULL"},
		{"alert_events", "fk_alert_ev_res", "resolved_by_id", "users", "id", "SET NULL"},
		{"alert_notifications", "fk_alert_notif_alert", "alert_id", "alert_events", "id", "CASCADE"},
		{"on_call_assignments", "fk_occ_group", "group_id", "on_call_groups", "id", "CASCADE"},
		{"on_call_assignments", "fk_occ_user", "user_id", "users", "id", "CASCADE"},
		{"on_call_assignments", "fk_occ_pipe", "pipeline_id", "pipelines", "id", "SET NULL"},
		{"handover_summaries", "fk_handover_from", "from_user_id", "users", "id", "RESTRICT"},
		{"handover_summaries", "fk_handover_to", "to_user_id", "users", "id", "RESTRICT"},
		{"audit_logs", "fk_audit_user", "user_id", "users", "id", "SET NULL"},
	}

	for _, fk := range fkDefinitions {
		var count int64
		if err := db.Raw(
			`SELECT COUNT(*) FROM information_schema.table_constraints WHERE constraint_name = ? AND table_name = ?`,
			fk.constraint, fk.table,
		).Scan(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}

		sql := fmt.Sprintf(
			`ALTER TABLE %s ADD CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s(%s) ON DELETE %s`,
			fk.table, fk.constraint, fk.column, fk.refTable, fk.refColumn, fk.onDelete,
		)
		if err := db.Exec(sql).Error; err != nil {
			return err
		}
	}

	return nil
}
