// Package audit verifies the T07 expression-default family: supplemented
// parenthesized defaults, ALTER SET/DROP DEFAULT, typed facts, ordered state,
// and the compatibility/dependency rule behavior for both dialects.
// input: real-parser SQL plus controlled provider snapshots
// output: exact column facts, actions, findings, gaps, and post-state shapes
// pos: T07 default-expression coverage beside the generated-column suite
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// t07Columns parses and extracts one SQL string, failing on any parse error.
func t07Columns(t *testing.T, sqlText string, dialect spec.Dialect) []spec.Statement {
	t.Helper()
	parsed, err := Parse(context.Background(), sqlText, dialect)
	if err != nil {
		t.Fatalf("parse %q: %v", sqlText, err)
	}
	statements, err := Extract(context.Background(), parsed)
	if err != nil {
		t.Fatalf("extract %q: %v", sqlText, err)
	}
	return statements
}

// TestT07ExpressionDefaultExtraction pins the declaration-time expression
// facts the supplement produces: real AST references and functions, preserved
// raw default text, and no placeholder residue in statement identity.
func TestT07ExpressionDefaultExtraction(t *testing.T) {
	t.Parallel()
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			cases := []struct {
				name         string
				sql          string
				column       string
				wantRefs     []string
				wantFuncs    []string
				wantFeatures []string
				wantExpr     bool
			}{
				{"arith_ref", "CREATE TABLE t (a INT, b INT DEFAULT (a + 1))", "b", []string{"a"}, nil, []string{"arithmetic"}, true},
				{"arith_constant", "CREATE TABLE t (a INT, b INT DEFAULT (1 + 2))", "b", nil, nil, []string{"arithmetic"}, true},
				{"uuid", "CREATE TABLE t (a INT, b INT DEFAULT (uuid()))", "b", nil, []string{"uuid"}, nil, true},
				{"json_object_ref", "CREATE TABLE t (a INT, b JSON DEFAULT (json_object('k', a)))", "b", []string{"a"}, []string{"json_object"}, nil, true},
				{"literal", "CREATE TABLE t (a INT, b INT DEFAULT 7)", "b", nil, nil, nil, false},
				{"null_literal", "CREATE TABLE t (a INT, b INT DEFAULT NULL)", "b", nil, nil, nil, false},
				{"empty_string", "CREATE TABLE t (a INT, b VARCHAR(8) DEFAULT '')", "b", nil, nil, nil, false},
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					statements := t07Columns(t, tc.sql, dialect)
					if len(statements) != 1 {
						t.Fatalf("statements = %d, want 1", len(statements))
					}
					if !strings.Contains(statements[0].RawSQL, "DEFAULT") {
						t.Fatalf("raw sql lost the default clause: %q", statements[0].RawSQL)
					}
					var got *spec.Column
					for i := range statements[0].DDL.Columns {
						if statements[0].DDL.Columns[i].Name == tc.column {
							got = &statements[0].DDL.Columns[i]
						}
					}
					if got == nil || !got.HasDefault {
						t.Fatalf("column %q hasDefault = %+v", tc.column, got)
					}
					if tc.wantExpr {
						if got.DefaultExpression == nil {
							t.Fatalf("column %q missing DefaultExpression for %q", tc.column, tc.sql)
						}
						expr := *got.DefaultExpression
						if !expr.Known {
							t.Fatalf("column %q expression facts unknown for %q", tc.column, tc.sql)
						}
						var refs []string
						for _, ref := range expr.References {
							refs = append(refs, ref.Name)
						}
						if !reflect.DeepEqual(refs, tc.wantRefs) {
							t.Fatalf("column %q refs = %v, want %v", tc.column, refs, tc.wantRefs)
						}
						if !reflect.DeepEqual(normalizedNil(expr.Functions), normalizedNil(tc.wantFuncs)) {
							t.Fatalf("column %q functions = %v, want %v", tc.column, expr.Functions, tc.wantFuncs)
						}
						for _, feature := range tc.wantFeatures {
							found := false
							for _, got := range expr.Features {
								if got == feature {
									found = true
								}
							}
							if !found {
								t.Fatalf("column %q features = %v, want feature %q", tc.column, expr.Features, feature)
							}
						}
						if expr.Fingerprint == "" {
							t.Fatalf("column %q missing fingerprint", tc.column)
						}
					} else if got.DefaultExpression != nil {
						t.Fatalf("column %q literal default gained expression facts: %+v", tc.column, got.DefaultExpression)
					}
				})
			}
		})
	}
}

