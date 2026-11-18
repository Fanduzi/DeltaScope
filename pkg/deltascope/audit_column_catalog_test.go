package deltascope

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestPublicPGColumnCatalogClone(t *testing.T) {
	value := int64(2147483648)
	for _, catalog := range []*PGColumnCatalog{nil, {Ordinary: true, Columns: []PGColumnState{}}, {Ordinary: true, Columns: []PGColumnState{{Name: "private_catalog_member", Identity: "d", IdentitySequence: &PGIdentitySequenceState{TypeWidth: 64, Definition: SequenceDefinition{Start: &value}}}}}} {
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
		raw, err := json.Marshal(s)
		if err != nil || strings.Contains(string(raw), "private_catalog_member") || strings.Contains(string(raw), "PGColumns") {
			t.Fatalf("private facts leaked: %s %v", raw, err)
		}
	}
}
