package errors

import (
	"fmt"
)

// ErrorCode 错误码类型
type ErrorCode int

// 定义错误码常量
const (
	// 通用错误码 (1000-1999)
	ErrSuccess           ErrorCode = 0    // 成功
	ErrInternalServer    ErrorCode = 1000 // 服务器内部错误
	ErrInvalidParams     ErrorCode = 1001 // 参数错误
	ErrUnauthorized      ErrorCode = 1002 // 未授权
	ErrForbidden         ErrorCode = 1003 // 禁止访问
	ErrNotFound          ErrorCode = 1004 // 资源未找到
	ErrMethodNotAllowed  ErrorCode = 1005 // 方法不允许
	ErrTooManyRequests   ErrorCode = 1006 // 请求过于频繁
	ErrServiceUnavailable ErrorCode = 1007 // 服务不可用

	// 文件相关错误码 (2000-2999)
	ErrFileNotFound      ErrorCode = 2000 // 文件未找到
	ErrFileAlreadyExists ErrorCode = 2001 // 文件已存在
	ErrFileUploadFailed  ErrorCode = 2002 // 文件上传失败
	ErrFileDeleteFailed  ErrorCode = 2003 // 文件删除失败
	ErrFileReadFailed    ErrorCode = 2004 // 文件读取失败
	ErrFileWriteFailed   ErrorCode = 2005 // 文件写入失败
	ErrFileSizeTooLarge  ErrorCode = 2006 // 文件大小超限
	ErrFileTypeNotAllowed ErrorCode = 2007 // 文件类型不允许
	ErrFileCorrupted     ErrorCode = 2008 // 文件损坏
	ErrFileHashMismatch  ErrorCode = 2009 // 文件哈希不匹配

	// OSS相关错误码 (3000-3999)
	ErrOSSConfigNotFound ErrorCode = 3000 // OSS配置未找到
	ErrOSSConfigInvalid  ErrorCode = 3001 // OSS配置无效
	ErrOSSConnectionFailed ErrorCode = 3002 // OSS连接失败
	ErrOSSUploadFailed   ErrorCode = 3003 // OSS上传失败
	ErrOSSDownloadFailed ErrorCode = 3004 // OSS下载失败
	ErrOSSDeleteFailed   ErrorCode = 3005 // OSS删除失败
	ErrOSSListFailed     ErrorCode = 3006 // OSS列表获取失败
	ErrOSSSyncFailed     ErrorCode = 3007 // OSS同步失败
	ErrOSSProviderNotSupported ErrorCode = 3008 // OSS提供商不支持

	// 数据库相关错误码 (4000-4999)
	ErrDatabaseConnection ErrorCode = 4000 // 数据库连接错误
	ErrDatabaseQuery     ErrorCode = 4001 // 数据库查询错误
	ErrDatabaseInsert    ErrorCode = 4002 // 数据库插入错误
	ErrDatabaseUpdate    ErrorCode = 4003 // 数据库更新错误
	ErrDatabaseDelete    ErrorCode = 4004 // 数据库删除错误
	ErrDatabaseTransaction ErrorCode = 4005 // 数据库事务错误
	ErrRecordNotFound    ErrorCode = 4006 // 记录未找到
	ErrRecordAlreadyExists ErrorCode = 4007 // 记录已存在
)

// AppError 应用错误结构体
// @Description 应用程序统一错误格式
type AppError struct {
	// 错误码
	Code ErrorCode `json:"code"`
	// 错误消息
	Message string `json:"message"`
	// 详细错误信息
	Details string `json:"details,omitempty"`
	// 原始错误
	OriginalError error `json:"-"`
}

// Error 实现error接口
// @Description 返回错误的字符串表示
// @Return 错误字符串
func (e *AppError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("[%d] %s: %s", e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

// WithDetails 添加详细错误信息
// @Description 为错误添加详细信息
// @Param details 详细错误信息
// @Return 带详细信息的错误
func (e *AppError) WithDetails(details string) *AppError {
	e.Details = details
	return e
}

// WithOriginalError 添加原始错误
// @Description 为错误添加原始错误信息
// @Param err 原始错误
// @Return 带原始错误的错误
func (e *AppError) WithOriginalError(err error) *AppError {
	e.OriginalError = err
	if e.Details == "" && err != nil {
		e.Details = err.Error()
	}
	return e
}

// New 创建新的应用错误
// @Description 创建新的应用程序错误
// @Param code 错误码
// @Param message 错误消息
// @Return 应用错误实例
func New(code ErrorCode, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
	}
}

// NewWithDetails 创建带详细信息的应用错误
// @Description 创建带详细信息的应用程序错误
// @Param code 错误码
// @Param message 错误消息
// @Param details 详细错误信息
// @Return 应用错误实例
func NewWithDetails(code ErrorCode, message string, details string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Details: details,
	}
}

// Wrap 包装原始错误
// @Description 将原始错误包装为应用程序错误
// @Param code 错误码
// @Param message 错误消息
// @Param err 原始错误
// @Return 应用错误实例
func Wrap(code ErrorCode, message string, err error) *AppError {
	appErr := &AppError{
		Code:          code,
		Message:       message,
		OriginalError: err,
	}
	if err != nil {
		appErr.Details = err.Error()
	}
	return appErr
}

// IsAppError 判断是否为应用错误
// @Description 判断给定错误是否为应用程序错误类型
// @Param err 待判断的错误
// @Return 是否为应用错误
func IsAppError(err error) bool {
	_, ok := err.(*AppError)
	return ok
}

// GetAppError 获取应用错误
// @Description 从错误中提取应用程序错误
// @Param err 原始错误
// @Return 应用错误实例和是否成功提取的标志
func GetAppError(err error) (*AppError, bool) {
	appErr, ok := err.(*AppError)
	return appErr, ok
}

