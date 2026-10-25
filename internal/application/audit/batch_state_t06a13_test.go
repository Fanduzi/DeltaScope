// Package audit verifies the T06-A13 five ordinary CREATE TABLE type-family
// forbid policies at the application seam.
package audit

import (
	"context"
	"testing"

	domainpolicy "github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	ddlrules "github.com/Fanduzi/DeltaScope/internal/domain/rule/ddl"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

const t06A13MixedSQL = "CREATE TABLE t (c_blob BLOB, c_text TEXT, c_json JSON, c_bit BIT(1), c_float FLOAT, c_double DOUBLE, c_timestamp TIMESTAMP NULL, c_datetime DATETIME NULL, c_int INT);"
const t06A13SafeSQL = "CREATE TABLE t (c_int INT, c_string VARCHAR(8), c_datetime DATETIME NULL);"

// t06A13Families maps the five family keys to their rule IDs and the frozen
// per-role attribution on M.
func t06A13Families() []struct {
	key     string
	ruleID  string
	columns []string
	types   []string
} {
	return []struct {
		key     string
		ruleID  string
		columns []string
		types   []string
	}{
		{"blob-text", "ddl.column.blob_text.forbid",
			[]string{"c_blob", "c_text"}, []string{"blob binary", "text"}},
		{"json", "ddl.column.json.forbid",
			[]string{"c_json"}, []string{"json"}},
		{"bit", "ddl.column.bit.forbid",
			[]string{"c_bit"}, []string{"bit(1)"}},
		{"float-double", "ddl.column.float_double.forbid",
			[]string{"c_float", "c_double"}, []string{"float", "double"}},
		{"timestamp", "ddl.column.timestamp.forbid",
			[]string{"c_timestamp"}, []string{"timestamp"}},
	}
}

func t06A13Policy(t *testing.T, ruleID, params string) string {
	t.Helper()
	return t05PolicyPath(t, map[string]string{ruleID: params})
}

func t06A13Audit(t *testing.T, dialect spec.Dialect, sql, policy string) report.Result {
	t.Helper()
	result, err := AuditSQL(context.Background(), Request{
		SQL: sql, Dialect: dialect, Schema: "golden",
		ConfigPath: policy, MetadataProvider: &t05AbsentProvider{},
	})
	if err != nil {
		t.Fatalf("%s %q: unexpected error %v", dialect, sql, err)
	}
	if len(result.Statements) == 0 {
		t.Fatalf("%s %q: no statements projected", dialect, sql)
	}
	return result
}

// TestT06A13OfflineRoleMatrix runs the 32-role frozen matrix through
// AuditSQL under both dialects: M×ON pins the per-column attribution and
// complete finding identity; S×ON pins loaded+applicable with zero hits;
// M×FALSE pins loaded-but-inapplicable; M×OFF pins the absent summary.
func TestT06A13OfflineRoleMatrix(t *testing.T) {
	t.Parallel()
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		for _, family := range t06A13Families() {
			family := family
			on := t06A13Policy(t, family.ruleID, "      forbid: true\n")
			offFalse := t06A13Policy(t, family.ruleID, "      forbid: false\n")

			t.Run(string(dialect)+"/"+family.key+"-mixed-hit", func(t *testing.T) {
				result := t06A13Audit(t, dialect, t06A13MixedSQL, on)
				statement := result.Statements[0]
				if statement.Index != 0 || statement.Kind != "ddl" || statement.RawSQL != t06A13MixedSQL {
					t.Fatalf("statement identity = %+v", statement)
				}
				if result.Verdict != report.VerdictReject || result.Summary.Blockers != len(family.columns) ||
					result.Summary.Warnings != 0 || result.Summary.Notices != 0 {
					t.Fatalf("verdict/summary = %s/%+v, want reject with %d blockers",
						result.Verdict, result.Summary, len(family.columns))
				}
				if result.RuleSummary == nil || result.RuleSummary.Loaded != 1 || result.RuleSummary.Applicable != 1 {
					t.Fatalf("rule summary = %+v, want loaded=1 applicable=1", result.RuleSummary)
				}
				if len(statement.Findings) != len(family.columns) {
					t.Fatalf("findings = %+v, want %d on %v", statement.Findings, len(family.columns), family.columns)
				}
				for i, column := range family.columns {
					finding := statement.Findings[i]
					if finding.RuleID != family.ruleID || finding.Level != "blocker" {
						t.Fatalf("finding[%d] = %+v, want %s blocker", i, finding, family.ruleID)
					}
					var wantMessage string
					if family.key == "float-double" {
						wantMessage = `column "` + column + `" should not use ` + family.types[i]
					} else {
						familyName := map[string]string{"blob-text": "blob/text", "json": "json", "bit": "bit", "timestamp": "timestamp"}[family.key]
						wantMessage = `column "` + column + `" must not use the ` + familyName + ` type family`
					}
					if finding.Message != wantMessage {
						t.Fatalf("finding[%d] message = %q, want %q", i, finding.Message, wantMessage)
					}
					if finding.Metadata["table"] != "t" || finding.Metadata["column"] != column ||
						finding.Metadata["type"] != family.types[i] {
						t.Fatalf("finding[%d] metadata = %+v, want %s %s", i, finding.Metadata, column, family.types[i])
					}
					if family.key == "float-double" {
						if _, ok := finding.Metadata["family"]; ok {
							t.Fatalf("float finding[%d] metadata = %+v carries family", i, finding.Metadata)
						}
					} else {
						if _, ok := finding.Metadata["charset"]; !ok {
							t.Fatalf("finding[%d] metadata = %+v lacks charset", i, finding.Metadata)
						}
					}
				}
			})

			t.Run(string(dialect)+"/"+family.key+"-safe", func(t *testing.T) {
				result := t06A13Audit(t, dialect, t06A13SafeSQL, on)
				if result.Verdict != report.VerdictPass || len(result.Statements[0].Findings) != 0 {
					t.Fatalf("verdict/findings = %s/%+v, want pass with none",
						result.Verdict, result.Statements[0].Findings)
				}
				if result.RuleSummary == nil || result.RuleSummary.Loaded != 1 || result.RuleSummary.Applicable != 1 {
					t.Fatalf("rule summary = %+v, want loaded=1 applicable=1 — applicable, no hits", result.RuleSummary)
				}
			})

			t.Run(string(dialect)+"/"+family.key+"-forbid-false", func(t *testing.T) {
				result := t06A13Audit(t, dialect, t06A13MixedSQL, offFalse)
				if result.Verdict != report.VerdictPass || len(result.Statements[0].Findings) != 0 {
					t.Fatalf("verdict/findings = %s/%+v, want pass with none",
						result.Verdict, result.Statements[0].Findings)
				}
				if result.RuleSummary == nil || result.RuleSummary.Loaded != 1 || result.RuleSummary.Applicable != 0 {
					t.Fatalf("rule summary = %+v, want loaded=1 applicable=0 — forbid=false", result.RuleSummary)
				}
			})
		}

		t.Run(string(dialect)+"/mixed-off", func(t *testing.T) {
			result := t06A13Audit(t, dialect, t06A13MixedSQL, t05PolicyPath(t, map[string]string{}))
			if result.Verdict != report.VerdictPass || len(result.Statements[0].Findings) != 0 {
				t.Fatalf("verdict/findings = %s/%+v, want pass with none",
					result.Verdict, result.Statements[0].Findings)
			}
			if result.RuleSummary != nil {
				t.Fatalf("rule summary = %+v, want absent for all-off", result.RuleSummary)
			}
		})
	}
}

