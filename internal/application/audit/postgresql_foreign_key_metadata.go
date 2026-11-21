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
		ref, keys, err := loadPGForeignKeyReference(ctx, request, target.Reference, snapshots)
		if err != nil {
			return err
		}
		s.Metadata.PGForeignKeyReference, s.Metadata.PGForeignKeyKeys = ref, keys
		if keys == nil {
			continue
		}
		if len(keys.Keys) == 0 || len(target.ReferencedColumns) > 0 && len(target.ReferencedColumns) != len(target.Columns) {
			continue
		}
		equality := spec.PGForeignKeyEqualityTarget{Source: *s.DDL.Table, Reference: target.Reference, Columns: target.Columns, ReferencedColumns: target.ReferencedColumns}
		lookupTarget := equality
		lookupTarget.Columns = slices.Clone(equality.Columns)
		lookupTarget.ReferencedColumns = slices.Clone(equality.ReferencedColumns)
		result, err := resolveObjectSnapshot(ctx, request, spec.DialectPostgreSQL, &spec.ObjectLookupRequest{Type: "foreign_key_scalar_equality", PGForeignKeyEqualityTarget: &lookupTarget})
		if err != nil {
			return err
		}
		if result != nil && result.PGForeignKeyEquality != nil && result.PGForeignKeyEquality.Target.Matches(equality) {
			equality.Columns = slices.Clone(equality.Columns)
			equality.ReferencedColumns = slices.Clone(equality.ReferencedColumns)
			s.Metadata.PGForeignKeyEquality = &spec.PGForeignKeyEquality{Target: equality, IncompatiblePosition: result.PGForeignKeyEquality.IncompatiblePosition}
		}
	}
	return nil
}

func loadPGForeignKeyReference(ctx context.Context, request *MetadataRequest, target spec.Table, snapshots map[[2]string]*spec.TableSnapshot) (*spec.TableSnapshot, *spec.PGForeignKeyKeys, error) {
	schema, name := target.Schema, target.Name
	key := [2]string{schema, name}
	snapshot, cached := snapshots[key]
	if !cached {
		var err error
		snapshot, err = request.Provider.LoadTableSnapshot(ctx, spec.DialectPostgreSQL, schema, name)
		if err != nil {
			return nil, nil, metadataObjectReadError{cause: err}
		}
		snapshots[key] = snapshot
	}
	if snapshot == nil || snapshot.Schema != schema || snapshot.PGRelation.Name != name ||
		(snapshot.Table != nil && snapshot.Table.Name != name) {
		return nil, nil, nil
	}
	ref := &spec.TableSnapshot{Schema: snapshot.Schema, Exists: snapshot.Exists,
		PGRelation: snapshot.PGRelation, PGColumns: snapshot.PGColumns.Clone()}
	if snapshot.PGRelation.Kind != "r" || !snapshot.Exists || snapshot.PGColumns == nil || !snapshot.PGColumns.Ordinary || snapshot.PGColumns.Typed {
		return ref, nil, nil
	}
	lookup := spec.ObjectLookupRequest{Type: "foreign_key_reference_keys", Schema: schema, Name: name}
	object, err := resolveObjectSnapshot(ctx, request, spec.DialectPostgreSQL, &lookup)
	if err != nil || object == nil || object.PGForeignKeyKeys == nil {
		return ref, nil, err
	}
	keys := *object.PGForeignKeyKeys
	if keys.Schema != schema || keys.Table != name {
		return ref, nil, nil
	}
	keys.Keys = slices.Clone(keys.Keys)
	for i := range keys.Keys {
		keys.Keys[i].Columns = slices.Clone(keys.Keys[i].Columns)
	}
	return ref, &keys, nil
}
