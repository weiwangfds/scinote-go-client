// Package note 提供笔记管理相关的业务逻辑服务
// 包含笔记的创建、修改、删除、查询等核心功能
// 支持无限层级的笔记组织结构和批量操作
package note

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/weiwangfds/scinote/internal/database"
	fileservice "github.com/weiwangfds/scinote/internal/service/file"
	"gorm.io/gorm"
)

// NoteService 笔记服务接口
// 提供完整的笔记管理功能，包括层级结构管理、文件内容集成等
type NoteService interface {
	// CreateNote 创建新笔记
	// 参数:
	//   req - 创建笔记请求
	// 返回:
	//   *database.Note - 创建的笔记信息
	//   error - 错误信息
	CreateNote(req *CreateNoteRequest) (*database.Note, error)

	// GetNoteByID 根据ID获取笔记详情
	// 参数:
	//   noteID - 笔记唯一标识符
	//   includeContent - 是否包含文件内容
	// 返回:
	//   *database.Note - 笔记信息
	//   error - 错误信息
	GetNoteByID(noteID string, includeContent bool) (*database.Note, error)

	// UpdateNote 更新笔记信息
	// 参数:
	//   noteID - 笔记唯一标识符
	//   req - 更新请求
	// 返回:
	//   *database.Note - 更新后的笔记信息
	//   error - 错误信息
	UpdateNote(noteID string, req *UpdateNoteRequest) (*database.Note, error)

	// DeleteNote 删除笔记（软删除）
	// 参数:
	//   noteID - 笔记唯一标识符
	//   cascade - 是否级联删除子笔记
	// 返回:
	//   error - 错误信息
	DeleteNote(noteID string, cascade bool) error

	// GetNoteChildren 获取笔记的直接子笔记
	// 参数:
	//   noteID - 父笔记ID，空字符串表示获取根笔记
	//   page - 页码
	//   pageSize - 每页数量
	// 返回:
	//   []database.Note - 子笔记列表
	//   int64 - 总数量
	//   error - 错误信息
	GetNoteChildren(noteID string, page, pageSize int) ([]database.Note, int64, error)

	// GetNoteTree 获取完整的笔记树结构
	// 参数:
	//   rootID - 根笔记ID，空字符串表示从顶级开始
	//   maxDepth - 最大深度，0表示无限制
	// 返回:
	//   []database.Note - 树形结构的笔记列表
	//   error - 错误信息
	GetNoteTree(rootID string, maxDepth int) ([]database.Note, error)

	// MoveNote 移动单个笔记到新的父笔记下
	// 参数:
	//   noteID - 要移动的笔记ID
	//   newParentID - 新父笔记ID，空字符串表示移动到根级别
	//   newSortOrder - 新的排序位置
	// 返回:
	//   error - 错误信息
	MoveNote(noteID string, newParentID string, newSortOrder int) error

	// BatchMoveNotes 批量移动多个笔记
	// 参数:
	//   noteIDs - 要移动的笔记ID列表
	//   newParentID - 新父笔记ID
	// 返回:
	//   error - 错误信息
	BatchMoveNotes(noteIDs []string, newParentID string) error

	// MoveNoteTree 移动整个笔记树
	// 参数:
	//   rootNoteID - 要移动的树根笔记ID
	//   newParentID - 新父笔记ID
	// 返回:
	//   error - 错误信息
	MoveNoteTree(rootNoteID string, newParentID string) error

	// SearchNotes 搜索笔记
	// 参数:
	//   query - 搜索关键词
	//   page - 页码
	//   pageSize - 每页数量
	// 返回:
	//   []database.Note - 搜索结果
	//   int64 - 总数量
	//   error - 错误信息
	SearchNotes(query string, page, pageSize int) ([]database.Note, int64, error)

	// AddNoteTag 为笔记添加标签
	// 参数:
	//   noteID - 笔记ID
	//   tagID - 标签ID
	// 返回:
	//   error - 错误信息
	AddNoteTag(noteID string, tagID string) error

	// RemoveNoteTag 移除笔记标签
	// 参数:
	//   noteID - 笔记ID
	//   tagID - 标签ID
	// 返回:
	//   error - 错误信息
	RemoveNoteTag(noteID string, tagID string) error

	// SetNoteProperty 设置笔记扩展属性
	// 参数:
	//   noteID - 笔记ID
	//   key - 属性键
	//   value - 属性值
	//   propertyType - 属性类型
	// 返回:
	//   error - 错误信息
	SetNoteProperty(noteID string, key string, value interface{}, propertyType string) error

	// GetNoteProperties 获取笔记的所有扩展属性
	// 参数:
	//   noteID - 笔记ID
	// 返回:
	//   []database.NoteProperty - 属性列表
	//   error - 错误信息
	GetNoteProperties(noteID string) ([]database.NoteProperty, error)
}

