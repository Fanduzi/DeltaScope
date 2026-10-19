// Package audit verifies the frozen T07 advanced-column contract: generated
// columns, expression defaults, and column visibility on the ordered
// MySQL/TiDB path.
// input: real-parser AuditSQL runs over the frozen role SQL with isolated
// policies that enable only the new advanced-column rules
// output: exact finding/message/metadata/location, coverage, verdict, and
// statement-identity assertions matching the frozen T07 oracle
// pos: T07 shared-layer acceptance seam beside the T05/T06 batch-state suites
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	domainpolicy "github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

const (
	t07RuleDependency = "ddl.alter.column.dependency.require"
	t07RuleCompat     = "ddl.column.advanced.compatibility.require"
)

// t07PolicyPath writes an isolated policy: every default rule disabled except
// the listed ones, which are enabled at blocker with the given params text.
func t07PolicyPath(t *testing.T, enabled map[string]string) string {
	t.Helper()
	var builder strings.Builder
	builder.WriteString("rules:\n")
	ruleIDs := make([]string, 0, len(domainpolicy.Default().Rules))
	for id := range domainpolicy.Default().Rules {
		ruleIDs = append(ruleIDs, id)
	}
	sort.Strings(ruleIDs)
	for _, id := range ruleIDs {
		if _, keep := enabled[id]; keep {
			continue
		}
		fmt.Fprintf(&builder, "  %s:\n    enabled: false\n", strconv.Quote(id))
	}
	enabledIDs := make([]string, 0, len(enabled))
	for id := range enabled {
		enabledIDs = append(enabledIDs, id)
	}
	sort.Strings(enabledIDs)
	for _, id := range enabledIDs {
		if _, known := domainpolicy.Default().Rules[id]; !known {
			t.Fatalf("policy enables unregistered rule %q", id)
		}
		fmt.Fprintf(&builder, "  %s:\n    enabled: true\n    level: blocker\n", strconv.Quote(id))
		builder.WriteString("    params:\n")
		builder.WriteString(enabled[id])
	}
	path := filepath.Join(t.TempDir(), "policy.yaml")
	if err := os.WriteFile(path, []byte(builder.String()), 0o600); err != nil {
		t.Fatalf("write policy: %v", err)
	}
	return path
}

// t07FirstPathSQL is the frozen contract seed: a two-statement sequence whose
// second statement is blocked by the generated-column dependency.
var t07FirstPathSQL = []string{
	"CREATE TABLE t (\n  a INT,\n  b INT GENERATED ALWAYS AS (a + 1) STORED\n)",
	"ALTER TABLE t DROP COLUMN a",
}

func t07FirstPathJoined() string {
	return strings.Join(t07FirstPathSQL, ";\n") + ";"
}

