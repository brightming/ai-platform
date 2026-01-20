package registry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/brightming/ai-platform/pkg/model"
	"gorm.io/gorm"
)

// ServiceImpl 服务注册中心实现
type ServiceImpl struct {
	db            *gorm.DB
	mu            sync.RWMutex
	services      map[string]*model.RegisteredService
	servicesByType map[string][]string // serviceType -> []serviceID
	heartbeatCh   chan *model.RegisteredService
	configCh      chan *ConfigUpdate
}

// ConfigUpdate 配置更新
type ConfigUpdate struct {
	ServiceID string
	Config    map[string]interface{}
}

// NewService 创建服务注册中心
func NewService(db *gorm.DB) *ServiceImpl {
	s := &ServiceImpl{
		db:            db,
		services:      make(map[string]*model.RegisteredService),
		servicesByType: make(map[string][]string),
		heartbeatCh:   make(chan *model.RegisteredService, 100),
		configCh:      make(chan *ConfigUpdate, 100),
	}
	// 启动时加载现有服务
	s.loadServices()
	// 启动健康检查
	go s.startHealthCheck()
	return s
}

// Register 服务注册
func (s *ServiceImpl) Register(req *model.RegisterRequest) (*model.RegisterResponse, error) {
	serviceID := generateServiceID(req.ServiceType)

	now := time.Now()
	service := &model.RegisteredService{
		ID:            serviceID,
		ServiceType:   req.ServiceType,
		Version:       req.Version,
		Hostname:      req.Hostname,
		IPAddress:     req.IPAddress,
		Port:          req.Port,
		Capabilities:  req.Metadata,
		Resources:     req.Resources,
		Performance:   req.Performance,
		Status:        model.StatusHealthy,
		LastHeartbeat: now,
		StartedAt:     now,
		RegisteredAt:  now,
		UpdatedAt:     now,
		Metadata:      make(map[string]string),
	}

	s.mu.Lock()
	// 检查是否已存在
	if existing, ok := s.services[serviceID]; ok {
		// 更新已有服务
		existing.Hostname = req.Hostname
		existing.IPAddress = req.IPAddress
		existing.Port = req.Port
		existing.Capabilities = req.Metadata
		existing.Resources = req.Resources
		existing.Performance = req.Performance
		existing.Status = model.StatusHealthy
		existing.LastHeartbeat = now
		existing.UpdatedAt = now
		service = existing
	} else {
		// 新服务
		s.services[serviceID] = service
		s.servicesByType[req.ServiceType] = append(s.servicesByType[req.ServiceType], serviceID)
	}
	s.mu.Unlock()

	// 持久化到数据库
	if err := s.saveService(service); err != nil {
		return nil, err
	}

	// 生成心跳token
	token := generateToken()

	return &model.RegisterResponse{
		ServiceID:         serviceID,
		HeartbeatInterval: 30, // 30秒
		ConfigVersion:     "v1",
		Token:             token,
	}, nil
}

