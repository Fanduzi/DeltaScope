//go:build postgresql

// Package postgresql tests internal effect-candidate extraction (T5).
// input: PostgreSQL SQL with operator/function/cast effects
// output: ordered EffectCandidate facts + unchanged unproven_* reason codes
// pos: parser coverage for internal candidates (not public Result fields)
// note: if this file changes, update this header and module README.md.
package postgresql

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestExtractQueryAccess_EffectCandidates_CoreKinds(t *testing.T) {
	t.Parallel()
	e := &QueryAccessExtractor{}

	cases := []struct {
		name       string
		sql        string
		wantKind   EffectCandidateKind
		wantName0  string // first name path segment when applicable
		wantAgg    bool
		wantWindow bool
		wantFilter bool
		minArity   int
	}{
		{name: "comparison", sql: "SELECT id FROM users WHERE id = 1", wantKind: EffectCandidateOperator, wantName0: "=", minArity: 2},
		// COUNT(*) sets AggStar on the raw FuncCall; COUNT(col) may only be
		// distinguishable as aggregate after catalog resolve (prokind) — T5 records
		// function candidate only; IsAggregate is structural, not name-based.
		{name: "count_star", sql: "SELECT COUNT(*) FROM users", wantKind: EffectCandidateFunction, wantName0: "count", wantAgg: true, minArity: 0},
		{name: "count_column", sql: "SELECT COUNT(id) FROM users", wantKind: EffectCandidateFunction, wantName0: "count", wantAgg: false, minArity: 1},
		{name: "plain_function", sql: "SELECT length(name) FROM users", wantKind: EffectCandidateFunction, wantName0: "length", minArity: 1},
		{name: "cast", sql: "SELECT id::text FROM users", wantKind: EffectCandidateCast, minArity: 1},
		{name: "agg_filter", sql: "SELECT count(*) FILTER (WHERE length(name) > 0) FROM users", wantKind: EffectCandidateFunction, wantName0: "count", wantAgg: true, wantFilter: true},
		{name: "window", sql: "SELECT row_number() OVER (PARTITION BY length(name)) FROM users", wantKind: EffectCandidateFunction, wantName0: "row_number", wantWindow: true},
		{name: "current_setting", sql: "SELECT current_setting('search_path')", wantKind: EffectCandidateFunction, wantName0: "current_setting"},
		{name: "udf_like", sql: "SELECT my_udf(id) FROM users", wantKind: EffectCandidateFunction, wantName0: "my_udf"},
		{name: "schema_qualified", sql: "SELECT pg_catalog.length(name) FROM users", wantKind: EffectCandidateFunction, wantName0: "pg_catalog"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			facts, err := e.ExtractQueryAccess(context.Background(), tc.sql, "postgresql", "public")
			if err != nil {
				t.Fatalf("extract: %v", err)
			}
			if facts.ReadClassification != "indeterminate" {
				t.Errorf("classification: got %q, want indeterminate", facts.ReadClassification)
			}
			if len(facts.EffectCandidates) == 0 {
				t.Fatalf("expected candidates, got none")
			}
			found := false
			for _, c := range facts.EffectCandidates {
				if c.Kind != tc.wantKind {
					continue
				}
				if tc.wantName0 != "" {
					if len(c.NamePath) == 0 || !strings.EqualFold(c.NamePath[0], tc.wantName0) {
						// schema-qualified may start with schema
						if !(len(c.NamePath) > 1 && strings.EqualFold(c.NamePath[0], tc.wantName0)) {
							continue
						}
					}
				}
				if tc.wantAgg && !c.IsAggregate {
					continue
				}
				if tc.wantWindow && !c.HasWindow {
					continue
				}
				if tc.wantFilter && !c.HasFilter {
					continue
				}
				if c.Arity < tc.minArity {
					t.Errorf("arity: got %d, want >= %d for candidate %+v", c.Arity, tc.minArity, c)
				}
				found = true
				// No literal values stored in operand kinds.
				for _, k := range c.OperandKinds {
					if string(k) == "1" || string(k) == "search_path" || strings.Contains(string(k), "'") {
						t.Errorf("operand kind leaked value-like text: %q", k)
					}
				}
				break
			}
			if !found {
				t.Fatalf("did not find expected candidate kind=%s name0=%q in %+v", tc.wantKind, tc.wantName0, facts.EffectCandidates)
			}
			// Public reason codes must remain bounded unproven_* only.
			for _, rc := range facts.ReasonCodes {
				if !strings.HasPrefix(rc, "unproven_") {
					t.Errorf("reason must be unproven_*, got %q", rc)
				}
				if strings.Contains(rc, tc.wantName0) && tc.wantName0 != "" {
					t.Errorf("reason must not embed effect name %q: %q", tc.wantName0, rc)
				}
			}
		})
	}
}

