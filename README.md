# Scinote Go Client

一个基于Go语言和Gin框架的Web服务框架，提供基础的路由配置、中间件集成、HTTP服务和SQLite数据库支持。

## 功能特性

- 轻量级Web服务框架
- RESTful API设计
- 中间件支持（日志、CORS）
- SQLite数据库支持（使用GORM）
- 配置文件支持
- 优雅关闭
- 健康检查接口
- 数据库状态检查

## 技术栈

- Go 1.21+
- Gin Web框架
- GORM ORM框架
- SQLite数据库
- Viper配置管理
- Logrus日志库

## 项目结构

```
scinote-go-client/
├── config.toml              # 配置文件
├── go.mod                   # Go模块文件
├── main.go                  # 主程序入口
├── README.md                # 项目说明
├── Makefile                 # 构建脚本
├── Dockerfile               # Docker构建文件
├── docker-compose.yml       # Docker Compose配置
├── .gitignore               # Git忽略文件
├── config/                  # 配置包
│   └── config.go
└── internal/                # 内部包
    ├── database/           # 数据库相关
    │   └── database.go
    ├── middleware/         # 中间件
    │   └── logger_middleware.go
    └── router/             # 路由配置
        └── router.go
```

## 快速开始

### 1. 安装依赖

```bash
go mod tidy
```

### 2. 运行服务

```bash
go run main.go
```

服务将在 `http://localhost:8080` 启动

### 3. 健康检查

```bash
curl http://localhost:8080/health
```

### 4. 服务信息

```bash
curl http://localhost:8080/api/v1/info
```

### 5. 数据库状态检查

```bash
curl http://localhost:8080/api/v1/db/status
```

## API接口

### 基础接口

- `GET /health` - 健康检查
- `GET /api/v1/info` - 服务信息
- `GET /api/v1/db/status` - 数据库状态检查

## 配置说明

配置文件 `config.toml` 包含以下配置项：

- 服务器配置（端口、超时等）
- 数据库配置（驱动、连接字符串等）
- 日志配置（级别、格式等）
- CORS配置（跨域设置）

## 开发说明

### 添加新的API接口

1. 在 `internal/router` 中添加新的路由
2. 根据需要添加相应的处理器和中间件

### 添加新的中间件

1. 在 `internal/middleware` 中实现中间件
2. 在 `internal/router` 中注册中间件

### 数据库操作

项目使用GORM作为ORM框架，支持SQLite数据库。可以通过路由的`GetDB()`方法获取数据库连接进行数据操作。

## 部署

### 使用Makefile

```bash
# 构建应用
make build

# 运行应用
make run

# 查看帮助
make help
```

### 使用Docker

```bash
# 构建并运行
docker-compose up --build

# 后台运行
docker-compose up -d
```

### 手动构建

```bash
# 构建二进制文件
go build -o scinote main.go

# 运行
./scinote
```

## 许可证

MIT License
