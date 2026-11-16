package ddl

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func pgGenerationStatement(c spec.Column) spec.Statement {
	return spec.Statement{Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL, DDL: &spec.DDL{Operation: spec.DDLOperationCreateTable, Table: &spec.Table{Schema: "app", Name: "t"}, Columns: []spec.Column{c}}}
}

func pgGenerationOption(k spec.SequenceOptionKind, v int64) spec.SequenceOption {
	return spec.SequenceOption{Kind: k, Value: &v}
}

func TestPGGenerationAssessment(t *testing.T) {
	for _, tc := range []struct {
		name            string
		change          func(*spec.Column)
		reason, missing string
		cycle           bool
	}{
		{"valid", func(c *spec.Column) {}, "", "", false},
		{"by default", func(c *spec.Column) { c.PGGeneration.Mode = "d" }, "", "", false},
		{"cycle", func(c *spec.Column) { c.PGGeneration.Options = []spec.SequenceOption{{Kind: spec.SequenceOptionCycle}} }, "", "", true},
		{"no cycle", func(c *spec.Column) {
			c.PGGeneration.Options = []spec.SequenceOption{{Kind: spec.SequenceOptionNoCycle}}
		}, "", "", false},
		{"increment zero", func(c *spec.Column) {
			c.PGGeneration.Options = []spec.SequenceOption{pgGenerationOption(spec.SequenceOptionIncrementBy, 0)}
		}, "identity_increment_zero", "", false},
		{"cache zero", func(c *spec.Column) {
			c.PGGeneration.Options = []spec.SequenceOption{pgGenerationOption(spec.SequenceOptionCache, 0)}
		}, "identity_cache_invalid", "", false},
		{"cache negative", func(c *spec.Column) {
			c.PGGeneration.Options = []spec.SequenceOption{pgGenerationOption(spec.SequenceOptionCache, -1)}
		}, "identity_cache_invalid", "", false},
		{"integer range", func(c *spec.Column) {
			c.PGGeneration.TypeWidth = 16
			c.PGGeneration.Options = []spec.SequenceOption{pgGenerationOption(spec.SequenceOptionMaxValue, 32768)}
		}, "identity_bounds_invalid", "", false},
		{"equal bounds", func(c *spec.Column) {
			c.PGGeneration.Options = []spec.SequenceOption{pgGenerationOption(spec.SequenceOptionMinValue, 10), pgGenerationOption(spec.SequenceOptionMaxValue, 10)}
		}, "identity_bounds_invalid", "", false},
		{"start range", func(c *spec.Column) {
			c.PGGeneration.Options = []spec.SequenceOption{pgGenerationOption(spec.SequenceOptionStartWith, 0)}
		}, "identity_start_out_of_bounds", "", false},
		{"large values", func(c *spec.Column) {
			c.PGGeneration.Options = []spec.SequenceOption{pgGenerationOption(spec.SequenceOptionStartWith, 2147483648), pgGenerationOption(spec.SequenceOptionMaxValue, math.MaxInt64)}
		}, "", "", false},
		{"descending", func(c *spec.Column) {
			c.PGGeneration.Options = []spec.SequenceOption{pgGenerationOption(spec.SequenceOptionIncrementBy, math.MinInt64), pgGenerationOption(spec.SequenceOptionMinValue, math.MinInt64), pgGenerationOption(spec.SequenceOptionStartWith, math.MinInt64)}
		}, "", "", false},
		{"null", func(c *spec.Column) { c.PGGeneration.ExplicitNull = true }, "identity_null_conflict", "", false},
		{"type", func(c *spec.Column) { c.PGGeneration.TypeWidth = -1 }, "identity_type_inapplicable", "", false},
		{"default", func(c *spec.Column) { c.HasDefault = true }, "generation_default_conflict", "", false},
		{"stored default", func(c *spec.Column) {
			c.PGGeneration.IdentityCount = 0
			c.PGGeneration.StoredCount = 1
			c.HasDefault = true
		}, "generation_default_conflict", "", false},
		{"stored", func(c *spec.Column) { c.PGGeneration.IdentityCount = 0; c.PGGeneration.StoredCount = 1 }, "", "", false},
		{"nil", func(c *spec.Column) { c.PGGeneration = nil }, "", "column.generation_declaration", false},
		{"repeated mode", func(c *spec.Column) { c.PGGeneration.IdentityCount = 2 }, "", "column.generation_declaration", false},
		{"unknown mode", func(c *spec.Column) { c.PGGeneration.Mode = "x" }, "", "column.generation_declaration", false},
		{"unproven type", func(c *spec.Column) { c.PGGeneration.TypeWidth = 0 }, "", "column.identity_type", false},
		{"unmodeled", func(c *spec.Column) { c.PGGeneration.Unmodeled = true }, "", "column.identity_options", false},
		{"duplicate cycle", func(c *spec.Column) {
			c.PGGeneration.Options = []spec.SequenceOption{{Kind: spec.SequenceOptionCycle}, {Kind: spec.SequenceOptionNoCycle}}
		}, "", "column.identity_options", false},
		{"duplicate bound", func(c *spec.Column) {
			c.PGGeneration.Options = []spec.SequenceOption{{Kind: spec.SequenceOptionNoMinValue}, pgGenerationOption(spec.SequenceOptionMinValue, 1)}
		}, "", "column.identity_options", false},
		{"missing numeric", func(c *spec.Column) { c.PGGeneration.Options = []spec.SequenceOption{{Kind: spec.SequenceOptionCache}} }, "", "column.identity_options", false},
		{"unknown option", func(c *spec.Column) {
			c.PGGeneration.Options = []spec.SequenceOption{{Kind: spec.SequenceOptionUnknown}}
		}, "", "column.identity_options", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := spec.Column{Name: "id", IsIdentity: true, GeneratedWhen: "a", PGGeneration: &spec.PGColumnGeneration{IdentityCount: 1, Mode: "a", TypeWidth: 64}}
			tc.change(&c)
			reason, missing, cycle := pgGenerationAssessment(c)
			if reason != tc.reason || missing != tc.missing || cycle != tc.cycle {
				t.Fatalf("got %q %q %t, want %q %q %t", reason, missing, cycle, tc.reason, tc.missing, tc.cycle)
			}
		})
	}
}

