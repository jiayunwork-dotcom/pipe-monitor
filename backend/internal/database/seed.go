package database

import (
	"fmt"
	"pipe-monitor/internal/models"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func Seed(db *gorm.DB) error {
	var count int64
	db.Model(&models.Tenant{}).Count(&count)
	if count > 0 {
		return nil
	}

	defaultTenant := models.Tenant{
		Name:        "platform",
		DisplayName: "平台超级管理员租户",
		Description: "超级管理员所在租户，可全局查看所有管道",
		Status:      "active",
	}
	if err := db.Create(&defaultTenant).Error; err != nil {
		return err
	}

	businessTenant := models.Tenant{
		Name:        "data_bi",
		DisplayName: "数据分析业务线",
		Description: "数据分析团队业务租户",
		Status:      "active",
	}
	if err := db.Create(&businessTenant).Error; err != nil {
		return err
	}

	dataEngTenant := models.Tenant{
		Name:        "data_eng",
		DisplayName: "数据工程业务线",
		Description: "数据工程团队业务租户",
		Status:      "active",
	}
	if err := db.Create(&dataEngTenant).Error; err != nil {
		return err
	}

	superPwd, _ := hashPassword("Super@2024!")
	superUser := models.User{
		TenantID:     defaultTenant.ID,
		Username:     "superadmin",
		Email:        "superadmin@example.com",
		PasswordHash: superPwd,
		FullName:     "超级管理员",
		Role:         models.RoleSuperAdmin,
		Status:       "active",
	}
	if err := db.Create(&superUser).Error; err != nil {
		return err
	}

	adminPwd, _ := hashPassword("Admin@2024!")
	biAdmin := models.User{
		TenantID:     businessTenant.ID,
		Username:     "bi_admin",
		Email:        "bi_admin@example.com",
		PasswordHash: adminPwd,
		FullName:     "BI管理员",
		Role:         models.RoleAdmin,
		Status:       "active",
	}
	if err := db.Create(&biAdmin).Error; err != nil {
		return err
	}

	memberPwd, _ := hashPassword("User@2024!")
	biMember := models.User{
		TenantID:     businessTenant.ID,
		Username:     "bi_member",
		Email:        "bi_member@example.com",
		PasswordHash: memberPwd,
		FullName:     "BI普通成员",
		Role:         models.RoleMember,
		Status:       "active",
	}
	if err := db.Create(&biMember).Error; err != nil {
		return err
	}

	mysqlSource := models.DataSource{
		TenantID:    dataEngTenant.ID,
		Name:        "订单中心MySQL",
		Type:        "mysql",
		Host:        "mysql-order.internal",
		Port:        3306,
		Database:    "order_db",
		Description: "订单业务核心数据库",
	}
	if err := db.Create(&mysqlSource).Error; err != nil {
		return err
	}

	hiveSink := models.DataSource{
		TenantID:    dataEngTenant.ID,
		Name:        "数仓ODS层Hive",
		Type:        "hive",
		Host:        "hive-metastore.internal",
		Database:    "ods",
		Description: "数仓ODS层",
	}
	if err := db.Create(&hiveSink).Error; err != nil {
		return err
	}

	now := time.Now()

	pipe1 := models.Pipeline{
		TenantID:       dataEngTenant.ID,
		Name:           "订单数据ODS抽取",
		Code:           "ods_order_extract",
		Description:    "从订单中心MySQL抽取数据到Hive ODS层",
		DataDomain:     "交易域",
		SourceID:       &mysqlSource.ID,
		SourceDetail:   "order_db.t_order 增量抽取",
		TargetID:       &hiveSink.ID,
		TargetDetail:   "ods.ods_order_incr 按天分区",
		ScheduleFreq:   models.ScheduleDaily,
		CronExpression: "0 30 1 * * ?",
		OwnerID:        biAdmin.ID,
		Team:           "数据工程",
		Status:         models.PipelineActive,
		ExpectedRunSec: 1200,
	}
	if err := db.Create(&pipe1).Error; err != nil {
		return err
	}

	pipe2 := models.Pipeline{
		TenantID:       dataEngTenant.ID,
		Name:           "订单数据DWD清洗",
		Code:           "dwd_order_clean",
		Description:    "清洗订单ODS数据写入DWD层",
		DataDomain:     "交易域",
		SourceDetail:   "ods.ods_order_incr",
		TargetDetail:   "dwd.dwd_order_detail",
		ScheduleFreq:   models.ScheduleDaily,
		CronExpression: "0 0 3 * * ?",
		OwnerID:        biAdmin.ID,
		Team:           "数据工程",
		Status:         models.PipelineActive,
		ExpectedRunSec: 1800,
	}
	if err := db.Create(&pipe2).Error; err != nil {
		return err
	}

	pipe3 := models.Pipeline{
		TenantID:       businessTenant.ID,
		Name:           "GMV日报报表计算",
		Code:           "ads_daily_gmv",
		Description:    "计算每日GMV报表指标",
		DataDomain:     "交易域",
		SourceDetail:   "dwd.dwd_order_detail",
		TargetDetail:   "ads.ads_gmv_daily",
		ScheduleFreq:   models.ScheduleDaily,
		CronExpression: "0 30 5 * * ?",
		OwnerID:        biAdmin.ID,
		Team:           "数据分析",
		Status:         models.PipelineActive,
		ExpectedRunSec: 600,
	}
	if err := db.Create(&pipe3).Error; err != nil {
		return err
	}

	dep1 := models.PipelineDependency{
		TenantID:       dataEngTenant.ID,
		PipelineID:     pipe2.ID,
		UpstreamID:     pipe1.ID,
		DependencyType: "hard",
	}
	if err := db.Create(&dep1).Error; err != nil {
		return err
	}

	dep2 := models.PipelineDependency{
		TenantID:       businessTenant.ID,
		PipelineID:     pipe3.ID,
		UpstreamID:     pipe2.ID,
		DependencyType: "hard",
	}
	if err := db.Create(&dep2).Error; err != nil {
		return err
	}

	sla1 := models.SLARule{
		TenantID:           dataEngTenant.ID,
		PipelineID:         pipe1.ID,
		Name:               "订单ODS抽取工作日6点前完成",
		RuleType:           models.SLAFinishByTime,
		DateType:           models.DateWorkday,
		FinishDeadlineTime: "06:00",
		WarnThresholdSec:   1800,
		AlertSeverity:      "critical",
		Enabled:            true,
	}
	if err := db.Create(&sla1).Error; err != nil {
		return err
	}

	sla2 := models.SLARule{
		TenantID:           dataEngTenant.ID,
		PipelineID:         pipe1.ID,
		Name:               "单次运行不超过45分钟",
		RuleType:           models.SLAMaxDuration,
		DateType:           models.DateAny,
		MaxDurationSec:     2700,
		WarnThresholdSec:   300,
		AlertSeverity:      "warning",
		Enabled:            true,
	}
	if err := db.Create(&sla2).Error; err != nil {
		return err
	}

	sla3 := models.SLARule{
		TenantID:           businessTenant.ID,
		PipelineID:         pipe3.ID,
		Name:               "GMV报表工作日8点前产出",
		RuleType:           models.SLAFinishByTime,
		DateType:           models.DateWorkday,
		FinishDeadlineTime: "08:00",
		WarnThresholdSec:   1800,
		AlertSeverity:      "critical",
		Enabled:            true,
	}
	if err := db.Create(&sla3).Error; err != nil {
		return err
	}

	alertRule1 := models.AlertRule{
		TenantID:       dataEngTenant.ID,
		PipelineID:     &pipe1.ID,
		Name:           "订单抽取连续失败3次告警",
		RuleType:       models.AlertConsecutiveFail,
		ConsecutiveFailN: 3,
		Severity:       models.AlertCritical,
		Channels:       `["feishu","email"]`,
		NotifyOnCall:   true,
		Enabled:        true,
	}
	if err := db.Create(&alertRule1).Error; err != nil {
		return err
	}

	onCallGroup := models.OnCallGroup{
		TenantID:     dataEngTenant.ID,
		Name:         "数据工程值班组",
		Description:  "负责数据工程团队所有管道的值班响应",
		RotationMode: models.RotationWeekly,
		Timezone:     "Asia/Shanghai",
		StartDate:    now,
		Members:      fmt.Sprintf(`[%d,%d]`, biAdmin.ID, biMember.ID),
	}
	if err := db.Create(&onCallGroup).Error; err != nil {
		return err
	}

	occ := models.OnCallAssignment{
		TenantID:   dataEngTenant.ID,
		GroupID:    onCallGroup.ID,
		PipelineID: &pipe1.ID,
		UserID:     biAdmin.ID,
		StartDate:  now,
		EndDate:    now.AddDate(0, 0, 7),
		ShiftType:  "primary",
	}
	if err := db.Create(&occ).Error; err != nil {
		return err
	}

	return nil
}

func hashPassword(pwd string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(pwd), bcrypt.DefaultCost)
	return string(bytes), err
}
