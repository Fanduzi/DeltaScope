//go:build postgresql

package postgresql

import (
	"fmt"
	"strconv"
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
	case *pg_query.Node_RenameStmt:
		return extractRenameStmt(statement, node.RenameStmt), nil
	case *pg_query.Node_DropStmt:
		return extractDropStmt(statement, node.DropStmt), nil
	case *pg_query.Node_IndexStmt:
		return extractIndexStmt(statement, node.IndexStmt), nil
	case *pg_query.Node_TruncateStmt:
		return extractTruncateStmt(statement, node.TruncateStmt), nil
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

func extractCreateStmt(statement spec.Statement, stmt *pg_query.CreateStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "create_table", "postgresql create statement payload is missing")
	}
	if stmt.GetPartspec() != nil || stmt.GetPartbound() != nil {
		return unsupportedStatement(statement, "partitioning", "postgresql partitioning features are unsupported in v1")
	}

	ddl := &spec.DDL{
		Operation:   spec.DDLOperationCreateTable,
		Table:       tableFromRangeVar(stmt.GetRelation()),
		Columns:     make([]spec.Column, 0, len(stmt.GetTableElts())),
		Indexes:     make([]spec.Index, 0),
		Constraints: make([]spec.Constraint, 0),
	}

	for _, item := range stmt.GetTableElts() {
		if column := item.GetColumnDef(); column != nil {
			if column.GetIdentity() != "" {
				return unsupportedStatement(statement, "generated_as_identity", "postgresql GENERATED ... AS IDENTITY is unsupported in v1")
			}
			if unsupported := hasUnsupportedColumnConstraint(column); unsupported != nil {
					return unsupportedStatement(statement, unsupported.Feature, unsupported.Reason)
				}
				ddl.Columns = append(ddl.Columns, columnFromDef(column))
				applyColumnConstraints(ddl, column)
			continue
		}
		if constraint := item.GetConstraint(); constraint != nil {
			if unsupported := applyTableConstraint(ddl, constraint); unsupported != nil {
					return unsupportedStatement(statement, unsupported.Feature, unsupported.Reason)
				}
		}
	}

	statement.DDL = ddl
	return statement
}

func extractViewStmt(statement spec.Statement, stmt *pg_query.ViewStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "create_view", "postgresql create view statement payload is missing")
	}
	if stmt.GetReplace() {
		return unsupportedStatement(statement, "create_view", "postgresql create or replace view is unsupported in v1")
	}
	if len(stmt.GetOptions()) > 0 {
		return unsupportedStatement(statement, "create_view", "postgresql create view options are unsupported in v1")
	}
	if stmt.GetWithCheckOption() != pg_query.ViewCheckOption_NO_CHECK_OPTION {
		return unsupportedStatement(statement, "create_view", "postgresql create view check options are unsupported in v1")
	}
	view := stmt.GetView()
	if view == nil {
		return unsupportedStatement(statement, "create_view", "postgresql create view target is missing")
	}
	if persistence := view.GetRelpersistence(); persistence != "" && persistence != "p" {
		return unsupportedStatement(statement, "create_view", "postgresql temporary view variants are unsupported in v1")
	}
	statement.DDL = &spec.DDL{
		Operation: spec.DDLOperationCreateView,
		Table:     tableFromRangeVar(view),
		HasSelect: stmt.GetQuery() != nil,
	}
	return statement
}

func extractAlterTableStmt(statement spec.Statement, stmt *pg_query.AlterTableStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "alter_table", "postgresql alter table statement payload is missing")
	}
	if stmt.GetObjtype() != pg_query.ObjectType_OBJECT_TYPE_UNDEFINED && stmt.GetObjtype() != pg_query.ObjectType_OBJECT_TABLE {
		return unsupportedStatement(statement, "alter_table", "postgresql alter table object type is unsupported in v1")
	}

	ddl := &spec.DDL{
		Operation: spec.DDLOperationAlterTable,
		Table:     tableFromRangeVar(stmt.GetRelation()),
		Alter:     make([]spec.Alter, 0, len(stmt.GetCmds())),
	}
	for _, item := range stmt.GetCmds() {
		cmd := item.GetAlterTableCmd()
		if cmd == nil {
			return unsupportedStatement(statement, "alter_table", "postgresql alter table command payload is missing")
		}
		alter, ok, unsupported := alterFromCmd(cmd)
		if unsupported != nil {
			return unsupportedStatement(statement, unsupported.Feature, unsupported.Reason)
		}
		if !ok {
			return unsupportedStatement(statement, "alter_table", "postgresql alter table command is unsupported in v1")
		}
		ddl.Alter = append(ddl.Alter, alter)
	}

	statement.DDL = ddl
	return statement
}

