// Package main verifies bounded failures in the T06-A17-R1 proof collector.
package main

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var fixtureTypes = []string{
	"decimal(12,2)", "decimal(12,3)", "decimal(12,0)", "decimal(12,0)", "decimal(10,0)",
	"float(10,2)", "float(10,3)", "float(10,0)", "float", "double(16,2)", "double(16,4)", "double(16,0)", "double",
	"enum('Ready','Closed')", "enum('ready','closed')", "enum('Closed','Ready')",
	"set('Read','Write')", "set('read','write')", "set('Write','Read')",
	"enum('','A,B','A)B','O''Reilly')", "set('Read','A)B','O''Reilly')", "int", "varchar(8)", "bigint unsigned",
}

type faultFixture struct {
	catalog         [][]driver.Value
	provider        [][]driver.Value
	preCount        driver.Value
	preError        error
	postError       error
	catalogErrorAt  int
	providerAbsent  bool
	dropError       error
	created         bool
	creates         int
	drops           int
	counts          int
	providerQueries int
}

func newFaultFixture() *faultFixture {
	fixture := &faultFixture{preCount: int64(0), catalogErrorAt: -1}
	for i, name := range columnNames {
		var precision, scale, charset, collation driver.Value
		if i < 5 {
			precision, scale = int64(12), int64(0)
			if i == 0 {
				scale = int64(2)
			} else if i == 1 {
				scale = int64(3)
			} else if i == 4 {
				precision = int64(10)
			}
		}
		if _, ok := memberFacts[name]; ok || name == "v" {
			charset, collation = "utf8mb4", "utf8mb4_bin"
		}
		fixture.catalog = append(fixture.catalog, []driver.Value{name, int64(i + 1), columnFamilies[i], fixtureTypes[i], strings.ToUpper(hex.EncodeToString([]byte(fixtureTypes[i]))), precision, scale, charset, collation})
		fixture.provider = append(fixture.provider, []driver.Value{name, fixtureTypes[i], charset, collation, "", nil, "YES", "", nil})
	}
	return fixture
}

type faultConnector struct{ fixture *faultFixture }

type faultDriver struct{}

func (faultDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("use the test connector")
}
func (c faultConnector) Driver() driver.Driver { return faultDriver{} }
func (c faultConnector) Connect(context.Context) (driver.Conn, error) {
	return &faultConn{c.fixture}, nil
}

type faultConn struct{ fixture *faultFixture }

func (c *faultConn) Close() error { return nil }
func (c *faultConn) Begin() (driver.Tx, error) {
	return nil, errors.New("transactions are outside the proof")
}
func (c *faultConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected prepared query")
}

func (c *faultConn) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	f := c.fixture
	switch query {
	case "CREATE DATABASE IF NOT EXISTS golden":
	case createSQL:
		f.creates++
		f.created = true
	case dropSQL:
		f.drops++
		if f.dropError != nil {
			return nil, f.dropError
		}
		f.created = false
	default:
		return nil, fmt.Errorf("unexpected exec %q", query)
	}
	return driver.RowsAffected(0), nil
}

