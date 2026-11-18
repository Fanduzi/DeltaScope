package audit

import (
	"context"
	"reflect"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func pgConstraintMetadataStatement(schema, table, action string, snapshot *spec.TableSnapshot) spec.Statement {
	return spec.Statement{Dialect: spec.DialectPostgreSQL, Kind: spec.KindDDL, Line: 1, Column: 1,
		DDL:      &spec.DDL{Operation: spec.DDLOperationAlterTable, Table: &spec.Table{Schema: schema, Name: table}, Alter: []spec.Alter{{Action: action, Name: "ck"}}},
		Metadata: &spec.Metadata{Schema: schema, TargetTable: snapshot}}
}

func TestPGConstraintMetadataFreshness(t *testing.T) {
	for _, name := range []string{"same target", "other target", "other schema", "quoted identity", "unsupported", "drop cascade", "drop table", "rename", "omitted targets", "read only", "prior failure", "later failure"} {
		t.Run(name, func(t *testing.T) {
			source := &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGConstraints: []spec.PGConstraintState{{Name: "ck", Type: "c", Validated: true}}, PGColumns: &spec.PGColumnCatalog{Ordinary: true, Columns: []spec.PGColumnState{}}}
			first := pgConstraintMetadataStatement("app", "t", "add_constraint", source)
			second := pgConstraintMetadataStatement("app", "t", "validate_constraint", source)
			second.Line = 3
			wantStale := true
			var failures []parseFailure
			switch name {
			case "other target":
				first.DDL.Table.Name = "u"
				wantStale = false
			case "other schema":
				first.DDL.Table.Schema = "other"
				first.Metadata.Schema = "other"
				wantStale = false
			case "quoted identity":
				first.DDL.Table.Name = "T"
				wantStale = false
			case "unsupported":
				first.Unsupported = &spec.UnsupportedDetail{Feature: "unknown"}
			case "drop cascade":
				first.DDL.Alter[0].Action = "drop_constraint"
				first.DDL.Alter[0].Options = map[string]string{"cascade": "true"}
			case "drop table":
				first.DDL.Operation = spec.DDLOperationDropTable
			case "rename":
				first.DDL.Alter[0].Action = "rename_table"
			case "omitted targets":
				first.DDL.OmittedTargets = 1
			case "read only":
				first.Kind = spec.KindDML
				first.DDL = nil
				wantStale = false
			case "prior failure", "later failure":
				first.Kind = spec.KindDML
				first.DDL = nil
				line := 2
				if name == "later failure" {
					line = 4
					wantStale = false
				}
				failures = []parseFailure{{Line: line, Column: 1}}
			}
			if name == "drop cascade" || name == "drop table" || name == "rename" || name == "unsupported" || name == "omitted targets" {
				first.DDL.Table.Name = "unrelated"
			}
			statements := []spec.Statement{first, second}
			invalidateStalePGTargetCatalogs(statements, failures)
			if (statements[1].Metadata.TargetTable.PGConstraints == nil) != wantStale || (statements[1].Metadata.TargetTable.PGColumns == nil) != wantStale {
				t.Fatalf("stale=%v expected=%v", statements[1].Metadata.TargetTable.PGConstraints == nil, wantStale)
			}
			if statements[0].Metadata.TargetTable.PGConstraints == nil || statements[0].Metadata.TargetTable.PGColumns == nil || source.PGColumns == nil || len(source.PGConstraints) != 1 || !source.PGConstraints[0].Validated {
				t.Fatal("provider or earlier statement mutated")
			}
		})
	}
}

func TestPGConstraintFailureOrdering(t *testing.T) {
	for _, tc := range []struct {
		failures                 []parseFailure
		prefixStale, targetStale bool
	}{
		{[]parseFailure{{Line: 5, Column: 1}, {Line: 2, Column: 1}}, false, true},
		{[]parseFailure{{Line: 5, Column: 1}, {Line: 4, Column: 1}}, false, false},
		{[]parseFailure{{Line: 5, Column: 1}, {Line: 0}}, true, true},
	} {
		source := &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGConstraints: []spec.PGConstraintState{}, PGColumns: &spec.PGColumnCatalog{Ordinary: true, Columns: []spec.PGColumnState{}}}
		prefix := pgConstraintMetadataStatement("app", "t", "", source)
		prefix.Kind, prefix.DDL = spec.KindDML, nil
		target := pgConstraintMetadataStatement("app", "t", "validate_constraint", source)
		target.Line = 3
		statements := []spec.Statement{prefix, target}
		invalidateStalePGTargetCatalogs(statements, tc.failures)
		if (statements[0].Metadata.TargetTable.PGConstraints == nil) != tc.prefixStale || (statements[1].Metadata.TargetTable.PGConstraints == nil) != tc.targetStale || (statements[0].Metadata.TargetTable.PGColumns == nil) != tc.prefixStale || (statements[1].Metadata.TargetTable.PGColumns == nil) != tc.targetStale || source.PGConstraints == nil || source.PGColumns == nil {
			t.Fatalf("lost earliest-failure/prefix proof: %+v", tc)
		}
	}
}

func TestPGConstraintMetadataIdentityCache(t *testing.T) {
	p := &fakeMetadataProvider{instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"}, snapshot: &spec.TableSnapshot{Exists: true, Table: &spec.Table{Name: "t"}, PGConstraints: []spec.PGConstraintState{}}}
	var statements []spec.Statement
	for _, id := range [][2]string{{"app", "t"}, {"app", "T"}, {"a.b", "c"}, {"a", "b.c"}, {"app", "t"}} {
		statements = append(statements, pgConstraintMetadataStatement(id[0], id[1], "validate_constraint", nil))
	}
	enriched, err := enrichStatementsWithMetadata(context.Background(), spec.DialectPostgreSQL, &MetadataRequest{Schema: "fallback", Provider: p}, statements, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p.tableCalls, []string{"t", "T", "c", "b.c"}) || !reflect.DeepEqual(p.tableSchemas, []string{"app", "app", "a.b", "a"}) {
		t.Fatalf("calls=%v schemas=%v", p.tableCalls, p.tableSchemas)
	}
	if enriched[0].Metadata.Schema != "app" || enriched[4].Metadata.TargetTable.PGConstraints != nil || p.snapshot.PGConstraints == nil {
		t.Fatal("binding/freshness or cache immutability failed")
	}
}

func TestPGConstraintMetadataClonePreservesProof(t *testing.T) {
	for _, members := range [][]spec.PGConstraintState{nil, {}, {{Name: "ck", Type: "c", Validated: true}}} {
		s := &spec.TableSnapshot{PGConstraints: members}
		cloned := cloneTableSnapshot(s)
		if !reflect.DeepEqual(s, cloned) {
			t.Fatalf("clone lost proof: %+v", cloned)
		}
		if len(cloned.PGConstraints) > 0 {
			cloned.PGConstraints[0].Validated = false
			if !s.PGConstraints[0].Validated {
				t.Fatal("clone aliases catalog")
			}
		}
	}
}
