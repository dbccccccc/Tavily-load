package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
)

// Config holds all configuration for the application
type Config struct {
	// Server Configuration
	Port string `json:"port"`
	Host string `json:"host"`

	// Database Configuration
	DBHost           string        `json:"db_host"`
	DBPort           string        `json:"db_port"`
	DBUsername       string        `json:"db_username"`
	DBPassword       string        `json:"db_password"`
	DBName           string        `json:"db_name"`
	DBMaxOpenConns   int           `json:"db_max_open_conns"`
	DBMaxIdleConns   int           `json:"db_max_idle_conns"`
	DBConnMaxLifetime time.Duration `json:"db_conn_max_lifetime"`

	// Redis Configuration
	RedisHost     string `json:"redis_host"`
	RedisPort     string `json:"redis_port"`
	RedisPassword string `json:"redis_password"`
	RedisDB       int    `json:"redis_db"`
	RedisPoolSize int    `json:"redis_pool_size"`

	// Migration Configuration
	MigrateUp     bool   `json:"migrate_up"`
	MigrationPath string `json:"migration_path"`

	// API Keys Configuration (Legacy - now stored in database)
	KeysFile   string `json:"keys_file"`
	StartIndex int    `json:"start_index"`

	// Load Balancing & Error Handling
	BlacklistThreshold    int `json:"blacklist_threshold"`
	MaxRetries            int `json:"max_retries"`
	MaxConcurrentRequests int `json:"max_concurrent_requests"`

	// Tavily API Configuration
	TavilyBaseURL   string        `json:"tavily_base_url"`
	RequestTimeout  time.Duration `json:"request_timeout"`
	ResponseTimeout time.Duration `json:"response_timeout"`
	IdleConnTimeout time.Duration `json:"idle_conn_timeout"`

	// Authentication (Optional)
	AuthKey string `json:"auth_key,omitempty"`

	// CORS Configuration
	EnableCORS       bool     `json:"enable_cors"`
	AllowedOrigins   []string `json:"allowed_origins"`
	AllowedMethods   []string `json:"allowed_methods"`
	AllowedHeaders   []string `json:"allowed_headers"`
	AllowCredentials bool     `json:"allow_credentials"`

	// Compression
	EnableGzip bool `json:"enable_gzip"`

	// Logging Configuration
	LogLevel         string `json:"log_level"`
	LogFormat        string `json:"log_format"`
	LogEnableFile    bool   `json:"log_enable_file"`
	LogFilePath      string `json:"log_file_path"`
	LogEnableRequest bool   `json:"log_enable_request"`

	// Server Timeouts
	ServerReadTimeout             time.Duration `json:"server_read_timeout"`
	ServerWriteTimeout            time.Duration `json:"server_write_timeout"`
	ServerIdleTimeout             time.Duration `json:"server_idle_timeout"`
	ServerGracefulShutdownTimeout time.Duration `json:"server_graceful_shutdown_timeout"`

	// Usage Tracking Configuration
	EnableUsageTracking      bool          `json:"enable_usage_tracking"`
	UsageUpdateInterval      time.Duration `json:"usage_update_interval"`
	DefaultSelectionStrategy string        `json:"default_selection_strategy"`
	AutoStrategyOptimization bool          `json:"auto_strategy_optimization"`

	// Cache Configuration
	CacheUsageTTL     time.Duration `json:"cache_usage_ttl"`
	CacheAnalyticsTTL time.Duration `json:"cache_analytics_ttl"`
	CacheStatsTTL     time.Duration `json:"cache_stats_ttl"`
	CacheBlacklistTTL time.Duration `json:"cache_blacklist_ttl"`
}

// Manager handles configuration loading and management
type Manager struct {
	config *Config
	logger *logrus.Logger
}

// NewManager creates a new configuration manager
func NewManager(logger *logrus.Logger) *Manager {
	return &Manager{
		logger: logger,
	}
}

