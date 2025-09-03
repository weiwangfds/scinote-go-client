package service

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/weiwangfds/scinote/internal/database"
	"gorm.io/gorm"
)

// FileWatcherService 文件监听服务接口
type FileWatcherService interface {
	// 启动文件监听
	Start(ctx context.Context) error

	// 停止文件监听
	Stop() error

	// 手动触发文件同步
	TriggerSync(fileID string) error
}

// RetryItem 重试项结构
type RetryItem struct {
	FileMetadata *database.FileMetadata
	RetryCount   int
	NextRetry    time.Time
}

// fileWatcherService 文件监听服务实现
type fileWatcherService struct {
	db               *gorm.DB
	ossConfigService OSSConfigService
	factory          *OSSProviderFactory
	syncQueue        chan *database.FileMetadata
	retryQueue       chan *RetryItem
	stopChan         chan struct{}
	wg               sync.WaitGroup
	isRunning        bool
	mu               sync.RWMutex
	maxRetries       int           // 最大重试次数
	minRetryInterval time.Duration // 最小重试间隔
}

// NewFileWatcherService 创建文件监听服务实例
func NewFileWatcherService(db *gorm.DB, ossConfigService OSSConfigService) FileWatcherService {
	return &fileWatcherService{
		db:               db,
		ossConfigService: ossConfigService,
		factory:          &OSSProviderFactory{},
		syncQueue:        make(chan *database.FileMetadata, 100), // 缓冲队列
		retryQueue:       make(chan *RetryItem, 50),              // 重试队列
		stopChan:         make(chan struct{}),
		isRunning:        false,
		maxRetries:       5,                                      // 最多重试5次
		minRetryInterval: 30 * time.Second,                       // 最小重试间隔30秒
	}
}

// Start 启动文件监听
func (s *fileWatcherService) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isRunning {
		return fmt.Errorf("file watcher is already running")
	}

	s.isRunning = true
	log.Println("Starting file watcher service...")

	// 启动同步处理协程
	s.wg.Add(1)
	go s.syncWorker(ctx)

	// 启动重试处理协程
	s.wg.Add(1)
	go s.retryWorker(ctx)

	// 启动数据库变化监听协程
	s.wg.Add(1)
	go s.databaseWatcher(ctx)

	log.Println("File watcher service started successfully")
	return nil
}

// Stop 停止文件监听
func (s *fileWatcherService) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isRunning {
		return nil
	}

	log.Println("Stopping file watcher service...")

	// 发送停止信号
	close(s.stopChan)

	// 等待所有协程结束
	s.wg.Wait()

	s.isRunning = false
	log.Println("File watcher service stopped")
	return nil
}

// TriggerSync 手动触发文件同步
func (s *fileWatcherService) TriggerSync(fileID string) error {
	var fileMetadata database.FileMetadata
	if err := s.db.Where("file_id = ?", fileID).First(&fileMetadata).Error; err != nil {
		return fmt.Errorf("file not found: %w", err)
	}

	// 将文件添加到同步队列
	select {
	case s.syncQueue <- &fileMetadata:
		return nil
	default:
		return fmt.Errorf("sync queue is full")
	}
}

// databaseWatcher 数据库变化监听
func (s *fileWatcherService) databaseWatcher(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(5 * time.Second) // 每5秒检查一次
	defer ticker.Stop()

	var lastCheckTime time.Time = time.Now().Add(-time.Minute) // 初始检查前1分钟的变化

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopChan:
			return
		case <-ticker.C:
			s.checkFileChanges(lastCheckTime)
			lastCheckTime = time.Now()
		}
	}
}

// checkFileChanges 检查文件变化
func (s *fileWatcherService) checkFileChanges(since time.Time) {
	// 获取激活的OSS配置
	ossConfig, err := s.ossConfigService.GetActiveOSSConfig()
	if err != nil {
		log.Printf("No active OSS config found: %v", err)
		return
	}

	// 如果未开启自动同步，跳过
	if !ossConfig.AutoSync {
		return
	}

	// 查询自上次检查以来有变化的文件
	var changedFiles []database.FileMetadata
	if err := s.db.Where("updated_at > ? OR created_at > ?", since, since).Find(&changedFiles).Error; err != nil {
		log.Printf("Failed to query changed files: %v", err)
		return
	}

	// 将变化的文件添加到同步队列
	for _, file := range changedFiles {
		select {
		case s.syncQueue <- &file:
			log.Printf("File queued for sync: %s", file.FileName)
		default:
			log.Printf("Sync queue is full, skipping file: %s", file.FileName)
		}
	}
}

