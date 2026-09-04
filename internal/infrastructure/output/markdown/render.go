// Package markdown renders audit results as human-readable Markdown.
// input: internal report results from the audit application flow
// output: deterministic Markdown bytes for CLI and agent consumption, with rule skip reasons aggregated by reason code
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
	"github.com/Fanduzi/DeltaScope/internal/domain/rule/catalog"
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
	writeActionSummary(builder, result)
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
		builder.WriteString("\n- Skipped with known reason: ")
		builder.WriteString(strconv.Itoa(len(result.RuleSummary.Skipped)))
		builder.WriteString("\n")

		if len(result.RuleSummary.Skipped) > 0 {
			builder.WriteString("\n### Skip Reasons\n")
			for _, group := range aggregateSkipReasons(result.RuleSummary.Skipped) {
				builder.WriteString("\n- ")
				builder.WriteString(formatSkipReason(group.Reason))
				builder.WriteString(": ")
				builder.WriteString(strconv.Itoa(group.Count))
			}
			builder.WriteString("\n")
		}
	}

	return []byte(builder.String()), nil
}

// writeActionSummary appends the derived action summary section when the result has findings.
// It groups findings by rule via report.BuildActionSummary using the shipped rule catalog,
// caps displayed rule groups at 10, and never emits raw SQL, finding metadata, or a severity
// field. Clean results (no findings) omit the section entirely.
func writeActionSummary(builder *strings.Builder, result report.Result) {
	summary := report.BuildActionSummary(result, catalog.All(), report.ActionSummaryOptions{Limit: 10})
	if summary.TotalItems == 0 {
		return
	}

	builder.WriteString("## Action Summary\n\n")
	for _, item := range summary.Items {
		writeActionItem(builder, item)
	}
	if shown := len(summary.Items); summary.TotalItems > shown {
		fmt.Fprintf(builder, "Showing %d of %d rule groups.\n", shown, summary.TotalItems)
	}
	builder.WriteString("\n")
}

// writeActionItem renders one derived rule group as a Markdown list item.
func writeActionItem(builder *strings.Builder, item report.ActionItem) {
	builder.WriteString("- [")
	builder.WriteString(string(item.Level))
	builder.WriteString("] `")
	builder.WriteString(item.RuleID)
	builder.WriteString("`: ")
	builder.WriteString(formatFindingCount(item.Count))
	builder.WriteString("\n")
	if item.Summary != "" {
		builder.WriteString("  Summary: ")
		builder.WriteString(item.Summary)
		builder.WriteString("\n")
	}
	if item.Suggestion != "" {
		builder.WriteString("  Suggestion: ")
		builder.WriteString(item.Suggestion)
		builder.WriteString("\n")
	}
	builder.WriteString("  Explain: ")
	builder.WriteString(item.ExplainCommand)
	builder.WriteString("\n")
	if len(item.StatementIndexes) > 0 {
		builder.WriteString("  Statements: ")
		builder.WriteString(formatStatementIndexes(item.StatementIndexes))
		builder.WriteString("\n")
	}
	if item.HasGlobalFindings {
		builder.WriteString("  Scope: global\n")
	}
}

// formatFindingCount renders a singular/plural finding count, e.g. "1 finding" or "2 findings".
func formatFindingCount(count int) string {
	if count == 1 {
		return "1 finding"
	}
	return strconv.Itoa(count) + " findings"
}

// formatStatementIndexes renders 1-based statement indexes as a comma-separated list, e.g. "1, 3".
func formatStatementIndexes(indexes []int) string {
	parts := make([]string, 0, len(indexes))
	for _, index := range indexes {
		parts = append(parts, strconv.Itoa(index))
	}
	return strings.Join(parts, ", ")
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

// skipReasonGroup is one aggregated skip-reason row: a distinct reason code and
// the number of skipped rules that share it.
type skipReasonGroup struct {
	Reason rule.SkipReason
	Count  int
}

// aggregateSkipReasons collapses the complete skipped-rule slice into one row
// per distinct SkipReason, ordered deterministically by the raw reason code.
// The dense per-rule list stays available in JSON; Markdown presents only the
// distinct explanations.
func aggregateSkipReasons(skipped []rule.SkippedRule) []skipReasonGroup {
	counts := make(map[rule.SkipReason]int)
	for _, skippedRule := range skipped {
		counts[skippedRule.Reason]++
	}
	reasons := make([]rule.SkipReason, 0, len(counts))
	for reason := range counts {
		reasons = append(reasons, reason)
	}
	slices.SortFunc(reasons, func(a, b rule.SkipReason) int {
		return strings.Compare(string(a), string(b))
	})
	groups := make([]skipReasonGroup, 0, len(reasons))
	for _, reason := range reasons {
		groups = append(groups, skipReasonGroup{Reason: reason, Count: counts[reason]})
	}
	return groups
}

func formatSkipReason(reason rule.SkipReason) string {
	switch reason {
	case rule.SkipReasonDialectMismatch:
		return "Not applicable to current dialect"
	case rule.SkipReasonFKForbid:
		return "Suppressed by ddl.table.foreign_key.forbid"
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
			fmt.Fprintf(builder, "%v", finding.Metadata[key])
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
