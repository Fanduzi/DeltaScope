package audit

import (
	"context"
	"slices"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func enrichPGEnums(ctx context.Context, request *MetadataRequest, statements []spec.Statement) error {
	for _, s := range statements {
		lookup := spec.PGTypeDeclarationLookup(s)
		if lookup == nil || s.Metadata == nil || !spec.PG17Observed(s.Metadata.Instance) {
			continue
		}
		object, err := resolveObjectSnapshot(ctx, request, spec.DialectPostgreSQL, lookup)
		if err != nil {
			return err
		}
		if object != nil && object.PGEnum != nil && object.PGEnum.Target.Table == (spec.Table{Schema: lookup.Schema, Name: lookup.Name}) {
			state := *object.PGEnum
			state.Labels = slices.Clone(state.Labels)
			s.Metadata.PGEnum = &state
			if s.DDL.PGRange == nil || !state.SchemaExists || state.NameOccupied {
				continue
			}
			typeLookup := spec.PGRangeSubtypeLookup(s)
			if typeLookup == nil {
				continue
			}
			subtype, err := resolveObjectSnapshot(ctx, request, spec.DialectPostgreSQL, typeLookup)
			if err != nil {
				return err
			}
			if subtype != nil && subtype.PGColumnType != nil && subtype.PGColumnType.Target == s.DDL.PGRange.Subtype {
				bound := *subtype.PGColumnType
				s.Metadata.PGRangeSubtype = &bound
			}
		}
	}
	return nil
}
