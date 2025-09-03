// Package service 提供各种云存储服务的实现
// 本文件实现了OSS配置管理服务，负责OSS配置的增删改查和状态管理
package service

import (
	"errors"
	"fmt"
	"log"

	"github.com/weiwangfds/scinote/internal/database"
	"gorm.io/gorm"
)

// OSSConfigService OSS配置服务接口
// 定义了OSS配置管理的所有操作，包括配置的增删改查、激活状态管理和连接测试
type OSSConfigService interface {
	// CreateOSSConfig 创建OSS配置
	// 验证配置参数并保存到数据库，如果是第一个配置会自动激活
	CreateOSSConfig(config *database.OSSConfig) error

	// GetOSSConfigByID 根据ID获取OSS配置
	// 从数据库中查询指定ID的OSS配置信息
	GetOSSConfigByID(id uint) (*database.OSSConfig, error)

	// ListOSSConfigs 获取所有OSS配置
	// 返回数据库中所有的OSS配置列表，按创建时间倒序排列
	ListOSSConfigs() ([]database.OSSConfig, error)

	// UpdateOSSConfig 更新OSS配置
	// 验证并更新指定的OSS配置，处理激活状态变更
	UpdateOSSConfig(config *database.OSSConfig) error

	// DeleteOSSConfig 删除OSS配置
	// 删除指定ID的OSS配置，不允许删除激活状态的配置
	DeleteOSSConfig(id uint) error

	// ActivateOSSConfig 激活OSS配置
	// 激活指定配置并取消其他配置的激活状态，确保只有一个激活配置
	ActivateOSSConfig(id uint) error

	// TestOSSConfig 测试OSS配置连接
	// 使用指定配置创建OSS提供商并测试连接是否正常
	TestOSSConfig(id uint) error

	// GetActiveOSSConfig 获取当前激活的OSS配置
	// 返回当前激活且启用的OSS配置
	GetActiveOSSConfig() (*database.OSSConfig, error)

	// ToggleOSSConfig 启用/禁用OSS配置
	// 切换指定配置的启用状态，不允许禁用激活状态的配置
	ToggleOSSConfig(id uint, enabled bool) error
}

// ossConfigService OSS配置服务实现
// 实现了OSSConfigService接口，提供完整的OSS配置管理功能
type ossConfigService struct {
	db      *gorm.DB            // 数据库连接实例
	factory *OSSProviderFactory // OSS提供商工厂，用于创建不同的OSS客户端
}

// NewOSSConfigService 创建OSS配置服务实例
// 初始化OSS配置服务，包含数据库连接和OSS提供商工厂
// 参数:
//   - db: GORM数据库连接实例
// 返回:
//   - OSSConfigService: OSS配置服务接口实现
func NewOSSConfigService(db *gorm.DB) OSSConfigService {
	log.Printf("Creating new OSS config service instance")
	service := &ossConfigService{
		db:      db,
		factory: &OSSProviderFactory{},
	}
	log.Printf("OSS config service instance created successfully")
	return service
}

