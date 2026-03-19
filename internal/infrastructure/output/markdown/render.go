// Package markdown renders audit results as human-readable Markdown.
// input: internal report results from the audit application flow
// output: deterministic Markdown bytes for CLI and agent consumption
// pos: infrastructure output adapter for the default human-oriented renderer
// note: if this file changes, update this header and module README.md.
package markdown

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
)

// Render formats an audit result into Markdown.
func Render(result report.Result) ([]byte, error) {
	var builder strings.Builder

	builder.WriteString("# DeltaScope Audit Result\n\n")
	builder.WriteString(fmt.Sprintf("Verdict: `%s`\n\n", result.Verdict))
	builder.WriteString(fmt.Sprintf("- Statements: %d\n", result.Summary.Statements))
	builder.WriteString(fmt.Sprintf("- Blockers: %d\n", result.Summary.Blockers))
	builder.WriteString(fmt.Sprintf("- Warnings: %d\n", result.Summary.Warnings))
	builder.WriteString(fmt.Sprintf("- Notices: %d\n\n", result.Summary.Notices))

	for _, statement := range result.Statements {
		builder.WriteString(fmt.Sprintf("## Statement %d\n\n", statement.Index+1))
		builder.WriteString(fmt.Sprintf("- Kind: `%s`\n", statement.Kind))
		if statement.NormalizedSQL != "" {
			builder.WriteString(fmt.Sprintf("- SQL: `%s`\n", statement.NormalizedSQL))
		} else if statement.RawSQL != "" {
			builder.WriteString(fmt.Sprintf("- SQL: `%s`\n", statement.RawSQL))
		}
		builder.WriteString("\n")

		if len(statement.Findings) == 0 {
			builder.WriteString("No findings.\n\n")
			continue
		}

		builder.WriteString("### Findings\n\n")
		for _, finding := range statement.Findings {
			writeFinding(&builder, finding)
		}
		builder.WriteString("\n")
	}

	if len(result.GlobalFindings) > 0 {
		builder.WriteString("## Global Findings\n\n")
		for _, finding := range result.GlobalFindings {
			writeFinding(&builder, finding)
		}
	}

	return []byte(builder.String()), nil
}

func writeFinding(builder *strings.Builder, finding rule.Finding) {
	builder.WriteString(fmt.Sprintf("- [%s] `%s`: %s\n", finding.Level, finding.RuleID, finding.Message))
	if finding.Suggestion != "" {
		builder.WriteString(fmt.Sprintf("  Suggestion: %s\n", finding.Suggestion))
	}
	if finding.StatementKind != "" {
		builder.WriteString(fmt.Sprintf("  Statement kind: `%s`\n", finding.StatementKind))
	}
	if len(finding.Metadata) > 0 {
		builder.WriteString("  Metadata:\n")
		keys := make([]string, 0, len(finding.Metadata))
		for key := range finding.Metadata {
			keys = append(keys, key)
		}
		slices.Sort(keys)
		for _, key := range keys {
			value := finding.Metadata[key]
			builder.WriteString(fmt.Sprintf("  - `%s`: `%v`\n", key, value))
		}
	}
}
