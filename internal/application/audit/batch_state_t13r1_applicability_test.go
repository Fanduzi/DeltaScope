// Package audit pins the T13-R1 applicability contract on the public Audit
// seam: partitioned truth, member truth, and counterpart truth drive
// findings, gaps, coverage, and verdict through the shared machinery —
// while isolated risk profiles stay untouched.
package audit

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

const (
	t13r1RuleDropApp     = "ddl.alter.drop_partition.applicability"
	t13r1RuleTruncateApp = "ddl.alter.truncate_partition.applicability"
	t13r1RuleExchangeApp = "ddl.alter.exchange_partition.applicability"
	t13r1RuleDropTarget  = "ddl.alter.drop_partition.target"
	t13r1RuleExchangeCtr = "ddl.alter.exchange_partition.counterpart"
)

func t13r1BoolP(v bool) *bool    { return &v }
func t13r1StrP(v string) *string { return &v }

func t13r1RangeShape(name string) *spec.TableSnapshot {
	return &spec.TableSnapshot{
		Exists:         true,
		Table:          &spec.Table{Name: name},
		Columns:        []spec.Column{{Name: "id", Type: "int"}, {Name: "c", Type: "int"}},
		PrimaryKey:     &spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: []string{"id"}},
		Partitioned:    t13r1BoolP(true),
		PartitionType:  t13r1StrP("range"),
		PartitionNames: []string{"p0", "p1"},
	}
}

func t13r1AuditWith(t *testing.T, sql string, provider MetadataProvider, ids ...string) report.Result {
	t.Helper()
	enabled := map[string]string{}
	for _, id := range ids {
		enabled[id] = ""
	}
	result, err := AuditSQL(context.Background(), Request{
		SQL:              sql,
		Dialect:          spec.DialectMySQL,
		Schema:           "golden",
		ConfigPath:       t05PolicyPath(t, enabled),
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	return result
}

// Proven-applicable DROP on a vouched RANGE member stays silent —
// complete/pass, no findings, no gaps.
func TestT13R1ApplicabilityProvenApplicableSilent(t *testing.T) {
	t.Parallel()
	provider := t13r1ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{
		"t": t13r1RangeShape("t"),
	})
	result := t13r1AuditWith(t, "ALTER TABLE t DROP PARTITION p0;", provider,
		t13r1RuleDropApp, t13r1RuleDropTarget)
	stmt := result.Statements[0]
	if len(stmt.Findings) != 0 || len(stmt.EvidenceGaps) != 0 {
		t.Fatalf("proven-applicable drop must stay silent, findings=%#v gaps=%+v", stmt.Findings, stmt.EvidenceGaps)
	}
	if stmt.Coverage.Status != report.CoverageComplete || result.Verdict != report.VerdictPass {
		t.Fatalf("proven-applicable drop keeps complete/pass, got %s/%s", stmt.Coverage.Status, result.Verdict)
	}
}

// Unproven partitioned truth holds exactly the consumed fact — never a
// refusal finding — and lowers the statement to unverified/review.
func TestT13R1ApplicabilityUnknownHoldsGap(t *testing.T) {
	t.Parallel()
	provider := t13r1ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{
		// A present table whose partition truth was never observed.
		"t": {Exists: true, Table: &spec.Table{Name: "t"}},
	})
	result := t13r1AuditWith(t, "ALTER TABLE t DROP PARTITION p0;", provider, t13r1RuleDropApp)
	stmt := result.Statements[0]
	gaps := t05GapsByRule(result, 0, t13r1RuleDropApp)
	if len(gaps) != 1 || gaps[0].ReasonCode != "unknown_table_state" {
		t.Fatalf("unobserved partitioned truth owes one gap, got %+v", stmt.EvidenceGaps)
	}
	found := false
	for _, fact := range gaps[0].RequiredFacts {
		found = found || fact == "target_table.partitioned"
	}
	if !found {
		t.Fatalf("gap must name target_table.partitioned, got %+v", gaps[0].RequiredFacts)
	}
	if len(stmt.Findings) != 0 {
		t.Fatalf("unproven facts must not fabricate a refusal, got %#v", stmt.Findings)
	}
	if stmt.Coverage.Status != report.CoverageUnverified || result.Verdict != report.VerdictReview {
		t.Fatalf("gap holds unverified/review, got %s/%s", stmt.Coverage.Status, result.Verdict)
	}
}

