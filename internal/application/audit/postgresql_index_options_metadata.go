package audit

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func enrichPGIndexOptions(ctx context.Context, request *MetadataRequest, statements []spec.Statement) error {
	for _, s := range statements {
		lookup := spec.PGIndexOptionsLookup(s)
		if lookup == nil || s.Metadata == nil || !spec.PG17Observed(s.Metadata.Instance) {
			continue
		}
		object, err := resolveObjectSnapshot(ctx, request, spec.DialectPostgreSQL, lookup)
		if err != nil {
			return err
		}
		if object != nil && object.PGIndexOptions != nil {
			state := *object.PGIndexOptions
			if state.Target.Table.Schema == lookup.Schema && state.Target.Table.Name == lookup.Name {
				s.Metadata.PGIndexOptions = &state
			}
		}
	}
	return nil
}
