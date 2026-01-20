package router

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/yijian/ai-platform/pkg/model"
	"github.com/yijian/ai-platform/pkg/provider"
)

// Engine 路由引擎
type Engine struct {
	configStore     ConfigStore
	registry        ServiceRegistry
	keyManager      KeyManager
	providerFactory  *provider.Factory
	costTracker     CostTracker
	mu              sync.RWMutex
}

// ConfigStore 配置存储接口
type ConfigStore interface {
	GetFeature(id string) (*model.Feature, error)
	GetFeatureByCategory(category string) ([]*model.Feature, error)
}

// ServiceRegistry 服务注册接口
type ServiceRegistry interface {
	GetHealthyServices(serviceType string) ([]*model.RegisteredService, error)
}

// KeyManager 密钥管理接口
type KeyManager interface {
	GetActiveKey(vendor, service string) (*model.APIKey, error)
	GetPlaintextKey(key *model.APIKey) (string, error)
	RecordUsage(keyID string, usage *model.KeyUsageRecord) error
}

// CostTracker 成本追踪接口
type CostTracker interface {
	RecordCost(requestID string, cost float64) error
}

// KeyUsageRecord 密钥使用记录
type KeyUsageRecord struct {
	KeyID        string
	RequestID    string
	Feature      string
	TokensInput  int
	TokensOutput int
	ImageCount   int
	Cost         float64
}

// NewEngine 创建路由引擎
func NewEngine(
	configStore ConfigStore,
	registry ServiceRegistry,
	keyManager KeyManager,
	providerFactory *provider.Factory,
	costTracker CostTracker,
) *Engine {
	return &Engine{
		configStore:     configStore,
		registry:        registry,
		keyManager:      keyManager,
		providerFactory: providerFactory,
		costTracker:     costTracker,
	}
}

// Route 路由请求
func (e *Engine) Route(ctx context.Context, feature string, params map[string]interface{}) (*model.InferenceResponse, error) {
	// 获取功能配置
	featureConfig, err := e.getFeatureConfig(feature)
	if err != nil {
		return nil, fmt.Errorf("get feature config failed: %w", err)
	}

	// 过滤可用的Providers
	availableProviders := e.filterAvailableProviders(featureConfig)
	if len(availableProviders) == 0 {
		return nil, fmt.Errorf("no available provider for feature: %s", feature)
	}

	// 选择Provider
	selectedProvider := e.selectProvider(featureConfig, availableProviders)

	// 执行请求
	resp, err := e.executeRequest(ctx, feature, selectedProvider, params)
	if err != nil {
		// 尝试fallback
		if featureConfig.Routing != nil && featureConfig.Routing.FallbackEnabled {
			for _, p := range availableProviders {
				if p.ID != selectedProvider.ID {
					resp, err = e.executeRequest(ctx, feature, p, params)
					if err == nil {
						resp.FallbackUsed = true
						break
					}
				}
			}
		}
		if err != nil {
			return nil, err
		}
	}

	return resp, nil
}

// getFeatureConfig 获取功能配置
func (e *Engine) getFeatureConfig(feature string) (*model.Feature, error) {
	// 先尝试直接获取
	f, err := e.configStore.GetFeature(feature)
	if err == nil {
		return f, nil
	}

	// 尝试按类别获取
	features, err := e.configStore.GetFeatureByCategory(feature)
	if err != nil || len(features) == 0 {
		return nil, fmt.Errorf("feature not found: %s", feature)
	}

	return features[0], nil
}

// filterAvailableProviders 过滤可用的Provider
func (e *Engine) filterAvailableProviders(feature *model.Feature) []*model.ProviderConfig {
	var available []*model.ProviderConfig

	for _, p := range feature.Providers {
		if !p.Enabled {
			continue
		}

		// 检查自研服务是否有可用实例
		if p.Type == "self_hosted" {
			services, err := e.registry.GetHealthyServices(feature.Category)
			if err != nil || len(services) == 0 {
				continue
			}
		}

		// 检查第三方API是否有可用密钥
		if p.Type == "third_party" {
			_, err := e.keyManager.GetActiveKey(p.Vendor, p.Service)
			if err != nil {
				continue
			}
		}

		available = append(available, p)
	}

	return available
}

// selectProvider 选择Provider
func (e *Engine) selectProvider(feature *model.Feature, providers []*model.ProviderConfig) *model.ProviderConfig {
	if feature.Routing == nil {
		// 默认按优先级
		return e.selectByPriority(providers)
	}

	switch feature.Routing.Strategy {
	case "weighted":
		return e.selectByWeight(providers)
	case "priority":
		return e.selectByPriority(providers)
	case "cost_based":
		return e.selectByCost(feature, providers)
	default:
		return e.selectByPriority(providers)
	}
}