// CreateNoteRequest 创建笔记请求
type CreateNoteRequest struct {
	Title      string                 `json:"title" binding:"required,max=255"` // 笔记标题
	ParentID   *string                `json:"parent_id"`                        // 父笔记ID
	Type       string                 `json:"type" binding:"required"`          // 笔记类型
	Icon       string                 `json:"icon"`                             // 图标
	Cover      string                 `json:"cover"`                            // 封面
	Content    string                 `json:"content"`                          // 笔记内容
	IsPublic   bool                   `json:"is_public"`                        // 是否公开
	IsFavorite bool                   `json:"is_favorite"`                      // 是否收藏
	SortOrder  int                    `json:"sort_order"`                       // 排序
	CreatorID  string                 `json:"creator_id" binding:"required"`    // 创建者ID
	Tags       []string               `json:"tags"`                             // 标签ID列表
	Properties map[string]interface{} `json:"properties"`                       // 扩展属性
}

// UpdateNoteRequest 更新笔记请求
type UpdateNoteRequest struct {
	Title      *string                `json:"title"`                         // 笔记标题
	Type       *string                `json:"type"`                          // 笔记类型
	Icon       *string                `json:"icon"`                          // 图标
	Cover      *string                `json:"cover"`                         // 封面
	Content    *string                `json:"content"`                       // 笔记内容
	IsPublic   *bool                  `json:"is_public"`                     // 是否公开
	IsArchived *bool                  `json:"is_archived"`                   // 是否归档
	IsFavorite *bool                  `json:"is_favorite"`                   // 是否收藏
	SortOrder  *int                   `json:"sort_order"`                    // 排序
	UpdaterID  string                 `json:"updater_id" binding:"required"` // 更新者ID
	Tags       []string               `json:"tags"`                          // 标签ID列表
	Properties map[string]interface{} `json:"properties"`                    // 扩展属性
}

// noteService 笔记服务实现
type noteService struct {
	db          *gorm.DB
	fileService fileservice.FileService
}

// NewNoteService 创建笔记服务实例
// 参数:
//
//	db - 数据库连接
//	fileService - 文件服务
//
// 返回:
//
//	NoteService - 笔记服务接口
func NewNoteService(db *gorm.DB, fileService fileservice.FileService) NoteService {
	log.Println("Initializing note service")
	return &noteService{
		db:          db,
		fileService: fileService,
	}
}

// CreateNote 创建新笔记
func (s *noteService) CreateNote(req *CreateNoteRequest) (*database.Note, error) {
	log.Printf("Creating note: %s", req.Title)

	// 开始事务
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 生成笔记ID
	noteID := uuid.New().String()
	log.Printf("Generated note ID: %s", noteID)

	// 验证父笔记是否存在
	var parentNote *database.Note
	if req.ParentID != nil && *req.ParentID != "" {
		var parent database.Note
		if err := tx.Where("note_id = ?", *req.ParentID).First(&parent).Error; err != nil {
			tx.Rollback()
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("parent note not found: %s", *req.ParentID)
			}
			return nil, err
		}
		parentNote = &parent
		log.Printf("Found parent note: %s (ID: %d)", parent.Title, parent.ID)
	}

	// 创建笔记记录
	note := &database.Note{
		NoteID:     noteID,
		Title:      req.Title,
		Type:       req.Type,
		Icon:       req.Icon,
		Cover:      req.Cover,
		IsPublic:   req.IsPublic,
		IsFavorite: req.IsFavorite,
		SortOrder:  req.SortOrder,
		CreatorID:  req.CreatorID,
		UpdaterID:  req.CreatorID,
	}

	// 设置父笔记关系和层级信息
	if parentNote != nil {
		note.ParentID = &parentNote.ID
		note.Level = parentNote.Level + 1
	} else {
		note.Level = 0
	}

	// 保存笔记到数据库
	if err := tx.Create(note).Error; err != nil {
		tx.Rollback()
		log.Printf("Failed to create note: %v", err)
		return nil, fmt.Errorf("failed to create note: %w", err)
	}

	// 更新路径（需要使用生成的ID）
	if parentNote != nil {
		note.Path = note.BuildPath(parentNote.Path)
	} else {
		note.Path = fmt.Sprintf("/%d", note.ID)
	}

	if err := tx.Model(note).Update("path", note.Path).Error; err != nil {
		tx.Rollback()
		log.Printf("Failed to update note path: %v", err)
		return nil, fmt.Errorf("failed to update note path: %w", err)
	}

	// 添加标签
	if len(req.Tags) > 0 {
		if err := s.addNoteTags(tx, note.ID, req.Tags); err != nil {
			tx.Rollback()
			log.Printf("Failed to add tags to note: %v", err)
			return nil, fmt.Errorf("failed to add tags: %w", err)
		}
	}

	// 设置扩展属性
	if len(req.Properties) > 0 {
		if err := s.setNoteProperties(tx, note.ID, req.Properties); err != nil {
			tx.Rollback()
			log.Printf("Failed to set note properties: %v", err)
			return nil, fmt.Errorf("failed to set properties: %w", err)
		}
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		log.Printf("Failed to commit note creation transaction: %v", err)
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// 如果有内容，创建文件（在事务外进行，避免死锁）
	if req.Content != "" {
		fileName := fmt.Sprintf("%s.md", req.Title)
		fileMetadata, err := s.fileService.UploadFile(fileName, strings.NewReader(req.Content))
		if err != nil {
			log.Printf("Failed to create file for note content: %v", err)
			return nil, fmt.Errorf("failed to create file for note content: %w", err)
		}

		// 关联文件（使用新的事务）
		if err := s.db.Model(note).Update("file_id", fileMetadata.FileID).Error; err != nil {
			log.Printf("Failed to associate file with note: %v", err)
			return nil, fmt.Errorf("failed to associate file with note: %w", err)
		}
		note.FileID = &fileMetadata.FileID
		log.Printf("Created file for note content: %s", fileMetadata.FileID)
	}

	log.Printf("Note created successfully: %s (ID: %s)", note.Title, note.NoteID)
	return note, nil
}

