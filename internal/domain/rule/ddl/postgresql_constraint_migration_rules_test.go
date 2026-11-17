package ddl

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func constraintMigrationStatement(kind, option, value string) spec.Statement {
	return spec.Statement{Dialect: spec.DialectPostgreSQL, Kind: spec.KindDDL, DDL: &spec.DDL{
		Operation: spec.DDLOperationAlterTable, Table: &spec.Table{Schema: "app", Name: "t"},
		Alter: []spec.Alter{{Action: "add_constraint", Name: "c", Options: map[string]string{"constraint_type": kind, option: value}}},
	}}
}

func TestPGConstraintMigrationArms(t *testing.T) {
	for _, pk := range []bool{false, true} {
		r := pgConstraintMigrationRule{primaryKey: pk, level: rule.LevelWarning}
		kind, option, fact := r.declaration()
		for _, value := range []string{"false", "true", "", "invalid"} {
			t.Run(kind+"/"+value, func(t *testing.T) {
				s := constraintMigrationStatement(kind, option, value)
				f, err := r.Evaluate(context.Background(), s)
				if err != nil {
					t.Fatal(err)
				}
				if value == "false" {
					want := map[string]any{"schema": "app", "table": "t", "constraint": "c", "constraint_type": kind, "action": "add_constraint", option: false}
					if len(f) != 1 || f[0].Level != rule.LevelWarning || !reflect.DeepEqual(f[0].Metadata, want) {
						t.Fatalf("finding=%+v", f)
					}
				} else if len(f) != 0 {
					t.Fatalf("unexpected finding: %+v", f)
				}
				gaps := r.EvidenceGaps(s)
				if value == "" || value == "invalid" {
					if len(gaps) != 1 || gaps[0].ReasonCode != "unproven_constraint_declaration" || !reflect.DeepEqual(gaps[0].RequiredFacts, []string{fact}) {
						t.Fatalf("gaps=%+v", gaps)
					}
				} else if len(gaps) != 0 {
					t.Fatalf("unexpected gap: %+v", gaps)
				}
			})
		}
		for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
			s := constraintMigrationStatement(kind, option, "false")
			s.Dialect = dialect
			if r.AppliesTo(s) {
				t.Fatalf("cross dialect %s", dialect)
			}
		}
		for _, change := range []func(*spec.Statement){
			func(s *spec.Statement) { s.DDL = nil },
			func(s *spec.Statement) { s.DDL.Table = nil },
			func(s *spec.Statement) { s.Kind = spec.KindDML },
			func(s *spec.Statement) { s.DDL.Operation = spec.DDLOperationCreateTable },
			func(s *spec.Statement) { s.DDL.Alter[0].Action = "validate_constraint" },
			func(s *spec.Statement) { s.DDL.Alter[0].Options["constraint_type"] = "check" },
		} {
			s := constraintMigrationStatement(kind, option, "false")
			change(&s)
			f, err := r.Evaluate(context.Background(), s)
			if r.AppliesTo(s) || err != nil || len(f) != 0 || len(r.EvidenceGaps(s)) != 0 {
				t.Fatalf("inapplicable: %+v %v", f, err)
			}
		}
		s := constraintMigrationStatement(kind, option, "false")
		s.DDL.Alter = append(s.DDL.Alter, spec.Alter{Action: "add_constraint", Options: map[string]string{"constraint_type": kind}})
		f, err := r.Evaluate(context.Background(), s)
		if err != nil || len(f) != 1 || len(r.EvidenceGaps(s)) != 1 {
			t.Fatalf("independent action finding/gap=%+v %v", f, err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := r.Evaluate(ctx, s); !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	}
}

func TestPGConstraintMigrationDefaults(t *testing.T) {
	for _, pk := range []bool{false, true} {
		r := pgConstraintMigrationRule{primaryKey: pk}
		kind, option, _ := r.declaration()
		p := policy.Default().Rules[r.ID()]
		if !p.Enabled || p.Level != rule.LevelWarning || len(p.Params) != 0 {
			t.Fatalf("default=%+v", p)
		}
		for _, enabled := range []bool{true, false} {
			reg := rule.NewRegistry()
			if err := Register(reg, policy.Policy{Rules: map[string]policy.RulePolicy{r.ID(): {Enabled: enabled, Level: rule.LevelBlocker}}}); err != nil {
				t.Fatal(err)
			}
			f, err := reg.EvaluateStatement(context.Background(), constraintMigrationStatement(kind, option, "false"))
			if err != nil {
				t.Fatal(err)
			}
			if enabled {
				if len(f) != 1 || f[0].RuleID != r.ID() || f[0].Level != rule.LevelBlocker {
					t.Fatalf("configured=%+v", f)
				}
			} else if len(f) != 0 {
				t.Fatalf("disabled=%+v", f)
			}
		}
	}
}

func TestPGUniqueAdvisoryIndexReuseControl(t *testing.T) {
	r, err := newAddUniqueConstraintAdvisoryRule(policy.RulePolicy{})
	if err != nil {
		t.Fatal(err)
	}
	for _, reuse := range []string{"true", "false"} {
		f, err := r.Evaluate(context.Background(), constraintMigrationStatement("unique", "using_index", reuse))
		if err != nil || (len(f) == 0) != (reuse == "true") {
			t.Fatalf("reuse=%s findings=%+v err=%v", reuse, f, err)
		}
	}
}
