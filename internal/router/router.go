package router

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/weiwangfds/scinote/config"
	_ "github.com/weiwangfds/scinote/docs" // swagger docs
	"github.com/weiwangfds/scinote/internal/handler"
	"github.com/weiwangfds/scinote/internal/middleware"
	fileservice "github.com/weiwangfds/scinote/internal/service/file"
	noteservice "github.com/weiwangfds/scinote/internal/service/note"
	ossservice "github.com/weiwangfds/scinote/internal/service/oss"
	tagservice "github.com/weiwangfds/scinote/internal/service/tag"
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
	ossConfigService := ossservice.NewOSSConfigService(db)
	fileService := fileservice.NewFileService(db, cfg.File)
	ossSyncService := ossservice.NewOSSyncService(db, fileService)
	// 设置OSS同步服务到文件服务中
	fileService.SetOSSSyncService(ossSyncService)

	// 初始化笔记服务
	noteService := noteservice.NewNoteService(db, fileService)

	// 初始化标签服务
	tagService := tagservice.NewTagService(db)

	// 初始化处理器
	ossHandler := handler.NewOSSHandler(ossConfigService, ossSyncService)
	fileHandler := handler.NewFileHandler(fileService)
	noteHandler := handler.NewNoteHandler(noteService)
	tagHandler := handler.NewTagHandler(tagService)

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

	// Swagger文档路由
	engine.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

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

		// 笔记管理接口
		notes := api.Group("/notes")
		{
			// 笔记基础CRUD操作
			notes.POST("", noteHandler.CreateNote)
			notes.GET("/:id", noteHandler.GetNote)
			notes.PUT("/:id", noteHandler.UpdateNote)
			notes.DELETE("/:id", noteHandler.DeleteNote)

			// 笔记层级结构操作
			notes.GET("/children", noteHandler.GetNoteChildren)     // 获取根笔记
			notes.GET("/:id/children", noteHandler.GetNoteChildren) // 获取子笔记
			notes.GET("/tree", noteHandler.GetNoteTree)             // 获取笔记树

			// 笔记移动操作
			notes.POST("/:id/move", noteHandler.MoveNote)          // 移动单个笔记
			notes.POST("/batch-move", noteHandler.BatchMoveNotes)  // 批量移动笔记
			notes.POST("/:id/move-tree", noteHandler.MoveNoteTree) // 移动笔记树

			// 笔记搜索
			notes.GET("/search", noteHandler.SearchNotes)

			// 笔记标签管理
			notes.POST("/:id/tags", noteHandler.AddNoteTag)              // 添加标签
			notes.DELETE("/:id/tags/:tag_id", noteHandler.RemoveNoteTag) // 移除标签

			// 笔记扩展属性管理
			notes.POST("/:id/properties", noteHandler.SetNoteProperty)  // 设置属性
			notes.GET("/:id/properties", noteHandler.GetNoteProperties) // 获取属性
		}

		// 标签管理接口
		tags := api.Group("/tags")
		{
			// 标签基础CRUD操作
			tags.POST("", tagHandler.CreateTag)       // 创建标签
			tags.GET("", tagHandler.GetAllTags)       // 获取标签列表
			tags.GET("/:id", tagHandler.GetTag)       // 获取标签详情
			tags.PUT("/:id", tagHandler.UpdateTag)    // 更新标签
			tags.DELETE("/:id", tagHandler.DeleteTag) // 删除标签

			// 标签搜索和统计
			tags.GET("/search", tagHandler.SearchTags)          // 搜索标签
			tags.GET("/popular", tagHandler.GetPopularTags)     // 获取热门标签
			tags.POST("/batch", tagHandler.BatchCreateTags)     // 批量创建标签
			tags.GET("/:id/stats", tagHandler.GetTagUsageStats) // 获取标签使用统计
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
