// Package main verifies the T07 supplemental proof rejects incomplete private facts and evidence failures.
// input: independently fixed eight-column facts and controlled corruptions or filesystem/encoding errors
// output: exact complete-set/known-state/provenance sensitivity and fail-closed evidence-write assertions
// pos: bounded proof-collector tests, not product/native acceptance results
// note: if this file changes, update this header and module README.md.
package main

import (
	"path/filepath"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func validFacts() []columnFact {
	return []columnFact{
		{Name: "id", Type: "bigint", Visibility: "visible"},
		{Name: "a", Type: "int", Visibility: "visible"},
		{Name: "stored_c", Type: "int", Generated: true, Storage: "stored", GenerationKnown: true, GenerationRefs: []spec.ColumnReference{{Name: "a"}}, Visibility: "visible"},
		{Name: "virtual_c", Type: "int", Generated: true, Storage: "virtual", GenerationKnown: true, GenerationRefs: []spec.ColumnReference{{Name: "a"}}, Visibility: "visible"},
		{Name: "literal_c", Type: "varchar(8)", Generated: true, Storage: "virtual", GenerationKnown: true, Visibility: "visible"},
		{Name: "nullable_c", Type: "int", Visibility: "visible"},
		{Name: "ordinary_c", Type: "varchar(8)", HasDefault: true, DefaultValueEmpty: true, Visibility: "visible"},
		{Name: "uuid_c", Type: "varchar(36)", Visibility: "visible"},
	}
}

func TestT07ProofFactChecks(t *testing.T) {
	_, want := fixedFixture("mysql57")
	if err := checkFacts(validFacts(), want, true); err != nil {
		t.Fatalf("independent complete control: %v", err)
	}
	cases := []struct {
		name string
		edit func([]columnFact) []columnFact
	}{
		{"empty", func(f []columnFact) []columnFact { return nil }},
		{"missing", func(f []columnFact) []columnFact { return f[:7] }},
		{"extra", func(f []columnFact) []columnFact { return append(f, columnFact{Name: "extra"}) }},
		{"duplicate", func(f []columnFact) []columnFact { f[1].Name = f[0].Name; return f }},
		{"reordered", func(f []columnFact) []columnFact { f[1], f[2] = f[2], f[1]; return f }},
		{"wrong-type", func(f []columnFact) []columnFact { f[1].Type = "varchar(8)"; return f }},
		{"missing-generated", func(f []columnFact) []columnFact { f[2].Generated = false; return f }},
		{"wrong-storage", func(f []columnFact) []columnFact { f[2].Storage = "virtual"; return f }},
		{"unknown-not-empty", func(f []columnFact) []columnFact { f[4].GenerationKnown = false; return f }},
		{"missing-dependency", func(f []columnFact) []columnFact { f[2].GenerationRefs = nil; return f }},
		{"wrong-dependency", func(f []columnFact) []columnFact {
			f[2].GenerationRefs = []spec.ColumnReference{{Name: "id"}}
			return f
		}},
		{"foreign-qualifier", func(f []columnFact) []columnFact {
			f[2].GenerationRefs = []spec.ColumnReference{{Table: "other", Name: "a"}}
			return f
		}},
		{"literal-false-reference", func(f []columnFact) []columnFact { f[4].GenerationRefs = []spec.ColumnReference{{Name: "a"}}; return f }},
		{"empty-default-lost", func(f []columnFact) []columnFact { f[6].HasDefault = false; return f }},
		{"empty-default-changed", func(f []columnFact) []columnFact { f[6].DefaultValueEmpty = false; return f }},
		{"visibility-unknown", func(f []columnFact) []columnFact { f[1].Visibility = ""; return f }},
		{"catalog-as-declaration", func(f []columnFact) []columnFact { f[1].VisibilityExplicit = true; return f }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := checkFacts(tc.edit(validFacts()), want, true); err == nil {
				t.Fatal("incomplete or forged fact collection accepted")
			}
		})
	}
}

func TestT07ProofKnownDefaultExpression(t *testing.T) {
	_, want := fixedFixture("tidb85")
	facts := append(validFacts(), columnFact{Name: "json_c", Type: "json", HasDefault: true, DefaultExpression: true, DefaultKnown: true, Visibility: "visible"})
	facts[7].DefaultExpression, facts[7].DefaultKnown, facts[7].HasDefault = true, true, true
	if err := checkFacts(facts, want, true); err != nil {
		t.Fatalf("known zero-reference JSON/UUID defaults: %v", err)
	}
	facts[8].DefaultKnown = false
	if err := checkFacts(facts, want, true); err == nil {
		t.Fatal("unknown default dependencies accepted as known empty")
	}
}

func TestT07ProofEvidenceWrites(t *testing.T) {
	dir := t.TempDir()
	if err := writeJSON(dir, "record.json", map[string]any{"fixed": true}); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(dir, "record.json", map[string]any{"overwrite": true}); err == nil {
		t.Fatal("evidence overwrite accepted")
	}
	if err := writeJSON(dir, "encode.json", make(chan int)); err == nil {
		t.Fatal("JSON encoding error accepted")
	}
	if err := writeJSON(dir, filepath.Join("missing", "record.json"), map[string]any{}); err == nil {
		t.Fatal("evidence creation error accepted")
	}
}
