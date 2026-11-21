package audit

import (
	"context"
	"slices"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func enrichPGSubscriptionChanges(ctx context.Context, request *MetadataRequest, statements []spec.Statement) error {
	for _, s := range statements {
		lookup := spec.PGSubscriptionChangeLookup(s)
		if lookup == nil || s.Metadata == nil || !spec.PG17Observed(s.Metadata.Instance) {
			continue
		}
		object, err := resolveObjectSnapshot(ctx, request, spec.DialectPostgreSQL, lookup)
		if err != nil {
			return err
		}
		if object != nil && object.PGSubscriptionChange != nil {
			state := *object.PGSubscriptionChange
			state.Publications = slices.Clone(state.Publications)
			s.Metadata.PGSubscriptionChange = &state
		}
	}
	return nil
}
