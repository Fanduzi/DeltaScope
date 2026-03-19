// Package rule defines domain findings and rule-facing severity types.
// input: rule evaluation details and source-location metadata
// output: normalized findings consumed by reporting and future rule engines
// pos: domain rule vocabulary shared across audit evaluation
// note: if this file changes, update this header and module README.md.
package rule

// Level describes how severe a finding is.
type Level string

const (
	LevelBlocker Level = "blocker"
	LevelWarning Level = "warning"
	LevelNotice  Level = "notice"
)

// Finding is the domain result produced by a rule.
type Finding struct {
	RuleID         string         `json:"rule_id"`
	Level          Level          `json:"level"`
	Message        string         `json:"message"`
	StatementIndex int            `json:"statement_index,omitempty"`
	StatementKind  string         `json:"statement_kind,omitempty"`
	Location       *Location      `json:"location,omitempty"`
	Suggestion     string         `json:"suggestion,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

// Location identifies a source span when available.
type Location struct {
	Line   int `json:"line,omitempty"`
	Column int `json:"column,omitempty"`
}
