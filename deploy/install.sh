#!/bin/bash
# AI Platform Installation Script

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Configuration
NAMESPACE="ai-platform"
REGISTRY="${REGISTRY:-registry.com/ai}"
VERSION="${VERSION:-v1.0.0}"

# Functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

check_prereqs() {
    log_info "Checking prerequisites..."

    # Check kubectl
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl not found. Please install kubectl."
        exit 1
    fi

    # Check kubectl context
    if ! kubectl cluster-info &> /dev/null; then
        log_error "Cannot connect to Kubernetes cluster. Please configure kubectl."
        exit 1
    fi

    log_info "Prerequisites check passed!"
}

create_namespace() {
    log_info "Creating namespace ${NAMESPACE}..."
    kubectl apply -f deploy/k8s/base/namespace.yaml
}

create_secrets() {
    log_info "Creating secrets..."
    log_warn "Please update the following secrets with your actual values:"
    log_warn "  - config-center-secret: Database credentials"
    log_warn "  - key-manager-rrsa: RRSA role name"
    log_warn "  - api-gateway-secret: JWT secret"

    # Prompt for database password
    read -sp "Enter MySQL password: " DB_PASSWORD
    echo

    # Create database secret
    kubectl create secret generic config-center-secret \
        --from-literal=DB_HOST="${DB_HOST:-rm-xxx.mysql.rds.aliyuncs.com}" \
        --from-literal=DB_PORT="${DB_PORT:-3306}" \
        --from-literal=DB_NAME="${DB_NAME:-ai_platform}" \
        --from-literal=DB_USER="${DB_USER:-ai_platform}" \
        --from-literal=DB_PASSWORD="${DB_PASSWORD}" \
        -n ${NAMESPACE} --dry-run=client -o yaml | kubectl apply -f -

    # Create JWT secret
    JWT_SECRET=$(openssl rand -base64 32)
    kubectl create secret generic api-gateway-secret \
        --from-literal=JWT_SECRET="${JWT_SECRET}" \
        -n ${NAMESPACE} --dry-run=client -o yaml | kubectl apply -f -
}

deploy_core_services() {
    log_info "Deploying core services..."

    # Deploy in order
    log_info "Deploying Config Center..."
    kubectl apply -f deploy/k8s/config-center/

    log_info "Deploying Key Manager..."
    kubectl apply -f deploy/k8s/key-manager/

    log_info "Deploying Service Registry..."
    kubectl apply -f deploy/k8s/service-registry/

    log_info "Deploying Router Engine..."
    kubectl apply -f deploy/k8s/router-engine/

    log_info "Deploying API Gateway..."
    kubectl apply -f deploy/k8s/api-gateway/

    # Wait for pods to be ready
    log_info "Waiting for pods to be ready..."
    kubectl wait --for=condition=ready pod -l app=config-center -n ${NAMESPACE} --timeout=300s
    kubectl wait --for=condition=ready pod -l app=key-manager -n ${NAMESPACE} --timeout=300s
    kubectl wait --for=condition=ready pod -l app=service-registry -n ${NAMESPACE} --timeout=300s
    kubectl wait --for=condition=ready pod -l app=router-engine -n ${NAMESPACE} --timeout=300s
    kubectl wait --for=condition=ready pod -l app=api-gateway -n ${NAMESPACE} --timeout=300s
}

deploy_supporting_services() {
    log_info "Deploying supporting services..."

    log_info "Deploying Redis..."
    kubectl apply -f deploy/k8s/redis/

    log_info "Deploying Scaler..."
    kubectl apply -f deploy/k8s/scaler/

    log_info "Deploying Monitoring (Prometheus, Grafana)..."
    kubectl apply -f deploy/k8s/monitoring/prometheus.yaml
    kubectl apply -f deploy/k8s/monitoring/grafana.yaml
    kubectl apply -f deploy/k8s/monitoring/servicemonitors.yaml
}

deploy_ha() {
    log_info "Deploying high availability configurations..."
    kubectl apply -f deploy/k8s/high-availability/
}

deploy_ingress() {
    log_info "Deploying Ingress..."
    kubectl apply -f deploy/k8s/ingress.yaml
}

verify_deployment() {
    log_info "Verifying deployment..."

    echo ""
    log_info "Pods status:"
    kubectl get pods -n ${NAMESPACE}

    echo ""
    log_info "Services status:"
    kubectl get svc -n ${NAMESPACE}

    echo ""
    log_info "Deployment summary:"
    echo "  Namespace: ${NAMESPACE}"
    echo "  API Gateway: http://api-gateway.${NAMESPACE}.svc.cluster.local"
    echo "  Grafana: http://grafana.ai-platform.example.com"
    echo ""
    log_warn "Update your DNS / Ingress to access the services externally."
}

main() {
    log_info "Starting AI Platform installation..."
    echo ""

    check_prereqs
    create_namespace
    create_secrets
    deploy_core_services
    deploy_supporting_services
    deploy_ha
    deploy_ingress
    verify_deployment

    echo ""
    log_info "Installation complete!"
    log_info "Run 'kubectl get pods -n ${NAMESPACE}' to check pod status."
}

# Run main function
main "$@"
