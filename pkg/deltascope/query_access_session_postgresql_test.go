//go:build postgresql

// Package deltascope verifies PostgreSQL session behavior without Docker.
// input: AnalyzeQueryAccess with PostgreSQL dialect
// output: regression coverage for default PostgreSQL fail-closed behavior
// pos: postgresql-tagged non-integration test for default fail-closed
// note: if this file changes, update this header and module README.md.
package deltascope

import (
	"testing"
)

func TestSession_PostgreSQLDefaultFailClosed(t *testing.T) {
	// Verify that AnalyzeQueryAccess with PostgreSQL dialect returns
	// indeterminate without constructing a trusted service or session.
	// This tests the default fail-closed path — no Docker needed.
	result, err := AnalyzeQueryAccess(t.Context(), QueryAccessRequest{
		SQL:           "SELECT count(*) FROM app.users",
		Dialect:       DialectPostgreSQL,
		Mode:          QueryAccessModeStrict,
		DefaultSchema: "app",
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if result.Admission != QueryAccessIndeterminateAdmission {
		t.Errorf("expected indeterminate admission, got %s", result.Admission)
	}
	if result.ReadClassification != QueryAccessIndeterminate {
		t.Errorf("expected indeterminate classification, got %s", result.ReadClassification)
	}
}

func TestNewTrustedServiceFromSession_NilSession(t *testing.T) {
	svc, err := newTrustedServiceFromSession(nil)
	if err == nil {
		t.Fatal("expected error for nil session")
	}
	if svc != nil {
		t.Fatal("expected nil service on error")
	}
}

func TestSession_PostgreSQLComparisonFailClosed(t *testing.T) {
	// Schema-qualified comparison also remains indeterminate in default path.
	result, err := AnalyzeQueryAccess(t.Context(), QueryAccessRequest{
		SQL:           "SELECT u.id FROM app.users u JOIN app.orders o ON u.id = o.user_id",
		Dialect:       DialectPostgreSQL,
		Mode:          QueryAccessModeStrict,
		DefaultSchema: "app",
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if result.Admission != QueryAccessIndeterminateAdmission {
		t.Errorf("expected indeterminate admission, got %s", result.Admission)
	}
}
