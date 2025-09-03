// Package handler 提供笔记管理相关的HTTP处理器
// 实现完整的RESTful API接口，支持笔记的增删改查和高级操作
package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/weiwangfds/scinote/internal/service/note"
)

// NoteHandler 笔记处理器
// 提供笔记管理的HTTP接口，包括CRUD操作、搜索、标签管理等功能
type NoteHandler struct {
	noteService note.NoteService
}

// NewNoteHandler 创建笔记处理器实例
// 参数:
//   noteService - 笔记服务接口
// 返回:
//   *NoteHandler - 笔记处理器实例
func NewNoteHandler(noteService note.NoteService) *NoteHandler {
	return &NoteHandler{
		noteService: noteService,
	}
}

// CreateNote 创建笔记
// @Summary 创建新笔记
// @Description 创建一个新的笔记，支持层级结构、标签和扩展属性
// @Tags 笔记管理
// @Accept json
// @Produce json
// @Param note body note.CreateNoteRequest true "创建笔记请求"
// @Success 201 {object} APIResponse{data=database.Note} "创建成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/notes [post]
func (h *NoteHandler) CreateNote(c *gin.Context) {
	var req note.CreateNoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "Invalid request parameters",
			Error:   err.Error(),
		})
		return
	}

	// 从上下文获取用户ID（假设已通过中间件设置）
	// 如果上下文中没有用户ID，则使用请求中的CreatorID
	if userID, exists := c.Get("user_id"); exists {
		req.CreatorID = userID.(string)
	} else if req.CreatorID == "" {
		c.JSON(http.StatusUnauthorized, APIResponse{
			Success: false,
			Message: "User not authenticated or creator_id not provided",
		})
		return
	}

	createdNote, err := h.noteService.CreateNote(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Message: "Failed to create note",
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, APIResponse{
		Success: true,
		Message: "Note created successfully",
		Data:    createdNote,
	})
}

// GetNote 获取笔记详情
// @Summary 获取笔记详情
// @Description 根据笔记ID获取笔记的详细信息
// @Tags 笔记管理
// @Accept json
// @Produce json
// @Param id path string true "笔记ID"
// @Param include_content query bool false "是否包含文件内容" default(false)
// @Success 200 {object} APIResponse{data=database.Note} "获取成功"
// @Failure 404 {object} APIResponse "笔记不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/notes/{id} [get]
func (h *NoteHandler) GetNote(c *gin.Context) {
	noteID := c.Param("id")
	if noteID == "" {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "Note ID is required",
		})
		return
	}

	// 解析是否包含内容参数
	includeContent := false
	if includeStr := c.Query("include_content"); includeStr != "" {
		includeContent, _ = strconv.ParseBool(includeStr)
	}

	note, err := h.noteService.GetNoteByID(noteID, includeContent)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, APIResponse{
				Success: false,
				Message: "Note not found",
				Error:   err.Error(),
			})
		} else {
			c.JSON(http.StatusInternalServerError, APIResponse{
				Success: false,
				Message: "Failed to get note",
				Error:   err.Error(),
			})
		}
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "Note retrieved successfully",
		Data:    note,
	})
}

// UpdateNote 更新笔记
// @Summary 更新笔记信息
// @Description 更新笔记的基本信息、内容、标签等
// @Tags 笔记管理
// @Accept json
// @Produce json
// @Param id path string true "笔记ID"
// @Param note body note.UpdateNoteRequest true "更新笔记请求"
// @Success 200 {object} APIResponse{data=database.Note} "更新成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 404 {object} APIResponse "笔记不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/notes/{id} [put]
func (h *NoteHandler) UpdateNote(c *gin.Context) {
	noteID := c.Param("id")
	if noteID == "" {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "Note ID is required",
		})
		return
	}

	var req note.UpdateNoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "Invalid request parameters",
			Error:   err.Error(),
		})
		return
	}

	// 从上下文获取用户ID
	if userID, exists := c.Get("user_id"); exists {
		req.UpdaterID = userID.(string)
	} else {
		c.JSON(http.StatusUnauthorized, APIResponse{
			Success: false,
			Message: "User not authenticated",
		})
		return
	}

	updatedNote, err := h.noteService.UpdateNote(noteID, &req)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, APIResponse{
				Success: false,
				Message: "Note not found",
				Error:   err.Error(),
			})
		} else {
			c.JSON(http.StatusInternalServerError, APIResponse{
				Success: false,
				Message: "Failed to update note",
				Error:   err.Error(),
			})
		}
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "Note updated successfully",
		Data:    updatedNote,
	})
}

