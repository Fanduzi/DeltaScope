// Package ddl verifies DDL denylist governance rules.
// input: parser-neutral multi-target drop/rename statement specs and schema/table denylist policies
// output: coverage for per-target denylist evaluation, qualified-name resolution, deduplication, and ordering
// pos: DDL denylist rule test coverage for multi-target completeness
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func newTestDenylistRule(t *testing.T, params map[string]any) rule.StatementRule {
	t.Helper()
	statementRule, err := newTableDenylistRule(ruleIDTableDenylistForbid, rule.LevelBlocker, policy.RulePolicy{
		Enabled: true,
		Level:   rule.LevelBlocker,
		Params:  params,
	})
	if err != nil {
		t.Fatalf("new table denylist rule: %v", err)
	}
	return statementRule
}

func denylistStatement(operation spec.DDLOperation, schema string, targets ...spec.Table) spec.Statement {
	statement := spec.Statement{
		Kind: spec.KindDDL,
		DDL:  &spec.DDL{Operation: operation, Targets: targets},
	}
	if len(targets) > 0 {
		first := targets[0]
		statement.DDL.Table = &first
	}
	if schema != "" {
		statement.Metadata = &spec.Metadata{Schema: schema}
	}
	return statement
}

func denylistTables(findings []rule.Finding) []string {
	tables := make([]string, 0, len(findings))
	for _, finding := range findings {
		table, _ := finding.Metadata["table"].(string)
		tables = append(tables, table)
	}
	return tables
}

func TestTableDenylistRuleChecksEveryDropTarget(t *testing.T) {
	t.Parallel()
	statementRule := newTestDenylistRule(t, map[string]any{"tables": []string{"sensitive"}})

	cases := []struct {
		name    string
		targets []spec.Table
	}{
		{name: "first", targets: []spec.Table{{Name: "sensitive"}, {Name: "harmless"}}},
		{name: "middle", targets: []spec.Table{{Name: "harmless"}, {Name: "sensitive"}, {Name: "other"}}},
		{name: "last", targets: []spec.Table{{Name: "harmless"}, {Name: "sensitive"}}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			findings, err := statementRule.Evaluate(context.Background(), denylistStatement(spec.DDLOperationDropTable, "", tc.targets...))
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			if len(findings) != 1 {
				t.Fatalf("expected 1 denylist finding, got %#v", findings)
			}
			if findings[0].Level != rule.LevelBlocker {
				t.Fatalf("expected blocker level, got %q", findings[0].Level)
			}
			if findings[0].Metadata["table"] != "sensitive" {
				t.Fatalf("expected metadata.table sensitive, got %#v", findings[0].Metadata)
			}
		})
	}
}

func TestTableDenylistRuleChecksRenameSourceAndDestination(t *testing.T) {
	t.Parallel()
	statementRule := newTestDenylistRule(t, map[string]any{"tables": []string{"sensitive"}})

	source := denylistStatement(spec.DDLOperationRenameTable, "",
		spec.Table{Name: "harmless"}, spec.Table{Name: "harmless_old"},
		spec.Table{Name: "sensitive"}, spec.Table{Name: "sensitive_old"})
	findings, err := statementRule.Evaluate(context.Background(), source)
	if err != nil {
		t.Fatalf("evaluate source: %v", err)
	}
	if len(findings) != 1 || findings[0].Metadata["table"] != "sensitive" {
		t.Fatalf("expected 1 finding for rename source, got %#v", findings)
	}

	destination := denylistStatement(spec.DDLOperationRenameTable, "",
		spec.Table{Name: "harmless"}, spec.Table{Name: "harmless_old"},
		spec.Table{Name: "other"}, spec.Table{Name: "sensitive"})
	findings, err = statementRule.Evaluate(context.Background(), destination)
	if err != nil {
		t.Fatalf("evaluate destination: %v", err)
	}
	if len(findings) != 1 || findings[0].Metadata["table"] != "sensitive" {
		t.Fatalf("expected 1 finding for rename destination, got %#v", findings)
	}

	allowed := denylistStatement(spec.DDLOperationRenameTable, "",
		spec.Table{Name: "harmless"}, spec.Table{Name: "harmless_old"})
	findings, err = statementRule.Evaluate(context.Background(), allowed)
	if err != nil {
		t.Fatalf("evaluate allowed: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings for allowed rename, got %#v", findings)
	}
}

func TestTableDenylistRuleResolvesQualifiedTargetSchema(t *testing.T) {
	t.Parallel()
	statementRule := newTestDenylistRule(t, map[string]any{"qualified_tables": []string{"prod.sensitive"}})

	qualified := denylistStatement(spec.DDLOperationDropTable, "", spec.Table{Schema: "prod", Name: "sensitive"})
	findings, err := statementRule.Evaluate(context.Background(), qualified)
	if err != nil {
		t.Fatalf("evaluate qualified: %v", err)
	}
	if len(findings) != 1 || findings[0].Metadata["schema"] != "prod" || findings[0].Metadata["table"] != "sensitive" {
		t.Fatalf("expected qualified_tables match via target schema, got %#v", findings)
	}

	// An unqualified target with no request schema is an unknown-schema boundary:
	// qualified selectors cannot match it.
	unqualified := denylistStatement(spec.DDLOperationDropTable, "", spec.Table{Name: "sensitive"})
	findings, err = statementRule.Evaluate(context.Background(), unqualified)
	if err != nil {
		t.Fatalf("evaluate unqualified: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected qualified selector to skip unknown-schema target, got %#v", findings)
	}

	// A target qualifier wins over the request schema when they disagree.
	conflict := denylistStatement(spec.DDLOperationDropTable, "app", spec.Table{Schema: "prod", Name: "sensitive"})
	findings, err = statementRule.Evaluate(context.Background(), conflict)
	if err != nil {
		t.Fatalf("evaluate conflicting schema: %v", err)
	}
	if len(findings) != 1 || findings[0].Metadata["schema"] != "prod" {
		t.Fatalf("expected target schema to win over request schema, got %#v", findings)
	}
}

