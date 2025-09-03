package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/tencentyun/cos-go-sdk-v5"
	"github.com/weiwangfds/scinote/internal/database"
)

// TencentCOSProvider 腾讯云COS提供商实现
type TencentCOSProvider struct {
	client *cos.Client
	config *database.OSSConfig
}

// NewTencentCOSProvider 创建腾讯云COS提供商实例
func NewTencentCOSProvider(config *database.OSSConfig) (*TencentCOSProvider, error) {
	// 构建URL
	bucketURL := fmt.Sprintf("https://%s.cos.%s.myqcloud.com", config.Bucket, config.Region)
	if config.Endpoint != "" {
		bucketURL = config.Endpoint
	}

	u, err := url.Parse(bucketURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse bucket URL: %w", err)
	}

	// 创建COS客户端
	client := cos.NewClient(&cos.BaseURL{BucketURL: u}, &http.Client{
		Transport: &cos.AuthorizationTransport{
			SecretID:  config.AccessKey,
			SecretKey: config.SecretKey,
		},
	})

	return &TencentCOSProvider{
		client: client,
		config: config,
	}, nil
}

// UploadFile 上传文件到腾讯云COS
func (p *TencentCOSProvider) UploadFile(objectKey string, reader io.Reader, contentType string) error {
	options := &cos.ObjectPutOptions{}
	if contentType != "" {
		options.ObjectPutHeaderOptions = &cos.ObjectPutHeaderOptions{
			ContentType: contentType,
		}
	}

	_, err := p.client.Object.Put(context.Background(), objectKey, reader, options)
	if err != nil {
		return fmt.Errorf("failed to upload file to tencent cos: %w", err)
	}

	return nil
}

// DownloadFile 从腾讯云COS下载文件
func (p *TencentCOSProvider) DownloadFile(objectKey string) (io.ReadCloser, error) {
	resp, err := p.client.Object.Get(context.Background(), objectKey, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to download file from tencent cos: %w", err)
	}

	return resp.Body, nil
}

// DeleteFile 删除腾讯云COS文件
func (p *TencentCOSProvider) DeleteFile(objectKey string) error {
	_, err := p.client.Object.Delete(context.Background(), objectKey)
	if err != nil {
		return fmt.Errorf("failed to delete file from tencent cos: %w", err)
	}

	return nil
}

// FileExists 检查文件是否存在
func (p *TencentCOSProvider) FileExists(objectKey string) (bool, error) {
	_, err := p.client.Object.Head(context.Background(), objectKey, nil)
	if err != nil {
		if cos.IsNotFoundError(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check file existence in tencent cos: %w", err)
	}

	return true, nil
}

// GetFileInfo 获取文件信息
func (p *TencentCOSProvider) GetFileInfo(objectKey string) (*FileInfo, error) {
	resp, err := p.client.Object.Head(context.Background(), objectKey, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info from tencent cos: %w", err)
	}

	return &FileInfo{
		Key:          objectKey,
		Size:         resp.ContentLength,
		LastModified: resp.Header.Get("Last-Modified"),
		ETag:         strings.Trim(resp.Header.Get("Etag"), "\""),
		ContentType:  resp.Header.Get("Content-Type"),
	}, nil
}

// ListFiles 列出文件
func (p *TencentCOSProvider) ListFiles(prefix string, maxKeys int) ([]FileInfo, error) {
	options := &cos.BucketGetOptions{
		Prefix:  prefix,
		MaxKeys: maxKeys,
	}

	result, _, err := p.client.Bucket.Get(context.Background(), options)
	if err != nil {
		return nil, fmt.Errorf("failed to list files from tencent cos: %w", err)
	}

	var files []FileInfo
	for _, object := range result.Contents {
		files = append(files, FileInfo{
			Key:          object.Key,
			Size:         int64(object.Size),
			LastModified: object.LastModified,
			ETag:         strings.Trim(object.ETag, "\""),
			ContentType:  "", // COS列表接口不返回ContentType
		})
	}

	return files, nil
}

// TestConnection 测试连接
func (p *TencentCOSProvider) TestConnection() error {
	// 尝试获取存储桶信息
	_, err := p.client.Bucket.Head(context.Background())
	if err != nil {
		return fmt.Errorf("failed to test tencent cos connection: %w", err)
	}

	return nil
}
