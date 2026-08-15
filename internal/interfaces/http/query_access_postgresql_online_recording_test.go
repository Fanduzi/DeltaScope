//go:build postgresql && integration

// Package httpapi verifies the PostgreSQL online query-access transport boundary.
// input: HTTP query-access requests with connection_id and a recording database/sql driver
// output: bounded PG17 adapter delegation, close, error, and no-execution/no-leak evidence
// pos: focused adapter proof; unified SDK tests own detailed probe sequencing
// note: if this file changes, update this header and module README.md.
package httpapi

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/application/online"
	"github.com/Fanduzi/DeltaScope/internal/infrastructure/runtimeconfig"
)

func TestHTTPOnlinePG17_CountIntegerOne_Recording(t *testing.T) {
	marker := "HTTP_PG17_COUNT1_MARKER_" + strings.NewReplacer("/", "_").Replace(t.Name())
	recorder := &httpPostgreSQLRecordingDriver{}
	driverName := registerHTTPPostgreSQLRecordingDriver(t, recorder)
	var closeCount atomic.Int32

	previous := openOnlineSession
	openOnlineSession = func(ctx context.Context, cfg online.SessionConfig) (*online.Session, error) {
		return openHTTPRecordingSession(ctx, driverName, cfg, &closeCount)
	}
	t.Cleanup(func() { openOnlineSession = previous })

	password := "http-pg17-recording-password"
	t.Setenv("TEST_HTTP_PG17_PASSWORD", password)
	t.Setenv("TEST_HTTP_PG17_API_KEY", "http-pg17-api-key")
	registry, err := runtimeconfig.ValidateAndBuildRegistry(runtimeconfig.Config{
		HTTP: runtimeconfig.HTTPConfig{
			Auth: runtimeconfig.AuthConfig{
				Enabled: true,
				Keys:    []runtimeconfig.APIKeyConfig{{ID: "http-pg17-key", SecretEnv: "TEST_HTTP_PG17_API_KEY"}},
			},
		},
		Metadata: runtimeconfig.MetadataConfig{
			Connections: []runtimeconfig.ConnectionConfig{{
				ID:               "http-pg17-recording",
				Dialect:          "postgresql",
				Host:             "recording.invalid",
				Port:             5432,
				User:             "http_pg17_user",
				PasswordEnv:      "TEST_HTTP_PG17_PASSWORD",
				Database:         "http_pg17_catalog",
				Schema:           "app",
				Purposes:         []string{"query_access"},
				AllowedAPIKeyIDs: []string{"http-pg17-key"},
			}},
		},
	})
	if err != nil {
		t.Fatalf("build registry: %v", err)
	}

	handler, err := NewHandler("", "test-build", WithRegistry(registry))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	sqlText := "SELECT COUNT(1) /* " + marker + " */ FROM app.orders"
	req := httptest.NewRequest(http.MethodPost, "/v1/query-access/analyze", bytes.NewBufferString(fmt.Sprintf(`{"sql":%q,"connection_id":"http-pg17-recording"}`, sqlText)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "http-pg17-api-key")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response["read_classification"] != "read_only" || response["admission"] != "admissible" {
		t.Fatalf("expected positive result, got %s; queries=%v", rec.Body.String(), recorder.recordedQueries())
	}
	if got := closeCount.Load(); got != 1 {
		t.Fatalf("expected one session close, got %d", got)
	}

	operations := recorder.recordedOperations()
	if len(operations) == 0 {
		t.Fatal("expected adapter delegation through the recording session")
	}
	for _, operation := range operations {
		if strings.Contains(operation, marker) {
			t.Fatalf("submitted SQL reached driver: %q", operation)
		}
		if strings.Contains(strings.ToLower(operation), "explain") {
			t.Fatalf("EXPLAIN reached driver: %q", operation)
		}
	}
	if recorder.preparedCount() != 0 {
		t.Fatalf("prepare operation reached driver: %v", recorder.preparedQueries())
	}
	body := strings.ToLower(rec.Body.String())
	for _, forbidden := range []string{
		marker,
		password,
		"http-pg17-api-key",
		"http_pg17_user",
		"recording.invalid",
		"http_pg17_catalog",
		"database_oid",
		"role_oid",
		"backend_pid",
		"search_path",
		"catalog_sql",
		"raw_sql",
		"postgresql 17.10",
		"16384",
		"170000",
		"2147",
		"2276",
		"pg_catalog",
		"876543210",
	} {
		if strings.Contains(body, strings.ToLower(forbidden)) {
			t.Errorf("HTTP response leaked %q: %s", forbidden, rec.Body.String())
		}
	}
}

func TestHTTPOnlinePG17_CatalogFailure_NoLeak(t *testing.T) {
	marker := "HTTP_PG17_CATALOG_FAILURE_MARKER"
	password := "HTTP_PG17_CATALOG_FAILURE_PASSWORD"
	t.Setenv("PG17_HTTP_PASSWORD", password)
	t.Setenv("PG17_HTTP_API_KEY", "http-pg17-api-key")
	recorder := &httpPostgreSQLRecordingDriver{
		failurePattern: "with any_type as",
		failure:        fmt.Errorf("catalog driver failure %s password=%s", marker, password),
	}
	driverName := registerHTTPPostgreSQLRecordingDriver(t, recorder)
	var closeCount atomic.Int32
	previous := openOnlineSession
	openOnlineSession = func(ctx context.Context, cfg online.SessionConfig) (*online.Session, error) {
		return openHTTPRecordingSession(ctx, driverName, cfg, &closeCount)
	}
	t.Cleanup(func() { openOnlineSession = previous })
	var logBuf syncBuffer
	handler := newPG17HTTPNoLeakHandler(t, false, &logBuf)
	status, body := postPG17QueryAccess(t, handler, fmt.Sprintf(`{"sql":%q,"connection_id":"pg17-online"}`, "SELECT COUNT(1) /* "+marker+" */ FROM app.orders"), true)
	if status != http.StatusOK {
		t.Fatalf("expected bounded catalog failure result status 200, got %d: %s", status, body)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(body), &result); err != nil {
		t.Fatalf("decode bounded catalog failure response: %v", err)
	}
	if result["read_classification"] != "indeterminate" || result["admission"] != "indeterminate" {
		t.Fatalf("expected fail-closed catalog failure result: %s", body)
	}
	assertPG17HTTPNoLeak(t, marker, "SELECT COUNT(1)", body)
	if got := closeCount.Load(); got != 1 {
		t.Fatalf("expected one session close after catalog failure, got %d", got)
	}
	if len(body) > 2048 {
		t.Fatalf("HTTP catalog failure response exceeded bound: %d", len(body))
	}
	assertAccessLogEntry(t, &logBuf, "/v1/query-access/analyze")
	combined := strings.ToLower(body + "\n" + logBuf.String())
	for _, forbidden := range []string{marker, password, "catalog driver failure", "pg17-online", "recording.invalid"} {
		if strings.Contains(combined, strings.ToLower(forbidden)) {
			t.Errorf("HTTP catalog failure leaked %q: %s", forbidden, combined)
		}
	}
}

func openHTTPRecordingSession(ctx context.Context, driverName string, cfg online.SessionConfig, closeCounts ...*atomic.Int32) (*online.Session, error) {
	db, err := sql.Open(driverName, "")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	conn, err := db.Conn(ctx)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	identity, err := online.IdentifyFromConn(ctx, conn, cfg.Dialect)
	if err != nil {
		_ = conn.Close()
		_ = db.Close()
		return nil, err
	}
	return &online.Session{
		DB:       db,
		Conn:     conn,
		Identity: identity,
		Target:   online.DeriveCapabilityTarget(identity),
		Close: func() error {
			if connErr := conn.Close(); connErr != nil {
				_ = db.Close()
				return connErr
			}
			if len(closeCounts) > 0 && closeCounts[0] != nil {
				closeCounts[0].Add(1)
			}
			return db.Close()
		},
	}, nil
}

var httpPostgreSQLRecordingDriverSequence struct {
	sync.Mutex
	next int
}

func registerHTTPPostgreSQLRecordingDriver(t *testing.T, recorder *httpPostgreSQLRecordingDriver) string {
	t.Helper()
	httpPostgreSQLRecordingDriverSequence.Lock()
	httpPostgreSQLRecordingDriverSequence.next++
	name := fmt.Sprintf("deltascope-http-pg-recording-%d", httpPostgreSQLRecordingDriverSequence.next)
	httpPostgreSQLRecordingDriverSequence.Unlock()
	sql.Register(name, recorder)
	return name
}

type httpPostgreSQLRecordingDriver struct {
	mu             sync.Mutex
	queries        []string
	prepared       []string
	operations     []string
	failurePattern string
	failure        error
}

func (d *httpPostgreSQLRecordingDriver) Open(string) (driver.Conn, error) {
	return httpPostgreSQLRecordingConn{recorder: d}, nil
}

func (d *httpPostgreSQLRecordingDriver) recordQuery(query string) {
	d.mu.Lock()
	d.queries = append(d.queries, query)
	d.operations = append(d.operations, "query: "+query)
	d.mu.Unlock()
}

func (d *httpPostgreSQLRecordingDriver) recordPrepare(query string) {
	d.mu.Lock()
	d.prepared = append(d.prepared, query)
	d.operations = append(d.operations, "prepare: "+query)
	d.mu.Unlock()
}

func (d *httpPostgreSQLRecordingDriver) recordOperation(operation string) {
	d.mu.Lock()
	d.operations = append(d.operations, operation)
	d.mu.Unlock()
}

func (d *httpPostgreSQLRecordingDriver) recordedQueries() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.queries...)
}

