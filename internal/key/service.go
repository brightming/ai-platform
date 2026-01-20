package key

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/yijian/ai-platform/pkg/model"
	"github.com/yijian/ai-platform/pkg/storage/kms"
	"gorm.io/gorm"
)

// ServiceImpl API密钥服务实现
type ServiceImpl struct {
	db       *gorm.DB
	kms      *kms.KMSClient
	cache    *keyCache
	healthCh chan *model.HealthStatus
}

type keyCache struct {
	sync.RWMutex
	keys map[string]*model.APIKey
}

// NewService 创建API密钥管理服务
func NewService(db *gorm.DB, kmsClient *kms.KMSClient) *ServiceImpl {
	s := &ServiceImpl{
		db:       db,
		kms:      kmsClient,
		cache:    &keyCache{keys: make(map[string]*model.APIKey)},
		healthCh: make(chan *model.HealthStatus, 100),
	}
	// 启动时加载启用状态的密钥到缓存
	s.loadCache()
	// 启动健康检查
	go s.startHealthCheck()
	return s
}

// loadCache 加载密钥到缓存
func (s *ServiceImpl) loadCache() error {
	var keys []*model.APIKey
	if err := s.db.Where("enabled = ?", true).Find(&keys).Error; err != nil {
		return err
	}

	s.cache.Lock()
	defer s.cache.Unlock()
	for _, k := range keys {
		s.cache.keys[k.ID] = k
	}
	return nil
}

// CreateKey 创建密钥
func (s *ServiceImpl) CreateKey(req *model.CreateKeyRequest) (*model.APIKey, error) {
	// 生成ID
	if req.ID == "" {
		req.ID = generateKeyID()
	}

	// 生成随机数据密钥(DEK)
	dek := make([]byte, 32)
	if _, err := rand.Read(dek); err != nil {
		return nil, fmt.Errorf("generate DEK failed: %w", err)
	}

	// 使用KMS加密DEK
	encryptedDEK, err := s.kms.Encrypt(dek)
	if err != nil {
		return nil, fmt.Errorf("KMS encrypt failed: %w", err)
	}

	// 使用DEK加密API Key
	encryptedKey, err := encryptAPIKey(req.APIKey, dek)
	if err != nil {
		return nil, fmt.Errorf("encrypt API key failed: %w", err)
	}

	// 计算密钥hash
	hash := sha256.Sum256([]byte(req.APIKey))

	// 创建记录
	key := &model.APIKey{
		ID:                 req.ID,
		Vendor:             req.Vendor,
		Service:            req.Service,
		EncryptedDEK:       hex.EncodeToString(encryptedDEK),
		EncryptedKey:       hex.EncodeToString(encryptedKey),
		KeyHash:            hex.EncodeToString(hash[:]),
		KeyAlias:           req.KeyAlias,
		Tier:               req.Tier,
		QuotaDailyRequests: req.QuotaDailyRequests,
		QuotaDailyTokens:   req.QuotaDailyTokens,
		QuotaMonthlyRequests: req.QuotaMonthlyRequests,
		Enabled:            true,
		AutoRotate:         req.AutoRotate,
		RotateDays:         req.RotateDays,
		ExpiresAt:          req.ExpiresAt,
		CreatedBy:          "system", // TODO: 从上下文获取用户
	}

	if err := s.db.Create(key).Error; err != nil {
		return nil, err
	}

	// 更新缓存
	s.cache.Lock()
	s.cache.keys[key.ID] = key
	s.cache.Unlock()

	// 不返回敏感信息
	key.EncryptedDEK = ""
	key.EncryptedKey = ""

	return key, nil
}

