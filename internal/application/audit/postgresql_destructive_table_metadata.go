package audit

import (
	"context"
	"slices"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func enrichPGDestructiveTableRelations(ctx context.Context, request *MetadataRequest, statements []spec.Statement, snapshots map[[2]string]*spec.TableSnapshot) error {
	for _, s := range statements {
		if s.Unsupported != nil || s.DDL == nil || s.DDL.PGDestructiveTable == nil || s.Metadata == nil || !spec.PG17Observed(s.Metadata.Instance) {
			continue
		}
		targets := s.DDL.PGDestructiveTable.Targets
		relations := make([]spec.PGDestructiveTableRelation, len(targets))
		for i, target := range targets {
			if !target.Qualified() {
				continue
			}
			schema, name := target.Table.Schema, target.Table.Name
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
				(snapshot.Table != nil && snapshot.Table.Name != name) || (snapshot.Exists && snapshot.PGRelation.Kind == "") {
				continue
			}
			relations[i] = spec.PGDestructiveTableRelation{Schema: schema, Relation: snapshot.PGRelation}
		}
		s.Metadata.PGDestructiveTableRelations = relations
		lookupType := "truncate_references"
		targets = spec.PGTruncateReferenceTargets(s)
		if s.DDL.Operation == spec.DDLOperationDropTable {
			lookupType = "drop_references"
			targets = spec.PGDropReferenceTargets(s)
		}
		if len(targets) == 0 {
			continue
		}
		lookup := spec.ObjectLookupRequest{Type: lookupType, PGDestructiveTargets: slices.Clone(targets), PGDropIfExists: s.DDL.PGDestructiveTable.IfExists}
		if s.DDL.PGDestructiveTable.Cascade {
			lookup.Qualifiers = map[string]string{"cascade": "true"}
		}
		object, err := resolveObjectSnapshot(ctx, request, spec.DialectPostgreSQL, &lookup)
		if err != nil {
			return err
		}
		if object != nil && object.PGDestructiveReferences != nil && object.PGDestructiveReferences.Matches(targets) {
			facts := *object.PGDestructiveReferences
			facts.Targets = slices.Clone(facts.Targets)
			s.Metadata.PGDestructiveReferences = &facts
		}
	}
	return nil
}
