package ddl

import (
	"context"
	"math"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgGenerationRule struct {
	cycleOnly bool
	level     rule.Level
}

func newPGGenerationDeclarationRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgGenerationRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func newPGIdentityCycleRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgGenerationRule{cycleOnly: true, level: configuredLevel(cfg, rule.LevelWarning)}, nil
}

func (r pgGenerationRule) ID() string {
	if r.cycleOnly {
		return ruleIDPGIdentityCycleWarn
	}
	return ruleIDPGGenerationDeclarationValid
}

type pgGenerationColumn struct {
	column spec.Column
	action string
}

func pgGenerationColumns(s spec.Statement) []pgGenerationColumn {
	if s.Dialect != spec.DialectPostgreSQL || s.Kind != spec.KindDDL || s.DDL == nil || s.DDL.Table == nil {
		return nil
	}
	var columns []pgGenerationColumn
	add := func(c spec.Column, action string) {
		if c.PGGeneration != nil || c.IsIdentity || c.GeneratedWhen != "" {
			columns = append(columns, pgGenerationColumn{c, action})
		}
	}
	switch s.DDL.Operation {
	case spec.DDLOperationCreateTable:
		for _, c := range s.DDL.Columns {
			add(c, "create_table")
		}
	case spec.DDLOperationAlterTable:
		for _, a := range s.DDL.Alter {
			if a.Action == "add_column" && a.Column != nil && a.Column.Definition != nil {
				add(*a.Column.Definition, "add_column")
			}
		}
	}
	return columns
}

func (r pgGenerationRule) AppliesTo(s spec.Statement) bool {
	for _, c := range pgGenerationColumns(s) {
		if !r.cycleOnly || c.column.IsIdentity || (c.column.PGGeneration != nil && c.column.PGGeneration.IdentityCount > 0) {
			return true
		}
	}
	return false
}

// pgGenerationAssessment returns one proven refusal or the exact missing fact;
// it never adjudicates expression legality or an existing column's state.
func pgGenerationAssessment(c spec.Column) (reason, missing string, cycle bool) {
	g := c.PGGeneration
	if g == nil || g.IdentityCount+g.StoredCount != 1 ||
		(g.Mode != "a" && g.Mode != "d") || (g.StoredCount == 1 && g.Mode != "a") {
		return "", "column.generation_declaration", false
	}
	if c.HasDefault {
		return "generation_default_conflict", "", false
	}
	if g.StoredCount == 1 {
		return "", "", false
	}
	if g.ExplicitNull {
		return "identity_null_conflict", "", false
	}
	if g.TypeWidth < 0 {
		return "identity_type_inapplicable", "", false
	}
	if g.TypeWidth != 16 && g.TypeWidth != 32 && g.TypeWidth != 64 {
		return "", "column.identity_type", false
	}
	if g.Unmodeled {
		return "", "column.identity_options", false
	}
	values := map[spec.SequenceOptionKind]int64{}
	seen := map[spec.SequenceOptionKind]bool{}
	for _, option := range g.Options {
		key := option.Kind
		switch key {
		case spec.SequenceOptionNoMinValue:
			key = spec.SequenceOptionMinValue
		case spec.SequenceOptionNoMaxValue:
			key = spec.SequenceOptionMaxValue
		case spec.SequenceOptionNoCycle:
			key = spec.SequenceOptionCycle
		}
		if seen[key] {
			return "", "column.identity_options", false
		}
		seen[key] = true
		switch option.Kind {
		case spec.SequenceOptionCycle, spec.SequenceOptionNoCycle, spec.SequenceOptionNoMinValue, spec.SequenceOptionNoMaxValue:
			if option.Value != nil {
				return "", "column.identity_options", false
			}
			cycle = option.Kind == spec.SequenceOptionCycle || cycle
		case spec.SequenceOptionIncrementBy, spec.SequenceOptionStartWith, spec.SequenceOptionMinValue, spec.SequenceOptionMaxValue, spec.SequenceOptionCache:
			if option.Value == nil {
				return "", "column.identity_options", false
			}
			values[option.Kind] = *option.Value
		default:
			return "", "column.identity_options", false
		}
	}
	increment := int64(1)
	if v, ok := values[spec.SequenceOptionIncrementBy]; ok {
		increment = v
	}
	if increment == 0 {
		return "identity_increment_zero", "", false
	}
	if v, ok := values[spec.SequenceOptionCache]; ok && v < 1 {
		return "identity_cache_invalid", "", false
	}
	typeMin, typeMax := int64(math.MinInt64), int64(math.MaxInt64)
	if g.TypeWidth != 64 {
		typeMax = (int64(1) << (g.TypeWidth - 1)) - 1
		typeMin = -typeMax - 1
	}
	min, max := int64(1), typeMax
	if increment < 0 {
		min, max = typeMin, -1
	}
	if v, ok := values[spec.SequenceOptionMinValue]; ok {
		min = v
	}
	if v, ok := values[spec.SequenceOptionMaxValue]; ok {
		max = v
	}
	if min < typeMin || min > typeMax || max < typeMin || max > typeMax || min >= max {
		return "identity_bounds_invalid", "", false
	}
	if start, ok := values[spec.SequenceOptionStartWith]; ok && (start < min || start > max) {
		return "identity_start_out_of_bounds", "", false
	}
	return "", "", cycle
}

