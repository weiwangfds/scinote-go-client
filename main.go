// @title Scinote Go Client API
// @version 1.0
// @description 文献管理系统
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8080
// @BasePath /api/v1
// @schemes http https

// @securityDefinitions.basic BasicAuth

// @externalDocs.description OpenAPI
// @externalDocs.url https://swagger.io/resources/open-api/
package main

import (
	"context"
	"crypto/tls"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"golang.org/x/net/http2"
	"github.com/weiwangfds/scinote/config"
	"github.com/weiwangfds/scinote/internal/database"
	"github.com/weiwangfds/scinote/internal/middleware"
	"github.com/weiwangfds/scinote/internal/router"
	ossservice "github.com/weiwangfds/scinote/internal/service/oss"
	watcherservice "github.com/weiwangfds/scinote/internal/service/watcher"
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
	ossConfigService := ossservice.NewOSSConfigService(db)
	fileWatcherService := watcherservice.NewFileWatcherService(db, ossConfigService)

	// 初始化路由
	r := router.NewRouter(loggerMiddleware, db, cfg)

	// 启动文件监听服务
	watcherCtx, cancelWatcher := context.WithCancel(context.Background())
	if err := fileWatcherService.Start(watcherCtx); err != nil {
		log.Printf("Failed to start file watcher service: %v", err)
	}

	// 创建HTTPS服务器（仅支持HTTPS和HTTP/2）
	var httpsSrv *http.Server
	if !cfg.Server.EnableHTTPS {
		log.Fatal("HTTPS必须启用，HTTP支持已被移除")
	}

	// 创建HTTPS服务器
	httpsSrv = &http.Server{
		Addr:         ":" + strconv.Itoa(cfg.Server.HTTPSPort),
		Handler:      r.GetEngine(),
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
		TLSConfig: &tls.Config{
			NextProtos: []string{"h2", "http/1.1"}, // 支持HTTP/2和HTTP/1.1
		},
	}

	// 如果启用HTTP/2，配置HTTP/2支持
	if cfg.Server.EnableHTTP2 {
		if err := http2.ConfigureServer(httpsSrv, &http2.Server{}); err != nil {
			log.Fatalf("配置HTTP/2失败: %v", err)
		}
	}

	// 启动HTTPS服务器
	go func() {
		log.Printf("HTTPS服务器启动在端口 %d (HTTP/2: %v) - HTTP支持已禁用", cfg.Server.HTTPSPort, cfg.Server.EnableHTTP2)
		if err := httpsSrv.ListenAndServeTLS(cfg.Server.TLSCertFile, cfg.Server.TLSKeyFile); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTPS服务器启动失败: %v", err)
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("正在关闭服务器...")

	// 停止文件监听服务
	cancelWatcher()
	if err := fileWatcherService.Stop(); err != nil {
		log.Printf("Error stopping file watcher service: %v", err)
	}

	// 优雅关闭服务器
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 关闭HTTPS服务器
	if err := httpsSrv.Shutdown(ctx); err != nil {
		log.Fatal("HTTPS服务器强制关闭:", err)
	}

	log.Println("服务器已退出")
}
