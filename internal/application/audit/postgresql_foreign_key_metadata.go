package audit

import (
	"context"
	"slices"

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
		if snapshot == nil || snapshot.Schema != schema || snapshot.PGRelation.Name != name ||
			(snapshot.Table != nil && snapshot.Table.Name != name) {
			continue
		}
		s.Metadata.PGForeignKeyReference = &spec.TableSnapshot{Schema: snapshot.Schema, Exists: snapshot.Exists,
			PGRelation: snapshot.PGRelation, PGColumns: snapshot.PGColumns.Clone()}
		if snapshot.PGRelation.Kind != "r" || !snapshot.Exists || snapshot.PGColumns == nil || !snapshot.PGColumns.Ordinary || snapshot.PGColumns.Typed {
			continue
		}
		lookup := spec.ObjectLookupRequest{Type: "foreign_key_reference_keys", Schema: schema, Name: name}
		object, err := resolveObjectSnapshot(ctx, request, spec.DialectPostgreSQL, &lookup)
		if err != nil {
			return err
		}
		if object == nil || object.PGForeignKeyKeys == nil {
			continue
		}
		keys := *object.PGForeignKeyKeys
		if keys.Schema != schema || keys.Table != name {
			continue
		}
		keys.Keys = slices.Clone(keys.Keys)
		for i := range keys.Keys {
			keys.Keys[i].Columns = slices.Clone(keys.Keys[i].Columns)
		}
		s.Metadata.PGForeignKeyKeys = &keys
	}
	return nil
}
