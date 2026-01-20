# Makefile for AI Platform

# 变量
REGISTRY := registry.com
IMAGE_PREFIX := $(REGISTRY)/ai
VERSION := v1.0.0

# Go相关
GOCMD := go
GOBUILD := $(GOCMD) build
GORUN := $(GOCMD) run
GOTEST := $(GOCMD) test
GOMOD := $(GOCMD) mod
GOFMT := gofmt

# Kubernetes
KUBECTL := kubectl
HELM := helm

# 目录
CMD_DIR := ./cmd
DEPLOY_DIR := ./deploy

# 服务列表
SERVICES := config-center key-manager service-registry router-engine api-gateway scaler

.PHONY: all build test clean run fmt deps k8s-apply k8s-delete

# 默认目标
all: build

# 编译所有服务
build:
	@echo "Building all services..."
	@for service in $(SERVICES); do \
		echo "Building $$service..."; \
		$(GOBUILD) -o bin/$$service $(CMD_DIR)/$$service/main.go; \
	done
	@echo "Build complete!"

# 编译单个服务
build-%:
	@echo "Building $*..."
	$(GOBUILD) -o bin/$* $(CMD_DIR)/$*/main.go

# 运行服务
run-%:
	@echo "Running $*..."
	$(GORUN) $(CMD_DIR)/$*/main.go

# 测试
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

# 测试覆盖率
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# 格式化代码
fmt:
	@echo "Formatting code..."
	$(GOFMT) -s -w .

# 下载依赖
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

# 清理
clean:
	@echo "Cleaning..."
	rm -rf bin/
	rm -f coverage.out coverage.html

# 构建Docker镜像
docker-build:
	@echo "Building Docker images..."
	@for service in $(SERVICES); do \
		echo "Building $(IMAGE_PREFIX)/$$service:$(VERSION)..."; \
		docker build --build-arg SERVICE=$$service --build-arg VERSION=$(VERSION) -f build/Dockerfile -t $(IMAGE_PREFIX)/$$service:$(VERSION) .; \
		docker tag $(IMAGE_PREFIX)/$$service:$(VERSION) $(IMAGE_PREFIX)/$$service:latest; \
	done

# 推送Docker镜像
docker-push:
	@echo "Pushing Docker images..."
	@for service in $(SERVICES); do \
		echo "Pushing $(IMAGE_PREFIX)/$$service:$(VERSION)..."; \
		docker push $(IMAGE_PREFIX)/$$service:$(VERSION); \
		docker push $(IMAGE_PREFIX)/$$service:latest; \
	done

# 部署到Kubernetes
k8s-apply:
	@echo "Applying Kubernetes manifests..."
	$(KUBECTL) apply -f $(DEPLOY_DIR)/k8s/base/
	$(KUBECTL) apply -f $(DEPLOY_DIR)/k8s/config-center/
	$(KUBECTL) apply -f $(DEPLOY_DIR)/k8s/key-manager/
	$(KUBECTL) apply -f $(DEPLOY_DIR)/k8s/service-registry/
	$(KUBECTL) apply -f $(DEPLOY_DIR)/k8s/router-engine/
	$(KUBECTL) apply -f $(DEPLOY_DIR)/k8s/api-gateway/
	$(KUBECTL) apply -f $(DEPLOY_DIR)/k8s/scaler/

# 删除Kubernetes资源
k8s-delete:
	@echo "Deleting Kubernetes resources..."
	$(KUBECTL) delete -f $(DEPLOY_DIR)/k8s/scaler/ --ignore-not-found=true
	$(KUBECTL) delete -f $(DEPLOY_DIR)/k8s/api-gateway/ --ignore-not-found=true
	$(KUBECTL) delete -f $(DEPLOY_DIR)/k8s/router-engine/ --ignore-not-found=true
	$(KUBECTL) delete -f $(DEPLOY_DIR)/k8s/service-registry/ --ignore-not-found=true
	$(KUBECTL) delete -f $(DEPLOY_DIR)/k8s/key-manager/ --ignore-not-found=true
	$(KUBECTL) delete -f $(DEPLOY_DIR)/k8s/config-center/ --ignore-not-found=true
	$(KUBECTL) delete -f $(DEPLOY_DIR)/k8s/base/ --ignore-not-found=true

# 重启服务
k8s-restart:
	@echo "Restarting services..."
	@for service in $(SERVICES); do \
		$(KUBECTL) rollout restart deployment/$$service -n ai-platform; \
	done

# 查看状态
k8s-status:
	@echo "Checking deployment status..."
	$(KUBECTL) get deployments -n ai-platform
	@echo ""
	$(KUBECTL) get pods -n ai-platform
	@echo ""
	$(KUBECTL) get services -n ai-platform

# 查看日志
k8s-logs-%:
	@$(KUBECTL) logs -f deployment/$* -n ai-platform

# 端口转发
k8s-portforward-gateway:
	@echo "Port forwarding api-gateway to localhost:8080..."
	$(KUBECTL) port-forward -n ai-platform svc/api-gateway 8080:80

# 数据库迁移
db-migrate:
	@echo "Running database migrations..."
	$(GORUN) $(CMD_DIR)/migrate/main.go up

# 生成Swagger文档
swagger:
	@echo "Generating Swagger documentation..."
	swag init -g $(CMD_DIR)/api-gateway/main.go -o docs/

# 帮助
help:
	@echo "Available targets:"
	@echo "  all              - Build all services"
	@echo "  build            - Build all services (alias for all)"
	@echo "  build-<service>  - Build specific service (e.g., build-api-gateway)"
	@echo "  run-<service>    - Run specific service (e.g., run-api-gateway)"
	@echo "  test             - Run tests"
	@echo "  test-coverage    - Run tests with coverage"
	@echo "  fmt              - Format code"
	@echo "  deps             - Download dependencies"
	@echo "  clean            - Clean build artifacts"
	@echo "  docker-build     - Build Docker images"
	@echo "  docker-push      - Push Docker images"
	@echo "  k8s-apply        - Deploy to Kubernetes"
	@echo "  k8s-delete       - Delete from Kubernetes"
	@echo "  k8s-restart      - Restart deployments"
	@echo "  k8s-status       - Show deployment status"
	@echo "  k8s-logs-<svc>   - Show logs for service"
	@echo "  db-migrate       - Run database migrations"
