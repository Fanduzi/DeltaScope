//go:build postgresql

package postgresql

import (
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

func pgCreateColumns(stmt *pg_query.CreateStmt, ddl *spec.DDL, target spec.PGDestructiveTableTarget) *spec.PGCreateColumns {
	if stmt.GetRelation().GetRelpersistence() == "t" || len(stmt.GetInhRelations()) != 0 || ddl.PGCreateTyped != nil || len(ddl.PGCreateLike) != 0 {
		return nil
	}
	facts := &spec.PGCreateColumns{Target: target, IfNotExists: stmt.GetIfNotExists()}
	seen := make(map[string]bool, len(ddl.Columns))
	for _, column := range ddl.Columns {
		if seen[column.Name] {
			facts.Refusal = "create_column_duplicate"
			return facts
		}
		seen[column.Name] = true
		if spec.PGSystemColumnName(column.Name) {
			facts.Refusal = "create_column_system_name"
		}
	}
	for _, item := range stmt.GetTableElts() {
		typeName := item.GetColumnDef().GetTypeName()
		names := typeName.GetNames()
		if len(names) != 2 || typeName.GetPctType() {
			continue
		}
		target := spec.PGColumnTypeTarget{Schema: stringNodeValue(names[0]), Name: stringNodeValue(names[1]), Array: len(typeName.GetArrayBounds()) != 0}
		qualified := spec.PGDestructiveTableTarget{Table: spec.Table{Schema: target.Schema, Name: target.Name}}
		if qualified.Qualified() && target.Schema != "pg_toast_temp" && !pgBuiltinColumnType(target) {
			facts.Types = append(facts.Types, target)
		}
	}
	if facts.Refusal == "" && len(facts.Types) == 0 {
		return nil
	}
	return facts
}

func pgBuiltinColumnType(target spec.PGColumnTypeTarget) bool {
	if target.Schema != "pg_catalog" {
		return false
	}
	// These PostgreSQL grammar types and their arrays already have fixed identities.
	switch target.Name {
	case "bit", "bool", "bpchar", "float4", "float8", "int2", "int4", "int8", "interval", "json", "numeric", "text", "time", "timestamp", "timestamptz", "timetz", "varbit", "varchar":
		return true
	default:
		return false
	}
}
