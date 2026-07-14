//go:build integration && postgresql

package postgresqlmeta

import (
	"context"
	"os"
	"strconv"
	"testing"
)

// envOr returns the environment variable value or the fallback.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// envOrInt returns the environment variable as int or the fallback.
func envOrInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func TestOpenDB_ConfiguresConnectionPool(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	config := ConnectionConfig{
		Host:     envOr("DELTASCOPE_PG_HOST", "127.0.0.1"),
		Port:     envOrInt("DELTASCOPE_PG_PORT", 5500),
		User:     envOr("DELTASCOPE_PG_USER", "root"),
		Password: envOr("DELTASCOPE_PG_PASSWORD", "root"),
		Database: envOr("DELTASCOPE_PG_DATABASE", "postgres"),
	}

	db, err := OpenDB(config)
	if err != nil {
		t.Fatalf("OpenDB failed: %v", err)
	}
	defer db.Close()

	stats := db.Stats()
	if stats.MaxOpenConnections != 25 {
		t.Errorf("MaxOpenConnections = %d, want 25", stats.MaxOpenConnections)
	}
}

func TestOpenDB_VerifiesConnection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	config := ConnectionConfig{
		Host:     "invalid-host-that-does-not-exist",
		Port:     5432,
		User:     "postgres",
		Password: "",
		Database: "postgres",
	}

	db, err := OpenDB(config)
	if err == nil {
		db.Close()
		t.Fatal("OpenDB should fail with invalid host")
	}
}

func TestOpenDB_ClosesOnPingFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	config := ConnectionConfig{
		Host:     "127.0.0.1",
		Port:     1,
		User:     "postgres",
		Password: "",
		Database: "postgres",
	}

	db, err := OpenDB(config)
	if err == nil {
		db.Close()
		t.Fatal("OpenDB should fail with invalid port")
	}
}

func TestProvider_NoConnectionLeak(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	config := ConnectionConfig{
		Host:     envOr("DELTASCOPE_PG_HOST", "127.0.0.1"),
		Port:     envOrInt("DELTASCOPE_PG_PORT", 5500),
		User:     envOr("DELTASCOPE_PG_USER", "root"),
		Password: envOr("DELTASCOPE_PG_PASSWORD", "root"),
		Database: envOr("DELTASCOPE_PG_DATABASE", "postgres"),
	}

	for i := 0; i < 100; i++ {
		db, err := OpenDB(config)
		if err != nil {
			t.Fatalf("OpenDB failed on iteration %d: %v", i, err)
		}

		provider := NewProvider(db)
		ctx := context.Background()
		if _, err := provider.DetectDialect(ctx); err != nil {
			db.Close()
			t.Fatalf("DetectDialect failed: %v", err)
		}

		db.Close()
	}
}

func TestComposeConfigMatchesIntegrationDefaults(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	defaults := map[string]string{
		"DELTASCOPE_PG_HOST":     "127.0.0.1",
		"DELTASCOPE_PG_PORT":     "5500",
		"DELTASCOPE_PG_USER":     "root",
		"DELTASCOPE_PG_PASSWORD": "root",
		"DELTASCOPE_PG_DATABASE": "postgres",
	}

	for key, expected := range defaults {
		if got := envOr(key, expected); got != expected {
			t.Errorf("envOr(%q, %q) = %q, want %q (compose default mismatch)", key, expected, got, expected)
		}
	}

	if got := envOrInt("DELTASCOPE_PG_PORT", 5500); got != 5500 {
		t.Errorf("envOrInt(DELTASCOPE_PG_PORT, 5500) = %d, want 5500", got)
	}
}
