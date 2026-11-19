//go:build postgresql

package audit

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGConstraintTargetPipeline(t *testing.T) {
	for _, tc := range []struct {
		name, sql, verdict string
		members            []spec.PGConstraintState
		blockers, gaps     int
	}{
		{"wrong validate kind", "ALTER TABLE app.t VALIDATE CONSTRAINT c", "reject", []spec.PGConstraintState{{Name: "c", Type: "p", Validated: true}}, 1, 0},
		{"wrong deferrability kind", "ALTER TABLE app.t ALTER CONSTRAINT c DEFERRABLE", "reject", []spec.PGConstraintState{{Name: "c", Type: "c", Validated: true}}, 1, 0},
		{"missing drop", "ALTER TABLE app.t DROP CONSTRAINT c", "reject", []spec.PGConstraintState{}, 1, 0},
		{"guarded noop", "ALTER TABLE app.t DROP CONSTRAINT IF EXISTS c", "pass", []spec.PGConstraintState{}, 0, 0},
		{"validated noop", "ALTER TABLE app.t VALIDATE CONSTRAINT c", "pass", []spec.PGConstraintState{{Name: "c", Type: "c", Validated: true}}, 0, 0},
		{"unvalidated target", "ALTER TABLE app.t VALIDATE CONSTRAINT c", "pass", []spec.PGConstraintState{{Name: "c", Type: "f", Validated: false}}, 0, 0},
		{"unobserved catalog", "ALTER TABLE app.t VALIDATE CONSTRAINT c", "review", nil, 0, 1},
		{"add then validate", "ALTER TABLE app.t ADD CONSTRAINT c CHECK(id>0) NOT VALID;ALTER TABLE app.t VALIDATE CONSTRAINT c", "review", []spec.PGConstraintState{}, 0, 1},
		{"drop then validate", "ALTER TABLE app.t DROP CONSTRAINT c;ALTER TABLE app.t VALIDATE CONSTRAINT c", "review", []spec.PGConstraintState{{Name: "c", Type: "c", Validated: true}}, 0, 1},
		{"same statement prefix", "ALTER TABLE app.t ADD CONSTRAINT c CHECK(id>0) NOT VALID, VALIDATE CONSTRAINT c", "review", []spec.PGConstraintState{}, 0, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var original []spec.PGConstraintState
			if tc.members != nil {
				original = append([]spec.PGConstraintState{}, tc.members...)
			}
			snapshot := &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGConstraints: tc.members,
				PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}
			p := &fakeMetadataProvider{instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"}, snapshot: snapshot}
			r, err := AuditSQL(context.Background(), Request{SQL: tc.sql, Dialect: spec.DialectPostgreSQL, Schema: "app", MetadataProvider: p})
			if err != nil || string(r.Verdict) != tc.verdict {
				t.Fatalf("result=%+v err=%v", r, err)
			}
			blockers, gaps := 0, 0
			for _, s := range r.Statements {
				gaps += len(s.EvidenceGaps)
				for _, f := range s.Findings {
					if strings.HasSuffix(f.RuleID, ".target.require") && f.Level == "blocker" {
						blockers++
					}
				}
			}
			if blockers != tc.blockers || gaps != tc.gaps {
				t.Fatalf("blockers=%d gaps=%d result=%+v", blockers, gaps, r)
			}
			if tc.gaps > 0 && r.Coverage.Status != "unverified" || tc.gaps == 0 && r.Coverage.Status != "complete" {
				t.Fatalf("coverage=%+v", r.Coverage)
			}
			if !reflect.DeepEqual(snapshot.PGConstraints, original) {
				t.Fatal("provider catalog changed")
			}
		})
	}
}

func TestCorpusPGConstraintProofConversion(t *testing.T) {
	no := false
	for _, tc := range []struct {
		members []corpusPGConstraint
		known   bool
		count   int
	}{
		{nil, false, 0}, {[]corpusPGConstraint{}, true, 0},
		{[]corpusPGConstraint{{Name: "c", Type: "c"}}, false, 0},
		{[]corpusPGConstraint{{Name: "c", Type: "c", Validated: &no}}, true, 1},
	} {
		s := corpusTableSnapshotToSpec("t", "app", corpusTableSnapshot{Exists: true, PGConstraints: tc.members})
		if (s.PGConstraints != nil) != tc.known || len(s.PGConstraints) != tc.count {
			t.Fatalf("states=%+v known=%v", s.PGConstraints, tc.known)
		}
	}
}
