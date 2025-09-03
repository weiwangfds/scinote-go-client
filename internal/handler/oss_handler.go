package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/weiwangfds/scinote/internal/database"
	"github.com/weiwangfds/scinote/internal/service"
)

// OSSHandler OSS处理器
type OSSHandler struct {
	ossConfigService service.OSSConfigService
	ossSyncService   service.OSSyncService
}

// NewOSSHandler 创建OSS处理器实例
func NewOSSHandler(ossConfigService service.OSSConfigService, ossSyncService service.OSSyncService) *OSSHandler {
	return &OSSHandler{
		ossConfigService: ossConfigService,
		ossSyncService:   ossSyncService,
	}
}

// CreateOSSConfig 创建OSS配置
func (h *OSSHandler) CreateOSSConfig(c *gin.Context) {
	var config database.OSSConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body: " + err.Error(),
		})
		return
	}

	if err := h.ossConfigService.CreateOSSConfig(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "OSS configuration created successfully",
		"config":  config,
	})
}

// GetOSSConfig 获取OSS配置
func (h *OSSHandler) GetOSSConfig(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid configuration ID",
		})
		return
	}

	config, err := h.ossConfigService.GetOSSConfigByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"config": config,
	})
}

// ListOSSConfigs 获取OSS配置列表
func (h *OSSHandler) ListOSSConfigs(c *gin.Context) {
	configs, err := h.ossConfigService.ListOSSConfigs()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"configs": configs,
		"total":   len(configs),
	})
}

// UpdateOSSConfig 更新OSS配置
func (h *OSSHandler) UpdateOSSConfig(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid configuration ID",
		})
		return
	}

	var config database.OSSConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body: " + err.Error(),
		})
		return
	}

	config.ID = uint(id)
	if err := h.ossConfigService.UpdateOSSConfig(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "OSS configuration updated successfully",
		"config":  config,
	})
}

// DeleteOSSConfig 删除OSS配置
func (h *OSSHandler) DeleteOSSConfig(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid configuration ID",
		})
		return
	}

	if err := h.ossConfigService.DeleteOSSConfig(uint(id)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "OSS configuration deleted successfully",
	})
}

// ActivateOSSConfig 激活OSS配置
func (h *OSSHandler) ActivateOSSConfig(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid configuration ID",
		})
		return
	}

	if err := h.ossConfigService.ActivateOSSConfig(uint(id)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "OSS configuration activated successfully",
	})
}

// TestOSSConfig 测试OSS配置连接
func (h *OSSHandler) TestOSSConfig(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid configuration ID",
		})
		return
	}

	if err := h.ossConfigService.TestOSSConfig(uint(id)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Connection test failed: " + err.Error(),
			"success": false,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "OSS configuration test successful",
		"success": true,
	})
}

// GetActiveOSSConfig 获取当前激活的OSS配置
func (h *OSSHandler) GetActiveOSSConfig(c *gin.Context) {
	config, err := h.ossConfigService.GetActiveOSSConfig()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"config": config,
	})
}

// ToggleOSSConfig 启用/禁用OSS配置
func (h *OSSHandler) ToggleOSSConfig(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid configuration ID",
		})
		return
	}

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body: " + err.Error(),
		})
		return
	}

	if err := h.ossConfigService.ToggleOSSConfig(uint(id), req.Enabled); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	status := "disabled"
	if req.Enabled {
		status = "enabled"
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "OSS configuration " + status + " successfully",
	})
}

// SyncAllFromOSS 从OSS同步所有文件到本地
func (h *OSSHandler) SyncAllFromOSS(c *gin.Context) {
	if err := h.ossSyncService.SyncAllFromOSS(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Sync all files from OSS started successfully",
	})
}

// ScanAndCompareFiles 扫描文件表并与云端对比
func (h *OSSHandler) ScanAndCompareFiles(c *gin.Context) {
	needUpdateFiles, cloudOnlyFiles, err := h.ossSyncService.ScanAndCompareFiles()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"need_update_files": needUpdateFiles,
		"cloud_only_files":  cloudOnlyFiles,
		"total_need_update": len(needUpdateFiles),
		"total_cloud_only":  len(cloudOnlyFiles),
	})
}

// GetSyncLogs 获取同步日志
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
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"logs":     logs,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

// GetFileSyncStatus 获取文件的同步状态
func (h *OSSHandler) GetFileSyncStatus(c *gin.Context) {
	fileID := c.Param("fileID")
	if fileID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "File ID is required",
		})
		return
	}

	status, err := h.ossSyncService.GetFileSyncStatus(fileID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": status,
	})
}

// RetryFailedSync 重试失败的同步任务
func (h *OSSHandler) RetryFailedSync(c *gin.Context) {
	logIDStr := c.Param("logID")
	logID, err := strconv.ParseUint(logIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid log ID",
		})
		return
	}

	if err := h.ossSyncService.RetryFailedSync(uint(logID)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Sync task retry started successfully",
	})
}

// SyncFileToOSS 同步单个文件到OSS
func (h *OSSHandler) SyncFileToOSS(c *gin.Context) {
	fileID := c.Param("fileID")
	if fileID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "File ID is required",
		})
		return
	}

	if err := h.ossSyncService.SyncToOSS(fileID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "File sync to OSS started successfully",
		"file_id": fileID,
	})
}

// BatchSyncToOSS 批量同步文件到OSS
func (h *OSSHandler) BatchSyncToOSS(c *gin.Context) {
	var request struct {
		FileIDs []string `json:"file_ids" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body: " + err.Error(),
		})
		return
	}

	if len(request.FileIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "At least one file ID is required",
		})
		return
	}

	if err := h.ossSyncService.BatchSyncToOSS(request.FileIDs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Batch sync to OSS started successfully",
		"file_count": len(request.FileIDs),
	})
}
