package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/brightming/ai-platform/internal/scaler"
	"github.com/brightming/ai-platform/pkg/model"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var (
	Version   = "v1.0.0"
	BuildTime = "unknown"
)

func main() {
	log.Printf("[Scaler] Starting AI Platform Scaler %s (build: %s)", Version, BuildTime)

	// Initialize database
	db, err := initDB()
	if err != nil {
		log.Fatalf("[Scaler] Failed to initialize database: %v", err)
	}

	// Create config store
	configStore := &DBConfigStore{db: db}

	// Create service registry client
	registry := &RegistryClient{}

	// Create scaler controller
	controller, err := scaler.NewController(configStore, registry)
	if err != nil {
		log.Fatalf("[Scaler] Failed to create controller: %v", err)
	}

	// Start HTTP server
	go serveHTTP(controller)

	// Keep running
	select {}
}

func initDB() (*gorm.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		getEnv("DB_USER", "root"),
		getEnv("DB_PASSWORD", ""),
		getEnv("DB_HOST", "localhost"),
		getEnv("DB_PORT", "3306"),
		getEnv("DB_NAME", "ai_platform"),
	)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	return db, nil
}

func serveHTTP(controller *scaler.Controller) {
	gin.SetMode(getEnv("GIN_MODE", "release"))
	r := gin.New()
	r.Use(gin.Recovery(), gin.Logger())

	// Health check
	r.GET("/healthz", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	r.GET("/ready", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Metrics endpoint
	r.GET("/metrics", func(c *gin.Context) {
		// Return basic metrics
		c.String(http.StatusOK, "# Scaler metrics\n# Version: %s\n", Version)
	})

	// API routes
	api := r.Group("/api/v1")
	{
		// Scale configuration
		api.GET("/scale-config/:feature_id", func(c *gin.Context) {
			config, err := controller.GetScaleConfig(c.Param("feature_id"))
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, config)
		})

		api.PUT("/scale-config/:feature_id", func(c *gin.Context) {
			var config scaler.ScaleConfig
			if err := c.BindJSON(&config); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			config.FeatureID = c.Param("feature_id")
			if err := controller.UpdateScaleConfig(&config); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, config)
		})

		// Manual scale operations
		api.POST("/scale/:feature_id/up", func(c *gin.Context) {
			var count int32 = 1
			if c.Query("count") != "" {
				n, _ := strconv.ParseInt(c.Query("count"), 10, 32)
				count = int32(n)
			}
			if err := controller.ScaleUp(c.Param("feature_id"), count); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"status": "scaled up"})
		})

		api.POST("/scale/:feature_id/down", func(c *gin.Context) {
			if err := controller.ScaleToZero(c.Param("feature_id")); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"status": "scaled down"})
		})

		api.POST("/scale/:feature_id/check", func(c *gin.Context) {
			decision, err := controller.CheckScale(context.Background(), c.Param("feature_id"))
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, decision)
		})

		// Watch scale events
		api.GET("/events", func(c *gin.Context) {
			ctx := c.Request.Context()
			events := controller.WatchScaleEvents(ctx)
			c.Stream(func(w gin.Writer) bool {
				select {
				case <-ctx.Done():
					return false
				case event, ok := <-events:
					if !ok {
						return false
					}
					c.SSEvent("scale-event", event)
					return true
				}
			})
		})
	}

	port := getEnv("PORT", "8080")
	log.Printf("[Scaler] HTTP server listening on :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("[Scaler] Failed to start HTTP server: %v", err)
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// DBConfigStore implements ConfigStore
type DBConfigStore struct {
	db *gorm.DB
}

func (s *DBConfigStore) GetFeature(id string) (*model.Feature, error) {
	var feature model.Feature
	err := s.db.Where("id = ?", id).First(&feature).Error
	if err != nil {
		return nil, err
	}
	return &feature, nil
}

// RegistryClient implements ServiceRegistry
type RegistryClient struct{}

func (r *RegistryClient) GetServicesByType(serviceType string) ([]*model.RegisteredService, error) {
	// TODO: Implement actual service registry query
	return []*model.RegisteredService{}, nil
}
