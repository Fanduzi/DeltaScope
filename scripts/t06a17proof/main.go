// Package main collects the bounded T06-A17-R1 native catalog/provider proof.
// input: one of four pinned loopback anchors, environment-only credentials, and a new evidence directory
// output: complete raw observations, query ledgers, strict pairs, and owned-table cleanup results; any failure is nonzero
// pos: task-specific proof command outside the product audit and Golden runner
// note: if this file changes, update this header and module README.md.
package main

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	mysqlmeta "github.com/Fanduzi/DeltaScope/internal/infrastructure/metadata/mysql"
	gomysql "github.com/go-sql-driver/mysql"
)

const countSQL = "select count(*) from information_schema.tables where table_schema='golden' and table_name='t06_a17_type_facts'"
const catalogSQL = "select column_name, ordinal_position, data_type, column_type, hex(column_type), numeric_precision, numeric_scale, character_set_name, collation_name from information_schema.columns where table_schema='golden' and table_name='t06_a17_type_facts' order by ordinal_position"
const dropSQL = "DROP TABLE golden.t06_a17_type_facts"

var sessionQueries = []struct{ name, query string }{
	{"version", "select version()"},
	{"character_set_client", "select @@character_set_client"},
	{"character_set_connection", "select @@character_set_connection"},
	{"character_set_results", "select @@character_set_results"},
	{"collation_connection", "select @@collation_connection"},
	{"sql_mode", "select @@sql_mode"},
}

type ledgerEntry struct {
	Op    string `json:"op"`
	Query string `json:"query"`
}

type queryLedger struct {
	mu      sync.Mutex
	entries []ledgerEntry
}

func (l *queryLedger) add(op, query string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, ledgerEntry{op, query})
}

func (l *queryLedger) snapshot() []ledgerEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]ledgerEntry(nil), l.entries...)
}

type ledgerConnector struct {
	inner  driver.Connector
	ledger *queryLedger
}

func (c *ledgerConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.inner.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &ledgerConn{conn, c.ledger}, nil
}

func (c *ledgerConnector) Driver() driver.Driver { return c.inner.Driver() }

type ledgerConn struct {
	conn   driver.Conn
	ledger *queryLedger
}

func (c *ledgerConn) Prepare(query string) (driver.Stmt, error) {
	stmt, err := c.conn.Prepare(query)
	if err != nil {
		return nil, err
	}
	return &ledgerStmt{stmt, query, c.ledger}, nil
}

func (c *ledgerConn) Close() error              { return c.conn.Close() }
func (c *ledgerConn) Begin() (driver.Tx, error) { return c.conn.Begin() }

func (c *ledgerConn) Ping(ctx context.Context) error {
	if pinger, ok := c.conn.(driver.Pinger); ok {
		return pinger.Ping(ctx)
	}
	return nil
}

func (c *ledgerConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	c.ledger.add("query", query)
	if querier, ok := c.conn.(driver.QueryerContext); ok {
		return querier.QueryContext(ctx, query, args)
	}
	return nil, driver.ErrSkip
}

func (c *ledgerConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	c.ledger.add("exec", query)
	if execer, ok := c.conn.(driver.ExecerContext); ok {
		return execer.ExecContext(ctx, query, args)
	}
	return nil, driver.ErrSkip
}

type ledgerStmt struct {
	stmt   driver.Stmt
	query  string
	ledger *queryLedger
}

func (s *ledgerStmt) Close() error  { return s.stmt.Close() }
func (s *ledgerStmt) NumInput() int { return s.stmt.NumInput() }
func (s *ledgerStmt) Exec(args []driver.Value) (driver.Result, error) {
	s.ledger.add("stmt-exec", s.query)
	return s.stmt.Exec(args)
}
func (s *ledgerStmt) Query(args []driver.Value) (driver.Rows, error) {
	s.ledger.add("stmt-query", s.query)
	return s.stmt.Query(args)
}

type evidenceWriter func(string, []byte) error

