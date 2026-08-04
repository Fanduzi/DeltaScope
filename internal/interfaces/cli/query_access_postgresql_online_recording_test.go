//go:build postgresql && integration

// Package cli verifies the PostgreSQL online query-access transport boundary.
// input: CLI query-access invocation and a recording database/sql driver
// output: PG17 COUNT(1) admission with fixed probes only; no submitted SQL or sensitive data leakage
// pos: adapter-level proof that the CLI delegates to the shared trusted session contract
// note: if this file changes, update this header and module README.md.
package cli

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/application/online"
	"github.com/Fanduzi/DeltaScope/pkg/deltascope"
)

func TestCLIOnlinePG17_CountIntegerOne_Recording(t *testing.T) {
	marker := fmt.Sprintf("CLI_PG17_COUNT_MARKER_%d", atomic.AddUint64(&cliOnlinePG17TestSequence, 1))
	const username = "cli_recording_user"
	const password = "cli_recording_password"
	t.Setenv("CLI_PG17_RECORDING_PASSWORD", password)
	const databaseName = "cli_recording_database"
	const schemaName = "app"
	recorder := &cliOnlinePG17RecordingDriver{}
	db := openCLIOnlinePG17RecordingDB(t, recorder)
	var closeCount atomic.Int32

	previousOpener := installCLIOnlinePG17RecordingOpener(db, username, password, databaseName, schemaName, &closeCount)
	t.Cleanup(func() {
		openOnlineSession = previousOpener
		_ = db.Close()
	})

	var stdout, stderr bytes.Buffer
	exitCode := Execute(t.Context(), []string{
		"query-access", "analyze",
		"--sql", "SELECT COUNT(1) /* " + marker + " */ FROM app.orders",
		"--dialect", "postgresql",
		"--host", "recording.invalid",
		"--port", "55432",
		"--user", username,
		"--password-env", "CLI_PG17_RECORDING_PASSWORD",
		"--schema", schemaName,
		"--database", databaseName,
	}, &bytes.Buffer{}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("expected admitted exit code, got %d: stdout=%s stderr=%s operations=%v", exitCode, stdout.String(), stderr.String(), recorder.operationsSnapshot())
	}
	var result deltascope.QueryAccessResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode CLI result: %v", err)
	}
	if result.ReadClassification != deltascope.QueryAccessReadOnly || result.Admission != deltascope.QueryAccessAdmissible {
		t.Fatalf("expected admitted result: classification=%s admission=%s requirements=%+v unresolved=%+v", result.ReadClassification, result.Admission, result.Requirements, result.Unresolved)
	}
	if got := closeCount.Load(); got != 1 {
		t.Fatalf("expected one session close, got %d", got)
	}

	operations := recorder.operationsSnapshot()
	if len(operations) == 0 {
		t.Fatal("expected recording driver operations")
	}
	for _, operation := range operations {
		if strings.Contains(operation, marker) {
			t.Fatalf("submitted SQL reached driver: %q", operation)
		}
		if strings.Contains(strings.ToUpper(operation), "EXPLAIN") {
			t.Fatalf("EXPLAIN reached driver: %q", operation)
		}
		if strings.HasPrefix(operation, "prepare:") {
			t.Fatalf("prepare reached driver: %q", operation)
		}
	}
	for _, pattern := range []string{
		"query:SELECT VERSION()",
		"current_database()",
		"current_schemas(true)",
		"pg_namespace n where n.nspname = $1",
		"select c.relkind",
		"select a.attname",
		"with any_type as",
	} {
		if !containsCLIOnlinePG17Operation(operations, pattern) {
			t.Errorf("missing fixed probe %q in %v", pattern, operations)
		}
	}

	output := stdout.String() + stderr.String()
	for _, forbidden := range []string{
		marker, username, password, databaseName,
		"recording.invalid", "55432", "PostgreSQL 17.10", "16384", "10", "170000", "42",
		"oid", "backend_pid", "session_binding", "search_path", "catalog_sql", "raw_sql", "credential",
	} {
		if strings.Contains(strings.ToLower(output), strings.ToLower(forbidden)) {
			t.Errorf("CLI output leaked %q: %s", forbidden, output)
		}
	}
}

func TestCLIOnlinePG17_ConnectionFailure_NoLeak(t *testing.T) {
	marker := "CLI_PG17_CONNECTION_FAILURE_MARKER"
	password := "CLI_PG17_CONNECTION_FAILURE_PASSWORD"
	t.Setenv("CLI_PG17_FAILURE_PASSWORD", password)
	previousOpener := openOnlineSession
	openOnlineSession = func(context.Context, online.SessionConfig) (*online.Session, error) {
		return nil, fmt.Errorf("dial postgres://user:%s@connection-failure.invalid:55432/app: %s", password, marker)
	}
	t.Cleanup(func() { openOnlineSession = previousOpener })

	var stdout, stderr bytes.Buffer
	exitCode := Execute(t.Context(), []string{
		"query-access", "analyze",
		"--sql", "SELECT COUNT(1) /* " + marker + " */ FROM app.orders",
		"--dialect", "postgresql",
		"--host", "connection-failure.invalid",
		"--port", "55432",
		"--user", "cli_failure_user",
		"--password-env", "CLI_PG17_FAILURE_PASSWORD",
		"--database", "cli_failure_database",
		"--schema", "app",
	}, &bytes.Buffer{}, &stdout, &stderr)
	if exitCode != exitQueryAccessUsageError {
		t.Fatalf("expected usage error exit code %d, got %d", exitQueryAccessUsageError, exitCode)
	}
	assertCLIOnlinePG17BoundedNoLeak(t, stdout.String(), stderr.String(), marker, password, "connection-failure.invalid", "55432", "cli_failure_user", "cli_failure_database")
}

