package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yijian/ai-platform/internal/config"
	"github.com/yijian/ai-platform/internal/registry"
	"github.com/yijian/ai-platform/internal/router"
	"github.com/yijian/ai-platform/pkg/metrics/prometheus"
	"github.com/yijian/ai-platform/pkg/model"
	"github.com/yijian/ai-platform/pkg/provider"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	cfg := loadConfig()

	// 初始化Prometheus指标
	metricsRegistry := prometheus.NewRegistry()

	db, err := initDB(cfg)
	if err != nil {
		log.Fatalf("Failed to init database: %v", err)
	}

	// 初始化依赖
	configStore := &configStoreImpl{db: db}
	serviceRegistry := &serviceRegistryImpl{db: db}
	keyManager := &keyManagerImpl{}
	providerFactory := provider.NewFactory()
	costTracker := &costTrackerImpl{}

	// 初始化路由引擎
	routerEngine := router.NewEngine(configStore, serviceRegistry, keyManager, providerFactory, costTracker)

	// 初始化Gin
	if cfg.GinMode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(gin.Logger())
	r.Use(corsMiddleware())

	// 健康检查
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": "router-engine",
		})
	})

	r.GET("/ready", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	// Metrics端点
	r.GET("/metrics", gin.WrapH(metricsRegistry.Handler()))

	// 路由API
	r.POST("/api/v1/route/:feature", func(c *gin.Context) {
		feature := c.Param("feature")
		var params map[string]interface{}
		if err := c.ShouldBindJSON(&params); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		resp, err := routerEngine.Route(c.Request.Context(), feature, params)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, resp)
	})

	srv := &http.Server{
		Addr:    ":8080",
		Handler: r,
	}

	go func() {
		log.Printf("Starting router-engine on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down router-engine...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Router-engine exited")
}

type Config struct {
	LogLevel         string
	GinMode          string
	DB               DBConfig
	ConfigCenterAddr string
	RegistryAddr     string
	KeyManagerAddr   string
	RoutingStrategy  string
	FallbackEnabled  bool
}

type DBConfig struct {
	Host     string
	Port     string
	Name     string
	User     string
	Password string
}

func loadConfig() *Config {
	return &Config{
		LogLevel:         getEnv("LOG_LEVEL", "info"),
		GinMode:          getEnv("GIN_MODE", "debug"),
		ConfigCenterAddr: getEnv("CONFIG_CENTER_ADDR", "config-center:80"),
		RegistryAddr:     getEnv("REGISTRY_ADDR", "service-registry:80"),
		KeyManagerAddr:   getEnv("KEY_MANAGER_ADDR", "key-manager:80"),
		RoutingStrategy:  getEnv("ROUTING_STRATEGY", "weighted"),
		FallbackEnabled:  getEnvBool("FALLBACK_ENABLED", true),
		DB: DBConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "3306"),
			Name:     getEnv("DB_NAME", "ai_platform"),
			User:     getEnv("DB_USER", "root"),
			Password: getEnv("DB_PASSWORD", ""),
		},
	}
}

func initDB(cfg *Config) (*gorm.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.DB.User, cfg.DB.Password, cfg.DB.Host, cfg.DB.Port, cfg.DB.Name)
	return gorm.Open(mysql.Open(dsn), &gorm.Config{})
}

// 实现接口
type configStoreImpl struct {
	db *gorm.DB
}

func (c *configStoreImpl) GetFeature(id string) (*model.Feature, error) {
	var feature model.Feature
	err := c.db.Where("id = ? AND enabled = ?", id, true).First(&feature).Error
	if err != nil {
		return nil, err
	}
	err = c.db.Where("feature_id = ?", id).Find(&feature.Providers).Error
	return &feature, err
}

func (c *configStoreImpl) GetFeatureByCategory(category string) ([]*model.Feature, error) {
	var features []*model.Feature
	err := c.db.Where("category = ? AND enabled = ?", category, true).Find(&features).Error
	if err != nil {
		return nil, err
	}
	for _, f := range features {
		c.db.Where("feature_id = ?", f.ID).Find(&f.Providers)
	}
	return features, nil
}

type serviceRegistryImpl struct {
	db *gorm.DB
}

func (s *serviceRegistryImpl) GetHealthyServices(serviceType string) ([]*model.RegisteredService, error) {
	var services []*model.RegisteredService
	err := s.db.Where("service_type = ? AND status IN ?", serviceType, []string{"healthy", "degraded"}).
		Find(&services).Error
	return services, err
}

type keyManagerImpl struct{}

func (k *keyManagerImpl) GetActiveKey(vendor, svc string) (*model.APIKey, error) {
	// TODO: 调用key-manager服务
	return &model.APIKey{ID: "key-" + vendor}, nil
}

func (k *keyManagerImpl) GetPlaintextKey(key *model.APIKey) (string, error) {
	// TODO: 调用key-manager服务
	return "sk-test", nil
}

func (k *keyManagerImpl) RecordUsage(record *router.KeyUsageRecord) error {
	// TODO: 记录使用
	return nil
}

type costTrackerImpl struct{}

func (c *costTrackerImpl) RecordCost(requestID string, cost float64) error {
	// TODO: 记录成本
	return nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return value == "true"
	}
	return defaultValue
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
