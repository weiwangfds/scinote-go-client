// Package tag 提供标签管理相关的业务逻辑服务
// 包含标签的创建、查询、更新、删除等核心功能
// 支持标签的使用统计和批量操作
package tag

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/weiwangfds/scinote/internal/database"
	"gorm.io/gorm"
)

// TagService 标签服务接口
// 定义了标签管理的所有业务操作方法
type TagService interface {
	// CreateTag 创建新标签
	// 参数:
	//   req - 创建标签请求
	// 返回:
	//   *database.Tag - 创建的标签对象
	//   error - 错误信息
	CreateTag(req *CreateTagRequest) (*database.Tag, error)

	// GetTagByID 根据ID获取标签
	// 参数:
	//   tagID - 标签ID
	// 返回:
	//   *database.Tag - 标签对象
	//   error - 错误信息
	GetTagByID(tagID string) (*database.Tag, error)

	// GetTagByName 根据名称获取标签
	// 参数:
	//   name - 标签名称
	// 返回:
	//   *database.Tag - 标签对象
	//   error - 错误信息
	GetTagByName(name string) (*database.Tag, error)

	// UpdateTag 更新标签信息
	// 参数:
	//   tagID - 标签ID
	//   req - 更新标签请求
	// 返回:
	//   *database.Tag - 更新后的标签对象
	//   error - 错误信息
	UpdateTag(tagID string, req *UpdateTagRequest) (*database.Tag, error)

	// DeleteTag 删除标签
	// 参数:
	//   tagID - 标签ID
	//   force - 是否强制删除（即使有关联的笔记）
	// 返回:
	//   error - 错误信息
	DeleteTag(tagID string, force bool) error

	// GetAllTags 获取所有标签列表
	// 参数:
	//   page - 页码（从1开始）
	//   pageSize - 每页数量
	//   sortBy - 排序字段（name、usage_count、created_at）
	//   sortOrder - 排序方向（asc、desc）
	// 返回:
	//   []database.Tag - 标签列表
	//   int64 - 总数量
	//   error - 错误信息
	GetAllTags(page, pageSize int, sortBy, sortOrder string) ([]database.Tag, int64, error)

	// SearchTags 搜索标签
	// 参数:
	//   query - 搜索关键词
	//   page - 页码（从1开始）
	//   pageSize - 每页数量
	// 返回:
	//   []database.Tag - 标签列表
	//   int64 - 总数量
	//   error - 错误信息
	SearchTags(query string, page, pageSize int) ([]database.Tag, int64, error)

	// GetPopularTags 获取热门标签
	// 参数:
	//   limit - 返回数量限制
	// 返回:
	//   []database.Tag - 标签列表
	//   error - 错误信息
	GetPopularTags(limit int) ([]database.Tag, error)

	// BatchCreateTags 批量创建标签
	// 参数:
	//   names - 标签名称列表
	// 返回:
	//   []database.Tag - 创建的标签列表
	//   error - 错误信息
	BatchCreateTags(names []string) ([]database.Tag, error)

	// GetTagUsageStats 获取标签使用统计
	// 参数:
	//   tagID - 标签ID
	// 返回:
	//   *TagUsageStats - 使用统计信息
	//   error - 错误信息
	GetTagUsageStats(tagID string) (*TagUsageStats, error)
}

// CreateTagRequest 创建标签请求
type CreateTagRequest struct {
	Name        string `json:"name" binding:"required,max=100"`        // 标签名称
	Color       string `json:"color" binding:"max=20"`                 // 标签颜色
	Description string `json:"description" binding:"max=500"`          // 标签描述
}

// UpdateTagRequest 更新标签请求
type UpdateTagRequest struct {
	Name        *string `json:"name" binding:"omitempty,max=100"`        // 标签名称
	Color       *string `json:"color" binding:"omitempty,max=20"`        // 标签颜色
	Description *string `json:"description" binding:"omitempty,max=500"` // 标签描述
}

// TagUsageStats 标签使用统计
type TagUsageStats struct {
	TagID       string    `json:"tag_id"`       // 标签ID
	TagName     string    `json:"tag_name"`     // 标签名称
	UsageCount  int64     `json:"usage_count"`  // 使用次数
	NoteCount   int64     `json:"note_count"`   // 关联笔记数量
	LastUsedAt  time.Time `json:"last_used_at"` // 最后使用时间
	CreatedAt   time.Time `json:"created_at"`   // 创建时间
}

// tagService 标签服务实现
type tagService struct {
	db *gorm.DB
}

