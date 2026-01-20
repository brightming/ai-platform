# API 密钥配置指南

## 概述

AI Platform 支持通过管理后台动态配置第三方 API 密钥，无需重启服务。

## 密钥管理架构

```
┌─────────────┐     HTTP API     ┌──────────────┐
│  管理后台    │ ────────────────> │ Key Manager  │
└─────────────┘                    └──────────────┘
                                           │
                                           │ 加密存储 (KMS)
                                           ▼
                                    ┌──────────────┐
                                    │   MySQL      │
                                    └──────────────┘
```

## 通过 API 添加密钥

### 1. 添加 OpenAI 密钥

```powershell
# 添加 OpenAI DALL-E 密钥
$body = @{
    vendor = "openai"
    service = "dalle"
    key_name = "OpenAI DALL-E Key"
    api_key = "sk-your-actual-key-here"
    tier = "primary"
    description = "用于 DALL-E 图像生成"
} | ConvertTo-Json -Depth 10

Invoke-RestMethod -Uri "http://localhost:8002/api/v1/keys" `
    -Method POST -Body $body -ContentType "application/json"
```

### 2. 添加阿里云密钥

```powershell
# 添加阿里云通义万相密钥
$body = @{
    vendor = "aliyun"
    service = "wanx"
    key_name = "阿里云通义万相"
    api_key = "sk-your-aliyun-key-here"
    tier = "primary"
    description = "用于通义万相图像生成"
} | ConvertTo-Json -Depth 10

Invoke-RestMethod -Uri "http://localhost:8002/api/v1/keys" `
    -Method POST -Body $body -ContentType "application/json"
```

## 密钥参数说明

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| vendor | string | 是 | 厂商标识: openai, aliyun |
| service | string | 是 | 服务名称: dalle, gpt-4, wanx, qwen |
| key_name | string | 是 | 密钥名称（便于识别） |
| api_key | string | 是 | 实际的 API 密钥 |
| tier | string | 否 | 密钥层级: primary(主), backup(备), overflow(溢出) |
| description | string | 否 | 密钥描述 |

## 查询密钥

```powershell
# 查询所有密钥
Invoke-RestMethod -Uri "http://localhost:8002/api/v1/keys" -Method GET

# 查询特定密钥
$keyId = "your-key-id"
Invoke-RestMethod -Uri "http://localhost:8002/api/v1/keys/$keyId" -Method GET

# 按厂商查询
Invoke-RestMethod -Uri "http://localhost:8002/api/v1/keys?vendor=openai" -Method GET
```

## 更新密钥

```powershell
$keyId = "your-key-id"
$body = @{
    key_name = "Updated Key Name"
    description = "新的描述"
    tier = "backup"
} | ConvertTo-Json

Invoke-RestMethod -Uri "http://localhost:8002/api/v1/keys/$keyId" `
    -Method PUT -Body $body -ContentType "application/json"
```

## 删除密钥

```powershell
$keyId = "your-key-id"
Invoke-RestMethod -Uri "http://localhost:8002/api/v1/keys/$keyId" -Method DELETE
```

## 密钥使用统计

```powershell
$keyId = "your-key-id"
Invoke-RestMethod -Uri "http://localhost:8002/api/v1/keys/$keyId/usage" -Method GET
```

## 支持的厂商和服务

### OpenAI
| vendor | service | 说明 |
|--------|---------|------|
| openai | dalle | DALL-E 图像生成 |
| openai | gpt-4 | GPT-4 文本生成 |
| openai | gpt-3.5-turbo | GPT-3.5 文本生成 |

### 阿里云
| vendor | service | 说明 |
|--------|---------|------|
| aliyun | wanx | 通义万相图像生成 |
| aliyun | qwen | 通义千问文本生成 |

## 密钥层级说明

| tier | 说明 | 使用场景 |
|------|------|----------|
| primary | 主密钥 | 优先使用 |
| backup | 备份密钥 | 主密钥不可用时使用 |
| overflow | 溢出密钥 | 主备密钥配额用尽时使用 |

## 安全说明

1. **密钥加密**: 所有密钥使用 KMS 加密后存储在数据库中
2. **审计日志**: 密钥的创建、更新、删除操作都会记录审计日志
3. **使用追踪**: 每次使用密钥都会记录使用日志
4. **权限控制**: 建议配置访问控制，限制只有管理员可以操作密钥

## 故障排查

### 密钥配置后仍然失败

1. 检查密钥是否启用：
```powershell
$key = Invoke-RestMethod -Uri "http://localhost:8002/api/v1/keys" -Method GET
$key.data | Where-Object { $_.enabled -eq $true }
```

2. 检查服务日志：
```powershell
docker-compose logs key-manager
docker-compose logs router-engine
```

3. 验证密钥健康状态：
```powershell
$keyId = "your-key-id"
Invoke-RestMethod -Uri "http://localhost:8002/api/v1/keys/$keyId/health" -Method GET
```