func (c *faultConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	f := c.fixture
	if query == countSQL {
		f.counts++
		if f.counts == 1 {
			if f.preError != nil {
				return nil, f.preError
			}
			return newFaultRows([]string{"count"}, [][]driver.Value{{f.preCount}}, -1), nil
		}
		if f.postError != nil {
			return nil, f.postError
		}
		value := int64(0)
		if f.created {
			value = 1
		}
		return newFaultRows([]string{"count"}, [][]driver.Value{{value}}, -1), nil
	}
	for _, fact := range sessionQueries {
		if query == fact.query {
			value := "utf8mb4"
			switch fact.name {
			case "version":
				value = "8.4.10"
			case "collation_connection":
				value = "utf8mb4_general_ci"
			case "sql_mode":
				value = "STRICT_TRANS_TABLES"
			}
			return newFaultRows([]string{fact.name}, [][]driver.Value{{value}}, -1), nil
		}
	}
	if query == catalogSQL {
		return newFaultRows([]string{"column_name", "ordinal_position", "data_type", "column_type", "hex(column_type)", "numeric_precision", "numeric_scale", "character_set_name", "collation_name"}, f.catalog, f.catalogErrorAt), nil
	}
	if strings.Contains(query, "from information_schema.tables") {
		f.providerQueries++
		rows := [][]driver.Value{{"InnoDB", "utf8mb4_bin", "", nil, "Dynamic", int64(0)}}
		if f.providerAbsent {
			rows = nil
		}
		return newFaultRows([]string{"engine", "table_collation", "table_comment", "auto_increment", "row_format", "table_rows"}, rows, -1), nil
	}
	if strings.Contains(query, "from information_schema.columns") {
		f.providerQueries++
		return newFaultRows([]string{"column_name", "column_type", "character_set_name", "collation_name", "column_comment", "column_default", "is_nullable", "extra", "generation_expression"}, f.provider, -1), nil
	}
	if strings.Contains(query, "constraint_type = 'CHECK'") {
		f.providerQueries++
		// T12 CHECK member projection — this fixture stores no checks.
		return newFaultRows([]string{"constraint_name", "enforced", "check_clause"}, nil, -1), nil
	}
	if strings.Contains(query, "table_name = 'CHECK_CONSTRAINTS'") || strings.Contains(query, "table_name = 'TIDB_CHECK_CONSTRAINTS'") || strings.Contains(query, "column_name = 'ENFORCED'") {
		f.providerQueries++
		// T12 catalog capability probes — this fixture predates them.
		return newFaultRows([]string{"count(*)"}, [][]driver.Value{{int64(0)}}, -1), nil
	}
	if strings.Contains(query, "from information_schema.`columns`") {
		f.providerQueries++
		// IS_VISIBLE capability probe: this fixture models a catalog without
		// the column (MySQL 5.7 shape) — a zero count keeps the index query
		// on the legacy seven-column form.
		return newFaultRows([]string{"count(*)"}, [][]driver.Value{{int64(0)}}, -1), nil
	}
	if strings.Contains(query, "from information_schema.statistics") {
		f.providerQueries++
		return newFaultRows([]string{"index_name", "non_unique", "index_type", "column_name", "cardinality", "sub_part", "collation"}, nil, -1), nil
	}
	if strings.Contains(query, "information_schema.referential_constraints") {
		f.providerQueries++
		return newFaultRows([]string{"constraint_name", "column_name", "referenced_table_schema", "referenced_table_name", "referenced_column_name", "delete_rule", "update_rule", "match_option"}, nil, -1), nil
	}
	if strings.Contains(query, "constraint_type not in") {
		f.providerQueries++
		return newFaultRows([]string{"count"}, [][]driver.Value{{int64(0)}}, -1), nil
	}
	if strings.Contains(query, "from information_schema.partitions") {
		f.providerQueries++
		return newFaultRows([]string{"count(*)"}, [][]driver.Value{{int64(0)}}, -1), nil
	}
	return nil, fmt.Errorf("unexpected query %q", query)
}

type faultRows struct {
	columns []string
	rows    [][]driver.Value
	index   int
	faultAt int
}

func newFaultRows(columns []string, rows [][]driver.Value, faultAt int) *faultRows {
	return &faultRows{columns: columns, rows: rows, faultAt: faultAt}
}
func (r *faultRows) Columns() []string { return r.columns }
func (r *faultRows) Close() error      { return nil }
func (r *faultRows) Next(values []driver.Value) error {
	if r.index == r.faultAt {
		return errors.New("injected catalog stream error")
	}
	if r.index >= len(r.rows) {
		return io.EOF
	}
	copy(values, r.rows[r.index])
	r.index++
	return nil
}

func runFixture(t *testing.T, fixture *faultFixture, failFile string) (map[string][]byte, error) {
	t.Helper()
	ledger := &queryLedger{}
	db := sql.OpenDB(&ledgerConnector{faultConnector{fixture}, ledger})
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close controlled DB: %v", err)
		}
	})
	target, err := anchorFor("mysql84")
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	write := func(name string, data []byte) error {
		if name == failFile {
			return errors.New("injected evidence write failure")
		}
		files[name] = append([]byte(nil), data...)
		return nil
	}
	err = collect(context.Background(), db, target, ledger, write)
	return files, err
}

func TestT06A17R1CollectorComplete(t *testing.T) {
	fixture := newFaultFixture()
	files, err := runFixture(t, fixture, "")
	if err != nil {
		t.Fatal(err)
	}
	// providerQueries: tables row + columns + the IS_VISIBLE, EXPRESSION,
	// TABLE_CONSTRAINTS.ENFORCED, CHECK_CONSTRAINTS, and
	// TIDB_CHECK_CONSTRAINTS capability probes + statistics member rows +
	// the T11 foreign-key catalog join + the T12 CHECK member read + the
	// unmodeled-constraint-type scan = 11 catalog round-trips (the
	// PARTITIONS/TIDB_INDEXES probes run only for TiDB, and this fixture
	// loads as MySQL).
	if fixture.creates != 1 || fixture.drops != 1 || fixture.created || fixture.counts != 2 || fixture.providerQueries != 11 {
		t.Fatalf("incomplete lifecycle/provider calls: %+v", fixture)
	}
	var pairs []pair
	if err := json.Unmarshal(files["catalog-provider-pairs.json"], &pairs); err != nil || len(pairs) != 24 {
		t.Fatalf("pairs=%d decode=%v", len(pairs), err)
	}
	var result struct{ Success bool }
	if err := json.Unmarshal(files["run-result.json"], &result); err != nil || !result.Success {
		t.Fatalf("success record: %s (%v)", files["run-result.json"], err)
	}
}

