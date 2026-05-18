package s3

import (
	"context"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"go.uber.org/zap"

	"yomirrorsite/internal/config"
	"yomirrorsite/internal/util"
)

// Client S3客户端
type Client struct {
	client     *s3.Client
	bucketName string
	listenDir  string
	corsConfig config.CORSConfig
}

// NewClient 创建S3客户端
func NewClient(cfg *config.S3Config) *Client {
	// 验证配置
	if cfg.BucketName == "" {
		util.Error("Bucket name is empty in S3 config")
	} else {
		util.Debug("Creating S3 client", zap.String("bucket", cfg.BucketName), zap.String("endpoint", cfg.Endpoint))
	}

	awsCfg := cfg.AWSConfig()
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	return &Client{
		client:     client,
		bucketName: cfg.BucketName,
		listenDir:  cfg.ListenDir,
		corsConfig: cfg.CORS,
	}
}

// ListObjects 列出对象
func (c *Client) ListObjects(ctx context.Context, prefix string) ([]types.Object, error) {
	// 验证bucket名称
	if c.bucketName == "" {
		return nil, errors.New("bucket name is empty")
	}

	// 如果prefix已经包含listen_dir前缀，则不再重复添加
	fullPrefix := prefix
	if c.listenDir != "" && !strings.HasPrefix(prefix, c.listenDir) {
		fullPrefix = c.listenDir + prefix
	}

	input := &s3.ListObjectsV2Input{
		Bucket: &c.bucketName,
		Prefix: &fullPrefix,
	}

	util.Debug("Listing S3 objects", zap.String("bucket", c.bucketName), zap.String("prefix", fullPrefix))

	result, err := c.client.ListObjectsV2(ctx, input)
	if err != nil {
		var apiErr smithy.APIError
		if ok := errors.As(err, &apiErr); ok {
			util.Error("S3 API error", zap.String("code", apiErr.ErrorCode()), zap.String("message", apiErr.ErrorMessage()))
		}
		return nil, err
	}

	return result.Contents, nil
}

// GetObjectURL 生成对象下载URL
func (c *Client) GetObjectURL(ctx context.Context, key string, expiresIn int64) (string, error) {
	// 如果key已经包含listen_dir前缀，则不再重复添加
	fullKey := key
	if c.listenDir != "" && !strings.HasPrefix(key, c.listenDir) {
		fullKey = c.listenDir + key
	}

	presignClient := s3.NewPresignClient(c.client)

	input := &s3.GetObjectInput{
		Bucket: &c.bucketName,
		Key:    &fullKey,
	}

	// 确保过期时间在合理范围内
	if expiresIn <= 0 {
		expiresIn = 3600 // 默认1小时
	}
	if expiresIn > 7*24*3600 { // 最大7天
		expiresIn = 7 * 24 * 3600
	}

	presignedRequest, err := presignClient.PresignGetObject(ctx, input, func(po *s3.PresignOptions) {
		po.Expires = time.Duration(expiresIn) * time.Second
	})
	if err != nil {
		util.Error("Failed to generate presigned URL", zap.String("key", fullKey), zap.Error(err))
		return "", err
	}

	util.Debug("Generated presigned URL",
		zap.String("key", fullKey),
		zap.String("url", presignedRequest.URL),
		zap.Int64("expires_in", expiresIn))
	return presignedRequest.URL, nil
}

// DeleteObject 删除对象
func (c *Client) DeleteObject(ctx context.Context, input *s3.DeleteObjectInput) (*s3.DeleteObjectOutput, error) {
	return c.client.DeleteObject(ctx, input)
}

// DeleteObjects 批量删除对象
func (c *Client) DeleteObjects(ctx context.Context, input *s3.DeleteObjectsInput) (*s3.DeleteObjectsOutput, error) {
	return c.client.DeleteObjects(ctx, input)
}

// BucketName 返回当前使用的 S3 存储桶名称
func (c *Client) BucketName() string {
	return c.bucketName
}

