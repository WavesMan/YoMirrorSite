// 配置模块单元测试
// 测试 Validate、DSN、MatchAssetName、ApplyDefaults 等纯函数

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ============================================================
// PostgresConfig.DSN 测试
// ============================================================

func TestPostgresConfig_DSN_Defaults(t *testing.T) {
	cfg := PostgresConfig{
		Host:     "localhost",
		User:     "testuser",
		Password: "testpass",
		DBName:   "testdb",
	}
	dsn := cfg.DSN()
	assert.Contains(t, dsn, "host=localhost")
	assert.Contains(t, dsn, "port=5432") // default
	assert.Contains(t, dsn, "user=testuser")
	assert.Contains(t, dsn, "password=testpass")
	assert.Contains(t, dsn, "dbname=testdb")
	assert.Contains(t, dsn, "sslmode=disable") // default
}

func TestPostgresConfig_DSN_Custom(t *testing.T) {
	cfg := PostgresConfig{
		Host:     "db.example.com",
		Port:     5433,
		User:     "admin",
		Password: "secret",
		DBName:   "prod",
		SSLMode:  "require",
	}
	dsn := cfg.DSN()
	assert.Contains(t, dsn, "port=5433")
	assert.Contains(t, dsn, "sslmode=require")
}

// ============================================================
// PostgresConfig.IsEnabled 测试
// ============================================================

func TestPostgresConfig_IsEnabled(t *testing.T) {
	assert.True(t, (&PostgresConfig{Host: "localhost"}).IsEnabled())
	assert.False(t, (&PostgresConfig{Host: ""}).IsEnabled())
}

// ============================================================
// Config.Validate 测试
// ============================================================

func TestConfig_Validate_Success(t *testing.T) {
	cfg := &Config{
		S3: S3Config{
			BucketName: "test-bucket",
			AccessKey:  "AK",
			SecretKey:  "SK",
			Endpoint:   "http://localhost:9000",
		},
	}
	err := cfg.Validate()
	assert.NoError(t, err)
}

func TestConfig_Validate_MissingBucket(t *testing.T) {
	cfg := &Config{S3: S3Config{AccessKey: "AK", SecretKey: "SK", Endpoint: "http://x"}}
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bucket_name")
}

func TestConfig_Validate_MissingAccessKey(t *testing.T) {
	cfg := &Config{S3: S3Config{BucketName: "b", SecretKey: "SK", Endpoint: "http://x"}}
	assert.Error(t, cfg.Validate())
}

func TestConfig_Validate_MissingSecretKey(t *testing.T) {
	cfg := &Config{S3: S3Config{BucketName: "b", AccessKey: "AK", Endpoint: "http://x"}}
	assert.Error(t, cfg.Validate())
}

func TestConfig_Validate_MissingEndpoint(t *testing.T) {
	cfg := &Config{S3: S3Config{BucketName: "b", AccessKey: "AK", SecretKey: "SK"}}
	assert.Error(t, cfg.Validate())
}

// ============================================================
// MirrorConfig.ApplyDefaults 测试
// ============================================================

func TestMirrorConfig_ApplyDefaults(t *testing.T) {
	cfg := &MirrorConfig{
		Sync: SyncConfig{},
	}
	cfg.ApplyDefaults()
	assert.Equal(t, 30, cfg.Sync.IntervalMinutes)
	assert.Equal(t, 3, cfg.Sync.MaxConcurrent)
	assert.Equal(t, 3, cfg.Sync.RetryAttempts)
}

func TestMirrorConfig_ApplyDefaults_NoOverwrite(t *testing.T) {
	cfg := &MirrorConfig{
		Sync: SyncConfig{
			IntervalMinutes: 60,
			MaxConcurrent:   5,
			RetryAttempts:   10,
		},
	}
	cfg.ApplyDefaults()
	assert.Equal(t, 60, cfg.Sync.IntervalMinutes)
	assert.Equal(t, 5, cfg.Sync.MaxConcurrent)
	assert.Equal(t, 10, cfg.Sync.RetryAttempts)
}

func TestMirrorConfig_ApplyDefaults_Category(t *testing.T) {
	cfg := &MirrorConfig{
		SoftwareList: []SoftwareConfig{
			{ID: "sw1", Category: ""},
			{ID: "sw2", Category: "editor"},
		},
	}
	cfg.ApplyDefaults()
	assert.Equal(t, "uncategorized", cfg.SoftwareList[0].Category)
	assert.Equal(t, "editor", cfg.SoftwareList[1].Category)
}

// ============================================================
// SoftwareConfig.MatchAssetName 测试
// ============================================================

func TestSoftwareConfig_MatchAssetName_WithFilter(t *testing.T) {
	sw := SoftwareConfig{
		FilterAssets: []AssetFilter{
			{Pattern: "VSCode-win32-x64-*.zip"},
		},
	}
	assert.True(t, sw.MatchAssetName("VSCode-win32-x64-1.85.0.zip"))
	assert.False(t, sw.MatchAssetName("VSCode-linux-x64-1.85.0.tar.gz"))
}

func TestSoftwareConfig_MatchAssetName_NoFilter_AllPass(t *testing.T) {
	sw := SoftwareConfig{}
	assert.True(t, sw.MatchAssetName("anything.exe"))
	assert.True(t, sw.MatchAssetName(""))
}

