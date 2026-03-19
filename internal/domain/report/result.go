// Package report defines audit results, summaries, and verdict aggregation.
// input: statement findings and global findings from audit evaluation
// output: normalized audit results for CLI, APIs, and future integrations
// pos: domain reporting model and verdict aggregation logic
// note: if this file changes, update this header and module README.md.
package report

import "github.com/Fanduzi/DeltaScope/internal/domain/rule"

// Verdict describes the final audit outcome.
type Verdict string

const (
	VerdictPass   Verdict = "pass"
	VerdictReview Verdict = "review"
	VerdictReject Verdict = "reject"
)

// StatementResult stores findings for a single SQL statement.
type StatementResult struct {
	Index         int            `json:"index"`
	Kind          string         `json:"kind"`
	RawSQL        string         `json:"raw_sql,omitempty"`
	NormalizedSQL string         `json:"normalized_sql,omitempty"`
	Findings      []rule.Finding `json:"findings,omitempty"`
}

// Summary captures high-level counts for the full audit result.
type Summary struct {
	Statements int `json:"statements"`
	Blockers   int `json:"blockers"`
	Warnings   int `json:"warnings"`
	Notices    int `json:"notices"`
}

// Result is the aggregated audit output.
type Result struct {
	Verdict    Verdict           `json:"verdict"`
	Summary    Summary           `json:"summary"`
	Statements []StatementResult `json:"statements,omitempty"`
	Findings   []rule.Finding    `json:"findings,omitempty"`
}

// Aggregate builds a final Result from statement and global findings.
func Aggregate(statements []StatementResult, findings []rule.Finding) Result {
	result := Result{
		Statements: statements,
		Findings:   findings,
		Summary: Summary{
			Statements: len(statements),
		},
	}

	allFindings := make([]rule.Finding, 0, len(findings))
	for _, stmt := range statements {
		allFindings = append(allFindings, stmt.Findings...)
	}
	allFindings = append(allFindings, findings...)

	for _, finding := range allFindings {
		switch finding.Level {
		case rule.LevelBlocker:
			result.Summary.Blockers++
		case rule.LevelWarning:
			result.Summary.Warnings++
		case rule.LevelNotice:
			result.Summary.Notices++
		}
	}

	switch {
	case result.Summary.Blockers > 0:
		result.Verdict = VerdictReject
	case result.Summary.Warnings > 0:
		result.Verdict = VerdictReview
	default:
		result.Verdict = VerdictPass
	}

	return result
}
