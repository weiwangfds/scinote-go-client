package service

import (
	"io"

	"github.com/weiwangfds/scinote/internal/database"
)

// OSSProvider OSS提供商接口
type OSSProvider interface {
	// 上传文件到OSS
	UploadFile(objectKey string, reader io.Reader, contentType string) error

	// 从OSS下载文件
	DownloadFile(objectKey string) (io.ReadCloser, error)

	// 删除OSS文件
	DeleteFile(objectKey string) error

	// 检查文件是否存在
	FileExists(objectKey string) (bool, error)

	// 获取文件信息
	GetFileInfo(objectKey string) (*FileInfo, error)

	// 列出文件
	ListFiles(prefix string, maxKeys int) ([]FileInfo, error)

	// 测试连接
	TestConnection() error
}

// FileInfo OSS文件信息
type FileInfo struct {
	Key          string `json:"key"`           // 文件键名
	Size         int64  `json:"size"`          // 文件大小
	LastModified string `json:"last_modified"` // 最后修改时间
	ETag         string `json:"etag"`          // ETag
	ContentType  string `json:"content_type"`  // 内容类型
}

// OSSProviderFactory OSS提供商工厂
type OSSProviderFactory struct{}

// CreateProvider 根据配置创建OSS提供商实例
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