// GetNoteByID 根据ID获取笔记详情
func (s *noteService) GetNoteByID(noteID string, includeContent bool) (*database.Note, error) {
	log.Printf("Getting note by ID: %s (include content: %v)", noteID, includeContent)

	var note database.Note
	query := s.db.Where("note_id = ?", noteID)

	// 预加载关联数据
	if includeContent {
		query = query.Preload("File").Preload("Tags").Preload("Properties")
	} else {
		query = query.Preload("Tags").Preload("Properties")
	}

	if err := query.First(&note).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("Note not found: %s", noteID)
			return nil, fmt.Errorf("note not found: %s", noteID)
		}
		log.Printf("Failed to get note %s: %v", noteID, err)
		return nil, err
	}

	// 增加查看次数
	go func() {
		s.db.Model(&note).Update("view_count", gorm.Expr("view_count + 1"))
	}()

	log.Printf("Found note: %s (Title: %s)", noteID, note.Title)
	return &note, nil
}

// UpdateNote 更新笔记信息
func (s *noteService) UpdateNote(noteID string, req *UpdateNoteRequest) (*database.Note, error) {
	log.Printf("Updating note: %s", noteID)

	// 开始事务
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 获取现有笔记
	var note database.Note
	if err := tx.Where("note_id = ?", noteID).First(&note).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("note not found: %s", noteID)
		}
		return nil, err
	}

	// 构建更新数据
	updates := make(map[string]interface{})
	updates["updated_at"] = time.Now()
	updates["updater_id"] = req.UpdaterID

	if req.Title != nil {
		updates["title"] = *req.Title
	}
	if req.Type != nil {
		updates["type"] = *req.Type
	}
	if req.Icon != nil {
		updates["icon"] = *req.Icon
	}
	if req.Cover != nil {
		updates["cover"] = *req.Cover
	}
	if req.IsPublic != nil {
		updates["is_public"] = *req.IsPublic
	}
	if req.IsArchived != nil {
		updates["is_archived"] = *req.IsArchived
	}
	if req.IsFavorite != nil {
		updates["is_favorite"] = *req.IsFavorite
	}
	if req.SortOrder != nil {
		updates["sort_order"] = *req.SortOrder
	}

	// 更新笔记基本信息
	if err := tx.Model(&note).Updates(updates).Error; err != nil {
		tx.Rollback()
		log.Printf("Failed to update note %s: %v", noteID, err)
		return nil, fmt.Errorf("failed to update note: %w", err)
	}

	// 更新内容文件
	if req.Content != nil {
		if note.FileID != nil {
			// 更新现有文件
			_, err := s.fileService.UpdateFile(*note.FileID, strings.NewReader(*req.Content))
			if err != nil {
				tx.Rollback()
				log.Printf("Failed to update note content file: %v", err)
				return nil, fmt.Errorf("failed to update note content: %w", err)
			}
		} else {
			// 创建新文件
			fileName := fmt.Sprintf("%s.md", note.Title)
			if req.Title != nil {
				fileName = fmt.Sprintf("%s.md", *req.Title)
			}
			fileMetadata, err := s.fileService.UploadFile(fileName, strings.NewReader(*req.Content))
			if err != nil {
				tx.Rollback()
				log.Printf("Failed to create file for note content: %v", err)
				return nil, fmt.Errorf("failed to create file for note content: %w", err)
			}

			// 关联文件
			if err := tx.Model(&note).Update("file_id", fileMetadata.FileID).Error; err != nil {
				tx.Rollback()
				log.Printf("Failed to associate file with note: %v", err)
				return nil, fmt.Errorf("failed to associate file with note: %w", err)
			}
			note.FileID = &fileMetadata.FileID
		}
	}

	// 更新标签
	if req.Tags != nil {
		// 删除现有标签关联
		if err := tx.Where("note_id = ?", note.ID).Delete(&database.NoteTag{}).Error; err != nil {
			tx.Rollback()
			log.Printf("Failed to remove existing tags: %v", err)
			return nil, fmt.Errorf("failed to remove existing tags: %w", err)
		}

		// 添加新标签
		if len(req.Tags) > 0 {
			if err := s.addNoteTags(tx, note.ID, req.Tags); err != nil {
				tx.Rollback()
				log.Printf("Failed to add new tags: %v", err)
				return nil, fmt.Errorf("failed to add new tags: %w", err)
			}
		}
	}

	// 更新扩展属性
	if req.Properties != nil {
		// 删除现有属性
		if err := tx.Where("note_id = ?", note.ID).Delete(&database.NoteProperty{}).Error; err != nil {
			tx.Rollback()
			log.Printf("Failed to remove existing properties: %v", err)
			return nil, fmt.Errorf("failed to remove existing properties: %w", err)
		}

		// 设置新属性
		if len(req.Properties) > 0 {
			if err := s.setNoteProperties(tx, note.ID, req.Properties); err != nil {
				tx.Rollback()
				log.Printf("Failed to set new properties: %v", err)
				return nil, fmt.Errorf("failed to set new properties: %w", err)
			}
		}
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		log.Printf("Failed to commit note update transaction: %v", err)
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// 重新获取更新后的笔记
	updatedNote, err := s.GetNoteByID(noteID, false)
	if err != nil {
		log.Printf("Failed to get updated note: %v", err)
		return nil, err
	}

	log.Printf("Note updated successfully: %s", noteID)
	return updatedNote, nil
}

