package config

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/brightming/ai-platform/pkg/model"
)

// Handler 功能配置处理器
type Handler struct {
	service Service
}

// Service 功能配置服务接口
type Service interface {
	CreateFeature(feature *model.Feature) error
	UpdateFeature(id string, feature *model.Feature) error
	DeleteFeature(id string) error
	GetFeature(id string) (*model.Feature, error)
	ListFeatures(filter *model.FeatureFilter) ([]*model.Feature, int, error)
	AddProvider(featureID string, provider *model.ProviderConfig) error
	UpdateProvider(featureID, providerID string, provider *model.ProviderConfig) error
	RemoveProvider(featureID, providerID string) error
	UpdateRoutingStrategy(featureID string, strategy *model.RoutingStrategy) error
}

// NewHandler 创建配置处理器
func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	features := r.Group("/features")
	{
		features.POST("", h.CreateFeature)
		features.GET("", h.ListFeatures)
		features.GET("/:id", h.GetFeature)
		features.PUT("/:id", h.UpdateFeature)
		features.DELETE("/:id", h.DeleteFeature)

		// Provider管理
		features.POST("/:id/providers", h.AddProvider)
		features.PUT("/:id/providers/:providerId", h.UpdateProvider)
		features.DELETE("/:id/providers/:providerId", h.RemoveProvider)

		// 路由策略
		features.PUT("/:id/routing", h.UpdateRouting)
	}
}

// CreateFeature 创建功能
// @Summary 创建功能
// @Description 创建新的AI功能配置
// @Tags config
// @Accept json
// @Produce json
// @Param request body model.CreateFeatureRequest true "创建请求"
// @Success 200 {object} Response{data=model.Feature}
// @Router /api/v1/features [post]
func (h *Handler) CreateFeature(c *gin.Context) {
	var req model.CreateFeatureRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    1001,
			Message: "参数错误: " + err.Error(),
		})
		return
	}

	feature := &model.Feature{
		ID:          req.ID,
		Name:        req.Name,
		Category:    req.Category,
		Description: req.Description,
		Tags:        req.Tags,
		Enabled:     true,
		Providers:   req.Providers,
		Routing:     req.Routing,
		Cost:        req.Cost,
	}

	if err := h.service.CreateFeature(feature); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    5001,
			Message: "创建失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    feature,
	})
}

// ListFeatures 列出功能
// @Summary 列出功能
// @Description 获取功能列表
// @Tags config
// @Produce json
// @Param category query string false "功能类别"
// @Param enabled query bool false "是否启用"
// @Param limit query int false "限制数量" default(20)
// @Param offset query int false "偏移量" default(0)
// @Success 200 {object} Response{data=ListFeaturesResponse}
// @Router /api/v1/features [get]
func (h *Handler) ListFeatures(c *gin.Context) {
	filter := &model.FeatureFilter{
		Category: c.Query("category"),
		Limit:    20,
		Offset:   0,
	}

	if limit := c.Query("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil {
			filter.Limit = l
		}
	}

	if offset := c.Query("offset"); offset != "" {
		if o, err := strconv.Atoi(offset); err == nil {
			filter.Offset = o
		}
	}

	if enabled := c.Query("enabled"); enabled != "" {
		if e, err := strconv.ParseBool(enabled); err == nil {
			filter.Enabled = &e
		}
	}

	features, total, err := h.service.ListFeatures(filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    5001,
			Message: "查询失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: ListFeaturesResponse{
			Features:   features,
			TotalCount: total,
		},
	})
}

// GetFeature 获取功能详情
// @Summary 获取功能详情
// @Description 获取指定功能的详细信息
// @Tags config
// @Produce json
// @Param id path string true "功能ID"
// @Success 200 {object} Response{data=model.Feature}
// @Router /api/v1/features/{id} [get]
func (h *Handler) GetFeature(c *gin.Context) {
	id := c.Param("id")

	feature, err := h.service.GetFeature(id)
	if err != nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    2001,
			Message: "功能不存在: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    feature,
	})
}

// UpdateFeature 更新功能
// @Summary 更新功能
// @Description 更新功能配置
// @Tags config
// @Accept json
// @Produce json
// @Param id path string true "功能ID"
// @Param request body model.UpdateFeatureRequest true "更新请求"
// @Success 200 {object} Response
// @Router /api/v1/features/{id} [put]
func (h *Handler) UpdateFeature(c *gin.Context) {
	id := c.Param("id")

	var req model.UpdateFeatureRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    1001,
			Message: "参数错误: " + err.Error(),
		})
		return
	}

	existing, err := h.service.GetFeature(id)
	if err != nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    2001,
			Message: "功能不存在",
		})
		return
	}

	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.Description != nil {
		existing.Description = *req.Description
	}
	if req.Tags != nil {
		existing.Tags = req.Tags
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	if req.Routing != nil {
		existing.Routing = req.Routing
	}
	if req.Cost != nil {
		existing.Cost = req.Cost
	}

	if err := h.service.UpdateFeature(id, existing); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    5001,
			Message: "更新失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
	})
}

