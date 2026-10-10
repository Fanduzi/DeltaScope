// Package audit verifies the T06-A10 AUTO_INCREMENT contract end to end.
// input: the eight frozen CREATE TABLE inputs under the isolated P_PK,
// P_INIT, and all-disabled policies plus create-then-alter batches
// output: independent rule attribution, exact finding identities, and the
// derived post-state carrying the column attribute and the declared table
// option as separate facts across statement boundaries
// pos: application audit regression for issue #85 T06-A10
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"fmt"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

const (
	t06A10PKRuleID   = "ddl.table.primary_key.auto_increment.require"
	t06A10InitRuleID = "ddl.table.auto_increment.init_value.require"
)

func t06A10PKPolicy(t *testing.T) string {
	t.Helper()
	return t05PolicyPath(t, map[string]string{
		t06A10PKRuleID: "      required: true\n",
	})
}

func t06A10InitPolicy(t *testing.T) string {
	t.Helper()
	return t05PolicyPath(t, map[string]string{
		t06A10InitRuleID: "      value: 8\n",
	})
}

// t06A10OffPolicy disables every default-policy rule — the P_OFF control.
func t06A10OffPolicy(t *testing.T) string {
	t.Helper()
	return t05PolicyPath(t, map[string]string{})
}

// t06A10StatementMatrix audits each fixed input through the real AuditSQL
// path under both dialects and pins the exact finding identity.
func TestT06A10StatementMatrix(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	type want struct {
		verdict  report.Verdict
		ruleID   string
		message  string
		metadata map[string]any
	}
	cases := []struct {
		name   string
		sql    string
		policy func(*testing.T) string
		want   want
	}{
		{name: "pk-auto", sql: "CREATE TABLE t (id BIGINT AUTO_INCREMENT PRIMARY KEY);",
			policy: t06A10PKPolicy, want: want{verdict: report.VerdictPass}},
		{name: "pk-no-auto", sql: "CREATE TABLE t (id BIGINT PRIMARY KEY);",
			policy: t06A10PKPolicy,
			want: want{verdict: report.VerdictReject,
				ruleID:  t06A10PKRuleID,
				message: `primary key column "id" must use auto_increment`,
				metadata: map[string]any{
					"table": "t", "column": "id", "type": "bigint(20)",
				}}},
		{name: "composite", sql: "CREATE TABLE t (id BIGINT NOT NULL, tenant_id INT NOT NULL, PRIMARY KEY (id, tenant_id));",
			policy: t06A10PKPolicy, want: want{verdict: report.VerdictPass}},
		{name: "pk-off", sql: "CREATE TABLE t (id BIGINT PRIMARY KEY);",
			policy: t06A10OffPolicy, want: want{verdict: report.VerdictPass}},
		{name: "init-match", sql: "CREATE TABLE t (id BIGINT AUTO_INCREMENT PRIMARY KEY) AUTO_INCREMENT=8;",
			policy: t06A10InitPolicy, want: want{verdict: report.VerdictPass}},
		{name: "init-mismatch", sql: "CREATE TABLE t (id BIGINT AUTO_INCREMENT PRIMARY KEY) AUTO_INCREMENT=9;",
			policy: t06A10InitPolicy,
			want: want{verdict: report.VerdictReject,
				ruleID:  t06A10InitRuleID,
				message: "table auto_increment init value must be 8",
				metadata: map[string]any{
					"table": "t", "required_value": 8, "actual_value": 9,
				}}},
		{name: "init-omitted", sql: "CREATE TABLE t (id BIGINT AUTO_INCREMENT PRIMARY KEY);",
			policy: t06A10InitPolicy, want: want{verdict: report.VerdictPass}},
		{name: "init-off", sql: "CREATE TABLE t (id BIGINT AUTO_INCREMENT PRIMARY KEY) AUTO_INCREMENT=9;",
			policy: t06A10OffPolicy, want: want{verdict: report.VerdictPass}},
	}

	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		for _, tc := range cases {
			tc := tc
			t.Run(fmt.Sprintf("%s/%s", dialect, tc.name), func(t *testing.T) {
				t.Parallel()
				result, err := AuditSQL(ctx, Request{
					SQL:        tc.sql,
					Dialect:    dialect,
					Schema:     "golden",
					ConfigPath: tc.policy(t),
				})
				if err != nil {
					t.Fatalf("audit: %v", err)
				}
				if len(result.Statements) != 1 {
					t.Fatalf("statements = %d, want 1", len(result.Statements))
				}
				statement := result.Statements[0]
				if statement.Index != 0 || statement.Kind != "ddl" || statement.RawSQL != tc.sql {
					t.Fatalf("statement identity = %+v", statement)
				}
				if statement.Coverage.Status != report.CoverageComplete ||
					len(statement.EvidenceGaps) != 0 {
					t.Fatalf("coverage/gaps = %+v / %+v", statement.Coverage, statement.EvidenceGaps)
				}
				if result.Verdict != tc.want.verdict {
					t.Fatalf("verdict = %s, want %s", result.Verdict, tc.want.verdict)
				}
				if len(result.Unsupported) != 0 || len(result.Diagnostics) != 0 || len(result.GlobalFindings) != 0 {
					t.Fatalf("unexpected channels: unsupported=%+v diagnostics=%+v global=%+v",
						result.Unsupported, result.Diagnostics, result.GlobalFindings)
				}
				if tc.want.ruleID == "" {
					if len(statement.Findings) != 0 {
						t.Fatalf("findings = %+v, want none", statement.Findings)
					}
					if result.Summary.Blockers != 0 {
						t.Fatalf("blockers = %d, want 0", result.Summary.Blockers)
					}
					return
				}
				if len(statement.Findings) != 1 {
					t.Fatalf("findings = %+v, want exactly one", statement.Findings)
				}
				finding := statement.Findings[0]
				if finding.RuleID != tc.want.ruleID || finding.Level != rule.LevelBlocker ||
					finding.Message != tc.want.message {
					t.Fatalf("finding identity = %+v", finding)
				}
				for key, want := range tc.want.metadata {
					if finding.Metadata[key] != want {
						t.Fatalf("metadata[%q] = %v, want %v (full %#v)", key, finding.Metadata[key], want, finding.Metadata)
					}
				}
				if result.Summary.Blockers != 1 {
					t.Fatalf("blockers = %d, want 1", result.Summary.Blockers)
				}
			})
		}
	}
}