// DeleteNote 删除笔记
// @Summary 删除笔记
// @Description 软删除笔记，支持级联删除子笔记
// @Tags 笔记管理
// @Accept json
// @Produce json
// @Param id path string true "笔记ID"
// @Param cascade query bool false "是否级联删除子笔记" default(false)
// @Success 200 {object} APIResponse "删除成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 404 {object} APIResponse "笔记不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/notes/{id} [delete]
func (h *NoteHandler) DeleteNote(c *gin.Context) {
	noteID := c.Param("id")
	if noteID == "" {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "Note ID is required",
		})
		return
	}

	// 解析级联删除参数
	cascade := false
	if cascadeStr := c.Query("cascade"); cascadeStr != "" {
		cascade, _ = strconv.ParseBool(cascadeStr)
	}

	err := h.noteService.DeleteNote(noteID, cascade)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, APIResponse{
				Success: false,
				Message: "Note not found",
				Error:   err.Error(),
			})
		} else if strings.Contains(err.Error(), "cannot delete note with children") {
			c.JSON(http.StatusBadRequest, APIResponse{
				Success: false,
				Message: "Cannot delete note with children",
				Error:   err.Error(),
			})
		} else {
			c.JSON(http.StatusInternalServerError, APIResponse{
				Success: false,
				Message: "Failed to delete note",
				Error:   err.Error(),
			})
		}
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "Note deleted successfully",
	})
}

// GetNoteChildren 获取笔记的子笔记
// @Summary 获取笔记的子笔记
// @Description 获取指定笔记的直接子笔记列表，支持分页
// @Tags 笔记管理
// @Accept json
// @Produce json
// @Param id path string false "父笔记ID，空表示获取根笔记"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200 {object} APIResponse{data=PaginatedResponse} "获取成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/notes/{id}/children [get]
// @Router /api/notes/children [get]
func (h *NoteHandler) GetNoteChildren(c *gin.Context) {
	noteID := c.Param("id")

	// 解析分页参数
	page := 1
	pageSize := 20

	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if sizeStr := c.Query("page_size"); sizeStr != "" {
		if s, err := strconv.Atoi(sizeStr); err == nil && s > 0 && s <= 100 {
			pageSize = s
		}
	}

	notes, total, err := h.noteService.GetNoteChildren(noteID, page, pageSize)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, APIResponse{
				Success: false,
				Message: "Parent note not found",
				Error:   err.Error(),
			})
		} else {
			c.JSON(http.StatusInternalServerError, APIResponse{
				Success: false,
				Message: "Failed to get note children",
				Error:   err.Error(),
			})
		}
		return
	}

	response := PaginatedResponse{
		Data:       notes,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: (total + int64(pageSize) - 1) / int64(pageSize),
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "Note children retrieved successfully",
		Data:    response,
	})
}

// GetNoteTree 获取笔记树结构
// @Summary 获取笔记树结构
// @Description 获取完整的笔记树结构，支持指定根节点和最大深度
// @Tags 笔记管理
// @Accept json
// @Produce json
// @Param root_id query string false "根笔记ID，空表示从顶级开始"
// @Param max_depth query int false "最大深度，0表示无限制" default(0)
// @Success 200 {object} APIResponse{data=[]database.Note} "获取成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/notes/tree [get]
func (h *NoteHandler) GetNoteTree(c *gin.Context) {
	rootID := c.Query("root_id")
	maxDepth := 0

	if depthStr := c.Query("max_depth"); depthStr != "" {
		if d, err := strconv.Atoi(depthStr); err == nil && d >= 0 {
			maxDepth = d
		}
	}

	notes, err := h.noteService.GetNoteTree(rootID, maxDepth)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, APIResponse{
				Success: false,
				Message: "Root note not found",
				Error:   err.Error(),
			})
		} else {
			c.JSON(http.StatusInternalServerError, APIResponse{
				Success: false,
				Message: "Failed to get note tree",
				Error:   err.Error(),
			})
		}
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "Note tree retrieved successfully",
		Data:    notes,
	})
}

