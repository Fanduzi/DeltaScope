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
		return unsupportedStatement(statement, "create_type_composite", "postgresql composite type creation is not in the approved v0.55.0 subset"), nil
	case *pg_query.Node_CreateDomainStmt:
		return unsupportedStatement(statement, "create_domain", "postgresql domain creation is not in the approved v0.55.0 subset"), nil
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
			if unsupported := hasUnsupportedColumnConstraint(column); unsupported != nil {
					return unsupportedStatementWithDetail(statement, unsupported)
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
			return unsupportedStatementWithDetail(statement, unsupported)
		}
		if !ok {
			return unsupportedStatement(statement, "alter_table", "postgresql alter table command is unsupported in v1")
		}
		ddl.Alter = append(ddl.Alter, alter)
			projectAlterConstraintFK(ddl, alter)
			projectAlterConstraintCheck(ddl, alter)
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

func extractAlterObjectSchemaStmt(statement spec.Statement, stmt *pg_query.AlterObjectSchemaStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "alter_table", "postgresql alter object schema statement payload is missing")
	}
	statement.DDL = &spec.DDL{
		Operation: spec.DDLOperationAlterTable,
		Table:     tableFromRangeVar(stmt.GetRelation()),
		Alter: []spec.Alter{{
			Action:  "set_schema",
			Options: map[string]string{"new_schema": stmt.GetNewschema()},
		}},
	}
	return statement
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
	case pg_query.ObjectType_OBJECT_SCHEMA:
		options := map[string]string{"if_exists": fmt.Sprintf("%t", stmt.GetMissingOk())}
		if stmt.GetBehavior() == pg_query.DropBehavior_DROP_CASCADE {
			options["cascade"] = "true"
		}
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationDropSchema,
			ObjectName: dropTargetName(stmt),
			ObjectType: "schema",
			Options:    options,
		}
	case pg_query.ObjectType_OBJECT_SEQUENCE:
		options := map[string]string{"if_exists": fmt.Sprintf("%t", stmt.GetMissingOk())}
		if stmt.GetBehavior() == pg_query.DropBehavior_DROP_CASCADE {
			options["cascade"] = "true"
		}
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationDropSequence,
			ObjectName: dropTargetName(stmt),
			ObjectType: "sequence",
			Options:    options,
		}
	case pg_query.ObjectType_OBJECT_MATVIEW:
		options := map[string]string{"if_exists": fmt.Sprintf("%t", stmt.GetMissingOk())}
		if stmt.GetBehavior() == pg_query.DropBehavior_DROP_CASCADE {
			options["cascade"] = "true"
		}
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationDropMaterializedView,
			ObjectName: dropTargetName(stmt),
			ObjectType: "materialized_view",
			Options:    options,
		}
	case pg_query.ObjectType_OBJECT_TYPE:
		options := map[string]string{"if_exists": fmt.Sprintf("%t", stmt.GetMissingOk())}
		if stmt.GetBehavior() == pg_query.DropBehavior_DROP_CASCADE {
			options["cascade"] = "true"
		}
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationDropType,
			ObjectName: dropTypeNameFromObjects(stmt.GetObjects()),
			ObjectType: "type",
			Options:    options,
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
	if stmt.GetNullsNotDistinct() {
		return unsupportedStatement(statement, "create_index", "postgresql create index nulls not distinct is unsupported in this milestone")
	}

	columns := indexColumnsFromIndexParams(stmt.GetIndexParams())
	kind := spec.IndexKindSecondary
	if stmt.GetUnique() {
		kind = spec.IndexKindUnique
	}

	accessMethod := stmt.GetAccessMethod()
	if accessMethod == "" {
		accessMethod = "btree"
	}

	exprCount := expressionIndexElemCount(stmt.GetIndexParams())
	includedColumns := indexElemNames(stmt.GetIndexIncludingParams())

	statement.DDL = &spec.DDL{
		Operation: spec.DDLOperationCreateIndex,
		Table:     tableFromRangeVar(stmt.GetRelation()),
		Indexes: []spec.Index{
			{
				Name:              stmt.GetIdxname(),
				Kind:              kind,
				Columns:           columns,
				AccessMethod:      accessMethod,
				IncludedColumns:   includedColumns,
				HasPredicate:      stmt.GetWhereClause() != nil,
				HasExpressionKeys: exprCount > 0,
				ExpressionCount:   exprCount,
			},
		},
		Options: map[string]string{
			"concurrently": strconv.FormatBool(stmt.GetConcurrent()),
		},
	}
	return statement
}