// normalizedNil collapses nil and empty slices for the equality checks above.
func normalizedNil(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	return in
}

// TestT07DefaultLiteralIdentities keeps the T06 literal/NULL/temporal default
// contract intact through the new shared default path.
func TestT07DefaultLiteralIdentities(t *testing.T) {
	t.Parallel()
	statements := t07Columns(t, "CREATE TABLE t (a INT DEFAULT NULL, b VARCHAR(4) DEFAULT '', c TIMESTAMP DEFAULT CURRENT_TIMESTAMP, d INT DEFAULT -3)", spec.DialectMySQL)
	cols := statements[0].DDL.Columns
	if !cols[0].DefaultIsNull || cols[0].DefaultValue != "NULL" {
		t.Fatalf("DEFAULT NULL = %+v", cols[0])
	}
	if !cols[1].HasDefault || cols[1].DefaultValue != "''" || cols[1].DefaultIsNull {
		t.Fatalf("empty-string default = %+v", cols[1])
	}
	if !cols[2].DefaultIsCurrentTimestamp || cols[2].DefaultExpression != nil {
		t.Fatalf("current_timestamp default = %+v", cols[2])
	}
	// A signed literal default keeps its pre-existing normalized shape: a
	// unary-minus AST node has no literal text, so DefaultValue stays empty
	// while HasDefault is recorded — identical to the pre-T07 path.
	if !cols[3].HasDefault || cols[3].DefaultIsNull || cols[3].DefaultExpression != nil {
		t.Fatalf("signed literal default = %+v", cols[3])
	}
}

// TestT07AlterSetDropDefaultExtraction pins the default-only setter actions:
// real column identity, set_column_default/drop_column_default naming, and
// full expression facts on the definition.
func TestT07AlterSetDropDefaultExtraction(t *testing.T) {
	t.Parallel()
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			set := t07Columns(t, "ALTER TABLE t ALTER COLUMN b SET DEFAULT (a + 1)", dialect)
			if len(set[0].DDL.Alter) != 1 {
				t.Fatalf("set default alter actions = %+v, want exactly 1", set[0].DDL.Alter)
			}
			alter := set[0].DDL.Alter[0]
			if alter.Action != "set_column_default" {
				t.Fatalf("set default action = %q", alter.Action)
			}
			if alter.Name != "b" || alter.Column == nil || alter.Column.OldName != "b" || alter.Column.Definition == nil {
				t.Fatalf("set default column = %+v", alter.Column)
			}
			def := alter.Column.Definition
			if def.DefaultExpression == nil || !def.DefaultExpression.Known ||
				len(def.DefaultExpression.References) != 1 || def.DefaultExpression.References[0].Name != "a" {
				t.Fatalf("set default expression facts = %+v", def.DefaultExpression)
			}
			if alter.Column.Change == nil || !alter.Column.Change.TouchesDefault {
				t.Fatalf("set default must mark TouchesDefault: %+v", alter.Column.Change)
			}
			if !strings.Contains(set[0].RawSQL, "a + 1") {
				t.Fatalf("set default raw sql lost the expression: %q", set[0].RawSQL)
			}

			drop := t07Columns(t, "ALTER TABLE t ALTER COLUMN b DROP DEFAULT", dialect)
			if len(drop[0].DDL.Alter) != 1 {
				t.Fatalf("drop default alter actions = %+v, want exactly 1", drop[0].DDL.Alter)
			}
			alter = drop[0].DDL.Alter[0]
			if alter.Action != "drop_column_default" || alter.Name != "b" || alter.Column == nil || alter.Column.OldName != "b" {
				t.Fatalf("drop default = action %q column %+v", alter.Action, alter.Column)
			}
			if alter.Column.Definition == nil || alter.Column.Definition.DefaultExpression != nil {
				t.Fatalf("drop default definition = %+v", alter.Column.Definition)
			}
		})
	}
}

