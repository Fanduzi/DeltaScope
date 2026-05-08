// Package ddl verifies create-table option and shape rule behavior.
// input: synthetic create-table statements with options, constraints, and shape flags plus rule-specific policy overrides
// output: focused coverage for table comment, engine/charset, and object-shape findings
// pos: domain DDL rule test coverage for create-table option governance
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestTableCommentMaxLengthRuleFindsLongComments(t *testing.T) {
	t.Parallel()
	statementRule, err := newTableCommentMaxLengthRule(policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelWarning,
		Params:  map[string]any{"limit": 5},
	})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	findings, err := statementRule.Evaluate(context.Background(), tableOptionStatement(func(ddl *spec.DDL) {
		ddl.Table.Comment = "too long"
	}))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestTableEngineAllowlistRuleFindsDisallowedEngine(t *testing.T) {
	t.Parallel()
	statementRule, err := newTableOptionAllowlistRule(ruleIDTableEngineAllowlist, "engine", "engine", []string{"InnoDB"}, rule.LevelBlocker, policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params:  map[string]any{"values": []any{"InnoDB"}},
	})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	findings, err := statementRule.Evaluate(context.Background(), tableOptionStatement(func(ddl *spec.DDL) {
		ddl.Options["engine"] = "MyISAM"
	}))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestTableCharsetAllowlistRuleFindsMissingCharset(t *testing.T) {
	t.Parallel()
	statementRule, err := newTableOptionAllowlistRule(ruleIDTableCharsetAllowlist, "charset", "charset", []string{"utf8mb4"}, rule.LevelBlocker, policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params:  map[string]any{"values": []any{"utf8mb4"}},
	})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	findings, err := statementRule.Evaluate(context.Background(), tableOptionStatement(nil))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestTableRowFormatAllowlistRuleFindsDisallowedRowFormat(t *testing.T) {
	t.Parallel()
	statementRule, err := newTableOptionAllowlistRule(ruleIDTableRowFormatAllowlist, "row_format", "row format", []string{"DYNAMIC"}, rule.LevelBlocker, policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params:  map[string]any{"values": []any{"DYNAMIC"}},
	})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	findings, err := statementRule.Evaluate(context.Background(), tableOptionStatement(func(ddl *spec.DDL) {
		ddl.Options["row_format"] = "COMPACT"
	}))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestTableEngineAllowlistRuleSkipsPostgreSQL(t *testing.T) {
	t.Parallel()
	statementRule, err := newTableOptionAllowlistRule(ruleIDTableEngineAllowlist, "engine", "engine", []string{"InnoDB"}, rule.LevelBlocker, policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params:  map[string]any{"values": []any{"InnoDB"}},
	})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	statement := tableOptionStatement(func(ddl *spec.DDL) {
		ddl.Options["engine"] = "MyISAM"
	})
	statement.Dialect = spec.DialectPostgreSQL

	findings, err := statementRule.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected PostgreSQL engine rule to be skipped, got %d findings", len(findings))
	}
}

func TestTableRowFormatAllowlistRuleSkipsPostgreSQL(t *testing.T) {
	t.Parallel()
	statementRule, err := newTableOptionAllowlistRule(ruleIDTableRowFormatAllowlist, "row_format", "row format", []string{"DYNAMIC"}, rule.LevelBlocker, policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params:  map[string]any{"values": []any{"DYNAMIC"}},
	})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	statement := tableOptionStatement(func(ddl *spec.DDL) {
		ddl.Options["row_format"] = "COMPACT"
	})
	statement.Dialect = spec.DialectPostgreSQL

	findings, err := statementRule.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected PostgreSQL row_format rule to be skipped, got %d findings", len(findings))
	}
}

func TestTableCharsetAllowlistRuleSkipsPostgreSQL(t *testing.T) {
	t.Parallel()
	statementRule, err := newTableOptionAllowlistRule(ruleIDTableCharsetAllowlist, "charset", "charset", []string{"utf8mb4"}, rule.LevelBlocker, policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params:  map[string]any{"values": []any{"utf8mb4"}},
	})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	statement := tableOptionStatement(nil)
	statement.Dialect = spec.DialectPostgreSQL

	findings, err := statementRule.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected PostgreSQL table charset rule to be skipped, got %d findings", len(findings))
	}
}

func TestTableAutoIncrementInitValueRuleFindsNonDefaultSeed(t *testing.T) {
	t.Parallel()
	statementRule, err := newTableAutoIncrementInitValueRule(policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params:  map[string]any{"value": 1},
	})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	findings, err := statementRule.Evaluate(context.Background(), tableOptionStatement(func(ddl *spec.DDL) {
		ddl.Options["auto_increment"] = "42"
	}))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestTableForeignKeyForbidRuleFindsForeignKeys(t *testing.T) {
	t.Parallel()
	statementRule, err := newTableForeignKeyForbidRule(policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params:  map[string]any{"forbid": true},
	})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	findings, err := statementRule.Evaluate(context.Background(), tableOptionStatement(func(ddl *spec.DDL) {
		ddl.Constraints = append(ddl.Constraints, spec.Constraint{
			Type:    "foreign_key",
			Name:    "fk_users_org",
			Columns: []string{"org_id"},
		})
	}))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestTableCreateLikeForbidRuleFindsLikeStatements(t *testing.T) {
	t.Parallel()
	statementRule, err := newTableBooleanShapeRule(ruleIDTableCreateLikeForbid, "like", rule.LevelBlocker, func(ddl *spec.DDL) bool {
		return ddl.HasReferTable
	}, policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params:  map[string]any{"forbid": true},
	})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	findings, err := statementRule.Evaluate(context.Background(), tableOptionStatement(func(ddl *spec.DDL) {
		ddl.HasReferTable = true
	}))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestTableCreateAsForbidRuleFindsAsSelectStatements(t *testing.T) {
	t.Parallel()
	statementRule, err := newTableBooleanShapeRule(ruleIDTableCreateAsForbid, "as select", rule.LevelBlocker, func(ddl *spec.DDL) bool {
		return ddl.HasSelect
	}, policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params:  map[string]any{"forbid": true},
	})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	findings, err := statementRule.Evaluate(context.Background(), tableOptionStatement(func(ddl *spec.DDL) {
		ddl.HasSelect = true
	}))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestTablePartitionForbidRuleFindsPartitionedTables(t *testing.T) {
	t.Parallel()
	statementRule, err := newTableBooleanShapeRule(ruleIDTablePartitionForbid, "partitioning", rule.LevelBlocker, func(ddl *spec.DDL) bool {
		return ddl.HasPartition
	}, policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params:  map[string]any{"forbid": true},
	})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	findings, err := statementRule.Evaluate(context.Background(), tableOptionStatement(func(ddl *spec.DDL) {
		ddl.HasPartition = true
	}))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func tableOptionStatement(mutate func(*spec.DDL)) spec.Statement {
	ddl := &spec.DDL{
		Table:   &spec.Table{Name: "users", Comment: "user table"},
		Options: map[string]string{},
	}
	if mutate != nil {
		mutate(ddl)
	}
	return spec.Statement{
		Kind: spec.KindDDL,
		DDL:  ddl,
	}
}
