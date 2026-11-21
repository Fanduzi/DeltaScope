package audit

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func enrichPGDropViews(ctx context.Context, request *MetadataRequest, statements []spec.Statement) error {
	for _, s := range statements {
		lookup := spec.PGDropViewLookup(s)
		if lookup == nil {
			lookup = spec.PGAlterViewOptionsLookup(s)
		}
		if lookup == nil {
			lookup = spec.PGMaterializedViewOptionsLookup(s)
		}
		if lookup == nil || s.Metadata == nil || !spec.PG17Observed(s.Metadata.Instance) {
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
