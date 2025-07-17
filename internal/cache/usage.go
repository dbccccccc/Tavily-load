package cache

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/dbccccccc/tavily-load/pkg/types"
)

const (
	KeyUsageCachePrefix     = "usage:"
	KeyAnalyticsCachePrefix = "analytics:"
	KeyStatsCachePrefix     = "stats:"
	BlacklistCachePrefix    = "blacklist:"

	DefaultUsageTTL     = 5 * time.Minute
	DefaultAnalyticsTTL = 10 * time.Minute
	DefaultStatsTTL     = 2 * time.Minute
	DefaultBlacklistTTL = 1 * time.Hour
)

type UsageCache struct {
	client *RedisClient
}

func NewUsageCache(client *RedisClient) *UsageCache {
	return &UsageCache{client: client}
}

func (c *UsageCache) SetUsage(ctx context.Context, key string, usage *types.TavilyUsage) error {
	cacheKey := KeyUsageCachePrefix + key
	return c.client.SetJSON(ctx, cacheKey, usage, DefaultUsageTTL)
}

func (c *UsageCache) GetUsage(ctx context.Context, key string) (*types.TavilyUsage, error) {
	cacheKey := KeyUsageCachePrefix + key
	var usage types.TavilyUsage
	err := c.client.GetJSON(ctx, cacheKey, &usage)
	if err != nil {
		return nil, err
	}
	return &usage, nil
}

func (c *UsageCache) DeleteUsage(ctx context.Context, key string) error {
	cacheKey := KeyUsageCachePrefix + key
	return c.client.Del(ctx, cacheKey).Err()
}

func (c *UsageCache) SetKeyAnalytics(ctx context.Context, key string, analytics *types.KeyAnalytics) error {
	cacheKey := KeyAnalyticsCachePrefix + key
	return c.client.SetJSON(ctx, cacheKey, analytics, DefaultAnalyticsTTL)
}

func (c *UsageCache) GetKeyAnalytics(ctx context.Context, key string) (*types.KeyAnalytics, error) {
	cacheKey := KeyAnalyticsCachePrefix + key
	var analytics types.KeyAnalytics
	err := c.client.GetJSON(ctx, cacheKey, &analytics)
	if err != nil {
		return nil, err
	}
	return &analytics, nil
}

func (c *UsageCache) SetKeyStats(ctx context.Context, key string, stats *types.KeyStatus) error {
	cacheKey := KeyStatsCachePrefix + key
	return c.client.SetJSON(ctx, cacheKey, stats, DefaultStatsTTL)
}

func (c *UsageCache) GetKeyStats(ctx context.Context, key string) (*types.KeyStatus, error) {
	cacheKey := KeyStatsCachePrefix + key
	var stats types.KeyStatus
	err := c.client.GetJSON(ctx, cacheKey, &stats)
	if err != nil {
		return nil, err
	}
	return &stats, nil
}

func (c *UsageCache) SetBlacklistStatus(ctx context.Context, key string, isBlacklisted bool, reason string, until *time.Time) error {
	cacheKey := BlacklistCachePrefix + key
	blacklistInfo := map[string]interface{}{
		"is_blacklisted": isBlacklisted,
		"reason":         reason,
		"until":          until,
		"cached_at":      time.Now(),
	}
	return c.client.SetJSON(ctx, cacheKey, blacklistInfo, DefaultBlacklistTTL)
}

func (c *UsageCache) GetBlacklistStatus(ctx context.Context, key string) (bool, string, *time.Time, error) {
	cacheKey := BlacklistCachePrefix + key
	var blacklistInfo map[string]interface{}
	err := c.client.GetJSON(ctx, cacheKey, &blacklistInfo)
	if err != nil {
		return false, "", nil, err
	}

	isBlacklisted, ok := blacklistInfo["is_blacklisted"].(bool)
	if !ok {
		return false, "", nil, fmt.Errorf("invalid blacklist status format")
	}

	reason, _ := blacklistInfo["reason"].(string)

	var until *time.Time
	if untilStr, ok := blacklistInfo["until"].(string); ok && untilStr != "" {
		if parsedTime, err := time.Parse(time.RFC3339, untilStr); err == nil {
			until = &parsedTime
		}
	}

	return isBlacklisted, reason, until, nil
}

