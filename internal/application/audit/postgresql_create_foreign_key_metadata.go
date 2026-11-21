package audit

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func enrichPGCreateForeignKeys(ctx context.Context, request *MetadataRequest, statements []spec.Statement, snapshots map[[2]string]*spec.TableSnapshot) error {
	for _, s := range statements {
		if s.DDL == nil || s.DDL.PGCreateForeignKeys == nil || s.Metadata == nil {
			continue
		}
		decl := s.DDL.PGCreateForeignKeys
		destination := s.Metadata.PGCreateDestination
		if decl.Unbound || destination == nil || destination.Target != decl.Target || !destination.SchemaExists || destination.RelationExists {
			continue
		}
		refs := make([]spec.PGCreateForeignKeyReference, len(decl.Declarations))
		for i, fk := range decl.Declarations {
			if !fk.CreateReferenceQualified() || fk.Reference == decl.Target.Table {
				continue
			}
			ref, keys, err := loadPGForeignKeyReference(ctx, request, fk.Reference, snapshots)
			if err != nil {
				return err
			}
			refs[i] = spec.PGCreateForeignKeyReference{Target: fk.Reference, Table: ref, Keys: keys}
		}
		s.Metadata.PGCreateForeignKeys = refs
	}
	return nil
}
