//go:build postgresql

package postgresql

import (
	"reflect"
	"sort"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

func TestASTAlterTableAddNamedForeignKey(t *testing.T) {
	t.Parallel()
	stmt := parseAlterTableStmtAST(t,
		"ALTER TABLE orders ADD CONSTRAINT fk_orders_user FOREIGN KEY (user_id) REFERENCES users(id);")
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

	// FACT 1: constraint type is CONSTR_FOREIGN.
	if constraint.GetContype() != pg_query.ConstrType_CONSTR_FOREIGN {
		t.Fatalf("expected CONSTR_FOREIGN, got %s", constraint.GetContype().String())
	}

	// FACT 2: constraint name.
	if constraint.GetConname() != "fk_orders_user" {
		t.Fatalf("expected constraint name fk_orders_user, got %q", constraint.GetConname())
	}

	// FACT 3: local columns (fk_attrs).
	fkAttrs := constraint.GetFkAttrs()
	if len(fkAttrs) != 1 {
		t.Fatalf("expected 1 fk_attr, got %d", len(fkAttrs))
	}
	if fkAttrs[0].GetString_().GetSval() != "user_id" {
		t.Fatalf("expected fk_attr user_id, got %q", fkAttrs[0].GetString_().GetSval())
	}

	// FACT 4: referenced table (pktable).
	pkTable := constraint.GetPktable()
	if pkTable == nil {
		t.Fatal("expected pktable node")
	}
	if pkTable.GetRelname() != "users" {
		t.Fatalf("expected pktable relname users, got %q", pkTable.GetRelname())
	}

	// FACT 5: referenced columns (pk_attrs).
	pkAttrs := constraint.GetPkAttrs()
	if len(pkAttrs) != 1 {
		t.Fatalf("expected 1 pk_attr, got %d", len(pkAttrs))
	}
	if pkAttrs[0].GetString_().GetSval() != "id" {
		t.Fatalf("expected pk_attr id, got %q", pkAttrs[0].GetString_().GetSval())
	}

	// FACT 6: skip_validation (NOT VALID).
	t.Logf("SkipValidation=%v", constraint.GetSkipValidation())

	// FACT 7: confdeltype / confupdtype / matchtype.
	t.Logf("FkDelAction=%q FkUpdAction=%q FkMatchtype=%q",
		constraint.GetFkDelAction(), constraint.GetFkUpdAction(), constraint.GetFkMatchtype())

	// FACT 8: deferrable / initially.
	t.Logf("Deferrable=%v Initdeferred=%v", constraint.GetDeferrable(), constraint.GetInitdeferred())
}

func TestASTAlterTableAddUnnamedForeignKey(t *testing.T) {
	t.Parallel()
	stmt := parseAlterTableStmtAST(t,
		"ALTER TABLE orders ADD FOREIGN KEY (user_id) REFERENCES users(id);")
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

	// FACT 1: CONSTR_FOREIGN.
	if constraint.GetContype() != pg_query.ConstrType_CONSTR_FOREIGN {
		t.Fatalf("expected CONSTR_FOREIGN, got %s", constraint.GetContype().String())
	}

	// FACT 2: unnamed constraint has empty conname.
	if constraint.GetConname() != "" {
		t.Fatalf("expected empty constraint name for unnamed FK, got %q", constraint.GetConname())
	}

	// FACT 3: local columns.
	fkAttrs := constraint.GetFkAttrs()
	if len(fkAttrs) != 1 || fkAttrs[0].GetString_().GetSval() != "user_id" {
		t.Fatalf("expected fk_attrs [user_id], got %v", stringValuesFromNodesForTest(fkAttrs))
	}

	// FACT 4: referenced table.
	pkTable := constraint.GetPktable()
	if pkTable == nil || pkTable.GetRelname() != "users" {
		t.Fatalf("expected pktable users, got %v", pkTable)
	}

	// FACT 5: referenced columns.
	pkAttrs := constraint.GetPkAttrs()
	if len(pkAttrs) != 1 || pkAttrs[0].GetString_().GetSval() != "id" {
		t.Fatalf("expected pk_attrs [id], got %v", stringValuesFromNodesForTest(pkAttrs))
	}
}

func TestASTAlterTableAddCompositeForeignKey(t *testing.T) {
	t.Parallel()
	stmt := parseAlterTableStmtAST(t,
		"ALTER TABLE orders ADD CONSTRAINT fk_orders_user FOREIGN KEY (tenant_id, user_id) REFERENCES users(tenant_id, id);")
	cmds := stmt.GetCmds()
	if len(cmds) != 1 {
		t.Fatalf("expected 1 cmd, got %d", len(cmds))
	}
	cmd := cmds[0].GetAlterTableCmd()
	constraint := cmd.GetDef().GetConstraint()
	if constraint == nil {
		t.Fatal("expected constraint node")
	}
	if constraint.GetContype() != pg_query.ConstrType_CONSTR_FOREIGN {
		t.Fatalf("expected CONSTR_FOREIGN, got %s", constraint.GetContype().String())
	}

	// FACT: composite local columns.
	fkAttrs := constraint.GetFkAttrs()
	if len(fkAttrs) != 2 {
		t.Fatalf("expected 2 fk_attrs, got %d", len(fkAttrs))
	}
	if fkAttrs[0].GetString_().GetSval() != "tenant_id" {
		t.Fatalf("expected fk_attr 0 tenant_id, got %q", fkAttrs[0].GetString_().GetSval())
	}
	if fkAttrs[1].GetString_().GetSval() != "user_id" {
		t.Fatalf("expected fk_attr 1 user_id, got %q", fkAttrs[1].GetString_().GetSval())
	}

	// FACT: composite referenced columns.
	pkAttrs := constraint.GetPkAttrs()
	if len(pkAttrs) != 2 {
		t.Fatalf("expected 2 pk_attrs, got %d", len(pkAttrs))
	}
	if pkAttrs[0].GetString_().GetSval() != "tenant_id" {
		t.Fatalf("expected pk_attr 0 tenant_id, got %q", pkAttrs[0].GetString_().GetSval())
	}
	if pkAttrs[1].GetString_().GetSval() != "id" {
		t.Fatalf("expected pk_attr 1 id, got %q", pkAttrs[1].GetString_().GetSval())
	}
}

func TestASTAlterTableAddForeignKeyWithOnDeleteCascade(t *testing.T) {
	t.Parallel()
	stmt := parseAlterTableStmtAST(t,
		"ALTER TABLE orders ADD CONSTRAINT fk_orders_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;")
	cmds := stmt.GetCmds()
	cmd := cmds[0].GetAlterTableCmd()
	constraint := cmd.GetDef().GetConstraint()

	// FACT: FkDelAction encodes CASCADE (non-empty string).
	t.Logf("FkDelAction=%q", constraint.GetFkDelAction())
	if constraint.GetFkDelAction() == "" {
		t.Fatal("expected non-empty FkDelAction for ON DELETE CASCADE")
	}
}

func TestASTAlterTableAddForeignKeyWithOnUpdateRestrict(t *testing.T) {
	t.Parallel()
	stmt := parseAlterTableStmtAST(t,
		"ALTER TABLE orders ADD CONSTRAINT fk_orders_user FOREIGN KEY (user_id) REFERENCES users(id) ON UPDATE RESTRICT;")
	cmds := stmt.GetCmds()
	cmd := cmds[0].GetAlterTableCmd()
	constraint := cmd.GetDef().GetConstraint()

	// FACT: FkUpdAction encodes RESTRICT (non-empty string).
	t.Logf("FkUpdAction=%q", constraint.GetFkUpdAction())
	if constraint.GetFkUpdAction() == "" {
		t.Fatal("expected non-empty FkUpdAction for ON UPDATE RESTRICT")
	}
}

func TestASTAlterTableAddForeignKeyWithNotValid(t *testing.T) {
	t.Parallel()
	stmt := parseAlterTableStmtAST(t,
		"ALTER TABLE orders ADD CONSTRAINT fk_orders_user FOREIGN KEY (user_id) REFERENCES users(id) NOT VALID;")
	cmds := stmt.GetCmds()
	cmd := cmds[0].GetAlterTableCmd()
	constraint := cmd.GetDef().GetConstraint()

	// FACT: SkipValidation is true for NOT VALID.
	if !constraint.GetSkipValidation() {
		t.Fatal("expected SkipValidation=true for NOT VALID")
	}
}

func TestASTAlterTableAddForeignKeySchemaQualifiedReference(t *testing.T) {
	t.Parallel()
	stmt := parseAlterTableStmtAST(t,
		"ALTER TABLE orders ADD CONSTRAINT fk_orders_user FOREIGN KEY (user_id) REFERENCES public.users(id);")
	cmds := stmt.GetCmds()
	cmd := cmds[0].GetAlterTableCmd()
	constraint := cmd.GetDef().GetConstraint()

	// FACT: pktable carries schema.
	pkTable := constraint.GetPktable()
	if pkTable == nil {
		t.Fatal("expected pktable node")
	}
	if pkTable.GetSchemaname() != "public" {
		t.Fatalf("expected pktable schemaname public, got %q", pkTable.GetSchemaname())
	}
	if pkTable.GetRelname() != "users" {
		t.Fatalf("expected pktable relname users, got %q", pkTable.GetRelname())
	}
}

func TestASTAlterTableAddForeignKeyDeferrable(t *testing.T) {
	t.Parallel()
	stmt := parseAlterTableStmtAST(t,
		"ALTER TABLE orders ADD CONSTRAINT fk_orders_user FOREIGN KEY (user_id) REFERENCES users(id) DEFERRABLE INITIALLY DEFERRED;")
	cmds := stmt.GetCmds()
	cmd := cmds[0].GetAlterTableCmd()
	constraint := cmd.GetDef().GetConstraint()

	// FACT: deferrable / initially deferred exposed as typed fields.
	t.Logf("Deferrable=%v Initdeferred=%v", constraint.GetDeferrable(), constraint.GetInitdeferred())
	// Record stability; do not hard-assert until v0.40.0 scope decision.
}

// stringValuesFromNodesForTest is a test-only helper for readable FK assertion errors.
func stringValuesFromNodesForTest(nodes []*pg_query.Node) []string {
	values := make([]string, 0, len(nodes))
	for _, n := range nodes {
		if s := n.GetString_(); s != nil {
			values = append(values, s.GetSval())
		}
	}
	return values
}

// ---------------------------------------------------------------------------
// v0.40.0 Task 1: Extractor red tests — ALTER TABLE ADD CONSTRAINT FOREIGN KEY
// These tests assert current status and characterize the FK fact gap.
// Task 2 must make these pass without changing public spec contract.
// ---------------------------------------------------------------------------

func TestExtractAlterTableAddNamedForeignKeyBecomesSupportedFact(t *testing.T) {
	t.Parallel()
	statement := extractPostgreSQLStatement(t,
		"ALTER TABLE orders ADD CONSTRAINT fk_orders_user FOREIGN KEY (user_id) REFERENCES users(id);")

	if statement.Unsupported != nil {
		t.Fatalf("expected supported statement, got unsupported feature=%q reason=%q",
			statement.Unsupported.Feature, statement.Unsupported.Reason)
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
	if alter.Name != "fk_orders_user" {
		t.Fatalf("expected name fk_orders_user, got %q", alter.Name)
	}
	if alter.Options["constraint_type"] != "foreign_key" {
		t.Fatalf("expected constraint_type=foreign_key, got %q", alter.Options["constraint_type"])
	}

	// RED ASSERTION: local columns must be populated.
	if alter.Options["columns"] != "user_id" {
		t.Fatalf("RED: expected Options[columns]=user_id (Task 2 must populate), got %q", alter.Options["columns"])
	}

	// RED ASSERTION: referenced table must be populated.
	if alter.Options["referenced_table"] != "users" {
		t.Fatalf("RED: expected Options[referenced_table]=users (Task 2 must populate), got %q", alter.Options["referenced_table"])
	}

	// RED ASSERTION: referenced columns must be populated.
	if alter.Options["referenced_columns"] != "id" {
		t.Fatalf("RED: expected Options[referenced_columns]=id (Task 2 must populate), got %q", alter.Options["referenced_columns"])
	}

	// RED ASSERTION: FK must be projected to DDL.Constraints for existing rules.
	if len(statement.DDL.Constraints) == 0 {
		t.Fatal("RED: expected DDL.Constraints to contain projected FK (Task 2 must project)")
	}
	fk := statement.DDL.Constraints[0]
	if fk.Type != "foreign_key" {
		t.Fatalf("RED: expected constraint type foreign_key, got %q", fk.Type)
	}
	if fk.Name != "fk_orders_user" {
		t.Fatalf("RED: expected constraint name fk_orders_user, got %q", fk.Name)
	}
	if len(fk.Columns) != 1 || fk.Columns[0] != "user_id" {
		t.Fatalf("RED: expected constraint columns [user_id], got %v", fk.Columns)
	}
	if fk.ReferencedTable != "users" {
		t.Fatalf("RED: expected referenced table users, got %q", fk.ReferencedTable)
	}
	if len(fk.ReferencedColumns) != 1 || fk.ReferencedColumns[0] != "id" {
		t.Fatalf("RED: expected referenced columns [id], got %v", fk.ReferencedColumns)
	}
}

func TestExtractAlterTableAddUnnamedForeignKeyBecomesSupportedFact(t *testing.T) {
	t.Parallel()
	statement := extractPostgreSQLStatement(t,
		"ALTER TABLE orders ADD FOREIGN KEY (user_id) REFERENCES users(id);")

	if statement.Unsupported != nil {
		t.Fatalf("expected supported statement, got unsupported feature=%q reason=%q",
			statement.Unsupported.Feature, statement.Unsupported.Reason)
	}

	alter := statement.DDL.Alter[0]
	if alter.Options["constraint_type"] != "foreign_key" {
		t.Fatalf("expected constraint_type=foreign_key, got %q", alter.Options["constraint_type"])
	}

	// RED ASSERTION: unnamed FK has empty name but still populated columns.
	if alter.Options["columns"] != "user_id" {
		t.Fatalf("RED: expected Options[columns]=user_id, got %q", alter.Options["columns"])
	}

	// RED: projection to DDL.Constraints.
	if len(statement.DDL.Constraints) == 0 {
		t.Fatal("RED: expected DDL.Constraints to contain projected unnamed FK")
	}
}

func TestExtractAlterTableAddCompositeForeignKeyBecomesSupportedFact(t *testing.T) {
	t.Parallel()
	statement := extractPostgreSQLStatement(t,
		"ALTER TABLE orders ADD CONSTRAINT fk_orders_user FOREIGN KEY (tenant_id, user_id) REFERENCES users(tenant_id, id);")

	if statement.Unsupported != nil {
		t.Fatalf("expected supported statement, got unsupported feature=%q",
			statement.Unsupported.Feature)
	}

	alter := statement.DDL.Alter[0]

	// RED: composite local columns.
	if alter.Options["columns"] != "tenant_id,user_id" {
		t.Fatalf("RED: expected Options[columns]=tenant_id,user_id, got %q", alter.Options["columns"])
	}

	// RED: projection to DDL.Constraints.
	if len(statement.DDL.Constraints) == 0 {
		t.Fatal("RED: expected DDL.Constraints to contain projected composite FK")
	}
	fk := statement.DDL.Constraints[0]
	if len(fk.Columns) != 2 || fk.Columns[0] != "tenant_id" || fk.Columns[1] != "user_id" {
		t.Fatalf("RED: expected constraint columns [tenant_id,user_id], got %v", fk.Columns)
	}
	if len(fk.ReferencedColumns) != 2 || fk.ReferencedColumns[0] != "tenant_id" || fk.ReferencedColumns[1] != "id" {
		t.Fatalf("RED: expected referenced columns [tenant_id,id], got %v", fk.ReferencedColumns)
	}
}

func TestExtractAlterTableAddForeignKeySchemaQualifiedReference(t *testing.T) {
	t.Parallel()
	statement := extractPostgreSQLStatement(t,
		"ALTER TABLE orders ADD CONSTRAINT fk_orders_user FOREIGN KEY (user_id) REFERENCES public.users(id);")

	if statement.Unsupported != nil {
		t.Fatalf("expected supported statement, got unsupported feature=%q",
			statement.Unsupported.Feature)
	}

	// RED: schema-qualified reference must preserve schema.
	if len(statement.DDL.Constraints) == 0 {
		t.Fatal("RED: expected DDL.Constraints to contain projected FK with schema")
	}
	fk := statement.DDL.Constraints[0]
	if fk.ReferencedSchema != "public" {
		t.Fatalf("RED: expected referenced schema public, got %q", fk.ReferencedSchema)
	}
}

// ---------------------------------------------------------------------------
// v0.41.0 Task 1: AST characterization + extractor red tests
// for PostgreSQL ALTER TABLE ... ADD CONSTRAINT ... CHECK
// ---------------------------------------------------------------------------

// --- AST characterization tests ---

func TestAlterTableAddCheckASTCharacterize_NamedCheck(t *testing.T) {
	t.Parallel()
	ast := parseAlterTableStmtAST(t,
		"ALTER TABLE orders ADD CONSTRAINT chk_orders_amount CHECK (amount >= 0);")

	cmds := ast.GetCmds()
	if len(cmds) == 0 {
		t.Fatal("expected at least one AlterTableCmd")
	}
	cmd := cmds[0].GetAlterTableCmd()
	if cmd == nil {
		t.Fatal("expected AlterTableCmd node")
	}
	if cmd.GetSubtype() != pg_query.AlterTableType_AT_AddConstraint {
		t.Fatalf("expected AT_AddConstraint, got %v", cmd.GetSubtype())
	}
	constraint := cmd.GetDef().GetConstraint()
	if constraint == nil {
		t.Fatal("expected Constraint node")
	}
	if constraint.GetContype() != pg_query.ConstrType_CONSTR_CHECK {
		t.Fatalf("expected CONSTR_CHECK, got %v", constraint.GetContype())
	}
	if constraint.GetConname() != "chk_orders_amount" {
		t.Fatalf("expected conname chk_orders_amount, got %q", constraint.GetConname())
	}
	if constraint.GetSkipValidation() {
		t.Fatal("expected skipValidation=false for check without NOT VALID")
	}
	if constraint.GetRawExpr() == nil {
		t.Fatal("expected non-nil RawExpr")
	}
	cols := columnRefsFromExpr(constraint.GetRawExpr())
	sort.Strings(cols)
	if !reflect.DeepEqual(cols, []string{"amount"}) {
		t.Fatalf("expected column refs [amount], got %v", cols)
	}
}

func TestAlterTableAddCheckASTCharacterize_UnnamedCheck(t *testing.T) {
	t.Parallel()
	ast := parseAlterTableStmtAST(t,
		"ALTER TABLE orders ADD CHECK (amount >= 0);")

	cmds := ast.GetCmds()
	if len(cmds) == 0 {
		t.Fatal("expected at least one AlterTableCmd")
	}
	cmd := cmds[0].GetAlterTableCmd()
	if cmd.GetSubtype() != pg_query.AlterTableType_AT_AddConstraint {
		t.Fatalf("expected AT_AddConstraint, got %v", cmd.GetSubtype())
	}
	constraint := cmd.GetDef().GetConstraint()
	if constraint.GetContype() != pg_query.ConstrType_CONSTR_CHECK {
		t.Fatalf("expected CONSTR_CHECK, got %v", constraint.GetContype())
	}
	if constraint.GetConname() != "" {
		t.Fatalf("expected empty conname for unnamed check, got %q", constraint.GetConname())
	}
	if constraint.GetRawExpr() == nil {
		t.Fatal("expected non-nil RawExpr for unnamed check")
	}
	cols := columnRefsFromExpr(constraint.GetRawExpr())
	sort.Strings(cols)
	if !reflect.DeepEqual(cols, []string{"amount"}) {
		t.Fatalf("expected column refs [amount], got %v", cols)
	}
}

func TestAlterTableAddCheckASTCharacterize_NotValid(t *testing.T) {
	t.Parallel()
	ast := parseAlterTableStmtAST(t,
		"ALTER TABLE orders ADD CONSTRAINT chk_orders_amount CHECK (amount >= 0) NOT VALID;")

	cmds := ast.GetCmds()
	cmd := cmds[0].GetAlterTableCmd()
	constraint := cmd.GetDef().GetConstraint()
	if constraint.GetContype() != pg_query.ConstrType_CONSTR_CHECK {
		t.Fatalf("expected CONSTR_CHECK, got %v", constraint.GetContype())
	}
	if constraint.GetConname() != "chk_orders_amount" {
		t.Fatalf("expected conname chk_orders_amount, got %q", constraint.GetConname())
	}
	if !constraint.GetSkipValidation() {
		t.Fatal("expected skipValidation=true for NOT VALID")
	}
	if constraint.GetRawExpr() == nil {
		t.Fatal("expected non-nil RawExpr")
	}
	cols := columnRefsFromExpr(constraint.GetRawExpr())
	sort.Strings(cols)
	if !reflect.DeepEqual(cols, []string{"amount"}) {
		t.Fatalf("expected column refs [amount], got %v", cols)
	}
}

func TestAlterTableAddCheckASTCharacterize_MultiColumnExpr(t *testing.T) {
	t.Parallel()
	ast := parseAlterTableStmtAST(t,
		"ALTER TABLE orders ADD CONSTRAINT chk_orders_total CHECK (amount + tax >= 0);")

	cmds := ast.GetCmds()
	cmd := cmds[0].GetAlterTableCmd()
	constraint := cmd.GetDef().GetConstraint()
	if constraint.GetContype() != pg_query.ConstrType_CONSTR_CHECK {
		t.Fatalf("expected CONSTR_CHECK, got %v", constraint.GetContype())
	}
	if constraint.GetRawExpr() == nil {
		t.Fatal("expected non-nil RawExpr")
	}
	cols := columnRefsFromExpr(constraint.GetRawExpr())
	sort.Strings(cols)
	if !reflect.DeepEqual(cols, []string{"amount", "tax"}) {
		t.Fatalf("expected column refs [amount tax], got %v", cols)
	}
}

func TestAlterTableAddCheckASTCharacterize_EnumListExpr(t *testing.T) {
	t.Parallel()
	ast := parseAlterTableStmtAST(t,
		"ALTER TABLE orders ADD CONSTRAINT chk_orders_status CHECK (status IN ('open', 'closed'));")

	cmds := ast.GetCmds()
	cmd := cmds[0].GetAlterTableCmd()
	constraint := cmd.GetDef().GetConstraint()
	if constraint.GetContype() != pg_query.ConstrType_CONSTR_CHECK {
		t.Fatalf("expected CONSTR_CHECK, got %v", constraint.GetContype())
	}
	if constraint.GetRawExpr() == nil {
		t.Fatal("expected non-nil RawExpr")
	}
	cols := columnRefsFromExpr(constraint.GetRawExpr())
	sort.Strings(cols)
	if !reflect.DeepEqual(cols, []string{"status"}) {
		t.Fatalf("expected column refs [status], got %v", cols)
	}
}

// --- Extractor red tests (expected FAIL until Task 2) ---

func TestExtractAlterTableAddNamedCheckBecomesProjectedConstraintFact(t *testing.T) {
	t.Parallel()
	statement := extractPostgreSQLStatement(t,
		"ALTER TABLE orders ADD CONSTRAINT amount_positive CHECK (amount >= 0);")

	if statement.Unsupported != nil {
		t.Fatalf("expected supported statement, got unsupported %#v", statement.Unsupported)
	}
	if statement.DDL == nil || statement.DDL.Operation != spec.DDLOperationAlterTable {
		t.Fatalf("expected alter table DDL, got %#v", statement.DDL)
	}
	if len(statement.DDL.Alter) != 1 {
		t.Fatalf("expected one alter action, got %d", len(statement.DDL.Alter))
	}
	alter := statement.DDL.Alter[0]
	if alter.Action != "add_constraint" || alter.Name != "amount_positive" {
		t.Fatalf("expected add_constraint amount_positive, got %#v", alter)
	}
	if alter.Options["constraint_type"] != "check" {
		t.Fatalf("expected constraint_type=check, got %#v", alter.Options)
	}

	// RED: columns option not populated for ALTER CHECK (uses GetKeys which is empty for CHECK).
	if alter.Options["columns"] != "amount" {
		t.Fatalf("RED: expected columns=amount, got %q", alter.Options["columns"])
	}

	// RED: DDL.Constraints not projected for ALTER CHECK.
	if len(statement.DDL.Constraints) != 1 {
		t.Fatalf("RED: expected one projected check constraint, got %d", len(statement.DDL.Constraints))
	}
	c := statement.DDL.Constraints[0]
	if c.Type != "check" || c.Name != "amount_positive" {
		t.Fatalf("RED: expected projected check constraint amount_positive, got %#v", c)
	}
	if !reflect.DeepEqual(c.Columns, []string{"amount"}) {
		t.Fatalf("RED: expected projected columns [amount], got %v", c.Columns)
	}
}

func TestExtractAlterTableAddUnnamedCheckBecomesProjectedConstraintFact(t *testing.T) {
	t.Parallel()
	statement := extractPostgreSQLStatement(t,
		"ALTER TABLE orders ADD CHECK (amount >= 0);")

	if statement.Unsupported != nil {
		t.Fatalf("expected supported statement, got unsupported %#v", statement.Unsupported)
	}
	if len(statement.DDL.Alter) != 1 {
		t.Fatalf("expected one alter action, got %d", len(statement.DDL.Alter))
	}
	alter := statement.DDL.Alter[0]
	if alter.Action != "add_constraint" {
		t.Fatalf("expected add_constraint, got %q", alter.Action)
	}
	if alter.Options["constraint_type"] != "check" {
		t.Fatalf("expected constraint_type=check, got %#v", alter.Options)
	}

	// RED: columns option not populated for ALTER CHECK.
	if alter.Options["columns"] != "amount" {
		t.Fatalf("RED: expected columns=amount, got %q", alter.Options["columns"])
	}

	// RED: DDL.Constraints not projected for ALTER CHECK.
	if len(statement.DDL.Constraints) != 1 {
		t.Fatalf("RED: expected one projected check constraint, got %d", len(statement.DDL.Constraints))
	}
	c := statement.DDL.Constraints[0]
	if c.Type != "check" {
		t.Fatalf("RED: expected projected check constraint, got %#v", c)
	}
	// Unnamed checks have empty Name.
	if !reflect.DeepEqual(c.Columns, []string{"amount"}) {
		t.Fatalf("RED: expected projected columns [amount], got %v", c.Columns)
	}
}

func TestExtractAlterTableAddCheckNotValidStillProjectsConstraint(t *testing.T) {
	t.Parallel()
	statement := extractPostgreSQLStatement(t,
		"ALTER TABLE orders ADD CONSTRAINT chk_orders_amount CHECK (amount >= 0) NOT VALID;")

	if statement.Unsupported != nil {
		t.Fatalf("expected supported statement, got unsupported %#v", statement.Unsupported)
	}
	if len(statement.DDL.Alter) != 1 {
		t.Fatalf("expected one alter action, got %d", len(statement.DDL.Alter))
	}
	alter := statement.DDL.Alter[0]

	// Existing behavior: not_valid flag must be preserved.
	if alter.Options["not_valid"] != "true" {
		t.Fatalf("expected not_valid=true, got %q", alter.Options["not_valid"])
	}
	if alter.Options["constraint_type"] != "check" {
		t.Fatalf("expected constraint_type=check, got %q", alter.Options["constraint_type"])
	}

	// RED: columns still not populated even for NOT VALID variant.
	if alter.Options["columns"] != "amount" {
		t.Fatalf("RED: expected columns=amount, got %q", alter.Options["columns"])
	}

	// RED: projection must exist even with NOT VALID.
	if len(statement.DDL.Constraints) != 1 {
		t.Fatalf("RED: expected one projected check constraint, got %d", len(statement.DDL.Constraints))
	}
	c := statement.DDL.Constraints[0]
	if c.Type != "check" || c.Name != "chk_orders_amount" {
		t.Fatalf("RED: expected projected check constraint chk_orders_amount, got %#v", c)
	}
	if !reflect.DeepEqual(c.Columns, []string{"amount"}) {
		t.Fatalf("RED: expected projected columns [amount], got %v", c.Columns)
	}
}

func TestExtractAlterTableAddCheckMultiColumnBecomesProjectedConstraintFact(t *testing.T) {
	t.Parallel()
	statement := extractPostgreSQLStatement(t,
		"ALTER TABLE orders ADD CONSTRAINT chk_orders_total CHECK (amount + tax >= 0);")

	if statement.Unsupported != nil {
		t.Fatalf("expected supported statement, got unsupported %#v", statement.Unsupported)
	}
	if len(statement.DDL.Alter) != 1 {
		t.Fatalf("expected one alter action, got %d", len(statement.DDL.Alter))
	}
	alter := statement.DDL.Alter[0]

	// RED: multi-column expression columns not populated.
	colsOpt := alter.Options["columns"]
	if colsOpt != "amount,tax" && colsOpt != "tax,amount" {
		t.Fatalf("RED: expected columns=amount,tax, got %q", colsOpt)
	}

	// RED: DDL.Constraints not projected.
	if len(statement.DDL.Constraints) != 1 {
		t.Fatalf("RED: expected one projected check constraint, got %d", len(statement.DDL.Constraints))
	}
	c := statement.DDL.Constraints[0]
	sort.Strings(c.Columns)
	if !reflect.DeepEqual(c.Columns, []string{"amount", "tax"}) {
		t.Fatalf("RED: expected projected columns [amount tax], got %v", c.Columns)
	}
}
