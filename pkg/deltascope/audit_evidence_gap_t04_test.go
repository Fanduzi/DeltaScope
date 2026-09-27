// Package deltascope verifies the public evidence-gap projection (T04-A/#83).
// input: public audit requests under an isolated modify-column compatibility policy
// output: stable unverified/review results with evidence_gaps, preserving unsupported error semantics
// pos: public SDK contract tests for metadata evidence gaps
// note: if this file changes, update this header and module README.md.
package deltascope

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
)

const t04TargetRuleID = "ddl.alter.modify_column.compatibility.require"

func writeT04IsolatedPolicy(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "t04-isolated.yaml")
	ids := make([]string, 0, len(policy.Default().Rules))
	for id := range policy.Default().Rules {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var builder strings.Builder
	builder.WriteString("rules:\n")
	for _, id := range ids {
		if id == t04TargetRuleID {
			builder.WriteString("  " + strconv.Quote(id) + ":\n    enabled: true\n    level: blocker\n    params:\n      required: true\n      requires_metadata: true\n")
			continue
		}
		fmt.Fprintf(&builder, "  %s:\n    enabled: false\n", strconv.Quote(id))
	}
	if err := os.WriteFile(path, []byte(builder.String()), 0o600); err != nil {
		t.Fatalf("write isolated policy: %v", err)
	}
	return path
}

// A gap-only result is a successful audit: nil error, coverage unverified,
// verdict review, and the bounded evidence gap projected on the statement.
func TestAuditT04EvidenceGapProjectsThroughSDK(t *testing.T) {
	t.Parallel()

	result, err := Audit(context.Background(), Request{
		SQL:        "ALTER TABLE t MODIFY COLUMN c VARCHAR(20);",
		Dialect:    DialectMySQL,
		ConfigPath: writeT04IsolatedPolicy(t),
	})
	if err != nil {
		t.Fatalf("expected nil error for gap-only result, got %v", err)
	}
	if result.Coverage.Status != CoverageUnverified {
		t.Fatalf("expected aggregate unverified, got %q", result.Coverage.Status)
	}
	if result.Verdict != VerdictReview {
		t.Fatalf("expected review, got %q", result.Verdict)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %#v", result.Statements)
	}
	statement := result.Statements[0]
	if statement.Coverage.Status != CoverageUnverified {
		t.Fatalf("expected statement unverified, got %q", statement.Coverage.Status)
	}
	if len(statement.EvidenceGaps) != 1 {
		t.Fatalf("expected 1 evidence gap, got %#v", statement.EvidenceGaps)
	}
	gap := statement.EvidenceGaps[0]
	if gap.RuleID != t04TargetRuleID || gap.ReasonCode != "missing_source_column" {
		t.Fatalf("gap = %#v, want %q/missing_source_column", gap, t04TargetRuleID)
	}
	if len(gap.RequiredFacts) != 1 || gap.RequiredFacts[0] != "source_column.definition" {
		t.Fatalf("expected [source_column.definition], got %#v", gap.RequiredFacts)
	}
	if len(statement.Findings) != 0 {
		t.Fatalf("gap must not appear as a finding, got %#v", statement.Findings)
	}
}

// Mixed unverified + unsupported keeps ErrUnsupportedStatement and the partial
// result — evidence gaps never mask the existing unsupported contract.
func TestAuditT04GapWithUnsupportedKeepsErrorContract(t *testing.T) {
	t.Parallel()

	result, err := Audit(context.Background(), Request{
		SQL:        "ALTER TABLE t MODIFY COLUMN c VARCHAR(20); CREATE SEQUENCE golden_seq START WITH 1;",
		Dialect:    DialectMySQL,
		ConfigPath: writeT04IsolatedPolicy(t),
	})
	if !errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("expected ErrUnsupportedStatement, got %v", err)
	}
	if result.Coverage.Status != CoverageIncomplete {
		t.Fatalf("expected aggregate incomplete, got %q", result.Coverage.Status)
	}
	if len(result.Statements) != 2 {
		t.Fatalf("expected 2 statements, got %#v", result.Statements)
	}
	if result.Statements[0].Coverage.Status != CoverageUnverified || len(result.Statements[0].EvidenceGaps) != 1 {
		t.Fatalf("expected unverified+gap on first statement, got %#v", result.Statements[0])
	}
	if result.Statements[1].Coverage.Status != CoverageIncomplete {
		t.Fatalf("expected incomplete on second statement, got %#v", result.Statements[1])
	}
	if len(result.Unsupported) != 1 {
		t.Fatalf("expected 1 unsupported entry, got %#v", result.Unsupported)
	}
}
