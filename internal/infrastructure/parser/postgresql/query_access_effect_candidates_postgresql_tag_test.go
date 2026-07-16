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
