// Package mysqlmeta verifies metadata normalization helpers for the MySQL provider.
// input: synthetic variable, version, DSN, collation, type, column-nullability, and index classification scenarios
// output: stable provider helper behavior and normalized column metadata without requiring a live database
// pos: infrastructure metadata adapter test coverage
// note: if this file changes, update this header and module README.md.
package mysqlmeta

import (
	"context"
	"crypto/x509"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestConnectionConfigDSNUsesTCPHostAndPort(t *testing.T) {
	config := ConnectionConfig{
		Host:     "127.0.0.1",
		Port:     3307,
		User:     "root",
		Password: "secret",
	}

	if got := config.Address(); got != "127.0.0.1:3307" {
		t.Fatalf("expected tcp address 127.0.0.1:3307, got %q", got)
	}
	if got := config.Network(); got != "tcp" {
		t.Fatalf("expected tcp network, got %q", got)
	}
}

func TestConnectionConfigDSNUsesUnixSocket(t *testing.T) {
	config := ConnectionConfig{
		Socket:   "/tmp/mysql.sock",
		User:     "root",
		Password: "secret",
	}

	if got := config.Address(); got != "/tmp/mysql.sock" {
		t.Fatalf("expected socket address /tmp/mysql.sock, got %q", got)
	}
	if got := config.Network(); got != "unix" {
		t.Fatalf("expected unix network, got %q", got)
	}
}

func TestConnectionConfigDefaultsWhenEmpty(t *testing.T) {
	config := ConnectionConfig{}

	if got := config.Network(); got != "tcp" {
		t.Fatalf("expected tcp for empty config, got %q", got)
	}
	if got := config.Address(); got != "127.0.0.1:3306" {
		t.Fatalf("expected default host:port, got %q", got)
	}
}

func TestConnectTimeoutDefaultsToFiveSeconds(t *testing.T) {
	config := ConnectionConfig{}
	if got := config.connectTimeout(); got != DefaultConnectTimeout {
		t.Fatalf("expected default timeout %v, got %v", DefaultConnectTimeout, got)
	}
}

func TestConnectTimeoutPreservesCustomValue(t *testing.T) {
	config := ConnectionConfig{ConnectTimeout: 10 * time.Second}
	if got := config.connectTimeout(); got != 10*time.Second {
		t.Fatalf("expected custom timeout 10s, got %v", got)
	}
}

func TestConnectTimeoutDSNIncludesDefaultTimeout(t *testing.T) {
	config := ConnectionConfig{
		Host: "127.0.0.1",
		Port: 3306,
		User: "root",
	}
	dsn := config.DSN()
	if !strings.Contains(dsn, "5s") {
		t.Fatalf("expected default timeout 5s in DSN, got %q", dsn)
	}
}

func TestConnectTimeoutDSNReflectsCustomValue(t *testing.T) {
	config := ConnectionConfig{
		Host:           "127.0.0.1",
		Port:           3306,
		User:           "root",
		ConnectTimeout: 15 * time.Second,
	}
	dsn := config.DSN()
	if !strings.Contains(dsn, "15s") {
		t.Fatalf("expected custom timeout 15s in DSN, got %q", dsn)
	}
}

func TestOpenDBContextReturnsCanceledWhenCtxAlreadyCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	config := ConnectionConfig{
		Host: "127.0.0.1",
		Port: 3306,
		User: "root",
	}
	_, err := OpenDBContext(ctx, config)
	if err == nil {
		t.Fatal("expected error from OpenDBContext with canceled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestOpenDBContextDoesNotPanicOnNilContext(t *testing.T) {
	config := ConnectionConfig{
		Host:           "127.0.0.1",
		Port:           3306,
		User:           "root",
		ConnectTimeout: time.Millisecond,
	}
	db, err := OpenDBContext(nil, config)
	if db != nil {
		_ = db.Close()
	}
	_ = err
}

func TestConnectionConfigDSNFormat(t *testing.T) {
	config := ConnectionConfig{
		Host: "db.example.com",
		Port: 3306,
		User: "appuser",
	}

	dsn := config.DSN()
	if !strings.Contains(dsn, "appuser") {
		t.Fatalf("expected DSN to contain user, got %q", dsn)
	}
	if !strings.Contains(dsn, "tcp") {
		t.Fatalf("expected DSN to contain tcp network, got %q", dsn)
	}
	if !strings.Contains(dsn, "db.example.com") {
		t.Fatalf("expected DSN to contain host, got %q", dsn)
	}
}

func TestConnectionConfigDSNSocketFormat(t *testing.T) {
	config := ConnectionConfig{
		Socket: "/var/run/mysqld/mysqld.sock",
		User:   "root",
	}

	dsn := config.DSN()
	if !strings.Contains(dsn, "unix") {
		t.Fatalf("expected DSN to contain unix network, got %q", dsn)
	}
	if !strings.Contains(dsn, "/var/run/mysqld/mysqld.sock") {
		t.Fatalf("expected DSN to contain socket path, got %q", dsn)
	}
}

func TestConnectionConfigDSNIncludesDatabase(t *testing.T) {
	config := ConnectionConfig{
		Host:     "127.0.0.1",
		Port:     3306,
		User:     "root",
		Password: "secret",
		Database: "mydb",
	}

	dsn := config.DSN()
	if !strings.Contains(dsn, "mydb") {
		t.Fatalf("expected DSN to contain database mydb, got %q", dsn)
	}
}

func TestConnectionConfigDSNOmitsDatabaseWhenEmpty(t *testing.T) {
	config := ConnectionConfig{
		Host:     "127.0.0.1",
		Port:     3306,
		User:     "root",
		Password: "secret",
	}

	dsn := config.DSN()
	// MySQL DSN format: user:pass@tcp(host:port)/dbname?params
	// When Database is empty, the DSN should end with "/" after host:port
	// (no database name between "/" and "?")
	if strings.Contains(dsn, "/?") || strings.HasSuffix(dsn, "/") {
		// This is correct - empty database means "/" immediately followed by "?" or end
		return
	}
	// If there's a database name between "/" and "?", that's wrong
	parts := strings.SplitN(dsn, "/", 2)
	if len(parts) == 2 {
		dbPart := strings.SplitN(parts[1], "?", 2)[0]
		if dbPart != "" {
			t.Fatalf("expected no database name in DSN when empty, got %q", dsn)
		}
	}
}

func TestConnectionConfigTLSEnabledSetsServerName(t *testing.T) {
	config := ConnectionConfig{
		Host:     "mysql-tls.example.com",
		Port:     3306,
		User:     "root",
		Password: "secret",
		TLSMode:  "enabled",
	}

	cfg := config.mysqlConfig()
	if cfg.TLS == nil {
		t.Fatal("expected TLS config to be set when TLSMode is enabled")
	}
	if cfg.TLS.ServerName != "mysql-tls.example.com" {
		t.Fatalf("expected TLS ServerName mysql-tls.example.com, got %q", cfg.TLS.ServerName)
	}
	if cfg.TLSConfig != "" {
		t.Fatalf("expected empty TLSConfig name (no RegisterTLSConfig), got %q", cfg.TLSConfig)
	}
}

func TestConnectionConfigTLSEnabledDefaultsServerNameToLocalhost(t *testing.T) {
	config := ConnectionConfig{
		Host:    "",
		Port:    3306,
		User:    "root",
		TLSMode: "enabled",
	}

	cfg := config.mysqlConfig()
	if cfg.TLS == nil {
		t.Fatal("expected TLS config to be set when TLSMode is enabled")
	}
	if cfg.TLS.ServerName != "127.0.0.1" {
		t.Fatalf("expected TLS ServerName 127.0.0.1 for empty host, got %q", cfg.TLS.ServerName)
	}
	if cfg.TLSConfig != "" {
		t.Fatalf("expected empty TLSConfig name (no RegisterTLSConfig), got %q", cfg.TLSConfig)
	}
}

func TestConnectionConfigTLSDisabledDoesNotSetTLS(t *testing.T) {
	config := ConnectionConfig{
		Host:     "127.0.0.1",
		Port:     3306,
		User:     "root",
		Password: "secret",
		TLSMode:  "disabled",
	}

	cfg := config.mysqlConfig()
	if cfg.TLS != nil {
		t.Fatal("expected TLS config to be nil when TLSMode is disabled")
	}
}

func TestConnectionConfigTLSEmptyDoesNotSetTLS(t *testing.T) {
	config := ConnectionConfig{
		Host:     "127.0.0.1",
		Port:     3306,
		User:     "root",
		Password: "secret",
	}

	cfg := config.mysqlConfig()
	if cfg.TLS != nil {
		t.Fatal("expected TLS config to be nil when TLSMode is empty")
	}
}

func TestConnectionConfigTLSCACertDoesNotFallBackToSystemRoots(t *testing.T) {
	customPool := x509.NewCertPool()
	customPool.AppendCertsFromPEM([]byte("-----BEGIN CERTIFICATE-----\nMIIBkTCB+wIJAL...\n-----END CERTIFICATE-----"))

	config := ConnectionConfig{
		Host:     "mysql-tls.example.com",
		Port:     3306,
		User:     "root",
		Password: "secret",
		TLSMode:  "enabled",
		CACert:   customPool,
	}

	cfg := config.mysqlConfig()
	if cfg.TLS == nil {
		t.Fatal("expected TLS config to be set when TLSMode is enabled")
	}
	if cfg.TLS.RootCAs != customPool {
		t.Fatal("expected TLS RootCAs to be the custom pool, not system roots")
	}
}

func TestConnectionConfigTLSNilCACertUsesSystemRoots(t *testing.T) {
	config := ConnectionConfig{
		Host:     "mysql-tls.example.com",
		Port:     3306,
		User:     "root",
		Password: "secret",
		TLSMode:  "enabled",
	}

	cfg := config.mysqlConfig()
	if cfg.TLS == nil {
		t.Fatal("expected TLS config to be set when TLSMode is enabled")
	}
	if cfg.TLS.RootCAs != nil {
		t.Fatal("expected TLS RootCAs to be nil (system roots) when CACert is nil")
	}
}

func TestDetectDialectFromVersion(t *testing.T) {
	tests := []struct {
		version string
		want    spec.Dialect
	}{
		{"8.0.36", spec.DialectMySQL},
		{"8.0.11-TiDB-v8.5.0", spec.DialectTiDB},
		{"5.7.44", spec.DialectMySQL},
		{"TiDB-v7.6.0", spec.DialectTiDB},
		{"8.0.11-tidb-v8.5.0", spec.DialectTiDB},
	}
	for _, tc := range tests {
		t.Run(tc.version, func(t *testing.T) {
			if got := detectDialectFromVersion(tc.version); got != tc.want {
				t.Fatalf("detectDialectFromVersion(%q) = %q, want %q", tc.version, got, tc.want)
			}
		})
	}
}

func TestNormalizeOnOff(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"ON", true},
		{"on", true},
		{"1", true},
		{"yes", true},
		{"true", true},
		{" True ", true},
		{"OFF", false},
		{"0", false},
		{"no", false},
		{"false", false},
		{"maybe", false},
		{"", false},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			if got := normalizeOnOff(tc.input); got != tc.want {
				t.Fatalf("normalizeOnOff(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestCharsetFromCollation(t *testing.T) {
	tests := []struct {
		collation string
		want      string
	}{
		{"utf8mb4_general_ci", "utf8mb4"},
		{"utf8_general_ci", "utf8"},
		{"latin1_swedish_ci", "latin1"},
		{"binary", ""},
		{"", ""},
		{"_underscore_first", ""},
	}
	for _, tc := range tests {
		t.Run(tc.collation, func(t *testing.T) {
			if got := charsetFromCollation(tc.collation); got != tc.want {
				t.Fatalf("charsetFromCollation(%q) = %q, want %q", tc.collation, got, tc.want)
			}
		})
	}
}

func TestParseColumnType(t *testing.T) {
	tests := []struct {
		input        string
		wantBase     string
		wantLen      int
		wantUnsigned bool
	}{
		{"varchar(255)", "varchar", 255, false},
		{"bigint(20) unsigned", "bigint", 20, true},
		{"int", "int", 0, false},
		{"text", "text", 0, false},
		{"decimal(10,2)", "decimal", 10, false},
		{"bigint unsigned", "bigint", 0, true},
		{"tinyint(1)", "tinyint", 1, false},
		{"varchar(64) unsigned", "varchar", 64, true},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			base, length, unsigned := parseColumnType(tc.input)
			if base != tc.wantBase || length != tc.wantLen || unsigned != tc.wantUnsigned {
				t.Fatalf("parseColumnType(%q) = (%q, %d, %v), want (%q, %d, %v)",
					tc.input, base, length, unsigned, tc.wantBase, tc.wantLen, tc.wantUnsigned)
			}
		})
	}
}

func TestClassifyIndex(t *testing.T) {
	if kind := classifyIndex("PRIMARY", 0, "BTREE"); kind != spec.IndexKindPrimary {
		t.Fatalf("expected primary index kind, got %q", kind)
	}
	if kind := classifyIndex("uniq_email", 0, "BTREE"); kind != spec.IndexKindUnique {
		t.Fatalf("expected unique index kind, got %q", kind)
	}
	if kind := classifyIndex("full_body", 1, "FULLTEXT"); kind != spec.IndexKindFulltext {
		t.Fatalf("expected fulltext index kind, got %q", kind)
	}
	if kind := classifyIndex("idx_email", 1, "BTREE"); kind != spec.IndexKindSecondary {
		t.Fatalf("expected secondary index kind, got %q", kind)
	}
}

func TestAccumulateIndexCardinalityPreservesMaximumNonNilValue(t *testing.T) {
	indexes := make(map[string]*spec.Index)
	order := make([]string, 0)

	accumulateIndexRow(indexes, &order, "PRIMARY", 0, "BTREE", "id", nil)
	accumulateIndexRow(indexes, &order, "PRIMARY", 0, "BTREE", "tenant_id", testPtrInt64(12))
	accumulateIndexRow(indexes, &order, "PRIMARY", 0, "BTREE", "account_id", testPtrInt64(7))

	index := indexes["PRIMARY"]
	if index == nil {
		t.Fatalf("expected primary index to be accumulated")
	}
	if index.Cardinality == nil || *index.Cardinality != 12 {
		t.Fatalf("expected max non-nil cardinality 12, got %#v", index)
	}
	if len(index.Columns) != 3 {
		t.Fatalf("expected all index columns to be preserved, got %#v", index.Columns)
	}
}

func TestAccumulateIndexRowMultipleIndexes(t *testing.T) {
	indexes := make(map[string]*spec.Index)
	order := make([]string, 0)

	accumulateIndexRow(indexes, &order, "PRIMARY", 0, "BTREE", "id", testPtrInt64(100))
	accumulateIndexRow(indexes, &order, "idx_email", 1, "BTREE", "email", testPtrInt64(50))

	if len(indexes) != 2 {
		t.Fatalf("expected 2 indexes, got %d", len(indexes))
	}
	if len(order) != 2 || order[0] != "PRIMARY" || order[1] != "idx_email" {
		t.Fatalf("unexpected order: %#v", order)
	}
}

func TestDetectDialectWithMockDB(t *testing.T) {
	db := openTestDB(t, map[string]testQueryResult{
		"select version()": {
			columns: []string{"version()"},
			rows:    [][]driver.Value{{"8.0.36"}},
		},
	})
	defer db.Close()

	provider := NewProvider(db)
	dialect, err := provider.DetectDialect(context.Background())
	if err != nil {
		t.Fatalf("DetectDialect: %v", err)
	}
	if dialect != spec.DialectMySQL {
		t.Fatalf("expected mysql, got %q", dialect)
	}
}

func TestDetectDialectTiDBWithMockDB(t *testing.T) {
	db := openTestDB(t, map[string]testQueryResult{
		"select version()": {
			columns: []string{"version()"},
			rows:    [][]driver.Value{{"8.0.11-TiDB-v8.5.0"}},
		},
	})
	defer db.Close()

	provider := NewProvider(db)
	dialect, err := provider.DetectDialect(context.Background())
	if err != nil {
		t.Fatalf("DetectDialect: %v", err)
	}
	if dialect != spec.DialectTiDB {
		t.Fatalf("expected tidb, got %q", dialect)
	}
}

func TestLoadInstanceFactsWithMockDB(t *testing.T) {
	db := openTestDB(t, map[string]testQueryResult{
		"show variables": {
			columns: []string{"Variable_name", "Value"},
			rows: [][]driver.Value{
				{"version", "8.0.36"},
				{"character_set_database", "utf8mb4"},
				{"innodb_large_prefix", "ON"},
				{"innodb_default_row_format", "dynamic"},
				{"innodb_adaptive_hash_index", "OFF"},
			},
		},
	})
	defer db.Close()

	provider := NewProvider(db)
	facts, err := provider.LoadInstanceFacts(context.Background(), spec.DialectMySQL, "app")
	if err != nil {
		t.Fatalf("LoadInstanceFacts: %v", err)
	}
	if facts.Version != "8.0.36" {
		t.Fatalf("expected version 8.0.36, got %q", facts.Version)
	}
	if facts.DefaultCharset != "utf8mb4" {
		t.Fatalf("expected charset utf8mb4, got %q", facts.DefaultCharset)
	}
	if !facts.InnoDBLargePrefixEnabled {
		t.Fatal("expected innodb_large_prefix enabled")
	}
	if facts.InnoDBDefaultRowFormat != "dynamic" {
		t.Fatalf("expected dynamic row format, got %q", facts.InnoDBDefaultRowFormat)
	}
	if facts.InnoDBAdaptiveHashEnabled {
		t.Fatal("expected innodb_adaptive_hash_index disabled")
	}
}

// innodb_page_size becomes a known numeric fact only when the server reports
// a positive integer; a missing or unparsable row must stay unknown so rules
// downgrade to a bounded evidence gap instead of assuming a page size.
func TestLoadInstanceFactsInnoDBPageSize(t *testing.T) {
	cases := []struct {
		name      string
		value     *string
		wantBytes int
		wantKnown bool
	}{
		{name: "16k page", value: ptrTestString("16384"), wantBytes: 16384, wantKnown: true},
		{name: "4k page", value: ptrTestString("4096"), wantBytes: 4096, wantKnown: true},
		{name: "missing row", value: nil, wantKnown: false},
		{name: "zero is unknown", value: ptrTestString("0"), wantKnown: false},
		{name: "unparsable is unknown", value: ptrTestString("auto"), wantKnown: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			variables := [][]driver.Value{
				{"version", "8.4.10"},
				{"character_set_database", "utf8mb4"},
			}
			if tc.value != nil {
				variables = append(variables, []driver.Value{"innodb_page_size", *tc.value})
			}
			db := openTestDB(t, map[string]testQueryResult{
				"show variables": {
					columns: []string{"Variable_name", "Value"},
					rows:    variables,
				},
			})
			defer db.Close()

			facts, err := NewProvider(db).LoadInstanceFacts(context.Background(), spec.DialectMySQL, "app")
			if err != nil {
				t.Fatalf("LoadInstanceFacts: %v", err)
			}
			if facts.InnoDBPageSizeKnown != tc.wantKnown {
				t.Fatalf("expected known=%t, got %t (bytes=%d)", tc.wantKnown, facts.InnoDBPageSizeKnown, facts.InnoDBPageSizeBytes)
			}
			if tc.wantKnown && facts.InnoDBPageSizeBytes != tc.wantBytes {
				t.Fatalf("expected %d bytes, got %d", tc.wantBytes, facts.InnoDBPageSizeBytes)
			}
		})
	}
}

