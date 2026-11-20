package audit

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func enrichPGConstraintUsingIndexes(ctx context.Context, request *MetadataRequest, statements []spec.Statement) error {
	for _, s := range statements {
		name := spec.PGConstraintUsingIndexTarget(s)
		if name == "" || !spec.PGOrdinaryTableBound(s) || !spec.PG17Observed(s.Metadata.Instance) {
			continue
		}
		table := s.DDL.Table
		lookup := spec.ObjectLookupRequest{Schema: table.Schema, Name: name, Type: "constraint_using_index",
			Qualifiers: map[string]string{"table": table.Name}}
		object, err := resolveObjectSnapshot(ctx, request, spec.DialectPostgreSQL, &lookup)
		if err != nil {
			return err
		}
		if object == nil || object.PGConstraintUsingIndex == nil {
			continue
		}
		facts := *object.PGConstraintUsingIndex
		if facts.Schema == table.Schema && facts.Table == table.Name && facts.Name == name {
			s.Metadata.PGConstraintUsingIndex = &facts
		}
	}
	return nil
}
