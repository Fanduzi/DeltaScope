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

	deleteRaw := parsed.Statements[1].RawSQL
	if deleteRaw == "" {
		t.Fatal("delete statement RawSQL is empty — progressive matching needs RawSQL")
	}
	t.Logf("delete RawSQL available for source mapper: %q", deleteRaw)
}

// TestSourceLocationTiDBSameAsMySQL verifies that TiDB also gets correct
// statement-start line numbers through the same source mapper.
func TestSourceLocationTiDBSameAsMySQL(t *testing.T) {
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
	if deleteStmt.Line != 9 {
		t.Errorf("TiDB delete statement Line=%d, want 9", deleteStmt.Line)
	}
	if deleteStmt.Column != 1 {
		t.Errorf("TiDB delete statement Column=%d, want 1", deleteStmt.Column)
	}
}
