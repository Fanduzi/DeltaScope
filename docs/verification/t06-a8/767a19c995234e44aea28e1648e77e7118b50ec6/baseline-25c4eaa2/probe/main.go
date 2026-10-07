// Probe: observe AST-level and extracted spec comment fields for
// T06-A8-BASELINE inputs. Runs outside the repository via module+replace;
// uses only real public parser/extractor entries.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"unicode/utf8"

	tidbast "github.com/pingcap/tidb/pkg/parser/ast"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	dstidb "github.com/Fanduzi/DeltaScope/internal/infrastructure/parser/tidb"
)

type obs struct {
	ID             string            `json:"id"`
	SQL            string            `json:"sql"`
	TableComment   string            `json:"table_comment"`
	TableCommentB  int               `json:"table_comment_bytes"`
	TableCommentR  int               `json:"table_comment_runes"`
	OptComment     string            `json:"options_comment"`
	OptCommentOK   bool              `json:"options_comment_present"`
	ColComment     map[string]string `json:"column_comments"`
	ColCommentInfo map[string]string `json:"column_comment_len"` // "bytes/runes"
	Unextracted    []string          `json:"unextracted"`
	ASTTableOpts   []string          `json:"ast_table_options"`
	ASTColOpts     map[string][]string `json:"ast_column_options"`
	StrValComment  string            `json:"ast_table_option_comment_strvalue"`
	ParseError     string            `json:"parse_error,omitempty"`
}

func main() {
	ctx := context.Background()
	inputs := []struct{ id, sql string }{
		{"1-table-missing", "CREATE TABLE t (c INT);"},
		{"2-table-present", "CREATE TABLE t (c INT) COMMENT='中文注';"},
		{"3-column-missing", "CREATE TABLE t (c INT);"},
		{"4-column-present", "CREATE TABLE t (c INT COMMENT '中文注');"},
		{"5-length-at", "CREATE TABLE t (c INT) COMMENT='12345678';"},
		{"6-length-above", "CREATE TABLE t (c INT) COMMENT='123456789';"},
		{"7-length-multibyte", "CREATE TABLE t (c INT) COMMENT='中文注';"},
		{"8-length-off", "CREATE TABLE t (c INT) COMMENT='123456789';"},
		// probe-only adjacent observations: empty, whitespace, escape, numeric
		{"P-empty-table", "CREATE TABLE t (c INT) COMMENT='';"},
		{"P-empty-column", "CREATE TABLE t (c INT COMMENT '');"},
		{"P-space-table", "CREATE TABLE t (c INT) COMMENT='   ';"},
		{"P-space-column", "CREATE TABLE t (c INT COMMENT '   ');"},
		{"P-escape-table", "CREATE TABLE t (c INT) COMMENT='it''s ok';"},
		{"P-escape-column", "CREATE TABLE t (c INT COMMENT 'it''s ok');"},
		{"P-bslash-table", "CREATE TABLE t (c INT) COMMENT='a\\nb';"},
		{"P-bslash-column", "CREATE TABLE t (c INT COMMENT 'a\\nb');"},
	}
	for _, in := range inputs {
		o := obs{ID: in.id, SQL: in.sql, ColComment: map[string]string{}, ColCommentInfo: map[string]string{}, ASTColOpts: map[string][]string{}}
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
				for _, to := range ct.Options {
					o.ASTTableOpts = append(o.ASTTableOpts, fmt.Sprintf("%T:%d str=%q", to, int(to.Tp), to.StrValue))
					if to.Tp == tidbast.TableOptionComment {
						o.StrValComment = to.StrValue
					}
				}
				for _, cd := range ct.Cols {
					for _, co := range cd.Options {
						o.ASTColOpts[cd.Name.Name.L] = append(o.ASTColOpts[cd.Name.Name.L], fmt.Sprintf("%d str=%q expr=%T", int(co.Tp), co.StrValue, co.Expr))
					}
				}
			}
			st, err := wrapped[i].Extractor.Extract(spec.DialectMySQL, in.sql)
			if err != nil {
				o.ParseError = "extract: " + err.Error()
				break
			}
			if st.DDL != nil && st.DDL.Table != nil {
				o.TableComment = st.DDL.Table.Comment
				o.TableCommentB = len(o.TableComment)
				o.TableCommentR = utf8.RuneCountInString(o.TableComment)
				v, present := st.DDL.Options["comment"]
				o.OptComment = v
				o.OptCommentOK = present
				for _, c := range st.DDL.Columns {
					o.ColComment[c.Name] = c.Comment
					o.ColCommentInfo[c.Name] = fmt.Sprintf("%d/%d", len(c.Comment), utf8.RuneCountInString(c.Comment))
				}
				o.Unextracted = st.DDL.UnextractedOptions
			}
		}
		emit(o)
	}
}

func emit(v any) {
	b, _ := json.Marshal(v)
	fmt.Println(string(b))
}
