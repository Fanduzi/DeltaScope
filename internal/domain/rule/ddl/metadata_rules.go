// Package ddl defines Tier-1 DDL rules.
// input: metadata-enriched DDL Statement specs plus per-rule policy values, where enriched snapshots may be unknown, known-absent, complete, or complete-present with incomplete structural collections
// output: existence and snapshot-backed findings for create-table, alter-table, and standalone create-index operations, plus evidence gaps (unknown_table_state / table_not_found) on the whitelisted existence rules when the ordered schema state cannot prove a premise
// pos: DDL rule implementations that depend on optional metadata-aware or batch-derived facts
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

const (
	// gapReasonUnknownTableState marks statements whose target table state the
	// ordered batch view could not establish (no provider snapshot, no
	// reliable in-batch derivation, a withheld member collection, or a
	// contaminated entry). The frozen gap vocabulary has no separate
	// "incomplete structure" reason — unknown member facts report the same
	// unknown-state reason with narrower required_facts.
	gapReasonUnknownTableState = "unknown_table_state"
)

// orderedGapDialects scopes this slice's evidence-gap projection to the
// dialects the ordered-state machine serves. PostgreSQL keeps its legacy
// contract — real provider-backed findings still fire through Evaluate, but
// the new unknown-state gaps never appear on it.
func orderedGapDialect(dialect spec.Dialect) bool {
	return dialect == spec.DialectMySQL || dialect == spec.DialectTiDB
}

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

