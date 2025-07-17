package handler

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/dbccccccc/tavily-load/internal/config"
	"github.com/dbccccccc/tavily-load/internal/errors"
	"github.com/dbccccccc/tavily-load/internal/keymanager"
	"github.com/dbccccccc/tavily-load/internal/middleware"
	"github.com/dbccccccc/tavily-load/internal/repository"
	"github.com/dbccccccc/tavily-load/pkg/types"
	"github.com/sirupsen/logrus"
)

// Handler manages HTTP requests for the Tavily API proxy
type Handler struct {
	keyManager *keymanager.Manager
	config     *config.Config
	logger     *logrus.Logger
	httpClient *http.Client
	startTime  time.Time
	stats      *Stats
	keyRepo    *repository.KeyRepository
}

// Stats tracks request statistics
type Stats struct {
	RequestsTotal   int64         `json:"requests_total"`
	RequestsSuccess int64         `json:"requests_success"`
	RequestsError   int64         `json:"requests_error"`
	AverageLatency  time.Duration `json:"average_latency"`
	TotalLatency    time.Duration `json:"total_latency"`
}

// NewHandler creates a new HTTP handler
func NewHandler(keyManager *keymanager.Manager, cfg *config.Config, logger *logrus.Logger, keyRepo *repository.KeyRepository) *Handler {
	// Create HTTP client with timeouts
	client := &http.Client{
		Timeout: cfg.RequestTimeout,
		Transport: &http.Transport{
			IdleConnTimeout:       cfg.IdleConnTimeout,
			ResponseHeaderTimeout: cfg.ResponseTimeout,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   10,
		},
	}

	return &Handler{
		keyManager: keyManager,
		config:     cfg,
		logger:     logger,
		httpClient: client,
		startTime:  time.Now(),
		stats:      &Stats{},
		keyRepo:    keyRepo,
	}
}

// TavilySearchHandler handles POST /search requests
func (h *Handler) TavilySearchHandler(w http.ResponseWriter, r *http.Request) {
	h.proxyTavilyRequest(w, r, "/search")
}

// TavilyExtractHandler handles POST /extract requests
func (h *Handler) TavilyExtractHandler(w http.ResponseWriter, r *http.Request) {
	h.proxyTavilyRequest(w, r, "/extract")
}

// TavilyCrawlHandler handles POST /crawl requests
func (h *Handler) TavilyCrawlHandler(w http.ResponseWriter, r *http.Request) {
	h.proxyTavilyRequest(w, r, "/crawl")
}

// TavilyMapHandler handles POST /map requests
func (h *Handler) TavilyMapHandler(w http.ResponseWriter, r *http.Request) {
	h.proxyTavilyRequest(w, r, "/map")
}

// TavilyUsageHandler handles GET /usage requests
func (h *Handler) TavilyUsageHandler(w http.ResponseWriter, r *http.Request) {
	h.proxyTavilyRequest(w, r, "/usage")
}

