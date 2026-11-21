//go:build postgresql

package postgresql

import (
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

func pgSequenceChange(relation *pg_query.RangeVar, options []*pg_query.Node, ifExists, ifNotExists bool) *spec.PGSequenceChange {
	decl := &spec.PGSequenceChange{
		Target:      spec.PGDestructiveTableTarget{Table: spec.Table{Schema: relation.GetSchemaname(), Name: relation.GetRelname()}, CatalogQualified: relation.GetCatalogname() != ""},
		Persistence: relation.GetRelpersistence(), IfExists: ifExists, IfNotExists: ifNotExists,
	}
	for _, item := range options {
		d := item.GetDefElem()
		o := spec.PGSequenceChangeOption{Name: d.GetDefname()}
		switch o.Name {
		case "as":
			o.Width = pgIdentityTypeWidth(d.GetArg().GetTypeName())
			o.Known = o.Width != 0
		case "owned_by":
			o.Argument = stringValuesFromNodes(d.GetArg().GetList().GetItems())
			o.Known = len(o.Argument) > 0
		default:
			option, known := pgIdentityOption(d)
			o.Value, o.Known = option.Value, known
		}
		decl.Options = append(decl.Options, o)
	}
	return decl
}
