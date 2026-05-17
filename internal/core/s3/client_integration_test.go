//go:build integration

// S3 集成测试
// 需要 MinIO service container 运行在 localhost:9000

package s3

import (
	"context"
	"os"
	"strings"
	"testing"

	"yomirrorsite/internal/config"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================
// 测试辅助
// ============================================================

func testS3Config() *config.S3Config {
	return &config.S3Config{
		AccessKey:  envOr("S3_ACCESS_KEY", "minioadmin"),
		SecretKey:  envOr("S3_SECRET_KEY", "minioadmin"),
		Endpoint:   envOr("S3_ENDPOINT", "http://localhost:9000"),
		BucketName: "yomirror-test",
		ListenDir:  "mirrors/",
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

var testS3Client *Client

func TestMain(m *testing.M) {
	cfg := testS3Config()
	testS3Client = NewClient(cfg)
	// 确保测试 bucket 存在
	testS3Client.client.CreateBucket(context.Background(),
		&s3.CreateBucketInput{Bucket: aws.String(cfg.BucketName)})
	os.Exit(m.Run())
}

// ============================================================
// Client 创建测试
// ============================================================

func TestIntegration_NewS3Client(t *testing.T) {
	cfg := testS3Config()
	client := NewClient(cfg)
	assert.NotNil(t, client)
}

// ============================================================
// Object CRUD 测试
// ============================================================

func TestIntegration_UploadAndDownload(t *testing.T) {
	cfg := testS3Config()
	client := NewClient(cfg)

	ctx := context.Background()
	key := "test/file.txt"
	content := "hello minio integration test"
	contentType := "text/plain"

	// 上传
	err := client.UploadObject(ctx, key, strings.NewReader(content), contentType)
	require.NoError(t, err)

	// 检查存在
	exists, err := client.ObjectExists(ctx, key)
	assert.NoError(t, err)
	assert.True(t, exists)

	// 生成下载 URL
	url, err := client.GetObjectURL(ctx, key, 600)
	assert.NoError(t, err)
	assert.Contains(t, url, "test/file.txt")
	assert.Contains(t, url, "X-Amz-")
}

func TestIntegration_ObjectNotExists(t *testing.T) {
	cfg := testS3Config()
	client := NewClient(cfg)

	exists, err := client.ObjectExists(context.Background(), "nonexistent/key.txt")
	assert.NoError(t, err)
	assert.False(t, exists)
}

func TestIntegration_ListObjects(t *testing.T) {
	cfg := testS3Config()
	client := NewClient(cfg)

	ctx := context.Background()

	// 上传测试文件
	require.NoError(t, client.UploadObject(ctx, "list/test1.txt", strings.NewReader("a"), "text/plain"))
	require.NoError(t, client.UploadObject(ctx, "list/test2.txt", strings.NewReader("b"), "text/plain"))

	objects, err := client.ListObjects(ctx, "list/")
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(objects), 2)
}

func TestIntegration_SearchFiles(t *testing.T) {
	cfg := testS3Config()
	client := NewClient(cfg)

	ctx := context.Background()
	require.NoError(t, client.UploadObject(ctx, "search/hello-world.zip", strings.NewReader(""), "application/zip"))

	results, err := client.SearchFiles(ctx, "hello", 10)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(results), 1)
}