func TestExtractQueryAccess_EffectCandidates_AggregateOperandsAndExclusions(t *testing.T) {
	t.Parallel()
	e := &QueryAccessExtractor{}

	tests := []struct {
		name       string
		sql        string
		wantName   string
		wantArity  int
		wantKinds  []OperandKindHint
		wantColumn string
		check      func(*testing.T, EffectCandidate)
	}{
		{
			name:      "count star",
			sql:       "SELECT count(*) FROM public.users",
			wantName:  "count",
			wantArity: 0,
			wantKinds: []OperandKindHint{OperandKindStar},
		},
		{
			name:       "sum direct column",
			sql:        "SELECT sum(amount) FROM public.orders",
			wantName:   "sum",
			wantArity:  1,
			wantKinds:  []OperandKindHint{OperandKindColumn},
			wantColumn: "amount",
		},
		{
			name:       "nested expression",
			sql:        "SELECT sum(amount + 1) FROM public.orders",
			wantName:   "sum",
			wantArity:  1,
			wantKinds:  []OperandKindHint{OperandKindExpr},
			wantColumn: "",
		},
		{
			name:      "filter",
			sql:       "SELECT count(*) FILTER (WHERE true) FROM public.users",
			wantName:  "count",
			wantArity: 0,
			wantKinds: []OperandKindHint{OperandKindStar},
			check: func(t *testing.T, c EffectCandidate) {
				t.Helper()
				if !c.HasFilter {
					t.Fatal("expected HasFilter")
				}
			},
		},
		{
			name:       "distinct",
			sql:        "SELECT count(DISTINCT id) FROM public.users",
			wantName:   "count",
			wantArity:  1,
			wantKinds:  []OperandKindHint{OperandKindColumn},
			wantColumn: "id",
			check: func(t *testing.T, c EffectCandidate) {
				t.Helper()
				if !c.HasDistinct {
					t.Fatal("expected HasDistinct")
				}
			},
		},
		{
			name:       "aggregate order",
			sql:        "SELECT count(id ORDER BY id) FROM public.users",
			wantName:   "count",
			wantArity:  1,
			wantKinds:  []OperandKindHint{OperandKindColumn},
			wantColumn: "id",
			check: func(t *testing.T, c EffectCandidate) {
				t.Helper()
				if !c.HasAggOrder {
					t.Fatal("expected HasAggOrder")
				}
			},
		},
		{
			name:      "within group",
			sql:       "SELECT percentile_cont(0.5) WITHIN GROUP (ORDER BY amount) FROM public.orders",
			wantName:  "percentile_cont",
			wantArity: 1,
			wantKinds: []OperandKindHint{OperandKindConst},
			check: func(t *testing.T, c EffectCandidate) {
				t.Helper()
				if !c.HasWithinGroup || !c.HasAggOrder {
					t.Fatalf("within group flags: %+v", c)
				}
			},
		},
		{
			name:       "window frame",
			sql:        "SELECT sum(amount) OVER (ORDER BY id ROWS BETWEEN 1 PRECEDING AND CURRENT ROW) FROM public.orders",
			wantName:   "sum",
			wantArity:  1,
			wantKinds:  []OperandKindHint{OperandKindColumn},
			wantColumn: "amount",
			check: func(t *testing.T, c EffectCandidate) {
				t.Helper()
				if !c.HasWindow || !c.HasFrame {
					t.Fatalf("window frame flags: %+v", c)
				}
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			facts, err := e.ExtractQueryAccess(context.Background(), tc.sql, "postgresql", "public")
			if err != nil {
				t.Fatalf("extract: %v", err)
			}
			var got *EffectCandidate
			for i := range facts.EffectCandidates {
				candidate := &facts.EffectCandidates[i]
				if candidate.Kind == EffectCandidateFunction && len(candidate.NamePath) > 0 && strings.EqualFold(candidate.NamePath[len(candidate.NamePath)-1], tc.wantName) {
					got = candidate
					break
				}
			}
			if got == nil {
				t.Fatalf("missing %s candidate: %+v", tc.wantName, facts.EffectCandidates)
			}
			if got.Arity != tc.wantArity {
				t.Fatalf("arity: got %d, want %d", got.Arity, tc.wantArity)
			}
			if !stringSlicesEqual(toStrings(got.OperandKinds), toStrings(tc.wantKinds)) {
				t.Fatalf("operand kinds: got %v, want %v", got.OperandKinds, tc.wantKinds)
			}
			if tc.wantColumn != "" {
				if len(got.OperandColumnRefs) != 1 || got.OperandColumnRefs[0].Column != tc.wantColumn {
					t.Fatalf("operand column refs: got %+v, want column %q", got.OperandColumnRefs, tc.wantColumn)
				}
			} else if len(got.OperandColumnRefs) != 0 {
				t.Fatalf("nested expression must not claim direct column provenance: %+v", got.OperandColumnRefs)
			}
			if tc.check != nil {
				tc.check(t, *got)
			}
		})
	}
}

