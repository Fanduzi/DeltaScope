// Package tidb verifies the T06-A14 extracted table-option facts the three
// allowlist policies consume.
package tidbparser

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	"github.com/pingcap/tidb/pkg/parser/ast"
)

const t06A14OmittedSQL = "CREATE TABLE t (c INT);"
const t06A14EngineAllowedSQL = "CREATE TABLE t (c INT) ENGINE=InnoDB;"
const t06A14EngineDeniedSQL = "CREATE TABLE t (c INT) ENGINE=MyISAM;"
const t06A14CharsetAllowedSQL = "CREATE TABLE t (c INT) DEFAULT CHARSET=utf8mb4;"
const t06A14CharsetDeniedSQL = "CREATE TABLE t (c INT) DEFAULT CHARSET=latin1;"
const t06A14RowFormatAllowedSQL = "CREATE TABLE t (c INT) ROW_FORMAT=DYNAMIC;"
const t06A14RowFormatDeniedSQL = "CREATE TABLE t (c INT) ROW_FORMAT=COMPACT;"
const t06A14ColumnCharsetSQL = "CREATE TABLE t (c VARCHAR(8) CHARACTER SET utf8mb4);"

// t06A14OptionCase pins one frozen input: its AST payload kind and the exact
// spec.Options content after extraction.
type t06A14OptionCase struct {
	name      string
	sql       string
	astTp     ast.TableOptionType
	strValue  string // engine/charset carry the declared text verbatim
	uintValue uint64 // row_format carries the enum, not text
	options   map[string]string
}

var t06A14OptionCases = []t06A14OptionCase{
	{name: "s0-omitted", sql: t06A14OmittedSQL, astTp: 0, options: nil},
	{name: "engine-allowed", sql: t06A14EngineAllowedSQL, astTp: ast.TableOptionEngine,
		strValue: "InnoDB", options: map[string]string{"engine": "InnoDB"}},
	{name: "engine-denied", sql: t06A14EngineDeniedSQL, astTp: ast.TableOptionEngine,
		strValue: "MyISAM", options: map[string]string{"engine": "MyISAM"}},
	{name: "charset-allowed", sql: t06A14CharsetAllowedSQL, astTp: ast.TableOptionCharset,
		strValue: "utf8mb4", options: map[string]string{"charset": "utf8mb4"}},
	{name: "charset-denied", sql: t06A14CharsetDeniedSQL, astTp: ast.TableOptionCharset,
		strValue: "latin1", options: map[string]string{"charset": "latin1"}},
	{name: "rowformat-allowed", sql: t06A14RowFormatAllowedSQL, astTp: ast.TableOptionRowFormat,
		uintValue: uint64(ast.RowFormatDynamic), options: map[string]string{"row_format": "DYNAMIC"}},
	{name: "rowformat-denied", sql: t06A14RowFormatDeniedSQL, astTp: ast.TableOptionRowFormat,
		uintValue: uint64(ast.RowFormatCompact), options: map[string]string{"row_format": "COMPACT"}},
}

// TestExtractorT06A14TableOptions pins the declared option facts per input:
// the AST payload kind, the verbatim StrValue for engine/charset, the
// UintValue enum for row_format and its canonical name, and — for the
// omitted input — every target key absent (never a server default).
func TestExtractorT06A14TableOptions(t *testing.T) {
	t.Parallel()
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		for _, tc := range t06A14OptionCases {
			tc := tc
			t.Run(string(dialect)+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				create, statement := parseExtractT06A12(t, dialect, tc.sql)
				if tc.astTp == 0 {
					if len(create.Options) != 0 {
						t.Fatalf("ast options = %#v, want none", create.Options)
					}
				} else {
					if len(create.Options) != 1 || create.Options[0].Tp != tc.astTp {
						t.Fatalf("ast options = %#v, want one %v", create.Options, tc.astTp)
					}
					if create.Options[0].StrValue != tc.strValue ||
						create.Options[0].UintValue != tc.uintValue {
						t.Fatalf("ast option payload = %q/%d, want %q/%d",
							create.Options[0].StrValue, create.Options[0].UintValue,
							tc.strValue, tc.uintValue)
					}
				}
				if len(statement.DDL.Options) != len(tc.options) {
					t.Fatalf("spec.Options = %#v, want %#v", statement.DDL.Options, tc.options)
				}
				for key, want := range tc.options {
					if got := statement.DDL.Options[key]; got != want {
						t.Fatalf("spec.Options[%q] = %q, want %q", key, got, want)
					}
				}
				for _, key := range []string{"engine", "charset", "row_format"} {
					if _, want := tc.options[key]; !want {
						if _, present := statement.DDL.Options[key]; present {
							t.Fatalf("spec.Options[%q] present on %s — undeclared keys stay absent", key, tc.name)
						}
					}
				}
				if len(statement.DDL.UnextractedOptions) != 0 {
					t.Fatalf("unextracted options = %#v, want none", statement.DDL.UnextractedOptions)
				}
			})
		}
	}
}

// TestExtractorT06A14ColumnCharsetNotTableOption pins the scope boundary the
// charset allowlist consumes: a column-level `CHARACTER SET` declaration
// lands on Column.Charset and never fabricates the table-level
// Options["charset"] key.
func TestExtractorT06A14ColumnCharsetNotTableOption(t *testing.T) {
	t.Parallel()
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			create, statement := parseExtractT06A12(t, dialect, t06A14ColumnCharsetSQL)
			if len(create.Options) != 0 {
				t.Fatalf("ast table options = %#v, want none — column charset is not a table option", create.Options)
			}
			if _, present := statement.DDL.Options["charset"]; present {
				t.Fatalf("spec.Options = %#v carries charset — column charset must not fabricate a table option", statement.DDL.Options)
			}
			if len(statement.DDL.Columns) != 1 || statement.DDL.Columns[0].Charset != "utf8mb4" {
				t.Fatalf("spec columns = %#v, want c charset=utf8mb4", statement.DDL.Columns)
			}
		})
	}
}
