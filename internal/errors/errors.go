package errors

import (
	"fmt"

	"github.com/weiwangfds/scinote/internal/i18n"
)

// ErrorCode 错误码类型
type ErrorCode int

// 定义错误码常量
const (
	// 通用错误码 (1000-1999)
	ErrSuccess            ErrorCode = 0    // 成功
	ErrInternalServer     ErrorCode = 1000 // 服务器内部错误
	ErrInvalidParams      ErrorCode = 1001 // 参数错误
	ErrUnauthorized       ErrorCode = 1002 // 未授权
	ErrForbidden          ErrorCode = 1003 // 禁止访问
	ErrNotFound           ErrorCode = 1004 // 资源未找到
	ErrMethodNotAllowed   ErrorCode = 1005 // 方法不允许
	ErrTooManyRequests    ErrorCode = 1006 // 请求过于频繁
	ErrServiceUnavailable ErrorCode = 1007 // 服务不可用

	// 文件相关错误码 (2000-2999)
	ErrFileNotFound       ErrorCode = 2000 // 文件未找到
	ErrFileAlreadyExists  ErrorCode = 2001 // 文件已存在
	ErrFileUploadFailed   ErrorCode = 2002 // 文件上传失败
	ErrFileDeleteFailed   ErrorCode = 2003 // 文件删除失败
	ErrFileReadFailed     ErrorCode = 2004 // 文件读取失败
	ErrFileWriteFailed    ErrorCode = 2005 // 文件写入失败
	ErrFileSizeTooLarge   ErrorCode = 2006 // 文件大小超限
	ErrFileTypeNotAllowed ErrorCode = 2007 // 文件类型不允许
	ErrFileCorrupted      ErrorCode = 2008 // 文件损坏
	ErrFileHashMismatch   ErrorCode = 2009 // 文件哈希不匹配

	// OSS相关错误码 (3000-3999)
	ErrOSSConfigNotFound       ErrorCode = 3000 // OSS配置未找到
	ErrOSSConfigInvalid        ErrorCode = 3001 // OSS配置无效
	ErrOSSConnectionFailed     ErrorCode = 3002 // OSS连接失败
	ErrOSSUploadFailed         ErrorCode = 3003 // OSS上传失败
	ErrOSSDownloadFailed       ErrorCode = 3004 // OSS下载失败
	ErrOSSDeleteFailed         ErrorCode = 3005 // OSS删除失败
	ErrOSSListFailed           ErrorCode = 3006 // OSS列表获取失败
	ErrOSSSyncFailed           ErrorCode = 3007 // OSS同步失败
	ErrOSSProviderNotSupported ErrorCode = 3008 // OSS提供商不支持

	// 数据库相关错误码 (4000-4999)
	ErrDatabaseConnection  ErrorCode = 4000 // 数据库连接错误
	ErrDatabaseQuery       ErrorCode = 4001 // 数据库查询错误
	ErrDatabaseInsert      ErrorCode = 4002 // 数据库插入错误
	ErrDatabaseUpdate      ErrorCode = 4003 // 数据库更新错误
	ErrDatabaseDelete      ErrorCode = 4004 // 数据库删除错误
	ErrDatabaseTransaction ErrorCode = 4005 // 数据库事务错误
	ErrRecordNotFound      ErrorCode = 4006 // 记录未找到
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
// @Param details query string true "详细错误信息"
// @Return 带详细信息的错误
func (e *AppError) WithDetails(details string) *AppError {
	e.Details = details
	return e
}

// WithOriginalError 添加原始错误
// @Description 为错误添加原始错误信息
// @Param err query string true "原始错误"
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
// @Param code query int true "错误码"
// @Param message query string true "错误消息"
// @Return 应用错误实例
func New(code ErrorCode, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
	}
}

