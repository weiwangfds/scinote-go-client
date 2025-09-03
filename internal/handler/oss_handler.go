package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/weiwangfds/scinote/internal/database"
	"github.com/weiwangfds/scinote/internal/errors"
	"github.com/weiwangfds/scinote/internal/response"
	ossservice "github.com/weiwangfds/scinote/internal/service/oss"
)

// OSSHandler OSS处理器
type OSSHandler struct {
	ossConfigService ossservice.OSSConfigService
	ossSyncService   ossservice.OSSyncService
}

// NewOSSHandler 创建OSS处理器实例
func NewOSSHandler(ossConfigService ossservice.OSSConfigService, ossSyncService ossservice.OSSyncService) *OSSHandler {
	return &OSSHandler{
		ossConfigService: ossConfigService,
		ossSyncService:   ossSyncService,
	}
}

// CreateOSSConfig 创建OSS配置
// @Summary 创建OSS配置
// @Description 创建新的OSS存储配置，支持阿里云、腾讯云、七牛云等多种云存储服务
// @Tags OSS配置管理
// @Accept json
// @Produce json
// @Param config body database.OSSConfig true "OSS配置信息"
// @Success 201 {object} map[string]interface{} "创建成功"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Router /oss/configs [post]
func (h *OSSHandler) CreateOSSConfig(c *gin.Context) {
	var config database.OSSConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		response.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	if err := h.ossConfigService.CreateOSSConfig(&config); err != nil {
		if appErr, ok := errors.GetAppError(err); ok {
			response.Error(c, int(appErr.Code), appErr.Message)
		} else {
			response.Error(c, int(errors.ErrOSSConfigInvalid), err.Error())
		}
		return
	}

	response.SuccessWithMessage(c, "OSS配置创建成功", gin.H{
		"config": config,
	})
}

// GetOSSConfig 获取OSS配置
// @Summary 获取单个OSS配置
// @Description 根据配置ID获取指定的OSS配置详细信息
// @Tags OSS配置管理
// @Accept json
// @Produce json
// @Param id path int true "配置ID"
// @Success 200 {object} database.OSSConfig "获取成功"
// @Failure 400 {object} map[string]interface{} "配置ID无效"
// @Failure 404 {object} map[string]interface{} "配置不存在"
// @Router /oss/configs/{id} [get]
func (h *OSSHandler) GetOSSConfig(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "配置ID无效")
		return
	}

	config, err := h.ossConfigService.GetOSSConfigByID(uint(id))
	if err != nil {
		if appErr, ok := errors.GetAppError(err); ok {
			response.Error(c, int(appErr.Code), appErr.Message)
		} else {
			response.NotFound(c, "OSS配置不存在")
		}
		return
	}

	response.Success(c, gin.H{
		"config": config,
	})
}

// ListOSSConfigs 获取OSS配置列表
// @Summary 获取所有OSS配置
// @Description 获取系统中所有已配置的OSS存储配置列表
// @Tags OSS配置管理
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "获取成功"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /oss/configs [get]
func (h *OSSHandler) ListOSSConfigs(c *gin.Context) {
	configs, err := h.ossConfigService.ListOSSConfigs()
	if err != nil {
		if appErr, ok := errors.GetAppError(err); ok {
			response.Error(c, int(appErr.Code), appErr.Message)
		} else {
			response.InternalServerError(c, "获取OSS配置列表失败")
		}
		return
	}

	response.Success(c, gin.H{
		"configs": configs,
		"total":   len(configs),
	})
}

// UpdateOSSConfig 更新OSS配置
// @Summary 更新OSS配置
// @Description 根据配置ID更新指定的OSS配置信息
// @Tags OSS配置管理
// @Accept json
// @Produce json
// @Param id path int true "配置ID"
// @Param config body database.OSSConfig true "OSS配置信息"
// @Success 200 {object} map[string]interface{} "更新成功"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Failure 404 {object} map[string]interface{} "配置不存在"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /oss/configs/{id} [put]
func (h *OSSHandler) UpdateOSSConfig(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "配置ID无效")
		return
	}

	var config database.OSSConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		response.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	config.ID = uint(id)
	if err := h.ossConfigService.UpdateOSSConfig(&config); err != nil {
		if appErr, ok := errors.GetAppError(err); ok {
			response.Error(c, int(appErr.Code), appErr.Message)
		} else {
			response.Error(c, int(errors.ErrOSSConfigInvalid), err.Error())
		}
		return
	}

	response.SuccessWithMessage(c, "OSS配置更新成功", gin.H{
		"config": config,
	})
}

