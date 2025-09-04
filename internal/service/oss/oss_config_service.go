// Package service 提供各种云存储服务的实现
// 本文件实现了OSS配置管理服务，负责OSS配置的增删改查和状态管理
package service

import (
	"errors"
	"fmt"

	"github.com/weiwangfds/scinote/internal/database"
	"github.com/weiwangfds/scinote/internal/logger"
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
//
// 返回:
//   - OSSConfigService: OSS配置服务接口实现
func NewOSSConfigService(db *gorm.DB) OSSConfigService {
	logger.Info("[OSS配置服务] 创建OSS配置服务实例")
	service := &ossConfigService{
		db:      db,
		factory: &OSSProviderFactory{},
	}
	logger.Info("[OSS配置服务] OSS配置服务实例创建成功")
	return service
}

// CreateOSSConfig 创建OSS配置
// 验证配置参数并保存到数据库，如果是第一个配置会自动激活
// 参数:
//   - config: 要创建的OSS配置信息
//
// 返回:
//   - error: 创建过程中的错误信息
func (s *ossConfigService) CreateOSSConfig(config *database.OSSConfig) error {
	logger.Infof("[OSS配置服务] 创建新的OSS配置: %s (提供商: %s, 区域: %s, 存储桶: %s)",
		config.Name, config.Provider, config.Region, config.Bucket)

	// 验证配置
	logger.Info("[OSS配置服务] 验证OSS配置: " + config.Name)
	if err := s.validateOSSConfig(config); err != nil {
		logger.Errorf("[OSS配置服务] OSS配置验证失败: %s, 错误: %v", config.Name, err)
		return err
	}
	logger.Info("[OSS配置服务] OSS配置验证通过: " + config.Name)

	// 如果这是第一个配置，自动设为激活状态
	var count int64
	s.db.Model(&database.OSSConfig{}).Count(&count)
	logger.Infof("[OSS配置服务] 当前OSS配置数量: %d", count)
	if count == 0 {
		config.IsActive = true
		logger.Infof("[OSS配置服务] 设置第一个OSS配置为激活状态: %s", config.Name)
	}

	// 如果设置为激活状态，需要先取消其他配置的激活状态
	if config.IsActive {
		logger.Infof("Deactivating other configs before activating: %s", config.Name)
		if err := s.deactivateAllConfigs(); err != nil {
			logger.Errorf("Failed to deactivate other configs: %v", err)
			return fmt.Errorf("failed to deactivate other configs: %w", err)
		}
		logger.Infof("Successfully deactivated other configs")
	}

	logger.Infof("Saving OSS config to database: %s", config.Name)
	if err := s.db.Create(config).Error; err != nil {
		logger.Errorf("Failed to create OSS config %s: %v", config.Name, err)
		return err
	}

	logger.Infof("Successfully created OSS config: %s (ID: %d, Active: %v)",
		config.Name, config.ID, config.IsActive)
	return nil
}

// GetOSSConfigByID 根据ID获取OSS配置
// 从数据库中查询指定ID的OSS配置信息
// 参数:
//   - id: OSS配置的唯一标识符
//
// 返回:
//   - *database.OSSConfig: 查询到的OSS配置信息
//   - error: 查询过程中的错误信息
func (s *ossConfigService) GetOSSConfigByID(id uint) (*database.OSSConfig, error) {
	logger.Infof("[OSS配置服务] 根据ID获取OSS配置: %d", id)

	var config database.OSSConfig
	if err := s.db.First(&config, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Info("[OSS配置服务] 未找到ID为" + fmt.Sprint(id) + "的OSS配置")
			return nil, fmt.Errorf("OSS配置未找到，ID: %d", id)
		}
		logger.Errorf("[OSS配置服务] 获取ID为%d的OSS配置失败: %v", id, err)
		return nil, err
	}

	logger.Infof("[OSS配置服务] 成功获取OSS配置: %s (ID: %d, 提供商: %s, 激活状态: %v)",
		config.Name, config.ID, config.Provider, config.IsActive)
	return &config, nil
}

