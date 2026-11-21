package audit

import (
	"context"
	"slices"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func enrichPGPartitionConstraints(ctx context.Context, request *MetadataRequest, statements []spec.Statement) error {
	for _, s := range statements {
		lookup := spec.PGPartitionConstraintLookup(s)
		if lookup == nil || !spec.PG17Observed(s.Metadata.Instance) {
			continue
		}
		object, err := resolveObjectSnapshot(ctx, request, spec.DialectPostgreSQL, lookup)
		if err != nil {
			return err
		}
		if object != nil && object.PGPartitionConstraint != nil && object.PGPartitionConstraint.Target == *s.DDL.Table {
			state := *object.PGPartitionConstraint
			state.Columns = slices.Clone(state.Columns)
			state.SystemColumns = slices.Clone(state.SystemColumns)
			state.PartitionKeys = slices.Clone(state.PartitionKeys)
			s.Metadata.PGPartitionConstraint = &state
		}
	}
	return nil
}