// DeleteOSSConfig 删除OSS配置
// @Summary 删除OSS配置
// @Description 根据配置ID删除指定的OSS配置
// @Tags OSS配置管理
// @Accept json
// @Produce json
// @Param id path int true "配置ID"
// @Success 200 {object} map[string]interface{} "删除成功"
// @Failure 400 {object} map[string]interface{} "配置ID无效"
// @Failure 404 {object} map[string]interface{} "配置不存在"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /oss/configs/{id} [delete]
func (h *OSSHandler) DeleteOSSConfig(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "配置ID无效")
		return
	}

	if err := h.ossConfigService.DeleteOSSConfig(uint(id)); err != nil {
		if appErr, ok := errors.GetAppError(err); ok {
			response.Error(c, int(appErr.Code), appErr.Message)
		} else {
			response.Error(c, int(errors.ErrOSSConfigNotFound), err.Error())
		}
		return
	}

	response.SuccessWithMessage(c, "OSS配置删除成功", nil)
}

// ActivateOSSConfig 激活OSS配置
// @Summary 激活OSS配置
// @Description 将指定的OSS配置设置为当前激活状态，同时将其他配置设为非激活状态
// @Tags OSS配置管理
// @Accept json
// @Produce json
// @Param id path int true "配置ID"
// @Success 200 {object} map[string]interface{} "激活成功"
// @Failure 400 {object} map[string]interface{} "配置ID无效"
// @Failure 404 {object} map[string]interface{} "配置不存在"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /oss/configs/{id}/activate [post]
func (h *OSSHandler) ActivateOSSConfig(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "配置ID无效")
		return
	}

	if err := h.ossConfigService.ActivateOSSConfig(uint(id)); err != nil {
		if appErr, ok := errors.GetAppError(err); ok {
			response.Error(c, int(appErr.Code), appErr.Message)
		} else {
			response.Error(c, int(errors.ErrOSSConfigNotFound), err.Error())
		}
		return
	}

	response.SuccessWithMessage(c, "OSS配置激活成功", nil)
}

// TestOSSConfig 测试OSS配置连接
// @Summary 测试OSS配置连接
// @Description 测试指定OSS配置的连接是否正常
// @Tags OSS配置管理
// @Accept json
// @Produce json
// @Param id path int true "配置ID"
// @Success 200 {object} map[string]interface{} "连接测试成功"
// @Failure 400 {object} map[string]interface{} "配置ID无效"
// @Failure 500 {object} map[string]interface{} "连接测试失败"
// @Router /oss/configs/{id}/test [post]
func (h *OSSHandler) TestOSSConfig(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "配置ID无效")
		return
	}

	if err := h.ossConfigService.TestOSSConfig(uint(id)); err != nil {
		response.Error(c, int(errors.ErrOSSConnectionFailed), "连接测试失败: "+err.Error())
		return
	}

	response.SuccessWithMessage(c, "OSS配置测试成功", gin.H{
		"success": true,
	})
}

// GetActiveOSSConfig 获取当前激活的OSS配置
// @Summary 获取当前激活的OSS配置
// @Description 获取系统中当前处于激活状态的OSS配置信息
// @Tags OSS配置管理
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "获取成功"
// @Failure 404 {object} map[string]interface{} "未找到激活的配置"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /oss/configs/active [get]
func (h *OSSHandler) GetActiveOSSConfig(c *gin.Context) {
	config, err := h.ossConfigService.GetActiveOSSConfig()
	if err != nil {
		if appErr, ok := errors.GetAppError(err); ok {
			response.Error(c, int(appErr.Code), appErr.Message)
		} else {
			response.NotFound(c, "未找到激活的OSS配置")
		}
		return
	}

	response.Success(c, gin.H{
		"config": config,
	})
}

