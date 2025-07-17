package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"github.com/dbccccccc/tavily-load/internal/cache"
	"github.com/dbccccccc/tavily-load/internal/config"
	"github.com/dbccccccc/tavily-load/internal/handler"
	"github.com/dbccccccc/tavily-load/internal/keymanager"
	"github.com/dbccccccc/tavily-load/internal/middleware"
	"github.com/dbccccccc/tavily-load/internal/repository"
	"github.com/dbccccccc/tavily-load/pkg/types"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
	"github.com/sirupsen/logrus"
)

// Server implements the ProxyServer interface
type Server struct {
	config      *config.Config
	logger      *logrus.Logger
	keyManager  *keymanager.Manager
	handler     *handler.Handler
	httpServer  *http.Server
	startTime   time.Time
	keyRepo     *repository.KeyRepository
	usageCache  *cache.UsageCache
}

// NewServer creates a new proxy server
func NewServer(cfg *config.Config, logger *logrus.Logger, keyRepo *repository.KeyRepository, usageCache *cache.UsageCache) (*Server, error) {
	// Create key manager
	keyManager, err := keymanager.NewManager(cfg, logger, keyRepo, usageCache)
	if err != nil {
		return nil, fmt.Errorf("failed to create key manager: %w", err)
	}

	// Create handler
	h := handler.NewHandler(keyManager, cfg, logger, keyRepo)

	server := &Server{
		config:     cfg,
		logger:     logger,
		keyManager: keyManager,
		handler:    h,
		startTime:  time.Now(),
		keyRepo:    keyRepo,
		usageCache: usageCache,
	}

	// Setup HTTP server
	if err := server.setupServer(); err != nil {
		return nil, fmt.Errorf("failed to setup server: %w", err)
	}

	return server, nil
}

// setupServer configures the HTTP server with routes and middleware
func (s *Server) setupServer() error {
	// Create router
	router := mux.NewRouter()

	// Setup middleware chain
	s.setupMiddleware(router)

	// Setup routes
	s.setupRoutes(router)

	// Setup CORS if enabled
	var finalHandler http.Handler = router
	if s.config.EnableCORS {
		corsHandler := cors.New(cors.Options{
			AllowedOrigins:   s.config.AllowedOrigins,
			AllowedMethods:   s.config.AllowedMethods,
			AllowedHeaders:   s.config.AllowedHeaders,
			AllowCredentials: s.config.AllowCredentials,
		})
		finalHandler = corsHandler.Handler(router)
	}

	// Create HTTP server
	s.httpServer = &http.Server{
		Addr:         s.config.Host + ":" + s.config.Port,
		Handler:      finalHandler,
		ReadTimeout:  s.config.ServerReadTimeout,
		WriteTimeout: s.config.ServerWriteTimeout,
		IdleTimeout:  s.config.ServerIdleTimeout,
	}

	return nil
}

// setupMiddleware configures middleware for the router
func (s *Server) setupMiddleware(router *mux.Router) {
	// Recovery middleware (should be first)
	recoveryMiddleware := middleware.NewRecoveryMiddleware(s.logger)
	router.Use(recoveryMiddleware.Handler)

	// Request ID middleware
	requestIDMiddleware := middleware.NewRequestIDMiddleware(s.logger)
	router.Use(requestIDMiddleware.Handler)

	// Logging middleware
	loggingMiddleware := middleware.NewLoggingMiddleware(s.config, s.logger)
	router.Use(loggingMiddleware.Handler)

	// Rate limiting middleware
	rateLimitMiddleware := middleware.NewRateLimitMiddleware(s.config, s.logger)
	router.Use(rateLimitMiddleware.Handler)

	// Gzip compression middleware
	gzipMiddleware := middleware.NewGzipMiddleware(s.config, s.logger)
	router.Use(gzipMiddleware.Handler)

	// Authentication middleware (if auth key is configured)
	if s.config.AuthKey != "" {
		authMiddleware := middleware.NewAuthMiddleware(s.config, s.logger)
		router.Use(authMiddleware.Handler)
	}
}

