# Enhanced Error Handling and Observability

## Structured Error Types

```go
// internal/domain/errors/types.go
package errors

import (
    "fmt"
    "time"
)

// ErrorCode represents a structured error code
type ErrorCode string

const (
    // Key management errors
    ErrCodeKeyNotFound      ErrorCode = "KEY_NOT_FOUND"
    ErrCodeKeyBlacklisted   ErrorCode = "KEY_BLACKLISTED"
    ErrCodeNoKeysAvailable  ErrorCode = "NO_KEYS_AVAILABLE"
    ErrCodeKeyInvalid       ErrorCode = "KEY_INVALID"
    
    // Proxy errors
    ErrCodeProxyTimeout     ErrorCode = "PROXY_TIMEOUT"
    ErrCodeProxyRateLimit   ErrorCode = "PROXY_RATE_LIMIT"
    ErrCodeProxyNetworkError ErrorCode = "PROXY_NETWORK_ERROR"
    
    // External API errors
    ErrCodeTavilyAPIError   ErrorCode = "TAVILY_API_ERROR"
    ErrCodeTavilyQuotaExceeded ErrorCode = "TAVILY_QUOTA_EXCEEDED"
    ErrCodeTavilyUnauthorized ErrorCode = "TAVILY_UNAUTHORIZED"
    
    // Infrastructure errors
    ErrCodeDatabaseError    ErrorCode = "DATABASE_ERROR"
    ErrCodeCacheError       ErrorCode = "CACHE_ERROR"
    ErrCodeConfigError      ErrorCode = "CONFIG_ERROR"
)

// DomainError represents a structured domain error
type DomainError struct {
    Code        ErrorCode              `json:"code"`
    Message     string                 `json:"message"`
    Details     map[string]interface{} `json:"details,omitempty"`
    Cause       error                  `json:"-"`
    Timestamp   time.Time              `json:"timestamp"`
    Retryable   bool                   `json:"retryable"`
    Permanent   bool                   `json:"permanent"`
    HTTPStatus  int                    `json:"http_status"`
}

func (e *DomainError) Error() string {
    if e.Cause != nil {
        return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
    }
    return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *DomainError) Unwrap() error {
    return e.Cause
}

func (e *DomainError) WithDetail(key string, value interface{}) *DomainError {
    if e.Details == nil {
        e.Details = make(map[string]interface{})
    }
    e.Details[key] = value
    return e
}

// Error constructors
func NewKeyNotFoundError(keyID string) *DomainError {
    return &DomainError{
        Code:       ErrCodeKeyNotFound,
        Message:    "API key not found",
        Details:    map[string]interface{}{"key_id": keyID},
        Timestamp:  time.Now(),
        Retryable:  false,
        Permanent:  false,
        HTTPStatus: 404,
    }
}

func NewKeyBlacklistedError(keyID string, reason string) *DomainError {
    return &DomainError{
        Code:       ErrCodeKeyBlacklisted,
        Message:    "API key is blacklisted",
        Details:    map[string]interface{}{"key_id": keyID, "reason": reason},
        Timestamp:  time.Now(),
        Retryable:  true,
        Permanent:  false,
        HTTPStatus: 503,
    }
}

func NewTavilyAPIError(statusCode int, message string, keyID string) *DomainError {
    return &DomainError{
        Code:       ErrCodeTavilyAPIError,
        Message:    message,
        Details:    map[string]interface{}{"key_id": keyID, "api_status": statusCode},
        Timestamp:  time.Now(),
        Retryable:  statusCode >= 500,
        Permanent:  statusCode == 401 || statusCode == 403,
        HTTPStatus: statusCode,
    }
}
```

## Structured Logging

