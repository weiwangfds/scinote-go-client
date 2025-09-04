// Package note 提供笔记管理相关的业务逻辑服务
// 包含笔记的创建、修改、删除、查询等核心功能
// 支持无限层级的笔记组织结构和批量操作
package note

import (
	"errors"
	"fmt"
	"time"

	"github.com/weiwangfds/scinote/internal/database"
	"github.com/weiwangfds/scinote/internal/logger"
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
	logger.Info("[笔记服务] 初始化笔记服务")
	return &noteService{
		db:          db,
		fileService: fileService,
	}
}

// CreateNote 创建新笔记
func (s *noteService) CreateNote(req *CreateNoteRequest) (*database.Note, error) {
	logger.Infof("[笔记服务] 创建笔记: %s", req.Title)

	// 开始事务
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 数据库将自动生成ID

	// 新的Note模型不再支持层级结构验证

	// 创建笔记记录
	note := &database.Note{
		Title:    req.Title,
		Content:  req.Content,
		Author:   req.CreatorID,
		Category: req.Type,
		IsPublic: req.IsPublic,
	}

	// 注意：新的Note模型不再支持层级结构，如需要可通过Category字段管理

	// 保存笔记到数据库
	if err := tx.Create(note).Error; err != nil {
		tx.Rollback()
		logger.Errorf("[笔记服务] 创建笔记失败: %v", err)
		return nil, fmt.Errorf("failed to create note: %w", err)
	}

	// 新的Note模型不再使用Path字段

	// 添加标签
	if len(req.Tags) > 0 {
		if err := s.addNoteTags(tx, note.ID, req.Tags); err != nil {
			tx.Rollback()
			logger.Errorf("[笔记服务] 为笔记添加标签失败: %v", err)
			return nil, fmt.Errorf("failed to add tags: %w", err)
		}
	}

	// 设置扩展属性
	if len(req.Properties) > 0 {
		if err := s.setNoteProperties(tx, note.ID, req.Properties); err != nil {
			tx.Rollback()
			logger.Errorf("[笔记服务] 设置笔记属性失败: %v", err)
			return nil, fmt.Errorf("failed to set properties: %w", err)
		}
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		logger.Errorf("[笔记服务] 提交笔记创建事务失败: %v", err)
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// 内容已直接存储在Note.Content字段中，无需额外文件处理

	logger.Infof("[笔记服务] 笔记创建成功: %s (ID: %d)", note.Title, note.ID)
	return note, nil
}

// GetNoteByID 根据ID获取笔记详情
func (s *noteService) GetNoteByID(noteID string, includeContent bool) (*database.Note, error) {
	logger.Infof("[笔记服务] 根据ID获取笔记: %s (包含内容: %v)", noteID, includeContent)

	var note database.Note
	query := s.db.Where("id = ?", noteID)

	// 预加载关联数据
	query = query.Preload("Tags").Preload("Properties")

	if err := query.First(&note).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Errorf("[笔记服务] 笔记不存在: %s", noteID)
			return nil, fmt.Errorf("note not found: %s", noteID)
		}
		logger.Errorf("[笔记服务] 获取笔记失败 %s: %v", noteID, err)
		return nil, err
	}

	// 增加查看次数
	go func() {
		s.db.Model(&note).Update("view_count", gorm.Expr("view_count + 1"))
	}()

	logger.Infof("[笔记服务] 找到笔记: %s (标题: %s)", noteID, note.Title)
	return &note, nil
}

