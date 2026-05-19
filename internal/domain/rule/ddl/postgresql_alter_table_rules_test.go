// Package ddl verifies PostgreSQL alter table advisory rule behavior.
// input: synthetic DDL statements with PostgreSQL alter table signals and cross-dialect policy controls
// output: focused coverage for the three PG-only alter table gap rules with PG-only gating
// pos: domain DDL rule test coverage for PG alter table advisories
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// ---------------------------------------------------------------------------
// Positive tests
// ---------------------------------------------------------------------------

func TestDropColumnAdvisoryFiresForPG(t *testing.T) {
	t.Parallel()
	r := mustNewDropColumnAdvisoryRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "users"},
			Alter: []spec.Alter{
				{Action: "drop_column", Name: "email"},
			},
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Level != rule.LevelWarning {
		t.Fatalf("expected warning level, got %q", findings[0].Level)
	}
	if findings[0].Metadata["action"] != "drop_column" {
		t.Fatalf("expected action=drop_column, got %v", findings[0].Metadata["action"])
	}
}

func TestValidateConstraintAdvisoryFiresForPG(t *testing.T) {
	t.Parallel()
	r := mustNewValidateConstraintAdvisoryRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "orders"},
			Alter: []spec.Alter{
				{Action: "validate_constraint", Name: "chk_positive_amount"},
			},
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Level != rule.LevelNotice {
		t.Fatalf("expected notice level, got %q", findings[0].Level)
	}
}

func TestAddColumnNullableNoticeFiresForPG(t *testing.T) {
	t.Parallel()
	r := mustNewAddColumnNullableNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "users"},
			Alter: []spec.Alter{
				{
					Action: "add_column",
					Name:   "nickname",
					Column: &spec.AlterColumn{
						Definition: &spec.Column{
							Name: "nickname",
							Type: "varchar(100)",
						},
					},
				},
			},
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Level != rule.LevelNotice {
		t.Fatalf("expected notice level, got %q", findings[0].Level)
	}
}

// ---------------------------------------------------------------------------
// Negative tests: nullable add-column skips covered cases
// ---------------------------------------------------------------------------

func TestAddColumnNullableSkipsNotNullWithDefault(t *testing.T) {
	t.Parallel()
	r := mustNewAddColumnNullableNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "users"},
			Alter: []spec.Alter{
				{
					Action: "add_column",
					Name:   "status",
					Column: &spec.AlterColumn{
						Definition: &spec.Column{
							Name:       "status",
							Type:       "varchar(20)",
							NotNull:    true,
							HasDefault: true,
						},
					},
				},
			},
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings for NOT NULL DEFAULT, got %d", len(findings))
	}
}

func TestAddColumnNullableSkipsNotNullNoDefault(t *testing.T) {
	t.Parallel()
	r := mustNewAddColumnNullableNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "users"},
			Alter: []spec.Alter{
				{
					Action: "add_column",
					Name:   "status",
					Column: &spec.AlterColumn{
						Definition: &spec.Column{
							Name:    "status",
							Type:    "varchar(20)",
							NotNull: true,
						},
					},
				},
			},
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings for NOT NULL without default, got %d", len(findings))
	}
}

func TestAddColumnNullableSkipsHasDefault(t *testing.T) {
	t.Parallel()
	r := mustNewAddColumnNullableNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "users"},
			Alter: []spec.Alter{
				{
					Action: "add_column",
					Name:   "status",
					Column: &spec.AlterColumn{
						Definition: &spec.Column{
							Name:       "status",
							Type:       "varchar(20)",
							HasDefault: true,
						},
					},
				},
			},
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings for nullable with default, got %d", len(findings))
	}
}

// ---------------------------------------------------------------------------
// Positive tests for unsupported-action rules
// ---------------------------------------------------------------------------

func TestSetSchemaAdvisoryFiresForPG(t *testing.T) {
	t.Parallel()
	r := mustNewSetSchemaAdvisoryRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "users"},
			Alter: []spec.Alter{
				{Action: "set_schema", Name: "archive"},
			},
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Level != rule.LevelNotice {
		t.Fatalf("expected notice level, got %q", findings[0].Level)
	}
	if findings[0].Metadata["action"] != "set_schema" {
		t.Fatalf("expected action=set_schema, got %v", findings[0].Metadata["action"])
	}
}

func TestOwnerAdvisoryFiresForPG(t *testing.T) {
	t.Parallel()
	r := mustNewOwnerAdvisoryRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "users"},
			Alter: []spec.Alter{
				{Action: "change_owner", Name: "app_owner"},
			},
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Level != rule.LevelNotice {
		t.Fatalf("expected notice level, got %q", findings[0].Level)
	}
	if findings[0].Metadata["action"] != "change_owner" {
		t.Fatalf("expected action=change_owner, got %v", findings[0].Metadata["action"])
	}
}

func TestEnableTriggerNoticeFiresForPG(t *testing.T) {
	t.Parallel()
	r := mustNewEnableTriggerNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "users"},
			Alter: []spec.Alter{
				{Action: "enable_trigger", Name: "trg_audit"},
			},
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Level != rule.LevelNotice {
		t.Fatalf("expected notice level, got %q", findings[0].Level)
	}
	if findings[0].Metadata["action"] != "enable_trigger" {
		t.Fatalf("expected action=enable_trigger, got %v", findings[0].Metadata["action"])
	}
}

func TestDisableTriggerWarnFiresForPG(t *testing.T) {
	t.Parallel()
	r := mustNewDisableTriggerWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "users"},
			Alter: []spec.Alter{
				{Action: "disable_trigger", Name: "trg_audit"},
			},
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Level != rule.LevelWarning {
		t.Fatalf("expected warning level, got %q", findings[0].Level)
	}
	if findings[0].Metadata["action"] != "disable_trigger" {
		t.Fatalf("expected action=disable_trigger, got %v", findings[0].Metadata["action"])
	}
}

func TestAttachPartitionAdvisoryFiresForPG(t *testing.T) {
	t.Parallel()
	r := mustNewAttachPartitionAdvisoryRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "measurement"},
			Alter: []spec.Alter{
				{Action: "attach_partition", Name: "measurement_y2026m04"},
			},
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Level != rule.LevelNotice {
		t.Fatalf("expected notice level, got %q", findings[0].Level)
	}
	if findings[0].Metadata["action"] != "attach_partition" {
		t.Fatalf("expected action=attach_partition, got %v", findings[0].Metadata["action"])
	}
}

