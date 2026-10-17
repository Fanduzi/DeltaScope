// Probe for T06-A13-BASELINE: real parser/extractor observation of the two
// fixed inputs under both dialects. Read-only; built outside the repository.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	tidbparser "github.com/Fanduzi/DeltaScope/internal/infrastructure/parser/tidb"
	"github.com/pingcap/tidb/pkg/parser/ast"
	tidbtypes "github.com/pingcap/tidb/pkg/parser/types"
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
	wrapped := tidbparser.WrapStatements(result.Statements, result.Warnings)
	statement, err := wrapped[0].Extractor.Extract(dialect, wrapped[0].RawSQL)
	if err != nil {
		fmt.Printf("  extract error: %v\n", err)
		return
	}
	for i, col := range create.Cols {
		opts := []string{}
		for _, o := range col.Options {
			if o != nil {
				opts = append(opts, fmt.Sprintf("%d", o.Tp))
			}
		}
		fmt.Printf("  ast[%d] name=%s tp=%d tpstr=%q flen=%d decimal=%d charset=%q collate=%q hascharset=%v options=%v\n",
			i, col.Name.Name.L, col.Tp.GetType(), col.Tp.String(), col.Tp.GetFlen(), col.Tp.GetDecimal(),
			col.Tp.GetCharset(), col.Tp.GetCollate(), tidbtypes.HasCharset(col.Tp), opts)
	}
	fmt.Printf("  spec: ddl=%v cols=%d pk=%v indexes=%d\n",
		statement.DDL != nil, len(statement.DDL.Columns), statement.DDL.PrimaryKey, len(statement.DDL.Indexes))
	for i, c := range statement.DDL.Columns {
		fmt.Printf("  spec[%d] name=%s type=%q length=%d charset=%q collation=%q notnull=%v hasdefault=%v defcurts=%v onupdcurts=%v unextracted=%v\n",
			i, c.Name, c.Type, c.Length, c.Charset, c.Collation, c.NotNull, c.HasDefault,
			c.DefaultIsCurrentTimestamp, c.OnUpdateCurrentTimestamp, c.UnextractedOptions)
	}
}

func main() {
	m := "CREATE TABLE t (c_blob BLOB, c_text TEXT, c_json JSON, c_bit BIT(1), c_float FLOAT, c_double DOUBLE, c_timestamp TIMESTAMP NULL, c_datetime DATETIME NULL, c_int INT);"
	s := "CREATE TABLE t (c_int INT, c_string VARCHAR(8), c_datetime DATETIME NULL);"
	for _, d := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		probe(d, "M", m)
		probe(d, "S", s)
	}
	_ = os.Stdout
}
