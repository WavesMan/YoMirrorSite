package router

import (
	"os"
	"path/filepath"

	"yomirrorsite/api/handler"
	"yomirrorsite/internal/core/s3"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

// SetupRouter 设置路由
func SetupRouter(fileHandler *handler.FileHandler, searchHandler *handler.SearchHandler, corsValidator *s3.CORSValidator) *fiber.App {
	// 创建Fiber应用
	app := fiber.New(fiber.Config{
		AppName: "YoMirrorSite",
	})

	// 注册CORS中间件
	app.Use(cors.New(cors.Config{
		AllowOrigins:     corsValidator.GetAllowedOrigins(),
		AllowMethods:     corsValidator.GetAllowedMethods(),
		AllowHeaders:     corsValidator.GetAllowedHeaders(),
		AllowCredentials: true,
		MaxAge:           corsValidator.GetMaxAge(),
	}))

	// 配置静态资源服务
	staticDir := "./web/dist"
	app.Static("/js", filepath.Join(staticDir, "js"))
	app.Static("/css", filepath.Join(staticDir, "css"))
	app.Static("/assets", filepath.Join(staticDir, "assets"))

	// 处理根目录下的静态文件（如favicon.ico, vite.svg等）
	app.Get("/favicon.ico", func(c *fiber.Ctx) error {
		return c.SendFile(filepath.Join(staticDir, "favicon.ico"))
	})
	app.Get("/vite.svg", func(c *fiber.Ctx) error {
		return c.SendFile(filepath.Join(staticDir, "vite.svg"))
	})

	// API路由组
	api := app.Group("/api")
	{
		// 文件相关路由
		files := api.Group("/files")
		{
			files.Get("/", fileHandler.GetFileList)
			// 将通配符路由放在最后，避免路由冲突
			files.Get("/url/*", fileHandler.GetDownloadURL)
		}

		// 搜索相关路由
		search := api.Group("/search")
		{
			search.Get("/files", searchHandler.SearchFiles)
		}
	}

	// 健康检查（基础版，具体依赖检查在 main.go 中覆盖）
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status": "ok",
		})
	})

	// 配置前端路由，支持SPA应用
	app.Use(func(c *fiber.Ctx) error {
		// 检查请求是否为API请求
		if len(c.Path()) >= 5 && c.Path()[:5] == "/api/" {
			return c.Next()
		}

		// 检查前端SPA应用的index.html
		indexPath := filepath.Join(staticDir, "index.html")
		if _, err := os.Stat(indexPath); err == nil {
			return c.SendFile(indexPath)
		}

		// 如果index.html不存在，返回404
		return c.Status(404).SendString("Not Found")
	})

	return app
}

// ============================================================
// 镜像站路由注册（YoMirrorSite 新增）
// ============================================================

// RegisterMirrorRoutes 在已有 Fiber app 上注册镜像站路由
// 与 SetupRouter 分离，降低耦合
func RegisterMirrorRoutes(app *fiber.App, softwareHandler *handler.SoftwareHandler, syncHandler *handler.SyncHandler) {
	api := app.Group("/api/mirror")
	{
		api.Get("/software", softwareHandler.ListSoftware)                     // 软件列表
		api.Get("/software/:id", softwareHandler.GetSoftware)                  // 软件详情
		api.Get("/software/:id/versions/:tag", softwareHandler.GetVersion)      // 版本详情
		api.Get("/software/:id/download/:tag/:asset", softwareHandler.GetDownloadURL) // 下载
		api.Get("/stats", softwareHandler.GetStats)                            // 镜像站统计
	}

	sync := app.Group("/api/sync")
	{
		sync.Get("/status", syncHandler.GetStatus)    // 同步状态
		sync.Post("/trigger", syncHandler.TriggerSync) // 手动触发同步
	}
}