func expressionIndexElemCount(params []*pg_query.Node) int {
	count := 0
	for _, n := range params {
		elem := n.GetIndexElem()
		if elem == nil || elem.GetExpr() != nil {
			count++
		}
	}
	return count
}

func indexElemNames(nodes []*pg_query.Node) []string {
	var names []string
	for _, n := range nodes {
		elem := n.GetIndexElem()
		if elem != nil && elem.GetName() != "" {
			names = append(names, elem.GetName())
		}
	}
	return names
}

func indexColumnsFromIndexParams(params []*pg_query.Node) []string {
	return indexElemNames(params)
}

func extractTruncateStmt(statement spec.Statement, stmt *pg_query.TruncateStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "truncate", "postgresql truncate statement payload is missing")
	}
	statement.DDL = &spec.DDL{Operation: spec.DDLOperationTruncateTable, Table: tableFromRelationNodeList(stmt.GetRelations())}
	return statement
}

func extractCreateSchemaStmt(statement spec.Statement, stmt *pg_query.CreateSchemaStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "create_schema", "postgresql create schema statement payload is missing")
	}
	options := map[string]string{}
	if stmt.GetIfNotExists() {
		options["if_not_exists"] = "true"
	}
	statement.DDL = &spec.DDL{
		Operation:  spec.DDLOperationCreateSchema,
		ObjectName: stmt.GetSchemaname(),
		ObjectType: "schema",
		Options:    options,
	}
	return statement
}

func extractCreateSeqStmt(statement spec.Statement, stmt *pg_query.CreateSeqStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "create_sequence", "postgresql create sequence statement payload is missing")
	}
	options := map[string]string{}
	if stmt.GetIfNotExists() {
		options["if_not_exists"] = "true"
	}
	for _, opt := range stmt.GetOptions() {
		elem := opt.GetDefElem()
		if elem == nil {
			continue
		}
		if elem.GetDefname() == "cycle" {
			if arg := elem.GetArg(); arg != nil {
				if b := arg.GetBoolean(); b != nil && b.GetBoolval() {
					options["cycle"] = "true"
				}
			}
		}
	}
	statement.DDL = &spec.DDL{
		Operation:  spec.DDLOperationCreateSequence,
		ObjectName: rangeVarName(stmt.GetSequence()),
		ObjectType: "sequence",
		Options:    options,
	}
	return statement
}

func extractAlterSeqStmt(statement spec.Statement, stmt *pg_query.AlterSeqStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "alter_sequence", "postgresql alter sequence statement payload is missing")
	}
	options := map[string]string{}
	for _, opt := range stmt.GetOptions() {
		elem := opt.GetDefElem()
		if elem == nil {
			continue
		}
		switch elem.GetDefname() {
		case "restart":
			options["restart"] = "true"
		case "cycle":
			if arg := elem.GetArg(); arg != nil {
				if b := arg.GetBoolean(); b != nil && b.GetBoolval() {
					options["cycle"] = "true"
				}
			}
		}
	}
	statement.DDL = &spec.DDL{
		Operation:  spec.DDLOperationAlterSequence,
		ObjectName: rangeVarName(stmt.GetSequence()),
		ObjectType: "sequence",
		Options:    options,
	}
	return statement
}

