package spec

import "testing"

func TestT18FullReviewFirstRootSharedAndCyclic(t *testing.T) {
	empty := &ViewQuery{}
	empty.Sources = []ViewQuerySource{{Query: empty}}
	first := &ViewQuery{Sources: []ViewQuerySource{{Table: &Table{Name: "first"}, Index: 7}}}
	second := &ViewQuery{Sources: []ViewQuerySource{{Table: &Table{Name: "second"}, Index: 9}}}
	query := &ViewQuery{
		Sources:     []ViewQuerySource{{Query: empty}, {Query: empty}, {Query: first}},
		SetOperands: []*ViewQuery{second},
	}
	if got := viewQueryFirstRoot(query); got != 7 {
		t.Fatalf("first root = %d, want 7", got)
	}
	if got := viewQueryFirstRoot(empty); got != -1 {
		t.Fatalf("cycle without a physical root = %d, want -1", got)
	}
}
