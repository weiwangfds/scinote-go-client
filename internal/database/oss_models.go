// Package database 定义了OSS相关的数据库模型
// 包含OSS配置和同步日志等核心数据模型
package database

import (
	"time"

	"gorm.io/gorm"
)

// OSSConfig 对象存储服务配置模型
// 用于管理不同云服务商的OSS配置信息，支持阿里云、腾讯云、七牛云等
// 包含连接认证、同步设置、状态管理等完整配置项
type OSSConfig struct {
	ID            uint           `gorm:"primarykey" json:"id"`                          // 主键ID，自增
	Name          string         `gorm:"not null;size:100" json:"name"`                 // 配置名称，用于标识不同的OSS配置
	Provider      string         `gorm:"not null;size:20" json:"provider"`              // OSS服务提供商：aliyun（阿里云）、tencent（腾讯云）、qiniu（七牛云）
	Region        string         `gorm:"not null;size:50" json:"region"`                // 服务区域，如：cn-hangzhou、ap-beijing等
	Bucket        string         `gorm:"not null;size:100" json:"bucket"`               // 存储桶名称，OSS中的容器名称
	AccessKey     string         `gorm:"not null;size:100" json:"access_key"`           // 访问密钥ID，用于API认证
	SecretKey     string         `gorm:"not null;size:200" json:"secret_key,omitempty"` // 访问密钥Secret，敏感信息，API响应时不返回
	Endpoint      string         `gorm:"size:200" json:"endpoint"`                      // 自定义服务端点URL，可选配置
	IsActive      bool           `gorm:"default:false" json:"is_active"`                // 是否为当前激活使用的配置，系统中只能有一个激活配置
	IsEnabled     bool           `gorm:"default:true" json:"is_enabled"`                // 配置是否启用，禁用后不可使用
	AutoSync      bool           `gorm:"default:false" json:"auto_sync"`                // 是否开启文件自动同步功能
	SyncPath      string         `gorm:"size:200;default:'files'" json:"sync_path"`     // OSS中的同步路径前缀，默认为"files"
	KeepStructure bool           `gorm:"default:true" json:"keep_structure"`            // 同步时是否保持本地文件目录结构
	CreatedAt     time.Time      `json:"created_at"`                                    // 配置创建时间
	UpdatedAt     time.Time      `json:"updated_at"`                                    // 配置最后修改时间
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`                                // 软删除时间戳，支持逻辑删除
}

// TableName 指定OSSConfig模型对应的数据库表名
// 返回值: "oss_configs" - 数据库中的表名
// 用途: GORM框架通过此方法确定模型对应的数据库表
func (OSSConfig) TableName() string {
	return "oss_configs"
}

// SyncLog 文件同步日志模型
// 记录文件与OSS之间的同步操作历史，包括上传、下载等操作的详细信息
// 用于追踪同步状态、性能分析和错误排查
type SyncLog struct {
	ID          uint           `gorm:"primarykey" json:"id"`                               // 主键ID，自增
	FileID      string         `gorm:"not null;size:36" json:"file_id"`                    // 关联的文件ID（UUID格式）
	OSSConfigID uint           `gorm:"not null" json:"oss_config_id"`                      // 关联的OSS配置ID
	OSSConfig   OSSConfig      `gorm:"foreignKey:OSSConfigID" json:"oss_config,omitempty"` // 关联的OSS配置对象，外键关联
	SyncType    string         `gorm:"not null;size:20" json:"sync_type"`                  // 同步操作类型：upload（上传）、download（下载）
	Status      string         `gorm:"not null;size:20" json:"status"`                     // 同步状态：pending（待处理）、success（成功）、failed（失败）
	OSSPath     string         `gorm:"size:500" json:"oss_path"`                           // 文件在OSS中的完整路径
	ErrorMsg    string         `gorm:"type:text" json:"error_msg"`                         // 同步失败时的详细错误信息
	FileSize    int64          `json:"file_size"`                                          // 同步文件的大小，单位为字节
	Duration    int64          `json:"duration"`                                           // 同步操作耗时，单位为毫秒
	CreatedAt   time.Time      `json:"created_at"`                                         // 同步日志创建时间
	UpdatedAt   time.Time      `json:"updated_at"`                                         // 同步日志最后更新时间
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`                                     // 软删除时间戳，支持逻辑删除
}

// TableName 指定SyncLog模型对应的数据库表名
// 返回值: "sync_logs" - 数据库中的表名
// 用途: GORM框架通过此方法确定模型对应的数据库表
func (SyncLog) TableName() string {
	return "sync_logs"
}