// Probe for T06-A14-BASELINE: real parser/extractor observation of the fixed
// table-option inputs under both dialects. Read-only; built outside the repo.
package main

import (
	"context"
	"fmt"
	"sort"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	tidbparser "github.com/Fanduzi/DeltaScope/internal/infrastructure/parser/tidb"
	"github.com/pingcap/tidb/pkg/parser/ast"
)

func probe(dialect spec.Dialect, label, sql string) {
	parser := tidbparser.New()
	result, err := parser.Parse(context.Background(), sql)
	if err != nil {
		fmt.Printf("%s/%s parse error: %v\n", label, dialect, err)
		return
	}
	fmt.Printf("=== %s dialect=%s statements=%d warnings=%v\n", label, dialect, len(result.Statements), result.Warnings)
	create, ok := result.Statements[0].(*ast.CreateTableStmt)
	if !ok {
		fmt.Printf("  statement type %T\n", result.Statements[0])
		return
	}
	fmt.Printf("  ast table-options=%d\n", len(create.Options))
	for i, o := range create.Options {
		if o == nil {
			continue
		}
		fmt.Printf("    ast[%d] tp=%d str=%q uint=%d bool=%v\n", i, o.Tp, o.StrValue, o.UintValue, o.BoolValue)
	}
	wrapped := tidbparser.WrapStatements(result.Statements, result.Warnings)
	statement, err := wrapped[0].Extractor.Extract(dialect, wrapped[0].RawSQL)
	if err != nil {
		fmt.Printf("  extract error: %v\n", err)
		return
	}
	keys := []string{}
	for k := range statement.DDL.Options {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	fmt.Printf("  spec.Options keys=%v unextracted=%v\n", keys, statement.DDL.UnextractedOptions)
	for _, k := range keys {
		fmt.Printf("    Options[%q]=%q\n", k, statement.DDL.Options[k])
	}
	for i, c := range statement.DDL.Columns {
		fmt.Printf("  spec[%d] name=%s type=%q length=%d charset=%q collation=%q\n",
			i, c.Name, c.Type, c.Length, c.Charset, c.Collation)
	}
}

func main() {
	sqls := map[string]string{
		"s0-omitted":        "CREATE TABLE t (c INT);",
		"engine-allowed":    "CREATE TABLE t (c INT) ENGINE=InnoDB;",
		"engine-denied":     "CREATE TABLE t (c INT) ENGINE=MyISAM;",
		"charset-allowed":   "CREATE TABLE t (c INT) DEFAULT CHARSET=utf8mb4;",
		"charset-denied":    "CREATE TABLE t (c INT) DEFAULT CHARSET=latin1;",
		"rowformat-allowed": "CREATE TABLE t (c INT) ROW_FORMAT=DYNAMIC;",
		"rowformat-denied":  "CREATE TABLE t (c INT) ROW_FORMAT=COMPACT;",
		"column-charset":    "CREATE TABLE t (c VARCHAR(8) CHARACTER SET utf8mb4);",
	}
	order := []string{"s0-omitted", "engine-allowed", "engine-denied", "charset-allowed", "charset-denied", "rowformat-allowed", "rowformat-denied", "column-charset"}
	for _, d := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		for _, label := range order {
			probe(d, label, sqls[label])
		}
	}
}