// TiDB's index-length bound lives in tidb-server max-index-length config, read
// via SHOW CONFIG. Every matching row is verified — single row, same-value
// multi-row, disagreeing multi-row in both orders, zero rows, and unparsable
// values — while a real query/scan/iteration failure propagates as a provider
// error and never degrades into a gap.
func TestLoadInstanceFactsTiDBMaxIndexLength(t *testing.T) {
	tidbVariables := testQueryResult{
		columns: []string{"Variable_name", "Value"},
		rows: [][]driver.Value{
			{"version", "8.0.11-TiDB-v8.5.0"},
			{"character_set_database", "utf8mb4"},
			{"innodb_page_size", "16384"},
		},
	}
	configRow := func(instance, value string) []driver.Value {
		return []driver.Value{"tidb", instance, "max-index-length", value}
	}
	configColumns := []string{"Type", "Instance", "Name", "Value"}

	cases := []struct {
		name      string
		config    testQueryResult
		wantBytes int
		wantKnown bool
		wantErr   bool
	}{
		{
			name:      "single row becomes known",
			config:    testQueryResult{columns: configColumns, rows: [][]driver.Value{configRow("n1:4000", "12288")}},
			wantBytes: 12288, wantKnown: true,
		},
		{
			name: "same value across rows becomes known",
			config: testQueryResult{columns: configColumns, rows: [][]driver.Value{
				configRow("n1:4000", "3072"), configRow("n2:4000", "3072"), configRow("n3:4000", "3072"),
			}},
			wantBytes: 3072, wantKnown: true,
		},
		{
			name: "disagreeing rows ascending stays unknown",
			config: testQueryResult{columns: configColumns, rows: [][]driver.Value{
				configRow("n1:4000", "3072"), configRow("n2:4000", "12288"),
			}},
			wantKnown: false,
		},
		{
			name: "disagreeing rows descending stays unknown",
			config: testQueryResult{columns: configColumns, rows: [][]driver.Value{
				configRow("n1:4000", "12288"), configRow("n2:4000", "3072"),
			}},
			wantKnown: false,
		},
		{
			name:      "zero rows stays unknown",
			config:    testQueryResult{columns: configColumns, rows: [][]driver.Value{}},
			wantKnown: false,
		},
		{
			name:      "unparsable value stays unknown",
			config:    testQueryResult{columns: configColumns, rows: [][]driver.Value{configRow("n1:4000", "auto")}},
			wantKnown: false,
		},
		{
			name:      "non-positive value stays unknown",
			config:    testQueryResult{columns: configColumns, rows: [][]driver.Value{configRow("n1:4000", "0")}},
			wantKnown: false,
		},
		{
			name: "unparsable row among consistent rows stays unknown",
			config: testQueryResult{columns: configColumns, rows: [][]driver.Value{
				configRow("n1:4000", "3072"), configRow("n2:4000", "auto"),
			}},
			wantKnown: false,
		},
		{
			name: "unparsable first row then valid stays unknown",
			config: testQueryResult{columns: configColumns, rows: [][]driver.Value{
				configRow("n1:4000", "auto"), configRow("n2:4000", "3072"),
			}},
			wantKnown: false,
		},
		{
			name:    "query error propagates as provider error",
			config:  testQueryResult{queryErr: errors.New("SHOW CONFIG denied")},
			wantErr: true,
		},
		{
			name: "scan error propagates as provider error",
			config: testQueryResult{
				columns: []string{"Type", "Instance", "Name"},
				rows:    [][]driver.Value{{"tidb", "n1:4000", "max-index-length"}},
			},
			wantErr: true,
		},
		{
			name: "mid-iteration error propagates as provider error",
			config: testQueryResult{
				columns:  configColumns,
				rows:     [][]driver.Value{configRow("n1:4000", "3072"), configRow("n2:4000", "3072")},
				rowErrAt: 1,
				rowErr:   errors.New("connection lost mid-result"),
			},
			wantErr: true,
		},
		// T04-B-R2 residual: an invalid value marks the read unusable but must
		// never short-circuit the stream — a following iteration or scan
		// failure is still a provider error, not an unknown fact.
		{
			name: "invalid value still surfaces following iteration error",
			config: testQueryResult{
				columns:  configColumns,
				rows:     [][]driver.Value{configRow("n1:4000", "auto"), configRow("n2:4000", "3072")},
				rowErrAt: 1,
				rowErr:   errors.New("connection lost mid-result"),
			},
			wantErr: true,
		},
		{
			name: "non-positive value still surfaces following iteration error",
			config: testQueryResult{
				columns:  configColumns,
				rows:     [][]driver.Value{configRow("n1:4000", "0"), configRow("n2:4000", "3072")},
				rowErrAt: 1,
				rowErr:   errors.New("connection lost mid-result"),
			},
			wantErr: true,
		},
		{
			name: "consistent prefix still surfaces trailing iteration error",
			config: testQueryResult{
				columns: configColumns,
				rows: [][]driver.Value{
					configRow("n1:4000", "3072"), configRow("n2:4000", "auto"), configRow("n3:4000", "3072"),
				},
				rowErrAt: 2,
				rowErr:   errors.New("connection lost mid-result"),
			},
			wantErr: true,
		},
		{
			name: "invalid value still surfaces following scan error",
			config: testQueryResult{
				columns: configColumns,
				rows: [][]driver.Value{
					configRow("n1:4000", "auto"),
					{"tidb", "n2:4000", "max-index-length"}, // short row → scan error on the second row
				},
			},
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := openTestDB(t, map[string]testQueryResult{
				"show variables":    tidbVariables,
				"show config where": tc.config,
			})
			defer db.Close()

			facts, err := NewProvider(db).LoadInstanceFacts(context.Background(), spec.DialectTiDB, "app")
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected provider error, got facts=%#v", facts)
				}
				return
			}
			if err != nil {
				t.Fatalf("LoadInstanceFacts: %v", err)
			}
			if facts.TiDBMaxIndexLengthKnown != tc.wantKnown {
				t.Fatalf("expected known=%t, got %t (bytes=%d)", tc.wantKnown, facts.TiDBMaxIndexLengthKnown, facts.TiDBMaxIndexLengthBytes)
			}
			if tc.wantKnown && facts.TiDBMaxIndexLengthBytes != tc.wantBytes {
				t.Fatalf("expected %d bytes, got %d", tc.wantBytes, facts.TiDBMaxIndexLengthBytes)
			}
		})
	}

	t.Run("mysql banner never probes tidb config", func(t *testing.T) {
		db := openTestDB(t, map[string]testQueryResult{
			"show variables": {
				columns: []string{"Variable_name", "Value"},
				rows:    [][]driver.Value{{"version", "8.4.10"}},
			},
			// SHOW CONFIG would not exist on MySQL; the provider must not
			// even attempt it for a MySQL banner.
		})
		defer db.Close()

		facts, err := NewProvider(db).LoadInstanceFacts(context.Background(), spec.DialectMySQL, "app")
		if err != nil {
			t.Fatalf("LoadInstanceFacts: %v", err)
		}
		if facts.TiDBMaxIndexLengthKnown {
			t.Fatalf("mysql banner must not produce tidb fact, got %d", facts.TiDBMaxIndexLengthBytes)
		}
	})
}

