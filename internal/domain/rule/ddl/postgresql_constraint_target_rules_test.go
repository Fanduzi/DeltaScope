package ddl

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func pgTargetStatement(action string, members []spec.PGConstraintState) spec.Statement {
	return spec.Statement{Dialect: spec.DialectPostgreSQL, Kind: spec.KindDDL,
		DDL:      &spec.DDL{Operation: spec.DDLOperationAlterTable, Table: &spec.Table{Schema: "app", Name: "t"}, Alter: []spec.Alter{{Action: action, Name: "c", Options: map[string]string{"if_exists": "false", "expression": "private_expression_marker"}}}},
		Metadata: &spec.Metadata{Schema: "app", Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"}, TargetTable: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGConstraints: members, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}}}}
}

func TestPGConstraintTargetArms(t *testing.T) {
	for _, tc := range []struct{ id, action string }{
		{ruleIDPGAlterValidateConstraintTargetRequire, "validate_constraint"},
		{ruleIDPGAlterConstraintDeferrabilityTargetRequire, "alter_constraint_deferrable"},
		{ruleIDPGAlterConstraintDeferrabilityTargetRequire, "alter_constraint_initially_deferred"},
		{ruleIDPGAlterDropConstraintTargetRequire, "drop_constraint"},
	} {
		t.Run(tc.action, func(t *testing.T) {
			r := pgConstraintTargetRule{id: tc.id, level: rule.LevelBlocker}
			for _, kind := range []string{"", "c", "f", "p", "u", "x"} {
				members := []spec.PGConstraintState{}
				if kind != "" {
					members = append(members, spec.PGConstraintState{Name: "c", Type: kind, InheritanceKnown: kind == "c"})
				}
				s := pgTargetStatement(tc.action, members)
				f, err := r.Evaluate(context.Background(), s)
				wantFailure := kind == "" || tc.id == ruleIDPGAlterValidateConstraintTargetRequire && kind != "c" && kind != "f" || tc.id == ruleIDPGAlterConstraintDeferrabilityTargetRequire && kind != "f"
				if err != nil || len(r.EvidenceGaps(s)) != 0 || (len(f) == 1) != wantFailure {
					t.Fatalf("kind=%q f=%+v gaps=%v err=%v", kind, f, r.EvidenceGaps(s), err)
				}
				if len(f) == 1 {
					if f[0].Level != rule.LevelBlocker || f[0].Metadata["schema"] != "app" || f[0].Metadata["constraint"] != "c" || f[0].Metadata["target_exists"] != (kind != "") {
						t.Fatalf("finding=%+v", f[0])
					}
					raw, err := json.Marshal(f[0])
					if err != nil {
						t.Fatal(err)
					}
					if strings.Contains(string(raw), "private_expression_marker") {
						t.Fatal("payload leak")
					}
				}
			}
			for _, mutate := range []func(*spec.Statement){
				func(s *spec.Statement) { s.Metadata = nil },
				func(s *spec.Statement) { s.Metadata.TargetTable = nil },
				func(s *spec.Statement) { s.Metadata.TargetTable.PGConstraints = nil },
				func(s *spec.Statement) {
					s.Metadata.TargetTable.PGConstraints = []spec.PGConstraintState{{Name: "c", Type: "f"}, {Name: "c", Type: "f"}}
				},
				func(s *spec.Statement) {
					s.Metadata.TargetTable.PGConstraints = []spec.PGConstraintState{{Name: "c", Type: "unknown"}}
				},
				func(s *spec.Statement) { s.Metadata.TargetTable.Schema = "other" },
				func(s *spec.Statement) { s.Metadata.TargetTable.Table.Name = "T" },
				func(s *spec.Statement) { s.Metadata.Schema = "other" },
				func(s *spec.Statement) { s.DDL.Alter[0].Name = "" },
			} {
				s := pgTargetStatement(tc.action, []spec.PGConstraintState{})
				mutate(&s)
				f, err := r.Evaluate(context.Background(), s)
				g := r.EvidenceGaps(s)
				if err != nil || len(f) != 0 || len(g) != 1 || g[0].ReasonCode != "unproven_constraint_target" || !reflect.DeepEqual(g[0].RequiredFacts, []string{"target_table.pg_constraints"}) {
					t.Fatalf("f=%+v gaps=%+v err=%v", f, g, err)
				}
			}
			s := pgTargetStatement(tc.action, []spec.PGConstraintState{})
			s.DDL.Alter = append(s.DDL.Alter, s.DDL.Alter[0])
			f, err := r.Evaluate(context.Background(), s)
			if err != nil || len(f) != 1 || len(r.EvidenceGaps(s)) != 1 {
				t.Fatalf("lost known failure or stale sibling gap: %+v %v", f, err)
			}
			for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
				s.Dialect = dialect
				if r.AppliesTo(s) || len(r.EvidenceGaps(s)) != 0 {
					t.Fatal("cross dialect match")
				}
			}
			s.Dialect = spec.DialectPostgreSQL
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if _, err := r.Evaluate(ctx, s); !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
			for _, mutate := range []func(*spec.Statement){
				func(s *spec.Statement) { s.DDL = nil },
				func(s *spec.Statement) { s.DDL.Table = nil },
				func(s *spec.Statement) { s.DDL.Operation = spec.DDLOperationCreateTable },
				func(s *spec.Statement) { s.DDL.Alter[0].Action = "add_constraint" },
				func(s *spec.Statement) { s.Unsupported = &spec.UnsupportedDetail{Feature: "unknown"} },
			} {
				s := pgTargetStatement(tc.action, []spec.PGConstraintState{})
				mutate(&s)
				f, err := r.Evaluate(context.Background(), s)
				if err != nil || r.AppliesTo(s) || len(f) != 0 || len(r.EvidenceGaps(s)) != 0 {
					t.Fatalf("inapplicable=%+v %v", f, err)
				}
			}
		})
	}
}

func TestPGConstraintTargetGuardAndDefaults(t *testing.T) {
	for _, id := range []string{ruleIDPGAlterValidateConstraintTargetRequire, ruleIDPGAlterConstraintDeferrabilityTargetRequire, ruleIDPGAlterDropConstraintTargetRequire} {
		p := policy.Default().Rules[id]
		if !p.Enabled || p.Level != rule.LevelBlocker || len(p.Params) != 0 {
			t.Fatalf("default %s: %+v", id, p)
		}
		for _, enabled := range []bool{true, false} {
			reg := rule.NewRegistry()
			if err := Register(reg, policy.Policy{Rules: map[string]policy.RulePolicy{id: {Enabled: enabled, Level: rule.LevelWarning}}}); err != nil {
				t.Fatal(err)
			}
			action := "drop_constraint"
			if id == ruleIDPGAlterValidateConstraintTargetRequire {
				action = "validate_constraint"
			}
			if id == ruleIDPGAlterConstraintDeferrabilityTargetRequire {
				action = "alter_constraint_deferrable"
			}
			f, err := reg.EvaluateStatement(context.Background(), pgTargetStatement(action, []spec.PGConstraintState{}))
			if err != nil || (len(f) == 1) != enabled || enabled && (f[0].RuleID != id || f[0].Level != rule.LevelWarning) {
				t.Fatalf("configured=%+v %v", f, err)
			}
		}
	}
	r := pgConstraintTargetRule{id: ruleIDPGAlterDropConstraintTargetRequire, level: rule.LevelBlocker}
	for _, guard := range []string{"true", "false", "", "unknown"} {
		s := pgTargetStatement("drop_constraint", []spec.PGConstraintState{})
		s.DDL.Alter[0].Options["if_exists"] = guard
		f, err := r.Evaluate(context.Background(), s)
		g := r.EvidenceGaps(s)
		if err != nil || (len(f) == 1) != (guard == "false") || (len(g) == 1) != (guard == "" || guard == "unknown") {
			t.Fatalf("guard=%q f=%v gaps=%v err=%v", guard, f, g, err)
		}
		if len(g) == 1 && !reflect.DeepEqual(g[0].RequiredFacts, []string{"constraint.if_exists"}) {
			t.Fatalf("gap=%v", g)
		}
	}
}

func TestDropConstraintPrimaryKeyIdentityDialect(t *testing.T) {
	for _, tc := range []struct {
		dialect spec.Dialect
		name    string
		count   int
	}{
		{spec.DialectPostgreSQL, "ck", 1}, {spec.DialectPostgreSQL, "CK", 0},
		{spec.DialectMySQL, "CK", 1}, {spec.DialectTiDB, "CK", 1},
	} {
		s := pgTargetStatement("drop_constraint", []spec.PGConstraintState{})
		s.Dialect = tc.dialect
		s.DDL.Alter[0].Name = tc.name
		s.Metadata.TargetTable.Constraints = []spec.Constraint{{Type: "primary_key", Name: "ck"}}
		if got := len(matchingDropPrimaryKeyActions(s)); got != tc.count {
			t.Fatalf("dialect=%s name=%q got=%d want=%d", tc.dialect, tc.name, got, tc.count)
		}
	}
}

func TestPGConstraintTargetVersionScope(t *testing.T) {
	r := pgConstraintTargetRule{id: ruleIDPGAlterValidateConstraintTargetRequire, level: rule.LevelBlocker}
	for _, banner := range []string{"", "17.7", "PostgreSQL 18.1", "PostgreSQL 170.1", "PostgreSQL 17.beta", "PostgreSQL 17.7beta", "PostgreSQL 17.7.1"} {
		s := pgTargetStatement("validate_constraint", []spec.PGConstraintState{})
		s.Metadata.Instance.Version = banner
		f, err := r.Evaluate(context.Background(), s)
		g := r.EvidenceGaps(s)
		if err != nil || len(f) != 0 || len(g) != 1 || !reflect.DeepEqual(g[0].RequiredFacts, []string{"instance.version"}) {
			t.Fatalf("banner=%q f=%v gaps=%v err=%v", banner, f, g, err)
		}
	}
	for _, banner := range []string{"PostgreSQL 17.7", "PostgreSQL 17.10 (Debian build)"} {
		s := pgTargetStatement("validate_constraint", []spec.PGConstraintState{})
		s.Metadata.Instance.Version = banner
		f, err := r.Evaluate(context.Background(), s)
		if err != nil || len(f) != 1 || len(r.EvidenceGaps(s)) != 0 {
			t.Fatalf("banner=%q f=%v err=%v", banner, f, err)
		}
	}
}

func TestPGConstraintTargetAdvisoryNoops(t *testing.T) {
	validate, _ := newValidateConstraintAdvisoryRule(policy.RulePolicy{})
	drop, _ := newDropConstraintAdvisoryRule(policy.RulePolicy{})
	for _, kind := range []string{"c", "f", "p"} {
		s := pgTargetStatement("validate_constraint", []spec.PGConstraintState{{Name: "c", Type: kind, Validated: true}})
		f, err := validate.Evaluate(context.Background(), s)
		if err != nil || (len(f) == 0) != (kind != "p") {
			t.Fatalf("kind=%s f=%v err=%v", kind, f, err)
		}
		s.Metadata.TargetTable.PGConstraints = nil
		f, err = validate.Evaluate(context.Background(), s)
		if err != nil || len(f) != 1 || !strings.Contains(f[0].Message, "may scan") {
			t.Fatalf("unknown: %v %v", f, err)
		}
	}
	s := pgTargetStatement("drop_constraint", []spec.PGConstraintState{})
	s.DDL.Alter[0].Options["if_exists"] = "true"
	f, err := drop.Evaluate(context.Background(), s)
	if err != nil || len(f) != 0 {
		t.Fatalf("known noop=%v %v", f, err)
	}
	s.Metadata.TargetTable.PGConstraints = nil
	f, err = drop.Evaluate(context.Background(), s)
	if err != nil || len(f) != 1 {
		t.Fatalf("unknown must retain conditional notice=%v %v", f, err)
	}
}

func TestPGDropConstraintDirectReference(t *testing.T) {
	for _, tc := range []struct {
		name, kind, cascade string
		fact                *spec.PGDropConstraintReference
		blocked, gap        bool
	}{
		{name: "direct primary reference", kind: "p", cascade: "false", fact: &spec.PGDropConstraintReference{Schema: "app", Table: "t", Name: "c", Kind: "p", Blocked: true}, blocked: true},
		{name: "direct unique reference", kind: "u", cascade: "false", fact: &spec.PGDropConstraintReference{Schema: "app", Table: "t", Name: "c", Kind: "u", Blocked: true}, blocked: true},
		{name: "no dependent", kind: "p", cascade: "false", fact: &spec.PGDropConstraintReference{Schema: "app", Table: "t", Name: "c", Kind: "p"}},
		{name: "unobserved", kind: "p", cascade: "false", gap: true},
		{name: "mismatched constraint", kind: "u", cascade: "false", fact: &spec.PGDropConstraintReference{Schema: "app", Table: "t", Name: "C", Kind: "u", Blocked: true}, gap: true},
		{name: "cascade", kind: "p", cascade: "true", fact: &spec.PGDropConstraintReference{Schema: "app", Table: "t", Name: "c", Kind: "p", Blocked: true}},
		{name: "check", kind: "c", cascade: "false"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := pgTargetStatement("drop_constraint", []spec.PGConstraintState{{Name: "c", Type: tc.kind, InheritanceKnown: tc.kind == "c"}})
			s.DDL.Alter[0].Options["cascade"] = tc.cascade
			s.Metadata.TargetTable.PGColumns = &spec.PGColumnCatalog{Ordinary: true}
			s.Metadata.TargetTable.PGRelation = spec.PGRelationState{Name: "t", Kind: "r"}
			s.Metadata.PGDropConstraintReference = tc.fact
			r := pgConstraintTargetRule{id: ruleIDPGAlterDropConstraintTargetRequire, level: rule.LevelBlocker}
			f, err := r.Evaluate(context.Background(), s)
			if err != nil || len(f) != 0 && !tc.blocked || tc.blocked && (len(f) != 1 || f[0].Metadata["reason_code"] != "constraint_referenced_by_foreign_key" || f[0].Metadata["target_constraint_kind"] != tc.kind) {
				t.Fatalf("findings=%+v error=%v", f, err)
			}
			g := r.EvidenceGaps(s)
			if len(g) != 0 && !tc.gap || tc.gap && (len(g) != 1 || g[0].ReasonCode != "unproven_constraint_target" || !reflect.DeepEqual(g[0].RequiredFacts, []string{"constraint.pg_foreign_key_references"})) {
				t.Fatalf("gaps=%+v", g)
			}
		})
	}
}

