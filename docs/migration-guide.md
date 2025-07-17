# Migration Guide: Implementing Backend Improvements

## Overview

This guide provides a step-by-step approach to implementing the proposed backend improvements while maintaining 100% backward compatibility and zero downtime.

## Phase 1: Foundation Setup (Week 1-2)

### Step 1: Create New Directory Structure

```bash
# Create new domain structure
mkdir -p internal/domain/{key,proxy,monitoring}/{entity,service,repository,strategy}
mkdir -p internal/infrastructure/{database,cache,http,config}
mkdir -p internal/application/{services,dto}
mkdir -p internal/testing/{mocks,fixtures,testcontainers}
mkdir -p docs/api
```

### Step 2: Implement Domain Entities

Start by creating the new domain entities while keeping existing code functional:

```go
// internal/domain/key/entity/key.go
// Implement the Key entity as shown in ddd-implementation-example.md
```

### Step 3: Create Repository Interfaces

```go
// internal/domain/key/repository/interface.go
package repository

import (
    "context"
    "github.com/dbccccccc/tavily-load/internal/domain/key/entity"
)

type KeyRepository interface {
    GetByID(ctx context.Context, id entity.KeyID) (*entity.Key, error)
    GetActiveKeys(ctx context.Context) ([]*entity.Key, error)
    Create(ctx context.Context, key *entity.Key) error
    Update(ctx context.Context, key *entity.Key) error
    Delete(ctx context.Context, id entity.KeyID) error
}
```

### Step 4: Implement Adapter Pattern

Create adapters to bridge existing code with new interfaces:

```go
// internal/infrastructure/database/key_repository_adapter.go
package database

import (
    "context"
    "github.com/dbccccccc/tavily-load/internal/domain/key/entity"
    "github.com/dbccccccc/tavily-load/internal/domain/key/repository"
    oldRepo "github.com/dbccccccc/tavily-load/internal/repository"
)

type KeyRepositoryAdapter struct {
    oldRepo *oldRepo.KeyRepository
}

func NewKeyRepositoryAdapter(oldRepo *oldRepo.KeyRepository) repository.KeyRepository {
    return &KeyRepositoryAdapter{oldRepo: oldRepo}
}

func (a *KeyRepositoryAdapter) GetByID(ctx context.Context, id entity.KeyID) (*entity.Key, error) {
    // Convert between old and new types
    oldKey, err := a.oldRepo.GetKeyByID(ctx, int64(id))
    if err != nil {
        return nil, err
    }
    
    return convertOldKeyToEntity(oldKey), nil
}

// Implement other methods...
```

## Phase 2: Gradual Migration (Week 3-4)

### Step 5: Implement Dependency Injection

```bash
# Add dependency injection library
go get go.uber.org/dig
```

Create the IoC container as shown in `dependency-injection-example.md`, but initially wire both old and new implementations:

```go
// internal/infrastructure/container/migration_container.go
func (c *Container) RegisterMigrationProviders() error {
    providers := []interface{}{
        // Old implementations (for backward compatibility)
        NewOldKeyManager,
        NewOldHandler,
        
        // New implementations (gradually replacing old ones)
        NewKeyRepository,
        NewKeyManager,
        
        // Adapters
        NewKeyRepositoryAdapter,
    }
    
    for _, provider := range providers {
        if err := c.container.Provide(provider); err != nil {
            return err
        }
    }
    
    return nil
}
```

### Step 6: Feature Flag System

Implement feature flags to gradually enable new functionality:

```go
// internal/infrastructure/config/feature_flags.go
package config

type FeatureFlags struct {
    UseNewKeyManager     bool `env:"USE_NEW_KEY_MANAGER" default:"false"`
    UseNewErrorHandling  bool `env:"USE_NEW_ERROR_HANDLING" default:"false"`
    UseNewLogging        bool `env:"USE_NEW_LOGGING" default:"false"`
    UseNewMetrics        bool `env:"USE_NEW_METRICS" default:"false"`
}

// In your service
func (s *Service) selectKeyManager() KeyManager {
    if s.featureFlags.UseNewKeyManager {
        return s.newKeyManager
    }
    return s.oldKeyManager
}
```

### Step 7: Implement New Error Handling

Add the new error handling system alongside the existing one:

```go
// internal/infrastructure/errors/migration_handler.go
package errors

func HandleError(err error, useNewSystem bool) error {
    if useNewSystem {
        return handleWithNewSystem(err)
    }
    return handleWithOldSystem(err)
}
```

## Phase 3: Testing and Validation (Week 5-6)

### Step 8: Comprehensive Testing

