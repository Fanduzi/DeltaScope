// Package tidbparser characterizes ordered MySQL/TiDB effect candidates.
// input: function-bearing expressions across query and modifier locations
// output: deterministic internal candidates without public-result exposure
// pos: parser-level candidate closure regression coverage
package tidbparser

import (
	"context"
	"fmt"
	"testing"
)

func TestQueryAccessEffectCandidates_CountStarAndDirectColumn(t *testing.T) {
	t.Parallel()

	// Given a canonical aggregate with a star and a direct physical column.
	facts, err := (&QueryAccessExtractor{}).ExtractQueryAccess(context.Background(),
		"SELECT COUNT(*), SUM(amount) FROM orders", "mysql", "app")
	// When extraction completes.
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	// Then both aggregate candidates retain shape and resolved operand facts.
	if len(facts.EffectCandidates) != 2 {
		t.Fatalf("candidates: got %d, want 2", len(facts.EffectCandidates))
	}
	count := facts.EffectCandidates[0]
	if count.NamePath[0] != "count" || !count.IsAggregate || count.Arity != 0 {
		t.Errorf("count candidate: %+v", count)
	}
	if len(count.OperandKinds) != 1 || count.OperandKinds[0] != OperandKindStar {
		t.Errorf("count operands: %+v", count.OperandKinds)
	}
	sum := facts.EffectCandidates[1]
	if sum.NamePath[0] != "sum" || !sum.IsAggregate || sum.Arity != 1 {
		t.Errorf("sum candidate: %+v", sum)
	}
	if len(sum.OperandColumnRefs) != 1 || sum.OperandColumnRefs[0] != (OperandColumnRef{Schema: "app", Table: "orders", Column: "amount"}) {
		t.Errorf("sum column refs: %+v", sum.OperandColumnRefs)
	}
}

func TestQueryAccessEffectCandidates_WindowModifiersAndFrame(t *testing.T) {
	t.Parallel()

	// Given a ranking window with partition, order, and an explicit frame.
	facts, err := (&QueryAccessExtractor{}).ExtractQueryAccess(context.Background(),
		"SELECT ROW_NUMBER() OVER (PARTITION BY dept ORDER BY salary ROWS BETWEEN 1 PRECEDING AND CURRENT ROW) FROM employees", "mysql", "")
	// When extraction completes.
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	// Then the window candidate records every gateway-relevant modifier.
	if len(facts.EffectCandidates) != 1 {
		t.Fatalf("candidates: got %d, want 1", len(facts.EffectCandidates))
	}
	candidate := facts.EffectCandidates[0]
	if candidate.NamePath[0] != "row_number" || !candidate.HasWindow || !candidate.HasFrame {
		t.Errorf("window candidate: %+v", candidate)
	}
	if !candidate.HasWindowPartition || !candidate.HasWindowOrder {
		t.Errorf("window locations: %+v", candidate)
	}
	if len(candidate.WindowPartitionColumnRefs) != 1 || len(candidate.WindowOrderColumnRefs) != 1 {
		t.Errorf("window column refs: %+v", candidate)
	}
}

func TestQueryAccessEffectCandidates_NestedAndModifierClosure(t *testing.T) {
	t.Parallel()

	// Given a nested aggregate and a DISTINCT aggregate-local ordering modifier.
	facts, err := (&QueryAccessExtractor{}).ExtractQueryAccess(context.Background(),
		"SELECT SUM(ABS(amount)), GROUP_CONCAT(DISTINCT name ORDER BY name) FROM orders", "tidb", "")
	// When extraction completes.
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	// Then traversal is pre-order and modifiers are explicit.
	if len(facts.EffectCandidates) != 3 {
		t.Fatalf("candidates: got %d, want 3: %+v", len(facts.EffectCandidates), facts.EffectCandidates)
	}
	wantNames := []string{"sum", "abs", "group_concat"}
	for i, want := range wantNames {
		if got := facts.EffectCandidates[i].NamePath[0]; got != want {
			t.Errorf("candidate[%d] name: got %q, want %q", i, got, want)
		}
	}
	if !facts.EffectCandidates[2].HasDistinct || !facts.EffectCandidates[2].HasAggOrder {
		t.Errorf("aggregate modifiers: %+v", facts.EffectCandidates[2])
	}
}

func TestQueryAccessEffectCandidates_QualifiedQuotedAndDeterministic(t *testing.T) {
	t.Parallel()

	// Given qualified and quoted spellings plus a repeated extraction.
	sql := "SELECT app.COUNT(id), `my_func`(amount) FROM app.orders"
	first, err := (&QueryAccessExtractor{}).ExtractQueryAccess(context.Background(), sql, "mysql", "")
	// When extraction completes twice.
	if err != nil {
		t.Fatalf("first extract: %v", err)
	}
	second, err := (&QueryAccessExtractor{}).ExtractQueryAccess(context.Background(), sql, "mysql", "")
	if err != nil {
		t.Fatalf("second extract: %v", err)
	}
	// Then parser spelling/path facts and ordinals are stable and qualification is not collapsed.
	if len(first.EffectCandidates) != 2 {
		t.Fatalf("candidates: got %d, want 2", len(first.EffectCandidates))
	}
	if !first.EffectCandidates[0].ExplicitSchema || len(first.EffectCandidates[0].OriginalNamePath) != 2 || first.EffectCandidates[0].OriginalNamePath[1] != "COUNT" {
		t.Errorf("qualified candidate: %+v", first.EffectCandidates[0])
	}
	if first.EffectCandidates[1].OriginalNamePath[0] != "my_func" || !first.EffectCandidates[1].IsQuoted || !first.EffectCandidates[1].Ambiguous || first.EffectCandidates[1].Canonical {
		t.Errorf("quoted candidate: %+v", first.EffectCandidates[1])
	}
	if fmt.Sprintf("%+v", first.EffectCandidates) != fmt.Sprintf("%+v", second.EffectCandidates) {
		t.Errorf("candidate order changed: %v vs %v", first.EffectCandidates, second.EffectCandidates)
	}
	for i, candidate := range first.EffectCandidates {
		if candidate.Ordinal != i {
			t.Errorf("candidate ordinal: got %d at index %d", candidate.Ordinal, i)
		}
	}
}

