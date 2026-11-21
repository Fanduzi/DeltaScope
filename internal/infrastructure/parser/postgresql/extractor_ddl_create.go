//go:build postgresql

package postgresql

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

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
		if like := item.GetTableLikeClause(); like != nil {
			relation := like.GetRelation()
			if source := tableFromRangeVar(relation); source != nil {
				ddl.PGCreateLike = append(ddl.PGCreateLike, spec.PGCreateLikeSource{
					Target: spec.PGDestructiveTableTarget{Table: *source, CatalogQualified: relation.GetCatalogname() != ""}, Options: like.GetOptions(),
				})
			}
			continue
		}
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

	if ddl.Table != nil {
		if source := stmt.GetOfTypename(); source != nil {
			typed := &spec.PGCreateTyped{
				Target:      spec.PGDestructiveTableTarget{Table: *ddl.Table, CatalogQualified: stmt.GetRelation().GetCatalogname() != ""},
				Temporary:   stmt.GetRelation().GetRelpersistence() == "t",
				IfNotExists: stmt.GetIfNotExists(),
			}
			names := source.GetNames()
			switch len(names) {
			case 1:
				typed.Type.Table.Name = stringNodeValue(names[0])
			case 2:
				typed.Type.Table = spec.Table{Schema: stringNodeValue(names[0]), Name: stringNodeValue(names[1])}
			case 3:
				typed.Type.Table = spec.Table{Schema: stringNodeValue(names[1]), Name: stringNodeValue(names[2])}
				typed.Type.CatalogQualified = true
			}
			ddl.PGCreateTyped = typed
		}
		ddl.PGCreateColumns = pgCreateColumns(stmt, ddl, spec.PGDestructiveTableTarget{
			Table: *ddl.Table, CatalogQualified: stmt.GetRelation().GetCatalogname() != "",
		})
		persistence := stmt.GetRelation().GetRelpersistence()
		if stmt.GetAccessMethod() != "" || stmt.GetTablespacename() != "" || len(stmt.GetOptions()) != 0 {
			ddl.PGCreateStorage = &spec.PGCreateStorage{
				Target:           spec.PGDestructiveTableTarget{Table: *ddl.Table, CatalogQualified: stmt.GetRelation().GetCatalogname() != ""},
				IfNotExists:      stmt.GetIfNotExists(),
				SessionDependent: persistence == "t" || len(stmt.GetInhRelations()) != 0 || len(ddl.PGCreateLike) != 0 || ddl.PGCreateTyped != nil,
				Method:           stmt.GetAccessMethod(), Tablespace: stmt.GetTablespacename(), Options: pgTableReloptions(stmt.GetOptions(), false, false),
			}
		}
		if parents := stmt.GetInhRelations(); len(parents) > 0 {
			decl := &spec.PGCreateInheritance{
				Target:      spec.PGDestructiveTableTarget{Table: *ddl.Table, CatalogQualified: stmt.GetRelation().GetCatalogname() != ""},
				IfNotExists: stmt.GetIfNotExists(), Unbound: persistence == "t" || ddl.PGCreateTyped != nil,
				Parents: make([]spec.PGDestructiveTableTarget, len(parents)),
			}
			for i, parent := range parents {
				if relation := parent.GetRangeVar(); relation != nil {
					decl.Parents[i] = spec.PGDestructiveTableTarget{Table: spec.Table{Schema: relation.GetSchemaname(), Name: relation.GetRelname()}, CatalogQualified: relation.GetCatalogname() != ""}
				}
			}
			ddl.PGCreateInheritance = decl
		}
		var action string
		switch stmt.GetOncommit() {
		case pg_query.OnCommitAction_ONCOMMIT_PRESERVE_ROWS:
			action = "preserve_rows"
		case pg_query.OnCommitAction_ONCOMMIT_DELETE_ROWS:
			action = "delete_rows"
		case pg_query.OnCommitAction_ONCOMMIT_DROP:
			action = "drop"
		}
		if reason := pgCreateExpressionRefusal(stmt.GetTableElts()); reason != "" {
			ddl.PGCreateExpression = &spec.PGCreateExpressionRefusal{
				Target:      spec.PGDestructiveTableTarget{Table: *ddl.Table, CatalogQualified: stmt.GetRelation().GetCatalogname() != ""},
				IfNotExists: stmt.GetIfNotExists(),
				Unbound:     persistence == "t" || len(stmt.GetInhRelations()) != 0 || len(ddl.PGCreateLike) != 0 || ddl.PGCreateTyped != nil,
				Reason:      reason,
			}
		}
		if fk := ddl.PGCreateForeignKeys; fk != nil {
			fk.Target = spec.PGDestructiveTableTarget{Table: *ddl.Table, CatalogQualified: stmt.GetRelation().GetCatalogname() != ""}
			fk.IfNotExists = stmt.GetIfNotExists()
			fk.Unbound = persistence == "t" || len(stmt.GetInhRelations()) != 0 || len(ddl.PGCreateLike) != 0 || ddl.PGCreateTyped != nil
		}
		if ddl.PGCreateKeys != nil {
			ddl.PGCreateKeys.Target = spec.PGDestructiveTableTarget{Table: *ddl.Table, CatalogQualified: stmt.GetRelation().GetCatalogname() != ""}
			ddl.PGCreateKeys.IfNotExists = stmt.GetIfNotExists()
			ddl.PGCreateKeys.Unbound = persistence == "t" || len(stmt.GetInhRelations()) != 0 || len(ddl.PGCreateLike) != 0 || ddl.PGCreateTyped != nil
		}
		if action != "" || persistence == "t" && ddl.Table.Schema != "" {
			ddl.PGCreateCommit = &spec.PGCreateCommit{
				Target:      spec.PGDestructiveTableTarget{Table: *ddl.Table, CatalogQualified: stmt.GetRelation().GetCatalogname() != ""},
				Persistence: persistence, Action: action, IfNotExists: stmt.GetIfNotExists(),
			}
		}
	}
	if len(ddl.PGCreateLike) > 0 {
		ddl.Options = map[string]string{"if_not_exists": strconv.FormatBool(stmt.GetIfNotExists())}
		ddl.PGTableCatalogQualified = stmt.GetRelation().GetCatalogname() != ""
		ddl.PGCreateLikeUnbound = stmt.GetRelation().GetRelpersistence() == "t" || len(stmt.GetInhRelations()) != 0 || stmt.GetOfTypename() != nil
	}
	statement.DDL = ddl
	return statement
}

