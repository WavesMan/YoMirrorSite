package s3

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"yomirrorsite/internal/util"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"go.uber.org/zap"
)

const (
	defaultPartSize = 5 * 1024 * 1024 // 5MB
)

// ParallelDownloader 并行下载器
type ParallelDownloader struct {
	client        *s3.Client
	bucketName    string
	partSize      int64
	parallelCount int
}

// NewParallelDownloader 创建并行下载器
func NewParallelDownloader(client *s3.Client, bucketName string, parallelCount int) *ParallelDownloader {
	return &ParallelDownloader{
		client:        client,
		bucketName:    bucketName,
		partSize:      defaultPartSize,
		parallelCount: parallelCount,
	}
}

// Download 并行下载文件
func (d *ParallelDownloader) Download(ctx context.Context, key string, filePath string) error {
	// 获取文件信息
	headInput := &s3.HeadObjectInput{
		Bucket: &d.bucketName,
		Key:    &key,
	}

	headOutput, err := d.client.HeadObject(ctx, headInput)
	if err != nil {
		util.Error("Failed to get object info", zap.String("key", key), zap.Error(err))
		return err
	}

	fileSize := headOutput.ContentLength
	if fileSize == nil || *fileSize == 0 {
		return errors.New("file is empty")
	}

	// 计算分片数量
	partCount := int((*fileSize + d.partSize - 1) / d.partSize)
	if partCount < 1 {
		partCount = 1
	}

	// 如果分片数量小于并行数，调整并行数
	if partCount < d.parallelCount {
		d.parallelCount = partCount
	}

	util.Info("Starting parallel download", zap.String("key", key), zap.Int64("file_size", *fileSize), zap.Int("part_count", partCount), zap.Int("parallel_count", d.parallelCount))

	// 创建本地文件
	file, err := os.Create(filePath)
	if err != nil {
		util.Error("Failed to create local file", zap.String("file_path", filePath), zap.Error(err))
		return err
	}
	defer file.Close()

	// 设置文件大小
	if err := file.Truncate(*fileSize); err != nil {
		util.Error("Failed to truncate file", zap.String("file_path", filePath), zap.Error(err))
		return err
	}

	// 创建等待组
	var wg sync.WaitGroup
	errChan := make(chan error, d.parallelCount)

	// 分片下载
	for i := 0; i < partCount; i++ {
		wg.Add(1)

		go func(partIndex int) {
			defer wg.Done()

			// 计算分片范围
			start := int64(partIndex) * d.partSize
			end := start + d.partSize - 1
			if end >= *fileSize {
				end = *fileSize - 1
			}

			// 下载分片
			err := d.downloadPart(ctx, key, file, start, end)
			if err != nil {
				errChan <- fmt.Errorf("failed to download part %d: %w", partIndex, err)
			}
		}(i)
	}

	// 等待所有分片下载完成
	wg.Wait()
	close(errChan)

	// 检查是否有错误
	if len(errChan) > 0 {
		return <-errChan
	}

	util.Info("Parallel download completed", zap.String("key", key), zap.String("file_path", filePath))
	return nil
}

// downloadPart 下载分片
func (d *ParallelDownloader) downloadPart(ctx context.Context, key string, file *os.File, start, end int64) error {
	// 设置范围请求头
	rangeHeader := fmt.Sprintf("bytes=%d-%d", start, end)

	// 下载分片
	getInput := &s3.GetObjectInput{
		Bucket: &d.bucketName,
		Key:    &key,
		Range:  &rangeHeader,
	}

	getOutput, err := d.client.GetObject(ctx, getInput)
	if err != nil {
		util.Error("Failed to get object part", zap.String("key", key), zap.Int64("start", start), zap.Int64("end", end), zap.Error(err))
		return err
	}
	defer getOutput.Body.Close()

	// 移动文件指针到分片位置
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		util.Error("Failed to seek file", zap.String("key", key), zap.Int64("start", start), zap.Error(err))
		return err
	}

	// 写入分片数据
	if _, err := io.Copy(file, getOutput.Body); err != nil {
		util.Error("Failed to write file part", zap.String("key", key), zap.Int64("start", start), zap.Int64("end", end), zap.Error(err))
		return err
	}

	util.Debug("Downloaded part", zap.String("key", key), zap.Int64("start", start), zap.Int64("end", end))
	return nil
}
