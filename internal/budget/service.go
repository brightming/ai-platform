package budget

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

// Service 成本预算控制服务
type Service struct {
	db          *gorm.DB
	mu          sync.RWMutex
	budgets     map[string]*Budget    // budget_id -> Budget
	spendings   map[string]*Spending   // budget_id -> Spending
	alertCh     chan *BudgetAlert
	configStore ConfigStore
}

// Budget 预算
type Budget struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`        // global, service, tenant
	TargetID    string    `json:"target_id"`   // service_id or tenant_id
	Amount      float64   `json:"amount"`
	Period      string    `json:"period"`      // daily, weekly, monthly
	PeriodStart time.Time `json:"period_start"`
	Alerts      []*AlertThreshold `json:"alerts"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// AlertThreshold 告警阈值
type AlertThreshold struct {
	At     float64 `json:"at"`           // 0.8 = 80%
	Action string   `json:"action"`       // notify, switch_to_third_party, block
	Enabled bool    `json:"enabled"`
}

// Spending 花费记录
type Spending struct {
	BudgetID    string    `json:"budget_id"`
	Amount      float64   `json:"amount"`
	PeriodStart time.Time `json:"period_start"`
	Records     []*CostRecord `json:"records"`
}

// CostRecord 成本记录
type CostRecord struct {
	ID         string    `json:"id"`
	RequestID  string    `json:"request_id"`
	Feature    string    `json:"feature"`
	Provider   string    `json:"provider"`
	Amount     float64   `json:"amount"`
	Timestamp  time.Time `json:"timestamp"`
}

// BudgetAlert 预算告警
type BudgetAlert struct {
	BudgetID    string    `json:"budget_id"`
	BudgetName  string    `json:"budget_name"`
	Type        string    `json:"type"`        // warning, critical
	UsedAmount  float64   `json:"used_amount"`
	TotalAmount float64   `json:"total_amount"`
	Percentage  float64   `json:"percentage"`
	Timestamp   time.Time `json:"timestamp"`
}

// ConfigStore 配置存储接口
type ConfigStore interface {
	GetFeature(id string) (*model.Feature, error)
}

// NewService 创建成本预算控制服务
func NewService(db *gorm.DB, configStore ConfigStore) *Service {
	s := &Service{
		db:          db,
		budgets:     make(map[string]*Budget),
		spendings:   make(map[string]*Spending),
		alertCh:     make(chan *BudgetAlert, 100),
		configStore: configStore,
	}
	// 加载预算配置
	s.loadBudgets()
	// 启动成本统计
	go s.startCostTracking()
	return s
}

// CheckBudget 检查预算
func (s *Service) CheckBudget(ctx context.Context, feature, tenantID string, estimatedCost float64) (*BudgetCheckResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := &BudgetCheckResult{
		Allowed: true,
		Reason:  "",
	}

	// 检查全局预算
	if globalBudget, ok := s.budgets["global"]; ok {
		if spending, ok := s.spendings["global"]; ok {
			used := spending.Amount + estimatedCost
			if used > globalBudget.Amount {
				result.Allowed = false
				result.Reason = "global budget exceeded"
				return result, nil
			}
			result.GlobalBudget = &BudgetInfo{
				Total:   globalBudget.Amount,
				Used:    spending.Amount,
				Remaining: globalBudget.Amount - spending.Amount,
				Percentage: (spending.Amount / globalBudget.Amount) * 100,
			}
		}
	}

	// 检查服务级预算
	serviceBudgetID := fmt.Sprintf("service:%s", feature)
	if serviceBudget, ok := s.budgets[serviceBudgetID]; ok {
		if spending, ok := s.spendings[serviceBudgetID]; ok {
			used := spending.Amount + estimatedCost
			if used > serviceBudget.Amount {
				result.Allowed = false
				result.Reason = fmt.Sprintf("service budget for %s exceeded", feature)
				return result, nil
			}
			result.ServiceBudget = &BudgetInfo{
				Total:   serviceBudget.Amount,
				Used:    spending.Amount,
				Remaining: serviceBudget.Amount - spending.Amount,
				Percentage: (spending.Amount / serviceBudget.Amount) * 100,
			}
		}
	}

	// 检查租户级预算
	if tenantID != "" {
		tenantBudgetID := fmt.Sprintf("tenant:%s", tenantID)
		if tenantBudget, ok := s.budgets[tenantBudgetID]; ok {
			if spending, ok := s.spendings[tenantBudgetID]; ok {
				used := spending.Amount + estimatedCost
				if used > tenantBudget.Amount {
					result.Allowed = false
					result.Reason = fmt.Sprintf("tenant budget for %s exceeded", tenantID)
					return result, nil
				}
				result.TenantBudget = &BudgetInfo{
					Total:   tenantBudget.Amount,
					Used:    spending.Amount,
					Remaining: tenantBudget.Amount - spending.Amount,
					Percentage: (spending.Amount / tenantBudget.Amount) * 100,
				}
			}
		}
	}

	// 检查是否需要触发告警
	s.checkAlerts(result)

	return result, nil
}

