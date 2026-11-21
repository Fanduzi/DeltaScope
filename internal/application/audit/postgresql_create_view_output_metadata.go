package audit

import (
	"context"
	"slices"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func enrichPGCreateViewTargets(ctx context.Context, request *MetadataRequest, statements []spec.Statement) error {
	for _, s := range statements {
		lookup := spec.PGCreateViewTargetLookup(s)
		if lookup == nil || s.Metadata == nil || !spec.PG17Observed(s.Metadata.Instance) {
			continue
		}
		object, err := resolveObjectSnapshot(ctx, request, spec.DialectPostgreSQL, lookup)
		if err != nil {
			return err
		}
		if object == nil || object.PGCreateViewTarget == nil {
			continue
		}
		state := *object.PGCreateViewTarget
		if state.Table.Schema == lookup.Schema && state.Table.Name == lookup.Name {
			state.Columns = slices.Clone(state.Columns)
			s.Metadata.PGCreateViewTarget = &state
		}
	}
	return nil
}
