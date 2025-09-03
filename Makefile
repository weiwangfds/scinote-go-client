# Makefile for Scinote Go Client

# 变量定义
BINARY_NAME=scinote
BUILD_DIR=build
MAIN_FILE=main.go

# 默认目标
.PHONY: all
all: clean build

# 清理构建目录
.PHONY: clean
clean:
	@echo "Cleaning build directory..."
	@rm -rf $(BUILD_DIR)
	@rm -f $(BINARY_NAME)
	@rm -f *.db

# 安装依赖
.PHONY: deps
deps:
	@echo "Installing dependencies..."
	@go mod tidy
	@go mod download

# 构建应用
.PHONY: build
build: deps
	@echo "Building application..."
	@mkdir -p $(BUILD_DIR)
	@go build -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_FILE)
	@echo "Build completed: $(BUILD_DIR)/$(BINARY_NAME)"

# 运行应用
.PHONY: run
run: deps
	@echo "Running application..."
	@go run $(MAIN_FILE)

# 运行数据库示例
.PHONY: run-db-example
run-db-example: deps
	@echo "Running database example..."
	@go run examples/database_example.go

# 测试
.PHONY: test
test: deps
	@echo "Running tests..."
	@go test ./...

# 测试覆盖率
.PHONY: test-coverage
test-coverage: deps
	@echo "Running tests with coverage..."
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out

# 代码格式化
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	@go fmt ./...

# 代码检查
.PHONY: lint
lint: deps
	@echo "Running linter..."
	@go vet ./...

# 生成文档
.PHONY: docs
docs:
	@echo "Generating documentation..."
	@go doc -all ./...

# 数据库操作
.PHONY: db-clean
db-clean:
	@echo "Cleaning database files..."
	@rm -f *.db
	@rm -f *.sqlite
	@rm -f *.sqlite3

# 帮助信息
.PHONY: help
help:
	@echo "Available commands:"
	@echo "  all           - Clean and build the application"
	@echo "  build         - Build the application"
	@echo "  clean         - Clean build directory and database files"
	@echo "  deps          - Install dependencies"
	@echo "  run           - Run the application"
	@echo "  run-db-example- Run database example"
	@echo "  test          - Run tests"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  fmt           - Format code"
	@echo "  lint          - Run linter"
	@echo "  docs          - Generate documentation"
	@echo "  db-clean      - Clean database files"
	@echo "  help          - Show this help message"
