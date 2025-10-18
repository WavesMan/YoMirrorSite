package model

import (
	"time"

	"s3-file-service/internal/service"
)

// APIResponse API响应
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// FileListResponse 文件列表响应
type FileListResponse struct {
	Files []FileInfo `json:"files"`
	Count int        `json:"count"`
}

// FileInfo 文件信息
type FileInfo struct {
	Name         string    `json:"name"`
	Key          string    `json:"key"`
	Size         int64     `json:"size"`
	LastModified time.Time `json:"last_modified"`
}

// DownloadURLResponse 下载URL响应
type DownloadURLResponse struct {
	URL     string `json:"url"`
	Expires int64  `json:"expires_in_seconds"`
}

// SyncStatusResponse 同步状态响应
type SyncStatusResponse struct {
	Syncing bool   `json:"syncing"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// RepositoryListResponse 仓库列表响应
type RepositoryListResponse struct {
	Repositories []RepositoryInfo `json:"repositories"`
	Count        int              `json:"count"`
}

// RepositoryInfo 仓库信息
type RepositoryInfo struct {
	URL           string `json:"url"`
	Branch        string `json:"branch"`
	LocalPath     string `json:"local_path"`
	S3TargetPath  string `json:"s3_target_path"`
	LatestVersion string `json:"latest_version,omitempty"`
	LastSynced    string `json:"last_synced,omitempty"`
}

// ConvertToFileListResponse 转换为文件列表响应
func ConvertToFileListResponse(fileList []service.FileInfo) FileListResponse {
	fileInfos := make([]FileInfo, len(fileList))
	for i, file := range fileList {
		fileInfos[i] = FileInfo{
			Name:         file.Name,
			Key:          file.Key,
			Size:         file.Size,
			LastModified: file.LastModified,
		}
	}

	return FileListResponse{
		Files: fileInfos,
		Count: len(fileList),
	}
}
