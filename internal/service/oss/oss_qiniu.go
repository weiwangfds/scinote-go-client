// Package service 提供七牛云Kodo对象存储服务的实现
// 该包实现了OSS接口，提供文件上传、下载、删除、查询等功能
// 支持七牛云Kodo存储服务的完整API操作
package service

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/qiniu/go-sdk/v7/auth/qbox"
	"github.com/qiniu/go-sdk/v7/storage"
	"github.com/weiwangfds/scinote/internal/database"
)

// QiniuKodoProvider 七牛云Kodo提供商实现
// 实现了OSS接口，提供七牛云Kodo存储服务的完整功能
// 包括文件的上传、下载、删除、查询和管理操作
type QiniuKodoProvider struct {
	mac          *qbox.Mac           // 七牛云认证凭证
	bucketName   string              // 存储桶名称
	bucketDomain string              // 存储桶域名
	region       *storage.Region     // 存储区域信息
	config       *database.OSSConfig // OSS配置信息
}

// NewQiniuKodoProvider 创建七牛云Kodo提供商实例
// 根据配置信息初始化七牛云Kodo客户端，包括认证、区域和域名设置
// 参数:
//   - config: OSS配置信息，包含访问密钥、存储桶等信息
//
// 返回:
//   - *QiniuKodoProvider: 七牛云Kodo提供商实例
//   - error: 初始化过程中的错误信息
func NewQiniuKodoProvider(config *database.OSSConfig) (*QiniuKodoProvider, error) {
	log.Printf("Creating Qiniu Kodo provider for bucket: %s, region: %s",
		config.Bucket, config.Region)

	// 创建认证凭证
	log.Printf("Initializing Qiniu authentication with access key: %s",
		config.AccessKey[:min(len(config.AccessKey), 8)]+"...")
	mac := qbox.NewMac(config.AccessKey, config.SecretKey)

	// 获取区域信息
	log.Printf("Getting Qiniu region information for bucket: %s", config.Bucket)
	region, err := storage.GetRegion(config.AccessKey, config.Bucket)
	if err != nil {
		log.Printf("Failed to get Qiniu region for bucket %s: %v", config.Bucket, err)
		return nil, fmt.Errorf("failed to get qiniu region: %w", err)
	}
	log.Printf("Successfully retrieved region information: %s", region.RsHost)

	// 构建域名
	bucketDomain := config.Endpoint
	if bucketDomain == "" {
		bucketDomain = fmt.Sprintf("%s.%s", config.Bucket, region.RsHost)
		log.Printf("Generated bucket domain: %s", bucketDomain)
	} else {
		log.Printf("Using configured endpoint as bucket domain: %s", bucketDomain)
	}

	provider := &QiniuKodoProvider{
		mac:          mac,
		bucketName:   config.Bucket,
		bucketDomain: bucketDomain,
		region:       region,
		config:       config,
	}

	log.Printf("Successfully created Qiniu Kodo provider for bucket: %s, domain: %s",
		config.Bucket, bucketDomain)
	return provider, nil
}

// UploadFile 上传文件到七牛云Kodo
// 将文件流上传到指定的对象键位置，支持自定义内容类型
// 参数:
//   - objectKey: 对象键（文件路径）
//   - reader: 文件内容读取器
//   - contentType: 文件MIME类型
//
// 返回:
//   - error: 上传过程中的错误信息
func (p *QiniuKodoProvider) UploadFile(objectKey string, reader io.Reader, contentType string) error {
	log.Printf("Uploading file to Qiniu Kodo: %s (bucket: %s, contentType: %s)",
		objectKey, p.bucketName, contentType)

	// 创建上传策略
	log.Printf("Creating upload policy for object: %s", objectKey)
	putPolicy := storage.PutPolicy{
		Scope: fmt.Sprintf("%s:%s", p.bucketName, objectKey),
	}
	upToken := putPolicy.UploadToken(p.mac)
	log.Printf("Generated upload token for object: %s", objectKey)

	// 配置上传参数
	cfg := storage.Config{
		Region:        p.region,
		UseHTTPS:      true,
		UseCdnDomains: false,
	}
	log.Printf("Configured upload settings: HTTPS=%v, CDN=%v", cfg.UseHTTPS, cfg.UseCdnDomains)

	// 创建表单上传器
	formUploader := storage.NewFormUploader(&cfg)
	ret := storage.PutRet{}

	// 设置上传额外参数
	putExtra := storage.PutExtra{}
	if contentType != "" {
		putExtra.MimeType = contentType
		log.Printf("Set MIME type for upload: %s", contentType)
	}

	// 执行上传
	log.Printf("Starting file upload to Qiniu Kodo: %s", objectKey)
	err := formUploader.Put(context.Background(), &ret, upToken, objectKey, reader, -1, &putExtra)
	if err != nil {
		log.Printf("Failed to upload file %s to Qiniu Kodo: %v", objectKey, err)
		return fmt.Errorf("failed to upload file to qiniu kodo: %w", err)
	}

	log.Printf("Successfully uploaded file to Qiniu Kodo: %s (hash: %s)",
		objectKey, ret.Hash)
	return nil
}