// UpdateNote 更新笔记信息
func (s *noteService) UpdateNote(noteID string, req *UpdateNoteRequest) (*database.Note, error) {
	logger.Infof("[笔记服务] 更新笔记: %s", noteID)

	// 开始事务
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 获取现有笔记
	var note database.Note
	if err := tx.Where("id = ?", noteID).First(&note).Error; err != nil {
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
		logger.Errorf("[笔记服务] 更新笔记失败 %s: %v", noteID, err)
		return nil, fmt.Errorf("failed to update note: %w", err)
	}

	// 更新内容
	if req.Content != nil {
		updates["content"] = *req.Content
	}

	// 更新标签
	if req.Tags != nil {
		// 删除现有标签关联
		if err := tx.Where("note_id = ?", note.ID).Delete(&database.NoteTag{}).Error; err != nil {
			tx.Rollback()
			logger.Errorf("[笔记服务] 删除现有标签失败: %v", err)
			return nil, fmt.Errorf("failed to remove existing tags: %w", err)
		}

		// 添加新标签
		if len(req.Tags) > 0 {
			if err := s.addNoteTags(tx, note.ID, req.Tags); err != nil {
				tx.Rollback()
				logger.Errorf("Failed to add new tags: %v", err)
				return nil, fmt.Errorf("failed to add new tags: %w", err)
			}
		}
	}

	// 更新扩展属性
	if req.Properties != nil {
		// 删除现有属性
		if err := tx.Where("note_id = ?", note.ID).Delete(&database.NoteProperty{}).Error; err != nil {
			tx.Rollback()
			logger.Errorf("Failed to remove existing properties: %v", err)
			return nil, fmt.Errorf("failed to remove existing properties: %w", err)
		}

		// 设置新属性
		if len(req.Properties) > 0 {
			if err := s.setNoteProperties(tx, note.ID, req.Properties); err != nil {
				tx.Rollback()
				logger.Errorf("Failed to set new properties: %v", err)
				return nil, fmt.Errorf("failed to set new properties: %w", err)
			}
		}
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		logger.Errorf("[笔记服务] 提交笔记更新事务失败: %v", err)
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// 重新获取更新后的笔记
	updatedNote, err := s.GetNoteByID(noteID, false)
	if err != nil {
		logger.Errorf("[笔记服务] 获取更新后笔记失败: %v", err)
		return nil, err
	}

	logger.Infof("[笔记服务] 笔记更新成功: %s", noteID)
	return updatedNote, nil
}

// DeleteNote 删除笔记（软删除）
func (s *noteService) DeleteNote(noteID string, cascade bool) error {
	logger.Infof("[笔记服务] 删除笔记: %s (级联: %v)", noteID, cascade)

	// 开始事务
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 获取笔记信息
	var note database.Note
	if err := tx.Where("id = ?", noteID).First(&note).Error; err != nil {
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
			logger.Errorf("[笔记服务] 查找子笔记失败: %v", err)
			return fmt.Errorf("failed to find child notes: %w", err)
		}

		for _, child := range childNotes {
			if err := s.deleteNoteRecursive(tx, fmt.Sprintf("%d", child.ID)); err != nil {
				tx.Rollback()
				logger.Errorf("[笔记服务] 删除子笔记失败 %d: %v", child.ID, err)
				return fmt.Errorf("failed to delete child note %d: %w", child.ID, err)
			}
		}
	} else {
		// 检查是否有子笔记
		var childCount int64
		if err := tx.Model(&database.Note{}).Where("parent_id = ?", note.ID).Count(&childCount).Error; err != nil {
			tx.Rollback()
			logger.Errorf("[笔记服务] 统计子笔记数量失败: %v", err)
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
		logger.Errorf("[笔记服务] 删除笔记失败 %s: %v", noteID, err)
		return fmt.Errorf("failed to delete note: %w", err)
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		logger.Errorf("[笔记服务] 提交笔记删除事务失败: %v", err)
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	logger.Infof("[笔记服务] 笔记删除成功: %s", noteID)
	return nil
}

// deleteNoteRecursive 递归删除笔记及其关联数据
func (s *noteService) deleteNoteRecursive(tx *gorm.DB, noteID string) error {
	// 获取笔记信息
	var note database.Note
	if err := tx.Where("id = ?", noteID).First(&note).Error; err != nil {
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

	// 软删除笔记记录
	if err := tx.Delete(&note).Error; err != nil {
		return fmt.Errorf("failed to delete note record: %w", err)
	}

	return nil
}

// GetNoteChildren 获取笔记的直接子笔记
func (s *noteService) GetNoteChildren(noteID string, page, pageSize int) ([]database.Note, int64, error) {
	logger.Infof("[笔记服务] 获取笔记的子笔记: %s (页码: %d, 每页大小: %d)", noteID, page, pageSize)

	var notes []database.Note
	var total int64

	query := s.db.Model(&database.Note{})

	// 新的Note模型不支持层级结构，返回所有笔记
	if noteID != "" {
		// 验证笔记是否存在
		var parentNote database.Note
		if err := s.db.Where("id = ?", noteID).First(&parentNote).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, 0, fmt.Errorf("parent note not found: %s", noteID)
			}
			return nil, 0, err
		}
	}

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		logger.Errorf("[笔记服务] 统计子笔记数量失败: %v", err)
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&notes).Error; err != nil {
		logger.Errorf("[笔记服务] 获取笔记失败: %v", err)
		return nil, 0, err
	}

	logger.Infof("[笔记服务] 找到 %d 个子笔记 (总数: %d)", len(notes), total)
	return notes, total, nil
}

