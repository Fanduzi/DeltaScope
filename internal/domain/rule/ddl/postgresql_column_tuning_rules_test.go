package ddl

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func pgTuningStatement(action string) spec.Statement {
	s := pgGenerationTransitionStatement(action, &spec.PGColumnState{})
	s.DDL.Alter[0].PGColumnTransition = nil
	s.DDL.Alter[0].PGColumnTuning = &spec.PGColumnTuning{}
	return s
}
func pgTuningNumber(name string, value float64) spec.PGColumnOption {
	return spec.PGColumnOption{Name: name, ValueKind: spec.PGColumnOptionNumeric, Numeric: &value}
}
func pgDropTestRule(t *testing.T) alterObjectExistenceRule {
	t.Helper()
	r, err := newAlterObjectExistenceRule(ruleIDAlterDropColumnExistsRequire, []string{"drop_column"}, "column", false, rule.LevelBlocker, policy.RulePolicy{}, alterObjectName, snapshotHasColumn)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func TestPGDropColumnGuardNoop(t *testing.T) {
	s := pgGenerationTransitionStatement("drop_column", nil)
	s.DDL.Alter[0].Options = map[string]string{"if_exists": "true", "cascade": "false"}
	s.Metadata.TargetTable.Columns = []spec.Column{}
	advisory, err := newDropColumnAdvisoryRule(policy.RulePolicy{})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range []rule.StatementRule{pgDropTestRule(t), advisory} {
		f, err := r.Evaluate(context.Background(), s)
		if err != nil || len(f) != 0 {
			t.Errorf("%s: proven guarded absence is no-op, got %+v %v", r.ID(), f, err)
		}
	}
}

func TestPGStatisticsTuningArms(t *testing.T) {
	valid := pgColumnTuningRule{ruleIDPGColumnStatisticsTargetValid, rule.LevelBlocker}
	clamp := pgColumnTuningRule{ruleIDPGColumnStatisticsClampWarn, rule.LevelWarning}
	for _, tc := range []struct {
		value              int64
		invalid, warn, gap bool
	}{
		{-2, true, false, false}, {-1, false, false, false}, {0, false, false, false}, {10000, false, false, false}, {10001, false, true, false}, {math.MaxInt32, false, true, false}, {math.MinInt32, true, false, false}, {int64(math.MaxInt32) + 1, false, false, true},
	} {
		s := pgTuningStatement("set_statistics")
		s.DDL.Alter[0].PGColumnTuning.StatisticsValue = &tc.value
		for _, r := range []pgColumnTuningRule{valid, clamp} {
			f, err := r.Evaluate(context.Background(), s)
			want := r.id == valid.id && tc.invalid || r.id == clamp.id && tc.warn
			if err != nil || (len(f) == 1) != want || (len(r.EvidenceGaps(s)) != 0) != tc.gap {
				t.Fatalf("value=%v id=%s f=%+v gap=%v err=%v", tc.value, r.id, f, r.EvidenceGaps(s), err)
			}
		}
	}
	s := pgTuningStatement("set_statistics")
	s.DDL.Alter[0].PGColumnTuning.StatisticsDefault = true
	for _, r := range []pgColumnTuningRule{valid, clamp} {
		f, e := r.Evaluate(context.Background(), s)
		if e != nil || len(f) != 0 || len(r.EvidenceGaps(s)) != 0 {
			t.Fatal("DEFAULT not silent", f, e)
		}
	}
	for _, f := range []*spec.PGColumnTuning{nil, {}, {StatisticsDefault: true, StatisticsValue: pgIdentityTestInt(0)}, {Unmodeled: true}, {StatisticsValue: pgIdentityTestInt(0), Options: []spec.PGColumnOption{{Name: "extra"}}}} {
		s.DDL.Alter[0].PGColumnTuning = f
		if got, err := valid.Evaluate(context.Background(), s); err != nil || len(got) != 0 || len(valid.EvidenceGaps(s)) != 1 {
			t.Fatalf("malformed=%+v: %v %v", f, got, err)
		}
	}
	s = pgTuningStatement("set_statistics")
	s.DDL.Alter[0].PGColumnTuning.StatisticsValue = pgIdentityTestInt(10001)
	s.Metadata.TargetTable.PGColumns.Columns = []spec.PGColumnState{}
	if f, _ := valid.Evaluate(context.Background(), s); len(f) != 1 {
		t.Fatal("missing target not rejected")
	}
	if f, _ := clamp.Evaluate(context.Background(), s); len(f) != 0 || len(clamp.EvidenceGaps(s)) != 0 {
		t.Fatal("risk on proved-invalid target")
	}
}

func TestPGColumnOptionsTuningArms(t *testing.T) {
	r := pgColumnTuningRule{ruleIDPGColumnOptionsValid, rule.LevelBlocker}
	for _, tc := range []struct {
		name, action string
		options      []spec.PGColumnOption
		invalid, gap bool
	}{
		{"minimum", "set_column_options", []spec.PGColumnOption{pgTuningNumber("n_distinct", -1)}, false, false},
		{"fraction", "set_column_options", []spec.PGColumnOption{pgTuningNumber("n_distinct", -0.5)}, false, false},
		{"zero", "set_column_options", []spec.PGColumnOption{pgTuningNumber("n_distinct", 0)}, false, false},
		{"positive-fraction", "set_column_options", []spec.PGColumnOption{pgTuningNumber("n_distinct", 0.5)}, false, false},
		{"maximum", "set_column_options", []spec.PGColumnOption{pgTuningNumber("n_distinct", math.MaxFloat64)}, false, false},
		{"inherited", "set_column_options", []spec.PGColumnOption{pgTuningNumber("n_distinct_inherited", -0.5)}, false, false},
		{"low", "set_column_options", []spec.PGColumnOption{pgTuningNumber("n_distinct", -1.1)}, true, false},
		{"inherited-low", "set_column_options", []spec.PGColumnOption{pgTuningNumber("n_distinct_inherited", -2)}, true, false},
		{"duplicate", "set_column_options", []spec.PGColumnOption{pgTuningNumber("n_distinct", 1), pgTuningNumber("n_distinct", 2)}, true, false},
		{"unknown", "set_column_options", []spec.PGColumnOption{pgTuningNumber("private_unknown_name", 1)}, false, true},
		{"unknown-sibling-known-bad", "set_column_options", []spec.PGColumnOption{pgTuningNumber("private_unknown_name", 1), pgTuningNumber("n_distinct", -2)}, true, false},
		{"quoted", "set_column_options", []spec.PGColumnOption{{Name: "n_distinct", ValueKind: spec.PGColumnOptionQuoted}}, false, true},
		{"missing-value", "set_column_options", []spec.PGColumnOption{{Name: "n_distinct", ValueKind: spec.PGColumnOptionAbsent}}, false, true},
		{"numeric-without-value", "set_column_options", []spec.PGColumnOption{{Name: "n_distinct", ValueKind: spec.PGColumnOptionNumeric}}, false, true},
		{"nan", "set_column_options", []spec.PGColumnOption{pgTuningNumber("n_distinct", math.NaN())}, false, true},
		{"infinity", "set_column_options", []spec.PGColumnOption{pgTuningNumber("n_distinct", math.Inf(-1))}, false, true},
		{"namespace", "set_column_options", []spec.PGColumnOption{{Name: "n_distinct", Namespace: "private_namespace", ValueKind: spec.PGColumnOptionNumeric, Numeric: pgTuningNumber("", 1).Numeric}}, false, true},
		{"reset-unknown", "reset_column_options", []spec.PGColumnOption{{Name: "private_unknown_name", ValueKind: spec.PGColumnOptionAbsent}}, false, false},
		{"reset-repeat", "reset_column_options", []spec.PGColumnOption{{Name: "n_distinct", ValueKind: spec.PGColumnOptionAbsent}, {Name: "n_distinct", ValueKind: spec.PGColumnOptionAbsent}}, false, false},
		{"reset-malformed-value", "reset_column_options", []spec.PGColumnOption{pgTuningNumber("n_distinct", -2)}, false, true},
		{"empty", "set_column_options", nil, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := pgTuningStatement(tc.action)
			s.DDL.Alter[0].PGColumnTuning.Options = tc.options
			f, err := r.Evaluate(context.Background(), s)
			if err != nil || (len(f) == 1) != tc.invalid || (len(r.EvidenceGaps(s)) != 0) != tc.gap {
				t.Fatalf("f=%+v gaps=%v err=%v", f, r.EvidenceGaps(s), err)
			}
			raw, _ := json.Marshal(f)
			for _, secret := range []string{"private_unknown_name", "private_namespace", "private_expression_marker", "-1.1"} {
				if strings.Contains(string(raw), secret) {
					t.Fatalf("payload leaked: %s", raw)
				}
			}
		})
	}
	for _, action := range []string{"set_column_options", "reset_column_options"} {
		s := pgTuningStatement(action)
		s.DDL.Alter[0].PGColumnTuning.Options = []spec.PGColumnOption{{Name: "unknown", ValueKind: spec.PGColumnOptionAbsent}}
		s.Metadata.TargetTable.PGColumns.Columns = []spec.PGColumnState{}
		if f, _ := r.Evaluate(context.Background(), s); len(f) != 1 {
			t.Fatal("missing target not rejected", action)
		}
	}
}

