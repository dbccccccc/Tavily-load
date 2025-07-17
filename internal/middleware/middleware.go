package middleware

import (
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/dbccccccc/tavily-load/internal/config"
	"github.com/dbccccccc/tavily-load/pkg/types"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

// RequestIDKey is the context key for request ID
type RequestIDKey struct{}

// RequestContextKey is the context key for request context
type RequestContextKey struct{}

// AuthMiddleware handles authentication
type AuthMiddleware struct {
	authKey string
	logger  *logrus.Logger
}

// NewAuthMiddleware creates a new auth middleware
func NewAuthMiddleware(cfg *config.Config, logger *logrus.Logger) *AuthMiddleware {
	return &AuthMiddleware{
		authKey: cfg.AuthKey,
		logger:  logger,
	}
}

// Handler implements the middleware interface
func (m *AuthMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth if no auth key is configured
		if m.authKey == "" {
			next.ServeHTTP(w, r)
			return
		}

		// Check Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		// Extract Bearer token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, "Invalid authorization header format", http.StatusUnauthorized)
			return
		}

		token := parts[1]
		if token != m.authKey {
			http.Error(w, "Invalid authorization token", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// RequestIDMiddleware adds a unique request ID to each request
type RequestIDMiddleware struct {
	logger *logrus.Logger
}

// NewRequestIDMiddleware creates a new request ID middleware
func NewRequestIDMiddleware(logger *logrus.Logger) *RequestIDMiddleware {
	return &RequestIDMiddleware{
		logger: logger,
	}
}

// Handler implements the middleware interface
func (m *RequestIDMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := uuid.New().String()

		// Add request ID to context
		ctx := context.WithValue(r.Context(), RequestIDKey{}, requestID)

		// Add request ID to response headers
		w.Header().Set("X-Request-ID", requestID)

		// Create request context
		reqCtx := &types.RequestContext{
			RequestID: requestID,
			StartTime: time.Now(),
			Method:    r.Method,
			Endpoint:  r.URL.Path,
			ClientIP:  getClientIP(r),
			UserAgent: r.Header.Get("User-Agent"),
		}

		ctx = context.WithValue(ctx, RequestContextKey{}, reqCtx)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// LoggingMiddleware logs HTTP requests
type LoggingMiddleware struct {
	logger        *logrus.Logger
	enableLogging bool
}

// NewLoggingMiddleware creates a new logging middleware
func NewLoggingMiddleware(cfg *config.Config, logger *logrus.Logger) *LoggingMiddleware {
	return &LoggingMiddleware{
		logger:        logger,
		enableLogging: cfg.LogEnableRequest,
	}
}

// Handler implements the middleware interface
func (m *LoggingMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !m.enableLogging {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()

		// Wrap response writer to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start)

		// Get request context
		requestID := ""
		if id := r.Context().Value(RequestIDKey{}); id != nil {
			requestID = id.(string)
		}

		m.logger.WithFields(logrus.Fields{
			"request_id": requestID,
			"method":     r.Method,
			"path":       r.URL.Path,
			"status":     wrapped.statusCode,
			"duration":   duration,
			"client_ip":  getClientIP(r),
			"user_agent": r.Header.Get("User-Agent"),
		}).Info("HTTP request")
	})
}

// RateLimitMiddleware implements rate limiting
type RateLimitMiddleware struct {
	limiter *rate.Limiter
	logger  *logrus.Logger
}

// NewRateLimitMiddleware creates a new rate limit middleware
func NewRateLimitMiddleware(cfg *config.Config, logger *logrus.Logger) *RateLimitMiddleware {
	// Create a rate limiter based on max concurrent requests
	// Allow burst of max concurrent requests, refill at 1/10 of that rate per second
	limit := rate.Limit(float64(cfg.MaxConcurrentRequests) / 10.0)
	limiter := rate.NewLimiter(limit, cfg.MaxConcurrentRequests)

	return &RateLimitMiddleware{
		limiter: limiter,
		logger:  logger,
	}
}

// Handler implements the middleware interface
func (m *RateLimitMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !m.limiter.Allow() {
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// GzipMiddleware handles gzip compression
type GzipMiddleware struct {
	enabled bool
	logger  *logrus.Logger
}

// NewGzipMiddleware creates a new gzip middleware
func NewGzipMiddleware(cfg *config.Config, logger *logrus.Logger) *GzipMiddleware {
	return &GzipMiddleware{
		enabled: cfg.EnableGzip,
		logger:  logger,
	}
}

// Handler implements the middleware interface
func (m *GzipMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !m.enabled {
			next.ServeHTTP(w, r)
			return
		}

		// Check if client accepts gzip
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		// Set gzip headers
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Vary", "Accept-Encoding")

		// Create gzip writer
		gz := gzip.NewWriter(w)
		defer gz.Close()

		// Wrap response writer
		gzw := &gzipResponseWriter{ResponseWriter: w, Writer: gz}
		next.ServeHTTP(gzw, r)
	})
}

// RecoveryMiddleware handles panics
type RecoveryMiddleware struct {
	logger *logrus.Logger
}

// NewRecoveryMiddleware creates a new recovery middleware
func NewRecoveryMiddleware(logger *logrus.Logger) *RecoveryMiddleware {
	return &RecoveryMiddleware{
		logger: logger,
	}
}

// Handler implements the middleware interface
func (m *RecoveryMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				requestID := ""
				if id := r.Context().Value(RequestIDKey{}); id != nil {
					requestID = id.(string)
				}

				m.logger.WithFields(logrus.Fields{
					"request_id": requestID,
					"method":     r.Method,
					"path":       r.URL.Path,
					"panic":      err,
				}).Error("Panic recovered")

				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, r)
	})
}

// Helper types and functions

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

type gzipResponseWriter struct {
	http.ResponseWriter
	io.Writer
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0])
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to remote address
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}

	return ip
}
