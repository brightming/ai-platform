# AI Platform 本地启动脚本 (Windows PowerShell)

param(
    [switch]$SkipBuild = $false,
    [switch]$Detach = $false
)

$ErrorActionPreference = "Stop"

function Write-ColorOutput {
    param(
        [string]$Message,
        [string]$Color = "White"
    )
    Write-Host $Message -ForegroundColor $Color
}

function Test-Docker {
    Write-ColorOutput "[INFO] 检查 Docker..." -ForegroundColor Green
    try {
        docker version | Out-Null
        Write-ColorOutput "[OK] Docker 已安装" -ForegroundColor Green
    } catch {
        Write-ColorOutput "[ERROR] Docker 未安装或未运行，请先安装 Docker Desktop" -ForegroundColor Red
        exit 1
    }

    try {
        docker ps | Out-Null
        Write-ColorOutput "[OK] Docker 运行正常" -ForegroundColor Green
    } catch {
        Write-ColorOutput "[ERROR] Docker 未运行，请启动 Docker Desktop" -ForegroundColor Red
        exit 1
    }
}

function Stop-ExistingServices {
    Write-ColorOutput "[INFO] 停止现有服务..." -ForegroundColor Yellow
    try {
        docker-compose down
    } catch {
        # Ignore if no services running
    }
}

function Build-Images {
    Write-ColorOutput "[INFO] 构建 Docker 镜像..." -ForegroundColor Green
    docker-compose build
    if ($LASTEXITCODE -ne 0) {
        Write-ColorOutput "[ERROR] 镜像构建失败" -ForegroundColor Red
        exit 1
    }
    Write-ColorOutput "[OK] 镜像构建成功" -ForegroundColor Green
}

function Start-Services {
    Write-ColorOutput "[INFO] 启动服务..." -ForegroundColor Green

    if ($Detach) {
        docker-compose up -d
    } else {
        docker-compose up
    }

    if ($LASTEXITCODE -ne 0) {
        Write-ColorOutput "[ERROR] 服务启动失败" -ForegroundColor Red
        exit 1
    }
}

function Wait-Services {
    Write-ColorOutput "[INFO] 等待服务启动..." -ForegroundColor Green

    $services = @(
        @{Name="MySQL"; Port=3306},
        @{Name="Redis"; Port=6379},
        @{Name="Config Center"; Port=8001},
        @{Name="Key Manager"; Port=8002},
        @{Name="Service Registry"; Port=8003},
        @{Name="Router Engine"; Port=8004},
        @{Name="API Gateway"; Port=8000}
    )

    foreach ($service in $services) {
        Write-Host "  等待 $($service.Name)..." -NoNewline
        $maxWait = 60
        $waited = 0
        while ($waited -lt $maxWait) {
            try {
                $tcp = New-Object System.Net.Sockets.TcpClient
                $tcp.Connect("localhost", $service.Port)
                $tcp.Close()
                Write-Host " OK" -ForegroundColor Green
                break
            } catch {
                Start-Sleep -Seconds 1
                $waited++
            }
        }
        if ($waited -ge $maxWait) {
            Write-Host " 超时!" -ForegroundColor Red
        }
    }
}

function Show-Summary {
    Write-Host ""
    Write-ColorOutput "========================================" -ForegroundColor Cyan
    Write-ColorOutput "  AI Platform 本地服务已启动!" -ForegroundColor Cyan
    Write-ColorOutput "========================================" -ForegroundColor Cyan
    Write-Host ""
    Write-ColorOutput "服务地址:" -ForegroundColor Yellow
    Write-Host "  API Gateway:        http://localhost:8000"
    Write-Host "  Config Center:      http://localhost:8001"
    Write-Host "  Key Manager:        http://localhost:8002"
    Write-Host "  Service Registry:   http://localhost:8003"
    Write-Host "  Router Engine:      http://localhost:8004"
    Write-Host "  Prometheus:         http://localhost:9090"
    Write-Host "  Grafana:            http://localhost:3000 (admin/admin123)"
    Write-Host ""
    Write-ColorOutput "常用命令:" -ForegroundColor Yellow
    Write-Host "  查看日志:    docker-compose logs -f [服务名]"
    Write-Host "  停止服务:    docker-compose down"
    Write-Host "  重启服务:    docker-compose restart [服务名]"
    Write-Host "  进入容器:    docker exec -it ai-platform-api-gateway sh"
    Write-Host ""
}

# 主流程
Write-ColorOutput "========================================" -ForegroundColor Cyan
Write-ColorOutput "  AI Platform 本地部署" -ForegroundColor Cyan
Write-ColorOutput "========================================" -ForegroundColor Cyan
Write-Host ""

Test-Docker
Stop-ExistingServices

if (-not $SkipBuild) {
    Build-Images
}

Start-Services

if ($Detach) {
    Wait-Services
    Start-Sleep -Seconds 2
    Show-Summary
}
