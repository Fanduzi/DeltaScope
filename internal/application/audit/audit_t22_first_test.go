package audit

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// audit_t22_first_test.go pins the T22/#101 first-path public contract:
// TiDB RU resource-group CREATE/ALTER/DROP reach kind=ddl with audited
// semantics — never unknown-kind, never generic-notice-only — while MySQL
// keeps its vendor boundary for TiDB RU syntax. Pre-implementation receipt:
// the same input produced kind=unknown plus unsupported
// create/alter_resource_group entries (docs/verification/t22/).

const (
	t22ValidRule     = "ddl.tidb.resource_group.valid"
	t22BurstableRule = "ddl.tidb.resource_group.burstable.warn"
	t22RunawayRule   = "ddl.tidb.resource_group.runaway.warn"

	t22CapabilityGap  = "unknown_resource_group_capability"
	t22StateGap       = "unknown_resource_group_state"
	t22CapabilityFact = "tidb_enable_resource_control"
)

func t22Audit(t *testing.T, sql string, dialect spec.Dialect, provider MetadataProvider, policyPath string) (report.Result, error) {
	t.Helper()
	return AuditSQL(context.Background(), Request{
		SQL:              sql,
		Dialect:          dialect,
		TargetVersion:    "8.5.0",
		ConfigPath:       policyPath,
		MetadataProvider: provider,
	})
}

func t22MustAudit(t *testing.T, sql string, dialect spec.Dialect, provider MetadataProvider, policyPath string) report.Result {
	t.Helper()
	result, err := t22Audit(t, sql, dialect, provider, policyPath)
	if err != nil {
		t.Fatalf("AuditSQL(%q): %v", sql, err)
	}
	return result
}

func t22GapCodes(stmt report.StatementResult) []string {
	var codes []string
	for _, gap := range stmt.EvidenceGaps {
		codes = append(codes, gap.ReasonCode)
	}
	return codes
}

func t22Findings(result report.Result, stmt int, id string) []rule.Finding {
	var out []rule.Finding
	for _, finding := range result.Statements[stmt].Findings {
		if finding.RuleID == id {
			out = append(out, finding)
		}
	}
	return out
}

func t22Policy(t *testing.T, overrides map[string]policy.RulePolicy) string {
	t.Helper()
	return writeIsolatedPolicy(t, overrides)
}

// First Golden Path: both statements classify as ddl and the bounded
// capability gap (not an unsupported marker, not silence) records the only
// missing premise offline. The authored 100→200 fidelity is pinned in the
// extractor seam tests.
func TestT22FirstPathRUChange(t *testing.T) {
	result := t22MustAudit(t,
		"CREATE RESOURCE GROUP golden_rg RU_PER_SEC = 100;\nALTER RESOURCE GROUP golden_rg RU_PER_SEC = 200;",
		spec.DialectTiDB, nil, "")
	if len(result.Statements) != 2 {
		t.Fatalf("statements = %d, want 2", len(result.Statements))
	}
	for i, stmt := range result.Statements {
		if stmt.Kind != string(spec.KindDDL) {
			t.Fatalf("statement %d kind = %q, want ddl", i, stmt.Kind)
		}
		if !slices.Contains(t22GapCodes(stmt), t22CapabilityGap) {
			t.Fatalf("statement %d gaps = %v, want %s offline", i, t22GapCodes(stmt), t22CapabilityGap)
		}
	}
	for _, unsupported := range result.Unsupported {
		if unsupported.Feature == "create_resource_group" || unsupported.Feature == "alter_resource_group" {
			t.Fatalf("unexpected unsupported entry: %+v", unsupported)
		}
	}
}

// MySQL must not accept the TiDB RU syntax as a natively supported
// operation: the same input is a vendor boundary, never a silent audited
// create, and never a MySQL CPU field projection.
func TestT22MySQLDialectKeepsVendorBoundary(t *testing.T) {
	result, err := t22Audit(t, "CREATE RESOURCE GROUP golden_rg RU_PER_SEC = 100;", spec.DialectMySQL, nil, "")
	if !errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("err = %v, want ErrUnsupportedStatement for MySQL RU syntax", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("statements = %d", len(result.Statements))
	}
	if len(result.Unsupported) != 1 {
		t.Fatalf("unsupported = %+v, want one vendor boundary", result.Unsupported)
	}
	entry := result.Unsupported[0]
	if entry.Feature != "create_resource_group" || entry.Metadata["boundary"] != "vendor" {
		t.Fatalf("unsupported = %+v, want vendor boundary create_resource_group", entry)
	}
	if result.Statements[0].Coverage.Status == report.CoverageComplete {
		t.Fatal("vendor-boundary statement reported complete coverage")
	}
}

// A safe declaration produces no risk findings; the bounded capability
// gap is the only missing-fact disclosure.
func TestT22SafeDeclarationNoWarning(t *testing.T) {
	result := t22MustAudit(t, "CREATE RESOURCE GROUP safe_rg RU_PER_SEC = 500;", spec.DialectTiDB, nil, "")
	stmt := result.Statements[0]
	for _, finding := range stmt.Findings {
		if finding.RuleID == t22BurstableRule || finding.RuleID == t22RunawayRule {
			t.Fatalf("unexpected risk finding on safe create: %+v", finding)
		}
	}
}

