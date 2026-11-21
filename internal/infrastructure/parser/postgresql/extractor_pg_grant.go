//go:build postgresql

package postgresql

import (
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

func pgGrantDeclaration(stmt *pg_query.GrantStmt, kind string) *spec.PGGrantDeclaration {
	d := &spec.PGGrantDeclaration{ObjectKind: kind, Grant: stmt.GetIsGrant(), GrantOption: stmt.GetGrantOption(),
		Cascade:     stmt.GetBehavior() == pg_query.DropBehavior_DROP_CASCADE,
		AllInSchema: stmt.GetTargtype() == pg_query.GrantTargetType_ACL_TARGET_ALL_IN_SCHEMA}
	if stmt.GetGrantor() != nil {
		d.Grantor = pgRoleSelection(stmt.GetGrantor())
	}
	for _, item := range stmt.GetGrantees() {
		d.Grantees = append(d.Grantees, pgRoleSelection(item.GetRoleSpec()))
	}
	for _, item := range stmt.GetPrivileges() {
		ap := item.GetAccessPriv()
		priv := spec.PGGrantPrivilege{Name: ap.GetPrivName()}
		for _, col := range ap.GetCols() {
			priv.Columns = append(priv.Columns, stringNodeValue(col))
		}
		d.Privileges = append(d.Privileges, priv)
	}
	for _, item := range stmt.GetObjects() {
		var target spec.PGDestructiveTableTarget
		if d.AllInSchema || kind == "schema" || kind == "database" || kind == "tablespace" {
			target.Table.Name = stringNodeValue(item)
		} else {
			r := item.GetRangeVar()
			target.Table = spec.Table{Schema: r.GetSchemaname(), Name: r.GetRelname()}
			target.CatalogQualified = r.GetCatalogname() != ""
		}
		d.Targets = append(d.Targets, target)
	}
	return d
}

func extractPGGrantObject(statement spec.Statement, stmt *pg_query.GrantStmt, kind string) spec.Statement {
	d := pgGrantDeclaration(stmt, kind)
	if len(d.Targets) == 0 || d.Targets[0].Table.Name == "" {
		return unsupportedStatement(statement, "grant_object", "PostgreSQL grant target identity is missing")
	}
	op := spec.DDLOperationGrantObject
	if !d.Grant {
		op = spec.DDLOperationRevokeObject
	}
	statement.DDL = &spec.DDL{Operation: op, ObjectName: d.Targets[0].Table.Name, ObjectType: kind, PGGrant: d}
	return statement
}