func extractRenameStmt(statement spec.Statement, stmt *pg_query.RenameStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "rename", "postgresql rename statement payload is missing")
	}
	if stmt.GetRelation() == nil {
		return unsupportedStatement(statement, "rename", "postgresql rename statement relation target is missing")
	}

	table := tableFromRangeVar(stmt.GetRelation())
	switch stmt.GetRenameType() {
	case pg_query.ObjectType_OBJECT_COLUMN:
		statement.DDL = &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     table,
			Alter: []spec.Alter{{
				Action: "rename_column",
				Name:   stmt.GetSubname(),
				Column: &spec.AlterColumn{
					OldName:    stmt.GetSubname(),
					Definition: &spec.Column{Name: stmt.GetNewname()},
				},
			}},
		}
		return statement
	case pg_query.ObjectType_OBJECT_TABLE:
		statement.DDL = &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     table,
			Alter:     []spec.Alter{{Action: "rename_table", Name: table.Name, Options: map[string]string{"new_name": stmt.GetNewname()}}},
		}
		return statement
	case pg_query.ObjectType_OBJECT_INDEX:
		options := map[string]string{"new_name": stmt.GetNewname()}
		if table != nil && strings.TrimSpace(table.Schema) != "" {
			options["schema"] = strings.TrimSpace(table.Schema)
		}
		statement.DDL = &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Alter: []spec.Alter{{
				Action:  "rename_index",
				Name:    table.Name,
				Options: options,
			}},
		}
		return statement
	default:
		return unsupportedStatement(statement, "rename", "postgresql rename target is not in the approved v1 subset")
	}
}

func extractDropStmt(statement spec.Statement, stmt *pg_query.DropStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "drop", "postgresql drop statement payload is missing")
	}
	switch stmt.GetRemoveType() {
	case pg_query.ObjectType_OBJECT_TABLE:
		statement.DDL = &spec.DDL{Operation: spec.DDLOperationDropTable, Table: tableFromObjectName(stmt.GetObjects())}
	case pg_query.ObjectType_OBJECT_VIEW:
		if len(stmt.GetObjects()) != 1 {
			return unsupportedStatement(statement, "drop", "postgresql multi-target drop view is unsupported in v1")
		}
		statement.DDL = &spec.DDL{
			Operation: spec.DDLOperationDropView,
			Table:     tableFromObjectName(stmt.GetObjects()),
			Options:   map[string]string{"if_exists": fmt.Sprintf("%t", stmt.GetMissingOk())},
		}
	case pg_query.ObjectType_OBJECT_INDEX:
		options := map[string]string{}
		indexTable := tableFromObjectName(stmt.GetObjects())
		if indexTable != nil && strings.TrimSpace(indexTable.Schema) != "" {
			options["schema"] = strings.TrimSpace(indexTable.Schema)
		}
		statement.DDL = &spec.DDL{
			Operation: spec.DDLOperationDropIndex,
			Alter:     []spec.Alter{{Action: "drop_index", Name: objectNameFromObjectName(stmt.GetObjects()), Options: options}},
		}
	default:
		return unsupportedStatement(statement, "drop", "postgresql drop target is not in the approved v1 subset")
	}
	return statement
}

