// GORM 数据模型 —— 与 PostgreSQL 表一一映射
// 用于软件镜像站的持久化存储层
// tag 格式：gorm:"column:xxx;type:xxx;..."
// 通过 AutoMigrate 自动建表（幂等）

package model

import (
	"time"
)

// ============================================================
// 1. 软件基本信息表 — software
// ============================================================

// SoftwareTable 软件基本信息（GORM 映射）
// 每行一个被镜像的 GitHub 仓库
type SoftwareTable struct {
	ID          string    `gorm:"column:id;type:varchar(64);primaryKey" json:"id"`
	Name        string    `gorm:"column:name;type:varchar(256);not null" json:"name"`
	Description string    `gorm:"column:description;type:text" json:"description"`
	GitHubRepo  string    `gorm:"column:github_repo;type:varchar(256);not null" json:"github_repo"`
	Homepage    string    `gorm:"column:homepage;type:varchar(512)" json:"homepage"`
	IconURL     string    `gorm:"column:icon_url;type:varchar(512)" json:"icon_url"`
	Category    string    `gorm:"column:category;type:varchar(64);default:uncategorized" json:"category"`
	License     string    `gorm:"column:license;type:varchar(128)" json:"license"`
	Stars       int       `gorm:"column:stars;type:int;default:0" json:"stars"`
	ReadmeMD    string    `gorm:"column:readme_md;type:text" json:"readme_md"`
	LatestVer   string    `gorm:"column:latest_ver;type:varchar(64)" json:"latest_ver"`
	TotalSize   int64     `gorm:"column:total_size;type:bigint;default:0" json:"total_size"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (SoftwareTable) TableName() string { return "software" }

// ToAPI 转换为接口返回的 Software 模型
func (s *SoftwareTable) ToAPI() Software {
	return Software{
		ID: s.ID, Name: s.Name, Description: s.Description,
		GitHubRepo: s.GitHubRepo, Homepage: s.Homepage,
		IconURL: s.IconURL, Category: s.Category,
		License: s.License, Stars: s.Stars,
		LatestVer: s.LatestVer, UpdatedAt: s.UpdatedAt.Format(time.RFC3339),
	}
}

// ============================================================
// 2. 软件标签表 — software_tag
// ============================================================

// SoftwareTagTable 软件多值标签
type SoftwareTagTable struct {
	SoftwareID string `gorm:"column:software_id;type:varchar(64);primaryKey" json:"software_id"`
	Tag        string `gorm:"column:tag;type:varchar(64);primaryKey" json:"tag"`
}

func (SoftwareTagTable) TableName() string { return "software_tag" }

// ============================================================
// 3. 版本表 — version
// ============================================================

// VersionTable 单个软件的版本发布记录
type VersionTable struct {
	ID          int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	SoftwareID  string    `gorm:"column:software_id;type:varchar(64);not null;uniqueIndex:uk_sw_tag" json:"software_id"`
	TagName     string    `gorm:"column:tag_name;type:varchar(128);not null;uniqueIndex:uk_sw_tag" json:"tag_name"`
	Name        string    `gorm:"column:name;type:varchar(256)" json:"name"`
	Prerelease  bool      `gorm:"column:prerelease;type:bool;default:false" json:"prerelease"`
	PublishedAt time.Time `gorm:"column:published_at;not null" json:"published_at"`
	Body        string    `gorm:"column:body;type:text" json:"body"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
}

func (VersionTable) TableName() string { return "version" }

// ============================================================
// 4. 资产文件表 — asset
// ============================================================

// AssetTable 版本中包含的可下载文件
type AssetTable struct {
	ID            int        `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	VersionID     int        `gorm:"column:version_id;type:int;not null;index" json:"version_id"`
	Name          string     `gorm:"column:name;type:varchar(512);not null" json:"name"`
	Size          int64      `gorm:"column:size;type:bigint;not null;default:0" json:"size"`
	ContentType   string     `gorm:"column:content_type;type:varchar(128)" json:"content_type"`
	Platform      string     `gorm:"column:platform;type:varchar(64);index" json:"platform"`
	S3Key         string     `gorm:"column:s3_key;type:varchar(1024);not null;uniqueIndex" json:"s3_key"`
	S3URL         string     `gorm:"column:s3_url;type:varchar(2048)" json:"s3_url"`
	S3URLExp      *time.Time `gorm:"column:s3_url_exp" json:"s3_url_exp"`
	Checksum      string     `gorm:"column:checksum;type:varchar(128)" json:"checksum"`
	DownloadCount int64      `gorm:"column:download_count;type:bigint;default:0" json:"download_count"`
	CreatedAt     time.Time  `gorm:"column:created_at;autoCreateTime" json:"created_at"`
}

func (AssetTable) TableName() string { return "asset" }

// ============================================================
// 5. 同步日志表 — sync_log
// ============================================================

// SyncLogTable 每次同步操作的完整记录
type SyncLogTable struct {
	ID          int        `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	SoftwareID  string     `gorm:"column:software_id;type:varchar(64);not null;index" json:"software_id"`
	Status      string     `gorm:"column:status;type:varchar(16);not null;default:running" json:"status"`
	NewVersions int        `gorm:"column:new_versions;type:int;default:0" json:"new_versions"`
	NewAssets   int        `gorm:"column:new_assets;type:int;default:0" json:"new_assets"`
	Skipped     int        `gorm:"column:skipped;type:int;default:0" json:"skipped"`
	TotalSize   int64      `gorm:"column:total_size;type:bigint;default:0" json:"total_size"`
	ErrorMsg    string     `gorm:"column:error_msg;type:text" json:"error_msg"`
	StartedAt   time.Time  `gorm:"column:started_at;autoCreateTime" json:"started_at"`
	FinishedAt  *time.Time `gorm:"column:finished_at" json:"finished_at"`
}

func (SyncLogTable) TableName() string { return "sync_log" }
