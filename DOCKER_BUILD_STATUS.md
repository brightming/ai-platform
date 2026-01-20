# Docker构建问题解决记录

## 当前状态

正在解决 Docker 镜像构建过程中的 Go 模块路径问题。

### 问题描述

AI Platform 项目使用纯本地代码（不在GitHub上），Go 模块系统要求模块路径必须是域名格式（如 `github.com/xxx/xxx`）。这导致 `go mod tidy` 和 `go build` 在 Docker 内部尝试访问互联网获取模块。

### 已尝试的方案

| 方案 | 模块路径 | 结果 | 问题 |
|------|----------|------|------|
| GitHub路径 | `github.com/yijian/ai-platform` | 失败 | 尝试访问GitHub |
| 本地域名 | `ai-platform` | 失败 | 被当作标准库路径 |
| local.dev | `local.dev/ai-platform` | 失败 | 尝试网络访问 |
| localhost | `localhost/ai-platform` | 失败 | localhost是保留关键字 |
| localdomain | `localdomain/ai-platform` | 失败 | 尝试网络访问 |
| mycompany.local | `mycompany.local/ai-platform` | 进行中 | 仍在尝试网络访问 |

### 当前配置

**go.mod:**
```
module mycompany.local/ai-platform
```

**Dockerfile 环境变量:**
```dockerfile
ENV GOPROXY=https://goproxy.cn,direct
ENV GOSUMDB=off
ENV GOPRIVATE=mycompany.local
ENV GONOPROXY=mycompany.local
ENV GONOSUMDB=mycompany.local
```

**构建命令:**
```dockerfile
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -v -mod=mod \
    -ldflags="-X main.Version=${VERSION} -s -w" \
    -o /bin/${SERVICE} \
    ./cmd/${SERVICE}
```

### 错误信息

```
go: mycompany.local/ai-platform/cmd/api-gateway imports
	mycompany.local/ai-platform/pkg/metrics/prometheus: cannot find module providing package mycompany.local/ai-platform/pkg/metrics/prometheus: unrecognized import path "mycompany.local/ai-platform/pkg/metrics/prometheus": https fetch: Get "https://mycompany.local/ai-platform/pkg/metrics/prometheus?go-get=1": dial tcp: lookup mycompany.local on 192.168.65.7:53: no such host
```

## 接下来要做的

### 方案1：使用 Vendor 目录（推荐）
1. 在本地生成 vendor 目录：`go mod vendor`
2. 修改 Dockerfile 使用 `-mod=vendor` 构建
3. 将 vendor 目录包含在 Docker 上下文中

### 方案2：简化导入路径
1. 将所有代码放在单一包下
2. 使用相对导入（不推荐，Go 社区不鼓励）

### 方案3：预先生成 go.sum
1. 在本地运行 `go mod tidy` 生成完整的 go.sum
2. 修改 Dockerfile 跳过 `go mod tidy` 步骤
3. 直接使用预生成的 go.sum 构建

### 方案4：使用 Go workspace
1. 创建 `go.work` 文件
2. 使用 Go 1.18+ 的 workspace 特性

## 项目结构

```
ai-platform/
├── cmd/
│   ├── config-center/main.go
│   ├── key-manager/main.go
│   ├── service-registry/main.go
│   ├── router-engine/main.go
│   ├── api-gateway/main.go
│   └── scaler/main.go
├── internal/
│   ├── config/service.go
│   ├── key/service.go
│   ├── registry/service.go
│   ├── router/engine.go
│   ├── budget/service.go
│   └── scaler/controller.go
├── pkg/
│   ├── api/
│   ├── metrics/
│   ├── model/
│   ├── provider/
│   └── storage/
├── deploy/
│   ├── k8s/
│   └── local/
├── build/
│   └── Dockerfile
├── go.mod
├── go.sum
└── docker-compose.cn.yml
```

## 依赖服务

- MySQL: 数据存储
- Redis: 缓存和速率限制
- Prometheus: 监控指标
- Grafana: 监控面板

## 待解决问题

1. **Go 模块本地化**: 需要让 Go 识别本地模块而不尝试网络访问
2. **Docker 多服务构建**: 需要支持构建 config-center, key-manager, service-registry, router-engine, api-gateway, scaler 等多个服务
