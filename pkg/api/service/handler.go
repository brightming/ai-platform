package service

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/brightming/ai-platform/pkg/model"
)

// Handler 服务注册处理器
type Handler struct {
	service Service
}

// Service 服务注册服务接口
type Service interface {
	Register(req *model.RegisterRequest) (*model.RegisterResponse, error)
	Heartbeat(req *model.HeartbeatRequest) (*model.HeartbeatResponse, error)
	Shutdown(req *model.ShutdownRequest) (*model.ShutdownResponse, error)
	GetService(id string) (*model.RegisteredService, error)
	ListServices(filter *model.ServiceFilter) (*model.GetServicesResponse, error)
	GetServicesByType(serviceType string) ([]*model.RegisteredService, error)
	GetHealthyServices(serviceType string) ([]*model.RegisteredService, error)
}

// NewHandler 创建服务注册处理器
func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	services := r.Group("/services")
	{
		services.POST("/register", h.Register)
		services.POST("/heartbeat", h.Heartbeat)
		services.POST("/shutdown", h.Shutdown)
		services.GET("", h.ListServices)
		services.GET("/:id", h.GetService)
		services.GET("/type/:type", h.GetServicesByType)
	}
}

// Register 服务注册
// @Summary 服务注册
// @Description 自研镜像启动时注册服务
// @Tags services
// @Accept json
// @Produce json
// @Param request body model.RegisterRequest true "注册请求"
// @Success 200 {object} Response{data=model.RegisterResponse}
// @Router /api/v1/services/register [post]
func (h *Handler) Register(c *gin.Context) {
	var req model.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    1001,
			Message: "参数错误: " + err.Error(),
		})
		return
	}

	resp, err := h.service.Register(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    4001,
			Message: "注册失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    resp,
	})
}

// Heartbeat 心跳上报
// @Summary 心跳上报
// @Description 服务定期上报心跳
// @Tags services
// @Accept json
// @Produce json
// @Param request body model.HeartbeatRequest true "心跳请求"
// @Success 200 {object} Response{data=model.HeartbeatResponse}
// @Router /api/v1/services/heartbeat [post]
func (h *Handler) Heartbeat(c *gin.Context) {
	var req model.HeartbeatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    1001,
			Message: "参数错误: " + err.Error(),
		})
		return
	}

	resp, err := h.service.Heartbeat(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    4001,
			Message: "心跳处理失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    resp,
	})
}

// Shutdown 优雅关闭
// @Summary 优雅关闭
// @Description 服务请求优雅关闭
// @Tags services
// @Accept json
// @Produce json
// @Param request body model.ShutdownRequest true "关闭请求"
// @Success 200 {object} Response{data=model.ShutdownResponse}
// @Router /api/v1/services/shutdown [post]
func (h *Handler) Shutdown(c *gin.Context) {
	var req model.ShutdownRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    1001,
			Message: "参数错误: " + err.Error(),
		})
		return
	}

	resp, err := h.service.Shutdown(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    4001,
			Message: "关闭失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    resp,
	})
}

// ListServices 列出服务
// @Summary 列出服务
// @Description 获取服务列表
// @Tags services
// @Produce json
// @Param type query string false "服务类型"
// @Param status query string false "服务状态"
// @Param limit query int false "限制数量"
// @Param offset query int false "偏移量"
// @Success 200 {object} Response{data=model.GetServicesResponse}
// @Router /api/v1/services [get]
func (h *Handler) ListServices(c *gin.Context) {
	filter := &model.ServiceFilter{
		ServiceType: c.Query("type"),
		Limit:       20,
		Offset:      0,
	}

	if status := c.Query("status"); status != "" {
		s := model.ServiceStatus(status)
		filter.Status = &s
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

	resp, err := h.service.ListServices(filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    4001,
			Message: "查询失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    resp,
	})
}

// GetService 获取服务详情
// @Summary 获取服务详情
// @Description 获取指定服务的详细信息
// @Tags services
// @Produce json
// @Param id path string true "服务ID"
// @Success 200 {object} Response{data=model.RegisteredService}
// @Router /api/v1/services/{id} [get]
func (h *Handler) GetService(c *gin.Context) {
	id := c.Param("id")

	service, err := h.service.GetService(id)
	if err != nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    4001,
			Message: "服务不存在",
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    service,
	})
}

// GetServicesByType 根据类型获取服务
// @Summary 根据类型获取服务
// @Description 获取指定类型的所有服务
// @Tags services
// @Produce json
// @Param type path string true "服务类型"
// @Success 200 {object} Response{data=[]model.RegisteredService}
// @Router /api/v1/services/type/{type} [get]
func (h *Handler) GetServicesByType(c *gin.Context) {
	serviceType := c.Param("type")

	services, err := h.service.GetServicesByType(serviceType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    4001,
			Message: "查询失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    services,
	})
}

// Response 通用响应
type Response struct {
	Code      int         `json:"code"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data,omitempty"`
	RequestID string      `json:"request_id,omitempty"`
}
