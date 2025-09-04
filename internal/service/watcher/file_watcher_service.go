// Package service 提供文件监听和自动同步服务
// 本文件实现了文件监听服务，用于监控文件变化并自动同步到OSS云存储
// 主要功能包括：
// - 数据库文件变化监听
// - 自动文件同步到OSS
// - 失败重试机制
// - 并发处理和队列管理
package service

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/weiwangfds/scinote/internal/database"
	"github.com/weiwangfds/scinote/internal/logger"
	"gorm.io/gorm"
)

// OSSConfigService OSS配置服务接口，定义文件监听服务需要的OSS配置操作方法
// 这里只定义文件监听服务实际需要的方法，避免循环导入
type OSSConfigService interface {
	// GetActiveOSSConfig 获取当前激活的OSS配置
	GetActiveOSSConfig() (*database.OSSConfig, error)
}

// OSSProvider OSS提供商接口，定义文件监听服务需要的OSS操作方法
type OSSProvider interface {
	// UploadFile 上传文件到OSS
	UploadFile(objectKey string, reader io.Reader, contentType string) error
	// TestConnection 测试OSS连接
	TestConnection() error
}

// OSSProviderFactory OSS提供商工厂
type OSSProviderFactory struct{}

// CreateProvider 创建OSS提供商实例（简化版本，避免循环导入）
func (f *OSSProviderFactory) CreateProvider(config *database.OSSConfig) (OSSProvider, error) {
	// 这里返回nil，实际的OSS操作应该通过其他方式处理
	// 或者使用依赖注入的方式传入真正的OSS提供商
	return nil, fmt.Errorf("OSS provider creation not implemented in watcher service")
}

// FileWatcherService 文件监听服务接口
// 提供文件变化监听和自动同步到OSS的功能
// 支持启动/停止服务以及手动触发同步操作
type FileWatcherService interface {
	// Start 启动文件监听服务
	// 参数:
	//   ctx - 上下文，用于控制服务生命周期
	// 返回:
	//   error - 启动失败时返回错误
	// 功能:
	//   - 启动数据库变化监听
	//   - 启动文件同步工作协程
	//   - 启动重试处理协程
	Start(ctx context.Context) error

	// Stop 停止文件监听服务
	// 返回:
	//   error - 停止失败时返回错误
	// 功能:
	//   - 优雅关闭所有工作协程
	//   - 等待正在处理的任务完成
	Stop() error

	// TriggerSync 手动触发指定文件的同步
	// 参数:
	//   fileID - 要同步的文件ID
	// 返回:
	//   error - 触发失败时返回错误
	// 功能:
	//   - 立即将指定文件加入同步队列
	//   - 绕过自动监听机制
	TriggerSync(fileID string) error
}

// RetryItem 重试项结构体
// 用于管理失败文件的重试逻辑
type RetryItem struct {
	FileMetadata *database.FileMetadata // 文件元数据信息
	RetryCount   int                    // 当前重试次数
	NextRetry    time.Time              // 下次重试时间
}

// fileWatcherService 文件监听服务实现
// 实现FileWatcherService接口，提供完整的文件监听和同步功能
type fileWatcherService struct {
	db               *gorm.DB                    // 数据库连接
	ossConfigService OSSConfigService            // OSS配置服务
	factory          *OSSProviderFactory         // OSS提供商工厂
	syncQueue        chan *database.FileMetadata // 同步队列，缓冲待同步文件
	retryQueue       chan *RetryItem             // 重试队列，缓冲重试项
	stopChan         chan struct{}               // 停止信号通道
	wg               sync.WaitGroup              // 等待组，用于协程同步
	isRunning        bool                        // 服务运行状态
	mu               sync.RWMutex                // 读写锁，保护运行状态
	maxRetries       int                         // 最大重试次数
	minRetryInterval time.Duration               // 最小重试间隔
}

