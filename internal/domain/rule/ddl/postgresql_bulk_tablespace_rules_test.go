package ddl

import (
	"context"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGBulkTablespaceAssessment(t *testing.T) {
	r := pgBulkTablespaceRule{level: rule.LevelBlocker}
	notice, err := newMoveAllTablespaceNoticeRule(policy.RulePolicy{})
	if err != nil {
		t.Fatal(err)
	}
	literal := spec.PGTableOwnerTarget{Kind: "named", Name: " Hidden Role "}
	public := spec.PGTableOwnerTarget{Kind: "public"}
	const scope = "instance.pg_bulk_tablespace_targets"
	for _, tc := range []struct {
		name, reason, fact string
		roles              []spec.PGTableOwnerTarget
		exists             []bool
		change             func(*spec.Statement)
		same               bool
	}{
		{name: "same endpoint", same: true},
		{name: "distinct endpoints", change: func(s *spec.Statement) { s.Metadata.PGBulkTablespace.Same = false }},
		{name: "literal before public", roles: []spec.PGTableOwnerTarget{literal, public}, exists: []bool{false, false}, reason: "owner_filter_role_absent"},
		{name: "public before literal", roles: []spec.PGTableOwnerTarget{public, literal}, exists: []bool{true, false}, reason: "owner_filter_public_inapplicable"},
		{name: "source before destination", reason: "source_tablespace_absent", change: func(s *spec.Statement) {
			s.Metadata.PGBulkTablespace.SourceExists = false
			s.Metadata.PGBulkTablespace.DestinationExists = false
		}},
		{name: "destination before global", reason: "destination_tablespace_absent", change: func(s *spec.Statement) {
			s.Metadata.PGBulkTablespace.SourceGlobal = true
			s.Metadata.PGBulkTablespace.DestinationExists = false
		}},
		{name: "global destination", reason: "tablespace_global_inapplicable", change: func(s *spec.Statement) { s.Metadata.PGBulkTablespace.DestinationGlobal = true }},
		{name: "unobserved alias", roles: []spec.PGTableOwnerTarget{{Kind: "session_user"}}, exists: []bool{false}, fact: scope},
		{name: "partial roles", roles: []spec.PGTableOwnerTarget{literal}, fact: scope},
		{name: "mismatched exact endpoint", fact: scope, change: func(s *spec.Statement) { s.Metadata.PGBulkTablespace.Target.Source = "source" }},
		{name: "unobserved", fact: "instance.version", change: func(s *spec.Statement) { s.Metadata = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target := spec.PGBulkTablespaceTarget{Source: " source ", Destination: "destination", Roles: tc.roles}
			s := spec.Statement{Dialect: spec.DialectPostgreSQL, Kind: spec.KindDDL,
				DDL:      &spec.DDL{Operation: spec.DDLOperationAlterTable, Alter: []spec.Alter{{Action: "move_all_tablespace", PGBulkTablespace: &target}}},
				Metadata: &spec.Metadata{Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"}, PGBulkTablespace: &spec.PGBulkTablespaceState{Target: target, SourceExists: true, DestinationExists: true, Same: true, RoleExists: tc.exists}}}
			if tc.change != nil {
				tc.change(&s)
			}
			findings, err := r.Evaluate(context.Background(), s)
			if err != nil || (len(findings) == 1) != (tc.reason != "") {
				t.Fatalf("findings=%+v err=%v", findings, err)
			}
			if len(findings) == 1 && (findings[0].Metadata["reason_code"] != tc.reason || len(findings[0].Metadata) != 2 || strings.Contains(findings[0].Message, literal.Name)) {
				t.Fatalf("finding=%+v", findings[0])
			}
			gaps := r.EvidenceGaps(s)
			if (len(gaps) == 1) != (tc.fact != "") || len(gaps) == 1 && (len(gaps[0].RequiredFacts) != 1 || gaps[0].RequiredFacts[0] != tc.fact) {
				t.Fatalf("gaps=%+v", gaps)
			}
			advice, err := notice.Evaluate(context.Background(), s)
			if err != nil || len(advice) != 1 || strings.Contains(advice[0].Message, "same observed") != tc.same || strings.Contains(advice[0].Explanation.Risk, "partway") {
				t.Fatalf("notice=%+v err=%v", advice, err)
			}
			s.Dialect = spec.DialectMySQL
			if r.AppliesTo(s) {
				t.Fatal("matched MySQL")
			}
		})
	}
}
