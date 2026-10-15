// Package audit verifies the T06-A12 ordinary CREATE TABLE non-PK-column
// NOT NULL audit, the time-like exemption, per-column finding attribution,
// and the derived-state boundary.
// input: the frozen A12 CREATE TABLE roles under the isolated profiles
// through AuditSQL, plus create+alter inputs through the enrich seam
// output: per-role verdicts, complete finding identity bound to the owning
// column, the required=false/disabled distinction, and follower pre-state
// NotNull/Type propagation without back-mutation
// pos: application-layer contract tests for the frozen T06-A12 oracle (issue #85)
// note: if this file changes, update this header and module README.md.
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

const t06A12RuleID = "ddl.column.not_null.require"

// t06A12Policy writes one of the three frozen isolated profiles: strict
// (allow_time_null=false), time-allow (allow_time_null=true), or
// not-required (required=false). The all-off profile is the shared
// disabled control.
func t06A12Policy(t *testing.T, params string) string {
	t.Helper()
	return t05PolicyPath(t, map[string]string{t06A12RuleID: params})
}

func t06A12Audit(t *testing.T, dialect spec.Dialect, sql, policy string) report.Result {
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

// TestT06A12OfflineRoleMatrix runs the eight frozen roles through AuditSQL
// under both dialects and pins the complete product contract: verdict,
// per-column finding identity (rule_id, level, message, bound metadata),
// the exemption paths, and the distinct required=false/disabled controls.
func TestT06A12OfflineRoleMatrix(t *testing.T) {
	t.Parallel()
	strict := "      required: true\n      allow_time_null: false\n"
	timeAllow := "      required: true\n      allow_time_null: true\n"
	notRequired := "      required: false\n      allow_time_null: false\n"
	roles := []struct {
		name           string
		sql            string
		params         string // empty means the all-off control
		wantFindings   int
		wantColumn     string
		wantType       string
		wantLoaded     int // -1 means rule_summary must be absent
		wantApplicable int
	}{
		{"ordinary-nullable", "CREATE TABLE t (c INT);", strict,
			1, "c", "int(11)", 1, 1},
		{"ordinary-not-null", "CREATE TABLE t (c INT NOT NULL);", strict,
			0, "", "", 1, 1},
		{"time-exempt", "CREATE TABLE t (c DATETIME NULL);", timeAllow,
			0, "", "", 1, 1},
		{"time-required", "CREATE TABLE t (c DATETIME NULL);", strict,
			1, "c", "datetime", 1, 1},
		{"text-exempt", "CREATE TABLE t (payload TEXT);", strict,
			0, "", "", 1, 1},
		{"mixed-attribution", "CREATE TABLE t (ordinary INT, time_c DATETIME NULL, payload TEXT);", timeAllow,
			1, "ordinary", "int(11)", 1, 1},
		{"required-false", "CREATE TABLE t (c INT);", notRequired,
			0, "", "", 1, 0},
		{"disabled", "CREATE TABLE t (c INT);", "",
			0, "", "", -1, 0},
	}
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		for _, role := range roles {
			role := role
			t.Run(string(dialect)+"/"+role.name, func(t *testing.T) {
				var policy string
				if role.params == "" {
					policy = t05PolicyPath(t, map[string]string{})
				} else {
					policy = t06A12Policy(t, role.params)
				}
				result := t06A12Audit(t, dialect, role.sql, policy)
				if len(result.Statements) != 1 {
					t.Fatalf("statements = %d, want 1", len(result.Statements))
				}
				statement := result.Statements[0]
				if statement.Index != 0 || statement.Kind != "ddl" ||
					statement.RawSQL != role.sql {
					t.Fatalf("statement identity = %+v, want ddl %q", statement, role.sql)
				}
				// rule_summary distinction: required=false keeps the rule
				// loaded but inapplicable; disabled never loads it.
				if role.wantLoaded < 0 {
					if result.RuleSummary != nil {
						t.Fatalf("rule summary = %+v, want absent for all-off", result.RuleSummary)
					}
				} else if result.RuleSummary == nil ||
					result.RuleSummary.Loaded != role.wantLoaded ||
					result.RuleSummary.Applicable != role.wantApplicable {
					t.Fatalf("rule summary = %+v, want loaded=%d applicable=%d",
						result.RuleSummary, role.wantLoaded, role.wantApplicable)
				}
				if len(statement.Findings) != role.wantFindings {
					t.Fatalf("role %s findings = %+v, want %d", role.name,
						statement.Findings, role.wantFindings)
				}
				if role.wantFindings == 0 {
					if result.Verdict != report.VerdictPass {
						t.Fatalf("role %s verdict = %s, want pass", role.name, result.Verdict)
					}
					return
				}
				finding := statement.Findings[0]
				wantMessage := `column "` + role.wantColumn + `" should be declared NOT NULL`
				if finding.RuleID != t06A12RuleID || finding.Level != "blocker" ||
					finding.Message != wantMessage {
					t.Fatalf("finding = %+v, want %s blocker %q", finding, t06A12RuleID, wantMessage)
				}
				if finding.Metadata["table"] != "t" ||
					finding.Metadata["column"] != role.wantColumn ||
					finding.Metadata["type"] != role.wantType {
					t.Fatalf("finding metadata = %+v, want table=t column=%s type=%q",
						finding.Metadata, role.wantColumn, role.wantType)
				}
				if result.Summary.Blockers != 1 || result.Verdict != report.VerdictReject {
					t.Fatalf("summary/verdict = %+v/%s, want 1/reject", result.Summary, result.Verdict)
				}
			})
		}
	}
}

