// Package audit provides application-layer parser mismatch hints.
package audit

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

var possiblePostgreSQLMismatchPatterns = []regexpPatternHint{
	{pattern: regexp.MustCompile(`(?i)\bon\s+conflict\b`), token: "ON CONFLICT"},
	{pattern: regexp.MustCompile(`::`), token: "::"},
	{pattern: regexp.MustCompile(`(?i)\balter\s+table\b[\s\S]*\balter\s+column\b[\s\S]*\btype\b[\s\S]*\busing\b`), token: "ALTER COLUMN TYPE USING"},
	{pattern: regexp.MustCompile(`(?i)\bgenerated\b[\s\S]*\bas\s+identity\b`), token: "GENERATED AS IDENTITY"},
}

// mysqlReturningUnsupportedNoticeRuleID is the stable rule id for the
// non-configurable MySQL dialect-boundary notice emitted when a parsed DML
// statement carries a RETURNING clause on the MySQL Server path. MySQL Server
// does not support DML RETURNING; TiDB does.
const mysqlReturningUnsupportedNoticeRuleID = "dialect.mysql.returning.unsupported.notice"

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

// statementHasMySQLUnsupportedReturning reports whether a parsed statement is a
// MySQL-dialect DML carrying a real RETURNING clause. This is a structural
// parser fact (statement.DML.HasReturning), not a raw token scan, so an
// identifier or alias named "returning" does not trigger it. TiDB statements
// never match because TiDB supports DML RETURNING.
func statementHasMySQLUnsupportedReturning(statement spec.Statement) bool {
	return statement.Dialect == spec.DialectMySQL && statement.DML != nil && statement.DML.HasReturning
}

// buildMySQLReturningUnsupportedFinding constructs the non-configurable MySQL
// dialect-boundary notice for a parsed DML RETURNING clause. The payload carries
// only bounded tokens (current dialect, suggested dialect, token name); it never
// carries raw SQL, returned column names, expressions, parser error fragments,
// connection details, credentials, or routine bodies.
func buildMySQLReturningUnsupportedFinding(dialect string) rule.Finding {
	message := "MySQL Server does not support DML RETURNING; if this SQL targets TiDB, re-run with --dialect tidb."
	suggestion := "If this SQL targets TiDB, re-run with --dialect tidb to audit RETURNING accurately. If it targets MySQL Server, remove the RETURNING clause."
	return rule.Finding{
		RuleID:     mysqlReturningUnsupportedNoticeRuleID,
		Level:      rule.LevelNotice,
		Message:    message,
		Suggestion: suggestion,
		Metadata: map[string]any{
			"dialect":           dialect,
			"suggested_dialect": "tidb",
			"token":             "RETURNING",
		},
		Explanation: &rule.FindingExplanation{
			Summary:    message,
			Why:        fmt.Sprintf("Detected a DML RETURNING clause while auditing on the %s dialect path. MySQL Server does not support RETURNING; TiDB does.", dialect),
			Risk:       "DeltaScope does not auto-switch dialect. RETURNING syntax is valid for TiDB but not for MySQL Server, so findings may not apply to MySQL Server.",
			Suggestion: suggestion,
		},
	}
}
