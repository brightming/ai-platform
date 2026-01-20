package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/brightming/ai-platform/pkg/model"
	"gorm.io/gorm"
)

// ServiceImpl 功能配置服务实现
type ServiceImpl struct {
	db        *gorm.DB
	mu        sync.RWMutex
	cache     map[string]*model.Feature
	configCh  chan *ConfigChangeEvent
}

// ConfigChangeEvent 配置变更事件
type ConfigChangeEvent struct {
	Type      string    // create, update, delete
	FeatureID string
	Feature   *model.Feature
	Timestamp time.Time
}

// NewService 创建功能配置服务
func NewService(db *gorm.DB) *ServiceImpl {
	s := &ServiceImpl{
		db:       db,
		cache:    make(map[string]*model.Feature),
		configCh: make(chan *ConfigChangeEvent, 100),
	}
	// 启动时加载缓存
	s.loadCache()
	return s
}

// loadCache 加载配置到缓存
func (s *ServiceImpl) loadCache() error {
	var features []*model.Feature
	if err := s.db.Where("enabled = ?", true).
		Preload("Providers").
		Find(&features).Error; err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache = make(map[string]*model.Feature)
	for _, f := range features {
		s.cache[f.ID] = f
	}
	return nil
}

// CreateFeature 创建功能
func (s *ServiceImpl) CreateFeature(feature *model.Feature) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		// 创建功能
		if err := tx.Create(feature).Error; err != nil {
			return err
		}

		// 创建Providers
		for _, p := range feature.Providers {
			p.FeatureID = feature.ID
			if err := tx.Create(p).Error; err != nil {
				return err
			}
		}

		// 记录变更日志
		changeLog := &model.ConfigChangeLog{
			ConfigType:    "feature",
			ConfigID:      feature.ID,
			Action:        "create",
			NewValue:      toJSON(feature),
			ChangedBy:     "system", // TODO: 从上下文获取用户
		}
		if err := tx.Table("config_change_logs").Create(changeLog).Error; err != nil {
			return err
		}

		return nil
	}); err != nil {
		return err
	}

	// 更新缓存
	s.cache[feature.ID] = feature

	// 发送变更事件
	s.publishEvent("create", feature.ID, feature)

	return nil
}

// UpdateFeature 更新功能
func (s *ServiceImpl) UpdateFeature(id string, feature *model.Feature) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 获取旧值
	oldFeature, err := s.GetFeature(id)
	if err != nil {
		return err
	}

	// 记录变更字段
	changedFields := []string{}
	if oldFeature.Name != feature.Name {
		changedFields = append(changedFields, "name")
	}
	if oldFeature.Description != feature.Description {
		changedFields = append(changedFields, "description")
	}
	if oldFeature.Enabled != feature.Enabled {
		changedFields = append(changedFields, "enabled")
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		// 更新功能
		if err := tx.Model(&model.Feature{}).
			Where("id = ?", id).
			Updates(map[string]interface{}{
				"name":        feature.Name,
				"description": feature.Description,
				"enabled":     feature.Enabled,
				"version":     gorm.Expr("version + 1"),
				"updated_at":  time.Now(),
			}).Error; err != nil {
			return err
		}

		// 记录变更日志
		changeLog := &model.ConfigChangeLog{
			ConfigType:    "feature",
			ConfigID:      id,
			Action:        "update",
			OldValue:      toJSON(oldFeature),
			NewValue:      toJSON(feature),
			ChangedFields: toJSON(changedFields),
			ChangedBy:     "system",
		}
		if err := tx.Table("config_change_logs").Create(changeLog).Error; err != nil {
			return err
		}

		return nil
	}); err != nil {
		return err
	}

	// 更新缓存
	s.cache[id] = feature

	// 发送变更事件
	s.publishEvent("update", id, feature)

	return nil
}

// DeleteFeature 删除功能
func (s *ServiceImpl) DeleteFeature(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 获取旧值
	oldFeature, err := s.GetFeature(id)
	if err != nil {
		return err
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		// 删除Providers
		if err := tx.Where("feature_id = ?", id).Delete(&model.ProviderConfig{}).Error; err != nil {
			return err
		}

		// 删除功能
		if err := tx.Where("id = ?", id).Delete(&model.Feature{}).Error; err != nil {
			return err
		}

		// 记录变更日志
		changeLog := &model.ConfigChangeLog{
			ConfigType:    "feature",
			ConfigID:      id,
			Action:        "delete",
			OldValue:      toJSON(oldFeature),
			ChangedBy:     "system",
		}
		if err := tx.Table("config_change_logs").Create(changeLog).Error; err != nil {
			return err
		}

		return nil
	}); err != nil {
		return err
	}

	// 删除缓存
	delete(s.cache, id)

	// 发送变更事件
	s.publishEvent("delete", id, oldFeature)

	return nil
}

// GetFeature 获取功能
func (s *ServiceImpl) GetFeature(id string) (*model.Feature, error) {
	// 先从缓存获取
	s.mu.RLock()
	if f, ok := s.cache[id]; ok {
		s.mu.RUnlock()
		// 重新加载Providers
		if err := s.db.Where("feature_id = ?", id).Find(&f.Providers).Error; err != nil {
			return nil, err
		}
		return f, nil
	}
	s.mu.RUnlock()

	// 从数据库获取
	var feature model.Feature
	if err := s.db.Where("id = ?", id).First(&feature).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("feature not found: %s", id)
		}
		return nil, err
	}

	// 加载Providers
	if err := s.db.Where("feature_id = ?", id).Find(&feature.Providers).Error; err != nil {
		return nil, err
	}

	return &feature, nil
}

