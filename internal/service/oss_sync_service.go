// Package service 提供OSS同步相关的业务逻辑实现
// 包含文件上传、下载、批量同步、状态管理等功能
// 支持阿里云OSS、七牛云Kodo、腾讯云COS等多种云存储服务
package service

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/weiwangfds/scinote/internal/database"
	"gorm.io/gorm"
)

// 预定义的错误类型
var (
	// ErrUnsupportedProvider 不支持的OSS提供商错误
	ErrUnsupportedProvider = errors.New("unsupported OSS provider")
	// ErrNoActiveConfig 没有激活的OSS配置错误
	ErrNoActiveConfig = errors.New("no active OSS configuration found")
	// ErrSyncInProgress 同步操作正在进行中错误
	ErrSyncInProgress = errors.New("sync operation already in progress")
)

// OSSyncService OSS同步服务接口
// 定义了文件与云存储之间同步操作的所有方法
type OSSyncService interface {
	// SyncToOSS 同步单个文件到OSS
	// 参数:
	//   fileID: 要同步的文件ID
	// 返回:
	//   error: 同步过程中的错误信息
	SyncToOSS(fileID string) error

	// SyncFromOSS 从OSS同步文件到本地
	// 参数:
	//   fileID: 本地文件ID
	//   ossPath: OSS中的文件路径
	// 返回:
	//   error: 同步过程中的错误信息
	SyncFromOSS(fileID string, ossPath string) error

	// BatchSyncToOSS 批量同步文件到OSS
	// 参数:
	//   fileIDs: 要同步的文件ID列表
	// 返回:
	//   error: 批量同步过程中的错误信息
	BatchSyncToOSS(fileIDs []string) error

	// SyncAllFromOSS 从OSS同步所有文件到本地
	// 返回:
	//   error: 同步过程中的错误信息
	SyncAllFromOSS() error

	// ScanAndCompareFiles 扫描文件表并与云端对比
	// 返回:
	//   []string: 需要更新的文件列表
	//   []string: 仅存在于云端的文件列表
	//   error: 扫描过程中的错误信息
	ScanAndCompareFiles() ([]string, []string, error)

	// GetSyncLogs 获取同步日志
	// 参数:
	//   page: 页码
	//   pageSize: 每页大小
	// 返回:
	//   []database.SyncLog: 同步日志列表
	//   int64: 总记录数
	//   error: 查询过程中的错误信息
	GetSyncLogs(page, pageSize int) ([]database.SyncLog, int64, error)

	// GetFileSyncStatus 获取文件的同步状态
	// 参数:
	//   fileID: 文件ID
	// 返回:
	//   *database.SyncLog: 最新的同步日志
	//   error: 查询过程中的错误信息
	GetFileSyncStatus(fileID string) (*database.SyncLog, error)

	// RetryFailedSync 重试失败的同步任务
	// 参数:
	//   logID: 同步日志ID
	// 返回:
	//   error: 重试过程中的错误信息
	RetryFailedSync(logID uint) error
}

// ossSyncService OSS同步服务实现
// 实现了OSSyncService接口的所有方法
type ossSyncService struct {
	// db 数据库连接实例
	db *gorm.DB
	// fileService 文件服务实例，用于本地文件操作
	fileService FileService
	// factory OSS提供商工厂，用于创建不同的OSS客户端
	factory *OSSProviderFactory
}

// NewOSSyncService 创建OSS同步服务实例
// 参数:
//   db: 数据库连接实例
//   fileService: 文件服务实例
// 返回:
//   OSSyncService: OSS同步服务接口实例
func NewOSSyncService(db *gorm.DB, fileService FileService) OSSyncService {
	log.Println("[OSS同步服务] 正在创建OSS同步服务实例")
	
	service := &ossSyncService{
		db:          db,
		fileService: fileService,
		factory:     &OSSProviderFactory{},
	}
	
	log.Println("[OSS同步服务] OSS同步服务实例创建成功")
	return service
}

