# AI Platform 本地部署指南 (Windows)

## 前置条件

1. **安装 Docker Desktop for Windows**
   - 下载: https://www.docker.com/products/docker-desktop/
   - 安装后启动 Docker Desktop
   - 确保 Docker 正在运行

2. **验证 Docker 安装**
   ```powershell
   docker --version
   docker ps
   ```

## 部署步骤

### 1. 打开 PowerShell

右键点击开始菜单，选择 "Windows PowerShell"

### 2. 进入项目目录

```powershell
cd D:\projects\yijian\docs\ai-platform
```

### 3. 启动服务

```powershell
# 方式一：使用启动脚本（推荐）
.\deploy\local\start.ps1 -Detach

# 方式二：手动启动
docker-compose up -d
```

### 4. 查看服务状态

```powershell
docker-compose ps
```

### 5. 查看日志

```powershell
# 查看所有服务日志
docker-compose logs -f

# 查看特定服务日志
docker-compose logs -f api-gateway
docker-compose logs -f config-center
```

## 服务地址

启动成功后，可以通过以下地址访问各服务：

| 服务 | 地址 | 说明 |
|------|------|------|
| API Gateway | http://localhost:8000 | 主API入口 |
| Config Center | http://localhost:8001 | 配置中心 |
| Key Manager | http://localhost:8002 | 密钥管理 |
| Service Registry | http://localhost:8003 | 服务注册 |
| Router Engine | http://localhost:8004 | 路由引擎 |
| Prometheus | http://localhost:9090 | 监控指标 |
| Grafana | http://localhost:3000 | 监控面板 (admin/admin123) |

## 测试API

### 运行自动化测试

```powershell
.\deploy\local\test.ps1
```

### 手动测试

```powershell
# 健康检查
curl http://localhost:8000/healthz

# 获取特性列表
curl http://localhost:8000/api/v1/features

# 获取特性详情
curl http://localhost:8000/api/v1/features/text_to_image

# 查询路由决策
curl -X POST http://localhost:8000/api/v1/route/decide `
  -H "Content-Type: application/json" `
  -d '{"feature":"text_to_image","estimated_cost":0.01}'

# 检查预算
curl http://localhost:8000/api/v1/budget/check?feature=text_to_image

# 获取统计信息
curl http://localhost:8000/api/v1/stats
```

## 常用命令

### 停止所有服务

```powershell
docker-compose down
```

### 重启某个服务

```powershell
docker-compose restart api-gateway
```

### 重新构建并启动

```powershell
docker-compose up -d --build
```

### 进入容器

```powershell
docker exec -it ai-platform-api-gateway sh
```

### 查看容器资源使用

```powershell
docker stats
```

## 故障排查

### 端口被占用

如果端口被占用，可以修改 `docker-compose.yml` 中的端口映射：

```yaml
ports:
  - "新端口:8080"  # 例如 "8100:8080"
```

### 服务启动失败

1. 查看详细日志
   ```powershell
   docker-compose logs [服务名]
   ```

2. 检查容器状态
   ```powershell
   docker-compose ps
   ```

3. 重新构建镜像
   ```powershell
   docker-compose build --no-cache [服务名]
   ```

### 数据库连接问题

确保MySQL容器已启动：
```powershell
docker-compose logs mysql
```

### 清理所有数据

```powershell
# 停止并删除所有容器、网络、卷
docker-compose down -v

# 删除所有镜像
docker rmi (docker images -q ai-platform-*)
```

## 开发模式

如果需要修改代码后重新构建：

```powershell
# 重新构建单个服务
docker-compose build api-gateway
docker-compose up -d api-gateway

# 重新构建所有服务
docker-compose build
docker-compose up -d
```

## 下一步

1. 测试各API端点功能
2. 查看Grafana监控面板
3. 配置实际的第三方API密钥
4. 部署自建镜像服务实例
