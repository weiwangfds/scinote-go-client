// Package database 提供数据库迁移和初始化功能
// 包含笔记系统相关表的创建和索引优化
package database

import (
	"fmt"
	"log"

	"gorm.io/gorm"
)

// MigrateNotesTables 执行笔记系统相关表的数据库迁移
// 参数: db *gorm.DB - GORM数据库连接实例
// 返回值: error - 迁移失败时返回错误信息
// 用途: 创建笔记、标签、关联表和扩展属性表，并建立必要的索引
func MigrateNotesTables(db *gorm.DB) error {
	log.Println("开始执行笔记系统数据库迁移...")

	// 自动迁移所有笔记相关的表结构
	err := db.AutoMigrate(
		&Note{},         // 笔记主表
		&Tag{},          // 标签表
		&NoteTag{},      // 笔记标签关联表
		&NoteProperty{}, // 笔记扩展属性表
	)
	if err != nil {
		return err
	}

	// 创建复合索引以优化查询性能
	if err := createNotesIndexes(db); err != nil {
		return err
	}

	log.Println("笔记系统数据库迁移完成")
	return nil
}

// createNotesIndexes 创建笔记系统的复合索引
// 参数: db *gorm.DB - GORM数据库连接实例
// 返回值: error - 创建索引失败时返回错误信息
// 用途: 优化层级查询、标签查询和属性查询的性能
func createNotesIndexes(db *gorm.DB) error {
	// 笔记表的复合索引
	indexes := []string{
		// 层级查询优化：根据父ID和排序字段查询子笔记
		"CREATE INDEX IF NOT EXISTS idx_notes_parent_sort ON notes(parent_id, sort_order) WHERE deleted_at IS NULL",
		// 路径查询优化：支持祖先路径的前缀查询
		"CREATE INDEX IF NOT EXISTS idx_notes_path_level ON notes(path, level) WHERE deleted_at IS NULL",
		// 用户笔记查询优化：根据创建者查询笔记
		"CREATE INDEX IF NOT EXISTS idx_notes_creator_created ON notes(creator_id, created_at DESC) WHERE deleted_at IS NULL",
		// 公开笔记查询优化
		"CREATE INDEX IF NOT EXISTS idx_notes_public_created ON notes(is_public, created_at DESC) WHERE deleted_at IS NULL AND is_public = true",
		// 收藏笔记查询优化
		"CREATE INDEX IF NOT EXISTS idx_notes_favorite_updated ON notes(creator_id, is_favorite, updated_at DESC) WHERE deleted_at IS NULL AND is_favorite = true",
		
		// 标签表索引
		"CREATE INDEX IF NOT EXISTS idx_tags_usage_count ON tags(usage_count DESC) WHERE deleted_at IS NULL",
		
		// 笔记标签关联表的复合索引
		"CREATE INDEX IF NOT EXISTS idx_note_tags_note_created ON note_tags(note_id, created_at DESC)",
		"CREATE INDEX IF NOT EXISTS idx_note_tags_tag_created ON note_tags(tag_id, created_at DESC)",
		
		// 笔记属性表的复合索引
		"CREATE INDEX IF NOT EXISTS idx_note_properties_key_type ON note_properties(note_id, property_key, property_type) WHERE deleted_at IS NULL",
		"CREATE INDEX IF NOT EXISTS idx_note_properties_key_value ON note_properties(property_key, text_value) WHERE deleted_at IS NULL AND property_type = 'text'",
	}

	// 执行所有索引创建语句
	for _, indexSQL := range indexes {
		if err := db.Exec(indexSQL).Error; err != nil {
			log.Printf("创建索引失败: %s, 错误: %v", indexSQL, err)
			return err
		}
	}

	log.Println("笔记系统索引创建完成")
	return nil
}

// SeedNotesData 初始化笔记系统的示例数据
// 参数: db *gorm.DB - GORM数据库连接实例
// 返回值: error - 初始化失败时返回错误信息
// 用途: 为开发和测试环境提供示例数据
func SeedNotesData(db *gorm.DB) error {
	log.Println("开始初始化笔记系统示例数据...")

	// 创建示例标签
	tags := []Tag{
		{
			TagID:       "tag-001",
			Name:        "实验记录",
			Color:       "#2196F3",
			Description: "实验过程和结果记录",
		},
		{
			TagID:       "tag-002", 
			Name:        "文献阅读",
			Color:       "#4CAF50",
			Description: "论文研读笔记和文献综述",
		},
		{
			TagID:       "tag-003",
			Name:        "会议记录",
			Color:       "#FF9800",
			Description: "学术会议和组会记录",
		},
		{
			TagID:       "tag-004",
			Name:        "研究思路",
			Color:       "#E91E63",
			Description: "研究想法和课题规划",
		},
		{
			TagID:       "tag-005",
			Name:        "数据分析",
			Color:       "#9C27B0",
			Description: "实验数据处理和分析",
		},
	}

	// 批量创建标签
	for _, tag := range tags {
		if err := db.FirstOrCreate(&tag, Tag{TagID: tag.TagID}).Error; err != nil {
			return err
		}
	}

	// 创建示例根笔记
	rootNote := Note{
		NoteID:    "note-root-001",
		Title:     "我的工作空间",
		Type:      "page",
		Icon:      "🏠",
		Path:      "/1",
		Level:     0,
		CreatorID: "user-001",
		UpdaterID: "user-001",
	}

	if err := db.FirstOrCreate(&rootNote, Note{NoteID: rootNote.NoteID}).Error; err != nil {
		return err
	}

	// 创建子笔记
	childNotes := []Note{
		{
			NoteID:    "note-child-001",
			Title:     "项目A - 需求分析",
			ParentID:  &rootNote.ID,
			Type:      "page",
			Icon:      "📋",
			Path:      fmt.Sprintf("/1/%d", rootNote.ID+1),
			Level:     1,
			SortOrder: 1,
			CreatorID: "user-001",
			UpdaterID: "user-001",
		},
		{
			NoteID:    "note-child-002",
			Title:     "学习笔记 - Go语言",
			ParentID:  &rootNote.ID,
			Type:      "page",
			Icon:      "📚",
			Path:      fmt.Sprintf("/1/%d", rootNote.ID+2),
			Level:     1,
			SortOrder: 2,
			CreatorID: "user-001",
			UpdaterID: "user-001",
		},
	}

	// 批量创建子笔记
	for _, note := range childNotes {
		if err := db.FirstOrCreate(&note, Note{NoteID: note.NoteID}).Error; err != nil {
			return err
		}
	}

	log.Println("笔记系统示例数据初始化完成")
	return nil
}