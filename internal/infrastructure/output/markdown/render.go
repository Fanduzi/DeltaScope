// Package markdown renders audit results as human-readable Markdown.
// input: internal report results from the audit application flow
// output: deterministic Markdown bytes for CLI and agent consumption
// pos: infrastructure output adapter for the default human-oriented renderer
// note: if this file changes, update this header and module README.md.
package markdown

import (
	"fmt"
	"math"
	"strconv"
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
	writeAggregateExplanation(&builder, 2, "Result Explanation", result.Explanation)

	for _, statement := range result.Statements {
		builder.WriteString(fmt.Sprintf("## Statement %d\n\n", statement.Index+1))
		builder.WriteString(fmt.Sprintf("- Kind: `%s`\n", statement.Kind))
		if statement.NormalizedSQL != "" {
			builder.WriteString(fmt.Sprintf("- SQL: %s\n", inlineCodeSpan(statement.NormalizedSQL)))
		} else if statement.RawSQL != "" {
			builder.WriteString(fmt.Sprintf("- SQL: %s\n", inlineCodeSpan(statement.RawSQL)))
		}
		builder.WriteString("\n")
		writeAggregateExplanation(&builder, 3, "Explanation", statement.Explanation)
		writeImpact(&builder, statement.Impact)

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

func writeImpact(builder *strings.Builder, impact *report.Impact) {
	if impact == nil {
		return
	}

	builder.WriteString("### Impact\n\n")
	if impact.EstimatedRows != nil {
		builder.WriteString(fmt.Sprintf("- `estimated_rows`: `%d`\n", *impact.EstimatedRows))
	}
	if impact.EstimatedRatio != nil {
		builder.WriteString(fmt.Sprintf("- `estimated_ratio`: `%s`\n", formatImpactRatio(*impact.EstimatedRatio)))
	}
	if impact.RiskLevel != "" {
		builder.WriteString(fmt.Sprintf("- `risk_level`: `%s`\n", impact.RiskLevel))
	}
	if impact.Confidence != "" {
		builder.WriteString(fmt.Sprintf("- `confidence`: `%s`\n", impact.Confidence))
	}
	if impact.Source != "" {
		builder.WriteString(fmt.Sprintf("- `source`: `%s`\n", impact.Source))
	}
	for _, code := range impact.ReasonCodes {
		builder.WriteString(fmt.Sprintf("- `reason_code`: `%s`\n", code))
	}
	builder.WriteString("\n")
}

func formatImpactRatio(ratio float64) string {
	if ratio == 0 {
		return "0.0000"
	}
	if math.Abs(ratio) < 0.0001 {
		return strconv.FormatFloat(ratio, 'g', -1, 64)
	}
	return fmt.Sprintf("%.4f", ratio)
}

func inlineCodeSpan(value string) string {
	delimiterWidth := longestBacktickRun(value) + 1
	delimiter := strings.Repeat("`", delimiterWidth)
	if strings.HasPrefix(value, "`") || strings.HasSuffix(value, "`") {
		return delimiter + " " + value + " " + delimiter
	}
	return delimiter + value + delimiter
}

func writeAggregateExplanation(builder *strings.Builder, level int, heading string, explanation *report.Explanation) {
	if explanation == nil {
		return
	}
	builder.WriteString(fmt.Sprintf("%s %s\n\n", strings.Repeat("#", level), heading))
	if explanation.Summary != "" {
		builder.WriteString(explanation.Summary)
		builder.WriteString("\n")
	}
	for _, reason := range explanation.Reasons {
		builder.WriteString(fmt.Sprintf("- %s\n", reason))
	}
	builder.WriteString("\n")
}

func longestBacktickRun(value string) int {
	longest := 0
	current := 0
	for _, r := range value {
		if r == '`' {
			current++
			if current > longest {
				longest = current
			}
			continue
		}
		current = 0
	}
	return longest
}

func writeFinding(builder *strings.Builder, finding rule.Finding) {
	builder.WriteString(fmt.Sprintf("- [%s] `%s`: %s\n", finding.Level, finding.RuleID, finding.Message))
	if finding.Explanation != nil {
		if finding.Explanation.Why != "" {
			builder.WriteString(fmt.Sprintf("  Why: %s\n", finding.Explanation.Why))
		}
		if finding.Explanation.Risk != "" {
			builder.WriteString(fmt.Sprintf("  Risk: %s\n", finding.Explanation.Risk))
		}
		suggestion := finding.Explanation.Suggestion
		if suggestion == "" {
			suggestion = finding.Suggestion
		}
		if suggestion != "" {
			builder.WriteString(fmt.Sprintf("  Suggestion: %s\n", suggestion))
		}
		if finding.Explanation.Metadata != nil {
			if finding.Explanation.Metadata.Status != "" {
				builder.WriteString(fmt.Sprintf("  Metadata status: `%s`\n", finding.Explanation.Metadata.Status))
			}
			if finding.Explanation.Metadata.Note != "" {
				builder.WriteString(fmt.Sprintf("  Metadata note: %s\n", finding.Explanation.Metadata.Note))
			}
		}
	} else if finding.Suggestion != "" {
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
