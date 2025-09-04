package database

import (
	"time"

	"gorm.io/gorm"
)

// Note 笔记模型
// 用于存储用户创建的笔记内容，支持富文本、标签分类、属性扩展等功能
// 提供完整的笔记管理能力，包括内容存储、分类管理、搜索索引等
type Note struct {
	ID          uint           `gorm:"primarykey" json:"id"`                    // 主键ID，自增
	Title       string         `gorm:"not null;size:200" json:"title"`          // 笔记标题，必填，最大200字符
	Content     string         `gorm:"type:longtext" json:"content"`            // 笔记内容，支持富文本，使用longtext存储大量文本
	Summary     string         `gorm:"size:500" json:"summary"`                 // 笔记摘要，用于快速预览，最大500字符
	Author      string         `gorm:"size:100" json:"author"`                  // 笔记作者，可选字段
	Category    string         `gorm:"size:50" json:"category"`                 // 笔记分类，用于组织管理
	IsPublic    bool           `gorm:"default:false" json:"is_public"`          // 是否公开，默认私有
	IsArchived  bool           `gorm:"default:false" json:"is_archived"`        // 是否已归档，归档后不在常规列表中显示
	ViewCount   int            `gorm:"default:0" json:"view_count"`             // 查看次数统计
	LikeCount   int            `gorm:"default:0" json:"like_count"`             // 点赞次数统计
	WordCount   int            `gorm:"default:0" json:"word_count"`             // 字数统计，用于内容分析
	ReadingTime int            `gorm:"default:0" json:"reading_time"`           // 预估阅读时间（分钟），基于字数计算
	CreatedAt   time.Time      `json:"created_at"`                             // 笔记创建时间
	UpdatedAt   time.Time      `json:"updated_at"`                             // 笔记最后修改时间
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`                         // 软删除时间戳，支持逻辑删除

	// 关联关系
	Tags       []Tag          `gorm:"many2many:note_tags;" json:"tags,omitempty"`       // 多对多关联标签
	Properties []NoteProperty `gorm:"foreignKey:NoteID" json:"properties,omitempty"`   // 一对多关联属性
}

// TableName 指定Note模型对应的数据库表名
// 返回值: "notes" - 数据库中的表名
// 用途: GORM框架通过此方法确定模型对应的数据库表
func (Note) TableName() string {
	return "notes"
}

// Tag 标签模型
// 用于对笔记进行分类和标记，支持层级结构、颜色标识等功能
// 提供灵活的标签管理系统，便于内容组织和快速检索
type Tag struct {
	ID          uint           `gorm:"primarykey" json:"id"`                    // 主键ID，自增
	Name        string         `gorm:"not null;uniqueIndex;size:50" json:"name"` // 标签名称，必填且唯一，最大50字符
	Description string         `gorm:"size:200" json:"description"`             // 标签描述，可选，最大200字符
	Color       string         `gorm:"size:7;default:'#007bff'" json:"color"`   // 标签颜色，十六进制格式，默认蓝色
	ParentID    *uint          `gorm:"index" json:"parent_id"`                  // 父标签ID，支持层级结构，可为空
	Parent      *Tag           `gorm:"foreignKey:ParentID" json:"parent,omitempty"` // 父标签对象，外键关联
	Children    []Tag          `gorm:"foreignKey:ParentID" json:"children,omitempty"` // 子标签列表，一对多关联
	SortOrder   int            `gorm:"default:0" json:"sort_order"`             // 排序顺序，用于标签显示排序
	IsActive    bool           `gorm:"default:true" json:"is_active"`           // 是否激活，非激活标签不可使用
	UsageCount  int            `gorm:"default:0" json:"usage_count"`            // 使用次数统计，用于热门标签分析
	CreatedAt   time.Time      `json:"created_at"`                             // 标签创建时间
	UpdatedAt   time.Time      `json:"updated_at"`                             // 标签最后修改时间
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`                         // 软删除时间戳，支持逻辑删除

	// 关联关系
	Notes []Note `gorm:"many2many:note_tags;" json:"notes,omitempty"` // 多对多关联笔记
}

// TableName 指定Tag模型对应的数据库表名
// 返回值: "tags" - 数据库中的表名
// 用途: GORM框架通过此方法确定模型对应的数据库表
func (Tag) TableName() string {
	return "tags"
}

// NoteTag 笔记标签关联模型
// 用于管理笔记与标签之间的多对多关系，支持关联时间记录等扩展功能
// 提供灵活的关联管理，便于统计分析和关系维护
type NoteTag struct {
	ID        uint           `gorm:"primarykey" json:"id"`        // 主键ID，自增
	NoteID    uint           `gorm:"not null;index" json:"note_id"` // 笔记ID，外键，必填
	TagID     uint           `gorm:"not null;index" json:"tag_id"`  // 标签ID，外键，必填
	Note      Note           `gorm:"foreignKey:NoteID" json:"note,omitempty"` // 关联的笔记对象
	Tag       Tag            `gorm:"foreignKey:TagID" json:"tag,omitempty"`   // 关联的标签对象
	CreatedAt time.Time      `json:"created_at"`                   // 关联创建时间
	UpdatedAt time.Time      `json:"updated_at"`                   // 关联最后修改时间
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`               // 软删除时间戳，支持逻辑删除
}

// TableName 指定NoteTag模型对应的数据库表名
// 返回值: "note_tags" - 数据库中的表名
// 用途: GORM框架通过此方法确定模型对应的数据库表
func (NoteTag) TableName() string {
	return "note_tags"
}

// NoteProperty 笔记属性模型
// 用于存储笔记的扩展属性，支持键值对形式的自定义字段
// 提供灵活的属性扩展机制，满足不同场景下的个性化需求
type NoteProperty struct {
	ID          uint           `gorm:"primarykey" json:"id"`                    // 主键ID，自增
	NoteID      uint           `gorm:"not null;index" json:"note_id"`           // 关联的笔记ID，外键，必填
	Note        Note           `gorm:"foreignKey:NoteID" json:"note,omitempty"` // 关联的笔记对象
	PropertyKey string         `gorm:"not null;size:100" json:"property_key"`   // 属性键名，必填，最大100字符
	PropertyValue string       `gorm:"type:text" json:"property_value"`         // 属性值，支持长文本存储
	DataType    string         `gorm:"size:20;default:'string'" json:"data_type"` // 数据类型：string、number、boolean、date等
	IsSearchable bool          `gorm:"default:false" json:"is_searchable"`      // 是否可搜索，用于搜索索引优化
	SortOrder   int            `gorm:"default:0" json:"sort_order"`             // 排序顺序，用于属性显示排序
	CreatedAt   time.Time      `json:"created_at"`                             // 属性创建时间
	UpdatedAt   time.Time      `json:"updated_at"`                             // 属性最后修改时间
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`                         // 软删除时间戳，支持逻辑删除
}

// TableName 指定NoteProperty模型对应的数据库表名
// 返回值: "note_properties" - 数据库中的表名
// 用途: GORM框架通过此方法确定模型对应的数据库表
func (NoteProperty) TableName() string {
	return "note_properties"
}