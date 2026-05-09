//go:build postgresql

package postgresql

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

func TestExtractCreateTableIdentityColumnIsSupported(t *testing.T) {
	t.Parallel()
	sql := `CREATE TABLE users (
	  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	  email text
	);`

	statement := extractPostgreSQLStatement(t, sql)

	if statement.Unsupported != nil {
		t.Fatalf("expected no unsupported detail, got feature=%q", statement.Unsupported.Feature)
	}
	if statement.Kind != spec.KindDDL {
		t.Fatalf("expected kind DDL, got %q", statement.Kind)
	}
	if statement.DDL == nil {
		t.Fatal("expected DDL payload")
	}
	if statement.DDL.Operation != spec.DDLOperationCreateTable {
		t.Fatalf("expected operation create_table, got %q", statement.DDL.Operation)
	}
	var col *spec.Column
	for i := range statement.DDL.Columns {
		if statement.DDL.Columns[i].Name == "id" {
			col = &statement.DDL.Columns[i]
			break
		}
	}
	if col == nil {
		t.Fatal("expected column id in DDL.Columns")
	}
	if !col.IsIdentity {
		t.Fatal("expected is_identity=true")
	}
	if col.GeneratedWhen != "a" {
		t.Fatalf("expected generated_when=a, got %q", col.GeneratedWhen)
	}
}

func TestExtractCreateTableGeneratedStoredColumnIsSupported(t *testing.T) {
	t.Parallel()
	sql := `CREATE TABLE users (
	  first_name text,
	  last_name text,
	  full_name text GENERATED ALWAYS AS (first_name || ' ' || last_name) STORED
	);`

	statement := extractPostgreSQLStatement(t, sql)

	if statement.Unsupported != nil {
		t.Fatalf("expected no unsupported detail, got feature=%q", statement.Unsupported.Feature)
	}
	if statement.Kind != spec.KindDDL {
		t.Fatalf("expected kind DDL, got %q", statement.Kind)
	}
	if statement.DDL == nil {
		t.Fatal("expected DDL payload")
	}
	if statement.DDL.Operation != spec.DDLOperationCreateTable {
		t.Fatalf("expected operation create_table, got %q", statement.DDL.Operation)
	}
	var col *spec.Column
	for i := range statement.DDL.Columns {
		if statement.DDL.Columns[i].Name == "full_name" {
			col = &statement.DDL.Columns[i]
			break
		}
	}
	if col == nil {
		t.Fatal("expected column full_name in DDL.Columns")
	}
	if col.GeneratedWhen != "a" {
		t.Fatalf("expected generated_when=a, got %q", col.GeneratedWhen)
	}
	if col.IsIdentity {
		t.Fatal("expected is_identity=false for generated stored column")
	}
}

func TestExtractCreateTableExclusionConstraintReturnsUnsupported(t *testing.T) {
	t.Parallel()
	sql := `CREATE TABLE bookings (
  room_id int,
  during tsrange,
  EXCLUDE USING gist (room_id WITH =, during WITH &&)
);`

	statement := extractPostgreSQLStatement(t, sql)

	if statement.Kind != spec.KindUnknown {
		t.Fatalf("expected kind %q, got %q", spec.KindUnknown, statement.Kind)
	}
	if statement.Unsupported == nil {
		t.Fatal("expected unsupported detail for EXCLUDE constraint")
	}
	if statement.Unsupported.Feature != "exclusion_constraint" {
		t.Fatalf("expected unsupported feature 'exclusion_constraint', got %q", statement.Unsupported.Feature)
	}
	if statement.Unsupported.Reason == "" {
		t.Fatal("expected non-empty unsupported reason")
	}
	t.Logf("unsupported: feature=%q reason=%q", statement.Unsupported.Feature, statement.Unsupported.Reason)
}

