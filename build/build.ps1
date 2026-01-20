# AI Platform Build Script for Windows PowerShell
# Builds Docker images for all services

param(
    [Parameter(Position=0)]
    [ValidateSet("build", "push", "build-push", "config-center", "key-manager", "service-registry", "router-engine", "api-gateway", "agent")]
    [string]$Action = "build",

    [string]$Registry = "registry.com/ai",
    [string]$Version = "v1.0.0"
)

$Services = @("config-center", "key-manager", "service-registry", "router-engine", "api-gateway", "agent")

function Build-Service {
    param(
        [string]$Service
    )

    Write-Host "Building $Service..." -ForegroundColor Yellow

    if ($Service -eq "agent") {
        docker build `
            -f build/Dockerfile.agent `
            -t "${Registry}/${Service}:${Version}" `
            -t "${Registry}/${Service}:latest" `
            .
    } else {
        docker build `
            --build-arg "SERVICE=${Service}" `
            --build-arg "VERSION=${Version}" `
            -f build/Dockerfile `
            -t "${Registry}/${Service}:${Version}" `
            -t "${Registry}/${Service}:latest" `
            .
    }

    if ($LASTEXITCODE -eq 0) {
        Write-Host "Successfully built $Service" -ForegroundColor Green
    } else {
        Write-Host "Failed to build $Service" -ForegroundColor Red
        exit 1
    }
}

function Push-Service {
    param(
        [string]$Service
    )

    Write-Host "Pushing $Service..." -ForegroundColor Yellow
    docker push "${Registry}/${Service}:${Version}"
    docker push "${Registry}/${Service}:latest"

    if ($LASTEXITCODE -eq 0) {
        Write-Host "Successfully pushed $Service" -ForegroundColor Green
    } else {
        Write-Host "Failed to push $Service" -ForegroundColor Red
        exit 1
    }
}

switch ($Action) {
    "build" {
        foreach ($service in $Services) {
            Build-Service -Service $service
        }
        Write-Host "All services built successfully!" -ForegroundColor Green
    }
    "push" {
        foreach ($service in $Services) {
            Push-Service -Service $service
        }
        Write-Host "All services pushed successfully!" -ForegroundColor Green
    }
    "build-push" {
        foreach ($service in $Services) {
            Build-Service -Service $service
            Push-Service -Service $service
        }
        Write-Host "All services built and pushed successfully!" -ForegroundColor Green
    }
    default {
        Build-Service -Service $Action
    }
}
