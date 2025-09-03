// Package service 提供OSS（对象存储服务）接口定义和工厂模式实现
// 本文件定义了统一的OSS接口，支持多种云存储提供商（阿里云、腾讯云、七牛云）
// 通过工厂模式实现不同提供商的统一管理和创建
package service

import (
	"io"

	"github.com/weiwangfds/scinote/internal/database"
)

// OSSProvider OSS提供商接口
// 定义了对象存储服务的标准操作接口，所有OSS提供商都需要实现此接口
// 支持文件上传、下载、删除、存在性检查、信息获取、列表查询和连接测试等操作
type OSSProvider interface {
	// UploadFile 上传文件到OSS
	// 参数:
	//   objectKey: 对象键（文件在OSS中的路径）
	//   reader: 文件内容读取器
	//   contentType: 文件内容类型（MIME类型）
	// 返回:
	//   error: 上传过程中的错误信息
	UploadFile(objectKey string, reader io.Reader, contentType string) error

	// DownloadFile 从OSS下载文件
	// 参数:
	//   objectKey: 对象键（文件在OSS中的路径）
	// 返回:
	//   io.ReadCloser: 文件内容读取器（需要调用者关闭）
	//   error: 下载过程中的错误信息
	DownloadFile(objectKey string) (io.ReadCloser, error)

	// DeleteFile 删除OSS文件
	// 参数:
	//   objectKey: 对象键（文件在OSS中的路径）
	// 返回:
	//   error: 删除过程中的错误信息
	DeleteFile(objectKey string) error

	// FileExists 检查文件是否存在
	// 参数:
	//   objectKey: 对象键（文件在OSS中的路径）
	// 返回:
	//   bool: 文件是否存在
	//   error: 检查过程中的错误信息
	FileExists(objectKey string) (bool, error)

	// GetFileInfo 获取文件信息
	// 参数:
	//   objectKey: 对象键（文件在OSS中的路径）
	// 返回:
	//   *FileInfo: 文件详细信息
	//   error: 获取过程中的错误信息
	GetFileInfo(objectKey string) (*FileInfo, error)

	// ListFiles 列出文件
	// 参数:
	//   prefix: 文件前缀过滤条件
	//   maxKeys: 最大返回文件数量
	// 返回:
	//   []FileInfo: 文件信息列表
	//   error: 列出过程中的错误信息
	ListFiles(prefix string, maxKeys int) ([]FileInfo, error)

	// TestConnection 测试连接
	// 返回:
	//   error: 连接测试过程中的错误信息
	TestConnection() error
}

// FileInfo OSS文件信息结构体
// 包含OSS中文件的基本元数据信息
type FileInfo struct {
	Key          string `json:"key"`           // 文件键名（对象在OSS中的唯一标识路径）
	Size         int64  `json:"size"`          // 文件大小（字节数）
	LastModified string `json:"last_modified"` // 最后修改时间（ISO 8601格式）
	ETag         string `json:"etag"`          // ETag（实体标签，用于文件完整性校验）
	ContentType  string `json:"content_type"`  // 内容类型（MIME类型，如image/jpeg、text/plain等）
}

// OSSProviderFactory OSS提供商工厂结构体
// 实现工厂模式，根据配置信息创建对应的OSS提供商实例
// 支持阿里云OSS、腾讯云COS、七牛云Kodo等多种云存储服务
type OSSProviderFactory struct{}

// CreateProvider 根据配置创建OSS提供商实例
// 功能: 工厂方法，根据配置中的提供商类型创建相应的OSS提供商实例
// 参数:
//   config: OSS配置信息，包含提供商类型、访问密钥、区域等
// 返回:
//   OSSProvider: OSS提供商接口实例
//   error: 创建过程中的错误信息
// 支持的提供商:
//   - "aliyun": 阿里云对象存储OSS
//   - "tencent": 腾讯云对象存储COS
//   - "qiniu": 七牛云对象存储Kodo
func (f *OSSProviderFactory) CreateProvider(config *database.OSSConfig) (OSSProvider, error) {
	switch config.Provider {
	case "aliyun":
		return NewAliyunOSSProvider(config)
	case "tencent":
		return NewTencentCOSProvider(config)
	case "qiniu":
		return NewQiniuKodoProvider(config)
	default:
		return nil, ErrUnsupportedProvider
	}
}