func TestExtractCreateTablePartitionByStillReturnsUnsupported(t *testing.T) {
	t.Parallel()
	sql := `CREATE TABLE events (
  id bigint,
  created_at timestamptz NOT NULL
) PARTITION BY RANGE (created_at);`

	statement := extractPostgreSQLStatement(t, sql)

	if statement.Kind != spec.KindUnknown {
		t.Fatalf("expected kind %q, got %q", spec.KindUnknown, statement.Kind)
	}
	if statement.Unsupported == nil {
		t.Fatal("expected unsupported detail for PARTITION BY")
	}
	if statement.Unsupported.Feature != "partitioning" {
		t.Fatalf("expected unsupported feature 'partitioning', got %q", statement.Unsupported.Feature)
	}
	if statement.Unsupported.Reason == "" {
		t.Fatal("expected non-empty unsupported reason")
	}
	t.Logf("unsupported: feature=%q reason=%q", statement.Unsupported.Feature, statement.Unsupported.Reason)
}

// ---------------------------------------------------------------------------
// v0.26.0 Task 1: AST characterization tests for CREATE TABLE boundary cases
// ---------------------------------------------------------------------------
// These tests lock down AST facts about how pg_query_go/v6 represents
// PostgreSQL features that are currently unsupported by the extractor.
// They do NOT assert extractor behavior — only the raw AST shape so that
// Task 2+ can make informed changes.
// ---------------------------------------------------------------------------

func TestParseCreateTableIdentityColumnAST(t *testing.T) {
	t.Parallel()
	sql := `CREATE TABLE users (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  email text
);`

	stmt := parseCreateStmtAST(t, sql)

	// There should be 2 table elements (id column, email column).
	// PRIMARY KEY is an inline column constraint, not a separate table element.
	elts := stmt.GetTableElts()
	if len(elts) != 2 {
		t.Fatalf("expected 2 table elements, got %d", len(elts))
	}

	// First element must be a ColumnDef.
	colNode := elts[0].GetColumnDef()
	if colNode == nil {
		t.Fatal("first table element is not a ColumnDef")
	}

	// FACT 1: ColumnDef.Identity is EMPTY for GENERATED ALWAYS AS IDENTITY.
	// Despite the protobuf field existing, pg_query_go/v6 represents identity
	// through the column constraint list (CONSTR_IDENTITY), not via this field.
	identity := colNode.GetIdentity()
	t.Logf("ColumnDef.Identity = %q", identity)
	if identity != "" {
		t.Logf("ColumnDef.Identity is non-empty (%q) — the hypothesis that it is always empty was wrong", identity)
	}

	// FACT 2: Identity is represented as CONSTR_IDENTITY (4) in the column
	// constraint list, with GeneratedWhen="a" (ALWAYS).
	constraints := colNode.GetConstraints()
	t.Logf("ColumnDef has %d inline constraints", len(constraints))
	var identityConstraint *pg_query.Constraint
	for _, c := range constraints {
		con := c.GetConstraint()
		if con != nil && con.GetContype() == pg_query.ConstrType_CONSTR_IDENTITY {
			identityConstraint = con
			break
		}
	}
	if identityConstraint == nil {
		t.Fatal("expected CONSTR_IDENTITY in column constraint list for GENERATED ALWAYS AS IDENTITY")
	}
	t.Logf("CONSTR_IDENTITY.GeneratedWhen = %q", identityConstraint.GetGeneratedWhen())
	if identityConstraint.GetGeneratedWhen() != "a" {
		t.Fatalf("expected GeneratedWhen='a' for ALWAYS, got %q", identityConstraint.GetGeneratedWhen())
	}

	// FACT 3: PRIMARY KEY appears as CONSTR_PRIMARY in the column constraint list.
	foundPrimary := false
	for _, c := range constraints {
		con := c.GetConstraint()
		if con != nil && con.GetContype() == pg_query.ConstrType_CONSTR_PRIMARY {
			foundPrimary = true
			break
		}
	}
	if !foundPrimary {
		t.Fatal("expected CONSTR_PRIMARY in column constraint list for PRIMARY KEY")
	}
}

