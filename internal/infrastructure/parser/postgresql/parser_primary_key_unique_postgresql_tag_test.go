//go:build postgresql

package postgresql

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

func TestParseCreateTableInlinePrimaryKeyAST(t *testing.T) {
	t.Parallel()
	sql := `CREATE TABLE users (id bigint PRIMARY KEY, name text);`
	stmt := parseCreateStmtAST(t, sql)

	elts := stmt.GetTableElts()
	if len(elts) != 2 {
		t.Fatalf("expected 2 table elements, got %d", len(elts))
	}

	colNode := elts[0].GetColumnDef()
	if colNode == nil {
		t.Fatal("first table element is not a ColumnDef")
	}

	// FACT: inline PRIMARY KEY appears as CONSTR_PRIMARY in the column constraint list.
	found := false
	for _, c := range colNode.GetConstraints() {
		con := c.GetConstraint()
		if con != nil && con.GetContype() == pg_query.ConstrType_CONSTR_PRIMARY {
			found = true
			t.Logf("inline CONSTR_PRIMARY found on column %q, conname=%q", colNode.GetColname(), con.GetConname())
			break
		}
	}
	if !found {
		t.Fatal("expected CONSTR_PRIMARY in column constraint list for inline PRIMARY KEY")
	}
}

func TestParseCreateTableNamedTableLevelPrimaryKeyAST(t *testing.T) {
	t.Parallel()
	sql := `CREATE TABLE users (id bigint, CONSTRAINT users_pkey PRIMARY KEY (id));`
	stmt := parseCreateStmtAST(t, sql)

	elts := stmt.GetTableElts()
	if len(elts) != 2 {
		t.Fatalf("expected 2 table elements, got %d", len(elts))
	}

	conNode := elts[1].GetConstraint()
	if conNode == nil {
		t.Fatal("second table element is not a Constraint")
	}

	if conNode.GetContype() != pg_query.ConstrType_CONSTR_PRIMARY {
		t.Fatalf("expected CONSTR_PRIMARY, got %v", conNode.GetContype())
	}
	if conNode.GetConname() != "users_pkey" {
		t.Fatalf("expected conname=users_pkey, got %q", conNode.GetConname())
	}

	keys := conNode.GetKeys()
	if len(keys) != 1 {
		t.Fatalf("expected 1 key column, got %d", len(keys))
	}
	colName := keys[0].GetString_().GetSval()
	if colName != "id" {
		t.Fatalf("expected key column id, got %q", colName)
	}
	t.Logf("named table-level PRIMARY KEY: conname=%q, keys=[%q]", conNode.GetConname(), colName)
}

func TestParseCreateTableUnnamedCompositePrimaryKeyAST(t *testing.T) {
	t.Parallel()
	sql := `CREATE TABLE memberships (tenant_id bigint, user_id bigint, PRIMARY KEY (tenant_id, user_id));`
	stmt := parseCreateStmtAST(t, sql)

	elts := stmt.GetTableElts()
	if len(elts) != 3 {
		t.Fatalf("expected 3 table elements, got %d", len(elts))
	}

	conNode := elts[2].GetConstraint()
	if conNode == nil {
		t.Fatal("third table element is not a Constraint")
	}

	if conNode.GetContype() != pg_query.ConstrType_CONSTR_PRIMARY {
		t.Fatalf("expected CONSTR_PRIMARY, got %v", conNode.GetContype())
	}
	if conNode.GetConname() != "" {
		t.Logf("unnamed composite PK has conname=%q (not empty, noted)", conNode.GetConname())
	}

	keys := conNode.GetKeys()
	if len(keys) != 2 {
		t.Fatalf("expected 2 key columns, got %d", len(keys))
	}
	col0 := keys[0].GetString_().GetSval()
	col1 := keys[1].GetString_().GetSval()
	if col0 != "tenant_id" || col1 != "user_id" {
		t.Fatalf("expected keys [tenant_id, user_id], got [%q, %q]", col0, col1)
	}
	t.Logf("unnamed composite PRIMARY KEY: keys=[%q, %q]", col0, col1)
}

