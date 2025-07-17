package keymanager

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dbccccccc/tavily-load/internal/cache"
	"github.com/dbccccccc/tavily-load/internal/config"
	"github.com/dbccccccc/tavily-load/internal/errors"
	"github.com/dbccccccc/tavily-load/internal/repository"
	"github.com/dbccccccc/tavily-load/internal/usage"
	"github.com/dbccccccc/tavily-load/pkg/types"
	"github.com/sirupsen/logrus"
)

// Manager implements the KeyManager interface
type Manager struct {
	keys              []string
	currentIndex      int64
	keyRepo           *repository.KeyRepository
	usageCache        *cache.UsageCache
	blacklist         sync.Map // map[string]*types.BlacklistEntry
	keyStatus         sync.Map // map[string]*types.KeyStatus
	requestCounts     sync.Map // map[string]int64
	errorCounts       sync.Map // map[string]int64
	lastUsed          sync.Map // map[string]time.Time
	config            *config.Config
	logger            *logrus.Logger
	usageTracker      *usage.Tracker
	selectionStrategy types.SelectionStrategy
	mu                sync.RWMutex
	startTime         time.Time
	ctx               context.Context
}

// NewManager creates a new key manager
func NewManager(cfg *config.Config, logger *logrus.Logger, keyRepo *repository.KeyRepository, usageCache *cache.UsageCache) (*Manager, error) {
	ctx := context.Background()
	manager := &Manager{
		config:            cfg,
		logger:            logger,
		keyRepo:           keyRepo,
		usageCache:        usageCache,
		usageTracker:      usage.NewTracker(cfg, logger, usageCache),
		selectionStrategy: types.StrategyPlanFirst,
		startTime:         time.Now(),
		ctx:               ctx,
	}

	if err := manager.loadKeys(); err != nil {
		return nil, fmt.Errorf("failed to load keys: %w", err)
	}

	manager.initializeKeyStatus()
	return manager, nil
}

// loadKeys loads API keys from the database
func (m *Manager) loadKeys() error {
	ctx, cancel := context.WithTimeout(m.ctx, 10*time.Second)
	defer cancel()

	apiKeys, err := m.keyRepo.GetAllActiveKeys(ctx)
	if err != nil {
		return fmt.Errorf("failed to load keys from database: %w", err)
	}

	if len(apiKeys) == 0 {
		return fmt.Errorf("no active API keys found in database")
	}

	var keys []string
	for _, apiKey := range apiKeys {
		keys = append(keys, apiKey.KeyValue)
	}

	m.keys = keys
	m.currentIndex = int64(m.config.StartIndex % len(keys))

	m.logger.Infof("Loaded %d API keys from database", len(keys))
	return nil
}

// initializeKeyStatus initializes the status for all keys
func (m *Manager) initializeKeyStatus() {
	for _, key := range m.keys {
		m.keyStatus.Store(key, &types.KeyStatus{
			Active:       true,
			ErrorCount:   0,
			RequestCount: 0,
			LastUsed:     time.Time{},
		})
		requestCount := int64(0)
		errorCount := int64(0)
		m.requestCounts.Store(key, &requestCount)
		m.errorCounts.Store(key, &errorCount)
	}
}

// GetNextKey returns the next available API key using the current strategy
func (m *Manager) GetNextKey() (string, error) {
	return m.GetNextKeyWithStrategy(m.selectionStrategy)
}

// GetNextKeyWithStrategy returns the next available API key using the specified strategy
func (m *Manager) GetNextKeyWithStrategy(strategy types.SelectionStrategy) (string, error) {
	// Try strategy-based selection first
	if strategy == types.StrategyPlanFirst {
		if key, err := m.usageTracker.GetOptimalKey(strategy); err == nil {
			// Verify the key is not blacklisted
			if _, blacklisted := m.blacklist.Load(key); !blacklisted {
				m.updateKeyUsage(key)
				return key, nil
			}
		}
	}

	// Fallback to round-robin selection
	return m.getRoundRobinKey()
}