// BURSTABLE is the contracted risky change: the declaration names the
// burst allowance and the rule reports it — never a performance claim.
func TestT22BurstableWarns(t *testing.T) {
	result := t22MustAudit(t, "CREATE RESOURCE GROUP burst_rg RU_PER_SEC = 100 BURSTABLE;", spec.DialectTiDB, nil, "")
	if got := t22Findings(result, 0, t22BurstableRule); len(got) != 1 {
		t.Fatalf("findings = %+v, want one burstable warning", result.Statements[0].Findings)
	}
}

// QUERY_LIMIT ACTION=KILL is the contracted runaway-risk shape.
func TestT22RunawayKillWarns(t *testing.T) {
	result := t22MustAudit(t,
		"CREATE RESOURCE GROUP rg1 RU_PER_SEC = 100 QUERY_LIMIT=(EXEC_ELAPSED='60s', ACTION=KILL);",
		spec.DialectTiDB, nil, "")
	if got := t22Findings(result, 0, t22RunawayRule); len(got) != 1 {
		t.Fatalf("findings = %+v, want one runaway warning", result.Statements[0].Findings)
	}
}

// An unproven target yields the bounded state gap — never a fabricated
// absent/present answer and never a fabricated blocker.
func TestT22UnknownTargetGapsNotBlocks(t *testing.T) {
	result := t22MustAudit(t, "ALTER RESOURCE GROUP ghost_rg RU_PER_SEC = 10;", spec.DialectTiDB, nil, "")
	stmt := result.Statements[0]
	if !slices.Contains(t22GapCodes(stmt), t22StateGap) {
		t.Fatalf("gaps = %v, want %s", t22GapCodes(stmt), t22StateGap)
	}
	if got := t22Findings(result, 0, t22ValidRule); len(got) != 0 {
		t.Fatalf("unexpected valid blocker on unproven target: %+v", got)
	}
}

// The reserved default group cannot be dropped — a proven refusal even
// with no catalog read.
func TestT22ReservedDefaultDropRefused(t *testing.T) {
	result := t22MustAudit(t, "DROP RESOURCE GROUP `default`;", spec.DialectTiDB, nil, "")
	if got := t22Findings(result, 0, t22ValidRule); len(got) != 1 {
		t.Fatalf("findings = %+v, want one reserved-drop blocker", result.Statements[0].Findings)
	}
}

// Invalid input keeps the parser-error surface.
func TestT22InvalidStaysParserError(t *testing.T) {
	if _, err := t22Audit(t, "CREATE RESOURCE GROUP ( RU_PER_SEC = 1;", spec.DialectTiDB, nil, ""); err == nil {
		t.Fatal("expected parser error for invalid resource group SQL")
	}
}

// Disabling the rules removes findings and gaps alike — the ordered
// binding itself is unaffected (next statement still adjudicates).
func TestT22RulesDisabledEmitNoGaps(t *testing.T) {
	policyPath := t22Policy(t, map[string]policy.RulePolicy{
		t22ValidRule:     {Enabled: false},
		t22BurstableRule: {Enabled: false},
		t22RunawayRule:   {Enabled: false},
	})
	result := t22MustAudit(t, "CREATE RESOURCE GROUP rg1 RU_PER_SEC = 100 BURSTABLE;", spec.DialectTiDB, nil, policyPath)
	stmt := result.Statements[0]
	if len(stmt.Findings) != 0 {
		t.Fatalf("findings = %+v, want none with rules disabled", stmt.Findings)
	}
	if len(stmt.EvidenceGaps) != 0 {
		t.Fatalf("gaps = %+v, want none with rules disabled", stmt.EvidenceGaps)
	}
}

// A non-empty QUERY_LIMIT list missing ACTION or every threshold member
// is refused at run time by fixed TiDB v8.5.0 (ERROR 1105, issue #101 T22
// native probe). Until that completeness refusal is modeled explicitly the
// option stays bounded unmodeled evidence — unsupported aspect plus
// non-complete coverage — never a silent clean audit.
func TestT22IncompleteRunawayStaysBounded(t *testing.T) {
	cases := []string{
		"CREATE RESOURCE GROUP q_noaction RU_PER_SEC=10 QUERY_LIMIT=(EXEC_ELAPSED='30s');",
		"CREATE RESOURCE GROUP q_nothreshold RU_PER_SEC=10 QUERY_LIMIT=(ACTION=KILL);",
	}
	for _, sql := range cases {
		result, err := t22Audit(t, sql, spec.DialectTiDB, nil, "")
		if !errors.Is(err, ErrUnsupportedStatement) {
			t.Fatalf("%q err = %v, want ErrUnsupportedStatement", sql, err)
		}
		if len(result.Unsupported) != 1 {
			t.Fatalf("%q unsupported = %+v, want one bounded aspect", sql, result.Unsupported)
		}
		if result.Statements[0].Coverage.Status == report.CoverageComplete {
			t.Fatalf("%q reported complete coverage", sql)
		}
	}
}
