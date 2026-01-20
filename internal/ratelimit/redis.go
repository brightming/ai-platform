package ratelimit

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"
)

// RateLimiter 限流器接口
type RateLimiter interface {
	Allow(ctx context.Context, tenantID, feature string) bool
	GetLimit(tenantID, feature string) int
	SetLimit(tenantID, feature string, limit int)
}

// RedisLimiter Redis限流器（简化版，实际应使用Redis）
type RedisLimiter struct {
	redisAddr     string
	redisPassword string
	mu            sync.RWMutex
	limits        map[string]int   // (tenantID, feature) -> limit
	counters      map[string]int   // (tenantID, feature, timestamp) -> count
	window        time.Duration
}

// NewRedisLimiter 创建Redis限流器
func NewRedisLimiter(addr, password string) *RedisLimiter {
	return &RedisLimiter{
		redisAddr:     addr,
		redisPassword: password,
		limits:        make(map[string]int),
		counters:      make(map[string]int),
		window:        time.Minute,
	}
}

// Allow 检查是否允许请求
func (r *RedisLimiter) Allow(ctx context.Context, tenantID, feature string) bool {
	key := fmt.Sprintf("%s:%s", tenantID, feature)
	windowKey := fmt.Sprintf("%s:%d", key, time.Now().Unix()/60)

	r.mu.Lock()
	defer r.mu.Unlock()

	limit := r.getLimit(key)
	current := r.counters[windowKey]

	if current >= limit {
		return false
	}

	r.counters[windowKey] = current + 1
	return true
}

// GetLimit 获取限流值
func (r *RedisLimiter) GetLimit(tenantID, feature string) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	key := fmt.Sprintf("%s:%s", tenantID, feature)
	return r.getLimit(key)
}

func (r *RedisLimiter) getLimit(key string) int {
	if limit, ok := r.limits[key]; ok {
		return limit
	}
	return 100 // 默认限制
}

// SetLimit 设置限流值
func (r *RedisLimiter) SetLimit(tenantID, feature string, limit int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := fmt.Sprintf("%s:%s", tenantID, feature)
	r.limits[key] = limit
}

// cleanupWindow 清理过期窗口
func (r *RedisLimiter) cleanupWindow() {
	r.mu.Lock()
	defer r.mu.Unlock()

	currentWindow := time.Now().Unix() / 60
	for key := range r.counters {
		// 解析窗口时间
		var tenantID, feature string
		fmt.Sscanf(key, "%s:%s:%d", &tenantID, &feature, new(int))
		// 简化：清理5分钟前的窗口
		// 实际应更精确地解析
	}
}

// MemoryLimiter 内存限流器（用于测试或单机）
type MemoryLimiter struct {
	mu       sync.RWMutex
	limits   map[string]int          // (tenantID, feature) -> limit
	counters map[string]*windowCounter // (tenantID, feature) -> counter
	window   time.Duration
}

// windowCounter 滑动窗口计数器
type windowCounter struct {
	counts []int
	start  time.Time
	mu     sync.Mutex
}

// newWindowCounter 创建滑动窗口计数器
func newWindowCounter(window time.Duration, buckets int) *windowCounter {
	return &windowCounter{
		counts: make([]int, buckets),
		start:  time.Now(),
	}
}

// count 获取当前计数
func (w *windowCounter) count() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	sum := 0
	for _, c := range w.counts {
		sum += c
	}
	return sum
}

// increment 增加计数
func (w *windowCounter) increment() {
	w.mu.Lock()
	defer w.mu.Unlock()
	// 简化：只增加第一个bucket
	// 实际应实现完整的滑动窗口
	w.counts[0]++
}

// NewMemoryLimiter 创建内存限流器
func NewMemoryLimiter() *MemoryLimiter {
	return &MemoryLimiter{
		limits:   make(map[string]int),
		counters: make(map[string]*windowCounter),
		window:   time.Minute,
	}
}

// Allow 检查是否允许请求
func (m *MemoryLimiter) Allow(ctx context.Context, tenantID, feature string) bool {
	key := fmt.Sprintf("%s:%s", tenantID, feature)

	m.mu.Lock()
	defer m.mu.Unlock()

	// 获取或创建计数器
	counter, ok := m.counters[key]
	if !ok {
		counter = newWindowCounter(m.window, 60)
		m.counters[key] = counter
	}

	// 获取限制
	limit := m.getLimit(key)
	current := counter.count()

	if current >= limit {
		return false
	}

	counter.increment()
	return true
}

// GetLimit 获取限流值
func (m *MemoryLimiter) GetLimit(tenantID, feature string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	key := fmt.Sprintf("%s:%s", tenantID, feature)
	return m.getLimit(key)
}

func (m *MemoryLimiter) getLimit(key string) int {
	if limit, ok := m.limits[key]; ok {
		return limit
	}
	return 100 // 默认限制
}

// SetLimit 设置限流值
func (m *MemoryLimiter) SetLimit(tenantID, feature string, limit int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := fmt.Sprintf("%s:%s", tenantID, feature)
	m.limits[key] = limit
}

// ResetCounters 重置计数器（用于测试）
func (m *MemoryLimiter) ResetCounters() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counters = make(map[string]*windowCounter)
}

// TokenBucket 令牌桶限流器
type TokenBucket struct {
	capacity  int
	tokens    int
	rate      int          // 每秒补充的令牌数
	lastRefill time.Time
	mu        sync.Mutex
}

// NewTokenBucket 创建令牌桶
func NewTokenBucket(capacity, rate int) *TokenBucket {
	return &TokenBucket{
		capacity:   capacity,
		tokens:     capacity,
		rate:       rate,
		lastRefill: time.Now(),
	}
}

// Allow 尝试获取令牌
func (tb *TokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refill()

	if tb.tokens > 0 {
		tb.tokens--
		return true
	}

	return false
}

// refill 补充令牌
func (tb *TokenBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()
	tokensToAdd := int(elapsed * float64(tb.rate))

	if tokensToAdd > 0 {
		tb.tokens = min(tb.tokens+tokensToAdd, tb.capacity)
		tb.lastRefill = now
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// LeakyBucket 漏桶限流器
type LeakyBucket struct {
	capacity int
	water    int
	rate     int          // 每秒漏出的水滴数
	lastLeak time.Time
	mu       sync.Mutex
}

// NewLeakyBucket 创建漏桶
func NewLeakyBucket(capacity, rate int) *LeakyBucket {
	return &LeakyBucket{
		capacity: capacity,
		rate:     rate,
		lastLeak: time.Now(),
	}
}

// Allow 尝试添加水滴
func (lb *LeakyBucket) Allow() bool {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	lb.leak()

	if lb.water < lb.capacity {
		lb.water++
		return true
	}

	return false
}

// leak 漏水
func (lb *LeakyBucket) leak() {
	now := time.Now()
	elapsed := now.Sub(lb.lastLeak).Seconds()
	waterToLeak := int(elapsed * float64(lb.rate))

	if waterToLeak > 0 {
		lb.water = max(lb.water-waterToLeak, 0)
		lb.lastLeak = now
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
