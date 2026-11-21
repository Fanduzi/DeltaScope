package audit

import (
	"context"
	"slices"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func enrichPGCreateTyped(ctx context.Context, request *MetadataRequest, statements []spec.Statement) error {
	for _, s := range statements {
		lookup := spec.PGCreateTypedLookup(s)
		if lookup == nil || s.Metadata == nil || !spec.PG17Observed(s.Metadata.Instance) {
			continue
		}
		object, err := resolveObjectSnapshot(ctx, request, spec.DialectPostgreSQL, lookup)
		if err != nil {
			return err
		}
		if object == nil || object.PGCreateTyped == nil {
			continue
		}
		state := *object.PGCreateTyped
		typed := s.DDL.PGCreateTyped
		if state.Target == typed.Target && state.Type == typed.Type {
			state.Columns = slices.Clone(state.Columns)
			s.Metadata.PGCreateTyped = &state
		}
	}
	return nil
}
