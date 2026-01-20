package model

import "time"

// InferenceRequest 推理请求
type InferenceRequest struct {
	RequestID    string                 `json:"request_id" binding:"required"`
	Feature      string                 `json:"feature" binding:"required"`     // text_to_image, image_editing, etc.
	TenantID     string                 `json:"tenant_id,omitempty"`
	UserID       string                 `json:"user_id,omitempty"`
	Params       map[string]interface{} `json:"params" binding:"required"`
	Priority     int                    `json:"priority"`    // 0=normal, 1=high, -1=low
	TraceID      string                 `json:"trace_id,omitempty"`
}

// TextToImageRequest 文生图请求参数
type TextToImageRequest struct {
	Prompt       string   `json:"prompt" binding:"required"`
	NegativePrompt string `json:"negative_prompt,omitempty"`
	Width        int      `json:"width" binding:"required,min=64,max=4096"`
	Height       int      `json:"height" binding:"required,min=64,max=4096"`
	Steps        int      `json:"steps,omitempty"`
	CFGScale     float64  `json:"cfg_scale,omitempty"`
	Seed         *int64   `json:"seed,omitempty"`
	Count        int      `json:"count,omitempty"`  // 生成数量
}

// ImageEditRequest 图像编辑请求参数
type ImageEditRequest struct {
	Image        string   `json:"image" binding:"required"`      // base64 or URL
	Mask         string   `json:"mask,omitempty"`               // base64 or URL
	Prompt       string   `json:"prompt" binding:"required"`
	NegativePrompt string `json:"negative_prompt,omitempty"`
	Width        int      `json:"width,omitempty"`
	Height       int      `json:"height,omitempty"`
	Steps        int      `json:"steps,omitempty"`
	CFGScale     float64  `json:"cfg_scale,omitempty"`
}

// ImageStylizationRequest 图像风格化请求参数
type ImageStylizationRequest struct {
	Image        string   `json:"image" binding:"required"`
	Style        string   `json:"style" binding:"required"`
	Strength     float64  `json:"strength,omitempty"`
}

// TextGenerationRequest 文本生成请求参数
type TextGenerationRequest struct {
	Prompt       string   `json:"prompt" binding:"required"`
	MaxTokens    int      `json:"max_tokens,omitempty"`
	Temperature  float64  `json:"temperature,omitempty"`
	TopP         float64  `json:"top_p,omitempty"`
	TopK         int      `json:"top_k,omitempty"`
	Stop         []string `json:"stop,omitempty"`
}

// InferenceResponse 推理响应
type InferenceResponse struct {
	RequestID    string                 `json:"request_id"`
	Feature      string                 `json:"feature"`
	Status       string                 `json:"status"`
	ProviderType string                 `json:"provider_type"`
	ProviderID   string                 `json:"provider_id"`
	Result       map[string]interface{} `json:"result,omitempty"`
	Error        *ErrorInfo             `json:"error,omitempty"`

	// 时间信息
	ReceivedAt   time.Time              `json:"received_at"`
	DispatchedAt time.Time              `json:"dispatched_at"`
	StartedAt    time.Time              `json:"started_at"`
	CompletedAt  time.Time              `json:"completed_at"`

	// 计算的时间
	WaitTimeMs   int                    `json:"wait_time_ms"`
	QueueTimeMs  int                    `json:"queue_time_ms"`
	ExecTimeMs   int                    `json:"exec_time_ms"`
	TotalMs      int                    `json:"total_ms"`

	// 资源消耗
	TokensInput  int                    `json:"tokens_input,omitempty"`
	TokensOutput int                    `json:"tokens_output,omitempty"`
	ImageCount   int                    `json:"image_count,omitempty"`

	// 成本
	Cost         float64                `json:"cost"`
}

// ErrorInfo 错误信息
type ErrorInfo struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Details   string `json:"details,omitempty"`
	Retryable bool   `json:"retryable"`
}

// QueueStatsSnapshot 队列统计快照
type QueueStatsSnapshot struct {
	SnapshotID       int64     `json:"snapshot_id" db:"snapshot_id"`
	SnapshotTime     time.Time `json:"snapshot_time" db:"snapshot_time"`
	Feature          string    `json:"feature" db:"feature"`

	// 队列状态
	QueueDepth       int       `json:"queue_depth" db:"queue_depth"`
	QueueWaitMsP50   *int      `json:"queue_wait_ms_p50,omitempty" db:"queue_wait_ms_p50"`
	QueueWaitMsP95   *int      `json:"queue_wait_ms_p95,omitempty" db:"queue_wait_ms_p95"`
	QueueWaitMsP99   *int      `json:"queue_wait_ms_p99,omitempty" db:"queue_wait_ms_p99"`
	QueueWaitMsMax   *int      `json:"queue_wait_ms_max,omitempty" db:"queue_wait_ms_max"`

	// 请求统计
	WaitingRequests  int       `json:"waiting_requests" db:"waiting_requests"`
	ProcessingRequests int     `json:"processing_requests" db:"processing_requests"`
	CompletedRequests int       `json:"completed_requests" db:"completed_requests"`
	FailedRequests   int       `json:"failed_requests" db:"failed_requests"`

	// 执行时间统计
	ExecTimeMsP50    *int      `json:"exec_time_ms_p50,omitempty" db:"exec_time_ms_p50"`
	ExecTimeMsP95    *int      `json:"exec_time_ms_p95,omitempty" db:"exec_time_ms_p95"`
	ExecTimeMsP99    *int      `json:"exec_time_ms_p99,omitempty" db:"exec_time_ms_p99"`

	// 路由分布
	RoutedToSelfHosted int       `json:"routed_to_self_hosted" db:"routed_to_self_hosted"`
	RoutedToThirdParty int       `json:"routed_to_third_party" db:"routed_to_third_party"`
}