func extractViewStmt(statement spec.Statement, stmt *pg_query.ViewStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "create_view", "postgresql create view statement payload is missing")
	}
	view := stmt.GetView()
	if view == nil {
		return unsupportedStatement(statement, "create_view", "postgresql create view target is missing")
	}

	options := map[string]string{}
	if stmt.GetReplace() {
		options["replace"] = "true"
	}
	if view.GetRelpersistence() == "t" {
		options["temporary"] = "true"
	}
	switch stmt.GetWithCheckOption() {
	case pg_query.ViewCheckOption_LOCAL_CHECK_OPTION:
		options["check_option"] = "true"
		options["check_option_scope"] = "local"
	case pg_query.ViewCheckOption_CASCADED_CHECK_OPTION:
		options["check_option"] = "true"
		options["check_option_scope"] = "cascaded"
	}

	statement.DDL = &spec.DDL{
		Operation:               spec.DDLOperationCreateView,
		ObjectName:              view.GetRelname(),
		ObjectType:              "view",
		Table:                   tableFromRangeVar(view),
		HasSelect:               stmt.GetQuery() != nil,
		Options:                 options,
		PGViewOptions:           pgViewOptions(stmt.GetOptions()),
		PGCreateViewOutput:      pgCreateViewOutput(stmt),
		PGTableCatalogQualified: view.GetCatalogname() != "",
	}
	return statement
}