// UpdateKey 更新密钥
func (s *ServiceImpl) UpdateKey(id string, req *model.UpdateKeyRequest) error {
	updates := make(map[string]interface{})

	if req.KeyAlias != nil {
		updates["key_alias"] = *req.KeyAlias
	}
	if req.Tier != nil {
		updates["tier"] = *req.Tier
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	if req.AutoRotate != nil {
		updates["auto_rotate"] = *req.AutoRotate
	}
	if req.RotateDays != nil {
		updates["rotate_days"] = *req.RotateDays
	}
	if req.ExpiresAt != nil {
		updates["expires_at"] = *req.ExpiresAt
	}
	if req.QuotaDailyRequests != nil {
		updates["quota_daily_requests"] = *req.QuotaDailyRequests
	}
	if req.QuotaDailyTokens != nil {
		updates["quota_daily_tokens"] = *req.QuotaDailyTokens
	}
	if req.QuotaMonthlyRequests != nil {
		updates["quota_monthly_requests"] = *req.QuotaMonthlyRequests
	}
	updates["updated_at"] = time.Now()

	if err := s.db.Model(&model.APIKey{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return err
	}

	// 更新缓存
	s.cache.Lock()
	defer s.cache.Unlock()
	if k, ok := s.cache.keys[id]; ok {
		s.db.Where("id = ?", id).First(k)
	}

	return nil
}

// DeleteKey 删除密钥
func (s *ServiceImpl) DeleteKey(id string) error {
	if err := s.db.Where("id = ?", id).Delete(&model.APIKey{}).Error; err != nil {
		return err
	}

	// 删除缓存
	s.cache.Lock()
	delete(s.cache.keys, id)
	s.cache.Unlock()

	return nil
}

// GetKey 获取密钥
func (s *ServiceImpl) GetKey(id string) (*model.APIKey, error) {
	var key model.APIKey
	if err := s.db.Where("id = ?", id).First(&key).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("key not found: %s", id)
		}
		return nil, err
	}
	return &key, nil
}

// ListKeys 列出密钥
func (s *ServiceImpl) ListKeys(filter *model.KeyFilter) ([]*model.APIKey, int, error) {
	var keys []*model.APIKey
	var total int64

	query := s.db.Model(&model.APIKey{})

	if filter.Vendor != "" {
		query = query.Where("vendor = ?", filter.Vendor)
	}
	if filter.Service != "" {
		query = query.Where("service = ?", filter.Service)
	}
	if filter.Enabled != nil {
		query = query.Where("enabled = ?", *filter.Enabled)
	}
	if filter.Tier != "" {
		query = query.Where("tier = ?", filter.Tier)
	}

	// 计算总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	if err := query.Offset(filter.Offset).
		Limit(filter.Limit).
		Order("created_at DESC").
		Find(&keys).Error; err != nil {
		return nil, 0, err
	}

	return keys, int(total), nil
}

// EnableKey 启用密钥
func (s *ServiceImpl) EnableKey(id string) error {
	return s.UpdateKey(id, &model.UpdateKeyRequest{
		Enabled: boolPtr(true),
	})
}

// DisableKey 禁用密钥
func (s *ServiceImpl) DisableKey(id string) error {
	return s.UpdateKey(id, &model.UpdateKeyRequest{
		Enabled: boolPtr(false),
	})
}

// RotateKey 轮换密钥
func (s *ServiceImpl) RotateKey(id string, req *model.RotateKeyRequest) (*model.APIKey, error) {
	// 获取旧密钥
	oldKey, err := s.GetKey(id)
	if err != nil {
		return nil, err
	}

	// 获取明文密钥
	oldAPIKey, err := s.getPlaintextKey(oldKey)
	if err != nil {
		return nil, err
	}

	// 验证旧密钥仍然有效
	if err := s.validateKey(oldKey.Vendor, oldKey.Service, oldAPIKey); err != nil {
		return nil, fmt.Errorf("old key validation failed: %w", err)
	}

	// 确定新密钥
	newAPIKey := oldAPIKey
	if req.NewAPIKey != "" {
		newAPIKey = req.NewAPIKey
	}

	// 生成新密钥记录
	newKeyID := generateKeyID()
	createReq := &model.CreateKeyRequest{
		ID:                    newKeyID,
		Vendor:                oldKey.Vendor,
		Service:               oldKey.Service,
		KeyAlias:              oldKey.KeyAlias + "-rotated",
		Tier:                  oldKey.Tier,
		APIKey:                newAPIKey,
		QuotaDailyRequests:    oldKey.QuotaDailyRequests,
		QuotaDailyTokens:      oldKey.QuotaDailyTokens,
		QuotaMonthlyRequests:  oldKey.QuotaMonthlyRequests,
		AutoRotate:            oldKey.AutoRotate,
		RotateDays:            oldKey.RotateDays,
	}

	newKey, err := s.CreateKey(createReq)
	if err != nil {
		return nil, err
	}

	// 禁用旧密钥
	if err := s.DisableKey(id); err != nil {
		return nil, err
	}

	return newKey, nil
}

