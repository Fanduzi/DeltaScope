// Package ddl defines Tier-1 DDL rules.
package ddl

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type forbiddenDDLOperationRule struct {
	ruleID    string
	operation spec.DDLOperation
	label     string
	level     rule.Level
	forbid    bool
}

func newForbiddenDDLOperationRule(ruleID string, operation spec.DDLOperation, label string, fallbackLevel rule.Level, cfg policy.RulePolicy) (rule.StatementRule, error) {
	forbid, err := boolParam(ruleID, cfg, "forbid", true)
	if err != nil {
		return nil, err
	}
	return forbiddenDDLOperationRule{
		ruleID:    ruleID,
		operation: operation,
		label:     label,
		level:     configuredLevel(cfg, fallbackLevel),
		forbid:    forbid,
	}, nil
}

func (r forbiddenDDLOperationRule) ID() string { return r.ruleID }

func (r forbiddenDDLOperationRule) AppliesTo(statement spec.Statement) bool {
	return r.forbid &&
		statement.Kind == spec.KindDDL &&
		statement.DDL != nil &&
		statement.DDL.Operation == r.operation &&
		statement.DDL.Table != nil
}

func (r forbiddenDDLOperationRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	return []rule.Finding{{
		RuleID:     r.ruleID,
		Level:      r.level,
		Message:    fmt.Sprintf("%s is forbidden for %q", r.label, statement.DDL.Table.Name),
		Suggestion: fmt.Sprintf("avoid %s in this change or relax the policy intentionally", r.label),
		Metadata: map[string]any{
			"table":     statement.DDL.Table.Name,
			"operation": string(statement.DDL.Operation),
		},
	}}, nil
}

type tableOperationExistenceRule struct {
	ruleID    string
	operation spec.DDLOperation
	label     string
	level     rule.Level
}

func newTableOperationExistenceRule(ruleID string, operation spec.DDLOperation, label string, fallbackLevel rule.Level, cfg policy.RulePolicy) (rule.StatementRule, error) {
	return tableOperationExistenceRule{
		ruleID:    ruleID,
		operation: operation,
		label:     label,
		level:     configuredLevel(cfg, fallbackLevel),
	}, nil
}

func (r tableOperationExistenceRule) ID() string { return r.ruleID }

func (r tableOperationExistenceRule) AppliesTo(statement spec.Statement) bool {
	return statement.Kind == spec.KindDDL &&
		statement.DDL != nil &&
		statement.DDL.Operation == r.operation &&
		statement.DDL.Table != nil
}

func (r tableOperationExistenceRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	// DROP ... IF EXISTS arms the conditional form: a proven-absent member
	// becomes a native no-op note, never an existence violation. Members
	// the batch cannot vouch stay a gap, not a fabricated refusal.
	ifExistsArmed := statement.DDL.Options["if_exists"] == "true"
	bindings := boundTargetBindings(statement)
	if len(bindings) == 0 {
		return nil, nil
	}
	var findings []rule.Finding
	seen := make(map[string]struct{})
	for _, binding := range bindings {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if binding.Ambiguous || binding.Snapshot == nil || binding.Snapshot.Exists || ifExistsArmed {
			continue
		}
		identity := boundIdentityKey(statement, binding)
		if _, duplicate := seen[identity]; duplicate {
			continue
		}
		seen[identity] = struct{}{}
		findings = append(findings, rule.Finding{
			RuleID:     r.ruleID,
			Level:      r.level,
			Message:    fmt.Sprintf("%s target %q does not exist in the target schema", r.label, binding.Table.Name),
			Suggestion: "refresh metadata, create the object first, or disable metadata mode for offline-only auditing",
			Metadata: map[string]any{
				"table":     binding.Table.Name,
				"operation": string(statement.DDL.Operation),
				"exists":    false,
			},
		})
	}
	return findings, nil
}