func TestPGGenerationRules(t *testing.T) {
	for _, cycleOnly := range []bool{false, true} {
		r := pgGenerationRule{cycleOnly: cycleOnly, level: rule.LevelWarning}
		c := spec.Column{Name: "id", IsIdentity: true, PGGeneration: &spec.PGColumnGeneration{IdentityCount: 1, Mode: "d", TypeWidth: 64, Options: []spec.SequenceOption{{Kind: spec.SequenceOptionCycle}}}}
		if !cycleOnly {
			c.HasDefault = true
		}
		s := pgGenerationStatement(c)
		f, err := r.Evaluate(context.Background(), s)
		if err != nil || len(f) != 1 || f[0].Level != rule.LevelWarning || f[0].Metadata["column"] != "id" || f[0].Metadata["schema"] != "app" || f[0].Metadata["generation_mode"] != "by_default" {
			t.Fatalf("finding=%+v err=%v", f, err)
		}
		s.DDL.Operation = spec.DDLOperationAlterTable
		s.DDL.Columns = nil
		s.DDL.Alter = []spec.Alter{{Action: "add_column", Column: &spec.AlterColumn{Definition: &c}}}
		f, err = r.Evaluate(context.Background(), s)
		if err != nil || len(f) != 1 || f[0].Metadata["action"] != "add_column" {
			t.Fatalf("ADD COLUMN=%+v err=%v", f, err)
		}
		for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
			s.Dialect = dialect
			if r.AppliesTo(s) {
				t.Fatalf("cross-dialect %s", dialect)
			}
		}
		s.Dialect = spec.DialectPostgreSQL
		s.DDL.Alter[0].Action = "add_identity"
		if r.AppliesTo(s) {
			t.Fatal("existing-column transition must remain outside scope")
		}
		if r.AppliesTo(spec.Statement{}) {
			t.Fatal("nil statement applies")
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := r.Evaluate(ctx, s); !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	}
}

func TestPGGenerationDefaultsAndRegistration(t *testing.T) {
	for _, id := range []string{ruleIDPGGenerationDeclarationValid, ruleIDPGIdentityCycleWarn} {
		cfg := policy.Default()
		p := cfg.Rules[id]
		if !p.Enabled || (id == ruleIDPGGenerationDeclarationValid && p.Level != rule.LevelBlocker) || (id == ruleIDPGIdentityCycleWarn && p.Level != rule.LevelWarning) {
			t.Fatalf("default %+v", p)
		}
		for _, enabled := range []bool{true, false} {
			cfg.Rules = map[string]policy.RulePolicy{id: {Enabled: enabled, Level: rule.LevelNotice}}
			reg := rule.NewRegistry()
			if err := Register(reg, cfg); err != nil {
				t.Fatal(err)
			}
			c := spec.Column{Name: "id", IsIdentity: true, PGGeneration: &spec.PGColumnGeneration{IdentityCount: 1, Mode: "a", TypeWidth: 64, Options: []spec.SequenceOption{{Kind: spec.SequenceOptionCycle}}}}
			if id == ruleIDPGGenerationDeclarationValid {
				c.HasDefault = true
			}
			f, err := reg.EvaluateStatement(context.Background(), pgGenerationStatement(c))
			if err != nil {
				t.Fatal(err)
			}
			if enabled {
				if len(f) != 1 || f[0].Level != rule.LevelNotice || f[0].RuleID != id {
					t.Fatalf("configured %+v", f)
				}
			} else if len(f) != 0 {
				t.Fatalf("disabled %+v", f)
			}
		}
	}
}