func TestParseCreateTableGeneratedStoredColumnAST(t *testing.T) {
	t.Parallel()
	sql := `CREATE TABLE users (
  first_name text,
  last_name text,
  full_name text GENERATED ALWAYS AS (first_name || ' ' || last_name) STORED
);`

	stmt := parseCreateStmtAST(t, sql)

	elts := stmt.GetTableElts()
	if len(elts) != 3 {
		t.Fatalf("expected 3 table elements, got %d", len(elts))
	}

	// Third element is the generated column.
	colNode := elts[2].GetColumnDef()
	if colNode == nil {
		t.Fatal("third table element is not a ColumnDef")
	}
	if colNode.GetColname() != "full_name" {
		t.Fatalf("expected column name 'full_name', got %q", colNode.GetColname())
	}

	// FACT 1: ColumnDef.Generated is EMPTY for GENERATED ALWAYS AS (...) STORED.
	// Despite the protobuf field existing, pg_query_go/v6 represents generated
	// columns through the column constraint list (CONSTR_GENERATED), not via this field.
	generated := colNode.GetGenerated()
	t.Logf("ColumnDef.Generated = %q", generated)
	if generated != "" {
		t.Logf("ColumnDef.Generated is non-empty (%q) — the hypothesis that it is always empty was wrong", generated)
	}

	// FACT 2: ColumnDef.Identity is also empty (this is a generated column, not identity).
	identity := colNode.GetIdentity()
	t.Logf("ColumnDef.Identity = %q", identity)
	if identity != "" {
		t.Fatalf("expected ColumnDef.Identity to be empty for GENERATED column, got %q", identity)
	}

	// FACT 3: The generated expression is represented as CONSTR_GENERATED (5) in
	// the column constraint list, with GeneratedWhen="a" (ALWAYS) and RawExpr populated.
	constraints := colNode.GetConstraints()
	t.Logf("ColumnDef has %d inline constraints", len(constraints))
	var generatedConstraint *pg_query.Constraint
	for _, c := range constraints {
		con := c.GetConstraint()
		if con != nil && con.GetContype() == pg_query.ConstrType_CONSTR_GENERATED {
			generatedConstraint = con
			break
		}
	}
	if generatedConstraint == nil {
		t.Fatal("expected CONSTR_GENERATED in column constraint list for GENERATED ALWAYS AS (...) STORED")
	}

	// FACT 4: GeneratedWhen is "a" for ALWAYS.
	gw := generatedConstraint.GetGeneratedWhen()
	t.Logf("CONSTR_GENERATED.GeneratedWhen = %q", gw)
	if gw != "a" {
		t.Fatalf("expected GeneratedWhen='a' for ALWAYS, got %q", gw)
	}

	// FACT 5: RawExpr is populated with the generation expression.
	if generatedConstraint.GetRawExpr() == nil {
		t.Fatal("expected CONSTR_GENERATED.RawExpr to be non-nil for generated column expression")
	}
}

func TestParseAlterTableAddGeneratedStoredColumnAST(t *testing.T) {
	t.Parallel()
	sql := `ALTER TABLE users
  ADD COLUMN full_name text GENERATED ALWAYS AS (first_name || ' ' || last_name) STORED;`

	stmt := parseAlterTableStmtAST(t, sql)
	if stmt == nil {
		t.Fatal("expected AlterTableStmt")
	}
	cmds := stmt.GetCmds()
	if len(cmds) != 1 {
		t.Fatalf("expected 1 alter command, got %d", len(cmds))
	}
	cmd := cmds[0].GetAlterTableCmd()
	if cmd == nil {
		t.Fatal("expected AlterTableCmd")
	}
	if cmd.GetSubtype() != pg_query.AlterTableType_AT_AddColumn {
		t.Fatalf("expected subtype AT_AddColumn, got %s (%d)", cmd.GetSubtype().String(), cmd.GetSubtype())
	}
	column := cmd.GetDef().GetColumnDef()
	if column == nil {
		t.Fatal("expected add-column command def to be ColumnDef")
	}
	if column.GetColname() != "full_name" {
		t.Fatalf("expected column name full_name, got %q", column.GetColname())
	}
	if column.GetGenerated() != "" {
		t.Fatalf("expected ColumnDef.Generated to be empty, got %q", column.GetGenerated())
	}
	constraints := column.GetConstraints()
	if len(constraints) != 1 {
		t.Fatalf("expected 1 column constraint, got %d", len(constraints))
	}
	constraint := constraints[0].GetConstraint()
	if constraint == nil {
		t.Fatal("expected generated constraint node")
	}
	if constraint.GetContype() != pg_query.ConstrType_CONSTR_GENERATED {
		t.Fatalf("expected CONSTR_GENERATED, got %s (%d)", constraint.GetContype().String(), constraint.GetContype())
	}
	if constraint.GetGeneratedWhen() != "a" {
		t.Fatalf("expected GeneratedWhen='a', got %q", constraint.GetGeneratedWhen())
	}
	if constraint.GetRawExpr() == nil {
		t.Fatal("expected CONSTR_GENERATED.RawExpr to be non-nil")
	}
}

