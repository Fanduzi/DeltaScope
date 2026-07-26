package queryaccess

import (
	"context"
	"testing"

	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

func TestBuiltinSemanticGateway_AllowsOnlyCompleteFixtureProof(t *testing.T) {
	manifest := builtinTestManifest(t)
	registry, err := newBuiltinSemanticRegistry(map[AnalysisProfile]*BuiltinSemanticManifest{
		AnalysisProfileMySQL57: manifest,
	})
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}

	result := builtinTestResult()
	proof := proveBuiltinSemantics(
		AnalysisProfileMySQL57,
		"mysql",
		[]EffectCandidate{builtinTestCandidate()},
		result,
		result.Requirements,
		registry,
	)
	if proof.decision != builtinSemanticAllProven {
		t.Fatalf("decision = %q, want all_proven", proof.decision)
	}
}

func TestBuiltinSemanticGateway_ScalarMixedConstMatchesManifest(t *testing.T) {
	t.Parallel()
	registry := mixedConstRegistry(t)
	candidate := mixedConstCandidate([]string{"column", "const"}, 2)
	result := mixedConstResult()

	proof := proveBuiltinSemantics(AnalysisProfileMySQL57, "mysql", []EffectCandidate{candidate}, result, result.Requirements, registry)
	if proof.decision != builtinSemanticAllProven {
		t.Fatalf("decision = %q, want all_proven", proof.decision)
	}
}

func TestBuiltinSemanticGateway_ScalarMixedConstVariableArityMatches(t *testing.T) {
	t.Parallel()
	registry := mixedConstRegistry(t)
	candidate := mixedConstCandidate([]string{"column", "const", "const"}, 3)
	result := mixedConstResult()

	proof := proveBuiltinSemantics(AnalysisProfileMySQL57, "mysql", []EffectCandidate{candidate}, result, result.Requirements, registry)
	if proof.decision != builtinSemanticAllProven {
		t.Fatalf("decision = %q, want all_proven", proof.decision)
	}
}

func TestBuiltinSemanticGateway_ScalarMixedConstReversedNotProven(t *testing.T) {
	t.Parallel()
	registry := mixedConstRegistry(t)
	candidate := mixedConstCandidate([]string{"const", "column"}, 2)
	result := mixedConstResult()

	proof := proveBuiltinSemantics(AnalysisProfileMySQL57, "mysql", []EffectCandidate{candidate}, result, result.Requirements, registry)
	if proof.decision == builtinSemanticAllProven {
		t.Fatal("reversed operand order was proven")
	}
}

func TestBuiltinSemanticGateway_ScalarMixedConstExprNotProven(t *testing.T) {
	t.Parallel()
	registry := mixedConstRegistry(t)
	candidate := mixedConstCandidate([]string{"column", "expr"}, 2)
	result := mixedConstResult()

	proof := proveBuiltinSemantics(AnalysisProfileMySQL57, "mysql", []EffectCandidate{candidate}, result, result.Requirements, registry)
	if proof.decision == builtinSemanticAllProven {
		t.Fatal("expr operand was proven")
	}
}

func TestBuiltinSemanticGateway_ScalarMixedConstArityMismatchNotProven(t *testing.T) {
	t.Parallel()
	registry := mixedConstRegistry(t)
	// Arity says 3 but OperandKinds only has 2 entries — must fail closed.
	candidate := mixedConstCandidate([]string{"column", "const"}, 3)
	result := mixedConstResult()

	proof := proveBuiltinSemantics(AnalysisProfileMySQL57, "mysql", []EffectCandidate{candidate}, result, result.Requirements, registry)
	if proof.decision == builtinSemanticAllProven {
		t.Fatal("arity/kinds mismatch was proven")
	}
}

