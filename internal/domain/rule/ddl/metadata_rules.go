// Package ddl defines Tier-1 DDL rules.
// input: metadata-enriched DDL Statement specs plus per-rule policy values
// output: existence and snapshot-backed findings for create-table and alter-table operations
// pos: DDL rule implementations that depend on optional metadata-aware facts
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type tableExistenceRule struct {
	ruleID       string
	requireExist bool
	level        rule.Level
}

func newTableExistenceRule(ruleID string, requireExist bool, fallbackLevel rule.Level, cfg policy.RulePolicy) (rule.StatementRule, error) {
	return tableExistenceRule{
		ruleID:       ruleID,
		requireExist: requireExist,
		level:        configuredLevel(cfg, fallbackLevel),
	}, nil
}

func (r tableExistenceRule) ID() string { return r.ruleID }

func (r tableExistenceRule) AppliesTo(statement spec.Statement) bool {
	if appliesToCreateTable(statement) {
		return !r.requireExist
	}
	if appliesToAlterTable(statement) {
		return r.requireExist
	}
	return false
}

func (r tableExistenceRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	snapshot, ok := targetTableSnapshot(statement)
	if !ok {
		return nil, nil
	}

	switch {
	case r.requireExist && !snapshot.Exists:
		return []rule.Finding{{
			Level:      r.level,
			Message:    fmt.Sprintf("table %q does not exist in the target schema", statement.DDL.Table.Name),
			Suggestion: "create the table first or run the audit without metadata mode if live schema checks are unavailable",
			Metadata: map[string]any{
				"table":  statement.DDL.Table.Name,
				"exists": false,
			},
		}}, nil
	case !r.requireExist && snapshot.Exists:
		return []rule.Finding{{
			Level:      r.level,
			Message:    fmt.Sprintf("table %q already exists in the target schema", statement.DDL.Table.Name),
			Suggestion: "rename the table, switch to ALTER TABLE, or remove metadata mode if existence checks are not desired",
			Metadata: map[string]any{
				"table":  statement.DDL.Table.Name,
				"exists": true,
			},
		}}, nil
	default:
		return nil, nil
	}
}

type alterObjectExistenceRule struct {
	ruleID         string
	actions        []string
	objectLabel    string
	forbidIfExists bool
	selectName     func(spec.Alter) string
	checkExists    func(*spec.TableSnapshot, string) bool
	level          rule.Level
}

func newAlterObjectExistenceRule(ruleID string, actions []string, objectLabel string, forbidIfExists bool, fallbackLevel rule.Level, cfg policy.RulePolicy, selectName func(spec.Alter) string, checkExists func(*spec.TableSnapshot, string) bool) (rule.StatementRule, error) {
	return alterObjectExistenceRule{
		ruleID:         ruleID,
		actions:        actions,
		objectLabel:    objectLabel,
		forbidIfExists: forbidIfExists,
		selectName:     selectName,
		checkExists:    checkExists,
		level:          configuredLevel(cfg, fallbackLevel),
	}, nil
}

func (r alterObjectExistenceRule) ID() string { return r.ruleID }

func (r alterObjectExistenceRule) AppliesTo(statement spec.Statement) bool {
	return len(r.actions) > 0 && appliesToAlterActions(statement, r.actions...)
}

func (r alterObjectExistenceRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	snapshot, ok := targetTableSnapshot(statement)
	if !ok || !snapshot.Exists {
		return nil, nil
	}

	findings := make([]rule.Finding, 0)
	for _, alter := range matchingAlterActions(statement, r.actions...) {
		name := r.selectName(alter)
		if name == "" {
			continue
		}
		exists := r.checkExists(snapshot, name)
		if r.forbidIfExists && exists {
			findings = append(findings, rule.Finding{
				Level:      r.level,
				Message:    fmt.Sprintf("%s %q already exists on table %q", r.objectLabel, name, statement.DDL.Table.Name),
				Suggestion: fmt.Sprintf("pick a different %s name or remove the duplicate add operation", r.objectLabel),
				Metadata: map[string]any{
					"table":  statement.DDL.Table.Name,
					"action": alter.Action,
					"name":   name,
					"exists": true,
				},
			})
			continue
		}
		if !r.forbidIfExists && !exists {
			findings = append(findings, rule.Finding{
				Level:      r.level,
				Message:    fmt.Sprintf("%s %q does not exist on table %q", r.objectLabel, name, statement.DDL.Table.Name),
				Suggestion: fmt.Sprintf("fix the %s name or refresh metadata before auditing this change", r.objectLabel),
				Metadata: map[string]any{
					"table":  statement.DDL.Table.Name,
					"action": alter.Action,
					"name":   name,
					"exists": false,
				},
			})
		}
	}
	return findings, nil
}

type alterPrimaryKeyExistenceRule struct {
	ruleID string
	level  rule.Level
}

func newAlterPrimaryKeyExistenceRule(ruleID string, fallbackLevel rule.Level, cfg policy.RulePolicy) (rule.StatementRule, error) {
	return alterPrimaryKeyExistenceRule{
		ruleID: ruleID,
		level:  configuredLevel(cfg, fallbackLevel),
	}, nil
}

func (r alterPrimaryKeyExistenceRule) ID() string { return r.ruleID }

func (r alterPrimaryKeyExistenceRule) AppliesTo(statement spec.Statement) bool {
	return appliesToAlterActions(statement, "drop_primary_key")
}

func (r alterPrimaryKeyExistenceRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	snapshot, ok := targetTableSnapshot(statement)
	if !ok || !snapshot.Exists || snapshot.HasPrimaryKey() {
		return nil, nil
	}
	return []rule.Finding{{
		Level:      r.level,
		Message:    fmt.Sprintf("primary key does not exist on table %q", statement.DDL.Table.Name),
		Suggestion: "remove the drop primary key action or refresh metadata before auditing this change",
		Metadata: map[string]any{
			"table":  statement.DDL.Table.Name,
			"action": "drop_primary_key",
			"exists": false,
		},
	}}, nil
}

func alterObjectName(alter spec.Alter) string {
	return alter.Name
}

func snapshotHasColumn(snapshot *spec.TableSnapshot, name string) bool {
	return snapshot.HasColumn(name)
}

func snapshotHasIndex(snapshot *spec.TableSnapshot, name string) bool {
	return snapshot.HasIndex(name)
}