// Proven-inapplicable DROP on HASH blocks once with the native identity.
func TestT13R1ApplicabilityHashDropBlocks(t *testing.T) {
	t.Parallel()
	shape := t13r1RangeShape("t")
	shape.PartitionType = t13r1StrP("hash")
	provider := t13r1ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{"t": shape})
	result := t13r1AuditWith(t, "ALTER TABLE t DROP PARTITION p0;", provider, t13r1RuleDropApp)
	findings := t05FindingsByRule(result, 0, t13r1RuleDropApp)
	if len(findings) != 1 || findings[0].Metadata["reason_code"] != "partition_type_inapplicable" ||
		findings[0].Metadata["native_error"] != "1512" {
		t.Fatalf("hash drop must block once, got %#v", result.Statements[0].Findings)
	}
	if result.Verdict != report.VerdictReject {
		t.Fatalf("proven refusal rejects, got %s", result.Verdict)
	}
}

// The isolated risk profile keeps firing on the action alone and owes no
// applicability gaps — the new rules are not part of it.
func TestT13R1RiskPolicyIndependence(t *testing.T) {
	t.Parallel()
	result, err := AuditSQL(context.Background(), Request{
		SQL:        "ALTER TABLE t DROP PARTITION p0;",
		Dialect:    spec.DialectMySQL,
		Schema:     "golden",
		ConfigPath: t05PolicyPath(t, map[string]string{t13RuleDropPartition: ""}),
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	stmt := result.Statements[0]
	if len(t05FindingsByRule(result, 0, t13RuleDropPartition)) != 1 {
		t.Fatalf("risk policy fires on the action alone, got %#v", stmt.Findings)
	}
	if len(stmt.EvidenceGaps) != 0 {
		t.Fatalf("isolated risk profile owes no applicability gaps, got %+v", stmt.EvidenceGaps)
	}
	if stmt.Coverage.Status != report.CoverageComplete {
		t.Fatalf("isolated risk profile keeps complete coverage, got %s", stmt.Coverage.Status)
	}
}

// The counterpart projection reaches the rule layer through ordered state:
// a provider-known absent counterpart blocks the EXCHANGE with its native
// identity.
func TestT13R1CounterpartAbsentPublicAudit(t *testing.T) {
	t.Parallel()
	provider := t13r1ProviderFor(spec.DialectMySQL, map[string]*spec.TableSnapshot{
		"t":  t13r1RangeShape("t"),
		"t2": {Exists: false, Table: &spec.Table{Name: "t2"}},
	})
	result := t13r1AuditWith(t, "ALTER TABLE t EXCHANGE PARTITION p0 WITH TABLE t2;", provider,
		t13r1RuleExchangeCtr)
	findings := t05FindingsByRule(result, 0, t13r1RuleExchangeCtr)
	if len(findings) != 1 || findings[0].Metadata["reason_code"] != "counterpart_absent" ||
		findings[0].Metadata["native_error"] != "1146" {
		t.Fatalf("absent counterpart must block once, got %#v", result.Statements[0].Findings)
	}
	if result.Verdict != report.VerdictReject {
		t.Fatalf("proven refusal rejects, got %s", result.Verdict)
	}
}

// An unvouched counterpart reports the bounded existence fact — never a
// fabricated refusal. No provider at all leaves the counterpart projection
// unobserved.
func TestT13R1CounterpartUnobservedHoldsGap(t *testing.T) {
	t.Parallel()
	result, err := AuditSQL(context.Background(), Request{
		SQL:        "ALTER TABLE t EXCHANGE PARTITION p0 WITH TABLE t2;",
		Dialect:    spec.DialectMySQL,
		Schema:     "golden",
		ConfigPath: t05PolicyPath(t, map[string]string{t13r1RuleExchangeCtr: ""}),
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	stmt := result.Statements[0]
	gaps := t05GapsByRule(result, 0, t13r1RuleExchangeCtr)
	if len(gaps) != 1 {
		t.Fatalf("unvouched counterpart owes one gap, got %+v", stmt.EvidenceGaps)
	}
	found := false
	for _, fact := range gaps[0].RequiredFacts {
		found = found || fact == "exchange_table.existence"
	}
	if !found {
		t.Fatalf("gap must name exchange_table.existence, got %+v", gaps[0].RequiredFacts)
	}
	if len(stmt.Findings) != 0 {
		t.Fatalf("unproven counterpart must not fabricate a refusal, got %#v", stmt.Findings)
	}
	if stmt.Coverage.Status != report.CoverageUnverified || result.Verdict != report.VerdictReview {
		t.Fatalf("gap holds unverified/review, got %s/%s", stmt.Coverage.Status, result.Verdict)
	}
}
