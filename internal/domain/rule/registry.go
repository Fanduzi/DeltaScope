// Package rule defines rule registration and evaluation infrastructure.
// input: domain statements and registered statement/global rule implementations
// output: deterministic finding collection for the audit engine
// pos: domain rule registry and execution coordination
// note: if this file changes, update this header and module README.md.
package rule

import (
	"context"
	"fmt"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// StatementRule evaluates one statement at a time.
type StatementRule interface {
	ID() string
	AppliesTo(statement spec.Statement) bool
	Evaluate(ctx context.Context, statement spec.Statement) ([]Finding, error)
}

// GlobalRule evaluates the full statement batch.
type GlobalRule interface {
	ID() string
	EvaluateAll(ctx context.Context, statements []spec.Statement) ([]Finding, error)
}

// Registry stores rule registrations in deterministic order.
type Registry struct {
	statementRules []StatementRule
	globalRules    []GlobalRule
	statementIDs   map[string]struct{}
	globalIDs      map[string]struct{}
}

// NewRegistry creates an empty rule registry.
func NewRegistry() *Registry {
	return &Registry{
		statementRules: make([]StatementRule, 0),
		globalRules:    make([]GlobalRule, 0),
		statementIDs:   make(map[string]struct{}),
		globalIDs:      make(map[string]struct{}),
	}
}

// RegisterStatement appends a statement rule to the registry.
func (r *Registry) RegisterStatement(rule StatementRule) error {
	id := rule.ID()
	if id == "" {
		return ErrEmptyRuleID
	}
	if _, exists := r.statementIDs[id]; exists {
		return fmt.Errorf("%w: %s", ErrDuplicateRuleID, id)
	}
	r.statementRules = append(r.statementRules, rule)
	r.statementIDs[id] = struct{}{}
	return nil
}

// RegisterGlobal appends a global rule to the registry.
func (r *Registry) RegisterGlobal(rule GlobalRule) error {
	id := rule.ID()
	if id == "" {
		return ErrEmptyRuleID
	}
	if _, exists := r.globalIDs[id]; exists {
		return fmt.Errorf("%w: %s", ErrDuplicateRuleID, id)
	}
	r.globalRules = append(r.globalRules, rule)
	r.globalIDs[id] = struct{}{}
	return nil
}

// EvaluateStatement applies all matching statement rules in registration order.
func (r *Registry) EvaluateStatement(ctx context.Context, statement spec.Statement) ([]Finding, error) {
	findings := make([]Finding, 0, 16)
	for _, registered := range r.statementRules {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("evaluation cancelled: %w", err)
		}
		if !registered.AppliesTo(statement) {
			continue
		}
		ruleFindings, err := registered.Evaluate(ctx, statement)
		if err != nil {
			return nil, err
		}
		ruleFindings, err = normalizeFindingRuleIDs(registered.ID(), ruleFindings)
		if err != nil {
			return nil, err
		}
		findings = append(findings, ruleFindings...)
	}
	return findings, nil
}

// EvaluateGlobal applies global rules in registration order.
func (r *Registry) EvaluateGlobal(ctx context.Context, statements []spec.Statement) ([]Finding, error) {
	findings := make([]Finding, 0, 8)
	for _, registered := range r.globalRules {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("evaluation cancelled: %w", err)
		}
		ruleFindings, err := registered.EvaluateAll(ctx, statements)
		if err != nil {
			return nil, err
		}
		ruleFindings, err = normalizeFindingRuleIDs(registered.ID(), ruleFindings)
		if err != nil {
			return nil, err
		}
		findings = append(findings, ruleFindings...)
	}
	return findings, nil
}

// LoadedStatementRuleCount returns the number of registered statement rules.
func (r *Registry) LoadedStatementRuleCount() int {
	return len(r.statementRules)
}

// EvaluateStatementDetailed applies all statement rules and returns findings alongside
// skipped-rule metadata for rules that did not apply with an inferable reason.
func (r *Registry) EvaluateStatementDetailed(ctx context.Context, statement spec.Statement) (StatementEvaluation, error) {
	var eval StatementEvaluation
	for _, registered := range r.statementRules {
		if err := ctx.Err(); err != nil {
			return StatementEvaluation{}, fmt.Errorf("evaluation cancelled: %w", err)
		}
		if !registered.AppliesTo(statement) {
			if reason := inferSkipReason(registered.ID(), statement); reason != "" {
				eval.Skipped = append(eval.Skipped, SkippedRule{
					RuleID: registered.ID(),
					Reason: reason,
				})
			}
			continue
		}
		ruleFindings, err := registered.Evaluate(ctx, statement)
		if err != nil {
			return StatementEvaluation{}, err
		}
		ruleFindings, err = normalizeFindingRuleIDs(registered.ID(), ruleFindings)
		if err != nil {
			return StatementEvaluation{}, err
		}
		eval.Findings = append(eval.Findings, ruleFindings...)
		eval.AppliedRuleIDs = append(eval.AppliedRuleIDs, registered.ID())
	}
	return eval, nil
}

// inferSkipReason returns a SkipReason when the reason a rule did not apply can be
// reliably inferred. Rules where the reason is uncertain are not reported.
func inferSkipReason(ruleID string, statement spec.Statement) SkipReason {
	if strings.HasPrefix(ruleID, "ddl.pg.") && statement.Dialect != spec.DialectPostgreSQL {
		return SkipReasonDialectMismatch
	}
	return ""
}

func normalizeFindingRuleIDs(ruleID string, findings []Finding) ([]Finding, error) {
	for i := range findings {
		switch findings[i].RuleID {
		case "":
			findings[i].RuleID = ruleID
		case ruleID:
			continue
		default:
			return nil, fmt.Errorf("%w: rule=%s finding=%s", ErrRuleIDMismatch, ruleID, findings[i].RuleID)
		}
	}
	return findings, nil
}
