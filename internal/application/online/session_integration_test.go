//go:build integration

// Package online provides integration tests for the session factory against real databases.
// input: Docker-backed MySQL and PostgreSQL services
// output: verified identity detection and capability target derivation
// pos: integration tests for OpenSession with real database connections
// note: if this file changes, update this header and module README.md.
package online

import (
	"context"
	"testing"
	"time"
)

func TestOpenSession_MySQL_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	cfg := SessionConfig{
		Host:           "127.0.0.1",
		Port:           3306,
		User:           "root",
		Password:       "root",
		Database:       "test",
		Dialect:        "mysql",
		ConnectTimeout: 5 * time.Second,
		TLSMode:        "disabled",
	}

	session, err := OpenSession(ctx, cfg)
	if err != nil {
		t.Fatalf("OpenSession: %v", err)
	}
	defer session.Close()

	if session.Identity == nil {
		t.Fatal("session.Identity is nil")
	}
	if session.Identity.Product != ProductMySQL {
		t.Errorf("Identity.Product = %q, want %q", session.Identity.Product, ProductMySQL)
	}
	if session.Target == "" {
		t.Error("session.Target is empty")
	}

	t.Logf("MySQL version: %d.%d.%d, series: %s, target: %s",
		session.Identity.Major, session.Identity.Minor, session.Identity.Patch,
		session.Identity.Series, session.Target)
}

func TestOpenSession_PostgreSQL_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	cfg := SessionConfig{
		Host:           "127.0.0.1",
		Port:           5432,
		User:           "postgres",
		Password:       "postgres",
		Database:       "test",
		Dialect:        "postgresql",
		ConnectTimeout: 5 * time.Second,
		TLSMode:        "disabled",
	}

	session, err := OpenSession(ctx, cfg)
	if err != nil {
		t.Fatalf("OpenSession: %v", err)
	}
	defer session.Close()

	if session.Identity == nil {
		t.Fatal("session.Identity is nil")
	}
	if session.Identity.Product != ProductPostgreSQL {
		t.Errorf("Identity.Product = %q, want %q", session.Identity.Product, ProductPostgreSQL)
	}
	if session.Identity.Series != SeriesPG17 {
		t.Errorf("Identity.Series = %q, want %q", session.Identity.Series, SeriesPG17)
	}
	if session.Target != TargetPG17 {
		t.Errorf("Target = %q, want %q", session.Target, TargetPG17)
	}

	t.Logf("PostgreSQL version: %d.%d.%d, series: %s, target: %s",
		session.Identity.Major, session.Identity.Minor, session.Identity.Patch,
		session.Identity.Series, session.Target)
}

func TestOpenSession_DoubleClose_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	cfg := SessionConfig{
		Host:           "127.0.0.1",
		Port:           3306,
		User:           "root",
		Password:       "root",
		Database:       "test",
		Dialect:        "mysql",
		ConnectTimeout: 5 * time.Second,
		TLSMode:        "disabled",
	}

	session, err := OpenSession(ctx, cfg)
	if err != nil {
		t.Fatalf("OpenSession: %v", err)
	}

	if err := session.Close(); err != nil {
		t.Fatalf("first close: %v", err)
	}

	if err := session.Close(); err != nil {
		t.Fatalf("second close should not error, got: %v", err)
	}
}
