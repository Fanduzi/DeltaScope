// Package deltascope adds no-leak regression coverage for the MySQL/TiDB
// builtin-effect identity feasibility probe boundary. The live Docker probes
// live in internal/infrastructure/metadata/mysql/builtin_effect_identity_live_probes_test.go
// (build tag: integration) and do NOT introduce a new public SDK path. This
// file therefore tests the UNCHANGED public SDK surface (AnalyzeQueryAccess)
// under MySQL/TiDB function-bearing SQL with injected markers, and asserts
// those markers, identity facts, candidates, session/context data, manifest
// data, raw SQL, and `severity` are absent from the public result and its JSON
// mapping.
//
// input: MySQL/TiDB function-bearing SQL with injected function-name and
//
//	literal markers on the SDK normal path
//
// output: public SDK result + JSON marshal contain none of the injected
//
//	markers, no identity/candidate/manifest/session fields, no raw SQL, no
//	`severity` field; DSN/credential/driver-error injection is covered by the
//
// # HTTP error-boundary test
//
// pos: public SDK no-leak regression for the MySQL/TiDB probe boundary;
//
//	no new public path is introduced or exercised here
//
// note: if this file changes, update this header, the module README.md, and
//
//	the decision record's no-leak evidence section. See
//	docs/decisions/2026-07-17-query-access-mysql-tidb-effect-identity-feasibility.md
package deltascope

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// mysqlTiDBProbeNoLeakMarkers is the canonical marker set for the MySQL/TiDB
// builtin-identity probe boundary. Every marker must be absent from the public
// SDK result and its JSON mapping. Markers are chosen to cover:
//   - injected function name (my_secret_udf)
//   - injected literal (SECRET_LITERAL)
//   - DSN/credential-shaped strings (P@ssw0rd, postgres://user:pass@host, mysql://root:root@host)
//   - driver-error-like text (driver: bad conn)
//   - identity/candidate/manifest/session field names that must not exist
//   - `severity` (must never appear on query access results)
var mysqlTiDBProbeNoLeakMarkers = []string{
	"my_secret_udf",
	"SECRET_LITERAL",
	"P@ssw0rd",
	"postgres://user:pass@host",
	"mysql://root:root@host:3406",
	"driver: bad conn",
	"severity",
	"identity",
	"candidate",
	"manifest",
	"session",
	"context",
	"dsn",
}

// TestAnalyzeQueryAccess_MySQLTiDBProbeBoundary_NoLeak verifies the unchanged
// public SDK surface (AnalyzeQueryAccess) does not leak injected markers,
// identity facts, candidates, session/context, manifest, raw SQL, or severity
// when analyzing MySQL/TiDB function-bearing SQL that references marker
// function names and literals.
//
// This test covers the SDK NORMAL path (successful analysis). It injects
// function-name and literal markers via SQL. DSN/credential/driver-error
// markers are NOT injected through the SDK normal path because
// AnalyzeQueryAccess does not expose a live-connection error path; those
// markers are covered by the HTTP error-boundary test
// (TestHandlerQueryAccess_MySQLTiDBProbeBoundary_DriverErrorNoLeak). The
// marker scan still includes all markers as a defense-in-depth check that no
// hardcoded DSN/credential/driver text appears in the SDK result.
func TestAnalyzeQueryAccess_MySQLTiDBProbeBoundary_NoLeak(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		sql     string
		dialect Dialect
	}{
		{
			name:    "mysql_udf_marker_function",
			sql:     "SELECT my_secret_udf(id) FROM users",
			dialect: DialectMySQL,
		},
		{
			name:    "tidb_udf_marker_function",
			sql:     "SELECT my_secret_udf(id) FROM users",
			dialect: DialectTiDB,
		},
		{
			name:    "mysql_count_with_marker_literal",
			sql:     "SELECT COUNT(SECRET_LITERAL) FROM users",
			dialect: DialectMySQL,
		},
		{
			name:    "tidb_count_with_marker_literal",
			sql:     "SELECT COUNT(SECRET_LITERAL) FROM users",
			dialect: DialectTiDB,
		},
		{
			name:    "mysql_qualified_marker_function",
			sql:     "SELECT app.my_secret_udf(id) FROM users",
			dialect: DialectMySQL,
		},
		{
			name:    "tidb_qualified_marker_function",
			sql:     "SELECT app.my_secret_udf(id) FROM users",
			dialect: DialectTiDB,
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

			// The result must remain fail-closed for function-bearing SQL.
			if result.ReadClassification != QueryAccessIndeterminate {
				t.Errorf("%s: expected indeterminate classification, got %q", tc.name, result.ReadClassification)
			}
			if result.Admission != QueryAccessIndeterminateAdmission {
				t.Errorf("%s: expected indeterminate admission, got %q", tc.name, result.Admission)
			}

			// Assert no-leak on the Go struct dump AND the JSON marshal.
			dump := stringifyResult(result)
			data, err := json.Marshal(result)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			jsonDump := string(data)

			for _, marker := range mysqlTiDBProbeNoLeakMarkers {
				if strings.Contains(dump, marker) {
					t.Errorf("%s: SDK struct dump leaked marker %q: %s", tc.name, marker, dump)
				}
				if strings.Contains(jsonDump, marker) {
					t.Errorf("%s: SDK JSON leaked marker %q: %s", tc.name, marker, jsonDump)
				}
			}

			// The raw SQL must not appear in the structured result dump.
			if strings.Contains(dump, tc.sql) {
				t.Errorf("%s: SDK struct dump must not embed raw SQL: %s", tc.name, dump)
			}
			if strings.Contains(jsonDump, tc.sql) {
				t.Errorf("%s: SDK JSON must not embed raw SQL: %s", tc.name, jsonDump)
			}

			// Public JSON must not carry identity/candidate/manifest/session/
			// severity/context fields. Verify by unmarshalling into a map and
			// checking top-level keys.
			var raw map[string]any
			if err := json.Unmarshal(data, &raw); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			forbiddenTopLevel := []string{
				"severity", "identity", "candidate", "manifest", "session",
				"context", "dsn", "raw_sql", "driver_error",
			}
			for _, field := range forbiddenTopLevel {
				if _, ok := raw[field]; ok {
					t.Errorf("%s: SDK JSON must not carry top-level field %q: %s", tc.name, field, jsonDump)
				}
			}
		})
	}
}