func TestT06A17R1CollectorRejectsFaults(t *testing.T) {
	for _, tc := range []struct {
		name, want, failFile string
		mutate               func(*faultFixture)
		owned                bool
	}{
		{"count-query", "pre-create: table COUNT", "", func(f *faultFixture) { f.preError = errors.New("injected COUNT query error") }, false},
		{"count-scan", "pre-create: table COUNT", "", func(f *faultFixture) { f.preCount = "not-an-integer" }, false},
		{"existing-table", "existing table is not owned", "", func(f *faultFixture) { f.preCount = int64(1) }, false},
		{"rows-error-after-full-set", "iterate catalog after 24 rows", "", func(f *faultFixture) { f.catalogErrorAt = 24 }, true},
		{"rows-error-mid-stream", "iterate catalog after 3 rows", "", func(f *faultFixture) { f.catalogErrorAt = 3 }, true},
		{"scan-error", "scan catalog row 2", "", func(f *faultFixture) { f.catalog[1][1] = "bad-ordinal" }, true},
		{"few-columns", "catalog column count=23", "", func(f *faultFixture) { f.catalog = f.catalog[:23] }, true},
		{"empty-columns", "catalog column count=0", "", func(f *faultFixture) { f.catalog = nil }, true},
		{"extra-column", "catalog column count=25", "", func(f *faultFixture) { f.catalog = append(f.catalog, f.catalog[0]) }, true},
		{"duplicate-column", "duplicate catalog column", "", func(f *faultFixture) { f.catalog[1][0] = "d2" }, true},
		{"provider-missing-column", "provider=23", "", func(f *faultFixture) { f.provider = f.provider[:23] }, true},
		{"provider-empty-columns", "provider=0", "", func(f *faultFixture) { f.provider = nil }, true},
		{"provider-duplicate-column", "duplicate provider column", "", func(f *faultFixture) { f.provider[1] = f.provider[0] }, true},
		{"provider-absent", "provider snapshot missing", "", func(f *faultFixture) { f.providerAbsent = true }, true},
		{"evidence-write", "injected evidence write failure", "catalog-rows.json", func(*faultFixture) {}, true},
		{"post-count-query", "cleanup final COUNT", "", func(f *faultFixture) { f.postError = errors.New("injected final COUNT error") }, true},
		{"primary-and-cleanup", "catalog column count=23", "", func(f *faultFixture) { f.catalog = f.catalog[:23]; f.dropError = errors.New("injected DROP error") }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newFaultFixture()
			tc.mutate(fixture)
			files, err := runFixture(t, fixture, tc.failFile)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v, want %q", err, tc.want)
			}
			if tc.owned {
				if fixture.creates != 1 || fixture.drops != 1 || fixture.counts != 2 {
					t.Fatalf("owned cleanup not attempted: creates=%d drops=%d counts=%d", fixture.creates, fixture.drops, fixture.counts)
				}
			} else if fixture.creates != 0 || fixture.drops != 0 {
				t.Fatalf("unowned table touched: creates=%d drops=%d", fixture.creates, fixture.drops)
			}
			var result struct{ Success bool }
			if decodeErr := json.Unmarshal(files["run-result.json"], &result); decodeErr != nil || result.Success {
				t.Fatalf("fault recorded as success: %s (%v)", files["run-result.json"], decodeErr)
			}
			if tc.name == "count-query" || tc.name == "count-scan" || tc.name == "post-count-query" {
				var life lifecycle
				if err := json.Unmarshal(files["lifecycle.json"], &life); err != nil {
					t.Fatal(err)
				}
				if tc.name == "post-count-query" && life.PostCount != nil || tc.name != "post-count-query" && life.PreCount != nil {
					t.Fatalf("failed COUNT fabricated an integer: %+v", life)
				}
			}
			if tc.name == "primary-and-cleanup" && (!strings.Contains(err.Error(), "injected DROP error") || !strings.Contains(err.Error(), "final COUNT=1")) {
				t.Fatalf("primary/cleanup errors lost: %v", err)
			}
			t.Logf("constructed fault rejected: %v", err)
		})
	}
}

func TestT06A17R1IndependentFacts(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*faultFixture)
	}{
		{"decimal-precision", func(f *faultFixture) { f.catalog[0][5] = int64(11) }},
		{"members-both-sides-corrupt", func(f *faultFixture) {
			value := "enum('ready','Closed')"
			f.catalog[13][3] = value
			f.catalog[13][4] = strings.ToUpper(hex.EncodeToString([]byte(value)))
			f.provider[13][1] = value
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newFaultFixture()
			tc.mutate(fixture)
			if _, err := runFixture(t, fixture, ""); err == nil {
				t.Fatal("independent input fact corruption accepted")
			}
		})
	}
}

func TestT06A17R1EvidenceEncodingAndDiskErrors(t *testing.T) {
	if err := writeJSON(func(string, []byte) error { return nil }, "unsupported.json", func() {}); err == nil {
		t.Fatal("JSON encoding failure accepted")
	}
	blocker := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocker, []byte("fixture"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(func(name string, data []byte) error {
		return os.WriteFile(filepath.Join(blocker, name), data, 0o644)
	}, "evidence.json", true); err == nil {
		t.Fatal("disk write failure accepted")
	}
}