func TestPGColumnTuningProofAndScope(t *testing.T) {
	for _, id := range []string{ruleIDPGColumnStatisticsTargetValid, ruleIDPGColumnStatisticsClampWarn, ruleIDPGColumnOptionsValid} {
		r := pgColumnTuningRule{id, rule.LevelBlocker}
		makeStatement := func() spec.Statement {
			s := pgTuningStatement("set_statistics")
			s.DDL.Alter[0].PGColumnTuning.StatisticsValue = pgIdentityTestInt(10001)
			if id == ruleIDPGColumnOptionsValid {
				s.DDL.Alter[0].Action = "set_column_options"
				s.DDL.Alter[0].PGColumnTuning = &spec.PGColumnTuning{Options: []spec.PGColumnOption{pgTuningNumber("n_distinct", 0.5)}}
			}
			return s
		}
		for _, mutate := range []func(*spec.Statement){
			func(s *spec.Statement) { s.Metadata = nil }, func(s *spec.Statement) { s.Metadata.Instance = nil }, func(s *spec.Statement) { s.Metadata.Instance.Version = "PostgreSQL 18.1" }, func(s *spec.Statement) { s.Metadata.Instance.Version = "17.7" },
			func(s *spec.Statement) { s.Metadata.TargetTable.PGColumns = nil }, func(s *spec.Statement) { s.Metadata.TargetTable.PGColumns.Ordinary = false }, func(s *spec.Statement) { s.Metadata.TargetTable.PGColumns.Columns = nil },
			func(s *spec.Statement) { s.Metadata.Schema = "other" }, func(s *spec.Statement) { s.Metadata.TargetTable.Schema = "other" }, func(s *spec.Statement) { s.Metadata.TargetTable.Table.Name = "T" },
			func(s *spec.Statement) {
				s.Metadata.TargetTable.PGColumns.Columns = append(s.Metadata.TargetTable.PGColumns.Columns, s.Metadata.TargetTable.PGColumns.Columns[0])
			},
		} {
			s := makeStatement()
			mutate(&s)
			f, err := r.Evaluate(context.Background(), s)
			if err != nil || len(f) != 0 || len(r.EvidenceGaps(s)) != 1 {
				t.Fatalf("id=%s f=%v gaps=%v err=%v", id, f, r.EvidenceGaps(s), err)
			}
		}
		s := makeStatement()
		s.DDL.Alter = append(s.DDL.Alter, s.DDL.Alter[0])
		if len(r.EvidenceGaps(s)) != 1 {
			t.Fatal("prior action reused old catalog")
		}
		for _, d := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
			s.Dialect = d
			if r.AppliesTo(s) || len(r.EvidenceGaps(s)) != 0 {
				t.Fatal("cross dialect match")
			}
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := r.Evaluate(ctx, makeStatement()); !errors.Is(err, context.Canceled) {
			t.Fatal("lost cancellation", err)
		}
	}
	for _, action := range []string{"set_statistics", "set_column_options"} {
		s := pgTuningStatement(action)
		r := pgColumnTuningRule{ruleIDPGColumnStatisticsTargetValid, rule.LevelBlocker}
		s.DDL.Alter[0].PGColumnTuning.StatisticsValue = pgIdentityTestInt(-2)
		if action == "set_column_options" {
			r.id = ruleIDPGColumnOptionsValid
			s.DDL.Alter[0].PGColumnTuning = &spec.PGColumnTuning{Options: []spec.PGColumnOption{pgTuningNumber("n_distinct", -2)}}
		}
		s.Metadata.TargetTable.PGColumns = nil
		if f, _ := r.Evaluate(context.Background(), s); len(f) != 1 || len(r.EvidenceGaps(s)) != 0 {
			t.Fatal("intrinsic refusal depended on catalog", action)
		}
		s.Metadata.Instance.Version = ""
		if f, _ := r.Evaluate(context.Background(), s); len(f) != 0 || len(r.EvidenceGaps(s)) != 1 {
			t.Fatal("unobserved version guessed", action)
		}
	}
}

