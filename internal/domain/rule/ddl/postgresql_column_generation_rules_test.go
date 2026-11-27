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

func pgIdentityTestInt(v int64) *int64 { return &v }
func pgIdentityTestSequence() *spec.PGIdentitySequenceState {
	cycle := false
	return &spec.PGIdentitySequenceState{TypeWidth: 16, Definition: spec.SequenceDefinition{Start: pgIdentityTestInt(1), Increment: pgIdentityTestInt(1), MinValue: pgIdentityTestInt(1), MaxValue: pgIdentityTestInt(10), CacheValue: pgIdentityTestInt(1), Cycle: &cycle}}
}
func pgGenerationTransitionStatement(action string, c *spec.PGColumnState) spec.Statement {
	members := []spec.PGColumnState{}
	if c != nil {
		copy := *c
		copy.Name = "id"
		members = append(members, copy)
	}
	f := &spec.PGColumnTransition{}
	if action == "add_identity" {
		f.Mode, f.ModeCount = "a", 1
	}
	return spec.Statement{Dialect: spec.DialectPostgreSQL, Kind: spec.KindDDL,
		DDL:      &spec.DDL{Operation: spec.DDLOperationAlterTable, Table: &spec.Table{Schema: "app", Name: "t"}, Alter: []spec.Alter{{Action: action, Name: "id", PGColumnTransition: f, Options: map[string]string{"expression": "private_expression_marker"}}}},
		Metadata: &spec.Metadata{Schema: "app", Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"}, TargetTable: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true, Columns: members}}}}
}

func TestPGColumnGenerationTargetArms(t *testing.T) {
	plain := spec.PGColumnState{TypeWidth: 64, NotNull: true, ReplicaIdentityKeyKnown: true}
	identity := plain
	identity.Identity = "d"
	identity.IdentitySequence = pgIdentityTestSequence()
	stored := plain
	stored.Generated = "s"
	stored.HasDefault = true
	r := pgColumnGenerationRule{id: ruleIDPGColumnGenerationTransitionValid, level: rule.LevelBlocker}
	for _, tc := range []struct {
		name, action    string
		c               *spec.PGColumnState
		guard           bool
		reason, missing string
	}{
		{"add-valid", "add_identity", &plain, false, "", ""},
		{"add-nullable", "add_identity", &spec.PGColumnState{TypeWidth: 64}, false, "identity_requires_not_null", ""},
		{"add-default", "add_identity", &spec.PGColumnState{TypeWidth: 64, NotNull: true, HasDefault: true}, false, "identity_default_conflict", ""},
		{"add-type", "add_identity", &spec.PGColumnState{TypeWidth: -1, NotNull: true}, false, "identity_type_inapplicable", ""},
		{"add-unproven-type", "add_identity", &spec.PGColumnState{NotNull: true}, false, "", "column.identity_type"},
		{"add-identity", "add_identity", &identity, false, "column_already_identity", ""},
		{"add-generated", "add_identity", &stored, false, "column_already_generated", ""},
		{"set-valid", "set_generated", &identity, false, "", ""},
		{"set-ordinary", "set_generated", &plain, false, "identity_target_required", ""},
		{"drop-missing-kind", "drop_identity", &plain, false, "generation_target_kind_mismatch", ""},
		{"drop-guard", "drop_identity", &plain, true, "", ""},
		{"drop-valid", "drop_identity", &identity, false, "", ""},
		{"drop-column-absent-guard", "drop_identity", nil, true, "column_absent", ""},
		{"expression-valid", "drop_expression", &stored, false, "", ""},
		{"expression-wrong-kind", "drop_expression", &identity, false, "generation_target_kind_mismatch", ""},
		{"expression-guard", "drop_expression", &plain, true, "", ""},
		{"set-default-identity", "set_default", &identity, false, "generation_default_conflict", ""},
		{"drop-default-stored", "drop_default", &stored, false, "generation_default_conflict", ""},
		{"default-ordinary", "set_default", &plain, false, "", ""},
		{"drop-null-identity", "drop_not_null", &identity, false, "not_null_required_by_identity_or_primary_key", ""},
		{"drop-null-pk", "drop_not_null", &spec.PGColumnState{PrimaryKey: true}, false, "not_null_required_by_identity_or_primary_key", ""},
		{"drop-null-ordinary", "drop_not_null", &plain, false, "", ""},
		{"set-null-noop", "set_not_null", &plain, false, "", ""},
		{"set-null-data-gap", "set_not_null", &spec.PGColumnState{TypeWidth: 64}, false, "", "column.nullability_data"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := pgGenerationTransitionStatement(tc.action, tc.c)
			s.DDL.Alter[0].PGColumnTransition.IfExists = tc.guard
			reason, missing := r.assess(s, 0)
			if reason != tc.reason || missing != tc.missing {
				t.Fatalf("reason=%s missing=%s", reason, missing)
			}
			f, err := r.Evaluate(context.Background(), s)
			if err != nil || (len(f) == 1) != (reason != "") || (len(r.EvidenceGaps(s)) == 1) != (missing != "") {
				t.Fatalf("f=%+v gaps=%v err=%v", f, r.EvidenceGaps(s), err)
			}
			b, _ := json.Marshal(f)
			if strings.Contains(string(b), "private_expression_marker") {
				t.Fatal("expression leaked")
			}
		})
	}
}

func TestPGPartitionIdentityTransition(t *testing.T) {
	transition := pgColumnGenerationRule{id: ruleIDPGColumnGenerationTransitionValid, level: rule.LevelBlocker}
	options := pgColumnGenerationRule{id: ruleIDPGAlterIdentityOptionsValid, level: rule.LevelBlocker}
	for _, tc := range []struct {
		name, action, kind string
		only, proof        bool
		change             func(*spec.Statement)
	}{
		{"leaf add", "add_identity", "r", false, true, nil},
		{"leaf guarded drop", "drop_identity", "r", false, true, func(s *spec.Statement) { s.DDL.Alter[0].PGColumnTransition.IfExists = true }},
		{"root only drop", "drop_identity", "p", true, true, nil},
		{"root only mode", "set_generated", "p", true, true, func(s *spec.Statement) {
			s.DDL.Alter[0].PGColumnTransition.Mode, s.DDL.Alter[0].PGColumnTransition.ModeCount = "a", 1
		}},
		{"leaf restart", "set_generated", "r", false, true, func(s *spec.Statement) {
			s.DDL.Alter[0].PGColumnTransition.Options = []spec.SequenceOption{{Kind: spec.SequenceOptionRestart}}
		}},
		{"recursive root", "add_identity", "p", false, true, nil},
		{"unproved relation shape", "add_identity", "r", false, false, nil},
		{"unmodeled options", "add_identity", "r", false, true, func(s *spec.Statement) { s.DDL.Alter[0].PGColumnTransition.Unmodeled = true }},
		{"stale fact", "add_identity", "r", false, true, func(s *spec.Statement) { s.Metadata.PGColumnTraversal.Column = "other" }},
		{"unqualified", "add_identity", "r", false, true, func(s *spec.Statement) { s.DDL.Table.Schema = "" }},
		{"multiple actions", "add_identity", "r", false, true, func(s *spec.Statement) { s.DDL.Alter = append(s.DDL.Alter, s.DDL.Alter[0]) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := pgGenerationTransitionStatement(tc.action, nil)
			s.Metadata.TargetTable.PGRelation.Kind = tc.kind
			s.Metadata.TargetTable.PGColumns.Ordinary = false
			s.Metadata.PGColumnTraversal = &spec.PGColumnTraversal{Schema: "app", Table: "t", Column: "id", IdentityChangeBlocked: tc.proof}
			s.DDL.PGTableOnly = tc.only
			if tc.change != nil {
				tc.change(&s)
			}
			reason, missing := transition.assess(s, 0)
			wantBlocked := tc.name == "leaf add" || tc.name == "leaf guarded drop" || tc.name == "root only drop" || tc.name == "root only mode" || tc.name == "leaf restart"
			if (reason == "identity_partition_target_inapplicable") != wantBlocked || (missing != "") == wantBlocked {
				t.Fatalf("reason=%q missing=%q", reason, missing)
			}
			if wantBlocked && options.matches(s.DDL.Alter[0]) {
				if reason, missing := options.assess(s, 0); reason != "" || missing != "" {
					t.Fatalf("options: reason=%q missing=%q", reason, missing)
				}
			}
		})
	}
}

func TestPGSetNotNullValidatedCheck(t *testing.T) {
	r := pgColumnGenerationRule{id: ruleIDPGColumnGenerationTransitionValid, level: rule.LevelBlocker}
	for _, tc := range []struct {
		name   string
		change func(*spec.Statement)
	}{
		{"exact", nil},
		{"unproven", func(s *spec.Statement) { s.Metadata.PGColumnTraversal.ValidatedNotNullCheck = false }},
		{"wrong column", func(s *spec.Statement) { s.Metadata.PGColumnTraversal.Column = "ID" }},
		{"unqualified", func(s *spec.Statement) { s.DDL.Table.Schema = "" }},
		{"stale", func(s *spec.Statement) { s.Metadata.TargetTable.PGRelation = spec.PGRelationState{} }},
		{"wide integer", func(s *spec.Statement) { s.Metadata.TargetTable.PGColumns.Columns[0].TypeWidth = 64 }},
		{"multi-action", func(s *spec.Statement) { s.DDL.Alter = append(s.DDL.Alter, spec.Alter{Action: "drop_constraint"}) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := pgGenerationTransitionStatement("set_not_null", &spec.PGColumnState{TypeWidth: 32})
			s.Metadata.PGColumnTraversal = &spec.PGColumnTraversal{Schema: "app", Table: "t", Column: "id", ColumnExists: true, ValidatedNotNullCheck: true}
			want := ""
			if tc.change != nil {
				tc.change(&s)
				want = "column.nullability_data"
			}
			if reason, gap := r.assess(s, 0); reason != "" || gap != want {
				t.Fatalf("reason=%q gap=%q; want gap=%q", reason, gap, want)
			}
		})
	}
}

func TestPGSetDefaultReferenceTargetPrecedence(t *testing.T) {
	for _, tc := range []struct {
		name, action, reason, missing string
		column                        *spec.PGColumnState
		stale                         bool
	}{
		{name: "ordinary column", action: "set_default", column: &spec.PGColumnState{}, reason: "default_expression_reference_inapplicable"},
		{name: "identity column", action: "set_default", column: &spec.PGColumnState{Identity: "a"}, reason: "generation_default_conflict"},
		{name: "generated column", action: "set_default", column: &spec.PGColumnState{Generated: "s"}, reason: "generation_default_conflict"},
		{name: "absent column", action: "set_default", reason: "column_absent"},
		{name: "stale column", action: "set_default", column: &spec.PGColumnState{}, stale: true, missing: "target_table.pg_columns"},
		{name: "drop default", action: "drop_default", column: &spec.PGColumnState{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := pgGenerationTransitionStatement(tc.action, tc.column)
			s.DDL.Alter[0].PGColumnTransition.DefaultHasDisallowedReference = true
			if tc.stale {
				s.Metadata.TargetTable.PGColumns = nil
			}
			r := pgColumnGenerationRule{id: ruleIDPGColumnGenerationTransitionValid, level: rule.LevelBlocker}
			if reason, missing := r.assess(s, 0); reason != tc.reason || missing != tc.missing {
				t.Fatalf("got reason %q and missing %q, want %q and %q", reason, missing, tc.reason, tc.missing)
			}
		})
	}
}

func TestPGSetDefaultParameterScope(t *testing.T) {
	r := pgColumnGenerationRule{id: ruleIDPGColumnGenerationTransitionValid, level: rule.LevelBlocker}
	for _, tc := range []struct {
		name, reason, gap string
		change            func(*spec.Statement)
	}{
		{"qualified ordinary", "default_parameter_inapplicable", "", func(*spec.Statement) {}},
		{"unqualified", "", "", func(s *spec.Statement) { s.DDL.Table.Schema = "" }},
		{"multi-action", "", "", func(s *spec.Statement) { s.DDL.Alter = append(s.DDL.Alter, spec.Alter{Action: "drop_default"}) }},
		{"nonordinary", "", "target_table.pg_columns", func(s *spec.Statement) { s.Metadata.TargetTable.PGColumns.Ordinary = false }},
		{"generated first", "generation_default_conflict", "", func(s *spec.Statement) { s.Metadata.TargetTable.PGColumns.Columns[0].Generated = "s" }},
		{"reference first", "default_expression_reference_inapplicable", "", func(s *spec.Statement) { s.DDL.Alter[0].PGColumnTransition.DefaultHasDisallowedReference = true }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := pgGenerationTransitionStatement("set_default", &spec.PGColumnState{})
			s.DDL.Alter[0].PGColumnTransition.DefaultHasParameter = true
			tc.change(&s)
			if reason, gap := r.assess(s, 0); reason != tc.reason || gap != tc.gap {
				t.Fatalf("reason=%q gap=%q; want reason=%q gap=%q", reason, gap, tc.reason, tc.gap)
			}
		})
	}
}

