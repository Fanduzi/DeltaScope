package audit

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func enrichPGColumnTraversal(ctx context.Context, request *MetadataRequest, statements []spec.Statement) error {
	for _, s := range statements {
		name := spec.PGColumnTraversalTarget(s)
		if name == "" {
			continue
		}
		table := s.DDL.Table
		lookup := spec.ObjectLookupRequest{Type: "column_traversal", Schema: table.Schema, Name: name, Qualifiers: map[string]string{"table": table.Name}}
		object, err := resolveObjectSnapshot(ctx, request, spec.DialectPostgreSQL, &lookup)
		if err != nil {
			return err
		}
		if object != nil && object.PGColumnTraversal != nil {
			fact := *object.PGColumnTraversal
			if fact.Schema == table.Schema && fact.Table == table.Name && fact.Column == name {
				s.Metadata.PGColumnTraversal = &fact
			}
		}
	}
	return nil
}