// SyncToOSS 同步单个文件到OSS
// 功能: 将本地文件上传到云存储服务
// 参数:
//   fileID: 要同步的文件ID
// 返回:
//   error: 同步过程中的错误信息
func (s *ossSyncService) SyncToOSS(fileID string) error {
	log.Printf("[OSS同步服务] 开始同步文件到OSS, 文件ID: %s", fileID)
	
	// 获取激活的OSS配置
	log.Println("[OSS同步服务] 正在获取激活的OSS配置")
	ossConfig, err := s.getActiveOSSConfig()
	if err != nil {
		log.Printf("[OSS同步服务] 获取OSS配置失败: %v", err)
		return err
	}
	log.Printf("[OSS同步服务] 成功获取OSS配置, 提供商: %s", ossConfig.Provider)

	// 获取文件信息
	log.Printf("[OSS同步服务] 正在获取文件元数据, 文件ID: %s", fileID)
	fileMetadata, err := s.fileService.GetFileByID(fileID)
	if err != nil {
		log.Printf("[OSS同步服务] 获取文件元数据失败: %v", err)
		return fmt.Errorf("failed to get file metadata: %w", err)
	}
	log.Printf("[OSS同步服务] 成功获取文件元数据, 文件名: %s, 大小: %d bytes", fileMetadata.FileName, fileMetadata.FileSize)

	// 检查是否已经在同步中
	log.Println("[OSS同步服务] 检查是否存在进行中的同步任务")
	var existingLog database.SyncLog
	if err := s.db.Where("file_id = ? AND oss_config_id = ? AND sync_type = ? AND status = ?",
		fileID, ossConfig.ID, "upload", "pending").First(&existingLog).Error; err == nil {
		log.Printf("[OSS同步服务] 文件正在同步中, 同步日志ID: %d", existingLog.ID)
		return ErrSyncInProgress
	}

	// 创建同步日志
	ossPath := s.generateOSSPath(fileMetadata)
	log.Printf("[OSS同步服务] 生成OSS路径: %s", ossPath)
	
	syncLog := &database.SyncLog{
		FileID:      fileID,
		OSSConfigID: ossConfig.ID,
		SyncType:    "upload",
		Status:      "pending",
		OSSPath:     ossPath,
		FileSize:    fileMetadata.FileSize,
	}

	log.Println("[OSS同步服务] 正在创建同步日志记录")
	if err := s.db.Create(syncLog).Error; err != nil {
		log.Printf("[OSS同步服务] 创建同步日志失败: %v", err)
		return fmt.Errorf("failed to create sync log: %w", err)
	}
	log.Printf("[OSS同步服务] 同步日志创建成功, 日志ID: %d", syncLog.ID)

	// 执行同步
	log.Println("[OSS同步服务] 启动异步上传任务")
	go s.performSync(syncLog, ossConfig, fileMetadata)

	log.Printf("[OSS同步服务] 文件同步任务已启动, 文件ID: %s", fileID)
	return nil
}

// SyncFromOSS 从OSS同步文件到本地
// 功能: 从云存储服务下载文件到本地
// 参数:
//   fileID: 本地文件ID
//   ossPath: OSS中的文件路径
// 返回:
//   error: 同步过程中的错误信息
func (s *ossSyncService) SyncFromOSS(fileID string, ossPath string) error {
	log.Printf("[OSS同步服务] 开始从OSS同步文件到本地, 文件ID: %s, OSS路径: %s", fileID, ossPath)
	
	// 获取激活的OSS配置
	log.Println("[OSS同步服务] 正在获取激活的OSS配置")
	ossConfig, err := s.getActiveOSSConfig()
	if err != nil {
		log.Printf("[OSS同步服务] 获取OSS配置失败: %v", err)
		return err
	}
	log.Printf("[OSS同步服务] 成功获取OSS配置, 提供商: %s", ossConfig.Provider)

	// 检查是否已经在同步中
	log.Println("[OSS同步服务] 检查是否存在进行中的下载任务")
	var existingLog database.SyncLog
	if dbErr := s.db.Where("file_id = ? AND oss_config_id = ? AND sync_type = ? AND status = ?",
		fileID, ossConfig.ID, "download", "pending").First(&existingLog).Error; dbErr == nil {
		log.Printf("[OSS同步服务] 文件正在下载中, 同步日志ID: %d", existingLog.ID)
		return ErrSyncInProgress
	}

	// 创建OSS提供商实例
	log.Printf("[OSS同步服务] 正在创建OSS提供商实例, 提供商: %s", ossConfig.Provider)
	provider, err := s.factory.CreateProvider(ossConfig)
	if err != nil {
		log.Printf("[OSS同步服务] 创建OSS提供商实例失败: %v", err)
		return fmt.Errorf("failed to create OSS provider: %w", err)
	}
	log.Println("[OSS同步服务] OSS提供商实例创建成功")

	// 检查OSS文件是否存在
	log.Printf("[OSS同步服务] 检查OSS文件是否存在: %s", ossPath)
	exists, err := provider.FileExists(ossPath)
	if err != nil {
		log.Printf("[OSS同步服务] 检查OSS文件存在性失败: %v", err)
		return fmt.Errorf("failed to check OSS file existence: %w", err)
	}
	if !exists {
		log.Printf("[OSS同步服务] OSS文件不存在: %s", ossPath)
		return fmt.Errorf("file not found in OSS: %s", ossPath)
	}
	log.Println("[OSS同步服务] OSS文件存在，可以下载")

	// 获取OSS文件信息
	log.Printf("[OSS同步服务] 正在获取OSS文件信息: %s", ossPath)
	ossFileInfo, err := provider.GetFileInfo(ossPath)
	if err != nil {
		log.Printf("[OSS同步服务] 获取OSS文件信息失败: %v", err)
		return fmt.Errorf("failed to get OSS file info: %w", err)
	}
	log.Printf("[OSS同步服务] 成功获取OSS文件信息, 大小: %d bytes", ossFileInfo.Size)

	// 创建同步日志
	syncLog := &database.SyncLog{
		FileID:      fileID,
		OSSConfigID: ossConfig.ID,
		SyncType:    "download",
		Status:      "pending",
		OSSPath:     ossPath,
		FileSize:    ossFileInfo.Size,
	}

	log.Println("[OSS同步服务] 正在创建下载同步日志记录")
	if err := s.db.Create(syncLog).Error; err != nil {
		log.Printf("[OSS同步服务] 创建同步日志失败: %v", err)
		return fmt.Errorf("failed to create sync log: %w", err)
	}
	log.Printf("[OSS同步服务] 下载同步日志创建成功, 日志ID: %d", syncLog.ID)

	// 执行下载同步
	log.Println("[OSS同步服务] 启动异步下载任务")
	go s.performDownloadSync(syncLog, ossConfig, ossFileInfo)

	log.Printf("[OSS同步服务] 文件下载任务已启动, 文件ID: %s", fileID)
	return nil
}

