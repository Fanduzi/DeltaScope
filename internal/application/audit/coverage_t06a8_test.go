// Package audit verifies the T06-A8 comment fidelity upgrade: column COMMENT
// values keep the parser-decoded content without literal quote wrappers, the
// table comment length policy counts Unicode code points, and the
// CREATE-derived table snapshot carries the declared table comment.
// input: CREATE TABLE batches with declared table/column comments through AuditSQL plus the enrichment seam
// output: exact per-rule findings, rune-counted comment lengths, preserved comment facts in derived state
// pos: issue #85 T06-A8 audit-layer contract tests
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"reflect"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

const (
	t06A8TableCommentRule  = "ddl.table.comment.require"
	t06A8ColumnCommentRule = "ddl.column.comment.require"
	t06A8TableCommentLen   = "ddl.table.comment.max_length"
)

// t06A8Audit runs one AuditSQL request under an isolated policy file.
func t06A8Audit(t *testing.T, dialect spec.Dialect, sql, policy string) report.Result {
	t.Helper()
	result, err := AuditSQL(context.Background(), Request{
		SQL: sql, Dialect: dialect, Schema: "golden", ConfigPath: policy,
	})
	if err != nil {
		t.Fatalf("%s: unexpected error %v", dialect, err)
	}
	return result
}

// t06A8Policy enables exactly the named rules at blocker with the given
// params blocks; everything else stays disabled.
func t06A8Policy(t *testing.T, enabled map[string]string) string {
	t.Helper()
	return t05PolicyPath(t, enabled)
}

// t06A8WantFinding pins exactly one blocker finding on statement 0 with the
// expected rule ID, message, and metadata.
func t06A8WantFinding(t *testing.T, result report.Result, ruleID, message string, metadata map[string]any) {
	t.Helper()
	if result.Verdict != report.VerdictReject || result.Coverage.Status != report.CoverageComplete {
		t.Fatalf("aggregate = %s/%s, want reject/complete", result.Verdict, result.Coverage.Status)
	}
	if len(result.Unsupported) != 0 {
		t.Fatalf("unsupported = %#v, want none", result.Unsupported)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("statements = %d, want 1", len(result.Statements))
	}
	statement := result.Statements[0]
	if statement.Index != 0 || statement.Kind != "ddl" {
		t.Fatalf("statement identity = index %d kind %s, want 0/ddl", statement.Index, statement.Kind)
	}
	if len(statement.Findings) != 1 {
		t.Fatalf("findings = %#v, want exactly 1", statement.Findings)
	}
	finding := statement.Findings[0]
	if finding.RuleID != ruleID || finding.Level != "blocker" {
		t.Fatalf("finding = %s/%s, want %s blocker", finding.RuleID, finding.Level, ruleID)
	}
	if finding.Message != message {
		t.Fatalf("message = %q, want %q", finding.Message, message)
	}
	if !reflect.DeepEqual(finding.Metadata, metadata) {
		t.Fatalf("metadata = %#v, want %#v", finding.Metadata, metadata)
	}
	if finding.Location == nil || finding.Location.Line != 1 || finding.Location.Column != 1 {
		t.Fatalf("location = %+v, want 1:1", finding.Location)
	}
}

// t06A8WantPass pins a complete, clean pass on a single statement.
func t06A8WantPass(t *testing.T, result report.Result) {
	t.Helper()
	if result.Verdict != report.VerdictPass || result.Coverage.Status != report.CoverageComplete {
		t.Fatalf("aggregate = %s/%s, want pass/complete", result.Verdict, result.Coverage.Status)
	}
	if len(result.Unsupported) != 0 {
		t.Fatalf("unsupported = %#v, want none", result.Unsupported)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("statements = %d, want 1", len(result.Statements))
	}
	statement := result.Statements[0]
	if statement.Coverage.Status != report.CoverageComplete {
		t.Fatalf("statement coverage = %s, want complete", statement.Coverage.Status)
	}
	if len(statement.Findings) != 0 {
		t.Fatalf("findings = %#v, want none", statement.Findings)
	}
}

