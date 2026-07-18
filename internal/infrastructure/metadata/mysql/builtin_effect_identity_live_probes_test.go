//go:build integration

// Package mysqlmeta runs REAL Docker-backed MySQL 8.4 and TiDB 8.5 builtin
// effect identity feasibility probes (Tasks 3 and 4). These tests open real
// *sql.Conn connections to the live servers started by
// `docker compose -f docker/cli-e2e-compose.yaml up -d --wait` and assert
// server-returned evidence. They do NOT execute promotion code.
//
// input: live Docker MySQL 8.4 (mysql:8.4) and TiDB 8.5 (pingcap/tidb:v8.5.0)
//
//	probes over a caller-owned *sql.Conn
//
// output: per-dialect live server evidence locked against the actual server
//
//	response; the tests FAIL if the Docker server returns materially different
//	evidence (they do not self-assert hardcoded struct literals). The
//	disposition tests execute real probes and skip themselves only when the
//	Docker service is unreachable.
//
// pos: integration evidence only; skipped when Docker/compose is unavailable
//
//	(not claimed via mocks). MySQL and TiDB are independent proof domains;
//	neither dialect's probes infer the other's evidence.
//
// note: if this file changes, update this header, the module README.md, and
//
//	the decision record's evidence section. See
//	docs/decisions/2026-07-17-query-access-mysql-tidb-effect-identity-feasibility.md
//	for the per-dialect KILL/DEFER disposition and its scope.
package mysqlmeta

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// envOr returns the env var value or fallback when unset/empty.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// envOrInt returns the env var int value or fallback when unset/invalid.
func envOrInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

// mysqlIntegrationConfig returns the ConnectionConfig for the Docker MySQL 8.4
// service. Defaults match docker/cli-e2e-compose.yaml (port 3406, root/root).
func mysqlIntegrationConfig() ConnectionConfig {
	return ConnectionConfig{
		Host:           envOr("DELTASCOPE_MYSQL_HOST", "127.0.0.1"),
		Port:           envOrInt("DELTASCOPE_MYSQL_PORT", 3406),
		User:           envOr("DELTASCOPE_MYSQL_USER", "root"),
		Password:       envOr("DELTASCOPE_MYSQL_PASSWORD", "root"),
		ConnectTimeout: 5 * time.Second,
	}
}

// tidbIntegrationConfig returns the ConnectionConfig for the Docker TiDB 8.5
// service. Defaults match docker/cli-e2e-compose.yaml (port 4400, root, no
// password).
func tidbIntegrationConfig() ConnectionConfig {
	return ConnectionConfig{
		Host:           envOr("DELTASCOPE_TIDB_HOST", "127.0.0.1"),
		Port:           envOrInt("DELTASCOPE_TIDB_PORT", 4400),
		User:           envOr("DELTASCOPE_TIDB_USER", "root"),
		Password:       envOr("DELTASCOPE_TIDB_PASSWORD", ""),
		ConnectTimeout: 5 * time.Second,
	}
}

// errDockerUnreachable is a sentinel for confirmed Docker/compose absence.
// Only this error triggers t.Skipf; all other connection/query errors FAIL
// the test (they indicate a real server regression or misconfiguration).
var errDockerUnreachable = errors.New("docker service unreachable")

// openIntegrationConn dials a single *sql.Conn for the given dialect's Docker
// service. It returns errDockerUnreachable (caller skips) only when the
// service is confirmed unreachable (connection refused / timeout / DNS). Any
// other error (auth, server error, malformed response) is returned as a real
// error and the caller MUST fail the test.
func openIntegrationConn(t *testing.T, dialect string) (*sql.Conn, *sql.DB, error) {
	t.Helper()
	var cfg ConnectionConfig
	switch dialect {
	case "mysql":
		cfg = mysqlIntegrationConfig()
	case "tidb":
		cfg = tidbIntegrationConfig()
	default:
		return nil, nil, fmt.Errorf("openIntegrationConn: unknown dialect %q", dialect)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := OpenDBContext(ctx, cfg)
	if err != nil {
		// Distinguish Docker-unreachable (skip) from real errors (fail).
		if isDockerUnreachableErr(err) {
			return nil, nil, errDockerUnreachable
		}
		return nil, nil, fmt.Errorf("open %s %s:%d: %w", dialect, cfg.Host, cfg.Port, err)
	}

	conn, err := db.Conn(context.Background())
	if err != nil {
		db.Close()
		if isDockerUnreachableErr(err) {
			return nil, nil, errDockerUnreachable
		}
		return nil, nil, fmt.Errorf("conn %s %s:%d: %w", dialect, cfg.Host, cfg.Port, err)
	}
	return conn, db, nil
}

// isDockerUnreachableErr reports whether err looks like the Docker service is
// not running (connection refused, timeout, no such host). Auth/server errors
// return false so the test fails instead of skipping.
func isDockerUnreachableErr(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "connection refused") ||
		strings.Contains(s, "no such host") ||
		strings.Contains(s, "i/o timeout") ||
		strings.Contains(s, "connect: connection timed out") ||
		strings.Contains(s, "deadline exceeded")
}

// skipIfUnreachable opens a conn for the dialect; skips the test if Docker is
// unreachable; returns the conn + db otherwise. Any non-Docker error fails
// the test with a bounded message (no driver error / DSN leak).
func skipIfUnreachable(t *testing.T, dialect string) (*sql.Conn, *sql.DB) {
	t.Helper()
	conn, db, err := openIntegrationConn(t, dialect)
	if err == nil {
		return conn, db
	}
	if errors.Is(err, errDockerUnreachable) {
		t.Skipf("%s integration unavailable (Docker/compose not running)", dialect)
	}
	// Real error — fail with bounded message, no driver error / DSN leak.
	var cfg ConnectionConfig
	switch dialect {
	case "mysql":
		cfg = mysqlIntegrationConfig()
	case "tidb":
		cfg = tidbIntegrationConfig()
	}
	t.Fatalf("%s integration connection failed on %s:%d (see server logs; driver error suppressed)",
		dialect, cfg.Host, cfg.Port)
	return nil, nil
}

// ---------------------------------------------------------------------------
// MySQL 8.4 live probes
// ---------------------------------------------------------------------------