// ToggleOSSConfig 启用/禁用OSS配置
// @Summary 启用/禁用OSS配置
// @Description 切换指定OSS配置的启用状态
// @Tags OSS配置管理
// @Accept json
// @Produce json
// @Param id path int true "配置ID"
// @Param request body object{enabled=bool} true "启用状态"
// @Success 200 {object} map[string]interface{} "操作成功"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Failure 404 {object} map[string]interface{} "配置不存在"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /oss/configs/{id}/toggle [post]
func (h *OSSHandler) ToggleOSSConfig(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "配置ID无效")
		return
	}

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	if err := h.ossConfigService.ToggleOSSConfig(uint(id), req.Enabled); err != nil {
		if appErr, ok := errors.GetAppError(err); ok {
			response.Error(c, int(appErr.Code), appErr.Message)
		} else {
			response.Error(c, int(errors.ErrOSSConfigNotFound), err.Error())
		}
		return
	}

	status := "禁用"
	if req.Enabled {
		status = "启用"
	}

	response.SuccessWithMessage(c, "OSS配置"+status+"成功", nil)
}

// SyncAllFromOSS 从OSS同步所有文件到本地
// @Summary 从OSS同步所有文件
// @Description 启动从OSS云存储同步所有文件到本地的任务
// @Tags OSS同步管理
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "同步任务启动成功"
// @Failure 500 {object} map[string]interface{} "同步任务启动失败"
// @Router /oss/sync/all [post]
func (h *OSSHandler) SyncAllFromOSS(c *gin.Context) {
	if err := h.ossSyncService.SyncAllFromOSS(); err != nil {
		if appErr, ok := errors.GetAppError(err); ok {
			response.Error(c, int(appErr.Code), appErr.Message)
		} else {
			response.Error(c, int(errors.ErrOSSSyncFailed), err.Error())
		}
		return
	}

	response.SuccessWithMessage(c, "从OSS同步所有文件任务已启动", nil)
}

// ScanAndCompareFiles 扫描文件表并与云端对比
// @Summary 扫描并对比文件
// @Description 扫描本地文件表并与OSS云端文件进行对比，返回需要更新的文件和仅存在于云端的文件列表
// @Tags OSS同步管理
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "对比结果"
// @Failure 500 {object} map[string]interface{} "扫描对比失败"
// @Router /oss/sync/compare [get]
func (h *OSSHandler) ScanAndCompareFiles(c *gin.Context) {
	needUpdateFiles, cloudOnlyFiles, err := h.ossSyncService.ScanAndCompareFiles()
	if err != nil {
		if appErr, ok := errors.GetAppError(err); ok {
			response.Error(c, int(appErr.Code), appErr.Message)
		} else {
			response.Error(c, int(errors.ErrOSSSyncFailed), err.Error())
		}
		return
	}

	response.Success(c, gin.H{
		"need_update_files": needUpdateFiles,
		"cloud_only_files":  cloudOnlyFiles,
		"total_need_update": len(needUpdateFiles),
		"total_cloud_only":  len(cloudOnlyFiles),
	})
}

// GetSyncLogs 获取同步日志
// @Summary 获取同步日志
// @Description 分页获取OSS文件同步操作的历史日志记录
// @Tags OSS同步管理
// @Accept json
// @Produce json
// @Param page query int false "页码" default(1)
// @Param pageSize query int false "每页数量" default(10)
// @Success 200 {object} map[string]interface{} "同步日志列表"
// @Failure 500 {object} map[string]interface{} "获取日志失败"
// @Router /oss/sync/logs [get]
func (h *OSSHandler) GetSyncLogs(c *gin.Context) {
	page := 1
	pageSize := 10

	// 从查询参数获取分页信息
	if pageParam := c.Query("page"); pageParam != "" {
		if p, err := strconv.Atoi(pageParam); err == nil && p > 0 {
			page = p
		}
	}

	if pageSizeParam := c.Query("pageSize"); pageSizeParam != "" {
		if ps, err := strconv.Atoi(pageSizeParam); err == nil && ps > 0 {
			pageSize = ps
		}
	}

	logs, total, err := h.ossSyncService.GetSyncLogs(page, pageSize)
	if err != nil {
		if appErr, ok := errors.GetAppError(err); ok {
			response.Error(c, int(appErr.Code), appErr.Message)
		} else {
			response.InternalServerError(c, "获取同步日志失败")
		}
		return
	}

	response.SuccessWithPage(c, logs, int64(total), page, pageSize)
}

