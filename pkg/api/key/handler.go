package key

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/yijian/ai-platform/pkg/model"
)

// Handler API密钥处理器
type Handler struct {
	service Service
}

// Service API密钥服务接口
type Service interface {
	CreateKey(req *model.CreateKeyRequest) (*model.APIKey, error)
	UpdateKey(id string, req *model.UpdateKeyRequest) error
	DeleteKey(id string) error
	GetKey(id string) (*model.APIKey, error)
	ListKeys(filter *model.KeyFilter) ([]*model.APIKey, int, error)
	EnableKey(id string) error
	DisableKey(id string) error
	RotateKey(id string, req *model.RotateKeyRequest) (*model.APIKey, error)
	GetActiveKey(vendor, service string) (*model.APIKey, error)
	GetUsage(id, period string) (*model.UsageStats, error)
	HealthCheck(id string) (*model.HealthStatus, error)
}

// NewHandler 创建密钥管理处理器
func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	keys := r.Group("/keys")
	{
		keys.POST("", h.CreateKey)
		keys.GET("", h.ListKeys)
		keys.GET("/:id", h.GetKey)
		keys.PUT("/:id", h.UpdateKey)
		keys.DELETE("/:id", h.DeleteKey)
		keys.POST("/:id/enable", h.EnableKey)
		keys.POST("/:id/disable", h.DisableKey)
		keys.POST("/:id/rotate", h.RotateKey)
		keys.GET("/:id/usage", h.GetUsage)
		keys.POST("/:id/health-check", h.HealthCheck)
	}
}

// CreateKey 创建密钥
// @Summary 创建API密钥
// @Description 创建新的第三方API密钥
// @Tags keys
// @Accept json
// @Produce json
// @Param request body model.CreateKeyRequest true "创建请求"
// @Success 200 {object} Response{data=model.APIKey}
// @Router /api/v1/keys [post]
func (h *Handler) CreateKey(c *gin.Context) {
	var req model.CreateKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    1001,
			Message: "参数错误: " + err.Error(),
		})
		return
	}

	key, err := h.service.CreateKey(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    3001,
			Message: "创建失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    key,
	})
}

// ListKeys 列出密钥
// @Summary 列出API密钥
// @Description 获取密钥列表
// @Tags keys
// @Produce json
// @Param vendor query string false "厂商"
// @Param service query string false "服务"
// @Param enabled query bool false "是否启用"
// @Param limit query int false "限制数量"
// @Param offset query int false "偏移量"
// @Success 200 {object} Response{data=ListKeysResponse}
// @Router /api/v1/keys [get]
func (h *Handler) ListKeys(c *gin.Context) {
	filter := &model.KeyFilter{
		Vendor: c.Query("vendor"),
		Service: c.Query("service"),
		Limit:   20,
		Offset:  0,
	}

	if limit := c.Query("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil && l > 0 {
			filter.Limit = l
		}
	}

	if offset := c.Query("offset"); offset != "" {
		if o, err := strconv.Atoi(offset); err == nil && o >= 0 {
			filter.Offset = o
		}
	}

	if enabled := c.Query("enabled"); enabled != "" {
		if e, err := strconv.ParseBool(enabled); err == nil {
			filter.Enabled = &e
		}
	}

	keys, total, err := h.service.ListKeys(filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    3001,
			Message: "查询失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: ListKeysResponse{
			Keys:      keys,
			TotalCount: total,
		},
	})
}

// GetKey 获取密钥详情
// @Summary 获取API密钥详情
// @Description 获取指定密钥的详细信息（不含明文）
// @Tags keys
// @Produce json
// @Param id path string true "密钥ID"
// @Success 200 {object} Response{data=model.APIKey}
// @Router /api/v1/keys/{id} [get]
func (h *Handler) GetKey(c *gin.Context) {
	id := c.Param("id")

	key, err := h.service.GetKey(id)
	if err != nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    3001,
			Message: "密钥不存在",
		})
		return
	}

	// 清除敏感字段
	key.EncryptedDEK = ""
	key.EncryptedKey = ""

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    key,
	})
}