// GetNoteTree 获取完整的笔记树结构
func (s *noteService) GetNoteTree(rootID string, maxDepth int) ([]database.Note, error) {
	logger.Infof("[笔记服务] 获取笔记树结构，根节点: %s (最大深度: %d)", rootID, maxDepth)

	var notes []database.Note
	query := s.db.Model(&database.Note{})

	if rootID != "" {
		// 验证根笔记是否存在
		var rootNote database.Note
		if err := s.db.Where("id = ?", rootID).First(&rootNote).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("root note not found: %s", rootID)
			}
			return nil, err
		}
		// 新的Note模型不支持层级结构，返回单个笔记
		return []database.Note{rootNote}, nil
	}

	if err := query.Order("created_at DESC").Find(&notes).Error; err != nil {
		logger.Errorf("[笔记服务] 获取笔记树失败: %v", err)
		return nil, err
	}

	logger.Infof("[笔记服务] 在树中找到 %d 个笔记", len(notes))
	return notes, nil
}

// MoveNote 移动单个笔记到新的父笔记下
func (s *noteService) MoveNote(noteID string, newParentID string, newSortOrder int) error {
	logger.Infof("[笔记服务] 移动笔记 %s 到父节点 %s，排序号: %d", noteID, newParentID, newSortOrder)

	// 新的Note模型不支持层级结构和移动操作
	logger.Infof("[笔记服务] 新模型不支持笔记移动操作")
	return nil
}

// updateChildrenPaths 递归更新子笔记的路径和层级
func (s *noteService) updateChildrenPaths(tx *gorm.DB, parentID uint, newParentPath string, newParentLevel int) error {
	// 新的Note模型不支持层级结构
	return nil
}

// BatchMoveNotes 批量移动多个笔记
func (s *noteService) BatchMoveNotes(noteIDs []string, newParentID string) error {
	logger.Infof("[笔记服务] 批量移动 %d 个笔记到父节点: %s", len(noteIDs), newParentID)

	// 新的Note模型不支持批量移动操作
	logger.Infof("[笔记服务] 新模型不支持批量移动操作")
	return nil
}

// MoveNoteTree 移动整个笔记树
func (s *noteService) MoveNoteTree(rootNoteID string, newParentID string) error {
	logger.Infof("[笔记服务] 移动笔记树，根节点: %s 到父节点: %s", rootNoteID, newParentID)

	// 新的Note模型不支持树移动操作
	logger.Infof("[笔记服务] 新模型不支持树移动操作")
	return nil
}