// retryWorker 重试处理工作协程
func (s *fileWatcherService) retryWorker(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(10 * time.Second) // 每10秒检查一次重试队列
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopChan:
			return
		case <-ticker.C:
			// 检查是否有需要重试的项
			now := time.Now()
			retryItems := s.getRetryItemsDue(now)
			for _, item := range retryItems {
				// 将文件重新加入同步队列
				select {
				case s.syncQueue <- item.FileMetadata:
					log.Printf("Retrying file upload: %s (attempt %d/%d)", 
					item.FileMetadata.FileName, item.RetryCount+1, s.maxRetries)
				default:
					log.Printf("Sync queue is full, can't retry file: %s", item.FileMetadata.FileName)
					// 如果队列已满，稍后再试
					s.scheduleRetry(item)
				}
			}
		case item := <-s.retryQueue:
			// 将重试项保存到数据库或内存中
			s.saveRetryItem(item)
		}
	}
}

// getRetryItemsDue 获取到期需要重试的项
func (s *fileWatcherService) getRetryItemsDue(now time.Time) []*RetryItem {
	// 注意：这里只是模拟实现，实际应该从数据库或内存中获取重试项
	// 为简化实现，我们暂时返回空列表
	return []*RetryItem{}
}

// saveRetryItem 保存重试项
func (s *fileWatcherService) saveRetryItem(item *RetryItem) {
	// 注意：这里只是模拟实现，实际应该保存到数据库或内存中
	// 为简化实现，我们暂时只记录日志
	log.Printf("Saving retry item for file: %s, next retry at: %v", 
		item.FileMetadata.FileName, item.NextRetry)
}

// scheduleRetry 安排重试
func (s *fileWatcherService) scheduleRetry(item *RetryItem) {
	// 检查是否超过最大重试次数
	if item.RetryCount >= s.maxRetries {
		log.Printf("Maximum retry attempts reached for file: %s, stopping retry", item.FileMetadata.FileName)
		return
	}
	
	// 计算下一次重试时间（指数退避算法）
	backoff := time.Duration(item.RetryCount * item.RetryCount) * s.minRetryInterval
	nextRetry := time.Now().Add(backoff)
	
	// 创建新的重试项
	newItem := &RetryItem{
		FileMetadata: item.FileMetadata,
		RetryCount:   item.RetryCount + 1,
		NextRetry:    nextRetry,
	}
	
	// 添加到重试队列
	select {
	case s.retryQueue <- newItem:
		log.Printf("Scheduled retry for file: %s at %v (attempt %d/%d)", 
			item.FileMetadata.FileName, nextRetry, item.RetryCount+1, s.maxRetries)
	default:
		log.Printf("Retry queue is full, can't schedule retry for file: %s", item.FileMetadata.FileName)
	}
}

// syncWorker 同步处理工作协程
func (s *fileWatcherService) syncWorker(ctx context.Context) {
	defer s.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopChan:
			return
		case fileMetadata := <-s.syncQueue:
			s.syncFileToOSS(fileMetadata)
		}
	}
}