// ListOSSConfigs 获取所有OSS配置
// 返回数据库中所有的OSS配置列表，按创建时间倒序排列
// 返回:
//   - []database.OSSConfig: OSS配置列表
//   - error: 查询过程中的错误信息
func (s *ossConfigService) ListOSSConfigs() ([]database.OSSConfig, error) {
	logger.Info("[OSS配置服务] 获取所有OSS配置")

	var configs []database.OSSConfig
	if err := s.db.Order("created_at DESC").Find(&configs).Error; err != nil {
		logger.Errorf("[OSS配置服务] 获取OSS配置列表失败: %v", err)
		return nil, err
	}

	logger.Infof("[OSS配置服务] 成功获取%d个OSS配置", len(configs))
	for i, config := range configs {
		logger.Infof("[OSS配置服务] 配置 %d: %s (ID: %d, 提供商: %s, 激活状态: %v, 启用状态: %v)",
			i+1, config.Name, config.ID, config.Provider, config.IsActive, config.IsEnabled)
	}
	return configs, nil
}

// UpdateOSSConfig 更新OSS配置
// 验证并更新指定的OSS配置，处理激活状态变更
// 参数:
//   - config: 要更新的OSS配置信息（包含ID）
//
// 返回:
//   - error: 更新过程中的错误信息
func (s *ossConfigService) UpdateOSSConfig(config *database.OSSConfig) error {
	logger.Infof("[OSS配置服务] 更新OSS配置 ID: %d 名称: %s (提供商: %s, 区域: %s, 存储桶: %s)",
		config.ID, config.Name, config.Provider, config.Region, config.Bucket)

	// 验证配置
	logger.Info("[OSS配置服务] 验证更新的OSS配置: " + config.Name)
	if err := s.validateOSSConfig(config); err != nil {
		logger.Errorf("[OSS配置服务] OSS配置验证失败: %s, 错误: %v", config.Name, err)
		return err
	}
	logger.Info("[OSS配置服务] OSS配置验证通过: " + config.Name)

	// 获取原有配置
	logger.Infof("[OSS配置服务] 获取现有OSS配置 ID: %d", config.ID)
	var existingConfig database.OSSConfig
	if err := s.db.First(&existingConfig, config.ID).Error; err != nil {
		logger.Errorf("[OSS配置服务] 未找到ID为%d的OSS配置: %v", config.ID, err)
		return fmt.Errorf("OSS配置未找到: %w", err)
	}
	logger.Infof("[OSS配置服务] 找到现有OSS配置: %s (激活状态: %v)", existingConfig.Name, existingConfig.IsActive)

	// 如果要激活此配置，需要先取消其他配置的激活状态
	if config.IsActive && !existingConfig.IsActive {
		logger.Infof("[OSS配置服务] 在激活更新的配置前取消其他配置激活状态: %s", config.Name)
		if err := s.deactivateAllConfigs(); err != nil {
			logger.Errorf("[OSS配置服务] 取消其他配置激活状态失败: %v", err)
			return fmt.Errorf("取消其他配置激活状态失败: %w", err)
		}
		logger.Info("[OSS配置服务] 成功取消其他配置的激活状态")
	}

	logger.Infof("Saving updated OSS config to database: %s", config.Name)
	if err := s.db.Save(config).Error; err != nil {
		logger.Errorf("Failed to update OSS config %s (ID: %d): %v", config.Name, config.ID, err)
		return err
	}

	logger.Infof("Successfully updated OSS config: %s (ID: %d, Active: %v)",
		config.Name, config.ID, config.IsActive)
	return nil
}