func extractIndexStmt(statement spec.Statement, stmt *pg_query.IndexStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "create_index", "postgresql create index statement payload is missing")
	}
	if stmt.GetWhereClause() != nil {
		return unsupportedStatement(statement, "create_index", "postgresql partial index is unsupported in this milestone")
	}
	if len(stmt.GetIndexIncludingParams()) > 0 {
		return unsupportedStatement(statement, "create_index", "postgresql create index include clause is unsupported in this milestone")
	}
	if am := stmt.GetAccessMethod(); am != "" && am != "btree" {
		return unsupportedStatement(statement, "create_index", "postgresql create index with non-btree access method is unsupported in this milestone")
	}
	if stmt.GetNullsNotDistinct() {
		return unsupportedStatement(statement, "create_index", "postgresql create index nulls not distinct is unsupported in this milestone")
	}
	if hasExpressionIndexColumn(stmt.GetIndexParams()) {
		return unsupportedStatement(statement, "create_index", "postgresql expression index is unsupported in this milestone")
	}

	columns := indexColumnsFromIndexParams(stmt.GetIndexParams())
	kind := spec.IndexKindSecondary
	if stmt.GetUnique() {
		kind = spec.IndexKindUnique
	}

	statement.DDL = &spec.DDL{
		Operation: spec.DDLOperationCreateIndex,
		Table:     tableFromRangeVar(stmt.GetRelation()),
		Indexes: []spec.Index{
			{Name: stmt.GetIdxname(), Kind: kind, Columns: columns},
		},
		Options: map[string]string{
			"concurrently": strconv.FormatBool(stmt.GetConcurrent()),
		},
	}
	return statement
}

func hasExpressionIndexColumn(params []*pg_query.Node) bool {
	for _, param := range params {
		elem := param.GetIndexElem()
		if elem == nil {
			return true
		}
		if elem.GetExpr() != nil {
			return true
		}
	}
	return false
}

func indexColumnsFromIndexParams(params []*pg_query.Node) []string {
	columns := make([]string, 0, len(params))
	for _, param := range params {
		if name := param.GetIndexElem().GetName(); name != "" {
			columns = append(columns, name)
		}
	}
	return columns
}

func extractTruncateStmt(statement spec.Statement, stmt *pg_query.TruncateStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "truncate", "postgresql truncate statement payload is missing")
	}
	statement.DDL = &spec.DDL{Operation: spec.DDLOperationTruncateTable, Table: tableFromRelationNodeList(stmt.GetRelations())}
	return statement
}

func extractInsert(stmt *pg_query.InsertStmt) *spec.DML {
	return &spec.DML{
		Operation:      spec.DMLOperationInsert,
		Tables:         singleTableSlice(tableFromRangeVar(stmt.GetRelation())),
		IsInsertSelect: isInsertSelect(stmt),
		HasOnDuplicate: false,
	}
}

func isInsertSelect(stmt *pg_query.InsertStmt) bool {
	if stmt == nil || stmt.GetSelectStmt() == nil {
		return false
	}
	selectStmt := stmt.GetSelectStmt().GetSelectStmt()
	if selectStmt == nil {
		return true
	}
	return len(selectStmt.GetValuesLists()) == 0
}

func extractUpdate(stmt *pg_query.UpdateStmt) *spec.DML {
	hasJoin := len(stmt.GetFromClause()) > 0
	return &spec.DML{
		Operation:     spec.DMLOperationUpdate,
		Tables:        singleTableSlice(tableFromRangeVar(stmt.GetRelation())),
		HasWhere:      stmt.GetWhereClause() != nil,
		HasJoin:       hasJoin,
		IsSingleTable: !hasJoin,
	}
}

func extractDelete(stmt *pg_query.DeleteStmt) *spec.DML {
	hasJoin := len(stmt.GetUsingClause()) > 0
	return &spec.DML{
		Operation:     spec.DMLOperationDelete,
		Tables:        singleTableSlice(tableFromRangeVar(stmt.GetRelation())),
		HasWhere:      stmt.GetWhereClause() != nil,
		HasJoin:       hasJoin,
		IsSingleTable: !hasJoin,
	}
}

