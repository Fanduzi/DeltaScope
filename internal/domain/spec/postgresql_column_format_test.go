package spec

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPGColumnSystemDirectoryProof(t *testing.T) {
	for _, tc := range []struct {
		name, target           string
		catalog                *PGColumnCatalog
		known, present, system bool
	}{
		{"user-empty", "ghost", &PGColumnCatalog{Ordinary: true, Columns: []PGColumnState{}}, true, false, false},
		{"system-unobserved", "ctid", &PGColumnCatalog{Ordinary: true, Columns: []PGColumnState{}}, false, false, false},
		{"empty-system-is-not-absence", "ctid", &PGColumnCatalog{Ordinary: true, Columns: []PGColumnState{}, SystemColumns: []PGColumnState{}}, false, false, false},
		{"observed-system", "ctid", &PGColumnCatalog{Ordinary: true, Columns: []PGColumnState{}, SystemColumns: []PGColumnState{{Name: "ctid", System: true, TypeStorage: "p"}}}, true, true, true},
		{"quoted-user", "CTID", &PGColumnCatalog{Ordinary: true, Columns: []PGColumnState{{Name: "CTID", TypeStorage: "x"}}, SystemColumns: []PGColumnState{{Name: "ctid", System: true, TypeStorage: "p"}}}, true, true, false},
		{"not-inferred-from-name", "ctid", &PGColumnCatalog{Ordinary: true, Columns: []PGColumnState{{Name: "ctid"}}}, false, false, false},
		{"missing-negative-marker", "ctid", &PGColumnCatalog{Ordinary: true, Columns: []PGColumnState{}, SystemColumns: []PGColumnState{{Name: "ctid"}}}, false, false, false},
		{"duplicate-system", "ctid", &PGColumnCatalog{Ordinary: true, Columns: []PGColumnState{}, SystemColumns: []PGColumnState{{Name: "ctid", System: true}, {Name: "ctid", System: true}}}, false, false, false},
		{"misclassified-user", "id", &PGColumnCatalog{Ordinary: true, Columns: []PGColumnState{{Name: "id", System: true}}}, false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, known := tc.catalog.FindColumn(tc.target)
			if known != tc.known || (m != nil) != tc.present || m != nil && m.System != tc.system {
				t.Fatalf("member=%+v known=%v", m, known)
			}
		})
	}
	for _, name := range []string{"ctid", "xmin", "cmin", "xmax", "cmax", "tableoid"} {
		c := &PGColumnCatalog{Ordinary: true, Columns: []PGColumnState{}}
		if m, known := c.FindColumn(name); m != nil || known {
			t.Fatalf("%s synthesized or guessed absent", name)
		}
	}
	c := &PGColumnCatalog{Ordinary: true, Columns: []PGColumnState{}, SystemColumns: []PGColumnState{{Name: "ctid", System: true, TypeStorage: "p"}}}
	cloned := c.Clone()
	cloned.SystemColumns[0].TypeStorage = "x"
	if c.SystemColumns[0].TypeStorage != "p" || cloned.Columns == nil {
		t.Fatal("system clone aliases or loses user known-empty")
	}
	b, err := json.Marshal(TableSnapshot{PGColumns: c})
	if err != nil || strings.Contains(string(b), "ctid") || strings.Contains(string(b), "TypeStorage") {
		t.Fatalf("private system catalog leaked: %s %v", b, err)
	}
}

func TestPGCompressionCapabilityPrivacy(t *testing.T) {
	b, err := json.Marshal(InstanceFacts{Version: "PostgreSQL 17.7", PGCompressionLZ4Known: true, PGCompressionLZ4Supported: true})
	if err != nil || strings.Contains(strings.ToLower(string(b)), "lz4") || strings.Contains(strings.ToLower(string(b)), "compression") {
		t.Fatalf("private build fact leaked: %s %v", b, err)
	}
}
