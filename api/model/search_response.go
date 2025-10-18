package model

import (
	"time"
)

// SearchResult 搜索结果
type SearchResult struct {
	Key          string    `json:"key"`           // 文件完整路径
	Name         string    `json:"name"`          // 文件名
	Path         string    `json:"path"`          // 文件所在目录路径
	Size         int64     `json:"size"`          // 文件大小
	LastModified time.Time `json:"last_modified"` // 最后修改时间
	Type         string    `json:"type"`          // 文件类型 (file/directory)
}

// SearchResponse 搜索响应
type SearchResponse struct {
	Results    []SearchResult `json:"results"`     // 搜索结果列表
	TotalCount int            `json:"total_count"` // 总匹配数量
	Keyword    string         `json:"keyword"`     // 搜索关键词
	Limit      int            `json:"limit"`       // 结果数量限制
}

// ConvertToSearchResponse 将文件列表转换为搜索响应
func ConvertToSearchResponse(files []FileInfo, keyword string, limit int) *SearchResponse {
	results := make([]SearchResult, 0, len(files))

	for _, file := range files {
		// 提取文件名和路径
		fileName := extractFileName(file.Key)
		filePath := extractDirectoryPath(file.Key)

		result := SearchResult{
			Key:          file.Key,
			Name:         fileName,
			Path:         filePath,
			Size:         file.Size,
			LastModified: file.LastModified,
			Type:         "file",
		}
		results = append(results, result)
	}

	return &SearchResponse{
		Results:    results,
		TotalCount: len(results),
		Keyword:    keyword,
		Limit:      limit,
	}
}

// extractFileName 从文件路径中提取文件名
func extractFileName(filePath string) string {
	lastSlashIndex := len(filePath) - 1
	for i := len(filePath) - 1; i >= 0; i-- {
		if filePath[i] == '/' {
			lastSlashIndex = i
			break
		}
	}

	if lastSlashIndex == len(filePath)-1 {
		// 如果是目录路径，返回目录名
		parts := filePath[:len(filePath)-1]
		lastSlash := len(parts) - 1
		for i := len(parts) - 1; i >= 0; i-- {
			if parts[i] == '/' {
				lastSlash = i
				break
			}
		}
		return parts[lastSlash+1:]
	}

	return filePath[lastSlashIndex+1:]
}

// extractDirectoryPath 从文件路径中提取目录路径
func extractDirectoryPath(filePath string) string {
	lastSlashIndex := len(filePath) - 1
	for i := len(filePath) - 1; i >= 0; i-- {
		if filePath[i] == '/' {
			lastSlashIndex = i
			break
		}
	}

	if lastSlashIndex == len(filePath)-1 {
		// 如果是目录路径，返回上级目录
		trimmed := filePath[:len(filePath)-1]
		lastSlash := len(trimmed) - 1
		for i := len(trimmed) - 1; i >= 0; i-- {
			if trimmed[i] == '/' {
				lastSlash = i
				break
			}
		}
		if lastSlash == len(trimmed)-1 {
			return ""
		}
		return trimmed[:lastSlash+1]
	}

	return filePath[:lastSlashIndex+1]
}
