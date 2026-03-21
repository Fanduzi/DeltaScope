// Package ddl verifies DDL object lifecycle governance rules.
// input: parser-neutral create-view, drop-table, and truncate-table statement specs
// output: coverage for object lifecycle forbids, metadata-backed existence checks, and adaptive-hash cautions
// pos: DDL lifecycle rule test coverage for remaining matrix gaps
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestCreateViewForbidRule(t *testing.T) {
	statementRule, err := newForbiddenDDLOperationRule(ruleIDViewCreateForbid, spec.DDLOperationCreateView, "create view", rule.LevelBlocker, policy.RulePolicy{
		Enabled: true,
		Params:  map[string]any{"forbid": true},
	})
	if err != nil {
		t.Fatalf("new create-view forbid rule: %v", err)
	}

	findings, err := statementRule.Evaluate(spec.Statement{
		Kind: spec.KindDDL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationCreateView,
			Table:     &spec.Table{Name: "active_users"},
			HasSelect: true,
		},
	})
	if err != nil {
		t.Fatalf("evaluate create-view rule: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected one create-view finding, got %d", len(findings))
	}
}

func TestDropAndTruncateRules(t *testing.T) {
	dropRule, err := newForbiddenDDLOperationRule(ruleIDTableDropForbid, spec.DDLOperationDropTable, "drop table", rule.LevelBlocker, policy.RulePolicy{
		Enabled: true,
		Params:  map[string]any{"forbid": true},
	})
	if err != nil {
		t.Fatalf("new drop-table rule: %v", err)
	}
	truncateRule, err := newForbiddenDDLOperationRule(ruleIDTableTruncateForbid, spec.DDLOperationTruncateTable, "truncate table", rule.LevelBlocker, policy.RulePolicy{
		Enabled: true,
		Params:  map[string]any{"forbid": true},
	})
	if err != nil {
		t.Fatalf("new truncate-table rule: %v", err)
	}

	dropStmt := spec.Statement{
		Kind: spec.KindDDL,
		DDL:  &spec.DDL{Operation: spec.DDLOperationDropTable, Table: &spec.Table{Name: "users"}},
	}
	dropFindings, err := dropRule.Evaluate(dropStmt)
	if err != nil {
		t.Fatalf("evaluate drop rule: %v", err)
	}
	if len(dropFindings) != 1 {
		t.Fatalf("expected one drop-table finding, got %d", len(dropFindings))
	}

	truncateStmt := spec.Statement{
		Kind: spec.KindDDL,
		DDL:  &spec.DDL{Operation: spec.DDLOperationTruncateTable, Table: &spec.Table{Name: "users"}},
	}
	truncateFindings, err := truncateRule.Evaluate(truncateStmt)
	if err != nil {
		t.Fatalf("evaluate truncate rule: %v", err)
	}
	if len(truncateFindings) != 1 {
		t.Fatalf("expected one truncate-table finding, got %d", len(truncateFindings))
	}
}

func TestLifecycleMetadataRules(t *testing.T) {
	dropExistsRule, err := newTableOperationExistenceRule(ruleIDTableDropExistsRequire, spec.DDLOperationDropTable, "drop table", rule.LevelBlocker, policy.RulePolicy{
		Enabled: true,
		Params:  map[string]any{"requires_metadata": true},
	})
	if err != nil {
		t.Fatalf("new drop-table existence rule: %v", err)
	}
	adaptiveHashRule, err := newAdaptiveHashLifecycleRule(ruleIDTableTruncateAdaptiveHashWarn, spec.DDLOperationTruncateTable, "truncate table", rule.LevelWarning, policy.RulePolicy{
		Enabled: true,
		Params:  map[string]any{"requires_metadata": true},
	})
	if err != nil {
		t.Fatalf("new adaptive-hash rule: %v", err)
	}

	dropStmt := spec.Statement{
		Kind: spec.KindDDL,
		DDL:  &spec.DDL{Operation: spec.DDLOperationDropTable, Table: &spec.Table{Name: "users"}},
		Metadata: &spec.Metadata{
			TargetTable: &spec.TableSnapshot{Exists: false, Table: &spec.Table{Name: "users"}},
		},
	}
	findings, err := dropExistsRule.Evaluate(dropStmt)
	if err != nil {
		t.Fatalf("evaluate drop-table existence rule: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected one drop-table existence finding, got %d", len(findings))
	}

	truncateStmt := spec.Statement{
		Kind: spec.KindDDL,
		DDL:  &spec.DDL{Operation: spec.DDLOperationTruncateTable, Table: &spec.Table{Name: "users"}},
		Metadata: &spec.Metadata{
			Instance:    &spec.InstanceFacts{InnoDBAdaptiveHashEnabled: true},
			TargetTable: &spec.TableSnapshot{Exists: true, Table: &spec.Table{Name: "users"}},
		},
	}
	findings, err = adaptiveHashRule.Evaluate(truncateStmt)
	if err != nil {
		t.Fatalf("evaluate adaptive-hash rule: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected one adaptive-hash caution finding, got %d", len(findings))
	}
}
