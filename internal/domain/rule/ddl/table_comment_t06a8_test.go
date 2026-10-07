// Package ddl verifies the T06-A8 comment contracts at the rule layer.
// input: synthetic create-table statements and per-rule policy values
// output: code-point length boundaries, per-field presence semantics, constructor guards
// pos: domain DDL rule test coverage for comment declaration fidelity (issue #85 T06-A8)
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestT06A8TableCommentMaxLengthCountsCodePoints(t *testing.T) {
	t.Parallel()
	statementRule, err := newTableCommentMaxLengthRule(policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params:  map[string]any{"limit": 8},
	})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	cases := []struct {
		name        string
		comment     string
		wantFinding bool
		wantActual  int
	}{
		{"ascii below", "1234567", false, 0},
		{"ascii at limit", "12345678", false, 0},
		{"ascii above", "123456789", true, 9},
		{"multibyte below", "中文注", false, 0},
		{"multibyte at limit", "中文注中文注ab", false, 0},
		{"multibyte above", "中文注中文注abc", true, 9},
		// Combining sequences count code points, not grapheme clusters:
		// "e" + combining acute (U+0301) is two runes for one glyph, so
		// four visible glyphs are eight runes and stay at the limit while
		// eight glyphs are sixteen runes and reject.
		{"combining runes at limit", "e\u0301e\u0301e\u0301e\u0301", false, 0},
		{"combining runes above", "e\u0301e\u0301e\u0301e\u0301e\u0301e\u0301e\u0301e\u0301", true, 16},
		// Whitespace is content: the rule never trims before counting.
		{"padding counted", "    pad  ", true, 9},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings, err := statementRule.Evaluate(context.Background(), tableOptionStatement(func(ddl *spec.DDL) {
				ddl.Table.Comment = tc.comment
			}))
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			if tc.wantFinding != (len(findings) == 1) {
				t.Fatalf("comment %q: findings=%d wantFinding=%v", tc.comment, len(findings), tc.wantFinding)
			}
			if tc.wantFinding {
				metadata := findings[0].Metadata
				if metadata["actual"] != tc.wantActual || metadata["limit"] != 8 || metadata["table"] != "users" {
					t.Fatalf("comment %q: metadata=%v", tc.comment, metadata)
				}
			}
		})
	}
}

func TestT06A8TableCommentMaxLengthConstructorBounds(t *testing.T) {
	t.Parallel()
	for _, limit := range []int{0, -1} {
		if _, err := newTableCommentMaxLengthRule(policy.RulePolicy{
			Enabled: true,
			Params:  map[string]any{"limit": limit},
		}); err == nil {
			t.Fatalf("limit=%d must fail construction", limit)
		}
	}
	if _, err := newTableCommentMaxLengthRule(policy.RulePolicy{}); err != nil {
		t.Fatalf("default limit construction: %v", err)
	}
}

func TestT06A8RequireRulesReadSeparateFields(t *testing.T) {
	t.Parallel()
	tableRule, err := newTableCommentRequiredRule(policy.RulePolicy{
		Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{"required": true},
	})
	if err != nil {
		t.Fatalf("table rule: %v", err)
	}
	columnRule, err := newColumnCommentRequiredRule(policy.RulePolicy{
		Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{"required": true},
	})
	if err != nil {
		t.Fatalf("column rule: %v", err)
	}

	// A table comment never satisfies the column rule: two columns, only
	// the un-commented one is reported, on its own field.
	statement := tableOptionStatement(func(ddl *spec.DDL) {
		ddl.Table.Comment = "documented"
		ddl.Columns = []spec.Column{
			{Name: "ok", Comment: "noted"},
			{Name: "bare"},
			{Name: "blank", Comment: "   "},
		}
	})
	if findings, err := tableRule.Evaluate(context.Background(), statement); err != nil || len(findings) != 0 {
		t.Fatalf("table require with comment: findings=%v err=%v", findings, err)
	}
	findings, err := columnRule.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("column evaluate: %v", err)
	}
	if len(findings) != 2 {
		t.Fatalf("column require: findings=%v want bare+blank", findings)
	}
	if findings[0].Metadata["column"] != "bare" || findings[1].Metadata["column"] != "blank" {
		t.Fatalf("column metadata = %v / %v", findings[0].Metadata, findings[1].Metadata)
	}

	// And a column comment never satisfies the table rule.
	statement.DDL.Table.Comment = " "
	findings, err = tableRule.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("table evaluate: %v", err)
	}
	if len(findings) != 1 || findings[0].Metadata["table"] != "users" {
		t.Fatalf("table require: findings=%v", findings)
	}
}
