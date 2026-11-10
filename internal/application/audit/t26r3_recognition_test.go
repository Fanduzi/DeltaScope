package audit

// T26-R3 TDD: recognition, named-boundary, and output-redaction assertions
// for the MySQL SERVER family, spatial reference systems, and the two
// inventory-owned ALTER INSTANCE subactions (issue #105 T26-R3).
// Scope is recognition + named boundaries + safe output only — no SERVER
// or SRS lifecycle semantics are claimed.

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

const t26GoldenServer = "CREATE SERVER golden_srv FOREIGN DATA WRAPPER mysql OPTIONS (HOST '192.0.2.1', DATABASE 'app', USER 'u', PASSWORD 'golden-secret')"

func t26Audit(t *testing.T, dialect spec.Dialect, sql string) (report.Result, error) {
	t.Helper()
	return AuditSQL(context.Background(), Request{SQL: sql, Dialect: dialect})
}

func t26OneBoundary(t *testing.T, dialect spec.Dialect, sql string) spec.UnsupportedDetail {
	t.Helper()
	result, err := t26Audit(t, dialect, sql)
	if !errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("%s AuditSQL(%q): err=%v want ErrUnsupportedStatement (named boundary)", dialect, sql, err)
	}
	if len(result.Unsupported) != 1 {
		t.Fatalf("%s AuditSQL(%q): unsupported=%+v want exactly one entry", dialect, sql, result.Unsupported)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("%s AuditSQL(%q): statements=%d want 1 — identity lost", dialect, sql, len(result.Statements))
	}
	return result.Unsupported[0]
}

func TestT26R3_ServerRecognitionBoundary(t *testing.T) {
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		u := t26OneBoundary(t, dialect, t26GoldenServer)
		if u.Feature != "create_server" {
			t.Fatalf("%s: feature=%q want create_server", dialect, u.Feature)
		}
		switch dialect {
		case spec.DialectTiDB:
			if u.Reason != spec.UnsupportedVendorBoundaryReason {
				t.Fatalf("tidb: reason=%q want vendor boundary", u.Reason)
			}
		case spec.DialectMySQL:
			if u.Reason != spec.UnsupportedUnauditedReason {
				t.Fatalf("mysql: reason=%q want named unaudited boundary", u.Reason)
			}
		}
	}
}

func TestT26R3_ServerForms(t *testing.T) {
	for _, f := range []struct{ sql, feature string }{
		{"ALTER SERVER golden_srv OPTIONS (HOST 'db2.example.com')", "alter_server"},
		{"DROP SERVER golden_srv", "drop_server"},
		{"DROP SERVER IF EXISTS golden_srv", "drop_server"},
	} {
		u := t26OneBoundary(t, spec.DialectMySQL, f.sql)
		if u.Feature != f.feature {
			t.Fatalf("%s: feature=%q want %s", f.sql, u.Feature, f.feature)
		}
	}
}

func TestT26R3_ServerStrictRejects(t *testing.T) {
	// Ambiguous or malformed forms must never be half-claimed — they keep
	// the base grammar's own parser diagnostic.
	for _, sql := range []string{
		"CREATE SERVER s FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h') trailing",
		"CREATE SERVER s OPTIONS (HOST 'h')",
		"DROP SERVER s extra",
	} {
		result, err := t26Audit(t, spec.DialectMySQL, sql)
		if err == nil {
			t.Fatalf("%q: unexpectedly clean audit", sql)
		}
		for _, u := range result.Unsupported {
			if strings.HasPrefix(u.Feature, "create_server") || strings.HasPrefix(u.Feature, "alter_server") || strings.HasPrefix(u.Feature, "drop_server") {
				t.Fatalf("%q: half-claimed as %q — must stay a parser diagnostic", sql, u.Feature)
			}
		}
	}
}

func TestT26R3_SRSSevenForms(t *testing.T) {
	for _, f := range []struct{ sql, feature string }{
		{"CREATE SPATIAL REFERENCE SYSTEM 4120 NAME 'Greek' DEFINITION 'GEOGCS[\"Greek\"]'", "create_spatial_reference_system"},
		{"CREATE SPATIAL REFERENCE SYSTEM IF NOT EXISTS 4120 NAME 'Greek' DEFINITION 'x'", "create_spatial_reference_system"},
		{"CREATE OR REPLACE SPATIAL REFERENCE SYSTEM 40001 NAME 'X' DEFINITION 'x'", "create_spatial_reference_system"},
		{"CREATE SPATIAL REFERENCE SYSTEM 4120", "create_spatial_reference_system"},
		{"DROP SPATIAL REFERENCE SYSTEM 40001", "drop_spatial_reference_system"},
		{"DROP SPATIAL REFERENCE SYSTEM IF EXISTS 40001", "drop_spatial_reference_system"},
	} {
		for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
			u := t26OneBoundary(t, dialect, f.sql)
			if u.Feature != f.feature {
				t.Fatalf("%s %s: feature=%q want %s", dialect, f.sql, u.Feature, f.feature)
			}
		}
	}
}