func TestParseCreateTableGeneratedIdentityInlinePrimaryKeyAST(t *testing.T) {
	t.Parallel()
	sql := `CREATE TABLE users (id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY);`
	stmt := parseCreateStmtAST(t, sql)

	elts := stmt.GetTableElts()
	if len(elts) != 1 {
		t.Fatalf("expected 1 table element, got %d", len(elts))
	}

	colNode := elts[0].GetColumnDef()
	if colNode == nil {
		t.Fatal("table element is not a ColumnDef")
	}

	// FACT: both CONSTR_IDENTITY and CONSTR_PRIMARY coexist in the constraint list.
	var hasIdentity, hasPrimary bool
	for _, c := range colNode.GetConstraints() {
		con := c.GetConstraint()
		if con == nil {
			continue
		}
		switch con.GetContype() {
		case pg_query.ConstrType_CONSTR_IDENTITY:
			hasIdentity = true
			t.Logf("CONSTR_IDENTITY found, GeneratedWhen=%q", con.GetGeneratedWhen())
		case pg_query.ConstrType_CONSTR_PRIMARY:
			hasPrimary = true
			t.Logf("CONSTR_PRIMARY found alongside CONSTR_IDENTITY")
		}
	}
	if !hasIdentity {
		t.Fatal("expected CONSTR_IDENTITY in column constraints")
	}
	if !hasPrimary {
		t.Fatal("expected CONSTR_PRIMARY in column constraints for generated identity PRIMARY KEY")
	}
}

// --- Extractor-level: prove DDL.PrimaryKey is currently nil (red tests) ---
// These tests FAIL until the extractor populates DDL.PrimaryKey for PostgreSQL.

func TestExtractCreateTableInlinePrimaryKeyBecomesSupportedFact(t *testing.T) {
	t.Parallel()
	sql := `CREATE TABLE users (id bigint PRIMARY KEY, name text);`

	statement := extractPostgreSQLStatement(t, sql)

	// A. No unsupported.
	if statement.Unsupported != nil {
		t.Fatalf("expected no unsupported detail, got feature=%q reason=%q", statement.Unsupported.Feature, statement.Unsupported.Reason)
	}

	// B. Statement/DDL payload is valid.
	if statement.DDL == nil {
		t.Fatal("expected DDL payload")
	}

	// C. Primary key fact -- currently nil, will be populated by Task 2.
	if statement.DDL.PrimaryKey == nil {
		t.Fatalf("CHARACTERIZATION FAIL: DDL.PrimaryKey is nil for inline PRIMARY KEY; Task 2 must populate it")
	}
	if statement.DDL.PrimaryKey.Kind != spec.IndexKindPrimary {
		t.Fatalf("expected primary key kind %q, got %q", spec.IndexKindPrimary, statement.DDL.PrimaryKey.Kind)
	}
	if len(statement.DDL.PrimaryKey.Columns) != 1 || statement.DDL.PrimaryKey.Columns[0] != "id" {
		t.Fatalf("expected primary key columns [id], got %v", statement.DDL.PrimaryKey.Columns)
	}

	// D. NotNull derivation: PK column must be NotNull, non-PK column must not.
	for _, col := range statement.DDL.Columns {
		if col.Name == "id" && !col.NotNull {
			t.Fatal("expected PK column id to have NotNull=true")
		}
		if col.Name == "name" && col.NotNull {
			t.Fatal("expected non-PK column name to have NotNull=false")
		}
	}
}

