// T06-A12-BASELINE parser/extractor probe. Observes the real AST column
// option list (ColumnOptionNotNull/ColumnOptionNull presence vs absence),
// FieldType rendering, and the extracted spec.Column facts (NotNull,
// HasDefault, DefaultIsCurrentTimestamp, OnUpdateCurrentTimestamp) for the
// eight fixed CREATE TABLE inputs plus one parser-only explicit-NULL
// control. Nothing is fabricated: every printed value comes from
// tidbparser.New().Parse + WrapStatements + Extractor.Extract.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	tidbparser "github.com/Fanduzi/DeltaScope/internal/infrastructure/parser/tidb"
	"github.com/pingcap/tidb/pkg/parser/ast"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: probe <inputs-dir>")
		os.Exit(2)
	}
	files, err := filepath.Glob(filepath.Join(os.Args[1], "*.sql"))
	if err != nil || len(files) == 0 {
		fmt.Fprintf(os.Stderr, "no inputs under %s\n", os.Args[1])
		os.Exit(2)
	}
	sort.Strings(files)

	p := tidbparser.New()
	for _, path := range files {
		raw, _ := os.ReadFile(path)
		result, err := p.Parse(context.Background(), string(raw))
		fmt.Printf("===== %s =====\n", filepath.Base(path))
		if err != nil {
			fmt.Printf("parse ERROR: %v\n", err)
			continue
		}
		fmt.Printf("ast_nodes=%d warnings=%v\n", len(result.Statements), result.Warnings)
		for i, node := range result.Statements {
			ct, ok := node.(*ast.CreateTableStmt)
			if !ok {
				fmt.Printf("  node[%d] type=%T\n", i, node)
				continue
			}
			for ci, col := range ct.Cols {
				opts := []int{}
				for _, o := range col.Options {
					if o != nil {
						opts = append(opts, int(o.Tp))
					}
				}
				fmt.Printf("  ast.col[%d] name=%s tp_str=%q flen=%d optionTps=%v\n",
					ci, col.Name.Name.L, col.Tp.String(), col.Tp.GetFlen(), opts)
			}
			fmt.Printf("  ast.constraints=%d\n", len(ct.Constraints))
		}
		for wi, w := range tidbparser.WrapStatements(result.Statements, result.Warnings) {
			for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
				st, err := w.Extractor.Extract(dialect, w.RawSQL)
				if err != nil {
					fmt.Printf("  extract[%d][%s] ERROR: %v\n", wi, dialect, err)
					continue
				}
				fmt.Printf("  extract[%d][%s] kind=%s pk_nil=%v\n",
					wi, dialect, st.Kind, st.DDL.PrimaryKey == nil)
				for ci, c := range st.DDL.Columns {
					b, _ := json.Marshal(map[string]any{
						"name": c.Name, "type": c.Type, "not_null": c.NotNull,
						"has_default": c.HasDefault, "default_is_null": c.DefaultIsNull,
						"default_cts": c.DefaultIsCurrentTimestamp,
						"on_update_cts": c.OnUpdateCurrentTimestamp,
						"unextracted": c.UnextractedOptions,
					})
					fmt.Printf("    Column[%d]=%s\n", ci, b)
				}
			}
		}
	}
}