// getRoundRobinKey returns the next available API key using round-robin
func (m *Manager) getRoundRobinKey() (string, error) {
	m.mu.RLock()
	totalKeys := len(m.keys)
	m.mu.RUnlock()

	if totalKeys == 0 {
		return "", errors.NewTavilyError(errors.ErrorTypeNoKeysAvailable, "no API keys available", 500)
	}

	// Try to find an active key, starting from current index
	for i := 0; i < totalKeys; i++ {
		index := atomic.AddInt64(&m.currentIndex, 1) % int64(totalKeys)

		m.mu.RLock()
		key := m.keys[index]
		m.mu.RUnlock()

		// Check if key is blacklisted
		if _, blacklisted := m.blacklist.Load(key); blacklisted {
			continue
		}

		// Update usage statistics
		m.updateKeyUsage(key)
		keyPreview := key
		if len(key) > 12 {
			keyPreview = key[:12] + "..."
		}
		m.logger.Debugf("Selected key: %s (index: %d)", keyPreview, index)
		return key, nil
	}

	return "", errors.NewTavilyError(errors.ErrorTypeNoKeysAvailable, "all API keys are blacklisted", 500)
}

// BlacklistKey adds a key to the blacklist
func (m *Manager) BlacklistKey(key string, permanent bool) {
	now := time.Now()
	reason := "temporary error"
	var until *time.Time
	
	if permanent {
		reason = "permanent error"
	} else {
		// Temporary blacklist for 5 minutes
		tempUntil := now.Add(5 * time.Minute)
		until = &tempUntil
	}

	// Get current error count
	errorCount := int(atomic.LoadInt64(m.getErrorCountPtr(key)))

	// Blacklist in database
	ctx, cancel := context.WithTimeout(m.ctx, 5*time.Second)
	defer cancel()
	
	if err := m.keyRepo.BlacklistKey(ctx, key, reason, permanent, until); err != nil {
		m.logger.WithError(err).Error("Failed to blacklist key in database")
	}

	// Cache blacklist status
	if err := m.usageCache.SetBlacklistStatus(ctx, key, true, reason, until); err != nil {
		m.logger.WithError(err).Warn("Failed to cache blacklist status")
	}

	entry := &types.BlacklistEntry{
		Key:           key,
		Reason:        reason,
		BlacklistedAt: now,
		Permanent:     permanent,
		ErrorCount:    errorCount,
	}

	m.blacklist.Store(key, entry)

	// Update key status
	if statusInterface, ok := m.keyStatus.Load(key); ok {
		status := statusInterface.(*types.KeyStatus)
		status.Active = false
		status.BlacklistedAt = now
		status.Permanent = permanent
		m.keyStatus.Store(key, status)
	}

	logLevel := logrus.InfoLevel
	if permanent {
		logLevel = logrus.WarnLevel
	}

	keyPreview := key
	if len(key) > 12 {
		keyPreview = key[:12] + "..."
	}
	m.logger.WithField("key", keyPreview).
		WithField("permanent", permanent).
		WithField("error_count", errorCount).
		Log(logLevel, "Key blacklisted")
}

// ResetKeys clears all blacklisted keys and resets statistics
func (m *Manager) ResetKeys() {
	m.blacklist.Range(func(key, value interface{}) bool {
		m.blacklist.Delete(key)
		return true
	})

	// Reset key status
	for _, key := range m.keys {
		m.keyStatus.Store(key, &types.KeyStatus{
			Active:       true,
			ErrorCount:   0,
			RequestCount: 0,
			LastUsed:     time.Time{},
		})
		requestCount := int64(0)
		errorCount := int64(0)
		m.requestCounts.Store(key, &requestCount)
		m.errorCounts.Store(key, &errorCount)
	}

	m.logger.Info("All keys reset and blacklist cleared")
}

// RecordError records an error for a specific key
func (m *Manager) RecordError(key string, err error) {
	atomic.AddInt64(m.getErrorCountPtr(key), 1)

	// Update key status
	if statusInterface, ok := m.keyStatus.Load(key); ok {
		status := statusInterface.(*types.KeyStatus)
		status.ErrorCount++
		status.LastError = err.Error()
		m.keyStatus.Store(key, status)
	}

	// Check if we should blacklist the key
	errorCount := atomic.LoadInt64(m.getErrorCountPtr(key))
	if int(errorCount) >= m.config.BlacklistThreshold {
		permanent := false
		if tavilyErr, ok := err.(*errors.TavilyError); ok {
			permanent = tavilyErr.IsPermanent()
		}
		m.BlacklistKey(key, permanent)
	}
}

