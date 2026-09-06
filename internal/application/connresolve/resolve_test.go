// Package connresolve verifies Resolve before open.
// input: connection requests with TLS, password, timeout, and catalog hints
// output: ready-to-open Config or classified Error
// pos: application tests at the Resolve seam
// note: if this file changes, update this header and module README.md.
package connresolve

import (
	"errors"
	"testing"
)

func TestResolveBindsMySQLCatalogAndPassword(t *testing.T) {
	t.Parallel()

	cfg, err := Resolve(Request{
		Host:     "127.0.0.1",
		Port:     3306,
		User:     "root",
		Database: "app",
		Dialect:  "mysql",
		Password: "secret",
		TLSMode:  "disabled",
	}, Options{})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if cfg.Database != "app" || cfg.Schema != "app" || cfg.Qualifier != "app" {
		t.Fatalf("catalog: database=%q schema=%q qualifier=%q", cfg.Database, cfg.Schema, cfg.Qualifier)
	}
	if cfg.Password != "secret" || cfg.TLSMode != "disabled" {
		t.Fatalf("password/tls: %#v", cfg)
	}
}

func TestResolvePostgreSQLOmittedPort(t *testing.T) {
	t.Parallel()

	cfg, err := Resolve(Request{
		Host:            "127.0.0.1",
		Port:            3306,
		User:            "postgres",
		Dialect:         "postgresql",
		DialectExplicit: true,
	}, Options{})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if cfg.Port != 5432 {
		t.Fatalf("port=%d, want 5432", cfg.Port)
	}
}

func TestResolveRejectsCatalogConflict(t *testing.T) {
	t.Parallel()

	_, err := Resolve(Request{
		Host:     "127.0.0.1",
		User:     "root",
		Database: "app",
		Schema:   "other",
		Dialect:  "mysql",
	}, Options{})
	var resolved *Error
	if !errors.As(err, &resolved) || resolved.Code != CodeCatalogConflict || resolved.Class != ClassValidation {
		t.Fatalf("got %#v", err)
	}
}

func TestResolveRejectsTLSWithoutHost(t *testing.T) {
	t.Parallel()

	_, err := Resolve(Request{
		User:    "root",
		TLSMode: "enabled",
	}, Options{})
	var resolved *Error
	if !errors.As(err, &resolved) || resolved.Code != CodeTLSRequiresHost {
		t.Fatalf("got %#v", err)
	}
}
