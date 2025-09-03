// Package service 提供腾讯云COS对象存储服务的实现
// 本文件实现了腾讯云COS（Cloud Object Storage）的OSS接口
// 支持文件上传、下载、删除、列表、存在性检查等基本操作
package service

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/tencentyun/cos-go-sdk-v5"
	"github.com/weiwangfds/scinote/internal/database"
)

// TencentCOSProvider 腾讯云COS提供商实现
// 实现了OSS接口，提供腾讯云COS对象存储服务
type TencentCOSProvider struct {
	client *cos.Client         // COS客户端实例
	config *database.OSSConfig // OSS配置信息
}

// NewTencentCOSProvider 创建腾讯云COS提供商实例
// 功能: 根据配置信息创建腾讯云COS客户端实例
// 参数:
//   config: OSS配置信息，包含访问密钥、区域、存储桶等
// 返回:
//   *TencentCOSProvider: 腾讯云COS提供商实例
//   error: 创建过程中的错误信息
func NewTencentCOSProvider(config *database.OSSConfig) (*TencentCOSProvider, error) {
	log.Printf("[腾讯云COS] 开始创建COS提供商实例, 存储桶: %s, 区域: %s", config.Bucket, config.Region)
	
	// 构建URL
	bucketURL := fmt.Sprintf("https://%s.cos.%s.myqcloud.com", config.Bucket, config.Region)
	if config.Endpoint != "" {
		log.Printf("[腾讯云COS] 使用自定义端点: %s", config.Endpoint)
		bucketURL = config.Endpoint
	} else {
		log.Printf("[腾讯云COS] 使用默认端点: %s", bucketURL)
	}

	log.Printf("[腾讯云COS] 正在解析存储桶URL: %s", bucketURL)
	u, err := url.Parse(bucketURL)
	if err != nil {
		log.Printf("[腾讯云COS] 解析存储桶URL失败: %v", err)
		return nil, fmt.Errorf("failed to parse bucket URL: %w", err)
	}
	log.Println("[腾讯云COS] 存储桶URL解析成功")

	// 创建COS客户端
	log.Println("[腾讯云COS] 正在创建COS客户端实例")
	client := cos.NewClient(&cos.BaseURL{BucketURL: u}, &http.Client{
		Transport: &cos.AuthorizationTransport{
			SecretID:  config.AccessKey,
			SecretKey: config.SecretKey,
		},
	})
	log.Println("[腾讯云COS] COS客户端实例创建成功")

	provider := &TencentCOSProvider{
		client: client,
		config: config,
	}
	
	log.Printf("[腾讯云COS] 腾讯云COS提供商实例创建完成, 存储桶: %s", config.Bucket)
	return provider, nil
}

// UploadFile 上传文件到腾讯云COS
// 功能: 将文件上传到腾讯云COS存储桶
// 参数:
//   objectKey: 对象键（文件在COS中的路径）
//   reader: 文件内容读取器
//   contentType: 文件内容类型
// 返回:
//   error: 上传过程中的错误信息
func (p *TencentCOSProvider) UploadFile(objectKey string, reader io.Reader, contentType string) error {
	log.Printf("[腾讯云COS] 开始上传文件, 对象键: %s, 内容类型: %s", objectKey, contentType)
	
	options := &cos.ObjectPutOptions{}
	if contentType != "" {
		log.Printf("[腾讯云COS] 设置文件内容类型: %s", contentType)
		options.ObjectPutHeaderOptions = &cos.ObjectPutHeaderOptions{
			ContentType: contentType,
		}
	} else {
		log.Println("[腾讯云COS] 未指定内容类型，使用默认值")
	}

	log.Printf("[腾讯云COS] 正在上传文件到COS, 对象键: %s", objectKey)
	_, err := p.client.Object.Put(context.Background(), objectKey, reader, options)
	if err != nil {
		log.Printf("[腾讯云COS] 文件上传失败: %v", err)
		return fmt.Errorf("failed to upload file to tencent cos: %w", err)
	}

	log.Printf("[腾讯云COS] 文件上传成功, 对象键: %s", objectKey)
	return nil
}

// DownloadFile 从腾讯云COS下载文件
// 功能: 从腾讯云COS存储桶下载指定文件
// 参数:
//   objectKey: 对象键（文件在COS中的路径）
// 返回:
//   io.ReadCloser: 文件内容读取器
//   error: 下载过程中的错误信息
func (p *TencentCOSProvider) DownloadFile(objectKey string) (io.ReadCloser, error) {
	log.Printf("[腾讯云COS] 开始下载文件, 对象键: %s", objectKey)
	
	log.Printf("[腾讯云COS] 正在从COS下载文件, 对象键: %s", objectKey)
	resp, err := p.client.Object.Get(context.Background(), objectKey, nil)
	if err != nil {
		log.Printf("[腾讯云COS] 文件下载失败: %v", err)
		return nil, fmt.Errorf("failed to download file from tencent cos: %w", err)
	}

	log.Printf("[腾讯云COS] 文件下载成功, 对象键: %s", objectKey)
	return resp.Body, nil
}