func TestExtractCreateTableNamedTableLevelPrimaryKeyBecomesSupportedFact(t *testing.T) {
	t.Parallel()
	sql := `CREATE TABLE users (id bigint, CONSTRAINT users_pkey PRIMARY KEY (id));`

	statement := extractPostgreSQLStatement(t, sql)

	// A. No unsupported.
	if statement.Unsupported != nil {
		t.Fatalf("expected no unsupported detail, got feature=%q reason=%q", statement.Unsupported.Feature, statement.Unsupported.Reason)
	}

	// B. Statement/DDL payload is valid.
	if statement.DDL == nil {
		t.Fatal("expected DDL payload")
	}

	// C. Primary key fact.
	if statement.DDL.PrimaryKey == nil {
		t.Fatalf("CHARACTERIZATION FAIL: DDL.PrimaryKey is nil for named table-level PRIMARY KEY; Task 2 must populate it")
	}
	if statement.DDL.PrimaryKey.Kind != spec.IndexKindPrimary {
		t.Fatalf("expected primary key kind %q, got %q", spec.IndexKindPrimary, statement.DDL.PrimaryKey.Kind)
	}
	if statement.DDL.PrimaryKey.Name != "users_pkey" {
		t.Fatalf("expected primary key name users_pkey, got %q", statement.DDL.PrimaryKey.Name)
	}
	if len(statement.DDL.PrimaryKey.Columns) != 1 || statement.DDL.PrimaryKey.Columns[0] != "id" {
		t.Fatalf("expected primary key columns [id], got %v", statement.DDL.PrimaryKey.Columns)
	}

	// D. NotNull derivation: PK column must be NotNull.
	for _, col := range statement.DDL.Columns {
		if col.Name == "id" && !col.NotNull {
			t.Fatal("expected PK column id to have NotNull=true")
		}
	}
}

func TestExtractCreateTableUnnamedCompositePrimaryKeyBecomesSupportedFact(t *testing.T) {
	t.Parallel()
	sql := `CREATE TABLE memberships (tenant_id bigint, user_id bigint, PRIMARY KEY (tenant_id, user_id));`

	statement := extractPostgreSQLStatement(t, sql)

	// A. No unsupported.
	if statement.Unsupported != nil {
		t.Fatalf("expected no unsupported detail, got feature=%q reason=%q", statement.Unsupported.Feature, statement.Unsupported.Reason)
	}

	// B. Statement/DDL payload is valid.
	if statement.DDL == nil {
		t.Fatal("expected DDL payload")
	}

	// C. Primary key fact.
	if statement.DDL.PrimaryKey == nil {
		t.Fatalf("CHARACTERIZATION FAIL: DDL.PrimaryKey is nil for unnamed composite PRIMARY KEY; Task 2 must populate it")
	}
	if statement.DDL.PrimaryKey.Kind != spec.IndexKindPrimary {
		t.Fatalf("expected primary key kind %q, got %q", spec.IndexKindPrimary, statement.DDL.PrimaryKey.Kind)
	}
	if len(statement.DDL.PrimaryKey.Columns) != 2 {
		t.Fatalf("expected 2 primary key columns, got %d", len(statement.DDL.PrimaryKey.Columns))
	}
	if statement.DDL.PrimaryKey.Columns[0] != "tenant_id" || statement.DDL.PrimaryKey.Columns[1] != "user_id" {
		t.Fatalf("expected primary key columns [tenant_id, user_id], got %v", statement.DDL.PrimaryKey.Columns)
	}

	// D. NotNull derivation: both composite PK columns must be NotNull.
	for _, col := range statement.DDL.Columns {
		if (col.Name == "tenant_id" || col.Name == "user_id") && !col.NotNull {
			t.Fatalf("expected composite PK column %s to have NotNull=true", col.Name)
		}
	}
}

