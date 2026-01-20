package model

import "time"

// APIKey API密钥
type APIKey struct {
	ID            string    `json:"id" db:"id"`
	Vendor        string    `json:"vendor" db:"vendor"`           // openai, aliyun, etc.
	Service       string    `json:"service" db:"service"`         // dalle, gpt-4, wanx, etc.

	// 加密字段
	EncryptedDEK  string    `json:"-" db:"encrypted_dek"`        // KMS加密后的数据密钥
	EncryptedKey  string    `json:"-" db:"encrypted_key"`        // 用DEK加密后的实际API Key

	// 元数据
	KeyHash       string    `json:"key_hash" db:"key_hash"`      // SHA256 for verification
	KeyAlias      string    `json:"key_alias" db:"key_alias"`
	Tier          string    `json:"tier" db:"tier"`              // primary, backup, overflow

	// 配额
	QuotaDailyRequests int     `json:"quota_daily_requests" db:"quota_daily_requests"`
	QuotaDailyTokens   int64   `json:"quota_daily_tokens" db:"quota_daily_tokens"`
	QuotaMonthlyRequests int   `json:"quota_monthly_requests" db:"quota_monthly_requests"`

	// 状态
	Enabled       bool       `json:"enabled" db:"enabled"`
	AutoRotate    bool       `json:"auto_rotate" db:"auto_rotate"`
	RotateDays    int        `json:"rotate_days" db:"rotate_days"`
	LastRotatedAt *time.Time `json:"last_rotated_at" db:"last_rotated_at"`
	LastUsedAt    *time.Time `json:"last_used_at" db:"last_used_at"`
	ExpiresAt     *time.Time `json:"expires_at" db:"expires_at"`

	// 审计
	CreatedBy     string     `json:"created_by" db:"created_by"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at" db:"updated_at"`

	// 运行时字段（不存数据库）
	Usage         *KeyUsage  `json:"usage,omitempty" db:"-"`
	HealthStatus  string     `json:"health_status,omitempty" db:"-"`
}

// KeyUsage 密钥使用统计
type KeyUsage struct {
	DayRequests    int64    `json:"day_requests"`
	DayTokens      int64    `json:"day_tokens"`
	MonthRequests int64    `json:"month_requests"`
	DayCost        float64  `json:"day_cost"`
	MonthCost      float64  `json:"month_cost"`
	LastResetTime  time.Time `json:"last_reset_time"`
}

// APIKeyUsageLog 密钥使用记录（永久保存）
type APIKeyUsageLog struct {
	ID            int64     `json:"id" db:"id"`
	KeyID         string    `json:"key_id" db:"key_id"`
	RequestID     string    `json:"request_id" db:"request_id"`

	// 请求信息
	Feature       string    `json:"feature" db:"feature"`
	RequestSize   int       `json:"request_size" db:"request_size"`
	ResponseSize  int       `json:"response_size" db:"response_size"`

	// 结果
	Status        string    `json:"status" db:"status"`     // success, error, rate_limited
	ErrorCode     string    `json:"error_code,omitempty" db:"error_code"`
	LatencyMs     int       `json:"latency_ms" db:"latency_ms"`

	// 成本
	CostAmount    float64   `json:"cost_amount" db:"cost_amount"`
	CostCurrency  string    `json:"cost_currency" db:"cost_currency"`

	// 时间
	RequestedAt   time.Time `json:"requested_at" db:"requested_at"`
	CompletedAt   *time.Time `json:"completed_at,omitempty" db:"completed_at"`
}

// KeyFilter 密钥查询过滤条件
type KeyFilter struct {
	Vendor  string `json:"vendor"`
	Service string `json:"service"`
	Enabled *bool  `json:"enabled"`
	Tier    string `json:"tier"`
	Limit   int    `json:"limit"`
	Offset  int    `json:"offset"`
}

// CreateKeyRequest 创建密钥请求
type CreateKeyRequest struct {
	ID            string   `json:"id"`
	Vendor        string   `json:"vendor" binding:"required"`
	Service       string   `json:"service" binding:"required"`
	KeyAlias      string   `json:"key_alias"`
	Tier          string   `json:"tier"`           // primary, backup, overflow
	APIKey        string   `json:"api_key" binding:"required"`  // 明文密钥，仅创建时使用
	AutoRotate    bool     `json:"auto_rotate"`
	RotateDays    int      `json:"rotate_days"`
	ExpiresAt     *time.Time `json:"expires_at"`

	// 配额
	QuotaDailyRequests int     `json:"quota_daily_requests"`
	QuotaDailyTokens   int64   `json:"quota_daily_tokens"`
	QuotaMonthlyRequests int   `json:"quota_monthly_requests"`
}

// UpdateKeyRequest 更新密钥请求
type UpdateKeyRequest struct {
	KeyAlias      *string    `json:"key_alias"`
	Tier          *string    `json:"tier"`
	Enabled       *bool      `json:"enabled"`
	AutoRotate    *bool      `json:"auto_rotate"`
	RotateDays    *int       `json:"rotate_days"`
	ExpiresAt     *time.Time `json:"expires_at"`

	// 配额更新
	QuotaDailyRequests *int   `json:"quota_daily_requests"`
	QuotaDailyTokens   *int64 `json:"quota_daily_tokens"`
	QuotaMonthlyRequests *int `json:"quota_monthly_requests"`
}

// RotateKeyRequest 轮换密钥请求
type RotateKeyRequest struct {
	NewAPIKey string `json:"new_key,omitempty"` // 留空表示保持相同密钥
}

// HealthStatus 健康状态
type HealthStatus struct {
	KeyID        string    `json:"key_id"`
	Status       string    `json:"status"`        // healthy, degraded, unhealthy
	LastCheckAt  time.Time `json:"last_check_at"`
	LatencyMs    int       `json:"latency_ms"`
	ErrorMessage string    `json:"error_message,omitempty"`
}

// UsageStats 使用统计
type UsageStats struct {
	KeyID           string  `json:"key_id"`
	Period          string  `json:"period"`           // daily, monthly
	TotalRequests   int64   `json:"total_requests"`
	SuccessRequests int64   `json:"success_requests"`
	FailedRequests  int64   `json:"failed_requests"`
	TotalTokens     int64   `json:"total_tokens"`
	TotalImages     int64   `json:"total_images"`
	TotalCost       float64 `json:"total_cost"`
	AvgLatencyMs    float64 `json:"avg_latency_ms"`
	P95LatencyMs    int     `json:"p95_latency_ms"`
	P99LatencyMs    int     `json:"p99_latency_ms"`
}
