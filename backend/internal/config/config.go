package config

import (
	"os"
	"strconv"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	JWT      JWTConfig
	Alert    AlertConfig
}

type ServerConfig struct {
	Port string
	Env  string
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type JWTConfig struct {
	Secret     string
	ExpireHour int
}

type AlertConfig struct {
	SMTPHost     string
	SMTPPort     int
	SMTPUser     string
	SMTPPassword string
	SMTPFrom     string
	FeishuWebhook string
	DingTalkWebhook string
	SlackWebhook  string
}

func Load() *Config {
	db, _ := strconv.Atoi(getEnv("REDIS_DB", "0"))
	expireHour, _ := strconv.Atoi(getEnv("JWT_EXPIRE_HOUR", "24"))
	smtpPort, _ := strconv.Atoi(getEnv("SMTP_PORT", "587"))

	return &Config{
		Server: ServerConfig{
			Port: getEnv("SERVER_PORT", "8080"),
			Env:  getEnv("GO_ENV", "development"),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "pipe_admin"),
			Password: getEnv("DB_PASSWORD", "pipe_monitor_pwd_2024"),
			Name:     getEnv("DB_NAME", "pipe_monitor"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		Redis: RedisConfig{
			Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       db,
		},
		JWT: JWTConfig{
			Secret:     getEnv("JWT_SECRET", "pipe-monitor-super-secret-key-change-in-production"),
			ExpireHour: expireHour,
		},
		Alert: AlertConfig{
			SMTPHost:        getEnv("SMTP_HOST", ""),
			SMTPPort:        smtpPort,
			SMTPUser:        getEnv("SMTP_USER", ""),
			SMTPPassword:    getEnv("SMTP_PASSWORD", ""),
			SMTPFrom:        getEnv("SMTP_FROM", ""),
			FeishuWebhook:   getEnv("FEISHU_WEBHOOK", ""),
			DingTalkWebhook: getEnv("DINGTALK_WEBHOOK", ""),
			SlackWebhook:    getEnv("SLACK_WEBHOOK", ""),
		},
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
