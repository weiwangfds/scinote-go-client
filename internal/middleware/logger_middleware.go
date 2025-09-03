package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// LoggerMiddleware 日志中间件
type LoggerMiddleware struct {
	logger *logrus.Logger
}

// NewLoggerMiddleware 创建日志中间件实例
func NewLoggerMiddleware() *LoggerMiddleware {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	return &LoggerMiddleware{
		logger: logger,
	}
}

// Logger 日志中间件
func (m *LoggerMiddleware) Logger() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		// 记录请求信息
		m.logger.WithFields(logrus.Fields{
			"timestamp": param.TimeStamp.Format(time.RFC3339),
			"status":    param.StatusCode,
			"latency":   param.Latency,
			"client_ip": param.ClientIP,
			"method":    param.Method,
			"path":      param.Path,
			"error":     param.ErrorMessage,
		}).Info("HTTP Request")

		return ""
	})
}

// RequestLogger 请求日志中间件
func (m *LoggerMiddleware) RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// 处理请求
		c.Next()

		// 记录响应信息
		latency := time.Since(start)
		status := c.Writer.Status()
		errorMessage := c.Errors.String()

		m.logger.WithFields(logrus.Fields{
			"timestamp":     time.Now().Format(time.RFC3339),
			"status":        status,
			"latency":       latency,
			"client_ip":     c.ClientIP(),
			"method":        c.Request.Method,
			"path":          path,
			"raw_query":     raw,
			"user_agent":    c.Request.UserAgent(),
			"error_message": errorMessage,
		}).Info("HTTP Response")
	}
}
