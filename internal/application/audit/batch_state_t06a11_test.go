// Package audit verifies the T06-A11 single-member primary-key BIGINT/UNSIGNED
// audit and derived-state boundary.
package audit

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

const (
	t06A11BigintRuleID   = "ddl.table.primary_key.bigint.require"
	t06A11UnsignedRuleID = "ddl.table.primary_key.unsigned.require"
)

func t06A11IsolatedPolicy(t *testing.T, ruleID string, enabled bool) string {
	t.Helper()
	if !enabled {
		return t05PolicyPath(t, map[string]string{})
	}
	return t05PolicyPath(t, map[string]string{ruleID: "      required: true\n"})
}

func t06A11Audit(t *testing.T, dialect spec.Dialect, sql, policy string) report.Result {
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

// TestT06A11OfflineRoleMatrix runs the eight frozen roles through AuditSQL
// under both dialects and pins the complete product contract: verdict, single
// finding identity (rule_id, level, message, bound metadata), composite-skip
// semantics, and the distinct disabled control.
func TestT06A11OfflineRoleMatrix(t *testing.T) {
	t.Parallel()
	roles := []struct {
		name     string
		sql      string
		ruleID   string
		rejects  bool
		wantType string
		wantMsg  string
	}{
		{"bigint-pass", "CREATE TABLE t (id BIGINT PRIMARY KEY);",
			t06A11BigintRuleID, false, "", ""},
		{"bigint-reject", "CREATE TABLE t (id INT PRIMARY KEY);",
			t06A11BigintRuleID, true, "int(11)", `primary key column "id" must use bigint`},
		{"bigint-composite-skip", "CREATE TABLE t (id INT NOT NULL, tenant_id INT NOT NULL, PRIMARY KEY (id, tenant_id));",
			t06A11BigintRuleID, false, "", ""},
		{"bigint-off", "CREATE TABLE t (id INT PRIMARY KEY);",
			"", false, "", ""},
		{"unsigned-pass", "CREATE TABLE t (id BIGINT UNSIGNED PRIMARY KEY);",
			t06A11UnsignedRuleID, false, "", ""},
		{"unsigned-reject", "CREATE TABLE t (id BIGINT PRIMARY KEY);",
			t06A11UnsignedRuleID, true, "bigint(20)", `primary key column "id" must be unsigned`},
		{"unsigned-composite-skip", "CREATE TABLE t (id BIGINT NOT NULL, tenant_id INT NOT NULL, PRIMARY KEY (id, tenant_id));",
			t06A11UnsignedRuleID, false, "", ""},
		{"unsigned-off", "CREATE TABLE t (id BIGINT PRIMARY KEY);",
			"", false, "", ""},
	}
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		for _, role := range roles {
			role := role
			t.Run(string(dialect)+"/"+role.name, func(t *testing.T) {
				var policy string
				if role.ruleID == "" {
					policy = t06A11IsolatedPolicy(t, "", false)
				} else {
					policy = t06A11IsolatedPolicy(t, role.ruleID, true)
				}
				result := t06A11Audit(t, dialect, role.sql, policy)
				if len(result.Statements) != 1 {
					t.Fatalf("statements = %d, want 1", len(result.Statements))
				}
				statement := result.Statements[0]
				if statement.Index != 0 || statement.Kind != "ddl" ||
					statement.RawSQL != role.sql {
					t.Fatalf("statement identity = %+v, want ddl %q", statement, role.sql)
				}
				if len(statement.Findings) != 0 && role.ruleID == "" {
					t.Fatalf("disabled control produced findings = %+v", statement.Findings)
				}
				if !role.rejects {
					if len(statement.Findings) != 0 {
						t.Fatalf("role %s findings = %+v, want none", role.name, statement.Findings)
					}
					return
				}
				if len(statement.Findings) != 1 {
					t.Fatalf("role %s findings = %+v, want exactly one", role.name, statement.Findings)
				}
				finding := statement.Findings[0]
				if finding.RuleID != role.ruleID || finding.Level != "blocker" ||
					finding.Message != role.wantMsg {
					t.Fatalf("finding = %+v, want %s blocker %q", finding, role.ruleID, role.wantMsg)
				}
				if finding.Metadata["table"] != "t" || finding.Metadata["column"] != "id" ||
					finding.Metadata["type"] != role.wantType {
					t.Fatalf("finding metadata = %+v, want table=t column=id type=%q",
						finding.Metadata, role.wantType)
				}
				if result.Summary.Blockers != 1 || result.Verdict != report.VerdictReject {
					t.Fatalf("summary/verdict = %+v/%s, want 1/reject", result.Summary, result.Verdict)
				}
			})
		}
	}
}

// TestT06A11DerivedPreStateCarriesTypeAndUnsigned pins that the derived
// create shape publishes the bound member's Type/Unsigned/Length and the PK
// member list into the follower statement's pre-state snapshot — for the
// unsigned-pass and bigint-pass inputs, and for the bigint-reject input whose
// policy verdict stays a reject without rewriting the declared signed fact.
// The provider is consulted exactly once for golden.t (the CREATE's own
// existence check); the follower's pre-state is a clone, never a provider
// re-read or a mutation of the source DDL.
func TestT06A11DerivedPreStateCarriesTypeAndUnsigned(t *testing.T) {
	t.Parallel()
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		for name, create := range map[string]string{
			"bigint-pass":   "CREATE TABLE t (id BIGINT PRIMARY KEY);",
			"unsigned-pass": "CREATE TABLE t (id BIGINT UNSIGNED PRIMARY KEY);",
			"bigint-reject": "CREATE TABLE t (id INT PRIMARY KEY);",
		} {
			t.Run(string(dialect)+"/"+name, func(t *testing.T) {
				ctx := context.Background()
				sql := create + "\nALTER TABLE t ADD COLUMN note INT;"
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
				if len(snapshot.Columns) != 1 {
					t.Fatalf("pre-state columns = %#v, want one derived column", snapshot.Columns)
				}
				column := snapshot.Columns[0]
				wantType := map[string]string{
					"bigint-pass":   "bigint(20)",
					"unsigned-pass": "bigint(20) unsigned",
					"bigint-reject": "int(11)",
				}[name]
				if column.Name != "id" || column.Type != wantType || column.Length != -1 ||
					column.Unsigned != (name == "unsigned-pass") {
					t.Fatalf("pre-state column = %#v, want id type=%q length=-1 unsigned=%v",
						column, wantType, name == "unsigned-pass")
				}
				if snapshot.PrimaryKey == nil ||
					len(snapshot.PrimaryKey.Columns) != 1 ||
					snapshot.PrimaryKey.Columns[0] != "id" {
					t.Fatalf("pre-state pk = %#v, want single member id", snapshot.PrimaryKey)
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
				// The derived column clone must not alias or back-mutate the
				// source statement's declared column.
				if &snapshot.Columns[0] == &statements[0].DDL.Columns[0] {
					t.Fatal("pre-state shares the statement's column storage — it must be a clone")
				}
				if statements[0].DDL.Columns[0].Type != wantType {
					t.Fatalf("source column mutated to %q, want declared %q",
						statements[0].DDL.Columns[0].Type, wantType)
				}
			})
		}
	}
}
