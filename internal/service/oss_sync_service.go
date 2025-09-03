package service

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/weiwangfds/scinote/internal/database"
	"gorm.io/gorm"
)

var (
	ErrUnsupportedProvider = errors.New("unsupported OSS provider")
	ErrNoActiveConfig      = errors.New("no active OSS configuration found")
	ErrSyncInProgress      = errors.New("sync operation already in progress")
)

// OSSyncService OSS同步服务接口
type OSSyncService interface {
	// 同步文件到OSS
	SyncToOSS(fileID string) error

	// 从OSS同步文件到本地
	SyncFromOSS(fileID string, ossPath string) error

	// 批量同步文件到OSS
	BatchSyncToOSS(fileIDs []string) error

	// 从OSS同步所有文件到本地
	SyncAllFromOSS() error

	// 扫描文件表并与云端对比
	ScanAndCompareFiles() ([]string, []string, error)

	// 获取同步日志
	GetSyncLogs(page, pageSize int) ([]database.SyncLog, int64, error)

	// 获取文件的同步状态
	GetFileSyncStatus(fileID string) (*database.SyncLog, error)

	// 重试失败的同步任务
	RetryFailedSync(logID uint) error
}

// ossSyncService OSS同步服务实现
type ossSyncService struct {
	db          *gorm.DB
	fileService FileService
	factory     *OSSProviderFactory
}

// NewOSSyncService 创建OSS同步服务实例
func NewOSSyncService(db *gorm.DB, fileService FileService) OSSyncService {
	return &ossSyncService{
		db:          db,
		fileService: fileService,
		factory:     &OSSProviderFactory{},
	}
}

// SyncToOSS 同步文件到OSS
func (s *ossSyncService) SyncToOSS(fileID string) error {
	// 获取激活的OSS配置
	ossConfig, err := s.getActiveOSSConfig()
	if err != nil {
		return err
	}

	// 获取文件信息
	fileMetadata, err := s.fileService.GetFileByID(fileID)
	if err != nil {
		return fmt.Errorf("failed to get file metadata: %w", err)
	}

	// 检查是否已经在同步中
	var existingLog database.SyncLog
	if err := s.db.Where("file_id = ? AND oss_config_id = ? AND sync_type = ? AND status = ?",
		fileID, ossConfig.ID, "upload", "pending").First(&existingLog).Error; err == nil {
		return ErrSyncInProgress
	}

	// 创建同步日志
	syncLog := &database.SyncLog{
		FileID:      fileID,
		OSSConfigID: ossConfig.ID,
		SyncType:    "upload",
		Status:      "pending",
		OSSPath:     s.generateOSSPath(fileMetadata),
		FileSize:    fileMetadata.FileSize,
	}

	if err := s.db.Create(syncLog).Error; err != nil {
		return fmt.Errorf("failed to create sync log: %w", err)
	}

	// 执行同步
	go s.performSync(syncLog, ossConfig, fileMetadata)

	return nil
}

// SyncFromOSS 从OSS同步文件到本地
func (s *ossSyncService) SyncFromOSS(fileID string, ossPath string) error {
	// 获取激活的OSS配置
	ossConfig, err := s.getActiveOSSConfig()
	if err != nil {
		return err
	}

	// 检查是否已经在同步中
	var existingLog database.SyncLog
	if err := s.db.Where("file_id = ? AND oss_config_id = ? AND sync_type = ? AND status = ?",
		fileID, ossConfig.ID, "download", "pending").First(&existingLog).Error; err == nil {
		return ErrSyncInProgress
	}

	// 创建OSS提供商实例
	provider, err := s.factory.CreateProvider(ossConfig)
	if err != nil {
		return fmt.Errorf("failed to create OSS provider: %w", err)
	}

	// 检查OSS文件是否存在
	exists, err := provider.FileExists(ossPath)
	if err != nil {
		return fmt.Errorf("failed to check OSS file existence: %w", err)
	}
	if !exists {
		return fmt.Errorf("file not found in OSS: %s", ossPath)
	}

	// 获取OSS文件信息
	ossFileInfo, err := provider.GetFileInfo(ossPath)
	if err != nil {
		return fmt.Errorf("failed to get OSS file info: %w", err)
	}

	// 创建同步日志
	syncLog := &database.SyncLog{
		FileID:      fileID,
		OSSConfigID: ossConfig.ID,
		SyncType:    "download",
		Status:      "pending",
		OSSPath:     ossPath,
		FileSize:    ossFileInfo.Size,
	}

	if err := s.db.Create(syncLog).Error; err != nil {
		return fmt.Errorf("failed to create sync log: %w", err)
	}

	// 执行下载同步
	go s.performDownloadSync(syncLog, ossConfig, ossFileInfo)

	return nil
}