// TestT06A8ColumnEmptyCommentRequired is the first red for production change
// A: an explicitly empty column comment is a missing comment — the stored
// fact must not carry literal quote wrappers that fool the TrimSpace check.
func TestT06A8ColumnEmptyCommentRequired(t *testing.T) {
	policy := t06A8Policy(t, map[string]string{
		t06A8ColumnCommentRule: "      required: true\n",
	})
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect)+"/empty", func(t *testing.T) {
			t06A8WantFinding(t, t06A8Audit(t, dialect,
				"CREATE TABLE t (c INT COMMENT '');", policy),
				t06A8ColumnCommentRule,
				`column "c" must include a comment`,
				map[string]any{"table": "t", "column": "c"})
		})
		t.Run(string(dialect)+"/blank", func(t *testing.T) {
			t06A8WantFinding(t, t06A8Audit(t, dialect,
				"CREATE TABLE t (c INT COMMENT '   ');", policy),
				t06A8ColumnCommentRule,
				`column "c" must include a comment`,
				map[string]any{"table": "t", "column": "c"})
		})
	}
}

// TestT06A8TableCommentRuneLength is the first red for production change B:
// the declared comment '中文注' is three Unicode code points, not nine bytes.
func TestT06A8TableCommentRuneLength(t *testing.T) {
	policy := t06A8Policy(t, map[string]string{
		t06A8TableCommentLen: "      limit: 8\n",
	})
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect), func(t *testing.T) {
			t06A8WantPass(t, t06A8Audit(t, dialect,
				"CREATE TABLE t (c INT) COMMENT='中文注';", policy))
		})
	}
}

// TestT06A8DerivedStateCarriesTableComment is the first red for production
// change C: the CREATE-derived prospective snapshot must carry the declared
// table comment on Table.Comment, while Options["comment"] keeps its value
// and column comments stay decoded facts.
func TestT06A8DerivedStateCarriesTableComment(t *testing.T) {
	sql := "CREATE TABLE t (c INT COMMENT '列注') COMMENT='表注';\nALTER TABLE t ADD COLUMN d INT;"
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect), func(t *testing.T) {
			enriched := enrichA3(t, sql, dialect, &t05AbsentProvider{})
			preAlter := enriched[1].Metadata.TargetTable
			if preAlter == nil || !preAlter.Exists {
				t.Fatalf("pre-alter state = %+v, want present derived table", preAlter)
			}
			if preAlter.Table == nil || preAlter.Table.Comment != "表注" {
				t.Fatalf("derived Table.Comment = %+v, want 表注", preAlter.Table)
			}
			if got := preAlter.Options["comment"]; got != "表注" {
				t.Fatalf("derived Options[comment] = %q, want 表注", got)
			}
			column := preAlter.FindColumn("c")
			if column == nil || column.Comment != "列注" {
				t.Fatalf("derived column comment = %+v, want decoded 列注", column)
			}
		})
	}
}

