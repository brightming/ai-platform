package scaler

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/brightming/ai-platform/pkg/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Controller GPU实例弹性伸缩控制器
type Controller struct {
	k8sClient    *kubernetes.Clientset
	mu           sync.RWMutex
	scales       map[string]*ScaleConfig   // feature_id -> config
	scaleEvents  chan *ScaleEvent
	registry    ServiceRegistry
	configStore ConfigStore
}

// ScaleConfig 伸缩配置
type ScaleConfig struct {
	FeatureID       string    `json:"feature_id"`
	MinInstances    int32     `json:"min_instances"`
	MaxInstances    int32     `json:"max_instances"`
	TargetCPU       float64   `json:"target_cpu"`        // 目标CPU使用率
	TargetMemory    float64   `json:"target_memory"`     // 目标内存使用率
	TargetQueueSize int       `json:"target_queue_size"` // 目标队列长度
	IdleTimeout     int       `json:"idle_timeout"`      // 空闲超时(秒)
	ScaleUpCooldown int       `json:"scale_up_cooldown"` // 扩容冷却时间(秒)
	ScaleDownCooldown int     `json:"scale_down_cooldown"` // 缩容冷却时间
	LastScaleUp     time.Time `json:"last_scale_up"`
	LastScaleDown   time.Time `json:"last_scale_down"`
	DeploymentName  string    `json:"deployment_name"`
	Namespace       string    `json:"namespace"`
}

// ScaleEvent 伸缩事件
type ScaleEvent struct {
	FeatureID   string    `json:"feature_id"`
	Action      string    `json:"action"`     // scale_up, scale_down, scale_to_zero
	Current     int32     `json:"current"`
	Target      int32     `json:"target"`
	Reason      string    `json:"reason"`
	Timestamp   time.Time `json:"timestamp"`
}

// ServiceRegistry 服务注册接口
type ServiceRegistry interface {
	GetServicesByType(serviceType string) ([]*model.RegisteredService, error)
}

// ConfigStore 配置存储接口
type ConfigStore interface {
	GetFeature(id string) (*model.Feature, error)
}

// NewController 创建伸缩控制器
func NewController(configStore ConfigStore, registry ServiceRegistry) (*Controller, error) {
	// 初始化Kubernetes客户端
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("create k8s client failed: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("create k8s clientset failed: %w", err)
	}

	c := &Controller{
		k8sClient:   clientset,
		scales:      make(map[string]*ScaleConfig),
		scaleEvents: make(chan *ScaleEvent, 100),
		registry:    registry,
		configStore: configStore,
	}

	// 加载伸缩配置
	c.loadScaleConfigs()

	// 启动伸缩循环
	go c.scaleLoop()

	return c, nil
}

// CheckScale 检查是否需要伸缩
func (c *Controller) CheckScale(ctx context.Context, featureID string) (*ScaleDecision, error) {
	c.mu.RLock()
	config, exists := c.scales[featureID]
	c.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no scale config for feature: %s", featureID)
	}

	// 获取当前实例数
	currentReplicas, err := c.getCurrentReplicas(config)
	if err != nil {
		return nil, err
	}

	// 获取服务状态
	services, err := c.registry.GetServicesByType(featureID)
	if err != nil {
		return nil, err
	}

	// 计算指标
	metrics := c.calculateMetrics(services)

	decision := &ScaleDecision{
		FeatureID:       featureID,
		CurrentReplicas: int32(currentReplicas),
		Metrics:         metrics,
	}

	// 判断是否需要扩容
	if c.shouldScaleUp(config, metrics, currentReplicas) {
		// 冷却检查
		if time.Since(config.LastScaleUp) < time.Duration(config.ScaleUpCooldown)*time.Second {
			decision.Reason = "scale up cooldown"
			return decision, nil
		}

		target := min(int32(currentReplicas)+1, config.MaxInstances)
		decision.Action = "scale_up"
		decision.TargetReplicas = target
		decision.Reason = fmt.Sprintf("cpu usage: %.2f%%, queue: %d", metrics.CPUUsage, metrics.QueueSize)

		// 执行扩容
		if err := c.scale(config, int(target)); err == nil {
			config.LastScaleUp = time.Now()
			c.scaleEvents <- &ScaleEvent{
				FeatureID: featureID,
				Action:    "scale_up",
				Current:   int32(currentReplicas),
				Target:    target,
				Reason:    decision.Reason,
				Timestamp: time.Now(),
			}
		}

		return decision, nil
	}

	// 判断是否需要缩容
	if c.shouldScaleDown(config, metrics, currentReplicas) {
		// 冷却检查
		if time.Since(config.LastScaleDown) < time.Duration(config.ScaleDownCooldown)*time.Second {
			decision.Reason = "scale down cooldown"
			return decision, nil
		}

		target := max(int32(currentReplicas)-1, config.MinInstances)
		if target == 0 && currentReplicas > 0 {
			// 全部缩容前检查
			decision.Action = "scale_to_zero"
			decision.TargetReplicas = 0
			decision.Reason = "idle timeout, scale to zero"

			if err := c.scale(config, 0); err == nil {
				config.LastScaleDown = time.Now()
				c.scaleEvents <- &ScaleEvent{
					FeatureID: featureID,
					Action:    "scale_to_zero",
					Current:   int32(currentReplicas),
					Target:    0,
					Reason:    decision.Reason,
					Timestamp: time.Now(),
				}
			}

			return decision, nil
		}

		decision.Action = "scale_down"
		decision.TargetReplicas = target
		decision.Reason = fmt.Sprintf("low utilization: cpu=%.2f%%", metrics.CPUUsage)

		if err := c.scale(config, int(target)); err == nil {
			config.LastScaleDown = time.Now()
			c.scaleEvents <- &ScaleEvent{
				FeatureID: featureID,
				Action:    "scale_down",
				Current:   int32(currentReplicas),
				Target:    target,
				Reason:    decision.Reason,
				Timestamp: time.Now(),
			}
		}

		return decision, nil
	}

	decision.Reason = "no scale needed"
	return decision, nil
}