// MoveNote 移动笔记
// @Summary 移动笔记
// @Description 将笔记移动到新的父笔记下，并设置新的排序位置
// @Tags 笔记管理
// @Accept json
// @Produce json
// @Param id path string true "笔记ID"
// @Param request body MoveNoteRequest true "移动笔记请求"
// @Success 200 {object} APIResponse "移动成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 404 {object} APIResponse "笔记不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/notes/{id}/move [post]
func (h *NoteHandler) MoveNote(c *gin.Context) {
	noteID := c.Param("id")
	if noteID == "" {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "Note ID is required",
		})
		return
	}

	var req MoveNoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "Invalid request parameters",
			Error:   err.Error(),
		})
		return
	}

	err := h.noteService.MoveNote(noteID, req.NewParentID, req.NewSortOrder)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, APIResponse{
				Success: false,
				Message: "Note not found",
				Error:   err.Error(),
			})
		} else if strings.Contains(err.Error(), "circular reference") || strings.Contains(err.Error(), "descendant") {
			c.JSON(http.StatusBadRequest, APIResponse{
				Success: false,
				Message: "Invalid move operation",
				Error:   err.Error(),
			})
		} else {
			c.JSON(http.StatusInternalServerError, APIResponse{
				Success: false,
				Message: "Failed to move note",
				Error:   err.Error(),
			})
		}
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "Note moved successfully",
	})
}

// BatchMoveNotes 批量移动笔记
// @Summary 批量移动笔记
// @Description 将多个笔记批量移动到新的父笔记下
// @Tags 笔记管理
// @Accept json
// @Produce json
// @Param request body BatchMoveNotesRequest true "批量移动笔记请求"
// @Success 200 {object} APIResponse "移动成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/notes/batch-move [post]
func (h *NoteHandler) BatchMoveNotes(c *gin.Context) {
	var req BatchMoveNotesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "Invalid request parameters",
			Error:   err.Error(),
		})
		return
	}

	if len(req.NoteIDs) == 0 {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "Note IDs are required",
		})
		return
	}

	err := h.noteService.BatchMoveNotes(req.NoteIDs, req.NewParentID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, APIResponse{
				Success: false,
				Message: "Parent note not found",
				Error:   err.Error(),
			})
		} else {
			c.JSON(http.StatusInternalServerError, APIResponse{
				Success: false,
				Message: "Failed to batch move notes",
				Error:   err.Error(),
			})
		}
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "Notes moved successfully",
	})
}

// MoveNoteTree 移动笔记树
// @Summary 移动笔记树
// @Description 移动整个笔记树到新的父笔记下
// @Tags 笔记管理
// @Accept json
// @Produce json
// @Param id path string true "根笔记ID"
// @Param request body MoveNoteTreeRequest true "移动笔记树请求"
// @Success 200 {object} APIResponse "移动成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 404 {object} APIResponse "笔记不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/notes/{id}/move-tree [post]
func (h *NoteHandler) MoveNoteTree(c *gin.Context) {
	rootNoteID := c.Param("id")
	if rootNoteID == "" {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "Root note ID is required",
		})
		return
	}

	var req MoveNoteTreeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "Invalid request parameters",
			Error:   err.Error(),
		})
		return
	}

	err := h.noteService.MoveNoteTree(rootNoteID, req.NewParentID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, APIResponse{
				Success: false,
				Message: "Note not found",
				Error:   err.Error(),
			})
		} else {
			c.JSON(http.StatusInternalServerError, APIResponse{
				Success: false,
				Message: "Failed to move note tree",
				Error:   err.Error(),
			})
		}
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "Note tree moved successfully",
	})
}