func TestDetachPartitionWarnFiresForPG(t *testing.T) {
	t.Parallel()
	r := mustNewDetachPartitionWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "measurement"},
			Alter: []spec.Alter{
				{Action: "detach_partition", Name: "measurement_y2026m04"},
			},
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Level != rule.LevelWarning {
		t.Fatalf("expected warning level, got %q", findings[0].Level)
	}
	if findings[0].Metadata["action"] != "detach_partition" {
		t.Fatalf("expected action=detach_partition, got %v", findings[0].Metadata["action"])
	}
}

// ---------------------------------------------------------------------------
// Positive tests: logged-state rules
// ---------------------------------------------------------------------------

func TestSetLoggedNoticeFiresForPG(t *testing.T) {
	t.Parallel()
	r := mustNewSetLoggedNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "users"},
			Alter: []spec.Alter{
				{Action: "set_logged", Options: map[string]string{"logged": "true"}},
			},
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Level != rule.LevelNotice {
		t.Fatalf("expected notice level, got %q", findings[0].Level)
	}
	if findings[0].Metadata["action"] != "set_logged" {
		t.Fatalf("expected action=set_logged, got %v", findings[0].Metadata["action"])
	}
}

func TestSetUnloggedNoticeFiresForPG(t *testing.T) {
	t.Parallel()
	r := mustNewSetUnloggedNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "users"},
			Alter: []spec.Alter{
				{Action: "set_unlogged", Options: map[string]string{"logged": "false"}},
			},
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Level != rule.LevelNotice {
		t.Fatalf("expected notice level, got %q", findings[0].Level)
	}
	if findings[0].Metadata["action"] != "set_unlogged" {
		t.Fatalf("expected action=set_unlogged, got %v", findings[0].Metadata["action"])
	}
}

func TestSetLoggedRulesSkipNonPGDialects(t *testing.T) {
	t.Parallel()
	nonPGDialects := []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB}

	rules := []struct {
		name string
		r    rule.StatementRule
		stmt spec.Statement
	}{
		{
			name: "set_logged_notice",
			r:    mustNewSetLoggedNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
			stmt: spec.Statement{
				Kind: spec.KindDDL,
				DDL: &spec.DDL{
					Operation: spec.DDLOperationAlterTable,
					Table:     &spec.Table{Name: "users"},
					Alter:     []spec.Alter{{Action: "set_logged", Options: map[string]string{"logged": "true"}}},
				},
			},
		},
		{
			name: "set_unlogged_notice",
			r:    mustNewSetUnloggedNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
			stmt: spec.Statement{
				Kind: spec.KindDDL,
				DDL: &spec.DDL{
					Operation: spec.DDLOperationAlterTable,
					Table:     &spec.Table{Name: "users"},
					Alter:     []spec.Alter{{Action: "set_unlogged", Options: map[string]string{"logged": "false"}}},
				},
			},
		},
	}

	for _, rl := range rules {
		for _, dialect := range nonPGDialects {
			t.Run(rl.name+"_dialect_"+string(dialect), func(t *testing.T) {
				t.Parallel()
				stmt := rl.stmt
				stmt.Dialect = dialect
				if rl.r.AppliesTo(stmt) {
					t.Fatalf("expected AppliesTo() == false for dialect %s", dialect)
				}
				findings, err := rl.r.Evaluate(context.Background(), stmt)
				if err != nil {
					t.Fatalf("evaluate: %v", err)
				}
				if len(findings) != 0 {
					t.Fatalf("expected 0 findings for dialect %s, got %d", dialect, len(findings))
				}
			})
		}
	}
}

func TestSetLoggedRulesDoNotFireForSetTablespace(t *testing.T) {
	t.Parallel()
	loggedRule := mustNewSetLoggedNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})
	unloggedRule := mustNewSetUnloggedNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "users"},
			Alter: []spec.Alter{
				{Action: "unsupported-explicit", Name: "fastspace", Options: map[string]string{"feature": "set_tablespace"}},
			},
		},
	}

	for _, r := range []rule.StatementRule{loggedRule, unloggedRule} {
		findings, err := r.Evaluate(context.Background(), stmt)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for SET TABLESPACE, got %d", len(findings))
		}
	}
}

// ---------------------------------------------------------------------------
// Positive tests: replica identity rules
// ---------------------------------------------------------------------------

func TestReplicaIdentityFullWarnFiresForPG(t *testing.T) {
	t.Parallel()
	r := mustNewReplicaIdentityFullWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "users"},
			Alter: []spec.Alter{
				{Action: "replica_identity", Options: map[string]string{"identity": "full"}},
			},
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Level != rule.LevelWarning {
		t.Fatalf("expected warning level, got %q", findings[0].Level)
	}
	if findings[0].Metadata["identity"] != "full" {
		t.Fatalf("expected identity=full, got %v", findings[0].Metadata["identity"])
	}
}

func TestReplicaIdentityNothingWarnFiresForPG(t *testing.T) {
	t.Parallel()
	r := mustNewReplicaIdentityNothingWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "users"},
			Alter: []spec.Alter{
				{Action: "replica_identity", Options: map[string]string{"identity": "nothing"}},
			},
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Level != rule.LevelWarning {
		t.Fatalf("expected warning level, got %q", findings[0].Level)
	}
}

func TestReplicaIdentityUsingIndexNoticeFiresForPG(t *testing.T) {
	t.Parallel()
	r := mustNewReplicaIdentityUsingIndexNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "users"},
			Alter: []spec.Alter{
				{Action: "replica_identity", Options: map[string]string{"identity": "using_index", "index": "users_replica_identity_idx"}},
			},
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Level != rule.LevelNotice {
		t.Fatalf("expected notice level, got %q", findings[0].Level)
	}
	if findings[0].Metadata["index"] != "users_replica_identity_idx" {
		t.Fatalf("expected index=users_replica_identity_idx, got %v", findings[0].Metadata["index"])
	}
}

// ---------------------------------------------------------------------------
// Negative tests: replica identity rules
// ---------------------------------------------------------------------------

func TestReplicaIdentityDefaultDoesNotFire(t *testing.T) {
	t.Parallel()
	rules := []rule.StatementRule{
		mustNewReplicaIdentityFullWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}),
		mustNewReplicaIdentityNothingWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}),
		mustNewReplicaIdentityUsingIndexNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
	}

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "users"},
			Alter: []spec.Alter{
				{Action: "replica_identity", Options: map[string]string{"identity": "default"}},
			},
		},
	}

	for i, r := range rules {
		findings, err := r.Evaluate(context.Background(), stmt)
		if err != nil {
			t.Fatalf("rule %d evaluate: %v", i, err)
		}
		if len(findings) != 0 {
			t.Fatalf("rule %d: expected 0 findings for DEFAULT identity, got %d", i, len(findings))
		}
	}
}

