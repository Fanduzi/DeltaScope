package spec

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPGConstraintDirectoryProof(t *testing.T) {
	for _, tc := range []struct {
		name, target, kind string
		members            []PGConstraintState
		known              bool
	}{
		{name: "unobserved", target: "ck"},
		{name: "known empty", target: "ck", members: []PGConstraintState{}, known: true},
		{name: "exact lower", target: "ck", members: []PGConstraintState{{Name: "ck", Type: "c"}, {Name: "CK", Type: "u"}}, known: true, kind: "c"},
		{name: "exact quoted", target: "CK", members: []PGConstraintState{{Name: "ck", Type: "c"}, {Name: "CK", Type: "u"}}, known: true, kind: "u"},
		{name: "no case fold", target: "Ck", members: []PGConstraintState{{Name: "ck", Type: "c"}}, known: true},
		{name: "duplicate", target: "ck", members: []PGConstraintState{{Name: "ck", Type: "c"}, {Name: "ck", Type: "f"}}},
		{name: "identical duplicate", target: "ck", members: []PGConstraintState{{Name: "ck", Type: "c"}, {Name: "ck", Type: "c"}}},
		{name: "unowned name", target: "ck", members: []PGConstraintState{{Name: "ck", Type: "c"}, {Type: "f"}}},
		{name: "unknown type", target: "ck", members: []PGConstraintState{{Name: "ck", Type: "future"}}},
		{name: "missing type", target: "ck", members: []PGConstraintState{{Name: "ck"}}},
		{name: "missing target", members: []PGConstraintState{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, known := (TableSnapshot{PGConstraints: tc.members}).FindPGConstraint(tc.target)
			if known != tc.known || (m != nil) != (tc.kind != "") || m != nil && m.Type != tc.kind {
				t.Fatalf("member=%+v known=%v", m, known)
			}
		})
	}
}

func TestPGConstraintDirectoryNotSerialized(t *testing.T) {
	s := TableSnapshot{PGConstraints: []PGConstraintState{{Name: "hidden_constraint_marker", Type: "f", Validated: true}}}
	raw, err := json.Marshal(s)
	if err != nil || strings.Contains(string(raw), "hidden_constraint_marker") || strings.Contains(string(raw), "PGConstraints") {
		t.Fatalf("raw=%s err=%v", raw, err)
	}
}