func (r tableOperationExistenceRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if r.ruleID != ruleIDTableDropExistsRequire || r.operation != spec.DDLOperationDropTable || !orderedGapDialect(statement.Dialect) || !r.AppliesTo(statement) {
		return nil
	}
	bindings := boundTargetBindings(statement)
	if len(bindings) == 0 {
		// No bound answer at all — no metadata seam ran — keeps the legacy
		// single-target unknown-state gap the frozen contract expects.
		if target := primaryTarget(statement); strings.TrimSpace(target.Name) != "" {
			return []rule.EvidenceGap{{
				ReasonCode:    gapReasonUnknownTableState,
				RequiredFacts: []string{"target_table.existence"},
			}}
		}
		return nil
	}
	var gaps []rule.EvidenceGap
	for _, binding := range bindings {
		if binding.Ambiguous || binding.Snapshot == nil {
			gaps = append(gaps, rule.EvidenceGap{
				ReasonCode:    gapReasonUnknownTableState,
				RequiredFacts: []string{"target_table.existence"},
			})
		}
	}
	return gaps
}

type adaptiveHashLifecycleRule struct {
	ruleID    string
	operation spec.DDLOperation
	label     string
	level     rule.Level
}

func newAdaptiveHashLifecycleRule(ruleID string, operation spec.DDLOperation, label string, fallbackLevel rule.Level, cfg policy.RulePolicy) (rule.StatementRule, error) {
	return adaptiveHashLifecycleRule{
		ruleID:    ruleID,
		operation: operation,
		label:     label,
		level:     configuredLevel(cfg, fallbackLevel),
	}, nil
}

func (r adaptiveHashLifecycleRule) ID() string { return r.ruleID }

func (r adaptiveHashLifecycleRule) AppliesTo(statement spec.Statement) bool {
	return statement.Kind == spec.KindDDL &&
		statement.DDL != nil &&
		statement.DDL.Operation == r.operation &&
		statement.DDL.Table != nil
}

func (r adaptiveHashLifecycleRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) || statement.Metadata == nil || statement.Metadata.Instance == nil || !statement.Metadata.Instance.InnoDBAdaptiveHashEnabled {
		return nil, nil
	}
	return []rule.Finding{{
		RuleID:     r.ruleID,
		Level:      r.level,
		Message:    fmt.Sprintf("%s on %q may contend with adaptive hash index activity", r.label, statement.DDL.Table.Name),
		Suggestion: "review operational timing and consider disabling the warning only if this environment is known to be safe",
		Metadata: map[string]any{
			"table":                        statement.DDL.Table.Name,
			"operation":                    string(statement.DDL.Operation),
			"innodb_adaptive_hash_enabled": true,
		},
	}}, nil
}

type tableRowCountRiskRule struct {
	ruleID    string
	operation spec.DDLOperation
	label     string
	limit     int
	level     rule.Level
}

func newTableRowCountRiskRule(ruleID string, operation spec.DDLOperation, label string, fallbackLevel rule.Level, cfg policy.RulePolicy) (rule.StatementRule, error) {
	limit, err := boundedIntParam(ruleID, cfg, "limit", 100, 1)
	if err != nil {
		return nil, err
	}
	return tableRowCountRiskRule{
		ruleID:    ruleID,
		operation: operation,
		label:     label,
		limit:     limit,
		level:     configuredLevel(cfg, fallbackLevel),
	}, nil
}

func (r tableRowCountRiskRule) ID() string { return r.ruleID }

func (r tableRowCountRiskRule) AppliesTo(statement spec.Statement) bool {
	return statement.Kind == spec.KindDDL &&
		statement.DDL != nil &&
		statement.DDL.Operation == r.operation &&
		statement.DDL.Table != nil
}

func (r tableRowCountRiskRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	snapshot, ok := targetTableSnapshot(statement)
	if !ok || !snapshot.Exists {
		return nil, nil
	}
	rowCountText := snapshot.Options["table_rows"]
	if rowCountText == "" {
		return nil, nil
	}
	rowCount, err := strconv.Atoi(rowCountText)
	if err != nil {
		return nil, fmt.Errorf("parse table_rows for %s: %w", statement.DDL.Table.Name, err)
	}
	if rowCount <= r.limit {
		return nil, nil
	}
	return []rule.Finding{{
		RuleID:     r.ruleID,
		Level:      r.level,
		Message:    fmt.Sprintf("%s on %q exceeds the configured table-row risk threshold", r.label, statement.DDL.Table.Name),
		Suggestion: fmt.Sprintf("split cleanup work first or only proceed after manual review when row count is above %d", r.limit),
		Metadata: map[string]any{
			"table":      statement.DDL.Table.Name,
			"operation":  string(statement.DDL.Operation),
			"table_rows": rowCount,
			"limit":      r.limit,
		},
	}}, nil
}
