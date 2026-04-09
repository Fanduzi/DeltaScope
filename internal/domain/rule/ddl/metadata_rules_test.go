// Package ddl verifies metadata-backed DDL existence rules.
// input: metadata-enriched create-table and alter-table statement scenarios
// output: coverage for table, column, index, and primary-key existence checks
// pos: DDL metadata rule test coverage
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestTableExistenceRules(t *testing.T) {
	createRule, err := newTableExistenceRule(ruleIDTableExistsCreateForbid, false, rule.LevelBlocker, policy.RulePolicy{
		Enabled: true,
		Params:  map[string]any{},
	})
	if err != nil {
		t.Fatalf("new create existence rule: %v", err)
	}
	alterRule, err := newTableExistenceRule(ruleIDTableExistsAlterRequire, true, rule.LevelBlocker, policy.RulePolicy{
		Enabled: true,
		Params:  map[string]any{},
	})
	if err != nil {
		t.Fatalf("new alter existence rule: %v", err)
	}

	createStmt := spec.Statement{
		Kind: spec.KindDDL,
		DDL:  &spec.DDL{Table: &spec.Table{Name: "users"}},
		Metadata: &spec.Metadata{
			TargetTable: &spec.TableSnapshot{Exists: true, Table: &spec.Table{Name: "users"}},
		},
	}
	findings, err := createRule.Evaluate(createStmt)
	if err != nil {
		t.Fatalf("evaluate create rule: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected one create-table existence finding, got %d", len(findings))
	}

	alterStmt := spec.Statement{
		Kind: spec.KindDDL,
		DDL: &spec.DDL{
			Table: &spec.Table{Name: "users"},
			Alter: []spec.Alter{{Action: "drop_column", Name: "email"}},
		},
		Metadata: &spec.Metadata{
			TargetTable: &spec.TableSnapshot{Exists: false, Table: &spec.Table{Name: "users"}},
		},
	}
	findings, err = alterRule.Evaluate(alterStmt)
	if err != nil {
		t.Fatalf("evaluate alter rule: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected one alter-table existence finding, got %d", len(findings))
	}
}

func TestAlterColumnExistenceRules(t *testing.T) {
	addRule, err := newAlterObjectExistenceRule(ruleIDAlterAddColumnExistsForbid, []string{"add_columns"}, "column", true, rule.LevelBlocker, policy.RulePolicy{
		Enabled: true,
		Params:  map[string]any{},
	}, alterObjectName, snapshotHasColumn)
	if err != nil {
		t.Fatalf("new add-column existence rule: %v", err)
	}
	dropRule, err := newAlterObjectExistenceRule(ruleIDAlterDropColumnExistsRequire, []string{"drop_column"}, "column", false, rule.LevelBlocker, policy.RulePolicy{
		Enabled: true,
		Params:  map[string]any{},
	}, alterObjectName, snapshotHasColumn)
	if err != nil {
		t.Fatalf("new drop-column existence rule: %v", err)
	}

	statement := spec.Statement{
		Kind: spec.KindDDL,
		DDL: &spec.DDL{
			Table: &spec.Table{Name: "users"},
			Alter: []spec.Alter{
				{Action: "add_columns", Name: "email"},
				{Action: "drop_column", Name: "missing_col"},
			},
		},
		Metadata: &spec.Metadata{
			TargetTable: &spec.TableSnapshot{
				Exists: true,
				Table:  &spec.Table{Name: "users"},
				Columns: []spec.Column{
					{Name: "email"},
				},
			},
		},
	}

	addFindings, err := addRule.Evaluate(statement)
	if err != nil {
		t.Fatalf("evaluate add-column rule: %v", err)
	}
	if len(addFindings) != 1 {
		t.Fatalf("expected one add-column finding, got %d", len(addFindings))
	}

	dropFindings, err := dropRule.Evaluate(statement)
	if err != nil {
		t.Fatalf("evaluate drop-column rule: %v", err)
	}
	if len(dropFindings) != 1 {
		t.Fatalf("expected one drop-column finding, got %d", len(dropFindings))
	}
}