// Load loads configuration from environment variables and .env file
func (m *Manager) Load() (*Config, error) {
	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		m.logger.Debug("No .env file found, using environment variables only")
	}

	config := &Config{
		// Server Configuration
		Port: getEnvString("PORT", "3000"),
		Host: getEnvString("HOST", "0.0.0.0"),

		// Database Configuration
		DBHost:            getEnvString("DB_HOST", "localhost"),
		DBPort:            getEnvString("DB_PORT", "3306"),
		DBUsername:        getEnvString("DB_USERNAME", "tavily_user"),
		DBPassword:        getEnvString("DB_PASSWORD", "tavily_password"),
		DBName:            getEnvString("DB_NAME", "tavily_load"),
		DBMaxOpenConns:    getEnvInt("DB_MAX_OPEN_CONNS", 25),
		DBMaxIdleConns:    getEnvInt("DB_MAX_IDLE_CONNS", 5),
		DBConnMaxLifetime: getEnvDuration("DB_CONN_MAX_LIFETIME", 300*time.Second),

		// Redis Configuration
		RedisHost:     getEnvString("REDIS_HOST", "localhost"),
		RedisPort:     getEnvString("REDIS_PORT", "6379"),
		RedisPassword: getEnvString("REDIS_PASSWORD", ""),
		RedisDB:       getEnvInt("REDIS_DB", 0),
		RedisPoolSize: getEnvInt("REDIS_POOL_SIZE", 10),

		// Migration Configuration
		MigrateUp:     getEnvBool("MIGRATE_UP", false),
		MigrationPath: getEnvString("MIGRATION_PATH", "migrations"),

		// API Keys Configuration (Legacy - now stored in database)
		KeysFile:   getEnvString("KEYS_FILE", "keys.txt"),
		StartIndex: getEnvInt("START_INDEX", 0),

		// Load Balancing & Error Handling
		BlacklistThreshold:    getEnvInt("BLACKLIST_THRESHOLD", 1),
		MaxRetries:            getEnvInt("MAX_RETRIES", 3),
		MaxConcurrentRequests: getEnvInt("MAX_CONCURRENT_REQUESTS", 100),

		// Tavily API Configuration
		TavilyBaseURL:   getEnvString("TAVILY_BASE_URL", "https://api.tavily.com"),
		RequestTimeout:  getEnvDuration("REQUEST_TIMEOUT", 30*time.Second),
		ResponseTimeout: getEnvDuration("RESPONSE_TIMEOUT", 30*time.Second),
		IdleConnTimeout: getEnvDuration("IDLE_CONN_TIMEOUT", 120*time.Second),

		// Authentication (Optional)
		AuthKey: getEnvString("AUTH_KEY", ""),

		// CORS Configuration
		EnableCORS:       getEnvBool("ENABLE_CORS", true),
		AllowedOrigins:   getEnvStringSlice("ALLOWED_ORIGINS", []string{"*"}),
		AllowedMethods:   getEnvStringSlice("ALLOWED_METHODS", []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}),
		AllowedHeaders:   getEnvStringSlice("ALLOWED_HEADERS", []string{"*"}),
		AllowCredentials: getEnvBool("ALLOW_CREDENTIALS", false),

		// Compression
		EnableGzip: getEnvBool("ENABLE_GZIP", true),

		// Logging Configuration
		LogLevel:         getEnvString("LOG_LEVEL", "info"),
		LogFormat:        getEnvString("LOG_FORMAT", "text"),
		LogEnableFile:    getEnvBool("LOG_ENABLE_FILE", false),
		LogFilePath:      getEnvString("LOG_FILE_PATH", "logs/app.log"),
		LogEnableRequest: getEnvBool("LOG_ENABLE_REQUEST", true),

		// Server Timeouts
		ServerReadTimeout:             getEnvDuration("SERVER_READ_TIMEOUT", 120*time.Second),
		ServerWriteTimeout:            getEnvDuration("SERVER_WRITE_TIMEOUT", 1800*time.Second),
		ServerIdleTimeout:             getEnvDuration("SERVER_IDLE_TIMEOUT", 120*time.Second),
		ServerGracefulShutdownTimeout: getEnvDuration("SERVER_GRACEFUL_SHUTDOWN_TIMEOUT", 60*time.Second),

		// Usage Tracking Configuration
		EnableUsageTracking:      getEnvBool("ENABLE_USAGE_TRACKING", true),
		UsageUpdateInterval:      getEnvDuration("USAGE_UPDATE_INTERVAL", 300*time.Second), // 5 minutes
		DefaultSelectionStrategy: getEnvString("DEFAULT_SELECTION_STRATEGY", "round_robin"),
		AutoStrategyOptimization: getEnvBool("AUTO_STRATEGY_OPTIMIZATION", false),

		// Cache Configuration
		CacheUsageTTL:     getEnvDuration("CACHE_USAGE_TTL", 300*time.Second),
		CacheAnalyticsTTL: getEnvDuration("CACHE_ANALYTICS_TTL", 600*time.Second),
		CacheStatsTTL:     getEnvDuration("CACHE_STATS_TTL", 120*time.Second),
		CacheBlacklistTTL: getEnvDuration("CACHE_BLACKLIST_TTL", 3600*time.Second),
	}

	// Validate configuration
	if err := m.validate(config); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	m.config = config
	return config, nil
}

