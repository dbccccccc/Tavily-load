# Dependency Injection Implementation Example

## IoC Container Setup

```go
// internal/infrastructure/container/container.go
package container

import (
    "context"
    "database/sql"
    
    "go.uber.org/dig"
    "github.com/sirupsen/logrus"
    
    "github.com/dbccccccc/tavily-load/internal/infrastructure/config"
    "github.com/dbccccccc/tavily-load/internal/infrastructure/database"
    "github.com/dbccccccc/tavily-load/internal/infrastructure/cache"
    "github.com/dbccccccc/tavily-load/internal/domain/key/service"
    "github.com/dbccccccc/tavily-load/internal/domain/key/repository"
    "github.com/dbccccccc/tavily-load/internal/domain/key/strategy"
    "github.com/dbccccccc/tavily-load/internal/application/services"
)

// Container wraps the dig container
type Container struct {
    container *dig.Container
}

// New creates a new IoC container
func New() *Container {
    return &Container{
        container: dig.New(),
    }
}

// RegisterProviders registers all service providers
func (c *Container) RegisterProviders() error {
    providers := []interface{}{
        // Infrastructure providers
        NewLogger,
        NewConfig,
        NewDatabase,
        NewRedisClient,
        
        // Repository providers
        NewKeyRepository,
        NewUsageCache,
        
        // Domain service providers
        NewKeyManager,
        NewUsageTracker,
        
        // Strategy providers
        NewPlanFirstStrategy,
        NewRoundRobinStrategy,
        NewWeightedStrategy,
        
        // Application service providers
        NewKeyManagementService,
        NewProxyService,
        NewMonitoringService,
        
        // HTTP providers
        NewHTTPServer,
        NewHandlers,
    }
    
    for _, provider := range providers {
        if err := c.container.Provide(provider); err != nil {
            return err
        }
    }
    
    return nil
}

// Invoke executes a function with dependency injection
func (c *Container) Invoke(fn interface{}) error {
    return c.container.Invoke(fn)
}

// Infrastructure Providers

func NewLogger(cfg *config.Config) *logrus.Logger {
    logger := logrus.New()
    
    if cfg.LogLevel != "" {
        level, err := logrus.ParseLevel(cfg.LogLevel)
        if err == nil {
            logger.SetLevel(level)
        }
    }
    
    if cfg.LogFormat == "json" {
        logger.SetFormatter(&logrus.JSONFormatter{})
    }
    
    return logger
}

func NewConfig() (*config.Config, error) {
    return config.Load()
}

func NewDatabase(cfg *config.Config, logger *logrus.Logger) (*sql.DB, error) {
    dbConfig := &database.Config{
        Host:            cfg.DBHost,
        Port:            cfg.DBPort,
        Username:        cfg.DBUsername,
        Password:        cfg.DBPassword,
        Database:        cfg.DBName,
        MaxOpenConns:    cfg.DBMaxOpenConns,
        MaxIdleConns:    cfg.DBMaxIdleConns,
        ConnMaxLifetime: cfg.DBConnMaxLifetime,
    }
    
    db, err := database.NewConnection(dbConfig)
    if err != nil {
        return nil, err
    }
    
    logger.Info("Database connection established")
    return db.DB, nil
}

func NewRedisClient(cfg *config.Config, logger *logrus.Logger) (*cache.RedisClient, error) {
    redisConfig := &cache.Config{
        Host:     cfg.RedisHost,
        Port:     cfg.RedisPort,
        Password: cfg.RedisPassword,
        DB:       cfg.RedisDB,
        PoolSize: cfg.RedisPoolSize,
    }
    
    client, err := cache.NewRedisClient(redisConfig)
    if err != nil {
        return nil, err
    }
    
    logger.Info("Redis connection established")
    return client, nil
}

// Repository Providers

func NewKeyRepository(db *sql.DB) repository.KeyRepository {
    return repository.NewMySQLKeyRepository(db)
}

func NewUsageCache(client *cache.RedisClient) *cache.UsageCache {
    return cache.NewUsageCache(client)
}

// Domain Service Providers

func NewKeyManager(
    repo repository.KeyRepository,
    cache *cache.UsageCache,
    logger *logrus.Logger,
    cfg *config.Config,
) *service.Manager {
    manager := service.NewManager(repo, cache, logger)
    
    // Register strategies
    manager.RegisterStrategy(strategy.NewPlanFirstStrategy(cache))
    manager.RegisterStrategy(strategy.NewRoundRobinStrategy())
    manager.RegisterStrategy(strategy.NewWeightedStrategy(cache))
    
    return manager
}

func NewUsageTracker(
    cache *cache.UsageCache,
    logger *logrus.Logger,
    cfg *config.Config,
) *service.UsageTracker {
    return service.NewUsageTracker(cache, logger, cfg)
}

// Strategy Providers

func NewPlanFirstStrategy(cache *cache.UsageCache) strategy.SelectionStrategy {
    return strategy.NewPlanFirstStrategy(cache)
}

func NewRoundRobinStrategy() strategy.SelectionStrategy {
    return strategy.NewRoundRobinStrategy()
}

func NewWeightedStrategy(cache *cache.UsageCache) strategy.SelectionStrategy {
    return strategy.NewWeightedStrategy(cache)
}

// Application Service Providers

func NewKeyManagementService(
    keyManager *service.Manager,
    usageTracker *service.UsageTracker,
    logger *logrus.Logger,
) *services.KeyManagementService {
    return services.NewKeyManagementService(keyManager, usageTracker, logger)
}

func NewProxyService(
    keyManager *service.Manager,
    logger *logrus.Logger,
    cfg *config.Config,
) *services.ProxyService {
    return services.NewProxyService(keyManager, logger, cfg)
}

func NewMonitoringService(
    keyManager *service.Manager,
    usageTracker *service.UsageTracker,
    logger *logrus.Logger,
) *services.MonitoringService {
    return services.NewMonitoringService(keyManager, usageTracker, logger)
}

// HTTP Providers

func NewHTTPServer(
    handlers *handlers.Handlers,
    cfg *config.Config,
    logger *logrus.Logger,
) *http.Server {
    return http.NewServer(handlers, cfg, logger)
}

func NewHandlers(
    keyService *services.KeyManagementService,
    proxyService *services.ProxyService,
    monitoringService *services.MonitoringService,
    logger *logrus.Logger,
) *handlers.Handlers {
    return handlers.New(keyService, proxyService, monitoringService, logger)
}
```

