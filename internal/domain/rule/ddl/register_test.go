// Package ddl verifies DDL rule registration behavior.
// input: policy objects, synthetic create-table statements, and the shared registry
// output: deterministic registration and evaluation coverage for the first DDL rule batch
// pos: domain DDL rule integration tests across policy-backed registry assembly
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"sort"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// Dialect hygiene characterization keeps explicit rule buckets in-test so we can
// detect out-of-dialect leakage at registry level without touching production
// registration logic. The rule lists intentionally mirror the audit-layer lists
// because registry-level and service-level characterization should fail independently.
var defaultPolicyDialectHygieneRegistryMySQLFamilyOnlyRuleIDs = []string{
	"ddl.table.primary_key.not_null.require",
	"ddl.table.primary_key.unsigned.require",
	"ddl.table.primary_key.auto_increment.require",
	"ddl.table.engine.allowlist",
	"ddl.table.charset.allowlist",
	"ddl.table.row_format.allowlist",
	"ddl.column.charset.allowlist",
	"ddl.column.collation.allowlist",
	"ddl.column.charset_collation.match.require",
	"ddl.alter.change_column.forbid",
	"ddl.alter.modify_column.forbid",
	"ddl.alter.modify_column.explicit_default_change.forbid",
	"ddl.alter.change_column.explicit_default_change.forbid",
}

var defaultPolicyDialectHygieneRegistryPostgreSQLOnlyRuleIDs = []string{
	"ddl.alter.set_data_type.forbid",
	"ddl.alter.set_default.forbid",
	"ddl.alter.drop_default.forbid",
	"ddl.alter.set_not_null.forbid",
	"ddl.alter.drop_not_null.forbid",
	"ddl.alter.drop_expression.forbid",
	"ddl.alter.set_generated.forbid",
	"ddl.alter.drop_identity.forbid",
	"ddl.alter.set_default.explicit_default_change.forbid",
	"ddl.alter.drop_default.explicit_default_change.forbid",
	"ddl.alter.set_not_null.explicit_nullability_change.forbid",
	"ddl.alter.drop_not_null.explicit_nullability_change.forbid",
}

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
			Operation: spec.DDLOperationCreateTable,
			Table:     &spec.Table{Name: "orders_archive"},
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

