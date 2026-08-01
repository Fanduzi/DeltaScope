//go:build postgresql

// Package postgresql tests scope resolution and operand column provenance.
// input: PostgreSQL SQL with JOINs, CTEs, derived tables, and column references
// output: ColumnRefResult provenance and EffectCandidate OperandColumnRefs
// pos: parser coverage for scope resolution and provenance collection
// note: if this file changes, update this header and module README.md.
package postgresql

import (
	"testing"
)

func TestBuildSelectScope_AliasedJoin(t *testing.T) {
	t.Parallel()
	sql := "SELECT u.id FROM users u JOIN orders o ON u.id = o.user_id"
	facts := extractQA(t, sql)
	if len(facts.EffectCandidates) == 0 {
		t.Fatal("expected operator candidate for u.id = o.user_id")
	}
	var opCand *EffectCandidate
	for i := range facts.EffectCandidates {
		if facts.EffectCandidates[i].Kind == EffectCandidateOperator {
			opCand = &facts.EffectCandidates[i]
			break
		}
	}
	if opCand == nil {
		t.Fatal("expected operator candidate")
	}
	if len(opCand.OperandColumnRefs) != 2 {
		t.Fatalf("expected 2 operand column refs, got %d: %+v", len(opCand.OperandColumnRefs), opCand.OperandColumnRefs)
	}
	ref0 := opCand.OperandColumnRefs[0]
	if ref0.Table != "users" || ref0.Column != "id" {
		t.Errorf("left operand: got table=%q column=%q, want users.id", ref0.Table, ref0.Column)
	}
	ref1 := opCand.OperandColumnRefs[1]
	if ref1.Table != "orders" || ref1.Column != "user_id" {
		t.Errorf("right operand: got table=%q column=%q, want orders.user_id", ref1.Table, ref1.Column)
	}
}

func TestBuildSelectScope_CTERejection(t *testing.T) {
	t.Parallel()
	sql := "WITH cte AS (SELECT id FROM users) SELECT cte.id FROM cte WHERE cte.id = 1"
	facts := extractQA(t, sql)
	if len(facts.EffectCandidates) == 0 {
		t.Fatal("expected operator candidate for cte.id = 1")
	}
	var opCand *EffectCandidate
	for i := range facts.EffectCandidates {
		if facts.EffectCandidates[i].Kind == EffectCandidateOperator {
			opCand = &facts.EffectCandidates[i]
			break
		}
	}
	if opCand == nil {
		t.Fatal("expected operator candidate")
	}
	if len(opCand.OperandColumnRefs) != 0 {
		t.Errorf("CTE columns must not produce operand column refs, got %+v", opCand.OperandColumnRefs)
	}
}

func TestBuildSelectScope_DerivedTableRejection(t *testing.T) {
	t.Parallel()
	sql := "SELECT d.id FROM (SELECT id FROM users) d WHERE d.id = 1"
	facts := extractQA(t, sql)
	if len(facts.EffectCandidates) == 0 {
		t.Fatal("expected operator candidate for d.id = 1")
	}
	var opCand *EffectCandidate
	for i := range facts.EffectCandidates {
		if facts.EffectCandidates[i].Kind == EffectCandidateOperator {
			opCand = &facts.EffectCandidates[i]
			break
		}
	}
	if opCand == nil {
		t.Fatal("expected operator candidate")
	}
	if len(opCand.OperandColumnRefs) != 0 {
		t.Errorf("derived table columns must not produce operand column refs, got %+v", opCand.OperandColumnRefs)
	}
}

func TestBuildSelectScope_AmbiguousColumnRejection(t *testing.T) {
	t.Parallel()
	sql := "SELECT id FROM users, orders WHERE id = 1"
	facts := extractQA(t, sql)
	if len(facts.EffectCandidates) == 0 {
		t.Fatal("expected operator candidate for id = 1")
	}
	var opCand *EffectCandidate
	for i := range facts.EffectCandidates {
		if facts.EffectCandidates[i].Kind == EffectCandidateOperator {
			opCand = &facts.EffectCandidates[i]
			break
		}
	}
	if opCand == nil {
		t.Fatal("expected operator candidate")
	}
	if len(opCand.OperandColumnRefs) != 0 {
		t.Errorf("ambiguous columns must not produce operand column refs, got %+v", opCand.OperandColumnRefs)
	}
}