func (c *UsageCache) DeleteBlacklistStatus(ctx context.Context, key string) error {
	cacheKey := BlacklistCachePrefix + key
	return c.client.Del(ctx, cacheKey).Err()
}

func (c *UsageCache) InvalidateKeyCache(ctx context.Context, key string) error {
	patterns := []string{
		KeyUsageCachePrefix + key,
		KeyAnalyticsCachePrefix + key,
		KeyStatsCachePrefix + key,
		BlacklistCachePrefix + key,
	}

	for _, pattern := range patterns {
		if err := c.client.Del(ctx, pattern).Err(); err != nil {
			return err
		}
	}

	return nil
}

func (c *UsageCache) InvalidateAllUsage(ctx context.Context) error {
	return c.client.DeletePattern(ctx, KeyUsageCachePrefix+"*")
}

func (c *UsageCache) InvalidateAllAnalytics(ctx context.Context) error {
	return c.client.DeletePattern(ctx, KeyAnalyticsCachePrefix+"*")
}

func (c *UsageCache) SetUsageAnalytics(ctx context.Context, analytics *types.UsageAnalytics) error {
	return c.client.SetJSON(ctx, "usage_analytics", analytics, DefaultAnalyticsTTL)
}

func (c *UsageCache) GetUsageAnalytics(ctx context.Context) (*types.UsageAnalytics, error) {
	var analytics types.UsageAnalytics
	err := c.client.GetJSON(ctx, "usage_analytics", &analytics)
	if err != nil {
		return nil, err
	}
	return &analytics, nil
}

func (c *UsageCache) SetStrategyMetrics(ctx context.Context, strategy types.SelectionStrategy, metrics *types.StrategyMetrics) error {
	cacheKey := fmt.Sprintf("strategy_metrics:%s", strategy)
	return c.client.SetJSON(ctx, cacheKey, metrics, DefaultAnalyticsTTL)
}

func (c *UsageCache) GetStrategyMetrics(ctx context.Context, strategy types.SelectionStrategy) (*types.StrategyMetrics, error) {
	cacheKey := fmt.Sprintf("strategy_metrics:%s", strategy)
	var metrics types.StrategyMetrics
	err := c.client.GetJSON(ctx, cacheKey, &metrics)
	if err != nil {
		return nil, err
	}
	return &metrics, nil
}

func (c *UsageCache) IncrementKeyUsage(ctx context.Context, key string, success bool) error {
	pipe := c.client.Pipeline()

	requestKey := fmt.Sprintf("counter:requests:%s", key)
	pipe.Incr(ctx, requestKey)
	pipe.Expire(ctx, requestKey, 24*time.Hour)

	if !success {
		errorKey := fmt.Sprintf("counter:errors:%s", key)
		pipe.Incr(ctx, errorKey)
		pipe.Expire(ctx, errorKey, 24*time.Hour)
	}

	lastUsedKey := fmt.Sprintf("last_used:%s", key)
	pipe.Set(ctx, lastUsedKey, time.Now().Unix(), 24*time.Hour)

	_, err := pipe.Exec(ctx)
	return err
}

func (c *UsageCache) GetKeyCounters(ctx context.Context, key string) (int64, int64, *time.Time, error) {
	pipe := c.client.Pipeline()

	requestKey := fmt.Sprintf("counter:requests:%s", key)
	errorKey := fmt.Sprintf("counter:errors:%s", key)
	lastUsedKey := fmt.Sprintf("last_used:%s", key)

	requestCmd := pipe.Get(ctx, requestKey)
	errorCmd := pipe.Get(ctx, errorKey)
	lastUsedCmd := pipe.Get(ctx, lastUsedKey)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return 0, 0, nil, err
	}

	var requests, errors int64
	var lastUsed *time.Time

	if requestCmd.Val() != "" {
		if val, err := strconv.ParseInt(requestCmd.Val(), 10, 64); err == nil {
			requests = val
		}
	}

	if errorCmd.Val() != "" {
		if val, err := strconv.ParseInt(errorCmd.Val(), 10, 64); err == nil {
			errors = val
		}
	}

	if lastUsedCmd.Val() != "" {
		if timestamp, err := strconv.ParseInt(lastUsedCmd.Val(), 10, 64); err == nil && timestamp > 0 {
			t := time.Unix(timestamp, 0)
			lastUsed = &t
		}
	}

	return requests, errors, lastUsed, nil
}
