// Package database 定义了数据库相关的模型和结构体
// 包含文件元数据、OSS配置和同步日志等核心数据模型
package database

import (
	"fmt"
	"strconv"
	"strings"
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

// Note 笔记模型
// 支持无限层级的笔记组织结构，类似Notion的页面层级管理
// 笔记内容以文件形式存储在FileMetadata表中，此表仅保存元数据和层级关系
type Note struct {
	ID          uint           `gorm:"primarykey" json:"id"`                        // 主键ID，自增
	NoteID      string         `gorm:"uniqueIndex;not null;size:36" json:"note_id"` // 笔记唯一标识符（UUID格式）
	Title       string         `gorm:"not null;size:255" json:"title"`              // 笔记标题，最大255字符
	ParentID    *uint          `gorm:"index" json:"parent_id"`                     // 父笔记ID，支持无限层级结构，根笔记为null
	Parent      *Note          `gorm:"foreignKey:ParentID" json:"parent,omitempty"` // 父笔记对象，外键关联
	Children    []Note         `gorm:"foreignKey:ParentID" json:"children,omitempty"` // 子笔记列表，一对多关联
	FileID      *string        `gorm:"size:36;index" json:"file_id"`               // 关联的文件ID，指向FileMetadata表中的文件内容
	File        *FileMetadata  `gorm:"foreignKey:FileID;references:FileID" json:"file,omitempty"` // 关联的文件对象
	Type        string         `gorm:"not null;size:20;default:'page'" json:"type"` // 笔记类型：page（页面）、database（数据库）、text（文本）等
	Icon        string         `gorm:"size:100" json:"icon"`                       // 笔记图标，可以是emoji或图标名称
	Cover       string         `gorm:"size:500" json:"cover"`                      // 封面图片URL或路径
	IsPublic    bool           `gorm:"default:false" json:"is_public"`             // 是否公开可见
	IsArchived  bool           `gorm:"default:false" json:"is_archived"`           // 是否已归档
	IsFavorite  bool           `gorm:"default:false" json:"is_favorite"`           // 是否收藏
	SortOrder   int            `gorm:"default:0" json:"sort_order"`                // 在同级笔记中的排序顺序
	Path        string         `gorm:"size:1000;index" json:"path"`                // 笔记的完整路径，用于快速层级查询，格式如：/1/2/3
	Level       int            `gorm:"default:0;index" json:"level"`               // 笔记层级深度，根笔记为0
	CreatorID   string         `gorm:"size:36" json:"creator_id"`                  // 创建者ID
	UpdaterID   string         `gorm:"size:36" json:"updater_id"`                  // 最后更新者ID
	ViewCount   int64          `gorm:"default:0" json:"view_count"`                // 笔记被查看次数
	CreatedAt   time.Time      `json:"created_at"`                                 // 创建时间
	UpdatedAt   time.Time      `json:"updated_at"`                                 // 最后更新时间
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`                             // 软删除时间戳

	// 关联关系
	Tags       []Tag           `gorm:"many2many:note_tags;" json:"tags,omitempty"`       // 多对多关联标签
	Properties []NoteProperty `gorm:"foreignKey:NoteID" json:"properties,omitempty"` // 一对多关联扩展属性
}

// TableName 指定Note模型对应的数据库表名
func (Note) TableName() string {
	return "notes"
}

// Tag 标签模型
// 用于笔记的分类和标记，支持多标签系统
type Tag struct {
	ID          uint           `gorm:"primarykey" json:"id"`                      // 主键ID，自增
	TagID       string         `gorm:"uniqueIndex;not null;size:36" json:"tag_id"` // 标签唯一标识符（UUID格式）
	Name        string         `gorm:"uniqueIndex;not null;size:100" json:"name"` // 标签名称，唯一索引
	Color       string         `gorm:"size:20;default:'#gray'" json:"color"`      // 标签颜色，支持预定义颜色或十六进制色值
	Description string         `gorm:"size:500" json:"description"`              // 标签描述
	UsageCount  int64          `gorm:"default:0" json:"usage_count"`             // 标签使用次数统计
	CreatedAt   time.Time      `json:"created_at"`                               // 创建时间
	UpdatedAt   time.Time      `json:"updated_at"`                               // 最后更新时间
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`                           // 软删除时间戳

	// 关联关系
	Notes []Note `gorm:"many2many:note_tags;" json:"notes,omitempty"` // 多对多关联笔记
}

// TableName 指定Tag模型对应的数据库表名
func (Tag) TableName() string {
	return "tags"
}

// NoteTag 笔记标签关联模型
// 处理笔记与标签的多对多关系，支持关联时间等额外信息
type NoteTag struct {
	ID        uint      `gorm:"primarykey" json:"id"`        // 主键ID，自增
	NoteID    uint      `gorm:"not null;index" json:"note_id"` // 笔记ID，外键
	TagID     uint      `gorm:"not null;index" json:"tag_id"`  // 标签ID，外键
	Note      Note      `gorm:"foreignKey:NoteID" json:"note,omitempty"` // 关联的笔记对象
	Tag       Tag       `gorm:"foreignKey:TagID" json:"tag,omitempty"`   // 关联的标签对象
	CreatedAt time.Time `json:"created_at"`                   // 关联创建时间
}

// TableName 指定NoteTag模型对应的数据库表名
func (NoteTag) TableName() string {
	return "note_tags"
}

// NoteProperty 笔记扩展属性模型
// 用于存储笔记的自定义属性和元数据，支持灵活的扩展字段
type NoteProperty struct {
	ID           uint           `gorm:"primarykey" json:"id"`                         // 主键ID，自增
	NoteID       uint           `gorm:"not null;index" json:"note_id"`                // 关联的笔记ID，外键
	Note         Note           `gorm:"foreignKey:NoteID" json:"note,omitempty"`      // 关联的笔记对象
	PropertyKey  string         `gorm:"not null;size:100" json:"property_key"`        // 属性键名，如：priority、status、due_date等
	PropertyType string         `gorm:"not null;size:20" json:"property_type"`        // 属性类型：text、number、date、boolean、select、multi_select等
	TextValue    string         `gorm:"type:text" json:"text_value"`                  // 文本类型值
	NumberValue  *float64       `json:"number_value"`                               // 数字类型值
	DateValue    *time.Time     `json:"date_value"`                                 // 日期类型值
	BoolValue    *bool          `json:"bool_value"`                                 // 布尔类型值
	JSONValue    string         `gorm:"type:json" json:"json_value"`                 // JSON类型值，用于复杂数据结构
	CreatedAt    time.Time      `json:"created_at"`                                 // 创建时间
	UpdatedAt    time.Time      `json:"updated_at"`                                 // 最后更新时间
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`                             // 软删除时间戳
}

// TableName 指定NoteProperty模型对应的数据库表名
func (NoteProperty) TableName() string {
	return "note_properties"
}

// Note模型的辅助方法

// IsRoot 判断是否为根笔记
// 返回值: bool - true表示为根笔记（无父笔记），false表示有父笔记
func (n *Note) IsRoot() bool {
	return n.ParentID == nil
}

// HasChildren 判断是否有子笔记
// 返回值: bool - true表示有子笔记，false表示无子笔记
func (n *Note) HasChildren() bool {
	return len(n.Children) > 0
}

// BuildPath 构建笔记的完整路径
// 参数: parentPath string - 父笔记的路径
// 返回值: string - 完整的路径字符串，格式如：/1/2/3
// 用途: 在创建或移动笔记时自动构建路径，便于层级查询
func (n *Note) BuildPath(parentPath string) string {
	if parentPath == "" {
		return fmt.Sprintf("/%d", n.ID)
	}
	return fmt.Sprintf("%s/%d", parentPath, n.ID)
}

// CalculateLevel 根据路径计算层级深度
// 返回值: int - 层级深度，根笔记为0
// 用途: 在创建或移动笔记时自动计算层级深度
func (n *Note) CalculateLevel() int {
	if n.Path == "" {
		return 0
	}
	// 路径格式为 /1/2/3，通过统计斜杠数量计算层级
	return len(strings.Split(strings.Trim(n.Path, "/"), "/"))
}

// GetAncestorIDs 从路径中提取所有祖先笔记的ID
// 返回值: []uint - 祖先笔记ID列表，按层级从根到父的顺序
// 用途: 用于权限检查、面包屑导航等场景
func (n *Note) GetAncestorIDs() []uint {
	if n.Path == "" {
		return []uint{}
	}
	
	pathParts := strings.Split(strings.Trim(n.Path, "/"), "/")
	ancestorIDs := make([]uint, 0, len(pathParts)-1)
	
	// 排除最后一个ID（自己的ID）
	for i := 0; i < len(pathParts)-1; i++ {
		if id, err := strconv.ParseUint(pathParts[i], 10, 32); err == nil {
			ancestorIDs = append(ancestorIDs, uint(id))
		}
	}
	
	return ancestorIDs
}

// Tag模型的辅助方法

// IncrementUsage 增加标签使用次数
// 用途: 在笔记添加标签时调用，用于统计标签的使用频率
func (t *Tag) IncrementUsage() {
	t.UsageCount++
}

// DecrementUsage 减少标签使用次数
// 用途: 在笔记移除标签时调用，确保使用次数不会小于0
func (t *Tag) DecrementUsage() {
	if t.UsageCount > 0 {
		t.UsageCount--
	}
}

// NoteProperty模型的辅助方法

// GetValue 根据属性类型获取对应的值
// 返回值: interface{} - 根据PropertyType返回对应类型的值
// 用途: 统一的值获取接口，避免调用方需要判断类型
func (np *NoteProperty) GetValue() interface{} {
	switch np.PropertyType {
	case "text":
		return np.TextValue
	case "number":
		return np.NumberValue
	case "date":
		return np.DateValue
	case "boolean":
		return np.BoolValue
	case "json":
		return np.JSONValue
	default:
		return np.TextValue
	}
}

// SetValue 根据属性类型设置对应的值
// 参数: value interface{} - 要设置的值
// 返回值: error - 设置失败时返回错误
// 用途: 统一的值设置接口，自动根据类型设置到对应字段
func (np *NoteProperty) SetValue(value interface{}) error {
	// 清空所有值字段
	np.TextValue = ""
	np.NumberValue = nil
	np.DateValue = nil
	np.BoolValue = nil
	np.JSONValue = ""
	
	switch np.PropertyType {
	case "text":
		if v, ok := value.(string); ok {
			np.TextValue = v
		} else {
			return fmt.Errorf("invalid value type for text property")
		}
	case "number":
		if v, ok := value.(float64); ok {
			np.NumberValue = &v
		} else {
			return fmt.Errorf("invalid value type for number property")
		}
	case "date":
		if v, ok := value.(time.Time); ok {
			np.DateValue = &v
		} else {
			return fmt.Errorf("invalid value type for date property")
		}
	case "boolean":
		if v, ok := value.(bool); ok {
			np.BoolValue = &v
		} else {
			return fmt.Errorf("invalid value type for boolean property")
		}
	case "json":
		if v, ok := value.(string); ok {
			np.JSONValue = v
		} else {
			return fmt.Errorf("invalid value type for json property")
		}
	default:
		return fmt.Errorf("unsupported property type: %s", np.PropertyType)
	}
	
	return nil
}
