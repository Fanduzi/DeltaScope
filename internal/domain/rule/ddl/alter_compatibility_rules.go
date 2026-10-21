// Package ddl defines Tier-1 DDL rules.
// input: metadata-enriched alter-table Statement specs with current and target column shapes
// output: source-aware alter compatibility findings and bounded evidence gaps for change/modify column operations,
// with the shared three-state default comparison (proven-same / proven-different / unproven) driving both
// pos: DDL compatibility rules layered above metadata-backed existence checks
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// Evidence-gap vocabulary for alter column compatibility (issue #83 T04-A).
// Reason codes and fact identifiers are a fixed public contract: they carry
// machine-readable fact names only, never SQL text, credentials, or provider
// error payloads.
const (
	gapReasonMissingSourceColumn    = "missing_source_column"
	gapReasonIncompleteSourceColumn = "incomplete_source_column"
	gapReasonUnprovenDefault        = "unproven_default_identity"

	factSourceColumnDefinition = "source_column.definition"
	factSourceColumnType       = "source_column.type"
	factSourceColumnLength     = "source_column.length"
	factSourceColumnDefault    = "source_column.default"
	factTargetColumnDefault    = "target_column.default"
)

// Effective-default classes for the MODIFY/CHANGE transition compare. These
// are catalog-shape classes, never declaration spelling: a `DEFAULT NULL`
// target stores a NULL COLUMN_DEFAULT exactly like an omitted clause, so the
// null class carries no default presence. Diagnostics expose only the class
// name — never literal text, expression text, or fingerprints.
const (
	defaultClassNone             = "none"
	defaultClassNull             = "null"
	defaultClassLiteral          = "literal"
	defaultClassCurrentTimestamp = "current_timestamp"
	defaultClassExpression       = "expression"
)

type alterColumnCompatibilityRule struct {
	ruleID   string
	action   string
	label    string
	required bool
	level    rule.Level
}

type alterTableOptionCompatibilityRule struct {
	required bool
	level    rule.Level
}

func newAlterColumnCompatibilityRule(ruleID, action, label string, fallbackLevel rule.Level, cfg policy.RulePolicy) (rule.StatementRule, error) {
	required, err := boolParam(ruleID, cfg, "required", true)
	if err != nil {
		return nil, err
	}
	// requires_metadata is accepted for config compatibility but inert: it no
	// longer gates evidence-gap emission. An enabled, required, applicable
	// rule always reports the facts it could not obtain.
	if _, err := boolParam(ruleID, cfg, "requires_metadata", false); err != nil {
		return nil, err
	}
	return alterColumnCompatibilityRule{
		ruleID:   ruleID,
		action:   action,
		label:    label,
		required: required,
		level:    configuredLevel(cfg, fallbackLevel),
	}, nil
}

func newAlterTableOptionCompatibilityRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	required, err := boolParam(ruleIDAlterTableOptionCompatibilityRequire, cfg, "required", true)
	if err != nil {
		return nil, err
	}
	return alterTableOptionCompatibilityRule{
		required: required,
		level:    configuredLevel(cfg, rule.LevelWarning),
	}, nil
}

func (r alterColumnCompatibilityRule) ID() string { return r.ruleID }

func (r alterTableOptionCompatibilityRule) ID() string {
	return ruleIDAlterTableOptionCompatibilityRequire
}

func (r alterColumnCompatibilityRule) AppliesTo(statement spec.Statement) bool {
	return r.required && appliesToAlterActions(statement, r.action)
}

func (r alterTableOptionCompatibilityRule) AppliesTo(statement spec.Statement) bool {
	return r.required && appliesToAlterActions(statement, "table_option")
}

func (r alterColumnCompatibilityRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	snapshot, ok := targetTableSnapshot(statement)
	if !ok || !snapshot.Exists {
		return nil, nil
	}

	findings := make([]rule.Finding, 0)
	for _, alter := range matchingAlterActions(statement, r.action) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		source := snapshot.FindColumn(alter.Name)
		target, ok := alterColumnDefinition(alter)
		if source == nil || !ok {
			continue
		}
		findings = append(findings, compatibilityFindings(r.ruleID, r.level, statement.DDL.Table.Name, alter, *source, *target)...)
	}
	return findings, nil
}

