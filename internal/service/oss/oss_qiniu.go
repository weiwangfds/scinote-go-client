// Package service 提供七牛云Kodo对象存储服务的实现
// 该包实现了OSS接口，提供文件上传、下载、删除、查询等功能
// 支持七牛云Kodo存储服务的完整API操作
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
	"github.com/weiwangfds/scinote/internal/logger"
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
// 返回:
//   - *QiniuKodoProvider: 七牛云Kodo提供商实例
//   - error: 初始化过程中的错误信息
func NewQiniuKodoProvider(config *database.OSSConfig) (*QiniuKodoProvider, error) {
	logger.Infof("创建七牛云Kodo提供商: 存储桶=%s, 区域=%s", 
		config.Bucket, config.Region)
	
	// 创建认证凭证
	logger.Infof("初始化七牛云认证: AccessKey=%s", 
		config.AccessKey[:min(len(config.AccessKey), 8)]+"...")
	mac := qbox.NewMac(config.AccessKey, config.SecretKey)

	// 获取区域信息
	logger.Infof("获取七牛云区域信息: 存储桶=%s", config.Bucket)
	region, err := storage.GetRegion(config.AccessKey, config.Bucket)
	if err != nil {
		logger.Errorf("获取七牛云区域失败: 存储桶=%s, 错误=%v", config.Bucket, err)
		return nil, fmt.Errorf("failed to get qiniu region: %w", err)
	}
	logger.Infof("成功获取区域信息: %s", region.RsHost)

	// 构建域名
	bucketDomain := config.Endpoint
	if bucketDomain == "" {
		bucketDomain = fmt.Sprintf("%s.%s", config.Bucket, region.RsHost)
		logger.Infof("生成存储桶域名: %s", bucketDomain)
	} else {
		logger.Infof("使用配置的端点作为存储桶域名: %s", bucketDomain)
	}

	provider := &QiniuKodoProvider{
		mac:          mac,
		bucketName:   config.Bucket,
		bucketDomain: bucketDomain,
		region:       region,
		config:       config,
	}
	
	logger.Infof("成功创建七牛云Kodo提供商: 存储桶=%s, 域名=%s", 
		config.Bucket, bucketDomain)
	return provider, nil
}

// UploadFile 上传文件到七牛云Kodo
// 将文件流上传到指定的对象键位置，支持自定义内容类型
// 参数:
//   - objectKey: 对象键（文件路径）
//   - reader: 文件内容读取器
//   - contentType: 文件MIME类型
// 返回:
//   - error: 上传过程中的错误信息
func (p *QiniuKodoProvider) UploadFile(objectKey string, reader io.Reader, contentType string) error {
	logger.Infof("上传文件到七牛云Kodo: 对象键=%s, 存储桶=%s, 内容类型=%s", 
		objectKey, p.bucketName, contentType)
	
	// 创建上传策略
	logger.Infof("创建上传策略: 对象键=%s", objectKey)
	putPolicy := storage.PutPolicy{
		Scope: fmt.Sprintf("%s:%s", p.bucketName, objectKey),
	}
	upToken := putPolicy.UploadToken(p.mac)
	logger.Infof("生成上传令牌: 对象键=%s", objectKey)

	// 配置上传参数
	cfg := storage.Config{
		Region:        p.region,
		UseHTTPS:      true,
		UseCdnDomains: false,
	}
	logger.Infof("配置上传设置: HTTPS=%v, CDN=%v", cfg.UseHTTPS, cfg.UseCdnDomains)

	// 创建表单上传器
	formUploader := storage.NewFormUploader(&cfg)
	ret := storage.PutRet{}

	// 设置上传额外参数
	putExtra := storage.PutExtra{}
	if contentType != "" {
		putExtra.MimeType = contentType
		logger.Infof("设置上传MIME类型: %s", contentType)
	}

	// 执行上传
	logger.Infof("开始文件上传: %s", objectKey)
	err := formUploader.Put(context.Background(), &ret, upToken, objectKey, reader, -1, &putExtra)
	if err != nil {
		logger.Errorf("文件上传失败: 对象键=%s, 错误=%v", objectKey, err)
		return fmt.Errorf("failed to upload file to qiniu kodo: %w", err)
	}

	logger.Infof("文件上传成功: 对象键=%s, 哈希值=%s", 
		objectKey, ret.Hash)
	return nil
}

