#!/bin/bash
# AI Platform Build Script
# Builds Docker images for all services

set -e

# Configuration
REGISTRY="${REGISTRY:-registry.com/ai}"
VERSION="${VERSION:-v1.0.0}"
SERVICES=("config-center" "key-manager" "service-registry" "router-engine" "api-gateway" "agent")

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to build a service
build_service() {
    local service=$1
    local dockerfile="${2:-Dockerfile}"

    echo -e "${YELLOW}Building ${service}...${NC}"

    if [ "$service" = "agent" ]; then
        docker build \
            -f build/Dockerfile.agent \
            -t ${REGISTRY}/${service}:${VERSION} \
            -t ${REGISTRY}/${service}:latest \
            .
    else
        docker build \
            --build-arg SERVICE=${service} \
            --build-arg VERSION=${VERSION} \
            -f build/Dockerfile \
            -t ${REGISTRY}/${service}:${VERSION} \
            -t ${REGISTRY}/${service}:latest \
            .
    fi

    if [ $? -eq 0 ]; then
        echo -e "${GREEN}Successfully built ${service}${NC}"
    else
        echo -e "${RED}Failed to build ${service}${NC}"
        exit 1
    fi
}

# Function to push a service
push_service() {
    local service=$1

    echo -e "${YELLOW}Pushing ${service}...${NC}"
    docker push ${REGISTRY}/${service}:${VERSION}
    docker push ${REGISTRY}/${service}:latest

    if [ $? -eq 0 ]; then
        echo -e "${GREEN}Successfully pushed ${service}${NC}"
    else
        echo -e "${RED}Failed to push ${service}${NC}"
        exit 1
    fi
}

# Main script
case "${1:-build}" in
    build)
        for service in "${SERVICES[@]}"; do
            build_service "$service"
        done
        echo -e "${GREEN}All services built successfully!${NC}"
        ;;
    push)
        for service in "${SERVICES[@]}"; do
            push_service "$service"
        done
        echo -e "${GREEN}All services pushed successfully!${NC}"
        ;;
    build-push)
        for service in "${SERVICES[@]}"; do
            build_service "$service"
            push_service "$service"
        done
        echo -e "${GREEN}All services built and pushed successfully!${NC}"
        ;;
    *)
        service=$1
        if [[ " ${SERVICES[@]} " =~ " ${service} " ]]; then
            build_service "$service"
        else
            echo "Usage: $0 [build|push|build-push|<service-name>]"
            echo "Services: ${SERVICES[@]}"
            echo ""
            echo "Environment variables:"
            echo "  REGISTRY - Docker registry (default: registry.com/ai)"
            echo "  VERSION  - Image version tag (default: v1.0.0)"
            exit 1
        fi
        ;;
esac
