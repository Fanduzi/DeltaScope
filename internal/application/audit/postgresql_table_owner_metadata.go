package audit

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func enrichPGTableOwners(ctx context.Context, request *MetadataRequest, statements []spec.Statement) error {
	for _, s := range statements {
		target := spec.PGTableOwnerSelection(s)
		bound := spec.PGOrdinaryTableBound(s) || spec.PGPartitionedTableBound(s) && (target.Kind == "named" || target.Kind == "public")
		if target.Kind == "" || !bound || !spec.PG17Observed(s.Metadata.Instance) {
			continue
		}
		table := s.DDL.Table
		lookup := spec.ObjectLookupRequest{
			Schema: table.Schema, Name: target.Name, Type: "table_owner",
			Qualifiers: map[string]string{"table": table.Name, "kind": target.Kind, "relation_kind": s.Metadata.TargetTable.PGRelation.Kind},
		}
		object, err := resolveObjectSnapshot(ctx, request, spec.DialectPostgreSQL, &lookup)
		if err != nil {
			return err
		}
		if object == nil || object.PGTableOwner == nil {
			continue
		}
		facts := *object.PGTableOwner
		if facts.Schema == table.Schema && facts.Table == table.Name && facts.Target == target {
			s.Metadata.PGTableOwner = &facts
		}
	}
	return nil
}