// proxyTavilyRequest proxies requests to the Tavily API with key rotation
func (h *Handler) proxyTavilyRequest(w http.ResponseWriter, r *http.Request, endpoint string) {
	startTime := time.Now()
	h.stats.RequestsTotal++

	// Get request context
	reqCtx := h.getRequestContext(r)
	reqCtx.Endpoint = endpoint

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.WithError(err).Error("Failed to read request body")
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		h.stats.RequestsError++
		return
	}
	defer r.Body.Close()

	// Try request with retries
	var lastErr error
	for attempt := 0; attempt <= h.config.MaxRetries; attempt++ {
		reqCtx.RetryCount = attempt

		// Get next API key
		apiKey, err := h.keyManager.GetNextKey()
		if err != nil {
			h.logger.WithError(err).Error("Failed to get API key")
			http.Error(w, "No API keys available", http.StatusServiceUnavailable)
			h.stats.RequestsError++
			return
		}

		reqCtx.Key = apiKey

		// Make request to Tavily API
		resp, err := h.makeRequest(r.Context(), r.Method, endpoint, apiKey, body, r.Header)
		if err != nil {
			lastErr = err
			h.keyManager.RecordError(apiKey, err)

			// Update usage tracker metrics for failed request
			if usageTracker := h.getUsageTracker(); usageTracker != nil {
				usageTracker.UpdateKeyMetrics(apiKey, false, time.Since(startTime))
			}

			// Check if we should retry
			if tavilyErr, ok := err.(*errors.TavilyError); ok && !tavilyErr.IsRetryable() {
				break
			}

			h.logger.WithError(err).
				WithField("attempt", attempt+1).
				WithField("key", apiKey[:12]+"...").
				Warn("Request failed, retrying with different key")
			continue
		}

		// Success - copy response
		h.copyResponse(w, resp)
		h.stats.RequestsSuccess++

		// Update latency stats
		latency := time.Since(startTime)
		h.stats.TotalLatency += latency
		if h.stats.RequestsTotal > 0 {
			h.stats.AverageLatency = h.stats.TotalLatency / time.Duration(h.stats.RequestsTotal)
		}

		reqCtx.ResponseTime = latency

		// Update usage tracker metrics
		if usageTracker := h.getUsageTracker(); usageTracker != nil {
			usageTracker.UpdateKeyMetrics(apiKey, true, latency)
		}

		h.logger.WithFields(logrus.Fields{
			"endpoint":      endpoint,
			"key":           apiKey[:12] + "...",
			"attempt":       attempt + 1,
			"response_time": latency,
			"status":        resp.StatusCode,
		}).Info("Request successful")

		return
	}

	// All retries failed
	h.stats.RequestsError++
	h.logger.WithError(lastErr).Error("All retries failed")

	if tavilyErr, ok := lastErr.(*errors.TavilyError); ok {
		http.Error(w, tavilyErr.Message, tavilyErr.StatusCode)
	} else {
		http.Error(w, "Request failed after all retries", http.StatusInternalServerError)
	}
}

// makeRequest makes a request to the Tavily API
func (h *Handler) makeRequest(ctx context.Context, method, endpoint, apiKey string, body []byte, headers http.Header) (*http.Response, error) {
	url := h.config.TavilyBaseURL + endpoint

	// Create request
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, errors.NewTavilyError(errors.ErrorTypeInternalError, "Failed to create request", 500)
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "tavily-load/1.0")

	// Copy relevant headers from original request
	for key, values := range headers {
		if shouldCopyHeader(key) {
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}
	}

	// Make request
	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, errors.NewTavilyErrorWithKey(errors.ErrorTypeNetworkError, "Network error: "+err.Error(), 500, apiKey)
	}

	// Check for HTTP errors
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, errors.ParseHTTPError(resp.StatusCode, body, apiKey)
	}

	return resp, nil
}

// copyResponse copies the response from Tavily API to the client
func (h *Handler) copyResponse(w http.ResponseWriter, resp *http.Response) {
	defer resp.Body.Close()

	// Copy headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Set status code
	w.WriteHeader(resp.StatusCode)

	// Copy body
	io.Copy(w, resp.Body)
}

// shouldCopyHeader determines if a header should be copied to the upstream request
func shouldCopyHeader(header string) bool {
	header = strings.ToLower(header)

	// Headers to skip
	skipHeaders := []string{
		"authorization",
		"host",
		"content-length",
		"connection",
		"upgrade",
		"proxy-connection",
		"proxy-authenticate",
		"proxy-authorization",
		"te",
		"trailers",
		"transfer-encoding",
	}

	for _, skip := range skipHeaders {
		if header == skip {
			return false
		}
	}

	return true
}

