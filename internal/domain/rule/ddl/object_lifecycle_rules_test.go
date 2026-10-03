// Package ddl verifies DDL object lifecycle governance rules.
// input: parser-neutral create-view, drop-table, and truncate-table statement specs
// output: coverage for object lifecycle forbids, metadata-backed existence checks, and adaptive-hash cautions
// pos: DDL lifecycle rule test coverage for remaining matrix gaps plus the drop-existence evidence-gap reporter guards
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestCreateViewForbidRule(t *testing.T) {
	t.Parallel()
	statementRule, err := newForbiddenDDLOperationRule(ruleIDViewCreateForbid, spec.DDLOperationCreateView, "create view", rule.LevelBlocker, policy.RulePolicy{
		Enabled: true,
		Params:  map[string]any{"forbid": true},
	})
	if err != nil {
		t.Fatalf("new create-view forbid rule: %v", err)
	}

	findings, err := statementRule.Evaluate(context.Background(), spec.Statement{
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
	t.Parallel()
	dropRule, err := newForbiddenDDLOperationRule(ruleIDTableDropForbid, spec.DDLOperationDropTable, "drop table", rule.LevelBlocker, policy.RulePolicy{
		Enabled: true,
		Params:  map[string]any{"forbid": true},
	})
	if err != nil {
		t.Fatalf("new drop-table rule: %v", err)
	}
	dropViewRule, err := newForbiddenDDLOperationRule(ruleIDViewDropForbid, spec.DDLOperationDropView, "drop view", rule.LevelBlocker, policy.RulePolicy{
		Enabled: true,
		Params:  map[string]any{"forbid": true},
	})
	if err != nil {
		t.Fatalf("new drop-view rule: %v", err)
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
	dropFindings, err := dropRule.Evaluate(context.Background(), dropStmt)
	if err != nil {
		t.Fatalf("evaluate drop rule: %v", err)
	}
	if len(dropFindings) != 1 {
		t.Fatalf("expected one drop-table finding, got %d", len(dropFindings))
	}

	dropViewStmt := spec.Statement{
		Kind: spec.KindDDL,
		DDL:  &spec.DDL{Operation: spec.DDLOperationDropView, Table: &spec.Table{Name: "active_users"}},
	}
	dropViewFindings, err := dropViewRule.Evaluate(context.Background(), dropViewStmt)
	if err != nil {
		t.Fatalf("evaluate drop-view rule: %v", err)
	}
	if len(dropViewFindings) != 1 {
		t.Fatalf("expected one drop-view finding, got %d", len(dropViewFindings))
	}

	truncateStmt := spec.Statement{
		Kind: spec.KindDDL,
		DDL:  &spec.DDL{Operation: spec.DDLOperationTruncateTable, Table: &spec.Table{Name: "users"}},
	}
	truncateFindings, err := truncateRule.Evaluate(context.Background(), truncateStmt)
	if err != nil {
		t.Fatalf("evaluate truncate rule: %v", err)
	}
	if len(truncateFindings) != 1 {
		t.Fatalf("expected one truncate-table finding, got %d", len(truncateFindings))
	}
}

func TestLifecycleMetadataRules(t *testing.T) {
	t.Parallel()
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
	findings, err := dropExistsRule.Evaluate(context.Background(), dropStmt)
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
	findings, err = adaptiveHashRule.Evaluate(context.Background(), truncateStmt)
	if err != nil {
		t.Fatalf("evaluate adaptive-hash rule: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected one adaptive-hash caution finding, got %d", len(findings))
	}
}

func TestLifecycleRowCountRules(t *testing.T) {
	t.Parallel()
	dropRowsRule, err := newTableRowCountRiskRule(ruleIDTableDropRowsMaxCount, spec.DDLOperationDropTable, "drop table", rule.LevelWarning, policy.RulePolicy{
		Enabled: true,
		Params:  map[string]any{"limit": 100, "requires_metadata": true},
	})
	if err != nil {
		t.Fatalf("new drop row-count rule: %v", err)
	}

	findings, err := dropRowsRule.Evaluate(context.Background(), spec.Statement{
		Kind: spec.KindDDL,
		DDL:  &spec.DDL{Operation: spec.DDLOperationDropTable, Table: &spec.Table{Name: "users"}},
		Metadata: &spec.Metadata{
			TargetTable: &spec.TableSnapshot{
				Exists:  true,
				Table:   &spec.Table{Name: "users"},
				Options: map[string]string{"table_rows": "250"},
			},
		},
	})
	if err != nil {
		t.Fatalf("evaluate drop row-count rule: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected one drop row-count finding, got %d", len(findings))
	}
}

func TestDropTableExistenceRuleEvidenceGaps(t *testing.T) {
	t.Parallel()
	dropRule, err := newTableOperationExistenceRule(ruleIDTableDropExistsRequire, spec.DDLOperationDropTable, "drop table", rule.LevelBlocker, policy.RulePolicy{
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("new drop-table existence rule: %v", err)
	}
	reporter, ok := dropRule.(rule.EvidenceReporter)
	if !ok {
		t.Fatal("drop-table existence rule must implement rule.EvidenceReporter")
	}
	dropStatement := func(dialect spec.Dialect, snapshot *spec.TableSnapshot) spec.Statement {
		statement := spec.Statement{
			Dialect: dialect,
			Kind:    spec.KindDDL,
			DDL:     &spec.DDL{Operation: spec.DDLOperationDropTable, Table: &spec.Table{Name: "t"}},
		}
		if snapshot != nil {
			statement.Metadata = &spec.Metadata{TargetTable: snapshot}
		}
		return statement
	}
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		gaps := reporter.EvidenceGaps(dropStatement(dialect, nil))
		if len(gaps) != 1 || gaps[0].ReasonCode != "unknown_table_state" || len(gaps[0].RequiredFacts) != 1 || gaps[0].RequiredFacts[0] != "target_table.existence" {
			t.Fatalf("unknown %s drop must report the frozen existence gap, got %+v", dialect, gaps)
		}
	}
	if gaps := reporter.EvidenceGaps(dropStatement(spec.DialectMySQL, &spec.TableSnapshot{Exists: true, Table: &spec.Table{Name: "t"}})); len(gaps) != 0 {
		t.Fatalf("present drop target must not gap, got %+v", gaps)
	}
	if gaps := reporter.EvidenceGaps(dropStatement(spec.DialectMySQL, &spec.TableSnapshot{Exists: false, Table: &spec.Table{Name: "t"}})); len(gaps) != 0 {
		t.Fatalf("known-absent drop target keeps its blocker instead of a gap, got %+v", gaps)
	}
	if gaps := reporter.EvidenceGaps(dropStatement(spec.DialectPostgreSQL, nil)); len(gaps) != 0 {
		t.Fatalf("the drop gap must never leak to PostgreSQL, got %+v", gaps)
	}
	if gaps := reporter.EvidenceGaps(spec.Statement{
		Dialect: spec.DialectMySQL,
		Kind:    spec.KindDDL,
		DDL:     &spec.DDL{Operation: spec.DDLOperationCreateTable, Table: &spec.Table{Name: "t"}},
	}); len(gaps) != 0 {
		t.Fatalf("non-applicable statement must not gap, got %+v", gaps)
	}
	truncateRule, err := newTableOperationExistenceRule(ruleIDTableTruncateExistsRequire, spec.DDLOperationTruncateTable, "truncate table", rule.LevelBlocker, policy.RulePolicy{
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("new truncate existence rule: %v", err)
	}
	truncateReporter, ok := truncateRule.(rule.EvidenceReporter)
	if !ok {
		t.Fatal("truncate existence rule shares the type but must not emit this gap")
	}
	if gaps := truncateReporter.EvidenceGaps(spec.Statement{
		Dialect: spec.DialectMySQL,
		Kind:    spec.KindDDL,
		DDL:     &spec.DDL{Operation: spec.DDLOperationTruncateTable, Table: &spec.Table{Name: "t"}},
	}); len(gaps) != 0 {
		t.Fatalf("truncate must not inherit the drop gap, got %+v", gaps)
	}
}
