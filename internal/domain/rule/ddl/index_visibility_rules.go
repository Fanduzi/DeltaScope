// Package ddl defines Tier-1 DDL rules.
// input: normalized statements carrying T10 index visibility declarations
// and advanced-index member marks, plus metadata-enriched ordered snapshots
// output: invisibility-forbid findings, dialect-aware alter-index existence
// findings and evidence gaps (MySQL PRIMARY key / TiDB real object vs
// PKIsHandle), and product/version compatibility findings and gaps for
// visibility, expression keys, descending keys, multi-valued members,
// prefix length, fulltext, spatial, and global forms — including the
// implicit-primary role refusal on qualifying UNIQUE indexes
// pos: T10 advanced-index and visibility rule implementations
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

// indexInvisibilitySubject binds one index surface that declares or requests
// invisibility to the action that carries it.
type indexInvisibilitySubject struct {
	action string
	name   string
}

// invisibilitySubjects returns every invisibility event the statement
// declares: create-table index/PK attributes, standalone CREATE INDEX and
// ALTER ADD INDEX options, and ALTER INDEX ... INVISIBLE transitions.
// ALTER INDEX ... VISIBLE is not an invisibility event.
func invisibilitySubjects(statement spec.Statement) []indexInvisibilitySubject {
	var out []indexInvisibilitySubject
	if statement.DDL == nil {
		return out
	}
	add := func(index *spec.Index, action string) {
		if index == nil || index.Visibility != spec.ColumnVisibilityInvisible {
			return
		}
		out = append(out, indexInvisibilitySubject{
			action: action,
			name:   strings.TrimSpace(index.Name),
		})
	}
	switch statement.DDL.Operation {
	case spec.DDLOperationCreateTable:
		add(statement.DDL.PrimaryKey, "create_table")
		for i := range statement.DDL.Indexes {
			add(&statement.DDL.Indexes[i], "create_table")
		}
	case spec.DDLOperationCreateIndex, spec.DDLOperationAlterTable:
		for i := range statement.DDL.Alter {
			alter := &statement.DDL.Alter[i]
			if alter.Index == nil {
				continue
			}
			switch alter.Action {
			case "create_index", "add_index", "add_constraint":
				add(alter.Index.Definition, alter.Action)
			case "alter_index":
				definition, ok := alterIndexDefinition(*alter)
				if !ok || definition.Visibility != spec.ColumnVisibilityInvisible {
					continue
				}
				out = append(out, indexInvisibilitySubject{
					action: alter.Action,
					name:   alterIndexTargetName(*alter),
				})
			}
		}
	}
	return out
}

// indexInvisibleForbidRule rejects any index the statement hides — a declared
// INVISIBLE attribute or an ALTER INDEX ... INVISIBLE transition — with a
// configurable policy level.
type indexInvisibleForbidRule struct {
	ruleID string
	level  rule.Level
}

func newIndexInvisibleForbidRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return indexInvisibleForbidRule{
		ruleID: ruleIDIndexInvisibleForbid,
		level:  configuredLevel(cfg, rule.LevelBlocker),
	}, nil
}

func (r indexInvisibleForbidRule) ID() string { return r.ruleID }

func (r indexInvisibleForbidRule) AppliesTo(statement spec.Statement) bool {
	return len(invisibilitySubjects(statement)) > 0
}

func (r indexInvisibleForbidRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	snapshot, _ := targetTableSnapshot(statement)
	table := metadataTargetTableName(statement, snapshot)
	var findings []rule.Finding
	for _, subject := range invisibilitySubjects(statement) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		findings = append(findings, rule.Finding{
			Level:   r.level,
			Message: fmt.Sprintf("index %q is declared or transitioned invisible on table %q", subject.name, table),
			Metadata: map[string]any{
				"table":      table,
				"index":      subject.name,
				"action":     subject.action,
				"visibility": spec.ColumnVisibilityInvisible,
			},
		})
	}
	return findings, nil
}

