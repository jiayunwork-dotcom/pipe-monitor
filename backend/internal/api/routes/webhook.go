package routes

import (
	"pipe-monitor/internal/services"
	"time"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"pipe-monitor/internal/models"
)

func setupWebhookRoutes(r fiber.Router, runService *services.RunService, db *gorm.DB) {
	r.Post("/run/:pipelineCode", func(c *fiber.Ctx) error {
		code := c.Params("pipelineCode")
		token := c.Get("X-Webhook-Token", c.Query("token"))

		var pipe models.Pipeline
		if err := db.Where("code = ?", code).First(&pipe).Error; err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "管道不存在"})
		}
		if pipe.WebhookToken != "" && pipe.WebhookToken != token {
			return c.Status(403).JSON(fiber.Map{"error": "Webhook Token无效"})
		}

		var payload struct {
			RunID        string     `json:"runId"`
			Status       string     `json:"status"`
			TriggerType  string     `json:"triggerType"`
			TriggeredBy  string     `json:"triggeredBy"`
			ActualStart  *time.Time `json:"actualStart"`
			ActualEnd    *time.Time `json:"actualEnd"`
			DurationSec  int        `json:"durationSec"`
			ErrorMessage string     `json:"errorMessage"`
			DataVolume   int64      `json:"dataVolume"`
			LogsURL      string     `json:"logsUrl"`
			ExternalURL  string     `json:"externalUrl"`
		}
		if err := c.BodyParser(&payload); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "无效的JSON: " + err.Error()})
		}
		if payload.RunID == "" {
			payload.RunID = code + "-" + time.Now().Format("20060102150405")
		}

		req := &services.ReportRunReq{
			RunID:        payload.RunID,
			PipelineCode: code,
			PipelineID:   pipe.ID,
			TenantID:     pipe.TenantID,
			Status:       payload.Status,
			TriggerType:  payload.TriggerType,
			TriggeredBy:  payload.TriggeredBy,
			ActualStart:  payload.ActualStart,
			ActualEnd:    payload.ActualEnd,
			DurationSec:  payload.DurationSec,
			ErrorMessage: payload.ErrorMessage,
			DataVolume:   payload.DataVolume,
			LogsURL:      payload.LogsURL,
			ExternalURL:  payload.ExternalURL,
		}
		run, err := runService.ReportRun(req)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{
			"success": true,
			"message": "状态已接收",
			"data": fiber.Map{
				"runId":  run.RunID,
				"status": run.Status,
			},
		})
	})

	r.Post("/generic", func(c *fiber.Ctx) error {
		var payload struct {
			PipelineCode string `json:"pipelineCode"`
			PipelineID   uint   `json:"pipelineId"`
			Token        string `json:"token"`
			RunID        string `json:"runId"`
			Status       string `json:"status"`
		}
		if err := c.BodyParser(&payload); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "无效的JSON"})
		}
		req := &services.ReportRunReq{
			RunID:        payload.RunID,
			PipelineCode: payload.PipelineCode,
			PipelineID:   payload.PipelineID,
			Token:        payload.Token,
			Status:       payload.Status,
		}
		run, err := runService.ReportRun(req)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"success": true, "runId": run.RunID})
	})
}
