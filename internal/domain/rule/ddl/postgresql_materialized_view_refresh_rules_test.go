package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func refreshMatViewStatement(opts map[string]string) spec.Statement {
	return spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationRefreshMaterializedView,
			ObjectName: "mv_stats",
			ObjectType: "materialized_view",
			Options:    opts,
		},
	}
}

func TestPGRefreshMaterializedViewConcurrentlyWarn_NonConcurrentFiresWarning(t *testing.T) {
	r, err := newRefreshMaterializedViewConcurrentlyWarnRule(policy.RulePolicy{Enabled: true})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	findings, err := r.Evaluate(context.Background(), refreshMatViewStatement(map[string]string{
		"concurrently": "false",
		"with_no_data": "false",
	}))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Level != rule.LevelWarning {
		t.Errorf("expected warning, got %s", findings[0].Level)
	}
}

func TestPGRefreshMaterializedViewConcurrentlyWarn_ConcurrentNoFinding(t *testing.T) {
	r, err := newRefreshMaterializedViewConcurrentlyWarnRule(policy.RulePolicy{Enabled: true})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	findings, err := r.Evaluate(context.Background(), refreshMatViewStatement(map[string]string{
		"concurrently": "true",
		"with_no_data": "false",
	}))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings for concurrent refresh, got %d", len(findings))
	}
}

func TestPGRefreshMaterializedViewConcurrentlyWarn_MissingOptionsFiresWarning(t *testing.T) {
	r, err := newRefreshMaterializedViewConcurrentlyWarnRule(policy.RulePolicy{Enabled: true})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	findings, err := r.Evaluate(context.Background(), refreshMatViewStatement(map[string]string{}))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding when concurrently absent, got %d", len(findings))
	}
}

func TestPGRefreshMaterializedViewNoDataNotice_WithNoDataFiresNotice(t *testing.T) {
	r, err := newRefreshMaterializedViewNoDataNoticeRule(policy.RulePolicy{Enabled: true})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	findings, err := r.Evaluate(context.Background(), refreshMatViewStatement(map[string]string{
		"concurrently": "false",
		"with_no_data": "true",
	}))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Level != rule.LevelNotice {
		t.Errorf("expected notice, got %s", findings[0].Level)
	}
}

func TestPGRefreshMaterializedViewNoDataNotice_WithDataNoFinding(t *testing.T) {
	r, err := newRefreshMaterializedViewNoDataNoticeRule(policy.RulePolicy{Enabled: true})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}

	findings, err := r.Evaluate(context.Background(), refreshMatViewStatement(map[string]string{
		"concurrently": "false",
		"with_no_data": "false",
	}))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings for with_data, got %d", len(findings))
	}
}

func TestPGRefreshMaterializedViewRules_WrongDialectNotApplies(t *testing.T) {
	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectMySQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationRefreshMaterializedView,
			ObjectName: "mv_stats",
			ObjectType: "materialized_view",
			Options:    map[string]string{"concurrently": "false", "with_no_data": "false"},
		},
	}

	concurrentRule, _ := newRefreshMaterializedViewConcurrentlyWarnRule(policy.RulePolicy{Enabled: true})
	noDataRule, _ := newRefreshMaterializedViewNoDataNoticeRule(policy.RulePolicy{Enabled: true})

	if concurrentRule.AppliesTo(stmt) {
		t.Error("concurrently rule should not apply to MySQL")
	}
	if noDataRule.AppliesTo(stmt) {
		t.Error("no-data rule should not apply to MySQL")
	}
}

func TestPGRefreshMaterializedViewRules_WrongOperationNotApplies(t *testing.T) {
	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationDropMaterializedView,
			ObjectName: "mv_stats",
			ObjectType: "materialized_view",
			Options:    map[string]string{"concurrently": "false", "with_no_data": "false"},
		},
	}

	concurrentRule, _ := newRefreshMaterializedViewConcurrentlyWarnRule(policy.RulePolicy{Enabled: true})
	noDataRule, _ := newRefreshMaterializedViewNoDataNoticeRule(policy.RulePolicy{Enabled: true})

	if concurrentRule.AppliesTo(stmt) {
		t.Error("concurrently rule should not apply to drop_materialized_view")
	}
	if noDataRule.AppliesTo(stmt) {
		t.Error("no-data rule should not apply to drop_materialized_view")
	}
}

func TestPGRefreshMaterializedViewRules_MetadataIncludesAllFields(t *testing.T) {
	concurrentRule, _ := newRefreshMaterializedViewConcurrentlyWarnRule(policy.RulePolicy{Enabled: true})

	findings, err := concurrentRule.Evaluate(context.Background(), refreshMatViewStatement(map[string]string{
		"concurrently": "false",
		"with_no_data": "true",
	}))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}

	m := findings[0].Metadata
	for _, key := range []string{"operation", "object_type", "object", "concurrently", "with_no_data"} {
		if _, ok := m[key]; !ok {
			t.Errorf("metadata missing key %q", key)
		}
	}
	if m["operation"] != "refresh_materialized_view" {
		t.Errorf("expected operation=refresh_materialized_view, got %v", m["operation"])
	}
	if m["object_type"] != "materialized_view" {
		t.Errorf("expected object_type=materialized_view, got %v", m["object_type"])
	}
	if m["object"] != "mv_stats" {
		t.Errorf("expected object=mv_stats, got %v", m["object"])
	}
}

func TestPGRefreshMaterializedViewRules_BothRulesRegistered(t *testing.T) {
	registry := rule.NewRegistry()
	cfg := policy.Default()
	if err := Register(registry, cfg); err != nil {
		t.Fatalf("register: %v", err)
	}

	stmt := refreshMatViewStatement(map[string]string{
		"concurrently": "false",
		"with_no_data": "true",
	})
	findings, err := registry.EvaluateStatement(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}

	foundConcurrent := false
	foundNoData := false
	for _, f := range findings {
		if f.RuleID == ruleIDPGRefreshMaterializedViewConcurrentlyWarn {
			foundConcurrent = true
		}
		if f.RuleID == ruleIDPGRefreshMaterializedViewNoDataNotice {
			foundNoData = true
		}
	}
	if !foundConcurrent {
		t.Error("concurrently warn rule not registered or did not fire")
	}
	if !foundNoData {
		t.Error("no-data notice rule not registered or did not fire")
	}
}

func TestPGRefreshMaterializedViewRules_PostgreSQLDefaultsContainBoth(t *testing.T) {
	cfg := policy.Default()

	concurrentEntry, ok := cfg.Rules[ruleIDPGRefreshMaterializedViewConcurrentlyWarn]
	if !ok {
		t.Fatal("defaults missing concurrently warn rule")
	}
	if !concurrentEntry.Enabled {
		t.Error("expected concurrently warn rule enabled")
	}
	if concurrentEntry.Level != rule.LevelWarning {
		t.Errorf("expected concurrently warn level=warning, got %s", concurrentEntry.Level)
	}

	noDataEntry, ok := cfg.Rules[ruleIDPGRefreshMaterializedViewNoDataNotice]
	if !ok {
		t.Fatal("defaults missing no-data notice rule")
	}
	if !noDataEntry.Enabled {
		t.Error("expected no-data notice rule enabled")
	}
	if noDataEntry.Level != rule.LevelNotice {
		t.Errorf("expected no-data notice level=notice, got %s", noDataEntry.Level)
	}
}
