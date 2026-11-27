package audit

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func enrichPGTableTablespaces(ctx context.Context, request *MetadataRequest, statements []spec.Statement) error {
	for _, s := range statements {
		name := spec.PGTableTablespaceTarget(s)
		if name == "" || !spec.PGOrdinaryTableBound(s) && !spec.PGPartitionedTableBound(s) || !spec.PG17Observed(s.Metadata.Instance) {
			continue
		}
		table := s.DDL.Table
		lookup := spec.ObjectLookupRequest{
			Schema: table.Schema, Name: name, Type: "table_tablespace",
			Qualifiers: map[string]string{"table": table.Name, "relation_kind": s.Metadata.TargetTable.PGRelation.Kind},
		}
		object, err := resolveObjectSnapshot(ctx, request, spec.DialectPostgreSQL, &lookup)
		if err != nil {
			return err
		}
		if object == nil || object.PGTableTablespace == nil {
			continue
		}
		facts := *object.PGTableTablespace
		if facts.Schema == table.Schema && facts.Table == table.Name && facts.Name == name {
			s.Metadata.PGTableTablespace = &facts
		}
	}
	return nil
}
