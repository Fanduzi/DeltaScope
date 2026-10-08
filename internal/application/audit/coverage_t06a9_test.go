// Package audit verifies the T06-A9 explicit audit-time-column proof: the
// audit_columns rule recognizes the created/updated roles from extracted
// facts (time type + current-timestamp default + ON UPDATE), never from
// column names, and emits exactly one blocker per missing role — including
// the two-finding missing-both shape.
// input: frozen CREATE TABLE roles through AuditSQL plus the enrichment seam
// output: exact per-role findings, counter-control rejections, and preserved flags in derived state
// pos: issue #85 T06-A9 audit-layer contract tests
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"reflect"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

const t06A9AuditColumnsRule = "ddl.table.audit_columns.require"

const (
	t06A9CreatedMessage = "table should include a created-time audit column with DEFAULT CURRENT_TIMESTAMP"
	t06A9UpdatedMessage = "table should include an updated-time audit column with DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP"
)

// t06A9Policy returns the isolated profile: only the audit-columns rule at
// blocker with required=true, every other catalog rule disabled.
func t06A9Policy(t *testing.T) string {
	t.Helper()
	return t05PolicyPath(t, map[string]string{
		t06A9AuditColumnsRule: "      required: true\n",
	})
}

// t06A9Audit runs one AuditSQL request under the isolated profile.
func t06A9Audit(t *testing.T, dialect spec.Dialect, sql string) report.Result {
	t.Helper()
	return t06A9AuditWith(t, dialect, sql, t06A9Policy(t))
}

func t06A9AuditWith(t *testing.T, dialect spec.Dialect, sql, policy string) report.Result {
	t.Helper()
	result, err := AuditSQL(context.Background(), Request{
		SQL: sql, Dialect: dialect, Schema: "golden", ConfigPath: policy,
	})
	if err != nil {
		t.Fatalf("%s: unexpected error %v", dialect, err)
	}
	return result
}

// t06A9WantPass pins a complete, clean pass on a single statement.
func t06A9WantPass(t *testing.T, result report.Result) {
	t.Helper()
	if result.Verdict != report.VerdictPass || result.Coverage.Status != report.CoverageComplete {
		t.Fatalf("aggregate = %s/%s, want pass/complete", result.Verdict, result.Coverage.Status)
	}
	if len(result.Unsupported) != 0 || len(result.Statements) != 1 {
		t.Fatalf("unsupported = %#v statements = %d, want none/1", result.Unsupported, len(result.Statements))
	}
	statement := result.Statements[0]
	if statement.Index != 0 || statement.Kind != "ddl" ||
		statement.Coverage.Status != report.CoverageComplete || len(statement.Findings) != 0 {
		t.Fatalf("statement = %+v, want idx0/ddl/complete/0 findings", statement)
	}
}