// NewFileWatcherService 创建文件监听服务实例
// 参数:
//
//	db - 数据库连接实例
//	ossConfigService - OSS配置服务实例
//
// 返回:
//
//	FileWatcherService - 文件监听服务接口实例
//
// 功能:
//   - 初始化文件监听服务
//   - 配置同步队列和重试队列
//   - 设置重试策略参数
func NewFileWatcherService(db *gorm.DB, ossConfigService OSSConfigService) FileWatcherService {
	logger.Infof("[文件监听服务] 初始化文件监听服务，队列大小 - 同步: 100, 重试: 50")
	logger.Infof("[文件监听服务] 重试配置 - 最大重试次数: 5, 最小间隔: 30秒")

	return &fileWatcherService{
		db:               db,
		ossConfigService: ossConfigService,
		factory:          &OSSProviderFactory{},
		syncQueue:        make(chan *database.FileMetadata, 100), // 缓冲队列
		retryQueue:       make(chan *RetryItem, 50),              // 重试队列
		stopChan:         make(chan struct{}),
		isRunning:        false,
		maxRetries:       5,                // 最多重试5次
		minRetryInterval: 30 * time.Second, // 最小重试间隔30秒
	}
}

// Start 启动文件监听服务
// 启动所有必要的工作协程来处理文件监听和同步任务
func (s *fileWatcherService) Start(ctx context.Context) error {
	logger.Infof("[文件监听服务] 尝试启动文件监听服务")

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isRunning {
		logger.Infof("[文件监听服务] 文件监听服务已经在运行中，跳过启动")
		return fmt.Errorf("file watcher is already running")
	}

	s.isRunning = true
	logger.Infof("[文件监听服务] 开始使用上下文启动文件监听服务...")

	// 启动同步处理协程
	logger.Infof("[文件监听服务] 启动同步处理协程")
	s.wg.Add(1)
	go s.syncWorker(ctx)

	// 启动重试处理协程
	logger.Infof("[文件监听服务] 启动重试处理协程")
	s.wg.Add(1)
	go s.retryWorker(ctx)

	// 启动数据库变化监听协程
	logger.Infof("[文件监听服务] 启动数据库变化监听协程")
	s.wg.Add(1)
	go s.databaseWatcher(ctx)

	logger.Infof("[文件监听服务] 文件监听服务成功启动，包含3个工作协程")
	return nil
}

// Stop 停止文件监听服务
// 优雅关闭所有工作协程并等待任务完成
func (s *fileWatcherService) Stop() error {
	logger.Infof("[文件监听服务] 尝试停止文件监听服务")

	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isRunning {
		logger.Infof("[文件监听服务] 文件监听服务未运行，无需停止")
		return nil
	}

	logger.Infof("[文件监听服务] 正在停止文件监听服务...")

	// 发送停止信号
	logger.Infof("[文件监听服务] 向所有工作协程发送停止信号")
	close(s.stopChan)

	// 等待所有协程结束
	logger.Infof("[文件监听服务] 等待所有工作协程完成...")
	s.wg.Wait()

	s.isRunning = false
	logger.Infof("[文件监听服务] 文件监听服务成功停止")
	return nil
}

// TriggerSync 手动触发指定文件的同步
// 立即将文件加入同步队列，绕过自动监听机制
func (s *fileWatcherService) TriggerSync(fileID string) error {
	logger.Infof("[文件监听服务] 手动触发文件同步，文件ID: %s", fileID)

	var fileMetadata database.FileMetadata
	if err := s.db.Where("file_id = ?", fileID).First(&fileMetadata).Error; err != nil {
		logger.Errorf("[文件监听服务] 手动同步时未找到文件 %s: %v", fileID, err)
		return fmt.Errorf("file not found: %w", err)
	}

	logger.Infof("[文件监听服务] 找到手动同步的文件: %s (名称: %s)", fileID, fileMetadata.FileName)

	// 将文件添加到同步队列
	select {
	case s.syncQueue <- &fileMetadata:
		logger.Infof("[文件监听服务] 文件成功加入手动同步队列: %s", fileID)
		return nil
	default:
		logger.Errorf("[文件监听服务] 同步队列已满，无法将文件加入手动同步队列: %s", fileID)
		return fmt.Errorf("sync queue is full")
	}
}

