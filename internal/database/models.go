package database

import (
	"time"

	"gorm.io/gorm"
)

// FileMetadata 文件元数据模型
type FileMetadata struct {
	ID          uint           `gorm:"primarykey" json:"id"`
	FileID      string         `gorm:"uniqueIndex;not null;size:36" json:"file_id"` // UUID
	FileName    string         `gorm:"not null;size:255" json:"file_name"`          // 文件名称
	StoragePath string         `gorm:"not null;size:500" json:"storage_path"`       // 存储地址
	FileSize    int64          `gorm:"not null" json:"file_size"`                   // 文件大小（字节）
	FileHash    string         `gorm:"not null;size:64" json:"file_hash"`           // 文件Hash值（SHA256）
	FileFormat  string         `gorm:"not null;size:50" json:"file_format"`         // 文件格式/扩展名
	ViewCount   int64          `gorm:"default:0" json:"view_count"`                 // 查看次数
	ModifyCount int64          `gorm:"default:0" json:"modify_count"`               // 修改次数
	CreatedAt   time.Time      `json:"created_at"`                                  // 创建时间
	UpdatedAt   time.Time      `json:"updated_at"`                                  // 修改时间
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`                              // 软删除
}

// TableName 指定表名
func (FileMetadata) TableName() string {
	return "file_metadata"
}

// OSSConfig OSS配置模型
type OSSConfig struct {
	ID            uint           `gorm:"primarykey" json:"id"`
	Name          string         `gorm:"not null;size:100" json:"name"`             // 配置名称
	Provider      string         `gorm:"not null;size:20" json:"provider"`          // OSS提供商：aliyun, tencent, qiniu
	Region        string         `gorm:"not null;size:50" json:"region"`            // 区域
	Bucket        string         `gorm:"not null;size:100" json:"bucket"`           // 存储桶名称
	AccessKey     string         `gorm:"not null;size:100" json:"access_key"`       // 访问密钥ID
	SecretKey     string         `gorm:"not null;size:200" json:"secret_key,omitempty"`                // 访问密钥Secret（不返回给前端）
	Endpoint      string         `gorm:"size:200" json:"endpoint"`                  // 自定义端点
	IsActive      bool           `gorm:"default:false" json:"is_active"`            // 是否为当前激活的配置
	IsEnabled     bool           `gorm:"default:true" json:"is_enabled"`            // 是否启用
	AutoSync      bool           `gorm:"default:false" json:"auto_sync"`            // 是否开启自动同步
	SyncPath      string         `gorm:"size:200;default:'files'" json:"sync_path"` // OSS同步路径前缀
	KeepStructure bool           `gorm:"default:true" json:"keep_structure"`        // 是否保持本地文件结构
	CreatedAt     time.Time      `json:"created_at"`                                // 创建时间
	UpdatedAt     time.Time      `json:"updated_at"`                                // 修改时间
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`                            // 软删除
}

// TableName 指定表名
func (OSSConfig) TableName() string {
	return "oss_configs"
}

// SyncLog 同步日志模型
type SyncLog struct {
	ID          uint           `gorm:"primarykey" json:"id"`
	FileID      string         `gorm:"not null;size:36" json:"file_id"` // 文件ID
	OSSConfigID uint           `gorm:"not null" json:"oss_config_id"`   // OSS配置ID
	OSSConfig   OSSConfig      `gorm:"foreignKey:OSSConfigID" json:"oss_config,omitempty"`
	SyncType    string         `gorm:"not null;size:20" json:"sync_type"` // 同步类型：upload, download
	Status      string         `gorm:"not null;size:20" json:"status"`    // 状态：pending, success, failed
	OSSPath     string         `gorm:"size:500" json:"oss_path"`          // OSS路径
	ErrorMsg    string         `gorm:"type:text" json:"error_msg"`        // 错误信息
	FileSize    int64          `json:"file_size"`                         // 文件大小
	Duration    int64          `json:"duration"`                          // 同步耗时（毫秒）
	CreatedAt   time.Time      `json:"created_at"`                        // 创建时间
	UpdatedAt   time.Time      `json:"updated_at"`                        // 修改时间
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`                    // 软删除
}

// TableName 指定表名
func (SyncLog) TableName() string {
	return "sync_logs"
}