// getRequestContext extracts request context from the request
func (h *Handler) getRequestContext(r *http.Request) *types.RequestContext {
	if ctx := r.Context().Value(middleware.RequestContextKey{}); ctx != nil {
		return ctx.(*types.RequestContext)
	}

	// Fallback if middleware didn't set context
	return &types.RequestContext{
		RequestID: "unknown",
		StartTime: time.Now(),
		Method:    r.Method,
		ClientIP:  r.RemoteAddr,
		UserAgent: r.Header.Get("User-Agent"),
	}
}

// HealthHandler handles GET /health requests
func (h *Handler) HealthHandler(w http.ResponseWriter, r *http.Request) {
	keyStats := h.keyManager.GetStats()

	health := types.HealthStatus{
		Status:    "healthy",
		Timestamp: time.Now(),
		Version:   "1.0.0",
		Uptime:    time.Since(h.startTime),
		KeyManager: types.KeyManagerHealth{
			TotalKeys:       keyStats.TotalKeys,
			ActiveKeys:      keyStats.ActiveKeys,
			BlacklistedKeys: keyStats.BlacklistedKeys,
		},
		Server: types.ServerHealth{
			RequestsTotal:   h.stats.RequestsTotal,
			RequestsSuccess: h.stats.RequestsSuccess,
			RequestsError:   h.stats.RequestsError,
			AverageLatency:  h.stats.AverageLatency,
		},
		Connections: types.ConnectionHealth{
			ActiveConnections: 0, // TODO: implement connection tracking
			TotalConnections:  0,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

// StatsHandler handles GET /stats requests
func (h *Handler) StatsHandler(w http.ResponseWriter, r *http.Request) {
	stats := h.keyManager.GetStats()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// BlacklistHandler handles GET /blacklist requests
func (h *Handler) BlacklistHandler(w http.ResponseWriter, r *http.Request) {
	blacklist := h.keyManager.GetBlacklist()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"blacklisted_keys": blacklist,
		"count":            len(blacklist),
	})
}

// ResetKeysHandler handles GET /reset-keys requests
func (h *Handler) ResetKeysHandler(w http.ResponseWriter, r *http.Request) {
	h.keyManager.ResetKeys()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "All keys reset and blacklist cleared",
	})
}

// UsageAnalyticsHandler handles GET /usage-analytics requests
func (h *Handler) UsageAnalyticsHandler(w http.ResponseWriter, r *http.Request) {
	analytics := h.keyManager.GetUsageAnalytics()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(analytics)
}