func TestBuiltinSemanticGateway_ScalarLiteralOnlyMatchesFixedArityManifest(t *testing.T) {
	t.Parallel()

	registry := fixedManifestRegistry(t, BuiltinSemanticEntry{
		Dialect:      "mysql",
		Profile:      AnalysisProfileMySQL57,
		Name:         "lower",
		CallClass:    BuiltinSemanticScalar,
		Arity:        1,
		OperandKinds: []string{"const"},
	})
	candidate := EffectCandidate{
		Kind:                 EffectCandidateFunction,
		Ordinal:              0,
		NamePath:             []string{"lower"},
		OriginalNamePath:     []string{"LOWER"},
		Canonical:            true,
		ParserClassification: "generic",
		Arity:                1,
		OperandKinds:         []string{"const"},
	}

	proof := proveBuiltinSemantics(AnalysisProfileMySQL57, "mysql", []EffectCandidate{candidate}, literalOnlyResult(), literalOnlyResult().Requirements, registry)
	if proof.decision != builtinSemanticAllProven {
		t.Fatalf("decision = %q, want all_proven", proof.decision)
	}
}

func TestBuiltinSemanticGateway_CountLiteralMatchesAggregateManifest(t *testing.T) {
	t.Parallel()

	registry := fixedManifestRegistry(t, BuiltinSemanticEntry{
		Dialect:      "mysql",
		Profile:      AnalysisProfileMySQL57,
		Name:         "count",
		CallClass:    BuiltinSemanticAggregate,
		Arity:        1,
		OperandKinds: []string{"const"},
	})
	candidate := EffectCandidate{
		Kind:                 EffectCandidateFunction,
		Ordinal:              0,
		NamePath:             []string{"count"},
		OriginalNamePath:     []string{"COUNT"},
		Canonical:            true,
		ParserClassification: "aggregate",
		Arity:                1,
		OperandKinds:         []string{"const"},
		IsAggregate:          true,
	}

	result := literalOnlyResult()
	proof := proveBuiltinSemantics(AnalysisProfileMySQL57, "mysql", []EffectCandidate{candidate}, result, result.Requirements, registry)
	if proof.decision != builtinSemanticAllProven {
		t.Fatalf("decision = %q, want all_proven", proof.decision)
	}
}

func TestBuiltinSemanticGateway_ScalarReversedOperandsMatchFixedArityManifest(t *testing.T) {
	t.Parallel()

	registry := fixedManifestRegistry(t, BuiltinSemanticEntry{
		Dialect:      "mysql",
		Profile:      AnalysisProfileMySQL57,
		Name:         "coalesce",
		CallClass:    BuiltinSemanticScalar,
		Arity:        2,
		OperandKinds: []string{"const", "column"},
	})
	candidate := mixedConstCandidate([]string{"const", "column"}, 2)
	result := mixedConstResult()

	proof := proveBuiltinSemantics(AnalysisProfileMySQL57, "mysql", []EffectCandidate{candidate}, result, result.Requirements, registry)
	if proof.decision != builtinSemanticAllProven {
		t.Fatalf("decision = %q, want all_proven", proof.decision)
	}
}

func TestBuiltinSemanticGateway_ScalarAllConstantsMatchFixedArityManifest(t *testing.T) {
	t.Parallel()

	registry := fixedManifestRegistry(t, BuiltinSemanticEntry{
		Dialect:      "mysql",
		Profile:      AnalysisProfileMySQL57,
		Name:         "coalesce",
		CallClass:    BuiltinSemanticScalar,
		Arity:        2,
		OperandKinds: []string{"const", "const"},
	})
	candidate := mixedConstCandidate([]string{"const", "const"}, 2)
	candidate.OperandColumnRefs = nil
	result := literalOnlyResult()

	proof := proveBuiltinSemantics(AnalysisProfileMySQL57, "mysql", []EffectCandidate{candidate}, result, result.Requirements, registry)
	if proof.decision != builtinSemanticAllProven {
		t.Fatalf("decision = %q, want all_proven", proof.decision)
	}
}

