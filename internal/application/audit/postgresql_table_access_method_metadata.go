package audit

import (
	"context"
	"strconv"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func enrichPGTableAccessMethods(ctx context.Context, request *MetadataRequest, statements []spec.Statement) error {
	for _, s := range statements {
		name, useDefault := spec.PGTableAccessMethodTarget(s)
		if name == "" || !spec.PGOrdinaryTableBound(s) && (useDefault || !spec.PGPartitionedTableBound(s)) || !spec.PG17Observed(s.Metadata.Instance) {
			continue
		}
		table := s.DDL.Table
		lookup := spec.ObjectLookupRequest{
			Schema: table.Schema, Name: name, Type: "table_access_method",
			Qualifiers: map[string]string{"table": table.Name, "is_default": strconv.FormatBool(useDefault), "relation_kind": s.Metadata.TargetTable.PGRelation.Kind},
		}
		object, err := resolveObjectSnapshot(ctx, request, spec.DialectPostgreSQL, &lookup)
		if err != nil {
			return err
		}
		if object == nil || object.PGTableAccessMethod == nil {
			continue
		}
		facts := *object.PGTableAccessMethod
		if facts.Schema == table.Schema && facts.Table == table.Name && facts.Name == name && facts.Default == useDefault {
			s.Metadata.PGTableAccessMethod = &facts
		}
	}
	return nil
}
