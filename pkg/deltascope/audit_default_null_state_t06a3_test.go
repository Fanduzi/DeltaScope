// Package deltascope verifies the T06-A3 typed DEFAULT NULL contract at the public SDK seam.
// input: MySQL and TiDB CREATE+DROP+INDEX batches and CREATE default-presence audits with isolated policies
// output: the three-statement path passes with a known initial state; the default-presence reject stays a policy finding
// pos: public SDK contract test for the frozen T06-A3 oracle (issue #85)
// note: if this file changes, update this header and module README.md.
package deltascope

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule/catalog"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

const t06A3NullDropSQL = "CREATE TABLE t (id INT PRIMARY KEY, obsolete INT, keep_c VARCHAR(8) DEFAULT NULL);\n" +
	"ALTER TABLE t DROP COLUMN obsolete;\n" +
	"CREATE INDEX idx_keep ON t(keep_c);"

// t06A3AbsentProvider reports a known-absent golden.t, mirroring the frozen
// shared-path fixture for the three-statement drop/index flow.
type t06A3AbsentProvider struct{}

func (t06A3AbsentProvider) LoadInstanceFacts(context.Context, Dialect, string) (*InstanceFacts, error) {
	return &InstanceFacts{}, nil
}

func (t06A3AbsentProvider) LoadTableSnapshot(_ context.Context, _ Dialect, schema, table string) (*TableSnapshot, error) {
	return &spec.TableSnapshot{Exists: false, Table: &spec.Table{Schema: schema, Name: table}}, nil
}

func t06A3SDKPolicy(t *testing.T, enabled map[string]bool) string {
	t.Helper()
	required := map[string]bool{
		"ddl.create_index.columns.exists.require": true,
		"ddl.column.default.require":              true,
	}
	var text strings.Builder
	text.WriteString("rules:\n")
	for _, entry := range catalog.All() {
		if enabled[entry.RuleID] {
			fmt.Fprintf(&text, "  %q:\n    enabled: true\n    level: blocker\n", entry.RuleID)
			if required[entry.RuleID] {
				text.WriteString("    params:\n      required: true\n")
			}
		} else {
			fmt.Fprintf(&text, "  %q:\n    enabled: false\n", entry.RuleID)
		}
	}
	path := filepath.Join(t.TempDir(), "policy.yaml")
	if err := os.WriteFile(path, []byte(text.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestT06A3NullDropStateSDK(t *testing.T) {
	policy := t06A3SDKPolicy(t, map[string]bool{
		"ddl.table.exists.create.forbid":          true,
		"ddl.table.exists.alter.require":          true,
		"ddl.alter.drop_column.exists.require":    true,
		"ddl.create_index.columns.exists.require": true,
	})
	for _, dialect := range []Dialect{DialectMySQL, DialectTiDB} {
		t.Run(string(dialect), func(t *testing.T) {
			result, err := Audit(context.Background(), Request{
				SQL:              t06A3NullDropSQL,
				Dialect:          dialect,
				Schema:           "golden",
				ConfigPath:       policy,
				MetadataProvider: t06A3AbsentProvider{},
			})
			if err != nil {
				t.Fatalf("audit: %v", err)
			}
			if result.Verdict != VerdictPass || result.Coverage.Status != "complete" {
				t.Fatalf("aggregate = %s/%s, want pass/complete", result.Verdict, result.Coverage.Status)
			}
			if len(result.Statements) != 3 {
				t.Fatalf("statements = %d, want 3", len(result.Statements))
			}
			for i, statement := range result.Statements {
				if len(statement.Findings) != 0 || len(statement.EvidenceGaps) != 0 {
					t.Fatalf("statement %d findings=%v gaps=%v, want none", i, statement.Findings, statement.EvidenceGaps)
				}
			}
		})
	}
}

func TestT06A3DefaultRequireSDK(t *testing.T) {
	policy := t06A3SDKPolicy(t, map[string]bool{"ddl.column.default.require": true})
	for _, dialect := range []Dialect{DialectMySQL, DialectTiDB} {
		t.Run(string(dialect), func(t *testing.T) {
			reject, err := Audit(context.Background(), Request{
				SQL: "CREATE TABLE t (c VARCHAR(8));", Dialect: dialect, Schema: "golden", ConfigPath: policy,
			})
			if err != nil {
				t.Fatalf("audit: %v", err)
			}
			if reject.Verdict != VerdictReject || len(reject.Statements[0].Findings) != 1 ||
				reject.Statements[0].Findings[0].RuleID != "ddl.column.default.require" {
				t.Fatalf("absent DEFAULT = %s findings=%v, want one default-require blocker", reject.Verdict, reject.Statements[0].Findings)
			}
			pass, err := Audit(context.Background(), Request{
				SQL: "CREATE TABLE t (c VARCHAR(8) DEFAULT NULL);", Dialect: dialect, Schema: "golden", ConfigPath: policy,
			})
			if err != nil {
				t.Fatalf("audit: %v", err)
			}
			if pass.Verdict != VerdictPass || len(pass.Statements[0].Findings) != 0 {
				t.Fatalf("DEFAULT NULL = %s findings=%v, want pass", pass.Verdict, pass.Statements[0].Findings)
			}
		})
	}
}