// DeleteNote 删除笔记（软删除）
func (s *noteService) DeleteNote(noteID string, cascade bool) error {
	log.Printf("Deleting note: %s (cascade: %v)", noteID, cascade)

	// 开始事务
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 获取笔记信息
	var note database.Note
	if err := tx.Where("note_id = ?", noteID).First(&note).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("note not found: %s", noteID)
		}
		return err
	}

	// 如果需要级联删除，先删除所有子笔记
	if cascade {
		var childNotes []database.Note
		if err := tx.Where("parent_id = ?", note.ID).Find(&childNotes).Error; err != nil {
			tx.Rollback()
			log.Printf("Failed to find child notes: %v", err)
			return fmt.Errorf("failed to find child notes: %w", err)
		}

		for _, child := range childNotes {
			if err := s.deleteNoteRecursive(tx, child.NoteID); err != nil {
				tx.Rollback()
				log.Printf("Failed to delete child note %s: %v", child.NoteID, err)
				return fmt.Errorf("failed to delete child note %s: %w", child.NoteID, err)
			}
		}
	} else {
		// 检查是否有子笔记
		var childCount int64
		if err := tx.Model(&database.Note{}).Where("parent_id = ?", note.ID).Count(&childCount).Error; err != nil {
			tx.Rollback()
			log.Printf("Failed to count child notes: %v", err)
			return fmt.Errorf("failed to count child notes: %w", err)
		}

		if childCount > 0 {
			tx.Rollback()
			return fmt.Errorf("cannot delete note with children, use cascade=true or move children first")
		}
	}

	// 删除笔记本身
	if err := s.deleteNoteRecursive(tx, noteID); err != nil {
		tx.Rollback()
		log.Printf("Failed to delete note %s: %v", noteID, err)
		return fmt.Errorf("failed to delete note: %w", err)
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		log.Printf("Failed to commit note deletion transaction: %v", err)
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("Note deleted successfully: %s", noteID)
	return nil
}

