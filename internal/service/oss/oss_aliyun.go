// Package service 提供各种云存储服务的实现
// 本文件实现了阿里云OSS（Object Storage Service）的具体操作
package service

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/weiwangfds/scinote/internal/database"
	"github.com/weiwangfds/scinote/internal/logger"
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
	logger.Infof("[阿里云OSS] 初始化提供商实例, 配置名称: %s, 区域: %s, 存储桶: %s", 
		config.Name, config.Region, config.Bucket)
	
	// 构建endpoint
	endpoint := config.Endpoint
	if endpoint == "" {
		endpoint = fmt.Sprintf("https://oss-%s.aliyuncs.com", config.Region)
		logger.Infof("[阿里云OSS] 使用默认区域域名: %s, 区域: %s", endpoint, config.Region)
	} else {
		logger.Infof("[阿里云OSS] 使用自定义域名: %s", endpoint)
	}

	// 创建OSS客户端
	logger.Infof("[阿里云OSS] 创建客户端实例, 域名: %s", endpoint)
	client, err := oss.New(endpoint, config.AccessKey, config.SecretKey)
	if err != nil {
		logger.Errorf("[阿里云OSS] 创建客户端失败, 错误: %v", err)
		return nil, fmt.Errorf("failed to create aliyun oss client: %w", err)
	}
	logger.Info("[阿里云OSS] 客户端实例创建成功")

	// 获取存储桶
	logger.Infof("[阿里云OSS] 连接存储桶: %s", config.Bucket)
	bucket, err := client.Bucket(config.Bucket)
	if err != nil {
		logger.Errorf("[阿里云OSS] 连接存储桶失败, 存储桶: %s, 错误: %v", config.Bucket, err)
		return nil, fmt.Errorf("failed to get bucket %s: %w", config.Bucket, err)
	}
	logger.Infof("[阿里云OSS] 成功连接存储桶: %s", config.Bucket)

	provider := &AliyunOSSProvider{
		client: client,
		bucket: bucket,
		config: config,
	}
	
	logger.Infof("[阿里云OSS] 提供商实例初始化成功, 配置名称: %s", config.Name)
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
	logger.Infof("[阿里云OSS] 开始上传文件: %s, 内容类型: %s", objectKey, contentType)
	
	options := []oss.Option{}
	if contentType != "" {
		options = append(options, oss.ContentType(contentType))
		logger.Infof("[阿里云OSS] 设置上传内容类型: %s", contentType)
	}

	logger.Infof("[阿里云OSS] 上传文件到存储桶: %s, 对象键: %s", p.config.Bucket, objectKey)
	err := p.bucket.PutObject(objectKey, reader, options...)
	if err != nil {
		logger.Errorf("[阿里云OSS] 上传文件失败, 对象键: %s, 错误: %v", objectKey, err)
		return fmt.Errorf("failed to upload file to aliyun oss: %w", err)
	}

	logger.Infof("[阿里云OSS] 成功上传文件: %s", objectKey)
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
	logger.Infof("[阿里云OSS] 开始下载文件: %s", objectKey)
	
	body, err := p.bucket.GetObject(objectKey)
	if err != nil {
		logger.Errorf("[阿里云OSS] 下载文件失败, 对象键: %s, 错误: %v", objectKey, err)
		return nil, fmt.Errorf("failed to download file from aliyun oss: %w", err)
	}

	logger.Infof("[阿里云OSS] 成功下载文件: %s", objectKey)
	return body, nil
}

