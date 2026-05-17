// 软件数据模型单元测试
// 测试 FormatSize、Software 结构等纯函数

package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ============================================================
// FormatSize 测试
// ============================================================

func TestFormatSize_Bytes(t *testing.T) {
	tests := []struct {
		name     string
		bytes    int64
		expected string
	}{
		{"zero", 0, "0 B"},
		{"1 byte", 1, "1 B"},
		{"1023 bytes", 1023, "1023 B"},
		{"1 KB", 1024, "1.0 KB"},
		{"1.5 KB", 1536, "1.5 KB"},
		{"1 MB", 1048576, "1.0 MB"},
		{"2.5 MB", 2621440, "2.5 MB"},
		{"1 GB", 1073741824, "1.0 GB"},
		{"1 TB", 1099511627776, "1.0 TB"},
		{"negative", -1024, "-1.0 KB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatSize(tt.bytes)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ============================================================
// APIResponse 结构测试
// ============================================================

func TestAPIResponse_Success(t *testing.T) {
	resp := APIResponse{
		Success: true,
		Data:    map[string]string{"key": "value"},
	}
	assert.True(t, resp.Success)
	assert.NotNil(t, resp.Data)
	assert.Empty(t, resp.Error)
}

func TestAPIResponse_Error(t *testing.T) {
	resp := APIResponse{
		Success: false,
		Error:   "something went wrong",
	}
	assert.False(t, resp.Success)
	assert.Equal(t, "something went wrong", resp.Error)
	assert.Nil(t, resp.Data)
}

func TestAPIResponse_WithTotal(t *testing.T) {
	resp := APIResponse{
		Success: true,
		Total:   42,
	}
	assert.Equal(t, 42, resp.Total)
}

// ============================================================
// SoftwareListPage 测试
// ============================================================

func TestSoftwareListPage_EmptyPage(t *testing.T) {
	page := SoftwareListPage{
		Items:      []Software{},
		Page:       3,
		PageSize:   20,
		TotalCount: 100,
	}
	assert.Empty(t, page.Items)
	assert.Equal(t, 3, page.Page)
	assert.Equal(t, 100, page.TotalCount)
}