// RecordCost 记录成本
func (s *Service) RecordCost(record *CostRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 获取特性配置
	feature, err := s.configStore.GetFeature(record.Feature)
	if err != nil {
		return err
	}

	// 计算成本分配
	// self_hosted vs third_party
	var costType string
	if record.Provider == "self_hosted" {
		costType = "self_hosted"
	} else {
		costType = "third_party_" + record.Provider
	}

	// 更新各预算的花费
	s.updateSpending("global", record.Amount)
	s.updateSpending(fmt.Sprintf("service:%s", record.Feature), record.Amount)

	// 更新成本统计表
	return s.saveCostRecord(record, costType, feature.Cost)
}

// BudgetCheckResult 预算检查结果
type BudgetCheckResult struct {
	Allowed       bool         `json:"allowed"`
	Reason        string       `json:"reason,omitempty"`
	GlobalBudget  *BudgetInfo  `json:"global_budget,omitempty"`
	ServiceBudget *BudgetInfo  `json:"service_budget,omitempty"`
	TenantBudget  *BudgetInfo  `json:"tenant_budget,omitempty"`
}

// BudgetInfo 预算信息
type BudgetInfo struct {
	Total      float64 `json:"total"`
	Used       float64 `json:"used"`
	Remaining  float64 `json:"remaining"`
	Percentage float64 `json:"percentage"`
}

// CreateBudget 创建预算
func (s *Service) CreateBudget(budget *Budget) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if budget.ID == "" {
		budget.ID = generateBudgetID(budget.Type, budget.TargetID)
	}

	budget.CreatedAt = time.Now()
	budget.UpdatedAt = time.Now()

	// 设置默认告警阈值
	if len(budget.Alerts) == 0 {
		budget.Alerts = []*AlertThreshold{
			{At: 0.7, Action: "notify", Enabled: true},
			{At: 0.9, Action: "switch_to_third_party", Enabled: true},
		}
	}

	// 保存到数据库
	if err := s.db.Table("budgets").Create(budget).Error; err != nil {
		return err
	}

	s.budgets[budget.ID] = budget
	s.spendings[budget.ID] = &Spending{
		BudgetID:    budget.ID,
		Amount:      0,
		PeriodStart: time.Now(),
		Records:     make([]*CostRecord, 0),
	}

	return nil
}

// UpdateBudget 更新预算
func (s *Service) UpdateBudget(id string, budget *Budget) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	updates := map[string]interface{}{
		"amount":    budget.Amount,
		"period":    budget.Period,
		"alerts":    budget.Alerts,
		"updated_at": time.Now(),
	}

	if err := s.db.Table("budgets").Where("id = ?", id).Updates(updates).Error; err != nil {
		return err
	}

	if existing, ok := s.budgets[id]; ok {
		existing.Amount = budget.Amount
		existing.Period = budget.Period
		existing.Alerts = budget.Alerts
		existing.UpdatedAt = time.Now()
	}

	return nil
}

// GetBudget 获取预算
func (s *Service) GetBudget(id string) (*Budget, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	budget, ok := s.budgets[id]
	if !ok {
		return nil, fmt.Errorf("budget not found: %s", id)
	}

	return budget, nil
}

// ListBudgets 列出预算
func (s *Service) ListBudgets(filter *BudgetFilter) ([]*Budget, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var budgets []*Budget
	for _, budget := range s.budgets {
		if filter.Type != "" && budget.Type != filter.Type {
			continue
		}
		if filter.TargetID != "" && budget.TargetID != filter.TargetID {
			continue
		}
		budgets = append(budgets, budget)
	}

	return budgets, nil
}

// GetSpending 获取花费
func (s *Service) GetSpending(budgetID string) (*Spending, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	spending, ok := s.spendings[budgetID]
	if !ok {
		return nil, fmt.Errorf("spending not found: %s", budgetID)
	}

	return spending, nil
}