func TestPGDropInheritedCheck(t *testing.T) {
	r := pgConstraintTargetRule{id: ruleIDPGAlterDropConstraintTargetRequire, level: rule.LevelBlocker}
	for _, tc := range []struct {
		name, kind                                        string
		known, inherited, guard, cascade, stale, boundary bool
	}{
		{name: "inherited", kind: "r", known: true, inherited: true},
		{name: "guard cannot hide inherited check", kind: "r", known: true, inherited: true, guard: true},
		{name: "cascade cannot hide inherited check", kind: "r", known: true, inherited: true, cascade: true},
		{name: "partitioned table", kind: "p", known: true, inherited: true},
		{name: "local", kind: "r", known: true},
		{name: "unobserved", kind: "r"},
		{name: "stale", known: true, inherited: true, stale: true},
		{name: "unsupported kind", kind: "f", known: true, inherited: true, boundary: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := pgTargetStatement("drop_constraint", []spec.PGConstraintState{{Name: "c", Type: "c", InheritanceKnown: tc.known, Inherited: tc.inherited}})
			s.Metadata.TargetTable.PGRelation = spec.PGRelationState{Name: "t", Kind: tc.kind}
			if tc.stale {
				s.Metadata.TargetTable.PGRelation = spec.PGRelationState{}
			}
			if tc.guard {
				s.DDL.Alter[0].Options["if_exists"] = "true"
			}
			if tc.cascade {
				s.DDL.Alter[0].Options["cascade"] = "true"
			}
			f, err := r.Evaluate(context.Background(), s)
			want := tc.known && tc.inherited && !tc.stale && !tc.boundary
			if err != nil || (len(f) == 1) != want || len(f) > 1 || len(f) == 1 && (f[0].Metadata["reason_code"] != "constraint_inherited" || f[0].Metadata["target_constraint_kind"] != "c") {
				t.Fatalf("findings=%+v err=%v", f, err)
			}
			g := r.EvidenceGaps(s)
			if len(g) > 1 || (len(g) == 1) != (!tc.known || tc.stale) || len(g) == 1 && (g[0].ReasonCode != "unproven_constraint_target" || !reflect.DeepEqual(g[0].RequiredFacts, []string{"constraint.pg_inheritance"})) {
				t.Fatalf("gaps=%+v", g)
			}
		})
	}
}

