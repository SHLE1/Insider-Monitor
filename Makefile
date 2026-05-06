.PHONY: build run test clean setup dev dev-api dev-web build-web

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
BINARY_NAME=insider-monitor
BINARY_UNIX=$(BINARY_NAME)_unix

# Build directory
BUILD_DIR=bin

all: test build

build: build-web
	mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME) -v ./cmd/monitor

build-web:
	cd web && pnpm install --frozen-lockfile && pnpm build

clean: 
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -rf data/*.log
	rm -rf internal/configui/dist

run:
	$(GOCMD) run cmd/monitor/main.go

test: 
	$(GOTEST) -v ./...

# dev: 后端用 air 热重载，前端用 pnpm dev（需分两个终端）
dev-api:
	INSIDER_DEV=1 air

dev-web:
	cd web && pnpm dev

dev:
	@echo "请在两个终端分别运行:"
	@echo "  make dev-api   # 启动后端 (air 热重载, :8081)"
	@echo "  make dev-web   # 启动前端 (Vite, :5173)"

setup:
	$(GOGET) -v ./...
	cp -n config.example.json config.json || true
	cd web && pnpm install

# Cross compilation
build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BUILD_DIR)/$(BINARY_UNIX) -v ./cmd/monitor

docker-build:
	docker build -t $(BINARY_NAME) .

# Help target
help:
	@echo "Available targets:"
	@echo "  make build       - 构建生产版本（含前端）"
	@echo "  make dev-api     - 启动后端开发服务器 (air 热重载, :8081)"
	@echo "  make dev-web     - 启动前端开发服务器 (Vite, :5173)"
	@echo "  make dev         - 显示开发模式说明"
	@echo "  make run         - 运行监控主程序"
	@echo "  make test        - 运行 Go 测试"
	@echo "  make clean       - 清理构建产物"
	@echo "  make setup       - 初始化环境"

