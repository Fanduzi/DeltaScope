package audit

import (
	"reflect"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGColumnMetadataCloneOwnsCatalog(t *testing.T) {
	value := int64(2147483648)
	for _, catalog := range []*spec.PGColumnCatalog{{Ordinary: true, Columns: []spec.PGColumnState{}, SystemColumns: []spec.PGColumnState{{Name: "ctid", System: true, TypeStorage: "p"}}}, nil, {Ordinary: true, Columns: []spec.PGColumnState{}}, {Ordinary: true, Columns: []spec.PGColumnState{{Name: "id", Identity: "d", IdentitySequence: &spec.PGIdentitySequenceState{TypeWidth: 64, Definition: spec.SequenceDefinition{Start: &value}}}}}} {
		s := &spec.TableSnapshot{PGColumns: catalog}
		cloned := cloneTableSnapshot(s)
		if !reflect.DeepEqual(s, cloned) {
			t.Fatal("clone changed proof shape")
		}
		if catalog != nil && len(catalog.SystemColumns) > 0 {
			cloned.PGColumns.SystemColumns[0].TypeStorage = "x"
			if catalog.SystemColumns[0].TypeStorage != "p" {
				t.Fatal("clone aliases system directory")
			}
		}
		if catalog != nil && len(catalog.Columns) > 0 {
			cloned.PGColumns.Columns[0].Identity = "a"
			*cloned.PGColumns.Columns[0].IdentitySequence.Definition.Start = 7
			if catalog.Columns[0].Identity != "d" || value != 2147483648 {
				t.Fatal("clone aliases catalog")
			}
		}
	}
}

func TestPGColumnMetadataSequenceDDLInvalidatesProof(t *testing.T) {
	source := &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true, Columns: []spec.PGColumnState{}}}
	first := spec.Statement{Dialect: spec.DialectPostgreSQL, Kind: spec.KindDDL, Line: 1, DDL: &spec.DDL{Operation: spec.DDLOperationAlterSequence, ObjectName: "renamed_identity_sequence"}}
	second := pgConstraintMetadataStatement("app", "t", "set_generated", source)
	second.Line = 3
	statements := []spec.Statement{first, second}
	invalidateStalePGTargetCatalogs(statements, nil)
	if statements[1].Metadata.TargetTable.PGColumns != nil || source.PGColumns == nil {
		t.Fatal("sequence definitions retained or source mutated")
	}
}