func TestAlterIndexAndPrimaryKeyExistenceRules(t *testing.T) {
	indexRule, err := newAlterObjectExistenceRule(ruleIDAlterDropIndexExistsRequire, []string{"drop_index"}, "index", false, rule.LevelBlocker, policy.RulePolicy{
		Enabled: true,
		Params:  map[string]any{},
	}, alterObjectName, snapshotHasIndex)
	if err != nil {
		t.Fatalf("new drop-index existence rule: %v", err)
	}
	pkRule, err := newAlterPrimaryKeyExistenceRule(ruleIDAlterDropPrimaryKeyExistsRequire, rule.LevelBlocker, policy.RulePolicy{
		Enabled: true,
		Params:  map[string]any{},
	})
	if err != nil {
		t.Fatalf("new primary-key existence rule: %v", err)
	}

	statement := spec.Statement{
		Kind: spec.KindDDL,
		DDL: &spec.DDL{
			Table: &spec.Table{Name: "users"},
			Alter: []spec.Alter{
				{Action: "drop_index", Name: "missing_idx"},
				{Action: "drop_constraint", Name: "USERS_PKEY"},
			},
		},
		Metadata: &spec.Metadata{
			TargetTable: &spec.TableSnapshot{
				Exists:      true,
				Table:       &spec.Table{Name: "users"},
				PrimaryKey:  &spec.Index{Name: "users_primary_idx", Kind: spec.IndexKindPrimary},
				Constraints: []spec.Constraint{{Type: "primary_key", Name: "users_pkey"}},
				Indexes:     []spec.Index{{Name: "idx_email", Kind: spec.IndexKindSecondary}},
			},
		},
	}

	indexFindings, err := indexRule.Evaluate(statement)
	if err != nil {
		t.Fatalf("evaluate index rule: %v", err)
	}
	if len(indexFindings) != 1 {
		t.Fatalf("expected one index existence finding, got %d", len(indexFindings))
	}

	pkFindings, err := pkRule.Evaluate(statement)
	if err != nil {
		t.Fatalf("evaluate primary-key rule: %v", err)
	}
	if len(pkFindings) != 0 {
		t.Fatalf("expected no primary-key existence finding while primary key exists, got %d", len(pkFindings))
	}

	explicitDropPrimaryKey := statement
	explicitDropPrimaryKey.DDL = &spec.DDL{
		Table: &spec.Table{Name: "users"},
		Alter: []spec.Alter{{Action: "drop_primary_key", Name: "users_pkey"}},
	}
	explicitDropPrimaryKey.Metadata = &spec.Metadata{
		TargetTable: &spec.TableSnapshot{
			Exists:  true,
			Table:   &spec.Table{Name: "users"},
			Indexes: []spec.Index{{Name: "idx_email", Kind: spec.IndexKindSecondary}},
		},
	}
	pkFindings, err = pkRule.Evaluate(explicitDropPrimaryKey)
	if err != nil {
		t.Fatalf("evaluate explicit drop primary key rule without primary key: %v", err)
	}
	if len(pkFindings) != 1 {
		t.Fatalf("expected one primary-key existence finding, got %d", len(pkFindings))
	}
}

func TestAlterIndexExistenceRuleSupportsStandaloneIndexDDL(t *testing.T) {
	indexRule, err := newAlterObjectExistenceRule(ruleIDAlterRenameIndexExistsRequire, []string{"rename_index"}, "index", false, rule.LevelBlocker, policy.RulePolicy{
		Enabled: true,
		Params:  map[string]any{},
	}, alterObjectName, snapshotHasIndex)
	if err != nil {
		t.Fatalf("new rename-index existence rule: %v", err)
	}

	statement := spec.Statement{
		Kind: spec.KindDDL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Alter: []spec.Alter{{Action: "rename_index", Name: "missing_idx", Options: map[string]string{"new_name": "idx_new"}}},
		},
		Metadata: &spec.Metadata{
			TargetTable: &spec.TableSnapshot{
				Exists:  true,
				Table:   &spec.Table{Name: "users"},
				Indexes: []spec.Index{{Name: "idx_email", Kind: spec.IndexKindSecondary}},
			},
		},
	}

	findings, err := indexRule.Evaluate(statement)
	if err != nil {
		t.Fatalf("evaluate standalone rename-index rule: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected one standalone rename-index existence finding, got %d", len(findings))
	}
	if findings[0].Metadata["table"] != "users" {
		t.Fatalf("expected snapshot table name fallback, got %#v", findings[0].Metadata)
	}
}

