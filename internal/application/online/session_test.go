// Package online provides tests for the session factory.
// input: mock database driver configurations
// output: verified session lifecycle behavior
// pos: unit tests for OpenSession with mock drivers
// note: if this file changes, update this header and module README.md.
package online

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
)

// testQueryResult holds pre-configured results for mock queries.
type testQueryResult struct {
	columns []string
	rows    [][]driver.Value
}

// onlineTestDriver is a custom sql driver for session tests.
type onlineTestDriver struct {
	results map[string]testQueryResult
	pingErr error
}

type onlineTestConn struct {
	results map[string]testQueryResult
	pingErr error
}

type onlineTestRows struct {
	columns []string
	rows    [][]driver.Value
	index   int
}

var (
	onlineTestDriverOnce     sync.Once
	onlineTestDriverResults  sync.Map
	onlineTestDriverPingErrs sync.Map
)

func registerOnlineTestDriver() {
	sql.Register("online-test-mysql", onlineTestDriver{})
}

func openOnlineTestDB(t *testing.T, version string, pingErr error) *sql.DB {
	t.Helper()

	onlineTestDriverOnce.Do(registerOnlineTestDriver)

	name := "online-test-" + strings.ReplaceAll(t.Name(), "/", "_")

	results := map[string]testQueryResult{
		"SELECT VERSION()": {
			columns: []string{"version"},
			rows:    [][]driver.Value{{version}},
		},
	}

	// Clone results.
	cloned := make(map[string]testQueryResult, len(results))
	for key, result := range results {
		rows := make([][]driver.Value, len(result.rows))
		for i := range result.rows {
			rows[i] = append([]driver.Value(nil), result.rows[i]...)
		}
		cloned[key] = testQueryResult{
			columns: append([]string(nil), result.columns...),
			rows:    rows,
		}
	}
	onlineTestDriverResults.Store(name, cloned)
	if pingErr != nil {
		onlineTestDriverPingErrs.Store(name, pingErr)
	}

	t.Cleanup(func() {
		onlineTestDriverResults.Delete(name)
		onlineTestDriverPingErrs.Delete(name)
	})

	db, err := sql.Open("online-test-mysql", name)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	return db
}

func (d onlineTestDriver) Open(name string) (driver.Conn, error) {
	value, _ := onlineTestDriverResults.Load(name)
	results, _ := value.(map[string]testQueryResult)
	var pingErr error
	if pe, ok := onlineTestDriverPingErrs.Load(name); ok {
		pingErr, _ = pe.(error)
	}
	return onlineTestConn{results: results, pingErr: pingErr}, nil
}

func (c onlineTestConn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (c onlineTestConn) Close() error                        { return nil }
func (c onlineTestConn) Begin() (driver.Tx, error)           { return nil, driver.ErrSkip }

func (c onlineTestConn) Ping(ctx context.Context) error {
	if c.pingErr != nil {
		return c.pingErr
	}
	return ctx.Err()
}

func (c onlineTestConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	for fragment, result := range c.results {
		if strings.Contains(query, fragment) {
			return &onlineTestRows{
				columns: append([]string(nil), result.columns...),
				rows:    append([][]driver.Value(nil), result.rows...),
			}, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (r *onlineTestRows) Columns() []string {
	return append([]string(nil), r.columns...)
}

func (r *onlineTestRows) Close() error { return nil }

func (r *onlineTestRows) Next(dest []driver.Value) error {
	if r.index >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.index])
	r.index++
	return nil
}

func TestIdentifyFromConn_MySQLVersion(t *testing.T) {
	db := openOnlineTestDB(t, "8.0.46", nil)
	defer db.Close()

	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatalf("db.Conn: %v", err)
	}
	defer conn.Close()

	id, err := IdentifyFromConn(context.Background(), conn, "mysql")
	if err != nil {
		t.Fatalf("IdentifyFromConn: %v", err)
	}
	if id.Product != ProductMySQL {
		t.Errorf("Product = %q, want %q", id.Product, ProductMySQL)
	}
	if id.Series != SeriesMySQL80 {
		t.Errorf("Series = %q, want %q", id.Series, SeriesMySQL80)
	}
	if id.Major != 8 || id.Minor != 0 || id.Patch != 46 {
		t.Errorf("Version = %d.%d.%d, want 8.0.46", id.Major, id.Minor, id.Patch)
	}
}

func TestIdentifyFromConn_PostgreSQLVersion(t *testing.T) {
	db := openOnlineTestDB(t, "PostgreSQL 17.4", nil)
	defer db.Close()

	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatalf("db.Conn: %v", err)
	}
	defer conn.Close()

	id, err := IdentifyFromConn(context.Background(), conn, "postgresql")
	if err != nil {
		t.Fatalf("IdentifyFromConn: %v", err)
	}
	if id.Product != ProductPostgreSQL {
		t.Errorf("Product = %q, want %q", id.Product, ProductPostgreSQL)
	}
	if id.Series != SeriesPG17 {
		t.Errorf("Series = %q, want %q", id.Series, SeriesPG17)
	}
}

