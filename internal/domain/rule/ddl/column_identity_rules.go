// Package ddl defines Tier-1 DDL rules.
// input: one unconditional CHANGE COLUMN or RENAME COLUMN statement plus the ordered table snapshot and the statement version identity
// output: a target-name conflict blocker, or a RENAME COLUMN version blocker or evidence gap; the same identity emits neither a finding nor a gap
// pos: T05-A5 column-identity checks beside the existing source-existence rules; the version threshold is the shared spec function
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"context"
	"fmt"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type columnTargetExistsRule struct {
	ruleID string
	action string
	level  rule.Level
}

func newColumnTargetExistsRule(ruleID, action string, fallbackLevel rule.Level, cfg policy.RulePolicy) (rule.StatementRule, error) {
	return columnTargetExistsRule{
		ruleID: ruleID,
		action: action,
		level:  configuredLevel(cfg, fallbackLevel),
	}, nil
}

func (r columnTargetExistsRule) ID() string { return r.ruleID }

func (r columnTargetExistsRule) AppliesTo(statement spec.Statement) bool {
	_, ok := singleColumnIdentityAlter(statement, r.action)
	return ok
}

func (r columnTargetExistsRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	alter, ok := singleColumnIdentityAlter(statement, r.action)
	if !ok {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	oldName, newName, namesOK := columnIdentityAlterNames(alter)
	if !namesOK || strings.EqualFold(oldName, newName) {
		return nil, nil
	}
	snapshot, ok := targetTableSnapshot(statement)
	if !ok || snapshot == nil || !snapshot.Exists || snapshot.Columns == nil {
		return nil, nil
	}
	if !snapshot.HasColumn(newName) {
		return nil, nil
	}
	return []rule.Finding{{
		Level:      r.level,
		Message:    fmt.Sprintf("column %q already exists on table %q", newName, metadataTargetTableName(statement, snapshot)),
		Suggestion: "pick a column name that is not already present",
		Metadata: map[string]any{
			"table":         metadataTargetTableName(statement, snapshot),
			"action":        alter.Action,
			"source_column": oldName,
			"target_column": newName,
			"exists":        true,
		},
	}}, nil
}

func (r columnTargetExistsRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !orderedGapDialect(statement.Dialect) || !r.AppliesTo(statement) {
		return nil
	}
	alter, ok := singleColumnIdentityAlter(statement, r.action)
	if !ok {
		return nil
	}
	oldName, newName, namesOK := columnIdentityAlterNames(alter)
	if !namesOK || strings.EqualFold(oldName, newName) {
		return nil
	}
	snapshot, snapshotOK := targetTableSnapshot(statement)
	return memberExistenceGaps(snapshot, snapshotOK, "column")
}

type renameColumnVersionRule struct {
	ruleID   string
	required bool
	level    rule.Level
}

func newRenameColumnVersionRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	required, err := boolParam(ruleIDAlterRenameColumnVersionRequire, cfg, "required", true)
	if err != nil {
		return nil, err
	}
	return renameColumnVersionRule{
		ruleID:   ruleIDAlterRenameColumnVersionRequire,
		required: required,
		level:    configuredLevel(cfg, rule.LevelBlocker),
	}, nil
}

func (r renameColumnVersionRule) ID() string { return r.ruleID }

func (r renameColumnVersionRule) AppliesTo(statement spec.Statement) bool {
	if !r.required {
		return false
	}
	_, ok := singleColumnIdentityAlter(statement, "rename_column")
	return ok
}

func (r renameColumnVersionRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	version := statementVersionIdentity(statement)
	support := spec.RenameColumnVersionSupportFor(version)
	if !support.Incompatible || version == nil {
		return nil, nil
	}
	return []rule.Finding{{
		Level:      r.level,
		Message:    fmt.Sprintf("RENAME COLUMN requires MySQL %s or newer; %s is not supported", spec.RenameColumnMinimumSupportedVersion, version.Version),
		Suggestion: "use CHANGE COLUMN on this version, or retarget the audit at MySQL 8.0.3 or newer",
		Metadata: map[string]any{
			"action":                    "rename_column",
			"product":                   version.Product,
			"target_version":            version.Version,
			"minimum_supported_version": spec.RenameColumnMinimumSupportedVersion,
		},
	}}, nil
}

func (r renameColumnVersionRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(statement) {
		return nil
	}
	support := spec.RenameColumnVersionSupportFor(statementVersionIdentity(statement))
	if support.GapReason == "" {
		return nil
	}
	return []rule.EvidenceGap{{
		ReasonCode:    support.GapReason,
		RequiredFacts: append([]string(nil), support.GapFacts...),
	}}
}

// singleColumnIdentityAlter accepts one unconditional CHANGE or RENAME COLUMN.
// Multi-action, positional, and IF EXISTS forms stay outside this slice.
func singleColumnIdentityAlter(statement spec.Statement, action string) (spec.Alter, bool) {
	if statement.Dialect != spec.DialectMySQL && statement.Dialect != spec.DialectTiDB {
		return spec.Alter{}, false
	}
	if !appliesToAlterTable(statement) || len(statement.DDL.Alter) != 1 {
		return spec.Alter{}, false
	}
	alter := statement.DDL.Alter[0]
	if alter.Action != action || alter.HasColumnPosition {
		return spec.Alter{}, false
	}
	if alter.Options["if_exists"] == "true" || alter.Options["if_not_exists"] == "true" {
		return spec.Alter{}, false
	}
	if _, _, ok := columnIdentityAlterNames(alter); !ok {
		return spec.Alter{}, false
	}
	return alter, true
}

func columnIdentityAlterNames(alter spec.Alter) (string, string, bool) {
	if alter.Column == nil || alter.Column.Definition == nil {
		return "", "", false
	}
	oldName := strings.TrimSpace(alter.Name)
	columnOld := strings.TrimSpace(alter.Column.OldName)
	newName := strings.TrimSpace(alter.Column.Definition.Name)
	if oldName == "" || columnOld == "" || newName == "" || !strings.EqualFold(oldName, columnOld) {
		return "", "", false
	}
	return oldName, newName, true
}