// CreateOSSConfig 创建OSS配置
// 验证配置参数并保存到数据库，如果是第一个配置会自动激活
// 参数:
//   - config: 要创建的OSS配置信息
// 返回:
//   - error: 创建过程中的错误信息
func (s *ossConfigService) CreateOSSConfig(config *database.OSSConfig) error {
	log.Printf("Creating new OSS config: %s (Provider: %s, Region: %s, Bucket: %s)", 
		config.Name, config.Provider, config.Region, config.Bucket)
	
	// 验证配置
	log.Printf("Validating OSS config: %s", config.Name)
	if err := s.validateOSSConfig(config); err != nil {
		log.Printf("OSS config validation failed for %s: %v", config.Name, err)
		return err
	}
	log.Printf("OSS config validation passed for: %s", config.Name)

	// 如果这是第一个配置，自动设为激活状态
	var count int64
	s.db.Model(&database.OSSConfig{}).Count(&count)
	log.Printf("Current OSS config count: %d", count)
	if count == 0 {
		config.IsActive = true
		log.Printf("Setting first OSS config as active: %s", config.Name)
	}

	// 如果设置为激活状态，需要先取消其他配置的激活状态
	if config.IsActive {
		log.Printf("Deactivating other configs before activating: %s", config.Name)
		if err := s.deactivateAllConfigs(); err != nil {
			log.Printf("Failed to deactivate other configs: %v", err)
			return fmt.Errorf("failed to deactivate other configs: %w", err)
		}
		log.Printf("Successfully deactivated other configs")
	}

	log.Printf("Saving OSS config to database: %s", config.Name)
	if err := s.db.Create(config).Error; err != nil {
		log.Printf("Failed to create OSS config %s: %v", config.Name, err)
		return err
	}
	
	log.Printf("Successfully created OSS config: %s (ID: %d, Active: %v)", 
		config.Name, config.ID, config.IsActive)
	return nil
}

// GetOSSConfigByID 根据ID获取OSS配置
// 从数据库中查询指定ID的OSS配置信息
// 参数:
//   - id: OSS配置的唯一标识符
// 返回:
//   - *database.OSSConfig: 查询到的OSS配置信息
//   - error: 查询过程中的错误信息
func (s *ossConfigService) GetOSSConfigByID(id uint) (*database.OSSConfig, error) {
	log.Printf("Getting OSS config by ID: %d", id)
	
	var config database.OSSConfig
	if err := s.db.First(&config, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("OSS config not found with ID: %d", id)
			return nil, fmt.Errorf("OSS config not found with id: %d", id)
		}
		log.Printf("Failed to get OSS config with ID %d: %v", id, err)
		return nil, err
	}
	
	log.Printf("Successfully retrieved OSS config: %s (ID: %d, Provider: %s, Active: %v)", 
		config.Name, config.ID, config.Provider, config.IsActive)
	return &config, nil
}

// ListOSSConfigs 获取所有OSS配置
// 返回数据库中所有的OSS配置列表，按创建时间倒序排列
// 返回:
//   - []database.OSSConfig: OSS配置列表
//   - error: 查询过程中的错误信息
func (s *ossConfigService) ListOSSConfigs() ([]database.OSSConfig, error) {
	log.Printf("Listing all OSS configs")
	
	var configs []database.OSSConfig
	if err := s.db.Order("created_at DESC").Find(&configs).Error; err != nil {
		log.Printf("Failed to list OSS configs: %v", err)
		return nil, err
	}
	
	log.Printf("Successfully retrieved %d OSS configs", len(configs))
	for i, config := range configs {
		log.Printf("Config %d: %s (ID: %d, Provider: %s, Active: %v, Enabled: %v)", 
			i+1, config.Name, config.ID, config.Provider, config.IsActive, config.IsEnabled)
	}
	return configs, nil
}

