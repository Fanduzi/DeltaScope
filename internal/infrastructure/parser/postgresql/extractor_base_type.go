//go:build postgresql

package postgresql

import (
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

func extractBaseTypeDeclaration(s spec.Statement, names, options []*pg_query.Node, action string) spec.Statement {
	decl := &spec.PGBaseTypeDeclaration{Action: action}
	if len(names) == 2 {
		decl.Target.Table = spec.Table{Schema: stringNodeValue(names[0]), Name: stringNodeValue(names[1])}
	}
	for _, node := range options {
		option := node.GetDefElem()
		fact := spec.PGBaseTypeOption{Name: option.GetDefname()}
		if fact.Name == "storage" {
			arg := option.GetArg()
			if v := arg.GetString_(); v != nil {
				fact.Storage, fact.ValueKnown = v.GetSval(), true
			} else if names := arg.GetTypeName().GetNames(); len(names) == 1 && names[0].GetString_() != nil {
				fact.Storage, fact.ValueKnown = stringNodeValue(names[0]), true
			}
		}
		decl.Options = append(decl.Options, fact)
	}
	operation := spec.DDLOperationCreateType
	if action == "set_properties" {
		operation = spec.DDLOperationAlterType
	}
	s.DDL = &spec.DDL{Operation: operation, ObjectType: "type", ObjectName: lastStringFromNodes(names), PGBaseType: decl}
	return s
}