// UpdateKey 更新密钥
// @Summary 更新API密钥
// @Description 更新密钥配置
// @Tags keys
// @Accept json
// @Produce json
// @Param id path string true "密钥ID"
// @Param request body model.UpdateKeyRequest true "更新请求"
// @Success 200 {object} Response
// @Router /api/v1/keys/{id} [put]
func (h *Handler) UpdateKey(c *gin.Context) {
	id := c.Param("id")

	var req model.UpdateKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    1001,
			Message: "参数错误: " + err.Error(),
		})
		return
	}

	if err := h.service.UpdateKey(id, &req); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    3001,
			Message: "更新失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
	})
}

// DeleteKey 删除密钥
// @Summary 删除API密钥
// @Description 删除指定密钥
// @Tags keys
// @Produce json
// @Param id path string true "密钥ID"
// @Success 200 {object} Response
// @Router /api/v1/keys/{id} [delete]
func (h *Handler) DeleteKey(c *gin.Context) {
	id := c.Param("id")

	if err := h.service.DeleteKey(id); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    3001,
			Message: "删除失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
	})
}

// EnableKey 启用密钥
// @Summary 启用API密钥
// @Description 启用指定密钥
// @Tags keys
// @Produce json
// @Param id path string true "密钥ID"
// @Success 200 {object} Response
// @Router /api/v1/keys/{id}/enable [post]
func (h *Handler) EnableKey(c *gin.Context) {
	id := c.Param("id")

	if err := h.service.EnableKey(id); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    3001,
			Message: "启用失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
	})
}

// DisableKey 禁用密钥
// @Summary 禁用API密钥
// @Description 禁用指定密钥
// @Tags keys
// @Produce json
// @Param id path string true "密钥ID"
// @Success 200 {object} Response
// @Router /api/v1/keys/{id}/disable [post]
func (h *Handler) DisableKey(c *gin.Context) {
	id := c.Param("id")

	if err := h.service.DisableKey(id); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    3001,
			Message: "禁用失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
	})
}

// RotateKey 轮换密钥
// @Summary 轮换API密钥
// @Description 轮换指定密钥
// @Tags keys
// @Accept json
// @Produce json
// @Param id path string true "密钥ID"
// @Param request body model.RotateKeyRequest true "轮换请求"
// @Success 200 {object} Response{data=model.APIKey}
// @Router /api/v1/keys/{id}/rotate [post]
func (h *Handler) RotateKey(c *gin.Context) {
	id := c.Param("id")

	var req model.RotateKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    1001,
			Message: "参数错误: " + err.Error(),
		})
		return
	}

	key, err := h.service.RotateKey(id, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    3001,
			Message: "轮换失败: " + err.Error(),
		})
		return
	}

	// 清除敏感字段
	key.EncryptedDEK = ""
	key.EncryptedKey = ""

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    key,
	})
}

// GetUsage 获取密钥使用统计
// @Summary 获取密钥使用统计
// @Description 获取指定密钥的使用统计
// @Tags keys
// @Produce json
// @Param id path string true "密钥ID"
// @Param period query string false "统计周期: daily, monthly" default(daily)
// @Success 200 {object} Response{data=model.UsageStats}
// @Router /api/v1/keys/{id}/usage [get]
func (h *Handler) GetUsage(c *gin.Context) {
	id := c.Param("id")
	period := c.DefaultQuery("period", "daily")

	usage, err := h.service.GetUsage(id, period)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    3001,
			Message: "查询失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    usage,
	})
}

// HealthCheck 健康检查
// @Summary 密钥健康检查
// @Description 检查密钥是否可用
// @Tags keys
// @Produce json
// @Param id path string true "密钥ID"
// @Success 200 {object} Response{data=model.HealthStatus}
// @Router /api/v1/keys/{id}/health-check [post]
func (h *Handler) HealthCheck(c *gin.Context) {
	id := c.Param("id")

	status, err := h.service.HealthCheck(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    3001,
			Message: "健康检查失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    status,
	})
}

// Response 通用响应
type Response struct {
	Code     int         `json:"code"`
	Message  string      `json:"message"`
	Data     interface{} `json:"data,omitempty"`
	RequestID string     `json:"request_id,omitempty"`
}

// ListKeysResponse 密钥列表响应
type ListKeysResponse struct {
	Keys       []*model.APIKey `json:"keys"`
	TotalCount int             `json:"total_count"`
}
