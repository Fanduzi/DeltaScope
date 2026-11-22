package audit

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func enrichPGCheckQualifiedReferences(ctx context.Context, request *MetadataRequest, statements []spec.Statement) error {
	for _, statement := range statements {
		lookup := spec.PGCheckQualifiedReferenceLookup(statement)
		if lookup == nil || !spec.PG17Observed(statement.Metadata.Instance) {
			continue
		}
		object, err := resolveObjectSnapshot(ctx, request, spec.DialectPostgreSQL, lookup)
		if err != nil {
			return err
		}
		if object != nil && object.PGCheckQualifiedReference != nil {
			state := *object.PGCheckQualifiedReference
			statement.Metadata.PGCheckQualifiedReference = &state
		}
	}
	return nil
}