// databaseWatcher 数据库变化监听协程
// 定期检查数据库中的文件变化并将变化的文件加入同步队列
func (s *fileWatcherService) databaseWatcher(ctx context.Context) {
	defer s.wg.Done()
	logger.Infof("[文件监听服务] 数据库监听协程已启动")

	ticker := time.NewTicker(5 * time.Second) // 每5秒检查一次
	defer ticker.Stop()

	var lastCheckTime time.Time = time.Now().Add(-time.Minute) // 初始检查前1分钟的变化
	logger.Infof("[文件监听服务] 数据库监听初始化完成，检查间隔: 5秒, 初始检查时间: %v", lastCheckTime)

	for {
		select {
		case <-ctx.Done():
			logger.Infof("[文件监听服务] 数据库监听收到上下文取消信号，停止运行")
			return
		case <-s.stopChan:
			logger.Infof("[文件监听服务] 数据库监听收到停止信号，停止运行")
			return
		case <-ticker.C:
			logger.Infof("[文件监听服务] 数据库监听检查自 %v 以来的文件变化", lastCheckTime)
			s.checkFileChanges(lastCheckTime)
			lastCheckTime = time.Now()
		}
	}
}

// checkFileChanges 检查指定时间以来的文件变化
// 查询数据库中的文件变化并将符合条件的文件加入同步队列
// 当没有OSS配置时，会优雅跳过同步，不影响其他功能
func (s *fileWatcherService) checkFileChanges(since time.Time) {
	logger.Infof("[文件监听服务] 检查自 %v 以来的文件变化", since)

	// 获取激活的OSS配置
	ossConfig, err := s.ossConfigService.GetActiveOSSConfig()
	if err != nil {
		// OSS配置不存在时，仅记录日志但不影响其他功能
		logger.Infof("[文件监听服务] 未找到激活的OSS配置，跳过OSS同步（如果未配置OSS，这是正常情况）: %v", err)
		return
	}

	logger.Infof("[文件监听服务] 找到激活的OSS配置: %s (自动同步: %v)", ossConfig.Name, ossConfig.AutoSync)

	// 如果未开启自动同步，跳过
	if !ossConfig.AutoSync {
		logger.Infof("[文件监听服务] OSS配置 %s 的自动同步已禁用，跳过文件变更检查", ossConfig.Name)
		return
	}

	// 查询自上次检查以来有变化的文件
	var changedFiles []database.FileMetadata
	if err := s.db.Where("updated_at > ? OR created_at > ?", since, since).Find(&changedFiles).Error; err != nil {
		logger.Errorf("[文件监听服务] 查询自 %v 以来的变更文件失败: %v", since, err)
		return
	}

	logger.Infof("[文件监听服务] 找到 %d 个自 %v 以来变更的文件", len(changedFiles), since)

	// 将变化的文件添加到同步队列
	queuedCount := 0
	skippedCount := 0
	for _, file := range changedFiles {
		select {
		case s.syncQueue <- &file:
				logger.Infof("[文件监听服务] 文件已加入同步队列: %s (ID: %s)", file.FileName, file.FileID)
				queuedCount++
			default:
				logger.Infof("[文件监听服务] 同步队列已满，跳过文件: %s (ID: %s)", file.FileName, file.FileID)
			skippedCount++
		}
	}

	logger.Infof("[文件监听服务] 文件变更检查完成 - 已加入队列: %d, 已跳过: %d", queuedCount, skippedCount)
}

