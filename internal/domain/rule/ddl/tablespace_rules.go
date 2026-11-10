// Package ddl: T25 table-level InnoDB tablespace alter rules.
//
// ALTER TABLE ... DISCARD TABLESPACE is governed by a forbid rule (default
// blocker): the authored action detaches the table's data file outright.
// IMPORT TABLESPACE is governed by a warning: it attaches an external data
// file whose provenance the audit cannot see. Both share an applicability
// rule — the InnoDB-only contract — which emits a proven native refusal on
// a non-InnoDB or partitioned target and owes the engine/partitioned facts
// through EvidenceGaps when unobserved. TiDB refuses the actions at the
// vendor boundary, so every rule here is TiDB-gated and the vendor aspect
// in coverage answers the statement there.
package ddl

import (
	"context"
	"fmt"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func newDiscardTablespaceForbidRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	inner, err := newPartitionForbidRule(ruleIDAlterDiscardTablespaceForbid,
		"discard_tablespace", "discard tablespace", cfg)
	if err != nil {
		return nil, err
	}
	return tidbVendorGatedRule{StatementRule: inner, action: "discard_tablespace"}, nil
}

func newImportTablespaceWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	inner, err := newPartitionTopologyWarnRule(ruleIDAlterImportTablespaceWarn,
		"import_tablespace", "import tablespace",
		"attaches an external InnoDB data file to the table — the file's provenance, version, and contents are outside the audit's sight",
		"verify the source tablespace export and the table definition match before importing; the attached file bypasses normal row validation", rule.LevelWarning, cfg)
	if err != nil {
		return nil, err
	}
	return tidbVendorGatedRule{StatementRule: inner, action: "import_tablespace"}, nil
}

const (
	tablespaceReasonEngineInapplicable = "tablespace_engine_inapplicable"
	tablespaceReasonPartitionedTarget  = "tablespace_partitioned_target"
)

// alterTablespaceApplicabilityRule binds the InnoDB-only contract to the proven
// target state:
//   - a proven non-InnoDB engine refuses natively (the engine-option
//     refusal family — 1031, same family probed for the partition ops);
//   - a proven partitioned target refuses natively — the whole-table form
//     cannot address member tablespaces (use the partition-level variant);
//   - an unproven engine or partitioned state never fabricates a verdict —
//     the missing fact is owed through EvidenceGaps instead.
type alterTablespaceApplicabilityRule struct {
	ruleID string
	action string
	label  string
	level  rule.Level
}

func newAlterTablespaceApplicabilityRule(ruleID, action, label string, cfg policy.RulePolicy) (rule.StatementRule, error) {
	return tidbVendorGatedRule{
		StatementRule: alterTablespaceApplicabilityRule{
			ruleID: ruleID,
			action: action,
			label:  label,
			level:  configuredLevel(cfg, rule.LevelBlocker),
		},
		action: action,
	}, nil
}

func newDiscardTablespaceApplicabilityRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newAlterTablespaceApplicabilityRule(ruleIDAlterDiscardTablespaceApplicability,
		"discard_tablespace", "discard tablespace", cfg)
}

func newImportTablespaceApplicabilityRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newAlterTablespaceApplicabilityRule(ruleIDAlterImportTablespaceApplicability,
		"import_tablespace", "import tablespace", cfg)
}

func (r alterTablespaceApplicabilityRule) ID() string { return r.ruleID }

func (r alterTablespaceApplicabilityRule) AppliesTo(statement spec.Statement) bool {
	return appliesToAlterTable(statement) &&
		len(matchingAlterActions(statement, r.action)) > 0
}

// missingAlterTablespaceFacts reports the target facts the rule consumed but the
// projection never answered. The table's own existence stays with the
// shared unknown-table-state reporter; this rule only owes the facts it
// reads itself.
func (r alterTablespaceApplicabilityRule) missingAlterTablespaceFacts(statement spec.Statement) []string {
	snapshot, ok := targetTableSnapshot(statement)
	if !ok || snapshot == nil || !snapshot.Exists {
		return nil
	}
	missing := make([]string, 0, 2)
	if _, ok := provenEngineOption(statement); !ok {
		missing = append(missing, "target_table.engine")
	}
	if snapshot.Partitioned == nil {
		missing = append(missing, "target_table.partitioned")
	}
	return missing
}

func (r alterTablespaceApplicabilityRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(statement) {
		return nil
	}
	missing := r.missingAlterTablespaceFacts(statement)
	if len(missing) == 0 {
		return nil
	}
	return []rule.EvidenceGap{{
		ReasonCode:    gapReasonPartitionFacts,
		RequiredFacts: missing,
	}}
}

func (r alterTablespaceApplicabilityRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	snapshot, ok := targetTableSnapshot(statement)
	if !ok || snapshot == nil || !snapshot.Exists {
		return nil, ctx.Err()
	}
	findings := make([]rule.Finding, 0)
	for _, alter := range matchingAlterActions(statement, r.action) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		switch {
		case snapshot.Partitioned != nil && *snapshot.Partitioned:
			metadata := partitionMetadata(statement, alter, map[string]any{
				"reason_code": tablespaceReasonPartitionedTarget,
			})
			findings = append(findings, rule.Finding{
				Level:      r.level,
				Message:    fmt.Sprintf("ALTER TABLE %s cannot apply to a partitioned table — use the partition-level tablespace form", r.label),
				Suggestion: "address the tablespace members with the partition-level tablespace form instead",
				Metadata:   metadata,
			})
		case func() bool {
			engine, ok := provenEngineOption(statement)
			return ok && !strings.EqualFold(engine, "innodb")
		}():
			engine, _ := provenEngineOption(statement)
			metadata := partitionMetadata(statement, alter, map[string]any{
				"reason_code":  tablespaceReasonEngineInapplicable,
				"native_error": "1031",
				"engine":       engine,
			})
			findings = append(findings, rule.Finding{
				Level:      r.level,
				Message:    fmt.Sprintf("ALTER TABLE %s is not supported on the %s storage engine (native 1031)", r.label, engine),
				Suggestion: "the whole-table tablespace form is InnoDB-only — verify the engine before shipping",
				Metadata:   metadata,
			})
		}
	}
	return findings, nil
}
