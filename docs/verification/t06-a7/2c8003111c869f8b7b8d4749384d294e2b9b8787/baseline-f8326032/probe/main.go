// Probe: observe AST-level and extracted spec fields for T06-A7-BASELINE inputs.
// Runs outside the repository via module+replace; uses only real public entries.
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
	Name        string   `json:"name"`
	FTType      string   `json:"ft_type_string"`
	FTFlen      int      `json:"ft_flen"`
	FTDecimal   int      `json:"ft_decimal"`
	FTCharset   string   `json:"ft_charset"`
	FTCollate   string   `json:"ft_collate"`
	FTFlag      uint     `json:"ft_flag"`
	OptTypes    []string `json:"col_option_types"`
	SpecCharset string   `json:"spec_charset"`
	SpecCollate string   `json:"spec_collate"`
	SpecType    string   `json:"spec_type"`
	SpecLength  int      `json:"spec_length"`
}

type tblObs struct {
	ASTType         string            `json:"ast_type"`
	ASTTableOptions []string          `json:"ast_table_options"`
	Columns         []colObs          `json:"columns"`
	SpecOptions     map[string]string `json:"spec_options"`
	SpecUnextracted []string          `json:"spec_unextracted_options"`
}

func main() {
	ctx := context.Background()
	inputs := map[string]string{
		"S1": "CREATE TABLE t (\n  c VARCHAR(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin\n);",
		"S2": "CREATE TABLE t (c VARCHAR(16) COLLATE utf8mb4_bin);",
		"S3": "CREATE TABLE t (\n  c VARCHAR(16) CHARACTER SET utf8mb4 COLLATE latin1_swedish_ci\n);",
		"S4": "CREATE TABLE t (c INT) COLLATE=utf8mb4_bin;",
		"B1": "CREATE TABLE t (c VARCHAR(16));",
	}
	for _, id := range []string{"S1", "S2", "S3", "S4", "B1"} {
		sql := inputs[id]
		out := map[string]any{"id": id, "sql": sql}
		p := dstidb.New()
		res, err := p.Parse(ctx, sql)
		if err != nil {
			out["parse_error"] = err.Error()
			emit(out)
			continue
		}
		out["warnings"] = res.Warnings
		wrapped := dstidb.WrapStatements(res.Statements, res.Warnings)
		var obs []tblObs
		for i, node := range res.Statements {
			o := tblObs{}
			ct, ok := node.(*tidbast.CreateTableStmt)
			if ok {
				o.ASTType = "*ast.CreateTableStmt"
				for _, to := range ct.Options {
					o.ASTTableOptions = append(o.ASTTableOptions, fmt.Sprintf("%T:%d", to, int(to.Tp)))
				}
			} else {
				o.ASTType = fmt.Sprintf("%T", node)
			}
			st, err := wrapped[i].Extractor.Extract(spec.DialectMySQL, sql)
			if err != nil {
				out["extract_error"] = err.Error()
				break
			}
			if st.DDL != nil {
				o.SpecOptions = st.DDL.Options
				o.SpecUnextracted = st.DDL.UnextractedOptions
			}
			if ok {
				specCols := map[string]spec.Column{}
				if st.DDL != nil {
					for _, c := range st.DDL.Columns {
						specCols[c.Name] = c
					}
				}
				for _, cd := range ct.Cols {
					co := colObs{
						Name: cd.Name.Name.L, FTType: cd.Tp.String(), FTFlen: cd.Tp.GetFlen(),
						FTDecimal: cd.Tp.GetDecimal(), FTCharset: cd.Tp.GetCharset(),
						FTCollate: cd.Tp.GetCollate(), FTFlag: cd.Tp.GetFlag(),
					}
					for _, op := range cd.Options {
						co.OptTypes = append(co.OptTypes, fmt.Sprintf("%T:%d", op, int(op.Tp)))
					}
					if sc, ok := specCols[co.Name]; ok {
						co.SpecCharset, co.SpecCollate = sc.Charset, sc.Collation
						co.SpecType, co.SpecLength = sc.Type, sc.Length
					}
					o.Columns = append(o.Columns, co)
				}
			}
			obs = append(obs, o)
		}
		out["tables"] = obs
		emit(out)
	}
}

func emit(v any) {
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(b))
}
