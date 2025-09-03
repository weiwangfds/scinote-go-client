// Package handler 提供标签管理相关的HTTP处理器
// 包含标签的创建、查询、更新、删除等API接口
// 支持标签搜索、批量操作和使用统计功能
package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/weiwangfds/scinote/internal/service/tag"
)

// TagHandler 标签处理器
// 处理所有标签相关的HTTP请求
type TagHandler struct {
	tagService tag.TagService
}

// NewTagHandler 创建标签处理器实例
// 参数:
//   tagService - 标签服务接口
// 返回:
//   *TagHandler - 标签处理器实例
func NewTagHandler(tagService tag.TagService) *TagHandler {
	return &TagHandler{
		tagService: tagService,
	}
}

// CreateTag 创建标签
// @Summary 创建新标签
// @Description 创建一个新的标签，标签名称必须唯一
// @Tags 标签管理
// @Accept json
// @Produce json
// @Param tag body tag.CreateTagRequest true "创建标签请求"
// @Success 201 {object} APIResponse{data=database.Tag} "创建成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 409 {object} APIResponse "标签名称已存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/tags [post]
func (h *TagHandler) CreateTag(c *gin.Context) {
	var req tag.CreateTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "请求参数错误",
			Error:   err.Error(),
		})
		return
	}

	createdTag, err := h.tagService.CreateTag(&req)
	if err != nil {
		if strings.Contains(err.Error(), "已存在") {
			c.JSON(http.StatusConflict, APIResponse{
				Success: false,
				Message: "标签创建失败",
				Error:   err.Error(),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Message: "标签创建失败",
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, APIResponse{
		Success: true,
		Message: "标签创建成功",
		Data:    createdTag,
	})
}

// GetTag 获取标签详情
// @Summary 获取标签详情
// @Description 根据标签ID获取标签的详细信息
// @Tags 标签管理
// @Accept json
// @Produce json
// @Param id path string true "标签ID"
// @Success 200 {object} APIResponse{data=database.Tag} "获取成功"
// @Failure 404 {object} APIResponse "标签不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/tags/{id} [get]
func (h *TagHandler) GetTag(c *gin.Context) {
	tagID := c.Param("id")
	if tagID == "" {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "标签ID不能为空",
		})
		return
	}

	tag, err := h.tagService.GetTagByID(tagID)
	if err != nil {
		if strings.Contains(err.Error(), "不存在") {
			c.JSON(http.StatusNotFound, APIResponse{
				Success: false,
				Message: "标签不存在",
				Error:   err.Error(),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Message: "获取标签失败",
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "获取标签成功",
		Data:    tag,
	})
}

// UpdateTag 更新标签
// @Summary 更新标签信息
// @Description 更新标签的名称、颜色或描述信息
// @Tags 标签管理
// @Accept json
// @Produce json
// @Param id path string true "标签ID"
// @Param tag body tag.UpdateTagRequest true "更新标签请求"
// @Success 200 {object} APIResponse{data=database.Tag} "更新成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 404 {object} APIResponse "标签不存在"
// @Failure 409 {object} APIResponse "标签名称已存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/tags/{id} [put]
func (h *TagHandler) UpdateTag(c *gin.Context) {
	tagID := c.Param("id")
	if tagID == "" {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "标签ID不能为空",
		})
		return
	}

	var req tag.UpdateTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "请求参数错误",
			Error:   err.Error(),
		})
		return
	}

	updatedTag, err := h.tagService.UpdateTag(tagID, &req)
	if err != nil {
		if strings.Contains(err.Error(), "不存在") {
			c.JSON(http.StatusNotFound, APIResponse{
				Success: false,
				Message: "标签不存在",
				Error:   err.Error(),
			})
			return
		}
		if strings.Contains(err.Error(), "已存在") {
			c.JSON(http.StatusConflict, APIResponse{
				Success: false,
				Message: "标签更新失败",
				Error:   err.Error(),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Message: "标签更新失败",
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "标签更新成功",
		Data:    updatedTag,
	})
}

// DeleteTag 删除标签
// @Summary 删除标签
// @Description 删除指定的标签，可选择是否强制删除（即使有关联笔记）
// @Tags 标签管理
// @Accept json
// @Produce json
// @Param id path string true "标签ID"
// @Param force query bool false "是否强制删除（默认false）"
// @Success 200 {object} APIResponse "删除成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 404 {object} APIResponse "标签不存在"
// @Failure 409 {object} APIResponse "标签仍有关联笔记"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/tags/{id} [delete]
func (h *TagHandler) DeleteTag(c *gin.Context) {
	tagID := c.Param("id")
	if tagID == "" {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "标签ID不能为空",
		})
		return
	}

	// 获取force参数
	force := c.Query("force") == "true"

	err := h.tagService.DeleteTag(tagID, force)
	if err != nil {
		if strings.Contains(err.Error(), "不存在") {
			c.JSON(http.StatusNotFound, APIResponse{
				Success: false,
				Message: "标签不存在",
				Error:   err.Error(),
			})
			return
		}
		if strings.Contains(err.Error(), "关联笔记") {
			c.JSON(http.StatusConflict, APIResponse{
				Success: false,
				Message: "标签删除失败",
				Error:   err.Error(),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Message: "标签删除失败",
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "标签删除成功",
	})
}

// GetAllTags 获取标签列表
// @Summary 获取标签列表
// @Description 获取所有标签的分页列表，支持排序
// @Tags 标签管理
// @Accept json
// @Produce json
// @Param page query int false "页码（默认1）"
// @Param page_size query int false "每页数量（默认20，最大100）"
// @Param sort_by query string false "排序字段（name、usage_count、created_at、updated_at，默认created_at）"
// @Param sort_order query string false "排序方向（asc、desc，默认desc）"
// @Success 200 {object} APIResponse{data=PaginatedResponse} "获取成功"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/tags [get]
func (h *TagHandler) GetAllTags(c *gin.Context) {
	// 解析分页参数
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	sortBy := c.DefaultQuery("sort_by", "created_at")
	sortOrder := c.DefaultQuery("sort_order", "desc")

	tags, total, err := h.tagService.GetAllTags(page, pageSize, sortBy, sortOrder)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Message: "获取标签列表失败",
			Error:   err.Error(),
		})
		return
	}

	// 计算总页数
	totalPages := (total + int64(pageSize) - 1) / int64(pageSize)

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "获取标签列表成功",
		Data: PaginatedResponse{
			Data:       tags,
			Total:      total,
			Page:       page,
			PageSize:   pageSize,
			TotalPages: totalPages,
		},
	})
}

