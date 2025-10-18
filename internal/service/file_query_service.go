package service

import (
	"context"

	"s3-file-service/internal/core/s3"
	"s3-file-service/internal/util"

	"go.uber.org/zap"
)

// FileQueryService 文件查询服务
type FileQueryService struct {
	s3Client *s3.Client
}

// NewFileQueryService 创建文件查询服务
func NewFileQueryService(s3Client *s3.Client) *FileQueryService {
	return &FileQueryService{
		s3Client: s3Client,
	}
}

// queryFileListWithoutCache 查询文件列表但不缓存（降级策略）
func (s *FileQueryService) queryFileListWithoutCache(ctx context.Context, prefix string) ([]FileInfo, error) {
	objects, err := s.s3Client.ListObjects(ctx, prefix)
	if err != nil {
		util.Error("Failed to list S3 objects", zap.String("prefix", prefix), zap.Error(err))
		return nil, err
	}

	var fileList []FileInfo
	for _, obj := range objects {
		// 过滤掉目录样式的对象，通常是 Size==0 且 Key 以 '/' 结尾
		if *obj.Size == 0 && (*obj.Key)[len(*obj.Key)-1] == '/' {
			continue
		}
		fileList = append(fileList, FileInfo{
			Name:         *obj.Key,
			Key:          *obj.Key,
			Size:         *obj.Size,
			LastModified: *obj.LastModified,
		})
	}

	util.Info("Got file list without cache", zap.String("prefix", prefix), zap.Int("count", len(fileList)))
	return fileList, nil
}

// queryAndCacheFileList 查询 OSS 并缓存
func (s *FileQueryService) queryAndCacheFileList(ctx context.Context, prefix, cacheKey string, cacheService *FileCacheService) ([]FileInfo, error) {
	objects, err := s.s3Client.ListObjects(ctx, prefix)
	if err != nil {
		util.Error("Failed to list S3 objects", zap.String("prefix", prefix), zap.Error(err))
		return nil, err
	}

	var fileList []FileInfo
	for _, obj := range objects {
		// 过滤掉目录样式的对象，通常是 Size==0 且 Key 以 '/' 结尾
		if *obj.Size == 0 && (*obj.Key)[len(*obj.Key)-1] == '/' {
			continue
		}
		fileList = append(fileList, FileInfo{
			Name:         *obj.Key,
			Key:          *obj.Key,
			Size:         *obj.Size,
			LastModified: *obj.LastModified,
		})
	}

	// 使用分片缓存写入
	err = cacheService.cacheFileListInShards(ctx, prefix, fileList)
	if err != nil {
		util.Error("Failed to write cache", zap.String("prefix", prefix), zap.Error(err))
	}

	util.Info("Updated file list cache", zap.String("prefix", prefix), zap.Int("count", len(fileList)), zap.Int("shards", (len(fileList)+100-1)/100))
	return fileList, nil
}