func TestPGDropPartitionCheckOnly(t *testing.T) {
	r := pgConstraintTargetRule{id: ruleIDPGAlterDropConstraintTargetRequire, level: rule.LevelBlocker}
	for _, tc := range []struct {
		name, kind                       string
		only, inherited, known, children bool
		reason, gap                      string
	}{
		{name: "parent ONLY", kind: "p", only: true, known: true, children: true, reason: "constraint_partition_parent_only"},
		{name: "no partitions", kind: "p", only: true, known: true},
		{name: "children unknown", kind: "p", only: true, gap: "constraint.pg_partition_children"},
		{name: "recursive", kind: "p", children: true, known: true},
		{name: "ordinary parent ONLY", kind: "r", only: true},
		{name: "inherited before ONLY", kind: "p", only: true, inherited: true, reason: "constraint_inherited"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := pgTargetStatement("drop_constraint", []spec.PGConstraintState{{Name: "c", Type: "c", InheritanceKnown: true, Inherited: tc.inherited, PartitionCheckHasChildrenKnown: tc.known, PartitionCheckHasChildren: tc.children}})
			s.DDL.PGTableOnly = tc.only
			s.Metadata.TargetTable.PGRelation.Kind = tc.kind
			s.DDL.Alter[0].Options["cascade"] = "true"
			f, err := r.Evaluate(context.Background(), s)
			if err != nil || len(f) > 1 || (len(f) == 1) != (tc.reason != "") || len(f) == 1 && f[0].Metadata["reason_code"] != tc.reason {
				t.Fatalf("findings=%+v err=%v", f, err)
			}
			gaps := r.EvidenceGaps(s)
			if len(gaps) > 1 || (len(gaps) == 1) != (tc.gap != "") || len(gaps) == 1 && !reflect.DeepEqual(gaps[0].RequiredFacts, []string{tc.gap}) {
				t.Fatalf("gaps=%+v", gaps)
			}
		})
	}
}
