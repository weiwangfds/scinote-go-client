// Package test 提供笔记服务的单元测试
// 测试笔记的创建、查询、更新、删除等核心功能
package test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/weiwangfds/scinote/config"
	"github.com/weiwangfds/scinote/internal/database"
	fileservice "github.com/weiwangfds/scinote/internal/service/file"
	noteservice "github.com/weiwangfds/scinote/internal/service/note"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupTestDB 设置测试数据库
func setupTestDB(t *testing.T) *gorm.DB {
	// 使用内存SQLite数据库进行测试
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// 自动迁移表结构
	err = db.AutoMigrate(
		&database.FileMetadata{},
		&database.Note{},
		&database.Tag{},
		&database.NoteTag{},
		&database.NoteProperty{},
	)
	require.NoError(t, err)

	return db
}

// setupServices 设置测试服务
func setupServices(t *testing.T) (noteservice.NoteService, fileservice.FileService, *gorm.DB) {
	db := setupTestDB(t)

	// 创建文件服务配置
	fileConfig := config.FileConfig{
		StoragePath:      "./test_data",
		MaxFileSize:      10 * 1024 * 1024, // 10MB
		AllowedExtensions: []string{"*"},
	}

	// 创建服务实例，确保使用同一个数据库实例
	fileService := fileservice.NewFileService(db, fileConfig)
	noteService := noteservice.NewNoteService(db, fileService)

	return noteService, fileService, db
}

// TestCreateNote 测试创建笔记
func TestCreateNote(t *testing.T) {
	noteService, _, db := setupServices(t)

	// 验证数据库表是否存在
	t.Run("验证数据库表", func(t *testing.T) {
		var count int64
		err := db.Table("file_metadata").Count(&count).Error
		require.NoError(t, err, "file_metadata表应该存在")
		
		err = db.Table("notes").Count(&count).Error
		require.NoError(t, err, "notes表应该存在")
	})

	t.Run("创建根笔记", func(t *testing.T) {
		req := &noteservice.CreateNoteRequest{
			Title:     "测试笔记",
			Type:      "page",
			Icon:      "📝",
			// Content:   "这是一个测试笔记的内容", // 暂时不测试文件内容
			IsPublic:  true,
			CreatorID: "user123",
		}

		note, err := noteService.CreateNote(req)
		require.NoError(t, err)
		assert.NotNil(t, note)
		assert.Equal(t, req.Title, note.Title)
		assert.Equal(t, req.Type, note.Type)
		assert.Equal(t, req.Icon, note.Icon)
		assert.Equal(t, req.IsPublic, note.IsPublic)
		assert.Equal(t, req.CreatorID, note.CreatorID)
		assert.Equal(t, 0, note.Level) // 根笔记层级为0
		assert.NotEmpty(t, note.NoteID)
		// assert.NotNil(t, note.FileID) // 没有提供Content，所以不会创建文件
	})

	t.Run("创建子笔记", func(t *testing.T) {
		// 先创建父笔记
		parentReq := &noteservice.CreateNoteRequest{
			Title:     "父笔记",
			Type:      "page",
			CreatorID: "user123",
		}
		parentNote, err := noteService.CreateNote(parentReq)
		require.NoError(t, err)

		// 创建子笔记
		childReq := &noteservice.CreateNoteRequest{
			Title:     "子笔记",
			Type:      "page",
			ParentID:  &parentNote.NoteID,
			CreatorID: "user123",
		}
		childNote, err := noteService.CreateNote(childReq)
		require.NoError(t, err)
		assert.NotNil(t, childNote)
		assert.Equal(t, 1, childNote.Level) // 子笔记层级为1
		assert.NotNil(t, childNote.ParentID)
		assert.Contains(t, childNote.Path, "/")
	})

	t.Run("创建带标签和属性的笔记", func(t *testing.T) {
		// 先创建标签
		tag := &database.Tag{
			Name:  "测试标签",
			Color: "#FF0000",
		}
		err := db.Create(tag).Error
		require.NoError(t, err)

		req := &noteservice.CreateNoteRequest{
			Title:     "带标签的笔记",
			Type:      "page",
			CreatorID: "user123",
			Tags:      []string{tag.TagID},
			Properties: map[string]interface{}{
				"priority": "high",
				"status":   "draft",
			},
		}

		note, err := noteService.CreateNote(req)
		require.NoError(t, err)
		assert.NotNil(t, note)

		// 验证标签和属性
		properties, err := noteService.GetNoteProperties(note.NoteID)
		require.NoError(t, err)
		assert.Len(t, properties, 2)
	})
}