// BatchSyncToOSS 批量同步文件到OSS
// 功能: 批量将多个本地文件上传到云存储服务
// 参数:
//   fileIDs: 要同步的文件ID列表
// 返回:
//   error: 批量同步过程中的错误信息
func (s *ossSyncService) BatchSyncToOSS(fileIDs []string) error {
	log.Printf("[OSS同步服务] 开始批量同步文件到OSS, 文件数量: %d", len(fileIDs))
	
	var errors []string
	successCount := 0

	for i, fileID := range fileIDs {
		log.Printf("[OSS同步服务] 正在同步第 %d/%d 个文件, 文件ID: %s", i+1, len(fileIDs), fileID)
		
		if err := s.SyncToOSS(fileID); err != nil {
			errorMsg := fmt.Sprintf("file %s: %v", fileID, err)
			errors = append(errors, errorMsg)
			log.Printf("[OSS同步服务] 文件同步失败: %s", errorMsg)
		} else {
			successCount++
			log.Printf("[OSS同步服务] 文件同步任务启动成功, 文件ID: %s", fileID)
		}
	}

	log.Printf("[OSS同步服务] 批量同步完成, 成功启动: %d, 失败: %d", successCount, len(errors))
	
	if len(errors) > 0 {
		errorMsg := fmt.Sprintf("batch sync errors: %s", strings.Join(errors, "; "))
		log.Printf("[OSS同步服务] 批量同步存在错误: %s", errorMsg)
		return fmt.Errorf("[OSS同步服务] 批量同步存在错误: %s", errorMsg)
	}

	log.Println("[OSS同步服务] 批量同步全部成功")
	return nil
}