func TestBuiltinSemanticGateway_FixedArityRejectsShapeMismatches(t *testing.T) {
	t.Parallel()

	registry := fixedManifestRegistry(t, BuiltinSemanticEntry{
		Dialect:      "mysql",
		Profile:      AnalysisProfileMySQL57,
		Name:         "coalesce",
		CallClass:    BuiltinSemanticScalar,
		Arity:        2,
		OperandKinds: []string{"const", "column"},
	})
	cases := []struct {
		name      string
		candidate EffectCandidate
	}{
		{
			name:      "wrong arity",
			candidate: mixedConstCandidate([]string{"const"}, 1),
		},
		{
			name:      "wrong position",
			candidate: mixedConstCandidate([]string{"column", "const"}, 2),
		},
		{
			name:      "expression operand",
			candidate: mixedConstCandidate([]string{"const", "expr"}, 2),
		},
		{
			name:      "parameter operand",
			candidate: mixedConstCandidate([]string{"const", "param"}, 2),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			candidate := tc.candidate
			result := mixedConstResult()
			proof := proveBuiltinSemantics(AnalysisProfileMySQL57, "mysql", []EffectCandidate{candidate}, result, result.Requirements, registry)
			if proof.decision == builtinSemanticAllProven {
				t.Fatalf("shape mismatch was proven: %+v", candidate)
			}
		})
	}
}

func TestBuiltinSemanticGateway_PostgreSQLMixedConstNotProven(t *testing.T) {
	t.Parallel()
	registry := mixedConstRegistry(t)
	candidate := mixedConstCandidate([]string{"column", "const"}, 2)
	result := mixedConstResult()

	proof := proveBuiltinSemantics(AnalysisProfileMySQL57, "postgresql", []EffectCandidate{candidate}, result, result.Requirements, registry)
	if proof.decision == builtinSemanticAllProven {
		t.Fatal("postgresql dialect was proven by mysql/tidb gateway")
	}
}

func TestBuiltinSemanticGateway_RejectsAdversarialCandidates(t *testing.T) {
	manifest := builtinTestManifest(t)
	registry, err := newBuiltinSemanticRegistry(map[AnalysisProfile]*BuiltinSemanticManifest{
		AnalysisProfileMySQL57: manifest,
	})
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}
	result := builtinTestResult()
	cases := map[string]EffectCandidate{
		"foreign kind":         func() EffectCandidate { c := builtinTestCandidate(); c.Kind = EffectCandidateCast; return c }(),
		"unknown kind":         func() EffectCandidate { c := builtinTestCandidate(); c.Kind = EffectCandidateUnknown; return c }(),
		"quoted":               func() EffectCandidate { c := builtinTestCandidate(); c.IsQuoted = true; return c }(),
		"qualified":            func() EffectCandidate { c := builtinTestCandidate(); c.ExplicitSchema = true; return c }(),
		"unqualified relation": func() EffectCandidate { c := builtinTestCandidate(); c.UnqualifiedRelation = true; return c }(),
		"ambiguous":            func() EffectCandidate { c := builtinTestCandidate(); c.Ambiguous = true; return c }(),
		"generic parser":       func() EffectCandidate { c := builtinTestCandidate(); c.ParserClassification = "generic"; return c }(),
		"noncanonical":         func() EffectCandidate { c := builtinTestCandidate(); c.Canonical = false; return c }(),
		"nested operand":       func() EffectCandidate { c := builtinTestCandidate(); c.OperandKinds = []string{"expr"}; return c }(),
		"wrong name":           func() EffectCandidate { c := builtinTestCandidate(); c.NamePath = []string{"sum"}; return c }(),
		"wrong arity":          func() EffectCandidate { c := builtinTestCandidate(); c.Arity = 1; return c }(),
		"cast target":          func() EffectCandidate { c := builtinTestCandidate(); c.TargetTypePath = []string{"decimal"}; return c }(),
	}
	for name, candidate := range cases {
		candidate.Ordinal = 0
		proof := proveBuiltinSemantics(AnalysisProfileMySQL57, "mysql", []EffectCandidate{candidate}, result, result.Requirements, registry)
		if proof.decision == builtinSemanticAllProven {
			t.Errorf("%s candidate was proven: %+v", name, candidate)
		}
	}
	duplicate := builtinTestCandidate()
	duplicate.Ordinal = 0
	if proof := proveBuiltinSemantics(AnalysisProfileMySQL57, "mysql", []EffectCandidate{builtinTestCandidate(), duplicate}, result, result.Requirements, registry); proof.decision == builtinSemanticAllProven {
		t.Fatal("duplicate ordinals were proven")
	}
}