func ptrTestString(value string) *string {
	return &value
}

func TestLoadTableSnapshotReturnsNotExistsWhenNoTableRow(t *testing.T) {
	db := openTestDB(t, map[string]testQueryResult{
		"from information_schema.tables": {
			columns: []string{"engine", "table_collation", "table_comment", "auto_increment", "row_format", "table_rows"},
			rows:    [][]driver.Value{},
		},
	})
	defer db.Close()

	provider := NewProvider(db)
	snapshot, err := provider.LoadTableSnapshot(context.Background(), spec.DialectMySQL, "app", "missing")
	if err != nil {
		t.Fatalf("LoadTableSnapshot: %v", err)
	}
	if snapshot.Exists {
		t.Fatal("expected Exists=false for missing table")
	}
	if snapshot.Schema != "app" {
		t.Fatalf("expected schema=app, got %q", snapshot.Schema)
	}
}

func TestFindSchemasForTableWithMockDB(t *testing.T) {
	db := openTestDB(t, map[string]testQueryResult{
		"from information_schema.tables": {
			columns: []string{"table_schema"},
			rows: [][]driver.Value{
				{"app"},
				{"staging"},
			},
		},
	})
	defer db.Close()

	provider := NewProvider(db)
	schemas, err := provider.FindSchemasForTable(context.Background(), "users")
	if err != nil {
		t.Fatalf("FindSchemasForTable: %v", err)
	}
	if len(schemas) != 2 || schemas[0] != "app" || schemas[1] != "staging" {
		t.Fatalf("expected [app staging], got %#v", schemas)
	}
}

