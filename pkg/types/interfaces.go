package types

import (
	"context"
	"net/http"
	"time"
)

// KeyManager defines the interface for API key management
type KeyManager interface {
	GetNextKey() (string, error)
	BlacklistKey(key string, permanent bool)
	ResetKeys()
	GetStats() KeyStats
	GetBlacklist() []BlacklistEntry
}

// ProxyServer defines the interface for the proxy server
type ProxyServer interface {
	Start() error
	Stop(ctx context.Context) error
	Health() HealthStatus
}

// KeyStats represents statistics for key usage
type KeyStats struct {
	TotalKeys       int                  `json:"total_keys"`
	ActiveKeys      int                  `json:"active_keys"`
	BlacklistedKeys int                  `json:"blacklisted_keys"`
	CurrentIndex    int                  `json:"current_index"`
	RequestCounts   map[string]int       `json:"request_counts"`
	ErrorCounts     map[string]int       `json:"error_counts"`
	LastUsed        map[string]time.Time `json:"last_used"`
	KeyStatus       map[string]KeyStatus `json:"key_status"`
}

// KeyStatus represents the status of an API key
type KeyStatus struct {
	Active        bool      `json:"active"`
	ErrorCount    int       `json:"error_count"`
	RequestCount  int       `json:"request_count"`
	LastUsed      time.Time `json:"last_used"`
	LastError     string    `json:"last_error,omitempty"`
	BlacklistedAt time.Time `json:"blacklisted_at,omitempty"`
	Permanent     bool      `json:"permanent"`
}

// BlacklistEntry represents a blacklisted key
type BlacklistEntry struct {
	Key           string    `json:"key"`
	Reason        string    `json:"reason"`
	BlacklistedAt time.Time `json:"blacklisted_at"`
	Permanent     bool      `json:"permanent"`
	ErrorCount    int       `json:"error_count"`
}

// HealthStatus represents the health status of the service
type HealthStatus struct {
	Status      string           `json:"status"`
	Timestamp   time.Time        `json:"timestamp"`
	Version     string           `json:"version"`
	Uptime      time.Duration    `json:"uptime"`
	KeyManager  KeyManagerHealth `json:"key_manager"`
	Server      ServerHealth     `json:"server"`
	Connections ConnectionHealth `json:"connections"`
}

// KeyManagerHealth represents key manager health
type KeyManagerHealth struct {
	TotalKeys       int `json:"total_keys"`
	ActiveKeys      int `json:"active_keys"`
	BlacklistedKeys int `json:"blacklisted_keys"`
}

// ServerHealth represents server health
type ServerHealth struct {
	RequestsTotal   int64         `json:"requests_total"`
	RequestsSuccess int64         `json:"requests_success"`
	RequestsError   int64         `json:"requests_error"`
	AverageLatency  time.Duration `json:"average_latency"`
}

// ConnectionHealth represents connection health
type ConnectionHealth struct {
	ActiveConnections int `json:"active_connections"`
	TotalConnections  int `json:"total_connections"`
}

// TavilyRequest represents a generic Tavily API request
type TavilyRequest struct {
	Method   string            `json:"method"`
	Endpoint string            `json:"endpoint"`
	Headers  map[string]string `json:"headers"`
	Body     interface{}       `json:"body"`
}

// TavilyResponse represents a generic Tavily API response
type TavilyResponse struct {
	StatusCode   int               `json:"status_code"`
	Headers      map[string]string `json:"headers"`
	Body         interface{}       `json:"body"`
	ResponseTime time.Duration     `json:"response_time"`
}

// RequestContext contains context information for a request
type RequestContext struct {
	RequestID    string
	StartTime    time.Time
	Key          string
	Endpoint     string
	Method       string
	ClientIP     string
	UserAgent    string
	RetryCount   int
	ResponseTime time.Duration
}

// Middleware defines the interface for HTTP middleware
type Middleware interface {
	Handler(next http.Handler) http.Handler
}

// UsageTracker defines the interface for tracking API usage
type UsageTracker interface {
	UpdateUsage(key string, usage *TavilyUsage) error
	GetUsage(key string) (*TavilyUsage, error)
	GetAllUsage() map[string]*TavilyUsage
	GetOptimalKey(strategy SelectionStrategy) (string, error)
	CalculateRemainingPoints(key string) (*RemainingPoints, error)
	UpdateKeyMetrics(key string, success bool, latency time.Duration)
	GetRecommendedStrategy() SelectionStrategy
	FetchUsageFromAPI(key string) (*TavilyUsage, error)
}