func TestPGSetDefaultLiteralAssignment(t *testing.T) {
	r := pgColumnGenerationRule{id: ruleIDPGColumnGenerationTransitionValid, level: rule.LevelBlocker}
	for _, tc := range []struct {
		name, literal string
		column        spec.PGColumnState
		mismatch      bool
	}{
		{"bool to int4", "bool", spec.PGColumnState{TypeWidth: 32}, true},
		{"int4 to bool", "int4", spec.PGColumnState{TypeWidth: -1, BuiltinBool: true}, true},
		{"bool identity", "bool", spec.PGColumnState{TypeWidth: -1, BuiltinBool: true}, false},
		{"integer identity", "int4", spec.PGColumnState{TypeWidth: 32}, false},
		{"non-int4 integer outside slice", "bool", spec.PGColumnState{TypeWidth: 64}, false},
		{"unknown type outside slice", "int4", spec.PGColumnState{}, false},
		{"expression outside slice", "", spec.PGColumnState{TypeWidth: 32}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := pgGenerationTransitionStatement("set_default", &tc.column)
			s.DDL.Alter[0].PGColumnTransition.DefaultLiteralType = tc.literal
			findings, err := r.Evaluate(context.Background(), s)
			if err != nil || (len(findings) == 1) != tc.mismatch || len(findings) == 1 && findings[0].Metadata["reason_code"] != "default_literal_type_mismatch" {
				t.Fatalf("findings=%+v error=%v", findings, err)
			}
		})
	}
}

