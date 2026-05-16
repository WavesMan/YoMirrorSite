package service

import (
	"context"
	"fmt"
	"strings"

	"yomirrorsite/internal/core/s3"
	"yomirrorsite/internal/util"

	"go.uber.org/zap"
)

// SearchService 搜索服务
type SearchService struct {
	s3Client *s3.Client
}

// NewSearchService 创建搜索服务
func NewSearchService(s3Client *s3.Client) *SearchService {
	return &SearchService{
		s3Client: s3Client,
	}
}

// SearchFiles 搜索文件（基于文件列表缓存）
func (s *SearchService) SearchFiles(ctx context.Context, keyword string, limit int) ([]FileInfo, error) {
	util.Info("Searching files", zap.String("keyword", keyword), zap.Int("limit", limit))

	// 验证参数
	if strings.TrimSpace(keyword) == "" {
		return nil, nil
	}

	// 从文件列表缓存中获取数据
	fileList, err := s.getFileListFromCache(ctx, "")
	if err != nil {
		util.Error("Failed to get file list from cache", zap.String("keyword", keyword), zap.Error(err))
		return nil, fmt.Errorf("缓存数据不可用，请稍后重试")
	}

	// 在内存中执行搜索
	results := s.searchInMemory(fileList, keyword, limit)

	util.Info("Search completed from cache",
		zap.String("keyword", keyword),
		zap.Int("results", len(results)),
		zap.Int("limit", limit),
		zap.Int("total_files", len(fileList)))

	return results, nil
}

// getFileListFromCache 从文件列表缓存获取数据
func (s *SearchService) getFileListFromCache(ctx context.Context, prefix string) ([]FileInfo, error) {
	cacheKey := "files_list_cache:" + prefix

	// 读取元数据获取分片数量
	var shardCount int
	found, err := util.GetJSON(ctx, cacheKey, &shardCount)
	if err != nil || !found {
		return nil, fmt.Errorf("cache not found for prefix: %s", prefix)
	}

	// 读取所有分片数据
	var fullList []FileInfo
	for i := 0; i < shardCount; i++ {
		shardKey := fmt.Sprintf("%s:shard:%d", cacheKey, i)
		var shardData []FileInfo
		found, err := util.GetJSON(ctx, shardKey, &shardData)
		if err != nil || !found {
			return nil, fmt.Errorf("shard cache missing: %s", shardKey)
		}
		fullList = append(fullList, shardData...)
	}

	return fullList, nil
}

// searchInMemory 在内存中执行搜索
func (s *SearchService) searchInMemory(fileList []FileInfo, keyword string, limit int) []FileInfo {
	var results []FileInfo
	keyword = strings.ToLower(keyword)

	// 遍历文件列表进行关键词匹配
	for _, file := range fileList {
		// 在文件名和完整路径中搜索
		fileName := strings.ToLower(file.Name)
		fileKey := strings.ToLower(file.Key)

		if strings.Contains(fileName, keyword) || strings.Contains(fileKey, keyword) {
			results = append(results, file)
			if len(results) >= limit {
				break
			}
		}
	}

	return results
}

// extractFileName 从文件路径中提取文件名
func extractFileName(key string) string {
	// 移除路径前缀，只保留文件名
	lastSlashIndex := strings.LastIndex(key, "/")
	if lastSlashIndex == -1 {
		return key
	}

	// 如果以斜杠结尾，说明是目录，需要进一步处理
	if lastSlashIndex == len(key)-1 {
		trimmed := key[:len(key)-1]
		lastSlash := strings.LastIndex(trimmed, "/")
		if lastSlash == -1 {
			return trimmed
		}
		return trimmed[lastSlash+1:]
	}

	return key[lastSlashIndex+1:]
}
