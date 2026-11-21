package audit

import (
	"context"
	"slices"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func enrichPGCreateSources(ctx context.Context, request *MetadataRequest, statements []spec.Statement) error {
	for _, s := range statements {
		if s.Metadata == nil || !spec.PG17Observed(s.Metadata.Instance) {
			continue
		}
		like, inheritance := spec.PGCreateLikeLookup(s), spec.PGCreateInheritanceLookup(s)
		for _, lookup := range []*spec.ObjectLookupRequest{like, inheritance} {
			if lookup == nil {
				continue
			}
			object, err := resolveObjectSnapshot(ctx, request, spec.DialectPostgreSQL, lookup)
			if err != nil {
				return err
			}
			if object == nil {
				continue
			}
			state := object.PGCreateLike
			out := &s.Metadata.PGCreateLike
			if lookup == inheritance {
				state, out = object.PGCreateInheritance, &s.Metadata.PGCreateInheritance
			}
			if state == nil || state.Target.Table != *s.DDL.Table {
				continue
			}
			copy := *state
			copy.Sources = slices.Clone(state.Sources)
			for i := range copy.Sources {
				copy.Sources[i].Columns = slices.Clone(copy.Sources[i].Columns)
				if partition := copy.Sources[i].Partition; partition != nil {
					value := *partition
					copy.Sources[i].Partition = &value
				}
			}
			*out = &copy
		}
	}
	return nil
}
