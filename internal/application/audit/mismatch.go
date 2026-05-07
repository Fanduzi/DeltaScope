// Package audit provides application-layer parser mismatch hints.
package audit

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
)

var possiblePostgreSQLMismatchPatterns = []regexpPatternHint{
	{pattern: regexp.MustCompile(`(?i)\breturning\b`), token: "RETURNING"},
	{pattern: regexp.MustCompile(`(?i)\bon\s+conflict\b`), token: "ON CONFLICT"},
	{pattern: regexp.MustCompile(`::`), token: "::"},
	{pattern: regexp.MustCompile(`(?i)\balter\s+table\b[\s\S]*\balter\s+column\b[\s\S]*\btype\b[\s\S]*\busing\b`), token: "ALTER COLUMN TYPE USING"},
	{pattern: regexp.MustCompile(`(?i)\bgenerated\b[\s\S]*\bas\s+identity\b`), token: "GENERATED AS IDENTITY"},
}

type regexpPatternHint struct {
	pattern *regexp.Regexp
	token   string
}

func possiblePostgreSQLMismatch(sql string) (string, bool) {
	trimmed := strings.TrimSpace(sql)
	if trimmed == "" {
		return "", false
	}
	masked := maskLiteralsAndComments(trimmed)
	for _, candidate := range possiblePostgreSQLMismatchPatterns {
		if candidate.pattern.MatchString(masked) {
			return candidate.token, true
		}
	}
	return "", false
}

// maskLiteralsAndComments replaces the content of string literals, double-quoted
// identifiers, backtick-quoted identifiers, block comments, and line comments
// with spaces so that the PG heuristic regex scan only sees tokens in real SQL syntax.
func maskLiteralsAndComments(sql string) string {
	out := []byte(sql)
	i := 0
	for i < len(out) {
		switch {
		case out[i] == '\'':
			i++
			for i < len(out) {
				if out[i] == '\\' && i+1 < len(out) {
					out[i] = ' '
					i++
					if out[i] != '\n' {
						out[i] = ' '
					}
					i++
					continue
				}
				if out[i] == '\'' {
					i++
					if i >= len(out) || out[i] != '\'' {
						break
					}
					i++
					continue
				}
				if out[i] != '\n' {
					out[i] = ' '
				}
				i++
			}
		case out[i] == '"':
			i++
			for i < len(out) {
				if out[i] == '"' {
					i++
					break
				}
				if out[i] != '\n' {
					out[i] = ' '
				}
				i++
			}
		case out[i] == '`':
			i++
			for i < len(out) {
				if out[i] == '`' {
					i++
					break
				}
				if out[i] != '\n' {
					out[i] = ' '
				}
				i++
			}
		case i+1 < len(out) && out[i] == '-' && out[i+1] == '-':
			i += 2
			for i < len(out) && out[i] != '\n' {
				out[i] = ' '
				i++
			}
		case i+1 < len(out) && out[i] == '/' && out[i+1] == '*':
			i += 2
			for i < len(out) {
				if i+1 < len(out) && out[i] == '*' && out[i+1] == '/' {
					i += 2
					break
				}
				if out[i] != '\n' {
					out[i] = ' '
				}
				i++
			}
		default:
			i++
		}
	}
	return string(out)
}

//nolint:unused
func formatPossiblePostgreSQLMismatchError(err error, dialect string, token string) error {
	return fmt.Errorf("%w; possible dialect mismatch: %s syntax %q is commonly PostgreSQL-specific; if this SQL targets PostgreSQL, use the postgresql dialect", err, dialect, token)
}

func buildPossiblePostgreSQLMismatchFinding(dialect string, token string) rule.Finding {
	message := fmt.Sprintf("SQL looks like PostgreSQL because it uses %q syntax; if you are auditing PostgreSQL, pass --dialect postgresql", token)
	return rule.Finding{
		RuleID:     "dialect.postgresql.syntax.detected.notice",
		Level:      rule.LevelNotice,
		Message:    message,
		Suggestion: "If this SQL targets PostgreSQL, re-run with --dialect postgresql to get accurate findings. If not, you can safely ignore this notice.",
		Metadata: map[string]any{
			"dialect":           dialect,
			"suggested_dialect": "postgresql",
			"token":             token,
		},
		Explanation: &rule.FindingExplanation{
			Summary:    message,
			Why:        fmt.Sprintf("Detected PostgreSQL-specific syntax %q while auditing on the %s path.", token, dialect),
			Risk:       "DeltaScope does not auto-switch dialect. Auditing PostgreSQL SQL with the MySQL/TiDB parser can produce misleading parse errors or incomplete findings.",
			Suggestion: "If this SQL targets PostgreSQL, re-run with --dialect postgresql to get accurate findings. If not, you can safely ignore this notice.",
		},
	}
}