// alterIndexTargetExists resolves the ALTER INDEX target against the
// ordered snapshot with a dialect-aware PRIMARY shortcut: MySQL names the
// primary key itself (the 3522 visibility refusal then belongs to the
// compatibility rule). TiDB resolves `PRIMARY` to a real index object only
// on non-handle tables — a PKIsHandle single-int clustered PK names no
// index object (1176), while NONCLUSTERED and non-int clustered primary
// keys are operable objects. An unvouched primary-key collection answers
// true on either dialect — presence is unproven rather than absent, and
// the rule's EvidenceGaps reports the missing primary_key fact instead of
// a fabricated blocker.
func alterIndexTargetExists(statement spec.Statement, snapshot *spec.TableSnapshot, name string) bool {
	if snapshotHasIndex(statement, snapshot, name) {
		return true
	}
	if (statement.Dialect == spec.DialectMySQL || statement.Dialect == spec.DialectTiDB) && strings.EqualFold(name, "primary") {
		exists, resolved := spec.ResolveIndexPrimaryObject(snapshot, statement.Dialect)
		if !resolved {
			// An unvouched primary-key state suppresses the blocker — the
			// rule's EvidenceGaps reports the missing primary_key fact
			// instead of a fabricated absence.
			return true
		}
		return exists
	}
	return false
}

// alterIndexExistsRule reuses the shared existence frame but reports the
// extra primary_key fact an unvouched PRIMARY target leaves behind.
type alterIndexExistsRule struct {
	inner alterObjectExistenceRule
}

func newAlterIndexExistsRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	inner, err := newAlterObjectExistenceRule(
		ruleIDAlterIndexExistsRequire,
		[]string{"alter_index"},
		"index", false, rule.LevelBlocker, cfg, alterIndexTargetName, alterIndexTargetExists)
	if err != nil {
		return nil, err
	}
	return alterIndexExistsRule{inner: inner.(alterObjectExistenceRule)}, nil
}

func (r alterIndexExistsRule) ID() string { return r.inner.ruleID }

func (r alterIndexExistsRule) AppliesTo(statement spec.Statement) bool {
	return r.inner.AppliesTo(statement)
}

func (r alterIndexExistsRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	return r.inner.Evaluate(ctx, statement)
}

// EvidenceGaps appends a target_table.primary_key gap when a MySQL
// alter_index PRIMARY target cannot be judged because the primary-key
// collection is unvouched — the suppressed finding must surface as a gap,
// not disappear.
func (r alterIndexExistsRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	gaps := r.inner.EvidenceGaps(statement)
	if statement.Dialect != spec.DialectMySQL && statement.Dialect != spec.DialectTiDB {
		return gaps
	}
	primaryTarget := false
	for _, alter := range matchingAlterObjectActions(statement, r.inner.actions...) {
		if strings.EqualFold(alterIndexTargetName(alter), "primary") {
			primaryTarget = true
			break
		}
	}
	if !primaryTarget {
		return gaps
	}
	snapshot, ok := targetTableSnapshot(statement)
	if !ok || snapshot == nil || !snapshot.Exists || !(snapshot.PrimaryKeyUnknown || snapshot.ConstraintsUnknown) {
		return gaps
	}
	return append(gaps, rule.EvidenceGap{
		ReasonCode:    gapReasonUnknownTableState,
		RequiredFacts: []string{"target_table.primary_key"},
	})
}

var indexAdvancedCompatibilityFeatures = map[string]bool{
	spec.AdvancedIndexFeatureVisibility:     true,
	spec.AdvancedIndexFeatureExpressionKeys: true,
	spec.AdvancedIndexFeatureDescendingKeys: true,
	spec.AdvancedIndexFeatureMultiValued:    true,
	spec.AdvancedIndexFeaturePrefixLength:   true,
	spec.AdvancedIndexFeatureFulltext:       true,
	spec.AdvancedIndexFeatureSpatial:        true,
	spec.AdvancedIndexFeatureGlobal:         true,
}

// indexAdvancedCompatibilityRule enforces the frozen T10 product/version
// responsibility table for index visibility, expression keys, descending
// keys, multi-valued members, and the member-legality families
// prefix_length, fulltext, spatial, and global (primary-key invisibility
// and TiDB PRIMARY-object identity included). Determined failures become
// findings; missing version/fact evidence becomes a bounded gap, never a
// silent pass.
type indexAdvancedCompatibilityRule struct {
	ruleID   string
	required bool
	level    rule.Level
	features map[string]bool
}

func newIndexAdvancedCompatibilityRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	required, err := boolParam(ruleIDIndexAdvancedCompatibilityRequire, cfg, "required", true)
	if err != nil {
		return nil, err
	}
	features := map[string]bool{
		spec.AdvancedIndexFeatureVisibility:     true,
		spec.AdvancedIndexFeatureExpressionKeys: true,
		spec.AdvancedIndexFeatureDescendingKeys: true,
		spec.AdvancedIndexFeatureMultiValued:    true,
	}
	if raw, ok := cfg.Params["features"]; ok {
		features = map[string]bool{}
		var names []string
		switch list := raw.(type) {
		case []string:
			names = list
		case []any:
			for _, item := range list {
				name, ok := item.(string)
				if !ok {
					return nil, fmt.Errorf("rule %s param %q entries must be strings, got %T", ruleIDIndexAdvancedCompatibilityRequire, "features", item)
				}
				names = append(names, name)
			}
		default:
			return nil, fmt.Errorf("rule %s param %q must be a list of feature names, got %T", ruleIDIndexAdvancedCompatibilityRequire, "features", raw)
		}
		for _, name := range names {
			if !indexAdvancedCompatibilityFeatures[name] {
				return nil, fmt.Errorf("rule %s param %q has unknown feature %q", ruleIDIndexAdvancedCompatibilityRequire, "features", name)
			}
			features[name] = true
		}
	}
	return indexAdvancedCompatibilityRule{
		ruleID:   ruleIDIndexAdvancedCompatibilityRequire,
		required: required,
		level:    configuredLevel(cfg, rule.LevelBlocker),
		features: features,
	}, nil
}

func (r indexAdvancedCompatibilityRule) ID() string { return r.ruleID }

func (r indexAdvancedCompatibilityRule) featureEnabled(feature string) bool {
	return r.features[feature]
}

func (r indexAdvancedCompatibilityRule) AppliesTo(statement spec.Statement) bool {
	if !r.required || len(r.features) == 0 {
		return false
	}
	assessment := spec.AssessAdvancedIndexes(statement)
	for feature := range assessment.Features {
		if r.featureEnabled(feature) {
			return true
		}
	}
	return false
}

func (r indexAdvancedCompatibilityRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	assessment := spec.AssessAdvancedIndexes(statement)
	snapshot, _ := targetTableSnapshot(statement)
	table := metadataTargetTableName(statement, snapshot)
	targetVersion := ""
	if statement.Metadata != nil && statement.Metadata.Version != nil {
		targetVersion = statement.Metadata.Version.Version
	}
	var findings []rule.Finding
	for _, issue := range assessment.Issues {
		if !r.featureEnabled(issue.Feature) {
			continue
		}
		findings = append(findings, rule.Finding{
			Level:   r.level,
			Message: fmt.Sprintf("index %q advanced form is not supported for the target (%s)", issue.Index, issue.ReasonCode),
			Metadata: map[string]any{
				"table":           table,
				"index":           issue.Index,
				"feature":         issue.Feature,
				"action":          issue.Action,
				"reason_code":     issue.ReasonCode,
				"product":         spec.VersionProductForDialect(statement.Dialect),
				"target_version":  targetVersion,
				"minimum_version": issue.MinimumVersion,
			},
		})
	}
	return findings, nil
}

// EvidenceGaps projects feature-scoped compatibility gaps: version gaps apply
// only when an enabled feature consults the version, and feature-specific
// gaps stay inside their enabled family.
func (r indexAdvancedCompatibilityRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(statement) {
		return nil
	}
	assessment := spec.AssessAdvancedIndexes(statement)
	gated := false
	for feature := range assessment.VersionGated {
		if r.featureEnabled(feature) {
			gated = true
			break
		}
	}
	var gaps []rule.EvidenceGap
	for _, gap := range assessment.Gaps {
		versionGap := gap.ReasonCode == spec.AdvancedReasonMissingTargetVersion ||
			gap.ReasonCode == spec.AdvancedReasonOutOfValidatedRange
		if versionGap && !gated {
			continue
		}
		if gap.Feature != "" && !r.featureEnabled(gap.Feature) {
			continue
		}
		gaps = append(gaps, rule.EvidenceGap{
			ReasonCode:    gap.ReasonCode,
			RequiredFacts: append([]string(nil), gap.RequiredFacts...),
		})
	}
	return gaps
}