// TestT06A10CompositeSkipsSingleMemberRule proves the composite input keeps
// two bound PK members with no column-level AUTO_INCREMENT — the rule sees a
// member count of two and skips, which is not "the shape satisfies the policy".
func TestT06A10CompositeSkipsSingleMemberRule(t *testing.T) {
	t.Parallel()
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			provider := &t05AbsentProvider{}
			enriched := enrichA3(t,
				"CREATE TABLE t (id BIGINT NOT NULL, tenant_id INT NOT NULL, PRIMARY KEY (id, tenant_id));",
				dialect, provider)
			if len(enriched) != 1 || enriched[0].DDL == nil {
				t.Fatalf("enriched = %+v", enriched)
			}
			ddl := enriched[0].DDL
			if ddl.PrimaryKey == nil || len(ddl.PrimaryKey.Columns) != 2 ||
				ddl.PrimaryKey.Columns[0] != "id" || ddl.PrimaryKey.Columns[1] != "tenant_id" {
				t.Fatalf("pk members = %#v, want ordered [id tenant_id]", ddl.PrimaryKey)
			}
			for _, column := range ddl.Columns {
				if column.AutoIncrement {
					t.Fatalf("column %s unexpectedly marked auto_increment", column.Name)
				}
			}
			result, err := AuditSQL(context.Background(), Request{
				SQL:        "CREATE TABLE t (id BIGINT NOT NULL, tenant_id INT NOT NULL, PRIMARY KEY (id, tenant_id));",
				Dialect:    dialect,
				Schema:     "golden",
				ConfigPath: t06A10PKPolicy(t),
			})
			if err != nil {
				t.Fatalf("audit: %v", err)
			}
			if result.Verdict != report.VerdictPass || len(result.Statements[0].Findings) != 0 {
				t.Fatalf("composite verdict = %s findings = %+v, want pass with none — member-count skip",
					result.Verdict, result.Statements[0].Findings)
			}
		})
	}
}

// TestT06A10DerivedPostStateCarriesDeclarationFacts proves the CREATE-derived
// post-state carries Column.AutoIncrement and Options["auto_increment"] as
// separate declared facts into the next statement's pre-state — a provider
// read happens exactly once and only confirms the table's initial absence.
func TestT06A10DerivedPostStateCarriesDeclarationFacts(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		create     string
		wantOption bool
		wantValue  string
	}{
		{name: "pk-auto", create: "CREATE TABLE t (id BIGINT AUTO_INCREMENT PRIMARY KEY)",
			wantOption: false},
		{name: "init-match", create: "CREATE TABLE t (id BIGINT AUTO_INCREMENT PRIMARY KEY) AUTO_INCREMENT=8",
			wantOption: true, wantValue: "8"},
		{name: "init-mismatch", create: "CREATE TABLE t (id BIGINT AUTO_INCREMENT PRIMARY KEY) AUTO_INCREMENT=9",
			wantOption: true, wantValue: "9"},
	}
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		for _, tc := range cases {
			tc := tc
			t.Run(fmt.Sprintf("%s/%s", dialect, tc.name), func(t *testing.T) {
				t.Parallel()
				provider := &t05AbsentProvider{}
				enriched := enrichA3(t, tc.create+"; ALTER TABLE t ADD COLUMN note INT;", dialect, provider)
				if len(enriched) != 2 {
					t.Fatalf("enriched statements = %d, want 2", len(enriched))
				}
				// The source DDL is not rewritten by enrichment.
				if enriched[0].DDL == nil || len(enriched[0].DDL.Columns) != 1 ||
					!enriched[0].DDL.Columns[0].AutoIncrement {
					t.Fatalf("source ddl columns = %+v, want id with auto_increment", enriched[0].DDL)
				}
				post := enriched[1].Metadata.TargetTable
				if post == nil || !post.Exists {
					t.Fatalf("post-state = %+v, want existing derived table", post)
				}
				column := post.FindColumn("id")
				if column == nil || !column.AutoIncrement {
					t.Fatalf("post-state id = %+v, want auto_increment=true", column)
				}
				got, present := post.Options["auto_increment"]
				if present != tc.wantOption {
					t.Fatalf("post Options = %#v, auto_increment present=%v want %v", post.Options, present, tc.wantOption)
				}
				if tc.wantOption && got != tc.wantValue {
					t.Fatalf("post Options[auto_increment] = %q, want declared %q", got, tc.wantValue)
				}
				// The post-state option records the *declaration*, even when the
				// init policy would reject it (init-mismatch's 9) — it is not an
				// observed catalog allocator value.
				if len(provider.calls) != 1 || provider.calls[0] != "golden.t" {
					t.Fatalf("provider read ledger = %#v, want exactly [golden.t]", provider.calls)
				}
			})
		}
	}
}
