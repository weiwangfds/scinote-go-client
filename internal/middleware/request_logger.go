package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/weiwangfds/scinote/internal/logger"
)

// RequestLogEntry 请求日志条目结构
// 包含完整的请求生命周期信息和响应数据
type RequestLogEntry struct {
	// 基础信息
	Timestamp    string `json:"timestamp"`     // ISO 8601格式时间戳
	ReadableTime string `json:"readable_time"` // 可读时间格式
	TraceID      string `json:"trace_id"`      // 请求追踪ID

	// 请求信息
	Method     string                 `json:"method"`      // HTTP方法
	Path       string                 `json:"path"`        // 请求路径
	Query      map[string]interface{} `json:"query"`       // 查询参数
	Headers    map[string]string      `json:"headers"`     // 请求头
	Body       interface{}            `json:"body"`        // 请求体
	ClientIP   string                 `json:"client_ip"`   // 客户端IP
	UserAgent  string                 `json:"user_agent"`  // 用户代理

	// 响应信息
	StatusCode   int         `json:"status_code"`   // 响应状态码
	ResponseBody interface{} `json:"response_body"` // 响应体
	ResponseSize int         `json:"response_size"` // 响应大小（字节）

	// 时间信息
	StartTime    string `json:"start_time"`    // 请求开始时间
	EndTime      string `json:"end_time"`      // 请求结束时间
	Duration     string `json:"duration"`      // 请求持续时间
	DurationMs   int64  `json:"duration_ms"`   // 请求持续时间（毫秒）

	// 错误信息
	Error string `json:"error,omitempty"` // 错误信息（如果有）
}

// responseWriter 自定义响应写入器，用于捕获响应数据
type responseWriter struct {
	gin.ResponseWriter
	body   *bytes.Buffer // 响应体缓冲区
	size   int           // 响应大小
	status int           // 状态码
}

// Write 实现io.Writer接口，捕获响应数据
func (w *responseWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	w.size += len(b)
	return w.ResponseWriter.Write(b)
}

