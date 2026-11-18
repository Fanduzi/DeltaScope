package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGColumnIdentityProof(t *testing.T) {
	for _, tc := range []struct {
		name, action, source, destination, ruleID, reason, missing string
		guard, sibling, unobservedVersion, stale                   bool
	}{
		{name: "add duplicate", action: "add_column", source: "id", ruleID: ruleIDAlterAddColumnExistsForbid, reason: "column_already_exists"},
		{name: "guard duplicate", action: "add_column", source: "id", ruleID: ruleIDAlterAddColumnExistsForbid, guard: true},
		{name: "guard system", action: "add_column", source: "ctid", ruleID: ruleIDAlterAddColumnExistsForbid, reason: "system_column_name_conflict", guard: true},
		{name: "add uppercase user", action: "add_column", source: "CTID", ruleID: ruleIDAlterAddColumnExistsForbid},
		{name: "rename exact miss", action: "rename_column", source: "ID", ruleID: ruleIDAlterRenameColumnExistsRequire, reason: "column_absent"},
		{name: "rename system source", action: "rename_column", source: "ctid", ruleID: ruleIDAlterRenameColumnExistsRequire, reason: "system_column_inapplicable"},
		{name: "rename spaces", action: "rename_column", source: " ", ruleID: ruleIDAlterRenameColumnExistsRequire},
		{name: "rename exact case", action: "rename_column", source: "id", destination: "ID", ruleID: ruleIDAlterRenameColumnTargetExistsForbid},
		{name: "rename self", action: "rename_column", source: "id", destination: "id", ruleID: ruleIDAlterRenameColumnTargetExistsForbid, reason: "column_already_exists"},
		{name: "rename occupied spaces", action: "rename_column", source: "id", destination: " ", ruleID: ruleIDAlterRenameColumnTargetExistsForbid, reason: "column_already_exists"},
		{name: "rename unobserved system", action: "rename_column", source: "id", destination: "xmin", ruleID: ruleIDAlterRenameColumnTargetExistsForbid, missing: "target_table.pg_columns"},
		{name: "no destination finding on absent source", action: "rename_column", source: "missing", destination: "id", ruleID: ruleIDAlterRenameColumnTargetExistsForbid},
		{name: "phase ordering withheld", action: "add_column", source: "id", ruleID: ruleIDAlterAddColumnExistsForbid, missing: "target_table.pg_columns", sibling: true, guard: true},
		{name: "offline", action: "add_column", source: "id", ruleID: ruleIDAlterAddColumnExistsForbid, missing: "instance.version", unobservedVersion: true},
		{name: "stale", action: "rename_column", source: "id", ruleID: ruleIDAlterRenameColumnExistsRequire, missing: "target_table.pg_columns", stale: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := pgGenerationTransitionStatement(tc.action, nil)
			s.DDL.Alter[0].Name = tc.source
			s.DDL.Alter[0].Column = &spec.AlterColumn{OldName: tc.source, Definition: &spec.Column{Name: tc.destination}}
			s.DDL.Alter[0].PGAddColumnIfNotExists = tc.guard
			s.Metadata.TargetTable.PGColumns = &spec.PGColumnCatalog{Ordinary: true, Columns: []spec.PGColumnState{{Name: "id"}, {Name: " "}}, SystemColumns: []spec.PGColumnState{{Name: "ctid", System: true}}}
			if tc.sibling {
				s.DDL.Alter = append(s.DDL.Alter, spec.Alter{Action: "drop_column", Name: tc.source})
			}
			if tc.unobservedVersion {
				s.Metadata.Instance = nil
			}
			if tc.stale {
				s.Metadata.TargetTable.PGColumns = nil
			}
			reason, missing := pgColumnIdentityTarget(s, 0, tc.ruleID)
			if reason != tc.reason || missing != tc.missing {
				t.Fatalf("reason=%q missing=%q, want %q/%q", reason, missing, tc.reason, tc.missing)
			}
			wantNoop := tc.name == "guard duplicate"
			if tc.action == "add_column" && pgAddColumnNoop(s, 0) != wantNoop {
				t.Fatalf("expected no-op=%t", wantNoop)
			}
		})
	}
}

func TestPGGuardedAlterRelation(t *testing.T) {
	for _, tc := range []struct {
		name, qualifier, schema, relation, kind string
		legacyExists, exists, known             bool
	}{
		{name: "qualified absent", qualifier: "app", schema: "app", relation: "t", known: true},
		{name: "sequence filtered by table loader", qualifier: "app", schema: "app", relation: "t", kind: "S", exists: true, known: true},
		{name: "index filtered by table loader", qualifier: "app", schema: "app", relation: "t", kind: "i", exists: true, known: true},
		{name: "unqualified local miss", schema: "app", relation: "t"},
		{name: "legacy absence", qualifier: "app", schema: "app"},
		{name: "misbound schema", qualifier: "app", schema: "other", relation: "t"},
		{name: "misbound name", qualifier: "app", schema: "app", relation: "T"},
		{name: "temp alias", qualifier: "pg_temp", schema: "pg_temp", relation: "t"},
		{name: "contradictory", qualifier: "app", schema: "app", relation: "t", legacyExists: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := pgGenerationTransitionStatement("add_column", nil)
			s.DDL.Table.Schema, s.DDL.PGTableIfExists = tc.qualifier, true
			s.Metadata.Schema = tc.schema
			s.Metadata.TargetTable = &spec.TableSnapshot{Schema: tc.schema, Exists: tc.legacyExists, PGRelation: spec.PGRelationState{Name: tc.relation, Kind: tc.kind}}
			if exists, known := pgAlterRelation(s); exists != tc.exists || known != tc.known {
				t.Fatalf("exists=%t known=%t", exists, known)
			}
			r := tableExistenceRule{requireExist: true}
			if f, err := r.Evaluate(context.Background(), s); len(f) != 0 || err != nil {
				t.Fatalf("guarded relation findings=%v err=%v", f, err)
			}
			if gaps := r.EvidenceGaps(s); (len(gaps) != 0) == tc.known {
				t.Fatalf("unexpected evidence gaps: %v", gaps)
			}
		})
	}
}