// TestGetNoteByID 测试获取笔记
func TestGetNoteByID(t *testing.T) {
	noteService, _, _ := setupServices(t)

	// 创建测试笔记
	req := &noteservice.CreateNoteRequest{
		Title:     "测试获取笔记",
		Type:      "page",
		Content:   "测试内容",
		CreatorID: "user123",
	}
	createdNote, err := noteService.CreateNote(req)
	require.NoError(t, err)

	t.Run("获取存在的笔记", func(t *testing.T) {
		note, err := noteService.GetNoteByID(createdNote.NoteID, true)
		require.NoError(t, err)
		assert.NotNil(t, note)
		assert.Equal(t, createdNote.NoteID, note.NoteID)
		assert.Equal(t, createdNote.Title, note.Title)
	})

	t.Run("获取不存在的笔记", func(t *testing.T) {
		note, err := noteService.GetNoteByID("nonexistent", false)
		assert.Error(t, err)
		assert.Nil(t, note)
		assert.Contains(t, err.Error(), "note not found")
	})
}

// TestUpdateNote 测试更新笔记
func TestUpdateNote(t *testing.T) {
	noteService, _, _ := setupServices(t)

	// 创建测试笔记
	req := &noteservice.CreateNoteRequest{
		Title:     "原始标题",
		Type:      "page",
		Content:   "原始内容",
		CreatorID: "user123",
	}
	createdNote, err := noteService.CreateNote(req)
	require.NoError(t, err)

	t.Run("更新笔记基本信息", func(t *testing.T) {
		newTitle := "更新后的标题"
		newContent := "更新后的内容"
		isPublic := true

		updateReq := &noteservice.UpdateNoteRequest{
			Title:     &newTitle,
			Content:   &newContent,
			IsPublic:  &isPublic,
			UpdaterID: "user456",
		}

		updatedNote, err := noteService.UpdateNote(createdNote.NoteID, updateReq)
		require.NoError(t, err)
		assert.NotNil(t, updatedNote)
		assert.Equal(t, newTitle, updatedNote.Title)
		assert.Equal(t, isPublic, updatedNote.IsPublic)
		assert.Equal(t, "user456", updatedNote.UpdaterID)
	})

	t.Run("更新不存在的笔记", func(t *testing.T) {
		newTitle := "新标题"
		updateReq := &noteservice.UpdateNoteRequest{
			Title:     &newTitle,
			UpdaterID: "user123",
		}

		updatedNote, err := noteService.UpdateNote("nonexistent", updateReq)
		assert.Error(t, err)
		assert.Nil(t, updatedNote)
		assert.Contains(t, err.Error(), "note not found")
	})
}

// TestDeleteNote 测试删除笔记
func TestDeleteNote(t *testing.T) {
	noteService, _, _ := setupServices(t)

	t.Run("删除单个笔记", func(t *testing.T) {
		// 创建测试笔记
		req := &noteservice.CreateNoteRequest{
			Title:     "待删除笔记",
			Type:      "page",
			CreatorID: "user123",
		}
		createdNote, err := noteService.CreateNote(req)
		require.NoError(t, err)

		// 删除笔记
		err = noteService.DeleteNote(createdNote.NoteID, false)
		require.NoError(t, err)

		// 验证笔记已被删除
		note, err := noteService.GetNoteByID(createdNote.NoteID, false)
		assert.Error(t, err)
		assert.Nil(t, note)
	})

	t.Run("级联删除笔记树", func(t *testing.T) {
		// 创建父笔记
		parentReq := &noteservice.CreateNoteRequest{
			Title:     "父笔记",
			Type:      "page",
			CreatorID: "user123",
		}
		parentNote, err := noteService.CreateNote(parentReq)
		require.NoError(t, err)

		// 创建子笔记
		childReq := &noteservice.CreateNoteRequest{
			Title:     "子笔记",
			Type:      "page",
			ParentID:  &parentNote.NoteID,
			CreatorID: "user123",
		}
		childNote, err := noteService.CreateNote(childReq)
		require.NoError(t, err)

		// 级联删除父笔记
		err = noteService.DeleteNote(parentNote.NoteID, true)
		require.NoError(t, err)

		// 验证父笔记和子笔记都被删除
		parent, err := noteService.GetNoteByID(parentNote.NoteID, false)
		assert.Error(t, err)
		assert.Nil(t, parent)

		child, err := noteService.GetNoteByID(childNote.NoteID, false)
		assert.Error(t, err)
		assert.Nil(t, child)
	})
}