// TestT07ExpressionDefaultMalformedFailsClosed keeps supplemented grammar
// bounded: malformed defaults and non-expression SELECT structure stay parse
// errors, never fabricated semantics.
func TestT07ExpressionDefaultMalformedFailsClosed(t *testing.T) {
	t.Parallel()
	for _, sqlText := range []string{
		"CREATE TABLE t (a INT DEFAULT (a +",
		"CREATE TABLE t (a INT DEFAULT ())",
	} {
		if _, err := Parse(context.Background(), sqlText, spec.DialectMySQL); err == nil {
			t.Fatalf("malformed default %q parsed without error", sqlText)
		}
	}
	// A subquery inside DEFAULT (...) is structurally an expression: it
	// parses through the supplement and the construct is a recognized
	// forbidden-construct fact, not a parse failure.
	parsed, err := Parse(context.Background(), "CREATE TABLE t (a INT DEFAULT (SELECT 1 FROM t2))", spec.DialectMySQL)
	if err != nil {
		t.Fatalf("subquery default should parse structurally: %v", err)
	}
	statements, err := Extract(context.Background(), parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	col := statements[0].DDL.Columns[0]
	if col.DefaultExpression == nil || col.DefaultExpression.Known {
		t.Fatalf("subquery default facts = %+v, want explicit unknown construct facts", col.DefaultExpression)
	}
	found := false
	for _, feature := range col.DefaultExpression.Features {
		if feature == spec.ExpressionFeatureSubquery {
			found = true
		}
	}
	if !found {
		t.Fatalf("subquery default features = %v, want %q", col.DefaultExpression.Features, spec.ExpressionFeatureSubquery)
	}
}

// TestT07SetDropDefaultPreservesState proves the default-only setter clones
// the stored column: type/generated/identity facts survive, only the default
// facts change, and the resulting dependency keeps DROP a blocked.
func TestT07SetDropDefaultPreservesState(t *testing.T) {
	t.Parallel()
	for _, variant := range []struct {
		dialect  spec.Dialect
		setExpr  string
		wantRef  string
		bType    string
		wantType string
	}{
		{spec.DialectMySQL, "(a + 1)", "a", "INT", "int(11)"},
		// TiDB's observed positive expression-default family is uuid() on
		// VARCHAR(36); an arithmetic reference default is a vendor negative
		// that must not publish a post-state, so the TiDB state case uses
		// the observed positive datatype.
		{spec.DialectTiDB, "(uuid())", "", "VARCHAR(36)", "varchar(36)"},
	} {
		variant := variant
		t.Run(string(variant.dialect), func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			statements := t07Columns(t, strings.Join([]string{
				"CREATE TABLE t (a INT, b " + variant.bType + ")",
				"ALTER TABLE t ALTER COLUMN b SET DEFAULT " + variant.setExpr,
				"ALTER TABLE t ALTER COLUMN b DROP DEFAULT",
			}, ";"), variant.dialect)
			version := &spec.VersionIdentity{Product: spec.VersionProductMySQL, Version: "8.0.46", Major: 8, Minor: 0, Patch: 46, ValidatedRange: true}
			if variant.dialect == spec.DialectTiDB {
				version = &spec.VersionIdentity{Product: spec.VersionProductTiDB, Version: "8.5.0", Major: 8, Minor: 5, Patch: 0, ValidatedRange: true}
			}
			state := newBatchState(variant.dialect, "golden", nil)
			target := spec.Table{Schema: "golden", Name: "t"}
			for i := range statements {
				pre, err := state.preState(ctx, "golden", target)
				if err != nil {
					t.Fatalf("pre-state before statement %d: %v", i, err)
				}
				statements[i].Metadata = &spec.Metadata{Version: version, TargetTable: pre}
				if err := state.apply(ctx, statements[i]); err != nil {
					t.Fatalf("apply statement %d: %v", i, err)
				}
				// Between statements the applied shape must reflect the setter.
				if i == 1 {
					pre, err := state.preState(ctx, "golden", target)
					if err != nil {
						t.Fatalf("pre-state after set: %v", err)
					}
					if pre == nil {
						t.Fatalf("pre-state after set is nil — the positive default transition must publish a shape")
					}
					var b *spec.Column
					for j := range pre.Columns {
						if pre.Columns[j].Name == "b" {
							b = &pre.Columns[j]
						}
					}
					if b == nil || !b.HasDefault || b.Type != variant.wantType {
						t.Fatalf("post-set column b = %+v", b)
					}
					if b.DefaultExpression == nil || !b.DefaultExpression.Known {
						t.Fatalf("post-set default facts = %+v", b.DefaultExpression)
					}
					if variant.wantRef != "" {
						if len(b.DefaultExpression.References) != 1 || b.DefaultExpression.References[0].Name != variant.wantRef {
							t.Fatalf("post-set default refs = %+v, want %q", b.DefaultExpression.References, variant.wantRef)
						}
					}
				}
			}
			post, err := state.preState(ctx, "golden", target)
			if err != nil {
				t.Fatalf("post-state: %v", err)
			}
			if post == nil {
				t.Fatalf("post-state is nil — DROP DEFAULT must keep a known shape")
			}
			var b *spec.Column
			for j := range post.Columns {
				if post.Columns[j].Name == "b" {
					b = &post.Columns[j]
				}
			}
			if b == nil || b.HasDefault || b.DefaultExpression != nil || b.DefaultIsNull || b.Type != variant.wantType {
				t.Fatalf("post-drop column b = %+v, want default facts cleared and type preserved", b)
			}
		})
	}
}