// Heartbeat 处理心跳
func (s *ServiceImpl) Heartbeat(req *model.HeartbeatRequest) (*model.HeartbeatResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	service, ok := s.services[req.ServiceID]
	if !ok {
		return nil, fmt.Errorf("service not found: %s", req.ServiceID)
	}

	// 验证token
	if req.Token == "" || !validateToken(req.ServiceID, req.Token) {
		return nil, fmt.Errorf("invalid token")
	}

	// 更新状态
	now := time.Now()
	service.LastHeartbeat = now
	service.HeartbeatMissed = 0
	service.CurrentLoad = req.CurrentLoad
	service.QueueSize = req.QueueSize
	service.ProcessedCount = req.ProcessedCount
	service.ErrorCount = req.ErrorCount
	service.CPUUtilization = req.CPUUtilization
	service.GPUUtilization = req.GPUUtilization
	service.MemoryUsage = req.MemoryUsage
	service.UpdatedAt = now

	// 根据错误率判断状态
	if req.ProcessedCount > 0 {
		errorRate := float64(req.ErrorCount) / float64(req.ProcessedCount)
		if errorRate > 0.1 {
			service.Status = model.StatusDegraded
		} else if service.Status == model.StatusDegraded {
			service.Status = model.StatusHealthy
		}
	}

	// 检查是否有配置更新
	var configUpdate *model.ConfigUpdate
	select {
	case cu := <-s.configCh:
		if cu.ServiceID == req.ServiceID {
			configUpdate = &model.ConfigUpdate{
				Version: fmt.Sprint(time.Now().Unix()),
				Config:  cu.Config,
			}
		} else {
			// 不是给这个服务的，放回队列
			s.configCh <- cu
		}
	default:
	}

	// 持久化更新
	go s.saveService(service)

	// 通知心跳
	select {
	case s.heartbeatCh <- service:
	default:
	}

	status := "healthy"
	if service.Status == model.StatusDegraded {
		status = "degraded"
	} else if service.Status == model.StatusDraining {
		status = "draining"
	}

	return &model.HeartbeatResponse{
		Status:         status,
		ConfigUpdate:   configUpdate,
		DrainRequested: service.Status == model.StatusDraining,
	}, nil
}

// Shutdown 优雅关闭
func (s *ServiceImpl) Shutdown(req *model.ShutdownRequest) (*model.ShutdownResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	service, ok := s.services[req.ServiceID]
	if !ok {
		return nil, fmt.Errorf("service not found: %s", req.ServiceID)
	}

	// 标记为draining状态
	service.Status = model.StatusDraining
	service.UpdatedAt = time.Now()

	gracePeriod := 30 // 默认30秒优雅期

	return &model.ShutdownResponse{
		GracePeriodSeconds: gracePeriod,
		Message:            "Shutdown accepted. Complete in-flight requests.",
	}, nil
}

// GetService 获取服务
func (s *ServiceImpl) GetService(id string) (*model.RegisteredService, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	service, ok := s.services[id]
	if !ok {
		return nil, fmt.Errorf("service not found: %s", id)
	}

	return service, nil
}

// ListServices 列出服务
func (s *ServiceImpl) ListServices(filter *model.ServiceFilter) (*model.GetServicesResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var services []*model.RegisteredService

	for _, service := range s.services {
		if filter.ServiceType != "" && service.ServiceType != filter.ServiceType {
			continue
		}
		if filter.Status != nil && service.Status != *filter.Status {
			continue
		}
		services = append(services, service)
	}

	// 统计状态
	healthy := 0
	degraded := 0
	unhealthy := 0
	for _, s := range services {
		switch s.Status {
		case model.StatusHealthy:
			healthy++
		case model.StatusDegraded:
			degraded++
		case model.StatusUnhealthy:
			unhealthy++
		}
	}

	return &model.GetServicesResponse{
		Services:       services,
		TotalCount:     len(services),
		HealthyCount:   healthy,
		DegradedCount:  degraded,
		UnhealthyCount: unhealthy,
	}, nil
}

// GetServicesByType 根据类型获取服务
func (s *ServiceImpl) GetServicesByType(serviceType string) ([]*model.RegisteredService, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids, ok := s.servicesByType[serviceType]
	if !ok {
		return []*model.RegisteredService{}, nil
	}

	services := make([]*model.RegisteredService, 0, len(ids))
	for _, id := range ids {
		if service, ok := s.services[id]; ok {
			if service.Status == model.StatusHealthy || service.Status == model.StatusDegraded {
				services = append(services, service)
			}
		}
	}

	return services, nil
}

// GetHealthyServices 获取健康的服务
func (s *ServiceImpl) GetHealthyServices(serviceType string) ([]*model.RegisteredService, error) {
	services, err := s.GetServicesByType(serviceType)
	if err != nil {
		return nil, err
	}

	healthy := make([]*model.RegisteredService, 0)
	for _, s := range services {
		if s.Status == model.StatusHealthy {
			healthy = append(healthy, s)
		}
	}

	return healthy, nil
}

