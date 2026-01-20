package metrics

import (
	"sync"
	"time"
)

// Collector 监控指标采集器
type Collector struct {
	mu              sync.RWMutex
	requests        *RequestMetrics
	queueMetrics    map[string]*QueueMetrics
	providerMetrics map[string]*ProviderMetrics
	costMetrics     *CostMetrics
}

// RequestMetrics 请求指标
type RequestMetrics struct {
	Total      int64         `json:"total"`
	Success    int64         `json:"success"`
	Failed     int64         `json:"failed"`
	ByFeature  map[string]int64 `json:"by_feature"`
	ByProvider map[string]int64 `json:"by_provider"`
}

// QueueMetrics 队列指标
type QueueMetrics struct {
	Feature         string    `json:"feature"`
	CurrentDepth    int       `json:"current_depth"`
	WaitTimeMs      Histogram `json:"wait_time_ms"`
	ExecTimeMs      Histogram `json:"exec_time_ms"`
	TotalLatencyMs  Histogram `json:"total_latency_ms"`
	LastUpdate      time.Time `json:"last_update"`
}

// ProviderMetrics Provider指标
type ProviderMetrics struct {
	ProviderID    string    `json:"provider_id"`
	Type          string    `json:"type"`          // self_hosted, third_party
	Requests      int64     `json:"requests"`
	Success       int64     `json:"success"`
	Failed        int64     `json:"failed"`
	AvgLatencyMs  float64   `json:"avg_latency_ms"`
	P95LatencyMs  int       `json:"p95_latency_ms"`
	P99LatencyMs  int       `json:"p99_latency_ms"`
	TokensInput   int64     `json:"tokens_input"`
	TokensOutput  int64     `json:"tokens_output"`
	ImagesGenerated int64   `json:"images_generated"`
	Cost          float64   `json:"cost"`
	LastUpdate    time.Time `json:"last_update"`
}

// CostMetrics 成本指标
type CostMetrics struct {
	SelfHostedCost float64            `json:"self_hosted_cost"`
	ThirdPartyCost map[string]float64 `json:"third_party_cost"` // vendor -> cost
	TotalCost      float64            `json:"total_cost"`
	ByFeature      map[string]float64 `json:"by_feature"`
	Period         string             `json:"period"` // hour, day, month
	PeriodStart    time.Time          `json:"period_start"`
}

// Histogram 直方图（简化版，用于计算P50/P95/P99）
type Histogram struct {
	Values []int `json:"values"`
	maxLen int   // 最大保留样本数
}

// NewHistogram 创建直方图
func NewHistogram(maxLen int) *Histogram {
	return &Histogram{
		Values: make([]int, 0, maxLen),
		maxLen: maxLen,
	}
}

// Record 记录值
func (h *Histogram) Record(value int) {
	h.Values = append(h.Values, value)
	if len(h.Values) > h.maxLen {
		// 移除最旧的值
		h.Values = h.Values[1:]
	}
}

// P50 计算P50
func (h *Histogram) P50() int {
	return h.Percentile(50)
}

// P95 计算P95
func (h *Histogram) P95() int {
	return h.Percentile(95)
}

// P99 记录P99
func (h *Histogram) P99() int {
	return h.Percentile(99)
}

// Percentile 计算百分位
func (h *Histogram) Percentile(p int) int {
	if len(h.Values) == 0 {
		return 0
	}

	// 简化实现：排序后取值
	sorted := make([]int, len(h.Values))
	copy(sorted, h.Values)

	// 简单排序（实际应使用更高效的算法）
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	idx := len(sorted) * p / 100
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// NewCollector 创建指标采集器
func NewCollector() *Collector {
	return &Collector{
		requests: &RequestMetrics{
			ByFeature:  make(map[string]int64),
			ByProvider: make(map[string]int64),
		},
		queueMetrics:    make(map[string]*QueueMetrics),
		providerMetrics: make(map[string]*ProviderMetrics),
		costMetrics: &CostMetrics{
			ThirdPartyCost: make(map[string]float64),
			ByFeature:      make(map[string]float64),
			PeriodStart:    time.Now(),
		},
	}
}

// RecordRequest 记录请求
func (c *Collector) RecordRequest(feature, providerID, providerType string, success bool, latencyMs int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.requests.Total++
	if success {
		c.requests.Success++
	} else {
		c.requests.Failed++
	}

	c.requests.ByFeature[feature]++
	c.requests.ByProvider[providerID]++

	// 更新Provider指标
	if _, ok := c.providerMetrics[providerID]; !ok {
		c.providerMetrics[providerID] = &ProviderMetrics{
			ProviderID: providerID,
			Type:       providerType,
		}
	}

	pm := c.providerMetrics[providerID]
	pm.Requests++
	if success {
		pm.Success++
	} else {
		pm.Failed++
	}
	pm.LastUpdate = time.Now()
}

// RecordTokens 记录Token消耗
func (c *Collector) RecordTokens(providerID string, input, output int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if pm, ok := c.providerMetrics[providerID]; ok {
		pm.TokensInput += int64(input)
		pm.TokensOutput += int64(output)
	}
}

// RecordImage 记录图像生成
func (c *Collector) RecordImage(providerID string, count int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if pm, ok := c.providerMetrics[providerID]; ok {
		pm.ImagesGenerated += int64(count)
	}
}

// RecordCost 记录成本
func (c *Collector) RecordCost(providerID, providerType string, cost float64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if providerType == "self_hosted" {
		c.costMetrics.SelfHostedCost += cost
	} else {
		c.costMetrics.ThirdPartyCost[providerID] += cost
	}
	c.costMetrics.TotalCost += cost

	if pm, ok := c.providerMetrics[providerID]; ok {
		pm.Cost += cost
	}
}

// RecordQueueMetrics 记录队列指标
func (c *Collector) RecordQueueMetrics(feature string, waitTimeMs, execTimeMs, totalLatencyMs int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.queueMetrics[feature]; !ok {
		c.queueMetrics[feature] = &QueueMetrics{
			Feature:    feature,
			WaitTimeMs: *NewHistogram(1000),
			ExecTimeMs: *NewHistogram(1000),
			TotalLatencyMs: *NewHistogram(1000),
		}
	}

	qm := c.queueMetrics[feature]
	qm.WaitTimeMs.Record(waitTimeMs)
	qm.ExecTimeMs.Record(execTimeMs)
	qm.TotalLatencyMs.Record(totalLatencyMs)
	qm.LastUpdate = time.Now()
}

// UpdateQueueDepth 更新队列深度
func (c *Collector) UpdateQueueDepth(feature string, depth int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.queueMetrics[feature]; !ok {
		c.queueMetrics[feature] = &QueueMetrics{
			Feature:    feature,
			WaitTimeMs: *NewHistogram(1000),
			ExecTimeMs: *NewHistogram(1000),
			TotalLatencyMs: *NewHistogram(1000),
		}
	}

	c.queueMetrics[feature].CurrentDepth = depth
	c.queueMetrics[feature].LastUpdate = time.Now()
}

// GetRequests 获取请求指标
func (c *Collector) GetRequests() *RequestMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// 返回副本
	return &RequestMetrics{
		Total:      c.requests.Total,
		Success:    c.requests.Success,
		Failed:     c.requests.Failed,
		ByFeature:  copyMap(c.requests.ByFeature),
		ByProvider: copyMap(c.requests.ByProvider),
	}
}