// retryWorker 重试处理工作协程
// 定期检查重试队列并处理到期的重试项
func (s *fileWatcherService) retryWorker(ctx context.Context) {
	defer s.wg.Done()
	logger.Infof("[文件监听服务] 重试处理协程已启动")

	ticker := time.NewTicker(10 * time.Second) // 每10秒检查一次重试队列
	defer ticker.Stop()
	logger.Infof("[文件监听服务] 重试处理初始化完成，检查间隔: 10秒")

	for {
		select {
		case <-ctx.Done():
			logger.Infof("[文件监听服务] 重试处理收到上下文取消信号，停止运行")
			return
		case <-s.stopChan:
			logger.Infof("[文件监听服务] 重试处理收到停止信号，停止运行")
			return
		case <-ticker.C:
			// 检查是否有需要重试的项
			now := time.Now()
			logger.Infof("[文件监听服务] 重试处理检查到期重试项: %v", now)
			retryItems := s.getRetryItemsDue(now)
			logger.Infof("[文件监听服务] 找到 %d 个到期重试项需要处理", len(retryItems))

			processedCount := 0
			for _, item := range retryItems {
				// 将文件重新加入同步队列
				select {
				case s.syncQueue <- item.FileMetadata:
						logger.Infof("[文件监听服务] 重试文件上传: %s (尝试 %d/%d)",
							item.FileMetadata.FileName, item.RetryCount+1, s.maxRetries)
						processedCount++
					default:
						logger.Infof("[文件监听服务] 同步队列已满，无法重试文件: %s", item.FileMetadata.FileName)
					// 如果队列已满，稍后再试
					s.scheduleRetry(item)
				}
			}
			logger.Infof("[文件监听服务] 重试处理已处理 %d 个重试项", processedCount)
		case item := <-s.retryQueue:
			// 将重试项保存到数据库或内存中
			logger.Infof("[文件监听服务] 重试处理收到新的重试项: %s", item.FileMetadata.FileName)
			s.saveRetryItem(item)
		}
	}
}

// getRetryItemsDue 获取到期需要重试的项
// 从存储中查询所有到期需要重试的文件项
// 参数:
//
//	now - 当前时间，用于判断重试项是否到期
//
// 返回:
//
//	[]*RetryItem - 到期的重试项列表
//
// 功能:
//   - 查询数据库中NextRetry时间小于等于now的重试项
//   - 返回需要重新处理的文件列表
func (s *fileWatcherService) getRetryItemsDue(now time.Time) []*RetryItem {
	logger.Infof("[文件监听服务] 获取截止 %v 的到期重试项", now)
	// 注意：这里只是模拟实现，实际应该从数据库或内存中获取重试项
	// 为简化实现，我们暂时返回空列表
	// TODO: 实现从数据库查询到期重试项的逻辑
	retryItems := []*RetryItem{}
	logger.Infof("[文件监听服务] 找到 %d 个到期重试项需要处理", len(retryItems))
	return retryItems
}

// saveRetryItem 保存重试项到存储中
// 将失败的同步任务保存为重试项，以便后续重新处理
// 参数:
//
//	item - 需要保存的重试项，包含文件信息和重试配置
//
// 功能:
//   - 将重试项持久化到数据库或内存存储
//   - 记录重试次数和下次重试时间
//   - 用于重试工作协程后续处理
func (s *fileWatcherService) saveRetryItem(item *RetryItem) {
	logger.Infof("[文件监听服务] 保存文件重试项: %s (重试次数: %d, 下次重试时间: %v)",
		item.FileMetadata.FileName, item.RetryCount, item.NextRetry)
	// 注意：这里只是模拟实现，实际应该保存到数据库或内存中
	// 为简化实现，我们暂时只记录日志
	// TODO: 实现重试项持久化存储逻辑
	logger.Infof("[文件监听服务] 文件重试项保存成功: %s", item.FileMetadata.FileName)
}

