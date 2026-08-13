// Package cli verifies the CLI query access adapter structure.
// input: CLI query access source and online routing implementation
// output: structural evidence that online Query Access uses the unified SDK entry
// pos: CLI migration contract tests
// note: if this file changes, update this header and module README.md.
package cli

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRunQueryAccessOnlineUsesUnifiedQueryAccessEntry(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	source, err := os.ReadFile(filepath.Join(filepath.Dir(filename), "query_access.go"))
	if err != nil {
		t.Fatalf("read query_access.go: %v", err)
	}

	file, err := parser.ParseFile(token.NewFileSet(), "query_access.go", source, 0)
	if err != nil {
		t.Fatalf("parse query_access.go: %v", err)
	}

	var onlineFunction *ast.FuncDecl
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if ok && function.Name.Name == "runQueryAccessOnline" {
			onlineFunction = function
			break
		}
	}
	if onlineFunction == nil {
		t.Fatal("runQueryAccessOnline not found")
	}

	var body strings.Builder
	ast.Fprint(&body, token.NewFileSet(), onlineFunction.Body, nil)
	bodyText := body.String()
	for _, forbidden := range []string{
		"Identity.Product",
		"NewPostgreSQLQueryAccessSessionFromConn",
		"NewMySQLTiDBQueryAccessSessionFromConn",
		"AnalyzePostgreSQLQueryAccessWithSession",
		"AnalyzeMySQLTiDBQueryAccessWithSession",
	} {
		if strings.Contains(bodyText, forbidden) {
			t.Errorf("runQueryAccessOnline still contains product-specific routing: %s", forbidden)
		}
	}
	for _, required := range []string{
		"NewOnlineQueryAccessSessionFromConn",
		"AnalyzeOnlineQueryAccessWithSession",
	} {
		if !strings.Contains(bodyText, required) {
			t.Errorf("runQueryAccessOnline must use unified entry symbol: %s", required)
		}
	}
}
