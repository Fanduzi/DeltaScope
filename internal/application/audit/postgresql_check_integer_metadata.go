package audit

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func enrichPGCheckIntegers(ctx context.Context, request *MetadataRequest, statements []spec.Statement) error {
	for _, s := range statements {
		lookup := spec.PGCheckIntegerLookup(s)
		if lookup == nil || !spec.PG17Observed(s.Metadata.Instance) {
			continue
		}
		object, err := resolveObjectSnapshot(ctx, request, spec.DialectPostgreSQL, lookup)
		if err != nil {
			return err
		}
		if object != nil && object.PGCheckIntegerCoercion != nil {
			state := *object.PGCheckIntegerCoercion
			s.Metadata.PGCheckIntegerCoercion = &state
		}
	}
	return nil
}
