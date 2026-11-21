//go:build postgresql

package deltascope

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestAuditPGConstraintTargetContract(t *testing.T) {
	for _, tc := range []struct {
		sql, id, verdict string
		members          []PGConstraintState
	}{
		{`ALTER TABLE app.t ADD CONSTRAINT " Exact Check " CHECK (1 > 0) NOT VALID`, "ddl.pg.alter.add_check.name.available", "reject", []PGConstraintState{{Name: " Exact Check ", Type: "c", Validated: true}}},
		{`ALTER TABLE app.t ADD CHECK (id > (SELECT 0)) NOT VALID`, "ddl.pg.alter.add_check.name.available", "reject", nil},
		{`ALTER TABLE app.t ADD UNIQUE (id, id)`, "ddl.pg.alter.constraint_columns.valid", "reject", nil},
		{`ALTER TABLE app.t ADD FOREIGN KEY (id) REFERENCES app.t ("Missing") NOT VALID`, "ddl.pg.alter.add_foreign_key.targets.valid", "reject", []PGConstraintState{}},
		{"ALTER TABLE ONLY app.t VALIDATE CONSTRAINT c", "ddl.pg.alter.validate_constraint.target.require", "reject", []PGConstraintState{{Name: "c", Type: "c", CheckValidationChildrenKnown: true, CheckValidationHasChildren: true}}},
		{"ALTER TABLE app.t VALIDATE CONSTRAINT c", "ddl.pg.alter.validate_constraint.target.require", "reject", []PGConstraintState{{Name: "c", Type: "u", Validated: true}}},
		{"ALTER TABLE app.t ALTER CONSTRAINT c DEFERRABLE", "ddl.pg.alter.constraint_deferrability.target.require", "reject", []PGConstraintState{{Name: "c", Type: "c", Validated: true}}},
		{"ALTER TABLE app.t DROP CONSTRAINT IF EXISTS c CASCADE", "ddl.pg.alter.drop_constraint.target.require", "reject", []PGConstraintState{{Name: "c", Type: "c", Validated: true, InheritanceKnown: true, Inherited: true}}},
		{"ALTER TABLE ONLY app.t DROP CONSTRAINT c CASCADE", "ddl.pg.alter.drop_constraint.target.require", "reject", []PGConstraintState{{Name: "c", Type: "c", Validated: true, InheritanceKnown: true, PartitionCheckHasChildrenKnown: true, PartitionCheckHasChildren: true}}},
		{"ALTER TABLE app.t DROP CONSTRAINT c", "ddl.pg.alter.drop_constraint.target.require", "reject", []PGConstraintState{}},
		{"ALTER TABLE app.t DROP CONSTRAINT IF EXISTS c", "", "pass", []PGConstraintState{}},
		{"ALTER TABLE app.t VALIDATE CONSTRAINT c", "", "pass", []PGConstraintState{{Name: "c", Type: "c", Validated: true}}},
	} {
		kind := "r"
		if len(tc.members) == 1 && tc.members[0].PartitionCheckHasChildrenKnown {
			kind = "p"
		}
		p := &fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"}, snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"}, PGConstraints: tc.members, PGRelation: PGRelationState{Name: "t", Kind: kind}, PGColumns: &PGColumnCatalog{Ordinary: true, Columns: []PGColumnState{{Name: "id"}}}, Options: map[string]string{"private": "private_catalog_payload"}}}
		r, err := Audit(context.Background(), Request{SQL: tc.sql, Dialect: DialectPostgreSQL, Schema: "app", MetadataProvider: p})
		if err != nil || string(r.Verdict) != tc.verdict {
			t.Fatalf("result=%+v err=%v", r, err)
		}
		found := false
		for _, s := range r.Statements {
			if len(s.EvidenceGaps) != 0 {
				t.Fatalf("unexpected gap: %+v", s.EvidenceGaps)
			}
			for _, f := range s.Findings {
				if f.RuleID == tc.id {
					found = true
					if f.Level != "blocker" {
						t.Fatal(f.Level)
					}
				}
				raw, err := json.Marshal(f)
				if err != nil || strings.Contains(string(raw), "private_catalog_payload") {
					t.Fatalf("finding=%s err=%v", raw, err)
				}
			}
		}
		if found != (tc.id != "") {
			t.Fatalf("missing target finding %q", tc.id)
		}
	}
	p := &fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"}, snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"}, PGConstraints: []PGConstraintState{{Name: "c", Type: "p", Validated: true}}}}
	r, err := Audit(context.Background(), Request{SQL: "ALTER TABLE app.t VALIDATE CONSTRAINT c;ALTER TABLE app.t ALTER CONSTRAINT x NOT ENFORCED", Dialect: DialectPostgreSQL, Schema: "app", MetadataProvider: p})
	if err == nil || len(r.Diagnostics) != 1 || string(r.Verdict) != "reject" || len(r.Statements) != 1 {
		t.Fatalf("lost partial target check: %+v %v", r, err)
	}
	found := false
	for _, f := range r.Statements[0].Findings {
		if f.RuleID == "ddl.pg.alter.validate_constraint.target.require" {
			found = true
		}
	}
	if !found {
		t.Fatal("lost blocker beside parser-error boundary")
	}
}

func TestPublicPGConstraintCloneOwnsProof(t *testing.T) {
	for _, members := range [][]PGConstraintState{nil, {}, {{Name: "c", Type: "c", Validated: true}}} {
		source := &TableSnapshot{PGConstraints: members}
		cloned := cloneTableSnapshot(source)
		if (cloned.PGConstraints == nil) != (source.PGConstraints == nil) || len(cloned.PGConstraints) != len(members) {
			t.Fatal("nil/empty proof lost")
		}
		if len(members) > 0 {
			cloned.PGConstraints[0].Validated = false
			if !source.PGConstraints[0].Validated {
				t.Fatal("provider catalog aliased")
			}
		}
	}
}
