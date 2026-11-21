package audit

import (
	"context"
	"slices"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func enrichPGCompositeTypes(ctx context.Context, request *MetadataRequest, statements []spec.Statement) error {
	for _, s := range statements {
		lookup := spec.PGCompositeLookup(s)
		if lookup == nil || s.Metadata == nil || !spec.PG17Observed(s.Metadata.Instance) {
			continue
		}
		object, err := resolveObjectSnapshot(ctx, request, spec.DialectPostgreSQL, lookup)
		if err != nil {
			return err
		}
		if object != nil && object.PGComposite != nil && object.PGComposite.Target.Table == (spec.Table{Schema: lookup.Schema, Name: lookup.Name}) {
			state := *object.PGComposite
			state.Attributes = slices.Clone(state.Attributes)
			s.Metadata.PGComposite = &state
		}
	}
	return nil
}