// SyncAllFromOSS 从OSS同步所有文件到本地
// 功能: 从云存储服务下载所有文件到本地存储
// 返回:
//   error: 同步过程中的错误信息
func (s *ossSyncService) SyncAllFromOSS() error {
	log.Println("[OSS同步服务] 开始从OSS同步所有文件到本地")
	
	// 获取激活的OSS配置
	log.Println("[OSS同步服务] 正在获取激活的OSS配置")
	ossConfig, err := s.getActiveOSSConfig()
	if err != nil {
		log.Printf("[OSS同步服务] 获取OSS配置失败: %v", err)
		return err
	}
	log.Printf("[OSS同步服务] 成功获取OSS配置, 提供商: %s, 同步路径: %s", ossConfig.Provider, ossConfig.SyncPath)

	// 创建OSS提供商实例
	log.Printf("[OSS同步服务] 正在创建OSS提供商实例, 提供商: %s", ossConfig.Provider)
	provider, err := s.factory.CreateProvider(ossConfig)
	if err != nil {
		log.Printf("[OSS同步服务] 创建OSS提供商实例失败: %v", err)
		return fmt.Errorf("failed to create OSS provider: %w", err)
	}
	log.Println("[OSS同步服务] OSS提供商实例创建成功")

	// 列出OSS中的所有文件
	log.Printf("[OSS同步服务] 正在列出OSS中的文件, 路径: %s", ossConfig.SyncPath)
	ossFiles, err := provider.ListFiles(ossConfig.SyncPath, 1000)
	if err != nil {
		log.Printf("[OSS同步服务] 列出OSS文件失败: %v", err)
		return fmt.Errorf("failed to list OSS files: %w", err)
	}
	log.Printf("[OSS同步服务] 成功列出OSS文件, 文件数量: %d", len(ossFiles))

	// 检查是否已经在同步中
	log.Println("[OSS同步服务] 检查是否存在进行中的下载任务")
	var inProgressCount int64
	if err := s.db.Model(&database.SyncLog{}).Where("sync_type = ? AND status = ?", "download", "pending").Count(&inProgressCount).Error; err != nil {
		log.Printf("[OSS同步服务] 检查同步状态失败: %v", err)
		return fmt.Errorf("failed to check sync status: %w", err)
	}

	if inProgressCount > 0 {
		log.Printf("[OSS同步服务] 存在 %d 个进行中的下载任务，无法启动全量同步", inProgressCount)
		return ErrSyncInProgress
	}
	log.Println("[OSS同步服务] 没有进行中的下载任务，可以开始全量同步")

	// 开始同步每个文件
	log.Printf("[OSS同步服务] 开始为 %d 个文件创建下载任务", len(ossFiles))
	var syncErrors []string
	successCount := 0
	
	for i, ossFile := range ossFiles {
		log.Printf("[OSS同步服务] 正在处理第 %d/%d 个文件: %s", i+1, len(ossFiles), ossFile.Key)
		
		// 为每个文件生成唯一的ID
		fileID := uuid.New().String()
		log.Printf("[OSS同步服务] 为文件生成ID: %s -> %s", ossFile.Key, fileID)

		// 创建同步日志
		syncLog := &database.SyncLog{
			FileID:      fileID,
			OSSConfigID: ossConfig.ID,
			SyncType:    "download",
			Status:      "pending",
			OSSPath:     ossFile.Key,
			FileSize:    ossFile.Size,
		}

		if err := s.db.Create(syncLog).Error; err != nil {
			errorMsg := fmt.Sprintf("failed to create sync log for %s: %v", ossFile.Key, err)
			syncErrors = append(syncErrors, errorMsg)
			log.Printf("[OSS同步服务] 创建同步日志失败: %s", errorMsg)
			continue
		}
		log.Printf("[OSS同步服务] 同步日志创建成功, 日志ID: %d", syncLog.ID)

		// 异步执行下载同步
		go s.performDownloadSync(syncLog, ossConfig, &ossFile)
		successCount++
		log.Printf("[OSS同步服务] 文件下载任务启动成功: %s", ossFile.Key)
	}

	log.Printf("[OSS同步服务] 全量同步任务创建完成, 成功: %d, 失败: %d", successCount, len(syncErrors))
	
	if len(syncErrors) > 0 {
		errorMsg := fmt.Sprintf("some files failed to start sync: %s", strings.Join(syncErrors, "; "))
		log.Printf("[OSS同步服务] 部分文件同步启动失败: %s", errorMsg)
		return fmt.Errorf("[OSS同步服务] 部分文件同步启动失败: %s", errorMsg)
	}

	log.Println("[OSS同步服务] 全量同步任务全部启动成功")
	return nil
}