func extractCreateSchemaStmt(statement spec.Statement, stmt *pg_query.CreateSchemaStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "create_schema", "postgresql create schema statement payload is missing")
	}
	target := &spec.PGCreateSchemaTarget{Name: stmt.GetSchemaname()}
	if stmt.GetAuthrole() != nil {
		target.Owner = pgRoleSelection(stmt.GetAuthrole())
		if !target.Owner.Valid() {
			return unsupportedStatement(statement, "create_schema_authorization", "postgresql schema owner is unmodeled")
		}
	}
	if len(stmt.GetSchemaElts()) > 0 {
		return unsupportedStatement(statement, "create_schema_nested_objects", "postgresql CREATE SCHEMA with nested objects is unsupported")
	}
	options := map[string]string{}
	if stmt.GetIfNotExists() {
		options["if_not_exists"] = "true"
	}
	statement.DDL = &spec.DDL{
		PGCreateSchema: target,
		Operation:      spec.DDLOperationCreateSchema,
		ObjectName:     stmt.GetSchemaname(),
		ObjectType:     "schema",
		Options:        options,
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
		PGSequenceChange: pgSequenceChange(stmt.GetSequence(), stmt.GetOptions(), false, stmt.GetIfNotExists()),
		Operation:        spec.DDLOperationCreateSequence,
		ObjectName:       rangeVarName(stmt.GetSequence()),
		ObjectType:       "sequence",
		Options:          options,
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
	declaration := &spec.PGCreateEnum{}
	names := stmt.GetTypeName()
	if len(names) == 2 {
		declaration.Target.Table = spec.Table{Schema: stringNodeValue(names[0]), Name: stringNodeValue(names[1])}
	}
	for _, value := range stmt.GetVals() {
		declaration.Labels = append(declaration.Labels, value.GetString_().GetSval())
	}
	statement.DDL = &spec.DDL{
		PGCreateEnum: declaration,
		Operation:    spec.DDLOperationCreateType,
		ObjectName:   objectName,
		ObjectType:   "type",
		Options:      options,
	}
	return statement
}

func extractCreateDomainStmt(statement spec.Statement, stmt *pg_query.CreateDomainStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "create_domain", "postgresql create domain statement payload is missing")
	}
	objectName := firstStringFromNodes(stmt.GetDomainname())
	if objectName == "" {
		return unsupportedStatement(statement, "create_domain", "postgresql create domain name is missing")
	}
	baseType := ""
	if tn := stmt.GetTypeName(); tn != nil {
		baseType = firstStringFromNodes(tn.GetNames())
	}
	options := map[string]string{"type_kind": "domain"}
	if baseType != "" {
		options["base_type"] = baseType
	}
	for _, c := range stmt.GetConstraints() {
		con := c.GetConstraint()
		if con == nil {
			continue
		}
		switch con.GetContype() {
		case pg_query.ConstrType_CONSTR_NOTNULL:
			options["not_null"] = "true"
		case pg_query.ConstrType_CONSTR_DEFAULT:
			options["has_default"] = "true"
		case pg_query.ConstrType_CONSTR_CHECK:
			options["has_check"] = "true"
			if con.GetConname() != "" {
				options["constraint"] = con.GetConname()
			}
		}
	}
	statement.DDL = &spec.DDL{
		Operation:  spec.DDLOperationCreateDomain,
		ObjectName: objectName,
		ObjectType: "domain",
		Options:    options,
	}
	return statement
}

func extractCreateExtensionStmt(statement spec.Statement, stmt *pg_query.CreateExtensionStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "create_extension", "postgresql create extension statement payload is missing")
	}
	options := map[string]string{}
	if stmt.GetIfNotExists() {
		options["if_not_exists"] = "true"
	}
	for _, optNode := range stmt.GetOptions() {
		defElem := optNode.GetDefElem()
		if defElem == nil {
			continue
		}
		switch defElem.GetDefname() {
		case "schema":
			if arg := defElem.GetArg(); arg != nil {
				if s := arg.GetString_(); s != nil {
					options["schema"] = s.GetSval()
				}
			}
		case "new_version":
			if arg := defElem.GetArg(); arg != nil {
				if s := arg.GetString_(); s != nil {
					options["version"] = s.GetSval()
				}
			}
		case "cascade":
			options["cascade"] = "true"
		}
	}
	statement.DDL = &spec.DDL{
		Operation:  spec.DDLOperationCreateExtension,
		ObjectName: stmt.GetExtname(),
		ObjectType: "extension",
		Options:    options,
	}
	return statement
}