// TavilyUsage represents the usage response from Tavily API
type TavilyUsage struct {
	Key     KeyUsage     `json:"key"`
	Account AccountUsage `json:"account"`
}

// KeyUsage represents individual key usage
type KeyUsage struct {
	Usage int `json:"usage"`
	Limit int `json:"limit"`
}

// AccountUsage represents account-level usage
type AccountUsage struct {
	CurrentPlan string `json:"current_plan"`
	PlanUsage   int    `json:"plan_usage"`
	PlanLimit   int    `json:"plan_limit"`
	PaygoUsage  int    `json:"paygo_usage"`
	PaygoLimit  int    `json:"paygo_limit"`
}

// RemainingPoints represents calculated remaining points
type RemainingPoints struct {
	KeyRemaining     int     `json:"key_remaining"`
	PlanRemaining    int     `json:"plan_remaining"`
	PaygoRemaining   int     `json:"paygo_remaining"`
	TotalRemaining   int     `json:"total_remaining"`
	KeyUtilization   float64 `json:"key_utilization"`
	PlanUtilization  float64 `json:"plan_utilization"`
	PaygoUtilization float64 `json:"paygo_utilization"`
}

// SelectionStrategy defines different key selection strategies
type SelectionStrategy string

const (
	StrategyPlanFirst  SelectionStrategy = "plan_first"  // Default: Prefer plan credits over paygo, only switch to paid when no plans available
	StrategyRoundRobin SelectionStrategy = "round_robin" // Round-robin selection across all available keys
)

// UsageStrategy represents a usage optimization strategy
type UsageStrategy struct {
	Strategy         SelectionStrategy `json:"strategy"`
	Description      string            `json:"description"`
	PreferPlan       bool              `json:"prefer_plan"`
	PreferPaygo      bool              `json:"prefer_paygo"`
	ThresholdPercent float64           `json:"threshold_percent"`
	CostWeight       float64           `json:"cost_weight"`
	BalanceWeight    float64           `json:"balance_weight"`
}

// UsageAnalytics represents comprehensive usage analytics
type UsageAnalytics struct {
	TotalKeys           int                                    `json:"total_keys"`
	ActiveKeys          int                                    `json:"active_keys"`
	KeysWithUsage       int                                    `json:"keys_with_usage"`
	TotalPlanUsage      int                                    `json:"total_plan_usage"`
	TotalPlanLimit      int                                    `json:"total_plan_limit"`
	TotalPaygoUsage     int                                    `json:"total_paygo_usage"`
	TotalPaygoLimit     int                                    `json:"total_paygo_limit"`
	AveragePlanUtil     float64                                `json:"average_plan_utilization"`
	AveragePaygoUtil    float64                                `json:"average_paygo_utilization"`
	RecommendedStrategy SelectionStrategy                      `json:"recommended_strategy"`
	KeyAnalytics        map[string]*KeyAnalytics               `json:"key_analytics"`
	StrategyMetrics     map[SelectionStrategy]*StrategyMetrics `json:"strategy_metrics"`
}

// KeyAnalytics represents analytics for a specific key
type KeyAnalytics struct {
	Key             string           `json:"key"`
	Usage           *TavilyUsage     `json:"usage"`
	RemainingPoints *RemainingPoints `json:"remaining_points"`
	RequestCount    int64            `json:"request_count"`
	ErrorCount      int64            `json:"error_count"`
	LastUsed        time.Time        `json:"last_used"`
	LastUpdated     time.Time        `json:"last_updated"`
	HealthScore     float64          `json:"health_score"`
	CostEfficiency  float64          `json:"cost_efficiency"`
	RecommendedUse  bool             `json:"recommended_use"`
}

// StrategyMetrics represents metrics for a selection strategy
type StrategyMetrics struct {
	Strategy       SelectionStrategy `json:"strategy"`
	TimesUsed      int64             `json:"times_used"`
	SuccessRate    float64           `json:"success_rate"`
	AverageLatency time.Duration     `json:"average_latency"`
	CostEfficiency float64           `json:"cost_efficiency"`
	LastUsed       time.Time         `json:"last_used"`
}