// TestGetNoteChildren 测试获取子笔记
func TestGetNoteChildren(t *testing.T) {
	noteService, _, _ := setupServices(t)

	// 创建父笔记
	parentReq := &noteservice.CreateNoteRequest{
		Title:     "父笔记",
		Type:      "page",
		CreatorID: "user123",
	}
	parentNote, err := noteService.CreateNote(parentReq)
	require.NoError(t, err)

	// 创建多个子笔记
	for i := 0; i < 3; i++ {
		childReq := &noteservice.CreateNoteRequest{
			Title:     fmt.Sprintf("子笔记%d", i+1),
			Type:      "page",
			ParentID:  &parentNote.NoteID,
			SortOrder: i,
			CreatorID: "user123",
		}
		_, err := noteService.CreateNote(childReq)
		require.NoError(t, err)
	}

	t.Run("获取子笔记列表", func(t *testing.T) {
		children, total, err := noteService.GetNoteChildren(parentNote.NoteID, 1, 10)
		require.NoError(t, err)
		assert.Len(t, children, 3)
		assert.Equal(t, int64(3), total)
	})

	t.Run("获取根笔记列表", func(t *testing.T) {
		roots, total, err := noteService.GetNoteChildren("", 1, 10)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(roots), 1) // 至少有一个根笔记
		assert.GreaterOrEqual(t, total, int64(1))
	})
}

// TestMoveNote 测试移动笔记
func TestMoveNote(t *testing.T) {
	noteService, _, _ := setupServices(t)

	// 创建测试笔记结构
	parent1Req := &noteservice.CreateNoteRequest{
		Title:     "父笔记1",
		Type:      "page",
		CreatorID: "user123",
	}
	parent1, err := noteService.CreateNote(parent1Req)
	require.NoError(t, err)

	parent2Req := &noteservice.CreateNoteRequest{
		Title:     "父笔记2",
		Type:      "page",
		CreatorID: "user123",
	}
	parent2, err := noteService.CreateNote(parent2Req)
	require.NoError(t, err)

	childReq := &noteservice.CreateNoteRequest{
		Title:     "子笔记",
		Type:      "page",
		ParentID:  &parent1.NoteID,
		CreatorID: "user123",
	}
	child, err := noteService.CreateNote(childReq)
	require.NoError(t, err)

	t.Run("移动笔记到新父笔记", func(t *testing.T) {
		err := noteService.MoveNote(child.NoteID, parent2.NoteID, 0)
		require.NoError(t, err)

		// 验证笔记已移动
		updatedChild, err := noteService.GetNoteByID(child.NoteID, false)
		require.NoError(t, err)
		assert.NotNil(t, updatedChild.ParentID)

		// 验证新父笔记下有子笔记
		children, total, err := noteService.GetNoteChildren(parent2.NoteID, 1, 10)
		require.NoError(t, err)
		assert.Equal(t, int64(1), total)
		assert.Len(t, children, 1)
	})
}

// TestSearchNotes 测试搜索笔记
func TestSearchNotes(t *testing.T) {
	noteService, _, _ := setupServices(t)

	// 创建测试笔记
	testNotes := []struct {
		title   string
		content string
	}{
		{"Go语言学习", "Go是一门现代编程语言"},
		{"Python教程", "Python是一门简单易学的语言"},
		{"数据结构", "学习各种数据结构和算法"},
	}

	for _, testNote := range testNotes {
		req := &noteservice.CreateNoteRequest{
			Title:     testNote.title,
			Type:      "page",
			Content:   testNote.content,
			CreatorID: "user123",
		}
		_, err := noteService.CreateNote(req)
		require.NoError(t, err)
	}

	t.Run("按标题搜索", func(t *testing.T) {
		results, total, err := noteService.SearchNotes("Go语言", 1, 10)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(results), 1)
		assert.GreaterOrEqual(t, total, int64(1))
	})

	t.Run("按内容搜索", func(t *testing.T) {
		results, total, err := noteService.SearchNotes("语言", 1, 10)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(results), 2) // 应该找到Go和Python相关的笔记
		assert.GreaterOrEqual(t, total, int64(2))
	})
}

