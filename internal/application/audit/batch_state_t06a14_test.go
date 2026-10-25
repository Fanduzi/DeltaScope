// Package audit verifies the T06-A14 three ordinary CREATE TABLE
// table-option allowlist policies at the application seam.
package audit

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

const t06A14OmittedSQL = "CREATE TABLE t (c INT);"
const t06A14ColumnCharsetSQL = "CREATE TABLE t (c VARCHAR(8) CHARACTER SET utf8mb4);"

// t06A14Families maps each family key to its rule, allowed/denied inputs,
// message label, and metadata option key.
func t06A14Families() []struct {
	key       string
	ruleID    string
	option    string
	label     string
	allowed   string
	allowSQL  string
	denySQL   string
	denyValue string
} {
	return []struct {
		key       string
		ruleID    string
		option    string
		label     string
		allowed   string
		allowSQL  string
		denySQL   string
		denyValue string
	}{
		{"engine", "ddl.table.engine.allowlist", "engine", "engine", "InnoDB",
			"CREATE TABLE t (c INT) ENGINE=InnoDB;",
			"CREATE TABLE t (c INT) ENGINE=MyISAM;", "MyISAM"},
		{"charset", "ddl.table.charset.allowlist", "charset", "charset", "utf8mb4",
			"CREATE TABLE t (c INT) DEFAULT CHARSET=utf8mb4;",
			"CREATE TABLE t (c INT) DEFAULT CHARSET=latin1;", "latin1"},
		{"row-format", "ddl.table.row_format.allowlist", "row_format", "row format", "DYNAMIC",
			"CREATE TABLE t (c INT) ROW_FORMAT=DYNAMIC;",
			"CREATE TABLE t (c INT) ROW_FORMAT=COMPACT;", "COMPACT"},
	}
}

func t06A14Policy(t *testing.T, ruleID, valuesYAML string, requireExplicit bool) string {
	t.Helper()
	params := "      values: [" + valuesYAML + "]\n"
	if requireExplicit {
		params += "      require_explicit: true\n"
	} else {
		params += "      require_explicit: false\n"
	}
	return t05PolicyPath(t, map[string]string{ruleID: params})
}