// TestT07ExpressionDefaultDependency proves a stored expression default binds
// a column dependency: dropping the referenced source is the exact blocker.
func TestT07ExpressionDefaultDependency(t *testing.T) {
	t.Parallel()
	sql := strings.Join([]string{
		"CREATE TABLE t (\n  a INT,\n  b INT DEFAULT (a + 1)\n)",
		"ALTER TABLE t DROP COLUMN a",
	}, ";\n") + ";"
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
				SQL:           sql,
				Dialect:       variant.dialect,
				Schema:        "golden",
				TargetVersion: variant.version,
				ConfigPath:    t07PolicyPath(t, map[string]string{t07RuleDependency: "      required: true\n"}),
			})
			if err != nil {
				t.Fatalf("audit: %v", err)
			}
			// TiDB rejects the expression default on statement 0 — an invalid
			// transition that keeps the table unpublished, so the follow-up
			// DROP reports the bounded unknown-table-state gap, not a known
			// dependent blocker.
			if variant.dialect == spec.DialectTiDB {
				if len(result.Statements[1].Findings) != 0 {
					t.Fatalf("tidb statement 1 findings = %+v, want none", result.Statements[1].Findings)
				}
				found := false
				for _, gap := range result.Statements[1].EvidenceGaps {
					if gap.ReasonCode == "unknown_table_state" {
						found = true
					}
				}
				if !found {
					t.Fatalf("tidb statement 1 gaps = %+v, want unknown_table_state", result.Statements[1].EvidenceGaps)
				}
				return
			}
			if len(result.Statements[0].Findings) != 0 {
				t.Fatalf("statement 0 findings = %+v", result.Statements[0].Findings)
			}
			findings := result.Statements[1].Findings
			if len(findings) != 1 {
				t.Fatalf("statement 1 findings = %+v, want one dependency blocker", findings)
			}
			finding := findings[0]
			wantMessage := `cannot drop column "a": expression-default column "b" depends on it`
			if finding.Message != wantMessage {
				t.Fatalf("finding message = %q, want %q", finding.Message, wantMessage)
			}
			if finding.Metadata["dependency_kind"] != "expression_default" || finding.Metadata["dependent_column"] != "b" {
				t.Fatalf("finding metadata = %#v", finding.Metadata)
			}
		})
	}
}