func extractCreateTableAsStmt(statement spec.Statement, stmt *pg_query.CreateTableAsStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "create_table_as", "postgresql create table as statement payload is missing")
	}
	if stmt.GetObjtype() != pg_query.ObjectType_OBJECT_MATVIEW {
		return unsupportedStatement(statement, "create_table_as", "postgresql create table as select is unsupported in v1")
	}
	into := stmt.GetInto()
	relName := ""
	skipData := false
	if into != nil {
		relName = rangeVarName(into.GetRel())
		skipData = into.GetSkipData()
	}
	options := map[string]string{}
	if skipData {
		options["with_no_data"] = "true"
	}
	statement.DDL = &spec.DDL{
		Operation:  spec.DDLOperationCreateMaterializedView,
		ObjectName: relName,
		ObjectType: "materialized_view",
		HasSelect:  stmt.GetQuery() != nil,
		Options:    options,
	}
	return statement
}

func extractRefreshMatViewStmt(statement spec.Statement, stmt *pg_query.RefreshMatViewStmt) spec.Statement {
	if stmt == nil || stmt.GetRelation() == nil {
		return unsupportedStatement(statement, "refresh_materialized_view", "postgresql refresh materialized view target is missing")
	}
	statement.DDL = &spec.DDL{
		Operation:  spec.DDLOperationRefreshMaterializedView,
		ObjectName: rangeVarName(stmt.GetRelation()),
		ObjectType: "materialized_view",
		Options: map[string]string{
			"concurrently": strconv.FormatBool(stmt.GetConcurrent()),
			"with_no_data": strconv.FormatBool(stmt.GetSkipData()),
		},
	}
	return statement
}