func TestLoadTableSnapshotPreservesIndexCardinalityFromStatisticsRows(t *testing.T) {
	db := openTestDB(t, map[string]testQueryResult{
		"from information_schema.tables": {
			columns: []string{"engine", "table_collation", "table_comment", "auto_increment", "row_format", "table_rows"},
			rows: [][]driver.Value{{
				"InnoDB", "utf8mb4_general_ci", "users table", nil, "Dynamic", int64(100),
			}},
		},
		"from information_schema.columns": {
			columns: []string{"column_name", "column_type", "character_set_name", "collation_name", "column_comment", "column_default", "is_nullable", "extra"},
			rows: [][]driver.Value{
				{"id", "bigint(20)", nil, nil, "", nil, "NO", ""},
				{"email", "varchar(255)", "utf8mb4", "utf8mb4_general_ci", "", nil, "YES", ""},
			},
		},
		"from information_schema.statistics": {
			columns: []string{"index_name", "non_unique", "index_type", "column_name", "cardinality"},
			rows: [][]driver.Value{
				{"PRIMARY", int64(0), "BTREE", "id", int64(120)},
				{"idx_users_email", int64(1), "BTREE", "email", int64(45)},
				{"idx_users_email", int64(1), "BTREE", "tenant_id", int64(42)},
			},
		},
	})
	defer db.Close()

	provider := NewProvider(db)
	snapshot, err := provider.LoadTableSnapshot(context.Background(), spec.DialectMySQL, "app", "users")
	if err != nil {
		t.Fatalf("load table snapshot: %v", err)
	}

	if snapshot.PrimaryKey == nil || snapshot.PrimaryKey.Cardinality == nil || *snapshot.PrimaryKey.Cardinality != 120 {
		t.Fatalf("expected primary key cardinality 120, got %#v", snapshot.PrimaryKey)
	}
	if len(snapshot.Indexes) != 1 {
		t.Fatalf("expected one secondary index, got %#v", snapshot.Indexes)
	}
	if snapshot.Indexes[0].Cardinality == nil || *snapshot.Indexes[0].Cardinality != 45 {
		t.Fatalf("expected max secondary index cardinality 45, got %#v", snapshot.Indexes[0])
	}
	if len(snapshot.Indexes[0].Columns) != 2 {
		t.Fatalf("expected secondary index columns to be preserved, got %#v", snapshot.Indexes[0])
	}
	if !snapshot.Columns[0].NotNull || snapshot.Columns[1].NotNull {
		t.Fatalf("expected information_schema.is_nullable to map to NotNull, got %#v", snapshot.Columns)
	}
}

