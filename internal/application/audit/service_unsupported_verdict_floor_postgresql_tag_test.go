//go:build postgresql

// Package audit verifies the application audit service behavior.
// input: PostgreSQL audit requests containing structured unsupported statements
// output: AuditSQL completeness floor from pass to review with audited-only statements
// pos: application-layer regression coverage for the unsupported-statement verdict floor
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestAuditUnsupportedStatementFloorsPassVerdictToReview(t *testing.T) {
	t.Parallel()

	result, err := AuditSQL(context.Background(), Request{
		SQL:     "SELECT 1",
		Dialect: spec.DialectPostgreSQL,
	})
	if !errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("expected unsupported statement sentinel, got %v", err)
	}
	if result.Verdict != report.VerdictReview {
		t.Fatalf("expected unsupported completeness floor to review, got %q", result.Verdict)
	}
	if len(result.Statements) != 0 || result.Summary.Statements != 0 {
		t.Fatalf("expected zero audited statements, got statements=%#v summary=%+v", result.Statements, result.Summary)
	}
	if len(result.Unsupported) != 1 || result.Unsupported[0].Feature != "select" {
		t.Fatalf("expected one select unsupported detail, got %#v", result.Unsupported)
	}
	if len(result.Diagnostics) != 1 {
		t.Fatalf("expected one unsupported diagnostic, got %#v", result.Diagnostics)
	}
	diagnostic := result.Diagnostics[0]
	if diagnostic.Classification != DiagnosticUnsupportedStatement || diagnostic.Audited {
		t.Fatalf("expected unaudited unsupported_statement diagnostic, got %#v", diagnostic)
	}
	payload, marshalErr := json.Marshal(diagnostic)
	if marshalErr != nil {
		t.Fatalf("marshal diagnostic: %v", marshalErr)
	}
	combined := diagnostic.Reason + diagnostic.ActionHint + diagnostic.Dialect + err.Error() + string(payload)
	if strings.Contains(combined, "SELECT 1") || strings.Contains(strings.ToLower(combined), "select 1") {
		t.Fatalf("unsupported diagnostic leaked SQL text: %s", payload)
	}
}

func TestAuditUnsupportedStatementPreservesSupportedSiblingsAndFloorsPass(t *testing.T) {
	t.Parallel()

	result, err := AuditSQL(context.Background(), Request{
		SQL:     "CREATE SCHEMA staging; SELECT 1;",
		Dialect: spec.DialectPostgreSQL,
	})
	if !errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("expected unsupported statement sentinel, got %v", err)
	}
	if result.Verdict != report.VerdictReview {
		t.Fatalf("expected mixed unsupported completeness floor to review, got %q", result.Verdict)
	}
	if result.Summary.Blockers != 0 || result.Summary.Warnings != 0 {
		t.Fatalf("expected notice-only audited findings so the floor is pass-to-review, got summary=%+v", result.Summary)
	}
	if result.Summary.Notices == 0 {
		t.Fatalf("expected preserved CREATE SCHEMA notice, got summary=%+v statements=%#v", result.Summary, result.Statements)
	}
	if len(result.Statements) != 1 || result.Summary.Statements != 1 {
		t.Fatalf("expected one audited statement, got statements=%#v summary=%+v", result.Statements, result.Summary)
	}
	if result.Statements[0].Kind != spec.KindDDL.String() {
		t.Fatalf("expected supported statement kind ddl, got %#v", result.Statements[0])
	}
	if result.Statements[0].Index != 0 {
		t.Fatalf("expected audited statement to keep source order index 0, got %#v", result.Statements[0])
	}
	if len(result.Unsupported) != 1 || result.Unsupported[0].Feature != "select" || result.Unsupported[0].Index != 1 {
		t.Fatalf("expected trailing select unsupported detail, got %#v", result.Unsupported)
	}
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Classification != DiagnosticUnsupportedStatement || result.Diagnostics[0].Audited {
		t.Fatalf("expected one unaudited unsupported diagnostic, got %#v", result.Diagnostics)
	}
}

func TestAuditUnsupportedStatementKeepsReviewVerdict(t *testing.T) {
	t.Parallel()

	result, err := AuditSQL(context.Background(), Request{
		SQL:     "DROP SCHEMA staging CASCADE; SELECT 1;",
		Dialect: spec.DialectPostgreSQL,
	})
	if !errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("expected unsupported statement sentinel, got %v", err)
	}
	if result.Verdict != report.VerdictReview {
		t.Fatalf("expected existing review verdict to remain unchanged, got %q", result.Verdict)
	}
	if result.Summary.Warnings == 0 {
		t.Fatalf("expected warning-level sibling findings to remain, got summary=%+v", result.Summary)
	}
	if len(result.Statements) != 1 || result.Summary.Statements != 1 {
		t.Fatalf("expected one audited statement, got statements=%#v summary=%+v", result.Statements, result.Summary)
	}
	if len(result.Unsupported) != 1 || result.Unsupported[0].Feature != "select" {
		t.Fatalf("expected one select unsupported detail, got %#v", result.Unsupported)
	}
}

func TestAuditUnsupportedStatementKeepsRejectVerdict(t *testing.T) {
	t.Parallel()

	result, err := AuditSQL(context.Background(), Request{
		SQL:     "DELETE FROM users; SELECT 1;",
		Dialect: spec.DialectPostgreSQL,
	})
	if !errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("expected unsupported statement sentinel, got %v", err)
	}
	if result.Verdict != report.VerdictReject {
		t.Fatalf("expected existing reject verdict to remain unchanged, got %q", result.Verdict)
	}
	if result.Summary.Blockers == 0 {
		t.Fatalf("expected blocker-level sibling findings to remain, got summary=%+v", result.Summary)
	}
	if len(result.Statements) != 1 || result.Summary.Statements != 1 {
		t.Fatalf("expected one audited statement, got statements=%#v summary=%+v", result.Statements, result.Summary)
	}
	if result.Statements[0].Impact == nil {
		t.Fatalf("expected audited DELETE impact to be preserved, got %#v", result.Statements[0])
	}
	if len(result.Unsupported) != 1 || result.Unsupported[0].Feature != "select" {
		t.Fatalf("expected one select unsupported detail, got %#v", result.Unsupported)
	}
}

func TestAuditUnsupportedStatementMetadataAwareRouteMatchesOfflineFloor(t *testing.T) {
	t.Parallel()

	result, err := AuditSQL(context.Background(), Request{
		SQL:              "SELECT 1",
		Dialect:          spec.DialectPostgreSQL,
		Schema:           "public",
		MetadataProvider: &fakeMetadataProvider{instance: &spec.InstanceFacts{}},
	})
	if !errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("expected unsupported statement sentinel, got %v", err)
	}
	if result.Verdict != report.VerdictReview {
		t.Fatalf("expected metadata-aware unsupported completeness floor to review, got %q", result.Verdict)
	}
	if len(result.Statements) != 0 || result.Summary.Statements != 0 {
		t.Fatalf("expected zero audited statements, got statements=%#v summary=%+v", result.Statements, result.Summary)
	}
	if len(result.Unsupported) != 1 || result.Unsupported[0].Feature != "select" {
		t.Fatalf("expected one select unsupported detail, got %#v", result.Unsupported)
	}
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Classification != DiagnosticUnsupportedStatement || result.Diagnostics[0].Audited {
		t.Fatalf("expected one unaudited unsupported diagnostic, got %#v", result.Diagnostics)
	}
}