// GetStats returns current statistics
func (m *Manager) GetStats() types.KeyStats {
	m.mu.RLock()
	totalKeys := len(m.keys)
	currentIndex := int(atomic.LoadInt64(&m.currentIndex)) % totalKeys
	m.mu.RUnlock()

	stats := types.KeyStats{
		TotalKeys:     totalKeys,
		CurrentIndex:  currentIndex,
		RequestCounts: make(map[string]int),
		ErrorCounts:   make(map[string]int),
		LastUsed:      make(map[string]time.Time),
		KeyStatus:     make(map[string]types.KeyStatus),
	}

	activeKeys := 0
	blacklistedKeys := 0

	for _, key := range m.keys {
		// Get request count
		if countInterface, ok := m.requestCounts.Load(key); ok {
			stats.RequestCounts[key] = int(atomic.LoadInt64(countInterface.(*int64)))
		}

		// Get error count
		if countInterface, ok := m.errorCounts.Load(key); ok {
			stats.ErrorCounts[key] = int(atomic.LoadInt64(countInterface.(*int64)))
		}

		// Get last used
		if timeInterface, ok := m.lastUsed.Load(key); ok {
			stats.LastUsed[key] = timeInterface.(time.Time)
		}

		// Get key status
		if statusInterface, ok := m.keyStatus.Load(key); ok {
			status := *statusInterface.(*types.KeyStatus)
			stats.KeyStatus[key] = status

			if status.Active {
				activeKeys++
			} else {
				blacklistedKeys++
			}
		}
	}

	stats.ActiveKeys = activeKeys
	stats.BlacklistedKeys = blacklistedKeys

	return stats
}

// GetBlacklist returns current blacklisted keys
func (m *Manager) GetBlacklist() []types.BlacklistEntry {
	var entries []types.BlacklistEntry

	m.blacklist.Range(func(key, value interface{}) bool {
		entry := *value.(*types.BlacklistEntry)
		entries = append(entries, entry)
		return true
	})

	return entries
}

// Helper methods for atomic operations
func (m *Manager) getRequestCountPtr(key string) *int64 {
	if countInterface, ok := m.requestCounts.Load(key); ok {
		return countInterface.(*int64)
	}

	// Initialize if not exists
	count := int64(0)
	m.requestCounts.Store(key, &count)
	return &count
}

// updateKeyUsage updates usage statistics for a key
func (m *Manager) updateKeyUsage(key string) {
	now := time.Now()
	m.lastUsed.Store(key, now)
	atomic.AddInt64(m.getRequestCountPtr(key), 1)

	// Update in database
	ctx, cancel := context.WithTimeout(m.ctx, 2*time.Second)
	defer cancel()
	
	go func() {
		if err := m.keyRepo.UpdateKeyUsage(ctx, key, 1, 0); err != nil {
			m.logger.WithError(err).Debug("Failed to update key usage in database")
		}
	}()

	// Update in cache
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		if err := m.usageCache.IncrementKeyUsage(ctx, key, true); err != nil {
			m.logger.WithError(err).Debug("Failed to update key usage in cache")
		}
	}()

	// Update key status
	if statusInterface, ok := m.keyStatus.Load(key); ok {
		status := statusInterface.(*types.KeyStatus)
		status.LastUsed = now
		status.RequestCount++
		m.keyStatus.Store(key, status)
	}
}

// SetSelectionStrategy sets the key selection strategy
func (m *Manager) SetSelectionStrategy(strategy types.SelectionStrategy) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.selectionStrategy = strategy
	m.logger.WithField("strategy", strategy).Info("Selection strategy updated")
}

// GetSelectionStrategy returns the current selection strategy
func (m *Manager) GetSelectionStrategy() types.SelectionStrategy {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.selectionStrategy
}

// UpdateUsageFromAPI fetches and updates usage information for all keys
func (m *Manager) UpdateUsageFromAPI() error {
	m.mu.RLock()
	keys := make([]string, len(m.keys))
	copy(keys, m.keys)
	m.mu.RUnlock()

	var errors []error
	for _, key := range keys {
		if usage, err := m.usageTracker.FetchUsageFromAPI(key); err == nil {
			m.usageTracker.UpdateUsage(key, usage)
		} else {
			keyPreview := key
			if len(key) > 12 {
				keyPreview = key[:12] + "..."
			}
			errors = append(errors, fmt.Errorf("failed to update usage for key %s: %w", keyPreview, err))
		}
	}

	if len(errors) > 0 {
		m.logger.WithField("errors", len(errors)).Warn("Some keys failed to update usage")
		return fmt.Errorf("failed to update usage for %d keys", len(errors))
	}

	return nil
}

