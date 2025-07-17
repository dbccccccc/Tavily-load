package usage

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/dbccccccc/tavily-load/internal/cache"
	"github.com/dbccccccc/tavily-load/internal/config"
	"github.com/dbccccccc/tavily-load/internal/errors"
	"github.com/dbccccccc/tavily-load/pkg/types"
	"github.com/sirupsen/logrus"
)

// Tracker implements the UsageTracker interface
type Tracker struct {
	config         *config.Config
	logger         *logrus.Logger
	httpClient     *http.Client
	usageCache     *cache.UsageCache
	memoryCache    sync.Map // map[string]*types.TavilyUsage - in-memory fallback
	analytics      sync.Map // map[string]*types.KeyAnalytics
	strategies     map[types.SelectionStrategy]*types.UsageStrategy
	mu             sync.RWMutex
	lastUpdate     time.Time
	updateInterval time.Duration
	ctx            context.Context
}

// NewTracker creates a new usage tracker
func NewTracker(cfg *config.Config, logger *logrus.Logger, usageCache *cache.UsageCache) *Tracker {
	client := &http.Client{
		Timeout: cfg.RequestTimeout,
		Transport: &http.Transport{
			IdleConnTimeout:       cfg.IdleConnTimeout,
			ResponseHeaderTimeout: cfg.ResponseTimeout,
			MaxIdleConns:          10,
			MaxIdleConnsPerHost:   5,
		},
	}

	tracker := &Tracker{
		config:         cfg,
		logger:         logger,
		httpClient:     client,
		usageCache:     usageCache,
		updateInterval: 5 * time.Minute, // Update usage every 5 minutes
		strategies:     make(map[types.SelectionStrategy]*types.UsageStrategy),
		ctx:            context.Background(),
	}

	tracker.initializeStrategies()
	return tracker
}

// initializeStrategies sets up the available selection strategies
func (t *Tracker) initializeStrategies() {
	t.strategies[types.StrategyPlanFirst] = &types.UsageStrategy{
		Strategy:         types.StrategyPlanFirst,
		Description:      "Default: Prefer plan credits over paygo, only switch to paid when no plans available",
		PreferPlan:       true,
		PreferPaygo:      false,
		ThresholdPercent: 0.9,
		CostWeight:       0.1,
		BalanceWeight:    0.9,
	}

	t.strategies[types.StrategyRoundRobin] = &types.UsageStrategy{
		Strategy:         types.StrategyRoundRobin,
		Description:      "Round-robin selection across all available keys",
		PreferPlan:       false,
		PreferPaygo:      false,
		ThresholdPercent: 0.0,
		CostWeight:       0.0,
		BalanceWeight:    1.0,
	}
}

// UpdateUsage updates the usage information for a specific key
func (t *Tracker) UpdateUsage(key string, usage *types.TavilyUsage) error {
	// Store in Redis cache
	ctx, cancel := context.WithTimeout(t.ctx, 2*time.Second)
	defer cancel()
	
	if err := t.usageCache.SetUsage(ctx, key, usage); err != nil {
		t.logger.WithError(err).Warn("Failed to cache usage in Redis, storing in memory")
		t.memoryCache.Store(key, usage) // Fallback to memory
	} else {
		// Also store in memory for fast access
		t.memoryCache.Store(key, usage)
	}

	// Update analytics
	analytics := t.getOrCreateKeyAnalytics(key)
	analytics.Usage = usage
	analytics.LastUpdated = time.Now()
	analytics.RemainingPoints, _ = t.CalculateRemainingPoints(key)
	analytics.HealthScore = t.calculateHealthScore(analytics)
	analytics.CostEfficiency = t.calculateCostEfficiency(analytics)

	// Cache analytics
	ctx2, cancel2 := context.WithTimeout(t.ctx, 1*time.Second)
	defer cancel2()
	if err := t.usageCache.SetKeyAnalytics(ctx2, key, analytics); err != nil {
		t.logger.WithError(err).Debug("Failed to cache analytics")
	}

	t.analytics.Store(key, analytics)
	t.lastUpdate = time.Now()

	t.logger.WithFields(logrus.Fields{
		"key":             key[:12] + "...",
		"key_usage":       usage.Key.Usage,
		"key_limit":       usage.Key.Limit,
		"plan_usage":      usage.Account.PlanUsage,
		"plan_limit":      usage.Account.PlanLimit,
		"paygo_usage":     usage.Account.PaygoUsage,
		"paygo_limit":     usage.Account.PaygoLimit,
		"health_score":    analytics.HealthScore,
		"cost_efficiency": analytics.CostEfficiency,
	}).Debug("Updated usage information")

	return nil
}