func TestExtractCreateTableGeneratedIdentityInlinePrimaryKeyBecomesSupportedFact(t *testing.T) {
	t.Parallel()
	sql := `CREATE TABLE users (id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY);`

	statement := extractPostgreSQLStatement(t, sql)

	// A. No unsupported.
	if statement.Unsupported != nil {
		t.Fatalf("expected no unsupported detail, got feature=%q reason=%q", statement.Unsupported.Feature, statement.Unsupported.Reason)
	}

	// B. Statement/DDL payload is valid.
	if statement.DDL == nil {
		t.Fatal("expected DDL payload")
	}

	// C. Identity facts still present (no regression).
	var idCol *spec.Column
	for i := range statement.DDL.Columns {
		if statement.DDL.Columns[i].Name == "id" {
			idCol = &statement.DDL.Columns[i]
			break
		}
	}
	if idCol == nil {
		t.Fatal("expected column id in DDL.Columns")
	}
	if idCol.GeneratedWhen != "a" {
		t.Fatalf("expected generated_when=a (ALWAYS), got %q", idCol.GeneratedWhen)
	}
	if !idCol.IsIdentity {
		t.Fatal("expected is_identity=true")
	}

	// D. Primary key fact.
	if statement.DDL.PrimaryKey == nil {
		t.Fatalf("CHARACTERIZATION FAIL: DDL.PrimaryKey is nil for generated identity inline PRIMARY KEY; Task 2 must populate it")
	}
	if statement.DDL.PrimaryKey.Kind != spec.IndexKindPrimary {
		t.Fatalf("expected primary key kind %q, got %q", spec.IndexKindPrimary, statement.DDL.PrimaryKey.Kind)
	}
	if len(statement.DDL.PrimaryKey.Columns) != 1 || statement.DDL.PrimaryKey.Columns[0] != "id" {
		t.Fatalf("expected primary key columns [id], got %v", statement.DDL.PrimaryKey.Columns)
	}

	// E. NotNull derivation: PK column must be NotNull (alongside identity).
	if !idCol.NotNull {
		t.Fatal("expected generated identity PK column id to have NotNull=true")
	}
}

// ---------------------------------------------------------------------------
// v0.39.0 Task 1: AST characterization — ALTER TABLE ADD CONSTRAINT
// ---------------------------------------------------------------------------

func TestASTAlterTableAddPrimaryKeyUnnamed(t *testing.T) {
	t.Parallel()
	stmt := parseAlterTableStmtAST(t, "ALTER TABLE users ADD PRIMARY KEY (id);")
	cmds := stmt.GetCmds()
	if len(cmds) != 1 {
		t.Fatalf("expected 1 cmd, got %d", len(cmds))
	}
	cmd := cmds[0].GetAlterTableCmd()
	if cmd.GetSubtype() != pg_query.AlterTableType_AT_AddConstraint {
		t.Fatalf("expected AT_AddConstraint, got %s", cmd.GetSubtype().String())
	}
	constraint := cmd.GetDef().GetConstraint()
	if constraint == nil {
		t.Fatal("expected constraint node")
	}
	if constraint.GetContype() != pg_query.ConstrType_CONSTR_PRIMARY {
		t.Fatalf("expected CONSTR_PRIMARY, got %s", constraint.GetContype().String())
	}
	if constraint.GetConname() != "" {
		t.Fatalf("expected empty constraint name for unnamed PK, got %q", constraint.GetConname())
	}
	keys := constraint.GetKeys()
	if len(keys) != 1 {
		t.Fatalf("expected 1 key column, got %d", len(keys))
	}
	if keys[0].GetString_().GetSval() != "id" {
		t.Fatalf("expected key column id, got %q", keys[0].GetString_().GetSval())
	}
	t.Logf("SkipValidation=%v", constraint.GetSkipValidation())
}

func TestASTAlterTableAddNamedPrimaryKey(t *testing.T) {
	t.Parallel()
	stmt := parseAlterTableStmtAST(t, "ALTER TABLE users ADD CONSTRAINT users_pkey PRIMARY KEY (id);")
	cmds := stmt.GetCmds()
	if len(cmds) != 1 {
		t.Fatalf("expected 1 cmd, got %d", len(cmds))
	}
	cmd := cmds[0].GetAlterTableCmd()
	if cmd.GetSubtype() != pg_query.AlterTableType_AT_AddConstraint {
		t.Fatalf("expected AT_AddConstraint, got %s", cmd.GetSubtype().String())
	}
	constraint := cmd.GetDef().GetConstraint()
	if constraint == nil {
		t.Fatal("expected constraint node")
	}
	if constraint.GetContype() != pg_query.ConstrType_CONSTR_PRIMARY {
		t.Fatalf("expected CONSTR_PRIMARY, got %s", constraint.GetContype().String())
	}
	if constraint.GetConname() != "users_pkey" {
		t.Fatalf("expected constraint name users_pkey, got %q", constraint.GetConname())
	}
	keys := constraint.GetKeys()
	if len(keys) != 1 {
		t.Fatalf("expected 1 key column, got %d", len(keys))
	}
	if keys[0].GetString_().GetSval() != "id" {
		t.Fatalf("expected key column id, got %q", keys[0].GetString_().GetSval())
	}
	t.Logf("SkipValidation=%v", constraint.GetSkipValidation())
}