// EvidenceGaps reports which source-column facts the rule needed but could not
// obtain for an applicable alter. It runs whenever the rule is enabled,
// required, and applicable — there is no opt-in: silently skipping the fact
// check would let unverifiable audits report complete. Missing facts never
// suppress Evaluate — statements can carry both findings and gaps.
func (r alterColumnCompatibilityRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(statement) {
		return nil
	}

	missingDefinition := false
	incomplete := make(map[string]struct{}, 2)
	unproven := make(map[string]struct{}, 2)
	snapshot, hasSnapshot := targetTableSnapshot(statement)
	for _, alter := range matchingAlterActions(statement, r.action) {
		target, hasTarget := alterColumnDefinition(alter)
		if !hasSnapshot || snapshot == nil || !snapshot.Exists || !hasTarget {
			missingDefinition = true
			continue
		}
		source := snapshot.FindColumn(alter.Name)
		if source == nil {
			missingDefinition = true
			continue
		}
		if strings.TrimSpace(source.Type) == "" {
			incomplete[factSourceColumnType] = struct{}{}
		}
		if sourceLengthFactRequired(*source, target) && source.Length <= 0 {
			incomplete[factSourceColumnLength] = struct{}{}
		}
		// A source default whose comparable identity cannot be proven is a
		// fact deficit regardless of the transition verdict — a determined
		// presence flip or class switch does not wash the deficit away.
		if sourceDefaultIdentityUnproven(*source) {
			incomplete[factSourceColumnDefault] = struct{}{}
		}
		// The shared comparison is the one truth for unproven identity:
		// whatever Evaluate cannot convict, it also cannot call changed.
		_, factDeficits, pairDeficits := defaultComparison(*source, *target)
		for _, fact := range factDeficits {
			incomplete[fact] = struct{}{}
		}
		for _, fact := range pairDeficits {
			unproven[fact] = struct{}{}
		}
	}

	gaps := make([]rule.EvidenceGap, 0, 3)
	if missingDefinition {
		gaps = append(gaps, rule.EvidenceGap{
			ReasonCode:    gapReasonMissingSourceColumn,
			RequiredFacts: []string{factSourceColumnDefinition},
		})
	}
	if len(incomplete) > 0 {
		facts := make([]string, 0, len(incomplete))
		for fact := range incomplete {
			facts = append(facts, fact)
		}
		sort.Strings(facts)
		gaps = append(gaps, rule.EvidenceGap{
			ReasonCode:    gapReasonIncompleteSourceColumn,
			RequiredFacts: facts,
		})
	}
	if len(unproven) > 0 {
		facts := make([]string, 0, len(unproven))
		for fact := range unproven {
			facts = append(facts, fact)
		}
		sort.Strings(facts)
		gaps = append(gaps, rule.EvidenceGap{
			ReasonCode:    gapReasonUnprovenDefault,
			RequiredFacts: facts,
		})
	}
	return gaps
}

// sourceLengthFactRequired reports whether the source column length is a fact
// the compatibility comparison actually consumes: only a string-to-string
// transition compares lengths. An unknown source type stays required because
// it might be a string; a known non-string source never needs a length.
func sourceLengthFactRequired(source spec.Column, target *spec.Column) bool {
	if target == nil || columnTypeFamily(*target) != "string" {
		return false
	}
	if strings.TrimSpace(source.Type) == "" {
		return true
	}
	return columnTypeFamily(source) == "string"
}

// effectiveDefaultClass reduces a column's default facts to their
// catalog-shape class. Both call sites share the field vocabulary: the AST
// path asserts DefaultIsNull on an explicit `DEFAULT NULL`; the metadata
// path never sets it, so a source column can only classify to none,
// current_timestamp, expression, or literal.
func effectiveDefaultClass(column spec.Column) string {
	if !column.HasDefault {
		return defaultClassNone
	}
	switch {
	case column.DefaultIsNull:
		return defaultClassNull
	case column.DefaultIsCurrentTimestamp:
		return defaultClassCurrentTimestamp
	case column.DefaultExpression != nil:
		return defaultClassExpression
	default:
		return defaultClassLiteral
	}
}

