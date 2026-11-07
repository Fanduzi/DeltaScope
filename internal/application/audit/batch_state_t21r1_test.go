package audit

import (
	"errors"
	"strings"
	"testing"
)

// T21-R1 B1: the three legacy lifecycle notices consume the shared
// ordered-state judgment. A proven refusal or guard no-op must not emit a
// notice claiming the object was created, changed, or removed; an
// applicable or unproven target keeps the declarative notice. These tests
// fail on the rejected A1 patch, where the notices fired unconditionally.
const (
	t21CreateNoticeRule = "ddl.create_sequence.notice"
	t21AlterNoticeRule  = "ddl.alter_sequence.notice"
	t21DropNoticeRule   = "ddl.drop_sequence.notice"
)

func TestT21R1NoticesFollowProvenOutcome(t *testing.T) {
	policyPath := t21Policy(t, t21ValidRule, t21CreateNoticeRule, t21AlterNoticeRule, t21DropNoticeRule)
	cases := []struct {
		name     string
		sql      string
		ruleID   string
		suppress bool
	}{
		{"create refused on present target", "CREATE SEQUENCE seq", t21CreateNoticeRule, true},
		{"create guard no-op", "CREATE SEQUENCE IF NOT EXISTS seq", t21CreateNoticeRule, true},
		{"create on absent lands", "CREATE SEQUENCE missing", t21CreateNoticeRule, false},
		{"alter refused on absent", "ALTER SEQUENCE missing START WITH 5", t21AlterNoticeRule, true},
		{"alter guard no-op", "ALTER SEQUENCE IF EXISTS missing START WITH 5", t21AlterNoticeRule, true},
		{"alter on present lands", "ALTER SEQUENCE seq START WITH 5", t21AlterNoticeRule, false},
		{"drop refused on absent", "DROP SEQUENCE missing", t21DropNoticeRule, true},
		{"drop guard no-op", "DROP SEQUENCE IF EXISTS missing", t21DropNoticeRule, true},
		{"drop on present lands", "DROP SEQUENCE seq_unref", t21DropNoticeRule, false},
		{"unproven target keeps declarative notice", "ALTER SEQUENCE ghost START WITH 5", t21AlterNoticeRule, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := t21Audit(t, tc.sql, t21Fixture(), policyPath)
			if err != nil {
				t.Fatalf("audit: %v", err)
			}
			findings := t21Findings(result, 0, tc.ruleID)
			if tc.suppress && len(findings) != 0 {
				t.Fatalf("notice findings = %#v, want none on proven refusal/no-op", findings)
			}
			if !tc.suppress && len(findings) != 1 {
				t.Fatalf("notice findings = %#v, want one", findings)
			}
		})
	}
}

// Suppression cannot depend on the valid rule emitting its blocker: with
// only the notice enabled the proven refusal still silences the success
// claim — a policy toggle never revives a refused operation.
func TestT21R1NoticeSuppressedWithoutValidRule(t *testing.T) {
	policyPath := t21Policy(t, t21AlterNoticeRule)
	result, err := t21Audit(t, "ALTER SEQUENCE missing START WITH 5", t21Fixture(), policyPath)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if findings := t21Findings(result, 0, t21AlterNoticeRule); len(findings) != 0 {
		t.Fatalf("notice findings = %#v, want none — proven refusal suppresses without the blocker", findings)
	}
}

// Multi-target drops adjudicate per member: the refused member produces
// neither a notice nor an erased verdict for the applicable member, and no
// finding may claim the refused member was removed.
func TestT21R1MultiTargetNoticeAdjudicatesPerMember(t *testing.T) {
	policyPath := t21Policy(t, t21ValidRule, t21DropNoticeRule)
	result, err := t21Audit(t, "DROP SEQUENCE missing, seq_unref", t21Fixture(), policyPath)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if findings := t21Findings(result, 0, t21ValidRule); len(findings) != 1 {
		t.Fatalf("valid findings = %#v, want one blocker on missing", findings)
	}
	findings := t21Findings(result, 0, t21DropNoticeRule)
	if len(findings) != 1 {
		t.Fatalf("notice findings = %#v, want exactly one for the applicable member", findings)
	}
	if strings.Contains(findings[0].Message, `"missing"`) || !strings.Contains(findings[0].Message, `"t21.seq_unref"`) {
		t.Fatalf("notice message = %q, want it to name \"t21.seq_unref\" only", findings[0].Message)
	}
}

