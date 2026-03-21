// Package ddl verifies DDL rule registration behavior.
// input: policy objects, synthetic create-table statements, and the shared registry
// output: deterministic registration and evaluation coverage for the first DDL rule batch
// pos: domain DDL rule integration tests across policy-backed registry assembly
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestRegisterAddsEnabledDDLRulesInDeterministicOrder(t *testing.T) {
	registry := rule.NewRegistry()
	cfg := policy.Default()
	cfg.Rules["ddl.table.name.max_length"] = policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params: map[string]any{
			"limit": 5,
		},
	}
	cfg.Rules["ddl.column.name.max_length"] = policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params: map[string]any{
			"limit": 8,
		},
	}
	cfg.Rules["ddl.column.varchar.max_length"] = policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params: map[string]any{
			"limit": 64,
		},
	}
	cfg.Rules["ddl.index.total.max_count"] = policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelWarning,
		Params: map[string]any{
			"limit": 3,
		},
	}

	if err := Register(registry, cfg); err != nil {
		t.Fatalf("register: %v", err)
	}

	findings, err := registry.EvaluateStatement(spec.Statement{
		Kind: spec.KindDDL,
		DDL: &spec.DDL{
			Table: &spec.Table{Name: "orders_archive"},
			Columns: []spec.Column{
				{Name: "display_name", Type: "varchar(255)", Length: 255},
				{Name: "ratio", Type: "float"},
				{Name: "body", Type: "text"},
			},
			Options: map[string]string{
				"engine":  "InnoDB",
				"charset": "utf8mb4",
			},
			Indexes: []spec.Index{
				{Name: "badsecondary", Kind: spec.IndexKindSecondary, Columns: []string{"display_name", "ratio", "created_at", "updated_at", "tenant_id", "region_id", "source_id", "org_id", "category_id"}},
				{Name: "badunique", Kind: spec.IndexKindUnique, Columns: []string{"display_name"}},
				{Name: "badfull", Kind: spec.IndexKindFulltext, Columns: []string{"body"}},
				{Name: "idx_dup_one", Kind: spec.IndexKindSecondary, Columns: []string{"ratio"}},
				{Name: "idx_dup_two", Kind: spec.IndexKindSecondary, Columns: []string{"ratio"}},
			},
		},
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}

	if len(findings) != 21 {
		t.Fatalf("expected 21 findings, got %d", len(findings))
	}

	wantIDs := []string{
		"ddl.table.comment.require",
		"ddl.table.name.max_length",
		"ddl.table.primary_key.require",
		"ddl.table.audit_columns.require",
		"ddl.table.audit_columns.require",
		"ddl.column.comment.require",
		"ddl.column.comment.require",
		"ddl.column.comment.require",
		"ddl.column.name.max_length",
		"ddl.column.varchar.max_length",
		"ddl.column.default.require",
		"ddl.column.default.require",
		"ddl.column.not_null.require",
		"ddl.column.not_null.require",
		"ddl.column.float_double.forbid",
		"ddl.index.total.max_count",
		"ddl.index.columns.max_count",
		"ddl.index.unique.prefix.require",
		"ddl.index.secondary.prefix.require",
		"ddl.index.fulltext.prefix.require",
		"ddl.index.duplicate.forbid",
	}
	for i, want := range wantIDs {
		if findings[i].RuleID != want {
			t.Fatalf("expected finding %d to use rule %q, got %q", i, want, findings[i].RuleID)
		}
	}
}

func TestRegisterSkipsDisabledDDLRules(t *testing.T) {
	registry := rule.NewRegistry()
	cfg := policy.Default()
	cfg.Rules["ddl.table.comment.require"] = policy.RulePolicy{
		Enabled: false,
		Level:   rule.LevelWarning,
		Params: map[string]any{
			"required": true,
		},
	}

	if err := Register(registry, cfg); err != nil {
		t.Fatalf("register: %v", err)
	}

	findings, err := registry.EvaluateStatement(createTableWithColumns("users", spec.Column{
		Name:       "id",
		Type:       "bigint",
		Comment:    "'pk'",
		NotNull:    true,
		HasDefault: true,
	}))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}

	for _, finding := range findings {
		if finding.RuleID == "ddl.table.comment.require" {
			t.Fatalf("expected disabled comment rule not to run, got %+v", finding)
		}
	}
}

