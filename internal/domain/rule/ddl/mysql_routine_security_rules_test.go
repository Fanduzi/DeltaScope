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

func routineSecurityStatement(op spec.DDLOperation, kind spec.RoutineKind, modes ...string) spec.Statement {
	decl := &spec.RoutineDeclaration{Kind: kind, Name: spec.Table{Schema: "app", Name: "p"}}
	for _, mode := range modes {
		decl.Characteristics = append(decl.Characteristics, spec.RoutineCharacteristic{Key: "sql_security", Value: mode})
	}
	return spec.Statement{Dialect: spec.DialectMySQL, Kind: spec.KindDDL, DDL: &spec.DDL{Operation: op, Routine: decl}}
}

func TestMySQLRoutineSecurityDefinerSemantics(t *testing.T) {
	r, err := newMySQLRoutineSecurityDefinerRule(policy.RulePolicy{})
	if err != nil {
		t.Fatal(err)
	}
	for _, pair := range []struct {
		kind          spec.RoutineKind
		create, alter spec.DDLOperation
	}{
		{spec.RoutineKindProcedure, spec.DDLOperationCreateProcedure, spec.DDLOperationAlterProcedure},
		{spec.RoutineKindFunction, spec.DDLOperationCreateFunction, spec.DDLOperationAlterFunction},
	} {
		for _, op := range []spec.DDLOperation{pair.create, pair.alter} {
			for _, mode := range []string{"definer", "invoker", ""} {
				t.Run(string(op)+"/"+mode, func(t *testing.T) {
					stmt := routineSecurityStatement(op, pair.kind)
					if mode != "" {
						stmt.DDL.Routine.Characteristics = []spec.RoutineCharacteristic{{Key: "sql_security", Value: mode}}
					}
					findings, err := r.Evaluate(context.Background(), stmt)
					if err != nil {
						t.Fatal(err)
					}
					warn := mode == "definer" || (mode == "" && op == pair.create)
					if !warn {
						if len(findings) != 0 {
							t.Fatalf("unexpected findings: %+v", findings)
						}
						return
					}
					if len(findings) != 1 || findings[0].Level != rule.LevelWarning {
						t.Fatalf("expected one warning: %+v", findings)
					}
					source := "authored"
					if mode == "" {
						source = "create_default"
					}
					want := map[string]any{"routine": "p", "schema": "app", "kind": string(pair.kind), "operation": string(op), "security": "definer", "security_source": source}
					if !reflect.DeepEqual(findings[0].Metadata, want) {
						t.Fatalf("metadata=%+v, want %+v", findings[0].Metadata, want)
					}
				})
			}
		}
	}
}

func TestMySQLRoutineSecurityDefinerBoundaries(t *testing.T) {
	r := mysqlRoutineSecurityDefinerRule{level: rule.LevelWarning}
	base := func() spec.Statement {
		return routineSecurityStatement(spec.DDLOperationCreateProcedure, spec.RoutineKindProcedure)
	}
	for _, tc := range []struct {
		name   string
		change func(*spec.Statement)
	}{
		{"nil ddl", func(s *spec.Statement) { s.DDL = nil }},
		{"nil routine", func(s *spec.Statement) { s.DDL.Routine = nil }},
		{"non ddl", func(s *spec.Statement) { s.Kind = spec.KindDML }},
		{"tidb", func(s *spec.Statement) { s.Dialect = spec.DialectTiDB }},
		{"postgresql", func(s *spec.Statement) { s.Dialect = spec.DialectPostgreSQL }},
		{"drop", func(s *spec.Statement) { s.DDL.Operation = spec.DDLOperationDropProcedure }},
		{"trigger", func(s *spec.Statement) { s.DDL.Operation = spec.DDLOperationCreateTrigger }},
		{"unknown kind", func(s *spec.Statement) { s.DDL.Routine.Kind = "" }},
		{"soname", func(s *spec.Statement) { s.DDL.Routine.Characteristics = []spec.RoutineCharacteristic{{Key: "soname"}} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := base()
			tc.change(&s)
			f, err := r.Evaluate(context.Background(), s)
			if err != nil || len(f) != 0 || r.AppliesTo(s) || len(r.EvidenceGaps(s)) != 0 {
				t.Fatalf("inapplicable statement: findings=%+v err=%v", f, err)
			}
		})
	}
	// An account clause never overrides INVOKER; omission still uses the CREATE default.
	for _, mode := range []string{"invoker", ""} {
		s := base()
		s.DDL.Routine.Definer = "named"
		if mode != "" {
			s.DDL.Routine.Characteristics = []spec.RoutineCharacteristic{{Key: "sql_security", Value: mode}}
		}
		f, err := r.Evaluate(context.Background(), s)
		if err != nil || (len(f) == 0) != (mode == "invoker") {
			t.Fatalf("account clause changed mode semantics: %+v %v", f, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := r.Evaluate(ctx, base()); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
}

func TestMySQLRoutineSecurityUnownedCharacteristics(t *testing.T) {
	r := mysqlRoutineSecurityDefinerRule{level: rule.LevelWarning}
	for _, modes := range [][]string{{"invalid"}, {""}, {"definer", "invoker"}, {"invoker", "definer"}, {"invoker", "invoker"}} {
		stmt := routineSecurityStatement(spec.DDLOperationCreateProcedure, spec.RoutineKindProcedure, modes...)
		f, err := r.Evaluate(context.Background(), stmt)
		if err != nil || len(f) != 0 {
			t.Fatalf("unowned mode must not pick a mode: %+v %v", f, err)
		}
		gaps := r.EvidenceGaps(stmt)
		if len(gaps) != 1 || gaps[0].ReasonCode != "unproven_routine_security" || !reflect.DeepEqual(gaps[0].RequiredFacts, []string{"routine.sql_security"}) {
			t.Fatalf("unowned mode must remain explicit: %+v", gaps)
		}
	}
}

func TestMySQLRoutineSecurityRegistration(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		cfg := policy.Default()
		def := cfg.Rules[ruleIDRoutineSecurityDefinerWarn]
		if !def.Enabled || def.Level != rule.LevelWarning || len(def.Params) != 0 {
			t.Fatalf("unexpected default: %+v", def)
		}
		cfg.Rules = map[string]policy.RulePolicy{ruleIDRoutineSecurityDefinerWarn: {Enabled: enabled, Level: rule.LevelBlocker}}
		registry := rule.NewRegistry()
		if err := Register(registry, cfg); err != nil {
			t.Fatal(err)
		}
		f, err := registry.EvaluateStatement(context.Background(), routineSecurityStatement(spec.DDLOperationCreateFunction, spec.RoutineKindFunction))
		if err != nil {
			t.Fatal(err)
		}
		if !enabled {
			if len(f) != 0 {
				t.Fatalf("disabled findings: %+v", f)
			}
		} else if len(f) != 1 || f[0].RuleID != ruleIDRoutineSecurityDefinerWarn || f[0].Level != rule.LevelBlocker {
			t.Fatalf("configured finding: %+v", f)
		}
	}
}
