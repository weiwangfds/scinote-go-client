package service

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/weiwangfds/scinote/config"
	"github.com/weiwangfds/scinote/internal/database"
	"gorm.io/gorm"
)

// FileService 文件服务接口
type FileService interface {
	// 上传文件
	UploadFile(fileName string, fileData io.Reader) (*database.FileMetadata, error)

	// 根据文件ID获取文件信息
	GetFileByID(fileID string) (*database.FileMetadata, error)

	// 根据文件ID获取文件内容
	GetFileContent(fileID string) (io.ReadCloser, error)

	// 更新文件内容
	UpdateFile(fileID string, fileData io.Reader) (*database.FileMetadata, error)

	// 删除文件
	DeleteFile(fileID string) error

	// 获取文件列表（分页）
	ListFiles(page, pageSize int) ([]database.FileMetadata, int64, error)

	// 根据文件名搜索文件
	SearchFilesByName(fileName string, page, pageSize int) ([]database.FileMetadata, int64, error)

	// 增加查看次数
	IncrementViewCount(fileID string) error

	// 获取文件统计信息
	GetFileStats() (map[string]interface{}, error)
    
	// 设置OSS同步服务
	SetOSSSyncService(syncService OSSyncService)
}

// fileService 文件服务实现
type fileService struct {
	db          *gorm.DB
	config      config.FileConfig
	ossSyncService OSSyncService
}

// NewFileService 创建文件服务实例
func NewFileService(db *gorm.DB, cfg config.FileConfig) FileService {
	// 确保存储目录存在
	if err := os.MkdirAll(cfg.StoragePath, 0755); err != nil {
		panic(fmt.Sprintf("Failed to create storage directory: %v", err))
	}

	return &fileService{
		db:     db,
		config: cfg,
	}
}

// UploadFile 上传文件
func (s *fileService) UploadFile(fileName string, fileData io.Reader) (*database.FileMetadata, error) {
	// 生成唯一文件ID
	fileID := uuid.New().String()

	// 获取文件扩展名
	fileExt := filepath.Ext(fileName)
	if fileExt == "" {
		fileExt = ".bin" // 默认扩展名
	}

	// 检查文件扩展名是否允许
	if !s.isAllowedExtension(fileExt) {
		return nil, fmt.Errorf("file extension %s is not allowed", fileExt)
	}

	// 构建存储路径
	storagePath := filepath.Join(s.config.StoragePath, fileID+fileExt)

	// 创建临时文件用于计算哈希和大小
	tempFile, err := os.CreateTemp("", "upload_*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	// 将数据写入临时文件并计算哈希
	hasher := sha256.New()
	multiWriter := io.MultiWriter(tempFile, hasher)

	fileSize, err := io.Copy(multiWriter, fileData)
	if err != nil {
		return nil, fmt.Errorf("failed to copy file data: %w", err)
	}

	// 检查文件大小
	if fileSize > s.config.MaxFileSize {
		return nil, fmt.Errorf("file size %d exceeds maximum allowed size %d", fileSize, s.config.MaxFileSize)
	}

	// 计算文件哈希
	fileHash := fmt.Sprintf("%x", hasher.Sum(nil))

	// 检查是否已存在相同哈希的文件
	var existingFile database.FileMetadata
	if err := s.db.Where("file_hash = ?", fileHash).First(&existingFile).Error; err == nil {
		// 文件已存在，返回现有文件信息
		return &existingFile, nil
	}

	// 将临时文件移动到最终位置
	if err := s.moveFile(tempFile.Name(), storagePath); err != nil {
		return nil, fmt.Errorf("failed to move file to storage: %w", err)
	}

	// 创建文件元数据记录
	metadata := &database.FileMetadata{
		FileID:      fileID,
		FileName:    fileName,
		StoragePath: storagePath,
		FileSize:    fileSize,
		FileHash:    fileHash,
		FileFormat:  strings.ToLower(fileExt),
		ViewCount:   0,
		ModifyCount: 0,
	}

	if err := s.db.Create(metadata).Error; err != nil {
		// 如果数据库操作失败，删除已上传的文件
		os.Remove(storagePath)
		return nil, fmt.Errorf("failed to save file metadata: %w", err)
	}

	return metadata, nil
}

// GetFileByID 根据文件ID获取文件信息
func (s *fileService) GetFileByID(fileID string) (*database.FileMetadata, error) {
	var metadata database.FileMetadata
	if err := s.db.Where("file_id = ?", fileID).First(&metadata).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("file not found with id: %s", fileID)
		}
		return nil, err
	}
	return &metadata, nil
}

