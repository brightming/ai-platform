package provider

import (
	"context"
	"io"
)

// LLMProvider 大语言模型提供者接口
type LLMProvider interface {
	// GenerateText 文本生成
	GenerateText(ctx context.Context, req *TextRequest) (*TextResponse, error)

	// GenerateImage 图像生成
	GenerateImage(ctx context.Context, req *ImageRequest) (*ImageResponse, error)

	// EditImage 图像编辑
	EditImage(ctx context.Context, req *ImageEditRequest) (*ImageResponse, error)

	// StylizeImage 图像风格化
	StylizeImage(ctx context.Context, req *ImageStylizationRequest) (*ImageResponse, error)

	// GetCapabilities 获取能力
	GetCapabilities(ctx context.Context) (*Capabilities, error)

	// HealthCheck 健康检查
	HealthCheck(ctx context.Context) error

	// Close 关闭连接
	Close() error
}

// TextRequest 文本生成请求
type TextRequest struct {
	Prompt      string  `json:"prompt"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
	TopP        float64 `json:"top_p,omitempty"`
	TopK        int     `json:"top_k,omitempty"`
	Stop        []string `json:"stop,omitempty"`
	Stream      bool    `json:"stream,omitempty"`
}

// TextResponse 文本生成响应
type TextResponse struct {
	Text      string `json:"text"`
	FinishReason string `json:"finish_reason"`
	TokensInput  int    `json:"tokens_input"`
	TokensOutput int    `json:"tokens_output"`
}

// ImageRequest 图像生成请求
type ImageRequest struct {
	Prompt       string `json:"prompt"`
	NegativePrompt string `json:"negative_prompt,omitempty"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	Steps        int    `json:"steps,omitempty"`
	CFGScale     float64 `json:"cfg_scale,omitempty"`
	Seed         *int64 `json:"seed,omitempty"`
	Count        int    `json:"count,omitempty"`
	Model        string `json:"model,omitempty"`
}

// ImageResponse 图像生成/编辑响应
type ImageResponse struct {
	Images      []*ImageResult `json:"images"`
	Parameters  string         `json:"parameters"`
	TokensUsed  int            `json:"tokens_used,omitempty"`
}

// ImageResult 单个图像结果
type ImageResult struct {
	URL         string `json:"url,omitempty"`
	Base64Data  string `json:"b64_json,omitempty"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	Seed        *int64 `json:"seed,omitempty"`
}

// ImageEditRequest 图像编辑请求
type ImageEditRequest struct {
	Image          string  `json:"image"`           // base64 or URL
	Mask           string  `json:"mask,omitempty"`  // base64 or URL
	Prompt         string  `json:"prompt"`
	NegativePrompt string  `json:"negative_prompt,omitempty"`
	Width          int     `json:"width,omitempty"`
	Height         int     `json:"height,omitempty"`
	Steps          int     `json:"steps,omitempty"`
	CFGScale       float64 `json:"cfg_scale,omitempty"`
	Count          int     `json:"count,omitempty"`
}

// ImageStylizationRequest 图像风格化请求
type ImageStylizationRequest struct {
	Image    string  `json:"image"`    // base64 or URL
	Style    string  `json:"style"`
	Strength float64 `json:"strength,omitempty"`
}

// Capabilities 能力描述
type Capabilities struct {
	TextGeneration    bool                   `json:"text_generation"`
	ImageGeneration   bool                   `json:"image_generation"`
	ImageEditing      bool                   `json:"image_editing"`
	ImageStylization  bool                   `json:"image_stylization"`
	SupportedModels   []string               `json:"supported_models"`
	SupportedSizes    [][]int                `json:"supported_sizes"`
	RateLimits        *RateLimits            `json:"rate_limits,omitempty"`
	Pricing           *Pricing               `json:"pricing,omitempty"`
	MaxBatchSize      int                    `json:"max_batch_size"`
	Custom            map[string]interface{} `json:"custom,omitempty"`
}

// RateLimits 速率限制
type RateLimits struct {
	RPM        int `json:"rpm"`         // Requests Per Minute
	TPM        int `json:"tpm"`         // Tokens Per Minute
	Concurrent int `json:"concurrent"`  // 最大并发数
}

// Pricing 价格信息
type Pricing struct {
	TextPer1KTokens     float64 `json:"text_per_1k_tokens"`
	ImagePerGeneration  float64 `json:"image_per_generation"`
	ImagePerEdit        float64 `json:"image_per_edit"`
}

// Config 提供者配置
type Config struct {
	APIKey      string
	Endpoint    string
	Model       string
	Timeout     int // 秒
	MaxRetries  int
	RateLimit   *RateLimits
}

// ProviderError 提供者错误
type ProviderError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Type      string `json:"type"`  // rate_limit, invalid_request, auth_error, api_error
	Retryable bool   `json:"retryable"`
}

func (e *ProviderError) Error() string {
	return e.Message
}

// IsRetryable 检查错误是否可重试
func IsRetryable(err error) bool {
	if providerErr, ok := err.(*ProviderError); ok {
		return providerErr.Retryable
	}
	return false
}

// ImageReader 图像读取接口
type ImageReader interface {
	Read() (io.ReadCloser, error)
	GetURL() string
	GetBase64() (string, error)
}