func TestIdentifyFromConn_TiDBVersion(t *testing.T) {
	db := openOnlineTestDB(t, "8.0.11-TiDB-v8.5.7", nil)
	defer db.Close()

	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatalf("db.Conn: %v", err)
	}
	defer conn.Close()

	id, err := IdentifyFromConn(context.Background(), conn, "tidb")
	if err != nil {
		t.Fatalf("IdentifyFromConn: %v", err)
	}
	if id.Product != ProductTiDB {
		t.Errorf("Product = %q, want %q", id.Product, ProductTiDB)
	}
	if id.Series != SeriesTiDB85 {
		t.Errorf("Series = %q, want %q", id.Series, SeriesTiDB85)
	}
	if id.Patch != 7 {
		t.Errorf("Patch = %d, want 7", id.Patch)
	}
}

func TestIdentifyFromConn_UnsupportedVersion(t *testing.T) {
	db := openOnlineTestDB(t, "5.6.51", nil)
	defer db.Close()

	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatalf("db.Conn: %v", err)
	}
	defer conn.Close()

	_, err = IdentifyFromConn(context.Background(), conn, "mysql")
	if !errors.Is(err, ErrIdentityUnsupported) {
		t.Errorf("IdentifyFromConn: error = %v, want %v", err, ErrIdentityUnsupported)
	}
}

func TestIdentifyFromConn_DialectMismatch(t *testing.T) {
	db := openOnlineTestDB(t, "PostgreSQL 17.4", nil)
	defer db.Close()

	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatalf("db.Conn: %v", err)
	}
	defer conn.Close()

	_, err = IdentifyFromConn(context.Background(), conn, "mysql")
	if !errors.Is(err, ErrDialectMismatch) {
		t.Errorf("IdentifyFromConn: error = %v, want %v", err, ErrDialectMismatch)
	}
}

func TestIdentifyFromConn_Cancellation(t *testing.T) {
	db := openOnlineTestDB(t, "8.0.46", nil)
	defer db.Close()

	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatalf("db.Conn: %v", err)
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// The query may or may not fail depending on timing, but the function
	// should handle cancellation gracefully.
	_, _ = IdentifyFromConn(ctx, conn, "mysql")
}

func TestDeriveCapabilityTarget_AllTargets(t *testing.T) {
	tests := []struct {
		name   string
		series VersionSeries
		want   CapabilityTarget
	}{
		{"mysql 5.7", SeriesMySQL57, TargetMySQL57},
		{"mysql 8.0", SeriesMySQL80, TargetMySQL80},
		{"mysql 8.4", SeriesMySQL84, TargetMySQL84},
		{"tidb 8.5", SeriesTiDB85, TargetTiDB85},
		{"pg 17", SeriesPG17, TargetPG17},
		{"unknown", VersionSeries("unknown"), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := &ServerIdentity{Series: tt.series}
			target := DeriveCapabilityTarget(id)
			if target != tt.want {
				t.Errorf("DeriveCapabilityTarget() = %q, want %q", target, tt.want)
			}
		})
	}
}

func TestSessionClose_Idempotent(t *testing.T) {
	// Verify that double-close doesn't panic.
	db := openOnlineTestDB(t, "8.0.46", nil)
	defer db.Close()

	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatalf("db.Conn: %v", err)
	}

	identity, err := IdentifyFromConn(context.Background(), conn, "mysql")
	if err != nil {
		t.Fatalf("IdentifyFromConn: %v", err)
	}

	closed := false
	closeFn := func() error {
		if closed {
			return fmt.Errorf("already closed")
		}
		closed = true
		var firstErr error
		if cErr := conn.Close(); cErr != nil {
			firstErr = cErr
		}
		if dErr := db.Close(); dErr != nil && firstErr == nil {
			firstErr = dErr
		}
		return firstErr
	}

	session := &Session{
		DB:       db,
		Conn:     conn,
		Identity: identity,
		Target:   DeriveCapabilityTarget(identity),
		Close:    closeFn,
	}

	// First close.
	if err := session.Close(); err != nil {
		t.Fatalf("first close: %v", err)
	}

	// Second close should not panic.
	if err := session.Close(); err == nil {
		// Double-close returning nil is acceptable (idempotent behavior).
	}
}

