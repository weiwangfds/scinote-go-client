package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/qiniu/go-sdk/v7/auth/qbox"
	"github.com/qiniu/go-sdk/v7/storage"
	"github.com/weiwangfds/scinote/internal/database"
)

// QiniuKodoProvider 七牛云Kodo提供商实现
type QiniuKodoProvider struct {
	mac          *qbox.Mac
	bucketName   string
	bucketDomain string
	region       *storage.Region
	config       *database.OSSConfig
}

// NewQiniuKodoProvider 创建七牛云Kodo提供商实例
func NewQiniuKodoProvider(config *database.OSSConfig) (*QiniuKodoProvider, error) {
	mac := qbox.NewMac(config.AccessKey, config.SecretKey)

	// 获取区域信息
	region, err := storage.GetRegion(config.AccessKey, config.Bucket)
	if err != nil {
		return nil, fmt.Errorf("failed to get qiniu region: %w", err)
	}

	// 构建域名
	bucketDomain := config.Endpoint
	if bucketDomain == "" {
		bucketDomain = fmt.Sprintf("%s.%s", config.Bucket, region.RsHost)
	}

	return &QiniuKodoProvider{
		mac:          mac,
		bucketName:   config.Bucket,
		bucketDomain: bucketDomain,
		region:       region,
		config:       config,
	}, nil
}

// UploadFile 上传文件到七牛云Kodo
func (p *QiniuKodoProvider) UploadFile(objectKey string, reader io.Reader, contentType string) error {
	putPolicy := storage.PutPolicy{
		Scope: fmt.Sprintf("%s:%s", p.bucketName, objectKey),
	}
	upToken := putPolicy.UploadToken(p.mac)

	cfg := storage.Config{
		Region:        p.region,
		UseHTTPS:      true,
		UseCdnDomains: false,
	}

	formUploader := storage.NewFormUploader(&cfg)
	ret := storage.PutRet{}

	putExtra := storage.PutExtra{}
	if contentType != "" {
		putExtra.MimeType = contentType
	}

	err := formUploader.Put(context.Background(), &ret, upToken, objectKey, reader, -1, &putExtra)
	if err != nil {
		return fmt.Errorf("failed to upload file to qiniu kodo: %w", err)
	}

	return nil
}

// DownloadFile 从七牛云Kodo下载文件
func (p *QiniuKodoProvider) DownloadFile(objectKey string) (io.ReadCloser, error) {
	// 获取私有下载链接
	deadline := time.Now().Add(time.Hour).Unix()
	privateURL := storage.MakePrivateURL(p.mac, p.bucketDomain, objectKey, deadline)

	// 使用HTTP客户端下载文件
	resp, err := http.Get(privateURL)
	if err != nil {
		return nil, fmt.Errorf("failed to download file from qiniu kodo: %w", err)
	}

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("failed to download file, status: %s", resp.Status)
	}

	return resp.Body, nil
}

// DeleteFile 删除七牛云Kodo文件
func (p *QiniuKodoProvider) DeleteFile(objectKey string) error {
	bucketManager := storage.NewBucketManager(p.mac, &storage.Config{
		Region: p.region,
	})

	err := bucketManager.Delete(p.bucketName, objectKey)
	if err != nil {
		return fmt.Errorf("failed to delete file from qiniu kodo: %w", err)
	}

	return nil
}

// FileExists 检查文件是否存在
func (p *QiniuKodoProvider) FileExists(objectKey string) (bool, error) {
	bucketManager := storage.NewBucketManager(p.mac, &storage.Config{
		Region: p.region,
	})

	_, err := bucketManager.Stat(p.bucketName, objectKey)
	if err != nil {
		if strings.Contains(err.Error(), "no such file or directory") {
			return false, nil
		}
		return false, fmt.Errorf("failed to check file existence in qiniu kodo: %w", err)
	}

	return true, nil
}

// GetFileInfo 获取文件信息
func (p *QiniuKodoProvider) GetFileInfo(objectKey string) (*FileInfo, error) {
	bucketManager := storage.NewBucketManager(p.mac, &storage.Config{
		Region: p.region,
	})

	fileInfo, err := bucketManager.Stat(p.bucketName, objectKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info from qiniu kodo: %w", err)
	}

	return &FileInfo{
		Key:          objectKey,
		Size:         fileInfo.Fsize,
		LastModified: time.Unix(fileInfo.PutTime/10000000, 0).Format(time.RFC3339),
		ETag:         fileInfo.Hash,
		ContentType:  fileInfo.MimeType,
	}, nil
}

// ListFiles 列出文件
func (p *QiniuKodoProvider) ListFiles(prefix string, maxKeys int) ([]FileInfo, error) {
	bucketManager := storage.NewBucketManager(p.mac, &storage.Config{
		Region: p.region,
	})

	entries, _, _, hasNext, err := bucketManager.ListFiles(p.bucketName, prefix, "", "", maxKeys)
	if err != nil {
		return nil, fmt.Errorf("failed to list files from qiniu kodo: %w", err)
	}

	var files []FileInfo
	for _, entry := range entries {
		files = append(files, FileInfo{
			Key:          entry.Key,
			Size:         entry.Fsize,
			LastModified: time.Unix(entry.PutTime/10000000, 0).Format(time.RFC3339),
			ETag:         entry.Hash,
			ContentType:  entry.MimeType,
		})
	}

	// 如果还有更多文件但受限于maxKeys，可以在这里处理
	_ = hasNext

	return files, nil
}

// TestConnection 测试连接
func (p *QiniuKodoProvider) TestConnection() error {
	bucketManager := storage.NewBucketManager(p.mac, &storage.Config{
		Region: p.region,
	})

	// 尝试列出存储桶中的文件（限制为1个）
	_, _, _, _, err := bucketManager.ListFiles(p.bucketName, "", "", "", 1)
	if err != nil {
		return fmt.Errorf("failed to test qiniu kodo connection: %w", err)
	}

	return nil
}
