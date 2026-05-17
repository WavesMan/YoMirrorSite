// API 响应模型测试
// 测试 APIResponse 的 JSON 序列化和工厂方法

package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ============================================================
// APIResponse JSON 序列化
// ============================================================

func TestAPIResponse_Marshal(t *testing.T) {
	resp := APIResponse{
		Success: true,
		Data:    map[string]int{"count": 42},
		Total:   100,
	}
	data, err := json.Marshal(resp)
	assert.NoError(t, err)
	raw := string(data)
	assert.Contains(t, raw, `"success":true`)
	assert.Contains(t, raw, `"count":42`)
	assert.Contains(t, raw, `"total":100`)
}

func TestAPIResponse_ErrorMarshal(t *testing.T) {
	resp := APIResponse{
		Success: false,
		Error:   "not found",
	}
	data, err := json.Marshal(resp)
	assert.NoError(t, err)
	raw := string(data)
	assert.Contains(t, raw, `"success":false`)
	assert.Contains(t, raw, `"not found"`)
	assert.NotContains(t, raw, `"data"`) // omitempty
}

func TestAPIResponse_OmitEmpty(t *testing.T) {
	resp := APIResponse{Success: true}
	data, _ := json.Marshal(resp)
	raw := string(data)
	assert.NotContains(t, raw, `"error"`)
	assert.NotContains(t, raw, `"total"`)
}

// ============================================================
// SyncStatus 序列化
// ============================================================

func TestSyncStatus_Marshal(t *testing.T) {
	status := SyncStatus{
		InProgress:  true,
		CurrentJob:  "vscode",
		QueueLength: 3,
	}
	data, _ := json.Marshal(status)
	raw := string(data)
	assert.Contains(t, raw, `"in_progress":true`)
	assert.Contains(t, raw, `"vscode"`)
}

func TestSyncResultBrief_Marshal(t *testing.T) {
	result := SyncResultBrief{
		SoftwareID:  "docker",
		NewVersions: 2,
		NewAssets:   5,
		Duration:    "30s",
	}
	data, _ := json.Marshal(result)
	raw := string(data)
	assert.Contains(t, raw, `"docker"`)
	assert.Contains(t, raw, `"new_versions":2`)
	assert.NotContains(t, raw, "errors") // omitempty
}

// ============================================================
// MirrorStats 序列化
// ============================================================

func TestMirrorStats_Marshal(t *testing.T) {
	stats := MirrorStats{
		TotalSoftware:  12,
		TotalDownloads: 85200,
	}
	data, _ := json.Marshal(stats)
	raw := string(data)
	assert.Contains(t, raw, `"total_software":12`)
	assert.Contains(t, raw, `"total_downloads":85200`)
}
