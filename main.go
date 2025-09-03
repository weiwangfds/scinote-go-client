package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/weiwangfds/scinote/config"
	"github.com/weiwangfds/scinote/internal/database"
	"github.com/weiwangfds/scinote/internal/middleware"
	"github.com/weiwangfds/scinote/internal/router"
	"github.com/weiwangfds/scinote/internal/service"
)

func main() {
	// 加载配置
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 初始化数据库
	db, err := database.Init(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// 初始化中间件
	loggerMiddleware := middleware.NewLoggerMiddleware()

	// 初始化文件监听服务
	ossConfigService := service.NewOSSConfigService(db)
	fileWatcherService := service.NewFileWatcherService(db, ossConfigService)

	// 初始化路由
	r := router.NewRouter(loggerMiddleware, db, cfg)

	// 启动文件监听服务
	watcherCtx, cancelWatcher := context.WithCancel(context.Background())
	if err := fileWatcherService.Start(watcherCtx); err != nil {
		log.Printf("Failed to start file watcher service: %v", err)
	}

	// 创建HTTP服务器
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      r.GetEngine(),
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
	}

	// 启动服务器
	go func() {
		log.Printf("Server starting on port %d", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// 停止文件监听服务
	cancelWatcher()
	if err := fileWatcherService.Stop(); err != nil {
		log.Printf("Error stopping file watcher service: %v", err)
	}

	// 优雅关闭服务器
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	log.Println("Server exited")
}
