package config

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
)

// Config 应用程序配置
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	S3       S3Config       `yaml:"s3"`
	Redis    RedisConfig    `yaml:"redis"`
	Cache    CacheConfig    `yaml:"cache"`
	Mirror   MirrorConfig   `yaml:"mirror"`   // 软件镜像站配置
	Postgres PostgresConfig `yaml:"postgres"` // PG 持久化配置（可选，不配则降级）
}

// ServerConfig 服务配置
type ServerConfig struct {
	Port                    int `yaml:"port"`
	GoroutinePoolSize       int `yaml:"goroutine_pool_size"`
	ParallelDownloadThreads int `yaml:"parallel_download_threads"`
}

// S3Config S3对象存储配置
type S3Config struct {
	AccessKey  string     `yaml:"access_key"`
	SecretKey  string     `yaml:"secret_key"`
	Endpoint   string     `yaml:"endpoint"`
	BucketName string     `yaml:"bucket_name"`
	ListenDir  string     `yaml:"listen_dir"`
	CORS       CORSConfig `yaml:"cors"`
}

// CORSConfig CORS配置
type CORSConfig struct {
	AllowedOrigins []string `yaml:"allowed_origins"`
	AllowedMethods []string `yaml:"allowed_methods"`
	AllowedHeaders []string `yaml:"allowed_headers"`
	MaxAgeSeconds  int      `yaml:"max_age_seconds"`
}

// RedisConfig Redis配置
type RedisConfig struct {
	Addr     string `yaml:"addr"`
	UseTLS             bool   `yaml:"use_tls"`              // 启用 TLS 连接
	InsecureSkipVerify bool   `yaml:"tls_insecure_skip"`    // 跳过证书校检（仅开发环境）
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

// CacheConfig 缓存配置
type CacheConfig struct {
	FilesListRefreshIntervalSec int              `yaml:"filesListRefreshIntervalSec"`
	LocalCache                  LocalCacheConfig `yaml:"local_cache"`
}

// LocalCacheConfig 本地缓存配置
type LocalCacheConfig struct {
	Enabled           bool   `yaml:"enabled"`
	Size              int    `yaml:"size"`
	Mode              string `yaml:"mode"` // "lru" 或 "lfu"
	HotDataRefreshSec int    `yaml:"hot_data_refresh_sec"`
	HotDataThreshold  int    `yaml:"hot_data_threshold"`
}

// AWSConfig 将S3Config转换为aws.Config
func (c *S3Config) AWSConfig() aws.Config {
	return aws.Config{
		Region: "us-east-1", // 设置一个默认region，即使对于自定义端点也需要
		Credentials: aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
			return aws.Credentials{
				AccessKeyID:     c.AccessKey,
				SecretAccessKey: c.SecretKey,
				SessionToken:    "", // 明确设置空session token
			}, nil
		}),
		EndpointResolver: aws.EndpointResolverFunc(func(service, region string) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL:           c.Endpoint,
				SigningRegion: "us-east-1",              // 为自定义端点设置签名region
				Source:        aws.EndpointSourceCustom, // 明确标记为自定义端点
			}, nil
		}),
	}
}

// ============================================================
// 配置验证
// ============================================================

// Validate 验证配置是否有效
func (c *Config) Validate() error {
	// 验证S3配置
	if c.S3.BucketName == "" {
		return fmt.Errorf("s3.bucket_name is required")
	}
	if c.S3.AccessKey == "" {
		return fmt.Errorf("s3.access_key is required")
	}
	if c.S3.SecretKey == "" {
		return fmt.Errorf("s3.secret_key is required")
	}
	if c.S3.Endpoint == "" {
		return fmt.Errorf("s3.endpoint is required")
	}

	return nil
}

// ============================================================
// 多同步规则（YoMirrorSite 新增）
// ============================================================

// SyncRule 同步规则枚举
// 控制每个软件仓库的同步策略
type SyncRule string

