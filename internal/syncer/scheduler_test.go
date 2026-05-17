// 调度器单元测试
// 测试 NewScheduler 默认值逻辑

package syncer

import (
	"testing"
	"time"

	"yomirrorsite/internal/config"

	"github.com/stretchr/testify/assert"
)

// ============================================================
// NewScheduler 测试
// ============================================================

func TestNewScheduler_Defaults(t *testing.T) {
	cfg := &config.MirrorConfig{
		SoftwareList: []config.SoftwareConfig{
			{ID: "test"},
		},
		Sync: config.SyncConfig{
			IntervalMinutes: 0,
			MaxConcurrent:   0,
		},
	}
	s := NewScheduler(nil, cfg)
	assert.NotNil(t, s)
	assert.Equal(t, 30*time.Minute, s.interval)
	assert.Equal(t, 3, s.maxConcurrent)
	assert.Len(t, s.softwares, 1)
}

func TestNewScheduler_Custom(t *testing.T) {
	cfg := &config.MirrorConfig{
		SoftwareList: []config.SoftwareConfig{
			{ID: "test"},
			{ID: "test2"},
		},
		Sync: config.SyncConfig{
			IntervalMinutes: 15,
			MaxConcurrent:   5,
		},
	}
	s := NewScheduler(nil, cfg)
	assert.Equal(t, 15*time.Minute, s.interval)
	assert.Equal(t, 5, s.maxConcurrent)
	assert.Len(t, s.softwares, 2)
}

func TestNewScheduler_ZeroInterval(t *testing.T) {
	cfg := &config.MirrorConfig{
		Sync: config.SyncConfig{IntervalMinutes: -1},
	}
	s := NewScheduler(nil, cfg)
	// Negative or zero → defaults to 30 minutes
	assert.Equal(t, 30*time.Minute, s.interval)
}
