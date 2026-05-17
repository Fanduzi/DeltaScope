package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestBoundaryLifecycleRules(t *testing.T) {
	t.Parallel()
	cases := []struct {
		ruleID    string
		operation spec.DDLOperation
		options   map[string]string
	}{
		{ruleIDPGCreateTransformNotice, spec.DDLOperationCreateTransform, nil},
		{ruleIDPGCreateAccessMethodNotice, spec.DDLOperationCreateAccessMethod, nil},
		{ruleIDPGDropTransformWarn, spec.DDLOperationDropTransform, nil},
		{ruleIDPGDropAccessMethodWarn, spec.DDLOperationDropAccessMethod, nil},
		{ruleIDPGAlterLargeObjectOwnerNotice, spec.DDLOperationAlterLargeObject, map[string]string{"action": "set_owner"}},
	}
	cfg := policy.RulePolicy{}
	constructors := map[string]func(policy.RulePolicy) (rule.StatementRule, error){
		ruleIDPGCreateTransformNotice:       newCreateTransformNoticeRule,
		ruleIDPGCreateAccessMethodNotice:    newCreateAccessMethodNoticeRule,
		ruleIDPGDropTransformWarn:           newDropTransformWarnRule,
		ruleIDPGDropAccessMethodWarn:        newDropAccessMethodWarnRule,
		ruleIDPGAlterLargeObjectOwnerNotice: newAlterLargeObjectOwnerNoticeRule,
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
			stmt := spec.Statement{Dialect: spec.DialectPostgreSQL, Kind: spec.KindDDL, DDL: &spec.DDL{Operation: tc.operation, ObjectName: "test_obj", ObjectType: "boundary", Options: tc.options}}
			findings, err := r.Evaluate(context.Background(), stmt)
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

func TestBoundaryLifecycleWrongDialect(t *testing.T) {
	t.Parallel()
	r, _ := newDropTransformWarnRule(policy.RulePolicy{})
	stmt := spec.Statement{Dialect: spec.DialectMySQL, Kind: spec.KindDDL, DDL: &spec.DDL{Operation: spec.DDLOperationDropTransform}}
	findings, _ := r.Evaluate(nil, stmt)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for MySQL, got %d", len(findings))
	}
}

func TestBoundaryLifecycleWrongOperation(t *testing.T) {
	t.Parallel()
	r, _ := newDropTransformWarnRule(policy.RulePolicy{})
	stmt := spec.Statement{Dialect: spec.DialectPostgreSQL, Kind: spec.KindDDL, DDL: &spec.DDL{Operation: spec.DDLOperationCreateTable}}
	findings, _ := r.Evaluate(nil, stmt)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for wrong operation, got %d", len(findings))
	}
}
