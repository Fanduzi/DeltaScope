package audit

import (
	"context"
	"slices"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func enrichPGIndexIncludes(ctx context.Context, request *MetadataRequest, statements []spec.Statement) error {
	for _, s := range statements {
		lookup := spec.PGIndexIncludeLookup(s)
		if lookup == nil || s.Metadata == nil || !spec.PG17Observed(s.Metadata.Instance) {
			continue
		}
		object, err := resolveObjectSnapshot(ctx, request, spec.DialectPostgreSQL, lookup)
		if err != nil {
			return err
		}
		if object != nil && object.PGIndexInclude != nil {
			state := *object.PGIndexInclude
			if state.Target.Table.Schema == lookup.Schema && state.Target.Table.Name == lookup.Name {
				state.Columns, state.System = slices.Clone(state.Columns), slices.Clone(state.System)
				s.Metadata.PGIndexInclude = &state
			}
		}
	}
	return nil
}