// WriteHeader 捕获状态码
func (w *responseWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// RequestLoggerConfig 请求日志中间件配置
type RequestLoggerConfig struct {
	Enabled         bool     `json:"enabled"`          // 是否启用
	Environment     string   `json:"environment"`      // 环境标识
	LogLevel        string   `json:"log_level"`        // 日志级别
	SkipPaths       []string `json:"skip_paths"`       // 跳过记录的路径
	MaxBodySize     int      `json:"max_body_size"`    // 最大请求体大小（字节）
	IncludeHeaders  bool     `json:"include_headers"`  // 是否包含请求头
	IncludeBody     bool     `json:"include_body"`     // 是否包含请求体
	IncludeResponse bool     `json:"include_response"` // 是否包含响应体
	AsyncLogging    bool     `json:"async_logging"`    // 是否异步记录
}

// DefaultRequestLoggerConfig 默认配置
func DefaultRequestLoggerConfig() *RequestLoggerConfig {
	return &RequestLoggerConfig{
		Enabled:         isDevEnvironment(),
		Environment:     getEnvironment(),
		LogLevel:        "info",
		SkipPaths:       []string{"/health", "/metrics", "/favicon.ico"},
		MaxBodySize:     1024 * 1024, // 1MB
		IncludeHeaders:  true,
		IncludeBody:     true,
		IncludeResponse: true,
		AsyncLogging:    true,
	}
}

// isDevEnvironment 检查是否为开发环境
func isDevEnvironment() bool {
	env := strings.ToLower(os.Getenv("GO_ENV"))
	if env == "" {
		env = strings.ToLower(os.Getenv("GIN_MODE"))
	}
	if env == "" {
		env = "development" // 默认为开发环境
	}
	return env == "development" || env == "dev" || env == "debug"
}

// getEnvironment 获取当前环境
func getEnvironment() string {
	env := os.Getenv("GO_ENV")
	if env == "" {
		env = os.Getenv("GIN_MODE")
	}
	if env == "" {
		env = "development"
	}
	return env
}

// RequestLogger 创建请求日志记录中间件
// 支持完整的请求生命周期跟踪和异步日志记录
func RequestLogger(config ...*RequestLoggerConfig) gin.HandlerFunc {
	// 使用默认配置或用户提供的配置
	var cfg *RequestLoggerConfig
	if len(config) > 0 && config[0] != nil {
		cfg = config[0]
	} else {
		cfg = DefaultRequestLoggerConfig()
	}

	// 如果未启用或非开发环境，返回空中间件
	if !cfg.Enabled {
		return gin.HandlerFunc(func(c *gin.Context) {
			c.Next()
		})
	}

	// 使用全局日志系统

	return gin.HandlerFunc(func(c *gin.Context) {
		// 检查是否跳过此路径
		for _, skipPath := range cfg.SkipPaths {
			if c.Request.URL.Path == skipPath {
				c.Next()
				return
			}
		}

		// 记录开始时间
		startTime := time.Now()
		traceID := generateTraceID()

		// 设置追踪ID到上下文
		c.Set("trace_id", traceID)

		// 创建自定义响应写入器
		writer := &responseWriter{
			ResponseWriter: c.Writer,
			body:           bytes.NewBuffer([]byte{}),
			size:           0,
			status:         200, // 默认状态码
		}
		c.Writer = writer

		// 读取请求体（如果需要）
		var requestBody interface{}
		if cfg.IncludeBody && c.Request.Body != nil {
			requestBody = readRequestBody(c, cfg.MaxBodySize)
		}

		// 处理请求
		c.Next()

		// 记录结束时间
		endTime := time.Now()
		duration := endTime.Sub(startTime)

		// 创建日志条目
		logEntry := &RequestLogEntry{
			Timestamp:    startTime.Format(time.RFC3339),
			ReadableTime: startTime.Format("2006-01-02 15:04:05"),
			TraceID:      traceID,
			Method:       c.Request.Method,
			Path:         c.Request.URL.Path,
			Query:        parseQueryParams(c.Request.URL.RawQuery),
			ClientIP:     c.ClientIP(),
			UserAgent:    c.Request.UserAgent(),
			StatusCode:   writer.status,
			ResponseSize: writer.size,
			StartTime:    startTime.Format(time.RFC3339),
			EndTime:      endTime.Format(time.RFC3339),
			Duration:     duration.String(),
			DurationMs:   duration.Nanoseconds() / 1000000,
		}

		// 添加请求头（如果需要）
		if cfg.IncludeHeaders {
			logEntry.Headers = extractHeaders(c.Request.Header)
		}

		// 添加请求体（如果需要）
		if cfg.IncludeBody {
			logEntry.Body = requestBody
		}

		// 添加响应体（如果需要）
		if cfg.IncludeResponse && writer.body.Len() > 0 {
			logEntry.ResponseBody = parseResponseBody(writer.body.Bytes())
		}

		// 添加错误信息（如果有）
		if len(c.Errors) > 0 {
			logEntry.Error = c.Errors.String()
		}

		// 记录日志
		if cfg.AsyncLogging {
			// 异步记录日志
			go func(entry *RequestLogEntry) {
				logRequestEntry(entry)
			}(logEntry)
		} else {
			// 同步记录日志
			logRequestEntry(logEntry)
		}
	})
}

// generateTraceID 生成追踪ID
func generateTraceID() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), time.Now().Nanosecond())
}

// readRequestBody 读取请求体
func readRequestBody(c *gin.Context, maxSize int) interface{} {
	if c.Request.Body == nil {
		return nil
	}

	// 读取请求体
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, int64(maxSize)))
	if err != nil {
		return map[string]string{"error": "failed to read request body"}
	}

	// 重置请求体，以便后续处理器可以读取
	c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

	// 尝试解析JSON
	if len(body) > 0 {
		var jsonBody interface{}
		if err := json.Unmarshal(body, &jsonBody); err == nil {
			return jsonBody
		}
		// 如果不是JSON，返回字符串
		return string(body)
	}

	return nil
}