// TestT06A12MixedStrictControl runs the same three-column input under the
// strict profile: the ordinary column and the time column each own one
// finding while the TEXT column stays exempt — a whole-entry comparison, not
// a per-rule count. The reordered control proves a leading exempt column
// never swallows a later ordinary column's attribution.
func TestT06A12MixedStrictControl(t *testing.T) {
	t.Parallel()
	strict := "      required: true\n      allow_time_null: false\n"
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect)+"/strict-mixed", func(t *testing.T) {
			result := t06A12Audit(t, dialect,
				"CREATE TABLE t (ordinary INT, time_c DATETIME NULL, payload TEXT);",
				t06A12Policy(t, strict))
			findings := result.Statements[0].Findings
			if len(findings) != 2 {
				t.Fatalf("findings = %+v, want ordinary+time_c", findings)
			}
			wants := []struct{ column, typ string }{
				{"ordinary", "int(11)"}, {"time_c", "datetime"},
			}
			for i, want := range wants {
				finding := findings[i]
				if finding.RuleID != t06A12RuleID || finding.Level != "blocker" ||
					finding.Message != `column "`+want.column+`" should be declared NOT NULL` ||
					finding.Metadata["column"] != want.column ||
					finding.Metadata["type"] != want.typ ||
					finding.Metadata["table"] != "t" {
					t.Fatalf("finding[%d] = %+v, want %s %s blocker", i, finding, want.column, want.typ)
				}
			}
		})
		t.Run(string(dialect)+"/reordered-strict", func(t *testing.T) {
			// Declaration order governs finding order: a leading time column
			// never swallows the ordinary column behind it.
			result := t06A12Audit(t, dialect,
				"CREATE TABLE t (time_c DATETIME NULL, ordinary INT);",
				t06A12Policy(t, strict))
			findings := result.Statements[0].Findings
			if len(findings) != 2 ||
				findings[0].Metadata["column"] != "time_c" ||
				findings[1].Metadata["column"] != "ordinary" {
				t.Fatalf("findings = %+v, want [time_c, ordinary]", findings)
			}
		})
		t.Run(string(dialect)+"/reordered-time-allow", func(t *testing.T) {
			result := t06A12Audit(t, dialect,
				"CREATE TABLE t (time_c DATETIME NULL, ordinary INT);",
				t06A12Policy(t, "      required: true\n      allow_time_null: true\n"))
			findings := result.Statements[0].Findings
			if len(findings) != 1 || findings[0].Metadata["column"] != "ordinary" {
				t.Fatalf("findings = %+v, want exactly one on ordinary", findings)
			}
		})
	}
}

