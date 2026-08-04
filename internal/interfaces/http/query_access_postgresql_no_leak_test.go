//go:build postgresql && integration

// Package httpapi verifies that the PostgreSQL 17 query-access HTTP surface
// does not leak SQL markers, credentials, catalog facts, server identity,
// backend details, or raw driver errors across online and guarded paths.
// input: PostgreSQL 17 HTTP query-access requests with unique markers
// output: bounded JSON responses with no forbidden fields or marker leakage
// pos: HTTP no-leak regression coverage for the PG17 COUNT(1) online contract
// note: if this file changes, update this header and module README.md.
package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/application/online"
	"github.com/Fanduzi/DeltaScope/internal/infrastructure/runtimeconfig"
)

func TestHTTPOnlinePG17_CountIntegerOne_NoLeak(t *testing.T) {
	marker := "PG17ONLINE_NOLEAK_HTTP_COUNT_8A4F2D1C"
	handler := newPG17HTTPNoLeakHandler(t, false)
	sqlText := "SELECT COUNT(1) /* " + marker + " */ FROM app.orders"

	status, body := postPG17QueryAccess(t, handler, fmt.Sprintf(`{"sql":%q,"connection_id":"pg17-online"}`, sqlText), true)
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", status, body)
	}
	assertPG17HTTPNoLeak(t, marker, sqlText, body)

	var result map[string]any
	if err := json.Unmarshal([]byte(body), &result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if result["read_classification"] != "read_only" || result["admission"] != "admissible" {
		t.Fatalf("unexpected COUNT(1) result: %#v", result)
	}
}

func TestHTTPOnlinePG17_ExcludedShapes_NoLeak(t *testing.T) {
	handler := newPG17HTTPNoLeakHandler(t, false)
	cases := []struct {
		name   string
		marker string
		sql    string
	}{
		{"count_two", "PG17ONLINE_NOLEAK_HTTP_EXCLUDED_2_5B8E1D3A", "SELECT COUNT(2) /* PG17ONLINE_NOLEAK_HTTP_EXCLUDED_2_5B8E1D3A */ FROM app.orders"},
		{"count_filter", "PG17ONLINE_NOLEAK_HTTP_EXCLUDED_FILTER_7C2A9E4D", "SELECT COUNT(1) FILTER (WHERE true) /* PG17ONLINE_NOLEAK_HTTP_EXCLUDED_FILTER_7C2A9E4D */ FROM app.orders"},
		{"count_window", "PG17ONLINE_NOLEAK_HTTP_EXCLUDED_WINDOW_1D6F4A8B", "SELECT COUNT(1) OVER () /* PG17ONLINE_NOLEAK_HTTP_EXCLUDED_WINDOW_1D6F4A8B */ FROM app.orders"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, body := postPG17QueryAccess(t, handler, fmt.Sprintf(`{"sql":%q,"connection_id":"pg17-online"}`, tc.sql), true)
			if status != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", status, body)
			}
			assertPG17HTTPNoLeak(t, tc.marker, tc.sql, body)
			var result map[string]any
			if err := json.Unmarshal([]byte(body), &result); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if result["read_classification"] != "indeterminate" || result["admission"] != "indeterminate" {
				t.Fatalf("excluded shape was promoted: %#v", result)
			}
		})
	}
}

func TestHTTPOnlinePG17_NoConnectionID_NoLeak(t *testing.T) {
	marker := "PG17ONLINE_NOLEAK_HTTP_OFFLINE_3E7A5C9B"
	sqlText := "SELECT COUNT(1) /* " + marker + " */ FROM app.orders"
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	status, body := postPG17QueryAccess(t, handler, fmt.Sprintf(`{"sql":%q}`, sqlText), false)
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", status, body)
	}
	assertPG17HTTPNoLeak(t, marker, sqlText, body)
	var result map[string]any
	if err := json.Unmarshal([]byte(body), &result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if result["admission"] != "indeterminate" {
		t.Fatalf("expected offline indeterminate admission: %#v", result)
	}
}

