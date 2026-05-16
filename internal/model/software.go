// 软件镜像站核心数据模型
// 定义 Software / Version / Asset / MirrorStats 等结构体
// 用于前后端数据交换，与前端 TypeScript 类型一一对应

package model

import (
	"fmt"
	"time"
)

// ============================================================
// 软件条目（列表页使用，不包含版本详情）
// ============================================================

// Software 软件条目概要信息
// 用于软件列表页、搜索结果、首页卡片展示
type Software struct {
	ID          string   `json:"id"`           // 唯一标识符，如 "vscode"、"obsidian"
	Name        string   `json:"name"`         // 显示名称，如 "Visual Studio Code"
	Description string   `json:"description"`  // 简介文字
	Homepage    string   `json:"homepage"`     // 项目主页 URL
	GitHubRepo  string   `json:"github_repo"`  // GitHub 仓库 "owner/repo" 格式
	IconURL     string   `json:"icon_url"`     // 软件图标 URL（S3 预签名地址）
	Category    string   `json:"category"`     // 分类：editor / dev-tool / media / system 等
	Tags        []string `json:"tags"`         // 标签列表，如 ["ide", "microsoft"]
	License     string   `json:"license"`      // 开源许可证，如 "MIT"
	Stars       int      `json:"stars"`        // GitHub 星数
	LatestVer   string   `json:"latest_ver"`   // 最新版本号，如 "v1.92.0"
	TotalAssets int      `json:"total_assets"` // 所有版本的资产文件总数
	UpdatedAt   string   `json:"updated_at"`   // 最后同步时间 RFC3339 格式
}

// ============================================================
// 软件详情（详情页使用，包含版本列表）
// ============================================================

// SoftwareDetail 软件完整详情
// 在 Software 基础上追加版本列表和统计信息
type SoftwareDetail struct {
	Software                         // 嵌入软件基本字段
	Versions      []VersionBrief `json:"versions"`       // 版本概要列表（不含资产详情）
	TotalVersions int            `json:"total_versions"` // 已镜像的版本总数
	TotalSize     int64          `json:"total_size"`     // 所有文件总大小（字节）
	ReadmeMD      string         `json:"readme_md"`      // 项目的 README.md 内容（Markdown）
}

// VersionBrief 版本概要（列表展示用，不含资产列表）
type VersionBrief struct {
	TagName     string `json:"tag_name"`     // Git tag，如 "v1.85.0"
	Name        string `json:"name"`         // 版本名称，如 "January 2024 Update"
	Prerelease  bool   `json:"prerelease"`   // 是否为预发布版本
	PublishedAt string `json:"published_at"` // 发布时间 RFC3339
	AssetCount  int    `json:"asset_count"`  // 该版本的资产文件数
}

// ============================================================
// 版本详情（单版本页面）
// ============================================================

// VersionDetail 版本完整详情
// 包含 Release Notes 和可下载资产列表
type VersionDetail struct {
	TagName     string      `json:"tag_name"`     // 版本 tag
	Name        string      `json:"name"`         // 版本名称
	Body        string      `json:"body"`         // Release Notes 正文（Markdown）
	Prerelease  bool        `json:"prerelease"`   // 是否预发布
	PublishedAt string      `json:"published_at"` // 发布时间
	Assets      []AssetInfo `json:"assets"`       // 可下载资产列表
}

// ============================================================
// 资产文件信息
// ============================================================

// AssetInfo 单个可下载文件的信息
// 对应 GitHub Release 的一个 Asset
type AssetInfo struct {
	Name        string `json:"name"`         // 文件名，如 "VSCode-win32-x64-1.85.0.zip"
	Size        int64  `json:"size"`         // 文件大小（字节）
	SizeHuman   string `json:"size_human"`   // 人类可读大小，如 "120.5 MB"
	Platform    string `json:"platform"`     // 目标平台：windows-x64 / linux-arm64 / macos-universal
	ContentType string `json:"content_type"` // MIME 类型，如 "application/zip"
	DownloadURL string `json:"download_url"` // S3 预签名下载 URL（带有效期）
	Checksum    string `json:"checksum"`     // SHA256 校验和
	Downloads   int64  `json:"downloads"`    // 累计下载次数（从 Redis 计数器获取）
}

// ============================================================
// 镜像站全局统计
// ============================================================

// MirrorStats 镜像站整体统计信息
// 用于首页统计展示
type MirrorStats struct {
	TotalSoftware  int       `json:"total_software"`   // 已镜像软件数量
	TotalVersions  int       `json:"total_versions"`   // 已镜像版本总数
	TotalAssets    int       `json:"total_assets"`     // 资产文件总数
	TotalSize      int64     `json:"total_size"`       // 总存储占用（字节）
	TotalDownloads int64     `json:"total_downloads"`  // 累计下载次数
	LastSyncAt     time.Time `json:"last_sync_at"`     // 最近一次同步完成时间
	SyncInProgress bool      `json:"sync_in_progress"` // 当前是否有同步任务运行中
}

// ============================================================
// 分页列表响应
// ============================================================

// SoftwareListPage 软件列表的分页响应
type SoftwareListPage struct {
	Items      []Software `json:"items"`       // 当前页软件列表
	Page       int        `json:"page"`        // 当前页码
	PageSize   int        `json:"page_size"`   // 每页数量
	TotalCount int        `json:"total_count"` // 符合条件的总数量
}

// ============================================================
// 同步状态
// ============================================================

// SyncStatus 同步任务状态
// 用于 /api/sync/status 接口
type SyncStatus struct {
	InProgress  bool              `json:"in_progress"` // 是否有同步进行中
	CurrentJob  string            `json:"current_job"` // 当前同步的软件 ID
	QueueLength int               `json:"queue_length"`// 队列中待同步的软件数
	LastSyncAt  time.Time         `json:"last_sync_at"`// 上一次全量同步完成时间
	LastResult  *SyncResultBrief  `json:"last_result"` // 上一次同步结果概要
}

// SyncResultBrief 单次同步结果概要
type SyncResultBrief struct {
	SoftwareID  string   `json:"software_id"`
	NewVersions int      `json:"new_versions"`
	NewAssets   int      `json:"new_assets"`
	Errors      []string `json:"errors,omitempty"`
	Duration    string   `json:"duration"` // 耗时字符串
}

// ============================================================
// 通用 API 响应
// ============================================================

// APIResponse 统一 API 响应包装
// 所有 HTTP 接口返回此结构
type APIResponse struct {
	Success bool        `json:"success"`          // 请求是否成功
	Data    interface{} `json:"data,omitempty"`   // 响应数据
	Error   string      `json:"error,omitempty"`  // 错误信息
	Total   int         `json:"total,omitempty"`  // 分页总数量
}

// ============================================================
// 辅助函数
// ============================================================

// FormatSize 将字节数转换为人类可读的大小字符串
// 例如：1048576 → "1.0 MB"，1540234 → "1.5 MB"
func FormatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
