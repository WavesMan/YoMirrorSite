package service

import (
	"context"
	"fmt"
	"time"

	"yomirrorsite/internal/core/s3"
	"yomirrorsite/internal/util"

	"go.uber.org/zap"
)

const (
	cacheKeyPrefix   = "files_list_cache:"
	shardSize        = 100             // 每片100条记录
	cacheTTL         = 5 * time.Minute // 缓存TTL统一为5分钟
	refreshThreshold = 1 * time.Minute // 刷新阈值为1分钟
)

// FileService 文件业务服务
type FileService struct {
	s3Client           *s3.Client
	downloadURLManager *DownloadURLManager
	cacheService       *FileCacheService
	queryService       *FileQueryService
	localCacheManager  *util.CacheManager
	hotDataManager     *util.HotDataManager
}

// NewFileService 创建文件业务服务
func NewFileService(s3Client *s3.Client) *FileService {
	fileService := &FileService{
		s3Client: s3Client,
	}
	fileService.cacheService = NewFileCacheService(fileService)
	fileService.queryService = NewFileQueryService(s3Client)
	return fileService
}

// SetLocalCacheManager 设置本地缓存管理器
func (s *FileService) SetLocalCacheManager(cacheManager *util.CacheManager) {
	s.localCacheManager = cacheManager
	if cacheManager != nil {
		s.hotDataManager = util.NewHotDataManager(cacheManager, 10) // 默认阈值10次
		util.Info("Local cache manager set for file service")
	}
}

// InitDownloadURLManager 初始化下载URL管理器
func (s *FileService) InitDownloadURLManager(workerCount int, queueSize int) {
	if workerCount <= 0 {
		workerCount = 5 // 默认5个工作协程
	}
	if queueSize <= 0 {
		queueSize = 100 // 默认队列大小100
	}

	s.downloadURLManager = NewDownloadURLManager(workerCount, queueSize, s.s3Client, s)
	util.Info("Download URL manager initialized",
		zap.Int("worker_count", workerCount),
		zap.Int("queue_size", queueSize))
}

// FileInfo 文件信息
type FileInfo struct {
	Name         string    `json:"name"`
	Key          string    `json:"key"`
	Size         int64     `json:"size"`
	LastModified time.Time `json:"last_modified"`
}

// GetFileList 获取文件列表（带防击穿保护）
func (s *FileService) GetFileList(ctx context.Context, prefix string) ([]FileInfo, error) {
	cacheKey := cacheKeyPrefix + prefix

	// 1. 先尝试从本地缓存获取
	if s.localCacheManager != nil {
		if value, found := s.localCacheManager.Get(cacheKey); found {
			if fileList, ok := value.([]FileInfo); ok {
				// 记录热点数据访问
				if s.hotDataManager != nil {
					s.hotDataManager.RecordAccess(cacheKey)
				}
				util.Info("Got file list from local cache", zap.String("prefix", prefix), zap.Int("count", len(fileList)))
				return fileList, nil
			}
		}
	}

	// 2. 尝试从 Redis 分片缓存读取
	cachedFileList, found, err := s.cacheService.readFileListFromShards(ctx, prefix)
	if err != nil {
		util.Error("Failed to read cache", zap.String("prefix", prefix), zap.Error(err))
	}
	if found {
		// 如果缓存存在，先返回缓存，同时异步检查是否需要刷新（带锁保护）
		go s.cacheService.asyncRefreshCacheIfNeeded(prefix, cacheKey)

		// 将数据存入本地缓存
		if s.localCacheManager != nil {
			s.localCacheManager.Set(cacheKey, cachedFileList)
		}

		util.Info("Got file list from Redis cache", zap.String("prefix", prefix), zap.Int("count", len(cachedFileList)))
		return cachedFileList, nil
	}

	// 3. 缓存没命中时，用分布式锁防止击穿
	lockKey := "lock:" + cacheKey
	locked, err := util.AcquireLock(ctx, lockKey, 10*time.Second)
	if err != nil {
		util.Error("Failed to acquire lock", zap.String("prefix", prefix), zap.Error(err))
		// 降级策略：直接查询S3，不缓存
		return s.queryService.queryFileListWithoutCache(ctx, prefix)
	}
	if !locked {
		// 未获取锁，等待后重试读取缓存，避免击穿
		time.Sleep(100 * time.Millisecond)
		return s.GetFileList(ctx, prefix)
	}
	// 确保释放锁
	defer util.ReleaseLock(ctx, lockKey)

	// 4. 从S3查询并缓存
	return s.queryAndCacheFileList(ctx, prefix, cacheKey)
}

