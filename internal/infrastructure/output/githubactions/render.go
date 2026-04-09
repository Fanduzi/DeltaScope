// Package githubactions renders audit results as GitHub Actions workflow commands.
// input: internal report results from the audit application flow
// output: GitHub Actions annotation commands for CI pipeline integration
// pos: infrastructure output adapter for the GitHub Actions CI-native renderer
// note: if this file changes, update this header and module README.md.
package githubactions

import (
	"fmt"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
)

// Render formats an audit result into GitHub Actions annotation commands.
func Render(result report.Result) ([]byte, error) {
	var lines []string

	for _, stmt := range result.Statements {
		for _, finding := range stmt.Findings {
			lines = append(lines, formatAnnotation(finding))
		}
	}
	for _, finding := range result.GlobalFindings {
		lines = append(lines, formatAnnotation(finding))
	}
	for _, item := range result.Unsupported {
		lines = append(lines, fmt.Sprintf("::notice title=Unsupported Statement %d::%s: %s", item.Index+1, item.Feature, item.Reason))
	}

	if len(lines) == 0 {
		return nil, nil
	}
	return []byte(strings.Join(lines, "\n")), nil
}

func formatAnnotation(finding rule.Finding) string {
	level := mapLevel(finding.Level)
	title := escapeValue(finding.RuleID)
	message := escapeValue(finding.Message)
	if finding.Explanation != nil && finding.Explanation.Suggestion != "" {
		message = escapeValue(fmt.Sprintf("%s Suggestion: %s", finding.Message, finding.Explanation.Suggestion))
	}

	if finding.Location != nil {
		return fmt.Sprintf("::%s file=,line=%d,col=%d,title=%s::%s", level, finding.Location.Line, finding.Location.Column, title, message)
	}
	return fmt.Sprintf("::%s title=%s::%s", level, title, message)
}

// escapeValue encodes values embedded in GitHub Actions workflow commands.
// Percent, newlines, and carriage returns must be escaped to prevent
// truncation or parsing errors.
func escapeValue(s string) string {
	s = strings.ReplaceAll(s, "%", "%25")
	s = strings.ReplaceAll(s, "\n", "%0A")
	s = strings.ReplaceAll(s, "\r", "%0D")
	return s
}

func mapLevel(level rule.Level) string {
	switch level {
	case rule.LevelBlocker:
		return "error"
	case rule.LevelWarning:
		return "warning"
	case rule.LevelNotice:
		return "notice"
	default:
		return "notice"
	}
}
