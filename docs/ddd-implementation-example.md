# Domain-Driven Design Implementation Example

## Key Domain Entity Example

```go
// internal/domain/key/entity/key.go
package entity

import (
    "time"
    "errors"
)

// Key represents an API key entity with business logic
type Key struct {
    id               KeyID
    value            string
    name             string
    description      string
    isActive         bool
    isBlacklisted    bool
    blacklistedUntil *time.Time
    blacklistReason  string
    createdAt        time.Time
    updatedAt        time.Time
    
    // Business logic state
    requestCount     int64
    errorCount       int64
    lastUsed         time.Time
    healthScore      float64
}

// KeyID is a value object for key identification
type KeyID struct {
    value int64
}

// NewKey creates a new key entity
func NewKey(value, name, description string) (*Key, error) {
    if value == "" {
        return nil, errors.New("key value cannot be empty")
    }
    
    return &Key{
        value:       value,
        name:        name,
        description: description,
        isActive:    true,
        createdAt:   time.Now(),
        updatedAt:   time.Now(),
    }, nil
}

// Business methods
func (k *Key) Blacklist(reason string, permanent bool) error {
    if k.isBlacklisted {
        return errors.New("key is already blacklisted")
    }
    
    k.isBlacklisted = true
    k.blacklistReason = reason
    k.updatedAt = time.Now()
    
    if !permanent {
        until := time.Now().Add(5 * time.Minute)
        k.blacklistedUntil = &until
    }
    
    return nil
}

func (k *Key) Unblacklist() error {
    if !k.isBlacklisted {
        return errors.New("key is not blacklisted")
    }
    
    k.isBlacklisted = false
    k.blacklistedUntil = nil
    k.blacklistReason = ""
    k.updatedAt = time.Now()
    
    return nil
}

func (k *Key) RecordUsage(success bool) {
    k.requestCount++
    k.lastUsed = time.Now()
    
    if !success {
        k.errorCount++
    }
    
    k.calculateHealthScore()
    k.updatedAt = time.Now()
}

func (k *Key) IsAvailable() bool {
    if !k.isActive || k.isBlacklisted {
        return false
    }
    
    // Check if temporary blacklist has expired
    if k.blacklistedUntil != nil && time.Now().After(*k.blacklistedUntil) {
        k.isBlacklisted = false
        k.blacklistedUntil = nil
        k.blacklistReason = ""
    }
    
    return !k.isBlacklisted
}

func (k *Key) calculateHealthScore() {
    if k.requestCount == 0 {
        k.healthScore = 1.0
        return
    }
    
    errorRate := float64(k.errorCount) / float64(k.requestCount)
    k.healthScore = 1.0 - errorRate
    
    if k.healthScore < 0 {
        k.healthScore = 0
    }
}

// Getters
func (k *Key) ID() KeyID { return k.id }
func (k *Key) Value() string { return k.value }
func (k *Key) Name() string { return k.name }
func (k *Key) IsActive() bool { return k.isActive }
func (k *Key) IsBlacklisted() bool { return k.isBlacklisted }
func (k *Key) RequestCount() int64 { return k.requestCount }
func (k *Key) ErrorCount() int64 { return k.errorCount }
func (k *Key) HealthScore() float64 { return k.healthScore }
```

## Key Selection Strategy Interface

```go
// internal/domain/key/strategy/interface.go
package strategy

import (
    "context"
    "github.com/dbccccccc/tavily-load/internal/domain/key/entity"
)

// SelectionStrategy defines the interface for key selection algorithms
type SelectionStrategy interface {
    SelectKey(ctx context.Context, keys []*entity.Key) (*entity.Key, error)
    Name() string
    Priority() int
}

// SelectionContext provides context for key selection
type SelectionContext struct {
    RequestType    string
    ClientIP       string
    UserAgent      string
    PreviousErrors []string
    RetryCount     int
}
```

## Strategy Implementation Example