// DeleteFeature 删除功能
// @Summary 删除功能
// @Description 删除指定功能
// @Tags config
// @Produce json
// @Param id path string true "功能ID"
// @Success 200 {object} Response
// @Router /api/v1/features/{id} [delete]
func (h *Handler) DeleteFeature(c *gin.Context) {
	id := c.Param("id")

	if err := h.service.DeleteFeature(id); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    5001,
			Message: "删除失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
	})
}

// AddProvider 添加Provider
// @Summary 添加Provider
// @Description 为指定功能添加Provider
// @Tags config
// @Accept json
// @Produce json
// @Param id path string true "功能ID"
// @Param request body model.AddProviderRequest true "Provider配置"
// @Success 200 {object} Response{data=model.ProviderConfig}
// @Router /api/v1/features/{id}/providers [post]
func (h *Handler) AddProvider(c *gin.Context) {
	featureID := c.Param("id")

	var req model.AddProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    1001,
			Message: "参数错误: " + err.Error(),
		})
		return
	}

	provider := &model.ProviderConfig{
		ID:                   req.ID,
		FeatureID:            featureID,
		Type:                 req.Type,
		Vendor:               req.Vendor,
		Enabled:              req.Enabled,
		Priority:             req.Priority,
		Weight:               req.Weight,
		Image:                req.Image,
		MinInstances:         req.MinInstances,
		MaxInstances:         req.MaxInstances,
		ResourceRequirements:  req.ResourceRequirements,
		CapabilityMatch:      req.CapabilityMatch,
		Model:                req.Model,
		APIKeyRef:            req.APIKeyRef,
		RateLimit:            req.RateLimit,
		Endpoint:             req.Endpoint,
		Extra:                req.Extra,
	}

	if err := h.service.AddProvider(featureID, provider); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    5001,
			Message: "添加失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    provider,
	})
}

// UpdateProvider 更新Provider
// @Summary 更新Provider
// @Description 更新Provider配置
// @Tags config
// @Accept json
// @Produce json
// @Param id path string true "功能ID"
// @Param providerId path string true "Provider ID"
// @Param request body model.UpdateProviderRequest true "更新请求"
// @Success 200 {object} Response
// @Router /api/v1/features/{id}/providers/{providerId} [put]
func (h *Handler) UpdateProvider(c *gin.Context) {
	featureID := c.Param("id")
	providerID := c.Param("providerId")

	var req model.UpdateProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    1001,
			Message: "参数错误: " + err.Error(),
		})
		return
	}

	provider := &model.ProviderConfig{ID: providerID}
	if req.Enabled != nil {
		provider.Enabled = *req.Enabled
	}
	if req.Priority != nil {
		provider.Priority = *req.Priority
	}
	if req.Weight != nil {
		provider.Weight = *req.Weight
	}
	if req.Extra != nil {
		provider.Extra = req.Extra
	}

	if err := h.service.UpdateProvider(featureID, providerID, provider); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    5001,
			Message: "更新失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
	})
}

// RemoveProvider 删除Provider
// @Summary 删除Provider
// @Description 删除指定Provider
// @Tags config
// @Produce json
// @Param id path string true "功能ID"
// @Param providerId path string true "Provider ID"
// @Success 200 {object} Response
// @Router /api/v1/features/{id}/providers/{providerId} [delete]
func (h *Handler) RemoveProvider(c *gin.Context) {
	featureID := c.Param("id")
	providerID := c.Param("providerId")

	if err := h.service.RemoveProvider(featureID, providerID); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    5001,
			Message: "删除失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
	})
}

// UpdateRouting 更新路由策略
// @Summary 更新路由策略
// @Description 更新功能路由策略
// @Tags config
// @Accept json
// @Produce json
// @Param id path string true "功能ID"
// @Param request body model.RoutingStrategy true "路由策略"
// @Success 200 {object} Response
// @Router /api/v1/features/{id}/routing [put]
func (h *Handler) UpdateRouting(c *gin.Context) {
	featureID := c.Param("id")

	var strategy model.RoutingStrategy
	if err := c.ShouldBindJSON(&strategy); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    1001,
			Message: "参数错误: " + err.Error(),
		})
		return
	}

	if err := h.service.UpdateRoutingStrategy(featureID, &strategy); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    5001,
			Message: "更新失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
	})
}

// Response 通用响应
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	RequestID string    `json:"request_id,omitempty"`
}

// ListFeaturesResponse 功能列表响应
type ListFeaturesResponse struct {
	Features   []*model.Feature `json:"features"`
	TotalCount int             `json:"total_count"`
}