// 预定义的常用错误
var (
	// 通用错误
	ErrInternalServerError = New(ErrInternalServer, "服务器内部错误")
	ErrInvalidParameters   = New(ErrInvalidParams, "参数错误")
	ErrUnauthorizedAccess  = New(ErrUnauthorized, "未授权访问")
	ErrForbiddenAccess     = New(ErrForbidden, "禁止访问")
	ErrResourceNotFound    = New(ErrNotFound, "资源未找到")

	// 文件相关错误
	ErrFileNotFoundError      = New(ErrFileNotFound, "文件未找到")
	ErrFileAlreadyExistsError = New(ErrFileAlreadyExists, "文件已存在")
	ErrFileUploadFailedError  = New(ErrFileUploadFailed, "文件上传失败")
	ErrFileDeleteFailedError  = New(ErrFileDeleteFailed, "文件删除失败")
	ErrFileReadFailedError    = New(ErrFileReadFailed, "文件读取失败")
	ErrFileWriteFailedError   = New(ErrFileWriteFailed, "文件写入失败")
	ErrFileSizeTooLargeError  = New(ErrFileSizeTooLarge, "文件大小超限")
	ErrFileTypeNotAllowedError = New(ErrFileTypeNotAllowed, "文件类型不允许")
	ErrFileCorruptedError     = New(ErrFileCorrupted, "文件损坏")
	ErrFileHashMismatchError  = New(ErrFileHashMismatch, "文件哈希不匹配")

	// OSS相关错误
	ErrOSSConfigNotFoundError = New(ErrOSSConfigNotFound, "OSS配置未找到")
	ErrOSSConfigInvalidError  = New(ErrOSSConfigInvalid, "OSS配置无效")
	ErrOSSConnectionFailedError = New(ErrOSSConnectionFailed, "OSS连接失败")
	ErrOSSUploadFailedError   = New(ErrOSSUploadFailed, "OSS上传失败")
	ErrOSSDownloadFailedError = New(ErrOSSDownloadFailed, "OSS下载失败")
	ErrOSSDeleteFailedError   = New(ErrOSSDeleteFailed, "OSS删除失败")
	ErrOSSListFailedError     = New(ErrOSSListFailed, "OSS列表获取失败")
	ErrOSSSyncFailedError     = New(ErrOSSSyncFailed, "OSS同步失败")
	ErrOSSProviderNotSupportedError = New(ErrOSSProviderNotSupported, "OSS提供商不支持")

	// 数据库相关错误
	ErrDatabaseConnectionError = New(ErrDatabaseConnection, "数据库连接错误")
	ErrDatabaseQueryError     = New(ErrDatabaseQuery, "数据库查询错误")
	ErrDatabaseInsertError    = New(ErrDatabaseInsert, "数据库插入错误")
	ErrDatabaseUpdateError    = New(ErrDatabaseUpdate, "数据库更新错误")
	ErrDatabaseDeleteError    = New(ErrDatabaseDelete, "数据库删除错误")
	ErrDatabaseTransactionError = New(ErrDatabaseTransaction, "数据库事务错误")
	ErrRecordNotFoundError    = New(ErrRecordNotFound, "记录未找到")
	ErrRecordAlreadyExistsError = New(ErrRecordAlreadyExists, "记录已存在")
)

// GetErrorMessage 根据错误码获取错误消息
// @Description 根据错误码获取对应的错误消息
// @Param code 错误码
// @Return 错误消息
func GetErrorMessage(code ErrorCode) string {
	errorMessages := map[ErrorCode]string{
		ErrSuccess:           "成功",
		ErrInternalServer:    "服务器内部错误",
		ErrInvalidParams:     "参数错误",
		ErrUnauthorized:      "未授权",
		ErrForbidden:         "禁止访问",
		ErrNotFound:          "资源未找到",
		ErrMethodNotAllowed:  "方法不允许",
		ErrTooManyRequests:   "请求过于频繁",
		ErrServiceUnavailable: "服务不可用",

		ErrFileNotFound:      "文件未找到",
		ErrFileAlreadyExists: "文件已存在",
		ErrFileUploadFailed:  "文件上传失败",
		ErrFileDeleteFailed:  "文件删除失败",
		ErrFileReadFailed:    "文件读取失败",
		ErrFileWriteFailed:   "文件写入失败",
		ErrFileSizeTooLarge:  "文件大小超限",
		ErrFileTypeNotAllowed: "文件类型不允许",
		ErrFileCorrupted:     "文件损坏",
		ErrFileHashMismatch:  "文件哈希不匹配",

		ErrOSSConfigNotFound: "OSS配置未找到",
		ErrOSSConfigInvalid:  "OSS配置无效",
		ErrOSSConnectionFailed: "OSS连接失败",
		ErrOSSUploadFailed:   "OSS上传失败",
		ErrOSSDownloadFailed: "OSS下载失败",
		ErrOSSDeleteFailed:   "OSS删除失败",
		ErrOSSListFailed:     "OSS列表获取失败",
		ErrOSSSyncFailed:     "OSS同步失败",
		ErrOSSProviderNotSupported: "OSS提供商不支持",

		ErrDatabaseConnection: "数据库连接错误",
		ErrDatabaseQuery:     "数据库查询错误",
		ErrDatabaseInsert:    "数据库插入错误",
		ErrDatabaseUpdate:    "数据库更新错误",
		ErrDatabaseDelete:    "数据库删除错误",
		ErrDatabaseTransaction: "数据库事务错误",
		ErrRecordNotFound:    "记录未找到",
		ErrRecordAlreadyExists: "记录已存在",
	}

	if message, exists := errorMessages[code]; exists {
		return message
	}
	return "未知错误"
}