// GetFileSyncStatus 获取文件的同步状态
// @Summary 获取文件同步状态
// @Description 根据文件ID获取指定文件的OSS同步状态信息
// @Tags OSS同步管理
// @Accept json
// @Produce json
// @Param fileID path string true "文件ID"
// @Success 200 {object} map[string]interface{} "文件同步状态"
// @Failure 400 {object} map[string]interface{} "文件ID无效"
// @Failure 404 {object} map[string]interface{} "文件不存在"
// @Failure 500 {object} map[string]interface{} "获取状态失败"
// @Router /oss/sync/status/{fileID} [get]
func (h *OSSHandler) GetFileSyncStatus(c *gin.Context) {
	fileID := c.Param("fileID")
	if fileID == "" {
		response.BadRequest(c, "文件ID不能为空")
		return
	}

	status, err := h.ossSyncService.GetFileSyncStatus(fileID)
	if err != nil {
		if appErr, ok := errors.GetAppError(err); ok {
			response.Error(c, int(appErr.Code), appErr.Message)
		} else {
			response.Error(c, int(errors.ErrFileNotFound), err.Error())
		}
		return
	}

	response.Success(c, gin.H{
		"status": status,
	})
}

// RetryFailedSync 重试失败的同步任务
// @Summary 重试失败的同步任务
// @Description 根据日志ID重新执行失败的OSS同步任务
// @Tags OSS同步管理
// @Accept json
// @Produce json
// @Param logID path int true "同步日志ID"
// @Success 200 {object} map[string]interface{} "重试任务启动成功"
// @Failure 400 {object} map[string]interface{} "日志ID无效"
// @Failure 500 {object} map[string]interface{} "重试任务启动失败"
// @Router /oss/sync/retry/{logID} [post]
func (h *OSSHandler) RetryFailedSync(c *gin.Context) {
	logIDStr := c.Param("logID")
	logID, err := strconv.ParseUint(logIDStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "日志ID无效")
		return
	}

	if err := h.ossSyncService.RetryFailedSync(uint(logID)); err != nil {
		if appErr, ok := errors.GetAppError(err); ok {
			response.Error(c, int(appErr.Code), appErr.Message)
		} else {
			response.Error(c, int(errors.ErrOSSSyncFailed), err.Error())
		}
		return
	}

	response.SuccessWithMessage(c, "同步任务重试已启动", nil)
}

// SyncFileToOSS 同步单个文件到OSS
// @Summary 同步单个文件到OSS
// @Description 将指定的单个文件同步上传到OSS云存储
// @Tags OSS同步管理
// @Accept json
// @Produce json
// @Param fileID path string true "文件ID"
// @Success 200 {object} map[string]interface{} "同步任务启动成功"
// @Failure 400 {object} map[string]interface{} "文件ID无效"
// @Failure 500 {object} map[string]interface{} "同步任务启动失败"
// @Router /oss/sync/upload/{fileID} [post]
func (h *OSSHandler) SyncFileToOSS(c *gin.Context) {
	fileID := c.Param("fileID")
	if fileID == "" {
		response.BadRequest(c, "文件ID不能为空")
		return
	}

	if err := h.ossSyncService.SyncToOSS(fileID); err != nil {
		if appErr, ok := errors.GetAppError(err); ok {
			response.Error(c, int(appErr.Code), appErr.Message)
		} else {
			response.Error(c, int(errors.ErrOSSSyncFailed), err.Error())
		}
		return
	}

	response.SuccessWithMessage(c, "文件同步到OSS任务已启动", gin.H{
		"file_id": fileID,
	})
}

// BatchSyncToOSS 批量同步文件到OSS
// @Summary 批量同步文件到OSS
// @Description 将多个指定文件批量同步上传到OSS云存储
// @Tags OSS同步管理
// @Accept json
// @Produce json
// @Param request body object{file_ids=[]string} true "文件ID列表"
// @Success 200 {object} map[string]interface{} "批量同步任务启动成功"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Failure 500 {object} map[string]interface{} "批量同步任务启动失败"
// @Router /oss/sync/batch [post]
func (h *OSSHandler) BatchSyncToOSS(c *gin.Context) {
	var request struct {
		FileIDs []string `json:"file_ids" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	if len(request.FileIDs) == 0 {
		response.BadRequest(c, "至少需要一个文件ID")
		return
	}

	if err := h.ossSyncService.BatchSyncToOSS(request.FileIDs); err != nil {
		if appErr, ok := errors.GetAppError(err); ok {
			response.Error(c, int(appErr.Code), appErr.Message)
		} else {
			response.Error(c, int(errors.ErrOSSSyncFailed), err.Error())
		}
		return
	}

	response.SuccessWithMessage(c, "批量同步到OSS任务已启动", gin.H{
		"file_count": len(request.FileIDs),
	})
}
