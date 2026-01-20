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
	alibabaDefaultEndpoint = "https://dashscope.aliyuncs.com/api/v1"
	alibabaDefaultTimeout  = 60 * time.Second
)

// AliyunClient 阿里云客户端
type AliyunClient struct {
	config     *Config
	httpClient *http.Client
}

// NewAliyunClient 创建阿里云客户端
func NewAliyunClient(cfg *Config) *AliyunClient {
	if cfg.Endpoint == "" {
		cfg.Endpoint = alibabaDefaultEndpoint
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = int(alibabaDefaultTimeout.Seconds())
	}

	return &AliyunClient{
		config: cfg,
		httpClient: &http.Client{
			Timeout: time.Duration(cfg.Timeout) * time.Second,
		},
	}
}

// GenerateText 文本生成 (通义千问)
func (c *AliyunClient) GenerateText(ctx context.Context, req *TextRequest) (*TextResponse, error) {
	body := map[string]interface{}{
		"model": c.config.Model,
		"input": map[string]interface{}{
			"messages": []map[string]string{
				{"role": "user", "content": req.Prompt},
			},
		},
		"parameters": map[string]interface{}{
			"max_tokens":   req.MaxTokens,
			"temperature": req.Temperature,
			"top_p":       req.TopP,
		},
	}

	if req.MaxTokens == 0 {
		body["input"].(map[string]interface{})["parameters"].(map[string]interface{})["max_tokens"] = 1500
	}

	var resp qwenResponse
	if err := c.doRequest(ctx, http.MethodPost, "/services/aigc/text-generation/generation", body, &resp); err != nil {
		return nil, err
	}

	if resp.Usage == nil && len(resp.Output.Choices) > 0 {
		return &TextResponse{
			Text:         resp.Output.Choices[0].Message.Content,
			FinishReason: resp.Output.Choices[0].FinishReason,
		}, nil
	}

	return &TextResponse{
		Text:         resp.Output.Text,
		FinishReason: "stop",
		TokensInput:  resp.Usage.InputTokens,
		TokensOutput: resp.Usage.OutputTokens,
	}, nil
}

// GenerateImage 图像生成 (通义万相)
func (c *AliyunClient) GenerateImage(ctx context.Context, req *ImageRequest) (*ImageResponse, error) {
	body := map[string]interface{}{
		"model": "wanx-v1",
		"input": map[string]interface{}{
			"prompt": req.Prompt,
			"n":      req.Count,
			"size":   fmt.Sprintf("%d*%d", req.Width, req.Height),
		},
	}

	if req.NegativePrompt != "" {
		body["input"].(map[string]interface{})["negative_prompt"] = req.NegativePrompt
	}

	var resp wanxResponse
	if err := c.doRequest(ctx, http.MethodPost, "/services/aigc/text2image/image-synthesis", body, &resp); err != nil {
		return nil, err
	}

	images := make([]*ImageResult, len(resp.Output.Results))
	for i, result := range resp.Output.Results {
		images[i] = &ImageResult{
			URL:    result.URL,
			Width:  req.Width,
			Height: req.Height,
		}
	}

	return &ImageResponse{
		Images: images,
	}, nil
}

// EditImage 图像编辑
func (c *AliyunClient) EditImage(ctx context.Context, req *ImageEditRequest) (*ImageResponse, error) {
	body := map[string]interface{}{
		"model": "wanx-v1",
		"input": map[string]interface{}{
			"image_url": req.Image,
			"prompt":    req.Prompt,
		},
	}

	if req.Mask != "" {
		body["input"].(map[string]interface{})["mask_url"] = req.Mask
	}

	var resp wanxResponse
	if err := c.doRequest(ctx, http.MethodPost, "/services/aigc/image-editing/edit", body, &resp); err != nil {
		return nil, err
	}

	images := make([]*ImageResult, len(resp.Output.Results))
	for i, result := range resp.Output.Results {
		images[i] = &ImageResult{
			URL: result.URL,
		}
	}

	return &ImageResponse{
		Images: images,
	}, nil
}

// StylizeImage 图像风格化
func (c *AliyunClient) StylizeImage(ctx context.Context, req *ImageStylizationRequest) (*ImageResponse, error) {
	prompt := c.getStylePrompt(req.Style)

	body := map[string]interface{}{
		"model": "wanx-v1",
		"input": map[string]interface{}{
			"image_url": req.Image,
			"prompt":    prompt + " " + req.Style,
		},
	}

	if req.Strength > 0 {
		body["input"].(map[string]interface{})["strength"] = req.Strength
	}

	var resp wanxResponse
	if err := c.doRequest(ctx, http.MethodPost, "/services/aigc/image-editing/stylize", body, &resp); err != nil {
		return nil, err
	}

	images := make([]*ImageResult, len(resp.Output.Results))
	for i, result := range resp.Output.Results {
		images[i] = &ImageResult{
			URL: result.URL,
		}
	}

	return &ImageResponse{
		Images: images,
	}, nil
}

// GetCapabilities 获取能力
func (c *AliyunClient) GetCapabilities(ctx context.Context) (*Capabilities, error) {
	return &Capabilities{
		TextGeneration:   true,
		ImageGeneration:  true,
		ImageEditing:     true,
		ImageStylization: true,
		SupportedModels:   []string{"qwen-turbo", "qwen-plus", "qwen-max", "wanx-v1"},
		SupportedSizes:    [][]int{{1024, 1024}},
		RateLimits:        c.config.RateLimit,
		Pricing: &Pricing{
			TextPer1KTokens:    0.008,
			ImagePerGeneration: 0.02,
		},
		MaxBatchSize: 4,
	}, nil
}

// HealthCheck 健康检查
func (c *AliyunClient) HealthCheck(ctx context.Context) error {
	_, err := c.GenerateText(ctx, &TextRequest{
		Prompt:    "Hi",
		MaxTokens: 5,
	})
	return err
}

// Close 关闭连接
func (c *AliyunClient) Close() error {
	c.httpClient.CloseIdleConnections()
	return nil
}

// doRequest 发送HTTP请求
func (c *AliyunClient) doRequest(ctx context.Context, method, path string, body interface{}, resp interface{}) error {
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
	req.Header.Set("X-DashScope-SSE", "disable")

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

// getStylePrompt 获取风格化提示词
func (c *AliyunClient) getStylePrompt(style string) string {
	prompts := map[string]string{
		"anime":       "动漫风格，鲜艳色彩，简洁线条",
		"oil_painting": "油画风格，丰富纹理，笔触感",
		"watercolor":   "水彩风格，柔和流动色彩",
		"sketch":       "素描风格，铅笔线条，阴影细节",
		"chinese":      "中国水墨画风格",
		"cyberpunk":    "赛博朋克风格，霓虹灯，未来感",
	}

	if p, ok := prompts[style]; ok {
		return p
	}
	return "应用" + style + "风格"
}

// qwenResponse 通义千问响应
type qwenResponse struct {
	Output struct {
		Text         string `json:"text,omitempty"`
		Choices []struct {
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	} `json:"output"`
	Usage *struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage,omitempty"`
}

// wanxResponse 通义万相响应
type wanxResponse struct {
	Output struct {
		Results []struct {
			URL string `json:"url"`
		} `json:"results"`
	} `json:"output"`
	Usage struct {
		ImageCount int `json:"image_count"`
	} `json:"usage"`
}