func TestReplicaIdentityRulesSkipWrongAction(t *testing.T) {
	t.Parallel()
	r := mustNewReplicaIdentityFullWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "users"},
			Alter: []spec.Alter{
				{Action: "drop_column", Name: "email"},
			},
		},
	}

	if r.AppliesTo(stmt) {
		t.Fatalf("expected AppliesTo() == false for wrong action")
	}
	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings for wrong action, got %d", len(findings))
	}
}

func TestReplicaIdentityRulesSkipWrongIdentityOption(t *testing.T) {
	t.Parallel()
	r := mustNewReplicaIdentityFullWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "users"},
			Alter: []spec.Alter{
				{Action: "replica_identity", Options: map[string]string{"identity": "nothing"}},
			},
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings for wrong identity option, got %d", len(findings))
	}
}

func TestReplicaIdentityRulesSkipNonPGDialects(t *testing.T) {
	t.Parallel()
	nonPGDialects := []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB}

	replicaStmt := spec.Statement{
		Kind: spec.KindDDL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "users"},
			Alter: []spec.Alter{
				{Action: "replica_identity", Options: map[string]string{"identity": "full"}},
			},
		},
	}

	rules := []struct {
		name string
		r    rule.StatementRule
	}{
		{"full_warn", mustNewReplicaIdentityFullWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})},
		{"nothing_warn", mustNewReplicaIdentityNothingWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})},
		{"using_index_notice", mustNewReplicaIdentityUsingIndexNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})},
	}

	for _, rl := range rules {
		for _, dialect := range nonPGDialects {
			t.Run(rl.name+"_dialect_"+string(dialect), func(t *testing.T) {
				t.Parallel()
				stmt := replicaStmt
				stmt.Dialect = dialect
				if rl.r.AppliesTo(stmt) {
					t.Fatalf("expected AppliesTo() == false for dialect %s", dialect)
				}
				findings, err := rl.r.Evaluate(context.Background(), stmt)
				if err != nil {
					t.Fatalf("evaluate: %v", err)
				}
				if len(findings) != 0 {
					t.Fatalf("expected 0 findings for dialect %s, got %d", dialect, len(findings))
				}
			})
		}
	}
}

func TestTriggerRulesStillFireOnceForTriggerALL(t *testing.T) {
	t.Parallel()
	enableRule := mustNewEnableTriggerNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})
	disableRule := mustNewDisableTriggerWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	enableStmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "users"},
			Alter:     []spec.Alter{{Action: "enable_trigger", Name: "ALL"}},
		},
	}

	findings, err := enableRule.Evaluate(context.Background(), enableStmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for ENABLE TRIGGER ALL, got %d", len(findings))
	}
	if findings[0].Level != rule.LevelNotice {
		t.Fatalf("expected notice level, got %q", findings[0].Level)
	}

	disableStmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "users"},
			Alter:     []spec.Alter{{Action: "disable_trigger", Name: "USER"}},
		},
	}

	findings, err = disableRule.Evaluate(context.Background(), disableStmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for DISABLE TRIGGER USER, got %d", len(findings))
	}
}

// ---------------------------------------------------------------------------
// Cross-dialect negative tests
// ---------------------------------------------------------------------------

func TestPGAlterTableRulesSkipNonPGDialects(t *testing.T) {
	t.Parallel()
	nonPGDialects := []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB}

	baseStmt := spec.Statement{
		Kind: spec.KindDDL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "users"},
			Alter: []spec.Alter{
				{Action: "drop_column", Name: "email"},
			},
		},
	}

	rules := []struct {
		name string
		r    rule.StatementRule
		stmt spec.Statement
	}{
		{
			name: "drop_column_advisory",
			r:    mustNewDropColumnAdvisoryRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}),
			stmt: baseStmt,
		},
		{
			name: "validate_constraint_advisory",
			r:    mustNewValidateConstraintAdvisoryRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
			stmt: spec.Statement{
				Kind: spec.KindDDL,
				DDL: &spec.DDL{
					Operation: spec.DDLOperationAlterTable,
					Table:     &spec.Table{Name: "users"},
					Alter: []spec.Alter{
						{Action: "validate_constraint", Name: "chk"},
					},
				},
			},
		},
		{
			name: "add_column_nullable_notice",
			r:    mustNewAddColumnNullableNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
			stmt: spec.Statement{
				Kind: spec.KindDDL,
				DDL: &spec.DDL{
					Operation: spec.DDLOperationAlterTable,
					Table:     &spec.Table{Name: "users"},
					Alter: []spec.Alter{
						{
							Action: "add_column",
							Name:   "nick",
							Column: &spec.AlterColumn{
								Definition: &spec.Column{Name: "nick", Type: "text"},
							},
						},
					},
				},
			},
		},
		{
			name: "set_schema_advisory",
			r:    mustNewSetSchemaAdvisoryRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
			stmt: spec.Statement{
				Kind: spec.KindDDL,
				DDL: &spec.DDL{
					Operation: spec.DDLOperationAlterTable,
					Table:     &spec.Table{Name: "users"},
					Alter:     []spec.Alter{{Action: "set_schema", Name: "archive"}},
				},
			},
		},
		{
			name: "owner_advisory",
			r:    mustNewOwnerAdvisoryRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
			stmt: spec.Statement{
				Kind: spec.KindDDL,
				DDL: &spec.DDL{
					Operation: spec.DDLOperationAlterTable,
					Table:     &spec.Table{Name: "users"},
					Alter:     []spec.Alter{{Action: "change_owner", Name: "app_owner"}},
				},
			},
		},
		{
			name: "enable_trigger_notice",
			r:    mustNewEnableTriggerNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
			stmt: spec.Statement{
				Kind: spec.KindDDL,
				DDL: &spec.DDL{
					Operation: spec.DDLOperationAlterTable,
					Table:     &spec.Table{Name: "users"},
					Alter:     []spec.Alter{{Action: "enable_trigger", Name: "trg_audit"}},
				},
			},
		},
		{
			name: "disable_trigger_warn",
			r:    mustNewDisableTriggerWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}),
			stmt: spec.Statement{
				Kind: spec.KindDDL,
				DDL: &spec.DDL{
					Operation: spec.DDLOperationAlterTable,
					Table:     &spec.Table{Name: "users"},
					Alter:     []spec.Alter{{Action: "disable_trigger", Name: "trg_audit"}},
				},
			},
		},
		{
			name: "attach_partition_advisory",
			r:    mustNewAttachPartitionAdvisoryRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
			stmt: spec.Statement{
				Kind: spec.KindDDL,
				DDL: &spec.DDL{
					Operation: spec.DDLOperationAlterTable,
					Table:     &spec.Table{Name: "measurement"},
					Alter:     []spec.Alter{{Action: "attach_partition", Name: "measurement_y2026m04"}},
				},
			},
		},
		{
			name: "detach_partition_warn",
			r:    mustNewDetachPartitionWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}),
			stmt: spec.Statement{
				Kind: spec.KindDDL,
				DDL: &spec.DDL{
					Operation: spec.DDLOperationAlterTable,
					Table:     &spec.Table{Name: "measurement"},
					Alter:     []spec.Alter{{Action: "detach_partition", Name: "measurement_y2026m04"}},
				},
			},
		},
		{
			name: "set_logged_notice",
			r:    mustNewSetLoggedNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
			stmt: spec.Statement{
				Kind: spec.KindDDL,
				DDL: &spec.DDL{
					Operation: spec.DDLOperationAlterTable,
					Table:     &spec.Table{Name: "users"},
					Alter:     []spec.Alter{{Action: "set_logged", Options: map[string]string{"logged": "true"}}},
				},
			},
		},
		{
			name: "set_unlogged_notice",
			r:    mustNewSetUnloggedNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
			stmt: spec.Statement{
				Kind: spec.KindDDL,
				DDL: &spec.DDL{
					Operation: spec.DDLOperationAlterTable,
					Table:     &spec.Table{Name: "users"},
					Alter:     []spec.Alter{{Action: "set_unlogged", Options: map[string]string{"logged": "false"}}},
				},
			},
		},
		{
			name: "replica_identity_full_warn",
			r:    mustNewReplicaIdentityFullWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}),
			stmt: spec.Statement{
				Kind: spec.KindDDL,
				DDL: &spec.DDL{
					Operation: spec.DDLOperationAlterTable,
					Table:     &spec.Table{Name: "users"},
					Alter:     []spec.Alter{{Action: "replica_identity", Options: map[string]string{"identity": "full"}}},
				},
			},
		},
		{
			name: "replica_identity_nothing_warn",
			r:    mustNewReplicaIdentityNothingWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}),
			stmt: spec.Statement{
				Kind: spec.KindDDL,
				DDL: &spec.DDL{
					Operation: spec.DDLOperationAlterTable,
					Table:     &spec.Table{Name: "users"},
					Alter:     []spec.Alter{{Action: "replica_identity", Options: map[string]string{"identity": "nothing"}}},
				},
			},
		},
		{
			name: "replica_identity_using_index_notice",
			r:    mustNewReplicaIdentityUsingIndexNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
			stmt: spec.Statement{
				Kind: spec.KindDDL,
				DDL: &spec.DDL{
					Operation: spec.DDLOperationAlterTable,
					Table:     &spec.Table{Name: "users"},
					Alter:     []spec.Alter{{Action: "replica_identity", Options: map[string]string{"identity": "using_index", "index": "idx"}}},
				},
			},
		},
	}

	for _, rl := range rules {
		for _, dialect := range nonPGDialects {
			t.Run(rl.name+"_dialect_"+string(dialect), func(t *testing.T) {
				t.Parallel()
				stmt := rl.stmt
				stmt.Dialect = dialect
				if rl.r.AppliesTo(stmt) {
					t.Fatalf("expected AppliesTo() == false for dialect %s", dialect)
				}
				findings, err := rl.r.Evaluate(context.Background(), stmt)
				if err != nil {
					t.Fatalf("evaluate: %v", err)
				}
				if len(findings) != 0 {
					t.Fatalf("expected 0 findings for dialect %s, got %d", dialect, len(findings))
				}
			})
		}
	}
}

