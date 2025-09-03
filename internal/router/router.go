package router

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/weiwangfds/scinote/config"
	"github.com/weiwangfds/scinote/internal/handler"
	"github.com/weiwangfds/scinote/internal/middleware"
	"github.com/weiwangfds/scinote/internal/service"
	"gorm.io/gorm"
)

// Router 路由配置
type Router struct {
	engine *gin.Engine
	db     *gorm.DB
}

// NewRouter 创建路由实例
func NewRouter(loggerMiddleware *middleware.LoggerMiddleware, db *gorm.DB, cfg *config.Config) *Router {
	// 设置Gin模式
	gin.SetMode(gin.ReleaseMode)

	engine := gin.New()

	// 初始化服务
	ossConfigService := service.NewOSSConfigService(db)
	fileService := service.NewFileService(db, cfg.File)
	ossSyncService := service.NewOSSyncService(db, fileService)
	// 设置OSS同步服务到文件服务中
	fileService.SetOSSSyncService(ossSyncService)

	// 初始化处理器
	ossHandler := handler.NewOSSHandler(ossConfigService, ossSyncService)
	fileHandler := handler.NewFileHandler(fileService)

	// 使用中间件
	engine.Use(gin.Recovery())
	engine.Use(loggerMiddleware.Logger())
	engine.Use(loggerMiddleware.RequestLogger())

	// 配置CORS
	engine.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"*"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           86400,
	}))

	// 健康检查
	engine.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"message": "Service is running",
		})
	})

	// API路由组
	api := engine.Group("/api/v1")
	{
		// 基础信息接口
		api.GET("/info", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"service": "Scinote Go Client",
				"version": "1.0.0",
				"status":  "running",
			})
		})

		// 数据库状态检查
		api.GET("/db/status", func(c *gin.Context) {
			sqlDB, err := db.DB()
			if err != nil {
				c.JSON(500, gin.H{
					"error": "Database connection error",
				})
				return
			}

			if err := sqlDB.Ping(); err != nil {
				c.JSON(500, gin.H{
					"error": "Database ping failed",
				})
				return
			}

			c.JSON(200, gin.H{
				"status": "Database connection OK",
			})
		})

		// OSS配置管理接口
		oss := api.Group("/oss")
		{
			// OSS配置CRUD
			oss.POST("/configs", ossHandler.CreateOSSConfig)
			oss.GET("/configs", ossHandler.ListOSSConfigs)
			oss.GET("/configs/:id", ossHandler.GetOSSConfig)
			oss.PUT("/configs/:id", ossHandler.UpdateOSSConfig)
			oss.DELETE("/configs/:id", ossHandler.DeleteOSSConfig)

			// OSS配置管理
			oss.POST("/configs/:id/activate", ossHandler.ActivateOSSConfig)
			oss.POST("/configs/:id/test", ossHandler.TestOSSConfig)
			oss.GET("/configs/active", ossHandler.GetActiveOSSConfig)
			oss.PUT("/configs/:id/toggle", ossHandler.ToggleOSSConfig)

			// OSS同步相关接口
		oss.POST("/sync/all", ossHandler.SyncAllFromOSS)
		oss.GET("/sync/scan", ossHandler.ScanAndCompareFiles)
		oss.GET("/sync/logs", ossHandler.GetSyncLogs)
		oss.GET("/sync/status/:fileID", ossHandler.GetFileSyncStatus)
		oss.POST("/sync/retry/:logID", ossHandler.RetryFailedSync)
		oss.POST("/sync/file/:fileID", ossHandler.SyncFileToOSS)
		oss.POST("/sync/batch", ossHandler.BatchSyncToOSS)
		}

		// 文件管理接口
		files := api.Group("/files")
		{
			// 文件CRUD操作
			files.POST("/upload", fileHandler.UploadFile)
			files.GET("", fileHandler.ListFiles)
			files.GET("/search", fileHandler.SearchFiles)
			files.GET("/stats", fileHandler.GetFileStats)
			files.GET("/:id", fileHandler.GetFile)
			files.GET("/:id/download", fileHandler.DownloadFile)
			files.PUT("/:id", fileHandler.UpdateFile)
			files.DELETE("/:id", fileHandler.DeleteFile)
		}
	}

	return &Router{
		engine: engine,
		db:     db,
	}
}

// GetEngine 获取Gin引擎
func (r *Router) GetEngine() *gin.Engine {
	return r.engine
}

// GetDB 获取数据库连接
func (r *Router) GetDB() *gorm.DB {
	return r.db
}