// stringifyResult produces a deterministic string for substring scanning. It
// must cover all exported fields of QueryAccessResult.
func stringifyResult(r *QueryAccessResult) string {
	if r == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(r.Dialect)
	b.WriteString(string(r.Mode))
	b.WriteString(string(r.ReadClassification))
	b.WriteString(string(r.Admission))
	for _, rc := range r.ReasonCodes {
		b.WriteString(rc)
	}
	for _, rel := range r.Relations {
		b.WriteString(rel.Schema)
		b.WriteString(rel.Name)
		b.WriteString(rel.Alias)
		b.WriteString(rel.Kind)
	}
	for _, col := range r.ReferencedColumns {
		b.WriteString(col.Schema)
		b.WriteString(col.Table)
		b.WriteString(col.Column)
		for _, u := range col.Usages {
			b.WriteString(u)
		}
	}
	for _, out := range r.Outputs {
		b.WriteString(out.Name)
		for _, s := range out.Sources {
			b.WriteString(s)
		}
	}
	for _, req := range r.Requirements {
		b.WriteString(req.Object)
		b.WriteString(req.Privilege)
	}
	for _, u := range r.Unresolved {
		b.WriteString(u.Reference)
		b.WriteString(u.Reason)
	}
	for _, w := range r.Warnings {
		b.WriteString(w)
	}
	return b.String()
}

// TestAnalyzeQueryAccess_MySQLTiDBProbeBoundary_FunctionFreeAdmissibleNoLeak
// verifies the unchanged public SDK surface keeps function-free admissible
// MySQL/TiDB SELECTs leak-free under marker injection. This is the
// non-regression baseline: the probe boundary must not widen or narrow the
// existing admissible set, and markers injected via literals must not leak.
func TestAnalyzeQueryAccess_MySQLTiDBProbeBoundary_FunctionFreeAdmissibleNoLeak(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		sql     string
		dialect Dialect
	}{
		{
			name:    "mysql_where_marker_literal",
			sql:     "SELECT id FROM users WHERE name = 'SECRET_LITERAL'",
			dialect: DialectMySQL,
		},
		{
			name:    "tidb_where_marker_literal",
			sql:     "SELECT id FROM users WHERE name = 'SECRET_LITERAL'",
			dialect: DialectTiDB,
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

			if result.ReadClassification != QueryAccessReadOnly {
				t.Errorf("%s: expected read_only, got %q", tc.name, result.ReadClassification)
			}
			if result.Admission != QueryAccessAdmissible {
				t.Errorf("%s: expected admissible, got %q", tc.name, result.Admission)
			}

			dump := stringifyResult(result)
			data, err := json.Marshal(result)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			jsonDump := string(data)

			for _, marker := range mysqlTiDBProbeNoLeakMarkers {
				if strings.Contains(dump, marker) {
					t.Errorf("%s: SDK struct dump leaked marker %q: %s", tc.name, marker, dump)
				}
				if strings.Contains(jsonDump, marker) {
					t.Errorf("%s: SDK JSON leaked marker %q: %s", tc.name, marker, jsonDump)
				}
			}
		})
	}
}
