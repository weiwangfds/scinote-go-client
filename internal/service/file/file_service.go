// Package service 提供文件管理相关的业务逻辑服务
// 包含文件上传、下载、更新、删除等核心功能
// 支持文件去重、统计分析和OSS同步等高级特性
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
	"github.com/weiwangfds/scinote/internal/logger"
	"gorm.io/gorm"
)

// FileService 文件服务接口
// 提供完整的文件管理功能，包括上传、下载、更新、删除、搜索和统计等操作
// 支持文件去重、访问统计和OSS云存储同步功能
type FileService interface {
	// UploadFile 上传文件到本地存储
	// 参数:
	//   fileName - 原始文件名
	//   fileData - 文件数据流
	// 返回:
	//   *database.FileMetadata - 文件元数据信息
	//   error - 错误信息
	// 功能:
	//   - 自动生成唯一文件ID
	//   - 计算文件哈希值进行去重
	//   - 验证文件大小和扩展名
	//   - 保存文件到本地存储
	UploadFile(fileName string, fileData io.Reader) (*database.FileMetadata, error)

	// GetFileByID 根据文件ID获取文件元数据信息
	// 参数:
	//   fileID - 文件唯一标识符
	// 返回:
	//   *database.FileMetadata - 文件元数据信息
	//   error - 错误信息（如文件不存在）
	GetFileByID(fileID string) (*database.FileMetadata, error)

	// GetFileContent 根据文件ID获取文件内容流
	// 参数:
	//   fileID - 文件唯一标识符
	// 返回:
	//   io.ReadCloser - 文件内容读取器（需要调用者关闭）
	//   error - 错误信息
	// 注意:
	//   - 会自动增加文件查看次数
	//   - 返回的ReadCloser需要调用者负责关闭
	GetFileContent(fileID string) (io.ReadCloser, error)

	// UpdateFile 更新文件内容
	// 参数:
	//   fileID - 文件唯一标识符
	//   fileData - 新的文件数据流
	// 返回:
	//   *database.FileMetadata - 更新后的文件元数据
	//   error - 错误信息
	// 功能:
	//   - 自动备份原文件
	//   - 计算新文件哈希值
	//   - 更新修改次数和时间戳
	UpdateFile(fileID string, fileData io.Reader) (*database.FileMetadata, error)

	// DeleteFile 删除文件（包括数据库记录和物理文件）
	// 参数:
	//   fileID - 文件唯一标识符
	// 返回:
	//   error - 错误信息
	// 功能:
	//   - 软删除数据库记录
	//   - 删除物理文件
	//   - 尝试删除OSS中的文件（如果已同步）
	DeleteFile(fileID string) error

	// ListFiles 获取文件列表（支持分页）
	// 参数:
	//   page - 页码（从1开始）
	//   pageSize - 每页数量
	// 返回:
	//   []database.FileMetadata - 文件列表
	//   int64 - 总文件数量
	//   error - 错误信息
	ListFiles(page, pageSize int) ([]database.FileMetadata, int64, error)

	// SearchFilesByName 根据文件名搜索文件（支持模糊匹配和分页）
	// 参数:
	//   fileName - 搜索关键词
	//   page - 页码（从1开始）
	//   pageSize - 每页数量
	// 返回:
	//   []database.FileMetadata - 匹配的文件列表
	//   int64 - 匹配的文件总数
	//   error - 错误信息
	SearchFilesByName(fileName string, page, pageSize int) ([]database.FileMetadata, int64, error)

	// IncrementViewCount 增加文件查看次数
	// 参数:
	//   fileID - 文件唯一标识符
	// 返回:
	//   error - 错误信息
	// 注意:
	//   - 通常在文件下载或查看时自动调用
	IncrementViewCount(fileID string) error

	// GetFileStats 获取文件统计信息
	// 返回:
	//   map[string]interface{} - 统计信息，包括：
	//     - total_files: 总文件数
	//     - total_size: 总文件大小
	//     - total_views: 总查看次数
	//     - format_stats: 各格式文件统计
	//   error - 错误信息
	GetFileStats() (map[string]interface{}, error)

	// SetOSSSyncService 设置OSS同步服务
	// 参数:
	//   syncService - OSS同步服务实例
	// 功能:
	//   - 用于在文件删除时同步删除OSS中的文件
	SetOSSSyncService(syncService OSSyncService)
}