// UploadObject 上传对象（仅用于同步器）
func (c *Client) UploadObject(ctx context.Context, key string, body io.Reader, contentType string) error {
	// 如果key已经包含listen_dir前缀，则不再重复添加
	fullKey := key
	if c.listenDir != "" && !strings.HasPrefix(key, c.listenDir) {
		fullKey = c.listenDir + key
	}

	input := &s3.PutObjectInput{
		Bucket:      &c.bucketName,
		Key:         &fullKey,
		Body:        body,
		ContentType: &contentType,
	}

	_, err := c.client.PutObject(ctx, input)
	if err != nil {
		var apiErr smithy.APIError
		if ok := errors.As(err, &apiErr); ok {
			util.Error("S3 API error", zap.String("code", apiErr.ErrorCode()), zap.String("message", apiErr.ErrorMessage()))
		}
		return err
	}

	return nil
}

// UploadObjectWithSize 上传对象（带文件大小）
func (c *Client) UploadObjectWithSize(ctx context.Context, key string, body io.Reader, contentLength int64, contentType string) error {
	// 如果key已经包含listen_dir前缀，则不再重复添加
	fullKey := key
	if c.listenDir != "" && !strings.HasPrefix(key, c.listenDir) {
		fullKey = c.listenDir + key
	}

	input := &s3.PutObjectInput{
		Bucket:        &c.bucketName,
		Key:           &fullKey,
		Body:          body,
		ContentLength: &contentLength,
		ContentType:   &contentType,
	}

	_, err := c.client.PutObject(ctx, input)
	if err != nil {
		var apiErr smithy.APIError
		if ok := errors.As(err, &apiErr); ok {
			util.Error("S3 API error", zap.String("code", apiErr.ErrorCode()), zap.String("message", apiErr.ErrorMessage()))
		}
		return err
	}

	util.Info("File uploaded to S3 successfully",
		zap.String("key", fullKey),
		zap.Int64("size", contentLength))
	return nil
}

// SearchFiles 在存储桶中搜索文件
func (c *Client) SearchFiles(ctx context.Context, keyword string, limit int) ([]types.Object, error) {
	util.Info("Starting file search", zap.String("keyword", keyword), zap.Int("limit", limit))

	// 获取所有文件列表
	allObjects, err := c.ListObjects(ctx, "")
	if err != nil {
		util.Error("Failed to list files for search", zap.Error(err))
		return nil, err
	}

	util.Info("Total files to search", zap.Int("count", len(allObjects)))

	// 执行搜索
	results := make([]types.Object, 0)
	keywordLower := strings.ToLower(keyword)

	for _, obj := range allObjects {
		// 检查文件名是否包含关键词（不区分大小写）
		fileName := extractFileNameFromKey(*obj.Key)
		if strings.Contains(strings.ToLower(fileName), keywordLower) {
			results = append(results, obj)

			// 如果达到限制数量，提前返回
			if limit > 0 && len(results) >= limit {
				break
			}
		}
	}

	util.Info("Search completed",
		zap.String("keyword", keyword),
		zap.Int("results", len(results)),
		zap.Int("limit", limit))

	return results, nil
}

// extractFileNameFromKey 从文件路径中提取文件名
func extractFileNameFromKey(key string) string {
	// 移除路径前缀，只保留文件名
	lastSlashIndex := strings.LastIndex(key, "/")
	if lastSlashIndex == -1 {
		return key
	}

	// 如果以斜杠结尾，说明是目录，需要进一步处理
	if lastSlashIndex == len(key)-1 {
		trimmed := key[:len(key)-1]
		lastSlash := strings.LastIndex(trimmed, "/")
		if lastSlash == -1 {
			return trimmed
		}
		return trimmed[lastSlash+1:]
	}

	return key[lastSlashIndex+1:]
}

// ObjectExists 检查对象是否存在
func (c *Client) ObjectExists(ctx context.Context, key string) (bool, error) {
	// 如果key已经包含listen_dir前缀，则不再重复添加
	fullKey := key
	if c.listenDir != "" && !strings.HasPrefix(key, c.listenDir) {
		fullKey = c.listenDir + key
	}

	input := &s3.HeadObjectInput{
		Bucket: &c.bucketName,
		Key:    &fullKey,
	}

	_, err := c.client.HeadObject(ctx, input)
	if err != nil {
		var apiErr smithy.APIError
		if ok := errors.As(err, &apiErr); ok {
			// 如果对象不存在，返回false而不是错误
			if apiErr.ErrorCode() == "NotFound" {
				return false, nil
			}
			util.Error("S3 API error", zap.String("code", apiErr.ErrorCode()), zap.String("message", apiErr.ErrorMessage()))
		}
		return false, err
	}

	return true, nil
}