// UpdateOSSConfig 更新OSS配置
// 验证并更新指定的OSS配置，处理激活状态变更
// 参数:
//   - config: 要更新的OSS配置信息（包含ID）
// 返回:
//   - error: 更新过程中的错误信息
func (s *ossConfigService) UpdateOSSConfig(config *database.OSSConfig) error {
	log.Printf("Updating OSS config ID: %d with name: %s (Provider: %s, Region: %s, Bucket: %s)", 
		config.ID, config.Name, config.Provider, config.Region, config.Bucket)
	
	// 验证配置
	log.Printf("Validating updated OSS config: %s", config.Name)
	if err := s.validateOSSConfig(config); err != nil {
		log.Printf("OSS config validation failed for %s: %v", config.Name, err)
		return err
	}
	log.Printf("OSS config validation passed for: %s", config.Name)

	// 获取原有配置
	log.Printf("Retrieving existing OSS config with ID: %d", config.ID)
	var existingConfig database.OSSConfig
	if err := s.db.First(&existingConfig, config.ID).Error; err != nil {
		log.Printf("Failed to find existing OSS config with ID %d: %v", config.ID, err)
		return fmt.Errorf("OSS config not found: %w", err)
	}
	log.Printf("Found existing OSS config: %s (Active: %v)", existingConfig.Name, existingConfig.IsActive)

	// 如果要激活此配置，需要先取消其他配置的激活状态
	if config.IsActive && !existingConfig.IsActive {
		log.Printf("Deactivating other configs before activating updated config: %s", config.Name)
		if err := s.deactivateAllConfigs(); err != nil {
			log.Printf("Failed to deactivate other configs: %v", err)
			return fmt.Errorf("failed to deactivate other configs: %w", err)
		}
		log.Printf("Successfully deactivated other configs")
	}

	log.Printf("Saving updated OSS config to database: %s", config.Name)
	if err := s.db.Save(config).Error; err != nil {
		log.Printf("Failed to update OSS config %s (ID: %d): %v", config.Name, config.ID, err)
		return err
	}
	
	log.Printf("Successfully updated OSS config: %s (ID: %d, Active: %v)", 
		config.Name, config.ID, config.IsActive)
	return nil
}

// DeleteOSSConfig 删除OSS配置
// 删除指定ID的OSS配置，不允许删除激活状态的配置
// 参数:
//   - id: 要删除的OSS配置ID
// 返回:
//   - error: 删除过程中的错误信息
func (s *ossConfigService) DeleteOSSConfig(id uint) error {
	log.Printf("Deleting OSS config with ID: %d", id)
	
	// 检查是否为激活配置
	log.Printf("Checking if OSS config ID %d is active before deletion", id)
	var config database.OSSConfig
	if err := s.db.First(&config, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("OSS config not found with ID: %d", id)
			return fmt.Errorf("OSS config not found with id: %d", id)
		}
		log.Printf("Failed to retrieve OSS config with ID %d: %v", id, err)
		return fmt.Errorf("OSS config not found: %w", err)
	}
	log.Printf("Found OSS config to delete: %s (Provider: %s, Active: %v)", 
		config.Name, config.Provider, config.IsActive)

	if config.IsActive {
		log.Printf("Cannot delete active OSS configuration: %s (ID: %d)", config.Name, id)
		return fmt.Errorf("cannot delete active OSS configuration")
	}

	log.Printf("Deleting OSS config from database: %s (ID: %d)", config.Name, id)
	if err := s.db.Delete(&database.OSSConfig{}, id).Error; err != nil {
		log.Printf("Failed to delete OSS config ID %d: %v", id, err)
		return err
	}
	
	log.Printf("Successfully deleted OSS config: %s (ID: %d)", config.Name, id)
	return nil
}

// ActivateOSSConfig 激活OSS配置
// 激活指定配置并取消其他配置的激活状态，确保只有一个激活配置
// 参数:
//   - id: 要激活的OSS配置ID
// 返回:
//   - error: 激活过程中的错误信息
func (s *ossConfigService) ActivateOSSConfig(id uint) error {
	log.Printf("Activating OSS config with ID: %d", id)
	
	// 先获取配置信息用于日志记录
	var config database.OSSConfig
	if err := s.db.First(&config, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("OSS config not found with ID: %d", id)
			return fmt.Errorf("OSS config not found with id: %d", id)
		}
		log.Printf("Failed to retrieve OSS config with ID %d: %v", id, err)
		return fmt.Errorf("OSS config not found: %w", err)
	}
	log.Printf("Found OSS config to activate: %s (Provider: %s)", config.Name, config.Provider)
	
	// 先取消所有配置的激活状态
	log.Printf("Deactivating all other OSS configs before activating ID: %d", id)
	if err := s.deactivateAllConfigs(); err != nil {
		log.Printf("Failed to deactivate other configs: %v", err)
		return fmt.Errorf("failed to deactivate other configs: %w", err)
	}
	log.Printf("Successfully deactivated all other configs")

	// 激活指定配置
	log.Printf("Setting OSS config ID %d as active", id)
	if err := s.db.Model(&database.OSSConfig{}).Where("id = ?", id).
		Update("is_active", true).Error; err != nil {
		log.Printf("Failed to activate OSS config ID %d: %v", id, err)
		return fmt.Errorf("failed to activate OSS config: %w", err)
	}
	
	log.Printf("Successfully activated OSS config: %s (ID: %d)", config.Name, id)
	return nil
}

