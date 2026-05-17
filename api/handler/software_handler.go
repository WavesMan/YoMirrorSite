package handler

import (
	"strconv"

	"yomirrorsite/internal/model"
	"yomirrorsite/internal/service"
	"yomirrorsite/internal/util"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

type SoftwareHandler struct {
	softwareService *service.SoftwareService
}

func NewSoftwareHandler(svc *service.SoftwareService) *SoftwareHandler {
	return &SoftwareHandler{softwareService: svc}
}

func (h *SoftwareHandler) ListSoftware(c *fiber.Ctx) error {
	category := c.Query("category", "")
	keyword := c.Query("keyword", "")
	page, _ := strconv.Atoi(c.Query("page", "1"))
	pageSize, _ := strconv.Atoi(c.Query("page_size", "20"))

	util.Debug("软件列表请求", util.Module("handler"), zap.Int("page", page), zap.Int("size", pageSize))

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

func (h *SoftwareHandler) GetVersion(c *fiber.Ctx) error {
	util.Info("版本详情请求", util.Module("handler"), util.Software(c.Params("id")), zap.String("tag", c.Params("tag")))
	softwareID := c.Params("id")
	tagName := c.Params("tag")

	expires := int64(3600)
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

func (h *SoftwareHandler) GetDownloadURL(c *fiber.Ctx) error {
	util.Info("下载请求", util.Module("handler"), util.Software(c.Params("id")), zap.String("asset", c.Params("asset")))
	softwareID := c.Params("id")
	tagName := c.Params("tag")
	assetName := c.Params("asset")

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

	return c.Redirect(url, fiber.StatusFound)
}

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