func TestPGDropColumnTargetProof(t *testing.T) {
	r := pgDropTestRule(t)
	for _, tc := range []struct {
		name, guard         string
		exists, block, noop bool
	}{
		{"absent-guard", "true", false, false, true}, {"absent-plain", "false", false, true, false}, {"present-guard", "true", true, false, false}, {"present-plain", "false", true, false, false},
	} {
		s := pgGenerationTransitionStatement("drop_column", nil)
		if tc.exists {
			s.Metadata.TargetTable.PGColumns.Columns = []spec.PGColumnState{{Name: "id"}}
		}
		s.DDL.Alter[0].Options = map[string]string{"if_exists": tc.guard}
		// The private exact catalog wins over both a missing legacy shape and
		// a case-folded legacy column that is not the authored identifier.
		s.Metadata.TargetTable.Columns = []spec.Column{{Name: "ID"}}
		f, err := r.Evaluate(context.Background(), s)
		noop, _, missing := pgDropColumnTarget(s, 0)
		if err != nil || (len(f) == 1) != tc.block || noop != tc.noop || missing != "" || len(r.EvidenceGaps(s)) != 0 {
			t.Fatalf("%s f=%v noop=%v missing=%s err=%v", tc.name, f, noop, missing, err)
		}
	}
	for _, mutate := range []func(*spec.Statement){
		func(s *spec.Statement) { s.Metadata = nil }, func(s *spec.Statement) { s.Metadata.Instance = nil }, func(s *spec.Statement) { s.Metadata.TargetTable.PGColumns = nil }, func(s *spec.Statement) { s.Metadata.TargetTable.PGColumns.Columns = nil },
		func(s *spec.Statement) { s.Metadata.TargetTable.PGColumns.Ordinary = false }, func(s *spec.Statement) { s.Metadata.Schema = "other" }, func(s *spec.Statement) { s.Metadata.TargetTable.Table.Name = "T" },
		func(s *spec.Statement) { s.DDL.Alter[0].Options = nil }, func(s *spec.Statement) { s.DDL.Alter[0].Options["if_exists"] = "unknown" },
		func(s *spec.Statement) {
			s.Metadata.TargetTable.PGColumns.Columns = []spec.PGColumnState{{Name: "id"}, {Name: "id"}}
		},
	} {
		s := pgGenerationTransitionStatement("drop_column", nil)
		s.DDL.Alter[0].Options = map[string]string{"if_exists": "true"}
		mutate(&s)
		f, err := r.Evaluate(context.Background(), s)
		if err != nil || len(f) != 0 || len(r.EvidenceGaps(s)) != 1 {
			t.Fatalf("unknown f=%v gap=%v err=%v", f, r.EvidenceGaps(s), err)
		}
		if noop, _, _ := pgDropColumnTarget(s, 0); noop {
			t.Fatal("unknown proof became no-op")
		}
	}
	s := pgGenerationTransitionStatement("drop_column", nil)
	s.DDL.Alter[0].Options = map[string]string{"if_exists": "true"}
	s.DDL.Alter = append(s.DDL.Alter, s.DDL.Alter[0])
	if len(r.EvidenceGaps(s)) != 1 {
		t.Fatal("later DROP guessed prior action outcome")
	}
	registry := rule.NewRegistry()
	cfg := policy.Default()
	for id, p := range cfg.Rules {
		p.Enabled = id == ruleIDAlterDropColumnForbid
		cfg.Rules[id] = p
	}
	p := cfg.Rules[ruleIDAlterDropColumnForbid]
	p.Params = map[string]any{"forbid": true}
	cfg.Rules[ruleIDAlterDropColumnForbid] = p
	if err := Register(registry, cfg); err != nil {
		t.Fatal(err)
	}
	f, err := registry.EvaluateStatement(context.Background(), s)
	if err != nil || len(f) == 0 {
		t.Fatal("independent forbid was suppressed", f, err)
	}
}
