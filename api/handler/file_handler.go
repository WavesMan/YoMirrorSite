package handler

import (
	"strconv"

	"yomirrorsite/api/model"
	"yomirrorsite/internal/service"
	"yomirrorsite/internal/util"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

// FileHandler 文件处理器
type FileHandler struct {
	fileService *service.FileService
}

// NewFileHandler 创建文件处理器
func NewFileHandler(fileService *service.FileService) *FileHandler {
	return &FileHandler{
		fileService: fileService,
	}
}

// GetFileList 获取文件列表
func (h *FileHandler) GetFileList(c *fiber.Ctx) error {
	// 获取查询参数
	prefix := c.Query("prefix")

	// 获取文件列表
	fileList, err := h.fileService.GetFileList(c.Context(), prefix)
	if err != nil {
		util.Error("Failed to get file list", zap.String("prefix", prefix), zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(model.APIResponse{
			Success: false,
			Error:   "Failed to get file list",
		})
	}

	// 转换为响应格式
	response := model.ConvertToFileListResponse(fileList)

	// 返回响应
	return c.Status(fiber.StatusOK).JSON(model.APIResponse{
		Success: true,
		Data:    response,
	})
}

// GetDownloadURL 获取下载URL
func (h *FileHandler) GetDownloadURL(c *fiber.Ctx) error {
	// 获取路径参数（使用通配符参数，可能包含前导斜杠）
	key := c.Params("*")
	// 移除前导斜杠（如果存在）
	if len(key) > 0 && key[0] == '/' {
		key = key[1:]
	}

	// 获取查询参数
	expiresStr := c.Query("expires")
	expiresIn := int64(3600) // 默认1小时
	if expiresStr != "" {
		expires, err := strconv.ParseInt(expiresStr, 10, 64)
		if err != nil {
			util.Warn("Invalid expires parameter", zap.String("expires", expiresStr), zap.Error(err))
			return c.Status(fiber.StatusBadRequest).JSON(model.APIResponse{
				Success: false,
				Error:   "Invalid expires parameter",
			})
		}
		expiresIn = expires
	}

	// 生成下载URL
	url, err := h.fileService.GetDownloadURL(c.Context(), key, expiresIn)
	if err != nil {
		util.Error("Failed to generate download URL", zap.String("key", key), zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(model.APIResponse{
			Success: false,
			Error:   "Failed to generate download URL",
		})
	}

	// 创建响应数据
	downloadResp := &model.DownloadURLResponse{
		URL:     url,
		Expires: expiresIn,
	}

	// 使用自定义JSON编码器确保URL不被转义
	responseData, err := model.EncodeAPIResponse(&model.APIResponse{
		Success: true,
		Data:    downloadResp,
	})
	if err != nil {
		util.Error("Failed to encode response", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(model.APIResponse{
			Success: false,
			Error:   "Failed to encode response",
		})
	}

	// 返回自定义编码的响应
	c.Set("Content-Type", "application/json")
	return c.Status(fiber.StatusOK).Send(responseData)
}