func TestRegisterAddsStandalonePostgreSQLDropIndexRule(t *testing.T) {
	registry := rule.NewRegistry()
	cfg := policy.Default()
	cfg.Rules["ddl.alter.drop_index.forbid"] = policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelWarning,
		Params: map[string]any{
			"forbid": true,
		},
	}

	if err := Register(registry, cfg); err != nil {
		t.Fatalf("register: %v", err)
	}

	findings, err := registry.EvaluateStatement(spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationDropIndex,
			Alter:     []spec.Alter{{Action: "drop_index", Name: "idx_users_email"}},
		},
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].RuleID != "ddl.alter.drop_index.forbid" {
		t.Fatalf("expected standalone drop index to reuse existing rule id, got %q", findings[0].RuleID)
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

func TestRegisterDefaultPolicyDialectHygiene(t *testing.T) {
	registry := rule.NewRegistry()
	if err := Register(registry, policy.Default()); err != nil {
		t.Fatalf("register: %v", err)
	}

	t.Run("postgresql_create_table_excludes_mysql_family_only_rules", func(t *testing.T) {
		stmt := spec.Statement{
			Kind:    spec.KindDDL,
			Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{
				Operation: spec.DDLOperationCreateTable,
				Table:     &spec.Table{Name: "pg_smoke"},
				Columns: []spec.Column{{Name: "id", Type: "bigint", NotNull: true}},
				PrimaryKey: &spec.Index{Name: "pg_smoke_pkey", Kind: spec.IndexKindPrimary, Columns: []string{"id"}},
			},
		}

		eval, err := registry.EvaluateStatementDetailed(stmt)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		assertNoAppliedDDLRuleIDs(t, eval.AppliedRuleIDs, defaultPolicyDialectHygieneRegistryMySQLFamilyOnlyRuleIDs)
		assertNoDDLRuleIDs(t, eval.Findings, defaultPolicyDialectHygieneRegistryMySQLFamilyOnlyRuleIDs)
	})

	t.Run("mysql_alter_excludes_postgresql_only_rules", func(t *testing.T) {
		stmt := spec.Statement{
			Kind:    spec.KindDDL,
			Dialect: spec.DialectMySQL,
			DDL: &spec.DDL{
				Operation: spec.DDLOperationAlterTable,
				Table:     &spec.Table{Name: "smoke_users"},
				Alter: []spec.Alter{{
					Action: "modify_column",
					Name:   "status",
					Column: &spec.AlterColumn{
						OldName:    "status",
						Definition: &spec.Column{Name: "status", Type: "varchar(32)"},
						Change:     &spec.AlterColumnChange{TouchesDefault: true, TouchesNullability: true},
					},
				}},
			},
		}

		eval, err := registry.EvaluateStatementDetailed(stmt)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		assertNoAppliedDDLRuleIDs(t, eval.AppliedRuleIDs, defaultPolicyDialectHygieneRegistryPostgreSQLOnlyRuleIDs)
		assertNoDDLRuleIDs(t, eval.Findings, defaultPolicyDialectHygieneRegistryPostgreSQLOnlyRuleIDs)
	})

	t.Run("tidb_alter_excludes_postgresql_only_rules", func(t *testing.T) {
		stmt := spec.Statement{
			Kind:    spec.KindDDL,
			Dialect: spec.DialectTiDB,
			DDL: &spec.DDL{
				Operation: spec.DDLOperationAlterTable,
				Table:     &spec.Table{Name: "smoke_users"},
				Alter: []spec.Alter{{
					Action: "modify_column",
					Name:   "status",
					Column: &spec.AlterColumn{
						OldName:    "status",
						Definition: &spec.Column{Name: "status", Type: "varchar(32)"},
						Change:     &spec.AlterColumnChange{TouchesDefault: true, TouchesNullability: true},
					},
				}},
			},
		}

		eval, err := registry.EvaluateStatementDetailed(stmt)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		assertNoAppliedDDLRuleIDs(t, eval.AppliedRuleIDs, defaultPolicyDialectHygieneRegistryPostgreSQLOnlyRuleIDs)
		assertNoDDLRuleIDs(t, eval.Findings, defaultPolicyDialectHygieneRegistryPostgreSQLOnlyRuleIDs)
	})
}

func TestRegisterAddsPGNativeAlterActionForbidRules(t *testing.T) {
	registry := rule.NewRegistry()
	cfg := policy.Default()

	// Enable the 5 PG-native alter action forbid rules
	pgRules := []string{
		"ddl.alter.set_data_type.forbid",
		"ddl.alter.set_default.forbid",
		"ddl.alter.drop_default.forbid",
		"ddl.alter.set_not_null.forbid",
		"ddl.alter.drop_not_null.forbid",
	}
	for _, ruleID := range pgRules {
		cfg.Rules[ruleID] = policy.RulePolicy{
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{"forbid": true},
		}
	}

	if err := Register(registry, cfg); err != nil {
		t.Fatalf("register: %v", err)
	}

	// Test PostgreSQL dialect: all 5 forbid rules + 1 migration-safety rule should fire
	t.Run("postgresql_fires_all_five_rules", func(t *testing.T) {
		stmt := spec.Statement{
			Kind:    spec.KindDDL,
			Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{
				Table: &spec.Table{Name: "users"},
				Alter: []spec.Alter{
					{Action: "set_data_type", Name: "status"},
					{Action: "set_default", Name: "created_at"},
					{Action: "drop_default", Name: "updated_at"},
					{Action: "set_not_null", Name: "email"},
					{Action: "drop_not_null", Name: "phone"},
				},
			},
		}

		findings, err := registry.EvaluateStatement(stmt)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}

		if len(findings) != 6 {
			t.Fatalf("expected 6 findings for PostgreSQL, got %d: %v", len(findings), findings)
		}

		wantIDs := []string{
			"ddl.alter.set_data_type.forbid",
			"ddl.alter.set_default.forbid",
			"ddl.alter.drop_default.forbid",
			"ddl.alter.set_not_null.forbid",
			"ddl.alter.drop_not_null.forbid",
		}
		for i, want := range wantIDs {
			if findings[i].RuleID != want {
				t.Fatalf("expected finding %d to use rule %q, got %q", i, want, findings[i].RuleID)
			}
		}
	})

	// Test MySQL dialect: none of the 5 PG-native rules should fire
	t.Run("mysql_skips_pg_native_rules", func(t *testing.T) {
		stmt := spec.Statement{
			Kind:    spec.KindDDL,
			Dialect: spec.DialectMySQL,
			DDL: &spec.DDL{
				Table: &spec.Table{Name: "users"},
				Alter: []spec.Alter{
					{Action: "set_data_type", Name: "status"},
					{Action: "set_default", Name: "created_at"},
					{Action: "drop_default", Name: "updated_at"},
					{Action: "set_not_null", Name: "email"},
					{Action: "drop_not_null", Name: "phone"},
				},
			},
		}

		findings, err := registry.EvaluateStatement(stmt)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}

		// Filter findings to check if any PG-native rules fired
		pgNativeRuleIDs := map[string]bool{
			"ddl.alter.set_data_type.forbid": true,
			"ddl.alter.set_default.forbid":   true,
			"ddl.alter.drop_default.forbid":  true,
			"ddl.alter.set_not_null.forbid":  true,
			"ddl.alter.drop_not_null.forbid": true,
		}
		for _, finding := range findings {
			if pgNativeRuleIDs[finding.RuleID] {
				t.Fatalf("expected PG-native rule %q not to fire for MySQL dialect", finding.RuleID)
			}
		}
	})

	// Test TiDB dialect: none of the 5 PG-native rules should fire
	t.Run("tidb_skips_pg_native_rules", func(t *testing.T) {
		stmt := spec.Statement{
			Kind:    spec.KindDDL,
			Dialect: spec.DialectTiDB,
			DDL: &spec.DDL{
				Table: &spec.Table{Name: "users"},
				Alter: []spec.Alter{
					{Action: "set_data_type", Name: "status"},
					{Action: "set_default", Name: "created_at"},
					{Action: "drop_default", Name: "updated_at"},
					{Action: "set_not_null", Name: "email"},
					{Action: "drop_not_null", Name: "phone"},
				},
			},
		}

		findings, err := registry.EvaluateStatement(stmt)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}

		pgNativeRuleIDs := map[string]bool{
			"ddl.alter.set_data_type.forbid": true,
			"ddl.alter.set_default.forbid":   true,
			"ddl.alter.drop_default.forbid":  true,
			"ddl.alter.set_not_null.forbid":  true,
			"ddl.alter.drop_not_null.forbid": true,
		}
		for _, finding := range findings {
			if pgNativeRuleIDs[finding.RuleID] {
				t.Fatalf("expected PG-native rule %q not to fire for TiDB dialect", finding.RuleID)
			}
		}
	})
}

