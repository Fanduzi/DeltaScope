package audit

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func enrichPGForeignKeyReferences(ctx context.Context, request *MetadataRequest, statements []spec.Statement, snapshots map[[2]string]*spec.TableSnapshot) error {
	for _, s := range statements {
		target := spec.PGForeignKeyTarget(s)
		if target == nil || !target.ReferenceQualified() || !spec.PGOrdinaryTableBound(s) || !spec.PG17Observed(s.Metadata.Instance) {
			continue
		}
		schema, name := target.Reference.Schema, target.Reference.Name
		key := [2]string{schema, name}
		snapshot, cached := snapshots[key]
		if !cached {
			var err error
			snapshot, err = request.Provider.LoadTableSnapshot(ctx, spec.DialectPostgreSQL, schema, name)
			if err != nil {
				return metadataObjectReadError{cause: err}
			}
			snapshots[key] = snapshot
		}
		if snapshot != nil && snapshot.Schema == schema && snapshot.PGRelation.Name == name &&
			(snapshot.Table == nil || snapshot.Table.Name == name) {
			s.Metadata.PGForeignKeyReference = &spec.TableSnapshot{Schema: snapshot.Schema, Exists: snapshot.Exists,
				PGRelation: snapshot.PGRelation, PGColumns: snapshot.PGColumns.Clone()}
		}
	}
	return nil
}
