package audit

import (
	"context"
	"slices"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func enrichPGPublicationChanges(ctx context.Context, request *MetadataRequest, statements []spec.Statement) error {
	for _, s := range statements {
		lookup := spec.PGPublicationChangeLookup(s)
		if lookup == nil || s.Metadata == nil || !spec.PG17Observed(s.Metadata.Instance) {
			continue
		}
		object, err := resolveObjectSnapshot(ctx, request, spec.DialectPostgreSQL, lookup)
		if err != nil {
			return err
		}
		if object != nil && object.PGPublicationChange != nil {
			state := *object.PGPublicationChange
			state.Members = slices.Clone(state.Members)
			s.Metadata.PGPublicationChange = &state
		}
	}
	return nil
}
