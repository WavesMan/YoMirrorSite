package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"yomirrorsite/api/handler"
	"yomirrorsite/api/router"
	"yomirrorsite/internal/config"
	"yomirrorsite/internal/core/github"
	"yomirrorsite/internal/core/s3"
	"yomirrorsite/internal/service"
	"yomirrorsite/internal/syncer"
	"yomirrorsite/internal/util"

	"github.com/gofiber/fiber/v2"
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

	// 填充镜像站配置默认值
	cfg.Mirror.ApplyDefaults()

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

	ctx := context.Background()

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
		fileService.StartCacheRefresher(ctx, prefixes, refreshInterval)
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

	// ============================================================
	// YoMirrorSite 新增模块初始化
	// ============================================================

	// 初始化 GitHub API 客户端
	// 复用 util/proxy.go 的 HTTP 客户端配置
	ghClient := github.NewClient(nil, cfg.Mirror.Sync.GitHubToken)

	// 初始化 GitHub Release 同步器
	ghSyncer := syncer.NewGitHubSyncer(ghClient, s3Client, &cfg.Mirror)

	// 初始化同步调度器
	scheduler := syncer.NewScheduler(ghSyncer, &cfg.Mirror)

	// 启动调度器（后台运行）
	scheduler.Start(ctx)
	defer scheduler.Stop()

	// 初始化软件业务服务
	softwareService := service.NewSoftwareService(s3Client, cacheManager)

	// 初始化软件处理器
	softwareHandler := handler.NewSoftwareHandler(softwareService)
	syncHandler := handler.NewSyncHandler(scheduler)

	// 设置路由
	app := router.SetupRouter(fileHandler, searchHandler, corsValidator)
	router.RegisterMirrorRoutes(app, softwareHandler, syncHandler)

	// 强化 /health 端点（WaveYo Layer0 要求：返回依赖组件状态）
	app.Get("/health", func(c *fiber.Ctx) error {
		deps := map[string]string{}
		if err := util.HealthCheck(c.Context()); err != nil {
			deps["redis"] = "error: " + err.Error()
		} else {
			deps["redis"] = "ok"
		}
		s3Status := "ok"
		// 简单的 S3 可达性检查：尝试 ListObjects 空前缀（1条即可）
		_, err := s3Client.ListObjects(c.Context(), "")
		if err != nil {
			s3Status = "error: " + err.Error()
		}
		deps["s3"] = s3Status

		status := "ok"
		for _, v := range deps {
			if v != "ok" {
				status = "degraded"
				break
			}
		}
		return c.JSON(fiber.Map{
			"status": status,
			"deps":   deps,
		})
	})

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