func TestHTTPOnlinePG17_Unauthorized_NoLeak(t *testing.T) {
	marker := "PG17ONLINE_NOLEAK_HTTP_UNAUTHORIZED_6D2B9F4A"
	sqlText := "SELECT COUNT(1) /* " + marker + " */ FROM app.orders"
	handler := newPG17HTTPNoLeakHandler(t, true)

	status, body := postPG17QueryAccess(t, handler, fmt.Sprintf(`{"sql":%q,"connection_id":"pg17-online"}`, sqlText), true)
	if status != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", status, body)
	}
	assertPG17HTTPNoLeak(t, marker, sqlText, body)
	if !strings.Contains(body, `"not_authorized"`) {
		t.Fatalf("expected bounded not_authorized response: %s", body)
	}
}

func TestHTTPOnlinePG17_Unauthorized_ZeroDial(t *testing.T) {
	var openCount atomic.Int32
	previous := openOnlineSession
	openOnlineSession = func(context.Context, online.SessionConfig) (*online.Session, error) {
		openCount.Add(1)
		return nil, errors.New("unexpected opener call")
	}
	t.Cleanup(func() { openOnlineSession = previous })
	handler := newPG17HTTPNoLeakHandler(t, true)
	status, body := postPG17QueryAccess(t, handler, `{"sql":"SELECT COUNT(1) FROM app.orders","connection_id":"pg17-online"}`, true)
	if status != http.StatusForbidden || !strings.Contains(body, `"not_authorized"`) {
		t.Fatalf("expected bounded unauthorized response, got %d: %s", status, body)
	}
	if got := openCount.Load(); got != 0 {
		t.Fatalf("unauthorized request opened a session %d times", got)
	}
	assertPG17HTTPNoLeak(t, "pg17-online", "SELECT COUNT(1)", body)
}

func TestHTTPOnlinePG17_UnknownConnection_ZeroDial(t *testing.T) {
	var openCount atomic.Int32
	previous := openOnlineSession
	openOnlineSession = func(context.Context, online.SessionConfig) (*online.Session, error) {
		openCount.Add(1)
		return nil, errors.New("unexpected opener call")
	}
	t.Cleanup(func() { openOnlineSession = previous })
	handler := newPG17HTTPNoLeakHandler(t, false)
	marker := "HTTP_PG17_UNKNOWN_CONNECTION_MARKER"
	status, body := postPG17QueryAccess(t, handler, fmt.Sprintf(`{"sql":%q,"connection_id":"pg17-unknown"}`, "SELECT COUNT(1) /* "+marker+" */ FROM app.orders"), true)
	if status != http.StatusNotFound || !strings.Contains(body, `"connection_not_found"`) {
		t.Fatalf("expected bounded connection-not-found response, got %d: %s", status, body)
	}
	if got := openCount.Load(); got != 0 {
		t.Fatalf("unknown connection opened a session %d times", got)
	}
	assertPG17HTTPNoLeak(t, marker, "pg17-unknown", body)
}

func TestHTTPOnlinePG17_ConnectionFailure_NoLeak(t *testing.T) {
	marker := "HTTP_PG17_CONNECTION_FAILURE_MARKER"
	password := "HTTP_PG17_CONNECTION_FAILURE_PASSWORD"
	configMarker := "HTTP_PG17_CONNECTION_FAILURE_CONFIG"
	var logBuf syncBuffer
	previous := openOnlineSession
	openOnlineSession = func(context.Context, online.SessionConfig) (*online.Session, error) {
		return nil, fmt.Errorf("%w: dial postgres://user:%s@%s:55434/app: %s", online.ErrConnectionFailed, password, configMarker, marker)
	}
	t.Cleanup(func() { openOnlineSession = previous })
	handler := newPG17HTTPNoLeakHandler(t, false, &logBuf)
	sqlText := "SELECT COUNT(1) /* " + marker + " */ FROM app.orders"
	status, body := postPG17QueryAccess(t, handler, fmt.Sprintf(`{"sql":%q,"connection_id":"pg17-online"}`, sqlText), true)
	if status != http.StatusBadGateway || !strings.Contains(body, `"connection_failed"`) {
		t.Fatalf("expected bounded connection_failed response, got %d: %s", status, body)
	}
	if len(body) > 2048 {
		t.Fatalf("HTTP connection failure response exceeded bound: %d", len(body))
	}
	assertAccessLogEntry(t, &logBuf, "/v1/query-access/analyze")
	combined := strings.ToLower(body + "\n" + logBuf.String())
	for _, forbidden := range []string{marker, password, configMarker, "postgres://", "recording.invalid", "55434", "driver failure"} {
		if strings.Contains(combined, strings.ToLower(forbidden)) {
			t.Errorf("HTTP connection failure leaked %q: %s", forbidden, combined)
		}
	}
}