// ScanAndCompareFiles 扫描文件表并与云端对比
// 功能: 对比本地文件和云端文件，找出差异
// 返回:
//   []string: 需要更新的文件列表
//   []string: 仅存在于云端的文件列表
//   error: 扫描过程中的错误信息
func (s *ossSyncService) ScanAndCompareFiles() ([]string, []string, error) {
	log.Println("[OSS同步服务] 开始扫描文件表并与云端对比")
	
	// 获取激活的OSS配置
	log.Println("[OSS同步服务] 正在获取激活的OSS配置")
	ossConfig, err := s.getActiveOSSConfig()
	if err != nil {
		log.Printf("[OSS同步服务] 获取OSS配置失败: %v", err)
		return nil, nil, err
	}
	log.Printf("[OSS同步服务] 成功获取OSS配置, 提供商: %s", ossConfig.Provider)

	// 创建OSS提供商实例
	log.Printf("[OSS同步服务] 正在创建OSS提供商实例, 提供商: %s", ossConfig.Provider)
	provider, err := s.factory.CreateProvider(ossConfig)
	if err != nil {
		log.Printf("[OSS同步服务] 创建OSS提供商实例失败: %v", err)
		return nil, nil, fmt.Errorf("failed to create OSS provider: %w", err)
	}
	log.Println("[OSS同步服务] OSS提供商实例创建成功")

	// 列出OSS中的所有文件
	log.Printf("[OSS同步服务] 正在列出OSS中的文件, 路径: %s", ossConfig.SyncPath)
	ossFiles, err := provider.ListFiles(ossConfig.SyncPath, 1000)
	if err != nil {
		log.Printf("[OSS同步服务] 列出OSS文件失败: %v", err)
		return nil, nil, fmt.Errorf("failed to list OSS files: %w", err)
	}
	log.Printf("[OSS同步服务] 成功列出OSS文件, 文件数量: %d", len(ossFiles))

	// 将OSS文件存储在map中以提高查找效率
	log.Println("[OSS同步服务] 正在构建OSS文件索引")
	ossFileMap := make(map[string]FileInfo)
	for _, file := range ossFiles {
		ossFileMap[file.Key] = file
	}
	log.Printf("[OSS同步服务] OSS文件索引构建完成, 索引数量: %d", len(ossFileMap))

	// 查询所有本地文件
	log.Println("[OSS同步服务] 正在查询本地文件")
	var localFiles []database.FileMetadata
	if err := s.db.Find(&localFiles).Error; err != nil {
		log.Printf("[OSS同步服务] 查询本地文件失败: %v", err)
		return nil, nil, fmt.Errorf("failed to get local files: %w", err)
	}
	log.Printf("[OSS同步服务] 成功查询本地文件, 文件数量: %d", len(localFiles))

	// 需要更新的文件列表（本地不存在或有差异）
	needUpdateFiles := []string{}
	// 仅存在于云端的文件列表
	cloudOnlyFiles := []string{}

	// 检查云端文件在本地的存在情况和状态
	log.Println("[OSS同步服务] 开始对比云端文件与本地文件")
	for key, ossFile := range ossFileMap {
		// 检查是否存在于本地
		existsLocally := false
		for _, localFile := range localFiles {
			// 这里简化处理，实际应该有更好的文件名映射逻辑
			if strings.Contains(localFile.FileName, filepath.Base(key)) {
				existsLocally = true
				// 检查文件大小是否匹配（简化的对比逻辑）
				if localFile.FileSize != ossFile.Size {
					needUpdateFiles = append(needUpdateFiles, key)
					log.Printf("[OSS同步服务] 发现文件大小不匹配: %s (本地: %d, 云端: %d)", key, localFile.FileSize, ossFile.Size)
				}
				break
			}
		}

		if !existsLocally {
			cloudOnlyFiles = append(cloudOnlyFiles, key)
			log.Printf("[OSS同步服务] 发现仅存在于云端的文件: %s", key)
		}
	}

	log.Printf("[OSS同步服务] 文件对比完成, 需要更新: %d, 仅云端存在: %d", len(needUpdateFiles), len(cloudOnlyFiles))
	return needUpdateFiles, cloudOnlyFiles, nil
}

// GetSyncLogs 获取同步日志
// 功能: 分页查询同步日志记录
// 参数:
//   page: 页码
//   pageSize: 每页大小
// 返回:
//   []database.SyncLog: 同步日志列表
//   int64: 总记录数
//   error: 查询过程中的错误信息
func (s *ossSyncService) GetSyncLogs(page, pageSize int) ([]database.SyncLog, int64, error) {
	log.Printf("[OSS同步服务] 开始获取同步日志, 页码: %d, 每页大小: %d", page, pageSize)
	
	var logs []database.SyncLog
	var total int64

	// 获取总数
	log.Println("[OSS同步服务] 正在统计同步日志总数")
	if err := s.db.Model(&database.SyncLog{}).Count(&total).Error; err != nil {
		log.Printf("[OSS同步服务] 统计同步日志总数失败: %v", err)
		return nil, 0, err
	}
	log.Printf("[OSS同步服务] 同步日志总数: %d", total)

	// 分页查询
	offset := (page - 1) * pageSize
	log.Printf("[OSS同步服务] 正在分页查询同步日志, 偏移量: %d, 限制: %d", offset, pageSize)
	if err := s.db.Preload("OSSConfig").Offset(offset).Limit(pageSize).
		Order("created_at DESC").Find(&logs).Error; err != nil {
		log.Printf("[OSS同步服务] 分页查询同步日志失败: %v", err)
		return nil, 0, err
	}

	log.Printf("[OSS同步服务] 成功获取同步日志, 返回记录数: %d", len(logs))
	return logs, total, nil
}

