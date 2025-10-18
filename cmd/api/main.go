package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"s3-file-service/api/handler"
	"s3-file-service/api/router"
	"s3-file-service/internal/config"
	"s3-file-service/internal/core/s3"
	"s3-file-service/internal/service"
	"s3-file-service/internal/util"

	"go.uber.org/zap"
)

func main() {
	// 初始化日志
	if err := util.InitLogger(true); err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer util.SyncLogger()

	// 加载配置
	cfg, err := config.LoadDefaultConfig()
	if err != nil {
		util.Fatal("Failed to load config", zap.Error(err))
	}

	// 验证配置
	if err := cfg.Validate(); err != nil {
		util.Fatal("Invalid configuration", zap.Error(err))
	}

	// 调试：检查S3配置
	util.Debug("S3 config loaded",
		zap.String("bucket_name", cfg.S3.BucketName),
		zap.String("endpoint", cfg.S3.Endpoint),
		zap.String("access_key", cfg.S3.AccessKey),
		zap.String("listen_dir", cfg.S3.ListenDir),
	)

	// 初始化协程池
	if err := util.InitGoroutinePool(cfg.Server.GoroutinePoolSize); err != nil {
		util.Fatal("Failed to initialize goroutine pool", zap.Error(err))
	}
	defer util.ReleaseGoroutinePool()

	// 初始化Redis客户端
	util.InitRedisClient(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
	defer util.CloseRedisClient()

	// 初始化本地缓存管理器
	var cacheManager *util.CacheManager
	if cfg.Cache.LocalCache.Enabled {
		cacheManager = util.NewCacheManager(cfg.Cache.LocalCache.Size, cfg.Cache.LocalCache.Mode)
		util.Info("Local cache manager initialized",
			zap.Int("size", cfg.Cache.LocalCache.Size),
			zap.String("mode", cfg.Cache.LocalCache.Mode))
	} else {
		util.Info("Local cache is disabled")
	}

	// 初始化S3客户端
	s3Client := s3.NewClient(&cfg.S3)

	// 初始化CORS验证器
	corsValidator := s3.NewCORSValidator(cfg.S3.CORS)

	// 初始化文件服务
	fileService := service.NewFileService(s3Client)

	// 设置本地缓存管理器
	if cacheManager != nil {
		fileService.SetLocalCacheManager(cacheManager)
	}

	// 启动缓存刷新任务
	if cfg.Cache.FilesListRefreshIntervalSec > 0 {
		refreshInterval := time.Duration(cfg.Cache.FilesListRefreshIntervalSec) * time.Second
		prefixes := []string{""} // 根目录和常用目录
		fileService.StartCacheRefresher(context.Background(), prefixes, refreshInterval)
	}

	// 启动热点数据动态加载任务
	if cacheManager != nil && cfg.Cache.LocalCache.HotDataRefreshSec > 0 {
		go startDynamicCacheLoader(cacheManager, fileService, cfg.Cache.LocalCache.HotDataRefreshSec)
	}

	// 初始化文件处理器
	fileHandler := handler.NewFileHandler(fileService)

	// 初始化搜索服务
	searchService := service.NewSearchService(s3Client)

	// 初始化搜索处理器
	searchHandler := handler.NewSearchHandler(searchService)

	// 设置路由
	app := router.SetupRouter(fileHandler, searchHandler, corsValidator)

	// 启动服务器（异步）
	go func() {
		util.Info("Starting API server", zap.Int("port", cfg.Server.Port))
		if err := app.Listen(fmt.Sprintf(":%d", cfg.Server.Port)); err != nil {
			util.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	util.Info("Shutting down server...")

	// 关闭服务器
	if err := app.Shutdown(); err != nil {
		util.Fatal("Server forced to shutdown", zap.Error(err))
	}

	util.Info("Server exiting")
}

// startDynamicCacheLoader 定期动态载入热点数据到本地缓存
func startDynamicCacheLoader(cacheManager *util.CacheManager, fileService *service.FileService, refreshSec int) {
	ticker := time.NewTicker(time.Duration(refreshSec) * time.Second)
	defer ticker.Stop()

	util.Info("Dynamic cache loader started", zap.Int("refresh_sec", refreshSec))

	for range ticker.C {
		// 获取缓存统计信息
		stats := cacheManager.GetStats()
		util.Debug("Cache stats",
			zap.Int64("hits", stats.Hits),
			zap.Int64("misses", stats.Misses),
			zap.Float64("hit_rate", stats.HitRate),
			zap.Int("size", stats.Size))

		// 这里可以添加更复杂的热点数据识别逻辑
		// 例如：基于访问频率、业务规则等识别热点数据

		// 示例：预加载一些常用数据
		hotKeys := []string{"files_list_cache:", "files_list_cache:papermc", "files_list_cache:purpur"}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

		for _, key := range hotKeys {
			// 如果本地缓存中没有该数据，尝试从Redis加载
			if !cacheManager.Contains(key) {
				prefix := key[len("files_list_cache:"):]
				fileList, err := fileService.GetFileList(ctx, prefix)
				if err == nil {
					cacheManager.Set(key, fileList)
					util.Debug("Preloaded hot data to local cache", zap.String("key", key), zap.Int("count", len(fileList)))
				}
			}
		}

		cancel()
	}
}
