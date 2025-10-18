package config

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
)

// Config 应用程序配置
type Config struct {
	Server ServerConfig `yaml:"server"`
	S3     S3Config     `yaml:"s3"`
	Redis  RedisConfig  `yaml:"redis"`
	Cache  CacheConfig  `yaml:"cache"`
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
