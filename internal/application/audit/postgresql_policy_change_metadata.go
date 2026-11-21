package audit

import (
	"context"
	"slices"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func enrichPGPolicyChanges(ctx context.Context, request *MetadataRequest, statements []spec.Statement) error {
	for _, s := range statements {
		lookup := spec.PGPolicyChangeLookup(s)
		if lookup == nil || s.Metadata == nil || !spec.PG17Observed(s.Metadata.Instance) {
			continue
		}
		snapshot, err := resolveObjectSnapshot(ctx, request, spec.DialectPostgreSQL, lookup)
		if err != nil {
			return err
		}
		if snapshot == nil || snapshot.PGPolicyChange == nil {
			continue
		}
		state := *snapshot.PGPolicyChange
		state.Columns = slices.Clone(state.Columns)
		state.NamedRoles = slices.Clone(state.NamedRoles)
		s.Metadata.PGPolicyChange = &state
	}
	return nil
}
