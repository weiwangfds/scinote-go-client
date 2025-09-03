package service

import (
	"errors"
	"fmt"

	"github.com/weiwangfds/scinote/internal/database"
	"gorm.io/gorm"
)

// OSSConfigService OSS配置服务接口
type OSSConfigService interface {
	// 创建OSS配置
	CreateOSSConfig(config *database.OSSConfig) error

	// 根据ID获取OSS配置
	GetOSSConfigByID(id uint) (*database.OSSConfig, error)

	// 获取所有OSS配置
	ListOSSConfigs() ([]database.OSSConfig, error)

	// 更新OSS配置
	UpdateOSSConfig(config *database.OSSConfig) error

	// 删除OSS配置
	DeleteOSSConfig(id uint) error

	// 激活OSS配置（同时会取消其他配置的激活状态）
	ActivateOSSConfig(id uint) error

	// 测试OSS配置连接
	TestOSSConfig(id uint) error

	// 获取当前激活的OSS配置
	GetActiveOSSConfig() (*database.OSSConfig, error)

	// 启用/禁用OSS配置
	ToggleOSSConfig(id uint, enabled bool) error
}

// ossConfigService OSS配置服务实现
type ossConfigService struct {
	db      *gorm.DB
	factory *OSSProviderFactory
}

// NewOSSConfigService 创建OSS配置服务实例
func NewOSSConfigService(db *gorm.DB) OSSConfigService {
	return &ossConfigService{
		db:      db,
		factory: &OSSProviderFactory{},
	}
}

// CreateOSSConfig 创建OSS配置
func (s *ossConfigService) CreateOSSConfig(config *database.OSSConfig) error {
	// 验证配置
	if err := s.validateOSSConfig(config); err != nil {
		return err
	}

	// 如果这是第一个配置，自动设为激活状态
	var count int64
	s.db.Model(&database.OSSConfig{}).Count(&count)
	if count == 0 {
		config.IsActive = true
	}

	// 如果设置为激活状态，需要先取消其他配置的激活状态
	if config.IsActive {
		if err := s.deactivateAllConfigs(); err != nil {
			return fmt.Errorf("failed to deactivate other configs: %w", err)
		}
	}

	return s.db.Create(config).Error
}

// GetOSSConfigByID 根据ID获取OSS配置
func (s *ossConfigService) GetOSSConfigByID(id uint) (*database.OSSConfig, error) {
	var config database.OSSConfig
	if err := s.db.First(&config, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("OSS config not found with id: %d", id)
		}
		return nil, err
	}
	return &config, nil
}

// ListOSSConfigs 获取所有OSS配置
func (s *ossConfigService) ListOSSConfigs() ([]database.OSSConfig, error) {
	var configs []database.OSSConfig
	if err := s.db.Order("created_at DESC").Find(&configs).Error; err != nil {
		return nil, err
	}
	return configs, nil
}

// UpdateOSSConfig 更新OSS配置
func (s *ossConfigService) UpdateOSSConfig(config *database.OSSConfig) error {
	// 验证配置
	if err := s.validateOSSConfig(config); err != nil {
		return err
	}

	// 获取原有配置
	var existingConfig database.OSSConfig
	if err := s.db.First(&existingConfig, config.ID).Error; err != nil {
		return fmt.Errorf("OSS config not found: %w", err)
	}

	// 如果要激活此配置，需要先取消其他配置的激活状态
	if config.IsActive && !existingConfig.IsActive {
		if err := s.deactivateAllConfigs(); err != nil {
			return fmt.Errorf("failed to deactivate other configs: %w", err)
		}
	}

	return s.db.Save(config).Error
}

// DeleteOSSConfig 删除OSS配置
func (s *ossConfigService) DeleteOSSConfig(id uint) error {
	// 检查是否为激活配置
	var config database.OSSConfig
	if err := s.db.First(&config, id).Error; err != nil {
		return fmt.Errorf("OSS config not found: %w", err)
	}

	if config.IsActive {
		return fmt.Errorf("cannot delete active OSS configuration")
	}

	return s.db.Delete(&database.OSSConfig{}, id).Error
}

// ActivateOSSConfig 激活OSS配置
func (s *ossConfigService) ActivateOSSConfig(id uint) error {
	// 先取消所有配置的激活状态
	if err := s.deactivateAllConfigs(); err != nil {
		return fmt.Errorf("failed to deactivate other configs: %w", err)
	}

	// 激活指定配置
	if err := s.db.Model(&database.OSSConfig{}).Where("id = ?", id).
		Update("is_active", true).Error; err != nil {
		return fmt.Errorf("failed to activate OSS config: %w", err)
	}

	return nil
}

// TestOSSConfig 测试OSS配置连接
func (s *ossConfigService) TestOSSConfig(id uint) error {
	config, err := s.GetOSSConfigByID(id)
	if err != nil {
		return err
	}

	provider, err := s.factory.CreateProvider(config)
	if err != nil {
		return fmt.Errorf("failed to create OSS provider: %w", err)
	}

	return provider.TestConnection()
}

// GetActiveOSSConfig 获取当前激活的OSS配置
func (s *ossConfigService) GetActiveOSSConfig() (*database.OSSConfig, error) {
	var config database.OSSConfig
	if err := s.db.Where("is_active = ? AND is_enabled = ?", true, true).First(&config).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("no active OSS configuration found")
		}
		return nil, err
	}
	return &config, nil
}

// ToggleOSSConfig 启用/禁用OSS配置
func (s *ossConfigService) ToggleOSSConfig(id uint, enabled bool) error {
	// 如果要禁用激活的配置，先检查是否有其他可用配置
	if !enabled {
		var config database.OSSConfig
		if err := s.db.First(&config, id).Error; err != nil {
			return fmt.Errorf("OSS config not found: %w", err)
		}

		if config.IsActive {
			return fmt.Errorf("cannot disable active OSS configuration")
		}
	}

	return s.db.Model(&database.OSSConfig{}).Where("id = ?", id).
		Update("is_enabled", enabled).Error
}

// validateOSSConfig 验证OSS配置
func (s *ossConfigService) validateOSSConfig(config *database.OSSConfig) error {
	if config.Name == "" {
		return fmt.Errorf("configuration name is required")
	}

	if config.Provider == "" {
		return fmt.Errorf("OSS provider is required")
	}

	// 验证支持的提供商
	supportedProviders := []string{"aliyun", "tencent", "qiniu"}
	isSupported := false
	for _, provider := range supportedProviders {
		if config.Provider == provider {
			isSupported = true
			break
		}
	}
	if !isSupported {
		return fmt.Errorf("unsupported OSS provider: %s", config.Provider)
	}

	if config.Region == "" {
		return fmt.Errorf("region is required")
	}

	if config.Bucket == "" {
		return fmt.Errorf("bucket name is required")
	}

	if config.AccessKey == "" {
		return fmt.Errorf("access key is required")
	}

	if config.SecretKey == "" {
		return fmt.Errorf("secret key is required")
	}

	// 检查配置名称是否重复
	var count int64
	query := s.db.Model(&database.OSSConfig{}).Where("name = ?", config.Name)
	if config.ID > 0 {
		query = query.Where("id != ?", config.ID)
	}
	query.Count(&count)

	if count > 0 {
		return fmt.Errorf("configuration name already exists: %s", config.Name)
	}

	return nil
}

// deactivateAllConfigs 取消所有配置的激活状态
func (s *ossConfigService) deactivateAllConfigs() error {
	return s.db.Model(&database.OSSConfig{}).Where("is_active = ?", true).
		Update("is_active", false).Error
}