func TestBuiltinSemanticGateway_RejectsOrdinalGapsAndMixedCandidates(t *testing.T) {
	manifest := builtinTestManifest(t)
	registry, err := newBuiltinSemanticRegistry(map[AnalysisProfile]*BuiltinSemanticManifest{
		AnalysisProfileMySQL57: manifest,
	})
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}
	result := builtinTestResult()
	first := builtinTestCandidate()
	second := builtinTestCandidate()
	second.Ordinal = 2
	if proof := proveBuiltinSemantics(AnalysisProfileMySQL57, "mysql", []EffectCandidate{first, second}, result, result.Requirements, registry); proof.decision == builtinSemanticAllProven {
		t.Fatal("ordinal gap was proven")
	}

	unknown := builtinTestCandidate()
	unknown.Ordinal = 1
	unknown.NamePath = []string{"unknown_fn"}
	proof := proveBuiltinSemantics(AnalysisProfileMySQL57, "mysql", []EffectCandidate{first, unknown}, result, result.Requirements, registry)
	if proof.decision == builtinSemanticAllProven {
		t.Fatal("mixed proven and unknown candidates were promoted")
	}
}

func TestBuiltinSemanticGateway_RejectsProfileDialectAndPhysicalGaps(t *testing.T) {
	manifest := builtinTestManifest(t)
	registry, err := newBuiltinSemanticRegistry(map[AnalysisProfile]*BuiltinSemanticManifest{
		AnalysisProfileMySQL57: manifest,
	})
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}
	candidate := []EffectCandidate{builtinTestCandidate()}
	result := builtinTestResult()
	cases := []struct {
		name    string
		profile AnalysisProfile
		dialect string
		mutate  func(*domain.Result, *[]domain.Requirement)
	}{
		{name: "empty profile", profile: AnalysisProfileEmpty, dialect: "mysql"},
		{name: "wrong dialect", profile: AnalysisProfileMySQL57, dialect: "tidb"},
		{name: "projection only", profile: AnalysisProfileMySQL57, dialect: "mysql", mutate: func(r *domain.Result, _ *[]domain.Requirement) { r.Mode = domain.ModeProjectionOnly }},
		{name: "unresolved metadata", profile: AnalysisProfileMySQL57, dialect: "mysql", mutate: func(r *domain.Result, _ *[]domain.Requirement) {
			r.Unresolved = []domain.Unresolved{{Reference: "missing", Reason: domain.ReasonSchemaUnavailable}}
		}},
		{name: "view", profile: AnalysisProfileMySQL57, dialect: "mysql", mutate: func(r *domain.Result, _ *[]domain.Requirement) { r.Relations[0].Kind = domain.RelationView }},
		{name: "cte", profile: AnalysisProfileMySQL57, dialect: "mysql", mutate: func(r *domain.Result, _ *[]domain.Requirement) { r.Relations[0].Kind = domain.RelationCTE }},
		{name: "derived", profile: AnalysisProfileMySQL57, dialect: "mysql", mutate: func(r *domain.Result, _ *[]domain.Requirement) { r.Relations[0].Kind = domain.RelationDerived }},
		{name: "wildcard", profile: AnalysisProfileMySQL57, dialect: "mysql", mutate: func(r *domain.Result, _ *[]domain.Requirement) { r.Outputs = []domain.OutputColumn{{Name: "*"}} }},
		{name: "unqualified relation", profile: AnalysisProfileMySQL57, dialect: "mysql", mutate: func(r *domain.Result, _ *[]domain.Requirement) { r.Relations[0].Schema = "" }},
		{name: "missing table requirement", profile: AnalysisProfileMySQL57, dialect: "mysql", mutate: func(_ *domain.Result, reqs *[]domain.Requirement) { *reqs = nil }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			current := result
			reqs := append([]domain.Requirement(nil), result.Requirements...)
			if tc.mutate != nil {
				tc.mutate(&current, &reqs)
			}
			proof := proveBuiltinSemantics(tc.profile, tc.dialect, candidate, current, reqs, registry)
			if proof.decision == builtinSemanticAllProven {
				t.Fatal("incomplete proof was promoted")
			}
		})
	}
}

