// Probe: observe AST-level expression nodes and extracted spec fields for
// T06-A9-BASELINE audit-column inputs. Runs outside the repository via
// module+replace; uses only real public parser/extractor entries.
package main

import (
	"context"
	"encoding/json"
	"fmt"

	tidbast "github.com/pingcap/tidb/pkg/parser/ast"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	dstidb "github.com/Fanduzi/DeltaScope/internal/infrastructure/parser/tidb"
)

type colObs struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	NotNull      bool   `json:"not_null"`
	HasDefault   bool   `json:"has_default"`
	DefaultValue string `json:"default_value"`
	DefaultNull  bool   `json:"default_is_null"`
	DefaultCT    bool   `json:"default_is_current_timestamp"`
	OnUpdateCT   bool   `json:"on_update_current_timestamp"`
	Unextracted  []string `json:"unextracted,omitempty"`
	ASTOptions   []string `json:"ast_options"`
}

type obs struct {
	ID        string   `json:"id"`
	SQL       string   `json:"sql"`
	Columns   []colObs `json:"columns"`
	ParseError string  `json:"parse_error,omitempty"`
}

func describeExpr(e tidbast.ExprNode) string {
	if e == nil {
		return "<nil>"
	}
	switch v := e.(type) {
	case *tidbast.FuncCallExpr:
		return fmt.Sprintf("FuncCallExpr fn=%q args=%d", v.FnName.L, len(v.Args))
	case *tidbast.ColumnNameExpr:
		return fmt.Sprintf("ColumnNameExpr name=%q", v.Name.Name.L)
	case tidbast.ValueExpr:
		return fmt.Sprintf("ValueExpr %T val=%v", v.GetValue(), v.GetValue())
	default:
		return fmt.Sprintf("%T", e)
	}
}

func main() {
	ctx := context.Background()
	inputs := []struct{ id, sql string }{
		// spec roles (P_ON variants; F shares D's shape)
		{"A-complete-pair", "CREATE TABLE t (id INT PRIMARY KEY, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP);"},
		{"B-missing-created", "CREATE TABLE t (id INT PRIMARY KEY, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP);"},
		{"C-missing-updated", "CREATE TABLE t (id INT PRIMARY KEY, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP);"},
		{"D-missing-both", "CREATE TABLE t (id INT PRIMARY KEY);"},
		{"E-now-spelling", "CREATE TABLE t (id INT PRIMARY KEY, created_at DATETIME NOT NULL DEFAULT NOW(), updated_at DATETIME NOT NULL DEFAULT NOW() ON UPDATE NOW());"},
		// probe-only adjacent observations: spelling/type variants, not new spec inputs
		{"P-ct-parens", "CREATE TABLE t (a DATETIME DEFAULT CURRENT_TIMESTAMP(), b DATETIME DEFAULT current_timestamp);"},
		{"P-synonyms", "CREATE TABLE t (a DATETIME DEFAULT LOCALTIME, b DATETIME DEFAULT LOCALTIMESTAMP, c DATETIME DEFAULT LOCALTIME() ON UPDATE LOCALTIME);"},
		{"P-now-fsp", "CREATE TABLE t (a DATETIME(3) DEFAULT NOW(3) ON UPDATE NOW(3));"},
		{"P-not-ct", "CREATE TABLE t (a DATETIME DEFAULT '2020-01-01 00:00:00', b DATETIME DEFAULT 0, c DATETIME DEFAULT NULL);"},
		{"P-timestamp-type", "CREATE TABLE t (a TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP);"},
		{"P-nontime-type", "CREATE TABLE t (a INT DEFAULT 1, b VARCHAR(20) DEFAULT CURRENT_TIMESTAMP);"},
		{"P-onupdate-only", "CREATE TABLE t (a DATETIME ON UPDATE CURRENT_TIMESTAMP, b DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE 'x');"},
		{"P-onupdate-no-default", "CREATE TABLE t (a DATETIME NOT NULL ON UPDATE CURRENT_TIMESTAMP);"},
	}
	for _, in := range inputs {
		o := obs{ID: in.id, SQL: in.sql}
		p := dstidb.New()
		res, err := p.Parse(ctx, in.sql)
		if err != nil {
			o.ParseError = err.Error()
			emit(o)
			continue
		}
		wrapped := dstidb.WrapStatements(res.Statements, res.Warnings)
		for i, node := range res.Statements {
			ct, ok := node.(*tidbast.CreateTableStmt)
			if ok {
				for _, cd := range ct.Cols {
					co := colObs{Name: cd.Name.Name.L}
					for _, opt := range cd.Options {
						co.ASTOptions = append(co.ASTOptions, fmt.Sprintf("tp=%d expr=%s", int(opt.Tp), describeExpr(opt.Expr)))
					}
					o.Columns = append(o.Columns, co)
				}
			}
			st, err := wrapped[i].Extractor.Extract(spec.DialectMySQL, in.sql)
			if err != nil {
				o.ParseError = "extract: " + err.Error()
				break
			}
			if st.DDL != nil {
				for ci, c := range st.DDL.Columns {
					o.Columns[ci].Type = c.Type
					o.Columns[ci].NotNull = c.NotNull
					o.Columns[ci].HasDefault = c.HasDefault
					o.Columns[ci].DefaultValue = c.DefaultValue
					o.Columns[ci].DefaultNull = c.DefaultIsNull
					o.Columns[ci].DefaultCT = c.DefaultIsCurrentTimestamp
					o.Columns[ci].OnUpdateCT = c.OnUpdateCurrentTimestamp
					o.Columns[ci].Unextracted = c.UnextractedOptions
				}
			}
		}
		emit(o)
	}
}

func emit(v any) {
	b, _ := json.Marshal(v)
	fmt.Println(string(b))
}