func TestQueryAccessEffectCandidates_RetainRelationQualification(t *testing.T) {
	t.Parallel()

	qualified, err := (&QueryAccessExtractor{}).ExtractQueryAccess(
		context.Background(), "SELECT COUNT(*) FROM app.users", "mysql", "app",
	)
	if err != nil {
		t.Fatalf("qualified extract: %v", err)
	}
	if len(qualified.EffectCandidates) != 1 || qualified.EffectCandidates[0].UnqualifiedRelation {
		t.Fatalf("qualified relation fact: %+v", qualified.EffectCandidates)
	}

	unqualified, err := (&QueryAccessExtractor{}).ExtractQueryAccess(
		context.Background(), "SELECT COUNT(*) FROM users", "mysql", "app",
	)
	if err != nil {
		t.Fatalf("unqualified extract: %v", err)
	}
	if len(unqualified.EffectCandidates) != 1 || !unqualified.EffectCandidates[0].UnqualifiedRelation {
		t.Fatalf("unqualified relation fact: %+v", unqualified.EffectCandidates)
	}
}

func TestQueryAccessEffectCandidates_RejectNonCanonicalSpacing(t *testing.T) {
	t.Parallel()

	facts, err := (&QueryAccessExtractor{}).ExtractQueryAccess(
		context.Background(), "SELECT COUNT (id) FROM app.users", "mysql", "app",
	)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(facts.EffectCandidates) != 1 {
		t.Fatalf("candidates: got %d, want 1", len(facts.EffectCandidates))
	}
	candidate := facts.EffectCandidates[0]
	if candidate.Canonical || !candidate.Ambiguous {
		t.Fatalf("noncanonical spacing was accepted: %+v", candidate)
	}
}

func TestQueryAccessEffectCandidates_AllExpressionLocations(t *testing.T) {
	t.Parallel()

	// Given function-bearing expressions in every exposed query location.
	cases := []struct {
		name string
		sql  string
		want string
	}{
		{name: "cte", sql: "WITH c AS (SELECT MAX(id) AS id FROM inner_t) SELECT COUNT(*) FROM users", want: "max"},
		{name: "join", sql: "SELECT u.id FROM users u JOIN orders o ON LOWER(u.name) = LOWER(o.name)", want: "lower"},
		{name: "where", sql: "SELECT id FROM users WHERE ABS(id) > 0", want: "abs"},
		{name: "group", sql: "SELECT LOWER(name) FROM users GROUP BY LOWER(name)", want: "lower"},
		{name: "having", sql: "SELECT dept FROM users GROUP BY dept HAVING COUNT(*) > 1", want: "count"},
		{name: "order", sql: "SELECT id FROM users ORDER BY MAX(id)", want: "max"},
		{name: "limit_offset", sql: "SELECT id FROM users LIMIT 1, 0", want: ""},
		{name: "scalar", sql: "SELECT (SELECT MAX(id) FROM orders) FROM users", want: "max"},
		{name: "set", sql: "SELECT MAX(id) FROM users UNION SELECT MAX(id) FROM orders", want: "max"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			facts, err := (&QueryAccessExtractor{}).ExtractQueryAccess(context.Background(), tc.sql, "mysql", "")
			// When extraction completes.
			if err != nil {
				t.Fatalf("extract: %v", err)
			}
			// Then the function in that expression location is retained when the AST exposes one.
			if tc.want != "" && !candidateHasName(facts.EffectCandidates, tc.want) {
				t.Errorf("missing candidate %q in %+v", tc.want, facts.EffectCandidates)
			}
			assertCandidateOrdinals(t, facts.EffectCandidates)
		})
	}
}

func TestQueryAccessEffectCandidates_UnsupportedTraversalMarker(t *testing.T) {
	t.Parallel()

	// Given a parser-supported function-like expression without a native call node.
	facts, err := (&QueryAccessExtractor{}).ExtractQueryAccess(context.Background(),
		"SELECT MATCH(title) AGAINST ('term') FROM articles", "mysql", "")
	// When extraction completes.
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	// Then traversal remains explicit and fail-closed internally.
	found := false
	for _, candidate := range facts.EffectCandidates {
		if candidate.Kind == EffectCandidateUnknown && candidate.ParserClassification == "unsupported_traversal" {
			found = true
		}
	}
	if !found {
		t.Errorf("missing unsupported traversal marker: %+v", facts.EffectCandidates)
	}
}

func candidateHasName(candidates []EffectCandidate, want string) bool {
	for _, candidate := range candidates {
		if len(candidate.NamePath) > 0 && candidate.NamePath[len(candidate.NamePath)-1] == want {
			return true
		}
	}
	return false
}

func assertCandidateOrdinals(t *testing.T, candidates []EffectCandidate) {
	t.Helper()
	for i, candidate := range candidates {
		if candidate.Ordinal != i {
			t.Errorf("candidate[%d] ordinal: got %d", i, candidate.Ordinal)
		}
	}
}
