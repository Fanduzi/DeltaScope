package audit

import "github.com/Fanduzi/DeltaScope/internal/domain/spec"

// invalidateStalePGConstraints withholds catalog proof after unapplied DDL;
// it never predicts a post-state or mutates the provider's cached snapshot.
func invalidateStalePGConstraints(statements []spec.Statement, failures []parseFailure) {
	stale := make(map[[2]string]bool)
	allStale := false
	for i, statement := range statements {
		for _, failure := range failures {
			if statement.Line <= 0 || failure.Line <= 0 || failureBefore(failure.Line, failure.Column, statement.Line, statement.Column) {
				allStale = true
				break
			}
		}
		if m := statement.Metadata; m != nil && m.TargetTable != nil && m.TargetTable.Table != nil {
			name := m.TargetTable.Table.Name
			if statement.DDL != nil && statement.DDL.Table != nil {
				name = statement.DDL.Table.Name
			}
			key := [2]string{m.Schema, name}
			if allStale || stale[key] {
				metadata := *m
				snapshot := *m.TargetTable
				snapshot.PGConstraints = nil
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
}
