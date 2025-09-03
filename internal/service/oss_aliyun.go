// Package service 提供各种云存储服务的实现
// 本文件实现了阿里云OSS（Object Storage Service）的具体操作
package service

import (
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/weiwangfds/scinote/internal/database"
)

// AliyunOSSProvider 阿里云OSS提供商实现
// 实现了OSSProvider接口，提供阿里云对象存储服务的完整功能
// 包括文件上传、下载、删除、列表查询等操作
type AliyunOSSProvider struct {
	client *oss.Client        // 阿里云OSS客户端实例
	bucket *oss.Bucket        // OSS存储桶实例
	config *database.OSSConfig // OSS配置信息
}

// NewAliyunOSSProvider 创建阿里云OSS提供商实例
// 根据配置信息初始化阿里云OSS客户端和存储桶连接
// 参数:
//   - config: OSS配置信息，包含访问密钥、区域、存储桶等
// 返回:
//   - *AliyunOSSProvider: 初始化完成的阿里云OSS提供商实例
//   - error: 初始化过程中的错误信息
func NewAliyunOSSProvider(config *database.OSSConfig) (*AliyunOSSProvider, error) {
	log.Printf("Initializing Aliyun OSS provider with config: %s (Region: %s, Bucket: %s)", 
		config.Name, config.Region, config.Bucket)
	
	// 构建endpoint
	endpoint := config.Endpoint
	if endpoint == "" {
		endpoint = fmt.Sprintf("https://oss-%s.aliyuncs.com", config.Region)
		log.Printf("Using default endpoint for region %s: %s", config.Region, endpoint)
	} else {
		log.Printf("Using custom endpoint: %s", endpoint)
	}

	// 创建OSS客户端
	log.Printf("Creating Aliyun OSS client with endpoint: %s", endpoint)
	client, err := oss.New(endpoint, config.AccessKey, config.SecretKey)
	if err != nil {
		log.Printf("Failed to create Aliyun OSS client: %v", err)
		return nil, fmt.Errorf("failed to create aliyun oss client: %w", err)
	}
	log.Printf("Aliyun OSS client created successfully")

	// 获取存储桶
	log.Printf("Connecting to bucket: %s", config.Bucket)
	bucket, err := client.Bucket(config.Bucket)
	if err != nil {
		log.Printf("Failed to connect to bucket %s: %v", config.Bucket, err)
		return nil, fmt.Errorf("failed to get bucket %s: %w", config.Bucket, err)
	}
	log.Printf("Successfully connected to bucket: %s", config.Bucket)

	provider := &AliyunOSSProvider{
		client: client,
		bucket: bucket,
		config: config,
	}
	
	log.Printf("Aliyun OSS provider initialized successfully for config: %s", config.Name)
	return provider, nil
}

// UploadFile 上传文件到阿里云OSS
// 将文件数据流上传到指定的OSS对象路径
// 参数:
//   - objectKey: OSS中的对象键（文件路径）
//   - reader: 文件数据流
//   - contentType: 文件的MIME类型
// 返回:
//   - error: 上传过程中的错误信息
func (p *AliyunOSSProvider) UploadFile(objectKey string, reader io.Reader, contentType string) error {
	log.Printf("Starting file upload to Aliyun OSS: %s (ContentType: %s)", objectKey, contentType)
	
	options := []oss.Option{}
	if contentType != "" {
		options = append(options, oss.ContentType(contentType))
		log.Printf("Set content type for upload: %s", contentType)
	}

	log.Printf("Uploading file to bucket %s with key: %s", p.config.Bucket, objectKey)
	err := p.bucket.PutObject(objectKey, reader, options...)
	if err != nil {
		log.Printf("Failed to upload file %s to Aliyun OSS: %v", objectKey, err)
		return fmt.Errorf("failed to upload file to aliyun oss: %w", err)
	}

	log.Printf("Successfully uploaded file to Aliyun OSS: %s", objectKey)
	return nil
}

// DownloadFile 从阿里云OSS下载文件
// 获取指定对象键的文件数据流
// 参数:
//   - objectKey: OSS中的对象键（文件路径）
// 返回:
//   - io.ReadCloser: 文件数据流，使用完毕后需要关闭
//   - error: 下载过程中的错误信息
func (p *AliyunOSSProvider) DownloadFile(objectKey string) (io.ReadCloser, error) {
	log.Printf("Starting file download from Aliyun OSS: %s", objectKey)
	
	body, err := p.bucket.GetObject(objectKey)
	if err != nil {
		log.Printf("Failed to download file %s from Aliyun OSS: %v", objectKey, err)
		return nil, fmt.Errorf("failed to download file from aliyun oss: %w", err)
	}

	log.Printf("Successfully downloaded file from Aliyun OSS: %s", objectKey)
	return body, nil
}

// DeleteFile 删除阿里云OSS文件
// 从OSS存储桶中删除指定的对象
// 参数:
//   - objectKey: OSS中的对象键（文件路径）
// 返回:
//   - error: 删除过程中的错误信息
func (p *AliyunOSSProvider) DeleteFile(objectKey string) error {
	log.Printf("Starting file deletion from Aliyun OSS: %s", objectKey)
	
	err := p.bucket.DeleteObject(objectKey)
	if err != nil {
		log.Printf("Failed to delete file %s from Aliyun OSS: %v", objectKey, err)
		return fmt.Errorf("failed to delete file from aliyun oss: %w", err)
	}

	log.Printf("Successfully deleted file from Aliyun OSS: %s", objectKey)
	return nil
}