// TestNoteTagsAndProperties 测试笔记标签和属性管理
func TestNoteTagsAndProperties(t *testing.T) {
	noteService, _, db := setupServices(t)

	// 创建测试笔记
	req := &noteservice.CreateNoteRequest{
		Title:     "测试笔记",
		Type:      "page",
		CreatorID: "user123",
	}
	note, err := noteService.CreateNote(req)
	require.NoError(t, err)

	// 创建测试标签
	tag := &database.Tag{
		Name:  "重要",
		Color: "#FF0000",
	}
	err = db.Create(tag).Error
	require.NoError(t, err)

	t.Run("添加和移除标签", func(t *testing.T) {
		// 添加标签
		err := noteService.AddNoteTag(note.NoteID, tag.TagID)
		require.NoError(t, err)

		// 移除标签
		err = noteService.RemoveNoteTag(note.NoteID, tag.TagID)
		require.NoError(t, err)
	})

	t.Run("设置和获取属性", func(t *testing.T) {
		// 设置属性
		err = noteService.SetNoteProperty(note.NoteID, "priority", "high", "text")
		require.NoError(t, err)

		err = noteService.SetNoteProperty(note.NoteID, "score", 95.0, "number")
		require.NoError(t, err)

		// 获取属性
		properties, err := noteService.GetNoteProperties(note.NoteID)
		require.NoError(t, err)
		assert.Len(t, properties, 2)

		// 验证属性值
		priorityFound := false
		scoreFound := false
		for _, prop := range properties {
			if prop.PropertyKey == "priority" {
				assert.Equal(t, "high", prop.TextValue)
				assert.Equal(t, "text", prop.PropertyType)
				priorityFound = true
			}
			if prop.PropertyKey == "score" {
				assert.Equal(t, 95.0, *prop.NumberValue) // 数字存储在NumberValue字段
				assert.Equal(t, "number", prop.PropertyType)
				scoreFound = true
			}
		}
		assert.True(t, priorityFound, "priority属性未找到")
		assert.True(t, scoreFound, "score属性未找到")
	})
}

// TestBatchOperations 测试批量操作
func TestBatchOperations(t *testing.T) {
	noteService, _, _ := setupServices(t)

	// 创建测试笔记结构
	parent1Req := &noteservice.CreateNoteRequest{
		Title:     "源父笔记",
		Type:      "page",
		CreatorID: "user123",
	}
	parent1, err := noteService.CreateNote(parent1Req)
	require.NoError(t, err)

	parent2Req := &noteservice.CreateNoteRequest{
		Title:     "目标父笔记",
		Type:      "page",
		CreatorID: "user123",
	}
	parent2, err := noteService.CreateNote(parent2Req)
	require.NoError(t, err)

	// 创建多个子笔记
	var childIDs []string
	for i := 0; i < 3; i++ {
		childReq := &noteservice.CreateNoteRequest{
			Title:     fmt.Sprintf("子笔记%d", i+1),
			Type:      "page",
			ParentID:  &parent1.NoteID,
			CreatorID: "user123",
		}
		child, err := noteService.CreateNote(childReq)
		require.NoError(t, err)
		childIDs = append(childIDs, child.NoteID)
	}

	t.Run("批量移动笔记", func(t *testing.T) {
		err := noteService.BatchMoveNotes(childIDs, parent2.NoteID)
		require.NoError(t, err)

		// 验证笔记已移动到新父笔记下
		children, total, err := noteService.GetNoteChildren(parent2.NoteID, 1, 10)
		require.NoError(t, err)
		assert.Equal(t, int64(3), total)
		assert.Len(t, children, 3)
	})

	t.Run("移动笔记树", func(t *testing.T) {
		// 创建一个新的目标父笔记
		newParentReq := &noteservice.CreateNoteRequest{
			Title:     "新目标父笔记",
			Type:      "page",
			CreatorID: "user123",
		}
		newParent, err := noteService.CreateNote(newParentReq)
		require.NoError(t, err)

		// 移动整个笔记树
		err = noteService.MoveNoteTree(parent2.NoteID, newParent.NoteID)
		require.NoError(t, err)

		// 验证笔记树已移动
		children, total, err := noteService.GetNoteChildren(newParent.NoteID, 1, 10)
		require.NoError(t, err)
		assert.Equal(t, int64(1), total) // 应该有一个子笔记（parent2）
		assert.Len(t, children, 1)
	})
}