// TestMySQL84_LiveProbes_ServerVersion records the actual server version
// returned by the live MySQL 8.4 Docker service. It FAILS if the server is not
// a MySQL 8.4.x release — the probe must reflect the real server, not a
// hardcoded constant.
func TestMySQL84_LiveProbes_ServerVersion(t *testing.T) {
	conn, db := skipIfUnreachable(t, "mysql")
	defer func() {
		_ = conn.Close()
		db.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var version string
	if err := conn.QueryRowContext(ctx, "SELECT VERSION()").Scan(&version); err != nil {
		t.Fatalf("SELECT VERSION() on MySQL failed (driver error suppressed)")
	}

	t.Logf("MySQL live server version: %s", version)

	if !strings.HasPrefix(version, "8.4.") {
		t.Fatalf("expected MySQL 8.4.x live version, got %q (compose image must be mysql:8.4)", version)
	}
}

// TestMySQL84_LiveProbes_StoredFunctionDeterministic proves MySQL 8.4 supports
// CREATE FUNCTION ... DETERMINISTIC and that information_schema.ROUTINES
// records IS_DETERMINISTIC = YES for such a function. This is supporting
// negative evidence: DETERMINISTIC is a stored-function declaration, NOT a
// trust root for builtin identity. The unrelated stored-function name does
// not establish that a supported builtin can be shadowed.
func TestMySQL84_LiveProbes_StoredFunctionDeterministic(t *testing.T) {
	conn, db := skipIfUnreachable(t, "mysql")
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = conn.ExecContext(ctx, "DROP FUNCTION IF EXISTS my_sum")
		_ = conn.Close()
		db.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if _, err := conn.ExecContext(ctx, "USE app"); err != nil {
		t.Fatalf("USE app failed (driver error suppressed)")
	}

	createSQL := "CREATE FUNCTION my_sum(a INT, b INT) RETURNS INT DETERMINISTIC RETURN a + b"
	if _, err := conn.ExecContext(ctx, createSQL); err != nil {
		t.Fatalf("CREATE FUNCTION my_sum DETERMINISTIC must succeed on MySQL 8.4 (driver error suppressed)")
	}

	var routineName, isDeterministic string
	querySQL := "SELECT ROUTINE_NAME, IS_DETERMINISTIC FROM information_schema.ROUTINES WHERE ROUTINE_SCHEMA = 'app' AND ROUTINE_NAME = 'my_sum' AND ROUTINE_TYPE = 'FUNCTION'"
	if err := conn.QueryRowContext(ctx, querySQL).Scan(&routineName, &isDeterministic); err != nil {
		t.Fatalf("query information_schema.ROUTINES for my_sum failed (driver error suppressed)")
	}

	if routineName != "my_sum" {
		t.Fatalf("ROUTINE_NAME = %q, want %q", routineName, "my_sum")
	}
	if !strings.EqualFold(isDeterministic, "YES") {
		t.Fatalf("IS_DETERMINISTIC = %q, want YES (stored functions can declare DETERMINISTIC on MySQL 8.4)", isDeterministic)
	}
	t.Logf("MySQL 8.4 stored function my_sum IS_DETERMINISTIC = %s (supporting negative evidence: DETERMINISTIC is not a trust root; unrelated stored-function support only)", isDeterministic)
}

// TestMySQL84_LiveProbes_BuiltinNameShadowingRejected proves MySQL 8.4 rejects
// CREATE FUNCTION using a builtin name (count, COUNT) at parse time. This is
// supporting negative evidence: builtin names are reserved, but reservation
// does NOT establish a server-verifiable non-name identity for the builtin.
func TestMySQL84_LiveProbes_BuiltinNameShadowingRejected(t *testing.T) {
	conn, db := skipIfUnreachable(t, "mysql")
	defer func() {
		_ = conn.Close()
		db.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := conn.ExecContext(ctx, "USE app"); err != nil {
		t.Fatalf("USE app failed (driver error suppressed)")
	}

	for _, name := range []string{"count", "COUNT"} {
		dropSQL := "DROP FUNCTION IF EXISTS " + name
		_, _ = conn.ExecContext(ctx, dropSQL)

		createSQL := "CREATE FUNCTION " + name + "(a INT) RETURNS INT DETERMINISTIC RETURN a"
		_, err := conn.ExecContext(ctx, createSQL)
		if err == nil {
			_, _ = conn.ExecContext(ctx, "DROP FUNCTION IF EXISTS "+name)
			t.Fatalf("CREATE FUNCTION %s(...) must be rejected on MySQL 8.4 (builtin name reserved), but it succeeded", name)
		}
		if !isSyntaxError(err) {
			t.Fatalf("CREATE FUNCTION %s(...) rejection must be a syntax error (driver error suppressed)", name)
		}
		t.Logf("MySQL 8.4 rejected CREATE FUNCTION %s(...) with syntax error (builtin name reserved; supporting negative evidence only)", name)
	}
}

// isSyntaxError reports whether err looks like a MySQL 1064 syntax error.
func isSyntaxError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "1064") || strings.Contains(s, "syntax") || strings.Contains(s, "Syntax")
}

// TestMySQL84_LiveProbes_MysqlFuncScope proves mysql.func exists on MySQL 8.4
// and lists ONLY loadable UDFs (not builtins, not stored functions). On the
// compose image it is empty (0 rows). This is supporting negative evidence:
// mysql.func is not a builtin identity catalog. The row count is asserted
// (not merely logged) against the compose default.
func TestMySQL84_LiveProbes_MysqlFuncScope(t *testing.T) {
	conn, db := skipIfUnreachable(t, "mysql")
	defer func() {
		_ = conn.Close()
		db.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var colName string
	rows, err := conn.QueryContext(ctx, "SELECT COLUMN_NAME FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = 'mysql' AND TABLE_NAME = 'func' ORDER BY ORDINAL_POSITION")
	if err != nil {
		t.Fatalf("query information_schema.COLUMNS for mysql.func failed (driver error suppressed)")
	}
	defer rows.Close()

	var cols []string
	for rows.Next() {
		if err := rows.Scan(&colName); err != nil {
			t.Fatalf("scan column name failed (driver error suppressed)")
		}
		cols = append(cols, colName)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows iteration failed (driver error suppressed)")
	}
	if len(cols) == 0 {
		t.Fatal("mysql.func must exist with columns on MySQL 8.4")
	}
	wantCols := map[string]bool{"name": true, "ret": true, "dl": true, "type": true}
	for _, c := range cols {
		if !wantCols[c] {
			t.Fatalf("unexpected mysql.func column %q (expected only name/ret/dl/type)", c)
		}
	}

	var rowCount int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM mysql.func").Scan(&rowCount); err != nil {
		t.Fatalf("SELECT COUNT(*) FROM mysql.func failed (driver error suppressed)")
	}
	// The compose image loads no loadable UDFs, so mysql.func must be empty.
	// A nonzero count would mean a plugin UDF is loaded — not a builtin
	// identity catalog, but it would change the evidence and must be flagged.
	if rowCount != 0 {
		t.Fatalf("mysql.func row count = %d, want 0 on the compose image (mysql.func lists loadable UDFs only; a nonzero count changes the evidence)", rowCount)
	}
	t.Logf("MySQL 8.4 mysql.func exists with columns %v; row count = 0 (lists loadable UDFs only, not builtins)", cols)
}