func TestBuildSelectScope_ThreePartColumnResolution(t *testing.T) {
	t.Parallel()
	sql := "SELECT public.users.id FROM public.users WHERE public.users.id = 1"
	facts := extractQA(t, sql)
	if len(facts.EffectCandidates) == 0 {
		t.Fatal("expected operator candidate for public.users.id = 1")
	}
	var opCand *EffectCandidate
	for i := range facts.EffectCandidates {
		if facts.EffectCandidates[i].Kind == EffectCandidateOperator {
			opCand = &facts.EffectCandidates[i]
			break
		}
	}
	if opCand == nil {
		t.Fatal("expected operator candidate")
	}
	if len(opCand.OperandColumnRefs) != 1 {
		t.Fatalf("expected 1 operand column ref, got %d: %+v", len(opCand.OperandColumnRefs), opCand.OperandColumnRefs)
	}
	ref := opCand.OperandColumnRefs[0]
	if ref.Schema != "public" || ref.Table != "users" || ref.Column != "id" {
		t.Errorf("operand: got schema=%q table=%q column=%q, want public.users.id", ref.Schema, ref.Table, ref.Column)
	}
}

func TestBuildSelectScope_MultipleJoinScopeBindings(t *testing.T) {
	t.Parallel()
	sql := "SELECT u.id, o.id FROM users u JOIN orders o ON u.id = o.user_id JOIN items i ON o.id = i.order_id WHERE u.name = 'alice'"
	facts := extractQA(t, sql)
	if len(facts.EffectCandidates) == 0 {
		t.Fatal("expected operator candidates")
	}
	var opCand *EffectCandidate
	for i := range facts.EffectCandidates {
		if facts.EffectCandidates[i].Kind == EffectCandidateOperator {
			for _, rc := range facts.EffectCandidates[i].OperandColumnRefs {
				if rc.Column == "name" {
					opCand = &facts.EffectCandidates[i]
					break
				}
			}
		}
	}
	if opCand == nil {
		t.Fatal("expected operator candidate for u.name = 'alice'")
	}
	if len(opCand.OperandColumnRefs) != 1 {
		t.Fatalf("expected 1 operand column ref, got %d: %+v", len(opCand.OperandColumnRefs), opCand.OperandColumnRefs)
	}
	ref := opCand.OperandColumnRefs[0]
	if ref.Table != "users" || ref.Column != "name" {
		t.Errorf("operand: got table=%q column=%q, want users.name", ref.Table, ref.Column)
	}
}

func TestBuildSelectScope_DefaultSchemaInBindings(t *testing.T) {
	t.Parallel()
	sql := "SELECT id FROM users WHERE id = 1"
	facts := extractQA(t, sql)
	if len(facts.EffectCandidates) == 0 {
		t.Fatal("expected operator candidate for id = 1")
	}
	var opCand *EffectCandidate
	for i := range facts.EffectCandidates {
		if facts.EffectCandidates[i].Kind == EffectCandidateOperator {
			opCand = &facts.EffectCandidates[i]
			break
		}
	}
	if opCand == nil {
		t.Fatal("expected operator candidate")
	}
	if len(opCand.OperandColumnRefs) != 1 {
		t.Fatalf("expected 1 operand column ref, got %d: %+v", len(opCand.OperandColumnRefs), opCand.OperandColumnRefs)
	}
	ref := opCand.OperandColumnRefs[0]
	if ref.Schema != "" {
		t.Errorf("operand schema: got %q, want empty (unqualified)", ref.Schema)
	}
	if ref.Table != "users" || ref.Column != "id" {
		t.Errorf("operand: got table=%q column=%q, want users.id", ref.Table, ref.Column)
	}
}

func TestExtractQueryAccess_ScopeProvenance_NestedSubquery(t *testing.T) {
	t.Parallel()
	sql := "SELECT id FROM users WHERE id IN (SELECT user_id FROM orders WHERE amount > 100)"
	facts := extractQA(t, sql)
	if len(facts.EffectCandidates) == 0 {
		t.Fatal("expected operator candidates")
	}
	foundInner := false
	for _, c := range facts.EffectCandidates {
		if c.Kind != EffectCandidateOperator {
			continue
		}
		for _, rc := range c.OperandColumnRefs {
			if rc.Table == "orders" && rc.Column == "amount" {
				foundInner = true
			}
		}
	}
	if !foundInner {
		t.Error("expected inner query orders.amount in operand column refs")
	}
}

