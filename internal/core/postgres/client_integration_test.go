//go:build integration

// PostgreSQL 集成测试
// 需要 GitHub Actions service container 或在本地 docker compose 环境下运行

package postgres

import (
	"context"
	"os"
	"testing"

	"yomirrorsite/internal/config"
	"yomirrorsite/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================
// 测试辅助
// ============================================================

func testPGConfig() *config.PostgresConfig {
	port := 5432
	return &config.PostgresConfig{
		Host:     envOr("PG_HOST", "localhost"),
		Port:     port,
		User:     envOr("PG_USER", "yomirror"),
		Password: envOr("PG_PASSWORD", "test"),
		DBName:   envOr("PG_DB", "yomirror_test"),
		SSLMode:  "disable",
		MaxConns: 5,
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// ============================================================
// Client 连接测试
// ============================================================

func TestIntegration_NewClient(t *testing.T) {
	cfg := testPGConfig()
	client, err := NewClient(context.Background(), cfg)
	require.NoError(t, err)
	require.True(t, client.IsEnabled())
	defer func() {
		sqlDB, _ := client.DB.DB()
		sqlDB.Close()
	}()

	assert.NotNil(t, client.DB)
}

func TestIntegration_NewClient_Disabled(t *testing.T) {
	cfg := &config.PostgresConfig{Host: ""}
	client, err := NewClient(context.Background(), cfg)
	assert.NoError(t, err)
	assert.False(t, client.IsEnabled())
	assert.Nil(t, client.DB)
}

func TestIntegration_AutoMigrate(t *testing.T) {
	cfg := testPGConfig()
	client, err := NewClient(context.Background(), cfg)
	require.NoError(t, err)
	require.True(t, client.IsEnabled())
	defer func() {
		sqlDB, _ := client.DB.DB()
		sqlDB.Close()
	}()

	// 验证表存在
	hasTable := client.DB.Migrator().HasTable(&model.SoftwareTable{})
	assert.True(t, hasTable, "software table should exist")

	hasVersion := client.DB.Migrator().HasTable(&model.VersionTable{})
	assert.True(t, hasVersion, "version table should exist")

	hasAsset := client.DB.Migrator().HasTable(&model.AssetTable{})
	assert.True(t, hasAsset, "asset table should exist")
}

// ============================================================
// Software CRUD 测试
// ============================================================

func TestIntegration_UpsertSoftware(t *testing.T) {
	cfg := testPGConfig()
	client, err := NewClient(context.Background(), cfg)
	require.NoError(t, err)
	require.True(t, client.IsEnabled())
	defer cleanupDB(client)

	sw := &model.SoftwareTable{
		ID:         "test-software",
		Name:       "Test Software",
		GitHubRepo: "test-owner/test-repo",
		Category:   "test",
		LatestVer:  "1.0.0",
	}
	err = client.UpsertSoftware(context.Background(), sw)
	assert.NoError(t, err)

	// 再次 upsert 应幂等
	err = client.UpsertSoftware(context.Background(), sw)
	assert.NoError(t, err)
}

func TestIntegration_ListSoftware(t *testing.T) {
	cfg := testPGConfig()
	client, err := NewClient(context.Background(), cfg)
	require.NoError(t, err)
	require.True(t, client.IsEnabled())
	defer cleanupDB(client)

	// 插入测试数据
	sw := &model.SoftwareTable{ID: "list-test", Name: "List Test", GitHubRepo: "a/b", Category: "editor"}
	require.NoError(t, client.UpsertSoftware(context.Background(), sw))

	list, total, err := client.ListSoftware(context.Background(), 1, 10, "", "", "stars")
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(1))
	assert.GreaterOrEqual(t, len(list), 1)
}

// ============================================================
// Version 测试
// ============================================================

func TestIntegration_UpsertVersion(t *testing.T) {
	cfg := testPGConfig()
	client, err := NewClient(context.Background(), cfg)
	require.NoError(t, err)
	require.True(t, client.IsEnabled())
	defer cleanupDB(client)

	ver := &model.VersionTable{
		SoftwareID: "ver-test",
		TagName:    "v1.0.0",
		Name:       "Release v1.0.0",
	}
	id, err := client.UpsertVersion(context.Background(), ver)
	assert.NoError(t, err)
	assert.Greater(t, id, 0)

	// 幂等：再次 upsert 同一版本
	id2, err := client.UpsertVersion(context.Background(), ver)
	assert.NoError(t, err)
	assert.Equal(t, id, id2)
}

// ============================================================
// Tags 测试
// ============================================================

func TestIntegration_SaveAndGetTags(t *testing.T) {
	cfg := testPGConfig()
	client, err := NewClient(context.Background(), cfg)
	require.NoError(t, err)
	require.True(t, client.IsEnabled())
	defer cleanupDB(client)

	tags := []string{"开发工具", "编辑器"}
	err = client.SaveTags(context.Background(), "tag-test", tags)
	assert.NoError(t, err)

	got, err := client.GetTags(context.Background(), "tag-test")
	assert.NoError(t, err)
	assert.ElementsMatch(t, tags, got)
}

// ============================================================
// 清理
// ============================================================

func cleanupDB(client *Client) {
	client.DB.Exec("DELETE FROM software_tags")
	client.DB.Exec("DELETE FROM assets")
	client.DB.Exec("DELETE FROM versions")
	client.DB.Exec("DELETE FROM softwares")
	client.DB.Exec("DELETE FROM sync_logs")
	// 不删除表，保留 schema
}