```go
// internal/domain/key/strategy/plan_first.go
package strategy

import (
    "context"
    "errors"
    "sort"
    
    "github.com/dbccccccc/tavily-load/internal/domain/key/entity"
)

// PlanFirstStrategy implements plan-first key selection
type PlanFirstStrategy struct {
    usageService UsageService
}

type UsageService interface {
    GetUsage(ctx context.Context, key *entity.Key) (*Usage, error)
    GetRemainingQuota(ctx context.Context, key *entity.Key) (int64, error)
}

type Usage struct {
    PlanType        string
    UsedRequests    int64
    PlanLimit       int64
    RemainingQuota  int64
}

func NewPlanFirstStrategy(usageService UsageService) *PlanFirstStrategy {
    return &PlanFirstStrategy{
        usageService: usageService,
    }
}

func (s *PlanFirstStrategy) SelectKey(ctx context.Context, keys []*entity.Key) (*entity.Key, error) {
    availableKeys := make([]*entity.Key, 0)
    
    // Filter available keys
    for _, key := range keys {
        if key.IsAvailable() {
            availableKeys = append(availableKeys, key)
        }
    }
    
    if len(availableKeys) == 0 {
        return nil, errors.New("no available keys")
    }
    
    // Sort by plan priority and remaining quota
    sort.Slice(availableKeys, func(i, j int) bool {
        usageI, _ := s.usageService.GetUsage(ctx, availableKeys[i])
        usageJ, _ := s.usageService.GetUsage(ctx, availableKeys[j])
        
        // Prioritize plan keys over pay-as-you-go
        if s.isPlanKey(usageI) && !s.isPlanKey(usageJ) {
            return true
        }
        if !s.isPlanKey(usageI) && s.isPlanKey(usageJ) {
            return false
        }
        
        // Within same type, prioritize by remaining quota
        return usageI.RemainingQuota > usageJ.RemainingQuota
    })
    
    return availableKeys[0], nil
}

func (s *PlanFirstStrategy) Name() string {
    return "plan-first"
}

func (s *PlanFirstStrategy) Priority() int {
    return 100
}

func (s *PlanFirstStrategy) isPlanKey(usage *Usage) bool {
    return usage != nil && usage.PlanType != "pay-as-you-go"
}
```

## Service Layer Example

```go
// internal/domain/key/service/manager.go
package service

import (
    "context"
    "errors"
    
    "github.com/dbccccccc/tavily-load/internal/domain/key/entity"
    "github.com/dbccccccc/tavily-load/internal/domain/key/repository"
    "github.com/dbccccccc/tavily-load/internal/domain/key/strategy"
)

// Manager provides key management business logic
type Manager struct {
    repository repository.KeyRepository
    strategies map[string]strategy.SelectionStrategy
    cache      CacheService
    logger     Logger
}

type CacheService interface {
    Get(ctx context.Context, key string) (interface{}, error)
    Set(ctx context.Context, key string, value interface{}) error
}

type Logger interface {
    Info(msg string, fields ...interface{})
    Error(msg string, err error, fields ...interface{})
}

func NewManager(
    repo repository.KeyRepository,
    cache CacheService,
    logger Logger,
) *Manager {
    return &Manager{
        repository: repo,
        strategies: make(map[string]strategy.SelectionStrategy),
        cache:      cache,
        logger:     logger,
    }
}

func (m *Manager) RegisterStrategy(strategy strategy.SelectionStrategy) {
    m.strategies[strategy.Name()] = strategy
}

func (m *Manager) SelectKey(ctx context.Context, strategyName string) (*entity.Key, error) {
    strategy, exists := m.strategies[strategyName]
    if !exists {
        return nil, errors.New("strategy not found")
    }
    
    keys, err := m.repository.GetActiveKeys(ctx)
    if err != nil {
        return nil, err
    }
    
    selectedKey, err := strategy.SelectKey(ctx, keys)
    if err != nil {
        return nil, err
    }
    
    // Record usage
    selectedKey.RecordUsage(true)
    
    // Update in repository
    if err := m.repository.Update(ctx, selectedKey); err != nil {
        m.logger.Error("Failed to update key usage", err)
    }
    
    return selectedKey, nil
}

func (m *Manager) BlacklistKey(ctx context.Context, keyID entity.KeyID, reason string, permanent bool) error {
    key, err := m.repository.GetByID(ctx, keyID)
    if err != nil {
        return err
    }
    
    if err := key.Blacklist(reason, permanent); err != nil {
        return err
    }
    
    return m.repository.Update(ctx, key)
}
```

This DDD approach provides:

1. **Clear business logic** encapsulated in entities
2. **Flexible strategy pattern** for key selection
3. **Clean separation** between domain and infrastructure
4. **Testable components** with clear interfaces
5. **Maintainable code** with single responsibility principle