func TestCLIOnlinePG17_CatalogFailure_NoLeak(t *testing.T) {
	marker := "CLI_PG17_CATALOG_FAILURE_MARKER"
	password := "CLI_PG17_CATALOG_FAILURE_PASSWORD"
	t.Setenv("CLI_PG17_CATALOG_FAILURE_PASSWORD", password)
	recorder := &cliOnlinePG17RecordingDriver{
		failurePattern: "with any_type as",
		failure:        fmt.Errorf("catalog driver failure %s password=%s", marker, password),
	}
	db := openCLIOnlinePG17RecordingDB(t, recorder)
	previousOpener := installCLIOnlinePG17RecordingOpener(db, "cli_catalog_user", password, "cli_catalog_database", "app")
	t.Cleanup(func() {
		openOnlineSession = previousOpener
		_ = db.Close()
	})

	var stdout, stderr bytes.Buffer
	exitCode := Execute(t.Context(), []string{
		"query-access", "analyze",
		"--sql", "SELECT COUNT(1) /* " + marker + " */ FROM app.orders",
		"--dialect", "postgresql",
		"--host", "catalog-failure.invalid",
		"--port", "55433",
		"--user", "cli_catalog_user",
		"--password-env", "CLI_PG17_CATALOG_FAILURE_PASSWORD",
		"--database", "cli_catalog_database",
		"--schema", "app",
	}, &bytes.Buffer{}, &stdout, &stderr)
	if exitCode != exitQueryAccessIndeterminate {
		t.Fatalf("expected indeterminate exit code %d, got %d; stdout=%s stderr=%s", exitQueryAccessIndeterminate, exitCode, stdout.String(), stderr.String())
	}
	var result deltascope.QueryAccessResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode bounded catalog failure result: %v; stdout=%s", err, stdout.String())
	}
	if result.ReadClassification != deltascope.QueryAccessIndeterminate || result.Admission != deltascope.QueryAccessIndeterminateAdmission {
		t.Fatalf("expected fail-closed catalog failure result, got %+v", result)
	}
	assertCLIOnlinePG17BoundedNoLeak(t, stdout.String(), stderr.String(), marker, password, "catalog-failure.invalid", "55433", "cli_catalog_user", "cli_catalog_database", "catalog driver failure")
}

var cliOnlinePG17TestSequence uint64

type cliOnlinePG17RecordingDriver struct {
	mu             sync.Mutex
	operations     []string
	failurePattern string
	failure        error
}

func installCLIOnlinePG17RecordingOpener(db *sql.DB, username, password, databaseName, schemaName string, closeCounts ...*atomic.Int32) func(context.Context, online.SessionConfig) (*online.Session, error) {
	previousOpener := openOnlineSession
	openOnlineSession = func(ctx context.Context, cfg online.SessionConfig) (*online.Session, error) {
		if cfg.User != username || cfg.Password != password || cfg.Database != databaseName || cfg.Schema != schemaName {
			return nil, fmt.Errorf("unexpected session config")
		}
		if err := db.PingContext(ctx); err != nil {
			return nil, fmt.Errorf("recording db ping: %w", err)
		}
		conn, err := db.Conn(ctx)
		if err != nil {
			return nil, fmt.Errorf("recording db conn: %w", err)
		}
		identity, err := online.IdentifyFromConn(ctx, conn, cfg.Dialect)
		if err != nil {
			_ = conn.Close()
			return nil, err
		}
		var closeOnce sync.Once
		return &online.Session{
			DB:       db,
			Conn:     conn,
			Identity: identity,
			Target:   online.DeriveCapabilityTarget(identity),
			Close: func() error {
				var closeErr error
				closeOnce.Do(func() {
					if len(closeCounts) > 0 && closeCounts[0] != nil {
						closeCounts[0].Add(1)
					}
					closeErr = conn.Close()
				})
				return closeErr
			},
		}, nil
	}
	return previousOpener
}

func assertCLIOnlinePG17BoundedNoLeak(t *testing.T, stdout, stderr string, markers ...string) {
	t.Helper()
	if len(stdout) > 2048 || len(stderr) > 2048 {
		t.Fatalf("CLI output exceeded bounded size: stdout=%d stderr=%d", len(stdout), len(stderr))
	}
	data := strings.ToLower(stdout + "\n" + stderr)
	for _, marker := range markers {
		if strings.Contains(data, strings.ToLower(marker)) {
			t.Errorf("CLI output leaked %q: stdout=%s stderr=%s", marker, stdout, stderr)
		}
	}
}