// t06A9WantFindings pins the complete finding multiset on statement 0:
// (rule_id, level, message, metadata map, location) per entry, duplicates
// preserved by counting.
func t06A9WantFindings(t *testing.T, result report.Result, want []map[string]any) {
	t.Helper()
	if result.Verdict != report.VerdictReject || result.Coverage.Status != report.CoverageComplete {
		t.Fatalf("aggregate = %s/%s, want reject/complete", result.Verdict, result.Coverage.Status)
	}
	if len(result.Unsupported) != 0 || len(result.Statements) != 1 {
		t.Fatalf("unsupported = %#v statements = %d, want none/1", result.Unsupported, len(result.Statements))
	}
	statement := result.Statements[0]
	if statement.Index != 0 || statement.Kind != "ddl" {
		t.Fatalf("statement identity = index %d kind %s, want 0/ddl", statement.Index, statement.Kind)
	}
	if result.Summary.Blockers != len(want) || result.Summary.Warnings != 0 {
		t.Fatalf("summary = %+v, want %d blockers/0 warnings", result.Summary, len(want))
	}
	if len(statement.Findings) != len(want) {
		t.Fatalf("findings = %#v, want exactly %d", statement.Findings, len(want))
	}
	consumed := make([]bool, len(statement.Findings))
	for _, w := range want {
		found := false
		for i, f := range statement.Findings {
			if consumed[i] {
				continue
			}
			line, column := 0, 0
			if f.Location != nil {
				line, column = f.Location.Line, f.Location.Column
			}
			if f.RuleID == w["rule_id"] && string(f.Level) == w["level"] &&
				f.Message == w["message"] &&
				reflect.DeepEqual(f.Metadata, w["metadata"]) &&
				line == w["line"] && column == w["column"] {
				consumed[i] = true
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("no finding entry matches %#v; actual = %#v", w, statement.Findings)
		}
	}
}

func t06A9Finding(kind, message string) map[string]any {
	return map[string]any{
		"rule_id": t06A9AuditColumnsRule, "level": "blocker", "message": message,
		"metadata": map[string]any{"table": "t", "kind": kind},
		"line":     1, "column": 1,
	}
}

// TestT06A9RoleAttribution is group 2: real AuditSQL runs of the frozen
// A–F roles with per-role finding identity, plus the fact-not-name and
// updated-does-not-cover-created controls.
func TestT06A9RoleAttribution(t *testing.T) {
	roleSQL := map[string]string{
		"A": "CREATE TABLE t (id INT PRIMARY KEY, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP);",
		"B": "CREATE TABLE t (id INT PRIMARY KEY, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP);",
		"C": "CREATE TABLE t (id INT PRIMARY KEY, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP);",
		"D": "CREATE TABLE t (id INT PRIMARY KEY);",
		"E": "CREATE TABLE t (id INT PRIMARY KEY, created_at DATETIME NOT NULL DEFAULT NOW(), updated_at DATETIME NOT NULL DEFAULT NOW() ON UPDATE NOW());",
	}
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect)+"/A-complete-pair", func(t *testing.T) {
			t06A9WantPass(t, t06A9Audit(t, dialect, roleSQL["A"]))
		})
		t.Run(string(dialect)+"/E-now-spelling", func(t *testing.T) {
			t06A9WantPass(t, t06A9Audit(t, dialect, roleSQL["E"]))
		})
		t.Run(string(dialect)+"/B-missing-created", func(t *testing.T) {
			t06A9WantFindings(t, t06A9Audit(t, dialect, roleSQL["B"]),
				[]map[string]any{t06A9Finding("created", t06A9CreatedMessage)})
		})
		t.Run(string(dialect)+"/C-missing-updated", func(t *testing.T) {
			t06A9WantFindings(t, t06A9Audit(t, dialect, roleSQL["C"]),
				[]map[string]any{t06A9Finding("updated", t06A9UpdatedMessage)})
		})
		t.Run(string(dialect)+"/D-missing-both", func(t *testing.T) {
			t06A9WantFindings(t, t06A9Audit(t, dialect, roleSQL["D"]),
				[]map[string]any{
					t06A9Finding("created", t06A9CreatedMessage),
					t06A9Finding("updated", t06A9UpdatedMessage),
				})
		})
		t.Run(string(dialect)+"/updated_never_covers_created", func(t *testing.T) {
			// Two ON UPDATE columns still leave the created role missing.
			t06A9WantFindings(t, t06A9Audit(t, dialect,
				"CREATE TABLE t (id INT PRIMARY KEY, a DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP, b DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP);"),
				[]map[string]any{t06A9Finding("created", t06A9CreatedMessage)})
		})
		t.Run(string(dialect)+"/roles_follow_facts_not_names", func(t *testing.T) {
			// Arbitrary names carrying the same facts satisfy the same roles.
			t06A9WantPass(t, t06A9Audit(t, dialect,
				"CREATE TABLE t (id INT PRIMARY KEY, made_on DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, touched_on DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP);"))
		})
		t.Run(string(dialect)+"/canonical_names_not_forced", func(t *testing.T) {
			// created_at/updated_at names without the required facts still
			// trigger both roles.
			t06A9WantFindings(t, t06A9Audit(t, dialect,
				"CREATE TABLE t (id INT PRIMARY KEY, created_at INT, updated_at INT);"),
				[]map[string]any{
					t06A9Finding("created", t06A9CreatedMessage),
					t06A9Finding("updated", t06A9UpdatedMessage),
				})
		})
		t.Run(string(dialect)+"/multi_statement_attribution", func(t *testing.T) {
			result := t06A9Audit(t, dialect,
				"CREATE TABLE a (id INT PRIMARY KEY, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP);\nCREATE TABLE b (id INT PRIMARY KEY);")
			if len(result.Statements) != 2 {
				t.Fatalf("statements = %d, want 2", len(result.Statements))
			}
			if len(result.Statements[0].Findings) != 0 {
				t.Fatalf("statement 0 findings = %#v, want none", result.Statements[0].Findings)
			}
			second := result.Statements[1]
			if len(second.Findings) != 2 {
				t.Fatalf("statement 1 findings = %#v, want created+updated", second.Findings)
			}
			kinds := map[string]bool{}
			for _, f := range second.Findings {
				if f.RuleID != t06A9AuditColumnsRule || f.Location.Line != 2 ||
					f.Metadata["table"] != "b" {
					t.Fatalf("finding = %#v, want audit rule on statement 1 line 2 table b", f)
				}
				kinds[f.Metadata["kind"].(string)] = true
			}
			if !kinds["created"] || !kinds["updated"] {
				t.Fatalf("kinds = %v, want both roles", kinds)
			}
		})
	}
}