// DownloadFile 从七牛云Kodo下载文件
// 生成私有下载链接并返回文件内容流
// 参数:
//   - objectKey: 要下载的对象键（文件路径）
// 返回:
//   - io.ReadCloser: 文件内容读取器
//   - error: 下载过程中的错误信息
func (p *QiniuKodoProvider) DownloadFile(objectKey string) (io.ReadCloser, error) {
	logger.Infof("从七牛云Kodo下载文件: 对象键=%s, 存储桶=%s", 
		objectKey, p.bucketName)
	
	// 获取私有下载链接
	deadline := time.Now().Add(time.Hour).Unix()
	logger.Infof("生成私有下载URL: 对象键=%s, 过期时间=%d", 
		objectKey, deadline)
	privateURL := storage.MakePrivateURL(p.mac, p.bucketDomain, objectKey, deadline)
	logger.Infof("生成私有下载链接: %s", 
		privateURL[:min(len(privateURL), 50)]+"...")

	// 使用HTTP客户端下载文件
	logger.Infof("发起HTTP请求下载文件: %s", objectKey)
	resp, err := http.Get(privateURL)
	if err != nil {
		logger.Errorf("发起HTTP请求失败: 对象键=%s, 错误=%v", objectKey, err)
		return nil, fmt.Errorf("failed to download file from qiniu kodo: %w", err)
	}

	// 检查响应状态
	logger.Infof("收到HTTP响应: 对象键=%s, 状态=%s", objectKey, resp.Status)
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		logger.Errorf("文件下载失败: 对象键=%s, 状态=%s", objectKey, resp.Status)
		return nil, fmt.Errorf("failed to download file, status: %s", resp.Status)
	}

	logger.Infof("文件下载成功: 对象键=%s, 内容长度=%d", 
		objectKey, resp.ContentLength)
	return resp.Body, nil
}

// DeleteFile 删除七牛云Kodo文件
// 从存储桶中删除指定的对象
// 参数:
//   - objectKey: 要删除的对象键（文件路径）
// 返回:
//   - error: 删除过程中的错误信息
func (p *QiniuKodoProvider) DeleteFile(objectKey string) error {
	logger.Infof("从七牛云Kodo删除文件: 对象键=%s, 存储桶=%s", 
		objectKey, p.bucketName)
	
	// 创建存储桶管理器
	logger.Infof("创建存储桶管理器用于删除操作")
	bucketManager := storage.NewBucketManager(p.mac, &storage.Config{
		Region: p.region,
	})

	// 执行删除操作
	logger.Infof("执行删除操作: 对象键=%s", objectKey)
	err := bucketManager.Delete(p.bucketName, objectKey)
	if err != nil {
		logger.Errorf("文件删除失败: 对象键=%s, 错误=%v", objectKey, err)
		return fmt.Errorf("failed to delete file from qiniu kodo: %w", err)
	}

	logger.Infof("文件删除成功: 对象键=%s", objectKey)
	return nil
}

// FileExists 检查文件是否存在
// 通过获取文件状态信息来判断文件是否存在于存储桶中
// 参数:
//   - objectKey: 要检查的对象键（文件路径）
// 返回:
//   - bool: 文件是否存在
//   - error: 检查过程中的错误信息
func (p *QiniuKodoProvider) FileExists(objectKey string) (bool, error) {
	logger.Infof("检查文件在七牛云Kodo中是否存在: 对象键=%s, 存储桶=%s", 
		objectKey, p.bucketName)
	
	// 创建存储桶管理器
	logger.Infof("创建存储桶管理器用于文件存在性检查")
	bucketManager := storage.NewBucketManager(p.mac, &storage.Config{
		Region: p.region,
	})

	// 获取文件状态信息
	logger.Infof("获取文件状态: 对象键=%s", objectKey)
	_, err := bucketManager.Stat(p.bucketName, objectKey)
	if err != nil {
		if strings.Contains(err.Error(), "no such file or directory") {
			logger.Infof("文件在七牛云Kodo中不存在: %s", objectKey)
			return false, nil
		}
		logger.Errorf("检查文件存在性失败: 对象键=%s, 错误=%v", objectKey, err)
		return false, fmt.Errorf("failed to check file existence in qiniu kodo: %w", err)
	}

	logger.Infof("文件在七牛云Kodo中存在: %s", objectKey)
	return true, nil
}