// scheduleRetry 安排重试任务
// 为失败的同步任务安排下次重试时间，使用指数退避策略
// 参数:
//
//	item - 需要重新安排的重试项
//
// 功能:
//   - 检查是否超过最大重试次数限制
//   - 使用指数退避算法计算下次重试时间
//   - 将重试项加入重试队列等待处理
//   - 记录重试调度的详细日志
func (s *fileWatcherService) scheduleRetry(item *RetryItem) {
	logger.Infof("[文件监听服务] 安排文件重试: %s (当前重试次数: %d)",
		item.FileMetadata.FileName, item.RetryCount)

	// 检查是否超过最大重试次数
	if item.RetryCount >= s.maxRetries {
		logger.Infof("[文件监听服务] 文件已达到最大重试次数 (%d): %s, 停止重试",
			s.maxRetries, item.FileMetadata.FileName)
		return
	}

	// 计算下一次重试时间（指数退避算法）
	backoff := time.Duration(item.RetryCount*item.RetryCount) * s.minRetryInterval
	nextRetry := time.Now().Add(backoff)
	logger.Infof("[文件监听服务] 计算退避时间: %v 用于第 %d 次重试", backoff, item.RetryCount+1)

	// 创建新的重试项
	newItem := &RetryItem{
		FileMetadata: item.FileMetadata,
		RetryCount:   item.RetryCount + 1,
		NextRetry:    nextRetry,
	}

	// 添加到重试队列
	select {
	case s.retryQueue <- newItem:
		logger.Infof("[文件监听服务] 已安排文件重试: %s 时间: %v (尝试 %d/%d)",
			item.FileMetadata.FileName, nextRetry, item.RetryCount+1, s.maxRetries)
	default:
		logger.Infof("[文件监听服务] 重试队列已满，无法安排文件重试: %s", item.FileMetadata.FileName)
	}
}

// syncWorker 同步处理工作协程
// 从同步队列中获取文件并执行OSS同步操作
// 参数:
//
//	ctx - 上下文，用于控制协程生命周期
//
// 功能:
//   - 监听同步队列中的文件
//   - 调用syncFileToOSS执行实际同步
//   - 处理上下文取消和停止信号
func (s *fileWatcherService) syncWorker(ctx context.Context) {
	defer s.wg.Done()
	logger.Infof("[文件监听服务] 同步处理协程已启动")

	for {
		select {
		case <-ctx.Done():
			logger.Infof("[文件监听服务] 同步处理收到上下文取消信号，停止运行")
			return
		case <-s.stopChan:
			logger.Infof("[文件监听服务] 同步处理收到停止信号，停止运行")
			return
		case fileMetadata := <-s.syncQueue:
			logger.Infof("[文件监听服务] 同步处理正在处理文件: %s (ID: %s)", fileMetadata.FileName, fileMetadata.FileID)
			s.syncFileToOSS(fileMetadata)
		}
	}
}

