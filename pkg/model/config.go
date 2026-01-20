package model

import "time"

// Feature 功能定义
type Feature struct {
	ID          string         `json:"id" db:"id"`
	Name        string         `json:"name" db:"name"`
	Category    string         `json:"category" db:"category"`       // image_generation, text_generation, image_editing
	Description string         `json:"description" db:"description"`
	Tags        []string       `json:"tags" db:"-"`
	Enabled     bool           `json:"enabled" db:"enabled"`
	CreatedAt   time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at" db:"updated_at"`
	Version     int            `json:"version" db:"version"`

	// 关联的Providers
	Providers  []*ProviderConfig `json:"providers" db:"-"`
	Routing    *RoutingStrategy  `json:"routing" db:"-"`
	Cost       *CostConfig       `json:"cost" db:"-"`
	Metadata   *FeatureMetadata  `json:"metadata" db:"-"`
}

// FeatureMetadata 功能元数据
type FeatureMetadata struct {
	InputSchema   map[string]interface{} `json:"input_schema"`
	OutputSchema  map[string]interface{} `json:"output_schema"`
	ExampleParams map[string]interface{} `json:"example_params"`
}

// ProviderConfig 提供者配置
type ProviderConfig struct {
	ID            string                 `json:"id" db:"id"`
	FeatureID     string                 `json:"feature_id" db:"feature_id"`
	Type          string                 `json:"type" db:"type"`           // self_hosted, third_party
	Vendor        string                 `json:"vendor" db:"vendor"`       // openai, aliyun, etc.
	Enabled       bool                   `json:"enabled" db:"enabled"`
	Priority      int                    `json:"priority" db:"priority"`   // 1=最高
	Weight        int                    `json:"weight" db:"weight"`       // 流量分配权重

	// 自研镜像配置
	Image                string            `json:"image,omitempty" db:"image"`
	MinInstances         int32             `json:"min_instances,omitempty" db:"min_instances"`
	MaxInstances         int32             `json:"max_instances,omitempty" db:"max_instances"`
	ResourceRequirements  *ResourceSpec    `json:"resource_requirements,omitempty" db:"-"`
	CapabilityMatch      []string          `json:"capability_match,omitempty" db:"capability_match"`

	// 第三方API配置
	Model          string            `json:"model,omitempty" db:"model"`
	APIKeyRef      string            `json:"api_key_ref,omitempty" db:"api_key_ref"`
	RateLimit      *RateLimit        `json:"rate_limit,omitempty" db:"-"`
	Endpoint       string            `json:"endpoint,omitempty" db:"endpoint"`

	// 扩展配置
	Extra map[string]interface{} `json:"extra,omitempty" db:"-"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// ResourceSpec 资源规格
type ResourceSpec struct {
	GPUMemory string `json:"gpu_memory"`
	GPUCount  int32  `json:"gpu_count"`
	CPU       string `json:"cpu"`
	Memory    string `json:"memory"`
}

// RateLimit 速率限制
type RateLimit struct {
	RPM       int32 `json:"rpm"`        // Requests Per Minute
	TPM       int32 `json:"tpm"`        // Tokens Per Minute
	Concurrent int32 `json:"concurrent"` // 最大并发数
}

// RoutingStrategy 路由策略
type RoutingStrategy struct {
	Strategy        string `json:"strategy"`         // weighted, priority, cost_based
	FallbackEnabled bool   `json:"fallback_enabled"`
	Timeout         int32  `json:"timeout"`          // 超时时间(秒)
	MaxRetries      int32  `json:"max_retries"`
	RetryBackoff    string `json:"retry_backoff"`    // linear, exponential
}

// CostConfig 成本配置
type CostConfig struct {
	SelfHostedPerHour     float64            `json:"self_hosted_per_hour"`
	ThirdPartyPerRequest  map[string]float64 `json:"third_party_per_request"`
}

// FeatureFilter 功能查询过滤条件
type FeatureFilter struct {
	Category string `json:"category"`
	Enabled  *bool  `json:"enabled"`
	Limit    int    `json:"limit"`
	Offset   int    `json:"offset"`
}

// CreateFeatureRequest 创建功能请求
type CreateFeatureRequest struct {
	ID          string             `json:"id"`
	Name        string             `json:"name" binding:"required"`
	Category    string             `json:"category" binding:"required"`
	Description string             `json:"description"`
	Tags        []string           `json:"tags"`
	Providers   []*ProviderConfig  `json:"providers"`
	Routing     *RoutingStrategy  `json:"routing"`
	Cost        *CostConfig        `json:"cost"`
}

// UpdateFeatureRequest 更新功能请求
type UpdateFeatureRequest struct {
	Name        *string            `json:"name"`
	Description *string            `json:"description"`
	Tags        []string           `json:"tags"`
	Enabled     *bool              `json:"enabled"`
	Routing     *RoutingStrategy   `json:"routing"`
	Cost        *CostConfig        `json:"cost"`
}

// AddProviderRequest 添加Provider请求
type AddProviderRequest struct {
	ID                   string              `json:"id"`
	Type                 string              `json:"type" binding:"required"`
	Vendor               string              `json:"vendor"`
	Enabled              bool                `json:"enabled"`
	Priority             int                 `json:"priority"`
	Weight               int                 `json:"weight"`
	Image                string              `json:"image"`
	MinInstances         int32               `json:"min_instances"`
	MaxInstances         int32               `json:"max_instances"`
	ResourceRequirements *ResourceSpec       `json:"resource_requirements"`
	CapabilityMatch      []string            `json:"capability_match"`
	Model                string              `json:"model"`
	APIKeyRef            string              `json:"api_key_ref"`
	RateLimit            *RateLimit          `json:"rate_limit"`
	Endpoint             string              `json:"endpoint"`
	Extra                map[string]interface{} `json:"extra"`
}

// UpdateProviderRequest 更新Provider请求
type UpdateProviderRequest struct {
	Enabled  *bool              `json:"enabled"`
	Priority *int               `json:"priority"`
	Weight   *int               `json:"weight"`
	Extra    map[string]interface{} `json:"extra"`
}
