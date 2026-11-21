package ddl

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGSequenceChangeAssessment(t *testing.T) {
	for _, tc := range []struct {
		name, reason, fact string
		change             func(*spec.Statement, *spec.PGSequenceChange, *spec.PGSequenceChangeState)
	}{
		{name: "guard skips occupied relation", change: func(_ *spec.Statement, d *spec.PGSequenceChange, s *spec.PGSequenceChangeState) {
			d.IfNotExists, s.Kind = true, "r"
			d.Options = []spec.PGSequenceChangeOption{{Name: "increment", Value: new(int64(0)), Known: true}}
		}},
		{name: "guarded temporary namespace before relation", reason: "sequence_temporary_namespace", change: func(_ *spec.Statement, d *spec.PGSequenceChange, s *spec.PGSequenceChangeState) {
			d.IfNotExists, d.Persistence, s.Kind = true, "t", "r"
		}},
		{name: "descending defaults", change: func(_ *spec.Statement, d *spec.PGSequenceChange, _ *spec.PGSequenceChangeState) {
			d.Options = []spec.PGSequenceChangeOption{{Name: "as", Width: 16, Known: true}, {Name: "increment", Value: new(int64(-1)), Known: true}}
		}},
		{name: "narrow bounds require issuance evidence", fact: "sequence.current_value", change: func(stmt *spec.Statement, d *spec.PGSequenceChange, s *spec.PGSequenceChangeState) {
			stmt.DDL.Operation, s.Kind, s.Width = spec.DDLOperationAlterSequence, "S", 16
			d.Options = []spec.PGSequenceChangeOption{{Name: "maxvalue", Value: new(int64(20)), Known: true}}
		}},
		{name: "identity ownership", reason: "sequence_identity_ownership", change: func(stmt *spec.Statement, d *spec.PGSequenceChange, s *spec.PGSequenceChangeState) {
			stmt.DDL.Operation, s.Kind, s.Width = spec.DDLOperationAlterSequence, "S", 16
			d.Options = []spec.PGSequenceChangeOption{{Name: "owned_by", Argument: []string{"none"}, Known: true}}
			s.Ownership = &spec.PGSequenceOwnershipState{Sequence: d.Target.Table, IdentitySequence: true}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target := spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "s"}}
			decl := &spec.PGSequenceChange{Target: target}
			state := &spec.PGSequenceChangeState{Target: target, SchemaExists: true, Definition: &spec.SequenceDefinition{
				Start: new(int64(7)), MinValue: new(int64(1)), MaxValue: new(int64(99)), Increment: new(int64(2)), CacheValue: new(int64(3)),
			}}
			stmt := spec.Statement{Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
				DDL:      &spec.DDL{Operation: spec.DDLOperationCreateSequence, PGSequenceChange: decl},
				Metadata: &spec.Metadata{Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"}, PGSequenceChange: state}}
			tc.change(&stmt, decl, state)
			if reason, fact := assessPGSequenceChange(stmt); reason != tc.reason || fact != tc.fact {
				t.Fatalf("assessment = (%q, %q), want (%q, %q)", reason, fact, tc.reason, tc.fact)
			}
		})
	}
}
