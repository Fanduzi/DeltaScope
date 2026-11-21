package ddl

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGViewOwnerAssessment(t *testing.T) {
	for _, tc := range []struct {
		name, kind, reason, gap string
		owner                   spec.PGTableOwnerTarget
		guard, known, exists    bool
	}{
		{name: "guarded absence", guard: true, known: true},
		{name: "missing source", known: true, reason: "view_owner_source_absent"},
		{name: "guarded wrong kind", kind: "m", guard: true, known: true, reason: "view_owner_source_wrong_kind"},
		{name: "guarded unknown keeps gap", guard: true, gap: "view.pg_owner"},
		{name: "missing named role", kind: "v", known: true, reason: "owner_role_absent"},
		{name: "literal role", kind: "v", known: true, exists: true},
		{name: "public pseudo-role", kind: "v", known: true, owner: spec.PGTableOwnerTarget{Kind: "public"}, reason: "owner_public_inapplicable"},
		{name: "unknown session role", kind: "v", known: true, owner: spec.PGTableOwnerTarget{Kind: "session_user"}, gap: "view.pg_owner"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			owner := tc.owner
			if owner.Kind == "" {
				owner = spec.PGTableOwnerTarget{Kind: "named", Name: " Private Owner "}
			}
			s := spec.Statement{Dialect: spec.DialectPostgreSQL, Kind: spec.KindDDL,
				DDL: &spec.DDL{Operation: spec.DDLOperationAlterView, Table: &spec.Table{Schema: "app", Name: "v"}, PGTableIfExists: tc.guard,
					Alter: []spec.Alter{{Action: "change_owner", PGTableOwner: owner}}},
				Metadata: &spec.Metadata{Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"}, PGTableOwner: &spec.PGTableOwnerState{Schema: "app", Table: "v", Target: owner,
					SourceKind: tc.kind, SourceKindKnown: tc.known, Exists: tc.exists}},
			}
			if reason, gap := assessPGViewOwner(s); reason != tc.reason || gap != tc.gap {
				t.Fatalf("got %q/%q, want %q/%q", reason, gap, tc.reason, tc.gap)
			}
			r := pgViewOwnerRule{level: rule.LevelBlocker}
			findings, err := r.Evaluate(context.Background(), s)
			if err != nil || (len(findings) == 1) != (tc.reason != "") || (len(r.EvidenceGaps(s)) == 1) != (tc.gap != "") {
				t.Fatalf("findings = %v, gaps = %v, err = %v", findings, r.EvidenceGaps(s), err)
			}
			data, err := json.Marshal(findings)
			if err != nil || strings.Contains(string(data), "Private Owner") {
				t.Fatalf("public findings = %s (%v)", data, err)
			}
		})
	}
}
