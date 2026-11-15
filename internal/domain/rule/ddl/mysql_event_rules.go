package ddl

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// mysql_event_rules.go — T30 event scheduled-execution rule (issue
// #109): `ddl.event.enabled_execution.warn` fires whenever a statement
// provably leaves an ENABLED event scheduled to run — CREATE defaults
// to enabled, explicit ENABLE confirms it, and DISABLE ON SLAVE still
// executes on the primary. An ALTER that never authors a status leaves
// the premise unprovable and owes the `missing_event_state` evidence
// gap instead of guessing. The finding names schedule shape when
// authored (at/every, repeating) but never predicts a runtime outcome.

const gapReasonMissingEventState = "missing_event_state"

type eventEnabledExecutionRule struct {
	ruleID string
	level  rule.Level
}

func newEventEnabledExecutionWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return eventEnabledExecutionRule{
		ruleID: ruleIDEventEnabledExecutionWarn,
		level:  configuredLevel(cfg, rule.LevelWarning),
	}, nil
}

func (r eventEnabledExecutionRule) ID() string { return r.ruleID }

func (r eventEnabledExecutionRule) AppliesTo(statement spec.Statement) bool {
	if statement.Dialect != spec.DialectMySQL {
		return false
	}
	if statement.Kind != spec.KindDDL || statement.DDL == nil || statement.DDL.Event == nil {
		return false
	}
	return statement.DDL.Operation == spec.DDLOperationCreateEvent || statement.DDL.Operation == spec.DDLOperationAlterEvent
}

func (r eventEnabledExecutionRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	decl := statement.DDL.Event
	if !decl.EffectiveEnabled() {
		return nil, nil
	}
	meta := map[string]any{
		"event":     decl.Name.Name,
		"operation": string(statement.DDL.Operation),
	}
	if s := decl.Schedule; s != nil {
		meta["schedule_kind"] = string(s.Kind)
		meta["repeating"] = s.Repeating()
	}
	if decl.StatusSet && decl.Status == spec.EventStatusDisableOnSlave {
		meta["status"] = string(decl.Status)
	}
	return []rule.Finding{{
		Level:      r.level,
		Message:    fmt.Sprintf("event %q is left enabled and its body will run on schedule", decl.Name.Name),
		Suggestion: "verify the scheduled effects are intended; a DISABLED event still audits but does not execute",
		Metadata:   meta,
	}}, nil
}

// EvidenceGaps owes the enabled-state premise on ALTER statements that
// author no status clause: whether the altered event still executes is
// a catalog fact the statement itself cannot prove (T30).
func (r eventEnabledExecutionRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(statement) {
		return nil
	}
	decl := statement.DDL.Event
	if statement.DDL.Operation != spec.DDLOperationAlterEvent || decl.StatusSet {
		return nil
	}
	return []rule.EvidenceGap{{
		ReasonCode:    gapReasonMissingEventState,
		RequiredFacts: []string{"event.enabled_state"},
	}}
}
