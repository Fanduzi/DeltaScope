//go:build postgresql

package postgresql

import (
	"encoding/json"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

func pgAlterEnumDeclaration(stmt *pg_query.AlterEnumStmt, sql string) *spec.PGAlterEnum {
	decl := &spec.PGAlterEnum{Value: stmt.GetNewVal(), IfNotExists: stmt.GetSkipIfNewValExists()}
	names := stmt.GetTypeName()
	if len(names) == 2 {
		decl.Target.Table = spec.Table{Schema: stringNodeValue(names[0]), Name: stringNodeValue(names[1])}
	}
	if old := stmt.GetOldVal(); old != "" {
		decl.Old, decl.Known = &old, true
		return decl
	}
	if neighbor := stmt.GetNewValNeighbor(); neighbor != "" {
		decl.Neighbor, decl.Known = &neighbor, true
		return decl
	}
	// Protobuf uses the same string value for a missing field and an empty label.
	// PostgreSQL's JSON AST preserves that distinction.
	parsed, err := pg_query.ParseToJSON(sql)
	if err != nil {
		return decl
	}
	var tree struct {
		Statements []struct {
			Statement struct {
				Enum *struct {
					Old      *string `json:"oldVal"`
					Value    *string `json:"newVal"`
					Neighbor *string `json:"newValNeighbor"`
				} `json:"AlterEnumStmt"`
			} `json:"stmt"`
		} `json:"stmts"`
	}
	if json.Unmarshal([]byte(parsed), &tree) != nil || len(tree.Statements) != 1 {
		return decl
	}
	enum := tree.Statements[0].Statement.Enum
	if enum != nil && enum.Value != nil && *enum.Value == decl.Value {
		decl.Old, decl.Neighbor, decl.Known = enum.Old, enum.Neighbor, true
	}
	return decl
}