func TestExtractQueryAccess_EffectCandidates_AggregateOrdinalAndWindows(t *testing.T) {
	t.Parallel()
	e := &QueryAccessExtractor{}

	tests := []struct {
		name  string
		sql   string
		want  []string
		check func(*testing.T, []EffectCandidate)
	}{
		{
			name: "ordered aggregates",
			sql:  "SELECT avg(amount), min(amount), max(amount) FROM public.orders",
			want: []string{"avg", "min", "max"},
		},
		{
			name: "row number window",
			sql:  "SELECT row_number() OVER (PARTITION BY dept ORDER BY id) FROM public.employees",
			want: []string{"row_number"},
			check: func(t *testing.T, candidates []EffectCandidate) {
				t.Helper()
				if candidates[0].Arity != 0 || !candidates[0].HasWindow {
					t.Fatalf("window candidate: %+v", candidates[0])
				}
				if candidates[0].HasFrame {
					t.Fatalf("default OVER must not set HasFrame: %+v", candidates[0])
				}
			},
		},
		{
			name: "ranking windows",
			sql:  "SELECT rank() OVER (ORDER BY id), dense_rank() OVER (ORDER BY id) FROM public.employees",
			want: []string{"rank", "dense_rank"},
			check: func(t *testing.T, candidates []EffectCandidate) {
				t.Helper()
				for _, c := range candidates {
					if !c.HasWindow || c.HasFrame {
						t.Fatalf("default ranking OVER must be HasWindow without HasFrame: %+v", c)
					}
				}
			},
		},
		{
			name: "row number explicit frame",
			sql:  "SELECT row_number() OVER (PARTITION BY dept ORDER BY id ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW) FROM public.employees",
			want: []string{"row_number"},
			check: func(t *testing.T, candidates []EffectCandidate) {
				t.Helper()
				if !candidates[0].HasWindow || !candidates[0].HasFrame {
					t.Fatalf("explicit frame must set HasFrame: %+v", candidates[0])
				}
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			facts, err := e.ExtractQueryAccess(context.Background(), tc.sql, "postgresql", "public")
			if err != nil {
				t.Fatalf("extract: %v", err)
			}
			var functions []EffectCandidate
			for _, candidate := range facts.EffectCandidates {
				if candidate.Kind == EffectCandidateFunction {
					functions = append(functions, candidate)
				}
			}
			if len(functions) != len(tc.want) {
				t.Fatalf("function candidates: got %+v, want %v", functions, tc.want)
			}
			for i, want := range tc.want {
				if got := functions[i].NamePath[len(functions[i].NamePath)-1]; !strings.EqualFold(got, want) {
					t.Fatalf("candidate %d name: got %q, want %q", i, got, want)
				}
				if functions[i].Ordinal != i {
					t.Fatalf("candidate %d ordinal: got %d", i, functions[i].Ordinal)
				}
			}
			if tc.check != nil {
				tc.check(t, functions)
			}
		})
	}
}

func toStrings[K ~string](values []K) []string {
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = string(value)
	}
	return out
}

func TestExtractQueryAccess_EffectCandidates_StableOrderAndOrdinal(t *testing.T) {
	t.Parallel()
	e := &QueryAccessExtractor{}
	// Nested effects: cast of function under comparison + limit function.
	sql := "SELECT COUNT(id)::text FROM users WHERE id = 1 LIMIT length('a')"
	var first []EffectCandidate
	for i := 0; i < 5; i++ {
		facts, err := e.ExtractQueryAccess(context.Background(), sql, "postgresql", "public")
		if err != nil {
			t.Fatalf("extract: %v", err)
		}
		if i == 0 {
			first = append([]EffectCandidate(nil), facts.EffectCandidates...)
			continue
		}
		if len(facts.EffectCandidates) != len(first) {
			t.Fatalf("iteration %d: candidate count %d vs %d", i, len(facts.EffectCandidates), len(first))
		}
		for j := range first {
			a, b := facts.EffectCandidates[j], first[j]
			if a.Kind != b.Kind || a.Ordinal != b.Ordinal || a.Arity != b.Arity {
				t.Fatalf("iteration %d idx %d: unstable candidates %v vs %v", i, j, a, b)
			}
			if !stringSlicesEqual(a.NamePath, b.NamePath) {
				t.Fatalf("iteration %d idx %d: name path order unstable %v vs %v", i, j, a.NamePath, b.NamePath)
			}
		}
	}
	if len(first) < 3 {
		t.Fatalf("expected multiple candidates, got %d: %+v", len(first), first)
	}
	for i, c := range first {
		if c.Ordinal != i {
			t.Errorf("ordinal not contiguous: idx %d ordinal %d", i, c.Ordinal)
		}
	}
}