func TestRegisterAddsEnabledAlterRulesInDeterministicOrder(t *testing.T) {
	registry := rule.NewRegistry()
	cfg := policy.Default()
	cfg.Rules["ddl.alter.drop_column.forbid"] = policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelWarning,
		Params: map[string]any{
			"forbid": true,
		},
	}
	cfg.Rules["ddl.alter.drop_index.forbid"] = policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelWarning,
		Params: map[string]any{
			"forbid": true,
		},
	}
	cfg.Rules["ddl.alter.modify_column.forbid"] = policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelWarning,
		Params: map[string]any{
			"forbid": true,
		},
	}

	if err := Register(registry, cfg); err != nil {
		t.Fatalf("register: %v", err)
	}

	findings, err := registry.EvaluateStatement(alterStatement(
		spec.Alter{Action: "drop_column", Name: "legacy_name"},
		spec.Alter{Action: "drop_primary_key", Name: "primary"},
		spec.Alter{Action: "drop_index", Name: "idx_legacy"},
		spec.Alter{Action: "rename_table", Name: "users_archive"},
		spec.Alter{Action: "rename_column", Name: "old_email"},
		spec.Alter{Action: "change_column", Name: "old_phone"},
		spec.Alter{Action: "modify_column", Name: "status"},
	))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}

	wantIDs := []string{
		"ddl.alter.drop_column.forbid",
		"ddl.alter.drop_primary_key.forbid",
		"ddl.alter.drop_index.forbid",
		"ddl.alter.rename_table.forbid",
		"ddl.alter.rename_column.forbid",
		"ddl.alter.change_column.forbid",
		"ddl.alter.modify_column.forbid",
	}
	if len(findings) != len(wantIDs) {
		t.Fatalf("expected %d findings, got %d", len(wantIDs), len(findings))
	}
	for i, want := range wantIDs {
		if findings[i].RuleID != want {
			t.Fatalf("expected finding %d to use rule %q, got %q", i, want, findings[i].RuleID)
		}
	}
}

func TestRegisterAddsEnabledTableOptionRulesInDeterministicOrder(t *testing.T) {
	registry := rule.NewRegistry()
	cfg := policy.Default()
	cfg.Rules["ddl.table.primary_key.require"] = policy.RulePolicy{
		Enabled: false,
		Level:   rule.LevelBlocker,
		Params: map[string]any{
			"required": true,
		},
	}
	cfg.Rules["ddl.table.columns.min_count"] = policy.RulePolicy{
		Enabled: false,
		Level:   rule.LevelBlocker,
		Params: map[string]any{
			"limit": 1,
		},
	}
	cfg.Rules["ddl.table.audit_columns.require"] = policy.RulePolicy{
		Enabled: false,
		Level:   rule.LevelWarning,
		Params: map[string]any{
			"required": true,
		},
	}
	cfg.Rules["ddl.table.comment.max_length"] = policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelWarning,
		Params: map[string]any{
			"limit": 5,
		},
	}

	if err := Register(registry, cfg); err != nil {
		t.Fatalf("register: %v", err)
	}

	findings, err := registry.EvaluateStatement(spec.Statement{
		Kind: spec.KindDDL,
		DDL: &spec.DDL{
			Table:         &spec.Table{Name: "users", Comment: "too long"},
			Options:       map[string]string{"engine": "MyISAM", "charset": "latin1"},
			Constraints:   []spec.Constraint{{Type: "foreign_key", Name: "fk_users_org", Columns: []string{"org_id"}}},
			HasPartition:  true,
			HasReferTable: true,
			HasSelect:     true,
		},
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}

	wantIDs := []string{
		"ddl.table.comment.max_length",
		"ddl.table.engine.allowlist",
		"ddl.table.charset.allowlist",
		"ddl.table.foreign_key.forbid",
		"ddl.table.partition.forbid",
		"ddl.table.create_like.forbid",
		"ddl.table.create_as.forbid",
	}
	if len(findings) != len(wantIDs) {
		t.Fatalf("expected %d findings, got %d", len(wantIDs), len(findings))
	}
	for i, want := range wantIDs {
		if findings[i].RuleID != want {
			t.Fatalf("expected finding %d to use rule %q, got %q", i, want, findings[i].RuleID)
		}
	}
}