// TestT07ExpressionDefaultCompatVersions pins the MySQL 8.0.13 minimum and
// the missing-version gap behavior for expression defaults.
func TestT07ExpressionDefaultCompatVersions(t *testing.T) {
	t.Parallel()
	compatOnly := func(t *testing.T) string {
		return t07PolicyPath(t, map[string]string{t07RuleCompat: "      required: true\n"})
	}
	t.Run("mysql_below_minimum", func(t *testing.T) {
		result, err := AuditSQL(context.Background(), Request{
			SQL:           "CREATE TABLE t (a INT DEFAULT (1 + 2))",
			Dialect:       spec.DialectMySQL,
			Schema:        "golden",
			TargetVersion: "8.0.12",
			ConfigPath:    compatOnly(t),
		})
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		if len(result.Statements[0].Findings) != 1 ||
			result.Statements[0].Findings[0].Metadata["reason_code"] != "version_unsupported" ||
			result.Statements[0].Findings[0].Metadata["minimum_version"] != "8.0.13" {
			t.Fatalf("findings = %+v", result.Statements[0].Findings)
		}
	})
	t.Run("mysql_supported", func(t *testing.T) {
		result, err := AuditSQL(context.Background(), Request{
			SQL:           "CREATE TABLE t (a INT DEFAULT (1 + 2))",
			Dialect:       spec.DialectMySQL,
			Schema:        "golden",
			TargetVersion: "8.0.46",
			ConfigPath:    compatOnly(t),
		})
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		if len(result.Statements[0].Findings) != 0 || len(result.Statements[0].EvidenceGaps) != 0 {
			t.Fatalf("findings/gaps = %+v/%+v", result.Statements[0].Findings, result.Statements[0].EvidenceGaps)
		}
	})
	t.Run("mysql_missing_version", func(t *testing.T) {
		result, err := AuditSQL(context.Background(), Request{
			SQL:        "CREATE TABLE t (a INT DEFAULT (1 + 2))",
			Dialect:    spec.DialectMySQL,
			Schema:     "golden",
			ConfigPath: compatOnly(t),
		})
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		gaps := result.Statements[0].EvidenceGaps
		if len(result.Statements[0].Findings) != 0 {
			t.Fatalf("findings = %+v, want none", result.Statements[0].Findings)
		}
		found := false
		for _, gap := range gaps {
			if gap.ReasonCode == "missing_target_version" {
				found = true
			}
		}
		if !found {
			t.Fatalf("gaps = %+v, want missing_target_version", gaps)
		}
	})
	t.Run("tidb_constant_arithmetic", func(t *testing.T) {
		result, err := AuditSQL(context.Background(), Request{
			SQL:           "CREATE TABLE t (a INT DEFAULT (1 + 2))",
			Dialect:       spec.DialectTiDB,
			Schema:        "golden",
			TargetVersion: "8.5.0",
			ConfigPath:    compatOnly(t),
		})
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		findings := result.Statements[0].Findings
		if len(findings) != 1 || findings[0].Metadata["reason_code"] != "tidb_default_expression" {
			t.Fatalf("findings = %+v, want tidb_default_expression", findings)
		}
	})
	t.Run("tidb_alter_set_default_uuid_positive", func(t *testing.T) {
		snapshot := presentTable("golden", "t", "a")
		// The frozen native positive is uuid() on VARCHAR(36), not INT.
		snapshot.Columns[0].Type = "varchar(36)"
		provider := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{
			"golden.t": snapshot,
		}}
		provider.t07InstanceVersion = "8.0.11-TiDB-v8.5.0"
		result, err := AuditSQL(context.Background(), Request{
			SQL:              "ALTER TABLE t ALTER COLUMN a SET DEFAULT (uuid())",
			Dialect:          spec.DialectTiDB,
			Schema:           "golden",
			ConfigPath:       compatOnly(t),
			MetadataProvider: provider,
		})
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		if len(result.Statements[0].Findings) != 0 || len(result.Statements[0].EvidenceGaps) != 0 {
			t.Fatalf("findings/gaps = %+v/%+v", result.Statements[0].Findings, result.Statements[0].EvidenceGaps)
		}
	})
	t.Run("tidb_alter_set_default_uuid_int", func(t *testing.T) {
		// uuid() on an INT target is an observed positive cell (TiDB 8.5.0
		// stores the expression): a clean pass on the compat seam.
		snapshot := presentTable("golden", "t", "a")
		provider := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{
			"golden.t": snapshot,
		}}
		provider.t07InstanceVersion = "8.0.11-TiDB-v8.5.0"
		result, err := AuditSQL(context.Background(), Request{
			SQL:              "ALTER TABLE t ALTER COLUMN a SET DEFAULT (uuid())",
			Dialect:          spec.DialectTiDB,
			Schema:           "golden",
			ConfigPath:       compatOnly(t),
			MetadataProvider: provider,
		})
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		if len(result.Statements[0].Findings) != 0 || len(result.Statements[0].EvidenceGaps) != 0 {
			t.Fatalf("findings/gaps = %+v/%+v, want none", result.Statements[0].Findings, result.Statements[0].EvidenceGaps)
		}
	})
	t.Run("tidb_alter_set_default_uuid_on_date", func(t *testing.T) {
		// A single bare allowed call stores the expression verbatim on
		// every non-BLOB/TEXT/JSON target — uuid() on DATE is an observed
		// verbatim acceptance.
		snapshot := presentTable("golden", "t", "a")
		snapshot.Columns[0].Type = "date"
		provider := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{
			"golden.t": snapshot,
		}}
		provider.t07InstanceVersion = "8.0.11-TiDB-v8.5.0"
		result, err := AuditSQL(context.Background(), Request{
			SQL:              "ALTER TABLE t ALTER COLUMN a SET DEFAULT (uuid())",
			Dialect:          spec.DialectTiDB,
			Schema:           "golden",
			ConfigPath:       compatOnly(t),
			MetadataProvider: provider,
		})
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		if len(result.Statements[0].Findings) != 0 || len(result.Statements[0].EvidenceGaps) != 0 {
			t.Fatalf("findings/gaps = %+v/%+v, want none", result.Statements[0].Findings, result.Statements[0].EvidenceGaps)
		}
	})
	t.Run("mysql_set_default_on_prior_generated", func(t *testing.T) {
		// SET DEFAULT landing on a generated column is a documented negative
		// on both products; the declare-side grammar rejection does not cover
		// the alter-side setter.
		snapshot := presentTable("golden", "t", "a")
		snapshot.Columns[0].Generated = &spec.GeneratedColumn{
			Storage:    "virtual",
			Expression: spec.ExpressionFacts{Known: true},
		}
		provider := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{
			"golden.t": snapshot,
		}}
		result, err := AuditSQL(context.Background(), Request{
			SQL:              "ALTER TABLE t ALTER COLUMN a SET DEFAULT (uuid())",
			Dialect:          spec.DialectMySQL,
			Schema:           "golden",
			TargetVersion:    "8.0.46",
			ConfigPath:       compatOnly(t),
			MetadataProvider: provider,
		})
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		findings := result.Statements[0].Findings
		if len(findings) != 1 || findings[0].Metadata["reason_code"] != "expression_construct_forbidden" {
			t.Fatalf("findings = %+v, want expression_construct_forbidden", findings)
		}
	})
	t.Run("tidb_alter_set_json_default_on_json_target", func(t *testing.T) {
		// A JSON-target column cannot carry a default at all (observed
		// 1101) — the rescoped tidb_alter_json_default issue, independent
		// of which allowed callee is used.
		snapshot := presentTable("golden", "t", "a")
		snapshot.Columns[0].Type = "json"
		provider := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{
			"golden.t": snapshot,
		}}
		provider.t07InstanceVersion = "8.0.11-TiDB-v8.5.0"
		result, err := AuditSQL(context.Background(), Request{
			SQL:              "ALTER TABLE t ALTER COLUMN a SET DEFAULT (json_object('k', 1))",
			Dialect:          spec.DialectTiDB,
			Schema:           "golden",
			ConfigPath:       compatOnly(t),
			MetadataProvider: provider,
		})
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		findings := result.Statements[0].Findings
		if len(findings) != 1 || findings[0].Metadata["reason_code"] != "tidb_alter_json_default" {
			t.Fatalf("findings = %+v, want tidb_alter_json_default", findings)
		}
	})
	t.Run("tidb_alter_set_json_default_varchar", func(t *testing.T) {
		// json_object on a VARCHAR target is an observed positive cell —
		// the function is allowed and the bare call stores the expression.
		snapshot := presentTable("golden", "t", "a")
		snapshot.Columns[0].Type = "varchar(255)"
		provider := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{
			"golden.t": snapshot,
		}}
		provider.t07InstanceVersion = "8.0.11-TiDB-v8.5.0"
		result, err := AuditSQL(context.Background(), Request{
			SQL:              "ALTER TABLE t ALTER COLUMN a SET DEFAULT (json_object('k', 1))",
			Dialect:          spec.DialectTiDB,
			Schema:           "golden",
			ConfigPath:       compatOnly(t),
			MetadataProvider: provider,
		})
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		if len(result.Statements[0].Findings) != 0 || len(result.Statements[0].EvidenceGaps) != 0 {
			t.Fatalf("findings/gaps = %+v/%+v, want none", result.Statements[0].Findings, result.Statements[0].EvidenceGaps)
		}
	})
	t.Run("tidb_alter_set_json_default_on_int", func(t *testing.T) {
		// json_object on INT is an observed verbatim acceptance — the
		// bare-call setter stores the expression, deferred to INSERT.
		snapshot := presentTable("golden", "t", "a")
		provider := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{
			"golden.t": snapshot,
		}}
		provider.t07InstanceVersion = "8.0.11-TiDB-v8.5.0"
		result, err := AuditSQL(context.Background(), Request{
			SQL:              "ALTER TABLE t ALTER COLUMN a SET DEFAULT (json_object('k', 1))",
			Dialect:          spec.DialectTiDB,
			Schema:           "golden",
			ConfigPath:       compatOnly(t),
			MetadataProvider: provider,
		})
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		if len(result.Statements[0].Findings) != 0 || len(result.Statements[0].EvidenceGaps) != 0 {
			t.Fatalf("findings/gaps = %+v/%+v, want none", result.Statements[0].Findings, result.Statements[0].EvidenceGaps)
		}
	})
}

