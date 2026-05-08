// Package markdown renders audit results as human-readable Markdown.
// input: internal report results from the audit application flow
// output: deterministic Markdown bytes for CLI and agent consumption
// pos: infrastructure output adapter for the default human-oriented renderer
// note: if this file changes, update this header and module README.md.
package markdown

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/infrastructure/output"
)

// Render formats an audit result into Markdown.
func Render(result report.Result) ([]byte, error) {
	builder := output.GetBuilder()
	defer output.PutBuilder(builder)
	builder.Grow(estimateResultSize(result))

	builder.WriteString("# DeltaScope Audit Result\n\n")
	builder.WriteString("Verdict: `")
	builder.WriteString(string(result.Verdict))
	builder.WriteString("`\n\n- Statements: ")
	builder.WriteString(strconv.Itoa(result.Summary.Statements))
	builder.WriteString("\n- Blockers: ")
	builder.WriteString(strconv.Itoa(result.Summary.Blockers))
	builder.WriteString("\n- Warnings: ")
	builder.WriteString(strconv.Itoa(result.Summary.Warnings))
	builder.WriteString("\n- Notices: ")
	builder.WriteString(strconv.Itoa(result.Summary.Notices))
	builder.WriteString("\n\n")
	writeAggregateExplanation(builder, 2, "Result Explanation", result.Explanation)

	for _, statement := range result.Statements {
		builder.WriteString("## Statement ")
		builder.WriteString(strconv.Itoa(statement.Index + 1))
		builder.WriteString("\n\n- Kind: `")
		builder.WriteString(statement.Kind)
		builder.WriteString("`\n")
		if statement.NormalizedSQL != "" {
			builder.WriteString("- SQL: ")
			builder.WriteString(inlineCodeSpan(statement.NormalizedSQL))
			builder.WriteString("\n")
		} else if statement.RawSQL != "" {
			builder.WriteString("- SQL: ")
			builder.WriteString(inlineCodeSpan(statement.RawSQL))
			builder.WriteString("\n")
		}
		builder.WriteString("\n")
		writeAggregateExplanation(builder, 3, "Explanation", statement.Explanation)
		writeImpact(builder, statement.Impact)

		if len(statement.Findings) == 0 {
			builder.WriteString("No findings.\n\n")
			continue
		}

		builder.WriteString("### Findings\n\n")
		for _, finding := range statement.Findings {
			writeFinding(builder, finding)
		}
		builder.WriteString("\n")
	}

	if len(result.GlobalFindings) > 0 {
		builder.WriteString("## Global Findings\n\n")
		for _, finding := range result.GlobalFindings {
			writeFinding(builder, finding)
		}
	}

	if result.RuleSummary != nil {
		builder.WriteString("## Rule Summary\n\n- Loaded: ")
		builder.WriteString(strconv.Itoa(result.RuleSummary.Loaded))
		builder.WriteString("\n- Applicable: ")
		builder.WriteString(strconv.Itoa(result.RuleSummary.Applicable))
		builder.WriteString("\n- Skipped: ")
		builder.WriteString(strconv.Itoa(len(result.RuleSummary.Skipped)))
		builder.WriteString("\n\n")

		if len(result.RuleSummary.Skipped) > 0 {
			builder.WriteString("## Skipped Rules\n\n")
			for _, skipped := range result.RuleSummary.Skipped {
				builder.WriteString("- `")
				builder.WriteString(skipped.RuleID)
				builder.WriteString("`: ")
				builder.WriteString(formatSkipReason(skipped.Reason))
				builder.WriteString("\n")
			}
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
		builder.WriteString("- `estimated_rows`: `")
		builder.WriteString(strconv.FormatInt(*impact.EstimatedRows, 10))
		builder.WriteString("`\n")
	}
	if impact.EstimatedRatio != nil {
		builder.WriteString("- `estimated_ratio`: `")
		builder.WriteString(formatImpactRatio(*impact.EstimatedRatio))
		builder.WriteString("`\n")
	}
	if impact.RiskLevel != "" {
		builder.WriteString("- `risk_level`: `")
		builder.WriteString(string(impact.RiskLevel))
		builder.WriteString("`\n")
	}
	if impact.Confidence != "" {
		builder.WriteString("- `confidence`: `")
		builder.WriteString(string(impact.Confidence))
		builder.WriteString("`\n")
	}
	if impact.Source != "" {
		builder.WriteString("- `source`: `")
		builder.WriteString(string(impact.Source))
		builder.WriteString("`\n")
	}
	for _, code := range impact.ReasonCodes {
		builder.WriteString("- `reason_code`: `")
		builder.WriteString(code)
		builder.WriteString("`\n")
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

func formatSkipReason(reason rule.SkipReason) string {
	switch reason {
	case rule.SkipReasonDialectMismatch:
		return "not applicable to current dialect"
	default:
		return string(reason)
	}
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
	builder.WriteString(strings.Repeat("#", level))
	builder.WriteString(" ")
	builder.WriteString(heading)
	builder.WriteString("\n\n")
	if explanation.Summary != "" {
		builder.WriteString(explanation.Summary)
		builder.WriteString("\n")
	}
	for _, reason := range explanation.Reasons {
		builder.WriteString("- ")
		builder.WriteString(reason)
		builder.WriteString("\n")
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
	builder.WriteString("- [")
	builder.WriteString(string(finding.Level))
	builder.WriteString("] `")
	builder.WriteString(finding.RuleID)
	builder.WriteString("`: ")
	builder.WriteString(finding.Message)
	builder.WriteString("\n")
	if finding.Explanation != nil {
		if finding.Explanation.Why != "" {
			builder.WriteString("  Why: ")
			builder.WriteString(finding.Explanation.Why)
			builder.WriteString("\n")
		}
		if finding.Explanation.Risk != "" {
			builder.WriteString("  Risk: ")
			builder.WriteString(finding.Explanation.Risk)
			builder.WriteString("\n")
		}
		suggestion := finding.Explanation.Suggestion
		if suggestion == "" {
			suggestion = finding.Suggestion
		}
		if suggestion != "" {
			builder.WriteString("  Suggestion: ")
			builder.WriteString(suggestion)
			builder.WriteString("\n")
		}
		if finding.Explanation.Metadata != nil {
			if finding.Explanation.Metadata.Status != "" {
				builder.WriteString("  Metadata status: `")
				builder.WriteString(finding.Explanation.Metadata.Status)
				builder.WriteString("`\n")
			}
			if finding.Explanation.Metadata.Note != "" {
				builder.WriteString("  Metadata note: ")
				builder.WriteString(finding.Explanation.Metadata.Note)
				builder.WriteString("\n")
			}
		}
	} else if finding.Suggestion != "" {
		builder.WriteString("  Suggestion: ")
		builder.WriteString(finding.Suggestion)
		builder.WriteString("\n")
	}
	if finding.StatementKind != "" {
		builder.WriteString("  Statement kind: `")
		builder.WriteString(finding.StatementKind)
		builder.WriteString("`\n")
	}
	if len(finding.Metadata) > 0 {
		builder.WriteString("  Metadata:\n")
		keys := make([]string, 0, len(finding.Metadata))
		for key := range finding.Metadata {
			keys = append(keys, key)
		}
		slices.Sort(keys)
		for _, key := range keys {
			builder.WriteString("  - `")
			builder.WriteString(key)
			builder.WriteString("`: `")
			builder.WriteString(fmt.Sprintf("%v", finding.Metadata[key]))
			builder.WriteString("`\n")
		}
	}
}

func estimateResultSize(result report.Result) int {
	size := 256
	for _, stmt := range result.Statements {
		size += len(stmt.RawSQL) + len(stmt.NormalizedSQL) + 200
		size += len(stmt.Findings) * 150
	}
	size += len(result.GlobalFindings) * 150
	return size
}
