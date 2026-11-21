//go:build postgresql

package postgresql

import (
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

func extractCreatePolicyStmt(statement spec.Statement, stmt *pg_query.CreatePolicyStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "create_policy", "postgresql create policy statement payload is missing")
	}
	options := map[string]string{}
	if !stmt.GetPermissive() {
		options["permissive"] = "false"
	}
	if stmt.GetCmdName() != "" {
		options["cmd"] = stmt.GetCmdName()
	}
	statement.DDL = &spec.DDL{
		Operation:  spec.DDLOperationCreatePolicy,
		ObjectName: stmt.GetPolicyName(),
		ObjectType: "policy",
		Table:      tableFromRangeVar(stmt.GetTable()),
		Options:    options,
	}
	return statement
}

func extractAlterPolicyStmt(statement spec.Statement, stmt *pg_query.AlterPolicyStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "alter_policy", "postgresql alter policy statement payload is missing")
	}
	decl := pgPolicyChangeDeclaration(stmt.GetTable(), stmt.GetPolicyName(), "")
	for _, node := range stmt.GetRoles() {
		decl.Roles = append(decl.Roles, pgRoleSelection(node.GetRoleSpec()))
	}
	decl.Using = pgPolicyPredicate(stmt.GetQual())
	decl.WithCheck = pgPolicyPredicate(stmt.GetWithCheck())
	statement.DDL = &spec.DDL{
		PGPolicyChange: decl,
		Operation:      spec.DDLOperationAlterPolicy,
		ObjectName:     stmt.GetPolicyName(),
		ObjectType:     "policy",
		Table:          tableFromRangeVar(stmt.GetTable()),
	}
	return statement
}

func pgPolicyChangeDeclaration(relation *pg_query.RangeVar, name, renameTo string) *spec.PGPolicyChange {
	return &spec.PGPolicyChange{
		Table: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: relation.GetSchemaname(), Name: relation.GetRelname()}, CatalogQualified: relation.GetCatalogname() != ""},
		Name:  name, RenameTo: renameTo,
	}
}

func pgPolicyPredicate(node *pg_query.Node) spec.PGPolicyPredicate {
	if node == nil {
		return spec.PGPolicyPredicate{}
	}
	fact := spec.PGPolicyPredicate{Present: true}
	if value := node.GetAConst(); value != nil && (value.GetBoolval() != nil || value.GetIsnull()) {
		fact.BooleanResult = true
		return fact
	}
	ref := node.GetColumnRef()
	if test := node.GetNullTest(); test != nil {
		ref = test.GetArg().GetColumnRef()
		fact.BooleanResult = ref != nil
	}
	if fields := ref.GetFields(); len(fields) == 1 {
		fact.Column = stringNodeValue(fields[0])
	} else {
		fact.BooleanResult = false
	}
	return fact
}

func dropPolicyName(stmt *pg_query.DropStmt) string {
	return objectNameFromObjectName(stmt.GetObjects())
}