func TestPGAlterTableRulesSkipWrongAction(t *testing.T) {
	t.Parallel()
	r := mustNewDropColumnAdvisoryRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "users"},
			Alter: []spec.Alter{
				{Action: "add_column", Name: "email"},
			},
		},
	}

	if r.AppliesTo(stmt) {
		t.Fatalf("expected AppliesTo() == false for wrong action")
	}
	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings for wrong action, got %d", len(findings))
	}
}

// ---------------------------------------------------------------------------
// Registration tests
// ---------------------------------------------------------------------------

func TestRegisterIncludesPGAlterTableRules(t *testing.T) {
	t.Parallel()
	registry := rule.NewRegistry()
	cfg := policy.Default()

	pgAlterRuleIDs := []string{
		ruleIDPGAlterDropColumnAdvisory,
		ruleIDPGAlterValidateConstraintAdvisory,
		ruleIDPGAlterAddColumnNullableNotice,
		ruleIDPGAlterSetSchemaAdvisory,
		ruleIDPGAlterOwnerAdvisory,
		ruleIDPGAlterEnableTriggerNotice,
		ruleIDPGAlterDisableTriggerWarn,
		ruleIDPGAlterAttachPartitionAdvisory,
		ruleIDPGAlterDetachPartitionWarn,
		ruleIDPGAlterLoggedNotice,
		ruleIDPGAlterUnloggedNotice,
		ruleIDPGAlterReplicaIdentityFullWarn,
		ruleIDPGAlterReplicaIdentityNothingWarn,
		ruleIDPGAlterReplicaIdentityUsingIndexNotice,
		ruleIDPGAlterEnableReplicaTriggerNotice,
		ruleIDPGAlterEnableAlwaysTriggerNotice,
		ruleIDPGAlterEnableRuleNotice,
		ruleIDPGAlterDisableRuleWarn,
		ruleIDPGAlterEnableReplicaRuleNotice,
		ruleIDPGAlterEnableAlwaysRuleNotice,
	}
	for _, ruleID := range pgAlterRuleIDs {
		cfg.Rules[ruleID] = policy.RulePolicy{
			Enabled: true,
			Level:   rule.LevelNotice,
		}
	}

	if err := Register(registry, cfg); err != nil {
		t.Fatalf("register: %v", err)
	}

	t.Run("drop_column_advisory_fires_for_pg", func(t *testing.T) {
		t.Parallel()
		stmt := spec.Statement{
			Kind:    spec.KindDDL,
			Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{
				Operation: spec.DDLOperationAlterTable,
				Table:     &spec.Table{Name: "users"},
				Alter:     []spec.Alter{{Action: "drop_column", Name: "email"}},
			},
		}
		findings, err := registry.EvaluateStatement(context.Background(), stmt)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		found := false
		for _, f := range findings {
			if f.RuleID == ruleIDPGAlterDropColumnAdvisory {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected drop_column_advisory rule to fire, got %d findings", len(findings))
		}
	})

	t.Run("validate_constraint_advisory_fires_for_pg", func(t *testing.T) {
		t.Parallel()
		stmt := spec.Statement{
			Kind:    spec.KindDDL,
			Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{
				Operation: spec.DDLOperationAlterTable,
				Table:     &spec.Table{Name: "orders"},
				Alter:     []spec.Alter{{Action: "validate_constraint", Name: "chk_amt"}},
			},
		}
		findings, err := registry.EvaluateStatement(context.Background(), stmt)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		found := false
		for _, f := range findings {
			if f.RuleID == ruleIDPGAlterValidateConstraintAdvisory {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected validate_constraint_advisory rule to fire, got %d findings", len(findings))
		}
	})

	t.Run("pg_alter_table_rules_do_not_fire_for_mysql", func(t *testing.T) {
		t.Parallel()
		stmt := spec.Statement{
			Kind:    spec.KindDDL,
			Dialect: spec.DialectMySQL,
			DDL: &spec.DDL{
				Operation: spec.DDLOperationAlterTable,
				Table:     &spec.Table{Name: "users"},
				Alter:     []spec.Alter{{Action: "drop_column", Name: "email"}},
			},
		}
		findings, err := registry.EvaluateStatement(context.Background(), stmt)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		pgRuleIDs := map[string]bool{
			ruleIDPGAlterDropColumnAdvisory:         true,
			ruleIDPGAlterValidateConstraintAdvisory: true,
			ruleIDPGAlterAddColumnNullableNotice:    true,
			ruleIDPGAlterSetSchemaAdvisory:          true,
			ruleIDPGAlterOwnerAdvisory:              true,
			ruleIDPGAlterEnableTriggerNotice:        true,
			ruleIDPGAlterDisableTriggerWarn:         true,
			ruleIDPGAlterAttachPartitionAdvisory:    true,
			ruleIDPGAlterDetachPartitionWarn:        true,
			ruleIDPGAlterLoggedNotice:               true,
			ruleIDPGAlterUnloggedNotice:             true,
		}
		for _, f := range findings {
			if pgRuleIDs[f.RuleID] {
				t.Fatalf("expected PG rule %q not to fire for MySQL", f.RuleID)
			}
		}
	})
}

func TestDefaultPolicyIncludesPGAlterTableRules(t *testing.T) {
	t.Parallel()
	cfg := policy.Default()

	expected := []struct {
		id      string
		level   rule.Level
		enabled bool
	}{
		{ruleIDPGAlterDropColumnAdvisory, rule.LevelWarning, true},
		{ruleIDPGAlterValidateConstraintAdvisory, rule.LevelNotice, true},
		{ruleIDPGAlterAddColumnNullableNotice, rule.LevelNotice, true},
		{ruleIDPGAlterSetSchemaAdvisory, rule.LevelNotice, true},
		{ruleIDPGAlterOwnerAdvisory, rule.LevelNotice, true},
		{ruleIDPGAlterEnableTriggerNotice, rule.LevelNotice, true},
		{ruleIDPGAlterDisableTriggerWarn, rule.LevelWarning, true},
		{ruleIDPGAlterAttachPartitionAdvisory, rule.LevelNotice, true},
		{ruleIDPGAlterDetachPartitionWarn, rule.LevelWarning, true},
	}

	for _, exp := range expected {
		t.Run(exp.id, func(t *testing.T) {
			t.Parallel()
			p, ok := cfg.Rules[exp.id]
			if !ok {
				t.Fatalf("expected default policy to include %q", exp.id)
			}
			if !p.Enabled {
				t.Fatalf("expected %q to be enabled", exp.id)
			}
			if p.Level != exp.level {
				t.Fatalf("expected %q level %q, got %q", exp.id, exp.level, p.Level)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func mustNewDropColumnAdvisoryRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newDropColumnAdvisoryRule(cfg)
	if err != nil {
		t.Fatalf("new drop column advisory rule: %v", err)
	}
	return r
}

func mustNewValidateConstraintAdvisoryRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newValidateConstraintAdvisoryRule(cfg)
	if err != nil {
		t.Fatalf("new validate constraint advisory rule: %v", err)
	}
	return r
}

func mustNewAddColumnNullableNoticeRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newAddColumnNullableNoticeRule(cfg)
	if err != nil {
		t.Fatalf("new add column nullable notice rule: %v", err)
	}
	return r
}

func mustNewSetSchemaAdvisoryRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newSetSchemaAdvisoryRule(cfg)
	if err != nil {
		t.Fatalf("new set schema advisory rule: %v", err)
	}
	return r
}

func mustNewOwnerAdvisoryRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newOwnerAdvisoryRule(cfg)
	if err != nil {
		t.Fatalf("new owner advisory rule: %v", err)
	}
	return r
}

func mustNewEnableTriggerNoticeRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newEnableTriggerNoticeRule(cfg)
	if err != nil {
		t.Fatalf("new enable trigger notice rule: %v", err)
	}
	return r
}

func mustNewDisableTriggerWarnRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newDisableTriggerWarnRule(cfg)
	if err != nil {
		t.Fatalf("new disable trigger warn rule: %v", err)
	}
	return r
}

func mustNewAttachPartitionAdvisoryRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newAttachPartitionAdvisoryRule(cfg)
	if err != nil {
		t.Fatalf("new attach partition advisory rule: %v", err)
	}
	return r
}

func mustNewDetachPartitionWarnRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newDetachPartitionWarnRule(cfg)
	if err != nil {
		t.Fatalf("new detach partition warn rule: %v", err)
	}
	return r
}

func mustNewReplicaIdentityFullWarnRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newReplicaIdentityFullWarnRule(cfg)
	if err != nil {
		t.Fatalf("new replica identity full warn rule: %v", err)
	}
	return r
}

func mustNewReplicaIdentityNothingWarnRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newReplicaIdentityNothingWarnRule(cfg)
	if err != nil {
		t.Fatalf("new replica identity nothing warn rule: %v", err)
	}
	return r
}

func mustNewReplicaIdentityUsingIndexNoticeRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newReplicaIdentityUsingIndexNoticeRule(cfg)
	if err != nil {
		t.Fatalf("new replica identity using index notice rule: %v", err)
	}
	return r
}

func mustNewSetLoggedNoticeRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newSetLoggedNoticeRule(cfg)
	if err != nil {
		t.Fatalf("new set logged notice rule: %v", err)
	}
	return r
}

func mustNewSetUnloggedNoticeRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newSetUnloggedNoticeRule(cfg)
	if err != nil {
		t.Fatalf("new set unlogged notice rule: %v", err)
	}
	return r
}

func mustNewSetTablespaceNoticeRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newSetTablespaceNoticeRule(cfg)
	if err != nil {
		t.Fatalf("new set tablespace notice rule: %v", err)
	}
	return r
}

func mustNewEnableReplicaTriggerNoticeRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newEnableReplicaTriggerNoticeRule(cfg)
	if err != nil {
		t.Fatalf("newEnableReplicaTriggerNoticeRule: %v", err)
	}
	return r
}

