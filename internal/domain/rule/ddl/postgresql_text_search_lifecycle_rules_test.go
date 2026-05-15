package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestTextSearchLifecycleRules(t *testing.T) {
	t.Parallel()
	cases := []struct {
		ruleID    string
		operation spec.DDLOperation
		options   map[string]string
	}{
		{ruleIDPGCreateTextSearchConfigurationNotice, spec.DDLOperationCreateTextSearchConfiguration, nil},
		{ruleIDPGAlterTextSearchConfigurationNotice, spec.DDLOperationAlterTextSearchConfiguration, map[string]string{"action": "rename"}},
		{ruleIDPGDropTextSearchConfigurationWarn, spec.DDLOperationDropTextSearchConfiguration, nil},
		{ruleIDPGCreateTextSearchDictionaryNotice, spec.DDLOperationCreateTextSearchDictionary, nil},
		{ruleIDPGAlterTextSearchDictionaryNotice, spec.DDLOperationAlterTextSearchDictionary, map[string]string{"action": "rename"}},
		{ruleIDPGDropTextSearchDictionaryWarn, spec.DDLOperationDropTextSearchDictionary, nil},
		{ruleIDPGCreateTextSearchParserNotice, spec.DDLOperationCreateTextSearchParser, nil},
		{ruleIDPGAlterTextSearchParserNotice, spec.DDLOperationAlterTextSearchParser, map[string]string{"action": "rename"}},
		{ruleIDPGDropTextSearchParserWarn, spec.DDLOperationDropTextSearchParser, nil},
		{ruleIDPGCreateTextSearchTemplateNotice, spec.DDLOperationCreateTextSearchTemplate, nil},
		{ruleIDPGAlterTextSearchTemplateNotice, spec.DDLOperationAlterTextSearchTemplate, map[string]string{"action": "rename"}},
		{ruleIDPGDropTextSearchTemplateWarn, spec.DDLOperationDropTextSearchTemplate, nil},
	}
	cfg := policy.RulePolicy{}
	constructors := map[string]func(policy.RulePolicy) (rule.StatementRule, error){
		ruleIDPGCreateTextSearchConfigurationNotice: newCreateTextSearchConfigurationNoticeRule,
		ruleIDPGAlterTextSearchConfigurationNotice:  newAlterTextSearchConfigurationNoticeRule,
		ruleIDPGDropTextSearchConfigurationWarn:     newDropTextSearchConfigurationWarnRule,
		ruleIDPGCreateTextSearchDictionaryNotice:    newCreateTextSearchDictionaryNoticeRule,
		ruleIDPGAlterTextSearchDictionaryNotice:     newAlterTextSearchDictionaryNoticeRule,
		ruleIDPGDropTextSearchDictionaryWarn:        newDropTextSearchDictionaryWarnRule,
		ruleIDPGCreateTextSearchParserNotice:        newCreateTextSearchParserNoticeRule,
		ruleIDPGAlterTextSearchParserNotice:         newAlterTextSearchParserNoticeRule,
		ruleIDPGDropTextSearchParserWarn:            newDropTextSearchParserWarnRule,
		ruleIDPGCreateTextSearchTemplateNotice:      newCreateTextSearchTemplateNoticeRule,
		ruleIDPGAlterTextSearchTemplateNotice:       newAlterTextSearchTemplateNoticeRule,
		ruleIDPGDropTextSearchTemplateWarn:          newDropTextSearchTemplateWarnRule,
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
			stmt := spec.Statement{Dialect: spec.DialectPostgreSQL, Kind: spec.KindDDL, DDL: &spec.DDL{Operation: tc.operation, ObjectName: "test_obj", ObjectType: "text_search_configuration", Options: tc.options}}
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

func TestTextSearchLifecycleWrongDialect(t *testing.T) {
	t.Parallel()
	r, _ := newCreateTextSearchConfigurationNoticeRule(policy.RulePolicy{})
	stmt := spec.Statement{Dialect: spec.DialectMySQL, Kind: spec.KindDDL, DDL: &spec.DDL{Operation: spec.DDLOperationCreateTextSearchConfiguration}}
	findings, _ := r.Evaluate(nil, stmt)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for MySQL, got %d", len(findings))
	}
}

func TestTextSearchLifecycleWrongOperation(t *testing.T) {
	t.Parallel()
	r, _ := newCreateTextSearchConfigurationNoticeRule(policy.RulePolicy{})
	stmt := spec.Statement{Dialect: spec.DialectPostgreSQL, Kind: spec.KindDDL, DDL: &spec.DDL{Operation: spec.DDLOperationCreateTable}}
	findings, _ := r.Evaluate(nil, stmt)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for wrong operation, got %d", len(findings))
	}
}