// GetFileSyncStatus 获取文件的同步状态
// 功能: 查询指定文件的最新同步状态
// 参数:
//   fileID: 文件ID
// 返回:
//   *database.SyncLog: 最新的同步日志
//   error: 查询过程中的错误信息
func (s *ossSyncService) GetFileSyncStatus(fileID string) (*database.SyncLog, error) {
	log.Printf("[OSS同步服务] 开始获取文件同步状态, 文件ID: %s", fileID)
	
	var syncLog database.SyncLog
	log.Printf("[OSS同步服务] 正在查询文件的最新同步日志, 文件ID: %s", fileID)
	if err := s.db.Where("file_id = ?", fileID).
		Preload("OSSConfig").Order("created_at DESC").First(&syncLog).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("[OSS同步服务] 未找到文件的同步日志, 文件ID: %s", fileID)
			return nil, fmt.Errorf("no sync log found for file: %s", fileID)
		}
		log.Printf("[OSS同步服务] 查询文件同步状态失败: %v", err)
		return nil, err
	}

	log.Printf("[OSS同步服务] 成功获取文件同步状态, 文件ID: %s, 状态: %s", fileID, syncLog.Status)
	return &syncLog, nil
}

// RetryFailedSync 重试失败的同步任务
// 功能: 重新执行失败的同步任务
// 参数:
//   logID: 同步日志ID
// 返回:
//   error: 重试过程中的错误信息
func (s *ossSyncService) RetryFailedSync(logID uint) error {
	log.Printf("[OSS同步服务] 开始重试失败的同步任务, 日志ID: %d", logID)
	
	var syncLog database.SyncLog
	log.Printf("[OSS同步服务] 正在查询同步日志, 日志ID: %d", logID)
	if err := s.db.Preload("OSSConfig").First(&syncLog, logID).Error; err != nil {
		log.Printf("[OSS同步服务] 查询同步日志失败: %v", err)
		return fmt.Errorf("sync log not found: %w", err)
	}
	log.Printf("[OSS同步服务] 成功查询同步日志, 文件ID: %s, 当前状态: %s", syncLog.FileID, syncLog.Status)

	if syncLog.Status != "failed" {
		log.Printf("[OSS同步服务] 同步日志状态不是失败状态，无法重试, 当前状态: %s", syncLog.Status)
		return fmt.Errorf("sync log status is not failed: %s", syncLog.Status)
	}

	// 重置状态为pending
	log.Println("[OSS同步服务] 正在重置同步日志状态为pending")
	syncLog.Status = "pending"
	syncLog.ErrorMsg = ""
	if err := s.db.Save(&syncLog).Error; err != nil {
		log.Printf("[OSS同步服务] 更新同步日志状态失败: %v", err)
		return fmt.Errorf("failed to update sync log: %w", err)
	}
	log.Printf("[OSS同步服务] 同步日志状态重置成功, 日志ID: %d", logID)

	// 根据同步类型执行重试
	if syncLog.SyncType == "upload" {
		log.Printf("[OSS同步服务] 开始重试上传任务, 文件ID: %s", syncLog.FileID)
		fileMetadata, err := s.fileService.GetFileByID(syncLog.FileID)
		if err != nil {
			log.Printf("[OSS同步服务] 获取文件元数据失败: %v", err)
			return fmt.Errorf("failed to get file metadata: %w", err)
		}
		log.Println("[OSS同步服务] 启动异步上传重试任务")
		go s.performSync(&syncLog, &syncLog.OSSConfig, fileMetadata)
	} else {
		log.Printf("[OSS同步服务] 开始重试下载任务, OSS路径: %s", syncLog.OSSPath)
		ossFileInfo := &FileInfo{
			Key:  syncLog.OSSPath,
			Size: syncLog.FileSize,
		}
		log.Println("[OSS同步服务] 启动异步下载重试任务")
		go s.performDownloadSync(&syncLog, &syncLog.OSSConfig, ossFileInfo)
	}

	log.Printf("[OSS同步服务] 同步任务重试启动成功, 日志ID: %d, 类型: %s", logID, syncLog.SyncType)
	return nil
}

// getActiveOSSConfig 获取激活的OSS配置
// 功能: 查询当前激活且启用的OSS配置
// 返回:
//   *database.OSSConfig: 激活的OSS配置
//   error: 查询过程中的错误信息
func (s *ossSyncService) getActiveOSSConfig() (*database.OSSConfig, error) {
	log.Println("[OSS同步服务] 正在查询激活的OSS配置")
	
	var config database.OSSConfig
	if err := s.db.Where("is_active = ? AND is_enabled = ?", true, true).First(&config).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Println("[OSS同步服务] 未找到激活的OSS配置")
			return nil, ErrNoActiveConfig
		}
		log.Printf("[OSS同步服务] 查询OSS配置失败: %v", err)
		return nil, err
	}

	log.Printf("[OSS同步服务] 成功获取激活的OSS配置, ID: %d, 提供商: %s", config.ID, config.Provider)
	return &config, nil
}