// TestT07FirstPathRealParser is the contract's first-path acceptance seam:
// real parser, ordered-state pre-state projection, and one exact dependency
// blocker. MySQL 8.0.46 and TiDB 8.5.0 variants.
func TestT07FirstPathRealParser(t *testing.T) {
	t.Parallel()
	for _, variant := range []struct {
		dialect spec.Dialect
		version string
	}{
		{spec.DialectMySQL, "8.0.46"},
		{spec.DialectTiDB, "8.5.0"},
	} {
		variant := variant
		t.Run(string(variant.dialect)+"/"+variant.version, func(t *testing.T) {
			t.Parallel()
			result, err := AuditSQL(context.Background(), Request{
				SQL:           t07FirstPathJoined(),
				Dialect:       variant.dialect,
				Schema:        "golden",
				TargetVersion: variant.version,
				ConfigPath:    t07PolicyPath(t, map[string]string{t07RuleDependency: "      required: true\n"}),
			})
			if err != nil {
				t.Fatalf("audit: %v", err)
			}
			if len(result.Statements) != 2 {
				t.Fatalf("statements = %d, want 2", len(result.Statements))
			}
			for i, statement := range result.Statements {
				wantRaw := t07FirstPathSQL[i] + ";"
				if statement.Index != i || statement.RawSQL != wantRaw {
					t.Fatalf("statement %d identity = index %d raw %q, want index %d raw %q", i, statement.Index, statement.RawSQL, i, wantRaw)
				}
				if statement.Coverage.Status != report.CoverageComplete {
					t.Fatalf("statement %d coverage = %s, want complete", i, statement.Coverage.Status)
				}
			}
			if len(result.Statements[0].Findings) != 0 {
				t.Fatalf("statement 0 findings = %+v, want none", result.Statements[0].Findings)
			}
			findings := result.Statements[1].Findings
			if len(findings) != 1 {
				t.Fatalf("statement 1 findings = %+v, want exactly one dependency blocker", findings)
			}
			finding := findings[0]
			if finding.RuleID != t07RuleDependency || finding.Level != rule.LevelBlocker {
				t.Fatalf("finding rule/level = %s/%s, want %s/blocker", finding.RuleID, finding.Level, t07RuleDependency)
			}
			wantMessage := `cannot drop column "a": generated column "b" depends on it`
			if finding.Message != wantMessage {
				t.Fatalf("finding message = %q, want %q", finding.Message, wantMessage)
			}
			wantMetadata := map[string]any{
				"table":            "t",
				"column":           "a",
				"dependent_column": "b",
				"dependency_kind":  "generated",
				"dependency":       "direct",
				"action":           "drop_column",
			}
			if !reflect.DeepEqual(finding.Metadata, wantMetadata) {
				t.Fatalf("finding metadata = %#v, want %#v", finding.Metadata, wantMetadata)
			}
			if finding.StatementIndex != 1 || finding.Location == nil || finding.Location.Line != 5 || finding.Location.Column != 1 {
				t.Fatalf("finding location = index %d %+v, want index 1 line 5 column 1", finding.StatementIndex, finding.Location)
			}
			if result.Verdict != report.VerdictReject {
				t.Fatalf("verdict = %s, want reject", result.Verdict)
			}
		})
	}
}

// t07C03SQL is the frozen opposite control: dropping the unreferenced
// ordinary column is dependency-free, the generated fact survives the drop,
// and an index bound to the generated column keeps a precise post-state.
var t07C03SQL = []string{
	"CREATE TABLE t (\n  a INT,\n  b INT GENERATED ALWAYS AS (a + 1) STORED,\n  c INT\n)",
	"ALTER TABLE t DROP COLUMN c",
	"CREATE INDEX idx_b ON t (b)",
}

// TestT07C03OppositeControl proves the dependency rule does not over-fire:
// removing an ordinary leaf column carries no dependents, the generated
// column and its reference survive in the projected post-state (deep-cloned,
// so mutating a returned projection cannot corrupt the entry), and the
// follow-up CREATE INDEX binds to the surviving generated column.
func TestT07C03OppositeControl(t *testing.T) {
	t.Parallel()
	for _, variant := range []struct {
		dialect spec.Dialect
		version string
	}{
		{spec.DialectMySQL, "8.0.46"},
		{spec.DialectTiDB, "8.5.0"},
	} {
		variant := variant
		t.Run(string(variant.dialect)+"/"+variant.version, func(t *testing.T) {
			t.Parallel()
			provider := &t05AbsentProvider{}
			result, err := AuditSQL(context.Background(), Request{
				SQL:           strings.Join(t07C03SQL, ";\n") + ";",
				Dialect:       variant.dialect,
				Schema:        "golden",
				TargetVersion: variant.version,
				ConfigPath: t07PolicyPath(t, map[string]string{
					t07RuleDependency:                      "      required: true\n",
					"ddl.create_index.columns.exists.require": "      required: true\n",
				}),
				MetadataProvider: provider,
			})
			if err != nil {
				t.Fatalf("audit: %v", err)
			}
			if len(result.Statements) != 3 {
				t.Fatalf("statements = %d, want 3", len(result.Statements))
			}
			for i, statement := range result.Statements {
				wantRaw := t07C03SQL[i] + ";"
				if statement.Index != i || statement.RawSQL != wantRaw {
					t.Fatalf("statement %d identity = index %d raw %q, want index %d raw %q", i, statement.Index, statement.RawSQL, i, wantRaw)
				}
				if statement.Coverage.Status != report.CoverageComplete {
					t.Fatalf("statement %d coverage = %s, want complete", i, statement.Coverage.Status)
				}
				if len(statement.Findings) != 0 || len(statement.EvidenceGaps) != 0 {
					t.Fatalf("statement %d findings/gaps = %+v/%+v, want none", i, statement.Findings, statement.EvidenceGaps)
				}
			}
			if result.Verdict != report.VerdictPass {
				t.Fatalf("verdict = %s, want pass", result.Verdict)
			}
			if len(provider.calls) != 1 || provider.calls[0] != "golden.t" {
				t.Fatalf("provider calls = %#v, want exactly one read of golden.t", provider.calls)
			}
		})
	}
}

