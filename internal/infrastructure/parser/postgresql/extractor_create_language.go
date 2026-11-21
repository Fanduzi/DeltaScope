//go:build postgresql

package postgresql

import (
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

func extractCreateLanguageStmt(statement spec.Statement, stmt *pg_query.CreatePLangStmt) spec.Statement {
	if stmt == nil || len(stmt.GetPlhandler()) == 0 {
		return unsupportedStatement(statement, "create_language", "PostgreSQL language handler declaration is missing")
	}
	declaration := &spec.PGLanguageDeclaration{Name: stmt.GetPlname(), Replace: stmt.GetReplace(), Trusted: stmt.GetPltrusted()}
	for _, entry := range []struct {
		kind string
		path []*pg_query.Node
	}{{"handler", stmt.GetPlhandler()}, {"inline", stmt.GetPlinline()}, {"validator", stmt.GetPlvalidator()}} {
		if len(entry.path) == 0 {
			continue
		}
		routine := spec.PGLanguageRoutine{Kind: entry.kind}
		switch len(entry.path) {
		case 3:
			routine.Catalog = stringNodeValue(entry.path[0])
			entry.path = entry.path[1:]
			fallthrough
		case 2:
			routine.Schema = stringNodeValue(entry.path[0])
			entry.path = entry.path[1:]
			fallthrough
		case 1:
			routine.Name = stringNodeValue(entry.path[0])
		}
		declaration.Routines = append(declaration.Routines, routine)
	}
	statement.DDL = &spec.DDL{Operation: spec.DDLOperationCreateLanguage, ObjectName: declaration.Name, ObjectType: "language", PGLanguage: declaration}
	return statement
}
