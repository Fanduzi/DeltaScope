// Package report defines audit results, summaries, and verdict aggregation.
// input: statement findings and global findings from audit evaluation
// output: normalized audit results for CLI, APIs, and future integrations
// pos: domain reporting model and verdict aggregation logic
// note: if this file changes, update this header and module README.md.
package report

import (
	"fmt"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
)

// Verdict describes the final audit outcome.
type Verdict string

const (
	VerdictPass   Verdict = "pass"
	VerdictReview Verdict = "review"
	VerdictReject Verdict = "reject"
)

// Explanation captures additive human- and machine-readable context for a result.
type Explanation struct {
	Summary string   `json:"summary,omitempty"`
	Reasons []string `json:"reasons,omitempty"`
}

// StatementResult stores findings for a single SQL statement.
type StatementResult struct {
	Index         int            `json:"index"`
	Kind          string         `json:"kind"`
	RawSQL        string         `json:"raw_sql,omitempty"`
	NormalizedSQL string         `json:"normalized_sql,omitempty"`
	Findings      []rule.Finding `json:"findings,omitempty"`
	Explanation   *Explanation   `json:"explanation,omitempty"`
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
	Verdict        Verdict           `json:"verdict"`
	Summary        Summary           `json:"summary"`
	Statements     []StatementResult `json:"statements,omitempty"`
	GlobalFindings []rule.Finding    `json:"global_findings,omitempty"`
	Explanation    *Explanation      `json:"explanation,omitempty"`
}

// Aggregate builds a final Result from statement and global findings.
func Aggregate(statements []StatementResult, findings []rule.Finding) Result {
	result := Result{
		Statements:     append([]StatementResult(nil), statements...),
		GlobalFindings: findings,
		Summary: Summary{
			Statements: len(statements),
		},
	}

	allFindings := make([]rule.Finding, 0, len(findings))
	for i := range result.Statements {
		allFindings = append(allFindings, result.Statements[i].Findings...)
		if result.Statements[i].Explanation == nil {
			result.Statements[i].Explanation = buildStatementExplanation(result.Statements[i])
		}
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

	result.Explanation = buildResultExplanation(result)
	return result
}

func buildStatementExplanation(statement StatementResult) *Explanation {
	if len(statement.Findings) == 0 {
		return nil
	}
	return &Explanation{
		Summary: fmt.Sprintf("Statement %d has %d finding(s)", statement.Index+1, len(statement.Findings)),
		Reasons: collectFindingMessages(statement.Findings),
	}
}

func buildResultExplanation(result Result) *Explanation {
	findings := make([]rule.Finding, 0, len(result.GlobalFindings))
	for _, statement := range result.Statements {
		findings = append(findings, statement.Findings...)
	}
	findings = append(findings, result.GlobalFindings...)
	if len(findings) == 0 {
		return nil
	}
	return &Explanation{
		Summary: fmt.Sprintf("Audit produced %d finding(s) across %d statement(s)", len(findings), result.Summary.Statements),
		Reasons: collectFindingMessages(findings),
	}
}

func collectFindingMessages(findings []rule.Finding) []string {
	if len(findings) == 0 {
		return nil
	}
	reasons := make([]string, 0, len(findings))
	for _, finding := range findings {
		message := strings.TrimSpace(finding.Message)
		if message == "" {
			continue
		}
		reasons = append(reasons, message)
	}
	if len(reasons) == 0 {
		return nil
	}
	return reasons
}