// GetQueueMetrics 获取队列指标
func (c *Collector) GetQueueMetrics(feature string) *QueueMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if qm, ok := c.queueMetrics[feature]; ok {
		return &QueueMetrics{
			Feature:        qm.Feature,
			CurrentDepth:   qm.CurrentDepth,
			WaitTimeMs:     *copyHistogram(&qm.WaitTimeMs),
			ExecTimeMs:     *copyHistogram(&qm.ExecTimeMs),
			TotalLatencyMs: *copyHistogram(&qm.TotalLatencyMs),
			LastUpdate:     qm.LastUpdate,
		}
	}

	return nil
}

// GetAllQueueMetrics 获取所有队列指标
func (c *Collector) GetAllQueueMetrics() map[string]*QueueMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]*QueueMetrics)
	for k, v := range c.queueMetrics {
		result[k] = &QueueMetrics{
			Feature:        v.Feature,
			CurrentDepth:   v.CurrentDepth,
			WaitTimeMs:     *copyHistogram(&v.WaitTimeMs),
			ExecTimeMs:     *copyHistogram(&v.ExecTimeMs),
			TotalLatencyMs: *copyHistogram(&v.TotalLatencyMs),
			LastUpdate:     v.LastUpdate,
		}
	}
	return result
}

// GetProviderMetrics 获取Provider指标
func (c *Collector) GetProviderMetrics(providerID string) *ProviderMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if pm, ok := c.providerMetrics[providerID]; ok {
		return &ProviderMetrics{
			ProviderID:     pm.ProviderID,
			Type:           pm.Type,
			Requests:       pm.Requests,
			Success:        pm.Success,
			Failed:         pm.Failed,
			AvgLatencyMs:   pm.AvgLatencyMs,
			TokensInput:    pm.TokensInput,
			TokensOutput:   pm.TokensOutput,
			ImagesGenerated: pm.ImagesGenerated,
			Cost:           pm.Cost,
			LastUpdate:     pm.LastUpdate,
		}
	}

	return nil
}

// GetAllProviderMetrics 获取所有Provider指标
func (c *Collector) GetAllProviderMetrics() map[string]*ProviderMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]*ProviderMetrics)
	for k, v := range c.providerMetrics {
		result[k] = &ProviderMetrics{
			ProviderID:      v.ProviderID,
			Type:            v.Type,
			Requests:        v.Requests,
			Success:         v.Success,
			Failed:          v.Failed,
			AvgLatencyMs:    v.AvgLatencyMs,
			TokensInput:     v.TokensInput,
			TokensOutput:    v.TokensOutput,
			ImagesGenerated: v.ImagesGenerated,
			Cost:            v.Cost,
			LastUpdate:      v.LastUpdate,
		}
	}
	return result
}

// GetCostMetrics 获取成本指标
func (c *Collector) GetCostMetrics() *CostMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return &CostMetrics{
		SelfHostedCost:  c.costMetrics.SelfHostedCost,
		ThirdPartyCost:  copyMapFloat(c.costMetrics.ThirdPartyCost),
		TotalCost:       c.costMetrics.TotalCost,
		ByFeature:       copyMapFloat(c.costMetrics.ByFeature),
		Period:          c.costMetrics.Period,
		PeriodStart:     c.costMetrics.PeriodStart,
	}
}

// ResetPeriod 重置周期
func (c *Collector) ResetPeriod(period string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.costMetrics = &CostMetrics{
		ThirdPartyCost: make(map[string]float64),
		ByFeature:      make(map[string]float64),
		Period:         period,
		PeriodStart:    time.Now(),
	}
}

// 辅助函数
func copyMap(m map[string]int64) map[string]int64 {
	result := make(map[string]int64, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}

func copyMapFloat(m map[string]float64) map[string]float64 {
	result := make(map[string]float64, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}

func copyHistogram(h *Histogram) *Histogram {
	newH := &Histogram{
		maxLen: h.maxLen,
	}
	newH.Values = make([]int, len(h.Values))
	copy(newH.Values, h.Values)
	return newH
}
