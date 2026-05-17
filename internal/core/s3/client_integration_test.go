//go:build integration

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

func TestMain(m *testing.M) {
	cfg := testS3Config()
	client := NewClient(cfg)
	_, err := client.client.CreateBucket(context.Background(),
		&s3.CreateBucketInput{Bucket: aws.String(cfg.BucketName)})
	if err != nil {
		// BucketAlreadyOwnedByYou / BucketAlreadyExists 均可接受
		msg := err.Error()
		if !strings.Contains(msg, "BucketAlready") {
			panic("create bucket failed: " + msg)
		}
	}
	os.Exit(m.Run())
}

func TestIntegration_NewS3Client(t *testing.T) {
	client := NewClient(testS3Config())
	assert.NotNil(t, client)
}

func TestIntegration_UploadAndDownload(t *testing.T) {
	client := NewClient(testS3Config())
	ctx := context.Background()
	key := "test/file.txt"
	content := "hello minio"
	contentType := "text/plain"

	err := client.UploadObject(ctx, key, strings.NewReader(content), contentType)
	require.NoError(t, err)

	exists, err := client.ObjectExists(ctx, key)
	assert.NoError(t, err)
	assert.True(t, exists)

	url, err := client.GetObjectURL(ctx, key, 600)
	assert.NoError(t, err)
	assert.Contains(t, url, key)
}

func TestIntegration_ObjectNotExists(t *testing.T) {
	client := NewClient(testS3Config())
	exists, err := client.ObjectExists(context.Background(), "nonexistent/key.txt")
	assert.NoError(t, err)
	assert.False(t, exists)
}

func TestIntegration_ListObjects(t *testing.T) {
	client := NewClient(testS3Config())
	ctx := context.Background()
	require.NoError(t, client.UploadObject(ctx, "list/a.txt", strings.NewReader("a"), "text/plain"))
	require.NoError(t, client.UploadObject(ctx, "list/b.txt", strings.NewReader("b"), "text/plain"))
	objects, err := client.ListObjects(ctx, "list/")
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(objects), 2)
}

func TestIntegration_SearchFiles(t *testing.T) {
	client := NewClient(testS3Config())
	ctx := context.Background()
	require.NoError(t, client.UploadObject(ctx, "search/hi.zip", strings.NewReader(""), "application/zip"))
	results, err := client.SearchFiles(ctx, "hi", 10)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(results), 1)
}