func TestRegisterAddsPGNativeExplicitDefaultChangeSemanticRules(t *testing.T) {
	registry := rule.NewRegistry()
	cfg := policy.Default()

	if err := Register(registry, cfg); err != nil {
		t.Fatalf("register: %v", err)
	}

	pgSemanticRuleIDs := map[string]bool{
		ruleIDAlterSetDefaultExplicitDefaultChangeForbid:  true,
		ruleIDAlterDropDefaultExplicitDefaultChangeForbid: true,
	}

	t.Run("postgresql_fires_explicit_default_change_semantic_rules", func(t *testing.T) {
		stmt := spec.Statement{
			Kind:    spec.KindDDL,
			Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{
				Operation: spec.DDLOperationAlterTable,
				Table:     &spec.Table{Name: "users"},
				Alter: []spec.Alter{
					{
						Action: "set_default",
						Name:   "status",
						Column: &spec.AlterColumn{
							OldName: "status",
							Change:  &spec.AlterColumnChange{TouchesDefault: true},
						},
					},
					{
						Action: "drop_default",
						Name:   "email",
						Column: &spec.AlterColumn{
							OldName: "email",
							Change:  &spec.AlterColumnChange{TouchesDefault: true},
						},
					},
				},
			},
		}

		findings, err := registry.EvaluateStatement(stmt)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}

		setDefaultCount := 0
		dropDefaultCount := 0
		for _, f := range findings {
			if f.RuleID == ruleIDAlterSetDefaultExplicitDefaultChangeForbid {
				setDefaultCount++
			}
			if f.RuleID == ruleIDAlterDropDefaultExplicitDefaultChangeForbid {
				dropDefaultCount++
			}
		}
		if setDefaultCount != 1 {
			t.Fatalf("expected 1 set_default explicit_default_change finding, got %d", setDefaultCount)
		}
		if dropDefaultCount != 1 {
			t.Fatalf("expected 1 drop_default explicit_default_change finding, got %d", dropDefaultCount)
		}
	})

	t.Run("mysql_modify_column_does_not_trigger_pg_semantic_rules", func(t *testing.T) {
		stmt := spec.Statement{
			Kind:    spec.KindDDL,
			Dialect: spec.DialectMySQL,
			DDL: &spec.DDL{
				Operation: spec.DDLOperationAlterTable,
				Table:     &spec.Table{Name: "users"},
				Alter: []spec.Alter{
					{
						Action: "modify_column",
						Name:   "status",
						Column: &spec.AlterColumn{
							OldName:    "status",
							Definition: &spec.Column{Name: "status"},
							Change:     &spec.AlterColumnChange{TouchesDefault: true},
						},
					},
				},
			},
		}

		findings, err := registry.EvaluateStatement(stmt)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		for _, f := range findings {
			if pgSemanticRuleIDs[f.RuleID] {
				t.Fatalf("expected PG-native semantic rule %q not to fire for MySQL modify_column", f.RuleID)
			}
		}
	})

	t.Run("tidb_modify_column_does_not_trigger_pg_semantic_rules", func(t *testing.T) {
		stmt := spec.Statement{
			Kind:    spec.KindDDL,
			Dialect: spec.DialectTiDB,
			DDL: &spec.DDL{
				Operation: spec.DDLOperationAlterTable,
				Table:     &spec.Table{Name: "users"},
				Alter: []spec.Alter{
					{
						Action: "modify_column",
						Name:   "status",
						Column: &spec.AlterColumn{
							OldName:    "status",
							Definition: &spec.Column{Name: "status"},
							Change:     &spec.AlterColumnChange{TouchesDefault: true},
						},
					},
				},
			},
		}

		findings, err := registry.EvaluateStatement(stmt)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		for _, f := range findings {
			if pgSemanticRuleIDs[f.RuleID] {
				t.Fatalf("expected PG-native semantic rule %q not to fire for TiDB modify_column", f.RuleID)
			}
		}
	})
}