func TestPGDropNotNullReplicaIdentityKey(t *testing.T) {
	r := pgColumnGenerationRule{id: ruleIDPGColumnGenerationTransitionValid, level: rule.LevelBlocker}
	for _, tc := range []struct {
		name, reason, gap                       string
		key, known, stale, primary, unqualified bool
	}{
		{name: "attached key", key: true, known: true, reason: "not_null_required_by_replica_identity"},
		{name: "known nonmember", known: true},
		{name: "unobserved membership", key: true, gap: "column.replica_identity_key"},
		{name: "stale relation", key: true, known: true, stale: true, gap: "column.replica_identity_key"},
		{name: "primary precedence", primary: true, reason: "not_null_required_by_identity_or_primary_key"},
		{name: "unqualified retains prior behavior", key: true, known: true, unqualified: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := pgGenerationTransitionStatement("drop_not_null", &spec.PGColumnState{TypeWidth: 32, NotNull: true, PrimaryKey: tc.primary,
				ReplicaIdentityKey: tc.key, ReplicaIdentityKeyKnown: tc.known})
			if tc.stale {
				s.Metadata.TargetTable.PGRelation = spec.PGRelationState{}
			}
			if tc.unqualified {
				s.DDL.Table.Schema = ""
			}
			findings, err := r.Evaluate(context.Background(), s)
			if err != nil || (len(findings) == 1) != (tc.reason != "") || len(findings) == 1 && findings[0].Metadata["reason_code"] != tc.reason {
				t.Fatalf("findings=%+v error=%v", findings, err)
			}
			gaps := r.EvidenceGaps(s)
			if (len(gaps) == 1) != (tc.gap != "") || len(gaps) == 1 && (gaps[0].ReasonCode != "unproven_column_generation_target" || !reflect.DeepEqual(gaps[0].RequiredFacts, []string{tc.gap})) {
				t.Fatalf("gaps=%+v", gaps)
			}
		})
	}
}