func mustNewEnableAlwaysTriggerNoticeRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newEnableAlwaysTriggerNoticeRule(cfg)
	if err != nil {
		t.Fatalf("newEnableAlwaysTriggerNoticeRule: %v", err)
	}
	return r
}

func mustNewEnableRuleNoticeRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newEnableRuleNoticeRule(cfg)
	if err != nil {
		t.Fatalf("newEnableRuleNoticeRule: %v", err)
	}
	return r
}

func mustNewDisableRuleWarnRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newDisableRuleWarnRule(cfg)
	if err != nil {
		t.Fatalf("newDisableRuleWarnRule: %v", err)
	}
	return r
}

func mustNewEnableReplicaRuleNoticeRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newEnableReplicaRuleNoticeRule(cfg)
	if err != nil {
		t.Fatalf("newEnableReplicaRuleNoticeRule: %v", err)
	}
	return r
}

func mustNewEnableAlwaysRuleNoticeRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newEnableAlwaysRuleNoticeRule(cfg)
	if err != nil {
		t.Fatalf("newEnableAlwaysRuleNoticeRule: %v", err)
	}
	return r
}

func mustNewSetAccessMethodWarnRule(t *testing.T, cfg policy.RulePolicy) rule.StatementRule {
	t.Helper()
	r, err := newSetAccessMethodWarnRule(cfg)
	if err != nil {
		t.Fatalf("new set access method warn rule: %v", err)
	}
	return r
}

func TestSetTablespaceNoticeFiresForPG(t *testing.T) {
	t.Parallel()
	r := mustNewSetTablespaceNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "users"},
			Alter: []spec.Alter{
				{Action: "set_tablespace", Options: map[string]string{"tablespace": "fastspace"}},
			},
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Level != rule.LevelNotice {
		t.Fatalf("expected notice level, got %q", findings[0].Level)
	}
	if findings[0].Metadata["action"] != "set_tablespace" {
		t.Fatalf("expected action=set_tablespace, got %v", findings[0].Metadata["action"])
	}
	if findings[0].Metadata["tablespace"] != "fastspace" {
		t.Fatalf("expected tablespace=fastspace, got %v", findings[0].Metadata["tablespace"])
	}
}

func TestSetAccessMethodWarnFiresForPG(t *testing.T) {
	t.Parallel()
	r := mustNewSetAccessMethodWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "users"},
			Alter: []spec.Alter{
				{Action: "set_access_method", Options: map[string]string{"access_method": "heap", "is_default": "false"}},
			},
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Level != rule.LevelWarning {
		t.Fatalf("expected warning level, got %q", findings[0].Level)
	}
	if findings[0].Metadata["action"] != "set_access_method" {
		t.Fatalf("expected action=set_access_method, got %v", findings[0].Metadata["action"])
	}
	if findings[0].Metadata["access_method"] != "heap" {
		t.Fatalf("expected access_method=heap, got %v", findings[0].Metadata["access_method"])
	}
}

func TestSetAccessMethodWarnFiresForDefault(t *testing.T) {
	t.Parallel()
	r := mustNewSetAccessMethodWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "users"},
			Alter: []spec.Alter{
				{Action: "set_access_method", Options: map[string]string{"access_method": "default", "is_default": "true"}},
			},
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Metadata["access_method"] != "default" {
		t.Fatalf("expected access_method=default, got %v", findings[0].Metadata["access_method"])
	}
	if findings[0].Metadata["is_default"] != "true" {
		t.Fatalf("expected is_default=true, got %v", findings[0].Metadata["is_default"])
	}
}

func TestStorageLayoutRulesSkipNonPGDialects(t *testing.T) {
	t.Parallel()
	nonPGDialects := []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB}

	rules := []struct {
		name string
		r    rule.StatementRule
		stmt spec.Statement
	}{
		{
			name: "set_tablespace_notice",
			r:    mustNewSetTablespaceNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
			stmt: spec.Statement{
				Kind: spec.KindDDL,
				DDL: &spec.DDL{
					Operation: spec.DDLOperationAlterTable,
					Table:     &spec.Table{Name: "users"},
					Alter:     []spec.Alter{{Action: "set_tablespace", Options: map[string]string{"tablespace": "fastspace"}}},
				},
			},
		},
		{
			name: "set_access_method_warn",
			r:    mustNewSetAccessMethodWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}),
			stmt: spec.Statement{
				Kind: spec.KindDDL,
				DDL: &spec.DDL{
					Operation: spec.DDLOperationAlterTable,
					Table:     &spec.Table{Name: "users"},
					Alter:     []spec.Alter{{Action: "set_access_method", Options: map[string]string{"access_method": "heap", "is_default": "false"}}},
				},
			},
		},
	}

	for _, rl := range rules {
		for _, dialect := range nonPGDialects {
			t.Run(rl.name+"_dialect_"+string(dialect), func(t *testing.T) {
				t.Parallel()
				stmt := rl.stmt
				stmt.Dialect = dialect
				if rl.r.AppliesTo(stmt) {
					t.Fatalf("expected AppliesTo() == false for dialect %s", dialect)
				}
				findings, err := rl.r.Evaluate(context.Background(), stmt)
				if err != nil {
					t.Fatalf("evaluate: %v", err)
				}
				if len(findings) != 0 {
					t.Fatalf("expected 0 findings for dialect %s, got %d", dialect, len(findings))
				}
			})
		}
	}
}

func TestStorageLayoutRulesDoNotFireForWrongAction(t *testing.T) {
	t.Parallel()
	tablespaceRule := mustNewSetTablespaceNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})
	accessMethodRule := mustNewSetAccessMethodWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "users"},
			Alter: []spec.Alter{
				{Action: "set_logged", Options: map[string]string{"logged": "true"}},
			},
		},
	}

	for _, r := range []rule.StatementRule{tablespaceRule, accessMethodRule} {
		findings, err := r.Evaluate(context.Background(), stmt)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for set_logged action, got %d", len(findings))
		}
	}
}

func TestEnableReplicaTriggerNoticeFiresForPG(t *testing.T) {
	t.Parallel()
	r := mustNewEnableReplicaTriggerNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "users"},
			Alter: []spec.Alter{
				{Action: "enable_replica_trigger", Name: "sync_trigger", Options: map[string]string{"trigger": "sync_trigger", "trigger_mode": "replica"}},
			},
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Level != rule.LevelNotice {
		t.Fatalf("expected notice level, got %q", findings[0].Level)
	}
	if findings[0].Metadata["action"] != "enable_replica_trigger" {
		t.Fatalf("expected action=enable_replica_trigger, got %v", findings[0].Metadata["action"])
	}
	if findings[0].Metadata["trigger"] != "sync_trigger" {
		t.Fatalf("expected trigger=sync_trigger, got %v", findings[0].Metadata["trigger"])
	}
	if findings[0].Metadata["trigger_mode"] != "replica" {
		t.Fatalf("expected trigger_mode=replica, got %v", findings[0].Metadata["trigger_mode"])
	}
	if findings[0].Metadata["table"] != "users" {
		t.Fatalf("expected table=users, got %v", findings[0].Metadata["table"])
	}
}