// DeleteFile 删除腾讯云COS文件
// 功能: 从腾讯云COS存储桶删除指定文件
// 参数:
//   objectKey: 对象键（文件在COS中的路径）
// 返回:
//   error: 删除过程中的错误信息
func (p *TencentCOSProvider) DeleteFile(objectKey string) error {
	log.Printf("[腾讯云COS] 开始删除文件, 对象键: %s", objectKey)
	
	log.Printf("[腾讯云COS] 正在从COS删除文件, 对象键: %s", objectKey)
	_, err := p.client.Object.Delete(context.Background(), objectKey)
	if err != nil {
		log.Printf("[腾讯云COS] 文件删除失败: %v", err)
		return fmt.Errorf("failed to delete file from tencent cos: %w", err)
	}

	log.Printf("[腾讯云COS] 文件删除成功, 对象键: %s", objectKey)
	return nil
}

// FileExists 检查文件是否存在
// 功能: 检查指定文件在腾讯云COS中是否存在
// 参数:
//   objectKey: 对象键（文件在COS中的路径）
// 返回:
//   bool: 文件是否存在
//   error: 检查过程中的错误信息
func (p *TencentCOSProvider) FileExists(objectKey string) (bool, error) {
	log.Printf("[腾讯云COS] 开始检查文件是否存在, 对象键: %s", objectKey)
	
	log.Printf("[腾讯云COS] 正在检查文件存在性, 对象键: %s", objectKey)
	_, err := p.client.Object.Head(context.Background(), objectKey, nil)
	if err != nil {
		if cos.IsNotFoundError(err) {
			log.Printf("[腾讯云COS] 文件不存在, 对象键: %s", objectKey)
			return false, nil
		}
		log.Printf("[腾讯云COS] 检查文件存在性失败: %v", err)
		return false, fmt.Errorf("failed to check file existence in tencent cos: %w", err)
	}

	log.Printf("[腾讯云COS] 文件存在, 对象键: %s", objectKey)
	return true, nil
}

// GetFileInfo 获取文件信息
// 功能: 获取腾讯云COS中指定文件的详细信息
// 参数:
//   objectKey: 对象键（文件在COS中的路径）
// 返回:
//   *FileInfo: 文件信息结构体
//   error: 获取过程中的错误信息
func (p *TencentCOSProvider) GetFileInfo(objectKey string) (*FileInfo, error) {
	log.Printf("[腾讯云COS] 开始获取文件信息, 对象键: %s", objectKey)
	
	log.Printf("[腾讯云COS] 正在获取文件元数据, 对象键: %s", objectKey)
	resp, err := p.client.Object.Head(context.Background(), objectKey, nil)
	if err != nil {
		log.Printf("[腾讯云COS] 获取文件信息失败: %v", err)
		return nil, fmt.Errorf("failed to get file info from tencent cos: %w", err)
	}

	fileInfo := &FileInfo{
		Key:          objectKey,
		Size:         resp.ContentLength,
		LastModified: resp.Header.Get("Last-Modified"),
		ETag:         strings.Trim(resp.Header.Get("Etag"), "\""),
		ContentType:  resp.Header.Get("Content-Type"),
	}
	
	log.Printf("[腾讯云COS] 文件信息获取成功, 对象键: %s, 大小: %d, 类型: %s", 
		objectKey, fileInfo.Size, fileInfo.ContentType)
	return fileInfo, nil
}

// ListFiles 列出文件
// 功能: 列出腾讯云COS存储桶中指定前缀的文件
// 参数:
//   prefix: 文件前缀过滤条件
//   maxKeys: 最大返回文件数量
// 返回:
//   []FileInfo: 文件信息列表
//   error: 列出过程中的错误信息
func (p *TencentCOSProvider) ListFiles(prefix string, maxKeys int) ([]FileInfo, error) {
	log.Printf("[腾讯云COS] 开始列出文件, 前缀: %s, 最大数量: %d", prefix, maxKeys)
	
	options := &cos.BucketGetOptions{
		Prefix:  prefix,
		MaxKeys: maxKeys,
	}

	log.Printf("[腾讯云COS] 正在从COS获取文件列表, 前缀: %s", prefix)
	result, _, err := p.client.Bucket.Get(context.Background(), options)
	if err != nil {
		log.Printf("[腾讯云COS] 获取文件列表失败: %v", err)
		return nil, fmt.Errorf("failed to list files from tencent cos: %w", err)
	}

	var files []FileInfo
	log.Printf("[腾讯云COS] 正在处理文件列表, 共找到 %d 个文件", len(result.Contents))
	for _, object := range result.Contents {
		files = append(files, FileInfo{
			Key:          object.Key,
			Size:         int64(object.Size),
			LastModified: object.LastModified,
			ETag:         strings.Trim(object.ETag, "\""),
			ContentType:  "", // COS列表接口不返回ContentType
		})
	}

	log.Printf("[腾讯云COS] 文件列表获取成功, 返回 %d 个文件", len(files))
	return files, nil
}

// TestConnection 测试连接
// 功能: 测试与腾讯云COS的连接是否正常
// 返回:
//   error: 连接测试过程中的错误信息
func (p *TencentCOSProvider) TestConnection() error {
	log.Printf("[腾讯云COS] 开始测试COS连接, 存储桶: %s", p.config.Bucket)
	
	// 尝试获取存储桶信息
	log.Printf("[腾讯云COS] 正在获取存储桶信息进行连接测试, 存储桶: %s", p.config.Bucket)
	_, err := p.client.Bucket.Head(context.Background())
	if err != nil {
		log.Printf("[腾讯云COS] COS连接测试失败: %v", err)
		return fmt.Errorf("failed to test tencent cos connection: %w", err)
	}

	log.Printf("[腾讯云COS] COS连接测试成功, 存储桶: %s", p.config.Bucket)
	return nil
}
