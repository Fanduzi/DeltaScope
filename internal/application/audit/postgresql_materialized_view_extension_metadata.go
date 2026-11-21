package audit

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func enrichPGMaterializedViewExtensions(ctx context.Context, request *MetadataRequest, statements []spec.Statement) error {
	for _, s := range statements {
		lookup := spec.PGMaterializedViewExtensionLookup(s)
		if lookup == nil || s.Metadata == nil || !spec.PG17Observed(s.Metadata.Instance) {
			continue
		}
		object, err := resolveObjectSnapshot(ctx, request, spec.DialectPostgreSQL, lookup)
		if err != nil {
			return err
		}
		if object == nil || object.PGMaterializedViewExtension == nil {
			continue
		}
		state := *object.PGMaterializedViewExtension
		if state.Schema == lookup.Schema && state.View == lookup.Name && state.Extension == lookup.Qualifiers["extension"] {
			s.Metadata.PGMaterializedViewExtension = &state
		}
	}
	return nil
}
