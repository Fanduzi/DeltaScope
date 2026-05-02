//go:build postgresql

package postgresql

import (
	"fmt"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

type Parser struct{}

type Result struct {
	Statements []ExtractedStatement
	Warnings   []string
}

func New() *Parser { return &Parser{} }

func (p *Parser) Parse(sql string) (Result, error) {
	result, err := pg_query.Parse(sql)
	if err != nil {
		return Result{}, fmt.Errorf("parse postgresql sql: %w", err)
	}

	wrapped := make([]ExtractedStatement, 0, len(result.GetStmts()))
	for _, stmt := range result.GetStmts() {
		if stmt == nil || stmt.GetStmt() == nil {
			continue
		}
		rawSQL := statementSQL(sql, stmt.GetStmtLocation(), stmt.GetStmtLen())
		kind := classify(stmt.GetStmt())
		wrapped = append(wrapped, ExtractedStatement{
			Kind:      kind,
			RawSQL:    rawSQL,
			Extractor: pgExtractor{kind: kind, node: stmt.GetStmt()},
		})
	}

	return Result{Statements: wrapped}, nil
}

func statementSQL(sql string, location int32, length int32) string {
	start := int(location)
	if start < 0 || start >= len(sql) {
		return strings.TrimSpace(sql)
	}
	if length <= 0 {
		return strings.TrimSpace(sql[start:])
	}
	end := start + int(length)
	if end > len(sql) {
		end = len(sql)
	}
	for end < len(sql) {
		switch sql[end] {
		case ' ', '\t', '\n', '\r':
			end++
			continue
		case ';':
			end++
		}
		break
	}
	return strings.TrimSpace(sql[start:end])
}

func classify(node *pg_query.Node) spec.Kind {
	switch node.GetNode().(type) {
	case *pg_query.Node_CreateStmt,
		*pg_query.Node_ViewStmt,
		*pg_query.Node_AlterTableStmt,
		*pg_query.Node_AlterObjectSchemaStmt,
		*pg_query.Node_RenameStmt,
		*pg_query.Node_DropStmt,
		*pg_query.Node_IndexStmt,
		*pg_query.Node_TruncateStmt,
		*pg_query.Node_CreateSchemaStmt,
		*pg_query.Node_CreateSeqStmt,
		*pg_query.Node_AlterSeqStmt,
		*pg_query.Node_RefreshMatViewStmt,
		*pg_query.Node_CreateEnumStmt,
		*pg_query.Node_AlterEnumStmt,
		*pg_query.Node_CompositeTypeStmt,
		*pg_query.Node_CreateDomainStmt:
		return spec.KindDDL
	case *pg_query.Node_CreateTableAsStmt:
		if n := node.GetCreateTableAsStmt(); n != nil && n.GetObjtype() == pg_query.ObjectType_OBJECT_MATVIEW {
			return spec.KindDDL
		}
		return spec.KindUnknown
	case *pg_query.Node_InsertStmt,
		*pg_query.Node_UpdateStmt,
		*pg_query.Node_DeleteStmt:
		return spec.KindDML
	default:
		return spec.KindUnknown
	}
}