// sourceDefaultIdentityUnproven reports a source column whose stored
// default exists but whose comparable identity cannot be proven: an
// expression fact the catalog text could not decode (or that carries no
// fingerprint), or a literal class with no comparable identity. A NULL or
// absent COLUMN_DEFAULT is a complete absence, never a deficit.
func sourceDefaultIdentityUnproven(source spec.Column) bool {
	if !source.HasDefault {
		return false
	}
	switch effectiveDefaultClass(source) {
	case defaultClassExpression:
		return source.DefaultExpression == nil || !source.DefaultExpression.Known || source.DefaultExpression.Fingerprint == ""
	case defaultClassLiteral:
		return source.DefaultLiteral == nil
	default:
		return false
	}
}

// defaultComparison is the single three-state comparison both Evaluate and
// EvidenceGaps consume: presence flips and class switches are determined
// transitions, a same-class identity is proven only through the comparable
// facts, and anything unprovable reports the fact names it needed — never
// a determined "changed" verdict on an unknown identity. The returned
// deficit lists split honest bookkeeping: factDeficits are identities a
// side could not produce (its own fact is incomplete), pairDeficits are
// proven-but-incomparable pairs (the comparison, not the fact, is
// unproven).
func defaultComparison(source spec.Column, target spec.Column) (transition string, factDeficits []string, pairDeficits []string) {
	sourceClass := effectiveDefaultClass(source)
	targetClass := effectiveDefaultClass(target)
	sourcePresent := sourceClass != defaultClassNone && sourceClass != defaultClassNull
	targetPresent := targetClass != defaultClassNone && targetClass != defaultClassNull
	if sourcePresent != targetPresent {
		if sourcePresent {
			return "loses its default value", nil, nil
		}
		return "gains a default value", nil, nil
	}
	if !sourcePresent {
		return "", nil, nil
	}
	if sourceClass != targetClass {
		return "changes its default value", nil, nil
	}
	if sourceClass == defaultClassCurrentTimestamp {
		// The temporal class is a single fixed identity with nothing left
		// to compare.
		return "", nil, nil
	}
	switch sourceClass {
	case defaultClassLiteral:
		return literalDefaultComparison(source.DefaultLiteral, target.DefaultLiteral)
	case defaultClassExpression:
		return expressionDefaultComparison(source.DefaultExpression, target.DefaultExpression)
	}
	return "", nil, nil
}

// literalDefaultComparison compares two literal-class identities. Equal
// kind+value proves same, equal kind with different value proves
// different, a missing identity is that side's fact deficit, and a
// cross-domain pair is a comparison deficit — kinds never coerce.
func literalDefaultComparison(source, target *spec.LiteralDefaultFacts) (string, []string, []string) {
	switch {
	case source == nil && target == nil:
		return "", []string{factSourceColumnDefault}, []string{factTargetColumnDefault}
	case source == nil:
		return "", []string{factSourceColumnDefault}, nil
	case target == nil:
		return "", nil, []string{factTargetColumnDefault}
	case source.Kind != target.Kind:
		return "", nil, []string{factSourceColumnDefault, factTargetColumnDefault}
	case source.Value != target.Value:
		return "changes its default value", nil, nil
	}
	return "", nil, nil
}

// expressionDefaultComparison compares two expression-class identities by
// fingerprint — equal non-empty fingerprints on Known facts prove same.
// A missing, unknown, or fingerprintless fact is unproven: an empty
// fingerprint is an unknown identity, never a determined change.
func expressionDefaultComparison(source, target *spec.ExpressionFacts) (string, []string, []string) {
	sourceProven := source != nil && source.Known && source.Fingerprint != ""
	targetProven := target != nil && target.Known && target.Fingerprint != ""
	switch {
	case !sourceProven && !targetProven:
		return "", []string{factSourceColumnDefault}, []string{factTargetColumnDefault}
	case !sourceProven:
		return "", []string{factSourceColumnDefault}, nil
	case !targetProven:
		return "", nil, []string{factTargetColumnDefault}
	case source.Fingerprint != target.Fingerprint:
		return "changes its default value", nil, nil
	}
	return "", nil, nil
}

func (r alterTableOptionCompatibilityRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	snapshot, ok := targetTableSnapshot(statement)
	if !ok || !snapshot.Exists {
		return nil, nil
	}

	findings := make([]rule.Finding, 0)
	for _, alter := range matchingAlterActions(statement, "table_option") {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		findings = append(findings, optionCompatibilityFindings(r.level, statement.DDL.Table.Name, snapshot, alter)...)
	}
	return findings, nil
}

func compatibilityFindings(ruleID string, level rule.Level, tableName string, alter spec.Alter, source spec.Column, target spec.Column) []rule.Finding {
	columnName := target.Name
	if columnName == "" {
		columnName = alter.Name
	}
	findings := make([]rule.Finding, 0)

	// Type-family and width comparisons require a known source type: an empty
	// type is an unverified fact, not the "other" family, so it must not
	// fabricate a family-change or narrowing finding. Attribute comparisons
	// below still run — their facts are known independently of the type.
	if strings.TrimSpace(source.Type) != "" {
		sourceFamily := columnTypeFamily(source)
		targetFamily := columnTypeFamily(target)
		if sourceFamily != targetFamily {
			findings = append(findings, newCompatibilityFinding(ruleID, level, tableName, alter, columnName,
				fmt.Sprintf("column %q changes type family from %q to %q", alter.Name, sourceFamily, targetFamily),
				"keep the column in the same type family or split the change into a reviewed migration",
				map[string]any{"source_type": source.Type, "target_type": target.Type, "source_family": sourceFamily, "target_family": targetFamily},
			))
			return findings
		}

		if sourceFamily == "integer" && integerTypeRank(target) < integerTypeRank(source) {
			findings = append(findings, newCompatibilityFinding(ruleID, level, tableName, alter, columnName,
				fmt.Sprintf("column %q narrows integer width from %q to %q", alter.Name, source.Type, target.Type),
				"widen the target integer type or migrate data before shrinking integer width",
				map[string]any{"source_type": source.Type, "target_type": target.Type},
			))
		}

		if sourceFamily == "string" && target.Length > 0 && source.Length > 0 && target.Length < source.Length {
			findings = append(findings, newCompatibilityFinding(ruleID, level, tableName, alter, columnName,
				fmt.Sprintf("column %q shrinks string length from %d to %d", alter.Name, source.Length, target.Length),
				"keep the new length at or above the current size or validate existing data before shrinking",
				map[string]any{"source_length": source.Length, "target_length": target.Length},
			))
		}
	}

	if source.Unsigned != target.Unsigned {
		findings = append(findings, newCompatibilityFinding(ruleID, level, tableName, alter, columnName,
			fmt.Sprintf("column %q changes unsigned mode from %t to %t", alter.Name, source.Unsigned, target.Unsigned),
			"keep signedness unchanged unless the current data range is fully reviewed",
			map[string]any{"source_unsigned": source.Unsigned, "target_unsigned": target.Unsigned},
		))
	}

	if !source.NotNull && target.NotNull {
		findings = append(findings, newCompatibilityFinding(ruleID, level, tableName, alter, columnName,
			fmt.Sprintf("column %q tightens nullability from nullable to not null", alter.Name),
			"clean existing NULL values before making the column NOT NULL",
			map[string]any{"source_not_null": source.NotNull, "target_not_null": target.NotNull},
		))
	}

	if source.AutoIncrement && !target.AutoIncrement {
		findings = append(findings, newCompatibilityFinding(ruleID, level, tableName, alter, columnName,
			fmt.Sprintf("column %q removes auto_increment", alter.Name),
			"keep auto_increment unchanged or perform a reviewed multi-step migration",
			map[string]any{"source_auto_increment": source.AutoIncrement, "target_auto_increment": target.AutoIncrement},
		))
	}

	if transition, _, _ := defaultComparison(source, target); transition != "" {
		findings = append(findings, newCompatibilityFinding(ruleID, level, tableName, alter, columnName,
			fmt.Sprintf("column %q %s", alter.Name, transition),
			"keep the declared default identical to the stored default or review the transition explicitly",
			map[string]any{
				"source_default": effectiveDefaultClass(source),
				"target_default": effectiveDefaultClass(target),
			},
		))
	}

	return findings
}

