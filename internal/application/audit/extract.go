// Package audit orchestrates audit use cases at the application layer.
// input: application-owned parsed SQL statements and parser-neutral extractors
// output: first-pass StatementSpec values plus attached shape-only impact facts for later rule evaluation
// pos: application extraction step between parsing and rule execution
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// Extract converts parsed statements into first-pass domain StatementSpec values.
func Extract(ctx context.Context, parsed ParsedSQL) ([]spec.Statement, error) {
	statements := make([]spec.Statement, 0, len(parsed.Statements))
	for index, stmt := range parsed.Statements {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("extract cancelled: %w", err)
		}
		if stmt.Extractor == nil {
			return nil, fmt.Errorf("extract statement %d: missing extractor", index)
		}
		extracted, err := stmt.Extractor.Extract(parsed.Dialect, stmt.RawSQL)
		if err != nil {
			return nil, fmt.Errorf("extract statement %d: %w", index, err)
		}
		if extracted.DML != nil {
			extracted.DML.Impact = estimateStatementImpact(extracted)
		}
		statements = append(statements, extracted)
		statements[len(statements)-1].Line = stmt.Line
		statements[len(statements)-1].Column = stmt.Column
	}
	return statements, nil
}