func alterFromCmd(cmd *pg_query.AlterTableCmd) (spec.Alter, bool, *spec.UnsupportedDetail) {
	switch cmd.GetSubtype() {
	case pg_query.AlterTableType_AT_AddColumn:
		column := cmd.GetDef().GetColumnDef()
		if column == nil {
			return spec.Alter{}, false, &spec.UnsupportedDetail{Feature: "alter_table", Reason: "postgresql add column payload is missing"}
		}
		if column.GetIdentity() != "" {
			return spec.Alter{}, false, &spec.UnsupportedDetail{Feature: "generated_as_identity", Reason: "postgresql GENERATED ... AS IDENTITY is unsupported in v1"}
		}
		return spec.Alter{Action: "add_column", Name: column.GetColname(), Column: &spec.AlterColumn{Definition: columnPtr(columnFromDef(column))}}, true, nil
	case pg_query.AlterTableType_AT_DropColumn:
		return spec.Alter{Action: "drop_column", Name: cmd.GetName(), Column: &spec.AlterColumn{OldName: cmd.GetName()}}, true, nil
	case pg_query.AlterTableType_AT_AddConstraint:
		constraint := cmd.GetDef().GetConstraint()
		if constraint == nil {
			return spec.Alter{}, false, &spec.UnsupportedDetail{Feature: "alter_table", Reason: "postgresql add constraint payload is missing"}
		}
		constraintType, ok := supportedConstraintType(constraint.GetContype())
		if !ok {
			return spec.Alter{}, false, &spec.UnsupportedDetail{Feature: "add_constraint", Reason: "postgresql constraint type is not in the approved v1 subset"}
		}
		options := map[string]string{
			"constraint_type": constraintType,
			"not_valid":       strconv.FormatBool(constraint.GetSkipValidation()),
		}
		return spec.Alter{Action: "add_constraint", Name: constraint.GetConname(), Options: options}, true, nil
	case pg_query.AlterTableType_AT_DropConstraint:
		return spec.Alter{Action: "drop_constraint", Name: cmd.GetName()}, true, nil
	case pg_query.AlterTableType_AT_ValidateConstraint:
		return spec.Alter{Action: "validate_constraint", Name: cmd.GetName()}, true, nil
	case pg_query.AlterTableType_AT_AlterColumnType:
		column := cmd.GetDef().GetColumnDef()
		if column == nil || column.GetTypeName() == nil {
			return spec.Alter{}, false, &spec.UnsupportedDetail{Feature: "alter_column_type", Reason: "postgresql alter column type payload is missing"}
		}
		return spec.Alter{Action: "set_data_type", Name: cmd.GetName(), Column: &spec.AlterColumn{OldName: cmd.GetName(), Definition: &spec.Column{Name: cmd.GetName(), Type: typeNameString(column.GetTypeName())}}}, true, nil
	case pg_query.AlterTableType_AT_ColumnDefault:
		action := "set_default"
		if cmd.GetDef() == nil {
			action = "drop_default"
		}
		return spec.Alter{Action: action, Name: cmd.GetName(), Column: &spec.AlterColumn{OldName: cmd.GetName(), Change: &spec.AlterColumnChange{TouchesDefault: true}}}, true, nil
	case pg_query.AlterTableType_AT_SetNotNull:
		return spec.Alter{Action: "set_not_null", Name: cmd.GetName(), Column: &spec.AlterColumn{OldName: cmd.GetName(), Definition: &spec.Column{Name: cmd.GetName(), NotNull: true}, Change: &spec.AlterColumnChange{TouchesNullability: true}}}, true, nil
	case pg_query.AlterTableType_AT_DropNotNull:
		return spec.Alter{Action: "drop_not_null", Name: cmd.GetName(), Column: &spec.AlterColumn{OldName: cmd.GetName(), Definition: &spec.Column{Name: cmd.GetName(), NotNull: false}, Change: &spec.AlterColumnChange{TouchesNullability: true}}}, true, nil
	default:
		return spec.Alter{}, false, &spec.UnsupportedDetail{Feature: alterSubtypeFeature(cmd.GetSubtype()), Reason: "postgresql alter table command is not in the approved v1 whitelist"}
	}
}

func supportedConstraintType(kind pg_query.ConstrType) (string, bool) {
	switch kind {
	case pg_query.ConstrType_CONSTR_PRIMARY:
		return "primary_key", true
	case pg_query.ConstrType_CONSTR_UNIQUE:
		return "unique", true
	case pg_query.ConstrType_CONSTR_FOREIGN:
		return "foreign_key", true
	case pg_query.ConstrType_CONSTR_CHECK:
		return "check", true
	default:
		return "", false
	}
}