// TestT06A13ReorderedAttribution pins the bounded reorder control through
// AuditSQL: moving matching columns behind non-matching ones keeps the
// findings bound to the real declared columns in declaration order.
func TestT06A13ReorderedAttribution(t *testing.T) {
	t.Parallel()
	reordered := "CREATE TABLE t (c_int INT, c_text TEXT, c_double DOUBLE, c_blob BLOB, c_float FLOAT);"
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect)+"/blob-text", func(t *testing.T) {
			result := t06A13Audit(t, dialect, reordered,
				t06A13Policy(t, "ddl.column.blob_text.forbid", "      forbid: true\n"))
			findings := result.Statements[0].Findings
			if len(findings) != 2 || findings[0].Metadata["column"] != "c_text" ||
				findings[1].Metadata["column"] != "c_blob" {
				t.Fatalf("findings = %+v, want [c_text, c_blob]", findings)
			}
		})
		t.Run(string(dialect)+"/float-double", func(t *testing.T) {
			result := t06A13Audit(t, dialect, reordered,
				t06A13Policy(t, "ddl.column.float_double.forbid", "      forbid: true\n"))
			findings := result.Statements[0].Findings
			if len(findings) != 2 || findings[0].Metadata["column"] != "c_double" ||
				findings[1].Metadata["column"] != "c_float" {
				t.Fatalf("findings = %+v, want [c_double, c_float]", findings)
			}
		})
	}
}