const (
	// SyncRuleIncremental 增量同步：从上次同步的 tag 之后只拉取新版本（默认行为）
	SyncRuleIncremental SyncRule = "incremental"
	// SyncRuleLatestOnly 只保留最新：增量同步后清理旧版本，仅保留最近 N 个版本
	SyncRuleLatestOnly SyncRule = "latest_only"
	// SyncRuleFullHistorical 全量历史：无视 last_synced_tag，拉取所有 Release（新→旧）
	SyncRuleFullHistorical SyncRule = "full_historical"
)

// isValidSyncRule 校验 sync_rule 值是否合法
func (r SyncRule) isValidSyncRule() bool {
	switch r {
	case SyncRuleIncremental, SyncRuleLatestOnly, SyncRuleFullHistorical, "":
		return true
	default:
		return false
	}
}

// ============================================================
// 镜像站配置（YoMirrorSite 新增）
// ============================================================

// MirrorConfig 软件镜像站配置
// 定义需要镜像的软件列表和同步参数
type MirrorConfig struct {
	SoftwareList []SoftwareConfig `yaml:"software_list"` // 需镜像的软件列表
	Sync         SyncConfig       `yaml:"sync"`          // 同步参数
}

// SoftwareConfig 单个软件的镜像配置
// 每一条配置对应一个 GitHub 仓库的镜像策略
type SoftwareConfig struct {
	ID             string        `yaml:"id"`              // 软件唯一标识，如 "vscode"
	Name           string        `yaml:"name"`            // 显示名称，如 "Visual Studio Code"
	GitHubRepo     string        `yaml:"github_repo"`     // GitHub 仓库 "owner/repo"
	Category       string        `yaml:"category"`        // 分类标签
	Tags           []string      `yaml:"tags"`            // 标签列表
	FilterAssets   []AssetFilter `yaml:"filter_assets"`   // 资产过滤规则（只同步匹配的文件）
	SyncPrerelease bool          `yaml:"sync_prerelease"` // 是否同步预发布版本
	IconPattern    string        `yaml:"icon_pattern"`    // 图标文件在仓库中的路径模式（可选）
	// === 多同步规则（YoMirrorSite 新增） ===
	SyncRule     SyncRule `yaml:"sync_rule"`      // 同步规则，默认 incremental（向后兼容：空值 = incremental）
	KeepVersions int      `yaml:"keep_versions"`  // latest_only 模式下保留最近 N 个版本（默认 1）
	FullSyncOnce bool     `yaml:"full_sync_once"` // full_historical 模式下首次全量后自动切回 incremental
}

// AssetFilter 资产过滤规则
// 用于从 GitHub Release 的众多资产中筛选出需要镜像的文件
type AssetFilter struct {
	Platform string `yaml:"platform"` // 目标平台标识："windows-x64" / "linux-arm64" / "macos-universal"
	Pattern  string `yaml:"pattern"`  // 文件名匹配模式（支持 * 通配符），如 "VSCode-win32-x64-*.zip"
}

// SyncConfig 同步调度配置
type SyncConfig struct {
	IntervalMinutes int    `yaml:"interval_minutes"` // 同步间隔（分钟），默认 30
	GitHubToken     string `yaml:"github_token"`     // GitHub Personal Access Token（可选，提升速率限制）
	MaxConcurrent   int    `yaml:"max_concurrent"`   // 最大并发同步数，默认 3
	RetryAttempts   int    `yaml:"retry_attempts"`   // 单次同步失败重试次数，默认 3
}

// ============================================================
// 配置默认值填充
// ============================================================

