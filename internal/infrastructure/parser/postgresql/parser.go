//go:build postgresql

package postgresql

import (
	"context"
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

func (p *Parser) Parse(ctx context.Context, sql string) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, fmt.Errorf("parse cancelled: %w", err)
	}

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
		*pg_query.Node_CreateDomainStmt,
		*pg_query.Node_AlterDomainStmt,
		*pg_query.Node_CreateExtensionStmt,
		*pg_query.Node_AlterExtensionStmt,
		*pg_query.Node_AlterExtensionContentsStmt,
		*pg_query.Node_GrantStmt,
		*pg_query.Node_GrantRoleStmt,
		*pg_query.Node_AlterDefaultPrivilegesStmt,
		*pg_query.Node_CreatePolicyStmt,
		*pg_query.Node_AlterPolicyStmt,
		*pg_query.Node_CreateTrigStmt,
		*pg_query.Node_CreateFunctionStmt,
		*pg_query.Node_CreatePublicationStmt,
		*pg_query.Node_AlterPublicationStmt,
		*pg_query.Node_CreateSubscriptionStmt,
		*pg_query.Node_AlterSubscriptionStmt,
		*pg_query.Node_DropSubscriptionStmt,
		*pg_query.Node_AlterOwnerStmt,
		*pg_query.Node_CreateForeignTableStmt,
		*pg_query.Node_CreateForeignServerStmt,
		*pg_query.Node_AlterForeignServerStmt,
		*pg_query.Node_CreateUserMappingStmt,
		*pg_query.Node_AlterUserMappingStmt,
		*pg_query.Node_DropUserMappingStmt,
		*pg_query.Node_CreateFdwStmt,
		*pg_query.Node_AlterFdwStmt,
		*pg_query.Node_CommentStmt,
		*pg_query.Node_SecLabelStmt,
		*pg_query.Node_CreateEventTrigStmt,
		*pg_query.Node_AlterEventTrigStmt,
		*pg_query.Node_RuleStmt,
		*pg_query.Node_CreateStatsStmt,
		*pg_query.Node_AlterStatsStmt,
		*pg_query.Node_CreateConversionStmt,
		*pg_query.Node_CreateOpFamilyStmt,
		*pg_query.Node_CreateOpClassStmt,
		*pg_query.Node_DefineStmt:
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
