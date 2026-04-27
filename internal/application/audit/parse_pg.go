//go:build postgresql

package audit

import (
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pgparser "github.com/Fanduzi/DeltaScope/internal/infrastructure/parser/postgresql"
)

func parsePostgreSQL(sql string) (ParsedSQL, error) {
	result, err := pgparser.New().Parse(sql)
	if err != nil {
		return ParsedSQL{}, err
	}

	parsed := ParsedSQL{
		Dialect:    spec.DialectPostgreSQL,
		Statements: make([]ParsedStatement, 0, len(result.Statements)),
		Warnings:   append([]string(nil), result.Warnings...),
	}

	for _, stmt := range result.Statements {
		parsed.Statements = append(parsed.Statements, ParsedStatement{
			Kind:      stmt.Kind,
			RawSQL:    stmt.RawSQL,
			Extractor: stmt.Extractor,
		})
	}

	attachParsedStatementLocations(parsed.Statements, sql)

	return parsed, nil
}