// selectByPriority 按优先级选择
func (e *Engine) selectByPriority(providers []*model.ProviderConfig) *model.ProviderConfig {
	if len(providers) == 0 {
		return nil
	}

	minPriority := providers[0].Priority
	for _, p := range providers {
		if p.Priority < minPriority {
			minPriority = p.Priority
		}
	}

	// 从最高优先级中随机选择
	var highPriorityProviders []*model.ProviderConfig
	for _, p := range providers {
		if p.Priority == minPriority {
			highPriorityProviders = append(highPriorityProviders, p)
		}
	}

	if len(highPriorityProviders) == 1 {
		return highPriorityProviders[0]
	}

	return highPriorityProviders[rand.Intn(len(highPriorityProviders))]
}

// selectByWeight 按权重选择
func (e *Engine) selectByWeight(providers []*model.ProviderConfig) *model.ProviderConfig {
	if len(providers) == 0 {
		return nil
	}

	totalWeight := 0
	for _, p := range providers {
		totalWeight += p.Weight
	}

	if totalWeight == 0 {
		return providers[0]
	}

	r := rand.Intn(totalWeight)
	for _, p := range providers {
		r -= p.Weight
		if r < 0 {
			return p
		}
	}

	return providers[len(providers)-1]
}

// selectByCost 按成本选择
func (e *Engine) selectByCost(feature *model.Feature, providers []*model.ProviderConfig) *model.ProviderConfig {
	// 优先使用自研服务（成本更低）
	for _, p := range providers {
		if p.Type == "self_hosted" {
			return p
		}
	}

	// 没有自研服务，选择第三方中成本最低的
	if feature.Cost == nil {
		return providers[0]
	}

	minCost := math.MaxFloat64
	var selected *model.ProviderConfig
	for _, p := range providers {
		if cost, ok := feature.Cost.ThirdPartyPerRequest[p.ID]; ok {
			if cost < minCost {
				minCost = cost
				selected = p
			}
		}
	}

	if selected != nil {
		return selected
	}

	return providers[0]
}

// executeRequest 执行请求
func (e *Engine) executeRequest(ctx context.Context, feature string, provider *model.ProviderConfig, params map[string]interface{}) (*model.InferenceResponse, error) {
	startTime := time.Now()
	resp := &model.InferenceResponse{
		RequestID:    generateRequestID(),
		Feature:      feature,
		ProviderType: provider.Type,
		ProviderID:   provider.ID,
		ReceivedAt:   startTime,
	}

	if provider.Type == "self_hosted" {
		return e.executeSelfHosted(ctx, resp, provider, params)
	}

	return e.executeThirdParty(ctx, resp, provider, params)
}

// executeSelfHosted 执行自研服务请求
func (e *Engine) executeSelfHosted(ctx context.Context, resp *model.InferenceResponse, provider *model.ProviderConfig, params map[string]interface{}) (*model.InferenceResponse, error) {
	// 获取健康的服务实例
	services, err := e.registry.GetHealthyServices(resp.Feature)
	if err != nil || len(services) == 0 {
		return nil, fmt.Errorf("no healthy service available")
	}

	// 选择负载最低的服务
	selectedService := services[0]
	minLoad := services[0].CurrentLoad
	for _, s := range services {
		if s.CurrentLoad < minLoad {
			minLoad = s.CurrentLoad
			selectedService = s
		}
	}

	// TODO: 调用自研服务
	// 这里需要根据实际的自研服务接口进行调用
	resp.CompletedAt = time.Now()
	resp.ExecTimeMs = int(time.Since(resp.ReceivedAt).Milliseconds())
	resp.Status = "success"

	return resp, nil
}

// executeThirdParty 执行第三方API请求
func (e *Engine) executeThirdParty(ctx context.Context, resp *model.InferenceResponse, provider *model.ProviderConfig, params map[string]interface{}) (*model.InferenceResponse, error) {
	// 获取API密钥
	apiKey, err := e.keyManager.GetActiveKey(provider.Vendor, resp.Feature)
	if err != nil {
		return nil, fmt.Errorf("get API key failed: %w", err)
	}

	// 创建Provider客户端
	client, err := e.providerFactory.CreateWithConfig(provider.Vendor, &provider.Config{
		APIKey: "",
	})
	if err != nil {
		return nil, fmt.Errorf("create provider failed: %w", err)
	}
	defer client.Close()

	// 根据功能类型调用相应接口
	switch resp.Feature {
	case "text_to_image":
		return e.generateImage(ctx, resp, client, params)
	case "text_generation":
		return e.generateText(ctx, resp, client, params)
	case "image_editing":
		return e.editImage(ctx, resp, client, params)
	case "image_stylization":
		return e.stylizeImage(ctx, resp, client, params)
	default:
		return nil, fmt.Errorf("unsupported feature: %s", resp.Feature)
	}
}

