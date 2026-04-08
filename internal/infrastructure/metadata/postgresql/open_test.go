// Package postgresqlmeta verifies PostgreSQL metadata connection config helpers.
// input: synthetic TCP and unix-socket PostgreSQL connection configs
// output: stable DSN/address formatting behavior without opening a live database
// pos: infrastructure connection helper test coverage
// note: if this file changes, update this header and module README.md.
package postgresqlmeta

import (
	"strings"
	"testing"
)

func TestConnectionConfigDSNUsesTCPHostPortAndDatabase(t *testing.T) {
	config := ConnectionConfig{
		Host:     "127.0.0.1",
		Port:     5433,
		Database: "appdb",
		User:     "postgres",
		Password: "secret",
		SSLMode:  "disable",
	}

	if got := config.Address(); got != "127.0.0.1:5433" {
		t.Fatalf("expected tcp address 127.0.0.1:5433, got %q", got)
	}
	if got := config.DatabaseName(); got != "appdb" {
		t.Fatalf("expected database appdb, got %q", got)
	}
	if got := config.DSN(); !strings.Contains(got, "postgres://postgres:secret@127.0.0.1:5433/appdb?sslmode=disable") {
		t.Fatalf("expected postgres URL DSN, got %q", got)
	}
}

func TestConnectionConfigDSNUsesUnixSocket(t *testing.T) {
	config := ConnectionConfig{
		Socket:   "/tmp",
		Database: "appdb",
		User:     "postgres",
		Password: "secret",
		SSLMode:  "disable",
	}

	if got := config.Address(); got != "/tmp" {
		t.Fatalf("expected socket address /tmp, got %q", got)
	}
	if got := config.DSN(); !strings.Contains(got, "host=%2Ftmp") {
		t.Fatalf("expected unix socket host in DSN, got %q", got)
	}
}