// TestT06A13DerivedPreStateCarriesTypes pins the enrich seam on M+ALTER:
// the follower reads a derived snapshot carrying all nine declared columns
// in order with Type/Charset/Length intact, the provider was consulted
// exactly once for golden.t, and neither the policy verdict nor the
// publication rewrites the source declaration.
func TestT06A13DerivedPreStateCarriesTypes(t *testing.T) {
	t.Parallel()
	wants := []struct {
		name    string
		typ     string
		charset string
		length  int
	}{
		{"c_blob", "blob binary", "", -1}, {"c_text", "text", "", -1},
		{"c_json", "json", "", -1}, {"c_bit", "bit(1)", "", 1},
		{"c_float", "float", "", -1}, {"c_double", "double", "", -1},
		{"c_timestamp", "timestamp", "", 19}, {"c_datetime", "datetime", "", 19},
		{"c_int", "int(11)", "", -1},
	}
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect), func(t *testing.T) {
			ctx := context.Background()
			sql := t06A13MixedSQL + "\nALTER TABLE t ADD COLUMN next_c INT;"
			parsed, err := Parse(ctx, sql, dialect)
			if err != nil {
				t.Fatal(err)
			}
			statements, err := Extract(ctx, parsed)
			if err != nil {
				t.Fatal(err)
			}
			if len(statements) != 2 {
				t.Fatalf("extracted = %d, want 2", len(statements))
			}
			provider := &t05AbsentProvider{}
			state := newBatchState(dialect, "golden", provider)
			facts, _ := provider.LoadInstanceFacts(ctx, dialect, "golden")
			enriched, err := state.enrichStatements(ctx, &MetadataRequest{Schema: "golden", Provider: provider},
				statements, parsed.failures, facts, auditLimits{orderedStatements: 2})
			if err != nil {
				t.Fatal(err)
			}
			var snapshot *spec.TableSnapshot
			if enriched[1].Metadata != nil {
				snapshot = enriched[1].Metadata.TargetTable
			}
			if snapshot == nil || !snapshot.Exists {
				t.Fatalf("follower pre-state = %+v, want an existing t", snapshot)
			}
			if len(snapshot.Columns) != len(wants) {
				t.Fatalf("pre-state columns = %#v, want %d", snapshot.Columns, len(wants))
			}
			for i, want := range wants {
				column := snapshot.Columns[i]
				if column.Name != want.name || column.Type != want.typ ||
					column.Charset != want.charset || column.Length != want.length {
					t.Fatalf("pre-state column[%d] = %#v, want %s %s charset=%q len=%d",
						i, column, want.name, want.typ, want.charset, want.length)
				}
				if &snapshot.Columns[i] == &statements[0].DDL.Columns[i] {
					t.Fatalf("pre-state column[%d] shares statement storage — must be a clone", i)
				}
			}
			if snapshot.PrimaryKey != nil || len(snapshot.Indexes) != 0 {
				t.Fatalf("pre-state pk/indexes = %#v/%#v, want none", snapshot.PrimaryKey, snapshot.Indexes)
			}
			reads := 0
			for _, call := range provider.calls {
				if call == "golden.t" {
					reads++
				}
			}
			if reads != 1 {
				t.Fatalf("provider reads for golden.t = %d, want exactly 1", reads)
			}
			// The source declaration is never back-mutated.
			for i, want := range wants {
				if statements[0].DDL.Columns[i].Type != want.typ {
					t.Fatalf("source column[%d] Type mutated to %q, want %q",
						i, statements[0].DDL.Columns[i].Type, want.typ)
				}
			}
		})
	}
}

// TestT06A13PolicyVerdictDoesNotMutateSource pins the audit/immutability
// boundary on the real evaluation seam: the blob-text policy rejects the
// mixed declaration, yet the extracted columns keep their declared types —
// a verdict is never a rewrite.
func TestT06A13PolicyVerdictDoesNotMutateSource(t *testing.T) {
	t.Parallel()
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect), func(t *testing.T) {
			ctx := context.Background()
			parsed, err := Parse(ctx, t06A13MixedSQL, dialect)
			if err != nil {
				t.Fatal(err)
			}
			statements, err := Extract(ctx, parsed)
			if err != nil {
				t.Fatal(err)
			}
			registry := rule.NewRegistry()
			if err := ddlrules.Register(registry, domainpolicy.Policy{Rules: map[string]domainpolicy.RulePolicy{
				"ddl.column.blob_text.forbid": {Enabled: true, Level: rule.LevelBlocker,
					Params: map[string]any{"forbid": true}},
			}}); err != nil {
				t.Fatalf("register: %v", err)
			}
			result, err := EvaluateStatements(ctx, registry, statements)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			if result.Verdict != report.VerdictReject || len(result.Statements[0].Findings) != 2 {
				t.Fatalf("verdict/findings = %s/%+v, want reject with c_blob+c_text",
					result.Verdict, result.Statements[0].Findings)
			}
			wantTypes := []string{"blob binary", "text", "json", "bit(1)", "float",
				"double", "timestamp", "datetime", "int(11)"}
			for i, column := range statements[0].DDL.Columns {
				if column.Type != wantTypes[i] {
					t.Fatalf("column %s Type mutated to %q by policy reject", column.Name, column.Type)
				}
			}
		})
	}
}