// FileExists 检查文件是否存在
// 检查指定对象键的文件是否存在于OSS存储桶中
// 参数:
//   - objectKey: OSS中的对象键（文件路径）
// 返回:
//   - bool: 文件是否存在
//   - error: 检查过程中的错误信息
func (p *AliyunOSSProvider) FileExists(objectKey string) (bool, error) {
	log.Printf("Checking file existence in Aliyun OSS: %s", objectKey)
	
	exists, err := p.bucket.IsObjectExist(objectKey)
	if err != nil {
		log.Printf("Failed to check file existence %s in Aliyun OSS: %v", objectKey, err)
		return false, fmt.Errorf("failed to check file existence in aliyun oss: %w", err)
	}

	log.Printf("File existence check result for %s: %v", objectKey, exists)
	return exists, nil
}

// GetFileInfo 获取文件信息
// 获取指定对象键的详细文件元数据信息
// 参数:
//   - objectKey: OSS中的对象键（文件路径）
// 返回:
//   - *FileInfo: 文件信息结构体，包含大小、修改时间等
//   - error: 获取过程中的错误信息
func (p *AliyunOSSProvider) GetFileInfo(objectKey string) (*FileInfo, error) {
	log.Printf("Getting file info from Aliyun OSS: %s", objectKey)
	
	meta, err := p.bucket.GetObjectMeta(objectKey)
	if err != nil {
		log.Printf("Failed to get file info %s from Aliyun OSS: %v", objectKey, err)
		return nil, fmt.Errorf("failed to get file info from aliyun oss: %w", err)
	}

	// 解析文件大小
	var size int64
	if sizeStr := meta.Get("Content-Length"); sizeStr != "" {
		fmt.Sscanf(sizeStr, "%d", &size)
		log.Printf("Parsed file size for %s: %d bytes", objectKey, size)
	}

	fileInfo := &FileInfo{
		Key:          objectKey,
		Size:         size,
		LastModified: meta.Get("Last-Modified"),
		ETag:         strings.Trim(meta.Get("Etag"), "\""),
		ContentType:  meta.Get("Content-Type"),
	}
	
	log.Printf("Successfully retrieved file info for %s: Size=%d, ContentType=%s, LastModified=%s", 
		objectKey, fileInfo.Size, fileInfo.ContentType, fileInfo.LastModified)
	return fileInfo, nil
}

// ListFiles 列出文件
// 根据前缀和数量限制列出OSS存储桶中的文件
// 参数:
//   - prefix: 文件前缀过滤条件
//   - maxKeys: 最大返回文件数量
// 返回:
//   - []FileInfo: 文件信息列表
//   - error: 列表操作中的错误信息
func (p *AliyunOSSProvider) ListFiles(prefix string, maxKeys int) ([]FileInfo, error) {
	log.Printf("Listing files from Aliyun OSS with prefix: %s, maxKeys: %d", prefix, maxKeys)
	
	options := []oss.Option{
		oss.Prefix(prefix),
		oss.MaxKeys(maxKeys),
	}

	lsRes, err := p.bucket.ListObjects(options...)
	if err != nil {
		log.Printf("Failed to list files from Aliyun OSS with prefix %s: %v", prefix, err)
		return nil, fmt.Errorf("failed to list files from aliyun oss: %w", err)
	}

	log.Printf("Found %d objects in Aliyun OSS with prefix: %s", len(lsRes.Objects), prefix)
	
	var files []FileInfo
	for _, object := range lsRes.Objects {
		fileInfo := FileInfo{
			Key:          object.Key,
			Size:         object.Size,
			LastModified: object.LastModified.Format(time.RFC3339),
			ETag:         strings.Trim(object.ETag, "\""),
			ContentType:  object.Type,
		}
		files = append(files, fileInfo)
		log.Printf("Added file to list: %s (Size: %d, LastModified: %s)", 
			fileInfo.Key, fileInfo.Size, fileInfo.LastModified)
	}

	log.Printf("Successfully listed %d files from Aliyun OSS", len(files))
	return files, nil
}

// TestConnection 测试连接
// 通过获取存储桶信息来验证OSS连接是否正常
// 返回:
//   - error: 连接测试中的错误信息，nil表示连接正常
func (p *AliyunOSSProvider) TestConnection() error {
	log.Printf("Testing Aliyun OSS connection for bucket: %s", p.config.Bucket)
	
	// 尝试列出存储桶信息
	bucketInfo, err := p.client.GetBucketInfo(p.config.Bucket)
	if err != nil {
		log.Printf("Aliyun OSS connection test failed for bucket %s: %v", p.config.Bucket, err)
		return fmt.Errorf("failed to test aliyun oss connection: %w", err)
	}

	log.Printf("Aliyun OSS connection test successful for bucket: %s (CreationDate: %v, Location: %s)", 
		p.config.Bucket, bucketInfo.BucketInfo.CreationDate, bucketInfo.BucketInfo.Location)
	return nil
}