func TestParseAlterTableAddIdentityColumnAST(t *testing.T) {
	t.Parallel()
	sql := `ALTER TABLE users
  ADD COLUMN id bigint GENERATED ALWAYS AS IDENTITY;`

	stmt := parseAlterTableStmtAST(t, sql)
	if stmt == nil {
		t.Fatal("expected AlterTableStmt")
	}
	cmds := stmt.GetCmds()
	if len(cmds) != 1 {
		t.Fatalf("expected 1 alter command, got %d", len(cmds))
	}
	cmd := cmds[0].GetAlterTableCmd()
	if cmd == nil {
		t.Fatal("expected AlterTableCmd")
	}
	if cmd.GetSubtype() != pg_query.AlterTableType_AT_AddColumn {
		t.Fatalf("expected subtype AT_AddColumn, got %s (%d)", cmd.GetSubtype().String(), cmd.GetSubtype())
	}
	column := cmd.GetDef().GetColumnDef()
	if column == nil {
		t.Fatal("expected add-column command def to be ColumnDef")
	}
	if column.GetColname() != "id" {
		t.Fatalf("expected column name id, got %q", column.GetColname())
	}
	if column.GetIdentity() != "" {
		t.Fatalf("expected ColumnDef.Identity to be empty, got %q", column.GetIdentity())
	}
	constraints := column.GetConstraints()
	if len(constraints) != 1 {
		t.Fatalf("expected 1 column constraint, got %d", len(constraints))
	}
	constraint := constraints[0].GetConstraint()
	if constraint == nil {
		t.Fatal("expected identity constraint node")
	}
	if constraint.GetContype() != pg_query.ConstrType_CONSTR_IDENTITY {
		t.Fatalf("expected CONSTR_IDENTITY, got %s (%d)", constraint.GetContype().String(), constraint.GetContype())
	}
	if constraint.GetGeneratedWhen() != "a" {
		t.Fatalf("expected GeneratedWhen='a', got %q", constraint.GetGeneratedWhen())
	}
}

// ---------------------------------------------------------------------------
// v0.32.0 Task 2: Support-readiness characterization tests
// ---------------------------------------------------------------------------
// These tests prove specific AST facts about GENERATED BY DEFAULT AS IDENTITY
// and identity sequence options. They do NOT assert extractor behavior.
// ---------------------------------------------------------------------------

