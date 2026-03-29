// Package httpapi verifies HTTP request binding and response mapping.
// input: synthetic HTTP requests against the DeltaScope HTTP adapter
// output: focused coverage for health, version, audit success, and structured error responses
// pos: interface adapter test coverage for the HTTP service surface
// note: if this file changes, update this header and module README.md.
package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	appaudit "github.com/Fanduzi/DeltaScope/internal/application/audit"
	"github.com/Fanduzi/DeltaScope/pkg/deltascope"
	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

func TestHandlerHealthz(t *testing.T) {
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() == "" {
		t.Fatalf("expected health body")
	}
}

func TestHandlerVersion(t *testing.T) {
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/version", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("test-build")) {
		t.Fatalf("expected version payload, got %q", rec.Body.String())
	}
}

func TestHandlerAuditReturnsJSONResult(t *testing.T) {
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(`{"sql":"delete from users","dialect":"mysql"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["verdict"] == "" {
		t.Fatalf("expected verdict in response, got %+v", payload)
	}
}

func TestHandlerAuditReturnsFindingExplanationFields(t *testing.T) {
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(`{"sql":"delete from users","dialect":"mysql"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	finding, explanation := firstFindingByRuleID(t, payload, "dml.where.require")
	if finding["rule_id"] != "dml.where.require" {
		t.Fatalf("expected canonical where-rule finding, got %#v", finding["rule_id"])
	}
	assertExplanationFieldString(t, explanation, "summary")
	assertExplanationFieldString(t, explanation, "why")
	assertExplanationFieldString(t, explanation, "risk")
	assertExplanationFieldString(t, explanation, "suggestion")
}

func TestWriteJSONSerializesFindingExplanationMetadataFields(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusOK, deltascope.Result{
		Statements: []deltascope.StatementResult{{
			Index: 0,
			Kind:  "ddl",
			Findings: []deltascope.Finding{{
				RuleID:  "ddl.table.exists.create.forbid",
				Level:   deltascope.LevelBlocker,
				Message: "table already exists",
				Explanation: &deltascope.FindingExplanation{
					Summary:    "Forbid create existing table",
					Why:        "Live metadata says the target table already exists.",
					Risk:       "Re-running the statement can fail or mask drift.",
					Suggestion: "Use IF NOT EXISTS or reconcile with the live schema first.",
					Metadata: &deltascope.ExplanationMetadata{
						Status: "limited",
						Note:   "metadata unavailable",
					},
				},
			}},
		}},
	})

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	_, explanation := firstFindingByRuleID(t, payload, "ddl.table.exists.create.forbid")
	metadata, ok := explanation["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("expected explanation metadata object, got %#v", explanation["metadata"])
	}
	assertExplanationFieldString(t, metadata, "status")
	assertExplanationFieldString(t, metadata, "note")
}

func TestWriteJSONOmitsFindingExplanationWhenAbsent(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusOK, deltascope.Result{
		Statements: []deltascope.StatementResult{{
			Index: 0,
			Kind:  "dml",
			Findings: []deltascope.Finding{{
				RuleID:  "custom.rule",
				Level:   deltascope.LevelWarning,
				Message: "custom finding",
			}},
		}},
	})

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	finding := firstFindingWithoutExplanationByRuleID(t, payload, "custom.rule")
	if _, ok := finding["explanation"]; ok {
		t.Fatalf("expected explanation to be omitted, got %#v", finding["explanation"])
	}
}

func TestHandlerAuditRejectsInvalidJSON(t *testing.T) {
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(`{"sql":`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"invalid_json"`)) {
		t.Fatalf("expected invalid_json code, got %q", rec.Body.String())
	}
}

func TestHandlerAuditRejectsEmptySQL(t *testing.T) {
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(`{"sql":"   "}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"bad_request"`)) {
		t.Fatalf("expected bad_request code, got %q", rec.Body.String())
	}
}

func TestHandlerAuditRejectsOversizedRequestBody(t *testing.T) {
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	oversizedSQL := strings.Repeat("a", 1<<20)
	body := bytes.NewBufferString(`{"sql":"` + oversizedSQL + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/audit", body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for oversized body, got %d", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"invalid_json"`)) {
		t.Fatalf("expected invalid_json code, got %q", rec.Body.String())
	}
}

func TestNewHandlerRejectsInvalidConfigPath(t *testing.T) {
	if _, err := NewHandler("/tmp/deltascope-missing-config.yaml", "test-build"); err == nil {
		t.Fatalf("expected invalid config path to fail")
	}
}

func TestHandlerAuditRejectsMissingAPIKeyWhenAuthEnabled(t *testing.T) {
	handler, err := NewHandler("", "test-build", WithAuthConfig(AuthConfig{
		Enabled: true,
		Keys:    []string{"ds_test_key"},
	}))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(`{"sql":"delete from users","dialect":"mysql"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"auth_required"`)) {
		t.Fatalf("expected auth_required code, got %q", rec.Body.String())
	}
}