func TestBuiltinSemanticGateway_RequiresWindowDependencies(t *testing.T) {
	manifest, err := NewBuiltinSemanticManifest([]BuiltinSemanticEntry{{
		Dialect:              "mysql",
		Profile:              AnalysisProfileMySQL57,
		Name:                 "row_number",
		CallClass:            BuiltinSemanticWindow,
		AllowWindowPartition: true,
		AllowWindowOrder:     true,
	}})
	if err != nil {
		t.Fatalf("new manifest: %v", err)
	}
	registry, err := newBuiltinSemanticRegistry(map[AnalysisProfile]*BuiltinSemanticManifest{AnalysisProfileMySQL57: manifest})
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}
	candidate := EffectCandidate{
		Kind:                      EffectCandidateFunction,
		Ordinal:                   0,
		NamePath:                  []string{"row_number"},
		OriginalNamePath:          []string{"ROW_NUMBER"},
		Canonical:                 true,
		ParserClassification:      "window",
		HasWindow:                 true,
		HasWindowPartition:        true,
		HasWindowOrder:            true,
		WindowPartitionKinds:      []string{"column"},
		WindowOrderKinds:          []string{"column"},
		WindowPartitionColumnRefs: []OperandColumnRef{{Schema: "app", Table: "users", Column: "dept"}},
		WindowOrderColumnRefs:     []OperandColumnRef{{Schema: "app", Table: "users", Column: "id"}},
	}
	result := domain.Result{
		Dialect: "mysql", Mode: domain.ModeStrict, ReadClassification: domain.Indeterminate,
		Relations: []domain.RelationReference{{Schema: "app", Name: "users", Kind: domain.RelationTable, PermissionRequired: true}},
		ReferencedColumns: []domain.ColumnReference{
			{Schema: "app", Table: "users", Column: "dept"},
			{Schema: "app", Table: "users", Column: "id"},
		},
		Requirements: []domain.Requirement{
			{Object: "app.users", Privilege: "read_table"},
			{Object: "app.users.dept", Privilege: "read_column"},
			{Object: "app.users.id", Privilege: "read_column"},
		},
	}
	proof := proveBuiltinSemantics(AnalysisProfileMySQL57, "mysql", []EffectCandidate{candidate}, result, result.Requirements, registry)
	if proof.decision != builtinSemanticAllProven {
		t.Fatalf("window proof = %q, want all_proven", proof.decision)
	}
	result.Requirements = result.Requirements[:2]
	proof = proveBuiltinSemantics(AnalysisProfileMySQL57, "mysql", []EffectCandidate{candidate}, result, result.Requirements, registry)
	if proof.decision == builtinSemanticAllProven {
		t.Fatal("window proof ignored missing physical dependency")
	}
}

func TestBuiltinSemanticGateway_RejectsMalformedFrameFacts(t *testing.T) {
	manifest, err := NewBuiltinSemanticManifest([]BuiltinSemanticEntry{{
		Dialect: "mysql", Profile: AnalysisProfileMySQL57, Name: "row_number",
		CallClass: BuiltinSemanticWindow, AllowFrame: true,
	}})
	if err != nil {
		t.Fatalf("new manifest: %v", err)
	}
	registry, err := newBuiltinSemanticRegistry(map[AnalysisProfile]*BuiltinSemanticManifest{AnalysisProfileMySQL57: manifest})
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}
	candidate := builtinTestCandidate()
	candidate.NamePath = []string{"row_number"}
	candidate.OriginalNamePath = []string{"ROW_NUMBER"}
	candidate.ParserClassification = "window"
	candidate.IsAggregate = false
	candidate.HasWindow = true
	candidate.HasFrame = true
	for _, frameKinds := range [][]string{nil, {"const"}} {
		candidate.WindowFrameKinds = frameKinds
		if proof := proveBuiltinSemantics(AnalysisProfileMySQL57, "mysql", []EffectCandidate{candidate}, builtinTestResult(), builtinTestResult().Requirements, registry); proof.decision == builtinSemanticAllProven {
			t.Fatalf("malformed frame facts were proven: %v", frameKinds)
		}
	}
}

