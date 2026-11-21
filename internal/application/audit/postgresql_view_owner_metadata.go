package audit

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func enrichPGViewOwners(ctx context.Context, request *MetadataRequest, statements []spec.Statement) error {
	for _, s := range statements {
		lookup := spec.PGViewOwnerLookup(s)
		if lookup == nil || s.Metadata == nil || !spec.PG17Observed(s.Metadata.Instance) {
			continue
		}
		object, err := resolveObjectSnapshot(ctx, request, spec.DialectPostgreSQL, lookup)
		if err != nil {
			return err
		}
		if object == nil || object.PGTableOwner == nil {
			continue
		}
		state := *object.PGTableOwner
		if state.Schema == lookup.Schema && state.Table == lookup.Qualifiers["table"] && state.Target == s.DDL.Alter[0].PGTableOwner {
			s.Metadata.PGTableOwner = &state
		}
	}
	return nil
}