// GetUsageAnalytics returns comprehensive usage analytics
func (m *Manager) GetUsageAnalytics() *types.UsageAnalytics {
	allUsage := m.usageTracker.GetAllUsage()
	keyStats := m.GetStats()

	analytics := &types.UsageAnalytics{
		TotalKeys:           keyStats.TotalKeys,
		ActiveKeys:          keyStats.ActiveKeys,
		KeysWithUsage:       len(allUsage),
		RecommendedStrategy: m.usageTracker.GetRecommendedStrategy(),
		KeyAnalytics:        make(map[string]*types.KeyAnalytics),
		StrategyMetrics:     make(map[types.SelectionStrategy]*types.StrategyMetrics),
	}

	var totalPlanUsage, totalPlanLimit, totalPaygoUsage, totalPaygoLimit int
	var totalPlanUtil, totalPaygoUtil float64

	for key, usage := range allUsage {
		remaining, _ := m.usageTracker.CalculateRemainingPoints(key)

		keyAnalytics := &types.KeyAnalytics{
			Key:             key,
			Usage:           usage,
			RemainingPoints: remaining,
			RequestCount:    int64(keyStats.RequestCounts[key]),
			ErrorCount:      int64(keyStats.ErrorCounts[key]),
			LastUsed:        keyStats.LastUsed[key],
			LastUpdated:     time.Now(),
		}

		if remaining != nil {
			keyAnalytics.HealthScore = m.calculateHealthScore(keyAnalytics)
			keyAnalytics.CostEfficiency = m.calculateCostEfficiency(keyAnalytics)
			keyAnalytics.RecommendedUse = keyAnalytics.HealthScore > 0.5 && remaining.TotalRemaining > 0
		}

		analytics.KeyAnalytics[key] = keyAnalytics

		// Aggregate totals
		totalPlanUsage += usage.Account.PlanUsage
		totalPlanLimit += usage.Account.PlanLimit
		totalPaygoUsage += usage.Account.PaygoUsage
		totalPaygoLimit += usage.Account.PaygoLimit

		if remaining != nil {
			totalPlanUtil += remaining.PlanUtilization
			totalPaygoUtil += remaining.PaygoUtilization
		}
	}

	analytics.TotalPlanUsage = totalPlanUsage
	analytics.TotalPlanLimit = totalPlanLimit
	analytics.TotalPaygoUsage = totalPaygoUsage
	analytics.TotalPaygoLimit = totalPaygoLimit

	if len(allUsage) > 0 {
		analytics.AveragePlanUtil = totalPlanUtil / float64(len(allUsage))
		analytics.AveragePaygoUtil = totalPaygoUtil / float64(len(allUsage))
	}

	return analytics
}

// Helper methods for analytics calculations
func (m *Manager) calculateHealthScore(analytics *types.KeyAnalytics) float64 {
	if analytics.RequestCount == 0 {
		return 1.0
	}

	errorRate := float64(analytics.ErrorCount) / float64(analytics.RequestCount)
	healthScore := 1.0 - errorRate

	// Factor in remaining quota
	if analytics.RemainingPoints != nil {
		if analytics.RemainingPoints.TotalRemaining <= 0 {
			healthScore *= 0.1 // Severely penalize exhausted keys
		} else {
			// Bonus for having quota remaining
			quotaBonus := float64(analytics.RemainingPoints.TotalRemaining) / 1000.0
			if quotaBonus > 1.0 {
				quotaBonus = 1.0
			}
			healthScore = (healthScore * 0.7) + (quotaBonus * 0.3)
		}
	}

	if healthScore < 0 {
		healthScore = 0
	}
	if healthScore > 1 {
		healthScore = 1
	}

	return healthScore
}

func (m *Manager) calculateCostEfficiency(analytics *types.KeyAnalytics) float64 {
	if analytics.Usage == nil || analytics.RemainingPoints == nil {
		return 0.5
	}

	// Cost efficiency favors plan credits over paygo
	planWeight := 0.8
	paygoWeight := 0.2

	planEfficiency := 1.0 - analytics.RemainingPoints.PlanUtilization
	paygoEfficiency := 1.0 - analytics.RemainingPoints.PaygoUtilization

	efficiency := (planEfficiency * planWeight) + (paygoEfficiency * paygoWeight)

	// Factor in health score
	efficiency *= analytics.HealthScore

	return efficiency
}

// GetUsageTracker returns the usage tracker instance
func (m *Manager) GetUsageTracker() types.UsageTracker {
	return m.usageTracker
}

func (m *Manager) getErrorCountPtr(key string) *int64 {
	if countInterface, ok := m.errorCounts.Load(key); ok {
		return countInterface.(*int64)
	}

	// Initialize if not exists
	count := int64(0)
	m.errorCounts.Store(key, &count)
	return &count
}
