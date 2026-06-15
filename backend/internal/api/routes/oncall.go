package routes

import (
	"pipe-monitor/internal/middleware"
	"pipe-monitor/internal/models"
	"pipe-monitor/internal/services"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func setupOnCallRoutes(r fiber.Router, svc *services.OnCallService, db *gorm.DB) {
	oc := r.Group("/oncall")
	adminOnly := middleware.AdminRequired()

	oc.Get("/groups", func(c *fiber.Ctx) error {
		auth := middleware.GetAuthCtx(c)
		isSuper := auth.Role == "super_admin"
		groups, err := svc.GetGroups(auth.TenantID, isSuper)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"data": groups})
	})

	oc.Post("/groups", adminOnly, func(c *fiber.Ctx) error {
		auth := middleware.GetAuthCtx(c)
		var req struct {
			Name         string              `json:"name"`
			Description  string              `json:"description"`
			RotationMode models.RotationMode `json:"rotationMode"`
			Timezone     string              `json:"timezone"`
			StartDate    string              `json:"startDate"`
			MemberIDs    []uint              `json:"memberIds"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "参数错误: " + err.Error()})
		}
		sd, err := time.Parse(time.RFC3339, req.StartDate)
		if err != nil {
			sd = time.Now()
		}
		createReq := &services.CreateGroupReq{
			TenantID:     auth.TenantID,
			Name:         req.Name,
			Description:  req.Description,
			RotationMode: req.RotationMode,
			Timezone:     req.Timezone,
			StartDate:    sd,
			MemberIDs:    req.MemberIDs,
		}
		group, err := svc.CreateGroup(createReq)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"data": group})
	})

	oc.Get("/groups/:id/assignments", func(c *fiber.Ctx) error {
		id := parseUintID(c, "id")
		days := parseIntParam(c, "days", 30)
		occs, err := svc.GetAssignments(id, days)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"data": occs})
	})

	oc.Get("/groups/:id/current", func(c *fiber.Ctx) error {
		id := parseUintID(c, "id")
		pid := parseUintParam(c, "pipelineId")
		var ppid *uint
		if pid > 0 {
			ppid = &pid
		}
		occ, err := svc.GetCurrentAssignment(id, ppid)
		if err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "无当前值班人"})
		}
		return c.JSON(fiber.Map{"data": occ})
	})

	oc.Post("/groups/:id/handover", adminOnly, func(c *fiber.Ctx) error {
		auth := middleware.GetAuthCtx(c)
		gid := parseUintID(c, "id")
		var req struct {
			ToUserID uint   `json:"toUserId"`
			Notes    string `json:"notes"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "参数错误"})
		}
		handover, err := svc.CreateHandover(gid, auth.UserID, req.ToUserID, req.Notes)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"data": handover})
	})

	oc.Get("/groups/:id/handovers", func(c *fiber.Ctx) error {
		id := parseUintID(c, "id")
		limit := parseIntParam(c, "limit", 20)
		list, err := svc.GetHandovers(id, limit)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"data": list})
	})

	oc.Get("/me", func(c *fiber.Ctx) error {
		auth := middleware.GetAuthCtx(c)
		now := time.Now()
		var occs []models.OnCallAssignment
		db.Preload("Group").Where("user_id = ? AND start_date <= ? AND end_date >= ?",
			auth.UserID, now, now).Find(&occs)
		return c.JSON(fiber.Map{"data": occs})
	})
}