Implement the testing framework as shown in `testing-framework-example.md`:

```bash
# Add testing dependencies
go get github.com/stretchr/testify
go get github.com/testcontainers/testcontainers-go
```

### Step 9: A/B Testing in Production

Implement A/B testing to validate new implementations:

```go
// internal/infrastructure/testing/ab_test.go
package testing

type ABTest struct {
    name           string
    percentage     float64
    enabledForUser func(userID string) bool
}

func (ab *ABTest) IsEnabled(userID string) bool {
    if ab.enabledForUser != nil {
        return ab.enabledForUser(userID)
    }
    
    // Simple percentage-based rollout
    hash := hashString(userID + ab.name)
    return (hash % 100) < int(ab.percentage*100)
}
```

### Step 10: Monitoring and Observability

Implement comprehensive monitoring:

```go
// internal/infrastructure/monitoring/migration_monitor.go
package monitoring

type MigrationMonitor struct {
    oldMetrics *OldMetrics
    newMetrics *NewMetrics
}

func (m *MigrationMonitor) RecordKeySelection(strategy string, useNew bool) {
    if useNew {
        m.newMetrics.RecordKeySelection(strategy, "success")
    } else {
        m.oldMetrics.RecordKeySelection(strategy)
    }
}
```

## Migration Checklist

### Pre-Migration
- [ ] Backup database and configuration
- [ ] Set up monitoring and alerting
- [ ] Prepare rollback procedures
- [ ] Test in staging environment
- [ ] Document current API contracts

### Phase 1 Checklist
- [ ] New directory structure created
- [ ] Domain entities implemented
- [ ] Repository interfaces defined
- [ ] Adapter pattern implemented
- [ ] Basic dependency injection setup
- [ ] All existing tests pass

### Phase 2 Checklist
- [ ] Feature flags implemented
- [ ] New error handling system
- [ ] Enhanced logging system
- [ ] Metrics collection improved
- [ ] A/B testing framework
- [ ] Performance benchmarks established

### Phase 3 Checklist
- [ ] Comprehensive test suite
- [ ] Integration tests with test containers
- [ ] Load testing completed
- [ ] Documentation updated
- [ ] API documentation generated
- [ ] Migration monitoring in place

## Rollback Procedures

### Immediate Rollback (< 5 minutes)
```bash
# Disable new features via environment variables
export USE_NEW_KEY_MANAGER=false
export USE_NEW_ERROR_HANDLING=false
export USE_NEW_LOGGING=false

# Restart application
systemctl restart tavily-load
```

### Database Rollback
```sql
-- If database changes were made, rollback migrations
migrate -path migrations -database "mysql://..." down 1
```

### Configuration Rollback
```bash
# Restore previous configuration
cp config/backup/.env.backup .env
systemctl restart tavily-load
```

## Validation Steps

### Functional Validation
1. **API Contract Testing**: Ensure all endpoints return expected responses
2. **Key Management**: Verify key rotation and blacklisting work correctly
3. **Proxy Functionality**: Test all Tavily API endpoints
4. **Error Handling**: Verify error responses match existing format

### Performance Validation
1. **Response Times**: Compare before/after latency metrics
2. **Throughput**: Ensure request handling capacity maintained
3. **Memory Usage**: Monitor memory consumption patterns
4. **Database Performance**: Check query performance impact

### Monitoring Validation
1. **Metrics Collection**: Verify all metrics are being collected
2. **Alerting**: Test alert conditions and notifications
3. **Logging**: Ensure log format and content are appropriate
4. **Health Checks**: Validate health endpoint responses

## Success Criteria

- ✅ Zero API breaking changes
- ✅ Performance maintained or improved (< 5% degradation acceptable)
- ✅ All existing functionality preserved
- ✅ New features can be enabled/disabled via configuration
- ✅ Comprehensive test coverage (> 80%)
- ✅ Documentation updated and accurate
- ✅ Monitoring and alerting functional
- ✅ Rollback procedures tested and documented

## Post-Migration Tasks

1. **Cleanup**: Remove old code after new implementation is stable (30+ days)
2. **Documentation**: Update architecture documentation
3. **Training**: Train team on new codebase structure
4. **Optimization**: Fine-tune performance based on production metrics
5. **Security Review**: Conduct security audit of new implementation

This migration approach ensures:
- **Zero Downtime**: Services remain available throughout migration
- **Risk Mitigation**: Feature flags allow quick rollback
- **Gradual Adoption**: New features can be enabled incrementally
- **Validation**: Comprehensive testing at each phase
- **Monitoring**: Full observability during migration process
