// Package githubsummary renders concise GitHub Actions job summaries.
// input: internal report results from the audit application flow
// output: GitHub-flavored Markdown for GITHUB_STEP_SUMMARY
// pos: infrastructure output adapter for CI job-summary rendering
// note: if this file changes, update this header and module README.md.
package githubsummary

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule/catalog"
	"github.com/Fanduzi/DeltaScope/internal/infrastructure/output"
)

// actionSummaryLimit caps the number of rule groups rendered in the job summary,
// matching the default markdown Action Summary cap.
const actionSummaryLimit = 10

// Render formats an audit result as concise GitHub-flavored Markdown suitable for
// appending to GITHUB_STEP_SUMMARY. It intentionally omits raw SQL.
func Render(result report.Result) ([]byte, error) {
	builder := output.GetBuilder()
	defer output.PutBuilder(builder)

	builder.WriteString("## DeltaScope SQL Review\n\n")
	writeVerdict(builder, result.Verdict)
	writeCounts(builder, result.Summary)

	summary := report.BuildActionSummary(result, catalog.All(), report.ActionSummaryOptions{Limit: actionSummaryLimit})
	if summary.TotalItems == 0 {
		builder.WriteString("No findings.\n")
		writeUnsupported(builder, len(result.Unsupported))
		return []byte(builder.String()), nil
	}

	builder.WriteString("## Action Summary\n\n")
	for _, item := range summary.Items {
		writeActionItem(builder, item)
	}
	if shown := len(summary.Items); summary.TotalItems > shown {
		fmt.Fprintf(builder, "Showing %d of %d rule groups.\n", shown, summary.TotalItems)
	}
	writeUnsupported(builder, len(result.Unsupported))
	return []byte(builder.String()), nil
}

// writeVerdict renders the uppercased canonical verdict (pass/review/reject) so the
// job summary stays consistent with the report verdict model rather than inventing a
// binary PASS/FAIL term.
func writeVerdict(builder *strings.Builder, verdict report.Verdict) {
	builder.WriteString("Verdict: ")
	builder.WriteString(strings.ToUpper(string(verdict)))
	builder.WriteString("\n\n")
}

// writeCounts renders the audit metric counts as a right-aligned Markdown table.
func writeCounts(builder *strings.Builder, summary report.Summary) {
	builder.WriteString("| Metric | Count |\n")
	builder.WriteString("| --- | ---: |\n")
	builder.WriteString("| Statements | ")
	builder.WriteString(strconv.Itoa(summary.Statements))
	builder.WriteString(" |\n")
	builder.WriteString("| Blockers | ")
	builder.WriteString(strconv.Itoa(summary.Blockers))
	builder.WriteString(" |\n")
	builder.WriteString("| Warnings | ")
	builder.WriteString(strconv.Itoa(summary.Warnings))
	builder.WriteString(" |\n")
	builder.WriteString("| Notices | ")
	builder.WriteString(strconv.Itoa(summary.Notices))
	builder.WriteString(" |\n\n")
}

// writeActionItem renders one derived rule group as a Markdown list item, mirroring the
// markdown Action Summary layout but without raw SQL or finding metadata.
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

// writeUnsupported renders a concise count of unsupported statements without their SQL,
// so unsupported diagnostics are not silently hidden in a job summary.
func writeUnsupported(builder *strings.Builder, count int) {
	if count == 0 {
		return
	}
	builder.WriteString("Unsupported statements: ")
	builder.WriteString(strconv.Itoa(count))
	builder.WriteString("\n")
}

// formatFindingCount renders a singular/plural finding count, e.g. "1 finding" or "2 findings".
func formatFindingCount(count int) string {
	if count == 1 {
		return "1 finding"
	}
	return strconv.Itoa(count) + " findings"
}

// formatStatementIndexes renders 1-based statement indexes as a comma-separated list.
func formatStatementIndexes(indexes []int) string {
	parts := make([]string, 0, len(indexes))
	for _, index := range indexes {
		parts = append(parts, strconv.Itoa(index))
	}
	return strings.Join(parts, ", ")
}