func TestEnableAlwaysTriggerNoticeFiresForPG(t *testing.T) {
	t.Parallel()
	r := mustNewEnableAlwaysTriggerNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "users"},
			Alter: []spec.Alter{
				{Action: "enable_always_trigger", Name: "audit_trigger", Options: map[string]string{"trigger": "audit_trigger", "trigger_mode": "always"}},
			},
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Level != rule.LevelNotice {
		t.Fatalf("expected notice level, got %q", findings[0].Level)
	}
	if findings[0].Metadata["action"] != "enable_always_trigger" {
		t.Fatalf("expected action=enable_always_trigger, got %v", findings[0].Metadata["action"])
	}
	if findings[0].Metadata["trigger"] != "audit_trigger" {
		t.Fatalf("expected trigger=audit_trigger, got %v", findings[0].Metadata["trigger"])
	}
	if findings[0].Metadata["trigger_mode"] != "always" {
		t.Fatalf("expected trigger_mode=always, got %v", findings[0].Metadata["trigger_mode"])
	}
}

func TestEnableRuleNoticeFiresForPG(t *testing.T) {
	t.Parallel()
	r := mustNewEnableRuleNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "users"},
			Alter: []spec.Alter{
				{Action: "enable_rule", Name: "route_rule", Options: map[string]string{"rule": "route_rule"}},
			},
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Level != rule.LevelNotice {
		t.Fatalf("expected notice level, got %q", findings[0].Level)
	}
	if findings[0].Metadata["action"] != "enable_rule" {
		t.Fatalf("expected action=enable_rule, got %v", findings[0].Metadata["action"])
	}
	if findings[0].Metadata["rule"] != "route_rule" {
		t.Fatalf("expected rule=route_rule, got %v", findings[0].Metadata["rule"])
	}
	if findings[0].Metadata["table"] != "users" {
		t.Fatalf("expected table=users, got %v", findings[0].Metadata["table"])
	}
}

func TestDisableRuleWarnFiresForPG(t *testing.T) {
	t.Parallel()
	r := mustNewDisableRuleWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "users"},
			Alter: []spec.Alter{
				{Action: "disable_rule", Name: "route_rule", Options: map[string]string{"rule": "route_rule"}},
			},
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Level != rule.LevelWarning {
		t.Fatalf("expected warning level, got %q", findings[0].Level)
	}
	if findings[0].Metadata["action"] != "disable_rule" {
		t.Fatalf("expected action=disable_rule, got %v", findings[0].Metadata["action"])
	}
	if findings[0].Metadata["rule"] != "route_rule" {
		t.Fatalf("expected rule=route_rule, got %v", findings[0].Metadata["rule"])
	}
}

func TestEnableReplicaRuleNoticeFiresForPG(t *testing.T) {
	t.Parallel()
	r := mustNewEnableReplicaRuleNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "users"},
			Alter: []spec.Alter{
				{Action: "enable_replica_rule", Name: "route_rule", Options: map[string]string{"rule": "route_rule", "rule_mode": "replica"}},
			},
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Level != rule.LevelNotice {
		t.Fatalf("expected notice level, got %q", findings[0].Level)
	}
	if findings[0].Metadata["action"] != "enable_replica_rule" {
		t.Fatalf("expected action=enable_replica_rule, got %v", findings[0].Metadata["action"])
	}
	if findings[0].Metadata["rule"] != "route_rule" {
		t.Fatalf("expected rule=route_rule, got %v", findings[0].Metadata["rule"])
	}
	if findings[0].Metadata["rule_mode"] != "replica" {
		t.Fatalf("expected rule_mode=replica, got %v", findings[0].Metadata["rule_mode"])
	}
}

func TestEnableAlwaysRuleNoticeFiresForPG(t *testing.T) {
	t.Parallel()
	r := mustNewEnableAlwaysRuleNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})

	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "users"},
			Alter: []spec.Alter{
				{Action: "enable_always_rule", Name: "route_rule", Options: map[string]string{"rule": "route_rule", "rule_mode": "always"}},
			},
		},
	}

	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Level != rule.LevelNotice {
		t.Fatalf("expected notice level, got %q", findings[0].Level)
	}
	if findings[0].Metadata["action"] != "enable_always_rule" {
		t.Fatalf("expected action=enable_always_rule, got %v", findings[0].Metadata["action"])
	}
	if findings[0].Metadata["rule"] != "route_rule" {
		t.Fatalf("expected rule=route_rule, got %v", findings[0].Metadata["rule"])
	}
	if findings[0].Metadata["rule_mode"] != "always" {
		t.Fatalf("expected rule_mode=always, got %v", findings[0].Metadata["rule_mode"])
	}
}

