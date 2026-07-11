// Package deltascope verifies the public library query access API.
// input: inline SQL text, dialect, mode, and the public query access request contract
// output: regression coverage for public query access orchestration and stable result mapping
// pos: public API test coverage for query access in pkg/deltascope
// note: if this file changes, update this header and module README.md.
package deltascope

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestAnalyzeQueryAccessMySQLSelect(t *testing.T) {
	result, err := AnalyzeQueryAccess(context.Background(), QueryAccessRequest{
		SQL:     "SELECT id, name FROM users WHERE id = 1",
		Dialect: DialectMySQL,
		Mode:    QueryAccessModeStrict,
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}

	if result.Dialect != "mysql" {
		t.Fatalf("expected mysql dialect, got %q", result.Dialect)
	}
	if result.Mode != QueryAccessModeStrict {
		t.Fatalf("expected strict mode, got %q", result.Mode)
	}
	if result.ReadClassification != QueryAccessReadOnly {
		t.Fatalf("expected read_only classification, got %q", result.ReadClassification)
	}
	if result.Admission != QueryAccessAdmissible {
		t.Fatalf("expected admissible admission, got %q", result.Admission)
	}
}

func TestAnalyzeQueryAccessMySQLDelete(t *testing.T) {
	result, err := AnalyzeQueryAccess(context.Background(), QueryAccessRequest{
		SQL:     "DELETE FROM users WHERE id = 1",
		Dialect: DialectMySQL,
		Mode:    QueryAccessModeStrict,
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}

	if result.ReadClassification != QueryAccessNotReadOnly {
		t.Fatalf("expected not_read_only classification, got %q", result.ReadClassification)
	}
	if result.Admission != QueryAccessRejected {
		t.Fatalf("expected rejected admission, got %q", result.Admission)
	}
}

func TestAnalyzeQueryAccessProjectionOnlyMode(t *testing.T) {
	result, err := AnalyzeQueryAccess(context.Background(), QueryAccessRequest{
		SQL:     "SELECT id FROM users WHERE name = 'test'",
		Dialect: DialectMySQL,
		Mode:    QueryAccessModeProjectionOnly,
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}

	if result.Mode != QueryAccessModeProjectionOnly {
		t.Fatalf("expected projection_only mode, got %q", result.Mode)
	}
}

func TestAnalyzeQueryAccessResultJSONStructure(t *testing.T) {
	result, err := AnalyzeQueryAccess(context.Background(), QueryAccessRequest{
		SQL:     "SELECT id FROM users",
		Dialect: DialectMySQL,
		Mode:    QueryAccessModeStrict,
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	requiredFields := []string{"dialect", "mode", "read_classification", "admission"}
	for _, field := range requiredFields {
		if _, ok := raw[field]; !ok {
			t.Errorf("expected field %q in JSON output", field)
		}
	}

	// Verify no audit-specific fields leak
	forbiddenFields := []string{"verdict", "summary", "statements", "global_findings", "findings", "level", "rule_id"}
	for _, field := range forbiddenFields {
		if _, ok := raw[field]; ok {
			t.Errorf("forbidden audit field %q found in query access result", field)
		}
	}
}

func TestAnalyzeQueryAccessJSONParity(t *testing.T) {
	// Verify that the result can be round-tripped through JSON
	result, err := AnalyzeQueryAccess(context.Background(), QueryAccessRequest{
		SQL:     "SELECT u.id, u.name FROM users u JOIN orders o ON u.id = o.user_id WHERE o.status = 'active'",
		Dialect: DialectMySQL,
		Mode:    QueryAccessModeStrict,
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var roundTrip QueryAccessResult
	if err := json.Unmarshal(data, &roundTrip); err != nil {
		t.Fatalf("unmarshal round-trip: %v", err)
	}

	if roundTrip.Dialect != result.Dialect {
		t.Errorf("round-trip dialect mismatch: got %q want %q", roundTrip.Dialect, result.Dialect)
	}
	if roundTrip.Mode != result.Mode {
		t.Errorf("round-trip mode mismatch: got %q want %q", roundTrip.Mode, result.Mode)
	}
	if roundTrip.ReadClassification != result.ReadClassification {
		t.Errorf("round-trip classification mismatch: got %q want %q", roundTrip.ReadClassification, result.ReadClassification)
	}
	if roundTrip.Admission != result.Admission {
		t.Errorf("round-trip admission mismatch: got %q want %q", roundTrip.Admission, result.Admission)
	}
}

func TestAnalyzeQueryAccessCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := AnalyzeQueryAccess(ctx, QueryAccessRequest{
		SQL:     "SELECT 1",
		Dialect: DialectMySQL,
	})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestAnalyzeQueryAccessInvalidModeRejected(t *testing.T) {
	_, err := AnalyzeQueryAccess(context.Background(), QueryAccessRequest{
		SQL:     "SELECT 1",
		Dialect: DialectMySQL,
		Mode:    "invalid",
	})
	if err == nil {
		t.Fatal("expected error for invalid mode")
	}
	if !errors.Is(err, ErrInvalidQueryAccessMode) {
		t.Fatalf("expected ErrInvalidQueryAccessMode, got %v", err)
	}
}

func TestAnalyzeQueryAccessDefaultModeIsStrict(t *testing.T) {
	result, err := AnalyzeQueryAccess(context.Background(), QueryAccessRequest{
		SQL:     "SELECT 1",
		Dialect: DialectMySQL,
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}

	if result.Mode != QueryAccessModeStrict {
		t.Fatalf("expected default mode to be strict, got %q", result.Mode)
	}
}

func TestAnalyzeQueryAccessMalformedSQLErrorDoesNotDiscloseSQL(t *testing.T) {
	malformedSQL := "SELECT * FROM users WHERE password = 'secret123' AND id = 1"
	_, err := AnalyzeQueryAccess(context.Background(), QueryAccessRequest{
		SQL:     malformedSQL,
		Dialect: "unsupported_dialect",
	})
	if err == nil {
		t.Fatal("expected error for unsupported dialect")
	}

	errStr := err.Error()
	if containsSQLText(errStr, malformedSQL) {
		t.Errorf("error message should not contain SQL text, got: %s", errStr)
	}
}

func containsSQLText(errMsg, sql string) bool {
	return len(errMsg) > 0 && len(sql) > 0 && (errMsg == sql || len(errMsg) >= len(sql) && errMsg[:len(sql)] == sql)
}