// BatchSyncToOSS 批量同步文件到OSS
func (s *ossSyncService) BatchSyncToOSS(fileIDs []string) error {
	var errors []string

	for _, fileID := range fileIDs {
		if err := s.SyncToOSS(fileID); err != nil {
			errors = append(errors, fmt.Sprintf("file %s: %v", fileID, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("batch sync errors: %s", strings.Join(errors, "; "))
	}

	return nil
}

// SyncAllFromOSS 从OSS同步所有文件到本地
func (s *ossSyncService) SyncAllFromOSS() error {
	// 获取激活的OSS配置
	ossConfig, err := s.getActiveOSSConfig()
	if err != nil {
		return err
	}

	// 创建OSS提供商实例
	provider, err := s.factory.CreateProvider(ossConfig)
	if err != nil {
		return fmt.Errorf("failed to create OSS provider: %w", err)
	}

	// 列出OSS中的所有文件
	ossFiles, err := provider.ListFiles(ossConfig.SyncPath, 1000)
	if err != nil {
		return fmt.Errorf("failed to list OSS files: %w", err)
	}

	// 检查是否已经在同步中
	var inProgressCount int64
	if err := s.db.Model(&database.SyncLog{}).Where("sync_type = ? AND status = ?", "download", "pending").Count(&inProgressCount).Error; err != nil {
		return fmt.Errorf("failed to check sync status: %w", err)
	}

	if inProgressCount > 0 {
		return ErrSyncInProgress
	}

	// 开始同步每个文件
	var syncErrors []string
	for _, ossFile := range ossFiles {
		// 为每个文件生成唯一的ID
		fileID := uuid.New().String()

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
			syncErrors = append(syncErrors, fmt.Sprintf("failed to create sync log for %s: %v", ossFile.Key, err))
			continue
		}

		// 异步执行下载同步
		go s.performDownloadSync(syncLog, ossConfig, &ossFile)
	}

	if len(syncErrors) > 0 {
		return fmt.Errorf("some files failed to start sync: %s", strings.Join(syncErrors, "; "))
	}

	return nil
}

// ScanAndCompareFiles 扫描文件表并与云端对比
func (s *ossSyncService) ScanAndCompareFiles() ([]string, []string, error) {
	// 获取激活的OSS配置
	ossConfig, err := s.getActiveOSSConfig()
	if err != nil {
		return nil, nil, err
	}

	// 创建OSS提供商实例
	provider, err := s.factory.CreateProvider(ossConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create OSS provider: %w", err)
	}

	// 列出OSS中的所有文件
	ossFiles, err := provider.ListFiles(ossConfig.SyncPath, 1000)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list OSS files: %w", err)
	}

	// 将OSS文件存储在map中以提高查找效率
	ossFileMap := make(map[string]FileInfo)
	for _, file := range ossFiles {
		ossFileMap[file.Key] = file
	}

	// 查询所有本地文件
	var localFiles []database.FileMetadata
	if err := s.db.Find(&localFiles).Error; err != nil {
		return nil, nil, fmt.Errorf("failed to get local files: %w", err)
	}

	// 需要更新的文件列表（本地不存在或有差异）
	needUpdateFiles := []string{}
	// 仅存在于云端的文件列表
	cloudOnlyFiles := []string{}

	// 检查云端文件在本地的存在情况和状态
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
				}
				break
			}
		}

		if !existsLocally {
			cloudOnlyFiles = append(cloudOnlyFiles, key)
		}
	}

	return needUpdateFiles, cloudOnlyFiles, nil
}

// GetSyncLogs 获取同步日志
func (s *ossSyncService) GetSyncLogs(page, pageSize int) ([]database.SyncLog, int64, error) {
	var logs []database.SyncLog
	var total int64

	// 获取总数
	if err := s.db.Model(&database.SyncLog{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	if err := s.db.Preload("OSSConfig").Offset(offset).Limit(pageSize).
		Order("created_at DESC").Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

// GetFileSyncStatus 获取文件的同步状态
func (s *ossSyncService) GetFileSyncStatus(fileID string) (*database.SyncLog, error) {
	var log database.SyncLog
	if err := s.db.Where("file_id = ?", fileID).
		Preload("OSSConfig").Order("created_at DESC").First(&log).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("no sync log found for file: %s", fileID)
		}
		return nil, err
	}

	return &log, nil
}

// RetryFailedSync 重试失败的同步任务
func (s *ossSyncService) RetryFailedSync(logID uint) error {
	var syncLog database.SyncLog
	if err := s.db.Preload("OSSConfig").First(&syncLog, logID).Error; err != nil {
		return fmt.Errorf("sync log not found: %w", err)
	}

	if syncLog.Status != "failed" {
		return fmt.Errorf("sync log status is not failed: %s", syncLog.Status)
	}

	// 重置状态为pending
	syncLog.Status = "pending"
	syncLog.ErrorMsg = ""
	if err := s.db.Save(&syncLog).Error; err != nil {
		return fmt.Errorf("failed to update sync log: %w", err)
	}

	// 根据同步类型执行重试
	if syncLog.SyncType == "upload" {
		fileMetadata, err := s.fileService.GetFileByID(syncLog.FileID)
		if err != nil {
			return fmt.Errorf("failed to get file metadata: %w", err)
		}
		go s.performSync(&syncLog, &syncLog.OSSConfig, fileMetadata)
	} else {
		ossFileInfo := &FileInfo{
			Key:  syncLog.OSSPath,
			Size: syncLog.FileSize,
		}
		go s.performDownloadSync(&syncLog, &syncLog.OSSConfig, ossFileInfo)
	}

	return nil
}

// getActiveOSSConfig 获取激活的OSS配置
func (s *ossSyncService) getActiveOSSConfig() (*database.OSSConfig, error) {
	var config database.OSSConfig
	if err := s.db.Where("is_active = ? AND is_enabled = ?", true, true).First(&config).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNoActiveConfig
		}
		return nil, err
	}

	return &config, nil
}