// queryAndCacheFileList 查询 OSS 并缓存
func (s *FileService) queryAndCacheFileList(ctx context.Context, prefix, cacheKey string) ([]FileInfo, error) {
	fileList, err := s.queryService.queryAndCacheFileList(ctx, prefix, cacheKey, s.cacheService)
	if err == nil && s.localCacheManager != nil {
		// 将数据存入本地缓存
		s.localCacheManager.Set(cacheKey, fileList)
	}
	return fileList, err
}

// GetDownloadURL 生成下载URL（带Redis缓存和异步队列）
func (s *FileService) GetDownloadURL(ctx context.Context, key string, expiresIn int64) (string, error) {
	cacheKey := fmt.Sprintf("download_url:%s:%d", key, expiresIn)

	// 1. 从缓存中获取
	var cachedURL string
	found, err := util.GetJSON(ctx, cacheKey, &cachedURL)
	if err != nil {
		util.Error("Failed to get cache for download URL", zap.String("key", cacheKey), zap.Error(err))
	}
	if found {
		util.Info("Cache hit for download URL", zap.String("key", cacheKey))
		// 异步检查是否需要刷新缓存
		go s.asyncRefreshDownloadURL(key, expiresIn)
		return cachedURL, nil
	}

	// 2. 缓存未命中，使用分布式锁防止击穿
	lockKey := "lock:" + cacheKey
	locked, err := util.AcquireLock(ctx, lockKey, 10*time.Second)
	if err != nil {
		util.Error("Failed to acquire lock", zap.String("key", key), zap.Error(err))
		// 降级策略：直接生成URL但不缓存
		return s.s3Client.GetObjectURL(ctx, key, expiresIn)
	}
	if !locked {
		// 未获取锁，等待后重试读取缓存
		time.Sleep(100 * time.Millisecond)
		return s.GetDownloadURL(ctx, key, expiresIn)
	}
	defer util.ReleaseLock(ctx, lockKey)

	// 3. 再次检查缓存（防止重复生成）
	found, err = util.GetJSON(ctx, cacheKey, &cachedURL)
	if err == nil && found {
		util.Info("Cache hit after acquiring lock", zap.String("key", cacheKey))
		return cachedURL, nil
	}

	// 4. 提交到异步队列生成URL
	if s.downloadURLManager != nil {
		return s.downloadURLManager.SubmitTask(ctx, key, expiresIn)
	}

	// 5. 降级：直接生成URL并缓存
	url, err := s.s3Client.GetObjectURL(ctx, key, expiresIn)
	if err != nil {
		util.Error("Failed to generate download URL", zap.String("key", key), zap.Error(err))
		return "", err
	}

	// 写入缓存
	err = util.SetJSON(ctx, cacheKey, url, time.Duration(expiresIn)*time.Second)
	if err != nil {
		util.Error("Failed to cache download URL", zap.String("key", cacheKey), zap.Error(err))
	}

	util.Info("Generated and cached download URL", zap.String("key", key), zap.String("url", url))
	return url, nil
}

// StartCacheRefresher 启动后台定时刷新任务
func (s *FileService) StartCacheRefresher(ctx context.Context, prefixes []string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for {
			select {
			case <-ticker.C:
				for _, prefix := range prefixes {
					cacheKey := cacheKeyPrefix + prefix
					if _, err := s.queryAndCacheFileList(ctx, prefix, cacheKey); err != nil {
						util.Error("Periodic cache refresh failed", zap.String("prefix", prefix), zap.Error(err))
					}
				}
			case <-ctx.Done():
				ticker.Stop()
				util.Info("Cache refresher stopped")
				return
			}
		}
	}()
	util.Info("Cache refresher started", zap.Duration("interval", interval), zap.Strings("prefixes", prefixes))
}

// ClearShardCache 清理分片缓存
func (s *FileService) ClearShardCache(ctx context.Context, prefix string) error {
	return s.cacheService.ClearShardCache(ctx, prefix)
}
