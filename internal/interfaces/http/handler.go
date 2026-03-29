// Package httpapi exposes the HTTP adapter for DeltaScope.
// input: HTTP requests carrying SQL audit payloads plus service-level config/version wiring
// output: JSON audit, health, version, and structured error responses
// pos: interface adapter between net/http and the public DeltaScope audit API
// note: if this file changes, update this header and module README.md.
package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/time/rate"

	appaudit "github.com/Fanduzi/DeltaScope/internal/application/audit"
	apppolicy "github.com/Fanduzi/DeltaScope/internal/application/policy"
	"github.com/Fanduzi/DeltaScope/pkg/deltascope"
)

type auditRequest struct {
	SQL     string             `json:"sql"`
	Dialect deltascope.Dialect `json:"dialect,omitempty"`
}

type errorEnvelope struct {
	Error serviceError `json:"error"`
}

type serviceError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

const maxAuditRequestBodyBytes = 1 << 20

// AuthConfig controls API-key authentication on the HTTP adapter.
type AuthConfig struct {
	Enabled    bool
	Keys       []string
	AllowPaths []string
}

type handlerOptions struct {
	auth           AuthConfig
	requestTimeout time.Duration
	logger         *log.Logger
	auditFn        func(context.Context, deltascope.Request) (deltascope.Result, error)
	rateLimit      RateLimitConfig
	metricsEnabled bool
	trustedProxies []string
}

// HandlerOption configures NewHandler behavior.
type HandlerOption func(*handlerOptions)

// WithAuthConfig enables API-key authentication on selected routes.
func WithAuthConfig(cfg AuthConfig) HandlerOption {
	return func(options *handlerOptions) {
		options.auth = cfg
	}
}

// MiddlewareConfig controls default middleware behavior.
type MiddlewareConfig struct {
	RequestTimeout time.Duration
	Logger         *log.Logger
	RateLimit      RateLimitConfig
	MetricsEnabled *bool
	TrustedProxies []string
}

// RateLimitConfig controls per-key request throttling.
type RateLimitConfig struct {
	Enabled    bool
	RPS        float64
	Burst      int
	KeyBy      string
	AllowPaths []string
}

// WithMiddlewareConfig overrides default middleware settings.
func WithMiddlewareConfig(cfg MiddlewareConfig) HandlerOption {
	return func(options *handlerOptions) {
		if cfg.RequestTimeout > 0 {
			options.requestTimeout = cfg.RequestTimeout
		}
		if cfg.Logger != nil {
			options.logger = cfg.Logger
		}
		options.rateLimit = cfg.RateLimit
		if cfg.MetricsEnabled != nil {
			options.metricsEnabled = *cfg.MetricsEnabled
		}
		if cfg.TrustedProxies != nil {
			options.trustedProxies = cfg.TrustedProxies
		}
	}
}

// WithAuditFunc overrides the audit execution function (primarily for tests).
func WithAuditFunc(fn func(context.Context, deltascope.Request) (deltascope.Result, error)) HandlerOption {
	return func(options *handlerOptions) {
		if fn != nil {
			options.auditFn = fn
		}
	}
}

// NewHandler returns the JSON HTTP adapter for DeltaScope.
func NewHandler(configPath, version string, opts ...HandlerOption) (http.Handler, error) {
	if configPath != "" {
		if _, err := apppolicy.Load(configPath); err != nil {
			return nil, err
		}
	}

	options := handlerOptions{
		requestTimeout: 30 * time.Second,
		logger:         log.Default(),
		auditFn:        deltascope.Audit,
		metricsEnabled: true,
		trustedProxies: []string{},
	}
	for _, option := range opts {
		option(&options)
	}

	router := gin.New()
	if err := router.SetTrustedProxies(options.trustedProxies); err != nil {
		return nil, err
	}
	metricsMiddleware, metricsHandler := newMetricsMiddleware()
	router.Use(
		requestIDMiddleware(),
		recoveryMiddleware(options.logger),
		timeoutMiddleware(options.requestTimeout),
		metricsMiddleware,
		authMiddleware(options.auth),
		rateLimitMiddleware(options.rateLimit),
		accessLogMiddleware(options.logger),
	)

	router.GET("/healthz", func(c *gin.Context) {
		writeJSON(c.Writer, http.StatusOK, map[string]string{"status": "ok"})
	})
	router.GET("/readyz", func(c *gin.Context) {
		writeJSON(c.Writer, http.StatusOK, map[string]string{"status": "ready"})
	})
	router.GET("/version", func(c *gin.Context) {
		writeJSON(c.Writer, http.StatusOK, map[string]string{"version": version})
	})
	router.POST("/v1/audit", func(c *gin.Context) {
		handleAudit(c.Writer, c.Request, configPath, options.auditFn)
	})
	if options.metricsEnabled {
		router.GET("/metrics", gin.WrapH(metricsHandler))
	}

	return router, nil
}