func newPG17HTTPNoLeakHandler(t *testing.T, unauthorized bool, logWriters ...io.Writer) http.Handler {
	t.Helper()
	t.Setenv("PG17_HTTP_PASSWORD", "root")
	t.Setenv("PG17_HTTP_API_KEY", "pg17-test-key")
	t.Setenv("PG17_HTTP_OTHER_KEY", "pg17-other-key")

	allowed := []string{"pg17-key"}
	if unauthorized {
		allowed = []string{"pg17-other-key"}
	}
	cfg := runtimeconfig.Config{
		HTTP: runtimeconfig.HTTPConfig{Auth: runtimeconfig.AuthConfig{
			Enabled: true,
			Keys: []runtimeconfig.APIKeyConfig{
				{ID: "pg17-key", SecretEnv: "PG17_HTTP_API_KEY"},
				{ID: "pg17-other-key", SecretEnv: "PG17_HTTP_OTHER_KEY"},
			},
		}},
		Metadata: runtimeconfig.MetadataConfig{Connections: []runtimeconfig.ConnectionConfig{{
			ID:               "pg17-online",
			Dialect:          "postgresql",
			Host:             envOrPG17HTTP("DELTASCOPE_PG_HOST", "127.0.0.1"),
			Port:             envOrPG17HTTPPort(),
			User:             envOrPG17HTTP("DELTASCOPE_PG_USER", "root"),
			PasswordEnv:      "PG17_HTTP_PASSWORD",
			Database:         envOrPG17HTTP("DELTASCOPE_PG_DATABASE", "postgres"),
			Schema:           "app",
			Purposes:         []string{"query_access"},
			AllowedAPIKeyIDs: allowed,
		}}},
	}
	reg, err := runtimeconfig.ValidateAndBuildRegistry(cfg)
	if err != nil {
		t.Fatalf("build PG17 registry: %v", err)
	}
	options := []HandlerOption{WithRegistry(reg)}
	if len(logWriters) > 0 && logWriters[0] != nil {
		options = append(options, WithMiddlewareConfig(MiddlewareConfig{Logger: log.New(logWriters[0], "", 0)}))
	}
	handler, err := NewHandler("", "test-build", options...)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}
	return handler
}

func postPG17QueryAccess(t *testing.T, handler http.Handler, payload string, authenticated bool) (int, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/query-access/analyze", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	if authenticated {
		req.Header.Set("X-API-Key", "pg17-test-key")
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec.Code, rec.Body.String()
}

func envOrPG17HTTP(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envOrPG17HTTPPort() int {
	value := envOrPG17HTTP("DELTASCOPE_PG_PORT", "5500")
	port, err := strconv.Atoi(value)
	if err != nil {
		return 5500
	}
	return port
}

func assertPG17HTTPNoLeak(t *testing.T, marker, sqlText, body string) {
	t.Helper()
	for _, forbidden := range []string{
		marker,
		sqlText,
		"PG17ONLINE_NOLEAK_HTTP_USER_2C8A",
		"PG17ONLINE_NOLEAK_HTTP_PASSWORD_9F4D",
		"PG17ONLINE_NOLEAK_HTTP_CATALOG_7B1E",
		"PG17ONLINE_NOLEAK_HTTP_VERSION_17000",
		"PG17ONLINE_NOLEAK_HTTP_BACKEND_5E3A",
		"catalog lookup driver error",
		"postgres://",
		"pgx",
		"password",
		"server_version",
		"backend_pid",
	} {
		if strings.Contains(body, forbidden) {
			t.Errorf("HTTP response leaked %q: %s", forbidden, body)
		}
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatalf("decode HTTP JSON for forbidden-field check: %v", err)
	}
	for _, field := range []string{
		"raw_sql", "dsn", "credentials", "catalog", "server_version",
		"backend_pid", "backend_details", "driver_error", "identity",
		"manifest", "session", "context", "severity",
	} {
		if _, present := payload[field]; present {
			t.Errorf("HTTP JSON carried forbidden field %q: %s", field, body)
		}
	}
}