// UpdateUsageHandler handles POST /update-usage requests
func (h *Handler) UpdateUsageHandler(w http.ResponseWriter, r *http.Request) {
	err := h.keyManager.UpdateUsageFromAPI()

	response := map[string]interface{}{
		"status":  "success",
		"message": "Usage information updated",
	}

	if err != nil {
		response["status"] = "partial"
		response["message"] = "Some keys failed to update: " + err.Error()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// StrategyHandler handles GET/POST /strategy requests
func (h *Handler) StrategyHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		h.getStrategyHandler(w, r)
	case "POST":
		h.setStrategyHandler(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) getStrategyHandler(w http.ResponseWriter, r *http.Request) {
	currentStrategy := h.keyManager.GetSelectionStrategy()
	recommendedStrategy := types.StrategyRoundRobin

	if usageTracker := h.getUsageTracker(); usageTracker != nil {
		recommendedStrategy = usageTracker.GetRecommendedStrategy()
	}

	response := map[string]interface{}{
		"current_strategy":     currentStrategy,
		"recommended_strategy": recommendedStrategy,
		"available_strategies": []types.SelectionStrategy{
			types.StrategyPlanFirst,
			types.StrategyRoundRobin,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *Handler) setStrategyHandler(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Strategy types.SelectionStrategy `json:"strategy"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate strategy
	validStrategies := map[types.SelectionStrategy]bool{
		types.StrategyPlanFirst:  true,
		types.StrategyRoundRobin: true,
	}

	if !validStrategies[request.Strategy] {
		http.Error(w, "Invalid strategy", http.StatusBadRequest)
		return
	}

	h.keyManager.SetSelectionStrategy(request.Strategy)

	response := map[string]interface{}{
		"status":   "success",
		"message":  "Selection strategy updated",
		"strategy": request.Strategy,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// getUsageTracker returns the usage tracker from the key manager
func (h *Handler) getUsageTracker() types.UsageTracker {
	// Access the usage tracker through the key manager
	return h.keyManager.GetUsageTracker()
}

// KeysHandler handles GET /api/keys requests (list all keys)
func (h *Handler) KeysHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		h.listKeysHandler(w, r)
	case "POST":
		h.addKeyHandler(w, r)
	case "DELETE":
		h.deleteKeyHandler(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// listKeysHandler handles listing all keys
func (h *Handler) listKeysHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	keys, err := h.keyRepo.GetAllKeys(ctx)
	if err != nil {
		h.logger.WithError(err).Error("Failed to fetch keys from database")
		http.Error(w, "Failed to fetch keys", http.StatusInternalServerError)
		return
	}

	// Convert to response format (without exposing full key values)
	response := make([]map[string]interface{}, len(keys))
	for i, key := range keys {
		response[i] = map[string]interface{}{
			"id":                key.ID,
			"name":              key.Name,
			"description":       key.Description,
			"key_preview":       key.KeyValue[:12] + "...",
			"is_active":         key.IsActive,
			"is_blacklisted":    key.IsBlacklisted,
			"blacklisted_until": key.BlacklistedUntil,
			"blacklist_reason":  key.BlacklistReason,
			"created_at":        key.CreatedAt,
			"updated_at":        key.UpdatedAt,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"keys":  response,
		"count": len(response),
	})
}

// addKeyHandler handles adding a single key
func (h *Handler) addKeyHandler(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Key         string `json:"key"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate key format
	if !strings.HasPrefix(request.Key, "tvly-") {
		http.Error(w, "Invalid key format: key must start with 'tvly-'", http.StatusBadRequest)
		return
	}

	if request.Key == "" {
		http.Error(w, "Key is required", http.StatusBadRequest)
		return
	}

	if request.Name == "" {
		request.Name = "API Key"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	createdKey, err := h.keyRepo.CreateKey(ctx, request.Key, request.Name, request.Description)
	if err != nil {
		if strings.Contains(err.Error(), "Duplicate entry") {
			http.Error(w, "Key already exists", http.StatusConflict)
			return
		}
		h.logger.WithError(err).Error("Failed to create key")
		http.Error(w, "Failed to create key", http.StatusInternalServerError)
		return
	}

	h.logger.WithFields(logrus.Fields{
		"key_id":   createdKey.ID,
		"key_name": createdKey.Name,
	}).Info("New API key added")

	response := map[string]interface{}{
		"status":  "success",
		"message": "API key added successfully",
		"key": map[string]interface{}{
			"id":          createdKey.ID,
			"name":        createdKey.Name,
			"description": createdKey.Description,
			"key_preview": createdKey.KeyValue[:12] + "...",
			"created_at":  createdKey.CreatedAt,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// deleteKeyHandler handles deleting a key
func (h *Handler) deleteKeyHandler(w http.ResponseWriter, r *http.Request) {
	keyID := r.URL.Query().Get("id")
	if keyID == "" {
		http.Error(w, "Key ID is required", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseInt(keyID, 10, 64)
	if err != nil {
		http.Error(w, "Invalid key ID", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get key details before deletion for logging
	key, err := h.keyRepo.GetKeyByID(ctx, id)
	if err != nil {
		http.Error(w, "Key not found", http.StatusNotFound)
		return
	}

	if err := h.keyRepo.DeleteKey(ctx, key.KeyValue); err != nil {
		h.logger.WithError(err).Error("Failed to delete key")
		http.Error(w, "Failed to delete key", http.StatusInternalServerError)
		return
	}

	h.logger.WithFields(logrus.Fields{
		"key_id":   key.ID,
		"key_name": key.Name,
	}).Info("API key deleted")

	response := map[string]interface{}{
		"status":  "success",
		"message": "API key deleted successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// BulkImportKeysHandler handles POST /api/keys/bulk-import requests
func (h *Handler) BulkImportKeysHandler(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Keys   string `json:"keys"`   // Text with keys separated by newlines
		Prefix string `json:"prefix"` // Optional prefix for naming
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if request.Keys == "" {
		http.Error(w, "Keys text is required", http.StatusBadRequest)
		return
	}

	keys := h.parseKeysFromText(request.Keys)
	if len(keys) == 0 {
		http.Error(w, "No valid keys found in the provided text", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results := h.importKeysToDatabase(ctx, keys, request.Prefix)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// FileUploadKeysHandler handles POST /api/keys/upload requests
func (h *Handler) FileUploadKeysHandler(w http.ResponseWriter, r *http.Request) {
	// Parse multipart form
	err := r.ParseMultipartForm(10 << 20) // 10 MB max
	if err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Failed to get file from form", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Validate file type
	if !strings.HasSuffix(strings.ToLower(header.Filename), ".txt") {
		http.Error(w, "Only .txt files are allowed", http.StatusBadRequest)
		return
	}

	// Read file content
	content := make([]byte, header.Size)
	_, err = file.Read(content)
	if err != nil {
		http.Error(w, "Failed to read file content", http.StatusInternalServerError)
		return
	}

	keys := h.parseKeysFromText(string(content))
	if len(keys) == 0 {
		http.Error(w, "No valid keys found in the uploaded file", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	prefix := r.FormValue("prefix")
	results := h.importKeysToDatabase(ctx, keys, prefix)

	h.logger.WithFields(logrus.Fields{
		"filename":      header.Filename,
		"keys_found":    len(keys),
		"keys_imported": results["imported_count"],
	}).Info("Keys imported from file upload")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// parseKeysFromText parses API keys from text content
func (h *Handler) parseKeysFromText(text string) []string {
	var keys []string
	scanner := bufio.NewScanner(strings.NewReader(text))
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Validate key format (should start with "tvly-")
		if !strings.HasPrefix(line, "tvly-") {
			h.logger.Warnf("Invalid key format at line %d: key should start with 'tvly-'", lineNum)
			continue
		}

		keys = append(keys, line)
	}

	return keys
}

// importKeysToDatabase imports multiple keys to the database
func (h *Handler) importKeysToDatabase(ctx context.Context, keys []string, namePrefix string) map[string]interface{} {
	imported := 0
	skipped := 0
	errors := 0
	errorDetails := []string{}

	if namePrefix == "" {
		namePrefix = "Imported Key"
	}

	for i, key := range keys {
		name := fmt.Sprintf("%s %d", namePrefix, i+1)
		description := "Imported via web interface"

		if _, err := h.keyRepo.CreateKey(ctx, key, name, description); err != nil {
			if strings.Contains(err.Error(), "Duplicate entry") {
				skipped++
				h.logger.Debugf("Key %s already exists, skipping", key[:12]+"...")
			} else {
				errors++
				errorMsg := fmt.Sprintf("Key %s: %s", key[:12]+"...", err.Error())
				errorDetails = append(errorDetails, errorMsg)
				h.logger.WithError(err).Errorf("Failed to import key %s", key[:12]+"...")
			}
			continue
		}

		imported++
		h.logger.Debugf("Imported key: %s", key[:12]+"...")
	}

	results := map[string]interface{}{
		"status":         "success",
		"total_keys":     len(keys),
		"imported_count": imported,
		"skipped_count":  skipped,
		"error_count":    errors,
	}

	if errors > 0 {
		results["errors"] = errorDetails
	}

	if imported == 0 {
		results["status"] = "warning"
		results["message"] = "No new keys were imported"
	} else {
		results["message"] = fmt.Sprintf("Successfully imported %d keys", imported)
	}

	return results
}