// ScaleMetrics 伸缩指标
type ScaleMetrics struct {
	CPUUsage      float64 `json:"cpu_usage"`
	MemoryUsage   float64 `json:"memory_usage"`
	GPUUsage      float64 `json:"gpu_usage"`
	QueueSize     int     `json:"queue_size"`
	RequestsPerSec float64 `json:"requests_per_sec"`
	IdleTime      int     `json:"idle_time"` // 空闲时间(秒)
}

// ScaleDecision 伸缩决策
type ScaleDecision struct {
	FeatureID        string       `json:"feature_id"`
	Action           string       `json:"action"`     // scale_up, scale_down, scale_to_zero, none
	CurrentReplicas  int32        `json:"current_replicas"`
	TargetReplicas   int32        `json:"target_replicas"`
	Metrics          ScaleMetrics `json:"metrics"`
	Reason           string       `json:"reason"`
}

// shouldScaleUp 判断是否需要扩容
func (c *Controller) shouldScaleUp(config *ScaleConfig, metrics ScaleMetrics, current int) bool {
	if current >= config.MaxInstances {
		return false
	}

	// CPU/内存使用率过高
	if metrics.CPUUsage > config.TargetCPU {
		return true
	}

	// 队列积压
	if metrics.QueueSize > config.TargetQueueSize {
		return true
	}

	// QPS过高
	if metrics.RequestsPerSec > 100 {
		return true
	}

	return false
}

// shouldScaleDown 判断是否需要缩容
func (c *Controller) shouldScaleDown(config *ScaleConfig, metrics ScaleMetrics, current int) bool {
	if current <= config.MinInstances {
		return false
	}

	// 空闲超时
	if metrics.IdleTime >= config.IdleTimeout {
		return true
	}

	// 低利用率
	if metrics.CPUUsage < config.TargetCPU/2 && metrics.QueueSize == 0 {
		return true
	}

	return false
}

// calculateMetrics 计算指标
func (c *Controller) calculateMetrics(services []*model.RegisteredService) ScaleMetrics {
	if len(services) == 0 {
		return ScaleMetrics{}
	}

	var totalCPU, totalGPU, totalMemory float64
	var totalQueue, totalRequests int
	var maxIdleTime int

	for _, s := range services {
		totalCPU += s.CPUUtilization
		totalGPU += s.GPUUtilization
		totalMemory += float64(s.MemoryUsage)
		totalQueue += s.QueueSize
		totalRequests += int(s.ProcessedCount)
	}

	count := float64(len(services))
	return ScaleMetrics{
		CPUUsage:      totalCPU / count,
		MemoryUsage:   totalMemory / count,
		GPUUsage:      totalGPU / count,
		QueueSize:     totalQueue,
		RequestsPerSec: float64(totalRequests) / 60, // 简化
		IdleTime:      maxIdleTime,
	}
}

// getCurrentReplicas 获取当前副本数
func (c *Controller) getCurrentReplicas(config *ScaleConfig) (int, error) {
	if config.DeploymentName == "" {
		// 根据feature_id推断deployment名称
		config.DeploymentName = fmt.Sprintf("%s-inference", config.FeatureID)
	}

	deployment, err := c.k8sClient.AppsV1().Deployments(config.Namespace).Get(
		context.Background(), config.DeploymentName, metav1.GetOptions{})
	if err != nil {
		return 0, err
	}

	return int(*deployment.Spec.Replicas), nil
}