func applyTableConstraint(ddl *spec.DDL, constraint *pg_query.Constraint) *spec.UnsupportedDetail {
	switch constraint.GetContype() {
	case pg_query.ConstrType_CONSTR_UNIQUE:
		ddl.Indexes = append(ddl.Indexes, spec.Index{Name: constraint.GetConname(), Kind: spec.IndexKindUnique, Columns: stringValuesFromNodes(constraint.GetKeys())})
	case pg_query.ConstrType_CONSTR_FOREIGN:
		ddl.Constraints = append(ddl.Constraints, spec.Constraint{
			Type:              "foreign_key",
			Name:              constraint.GetConname(),
			Columns:           stringValuesFromNodes(constraint.GetFkAttrs()),
			ReferencedTable:   rangeVarName(constraint.GetPktable()),
			ReferencedColumns: stringValuesFromNodes(constraint.GetPkAttrs()),
		})
	case pg_query.ConstrType_CONSTR_CHECK:
		ddl.Constraints = append(ddl.Constraints, spec.Constraint{Type: "check", Name: constraint.GetConname(), Columns: columnRefsFromExpr(constraint.GetRawExpr())})
	case pg_query.ConstrType_CONSTR_EXCLUSION:
		return &spec.UnsupportedDetail{Feature: "exclusion_constraint", Reason: "postgresql EXCLUDE constraints are unsupported in v1"}
	}
	return nil
}

func applyColumnConstraints(ddl *spec.DDL, column *pg_query.ColumnDef) {
	for _, item := range column.GetConstraints() {
		constraint := item.GetConstraint()
		if constraint == nil {
			continue
		}
		switch constraint.GetContype() {
		case pg_query.ConstrType_CONSTR_UNIQUE:
			ddl.Indexes = append(ddl.Indexes, spec.Index{Name: constraint.GetConname(), Kind: spec.IndexKindUnique, Columns: []string{column.GetColname()}})
		case pg_query.ConstrType_CONSTR_FOREIGN:
			ddl.Constraints = append(ddl.Constraints, spec.Constraint{
				Type:              "foreign_key",
				Name:              constraint.GetConname(),
				Columns:           []string{column.GetColname()},
				ReferencedTable:   rangeVarName(constraint.GetPktable()),
				ReferencedColumns: stringValuesFromNodes(constraint.GetPkAttrs()),
			})
		case pg_query.ConstrType_CONSTR_CHECK:
			ddl.Constraints = append(ddl.Constraints, spec.Constraint{Type: "check", Name: constraint.GetConname(), Columns: columnRefsFromExpr(constraint.GetRawExpr())})
		}
	}
}

func hasUnsupportedColumnConstraint(column *pg_query.ColumnDef) *spec.UnsupportedDetail {
	for _, item := range column.GetConstraints() {
		constraint := item.GetConstraint()
		if constraint == nil {
			continue
		}
		switch constraint.GetContype() {
		case pg_query.ConstrType_CONSTR_IDENTITY:
			return &spec.UnsupportedDetail{Feature: "generated_as_identity", Reason: "postgresql GENERATED ... AS IDENTITY is unsupported in v1"}
		case pg_query.ConstrType_CONSTR_GENERATED:
			return &spec.UnsupportedDetail{Feature: "generated_column", Reason: "postgresql GENERATED ALWAYS AS ... STORED columns are unsupported in v1"}
		}
	}
	return nil
}

func columnFromDef(column *pg_query.ColumnDef) spec.Column {
	result := spec.Column{
		Name:       column.GetColname(),
		Type:       typeNameString(column.GetTypeName()),
		NotNull:    column.GetIsNotNull(),
		HasDefault: column.GetRawDefault() != nil || column.GetCookedDefault() != nil,
	}
	return result
}

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

func featureNameForNode(node *pg_query.Node) string {
	if node == nil {
		return "unknown"
	}
	switch node.GetNode().(type) {
	case *pg_query.Node_SelectStmt:
		return "select"
	case *pg_query.Node_AlterTableStmt:
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
	case *pg_query.Node_InsertStmt:
		return "insert"
	case *pg_query.Node_UpdateStmt:
		return "update"
	case *pg_query.Node_DeleteStmt:
		return "delete"
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

func alterSubtypeFeature(subtype pg_query.AlterTableType) string {
	name := subtype.String()
	if strings.HasPrefix(name, "AT_") {
		name = strings.TrimPrefix(name, "AT_")
	}
	if name == "" || name == "ALTER_TABLE_TYPE_UNDEFINED" {
		return "alter_table"
	}
	return strings.ToLower(name)
}

func columnPtr(column spec.Column) *spec.Column {
	return &column
}

var _ = fmt.Sprintf