```go
// internal/infrastructure/logging/logger.go
package logging

import (
    "context"
    "time"
    
    "github.com/sirupsen/logrus"
    "go.opentelemetry.io/otel/trace"
)

// StructuredLogger provides structured logging with context
type StructuredLogger struct {
    logger *logrus.Logger
}

func NewStructuredLogger(cfg *LogConfig) *StructuredLogger {
    logger := logrus.New()
    
    // Configure formatter
    if cfg.Format == "json" {
        logger.SetFormatter(&logrus.JSONFormatter{
            TimestampFormat: time.RFC3339,
            FieldMap: logrus.FieldMap{
                logrus.FieldKeyTime:  "timestamp",
                logrus.FieldKeyLevel: "level",
                logrus.FieldKeyMsg:   "message",
            },
        })
    }
    
    // Set level
    if level, err := logrus.ParseLevel(cfg.Level); err == nil {
        logger.SetLevel(level)
    }
    
    return &StructuredLogger{logger: logger}
}

// LogContext provides structured context for logging
type LogContext struct {
    RequestID   string
    UserID      string
    KeyID       string
    Operation   string
    Component   string
    TraceID     string
    SpanID      string
}

func (l *StructuredLogger) WithContext(ctx context.Context) *logrus.Entry {
    entry := l.logger.WithFields(logrus.Fields{})
    
    // Extract request context
    if reqCtx := GetRequestContext(ctx); reqCtx != nil {
        entry = entry.WithFields(logrus.Fields{
            "request_id": reqCtx.RequestID,
            "user_id":    reqCtx.UserID,
            "client_ip":  reqCtx.ClientIP,
            "user_agent": reqCtx.UserAgent,
        })
    }
    
    // Extract trace context
    if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
        entry = entry.WithFields(logrus.Fields{
            "trace_id": span.SpanContext().TraceID().String(),
            "span_id":  span.SpanContext().SpanID().String(),
        })
    }
    
    return entry
}

func (l *StructuredLogger) LogError(ctx context.Context, err error, operation string) {
    entry := l.WithContext(ctx).WithFields(logrus.Fields{
        "operation": operation,
        "error":     err.Error(),
    })
    
    // Add structured error details if available
    if domainErr, ok := err.(*DomainError); ok {
        entry = entry.WithFields(logrus.Fields{
            "error_code":    domainErr.Code,
            "error_details": domainErr.Details,
            "retryable":     domainErr.Retryable,
            "permanent":     domainErr.Permanent,
        })
    }
    
    entry.Error("Operation failed")
}

func (l *StructuredLogger) LogKeySelection(ctx context.Context, keyID, strategy string, duration time.Duration) {
    l.WithContext(ctx).WithFields(logrus.Fields{
        "key_id":     keyID,
        "strategy":   strategy,
        "duration":   duration,
        "operation":  "key_selection",
        "component":  "key_manager",
    }).Info("Key selected")
}

func (l *StructuredLogger) LogProxyRequest(ctx context.Context, endpoint, keyID string, statusCode int, duration time.Duration) {
    entry := l.WithContext(ctx).WithFields(logrus.Fields{
        "endpoint":    endpoint,
        "key_id":      keyID,
        "status_code": statusCode,
        "duration":    duration,
        "operation":   "proxy_request",
        "component":   "proxy",
    })
    
    if statusCode >= 400 {
        entry.Warn("Proxy request failed")
    } else {
        entry.Info("Proxy request successful")
    }
}
```

## Metrics Collection

```go
// internal/infrastructure/metrics/collector.go
package metrics

import (
    "context"
    "time"
    
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all application metrics
type Metrics struct {
    // Request metrics
    RequestsTotal    *prometheus.CounterVec
    RequestDuration  *prometheus.HistogramVec
    RequestsInFlight *prometheus.GaugeVec
    
    // Key management metrics
    KeysTotal        *prometheus.GaugeVec
    KeySelections    *prometheus.CounterVec
    KeyErrors        *prometheus.CounterVec
    KeyBlacklists    *prometheus.CounterVec
    
    // Proxy metrics
    ProxyRequests    *prometheus.CounterVec
    ProxyDuration    *prometheus.HistogramVec
    ProxyErrors      *prometheus.CounterVec
    
    // Infrastructure metrics
    DatabaseConnections *prometheus.GaugeVec
    CacheHitRate       *prometheus.GaugeVec
}

func NewMetrics() *Metrics {
    return &Metrics{
        RequestsTotal: promauto.NewCounterVec(
            prometheus.CounterOpts{
                Name: "tavily_load_requests_total",
                Help: "Total number of requests processed",
            },
            []string{"method", "endpoint", "status"},
        ),
        
        RequestDuration: promauto.NewHistogramVec(
            prometheus.HistogramOpts{
                Name:    "tavily_load_request_duration_seconds",
                Help:    "Request duration in seconds",
                Buckets: prometheus.DefBuckets,
            },
            []string{"method", "endpoint"},
        ),
        
        KeysTotal: promauto.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "tavily_load_keys_total",
                Help: "Total number of API keys",
            },
            []string{"status"},
        ),
        
        KeySelections: promauto.NewCounterVec(
            prometheus.CounterOpts{
                Name: "tavily_load_key_selections_total",
                Help: "Total number of key selections",
            },
            []string{"strategy", "result"},
        ),
        
        ProxyRequests: promauto.NewCounterVec(
            prometheus.CounterOpts{
                Name: "tavily_load_proxy_requests_total",
                Help: "Total number of proxy requests",
            },
            []string{"endpoint", "key_id", "status"},
        ),
    }
}

// RecordRequest records a request metric
func (m *Metrics) RecordRequest(method, endpoint, status string, duration time.Duration) {
    m.RequestsTotal.WithLabelValues(method, endpoint, status).Inc()
    m.RequestDuration.WithLabelValues(method, endpoint).Observe(duration.Seconds())
}

// RecordKeySelection records a key selection metric
func (m *Metrics) RecordKeySelection(strategy, result string) {
    m.KeySelections.WithLabelValues(strategy, result).Inc()
}

// RecordProxyRequest records a proxy request metric
func (m *Metrics) RecordProxyRequest(endpoint, keyID, status string) {
    m.ProxyRequests.WithLabelValues(endpoint, keyID, status).Inc()
}

// UpdateKeyStats updates key statistics
func (m *Metrics) UpdateKeyStats(active, blacklisted, total int) {
    m.KeysTotal.WithLabelValues("active").Set(float64(active))
    m.KeysTotal.WithLabelValues("blacklisted").Set(float64(blacklisted))
    m.KeysTotal.WithLabelValues("total").Set(float64(total))
}
```

