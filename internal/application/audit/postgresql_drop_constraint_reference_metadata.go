package audit

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func enrichPGDropConstraintReferences(ctx context.Context, request *MetadataRequest, statements []spec.Statement) error {
	for _, s := range statements {
		member := spec.PGDropConstraintReferenceTarget(s)
		if member == nil {
			continue
		}
		table := s.DDL.Table
		lookup := spec.ObjectLookupRequest{Type: "drop_constraint_reference", Schema: table.Schema, Name: member.Name,
			Qualifiers: map[string]string{"table": table.Name, "kind": member.Type}}
		object, err := resolveObjectSnapshot(ctx, request, spec.DialectPostgreSQL, &lookup)
		if err != nil {
			return err
		}
		if object == nil || object.PGDropConstraintReference == nil {
			continue
		}
		fact := *object.PGDropConstraintReference
		if fact.Schema == table.Schema && fact.Table == table.Name && fact.Name == member.Name && fact.Kind == member.Type {
			s.Metadata.PGDropConstraintReference = &fact
		}
	}
	return nil
}