// WatchAlerts 监听告警
func (s *Service) WatchAlerts(ctx context.Context) <-chan *BudgetAlert {
	ch := make(chan *BudgetAlert, 10)
	go func() {
		defer close(ch)
		for {
			select {
			case <-ctx.Done():
				return
			case alert := <-s.alertCh:
				select {
				case ch <- alert:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return ch
}

// loadBudgets 加载预算
func (s *Service) loadBudgets() error {
	var budgets []*Budget
	if err := s.db.Table("budgets").Find(&budgets).Error; err != nil {
		// 表可能不存在，初始化默认预算
		return s.initDefaultBudgets()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, budget := range budgets {
		s.budgets[budget.ID] = budget
		s.spendings[budget.ID] = &Spending{
			BudgetID:    budget.ID,
			Amount:      0,
			PeriodStart: time.Now(),
			Records:     make([]*CostRecord, 0),
		}
	}

	return nil
}

// initDefaultBudgets 初始化默认预算
func (s *Service) initDefaultBudgets() error {
	defaultBudgets := []*Budget{
		{
			ID:       "global",
			Name:     "全局预算",
			Type:     "global",
			Amount:   30000,  // 月度3万元
			Period:   "monthly",
			Alerts: []*AlertThreshold{
				{At: 0.7, Action: "notify", Enabled: true},
				{At: 0.9, Action: "switch_to_third_party", Enabled: true},
			},
		},
		{
			ID:       "service:text_to_image",
			Name:     "文生图预算",
			Type:     "service",
			TargetID: "text_to_image",
			Amount:   1000,  // 日度1000元
			Period:   "daily",
			Alerts: []*AlertThreshold{
				{At: 0.8, Action: "notify", Enabled: true},
				{At: 0.95, Action: "block", Enabled: true},
			},
		},
	}

	for _, budget := range defaultBudgets {
		if err := s.CreateBudget(budget); err != nil {
			return err
		}
	}

	return nil
}

// updateSpending 更新花费
func (s *Service) updateSpending(budgetID string, amount float64) {
	spending, ok := s.spendings[budgetID]
	if !ok {
		spending = &Spending{
			BudgetID:    budgetID,
			Amount:      0,
			PeriodStart: time.Now(),
			Records:     make([]*CostRecord, 0),
		}
		s.spendings[budgetID] = spending
	}

	spending.Amount += amount
}

// checkAlerts 检查告警
func (s *Service) checkAlerts(result *BudgetCheckResult) {
	checkBudgetAlert := func(budget *Budget, spending *Spending) {
		percentage := (spending.Amount / budget.Amount) * 100
		for _, alert := range budget.Alerts {
			if !alert.Enabled {
				continue
			}
			if percentage >= alert.At*100 {
				// 检查是否已发送过类似告警
				alertType := "warning"
				if percentage >= 90 {
					alertType = "critical"
				}

				select {
				case s.alertCh <- &BudgetAlert{
					BudgetID:    budget.ID,
					BudgetName:  budget.Name,
					Type:        alertType,
					UsedAmount:  spending.Amount,
					TotalAmount: budget.Amount,
					Percentage:  percentage,
					Timestamp:   time.Now(),
				}:
				default:
				}
			}
		}
	}

	if result.GlobalBudget != nil {
		if budget, ok := s.budgets["global"]; ok {
			if spending, ok := s.spendings["global"]; ok {
				checkBudgetAlert(budget, spending)
			}
		}
	}

	if result.ServiceBudget != nil {
		serviceBudgetID := fmt.Sprintf("service:%s", result.ServiceBudget.ID)
		if budget, ok := s.budgets[serviceBudgetID]; ok {
			if spending, ok := s.spendings[serviceBudgetID]; ok {
				checkBudgetAlert(budget, spending)
			}
		}
	}
}

// saveCostRecord 保存成本记录到数据库
func (s *Service) saveCostRecord(record *CostRecord, costType string, featureCost *model.CostConfig) error {
	return s.db.Table("cost_records").Create(record).Error
}

// startCostTracking 启动成本统计
func (s *Service) startCostTracking() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.syncSpendingToDB()
	}
}

// syncSpendingToDB 同步花费到数据库
func (s *Service) syncSpendingToDB() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for budgetID, spending := range s.spendings {
		// 保存到统计表
		now := time.Now()
		s.db.Exec(`
			INSERT INTO cost_statistics (statistic_date, feature, provider_type, cost_total, updated_at)
			VALUES (CURDATE(), ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE
				cost_total = cost_total + VALUES(cost_total),
				updated_at = VALUES(updated_at)
		`, now.Format("2006-01-02"), spending.BudgetID, "mixed", spending.Amount, now)
	}
}

// BudgetFilter 预算过滤器
type BudgetFilter struct {
	Type     string
	TargetID string
	Limit    int
	Offset   int
}

// generateBudgetID 生成预算ID
func generateBudgetID(budgetType, targetID string) string {
	if budgetType == "global" {
		return "global"
	}
	return fmt.Sprintf("%s:%s", budgetType, targetID)
}
