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
	{pattern: regexp.MustCompile(`(?i)\b(bigserial|serial|smallserial)\b`), token: "SERIAL"},
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
	for _, candidate := range possiblePostgreSQLMismatchPatterns {
		if candidate.pattern.MatchString(trimmed) {
			return candidate.token, true
		}
	}
	return "", false
}

func formatPossiblePostgreSQLMismatchError(err error, dialect string, token string) error {
	return fmt.Errorf("%w; possible dialect mismatch: %s syntax %q is commonly PostgreSQL-specific; if this SQL targets PostgreSQL, use the postgresql dialect", err, dialect, token)
}

func buildPossiblePostgreSQLMismatchFinding(dialect string, token string) rule.Finding {
	return rule.Finding{
		RuleID:  "audit.dialect.mismatch.notice",
		Level:   rule.LevelNotice,
		Message: fmt.Sprintf("possible dialect mismatch: %s syntax %q is commonly PostgreSQL-specific; if this SQL targets PostgreSQL, use the postgresql dialect", dialect, token),
		Metadata: map[string]any{
			"dialect":           dialect,
			"suggested_dialect": "postgresql",
			"token":             token,
		},
	}
}