// NewTagService 创建标签服务实例
// 参数:
//   db - 数据库连接
// 返回:
//   TagService - 标签服务接口实例
func NewTagService(db *gorm.DB) TagService {
	return &tagService{
		db: db,
	}
}

// CreateTag 创建新标签
func (s *tagService) CreateTag(req *CreateTagRequest) (*database.Tag, error) {
	// 检查标签名称是否已存在
	var existingTag database.Tag
	if err := s.db.Where("name = ?", req.Name).First(&existingTag).Error; err == nil {
		return nil, fmt.Errorf("标签名称 '%s' 已存在", req.Name)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("检查标签名称时发生错误: %v", err)
	}

	// 创建新标签
	tag := &database.Tag{
		TagID:       uuid.New().String(),
		Name:        strings.TrimSpace(req.Name),
		Color:       req.Color,
		Description: req.Description,
		UsageCount:  0,
	}

	// 设置默认颜色
	if tag.Color == "" {
		tag.Color = "#gray"
	}

	if err := s.db.Create(tag).Error; err != nil {
		return nil, fmt.Errorf("创建标签失败: %v", err)
	}

	return tag, nil
}

// GetTagByID 根据ID获取标签
func (s *tagService) GetTagByID(tagID string) (*database.Tag, error) {
	var tag database.Tag
	if err := s.db.Where("tag_id = ?", tagID).First(&tag).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("标签不存在")
		}
		return nil, fmt.Errorf("获取标签失败: %v", err)
	}
	return &tag, nil
}

// GetTagByName 根据名称获取标签
func (s *tagService) GetTagByName(name string) (*database.Tag, error) {
	var tag database.Tag
	if err := s.db.Where("name = ?", name).First(&tag).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("标签不存在")
		}
		return nil, fmt.Errorf("获取标签失败: %v", err)
	}
	return &tag, nil
}

// UpdateTag 更新标签信息
func (s *tagService) UpdateTag(tagID string, req *UpdateTagRequest) (*database.Tag, error) {
	// 获取现有标签
	tag, err := s.GetTagByID(tagID)
	if err != nil {
		return nil, err
	}

	// 如果要更新名称，检查新名称是否已存在
	if req.Name != nil && *req.Name != tag.Name {
		var existingTag database.Tag
		if err := s.db.Where("name = ? AND tag_id != ?", *req.Name, tagID).First(&existingTag).Error; err == nil {
			return nil, fmt.Errorf("标签名称 '%s' 已存在", *req.Name)
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("检查标签名称时发生错误: %v", err)
		}
	}

	// 更新字段
	updates := make(map[string]interface{})
	if req.Name != nil {
		updates["name"] = strings.TrimSpace(*req.Name)
	}
	if req.Color != nil {
		updates["color"] = *req.Color
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}

	if len(updates) > 0 {
		if err := s.db.Model(tag).Updates(updates).Error; err != nil {
			return nil, fmt.Errorf("更新标签失败: %v", err)
		}
	}

	// 重新获取更新后的标签
	return s.GetTagByID(tagID)
}

// DeleteTag 删除标签
func (s *tagService) DeleteTag(tagID string, force bool) error {
	// 检查标签是否存在
	tag, err := s.GetTagByID(tagID)
	if err != nil {
		return err
	}

	// 检查是否有关联的笔记
	var noteCount int64
	if err := s.db.Table("note_tags").Where("tag_id = ?", tag.ID).Count(&noteCount).Error; err != nil {
		return fmt.Errorf("检查标签关联笔记时发生错误: %v", err)
	}

	if noteCount > 0 && !force {
		return fmt.Errorf("标签仍有 %d 个关联笔记，无法删除。如需强制删除，请设置 force=true", noteCount)
	}

	// 开始事务
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 删除所有关联关系
	if err := tx.Where("tag_id = ?", tag.ID).Delete(&database.NoteTag{}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("删除标签关联关系失败: %v", err)
	}

	// 删除标签
	if err := tx.Delete(tag).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("删除标签失败: %v", err)
	}

	return tx.Commit().Error
}

// GetAllTags 获取所有标签列表
func (s *tagService) GetAllTags(page, pageSize int, sortBy, sortOrder string) ([]database.Tag, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	// 验证排序字段
	allowedSortFields := map[string]bool{
		"name":        true,
		"usage_count": true,
		"created_at":  true,
		"updated_at":  true,
	}
	if !allowedSortFields[sortBy] {
		sortBy = "created_at"
	}

	// 验证排序方向
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "desc"
	}

	var tags []database.Tag
	var total int64

	// 获取总数
	if err := s.db.Model(&database.Tag{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("获取标签总数失败: %v", err)
	}

	// 获取分页数据
	offset := (page - 1) * pageSize
	orderClause := fmt.Sprintf("%s %s", sortBy, sortOrder)
	if err := s.db.Order(orderClause).Offset(offset).Limit(pageSize).Find(&tags).Error; err != nil {
		return nil, 0, fmt.Errorf("获取标签列表失败: %v", err)
	}

	return tags, total, nil
}

