# AI Platform API 测试脚本

$BaseUrl = "http://localhost:8000"
$Headers = @{}

function Write-ColorOutput {
    param(
        [string]$Message,
        [string]$Color = "White"
    )
    Write-Host $Message -ForegroundColor $Color
}

function Test-Endpoint {
    param(
        [string]$Name,
        [string]$Method,
        [string]$Path,
        [hashtable]$Body = $null
    )

    Write-Host "Testing: $Name ... " -NoNewline

    try {
        $url = "$BaseUrl$Path"
        $bodyJson = if ($Body) { ($Body | ConvertTo-Json -Depth 10) } else { $null }

        if ($Method -eq "GET") {
            $response = Invoke-RestMethod -Uri $url -Method Get -Headers $Headers -ErrorAction Stop
        } elseif ($Method -eq "POST") {
            $response = Invoke-RestMethod -Uri $url -Method Post -Body $bodyJson -ContentType "application/json" -Headers $Headers -ErrorAction Stop
        } elseif ($Method -eq "PUT") {
            $response = Invoke-RestMethod -Uri $url -Method Put -Body $bodyJson -ContentType "application/json" -Headers $Headers -ErrorAction Stop
        } elseif ($Method -eq "DELETE") {
            $response = Invoke-RestMethod -Uri $url -Method Delete -Headers $Headers -ErrorAction Stop
        }

        Write-Host "OK" -ForegroundColor Green
        return $response
    } catch {
        Write-Host "FAILED" -ForegroundColor Red
        Write-Host "  Error: $($_.Exception.Message)" -ForegroundColor DarkRed
        return $null
    }
}

# 开始测试
Write-ColorOutput "========================================" -ForegroundColor Cyan
Write-ColorOutput "  AI Platform API 测试" -ForegroundColor Cyan
Write-ColorOutput "========================================" -ForegroundColor Cyan
Write-Host ""

Write-ColorOutput "[1] 健康检查" -ForegroundColor Yellow
$result = Test-Endpoint "Health Check" "GET" "/healthz"
Write-Host ""

Write-ColorOutput "[2] 就绪检查" -ForegroundColor Yellow
$result = Test-Endpoint "Readiness Check" "GET" "/ready"
Write-Host ""

Write-ColorOutput "[3] 创建特性配置 (文生图)" -ForegroundColor Yellow
$featureConfig = @{
    id = "text_to_image"
    name = "文生图"
    description = "文本生成图像功能"
    enabled = $true
    self_hosted = @{
        enabled = $true
        priority = 1
        cost_per_request = 0.01
        max_concurrent = 10
    }
    third_party = @(
        @{
            provider_id = "openai"
            provider_name = "OpenAI"
            provider_type = "openai"
            priority = 2
            cost_per_request = 0.02
            enabled = $true
        }
    )
    routing = @{
        strategy = "cost_based"
        fallback_enabled = $true
    }
}
$result = Test-Endpoint "Create Feature" "POST" "/api/v1/features" $featureConfig
Write-Host ""

Write-ColorOutput "[4] 获取特性配置" -ForegroundColor Yellow
$result = Test-Endpoint "Get Feature" "GET" "/api/v1/features/text_to_image"
if ($result) {
    Write-Host "  Feature: $($result.name)" -ForegroundColor Cyan
    Write-Host "  Enabled: $($result.enabled)" -ForegroundColor Cyan
}
Write-Host ""

Write-ColorOutput "[5] 注册第三方 API Key" -ForegroundColor Yellow
$apiKeyRequest = @{
    provider_id = "openai"
    provider_type = "openai"
    key_name = "OpenAI API Key"
    encrypted_key = "sk-test-key-for-local-testing-only"
    description = "测试用 OpenAI 密钥"
}
$result = Test-Endpoint "Register API Key" "POST" "/api/v1/keys" $apiKeyRequest
Write-Host ""

Write-ColorOutput "[6] 获取 API Keys 列表" -ForegroundColor Yellow
$result = Test-Endpoint "List API Keys" "GET" "/api/v1/keys"
Write-Host ""

Write-ColorOutput "[7] 查询路由决策" -ForegroundColor Yellow
$routeRequest = @{
    feature = "text_to_image"
    estimated_cost = 0.01
    priority = "normal"
}
$result = Test-Endpoint "Get Route Decision" "POST" "/api/v1/route/decide" $routeRequest
if ($result) {
    Write-Host "  Selected Provider: $($result.provider_id)" -ForegroundColor Cyan
    Write-Host "  Reason: $($result.reason)" -ForegroundColor Cyan
}
Write-Host ""

Write-ColorOutput "[8] 查询预算状态" -ForegroundColor Yellow
$result = Test-Endpoint "Check Budget" "GET" "/api/v1/budget/check?feature=text_to_image"
if ($result) {
    Write-Host "  Allowed: $($result.allowed)" -ForegroundColor Cyan
    if ($result.global_budget) {
        Write-Host "  Global Budget: $($result.global_budget.used)/$($result.global_budget.total)" -ForegroundColor Cyan
    }
}
Write-Host ""

Write-ColorOutput "[9] 获取服务列表" -ForegroundColor Yellow
$result = Test-Endpoint "List Services" "GET" "/api/v1/services?service_type=text_to_image"
Write-Host ""

Write-ColorOutput "[10] 获取服务统计" -ForegroundColor Yellow
$result = Test-Endpoint "Get Statistics" "GET" "/api/v1/stats"
Write-Host ""

Write-ColorOutput "========================================" -ForegroundColor Cyan
Write-ColorOutput "  测试完成!" -ForegroundColor Green
Write-ColorOutput "========================================" -ForegroundColor Cyan