// ApplyDefaults 为镜像站配置填充默认值（含多同步规则默认值）
func (c *MirrorConfig) ApplyDefaults() {
	if c.Sync.IntervalMinutes <= 0 {
		c.Sync.IntervalMinutes = 30
	}
	if c.Sync.MaxConcurrent <= 0 {
		c.Sync.MaxConcurrent = 3
	}
	if c.Sync.RetryAttempts <= 0 {
		c.Sync.RetryAttempts = 3
	}
	// 为每个软件配置提供默认值（含多同步规则）
	for i := range c.SoftwareList {
		sw := &c.SoftwareList[i]
		if sw.Category == "" {
			sw.Category = "uncategorized"
		}
		// 多同步规则默认值
		if sw.SyncRule == "" {
			sw.SyncRule = SyncRuleIncremental
		} else if !sw.SyncRule.isValidSyncRule() {
			// 非法值回退增量（防御）
			sw.SyncRule = SyncRuleIncremental
		}
		if sw.KeepVersions <= 0 {
			sw.KeepVersions = 1
		}
	}
}

// GetAssetFilter 根据平台标识获取资产过滤模式
// 如果没有配置该平台的过滤规则，返回空字符串
func (sw *SoftwareConfig) GetAssetFilter(platform string) string {
	for _, f := range sw.FilterAssets {
		if f.Platform == platform {
			return f.Pattern
		}
	}
	return ""
}

// ============================================================
// PostgreSQL 配置（YoMirrorSite 新增）
// ============================================================

// PostgresConfig PostgreSQL 连接配置
// Host 为空时，系统降级到纯 S3 + Redis 模式
type PostgresConfig struct {
	Host     string `yaml:"host"`      // 数据库地址
	Port     int    `yaml:"port"`      // 端口，默认 5432
	User     string `yaml:"user"`      // 用户名
	Password string `yaml:"password"`  // 密码
	DBName   string `yaml:"dbname"`    // 数据库名
	SSLMode  string `yaml:"sslmode"`   // sslmode，默认 disable
	MaxConns int    `yaml:"max_conns"` // 最大连接数，默认 10
}

// IsEnabled PG 是否已配置（Host 非空即为启用）
func (c *PostgresConfig) IsEnabled() bool {
	return c.Host != ""
}

// DSN 生成 database/sql 连接字符串
func (c *PostgresConfig) DSN() string {
	port := c.Port
	if port == 0 {
		port = 5432
	}
	sslmode := c.SSLMode
	if sslmode == "" {
		sslmode = "disable"
	}
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, port, c.User, c.Password, c.DBName, sslmode,
	)
}

// ApplyDefaults 填充默认值
func (c *PostgresConfig) ApplyDefaults() {
	if c.Port == 0 {
		c.Port = 5432
	}
	if c.SSLMode == "" {
		c.SSLMode = "disable"
	}
	if c.MaxConns == 0 {
		c.MaxConns = 10
	}
}

// MatchAssetName 检查资产文件名是否匹配任一过滤规则
// 支持 * 通配符匹配（使用 filepath.Match）
func (sw *SoftwareConfig) MatchAssetName(name string) bool {
	for _, f := range sw.FilterAssets {
		matched, err := filepath.Match(f.Pattern, name)
		if err == nil && matched {
			return true
		}
	}
	return len(sw.FilterAssets) == 0 // 如果没有配置过滤规则，则全部通过
}

// ============================================================
// 多同步规则校验（YoMirrorSite 新增）
// ============================================================

// ValidateSyncRules 校验所有软件配置的同步规则是否合法
// 返回首个不合法的错误，nil 表示全部通过
func (c *MirrorConfig) ValidateSyncRules() error {
	for i, sw := range c.SoftwareList {
		if !sw.SyncRule.isValidSyncRule() {
			return fmt.Errorf("software[%d] (%s): 未知的 sync_rule: %s", i, sw.ID, sw.SyncRule)
		}
		if sw.SyncRule == SyncRuleLatestOnly && sw.KeepVersions < 1 {
			return fmt.Errorf("software[%d] (%s): latest_only 模式下 keep_versions 至少为 1", i, sw.ID)
		}
	}
	return nil
}
