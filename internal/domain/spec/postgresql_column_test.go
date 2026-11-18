package spec

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPGColumnCatalogProof(t *testing.T) {
	for _, tc := range []struct {
		name           string
		catalog        *PGColumnCatalog
		known, present bool
	}{
		{"unknown", nil, false, false},
		{"wrong-scope", &PGColumnCatalog{Columns: []PGColumnState{{Name: "id"}}}, false, false},
		{"unobserved-members", &PGColumnCatalog{Ordinary: true}, false, false},
		{"known-empty", &PGColumnCatalog{Ordinary: true, Columns: []PGColumnState{}}, true, false},
		{"exact", &PGColumnCatalog{Ordinary: true, Columns: []PGColumnState{{Name: "id", Identity: "d"}}}, true, true},
		{"quoted-not-folded", &PGColumnCatalog{Ordinary: true, Columns: []PGColumnState{{Name: "ID", Identity: "a"}}}, true, false},
		{"both-identities", &PGColumnCatalog{Ordinary: true, Columns: []PGColumnState{{Name: "ID"}, {Name: "id"}}}, true, true},
		{"duplicate", &PGColumnCatalog{Ordinary: true, Columns: []PGColumnState{{Name: "id"}, {Name: "id", Identity: "a"}}}, false, false},
		{"unnamed", &PGColumnCatalog{Ordinary: true, Columns: []PGColumnState{{Name: ""}}}, false, false},
		{"unknown-mode", &PGColumnCatalog{Ordinary: true, Columns: []PGColumnState{{Name: "id", Identity: "x"}}}, false, false},
		{"unknown-generation", &PGColumnCatalog{Ordinary: true, Columns: []PGColumnState{{Name: "id", Generated: "v"}}}, false, false},
		{"conflicting-kinds", &PGColumnCatalog{Ordinary: true, Columns: []PGColumnState{{Name: "id", Identity: "a", Generated: "s"}}}, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			member, known := tc.catalog.FindColumn("id")
			if known != tc.known || (member != nil) != tc.present {
				t.Fatalf("member=%+v known=%v", member, known)
			}
		})
	}
}

func TestPGColumnCatalogCloneAndPrivacy(t *testing.T) {
	value := int64(2147483648)
	c := &PGColumnCatalog{Ordinary: true, Columns: []PGColumnState{{Name: "hidden_member", Identity: "d", IdentitySequence: &PGIdentitySequenceState{TypeWidth: 64, Definition: SequenceDefinition{Start: &value}}}}}
	copy := c.Clone()
	copy.Columns[0].Identity = "a"
	*copy.Columns[0].IdentitySequence.Definition.Start = 7
	if c.Columns[0].Identity != "d" || value != 2147483648 {
		t.Fatal("catalog clone aliases source")
	}
	if (&PGColumnCatalog{Ordinary: true, Columns: []PGColumnState{}}).Clone().Columns == nil {
		t.Fatal("known-empty became unknown")
	}
	var empty *PGColumnCatalog
	if empty.Clone() != nil {
		t.Fatal("unknown became known")
	}
	b, err := json.Marshal(TableSnapshot{PGColumns: c})
	if err != nil || strings.Contains(string(b), "hidden_member") || strings.Contains(string(b), "PGColumns") {
		t.Fatalf("private catalog leaked: %s %v", b, err)
	}
	b, err = json.Marshal(Alter{Action: "set_generated", PGColumnTransition: &PGColumnTransition{Mode: "private_mode", ModeCount: 2}})
	if err != nil || strings.Contains(string(b), "private_mode") || strings.Contains(string(b), "PGColumnTransition") {
		t.Fatalf("private declaration leaked: %s %v", b, err)
	}
}
