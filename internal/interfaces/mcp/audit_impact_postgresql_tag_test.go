//go:build postgresql

// Package mcpapi verifies PostgreSQL offline impact rendering.
package mcpapi

import (
	"testing"
)

func TestAuditSQLToolPostgreSQLOfflineIDEqualityImpact(t *testing.T) {
	result := callAuditSQL(t, map[string]any{
		"sql":     "delete from users where id = 42",
		"dialect": "postgresql",
	})
	body := requireAuditStructuredMap(t, result)
	statements, ok := body["statements"].([]any)
	if !ok || len(statements) != 1 {
		t.Fatalf("expected one statement, got %#v", body["statements"])
	}
	statement, ok := statements[0].(map[string]any)
	if !ok {
		t.Fatalf("expected statement object, got %#v", statements[0])
	}
	impact, ok := statement["impact"].(map[string]any)
	if !ok {
		t.Fatalf("expected impact object, got %#v", statement["impact"])
	}
	if impact["estimated_rows"] != float64(1) || impact["risk_level"] != "low" || impact["confidence"] != "high" || impact["source"] != "shape" {
		t.Fatalf("unexpected impact object: %#v", impact)
	}
	reasonCodes, ok := impact["reason_codes"].([]any)
	if !ok || len(reasonCodes) != 1 || reasonCodes[0] != "pk_equality" {
		t.Fatalf("reason codes = %#v, want [pk_equality]", impact["reason_codes"])
	}
}