func TestASTAlterTableAddUniqueUnnamed(t *testing.T) {
	t.Parallel()
	stmt := parseAlterTableStmtAST(t, "ALTER TABLE users ADD UNIQUE (email);")
	cmds := stmt.GetCmds()
	if len(cmds) != 1 {
		t.Fatalf("expected 1 cmd, got %d", len(cmds))
	}
	cmd := cmds[0].GetAlterTableCmd()
	if cmd.GetSubtype() != pg_query.AlterTableType_AT_AddConstraint {
		t.Fatalf("expected AT_AddConstraint, got %s", cmd.GetSubtype().String())
	}
	constraint := cmd.GetDef().GetConstraint()
	if constraint == nil {
		t.Fatal("expected constraint node")
	}
	if constraint.GetContype() != pg_query.ConstrType_CONSTR_UNIQUE {
		t.Fatalf("expected CONSTR_UNIQUE, got %s", constraint.GetContype().String())
	}
	if constraint.GetConname() != "" {
		t.Fatalf("expected empty constraint name for unnamed unique, got %q", constraint.GetConname())
	}
	keys := constraint.GetKeys()
	if len(keys) != 1 {
		t.Fatalf("expected 1 key column, got %d", len(keys))
	}
	if keys[0].GetString_().GetSval() != "email" {
		t.Fatalf("expected key column email, got %q", keys[0].GetString_().GetSval())
	}
	t.Logf("SkipValidation=%v", constraint.GetSkipValidation())
}

func TestASTAlterTableAddNamedUnique(t *testing.T) {
	t.Parallel()
	stmt := parseAlterTableStmtAST(t, "ALTER TABLE users ADD CONSTRAINT users_email_key UNIQUE (email);")
	cmds := stmt.GetCmds()
	if len(cmds) != 1 {
		t.Fatalf("expected 1 cmd, got %d", len(cmds))
	}
	cmd := cmds[0].GetAlterTableCmd()
	if cmd.GetSubtype() != pg_query.AlterTableType_AT_AddConstraint {
		t.Fatalf("expected AT_AddConstraint, got %s", cmd.GetSubtype().String())
	}
	constraint := cmd.GetDef().GetConstraint()
	if constraint == nil {
		t.Fatal("expected constraint node")
	}
	if constraint.GetContype() != pg_query.ConstrType_CONSTR_UNIQUE {
		t.Fatalf("expected CONSTR_UNIQUE, got %s", constraint.GetContype().String())
	}
	if constraint.GetConname() != "users_email_key" {
		t.Fatalf("expected constraint name users_email_key, got %q", constraint.GetConname())
	}
	keys := constraint.GetKeys()
	if len(keys) != 1 {
		t.Fatalf("expected 1 key column, got %d", len(keys))
	}
	if keys[0].GetString_().GetSval() != "email" {
		t.Fatalf("expected key column email, got %q", keys[0].GetString_().GetSval())
	}
	t.Logf("SkipValidation=%v", constraint.GetSkipValidation())
}

