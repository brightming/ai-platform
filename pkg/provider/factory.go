package provider

import (
	"fmt"
)

// Factory 提供者工厂
type Factory struct {
	keys map[string]string // vendor -> api_key
}

// NewFactory 创建提供者工厂
func NewFactory() *Factory {
	return &Factory{
		keys: make(map[string]string),
	}
}

// SetKey 设置密钥
func (f *Factory) SetKey(vendor, key string) {
	f.keys[vendor] = key
}

// SetKeys 批量设置密钥
func (f *Factory) SetKeys(keys map[string]string) {
	for k, v := range keys {
		f.keys[k] = v
	}
}

// Create 创建提供者
func (f *Factory) Create(vendor string) (LLMProvider, error) {
	apiKey, ok := f.keys[vendor]
	if !ok {
		return nil, fmt.Errorf("no API key for vendor: %s", vendor)
	}

	cfg := &Config{
		APIKey: apiKey,
	}

	return f.CreateWithConfig(vendor, cfg)
}

// CreateWithConfig 使用指定配置创建提供者
func (f *Factory) CreateWithConfig(vendor string, cfg *Config) (LLMProvider, error) {
	if cfg.APIKey == "" {
		apiKey, ok := f.keys[vendor]
		if !ok {
			return nil, fmt.Errorf("no API key for vendor: %s", vendor)
		}
		cfg.APIKey = apiKey
	}

	switch vendor {
	case "openai":
		return NewOpenAIClient(cfg), nil
	case "aliyun":
		return NewAliyunClient(cfg), nil
	default:
		return nil, fmt.Errorf("unsupported vendor: %s", vendor)
	}
}

// VendorNames 支持的厂商列表
var VendorNames = []string{"openai", "aliyun"}
