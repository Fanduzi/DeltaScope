//go:build postgresql

package postgresql

import (
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

func typeNameString(typeName *pg_query.TypeName) string {
	if typeName == nil {
		return ""
	}
	parts := make([]string, 0, len(typeName.GetNames()))
	for _, item := range typeName.GetNames() {
		if value := stringNodeValue(item); value != "" {
			parts = append(parts, value)
		}
	}
	return strings.ToLower(strings.Join(parts, "."))
}

func stringNodeValue(node *pg_query.Node) string {
	if node == nil {
		return ""
	}
	if value := node.GetString_(); value != nil {
		return value.GetSval()
	}
	return ""
}

func stringValuesFromNodes(nodes []*pg_query.Node) []string {
	values := make([]string, 0, len(nodes))
	for _, node := range nodes {
		if value := stringNodeValue(node); value != "" {
			values = append(values, value)
		}
	}
	return values
}

func columnRefsFromExpr(node *pg_query.Node) []string {
	seen := make(map[string]struct{})
	columns := make([]string, 0)
	collectColumnRefs(node, seen, &columns)
	return columns
}

func collectColumnRefs(node *pg_query.Node, seen map[string]struct{}, columns *[]string) {
	if node == nil {
		return
	}
	if columnRef := node.GetColumnRef(); columnRef != nil {
		if name := columnRefName(columnRef); name != "" {
			if _, ok := seen[name]; !ok {
				seen[name] = struct{}{}
				*columns = append(*columns, name)
			}
		}
		return
	}
	if expr := node.GetAExpr(); expr != nil {
		collectColumnRefs(expr.GetLexpr(), seen, columns)
		collectColumnRefs(expr.GetRexpr(), seen, columns)
		return
	}
	if boolExpr := node.GetBoolExpr(); boolExpr != nil {
		for _, arg := range boolExpr.GetArgs() {
			collectColumnRefs(arg, seen, columns)
		}
		return
	}
	if nullTest := node.GetNullTest(); nullTest != nil {
		collectColumnRefs(nullTest.GetArg(), seen, columns)
		return
	}
	if typeCast := node.GetTypeCast(); typeCast != nil {
		collectColumnRefs(typeCast.GetArg(), seen, columns)
		return
	}
	if coalesce := node.GetCoalesceExpr(); coalesce != nil {
		for _, arg := range coalesce.GetArgs() {
			collectColumnRefs(arg, seen, columns)
		}
		return
	}
	if indirection := node.GetAIndirection(); indirection != nil {
		collectColumnRefs(indirection.GetArg(), seen, columns)
		return
	}
	if function := node.GetFuncCall(); function != nil {
		for _, arg := range function.GetArgs() {
			collectColumnRefs(arg, seen, columns)
		}
	}
}

func columnRefName(ref *pg_query.ColumnRef) string {
	fields := ref.GetFields()
	if len(fields) == 0 {
		return ""
	}
	return stringNodeValue(fields[len(fields)-1])
}

func tableFromRangeVar(r *pg_query.RangeVar) *spec.Table {
	if r == nil || strings.TrimSpace(r.GetRelname()) == "" {
		return nil
	}
	return &spec.Table{Schema: r.GetSchemaname(), Name: r.GetRelname()}
}

func rangeVarName(r *pg_query.RangeVar) string {
	if r == nil {
		return ""
	}
	return r.GetRelname()
}

func rangeVarSchema(r *pg_query.RangeVar) string {
	if r == nil {
		return ""
	}
	return r.GetSchemaname()
}

func singleTableSlice(table *spec.Table) []spec.Table {
	if table == nil {
		return nil
	}
	return []spec.Table{*table}
}

func tableFromRelationNodeList(nodes []*pg_query.Node) *spec.Table {
	for _, node := range nodes {
		if relation := node.GetRangeVar(); relation != nil {
			return tableFromRangeVar(relation)
		}
	}
	return nil
}

func tableFromObjectName(nodes []*pg_query.Node) *spec.Table {
	parts := objectNameParts(nodes)
	if len(parts) == 0 {
		return nil
	}
	if len(parts) == 1 {
		return &spec.Table{Name: parts[0]}
	}
	return &spec.Table{Schema: parts[len(parts)-2], Name: parts[len(parts)-1]}
}

func objectNameFromObjectName(nodes []*pg_query.Node) string {
	parts := objectNameParts(nodes)
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

func objectNameParts(nodes []*pg_query.Node) []string {
	if len(nodes) == 0 {
		return nil
	}
	list := nodes[0].GetList()
	if list == nil {
		return nil
	}
	parts := make([]string, 0, len(list.GetItems()))
	for _, item := range list.GetItems() {
		if value := stringNodeValue(item); value != "" {
			parts = append(parts, value)
		}
	}
	if len(parts) == 0 {
		return nil
	}
	return parts
}

func unsupportedStatement(statement spec.Statement, feature string, reason string) spec.Statement {
	statement.Kind = spec.KindUnknown
	statement.DDL = nil
	statement.DML = nil
	statement.Unsupported = &spec.UnsupportedDetail{
		Feature: feature,
		Reason:  reason,
	}
	return statement
}

// unsupportedStatementWithDetail applies the full UnsupportedDetail (including
// Metadata) to the statement, preserving any structured context from the
// extraction path.
func unsupportedStatementWithDetail(statement spec.Statement, detail *spec.UnsupportedDetail) spec.Statement {
	statement.Kind = spec.KindUnknown
	statement.DDL = nil
	statement.DML = nil
	statement.Unsupported = detail
	return statement
}

func containsFold(items []string, target string) bool {
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item), strings.TrimSpace(target)) {
			return true
		}
	}
	return false
}

