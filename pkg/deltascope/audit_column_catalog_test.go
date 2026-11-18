package deltascope

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestPublicPGCompressionCapabilityClone(t *testing.T) {
	for _, source := range []InstanceFacts{
		{},
		{PGCompressionLZ4Known: true},
		{PGCompressionLZ4Known: true, PGCompressionLZ4Supported: true},
	} {
		cloned := cloneInstanceFacts(&source)
		if !reflect.DeepEqual(&source, cloned) {
			t.Fatal("capability known/false/unknown lost")
		}
		cloned.PGCompressionLZ4Known = !source.PGCompressionLZ4Known
		if cloned.PGCompressionLZ4Known == source.PGCompressionLZ4Known {
			t.Fatal("instance clone aliases source")
		}
		raw, err := json.Marshal(source)
		if err != nil || strings.Contains(strings.ToLower(string(raw)), "lz4") {
			t.Fatalf("private capability leaked: %s %v", raw, err)
		}
	}
}

func TestPublicPGColumnCatalogClone(t *testing.T) {
	value := int64(2147483648)
	for _, catalog := range []*PGColumnCatalog{{Ordinary: true, Columns: []PGColumnState{}, SystemColumns: []PGColumnState{{Name: "ctid", System: true, TypeStorage: "p"}}}, nil, {Ordinary: true, Columns: []PGColumnState{}}, {Ordinary: true, Columns: []PGColumnState{{Name: "private_catalog_member", Identity: "d", IdentitySequence: &PGIdentitySequenceState{TypeWidth: 64, Definition: SequenceDefinition{Start: &value}}}}}} {
		s := &TableSnapshot{PGColumns: catalog}
		cloned := cloneTableSnapshot(s)
		if !reflect.DeepEqual(s, cloned) {
			t.Fatal("public clone lost unknown/empty proof")
		}
		if catalog != nil && len(catalog.Columns) > 0 {
			cloned.PGColumns.Columns[0].Identity = "a"
			*cloned.PGColumns.Columns[0].IdentitySequence.Definition.Start = 0
			if catalog.Columns[0].Identity != "d" || value != 2147483648 {
				t.Fatal("public clone aliases source")
			}
		}
		if catalog != nil && len(catalog.SystemColumns) > 0 {
			cloned.PGColumns.SystemColumns[0].TypeStorage = "x"
			if catalog.SystemColumns[0].TypeStorage != "p" {
				t.Fatal("public clone aliases system directory")
			}
		}
		raw, err := json.Marshal(s)
		if err != nil || strings.Contains(string(raw), "private_catalog_member") || strings.Contains(string(raw), "PGColumns") || strings.Contains(string(raw), "ctid") {
			t.Fatalf("private facts leaked: %s %v", raw, err)
		}
	}
}