func newMetricsMiddleware() (gin.HandlerFunc, http.Handler) {
	registry := prometheus.NewRegistry()
	requestCount := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "deltascope_http_requests_total",
			Help: "Total HTTP requests handled by DeltaScope HTTP adapter.",
		},
		[]string{"method", "path", "status"},
	)
	requestDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "deltascope_http_request_duration_seconds",
			Help:    "HTTP request latency distribution in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "status"},
	)
	registry.MustRegister(requestCount, requestDuration)

	middleware := func(c *gin.Context) {
		start := time.Now()
		c.Next()

		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		status := http.StatusText(c.Writer.Status())
		if status == "" {
			status = "unknown"
		}
		statusCode := http.StatusText(c.Writer.Status())
		if statusCode == "" {
			statusCode = "unknown"
		}
		statusLabel := strings.ToLower(strings.ReplaceAll(statusCode, " ", "_"))
		requestCount.WithLabelValues(c.Request.Method, path, statusLabel).Inc()
		requestDuration.WithLabelValues(c.Request.Method, path, statusLabel).Observe(time.Since(start).Seconds())
	}

	return middleware, promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
}

func requestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := strings.TrimSpace(c.GetHeader("X-Request-ID"))
		if requestID == "" {
			requestID = newRequestID()
		}
		c.Writer.Header().Set("X-Request-ID", requestID)
		c.Next()
	}
}

func recoveryMiddleware(logger *log.Logger) gin.HandlerFunc {
	if logger == nil {
		logger = log.Default()
	}
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		logger.Printf("http panic recovered method=%s path=%s panic=%v", c.Request.Method, c.Request.URL.Path, recovered)
		writeError(c.Writer, http.StatusInternalServerError, "internal_error", "internal server error")
	})
}

func timeoutMiddleware(timeout time.Duration) gin.HandlerFunc {
	if timeout <= 0 {
		return func(c *gin.Context) {
			c.Next()
		}
	}
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// accessLogEntry is the structured JSON shape emitted by accessLogMiddleware.
type accessLogEntry struct {
	Timestamp  string `json:"timestamp"`
	Level      string `json:"level"`
	Msg        string `json:"msg"`
	Method     string `json:"method"`
	Path       string `json:"path"`
	Status     int    `json:"status"`
	DurationMs int64  `json:"duration_ms"`
	RequestID  string `json:"request_id"`
}

func accessLogMiddleware(logger *log.Logger) gin.HandlerFunc {
	if logger == nil {
		logger = log.Default()
	}
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		requestID := c.Writer.Header().Get("X-Request-ID")
		entry := accessLogEntry{
			Timestamp:  time.Now().UTC().Format(time.RFC3339),
			Level:      "info",
			Msg:        "http request",
			Method:     c.Request.Method,
			Path:       c.Request.URL.Path,
			Status:     c.Writer.Status(),
			DurationMs: time.Since(start).Milliseconds(),
			RequestID:  requestID,
		}
		b, err := json.Marshal(entry)
		if err != nil {
			logger.Printf("access log marshal error: %v", err)
			return
		}
		logger.Println(string(b))
	}
}

