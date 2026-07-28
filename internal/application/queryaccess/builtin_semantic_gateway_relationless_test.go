package queryaccess

import (
	"context"
	"strings"
	"testing"

	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

func relationlessLiteralGatewayResult() domain.Result {
	return domain.Result{
		Dialect:            "mysql",
		Mode:               domain.ModeStrict,
		ReadClassification: domain.Indeterminate,
		Admission:          domain.IndeterminateAdmission,
		ReasonCodes:        []domain.ReasonCode{domain.ReasonFunctionEffect},
		Outputs:            []domain.OutputColumn{{Name: "lower('x')"}},
	}
}

func relationlessLiteralCandidate(name, class string, kinds []string) EffectCandidate {
	return EffectCandidate{
		Kind:                 EffectCandidateFunction,
		Ordinal:              0,
		NamePath:             []string{name},
		OriginalNamePath:     []string{strings.ToUpper(name)},
		Canonical:            true,
		ParserClassification: class,
		Arity:                len(kinds),
		OperandKinds:         kinds,
		IsAggregate:          class == "aggregate",
	}
}

func TestRelationlessLiteralRequirementsComplete_AdmitsExactLiteralShape(t *testing.T) {
	t.Parallel()
	result := relationlessLiteralGatewayResult()
	candidate := relationlessLiteralCandidate("lower", "keyword", []string{"const"})
	if !relationlessLiteralRequirementsComplete(result, nil, []EffectCandidate{candidate}) {
		t.Fatal("exact relationless literal shape was not admitted")
	}
}

func TestRelationlessLiteralRequirementsComplete_FailClosedTable(t *testing.T) {
	t.Parallel()
	base := relationlessLiteralCandidate("lower", "keyword", []string{"const"})
	cases := []struct {
		name       string
		result     domain.Result
		reqs       []domain.Requirement
		candidates []EffectCandidate
	}{
		{
			name: "non strict mode",
			result: func() domain.Result {
				r := relationlessLiteralGatewayResult()
				r.Mode = domain.ModeProjectionOnly
				return r
			}(),
			candidates: []EffectCandidate{base},
		},
		{
			name: "relation present",
			result: func() domain.Result {
				r := relationlessLiteralGatewayResult()
				r.Relations = []domain.RelationReference{{Schema: "app", Name: "users", Kind: domain.RelationTable, PermissionRequired: true}}
				return r
			}(),
			candidates: []EffectCandidate{base},
		},
		{
			name: "referenced column present",
			result: func() domain.Result {
				r := relationlessLiteralGatewayResult()
				r.ReferencedColumns = []domain.ColumnReference{{Schema: "app", Table: "users", Column: "name"}}
				return r
			}(),
			candidates: []EffectCandidate{base},
		},
		{
			name: "unresolved present",
			result: func() domain.Result {
				r := relationlessLiteralGatewayResult()
				r.Unresolved = []domain.Unresolved{{Reference: "missing", Reason: domain.ReasonSchemaUnavailable}}
				return r
			}(),
			candidates: []EffectCandidate{base},
		},
		{
			name:       "requirement present",
			result:     relationlessLiteralGatewayResult(),
			reqs:       []domain.Requirement{{Object: "app.users", Privilege: "read_table"}},
			candidates: []EffectCandidate{base},
		},
		{
			name: "wildcard output",
			result: func() domain.Result {
				r := relationlessLiteralGatewayResult()
				r.Outputs = []domain.OutputColumn{{Name: "*"}}
				return r
			}(),
			candidates: []EffectCandidate{base},
		},
		{
			name: "qualified wildcard output",
			result: func() domain.Result {
				r := relationlessLiteralGatewayResult()
				r.Outputs = []domain.OutputColumn{{Name: "t.*"}}
				return r
			}(),
			candidates: []EffectCandidate{base},
		},
		{
			name:       "no candidates",
			result:     relationlessLiteralGatewayResult(),
			candidates: nil,
		},
		{
			name:   "column operand kind",
			result: relationlessLiteralGatewayResult(),
			candidates: []EffectCandidate{
				relationlessLiteralCandidate("lower", "keyword", []string{"column"}),
			},
		},
		{
			name:   "param operand kind",
			result: relationlessLiteralGatewayResult(),
			candidates: []EffectCandidate{
				relationlessLiteralCandidate("lower", "keyword", []string{"param"}),
			},
		},
		{
			name:   "expr operand kind",
			result: relationlessLiteralGatewayResult(),
			candidates: []EffectCandidate{
				relationlessLiteralCandidate("lower", "keyword", []string{"expr"}),
			},
		},
		{
			name:   "star operand kind",
			result: relationlessLiteralGatewayResult(),
			candidates: []EffectCandidate{
				relationlessLiteralCandidate("count", "aggregate", []string{"star"}),
			},
		},
		{
			name:   "zero operand candidate",
			result: relationlessLiteralGatewayResult(),
			candidates: []EffectCandidate{
				relationlessLiteralCandidate("lower", "keyword", nil),
			},
		},
		{
			name:   "operand column ref present",
			result: relationlessLiteralGatewayResult(),
			candidates: []EffectCandidate{func() EffectCandidate {
				c := relationlessLiteralCandidate("lower", "keyword", []string{"const"})
				c.OperandColumnRefs = []OperandColumnRef{{Schema: "app", Table: "users", Column: "name"}}
				return c
			}()},
		},
		{
			name:   "window partition column ref present",
			result: relationlessLiteralGatewayResult(),
			candidates: []EffectCandidate{func() EffectCandidate {
				c := relationlessLiteralCandidate("lower", "keyword", []string{"const"})
				c.WindowPartitionColumnRefs = []OperandColumnRef{{Schema: "app", Table: "users", Column: "dept"}}
				return c
			}()},
		},
		{
			name:   "window order column ref present",
			result: relationlessLiteralGatewayResult(),
			candidates: []EffectCandidate{func() EffectCandidate {
				c := relationlessLiteralCandidate("lower", "keyword", []string{"const"})
				c.WindowOrderColumnRefs = []OperandColumnRef{{Schema: "app", Table: "users", Column: "id"}}
				return c
			}()},
		},
		{
			name:   "mixed candidate breaks statement",
			result: relationlessLiteralGatewayResult(),
			candidates: []EffectCandidate{
				relationlessLiteralCandidate("lower", "keyword", []string{"const"}),
				func() EffectCandidate {
					c := relationlessLiteralCandidate("upper", "keyword", []string{"expr"})
					c.Ordinal = 1
					return c
				}(),
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if relationlessLiteralRequirementsComplete(tc.result, tc.reqs, tc.candidates) {
				t.Fatalf("%s was admitted by the relationless literal predicate", tc.name)
			}
		})
	}
}

func TestBuiltinSemanticGateway_RelationlessLiteralOnlyProven(t *testing.T) {
	t.Parallel()
	registry := fixedManifestRegistry(t, BuiltinSemanticEntry{
		Dialect:      "mysql",
		Profile:      AnalysisProfileMySQL57,
		Name:         "lower",
		CallClass:    BuiltinSemanticScalar,
		Arity:        1,
		OperandKinds: []string{"const"},
	})
	result := relationlessLiteralGatewayResult()
	candidate := relationlessLiteralCandidate("lower", "keyword", []string{"const"})

	proof := proveBuiltinSemantics(AnalysisProfileMySQL57, "mysql", []EffectCandidate{candidate}, result, result.Requirements, registry)
	if proof.decision != builtinSemanticAllProven {
		t.Fatalf("decision = %q, want all_proven", proof.decision)
	}
}

func TestBuiltinSemanticGateway_RelationlessCountLiteralProven(t *testing.T) {
	t.Parallel()
	registry := fixedManifestRegistry(t, BuiltinSemanticEntry{
		Dialect:      "mysql",
		Profile:      AnalysisProfileMySQL57,
		Name:         "count",
		CallClass:    BuiltinSemanticAggregate,
		Arity:        1,
		OperandKinds: []string{"const"},
	})
	result := relationlessLiteralGatewayResult()
	candidate := relationlessLiteralCandidate("count", "aggregate", []string{"const"})

	proof := proveBuiltinSemantics(AnalysisProfileMySQL57, "mysql", []EffectCandidate{candidate}, result, result.Requirements, registry)
	if proof.decision != builtinSemanticAllProven {
		t.Fatalf("decision = %q, want all_proven", proof.decision)
	}
}

func TestBuiltinSemanticGateway_RelationlessTwoConstProven(t *testing.T) {
	t.Parallel()
	registry := fixedManifestRegistry(t, BuiltinSemanticEntry{
		Dialect:      "mysql",
		Profile:      AnalysisProfileMySQL57,
		Name:         "coalesce",
		CallClass:    BuiltinSemanticScalar,
		Arity:        2,
		OperandKinds: []string{"const", "const"},
	})
	result := relationlessLiteralGatewayResult()
	candidate := relationlessLiteralCandidate("coalesce", "keyword", []string{"const", "const"})

	proof := proveBuiltinSemantics(AnalysisProfileMySQL57, "mysql", []EffectCandidate{candidate}, result, result.Requirements, registry)
	if proof.decision != builtinSemanticAllProven {
		t.Fatalf("decision = %q, want all_proven", proof.decision)
	}
}

func TestBuiltinSemanticGateway_RelationlessPostgreSQLNotProven(t *testing.T) {
	t.Parallel()
	registry := fixedManifestRegistry(t, BuiltinSemanticEntry{
		Dialect:      "mysql",
		Profile:      AnalysisProfileMySQL57,
		Name:         "lower",
		CallClass:    BuiltinSemanticScalar,
		Arity:        1,
		OperandKinds: []string{"const"},
	})
	result := relationlessLiteralGatewayResult()
	result.Dialect = "postgresql"
	candidate := relationlessLiteralCandidate("lower", "keyword", []string{"const"})

	proof := proveBuiltinSemantics(AnalysisProfileMySQL57, "postgresql", []EffectCandidate{candidate}, result, result.Requirements, registry)
	if proof.decision == builtinSemanticAllProven {
		t.Fatal("postgresql dialect was proven by the relationless path")
	}
}

func TestBuiltinSemanticGateway_RelationlessEmptyProfileNotProven(t *testing.T) {
	t.Parallel()
	registry := fixedManifestRegistry(t, BuiltinSemanticEntry{
		Dialect:      "mysql",
		Profile:      AnalysisProfileMySQL57,
		Name:         "lower",
		CallClass:    BuiltinSemanticScalar,
		Arity:        1,
		OperandKinds: []string{"const"},
	})
	result := relationlessLiteralGatewayResult()
	candidate := relationlessLiteralCandidate("lower", "keyword", []string{"const"})

	proof := proveBuiltinSemantics(AnalysisProfileEmpty, "mysql", []EffectCandidate{candidate}, result, result.Requirements, registry)
	if proof.decision == builtinSemanticAllProven {
		t.Fatal("empty profile was proven by the relationless path")
	}
}

func TestBuiltinSemanticGateway_RelationlessRejectsMalformedCandidates(t *testing.T) {
	t.Parallel()
	registry := fixedManifestRegistry(t, BuiltinSemanticEntry{
		Dialect:      "mysql",
		Profile:      AnalysisProfileMySQL57,
		Name:         "lower",
		CallClass:    BuiltinSemanticScalar,
		Arity:        1,
		OperandKinds: []string{"const"},
	})
	result := relationlessLiteralGatewayResult()
	cases := map[string]EffectCandidate{
		"quoted": func() EffectCandidate {
			c := relationlessLiteralCandidate("lower", "keyword", []string{"const"})
			c.IsQuoted = true
			return c
		}(),
		"explicit schema": func() EffectCandidate {
			c := relationlessLiteralCandidate("lower", "keyword", []string{"const"})
			c.ExplicitSchema = true
			return c
		}(),
		"noncanonical": func() EffectCandidate {
			c := relationlessLiteralCandidate("lower", "keyword", []string{"const"})
			c.Canonical = false
			return c
		}(),
		"ambiguous": func() EffectCandidate {
			c := relationlessLiteralCandidate("lower", "keyword", []string{"const"})
			c.Ambiguous = true
			return c
		}(),
		"cast kind": func() EffectCandidate {
			c := relationlessLiteralCandidate("lower", "keyword", []string{"const"})
			c.Kind = EffectCandidateCast
			return c
		}(),
		"cast target": func() EffectCandidate {
			c := relationlessLiteralCandidate("lower", "keyword", []string{"const"})
			c.TargetTypePath = []string{"char"}
			return c
		}(),
		"unknown name": relationlessLiteralCandidate("mystery_fn", "keyword", []string{"const"}),
		"arity kind mismatch": func() EffectCandidate {
			c := relationlessLiteralCandidate("lower", "keyword", []string{"const"})
			c.Arity = 2
			return c
		}(),
		"scalar modifier": func() EffectCandidate {
			c := relationlessLiteralCandidate("lower", "keyword", []string{"const"})
			c.HasDistinct = true
			return c
		}(),
	}
	for name, candidate := range cases {
		proof := proveBuiltinSemantics(AnalysisProfileMySQL57, "mysql", []EffectCandidate{candidate}, result, result.Requirements, registry)
		if proof.decision == builtinSemanticAllProven {
			t.Errorf("%s candidate was proven: %+v", name, candidate)
		}
	}
}

func TestBuiltinSemanticGateway_RelationBearingStillRequiresPhysicalRequirements(t *testing.T) {
	t.Parallel()
	registry := mixedConstRegistry(t)
	candidate := mixedConstCandidate([]string{"column", "const"}, 2)

	complete := mixedConstResult()
	proof := proveBuiltinSemantics(AnalysisProfileMySQL57, "mysql", []EffectCandidate{candidate}, complete, complete.Requirements, registry)
	if proof.decision != builtinSemanticAllProven {
		t.Fatalf("relation-bearing complete proof = %q, want all_proven", proof.decision)
	}

	missing := mixedConstResult()
	missing.Requirements = nil
	proof = proveBuiltinSemantics(AnalysisProfileMySQL57, "mysql", []EffectCandidate{candidate}, missing, nil, registry)
	if proof.decision == builtinSemanticAllProven {
		t.Fatal("relation-bearing result with empty requirements leaked through the relationless path")
	}
}

func TestBuiltinSemanticService_RelationlessLiteralShapesAcrossProfiles(t *testing.T) {
	t.Parallel()
	shapes := []struct {
		name string
		sql  string
	}{
		{"lower_const", "SELECT LOWER('x')"},
		{"upper_const", "SELECT UPPER('x')"},
		{"length_const", "SELECT LENGTH('x')"},
		{"char_length_const", "SELECT CHAR_LENGTH('x')"},
		{"abs_const", "SELECT ABS(42)"},
		{"ceil_const", "SELECT CEIL(42)"},
		{"ceiling_const", "SELECT CEILING(42)"},
		{"floor_const", "SELECT FLOOR(42)"},
		{"count_literal", "SELECT COUNT(1)"},
		{"coalesce_const_const", "SELECT COALESCE('x', 'y')"},
		{"nullif_const_const", "SELECT NULLIF('x', 'y')"},
		{"ifnull_const_const", "SELECT IFNULL('x', 'y')"},
	}
	profiles := []struct {
		profile AnalysisProfile
		dialect string
	}{
		{AnalysisProfileMySQL57, "mysql"},
		{AnalysisProfileMySQL80, "mysql"},
		{AnalysisProfileMySQL84, "mysql"},
		{AnalysisProfileTiDB85, "tidb"},
	}
	service, err := NewMySQLTiDBSemanticService(&builtinTestResolver{})
	if err != nil {
		t.Fatalf("semantic service: %v", err)
	}
	for _, profile := range profiles {
		for _, shape := range shapes {
			t.Run(string(profile.profile)+"/"+shape.name, func(t *testing.T) {
				t.Parallel()
				result, err := service.Analyze(context.Background(), QueryAccessRequest{
					SQL:             shape.sql,
					Dialect:         profile.dialect,
					Mode:            "strict",
					DefaultSchema:   "app",
					AnalysisProfile: profile.profile,
				})
				if err != nil {
					t.Fatalf("analyze: %v", err)
				}
				got := result.DomainResult
				if got.ReadClassification != domain.ReadOnly {
					t.Fatalf("classification = %q, want read_only (reasons %v)", got.ReadClassification, got.ReasonCodes)
				}
				if got.Admission != domain.Admissible {
					t.Fatalf("admission = %q, want admissible", got.Admission)
				}
				if len(got.Requirements) != 0 {
					t.Fatalf("requirements = %+v, want empty", got.Requirements)
				}
				if len(got.Relations) != 0 || len(got.ReferencedColumns) != 0 || len(got.Unresolved) != 0 {
					t.Fatalf("unexpected physical facts: %+v", got)
				}
			})
		}
	}
}

func TestBuiltinSemanticService_RelationlessDefaultServiceStaysIndeterminate(t *testing.T) {
	t.Parallel()
	result, err := (&Service{}).Analyze(context.Background(), QueryAccessRequest{
		SQL:             "SELECT LOWER('x')",
		Dialect:         "mysql",
		Mode:            "strict",
		DefaultSchema:   "app",
		AnalysisProfile: AnalysisProfileMySQL57,
		SchemaResolver:  &builtinTestResolver{},
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if result.DomainResult.ReadClassification != domain.Indeterminate || result.DomainResult.Admission != domain.IndeterminateAdmission {
		t.Fatalf("default service promoted relationless literal: %q/%q", result.DomainResult.ReadClassification, result.DomainResult.Admission)
	}
}

func TestBuiltinSemanticService_RelationlessCandidateFreeSelectOneUnchanged(t *testing.T) {
	t.Parallel()
	service, err := NewMySQLTiDBSemanticService(&builtinTestResolver{})
	if err != nil {
		t.Fatalf("semantic service: %v", err)
	}
	for _, tc := range []struct {
		service *Service
		label   string
	}{
		{service, "semantic service"},
		{&Service{}, "default service"},
	} {
		result, err := tc.service.Analyze(context.Background(), QueryAccessRequest{
			SQL:             "SELECT 1",
			Dialect:         "mysql",
			Mode:            "strict",
			DefaultSchema:   "app",
			AnalysisProfile: AnalysisProfileMySQL57,
		})
		if err != nil {
			t.Fatalf("%s analyze: %v", tc.label, err)
		}
		got := result.DomainResult
		if got.ReadClassification != domain.ReadOnly || got.Admission != domain.Admissible {
			t.Fatalf("%s SELECT 1 changed: %q/%q", tc.label, got.ReadClassification, got.Admission)
		}
		if len(got.Requirements) != 0 || len(got.Relations) != 0 || len(got.ReferencedColumns) != 0 || len(got.Unresolved) != 0 || len(got.ReasonCodes) != 0 {
			t.Fatalf("%s SELECT 1 facts changed: %+v", tc.label, got)
		}
	}
}

func TestBuiltinSemanticService_RelationBearingMixedConstKeepsRequirements(t *testing.T) {
	t.Parallel()
	service, err := NewMySQLTiDBSemanticService(&relationlessAmountResolver{})
	if err != nil {
		t.Fatalf("semantic service: %v", err)
	}
	result, err := service.Analyze(context.Background(), QueryAccessRequest{
		SQL:             "SELECT COALESCE(amount, 0) FROM app.facts",
		Dialect:         "mysql",
		Mode:            "strict",
		DefaultSchema:   "app",
		AnalysisProfile: AnalysisProfileMySQL57,
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	got := result.DomainResult
	if got.ReadClassification != domain.ReadOnly || got.Admission != domain.Admissible {
		t.Fatalf("mixed const regression: %q/%q (reasons %v)", got.ReadClassification, got.Admission, got.ReasonCodes)
	}
	wantRequirements := []domain.Requirement{
		{Object: "app.facts", Privilege: "read_table"},
		{Object: "app.facts.amount", Privilege: "read_column"},
	}
	if len(got.Requirements) != len(wantRequirements) {
		t.Fatalf("requirements = %+v, want %+v", got.Requirements, wantRequirements)
	}
	for i, want := range wantRequirements {
		if got.Requirements[i] != want {
			t.Fatalf("requirements[%d] = %+v, want %+v", i, got.Requirements[i], want)
		}
	}
}

type relationlessAmountResolver struct{}

func (*relationlessAmountResolver) ResolveRelation(context.Context, string, string, string) (RelationSchema, error) {
	return RelationSchema{Schema: "app", Name: "facts", Kind: "table", Columns: []ColumnSchema{{Name: "amount", Ordinal: 1}}}, nil
}

func TestBuiltinSemanticGateway_RelationlessRejectsTiDBProfileOnMySQLDialect(t *testing.T) {
	t.Parallel()
	registry := fixedManifestRegistry(t, BuiltinSemanticEntry{
		Dialect:      "mysql",
		Profile:      AnalysisProfileMySQL57,
		Name:         "lower",
		CallClass:    BuiltinSemanticScalar,
		Arity:        1,
		OperandKinds: []string{"const"},
	})
	result := relationlessLiteralGatewayResult()
	candidate := relationlessLiteralCandidate("lower", "keyword", []string{"const"})

	proof := proveBuiltinSemantics(AnalysisProfileTiDB85, "mysql", []EffectCandidate{candidate}, result, result.Requirements, registry)
	if proof.decision == builtinSemanticAllProven {
		t.Fatal("tidb profile with mysql dialect was proven")
	}
}