// GetUsage retrieves usage information for a specific key
func (t *Tracker) GetUsage(key string) (*types.TavilyUsage, error) {
	// Try Redis cache first
	ctx, cancel := context.WithTimeout(t.ctx, 1*time.Second)
	defer cancel()
	
	if usage, err := t.usageCache.GetUsage(ctx, key); err == nil {
		return usage, nil
	}

	// Fallback to memory cache
	if usageInterface, ok := t.memoryCache.Load(key); ok {
		return usageInterface.(*types.TavilyUsage), nil
	}
	
	return nil, fmt.Errorf("usage information not found for key")
}

// GetAllUsage returns usage information for all keys
func (t *Tracker) GetAllUsage() map[string]*types.TavilyUsage {
	result := make(map[string]*types.TavilyUsage)

	// Get from memory cache (which should have the most recent data)
	t.memoryCache.Range(func(key, value interface{}) bool {
		result[key.(string)] = value.(*types.TavilyUsage)
		return true
	})

	return result
}

// FetchUsageFromAPI fetches usage information from Tavily API
func (t *Tracker) FetchUsageFromAPI(key string) (*types.TavilyUsage, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", t.config.TavilyBaseURL+"/usage", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("User-Agent", "tavily-load/1.0")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch usage: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.ParseHTTPError(resp.StatusCode, nil, key)
	}

	var usage types.TavilyUsage
	if err := json.NewDecoder(resp.Body).Decode(&usage); err != nil {
		return nil, fmt.Errorf("failed to decode usage response: %w", err)
	}

	return &usage, nil
}

// CalculateRemainingPoints calculates remaining points for a key
func (t *Tracker) CalculateRemainingPoints(key string) (*types.RemainingPoints, error) {
	usage, err := t.GetUsage(key)
	if err != nil {
		return nil, err
	}

	keyRemaining := usage.Key.Limit - usage.Key.Usage
	planRemaining := usage.Account.PlanLimit - usage.Account.PlanUsage
	paygoRemaining := usage.Account.PaygoLimit - usage.Account.PaygoUsage
	totalRemaining := keyRemaining + planRemaining + paygoRemaining

	var keyUtil, planUtil, paygoUtil float64
	if usage.Key.Limit > 0 {
		keyUtil = float64(usage.Key.Usage) / float64(usage.Key.Limit)
	}
	if usage.Account.PlanLimit > 0 {
		planUtil = float64(usage.Account.PlanUsage) / float64(usage.Account.PlanLimit)
	}
	if usage.Account.PaygoLimit > 0 {
		paygoUtil = float64(usage.Account.PaygoUsage) / float64(usage.Account.PaygoLimit)
	}

	return &types.RemainingPoints{
		KeyRemaining:     keyRemaining,
		PlanRemaining:    planRemaining,
		PaygoRemaining:   paygoRemaining,
		TotalRemaining:   totalRemaining,
		KeyUtilization:   keyUtil,
		PlanUtilization:  planUtil,
		PaygoUtilization: paygoUtil,
	}, nil
}

// GetOptimalKey selects the optimal key based on the given strategy
func (t *Tracker) GetOptimalKey(strategy types.SelectionStrategy) (string, error) {
	allUsage := t.GetAllUsage()
	if len(allUsage) == 0 {
		return "", fmt.Errorf("no usage information available")
	}

	switch strategy {
	case types.StrategyPlanFirst:
		return t.selectPlanFirstKey(allUsage)
	default:
		// Default to round-robin (handled by key manager)
		return "", fmt.Errorf("strategy not implemented in usage tracker")
	}
}

// Helper methods for different selection strategies