// DownloadFile 从七牛云Kodo下载文件
// 生成私有下载链接并返回文件内容流
// 参数:
//   - objectKey: 要下载的对象键（文件路径）
//
// 返回:
//   - io.ReadCloser: 文件内容读取器
//   - error: 下载过程中的错误信息
func (p *QiniuKodoProvider) DownloadFile(objectKey string) (io.ReadCloser, error) {
	log.Printf("Downloading file from Qiniu Kodo: %s (bucket: %s)",
		objectKey, p.bucketName)

	// 获取私有下载链接
	deadline := time.Now().Add(time.Hour).Unix()
	log.Printf("Generating private download URL for object: %s (expires: %d)",
		objectKey, deadline)
	privateURL := storage.MakePrivateURL(p.mac, p.bucketDomain, objectKey, deadline)
	log.Printf("Generated private URL for download: %s",
		privateURL[:min(len(privateURL), 50)]+"...")

	// 使用HTTP客户端下载文件
	log.Printf("Initiating HTTP request to download file: %s", objectKey)
	resp, err := http.Get(privateURL)
	if err != nil {
		log.Printf("Failed to make HTTP request for file %s: %v", objectKey, err)
		return nil, fmt.Errorf("failed to download file from qiniu kodo: %w", err)
	}

	// 检查响应状态
	log.Printf("Received HTTP response for file %s: %s", objectKey, resp.Status)
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		log.Printf("Download failed for file %s with status: %s", objectKey, resp.Status)
		return nil, fmt.Errorf("failed to download file, status: %s", resp.Status)
	}

	log.Printf("Successfully initiated download for file: %s (content-length: %d)",
		objectKey, resp.ContentLength)
	return resp.Body, nil
}

// DeleteFile 删除七牛云Kodo文件
// 从存储桶中删除指定的对象
// 参数:
//   - objectKey: 要删除的对象键（文件路径）
//
// 返回:
//   - error: 删除过程中的错误信息
func (p *QiniuKodoProvider) DeleteFile(objectKey string) error {
	log.Printf("Deleting file from Qiniu Kodo: %s (bucket: %s)",
		objectKey, p.bucketName)

	// 创建存储桶管理器
	log.Printf("Creating bucket manager for deletion operation")
	bucketManager := storage.NewBucketManager(p.mac, &storage.Config{
		Region: p.region,
	})

	// 执行删除操作
	log.Printf("Executing delete operation for object: %s", objectKey)
	err := bucketManager.Delete(p.bucketName, objectKey)
	if err != nil {
		log.Printf("Failed to delete file %s from Qiniu Kodo: %v", objectKey, err)
		return fmt.Errorf("failed to delete file from qiniu kodo: %w", err)
	}

	log.Printf("Successfully deleted file from Qiniu Kodo: %s", objectKey)
	return nil
}

// FileExists 检查文件是否存在
// 通过获取文件状态信息来判断文件是否存在于存储桶中
// 参数:
//   - objectKey: 要检查的对象键（文件路径）
//
// 返回:
//   - bool: 文件是否存在
//   - error: 检查过程中的错误信息
func (p *QiniuKodoProvider) FileExists(objectKey string) (bool, error) {
	log.Printf("Checking file existence in Qiniu Kodo: %s (bucket: %s)",
		objectKey, p.bucketName)

	// 创建存储桶管理器
	log.Printf("Creating bucket manager for file existence check")
	bucketManager := storage.NewBucketManager(p.mac, &storage.Config{
		Region: p.region,
	})

	// 获取文件状态信息
	log.Printf("Getting file status for object: %s", objectKey)
	_, err := bucketManager.Stat(p.bucketName, objectKey)
	if err != nil {
		if strings.Contains(err.Error(), "no such file or directory") {
			log.Printf("File does not exist in Qiniu Kodo: %s", objectKey)
			return false, nil
		}
		log.Printf("Failed to check file existence for %s: %v", objectKey, err)
		return false, fmt.Errorf("failed to check file existence in qiniu kodo: %w", err)
	}

	log.Printf("File exists in Qiniu Kodo: %s", objectKey)
	return true, nil
}

