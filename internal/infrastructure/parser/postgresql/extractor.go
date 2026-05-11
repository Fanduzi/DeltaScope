//go:build postgresql

package postgresql

import (
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// ExtractedStatement is the adapter-owned parser result used by the application layer.
type ExtractedStatement struct {
	Kind      spec.Kind
	RawSQL    string
	Extractor spec.StatementExtractor
}

type pgExtractor struct {
	kind spec.Kind
	node *pg_query.Node
}

func (e pgExtractor) Extract(dialect spec.Dialect, rawSQL string) (spec.Statement, error) {
	statement := spec.Statement{
		Kind:          e.kind,
		Dialect:       dialect,
		RawSQL:        rawSQL,
		NormalizedSQL: strings.TrimSuffix(strings.TrimSpace(rawSQL), ";"),
	}
	if e.node == nil {
		return unsupportedStatement(statement, featureNameForUnknown(rawSQL), "postgresql statement did not include a parser node"), nil
	}

	switch node := e.node.GetNode().(type) {
	case *pg_query.Node_CreateStmt:
		return extractCreateStmt(statement, node.CreateStmt), nil
	case *pg_query.Node_ViewStmt:
		return extractViewStmt(statement, node.ViewStmt), nil
	case *pg_query.Node_AlterTableStmt:
		return extractAlterTableStmt(statement, node.AlterTableStmt), nil
	case *pg_query.Node_AlterObjectSchemaStmt:
		return extractAlterObjectSchemaStmt(statement, node.AlterObjectSchemaStmt), nil
	case *pg_query.Node_RenameStmt:
		return extractRenameStmt(statement, node.RenameStmt), nil
	case *pg_query.Node_DropStmt:
		return extractDropStmt(statement, node.DropStmt), nil
	case *pg_query.Node_IndexStmt:
		return extractIndexStmt(statement, node.IndexStmt), nil
	case *pg_query.Node_TruncateStmt:
		return extractTruncateStmt(statement, node.TruncateStmt), nil
	case *pg_query.Node_CreateSchemaStmt:
		return extractCreateSchemaStmt(statement, node.CreateSchemaStmt), nil
	case *pg_query.Node_CreateSeqStmt:
		return extractCreateSeqStmt(statement, node.CreateSeqStmt), nil
	case *pg_query.Node_AlterSeqStmt:
		return extractAlterSeqStmt(statement, node.AlterSeqStmt), nil
	case *pg_query.Node_CreateTableAsStmt:
		return extractCreateTableAsStmt(statement, node.CreateTableAsStmt), nil
	case *pg_query.Node_RefreshMatViewStmt:
		return extractRefreshMatViewStmt(statement, node.RefreshMatViewStmt), nil
	case *pg_query.Node_CreateEnumStmt:
		return extractCreateEnumStmt(statement, node.CreateEnumStmt), nil
	case *pg_query.Node_AlterEnumStmt:
		return extractAlterEnumStmt(statement, node.AlterEnumStmt), nil
	case *pg_query.Node_CompositeTypeStmt:
		return extractCompositeTypeStmt(statement, node.CompositeTypeStmt), nil
	case *pg_query.Node_CreateDomainStmt:
		return extractCreateDomainStmt(statement, node.CreateDomainStmt), nil
	case *pg_query.Node_AlterDomainStmt:
		return extractAlterDomainStmt(statement, node.AlterDomainStmt), nil
	case *pg_query.Node_CreateExtensionStmt:
		return extractCreateExtensionStmt(statement, node.CreateExtensionStmt), nil
	case *pg_query.Node_AlterExtensionStmt:
		return extractAlterExtensionStmt(statement, node.AlterExtensionStmt), nil
	case *pg_query.Node_AlterExtensionContentsStmt:
		return extractAlterExtensionContentsStmt(statement, node.AlterExtensionContentsStmt), nil
	case *pg_query.Node_GrantStmt:
		return extractGrantStmt(statement, node.GrantStmt), nil
	case *pg_query.Node_GrantRoleStmt:
		return extractGrantRoleStmt(statement, node.GrantRoleStmt), nil
	case *pg_query.Node_AlterDefaultPrivilegesStmt:
		return extractAlterDefaultPrivilegesStmt(statement, node.AlterDefaultPrivilegesStmt), nil
	case *pg_query.Node_CreatePolicyStmt:
		return extractCreatePolicyStmt(statement, node.CreatePolicyStmt), nil
	case *pg_query.Node_AlterPolicyStmt:
		return extractAlterPolicyStmt(statement, node.AlterPolicyStmt), nil
	case *pg_query.Node_CreateTrigStmt:
		return extractCreateTrigStmt(statement, node.CreateTrigStmt), nil
	case *pg_query.Node_CreateFunctionStmt:
		return extractCreateFunctionStmt(statement, node.CreateFunctionStmt), nil
	case *pg_query.Node_InsertStmt:
		statement.DML = extractInsert(node.InsertStmt)
		return statement, nil
	case *pg_query.Node_UpdateStmt:
		statement.DML = extractUpdate(node.UpdateStmt)
		return statement, nil
	case *pg_query.Node_DeleteStmt:
		statement.DML = extractDelete(node.DeleteStmt)
		return statement, nil
	default:
		return unsupportedStatement(statement, featureNameForNode(e.node), "postgresql statement type is not in the approved v1 subset"), nil
	}
}