func columnPtr(column spec.Column) *spec.Column {
	return &column
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func featureNameForNode(node *pg_query.Node) string {
	if node == nil {
		return "unknown"
	}
	switch node.GetNode().(type) {
	case *pg_query.Node_SelectStmt:
		return "select"
	case *pg_query.Node_AlterTableStmt:
		return "alter_table"
	case *pg_query.Node_AlterObjectSchemaStmt:
		return "alter_table"
	case *pg_query.Node_CreateStmt:
		return "create_table"
	case *pg_query.Node_RenameStmt:
		return "rename"
	case *pg_query.Node_DropStmt:
		return "drop"
	case *pg_query.Node_IndexStmt:
		return "create_index"
	case *pg_query.Node_TruncateStmt:
		return "truncate"
	case *pg_query.Node_RefreshMatViewStmt:
		return "refresh_materialized_view"
	case *pg_query.Node_InsertStmt:
		return "insert"
	case *pg_query.Node_UpdateStmt:
		return "update"
	case *pg_query.Node_DeleteStmt:
		return "delete"
	case *pg_query.Node_CreateEnumStmt:
		return "create_type"
	case *pg_query.Node_AlterEnumStmt:
		return "alter_type"
	case *pg_query.Node_CompositeTypeStmt:
		return "create_type"
	case *pg_query.Node_CreateDomainStmt:
		return "create_domain"
	case *pg_query.Node_AlterDomainStmt:
		return "alter_domain"
	case *pg_query.Node_CreateExtensionStmt:
		return "create_extension"
	case *pg_query.Node_AlterExtensionStmt:
		return "alter_extension"
	case *pg_query.Node_AlterExtensionContentsStmt:
		return "alter_extension"
	case *pg_query.Node_GrantStmt:
		return "grant_table"
	case *pg_query.Node_GrantRoleStmt:
		return "grant_role"
	case *pg_query.Node_AlterDefaultPrivilegesStmt:
		return "alter_default_privileges"
	default:
		return "unknown"
	}
}

func featureNameForUnknown(rawSQL string) string {
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(rawSQL)))
	if len(fields) == 0 {
		return "unknown"
	}
	return fields[0]
}