// generateImage 图像生成
func (e *Engine) generateImage(ctx context.Context, resp *model.InferenceResponse, client provider.LLMProvider, params map[string]interface{}) (*model.InferenceResponse, error) {
	req := &provider.ImageRequest{
		Prompt:       getString(params, "prompt"),
		NegativePrompt: getString(params, "negative_prompt"),
		Width:        getInt(params, "width", 1024),
		Height:       getInt(params, "height", 1024),
		Steps:        getInt(params, "steps", 50),
		CFGScale:     getFloat64(params, "cfg_scale", 7.5),
		Count:        getInt(params, "count", 1),
	}

	imageResp, err := client.GenerateImage(ctx, req)
	if err != nil {
		return nil, err
	}

	resp.CompletedAt = time.Now()
	resp.ExecTimeMs = int(time.Since(resp.ReceivedAt).Milliseconds())
	resp.Status = "success"
	resp.Result = map[string]interface{}{
		"images": imageResp.Images,
	}
	resp.ImageCount = len(imageResp.Images)

	return resp, nil
}

// generateText 文本生成
func (e *Engine) generateText(ctx context.Context, resp *model.InferenceResponse, client provider.LLMProvider, params map[string]interface{}) (*model.InferenceResponse, error) {
	req := &provider.TextRequest{
		Prompt:      getString(params, "prompt"),
		MaxTokens:   getInt(params, "max_tokens", 1000),
		Temperature: getFloat64(params, "temperature", 0.7),
		TopP:        getFloat64(params, "top_p", 1.0),
	}

	textResp, err := client.GenerateText(ctx, req)
	if err != nil {
		return nil, err
	}

	resp.CompletedAt = time.Now()
	resp.ExecTimeMs = int(time.Since(resp.ReceivedAt).Milliseconds())
	resp.Status = "success"
	resp.Result = map[string]interface{}{
		"text": textResp.Text,
	}
	resp.TokensInput = textResp.TokensInput
	resp.TokensOutput = textResp.TokensOutput

	return resp, nil
}

// editImage 图像编辑
func (e *Engine) editImage(ctx context.Context, resp *model.InferenceResponse, client provider.LLMProvider, params map[string]interface{}) (*model.InferenceResponse, error) {
	req := &provider.ImageEditRequest{
		Image:    getString(params, "image"),
		Mask:     getString(params, "mask"),
		Prompt:   getString(params, "prompt"),
		Width:    getInt(params, "width", 0),
		Height:   getInt(params, "height", 0),
		Steps:    getInt(params, "steps", 50),
		CFGScale: getFloat64(params, "cfg_scale", 7.5),
		Count:    getInt(params, "count", 1),
	}

	imageResp, err := client.EditImage(ctx, req)
	if err != nil {
		return nil, err
	}

	resp.CompletedAt = time.Now()
	resp.ExecTimeMs = int(time.Since(resp.ReceivedAt).Milliseconds())
	resp.Status = "success"
	resp.Result = map[string]interface{}{
		"images": imageResp.Images,
	}
	resp.ImageCount = len(imageResp.Images)

	return resp, nil
}

// stylizeImage 图像风格化
func (e *Engine) stylizeImage(ctx context.Context, resp *model.InferenceResponse, client provider.LLMProvider, params map[string]interface{}) (*model.InferenceResponse, error) {
	req := &provider.ImageStylizationRequest{
		Image:    getString(params, "image"),
		Style:    getString(params, "style"),
		Strength: getFloat64(params, "strength", 0.8),
	}

	imageResp, err := client.StylizeImage(ctx, req)
	if err != nil {
		return nil, err
	}

	resp.CompletedAt = time.Now()
	resp.ExecTimeMs = int(time.Since(resp.ReceivedAt).Milliseconds())
	resp.Status = "success"
	resp.Result = map[string]interface{}{
		"images": imageResp.Images,
	}
	resp.ImageCount = len(imageResp.Images)

	return resp, nil
}

// 辅助函数
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getInt(m map[string]interface{}, key string, defaultVal int) int {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case int:
			return val
		case float64:
			return int(val)
		case int32:
			return int(val)
		case int64:
			return int(val)
		}
	}
	return defaultVal
}

func getFloat64(m map[string]interface{}, key string, defaultVal float64) float64 {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case float64:
			return val
		case float32:
			return float64(val)
		case int:
			return float64(val)
		}
	}
	return defaultVal
}

func generateRequestID() string {
	return fmt.Sprintf("req-%d", time.Now().UnixNano())
}
