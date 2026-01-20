package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yijian/ai-platform/internal/auth"
	"github.com/yijian/ai-platform/internal/ratelimit"
	"github.com/yijian/ai-platform/pkg/api/gateway"
	"github.com/yijian/ai-platform/pkg/metrics/prometheus"
	"github.com/yijian/ai-platform/pkg/model"
	"github.com/yijian/ai-platform/pkg/provider"
)

func main() {
	cfg := loadConfig()

	// 初始化Prometheus指标
	metricsRegistry := prometheus.NewRegistry()

	// 初始化Provider工厂
	providerFactory := provider.NewFactory()

	// 注意：这是简化实现，仅用于本地开发测试
	//
	// 生产环境正确的流程：
	// 1. api-gateway 接收请求后转发到 router-engine 服务
	// 2. router-engine 根据配置选择合适的 provider
	// 3. router-engine 调用 key-manager 服务获取密钥
	// 4. router-engine 使用密钥调用第三方 API
	//
	// 当前简化实现：直接从环境变量读取密钥（不安全，仅供测试）
	// 实际使用时应通过 Key Manager 的 API 接口动态配置密钥：
	// POST http://localhost:8002/api/v1/keys
	providerFactory.SetKey("openai", os.Getenv("OPENAI_API_KEY"))
	providerFactory.SetKey("aliyun", os.Getenv("ALIYUN_API_KEY"))

	// 初始化认证器
	authenticator := auth.NewJWTAuth(cfg.JWTSecret, cfg.JWTExpire)

	// 初始化限流器
	rateLimiter := ratelimit.NewRedisLimiter(cfg.RedisAddr, cfg.RedisPassword)

	// 初始化路由器（简化版，实际应调用router-engine服务）
	router := NewSimpleRouter(providerFactory, metricsRegistry)

	// 初始化网关处理器
	gatewayHandler := gateway.NewHandler(router, authenticator, rateLimiter)

	// 初始化Gin
	if cfg.GinMode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(gin.Logger())
	r.Use(corsMiddleware())
	r.Use(requestIDMiddleware())

	// 健康检查
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": "api-gateway",
		})
	})

	r.GET("/ready", func(c *gin.Context) {
		// TODO: 检查后端服务
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	// Metrics端点
	r.GET("/metrics", gin.WrapH(metricsRegistry.Handler()))

	// API路由
	v1 := r.Group("/api/v1")
	{
		gatewayHandler.RegisterRoutes(v1)
	}

	// 服务代理（用于转发到内部服务）
	setupProxy(r, cfg)

	srv := &http.Server{
		Addr:    ":8080",
		Handler: r,
	}

	go func() {
		log.Printf("Starting api-gateway on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down api-gateway...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Api-gateway exited")
}

type Config struct {
	LogLevel          string
	GinMode           string
	JWTSecret         string
	JWTExpire         time.Duration
	RedisAddr         string
	RedisPassword     string
	ConfigCenterAddr  string
	RegistryAddr      string
	KeyManagerAddr    string
	RouterEngineAddr  string
}

func loadConfig() *Config {
	expire := getEnv("JWT_EXPIRE", "24h")
	expireDuration, _ := time.ParseDuration(expire)

	return &Config{
		LogLevel:         getEnv("LOG_LEVEL", "info"),
		GinMode:          getEnv("GIN_MODE", "debug"),
		JWTSecret:        getEnv("JWT_SECRET", "your-secret-key"),
		JWTExpire:        expireDuration,
		RedisAddr:        getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:    getEnv("REDIS_PASSWORD", ""),
		ConfigCenterAddr: getEnv("CONFIG_CENTER_ADDR", "config-center:80"),
		RegistryAddr:     getEnv("REGISTRY_ADDR", "service-registry:80"),
		KeyManagerAddr:   getEnv("KEY_MANAGER_ADDR", "key-manager:80"),
		RouterEngineAddr: getEnv("ROUTER_ENGINE_ADDR", "router-engine:80"),
	}
}

// SimpleRouter 简单路由器实现
//
// 注意：这是简化实现，仅供本地开发测试使用
// 生产环境应使用 router-engine 服务进行路由决策
type SimpleRouter struct {
	providerFactory *provider.Factory
	metricsRegistry *prometheus.Registry
}

func NewSimpleRouter(factory *provider.Factory, metrics *prometheus.Registry) *SimpleRouter {
	return &SimpleRouter{
		providerFactory: factory,
		metricsRegistry: metrics,
	}
}

func (r *SimpleRouter) Route(ctx context.Context, feature string, params map[string]interface{}) (*model.InferenceResponse, error) {
	// 简化实现：直接调用第三方 API (OpenAI)
	//
	// 生产环境流程：
	// 1. API Gateway 调用 Router Engine 的 /api/v1/route/:feature 接口
	// 2. Router Engine 根据配置选择 self_hosted 或 third_party
	// 3. Router Engine 从 Key Manager 获取密钥
	// 4. Router Engine 调用对应的服务并返回结果
	startTime := time.Now()

	// 记录请求开始
	if r.metricsRegistry != nil {
		r.metricsRegistry.IncrementInFlight(feature)
		defer r.metricsRegistry.DecrementInFlight(feature)
	}

	client, err := r.providerFactory.Create("openai")
	if err != nil {
		if r.metricsRegistry != nil {
			duration := time.Since(startTime).Seconds()
			r.metricsRegistry.RecordRequest(feature, "third_party", "openai", "error", duration)
		}
		return nil, err
	}
	defer client.Close()

	resp := &model.InferenceResponse{
		RequestID:     generateRequestID(),
		Feature:       feature,
		ProviderType:  "third_party",
		ProviderID:    "openai",
		ReceivedAt:    startTime,
	}

	var result *model.InferenceResponse
	switch feature {
	case "text_to_image":
		result, err = r.generateImage(ctx, resp, client, params)
	case "text_generation":
		result, err = r.generateText(ctx, resp, client, params)
	default:
		err = fmt.Errorf("unsupported feature: %s", feature)
	}

	// 记录请求完成
	if r.metricsRegistry != nil {
		duration := time.Since(startTime).Seconds()
		status := "success"
		if err != nil {
			status = "error"
		}
		r.metricsRegistry.RecordRequest(feature, "third_party", "openai", status, duration)
	}

	return result, err
}

func (r *SimpleRouter) generateImage(ctx context.Context, resp *model.InferenceResponse, client provider.LLMProvider, params map[string]interface{}) (*model.InferenceResponse, error) {
	req := &provider.ImageRequest{
		Prompt:    getString(params, "prompt"),
		Width:     getInt(params, "width", 1024),
		Height:    getInt(params, "height", 1024),
		Count:     getInt(params, "count", 1),
	}

	imageResp, err := client.GenerateImage(ctx, req)
	if err != nil {
		return nil, err
	}

	resp.CompletedAt = time.Now()
	resp.ExecTimeMs = int(time.Since(resp.ReceivedAt).Milliseconds())
	resp.Status = "success"
	resp.Result = map[string]interface{}{"images": imageResp.Images}
	resp.ImageCount = len(imageResp.Images)

	return resp, nil
}

func (r *SimpleRouter) generateText(ctx context.Context, resp *model.InferenceResponse, client provider.LLMProvider, params map[string]interface{}) (*model.InferenceResponse, error) {
	req := &provider.TextRequest{
		Prompt:      getString(params, "prompt"),
		MaxTokens:   getInt(params, "max_tokens", 1000),
		Temperature: getFloat64(params, "temperature", 0.7),
	}

	textResp, err := client.GenerateText(ctx, req)
	if err != nil {
		return nil, err
	}

	resp.CompletedAt = time.Now()
	resp.ExecTimeMs = int(time.Since(resp.ReceivedAt).Milliseconds())
	resp.Status = "success"
	resp.Result = map[string]interface{}{"text": textResp.Text}
	resp.TokensInput = textResp.TokensInput
	resp.TokensOutput = textResp.TokensOutput

	return resp, nil
}

func setupProxy(r *gin.Engine, cfg *Config) {
	// 代理到内部服务
	services := []struct {
		path   string
		target string
	}{
		{"/internal/config", "http://" + cfg.ConfigCenterAddr},
		{"/internal/registry", "http://" + cfg.RegistryAddr},
		{"/internal/keys", "http://" + cfg.KeyManagerAddr},
		{"/internal/router", "http://" + cfg.RouterEngineAddr},
	}

	for _, svc := range services {
		target, _ := url.Parse(svc.target)
		proxy := httputil.NewSingleHostReverseProxy(target)
		r.Any(svc.path+"/*path", func(c *gin.Context) {
			c.Request.URL.Path = strings.TrimPrefix(c.Request.URL.Path, svc.path)
			proxy.ServeHTTP(c.Writer, c.Request)
		})
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getInt(m map[string]interface{}, key string, defaultVal int) int {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case int:
			return val
		case float64:
			return int(val)
		}
	}
	return defaultVal
}

func getFloat64(m map[string]interface{}, key string, defaultVal float64) float64 {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case float64:
			return val
		case float32:
			return float64(val)
		}
	}
	return defaultVal
}

func generateRequestID() string {
	return fmt.Sprintf("req-%d", time.Now().UnixNano())
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization, X-Request-ID")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func requestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = c.GetHeader("X-Trace-ID")
		}
		if requestID == "" {
			requestID = generateRequestID()
		}
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}
