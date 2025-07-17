# Comprehensive Testing Framework

## Test Utilities and Mocks

```go
// internal/testing/mocks/key_repository.go
package mocks

import (
    "context"
    "github.com/stretchr/testify/mock"
    "github.com/dbccccccc/tavily-load/internal/domain/key/entity"
)

// MockKeyRepository is a mock implementation of KeyRepository
type MockKeyRepository struct {
    mock.Mock
}

func (m *MockKeyRepository) GetByID(ctx context.Context, id entity.KeyID) (*entity.Key, error) {
    args := m.Called(ctx, id)
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0).(*entity.Key), args.Error(1)
}

func (m *MockKeyRepository) GetActiveKeys(ctx context.Context) ([]*entity.Key, error) {
    args := m.Called(ctx)
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0).([]*entity.Key), args.Error(1)
}

func (m *MockKeyRepository) Update(ctx context.Context, key *entity.Key) error {
    args := m.Called(ctx, key)
    return args.Error(0)
}

func (m *MockKeyRepository) Create(ctx context.Context, key *entity.Key) error {
    args := m.Called(ctx, key)
    return args.Error(0)
}

func (m *MockKeyRepository) Delete(ctx context.Context, id entity.KeyID) error {
    args := m.Called(ctx, id)
    return args.Error(0)
}
```

## Unit Test Example

```go
// internal/domain/key/service/manager_test.go
package service_test

import (
    "context"
    "testing"
    "time"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
    "github.com/stretchr/testify/suite"
    
    "github.com/dbccccccc/tavily-load/internal/domain/key/entity"
    "github.com/dbccccccc/tavily-load/internal/domain/key/service"
    "github.com/dbccccccc/tavily-load/internal/testing/mocks"
    "github.com/dbccccccc/tavily-load/internal/testing/fixtures"
)

type ManagerTestSuite struct {
    suite.Suite
    manager    *service.Manager
    mockRepo   *mocks.MockKeyRepository
    mockCache  *mocks.MockCacheService
    mockLogger *mocks.MockLogger
    ctx        context.Context
}

func (suite *ManagerTestSuite) SetupTest() {
    suite.mockRepo = &mocks.MockKeyRepository{}
    suite.mockCache = &mocks.MockCacheService{}
    suite.mockLogger = &mocks.MockLogger{}
    suite.ctx = context.Background()
    
    suite.manager = service.NewManager(
        suite.mockRepo,
        suite.mockCache,
        suite.mockLogger,
    )
}

func (suite *ManagerTestSuite) TestSelectKey_Success() {
    // Arrange
    keys := fixtures.CreateTestKeys(3)
    strategy := "plan-first"
    
    suite.mockRepo.On("GetActiveKeys", suite.ctx).Return(keys, nil)
    
    // Act
    selectedKey, err := suite.manager.SelectKey(suite.ctx, strategy)
    
    // Assert
    assert.NoError(suite.T(), err)
    assert.NotNil(suite.T(), selectedKey)
    assert.True(suite.T(), selectedKey.IsAvailable())
    
    suite.mockRepo.AssertExpectations(suite.T())
}

func (suite *ManagerTestSuite) TestSelectKey_NoKeysAvailable() {
    // Arrange
    keys := []*entity.Key{} // Empty slice
    strategy := "plan-first"
    
    suite.mockRepo.On("GetActiveKeys", suite.ctx).Return(keys, nil)
    
    // Act
    selectedKey, err := suite.manager.SelectKey(suite.ctx, strategy)
    
    // Assert
    assert.Error(suite.T(), err)
    assert.Nil(suite.T(), selectedKey)
    assert.Contains(suite.T(), err.Error(), "no available keys")
    
    suite.mockRepo.AssertExpectations(suite.T())
}

func (suite *ManagerTestSuite) TestBlacklistKey_Success() {
    // Arrange
    key := fixtures.CreateTestKey("test-key-1")
    keyID := key.ID()
    reason := "test blacklist"
    
    suite.mockRepo.On("GetByID", suite.ctx, keyID).Return(key, nil)
    suite.mockRepo.On("Update", suite.ctx, mock.AnythingOfType("*entity.Key")).Return(nil)
    
    // Act
    err := suite.manager.BlacklistKey(suite.ctx, keyID, reason, false)
    
    // Assert
    assert.NoError(suite.T(), err)
    assert.True(suite.T(), key.IsBlacklisted())
    
    suite.mockRepo.AssertExpectations(suite.T())
}

func TestManagerTestSuite(t *testing.T) {
    suite.Run(t, new(ManagerTestSuite))
}
```