// GetConfig returns the current configuration
func (m *Manager) GetConfig() *Config {
	return m.config
}

// validate validates the configuration
func (m *Manager) validate(config *Config) error {
	// Validate database configuration
	if config.DBHost == "" {
		return fmt.Errorf("DB_HOST is required")
	}
	if config.DBUsername == "" {
		return fmt.Errorf("DB_USERNAME is required")
	}
	if config.DBPassword == "" {
		return fmt.Errorf("DB_PASSWORD is required")
	}
	if config.DBName == "" {
		return fmt.Errorf("DB_NAME is required")
	}

	// Validate Redis configuration
	if config.RedisHost == "" {
		return fmt.Errorf("REDIS_HOST is required")
	}

	// Validate required fields
	if config.TavilyBaseURL == "" {
		return fmt.Errorf("TAVILY_BASE_URL is required")
	}

	// Validate numeric ranges
	if config.MaxRetries < 0 {
		return fmt.Errorf("MAX_RETRIES must be >= 0")
	}

	if config.MaxConcurrentRequests <= 0 {
		return fmt.Errorf("MAX_CONCURRENT_REQUESTS must be > 0")
	}

	if config.BlacklistThreshold <= 0 {
		return fmt.Errorf("BLACKLIST_THRESHOLD must be > 0")
	}

	if config.DBMaxOpenConns <= 0 {
		return fmt.Errorf("DB_MAX_OPEN_CONNS must be > 0")
	}

	if config.DBMaxIdleConns < 0 {
		return fmt.Errorf("DB_MAX_IDLE_CONNS must be >= 0")
	}

	if config.RedisPoolSize <= 0 {
		return fmt.Errorf("REDIS_POOL_SIZE must be > 0")
	}

	// Validate log level
	validLogLevels := []string{"debug", "info", "warn", "error"}
	if !contains(validLogLevels, config.LogLevel) {
		return fmt.Errorf("LOG_LEVEL must be one of: %s", strings.Join(validLogLevels, ", "))
	}

	// Validate log format
	validLogFormats := []string{"text", "json"}
	if !contains(validLogFormats, config.LogFormat) {
		return fmt.Errorf("LOG_FORMAT must be one of: %s", strings.Join(validLogFormats, ", "))
	}

	return nil
}

// Helper functions for environment variable parsing
func getEnvString(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if seconds, err := strconv.Atoi(value); err == nil {
			return time.Duration(seconds) * time.Second
		}
	}
	return defaultValue
}

func getEnvStringSlice(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		return strings.Split(value, ",")
	}
	return defaultValue
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