// generateOSSPath 生成OSS路径
// 功能: 根据文件元数据生成OSS存储路径
// 参数:
//   fileMetadata: 文件元数据
// 返回:
//   string: 生成的OSS路径
func (s *ossSyncService) generateOSSPath(fileMetadata *database.FileMetadata) string {
	log.Printf("[OSS同步服务] 正在生成OSS路径, 文件ID: %s, 文件名: %s", fileMetadata.FileID, fileMetadata.FileName)
	
	// 使用年/月/日的目录结构
	now := time.Now()
	datePath := fmt.Sprintf("%d/%02d/%02d", now.Year(), now.Month(), now.Day())
	log.Printf("[OSS同步服务] 生成日期路径: %s", datePath)

	// 使用文件ID作为文件名，保持原扩展名
	ext := filepath.Ext(fileMetadata.FileName)
	fileName := fileMetadata.FileID + ext
	log.Printf("[OSS同步服务] 生成文件名: %s (扩展名: %s)", fileName, ext)

	ossPath := fmt.Sprintf("files/%s/%s", datePath, fileName)
	log.Printf("[OSS同步服务] 生成OSS路径: %s", ossPath)
	return ossPath
}

// performSync 执行上传同步
// 功能: 执行文件上传到OSS的同步操作
// 参数:
//   syncLog: 同步日志记录
//   ossConfig: OSS配置
//   fileMetadata: 文件元数据
func (s *ossSyncService) performSync(syncLog *database.SyncLog, ossConfig *database.OSSConfig, fileMetadata *database.FileMetadata) {
	log.Printf("[OSS同步服务] 开始执行上传同步操作, 文件ID: %s, OSS路径: %s", fileMetadata.FileID, syncLog.OSSPath)
	startTime := time.Now()

	// 创建OSS提供商实例
	log.Printf("[OSS同步服务] 正在创建OSS提供商实例, 提供商: %s", ossConfig.Provider)
	provider, err := s.factory.CreateProvider(ossConfig)
	if err != nil {
		log.Printf("[OSS同步服务] 创建OSS提供商实例失败: %v", err)
		s.updateSyncLogError(syncLog, fmt.Sprintf("failed to create OSS provider: %v", err))
		return
	}
	log.Println("[OSS同步服务] OSS提供商实例创建成功")

	// 打开本地文件
	log.Printf("[OSS同步服务] 正在打开本地文件: %s", fileMetadata.StoragePath)
	file, err := os.Open(fileMetadata.StoragePath)
	if err != nil {
		log.Printf("[OSS同步服务] 打开本地文件失败: %v", err)
		s.updateSyncLogError(syncLog, fmt.Sprintf("failed to open local file: %v", err))
		return
	}
	defer file.Close()
	log.Printf("[OSS同步服务] 本地文件打开成功: %s", fileMetadata.StoragePath)

	// 上传到OSS
	contentType := s.getContentType(fileMetadata.FileFormat)
	log.Printf("[OSS同步服务] 开始上传文件到OSS, 内容类型: %s", contentType)
	if err := provider.UploadFile(syncLog.OSSPath, file, contentType); err != nil {
		log.Printf("[OSS同步服务] 文件上传到OSS失败: %v", err)
		s.updateSyncLogError(syncLog, fmt.Sprintf("failed to upload to OSS: %v", err))
		return
	}
	log.Printf("[OSS同步服务] 文件上传到OSS成功, 文件ID: %s", fileMetadata.FileID)

	// 更新同步日志为成功
	duration := time.Since(startTime).Milliseconds()
	log.Printf("[OSS同步服务] 上传耗时: %d 毫秒", duration)
	updates := map[string]interface{}{
		"status":   "success",
		"duration": duration,
	}

	log.Printf("[OSS同步服务] 正在更新同步日志状态为成功, 日志ID: %d", syncLog.ID)
	if err := s.db.Model(syncLog).Updates(updates).Error; err != nil {
		// 记录日志但不影响同步结果
		log.Printf("[OSS同步服务] 更新同步日志失败: %v", err)
	} else {
		log.Printf("[OSS同步服务] 上传同步操作完成, 文件ID: %s", fileMetadata.FileID)
	}
}

