// CORS 验证器单元测试
// 测试纯函数：GetAllowedOrigins / GetAllowedMethods / GetAllowedHeaders / GetMaxAge

package s3

import (
	"testing"

	"yomirrorsite/internal/config"

	"github.com/stretchr/testify/assert"
)

// ============================================================
// GetAllowedOrigins 测试
// ============================================================

func TestNewCORSValidator(t *testing.T) {
	v := NewCORSValidator(config.CORSConfig{})
	assert.NotNil(t, v)
}

func TestGetAllowedOrigins_Empty(t *testing.T) {
	v := NewCORSValidator(config.CORSConfig{})
	assert.Equal(t, "*", v.GetAllowedOrigins())
}

func TestGetAllowedOrigins_Single(t *testing.T) {
	v := NewCORSValidator(config.CORSConfig{
		AllowedOrigins: []string{"https://example.com"},
	})
	assert.Equal(t, "https://example.com", v.GetAllowedOrigins())
}

func TestGetAllowedOrigins_Multiple(t *testing.T) {
	v := NewCORSValidator(config.CORSConfig{
		AllowedOrigins: []string{"https://a.com", "https://b.com"},
	})
	result := v.GetAllowedOrigins()
	assert.Contains(t, result, "https://a.com")
	assert.Contains(t, result, "https://b.com")
	assert.Contains(t, result, ",")
}

// ============================================================
// GetAllowedMethods 测试
// ============================================================

func TestGetAllowedMethods_Empty(t *testing.T) {
	v := NewCORSValidator(config.CORSConfig{})
	assert.Contains(t, v.GetAllowedMethods(), "GET")
	assert.Contains(t, v.GetAllowedMethods(), "POST")
}

func TestGetAllowedMethods_Custom(t *testing.T) {
	v := NewCORSValidator(config.CORSConfig{
		AllowedMethods: []string{"GET", "HEAD"},
	})
	assert.Equal(t, "GET,HEAD", v.GetAllowedMethods())
}

// ============================================================
// GetAllowedHeaders 测试
// ============================================================

func TestGetAllowedHeaders_Empty(t *testing.T) {
	v := NewCORSValidator(config.CORSConfig{})
	assert.Equal(t, "*", v.GetAllowedHeaders())
}

func TestGetAllowedHeaders_Custom(t *testing.T) {
	v := NewCORSValidator(config.CORSConfig{
		AllowedHeaders: []string{"Authorization", "X-Custom"},
	})
	assert.Contains(t, v.GetAllowedHeaders(), "Authorization")
	assert.Contains(t, v.GetAllowedHeaders(), "X-Custom")
}

// ============================================================
// GetMaxAge 测试
// ============================================================

func TestGetMaxAge_Zero(t *testing.T) {
	v := NewCORSValidator(config.CORSConfig{})
	assert.Equal(t, 0, v.GetMaxAge())
}

func TestGetMaxAge_Custom(t *testing.T) {
	v := NewCORSValidator(config.CORSConfig{MaxAgeSeconds: 3600})
	assert.Equal(t, 3600, v.GetMaxAge())
}