// SearchTags 搜索标签
func (s *tagService) SearchTags(query string, page, pageSize int) ([]database.Tag, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	query = strings.TrimSpace(query)
	if query == "" {
		return s.GetAllTags(page, pageSize, "created_at", "desc")
	}

	var tags []database.Tag
	var total int64

	// 构建搜索条件
	searchPattern := "%" + query + "%"
	db := s.db.Where("name LIKE ? OR description LIKE ?", searchPattern, searchPattern)

	// 获取总数
	if err := db.Model(&database.Tag{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("获取搜索结果总数失败: %v", err)
	}

	// 获取分页数据
	offset := (page - 1) * pageSize
	if err := db.Order("usage_count DESC, name ASC").Offset(offset).Limit(pageSize).Find(&tags).Error; err != nil {
		return nil, 0, fmt.Errorf("搜索标签失败: %v", err)
	}

	return tags, total, nil
}

// GetPopularTags 获取热门标签
func (s *tagService) GetPopularTags(limit int) ([]database.Tag, error) {
	if limit < 1 || limit > 100 {
		limit = 10
	}

	var tags []database.Tag
	if err := s.db.Where("usage_count > 0").Order("usage_count DESC, name ASC").Limit(limit).Find(&tags).Error; err != nil {
		return nil, fmt.Errorf("获取热门标签失败: %v", err)
	}

	return tags, nil
}

// BatchCreateTags 批量创建标签
func (s *tagService) BatchCreateTags(names []string) ([]database.Tag, error) {
	if len(names) == 0 {
		return []database.Tag{}, nil
	}

	// 去重和清理名称
	nameSet := make(map[string]bool)
	cleanNames := make([]string, 0, len(names))
	for _, name := range names {
		cleanName := strings.TrimSpace(name)
		if cleanName != "" && !nameSet[cleanName] {
			nameSet[cleanName] = true
			cleanNames = append(cleanNames, cleanName)
		}
	}

	if len(cleanNames) == 0 {
		return []database.Tag{}, nil
	}

	// 检查已存在的标签
	var existingTags []database.Tag
	if err := s.db.Where("name IN ?", cleanNames).Find(&existingTags).Error; err != nil {
		return nil, fmt.Errorf("检查已存在标签失败: %v", err)
	}

	// 构建已存在标签的映射
	existingNameSet := make(map[string]bool)
	for _, tag := range existingTags {
		existingNameSet[tag.Name] = true
	}

	// 创建新标签
	newTags := make([]database.Tag, 0)
	for _, name := range cleanNames {
		if !existingNameSet[name] {
			newTags = append(newTags, database.Tag{
				TagID:       uuid.New().String(),
				Name:        name,
				Color:       "#gray",
				Description: "",
				UsageCount:  0,
			})
		}
	}

	// 批量插入新标签
	if len(newTags) > 0 {
		if err := s.db.Create(&newTags).Error; err != nil {
			return nil, fmt.Errorf("批量创建标签失败: %v", err)
		}
	}

	// 返回所有相关标签（包括已存在的）
	allTags := append(existingTags, newTags...)
	return allTags, nil
}

// GetTagUsageStats 获取标签使用统计
func (s *tagService) GetTagUsageStats(tagID string) (*TagUsageStats, error) {
	tag, err := s.GetTagByID(tagID)
	if err != nil {
		return nil, err
	}

	// 获取关联笔记数量
	var noteCount int64
	if err := s.db.Table("note_tags").Where("tag_id = ?", tag.ID).Count(&noteCount).Error; err != nil {
		return nil, fmt.Errorf("获取关联笔记数量失败: %v", err)
	}

	// 获取最后使用时间（最近一次被添加到笔记的时间）
	var lastUsedAt time.Time
	if err := s.db.Table("note_tags").Where("tag_id = ?", tag.ID).Select("MAX(created_at)").Scan(&lastUsedAt).Error; err != nil {
		return nil, fmt.Errorf("获取最后使用时间失败: %v", err)
	}

	return &TagUsageStats{
		TagID:      tag.TagID,
		TagName:    tag.Name,
		UsageCount: tag.UsageCount,
		NoteCount:  noteCount,
		LastUsedAt: lastUsedAt,
		CreatedAt:  tag.CreatedAt,
	}, nil
}