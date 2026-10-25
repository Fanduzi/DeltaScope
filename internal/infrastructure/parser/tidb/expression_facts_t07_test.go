// Package tidbparser verifies the T07 catalog expression boundary:
// ParseCatalogExpressionFacts accepts exactly one bare data expression and
// decodes exactly one recognized escaped SQL-string layer.
package tidbparser

import (
	"context"
	"strings"
	"testing"

	"github.com/pingcap/tidb/pkg/parser/ast"
)

func TestParseCatalogExpressionFactsBareExpressions(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		text      string
		wantKnown bool
		wantRefs  []string
		wantFuncs []string
	}{
		{"simple_arith", "`a` + 1", true, []string{"a"}, nil},
		{"paren_arith", "(`a` + 1)", true, []string{"a"}, nil},
		{"concat_escaped", "concat(`a`,_utf8mb4\\'x\\')", true, []string{"a"}, []string{"concat"}},
		{"json_escaped", "json_object(_utf8mb4\\'k\\',1)", true, nil, []string{"json_object"}},
		{"json_extract_unhex_sha2", "json_unquote(json_extract(`payload`,'$.type'))", true, []string{"payload"}, []string{"json_unquote", "json_extract"}},
		{"unhex_sha2_cast", "unhex(sha2(cast(`payload` as char),256))", true, []string{"payload"}, []string{"unhex", "sha2"}},
		// Unknown callees stay an unknown/tool boundary but still keep the
		// structural references already collected inside their arguments.
		{"unknown_fn", "t07_unknown_fn(`a`)", false, []string{"a"}, nil},
		{"literal_only", "'x'", true, nil, nil},
		{"empty", "", false, nil, nil},
		{"whitespace", "   ", false, nil, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			facts := ParseCatalogExpressionFacts(tc.text)
			if facts.Known != tc.wantKnown {
				t.Fatalf("%q known = %v, want %v (facts %+v)", tc.text, facts.Known, tc.wantKnown, facts)
			}
			var refs []string
			for _, ref := range facts.References {
				refs = append(refs, ref.Name)
			}
			if len(refs) != len(tc.wantRefs) {
				t.Fatalf("%q refs = %v, want %v", tc.text, refs, tc.wantRefs)
			}
			for i, want := range tc.wantRefs {
				if refs[i] != want {
					t.Fatalf("%q refs = %v, want %v", tc.text, refs, tc.wantRefs)
				}
			}
			for _, want := range tc.wantFuncs {
				found := false
				for _, got := range facts.Functions {
					if got == want {
						found = true
					}
				}
				if !found {
					t.Fatalf("%q functions = %v, want %q", tc.text, facts.Functions, want)
				}
			}
			if facts.Known && facts.Fingerprint == "" {
				t.Fatalf("%q known but missing fingerprint", tc.text)
			}
		})
	}
}

// TestParseCatalogExpressionFactsRejectsSelectStructure proves catalog text
// can only be one bare data expression — SELECT structure is rejected
// outright, never silently truncated into an expression fact.
func TestParseCatalogExpressionFactsRejectsSelectStructure(t *testing.T) {
	t.Parallel()
	for _, text := range []string{
		"a from t",
		"a where b = 1",
		"a order by b",
		"a limit 1",
		"a as x",
		"*",
		"a union select 1",
		"a, b",
		"a; drop table t",
		"a into @v",
	} {
		if facts := ParseCatalogExpressionFacts(text); facts.Known {
			t.Fatalf("catalog expression %q parsed as known: %+v", text, facts)
		}
	}
}

// TestParseCatalogExpressionFactsEscapeBoundary pins the decode boundary:
// recognized quote/backslash escapes decode once; control escapes, LIKE
// wildcards, and second-layer encodings never become valid expressions.
func TestParseCatalogExpressionFactsEscapeBoundary(t *testing.T) {
	t.Parallel()
	// One recognized layer: \' and \\ decode to a parseable expression.
	if facts := ParseCatalogExpressionFacts("'a\\'b'"); !facts.Known {
		t.Fatalf("escaped quote literal rejected: %+v", facts)
	}
	for _, text := range []string{
		"'a%b' escape",  // trailing junk after a literal
		"a\\nb",         // bare control escape never becomes a valid expr
		"\\\\'x\\'",     // double-escaped quotes are a second layer — not decoded
		"concat(`a`\\)", // malformed escape before paren
	} {
		facts := ParseCatalogExpressionFacts(text)
		if facts.Known {
			t.Fatalf("catalog expression %q unexpectedly known: %+v", text, facts)
		}
	}
	// Underscore is a legal identifier and `%` is the modulo operator, so
	// bare `a%_` is a real expression — LIKE-style markers only apply inside
	// quoted literals and never get stripped into validity here.
	if facts := ParseCatalogExpressionFacts("a%_"); !facts.Known {
		t.Fatalf("modulo expression rejected: %+v", facts)
	}
}

// TestSupplementedDefaultPreservesRawText proves the placeholder never
// escapes: supplemented parses keep the authored bytes in statement text and
// expose real expression facts on the column definition.
func TestSupplementedDefaultPreservesRawText(t *testing.T) {
	t.Parallel()
	parser := New()
	input := "CREATE TABLE t (a INT, b INT DEFAULT (a + 1), c INT DEFAULT (uuid()));"
	result, err := parser.Parse(context.Background(), input)
	if err != nil {
		t.Fatalf("supplemented parse: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("statements = %d", len(result.Statements))
	}
	stmt := result.Statements[0]
	// Raw text is byte-exact the authored input — never the rewritten text.
	if stmt.Text() != input {
		t.Fatalf("statement text = %q, want byte-exact %q", stmt.Text(), input)
	}
	if !strings.Contains(stmt.Text(), "DEFAULT (a + 1)") {
		t.Fatalf("placeholder leaked into statement text: %q", stmt.Text())
	}
	node := ast.StmtNode(stmt)
	if carried, ok := stmt.(*supplementedStatement); ok {
		node = carried.StmtNode
	}
	create, ok := node.(*ast.CreateTableStmt)
	if !ok {
		t.Fatalf("stmt = %T, want CreateTableStmt", stmt)
	}
	if len(create.Cols) != 3 {
		t.Fatalf("cols = %d", len(create.Cols))
	}
	for _, col := range create.Cols {
		for _, option := range col.Options {
			if option.Expr == nil {
				continue
			}
			if option.Expr.OriginTextPosition() == 0 {
				t.Fatalf("col %q option expr lost its source position", col.Name.Name.O)
			}
		}
	}
}