// SearchNotes 搜索笔记
func (s *noteService) SearchNotes(query string, page, pageSize int) ([]database.Note, int64, error) {
	logger.Infof("[笔记服务] 搜索笔记，查询: '%s' (页码: %d, 每页大小: %d)", query, page, pageSize)

	var notes []database.Note
	var total int64

	searchQuery := "%" + query + "%"
	dbQuery := s.db.Model(&database.Note{}).Where("title LIKE ?", searchQuery)

	// 获取总数
	if err := dbQuery.Count(&total).Error; err != nil {
		logger.Errorf("[笔记服务] 统计搜索结果数量失败: %v", err)
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	if err := dbQuery.Offset(offset).Limit(pageSize).Order("updated_at DESC").Find(&notes).Error; err != nil {
		logger.Errorf("[笔记服务] 搜索笔记失败: %v", err)
		return nil, 0, err
	}

	logger.Infof("[笔记服务] 找到 %d 个匹配查询的笔记 (总数: %d)", len(notes), total)
	return notes, total, nil
}

// AddNoteTag 添加笔记标签
func (s *noteService) AddNoteTag(noteID string, tagID string) error {
	logger.Infof("[笔记服务] 为笔记添加标签 %s 到笔记 %s", tagID, noteID)

	// 开始事务
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 获取笔记
	var note database.Note
	if err := tx.Where("id = ?", noteID).First(&note).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("note not found: %s", noteID)
		}
		return err
	}

	// 获取标签
	var tag database.Tag
	if err := tx.Where("id = ?", tagID).First(&tag).Error; err != nil {
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

	if err := tx.Create(&noteTag).Error; err != nil {
		tx.Rollback()
		logger.Errorf("[笔记服务] 创建笔记-标签关联失败: %v", err)
		return fmt.Errorf("failed to add tag to note: %w", err)
	}

	// 增加标签使用次数
	if err := tx.Model(&tag).Update("usage_count", gorm.Expr("usage_count + 1")).Error; err != nil {
		tx.Rollback()
		logger.Errorf("[笔记服务] 增加标签使用次数失败: %v", err)
		return fmt.Errorf("failed to update tag usage count: %w", err)
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		logger.Errorf("[笔记服务] 提交添加笔记标签事务失败: %v", err)
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	logger.Infof("[笔记服务] 标签添加到笔记成功: %s -> %s", tagID, noteID)
	return nil
}

// RemoveNoteTag 移除笔记标签
func (s *noteService) RemoveNoteTag(noteID string, tagID string) error {
	logger.Infof("[笔记服务] 从笔记移除标签 %s 从笔记 %s", tagID, noteID)

	// 开始事务
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 获取笔记和标签
	var note database.Note
	if err := tx.Where("id = ?", noteID).First(&note).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("note not found: %s", noteID)
		}
		return err
	}

	var tag database.Tag
	if err := tx.Where("id = ?", tagID).First(&tag).Error; err != nil {
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
		logger.Errorf("[笔记服务] 移除笔记-标签关联失败: %v", result.Error)
		return fmt.Errorf("failed to remove tag from note: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		tx.Rollback()
		return fmt.Errorf("tag not associated with note")
	}

	// 减少标签使用次数
	if err := tx.Model(&tag).Update("usage_count", gorm.Expr("CASE WHEN usage_count > 0 THEN usage_count - 1 ELSE 0 END")).Error; err != nil {
		tx.Rollback()
		logger.Errorf("[笔记服务] 减少标签使用次数失败: %v", err)
		return fmt.Errorf("failed to update tag usage count: %w", err)
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		logger.Errorf("[笔记服务] 提交移除笔记标签事务失败: %v", err)
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	logger.Infof("[笔记服务] 标签从笔记移除成功: %s -> %s", tagID, noteID)
	return nil
}

// SetNoteProperty 设置笔记扩展属性
func (s *noteService) SetNoteProperty(noteID string, key string, value interface{}, propertyType string) error {
	logger.Infof("[笔记服务] 为笔记设置属性 %s 到笔记 %s (类型: %s)", key, noteID, propertyType)

	// 开始事务
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 获取笔记
	var note database.Note
	if err := tx.Where("id = ?", noteID).First(&note).Error; err != nil {
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
		logger.Errorf("[笔记服务] 查询现有属性失败: %v", err)
		return fmt.Errorf("failed to query existing property: %w", err)
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 创建新属性
		property = database.NoteProperty{
			NoteID:        note.ID,
			PropertyKey:   key,
			DataType:      propertyType,
			PropertyValue: fmt.Sprintf("%v", value),
		}
	} else {
		// 更新现有属性
		property.DataType = propertyType
		property.PropertyValue = fmt.Sprintf("%v", value)
	}

	// 保存属性
	if err := tx.Save(&property).Error; err != nil {
		tx.Rollback()
		logger.Errorf("[笔记服务] 保存属性失败: %v", err)
		return fmt.Errorf("failed to save property: %w", err)
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		logger.Errorf("[笔记服务] 提交设置笔记属性事务失败: %v", err)
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	logger.Infof("[笔记服务] 属性设置成功: %s = %v", key, value)
	return nil
}

// GetNoteProperties 获取笔记的所有扩展属性
func (s *noteService) GetNoteProperties(noteID string) ([]database.NoteProperty, error) {
	logger.Infof("[笔记服务] 获取笔记的所有属性: %s", noteID)

	// 获取笔记
	var note database.Note
	if err := s.db.Where("id = ?", noteID).First(&note).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("note not found: %s", noteID)
		}
		return nil, err
	}

	// 获取属性
	var properties []database.NoteProperty
	if err := s.db.Where("note_id = ?", note.ID).Find(&properties).Error; err != nil {
		logger.Errorf("[笔记服务] 获取笔记属性失败: %v", err)
		return nil, fmt.Errorf("failed to get note properties: %w", err)
	}

	logger.Infof("[笔记服务] 找到 %d 个笔记属性: %s", len(properties), noteID)
	return properties, nil
}

// addNoteTags 添加笔记标签（内部方法）
func (s *noteService) addNoteTags(tx *gorm.DB, noteID uint, tagIDs []string) error {
	for _, tagID := range tagIDs {
		// 获取标签
		var tag database.Tag
		if err := tx.Where("id = ?", tagID).First(&tag).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				logger.Errorf("[笔记服务] 标签不存在: %s", tagID)
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
			logger.Errorf("[笔记服务] 增加标签使用次数失败: %v", err)
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
				NoteID:        noteID,
				PropertyKey:   key,
				DataType:      propertyType,
				PropertyValue: fmt.Sprintf("%v", value),
			}
		} else {
			// 更新现有属性
			property.DataType = propertyType
			property.PropertyValue = fmt.Sprintf("%v", value)
		}

		// 保存属性
		if err := tx.Save(&property).Error; err != nil {
			return fmt.Errorf("failed to save property: %w", err)
		}
	}

	return nil
}