func TestTableDenylistRuleMatchesSchemaSelectorPerTarget(t *testing.T) {
	t.Parallel()
	statementRule := newTestDenylistRule(t, map[string]any{"schemas": []string{"prod"}})

	findings, err := statementRule.Evaluate(context.Background(), denylistStatement(spec.DDLOperationDropTable, "",
		spec.Table{Schema: "app", Name: "users"}, spec.Table{Schema: "prod", Name: "orders"}))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 || findings[0].Metadata["schema"] != "prod" || findings[0].Metadata["table"] != "orders" {
		t.Fatalf("expected schema selector to match only the prod target, got %#v", findings)
	}
}

func TestTableDenylistRuleFallsBackToMetadataSchema(t *testing.T) {
	t.Parallel()
	statementRule := newTestDenylistRule(t, map[string]any{"qualified_tables": []string{"app.users"}})

	findings, err := statementRule.Evaluate(context.Background(), denylistStatement(spec.DDLOperationDropTable, "app",
		spec.Table{Name: "users"}))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 || findings[0].Metadata["schema"] != "app" {
		t.Fatalf("expected metadata schema fallback for unqualified target, got %#v", findings)
	}
}

func TestTableDenylistRuleDeduplicatesProtectedObjects(t *testing.T) {
	t.Parallel()
	statementRule := newTestDenylistRule(t, map[string]any{
		"tables":           []string{"sensitive"},
		"schemas":          []string{"app"},
		"qualified_tables": []string{"app.sensitive"},
	})

	// One protected object matching every selector still yields one finding.
	single := denylistStatement(spec.DDLOperationDropTable, "", spec.Table{Schema: "app", Name: "sensitive"})
	findings, err := statementRule.Evaluate(context.Background(), single)
	if err != nil {
		t.Fatalf("evaluate multi-selector: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 deduplicated finding, got %#v", findings)
	}

	// Repeated references to the same resolved object deduplicate too.
	repeated := denylistStatement(spec.DDLOperationDropTable, "app",
		spec.Table{Schema: "app", Name: "sensitive"}, spec.Table{Name: "sensitive"})
	findings, err = statementRule.Evaluate(context.Background(), repeated)
	if err != nil {
		t.Fatalf("evaluate repeated: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected repeated object to deduplicate, got %#v", findings)
	}
}

func TestTableDenylistRuleDoesNotMergeDottedQualifiedIdentities(t *testing.T) {
	t.Parallel()
	statementRule := newTestDenylistRule(t, map[string]any{"tables": []string{"c", "b.c"}})

	// `a.b`.`c` and `a`.`b.c` resolve to different objects; a flat
	// "a.b.c" string key would wrongly deduplicate them into one finding.
	findings, err := statementRule.Evaluate(context.Background(), denylistStatement(spec.DDLOperationDropTable, "",
		spec.Table{Schema: "a.b", Name: "c"}, spec.Table{Schema: "a", Name: "b.c"}))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings for distinct dotted identities, got %#v", findings)
	}
	if findings[0].Metadata["schema"] != "a.b" || findings[0].Metadata["table"] != "c" {
		t.Fatalf("expected first finding a.b/c, got %#v", findings[0].Metadata)
	}
	if findings[1].Metadata["schema"] != "a" || findings[1].Metadata["table"] != "b.c" {
		t.Fatalf("expected second finding a/b.c, got %#v", findings[1].Metadata)
	}
}

func TestTableDenylistRulePreservesSourceOrderForMultipleProtectedObjects(t *testing.T) {
	t.Parallel()
	statementRule := newTestDenylistRule(t, map[string]any{"tables": []string{"first", "second"}})

	findings, err := statementRule.Evaluate(context.Background(), denylistStatement(spec.DDLOperationDropTable, "",
		spec.Table{Name: "second"}, spec.Table{Name: "harmless"}, spec.Table{Name: "first"}))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	got := denylistTables(findings)
	if len(got) != 2 || got[0] != "second" || got[1] != "first" {
		t.Fatalf("expected findings in source order [second first], got %#v", got)
	}
}

func TestTableDenylistRuleSkipsDDLWithoutTableTargets(t *testing.T) {
	t.Parallel()
	statementRule := newTestDenylistRule(t, map[string]any{"tables": []string{"sensitive"}})

	statement := spec.Statement{
		Kind: spec.KindDDL,
		DDL:  &spec.DDL{Operation: spec.DDLOperationDropSchema, ObjectName: "sensitive"},
	}
	if statementRule.AppliesTo(statement) {
		t.Fatal("expected schema-only DDL to be inapplicable")
	}
	findings, err := statementRule.Evaluate(context.Background(), statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %#v", findings)
	}
}
