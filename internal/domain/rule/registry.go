// Package rule defines rule registration and evaluation infrastructure.
// input: domain statements and registered statement/global rule implementations
// output: deterministic finding collection for the audit engine
// pos: domain rule registry and execution coordination
// note: if this file changes, update this header and module README.md.
package rule

import (
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// StatementRule evaluates one statement at a time.
type StatementRule interface {
	ID() string
	AppliesTo(statement spec.Statement) bool
	Evaluate(statement spec.Statement) ([]Finding, error)
}

// GlobalRule evaluates the full statement batch.
type GlobalRule interface {
	ID() string
	EvaluateAll(statements []spec.Statement) ([]Finding, error)
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
func (r *Registry) EvaluateStatement(statement spec.Statement) ([]Finding, error) {
	findings := make([]Finding, 0)
	for _, registered := range r.statementRules {
		if !registered.AppliesTo(statement) {
			continue
		}
		ruleFindings, err := registered.Evaluate(statement)
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
func (r *Registry) EvaluateGlobal(statements []spec.Statement) ([]Finding, error) {
	findings := make([]Finding, 0)
	for _, registered := range r.globalRules {
		ruleFindings, err := registered.EvaluateAll(statements)
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