// SearchTags 搜索标签
// @Summary 搜索标签
// @Description 根据关键词搜索标签名称和描述
// @Tags 标签管理
// @Accept json
// @Produce json
// @Param q query string true "搜索关键词"
// @Param page query int false "页码（默认1）"
// @Param page_size query int false "每页数量（默认20，最大100）"
// @Success 200 {object} APIResponse{data=PaginatedResponse} "搜索成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/tags/search [get]
func (h *TagHandler) SearchTags(c *gin.Context) {
	query := strings.TrimSpace(c.Query("q"))
	if query == "" {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "搜索关键词不能为空",
		})
		return
	}

	// 解析分页参数
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	tags, total, err := h.tagService.SearchTags(query, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Message: "搜索标签失败",
			Error:   err.Error(),
		})
		return
	}

	// 计算总页数
	totalPages := (total + int64(pageSize) - 1) / int64(pageSize)

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "搜索标签成功",
		Data: PaginatedResponse{
			Data:       tags,
			Total:      total,
			Page:       page,
			PageSize:   pageSize,
			TotalPages: totalPages,
		},
	})
}

// GetPopularTags 获取热门标签
// @Summary 获取热门标签
// @Description 获取使用次数最多的标签列表
// @Tags 标签管理
// @Accept json
// @Produce json
// @Param limit query int false "返回数量限制（默认10，最大100）"
// @Success 200 {object} APIResponse{data=[]database.Tag} "获取成功"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/tags/popular [get]
func (h *TagHandler) GetPopularTags(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	tags, err := h.tagService.GetPopularTags(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Message: "获取热门标签失败",
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "获取热门标签成功",
		Data:    tags,
	})
}

// BatchCreateTags 批量创建标签
// @Summary 批量创建标签
// @Description 批量创建多个标签，自动去重和跳过已存在的标签
// @Tags 标签管理
// @Accept json
// @Produce json
// @Param tags body BatchCreateTagsRequest true "批量创建标签请求"
// @Success 201 {object} APIResponse{data=[]database.Tag} "创建成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/tags/batch [post]
func (h *TagHandler) BatchCreateTags(c *gin.Context) {
	var req BatchCreateTagsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "请求参数错误",
			Error:   err.Error(),
		})
		return
	}

	if len(req.Names) == 0 {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "标签名称列表不能为空",
		})
		return
	}

	tags, err := h.tagService.BatchCreateTags(req.Names)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Message: "批量创建标签失败",
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, APIResponse{
		Success: true,
		Message: "批量创建标签成功",
		Data:    tags,
	})
}

// GetTagUsageStats 获取标签使用统计
// @Summary 获取标签使用统计
// @Description 获取标签的详细使用统计信息
// @Tags 标签管理
// @Accept json
// @Produce json
// @Param id path string true "标签ID"
// @Success 200 {object} APIResponse{data=tag.TagUsageStats} "获取成功"
// @Failure 404 {object} APIResponse "标签不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/tags/{id}/stats [get]
func (h *TagHandler) GetTagUsageStats(c *gin.Context) {
	tagID := c.Param("id")
	if tagID == "" {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "标签ID不能为空",
		})
		return
	}

	stats, err := h.tagService.GetTagUsageStats(tagID)
	if err != nil {
		if strings.Contains(err.Error(), "不存在") {
			c.JSON(http.StatusNotFound, APIResponse{
				Success: false,
				Message: "标签不存在",
				Error:   err.Error(),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Message: "获取标签统计失败",
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "获取标签统计成功",
		Data:    stats,
	})
}

// BatchCreateTagsRequest 批量创建标签请求
type BatchCreateTagsRequest struct {
	Names []string `json:"names" binding:"required,min=1"` // 标签名称列表
}