func TestRegisterPGExplicitDefaultChangeDoesNotBreakExistingMySQLRules(t *testing.T) {
	registry := rule.NewRegistry()
	cfg := policy.Default()

	if err := Register(registry, cfg); err != nil {
		t.Fatalf("register: %v", err)
	}

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectMySQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "users"},
			Alter: []spec.Alter{
				{
					Action: "modify_column",
					Name:   "status",
					Column: &spec.AlterColumn{
						OldName:    "status",
						Definition: &spec.Column{Name: "status", Type: "varchar(32)"},
						Change:     &spec.AlterColumnChange{TouchesDefault: true},
					},
				},
			},
		},
	}

	findings, err := registry.EvaluateStatement(stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}

	mysqlExplicitDefaultFound := false
	for _, f := range findings {
		if f.RuleID == ruleIDAlterModifyColumnExplicitDefaultChangeForbid {
			mysqlExplicitDefaultFound = true
		}
	}
	if !mysqlExplicitDefaultFound {
		t.Fatal("expected existing MySQL modify_column explicit_default_change rule to still fire")
	}
}
func TestRegisterAddsPGNativeExplicitNullabilityChangeSemanticRules(t *testing.T) {
	registry := rule.NewRegistry()
	cfg := policy.Default()

	if err := Register(registry, cfg); err != nil {
		t.Fatalf("register: %v", err)
	}

	pgNullabilityRuleIDs := map[string]bool{
		ruleIDAlterSetNotNullExplicitNullabilityChangeForbid:  true,
		ruleIDAlterDropNotNullExplicitNullabilityChangeForbid: true,
	}

	t.Run("postgresql_fires_explicit_nullability_change_semantic_rules", func(t *testing.T) {
		stmt := spec.Statement{
			Kind:    spec.KindDDL,
			Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{
				Operation: spec.DDLOperationAlterTable,
				Table:     &spec.Table{Name: "users"},
				Alter: []spec.Alter{
					{
						Action: "set_not_null",
						Name:   "email",
						Column: &spec.AlterColumn{
							OldName: "email",
							Change:  &spec.AlterColumnChange{TouchesNullability: true},
						},
					},
					{
						Action: "drop_not_null",
						Name:   "phone",
						Column: &spec.AlterColumn{
							OldName: "phone",
							Change:  &spec.AlterColumnChange{TouchesNullability: true},
						},
					},
				},
			},
		}

		findings, err := registry.EvaluateStatement(stmt)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}

		setNotNullCount := 0
		dropNotNullCount := 0
		for _, f := range findings {
			if f.RuleID == ruleIDAlterSetNotNullExplicitNullabilityChangeForbid {
				setNotNullCount++
			}
			if f.RuleID == ruleIDAlterDropNotNullExplicitNullabilityChangeForbid {
				dropNotNullCount++
			}
		}
		if setNotNullCount != 1 {
			t.Fatalf("expected 1 set_not_null explicit_nullability_change finding, got %d", setNotNullCount)
		}
		if dropNotNullCount != 1 {
			t.Fatalf("expected 1 drop_not_null explicit_nullability_change finding, got %d", dropNotNullCount)
		}
	})

	t.Run("mysql_modify_column_does_not_trigger_pg_nullability_semantic_rules", func(t *testing.T) {
		stmt := spec.Statement{
			Kind:    spec.KindDDL,
			Dialect: spec.DialectMySQL,
			DDL: &spec.DDL{
				Operation: spec.DDLOperationAlterTable,
				Table:     &spec.Table{Name: "users"},
				Alter: []spec.Alter{
					{
						Action: "modify_column",
						Name:   "email",
						Column: &spec.AlterColumn{
							OldName:    "email",
							Definition: &spec.Column{Name: "email"},
							Change:     &spec.AlterColumnChange{TouchesNullability: true},
						},
					},
				},
			},
		}

		findings, err := registry.EvaluateStatement(stmt)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		for _, f := range findings {
			if pgNullabilityRuleIDs[f.RuleID] {
				t.Fatalf("expected PG-native semantic rule %q not to fire for MySQL modify_column", f.RuleID)
			}
		}
	})

	t.Run("tidb_modify_column_does_not_trigger_pg_nullability_semantic_rules", func(t *testing.T) {
		stmt := spec.Statement{
			Kind:    spec.KindDDL,
			Dialect: spec.DialectTiDB,
			DDL: &spec.DDL{
				Operation: spec.DDLOperationAlterTable,
				Table:     &spec.Table{Name: "users"},
				Alter: []spec.Alter{
					{
						Action: "modify_column",
						Name:   "email",
						Column: &spec.AlterColumn{
							OldName:    "email",
							Definition: &spec.Column{Name: "email"},
							Change:     &spec.AlterColumnChange{TouchesNullability: true},
						},
					},
				},
			},
		}

		findings, err := registry.EvaluateStatement(stmt)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		for _, f := range findings {
			if pgNullabilityRuleIDs[f.RuleID] {
				t.Fatalf("expected PG-native semantic rule %q not to fire for TiDB modify_column", f.RuleID)
			}
		}
	})
}