func newCompatibilityFinding(ruleID string, level rule.Level, tableName string, alter spec.Alter, columnName string, message string, suggestion string, metadata map[string]any) rule.Finding {
	payload := map[string]any{
		"table":       tableName,
		"action":      alter.Action,
		"name":        alter.Name,
		"column_name": columnName,
	}
	for key, value := range metadata {
		payload[key] = value
	}
	return rule.Finding{
		RuleID:     ruleID,
		Level:      level,
		Message:    message,
		Suggestion: suggestion,
		Metadata:   payload,
	}
}

func optionCompatibilityFindings(level rule.Level, tableName string, snapshot *spec.TableSnapshot, alter spec.Alter) []rule.Finding {
	findings := make([]rule.Finding, 0)
	for _, optionKey := range []string{"engine", "charset", "collation", "row_format", "auto_increment"} {
		target, ok := alter.Options[optionKey]
		if !ok || strings.TrimSpace(target) == "" {
			continue
		}
		source := strings.TrimSpace(snapshot.Options[optionKey])
		if source == "" {
			continue
		}
		if optionKey == "auto_increment" {
			if finding, ok := autoIncrementCompatibilityFinding(level, tableName, alter, source, target); ok {
				findings = append(findings, finding)
			}
			continue
		}
		if strings.EqualFold(source, target) {
			continue
		}
		findings = append(findings, rule.Finding{
			RuleID:     ruleIDAlterTableOptionCompatibilityRequire,
			Level:      level,
			Message:    fmt.Sprintf("table option %q changes from %q to %q", optionKey, source, target),
			Suggestion: fmt.Sprintf("keep %s aligned with the current table setting or review the compatibility impact explicitly", optionKey),
			Metadata: map[string]any{
				"table":        tableName,
				"action":       alter.Action,
				"option":       optionKey,
				"source_value": source,
				"target_value": target,
			},
		})
	}
	return findings
}

func autoIncrementCompatibilityFinding(level rule.Level, tableName string, alter spec.Alter, source, target string) (rule.Finding, bool) {
	sourceValue, sourceErr := strconv.ParseInt(source, 10, 64)
	targetValue, targetErr := strconv.ParseInt(target, 10, 64)
	if sourceErr == nil && targetErr == nil {
		if targetValue >= sourceValue {
			return rule.Finding{}, false
		}
		return rule.Finding{
			RuleID:     ruleIDAlterTableOptionCompatibilityRequire,
			Level:      level,
			Message:    fmt.Sprintf("table auto_increment seed shrinks from %d to %d", sourceValue, targetValue),
			Suggestion: "keep AUTO_INCREMENT at or above the current seed unless the reset is fully reviewed",
			Metadata: map[string]any{
				"table":        tableName,
				"action":       alter.Action,
				"option":       "auto_increment",
				"source_value": sourceValue,
				"target_value": targetValue,
			},
		}, true
	}
	if strings.EqualFold(source, target) {
		return rule.Finding{}, false
	}
	return rule.Finding{
		RuleID:     ruleIDAlterTableOptionCompatibilityRequire,
		Level:      level,
		Message:    fmt.Sprintf("table option %q changes from %q to %q", "auto_increment", source, target),
		Suggestion: "keep AUTO_INCREMENT aligned with the current seed or review the reset explicitly",
		Metadata: map[string]any{
			"table":        tableName,
			"action":       alter.Action,
			"option":       "auto_increment",
			"source_value": source,
			"target_value": target,
		},
	}, true
}