func TestSoftwareConfig_MatchAssetName_MultipleFilters(t *testing.T) {
	sw := SoftwareConfig{
		FilterAssets: []AssetFilter{
			{Pattern: "*.zip"},
			{Pattern: "*.tar.gz"},
		},
	}
	assert.True(t, sw.MatchAssetName("file.zip"))
	assert.True(t, sw.MatchAssetName("file.tar.gz"))
	assert.False(t, sw.MatchAssetName("file.exe"))
}

// ============================================================
// PostgresConfig.ApplyDefaults 测试
// ============================================================

// ============================================================
// SyncRule 类型测试（YoMirrorSite 新增）
// ============================================================

func TestSyncRule_constants(t *testing.T) {
	assert.Equal(t, SyncRule("incremental"), SyncRuleIncremental)
	assert.Equal(t, SyncRule("latest_only"), SyncRuleLatestOnly)
	assert.Equal(t, SyncRule("full_historical"), SyncRuleFullHistorical)
}

func TestSyncRule_isValidSyncRule(t *testing.T) {
	assert.True(t, SyncRuleIncremental.isValidSyncRule())
	assert.True(t, SyncRuleLatestOnly.isValidSyncRule())
	assert.True(t, SyncRuleFullHistorical.isValidSyncRule())
	assert.True(t, SyncRule("").isValidSyncRule())
	assert.False(t, SyncRule("invalid").isValidSyncRule())
}

// ============================================================
// MirrorConfig.ApplyDefaults（多同步规则默认值）测试
// ============================================================

func TestMirrorConfig_ApplyDefaults_SyncRule_Default(t *testing.T) {
	cfg := &MirrorConfig{
		SoftwareList: []SoftwareConfig{
			{ID: "sw1"},
			{ID: "sw2", SyncRule: SyncRuleLatestOnly, KeepVersions: 5},
		},
	}
	cfg.ApplyDefaults()
	assert.Equal(t, SyncRuleIncremental, cfg.SoftwareList[0].SyncRule)
	assert.Equal(t, 1, cfg.SoftwareList[0].KeepVersions)
	assert.Equal(t, SyncRuleLatestOnly, cfg.SoftwareList[1].SyncRule)
	assert.Equal(t, 5, cfg.SoftwareList[1].KeepVersions)
}

func TestMirrorConfig_ApplyDefaults_SyncRule_InvalidFallback(t *testing.T) {
	cfg := &MirrorConfig{
		SoftwareList: []SoftwareConfig{
			{ID: "sw1", SyncRule: "nonexistent"},
		},
	}
	cfg.ApplyDefaults()
	assert.Equal(t, SyncRuleIncremental, cfg.SoftwareList[0].SyncRule)
}

func TestMirrorConfig_ApplyDefaults_KeepVersions_Zero(t *testing.T) {
	cfg := &MirrorConfig{
		SoftwareList: []SoftwareConfig{
			{ID: "sw1", KeepVersions: 0},
			{ID: "sw2", KeepVersions: -1},
		},
	}
	cfg.ApplyDefaults()
	assert.Equal(t, 1, cfg.SoftwareList[0].KeepVersions)
	assert.Equal(t, 1, cfg.SoftwareList[1].KeepVersions)
}

func TestMirrorConfig_ApplyDefaults_FullSyncOnce(t *testing.T) {
	cfg := &MirrorConfig{
		SoftwareList: []SoftwareConfig{
			{ID: "sw1", SyncRule: SyncRuleFullHistorical, FullSyncOnce: true},
		},
	}
	cfg.ApplyDefaults()
	assert.True(t, cfg.SoftwareList[0].FullSyncOnce)
}

// ============================================================
// MirrorConfig.ValidateSyncRules 测试
// ============================================================

func TestMirrorConfig_ValidateSyncRules_AllValid(t *testing.T) {
	cfg := &MirrorConfig{
		SoftwareList: []SoftwareConfig{
			{ID: "sw1", SyncRule: SyncRuleIncremental},
			{ID: "sw2", SyncRule: SyncRuleLatestOnly, KeepVersions: 3},
			{ID: "sw3", SyncRule: SyncRuleFullHistorical, FullSyncOnce: true},
			{ID: "sw4"},
		},
	}
	err := cfg.ValidateSyncRules()
	assert.NoError(t, err)
}

func TestMirrorConfig_ValidateSyncRules_InvalidRule(t *testing.T) {
	cfg := &MirrorConfig{
		SoftwareList: []SoftwareConfig{
			{ID: "sw1", SyncRule: "invalid_rule"},
		},
	}
	err := cfg.ValidateSyncRules()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "未知的 sync_rule")
}

func TestMirrorConfig_ValidateSyncRules_LatestOnly_KeepVersions(t *testing.T) {
	cfg := &MirrorConfig{
		SoftwareList: []SoftwareConfig{
			{ID: "sw1", SyncRule: SyncRuleLatestOnly, KeepVersions: 0},
		},
	}
	err := cfg.ValidateSyncRules()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "keep_versions 至少为 1")
}

func TestMirrorConfig_ValidateSyncRules_EmptyList(t *testing.T) {
	cfg := &MirrorConfig{SoftwareList: nil}
	err := cfg.ValidateSyncRules()
	assert.NoError(t, err)
}

func TestPostgresConfig_ApplyDefaults(t *testing.T) {
	cfg := &PostgresConfig{}
	cfg.ApplyDefaults()
	assert.Equal(t, 5432, cfg.Port)
	assert.Equal(t, "disable", cfg.SSLMode)
	assert.Equal(t, 10, cfg.MaxConns)
}