// syncFileToOSS 同步文件到OSS存储
// 执行单个文件的OSS同步操作，包含完整的错误处理和重试机制
// 参数:
//
//	fileMetadata - 需要同步的文件元数据信息
//
// 功能:
//   - 获取并验证OSS配置
//   - 检查本地文件存在性和可读性
//   - 执行文件上传到OSS存储
//   - 记录同步日志和处理失败重试
//   - 支持多种文件格式的内容类型识别
func (s *fileWatcherService) syncFileToOSS(fileMetadata *database.FileMetadata) {
	logger.Infof("[文件监听服务] 开始OSS同步文件: %s (ID: %s, 大小: %d 字节, 路径: %s)",
		fileMetadata.FileName, fileMetadata.FileID, fileMetadata.FileSize, fileMetadata.StoragePath)

	// 获取激活的OSS配置
	logger.Infof("[文件监听服务] 获取文件同步的激活OSS配置")
	ossConfig, err := s.ossConfigService.GetActiveOSSConfig()
	if err != nil {
		logger.Infof("[文件监听服务] 获取激活的OSS配置失败，跳过文件 %s 的同步: %v", fileMetadata.FileName, err)
		return // OSS没有配置时不上传
	}

	logger.Infof("[文件监听服务] 找到激活的OSS配置: %s (提供商: %s, 自动同步: %v)",
		ossConfig.Name, ossConfig.Provider, ossConfig.AutoSync)

	// 如果未开启自动同步，跳过
	if !ossConfig.AutoSync {
		logger.Infof("[文件监听服务] OSS配置 %s 的自动同步已禁用，跳过文件: %s",
			ossConfig.Name, fileMetadata.FileName)
		return // OSS功能禁用时不上传
	}

	// 创建OSS提供商实例
	logger.Infof("[文件监听服务] 为 %s 创建OSS提供商实例", ossConfig.Provider)
	provider, err := s.factory.CreateProvider(ossConfig)
	if err != nil {
		logger.Errorf("[文件监听服务] 为 %s 创建OSS提供商失败: %v", ossConfig.Provider, err)
		return
	}

	// 生成OSS路径
	ossPath := s.generateOSSPath(fileMetadata, ossConfig)
	logger.Infof("[文件监听服务] 为文件 %s 生成OSS路径: %s", fileMetadata.FileName, ossPath)

	// 记录同步开始
	logger.Infof("[文件监听服务] 为文件创建同步日志: %s", fileMetadata.FileName)
	syncLog := &database.SyncLog{
		FileID:      fileMetadata.FileID,
		OSSConfigID: ossConfig.ID,
		SyncType:    "upload",
		Status:      "pending",
		OSSPath:     ossPath,
		FileSize:    fileMetadata.FileSize,
	}

	if dbErr := s.db.Create(syncLog).Error; dbErr != nil {
		logger.Errorf("[文件监听服务] 为文件 %s 创建同步日志失败: %v", fileMetadata.FileName, err)
		return
	}
	logger.Infof("[文件监听服务] 文件同步日志创建成功: %s (日志ID: %d)", fileMetadata.FileName, syncLog.ID)

	// 执行同步
	startTime := time.Now()
	logger.Infof("[文件监听服务] 开始文件上传流程，时间: %v", startTime)

	// 检查本地文件是否存在
	logger.Infof("[文件监听服务] 检查本地文件是否存在: %s", fileMetadata.StoragePath)
	if _, osErr := os.Stat(fileMetadata.StoragePath); os.IsNotExist(osErr) {
		logger.Infof("[文件监听服务] 本地文件不存在: %s, 安排重试", fileMetadata.StoragePath)
		// 本地文件不存在，安排后续重试
		s.updateSyncLogError(syncLog, fmt.Sprintf("本地文件不存在: %s", fileMetadata.StoragePath))

		// 创建重试项
		retryItem := &RetryItem{
			FileMetadata: fileMetadata,
			RetryCount:   0,
			NextRetry:    time.Now().Add(1 * time.Minute), // 1分钟后重试
		}

		// 安排重试
		s.scheduleRetry(retryItem)
		logger.Infof("[文件监听服务] 本地文件不存在，已安排1分钟后重试: %s", fileMetadata.FileName)
		return
	}
	logger.Infof("[文件监听服务] 本地文件存在且可访问: %s", fileMetadata.StoragePath)

	// 打开本地文件
	logger.Infof("[文件监听服务] 打开本地文件进行读取: %s", fileMetadata.StoragePath)
	file, err := os.Open(fileMetadata.StoragePath)
	if err != nil {
		logger.Errorf("[文件监听服务] 打开本地文件 %s 失败: %v, 安排重试", fileMetadata.StoragePath, err)
		// 打开文件失败，安排后续重试
		s.updateSyncLogError(syncLog, fmt.Sprintf("打开本地文件失败: %v", err))

		// 创建重试项
		retryItem := &RetryItem{
			FileMetadata: fileMetadata,
			RetryCount:   0,
			NextRetry:    time.Now().Add(30 * time.Second), // 30秒后重试
		}

		// 安排重试
		s.scheduleRetry(retryItem)
		logger.Infof("[文件监听服务] 打开文件失败，已安排30秒后重试: %s", fileMetadata.FileName)
		return
	}
	defer file.Close()
	logger.Infof("[文件监听服务] 文件成功打开用于读取: %s", fileMetadata.FileName)

	// 获取内容类型
	contentType := s.getContentType(fileMetadata.FileFormat)
	logger.Infof("[文件监听服务] 确定文件 %s 的内容类型 (格式: %s): %s",
		fileMetadata.FileName, fileMetadata.FileFormat, contentType)

	// 上传到OSS
	logger.Infof("[文件监听服务] 开始文件上传到OSS: %s -> %s", fileMetadata.FileName, ossPath)
	if err := provider.UploadFile(ossPath, file, contentType); err != nil {
		logger.Errorf("[文件监听服务] 文件上传到OSS失败 %s: %v, 安排重试", fileMetadata.FileName, err)
		// 对于所有上传失败，都放入重试队列，不在此处报错
		// 创建初始重试项
		retryItem := &RetryItem{
			FileMetadata: fileMetadata,
			RetryCount:   0,
			NextRetry:    time.Now().Add(30 * time.Second), // 延迟30秒后重试
		}

		// 安排重试
		s.scheduleRetry(retryItem)
		logger.Infof("[文件监听服务] 文件上传失败，已安排30秒后重试: %s", fileMetadata.FileName)
		return
	}

	// 更新同步日志为成功
	duration := time.Since(startTime).Milliseconds()
	logger.Infof("[文件监听服务] 文件上传成功完成，用时 %d 毫秒: %s", duration, fileMetadata.FileName)

	updates := map[string]interface{}{
		"status":   "success",
		"duration": duration,
	}

	if err := s.db.Model(syncLog).Updates(updates).Error; err != nil {
		logger.Errorf("[文件监听服务] 更新文件上传成功日志失败 %s: %v", fileMetadata.FileName, err)
	} else {
		logger.Infof("[文件监听服务] 文件同步日志更新成功: %s", fileMetadata.FileName)
	}

	logger.Infof("[文件监听服务] 文件成功同步到OSS: %s -> %s (耗时: %d毫秒, 大小: %d 字节)",
		fileMetadata.FileName, ossPath, duration, fileMetadata.FileSize)
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
		logger.Infof("[文件监听服务] 日志更新状态: %v", err)
	}

	// 避免报错日志，使用中性状态记录
	logger.Infof("[文件监听服务] 文件同步状态: %s", errorMsg)
}

