//go:build postgresql

package audit

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule/ddl"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGConstraintMigrationPipeline(t *testing.T) {
	reg := rule.NewRegistry()
	if err := ddl.Register(reg, policy.Policy{Rules: map[string]policy.RulePolicy{
		"ddl.pg.alter.add_foreign_key.not_valid.require":               {Enabled: true},
		"ddl.pg.alter.add_primary_key.concurrent_index.warn":           {Enabled: true},
		"ddl.pg.alter.add_unique_constraint.concurrent_index.advisory": {Enabled: true},
		"ddl.pg.alter.not_valid_constraint.validate.require":           {Enabled: true},
	}}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		sql, verdict              string
		warnings, notices, global int
	}{
		{"ALTER TABLE t ADD PRIMARY KEY(id)", "review", 1, 0, 0},
		{"ALTER TABLE app.t ADD CONSTRAINT pk PRIMARY KEY USING INDEX idx", "pass", 0, 0, 0},
		{"ALTER TABLE t ADD CONSTRAINT uq UNIQUE(id)", "pass", 0, 1, 0},
		{"ALTER TABLE t ADD CONSTRAINT uq UNIQUE USING INDEX idx", "pass", 0, 0, 0},
		{"ALTER TABLE t ADD CONSTRAINT fk FOREIGN KEY(id) REFERENCES p(id)", "review", 1, 0, 0},
		{"ALTER TABLE t ADD CONSTRAINT fk FOREIGN KEY(id) REFERENCES p(id) NOT VALID", "review", 0, 0, 1},
		{"ALTER TABLE t ADD CONSTRAINT fk FOREIGN KEY(id) REFERENCES p(id) NOT VALID;ALTER TABLE t VALIDATE CONSTRAINT fk", "pass", 0, 0, 0},
		{"ALTER TABLE t ADD PRIMARY KEY(id), ADD CONSTRAINT fk FOREIGN KEY(pid) REFERENCES p(id)", "review", 2, 0, 0},
		{"ALTER TABLE t ADD CHECK(id>0)", "pass", 0, 0, 0},
	} {
		t.Run(tc.sql, func(t *testing.T) {
			r, err := EvaluateStatements(context.Background(), reg, t27Statements(t, spec.DialectPostgreSQL, tc.sql))
			if err != nil || string(r.Verdict) != tc.verdict || len(r.GlobalFindings) != tc.global {
				t.Fatalf("result=%+v err=%v", r, err)
			}
			warnings, notices := 0, 0
			for _, s := range r.Statements {
				if len(s.EvidenceGaps) != 0 {
					t.Fatalf("unexpected gap: %+v", s)
				}
				for _, f := range s.Findings {
					if f.Level == rule.LevelWarning {
						warnings++
					}
					if f.Level == rule.LevelNotice {
						notices++
					}
					if f.Location == nil {
						t.Fatalf("no location: %+v", f)
					}
				}
			}
			if warnings != tc.warnings || notices != tc.notices {
				t.Fatalf("statement findings warnings=%d notices=%d", warnings, notices)
			}
		})
	}
}