func (r pgGenerationRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(s) {
		return nil
	}
	var gaps []rule.EvidenceGap
	seen := map[string]bool{}
	for _, c := range pgGenerationColumns(s) {
		if r.cycleOnly && !c.column.IsIdentity && (c.column.PGGeneration == nil || c.column.PGGeneration.IdentityCount == 0) {
			continue
		}
		_, fact, _ := pgGenerationAssessment(c.column)
		if fact != "" && !seen[fact] {
			seen[fact] = true
			gaps = append(gaps, rule.EvidenceGap{ReasonCode: "unproven_generation_declaration", RequiredFacts: []string{fact}})
		}
	}
	return gaps
}

func (r pgGenerationRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	messages := map[string]string{
		"generation_default_conflict":  "generated/identity column cannot also declare DEFAULT",
		"identity_null_conflict":       "identity column cannot explicitly declare NULL",
		"identity_type_inapplicable":   "identity column requires a supported integer scalar type",
		"identity_increment_zero":      "identity sequence INCREMENT cannot be zero",
		"identity_cache_invalid":       "identity sequence CACHE must be positive",
		"identity_bounds_invalid":      "identity sequence MIN/MAX bounds are invalid for the declared integer type",
		"identity_start_out_of_bounds": "identity sequence START is outside its effective MIN/MAX bounds",
	}
	var findings []rule.Finding
	for _, target := range pgGenerationColumns(s) {
		c := target.column
		reason, missing, cycle := pgGenerationAssessment(c)
		if missing != "" || (r.cycleOnly && (reason != "" || !cycle)) || (!r.cycleOnly && reason == "") {
			continue
		}
		message, suggestion := messages[reason], "correct the conflicting generation declaration or identity sequence options"
		if r.cycleOnly {
			reason, message, suggestion = "identity_cycle", "identity sequence CYCLE can reuse values after reaching its limit", "review value-reuse requirements; use NO CYCLE when wraparound is not intended"
		}
		g := c.PGGeneration
		kind, mode := "stored", "always"
		if g.IdentityCount == 1 {
			kind = "identity"
		}
		if g.Mode == "d" {
			mode = "by_default"
		}
		metadata := map[string]any{"table": s.DDL.Table.Name, "column": c.Name, "action": target.action, "generation": kind, "generation_mode": mode, "reason_code": reason}
		if s.DDL.Table.Schema != "" {
			metadata["schema"] = s.DDL.Table.Schema
		}
		findings = append(findings, rule.Finding{Level: r.level, Message: message, Suggestion: suggestion, Metadata: metadata})
	}
	return findings, nil
}
