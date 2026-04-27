package audit

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestLocateStatementsMultiLineSecondStatement(t *testing.T) {
	sql := `create table ok_users (
  id bigint unsigned not null auto_increment,
  primary key (id)
) comment='ok users';

delete from users;`

	stmts := []ParsedStatement{
		{Kind: spec.KindDDL, RawSQL: "create table ok_users (\n  id bigint unsigned not null auto_increment,\n  primary key (id)\n) comment='ok users';"},
		{Kind: spec.KindDML, RawSQL: "delete from users;"},
	}
	attachParsedStatementLocations(stmts, sql)

	if stmts[0].Line != 1 {
		t.Errorf("first statement Line=%d, want 1", stmts[0].Line)
	}
	if stmts[0].Column != 1 {
		t.Errorf("first statement Column=%d, want 1", stmts[0].Column)
	}
	if stmts[1].Line != 6 {
		t.Errorf("second statement Line=%d, want 6", stmts[1].Line)
	}
	if stmts[1].Column != 1 {
		t.Errorf("second statement Column=%d, want 1", stmts[1].Column)
	}
}

func TestLocateStatementsHandlesLeadingNewlineInRawSQL(t *testing.T) {
	sql := "create table t (id int);\n\ndelete from users;"

	stmts := []ParsedStatement{
		{Kind: spec.KindDDL, RawSQL: "create table t (id int);"},
		{Kind: spec.KindDML, RawSQL: "\ndelete from users;"},
	}
	attachParsedStatementLocations(stmts, sql)

	if stmts[0].Line != 1 {
		t.Errorf("first statement Line=%d, want 1", stmts[0].Line)
	}
	if stmts[1].Line != 3 {
		t.Errorf("second statement Line=%d, want 3 (actual delete line, not raw SQL leading newline)", stmts[1].Line)
	}
	if stmts[1].Column != 1 {
		t.Errorf("second statement Column=%d, want 1", stmts[1].Column)
	}
}

func TestLocateStatementsRepeatedStatementTextUsesProgressiveMatch(t *testing.T) {
	sql := "delete from users;\n\ndelete from users;"

	stmts := []ParsedStatement{
		{Kind: spec.KindDML, RawSQL: "delete from users;"},
		{Kind: spec.KindDML, RawSQL: "delete from users;"},
	}
	attachParsedStatementLocations(stmts, sql)

	if stmts[0].Line != 1 {
		t.Errorf("first statement Line=%d, want 1", stmts[0].Line)
	}
	if stmts[1].Line != 3 {
		t.Errorf("second statement Line=%d, want 3 (progressive match, not re-matching first)", stmts[1].Line)
	}
}

func TestLocateStatementsSkipsBlankLines(t *testing.T) {
	sql := "\n\n\ncreate table t (id int);"

	stmts := []ParsedStatement{
		{Kind: spec.KindDDL, RawSQL: "create table t (id int);"},
	}
	attachParsedStatementLocations(stmts, sql)

	if stmts[0].Line != 4 {
		t.Errorf("statement Line=%d, want 4 (after 3 blank lines)", stmts[0].Line)
	}
}

func TestLocateStatementsFallbackLeavesZeroWhenNoMatch(t *testing.T) {
	sql := "create table t (id int);"

	stmts := []ParsedStatement{
		{Kind: spec.KindDML, RawSQL: "drop table nonexistent;"},
	}
	attachParsedStatementLocations(stmts, sql)

	if stmts[0].Line != 0 {
		t.Errorf("unmatched statement Line=%d, want 0 (no forced fallback)", stmts[0].Line)
	}
	if stmts[0].Column != 0 {
		t.Errorf("unmatched statement Column=%d, want 0", stmts[0].Column)
	}
}

func TestAttachParsedStatementLocationsKeepsStatementCount(t *testing.T) {
	sql := "select 1; select 2;"

	original := []ParsedStatement{
		{Kind: spec.KindDML, RawSQL: "select 1;"},
		{Kind: spec.KindDML, RawSQL: "select 2;"},
	}
	attachParsedStatementLocations(original, sql)

	if len(original) != 2 {
		t.Fatalf("statement count changed from 2 to %d", len(original))
	}
	if original[0].RawSQL != "select 1;" {
		t.Errorf("first statement RawSQL changed to %q", original[0].RawSQL)
	}
	if original[1].RawSQL != "select 2;" {
		t.Errorf("second statement RawSQL changed to %q", original[1].RawSQL)
	}
	if original[0].Kind != spec.KindDML {
		t.Errorf("first statement Kind changed to %v", original[0].Kind)
	}
	if original[1].Kind != spec.KindDML {
		t.Errorf("second statement Kind changed to %v", original[1].Kind)
	}
}