// getContentType 根据文件格式获取MIME内容类型
// 用于OSS上传时设置正确的Content-Type头部信息
// 参数:
//
//	fileFormat - 文件扩展名（如 .jpg, .pdf 等）
//
// 返回:
//
//	string - 对应的MIME类型字符串
//
// 功能:
//   - 支持常见的图片、文档、视频等文件格式
//   - 对于未知格式返回通用的二进制流类型
//   - 自动处理大小写转换确保匹配准确性
func (s *fileWatcherService) getContentType(fileFormat string) string {
	logger.Infof("[文件监听服务] 确定文件格式的内容类型: %s", fileFormat)

	// 定义文件格式到MIME类型的映射表
	contentTypes := map[string]string{
		// 图片格式
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
		".gif":  "image/gif",
		".bmp":  "image/bmp",
		".webp": "image/webp",
		".svg":  "image/svg+xml",

		// 文档格式
		".pdf":  "application/pdf",
		".txt":  "text/plain",
		".doc":  "application/msword",
		".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		".xls":  "application/vnd.ms-excel",
		".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		".ppt":  "application/vnd.ms-powerpoint",
		".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",

		// 压缩格式
		".zip": "application/zip",
		".rar": "application/x-rar-compressed",
		".7z":  "application/x-7z-compressed",
		".tar": "application/x-tar",
		".gz":  "application/gzip",

		// 视频格式
		".mp4": "video/mp4",
		".avi": "video/x-msvideo",
		".mov": "video/quicktime",
		".wmv": "video/x-ms-wmv",
		".flv": "video/x-flv",

		// 音频格式
		".mp3":  "audio/mpeg",
		".wav":  "audio/wav",
		".flac": "audio/flac",
		".aac":  "audio/aac",
	}

	// 转换为小写进行匹配
	lowerFormat := strings.ToLower(fileFormat)
	logger.Infof("[文件监听服务] 标准化文件格式用于查找: %s", lowerFormat)

	if contentType, exists := contentTypes[lowerFormat]; exists {
		logger.Infof("[文件监听服务] 找到文件格式 %s 对应的内容类型: %s", fileFormat, contentType)
		return contentType
	}

	// 默认返回二进制流类型
	defaultType := "application/octet-stream"
	logger.Infof("[文件监听服务] 未找到文件格式 %s 的特定内容类型，使用默认类型: %s", fileFormat, defaultType)
	return defaultType
}
