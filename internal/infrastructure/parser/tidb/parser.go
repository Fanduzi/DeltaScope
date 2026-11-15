// Package tidbparser loads SQL text through the TiDB parser.
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
	// The MySQL CPU resource-group family is claimed before the structural
	// paths: a whole-input pure MySQL-form statement fabricates its carrier
	// node; anything ambiguous falls through so the real grammar owns the
	// diagnostics (T24/#103).
	if rg, ok := parseMySQLResourceGroup(sql); ok {
		parsed, warns = []ast.StmtNode{rg}, nil
	} else if ts, tsOK := parseMySQLTablespace(sql); tsOK {
		// The MySQL tablespace family — CREATE/ALTER/DROP TABLESPACE,
		// UNDO TABLESPACE, LOGFILE GROUP — is claimed whole-input before
		// the structural paths; anything ambiguous falls through so the
		// real grammar owns the diagnostics (T25/#104).
		parsed, warns = []ast.StmtNode{ts}, nil
	} else if srv, srvOK := parseMySQLServer(sql); srvOK {
		// The MySQL SERVER family — CREATE/ALTER/DROP SERVER — is
		// claimed whole-input; option payloads stay opaque and the
		// serialized output renders the redacted shape (T26/#105).
		parsed, warns = []ast.StmtNode{srv}, nil
	} else if srs, srsOK := parseMySQLSpatialReference(sql); srsOK {
		// The MySQL spatial reference system family — CREATE [OR
		// REPLACE] [IF NOT EXISTS] and DROP [IF EXISTS] — claimed
		// whole-input; DEFINITION payloads stay opaque (T26/#105).
		parsed, warns = []ast.StmtNode{srs}, nil
	} else if inst, instOK := parseMySQLAlterInstance(sql); instOK {
		// The MySQL-only ALTER INSTANCE subaction ROTATE INNODB MASTER
		// KEY is claimed whole-input; RELOAD TLS stays on the native
		// ast path (T26/#105 inventory scope).
		parsed, warns = []ast.StmtNode{inst}, nil
	} else if rt, rtWarns, rtOK := parseMySQLRoutine(sql); rtOK {
		// The MySQL stored-routine family — CREATE PROCEDURE|FUNCTION
		// including compound BEGIN…END bodies the upstream grammar
		// cannot represent — claimed whole-input so body semicolons
		// are never outer statement boundaries (T27/#106).
		parsed, warns = []ast.StmtNode{rt}, rtWarns
	} else if tr, trWarns, trOK := parseMySQLTrigger(sql); trOK {
		// The MySQL trigger family — CREATE|DROP TRIGGER with the
		// FOR EACH ROW body riding the shared routine member
		// machinery — claimed whole-input (T29/#108).
		parsed, warns = []ast.StmtNode{tr}, trWarns
	} else if header, viewHeaderOK := parseViewHeader(sql); viewHeaderOK && (header.operation == spec.DDLOperationAlterView || header.declaration.CheckBare) {
		if view, viewWarns, ok := parseViewAdapter(ctx, sql); ok {
			parsed, warns = []ast.StmtNode{view}, viewWarns
		}
	}
	if parsed != nil {
		// The adapter owns the original statement and its parsed SELECT.
	} else if recovered, recoveredWarns, ok := runSupplementedParse(sql); ok {
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
