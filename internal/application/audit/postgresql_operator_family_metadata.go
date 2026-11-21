package audit

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func enrichPGOperatorFamilies(ctx context.Context, request *MetadataRequest, statements []spec.Statement) error {
	for _, s := range statements {
		if s.Metadata == nil || !spec.PG17Observed(s.Metadata.Instance) {
			continue
		}
		lookup := spec.PGOperatorFamilyLookup(s)
		if lookup == nil {
			lookup = spec.PGOperatorFamilyMemberLookup(s)
		}
		if lookup == nil {
			lookup = spec.PGOperatorFamilyChangeLookup(s)
		}
		if lookup == nil {
			continue
		}
		object, err := resolveObjectSnapshot(ctx, request, spec.DialectPostgreSQL, lookup)
		if err != nil {
			return err
		}
		if object != nil && object.PGOperatorFamily != nil && s.DDL.PGOperatorFamily != nil && object.PGOperatorFamily.Target == *s.DDL.PGOperatorFamily {
			state := *object.PGOperatorFamily
			s.Metadata.PGOperatorFamily = &state
		}
		if object != nil && object.PGOperatorFamilyMember != nil && lookup.PGOperatorFamilyMember != nil &&
			object.PGOperatorFamilyMember.Target == *s.DDL.PGOperatorFamily && object.PGOperatorFamilyMember.Member == *lookup.PGOperatorFamilyMember {
			state := *object.PGOperatorFamilyMember
			s.Metadata.PGOperatorFamilyMember = &state
		}
		if object != nil && object.PGOperatorFamilyChange != nil && s.DDL.PGOperatorFamilyChange != nil &&
			object.PGOperatorFamilyChange.Target == *s.DDL.PGOperatorFamilyChange {
			state := *object.PGOperatorFamilyChange
			s.Metadata.PGOperatorFamilyChange = &state
		}
	}
	return nil
}
