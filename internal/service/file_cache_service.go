package service

import (
	"context"
	"fmt"
	"time"

	"yomirrorsite/internal/util"

	"go.uber.org/zap"
)

// FileCacheService 文件缓存服务
type FileCacheService struct {
	fileService *FileService
}

// NewFileCacheService 创建文件缓存服务
func NewFileCacheService(fileService *FileService) *FileCacheService {
	return &FileCacheService{
		fileService: fileService,
	}
}

// cacheFileListInShards 分片写缓存
func (s *FileCacheService) cacheFileListInShards(ctx context.Context, prefix string, fileList []FileInfo) error {
	shardCount := (len(fileList) + 100 - 1) / 100 // shardSize = 100
	metaKey := "files_list_cache:" + prefix

	// 先写分片数据
	for i := 0; i < shardCount; i++ {
		start := i * 100
		end := (i + 1) * 100
		if end > len(fileList) {
			end = len(fileList)
		}
		shardKey := fmt.Sprintf("%s:shard:%d", metaKey, i)
		err := util.SetJSON(ctx, shardKey, fileList[start:end], 5*time.Minute) // cacheTTL = 5分钟
		if err != nil {
			return err
		}
	}

	// 写总缓存元信息，存 shardCount
	err := util.SetJSON(ctx, metaKey, shardCount, 5*time.Minute)
	if err != nil {
		return err
	}

	util.Debug("Cached file list in shards", zap.String("prefix", prefix), zap.Int("shardCount", shardCount), zap.Int("totalFiles", len(fileList)))
	return nil
}

// readFileListFromShards 分片读缓存
func (s *FileCacheService) readFileListFromShards(ctx context.Context, prefix string) ([]FileInfo, bool, error) {
	metaKey := "files_list_cache:" + prefix

	var shardCount int
	found, err := util.GetJSON(ctx, metaKey, &shardCount)
	if err != nil || !found {
		return nil, false, err
	}

	var fullList []FileInfo
	for i := 0; i < shardCount; i++ {
		shardKey := fmt.Sprintf("%s:shard:%d", metaKey, i)
		var shardData []FileInfo
		found, err := util.GetJSON(ctx, shardKey, &shardData)
		if err != nil || !found {
			// 某分片缺失，缓存不完整，需要刷新整缓存
			util.Warn("Shard cache missing", zap.String("prefix", prefix), zap.Int("shardIndex", i))
			return nil, false, nil
		}
		fullList = append(fullList, shardData...)
	}

	util.Debug("Read file list from shards", zap.String("prefix", prefix), zap.Int("shardCount", shardCount), zap.Int("totalFiles", len(fullList)))
	return fullList, true, nil
}

// ClearShardCache 清理分片缓存
func (s *FileCacheService) ClearShardCache(ctx context.Context, prefix string) error {
	metaKey := "files_list_cache:" + prefix

	// 先获取分片数量
	var shardCount int
	found, err := util.GetJSON(ctx, metaKey, &shardCount)
	if err != nil {
		return err
	}

	if found {
		// 删除所有分片
		for i := 0; i < shardCount; i++ {
			shardKey := fmt.Sprintf("%s:shard:%d", metaKey, i)
			if err := util.Delete(ctx, shardKey); err != nil {
				util.Error("Failed to delete shard cache", zap.String("prefix", prefix), zap.Int("shardIndex", i), zap.Error(err))
			}
		}
	}

	// 删除元数据
	if err := util.Delete(ctx, metaKey); err != nil {
		return err
	}

	util.Info("Cleared shard cache", zap.String("prefix", prefix))
	return nil
}

// asyncRefreshCacheWithLock 异步刷新缓存（带锁保护）
func (s *FileCacheService) asyncRefreshCacheWithLock(prefix, cacheKey string) {
	lockKey := "lock:" + cacheKey
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	locked, err := util.AcquireLock(ctx, lockKey, 10*time.Second)
	if err != nil || !locked {
		// 若不能获取锁，则认为已有刷新任务，直接返回
		return
	}
	defer util.ReleaseLock(ctx, lockKey)

	_, err = s.fileService.queryAndCacheFileList(ctx, prefix, cacheKey)
	if err != nil {
		util.Error("Failed to async refresh cache", zap.String("prefix", prefix), zap.Error(err))
	}
}

// asyncRefreshCacheIfNeeded 异步检查并刷新缓存（仅在必要时）
func (s *FileCacheService) asyncRefreshCacheIfNeeded(prefix, cacheKey string) {
	lockKey := "lock:" + cacheKey
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 检查缓存新鲜度 - 避免频繁刷新
	metaKey := "files_list_cache:" + prefix
	ttl, err := util.GetTTL(ctx, metaKey)
	if err != nil {
		util.Debug("Failed to check cache TTL", zap.String("prefix", prefix), zap.Error(err))
		return
	}

	// 如果缓存还有超过1分钟的TTL，不需要刷新
	if ttl > 1*time.Minute {
		util.Debug("Cache is still fresh, skipping refresh", zap.String("prefix", prefix), zap.Duration("remaining_ttl", ttl))
		return
	}

	// 获取锁进行刷新
	locked, err := util.AcquireLock(ctx, lockKey, 10*time.Second)
	if err != nil || !locked {
		// 若不能获取锁，则认为已有刷新任务，直接返回
		util.Debug("Refresh lock already acquired by another process", zap.String("prefix", prefix))
		return
	}
	defer util.ReleaseLock(ctx, lockKey)

	util.Info("Starting async cache refresh", zap.String("prefix", prefix), zap.Duration("remaining_ttl", ttl))
	_, err = s.fileService.queryAndCacheFileList(ctx, prefix, cacheKey)
	if err != nil {
		util.Error("Failed to async refresh cache", zap.String("prefix", prefix), zap.Error(err))
	} else {
		util.Info("Async cache refresh completed", zap.String("prefix", prefix))
	}
}