// setupRoutes configures API routes
func (s *Server) setupRoutes(router *mux.Router) {
	// API routes FIRST (more specific routes)
	// API routes with /api prefix to avoid conflicts
	apiRouter := router.PathPrefix("/api").Subrouter()
	
	// Tavily API endpoints
	apiRouter.HandleFunc("/search", s.handler.TavilySearchHandler).Methods("POST")
	apiRouter.HandleFunc("/extract", s.handler.TavilyExtractHandler).Methods("POST")
	apiRouter.HandleFunc("/crawl", s.handler.TavilyCrawlHandler).Methods("POST")
	apiRouter.HandleFunc("/map", s.handler.TavilyMapHandler).Methods("POST")
	apiRouter.HandleFunc("/usage", s.handler.TavilyUsageHandler).Methods("GET")

	// Management endpoints
	apiRouter.HandleFunc("/health", s.handler.HealthHandler).Methods("GET")
	apiRouter.HandleFunc("/stats", s.handler.StatsHandler).Methods("GET")
	apiRouter.HandleFunc("/blacklist", s.handler.BlacklistHandler).Methods("GET")
	apiRouter.HandleFunc("/reset-keys", s.handler.ResetKeysHandler).Methods("GET")

	// Usage and strategy endpoints
	apiRouter.HandleFunc("/usage-analytics", s.handler.UsageAnalyticsHandler).Methods("GET")
	apiRouter.HandleFunc("/update-usage", s.handler.UpdateUsageHandler).Methods("POST")
	apiRouter.HandleFunc("/strategy", s.handler.StrategyHandler).Methods("GET", "POST")

	// Key management endpoints
	apiRouter.HandleFunc("/keys", s.handler.KeysHandler).Methods("GET", "POST", "DELETE")
	apiRouter.HandleFunc("/keys/bulk-import", s.handler.BulkImportKeysHandler).Methods("POST")
	apiRouter.HandleFunc("/keys/upload", s.handler.FileUploadKeysHandler).Methods("POST")

	// Legacy API endpoints (without /api prefix for backward compatibility)
	router.HandleFunc("/search", s.handler.TavilySearchHandler).Methods("POST")
	router.HandleFunc("/extract", s.handler.TavilyExtractHandler).Methods("POST")
	router.HandleFunc("/crawl", s.handler.TavilyCrawlHandler).Methods("POST")
	router.HandleFunc("/map", s.handler.TavilyMapHandler).Methods("POST")
	router.HandleFunc("/usage", s.handler.TavilyUsageHandler).Methods("GET")
	router.HandleFunc("/health", s.handler.HealthHandler).Methods("GET")
	router.HandleFunc("/stats", s.handler.StatsHandler).Methods("GET")
	router.HandleFunc("/blacklist", s.handler.BlacklistHandler).Methods("GET")
	router.HandleFunc("/reset-keys", s.handler.ResetKeysHandler).Methods("GET")
	router.HandleFunc("/usage-analytics", s.handler.UsageAnalyticsHandler).Methods("GET")
	router.HandleFunc("/update-usage", s.handler.UpdateUsageHandler).Methods("POST")
	router.HandleFunc("/strategy", s.handler.StrategyHandler).Methods("GET", "POST")

	// Frontend routes LAST (catch-all route)
	s.setupFrontendRoutes(router)
}

// setupFrontendRoutes configures frontend static file serving
func (s *Server) setupFrontendRoutes(router *mux.Router) {
	// Check if web build directory exists
	webDir := "./web/out"
	if _, err := http.Dir(webDir).Open("/"); err != nil {
		// Fallback to development mode or disable frontend
		s.logger.Warn("Frontend build directory not found, serving API only")
		return
	}

	// Serve static files
	fs := http.FileServer(http.Dir(webDir))
	
	// Handle SPA routing - serve index.html for non-API routes
	router.PathPrefix("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if it's an API route
		if r.URL.Path == "/" || (!fileExists(filepath.Join(webDir, r.URL.Path)) && !isAPIRoute(r.URL.Path)) {
			// Serve index.html for SPA routing
			http.ServeFile(w, r, filepath.Join(webDir, "index.html"))
			return
		}
		// Serve static file
		fs.ServeHTTP(w, r)
	})
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	if _, err := http.Dir(".").Open(path); err != nil {
		return false
	}
	return true
}

