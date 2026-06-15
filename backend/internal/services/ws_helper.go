package services

import (
	"pipe-monitor/internal/utils"
)

func GenerateWSID() string {
	return "ws-" + utils.GenerateToken(16)
}