func TestExtractQueryAccess_ScopeProvenance_ExistingCandidatesUnchanged(t *testing.T) {
	t.Parallel()
	sql := "SELECT id FROM users WHERE id = 1"
	facts := extractQA(t, sql)
	if len(facts.EffectCandidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(facts.EffectCandidates))
	}
	c := facts.EffectCandidates[0]
	if c.Kind != EffectCandidateOperator {
		t.Errorf("kind: got %q, want operator", c.Kind)
	}
	if c.Arity != 2 {
		t.Errorf("arity: got %d, want 2", c.Arity)
	}
	if len(c.OperandKinds) != 2 {
		t.Errorf("operand kinds: got %d, want 2", len(c.OperandKinds))
	}
	if c.OperandKinds[0] != OperandKindColumn {
		t.Errorf("left operand kind: got %q, want column", c.OperandKinds[0])
	}
	if c.OperandKinds[1] != OperandKindIntegerOne {
		t.Errorf("right operand kind: got %q, want integer_one", c.OperandKinds[1])
	}
	if len(c.OperandColumnRefs) != 1 {
		t.Fatalf("expected 1 operand column ref, got %d", len(c.OperandColumnRefs))
	}
	if c.OperandColumnRefs[0].Table != "users" || c.OperandColumnRefs[0].Column != "id" {
		t.Errorf("operand column ref: got table=%q column=%q, want users.id",
			c.OperandColumnRefs[0].Table, c.OperandColumnRefs[0].Column)
	}
}

func TestExtractQueryAccess_CTELexicalScope_NestedSubquery(t *testing.T) {
	t.Parallel()
	sql := "WITH cte AS (SELECT id FROM users) SELECT * FROM cte WHERE id IN (SELECT id FROM cte WHERE id = 1)"
	facts := extractQA(t, sql)
	if len(facts.EffectCandidates) == 0 {
		t.Fatal("expected operator candidate for id = 1")
	}
	var opCand *EffectCandidate
	for i := range facts.EffectCandidates {
		if facts.EffectCandidates[i].Kind == EffectCandidateOperator {
			opCand = &facts.EffectCandidates[i]
			break
		}
	}
	if opCand == nil {
		t.Fatal("expected operator candidate")
	}
	if len(opCand.OperandColumnRefs) != 0 {
		t.Errorf("CTE columns in nested subquery must not produce operand column refs, got %+v", opCand.OperandColumnRefs)
	}
}

func TestExtractQueryAccess_CTELexicalScope_CTEShadowsPhysicalTable(t *testing.T) {
	t.Parallel()
	sql := "WITH users AS (SELECT id FROM orders) SELECT * FROM users WHERE id = 1"
	facts := extractQA(t, sql)
	if len(facts.EffectCandidates) == 0 {
		t.Fatal("expected operator candidate for id = 1")
	}
	var opCand *EffectCandidate
	for i := range facts.EffectCandidates {
		if facts.EffectCandidates[i].Kind == EffectCandidateOperator {
			opCand = &facts.EffectCandidates[i]
			break
		}
	}
	if opCand == nil {
		t.Fatal("expected operator candidate")
	}
	if len(opCand.OperandColumnRefs) != 0 {
		t.Errorf("CTE-sourced columns must not produce operand column refs (CTE shadows physical table), got %+v", opCand.OperandColumnRefs)
	}
}

func TestExtractQueryAccess_CTELexicalScope_NestedCTESeesOuterCTE(t *testing.T) {
	t.Parallel()
	sql := "WITH outer_cte AS (SELECT id FROM users), inner_cte AS (SELECT id FROM outer_cte) SELECT * FROM inner_cte WHERE id = 1"
	facts := extractQA(t, sql)
	if len(facts.EffectCandidates) == 0 {
		t.Fatal("expected operator candidate for id = 1")
	}
	var opCand *EffectCandidate
	for i := range facts.EffectCandidates {
		if facts.EffectCandidates[i].Kind == EffectCandidateOperator {
			opCand = &facts.EffectCandidates[i]
			break
		}
	}
	if opCand == nil {
		t.Fatal("expected operator candidate")
	}
	if len(opCand.OperandColumnRefs) != 0 {
		t.Errorf("nested CTE columns must not produce operand column refs, got %+v", opCand.OperandColumnRefs)
	}
}

func TestExtractQueryAccess_CTELexicalScope_CTEInJOIN(t *testing.T) {
	t.Parallel()
	sql := "WITH cte AS (SELECT id FROM users) SELECT * FROM cte JOIN orders ON cte.id = orders.user_id WHERE orders.amount > 100"
	facts := extractQA(t, sql)
	if len(facts.EffectCandidates) == 0 {
		t.Fatal("expected operator candidates")
	}
	for _, c := range facts.EffectCandidates {
		if c.Kind != EffectCandidateOperator {
			continue
		}
		for _, rc := range c.OperandColumnRefs {
			if rc.Table == "cte" {
				t.Errorf("CTE columns must not produce operand column refs, got %+v", rc)
			}
		}
	}
}