// ListFeatures 列出功能
func (s *ServiceImpl) ListFeatures(filter *model.FeatureFilter) ([]*model.Feature, int, error) {
	var features []*model.Feature
	var total int64

	query := s.db.Model(&model.Feature{})

	if filter.Category != "" {
		query = query.Where("category = ?", filter.Category)
	}
	if filter.Enabled != nil {
		query = query.Where("enabled = ?", *filter.Enabled)
	}

	// 计算总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	if err := query.Offset(filter.Offset).
		Limit(filter.Limit).
		Order("created_at DESC").
		Find(&features).Error; err != nil {
		return nil, 0, err
	}

	// 加载Providers
	for _, f := range features {
		s.db.Where("feature_id = ?", f.ID).Find(&f.Providers)
	}

	return features, int(total), nil
}

// AddProvider 添加Provider
func (s *ServiceImpl) AddProvider(featureID string, provider *model.ProviderConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 验证功能存在
	var feature model.Feature
	if err := s.db.Where("id = ?", featureID).First(&feature).Error; err != nil {
		return fmt.Errorf("feature not found: %s", featureID)
	}

	provider.FeatureID = featureID
	if err := s.db.Create(provider).Error; err != nil {
		return err
	}

	// 更新缓存
	if f, ok := s.cache[featureID]; ok {
		f.Providers = append(f.Providers, provider)
	}

	// 记录变更日志
	changeLog := &model.ConfigChangeLog{
		ConfigType:    "provider",
		ConfigID:      provider.ID,
		Action:        "create",
		NewValue:      toJSON(provider),
		ChangedBy:     "system",
	}
	s.db.Table("config_change_logs").Create(changeLog)

	return nil
}

// UpdateProvider 更新Provider
func (s *ServiceImpl) UpdateProvider(featureID, providerID string, provider *model.ProviderConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	updates := make(map[string]interface{})
	if provider.Enabled != nil {
		updates["enabled"] = provider.Enabled
	}
	if provider.Priority != nil {
		updates["priority"] = provider.Priority
	}
	if provider.Weight != nil {
		updates["weight"] = provider.Weight
	}
	updates["updated_at"] = time.Now()

	if err := s.db.Model(&model.ProviderConfig{}).
		Where("id = ? AND feature_id = ?", providerID, featureID).
		Updates(updates).Error; err != nil {
		return err
	}

	// 更新缓存
	if f, ok := s.cache[featureID]; ok {
		for _, p := range f.Providers {
			if p.ID == providerID {
				if provider.Enabled != nil {
					p.Enabled = *provider.Enabled
				}
				if provider.Priority != nil {
					p.Priority = *provider.Priority
				}
				if provider.Weight != nil {
					p.Weight = *provider.Weight
				}
			}
		}
	}

	return nil
}

// RemoveProvider 删除Provider
func (s *ServiceImpl) RemoveProvider(featureID, providerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.db.Where("id = ? AND feature_id = ?", providerID, featureID).
		Delete(&model.ProviderConfig{}).Error; err != nil {
		return err
	}

	// 更新缓存
	if f, ok := s.cache[featureID]; ok {
		newProviders := make([]*model.ProviderConfig, 0, len(f.Providers))
		for _, p := range f.Providers {
			if p.ID != providerID {
				newProviders = append(newProviders, p)
			}
		}
		f.Providers = newProviders
	}

	return nil
}

// UpdateRoutingStrategy 更新路由策略
func (s *ServiceImpl) UpdateRoutingStrategy(featureID string, strategy *model.RoutingStrategy) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	routingJSON := toJSON(strategy)
	if err := s.db.Model(&model.Feature{}).
		Where("id = ?", featureID).
		Update("routing", routingJSON).Error; err != nil {
		return err
	}

	// 更新缓存
	if f, ok := s.cache[featureID]; ok {
		f.Routing = strategy
	}

	return nil
}

// GetFeatureByCategory 根据类别获取功能
func (s *ServiceImpl) GetFeatureByCategory(category string) ([]*model.Feature, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var features []*model.Feature
	if err := s.db.Where("category = ? AND enabled = ?", category, true).
		Preload("Providers").
		Find(&features).Error; err != nil {
		return nil, err
	}

	return features, nil
}

// WatchConfig 监听配置变更
func (s *ServiceImpl) WatchConfig(ctx context.Context) <-chan *ConfigChangeEvent {
	ch := make(chan *ConfigChangeEvent, 10)
	go func() {
		defer close(ch)
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-s.configCh:
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

// publishEvent 发布配置变更事件
func (s *ServiceImpl) publishEvent(eventType, featureID string, feature *model.Feature) {
	select {
	case s.configCh <- &ConfigChangeEvent{
		Type:      eventType,
		FeatureID: featureID,
		Feature:   feature,
		Timestamp: time.Now(),
	}:
	default:
		// channel满，丢弃事件
	}
}

// toJSON 转换为JSON字符串
func toJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}