func dropTargetName(stmt *pg_query.DropStmt) string {
	if name := objectNameFromObjectName(stmt.GetObjects()); name != "" {
		return name
	}
	for _, obj := range stmt.GetObjects() {
		if s := obj.GetString_(); s != nil && s.GetSval() != "" {
			return s.GetSval()
		}
	}
	return ""
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
		if unsupported := hasUnsupportedColumnConstraint(column); unsupported != nil {
			return spec.Alter{}, false, unsupported
		}
		return spec.Alter{Action: "add_column", Name: column.GetColname(), Column: &spec.AlterColumn{Definition: columnPtr(columnFromDef(column))}}, true, nil
	case pg_query.AlterTableType_AT_DropColumn:
		return spec.Alter{Action: "drop_column", Name: cmd.GetName(), Column: &spec.AlterColumn{OldName: cmd.GetName()}}, true, nil
	case pg_query.AlterTableType_AT_DropExpression:
		return spec.Alter{Action: "drop_expression", Name: cmd.GetName(), Column: &spec.AlterColumn{OldName: cmd.GetName()}}, true, nil
	case pg_query.AlterTableType_AT_SetIdentity:
		generatedWhen := generatedWhenFromDef(cmd.GetDef())
		return spec.Alter{
			Action: "set_generated",
			Name:   cmd.GetName(),
			Column: &spec.AlterColumn{OldName: cmd.GetName()},
			Options: map[string]string{
				"generated_when": generatedWhen,
			},
		}, true, nil
	case pg_query.AlterTableType_AT_DropIdentity:
		return spec.Alter{Action: "drop_identity", Name: cmd.GetName(), Column: &spec.AlterColumn{OldName: cmd.GetName()}}, true, nil
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
		if constraint.GetContype() == pg_query.ConstrType_CONSTR_FOREIGN {
			if cols := stringValuesFromNodes(constraint.GetFkAttrs()); len(cols) > 0 {
				options["columns"] = strings.Join(cols, ",")
			}
			if refTable := rangeVarName(constraint.GetPktable()); refTable != "" {
				options["referenced_table"] = refTable
			}
			if refCols := stringValuesFromNodes(constraint.GetPkAttrs()); len(refCols) > 0 {
				options["referenced_columns"] = strings.Join(refCols, ",")
			}
			if refSchema := rangeVarSchema(constraint.GetPktable()); refSchema != "" {
				options["referenced_schema"] = refSchema
			}
		} else if constraint.GetContype() == pg_query.ConstrType_CONSTR_CHECK {
			if cols := columnRefsFromExpr(constraint.GetRawExpr()); len(cols) > 0 {
				options["columns"] = strings.Join(cols, ",")
			}
		} else if cols := stringValuesFromNodes(constraint.GetKeys()); len(cols) > 0 {
			options["columns"] = strings.Join(cols, ",")
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
		alter := spec.Alter{Action: "set_data_type", Name: cmd.GetName(), Column: &spec.AlterColumn{OldName: cmd.GetName(), Definition: &spec.Column{Name: cmd.GetName(), Type: typeNameString(column.GetTypeName())}}}
		if column.GetRawDefault() != nil {
			alter.Options = map[string]string{"has_using": "true"}
		}
		return alter, true, nil
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
	case pg_query.AlterTableType_AT_ChangeOwner:
		owner := ""
		if no := cmd.GetNewowner(); no != nil {
			owner = no.GetRolename()
		}
		if owner == "" {
			return spec.Alter{}, false, &spec.UnsupportedDetail{Feature: "changeowner", Reason: "postgresql alter table owner role is missing"}
		}
		return spec.Alter{Action: "change_owner", Options: map[string]string{"owner": owner}}, true, nil
	case pg_query.AlterTableType_AT_EnableTrig:
		name := cmd.GetName()
		if name != "" {
			return spec.Alter{Action: "enable_trigger", Name: name, Options: map[string]string{"trigger": name}}, true, nil
		}
		return spec.Alter{Action: "enable_trigger", Options: map[string]string{"trigger_scope": "user"}}, true, nil
	case pg_query.AlterTableType_AT_EnableTrigAll:
		return spec.Alter{Action: "enable_trigger", Options: map[string]string{"trigger_scope": "all"}}, true, nil
	case pg_query.AlterTableType_AT_EnableTrigUser:
		return spec.Alter{Action: "enable_trigger", Options: map[string]string{"trigger_scope": "user"}}, true, nil
	case pg_query.AlterTableType_AT_DisableTrig:
		name := cmd.GetName()
		if name != "" {
			return spec.Alter{Action: "disable_trigger", Name: name, Options: map[string]string{"trigger": name}}, true, nil
		}
		return spec.Alter{Action: "disable_trigger", Options: map[string]string{"trigger_scope": "user"}}, true, nil
	case pg_query.AlterTableType_AT_DisableTrigAll:
		return spec.Alter{Action: "disable_trigger", Options: map[string]string{"trigger_scope": "all"}}, true, nil
	case pg_query.AlterTableType_AT_DisableTrigUser:
		return spec.Alter{Action: "disable_trigger", Options: map[string]string{"trigger_scope": "user"}}, true, nil
	case pg_query.AlterTableType_AT_ReplicaIdentity:
		identity, indexName := replicaIdentityFromDef(cmd.GetDef())
		if identity == "" {
			return spec.Alter{}, false, &spec.UnsupportedDetail{Feature: "replicaidentity", Reason: "postgresql alter table replica identity payload is missing"}
		}
		options := map[string]string{"identity": identity}
		if indexName != "" {
			options["index"] = indexName
		}
		return spec.Alter{Action: "replica_identity", Name: indexName, Options: options}, true, nil
	case pg_query.AlterTableType_AT_AttachPartition:
		partName := ""
		hasBounds := false
		if def := cmd.GetDef(); def != nil {
			if pc := def.GetPartitionCmd(); pc != nil {
				if rv := pc.GetName(); rv != nil {
					partName = rv.GetRelname()
				}
				hasBounds = pc.GetBound() != nil
			}
		}
		if partName == "" {
			return spec.Alter{}, false, &spec.UnsupportedDetail{Feature: "attachpartition", Reason: "postgresql alter table attach partition name is missing"}
		}
		return spec.Alter{Action: "attach_partition", Name: partName, Options: map[string]string{"partition": partName, "has_bounds": strconv.FormatBool(hasBounds)}}, true, nil
	case pg_query.AlterTableType_AT_DetachPartition:
		partName := ""
		if def := cmd.GetDef(); def != nil {
			if pc := def.GetPartitionCmd(); pc != nil {
				if rv := pc.GetName(); rv != nil {
					partName = rv.GetRelname()
				}
			}
		}
		if partName == "" {
			return spec.Alter{}, false, &spec.UnsupportedDetail{Feature: "detachpartition", Reason: "postgresql alter table detach partition name is missing"}
		}
		return spec.Alter{Action: "detach_partition", Name: partName, Options: map[string]string{"partition": partName}}, true, nil
	case pg_query.AlterTableType_AT_SetLogged:
		return spec.Alter{Action: "set_logged", Options: map[string]string{"logged": "true"}}, true, nil
	case pg_query.AlterTableType_AT_SetUnLogged:
		return spec.Alter{Action: "set_unlogged", Options: map[string]string{"logged": "false"}}, true, nil
	case pg_query.AlterTableType_AT_SetTableSpace:
		return spec.Alter{}, false, &spec.UnsupportedDetail{Feature: "set_tablespace", Reason: "postgresql alter table set tablespace is not in the approved v1 whitelist"}
	default:
		return spec.Alter{}, false, &spec.UnsupportedDetail{Feature: alterSubtypeFeature(cmd.GetSubtype()), Reason: "postgresql alter table command is not in the approved v1 whitelist"}
	}
}