## Integration Test Example

```go
// internal/integration/key_management_test.go
package integration_test

import (
    "context"
    "database/sql"
    "testing"
    "time"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/suite"
    
    "github.com/dbccccccc/tavily-load/internal/infrastructure/database"
    "github.com/dbccccccc/tavily-load/internal/infrastructure/cache"
    "github.com/dbccccccc/tavily-load/internal/domain/key/service"
    "github.com/dbccccccc/tavily-load/internal/domain/key/repository"
    "github.com/dbccccccc/tavily-load/internal/testing/testcontainers"
)

type KeyManagementIntegrationTestSuite struct {
    suite.Suite
    db          *sql.DB
    redisClient *cache.RedisClient
    manager     *service.Manager
    repo        repository.KeyRepository
    ctx         context.Context
    
    // Test containers
    mysqlContainer *testcontainers.MySQLContainer
    redisContainer *testcontainers.RedisContainer
}

func (suite *KeyManagementIntegrationTestSuite) SetupSuite() {
    suite.ctx = context.Background()
    
    // Start test containers
    var err error
    suite.mysqlContainer, err = testcontainers.StartMySQL(suite.ctx)
    suite.Require().NoError(err)
    
    suite.redisContainer, err = testcontainers.StartRedis(suite.ctx)
    suite.Require().NoError(err)
    
    // Connect to databases
    suite.db, err = suite.mysqlContainer.GetDB()
    suite.Require().NoError(err)
    
    suite.redisClient, err = suite.redisContainer.GetClient()
    suite.Require().NoError(err)
    
    // Run migrations
    err = suite.mysqlContainer.RunMigrations("../../../migrations")
    suite.Require().NoError(err)
    
    // Setup services
    suite.repo = repository.NewMySQLKeyRepository(suite.db)
    usageCache := cache.NewUsageCache(suite.redisClient)
    logger := testcontainers.NewTestLogger()
    
    suite.manager = service.NewManager(suite.repo, usageCache, logger)
}

func (suite *KeyManagementIntegrationTestSuite) TearDownSuite() {
    if suite.mysqlContainer != nil {
        suite.mysqlContainer.Terminate(suite.ctx)
    }
    if suite.redisContainer != nil {
        suite.redisContainer.Terminate(suite.ctx)
    }
}

func (suite *KeyManagementIntegrationTestSuite) SetupTest() {
    // Clean database before each test
    suite.cleanDatabase()
}

func (suite *KeyManagementIntegrationTestSuite) TestKeyLifecycle() {
    // Create a key
    key, err := entity.NewKey("test-api-key-123", "Test Key", "Integration test key")
    suite.Require().NoError(err)
    
    err = suite.repo.Create(suite.ctx, key)
    suite.Require().NoError(err)
    
    // Select the key
    selectedKey, err := suite.manager.SelectKey(suite.ctx, "round-robin")
    assert.NoError(suite.T(), err)
    assert.Equal(suite.T(), key.Value(), selectedKey.Value())
    
    // Blacklist the key
    err = suite.manager.BlacklistKey(suite.ctx, key.ID(), "test blacklist", false)
    assert.NoError(suite.T(), err)
    
    // Verify key is blacklisted
    updatedKey, err := suite.repo.GetByID(suite.ctx, key.ID())
    assert.NoError(suite.T(), err)
    assert.True(suite.T(), updatedKey.IsBlacklisted())
    
    // Try to select again (should fail)
    _, err = suite.manager.SelectKey(suite.ctx, "round-robin")
    assert.Error(suite.T(), err)
    assert.Contains(suite.T(), err.Error(), "no available keys")
}

func (suite *KeyManagementIntegrationTestSuite) cleanDatabase() {
    suite.db.Exec("DELETE FROM key_blacklist_history")
    suite.db.Exec("DELETE FROM key_usage_stats")
    suite.db.Exec("DELETE FROM api_keys")
}

func TestKeyManagementIntegrationTestSuite(t *testing.T) {
    suite.Run(t, new(KeyManagementIntegrationTestSuite))
}
```

## Test Fixtures

