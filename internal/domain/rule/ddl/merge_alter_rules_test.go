// Package ddl verifies global merge-alter governance rules.
// input: statement batches with repeated alter-table operations on the same target
// output: coverage for dialect-specific merge-alter findings
// pos: DDL global-rule test coverage for cross-statement alter governance
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestMergeAlterRuleFindsRepeatedMySQLAlterTargets(t *testing.T) {
	t.Parallel()
	globalRule, err := newMergeAlterRule(ruleIDAlterMergeMySQLRequire, spec.DialectMySQL, rule.LevelWarning, policy.RulePolicy{
		Enabled: true,
		Params:  map[string]any{"required": true},
	})
	if err != nil {
		t.Fatalf("new mysql merge alter rule: %v", err)
	}

	findings, err := globalRule.EvaluateAll(context.Background(), []spec.Statement{
		{Kind: spec.KindDDL, Dialect: spec.DialectMySQL, DDL: &spec.DDL{Operation: spec.DDLOperationAlterTable, Table: &spec.Table{Name: "users"}, Alter: []spec.Alter{{Action: "drop_column", Name: "age"}}}},
		{Kind: spec.KindDDL, Dialect: spec.DialectMySQL, DDL: &spec.DDL{Operation: spec.DDLOperationAlterTable, Table: &spec.Table{Name: "users"}, Alter: []spec.Alter{{Action: "drop_index", Name: "idx_age"}}}},
	})
	if err != nil {
		t.Fatalf("evaluate merge alter rule: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected one merge-alter finding, got %d", len(findings))
	}
}

func TestMergeAlterRuleSkipsDifferentDialectAndSingleAlter(t *testing.T) {
	t.Parallel()
	globalRule, err := newMergeAlterRule(ruleIDAlterMergeTiDBRequire, spec.DialectTiDB, rule.LevelWarning, policy.RulePolicy{
		Enabled: true,
		Params:  map[string]any{"required": true},
	})
	if err != nil {
		t.Fatalf("new tidb merge alter rule: %v", err)
	}

	findings, err := globalRule.EvaluateAll(context.Background(), []spec.Statement{
		{Kind: spec.KindDDL, Dialect: spec.DialectMySQL, DDL: &spec.DDL{Operation: spec.DDLOperationAlterTable, Table: &spec.Table{Name: "users"}, Alter: []spec.Alter{{Action: "drop_column", Name: "age"}}}},
		{Kind: spec.KindDDL, Dialect: spec.DialectTiDB, DDL: &spec.DDL{Operation: spec.DDLOperationAlterTable, Table: &spec.Table{Name: "orders"}, Alter: []spec.Alter{{Action: "drop_column", Name: "legacy"}}}},
	})
	if err != nil {
		t.Fatalf("evaluate merge alter rule: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no merge-alter finding, got %d", len(findings))
	}
}