// TestT06A8RequireIsolation is group 2: the two require rules keep separate
// ownership — a table comment cannot satisfy the column rule, a column
// comment cannot satisfy the table rule, required=false and disabled stay
// silent, and per-target attribution stays exact across multiple columns and
// statements.
func TestT06A8RequireIsolation(t *testing.T) {
	tablePolicy := t06A8Policy(t, map[string]string{
		t06A8TableCommentRule: "      required: true\n",
	})
	columnPolicy := t06A8Policy(t, map[string]string{
		t06A8ColumnCommentRule: "      required: true\n",
	})
	optionalPolicy := t06A8Policy(t, map[string]string{
		t06A8TableCommentRule:  "      required: false\n",
		t06A8ColumnCommentRule: "      required: false\n",
	})
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect)+"/table_comment_not_column", func(t *testing.T) {
			// Table comment present, column comment absent: the column rule
			// must still fire — fields are independent.
			t06A8WantFinding(t, t06A8Audit(t, dialect,
				"CREATE TABLE t (c INT) COMMENT='表注';", columnPolicy),
				t06A8ColumnCommentRule,
				`column "c" must include a comment`,
				map[string]any{"table": "t", "column": "c"})
		})
		t.Run(string(dialect)+"/column_comment_not_table", func(t *testing.T) {
			// Column comment present, table comment absent: the table rule
			// must still fire.
			t06A8WantFinding(t, t06A8Audit(t, dialect,
				"CREATE TABLE t (c INT COMMENT '列注');", tablePolicy),
				t06A8TableCommentRule,
				"table comment is required",
				map[string]any{"table": "t"})
		})
		t.Run(string(dialect)+"/table_empty_and_blank", func(t *testing.T) {
			for _, sql := range []string{
				"CREATE TABLE t (c INT) COMMENT='';",
				"CREATE TABLE t (c INT) COMMENT='   ';",
			} {
				t06A8WantFinding(t, t06A8Audit(t, dialect, sql, tablePolicy),
					t06A8TableCommentRule,
					"table comment is required",
					map[string]any{"table": "t"})
			}
		})
		t.Run(string(dialect)+"/required_false_silent", func(t *testing.T) {
			t06A8WantPass(t, t06A8Audit(t, dialect,
				"CREATE TABLE t (c INT);", optionalPolicy))
		})
		t.Run(string(dialect)+"/disabled_silent", func(t *testing.T) {
			offPolicy := t05PolicyPath(t, map[string]string{})
			t06A8WantPass(t, t06A8Audit(t, dialect,
				"CREATE TABLE t (c INT);", offPolicy))
		})
		t.Run(string(dialect)+"/multi_column_attribution", func(t *testing.T) {
			result := t06A8Audit(t, dialect,
				"CREATE TABLE t (a INT, b INT COMMENT 'b注', c INT);", columnPolicy)
			if result.Verdict != report.VerdictReject {
				t.Fatalf("verdict = %s, want reject", result.Verdict)
			}
			findings := result.Statements[0].Findings
			if len(findings) != 2 {
				t.Fatalf("findings = %#v, want exactly 2 (a and c)", findings)
			}
			for i, column := range []string{"a", "c"} {
				f := findings[i]
				if f.RuleID != t06A8ColumnCommentRule ||
					f.Message != `column "`+column+`" must include a comment` ||
					!reflect.DeepEqual(f.Metadata, map[string]any{"table": "t", "column": column}) {
					t.Fatalf("finding %d = %#v, want column %q require blocker", i, f, column)
				}
			}
		})
		t.Run(string(dialect)+"/multi_statement_attribution", func(t *testing.T) {
			result := t06A8Audit(t, dialect,
				"CREATE TABLE a (c INT COMMENT 'x');\nCREATE TABLE b (c INT);", columnPolicy)
			if len(result.Statements) != 2 {
				t.Fatalf("statements = %d, want 2", len(result.Statements))
			}
			if len(result.Statements[0].Findings) != 0 {
				t.Fatalf("statement 0 findings = %#v, want none", result.Statements[0].Findings)
			}
			second := result.Statements[1]
			if len(second.Findings) != 1 || second.Findings[0].RuleID != t06A8ColumnCommentRule ||
				second.Findings[0].Location.Line != 2 {
				t.Fatalf("statement 1 findings = %#v, want one column require at line 2", second.Findings)
			}
		})
	}
}

// TestT06A8CommentLengthBoundaries is group 3: the table comment limit counts
// Unicode code points, includes untrimmed edge spaces, and a small combining
// control proves code points — not grapheme clusters — are the unit.
func TestT06A8CommentLengthBoundaries(t *testing.T) {
	policy := t06A8Policy(t, map[string]string{
		t06A8TableCommentLen: "      limit: 8\n",
	})
	cases := []struct {
		name       string
		sql        string
		want       string // "pass" or "reject"
		wantActual int    // metadata.actual when rejecting
	}{
		{"ascii_at", "CREATE TABLE t (c INT) COMMENT='12345678';", "pass", 0},
		{"ascii_above", "CREATE TABLE t (c INT) COMMENT='123456789';", "reject", 9},
		{"multibyte_3", "CREATE TABLE t (c INT) COMMENT='\u4e2d\u6587\u6ce8';", "pass", 0},
		{"multibyte_at_8", "CREATE TABLE t (c INT) COMMENT='\u4e2d\u6587\u6ce8\u4e2d\u6587\u6ce8ab';", "pass", 0},
		{"multibyte_above_9", "CREATE TABLE t (c INT) COMMENT='\u4e2d\u6587\u6ce8\u4e2d\u6587\u6ce8abc';", "reject", 9},
		{"untrimmed_edges", "CREATE TABLE t (c INT) COMMENT='  12345678  ';", "reject", 12},
		// e + combining acute (U+0301) is ONE grapheme but TWO code points:
		// 2+6+1=9 runes vs 8 graphemes — only the rune count exceeds the limit.
		{"combining_rune_not_grapheme", "CREATE TABLE t (c INT) COMMENT='e\u0301\u4e2d\u6587\u6ce8\u4e2d\u6587\u6ce8a';", "reject", 9},
	}
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		for _, tc := range cases {
			t.Run(string(dialect)+"/"+tc.name, func(t *testing.T) {
				result := t06A8Audit(t, dialect, tc.sql, policy)
				switch tc.want {
				case "pass":
					t06A8WantPass(t, result)
				case "reject":
					t06A8WantFinding(t, result, t06A8TableCommentLen,
						"table comment must not exceed 8 characters",
						map[string]any{"table": "t", "limit": 8, "actual": tc.wantActual})
				}
			})
		}
	}
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect)+"/actual_is_runes", func(t *testing.T) {
			result := t06A8Audit(t, dialect,
				"CREATE TABLE t (c INT) COMMENT='\u4e2d\u6587\u6ce8\u4e2d\u6587\u6ce8abc';", policy)
			f := result.Statements[0].Findings[0]
			if !reflect.DeepEqual(f.Metadata, map[string]any{"table": "t", "limit": 8, "actual": 9}) {
				t.Fatalf("metadata = %#v, want rune-counted actual=9 limit=8", f.Metadata)
			}
		})
		t.Run(string(dialect)+"/invalid_limit_still_rejected", func(t *testing.T) {
			bad := t06A8Policy(t, map[string]string{
				t06A8TableCommentLen: "      limit: 0\n",
			})
			if _, err := AuditSQL(context.Background(), Request{
				SQL: "CREATE TABLE t (c INT) COMMENT='x';", Dialect: dialect,
				Schema: "golden", ConfigPath: bad,
			}); err == nil {
				t.Fatalf("limit=0 policy accepted, want constructor error")
			}
		})
	}
}