func TestTriggerRuleModeRulesSkipNonPGDialects(t *testing.T) {
	t.Parallel()
	nonPGDialects := []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB}

	rules := []struct {
		name string
		r    rule.StatementRule
		stmt spec.Statement
	}{
		{
			name: "enable_replica_trigger_notice",
			r:    mustNewEnableReplicaTriggerNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
			stmt: spec.Statement{
				Kind: spec.KindDDL,
				DDL: &spec.DDL{
					Operation: spec.DDLOperationAlterTable,
					Table:     &spec.Table{Name: "users"},
					Alter:     []spec.Alter{{Action: "enable_replica_trigger", Name: "sync_trigger", Options: map[string]string{"trigger": "sync_trigger", "trigger_mode": "replica"}}},
				},
			},
		},
		{
			name: "enable_always_trigger_notice",
			r:    mustNewEnableAlwaysTriggerNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
			stmt: spec.Statement{
				Kind: spec.KindDDL,
				DDL: &spec.DDL{
					Operation: spec.DDLOperationAlterTable,
					Table:     &spec.Table{Name: "users"},
					Alter:     []spec.Alter{{Action: "enable_always_trigger", Name: "audit_trigger", Options: map[string]string{"trigger": "audit_trigger", "trigger_mode": "always"}}},
				},
			},
		},
		{
			name: "enable_rule_notice",
			r:    mustNewEnableRuleNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
			stmt: spec.Statement{
				Kind: spec.KindDDL,
				DDL: &spec.DDL{
					Operation: spec.DDLOperationAlterTable,
					Table:     &spec.Table{Name: "users"},
					Alter:     []spec.Alter{{Action: "enable_rule", Name: "route_rule", Options: map[string]string{"rule": "route_rule"}}},
				},
			},
		},
		{
			name: "disable_rule_warn",
			r:    mustNewDisableRuleWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}),
			stmt: spec.Statement{
				Kind: spec.KindDDL,
				DDL: &spec.DDL{
					Operation: spec.DDLOperationAlterTable,
					Table:     &spec.Table{Name: "users"},
					Alter:     []spec.Alter{{Action: "disable_rule", Name: "route_rule", Options: map[string]string{"rule": "route_rule"}}},
				},
			},
		},
		{
			name: "enable_replica_rule_notice",
			r:    mustNewEnableReplicaRuleNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
			stmt: spec.Statement{
				Kind: spec.KindDDL,
				DDL: &spec.DDL{
					Operation: spec.DDLOperationAlterTable,
					Table:     &spec.Table{Name: "users"},
					Alter:     []spec.Alter{{Action: "enable_replica_rule", Name: "route_rule", Options: map[string]string{"rule": "route_rule", "rule_mode": "replica"}}},
				},
			},
		},
		{
			name: "enable_always_rule_notice",
			r:    mustNewEnableAlwaysRuleNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
			stmt: spec.Statement{
				Kind: spec.KindDDL,
				DDL: &spec.DDL{
					Operation: spec.DDLOperationAlterTable,
					Table:     &spec.Table{Name: "users"},
					Alter:     []spec.Alter{{Action: "enable_always_rule", Name: "route_rule", Options: map[string]string{"rule": "route_rule", "rule_mode": "always"}}},
				},
			},
		},
	}

	for _, rl := range rules {
		for _, dialect := range nonPGDialects {
			t.Run(rl.name+"_dialect_"+string(dialect), func(t *testing.T) {
				t.Parallel()
				stmt := rl.stmt
				stmt.Dialect = dialect
				if rl.r.AppliesTo(stmt) {
					t.Fatalf("expected AppliesTo() == false for dialect %s", dialect)
				}
				findings, err := rl.r.Evaluate(context.Background(), stmt)
				if err != nil {
					t.Fatalf("evaluate: %v", err)
				}
				if len(findings) != 0 {
					t.Fatalf("expected 0 findings for dialect %s, got %d", dialect, len(findings))
				}
			})
		}
	}
}

func TestTriggerRuleModeRulesNoLeak(t *testing.T) {
	t.Parallel()
	forbiddenKeys := []string{
		"trigger_function", "trigger_body", "trigger_when", "trigger_events", "trigger_columns",
		"rule_query", "rule_body", "rule_commands", "rule_where",
	}

	rules := []struct {
		name string
		r    rule.StatementRule
		stmt spec.Statement
	}{
		{
			name: "enable_replica_trigger",
			r:    mustNewEnableReplicaTriggerNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
			stmt: spec.Statement{
				Kind:    spec.KindDDL,
				Dialect: spec.DialectPostgreSQL,
				DDL: &spec.DDL{
					Operation: spec.DDLOperationAlterTable,
					Table:     &spec.Table{Name: "users"},
					Alter:     []spec.Alter{{Action: "enable_replica_trigger", Name: "sync_trigger", Options: map[string]string{"trigger": "sync_trigger", "trigger_mode": "replica"}}},
				},
			},
		},
		{
			name: "enable_always_trigger",
			r:    mustNewEnableAlwaysTriggerNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
			stmt: spec.Statement{
				Kind:    spec.KindDDL,
				Dialect: spec.DialectPostgreSQL,
				DDL: &spec.DDL{
					Operation: spec.DDLOperationAlterTable,
					Table:     &spec.Table{Name: "users"},
					Alter:     []spec.Alter{{Action: "enable_always_trigger", Name: "audit_trigger", Options: map[string]string{"trigger": "audit_trigger", "trigger_mode": "always"}}},
				},
			},
		},
		{
			name: "enable_rule",
			r:    mustNewEnableRuleNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
			stmt: spec.Statement{
				Kind:    spec.KindDDL,
				Dialect: spec.DialectPostgreSQL,
				DDL: &spec.DDL{
					Operation: spec.DDLOperationAlterTable,
					Table:     &spec.Table{Name: "users"},
					Alter:     []spec.Alter{{Action: "enable_rule", Name: "route_rule", Options: map[string]string{"rule": "route_rule"}}},
				},
			},
		},
		{
			name: "disable_rule",
			r:    mustNewDisableRuleWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning}),
			stmt: spec.Statement{
				Kind:    spec.KindDDL,
				Dialect: spec.DialectPostgreSQL,
				DDL: &spec.DDL{
					Operation: spec.DDLOperationAlterTable,
					Table:     &spec.Table{Name: "users"},
					Alter:     []spec.Alter{{Action: "disable_rule", Name: "route_rule", Options: map[string]string{"rule": "route_rule"}}},
				},
			},
		},
		{
			name: "enable_replica_rule",
			r:    mustNewEnableReplicaRuleNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
			stmt: spec.Statement{
				Kind:    spec.KindDDL,
				Dialect: spec.DialectPostgreSQL,
				DDL: &spec.DDL{
					Operation: spec.DDLOperationAlterTable,
					Table:     &spec.Table{Name: "users"},
					Alter:     []spec.Alter{{Action: "enable_replica_rule", Name: "route_rule", Options: map[string]string{"rule": "route_rule", "rule_mode": "replica"}}},
				},
			},
		},
		{
			name: "enable_always_rule",
			r:    mustNewEnableAlwaysRuleNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice}),
			stmt: spec.Statement{
				Kind:    spec.KindDDL,
				Dialect: spec.DialectPostgreSQL,
				DDL: &spec.DDL{
					Operation: spec.DDLOperationAlterTable,
					Table:     &spec.Table{Name: "users"},
					Alter:     []spec.Alter{{Action: "enable_always_rule", Name: "route_rule", Options: map[string]string{"rule": "route_rule", "rule_mode": "always"}}},
				},
			},
		},
	}

	for _, rl := range rules {
		t.Run(rl.name, func(t *testing.T) {
			t.Parallel()
			findings, err := rl.r.Evaluate(context.Background(), rl.stmt)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			if len(findings) != 1 {
				t.Fatalf("expected 1 finding, got %d", len(findings))
			}
			for _, key := range forbiddenKeys {
				if _, ok := findings[0].Metadata[key]; ok {
					t.Fatalf("forbidden key %q present in finding metadata", key)
				}
			}
		})
	}
}