func (r tableExistenceRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	snapshot, ok := targetTableSnapshot(statement)
	if !ok || snapshot == nil {
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

// EvidenceGaps reports the table-existence fact this rule needed but could not
// establish. A nil or absent snapshot is definite evidence (the rule either
// fires or stays silent on purpose); only an unknown target state is a gap.
func (r tableExistenceRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !orderedGapDialect(statement.Dialect) || !r.AppliesTo(statement) {
		return nil
	}
	snapshot, ok := targetTableSnapshot(statement)
	if ok && snapshot != nil {
		return nil
	}
	return []rule.EvidenceGap{{
		ReasonCode:    gapReasonUnknownTableState,
		RequiredFacts: []string{"target_table.existence"},
	}}
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

func (r alterObjectExistenceRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	snapshot, ok := targetTableSnapshot(statement)
	// A present table with nil Columns means the column shape was not
	// provided — column-member existence checks must skip rather than
	// fabricate absent members. Index members are not gated on Columns: a
	// loaded-but-empty Indexes set is authoritative for the provider — but a
	// projection that cannot vouch for indexes (IndexesUnknown) must skip
	// the same way.
	if !ok || snapshot == nil || !snapshot.Exists ||
		(r.objectLabel == "column" && snapshot.Columns == nil) ||
		(r.objectLabel == "index" && snapshot.IndexesUnknown) {
		return nil, nil
	}

	findings := make([]rule.Finding, 0)
	tableName := metadataTargetTableName(statement, snapshot)
	for _, alter := range matchingAlterObjectActions(statement, r.actions...) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
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

// EvidenceGaps reports missing member-existence evidence for every rule in
// this shared class on the ordered-state path. The knowledge premise mirrors
// Evaluate exactly: a rule that skips because its member collection is
// unknown must also report the gap, so "not checked" can never read as
// "checked". A confirmed-absent table is a settled parent failure and adds
// no member gap.
func (r alterObjectExistenceRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !orderedGapDialect(statement.Dialect) || !r.AppliesTo(statement) {
		return nil
	}
	snapshot, ok := targetTableSnapshot(statement)
	return memberExistenceGaps(snapshot, ok, r.objectLabel)
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

func (r alterPrimaryKeyExistenceRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	snapshot, ok := targetTableSnapshot(statement)
	// Unknown member collections must not read as "primary key absent":
	// PrimaryKeyUnknown/ConstraintsUnknown mark projections that cannot
	// vouch for those sets, and nil-provider/derived-incomplete shapes
	// stay silent instead of fabricating a missing-key finding.
	if !ok || snapshot == nil || !snapshot.Exists ||
		snapshot.PrimaryKeyUnknown || snapshot.ConstraintsUnknown || snapshot.HasPrimaryKey() {
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

// EvidenceGaps reports the primary-key fact this rule needed but could not
// establish. Only the require-presence variant opts in; the forbid variant
// bans the action outright and needs no member facts. A confirmed-absent
// table is a settled parent failure — no member gap is emitted.
func (r alterPrimaryKeyExistenceRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if r.ruleID != ruleIDAlterDropPrimaryKeyExistsRequire ||
		!orderedGapDialect(statement.Dialect) || !r.AppliesTo(statement) {
		return nil
	}
	snapshot, ok := targetTableSnapshot(statement)
	if !ok || snapshot == nil {
		return []rule.EvidenceGap{{
			ReasonCode:    gapReasonUnknownTableState,
			RequiredFacts: []string{"target_table.primary_key", "target_table.existence"},
		}}
	}
	if snapshot.Exists && (snapshot.PrimaryKeyUnknown || snapshot.ConstraintsUnknown) {
		return []rule.EvidenceGap{{
			ReasonCode:    gapReasonUnknownTableState,
			RequiredFacts: []string{"target_table.primary_key"},
		}}
	}
	return nil
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

// alterColumnPositionExistsRule requires the AFTER relative column of a
// positioned add_columns/modify_column/change_column action to exist on the
// target table. FIRST carries no relative-column premise, so the rule only
// applies when at least one action declares an AFTER reference.
type alterColumnPositionExistsRule struct {
	inner alterObjectExistenceRule
}

func newAlterColumnPositionExistsRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	inner, err := newAlterObjectExistenceRule(
		ruleIDAlterColumnPositionExistsRequire,
		[]string{"add_columns", "modify_column", "change_column"},
		"column", false, rule.LevelBlocker, cfg, alterPositionAfterName, snapshotHasColumn)
	if err != nil {
		return nil, err
	}
	return alterColumnPositionExistsRule{inner: inner.(alterObjectExistenceRule)}, nil
}

func (r alterColumnPositionExistsRule) ID() string { return r.inner.ruleID }

func (r alterColumnPositionExistsRule) AppliesTo(statement spec.Statement) bool {
	for _, alter := range matchingAlterObjectActions(statement, r.inner.actions...) {
		if alter.Position != nil && alter.Position.Kind == "after" {
			return true
		}
	}
	return false
}

func (r alterColumnPositionExistsRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	if statement.Dialect != spec.DialectMySQL {
		// TiDB (and any other ordered-state dialect) validates AFTER
		// references against the pre-state only.
		return r.inner.Evaluate(ctx, statement)
	}
	return r.evaluateAccumulated(ctx, statement)
}

// evaluateAccumulated mirrors alterObjectExistenceRule.Evaluate but resolves
// each AFTER reference against the state MySQL validates: the pre-state plus
// column names materialized by earlier actions in the same statement minus
// names those actions destroyed (left-to-right clause semantics). Actions
// outside the position-capable set — DROP, RENAME — still update the
// accumulated name set so sibling premises match the server's working state.
func (r alterColumnPositionExistsRule) evaluateAccumulated(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	snapshot, ok := targetTableSnapshot(statement)
	if !ok || snapshot == nil || !snapshot.Exists || snapshot.Columns == nil {
		return nil, nil
	}

	if statement.DDL == nil {
		return nil, nil
	}
	actions := statement.DDL.Alter
	findings := make([]rule.Finding, 0)
	tableName := metadataTargetTableName(statement, snapshot)
	produced := make(map[string]struct{})
	removed := make(map[string]struct{})
	positioned := map[string]bool{}
	for _, action := range r.inner.actions {
		positioned[action] = true
	}
	for _, alter := range actions {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if name := alterPositionAfterName(alter); positioned[alter.Action] && name != "" &&
			!producedName(produced, name) && !(snapshotHasColumn(snapshot, name) && !removedName(removed, name)) {
			findings = append(findings, rule.Finding{
				Level:      r.inner.level,
				Message:    fmt.Sprintf("column %q does not exist on table %q", name, tableName),
				Suggestion: "fix the column name or refresh metadata before auditing this change",
				Metadata: map[string]any{
					"table":  tableName,
					"action": alter.Action,
					"name":   name,
					"exists": false,
				},
			})
		}
		for _, name := range alterRemovedColumnNames(alter) {
			lower := strings.ToLower(name)
			removed[lower] = struct{}{}
			delete(produced, lower)
		}
		for _, name := range alterProducedColumnNames(alter) {
			lower := strings.ToLower(name)
			produced[lower] = struct{}{}
			delete(removed, lower)
		}
	}
	return findings, nil
}

func producedName(produced map[string]struct{}, name string) bool {
	_, hit := produced[strings.ToLower(name)]
	return hit
}

func removedName(removed map[string]struct{}, name string) bool {
	_, hit := removed[strings.ToLower(name)]
	return hit
}

// alterRemovedColumnNames returns the column identities an alter action
// destroys within the same statement — a dropped column, or the source name
// a rename/change migrates away from. A same-name change removes nothing.
func alterRemovedColumnNames(alter spec.Alter) []string {
	switch alter.Action {
	case "drop_column":
		if strings.TrimSpace(alter.Name) != "" {
			return []string{alter.Name}
		}
	case "change_column", "rename_column":
		oldName := strings.TrimSpace(alter.Name)
		column, ok := alterColumnDefinition(alter)
		if !ok {
			return nil
		}
		if oldName != "" && !strings.EqualFold(oldName, strings.TrimSpace(column.Name)) {
			return []string{oldName}
		}
	}
	return nil
}

// alterProducedColumnNames returns the column names an alter action
// materializes within the same statement — only additions and the
// destination names of change/rename create a name. A setter, visibility,
// or modify target definition names an existing column; counting it as
// produced would launder a missing sibling reference.
func alterProducedColumnNames(alter spec.Alter) []string {
	switch alter.Action {
	case "add_columns":
		if column, ok := alterColumnDefinition(alter); ok && strings.TrimSpace(column.Name) != "" {
			return []string{column.Name}
		}
		if strings.TrimSpace(alter.Name) != "" {
			return []string{alter.Name}
		}
	case "change_column", "rename_column":
		if column, ok := alterColumnDefinition(alter); ok && strings.TrimSpace(column.Name) != "" {
			return []string{column.Name}
		}
	}
	return nil
}

// EvidenceGaps mirrors the inner member-existence projection but stays
// gated on an actual AFTER premise — a positioned-FIRST or unpositioned
// alter never reports a position-existence gap.
func (r alterColumnPositionExistsRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(statement) {
		return nil
	}
	return r.inner.EvidenceGaps(statement)
}

// alterPositionAfterName selects the relative column an AFTER clause
// references; FIRST and unpositioned actions carry no existence premise.
func alterPositionAfterName(alter spec.Alter) string {
	if alter.Position == nil || alter.Position.Kind != "after" {
		return ""
	}
	return alter.Position.After
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

// memberCollectionFact returns the required_facts name of the collection a
// member rule consults.
func memberCollectionFact(objectLabel string) string {
	if objectLabel == "index" {
		return "target_table.indexes"
	}
	return "target_table.columns"
}

// memberCollectionUnknown mirrors the Evaluate skip premise for each
// collection: a withheld column set is a nil Columns slice, while an index
// set the projection cannot vouch for is marked IndexesUnknown (a loaded
// empty Indexes list stays an authoritative answer).
func memberCollectionUnknown(snapshot *spec.TableSnapshot, objectLabel string) bool {
	if objectLabel == "index" {
		return snapshot.IndexesUnknown
	}
	return snapshot.Columns == nil
}

// memberExistenceGaps is the shared evidence-gap projection for rules that
// check table-member existence: an unknown target state needs both facts, a
// confirmed-present table whose relevant collection is unknown needs only
// that collection, and a confirmed-absent table is a settled parent failure
// that adds no member gap. The frozen fact ordering pins the member fact
// first for columns and existence first for indexes.
func memberExistenceGaps(snapshot *spec.TableSnapshot, ok bool, objectLabel string) []rule.EvidenceGap {
	collectionFact := memberCollectionFact(objectLabel)
	if !ok || snapshot == nil {
		if objectLabel == "index" {
			return []rule.EvidenceGap{{
				ReasonCode:    gapReasonUnknownTableState,
				RequiredFacts: []string{"target_table.existence", collectionFact},
			}}
		}
		return []rule.EvidenceGap{{
			ReasonCode:    gapReasonUnknownTableState,
			RequiredFacts: []string{collectionFact, "target_table.existence"},
		}}
	}
	if snapshot.Exists && memberCollectionUnknown(snapshot, objectLabel) {
		return []rule.EvidenceGap{{
			ReasonCode:    gapReasonUnknownTableState,
			RequiredFacts: []string{collectionFact},
		}}
	}
	return nil
}

// createIndexColumnsExistRule checks that every plain column part referenced
// by a standalone CREATE INDEX exists on the ordered-state view of the target
// table. Expression/prefix/descending parts stay out of Columns and are
// covered by the statement's own coverage/unsupported path, not by this rule.
type createIndexColumnsExistRule struct {
	ruleID   string
	required bool
	level    rule.Level
}

func newCreateIndexColumnsExistRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	required, err := boolParam(ruleIDCreateIndexColumnsExistRequire, cfg, "required", true)
	if err != nil {
		return nil, err
	}
	return createIndexColumnsExistRule{
		ruleID:   ruleIDCreateIndexColumnsExistRequire,
		required: required,
		level:    configuredLevel(cfg, rule.LevelBlocker),
	}, nil
}

func (r createIndexColumnsExistRule) ID() string { return r.ruleID }

func (r createIndexColumnsExistRule) AppliesTo(statement spec.Statement) bool {
	if !r.required {
		return false
	}
	if statement.Dialect != spec.DialectMySQL && statement.Dialect != spec.DialectTiDB {
		return false
	}
	if statement.Kind != spec.KindDDL || statement.DDL == nil ||
		statement.DDL.Operation != spec.DDLOperationCreateIndex || statement.DDL.Table == nil {
		return false
	}
	return len(standaloneCreateIndexActions(statement)) > 0
}

func (r createIndexColumnsExistRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	snapshot, ok := targetTableSnapshot(statement)
	if !ok || snapshot == nil {
		return nil, nil
	}

	tableName := metadataTargetTableName(statement, snapshot)
	// The snapshot's schema is the effective target identity — an explicit
	// qualifier beats the request schema recorded on statement metadata.
	schema := strings.TrimSpace(snapshot.Schema)
	if schema == "" {
		schema = strings.TrimSpace(statement.Metadata.Schema)
	}

	// A confirmed-absent table is a settled parent failure: report the
	// missing target once instead of inventing a missing list per column.
	if !snapshot.Exists {
		indexName := ""
		for _, alter := range standaloneCreateIndexActions(statement) {
			if definition, hasDefinition := alterIndexDefinition(alter); hasDefinition {
				indexName = definition.Name
				break
			}
		}
		return []rule.Finding{{
			Level:      r.level,
			Message:    fmt.Sprintf("table %q does not exist in the target schema", tableName),
			Suggestion: "create the table before creating indexes on it, or audit without metadata mode",
			Metadata: map[string]any{
				"schema": schema,
				"table":  tableName,
				"index":  indexName,
				"reason": "table_not_found",
			},
		}}, nil
	}
	if snapshot.Columns == nil {
		return nil, nil
	}

	findings := make([]rule.Finding, 0)
	reported := make(map[string]struct{})
	for _, alter := range standaloneCreateIndexActions(statement) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		definition, hasDefinition := alterIndexDefinition(alter)
		if !hasDefinition {
			continue
		}
		for _, column := range definition.Columns {
			key := strings.ToLower(column)
			if _, seen := reported[key]; seen || snapshot.HasColumn(column) {
				continue
			}
			reported[key] = struct{}{}
			findings = append(findings, rule.Finding{
				Level:      r.level,
				Message:    fmt.Sprintf("column %q referenced by index %q does not exist on table %q", column, definition.Name, tableName),
				Suggestion: "fix the index column list or create the missing column before adding the index",
				Metadata: map[string]any{
					"schema": schema,
					"table":  tableName,
					"index":  definition.Name,
					"column": column,
					"exists": false,
				},
			})
		}
	}
	return findings, nil
}

// EvidenceGaps reports the target-table facts this rule could not establish.
// A definite absent table is a settled answer (the parent-level finding above
// fires), so only unknown state and incomplete structure are gaps.
func (r createIndexColumnsExistRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(statement) {
		return nil
	}
	snapshot, ok := targetTableSnapshot(statement)
	return memberExistenceGaps(snapshot, ok, "column")
}

func standaloneCreateIndexActions(statement spec.Statement) []spec.Alter {
	if statement.DDL == nil {
		return nil
	}
	matched := make([]spec.Alter, 0, len(statement.DDL.Alter))
	for _, alter := range statement.DDL.Alter {
		if strings.EqualFold(alter.Action, "create_index") {
			matched = append(matched, alter)
		}
	}
	return matched
}