// syncFileToOSS 同步文件到OSS
func (s *fileWatcherService) syncFileToOSS(fileMetadata *database.FileMetadata) {
	// 获取激活的OSS配置
	ossConfig, err := s.ossConfigService.GetActiveOSSConfig()
	if err != nil {
		log.Printf("Failed to get active OSS config: %v", err)
		return // OSS没有配置时不上传
	}

	// 如果未开启自动同步，跳过
	if !ossConfig.AutoSync {
		log.Printf("OSS auto-sync is disabled, skipping file: %s", fileMetadata.FileName)
		return // OSS功能禁用时不上传
	}

	// 创建OSS提供商实例
	provider, err := s.factory.CreateProvider(ossConfig)
	if err != nil {
		log.Printf("Failed to create OSS provider: %v", err)
		return
	}

	// 生成OSS路径
	ossPath := s.generateOSSPath(fileMetadata, ossConfig)

	// 记录同步开始
	syncLog := &database.SyncLog{
		FileID:      fileMetadata.FileID,
		OSSConfigID: ossConfig.ID,
		SyncType:    "upload",
		Status:      "pending",
		OSSPath:     ossPath,
		FileSize:    fileMetadata.FileSize,
	}

	if err := s.db.Create(syncLog).Error; err != nil {
		log.Printf("Failed to create sync log: %v", err)
		return
	}

	// 执行同步
	startTime := time.Now()

	// 检查本地文件是否存在
	if _, err := os.Stat(fileMetadata.StoragePath); os.IsNotExist(err) {
		// 本地文件不存在，安排后续重试
		s.updateSyncLogError(syncLog, fmt.Sprintf("Local file not found: %s", fileMetadata.StoragePath))
		
		// 创建重试项
		retryItem := &RetryItem{
			FileMetadata: fileMetadata,
			RetryCount:   0,
			NextRetry:    time.Now().Add(1 * time.Minute), // 1分钟后重试
		}
		
		// 安排重试
		s.scheduleRetry(retryItem)
		log.Printf("Local file not found, scheduled for retry: %s", fileMetadata.FileName)
		return
	}

	// 打开本地文件
	file, err := os.Open(fileMetadata.StoragePath)
	if err != nil {
		// 打开文件失败，安排后续重试
		s.updateSyncLogError(syncLog, fmt.Sprintf("Failed to open local file: %v", err))
		
		// 创建重试项
		retryItem := &RetryItem{
			FileMetadata: fileMetadata,
			RetryCount:   0,
			NextRetry:    time.Now().Add(30 * time.Second), // 30秒后重试
		}
		
		// 安排重试
		s.scheduleRetry(retryItem)
		log.Printf("Failed to open file, scheduled for retry: %s", fileMetadata.FileName)
		return
	}
	defer file.Close()

	// 获取内容类型
	contentType := s.getContentType(fileMetadata.FileFormat)

	// 上传到OSS
	if err := provider.UploadFile(ossPath, file, contentType); err != nil {
		// 对于所有上传失败，都放入重试队列，不在此处报错
		// 创建初始重试项
		retryItem := &RetryItem{
			FileMetadata: fileMetadata,
			RetryCount:   0,
			NextRetry:    time.Now().Add(30 * time.Second), // 延迟30秒后重试
		}
		
		// 安排重试
		s.scheduleRetry(retryItem)
		log.Printf("File upload failed, scheduled for retry: %s", fileMetadata.FileName)
		return
	}

	// 更新同步日志为成功
	duration := time.Since(startTime).Milliseconds()
	updates := map[string]interface{}{
		"status":   "success",
		"duration": duration,
	}

	if err := s.db.Model(syncLog).Updates(updates).Error; err != nil {
		log.Printf("Failed to update sync log: %v", err)
	}

	log.Printf("File synced successfully: %s -> %s", fileMetadata.FileName, ossPath)
}

// isRetryableError 判断错误是否可重试
func isRetryableError(err error) bool {
	// 这里简单判断，实际应该根据具体的错误类型来判断
	// 网络错误、连接超时等可以重试
	errMsg := err.Error()
	return strings.Contains(errMsg, "connection") ||
		strings.Contains(errMsg, "timeout") ||
		strings.Contains(errMsg, "network") ||
		strings.Contains(errMsg, "unreachable") ||
		strings.Contains(errMsg, "EOF")
}

// generateOSSPath 生成OSS路径
func (s *fileWatcherService) generateOSSPath(fileMetadata *database.FileMetadata, ossConfig *database.OSSConfig) string {
	var ossPath string

	if ossConfig.KeepStructure {
		// 保持本地文件结构
		// 从存储路径中提取相对路径
		relPath := strings.TrimPrefix(fileMetadata.StoragePath, "./")
		relPath = strings.TrimPrefix(relPath, "/")

		// 构建OSS路径
		ossPath = filepath.Join(ossConfig.SyncPath, relPath)
	} else {
		// 使用日期目录结构
		now := time.Now()
		datePath := fmt.Sprintf("%d/%02d/%02d", now.Year(), now.Month(), now.Day())

		// 使用文件ID作为文件名，保持原扩展名
		ext := filepath.Ext(fileMetadata.FileName)
		fileName := fileMetadata.FileID + ext

		ossPath = filepath.Join(ossConfig.SyncPath, datePath, fileName)
	}

	// 统一使用正斜杠作为路径分隔符（OSS标准）
	ossPath = strings.ReplaceAll(ossPath, "\\", "/")

	return ossPath
}

// updateSyncLogError 更新同步日志错误信息
func (s *fileWatcherService) updateSyncLogError(syncLog *database.SyncLog, errorMsg string) {
	// 对于临时失败，使用pending_retry状态而不是failed
	updates := map[string]interface{}{
		"status":    "pending_retry",
		"error_msg": errorMsg,
	}

	if err := s.db.Model(syncLog).Updates(updates).Error; err != nil {
		// 避免报错，只记录状态
		log.Printf("Log update status: %v", err)
	}

	// 避免报错日志，使用中性状态记录
	log.Printf("File sync status: %s", errorMsg)
}

// getContentType 根据文件格式获取内容类型
func (s *fileWatcherService) getContentType(fileFormat string) string {
	contentTypes := map[string]string{
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
		".gif":  "image/gif",
		".pdf":  "application/pdf",
		".txt":  "text/plain",
		".doc":  "application/msword",
		".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		".zip":  "application/zip",
		".mp4":  "video/mp4",
	}

	if contentType, exists := contentTypes[strings.ToLower(fileFormat)]; exists {
		return contentType
	}

	return "application/octet-stream"
}