func TestExtractQueryAccess_EffectCandidates_CompleteTraversalPositions(t *testing.T) {
	t.Parallel()
	e := &QueryAccessExtractor{}
	cases := []struct {
		name string
		sql  string
	}{
		{name: "limit", sql: "SELECT id FROM users LIMIT length('a')"},
		{name: "offset", sql: "SELECT id FROM users OFFSET length('a')"},
		{name: "values", sql: "VALUES (length('a'))"},
		{name: "distinct_on", sql: "SELECT DISTINCT ON (length(name)) id FROM users"},
		{name: "order_by", sql: "SELECT id FROM users ORDER BY length(name)"},
		{name: "nested_cte", sql: "WITH c AS (SELECT id FROM users WHERE id = 1) SELECT id FROM c"},
		{name: "subquery", sql: "SELECT id FROM users WHERE id IN (SELECT length(name) FROM users)"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			facts, err := e.ExtractQueryAccess(context.Background(), tc.sql, "postgresql", "public")
			if err != nil {
				t.Fatalf("extract: %v", err)
			}
			if len(facts.EffectCandidates) == 0 {
				t.Fatalf("expected candidates for %s", tc.name)
			}
			if facts.ReadClassification != "indeterminate" {
				t.Errorf("classification: got %q", facts.ReadClassification)
			}
			if len(facts.ReasonCodes) == 0 {
				t.Errorf("expected unproven_* reasons for complete-traversal position")
			}
		})
	}
}

func TestExtractQueryAccess_EffectCandidates_BoolExprNotCandidate(t *testing.T) {
	t.Parallel()
	e := &QueryAccessExtractor{}
	facts, err := e.ExtractQueryAccess(context.Background(),
		"SELECT id FROM users WHERE active AND inactive", "postgresql", "public")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(facts.EffectCandidates) != 0 {
		t.Errorf("structural bool must not produce catalog candidates: %+v", facts.EffectCandidates)
	}
	if facts.ReadClassification != "read_only" {
		t.Errorf("classification: got %q, want read_only", facts.ReadClassification)
	}
}

func TestExtractQueryAccess_EffectCandidates_NoLiteralOrSQLStorage(t *testing.T) {
	t.Parallel()
	e := &QueryAccessExtractor{}
	sql := "SELECT current_setting('search_path'), id::text FROM users WHERE name = 'alice' LIMIT length('zz')"
	facts, err := e.ExtractQueryAccess(context.Background(), sql, "postgresql", "public")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	blob := fmt.Sprintf("%+v", facts.EffectCandidates)
	// Candidates may include name spellings (internal) but must not store literals/SQL text.
	for _, bad := range []string{"search_path", "alice", "zz", "SELECT ", "LIMIT length"} {
		if strings.Contains(blob, bad) {
			// name path may contain current_setting but not the literal argument.
			if bad == "search_path" {
				// current_setting name is OK; literal 'search_path' as stored value is not —
				// OperandKinds should not contain the string.
				for _, c := range facts.EffectCandidates {
					for _, k := range c.OperandKinds {
						if string(k) == "search_path" || strings.Contains(string(k), "search_path") {
							t.Errorf("operand kind leaked literal: %q", k)
						}
					}
				}
				continue
			}
			t.Errorf("candidate dump must not contain %q; got %s", bad, blob)
		}
	}
	// Never store type OID fields (we don't have OID field; ensure no large numeric oid-like in TargetTypePath).
	for _, c := range facts.EffectCandidates {
		for _, p := range c.TargetTypePath {
			if p == "23" || p == "25" { // int4/text OIDs must not appear as path
				t.Errorf("target type path looks like OID: %q", p)
			}
		}
	}
}

func TestExtractQueryAccess_EffectCandidates_SchemaQualifiedOperator(t *testing.T) {
	t.Parallel()
	e := &QueryAccessExtractor{}
	facts, err := e.ExtractQueryAccess(context.Background(),
		"SELECT id FROM users WHERE id OPERATOR(pg_catalog.=) 1", "postgresql", "public")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	found := false
	for _, c := range facts.EffectCandidates {
		if c.Kind != EffectCandidateOperator {
			continue
		}
		if c.ExplicitSchema && len(c.NamePath) >= 2 {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected schema-qualified operator candidate, got %+v", facts.EffectCandidates)
	}
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