// fileService 文件服务实现
// 实现FileService接口，提供完整的文件管理功能
type fileService struct {
	db             *gorm.DB          // 数据库连接
	config         config.FileConfig // 文件配置信息
	ossSyncService OSSyncService     // OSS同步服务（可选）
}

// NewFileService 创建文件服务实例
// 参数:
//
//	db - 数据库连接实例
//	cfg - 文件配置信息
//
// 返回:
//
//	FileService - 文件服务接口实例
//
// 功能:
//   - 初始化文件服务
//   - 创建存储目录（如果不存在）
//   - 配置文件大小和扩展名限制
func NewFileService(db *gorm.DB, cfg config.FileConfig) FileService {
	// 确保存储目录存在
	logger.Infof("[文件服务] 初始化文件服务，存储路径: %s", cfg.StoragePath)
	if err := os.MkdirAll(cfg.StoragePath, 0755); err != nil {
		logger.Fatalf("[文件服务] 创建存储目录失败 %s: %v", cfg.StoragePath, err)
		panic(fmt.Sprintf("Failed to create storage directory: %v", err))
	}

	logger.Infof("[文件服务] 文件服务初始化成功。最大文件大小: %d bytes, 允许的扩展名: %v",
		cfg.MaxFileSize, cfg.AllowedExtensions)

	return &fileService{
		db:     db,
		config: cfg,
	}
}

// UploadFile 上传文件到本地存储
// 实现文件上传的完整流程，包括验证、去重、存储等功能
func (s *fileService) UploadFile(fileName string, fileData io.Reader) (*database.FileMetadata, error) {
	logger.Infof("Starting file upload: %s", fileName)

	// 生成唯一文件ID
	fileID := uuid.New().String()
	logger.Infof("Generated file ID: %s for file: %s", fileID, fileName)

	// 获取文件扩展名
	fileExt := filepath.Ext(fileName)
	if fileExt == "" {
		fileExt = ".bin" // 默认扩展名
		logger.Infof("No extension found for file %s, using default: %s", fileName, fileExt)
	}

	// 检查文件扩展名是否允许
	if !s.isAllowedExtension(fileExt) {
		logger.Errorf("File extension %s is not allowed for file: %s", fileExt, fileName)
		return nil, fmt.Errorf("file extension %s is not allowed", fileExt)
	}

	// 构建存储路径
	storagePath := filepath.Join(s.config.StoragePath, fileID+fileExt)
	logger.Infof("Storage path for file %s: %s", fileName, storagePath)

	// 创建临时文件用于计算哈希和大小
	tempFile, err := os.CreateTemp("", "upload_*")
	if err != nil {
		logger.Errorf("Failed to create temp file for %s: %v", fileName, err)
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	// 将数据写入临时文件并计算哈希
	hasher := sha256.New()
	multiWriter := io.MultiWriter(tempFile, hasher)

	fileSize, err := io.Copy(multiWriter, fileData)
	if err != nil {
		logger.Errorf("Failed to copy file data for %s: %v", fileName, err)
		return nil, fmt.Errorf("failed to copy file data: %w", err)
	}

	logger.Infof("File %s size: %d bytes", fileName, fileSize)

	// 检查文件大小
	if fileSize > s.config.MaxFileSize {
		logger.Errorf("File %s size %d exceeds maximum allowed size %d", fileName, fileSize, s.config.MaxFileSize)
		return nil, fmt.Errorf("file size %d exceeds maximum allowed size %d", fileSize, s.config.MaxFileSize)
	}

	// 计算文件哈希
	fileHash := fmt.Sprintf("%x", hasher.Sum(nil))
	logger.Infof("Calculated hash for file %s: %s", fileName, fileHash)

	// 检查是否已存在相同哈希的文件（去重功能）
	var existingFile database.FileMetadata
	if err := s.db.Where("file_hash = ?", fileHash).First(&existingFile).Error; err == nil {
		// 文件已存在，返回现有文件信息
		logger.Infof("File with hash %s already exists, returning existing file: %s", fileHash, existingFile.FileID)
		return &existingFile, nil
	}

	// 将临时文件移动到最终位置
	logger.Infof("Moving temp file to storage path: %s", storagePath)
	if err := s.moveFile(tempFile.Name(), storagePath); err != nil {
		logger.Errorf("Failed to move file %s to storage: %v", fileName, err)
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

	logger.Infof("Saving file metadata to database for file: %s", fileName)
	if err := s.db.Create(metadata).Error; err != nil {
		// 如果数据库操作失败，删除已上传的文件
		logger.Errorf("Failed to save metadata for file %s, cleaning up: %v", fileName, err)
		os.Remove(storagePath)
		return nil, fmt.Errorf("failed to save file metadata: %w", err)
	}

	logger.Infof("File upload completed successfully: %s (ID: %s)", fileName, fileID)
	return metadata, nil
}

// GetFileByID 根据文件ID获取文件信息
func (s *fileService) GetFileByID(fileID string) (*database.FileMetadata, error) {
	var metadata database.FileMetadata
	if err := s.db.Where("file_id = ?", fileID).First(&metadata).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Errorf("[文件服务] 文件不存在, 文件ID: %s", fileID)
			return nil, fmt.Errorf("file not found with id: %s", fileID)
		}
		logger.Errorf("[文件服务] 查询文件元数据失败, 文件ID: %s, 错误: %v", fileID, err)
		return nil, err
	}
	logger.Infof("[文件服务] 成功获取文件元数据, 文件ID: %s, 文件名: %s", fileID, metadata.FileName)
	return &metadata, nil
}

