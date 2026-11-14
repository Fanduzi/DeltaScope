package audit

// T19 contract tests — TiDB TTL declaration lifecycle and automatic
// deletion risk. The public AuditSQL seam is the primary oracle; deeper
// ordered-state assertions live beside it in the same batch lifecycle.

import (
	"errors"
	"strings"
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
		Options: map[string]string{"cache_status": "disable", "engine": "InnoDB",
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

// MySQL has no /*T! feature-comment syntax, so a /*T![ttl] clause is an
// authored-but-inert table option: the audit surface must keep the clause
// visible as a vendor boundary instead of blanking it into a silent pass
// (issue #98 manifest case mysql-comment-noop).
func TestT19MySQLTTLFeatureCommentVendorBoundary(t *testing.T) {
	audit := func(t *testing.T, sql string, dialect spec.Dialect) report.Result {
		t.Helper()
		result, err := AuditSQL(t.Context(), Request{
			SQL:     sql,
			Dialect: dialect,
			Schema:  "golden",
		})
		if err != nil && !errors.Is(err, ErrUnsupportedStatement) {
			t.Fatalf("audit: %v", err)
		}
		return result
	}
	boundary := func(result report.Result) []spec.UnsupportedDetail {
		var hits []spec.UnsupportedDetail
		for _, entry := range result.Unsupported {
			if strings.Contains(entry.Feature, ".ttl.") {
				hits = append(hits, entry)
			}
		}
		return hits
	}

	t.Run("alter feature comment stays a vendor boundary", func(t *testing.T) {
		result := audit(t, "ALTER TABLE t /*T![ttl] TTL_ENABLE='ON' */;", spec.DialectMySQL)
		hits := boundary(result)
		if len(hits) != 1 || hits[0].Feature != "alter_table.table_option.ttl.ttl_enable" ||
			hits[0].Reason != spec.UnsupportedVendorBoundaryReason {
			t.Fatalf("expected the ttl_enable vendor boundary, got %#v", result.Unsupported)
		}
		if result.Statements[0].Coverage.Status != report.CoverageIncomplete {
			t.Fatalf("coverage = %+v, want incomplete", result.Statements[0].Coverage)
		}
	})

	t.Run("tidb feature comment stays executable evidence", func(t *testing.T) {
		result := audit(t, "ALTER TABLE t /*T![ttl] TTL_ENABLE='ON' */;", spec.DialectTiDB)
		if hits := boundary(result); len(hits) != 0 {
			t.Fatalf("TiDB feature comment must not vendor-refuse, got %#v", hits)
		}
	})

	t.Run("placement comment still ignored under mysql", func(t *testing.T) {
		result := audit(t, "ALTER TABLE t /*T![placement] PLACEMENT POLICY=p */;", spec.DialectMySQL)
		if hits := boundary(result); len(hits) != 0 {
			t.Fatalf("placement comment must stay silent under MySQL, got %#v", hits)
		}
	})

	t.Run("ordinary comment never fabricates a ttl member", func(t *testing.T) {
		result := audit(t, "ALTER TABLE t /* ttl_enable='ON' */;", spec.DialectMySQL)
		if hits := boundary(result); len(hits) != 0 {
			t.Fatalf("ordinary comment must not vendor-refuse, got %#v", hits)
		}
	})
}

// The #98 R4 blocker: when the bound pre-state proves the target exists
// without a TTL config, a standalone TTL_ENABLE / TTL_JOB_INTERVAL member is
// a proven TiDB refusal (native ERROR 8150) — a bounded vendor boundary, never
// a silent pass and never an inferred outcome on unvouched state.
func TestT19AlterTTLNoTTLConfigRefusal(t *testing.T) {
	boundary := func(result report.Result, index int) *spec.UnsupportedDetail {
		t.Helper()
		for i := range result.Unsupported {
			entry := &result.Unsupported[i]
			if entry.Index != index {
				continue
			}
			if entry.Reason == spec.UnsupportedVendorBoundaryReason &&
				entry.Metadata["aspect"] == "ttl" && entry.Metadata["refusal"] == "no_ttl_config" {
				return entry
			}
		}
		return nil
	}
	policy := t10Policy(t, t10Rule{id: t19RuleTTLWarn, level: "warning"})
	noTTL := t19TTLShape("t")

	t.Run("metadata proven no TTL refuses each member form", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			sql  string
			feat string
		}{
			{name: "enable off", sql: "ALTER TABLE t TTL_ENABLE='OFF'", feat: "alter_table.table_option.ttl.ttl_enable"},
			{name: "enable on", sql: "ALTER TABLE t TTL_ENABLE='ON'", feat: "alter_table.table_option.ttl.ttl_enable"},
			{name: "job interval", sql: "ALTER TABLE t TTL_JOB_INTERVAL='2h'", feat: "alter_table.table_option.ttl.ttl_job_interval"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				provider := &t10Provider{version: "8.0.11-TiDB-v8.5.0", tables: map[string]*spec.TableSnapshot{"t": noTTL}}
				result := t10Audit(t, spec.DialectTiDB, tc.sql, provider, policy)
				entry := boundary(result, 0)
				if entry == nil {
					t.Fatalf("expected no_ttl_config vendor boundary, got unsupported=%#v", result.Unsupported)
				}
				if entry.Feature != tc.feat {
					t.Fatalf("feature = %q, want %q", entry.Feature, tc.feat)
				}
				if result.Statements[0].Coverage.Status != report.CoverageIncomplete || result.Verdict != report.VerdictReview {
					t.Fatalf("coverage=%s verdict=%s, want incomplete/review", result.Statements[0].Coverage.Status, result.Verdict)
				}
				if warns := t05FindingsByRule(result, 0, t19RuleTTLWarn); len(warns) != 0 {
					t.Fatalf("refused ALTER must not fabricate findings, got %#v", warns)
				}
			})
		}
	})

	t.Run("unknown prior keeps evidence gap without boundary", func(t *testing.T) {
		unknown := t19TTLShape("t")
		unknown.TTLKnown = false
		provider := &t10Provider{version: "8.0.11-TiDB-v8.5.0", tables: map[string]*spec.TableSnapshot{"t": unknown}}
		result := t10Audit(t, spec.DialectTiDB, "ALTER TABLE t TTL_ENABLE='OFF'", provider, policy)
		if entry := boundary(result, 0); entry != nil {
			t.Fatalf("unknown prior must not infer refusal, got %#v", entry)
		}
		if gaps := t05GapsByRule(result, 0, t19RuleTTLWarn); len(gaps) == 0 {
			t.Fatalf("unknown prior owes an evidence gap, got none")
		}
	})

	t.Run("TTL-present controls keep R3 semantics", func(t *testing.T) {
		on := true
		off := false
		for _, tc := range []struct {
			name    string
			enabled *bool
			sql     string
			warns   int
		}{
			{name: "on to off silent", enabled: &on, sql: "ALTER TABLE t TTL_ENABLE='OFF'"},
			{name: "off to on warns", enabled: &off, sql: "ALTER TABLE t TTL_ENABLE='ON'", warns: 1},
			{name: "job interval on warns", enabled: &on, sql: "ALTER TABLE t TTL_JOB_INTERVAL='2h'", warns: 1},
			{name: "job interval off silent", enabled: &off, sql: "ALTER TABLE t TTL_JOB_INTERVAL='2h'"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				shape := t19TTLShape("t")
				shape.TTL = &spec.TTLConfig{TimeColumn: "created_at", IntervalValue: "1", IntervalUnit: "day", Enabled: tc.enabled, JobInterval: "1h"}
				provider := &t10Provider{version: "8.0.11-TiDB-v8.5.0", tables: map[string]*spec.TableSnapshot{"t": shape}}
				result := t10Audit(t, spec.DialectTiDB, tc.sql, provider, policy)
				if entry := boundary(result, 0); entry != nil {
					t.Fatalf("TTL-present table must not refuse, got %#v", entry)
				}
				if warns := t05FindingsByRule(result, 0, t19RuleTTLWarn); len(warns) != tc.warns {
					t.Fatalf("warnings = %#v, want %d", warns, tc.warns)
				}
			})
		}
	})

	t.Run("prospective state inherits the refusal premise", func(t *testing.T) {
		provider := &t10Provider{version: "8.0.11-TiDB-v8.5.0"}
		for _, tc := range []struct {
			name    string
			sql     string
			index   int
			refused bool
		}{
			{name: "create plain then enable", sql: "CREATE TABLE t (id INT PRIMARY KEY, created_at DATETIME); ALTER TABLE t TTL_ENABLE='ON'", index: 1, refused: true},
			{name: "create ttl then enable", sql: "CREATE TABLE t (id INT PRIMARY KEY, created_at DATETIME) TTL=created_at+INTERVAL 1 DAY; ALTER TABLE t TTL_ENABLE='OFF'", index: 1},
		} {
			t.Run(tc.name, func(t *testing.T) {
				result := t10Audit(t, spec.DialectTiDB, tc.sql, provider, policy)
				entry := boundary(result, tc.index)
				if tc.refused && entry == nil {
					t.Fatalf("statement %d must refuse on proven no TTL, got %#v", tc.index, result.Unsupported)
				}
				if !tc.refused && entry != nil {
					t.Fatalf("statement %d must not refuse, got %#v", tc.index, entry)
				}
			})
		}
	})

	t.Run("remove then enable refuses on inherited no TTL", func(t *testing.T) {
		on := true
		shape := t19TTLShape("t")
		shape.TTL = &spec.TTLConfig{TimeColumn: "created_at", IntervalValue: "1", IntervalUnit: "day", Enabled: &on, JobInterval: "1h"}
		provider := &t10Provider{version: "8.0.11-TiDB-v8.5.0", tables: map[string]*spec.TableSnapshot{"t": shape}}
		result := t10Audit(t, spec.DialectTiDB, "ALTER TABLE t REMOVE TTL; ALTER TABLE t TTL_ENABLE='OFF'", provider, policy)
		if entry := boundary(result, 1); entry == nil {
			t.Fatalf("REMOVE-then-ENABLE must refuse on proven no TTL, got %#v", result.Unsupported)
		}
	})

	t.Run("refused ALTER leaves successor state unpolluted", func(t *testing.T) {
		provider := &t10Provider{version: "8.0.11-TiDB-v8.5.0"}
		_, statements := t17Enrich(t, spec.DialectTiDB,
			"CREATE TABLE t (id INT PRIMARY KEY, created_at DATETIME); ALTER TABLE t TTL_ENABLE='ON'; ALTER TABLE t ADD COLUMN note INT;",
			"golden", &spec.VersionIdentity{Product: spec.VersionProductTiDB, Version: "8.5.0", Major: 8, Minor: 5, Patch: 0, ValidatedRange: true}, provider)
		if len(statements) != 3 || len(statements[2].Metadata.TargetTables) != 1 {
			t.Fatalf("successor bindings = %#v", statements[2].Metadata)
		}
		snapshot := statements[2].Metadata.TargetTables[0].Snapshot
		if snapshot == nil || !snapshot.TTLKnown || snapshot.TTL != nil {
			t.Fatalf("successor must keep proven no TTL, got %#v", snapshot)
		}
	})
}
