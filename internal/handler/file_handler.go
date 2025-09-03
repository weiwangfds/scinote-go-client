package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/weiwangfds/scinote/internal/errors"
	"github.com/weiwangfds/scinote/internal/response"
	fileservice "github.com/weiwangfds/scinote/internal/service/file"
)

// FileHandler 文件处理器
// @Description 文件管理相关的HTTP处理器
type FileHandler struct {
	fileService fileservice.FileService
}

// NewFileHandler 创建文件处理器实例
// @Description 创建新的文件处理器
func NewFileHandler(fileService fileservice.FileService) *FileHandler {
	return &FileHandler{
		fileService: fileService,
	}
}

// UploadFile 上传文件
// @Summary 上传文件
// @Description 上传单个文件到服务器
// @Tags 文件管理
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "要上传的文件"
// @Success 201 {object} map[string]interface{} "上传成功"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /api/v1/files/upload [post]
func (h *FileHandler) UploadFile(c *gin.Context) {
	// 获取上传的文件
	file, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "未选择文件或文件无效")
		return
	}

	// 打开文件
	src, err := file.Open()
	if err != nil {
		response.InternalServerError(c, "无法打开上传的文件")
		return
	}
	defer src.Close()

	// 调用文件服务上传文件
	metadata, err := h.fileService.UploadFile(file.Filename, src)
	if err != nil {
		if appErr, ok := errors.GetAppError(err); ok {
			response.Error(c, int(appErr.Code), appErr.Message)
		} else {
			response.Error(c, int(errors.ErrFileUploadFailed), err.Error())
		}
		return
	}

	response.SuccessWithMessage(c, "文件上传成功", gin.H{
		"file_id":  metadata.FileID,
		"filename": metadata.FileName,
		"size":     metadata.FileSize,
		"format":   metadata.FileFormat,
		"hash":     metadata.FileHash,
	})
}

// GetFile 获取文件信息
// @Summary 获取文件信息
// @Description 根据文件ID获取文件的详细信息
// @Tags 文件管理
// @Produce json
// @Param id path string true "文件ID"
// @Success 200 {object} map[string]interface{} "文件信息"
// @Failure 400 {object} map[string]interface{} "文件ID无效"
// @Failure 404 {object} map[string]interface{} "文件不存在"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /api/v1/files/{id} [get]
func (h *FileHandler) GetFile(c *gin.Context) {
	fileID := c.Param("id")
	if fileID == "" {
		response.BadRequest(c, "文件ID不能为空")
		return
	}

	metadata, err := h.fileService.GetFileByID(fileID)
	if err != nil {
		if appErr, ok := errors.GetAppError(err); ok {
			response.Error(c, int(appErr.Code), appErr.Message)
		} else {
			response.NotFound(c, "文件不存在")
		}
		return
	}

	response.Success(c, gin.H{
		"file_id":      metadata.FileID,
		"filename":     metadata.FileName,
		"size":         metadata.FileSize,
		"format":       metadata.FileFormat,
		"hash":         metadata.FileHash,
		"view_count":   metadata.ViewCount,
		"modify_count": metadata.ModifyCount,
		"created_at":   metadata.CreatedAt,
		"updated_at":   metadata.UpdatedAt,
	})
}

// DownloadFile 下载文件
// @Summary 下载文件
// @Description 根据文件ID下载文件内容
// @Tags 文件管理
// @Produce application/octet-stream
// @Param id path string true "文件ID"
// @Success 200 {file} file "文件内容"
// @Failure 400 {object} map[string]interface{} "文件ID无效"
// @Failure 404 {object} map[string]interface{} "文件不存在"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /api/v1/files/{id}/download [get]
func (h *FileHandler) DownloadFile(c *gin.Context) {
	fileID := c.Param("id")
	if fileID == "" {
		response.BadRequest(c, "文件ID不能为空")
		return
	}

	// 获取文件信息
	metadata, err := h.fileService.GetFileByID(fileID)
	if err != nil {
		if appErr, ok := errors.GetAppError(err); ok {
			response.Error(c, int(appErr.Code), appErr.Message)
		} else {
			response.NotFound(c, "文件不存在")
		}
		return
	}

	// 获取文件内容
	fileContent, err := h.fileService.GetFileContent(fileID)
	if err != nil {
		if appErr, ok := errors.GetAppError(err); ok {
			response.Error(c, int(appErr.Code), appErr.Message)
		} else {
			response.InternalServerError(c, "文件读取失败")
		}
		return
	}
	defer fileContent.Close()

	// 设置响应头
	c.Header("Content-Disposition", "attachment; filename=\""+metadata.FileName+"\"")
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Length", strconv.FormatInt(metadata.FileSize, 10))

	// 流式传输文件内容
	c.DataFromReader(http.StatusOK, metadata.FileSize, "application/octet-stream", fileContent, nil)
}

