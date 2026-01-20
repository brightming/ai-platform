package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	openaiDefaultEndpoint = "https://api.openai.com/v1"
	openaiDefaultTimeout  = 60 * time.Second
)

// OpenAIClient OpenAI 客户端
type OpenAIClient struct {
	config     *Config
	httpClient *http.Client
}

// NewOpenAIClient 创建 OpenAI 客户端
func NewOpenAIClient(cfg *Config) *OpenAIClient {
	if cfg.Endpoint == "" {
		cfg.Endpoint = openaiDefaultEndpoint
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = int(openaiDefaultTimeout.Seconds())
	}

	return &OpenAIClient{
		config: cfg,
		httpClient: &http.Client{
			Timeout: time.Duration(cfg.Timeout) * time.Second,
		},
	}
}

// GenerateText 文本生成 (GPT)
func (c *OpenAIClient) GenerateText(ctx context.Context, req *TextRequest) (*TextResponse, error) {
	model := c.config.Model
	if model == "" {
		model = "gpt-3.5-turbo"
	}

	body := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": req.Prompt},
		},
		"max_tokens":   req.MaxTokens,
		"temperature": req.Temperature,
		"top_p":       req.TopP,
	}

	var resp openaiChatResponse
	if err := c.doRequest(ctx, http.MethodPost, "/chat/completions", body, &resp); err != nil {
		return nil, err
	}

	if len(resp.Choices) == 0 {
		return nil, &ProviderError{
			Code:      "no_response",
			Message:   "No response from OpenAI",
			Retryable: false,
		}
	}

	return &TextResponse{
		Text:         resp.Choices[0].Message.Content,
		FinishReason: resp.Choices[0].FinishReason,
		TokensInput:  resp.Usage.PromptTokens,
		TokensOutput: resp.Usage.CompletionTokens,
	}, nil
}

// GenerateImage 图像生成 (DALL-E)
func (c *OpenAIClient) GenerateImage(ctx context.Context, req *ImageRequest) (*ImageResponse, error) {
	model := c.config.Model
	if model == "" {
		model = "dall-e-3"
	}

	body := map[string]interface{}{
		"model":          model,
		"prompt":         req.Prompt,
		"n":              req.Count,
		"size":           fmt.Sprintf("%dx%d", req.Width, req.Height),
		"response_format": "url",
	}

	var resp openaiImageResponse
	if err := c.doRequest(ctx, http.MethodPost, "/images/generations", body, &resp); err != nil {
		return nil, err
	}

	images := make([]*ImageResult, len(resp.Data))
	for i, data := range resp.Data {
		images[i] = &ImageResult{
			URL:    data.URL,
			Width:  req.Width,
			Height: req.Height,
		}
	}

	return &ImageResponse{
		Images: images,
	}, nil
}

// EditImage 图像编辑
func (c *OpenAIClient) EditImage(ctx context.Context, req *ImageEditRequest) (*ImageResponse, error) {
	body := map[string]interface{}{
		"image":          req.Image,
		"mask":           req.Mask,
		"prompt":         req.Prompt,
		"n":              req.Count,
		"size":           fmt.Sprintf("%dx%d", req.Width, req.Height),
		"response_format": "url",
	}

	var resp openaiImageResponse
	if err := c.doRequest(ctx, http.MethodPost, "/images/edits", body, &resp); err != nil {
		return nil, err
	}

	images := make([]*ImageResult, len(resp.Data))
	for i, data := range resp.Data {
		images[i] = &ImageResult{
			URL:    data.URL,
			Width:  req.Width,
			Height: req.Height,
		}
	}

	return &ImageResponse{
		Images: images,
	}, nil
}

// StylizeImage 图像风格化 (使用 DALL-E 编辑实现)
func (c *OpenAIClient) StylizeImage(ctx context.Context, req *ImageStylizationRequest) (*ImageResponse, error) {
	// DALL-E 3 不直接支持风格化，使用编辑方式
	stylePrompt := fmt.Sprintf("Apply %s style to this image. %s", req.Style, req.Prompt)

	return c.EditImage(ctx, &ImageEditRequest{
		Image:  req.Image,
		Prompt: stylePrompt,
		Width:  1024,
		Height: 1024,
		Count:  1,
	})
}

// GetCapabilities 获取能力
func (c *OpenAIClient) GetCapabilities(ctx context.Context) (*Capabilities, error) {
	return &Capabilities{
		TextGeneration:   true,
		ImageGeneration:  true,
		ImageEditing:     true,
		ImageStylization: true,
		SupportedModels:   []string{"gpt-3.5-turbo", "gpt-4", "gpt-4-turbo", "dall-e-2", "dall-e-3"},
		SupportedSizes:    [][]int{{256, 256}, {512, 512}, {1024, 1024}},
		RateLimits:        c.config.RateLimit,
		Pricing: &Pricing{
			TextPer1KTokens:    0.0015,
			ImagePerGeneration: 0.04,
		},
		MaxBatchSize: 10,
	}, nil
}

// HealthCheck 健康检查
func (c *OpenAIClient) HealthCheck(ctx context.Context) error {
	_, err := c.GenerateText(ctx, &TextRequest{
		Prompt:    "Hi",
		MaxTokens: 5,
	})
	return err
}

// Close 关闭连接
func (c *OpenAIClient) Close() error {
	c.httpClient.CloseIdleConnections()
	return nil
}

// doRequest 发送HTTP请求
func (c *OpenAIClient) doRequest(ctx context.Context, method, path string, body interface{}, resp interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(jsonData)
	}

	url := c.config.Endpoint + path
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)

	httpResp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return err
	}

	if httpResp.StatusCode >= 400 {
		return &ProviderError{
			Code:      fmt.Sprintf("http_%d", httpResp.StatusCode),
			Message:   string(respBody),
			Retryable: httpResp.StatusCode >= 500,
		}
	}

	if resp != nil {
		if err := json.Unmarshal(respBody, resp); err != nil {
			return err
		}
	}

	return nil
}

// openaiChatResponse OpenAI 聊天响应
type openaiChatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// openaiImageResponse OpenAI 图像响应
type openaiImageResponse struct {
	Created int64 `json:"created"`
	Data    []struct {
		URL           string `json:"url"`
		RevisedPrompt string `json:"revised_prompt"`
	} `json:"data"`
}
