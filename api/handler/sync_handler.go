// 同步管理 HTTP 处理器
// 薄层设计：提供同步状态查询和手动触发接口

package handler

import (
	"yomirrorsite/internal/model"
	"yomirrorsite/internal/syncer"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
	"yomirrorsite/internal/util"
)

// ============================================================
// 处理器结构
// ============================================================

// SyncHandler 同步管理 HTTP 处理器
type SyncHandler struct {
	scheduler *syncer.Scheduler
}

// NewSyncHandler 创建同步处理器
func NewSyncHandler(scheduler *syncer.Scheduler) *SyncHandler {
	return &SyncHandler{scheduler: scheduler}
}

// ============================================================
// 路由处理方法
// ============================================================

// GetStatus 获取同步状态
// GET /api/sync/status
// 返回当前同步进度、最近同步结果等
func (h *SyncHandler) GetStatus(c *fiber.Ctx) error {
	util.Debug("同步状态请求", util.Module("handler"))
	status := h.scheduler.GetStatus()
	return c.Status(fiber.StatusOK).JSON(model.APIResponse{
		Success: true,
		Data:    status,
	})
}

// TriggerSync 手动触发同步
// POST /api/sync/trigger
// Body: {"software_id": "vscode"}  空则同步全部
func (h *SyncHandler) TriggerSync(c *fiber.Ctx) error {
	util.Info("手动同步触发", util.Module("handler"))
	// 解析请求体
	var req struct {
		SoftwareID string `json:"software_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		// 空 body 或解析失败时触发全量同步
		req.SoftwareID = ""
	}

	if err := h.scheduler.TriggerSync(c.Context(), req.SoftwareID); err != nil {
		util.Error("触发同步失败",
			zap.String("software", req.SoftwareID),
			zap.Error(err))
		return c.Status(fiber.StatusBadRequest).JSON(model.APIResponse{
			Success: false,
			Error:   "触发同步失败: " + err.Error(),
		})
	}

	msg := "全量同步已触发"
	if req.SoftwareID != "" {
		msg = "软件 " + req.SoftwareID + " 的同步已触发"
	}

	util.Info(msg)
	return c.Status(fiber.StatusAccepted).JSON(model.APIResponse{
		Success: true,
		Data:    map[string]string{"message": msg},
	})
}