// generateOSSPath 生成OSS路径
func (s *ossSyncService) generateOSSPath(fileMetadata *database.FileMetadata) string {
	// 使用年/月/日的目录结构
	now := time.Now()
	datePath := fmt.Sprintf("%d/%02d/%02d", now.Year(), now.Month(), now.Day())

	// 使用文件ID作为文件名，保持原扩展名
	ext := filepath.Ext(fileMetadata.FileName)
	fileName := fileMetadata.FileID + ext

	return fmt.Sprintf("files/%s/%s", datePath, fileName)
}

// performSync 执行上传同步
func (s *ossSyncService) performSync(syncLog *database.SyncLog, ossConfig *database.OSSConfig, fileMetadata *database.FileMetadata) {
	startTime := time.Now()

	// 创建OSS提供商实例
	provider, err := s.factory.CreateProvider(ossConfig)
	if err != nil {
		s.updateSyncLogError(syncLog, fmt.Sprintf("failed to create OSS provider: %v", err))
		return
	}

	// 打开本地文件
	file, err := os.Open(fileMetadata.StoragePath)
	if err != nil {
		s.updateSyncLogError(syncLog, fmt.Sprintf("failed to open local file: %v", err))
		return
	}
	defer file.Close()

	// 上传到OSS
	contentType := s.getContentType(fileMetadata.FileFormat)
	if err := provider.UploadFile(syncLog.OSSPath, file, contentType); err != nil {
		s.updateSyncLogError(syncLog, fmt.Sprintf("failed to upload to OSS: %v", err))
		return
	}

	// 更新同步日志为成功
	duration := time.Since(startTime).Milliseconds()
	updates := map[string]interface{}{
		"status":   "success",
		"duration": duration,
	}

	if err := s.db.Model(syncLog).Updates(updates).Error; err != nil {
		// 记录日志但不影响同步结果
		fmt.Printf("Failed to update sync log: %v\n", err)
	}
}

// performDownloadSync 执行下载同步
func (s *ossSyncService) performDownloadSync(syncLog *database.SyncLog, ossConfig *database.OSSConfig, ossFileInfo *FileInfo) {
	startTime := time.Now()

	// 创建OSS提供商实例
	provider, err := s.factory.CreateProvider(ossConfig)
	if err != nil {
		s.updateSyncLogError(syncLog, fmt.Sprintf("failed to create OSS provider: %v", err))
		return
	}

	// 从OSS下载文件
	reader, err := provider.DownloadFile(syncLog.OSSPath)
	if err != nil {
		s.updateSyncLogError(syncLog, fmt.Sprintf("failed to download from OSS: %v", err))
		return
	}
	defer reader.Close()

	// 从OSS路径提取文件名
	fileName := filepath.Base(syncLog.OSSPath)

	// 上传到本地文件系统
	fileMetadata, err := s.fileService.UploadFile(fileName, reader)
	if err != nil {
		s.updateSyncLogError(syncLog, fmt.Sprintf("failed to save file locally: %v", err))
		return
	}

	// 更新同步日志
	duration := time.Since(startTime).Milliseconds()
	updates := map[string]interface{}{
		"status":   "success",
		"duration": duration,
		"file_id":  fileMetadata.FileID, // 更新为本地文件ID
	}

	if err := s.db.Model(syncLog).Updates(updates).Error; err != nil {
		fmt.Printf("Failed to update sync log: %v\n", err)
	}
}

// updateSyncLogError 更新同步日志错误信息
func (s *ossSyncService) updateSyncLogError(syncLog *database.SyncLog, errorMsg string) {
	// 对于临时失败，使用pending_retry状态而不是failed
	updates := map[string]interface{}{
		"status":    "pending_retry",
		"error_msg": errorMsg,
	}

	if err := s.db.Model(syncLog).Updates(updates).Error; err != nil {
		// 避免报错，只记录状态
		fmt.Printf("Log update status: %v\n", err)
	}
}

// getContentType 根据文件格式获取内容类型
func (s *ossSyncService) getContentType(fileFormat string) string {
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
