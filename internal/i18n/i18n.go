// Package i18n 提供国际化支持
// 负责管理应用程序的语言包和翻译功能
package i18n

import (
	"sync"

	"github.com/go-playground/locales/zh"
	"github.com/go-playground/locales/en_US"
	ut "github.com/go-playground/universal-translator"
	"github.com/weiwangfds/scinote/internal/logger"
)

// 支持的语言
const (
	LangZhCN = "zh-CN"
	LangEnUS = "en-US"
)

var (
	instance *I18n
	once     sync.Once

	// 语言包存储
	translations = map[string]map[string]string{
		LangZhCN: {
			"success":            "成功",
			"internal_server_error": "服务器内部错误",
			"invalid_params":      "参数错误",
			"unauthorized":       "未授权",
			"forbidden":          "禁止访问",
			"not_found":           "资源未找到",
			"method_not_allowed":   "方法不允许",
			"too_many_requests":    "请求过于频繁",
			"service_unavailable": "服务不可用",

			"file_not_found":       "文件未找到",
			"file_already_exists":  "文件已存在",
			"file_upload_failed":   "文件上传失败",
			"file_delete_failed":   "文件删除失败",
			"file_read_failed":     "文件读取失败",
			"file_write_failed":    "文件写入失败",
			"file_size_too_large":   "文件大小超限",
			"file_type_not_allowed": "文件类型不允许",
			"file_corrupted":      "文件损坏",
			"file_hash_mismatch":   "文件哈希不匹配",

			"oss_config_not_found":       "OSS配置未找到",
			"oss_config_invalid":        "OSS配置无效",
			"oss_connection_failed":     "OSS连接失败",
			"oss_upload_failed":         "OSS上传失败",
			"oss_download_failed":       "OSS下载失败",
			"oss_delete_failed":         "OSS删除失败",
			"oss_list_failed":           "OSS列表获取失败",
			"oss_sync_failed":           "OSS同步失败",
			"oss_provider_not_supported": "OSS提供商不支持",

			"database_connection":  "数据库连接错误",
			"database_query":       "数据库查询错误",
			"database_insert":      "数据库插入错误",
			"database_update":      "数据库更新错误",
			"database_delete":      "数据库删除错误",
			"database_transaction": "数据库事务错误",
			"record_not_found":      "记录未找到",
			"record_already_exists": "记录已存在",

			"unknown_error": "未知错误",
		},
		LangEnUS: {
			"success":            "Success",
			"internal_server_error": "Internal Server Error",
			"invalid_params":      "Invalid Parameters",
			"unauthorized":       "Unauthorized",
			"forbidden":          "Forbidden",
			"not_found":           "Resource Not Found",
			"method_not_allowed":   "Method Not Allowed",
			"too_many_requests":    "Too Many Requests",
			"service_unavailable": "Service Unavailable",

			"file_not_found":       "File Not Found",
			"file_already_exists":  "File Already Exists",
			"file_upload_failed":   "File Upload Failed",
			"file_delete_failed":   "File Delete Failed",
			"file_read_failed":     "File Read Failed",
			"file_write_failed":    "File Write Failed",
			"file_size_too_large":   "File Size Too Large",
			"file_type_not_allowed": "File Type Not Allowed",
			"file_corrupted":      "File Corrupted",
			"file_hash_mismatch":   "File Hash Mismatch",

			"oss_config_not_found":       "OSS Config Not Found",
			"oss_config_invalid":        "OSS Config Invalid",
			"oss_connection_failed":     "OSS Connection Failed",
			"oss_upload_failed":         "OSS Upload Failed",
			"oss_download_failed":       "OSS Download Failed",
			"oss_delete_failed":         "OSS Delete Failed",
			"oss_list_failed":           "OSS List Failed",
			"oss_sync_failed":           "OSS Sync Failed",
			"oss_provider_not_supported": "OSS Provider Not Supported",

			"database_connection":  "Database Connection Error",
			"database_query":       "Database Query Error",
			"database_insert":      "Database Insert Error",
			"database_update":      "Database Update Error",
			"database_delete":      "Database Delete Error",
			"database_transaction": "Database Transaction Error",
			"record_not_found":      "Record Not Found",
			"record_already_exists": "Record Already Exists",

			"unknown_error": "Unknown Error",
		},
	}
)

// I18n 国际化管理器
type I18n struct {
	translators map[string]ut.Translator
	defaultLang string
}

// GetInstance 获取I18n单例
func GetInstance() *I18n {
	once.Do(func() {
		instance = &I18n{
			translators: make(map[string]ut.Translator),
			defaultLang: LangZhCN,
		}
		instance.initTranslators()
	})
	return instance
}

// initTranslators 初始化翻译器
func (i *I18n) initTranslators() {
	// 创建通用翻译器
	zhCN := zh.New()
	enUS := en_US.New()
	uni := ut.New(zhCN, enUS, zhCN)

	// 注册支持的语言 - 使用locale库的标识符
	langMappings := map[string]string{
		LangZhCN: "zh",     // 中文使用 "zh"
		LangEnUS: "en_US",  // 英文使用 "en_US"
	}
	
	for ourLang, localeLang := range langMappings {
		trans, found := uni.GetTranslator(localeLang)
		if !found {
			logger.Errorf("初始化翻译器失败 for language %s (locale: %s): translator not found", ourLang, localeLang)
			continue
		}
		i.translators[ourLang] = trans
		logger.Infof("成功初始化翻译器: %s -> %s", ourLang, localeLang)
	}

	logger.Info("国际化翻译器初始化完成")
}

// Translate 根据键和语言获取翻译
func (i *I18n) Translate(key, lang string) string {
	// 检查语言是否支持，否则使用默认语言
	_, exists := i.translators[lang]
	if !exists {
		_, exists := i.translators[i.defaultLang]
		if !exists {
			logger.Warnf("未找到翻译器，使用默认文本: %s", key)
			return key
		}
	}

	// 查找翻译
	if translation, found := translations[lang][key]; found {
		return translation
	}

	// 如果当前语言没有找到，尝试在默认语言中查找
	if lang != i.defaultLang {
		if translation, found := translations[i.defaultLang][key]; found {
			return translation
		}
	}

	logger.Warnf("未找到翻译: %s, 语言: %s", key, lang)
	return key
}

// SetDefaultLanguage 设置默认语言
func (i *I18n) SetDefaultLanguage(lang string) {
	i.defaultLang = lang
	logger.Infof("设置默认语言为: %s", lang)
}

// GetDefaultLanguage 获取默认语言
func (i *I18n) GetDefaultLanguage() string {
	return i.defaultLang
}

// IsSupportedLanguage 检查语言是否支持
func (i *I18n) IsSupportedLanguage(lang string) bool {
	_, exists := i.translators[lang]
	return exists
}

// GetSupportedLanguages 获取支持的语言列表
func (i *I18n) GetSupportedLanguages() []string {
	langs := make([]string, 0, len(i.translators))
	for lang := range i.translators {
		langs = append(langs, lang)
	}
	return langs
}