// generatedWhenFromDef extracts the generated mode from an AT_SetIdentity
// def payload. PostgreSQL encodes ALWAYS as 97 ('a') and BY DEFAULT as 100 ('d').
func generatedWhenFromDef(defNode *pg_query.Node) string {
	if defNode == nil {
		return ""
	}
	listNode := defNode.GetList()
	if listNode == nil {
		return ""
	}
	for _, item := range listNode.GetItems() {
		defElem := item.GetDefElem()
		if defElem == nil || defElem.GetDefname() != "generated" {
			continue
		}
		arg := defElem.GetArg()
		if arg != nil && arg.GetInteger() != nil {
			return string(rune(arg.GetInteger().GetIval()))
		}
	}
	return ""
}

func replicaIdentityFromDef(defNode *pg_query.Node) (identity string, indexName string) {
	if defNode == nil {
		return "", ""
	}
	riNode, ok := defNode.GetNode().(*pg_query.Node_ReplicaIdentityStmt)
	if !ok || riNode.ReplicaIdentityStmt == nil {
		return "", ""
	}
	ri := riNode.ReplicaIdentityStmt
	switch ri.GetIdentityType() {
	case "d":
		return "default", ""
	case "f":
		return "full", ""
	case "n":
		return "nothing", ""
	case "i":
		return "using_index", ri.GetName()
	default:
		return "", ""
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
			ReferencedSchema:  rangeVarSchema(constraint.GetPktable()),
			ReferencedTable:   rangeVarName(constraint.GetPktable()),
			ReferencedColumns: stringValuesFromNodes(constraint.GetPkAttrs()),
			})
	case pg_query.ConstrType_CONSTR_PRIMARY:
		applyPrimaryKey(ddl, constraint.GetConname(), stringValuesFromNodes(constraint.GetKeys()))
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
				ReferencedSchema:  rangeVarSchema(constraint.GetPktable()),
				ReferencedTable:   rangeVarName(constraint.GetPktable()),
				ReferencedColumns: stringValuesFromNodes(constraint.GetPkAttrs()),
			})
		case pg_query.ConstrType_CONSTR_PRIMARY:
			applyPrimaryKey(ddl, constraint.GetConname(), []string{column.GetColname()})
		case pg_query.ConstrType_CONSTR_CHECK:
			ddl.Constraints = append(ddl.Constraints, spec.Constraint{Type: "check", Name: constraint.GetConname(), Columns: columnRefsFromExpr(constraint.GetRawExpr())})
		}
	}
}

func applyPrimaryKey(ddl *spec.DDL, name string, columns []string) {
	if ddl == nil || len(columns) == 0 {
		return
	}
	constraintName := strings.TrimSpace(name)
	if constraintName == "" {
		constraintName = "primary"
	}
	ddl.PrimaryKey = &spec.Index{
		Name:    constraintName,
		Kind:    spec.IndexKindPrimary,
		Columns: columns,
	}
	markPrimaryKeyColumnsNotNull(ddl, columns)
}

