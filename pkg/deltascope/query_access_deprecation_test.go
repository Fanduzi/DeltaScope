// Package deltascope pins the canonical Go deprecation notices on the six
// dialect-specific compatibility identifiers.
// input: local package source files parsed with go/parser and go/ast
// output: assertions that every deprecated identifier carries a Deprecated: notice naming its replacement
// pos: minimal deprecation-contract seam for the public compatibility API
// note: if this file changes, update this header and module README.md.
package deltascope

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// deprecatedContract maps each deprecated public identifier to its source
// declaration site and replacement, mirroring what go doc renders. The two
// PostgreSQL functions appear in both the tagged and untagged build files.
var deprecatedContract = []struct {
	name        string
	file        string
	replacement string
}{
	{"PostgreSQLQueryAccessSession", "query_access_session_common.go", "OnlineQueryAccessSession"},
	{"NewPostgreSQLQueryAccessSessionFromConn", "query_access_session.go", "NewOnlineQueryAccessSessionFromConn"},
	{"AnalyzePostgreSQLQueryAccessWithSession", "query_access_session.go", "AnalyzeOnlineQueryAccessWithSession"},
	{"NewPostgreSQLQueryAccessSessionFromConn", "query_access_session_stub.go", "NewOnlineQueryAccessSessionFromConn"},
	{"AnalyzePostgreSQLQueryAccessWithSession", "query_access_session_stub.go", "AnalyzeOnlineQueryAccessWithSession"},
	{"MySQLTiDBQueryAccessSession", "query_access_session_mysql_tidb.go", "OnlineQueryAccessSession"},
	{"NewMySQLTiDBQueryAccessSessionFromConn", "query_access_session_mysql_tidb.go", "NewOnlineQueryAccessSessionFromConn"},
	{"AnalyzeMySQLTiDBQueryAccessWithSession", "query_access_session_mysql_tidb.go", "AnalyzeOnlineQueryAccessWithSession"},
}

func TestDialectSessionAPI_DeprecationNotices(t *testing.T) {
	for _, c := range deprecatedContract {
		t.Run(c.name+"@"+c.file, func(t *testing.T) {
			doc := declarationDoc(t, c.file, c.name)
			if !strings.Contains(doc, "Deprecated:") {
				t.Fatalf("%s in %s must carry a canonical Deprecated: notice", c.name, c.file)
			}
			if !strings.Contains(doc, "Deprecated: Use "+c.replacement) {
				t.Fatalf("%s in %s must name replacement %s", c.name, c.file, c.replacement)
			}
		})
	}
}

// declarationDoc returns the doc comment text for the named top-level
// declaration in file, the same comment go doc renders for the identifier.
func declarationDoc(t *testing.T, file, name string) string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse %s: %v", file, err)
	}
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Name.Name == name && d.Doc != nil {
				return d.Doc.Text()
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				if ts, ok := spec.(*ast.TypeSpec); ok && ts.Name.Name == name && d.Doc != nil {
					return d.Doc.Text()
				}
			}
		}
	}
	t.Fatalf("%s not declared in %s", name, file)
	return ""
}