```go
// internal/testing/fixtures/keys.go
package fixtures

import (
    "time"
    "github.com/dbccccccc/tavily-load/internal/domain/key/entity"
)

// CreateTestKey creates a test key with default values
func CreateTestKey(value string) *entity.Key {
    key, _ := entity.NewKey(value, "Test Key", "Test description")
    return key
}

// CreateTestKeys creates multiple test keys
func CreateTestKeys(count int) []*entity.Key {
    keys := make([]*entity.Key, count)
    for i := 0; i < count; i++ {
        keys[i] = CreateTestKey(fmt.Sprintf("test-key-%d", i+1))
    }
    return keys
}

// CreateBlacklistedKey creates a blacklisted test key
func CreateBlacklistedKey(value string) *entity.Key {
    key := CreateTestKey(value)
    key.Blacklist("test blacklist", false)
    return key
}

// CreateKeyWithUsage creates a key with usage statistics
func CreateKeyWithUsage(value string, requestCount, errorCount int64) *entity.Key {
    key := CreateTestKey(value)
    
    // Simulate usage
    for i := int64(0); i < requestCount; i++ {
        success := i < (requestCount - errorCount)
        key.RecordUsage(success)
    }
    
    return key
}
```

## Test Containers Setup

```go
// internal/testing/testcontainers/mysql.go
package testcontainers

import (
    "context"
    "database/sql"
    "fmt"
    "path/filepath"
    
    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/modules/mysql"
    "github.com/golang-migrate/migrate/v4"
    "github.com/golang-migrate/migrate/v4/database/mysql"
    _ "github.com/golang-migrate/migrate/v4/source/file"
)

type MySQLContainer struct {
    container testcontainers.Container
    db        *sql.DB
}

func StartMySQL(ctx context.Context) (*MySQLContainer, error) {
    mysqlContainer, err := mysql.RunContainer(ctx,
        testcontainers.WithImage("mysql:8.0"),
        mysql.WithDatabase("testdb"),
        mysql.WithUsername("testuser"),
        mysql.WithPassword("testpass"),
    )
    if err != nil {
        return nil, err
    }
    
    connectionString, err := mysqlContainer.ConnectionString(ctx)
    if err != nil {
        return nil, err
    }
    
    db, err := sql.Open("mysql", connectionString)
    if err != nil {
        return nil, err
    }
    
    return &MySQLContainer{
        container: mysqlContainer,
        db:        db,
    }, nil
}

func (c *MySQLContainer) GetDB() (*sql.DB, error) {
    return c.db, nil
}

func (c *MySQLContainer) RunMigrations(migrationPath string) error {
    driver, err := mysql.WithInstance(c.db, &mysql.Config{})
    if err != nil {
        return err
    }
    
    absPath, err := filepath.Abs(migrationPath)
    if err != nil {
        return err
    }
    
    m, err := migrate.NewWithDatabaseInstance(
        fmt.Sprintf("file://%s", absPath),
        "mysql",
        driver,
    )
    if err != nil {
        return err
    }
    defer m.Close()
    
    return m.Up()
}

func (c *MySQLContainer) Terminate(ctx context.Context) error {
    if c.db != nil {
        c.db.Close()
    }
    return c.container.Terminate(ctx)
}
```

## Benchmark Tests

```go
// internal/domain/key/service/manager_bench_test.go
package service_test

import (
    "context"
    "testing"
    
    "github.com/dbccccccc/tavily-load/internal/domain/key/service"
    "github.com/dbccccccc/tavily-load/internal/testing/fixtures"
    "github.com/dbccccccc/tavily-load/internal/testing/mocks"
)

func BenchmarkManager_SelectKey(b *testing.B) {
    // Setup
    mockRepo := &mocks.MockKeyRepository{}
    mockCache := &mocks.MockCacheService{}
    mockLogger := &mocks.MockLogger{}
    
    manager := service.NewManager(mockRepo, mockCache, mockLogger)
    keys := fixtures.CreateTestKeys(100)
    ctx := context.Background()
    
    mockRepo.On("GetActiveKeys", ctx).Return(keys, nil)
    
    b.ResetTimer()
    
    // Benchmark
    for i := 0; i < b.N; i++ {
        _, err := manager.SelectKey(ctx, "round-robin")
        if err != nil {
            b.Fatal(err)
        }
    }
}

func BenchmarkManager_SelectKey_Concurrent(b *testing.B) {
    // Setup similar to above
    // ...
    
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            _, err := manager.SelectKey(ctx, "round-robin")
            if err != nil {
                b.Fatal(err)
            }
        }
    })
}
```

This comprehensive testing framework provides:

1. **Unit Tests**: Fast, isolated tests with mocks
2. **Integration Tests**: Real database/cache testing with containers
3. **Test Fixtures**: Reusable test data creation
4. **Test Utilities**: Helper functions and mocks
5. **Benchmark Tests**: Performance testing
6. **Test Containers**: Isolated test environments
7. **Test Suites**: Organized test execution