// DeleteFile 删除阿里云OSS文件
// 从OSS存储桶中删除指定的对象
// 参数:
//   - objectKey: OSS中的对象键（文件路径）
// 返回:
//   - error: 删除过程中的错误信息
func (p *AliyunOSSProvider) DeleteFile(objectKey string) error {
	logger.Infof("[阿里云OSS] 开始删除文件: %s", objectKey)
	
	err := p.bucket.DeleteObject(objectKey)
	if err != nil {
		logger.Errorf("[阿里云OSS] 删除文件失败, 对象键: %s, 错误: %v", objectKey, err)
		return fmt.Errorf("failed to delete file from aliyun oss: %w", err)
	}

	logger.Infof("[阿里云OSS] 成功删除文件: %s", objectKey)
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
	logger.Infof("[阿里云OSS] 检查文件是否存在: %s", objectKey)
	
	exists, err := p.bucket.IsObjectExist(objectKey)
	if err != nil {
		logger.Errorf("[阿里云OSS] 检查文件存在性失败, 对象键: %s, 错误: %v", objectKey, err)
		return false, fmt.Errorf("failed to check file existence in aliyun oss: %w", err)
	}

	logger.Infof("[阿里云OSS] 文件存在性检查结果, 对象键: %s, 存在: %v", objectKey, exists)
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
	logger.Infof("[阿里云OSS] 获取文件信息: %s", objectKey)
	
	meta, err := p.bucket.GetObjectMeta(objectKey)
	if err != nil {
		logger.Errorf("[阿里云OSS] 获取文件信息失败, 对象键: %s, 错误: %v", objectKey, err)
		return nil, fmt.Errorf("failed to get file info from aliyun oss: %w", err)
	}

	// 解析文件大小
	var size int64
	if sizeStr := meta.Get("Content-Length"); sizeStr != "" {
		fmt.Sscanf(sizeStr, "%d", &size)
		logger.Infof("[阿里云OSS] 解析文件大小, 对象键: %s, 大小: %d bytes", objectKey, size)
	}

	fileInfo := &FileInfo{
		Key:          objectKey,
		Size:         size,
		LastModified: meta.Get("Last-Modified"),
		ETag:         strings.Trim(meta.Get("Etag"), "\""),
		ContentType:  meta.Get("Content-Type"),
	}
	
	logger.Infof("[阿里云OSS] 成功获取文件信息, 对象键: %s, 大小: %d bytes, 内容类型: %s, 最后修改: %s", 
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
	logger.Infof("[阿里云OSS] 列出文件, 前缀: %s, 最大数量: %d", prefix, maxKeys)
	
	options := []oss.Option{
		oss.Prefix(prefix),
		oss.MaxKeys(maxKeys),
	}

	lsRes, err := p.bucket.ListObjects(options...)
	if err != nil {
		logger.Errorf("[阿里云OSS] 列出文件失败, 前缀: %s, 错误: %v", prefix, err)
		return nil, fmt.Errorf("failed to list files from aliyun oss: %w", err)
	}

	logger.Infof("[阿里云OSS] 找到 %d 个对象, 前缀: %s", len(lsRes.Objects), prefix)
	
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
		logger.Infof("[阿里云OSS] 添加文件到列表: %s, 大小: %d bytes, 最后修改: %s", 
			fileInfo.Key, fileInfo.Size, fileInfo.LastModified)
	}

	logger.Infof("[阿里云OSS] 成功列出 %d 个文件", len(files))
	return files, nil
}

// TestConnection 测试连接
// 通过获取存储桶信息来验证OSS连接是否正常
// 返回:
//   - error: 连接测试中的错误信息，nil表示连接正常
func (p *AliyunOSSProvider) TestConnection() error {
	logger.Infof("[阿里云OSS] 测试连接, 存储桶: %s", p.config.Bucket)
	
	// 尝试列出存储桶信息
	bucketInfo, err := p.client.GetBucketInfo(p.config.Bucket)
	if err != nil {
		logger.Errorf("[阿里云OSS] 连接测试失败, 存储桶: %s, 错误: %v", p.config.Bucket, err)
		return fmt.Errorf("failed to test aliyun oss connection: %w", err)
	}

	logger.Infof("[阿里云OSS] 连接测试成功, 存储桶: %s, 创建日期: %v, 位置: %s", 
		p.config.Bucket, bucketInfo.BucketInfo.CreationDate, bucketInfo.BucketInfo.Location)
	return nil
}