// NewWithDetails 创建带详细信息的应用错误
// @Description 创建带详细信息的应用程序错误
// @Param code query int true "错误码"
// @Param message query string true "错误消息"
// @Param details query string true "详细错误信息"
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
// @Param code query int true "错误码"
// @Param message query string true "错误消息"
// @Param err query string true "原始错误"
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
// @Param err query string true "待判断的错误"
// @Return 是否为应用错误
func IsAppError(err error) bool {
	_, ok := err.(*AppError)
	return ok
}

// GetAppError 获取应用错误
// @Description 从错误中提取应用程序错误
// @Param err query string true "原始错误"
// @Return 应用错误实例和是否成功提取的标志
func GetAppError(err error) (*AppError, bool) {
	appErr, ok := err.(*AppError)
	return appErr, ok
}

// 预定义的常用错误
var (
	// 通用错误
	ErrInternalServerError = New(ErrInternalServer, GetErrorMessage(ErrInternalServer))
	ErrInvalidParameters   = New(ErrInvalidParams, GetErrorMessage(ErrInvalidParams))
	ErrUnauthorizedAccess  = New(ErrUnauthorized, GetErrorMessage(ErrUnauthorized))
	ErrForbiddenAccess     = New(ErrForbidden, GetErrorMessage(ErrForbidden))
	ErrResourceNotFound    = New(ErrNotFound, GetErrorMessage(ErrNotFound))

	// 文件相关错误
	ErrFileNotFoundError       = New(ErrFileNotFound, GetErrorMessage(ErrFileNotFound))
	ErrFileAlreadyExistsError  = New(ErrFileAlreadyExists, GetErrorMessage(ErrFileAlreadyExists))
	ErrFileUploadFailedError   = New(ErrFileUploadFailed, GetErrorMessage(ErrFileUploadFailed))
	ErrFileDeleteFailedError   = New(ErrFileDeleteFailed, GetErrorMessage(ErrFileDeleteFailed))
	ErrFileReadFailedError     = New(ErrFileReadFailed, GetErrorMessage(ErrFileReadFailed))
	ErrFileWriteFailedError    = New(ErrFileWriteFailed, GetErrorMessage(ErrFileWriteFailed))
	ErrFileSizeTooLargeError   = New(ErrFileSizeTooLarge, GetErrorMessage(ErrFileSizeTooLarge))
	ErrFileTypeNotAllowedError = New(ErrFileTypeNotAllowed, GetErrorMessage(ErrFileTypeNotAllowed))
	ErrFileCorruptedError      = New(ErrFileCorrupted, GetErrorMessage(ErrFileCorrupted))
	ErrFileHashMismatchError   = New(ErrFileHashMismatch, GetErrorMessage(ErrFileHashMismatch))

	// OSS相关错误
	ErrOSSConfigNotFoundError       = New(ErrOSSConfigNotFound, GetErrorMessage(ErrOSSConfigNotFound))
	ErrOSSConfigInvalidError        = New(ErrOSSConfigInvalid, GetErrorMessage(ErrOSSConfigInvalid))
	ErrOSSConnectionFailedError     = New(ErrOSSConnectionFailed, GetErrorMessage(ErrOSSConnectionFailed))
	ErrOSSUploadFailedError         = New(ErrOSSUploadFailed, GetErrorMessage(ErrOSSUploadFailed))
	ErrOSSDownloadFailedError       = New(ErrOSSDownloadFailed, GetErrorMessage(ErrOSSDownloadFailed))
	ErrOSSDeleteFailedError         = New(ErrOSSDeleteFailed, GetErrorMessage(ErrOSSDeleteFailed))
	ErrOSSListFailedError           = New(ErrOSSListFailed, GetErrorMessage(ErrOSSListFailed))
	ErrOSSSyncFailedError           = New(ErrOSSSyncFailed, GetErrorMessage(ErrOSSSyncFailed))
	ErrOSSProviderNotSupportedError = New(ErrOSSProviderNotSupported, GetErrorMessage(ErrOSSProviderNotSupported))

	// 数据库相关错误
	ErrDatabaseConnectionError  = New(ErrDatabaseConnection, GetErrorMessage(ErrDatabaseConnection))
	ErrDatabaseQueryError       = New(ErrDatabaseQuery, GetErrorMessage(ErrDatabaseQuery))
	ErrDatabaseInsertError      = New(ErrDatabaseInsert, GetErrorMessage(ErrDatabaseInsert))
	ErrDatabaseUpdateError      = New(ErrDatabaseUpdate, GetErrorMessage(ErrDatabaseUpdate))
	ErrDatabaseDeleteError      = New(ErrDatabaseDelete, GetErrorMessage(ErrDatabaseDelete))
	ErrDatabaseTransactionError = New(ErrDatabaseTransaction, GetErrorMessage(ErrDatabaseTransaction))
	ErrRecordNotFoundError      = New(ErrRecordNotFound, GetErrorMessage(ErrRecordNotFound))
	ErrRecordAlreadyExistsError = New(ErrRecordAlreadyExists, GetErrorMessage(ErrRecordAlreadyExists))
)

