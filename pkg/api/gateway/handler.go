package gateway

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/brightming/ai-platform/pkg/model"
)

// Handler API网关处理器
type Handler struct {
	router      Router
	auth        Authenticator
	rateLimiter RateLimiter
}

// Router 路由器接口
type Router interface {
	Route(ctx context.Context, feature string, params map[string]interface{}) (*model.InferenceResponse, error)
}

// Authenticator 认证器接口
type Authenticator interface {
	Authenticate(ctx context.Context, token string) (*AuthInfo, error)
}

// RateLimiter 限流器接口
type RateLimiter interface {
	Allow(ctx context.Context, tenantID, feature string) bool
}

// AuthInfo 认证信息
type AuthInfo struct {
	TenantID string
	UserID   string
	Roles    []string
}

// NewHandler 创建网关处理器
func NewHandler(router Router, auth Authenticator, rateLimiter RateLimiter) *Handler {
	return &Handler{
		router:      router,
		auth:        auth,
		rateLimiter: rateLimiter,
	}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	// 健康检查
	r.GET("/healthz", h.HealthCheck)
	r.GET("/ready", h.Ready)
	r.GET("/metrics", h.Metrics)

	// API v1
	api := r.Group("/api/v1")
	{
		// 特性配置管理
		api.GET("/features", h.ListFeatures)
		api.GET("/features/:id", h.GetFeature)
		api.POST("/features", h.CreateFeature)
		api.PUT("/features/:id", h.UpdateFeature)
		api.DELETE("/features/:id", h.DeleteFeature)

		// API Key 管理
		api.GET("/keys", h.ListKeys)
		api.GET("/keys/:id", h.GetKey)
		api.POST("/keys", h.CreateKey)
		api.PUT("/keys/:id", h.UpdateKey)
		api.DELETE("/keys/:id", h.DeleteKey)

		// 服务注册管理
		api.GET("/services", h.ListServices)
		api.GET("/services/:id", h.GetService)
		api.POST("/services/register", h.RegisterService)
		api.POST("/services/:id/heartbeat", h.Heartbeat)

		// 路由决策
		api.POST("/route/decide", h.RouteDecide)

		// 预算管理
		api.GET("/budget/check", h.CheckBudget)
		api.GET("/budget/:id", h.GetBudget)
		api.POST("/budget", h.CreateBudget)

		// 统计信息
		api.GET("/stats", h.GetStats)
	}

	// 文生图
	inference := r.Group("/inference")
	{
		inference.POST("/text-to-image", h.TextToImage)
		inference.POST("/image-edit", h.ImageEdit)
		inference.POST("/image-stylize", h.ImageStylize)
		inference.POST("/text-generation", h.TextGeneration)
	}
}

// TextToImage 文生图
// @Summary 文生图
// @Description 根据文本描述生成图像
// @Tags inference
// @Accept json
// @Produce json
// @Param request body model.TextToImageRequest true "请求参数"
// @Success 200 {object} model.InferenceResponse
// @Router /api/v1/inference/text-to-image [post]
func (h *Handler) TextToImage(c *gin.Context) {
	h.handleInference(c, "text_to_image", func() map[string]interface{} {
		var req model.TextToImageRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			return nil
		}
		return map[string]interface{}{
			"prompt":          req.Prompt,
			"negative_prompt": req.NegativePrompt,
			"width":           req.Width,
			"height":          req.Height,
			"steps":           req.Steps,
			"cfg_scale":       req.CFGScale,
			"seed":            req.Seed,
			"count":           req.Count,
		}
	})
}

// ImageEdit 图像编辑
// @Summary 图像编辑
// @Description 编辑图像
// @Tags inference
// @Accept json
// @Produce json
// @Param request body model.ImageEditRequest true "请求参数"
// @Success 200 {object} model.InferenceResponse
// @Router /api/v1/inference/image-edit [post]
func (h *Handler) ImageEdit(c *gin.Context) {
	h.handleInference(c, "image_editing", func() map[string]interface{} {
		var req model.ImageEditRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			return nil
		}
		return map[string]interface{}{
			"image":           req.Image,
			"mask":            req.Mask,
			"prompt":          req.Prompt,
			"negative_prompt": req.NegativePrompt,
			"width":           req.Width,
			"height":          req.Height,
			"steps":           req.Steps,
			"cfg_scale":       req.CFGScale,
		}
	})
}