func t06A14Audit(t *testing.T, dialect spec.Dialect, sql, policy string) report.Result {
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

// TestT06A14OfflineRoleMatrix runs the 36-role frozen matrix through
// AuditSQL under both dialects: allowed pins the clean explicit path,
// denied-strict/omitted-strict/denied-optional pin the single frozen
// blocker with complete identity, omitted-optional pins loaded+applicable
// with zero findings (never the forbid=false 1/0 state), and denied-off
// pins the absent summary.
func TestT06A14OfflineRoleMatrix(t *testing.T) {
	t.Parallel()
	checkFinding := func(t *testing.T, result report.Result, family struct {
		key       string
		ruleID    string
		option    string
		label     string
		allowed   string
		allowSQL  string
		denySQL   string
		denyValue string
	}, actual string) {
		t.Helper()
		statement := result.Statements[0]
		if result.Verdict != report.VerdictReject || result.Summary.Blockers != 1 ||
			result.Summary.Warnings != 0 || result.Summary.Notices != 0 {
			t.Fatalf("verdict/summary = %s/%+v, want reject with 1 blocker", result.Verdict, result.Summary)
		}
		if len(statement.Findings) != 1 {
			t.Fatalf("findings = %+v, want 1", statement.Findings)
		}
		finding := statement.Findings[0]
		wantMessage := "table " + family.label + " must be one of [" + family.allowed + "]"
		if finding.RuleID != family.ruleID || finding.Level != "blocker" ||
			finding.Message != wantMessage {
			t.Fatalf("finding = %+v, want %s blocker %q", finding, family.ruleID, wantMessage)
		}
		if finding.Metadata["table"] != "t" || finding.Metadata["option"] != family.option ||
			finding.Metadata["actual"] != actual {
			t.Fatalf("metadata = %+v, want table=t option=%s actual=%q",
				finding.Metadata, family.option, actual)
		}
		allowed, ok := finding.Metadata["allowed"].([]string)
		if !ok || len(allowed) != 1 || allowed[0] != family.allowed {
			t.Fatalf("metadata.allowed = %+v, want [%s]", finding.Metadata["allowed"], family.allowed)
		}
		if finding.Location == nil || finding.Location.Line != 1 || finding.Location.Column != 1 {
			t.Fatalf("location = %+v, want 1:1", finding.Location)
		}
	}
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		for _, family := range t06A14Families() {
			family := family
			strict := t06A14Policy(t, family.ruleID, `"`+family.allowed+`"`, true)
			optional := t06A14Policy(t, family.ruleID, `"`+family.allowed+`"`, false)

			t.Run(string(dialect)+"/"+family.key+"-allowed-strict", func(t *testing.T) {
				result := t06A14Audit(t, dialect, family.allowSQL, strict)
				if result.Verdict != report.VerdictPass || len(result.Statements[0].Findings) != 0 {
					t.Fatalf("verdict/findings = %s/%+v, want pass with none",
						result.Verdict, result.Statements[0].Findings)
				}
				if result.RuleSummary == nil || result.RuleSummary.Loaded != 1 || result.RuleSummary.Applicable != 1 {
					t.Fatalf("rule summary = %+v, want loaded=1 applicable=1", result.RuleSummary)
				}
			})
			t.Run(string(dialect)+"/"+family.key+"-denied-strict", func(t *testing.T) {
				result := t06A14Audit(t, dialect, family.denySQL, strict)
				checkFinding(t, result, family, family.denyValue)
				if result.RuleSummary == nil || result.RuleSummary.Loaded != 1 || result.RuleSummary.Applicable != 1 {
					t.Fatalf("rule summary = %+v, want 1/1", result.RuleSummary)
				}
			})
			t.Run(string(dialect)+"/"+family.key+"-omitted-strict", func(t *testing.T) {
				result := t06A14Audit(t, dialect, t06A14OmittedSQL, strict)
				checkFinding(t, result, family, "")
			})
			t.Run(string(dialect)+"/"+family.key+"-omitted-optional", func(t *testing.T) {
				result := t06A14Audit(t, dialect, t06A14OmittedSQL, optional)
				if result.Verdict != report.VerdictPass || len(result.Statements[0].Findings) != 0 {
					t.Fatalf("verdict/findings = %s/%+v, want pass — optional exempts omission only",
						result.Verdict, result.Statements[0].Findings)
				}
				if result.RuleSummary == nil || result.RuleSummary.Loaded != 1 || result.RuleSummary.Applicable != 1 {
					t.Fatalf("rule summary = %+v, want loaded=1 applicable=1 — never the forbid=false 1/0 state",
						result.RuleSummary)
				}
			})
			t.Run(string(dialect)+"/"+family.key+"-denied-optional", func(t *testing.T) {
				result := t06A14Audit(t, dialect, family.denySQL, optional)
				checkFinding(t, result, family, family.denyValue)
			})
			t.Run(string(dialect)+"/"+family.key+"-denied-off", func(t *testing.T) {
				result := t06A14Audit(t, dialect, family.denySQL, t05PolicyPath(t, map[string]string{}))
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
}

// TestT06A14ColumnCharsetCannotSatisfyTableRequirement pins the scope
// boundary through AuditSQL: the column-level CHARACTER SET declaration
// lands on Column.Charset but never fabricates Options["charset"], so the
// table-charset STRICT policy still rejects the statement.
func TestT06A14ColumnCharsetCannotSatisfyTableRequirement(t *testing.T) {
	t.Parallel()
	strict := t06A14Policy(t, "ddl.table.charset.allowlist", `"utf8mb4"`, true)
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect), func(t *testing.T) {
			result := t06A14Audit(t, dialect, t06A14ColumnCharsetSQL, strict)
			statement := result.Statements[0]
			if result.Verdict != report.VerdictReject || len(statement.Findings) != 1 {
				t.Fatalf("verdict/findings = %s/%+v, want reject — column charset is not a table option",
					result.Verdict, statement.Findings)
			}
			finding := statement.Findings[0]
			if finding.RuleID != "ddl.table.charset.allowlist" ||
				finding.Metadata["actual"] != "" {
				t.Fatalf("finding = %+v, want charset allowlist actual=\"\"", finding)
			}
		})
	}
}

// TestT06A14DerivedPreStateCarriesDeclaredOptions pins the enrich seam on
// CREATE+ALTER: the declared Options keys reach the follower pre-state
// verbatim, the omitted input fabricates no option, the provider is read
// exactly once for golden.t, and neither the verdict nor the publication
// rewrites the source declaration.
func TestT06A14DerivedPreStateCarriesDeclaredOptions(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		sql     string
		options map[string]string
	}{
		{"engine-denied", "CREATE TABLE t (c INT) ENGINE=MyISAM;\nALTER TABLE t ADD COLUMN next_c INT;",
			map[string]string{"engine": "MyISAM"}},
		{"charset-denied", "CREATE TABLE t (c INT) DEFAULT CHARSET=latin1;\nALTER TABLE t ADD COLUMN next_c INT;",
			map[string]string{"charset": "latin1"}},
		{"rowformat-denied", "CREATE TABLE t (c INT) ROW_FORMAT=COMPACT;\nALTER TABLE t ADD COLUMN next_c INT;",
			map[string]string{"row_format": "COMPACT"}},
		{"omitted", "CREATE TABLE t (c INT);\nALTER TABLE t ADD COLUMN next_c INT;",
			map[string]string{}},
	}
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		for _, tc := range cases {
			tc := tc
			t.Run(string(dialect)+"/"+tc.name, func(t *testing.T) {
				ctx := context.Background()
				parsed, err := Parse(ctx, tc.sql, dialect)
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
				if len(snapshot.Options) != len(tc.options) {
					t.Fatalf("pre-state options = %#v, want %#v", snapshot.Options, tc.options)
				}
				for key, want := range tc.options {
					if got := snapshot.Options[key]; got != want {
						t.Fatalf("pre-state Options[%q] = %q, want %q", key, got, want)
					}
				}
				for _, key := range []string{"engine", "charset", "row_format"} {
					if _, want := tc.options[key]; !want {
						if _, present := snapshot.Options[key]; present {
							t.Fatalf("pre-state Options[%q] fabricated on %s", key, tc.name)
						}
					}
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
				// Clone isolation: mutating the derived snapshot never
				// back-writes the source declaration.
				for key := range snapshot.Options {
					snapshot.Options[key] = "MUTATED"
				}
				for key, want := range tc.options {
					if statements[0].DDL.Options[key] != want {
						t.Fatalf("source Options[%q] mutated to %q, want %q",
							key, statements[0].DDL.Options[key], want)
					}
				}
			})
		}
	}
}
