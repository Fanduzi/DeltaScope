// Package postgresqlmeta verifies PostgreSQL metadata connection config helpers.
// input: synthetic TCP and unix-socket PostgreSQL connection configs
// output: stable DSN/address formatting behavior without opening a live database
// pos: infrastructure connection helper test coverage
// note: if this file changes, update this header and module README.md.
package postgresqlmeta

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
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
	if got := config.DSN(); !strings.Contains(got, "postgres://postgres:secret@127.0.0.1:5433/appdb?") {
		t.Fatalf("expected postgres URL DSN, got %q", got)
	}
	if got := config.DSN(); !strings.Contains(got, "sslmode=disable") {
		t.Fatalf("expected sslmode in DSN, got %q", got)
	}
	if got := config.DSN(); !strings.Contains(got, "connect_timeout=5") {
		t.Fatalf("expected default connect_timeout=5 in DSN, got %q", got)
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
	if got := config.DSN(); !strings.Contains(got, "connect_timeout=5") {
		t.Fatalf("expected default connect_timeout in DSN, got %q", got)
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

func TestConnectTimeoutDSNReflectsCustomValue(t *testing.T) {
	config := ConnectionConfig{
		Host:           "db.example.com",
		Port:           5432,
		Database:       "mydb",
		User:           "admin",
		Password:       "pw",
		ConnectTimeout: 15 * time.Second,
	}
	dsn := config.DSN()
	if !strings.Contains(dsn, "connect_timeout=15") {
		t.Fatalf("expected connect_timeout=15 in DSN, got %q", dsn)
	}
}

func TestOpenDBContextReturnsCanceledWhenCtxAlreadyCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	config := ConnectionConfig{
		Host: "127.0.0.1",
		Port: 5432,
		User: "postgres",
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
		Port:           5432,
		User:           "postgres",
		ConnectTimeout: time.Millisecond,
	}
	db, err := OpenDBContext(nil, config)
	if db != nil {
		_ = db.Close()
	}
	_ = err
}

func TestConnectTimeoutDSNCeilsSubsecondTimeout(t *testing.T) {
	config := ConnectionConfig{
		Host:           "127.0.0.1",
		Port:           5432,
		Database:       "mydb",
		User:           "admin",
		Password:       "pw",
		ConnectTimeout: 1500 * time.Millisecond,
	}
	dsn := config.DSN()
	if !strings.Contains(dsn, "connect_timeout=2") {
		t.Fatalf("expected connect_timeout=2 (ceiled from 1.5s) in DSN, got %q", dsn)
	}
}

func TestConnectionConfigSSLModeVerifyFull(t *testing.T) {
	config := ConnectionConfig{
		Host:     "127.0.0.1",
		Port:     5432,
		Database: "mydb",
		User:     "admin",
		Password: "pw",
		SSLMode:  "verify-full",
	}

	dsn := config.DSN()
	if !strings.Contains(dsn, "sslmode=verify-full") {
		t.Fatalf("expected sslmode=verify-full in DSN, got %q", dsn)
	}
}

func TestConnectionConfigSSLModeDisable(t *testing.T) {
	config := ConnectionConfig{
		Host:     "127.0.0.1",
		Port:     5432,
		Database: "mydb",
		User:     "admin",
		Password: "pw",
		SSLMode:  "disable",
	}

	dsn := config.DSN()
	if !strings.Contains(dsn, "sslmode=disable") {
		t.Fatalf("expected sslmode=disable in DSN, got %q", dsn)
	}
}

func TestConnectionConfigSSLModeEmptyDefaultsToDisable(t *testing.T) {
	config := ConnectionConfig{
		Host:     "127.0.0.1",
		Port:     5432,
		Database: "mydb",
		User:     "admin",
		Password: "pw",
	}

	dsn := config.DSN()
	if !strings.Contains(dsn, "sslmode=disable") {
		t.Fatalf("expected sslmode=disable as default, got %q", dsn)
	}
}

func TestConnectionConfigDatabaseDefaultsToPostgres(t *testing.T) {
	config := ConnectionConfig{
		Host:     "127.0.0.1",
		Port:     5432,
		User:     "admin",
		Password: "pw",
	}

	if got := config.DatabaseName(); got != "postgres" {
		t.Fatalf("expected default database postgres, got %q", got)
	}
}