// ImageStylize 图像风格化
// @Summary 图像风格化
// @Description 对图像进行风格化处理
// @Tags inference
// @Accept json
// @Produce json
// @Param request body model.ImageStylizationRequest true "请求参数"
// @Success 200 {object} model.InferenceResponse
// @Router /api/v1/inference/image-stylize [post]
func (h *Handler) ImageStylize(c *gin.Context) {
	h.handleInference(c, "image_stylization", func() map[string]interface{} {
		var req model.ImageStylizationRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			return nil
		}
		return map[string]interface{}{
			"image":    req.Image,
			"style":    req.Style,
			"strength": req.Strength,
		}
	})
}

// TextGeneration 文本生成
// @Summary 文本生成
// @Description 根据提示生成文本
// @Tags inference
// @Accept json
// @Produce json
// @Param request body model.TextGenerationRequest true "请求参数"
// @Success 200 {object} model.InferenceResponse
// @Router /api/v1/inference/text-generation [post]
func (h *Handler) TextGeneration(c *gin.Context) {
	h.handleInference(c, "text_generation", func() map[string]interface{} {
		var req model.TextGenerationRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			return nil
		}
		return map[string]interface{}{
			"prompt":      req.Prompt,
			"max_tokens":  req.MaxTokens,
			"temperature": req.Temperature,
			"top_p":       req.TopP,
			"top_k":       req.TopK,
			"stop":        req.Stop,
		}
	})
}

// handleInference 处理推理请求
func (h *Handler) handleInference(c *gin.Context, feature string, paramsFunc func() map[string]interface{}) {
	// 获取认证信息
	authInfo, err := h.authenticate(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Code:    1002,
			Message: "认证失败: " + err.Error(),
		})
		return
	}

	// 限流检查
	if h.rateLimiter != nil && !h.rateLimiter.Allow(c.Request.Context(), authInfo.TenantID, feature) {
		c.JSON(http.StatusTooManyRequests, ErrorResponse{
			Code:    1003,
			Message: "请求过于频繁，请稍后再试",
		})
		return
	}

	// 解析参数
	params := paramsFunc()
	if params == nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    1001,
			Message: "参数错误",
		})
		return
	}

	// 构建请求
	requestID := generateRequestID()
	traceID := getTraceID(c)

	req := &model.InferenceRequest{
		RequestID: requestID,
		Feature:   feature,
		TenantID:  authInfo.TenantID,
		UserID:    authInfo.UserID,
		Params:    params,
		TraceID:   traceID,
	}

	// 记录接收时间
	startTime := time.Now()

	// 路由请求
	resp, err := h.router.Route(c.Request.Context(), feature, params)

	// 计算总耗时
	if resp != nil {
		resp.TotalMs = int(time.Since(startTime).Milliseconds())
		resp.ReceivedAt = startTime
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    5001,
			Message: "处理失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// authenticate 认证
func (h *Handler) authenticate(c *gin.Context) (*AuthInfo, error) {
	if h.auth == nil {
		return &AuthInfo{TenantID: "default", UserID: "anonymous"}, nil
	}

	token := c.GetHeader("Authorization")
	if token == "" {
		token = c.GetHeader("X-API-Key")
	}

	return h.auth.Authenticate(c.Request.Context(), token)
}

// HealthCheck 健康检查
func (h *Handler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"time":   time.Now().Unix(),
	})
}

// Ready 就绪检查
func (h *Handler) Ready(c *gin.Context) {
	// TODO: 检查依赖服务是否就绪
	c.JSON(http.StatusOK, gin.H{
		"status": "ready",
	})
}

// generateRequestID 生成请求ID
func generateRequestID() string {
	return "req-" + uuid.New().String()
}

