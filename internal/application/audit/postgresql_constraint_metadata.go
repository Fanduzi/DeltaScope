package audit

import "github.com/Fanduzi/DeltaScope/internal/domain/spec"

// invalidateStalePGTargetCatalogs withholds catalog proof after unapplied DDL.
// It returns the prefix whose cluster-wide identities can still use live facts.
// It never predicts a post-state or mutates the provider's cached snapshot.
func invalidateStalePGTargetCatalogs(statements []spec.Statement, failures []parseFailure) int {
	freshPrefix := len(statements)
	stale := make(map[[2]string]bool)
	allStale, earlierDDL := false, false
	var earliest *parseFailure
	for i := range failures {
		f := &failures[i]
		if earliest == nil || failureBefore(f.Line, f.Column, earliest.Line, earliest.Column) {
			earliest = f
		}
	}
	for i, statement := range statements {
		if earliest != nil && (statement.Line <= 0 || earliest.Line <= 0 || failureBefore(earliest.Line, earliest.Column, statement.Line, statement.Column)) {
			allStale = true
		}
		if (allStale || earlierDDL) && freshPrefix == len(statements) {
			freshPrefix = i
		}
		if m := statement.Metadata; m != nil && m.TargetTable != nil {
			name := m.TargetTable.PGRelation.Name
			if m.TargetTable.Table != nil {
				name = m.TargetTable.Table.Name
			}
			if statement.DDL != nil && statement.DDL.Table != nil {
				name = statement.DDL.Table.Name
			}
			staleMembers := allStale || stale[[2]string{m.Schema, name}]
			// Earlier DDL can create implicit relations under other names.
			staleRelation := earlierDDL && m.TargetTable.PGRelation.Name != ""
			if staleMembers || staleRelation {
				metadata := *m
				snapshot := *m.TargetTable
				if staleMembers {
					snapshot.PGConstraints = nil
					snapshot.PGColumns = nil
				}
				snapshot.PGRelation = spec.PGRelationState{}
				metadata.TargetTable = &snapshot
				statements[i].Metadata = &metadata
			}
		}

		if statement.Unsupported != nil || statement.Kind == spec.KindUnknown || statement.ResourceLimit != nil {
			allStale = true
			continue
		}
		if statement.Kind != spec.KindDDL {
			continue
		}
		earlierDDL = true
		ddl := statement.DDL
		if ddl == nil || len(ddl.TableTargets()) == 0 || ddl.OmittedTargets > 0 || ddl.Operation == spec.DDLOperationDropTable || ddl.Operation == spec.DDLOperationRenameTable {
			allStale = true
			continue
		}
		for _, target := range ddl.TableTargets() {
			if target.Name == "" {
				allStale = true
			}
			schema := target.Schema
			if schema == "" && statement.Metadata != nil {
				schema = statement.Metadata.Schema
			}
			stale[[2]string{schema, target.Name}] = true
		}
		for _, alter := range ddl.Alter {
			// These effects can reach another identity or a dependent table.
			if alter.Action == "rename_table" || alter.Action == "set_schema" || alter.Action == "drop_column" || alter.Options["cascade"] == "true" {
				allStale = true
			}
		}
	}
	return freshPrefix
}
