package audit

import (
	"context"
	"slices"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func enrichPGWrapperChanges(ctx context.Context, request *MetadataRequest, statements []spec.Statement) error {
	for _, s := range statements {
		lookup := spec.PGWrapperChangeLookup(s)
		if lookup == nil || s.Metadata == nil || !spec.PG17Observed(s.Metadata.Instance) {
			continue
		}
		object, err := resolveObjectSnapshot(ctx, request, spec.DialectPostgreSQL, lookup)
		if err != nil {
			return err
		}
		if object != nil && object.PGWrapperChange != nil {
			state := *object.PGWrapperChange
			state.Options = slices.Clone(state.Options)
			state.Functions = slices.Clone(state.Functions)
			s.Metadata.PGWrapperChange = &state
		}
	}
	return nil
}
