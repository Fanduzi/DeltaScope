// T06-A10-BASELINE parser/extractor probe. Observes the real AST and the real
// extracted spec fields for the eight fixed CREATE TABLE inputs. No fabricated
// spec values: everything printed comes from tidbparser.New().Parse +
// WrapStatements + ExtractedStatement.Extractor.Extract.
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

func colOptName(tp ast.ColumnOptionType) string {
	switch tp {
	case ast.ColumnOptionAutoIncrement:
		return "ColumnOptionAutoIncrement"
	case ast.ColumnOptionPrimaryKey:
		return "ColumnOptionPrimaryKey"
	case ast.ColumnOptionNotNull:
		return "ColumnOptionNotNull"
	case ast.ColumnOptionNull:
		return "ColumnOptionNull"
	case ast.ColumnOptionDefaultValue:
		return "ColumnOptionDefaultValue"
	case ast.ColumnOptionOnUpdate:
		return "ColumnOptionOnUpdate"
	}
	return fmt.Sprintf("ColumnOptionType(%d)", int(tp))
}

func tblOptName(tp ast.TableOptionType) string {
	switch tp {
	case ast.TableOptionAutoIncrement:
		return "TableOptionAutoIncrement"
	case ast.TableOptionEngine:
		return "TableOptionEngine"
	case ast.TableOptionCharset:
		return "TableOptionCharset"
	case ast.TableOptionCollate:
		return "TableOptionCollate"
	case ast.TableOptionComment:
		return "TableOptionComment"
	}
	return fmt.Sprintf("TableOptionType(%d)", int(tp))
}

func constraintName(tp ast.ConstraintType) string {
	switch tp {
	case ast.ConstraintPrimaryKey:
		return "ConstraintPrimaryKey"
	case ast.ConstraintKey:
		return "ConstraintKey"
	case ast.ConstraintIndex:
		return "ConstraintIndex"
	}
	return fmt.Sprintf("ConstraintType(%d)", int(tp))
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: probe <inputs-dir>")
		os.Exit(2)
	}
	dir := os.Args[1]
	files, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	if err != nil || len(files) == 0 {
		fmt.Fprintf(os.Stderr, "no inputs under %s\n", dir)
		os.Exit(2)
	}
	sort.Strings(files)

	p := tidbparser.New()
	for _, path := range files {
		raw, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "read %s: %v\n", path, err)
			os.Exit(1)
		}
		sql := string(raw)
		result, err := p.Parse(context.Background(), sql)
		if err != nil {
			fmt.Fprintf(os.Stderr, "parse %s: %v\n", path, err)
			os.Exit(1)
		}
		fmt.Printf("===== %s =====\n", filepath.Base(path))
		fmt.Printf("raw: %s", sql)
		fmt.Printf("ast_nodes=%d warnings=%v\n", len(result.Statements), result.Warnings)
		for i, node := range result.Statements {
			ct, ok := node.(*ast.CreateTableStmt)
			if !ok {
				fmt.Printf("  node[%d] type=%T (not CreateTableStmt)\n", i, node)
				continue
			}
			fmt.Printf("  node[%d] *ast.CreateTableStmt table=%s ifNotExists=%v\n", i, ct.Table.Name.L, ct.IfNotExists)
			for ci, col := range ct.Cols {
				fmt.Printf("    col[%d] name=%s type=%s flen=%d unsignedFlag=%v options=[", ci, col.Name.Name.L, col.Tp.String(), col.Tp.GetFlen(), col.Tp.GetFlag()&0x20 != 0)
				for oi, opt := range col.Options {
					if oi > 0 {
						fmt.Print(" ")
					}
					fmt.Print(colOptName(opt.Tp))
					if opt.Tp == ast.ColumnOptionPrimaryKey {
						fmt.Printf("(StrValue=%q PrimaryKeyTp=%v)", opt.StrValue, opt.PrimaryKeyTp)
					}
				}
				fmt.Println("]")
			}
			for ci, c := range ct.Constraints {
				fmt.Printf("    constraint[%d] %s keys=", ci, constraintName(c.Tp))
				for ki, k := range c.Keys {
					if ki > 0 {
						fmt.Print(",")
					}
					if k.Column != nil {
						fmt.Printf("%s(len=%d)", k.Column.Name.L, k.Length)
					} else {
						fmt.Printf("<expr:%T>", k.Expr)
					}
				}
				fmt.Println()
			}
			fmt.Print("    table_options=[")
			for oi, opt := range ct.Options {
				if oi > 0 {
					fmt.Print(" ")
				}
				fmt.Printf("%s(StrValue=%q UintValue=%d)", tblOptName(opt.Tp), opt.StrValue, opt.UintValue)
			}
			fmt.Println("]")
		}

		wrapped := tidbparser.WrapStatements(result.Statements, result.Warnings)
		for wi, w := range wrapped {
			for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
				st, err := w.Extractor.Extract(dialect, w.RawSQL)
				if err != nil {
					fmt.Printf("  extract[%d][%s] ERROR: %v\n", wi, dialect, err)
					continue
				}
				fmt.Printf("  extract[%d][%s] kind=%s\n", wi, dialect, st.Kind)
				if st.DDL == nil {
					fmt.Println("    DDL=nil")
					continue
				}
				if pk := st.DDL.PrimaryKey; pk != nil {
					fmt.Printf("    PrimaryKey name=%q kind=%s columns=%v\n", pk.Name, pk.Kind, pk.Columns)
				} else {
					fmt.Println("    PrimaryKey=nil")
				}
				for ci, c := range st.DDL.Columns {
					b, _ := json.Marshal(c)
					fmt.Printf("    Column[%d]=%s\n", ci, b)
				}
				optJSON, _ := json.Marshal(st.DDL.Options)
				fmt.Printf("    Options=%s auto_increment_present=%v auto_increment_value=%q\n",
					optJSON, func() bool { _, ok := st.DDL.Options["auto_increment"]; return ok }(), st.DDL.Options["auto_increment"])
				fmt.Printf("    UnextractedOptions=%v\n", st.DDL.UnextractedOptions)
			}
		}
	}
}
