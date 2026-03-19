// Package rule defines rule registration and evaluation infrastructure.
// input: domain statements and registered statement/global rule implementations
// output: deterministic finding collection for the audit engine
// pos: domain rule registry and execution coordination
// note: if this file changes, update this header and module README.md.
package rule

import "github.com/Fanduzi/DeltaScope/internal/domain/spec"

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
}

// NewRegistry creates an empty rule registry.
func NewRegistry() *Registry {
	return &Registry{
		statementRules: make([]StatementRule, 0),
		globalRules:    make([]GlobalRule, 0),
	}
}

// RegisterStatement appends a statement rule to the registry.
func (r *Registry) RegisterStatement(rule StatementRule) {
	r.statementRules = append(r.statementRules, rule)
}

// RegisterGlobal appends a global rule to the registry.
func (r *Registry) RegisterGlobal(rule GlobalRule) {
	r.globalRules = append(r.globalRules, rule)
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
		findings = append(findings, ruleFindings...)
	}
	return findings, nil
}