// TestT06A9EntryCounterControls is group 3: the role gate's inverse — facts
// that must NOT satisfy a role. Every input is verified to parse on the real
// parser (probe-proven); none of these shapes claim native legality on the
// four anchors.
func TestT06A9EntryCounterControls(t *testing.T) {
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect)+"/on_update_without_default", func(t *testing.T) {
			// OnUpdate alone satisfies neither role: the created branch needs
			// the current-timestamp DEFAULT, the updated branch needs both.
			t06A9WantFindings(t, t06A9Audit(t, dialect,
				"CREATE TABLE t (id INT PRIMARY KEY, c DATETIME NOT NULL ON UPDATE CURRENT_TIMESTAMP);"),
				[]map[string]any{
					t06A9Finding("created", t06A9CreatedMessage),
					t06A9Finding("updated", t06A9UpdatedMessage),
				})
		})
		t.Run(string(dialect)+"/literal_default_not_current_timestamp", func(t *testing.T) {
			t06A9WantFindings(t, t06A9Audit(t, dialect,
				"CREATE TABLE t (id INT PRIMARY KEY, c DATETIME NOT NULL DEFAULT '2020-01-01 00:00:00');"),
				[]map[string]any{
					t06A9Finding("created", t06A9CreatedMessage),
					t06A9Finding("updated", t06A9UpdatedMessage),
				})
		})
		t.Run(string(dialect)+"/non_time_type_flag_does_not_count", func(t *testing.T) {
			// VARCHAR accepts a current-timestamp expression in this grammar;
			// isTimeLike excludes it from both roles.
			t06A9WantFindings(t, t06A9Audit(t, dialect,
				"CREATE TABLE t (id INT PRIMARY KEY, c VARCHAR(20) DEFAULT CURRENT_TIMESTAMP);"),
				[]map[string]any{
					t06A9Finding("created", t06A9CreatedMessage),
					t06A9Finding("updated", t06A9UpdatedMessage),
				})
		})
	}
}