func TestBuiltinSemanticGateway_PreservesResidualReasonAfterProof(t *testing.T) {
	manifest := &builtinSemanticProofResult{decision: builtinSemanticAllProven}
	got := reclassifyAfterResolution(
		domain.Indeterminate,
		[]domain.ReasonCode{"residual_reason"},
		nil,
		true,
		"mysql",
		nil,
		manifest,
	)
	if got != domain.Indeterminate {
		t.Fatalf("classification = %q, want indeterminate", got)
	}
}

func TestBuiltinSemanticService_UsesFixtureButDefaultServiceCannotPromote(t *testing.T) {
	resolver := &builtinTestResolver{}
	fixtureService, err := newBuiltinSemanticService(resolver, mustBuiltinTestRegistry(t))
	if err != nil {
		t.Fatalf("fixture service: %v", err)
	}
	fixtureResult, err := fixtureService.Analyze(context.Background(), QueryAccessRequest{
		SQL:             "SELECT COUNT(*) FROM app.users",
		Dialect:         "mysql",
		Mode:            "strict",
		DefaultSchema:   "app",
		AnalysisProfile: AnalysisProfileMySQL57,
	})
	if err != nil {
		t.Fatalf("fixture analyze: %v", err)
	}
	if fixtureResult.DomainResult.ReadClassification != domain.ReadOnly || fixtureResult.DomainResult.Admission != domain.Admissible {
		t.Fatalf("fixture classification=%q admission=%q", fixtureResult.DomainResult.ReadClassification, fixtureResult.DomainResult.Admission)
	}
	unqualifiedResult, err := fixtureService.Analyze(context.Background(), QueryAccessRequest{
		SQL:             "SELECT COUNT(*) FROM users",
		Dialect:         "mysql",
		Mode:            "strict",
		DefaultSchema:   "app",
		AnalysisProfile: AnalysisProfileMySQL57,
	})
	if err != nil {
		t.Fatalf("unqualified analyze: %v", err)
	}
	if unqualifiedResult.DomainResult.ReadClassification != domain.Indeterminate || unqualifiedResult.DomainResult.Admission != domain.IndeterminateAdmission {
		t.Fatalf("unqualified relation promoted: classification=%q admission=%q", unqualifiedResult.DomainResult.ReadClassification, unqualifiedResult.DomainResult.Admission)
	}

	defaultResult, err := (&Service{}).Analyze(context.Background(), QueryAccessRequest{
		SQL:             "SELECT COUNT(*) FROM app.users",
		Dialect:         "mysql",
		Mode:            "strict",
		DefaultSchema:   "app",
		AnalysisProfile: AnalysisProfileMySQL57,
		SchemaResolver:  resolver,
	})
	if err != nil {
		t.Fatalf("default analyze: %v", err)
	}
	if defaultResult.DomainResult.ReadClassification != domain.Indeterminate || defaultResult.DomainResult.Admission != domain.IndeterminateAdmission {
		t.Fatalf("default resolver promoted: classification=%q admission=%q", defaultResult.DomainResult.ReadClassification, defaultResult.DomainResult.Admission)
	}
}

func builtinTestManifest(t *testing.T) *BuiltinSemanticManifest {
	t.Helper()
	manifest, err := NewBuiltinSemanticManifest([]BuiltinSemanticEntry{{
		Dialect:      "mysql",
		Profile:      AnalysisProfileMySQL57,
		Name:         "count",
		CallClass:    BuiltinSemanticAggregate,
		Arity:        0,
		OperandKinds: []string{"star"},
	}})
	if err != nil {
		t.Fatalf("fixture manifest: %v", err)
	}
	return manifest
}

func mixedConstRegistry(t *testing.T) *builtinSemanticRegistry {
	t.Helper()
	manifest, err := NewBuiltinSemanticManifest([]BuiltinSemanticEntry{{
		Dialect:      "mysql",
		Profile:      AnalysisProfileMySQL57,
		Name:         "coalesce",
		CallClass:    BuiltinSemanticScalar,
		MinArity:     2,
		OperandKinds: []string{"column", "const"},
	}})
	if err != nil {
		t.Fatalf("new manifest: %v", err)
	}
	registry, err := newBuiltinSemanticRegistry(map[AnalysisProfile]*BuiltinSemanticManifest{
		AnalysisProfileMySQL57: manifest,
	})
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}
	return registry
}

