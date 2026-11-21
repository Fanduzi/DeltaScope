package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGDropViewSetBindsRefusalToAuthoredPosition(t *testing.T) {
	targets := []spec.PGDestructiveTableTarget{
		{Table: spec.Table{Schema: "app", Name: "first"}},
		{Table: spec.Table{Schema: "other", Name: "second"}},
	}
	state := &spec.PGDropViewSetState{Targets: targets, IfExists: true, Position: 2, Reason: "drop_view_wrong_kind"}
	s := spec.Statement{Dialect: spec.DialectPostgreSQL, Kind: spec.KindDDL,
		DDL:      &spec.DDL{Operation: spec.DDLOperationDropView, PGDropViewTargets: targets, Options: map[string]string{"if_exists": "true"}},
		Metadata: &spec.Metadata{Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"}, PGDropViewSet: state},
	}
	r := pgDropViewTargetRule{level: rule.LevelBlocker}
	findings, err := r.Evaluate(context.Background(), s)
	if err != nil || len(findings) != 1 || findings[0].Metadata["view"] != "second" || findings[0].Metadata["schema"] != "other" {
		t.Fatalf("findings = %+v, err = %v", findings, err)
	}
	state.IfExists = false
	if gaps := r.EvidenceGaps(s); len(gaps) != 1 || gaps[0].RequiredFacts[0] != "view.pg_drop_state" {
		t.Fatalf("guard mismatch must leave evidence unverified, got %+v", gaps)
	}
}
