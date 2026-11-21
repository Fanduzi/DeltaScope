package audit

import (
	"context"
	"slices"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func enrichPGLanguageDeclarations(ctx context.Context, request *MetadataRequest, statements []spec.Statement) error {
	for _, statement := range statements {
		lookup := spec.PGLanguageLookup(statement)
		if lookup == nil || statement.Metadata == nil || !spec.PG17Observed(statement.Metadata.Instance) {
			continue
		}
		object, err := resolveObjectSnapshot(ctx, request, spec.DialectPostgreSQL, lookup)
		if err != nil {
			return err
		}
		if object != nil && object.PGLanguage != nil {
			state := *object.PGLanguage
			state.Routines = slices.Clone(state.Routines)
			statement.Metadata.PGLanguage = &state
		}
	}
	return nil
}