// isAPIRoute checks if the path is an API route
func isAPIRoute(path string) bool {
	apiPaths := []string{
		"/api/", "/search", "/extract", "/crawl", "/map", "/usage",
		"/health", "/stats", "/blacklist", "/reset-keys", 
		"/usage-analytics", "/update-usage", "/strategy",
	}
	
	for _, apiPath := range apiPaths {
		if len(path) >= len(apiPath) && path[:len(apiPath)] == apiPath {
			return true
		}
	}
	return false
}

// rootHandler handles requests to the root endpoint
func (s *Server) rootHandler(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"service":     "tavily-load",
		"version":     "1.0.0",
		"description": "High-performance proxy server for Tavily API with multi-key rotation and load balancing",
		"status":      "running",
		"uptime":      time.Since(s.startTime).String(),
		"endpoints": map[string]string{
			"POST /search":         "Tavily Search API",
			"POST /extract":        "Tavily Extract API",
			"POST /crawl":          "Tavily Crawl API (BETA)",
			"POST /map":            "Tavily Map API (BETA)",
			"GET /usage":           "Tavily Usage API",
			"GET /health":          "Health check",
			"GET /stats":           "Statistics",
			"GET /blacklist":       "Blacklisted keys",
			"GET /reset-keys":      "Reset all keys",
			"GET /usage-analytics": "Usage analytics and insights",
			"POST /update-usage":   "Update usage from Tavily API",
			"GET /strategy":        "Get current selection strategy",
			"POST /strategy":       "Set selection strategy",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.WithError(err).Error("Failed to encode root response")
	}
}

// Start starts the proxy server
func (s *Server) Start() error {
	s.logger.WithFields(logrus.Fields{
		"address": s.httpServer.Addr,
		"version": "1.0.0",
	}).Info("Starting Tavily Load Balancer")

	// Log configuration summary
	keyStats := s.keyManager.GetStats()
	s.logger.WithFields(logrus.Fields{
		"total_keys":              keyStats.TotalKeys,
		"tavily_base_url":         s.config.TavilyBaseURL,
		"max_retries":             s.config.MaxRetries,
		"blacklist_threshold":     s.config.BlacklistThreshold,
		"max_concurrent_requests": s.config.MaxConcurrentRequests,
		"cors_enabled":            s.config.EnableCORS,
		"gzip_enabled":            s.config.EnableGzip,
		"auth_enabled":            s.config.AuthKey != "",
	}).Info("Server configuration")

	// Start server
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start server: %w", err)
	}

	return nil
}

// Stop gracefully stops the proxy server
func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info("Shutting down server...")

	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(ctx, s.config.ServerGracefulShutdownTimeout)
	defer cancel()

	// Shutdown server
	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		s.logger.WithError(err).Error("Server shutdown failed")
		return err
	}

	s.logger.Info("Server shutdown complete")
	return nil
}

// Health returns the current health status
func (s *Server) Health() types.HealthStatus {
	keyStats := s.keyManager.GetStats()

	status := "healthy"
	if keyStats.ActiveKeys == 0 {
		status = "unhealthy"
	}

	return types.HealthStatus{
		Status:    status,
		Timestamp: time.Now(),
		Version:   "1.0.0",
		Uptime:    time.Since(s.startTime),
		KeyManager: types.KeyManagerHealth{
			TotalKeys:       keyStats.TotalKeys,
			ActiveKeys:      keyStats.ActiveKeys,
			BlacklistedKeys: keyStats.BlacklistedKeys,
		},
		Server: types.ServerHealth{
			RequestsTotal:   0, // TODO: get from handler stats
			RequestsSuccess: 0,
			RequestsError:   0,
			AverageLatency:  0,
		},
		Connections: types.ConnectionHealth{
			ActiveConnections: 0,
			TotalConnections:  0,
		},
	}
}
