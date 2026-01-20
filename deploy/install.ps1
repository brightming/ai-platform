# AI Platform Installation Script for Windows PowerShell

param(
    [string]$Namespace = "ai-platform",
    [string]$Registry = "registry.com/ai",
    [string]$Version = "v1.0.0",
    [switch]$SkipSecrets = $false
)

function Write-ColorOutput {
    param(
        [string]$Message,
        [string]$Color = "White"
    )
    Write-Host $Message -ForegroundColor $Color
}

function Test-Prerequisites {
    Write-ColorOutput "[INFO] Checking prerequisites..." -ForegroundColor Green

    # Check kubectl
    if (-not (Get-Command kubectl -ErrorAction SilentlyContinue)) {
        Write-ColorOutput "[ERROR] kubectl not found. Please install kubectl." -ForegroundColor Red
        exit 1
    }

    # Check kubectl context
    try {
        kubectl cluster-info | Out-Null
    } catch {
        Write-ColorOutput "[ERROR] Cannot connect to Kubernetes cluster. Please configure kubectl." -ForegroundColor Red
        exit 1
    }

    Write-ColorOutput "[INFO] Prerequisites check passed!" -ForegroundColor Green
}

function New-Namespace {
    Write-ColorOutput "[INFO] Creating namespace ${Namespace}..." -ForegroundColor Green
    kubectl apply -f deploy/k8s/base/namespace.yaml
}

function New-Secrets {
    if ($SkipSecrets) {
        Write-ColorOutput "[WARN] Skipping secrets creation. Please create them manually." -ForegroundColor Yellow
        return
    }

    Write-ColorOutput "[INFO] Creating secrets..." -ForegroundColor Green
    Write-ColorOutput "[WARN] Please update the following secrets with your actual values:" -ForegroundColor Yellow
    Write-ColorOutput "[WARN]   - config-center-secret: Database credentials" -ForegroundColor Yellow
    Write-ColorOutput "[WARN]   - key-manager-rrsa: RRSA role name" -ForegroundColor Yellow
    Write-ColorOutput "[WARN]   - api-gateway-secret: JWT secret" -ForegroundColor Yellow

    # Prompt for database password
    $DB_PASSWORD = Read-Host "Enter MySQL password" -AsSecureString
    $DB_PASSWORD_TEXT = [System.Runtime.InteropServices.Marshal]::PtrToStringAuto([System.Runtime.InteropServices.Marshal]::SecureStringToBSTR($DB_PASSWORD))

    # Create database secret
    $dbHost = Read-Host "Enter MySQL host [default: rm-xxx.mysql.rds.aliyuncs.com]"
    if (-not $dbHost) { $dbHost = "rm-xxx.mysql.rds.aliyuncs.com" }

    $dbPort = Read-Host "Enter MySQL port [default: 3306]"
    if (-not $dbPort) { $dbPort = "3306" }

    $dbName = Read-Host "Enter database name [default: ai_platform]"
    if (-not $dbName) { $dbName = "ai_platform" }

    $dbUser = Read-Host "Enter database user [default: ai_platform]"
    if (-not $dbUser) { $dbUser = "ai_platform" }

    kubectl create secret generic config-center-secret `
        --from-literal=DB_HOST=$dbHost `
        --from-literal=DB_PORT=$dbPort `
        --from-literal=DB_NAME=$dbName `
        --from-literal=DB_USER=$dbUser `
        --from-literal=DB_PASSWORD=$DB_PASSWORD_TEXT `
        -n $Namespace --dry-run=client -o yaml | kubectl apply -f -

    # Create JWT secret
    $jwtSecret = -join ((48..57) + (65..90) + (97..122) | Get-Random -Count 32 | % {[char]$_})
    kubectl create secret generic api-gateway-secret `
        --from-literal=JWT_SECRET=$jwtSecret `
        -n $Namespace --dry-run=client -o yaml | kubectl apply -f -
}