// DeleteOSSConfig 删除OSS配置
// 删除指定ID的OSS配置，不允许删除激活状态的配置
// 参数:
//   - id: 要删除的OSS配置ID
//
// 返回:
//   - error: 删除过程中的错误信息
func (s *ossConfigService) DeleteOSSConfig(id uint) error {
	logger.Infof("[OSS配置服务] 删除OSS配置 ID: %d", id)

	// 检查是否为激活配置
	logger.Infof("[OSS配置服务] 删除前检查OSS配置 ID %d 是否为激活状态", id)
	var config database.OSSConfig
	if err := s.db.First(&config, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Info("[OSS配置服务] 未找到ID为" + fmt.Sprint(id) + "的OSS配置")
			return fmt.Errorf("OSS配置未找到，ID: %d", id)
		}
		logger.Errorf("[OSS配置服务] 获取ID为%d的OSS配置失败: %v", id, err)
		return fmt.Errorf("OSS配置未找到: %w", err)
	}
	logger.Infof("[OSS配置服务] 找到要删除的OSS配置: %s (提供商: %s, 激活状态: %v)",
		config.Name, config.Provider, config.IsActive)

	if config.IsActive {
		logger.Info("[OSS配置服务] 不能删除激活状态的OSS配置: " + config.Name + " (ID: " + fmt.Sprint(id) + ")")
		return fmt.Errorf("不能删除激活状态的OSS配置")
	}

	logger.Infof("[OSS配置服务] 从数据库删除OSS配置: %s (ID: %d)", config.Name, id)
	if err := s.db.Delete(&database.OSSConfig{}, id).Error; err != nil {
		logger.Errorf("[OSS配置服务] 删除OSS配置 ID %d 失败: %v", id, err)
		return err
	}

	logger.Infof("[OSS配置服务] 成功删除OSS配置: %s (ID: %d)", config.Name, id)
	return nil
}

// ActivateOSSConfig 激活OSS配置
// 激活指定配置并取消其他配置的激活状态，确保只有一个激活配置
// 参数:
//   - id: 要激活的OSS配置ID
//
// 返回:
//   - error: 激活过程中的错误信息
func (s *ossConfigService) ActivateOSSConfig(id uint) error {
	logger.Infof("[OSS配置服务] 激活OSS配置 ID: %d", id)

	// 先获取配置信息用于日志记录
	var config database.OSSConfig
	if err := s.db.First(&config, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Info("[OSS配置服务] 未找到ID为" + fmt.Sprint(id) + "的OSS配置")
			return fmt.Errorf("OSS配置未找到，ID: %d", id)
		}
		logger.Errorf("[OSS配置服务] 获取ID为%d的OSS配置失败: %v", id, err)
		return fmt.Errorf("OSS配置未找到: %w", err)
	}
	logger.Infof("[OSS配置服务] 找到要激活的OSS配置: %s (提供商: %s)", config.Name, config.Provider)

	// 先取消所有配置的激活状态
	logger.Infof("[OSS配置服务] 激活前取消所有其他OSS配置激活状态，ID: %d", id)
	if err := s.deactivateAllConfigs(); err != nil {
		logger.Errorf("[OSS配置服务] 取消其他配置激活状态失败: %v", err)
		return fmt.Errorf("取消其他配置激活状态失败: %w", err)
	}
	logger.Info("[OSS配置服务] 成功取消所有其他配置的激活状态")

	// 激活指定配置
	logger.Infof("[OSS配置服务] 设置OSS配置 ID %d 为激活状态", id)
	if err := s.db.Model(&database.OSSConfig{}).Where("id = ?", id).
		Update("is_active", true).Error; err != nil {
		logger.Errorf("[OSS配置服务] 激活OSS配置 ID %d 失败: %v", id, err)
		return fmt.Errorf("激活OSS配置失败: %w", err)
	}

	logger.Infof("[OSS配置服务] 成功激活OSS配置: %s (ID: %d)", config.Name, id)
	return nil
}