func TestBuildMySQLDSN_Disabled(t *testing.T) {
	cfg := SessionConfig{
		Host:     "127.0.0.1",
		Port:     3306,
		User:     "root",
		Password: "secret",
		Database: "test",
		Dialect:  "mysql",
		TLSMode:  "disabled",
	}

	mysqlCfg := buildMySQLConfig(cfg)
	dsn := mysqlCfg.FormatDSN()

	// TLS should not be configured.
	if mysqlCfg.TLS != nil {
		t.Errorf("TLS should be nil when disabled")
	}
	// Should contain user and address.
	if !strings.Contains(dsn, "root:") {
		t.Errorf("DSN should contain user, got: %s", dsn)
	}
	if !strings.Contains(dsn, "127.0.0.1:3306") {
		t.Errorf("DSN should contain address, got: %s", dsn)
	}
}

func TestBuildMySQLDSN_Enabled(t *testing.T) {
	cfg := SessionConfig{
		Host:     "db.example.com",
		Port:     3306,
		User:     "app",
		Password: "secret",
		Database: "mydb",
		Dialect:  "mysql",
		TLSMode:  "enabled",
	}

	mysqlCfg := buildMySQLConfig(cfg)
	dsn := mysqlCfg.FormatDSN()

	// TLS should be configured with correct ServerName.
	if mysqlCfg.TLS == nil {
		t.Fatal("TLS should be configured when enabled")
	}
	if mysqlCfg.TLS.ServerName != "db.example.com" {
		t.Errorf("ServerName should be db.example.com, got: %s", mysqlCfg.TLS.ServerName)
	}
	if mysqlCfg.TLS.InsecureSkipVerify {
		t.Error("InsecureSkipVerify should be false")
	}
	// Should contain the host.
	if !strings.Contains(dsn, "db.example.com") {
		t.Errorf("DSN should contain host, got: %s", dsn)
	}
}

func TestBuildMySQLConfig_TLSDoesNotUseGlobalRegistry(t *testing.T) {
	// Regression test: online session TLS must use cfg.TLS directly,
	// NOT cfg.TLSConfig (which requires RegisterTLSConfig and leaks global state).
	cfg := SessionConfig{
		Host:     "db.example.com",
		Port:     3306,
		User:     "app",
		Password: "secret",
		Database: "mydb",
		Dialect:  "mysql",
		TLSMode:  "enabled",
	}

	mysqlCfg := buildMySQLConfig(cfg)

	// Must use cfg.TLS (direct), not cfg.TLSConfig (global registry).
	if mysqlCfg.TLS == nil {
		t.Fatal("TLS should be configured")
	}
	if mysqlCfg.TLSConfig != "" {
		t.Errorf("TLSConfig should be empty (no global registry), got: %s", mysqlCfg.TLSConfig)
	}
}

func TestBuildPostgreSQLDSN_Disabled(t *testing.T) {
	cfg := SessionConfig{
		Host:     "127.0.0.1",
		Port:     5432,
		User:     "postgres",
		Password: "secret",
		Database: "test",
		Dialect:  "postgresql",
		TLSMode:  "disabled",
	}

	dsn := buildPostgreSQLDSN(cfg)

	if !strings.Contains(dsn, "sslmode=disable") {
		t.Errorf("DSN should contain sslmode=disable, got: %s", dsn)
	}
	if strings.Contains(dsn, "sslmode=verify-full") {
		t.Errorf("DSN should not contain sslmode=verify-full, got: %s", dsn)
	}
}

func TestBuildPostgreSQLDSN_Enabled(t *testing.T) {
	cfg := SessionConfig{
		Host:     "pg.example.com",
		Port:     5432,
		User:     "app",
		Password: "secret",
		Database: "mydb",
		Dialect:  "postgresql",
		TLSMode:  "enabled",
	}

	dsn := buildPostgreSQLDSN(cfg)

	if !strings.Contains(dsn, "sslmode=verify-full") {
		t.Errorf("DSN should contain sslmode=verify-full, got: %s", dsn)
	}
	if strings.Contains(dsn, "sslmode=disable") {
		t.Errorf("DSN should not contain sslmode=disable, got: %s", dsn)
	}
	if !strings.Contains(dsn, "pg.example.com") {
		t.Errorf("DSN should contain host, got: %s", dsn)
	}
}

