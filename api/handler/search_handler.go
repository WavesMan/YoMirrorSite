package handler

import (
	"strconv"

	"yomirrorsite/api/model"
	"yomirrorsite/internal/service"
	"yomirrorsite/internal/util"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

// SearchHandler 搜索处理器
type SearchHandler struct {
	searchService *service.SearchService
}

// NewSearchHandler 创建搜索处理器
func NewSearchHandler(searchService *service.SearchService) *SearchHandler {
	return &SearchHandler{
		searchService: searchService,
	}
}

// SearchFiles 搜索文件
func (h *SearchHandler) SearchFiles(c *fiber.Ctx) error {
	util.Debug("搜索请求", util.Module("handler"))
	// 获取查询参数
	keyword := c.Query("keyword")
	limitStr := c.Query("limit")

	// 解析限制参数
	limit := 50 // 默认限制50个结果
	if limitStr != "" {
		parsedLimit, err := strconv.Atoi(limitStr)
		if err != nil {
			util.Warn("Invalid limit parameter", zap.String("limit", limitStr), zap.Error(err))
			return c.Status(fiber.StatusBadRequest).JSON(model.APIResponse{
				Success: false,
				Error:   "Invalid limit parameter",
			})
		}
		limit = parsedLimit
	}

	// 验证关键词
	if keyword == "" {
		return c.Status(fiber.StatusBadRequest).JSON(model.APIResponse{
			Success: false,
			Error:   "Keyword parameter is required",
		})
	}

	// 限制最大结果数量
	if limit > 100 {
		limit = 100
	}
	if limit < 1 {
		limit = 1
	}

	// 执行搜索
	searchResults, err := h.searchService.SearchFiles(c.Context(), keyword, limit)
	if err != nil {
		util.Error("Failed to search files", zap.String("keyword", keyword), zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(model.APIResponse{
			Success: false,
			Error:   "Failed to search files",
		})
	}

	// 转换为模型 FileInfo 格式
	modelFileInfos := make([]model.FileInfo, len(searchResults))
	for i, result := range searchResults {
		modelFileInfos[i] = model.FileInfo{
			Name:         result.Name,
			Key:          result.Key,
			Size:         result.Size,
			LastModified: result.LastModified,
		}
	}

	// 转换为响应格式
	response := model.ConvertToSearchResponse(modelFileInfos, keyword, limit)

	// 返回响应
	return c.Status(fiber.StatusOK).JSON(model.APIResponse{
		Success: true,
		Data:    response,
	})
}