// TestOSSConfig 测试OSS配置连接
// 使用指定配置创建OSS提供商并测试连接是否正常
// 参数:
//   - id: 要测试的OSS配置ID
//
// 返回:
//   - error: 测试过程中的错误信息
func (s *ossConfigService) TestOSSConfig(id uint) error {
	logger.Infof("[OSS配置服务] 测试OSS配置连接 ID: %d", id)

	config, err := s.GetOSSConfigByID(id)
	if err != nil {
		logger.Errorf("[OSS配置服务] 获取测试用OSS配置失败 (ID: %d): %v", id, err)
		return err
	}
	logger.Infof("[OSS配置服务] 获取测试用OSS配置: %s (提供商: %s, 区域: %s, 存储桶: %s)",
		config.Name, config.Provider, config.Region, config.Bucket)

	logger.Infof("[OSS配置服务] 创建测试用OSS提供商: %s", config.Provider)
	provider, err := s.factory.CreateProvider(config)
	if err != nil {
		logger.Errorf("[OSS配置服务] 为%s创建OSS提供商失败: %v", config.Name, err)
		return fmt.Errorf("创建OSS提供商失败: %w", err)
	}
	logger.Infof("[OSS配置服务] 成功创建OSS提供商: %s", config.Name)

	logger.Info("[OSS配置服务] 测试OSS配置连接: " + config.Name)
	if err := provider.TestConnection(); err != nil {
		logger.Errorf("[OSS配置服务] OSS配置%s连接测试失败: %v", config.Name, err)
		return err
	}

	logger.Infof("[OSS配置服务] OSS配置连接测试成功: %s (ID: %d)", config.Name, id)
	return nil
}

// GetActiveOSSConfig 获取当前激活的OSS配置
// 返回当前激活且启用的OSS配置
// 当没有配置OSS时，这是正常情况，不会影响系统其他功能
// 返回:
//   - *database.OSSConfig: 当前激活的OSS配置信息
//   - error: 查询过程中的错误信息
func (s *ossConfigService) GetActiveOSSConfig() (*database.OSSConfig, error) {
	logger.Info("[OSS配置服务] 获取当前激活的OSS配置")

	var config database.OSSConfig
	if err := s.db.Where("is_active = ? AND is_enabled = ?", true, true).First(&config).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 没有OSS配置是正常情况，不记录为错误
			logger.Info("[OSS配置服务] 未找到激活的OSS配置 (如未配置OSS，这是正常情况)")
			return nil, fmt.Errorf("未找到激活的OSS配置")
		}
		logger.Errorf("[OSS配置服务] 获取激活的OSS配置失败: %v", err)
		return nil, err
	}

	logger.Infof("[OSS配置服务] 找到激活的OSS配置: %s (ID: %d, 提供商: %s, 区域: %s, 存储桶: %s)",
		config.Name, config.ID, config.Provider, config.Region, config.Bucket)
	return &config, nil
}

// ToggleOSSConfig 启用/禁用OSS配置
// 切换指定配置的启用状态，不允许禁用激活状态的配置
// 参数:
//   - id: 要切换状态的OSS配置ID
//   - enabled: 新的启用状态
//
// 返回:
//   - error: 操作过程中的错误信息
func (s *ossConfigService) ToggleOSSConfig(id uint, enabled bool) error {
	logger.Infof("[OSS配置服务] 切换OSS配置 ID %d 启用状态: %v", id, enabled)

	// 如果要禁用激活的配置，先检查是否有其他可用配置
	if !enabled {
		logger.Infof("[OSS配置服务] 检查OSS配置 ID %d 是否可以禁用", id)
		var config database.OSSConfig
		if err := s.db.First(&config, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				logger.Info("[OSS配置服务] 未找到ID为" + fmt.Sprint(id) + "的OSS配置")
				return fmt.Errorf("OSS配置未找到，ID: %d", id)
			}
			logger.Errorf("[OSS配置服务] 获取ID为%d的OSS配置失败: %v", id, err)
			return fmt.Errorf("OSS配置未找到: %w", err)
		}
		logger.Infof("[OSS配置服务] 找到OSS配置: %s (激活状态: %v)", config.Name, config.IsActive)

		if config.IsActive {
			logger.Info("[OSS配置服务] 不能禁用激活状态的OSS配置: " + config.Name + " (ID: " + fmt.Sprint(id) + ")")
			return fmt.Errorf("不能禁用激活状态的OSS配置")
		}
		logger.Infof("[OSS配置服务] OSS配置 ID %d 可以禁用", id)
	}

	logger.Infof("[OSS配置服务] 更新OSS配置 ID %d 的启用状态为: %v", id, enabled)
	if err := s.db.Model(&database.OSSConfig{}).Where("id = ?", id).
		Update("is_enabled", enabled).Error; err != nil {
		logger.Errorf("[OSS配置服务] 切换OSS配置 ID %d 失败: %v", id, err)
		return err
	}

	logger.Infof("[OSS配置服务] 成功切换OSS配置 ID %d 的启用状态为: %v", id, enabled)
	return nil
}