func TestAlterPrimaryKeyExistenceRuleConstraintOnlySnapshot(t *testing.T) {
	pkRule, err := newAlterPrimaryKeyExistenceRule(ruleIDAlterDropPrimaryKeyExistsRequire, rule.LevelBlocker, policy.RulePolicy{
		Enabled: true,
		Params:  map[string]any{},
	})
	if err != nil {
		t.Fatalf("new primary-key existence rule: %v", err)
	}

	// Snapshot has a primary_key constraint but no PrimaryKey index pointer.
	// This can happen when metadata is loaded from a source that records
	// constraints but does not populate the index summary.
	statement := spec.Statement{
		Kind: spec.KindDDL,
		DDL: &spec.DDL{
			Table: &spec.Table{Name: "users"},
			Alter: []spec.Alter{{Action: "drop_constraint", Name: "users_pkey"}},
		},
		Metadata: &spec.Metadata{
			TargetTable: &spec.TableSnapshot{
				Exists: true,
				Table:  &spec.Table{Name: "users"},
				Constraints: []spec.Constraint{
					{Type: "primary_key", Name: "users_pkey", Columns: []string{"id"}},
				},
				// PrimaryKey is intentionally nil.
			},
		},
	}

	findings, err := pkRule.Evaluate(statement)
	if err != nil {
		t.Fatalf("evaluate primary-key rule: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no primary-key existence finding when constraint-based PK exists, got %d: %#v", len(findings), findings)
	}
}

func TestAlterPrimaryKeyExistenceRuleFallsBackToPrimaryKeyIndexName(t *testing.T) {
	pkRule, err := newAlterPrimaryKeyExistenceRule(ruleIDAlterDropPrimaryKeyExistsRequire, rule.LevelBlocker, policy.RulePolicy{
		Enabled: true,
		Params:  map[string]any{},
	})
	if err != nil {
		t.Fatalf("new primary-key existence rule: %v", err)
	}

	statement := spec.Statement{
		Kind: spec.KindDDL,
		DDL: &spec.DDL{
			Table: &spec.Table{Name: "users"},
			Alter: []spec.Alter{{Action: "drop_constraint", Name: "users_pkey"}},
		},
		Metadata: &spec.Metadata{
			TargetTable: &spec.TableSnapshot{
				Exists:     true,
				Table:      &spec.Table{Name: "users"},
				PrimaryKey: &spec.Index{Name: "users_pkey", Kind: spec.IndexKindPrimary},
			},
		},
	}

	findings, err := pkRule.Evaluate(statement)
	if err != nil {
		t.Fatalf("evaluate primary-key fallback rule: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no finding when fallback primary key name matches, got %d", len(findings))
	}
}

func TestMetadataExistenceRulesIgnoreLegacyRequiredParamAndSkipWithoutMetadata(t *testing.T) {
	createRule, err := newTableExistenceRule(ruleIDTableExistsCreateForbid, false, rule.LevelBlocker, policy.RulePolicy{
		Enabled: true,
		Params:  map[string]any{"required": false},
	})
	if err != nil {
		t.Fatalf("new create existence rule: %v", err)
	}

	findings, err := createRule.Evaluate(spec.Statement{
		Kind: spec.KindDDL,
		DDL:  &spec.DDL{Table: &spec.Table{Name: "users"}},
	})
	if err != nil {
		t.Fatalf("evaluate offline create rule: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected offline metadata-backed rule to no-op without metadata, got %d findings", len(findings))
	}

	findings, err = createRule.Evaluate(spec.Statement{
		Kind: spec.KindDDL,
		DDL:  &spec.DDL{Table: &spec.Table{Name: "users"}},
		Metadata: &spec.Metadata{
			TargetTable: &spec.TableSnapshot{Exists: true, Table: &spec.Table{Name: "users"}},
		},
	})
	if err != nil {
		t.Fatalf("evaluate create rule with metadata: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected metadata-backed create rule to stay active even when legacy required=false is present, got %d findings", len(findings))
	}
}
