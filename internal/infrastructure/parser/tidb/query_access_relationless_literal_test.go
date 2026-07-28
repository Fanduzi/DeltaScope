package tidbparser

import (
	"context"
	"testing"
)

func TestQueryAccessRelationlessLiteralAdmittedShapes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		sql       string
		funcName  string
		wantClass string
		wantArity int
		wantKinds []OperandKindHint
	}{
		{"lower_const", "SELECT LOWER('x')", "lower", "keyword", 1, []OperandKindHint{OperandKindConst}},
		{"upper_const", "SELECT UPPER('x')", "upper", "keyword", 1, []OperandKindHint{OperandKindConst}},
		{"length_const", "SELECT LENGTH('x')", "length", "keyword", 1, []OperandKindHint{OperandKindConst}},
		{"char_length_const", "SELECT CHAR_LENGTH('x')", "char_length", "keyword", 1, []OperandKindHint{OperandKindConst}},
		{"abs_const", "SELECT ABS(42)", "abs", "keyword", 1, []OperandKindHint{OperandKindConst}},
		{"ceil_const", "SELECT CEIL(42)", "ceil", "keyword", 1, []OperandKindHint{OperandKindConst}},
		{"ceiling_const", "SELECT CEILING(42)", "ceiling", "keyword", 1, []OperandKindHint{OperandKindConst}},
		{"floor_const", "SELECT FLOOR(42)", "floor", "keyword", 1, []OperandKindHint{OperandKindConst}},
		{"count_literal", "SELECT COUNT(1)", "count", "aggregate", 1, []OperandKindHint{OperandKindConst}},
		{"coalesce_const_const", "SELECT COALESCE('x', 'y')", "coalesce", "keyword", 2, []OperandKindHint{OperandKindConst, OperandKindConst}},
		{"nullif_const_const", "SELECT NULLIF('x', 'y')", "nullif", "keyword", 2, []OperandKindHint{OperandKindConst, OperandKindConst}},
		{"ifnull_const_const", "SELECT IFNULL('x', 'y')", "ifnull", "keyword", 2, []OperandKindHint{OperandKindConst, OperandKindConst}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			facts, err := (&QueryAccessExtractor{}).ExtractQueryAccess(context.Background(), tc.sql, "mysql", "app")
			if err != nil {
				t.Fatalf("extract: %v", err)
			}
			if len(facts.Relations) != 0 {
				t.Fatalf("Relations = %+v, want empty", facts.Relations)
			}
			if len(facts.ColumnReferences) != 0 {
				t.Fatalf("ColumnReferences = %+v, want empty", facts.ColumnReferences)
			}
			if len(facts.Unresolved) != 0 {
				t.Fatalf("Unresolved = %+v, want empty", facts.Unresolved)
			}
			if facts.ReadClassification != "indeterminate" {
				t.Fatalf("ReadClassification = %q, want indeterminate before online proof", facts.ReadClassification)
			}
			if !containsReason(facts.ReasonCodes, "function_call") {
				t.Fatalf("ReasonCodes = %v, want function_call", facts.ReasonCodes)
			}
			if len(facts.EffectCandidates) != 1 {
				t.Fatalf("EffectCandidates = %+v, want exactly one", facts.EffectCandidates)
			}
			c := facts.EffectCandidates[0]
			if c.Kind != EffectCandidateFunction {
				t.Errorf("Kind = %q, want function", c.Kind)
			}
			if c.Ordinal != 0 {
				t.Errorf("Ordinal = %d, want 0", c.Ordinal)
			}
			if len(c.NamePath) != 1 || c.NamePath[0] != tc.funcName {
				t.Errorf("NamePath = %v, want [%s]", c.NamePath, tc.funcName)
			}
			if !c.Canonical || c.IsQuoted || c.ExplicitSchema || c.Ambiguous || c.UnqualifiedRelation {
				t.Errorf("call facts canonical=%t quoted=%t schema=%t ambiguous=%t unqualified=%t", c.Canonical, c.IsQuoted, c.ExplicitSchema, c.Ambiguous, c.UnqualifiedRelation)
			}
			if c.ParserClassification != tc.wantClass {
				t.Errorf("ParserClassification = %q, want %q", c.ParserClassification, tc.wantClass)
			}
			if c.Arity != tc.wantArity {
				t.Errorf("Arity = %d, want %d", c.Arity, tc.wantArity)
			}
			if len(c.OperandKinds) != len(tc.wantKinds) {
				t.Fatalf("OperandKinds = %v, want %v", c.OperandKinds, tc.wantKinds)
			}
			for i, kind := range c.OperandKinds {
				if kind != tc.wantKinds[i] {
					t.Errorf("OperandKinds[%d] = %q, want %q", i, kind, tc.wantKinds[i])
				}
			}
			if len(c.OperandColumnRefs) != 0 {
				t.Errorf("OperandColumnRefs = %+v, want empty", c.OperandColumnRefs)
			}
			if c.HasWindow || c.HasFilter || c.HasDistinct || c.HasAggOrder || c.HasWithinGroup || c.HasFrame || c.HasNamedWindow || c.HasWindowPartition || c.HasWindowOrder {
				t.Errorf("unexpected modifiers on %+v", c)
			}
			if len(c.WindowPartitionKinds) != 0 || len(c.WindowOrderKinds) != 0 || len(c.WindowFrameKinds) != 0 || len(c.WindowPartitionColumnRefs) != 0 || len(c.WindowOrderColumnRefs) != 0 || len(c.TargetTypePath) != 0 {
				t.Errorf("unexpected window/cast facts on %+v", c)
			}
		})
	}
}