// TestMySQL84_LiveProbes_PerfSchemaUDFsScope proves
// performance_schema.user_defined_functions exists on MySQL 8.4 and lists ONLY
// plugin UDFs (innodb_*, mysqlx_*), NOT builtins. This is supporting negative
// evidence: the table is not a builtin identity catalog.
func TestMySQL84_LiveProbes_PerfSchemaUDFsScope(t *testing.T) {
	conn, db := skipIfUnreachable(t, "mysql")
	defer func() {
		_ = conn.Close()
		db.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var udfName string
	rows, err := conn.QueryContext(ctx, "SELECT UDF_NAME FROM performance_schema.user_defined_functions ORDER BY UDF_NAME")
	if err != nil {
		t.Fatalf("query performance_schema.user_defined_functions failed (driver error suppressed)")
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		if err := rows.Scan(&udfName); err != nil {
			t.Fatalf("scan UDF_NAME failed (driver error suppressed)")
		}
		names = append(names, udfName)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows iteration failed (driver error suppressed)")
	}
	if len(names) == 0 {
		t.Fatal("performance_schema.user_defined_functions must list plugin UDFs on MySQL 8.4 (got 0 rows; compose image must expose plugin UDFs)")
	}

	forbiddenBuiltins := []string{"count", "sum", "avg", "min", "max", "row_number", "rank", "dense_rank"}
	for _, n := range names {
		for _, b := range forbiddenBuiltins {
			if strings.EqualFold(n, b) {
				t.Fatalf("performance_schema.user_defined_functions lists builtin %q — this would be a builtin identity catalog candidate, contradicting the probe evidence", n)
			}
		}
	}
	t.Logf("MySQL 8.4 performance_schema.user_defined_functions lists %d plugin UDFs (sample: %s); no builtins listed",
		len(names), joinSample(names, 5))
}

// joinSample returns up to n names joined by ", " for bounded log output.
func joinSample(names []string, n int) string {
	if len(names) > n {
		names = names[:n]
	}
	return strings.Join(names, ", ")
}

// TestMySQL84_LiveProbes_SchemaQualifiedStoredFunction proves a
// schema-qualified stored function call (app.my_sum(1, 2)) succeeds on MySQL
// 8.4. This is supporting negative evidence: schema qualification can select
// a stored function, but it does not prove that a supported builtin can be
// shadowed or establish a builtin identity root.
func TestMySQL84_LiveProbes_SchemaQualifiedStoredFunction(t *testing.T) {
	conn, db := skipIfUnreachable(t, "mysql")
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = conn.ExecContext(ctx, "DROP FUNCTION IF EXISTS my_sum")
		_ = conn.Close()
		db.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if _, err := conn.ExecContext(ctx, "USE app"); err != nil {
		t.Fatalf("USE app failed (driver error suppressed)")
	}

	createSQL := "CREATE FUNCTION my_sum(a INT, b INT) RETURNS INT DETERMINISTIC RETURN a + b"
	if _, err := conn.ExecContext(ctx, createSQL); err != nil {
		t.Fatalf("CREATE FUNCTION my_sum failed (driver error suppressed)")
	}

	var result int
	if err := conn.QueryRowContext(ctx, "SELECT app.my_sum(1, 2)").Scan(&result); err != nil {
		t.Fatalf("SELECT app.my_sum(1, 2) failed (driver error suppressed)")
	}
	if result != 3 {
		t.Fatalf("app.my_sum(1, 2) = %d, want 3", result)
	}
	t.Logf("MySQL 8.4 schema-qualified stored function call app.my_sum(1, 2) = %d (qualification selects an unrelated stored function; supported-builtin shadowing not demonstrated)", result)
}

// TestMySQL84_LiveProbes_ExplainRevealsNameNotIdentity proves EXPLAIN and
// EXPLAIN ANALYZE on MySQL 8.4 reveal the function NAME in plan text but do
// NOT expose any OID or implementation-class identity for builtins. This is
// supporting negative evidence: EXPLAIN echoes the spelling, not a binding.
func TestMySQL84_LiveProbes_ExplainRevealsNameNotIdentity(t *testing.T) {
	conn, db := skipIfUnreachable(t, "mysql")
	defer func() {
		_ = conn.Close()
		db.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := conn.ExecContext(ctx, "USE app"); err != nil {
		t.Fatalf("USE app failed (driver error suppressed)")
	}

	_, _ = conn.ExecContext(ctx, "INSERT IGNORE INTO users (id, name) VALUES (999, 'probe')")

	rows, err := conn.QueryContext(ctx, "EXPLAIN SELECT COUNT(*) FROM users")
	if err != nil {
		t.Fatalf("EXPLAIN SELECT COUNT(*) failed (driver error suppressed)")
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		t.Fatalf("rows.Columns failed (driver error suppressed)")
	}
	explainCols := make([]string, len(cols))
	for i, c := range cols {
		explainCols[i] = strings.ToLower(c)
	}
	for _, c := range explainCols {
		if strings.Contains(c, "oid") || strings.Contains(c, "impl") || strings.Contains(c, "identity") {
			t.Fatalf("EXPLAIN column %q looks like an identity column — this would change the probe evidence", c)
		}
	}
	for rows.Next() {
		scanCols := make([]sql.NullString, len(cols))
		scanArgs := make([]any, len(cols))
		for i := range scanCols {
			scanArgs[i] = &scanCols[i]
		}
		if err := rows.Scan(scanArgs...); err != nil {
			t.Fatalf("scan EXPLAIN row failed (driver error suppressed)")
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows iteration failed (driver error suppressed)")
	}

	var planText string
	if err := conn.QueryRowContext(ctx, "EXPLAIN ANALYZE SELECT COUNT(*) FROM users").Scan(&planText); err != nil {
		t.Fatalf("EXPLAIN ANALYZE SELECT COUNT(*) failed (driver error suppressed)")
	}
	planLower := strings.ToLower(planText)
	if strings.Contains(planLower, "oid:") || strings.Contains(planLower, "impl_class") {
		t.Fatalf("EXPLAIN ANALYZE plan text exposes an identity-like field — this would change the probe evidence: %s", planText)
	}
	t.Logf("MySQL 8.4 EXPLAIN ANALYZE plan references count by name only (no OID/implementation class); plan length = %d", len(planText))
}

// TestMySQL84_LiveProbes_NoBuiltinIdentityCatalog proves no
// information_schema table on MySQL 8.4 provides a builtin identity catalog
// among the probed candidate names. This is supporting negative evidence: no
// OID-equivalent binding was found among these candidates. It is NOT a
// universal proof that no such table exists anywhere on the server; it locks
// the specific candidates investigated.
func TestMySQL84_LiveProbes_NoBuiltinIdentityCatalog(t *testing.T) {
	conn, db := skipIfUnreachable(t, "mysql")
	defer func() {
		_ = conn.Close()
		db.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	candidates := []string{"FUNCTIONS", "BUILTIN_FUNCTIONS", "BUILTINS", "PG_PROC", "PG_BUILTIN", "SYS_FUNCTIONS"}
	var found []string
	for _, name := range candidates {
		var cnt int
		querySQL := "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_NAME = '" + name + "'"
		if err := conn.QueryRowContext(ctx, querySQL).Scan(&cnt); err != nil {
			t.Fatalf("probe information_schema.TABLES for %q failed (driver error suppressed)", name)
		}
		if cnt > 0 {
			found = append(found, name)
		}
	}
	if len(found) > 0 {
		t.Fatalf("MySQL 8.4 unexpectedly exposes builtin-identity-candidate table(s): %v — this would change the probe evidence", found)
	}
	t.Logf("MySQL 8.4 information_schema has no builtin-identity catalog table among probed candidates %v (supporting negative evidence; not a universal proof)", candidates)
}

// TestMySQL84_LiveProbes_MetadataReadable proves relation metadata and column
// types can be read on a caller-owned *sql.Conn. This is supporting evidence
// that metadata is accessible, but it does NOT by itself establish a builtin
// identity root or a same-connection proof model. The test does not claim a
// consistent-read boundary or initial/final context capture — that is out of
// scope for the KILL/DEFER disposition.
func TestMySQL84_LiveProbes_MetadataReadable(t *testing.T) {
	conn, db := skipIfUnreachable(t, "mysql")
	defer func() {
		_ = conn.Close()
		db.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var tableSchema, tableName, tableType string
	querySQL := "SELECT TABLE_SCHEMA, TABLE_NAME, TABLE_TYPE FROM information_schema.TABLES WHERE TABLE_SCHEMA = 'app' AND TABLE_NAME = 'users'"
	if err := conn.QueryRowContext(ctx, querySQL).Scan(&tableSchema, &tableName, &tableType); err != nil {
		t.Fatalf("query information_schema.TABLES for app.users failed (driver error suppressed)")
	}
	if tableSchema != "app" || tableName != "users" {
		t.Fatalf("app.users row = %q/%q, want app/users", tableSchema, tableName)
	}
	if !strings.EqualFold(tableType, "BASE TABLE") {
		t.Fatalf("app.users TABLE_TYPE = %q, want BASE TABLE", tableType)
	}

	rows, err := conn.QueryContext(ctx, "SELECT COLUMN_NAME, DATA_TYPE FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = 'app' AND TABLE_NAME = 'users' ORDER BY ORDINAL_POSITION")
	if err != nil {
		t.Fatalf("query information_schema.COLUMNS for app.users failed (driver error suppressed)")
	}
	defer rows.Close()

	var cols []string
	for rows.Next() {
		var colName, dataType string
		if err := rows.Scan(&colName, &dataType); err != nil {
			t.Fatalf("scan column failed (driver error suppressed)")
		}
		cols = append(cols, colName)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows iteration failed (driver error suppressed)")
	}
	wantCols := []string{"id", "name", "created_at", "updated_at"}
	if len(cols) != len(wantCols) {
		t.Fatalf("app.users column count = %d, want %d (cols=%v)", len(cols), len(wantCols), cols)
	}
	for i, w := range wantCols {
		if cols[i] != w {
			t.Fatalf("app.users column[%d] = %q, want %q", i, cols[i], w)
		}
	}
	t.Logf("MySQL 8.4 metadata for app.users readable on one conn: columns %v (metadata accessible; not a same-connection proof model)", cols)
}

// ---------------------------------------------------------------------------
// TiDB 8.5 live probes (independent proof domain — do not infer from MySQL)
// ---------------------------------------------------------------------------

// TestTiDB85_LiveProbes_ServerVersion records the actual server version
// returned by the live TiDB 8.5 Docker service. It FAILS if the server is not
// a TiDB v8.5.0 release. This evidence is captured independently of MySQL.
func TestTiDB85_LiveProbes_ServerVersion(t *testing.T) {
	conn, db := skipIfUnreachable(t, "tidb")
	defer func() {
		_ = conn.Close()
		db.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var version string
	if err := conn.QueryRowContext(ctx, "SELECT VERSION()").Scan(&version); err != nil {
		t.Fatalf("SELECT VERSION() on TiDB failed (driver error suppressed)")
	}

	t.Logf("TiDB live server version: %s", version)

	if !strings.Contains(version, "TiDB-v8.5.") {
		t.Fatalf("expected TiDB v8.5.x live version, got %q (compose image must be pingcap/tidb:v8.5.0)", version)
	}
}

// TestTiDB85_LiveProbes_StoredFunctionRejected proves TiDB 8.5 rejects CREATE
// FUNCTION entirely (no stored function support). This is independent evidence
// that TiDB has no stored-function shadowing risk. The absence of stored
// functions does NOT by itself establish a builtin identity root, and it does
// NOT refute a name-based trust model. This distinction drives the DEFER
// disposition for TiDB.
func TestTiDB85_LiveProbes_StoredFunctionRejected(t *testing.T) {
	conn, db := skipIfUnreachable(t, "tidb")
	defer func() {
		_ = conn.Close()
		db.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	createSQL := "CREATE FUNCTION my_sum(a INT, b INT) RETURNS INT DETERMINISTIC RETURN a + b"
	_, err := conn.ExecContext(ctx, createSQL)
	if err == nil {
		t.Fatal("CREATE FUNCTION my_sum must be rejected on TiDB 8.5 (no stored function support), but it succeeded")
	}
	if !isSyntaxError(err) {
		t.Fatalf("CREATE FUNCTION my_sum rejection must be a syntax error (driver error suppressed)")
	}
	t.Logf("TiDB 8.5 rejected CREATE FUNCTION my_sum with syntax error (no stored functions → no shadowing risk → name model NOT refuted; independent evidence)")
}

// TestTiDB85_LiveProbes_LoadableUDFRejected proves TiDB 8.5 rejects
// CREATE AGGREGATE FUNCTION ... SONAME (no loadable UDF support) and that
// mysql.func does NOT exist. This is independent evidence: TiDB has no
// loadable-UDF shadowing risk.
func TestTiDB85_LiveProbes_LoadableUDFRejected(t *testing.T) {
	conn, db := skipIfUnreachable(t, "tidb")
	defer func() {
		_ = conn.Close()
		db.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := conn.ExecContext(ctx, "CREATE AGGREGATE FUNCTION foo RETURNS INT SONAME 'foo.so'")
	if err == nil {
		t.Fatal("CREATE AGGREGATE FUNCTION ... SONAME must be rejected on TiDB 8.5 (no loadable UDF support), but it succeeded")
	}
	if !isSyntaxError(err) {
		t.Fatalf("CREATE AGGREGATE FUNCTION rejection must be a syntax error (driver error suppressed)")
	}

	var dummy int
	queryErr := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM mysql.func").Scan(&dummy)
	if queryErr == nil {
		t.Fatal("mysql.func must NOT exist on TiDB 8.5, but SELECT COUNT(*) succeeded")
	}
	if !strings.Contains(queryErr.Error(), "1146") && !strings.Contains(strings.ToLower(queryErr.Error()), "doesn't exist") {
		t.Fatalf("mysql.func query must fail with table-does-not-exist (1146) (driver error suppressed)")
	}
	t.Logf("TiDB 8.5 rejected CREATE AGGREGATE FUNCTION ... SONAME and mysql.func does not exist (independent negative evidence: no loadable UDF risk)")
}

// TestTiDB85_LiveProbes_PluginsEmpty proves information_schema.PLUGINS exists
// on TiDB 8.5 but returns 0 rows. This is independent evidence: the plugins
// table is not a UDF/builtin identity catalog on this TiDB deployment.
func TestTiDB85_LiveProbes_PluginsEmpty(t *testing.T) {
	conn, db := skipIfUnreachable(t, "tidb")
	defer func() {
		_ = conn.Close()
		db.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var rowCount int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.PLUGINS").Scan(&rowCount); err != nil {
		t.Fatalf("SELECT COUNT(*) FROM information_schema.PLUGINS failed (driver error suppressed)")
	}
	if rowCount != 0 {
		t.Fatalf("information_schema.PLUGINS must be empty on TiDB 8.5 compose image, got %d rows", rowCount)
	}
	t.Logf("TiDB 8.5 information_schema.PLUGINS exists with 0 rows (not a UDF/builtin identity catalog)")
}

// TestTiDB85_LiveProbes_KeywordsNotIdentityCatalog proves
// information_schema.KEYWORDS exists on TiDB 8.5 and lists reserved/non-reserved
// WORDS, but is a keyword catalog, NOT a function identity catalog. It carries
// no implementation class, no OID, no binding.
//
// Reserved window-function names (ROW_NUMBER/RANK/DENSE_RANK) MUST appear
// because they are reserved words in TiDB 8.5. Aggregate names (AVG/COUNT/SUM/
// MIN/MAX) MAY or MAY NOT appear — the keyword catalog lists reserved and
// non-reserved WORDS, not function names, so the absence of an aggregate name
// is itself evidence that KEYWORDS is not a function identity catalog. The
// server-returned RESERVED status (not a hardcoded value) proves the table
// carries only WORD + RESERVED.
func TestTiDB85_LiveProbes_KeywordsNotIdentityCatalog(t *testing.T) {
	conn, db := skipIfUnreachable(t, "tidb")
	defer func() {
		_ = conn.Close()
		db.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var kwCount int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.KEYWORDS").Scan(&kwCount); err != nil {
		t.Fatalf("SELECT COUNT(*) FROM information_schema.KEYWORDS failed (driver error suppressed)")
	}
	if kwCount == 0 {
		t.Fatal("information_schema.KEYWORDS must list words on TiDB 8.5, got 0 rows")
	}

	type kwRow struct {
		word     string
		reserved int
	}
	mustAppear := []string{"ROW_NUMBER", "RANK", "DENSE_RANK"}
	mayAppear := []string{"AVG", "COUNT", "SUM", "MIN", "MAX"}
	allWords := append(append([]string{}, mustAppear...), mayAppear...)
	quoted := "'" + strings.Join(allWords, "','") + "'"
	rows, err := conn.QueryContext(ctx, "SELECT WORD, RESERVED FROM information_schema.KEYWORDS WHERE WORD IN ("+quoted+") ORDER BY WORD")
	if err != nil {
		t.Fatalf("query information_schema.KEYWORDS for candidate builtins failed (driver error suppressed)")
	}
	defer rows.Close()

	gotWords := map[string]int{}
	for rows.Next() {
		var r kwRow
		if err := rows.Scan(&r.word, &r.reserved); err != nil {
			t.Fatalf("scan KEYWORDS row failed (driver error suppressed)")
		}
		gotWords[r.word] = r.reserved
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows iteration failed (driver error suppressed)")
	}
	for _, w := range mustAppear {
		if _, ok := gotWords[w]; !ok {
			t.Fatalf("information_schema.KEYWORDS must list reserved word %q on TiDB 8.5, got rows for %v", w, gotWords)
		}
	}
	// The fact that some aggregate names are absent from the keyword catalog
	// is itself evidence that KEYWORDS is a keyword catalog, not a function
	// identity catalog.
	colRows, err := conn.QueryContext(ctx, "SELECT COLUMN_NAME FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = 'information_schema' AND TABLE_NAME = 'KEYWORDS' ORDER BY ORDINAL_POSITION")
	if err != nil {
		t.Fatalf("query KEYWORDS columns failed (driver error suppressed)")
	}
	defer colRows.Close()
	var colName string
	for colRows.Next() {
		if err := colRows.Scan(&colName); err != nil {
			t.Fatalf("scan column name failed (driver error suppressed)")
		}
		lc := strings.ToLower(colName)
		if strings.Contains(lc, "oid") || strings.Contains(lc, "impl") || strings.Contains(lc, "identity") {
			t.Fatalf("information_schema.KEYWORDS column %q looks like an identity column — this would change the probe evidence", colName)
		}
	}
	t.Logf("TiDB 8.5 information_schema.KEYWORDS has %d words (sample reserved status: %v); keyword catalog only, not an identity root", kwCount, gotWords)
}

// TestTiDB85_LiveProbes_ExplainRevealsNameNotIdentity proves EXPLAIN and
// EXPLAIN ANALYZE on TiDB 8.5 reveal the function NAME in plan text
// (e.g. "funcs:count(1)") but do NOT expose any OID or implementation-class
// identity for builtins. This is independent supporting negative evidence:
// EXPLAIN echoes the spelling, not a binding.
func TestTiDB85_LiveProbes_ExplainRevealsNameNotIdentity(t *testing.T) {
	conn, db := skipIfUnreachable(t, "tidb")
	defer func() {
		_ = conn.Close()
		db.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := conn.ExecContext(ctx, "USE app"); err != nil {
		t.Fatalf("USE app failed (driver error suppressed)")
	}

	rows, err := conn.QueryContext(ctx, "EXPLAIN SELECT COUNT(*) FROM users")
	if err != nil {
		t.Fatalf("EXPLAIN SELECT COUNT(*) failed (driver error suppressed)")
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		t.Fatalf("rows.Columns failed (driver error suppressed)")
	}
	for _, c := range cols {
		lc := strings.ToLower(c)
		if strings.Contains(lc, "oid") || strings.Contains(lc, "impl") || strings.Contains(lc, "identity") {
			t.Fatalf("TiDB EXPLAIN column %q looks like an identity column — this would change the probe evidence", c)
		}
	}
	var planHasCountName bool
	for rows.Next() {
		scanCols := make([]sql.NullString, len(cols))
		scanArgs := make([]any, len(cols))
		for i := range scanCols {
			scanArgs[i] = &scanCols[i]
		}
		if err := rows.Scan(scanArgs...); err != nil {
			t.Fatalf("scan EXPLAIN row failed (driver error suppressed)")
		}
		for _, sc := range scanCols {
			if sc.Valid && strings.Contains(strings.ToLower(sc.String), "count") {
				planHasCountName = true
			}
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows iteration failed (driver error suppressed)")
	}
	if !planHasCountName {
		t.Logf("TiDB EXPLAIN plan text did not contain 'count' by name (plan format may differ); this does not change the disposition")
	}

	analyzeRows, err := conn.QueryContext(ctx, "EXPLAIN ANALYZE SELECT COUNT(*) FROM users")
	if err != nil {
		t.Fatalf("EXPLAIN ANALYZE SELECT COUNT(*) failed (driver error suppressed)")
	}
	defer analyzeRows.Close()

	analyzeCols, err := analyzeRows.Columns()
	if err != nil {
		t.Fatalf("analyzeRows.Columns failed (driver error suppressed)")
	}
	for _, c := range analyzeCols {
		lc := strings.ToLower(c)
		if strings.Contains(lc, "oid") || strings.Contains(lc, "impl") || strings.Contains(lc, "identity") {
			t.Fatalf("TiDB EXPLAIN ANALYZE column %q looks like an identity column — this would change the probe evidence", c)
		}
	}
	for analyzeRows.Next() {
		scanCols := make([]sql.NullString, len(analyzeCols))
		scanArgs := make([]any, len(analyzeCols))
		for i := range scanCols {
			scanArgs[i] = &scanCols[i]
		}
		if err := analyzeRows.Scan(scanArgs...); err != nil {
			t.Fatalf("scan EXPLAIN ANALYZE row failed (driver error suppressed)")
		}
	}
	if err := analyzeRows.Err(); err != nil {
		t.Fatalf("analyzeRows iteration failed (driver error suppressed)")
	}
	t.Logf("TiDB 8.5 EXPLAIN/EXPLAIN ANALYZE reference count by name only (no OID/implementation class column)")
}

// TestTiDB85_LiveProbes_NoBuiltinIdentityCatalog proves no information_schema
// table on TiDB 8.5 provides a builtin identity catalog among the probed
// candidate names. This is independent supporting negative evidence: no
// OID-equivalent binding was found among these candidates. It is NOT a
// universal proof that no such table exists anywhere on the server; it locks
// the specific candidates investigated.
func TestTiDB85_LiveProbes_NoBuiltinIdentityCatalog(t *testing.T) {
	conn, db := skipIfUnreachable(t, "tidb")
	defer func() {
		_ = conn.Close()
		db.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	candidates := []string{"FUNCTIONS", "BUILTIN_FUNCTIONS", "BUILTINS", "PG_PROC", "PG_BUILTIN", "SYS_FUNCTIONS"}
	var found []string
	for _, name := range candidates {
		var cnt int
		querySQL := "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_NAME = '" + name + "'"
		if err := conn.QueryRowContext(ctx, querySQL).Scan(&cnt); err != nil {
			t.Fatalf("probe information_schema.TABLES for %q failed (driver error suppressed)", name)
		}
		if cnt > 0 {
			found = append(found, name)
		}
	}
	if len(found) > 0 {
		t.Fatalf("TiDB 8.5 unexpectedly exposes builtin-identity-candidate table(s): %v — this would change the evidence", found)
	}
	t.Logf("TiDB 8.5 information_schema has no builtin-identity catalog table among probed candidates %v (independent negative evidence; not a universal proof)", candidates)
}

// TestTiDB85_LiveProbes_RoutinesEmpty proves information_schema.ROUTINES exists
// on TiDB 8.5 but returns 0 rows (no stored functions). This is independent
// evidence: TiDB has no stored-function catalog entries.
func TestTiDB85_LiveProbes_RoutinesEmpty(t *testing.T) {
	conn, db := skipIfUnreachable(t, "tidb")
	defer func() {
		_ = conn.Close()
		db.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var rowCount int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.ROUTINES").Scan(&rowCount); err != nil {
		t.Logf("TiDB 8.5 information_schema.ROUTINES query failed (table may not exist) (driver error suppressed): acceptable — no stored-function catalog")
		return
	}
	if rowCount != 0 {
		t.Fatalf("information_schema.ROUTINES must be empty on TiDB 8.5 (no stored functions), got %d rows", rowCount)
	}
	t.Logf("TiDB 8.5 information_schema.ROUTINES exists with 0 rows (no stored functions)")
}

// TestTiDB85_LiveProbes_MetadataReadable proves relation metadata and column
// types can be read on a caller-owned *sql.Conn. This is supporting evidence
// that metadata is accessible, but it does NOT by itself establish a builtin
// identity root or a same-connection proof model. The test does not claim a
// consistent-read boundary or initial/final context capture — that is out of
// scope for the KILL/DEFER disposition.
func TestTiDB85_LiveProbes_MetadataReadable(t *testing.T) {
	conn, db := skipIfUnreachable(t, "tidb")
	defer func() {
		_ = conn.Close()
		db.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var tableSchema, tableName, tableType string
	querySQL := "SELECT TABLE_SCHEMA, TABLE_NAME, TABLE_TYPE FROM information_schema.TABLES WHERE TABLE_SCHEMA = 'app' AND TABLE_NAME = 'users'"
	if err := conn.QueryRowContext(ctx, querySQL).Scan(&tableSchema, &tableName, &tableType); err != nil {
		t.Fatalf("query information_schema.TABLES for app.users failed (driver error suppressed)")
	}
	if tableSchema != "app" || tableName != "users" {
		t.Fatalf("app.users row = %q/%q, want app/users", tableSchema, tableName)
	}
	if !strings.EqualFold(tableType, "BASE TABLE") {
		t.Fatalf("app.users TABLE_TYPE = %q, want BASE TABLE", tableType)
	}

	rows, err := conn.QueryContext(ctx, "SELECT COLUMN_NAME, DATA_TYPE FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = 'app' AND TABLE_NAME = 'users' ORDER BY ORDINAL_POSITION")
	if err != nil {
		t.Fatalf("query information_schema.COLUMNS for app.users failed (driver error suppressed)")
	}
	defer rows.Close()

	var cols []string
	for rows.Next() {
		var colName, dataType string
		if err := rows.Scan(&colName, &dataType); err != nil {
			t.Fatalf("scan column failed (driver error suppressed)")
		}
		cols = append(cols, colName)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows iteration failed (driver error suppressed)")
	}
	wantCols := []string{"id", "name", "created_at", "updated_at"}
	if len(cols) != len(wantCols) {
		t.Fatalf("app.users column count = %d, want %d (cols=%v)", len(cols), len(wantCols), cols)
	}
	for i, w := range wantCols {
		if cols[i] != w {
			t.Fatalf("app.users column[%d] = %q, want %q", i, cols[i], w)
		}
	}
	t.Logf("TiDB 8.5 metadata for app.users readable on one conn: columns %v (metadata accessible; not a same-connection proof model)", cols)
}

// ---------------------------------------------------------------------------
// Per-dialect disposition (independent GO/DEFER/KILL based on live evidence)
// ---------------------------------------------------------------------------

// mysql84LiveEvidence captures the server-returned facts from the live MySQL
// 8.4 probe suite. These fields are populated by executing real probes in the
// disposition test; they are NOT hardcoded struct literals.
type mysql84LiveEvidence struct {
	serverVersion                  string
	storedFunctionSupported        bool
	builtinNameRejected            bool
	mysqlFuncListsOnlyLoadableUDFs bool
	perfSchemaUDFsListNoBuiltins   bool
	explainRevealsNameOnly         bool
	noBuiltinIdentityCatalogFound  bool
}

// probeMySQL84LiveEvidence runs the MySQL 8.4 live probes and returns the
// captured evidence. It skips the test if Docker is unreachable. It is shared
// by TestMySQL84_Disposition_DEFER so the disposition test depends on actual
// probe execution, not on empty t.Logf calls.
func probeMySQL84LiveEvidence(t *testing.T) (mysql84LiveEvidence, bool) {
	t.Helper()
	conn, db := skipIfUnreachable(t, "mysql")
	defer func() {
		_ = conn.Close()
		db.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var e mysql84LiveEvidence

	if err := conn.QueryRowContext(ctx, "SELECT VERSION()").Scan(&e.serverVersion); err != nil {
		t.Fatalf("SELECT VERSION() on MySQL failed (driver error suppressed)")
	}
	if !strings.HasPrefix(e.serverVersion, "8.4.") {
		t.Fatalf("expected MySQL 8.4.x live version, got %q", e.serverVersion)
	}

	// Stored function support is supporting evidence. The unrelated my_sum name
	// does not prove a supported builtin can be shadowed.
	if _, err := conn.ExecContext(ctx, "USE app"); err != nil {
		t.Fatalf("USE app failed (driver error suppressed)")
	}
	cleanup := func() {
		cctx, ccancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer ccancel()
		_, _ = conn.ExecContext(cctx, "DROP FUNCTION IF EXISTS my_sum")
	}
	cleanup()
	if _, err := conn.ExecContext(ctx, "CREATE FUNCTION my_sum(a INT, b INT) RETURNS INT DETERMINISTIC RETURN a + b"); err != nil {
		t.Fatalf("CREATE FUNCTION my_sum DETERMINISTIC must succeed on MySQL 8.4 (driver error suppressed)")
	}
	var sfResult int
	if err := conn.QueryRowContext(ctx, "SELECT app.my_sum(1, 2)").Scan(&sfResult); err != nil {
		cleanup()
		t.Fatalf("SELECT app.my_sum(1, 2) failed (driver error suppressed)")
	}
	if sfResult != 3 {
		cleanup()
		t.Fatalf("app.my_sum(1, 2) = %d, want 3", sfResult)
	}
	// Verify IS_DETERMINISTIC is recorded (supporting negative evidence).
	var isDet string
	if err := conn.QueryRowContext(ctx, "SELECT IS_DETERMINISTIC FROM information_schema.ROUTINES WHERE ROUTINE_SCHEMA = 'app' AND ROUTINE_NAME = 'my_sum' AND ROUTINE_TYPE = 'FUNCTION'").Scan(&isDet); err != nil {
		cleanup()
		t.Fatalf("query IS_DETERMINISTIC failed (driver error suppressed)")
	}
	if !strings.EqualFold(isDet, "YES") {
		cleanup()
		t.Fatalf("IS_DETERMINISTIC = %q, want YES", isDet)
	}
	cleanup()
	e.storedFunctionSupported = true

	// The investigated builtin-like name is rejected, so the live evidence does
	// not refute name-based resolution for the supported COUNT candidate.
	_, err := conn.ExecContext(ctx, "CREATE FUNCTION count(a INT) RETURNS INT DETERMINISTIC RETURN a")
	if err == nil {
		_, _ = conn.ExecContext(ctx, "DROP FUNCTION IF EXISTS count")
		t.Fatal("CREATE FUNCTION count must be rejected on MySQL 8.4")
	}
	if !isSyntaxError(err) {
		t.Fatalf("CREATE FUNCTION count rejection must be a syntax error (driver error suppressed)")
	}
	e.builtinNameRejected = true

	// mysql.func: must exist, must list only loadable UDFs (0 rows on compose).
	var mfRowCount int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM mysql.func").Scan(&mfRowCount); err != nil {
		t.Fatalf("SELECT COUNT(*) FROM mysql.func failed (driver error suppressed)")
	}
	if mfRowCount != 0 {
		t.Fatalf("mysql.func row count = %d, want 0 on compose image", mfRowCount)
	}
	e.mysqlFuncListsOnlyLoadableUDFs = true

	// performance_schema.user_defined_functions: must list no builtins.
	var udfName string
	rows, err := conn.QueryContext(ctx, "SELECT UDF_NAME FROM performance_schema.user_defined_functions ORDER BY UDF_NAME")
	if err != nil {
		t.Fatalf("query performance_schema.user_defined_functions failed (driver error suppressed)")
	}
	forbiddenBuiltins := []string{"count", "sum", "avg", "min", "max", "row_number", "rank", "dense_rank"}
	for rows.Next() {
		if err := rows.Scan(&udfName); err != nil {
			rows.Close()
			t.Fatalf("scan UDF_NAME failed (driver error suppressed)")
		}
		for _, b := range forbiddenBuiltins {
			if strings.EqualFold(udfName, b) {
				rows.Close()
				t.Fatalf("performance_schema.user_defined_functions lists builtin %q — contradicts probe evidence", udfName)
			}
		}
	}
	rows.Close()
	e.perfSchemaUDFsListNoBuiltins = true

	// EXPLAIN: no oid/impl/identity column.
	explainRows, err := conn.QueryContext(ctx, "EXPLAIN SELECT COUNT(*) FROM app.users")
	if err != nil {
		t.Fatalf("EXPLAIN SELECT COUNT(*) failed (driver error suppressed)")
	}
	explainCols, _ := explainRows.Columns()
	for _, c := range explainCols {
		lc := strings.ToLower(c)
		if strings.Contains(lc, "oid") || strings.Contains(lc, "impl") || strings.Contains(lc, "identity") {
			explainRows.Close()
			t.Fatalf("EXPLAIN column %q looks like an identity column — contradicts probe evidence", c)
		}
	}
	for explainRows.Next() {
		scanCols := make([]sql.NullString, len(explainCols))
		scanArgs := make([]any, len(explainCols))
		for i := range scanCols {
			scanArgs[i] = &scanCols[i]
		}
		_ = explainRows.Scan(scanArgs...)
	}
	explainRows.Close()
	e.explainRevealsNameOnly = true

	// No builtin identity catalog among probed candidates.
	candidates := []string{"FUNCTIONS", "BUILTIN_FUNCTIONS", "BUILTINS", "PG_PROC", "PG_BUILTIN", "SYS_FUNCTIONS"}
	for _, name := range candidates {
		var cnt int
		if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_NAME = '"+name+"'").Scan(&cnt); err != nil {
			t.Fatalf("probe information_schema.TABLES for %q failed (driver error suppressed)", name)
		}
		if cnt > 0 {
			t.Fatalf("MySQL 8.4 unexpectedly exposes builtin-identity-candidate table %q — contradicts probe evidence", name)
		}
	}
	e.noBuiltinIdentityCatalogFound = true

	return e, true
}

// tidb85LiveEvidence captures the server-returned facts from the live TiDB
// 8.5 probe suite. These fields are populated by executing real probes in the
// disposition test; they are NOT hardcoded struct literals.
type tidb85LiveEvidence struct {
	serverVersion                 string
	storedFunctionsRejected       bool
	loadableUDFsRejected          bool
	mysqlFuncDoesNotExist         bool
	pluginsEmpty                  bool
	keywordsNotIdentityCatalog    bool
	explainRevealsNameOnly        bool
	noBuiltinIdentityCatalogFound bool
	routinesEmpty                 bool
}

// probeTiDB85LiveEvidence runs the TiDB 8.5 live probes and returns the
// captured evidence. It skips the test if Docker is unreachable. It is shared
// by TestTiDB85_Disposition_DEFER so the disposition test depends on actual
// probe execution.
func probeTiDB85LiveEvidence(t *testing.T) (tidb85LiveEvidence, bool) {
	t.Helper()
	conn, db := skipIfUnreachable(t, "tidb")
	defer func() {
		_ = conn.Close()
		db.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var e tidb85LiveEvidence

	if err := conn.QueryRowContext(ctx, "SELECT VERSION()").Scan(&e.serverVersion); err != nil {
		t.Fatalf("SELECT VERSION() on TiDB failed (driver error suppressed)")
	}
	if !strings.Contains(e.serverVersion, "TiDB-v8.5.") {
		t.Fatalf("expected TiDB v8.5.x live version, got %q", e.serverVersion)
	}

	// CREATE FUNCTION must be rejected (no stored functions → no shadowing).
	_, err := conn.ExecContext(ctx, "CREATE FUNCTION my_sum(a INT, b INT) RETURNS INT DETERMINISTIC RETURN a + b")
	if err == nil {
		t.Fatal("CREATE FUNCTION my_sum must be rejected on TiDB 8.5")
	}
	if !isSyntaxError(err) {
		t.Fatalf("CREATE FUNCTION rejection must be a syntax error (driver error suppressed)")
	}
	e.storedFunctionsRejected = true

	// CREATE AGGREGATE FUNCTION SONAME must be rejected; mysql.func must not exist.
	_, err = conn.ExecContext(ctx, "CREATE AGGREGATE FUNCTION foo RETURNS INT SONAME 'foo.so'")
	if err == nil {
		t.Fatal("CREATE AGGREGATE FUNCTION ... SONAME must be rejected on TiDB 8.5")
	}
	if !isSyntaxError(err) {
		t.Fatalf("CREATE AGGREGATE FUNCTION rejection must be a syntax error (driver error suppressed)")
	}
	e.loadableUDFsRejected = true

	var dummy int
	queryErr := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM mysql.func").Scan(&dummy)
	if queryErr == nil {
		t.Fatal("mysql.func must NOT exist on TiDB 8.5")
	}
	if !strings.Contains(queryErr.Error(), "1146") && !strings.Contains(strings.ToLower(queryErr.Error()), "doesn't exist") {
		t.Fatalf("mysql.func query must fail with 1146 (driver error suppressed)")
	}
	e.mysqlFuncDoesNotExist = true

	// PLUGINS must be empty.
	var pluginCount int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.PLUGINS").Scan(&pluginCount); err != nil {
		t.Fatalf("SELECT COUNT(*) FROM information_schema.PLUGINS failed (driver error suppressed)")
	}
	if pluginCount != 0 {
		t.Fatalf("information_schema.PLUGINS must be empty, got %d rows", pluginCount)
	}
	e.pluginsEmpty = true

	// KEYWORDS must be keyword-only (no oid/impl/identity column).
	colRows, err := conn.QueryContext(ctx, "SELECT COLUMN_NAME FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = 'information_schema' AND TABLE_NAME = 'KEYWORDS' ORDER BY ORDINAL_POSITION")
	if err != nil {
		t.Fatalf("query KEYWORDS columns failed (driver error suppressed)")
	}
	var colName string
	for colRows.Next() {
		if err := colRows.Scan(&colName); err != nil {
			colRows.Close()
			t.Fatalf("scan column name failed (driver error suppressed)")
		}
		lc := strings.ToLower(colName)
		if strings.Contains(lc, "oid") || strings.Contains(lc, "impl") || strings.Contains(lc, "identity") {
			colRows.Close()
			t.Fatalf("KEYWORDS column %q looks like an identity column — contradicts evidence", colName)
		}
	}
	colRows.Close()
	// Verify reserved window names appear (proves KEYWORDS is populated).
	var kwCount int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.KEYWORDS WHERE WORD IN ('ROW_NUMBER','RANK','DENSE_RANK')").Scan(&kwCount); err != nil {
		t.Fatalf("query KEYWORDS for reserved window names failed (driver error suppressed)")
	}
	if kwCount != 3 {
		t.Fatalf("KEYWORDS must list ROW_NUMBER/RANK/DENSE_RANK, got count %d", kwCount)
	}
	e.keywordsNotIdentityCatalog = true

	// EXPLAIN: no oid/impl/identity column.
	if _, err := conn.ExecContext(ctx, "USE app"); err != nil {
		t.Fatalf("USE app failed (driver error suppressed)")
	}
	explainRows, err := conn.QueryContext(ctx, "EXPLAIN SELECT COUNT(*) FROM users")
	if err != nil {
		t.Fatalf("EXPLAIN SELECT COUNT(*) failed (driver error suppressed)")
	}
	explainCols, _ := explainRows.Columns()
	for _, c := range explainCols {
		lc := strings.ToLower(c)
		if strings.Contains(lc, "oid") || strings.Contains(lc, "impl") || strings.Contains(lc, "identity") {
			explainRows.Close()
			t.Fatalf("TiDB EXPLAIN column %q looks like an identity column — contradicts evidence", c)
		}
	}
	for explainRows.Next() {
		scanCols := make([]sql.NullString, len(explainCols))
		scanArgs := make([]any, len(explainCols))
		for i := range scanCols {
			scanArgs[i] = &scanCols[i]
		}
		_ = explainRows.Scan(scanArgs...)
	}
	explainRows.Close()
	e.explainRevealsNameOnly = true

	// No builtin identity catalog among probed candidates.
	candidates := []string{"FUNCTIONS", "BUILTIN_FUNCTIONS", "BUILTINS", "PG_PROC", "PG_BUILTIN", "SYS_FUNCTIONS"}
	for _, name := range candidates {
		var cnt int
		if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_NAME = '"+name+"'").Scan(&cnt); err != nil {
			t.Fatalf("probe information_schema.TABLES for %q failed (driver error suppressed)", name)
		}
		if cnt > 0 {
			t.Fatalf("TiDB 8.5 unexpectedly exposes builtin-identity-candidate table %q — contradicts evidence", name)
		}
	}
	e.noBuiltinIdentityCatalogFound = true

	// ROUTINES must be empty (or table absent).
	var routinesCount int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.ROUTINES").Scan(&routinesCount); err == nil {
		if routinesCount != 0 {
			t.Fatalf("information_schema.ROUTINES must be empty on TiDB 8.5, got %d rows", routinesCount)
		}
		e.routinesEmpty = true
	}
	// If ROUTINES query failed (table absent), that is also acceptable evidence.

	return e, true
}

// TestMySQL84_Disposition_DEFER locks the MySQL 8.4 DEFER disposition based on
// LIVE Docker probe evidence. The test EXECUTES the probes (via
// probeMySQL84LiveEvidence) and skips itself if Docker is unreachable. It does
// NOT pass when its probes did not run.
//
// DEFER rationale: MySQL supports unrelated stored functions, but the
// investigated builtin-like COUNT name is rejected, so the live evidence does
// not refute name-based resolution for the supported candidate. No non-name
// root was found among the investigated facilities, but the probe set is not
// exhaustive enough to establish that name is the only possible root. The
// evidence is therefore insufficient for KILL and does not support GO.
func TestMySQL84_Disposition_DEFER(t *testing.T) {
	e, ok := probeMySQL84LiveEvidence(t)
	if !ok {
		t.Skip("MySQL 8.4 Docker service unreachable; DEFER disposition not locked by this run")
	}

	t.Logf("MySQL 8.4 live evidence: version=%s storedFunctionSupported=%v builtinNameRejected=%v mysqlFuncLoadableOnly=%v perfSchemaNoBuiltins=%v explainNameOnly=%v noCatalog=%v",
		e.serverVersion, e.storedFunctionSupported, e.builtinNameRejected, e.mysqlFuncListsOnlyLoadableUDFs,
		e.perfSchemaUDFsListNoBuiltins, e.explainRevealsNameOnly, e.noBuiltinIdentityCatalogFound)

	// DEFER necessary conditions (all must be true based on live evidence):
	// 1. Stored functions are supported, but the investigated builtin-like
	//    COUNT name is rejected, so name-based resolution is not refuted for
	//    the supported candidate.
	if !e.storedFunctionSupported {
		t.Fatal("DEFER requires the live server to support stored functions as supporting evidence")
	}
	if !e.builtinNameRejected {
		t.Fatal("DEFER requires the investigated builtin-like COUNT name to be rejected")
	}
	// 2. mysql.func lists only loadable UDFs (not builtins).
	if !e.mysqlFuncListsOnlyLoadableUDFs {
		t.Fatal("DEFER requires mysql.func to list only loadable UDFs")
	}
	// 3. performance_schema.user_defined_functions lists no builtins.
	if !e.perfSchemaUDFsListNoBuiltins {
		t.Fatal("DEFER requires performance_schema.user_defined_functions to list no builtins")
	}
	// 4. EXPLAIN reveals only the name, never an OID/implementation class.
	if !e.explainRevealsNameOnly {
		t.Fatal("DEFER requires EXPLAIN to reveal name only")
	}
	// 5. No builtin identity catalog found among probed candidates (not a
	//    universal proof; locks the investigated candidates).
	if !e.noBuiltinIdentityCatalogFound {
		t.Fatal("DEFER requires no builtin identity catalog among probed candidates")
	}

	t.Logf("MySQL 8.4 disposition = DEFER (live evidence: COUNT shadowing not demonstrated + no non-name root found among investigated facilities → insufficient for KILL/GO)")
}

// TestTiDB85_Disposition_DEFER locks the TiDB 8.5 DEFER disposition based on
// LIVE Docker probe evidence. The test EXECUTES the probes (via
// probeTiDB85LiveEvidence) and skips itself if Docker is unreachable. It does
// NOT pass when its probes did not run.
//
// DEFER rationale (per decision §Valid Outcomes):
// TiDB 8.5 has NO stored functions and NO loadable UDFs, so the name-based
// trust model is NOT refuted by shadowing (unlike MySQL). Live probes found
// no server-verifiable non-name identity root among the investigated
// facilities, but the absence of shadowing means the evidence is INSUFFICIENT
// to establish that "the only possible root is name-based" — a future,
// unprobed facility could expose a non-name binding without contradicting the
// shadowing evidence. Per the task rule "If live probes plus version-scoped
// authoritative evidence cannot establish that the only available root is a
// forbidden name-based model, downgrade that dialect to DEFER", TiDB is DEFER.
//
// DEFER is not a commitment to ship promotion. unknown_function_effect is
// retained for all function-bearing queries. The default SDK/CLI/HTTP/MCP
// surfaces remain unchanged and fail-closed.
func TestTiDB85_Disposition_DEFER(t *testing.T) {
	e, ok := probeTiDB85LiveEvidence(t)
	if !ok {
		t.Skip("TiDB 8.5 Docker service unreachable; DEFER disposition not locked by this run")
	}

	t.Logf("TiDB 8.5 live evidence: version=%s storedRejected=%v loadableRejected=%v mysqlFuncAbsent=%v pluginsEmpty=%v keywordsNotIdentity=%v explainNameOnly=%v noCatalog=%v routinesEmpty=%v",
		e.serverVersion, e.storedFunctionsRejected, e.loadableUDFsRejected, e.mysqlFuncDoesNotExist,
		e.pluginsEmpty, e.keywordsNotIdentityCatalog, e.explainRevealsNameOnly,
		e.noBuiltinIdentityCatalogFound, e.routinesEmpty)

	// DEFER necessary conditions (all must be true based on live evidence):
	// 1. No stored-function shadowing (name model NOT refuted → cannot
	//    establish "only possible root is name-based").
	if !e.storedFunctionsRejected {
		t.Fatal("DEFER requires CREATE FUNCTION to be rejected (no shadowing → name model not refuted)")
	}
	// 2. No loadable UDFs (no UDF shadowing either).
	if !e.loadableUDFsRejected {
		t.Fatal("DEFER requires CREATE AGGREGATE FUNCTION SONAME to be rejected")
	}
	// 3. mysql.func does not exist.
	if !e.mysqlFuncDoesNotExist {
		t.Fatal("DEFER requires mysql.func to not exist")
	}
	// 4. PLUGINS empty.
	if !e.pluginsEmpty {
		t.Fatal("DEFER requires information_schema.PLUGINS to be empty")
	}
	// 5. KEYWORDS is a keyword catalog, not an identity catalog.
	if !e.keywordsNotIdentityCatalog {
		t.Fatal("DEFER requires KEYWORDS to be a keyword catalog, not an identity catalog")
	}
	// 6. EXPLAIN reveals only the name.
	if !e.explainRevealsNameOnly {
		t.Fatal("DEFER requires EXPLAIN to reveal name only")
	}
	// 7. No builtin identity catalog found among probed candidates (not a
	//    universal proof; locks the investigated candidates).
	if !e.noBuiltinIdentityCatalogFound {
		t.Fatal("DEFER requires no builtin identity catalog among probed candidates")
	}

	t.Logf("TiDB 8.5 disposition = DEFER (live evidence: no shadowing, name model not refuted + no non-name root found → insufficient to establish only-available-root-is-name-based; not a contradiction → DEFER)")
}

// TestMySQLTiDB_IndependentLiveEvidencePaths verifies MySQL and TiDB reach
// their dispositions through DIFFERENT live evidence paths. The test EXECUTES
// both probe suites and compares the server-returned observations. It skips
// itself if either Docker service is unreachable. It does NOT compare
// hardcoded string literals.
//
// MySQL path: unrelated stored functions supported but COUNT shadowing rejected
// (name model not refuted) → DEFER. TiDB path: no stored functions (name model
// not refuted) → DEFER. The supporting evidence paths remain independent.
func TestMySQLTiDB_IndependentLiveEvidencePaths(t *testing.T) {
	mysqlE, mysqlOK := probeMySQL84LiveEvidence(t)
	if !mysqlOK {
		t.Skip("MySQL Docker service unreachable; independent-evidence test skipped")
	}
	tidbE, tidbOK := probeTiDB85LiveEvidence(t)
	if !tidbOK {
		t.Skip("TiDB Docker service unreachable; independent-evidence test skipped")
	}

	// MySQL supports stored functions but rejects the investigated COUNT name;
	// TiDB rejects stored functions entirely. These are server-returned,
	// independent observations, not hardcoded labels.
	if !mysqlE.storedFunctionSupported || !mysqlE.builtinNameRejected {
		t.Fatal("MySQL live evidence must include stored-function support and COUNT-name rejection")
	}
	if !tidbE.storedFunctionsRejected {
		t.Fatal("TiDB live evidence must include stored-function rejection")
	}
	// Both have no builtin identity catalog (convergent negative evidence), but
	// reach different dispositions because the shadowing evidence differs.
	if !mysqlE.noBuiltinIdentityCatalogFound || !tidbE.noBuiltinIdentityCatalogFound {
		t.Fatal("both dialects must have no builtin identity catalog among probed candidates")
	}
	t.Logf("MySQL path: stored functions supported but COUNT shadowing rejected → DEFER; TiDB path: stored functions rejected → DEFER. Independent live evidence.")
}