// RequestLog 请求日志
type RequestLog struct {
	ID              int64              `json:"id" db:"id"`
	RequestID       string             `json:"request_id" db:"request_id"`
	Feature         string             `json:"feature" db:"feature"`
	ProviderType    string             `json:"provider_type" db:"provider_type"`
	ProviderID      string             `json:"provider_id" db:"provider_id"`
	APIKeyID        string             `json:"api_key_id" db:"api_key_id"`

	// 请求内容
	PromptHash      string             `json:"prompt_hash" db:"prompt_hash"`
	PromptLength    int                `json:"prompt_length" db:"prompt_length"`
	Parameters      string             `json:"parameters" db:"parameters"` // JSON

	// 时间记录
	ReceivedAt      time.Time          `json:"received_at" db:"received_at"`
	DispatchedAt    *time.Time         `json:"dispatched_at,omitempty" db:"dispatched_at"`
	StartedAt       *time.Time         `json:"started_at,omitempty" db:"started_at"`
	CompletedAt     *time.Time         `json:"completed_at,omitempty" db:"completed_at"`

	// 计算的时间指标
	WaitTimeMs      *int               `json:"wait_time_ms,omitempty" db:"wait_time_ms"`
	QueueTimeMs     *int               `json:"queue_time_ms,omitempty" db:"queue_time_ms"`
	ExecTimeMs      *int               `json:"exec_time_ms,omitempty" db:"exec_time_ms"`
	TotalLatencyMs  *int               `json:"total_latency_ms,omitempty" db:"total_latency_ms"`

	// 结果
	Status          string             `json:"status" db:"status"`
	ErrorCode       string             `json:"error_code,omitempty" db:"error_code"`
	ErrorMessage    string             `json:"error_message,omitempty" db:"error_message"`

	// 资源消耗
	TokensInput     int                `json:"tokens_input" db:"tokens_input"`
	TokensOutput    int                `json:"tokens_output" db:"tokens_output"`
	ImageCount      int                `json:"image_count" db:"image_count"`

	// 成本
	CostCompute     float64            `json:"cost_compute" db:"cost_compute"`
	CostAPI         float64            `json:"cost_api" db:"cost_api"`
	CostTotal       float64            `json:"cost_total" db:"cost_total"`

	// 元数据
	TenantID        string             `json:"tenant_id,omitempty" db:"tenant_id"`
	UserID          string             `json:"user_id,omitempty" db:"user_id"`
	TraceID         string             `json:"trace_id,omitempty" db:"trace_id"`

	CreatedAt       time.Time          `json:"created_at" db:"created_at"`
}

// ConfigChangeLog 配置变更日志
type ConfigChangeLog struct {
	ID            int64              `json:"id" db:"id"`
	ConfigType    string             `json:"config_type" db:"config_type"`
	ConfigID      string             `json:"config_id" db:"config_id"`
	Action        string             `json:"action" db:"action"`
	OldValue      string             `json:"old_value,omitempty" db:"old_value"` // JSON
	NewValue      string             `json:"new_value,omitempty" db:"new_value"` // JSON
	ChangedFields string             `json:"changed_fields,omitempty" db:"changed_fields"` // JSON
	ChangedBy     string             `json:"changed_by" db:"changed_by"`
	ChangeReason  string             `json:"change_reason,omitempty" db:"change_reason"`
	ApprovedBy    string             `json:"approved_by,omitempty" db:"approved_by"`
	ApprovedAt    *time.Time         `json:"approved_at,omitempty" db:"approved_at"`
	CreatedAt     time.Time          `json:"created_at" db:"created_at"`
}

// CostStatistics 成本统计
type CostStatistics struct {
	ID                int64     `json:"id" db:"id"`
	StatisticDate     string    `json:"statistic_date" db:"statistic_date"` // DATE format
	Feature           string    `json:"feature" db:"feature"`
	ProviderType      string    `json:"provider_type" db:"provider_type"`
	ProviderID        string    `json:"provider_id,omitempty" db:"provider_id"`
	TenantID          string    `json:"tenant_id,omitempty" db:"tenant_id"`

	// 量统计
	RequestCount      int       `json:"request_count" db:"request_count"`
	SuccessCount      int       `json:"success_count" db:"success_count"`
	FailedCount       int       `json:"failed_count" db:"failed_count"`

	// 资源统计
	TotalTokensInput  int64     `json:"total_tokens_input" db:"total_tokens_input"`
	TotalTokensOutput int64     `json:"total_tokens_output" db:"total_tokens_output"`
	TotalImages       int64     `json:"total_images" db:"total_images"`
	TotalGPUSeconds   int64     `json:"total_gpu_seconds" db:"total_gpu_seconds"`

	// 成本统计
	CostCompute       float64   `json:"cost_compute" db:"cost_compute"`
	CostAPI           float64   `json:"cost_api" db:"cost_api"`
	CostStorage       float64   `json:"cost_storage" db:"cost_storage"`
	CostNetwork       float64   `json:"cost_network" db:"cost_network"`
	CostTotal         float64   `json:"cost_total" db:"cost_total"`

	UpdatedAt         time.Time `json:"updated_at" db:"updated_at"`
}
