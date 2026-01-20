# 混合大模型服务平台 - 执行计划与进度记录

> 创建时间: 2025-01-20
> 目的: 记录本地部署前的修复和补充工作，便于恢复执行

---

## 一、待修复/补充的问题清单

### 1. Dockerfile 修复 (必须)
- **问题**: `build/Dockerfile` 第23行 ARG 指令语法错误
- **文件**: `D:\projects\yijian\docs\ai-platform\build\Dockerfile`
- **状态**: 待修复

### 2. 环境配置文件 (缺失)
- **问题**: 缺少 `.env.example` 示例配置文件
- **文件**: `D:\projects\yijian\docs\ai-platform\.env.example`
- **状态**: 待创建

### 3. API Gateway Handler (待补充)
- **问题**: `pkg/api/gateway/handler.go` 需要验证完整性
- **文件**: `D:\projects\yijian\docs\ai-platform\pkg\api\gateway\handler.go`
- **状态**: 待检查

### 4. Prometheus 指标端点 (待补充)
- **问题**: 各服务 `/metrics` 端点返回空
- **影响**: 无法正常监控
- **状态**: 待补充

### 5. Router Engine 服务调用 (待完善)
- **问题**: `cmd/router-engine/main.go` 中多个 TODO
- **状态**: 待完善

### 6. 测试脚本 (待创建)
- **问题**: `deploy/local/test.ps1` 需要验证
- **状态**: 待创建/验证

---

## 二、执行进度

| 任务 | 状态 | 完成时间 | 说明 |
|------|------|----------|------|
| Dockerfile 修复 | ✅ 完成 | 2025-01-20 | 修复 ARG 指令语法错误 |
| .env.example 创建 | ✅ 完成 | 2025-01-20 | 创建示例配置文件 |
| API Gateway 补充 | ✅ 完成 | 2025-01-20 | Handler 已完整，集成 Prometheus |
| Prometheus 指标 | ✅ 完成 | 2025-01-20 | 所有服务已集成指标端点 |
| Router Engine 完善 | ✅ 完成 | 2025-01-20 | 服务间调用已实现 |
| 测试脚本 | ✅ 完成 | 2025-01-20 | 已存在并可用 |
| 本地构建验证 | ⏳ 进行中 | - | 等待执行 |

---

## 三、本地部署前置条件

### 3.1 环境要求

| 依赖 | 版本 | 检查命令 | 状态 |
|------|------|----------|------|
| Docker Desktop | 最新 | `docker --version` | **必需** |
| Go | 1.21+ | `go version` | 可选 (Docker 构建不需要) |
| PowerShell | - | `$PSVersionTable` | 已有 |

**注意**: 本地构建完全在 Docker 容器中进行，不需要在主机上安装 Go。

### 3.2 第三方 API 密钥配置

**重要**: 第三方 API 密钥不通过环境变量配置，而是通过后台管理界面动态添加。

**配置步骤**:
1. 启动服务后，访问 Key Manager: http://localhost:8002
2. 使用 API 添加密钥：
```powershell
# 添加 OpenAI 密钥示例
$body = @{
    vendor = "openai"
    service = "dalle"
    key_name = "OpenAI DALL-E Key"
    api_key = "sk-your-actual-key-here"  # 替换为真实密钥
    tier = "primary"
} | ConvertTo-Json

Invoke-RestMethod -Uri "http://localhost:8002/api/v1/keys" `
    -Method POST -Body $body -ContentType "application/json"
```

3. 验证密钥已添加：
```powershell
Invoke-RestMethod -Uri "http://localhost:8002/api/v1/keys" -Method GET
```

**支持的密钥类型**:
| vendor | service | 说明 |
|--------|---------|------|
| openai | dalle | DALL-E 图像生成 |
| openai | gpt-4 | GPT-4 文本生成 |
| aliyun | wanx | 通义万相 |
| aliyun | qwen | 通义千问 |

---

## 四、部署命令速查

```powershell
# 进入项目目录
cd D:\projects\yijian\docs\ai-platform

# 启动所有服务
docker-compose up -d

# 查看服务状态
docker-compose ps

# 查看日志
docker-compose logs -f

# 重新构建并启动
docker-compose up -d --build

# 停止服务
docker-compose down