func TestRegisterPGExplicitNullabilityChangeDoesNotBreakExistingMySQLRules(t *testing.T) {
	registry := rule.NewRegistry()
	cfg := policy.Default()

	if err := Register(registry, cfg); err != nil {
		t.Fatalf("register: %v", err)
	}

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectMySQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "users"},
			Alter: []spec.Alter{
				{
					Action: "modify_column",
					Name:   "email",
					Column: &spec.AlterColumn{
						OldName:    "email",
						Definition: &spec.Column{Name: "email", Type: "varchar(32)"},
						Change:     &spec.AlterColumnChange{TouchesNullability: true},
					},
				},
			},
		},
	}

	findings, err := registry.EvaluateStatement(stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}

	mysqlNullabilityFound := false
	for _, f := range findings {
		if f.RuleID == ruleIDAlterModifyColumnExplicitNullabilityChangeForbid {
			mysqlNullabilityFound = true
		}
	}
	if !mysqlNullabilityFound {
		t.Fatal("expected existing MySQL modify_column explicit_nullability_change rule to still fire")
	}
}

func assertNoDDLRuleIDs(t *testing.T, findings []rule.Finding, forbidden []string) {
	t.Helper()
	all := make([]string, 0, len(findings))
	seen := make(map[string]struct{}, len(findings))
	for _, finding := range findings {
		all = append(all, finding.RuleID)
		seen[finding.RuleID] = struct{}{}
	}
	sort.Strings(all)
	leaked := make([]string, 0)
	for _, ruleID := range forbidden {
		if _, ok := seen[ruleID]; ok {
			leaked = append(leaked, ruleID)
		}
	}
	if len(leaked) > 0 {
		sort.Strings(leaked)
		t.Fatalf("expected no forbidden finding rule IDs; leaked=%v all=%v", leaked, all)
	}
}

func assertNoAppliedDDLRuleIDs(t *testing.T, applied []string, forbidden []string) {
	t.Helper()
	all := append([]string(nil), applied...)
	sort.Strings(all)
	seen := make(map[string]struct{}, len(all))
	for _, ruleID := range all {
		seen[ruleID] = struct{}{}
	}
	leaked := make([]string, 0)
	for _, ruleID := range forbidden {
		if _, ok := seen[ruleID]; ok {
			leaked = append(leaked, ruleID)
		}
	}
	if len(leaked) > 0 {
		sort.Strings(leaked)
		t.Fatalf("expected no forbidden applied rule IDs; leaked=%v applied=%v", leaked, all)
	}
}
