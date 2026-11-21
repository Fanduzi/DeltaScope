package audit

import (
	"context"
	"slices"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func enrichPGTextSearchChanges(ctx context.Context, request *MetadataRequest, statements []spec.Statement) error {
	for _, s := range statements {
		lookup := spec.PGTextSearchChangeLookup(s)
		if lookup == nil || s.Metadata == nil || !spec.PG17Observed(s.Metadata.Instance) {
			continue
		}
		object, err := resolveObjectSnapshot(ctx, request, spec.DialectPostgreSQL, lookup)
		if err != nil {
			return err
		}
		if object != nil && object.PGTextSearchChange != nil {
			state := *object.PGTextSearchChange
			state.Mappings = slices.Clone(state.Mappings)
			state.Dictionaries = slices.Clone(state.Dictionaries)
			state.DictionaryExists = slices.Clone(state.DictionaryExists)
			s.Metadata.PGTextSearchChange = &state
		}
	}
	return nil
}
