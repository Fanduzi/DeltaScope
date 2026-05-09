// Package audit orchestrates audit use cases at the application layer.
// input: SQL text, selected dialect, and infrastructure-backed parser adapters
// output: application-owned parsed statements for later extraction and rule evaluation
// pos: application parsing entrypoint between interfaces and parser infrastructure
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	tidbparser "github.com/Fanduzi/DeltaScope/internal/infrastructure/parser/tidb"
)

// ParsedStatement keeps application-facing statement metadata while hiding parser nodes.
type ParsedStatement struct {
	Kind      spec.Kind               `json:"kind"`
	RawSQL    string                  `json:"raw_sql"`
	Line      int                     `json:"line,omitempty"`
	Column    int                     `json:"col,omitempty"`
	Extractor spec.StatementExtractor `json:"-"`
}

// ParsedSQL is the application-owned parsing result used by later extraction steps.
type ParsedSQL struct {
	Dialect    spec.Dialect      `json:"dialect"`
	Statements []ParsedStatement `json:"statements"`
	Warnings   []string          `json:"warnings,omitempty"`
}

// PostgreSQLCapabilityBoundaryError reports that PostgreSQL parsing needs a PostgreSQL-capable build.
type PostgreSQLCapabilityBoundaryError struct {
	Message string
}

func (e *PostgreSQLCapabilityBoundaryError) Error() string {
	return e.Message
}

// Parse delegates SQL parsing to the dialect-specific parser adapter.
func Parse(ctx context.Context, sql string, dialect spec.Dialect) (ParsedSQL, error) {
	switch dialect {
	case spec.DialectMySQL, spec.DialectTiDB:
		return parseTiDB(ctx, sql, dialect)
	case spec.DialectPostgreSQL:
		return parsePostgreSQL(ctx, sql)
	default:
		return ParsedSQL{}, fmt.Errorf("unsupported dialect: %s", dialect)
	}
}

func parseTiDB(ctx context.Context, sql string, dialect spec.Dialect) (ParsedSQL, error) {
	result, err := tidbparser.New().Parse(ctx, sql)
	if err != nil {
		return ParsedSQL{}, fmt.Errorf("parse sql: %w", err)
	}

	parsed := ParsedSQL{
		Dialect:    dialect,
		Statements: make([]ParsedStatement, 0, len(result.Statements)),
		Warnings:   append([]string(nil), result.Warnings...),
	}

	for _, stmt := range tidbparser.WrapStatements(result.Statements, result.Warnings) {
		parsed.Statements = append(parsed.Statements, ParsedStatement{
			Kind:      stmt.Kind,
			RawSQL:    stmt.RawSQL,
			Extractor: stmt.Extractor,
		})
	}

	attachParsedStatementLocations(parsed.Statements, sql)

	return parsed, nil
}
