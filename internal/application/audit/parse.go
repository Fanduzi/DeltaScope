// Package audit orchestrates audit use cases at the application layer.
// input: SQL text, selected dialect, and infrastructure-backed parser adapters
// output: application-owned parsed statements for later extraction and rule evaluation
// pos: application parsing entrypoint between interfaces and parser infrastructure
// note: if this file changes, update this header and module README.md.
package audit

import (
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	tidbparser "github.com/Fanduzi/DeltaScope/internal/infrastructure/parser/tidb"
	"github.com/pingcap/tidb/pkg/parser/ast"
)

// ParsedStatement keeps application-facing statement metadata while hiding parser nodes.
type ParsedStatement struct {
	Kind   spec.Kind `json:"kind"`
	RawSQL string    `json:"raw_sql"`
	node   ast.StmtNode
}

// ParsedSQL is the application-owned parsing result used by later extraction steps.
type ParsedSQL struct {
	Dialect    spec.Dialect     `json:"dialect"`
	Statements []ParsedStatement `json:"statements"`
	Warnings   []string         `json:"warnings,omitempty"`
}

// Parse delegates SQL parsing to the TiDB-backed parser adapter for supported v1 dialects.
func Parse(sql string, dialect spec.Dialect) (ParsedSQL, error) {
	result, err := tidbparser.New().Parse(sql)
	if err != nil {
		return ParsedSQL{}, err
	}

	parsed := ParsedSQL{
		Dialect:    dialect,
		Statements: make([]ParsedStatement, 0, len(result.Statements)),
		Warnings:   append([]string(nil), result.Warnings...),
	}

	for _, stmt := range result.Statements {
		parsed.Statements = append(parsed.Statements, ParsedStatement{
			Kind:   classify(stmt),
			RawSQL: stmt.Text(),
			node:   stmt,
		})
	}

	return parsed, nil
}

func classify(stmt ast.StmtNode) spec.Kind {
	switch stmt.(type) {
	case *ast.CreateTableStmt,
		*ast.CreateViewStmt,
		*ast.CreateIndexStmt,
		*ast.AlterTableStmt,
		*ast.DropTableStmt,
		*ast.DropIndexStmt,
		*ast.RenameTableStmt,
		*ast.TruncateTableStmt:
		return spec.KindDDL
	case *ast.InsertStmt,
		*ast.UpdateStmt,
		*ast.DeleteStmt:
		return spec.KindDML
	default:
		return spec.KindUnknown
	}
}
