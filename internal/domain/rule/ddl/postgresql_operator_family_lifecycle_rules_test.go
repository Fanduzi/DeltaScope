package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestOperatorFamilyLifecycleRules(t *testing.T) {
	t.Parallel()
	cases := []struct {
		ruleID    string
		operation spec.DDLOperation
		options   map[string]string
	}{
		{ruleIDPGCreateOperatorFamilyNotice, spec.DDLOperationCreateOperatorFamily, map[string]string{"access_method": "btree"}},
		{ruleIDPGAlterOperatorFamilyNotice, spec.DDLOperationAlterOperatorFamily, map[string]string{"action": "rename"}},
		{ruleIDPGDropOperatorFamilyWarn, spec.DDLOperationDropOperatorFamily, nil},
		{ruleIDPGCreateOperatorClassNotice, spec.DDLOperationCreateOperatorClass, map[string]string{"access_method": "btree"}},
		{ruleIDPGAlterOperatorClassNotice, spec.DDLOperationAlterOperatorClass, map[string]string{"action": "rename"}},
		{ruleIDPGDropOperatorClassWarn, spec.DDLOperationDropOperatorClass, nil},
	}
	cfg := policy.RulePolicy{}
	constructors := map[string]func(policy.RulePolicy) (rule.StatementRule, error){
		ruleIDPGCreateOperatorFamilyNotice: newCreateOperatorFamilyNoticeRule,
		ruleIDPGAlterOperatorFamilyNotice:  newAlterOperatorFamilyNoticeRule,
		ruleIDPGDropOperatorFamilyWarn:     newDropOperatorFamilyWarnRule,
		ruleIDPGCreateOperatorClassNotice:  newCreateOperatorClassNoticeRule,
		ruleIDPGAlterOperatorClassNotice:   newAlterOperatorClassNoticeRule,
		ruleIDPGDropOperatorClassWarn:      newDropOperatorClassWarnRule,
	}
	for _, tc := range cases {
		t.Run(tc.ruleID, func(t *testing.T) {
			t.Parallel()
			fn, ok := constructors[tc.ruleID]
			if !ok {
				t.Fatalf("no constructor for %s", tc.ruleID)
			}
			r, err := fn(cfg)
			if err != nil {
				t.Fatalf("constructor error: %v", err)
			}
			if r.ID() != tc.ruleID {
				t.Errorf("expected ID %s, got %s", tc.ruleID, r.ID())
			}
			stmt := spec.Statement{Dialect: spec.DialectPostgreSQL, Kind: spec.KindDDL, DDL: &spec.DDL{Operation: tc.operation, ObjectName: "test_obj", ObjectType: "operator_family", Options: tc.options}}
			findings, err := r.Evaluate(nil, stmt)
			if err != nil {
				t.Fatalf("evaluate error: %v", err)
			}
			if len(findings) != 1 {
				t.Fatalf("expected 1 finding, got %d", len(findings))
			}
			if findings[0].RuleID != tc.ruleID {
				t.Errorf("expected finding RuleID %s, got %s", tc.ruleID, findings[0].RuleID)
			}
		})
	}
}

func TestOperatorFamilyLifecycleWrongDialect(t *testing.T) {
	t.Parallel()
	r, _ := newCreateOperatorFamilyNoticeRule(policy.RulePolicy{})
	stmt := spec.Statement{Dialect: spec.DialectMySQL, Kind: spec.KindDDL, DDL: &spec.DDL{Operation: spec.DDLOperationCreateOperatorFamily}}
	findings, _ := r.Evaluate(nil, stmt)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for MySQL, got %d", len(findings))
	}
}

func TestOperatorFamilyLifecycleWrongOperation(t *testing.T) {
	t.Parallel()
	r, _ := newCreateOperatorFamilyNoticeRule(policy.RulePolicy{})
	stmt := spec.Statement{Dialect: spec.DialectPostgreSQL, Kind: spec.KindDDL, DDL: &spec.DDL{Operation: spec.DDLOperationCreateTable}}
	findings, _ := r.Evaluate(nil, stmt)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for wrong operation, got %d", len(findings))
	}
}

func TestOperatorFamilyLifecycleRegistryInclusion(t *testing.T) {
	t.Parallel()
	registry := rule.NewRegistry()
	cfg := policy.Default()
	if err := Register(registry, cfg); err != nil {
		t.Fatalf("register: %v", err)
	}

	cases := []struct {
		ruleID string
		stmt   spec.Statement
	}{
		{ruleIDPGCreateOperatorFamilyNotice, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationCreateOperatorFamily, ObjectName: "of1", ObjectType: "operator_family", Options: map[string]string{"access_method": "btree"}},
		}},
		{ruleIDPGAlterOperatorFamilyNotice, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationAlterOperatorFamily, ObjectName: "of1", ObjectType: "operator_family", Options: map[string]string{"action": "rename", "new_name": "of2"}},
		}},
		{ruleIDPGDropOperatorFamilyWarn, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationDropOperatorFamily, ObjectName: "of1", ObjectType: "operator_family", Options: map[string]string{}},
		}},
		{ruleIDPGCreateOperatorClassNotice, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationCreateOperatorClass, ObjectName: "oc1", ObjectType: "operator_class", Options: map[string]string{"access_method": "btree"}},
		}},
		{ruleIDPGAlterOperatorClassNotice, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationAlterOperatorClass, ObjectName: "oc1", ObjectType: "operator_class", Options: map[string]string{"action": "rename", "new_name": "oc2"}},
		}},
		{ruleIDPGDropOperatorClassWarn, spec.Statement{
			Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
			DDL: &spec.DDL{Operation: spec.DDLOperationDropOperatorClass, ObjectName: "oc1", ObjectType: "operator_class", Options: map[string]string{}},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.ruleID, func(t *testing.T) {
			t.Parallel()
			findings, err := registry.EvaluateStatement(context.Background(), tc.stmt)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			found := false
			for _, f := range findings {
				if f.RuleID == tc.ruleID {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("rule %s not fired via registry", tc.ruleID)
			}
		})
	}
}
