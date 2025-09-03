package service

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/weiwangfds/scinote/internal/database"
)

// AliyunOSSProvider 阿里云OSS提供商实现
type AliyunOSSProvider struct {
	client *oss.Client
	bucket *oss.Bucket
	config *database.OSSConfig
}

// NewAliyunOSSProvider 创建阿里云OSS提供商实例
func NewAliyunOSSProvider(config *database.OSSConfig) (*AliyunOSSProvider, error) {
	// 构建endpoint
	endpoint := config.Endpoint
	if endpoint == "" {
		endpoint = fmt.Sprintf("https://oss-%s.aliyuncs.com", config.Region)
	}

	// 创建OSS客户端
	client, err := oss.New(endpoint, config.AccessKey, config.SecretKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create aliyun oss client: %w", err)
	}

	// 获取存储桶
	bucket, err := client.Bucket(config.Bucket)
	if err != nil {
		return nil, fmt.Errorf("failed to get bucket %s: %w", config.Bucket, err)
	}

	return &AliyunOSSProvider{
		client: client,
		bucket: bucket,
		config: config,
	}, nil
}

// UploadFile 上传文件到阿里云OSS
func (p *AliyunOSSProvider) UploadFile(objectKey string, reader io.Reader, contentType string) error {
	options := []oss.Option{}
	if contentType != "" {
		options = append(options, oss.ContentType(contentType))
	}

	err := p.bucket.PutObject(objectKey, reader, options...)
	if err != nil {
		return fmt.Errorf("failed to upload file to aliyun oss: %w", err)
	}

	return nil
}

// DownloadFile 从阿里云OSS下载文件
func (p *AliyunOSSProvider) DownloadFile(objectKey string) (io.ReadCloser, error) {
	body, err := p.bucket.GetObject(objectKey)
	if err != nil {
		return nil, fmt.Errorf("failed to download file from aliyun oss: %w", err)
	}

	return body, nil
}

// DeleteFile 删除阿里云OSS文件
func (p *AliyunOSSProvider) DeleteFile(objectKey string) error {
	err := p.bucket.DeleteObject(objectKey)
	if err != nil {
		return fmt.Errorf("failed to delete file from aliyun oss: %w", err)
	}

	return nil
}

// FileExists 检查文件是否存在
func (p *AliyunOSSProvider) FileExists(objectKey string) (bool, error) {
	exists, err := p.bucket.IsObjectExist(objectKey)
	if err != nil {
		return false, fmt.Errorf("failed to check file existence in aliyun oss: %w", err)
	}

	return exists, nil
}

// GetFileInfo 获取文件信息
func (p *AliyunOSSProvider) GetFileInfo(objectKey string) (*FileInfo, error) {
	meta, err := p.bucket.GetObjectMeta(objectKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info from aliyun oss: %w", err)
	}

	// 解析文件大小
	var size int64
	if sizeStr := meta.Get("Content-Length"); sizeStr != "" {
		fmt.Sscanf(sizeStr, "%d", &size)
	}

	return &FileInfo{
		Key:          objectKey,
		Size:         size,
		LastModified: meta.Get("Last-Modified"),
		ETag:         strings.Trim(meta.Get("Etag"), "\""),
		ContentType:  meta.Get("Content-Type"),
	}, nil
}

// ListFiles 列出文件
func (p *AliyunOSSProvider) ListFiles(prefix string, maxKeys int) ([]FileInfo, error) {
	options := []oss.Option{
		oss.Prefix(prefix),
		oss.MaxKeys(maxKeys),
	}

	lsRes, err := p.bucket.ListObjects(options...)
	if err != nil {
		return nil, fmt.Errorf("failed to list files from aliyun oss: %w", err)
	}

	var files []FileInfo
	for _, object := range lsRes.Objects {
		files = append(files, FileInfo{
			Key:          object.Key,
			Size:         object.Size,
			LastModified: object.LastModified.Format(time.RFC3339),
			ETag:         strings.Trim(object.ETag, "\""),
			ContentType:  object.Type,
		})
	}

	return files, nil
}

// TestConnection 测试连接
func (p *AliyunOSSProvider) TestConnection() error {
	// 尝试列出存储桶信息
	_, err := p.client.GetBucketInfo(p.config.Bucket)
	if err != nil {
		return fmt.Errorf("failed to test aliyun oss connection: %w", err)
	}

	return nil
}
