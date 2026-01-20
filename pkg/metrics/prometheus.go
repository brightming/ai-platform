package prometheus

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Registry Prometheus指标注册表
type Registry struct {
	mu sync.RWMutex

	// 请求指标
	requestsTotal *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
	requestsInFlight *prometheus.GaugeVec

	// Provider指标
	providerRequestsTotal *prometheus.CounterVec
	providerErrorsTotal *prometheus.CounterVec
	providerLatency *prometheus.HistogramVec

	// 队列指标
	queueDepth *prometheus.GaugeVec
	queueWaitTime *prometheus.HistogramVec

	// 成本指标
	costTotal *prometheus.CounterVec
	costByProvider *prometheus.CounterVec

	// 服务实例指标
	serviceStatus *prometheus.GaugeVec
	serviceCPU *prometheus.GaugeVec
	serviceGPU *prometheus.GaugeVec
	serviceMemory *prometheus.GaugeVec
}

// NewRegistry 创建Prometheus指标注册表
func NewRegistry() *Registry {
	r := &Registry{
		// 请求指标
		requestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "ai_platform",
				Name:      "requests_total",
				Help:      "Total number of requests",
			},
			[]string{"feature", "provider_type", "provider_id", "status"},
		),
		requestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "ai_platform",
				Name:      "request_duration_seconds",
				Help:      "Request duration in seconds",
				Buckets:   []float64{0.01, 0.05, 0.1, 0.5, 1, 2, 5, 10, 30, 60},
			},
			[]string{"feature", "provider_type", "provider_id"},
		),
		requestsInFlight: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "ai_platform",
				Name:      "requests_in_flight",
				Help:      "Number of requests currently in flight",
			},
			[]string{"feature"},
		),

		// Provider指标
		providerRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "ai_platform",
				Name:      "provider_requests_total",
				Help:      "Total number of provider requests",
			},
			[]string{"provider_id", "provider_type", "feature"},
		),
		providerErrorsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "ai_platform",
				Name:      "provider_errors_total",
				Help:      "Total number of provider errors",
			},
			[]string{"provider_id", "error_type"},
		),
		providerLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "ai_platform",
				Name:      "provider_latency_seconds",
				Help:      "Provider request latency",
				Buckets:   []float64{0.1, 0.5, 1, 2, 5, 10, 30},
			},
			[]string{"provider_id", "feature"},
		),

		// 队列指标
		queueDepth: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "ai_platform",
				Name:      "queue_depth",
				Help:      "Current queue depth",
			},
			[]string{"feature", "provider_id"},
		),
		queueWaitTime: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "ai_platform",
				Name:      "queue_wait_seconds",
				Help:      "Time spent waiting in queue",
				Buckets:   []float64{0.001, 0.01, 0.05, 0.1, 0.5, 1, 5},
			},
			[]string{"feature", "provider_id"},
		),

		// 成本指标
		costTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "ai_platform",
				Name:      "cost_total",
				Help:      "Total cost",
			},
			[]string{"provider_type"},
		),
		costByProvider: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "ai_platform",
				Name:      "cost_by_provider",
				Help:      "Cost by provider",
			},
			[]string{"provider_id", "feature"},
		),

		// 服务实例指标
		serviceStatus: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "ai_platform",
				Name:      "service_status",
				Help:      "Service status (1=healthy, 0=unhealthy)",
			},
			[]string{"service_id", "service_type"},
		),
		serviceCPU: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "ai_platform",
				Name:      "service_cpu_usage_percent",
				Help:      "Service CPU usage percentage",
			},
			[]string{"service_id"},
		),
		serviceGPU: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "ai_platform",
				Name:      "service_gpu_usage_percent",
				Help:      "Service GPU usage percentage",
			},
			[]string{"service_id", "gpu_id"},
		),
		serviceMemory: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "ai_platform",
				Name:      "service_memory_bytes",
				Help:      "Service memory usage in bytes",
			},
			[]string{"service_id"},
		),
	}

	// 注册指标
	r.MustRegister()

	return r
}

// MustRegister 注册所有指标
func (r *Registry) MustRegister() {
	prometheus.MustRegister(r.requestsTotal)
	prometheus.MustRegister(r.requestDuration)
	prometheus.MustRegister(r.requestsInFlight)
	prometheus.MustRegister(r.providerRequestsTotal)
	prometheus.MustRegister(r.providerErrorsTotal)
	prometheus.MustRegister(r.providerLatency)
	prometheus.MustRegister(r.queueDepth)
	prometheus.MustRegister(r.queueWaitTime)
	prometheus.MustRegister(r.costTotal)
	prometheus.MustRegister(r.costByProvider)
	prometheus.MustRegister(r.serviceStatus)
	prometheus.MustRegister(r.serviceCPU)
	prometheus.MustRegister(r.serviceGPU)
	prometheus.MustRegister(r.serviceMemory)
}