// TestT06A12DerivedPreStateCarriesNullability pins that the derived create
// shape publishes each column's declared NotNull/Type and ordering into the
// follower statement's pre-state snapshot — for the nullable, not-null,
// time-exempt, time-required, and mixed inputs. A policy verdict never
// rewrites the declared fact: the exempted time column stays NotNull=false
// in the snapshot, and the rejected nullable column is not repaired. The
// provider is consulted exactly once for golden.t; the follower pre-state is
// a clone, never a provider re-read or an alias of the source DDL.
func TestT06A12DerivedPreStateCarriesNullability(t *testing.T) {
	t.Parallel()
	creates := map[string]struct {
		sql     string
		columns []struct {
			name    string
			typ     string
			notNull bool
		}
	}{
		"ordinary-nullable": {"CREATE TABLE t (c INT);", []struct {
			name    string
			typ     string
			notNull bool
		}{{"c", "int(11)", false}}},
		"ordinary-not-null": {"CREATE TABLE t (c INT NOT NULL);", []struct {
			name    string
			typ     string
			notNull bool
		}{{"c", "int(11)", true}}},
		"time-exempt": {"CREATE TABLE t (c DATETIME NULL);", []struct {
			name    string
			typ     string
			notNull bool
		}{{"c", "datetime", false}}},
		"time-required": {"CREATE TABLE t (c DATETIME NULL);", []struct {
			name    string
			typ     string
			notNull bool
		}{{"c", "datetime", false}}},
		"mixed-attribution": {"CREATE TABLE t (ordinary INT, time_c DATETIME NULL, payload TEXT);", []struct {
			name    string
			typ     string
			notNull bool
		}{{"ordinary", "int(11)", false}, {"time_c", "datetime", false}, {"payload", "text", false}}},
	}
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		for name, role := range creates {
			role := role
			t.Run(string(dialect)+"/"+name, func(t *testing.T) {
				ctx := context.Background()
				sql := role.sql + "\nALTER TABLE t ADD COLUMN next_c INT;"
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
				if len(snapshot.Columns) != len(role.columns) {
					t.Fatalf("pre-state columns = %#v, want %d", snapshot.Columns, len(role.columns))
				}
				for i, want := range role.columns {
					column := snapshot.Columns[i]
					if column.Name != want.name || column.Type != want.typ ||
						column.NotNull != want.notNull {
						t.Fatalf("pre-state column[%d] = %#v, want %s %s notNull=%v",
							i, column, want.name, want.typ, want.notNull)
					}
					if &snapshot.Columns[i] == &statements[0].DDL.Columns[i] {
						t.Fatalf("pre-state column[%d] shares statement storage — must be a clone", i)
					}
				}
				if snapshot.PrimaryKey != nil {
					t.Fatalf("pre-state pk = %#v, want none", snapshot.PrimaryKey)
				}
				// One provider read total for golden.t: the CREATE's own
				// existence check. The follower reads the derived clone.
				reads := 0
				for _, call := range provider.calls {
					if call == "golden.t" {
						reads++
					}
				}
				if reads != 1 {
					t.Fatalf("provider reads for golden.t = %d, want exactly 1", reads)
				}
				// The source declaration is never back-mutated by either the
				// policy verdict or the exemption path.
				for i, want := range role.columns {
					if statements[0].DDL.Columns[i].NotNull != want.notNull {
						t.Fatalf("source column[%d] NotNull mutated to %v, want %v",
							i, statements[0].DDL.Columns[i].NotNull, want.notNull)
					}
				}
			})
		}
	}
}

// TestT06A12PolicyVerdictDoesNotMutateSource pins the audit/immutability
// boundary on the real evaluation seam: the strict-policy registry rejects
// the extracted statement, yet the same spec columns the rule just read keep
// their declared NotNull=false — a verdict is never a rewrite of the audited
// declaration.
func TestT06A12PolicyVerdictDoesNotMutateSource(t *testing.T) {
	t.Parallel()
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect), func(t *testing.T) {
			ctx := context.Background()
			parsed, err := Parse(ctx, "CREATE TABLE t (c INT, d DATETIME NULL);", dialect)
			if err != nil {
				t.Fatal(err)
			}
			statements, err := Extract(ctx, parsed)
			if err != nil {
				t.Fatal(err)
			}
			if len(statements) != 1 || len(statements[0].DDL.Columns) != 2 {
				t.Fatalf("extracted = %+v, want one ddl with two columns", statements)
			}
			registry := rule.NewRegistry()
			if err := ddlrules.Register(registry, domainpolicy.Policy{
				Rules: map[string]domainpolicy.RulePolicy{
					t06A12RuleID: {Enabled: true, Level: rule.LevelBlocker,
						Params: map[string]any{"required": true, "allow_time_null": false}},
				},
			}); err != nil {
				t.Fatalf("register: %v", err)
			}
			result, err := EvaluateStatements(ctx, registry, statements)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			if result.Verdict != report.VerdictReject ||
				len(result.Statements[0].Findings) != 2 {
				t.Fatalf("verdict/findings = %s/%+v, want reject with c+d findings",
					result.Verdict, result.Statements[0].Findings)
			}
			// The policy rejected both columns as nullable, yet the declared
			// facts stay false — a reject is a verdict, never a rewrite.
			for _, column := range statements[0].DDL.Columns {
				if column.NotNull {
					t.Fatalf("column %s NotNull mutated to true by policy reject", column.Name)
				}
			}
		})
	}
}