func markPrimaryKeyColumnsNotNull(ddl *spec.DDL, columns []string) {
	if ddl == nil || len(columns) == 0 {
		return
	}
	for i := range ddl.Columns {
		if containsFold(columns, ddl.Columns[i].Name) {
			ddl.Columns[i].NotNull = true
		}
	}
}

func containsFold(items []string, target string) bool {
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item), strings.TrimSpace(target)) {
			return true
		}
	}
	return false
}

func hasUnsupportedColumnConstraint(column *pg_query.ColumnDef) *spec.UnsupportedDetail {
	return nil
}

func columnFromDef(column *pg_query.ColumnDef) spec.Column {
	result := spec.Column{
		Name:       column.GetColname(),
		Type:       typeNameString(column.GetTypeName()),
		NotNull:    column.GetIsNotNull() || hasColumnConstraint(column, pg_query.ConstrType_CONSTR_NOTNULL),
		HasDefault: column.GetRawDefault() != nil || column.GetCookedDefault() != nil || hasColumnConstraint(column, pg_query.ConstrType_CONSTR_DEFAULT),
	}
	applyGeneratedIdentityFacts(&result, column)
	return result
}

func hasColumnConstraint(column *pg_query.ColumnDef, wantType pg_query.ConstrType) bool {
	for _, item := range column.GetConstraints() {
		if c := item.GetConstraint(); c != nil && c.GetContype() == wantType {
			return true
		}
	}
	return false
}

// applyGeneratedIdentityFacts extracts GeneratedWhen, IsIdentity, and
// IdentityOptions from CONSTR_GENERATED / CONSTR_IDENTITY constraints and
// writes them into the spec.Column.
func applyGeneratedIdentityFacts(col *spec.Column, column *pg_query.ColumnDef) {
	for _, item := range column.GetConstraints() {
		constraint := item.GetConstraint()
		if constraint == nil {
			continue
		}
		switch constraint.GetContype() {
		case pg_query.ConstrType_CONSTR_GENERATED:
			col.GeneratedWhen = constraint.GetGeneratedWhen()
		case pg_query.ConstrType_CONSTR_IDENTITY:
			col.IsIdentity = true
			col.GeneratedWhen = constraint.GetGeneratedWhen()
			if opts := identityOptionsFromConstraint(constraint); len(opts) > 0 {
				col.IdentityOptions = opts
			}
		}
	}
}

// identityOptionsFromConstraint extracts identity sequence options from a
// CONSTR_IDENTITY constraint's Options list (DefElem nodes). Numeric options
// are int32; CYCLE is bool.
func identityOptionsFromConstraint(constraint *pg_query.Constraint) map[string]any {
	options := constraint.GetOptions()
	if len(options) == 0 {
		return nil
	}
	result := make(map[string]any, len(options))
	for _, opt := range options {
		defElem := opt.GetDefElem()
		if defElem == nil {
			continue
		}
		name := defElem.GetDefname()
		argNode := defElem.GetArg()
		if argNode == nil {
			continue
		}
		if intNode := argNode.GetInteger(); intNode != nil {
			result[name] = intNode.GetIval()
			continue
		}
		if boolNode := argNode.GetBoolean(); boolNode != nil {
			result[name] = boolNode.GetBoolval()
			continue
		}
	}
	return result
}