## Health Check System

```go
// internal/infrastructure/health/checker.go
package health

import (
    "context"
    "database/sql"
    "time"
    
    "github.com/dbccccccc/tavily-load/internal/infrastructure/cache"
)

// HealthChecker provides comprehensive health checking
type HealthChecker struct {
    db          *sql.DB
    redisClient *cache.RedisClient
    checks      map[string]HealthCheck
}

type HealthCheck interface {
    Name() string
    Check(ctx context.Context) HealthResult
}

type HealthResult struct {
    Status    HealthStatus           `json:"status"`
    Message   string                 `json:"message"`
    Details   map[string]interface{} `json:"details,omitempty"`
    Duration  time.Duration          `json:"duration"`
    Timestamp time.Time              `json:"timestamp"`
}

type HealthStatus string

const (
    HealthStatusHealthy   HealthStatus = "healthy"
    HealthStatusDegraded  HealthStatus = "degraded"
    HealthStatusUnhealthy HealthStatus = "unhealthy"
)

func NewHealthChecker(db *sql.DB, redisClient *cache.RedisClient) *HealthChecker {
    checker := &HealthChecker{
        db:          db,
        redisClient: redisClient,
        checks:      make(map[string]HealthCheck),
    }
    
    // Register default checks
    checker.RegisterCheck(&DatabaseHealthCheck{db: db})
    checker.RegisterCheck(&RedisHealthCheck{client: redisClient})
    checker.RegisterCheck(&DiskSpaceHealthCheck{})
    
    return checker
}

func (h *HealthChecker) RegisterCheck(check HealthCheck) {
    h.checks[check.Name()] = check
}

func (h *HealthChecker) CheckAll(ctx context.Context) map[string]HealthResult {
    results := make(map[string]HealthResult)
    
    for name, check := range h.checks {
        start := time.Now()
        result := check.Check(ctx)
        result.Duration = time.Since(start)
        result.Timestamp = time.Now()
        results[name] = result
    }
    
    return results
}

// Database health check
type DatabaseHealthCheck struct {
    db *sql.DB
}

func (c *DatabaseHealthCheck) Name() string {
    return "database"
}

func (c *DatabaseHealthCheck) Check(ctx context.Context) HealthResult {
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    
    if err := c.db.PingContext(ctx); err != nil {
        return HealthResult{
            Status:  HealthStatusUnhealthy,
            Message: "Database connection failed",
            Details: map[string]interface{}{"error": err.Error()},
        }
    }
    
    return HealthResult{
        Status:  HealthStatusHealthy,
        Message: "Database connection healthy",
    }
}
```

This enhanced error handling and observability system provides:

1. **Structured Errors**: Clear error codes and context
2. **Rich Logging**: Contextual and traceable logs
3. **Comprehensive Metrics**: Prometheus-compatible metrics
4. **Health Monitoring**: Multi-component health checks
5. **Tracing Support**: OpenTelemetry integration ready
6. **Debugging**: Detailed error information for troubleshooting