// GetFileContent 根据文件ID获取文件内容
func (s *fileService) GetFileContent(fileID string) (io.ReadCloser, error) {
	metadata, err := s.GetFileByID(fileID)
	if err != nil {
		return nil, err
	}

	// 检查文件是否存在
	if _, err := os.Stat(metadata.StoragePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("file not found on disk: %s", metadata.StoragePath)
	}

	// 打开文件
	file, err := os.Open(metadata.StoragePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	// 增加查看次数
	go s.IncrementViewCount(fileID)

	return file, nil
}

// UpdateFile 更新文件内容
func (s *fileService) UpdateFile(fileID string, fileData io.Reader) (*database.FileMetadata, error) {
	// 获取现有文件信息
	metadata, err := s.GetFileByID(fileID)
	if err != nil {
		return nil, err
	}

	// 创建临时文件
	tempFile, err := os.CreateTemp("", "update_*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	// 将新数据写入临时文件并计算哈希
	hasher := sha256.New()
	multiWriter := io.MultiWriter(tempFile, hasher)

	fileSize, err := io.Copy(multiWriter, fileData)
	if err != nil {
		return nil, fmt.Errorf("failed to copy file data: %w", err)
	}

	// 检查文件大小
	if fileSize > s.config.MaxFileSize {
		return nil, fmt.Errorf("file size %d exceeds maximum allowed size %d", fileSize, s.config.MaxFileSize)
	}

	// 计算新文件哈希
	newFileHash := fmt.Sprintf("%x", hasher.Sum(nil))

	// 如果哈希相同，说明文件内容未变化
	if newFileHash == metadata.FileHash {
		return metadata, nil
	}

	// 备份原文件
	backupPath := metadata.StoragePath + ".backup"
	if err := s.moveFile(metadata.StoragePath, backupPath); err != nil {
		return nil, fmt.Errorf("failed to backup original file: %w", err)
	}

	// 将新文件移动到原位置
	if err := s.moveFile(tempFile.Name(), metadata.StoragePath); err != nil {
		// 恢复备份文件
		s.moveFile(backupPath, metadata.StoragePath)
		return nil, fmt.Errorf("failed to move new file: %w", err)
	}

	// 更新数据库记录
	updates := map[string]interface{}{
		"file_size":    fileSize,
		"file_hash":    newFileHash,
		"modify_count": gorm.Expr("modify_count + 1"),
		"updated_at":   time.Now(),
	}

	if err := s.db.Model(metadata).Updates(updates).Error; err != nil {
		// 恢复备份文件
		s.moveFile(backupPath, metadata.StoragePath)
		return nil, fmt.Errorf("failed to update file metadata: %w", err)
	}

	// 删除备份文件
	os.Remove(backupPath)

	// 重新获取更新后的数据
	return s.GetFileByID(fileID)
}

// DeleteFile 删除文件
func (s *fileService) DeleteFile(fileID string) error {
	metadata, err := s.GetFileByID(fileID)
	if err != nil {
		return err
	}

	// 如果设置了OSS同步服务，尝试删除云端文件
	if s.ossSyncService != nil {
		// 先尝试获取该文件的同步日志，找到对应的OSS路径
		var syncLog database.SyncLog
		if err := s.db.Where("file_id = ? AND status = ?", fileID, "success").
			Order("created_at DESC").First(&syncLog).Error; err == nil {
			// 有成功的同步记录，尝试从OSS删除
			ossConfig, err := s.ossSyncService.(*ossSyncService).getActiveOSSConfig()
			if err == nil {
				factory := &OSSProviderFactory{}
				provider, err := factory.CreateProvider(ossConfig)
				if err == nil {
					// 异步删除OSS文件，不阻塞主流程
					go func() {
						if err := provider.DeleteFile(syncLog.OSSPath); err != nil {
							fmt.Printf("Failed to delete file from OSS: %v\n", err)
						}
					}()
				}
			}
		}
	}

	// 删除数据库记录（软删除）
	if err := s.db.Delete(metadata).Error; err != nil {
		return fmt.Errorf("failed to delete file metadata: %w", err)
	}

	// 删除物理文件
	if err := os.Remove(metadata.StoragePath); err != nil && !os.IsNotExist(err) {
		// 如果删除物理文件失败，恢复数据库记录
		s.db.Model(metadata).Update("deleted_at", nil)
		return fmt.Errorf("failed to delete physical file: %w", err)
	}

	return nil
}

// ListFiles 获取文件列表（分页）
func (s *fileService) ListFiles(page, pageSize int) ([]database.FileMetadata, int64, error) {
	var files []database.FileMetadata
	var total int64

	// 获取总数
	if err := s.db.Model(&database.FileMetadata{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	if err := s.db.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&files).Error; err != nil {
		return nil, 0, err
	}

	return files, total, nil
}

// SearchFilesByName 根据文件名搜索文件
func (s *fileService) SearchFilesByName(fileName string, page, pageSize int) ([]database.FileMetadata, int64, error) {
	var files []database.FileMetadata
	var total int64

	searchQuery := "%" + fileName + "%"

	// 获取总数
	if err := s.db.Model(&database.FileMetadata{}).Where("file_name LIKE ?", searchQuery).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	if err := s.db.Where("file_name LIKE ?", searchQuery).Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&files).Error; err != nil {
		return nil, 0, err
	}

	return files, total, nil
}

// IncrementViewCount 增加查看次数
func (s *fileService) IncrementViewCount(fileID string) error {
	return s.db.Model(&database.FileMetadata{}).Where("file_id = ?", fileID).Update("view_count", gorm.Expr("view_count + 1")).Error
}

// GetFileStats 获取文件统计信息
func (s *fileService) GetFileStats() (map[string]interface{}, error) {
	var stats struct {
		TotalFiles int64 `json:"total_files"`
		TotalSize  int64 `json:"total_size"`
		TotalViews int64 `json:"total_views"`
	}

	// 统计文件数量和总大小
	if err := s.db.Model(&database.FileMetadata{}).
		Select("COUNT(*) as total_files, COALESCE(SUM(file_size), 0) as total_size, COALESCE(SUM(view_count), 0) as total_views").
		Scan(&stats).Error; err != nil {
		return nil, err
	}

	// 统计各种格式的文件数量
	var formatStats []struct {
		FileFormat string `json:"file_format"`
		Count      int64  `json:"count"`
	}

	if err := s.db.Model(&database.FileMetadata{}).
		Select("file_format, COUNT(*) as count").
		Group("file_format").
		Order("count DESC").
		Scan(&formatStats).Error; err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"total_files":  stats.TotalFiles,
		"total_size":   stats.TotalSize,
		"total_views":  stats.TotalViews,
		"format_stats": formatStats,
	}, nil
}

// isAllowedExtension 检查文件扩展名是否允许
func (s *fileService) isAllowedExtension(ext string) bool {
	// 如果配置为允许所有扩展名
	for _, allowed := range s.config.AllowedExtensions {
		if allowed == "*" {
			return true
		}
		if strings.EqualFold(allowed, ext) {
			return true
		}
	}
	return false
}

// moveFile 移动文件
func (s *fileService) moveFile(src, dst string) error {
	// 确保目标目录存在
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	// 尝试重命名文件（同一文件系统）
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	// 如果重命名失败，则复制文件
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	// 复制成功后删除源文件
	return os.Remove(src)
}

// SetOSSSyncService 设置OSS同步服务
func (s *fileService) SetOSSSyncService(syncService OSSyncService) {
	s.ossSyncService = syncService
}