func fixedManifestRegistry(t *testing.T, entry BuiltinSemanticEntry) *builtinSemanticRegistry {
	t.Helper()
	manifest, err := NewBuiltinSemanticManifest([]BuiltinSemanticEntry{entry})
	if err != nil {
		t.Fatalf("new manifest: %v", err)
	}
	registry, err := newBuiltinSemanticRegistry(map[AnalysisProfile]*BuiltinSemanticManifest{
		AnalysisProfileMySQL57: manifest,
	})
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}
	return registry
}

func mixedConstCandidate(operandKinds []string, arity int) EffectCandidate {
	return EffectCandidate{
		Kind:                 EffectCandidateFunction,
		Ordinal:              0,
		NamePath:             []string{"coalesce"},
		OriginalNamePath:     []string{"COALESCE"},
		Canonical:            true,
		ParserClassification: "generic",
		Arity:                arity,
		OperandKinds:         operandKinds,
		OperandColumnRefs:    []OperandColumnRef{{Schema: "app", Table: "users", Column: "name"}},
	}
}

func mixedConstResult() domain.Result {
	return domain.Result{
		Dialect: "mysql", Mode: domain.ModeStrict, ReadClassification: domain.Indeterminate,
		Relations: []domain.RelationReference{{Schema: "app", Name: "users", Kind: domain.RelationTable, PermissionRequired: true}},
		ReferencedColumns: []domain.ColumnReference{
			{Schema: "app", Table: "users", Column: "name"},
		},
		Requirements: []domain.Requirement{
			{Object: "app.users", Privilege: "read_table"},
			{Object: "app.users.name", Privilege: "read_column"},
		},
	}
}

func literalOnlyResult() domain.Result {
	return domain.Result{
		Dialect:            "mysql",
		Mode:               domain.ModeStrict,
		ReadClassification: domain.Indeterminate,
		Relations: []domain.RelationReference{{
			Schema: "app", Name: "users", Kind: domain.RelationTable, PermissionRequired: true,
		}},
		Requirements: []domain.Requirement{{Object: "app.users", Privilege: "read_table"}},
	}
}

func mustBuiltinTestRegistry(t *testing.T) *builtinSemanticRegistry {
	t.Helper()
	registry, err := newBuiltinSemanticRegistry(map[AnalysisProfile]*BuiltinSemanticManifest{
		AnalysisProfileMySQL57: builtinTestManifest(t),
	})
	if err != nil {
		t.Fatalf("fixture registry: %v", err)
	}
	return registry
}

func builtinTestCandidate() EffectCandidate {
	return EffectCandidate{
		Kind:                 EffectCandidateFunction,
		Ordinal:              0,
		NamePath:             []string{"count"},
		OriginalNamePath:     []string{"COUNT"},
		Canonical:            true,
		ParserClassification: "aggregate",
		Arity:                0,
		OperandKinds:         []string{"star"},
		IsAggregate:          true,
	}
}

func builtinTestResult() domain.Result {
	return domain.Result{
		Dialect:            "mysql",
		Mode:               domain.ModeStrict,
		ReadClassification: domain.Indeterminate,
		Admission:          domain.IndeterminateAdmission,
		ReasonCodes:        []domain.ReasonCode{domain.ReasonFunctionEffect},
		Relations: []domain.RelationReference{{
			Schema: "app", Name: "users", Kind: domain.RelationTable, PermissionRequired: true,
		}},
		Requirements: []domain.Requirement{{Object: "app.users", Privilege: "read_table"}},
	}
}

type builtinTestResolver struct{}

func (*builtinTestResolver) ResolveRelation(context.Context, string, string, string) (RelationSchema, error) {
	return RelationSchema{Schema: "app", Name: "users", Kind: "table", Columns: []ColumnSchema{{Name: "id", Ordinal: 1}}}, nil
}