// parseQueryParams 解析查询参数
func parseQueryParams(rawQuery string) map[string]interface{} {
	params := make(map[string]interface{})
	if rawQuery == "" {
		return params
	}

	pairs := strings.Split(rawQuery, "&")
	for _, pair := range pairs {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) == 2 {
			params[parts[0]] = parts[1]
		} else if len(parts) == 1 {
			params[parts[0]] = ""
		}
	}

	return params
}

// extractHeaders 提取请求头
func extractHeaders(headers map[string][]string) map[string]string {
	headerMap := make(map[string]string)
	for key, values := range headers {
		if len(values) > 0 {
			headerMap[key] = values[0] // 只取第一个值
		}
	}
	return headerMap
}

// parseResponseBody 解析响应体
func parseResponseBody(body []byte) interface{} {
	if len(body) == 0 {
		return nil
	}

	// 尝试解析JSON
	var jsonBody interface{}
	if err := json.Unmarshal(body, &jsonBody); err == nil {
		return jsonBody
	}

	// 如果不是JSON，返回字符串
	return string(body)
}

// logRequestEntry 记录请求日志条目
func logRequestEntry(entry *RequestLogEntry) {
	// 创建日志消息
	message := fmt.Sprintf("[REQUEST_LOG] %s %s - %d (%dms)",
		entry.Method, entry.Path, entry.StatusCode, entry.DurationMs)

	// 创建详细的日志数据
	logData := map[string]interface{}{
		"type":           "request_log",
		"timestamp":      entry.Timestamp,
		"readable_time":  entry.ReadableTime,
		"trace_id":       entry.TraceID,
		"method":         entry.Method,
		"path":           entry.Path,
		"query":          entry.Query,
		"client_ip":      entry.ClientIP,
		"user_agent":     entry.UserAgent,
		"status_code":    entry.StatusCode,
		"response_size":  entry.ResponseSize,
		"start_time":     entry.StartTime,
		"end_time":       entry.EndTime,
		"duration":       entry.Duration,
		"duration_ms":    entry.DurationMs,
	}

	// 添加可选字段
	if entry.Headers != nil {
		logData["headers"] = entry.Headers
	}
	if entry.Body != nil {
		logData["body"] = entry.Body
	}
	if entry.ResponseBody != nil {
		logData["response_body"] = entry.ResponseBody
	}
	if entry.Error != "" {
		logData["error"] = entry.Error
	}

	// 将日志数据转换为JSON字符串
	logJSON, err := json.Marshal(logData)
	if err != nil {
		logger.Errorf("Failed to marshal request log: %v", err)
		return
	}

	// 根据状态码确定日志级别并记录
	switch {
	case entry.StatusCode >= 500:
		logger.Errorf("%s | %s", message, string(logJSON))
	case entry.StatusCode >= 400:
		logger.Warnf("%s | %s", message, string(logJSON))
	default:
		logger.Infof("%s | %s", message, string(logJSON))
	}
}

// RequestLoggerWithCustomConfig 使用自定义配置创建请求日志中间件
func RequestLoggerWithCustomConfig(enabled bool, environment string, asyncLogging bool) gin.HandlerFunc {
	config := &RequestLoggerConfig{
		Enabled:         enabled,
		Environment:     environment,
		LogLevel:        "info",
		SkipPaths:       []string{"/health", "/metrics", "/favicon.ico"},
		MaxBodySize:     1024 * 1024,
		IncludeHeaders:  true,
		IncludeBody:     true,
		IncludeResponse: true,
		AsyncLogging:    asyncLogging,
	}
	return RequestLogger(config)
}