// getTraceID 获取追踪ID
func getTraceID(c *gin.Context) string {
	traceID := c.GetHeader("X-Trace-ID")
	if traceID == "" {
		traceID = c.GetHeader("X-Request-ID")
	}
	if traceID == "" {
		traceID = "trace-" + uuid.New().String()
	}
	return traceID
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

// Metrics Prometheus 指标
func (h *Handler) Metrics(c *gin.Context) {
	// 简单的指标输出
	c.String(http.StatusOK, `# HELP ai_platform_requests_total Total number of requests
# TYPE ai_platform_requests_total counter
ai_platform_requests_total{feature="text_to_image",status="success"} 0
ai_platform_requests_total{feature="image_editing",status="success"} 0
ai_platform_requests_total{feature="text_generation",status="success"} 0

# HELP ai_platform_request_duration_seconds Request duration in seconds
# TYPE ai_platform_request_duration_seconds histogram
ai_platform_request_duration_seconds_bucket{feature="text_to_image",le="0.01"} 0
ai_platform_request_duration_seconds_bucket{feature="text_to_image",le="0.05"} 0
ai_platform_request_duration_seconds_bucket{feature="text_to_image",le="+Inf"} 0

# HELP ai_platform_up Service is up
# TYPE ai_platform_up gauge
ai_platform_up 1
`)
}

// ListFeatures 列出所有特性
func (h *Handler) ListFeatures(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": []gin.H{
			{
				"id":          "text_to_image",
				"name":        "文生图",
				"description": "文本生成图像功能",
				"enabled":     true,
			},
			{
				"id":          "image_editing",
				"name":        "图像编辑",
				"description": "图像编辑功能",
				"enabled":     true,
			},
			{
				"id":          "image_stylization",
				"name":        "图像风格化",
				"description": "图像风格化处理",
				"enabled":     true,
			},
			{
				"id":          "text_generation",
				"name":        "文本生成",
				"description": "AI文本生成功能",
				"enabled":     true,
			},
		},
	})
}

// GetFeature 获取特性配置
func (h *Handler) GetFeature(c *gin.Context) {
	featureID := c.Param("id")
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"id":          featureID,
			"name":        "文生图",
			"description": "文本生成图像功能",
			"enabled":     true,
			"self_hosted": gin.H{
				"enabled":          true,
				"priority":         1,
				"cost_per_request": 0.01,
				"max_concurrent":   10,
			},
			"third_party": []gin.H{
				{
					"provider_id":      "openai",
					"provider_name":    "OpenAI",
					"provider_type":    "openai",
					"priority":         2,
					"cost_per_request": 0.02,
					"enabled":          true,
				},
			},
			"routing": gin.H{
				"strategy":         "cost_based",
				"fallback_enabled": true,
			},
		},
	})
}

// CreateFeature 创建特性配置
func (h *Handler) CreateFeature(c *gin.Context) {
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    1001,
			Message: "参数错误",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "Feature created successfully",
		"data":    req,
	})
}

// UpdateFeature 更新特性配置
func (h *Handler) UpdateFeature(c *gin.Context) {
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    1001,
			Message: "参数错误",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "Feature updated successfully",
		"data":    req,
	})
}

// DeleteFeature 删除特性配置
func (h *Handler) DeleteFeature(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "Feature deleted successfully",
	})
}

// ListKeys 列出所有API Keys
func (h *Handler) ListKeys(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": []gin.H{
			{
				"id":           "key-1",
				"provider_id":  "openai",
				"provider_type": "openai",
				"key_name":     "OpenAI API Key",
				"description":  "测试用密钥",
				"created_at":   "2024-01-01T00:00:00Z",
			},
		},
	})
}

// GetKey 获取API Key详情
func (h *Handler) GetKey(c *gin.Context) {
	keyID := c.Param("id")
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"id":           keyID,
			"provider_id":  "openai",
			"provider_type": "openai",
			"key_name":     "OpenAI API Key",
			"description":  "测试用密钥",
			"created_at":   "2024-01-01T00:00:00Z",
		},
	})
}