// GetFileContent 根据文件ID获取文件内容
func (s *fileService) GetFileContent(fileID string) (io.ReadCloser, error) {
	metadata, err := s.GetFileByID(fileID)
	if err != nil {
		return nil, err
	}

	// 检查文件是否存在
	if _, osErr := os.Stat(metadata.StoragePath); os.IsNotExist(osErr) {
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

// UpdateFile 更新指定ID的文件内容
// 支持更新文件内容，自动处理文件去重和版本管理
func (s *fileService) UpdateFile(fileID string, fileData io.Reader) (*database.FileMetadata, error) {
	logger.Infof("[文件服务] 开始更新文件, 文件ID: %s", fileID)

	// 获取现有文件信息
	metadata, err := s.GetFileByID(fileID)
	if err != nil {
		logger.Errorf("[文件服务] 获取原始文件失败, 文件ID: %s, 错误: %v", fileID, err)
		return nil, err
	}

	logger.Infof("[文件服务] 原始文件信息 - 名称: %s, 大小: %d, 哈希: %s, 路径: %s",
		metadata.FileName, metadata.FileSize, metadata.FileHash, metadata.StoragePath)

	// 创建临时文件
	tempFile, err := os.CreateTemp("", "update_*")
	if err != nil {
		logger.Errorf("[文件服务] 创建临时文件失败, 文件ID: %s, 错误: %v", fileID, err)
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	logger.Infof("[文件服务] 创建临时更新文件: %s", tempFile.Name())

	// 将新数据写入临时文件并计算哈希
	hasher := sha256.New()
	multiWriter := io.MultiWriter(tempFile, hasher)

	fileSize, err := io.Copy(multiWriter, fileData)
	if err != nil {
		logger.Errorf("[文件服务] 复制新文件数据失败, 文件ID: %s, 错误: %v", fileID, err)
		return nil, fmt.Errorf("failed to copy file data: %w", err)
	}

	logger.Infof("[文件服务] 新文件大小: %d 字节", fileSize)

	// 检查文件大小
	if fileSize > s.config.MaxFileSize {
		logger.Errorf("[文件服务] 新文件大小 %d 超过允许的最大大小 %d, 文件ID: %s",
			fileSize, s.config.MaxFileSize, fileID)
		return nil, fmt.Errorf("file size %d exceeds maximum allowed size %d", fileSize, s.config.MaxFileSize)
	}

	// 计算新文件哈希
	newFileHash := fmt.Sprintf("%x", hasher.Sum(nil))
	logger.Infof("[文件服务] 新文件哈希: %s", newFileHash)

	// 如果哈希相同，说明文件内容未变化
	if newFileHash == metadata.FileHash {
		logger.Infof("[文件服务] 文件内容未变化（哈希相同）, 返回现有元数据, 文件ID: %s", fileID)
		return metadata, nil
	}

	// 备份原文件
	backupPath := metadata.StoragePath + ".backup"
	logger.Infof("[文件服务] 创建原始文件备份: %s -> %s", metadata.StoragePath, backupPath)
	if osErr := s.moveFile(metadata.StoragePath, backupPath); osErr != nil {
		logger.Errorf("[文件服务] 备份原始文件失败, 文件路径: %s, 错误: %v", metadata.StoragePath, osErr)
		return nil, fmt.Errorf("failed to backup original file: %w", osErr)
	}

	// 将新文件移动到原位置
	logger.Infof("[文件服务] 将新文件移动到原始位置: %s -> %s", tempFile.Name(), metadata.StoragePath)
	if osErr := s.moveFile(tempFile.Name(), metadata.StoragePath); osErr != nil {
		// 恢复备份文件
		logger.Errorf("[文件服务] 移动新文件失败, 正在恢复备份: %v", osErr)
		s.moveFile(backupPath, metadata.StoragePath)
		return nil, fmt.Errorf("failed to move new file: %w", osErr)
	}

	// 更新数据库记录
	updates := map[string]interface{}{
		"file_size":    fileSize,
		"file_hash":    newFileHash,
		"modify_count": gorm.Expr("modify_count + 1"),
		"updated_at":   time.Now(),
	}

	logger.Infof("[文件服务] 在数据库中更新文件元数据, 文件ID: %s", fileID)
	if osErr := s.db.Model(metadata).Updates(updates).Error; osErr != nil {
		// 恢复备份文件
		logger.Errorf("[文件服务] 更新文件元数据失败, 正在恢复备份, 文件ID: %s, 错误: %v", fileID, osErr)
		s.moveFile(backupPath, metadata.StoragePath)
		return nil, fmt.Errorf("failed to update file metadata: %w", osErr)
	}

	// 删除备份文件
	logger.Infof("[文件服务] 删除备份文件: %s", backupPath)
	os.Remove(backupPath)

	// 重新获取更新后的数据
	updatedMetadata, err := s.GetFileByID(fileID)
	if err != nil {
		logger.Errorf("[文件服务] 获取更新后的元数据失败, 文件ID: %s, 错误: %v", fileID, err)
		return nil, err
	}

	logger.Infof("[文件服务] 文件更新成功: %s (新大小: %d, 修改次数: %d)",
		fileID, updatedMetadata.FileSize, updatedMetadata.ModifyCount)
	return updatedMetadata, nil
}

// DeleteFile 删除指定ID的文件
// 包括删除物理文件、数据库记录和云端文件（如果配置了OSS同步）
func (s *fileService) DeleteFile(fileID string) error {
	logger.Infof("[文件服务] 开始删除文件, 文件ID: %s", fileID)

	metadata, err := s.GetFileByID(fileID)
	if err != nil {
		logger.Errorf("[文件服务] 获取文件元数据失败, 文件ID: %s, 错误: %v", fileID, err)
		return err
	}

	logger.Infof("[文件服务] 找到待删除文件: %s (文件名: %s, 路径: %s)", fileID, metadata.FileName, metadata.StoragePath)

	// 如果设置了OSS同步服务，尝试删除云端文件
	if s.ossSyncService != nil {
		logger.Infof("[文件服务] 尝试从OSS删除文件: %s", fileID)
		// 先尝试获取该文件的同步日志，找到对应的OSS路径
		var syncLog database.SyncLog
		if err := s.db.Where("file_id = ? AND status = ?", fileID, "success").
			Order("created_at DESC").First(&syncLog).Error; err == nil {
			logger.Infof("[文件服务] 找到文件同步日志, 文件ID: %s, OSS路径: %s", fileID, syncLog.OSSPath)
			// 有成功的同步记录，尝试从OSS删除
			// 注意：OSS文件删除应该通过OSS同步服务来处理
			// 这里只记录日志，实际删除由OSS同步服务负责
			logger.Infof("[文件服务] 文件 %s 存在OSS同步记录, OSS清理应由同步服务处理", fileID)
		} else {
			logger.Infof("[文件服务] 未找到文件同步日志, 跳过OSS删除: %s", fileID)
		}
	} else {
		logger.Infof("[文件服务] 未配置OSS同步服务, 跳过云端删除: %s", fileID)
	}

	// 删除数据库记录（软删除）
	logger.Infof("[文件服务] 从数据库删除文件记录: %s", fileID)
	if err := s.db.Delete(metadata).Error; err != nil {
		logger.Errorf("[文件服务] 从数据库删除文件记录失败, 文件ID: %s, 错误: %v", fileID, err)
		return fmt.Errorf("failed to delete file metadata: %w", err)
	}

	logger.Infof("[文件服务] 成功从数据库删除文件记录: %s", fileID)

	// 删除物理文件
	logger.Infof("[文件服务] 删除物理文件: %s", metadata.StoragePath)
	if err := os.Remove(metadata.StoragePath); err != nil && !os.IsNotExist(err) {
		// 如果删除物理文件失败，恢复数据库记录
		logger.Errorf("[文件服务] 删除物理文件失败, 尝试恢复数据库记录, 文件路径: %s, 错误: %v", metadata.StoragePath, err)
		s.db.Model(metadata).Update("deleted_at", nil)
		return fmt.Errorf("failed to delete physical file: %w", err)
	} else if err == nil {
		logger.Infof("[文件服务] 成功删除物理文件: %s", metadata.StoragePath)
	} else {
		logger.Infof("[文件服务] 物理文件 %s 不存在, 跳过删除", metadata.StoragePath)
	}

	logger.Infof("[文件服务] 文件删除成功, 文件ID: %s", fileID)
	return nil
}

// ListFiles 获取文件列表（分页）
// 支持分页查询，按创建时间倒序排列
func (s *fileService) ListFiles(page, pageSize int) ([]database.FileMetadata, int64, error) {
	logger.Infof("[文件服务] 获取文件列表 - 页码: %d, 每页数量: %d", page, pageSize)

	var files []database.FileMetadata
	var total int64

	// 获取总数
	if err := s.db.Model(&database.FileMetadata{}).Count(&total).Error; err != nil {
		logger.Errorf("[文件服务] 计算文件总数失败: %v", err)
		return nil, 0, err
	}

	logger.Infof("[文件服务] 找到的文件总数: %d", total)

	// 分页查询
	offset := (page - 1) * pageSize
	logger.Infof("[文件服务] 计算偏移量: %d", offset)

	if err := s.db.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&files).Error; err != nil {
		logger.Errorf("[文件服务] 获取文件列表失败: %v", err)
		return nil, 0, err
	}

	logger.Infof("[文件服务] 成功获取第%d页的%d个文件", page, len(files))
	return files, total, nil
}

// SearchFilesByName 根据文件名搜索文件
// 支持模糊匹配和分页查询
func (s *fileService) SearchFilesByName(fileName string, page, pageSize int) ([]database.FileMetadata, int64, error) {
	logger.Infof("[文件服务] 根据文件名搜索文件: '%s', 页码: %d, 每页数量: %d", fileName, page, pageSize)

	var files []database.FileMetadata
	var total int64

	searchQuery := "%" + fileName + "%"
	logger.Infof("[文件服务] 搜索查询模式: %s", searchQuery)

	// 获取总数
	if err := s.db.Model(&database.FileMetadata{}).Where("file_name LIKE ?", searchQuery).Count(&total).Error; err != nil {
		logger.Errorf("[文件服务] 计算文件名 '%s' 的搜索结果总数失败: %v", fileName, err)
		return nil, 0, err
	}

	logger.Infof("[文件服务] 找到匹配文件名 '%s' 的%d个文件", fileName, total)

	// 分页查询
	offset := (page - 1) * pageSize
	logger.Infof("[文件服务] 搜索偏移量: %d", offset)

	if err := s.db.Where("file_name LIKE ?", searchQuery).Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&files).Error; err != nil {
		logger.Errorf("[文件服务] 根据文件名 '%s' 搜索文件失败: %v", fileName, err)
		return nil, 0, err
	}

	logger.Infof("[文件服务] 成功获取文件名 '%s' 第%d页的%d个搜索结果", fileName, page, len(files))
	return files, total, nil
}

// IncrementViewCount 增加查看次数
// 用于统计文件的访问频率
func (s *fileService) IncrementViewCount(fileID string) error {
	logger.Infof("[文件服务] 增加文件查看次数: %s", fileID)

	err := s.db.Model(&database.FileMetadata{}).Where("file_id = ?", fileID).Update("view_count", gorm.Expr("view_count + 1")).Error
	if err != nil {
		logger.Errorf("[文件服务] 增加文件 %s 的查看次数失败: %v", fileID, err)
		return err
	}

	logger.Infof("[文件服务] 成功增加文件查看次数: %s", fileID)
	return nil
}

// GetFileStats 获取文件统计信息
// 返回系统中文件的详细统计数据，包括总数、大小、访问次数和格式分布
func (s *fileService) GetFileStats() (map[string]interface{}, error) {
	logger.Infof("[文件服务] 获取文件统计信息")

	var stats struct {
		TotalFiles int64 `json:"total_files"`
		TotalSize  int64 `json:"total_size"`
		TotalViews int64 `json:"total_views"`
	}

	// 统计文件数量和总大小
	if err := s.db.Model(&database.FileMetadata{}).
		Select("COUNT(*) as total_files, COALESCE(SUM(file_size), 0) as total_size, COALESCE(SUM(view_count), 0) as total_views").
		Scan(&stats).Error; err != nil {
		logger.Errorf("[文件服务] 获取基本文件统计信息失败: %v", err)
		return nil, err
	}

	logger.Infof("[文件服务] 基本统计 - 文件数: %d, 总大小: %d bytes, 总查看次数: %d",
		stats.TotalFiles, stats.TotalSize, stats.TotalViews)

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
		logger.Errorf("[文件服务] 获取格式统计信息失败: %v", err)
		return nil, err
	}

	logger.Infof("[文件服务] 找到 %d 种不同的文件格式", len(formatStats))
	for _, stat := range formatStats {
		logger.Infof("[文件服务] 格式 %s: %d 个文件", stat.FileFormat, stat.Count)
	}

	return map[string]interface{}{
		"total_files":  stats.TotalFiles,
		"total_size":   stats.TotalSize,
		"total_views":  stats.TotalViews,
		"format_stats": formatStats,
	}, nil
}

