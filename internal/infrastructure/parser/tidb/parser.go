// Package tidbparser loads SQL text through the TiDB parser.
// input: SQL text and TiDB parser runtime dependencies
// output: raw parsed statement nodes and parser warnings for application orchestration
// pos: infrastructure parser adapter for MySQL and TiDB SQL parsing
// note: if this file changes, update this header and module README.md.
package tidbparser

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	"github.com/pingcap/tidb/pkg/parser"
	"github.com/pingcap/tidb/pkg/parser/ast"
	_ "github.com/pingcap/tidb/pkg/parser/test_driver"
)

// Parser adapts TiDB parser behavior for DeltaScope.
type Parser struct{}

// Result is the infrastructure output of the parser adapter.
type Result struct {
	Statements []ast.StmtNode
	Warnings   []string
}

// New returns a TiDB-backed parser adapter.
func New() *Parser {
	return &Parser{}
}

// Parse parses SQL text into TiDB statements and preserves parser warnings.
func (p *Parser) Parse(ctx context.Context, sql string) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, fmt.Errorf("parse cancelled: %w", err)
	}

	// Structural supplements run BEFORE the base parse: column-option
	// DEFAULT (<expr>) sites, column VISIBLE/INVISIBLE attributes, and the
	// narrow SET VISIBLE|INVISIBLE setter are planned from the token stream,
	// rewritten same-width, then rebound to typed facts on real AST nodes.
	// Any plan that does not bind exactly falls through to the untouched
	// base parse so failures keep their original diagnostics.
	var parsed []ast.StmtNode
	var warns []error
	if recovered, recoveredWarns, ok := runSupplementedParse(sql); ok {
		parsed, warns = recovered, recoveredWarns
	} else {
		var err error
		parsed, warns, err = parser.New().Parse(sql, "", "")
		if err != nil {
			return Result{}, fmt.Errorf("parse sql: %w", err)
		}
	}

	result := Result{
		Statements: make([]ast.StmtNode, 0, len(parsed)),
		Warnings:   make([]string, 0, len(warns)),
	}

	for _, warn := range warns {
		result.Warnings = append(result.Warnings, warn.Error())
	}

	result.Statements = append(result.Statements, parsed...)

	return result, nil
}

var _ spec.StatementExtractor = tidbExtractor{}