// deleteNoteRecursive 递归删除笔记及其关联数据
func (s *noteService) deleteNoteRecursive(tx *gorm.DB, noteID string) error {
	// 获取笔记信息
	var note database.Note
	if err := tx.Where("note_id = ?", noteID).First(&note).Error; err != nil {
		return err
	}

	// 删除标签关联
	if err := tx.Where("note_id = ?", note.ID).Delete(&database.NoteTag{}).Error; err != nil {
		return fmt.Errorf("failed to delete note tags: %w", err)
	}

	// 删除扩展属性
	if err := tx.Where("note_id = ?", note.ID).Delete(&database.NoteProperty{}).Error; err != nil {
		return fmt.Errorf("failed to delete note properties: %w", err)
	}

	// 删除关联的文件
	if note.FileID != nil {
		go func() {
			if err := s.fileService.DeleteFile(*note.FileID); err != nil {
				log.Printf("Failed to delete associated file %s: %v", *note.FileID, err)
			}
		}()
	}

	// 软删除笔记记录
	if err := tx.Delete(&note).Error; err != nil {
		return fmt.Errorf("failed to delete note record: %w", err)
	}

	return nil
}

// GetNoteChildren 获取笔记的直接子笔记
func (s *noteService) GetNoteChildren(noteID string, page, pageSize int) ([]database.Note, int64, error) {
	log.Printf("Getting children for note: %s (page: %d, size: %d)", noteID, page, pageSize)

	var notes []database.Note
	var total int64

	query := s.db.Model(&database.Note{})

	if noteID == "" {
		// 获取根笔记
		query = query.Where("parent_id IS NULL")
	} else {
		// 获取指定笔记的子笔记
		var parentNote database.Note
		if err := s.db.Where("note_id = ?", noteID).First(&parentNote).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, 0, fmt.Errorf("parent note not found: %s", noteID)
			}
			return nil, 0, err
		}
		query = query.Where("parent_id = ?", parentNote.ID)
	}

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		log.Printf("Failed to count child notes: %v", err)
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Order("sort_order ASC, created_at DESC").Find(&notes).Error; err != nil {
		log.Printf("Failed to get child notes: %v", err)
		return nil, 0, err
	}

	log.Printf("Found %d child notes (total: %d)", len(notes), total)
	return notes, total, nil
}

// GetNoteTree 获取完整的笔记树结构
func (s *noteService) GetNoteTree(rootID string, maxDepth int) ([]database.Note, error) {
	log.Printf("Getting note tree from root: %s (max depth: %d)", rootID, maxDepth)

	var notes []database.Note
	query := s.db.Model(&database.Note{})

	if rootID == "" {
		// 从根级别开始
		if maxDepth > 0 {
			query = query.Where("level <= ?", maxDepth-1)
		}
	} else {
		// 从指定笔记开始
		var rootNote database.Note
		if err := s.db.Where("note_id = ?", rootID).First(&rootNote).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("root note not found: %s", rootID)
			}
			return nil, err
		}

		if maxDepth > 0 {
			query = query.Where("path LIKE ? AND level <= ?", rootNote.Path+"%", rootNote.Level+maxDepth)
		} else {
			query = query.Where("path LIKE ?", rootNote.Path+"%")
		}
	}

	if err := query.Order("level ASC, sort_order ASC, created_at DESC").Find(&notes).Error; err != nil {
		log.Printf("Failed to get note tree: %v", err)
		return nil, err
	}

	log.Printf("Found %d notes in tree", len(notes))
	return notes, nil
}

// MoveNote 移动单个笔记到新的父笔记下
func (s *noteService) MoveNote(noteID string, newParentID string, newSortOrder int) error {
	log.Printf("Moving note %s to parent %s with sort order %d", noteID, newParentID, newSortOrder)

	// 开始事务
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 获取要移动的笔记
	var note database.Note
	if err := tx.Where("note_id = ?", noteID).First(&note).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("note not found: %s", noteID)
		}
		return err
	}

	// 验证新父笔记
	var newParent *database.Note
	if newParentID != "" {
		var parent database.Note
		if err := tx.Where("note_id = ?", newParentID).First(&parent).Error; err != nil {
			tx.Rollback()
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("new parent note not found: %s", newParentID)
			}
			return err
		}
		newParent = &parent

		// 检查是否会形成循环引用
		if strings.Contains(parent.Path, fmt.Sprintf("/%d/", note.ID)) || strings.HasSuffix(parent.Path, fmt.Sprintf("/%d", note.ID)) {
			tx.Rollback()
			return fmt.Errorf("cannot move note to its descendant")
		}
	}

	// 更新笔记的父级关系
	updates := map[string]interface{}{
		"sort_order": newSortOrder,
		"updated_at": time.Now(),
	}

	if newParent != nil {
		updates["parent_id"] = newParent.ID
		updates["level"] = newParent.Level + 1
		updates["path"] = note.BuildPath(newParent.Path)
	} else {
		updates["parent_id"] = nil
		updates["level"] = 0
		updates["path"] = fmt.Sprintf("/%d", note.ID)
	}

	if err := tx.Model(&note).Updates(updates).Error; err != nil {
		tx.Rollback()
		log.Printf("Failed to update note parent: %v", err)
		return fmt.Errorf("failed to update note parent: %w", err)
	}

	// 更新所有子笔记的路径和层级
	if err := s.updateChildrenPaths(tx, note.ID, updates["path"].(string), updates["level"].(int)); err != nil {
		tx.Rollback()
		log.Printf("Failed to update children paths: %v", err)
		return fmt.Errorf("failed to update children paths: %w", err)
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		log.Printf("Failed to commit move transaction: %v", err)
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("Note moved successfully: %s", noteID)
	return nil
}

