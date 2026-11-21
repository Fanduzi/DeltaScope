package audit

import (
	"context"
	"slices"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func enrichPGGrantDeclarations(ctx context.Context, request *MetadataRequest, statements []spec.Statement) error {
	for _, s := range statements {
		lookup := spec.PGGrantLookup(s)
		if lookup == nil || s.Metadata == nil || !spec.PG17Observed(s.Metadata.Instance) {
			continue
		}
		object, err := resolveObjectSnapshot(ctx, request, spec.DialectPostgreSQL, lookup)
		if err != nil {
			return err
		}
		if object != nil && object.PGGrant != nil {
			state := *object.PGGrant
			state.Targets = slices.Clone(state.Targets)
			for i := range state.Targets {
				state.Targets[i].Columns = slices.Clone(state.Targets[i].Columns)
			}
			state.Grantees = slices.Clone(state.Grantees)
			state.GranteeExists = slices.Clone(state.GranteeExists)
			s.Metadata.PGGrant = &state
		}
	}
	return nil
}
