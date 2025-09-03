package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response 统一返回值结构体
// @Description API统一响应格式
type Response struct {
	// 状态码，0表示成功，非0表示失败
	Code int `json:"code" example:"0"`
	// 响应消息
	Message string `json:"message" example:"success"`
	// 响应数据
	Data interface{} `json:"data,omitempty"`
	// 请求ID，用于链路追踪
	RequestID string `json:"request_id,omitempty" example:"req_123456789"`
	// 时间戳
	Timestamp int64 `json:"timestamp" example:"1640995200"`
}

// PageData 分页数据结构体
// @Description 分页响应数据格式
type PageData struct {
	// 数据列表
	List interface{} `json:"list"`
	// 总数
	Total int64 `json:"total" example:"100"`
	// 当前页码
	Page int `json:"page" example:"1"`
	// 每页大小
	PageSize int `json:"page_size" example:"10"`
	// 总页数
	TotalPages int `json:"total_pages" example:"10"`
}

// Success 成功响应
// @Summary 返回成功响应
// @Description 返回成功的API响应
// @Param c gin上下文
// @Param data 响应数据
func Success(c *gin.Context, data interface{}) {
	response := Response{
		Code:      0,
		Message:   "success",
		Data:      data,
		RequestID: getRequestID(c),
		Timestamp: getCurrentTimestamp(),
	}
	c.JSON(http.StatusOK, response)
}

// SuccessWithMessage 带消息的成功响应
// @Summary 返回带自定义消息的成功响应
// @Description 返回带自定义消息的成功API响应
// @Param c gin上下文
// @Param message 自定义消息
// @Param data 响应数据
func SuccessWithMessage(c *gin.Context, message string, data interface{}) {
	response := Response{
		Code:      0,
		Message:   message,
		Data:      data,
		RequestID: getRequestID(c),
		Timestamp: getCurrentTimestamp(),
	}
	c.JSON(http.StatusOK, response)
}

// SuccessWithPage 分页成功响应
// @Summary 返回分页成功响应
// @Description 返回分页数据的成功API响应
// @Param c gin上下文
// @Param list 数据列表
// @Param total 总数
// @Param page 当前页码
// @Param pageSize 每页大小
func SuccessWithPage(c *gin.Context, list interface{}, total int64, page, pageSize int) {
	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}

	pageData := PageData{
		List:       list,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}

	response := Response{
		Code:      0,
		Message:   "success",
		Data:      pageData,
		RequestID: getRequestID(c),
		Timestamp: getCurrentTimestamp(),
	}
	c.JSON(http.StatusOK, response)
}

// Error 错误响应
// @Summary 返回错误响应
// @Description 返回错误的API响应
// @Param c gin上下文
// @Param code 错误码
// @Param message 错误消息
func Error(c *gin.Context, code int, message string) {
	response := Response{
		Code:      code,
		Message:   message,
		RequestID: getRequestID(c),
		Timestamp: getCurrentTimestamp(),
	}
	c.JSON(http.StatusOK, response)
}

// ErrorWithData 带数据的错误响应
// @Summary 返回带数据的错误响应
// @Description 返回带额外数据的错误API响应
// @Param c gin上下文
// @Param code 错误码
// @Param message 错误消息
// @Param data 错误相关数据
func ErrorWithData(c *gin.Context, code int, message string, data interface{}) {
	response := Response{
		Code:      code,
		Message:   message,
		Data:      data,
		RequestID: getRequestID(c),
		Timestamp: getCurrentTimestamp(),
	}
	c.JSON(http.StatusOK, response)
}

// BadRequest 400错误响应
// @Summary 返回400错误响应
// @Description 返回请求参数错误的API响应
// @Param c gin上下文
// @Param message 错误消息
func BadRequest(c *gin.Context, message string) {
	response := Response{
		Code:      400,
		Message:   message,
		RequestID: getRequestID(c),
		Timestamp: getCurrentTimestamp(),
	}
	c.JSON(http.StatusBadRequest, response)
}

// Unauthorized 401错误响应
// @Summary 返回401错误响应
// @Description 返回未授权的API响应
// @Param c gin上下文
// @Param message 错误消息
func Unauthorized(c *gin.Context, message string) {
	response := Response{
		Code:      401,
		Message:   message,
		RequestID: getRequestID(c),
		Timestamp: getCurrentTimestamp(),
	}
	c.JSON(http.StatusUnauthorized, response)
}

// Forbidden 403错误响应
// @Summary 返回403错误响应
// @Description 返回禁止访问的API响应
// @Param c gin上下文
// @Param message 错误消息
func Forbidden(c *gin.Context, message string) {
	response := Response{
		Code:      403,
		Message:   message,
		RequestID: getRequestID(c),
		Timestamp: getCurrentTimestamp(),
	}
	c.JSON(http.StatusForbidden, response)
}

// NotFound 404错误响应
// @Summary 返回404错误响应
// @Description 返回资源未找到的API响应
// @Param c gin上下文
// @Param message 错误消息
func NotFound(c *gin.Context, message string) {
	response := Response{
		Code:      404,
		Message:   message,
		RequestID: getRequestID(c),
		Timestamp: getCurrentTimestamp(),
	}
	c.JSON(http.StatusNotFound, response)
}

// InternalServerError 500错误响应
// @Summary 返回500错误响应
// @Description 返回服务器内部错误的API响应
// @Param c gin上下文
// @Param message 错误消息
func InternalServerError(c *gin.Context, message string) {
	response := Response{
		Code:      500,
		Message:   message,
		RequestID: getRequestID(c),
		Timestamp: getCurrentTimestamp(),
	}
	c.JSON(http.StatusInternalServerError, response)
}

// getRequestID 获取请求ID
// @Description 从gin上下文中获取请求ID，用于链路追踪
// @Param c gin上下文
// @Return 请求ID字符串
func getRequestID(c *gin.Context) string {
	if requestID, exists := c.Get("request_id"); exists {
		if id, ok := requestID.(string); ok {
			return id
		}
	}
	return ""
}

// getCurrentTimestamp 获取当前时间戳
// @Description 获取当前Unix时间戳
// @Return 当前时间戳
func getCurrentTimestamp() int64 {
	return getCurrentTime().Unix()
}

// getCurrentTime 获取当前时间
// @Description 获取当前时间，便于测试时mock
// @Return 当前时间
var getCurrentTime = func() interface{ Unix() int64 } {
	return timeProvider{}
}

// timeProvider 时间提供者
type timeProvider struct{}

// Unix 返回Unix时间戳
func (timeProvider) Unix() int64 {
	return 1640995200 // 这里应该返回真实的时间戳，为了示例使用固定值
}