// TestOSSConfig 测试OSS配置连接
// 使用指定配置创建OSS提供商并测试连接是否正常
// 参数:
//   - id: 要测试的OSS配置ID
// 返回:
//   - error: 测试过程中的错误信息
func (s *ossConfigService) TestOSSConfig(id uint) error {
	log.Printf("Testing OSS config connection with ID: %d", id)
	
	config, err := s.GetOSSConfigByID(id)
	if err != nil {
		log.Printf("Failed to get OSS config for testing (ID: %d): %v", id, err)
		return err
	}
	log.Printf("Retrieved OSS config for testing: %s (Provider: %s, Region: %s, Bucket: %s)", 
		config.Name, config.Provider, config.Region, config.Bucket)

	log.Printf("Creating OSS provider for testing: %s", config.Provider)
	provider, err := s.factory.CreateProvider(config)
	if err != nil {
		log.Printf("Failed to create OSS provider for %s: %v", config.Name, err)
		return fmt.Errorf("failed to create OSS provider: %w", err)
	}
	log.Printf("Successfully created OSS provider for: %s", config.Name)

	log.Printf("Testing connection for OSS config: %s", config.Name)
	if err := provider.TestConnection(); err != nil {
		log.Printf("Connection test failed for OSS config %s: %v", config.Name, err)
		return err
	}
	
	log.Printf("Connection test successful for OSS config: %s (ID: %d)", config.Name, id)
	return nil
}

// GetActiveOSSConfig 获取当前激活的OSS配置
// 返回当前激活且启用的OSS配置
// 返回:
//   - *database.OSSConfig: 当前激活的OSS配置信息
//   - error: 查询过程中的错误信息
func (s *ossConfigService) GetActiveOSSConfig() (*database.OSSConfig, error) {
	log.Printf("Getting active OSS configuration")
	
	var config database.OSSConfig
	if err := s.db.Where("is_active = ? AND is_enabled = ?", true, true).First(&config).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("No active OSS configuration found")
			return nil, fmt.Errorf("no active OSS configuration found")
		}
		log.Printf("Failed to get active OSS configuration: %v", err)
		return nil, err
	}
	
	log.Printf("Found active OSS configuration: %s (ID: %d, Provider: %s, Region: %s, Bucket: %s)", 
		config.Name, config.ID, config.Provider, config.Region, config.Bucket)
	return &config, nil
}

