// Package ddl defines Tier-1 DDL rules.
// input: metadata-enriched DDL Statement specs plus per-rule policy values
// output: existence and snapshot-backed findings for create-table and alter-table operations
// pos: DDL rule implementations that depend on optional metadata-aware facts
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"fmt"
	"strings"

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
	return len(r.actions) > 0 && len(matchingAlterObjectActions(statement, r.actions...)) > 0
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
	tableName := metadataTargetTableName(statement, snapshot)
	for _, alter := range matchingAlterObjectActions(statement, r.actions...) {
		name := r.selectName(alter)
		if name == "" {
			continue
		}
		exists := r.checkExists(snapshot, name)
		if r.forbidIfExists && exists {
			findings = append(findings, rule.Finding{
				Level:      r.level,
				Message:    fmt.Sprintf("%s %q already exists on table %q", r.objectLabel, name, tableName),
				Suggestion: fmt.Sprintf("pick a different %s name or remove the duplicate add operation", r.objectLabel),
				Metadata: map[string]any{
					"table":  tableName,
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
				Message:    fmt.Sprintf("%s %q does not exist on table %q", r.objectLabel, name, tableName),
				Suggestion: fmt.Sprintf("fix the %s name or refresh metadata before auditing this change", r.objectLabel),
				Metadata: map[string]any{
					"table":  tableName,
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
	return len(matchingDropPrimaryKeyActions(statement)) > 0
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

func matchingAlterObjectActions(statement spec.Statement, actions ...string) []spec.Alter {
	matched := matchingAlterActions(statement, actions...)
	if len(matched) > 0 {
		return matched
	}
	return matchingStandaloneDDLActions(statement, actions...)
}

func metadataTargetTableName(statement spec.Statement, snapshot *spec.TableSnapshot) string {
	if statement.DDL != nil && statement.DDL.Table != nil && statement.DDL.Table.Name != "" {
		return statement.DDL.Table.Name
	}
	if snapshot != nil && snapshot.Table != nil && snapshot.Table.Name != "" {
		return snapshot.Table.Name
	}
	return ""
}

func matchingDropPrimaryKeyActions(statement spec.Statement) []spec.Alter {
	matches := matchingAlterActions(statement, "drop_primary_key")
	if len(matches) > 0 {
		return matches
	}
	snapshot, ok := targetTableSnapshot(statement)
	if !ok || snapshot == nil {
		return nil
	}
	constraintName := primaryKeyConstraintName(snapshot)
	if constraintName == "" {
		return nil
	}
	for _, alter := range matchingAlterActions(statement, "drop_constraint") {
		if alter.Name != "" && strings.EqualFold(alter.Name, constraintName) {
			matches = append(matches, spec.Alter{Action: "drop_primary_key", Name: alter.Name})
		}
	}
	return matches
}

func primaryKeyConstraintName(snapshot *spec.TableSnapshot) string {
	for _, constraint := range snapshot.Constraints {
		if constraint.Type == "primary_key" && constraint.Name != "" {
			return constraint.Name
		}
	}
	if snapshot.PrimaryKey != nil {
		return snapshot.PrimaryKey.Name
	}
	return ""
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
