package audit

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func enrichPGIndexDestinations(ctx context.Context, request *MetadataRequest, statements []spec.Statement) error {
	for _, s := range statements {
		lookup := spec.PGIndexDestinationLookup(s)
		if lookup == nil || s.Metadata == nil || !spec.PG17Observed(s.Metadata.Instance) {
			continue
		}
		object, err := resolveObjectSnapshot(ctx, request, spec.DialectPostgreSQL, lookup)
		if err != nil {
			return err
		}
		if object != nil && object.PGIndexDestination != nil {
			state := *object.PGIndexDestination
			target := s.DDL.PGIndexDestination
			if state.Source == target.Source && state.Action == target.Action && state.Destination == target.Destination {
				s.Metadata.PGIndexDestination = &state
			}
		}
	}
	return nil
}
