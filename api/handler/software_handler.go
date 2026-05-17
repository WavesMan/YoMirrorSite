// 软件镜像 HTTP 处理器
// 薄层设计：只做参数绑定、类型转换、调用 service、返回响应
// 不包含任何业务逻辑

package handler

import (
	"strconv"

	"yomirrorsite/internal/model"
	"yomirrorsite/internal/service"
	"yomirrorsite/internal/util"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

// ============================================================
// 处理器结构
// ============================================================

// SoftwareHandler 软件镜像 HTTP 处理器
type SoftwareHandler struct {
	softwareService *service.SoftwareService
}

// NewSoftwareHandler 创建软件处理器
func NewSoftwareHandler(svc *service.SoftwareService) *SoftwareHandler {
	return &SoftwareHandler{softwareService: svc}
}

// ============================================================
// 路由处理方法
// ============================================================

// ListSoftware 获取软件列表
// GET /api/mirror/software?category=editor&keyword=vscode&page=1&page_size=20
func (h *SoftwareHandler) ListSoftware(c *fiber.Ctx) error {
	// 参数绑定
	category := c.Query("category", "")
	keyword := c.Query("keyword", "")
	page, _ := strconv.Atoi(c.Query("page", "1"))
	pageSize, _ := strconv.Atoi(c.Query("page_size", "20"))

	util.Debug("软件列表请求", util.Module("handler"), zap.Int("page", page), zap.Int("size", pageSize))

	// 调用 service
	result, err := h.softwareService.ListSoftware(c.Context(), category, keyword, page, pageSize)
	if err != nil {
		util.Error("获取软件列表失败", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(model.APIResponse{
			Success: false,
			Error:   "获取软件列表失败: " + err.Error(),
		})
	}

	util.Debug("请求完成", util.Module("handler"), util.Status("ok"))
	return c.Status(fiber.StatusOK).JSON(model.APIResponse{
		Success: true,
		Data:    result,
	})
}

// GetSoftware 获取软件详情（含版本列表）
// GET /api/mirror/software/:id
func (h *SoftwareHandler) GetSoftware(c *fiber.Ctx) error {
	util.Info("软件详情请求", util.Module("handler"), util.Software(c.Params("id")))
	softwareID := c.Params("id")
	if softwareID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(model.APIResponse{
			Success: false,
			Error:   "缺少软件 ID",
		})
	}

	detail, err := h.softwareService.GetSoftware(c.Context(), softwareID)
	if err != nil {
		util.Error("获取软件详情失败",
			zap.String("id", softwareID),
			zap.Error(err))
		return c.Status(fiber.StatusNotFound).JSON(model.APIResponse{
			Success: false,
			Error:   "软件不存在: " + softwareID,
		})
	}

	util.Debug("请求完成", util.Module("handler"), util.Status("ok"))
	return c.Status(fiber.StatusOK).JSON(model.APIResponse{
		Success: true,
		Data:    detail,
	})
}

// GetVersion 获取单个版本详情（含下载 URL）
// GET /api/mirror/software/:id/versions/:tag?expires=3600
func (h *SoftwareHandler) GetVersion(c *fiber.Ctx) error {
	util.Info("版本详情请求", util.Module("handler"), util.Software(c.Params("id")), zap.String("tag", c.Params("tag")))
	softwareID := c.Params("id")
	tagName := c.Params("tag")

	// 解析过期时间（秒）
	expires := int64(3600) // 默认 1 小时
	if expStr := c.Query("expires"); expStr != "" {
		if val, err := strconv.ParseInt(expStr, 10, 64); err == nil && val > 0 {
			expires = val
		}
	}

	detail, err := h.softwareService.GetVersion(c.Context(), softwareID, tagName, expires)
	if err != nil {
		util.Error("获取版本详情失败",
			zap.String("software", softwareID),
			zap.String("tag", tagName),
			zap.Error(err))
		return c.Status(fiber.StatusNotFound).JSON(model.APIResponse{
			Success: false,
			Error:   "版本不存在: " + softwareID + "/" + tagName,
		})
	}

	util.Debug("请求完成", util.Module("handler"), util.Status("ok"))
	return c.Status(fiber.StatusOK).JSON(model.APIResponse{
		Success: true,
		Data:    detail,
	})
}

// GetDownloadURL 获取资产下载 URL（302 重定向）
// GET /api/mirror/software/:id/download/:tag/:asset?expires=3600
func (h *SoftwareHandler) GetDownloadURL(c *fiber.Ctx) error {
	util.Info("下载请求", util.Module("handler"), util.Software(c.Params("id")), zap.String("asset", c.Params("asset")))
	softwareID := c.Params("id")
	tagName := c.Params("tag")
	assetName := c.Params("asset")

	// 解析过期时间
	expires := int64(3600)
	if expStr := c.Query("expires"); expStr != "" {
		if val, err := strconv.ParseInt(expStr, 10, 64); err == nil && val > 0 {
			expires = val
		}
	}

	url, err := h.softwareService.GetDownloadURL(c.Context(), softwareID, tagName, assetName, expires)
	if err != nil {
		util.Error("生成下载 URL 失败",
			zap.String("software", softwareID),
			zap.String("asset", assetName),
			zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(model.APIResponse{
			Success: false,
			Error:   "生成下载链接失败",
		})
	}

	// 302 重定向到 S3 预签名 URL
	return c.Redirect(url, fiber.StatusFound)
}

// GetStats 获取镜像站统计
// GET /api/mirror/stats
func (h *SoftwareHandler) GetStats(c *fiber.Ctx) error {
	util.Info("统计请求", util.Module("handler"))
	stats, err := h.softwareService.GetStats(c.Context())
	if err != nil {
		util.Error("获取统计信息失败", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(model.APIResponse{
			Success: false,
			Error:   "获取统计信息失败",
		})
	}

	util.Debug("请求完成", util.Module("handler"), util.Status("ok"))
	return c.Status(fiber.StatusOK).JSON(model.APIResponse{
		Success: true,
		Data:    stats,
	})
}