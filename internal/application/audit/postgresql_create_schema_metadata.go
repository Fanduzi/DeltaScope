package audit

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func enrichPGCreateSchemas(ctx context.Context, request *MetadataRequest, statements []spec.Statement) error {
	for _, s := range statements {
		lookup := spec.PGCreateSchemaLookup(s)
		if lookup == nil || s.Metadata == nil || !spec.PG17Observed(s.Metadata.Instance) {
			continue
		}
		object, err := resolveObjectSnapshot(ctx, request, spec.DialectPostgreSQL, lookup)
		if err != nil {
			return err
		}
		if object != nil && object.PGCreateSchema != nil && object.PGCreateSchema.Target == *s.DDL.PGCreateSchema {
			state := *object.PGCreateSchema
			s.Metadata.PGCreateSchema = &state
		}
	}
	return nil
}