func TestT26R3_SRSBoundariesDistinct(t *testing.T) {
	// MySQL gets the named implementation boundary; TiDB gets the vendor
	// boundary — the two must not collapse into one reason.
	sql := "CREATE SPATIAL REFERENCE SYSTEM 4120 NAME 'G' DEFINITION 'x'"
	m := t26OneBoundary(t, spec.DialectMySQL, sql)
	td := t26OneBoundary(t, spec.DialectTiDB, sql)
	if td.Reason != spec.UnsupportedVendorBoundaryReason {
		t.Fatalf("tidb reason=%q want vendor boundary", td.Reason)
	}
	if m.Reason != spec.UnsupportedUnauditedReason {
		t.Fatalf("mysql reason=%q want named unaudited boundary", m.Reason)
	}
}

func TestT26R3_SRSStrictNumeric(t *testing.T) {
	// Negative and overflowing SRIDs are grammar-invalid — the recognizer
	// must refuse the claim and let the base parser own the diagnostic;
	// a silent uint conversion must never produce a named boundary.
	for _, sql := range []string{
		"CREATE SPATIAL REFERENCE SYSTEM -1 NAME 'x' DEFINITION 'x'",
		"CREATE SPATIAL REFERENCE SYSTEM 4294967296 NAME 'x' DEFINITION 'x'",
		"DROP SPATIAL REFERENCE SYSTEM -1",
		"DROP SPATIAL REFERENCE SYSTEM 99999999999",
	} {
		result, err := t26Audit(t, spec.DialectMySQL, sql)
		if err == nil {
			t.Fatalf("%q: unexpectedly clean audit", sql)
		}
		for _, u := range result.Unsupported {
			if strings.Contains(u.Feature, "spatial_reference_system") {
				t.Fatalf("%q: invalid SRID claimed as %q", sql, u.Feature)
			}
		}
	}
}

func TestT26R3_AlterInstanceSubactions(t *testing.T) {
	// The two inventory-owned subactions get precise feature names.
	for _, c := range []struct{ sql, feature string }{
		{"ALTER INSTANCE RELOAD TLS", "alter_instance.reload_tls"},
		{"ALTER INSTANCE ROTATE INNODB MASTER KEY", "alter_instance.rotate_innodb_master_key"},
	} {
		u := t26OneBoundary(t, spec.DialectMySQL, c.sql)
		if u.Feature != c.feature {
			t.Fatalf("%s: feature=%q want %s", c.sql, u.Feature, c.feature)
		}
	}
	// The NO ROLLBACK member is recorded, not falsely merged into the
	// plain form — extended subactions stay named but distinct.
	u := t26OneBoundary(t, spec.DialectMySQL, "ALTER INSTANCE RELOAD TLS NO ROLLBACK ON ERROR")
	if u.Feature != "alter_instance.reload_tls" {
		t.Fatalf("NO ROLLBACK variant: feature=%q want alter_instance.reload_tls", u.Feature)
	}
	if v, _ := u.Metadata["no_rollback_on_error"].(bool); !v {
		t.Fatalf("NO ROLLBACK variant: metadata=%+v want no_rollback_on_error=true", u.Metadata)
	}
}

func TestT26R3_OutputRedaction(t *testing.T) {
	result, err := t26Audit(t, spec.DialectMySQL, t26GoldenServer)
	if !errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("err=%v want ErrUnsupportedStatement", err)
	}
	raw, merr := json.Marshal(result)
	if merr != nil {
		t.Fatalf("marshal: %v", merr)
	}
	full := string(raw)
	for _, marker := range []string{"golden-secret", "192.0.2.1"} {
		if strings.Contains(full, marker) {
			t.Fatalf("marker %q leaked into product output: %s", marker, full)
		}
	}
	stmt := result.Statements[0]
	if stmt.RawSQL == t26GoldenServer || stmt.RawSQL == "" {
		t.Fatalf("raw_sql=%q — verbatim leak or lost identity", stmt.RawSQL)
	}
	if !strings.Contains(stmt.RawSQL, "REDACTED") && !strings.Contains(stmt.RawSQL, "redacted") {
		t.Fatalf("raw_sql=%q lacks the fixed-shape redacted rendering", stmt.RawSQL)
	}
	if u := result.Unsupported[0]; u.SQL == t26GoldenServer || u.SQL == "" {
		t.Fatalf("unsupported[].sql=%q — verbatim leak or lost identity", u.SQL)
	}
}

func TestT26R3_MixedBatch(t *testing.T) {
	sql := "CREATE TABLE t (id INT); " + t26GoldenServer + "; DROP TABLE t"
	result, _ := t26Audit(t, spec.DialectMySQL, sql)
	if len(result.Statements) != 3 {
		t.Fatalf("statements=%d want 3 (siblings audited, server bounded)", len(result.Statements))
	}
	if len(result.Unsupported) != 1 || result.Unsupported[0].Index != 1 {
		t.Fatalf("unsupported=%+v want one entry at index 1", result.Unsupported)
	}
	raw, _ := json.Marshal(result)
	if strings.Contains(string(raw), "golden-secret") {
		t.Fatalf("golden-secret leaked in mixed batch")
	}
	// Siblings keep verbatim raw_sql — no over-redaction of unmarked input.
	if result.Statements[0].RawSQL == "" || strings.Contains(result.Statements[0].RawSQL, "REDACTED") {
		t.Fatalf("sibling raw_sql=%q over-redacted", result.Statements[0].RawSQL)
	}
	// The sensitive statement keeps its position/index.
	if result.Statements[1].Index != 1 {
		t.Fatalf("sensitive statement index=%d want 1", result.Statements[1].Index)
	}
}
