package audit

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

const locationFidelityMultiStmtSQL = `create table ok_users (
  id bigint unsigned not null auto_increment comment 'id',
  name varchar(32) not null default '' comment 'name',
  created_at datetime not null default current_timestamp comment 'created',
  updated_at datetime not null default current_timestamp on update current_timestamp comment 'updated',
  primary key (id)
) comment='ok users';

delete from users;`

// TestSourceLocationStatementLinePopulated verifies that Parse+Extract
// populates spec.Statement.Line with the correct statement-start line number.
func TestSourceLocationStatementLinePopulated(t *testing.T) {
	t.Parallel()
	parsed, err := Parse(context.Background(), locationFidelityMultiStmtSQL, spec.DialectMySQL)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(parsed.Statements) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(parsed.Statements))
	}

	statements, err := Extract(context.Background(), parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	// First statement (CREATE TABLE) starts at line 1.
	if statements[0].Line != 1 {
		t.Errorf("first statement Line=%d, want 1", statements[0].Line)
	}
	if statements[0].Column != 1 {
		t.Errorf("first statement Column=%d, want 1", statements[0].Column)
	}

	// Second statement (DELETE FROM) starts at line 9.
	deleteStmt := statements[1]
	if deleteStmt.Line != 9 {
		t.Errorf("delete statement Line=%d, want 9", deleteStmt.Line)
	}
	if deleteStmt.Column != 1 {
		t.Errorf("delete statement Column=%d, want 1", deleteStmt.Column)
	}
}

// TestSourceLocationFindingLocationPopulated verifies that rule.Finding.Location
// is populated for DML findings from multi-line SQL.
func TestSourceLocationFindingLocationPopulated(t *testing.T) {
	t.Parallel()
	result, err := AuditSQL(context.Background(), Request{
		SQL:     locationFidelityMultiStmtSQL,
		Dialect: spec.DialectMySQL,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}

	var whereFinding *rule.Finding
	for i := range result.Statements {
		for _, f := range result.Statements[i].Findings {
			if f.RuleID == "dml.where.require" {
				whereFinding = &f
				break
			}
		}
	}
	if whereFinding == nil {
		t.Fatal("expected dml.where.require finding for 'delete from users'")
	}

	if whereFinding.Location == nil {
		t.Fatal("dml.where.require finding Location is nil, expected {Line:9,Column:1}")
	}
	if whereFinding.Location.Line != 9 {
		t.Errorf("finding Location.Line=%d, want 9", whereFinding.Location.Line)
	}
	if whereFinding.Location.Column != 1 {
		t.Errorf("finding Location.Column=%d, want 1", whereFinding.Location.Column)
	}
}

// TestSourceLocationParsedStatementHasRawSQL confirms that RawSQL is populated
// for both statements, proving progressive matching has viable input.
func TestSourceLocationParsedStatementHasRawSQL(t *testing.T) {
	t.Parallel()
	parsed, err := Parse(context.Background(), locationFidelityMultiStmtSQL, spec.DialectMySQL)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(parsed.Statements) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(parsed.Statements))
	}

	for i, stmt := range parsed.Statements {
		if stmt.RawSQL == "" {
			t.Errorf("statement %d: RawSQL is empty", i)
		} else {
			t.Logf("statement %d RawSQL: %q", i, stmt.RawSQL)
		}
	}

	deleteRaw := parsed.Statements[1].RawSQL
	if deleteRaw == "" {
		t.Fatal("delete statement RawSQL is empty — progressive matching needs RawSQL")
	}
	t.Logf("delete RawSQL available for source mapper: %q", deleteRaw)
}

// TestSourceLocationTiDBSameAsMySQL verifies that TiDB also gets correct
// statement-start line numbers through the same source mapper.
func TestSourceLocationTiDBSameAsMySQL(t *testing.T) {
	t.Parallel()
	parsed, err := Parse(context.Background(), locationFidelityMultiStmtSQL, spec.DialectTiDB)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	statements, err := Extract(context.Background(), parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(statements) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(statements))
	}

	deleteStmt := statements[1]
	if deleteStmt.Line != 9 {
		t.Errorf("TiDB delete statement Line=%d, want 9", deleteStmt.Line)
	}
	if deleteStmt.Column != 1 {
		t.Errorf("TiDB delete statement Column=%d, want 1", deleteStmt.Column)
	}
}

// TestSourceLocationRuleProvidedLocationPreserved verifies that EvaluateStatements
// does NOT overwrite a Finding.Location already set by a rule. When the rule
// returns Location.Line=42 and the statement has Line=9, the final finding
// must retain Line=42.
func TestSourceLocationRuleProvidedLocationPreserved(t *testing.T) {
	t.Parallel()
	registry := rule.NewRegistry()
	if err := registry.RegisterStatement(&locationOverrideRule{}); err != nil {
		t.Fatalf("register rule: %v", err)
	}

	statements := []spec.Statement{
		{Kind: spec.KindDML, RawSQL: "delete from users;", Line: 9, Column: 1},
	}

	result, err := EvaluateStatements(context.Background(), registry, statements)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement result, got %d", len(result.Statements))
	}
	findings := result.Statements[0].Findings
	if len(findings) == 0 {
		t.Fatal("expected at least 1 finding from locationOverrideRule")
	}

	f := findings[0]
	if f.Location == nil {
		t.Fatal("finding Location is nil")
	}
	if f.Location.Line != 42 {
		t.Errorf("finding Location.Line=%d, want 42 (rule-provided, not overwritten by statement Line=9)", f.Location.Line)
	}
	if f.Location.Column != 7 {
		t.Errorf("finding Location.Column=%d, want 7 (rule-provided)", f.Location.Column)
	}
}

// locationOverrideRule is a test-only rule that always fires and sets a
// Finding.Location to prove EvaluateStatements preserves rule-provided locations.
type locationOverrideRule struct{}

func (locationOverrideRule) ID() string                      { return "test.location-override" }
func (locationOverrideRule) AppliesTo(_ spec.Statement) bool { return true }
func (locationOverrideRule) Evaluate(ctx context.Context, _ spec.Statement) ([]rule.Finding, error) {
	return []rule.Finding{
		{
			RuleID:   "test.location-override",
			Level:    rule.LevelWarning,
			Message:  "test finding with location",
			Location: &rule.Location{Line: 42, Column: 7},
		},
	}, nil
}
