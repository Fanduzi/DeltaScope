//go:build postgresql

package postgresql

import (
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

func pgPublicationChange(stmt *pg_query.AlterPublicationStmt, action string) *spec.PGPublicationChange {
	change := &spec.PGPublicationChange{Name: stmt.GetPubname(), Action: action}
	if len(stmt.GetOptions()) > 0 {
		change.Action = "set_options"
		for _, node := range stmt.GetOptions() {
			def := node.GetDefElem()
			if def != nil {
				option := spec.PGReplicationOption{Name: def.GetDefname()}
				arg := def.GetArg()
				if option.Name == "publish" || option.Name == "publish_via_partition_root" {
					option.Text, option.Known = pgReplicationOptionText(arg)
					option.Bare, option.Integer = arg == nil, arg.GetInteger() != nil
				}
				change.Options = append(change.Options, option)
			}
		}
		return change
	}
	for _, node := range stmt.GetPubobjects() {
		object := node.GetPublicationObjSpec()
		member := spec.PGPublicationMember{}
		switch object.GetPubobjtype() {
		case pg_query.PublicationObjSpecType_PUBLICATIONOBJ_TABLE:
			t := object.GetPubtable()
			r := t.GetRelation()
			member = spec.PGPublicationMember{Kind: "table", Table: spec.PGDestructiveTableTarget{
				Table: spec.Table{Schema: r.GetSchemaname(), Name: r.GetRelname()}, CatalogQualified: r.GetCatalogname() != "", Only: !r.GetInh(),
			}, HasColumns: len(t.GetColumns()) != 0, HasRowFilter: t.GetWhereClause() != nil}
		case pg_query.PublicationObjSpecType_PUBLICATIONOBJ_TABLES_IN_SCHEMA:
			member.Kind, member.Schema = "schema", object.GetName()
		case pg_query.PublicationObjSpecType_PUBLICATIONOBJ_TABLES_IN_CUR_SCHEMA:
			member.Kind = "current_schema"
		}
		change.Members = append(change.Members, member)
	}
	return change
}