// ListFiles 获取文件列表
// @Summary 获取文件列表
// @Description 分页获取文件列表
// @Tags 文件管理
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Success 200 {object} map[string]interface{} "文件列表"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /api/v1/files [get]
func (h *FileHandler) ListFiles(c *gin.Context) {
	// 解析分页参数
	page := 1
	pageSize := 10

	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if pageSizeStr := c.Query("page_size"); pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 && ps <= 100 {
			pageSize = ps
		}
	}

	// 获取文件列表
	files, total, err := h.fileService.ListFiles(page, pageSize)
	if err != nil {
		if appErr, ok := errors.GetAppError(err); ok {
			response.Error(c, int(appErr.Code), appErr.Message)
		} else {
			response.InternalServerError(c, "获取文件列表失败")
		}
		return
	}

	response.SuccessWithPage(c, files, total, page, pageSize)
}

// SearchFiles 搜索文件
// @Summary 搜索文件
// @Description 根据文件名搜索文件
// @Tags 文件管理
// @Produce json
// @Param q query string true "搜索关键词"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Success 200 {object} map[string]interface{} "搜索结果"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /api/v1/files/search [get]
func (h *FileHandler) SearchFiles(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		response.BadRequest(c, "搜索关键词不能为空")
		return
	}

	// 解析分页参数
	page := 1
	pageSize := 10

	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if pageSizeStr := c.Query("page_size"); pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 && ps <= 100 {
			pageSize = ps
		}
	}

	// 搜索文件
	files, total, err := h.fileService.SearchFilesByName(query, page, pageSize)
	if err != nil {
		if appErr, ok := errors.GetAppError(err); ok {
			response.Error(c, int(appErr.Code), appErr.Message)
		} else {
			response.InternalServerError(c, "搜索文件失败")
		}
		return
	}

	// 构建分页数据
	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}

	pageData := response.PageData{
		List:       files,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}

	response.SuccessWithMessage(c, "搜索完成", gin.H{
		"result": pageData,
		"query":  query,
	})
}

// DeleteFile 删除文件
// @Summary 删除文件
// @Description 根据文件ID删除文件
// @Tags 文件管理
// @Produce json
// @Param id path string true "文件ID"
// @Success 200 {object} map[string]interface{} "删除成功"
// @Failure 400 {object} map[string]interface{} "文件ID无效"
// @Failure 404 {object} map[string]interface{} "文件不存在"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /api/v1/files/{id} [delete]
func (h *FileHandler) DeleteFile(c *gin.Context) {
	fileID := c.Param("id")
	if fileID == "" {
		response.BadRequest(c, "文件ID不能为空")
		return
	}

	err := h.fileService.DeleteFile(fileID)
	if err != nil {
		if appErr, ok := errors.GetAppError(err); ok {
			response.Error(c, int(appErr.Code), appErr.Message)
		} else {
			response.NotFound(c, "文件不存在或删除失败")
		}
		return
	}

	response.SuccessWithMessage(c, "文件删除成功", gin.H{
		"file_id": fileID,
	})
}

// UpdateFile 更新文件
// @Summary 更新文件
// @Description 根据文件ID更新文件内容
// @Tags 文件管理
// @Accept multipart/form-data
// @Produce json
// @Param id path string true "文件ID"
// @Param file formData file true "新的文件内容"
// @Success 200 {object} map[string]interface{} "更新成功"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Failure 404 {object} map[string]interface{} "文件不存在"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /api/v1/files/{id} [put]
func (h *FileHandler) UpdateFile(c *gin.Context) {
	fileID := c.Param("id")
	if fileID == "" {
		response.BadRequest(c, "文件ID不能为空")
		return
	}

	// 获取上传的文件
	file, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "未选择文件或文件无效")
		return
	}

	// 打开文件
	src, err := file.Open()
	if err != nil {
		response.InternalServerError(c, "无法打开上传的文件")
		return
	}
	defer src.Close()

	// 调用文件服务更新文件
	metadata, err := h.fileService.UpdateFile(fileID, src)
	if err != nil {
		if appErr, ok := errors.GetAppError(err); ok {
			response.Error(c, int(appErr.Code), appErr.Message)
		} else {
			response.Error(c, int(errors.ErrFileUploadFailed), err.Error())
		}
		return
	}

	response.SuccessWithMessage(c, "文件更新成功", gin.H{
		"file_id":      metadata.FileID,
		"filename":     metadata.FileName,
		"size":         metadata.FileSize,
		"format":       metadata.FileFormat,
		"hash":         metadata.FileHash,
		"modify_count": metadata.ModifyCount,
	})
}

// GetFileStats 获取文件统计信息
// @Summary 获取文件统计信息
// @Description 获取系统中文件的统计信息
// @Tags 文件管理
// @Produce json
// @Success 200 {object} map[string]interface{} "统计信息"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /api/v1/files/stats [get]
func (h *FileHandler) GetFileStats(c *gin.Context) {
	stats, err := h.fileService.GetFileStats()
	if err != nil {
		if appErr, ok := errors.GetAppError(err); ok {
			response.Error(c, int(appErr.Code), appErr.Message)
		} else {
			response.InternalServerError(c, "获取文件统计信息失败")
		}
		return
	}

	response.Success(c, stats)
}