// SearchNotes 搜索笔记
// @Summary 搜索笔记
// @Description 根据关键词搜索笔记，支持分页
// @Tags 笔记管理
// @Accept json
// @Produce json
// @Param q query string true "搜索关键词"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200 {object} APIResponse{data=PaginatedResponse} "搜索成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/notes/search [get]
func (h *NoteHandler) SearchNotes(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "Search query is required",
		})
		return
	}

	// 解析分页参数
	page := 1
	pageSize := 20

	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if sizeStr := c.Query("page_size"); sizeStr != "" {
		if s, err := strconv.Atoi(sizeStr); err == nil && s > 0 && s <= 100 {
			pageSize = s
		}
	}

	notes, total, err := h.noteService.SearchNotes(query, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Message: "Failed to search notes",
			Error:   err.Error(),
		})
		return
	}

	response := PaginatedResponse{
		Data:       notes,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: (total + int64(pageSize) - 1) / int64(pageSize),
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "Notes searched successfully",
		Data:    response,
	})
}

// AddNoteTag 为笔记添加标签
// @Summary 为笔记添加标签
// @Description 为指定笔记添加标签
// @Tags 笔记管理
// @Accept json
// @Produce json
// @Param id path string true "笔记ID"
// @Param request body AddNoteTagRequest true "添加标签请求"
// @Success 200 {object} APIResponse "添加成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 404 {object} APIResponse "笔记或标签不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/notes/{id}/tags [post]
func (h *NoteHandler) AddNoteTag(c *gin.Context) {
	noteID := c.Param("id")
	if noteID == "" {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "Note ID is required",
		})
		return
	}

	var req AddNoteTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "Invalid request parameters",
			Error:   err.Error(),
		})
		return
	}

	err := h.noteService.AddNoteTag(noteID, req.TagID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, APIResponse{
				Success: false,
				Message: "Note or tag not found",
				Error:   err.Error(),
			})
		} else if strings.Contains(err.Error(), "already associated") {
			c.JSON(http.StatusBadRequest, APIResponse{
				Success: false,
				Message: "Tag already associated with note",
				Error:   err.Error(),
			})
		} else {
			c.JSON(http.StatusInternalServerError, APIResponse{
				Success: false,
				Message: "Failed to add tag to note",
				Error:   err.Error(),
			})
		}
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "Tag added to note successfully",
	})
}

// RemoveNoteTag 移除笔记标签
// @Summary 移除笔记标签
// @Description 从指定笔记移除标签
// @Tags 笔记管理
// @Accept json
// @Produce json
// @Param id path string true "笔记ID"
// @Param tag_id path string true "标签ID"
// @Success 200 {object} APIResponse "移除成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 404 {object} APIResponse "笔记或标签不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/notes/{id}/tags/{tag_id} [delete]
func (h *NoteHandler) RemoveNoteTag(c *gin.Context) {
	noteID := c.Param("id")
	tagID := c.Param("tag_id")

	if noteID == "" || tagID == "" {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "Note ID and tag ID are required",
		})
		return
	}

	err := h.noteService.RemoveNoteTag(noteID, tagID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, APIResponse{
				Success: false,
				Message: "Note or tag not found",
				Error:   err.Error(),
			})
		} else if strings.Contains(err.Error(), "not associated") {
			c.JSON(http.StatusBadRequest, APIResponse{
				Success: false,
				Message: "Tag not associated with note",
				Error:   err.Error(),
			})
		} else {
			c.JSON(http.StatusInternalServerError, APIResponse{
				Success: false,
				Message: "Failed to remove tag from note",
				Error:   err.Error(),
			})
		}
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "Tag removed from note successfully",
	})
}

