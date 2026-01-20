# AI Platform Services Dockerfile
# Build arg to specify which service to build
ARG SERVICE=config-center
ARG VERSION=v1.0.0

# Build stage
FROM golang:1.21-alpine AS builder

# Declare build args (need to re-declare after FROM)
ARG SERVICE
ARG VERSION

# Install build dependencies
# Use Alpine mirror for China region
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories && \
    apk add --no-cache git make gcc musl-dev

# Set working directory
WORKDIR /app

# Set environment variables for Go module handling
ENV GOPROXY=https://goproxy.cn,direct
ENV GOSUMDB=off

# Copy all source code
COPY . .

# Initialize git repo for local module resolution
RUN git init && \
    git config user.email "build@localhost" && \
    git config user.name "Build" && \
    git add -A && \
    git commit -m "init" && \
    git remote add origin https://github.com/yijian/ai-platform.git

# Generate go.sum with go mod tidy
RUN go mod tidy

# Download external dependencies
RUN go mod download

# Build the specified service
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -v \
    -ldflags="-X main.Version=${VERSION} -s -w" \
    -o /bin/${SERVICE} \
    ./cmd/${SERVICE}

# Verify the binary
RUN ls -lh /bin/${SERVICE} && file /bin/${SERVICE}

# Runtime stage
FROM alpine:3.19

# Install runtime dependencies
# Use Alpine mirror for China region
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories && \
    apk add --no-cache \
    ca-certificates \
    tzdata \
    curl

# Create a non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

# Set working directory
WORKDIR /app

# Copy binary from builder
ARG SERVICE
COPY --from=builder /bin/${SERVICE} /app/${SERVICE}

# Change ownership
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/healthz || exit 1

# Run the service
ENTRYPOINT ["/app/${SERVICE}"]