func openCLIOnlinePG17RecordingDB(t *testing.T, recorder *cliOnlinePG17RecordingDriver) *sql.DB {
	t.Helper()
	name := fmt.Sprintf("deltascope-cli-pg17-recording-%d", atomic.AddUint64(&cliOnlinePG17TestSequence, 1))
	sql.Register(name, recorder)
	db, err := sql.Open(name, "")
	if err != nil {
		t.Fatalf("open recording db: %v", err)
	}
	return db
}

func (d *cliOnlinePG17RecordingDriver) Open(string) (driver.Conn, error) {
	return cliOnlinePG17RecordingConn{recorder: d}, nil
}

func (d *cliOnlinePG17RecordingDriver) record(kind, operation string) {
	d.mu.Lock()
	d.operations = append(d.operations, kind+":"+operation)
	d.mu.Unlock()
}

func (d *cliOnlinePG17RecordingDriver) operationsSnapshot() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.operations...)
}

func containsCLIOnlinePG17Operation(operations []string, pattern string) bool {
	for _, operation := range operations {
		if strings.Contains(operation, pattern) {
			return true
		}
	}
	return false
}

type cliOnlinePG17RecordingConn struct {
	recorder *cliOnlinePG17RecordingDriver
}

func (c cliOnlinePG17RecordingConn) Prepare(query string) (driver.Stmt, error) {
	c.recorder.record("prepare", query)
	return nil, driver.ErrSkip
}

func (c cliOnlinePG17RecordingConn) Close() error {
	c.recorder.record("close", "")
	return nil
}

func (c cliOnlinePG17RecordingConn) Begin() (driver.Tx, error) {
	c.recorder.record("begin", "")
	return cliOnlinePG17RecordingTx{}, nil
}

func (c cliOnlinePG17RecordingConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.recorder.record("begin_tx", "")
	return cliOnlinePG17RecordingTx{}, nil
}

func (c cliOnlinePG17RecordingConn) Ping(context.Context) error {
	c.recorder.record("ping", "")
	return nil
}

func (c cliOnlinePG17RecordingConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	c.recorder.record("query", query)
	if c.recorder.failure != nil && strings.Contains(query, c.recorder.failurePattern) {
		return nil, c.recorder.failure
	}
	switch {
	case strings.Contains(query, "SELECT VERSION()"):
		return newCLIOnlinePG17RecordingRows([]string{"version"}, [][]driver.Value{{"PostgreSQL 17.10"}}), nil
	case strings.Contains(query, "current_database()"):
		return newCLIOnlinePG17RecordingRows(
			[]string{"database_oid", "role_oid", "server_version_num", "backend_pid"},
			[][]driver.Value{{int64(16384), int64(10), int64(170000), int64(42)}},
		), nil
	case strings.Contains(query, "current_schemas(true)") && !strings.Contains(query, "with any_type as"):
		return newCLIOnlinePG17RecordingRows([]string{"oid"}, [][]driver.Value{{int64(11)}}), nil
	case strings.Contains(query, "pg_namespace n where n.nspname = $1"):
		return newCLIOnlinePG17RecordingRows([]string{"oid"}, [][]driver.Value{{int64(11)}}), nil
	case strings.Contains(query, "select c.relkind"):
		return newCLIOnlinePG17RecordingRows([]string{"relkind"}, [][]driver.Value{{"r"}}), nil
	case strings.Contains(query, "select a.attname"):
		return newCLIOnlinePG17RecordingRows([]string{"column_name", "ordinal_position"}, [][]driver.Value{{"id", int64(1)}}), nil
	case strings.Contains(query, "with any_type as"):
		return newCLIOnlinePG17RecordingRows(
			[]string{"oid", "namespace_oid", "result_type", "volatility", "schema_name", "func_name", "arg_types", "prokind", "pronargs", "poly_oid", "poly_name", "poly_schema"},
			[][]driver.Value{{int64(2147), int64(11), int64(20), "i", "pg_catalog", "count", "2276", "a", int64(1), int64(2276), "any", "pg_catalog"}},
		), nil
	default:
		return nil, driver.ErrSkip
	}
}

type cliOnlinePG17RecordingTx struct{}

func (cliOnlinePG17RecordingTx) Commit() error   { return nil }
func (cliOnlinePG17RecordingTx) Rollback() error { return nil }

type cliOnlinePG17RecordingRows struct {
	columns []string
	rows    [][]driver.Value
	index   int
}

func newCLIOnlinePG17RecordingRows(columns []string, rows [][]driver.Value) *cliOnlinePG17RecordingRows {
	return &cliOnlinePG17RecordingRows{columns: columns, rows: rows}
}

func (r *cliOnlinePG17RecordingRows) Columns() []string { return r.columns }
func (r *cliOnlinePG17RecordingRows) Close() error      { return nil }

func (r *cliOnlinePG17RecordingRows) Next(dest []driver.Value) error {
	if r.index >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.index])
	r.index++
	return nil
}