## Updated Main Function

```go
// cmd/tavily-load/main.go
package main

import (
    "context"
    "os"
    "os/signal"
    "syscall"
    
    "github.com/dbccccccc/tavily-load/internal/infrastructure/container"
    "github.com/dbccccccc/tavily-load/internal/infrastructure/http"
)

func main() {
    // Create IoC container
    c := container.New()
    
    // Register all providers
    if err := c.RegisterProviders(); err != nil {
        panic("Failed to register providers: " + err.Error())
    }
    
    // Start the application
    err := c.Invoke(func(server *http.Server) {
        // Setup graceful shutdown
        ctx, cancel := context.WithCancel(context.Background())
        defer cancel()
        
        // Start server in goroutine
        serverErr := make(chan error, 1)
        go func() {
            serverErr <- server.Start()
        }()
        
        // Wait for interrupt signal
        sigChan := make(chan os.Signal, 1)
        signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
        
        select {
        case err := <-serverErr:
            if err != nil {
                panic("Server failed to start: " + err.Error())
            }
        case sig := <-sigChan:
            server.Logger().WithField("signal", sig).Info("Received shutdown signal")
            cancel()
            
            // Graceful shutdown
            if err := server.Stop(ctx); err != nil {
                server.Logger().WithError(err).Error("Failed to shutdown server gracefully")
                os.Exit(1)
            }
        }
        
        server.Logger().Info("Application stopped")
    })
    
    if err != nil {
        panic("Failed to start application: " + err.Error())
    }
}
```

## Service Interface Example

```go
// internal/application/services/key_management.go
package services

import (
    "context"
    
    "github.com/dbccccccc/tavily-load/internal/domain/key/entity"
    "github.com/dbccccccc/tavily-load/internal/domain/key/service"
    "github.com/sirupsen/logrus"
)

// KeyManagementService provides key management use cases
type KeyManagementService struct {
    keyManager   *service.Manager
    usageTracker *service.UsageTracker
    logger       *logrus.Logger
}

func NewKeyManagementService(
    keyManager *service.Manager,
    usageTracker *service.UsageTracker,
    logger *logrus.Logger,
) *KeyManagementService {
    return &KeyManagementService{
        keyManager:   keyManager,
        usageTracker: usageTracker,
        logger:       logger,
    }
}

func (s *KeyManagementService) GetNextKey(ctx context.Context, strategy string) (*entity.Key, error) {
    key, err := s.keyManager.SelectKey(ctx, strategy)
    if err != nil {
        s.logger.WithError(err).Error("Failed to select key")
        return nil, err
    }
    
    // Update usage tracking
    if err := s.usageTracker.RecordKeyUsage(ctx, key.ID()); err != nil {
        s.logger.WithError(err).Warn("Failed to record key usage")
    }
    
    return key, nil
}

func (s *KeyManagementService) BlacklistKey(ctx context.Context, keyID entity.KeyID, reason string) error {
    if err := s.keyManager.BlacklistKey(ctx, keyID, reason, false); err != nil {
        s.logger.WithError(err).Error("Failed to blacklist key")
        return err
    }
    
    s.logger.WithField("key_id", keyID).Info("Key blacklisted")
    return nil
}
```

## Benefits of This Approach

1. **Reduced Coupling**: Components depend on interfaces, not concrete implementations
2. **Easier Testing**: Mock dependencies can be easily injected
3. **Cleaner Main**: Application startup logic is simplified
4. **Lifecycle Management**: Container manages object creation and cleanup
5. **Configuration**: Centralized dependency configuration
6. **Flexibility**: Easy to swap implementations for different environments
