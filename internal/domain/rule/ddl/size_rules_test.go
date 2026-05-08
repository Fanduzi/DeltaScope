// Package ddl verifies metadata-backed size-estimation rules.
// input: create-table statements plus instance facts for charset, row format, and index-length limits
// output: coverage for rough row-size and index-key-length findings
// pos: DDL metadata-backed size rule test coverage
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestTableRowSizeRuleFindsCompactRowOverflow(t *testing.T) {
	ruleUnderTest, err := newTableRowSizeRule(policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params:  map[string]any{"required": true, "requires_metadata": true},
	})
	if err != nil {
		t.Fatalf("new row-size rule: %v", err)
	}

	statement := spec.Statement{
		Kind: spec.KindDDL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationCreateTable,
			Table:     &spec.Table{Name: "users"},
			Options:   map[string]string{"engine": "InnoDB", "row_format": "COMPACT", "charset": "utf8mb4"},
			Columns: []spec.Column{
				{Name: "c1", Type: "varchar(1000)", Length: 1000, Charset: "utf8mb4"},
				{Name: "c2", Type: "varchar(1000)", Length: 1000, Charset: "utf8mb4"},
				{Name: "c3", Type: "varchar(1000)", Length: 1000, Charset: "utf8mb4"},
				{Name: "c4", Type: "varchar(1000)", Length: 1000, Charset: "utf8mb4"},
				{Name: "c5", Type: "varchar(1000)", Length: 1000, Charset: "utf8mb4"},
				{Name: "c6", Type: "varchar(1000)", Length: 1000, Charset: "utf8mb4"},
				{Name: "c7", Type: "varchar(1000)", Length: 1000, Charset: "utf8mb4"},
				{Name: "c8", Type: "varchar(1000)", Length: 1000, Charset: "utf8mb4"},
				{Name: "c9", Type: "varchar(1000)", Length: 1000, Charset: "utf8mb4"},
				{Name: "c10", Type: "varchar(1000)", Length: 1000, Charset: "utf8mb4"},
				{Name: "c11", Type: "varchar(1000)", Length: 1000, Charset: "utf8mb4"},
			},
		},
		Metadata: &spec.Metadata{
			Instance: &spec.InstanceFacts{DefaultCharset: "utf8mb4", InnoDBDefaultRowFormat: "compact"},
		},
	}

	findings, err := ruleUnderTest.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate row-size rule: %v", err)
	}
	if len(findings) == 0 || findings[0].RuleID != ruleIDTableRowSizeMaxBytesRequire {
		t.Fatalf("expected row-size finding, got %+v", findings)
	}
}

func TestIndexKeyLengthRuleFindsLargePrefixOverflow(t *testing.T) {
	ruleUnderTest, err := newIndexKeyLengthRule(policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params:  map[string]any{"required": true, "requires_metadata": true},
	})
	if err != nil {
		t.Fatalf("new key-length rule: %v", err)
	}

	statement := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectMySQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationCreateTable,
			Table:     &spec.Table{Name: "users"},
			Options:   map[string]string{"engine": "InnoDB", "charset": "utf8mb4"},
			Columns: []spec.Column{
				{Name: "email", Type: "varchar(300)", Length: 300, Charset: "utf8mb4"},
				{Name: "tenant_code", Type: "varchar(300)", Length: 300, Charset: "utf8mb4"},
			},
			Indexes: []spec.Index{
				{Name: "idx_email_tenant", Kind: spec.IndexKindSecondary, Columns: []string{"email", "tenant_code"}},
			},
		},
		Metadata: &spec.Metadata{
			Instance: &spec.InstanceFacts{
				Version:                  "5.7.35",
				DefaultCharset:           "utf8mb4",
				InnoDBLargePrefixEnabled: false,
			},
		},
	}

	findings, err := ruleUnderTest.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate key-length rule: %v", err)
	}
	if len(findings) != 1 || findings[0].RuleID != ruleIDIndexKeyLengthMaxBytesRequire {
		t.Fatalf("expected key-length finding, got %+v", findings)
	}
}

func TestSizeRulesSkipWithoutInstanceFacts(t *testing.T) {
	rowRule, err := newTableRowSizeRule(policy.RulePolicy{Enabled: true, Params: map[string]any{"required": true}})
	if err != nil {
		t.Fatalf("new row-size rule: %v", err)
	}
	indexRule, err := newIndexKeyLengthRule(policy.RulePolicy{Enabled: true, Params: map[string]any{"required": true}})
	if err != nil {
		t.Fatalf("new key-length rule: %v", err)
	}

	statement := spec.Statement{
		Kind: spec.KindDDL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationCreateTable,
			Table:     &spec.Table{Name: "users"},
			Options:   map[string]string{"engine": "InnoDB"},
		},
	}

	if findings, err := rowRule.Evaluate(context.Background(), statement); err != nil || len(findings) != 0 {
		t.Fatalf("expected row-size rule to skip without metadata, got findings=%+v err=%v", findings, err)
	}
	if findings, err := indexRule.Evaluate(context.Background(), statement); err != nil || len(findings) != 0 {
		t.Fatalf("expected key-length rule to skip without metadata, got findings=%+v err=%v", findings, err)
	}
}
