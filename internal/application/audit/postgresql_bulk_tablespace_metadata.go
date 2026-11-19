package audit

import (
	"context"
	"slices"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func enrichPGBulkTablespaces(ctx context.Context, request *MetadataRequest, statements []spec.Statement) error {
	for _, s := range statements {
		target := spec.PGBulkTablespaceSelection(s)
		if target == nil || s.Metadata == nil || !spec.PG17Observed(s.Metadata.Instance) {
			continue
		}
		selection := *target
		selection.Roles = slices.Clone(target.Roles)
		lookup := spec.ObjectLookupRequest{Type: "bulk_tablespace", PGBulkTablespace: &selection}
		object, err := resolveObjectSnapshot(ctx, request, spec.DialectPostgreSQL, &lookup)
		if err != nil {
			return err
		}
		if object == nil || object.PGBulkTablespace == nil {
			continue
		}
		facts := *object.PGBulkTablespace
		if !target.Matches(facts.Target) || len(facts.RoleExists) != len(target.Roles) {
			continue
		}
		facts.Target.Roles = slices.Clone(facts.Target.Roles)
		facts.RoleExists = slices.Clone(facts.RoleExists)
		s.Metadata.PGBulkTablespace = &facts
	}
	return nil
}