// RecordRequest 记录请求
func (r *Registry) RecordRequest(feature, providerType, providerID, status string, duration float64) {
	r.requestsTotal.WithLabelValues(feature, providerType, providerID, status).Inc()
	r.requestDuration.WithLabelValues(feature, providerType, providerID).Observe(duration)
}

// IncrementInFlight 增加进行中请求数
func (r *Registry) IncrementInFlight(feature string) {
	r.requestsInFlight.WithLabelValues(feature).Inc()
}

// DecrementInFlight 减少进行中请求数
func (r *Registry) DecrementInFlight(feature string) {
	r.requestsInFlight.WithLabelValues(feature).Dec()
}

// RecordProviderRequest 记录Provider请求
func (r *Registry) RecordProviderRequest(providerID, providerType, feature string, duration float64, err error) {
	r.providerRequestsTotal.WithLabelValues(providerID, providerType, feature).Inc()
	r.providerLatency.WithLabelValues(providerID, feature).Observe(duration)

	if err != nil {
		r.providerErrorsTotal.WithLabelValues(providerID, "request_error").Inc()
	}
}

// UpdateQueueDepth 更新队列深度
func (r *Registry) UpdateQueueDepth(feature, providerID string, depth int) {
	r.queueDepth.WithLabelValues(feature, providerID).Set(float64(depth))
}

// RecordQueueWait 记录队列等待时间
func (r *Registry) RecordQueueWait(feature, providerID string, waitSeconds float64) {
	r.queueWaitTime.WithLabelValues(feature, providerID).Observe(waitSeconds)
}

// RecordCost 记录成本
func (r *Registry) RecordCost(providerType, providerID, feature string, cost float64) {
	r.costTotal.WithLabelValues(providerType).Add(cost)
	r.costByProvider.WithLabelValues(providerID, feature).Add(cost)
}

// UpdateServiceStatus 更新服务状态
func (r *Registry) UpdateServiceStatus(serviceID, serviceType string, status float64) {
	r.serviceStatus.WithLabelValues(serviceID, serviceType).Set(status)
}

// UpdateServiceCPU 更新服务CPU
func (r *Registry) UpdateServiceCPU(serviceID string, cpu float64) {
	r.serviceCPU.WithLabelValues(serviceID).Set(cpu)
}

// UpdateServiceGPU 更新服务GPU
func (r *Registry) UpdateServiceGPU(serviceID, gpuID string, gpu float64) {
	r.serviceGPU.WithLabelValues(serviceID, gpuID).Set(gpu)
}

// UpdateServiceMemory 更新服务内存
func (r *Registry) UpdateServiceMemory(serviceID string, memoryBytes float64) {
	r.serviceMemory.WithLabelValues(serviceID).Set(memoryBytes)
}

// Handler 返回Prometheus指标处理器
func (r *Registry) Handler() http.Handler {
	return promhttp.Handler()
}

// Middleware HTTP中间件，用于在请求处理时自动追踪
func (r *Registry) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// 提取feature标签（从路径或header）
		feature := extractFeature(req)
	provider := extractProvider(req)

	// 记录请求开始
	start := time.Now()
	r.IncrementInFlight(feature)

	// 调用下一个处理器
	// 注意：实际使用中需要在处理器中调用DecrementInFlight和RecordRequest

	// 创建响应包装器来追踪状态码
	wrappedWriter := &responseWriter{ResponseWriter: w, statusCode: 200}

	next.ServeHTTP(wrappedWriter, req)

	// 记录请求完成
	duration := time.Since(start).Seconds()
	status := statusCodeToString(wrappedWriter.statusCode)
	r.RecordRequest(feature, provider, provider, status, duration)
	r.DecrementInFlight(feature)
	})

// 自定义ResponseWriter
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *responseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

// extractFeature 从请求中提取feature标签
func extractFeature(req *http.Request) string {
	// 从路径提取
	if feature := req.URL.Query().Get("feature"); feature != "" {
		return feature
	}

	// 从路径推断
	path := req.URL.Path
	switch {
	case contains(path, "/text-to-image") || contains(path, "/text_to_image"):
		return "text_to_image"
	case contains(path, "/image-edit") || contains(path, "/image_editing"):
		return "image_editing"
	case contains(path, "/image-stylize") || contains(path, "/image_stylization"):
		return "image_stylization"
	case contains(path, "/text-generation") || contains(path, "/text_generation"):
		return "text_generation"
	}

	return "unknown"
}

// extractProvider 从请求中提取provider标签
func extractProvider(req *http.Request) string {
	return req.URL.Query().Get("provider")
}

func statusCodeToString(code int) string {
	switch {
	case code >= 200 && code < 300:
		return "success"
	case code >= 400 && code < 500:
		return "client_error"
	case code >= 500:
		return "server_error"
	default:
		return "unknown"
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (
		s[:len(substr)] == substr ||
		s[len(s)-len(substr):] == substr ||
		findSubstring(s, substr)
	))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