func TestASTAlterTableAddNamedCompositePrimaryKey(t *testing.T) {
	t.Parallel()
	stmt := parseAlterTableStmtAST(t, "ALTER TABLE memberships ADD CONSTRAINT memberships_pkey PRIMARY KEY (tenant_id, user_id);")
	cmds := stmt.GetCmds()
	if len(cmds) != 1 {
		t.Fatalf("expected 1 cmd, got %d", len(cmds))
	}
	cmd := cmds[0].GetAlterTableCmd()
	if cmd.GetSubtype() != pg_query.AlterTableType_AT_AddConstraint {
		t.Fatalf("expected AT_AddConstraint, got %s", cmd.GetSubtype().String())
	}
	constraint := cmd.GetDef().GetConstraint()
	if constraint == nil {
		t.Fatal("expected constraint node")
	}
	if constraint.GetContype() != pg_query.ConstrType_CONSTR_PRIMARY {
		t.Fatalf("expected CONSTR_PRIMARY, got %s", constraint.GetContype().String())
	}
	if constraint.GetConname() != "memberships_pkey" {
		t.Fatalf("expected constraint name memberships_pkey, got %q", constraint.GetConname())
	}
	keys := constraint.GetKeys()
	if len(keys) != 2 {
		t.Fatalf("expected 2 key columns, got %d", len(keys))
	}
	if keys[0].GetString_().GetSval() != "tenant_id" {
		t.Fatalf("expected key column 0 tenant_id, got %q", keys[0].GetString_().GetSval())
	}
	if keys[1].GetString_().GetSval() != "user_id" {
		t.Fatalf("expected key column 1 user_id, got %q", keys[1].GetString_().GetSval())
	}
	t.Logf("SkipValidation=%v", constraint.GetSkipValidation())
}

func TestASTAlterTableAddNamedCompositeUnique(t *testing.T) {
	t.Parallel()
	stmt := parseAlterTableStmtAST(t, "ALTER TABLE memberships ADD CONSTRAINT memberships_user_key UNIQUE (tenant_id, user_id);")
	cmds := stmt.GetCmds()
	if len(cmds) != 1 {
		t.Fatalf("expected 1 cmd, got %d", len(cmds))
	}
	cmd := cmds[0].GetAlterTableCmd()
	if cmd.GetSubtype() != pg_query.AlterTableType_AT_AddConstraint {
		t.Fatalf("expected AT_AddConstraint, got %s", cmd.GetSubtype().String())
	}
	constraint := cmd.GetDef().GetConstraint()
	if constraint == nil {
		t.Fatal("expected constraint node")
	}
	if constraint.GetContype() != pg_query.ConstrType_CONSTR_UNIQUE {
		t.Fatalf("expected CONSTR_UNIQUE, got %s", constraint.GetContype().String())
	}
	if constraint.GetConname() != "memberships_user_key" {
		t.Fatalf("expected constraint name memberships_user_key, got %q", constraint.GetConname())
	}
	keys := constraint.GetKeys()
	if len(keys) != 2 {
		t.Fatalf("expected 2 key columns, got %d", len(keys))
	}
	if keys[0].GetString_().GetSval() != "tenant_id" {
		t.Fatalf("expected key column 0 tenant_id, got %q", keys[0].GetString_().GetSval())
	}
	if keys[1].GetString_().GetSval() != "user_id" {
		t.Fatalf("expected key column 1 user_id, got %q", keys[1].GetString_().GetSval())
	}
	t.Logf("SkipValidation=%v", constraint.GetSkipValidation())
}

// ---------------------------------------------------------------------------
// v0.39.0 Task 1: Extractor red tests — ALTER TABLE ADD CONSTRAINT
// These tests assert current supported status and characterize the exact
// fact gap: constraint columns are not preserved in spec.Alter.Options.
// Task 2 must make these assertions pass without changing public spec.
// ---------------------------------------------------------------------------

func TestExtractAlterTableAddNamedPrimaryKeyBecomesSupportedFact(t *testing.T) {
	t.Parallel()
	statement := extractPostgreSQLStatement(t, "ALTER TABLE users ADD CONSTRAINT users_pkey PRIMARY KEY (id);")

	if statement.Unsupported != nil {
		t.Fatalf("expected supported statement, got unsupported feature=%q reason=%q", statement.Unsupported.Feature, statement.Unsupported.Reason)
	}
	if statement.DDL == nil || statement.DDL.Operation != spec.DDLOperationAlterTable {
		t.Fatalf("expected alter_table DDL, got %#v", statement.DDL)
	}
	if len(statement.DDL.Alter) != 1 {
		t.Fatalf("expected 1 alter action, got %d", len(statement.DDL.Alter))
	}
	alter := statement.DDL.Alter[0]
	if alter.Action != "add_constraint" {
		t.Fatalf("expected action add_constraint, got %q", alter.Action)
	}
	if alter.Name != "users_pkey" {
		t.Fatalf("expected name users_pkey, got %q", alter.Name)
	}
	if alter.Options["constraint_type"] != "primary_key" {
		t.Fatalf("expected constraint_type=primary_key, got %q", alter.Options["constraint_type"])
	}

	// RED ASSERTION: columns are not currently preserved.
	// Task 2 must populate Options["columns"] = "id" for this to pass.
	if alter.Options["columns"] != "id" {
		t.Fatalf("RED: expected Options[columns]=id (Task 2 must populate), got %q", alter.Options["columns"])
	}
}

