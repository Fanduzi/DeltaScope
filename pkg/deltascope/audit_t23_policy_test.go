package deltascope

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// audit_t23_policy_test.go pins the accept_tidb_auto_random policy
// contract through the real public path: Request.ConfigPath ->
// viperconfig.LoadPolicy -> policy.Default() merge -> rule engine.
// No hand-built RuleConfig and no direct boolParam calls — every
// scenario loads an actual YAML file (or the shipped defaults).

const t23PolicySQL = "CREATE TABLE t (id BIGINT PRIMARY KEY AUTO_RANDOM);"

func t23PolicyProvider(dialect string) MetadataProvider {
	if dialect == string(DialectMySQL) {
		return &t23FactsProvider{facts: &InstanceFacts{Version: "8.0.36"}}
	}
	return &t23FactsProvider{facts: &InstanceFacts{
		Version:                  "8.0.11-TiDB-v8.5.0",
		TiDBEnableClusteredIndex: "on",
		TiDBAlterPrimaryKey:      true,
		TiDBAlterPrimaryKeyKnown: true,
	}}
}

func t23AuditWithConfig(t *testing.T, sql, dialect, yaml string) StatementResult {
	t.Helper()
	configPath := ""
	if yaml != "" {
		path := filepath.Join(t.TempDir(), "deltascope.yaml")
		if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
			t.Fatalf("write policy: %v", err)
		}
		configPath = path
	}
	result, err := Audit(context.Background(), Request{
		SQL:              sql,
		Dialect:          Dialect(dialect),
		Schema:           "app",
		ConfigPath:       configPath,
		MetadataProvider: t23PolicyProvider(dialect),
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("statements = %d", len(result.Statements))
	}
	return result.Statements[0]
}

func t23HasRule(stmt StatementResult, ruleID string) bool {
	for _, f := range stmt.Findings {
		if f.RuleID == ruleID {
			return true
		}
	}
	return false
}

const t23AutoIncrementRule = "ddl.table.primary_key.auto_increment.require"

// Scenario 1: shipped defaults accept AUTO_RANDOM as the surrogate-key
// allocation — the named policy finding must not fire.
func TestT23PolicyShippedDefaultsAcceptAutoRandom(t *testing.T) {
	t.Parallel()
	stmt := t23AuditWithConfig(t, t23PolicySQL, string(DialectTiDB), "")
	if t23HasRule(stmt, t23AutoIncrementRule) {
		t.Fatalf("shipped defaults must accept AUTO_RANDOM, got %+v", stmt.Findings)
	}
}

// Scenario 2: a user file that re-defines the rule replaces the whole
// RulePolicy — the user's params map does not inherit the shipped
// accept_tidb_auto_random, so the omitted key falls back to the
// rule-code default (strict literal) and the blocker fires. This pins
// the merge contract: "unset in the effective config -> strict".
func TestT23PolicyUserFileWithoutParamFallsBackToStrict(t *testing.T) {
	t.Parallel()
	stmt := t23AuditWithConfig(t, t23PolicySQL, string(DialectTiDB), `
rules:
  ddl.table.primary_key.auto_increment.require:
    enabled: true
    level: blocker
    params:
      required: true
`)
	if !t23HasRule(stmt, t23AutoIncrementRule) {
		t.Fatalf("user params map replaces shipped params; omitted key must fall back to strict, got %+v", stmt.Findings)
	}
}

// Scenario 3: explicit false preserves strict literal semantics —
// AUTO_RANDOM does not satisfy the literal auto_increment requirement.
func TestT23PolicyExplicitFalseKeepsStrictLiteral(t *testing.T) {
	t.Parallel()
	stmt := t23AuditWithConfig(t, t23PolicySQL, string(DialectTiDB), `
rules:
  ddl.table.primary_key.auto_increment.require:
    enabled: true
    level: blocker
    params:
      required: true
      accept_tidb_auto_random: false
`)
	if !t23HasRule(stmt, t23AutoIncrementRule) {
		t.Fatalf("explicit false must restore the strict literal blocker, got %+v", stmt.Findings)
	}
}

// Scenario 4: explicit true accepts AUTO_RANDOM.
func TestT23PolicyExplicitTrueAcceptsAutoRandom(t *testing.T) {
	t.Parallel()
	stmt := t23AuditWithConfig(t, t23PolicySQL, string(DialectTiDB), `
rules:
  ddl.table.primary_key.auto_increment.require:
    enabled: true
    level: blocker
    params:
      required: true
      accept_tidb_auto_random: true
`)
	if t23HasRule(stmt, t23AutoIncrementRule) {
		t.Fatalf("explicit true must accept AUTO_RANDOM, got %+v", stmt.Findings)
	}
}

// The param is TiDB-scoped: MySQL semantics are unchanged either way —
// a plain AUTO_INCREMENT primary key satisfies the requirement and a
// non-allocating key still blocks.
func TestT23PolicyMySQLSemanticsUnchanged(t *testing.T) {
	t.Parallel()
	for _, yaml := range []string{"", `
rules:
  ddl.table.primary_key.auto_increment.require:
    enabled: true
    level: blocker
    params:
      required: true
      accept_tidb_auto_random: false
`} {
		ok := t23AuditWithConfig(t,
			"CREATE TABLE t (id BIGINT AUTO_INCREMENT PRIMARY KEY);", string(DialectMySQL), yaml)
		if t23HasRule(ok, t23AutoIncrementRule) {
			t.Fatalf("mysql AUTO_INCREMENT must satisfy the rule, got %+v", ok.Findings)
		}
		strict := t23AuditWithConfig(t,
			"CREATE TABLE t (id BIGINT PRIMARY KEY);", string(DialectMySQL), yaml)
		if !t23HasRule(strict, t23AutoIncrementRule) {
			t.Fatalf("mysql non-allocating PK must still block, got %+v", strict.Findings)
		}
	}
}

// unsigned.require is an independent policy: disabling it removes the
// unsigned blocker while auto_increment.require keeps working.
func TestT23PolicyUnsignedRequireStaysIndependent(t *testing.T) {
	t.Parallel()
	stmt := t23AuditWithConfig(t, "CREATE TABLE t (id BIGINT PRIMARY KEY AUTO_RANDOM);",
		string(DialectTiDB), `
rules:
  ddl.table.primary_key.unsigned.require:
    enabled: false
`)
	if t23HasRule(stmt, "ddl.table.primary_key.unsigned.require") {
		t.Fatalf("disabled unsigned.require must not fire, got %+v", stmt.Findings)
	}
}