// TestT06A8DerivedStateBoundaries is group 4: the comment facts on the
// CREATE-derived snapshot are copied without mutating provider facts or the
// extracted statement, and conditional/invalidation semantics stay unchanged.
func TestT06A8DerivedStateBoundaries(t *testing.T) {
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect)+"/if_not_exists_keeps_old_comment", func(t *testing.T) {
			// t exists in provider state with its own comment; a CREATE IF NOT
			// EXISTS must not overwrite it.
			provider := &t05A8Provider{snapshots: map[string]*spec.TableSnapshot{
				"golden.t": {
					Exists: true,
					Table:  &spec.Table{Schema: "golden", Name: "t", Comment: "旧注"},
					Columns: []spec.Column{
						{Name: "c", Type: "int", NotNull: true},
					},
				},
			}}
			enriched := enrichA3(t,
				"CREATE TABLE IF NOT EXISTS t (c INT COMMENT '新列') COMMENT='新注';\nALTER TABLE t ADD COLUMN d INT;",
				dialect, provider)
			preAlter := enriched[1].Metadata.TargetTable
			if preAlter == nil || preAlter.Table == nil || preAlter.Table.Comment != "旧注" {
				t.Fatalf("IF NOT EXISTS pre-state comment = %+v, want provider 旧注 kept", preAlter.Table)
			}
		})
		t.Run(string(dialect)+"/unknown_target_not_published", func(t *testing.T) {
			// IF NOT EXISTS on an unknown target yields present-incomplete —
			// the declared comment must not be published onto unknown shapes.
			enriched := enrichA3(t,
				"CREATE TABLE IF NOT EXISTS t (c INT) COMMENT='新注';\nALTER TABLE t ADD COLUMN d INT;",
				dialect, &t06A8NilProvider{})
			preAlter := enriched[1].Metadata.TargetTable
			if preAlter == nil || !preAlter.Exists {
				t.Fatalf("pre-alter state = %+v, want present-incomplete", preAlter)
			}
			if preAlter.Table != nil && preAlter.Table.Comment != "" {
				t.Fatalf("unknown-shape Table.Comment = %q, want unpublished", preAlter.Table.Comment)
			}
		})
		t.Run(string(dialect)+"/if_not_exists_absent_derives_comment", func(t *testing.T) {
			// Known-absent + IF NOT EXISTS deterministically creates the
			// declared shape — the comment belongs to the derived state.
			enriched := enrichA3(t,
				"CREATE TABLE IF NOT EXISTS t (c INT) COMMENT='新注';\nALTER TABLE t ADD COLUMN d INT;",
				dialect, &t05AbsentProvider{})
			preAlter := enriched[1].Metadata.TargetTable
			if preAlter == nil || !preAlter.Exists || preAlter.Table == nil || preAlter.Table.Comment != "新注" {
				t.Fatalf("known-absent IF NOT EXISTS derived comment = %+v, want 新注", preAlter)
			}
		})
		t.Run(string(dialect)+"/recreate_no_old_comment", func(t *testing.T) {
			// DROP + CREATE must not resurrect the previous comment.
			provider := &t05A8Provider{snapshots: map[string]*spec.TableSnapshot{
				"golden.t": {
					Exists: true,
					Table:  &spec.Table{Schema: "golden", Name: "t", Comment: "旧注"},
					Columns: []spec.Column{
						{Name: "c", Type: "int", NotNull: true},
					},
				},
			}}
			enriched := enrichA3(t,
				"DROP TABLE t;\nCREATE TABLE t (c INT);\nALTER TABLE t ADD COLUMN d INT;",
				dialect, provider)
			preAlter := enriched[2].Metadata.TargetTable
			if preAlter == nil || !preAlter.Exists || preAlter.Table == nil {
				t.Fatalf("pre-alter state = %+v, want present derived table", preAlter)
			}
			if preAlter.Table.Comment != "" {
				t.Fatalf("recreated Table.Comment = %q, want empty (no resurrection)", preAlter.Table.Comment)
			}
		})
		t.Run(string(dialect)+"/statement_facts_not_mutated", func(t *testing.T) {
			// The extracted DDL keeps its own comment fields; enrichment must
			// not rewrite them while building snapshots.
			enriched := enrichA3(t,
				"CREATE TABLE t (c INT COMMENT '列注') COMMENT='表注';\nALTER TABLE t ADD COLUMN d INT;",
				dialect, &t05AbsentProvider{})
			ddl := enriched[0].DDL
			if ddl.Table.Comment != "表注" || ddl.Options["comment"] != "表注" || ddl.Columns[0].Comment != "列注" {
				t.Fatalf("extracted facts mutated: table=%q opt=%q col=%q",
					ddl.Table.Comment, ddl.Options["comment"], ddl.Columns[0].Comment)
			}
		})
	}
}