// extractCompositeTypeStmt normalizes CREATE TYPE ... AS (...) into spec.DDL
// with operation create_type and type_kind=composite.
func extractCompositeTypeStmt(statement spec.Statement, stmt *pg_query.CompositeTypeStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "create_type", "postgresql composite type statement payload is missing")
	}
	objectName := ""
	schemaName := ""
	if rv := stmt.GetTypevar(); rv != nil {
		objectName = rv.GetRelname()
		schemaName = rv.GetSchemaname()
	}
	if objectName == "" {
		return unsupportedStatement(statement, "create_type", "postgresql composite type name is missing")
	}
	var attrNames []string
	for _, col := range stmt.GetColdeflist() {
		if cd := col.GetColumnDef(); cd != nil {
			attrNames = append(attrNames, cd.GetColname())
		}
	}
	options := map[string]string{
		"type_kind":  "composite",
		"attributes": strconv.Itoa(len(attrNames)),
	}
	if len(attrNames) > 0 {
		options["attribute_names"] = strings.Join(attrNames, ",")
	}
	if schemaName != "" {
		options["schema"] = schemaName
	}
	statement.DDL = &spec.DDL{
		PGComposite: &spec.PGCompositeDeclaration{
			Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: schemaName, Name: objectName}, CatalogQualified: stmt.GetTypevar().GetCatalogname() != ""},
			Action: "create", Attributes: attrNames,
		},
		Operation:  spec.DDLOperationCreateType,
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

func extractCreateFunctionStmt(statement spec.Statement, stmt *pg_query.CreateFunctionStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "create_function", "postgresql create function statement payload is missing")
	}

	options := map[string]string{}

	if stmt.GetReplace() {
		options["replace"] = "true"
	}

	for _, optNode := range stmt.GetOptions() {
		defElem := optNode.GetDefElem()
		if defElem == nil {
			continue
		}
		switch defElem.GetDefname() {
		case "security":
			if arg := defElem.GetArg(); arg != nil {
				if b := arg.GetBoolean(); b != nil {
					options["security_definer"] = fmt.Sprintf("%v", b.GetBoolval())
				}
			}
		}
	}

	// Extract function name from funcname nodes. Schema-qualified names have
	// two elements: [schema, name]. Plain names have one element: [name].
	funcName := ""
	schemaName := ""
	names := stmt.GetFuncname()
	if len(names) >= 2 {
		schemaName = firstStringFromNodes(names[:len(names)-1])
		funcName = firstStringFromNodes(names[len(names)-1:])
	} else if len(names) == 1 {
		funcName = firstStringFromNodes(names)
	}

	if schemaName != "" {
		options["schema"] = schemaName
	}

	operation := spec.DDLOperationCreateFunction
	objectType := "function"
	if stmt.GetIsProcedure() {
		operation = spec.DDLOperationCreateProcedure
		objectType = "procedure"
	}

	statement.DDL = &spec.DDL{
		Operation:  operation,
		ObjectName: funcName,
		ObjectType: objectType,
		Options:    options,
	}
	return statement
}

func retainPGCreateForeignKey(ddl *spec.DDL, c *pg_query.Constraint, columns []string) {
	relation := c.GetPktable()
	table := tableFromRangeVar(relation)
	if table == nil {
		return
	}
	if ddl.PGCreateForeignKeys == nil {
		ddl.PGCreateForeignKeys = &spec.PGCreateForeignKeys{}
	}
	ddl.PGCreateForeignKeys.Declarations = append(ddl.PGCreateForeignKeys.Declarations, spec.PGForeignKeySelection{
		Reference: *table, ReferenceCatalogQualified: relation.GetCatalogname() != "", Columns: columns,
		ReferencedColumns: stringValuesFromNodes(c.GetPkAttrs()), DeleteSetColumns: stringValuesFromNodes(c.GetFkDelSetCols()),
	})
}