func TestPGIdentityOptionsAndRiskArms(t *testing.T) {
	c := spec.PGColumnState{Identity: "d", TypeWidth: 64, NotNull: true, IdentitySequence: pgIdentityTestSequence()}
	inc := func(v int64) spec.SequenceOption {
		return spec.SequenceOption{Kind: spec.SequenceOptionIncrementBy, Value: pgIdentityTestInt(v)}
	}
	val := func(k spec.SequenceOptionKind, v int64) spec.SequenceOption {
		return spec.SequenceOption{Kind: k, Value: pgIdentityTestInt(v)}
	}
	bare := func(k spec.SequenceOptionKind) spec.SequenceOption { return spec.SequenceOption{Kind: k} }
	for _, tc := range []struct {
		name            string
		options         []spec.SequenceOption
		reason, missing string
		cycle, restart  bool
	}{
		{"zero-increment", []spec.SequenceOption{inc(0)}, "identity_increment_zero", "", false, false},
		{"zero-cache", []spec.SequenceOption{val(spec.SequenceOptionCache, 0)}, "identity_cache_invalid", "", false, false},
		{"duplicate", []spec.SequenceOption{inc(1), inc(2)}, "identity_options_duplicate", "", false, false},
		{"opposite-cycle", []spec.SequenceOption{bare(spec.SequenceOptionCycle), bare(spec.SequenceOptionNoCycle)}, "identity_options_duplicate", "", false, false},
		{"bounds", []spec.SequenceOption{val(spec.SequenceOptionMinValue, 5), val(spec.SequenceOptionMaxValue, 4)}, "identity_bounds_invalid", "", false, false},
		{"sequence-not-column-type", []spec.SequenceOption{val(spec.SequenceOptionMaxValue, 32768)}, "identity_bounds_invalid", "", false, false},
		{"start-bound", []spec.SequenceOption{val(spec.SequenceOptionStartWith, 11)}, "identity_start_out_of_bounds", "", false, false},
		{"restart-bound", []spec.SequenceOption{val(spec.SequenceOptionRestartWith, 0)}, "identity_restart_out_of_bounds", "", false, false},
		{"bare-restart", []spec.SequenceOption{bare(spec.SequenceOptionRestart)}, "", "", false, true},
		{"start-only", []spec.SequenceOption{val(spec.SequenceOptionStartWith, 5)}, "", "", false, false},
		{"start-and-restart", []spec.SequenceOption{val(spec.SequenceOptionStartWith, 5), bare(spec.SequenceOptionRestart)}, "", "", false, true},
		{"cycle", []spec.SequenceOption{bare(spec.SequenceOptionCycle)}, "", "", true, false},
		{"no-cycle", []spec.SequenceOption{bare(spec.SequenceOptionNoCycle)}, "", "", false, false},
		{"narrow-current", []spec.SequenceOption{val(spec.SequenceOptionMaxValue, 5)}, "", "column.identity_sequence.current_value", false, false},
		{"widen-current-not-needed", []spec.SequenceOption{val(spec.SequenceOptionMaxValue, 20)}, "", "", false, false},
		{"narrow-explicit-restart", []spec.SequenceOption{val(spec.SequenceOptionMaxValue, 5), val(spec.SequenceOptionRestartWith, 3)}, "", "", false, true},
		{"restart-zero-explicit-min", []spec.SequenceOption{val(spec.SequenceOptionMinValue, 1), val(spec.SequenceOptionRestartWith, 0)}, "identity_start_or_restart_out_of_bounds", "", false, false},
		{"malformed-value", []spec.SequenceOption{bare(spec.SequenceOptionCache)}, "", "column.identity_options", false, false},
		{"unknown", []spec.SequenceOption{bare(spec.SequenceOptionUnknown)}, "", "column.identity_options", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := pgGenerationTransitionStatement("set_generated", &c)
			s.DDL.Alter[0].PGColumnTransition.Options = tc.options
			r, m, cy, re := pgAlterIdentityOptions(s.DDL.Alter[0], &c, "", "")
			if r != tc.reason || m != tc.missing || cy != tc.cycle || re != tc.restart {
				t.Fatalf("reason=%s missing=%s cycle=%v restart=%v", r, m, cy, re)
			}
			for _, id := range []string{ruleIDPGAlterIdentityOptionsValid, ruleIDPGAlterIdentityCycleWarn, ruleIDPGAlterIdentityRestartWarn} {
				rule := pgColumnGenerationRule{id: id, level: rule.LevelWarning}
				f, err := rule.Evaluate(context.Background(), s)
				want := id == ruleIDPGAlterIdentityOptionsValid && r != "" || id == ruleIDPGAlterIdentityCycleWarn && cy || id == ruleIDPGAlterIdentityRestartWarn && re
				if err != nil || (len(f) == 1) != want {
					t.Fatalf("id=%s f=%+v err=%v", id, f, err)
				}
			}
		})
	}
}