// TestT06A8OtherSourcesUnchanged is group 5: DEFAULT literal quoting, typed
// DEFAULT NULL recognition, repeated DEFAULT handling, and CURRENT_TIMESTAMP
// facts keep their accepted A3/A4 contract — the comment decoding change
// never touched normalizedExprText or its consumers.
func TestT06A8OtherSourcesUnchanged(t *testing.T) {
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect)+"/default_literals_keep_quotes", func(t *testing.T) {
			enriched := enrichA3(t,
				"CREATE TABLE t (a VARCHAR(8) DEFAULT 'x', b VARCHAR(8) DEFAULT 'NULL', c VARCHAR(8) DEFAULT NULL, d TIMESTAMP DEFAULT CURRENT_TIMESTAMP);",
				dialect, &t05AbsentProvider{})
			ddl := enriched[0].DDL
			a, b, c, d := ddl.Columns[0], ddl.Columns[1], ddl.Columns[2], ddl.Columns[3]
			if a.DefaultValue != "'x'" || b.DefaultValue != "'NULL'" {
				t.Fatalf("literal defaults = %q/%q, want quoted text kept", a.DefaultValue, b.DefaultValue)
			}
			if !c.HasDefault || !c.DefaultIsNull || c.DefaultValue != "NULL" {
				t.Fatalf("typed NULL default = %+v, want HasDefault+DefaultIsNull+NULL", c)
			}
			if !d.DefaultIsCurrentTimestamp || d.DefaultIsNull {
				t.Fatalf("CURRENT_TIMESTAMP = %+v, want timestamp default non-null", d)
			}
		})
		t.Run(string(dialect)+"/comment_null_literal_is_content", func(t *testing.T) {
			enriched := enrichA3(t,
				"CREATE TABLE t (c INT COMMENT 'NULL');",
				dialect, &t05AbsentProvider{})
			c := enriched[0].DDL.Columns[0]
			if c.Comment != "NULL" || c.HasDefault {
				t.Fatalf("COMMENT 'NULL' = comment %q hasDefault %v, want plain content no default", c.Comment, c.HasDefault)
			}
		})
	}
}

// t06A8NilProvider reports "not consulted / unknown" by returning a nil
// snapshot — distinct from the absent provider's known-absent answer.
type t06A8NilProvider struct{}

func (p *t06A8NilProvider) LoadInstanceFacts(context.Context, spec.Dialect, string) (*spec.InstanceFacts, error) {
	return &spec.InstanceFacts{}, nil
}

func (p *t06A8NilProvider) LoadTableSnapshot(_ context.Context, _ spec.Dialect, schema, table string) (*spec.TableSnapshot, error) {
	return nil, nil
}