// TestT07MultiActionDependencyNetEffect pins the frozen multi-action answer:
// when one statement can both break and repair dependents, the net effect is
// unknown — a bounded gap, never a known-break blocker on one action alone.
func TestT07MultiActionDependencyNetEffect(t *testing.T) {
	t.Parallel()
	for _, variant := range []struct {
		dialect spec.Dialect
		version string
	}{
		{spec.DialectMySQL, "8.0.46"},
		{spec.DialectTiDB, "8.5.0"},
	} {
		variant := variant
		t.Run(string(variant.dialect), func(t *testing.T) {
			t.Parallel()
			snapshot := presentTable("golden", "t", "a", "b")
			snapshot.Columns[1].Generated = &spec.GeneratedColumn{
				Storage: spec.GeneratedStorageStored,
				Expression: spec.ExpressionFacts{
					Known:      true,
					References: []spec.ColumnReference{{Name: "a"}},
				},
			}
			provider := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{
				"golden.t": snapshot,
			}}
			result, err := AuditSQL(context.Background(), Request{
				SQL:              "ALTER TABLE t DROP COLUMN b, DROP COLUMN a",
				Dialect:          variant.dialect,
				Schema:           "golden",
				TargetVersion:    variant.version,
				ConfigPath:       t07PolicyPath(t, map[string]string{t07RuleDependency: "      required: true\n"}),
				MetadataProvider: provider,
			})
			if err != nil {
				t.Fatalf("audit: %v", err)
			}
			statement := result.Statements[0]
			if len(statement.Findings) != 0 {
				t.Fatalf("findings = %+v, want none — multi-action net effect is not provable", statement.Findings)
			}
			found := false
			for _, gap := range statement.EvidenceGaps {
				if gap.ReasonCode == "incomplete_alter_dependency_effects" {
					found = true
					if !reflect.DeepEqual(gap.RequiredFacts, []string{"alter.column_dependency_effects"}) {
						t.Fatalf("gap facts = %v", gap.RequiredFacts)
					}
				}
			}
			if !found {
				t.Fatalf("gaps = %+v, want incomplete_alter_dependency_effects", statement.EvidenceGaps)
			}
		})
	}
}

