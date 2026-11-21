package audit

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func enrichPGSequenceChanges(ctx context.Context, request *MetadataRequest, statements []spec.Statement) error {
	for _, s := range statements {
		lookup := spec.PGSequenceChangeLookup(s)
		if lookup == nil || s.Metadata == nil || !spec.PG17Observed(s.Metadata.Instance) {
			continue
		}
		object, err := resolveObjectSnapshot(ctx, request, spec.DialectPostgreSQL, lookup)
		if err != nil {
			return err
		}
		if object != nil && object.PGSequenceChange != nil && object.PGSequenceChange.Target == s.DDL.PGSequenceChange.Target {
			state := *object.PGSequenceChange
			state.Definition = state.Definition.Clone()
			if state.Ownership != nil {
				owned := *state.Ownership
				state.Ownership = &owned
			}
			s.Metadata.PGSequenceChange = &state
		}
	}
	return nil
}