func TestParseCreateTableIdentityByDefaultWithOptionsSupportReadinessFacts(t *testing.T) {
	t.Parallel()
	sql := `CREATE TABLE users (
  id bigint GENERATED BY DEFAULT AS IDENTITY (START WITH 10 INCREMENT BY 5 CACHE 20),
  email text
);`

	stmt := parseCreateStmtAST(t, sql)

	elts := stmt.GetTableElts()
	if len(elts) != 2 {
		t.Fatalf("expected 2 table elements, got %d", len(elts))
	}

	colNode := elts[0].GetColumnDef()
	if colNode == nil {
		t.Fatal("first table element is not a ColumnDef")
	}
	if colNode.GetColname() != "id" {
		t.Fatalf("expected column name 'id', got %q", colNode.GetColname())
	}

	// FACT 1: ColumnDef.Identity is empty — identity is conveyed through
	// the constraint list, not this protobuf field.
	if identity := colNode.GetIdentity(); identity != "" {
		t.Fatalf("expected ColumnDef.Identity to be empty, got %q", identity)
	}

	// FACT 2: Identity appears as CONSTR_IDENTITY with GeneratedWhen="d" (BY DEFAULT).
	constraints := colNode.GetConstraints()
	var identityConstraint *pg_query.Constraint
	for _, c := range constraints {
		con := c.GetConstraint()
		if con != nil && con.GetContype() == pg_query.ConstrType_CONSTR_IDENTITY {
			identityConstraint = con
			break
		}
	}
	if identityConstraint == nil {
		t.Fatal("expected CONSTR_IDENTITY in column constraint list")
	}
	if identityConstraint.GetGeneratedWhen() != "d" {
		t.Fatalf("expected GeneratedWhen='d' for BY DEFAULT, got %q", identityConstraint.GetGeneratedWhen())
	}

	// FACT 3: Identity sequence options are in Constraint.Options as DefElem nodes.
	options := identityConstraint.GetOptions()
	if len(options) != 3 {
		t.Fatalf("expected 3 identity sequence options, got %d", len(options))
	}

	optionValues := make(map[string]int32)
	for _, opt := range options {
		defElem := opt.GetDefElem()
		if defElem == nil {
			t.Fatal("expected identity option to be a DefElem")
		}
		name := defElem.GetDefname()
		argNode := defElem.GetArg()
		if argNode == nil || argNode.GetInteger() == nil {
			t.Fatalf("expected identity option %q arg to be an Integer", name)
		}
		optionValues[name] = argNode.GetInteger().GetIval()
	}

	// FACT 4: Option defnames are lowercase ("start", "increment", "cache").
	if _, ok := optionValues["start"]; !ok {
		t.Fatal("expected option 'start' in identity sequence options")
	}
	if _, ok := optionValues["increment"]; !ok {
		t.Fatal("expected option 'increment' in identity sequence options")
	}
	if _, ok := optionValues["cache"]; !ok {
		t.Fatal("expected option 'cache' in identity sequence options")
	}

	// FACT 5: Option values match the SQL literal values.
	if optionValues["start"] != 10 {
		t.Fatalf("expected start=10, got %d", optionValues["start"])
	}
	if optionValues["increment"] != 5 {
		t.Fatalf("expected increment=5, got %d", optionValues["increment"])
	}
	if optionValues["cache"] != 20 {
		t.Fatalf("expected cache=20, got %d", optionValues["cache"])
	}
}

