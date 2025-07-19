# Scintirete Makefile

.PHONY: all build test clean proto-gen lint format vet help server cli deps
.DEFAULT_GOAL := help

# 基本配置
GO := go
GOFMT := gofmt
GOLINT := golint
GOVET := go vet
PROTOC := protoc

# 项目路径
PROJECT_ROOT := $(shell pwd)
API_DIR := api/proto
GEN_DIR := gen/go
BIN_DIR := bin

# 生成的protobuf文件
PROTO_FILES := $(shell find $(API_DIR) -name "*.proto")
PROTO_GO_FILES := $(patsubst $(API_DIR)/%.proto,$(GEN_DIR)/%.pb.go,$(PROTO_FILES))

# 二进制文件名
SERVER_BINARY := scintirete-server
CLI_BINARY := scintirete-cli

help: ## 显示帮助信息
	@echo "Scintirete 构建系统"
	@echo ""
	@echo "可用命令:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

all: deps proto-gen build ## 完整构建流程

deps: ## 安装依赖
	@echo "Installing dependencies..."
	$(GO) mod download
	$(GO) mod tidy

proto-gen: ## 生成protobuf代码
	@echo "Generating protobuf code..."
	@mkdir -p $(GEN_DIR)
	@if ! command -v protoc > /dev/null; then \
		echo "Error: protoc not found. Please install Protocol Buffers compiler."; \
		exit 1; \
	fi
	@if ! $(GO) list -m google.golang.org/protobuf > /dev/null 2>&1; then \
		echo "Installing protobuf dependencies..."; \
		$(GO) get google.golang.org/protobuf/cmd/protoc-gen-go; \
		$(GO) get google.golang.org/grpc/cmd/protoc-gen-go-grpc; \
	fi
	$(PROTOC) \
		--proto_path=$(API_DIR) \
		--go_out=$(GEN_DIR) \
		--go_opt=paths=source_relative \
		--go-grpc_out=$(GEN_DIR) \
		--go-grpc_opt=paths=source_relative \
		$(PROTO_FILES)

build: proto-gen ## 构建所有二进制文件
	@echo "Building binaries..."
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN_DIR)/$(SERVER_BINARY) ./cmd/scintirete-server
	$(GO) build -o $(BIN_DIR)/$(CLI_BINARY) ./cmd/scintirete-cli

server: proto-gen ## 只构建服务端
	@echo "Building server..."
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN_DIR)/$(SERVER_BINARY) ./cmd/scintirete-server

cli: proto-gen ## 只构建客户端
	@echo "Building CLI..."
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN_DIR)/$(CLI_BINARY) ./cmd/scintirete-cli

test: ## 运行测试
	@echo "Running tests..."
	$(GO) test -v -race -cover ./...

test-coverage: ## 运行测试并生成覆盖率报告
	@echo "Running tests with coverage..."
	$(GO) test -v -race -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

benchmark: ## 运行性能测试
	@echo "Running benchmarks..."
	$(GO) test -bench=. -benchmem ./...

lint: ## 运行代码检查
	@echo "Running linter..."
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not found, using basic checks..."; \
		$(GOLINT) ./...; \
	fi

format: ## 格式化代码
	@echo "Formatting code..."
	$(GOFMT) -s -w .

vet: ## 运行go vet检查
	@echo "Running go vet..."
	$(GOVET) ./...

check: format vet lint test ## 运行所有检查

clean: ## 清理构建文件
	@echo "Cleaning up..."
	rm -rf $(BIN_DIR)
	rm -rf $(GEN_DIR)
	rm -f coverage.out coverage.html

install: build ## 安装到GOPATH/bin
	@echo "Installing binaries..."
	$(GO) install ./cmd/scintirete-server
	$(GO) install ./cmd/scintirete-cli

docker-build: ## 构建Docker镜像
	@echo "Building Docker image..."
	docker build -t scintirete:latest .

docker-run: ## 运行Docker容器
	@echo "Running Docker container..."
	docker run -p 8080:8080 -p 9090:9090 -v $(PWD)/data:/app/data scintirete:latest

tools: ## 安装开发工具
	@echo "Installing development tools..."
	$(GO) install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	$(GO) install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

.PHONY: run-server run-cli
run-server: server ## 运行服务端 (开发模式)
	@echo "Starting Scintirete server..."
	./$(BIN_DIR)/$(SERVER_BINARY) --config configs/scintirete.toml

run-cli: cli ## 运行CLI客户端示例
	@echo "Running CLI example..."
	./$(BIN_DIR)/$(CLI_BINARY) --help 