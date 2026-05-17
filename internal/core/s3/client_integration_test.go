//go:build integration

package s3

import (
	"context"
	"fmt"
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
	fmt.Printf("DIAG: endpoint=%s bucket=%s access_key=%s secret_key=%s\n",
		cfg.Endpoint, cfg.BucketName, cfg.AccessKey, cfg.SecretKey)

	client := NewClient(cfg)

	// 1. 尝试列出 bucket 确认连通性
	fmt.Println("DIAG: trying ListBuckets...")
	buckets, err := client.client.ListBuckets(context.Background(), &s3.ListBucketsInput{})
	if err != nil {
		fmt.Printf("DIAG: ListBuckets FAIL: %v\n", err)
	} else {
		fmt.Printf("DIAG: ListBuckets OK, found %d buckets\n", len(buckets.Buckets))
		for _, b := range buckets.Buckets {
			fmt.Printf("DIAG:   bucket: %s\n", aws.ToString(b.Name))
		}
	}

	// 2. 创建 bucket
	fmt.Println("DIAG: trying CreateBucket...")
	_, err = client.client.CreateBucket(context.Background(),
		&s3.CreateBucketInput{Bucket: aws.String(cfg.BucketName)})
	if err != nil {
		msg := err.Error()
		if !strings.Contains(msg, "BucketAlreadyOwnedByYou") &&
			!strings.Contains(msg, "BucketAlreadyExists") {
			fmt.Printf("DIAG: CreateBucket FAIL: %v\n", err)
			panic("create bucket failed: " + msg)
		}
		fmt.Println("DIAG: CreateBucket OK (already exists)")
	} else {
		fmt.Println("DIAG: CreateBucket OK (newly created)")
	}

	// 3. 验证 bucket 存在
	fmt.Println("DIAG: trying HeadBucket...")
	_, err = client.client.HeadBucket(context.Background(),
		&s3.HeadBucketInput{Bucket: aws.String(cfg.BucketName)})
	if err != nil {
		fmt.Printf("DIAG: HeadBucket FAIL: %v\n", err)
		panic("head bucket failed: " + err.Error())
	}
	fmt.Println("DIAG: HeadBucket OK")

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

	fmt.Printf("DIAG-UPLOAD: key=%s len=%d\n", key, len(content))

	err := client.UploadObject(ctx, key, strings.NewReader(content), contentType)
	if err != nil {
		fmt.Printf("DIAG-UPLOAD: UploadObject FAIL: %v\n", err)
	}
	require.NoError(t, err)
	fmt.Println("DIAG-UPLOAD: UploadObject OK")

	exists, err := client.ObjectExists(ctx, key)
	if err != nil {
		fmt.Printf("DIAG-UPLOAD: ObjectExists FAIL: %v\n", err)
	}
	assert.NoError(t, err)
	assert.True(t, exists, "expected object %s to exist", key)

	url, err := client.GetObjectURL(ctx, key, 600)
	if err != nil {
		fmt.Printf("DIAG-UPLOAD: GetObjectURL FAIL: %v\n", err)
	}
	assert.NoError(t, err)
	assert.Contains(t, url, key)
	fmt.Println("DIAG-UPLOAD: OK")
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

	fmt.Println("DIAG-LIST: uploading test files...")
	err := client.UploadObject(ctx, "list/a.txt", strings.NewReader("a"), "text/plain")
	if err != nil {
		fmt.Printf("DIAG-LIST: upload a.txt FAIL: %v\n", err)
	}
	require.NoError(t, err)
	err = client.UploadObject(ctx, "list/b.txt", strings.NewReader("b"), "text/plain")
	if err != nil {
		fmt.Printf("DIAG-LIST: upload b.txt FAIL: %v\n", err)
	}
	require.NoError(t, err)
	fmt.Println("DIAG-LIST: uploads OK")

	objects, err := client.ListObjects(ctx, "list/")
	if err != nil {
		fmt.Printf("DIAG-LIST: ListObjects FAIL: %v\n", err)
	}
	assert.NoError(t, err)
	fmt.Printf("DIAG-LIST: found %d objects\n", len(objects))
	for _, o := range objects {
		fmt.Printf("DIAG-LIST:   %s\n", aws.ToString(o.Key))
	}
	assert.GreaterOrEqual(t, len(objects), 2, "expected >=2 objects, got %d", len(objects))
}

func TestIntegration_SearchFiles(t *testing.T) {
	client := NewClient(testS3Config())
	ctx := context.Background()

	fmt.Println("DIAG-SEARCH: uploading test file...")
	err := client.UploadObject(ctx, "search/hi.zip", strings.NewReader(""), "application/zip")
	if err != nil {
		fmt.Printf("DIAG-SEARCH: upload FAIL: %v\n", err)
	}
	require.NoError(t, err)
	fmt.Println("DIAG-SEARCH: upload OK")

	results, err := client.SearchFiles(ctx, "hi", 10)
	if err != nil {
		fmt.Printf("DIAG-SEARCH: SearchFiles FAIL: %v\n", err)
	}
	assert.NoError(t, err)
	fmt.Printf("DIAG-SEARCH: found %d results\n", len(results))
	assert.GreaterOrEqual(t, len(results), 1, "expected >=1 results, got %d", len(results))
}