// UpdateConfig 更新服务配置
func (s *ServiceImpl) UpdateConfig(serviceID string, config map[string]interface{}) error {
	select {
	case s.configCh <- &ConfigUpdate{
		ServiceID: serviceID,
		Config:    config,
	}:
		return nil
	default:
		return errors.New("config channel full")
	}
}

// WatchHeartbeat 监听心跳
func (s *ServiceImpl) WatchHeartbeat(ctx context.Context) <-chan *model.RegisteredService {
	ch := make(chan *model.RegisteredService, 10)
	go func() {
		defer close(ch)
		for {
			select {
			case <-ctx.Done():
				return
			case service := <-s.heartbeatCh:
				select {
				case ch <- service:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return ch
}

// startHealthCheck 启动健康检查
func (s *ServiceImpl) startHealthCheck() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		s.checkHeartbeatTimeout()
	}
}

// checkHeartbeatTimeout 检查心跳超时
func (s *ServiceImpl) checkHeartbeatTimeout() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	timeout := 90 * time.Second // 3次心跳未响应

	for id, service := range s.services {
		if service.Status == model.StatusDraining || service.Status == model.StatusTerminated {
			continue
		}

		if now.Sub(service.LastHeartbeat) > timeout {
			service.HeartbeatMissed++
			if service.HeartbeatMissed >= 3 {
				service.Status = model.StatusUnhealthy
			}
		}
	}
}

// loadServices 从数据库加载服务
func (s *ServiceImpl) loadServices() error {
	var services []*model.RegisteredService
	if err := s.db.Find(&services).Error; err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, service := range services {
		s.services[service.ID] = service
		s.servicesByType[service.ServiceType] = append(s.servicesByType[service.ServiceType], service.ID)
	}

	return nil
}

// saveService 保存服务到数据库
func (s *ServiceImpl) saveService(service *model.RegisteredService) error {
	capabilitiesJSON, _ := json.Marshal(service.Capabilities)
	resourcesJSON, _ := json.Marshal(service.Resources)
	performanceJSON, _ := json.Marshal(service.Performance)
	metadataJSON, _ := json.Marshal(service.Metadata)

	// 使用UPSERT
	return s.db.Exec(`
		INSERT INTO registered_services (
			id, service_type, version, hostname, ip_address, port,
			capabilities, resources, performance, status,
			last_heartbeat, heartbeat_missed, started_at, registered_at, updated_at,
			current_load, queue_size, processed_count, error_count,
			cpu_utilization, gpu_utilization, memory_usage, metadata
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			status = VALUES(status),
			last_heartbeat = VALUES(last_heartbeat),
			heartbeat_missed = VALUES(heartbeat_missed),
			current_load = VALUES(current_load),
			queue_size = VALUES(queue_size),
			processed_count = VALUES(processed_count),
			error_count = VALUES(error_count),
			cpu_utilization = VALUES(cpu_utilization),
			gpu_utilization = VALUES(gpu_utilization),
			memory_usage = VALUES(memory_usage),
			updated_at = VALUES(updated_at)
	`, service.ID, service.ServiceType, service.Version, service.Hostname, service.IPAddress, service.Port,
		string(capabilitiesJSON), string(resourcesJSON), string(performanceJSON), string(service.Status),
		service.LastHeartbeat, service.HeartbeatMissed, service.StartedAt, service.RegisteredAt, service.UpdatedAt,
		service.CurrentLoad, service.QueueSize, service.ProcessedCount, service.ErrorCount,
		service.CPUUtilization, service.GPUUtilization, service.MemoryUsage, string(metadataJSON)).Error
}

// generateServiceID 生成服务ID
func generateServiceID(serviceType string) string {
	return serviceType + "-" + uuid.New().String()[:8]
}

// generateToken 生成token
func generateToken() string {
	return uuid.New().String()
}

// validateToken 验证token
func validateToken(serviceID, token string) bool {
	// TODO: 实现真正的token验证
	// 简单实现：检查格式
	return len(token) > 0
}