// TestT06A9ParamsAndDefaults is group 4: required=false and disabled stay
// silent, a non-bool required param is a configuration error, the shipped
// default level stays warning, and explicit blocker override applies.
func TestT06A9ParamsAndDefaults(t *testing.T) {
	missingBoth := "CREATE TABLE t (id INT PRIMARY KEY);"
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect)+"/required_false_silent", func(t *testing.T) {
			policy := t05PolicyPath(t, map[string]string{
				t06A9AuditColumnsRule: "      required: false\n",
			})
			t06A9WantPass(t, t06A9AuditWith(t, dialect, missingBoth, policy))
		})
		t.Run(string(dialect)+"/disabled_silent", func(t *testing.T) {
			t06A9WantPass(t, t06A9AuditWith(t, dialect, missingBoth,
				t05PolicyPath(t, map[string]string{})))
		})
		t.Run(string(dialect)+"/non_bool_required_errors", func(t *testing.T) {
			policy := t05PolicyPath(t, map[string]string{
				t06A9AuditColumnsRule: "      required: \"yes\"\n",
			})
			if _, err := AuditSQL(context.Background(), Request{
				SQL: missingBoth, Dialect: dialect, Schema: "golden", ConfigPath: policy,
			}); err == nil {
				t.Fatalf("non-bool required accepted, want constructor error")
			}
		})
		t.Run(string(dialect)+"/default_level_warning", func(t *testing.T) {
			// Enable without an explicit level: the shipped default warning
			// holds, so the aggregate is review — not a blocker reject.
			policy := t05PolicyPathConfigured(t, map[string]t05RuleConfig{
				t06A9AuditColumnsRule: {level: "warning", params: "      required: true\n"},
			})
			result := t06A9AuditWith(t, dialect, missingBoth, policy)
			if result.Verdict != report.VerdictReview || result.Summary.Warnings != 2 || result.Summary.Blockers != 0 {
				t.Fatalf("aggregate = %s summary=%+v, want review/2 warnings/0 blockers",
					result.Verdict, result.Summary)
			}
			for _, f := range result.Statements[0].Findings {
				if f.Level != "warning" {
					t.Fatalf("level = %s, want warning", f.Level)
				}
			}
		})
	}
}

// TestT06A9DerivedStateConsumption is group 5: the CREATE-derived snapshot
// carries both audit-column facts into the next statement's pre-state — the
// post-ALTER column view keeps HasDefault/DCT/OCT per declared role. Provider
// reads stay single-shot and neither the source statement nor the provider
// object is mutated.
func TestT06A9DerivedStateConsumption(t *testing.T) {
	sql := "CREATE TABLE t (id INT PRIMARY KEY, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP);\nALTER TABLE t ADD COLUMN note INT;"
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect), func(t *testing.T) {
			enriched := enrichA3(t, sql, dialect, &t05AbsentProvider{})
			preAlter := enriched[1].Metadata.TargetTable
			if preAlter == nil || !preAlter.Exists {
				t.Fatalf("pre-alter state = %+v, want present derived table", preAlter)
			}
			want := map[string][3]bool{
				"created_at": {true, true, false},
				"updated_at": {true, true, true},
			}
			for name, w := range want {
				c := preAlter.FindColumn(name)
				if c == nil {
					t.Fatalf("derived column %q missing", name)
				}
				if got := [3]bool{c.HasDefault, c.DefaultIsCurrentTimestamp, c.OnUpdateCurrentTimestamp}; got != w {
					t.Fatalf("derived column %s flags = %v, want %v", name, got, w)
				}
			}
			// The extracted statement keeps its own facts — enrichment does
			// not rewrite them while building the snapshot.
			ddl := enriched[0].DDL
			created := ddl.Columns[1]
			updated := ddl.Columns[2]
			if !created.DefaultIsCurrentTimestamp || created.OnUpdateCurrentTimestamp ||
				!updated.DefaultIsCurrentTimestamp || !updated.OnUpdateCurrentTimestamp {
				t.Fatalf("extracted facts mutated: created=%+v updated=%+v", created, updated)
			}
		})
	}
}