func (d *httpPostgreSQLRecordingDriver) preparedQueries() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.prepared...)
}

func (d *httpPostgreSQLRecordingDriver) preparedCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.prepared)
}

func (d *httpPostgreSQLRecordingDriver) recordedOperations() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.operations...)
}

type httpPostgreSQLRecordingConn struct {
	recorder *httpPostgreSQLRecordingDriver
}

func (c httpPostgreSQLRecordingConn) Prepare(query string) (driver.Stmt, error) {
	c.recorder.recordPrepare(query)
	return nil, driver.ErrSkip
}

func (c httpPostgreSQLRecordingConn) Close() error { return nil }
func (c httpPostgreSQLRecordingConn) Begin() (driver.Tx, error) {
	c.recorder.recordOperation("begin")
	return httpPostgreSQLRecordingTx{recorder: c.recorder}, nil
}
func (c httpPostgreSQLRecordingConn) Ping(context.Context) error {
	c.recorder.recordOperation("ping")
	return nil
}
func (c httpPostgreSQLRecordingConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.recorder.recordOperation("begin_tx")
	return httpPostgreSQLRecordingTx{recorder: c.recorder}, nil
}

func (c httpPostgreSQLRecordingConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	c.recorder.recordQuery(query)
	if c.recorder.failure != nil && strings.Contains(query, c.recorder.failurePattern) {
		return nil, c.recorder.failure
	}
	switch {
	case strings.Contains(query, "SELECT VERSION()"):
		return newHTTPPostgreSQLRecordingRows([]string{"version"}, [][]driver.Value{{"PostgreSQL 17.10"}}), nil
	case strings.Contains(query, "current_database()"):
		return newHTTPPostgreSQLRecordingRows([]string{"database_oid", "role_oid", "server_version_num", "backend_pid"}, [][]driver.Value{{int64(16384), int64(10), int64(170000), int64(42)}}), nil
	case strings.Contains(query, "current_schemas(true)") && !strings.Contains(query, "with any_type as"):
		return newHTTPPostgreSQLRecordingRows([]string{"oid"}, [][]driver.Value{{int64(11)}}), nil
	case strings.Contains(query, "pg_namespace n where n.nspname = $1"):
		return newHTTPPostgreSQLRecordingRows([]string{"oid"}, [][]driver.Value{{int64(11)}}), nil
	case strings.Contains(query, "select c.relkind"):
		return newHTTPPostgreSQLRecordingRows([]string{"relkind"}, [][]driver.Value{{"r"}}), nil
	case strings.Contains(query, "select a.attname"):
		return newHTTPPostgreSQLRecordingRows([]string{"column_name", "ordinal_position"}, [][]driver.Value{{"id", int64(1)}}), nil
	case strings.Contains(query, "with any_type as"):
		return newHTTPPostgreSQLRecordingRows([]string{"oid", "namespace_oid", "result_type", "volatility", "schema_name", "func_name", "arg_types", "prokind", "pronargs", "poly_oid", "poly_name", "poly_schema"}, [][]driver.Value{{int64(2147), int64(11), int64(20), "i", "pg_catalog", "count", "2276", "a", int64(1), int64(2276), "any", "pg_catalog"}}), nil
	default:
		return nil, fmt.Errorf("unexpected recording-driver query")
	}
}

func (c httpPostgreSQLRecordingConn) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	c.recorder.recordOperation("exec: " + query)
	return nil, fmt.Errorf("unexpected recording-driver exec")
}

type httpPostgreSQLRecordingTx struct {
	recorder *httpPostgreSQLRecordingDriver
}

func (tx httpPostgreSQLRecordingTx) Commit() error {
	tx.recorder.recordOperation("commit")
	return nil
}
func (tx httpPostgreSQLRecordingTx) Rollback() error {
	tx.recorder.recordOperation("rollback")
	return nil
}

type httpPostgreSQLRecordingRows struct {
	columns []string
	rows    [][]driver.Value
	index   int
}

func newHTTPPostgreSQLRecordingRows(columns []string, rows [][]driver.Value) *httpPostgreSQLRecordingRows {
	return &httpPostgreSQLRecordingRows{columns: columns, rows: rows}
}

func (r *httpPostgreSQLRecordingRows) Columns() []string { return r.columns }
func (r *httpPostgreSQLRecordingRows) Close() error      { return nil }

func (r *httpPostgreSQLRecordingRows) Next(dest []driver.Value) error {
	if r.index >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.index])
	r.index++
	return nil
}