func TestParseAlterTableAddIdentityByDefaultWithOptionsSupportReadinessFacts(t *testing.T) {
	t.Parallel()
	sql := `ALTER TABLE users
  ADD COLUMN id bigint GENERATED BY DEFAULT AS IDENTITY (START WITH 10 INCREMENT BY 5 CACHE 20);`

	stmt := parseAlterTableStmtAST(t, sql)
	if stmt == nil {
		t.Fatal("expected AlterTableStmt")
	}
	cmds := stmt.GetCmds()
	if len(cmds) != 1 {
		t.Fatalf("expected 1 alter command, got %d", len(cmds))
	}
	cmd := cmds[0].GetAlterTableCmd()
	if cmd == nil {
		t.Fatal("expected AlterTableCmd")
	}

	// FACT 1: Command subtype is AT_AddColumn.
	if cmd.GetSubtype() != pg_query.AlterTableType_AT_AddColumn {
		t.Fatalf("expected subtype AT_AddColumn, got %s (%d)", cmd.GetSubtype().String(), cmd.GetSubtype())
	}

	// FACT 2: Command def is a ColumnDef for column "id".
	column := cmd.GetDef().GetColumnDef()
	if column == nil {
		t.Fatal("expected add-column command def to be ColumnDef")
	}
	if column.GetColname() != "id" {
		t.Fatalf("expected column name 'id', got %q", column.GetColname())
	}

	// FACT 3: ColumnDef.Identity is empty (same as CREATE TABLE).
	if identity := column.GetIdentity(); identity != "" {
		t.Fatalf("expected ColumnDef.Identity to be empty, got %q", identity)
	}

	// FACT 4: Identity appears as CONSTR_IDENTITY with GeneratedWhen="d" (BY DEFAULT).
	constraints := column.GetConstraints()
	var identityConstraint *pg_query.Constraint
	for _, c := range constraints {
		con := c.GetConstraint()
		if con != nil && con.GetContype() == pg_query.ConstrType_CONSTR_IDENTITY {
			identityConstraint = con
			break
		}
	}
	if identityConstraint == nil {
		t.Fatal("expected CONSTR_IDENTITY in column constraint list")
	}
	if identityConstraint.GetGeneratedWhen() != "d" {
		t.Fatalf("expected GeneratedWhen='d' for BY DEFAULT, got %q", identityConstraint.GetGeneratedWhen())
	}

	// FACT 5: Sequence option shape is identical to CREATE TABLE (DefElem list).
	options := identityConstraint.GetOptions()
	if len(options) != 3 {
		t.Fatalf("expected 3 identity sequence options, got %d", len(options))
	}

	optionValues := make(map[string]int32)
	for _, opt := range options {
		defElem := opt.GetDefElem()
		if defElem == nil {
			t.Fatal("expected identity option to be a DefElem")
		}
		name := defElem.GetDefname()
		argNode := defElem.GetArg()
		if argNode == nil || argNode.GetInteger() == nil {
			t.Fatalf("expected identity option %q arg to be an Integer", name)
		}
		optionValues[name] = argNode.GetInteger().GetIval()
	}

	if _, ok := optionValues["start"]; !ok {
		t.Fatal("expected option 'start' in identity sequence options")
	}
	if _, ok := optionValues["increment"]; !ok {
		t.Fatal("expected option 'increment' in identity sequence options")
	}
	if _, ok := optionValues["cache"]; !ok {
		t.Fatal("expected option 'cache' in identity sequence options")
	}
	if optionValues["start"] != 10 {
		t.Fatalf("expected start=10, got %d", optionValues["start"])
	}
	if optionValues["increment"] != 5 {
		t.Fatalf("expected increment=5, got %d", optionValues["increment"])
	}
	if optionValues["cache"] != 20 {
		t.Fatalf("expected cache=20, got %d", optionValues["cache"])
	}
}

func TestParseAlterTableDropGeneratedExpressionAST(t *testing.T) {
	t.Parallel()
	sql := `ALTER TABLE users
  ALTER COLUMN full_name DROP EXPRESSION;`

	stmt := parseAlterTableStmtAST(t, sql)
	if stmt == nil {
		t.Fatal("expected AlterTableStmt")
	}
	cmds := stmt.GetCmds()
	if len(cmds) != 1 {
		t.Fatalf("expected 1 alter command, got %d", len(cmds))
	}
	cmd := cmds[0].GetAlterTableCmd()
	if cmd == nil {
		t.Fatal("expected AlterTableCmd")
	}
	if cmd.GetSubtype() != pg_query.AlterTableType_AT_DropExpression {
		t.Fatalf("expected subtype AT_DropExpression, got %s (%d)", cmd.GetSubtype().String(), cmd.GetSubtype())
	}
	if cmd.GetName() != "full_name" {
		t.Fatalf("expected column name full_name, got %q", cmd.GetName())
	}
	if cmd.GetDef() != nil {
		t.Fatalf("expected AT_DropExpression def to be nil, got %T", cmd.GetDef().GetNode())
	}
}