// ToggleOSSConfig 启用/禁用OSS配置
// 切换指定配置的启用状态，不允许禁用激活状态的配置
// 参数:
//   - id: 要切换状态的OSS配置ID
//   - enabled: 新的启用状态
// 返回:
//   - error: 操作过程中的错误信息
func (s *ossConfigService) ToggleOSSConfig(id uint, enabled bool) error {
	log.Printf("Toggling OSS config ID %d to enabled: %v", id, enabled)
	
	// 如果要禁用激活的配置，先检查是否有其他可用配置
	if !enabled {
		log.Printf("Checking if OSS config ID %d can be disabled", id)
		var config database.OSSConfig
		if err := s.db.First(&config, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				log.Printf("OSS config not found with ID: %d", id)
				return fmt.Errorf("OSS config not found with id: %d", id)
			}
			log.Printf("Failed to retrieve OSS config with ID %d: %v", id, err)
			return fmt.Errorf("OSS config not found: %w", err)
		}
		log.Printf("Found OSS config: %s (Active: %v)", config.Name, config.IsActive)

		if config.IsActive {
			log.Printf("Cannot disable active OSS configuration: %s (ID: %d)", config.Name, id)
			return fmt.Errorf("cannot disable active OSS configuration")
		}
		log.Printf("OSS config ID %d can be disabled", id)
	}

	log.Printf("Updating enabled status for OSS config ID %d to: %v", id, enabled)
	if err := s.db.Model(&database.OSSConfig{}).Where("id = ?", id).
		Update("is_enabled", enabled).Error; err != nil {
		log.Printf("Failed to toggle OSS config ID %d: %v", id, err)
		return err
	}
	
	log.Printf("Successfully toggled OSS config ID %d to enabled: %v", id, enabled)
	return nil
}

// validateOSSConfig 验证OSS配置
// 验证OSS配置的所有必需字段和业务规则
// 参数:
//   - config: 要验证的OSS配置
// 返回:
//   - error: 验证失败时的错误信息
func (s *ossConfigService) validateOSSConfig(config *database.OSSConfig) error {
	log.Printf("Validating OSS config: %s", config.Name)
	
	if config.Name == "" {
		log.Printf("Validation failed: configuration name is required")
		return fmt.Errorf("configuration name is required")
	}

	if config.Provider == "" {
		log.Printf("Validation failed: OSS provider is required")
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
		log.Printf("Validation failed: unsupported OSS provider: %s", config.Provider)
		return fmt.Errorf("unsupported OSS provider: %s", config.Provider)
	}
	log.Printf("Provider validation passed: %s", config.Provider)

	if config.Region == "" {
		log.Printf("Validation failed: region is required")
		return fmt.Errorf("region is required")
	}

	if config.Bucket == "" {
		log.Printf("Validation failed: bucket name is required")
		return fmt.Errorf("bucket name is required")
	}

	if config.AccessKey == "" {
		log.Printf("Validation failed: access key is required")
		return fmt.Errorf("access key is required")
	}

	if config.SecretKey == "" {
		log.Printf("Validation failed: secret key is required")
		return fmt.Errorf("secret key is required")
	}

	// 检查配置名称是否重复
	log.Printf("Checking for duplicate configuration name: %s", config.Name)
	var count int64
	query := s.db.Model(&database.OSSConfig{}).Where("name = ?", config.Name)
	if config.ID > 0 {
		query = query.Where("id != ?", config.ID)
		log.Printf("Excluding current config ID %d from duplicate check", config.ID)
	}
	query.Count(&count)

	if count > 0 {
		log.Printf("Validation failed: configuration name already exists: %s", config.Name)
		return fmt.Errorf("configuration name already exists: %s", config.Name)
	}
	log.Printf("Configuration name uniqueness check passed: %s", config.Name)

	log.Printf("All validations passed for OSS config: %s", config.Name)
	return nil
}

// deactivateAllConfigs 取消所有配置的激活状态
// 将所有当前激活的OSS配置设置为非激活状态
// 返回:
//   - error: 操作过程中的错误信息
func (s *ossConfigService) deactivateAllConfigs() error {
	log.Printf("Deactivating all currently active OSS configurations")
	
	// 先查询当前激活的配置数量用于日志记录
	var count int64
	s.db.Model(&database.OSSConfig{}).Where("is_active = ?", true).Count(&count)
	log.Printf("Found %d active OSS configurations to deactivate", count)
	
	if err := s.db.Model(&database.OSSConfig{}).Where("is_active = ?", true).
		Update("is_active", false).Error; err != nil {
		log.Printf("Failed to deactivate OSS configurations: %v", err)
		return err
	}
	
	log.Printf("Successfully deactivated %d OSS configurations", count)
	return nil
}
