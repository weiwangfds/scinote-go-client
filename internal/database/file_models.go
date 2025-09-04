// Package database 定义了文件相关的数据库模型
// 包含文件元数据等核心数据模型
package database

import (
	"time"

	"gorm.io/gorm"
)

// FileMetadata 文件元数据模型
// 用于存储上传文件的基本信息和统计数据
// 支持文件的唯一标识、存储路径、大小、格式等核心属性
type FileMetadata struct {
	ID          uint           `gorm:"primarykey" json:"id"`                        // 主键ID，自增
	FileID      string         `gorm:"uniqueIndex;not null;size:36" json:"file_id"` // 文件唯一标识符（UUID格式）
	FileName    string         `gorm:"not null;size:255" json:"file_name"`          // 原始文件名称，最大255字符
	StoragePath string         `gorm:"not null;size:500" json:"storage_path"`       // 文件在存储系统中的完整路径
	FileSize    int64          `gorm:"not null" json:"file_size"`                   // 文件大小，单位为字节
	FileHash    string         `gorm:"not null;size:64" json:"file_hash"`           // 文件内容的SHA256哈希值，用于去重和完整性校验
	FileFormat  string         `gorm:"not null;size:50" json:"file_format"`         // 文件格式/扩展名（如：pdf、jpg、txt等）
	ViewCount   int64          `gorm:"default:0" json:"view_count"`                 // 文件被查看的次数统计
	ModifyCount int64          `gorm:"default:0" json:"modify_count"`               // 文件被修改的次数统计
	CreatedAt   time.Time      `json:"created_at"`                                  // 记录创建时间
	UpdatedAt   time.Time      `json:"updated_at"`                                  // 记录最后更新时间
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`                              // 软删除时间戳，支持逻辑删除
}

// TableName 指定FileMetadata模型对应的数据库表名
// 返回值: "file_metadata" - 数据库中的表名
// 用途: GORM框架通过此方法确定模型对应的数据库表
func (FileMetadata) TableName() string {
	return "file_metadata"
}