// CreateKey 创建API Key
func (h *Handler) CreateKey(c *gin.Context) {
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    1001,
			Message: "参数错误",
		})
		return
	}
	req["id"] = "key-" + generateRequestID()[:8]
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "API Key created successfully",
		"data":    req,
	})
}

// UpdateKey 更新API Key
func (h *Handler) UpdateKey(c *gin.Context) {
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    1001,
			Message: "参数错误",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "API Key updated successfully",
		"data":    req,
	})
}

// DeleteKey 删除API Key
func (h *Handler) DeleteKey(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "API Key deleted successfully",
	})
}

// ListServices 列出所有服务
func (h *Handler) ListServices(c *gin.Context) {
	serviceType := c.Query("service_type")
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": []gin.H{
			{
				"service_id":       "svc-1",
				"service_type":     serviceType,
				"hostname":         "localhost",
				"ip_address":       "192.168.1.100",
				"port":             8080,
				"status":           "healthy",
				"cpu_utilization":  45.5,
				"gpu_utilization":  60.0,
				"memory_usage":     1024000000,
				"queue_size":       5,
				"processed_count":  1000,
			},
		},
	})
}

// GetService 获取服务详情
func (h *Handler) GetService(c *gin.Context) {
	serviceID := c.Param("id")
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"service_id":       serviceID,
			"service_type":     "text_to_image",
			"hostname":         "localhost",
			"ip_address":       "192.168.1.100",
			"port":             8080,
			"status":           "healthy",
			"cpu_utilization":  45.5,
			"gpu_utilization":  60.0,
			"memory_usage":     1024000000,
			"queue_size":       5,
			"processed_count":  1000,
		},
	})
}

// RegisterService 注册服务
func (h *Handler) RegisterService(c *gin.Context) {
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    1001,
			Message: "参数错误",
		})
		return
	}
	req["service_id"] = "svc-" + generateRequestID()[:8]
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "Service registered successfully",
		"data":    req,
	})
}

// Heartbeat 服务心跳
func (h *Handler) Heartbeat(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"config_update":   nil,
			"drain_requested": false,
		},
	})
}

// RouteDecide 路由决策
func (h *Handler) RouteDecide(c *gin.Context) {
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    1001,
			Message: "参数错误",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"provider_id": "self_hosted",
			"reason":      "Cost optimized: self-hosted is cheaper",
			"estimated_cost": 0.01,
			"estimated_latency_ms": 2000,
		},
	})
}

// CheckBudget 检查预算
func (h *Handler) CheckBudget(c *gin.Context) {
	feature := c.Query("feature")
	tenantID := c.Query("tenant_id")
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"allowed": true,
			"reason":  "",
			"global_budget": gin.H{
				"total":      30000,
				"used":       1500,
				"remaining":  28500,
				"percentage": 5.0,
			},
			"service_budget": gin.H{
				"id":         "service:" + feature,
				"total":      1000,
				"used":       50,
				"remaining":  950,
				"percentage": 5.0,
			},
		},
	})
}

// GetBudget 获取预算详情
func (h *Handler) GetBudget(c *gin.Context) {
	budgetID := c.Param("id")
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"id":          budgetID,
			"name":        "文生图预算",
			"type":        "service",
			"amount":      1000,
			"period":      "daily",
			"alerts": []gin.H{
				{"at": 0.8, "action": "notify", "enabled": true},
				{"at": 0.95, "action": "block", "enabled": true},
			},
		},
	})
}

// CreateBudget 创建预算
func (h *Handler) CreateBudget(c *gin.Context) {
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    1001,
			Message: "参数错误",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "Budget created successfully",
		"data":    req,
	})
}

// GetStats 获取统计信息
func (h *Handler) GetStats(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"total_requests":      10000,
			"successful_requests": 9500,
			"failed_requests":     500,
			"avg_latency_ms":      1500,
			"active_services":     5,
			"total_cost":          100.50,
			"features": []gin.H{
				{"name": "text_to_image", "requests": 5000, "avg_latency_ms": 2000},
				{"name": "image_editing", "requests": 3000, "avg_latency_ms": 1500},
				{"name": "text_generation", "requests": 2000, "avg_latency_ms": 500},
			},
		},
	})
}