// TestT07C03PostStateColumns pins the derived post-state directly at the
// batchState seam: a,b,c with b's generated fact before the drop, a,b after,
// and a projection mutation that leaves the stored entry and the parser-owned
// CREATE columns untouched.
func TestT07C03PostStateColumns(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			parsed, err := Parse(ctx, strings.Join(t07C03SQL, ";\n")+";", dialect)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			statements, err := Extract(ctx, parsed)
			if err != nil {
				t.Fatalf("extract: %v", err)
			}
			if len(statements) != 3 {
				t.Fatalf("statements = %d, want 3", len(statements))
			}
			createColumns := append([]spec.Column(nil), statements[0].DDL.Columns...)
			version := &spec.VersionIdentity{Product: spec.VersionProductMySQL, Version: "8.0.46", Major: 8, Minor: 0, Patch: 46, ValidatedRange: true}
			if dialect == spec.DialectTiDB {
				version = &spec.VersionIdentity{Product: spec.VersionProductTiDB, Version: "8.5.0", Major: 8, Minor: 5, Patch: 0, ValidatedRange: true}
			}
			for i := range statements {
				statements[i].Metadata = &spec.Metadata{Version: version}
			}
			state := newBatchState(dialect, "golden", nil)
			if err := state.apply(ctx, statements[0]); err != nil {
				t.Fatalf("apply create: %v", err)
			}
			pre, err := state.preState(ctx, "golden", *statements[1].DDL.Table)
			if err != nil {
				t.Fatalf("pre-state: %v", err)
			}
			if err := t07CheckColumns(pre, []string{"a", "b", "c"}); err != nil {
				t.Fatalf("%s pre-state: %v", dialect, err)
			}
			if err := t07CheckGenerated(pre, "b", "a"); err != nil {
				t.Fatalf("%s pre-state generated fact: %v", dialect, err)
			}
			if err := state.apply(ctx, statements[1]); err != nil {
				t.Fatalf("apply drop: %v", err)
			}
			post, err := state.preState(ctx, "golden", *statements[2].DDL.Table)
			if err != nil {
				t.Fatalf("post-state: %v", err)
			}
			if err := t07CheckColumns(post, []string{"a", "b"}); err != nil {
				t.Fatalf("%s post-state: %v", dialect, err)
			}
			if err := t07CheckGenerated(post, "b", "a"); err != nil {
				t.Fatalf("%s post-state generated fact: %v", dialect, err)
			}
			// Deep-clone isolation: mutating the returned projection must not
			// reach the stored entry or the parser-owned declaration.
			post.Columns[1].Generated.Expression.References[0].Name = "mutated"
			post.Columns[1].Generated.Storage = spec.GeneratedStorageUnknown
			post.Columns[1].Name = "mutated"
			reread, err := state.preState(ctx, "golden", *statements[2].DDL.Table)
			if err != nil {
				t.Fatalf("post-state re-read: %v", err)
			}
			if err := t07CheckGenerated(reread, "b", "a"); err != nil {
				t.Fatalf("%s post-state re-read: %v", dialect, err)
			}
			if !reflect.DeepEqual(statements[0].DDL.Columns, createColumns) {
				t.Fatalf("%s mutating the projection changed parser-owned columns: %+v", dialect, statements[0].DDL.Columns)
			}
		})
	}
}

