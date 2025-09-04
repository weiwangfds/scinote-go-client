package logger

import (
	"io"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// Logger 全局日志实例
var Logger *logrus.Logger

// Config 日志配置结构体
type Config struct {
	// Level 日志级别 (debug, info, warn, error, fatal, panic)
	Level string `toml:"level" json:"level"`
	// Format 日志格式 (json, text)
	Format string `toml:"format" json:"format"`
	// Output 输出方式 (console, file, both)
	Output string `toml:"output" json:"output"`
	// FilePath 日志文件路径
	FilePath string `toml:"file_path" json:"file_path"`
	// MaxSize 日志文件最大大小(MB)
	MaxSize int `toml:"max_size" json:"max_size"`
	// MaxAge 日志文件保留天数
	MaxAge int `toml:"max_age" json:"max_age"`
	// MaxBackups 最大备份文件数
	MaxBackups int `toml:"max_backups" json:"max_backups"`
	// Compress 是否压缩备份文件
	Compress bool `toml:"compress" json:"compress"`
}

// DefaultConfig 返回默认日志配置
func DefaultConfig() *Config {
	return &Config{
		Level:      "info",
		Format:     "text",
		Output:     "both",
		FilePath:   "logs/app.log",
		MaxSize:    100,
		MaxAge:     30,
		MaxBackups: 10,
		Compress:   true,
	}
}

// Init 初始化日志系统
// 参数:
//   - config: 日志配置，如果为nil则使用默认配置
// 返回值:
//   - error: 初始化错误
func Init(config *Config) error {
	if config == nil {
		config = DefaultConfig()
	}

	// 创建日志实例
	Logger = logrus.New()

	// 设置日志级别
	level, err := logrus.ParseLevel(config.Level)
	if err != nil {
		level = logrus.InfoLevel
		Logger.Warnf("无效的日志级别 '%s'，使用默认级别 'info'", config.Level)
	}
	Logger.SetLevel(level)

	// 设置日志格式
	switch config.Format {
	case "json":
		Logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02 15:04:05",
		})
	case "text":
		Logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02 15:04:05",
		})
	default:
		Logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02 15:04:05",
		})
		Logger.Warnf("无效的日志格式 '%s'，使用默认格式 'text'", config.Format)
	}

	// 设置输出
	if err := setupOutput(config); err != nil {
		return err
	}

	// 设置Gin的日志输出
	setupGinLogger()

	Logger.Info("日志系统初始化完成")
	return nil
}

// setupOutput 设置日志输出
func setupOutput(config *Config) error {
	switch config.Output {
	case "console":
		Logger.SetOutput(os.Stdout)
	case "file":
		return setupFileOutput(config)
	case "both":
		return setupBothOutput(config)
	default:
		Logger.SetOutput(os.Stdout)
		Logger.Warnf("无效的输出方式 '%s'，使用默认方式 'console'", config.Output)
	}
	return nil
}

// setupFileOutput 设置文件输出
func setupFileOutput(config *Config) error {
	// 创建日志目录
	logDir := filepath.Dir(config.FilePath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}

	// 打开日志文件
	logFile, err := os.OpenFile(config.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return err
	}

	Logger.SetOutput(logFile)
	return nil
}

// setupBothOutput 设置同时输出到控制台和文件
func setupBothOutput(config *Config) error {
	// 创建日志目录
	logDir := filepath.Dir(config.FilePath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}

	// 打开日志文件
	logFile, err := os.OpenFile(config.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return err
	}

	// 同时输出到控制台和文件
	multiWriter := io.MultiWriter(os.Stdout, logFile)
	Logger.SetOutput(multiWriter)
	return nil
}

// setupGinLogger 设置Gin的日志输出
func setupGinLogger() {
	// 创建自定义的Gin日志写入器
	ginWriter := &GinLogWriter{logger: Logger}
	gin.DefaultWriter = ginWriter
	gin.DefaultErrorWriter = ginWriter
}

// GinLogWriter Gin日志写入器
type GinLogWriter struct {
	logger *logrus.Logger
}

// Write 实现io.Writer接口
func (w *GinLogWriter) Write(p []byte) (n int, err error) {
	w.logger.Info(string(p))
	return len(p), nil
}

// GetLogger 获取日志实例
// 返回值:
//   - *logrus.Logger: 日志实例
func GetLogger() *logrus.Logger {
	if Logger == nil {
		// 如果日志未初始化，使用默认配置初始化
		if err := Init(nil); err != nil {
			// 如果初始化失败，返回标准日志
			logrus.Error("日志初始化失败，使用默认日志")
			return logrus.StandardLogger()
		}
	}
	return Logger
}

// Debug 记录调试级别日志
func Debug(args ...interface{}) {
	GetLogger().Debug(args...)
}

// Debugf 记录格式化调试级别日志
func Debugf(format string, args ...interface{}) {
	GetLogger().Debugf(format, args...)
}

// Info 记录信息级别日志
func Info(args ...interface{}) {
	GetLogger().Info(args...)
}

// Infof 记录格式化信息级别日志
func Infof(format string, args ...interface{}) {
	GetLogger().Infof(format, args...)
}

// Warn 记录警告级别日志
func Warn(args ...interface{}) {
	GetLogger().Warn(args...)
}

// Warnf 记录格式化警告级别日志
func Warnf(format string, args ...interface{}) {
	GetLogger().Warnf(format, args...)
}

// Error 记录错误级别日志
func Error(args ...interface{}) {
	GetLogger().Error(args...)
}

// Errorf 记录格式化错误级别日志
func Errorf(format string, args ...interface{}) {
	GetLogger().Errorf(format, args...)
}

// Fatal 记录致命级别日志并退出程序
func Fatal(args ...interface{}) {
	GetLogger().Fatal(args...)
}

// Fatalf 记录格式化致命级别日志并退出程序
func Fatalf(format string, args ...interface{}) {
	GetLogger().Fatalf(format, args...)
}

// Panic 记录恐慌级别日志并触发panic
func Panic(args ...interface{}) {
	GetLogger().Panic(args...)
}

// Panicf 记录格式化恐慌级别日志并触发panic
func Panicf(format string, args ...interface{}) {
	GetLogger().Panicf(format, args...)
}

// WithField 添加字段到日志条目
func WithField(key string, value interface{}) *logrus.Entry {
	return GetLogger().WithField(key, value)
}

// WithFields 添加多个字段到日志条目
func WithFields(fields logrus.Fields) *logrus.Entry {
	return GetLogger().WithFields(fields)
}