// updateChildrenPaths 递归更新子笔记的路径和层级
func (s *noteService) updateChildrenPaths(tx *gorm.DB, parentID uint, newParentPath string, newParentLevel int) error {
	var children []database.Note
	if err := tx.Where("parent_id = ?", parentID).Find(&children).Error; err != nil {
		return err
	}

	for _, child := range children {
		newPath := fmt.Sprintf("%s/%d", newParentPath, child.ID)
		newLevel := newParentLevel + 1

		if err := tx.Model(&child).Updates(map[string]interface{}{
			"path":  newPath,
			"level": newLevel,
		}).Error; err != nil {
			return err
		}

		// 递归更新子笔记的子笔记
		if err := s.updateChildrenPaths(tx, child.ID, newPath, newLevel); err != nil {
			return err
		}
	}

	return nil
}

// BatchMoveNotes 批量移动多个笔记
func (s *noteService) BatchMoveNotes(noteIDs []string, newParentID string) error {
	log.Printf("Batch moving %d notes to parent: %s", len(noteIDs), newParentID)

	// 开始事务
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 验证新父笔记
	var newParent *database.Note
	if newParentID != "" {
		var parent database.Note
		if err := tx.Where("note_id = ?", newParentID).First(&parent).Error; err != nil {
			tx.Rollback()
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("new parent note not found: %s", newParentID)
			}
			return err
		}
		newParent = &parent
	}

	// 获取目标父笔记下的最大排序值
	var maxSortOrder int
	if newParent != nil {
		tx.Model(&database.Note{}).Where("parent_id = ?", newParent.ID).Select("COALESCE(MAX(sort_order), 0)").Scan(&maxSortOrder)
	} else {
		tx.Model(&database.Note{}).Where("parent_id IS NULL").Select("COALESCE(MAX(sort_order), 0)").Scan(&maxSortOrder)
	}

	// 逐个移动笔记
	for i, noteID := range noteIDs {
		// 获取笔记
		var note database.Note
		if err := tx.Where("note_id = ?", noteID).First(&note).Error; err != nil {
			tx.Rollback()
			if errors.Is(err, gorm.ErrRecordNotFound) {
				log.Printf("Note not found during batch move: %s", noteID)
				continue // 跳过不存在的笔记
			}
			return err
		}

		// 检查循环引用
		if newParent != nil {
			if strings.Contains(newParent.Path, fmt.Sprintf("/%d/", note.ID)) || strings.HasSuffix(newParent.Path, fmt.Sprintf("/%d", note.ID)) {
				log.Printf("Skipping note %s to avoid circular reference", noteID)
				continue
			}
		}

		// 更新笔记
		updates := map[string]interface{}{
			"sort_order": maxSortOrder + i + 1,
			"updated_at": time.Now(),
		}

		if newParent != nil {
			updates["parent_id"] = newParent.ID
			updates["level"] = newParent.Level + 1
			updates["path"] = note.BuildPath(newParent.Path)
		} else {
			updates["parent_id"] = nil
			updates["level"] = 0
			updates["path"] = fmt.Sprintf("/%d", note.ID)
		}

		if err := tx.Model(&note).Updates(updates).Error; err != nil {
			tx.Rollback()
			log.Printf("Failed to update note %s during batch move: %v", noteID, err)
			return fmt.Errorf("failed to update note %s: %w", noteID, err)
		}

		// 更新子笔记路径
		if err := s.updateChildrenPaths(tx, note.ID, updates["path"].(string), updates["level"].(int)); err != nil {
			tx.Rollback()
			log.Printf("Failed to update children paths for note %s: %v", noteID, err)
			return fmt.Errorf("failed to update children paths for note %s: %w", noteID, err)
		}
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		log.Printf("Failed to commit batch move transaction: %v", err)
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("Batch move completed successfully for %d notes", len(noteIDs))
	return nil
}