// validateOSSConfig 验证OSS配置
// 验证OSS配置的所有必需字段和业务规则
// 参数:
//   - config: 要验证的OSS配置
//
// 返回:
//   - error: 验证失败时的错误信息
func (s *ossConfigService) validateOSSConfig(config *database.OSSConfig) error {
	logger.Info("[OSS配置服务] 验证OSS配置: " + config.Name)

	if config.Name == "" {
		logger.Info("[OSS配置服务] 验证失败: 配置名称不能为空")
		return fmt.Errorf("配置名称不能为空")
	}

	if config.Provider == "" {
		logger.Info("[OSS配置服务] 验证失败: OSS提供商不能为空")
		return fmt.Errorf("OSS提供商不能为空")
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
		logger.Infof("[OSS配置服务] 验证失败: 不支持的OSS提供商: %s", config.Provider)
		return fmt.Errorf("不支持的OSS提供商: %s", config.Provider)
	}
	logger.Infof("[OSS配置服务] 提供商验证通过: %s", config.Provider)

	if config.Region == "" {
		logger.Info("[OSS配置服务] 验证失败: 区域不能为空")
		return fmt.Errorf("区域不能为空")
	}

	if config.Bucket == "" {
		logger.Info("[OSS配置服务] 验证失败: 存储桶名称不能为空")
		return fmt.Errorf("存储桶名称不能为空")
	}

	if config.AccessKey == "" {
		logger.Info("[OSS配置服务] 验证失败: 访问密钥不能为空")
		return fmt.Errorf("访问密钥不能为空")
	}

	if config.SecretKey == "" {
		logger.Info("[OSS配置服务] 验证失败: 密钥不能为空")
		return fmt.Errorf("密钥不能为空")
	}

	// 检查配置名称是否重复
	logger.Infof("[OSS配置服务] 检查配置名称是否重复: %s", config.Name)
	var count int64
	query := s.db.Model(&database.OSSConfig{}).Where("name = ?", config.Name)
	if config.ID > 0 {
		query = query.Where("id != ?", config.ID)
		logger.Infof("[OSS配置服务] 重复检查时排除当前配置 ID %d", config.ID)
	}
	query.Count(&count)

	if count > 0 {
		logger.Infof("[OSS配置服务] 验证失败: 配置名称已存在: %s", config.Name)
		return fmt.Errorf("配置名称已存在: %s", config.Name)
	}
	logger.Infof("[OSS配置服务] 配置名称唯一性检查通过: %s", config.Name)

	logger.Info("[OSS配置服务] OSS配置所有验证通过: " + config.Name)
	return nil
}

// deactivateAllConfigs 取消所有配置的激活状态
// 将所有当前激活的OSS配置设置为非激活状态
// 返回:
//   - error: 操作过程中的错误信息
func (s *ossConfigService) deactivateAllConfigs() error {
	logger.Info("[OSS配置服务] 取消所有当前激活的OSS配置")

	// 先查询当前激活的配置数量用于日志记录
	var count int64
	s.db.Model(&database.OSSConfig{}).Where("is_active = ?", true).Count(&count)
	logger.Infof("[OSS配置服务] 找到%d个激活的OSS配置需要取消激活", count)

	if err := s.db.Model(&database.OSSConfig{}).Where("is_active = ?", true).
		Update("is_active", false).Error; err != nil {
		logger.Errorf("[OSS配置服务] 取消OSS配置激活状态失败: %v", err)
		return err
	}

	logger.Infof("[OSS配置服务] 成功取消%d个OSS配置的激活状态", count)
	return nil
}
