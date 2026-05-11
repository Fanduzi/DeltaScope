package ddl

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestCreateTriggerNoticeRuleApplies(t *testing.T) {
	t.Parallel()
	r, err := newCreateTriggerNoticeRule(policy.RulePolicy{Enabled: true})
	if err != nil {
		t.Fatalf("create rule: %v", err)
	}
	if r.ID() != "ddl.pg.create_trigger.notice" {
		t.Fatalf("expected rule_id ddl.pg.create_trigger.notice, got %q", r.ID())
	}
	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationCreateTrigger,
			ObjectName: "trg_audit",
			ObjectType: "trigger",
		},
	}
	if !r.AppliesTo(stmt) {
		t.Fatal("expected rule to apply to create_trigger statement")
	}
	findings, err := r.Evaluate(t.Context(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestCreateTriggerNoticeRuleWrongOperationNoMatch(t *testing.T) {
	t.Parallel()
	r, err := newCreateTriggerNoticeRule(policy.RulePolicy{Enabled: true})
	if err != nil {
		t.Fatalf("create rule: %v", err)
	}
	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationDropTrigger,
			ObjectName: "trg_audit",
			ObjectType: "trigger",
		},
	}
	if r.AppliesTo(stmt) {
		t.Fatal("expected rule NOT to apply to drop_trigger statement")
	}
}

func TestCreateTriggerNoticeRuleMySQLDialectNoMatch(t *testing.T) {
	t.Parallel()
	r, err := newCreateTriggerNoticeRule(policy.RulePolicy{Enabled: true})
	if err != nil {
		t.Fatalf("create rule: %v", err)
	}
	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectMySQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationCreateTrigger,
			ObjectName: "trg_audit",
			ObjectType: "trigger",
		},
	}
	if r.AppliesTo(stmt) {
		t.Fatal("expected rule NOT to apply to MySQL dialect")
	}
}

func TestCreateConstraintTriggerWarnRuleApplies(t *testing.T) {
	t.Parallel()
	r, err := newCreateConstraintTriggerWarnRule(policy.RulePolicy{Enabled: true})
	if err != nil {
		t.Fatalf("create rule: %v", err)
	}
	if r.ID() != "ddl.pg.create_constraint_trigger.warn" {
		t.Fatalf("expected rule_id ddl.pg.create_constraint_trigger.warn, got %q", r.ID())
	}
	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationCreateTrigger,
			ObjectName: "trg_fk",
			ObjectType: "trigger",
			Options:    map[string]string{"constraint": "true"},
		},
	}
	if !r.AppliesTo(stmt) {
		t.Fatal("expected rule to apply to constraint trigger statement")
	}
	findings, err := r.Evaluate(t.Context(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestCreateConstraintTriggerWarnRuleNoMatchWithoutConstraintOption(t *testing.T) {
	t.Parallel()
	r, err := newCreateConstraintTriggerWarnRule(policy.RulePolicy{Enabled: true})
	if err != nil {
		t.Fatalf("create rule: %v", err)
	}
	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationCreateTrigger,
			ObjectName: "trg_audit",
			ObjectType: "trigger",
		},
	}
	if !r.AppliesTo(stmt) {
		t.Fatal("AppliesTo should return true (option check is in Evaluate)")
	}
	findings, err := r.Evaluate(t.Context(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings for non-constraint trigger, got %d", len(findings))
	}
}

func TestDropTriggerAdvisoryRuleApplies(t *testing.T) {
	t.Parallel()
	r, err := newDropTriggerAdvisoryRule(policy.RulePolicy{Enabled: true})
	if err != nil {
		t.Fatalf("create rule: %v", err)
	}
	if r.ID() != "ddl.pg.drop_trigger.advisory" {
		t.Fatalf("expected rule_id ddl.pg.drop_trigger.advisory, got %q", r.ID())
	}
	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationDropTrigger,
			ObjectName: "trg_audit",
			ObjectType: "trigger",
		},
	}
	if !r.AppliesTo(stmt) {
		t.Fatal("expected rule to apply to drop_trigger statement")
	}
	findings, err := r.Evaluate(t.Context(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestDropTriggerAdvisoryRuleCreateOperationNoMatch(t *testing.T) {
	t.Parallel()
	r, err := newDropTriggerAdvisoryRule(policy.RulePolicy{Enabled: true})
	if err != nil {
		t.Fatalf("create rule: %v", err)
	}
	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationCreateTrigger,
			ObjectName: "trg_audit",
			ObjectType: "trigger",
		},
	}
	if r.AppliesTo(stmt) {
		t.Fatal("expected rule NOT to apply to create_trigger statement")
	}
}