func (t *Tracker) selectPlanFirstKey(allUsage map[string]*types.TavilyUsage) (string, error) {
	// First pass: Look for keys with plan credits available
	var bestPlanKey string
	var mostPlanRemaining int = -1

	for key := range allUsage {
		remaining, err := t.CalculateRemainingPoints(key)
		if err != nil || remaining.TotalRemaining <= 0 {
			continue
		}

		// Prioritize keys with plan credits
		if remaining.PlanRemaining > mostPlanRemaining {
			mostPlanRemaining = remaining.PlanRemaining
			bestPlanKey = key
		}
	}

	// If we found a key with plan credits, use it
	if bestPlanKey != "" && mostPlanRemaining > 0 {
		return bestPlanKey, nil
	}

	// Second pass: No plan credits available, find key with most paygo credits
	var bestPaygoKey string
	var mostPaygoRemaining int = -1

	for key := range allUsage {
		remaining, err := t.CalculateRemainingPoints(key)
		if err != nil || remaining.TotalRemaining <= 0 {
			continue
		}

		if remaining.PaygoRemaining > mostPaygoRemaining {
			mostPaygoRemaining = remaining.PaygoRemaining
			bestPaygoKey = key
		}
	}

	if bestPaygoKey != "" {
		return bestPaygoKey, nil
	}

	return "", fmt.Errorf("no available keys with remaining quota")
}

// Helper methods for analytics

func (t *Tracker) getOrCreateKeyAnalytics(key string) *types.KeyAnalytics {
	if analyticsInterface, ok := t.analytics.Load(key); ok {
		return analyticsInterface.(*types.KeyAnalytics)
	}

	analytics := &types.KeyAnalytics{
		Key:            key,
		RequestCount:   0,
		ErrorCount:     0,
		LastUsed:       time.Time{},
		LastUpdated:    time.Now(),
		HealthScore:    1.0,
		CostEfficiency: 0.5,
		RecommendedUse: true,
	}

	t.analytics.Store(key, analytics)
	return analytics
}

func (t *Tracker) calculateHealthScore(analytics *types.KeyAnalytics) float64 {
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

func (t *Tracker) calculateCostEfficiency(analytics *types.KeyAnalytics) float64 {
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

// UpdateKeyMetrics updates metrics for a key after a request
func (t *Tracker) UpdateKeyMetrics(key string, success bool, latency time.Duration) {
	// Update in Redis cache
	ctx, cancel := context.WithTimeout(t.ctx, 1*time.Second)
	defer cancel()
	
	go func() {
		if err := t.usageCache.IncrementKeyUsage(ctx, key, success); err != nil {
			t.logger.WithError(err).Debug("Failed to update key metrics in cache")
		}
	}()

	// Update analytics in memory
	analytics := t.getOrCreateKeyAnalytics(key)
	analytics.RequestCount++
	analytics.LastUsed = time.Now()

	if !success {
		analytics.ErrorCount++
	}

	// Recalculate scores
	analytics.HealthScore = t.calculateHealthScore(analytics)
	analytics.CostEfficiency = t.calculateCostEfficiency(analytics)
	analytics.RecommendedUse = analytics.HealthScore > 0.5 && analytics.RemainingPoints != nil && analytics.RemainingPoints.TotalRemaining > 0

	t.analytics.Store(key, analytics)
	
	// Cache updated analytics
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := t.usageCache.SetKeyAnalytics(ctx, key, analytics); err != nil {
			t.logger.WithError(err).Debug("Failed to cache updated analytics")
		}
	}()
}

// GetRecommendedStrategy returns the recommended strategy based on current usage patterns
func (t *Tracker) GetRecommendedStrategy() types.SelectionStrategy {
	allUsage := t.GetAllUsage()
	if len(allUsage) == 0 {
		return types.StrategyRoundRobin
	}

	var totalPlanRemaining int

	for key := range allUsage {
		remaining, err := t.CalculateRemainingPoints(key)
		if err != nil {
			continue
		}

		totalPlanRemaining += remaining.PlanRemaining
	}

	// Decision logic for strategy recommendation
	// Always prefer plan_first as it's the default and most cost-effective
	if totalPlanRemaining > 0 {
		return types.StrategyPlanFirst
	}

	// Fallback to round-robin when no plan credits available
	return types.StrategyRoundRobin
}