// 错误码到i18n键的映射
var errorCodeToKeyMap = map[ErrorCode]string{
	ErrSuccess:            "success",
	ErrInternalServer:     "internal_server_error",
	ErrInvalidParams:      "invalid_params",
	ErrUnauthorized:       "unauthorized",
	ErrForbidden:          "forbidden",
	ErrNotFound:           "not_found",
	ErrMethodNotAllowed:   "method_not_allowed",
	ErrTooManyRequests:    "too_many_requests",
	ErrServiceUnavailable: "service_unavailable",

	ErrFileNotFound:       "file_not_found",
	ErrFileAlreadyExists:  "file_already_exists",
	ErrFileUploadFailed:   "file_upload_failed",
	ErrFileDeleteFailed:   "file_delete_failed",
	ErrFileReadFailed:     "file_read_failed",
	ErrFileWriteFailed:    "file_write_failed",
	ErrFileSizeTooLarge:   "file_size_too_large",
	ErrFileTypeNotAllowed: "file_type_not_allowed",
	ErrFileCorrupted:      "file_corrupted",
	ErrFileHashMismatch:   "file_hash_mismatch",

	ErrOSSConfigNotFound:       "oss_config_not_found",
	ErrOSSConfigInvalid:        "oss_config_invalid",
	ErrOSSConnectionFailed:     "oss_connection_failed",
	ErrOSSUploadFailed:         "oss_upload_failed",
	ErrOSSDownloadFailed:       "oss_download_failed",
	ErrOSSDeleteFailed:         "oss_delete_failed",
	ErrOSSListFailed:           "oss_list_failed",
	ErrOSSSyncFailed:           "oss_sync_failed",
	ErrOSSProviderNotSupported: "oss_provider_not_supported",

	ErrDatabaseConnection:  "database_connection",
	ErrDatabaseQuery:       "database_query",
	ErrDatabaseInsert:      "database_insert",
	ErrDatabaseUpdate:      "database_update",
	ErrDatabaseDelete:      "database_delete",
	ErrDatabaseTransaction: "database_transaction",
	ErrRecordNotFound:      "record_not_found",
	ErrRecordAlreadyExists: "record_already_exists",
}

// GetErrorMessage 根据错误码获取错误消息（使用默认语言）
// @Description 根据错误码获取对应的错误消息
// @Param code query int true "错误码"
// @Return 错误消息
func GetErrorMessage(code ErrorCode) string {
	return GetErrorMessageWithLang(code, i18n.GetInstance().GetDefaultLanguage())
}

// GetErrorMessageWithLang 根据错误码和语言获取错误消息
// @Description 根据错误码和指定语言获取对应的错误消息
// @Param code query int true "错误码"
// @Param lang query string true "语言代码，如zh-CN、en-US"
// @Return 错误消息
func GetErrorMessageWithLang(code ErrorCode, lang string) string {
	// 获取错误码对应的i18n键
	key, exists := errorCodeToKeyMap[code]
	if !exists {
		key = "unknown_error"
	}

	// 使用i18n获取翻译
	return i18n.GetInstance().Translate(key, lang)
}
