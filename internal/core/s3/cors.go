package s3

import (
	"strings"

	"yomirrorsite/internal/config"
)

// CORSValidator CORS验证器
type CORSValidator struct {
	config config.CORSConfig
}

// NewCORSValidator 创建CORS验证器
func NewCORSValidator(cfg config.CORSConfig) *CORSValidator {
	return &CORSValidator{
		config: cfg,
	}
}

// GetAllowedOrigins 获取允许的Origin列表（Fiber兼容）
func (v *CORSValidator) GetAllowedOrigins() string {
	if len(v.config.AllowedOrigins) == 0 {
		return "*"
	}
	return strings.Join(v.config.AllowedOrigins, ",")
}

// GetAllowedMethods 获取允许的方法列表（Fiber兼容）
func (v *CORSValidator) GetAllowedMethods() string {
	if len(v.config.AllowedMethods) == 0 {
		return "GET,POST,PUT,DELETE,OPTIONS"
	}
	return strings.Join(v.config.AllowedMethods, ",")
}

// GetAllowedHeaders 获取允许的Header列表（Fiber兼容）
func (v *CORSValidator) GetAllowedHeaders() string {
	if len(v.config.AllowedHeaders) == 0 {
		return "*"
	}
	return strings.Join(v.config.AllowedHeaders, ",")
}

// GetMaxAge 获取最大缓存时间（Fiber兼容）
func (v *CORSValidator) GetMaxAge() int {
	return v.config.MaxAgeSeconds
}