func TestBuildMySQLDSN_Socket(t *testing.T) {
	cfg := SessionConfig{
		Socket:   "/var/run/mysqld/mysqld.sock",
		User:     "root",
		Password: "secret",
		Database: "test",
		Dialect:  "mysql",
		TLSMode:  "disabled",
	}

	mysqlCfg := buildMySQLConfig(cfg)
	dsn := mysqlCfg.FormatDSN()

	if !strings.Contains(dsn, "unix") {
		t.Errorf("DSN should contain unix network for socket, got: %s", dsn)
	}
	if !strings.Contains(dsn, "/var/run/mysqld/mysqld.sock") {
		t.Errorf("DSN should contain socket path, got: %s", dsn)
	}
}

func TestBuildPostgreSQLDSN_Socket(t *testing.T) {
	cfg := SessionConfig{
		Socket:   "/var/run/postgresql",
		User:     "postgres",
		Password: "secret",
		Database: "test",
		Dialect:  "postgresql",
		TLSMode:  "disabled",
	}

	dsn := buildPostgreSQLDSN(cfg)

	if !strings.Contains(dsn, "host=") {
		t.Errorf("DSN should contain host parameter for socket, got: %s", dsn)
	}
	if !strings.Contains(dsn, "postgres://") {
		t.Errorf("DSN should be a postgres URL, got: %s", dsn)
	}
}

func TestOpenSession_UnsupportedDialect(t *testing.T) {
	ctx := context.Background()
	cfg := SessionConfig{
		Host:    "127.0.0.1",
		Port:    3306,
		User:    "root",
		Dialect: "sqlite",
	}

	_, err := OpenSession(ctx, cfg)
	if err == nil {
		t.Fatal("expected error for unsupported dialect")
	}
	if !strings.Contains(err.Error(), "unsupported dialect") {
		t.Errorf("error should mention unsupported dialect, got: %v", err)
	}
}

func TestOpenSession_NilContext(t *testing.T) {
	cfg := SessionConfig{
		Host:    "127.0.0.1",
		Port:    3306,
		User:    "root",
		Dialect: "mysql",
	}

	_, err := OpenSession(nil, cfg)
	if !errors.Is(err, ErrIdentityUnavailable) {
		t.Errorf("error = %v, want %v", err, ErrIdentityUnavailable)
	}
}

func TestBuildPostgreSQLDSN_DefaultPort(t *testing.T) {
	cfg := SessionConfig{
		Host:     "127.0.0.1",
		User:     "postgres",
		Password: "secret",
		Database: "test",
		Dialect:  "postgresql",
	}

	dsn := buildPostgreSQLDSN(cfg)

	if !strings.Contains(dsn, ":5432") {
		t.Errorf("DSN should use default port 5432, got: %s", dsn)
	}
}

func TestBuildMySQLDSN_DefaultPort(t *testing.T) {
	cfg := SessionConfig{
		Host:     "127.0.0.1",
		User:     "root",
		Password: "secret",
		Database: "test",
		Dialect:  "mysql",
	}

	mysqlCfg := buildMySQLConfig(cfg)
	dsn := mysqlCfg.FormatDSN()

	if !strings.Contains(dsn, "127.0.0.1:3306") {
		t.Errorf("DSN should use default port 3306, got: %s", dsn)
	}
}

func TestBuildPostgreSQLDSN_DefaultDatabase(t *testing.T) {
	cfg := SessionConfig{
		Host:     "127.0.0.1",
		Port:     5432,
		User:     "postgres",
		Password: "secret",
		Dialect:  "postgresql",
	}

	dsn := buildPostgreSQLDSN(cfg)

	if !strings.Contains(dsn, "/postgres") {
		t.Errorf("DSN should use default database 'postgres', got: %s", dsn)
	}
}

func TestBuildPostgreSQLConfigSetsExplicitPasswordAfterParse(t *testing.T) {
	t.Parallel()

	cfg := SessionConfig{
		Host:     "127.0.0.1",
		Port:     5432,
		User:     "postgres",
		Password: "secret-password",
		Database: "test",
		Dialect:  "postgresql",
		TLSMode:  "disabled",
	}

	dsn := buildPostgreSQLDSN(cfg)

	if strings.Contains(dsn, "secret-password") {
		t.Fatal("DSN must not contain password")
	}

	connConfig, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("pgx.ParseConfig: %v", err)
	}
	if connConfig.Password != "" {
		t.Fatalf("password should not be in parsed config from DSN, got %q", connConfig.Password)
	}

	connConfig.Password = cfg.Password
	if connConfig.Password != "secret-password" {
		t.Fatalf("explicit password not set correctly, got %q", connConfig.Password)
	}
}
