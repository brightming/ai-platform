# AI Platform Deployment Guide

## Prerequisites

1. **Alibaba Cloud ACK Cluster** (Kubernetes 1.24+)
2. **MySQL Database** (RDS for MySQL 8.0)
3. **Alibaba Cloud KMS** (for API key encryption)
4. **Container Registry** (ACR or private registry)
5. **kubectl** configured for your cluster

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                         External Traffic                         │
└─────────────────────────────────────────────────────────────────┘
                                  │
                         ┌────────▼────────┐
                         │   API Gateway   │ (Nginx Ingress / ALB)
                         │   (3 replicas)  │
                         └────────┬────────┘
                                  │
        ┌─────────────────────────┼─────────────────────────┐
        │                         │                         │
┌───────▼────────┐      ┌────────▼────────┐      ┌────────▼────────┐
│ Router Engine  │      │ Config Center   │      │ Key Manager     │
│ (3 replicas)   │─────▶│ (2 replicas)    │─────▶│ (2 replicas)    │
└────────────────┘      └─────────────────┘      └─────────────────┘
        │                         │
┌───────▼────────┐      ┌────────▼────────┐
│ Service        │      │ Service         │
│ Registry       │─────▶│ Registry (DB)   │
│ (2 replicas)   │      └─────────────────┘
└────────────────┘
        │
┌───────▼────────┐      ┌─────────────────────────────────────┐
│ Scaler         │      │          Self-hosted Services        │
│ (1 replica)    │─────▶│  (Text-to-Image, Image Editing...)  │
└────────────────┘      └─────────────────────────────────────┘
```

## Deployment Steps

### 1. Prepare Database

```bash
# Create MySQL database
mysql -h rm-xxx.mysql.rds.aliyuncs.com -u root -p
```

```sql
CREATE DATABASE ai_platform CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER 'ai_platform'@'%' IDENTIFIED BY 'your-password';
GRANT ALL PRIVILEGES ON ai_platform.* TO 'ai_platform'@'%';
FLUSH PRIVILEGES;
```

Apply the schema:

```bash
kubectl exec -it -n ai-platform deployment/mysql -- mysql -u root -p < deploy/mysql/init.sql
```

### 2. Create Namespace

```bash
kubectl apply -f deploy/k8s/base/namespace.yaml
```

### 3. Configure Secrets

Edit the secrets with your actual values:

```bash
# Edit database secret
kubectl edit secret config-center-secret -n ai-platform

# Edit KMS configuration
kubectl edit configmap key-manager-config -n ai-platform
```

### 4. Deploy Core Services

```bash
# Deploy in order
kubectl apply -f deploy/k8s/config-center/
kubectl apply -f deploy/k8s/key-manager/
kubectl apply -f deploy/k8s/service-registry/
kubectl apply -f deploy/k8s/router-engine/
kubectl apply -f deploy/k8s/api-gateway/
```

### 5. Deploy Supporting Services

```bash
# Redis for rate limiting
kubectl apply -f deploy/k8s/redis/

# Scaler
kubectl apply -f deploy/k8s/scaler/

# Monitoring
kubectl apply -f deploy/k8s/monitoring/
```

### 6. Configure Ingress

```bash
# Update the Ingress with your domain
# Edit deploy/k8s/ingress.yaml and update:
# - api.ai-platform.example.com
# - ${SSL_CERTIFICATE_ARN}
# - ${SECURITY_GROUP_ID}

kubectl apply -f deploy/k8s/ingress.yaml
```

### 7. Configure High Availability

```bash
# PodDisruptionBudgets
kubectl apply -f deploy/k8s/high-availability/
```

### 8. Configure Network Policies (Optional)

```bash
# For production, enable network policies
kubectl apply -f deploy/k8s/security/networkpolicies.yaml
```

## Verification

```bash
# Check all pods are running
kubectl get pods -n ai-platform

# Check services
kubectl get svc -n ai-platform

# Port forward to test locally
kubectl port-forward -n ai-platform svc/api-gateway 8080:80

# Test health endpoint
curl http://localhost:8080/healthz
```

## Monitoring

Access Grafana at: `http://grafana.ai-platform.example.com`
- Default credentials: admin/admin123

## Scaling

### Manual Scaling

```bash
# Scale API Gateway
kubectl scale deployment api-gateway -n ai-platform --replicas=5

# Scale Router Engine
kubectl scale deployment router-engine -n ai-platform --replicas=5
```

### Auto-scaling (HPA)

HPA is configured for:
- config-center: 2-5 replicas
- api-gateway: 3-10 replicas

## Troubleshooting

### View Logs

```bash
# API Gateway logs
kubectl logs -f -n ai-platform deployment/api-gateway

# Router Engine logs
kubectl logs -f -n ai-platform deployment/router-engine

# All pods logs
kubectl logs -f -n ai-platform -l app=api-gateway --all-containers=true
```

### Debug

```bash
# Exec into a pod
kubectl exec -it -n ai-platform deployment/api-gateway -- /bin/sh

# Describe pod
kubectl describe pod -n ai-platform <pod-name>

# Check events
kubectl get events -n ai-platform --sort-by='.lastTimestamp'
```

## Upgrading

```bash
# Update images
kubectl set image deployment/api-gateway -n ai-platform api-gateway=registry.com/ai/api-gateway:v1.1.0

# Rollback if needed
kubectl rollout undo deployment/api-gateway -n ai-platform

# Check rollout status
kubectl rollout status deployment/api-gateway -n ai-platform
```

## Cleanup

```bash
# Delete all resources
kubectl delete namespace ai-platform
```

## Configuration Reference

### Environment Variables

| Service | Variable | Description | Default |
|---------|----------|-------------|---------|
| config-center | DB_HOST | MySQL host | localhost |
| config-center | DB_PORT | MySQL port | 3306 |
| key-manager | KMS_REGION_ID | Aliyun KMS region | cn-hangzhou |
| router-engine | ROUTING_STRATEGY | weighted/priority/cost_based | weighted |
| api-gateway | JWT_SECRET | JWT signing secret | - |
| scaler | SCALE_CHECK_INTERVAL | Scale check interval (seconds) | 30 |

### Resource Limits

| Service | CPU Request | CPU Limit | Memory Request | Memory Limit |
|---------|-------------|-----------|----------------|--------------|
| config-center | 200m | 1000m | 256Mi | 1Gi |
| key-manager | 200m | 1000m | 256Mi | 1Gi |
| service-registry | 200m | 1000m | 256Mi | 1Gi |
| router-engine | 300m | 1500m | 384Mi | 1Gi |
| api-gateway | 500m | 2000m | 512Mi | 2Gi |
| scaler | 100m | 500m | 128Mi | 512Mi |
