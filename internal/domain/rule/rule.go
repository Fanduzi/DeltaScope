// Package rule defines domain findings and rule-engine types.
// input: rule evaluation details, source-location metadata, and statement/global rule implementations
// output: normalized findings and registry-facing rule contracts
// pos: domain rule vocabulary and execution contracts shared across audit evaluation
// note: if this file changes, update this header and module README.md.
package rule

import "errors"

// Level describes how severe a finding is.
type Level string

const (
	LevelBlocker Level = "blocker"
	LevelWarning Level = "warning"
	LevelNotice  Level = "notice"
)

// ExplanationMetadata describes how metadata availability affected a finding explanation.
type ExplanationMetadata struct {
	Status string `json:"status,omitempty"`
	Note   string `json:"note,omitempty"`
}

// FindingExplanation captures additive explanation data for one finding.
type FindingExplanation struct {
	Summary    string               `json:"summary,omitempty"`
	Why        string               `json:"why,omitempty"`
	Risk       string               `json:"risk,omitempty"`
	Suggestion string               `json:"suggestion,omitempty"`
	Metadata   *ExplanationMetadata `json:"metadata,omitempty"`
}

// Finding is the domain result produced by a rule.
type Finding struct {
	RuleID         string              `json:"rule_id"`
	Level          Level               `json:"level"`
	Message        string              `json:"message"`
	StatementIndex int                 `json:"statement_index,omitempty"`
	StatementKind  string              `json:"statement_kind,omitempty"`
	Location       *Location           `json:"location,omitempty"`
	Suggestion     string              `json:"suggestion,omitempty"`
	Metadata       map[string]any      `json:"metadata,omitempty"`
	Explanation    *FindingExplanation `json:"explanation,omitempty"`
}

// Location identifies a source span when available.
type Location struct {
	Line   int `json:"line,omitempty"`
	Column int `json:"column,omitempty"`
}

var (
	// ErrEmptyRuleID indicates a rule was registered without an ID.
	ErrEmptyRuleID = errors.New("rule ID must not be empty")
	// ErrDuplicateRuleID indicates a rule ID was registered more than once.
	ErrDuplicateRuleID = errors.New("duplicate rule ID")
	// ErrRuleIDMismatch indicates a rule emitted a finding with a conflicting rule ID.
	ErrRuleIDMismatch = errors.New("finding rule ID does not match registered rule ID")
)
