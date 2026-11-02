package audit

// T19 contract tests — TiDB TTL declaration lifecycle and automatic
// deletion risk. The public AuditSQL seam is the primary oracle; deeper
// ordered-state assertions live beside it in the same batch lifecycle.

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

const t19RuleTTLWarn = "ddl.tidb.ttl.warn"

func t19TTLShape(name string) *spec.TableSnapshot {
	return &spec.TableSnapshot{
		Exists: true,
		Table:  &spec.Table{Name: name},
		Columns: []spec.Column{
			t10r2Col("created_at", "datetime", false, 0),
		},
		Constraints:  []spec.Constraint{},
		ReferencedBy: []spec.Table{},
		TTLKnown:     true,
		Options: map[string]string{
			"engine":    "InnoDB",
			"charset":   "utf8mb4",
			"collation": "utf8mb4_bin",
		},
	}
}

// The #98 first path: a complete TiDB 8.5 TTL declaration is not an
// unsupported option and produces the dedicated automatic-deletion warning.
func TestT19AlterTTLGoldenPath(t *testing.T) {
	provider := &t10Provider{
		version: "8.0.11-TiDB-v8.5.0",
		tables: map[string]*spec.TableSnapshot{
			"t": t19TTLShape("t"),
		},
	}
	result, err := AuditSQL(t.Context(), Request{
		SQL:              "ALTER TABLE t TTL = created_at + INTERVAL 30 DAY;",
		Dialect:          spec.DialectTiDB,
		Schema:           "golden",
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected one statement, got %d", len(result.Statements))
	}
	stmt := result.Statements[0]
	if stmt.RawSQL == "" || stmt.Index != 0 {
		t.Fatalf("statement identity = index:%d raw:%q", stmt.Index, stmt.RawSQL)
	}
	if stmt.Coverage.Status != report.CoverageComplete {
		t.Fatalf("coverage = %+v, gaps = %#v, findings = %#v, want complete", stmt.Coverage, stmt.EvidenceGaps, stmt.Findings)
	}
	if len(result.Unsupported) != 0 {
		t.Fatalf("unsupported entries = %#v, want none", result.Unsupported)
	}
	warns := t05FindingsByRule(result, 0, t19RuleTTLWarn)
	if len(warns) != 1 || warns[0].Level != rule.LevelWarning {
		t.Fatalf("expected exactly one TTL warning, got findings=%#v", stmt.Findings)
	}
	if result.Verdict != report.VerdictReview {
		t.Fatalf("verdict = %s, want review", result.Verdict)
	}
}

// The warning consumes the same checkTTLInfoValid outcome the ordered state
// publishes: a proven landing warns once, a proven refusal stays silent, and
// an unproven landing owes a gap instead of a finding.
func TestT19TTLRuleParity(t *testing.T) {
	policy := t10Policy(t, t10Rule{id: t19RuleTTLWarn, level: "warning"})
	t.Run("create side", func(t *testing.T) {
		for _, tc := range []struct {
			name    string
			sql     string
			tables  map[string]*spec.TableSnapshot
			warns   int
			gapFact string
		}{
			{name: "applied warns once", sql: "CREATE TABLE ttl_ok (created_at DATETIME) TTL=created_at+INTERVAL 1 DAY", warns: 1},
			{name: "local temporary silent", sql: "CREATE TEMPORARY TABLE ttl_ok (created_at DATETIME) TTL=created_at+INTERVAL 1 DAY"},
			{name: "non temporal column silent", sql: "CREATE TABLE ttl_ok (label VARCHAR(32)) TTL=label+INTERVAL 1 DAY"},
			{name: "invalid interval silent", sql: "CREATE TABLE ttl_ok (created_at DATETIME) TTL=created_at+INTERVAL 'abc' DAY"},
			{name: "self reference silent", sql: "CREATE TABLE ttl_ok (id INT PRIMARY KEY, parent_id INT, created_at DATETIME, FOREIGN KEY (parent_id) REFERENCES ttl_ok(id)) TTL=created_at+INTERVAL 1 DAY"},
			{name: "unproven clustered mode owes gap", sql: "CREATE TABLE ttl_ok (amount FLOAT, created_at DATETIME, PRIMARY KEY(amount)) TTL=created_at+INTERVAL 1 DAY", gapFact: "target_table.ttl"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				provider := &t10Provider{version: "8.0.11-TiDB-v8.5.0", tables: tc.tables}
				result := t10Audit(t, spec.DialectTiDB, tc.sql, provider, policy)
				warns := t05FindingsByRule(result, 0, t19RuleTTLWarn)
				if len(warns) != tc.warns {
					t.Fatalf("warnings = %#v, want %d", warns, tc.warns)
				}
				gaps := t05GapsByRule(result, 0, t19RuleTTLWarn)
				if tc.gapFact == "" {
					if len(gaps) != 0 {
						t.Fatalf("proven outcome owes no gap, got %#v", gaps)
					}
					return
				}
				if len(gaps) != 1 || len(gaps[0].RequiredFacts) != 1 || gaps[0].RequiredFacts[0] != tc.gapFact {
					t.Fatalf("unproven outcome owes gap %q, got %#v", tc.gapFact, gaps)
				}
			})
		}
	})
	t.Run("alter side", func(t *testing.T) {
		inbound := t19TTLShape("t")
		inbound.ReferencedBy = []spec.Table{{Schema: "golden", Name: "child"}}
		unvouched := t19TTLShape("t")
		unvouched.ReferencedBy = nil
		unvouched.ReferencedByUnknown = true
		for _, tc := range []struct {
			name    string
			sql     string
			shape   *spec.TableSnapshot
			warns   int
			gapFact string
		}{
			{name: "proven inbound edge silent", sql: "ALTER TABLE t TTL = created_at + INTERVAL 1 DAY", shape: inbound},
			{name: "invalid interval silent", sql: "ALTER TABLE t TTL = created_at + INTERVAL 'abc' DAY", shape: t19TTLShape("t")},
			{name: "unvouched inbound set owes gap", sql: "ALTER TABLE t TTL = created_at + INTERVAL 1 DAY", shape: unvouched, gapFact: "target_table.ttl"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				provider := &t10Provider{version: "8.0.11-TiDB-v8.5.0", tables: map[string]*spec.TableSnapshot{"t": tc.shape}}
				result := t10Audit(t, spec.DialectTiDB, tc.sql, provider, policy)
				warns := t05FindingsByRule(result, 0, t19RuleTTLWarn)
				if len(warns) != tc.warns {
					t.Fatalf("warnings = %#v, want %d", warns, tc.warns)
				}
				gaps := t05GapsByRule(result, 0, t19RuleTTLWarn)
				if tc.gapFact == "" {
					if len(gaps) != 0 {
						t.Fatalf("proven refusal owes no gap, got %#v", gaps)
					}
					return
				}
				if len(gaps) != 1 || len(gaps[0].RequiredFacts) != 1 || gaps[0].RequiredFacts[0] != tc.gapFact {
					t.Fatalf("unproven outcome owes gap %q, got %#v", tc.gapFact, gaps)
				}
			})
		}
	})
}
