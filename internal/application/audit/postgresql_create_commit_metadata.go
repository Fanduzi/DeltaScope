package audit

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func enrichPGCreateDestinations(ctx context.Context, request *MetadataRequest, statements []spec.Statement) error {
	for _, s := range statements {
		lookup := spec.PGCreateDestinationLookup(s)
		if lookup == nil || s.Metadata == nil || !spec.PG17Observed(s.Metadata.Instance) {
			continue
		}
		object, err := resolveObjectSnapshot(ctx, request, spec.DialectPostgreSQL, lookup)
		if err != nil {
			return err
		}
		if object != nil && object.PGCreateDestination != nil && object.PGCreateDestination.Target.Table == (spec.Table{Schema: lookup.Schema, Name: lookup.Name}) {
			state := *object.PGCreateDestination
			s.Metadata.PGCreateDestination = &state
		}
	}
	return nil
}