// performDownloadSync 执行下载同步
// 功能: 执行从OSS下载文件到本地的同步操作
// 参数:
//   syncLog: 同步日志记录
//   ossConfig: OSS配置
//   ossFileInfo: OSS文件信息
func (s *ossSyncService) performDownloadSync(syncLog *database.SyncLog, ossConfig *database.OSSConfig, ossFileInfo *FileInfo) {
	log.Printf("[OSS同步服务] 开始执行下载同步操作, 文件ID: %s, OSS路径: %s", syncLog.FileID, syncLog.OSSPath)
	startTime := time.Now()

	// 创建OSS提供商实例
	log.Printf("[OSS同步服务] 正在创建OSS提供商实例, 提供商: %s", ossConfig.Provider)
	provider, err := s.factory.CreateProvider(ossConfig)
	if err != nil {
		log.Printf("[OSS同步服务] 创建OSS提供商实例失败: %v", err)
		s.updateSyncLogError(syncLog, fmt.Sprintf("failed to create OSS provider: %v", err))
		return
	}
	log.Println("[OSS同步服务] OSS提供商实例创建成功")

	// 从OSS下载文件
	log.Printf("[OSS同步服务] 开始从OSS下载文件: %s", syncLog.OSSPath)
	reader, err := provider.DownloadFile(syncLog.OSSPath)
	if err != nil {
		log.Printf("[OSS同步服务] 从OSS下载文件失败: %v", err)
		s.updateSyncLogError(syncLog, fmt.Sprintf("failed to download from OSS: %v", err))
		return
	}
	defer reader.Close()
	log.Printf("[OSS同步服务] 从OSS下载文件成功: %s", syncLog.OSSPath)

	// 从OSS路径提取文件名
	fileName := filepath.Base(syncLog.OSSPath)
	log.Printf("[OSS同步服务] 提取文件名: %s", fileName)

	// 上传到本地文件系统
	log.Printf("[OSS同步服务] 开始保存文件到本地文件系统, 文件名: %s", fileName)
	fileMetadata, err := s.fileService.UploadFile(fileName, reader)
	if err != nil {
		log.Printf("[OSS同步服务] 保存文件到本地失败: %v", err)
		s.updateSyncLogError(syncLog, fmt.Sprintf("failed to save file locally: %v", err))
		return
	}
	log.Printf("[OSS同步服务] 文件保存到本地成功, 本地文件ID: %s", fileMetadata.FileID)

	// 更新同步日志
	duration := time.Since(startTime).Milliseconds()
	log.Printf("[OSS同步服务] 下载耗时: %d 毫秒", duration)
	updates := map[string]interface{}{
		"status":   "success",
		"duration": duration,
		"file_id":  fileMetadata.FileID, // 更新为本地文件ID
	}

	log.Printf("[OSS同步服务] 正在更新同步日志状态为成功, 日志ID: %d", syncLog.ID)
	if err := s.db.Model(syncLog).Updates(updates).Error; err != nil {
		log.Printf("[OSS同步服务] 更新同步日志失败: %v", err)
	} else {
		log.Printf("[OSS同步服务] 下载同步操作完成, 文件ID: %s", syncLog.FileID)
	}
}

// updateSyncLogError 更新同步日志错误信息
// 功能: 更新同步日志的错误状态和错误信息
// 参数:
//   syncLog: 同步日志记录
//   errorMsg: 错误信息
func (s *ossSyncService) updateSyncLogError(syncLog *database.SyncLog, errorMsg string) {
	log.Printf("[OSS同步服务] 更新同步日志错误信息, 日志ID: %d, 错误: %s", syncLog.ID, errorMsg)
	
	// 对于临时失败，使用pending_retry状态而不是failed
	updates := map[string]interface{}{
		"status":    "pending_retry",
		"error_msg": errorMsg,
	}

	log.Printf("[OSS同步服务] 正在更新同步日志状态为pending_retry, 日志ID: %d", syncLog.ID)
	if err := s.db.Model(syncLog).Updates(updates).Error; err != nil {
		// 避免报错，只记录状态
		log.Printf("[OSS同步服务] 更新同步日志状态失败: %v", err)
	} else {
		log.Printf("[OSS同步服务] 同步日志错误信息更新成功, 日志ID: %d", syncLog.ID)
	}
}

// getContentType 根据文件格式获取内容类型
// 功能: 根据文件格式判断并返回对应的MIME类型
// 参数:
//   fileFormat: 文件格式（扩展名）
// 返回:
//   string: 对应的MIME类型
func (s *ossSyncService) getContentType(fileFormat string) string {
	log.Printf("[OSS同步服务] 正在判断文件内容类型, 文件格式: %s", fileFormat)
	
	// 使用mime包自动检测文件类型,允许所有格式
	contentTypes := map[string]string{} // 空map,不限制文件类型

	var contentType string
	if ct, exists := contentTypes[strings.ToLower(fileFormat)]; exists {
		contentType = ct
	} else {
		contentType = "application/octet-stream"
	}

	log.Printf("[OSS同步服务] 文件内容类型判断完成, 格式: %s, 类型: %s", fileFormat, contentType)
	return contentType
}
