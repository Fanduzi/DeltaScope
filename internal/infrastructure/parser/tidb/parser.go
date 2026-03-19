// Package tidbparser loads SQL text through the TiDB parser.
// input: SQL text, selected SQL dialect, and TiDB parser runtime dependencies
// output: parsed statement wrappers, statement kinds, and parser warnings
// pos: infrastructure parser adapter for MySQL and TiDB SQL parsing
// note: if this file changes, update this header and module README.md.
package tidbparser

import (
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	"github.com/pingcap/tidb/pkg/parser"
	"github.com/pingcap/tidb/pkg/parser/ast"
	_ "github.com/pingcap/tidb/pkg/parser/test_driver"
)

// Parser adapts TiDB parser behavior for DeltaScope.
type Parser struct{}

// ParsedStatement wraps one parsed statement without exposing it to the domain layer.
type ParsedStatement struct {
	Kind spec.Kind
	Text string
	Node ast.StmtNode
}

// Result is the application-facing output of the parser adapter.
type Result struct {
	Dialect    spec.Dialect
	Statements []ParsedStatement
	Warnings   []string
}

// New returns a TiDB-backed parser adapter.
func New() *Parser {
	return &Parser{}
}

// Parse parses SQL text into TiDB statements and classifies each statement at a coarse level.
func (p *Parser) Parse(sql string, dialect spec.Dialect) (Result, error) {
	parsed, warns, err := parser.New().Parse(sql, "", "")
	if err != nil {
		return Result{}, fmt.Errorf("parse sql: %w", err)
	}

	result := Result{
		Dialect:    dialect,
		Statements: make([]ParsedStatement, 0, len(parsed)),
		Warnings:   make([]string, 0, len(warns)),
	}

	for _, warn := range warns {
		result.Warnings = append(result.Warnings, warn.Error())
	}

	for _, stmt := range parsed {
		result.Statements = append(result.Statements, ParsedStatement{
			Kind: classify(stmt),
			Text: stmt.Text(),
			Node: stmt,
		})
	}

	return result, nil
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
