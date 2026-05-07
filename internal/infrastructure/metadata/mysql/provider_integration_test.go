//go:build integration

package mysqlmeta

import (
	"context"
	"testing"
)

func TestOpenDB_ConfiguresConnectionPool(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	config := ConnectionConfig{
		Host:     "127.0.0.1",
		Port:     3306,
		User:     "root",
		Password: "",
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
		Port:     3306,
		User:     "root",
		Password: "",
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
		User:     "root",
		Password: "",
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
		Host:     "127.0.0.1",
		Port:     3306,
		User:     "root",
		Password: "",
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