// T21-R1 B2: proven edges warn per owner-slot/target pair while an
// unvouched or diverged set reports the bounded references gap — never
// silence, never a fabricated empty set.
func TestT21R1ReferencesEdgesAndCompletenessGap(t *testing.T) {
	policyPath := t21Policy(t, t21ValidRule, t21RefsRule)
	cases := []struct {
		name     string
		sql      string
		wantFind int
		wantGap  bool
	}{
		{"vouched edge warns per owner slot", "DROP SEQUENCE seq_ref", 1, false},
		{"vouched empty stays silent", "DROP SEQUENCE seq_unref", 0, false},
		{"unvouched set emits references gap", "DROP SEQUENCE seq", 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := t21Audit(t, tc.sql, t21Fixture(), policyPath)
			if err != nil {
				t.Fatalf("audit: %v", err)
			}
			findings := t21Findings(result, 0, t21RefsRule)
			if len(findings) != tc.wantFind {
				t.Fatalf("references findings = %#v, want %d", findings, tc.wantFind)
			}
			gotGap := false
			for _, gap := range t21Gaps(result, 0, t21RefsRule) {
				if gap.ReasonCode == "unknown_sequence_references" {
					gotGap = true
				}
			}
			if gotGap != tc.wantGap {
				t.Fatalf("references gap = %v, want %v (gaps %#v)", gotGap, tc.wantGap, result.Statements[0].EvidenceGaps)
			}
		})
	}
}

// A proven positive edge and an unproven completeness premise coexist:
// the provider-vouched edge still warns while the batch-authored reference
// — an expression default the audited boundary marks unsupported, leaving
// the owner's fate undecidable — keeps the set unprovable. The drop
// reports the warning AND the gap.
func TestT21R1AuthoredEdgeDivergesVouchedSet(t *testing.T) {
	policyPath := t21Policy(t, t21ValidRule, t21RefsRule)
	result, err := t21Audit(t,
		"CREATE TABLE t2 (id bigint DEFAULT NEXT VALUE FOR seq_ref); DROP SEQUENCE seq_ref",
		t21Fixture(), policyPath)
	if err != nil && !errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("audit: %v", err)
	}
	findings := t21Findings(result, 1, t21RefsRule)
	if len(findings) != 1 {
		t.Fatalf("references findings = %#v, want the one proven provider edge", findings)
	}
	gotGap := false
	for _, gap := range t21Gaps(result, 1, t21RefsRule) {
		if gap.ReasonCode == "unknown_sequence_references" {
			gotGap = true
		}
	}
	if !gotGap {
		t.Fatalf("gaps = %#v, want unknown_sequence_references — authored edge under undecidable owner", result.Statements[1].EvidenceGaps)
	}
}

// An owner whose fate is undecidable retires the provider edge from the
// proven set: an unsupported statement tombstoning the owner leaves the
// drop with no stale positive and owing the completeness gap.
func TestT21R1UndecidableOwnerRetiresProviderEdge(t *testing.T) {
	policyPath := t21Policy(t, t21ValidRule, t21RefsRule)
	result, err := t21Audit(t,
		"ALTER TABLE t ADD COLUMN x int DEFAULT NEXT VALUE FOR other_seq; DROP SEQUENCE seq_ref",
		t21Fixture(), policyPath)
	if err != nil && !errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("audit: %v", err)
	}
	if findings := t21Findings(result, 1, t21RefsRule); len(findings) != 0 {
		t.Fatalf("references findings = %#v, want none — undecidable owner revokes the positive edge", findings)
	}
	gotGap := false
	for _, gap := range t21Gaps(result, 1, t21RefsRule) {
		if gap.ReasonCode == "unknown_sequence_references" {
			gotGap = true
		}
	}
	if !gotGap {
		t.Fatalf("gaps = %#v, want unknown_sequence_references — edge fate undecidable", result.Statements[1].EvidenceGaps)
	}
}
