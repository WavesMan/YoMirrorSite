// PostgreSQL 连接管理层
// 基于 GORM + github.com/lib/pq 驱动
// 若 PostgresConfig.Host 为空，则 enabled=false，所有操作降级
// AutoMigrate 自动建表（幂等：CREATE TABLE IF NOT EXISTS）

package postgres

import (
	"context"
	"fmt"

	_ "github.com/lib/pq"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"yomirrorsite/internal/config"
	"yomirrorsite/internal/model"
	"yomirrorsite/internal/util"

	"go.uber.org/zap"
)

// ============================================================
// 客户端结构
// ============================================================

// Client PostgreSQL 持久化客户端
// 封装 GORM DB 实例，提供 repository 层的初始化入口
type Client struct {
	DB      *gorm.DB // GORM 数据库实例（nil 表示未启用）
	enabled bool
}

// NewClient 创建 PG 客户端并自动建表
// 若 cfg 未启用（Host 为空），返回 enabled=false 的 Client，所有操作降级
func NewClient(ctx context.Context, cfg *config.PostgresConfig) (*Client, error) {
	if !cfg.IsEnabled() {
		util.Info("PostgreSQL 未配置，跳过 PG 中间层，降级到 S3+Redis")
		return &Client{enabled: false}, nil
	}

	cfg.ApplyDefaults()

	db, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn), // 仅警告级别 SQL 日志
	})
	if err != nil {
		return nil, fmt.Errorf("连接 PostgreSQL 失败: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("获取底层 sql.DB 失败: %w", err)
	}
	sqlDB.SetMaxOpenConns(cfg.MaxConns)
	sqlDB.SetMaxIdleConns(cfg.MaxConns / 2)

	// 自动建表（幂等：CREATE TABLE IF NOT EXISTS + 结构同步）
	if err := db.AutoMigrate(
		&model.SoftwareTable{},
		&model.SoftwareTagTable{},
		&model.VersionTable{},
		&model.AssetTable{},
		&model.SyncLogTable{},
	); err != nil {
		return nil, fmt.Errorf("自动建表失败: %w", err)
	}

	util.Info("PostgreSQL 连接成功，表结构已同步",
		zap.String("host", cfg.Host),
		zap.Int("port", cfg.Port),
		zap.String("dbname", cfg.DBName))

	return &Client{DB: db, enabled: true}, nil
}

// IsEnabled PG 中间层是否启用
func (c *Client) IsEnabled() bool { return c.enabled && c.DB != nil }

// Close 关闭数据库连接
func (c *Client) Close() error {
	if c.DB != nil {
		sqlDB, err := c.DB.DB()
		if err != nil {
			return err
		}
		return sqlDB.Close()
	}
	return nil
}