func testPtrInt64(value int64) *int64 {
	return &value
}

type testQueryResult struct {
	columns []string
	rows    [][]driver.Value
	// queryErr fails the whole query at QueryContext time.
	queryErr error
	// rowErr fails iteration at the rowErrAt index (a mid-iteration error
	// surfaced through rows.Err() once rows.Next() returns false).
	rowErrAt int
	rowErr   error
}

type testDriver struct {
	results map[string]testQueryResult
}

type testConn struct {
	results map[string]testQueryResult
}

type testRows struct {
	columns  []string
	rows     [][]driver.Value
	index    int
	rowErrAt int
	rowErr   error
}

var (
	testDriverRegistry sync.Once
	testDriverCounter  int
)

func openTestDB(t *testing.T, results map[string]testQueryResult) *sql.DB {
	t.Helper()

	testDriverRegistry.Do(func() {
		sql.Register("mysqlmeta-test", testDriver{})
	})

	name := "mysqlmeta-test-" + strings.ReplaceAll(t.Name(), "/", "_")
	registerTestDriverResults(name, results)

	db, err := sql.Open("mysqlmeta-test", name)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { unregisterTestDriverResults(name) })
	return db
}

var testDriverResults sync.Map

func registerTestDriverResults(name string, results map[string]testQueryResult) {
	cloned := make(map[string]testQueryResult, len(results))
	for key, result := range results {
		rows := make([][]driver.Value, len(result.rows))
		for i := range result.rows {
			rows[i] = append([]driver.Value(nil), result.rows[i]...)
		}
		cloned[key] = testQueryResult{
			columns:  append([]string(nil), result.columns...),
			rows:     rows,
			queryErr: result.queryErr,
			rowErrAt: result.rowErrAt,
			rowErr:   result.rowErr,
		}
	}
	testDriverResults.Store(name, cloned)
}

