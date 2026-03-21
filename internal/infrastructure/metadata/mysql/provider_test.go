// Package mysqlmeta verifies metadata normalization helpers for the MySQL provider.
// input: synthetic variable, version, DSN, collation, type, and index classification scenarios
// output: stable provider helper behavior without requiring a live database
// pos: infrastructure metadata adapter test coverage
// note: if this file changes, update this header and module README.md.
package mysqlmeta

import (
	"testing"

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

func TestDetectDialectFromVersion(t *testing.T) {
	if got := detectDialectFromVersion("8.0.36"); got != spec.DialectMySQL {
		t.Fatalf("expected mysql dialect, got %q", got)
	}
	if got := detectDialectFromVersion("8.0.11-TiDB-v8.5.0"); got != spec.DialectTiDB {
		t.Fatalf("expected tidb dialect, got %q", got)
	}
}

func TestNormalizeOnOff(t *testing.T) {
	if !normalizeOnOff("ON") {
		t.Fatalf("expected ON to normalize to true")
	}
	if !normalizeOnOff("1") {
		t.Fatalf("expected 1 to normalize to true")
	}
	if normalizeOnOff("OFF") {
		t.Fatalf("expected OFF to normalize to false")
	}
}

func TestCharsetFromCollation(t *testing.T) {
	if got := charsetFromCollation("utf8mb4_general_ci"); got != "utf8mb4" {
		t.Fatalf("expected utf8mb4, got %q", got)
	}
	if got := charsetFromCollation(""); got != "" {
		t.Fatalf("expected empty charset, got %q", got)
	}
}

func TestParseColumnType(t *testing.T) {
	baseType, length, unsigned := parseColumnType("varchar(255)")
	if baseType != "varchar" || length != 255 || unsigned {
		t.Fatalf("unexpected varchar parse result: %q %d %v", baseType, length, unsigned)
	}

	baseType, length, unsigned = parseColumnType("bigint(20) unsigned")
	if baseType != "bigint" || length != 20 || !unsigned {
		t.Fatalf("unexpected bigint parse result: %q %d %v", baseType, length, unsigned)
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