// SetNoteProperty 设置笔记扩展属性
// @Summary 设置笔记扩展属性
// @Description 为指定笔记设置扩展属性
// @Tags 笔记管理
// @Accept json
// @Produce json
// @Param id path string true "笔记ID"
// @Param request body SetNotePropertyRequest true "设置属性请求"
// @Success 200 {object} APIResponse "设置成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 404 {object} APIResponse "笔记不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/notes/{id}/properties [post]
func (h *NoteHandler) SetNoteProperty(c *gin.Context) {
	noteID := c.Param("id")
	if noteID == "" {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "Note ID is required",
		})
		return
	}

	var req SetNotePropertyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "Invalid request parameters",
			Error:   err.Error(),
		})
		return
	}

	err := h.noteService.SetNoteProperty(noteID, req.Key, req.Value, req.PropertyType)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, APIResponse{
				Success: false,
				Message: "Note not found",
				Error:   err.Error(),
			})
		} else {
			c.JSON(http.StatusInternalServerError, APIResponse{
				Success: false,
				Message: "Failed to set note property",
				Error:   err.Error(),
			})
		}
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "Note property set successfully",
	})
}

// GetNoteProperties 获取笔记扩展属性
// @Summary 获取笔记扩展属性
// @Description 获取指定笔记的所有扩展属性
// @Tags 笔记管理
// @Accept json
// @Produce json
// @Param id path string true "笔记ID"
// @Success 200 {object} APIResponse{data=[]database.NoteProperty} "获取成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 404 {object} APIResponse "笔记不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/notes/{id}/properties [get]
func (h *NoteHandler) GetNoteProperties(c *gin.Context) {
	noteID := c.Param("id")
	if noteID == "" {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "Note ID is required",
		})
		return
	}

	properties, err := h.noteService.GetNoteProperties(noteID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, APIResponse{
				Success: false,
				Message: "Note not found",
				Error:   err.Error(),
			})
		} else {
			c.JSON(http.StatusInternalServerError, APIResponse{
				Success: false,
				Message: "Failed to get note properties",
				Error:   err.Error(),
			})
		}
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "Note properties retrieved successfully",
		Data:    properties,
	})
}

// 请求和响应结构体定义

// APIResponse 统一API响应格式
type APIResponse struct {
	Success bool        `json:"success"`           // 请求是否成功
	Message string      `json:"message"`           // 响应消息
	Data    interface{} `json:"data,omitempty"`    // 响应数据
	Error   string      `json:"error,omitempty"`   // 错误信息
}

// PaginatedResponse 分页响应格式
type PaginatedResponse struct {
	Data       interface{} `json:"data"`        // 数据列表
	Total      int64       `json:"total"`       // 总数量
	Page       int         `json:"page"`        // 当前页码
	PageSize   int         `json:"page_size"`   // 每页数量
	TotalPages int64       `json:"total_pages"` // 总页数
}

// MoveNoteRequest 移动笔记请求
type MoveNoteRequest struct {
	NewParentID  string `json:"new_parent_id"`  // 新父笔记ID
	NewSortOrder int    `json:"new_sort_order"` // 新排序位置
}

// BatchMoveNotesRequest 批量移动笔记请求
type BatchMoveNotesRequest struct {
	NoteIDs     []string `json:"note_ids" binding:"required,min=1"`     // 笔记ID列表
	NewParentID string   `json:"new_parent_id"`                         // 新父笔记ID
}

// MoveNoteTreeRequest 移动笔记树请求
type MoveNoteTreeRequest struct {
	NewParentID string `json:"new_parent_id"` // 新父笔记ID
}

// AddNoteTagRequest 添加笔记标签请求
type AddNoteTagRequest struct {
	TagID string `json:"tag_id" binding:"required"` // 标签ID
}

// SetNotePropertyRequest 设置笔记属性请求
type SetNotePropertyRequest struct {
	Key          string      `json:"key" binding:"required"`           // 属性键
	Value        interface{} `json:"value" binding:"required"`         // 属性值
	PropertyType string      `json:"property_type" binding:"required"` // 属性类型
}