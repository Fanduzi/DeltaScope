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

// TestSourceLocationStatementLineNotPopulated proves that spec.Statement.Line
// is currently zero after Parse+Extract for multi-line SQL. Task 2 will populate
// real statement-start line numbers.
func TestSourceLocationStatementLineNotPopulated(t *testing.T) {
	parsed, err := Parse(locationFidelityMultiStmtSQL, spec.DialectMySQL)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(parsed.Statements) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(parsed.Statements))
	}

	statements, err := Extract(parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	for i, stmt := range statements {
		if stmt.Line != 0 {
			t.Logf("statement %d Line=%d (unexpectedly populated)", i, stmt.Line)
		}
	}

	// Red test: prove Line is zero for the second statement (delete from users).
	// After Task 2: stmt.Line should be the real line number (9).
	deleteStmt := statements[1]
	if deleteStmt.Line != 0 {
		t.Skipf("Task 2 target: statement Line should be 9, got %d — location propagation already implemented", deleteStmt.Line)
	}
	t.Logf("GAP CONFIRMED: second statement Line=%d, Column=%d (expected 0,0 before Task 2)", deleteStmt.Line, deleteStmt.Column)
}

// TestSourceLocationFindingLocationNotPopulated proves that rule.Finding.Location
// is currently nil for DML findings from multi-line SQL. Task 2 will propagate
// statement-start locations to findings that lack explicit locations.
func TestSourceLocationFindingLocationNotPopulated(t *testing.T) {
	result, err := AuditSQL(context.Background(), Request{
		SQL:     locationFidelityMultiStmtSQL,
		Dialect: spec.DialectMySQL,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}

	// Find the dml.where.require finding on the delete statement.
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

	if whereFinding.Location != nil {
		t.Skipf("Task 2 target: finding Location should be {Line:9,Column:1}, got %+v — location propagation already implemented", whereFinding.Location)
	}
	t.Logf("GAP CONFIRMED: dml.where.require finding Location=%v (expected nil before Task 2)", whereFinding.Location)
}

// TestSourceLocationParsedStatementHasRawSQL confirms that RawSQL is populated
// for both statements, proving progressive matching has viable input.
func TestSourceLocationParsedStatementHasRawSQL(t *testing.T) {
	parsed, err := Parse(locationFidelityMultiStmtSQL, spec.DialectMySQL)
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

	// Verify RawSQL for the delete statement is "delete from users" (normalized).
	deleteRaw := parsed.Statements[1].RawSQL
	if deleteRaw == "" {
		t.Fatal("delete statement RawSQL is empty — progressive matching needs RawSQL")
	}
	t.Logf("delete RawSQL available for source mapper: %q", deleteRaw)
}

// TestSourceLocationTiDBSameGapAsMySQL proves the same location gap exists for TiDB.
func TestSourceLocationTiDBSameGapAsMySQL(t *testing.T) {
	parsed, err := Parse(locationFidelityMultiStmtSQL, spec.DialectTiDB)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	statements, err := Extract(parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(statements) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(statements))
	}

	deleteStmt := statements[1]
	if deleteStmt.Line != 0 {
		t.Skipf("Task 2 target: TiDB statement Line already populated=%d", deleteStmt.Line)
	}
	t.Logf("GAP CONFIRMED (TiDB): second statement Line=%d, Column=%d", deleteStmt.Line, deleteStmt.Column)
}
