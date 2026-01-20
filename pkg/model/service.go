package model

import "time"

// RegisteredService 已注册的服务
type RegisteredService struct {
	ID            string             `json:"id" db:"id"`
	ServiceType   string             `json:"service_type" db:"service_type"` // text_to_image, image_editing, etc.
	Version       string             `json:"version" db:"version"`
	Hostname      string             `json:"hostname" db:"hostname"`
	IPAddress     string             `json:"ip_address" db:"ip_address"`
	Port          int                `json:"port" db:"port"`

	// 能力元数据
	Capabilities  *ServiceCapabilities `json:"capabilities" db:"capabilities"`
	Resources     *ResourceSpec        `json:"resources" db:"resources"`
	Performance   *PerformanceSpec     `json:"performance" db:"performance"`

	// 状态
	Status        ServiceStatus       `json:"status" db:"status"`
	LastHeartbeat time.Time           `json:"last_heartbeat" db:"last_heartbeat"`
	HeartbeatMissed int               `json:"heartbeat_missed" db:"heartbeat_missed"`
	StartedAt     time.Time           `json:"started_at" db:"started_at"`

	// 运行时状态
	CurrentLoad   float64            `json:"current_load" db:"current_load"`
	QueueSize     int                `json:"queue_size" db:"queue_size"`
	ProcessedCount int64             `json:"processed_count" db:"processed_count"`
	ErrorCount    int64              `json:"error_count" db:"error_count"`
	CPUUtilization float64           `json:"cpu_utilization" db:"cpu_utilization"`
	GPUUtilization float64           `json:"gpu_utilization" db:"gpu_utilization"`
	MemoryUsage   int64              `json:"memory_usage" db:"memory_usage"`

	// 元数据
	Metadata      map[string]string  `json:"metadata" db:"-"`

	RegisteredAt  time.Time          `json:"registered_at" db:"registered_at"`
	UpdatedAt     time.Time          `json:"updated_at" db:"updated_at"`
}

// ServiceStatus 服务状态
type ServiceStatus string

const (
	StatusHealthy   ServiceStatus = "healthy"
	StatusDegraded  ServiceStatus = "degraded"
	StatusUnhealthy ServiceStatus = "unhealthy"
	StatusDraining  ServiceStatus = "draining"
	StatusTerminated ServiceStatus = "terminated"
)

// ServiceCapabilities 服务能力元数据
type ServiceCapabilities struct {
	SupportedModels        []string               `json:"supported_models"`
	SupportedResolutions   []string               `json:"supported_resolutions"`
	MaxBatchSize          int32                  `json:"max_batch_size"`
	SupportedFormats      []string               `json:"supported_formats"`
	SupportedStyles       []string               `json:"supported_styles,omitempty"`
	InferenceStepsRange   []int                  `json:"inference_steps_range,omitempty"`
	GuidanceScaleRange    []float64              `json:"guidance_scale_range,omitempty"`
	CustomCapabilities    map[string]interface{} `json:"custom_capabilities,omitempty"`
}

// PerformanceSpec 性能规格
type PerformanceSpec struct {
	EstimatedLatencyMs    int     `json:"estimated_latency_ms"`
	ThroughputPerMinute   int     `json:"throughput_per_minute"`
	WarmupTimeSeconds     int     `json:"warmup_time_seconds"`
}

// ServiceFilter 服务查询过滤条件
type ServiceFilter struct {
	ServiceType string         `json:"service_type"`
	Status      *ServiceStatus `json:"status"`
	Limit       int            `json:"limit"`
	Offset      int            `json:"offset"`
}

// RegisterRequest 服务注册请求
type RegisterRequest struct {
	Metadata      *ServiceCapabilities `json:"capabilities" binding:"required"`
	Hostname      string               `json:"hostname"`
	IPAddress     string               `json:"ip_address"`
	Port          int                  `json:"port"`
	Version       string               `json:"version"`
	Resources     *ResourceSpec        `json:"resources"`
	Performance   *PerformanceSpec     `json:"performance"`
}

// RegisterResponse 服务注册响应
type RegisterResponse struct {
	ServiceID           string `json:"service_id"`
	HeartbeatInterval   int    `json:"heartbeat_interval"`   // 秒
	ConfigVersion       string `json:"config_version"`
	Token               string `json:"token"`                // 心跳认证令牌
}

// HeartbeatRequest 心跳请求
type HeartbeatRequest struct {
	ServiceID      string  `json:"service_id" binding:"required"`
	Timestamp      string  `json:"timestamp" binding:"required"`
	CurrentLoad    float64 `json:"current_load"`
	QueueSize      int     `json:"queue_size"`
	ProcessedCount int64   `json:"processed_count"`
	ErrorCount     int64   `json:"error_count"`
	MemoryUsage    int64   `json:"memory_usage"`
	CPUUtilization float64 `json:"cpu_utilization"`
	GPUUtilization float64 `json:"gpu_utilization"`
	Token          string  `json:"token"`
}

// HeartbeatResponse 心跳响应
type HeartbeatResponse struct {
	Status         string           `json:"status"`
	ConfigUpdate   *ConfigUpdate    `json:"config_update,omitempty"`
	DrainRequested bool             `json:"drain_requested"`
	Message        string           `json:"message,omitempty"`
}

// ConfigUpdate 配置更新
type ConfigUpdate struct {
	Version  string                 `json:"version"`
	Config   map[string]interface{} `json:"config"`
}

// ShutdownRequest 关闭请求
type ShutdownRequest struct {
	ServiceID string `json:"service_id" binding:"required"`
	Reason    string `json:"reason"`
}

// ShutdownResponse 关闭响应
type ShutdownResponse struct {
	GracePeriodSeconds int    `json:"grace_period_seconds"`
	Message            string `json:"message"`
}

// GetServicesResponse 服务列表响应
type GetServicesResponse struct {
	Services    []*RegisteredService `json:"services"`
	TotalCount  int                  `json:"total_count"`
	HealthyCount int                 `json:"healthy_count"`
	DegradedCount int                `json:"degraded_count"`
	UnhealthyCount int               `json:"unhealthy_count"`
}