func TestExtractAlterTableAddNamedUniqueBecomesSupportedFact(t *testing.T) {
	t.Parallel()
	statement := extractPostgreSQLStatement(t, "ALTER TABLE users ADD CONSTRAINT bad_email_key UNIQUE (email);")

	if statement.Unsupported != nil {
		t.Fatalf("expected supported statement, got unsupported feature=%q reason=%q", statement.Unsupported.Feature, statement.Unsupported.Reason)
	}
	if statement.DDL == nil || statement.DDL.Operation != spec.DDLOperationAlterTable {
		t.Fatalf("expected alter_table DDL, got %#v", statement.DDL)
	}
	if len(statement.DDL.Alter) != 1 {
		t.Fatalf("expected 1 alter action, got %d", len(statement.DDL.Alter))
	}
	alter := statement.DDL.Alter[0]
	if alter.Action != "add_constraint" {
		t.Fatalf("expected action add_constraint, got %q", alter.Action)
	}
	if alter.Name != "bad_email_key" {
		t.Fatalf("expected name bad_email_key, got %q", alter.Name)
	}
	if alter.Options["constraint_type"] != "unique" {
		t.Fatalf("expected constraint_type=unique, got %q", alter.Options["constraint_type"])
	}

	// RED ASSERTION: columns are not currently preserved.
	if alter.Options["columns"] != "email" {
		t.Fatalf("RED: expected Options[columns]=email (Task 2 must populate), got %q", alter.Options["columns"])
	}
}

func TestExtractAlterTableAddCompositePrimaryKeyBecomesSupportedFact(t *testing.T) {
	t.Parallel()
	statement := extractPostgreSQLStatement(t, "ALTER TABLE memberships ADD CONSTRAINT memberships_pkey PRIMARY KEY (tenant_id, user_id);")

	if statement.Unsupported != nil {
		t.Fatalf("expected supported statement, got unsupported feature=%q", statement.Unsupported.Feature)
	}
	alter := statement.DDL.Alter[0]
	if alter.Options["constraint_type"] != "primary_key" {
		t.Fatalf("expected constraint_type=primary_key, got %q", alter.Options["constraint_type"])
	}

	// RED ASSERTION: composite columns must be comma-separated.
	if alter.Options["columns"] != "tenant_id,user_id" {
		t.Fatalf("RED: expected Options[columns]=tenant_id,user_id (Task 2 must populate), got %q", alter.Options["columns"])
	}
}

func TestExtractAlterTableAddCompositeUniqueBecomesSupportedFact(t *testing.T) {
	t.Parallel()
	statement := extractPostgreSQLStatement(t, "ALTER TABLE memberships ADD CONSTRAINT memberships_user_key UNIQUE (tenant_id, user_id);")

	if statement.Unsupported != nil {
		t.Fatalf("expected supported statement, got unsupported feature=%q", statement.Unsupported.Feature)
	}
	alter := statement.DDL.Alter[0]
	if alter.Options["constraint_type"] != "unique" {
		t.Fatalf("expected constraint_type=unique, got %q", alter.Options["constraint_type"])
	}

	// RED ASSERTION: composite columns must be comma-separated.
	if alter.Options["columns"] != "tenant_id,user_id" {
		t.Fatalf("RED: expected Options[columns]=tenant_id,user_id (Task 2 must populate), got %q", alter.Options["columns"])
	}
}

// ---------------------------------------------------------------------------
// v0.40.0 Task 1: AST characterization — ALTER TABLE ADD CONSTRAINT FOREIGN KEY
// ---------------------------------------------------------------------------