// MoveNoteTree 移动整个笔记树
func (s *noteService) MoveNoteTree(rootNoteID string, newParentID string) error {
	log.Printf("Moving note tree with root: %s to parent: %s", rootNoteID, newParentID)

	// 获取树中的所有笔记
	treeNotes, err := s.GetNoteTree(rootNoteID, 0)
	if err != nil {
		log.Printf("Failed to get note tree: %v", err)
		return fmt.Errorf("failed to get note tree: %w", err)
	}

	if len(treeNotes) == 0 {
		return fmt.Errorf("note tree not found: %s", rootNoteID)
	}

	// 只移动根笔记，子笔记会自动跟随
	rootNote := treeNotes[0]
	return s.MoveNote(rootNote.NoteID, newParentID, 0)
}

// SearchNotes 搜索笔记
func (s *noteService) SearchNotes(query string, page, pageSize int) ([]database.Note, int64, error) {
	log.Printf("Searching notes with query: '%s' (page: %d, size: %d)", query, page, pageSize)

	var notes []database.Note
	var total int64

	searchQuery := "%" + query + "%"
	dbQuery := s.db.Model(&database.Note{}).Where("title LIKE ?", searchQuery)

	// 获取总数
	if err := dbQuery.Count(&total).Error; err != nil {
		log.Printf("Failed to count search results: %v", err)
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	if err := dbQuery.Offset(offset).Limit(pageSize).Order("updated_at DESC").Find(&notes).Error; err != nil {
		log.Printf("Failed to search notes: %v", err)
		return nil, 0, err
	}

	log.Printf("Found %d notes matching query (total: %d)", len(notes), total)
	return notes, total, nil
}

// AddNoteTag 添加笔记标签
func (s *noteService) AddNoteTag(noteID string, tagID string) error {
	log.Printf("Adding tag %s to note %s", tagID, noteID)

	// 开始事务
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 获取笔记
	var note database.Note
	if err := tx.Where("note_id = ?", noteID).First(&note).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("note not found: %s", noteID)
		}
		return err
	}

	// 获取标签
	var tag database.Tag
	if err := tx.Where("tag_id = ?", tagID).First(&tag).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("tag not found: %s", tagID)
		}
		return err
	}

	// 检查关联是否已存在
	var existingAssoc database.NoteTag
	if err := tx.Where("note_id = ? AND tag_id = ?", note.ID, tag.ID).First(&existingAssoc).Error; err == nil {
		tx.Rollback()
		return fmt.Errorf("tag already associated with note")
	}

	// 创建关联
	noteTag := &database.NoteTag{
		NoteID: note.ID,
		TagID:  tag.ID,
	}

	if err := tx.Create(noteTag).Error; err != nil {
		tx.Rollback()
		log.Printf("Failed to create note-tag association: %v", err)
		return fmt.Errorf("failed to add tag to note: %w", err)
	}

	// 增加标签使用次数
	if err := tx.Model(&tag).Update("usage_count", gorm.Expr("usage_count + 1")).Error; err != nil {
		tx.Rollback()
		log.Printf("Failed to increment tag usage count: %v", err)
		return fmt.Errorf("failed to update tag usage count: %w", err)
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		log.Printf("Failed to commit add note tag transaction: %v", err)
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("Tag added to note successfully: %s -> %s", tagID, noteID)
	return nil
}

// RemoveNoteTag 移除笔记标签
func (s *noteService) RemoveNoteTag(noteID string, tagID string) error {
	log.Printf("Removing tag %s from note %s", tagID, noteID)

	// 开始事务
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 获取笔记和标签
	var note database.Note
	if err := tx.Where("note_id = ?", noteID).First(&note).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("note not found: %s", noteID)
		}
		return err
	}

	var tag database.Tag
	if err := tx.Where("tag_id = ?", tagID).First(&tag).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("tag not found: %s", tagID)
		}
		return err
	}

	// 删除关联
	result := tx.Where("note_id = ? AND tag_id = ?", note.ID, tag.ID).Delete(&database.NoteTag{})
	if result.Error != nil {
		tx.Rollback()
		log.Printf("Failed to remove note-tag association: %v", result.Error)
		return fmt.Errorf("failed to remove tag from note: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		tx.Rollback()
		return fmt.Errorf("tag not associated with note")
	}

	// 减少标签使用次数
	if err := tx.Model(&tag).Update("usage_count", gorm.Expr("CASE WHEN usage_count > 0 THEN usage_count - 1 ELSE 0 END")).Error; err != nil {
		tx.Rollback()
		log.Printf("Failed to decrement tag usage count: %v", err)
		return fmt.Errorf("failed to update tag usage count: %w", err)
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		log.Printf("Failed to commit remove note tag transaction: %v", err)
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("Tag removed from note successfully: %s -> %s", tagID, noteID)
	return nil
}

// SetNoteProperty 设置笔记扩展属性
func (s *noteService) SetNoteProperty(noteID string, key string, value interface{}, propertyType string) error {
	log.Printf("Setting property %s for note %s (type: %s)", key, noteID, propertyType)

	// 开始事务
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 获取笔记
	var note database.Note
	if err := tx.Where("note_id = ?", noteID).First(&note).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("note not found: %s", noteID)
		}
		return err
	}

	// 查找现有属性
	var property database.NoteProperty
	err := tx.Where("note_id = ? AND property_key = ?", note.ID, key).First(&property).Error

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		tx.Rollback()
		log.Printf("Failed to query existing property: %v", err)
		return fmt.Errorf("failed to query existing property: %w", err)
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 创建新属性
		property = database.NoteProperty{
			NoteID:       note.ID,
			PropertyKey:  key,
			PropertyType: propertyType,
		}
	} else {
		// 更新现有属性类型
		property.PropertyType = propertyType
	}

	// 设置值
	if err := property.SetValue(value); err != nil {
		tx.Rollback()
		log.Printf("Failed to set property value: %v", err)
		return fmt.Errorf("failed to set property value: %w", err)
	}

	// 保存属性
	if err := tx.Save(&property).Error; err != nil {
		tx.Rollback()
		log.Printf("Failed to save property: %v", err)
		return fmt.Errorf("failed to save property: %w", err)
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		log.Printf("Failed to commit set note property transaction: %v", err)
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("Property set successfully: %s = %v", key, value)
	return nil
}

