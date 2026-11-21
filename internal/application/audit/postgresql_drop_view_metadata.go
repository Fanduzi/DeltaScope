package audit

import (
	"context"
	"slices"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func enrichPGDropViews(ctx context.Context, request *MetadataRequest, statements []spec.Statement) error {
	for _, s := range statements {
		if s.Metadata == nil || !spec.PG17Observed(s.Metadata.Instance) {
			continue
		}
		if lookup := spec.PGDropViewSetLookup(s); lookup != nil {
			object, err := resolveObjectSnapshot(ctx, request, spec.DialectPostgreSQL, lookup)
			if err != nil {
				return err
			}
			if object != nil && object.PGDropViewSet != nil && object.PGDropViewSet.Matches(lookup.PGDestructiveTargets, lookup.PGDropIfExists, lookup.Qualifiers["cascade"] == "true") {
				state := *object.PGDropViewSet
				state.Targets = slices.Clone(state.Targets)
				s.Metadata.PGDropViewSet = &state
			}
			continue
		}
		lookup := spec.PGDropViewLookup(s)
		if lookup == nil {
			lookup = spec.PGAlterViewOptionsLookup(s)
		}
		if lookup == nil {
			lookup = spec.PGMaterializedViewOptionsLookup(s)
		}
		if lookup == nil {
			continue
		}
		object, err := resolveObjectSnapshot(ctx, request, spec.DialectPostgreSQL, lookup)
		if err != nil {
			return err
		}
		target := spec.PGDestructiveTableTarget{Table: spec.Table{Schema: lookup.Schema, Name: lookup.Name}}
		if object != nil && object.PGDropViewState != nil && object.PGDropViewState.Target == target {
			state := *object.PGDropViewState
			s.Metadata.PGDropViewState = &state
		}
	}
	return nil
}