func authMiddleware(cfg AuthConfig) gin.HandlerFunc {
	if !cfg.Enabled {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	allowPaths := make(map[string]struct{}, len(cfg.AllowPaths))
	for _, path := range cfg.AllowPaths {
		allowPaths[path] = struct{}{}
	}
	validKeys := make(map[string]struct{}, len(cfg.Keys))
	for _, key := range cfg.Keys {
		validKeys[key] = struct{}{}
	}

	return func(c *gin.Context) {
		if _, ok := allowPaths[c.Request.URL.Path]; ok {
			c.Next()
			return
		}

		providedKey := c.GetHeader("X-API-Key")
		if providedKey == "" {
			writeError(c.Writer, http.StatusUnauthorized, "auth_required", "X-API-Key header is required")
			c.Abort()
			return
		}
		if _, ok := validKeys[providedKey]; !ok {
			writeError(c.Writer, http.StatusForbidden, "auth_invalid", "invalid API key")
			c.Abort()
			return
		}

		c.Next()
	}
}

func rateLimitMiddleware(cfg RateLimitConfig) gin.HandlerFunc {
	if !cfg.Enabled {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	rps := cfg.RPS
	if rps <= 0 {
		rps = 5
	}
	burst := cfg.Burst
	if burst <= 0 {
		burst = 10
	}
	keyBy := strings.ToLower(strings.TrimSpace(cfg.KeyBy))
	if keyBy == "" {
		keyBy = "api-key"
	}

	allowPaths := make(map[string]struct{}, len(cfg.AllowPaths))
	for _, path := range cfg.AllowPaths {
		allowPaths[path] = struct{}{}
	}
	limiters := newLimiterStore(rate.Limit(rps), burst)

	return func(c *gin.Context) {
		if _, ok := allowPaths[c.Request.URL.Path]; ok {
			c.Next()
			return
		}

		key := requestRateLimitKey(c, keyBy)
		if !limiters.Allow(key) {
			c.Writer.Header().Set("Retry-After", "1")
			writeError(c.Writer, http.StatusTooManyRequests, "rate_limited", "rate limit exceeded")
			c.Abort()
			return
		}
		c.Next()
	}
}

func requestRateLimitKey(c *gin.Context, keyBy string) string {
	switch keyBy {
	case "ip":
		return clientIPFromContext(c)
	default:
		apiKey := strings.TrimSpace(c.GetHeader("X-API-Key"))
		if apiKey == "" {
			return "anon:" + clientIPFromContext(c)
		}
		return "api-key:" + apiKey
	}
}

func clientIPFromContext(c *gin.Context) string {
	if c != nil {
		if ip := strings.TrimSpace(c.ClientIP()); ip != "" {
			return ip
		}
	}
	return "unknown"
}

type limiterStore struct {
	mu              sync.Mutex
	limit           rate.Limit
	burst           int
	entries         map[string]limiterEntry
	ttl             time.Duration
	cleanupInterval time.Duration
	nextCleanup     time.Time
	callCount       uint64
}

type limiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func newLimiterStore(limit rate.Limit, burst int) *limiterStore {
	now := time.Now()
	return &limiterStore{
		limit:           limit,
		burst:           burst,
		entries:         make(map[string]limiterEntry),
		ttl:             10 * time.Minute,
		cleanupInterval: time.Minute,
		nextCleanup:     now.Add(time.Minute),
	}
}

func (s *limiterStore) Allow(key string) bool {
	now := time.Now()

	s.mu.Lock()
	entry, ok := s.entries[key]
	if !ok {
		entry = limiterEntry{
			limiter:  rate.NewLimiter(s.limit, s.burst),
			lastSeen: now,
		}
	} else {
		entry.lastSeen = now
	}
	s.entries[key] = entry

	s.callCount++
	if now.After(s.nextCleanup) || s.callCount%1024 == 0 {
		s.cleanupExpiredLocked(now)
	}

	s.mu.Unlock()
	return entry.limiter.Allow()
}

func (s *limiterStore) cleanupExpiredLocked(now time.Time) {
	expireBefore := now.Add(-s.ttl)
	for key, entry := range s.entries {
		if entry.lastSeen.Before(expireBefore) {
			delete(s.entries, key)
		}
	}
	s.nextCleanup = now.Add(s.cleanupInterval)
}

func handleAudit(
	w http.ResponseWriter,
	r *http.Request,
	configPath string,
	auditFn func(context.Context, deltascope.Request) (deltascope.Result, error),
) {
	defer r.Body.Close()

	r.Body = http.MaxBytesReader(w, r.Body, maxAuditRequestBodyBytes)

	var request auditRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	if err := decoder.Decode(&struct{}{}); err == nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must contain exactly one JSON object")
		return
	} else if !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must contain exactly one JSON object")
		return
	}

	type auditOutput struct {
		result deltascope.Result
		err    error
	}
	resultChan := make(chan auditOutput, 1)
	go func() {
		result, err := auditFn(r.Context(), deltascope.Request{
			SQL:        request.SQL,
			Dialect:    request.Dialect,
			ConfigPath: configPath,
		})
		resultChan <- auditOutput{result: result, err: err}
	}()

	var (
		result deltascope.Result
		err    error
	)
	select {
	case <-r.Context().Done():
		err = r.Context().Err()
	case out := <-resultChan:
		result = out.result
		err = out.err
	}
	if err != nil {
		status, code := mapAuditError(err)
		writeError(w, status, code, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func mapAuditError(err error) (status int, code string) {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return http.StatusGatewayTimeout, "request_timeout"
	case errors.Is(err, context.Canceled):
		return http.StatusRequestTimeout, "request_canceled"
	case errors.Is(err, appaudit.ErrEmptySQL), errors.Is(err, appaudit.ErrUnknownDialect):
		return http.StatusBadRequest, "bad_request"
	case strings.Contains(err.Error(), "load policy:"):
		return http.StatusInternalServerError, "config_invalid"
	default:
		return http.StatusBadRequest, "bad_request"
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorEnvelope{
		Error: serviceError{
			Code:    code,
			Message: message,
		},
	})
}

func newRequestID() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "req-fallback"
	}
	return "req-" + hex.EncodeToString(b[:])
}
