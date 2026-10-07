// Package ddl verifies the T06-A7 table-collation allowlist contract at the
// rule-method layer: constructor params, dialect and statement-shape gating,
// and the default-disabled registration entry.
// input: synthetic CREATE TABLE statements plus rule policies for the new table collation allowlist
// output: exact AppliesTo/Evaluate behavior for allowed, denied, missing, disabled, PG, and non-CREATE shapes
// pos: rule-layer contract tests for issue #85 T06-A7
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// t06a7CreateStatement builds one CREATE TABLE statement carrying the given
// table options and dialect.
func t06a7CreateStatement(dialect spec.Dialect, options map[string]string) spec.Statement {
	return spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: dialect,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationCreateTable,
			Table:     &spec.Table{Name: "t"},
			Columns:   []spec.Column{{Name: "c", Type: "int"}},
			Options:   options,
		},
	}
}

func t06a7TableCollationRule(t *testing.T, params map[string]any) rule.StatementRule {
	t.Helper()
	r, err := newTableOptionAllowlistRule(ruleIDTableCollationAllowlist, "collate", "collation",
		[]string{"utf8mb4_bin"}, rule.LevelBlocker, policy.RulePolicy{
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params:  params,
		})
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	return r
}

// TestT06A7RuleParamsContract pins the param surface: values list semantics
// (multi-value allow), require_explicit true/false, and the existing
// construction errors for malformed params.
func TestT06A7RuleParamsContract(t *testing.T) {
	t.Run("multi_value_allows", func(t *testing.T) {
		r := t06a7TableCollationRule(t, map[string]any{
			"values":           []any{"utf8mb4_bin", "utf8mb4_general_ci"},
			"require_explicit": false,
		})
		statement := t06a7CreateStatement(spec.DialectMySQL, map[string]string{"collate": "utf8mb4_general_ci"})
		findings, err := r.Evaluate(context.Background(), statement)
		if err != nil || len(findings) != 0 {
			t.Fatalf("evaluate = %v findings %#v, want clean", err, findings)
		}
	})
	t.Run("values_non_list_errors", func(t *testing.T) {
		_, err := newTableOptionAllowlistRule(ruleIDTableCollationAllowlist, "collate", "collation",
			[]string{"utf8mb4_bin"}, rule.LevelBlocker, policy.RulePolicy{
				Enabled: true,
				Level:   rule.LevelBlocker,
				Params:  map[string]any{"values": "not-a-list"},
			})
		if err == nil {
			t.Fatalf("expected construction error for non-list values")
		}
	})
	t.Run("require_explicit_non_bool_errors", func(t *testing.T) {
		_, err := newTableOptionAllowlistRule(ruleIDTableCollationAllowlist, "collate", "collation",
			[]string{"utf8mb4_bin"}, rule.LevelBlocker, policy.RulePolicy{
				Enabled: true,
				Level:   rule.LevelBlocker,
				Params:  map[string]any{"require_explicit": "yes"},
			})
		if err == nil {
			t.Fatalf("expected construction error for non-bool require_explicit")
		}
	})
}

// TestT06A7RuleAppliesTo pins the gate matrix: only MySQL/TiDB CREATE TABLE;
// PostgreSQL, schema ops, and ALTER shapes never reach Evaluate even with the
// rule explicitly enabled.
func TestT06A7RuleAppliesTo(t *testing.T) {
	r := t06a7TableCollationRule(t, map[string]any{"require_explicit": true})
	pgCreate := t06a7CreateStatement(spec.DialectPostgreSQL, map[string]string{"collate": "x"})
	if r.AppliesTo(pgCreate) {
		t.Fatalf("rule applies to postgresql create table")
	}
	alter := t06a7CreateStatement(spec.DialectMySQL, map[string]string{"collate": "utf8mb4_general_ci"})
	alter.DDL.Operation = spec.DDLOperationAlterTable
	if r.AppliesTo(alter) {
		t.Fatalf("rule applies to alter table")
	}
	createSchema := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectMySQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationCreateSchema,
			Table:     &spec.Table{Name: "d1"},
			Options:   map[string]string{"collate": "utf8mb4_general_ci"},
		},
	}
	if r.AppliesTo(createSchema) {
		t.Fatalf("rule applies to create schema")
	}
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		if !r.AppliesTo(t06a7CreateStatement(dialect, map[string]string{})) {
			t.Fatalf("rule does not apply to %s create table", dialect)
		}
	}
}

// TestT06A7RuleFindingContract pins the exact finding shape on denied and
// missing-declaration inputs.
func TestT06A7RuleFindingContract(t *testing.T) {
	r := t06a7TableCollationRule(t, map[string]any{"require_explicit": true})
	for _, tc := range []struct {
		name    string
		options map[string]string
		actual  string
	}{
		{"denied", map[string]string{"collate": "utf8mb4_general_ci"}, "utf8mb4_general_ci"},
		{"undeclared", map[string]string{}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			findings, err := r.Evaluate(context.Background(), t06a7CreateStatement(spec.DialectMySQL, tc.options))
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			if len(findings) != 1 {
				t.Fatalf("findings = %#v, want exactly 1", findings)
			}
			f := findings[0]
			if f.Level != rule.LevelBlocker {
				t.Fatalf("level = %s, want blocker", f.Level)
			}
			if f.Message != "table collation must be one of [utf8mb4_bin]" {
				t.Fatalf("message = %q", f.Message)
			}
			if f.Metadata["table"] != "t" || f.Metadata["option"] != "collate" ||
				f.Metadata["actual"] != tc.actual {
				t.Fatalf("metadata = %#v", f.Metadata)
			}
			allowed, ok := f.Metadata["allowed"].([]string)
			if !ok || len(allowed) != 1 || allowed[0] != "utf8mb4_bin" {
				t.Fatalf("allowed = %#v", f.Metadata["allowed"])
			}
		})
	}
}

// TestT06A7RuleDefaultDisabled pins the shipped registration choice: the new
// rule exists in the default policy but stays disabled until a profile opts
// in — existing behavior is unchanged without explicit enablement.
func TestT06A7RuleDefaultDisabled(t *testing.T) {
	entry, ok := policy.Default().Rules[ruleIDTableCollationAllowlist]
	if !ok {
		t.Fatalf("rule %s missing from default policy", ruleIDTableCollationAllowlist)
	}
	if entry.Enabled {
		t.Fatalf("rule %s shipped enabled, want disabled", ruleIDTableCollationAllowlist)
	}
	if entry.Level != rule.LevelBlocker {
		t.Fatalf("default level = %s, want blocker", entry.Level)
	}
	values, ok := entry.Params["values"].([]string)
	if !ok || len(values) != 1 || values[0] != "utf8mb4_bin" {
		t.Fatalf("default values = %#v, want [utf8mb4_bin]", entry.Params["values"])
	}
	if req, ok := entry.Params["require_explicit"].(bool); !ok || !req {
		t.Fatalf("default require_explicit = %#v, want true", entry.Params["require_explicit"])
	}
}
