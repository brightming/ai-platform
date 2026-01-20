# AI Platform - 混合大模型服务平台

## 项目结构

```
ai-platform/
├── cmd/                    # 主程序入口
│   ├── api-gateway/        # API网关
│   ├── router-engine/      # 路由引擎
│   ├── key-manager/        # 密钥管理服务
│   ├── service-registry/   # 服务注册中心
│   ├── metrics-collector/  # 监控采集器
│   └── config-center/      # 配置中心
├── pkg/                    # 公共包
│   ├── api/                # API定义
│   │   ├── config/         # 配置相关API
│   │   ├── key/            # 密钥相关API
│   │   ├── service/        # 服务相关API
│   │   └── router/         # 路由相关API
│   ├── model/              # 数据模型
│   │   ├── config.go
│   │   ├── key.go
│   │   ├── service.go
│   │   └── request.go
│   ├── service/            # 业务服务接口
│   │   ├── config/
│   │   ├── key/
│   │   ├── registry/
│   │   └── router/
│   ├── provider/           # 第三方API适配器
│   │   ├── openai/
│   │   ├── aliyun/
│   │   └── base.go
│   ├── storage/            # 存储层
│   │   ├── mysql/
│   │   ├── redis/
│   │   └── kms/
│   ├── metrics/            # 监控指标
│   └── util/               # 工具函数
├── internal/               # 内部实现
│   ├── config/
│   ├── key/
│   ├── registry/
│   └── router/
├── web/                    # 前端代码
├── deploy/                 # 部署配置
│   ├── k8s/               # Kubernetes配置
│   │   ├── base/
│   │   ├── config-center/
│   │   ├── key-manager/
│   │   ├── service-registry/
│   │   ├── router-engine/
│   │   └── api-gateway/
│   ├── mysql/
│   └── monitoring/
├── scripts/                # 脚本
├── docs/                   # 文档
├── go.mod
├── go.sum
├── Makefile
└── Dockerfile
```

## 快速开始

### 本地开发

```bash
# 安装依赖
go mod download

# 运行配置中心
make run-config-center

# 运行密钥管理服务
make run-key-manager

# 运行服务注册中心
make run-service-registry

# 运行路由引擎
make run-router-engine

# 运行API网关
make run-api-gateway
```

### 部署

```bash
# 构建镜像
make build-all

# 部署到Kubernetes
kubectl apply -f deploy/k8s/
```