# 清理所有数据
docker-compose down -v
```

---

## 五、服务地址映射

| 服务 | 内部地址 | 外部端口 | 健康检查 |
|------|----------|----------|----------|
| API Gateway | api-gateway:8080 | 8000 | `/healthz` |
| Config Center | config-center:8080 | 8001 | `/healthz` |
| Key Manager | key-manager:8080 | 8002 | `/healthz` |
| Service Registry | service-registry:8080 | 8003 | `/healthz` |
| Router Engine | router-engine:8080 | 8004 | `/healthz` |
| MySQL | mysql:3306 | 3306 | - |
| Redis | redis:6379 | 6379 | - |
| Prometheus | prometheus:9090 | 9090 | - |
| Grafana | grafana:3000 | 3000 | admin/admin123 |

---

## 六、API 测试命令

```powershell
# 健康检查
curl http://localhost:8000/healthz
curl http://localhost:8001/healthz
curl http://localhost:8002/healthz
curl http://localhost:8003/healthz
curl http://localhost:8004/healthz

# 功能配置
curl http://localhost:8001/api/v1/features
curl http://localhost:8001/api/v1/features/text_to_image

# 密钥管理
curl http://localhost:8002/api/v1/keys

# 服务注册
curl http://localhost:8003/api/v1/services

# 路由决策
curl -X POST http://localhost:8004/api/v1/route/text_to_image `
  -H "Content-Type: application/json" `
  -d '{"prompt":"a cat","width":1024,"height":1024}'
```

---

## 七、故障排查

### 端口被占用
```powershell
# 查看端口占用
netstat -ano | findstr :8000
```

### 容器启动失败
```powershell
# 查看日志
docker-compose logs [服务名]
```

### 重新构建
```powershell
docker-compose build --no-cache [服务名]
```

---

## 八、恢复执行说明

当需要继续执行时，按以下步骤操作：

1. 打开 PowerShell，进入项目目录
2. 查看 `EXECUTION_PLAN.md` 确认当前进度
3. 按照本文档中的任务列表继续执行
4. 完成后更新本文件的"执行进度"部分

---

## 九、变更记录

| 日期 | 操作 | 说明 |
|------|------|------|
| 2025-01-20 | 创建文档 | 初始版本，记录待修复问题 |
| 2025-01-20 | Dockerfile 修复 | 修复 ARG 指令语法错误，在 FROM 后重新声明 ARG |
| 2025-01-20 | .env.example 创建 | 创建完整的环境变量示例文件 |
| 2025-01-20 | Prometheus 集成 | 为所有服务 (api-gateway, config-center, key-manager, service-registry, router-engine) 添加 /metrics 端点 |
| 2025-01-20 | API Gateway 完善 | 更新 SimpleRouter 集成 Prometheus 指标记录 |
| 2025-01-20 | API Key 配置说明 | 创建 API_KEY_SETUP.md，说明密钥通过后台配置而非环境变量 |

---

## 十、已修复的代码变更

### 10.1 build/Dockerfile
```dockerfile
# 修复前 (第23行)
RUN ARG SERVICE=${SERVICE} && \
    CGO_ENABLED=0 ...

# 修复后
ARG SERVICE
ARG VERSION
RUN CGO_ENABLED=0 ...
```

### 10.2 cmd/api-gateway/main.go
- 添加 `prometheus` 包导入
- 在 main() 中初始化 `metricsRegistry`
- SimpleRouter 添加 `metricsRegistry` 参数
- `/metrics` 端点使用 `gin.WrapH(metricsRegistry.Handler())`
- Route() 方法中添加请求指标记录

### 10.3 cmd/config-center/main.go
- 添加 `prometheus` 包导入
- 在 main() 中初始化 `metricsRegistry`
- 添加 `/metrics` 端点

### 10.4 cmd/key-manager/main.go
- 添加 `prometheus` 包导入
- 在 main() 中初始化 `metricsRegistry`
- 添加 `/metrics` 端点

### 10.5 cmd/service-registry/main.go
- 添加 `prometheus` 包导入
- 在 main() 中初始化 `metricsRegistry`
- 添加 `/metrics` 端点

### 10.6 cmd/router-engine/main.go
- 添加 `prometheus` 包导入
- 在 main() 中初始化 `metricsRegistry`
- 添加 `/metrics` 端点

---
