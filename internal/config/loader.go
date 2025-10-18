package config

import (
	"fmt"
	"path/filepath"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
)

// LoadConfig 加载配置文件
func LoadConfig(configPath string) (*Config, error) {
	// 创建viper实例
	v := viper.New()

	// 设置配置文件路径和名称
	cleanPath := filepath.Clean(configPath)
	v.SetConfigFile(cleanPath)

	// 启用环境变量绑定
	v.AutomaticEnv()
	v.SetEnvPrefix("S3_FILE_SERVICE")

	// 读取配置文件
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// 解析配置文件到结构体
	var config Config

	// 使用明确的解码器配置
	decoderConfig := &mapstructure.DecoderConfig{
		Metadata: nil,
		Result:   &config,
		TagName:  "yaml",
	}

	decoder, err := mapstructure.NewDecoder(decoderConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create decoder: %w", err)
	}

	if err := decoder.Decode(v.AllSettings()); err != nil {
		return nil, fmt.Errorf("failed to decode config: %w", err)
	}

	// 调试：打印加载的配置信息
	fmt.Printf("DEBUG: Loaded config from: %s\n", cleanPath)
	fmt.Printf("DEBUG: S3 BucketName: '%s'\n", config.S3.BucketName)
	fmt.Printf("DEBUG: S3 Endpoint: '%s'\n", config.S3.Endpoint)
	fmt.Printf("DEBUG: S3 AccessKey: '%s'\n", config.S3.AccessKey)
	fmt.Printf("DEBUG: S3 SecretKey: '%s'\n", config.S3.SecretKey)
	fmt.Printf("DEBUG: S3 ListenDir: '%s'\n", config.S3.ListenDir)

	// 调试：打印所有配置键
	fmt.Printf("DEBUG: All config keys:\n")
	for _, key := range v.AllKeys() {
		fmt.Printf("  %s: %v\n", key, v.Get(key))
	}

	return &config, nil
}

// LoadDefaultConfig 加载默认配置文件
func LoadDefaultConfig() (*Config, error) {
	return LoadConfig("configs/config.yaml")
}
