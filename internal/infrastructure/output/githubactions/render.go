// Package githubactions renders audit results as GitHub Actions workflow commands.
// input: internal report findings and source-located parser diagnostics from the audit application flow
// output: GitHub Actions finding and parser-error annotation commands for CI pipeline integration
// pos: infrastructure output adapter for the GitHub Actions CI-native renderer
// note: if this file changes, update this header and module README.md.
package githubactions

import (
	"fmt"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// Options carries renderer configuration.
type Options struct {
	// Path is the source file path. When empty, annotations omit the file parameter.
	Path string
}

// Render formats an audit result into GitHub Actions annotation commands.
func Render(result report.Result, options Options) ([]byte, error) {
	lines := make([]string, 0, len(result.Statements))

	for _, stmt := range result.Statements {
		for _, finding := range stmt.Findings {
			lines = append(lines, formatAnnotation(finding, options))
		}
	}
	for _, finding := range result.GlobalFindings {
		lines = append(lines, formatAnnotation(finding, options))
	}
	for _, item := range result.Unsupported {
		lines = append(lines, fmt.Sprintf("::notice title=Unsupported Statement %d::%s: %s", item.Index+1, item.Feature, item.Reason))
	}
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Classification == "parser_error" {
			lines = append(lines, formatParserDiagnostic(diagnostic, options))
		}
	}

	if len(lines) == 0 {
		return nil, nil
	}
	return []byte(strings.Join(lines, "\n")), nil
}

func formatParserDiagnostic(diagnostic spec.Diagnostic, options Options) string {
	message := diagnostic.Reason
	if diagnostic.ActionHint != "" {
		message += "\nAction: " + diagnostic.ActionHint
	}
	params := "title=Parser Error"
	if options.Path != "" {
		params = "file=" + escapeValue(options.Path) + "," + params
	}
	if diagnostic.Line > 0 {
		params = fmt.Sprintf("%s,line=%d", params, diagnostic.Line)
		if diagnostic.Column > 0 {
			params = fmt.Sprintf("%s,col=%d", params, diagnostic.Column)
		}
	}
	return fmt.Sprintf("::error %s::%s", params, escapeValue(message))
}

func formatAnnotation(finding rule.Finding, options Options) string {
	level := mapLevel(finding.Level)
	title := escapeValue(fmt.Sprintf("[%s] %s", finding.Level, finding.RuleID))

	// Build the message before escaping so the inserted newlines are encoded
	// once. Suggestion precedes the explain command so remediation reads first.
	message := finding.Message
	if finding.Explanation != nil && finding.Explanation.Suggestion != "" {
		message = fmt.Sprintf("%s\nSuggestion: %s", message, finding.Explanation.Suggestion)
	}
	message = fmt.Sprintf("%s\nExplain: deltascope rules explain %s", message, finding.RuleID)
	escapedMessage := escapeValue(message)

	if finding.Location != nil {
		if options.Path != "" {
			return fmt.Sprintf("::%s file=%s,line=%d,col=%d,title=%s::%s", level, escapeValue(options.Path), finding.Location.Line, finding.Location.Column, title, escapedMessage)
		}
		return fmt.Sprintf("::%s line=%d,col=%d,title=%s::%s", level, finding.Location.Line, finding.Location.Column, title, escapedMessage)
	}
	return fmt.Sprintf("::%s title=%s::%s", level, title, escapedMessage)
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