func writeJSON(write evidenceWriter, name string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", name, err)
	}
	if err := write(name, append(data, '\n')); err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	return nil
}

func tableCount(ctx context.Context, db *sql.DB) (int, error) {
	var count int
	if err := db.QueryRowContext(ctx, countSQL).Scan(&count); err != nil {
		return 0, fmt.Errorf("table COUNT query/scan: %w", err)
	}
	return count, nil
}

func readCatalog(ctx context.Context, db *sql.DB) (result []catalogRow, err error) {
	rows, err := db.QueryContext(ctx, catalogSQL)
	if err != nil {
		return nil, fmt.Errorf("query catalog: %w", err)
	}
	defer func() { err = errors.Join(err, rows.Close()) }()
	for rows.Next() {
		var row catalogRow
		var precision, scale sql.NullInt64
		var charset, collation sql.NullString
		if err := rows.Scan(&row.ColumnName, &row.OrdinalPosition, &row.DataType, &row.ColumnType, &row.ColumnTypeHex, &precision, &scale, &charset, &collation); err != nil {
			return result, fmt.Errorf("scan catalog row %d: %w", len(result)+1, err)
		}
		if precision.Valid {
			row.NumericPrecision = &precision.Int64
		}
		if scale.Valid {
			row.NumericScale = &scale.Int64
		}
		if charset.Valid {
			row.CharacterSet = &charset.String
		}
		if collation.Valid {
			row.Collation = &collation.String
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return result, fmt.Errorf("iterate catalog after %d rows: %w", len(result), err)
	}
	return result, nil
}

type lifecycle struct {
	PreCount         *int   `json:"pre_create_count"`
	Created          bool   `json:"created_by_this_probe"`
	CleanupAttempted bool   `json:"cleanup_attempted"`
	DropSucceeded    bool   `json:"drop_succeeded"`
	PostCount        *int   `json:"post_drop_count"`
	PrimaryError     string `json:"primary_error,omitempty"`
	CleanupError     string `json:"cleanup_error,omitempty"`
}

func collect(ctx context.Context, db *sql.DB, target anchor, ledger *queryLedger, write evidenceWriter) (err error) {
	life := lifecycle{}
	defer func() {
		if err != nil {
			life.PrimaryError = err.Error()
		}
		if life.Created {
			life.CleanupAttempted = true
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			_, dropErr := db.ExecContext(cleanupCtx, dropSQL)
			life.DropSucceeded = dropErr == nil
			if dropErr != nil {
				dropErr = fmt.Errorf("cleanup DROP: %w", dropErr)
			}
			count, countErr := tableCount(cleanupCtx, db)
			if countErr == nil {
				life.PostCount = &count
				countErr = writeJSON(write, "post-drop-count.json", map[string]int{"count": count})
				if count != 0 {
					countErr = errors.Join(countErr, fmt.Errorf("cleanup final COUNT=%d, want 0", count))
				}
			} else {
				countErr = fmt.Errorf("cleanup final COUNT: %w", countErr)
			}
			cleanupErr := errors.Join(dropErr, countErr)
			if cleanupErr != nil {
				life.CleanupError = cleanupErr.Error()
				err = errors.Join(err, cleanupErr)
			}
		}
		err = errors.Join(err, writeJSON(write, "lifecycle.json", life))
		err = errors.Join(err, writeJSON(write, "query-ledger.json", ledger.snapshot()))
		result := struct {
			Anchor  string `json:"anchor"`
			Success bool   `json:"success"`
			Error   string `json:"error,omitempty"`
		}{Anchor: target.Name, Success: err == nil}
		if err != nil {
			result.Error = err.Error()
		}
		err = errors.Join(err, writeJSON(write, "run-result.json", result))
	}()
	if err := write("create.sql", []byte(createSQL+"\n")); err != nil {
		return fmt.Errorf("write create.sql: %w", err)
	}
	session := make(map[string]string, len(sessionQueries))
	for _, fact := range sessionQueries {
		var value string
		if err := db.QueryRowContext(ctx, fact.query).Scan(&value); err != nil {
			return fmt.Errorf("read session %s: %w", fact.name, err)
		}
		session[fact.name] = value
	}
	if err := writeJSON(write, "session-facts.json", session); err != nil {
		return err
	}
	if err := validateSession(session, target); err != nil {
		return err
	}
	count, err := tableCount(ctx, db)
	if err != nil {
		return fmt.Errorf("pre-create: %w", err)
	}
	life.PreCount = &count
	if err := writeJSON(write, "pre-create-count.json", map[string]int{"count": count}); err != nil {
		return err
	}
	if count != 0 {
		return fmt.Errorf("pre-create COUNT=%d: existing table is not owned by this probe", count)
	}
	if _, err := db.ExecContext(ctx, "CREATE DATABASE IF NOT EXISTS golden"); err != nil {
		return fmt.Errorf("create fixture schema: %w", err)
	}
	if _, err := db.ExecContext(ctx, createSQL); err != nil {
		return fmt.Errorf("create fixture table: %w", err)
	}
	life.Created = true
	catalog, readErr := readCatalog(ctx, db)
	writeErr := writeJSON(write, "catalog-rows.json", catalog)
	if err := errors.Join(readErr, writeErr); err != nil {
		return err
	}
	if err := validateCatalog(ctx, catalog); err != nil {
		return err
	}
	providerStart := len(ledger.snapshot())
	snapshot, providerErr := mysqlmeta.NewProvider(db).LoadTableSnapshot(ctx, target.Dialect, "golden", "t06_a17_type_facts")
	providerEnd := ledger.snapshot()
	writeErr = errors.Join(
		writeJSON(write, "provider-query-ledger.json", providerEnd[providerStart:]),
		writeJSON(write, "provider-snapshot.json", snapshot),
	)
	if err := errors.Join(providerErr, writeErr); err != nil {
		return fmt.Errorf("provider capture: %w", err)
	}
	pairs, err := pairCatalogProvider(catalog, snapshot, target)
	if err != nil {
		return err
	}
	return writeJSON(write, "catalog-provider-pairs.json", pairs)
}

func execute(args []string) (err error) {
	if len(args) != 2 {
		return fmt.Errorf("usage: t06a17proof <external-output-root> <mysql57|mysql80|mysql84|tidb85>")
	}
	target, err := anchorFor(args[1])
	if err != nil {
		return err
	}
	if os.Getenv("A17_USER") == "" {
		return fmt.Errorf("A17_USER is required; credentials must be passed through the environment")
	}
	if err := os.MkdirAll(args[0], 0o755); err != nil {
		return fmt.Errorf("create output root: %w", err)
	}
	directory := filepath.Join(args[0], target.Name)
	if err := os.Mkdir(directory, 0o755); err != nil {
		return fmt.Errorf("create new anchor evidence directory: %w", err)
	}
	write := func(name string, data []byte) error {
		return os.WriteFile(filepath.Join(directory, name), data, 0o644)
	}
	cfg := gomysql.NewConfig()
	cfg.Net = "tcp"
	cfg.Addr = "127.0.0.1:" + target.Port
	cfg.User = os.Getenv("A17_USER")
	cfg.Passwd = os.Getenv("A17_PASS")
	cfg.InterpolateParams = true
	cfg.Timeout = 30 * time.Second
	connector, err := gomysql.NewConnector(cfg)
	if err != nil {
		return fmt.Errorf("create connector: %w", err)
	}
	ledger := &queryLedger{}
	db := sql.OpenDB(&ledgerConnector{connector, ledger})
	db.SetMaxOpenConns(1)
	defer func() { err = errors.Join(err, db.Close()) }()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping anchor: %w", err)
	}
	if err := collect(ctx, db, target, ledger, write); err != nil {
		return err
	}
	fmt.Printf("anchor=%s columns=24 target_pairs=21 ordinary_controls=3 pre_count=0 post_count=0\n", target.Name)
	return nil
}

func main() {
	if err := execute(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