func TestHandlerAuditRejectsInvalidAPIKeyWhenAuthEnabled(t *testing.T) {
	handler, err := NewHandler("", "test-build", WithAuthConfig(AuthConfig{
		Enabled: true,
		Keys:    []string{"ds_test_key"},
	}))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(`{"sql":"delete from users","dialect":"mysql"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "wrong_key")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"auth_invalid"`)) {
		t.Fatalf("expected auth_invalid code, got %q", rec.Body.String())
	}
}

func TestHandlerAuditAllowsValidAPIKeyWhenAuthEnabled(t *testing.T) {
	handler, err := NewHandler("", "test-build", WithAuthConfig(AuthConfig{
		Enabled: true,
		Keys:    []string{"ds_test_key"},
	}))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(`{"sql":"delete from users","dialect":"mysql"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "ds_test_key")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandlerHealthzBypassesAuthWhenAllowPathConfigured(t *testing.T) {
	handler, err := NewHandler("", "test-build", WithAuthConfig(AuthConfig{
		Enabled:    true,
		Keys:       []string{"ds_test_key"},
		AllowPaths: []string{"/healthz"},
	}))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandlerHealthzSetsRequestIDHeader(t *testing.T) {
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	requestID := rec.Header().Get("X-Request-ID")
	if requestID == "" {
		t.Fatalf("expected X-Request-ID header to be set")
	}
	if matched, _ := regexp.MatchString(`^req-[a-f0-9]{24}$`, requestID); !matched {
		t.Fatalf("unexpected request id format: %q", requestID)
	}
}

func TestRecoveryMiddlewareReturnsJSONEnvelopeOnPanic(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(recoveryMiddleware(log.New(io.Discard, "", 0)))
	router.GET("/panic", func(c *gin.Context) {
		panic("boom")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"internal_error"`)) {
		t.Fatalf("expected internal_error code, got %q", rec.Body.String())
	}
}

func TestHandlerAuditReturnsTimeoutWhenRequestDeadlineExceeded(t *testing.T) {
	handler, err := NewHandler(
		"",
		"test-build",
		WithMiddlewareConfig(MiddlewareConfig{RequestTimeout: 5 * time.Millisecond}),
		WithAuditFunc(func(ctx context.Context, _ deltascope.Request) (deltascope.Result, error) {
			select {
			case <-time.After(100 * time.Millisecond):
				return deltascope.Result{}, nil
			case <-ctx.Done():
				return deltascope.Result{}, ctx.Err()
			}
		}),
	)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(`{"sql":"delete from users","dialect":"mysql"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("expected 504, got %d: %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"request_timeout"`)) {
		t.Fatalf("expected request_timeout code, got %q", rec.Body.String())
	}
}

func TestMapAuditErrorTimeout(t *testing.T) {
	status, code := mapAuditError(context.DeadlineExceeded)
	if status != http.StatusGatewayTimeout || code != "request_timeout" {
		t.Fatalf("unexpected timeout mapping: status=%d code=%s", status, code)
	}
}

func TestMapAuditErrorCanceled(t *testing.T) {
	status, code := mapAuditError(context.Canceled)
	if status != http.StatusRequestTimeout || code != "request_canceled" {
		t.Fatalf("unexpected canceled mapping: status=%d code=%s", status, code)
	}
}

func TestMapAuditErrorEmptySQL(t *testing.T) {
	status, code := mapAuditError(appaudit.ErrEmptySQL)
	if status != http.StatusBadRequest || code != "bad_request" {
		t.Fatalf("unexpected bad request mapping: status=%d code=%s", status, code)
	}
}