// GetFileInfo 获取文件信息
// 返回指定文件的详细信息，包括大小、修改时间、哈希值等
// 参数:
//   - objectKey: 要查询的对象键（文件路径）
//
// 返回:
//   - *FileInfo: 文件信息结构体
//   - error: 查询过程中的错误信息
func (p *QiniuKodoProvider) GetFileInfo(objectKey string) (*FileInfo, error) {
	log.Printf("Getting file info from Qiniu Kodo: %s (bucket: %s)",
		objectKey, p.bucketName)

	// 创建存储桶管理器
	log.Printf("Creating bucket manager for file info retrieval")
	bucketManager := storage.NewBucketManager(p.mac, &storage.Config{
		Region: p.region,
	})

	// 获取文件状态信息
	log.Printf("Retrieving file statistics for object: %s", objectKey)
	fileInfo, err := bucketManager.Stat(p.bucketName, objectKey)
	if err != nil {
		log.Printf("Failed to get file info for %s from Qiniu Kodo: %v", objectKey, err)
		return nil, fmt.Errorf("failed to get file info from qiniu kodo: %w", err)
	}

	// 转换时间格式
	lastModified := time.Unix(fileInfo.PutTime/10000000, 0).Format(time.RFC3339)
	log.Printf("Retrieved file info for %s: size=%d, hash=%s, mimeType=%s, lastModified=%s",
		objectKey, fileInfo.Fsize, fileInfo.Hash, fileInfo.MimeType, lastModified)

	result := &FileInfo{
		Key:          objectKey,
		Size:         fileInfo.Fsize,
		LastModified: lastModified,
		ETag:         fileInfo.Hash,
		ContentType:  fileInfo.MimeType,
	}

	log.Printf("Successfully retrieved file info for: %s", objectKey)
	return result, nil
}

// ListFiles 列出文件
// 根据前缀和最大数量限制列出存储桶中的文件
// 参数:
//   - prefix: 文件前缀过滤条件
//   - maxKeys: 返回的最大文件数量
//
// 返回:
//   - []FileInfo: 文件信息列表
//   - error: 列举过程中的错误信息
func (p *QiniuKodoProvider) ListFiles(prefix string, maxKeys int) ([]FileInfo, error) {
	log.Printf("Listing files from Qiniu Kodo: prefix=%s, maxKeys=%d (bucket: %s)",
		prefix, maxKeys, p.bucketName)

	// 创建存储桶管理器
	log.Printf("Creating bucket manager for file listing")
	bucketManager := storage.NewBucketManager(p.mac, &storage.Config{
		Region: p.region,
	})

	// 列出文件
	log.Printf("Executing list files operation with prefix: %s", prefix)
	entries, _, _, hasNext, err := bucketManager.ListFiles(p.bucketName, prefix, "", "", maxKeys)
	if err != nil {
		log.Printf("Failed to list files from Qiniu Kodo with prefix %s: %v", prefix, err)
		return nil, fmt.Errorf("failed to list files from qiniu kodo: %w", err)
	}

	log.Printf("Retrieved %d files from Qiniu Kodo (hasNext: %v)", len(entries), hasNext)

	// 转换文件信息
	var files []FileInfo
	for i, entry := range entries {
		lastModified := time.Unix(entry.PutTime/10000000, 0).Format(time.RFC3339)
		fileInfo := FileInfo{
			Key:          entry.Key,
			Size:         entry.Fsize,
			LastModified: lastModified,
			ETag:         entry.Hash,
			ContentType:  entry.MimeType,
		}
		files = append(files, fileInfo)

		log.Printf("File %d: %s (size: %d, hash: %s)",
			i+1, entry.Key, entry.Fsize, entry.Hash)
	}

	// 如果还有更多文件但受限于maxKeys，记录日志
	if hasNext {
		log.Printf("More files available beyond maxKeys limit (%d)", maxKeys)
	}

	log.Printf("Successfully listed %d files from Qiniu Kodo", len(files))
	return files, nil
}

// TestConnection 测试连接
// 通过尝试列出存储桶文件来验证连接和认证是否正常
// 返回:
//   - error: 连接测试过程中的错误信息
func (p *QiniuKodoProvider) TestConnection() error {
	log.Printf("Testing connection to Qiniu Kodo (bucket: %s, region: %s)",
		p.bucketName, p.region.RsHost)

	// 创建存储桶管理器
	log.Printf("Creating bucket manager for connection test")
	bucketManager := storage.NewBucketManager(p.mac, &storage.Config{
		Region: p.region,
	})

	// 尝试列出存储桶中的文件（限制为1个）
	log.Printf("Attempting to list files for connection test (limit: 1)")
	_, _, _, _, err := bucketManager.ListFiles(p.bucketName, "", "", "", 1)
	if err != nil {
		log.Printf("Qiniu Kodo connection test failed for bucket %s: %v", p.bucketName, err)
		return fmt.Errorf("failed to test qiniu kodo connection: %w", err)
	}

	log.Printf("Successfully tested connection to Qiniu Kodo: %s", p.bucketName)
	return nil
}