// GetActiveKey 获取激活状态的密钥
func (s *ServiceImpl) GetActiveKey(vendor, service string) (*model.APIKey, error) {
	var key model.APIKey
	if err := s.db.Where("vendor = ? AND service = ? AND enabled = ?", vendor, service, true).
		Order("tier ASC, created_at ASC").
		First(&key).Error; err != nil {
		return nil, fmt.Errorf("no active key found for %s/%s", vendor, service)
	}
	return &key, nil
}

// GetUsage 获取使用统计
func (s *ServiceImpl) GetUsage(id, period string) (*model.UsageStats, error) {
	// TODO: 实现使用统计查询
	stats := &model.UsageStats{
		KeyID:           id,
		Period:          period,
		TotalRequests:   0,
		SuccessRequests: 0,
		FailedRequests:  0,
		TotalTokens:     0,
		TotalImages:     0,
		TotalCost:       0,
	}
	return stats, nil
}

// HealthCheck 健康检查
func (s *ServiceImpl) HealthCheck(id string) (*model.HealthStatus, error) {
	key, err := s.GetKey(id)
	if err != nil {
		return nil, err
	}

	if !key.Enabled {
		return &model.HealthStatus{
			KeyID:        id,
			Status:       "unhealthy",
			LastCheckAt:  time.Now(),
			ErrorMessage: "key is disabled",
		}, nil
	}

	if key.ExpiresAt != nil && key.ExpiresAt.Before(time.Now()) {
		return &model.HealthStatus{
			KeyID:        id,
			Status:       "unhealthy",
			LastCheckAt:  time.Now(),
			ErrorMessage: "key has expired",
		}, nil
	}

	// 实际检查密钥有效性
	apiKey, err := s.getPlaintextKey(key)
	if err != nil {
		return &model.HealthStatus{
			KeyID:        id,
			Status:       "unhealthy",
			LastCheckAt:  time.Now(),
			ErrorMessage: err.Error(),
		}, nil
	}

	start := time.Now()
	err = s.validateKey(key.Vendor, key.Service, apiKey)
	latency := int(time.Since(start).Milliseconds())

	if err != nil {
		return &model.HealthStatus{
			KeyID:        id,
			Status:       "degraded",
			LastCheckAt:  time.Now(),
			LatencyMs:    latency,
			ErrorMessage: err.Error(),
		}, nil
	}

	return &model.HealthStatus{
		KeyID:       id,
		Status:      "healthy",
		LastCheckAt: time.Now(),
		LatencyMs:   latency,
	}, nil
}

// GetPlaintextKey 获取明文密钥（内部使用）
func (s *ServiceImpl) getPlaintextKey(key *model.APIKey) (string, error) {
	// 先查缓存
	// TODO: 添加Redis缓存

	// 解密DEK
	dekBytes, err := s.kms.Decrypt(key.EncryptedDEK)
	if err != nil {
		return "", fmt.Errorf("KMS decrypt failed: %w", err)
	}

	// 解密API Key
	keyBytes, err := hex.DecodeString(key.EncryptedKey)
	if err != nil {
		return "", err
	}

	apiKey, err := decryptAPIKey(keyBytes, dekBytes)
	if err != nil {
		return "", fmt.Errorf("decrypt API key failed: %w", err)
	}

	return apiKey, nil
}

// validateKey 验证密钥有效性
func (s *ServiceImpl) validateKey(vendor, service, apiKey string) error {
	// TODO: 根据厂商和service调用验证接口
	return nil
}

// startHealthCheck 启动健康检查
func (s *ServiceImpl) startHealthCheck() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.checkAllKeys()
	}
}

// checkAllKeys 检查所有密钥
func (s *ServiceImpl) checkAllKeys() {
	var keys []*model.APIKey
	if err := s.db.Where("enabled = ?", true).Find(&keys).Error; err != nil {
		return
	}

	for _, key := range keys {
		status, _ := s.HealthCheck(key.ID)
		select {
		case s.healthCh <- status:
		default:
		}
	}
}

// generateKeyID 生成密钥ID
func generateKeyID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "key-" + hex.EncodeToString(b)
}

// boolPtr 返回bool指针
func boolPtr(b bool) *bool {
	return &b
}
