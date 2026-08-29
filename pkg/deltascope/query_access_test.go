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
	"reflect"
	"strings"
	"testing"
)

func TestAnalyzeQueryAccessLeadingUTF8BOMMatchesBOMFreeInput(t *testing.T) {
	t.Parallel()

	const sql = "SELECT id FROM users\r\nWHERE id = 1"
	want, err := AnalyzeQueryAccess(context.Background(), QueryAccessRequest{
		SQL: sql, Dialect: DialectMySQL, Mode: QueryAccessModeStrict,
	})
	if err != nil {
		t.Fatalf("BOM-free analysis: %v", err)
	}
	got, err := AnalyzeQueryAccess(context.Background(), QueryAccessRequest{
		SQL: "\ufeff" + sql, Dialect: DialectMySQL, Mode: QueryAccessModeStrict,
	})
	if err != nil {
		t.Fatalf("BOM-prefixed analysis: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BOM-prefixed result differs from BOM-free result:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestAnalyzeQueryAccessBOMOnlyInputIsRejectedAsEmpty(t *testing.T) {
	t.Parallel()

	for _, sql := range []string{"\ufeff", "\ufeff \r\n"} {
		_, err := AnalyzeQueryAccess(context.Background(), QueryAccessRequest{
			SQL: sql, Dialect: DialectMySQL,
		})
		if err == nil || !strings.Contains(strings.ToLower(err.Error()), "empty") {
			t.Errorf("SQL %q: error = %v, want empty-input error", sql, err)
		}
	}
}

func TestAnalyzeQueryAccessBOMFreeEmptyInputKeepsZeroStatements(t *testing.T) {
	t.Parallel()

	result, err := AnalyzeQueryAccess(context.Background(), QueryAccessRequest{
		SQL: "", Dialect: DialectMySQL,
	})
	if err != nil {
		t.Fatalf("empty analysis: %v", err)
	}
	if !reflect.DeepEqual(result.ReasonCodes, []string{"zero_statements"}) {
		t.Fatalf("reason codes = %v, want [zero_statements]", result.ReasonCodes)
	}
}

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
	return errMsg != "" && sql != "" && (errMsg == sql || len(errMsg) >= len(sql) && errMsg[:len(sql)] == sql)
}

func TestAnalyzeQueryAccess_MixedConstOperand_OfflineIndeterminate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		sql     string
		dialect Dialect
	}{
		{
			name:    "mysql_coalesce_mixed_const",
			sql:     "SELECT COALESCE(name, 'unknown') FROM users",
			dialect: DialectMySQL,
		},
		{
			name:    "tidb_coalesce_mixed_const",
			sql:     "SELECT COALESCE(name, 'unknown') FROM users",
			dialect: DialectTiDB,
		},
		{
			name:    "mysql_nullif_mixed_const",
			sql:     "SELECT NULLIF(name, 'unknown') FROM users",
			dialect: DialectMySQL,
		},
		{
			name:    "mysql_ifnull_mixed_const",
			sql:     "SELECT IFNULL(name, 'unknown') FROM users",
			dialect: DialectMySQL,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result, err := AnalyzeQueryAccess(context.Background(), QueryAccessRequest{
				SQL:     tc.sql,
				Dialect: tc.dialect,
				Mode:    QueryAccessModeStrict,
			})
			if err != nil {
				t.Fatalf("analyze: %v", err)
			}
			// Default SDK (no session) must remain indeterminate for function-bearing SQL.
			if result.ReadClassification != QueryAccessIndeterminate {
				t.Errorf("expected indeterminate, got %q", result.ReadClassification)
			}
			if result.Admission != QueryAccessIndeterminateAdmission {
				t.Errorf("expected indeterminate admission, got %q", result.Admission)
			}
		})
	}
}

func TestAnalyzeQueryAccess_LiteralAndReversedOperands_OfflineIndeterminate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name           string
		sql            string
		literalMarkers []string
	}{
		{
			name:           "literal_only_lower",
			sql:            "SELECT LOWER('x') FROM app.users",
			literalMarkers: []string{"'x'"},
		},
		{
			name:           "count_literal",
			sql:            "SELECT COUNT(1) FROM app.orders",
			literalMarkers: []string{"1"},
		},
		{
			name:           "reversed_coalesce",
			sql:            "SELECT COALESCE('x', name) FROM app.users",
			literalMarkers: []string{"'x'"},
		},
		{
			name:           "all_constant_coalesce",
			sql:            "SELECT COALESCE('x', 'y') FROM app.users",
			literalMarkers: []string{"'x'", "'y'"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result, err := AnalyzeQueryAccess(context.Background(), QueryAccessRequest{
				SQL:     tc.sql,
				Dialect: DialectMySQL,
				Mode:    QueryAccessModeStrict,
			})
			if err != nil {
				t.Fatalf("analyze: %v", err)
			}
			if result.ReadClassification != QueryAccessIndeterminate {
				t.Errorf("expected indeterminate classification, got %q", result.ReadClassification)
			}
			if result.Admission != QueryAccessIndeterminateAdmission {
				t.Errorf("expected indeterminate admission, got %q", result.Admission)
			}

			dump := stringifyResult(result)
			data, err := json.Marshal(result)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			jsonDump := string(data)
			for _, marker := range mysqlTiDBProbeNoLeakMarkers {
				if strings.Contains(dump, marker) {
					t.Errorf("SDK struct dump leaked internal marker %q: %s", marker, dump)
				}
				if strings.Contains(jsonDump, marker) {
					t.Errorf("SDK JSON leaked internal marker %q: %s", marker, jsonDump)
				}
			}
			for _, literal := range tc.literalMarkers {
				if strings.Contains(dump, literal) {
					t.Errorf("SDK struct dump leaked literal %q: %s", literal, dump)
				}
				if strings.Contains(jsonDump, literal) {
					t.Errorf("SDK JSON leaked literal %q: %s", literal, jsonDump)
				}
			}
			if strings.Contains(dump, tc.sql) || strings.Contains(jsonDump, tc.sql) {
				t.Errorf("public output leaked raw SQL %q", tc.sql)
			}
		})
	}
}