// t07CheckColumns pins the exact column set in declaration order.
func t07CheckColumns(snapshot *spec.TableSnapshot, names []string) error {
	if snapshot == nil || !snapshot.Exists {
		return fmt.Errorf("snapshot missing or absent: %+v", snapshot)
	}
	if len(snapshot.Columns) != len(names) {
		return fmt.Errorf("columns = %d (%+v), want %v", len(snapshot.Columns), snapshot.Columns, names)
	}
	for i, want := range names {
		if snapshot.Columns[i].Name != want {
			return fmt.Errorf("column[%d] = %q, want %q", i, snapshot.Columns[i].Name, want)
		}
	}
	return nil
}

// t07CheckGenerated pins the generated fact on one column: stored storage and
// a single local reference to the named source column.
func t07CheckGenerated(snapshot *spec.TableSnapshot, column, ref string) error {
	for i := range snapshot.Columns {
		got := &snapshot.Columns[i]
		if got.Name != column {
			continue
		}
		if got.Generated == nil {
			return fmt.Errorf("column %q has no generated fact", column)
		}
		if got.Generated.Storage != spec.GeneratedStorageStored {
			return fmt.Errorf("column %q storage = %q, want stored", column, got.Generated.Storage)
		}
		expr := got.Generated.Expression
		if !expr.Known {
			return fmt.Errorf("column %q expression facts are unknown", column)
		}
		if len(expr.References) != 1 || expr.References[0].Name != ref || expr.References[0].Qualified() {
			return fmt.Errorf("column %q references = %+v, want one local ref to %q", column, expr.References, ref)
		}
		if expr.Fingerprint == "" {
			return fmt.Errorf("column %q missing expression fingerprint", column)
		}
		return nil
	}
	return fmt.Errorf("column %q not found", column)
}

// TestT07UnrelatedDropPreservesDependency proves an unrelated column removal
// keeps the remaining generated dependency: a follow-up drop of the
// referenced column is still the exact dependency blocker.
func TestT07UnrelatedDropPreservesDependency(t *testing.T) {
	t.Parallel()
	sql := []string{
		"CREATE TABLE t (\n  a INT,\n  b INT GENERATED ALWAYS AS (a + 1) STORED,\n  c INT\n)",
		"ALTER TABLE t DROP COLUMN c",
		"ALTER TABLE t DROP COLUMN a",
	}
	for _, variant := range []struct {
		dialect spec.Dialect
		version string
	}{
		{spec.DialectMySQL, "8.0.46"},
		{spec.DialectTiDB, "8.5.0"},
	} {
		variant := variant
		t.Run(string(variant.dialect)+"/"+variant.version, func(t *testing.T) {
			t.Parallel()
			result, err := AuditSQL(context.Background(), Request{
				SQL:           strings.Join(sql, ";\n") + ";",
				Dialect:       variant.dialect,
				Schema:        "golden",
				TargetVersion: variant.version,
				ConfigPath:    t07PolicyPath(t, map[string]string{t07RuleDependency: "      required: true\n"}),
			})
			if err != nil {
				t.Fatalf("audit: %v", err)
			}
			if len(result.Statements) != 3 {
				t.Fatalf("statements = %d, want 3", len(result.Statements))
			}
			for i := 0; i < 2; i++ {
				if len(result.Statements[i].Findings) != 0 {
					t.Fatalf("statement %d findings = %+v, want none", i, result.Statements[i].Findings)
				}
			}
			findings := result.Statements[2].Findings
			if len(findings) != 1 {
				t.Fatalf("statement 2 findings = %+v, want exactly one dependency blocker", findings)
			}
			finding := findings[0]
			if finding.RuleID != t07RuleDependency || finding.Level != rule.LevelBlocker {
				t.Fatalf("finding rule/level = %s/%s, want %s/blocker", finding.RuleID, finding.Level, t07RuleDependency)
			}
			wantMetadata := map[string]any{
				"table":            "t",
				"column":           "a",
				"dependent_column": "b",
				"dependency_kind":  "generated",
				"dependency":       "direct",
				"action":           "drop_column",
			}
			if !reflect.DeepEqual(finding.Metadata, wantMetadata) {
				t.Fatalf("finding metadata = %#v, want %#v", finding.Metadata, wantMetadata)
			}
			if result.Verdict != report.VerdictReject {
				t.Fatalf("verdict = %s, want reject", result.Verdict)
			}
		})
	}
}