func TestParseAlterTableSetIdentityGeneratedAST(t *testing.T) {
	t.Parallel()
	sql := `ALTER TABLE users
  ALTER COLUMN id SET GENERATED BY DEFAULT;`

	stmt := parseAlterTableStmtAST(t, sql)
	if stmt == nil {
		t.Fatal("expected AlterTableStmt")
	}
	cmds := stmt.GetCmds()
	if len(cmds) != 1 {
		t.Fatalf("expected 1 alter command, got %d", len(cmds))
	}
	cmd := cmds[0].GetAlterTableCmd()
	if cmd == nil {
		t.Fatal("expected AlterTableCmd")
	}
	if cmd.GetSubtype() != pg_query.AlterTableType_AT_SetIdentity {
		t.Fatalf("expected subtype AT_SetIdentity, got %s (%d)", cmd.GetSubtype().String(), cmd.GetSubtype())
	}
	if cmd.GetName() != "id" {
		t.Fatalf("expected column name id, got %q", cmd.GetName())
	}
	listNode := cmd.GetDef().GetList()
	if listNode == nil {
		t.Fatal("expected AT_SetIdentity def to be a List")
	}
	items := listNode.GetItems()
	if len(items) != 1 {
		t.Fatalf("expected 1 defelem item, got %d", len(items))
	}
	defElem := items[0].GetDefElem()
	if defElem == nil {
		t.Fatal("expected first list item to be DefElem")
	}
	if defElem.GetDefname() != "generated" {
		t.Fatalf("expected defname generated, got %q", defElem.GetDefname())
	}
	arg := defElem.GetArg()
	if arg == nil || arg.GetInteger() == nil {
		t.Fatal("expected defelem arg integer for generated setting")
	}
	if arg.GetInteger().GetIval() != 100 {
		t.Fatalf("expected integer 100 for BY DEFAULT, got %d", arg.GetInteger().GetIval())
	}
}

func TestParseAlterTableSetIdentityGeneratedAlwaysAST(t *testing.T) {
	t.Parallel()
	sql := `ALTER TABLE users
  ALTER COLUMN id SET GENERATED ALWAYS;`

	stmt := parseAlterTableStmtAST(t, sql)
	if stmt == nil {
		t.Fatal("expected AlterTableStmt")
	}
	cmds := stmt.GetCmds()
	if len(cmds) != 1 {
		t.Fatalf("expected 1 alter command, got %d", len(cmds))
	}
	cmd := cmds[0].GetAlterTableCmd()
	if cmd == nil {
		t.Fatal("expected AlterTableCmd")
	}
	if cmd.GetSubtype() != pg_query.AlterTableType_AT_SetIdentity {
		t.Fatalf("expected subtype AT_SetIdentity, got %s (%d)", cmd.GetSubtype().String(), cmd.GetSubtype())
	}
	if cmd.GetName() != "id" {
		t.Fatalf("expected column name id, got %q", cmd.GetName())
	}
	listNode := cmd.GetDef().GetList()
	if listNode == nil {
		t.Fatal("expected AT_SetIdentity def to be a List")
	}
	items := listNode.GetItems()
	if len(items) != 1 {
		t.Fatalf("expected 1 defelem item, got %d", len(items))
	}
	defElem := items[0].GetDefElem()
	if defElem == nil {
		t.Fatal("expected first list item to be DefElem")
	}
	if defElem.GetDefname() != "generated" {
		t.Fatalf("expected defname generated, got %q", defElem.GetDefname())
	}
	arg := defElem.GetArg()
	if arg == nil || arg.GetInteger() == nil {
		t.Fatal("expected defelem arg integer for generated setting")
	}
}

func TestParseAlterTableDropIdentityAST(t *testing.T) {
	t.Parallel()
	sql := `ALTER TABLE users
  ALTER COLUMN id DROP IDENTITY;`

	stmt := parseAlterTableStmtAST(t, sql)
	if stmt == nil {
		t.Fatal("expected AlterTableStmt")
	}
	cmds := stmt.GetCmds()
	if len(cmds) != 1 {
		t.Fatalf("expected 1 alter command, got %d", len(cmds))
	}
	cmd := cmds[0].GetAlterTableCmd()
	if cmd == nil {
		t.Fatal("expected AlterTableCmd")
	}
	if cmd.GetSubtype() != pg_query.AlterTableType_AT_DropIdentity {
		t.Fatalf("expected subtype AT_DropIdentity, got %s (%d)", cmd.GetSubtype().String(), cmd.GetSubtype())
	}
	if cmd.GetName() != "id" {
		t.Fatalf("expected column name id, got %q", cmd.GetName())
	}
	if cmd.GetDef() != nil {
		t.Fatalf("expected AT_DropIdentity def to be nil, got %T", cmd.GetDef().GetNode())
	}
}