func TestHandlerMetricsEndpoint(t *testing.T) {
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	primeReq := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	primeRec := httptest.NewRecorder()
	handler.ServeHTTP(primeRec, primeReq)
	if primeRec.Code != http.StatusOK {
		t.Fatalf("prime request expected 200, got %d: %s", primeRec.Code, primeRec.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("deltascope_http_requests_total")) {
		t.Fatalf("expected metrics payload, got %q", rec.Body.String())
	}
}

func TestHandlerAuditRateLimitByAPIKey(t *testing.T) {
	handler, err := NewHandler(
		"",
		"test-build",
		WithAuthConfig(AuthConfig{
			Enabled: true,
			Keys:    []string{"ds_test_key"},
		}),
		WithMiddlewareConfig(MiddlewareConfig{
			RateLimit: RateLimitConfig{
				Enabled: true,
				RPS:     1,
				Burst:   1,
				KeyBy:   "api-key",
			},
		}),
	)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req1 := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(`{"sql":"delete from users","dialect":"mysql"}`))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("X-API-Key", "ds_test_key")
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("first request expected 200, got %d: %s", rec1.Code, rec1.Body.String())
	}

	req2 := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(`{"sql":"delete from users","dialect":"mysql"}`))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("X-API-Key", "ds_test_key")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request expected 429, got %d: %s", rec2.Code, rec2.Body.String())
	}
	if !bytes.Contains(rec2.Body.Bytes(), []byte(`"rate_limited"`)) {
		t.Fatalf("expected rate_limited code, got %q", rec2.Body.String())
	}
}

func TestRequestRateLimitKeyIPUsesForwardedClientIP(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	rec := httptest.NewRecorder()
	ctx, engine := gin.CreateTestContext(rec)
	if err := engine.SetTrustedProxies([]string{"0.0.0.0/0", "::/0"}); err != nil {
		t.Fatalf("set trusted proxies: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/audit", nil)
	req.RemoteAddr = "10.0.0.2:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.8")
	ctx.Request = req

	got := requestRateLimitKey(ctx, "ip")
	if got != "203.0.113.8" {
		t.Fatalf("expected forwarded client ip, got %q", got)
	}
}

func TestLimiterStoreCleansExpiredEntries(t *testing.T) {
	store := newLimiterStore(rate.Limit(1), 1)
	store.ttl = time.Millisecond
	store.cleanupInterval = time.Millisecond
	store.nextCleanup = time.Now().Add(-time.Second)
	store.entries["stale"] = limiterEntry{
		limiter:  rate.NewLimiter(rate.Limit(1), 1),
		lastSeen: time.Now().Add(-time.Hour),
	}

	_ = store.Allow("fresh")

	store.mu.Lock()
	_, exists := store.entries["stale"]
	store.mu.Unlock()
	if exists {
		t.Fatalf("expected stale limiter entry to be evicted")
	}
}

func firstFindingByRuleID(t *testing.T, payload map[string]any, ruleID string) (map[string]any, map[string]any) {
	t.Helper()
	statements, ok := payload["statements"].([]any)
	if !ok {
		t.Fatalf("expected statements array, got %#v", payload["statements"])
	}
	for _, rawStatement := range statements {
		statement, ok := rawStatement.(map[string]any)
		if !ok {
			continue
		}
		findings, ok := statement["findings"].([]any)
		if !ok {
			continue
		}
		for _, rawFinding := range findings {
			finding, ok := rawFinding.(map[string]any)
			if !ok {
				continue
			}
			if finding["rule_id"] != ruleID {
				continue
			}
			explanation, ok := finding["explanation"].(map[string]any)
			if !ok {
				t.Fatalf("expected explanation on rule %q, got %#v", ruleID, finding["explanation"])
			}
			return finding, explanation
		}
	}
	t.Fatalf("expected finding for rule %q, got %#v", ruleID, payload)
	return nil, nil
}

func firstFindingWithoutExplanationByRuleID(t *testing.T, payload map[string]any, ruleID string) map[string]any {
	t.Helper()
	statements, ok := payload["statements"].([]any)
	if !ok {
		t.Fatalf("expected statements array, got %#v", payload["statements"])
	}
	for _, rawStatement := range statements {
		statement, ok := rawStatement.(map[string]any)
		if !ok {
			continue
		}
		findings, ok := statement["findings"].([]any)
		if !ok {
			continue
		}
		for _, rawFinding := range findings {
			finding, ok := rawFinding.(map[string]any)
			if !ok {
				continue
			}
			if finding["rule_id"] == ruleID {
				return finding
			}
		}
	}
	t.Fatalf("expected finding for rule %q, got %#v", ruleID, payload)
	return nil
}

func assertExplanationFieldString(t *testing.T, object map[string]any, key string) string {
	t.Helper()
	value, ok := object[key].(string)
	if !ok || value == "" {
		t.Fatalf("expected non-empty %q field, got %#v", key, object[key])
	}
	return value
}