func TestPGColumnGenerationUnknownAndScope(t *testing.T) {
	c := spec.PGColumnState{Identity: "d", TypeWidth: 64, NotNull: true, IdentitySequence: pgIdentityTestSequence()}
	r := pgColumnGenerationRule{id: ruleIDPGColumnGenerationTransitionValid, level: rule.LevelBlocker}
	for _, mutate := range []func(*spec.Statement){
		func(s *spec.Statement) { s.Metadata = nil },
		func(s *spec.Statement) { s.Metadata.Instance = nil },
		func(s *spec.Statement) { s.Metadata.Instance.Version = "PostgreSQL 18.1" },
		func(s *spec.Statement) { s.Metadata.TargetTable.PGColumns = nil },
		func(s *spec.Statement) { s.Metadata.TargetTable.PGColumns.Ordinary = false },
		func(s *spec.Statement) { s.Metadata.TargetTable.PGColumns.Columns = nil },
		func(s *spec.Statement) {
			s.Metadata.TargetTable.PGColumns.Columns = append(s.Metadata.TargetTable.PGColumns.Columns, s.Metadata.TargetTable.PGColumns.Columns[0])
		},
		func(s *spec.Statement) {
			s.Metadata.TargetTable.PGColumns.Columns[0].Name = "ID"
			s.DDL.Alter[0].Name = "ID"
			s.Metadata.TargetTable.PGColumns.Columns[0].Identity = "x"
		},
		func(s *spec.Statement) { s.Metadata.Schema = "other" },
		func(s *spec.Statement) { s.Metadata.TargetTable.Schema = "other" },
		func(s *spec.Statement) { s.Metadata.TargetTable.Table.Name = "T" },
	} {
		s := pgGenerationTransitionStatement("set_generated", &c)
		mutate(&s)
		f, err := r.Evaluate(context.Background(), s)
		if err != nil || len(f) != 0 || len(r.EvidenceGaps(s)) != 1 {
			t.Fatalf("unknown=%+v %v %v", f, r.EvidenceGaps(s), err)
		}
	}
	s := pgGenerationTransitionStatement("set_generated", &c)
	s.DDL.Alter = append(s.DDL.Alter, s.DDL.Alter[0])
	if len(r.EvidenceGaps(s)) != 1 {
		t.Fatal("unapplied sibling used old directory")
	}
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		s.Dialect = dialect
		if r.AppliesTo(s) || len(r.EvidenceGaps(s)) != 0 {
			t.Fatal("cross dialect match")
		}
	}
	s = pgGenerationTransitionStatement("drop_identity", &spec.PGColumnState{})
	s.DDL.Alter[0].PGColumnTransition = nil
	if _, m := r.assess(s, 0); m != "column.generation_transition" {
		t.Fatal("missing guard guessed")
	}
	s = pgGenerationTransitionStatement("set_generated", &c)
	s.DDL.Alter[0].PGColumnTransition.Mode = "a"
	s.DDL.Alter[0].PGColumnTransition.ModeCount = 1
	s.Metadata.TargetTable.PGColumns.Columns[0].IdentitySequence = nil
	opts := pgColumnGenerationRule{id: ruleIDPGAlterIdentityOptionsValid, level: rule.LevelBlocker}
	if reason, missing := opts.assess(s, 0); reason != "" || missing != "" {
		t.Fatal("mode-only needlessly required sequence")
	}
	s.DDL.Alter[0].PGColumnTransition.Options = []spec.SequenceOption{{Kind: spec.SequenceOptionCycle}}
	if _, missing := opts.assess(s, 0); missing != "column.identity_sequence_definition" {
		t.Fatal("missing sequence guessed")
	}
	s.DDL.Alter[0].PGColumnTransition.Options = []spec.SequenceOption{{Kind: spec.SequenceOptionCache, Value: pgIdentityTestInt(0)}}
	s.Metadata.TargetTable.PGColumns = nil
	if reason, _ := opts.assess(s, 0); reason != "identity_cache_invalid" {
		t.Fatal("intrinsic refusal lost behind missing catalog")
	}
	s.Metadata.Instance = nil
	if reason, missing := opts.assess(s, 0); reason != "" || missing != "instance.version" {
		t.Fatal("out-of-scope version proof guessed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := r.Evaluate(ctx, s); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestPGColumnGenerationDefaultsAndConfiguration(t *testing.T) {
	for _, id := range []string{ruleIDPGColumnGenerationTransitionValid, ruleIDPGAlterIdentityOptionsValid, ruleIDPGAlterIdentityRestartWarn, ruleIDPGAlterIdentityCycleWarn} {
		p := policy.Default().Rules[id]
		want := rule.LevelBlocker
		if strings.HasSuffix(id, ".warn") {
			want = rule.LevelWarning
		}
		if !p.Enabled || p.Level != want || len(p.Params) != 0 {
			t.Fatalf("default %s %+v", id, p)
		}
		for _, enabled := range []bool{true, false} {
			reg := rule.NewRegistry()
			if err := Register(reg, policy.Policy{Rules: map[string]policy.RulePolicy{id: {Enabled: enabled, Level: rule.LevelNotice}}}); err != nil {
				t.Fatal(err)
			}
			if reg.Contains(id) != enabled {
				t.Fatal("enabled flag ignored")
			}
		}
	}
	c := spec.PGColumnState{TypeWidth: 64, NotNull: true}
	s := pgGenerationTransitionStatement("add_identity", &c)
	s.DDL.Alter[0].PGColumnTransition.Options = []spec.SequenceOption{{Kind: spec.SequenceOptionIncrementBy, Value: pgIdentityTestInt(9223372036854775807)}}
	before := s.DDL.Alter[0].PGColumnTransition.Options[0]
	r := pgColumnGenerationRule{id: ruleIDPGAlterIdentityOptionsValid, level: rule.LevelBlocker}
	if reason, missing := r.assess(s, 0); reason != "" || missing != "" || !reflect.DeepEqual(before, s.DDL.Alter[0].PGColumnTransition.Options[0]) {
		t.Fatalf("int64 ADD reused declaration invalid/mutated: %s %s", reason, missing)
	}
	s.DDL.Alter[0].PGColumnTransition.ModeCount = 2
	if reason, _ := r.assess(s, 0); reason != "identity_modes_duplicate" {
		t.Fatal("duplicate mode became last-wins")
	}
}