func TestQueryAccessRelationlessRejectedShapes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		sql  string
	}{
		{"parameter_operand", "SELECT LOWER(?)"},
		{"cast_call", "SELECT CAST('x' AS CHAR)"},
		{"nested_function", "SELECT LOWER(UPPER('x'))"},
		{"unknown_function", "SELECT MYSTERY_FN('x')"},
		{"three_arg_coalesce", "SELECT COALESCE('x', 'y', 'z')"},
		{"column_operand", "SELECT LOWER(name)"},
		{"qualified_call", "SELECT app.lower('x')"},
		{"quoted_call", "SELECT `lower`('x')"},
		{"noncanonical_call", "SELECT LOWER /* c */ ('x')"},
		{"count_star", "SELECT COUNT(*)"},
		{"subquery_operand", "SELECT LOWER((SELECT 'x'))"},
		{"operator_inside_call", "SELECT ABS(1 + 2)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			facts, err := (&QueryAccessExtractor{}).ExtractQueryAccess(context.Background(), tc.sql, "mysql", "app")
			if err != nil {
				t.Fatalf("extract: %v", err)
			}
			// The gateway only promotes when every candidate matches the
			// manifest, so one non-admissible candidate fails the statement.
			allAdmissible := len(facts.EffectCandidates) > 0
			for _, c := range facts.EffectCandidates {
				if !relationlessLiteralAdmissibleCandidateShape(c) {
					allAdmissible = false
					break
				}
			}
			if allAdmissible {
				t.Fatalf("every candidate presents a relationless literal-only admissible shape: %+v", facts.EffectCandidates)
			}
		})
	}
}

// relationlessLiteralAdmissibleCandidateShape mirrors the exact manifest-backed
// shapes admitted by the relationless literal-only proposal. Rejected inputs
// must never emit a candidate that satisfies it.
func relationlessLiteralAdmissibleCandidateShape(c EffectCandidate) bool {
	if c.Kind != EffectCandidateFunction || !c.Canonical || c.IsQuoted || c.ExplicitSchema || c.Ambiguous || len(c.NamePath) != 1 || len(c.TargetTypePath) != 0 {
		return false
	}
	if len(c.OperandColumnRefs) != 0 || len(c.WindowPartitionColumnRefs) != 0 || len(c.WindowOrderColumnRefs) != 0 {
		return false
	}
	for _, kind := range c.OperandKinds {
		if kind != OperandKindConst {
			return false
		}
	}
	name := c.NamePath[0]
	switch name {
	case "lower", "upper", "length", "char_length", "abs", "ceil", "ceiling", "floor":
		return c.Arity == 1 && len(c.OperandKinds) == 1
	case "count":
		return c.Arity == 1 && len(c.OperandKinds) == 1
	case "coalesce", "nullif", "ifnull":
		return c.Arity == 2 && len(c.OperandKinds) == 2
	default:
		return false
	}
}

func TestQueryAccessRelationlessCandidateFreeSelectOneUnchanged(t *testing.T) {
	t.Parallel()
	facts, err := (&QueryAccessExtractor{}).ExtractQueryAccess(context.Background(), "SELECT 1", "mysql", "app")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if facts.ReadClassification != "read_only" {
		t.Fatalf("ReadClassification = %q, want read_only", facts.ReadClassification)
	}
	if len(facts.EffectCandidates) != 0 {
		t.Fatalf("EffectCandidates = %+v, want none", facts.EffectCandidates)
	}
	if len(facts.Relations) != 0 || len(facts.ColumnReferences) != 0 || len(facts.Unresolved) != 0 || len(facts.ReasonCodes) != 0 {
		t.Fatalf("SELECT 1 facts changed: %+v", facts)
	}
}

func containsReason(reasons []string, want string) bool {
	for _, reason := range reasons {
		if reason == want {
			return true
		}
	}
	return false
}