// GetNoteProperties 获取笔记的所有扩展属性
func (s *noteService) GetNoteProperties(noteID string) ([]database.NoteProperty, error) {
	log.Printf("Getting properties for note: %s", noteID)

	// 获取笔记
	var note database.Note
	if err := s.db.Where("note_id = ?", noteID).First(&note).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("note not found: %s", noteID)
		}
		return nil, err
	}

	// 获取属性
	var properties []database.NoteProperty
	if err := s.db.Where("note_id = ?", note.ID).Find(&properties).Error; err != nil {
		log.Printf("Failed to get note properties: %v", err)
		return nil, fmt.Errorf("failed to get note properties: %w", err)
	}

	log.Printf("Found %d properties for note %s", len(properties), noteID)
	return properties, nil
}

// addNoteTags 添加笔记标签（内部方法）
func (s *noteService) addNoteTags(tx *gorm.DB, noteID uint, tagIDs []string) error {
	for _, tagID := range tagIDs {
		// 获取标签
		var tag database.Tag
		if err := tx.Where("tag_id = ?", tagID).First(&tag).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				log.Printf("Tag not found: %s", tagID)
				continue
			}
			return err
		}

		// 检查关联是否已存在
		var existingAssoc database.NoteTag
		if err := tx.Where("note_id = ? AND tag_id = ?", noteID, tag.ID).First(&existingAssoc).Error; err == nil {
			continue // 已存在，跳过
		}

		// 创建关联
		noteTag := &database.NoteTag{
			NoteID: noteID,
			TagID:  tag.ID,
		}

		if err := tx.Create(noteTag).Error; err != nil {
			return fmt.Errorf("failed to create note-tag association: %w", err)
		}

		// 增加标签使用次数
		if err := tx.Model(&tag).Update("usage_count", gorm.Expr("usage_count + 1")).Error; err != nil {
			log.Printf("Failed to increment tag usage count: %v", err)
		}
	}

	return nil
}

// setNoteProperties 设置笔记扩展属性（内部方法）
func (s *noteService) setNoteProperties(tx *gorm.DB, noteID uint, properties map[string]interface{}) error {
	for key, value := range properties {
		// 推断属性类型
		propertyType := "text" // 默认为text类型，而不是string
		switch value.(type) {
		case int, int32, int64:
			propertyType = "number"
		case float32, float64:
			propertyType = "number"
		case bool:
			propertyType = "boolean"
		case time.Time:
			propertyType = "date"
		}

		// 查找现有属性
		var property database.NoteProperty
		err := tx.Where("note_id = ? AND property_key = ?", noteID, key).First(&property).Error

		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("failed to query existing property: %w", err)
		}

		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 创建新属性
			property = database.NoteProperty{
				NoteID:       noteID,
				PropertyKey:  key,
				PropertyType: propertyType,
			}
		} else {
			// 更新现有属性类型
			property.PropertyType = propertyType
		}

		// 设置值
		if err := property.SetValue(value); err != nil {
			return fmt.Errorf("failed to set property value: %w", err)
		}

		// 保存属性
		if err := tx.Save(&property).Error; err != nil {
			return fmt.Errorf("failed to save property: %w", err)
		}
	}

	return nil
}
