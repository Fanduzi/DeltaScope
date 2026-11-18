package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func pgSystemTargetStatement(action string) spec.Statement {
	s := pgTuningStatement(action)
	s.DDL.Alter[0].Name = "ctid"
	s.DDL.Alter[0].PGColumnTransition = &spec.PGColumnTransition{IfExists: true}
	s.DDL.Alter[0].PGColumnTuning = &spec.PGColumnTuning{StatisticsDefault: true}
	if action == "set_column_options" || action == "reset_column_options" {
		s.DDL.Alter[0].PGColumnTuning = &spec.PGColumnTuning{Options: []spec.PGColumnOption{{Name: "n_distinct", ValueKind: spec.PGColumnOptionAbsent}}}
		if action == "set_column_options" {
			s.DDL.Alter[0].PGColumnTuning.Options = []spec.PGColumnOption{pgTuningNumber("n_distinct", 0.5)}
		}
	}
	s.DDL.Alter[0].Options = map[string]string{"if_exists": "true", "cascade": "false"}
	s.Metadata.TargetTable.PGColumns.Columns = []spec.PGColumnState{}
	s.Metadata.TargetTable.PGColumns.SystemColumns = []spec.PGColumnState{{Name: "ctid", System: true, TypeStorage: "p"}}
	return s
}

func TestPGSystemTargetNeverAbsentOrGuardedNoop(t *testing.T) {
	for _, action := range []string{"drop_column", "set_statistics", "set_column_options", "reset_column_options", "drop_identity", "drop_expression", "set_default", "drop_default", "set_not_null", "drop_not_null", "add_identity", "set_generated"} {
		t.Run(action, func(t *testing.T) {
			s := pgSystemTargetStatement(action)
			var r rule.StatementRule = pgColumnGenerationRule{ruleIDPGColumnGenerationTransitionValid, rule.LevelBlocker}
			switch action {
			case "drop_column":
				r = pgDropTestRule(t)
			case "set_statistics":
				r = pgColumnTuningRule{ruleIDPGColumnStatisticsTargetValid, rule.LevelBlocker}
			case "set_column_options", "reset_column_options":
				r = pgColumnTuningRule{ruleIDPGColumnOptionsValid, rule.LevelBlocker}
			}
			f, err := r.Evaluate(context.Background(), s)
			if err != nil || len(f) != 1 || f[0].Metadata["reason_code"] != "system_column_inapplicable" {
				t.Fatalf("expected exact system rejection: %+v %v", f, err)
			}
			if action == "drop_column" {
				if f[0].Metadata["exists"] != true {
					t.Fatal("system column falsely absent")
				}
				advisory, err := newDropColumnAdvisoryRule(policy.RulePolicy{})
				if err != nil {
					t.Fatal(err)
				}
				if findings, err := advisory.Evaluate(context.Background(), s); err != nil || len(findings) != 0 {
					t.Fatal("advice implies executable system deletion", findings, err)
				}
				if noop, _, _ := pgDropColumnTarget(s, 0); noop {
					t.Fatal("existing system attribute became a no-op")
				}
			}
			s.Metadata.TargetTable.PGColumns.SystemColumns = nil
			f, err = r.Evaluate(context.Background(), s)
			gaps := r.(rule.EvidenceReporter).EvidenceGaps(s)
			if err != nil || len(f) != 0 || len(gaps) != 1 {
				t.Fatalf("unobserved system synthesized/absent: %+v %+v %v", f, gaps, err)
			}
		})
	}
	s := pgSystemTargetStatement("set_statistics")
	s.DDL.Alter[0].PGColumnTuning = &spec.PGColumnTuning{StatisticsValue: pgIdentityTestInt(10001)}
	clamp := pgColumnTuningRule{ruleIDPGColumnStatisticsClampWarn, rule.LevelWarning}
	if f, err := clamp.Evaluate(context.Background(), s); err != nil || len(f) != 0 || len(clamp.EvidenceGaps(s)) != 0 {
		t.Fatal("clamp risk on inapplicable system target", f, err)
	}
}