// scale 执行伸缩
func (c *Controller) scale(config *ScaleConfig, targetReplicas int) error {
	if config.DeploymentName == "" {
		config.DeploymentName = fmt.Sprintf("%s-inference", config.FeatureID)
	}

	deployment, err := c.k8sClient.AppsV1().Deployments(config.Namespace).Get(
		context.Background(), config.DeploymentName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get deployment failed: %w", err)
	}

	*deployment.Spec.Replicas = int32(targetReplicas)

	_, err = c.k8sClient.AppsV1().Deployments(config.Namespace).Update(
		context.Background(), deployment, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("update deployment failed: %w", err)
	}

	log.Printf("[Scaler] Scaled %s from %d to %d", config.FeatureID,
		*deployment.Spec.Replicas, targetReplicas)

	return nil
}

// loadScaleConfigs 加载伸缩配置
func (c *Controller) loadScaleConfigs() error {
	// 默认配置
	defaultConfigs := []*ScaleConfig{
		{
			FeatureID:         "text_to_image",
			MinInstances:      0,
			MaxInstances:      5,
			TargetCPU:         70,
			TargetQueueSize:   50,
			IdleTimeout:       900,  // 15分钟
			ScaleUpCooldown:   60,
			ScaleDownCooldown: 300,
			Namespace:         "ai-platform",
		},
		{
			FeatureID:         "image_editing",
			MinInstances:      0,
			MaxInstances:      3,
			TargetCPU:         70,
			TargetQueueSize:   30,
			IdleTimeout:       600,  // 10分钟
			ScaleUpCooldown:   60,
			ScaleDownCooldown: 300,
			Namespace:         "ai-platform",
		},
		{
			FeatureID:         "image_stylization",
			MinInstances:      0,
			MaxInstances:      2,
			TargetCPU:         70,
			TargetQueueSize:   20,
			IdleTimeout:       600,
			ScaleUpCooldown:   60,
			ScaleDownCooldown: 300,
			Namespace:         "ai-platform",
		},
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	for _, cfg := range defaultConfigs {
		c.scales[cfg.FeatureID] = cfg
	}

	return nil
}

// UpdateScaleConfig 更新伸缩配置
func (c *Controller) UpdateScaleConfig(config *ScaleConfig) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.scales[config.FeatureID] = config
	return nil
}

// GetScaleConfig 获取伸缩配置
func (c *Controller) GetScaleConfig(featureID string) (*ScaleConfig, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	config, ok := c.scales[featureID]
	if !ok {
		return nil, fmt.Errorf("no scale config for feature: %s", featureID)
	}

	return config, nil
}

// WatchScaleEvents 监听伸缩事件
func (c *Controller) WatchScaleEvents(ctx context.Context) <-chan *ScaleEvent {
	ch := make(chan *ScaleEvent, 10)
	go func() {
		defer close(ch)
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-c.scaleEvents:
				select {
				case ch <- event:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return ch
}

// scaleLoop 伸缩循环
func (c *Controller) scaleLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.RLock()
		features := make([]string, 0, len(c.scales))
		for featureID := range c.scales {
			features = append(features, featureID)
		}
		c.mu.RUnlock()

		for _, featureID := range features {
			decision, err := c.CheckScale(context.Background(), featureID)
			if err != nil {
				log.Printf("[Scaler] Check scale failed for %s: %v", featureID, err)
				continue
			}

			if decision.Action != "none" {
				log.Printf("[Scaler] Scale decision for %s: %s (%d -> %d), reason: %s",
					featureID, decision.Action, decision.CurrentReplicas, decision.TargetReplicas, decision.Reason)
			}
		}
	}
}

// ScaleToZero 缩容到零
func (c *Controller) ScaleToZero(featureID string) error {
	c.mu.RLock()
	config, ok := c.scales[featureID]
	c.mu.RUnlock()

	if !ok {
		return fmt.Errorf("no scale config for feature: %s", featureID)
	}

	return c.scale(config, 0)
}

// ScaleUp 扩容
func (c *Controller) ScaleUp(featureID string, count int32) error {
	c.mu.RLock()
	config, ok := c.scales[featureID]
	c.mu.RUnlock()

	if !ok {
		return fmt.Errorf("no scale config for feature: %s", featureID)
	}

	current, err := c.getCurrentReplicas(config)
	if err != nil {
		return err
	}

	target := min(int32(current)+count, config.MaxInstances)
	return c.scale(config, int(target))
}

func min(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}

func max(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}