func retainPGCreateKey(ddl *spec.DDL, constraint *pg_query.Constraint, keys []string) {
	if ddl.PGCreateKeys == nil {
		ddl.PGCreateKeys = &spec.PGCreateKeys{}
	}
	ddl.PGCreateKeys.Declarations = append(ddl.PGCreateKeys.Declarations, spec.PGCreateKeyDeclaration{
		Primary:    constraint.GetContype() == pg_query.ConstrType_CONSTR_PRIMARY,
		UsingIndex: constraint.GetIndexname() != "", NullsNotDistinct: constraint.GetNullsNotDistinct(),
		Keys: keys, Include: stringValuesFromNodes(constraint.GetIncluding()),
	})
}

func applyTableConstraint(ddl *spec.DDL, constraint *pg_query.Constraint) *spec.UnsupportedDetail {
	switch constraint.GetContype() {
	case pg_query.ConstrType_CONSTR_UNIQUE:
		retainPGCreateKey(ddl, constraint, stringValuesFromNodes(constraint.GetKeys()))
		ddl.Indexes = append(ddl.Indexes, spec.Index{Name: constraint.GetConname(), Kind: spec.IndexKindUnique, Columns: stringValuesFromNodes(constraint.GetKeys())})
	case pg_query.ConstrType_CONSTR_FOREIGN:
		retainPGCreateForeignKey(ddl, constraint, stringValuesFromNodes(constraint.GetFkAttrs()))
		ddl.Constraints = append(ddl.Constraints, spec.Constraint{
			Type:              "foreign_key",
			Name:              constraint.GetConname(),
			Columns:           stringValuesFromNodes(constraint.GetFkAttrs()),
			ReferencedSchema:  rangeVarSchema(constraint.GetPktable()),
			ReferencedTable:   rangeVarName(constraint.GetPktable()),
			ReferencedColumns: stringValuesFromNodes(constraint.GetPkAttrs()),
		})
	case pg_query.ConstrType_CONSTR_PRIMARY:
		retainPGCreateKey(ddl, constraint, stringValuesFromNodes(constraint.GetKeys()))
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
			retainPGCreateKey(ddl, constraint, []string{column.GetColname()})
			ddl.Indexes = append(ddl.Indexes, spec.Index{Name: constraint.GetConname(), Kind: spec.IndexKindUnique, Columns: []string{column.GetColname()}})
		case pg_query.ConstrType_CONSTR_FOREIGN:
			retainPGCreateForeignKey(ddl, constraint, []string{column.GetColname()})
			ddl.Constraints = append(ddl.Constraints, spec.Constraint{
				Type:              "foreign_key",
				Name:              constraint.GetConname(),
				Columns:           []string{column.GetColname()},
				ReferencedSchema:  rangeVarSchema(constraint.GetPktable()),
				ReferencedTable:   rangeVarName(constraint.GetPktable()),
				ReferencedColumns: stringValuesFromNodes(constraint.GetPkAttrs()),
			})
		case pg_query.ConstrType_CONSTR_PRIMARY:
			retainPGCreateKey(ddl, constraint, []string{column.GetColname()})
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
	if result.HasDefault {
		result.DefaultKind = classifyDefaultKind(column)
		if result.DefaultKind == "" {
			result.DefaultKind = "unknown"
		}
	}
	applyGeneratedIdentityFacts(&result, column)
	result.PGGeneration = pgGenerationFromDef(column)
	return result
}

func classifyDefaultKind(column *pg_query.ColumnDef) string {
	node := column.GetRawDefault()
	if node == nil {
		node = column.GetCookedDefault()
	}
	if node != nil {
		return defaultKindFromNode(node)
	}
	for _, item := range column.GetConstraints() {
		con := item.GetConstraint()
		if con == nil || con.GetContype() != pg_query.ConstrType_CONSTR_DEFAULT {
			continue
		}
		expr := con.GetRawExpr()
		if expr == nil {
			return "unknown"
		}
		return defaultKindFromNode(expr)
	}
	return ""
}

func defaultKindFromNode(node *pg_query.Node) string {
	switch node.GetNode().(type) {
	case *pg_query.Node_AConst:
		aconst := node.GetAConst()
		if aconst.GetIsnull() {
			return "null"
		}
		return "literal"
	case *pg_query.Node_FuncCall:
		return "function_call"
	default:
		return "expression"
	}
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
//
//nolint:unused
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