func TestRegisterAddsDisabledTableGovernanceRule(t *testing.T) {
	registry := rule.NewRegistry()
	cfg := policy.Default()
	cfg.Rules["ddl.table.denylist.forbid"] = policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params: map[string]any{
			"tables":           []string{"users"},
			"qualified_tables": []string{"app.audit_log"},
			"schemas":          []string{"mysql"},
		},
	}

	if err := Register(registry, cfg); err != nil {
		t.Fatalf("register: %v", err)
	}

	findings, err := registry.EvaluateStatement(spec.Statement{
		Kind: spec.KindDDL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationCreateTable,
			Table:     &spec.Table{Name: "users"},
		},
		Metadata: &spec.Metadata{Schema: "app"},
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	found := false
	for _, finding := range findings {
		if finding.RuleID == "ddl.table.denylist.forbid" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected disabled-table finding, got %+v", findings)
	}
}

func TestRegisterAddsEnabledPrimaryKeySemanticRulesInDeterministicOrder(t *testing.T) {
	registry := rule.NewRegistry()
	cfg := policy.Default()
	cfg.Rules["ddl.table.primary_key.columns.max_count"] = policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelWarning,
		Params: map[string]any{
			"limit": 1,
		},
	}

	if err := Register(registry, cfg); err != nil {
		t.Fatalf("register: %v", err)
	}

	findings, err := registry.EvaluateStatement(spec.Statement{
		Kind: spec.KindDDL,
		DDL: &spec.DDL{
			Table: &spec.Table{Name: "users", Comment: "user table"},
			Columns: []spec.Column{
				{Name: "id", Type: "int", NotNull: false, Unsigned: false, AutoIncrement: false, Comment: "'pk'"},
				{Name: "created_at", Type: "datetime", NotNull: true, HasDefault: true, DefaultIsCurrentTimestamp: true, Comment: "'created'"},
				{Name: "updated_at", Type: "datetime", NotNull: true, HasDefault: true, DefaultIsCurrentTimestamp: true, OnUpdateCurrentTimestamp: true, Comment: "'updated'"},
			},
			PrimaryKey: &spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: []string{"id"}},
			Options:    map[string]string{"engine": "InnoDB", "charset": "utf8mb4"},
		},
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}

	wantIDs := []string{
		"ddl.table.primary_key.bigint.require",
		"ddl.table.primary_key.unsigned.require",
		"ddl.table.primary_key.auto_increment.require",
		"ddl.table.primary_key.not_null.require",
		"ddl.column.default.require",
		"ddl.column.not_null.require",
	}
	if len(findings) != len(wantIDs) {
		t.Fatalf("expected %d findings, got %d", len(wantIDs), len(findings))
	}
	for i, want := range wantIDs {
		if findings[i].RuleID != want {
			t.Fatalf("expected finding %d to use rule %q, got %q", i, want, findings[i].RuleID)
		}
	}
}

func TestRegisterRejectsInvalidDDLRuleConfig(t *testing.T) {
	registry := rule.NewRegistry()
	cfg := policy.Default()
	cfg.Rules["ddl.table.name.max_length"] = policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params: map[string]any{
			"limit": "not-an-int",
		},
	}

	if err := Register(registry, cfg); err == nil {
		t.Fatal("expected invalid config to be rejected")
	}
}