// TestT07ExpressionDefaultFeatureFilter keeps the features param scoped: at
// MySQL 8.0.12 the expression default is below its 8.0.13 minimum while the
// generated column is above 5.7.6 — so filtering to generated only must
// produce zero findings, and filtering to expression_default only must
// produce exactly the column-a blocker.
func TestT07ExpressionDefaultFeatureFilter(t *testing.T) {
	t.Parallel()
	const sql = "CREATE TABLE t (a INT DEFAULT (1 + 2), b INT GENERATED ALWAYS AS (1 + 2) VIRTUAL)"
	audit := func(t *testing.T, features string) []rule.Finding {
		t.Helper()
		result, err := AuditSQL(context.Background(), Request{
			SQL:           sql,
			Dialect:       spec.DialectMySQL,
			Schema:        "golden",
			TargetVersion: "8.0.12",
			ConfigPath: t07PolicyPath(t, map[string]string{
				t07RuleCompat: "      required: true\n      features: [" + features + "]\n",
			}),
		})
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		return result.Statements[0].Findings
	}
	if findings := audit(t, "generated"); len(findings) != 0 {
		t.Fatalf("features=[generated] findings = %+v, want none", findings)
	}
	findings := audit(t, "expression_default")
	if len(findings) != 1 || findings[0].Metadata["feature"] != "expression_default" ||
		findings[0].Metadata["reason_code"] != "version_unsupported" || findings[0].Metadata["column"] != "a" {
		t.Fatalf("features=[expression_default] findings = %+v", findings)
	}
}