// isAllowedExtension 检查文件扩展名是否允许
// 根据配置的允许扩展名列表验证文件类型
func (s *fileService) isAllowedExtension(ext string) bool {
	logger.Debugf("[文件服务] 检查扩展名 '%s' 是否允许", ext)

	// 如果配置为允许所有扩展名
	for _, allowed := range s.config.AllowedExtensions {
		if allowed == "*" {
			logger.Debugf("[文件服务] 允许所有扩展名 (找到通配符)")
			return true
		}
		if strings.EqualFold(allowed, ext) {
			logger.Debugf("[文件服务] 扩展名 '%s' 被允许 (匹配: %s)", ext, allowed)
			return true
		}
	}

	logger.Debugf("[文件服务] 扩展名 '%s' 不被允许。允许的扩展名: %v", ext, s.config.AllowedExtensions)
	return false
}

// moveFile 移动文件
// 优先使用重命名操作，如果失败则使用复制+删除的方式
func (s *fileService) moveFile(src, dst string) error {
	logger.Infof("[文件服务] 移动文件: %s -> %s", src, dst)

	// 确保目标目录存在
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		logger.Errorf("[文件服务] 创建目标目录失败 %s: %v", filepath.Dir(dst), err)
		return err
	}

	// 尝试重命名文件（同一文件系统）
	if err := os.Rename(src, dst); err == nil {
		logger.Infof("[文件服务] 使用重命名成功移动文件: %s -> %s", src, dst)
		return nil
	} else {
		logger.Debugf("[文件服务] 重命名失败，使用复制+删除方式: %v", err)
	}

	// 如果重命名失败，则复制文件
	srcFile, err := os.Open(src)
	if err != nil {
		logger.Errorf("[文件服务] 打开源文件失败 %s: %v", src, err)
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		logger.Errorf("[文件服务] 创建目标文件失败 %s: %v", dst, err)
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		logger.Errorf("[文件服务] 复制文件内容失败 %s -> %s: %v", src, dst, err)
		return err
	}

	// 复制成功后删除源文件
	if err := os.Remove(src); err != nil {
		logger.Errorf("[文件服务] 复制后删除源文件失败 %s: %v", src, err)
		return err
	}

	logger.Infof("[文件服务] 使用复制+删除方式成功移动文件: %s -> %s", src, dst)
	return nil
}

// SetOSSSyncService 设置OSS同步服务
// 用于在文件删除时同步删除云端文件
// OSSyncService 定义了OSS同步服务的接口
type OSSyncService interface {
	// 在这里定义OSS同步服务需要的方法
}

func (s *fileService) SetOSSSyncService(syncService OSSyncService) {
	logger.Infof("[文件服务] 设置OSS同步服务")
	s.ossSyncService = syncService
	logger.Infof("[文件服务] OSS同步服务设置成功")
}