// GetFileInfo 获取文件信息
// 返回指定文件的详细信息，包括大小、修改时间、哈希值等
// 参数:
//   - objectKey: 要查询的对象键（文件路径）
// 返回:
//   - *FileInfo: 文件信息结构体
//   - error: 查询过程中的错误信息
func (p *QiniuKodoProvider) GetFileInfo(objectKey string) (*FileInfo, error) {
	logger.Infof("获取七牛云Kodo文件信息: 对象键=%s, 存储桶=%s", 
		objectKey, p.bucketName)
	
	// 创建存储桶管理器
	logger.Infof("创建存储桶管理器用于获取文件信息")
	bucketManager := storage.NewBucketManager(p.mac, &storage.Config{
		Region: p.region,
	})

	// 获取文件状态信息
	logger.Infof("获取文件统计信息: 对象键=%s", objectKey)
	fileInfo, err := bucketManager.Stat(p.bucketName, objectKey)
	if err != nil {
		logger.Errorf("获取文件信息失败: 对象键=%s, 错误=%v", objectKey, err)
		return nil, fmt.Errorf("failed to get file info from qiniu kodo: %w", err)
	}

	// 转换时间格式
	lastModified := time.Unix(fileInfo.PutTime/10000000, 0).Format(time.RFC3339)
	logger.Infof("获取文件信息: 对象键=%s, 大小=%d, 哈希=%s, MIME类型=%s, 最后修改时间=%s", 
		objectKey, fileInfo.Fsize, fileInfo.Hash, fileInfo.MimeType, lastModified)

	result := &FileInfo{
		Key:          objectKey,
		Size:         fileInfo.Fsize,
		LastModified: lastModified,
		ETag:         fileInfo.Hash,
		ContentType:  fileInfo.MimeType,
	}
	
	logger.Infof("成功获取文件信息: %s", objectKey)
	return result, nil
}

// ListFiles 列出文件
// 根据前缀和最大数量限制列出存储桶中的文件
// 参数:
//   - prefix: 文件前缀过滤条件
//   - maxKeys: 返回的最大文件数量
// 返回:
//   - []FileInfo: 文件信息列表
//   - error: 列举过程中的错误信息
func (p *QiniuKodoProvider) ListFiles(prefix string, maxKeys int) ([]FileInfo, error) {
	logger.Infof("列出七牛云Kodo文件: 前缀=%s, 最大数量=%d, 存储桶=%s", 
		prefix, maxKeys, p.bucketName)
	
	// 创建存储桶管理器
	logger.Infof("创建存储桶管理器用于文件列表")
	bucketManager := storage.NewBucketManager(p.mac, &storage.Config{
		Region: p.region,
	})

	// 列出文件
	logger.Infof("执行文件列表操作: 前缀=%s", prefix)
	entries, _, _, hasNext, err := bucketManager.ListFiles(p.bucketName, prefix, "", "", maxKeys)
	if err != nil {
		logger.Errorf("列出文件失败: 前缀=%s, 错误=%v", prefix, err)
		return nil, fmt.Errorf("failed to list files from qiniu kodo: %w", err)
	}

	logger.Infof("获取到 %d 个文件 (还有更多: %v)", len(entries), hasNext)

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
		
		logger.Infof("文件 %d: %s (大小: %d, 哈希: %s)", 
			i+1, entry.Key, entry.Fsize, entry.Hash)
	}

	// 如果还有更多文件但受限于maxKeys，记录日志
	if hasNext {
		logger.Infof("还有更多文件超出最大限制 (%d)", maxKeys)
	}

	logger.Infof("成功列出 %d 个文件", len(files))
	return files, nil
}

// TestConnection 测试连接
// 通过尝试列出存储桶文件来验证连接和认证是否正常
// 返回:
//   - error: 连接测试过程中的错误信息
func (p *QiniuKodoProvider) TestConnection() error {
	logger.Infof("测试七牛云Kodo连接: 存储桶=%s, 区域=%s", 
		p.bucketName, p.region.RsHost)
	
	// 创建存储桶管理器
	logger.Infof("创建存储桶管理器用于连接测试")
	bucketManager := storage.NewBucketManager(p.mac, &storage.Config{
		Region: p.region,
	})

	// 尝试列出存储桶中的文件（限制为1个）
	logger.Infof("尝试列出文件进行连接测试 (限制: 1个)")
	_, _, _, _, err := bucketManager.ListFiles(p.bucketName, "", "", "", 1)
	if err != nil {
		logger.Errorf("七牛云Kodo连接测试失败: 存储桶=%s, 错误=%v", p.bucketName, err)
		return fmt.Errorf("failed to test qiniu kodo connection: %w", err)
	}

	logger.Infof("七牛云Kodo连接测试成功: %s", p.bucketName)
	return nil
}