function Deploy-CoreServices {
    Write-ColorOutput "[INFO] Deploying core services..." -ForegroundColor Green

    Write-ColorOutput "[INFO] Deploying Config Center..." -ForegroundColor Green
    kubectl apply -f deploy/k8s/config-center/

    Write-ColorOutput "[INFO] Deploying Key Manager..." -ForegroundColor Green
    kubectl apply -f deploy/k8s/key-manager/

    Write-ColorOutput "[INFO] Deploying Service Registry..." -ForegroundColor Green
    kubectl apply -f deploy/k8s/service-registry/

    Write-ColorOutput "[INFO] Deploying Router Engine..." -ForegroundColor Green
    kubectl apply -f deploy/k8s/router-engine/

    Write-ColorOutput "[INFO] Deploying API Gateway..." -ForegroundColor Green
    kubectl apply -f deploy/k8s/api-gateway/

    Write-ColorOutput "[INFO] Waiting for pods to be ready..." -ForegroundColor Green
    kubectl wait --for=condition=ready pod -l app=config-center -n $Namespace --timeout=300s
    kubectl wait --for=condition=ready pod -l app=key-manager -n $Namespace --timeout=300s
    kubectl wait --for=condition=ready pod -l app=service-registry -n $Namespace --timeout=300s
    kubectl wait --for=condition=ready pod -l app=router-engine -n $Namespace --timeout=300s
    kubectl wait --for=condition=ready pod -l app=api-gateway -n $Namespace --timeout=300s
}

function Deploy-SupportingServices {
    Write-ColorOutput "[INFO] Deploying supporting services..." -ForegroundColor Green

    Write-ColorOutput "[INFO] Deploying Redis..." -ForegroundColor Green
    kubectl apply -f deploy/k8s/redis/

    Write-ColorOutput "[INFO] Deploying Scaler..." -ForegroundColor Green
    kubectl apply -f deploy/k8s/scaler/

    Write-ColorOutput "[INFO] Deploying Monitoring (Prometheus, Grafana)..." -ForegroundColor Green
    kubectl apply -f deploy/k8s/monitoring/prometheus.yaml
    kubectl apply -f deploy/k8s/monitoring/grafana.yaml
    kubectl apply -f deploy/k8s/monitoring/servicemonitors.yaml
}

function Deploy-HA {
    Write-ColorOutput "[INFO] Deploying high availability configurations..." -ForegroundColor Green
    kubectl apply -f deploy/k8s/high-availability/
}

function Deploy-Ingress {
    Write-ColorOutput "[INFO] Deploying Ingress..." -ForegroundColor Green
    kubectl apply -f deploy/k8s/ingress.yaml
}

function Show-DeploymentSummary {
    Write-ColorOutput "[INFO] Verifying deployment..." -ForegroundColor Green

    Write-Host ""
    Write-ColorOutput "[INFO] Pods status:" -ForegroundColor Green
    kubectl get pods -n $Namespace

    Write-Host ""
    Write-ColorOutput "[INFO] Services status:" -ForegroundColor Green
    kubectl get svc -n $Namespace

    Write-Host ""
    Write-ColorOutput "[INFO] Deployment summary:" -ForegroundColor Green
    Write-Host "  Namespace: $Namespace"
    Write-Host "  API Gateway: http://api-gateway.${Namespace}.svc.cluster.local"
    Write-Host "  Grafana: http://grafana.ai-platform.example.com"
    Write-Host ""
    Write-ColorOutput "[WARN] Update your DNS / Ingress to access the services externally." -ForegroundColor Yellow
}

# Main script
Write-ColorOutput "[INFO] Starting AI Platform installation..." -ForegroundColor Green
Write-Host ""

Test-Prerequisites
New-Namespace
New-Secrets
Deploy-CoreServices
Deploy-SupportingServices
Deploy-HA
Deploy-Ingress
Show-DeploymentSummary

Write-Host ""
Write-ColorOutput "[INFO] Installation complete!" -ForegroundColor Green
Write-ColorOutput "[INFO] Run 'kubectl get pods -n ${Namespace}' to check pod status." -ForegroundColor Green