// generatedIdentityUnsupportedMetadata builds UnsupportedDetail.Metadata for
// unsupported generated/identity column outcomes, carrying the column name,
// generated_when, is_identity flag, and identity options when present.
func generatedIdentityUnsupportedMetadata(column *pg_query.ColumnDef) map[string]any {
	metadata := map[string]any{
		"column": column.GetColname(),
	}
	for _, item := range column.GetConstraints() {
		constraint := item.GetConstraint()
		if constraint == nil {
			continue
		}
		switch constraint.GetContype() {
		case pg_query.ConstrType_CONSTR_GENERATED, pg_query.ConstrType_CONSTR_IDENTITY:
			metadata["generated_when"] = constraint.GetGeneratedWhen()
		}
		if constraint.GetContype() == pg_query.ConstrType_CONSTR_IDENTITY {
			metadata["is_identity"] = true
			if opts := identityOptionsFromConstraint(constraint); len(opts) > 0 {
				metadata["identity_options"] = opts
			}
		}
	}
	return metadata
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

func extractCreateEnumStmt(statement spec.Statement, stmt *pg_query.CreateEnumStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "create_type", "postgresql create enum type statement payload is missing")
	}
	objectName := firstStringFromTypeNameNodes(stmt.GetTypeName())
	if objectName == "" {
		return unsupportedStatement(statement, "create_type", "postgresql create enum type name is missing")
	}
	var labels []string
	for _, v := range stmt.GetVals() {
		if s := v.GetString_(); s != nil && s.GetSval() != "" {
			labels = append(labels, s.GetSval())
		}
	}
	options := map[string]string{"type_kind": "enum"}
	if len(labels) > 0 {
		options["labels"] = strings.Join(labels, ",")
	}
	statement.DDL = &spec.DDL{
		Operation:  spec.DDLOperationCreateType,
		ObjectName: objectName,
		ObjectType: "type",
		Options:    options,
	}
	return statement
}

func extractAlterEnumStmt(statement spec.Statement, stmt *pg_query.AlterEnumStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "alter_type", "postgresql alter enum type statement payload is missing")
	}
	objectName := firstStringFromTypeNameNodes(stmt.GetTypeName())
	if objectName == "" {
		return unsupportedStatement(statement, "alter_type", "postgresql alter enum type name is missing")
	}
	newVal := stmt.GetNewVal()
	if newVal == "" {
		return unsupportedStatement(statement, "alter_type", "postgresql alter enum type new value is missing")
	}
	options := map[string]string{
		"type_kind":     "enum",
		"action":        "add_value",
		"value":         newVal,
		"if_not_exists": strconv.FormatBool(stmt.GetSkipIfNewValExists()),
	}
	neighbor := stmt.GetNewValNeighbor()
	if neighbor != "" {
		if stmt.GetNewValIsAfter() {
			options["placement"] = "after"
		} else {
			options["placement"] = "before"
		}
		options["neighbor"] = neighbor
	}
	statement.DDL = &spec.DDL{
		Operation:  spec.DDLOperationAlterType,
		ObjectName: objectName,
		ObjectType: "type",
		Options:    options,
	}
	return statement
}

func firstStringFromTypeNameNodes(nodes []*pg_query.Node) string {
	for _, n := range nodes {
		if s := n.GetString_(); s != nil && s.GetSval() != "" {
			return s.GetSval()
		}
	}
	return ""
}

func dropTypeNameFromObjects(objects []*pg_query.Node) string {
	for _, obj := range objects {
		tn := obj.GetTypeName()
		if tn == nil {
			continue
		}
		for _, name := range tn.GetNames() {
			if s := name.GetString_(); s != nil && s.GetSval() != "" {
				return s.GetSval()
			}
		}
	}
	return ""
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
		return "create_type_composite"
	case *pg_query.Node_CreateDomainStmt:
		return "create_domain"
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

func projectAlterConstraintFK(ddl *spec.DDL, alter spec.Alter) {
	if alter.Action != "add_constraint" || alter.Options["constraint_type"] != "foreign_key" {
		return
	}
	cols := splitCSV(alter.Options["columns"])
	if len(cols) == 0 {
		return
	}
	ddl.Constraints = append(ddl.Constraints, spec.Constraint{
		Type:              "foreign_key",
		Name:              alter.Name,
		Columns:           cols,
		ReferencedSchema:  alter.Options["referenced_schema"],
		ReferencedTable:   alter.Options["referenced_table"],
		ReferencedColumns: splitCSV(alter.Options["referenced_columns"]),
	})
}

func projectAlterConstraintCheck(ddl *spec.DDL, alter spec.Alter) {
	if alter.Action != "add_constraint" || alter.Options["constraint_type"] != "check" {
		return
	}
	ddl.Constraints = append(ddl.Constraints, spec.Constraint{
		Type:    "check",
		Name:    alter.Name,
		Columns: splitCSV(alter.Options["columns"]),
	})
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