func unregisterTestDriverResults(name string) {
	testDriverResults.Delete(name)
}

func (d testDriver) Open(name string) (driver.Conn, error) {
	value, _ := testDriverResults.Load(name)
	results, _ := value.(map[string]testQueryResult)
	return testConn{results: results}, nil
}

func (c testConn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (c testConn) Close() error                        { return nil }
func (c testConn) Begin() (driver.Tx, error)           { return nil, driver.ErrSkip }

func (c testConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	for fragment, result := range c.results {
		if strings.Contains(query, fragment) {
			if result.queryErr != nil {
				return nil, result.queryErr
			}
			return &testRows{
				columns:  append([]string(nil), result.columns...),
				rows:     append([][]driver.Value(nil), result.rows...),
				rowErrAt: result.rowErrAt,
				rowErr:   result.rowErr,
			}, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (r *testRows) Columns() []string {
	return append([]string(nil), r.columns...)
}

func (r *testRows) Close() error { return nil }

func (r *testRows) Next(dest []driver.Value) error {
	if r.rowErr != nil && r.index == r.rowErrAt {
		return r.rowErr
	}
	if r.index >= len(r.rows) {
		return io.EOF
	}
	// database/sql reuses the raw buffer across Next calls; a short row must
	// leave nil holes (scanning NULL → error), not inherit stale values.
	for i := range dest {
		dest[i] = nil
	}
	copy(dest, r.rows[r.index])
	r.index++
	return nil
}
