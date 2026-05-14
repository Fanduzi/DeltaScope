//go:build postgresql

package postgresql

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

func extractAlterTableStmt(statement spec.Statement, stmt *pg_query.AlterTableStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "alter_table", "postgresql alter table statement payload is missing")
	}
	if stmt.GetObjtype() == pg_query.ObjectType_OBJECT_TYPE {
		return extractAlterTypeCompositeFromAlterTableStmt(statement, stmt)
	}
	if stmt.GetObjtype() == pg_query.ObjectType_OBJECT_INDEX {
		return extractAlterIndexStmt(statement, stmt)
	}
	if stmt.GetObjtype() == pg_query.ObjectType_OBJECT_FOREIGN_TABLE {
		return extractAlterForeignTableStmt(statement, stmt)
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

	switch stmt.GetRenameType() {
	case pg_query.ObjectType_OBJECT_TYPE:
		objectName := ""
		if stmt.GetRelation() != nil {
			objectName = stmt.GetRelation().GetRelname()
		}
		if objectName == "" {
			objectName = renameObjectTypeName(stmt)
		}
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationAlterType,
			ObjectName: objectName,
			ObjectType: "type",
			Options:    map[string]string{"type_kind": "composite", "action": "rename", "new_name": stmt.GetNewname()},
		}
		return statement
	case pg_query.ObjectType_OBJECT_ATTRIBUTE:
		typeName := ""
		if rng := stmt.GetRelation(); rng != nil {
			typeName = rng.GetRelname()
		}
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationAlterType,
			ObjectName: typeName,
			ObjectType: "type",
			Options: map[string]string{
				"type_kind": "composite",
				"action":    "rename_attribute",
				"attribute": stmt.GetSubname(),
				"new_name":  stmt.GetNewname(),
			},
		}
		return statement
	case pg_query.ObjectType_OBJECT_DOMAIN:
		objectName := renameObjectDomainName(stmt)
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationAlterDomain,
			ObjectName: objectName,
			ObjectType: "domain",
			Options:    map[string]string{"action": "rename", "new_name": stmt.GetNewname()},
		}
		return statement
	case pg_query.ObjectType_OBJECT_VIEW:
		objectName := ""
		if stmt.GetRelation() != nil {
			objectName = stmt.GetRelation().GetRelname()
		}
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationAlterView,
			ObjectName: objectName,
			ObjectType: "view",
			Options:    map[string]string{"action": "rename_view", "new_name": stmt.GetNewname()},
		}
		return statement
	case pg_query.ObjectType_OBJECT_SCHEMA:
		objectName := renameObjectSchemaName(stmt)
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationAlterSchema,
			ObjectName: objectName,
			ObjectType: "schema",
			Options:    map[string]string{"action": "rename", "new_name": stmt.GetNewname()},
		}
		return statement
	case pg_query.ObjectType_OBJECT_MATVIEW:
		objectName := ""
		if stmt.GetRelation() != nil {
			objectName = stmt.GetRelation().GetRelname()
		}
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationAlterMaterializedView,
			ObjectName: objectName,
			ObjectType: "materialized_view",
			Options:    map[string]string{"action": "rename", "new_name": stmt.GetNewname()},
		}
		return statement
	case pg_query.ObjectType_OBJECT_EVENT_TRIGGER:
		return extractRenameEventTrigger(statement, stmt)
	case pg_query.ObjectType_OBJECT_RULE:
		return extractRenameRule(statement, stmt)
	case pg_query.ObjectType_OBJECT_COLLATION:
		objectName := renameObjectCollationName(stmt)
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationAlterCollation,
			ObjectName: objectName,
			ObjectType: "collation",
			Options:    map[string]string{"action": "rename", "new_name": stmt.GetNewname()},
		}
		return statement
	case pg_query.ObjectType_OBJECT_STATISTIC_EXT:
		objectName := renameObjectCollationName(stmt)
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationAlterStatistics,
			ObjectName: objectName,
			ObjectType: "statistics",
			Options:    map[string]string{"action": "rename", "new_name": stmt.GetNewname()},
		}
		return statement
	default:
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
		var objectName string
		if table != nil {
			objectName = table.Name
		}
		options := map[string]string{"action": "rename", "new_name": stmt.GetNewname()}
		if table != nil && strings.TrimSpace(table.Schema) != "" {
			options["schema"] = strings.TrimSpace(table.Schema)
		}
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationAlterIndex,
			ObjectName: objectName,
			ObjectType: "index",
			Options:    options,
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
	if stmt.GetObjectType() == pg_query.ObjectType_OBJECT_TYPE {
		return extractAlterTypeSetSchema(statement, stmt)
	}
	if stmt.GetObjectType() == pg_query.ObjectType_OBJECT_EXTENSION {
		objectName := ""
		if obj := stmt.GetObject(); obj != nil {
			if list := obj.GetList(); list != nil {
				objectName = firstStringFromNodes(list.GetItems())
			} else if s := obj.GetString_(); s != nil {
				objectName = s.GetSval()
			}
		}
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationAlterExtension,
			ObjectName: objectName,
			ObjectType: "extension",
			Options: map[string]string{
				"action":     "set_schema",
				"new_schema": stmt.GetNewschema(),
			},
		}
		return statement
	}
	if stmt.GetObjectType() == pg_query.ObjectType_OBJECT_VIEW {
		objectName := ""
		if stmt.GetRelation() != nil {
			objectName = stmt.GetRelation().GetRelname()
		}
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationAlterView,
			ObjectName: objectName,
			ObjectType: "view",
			Options: map[string]string{
				"action":     "set_schema",
				"new_schema": stmt.GetNewschema(),
			},
		}
		return statement
	}
	if stmt.GetObjectType() == pg_query.ObjectType_OBJECT_MATVIEW {
		objectName := ""
		if stmt.GetRelation() != nil {
			objectName = stmt.GetRelation().GetRelname()
		}
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationAlterMaterializedView,
			ObjectName: objectName,
			ObjectType: "materialized_view",
			Options: map[string]string{
				"action":     "set_schema",
				"new_schema": stmt.GetNewschema(),
			},
		}
		return statement
	}
	if stmt.GetObjectType() == pg_query.ObjectType_OBJECT_COLLATION {
		objectName := alterObjectSchemaObjectName(stmt)
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationAlterCollation,
			ObjectName: objectName,
			ObjectType: "collation",
			Options: map[string]string{
				"action":     "set_schema",
				"new_schema": stmt.GetNewschema(),
			},
		}
		return statement
	}
	if stmt.GetObjectType() == pg_query.ObjectType_OBJECT_STATISTIC_EXT {
		objectName := alterObjectSchemaObjectName(stmt)
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationAlterStatistics,
			ObjectName: objectName,
			ObjectType: "statistics",
			Options: map[string]string{
				"action":     "set_schema",
				"new_schema": stmt.GetNewschema(),
			},
		}
		return statement
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

func extractAlterDomainStmt(statement spec.Statement, stmt *pg_query.AlterDomainStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "alter_domain", "postgresql alter domain statement payload is missing")
	}
	objectName := firstStringFromNodes(stmt.GetTypeName())
	if objectName == "" {
		return unsupportedStatement(statement, "alter_domain", "postgresql alter domain name is missing")
	}
	options := map[string]string{}
	switch stmt.GetSubtype() {
	case "T":
		if stmt.GetDef() != nil {
			options["action"] = "set_default"
			options["has_default"] = "true"
		} else {
			options["action"] = "drop_default"
		}
	case "O":
		options["action"] = "set_not_null"
		options["not_null"] = "true"
	case "N":
		options["action"] = "drop_not_null"
	case "C":
		options["action"] = "add_constraint"
		if stmt.GetName() != "" {
			options["constraint"] = stmt.GetName()
		}
		options["has_check"] = "true"
	case "X":
		options["action"] = "drop_constraint"
		if stmt.GetName() != "" {
			options["constraint"] = stmt.GetName()
		}
	case "V":
		options["action"] = "validate_constraint"
		if stmt.GetName() != "" {
			options["constraint"] = stmt.GetName()
		}
	default:
		return unsupportedStatement(statement, "alter_domain", fmt.Sprintf("postgresql alter domain subtype %q is not supported", stmt.GetSubtype()))
	}
	statement.DDL = &spec.DDL{
		Operation:  spec.DDLOperationAlterDomain,
		ObjectName: objectName,
		ObjectType: "domain",
		Options:    options,
	}
	return statement
}

func extractAlterExtensionStmt(statement spec.Statement, stmt *pg_query.AlterExtensionStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "alter_extension", "postgresql alter extension statement payload is missing")
	}
	options := map[string]string{"action": "update"}
	for _, optNode := range stmt.GetOptions() {
		defElem := optNode.GetDefElem()
		if defElem == nil {
			continue
		}
		if defElem.GetDefname() == "new_version" {
			if arg := defElem.GetArg(); arg != nil {
				if s := arg.GetString_(); s != nil {
					options["version"] = s.GetSval()
				}
			}
		}
	}
	statement.DDL = &spec.DDL{
		Operation:  spec.DDLOperationAlterExtension,
		ObjectName: stmt.GetExtname(),
		ObjectType: "extension",
		Options:    options,
	}
	return statement
}

func extractAlterExtensionContentsStmt(statement spec.Statement, stmt *pg_query.AlterExtensionContentsStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "alter_extension", "postgresql alter extension contents statement payload is missing")
	}

	var action string
	switch stmt.GetAction() {
	case 1:
		action = "add_member"
	case -1:
		action = "drop_member"
	default:
		return unsupportedStatement(statement, "alter_extension", fmt.Sprintf("postgresql alter extension contents action %d is deferred", stmt.GetAction()))
	}

	memberType := strings.ToLower(strings.TrimPrefix(stmt.GetObjtype().String(), "OBJECT_"))
	memberName := ""
	if obj := stmt.GetObject(); obj != nil {
		if list := obj.GetList(); list != nil {
			memberName = firstStringFromNodes(list.GetItems())
		} else {
			memberName = firstStringFromNodes([]*pg_query.Node{obj})
		}
	}

	options := map[string]string{
		"action":      action,
		"member_type": memberType,
	}
	if memberName != "" {
		options["member"] = memberName
	}

	statement.DDL = &spec.DDL{
		Operation:  spec.DDLOperationAlterExtension,
		ObjectName: stmt.GetExtname(),
		ObjectType: "extension",
		Options:    options,
	}
	return statement
}

// extractAlterTypeCompositeFromAlterTableStmt handles ALTER TYPE ...
// ADD/DROP/ALTER ATTRIBUTE which pg_query maps to AlterTableStmt with
// objtype=OBJECT_TYPE.
func extractAlterTypeCompositeFromAlterTableStmt(statement spec.Statement, stmt *pg_query.AlterTableStmt) spec.Statement {
	if len(stmt.GetCmds()) == 0 {
		return unsupportedStatement(statement, "alter_type", "postgresql alter type composite statement has no commands")
	}
	cmd := stmt.GetCmds()[0].GetAlterTableCmd()
	if cmd == nil {
		return unsupportedStatement(statement, "alter_type", "postgresql alter type composite command payload is missing")
	}

	objectName := ""
	if rng := stmt.GetRelation(); rng != nil {
		objectName = rng.GetRelname()
	}

	switch cmd.GetSubtype() {
	case pg_query.AlterTableType_AT_AddColumn:
		attribute := ""
		attributeType := ""
		if def := cmd.GetDef(); def != nil {
			if cd := def.GetColumnDef(); cd != nil {
				attribute = cd.GetColname()
				if tn := cd.GetTypeName(); tn != nil {
					attributeType = typeNameString(tn)
				}
			}
		}
		options := map[string]string{
			"type_kind": "composite",
			"action":    "add_attribute",
			"attribute": attribute,
		}
		if attributeType != "" {
			options["attribute_type"] = attributeType
		}
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationAlterType,
			ObjectName: objectName,
			ObjectType: "type",
			Options:    options,
		}
		return statement
	case pg_query.AlterTableType_AT_DropColumn:
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationAlterType,
			ObjectName: objectName,
			ObjectType: "type",
			Options: map[string]string{
				"type_kind": "composite",
				"action":    "drop_attribute",
				"attribute": cmd.GetName(),
			},
		}
		return statement
	case pg_query.AlterTableType_AT_AlterColumnType:
		attribute := cmd.GetName()
		attributeType := ""
		if def := cmd.GetDef(); def != nil {
			if cd := def.GetColumnDef(); cd != nil {
				if tn := cd.GetTypeName(); tn != nil {
					attributeType = typeNameString(tn)
				}
			} else if tn := def.GetTypeName(); tn != nil {
				attributeType = typeNameString(tn)
			}
		}
		options := map[string]string{
			"type_kind": "composite",
			"action":    "alter_attribute_type",
			"attribute": attribute,
		}
		if attributeType != "" {
			options["attribute_type"] = attributeType
		}
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationAlterType,
			ObjectName: objectName,
			ObjectType: "type",
			Options:    options,
		}
		return statement
	default:
		return unsupportedStatement(statement, "alter_type", fmt.Sprintf("postgresql alter type composite action %s is deferred", cmd.GetSubtype()))
	}
}

// extractAlterTypeSetSchema normalizes ALTER TYPE ... SET SCHEMA into
// spec.DDL with operation alter_type and action=set_schema.
func extractAlterTypeSetSchema(statement spec.Statement, stmt *pg_query.AlterObjectSchemaStmt) spec.Statement {
	objectName := ""
	if obj := stmt.GetObject(); obj != nil {
		if list := obj.GetList(); list != nil {
			objectName = firstStringFromNodes(list.GetItems())
		}
	}
	statement.DDL = &spec.DDL{
		Operation:  spec.DDLOperationAlterType,
		ObjectName: objectName,
		ObjectType: "type",
		Options: map[string]string{
			"type_kind":  "composite",
			"action":     "set_schema",
			"new_schema": stmt.GetNewschema(),
		},
	}
	return statement
}

func extractAlterDefaultPrivilegesStmt(statement spec.Statement, stmt *pg_query.AlterDefaultPrivilegesStmt) spec.Statement {
	return unsupportedStatement(statement, "alter_default_privileges", "postgresql alter default privileges is deferred")
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
	case pg_query.AlterTableType_AT_EnableRowSecurity:
		return spec.Alter{Action: "enable_rls"}, true, nil
	case pg_query.AlterTableType_AT_DisableRowSecurity:
		return spec.Alter{Action: "disable_rls"}, true, nil
	case pg_query.AlterTableType_AT_ForceRowSecurity:
		return spec.Alter{Action: "force_rls"}, true, nil
	case pg_query.AlterTableType_AT_NoForceRowSecurity:
		return spec.Alter{Action: "no_force_rls"}, true, nil
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
			return string(rune(arg.GetInteger().GetIval())) //nolint:unconvert // intentional rune→string conversion
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

func alterSubtypeFeature(subtype pg_query.AlterTableType) string {
	name := strings.TrimPrefix(subtype.String(), "AT_")
	if name == "" || name == "ALTER_TABLE_TYPE_UNDEFINED" {
		return "alter_table"
	}
	return strings.ToLower(name)
}

// renameObjectTypeName extracts the type name from a RenameStmt targeting
// OBJECT_TYPE. The type name is stored in the Object field as a List of
// String nodes.
func renameObjectTypeName(stmt *pg_query.RenameStmt) string {
	obj := stmt.GetObject()
	if obj == nil {
		return ""
	}
	if list := obj.GetList(); list != nil {
		return firstStringFromNodes(list.GetItems())
	}
	return firstStringFromNodes([]*pg_query.Node{obj})
}

// renameObjectDomainName extracts the domain name from a RenameStmt targeting
// OBJECT_DOMAIN. The domain name is stored in the Object field as a List of
// String nodes, not in the Relation field.
func renameObjectDomainName(stmt *pg_query.RenameStmt) string {
	obj := stmt.GetObject()
	if obj == nil {
		return ""
	}
	if list := obj.GetList(); list != nil {
		return firstStringFromNodes(list.GetItems())
	}
	return firstStringFromNodes([]*pg_query.Node{obj})
}

// firstStringFromNodes returns the first String node value from a flat node list.
func firstStringFromNodes(nodes []*pg_query.Node) string {
	for _, n := range nodes {
		if s := n.GetString_(); s != nil && s.GetSval() != "" {
			return s.GetSval()
		}
	}
	return ""
}

func lastStringFromNodes(nodes []*pg_query.Node) string {
	for i := len(nodes) - 1; i >= 0; i-- {
		if s := nodes[i].GetString_(); s != nil && s.GetSval() != "" {
			return s.GetSval()
		}
	}
	return ""
}

func extractGrantStmt(statement spec.Statement, stmt *pg_query.GrantStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "grant_table", "postgresql grant statement payload is missing")
	}

	// Only support ordinary table privileges (ACL_TARGET_OBJECT + OBJECT_TABLE).
	if stmt.GetTargtype() == pg_query.GrantTargetType_ACL_TARGET_ALL_IN_SCHEMA {
		return unsupportedStatement(statement, "grant_all_tables_in_schema", "postgresql grant all tables in schema is deferred")
	}
	if stmt.GetObjtype() != pg_query.ObjectType_OBJECT_TABLE {
		return unsupportedStatement(statement, "grant_table", fmt.Sprintf("postgresql grant object type %s is deferred", stmt.GetObjtype()))
	}
	if stmt.GetTargtype() != pg_query.GrantTargetType_ACL_TARGET_OBJECT {
		return unsupportedStatement(statement, "grant_table", fmt.Sprintf("postgresql grant target type %s is deferred", stmt.GetTargtype()))
	}

	// Extract table name from first RangeVar object.
	tableName, schemaName := grantObjectName(stmt.GetObjects())
	if tableName == "" {
		return unsupportedStatement(statement, "grant_table", "postgresql grant table target name is missing")
	}

	options := map[string]string{}
	// Privileges: empty list means ALL PRIVILEGES; otherwise CSV of named privileges.
	privNames := grantPrivilegeNames(stmt.GetPrivileges())
	if len(privNames) == 0 {
		options["all_privileges"] = "true"
	} else {
		options["privileges"] = strings.Join(privNames, ",")
	}
	// Grantees: CSV of role names.
	granteeNames := grantGranteeNames(stmt.GetGrantees())
	if len(granteeNames) > 0 {
		options["grantees"] = strings.Join(granteeNames, ",")
	}
	if schemaName != "" {
		options["schema"] = schemaName
	}
	if stmt.GetGrantOption() {
		options["grant_option"] = "true"
	}
	if !stmt.GetIsGrant() && stmt.GetBehavior() == pg_query.DropBehavior_DROP_CASCADE {
		options["cascade"] = "true"
	}

	operation := spec.DDLOperationGrantTable
	if !stmt.GetIsGrant() {
		operation = spec.DDLOperationRevokeTable
	}

	statement.DDL = &spec.DDL{
		Operation:  operation,
		ObjectName: tableName,
		ObjectType: "table",
		Options:    options,
	}
	return statement
}

func extractGrantRoleStmt(statement spec.Statement, stmt *pg_query.GrantRoleStmt) spec.Statement {
	return unsupportedStatement(statement, "grant_role", "postgresql role membership grant/revoke is deferred")
}

func extractAlterOwnerStmt(statement spec.Statement, stmt *pg_query.AlterOwnerStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "alter_owner", "postgresql alter owner statement payload is missing")
	}

	owner := ""
	if no := stmt.GetNewowner(); no != nil {
		owner = no.GetRolename()
	}

	switch stmt.GetObjectType() {
	case pg_query.ObjectType_OBJECT_SCHEMA:
		objectName := alterOwnerObjectName(stmt)
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationAlterSchema,
			ObjectName: objectName,
			ObjectType: "schema",
			Options:    map[string]string{"action": "set_owner", "owner": owner},
		}
		return statement
	case pg_query.ObjectType_OBJECT_COLLATION:
		objectName := alterOwnerObjectName(stmt)
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationAlterCollation,
			ObjectName: objectName,
			ObjectType: "collation",
			Options:    map[string]string{"action": "set_owner", "owner": owner},
		}
		return statement
	case pg_query.ObjectType_OBJECT_STATISTIC_EXT:
		objectName := alterOwnerObjectName(stmt)
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationAlterStatistics,
			ObjectName: objectName,
			ObjectType: "statistics",
			Options:    map[string]string{"action": "set_owner", "owner": owner},
		}
		return statement
	default:
		return unsupportedStatement(statement, "alter_owner", fmt.Sprintf("postgresql alter owner for %s is deferred", stmt.GetObjectType()))
	}
}

func extractAlterIndexStmt(statement spec.Statement, stmt *pg_query.AlterTableStmt) spec.Statement {
	if len(stmt.GetCmds()) == 0 {
		return unsupportedStatement(statement, "alter_index", "postgresql alter index statement has no commands")
	}
	table := tableFromRangeVar(stmt.GetRelation())
	objectName := ""
	if table != nil {
		objectName = table.Name
	}
	for _, item := range stmt.GetCmds() {
		cmd := item.GetAlterTableCmd()
		if cmd == nil {
			continue
		}
		switch cmd.GetSubtype() {
		case pg_query.AlterTableType_AT_SetTableSpace:
			tablespace := cmd.GetName()
			statement.DDL = &spec.DDL{
				Operation:  spec.DDLOperationAlterIndex,
				ObjectName: objectName,
				ObjectType: "index",
				Options:    map[string]string{"action": "set_tablespace", "tablespace": tablespace},
			}
			return statement
		default:
			return unsupportedStatement(statement, "alter_index", fmt.Sprintf("postgresql alter index command %s is deferred", cmd.GetSubtype()))
		}
	}
	return unsupportedStatement(statement, "alter_index", "postgresql alter index statement has no supported commands")
}

// renameObjectSchemaName extracts the schema name from a RenameStmt targeting
// OBJECT_SCHEMA. The schema name is stored in the Subname field.
func renameObjectSchemaName(stmt *pg_query.RenameStmt) string {
	if s := stmt.GetSubname(); s != "" {
		return s
	}
	if stmt.GetRelation() != nil {
		return stmt.GetRelation().GetRelname()
	}
	obj := stmt.GetObject()
	if obj == nil {
		return ""
	}
	if list := obj.GetList(); list != nil {
		return firstStringFromNodes(list.GetItems())
	}
	return firstStringFromNodes([]*pg_query.Node{obj})
}

// alterOwnerObjectName extracts the object name from an AlterOwnerStmt.
func alterOwnerObjectName(stmt *pg_query.AlterOwnerStmt) string {
	if obj := stmt.GetObject(); obj != nil {
		if list := obj.GetList(); list != nil {
			return firstStringFromNodes(list.GetItems())
		}
		return firstStringFromNodes([]*pg_query.Node{obj})
	}
	if rv := stmt.GetRelation(); rv != nil {
		return rv.GetRelname()
	}
	return ""
}

// grantObjectName extracts table name and optional schema from GrantStmt.Objects.
func grantObjectName(objects []*pg_query.Node) (name string, schema string) {
	for _, obj := range objects {
		if rv := obj.GetRangeVar(); rv != nil {
			return rv.GetRelname(), rv.GetSchemaname()
		}
	}
	return "", ""
}

// grantPrivilegeNames extracts named privileges from AccessPriv nodes.
// Returns empty slice when ALL PRIVILEGES is implied (no AccessPriv nodes or
// AccessPriv with empty priv_name).
func grantPrivilegeNames(privileges []*pg_query.Node) []string {
	var names []string
	for _, n := range privileges {
		ap := n.GetAccessPriv()
		if ap == nil {
			continue
		}
		pn := ap.GetPrivName()
		if pn != "" {
			names = append(names, strings.ToLower(pn))
		}
	}
	return names
}

// grantGranteeNames extracts role names from RoleSpec nodes.
func grantGranteeNames(grantees []*pg_query.Node) []string {
	var names []string
	for _, n := range grantees {
		if rs := n.GetRoleSpec(); rs != nil {
			names = append(names, rs.GetRolename())
		}
	}
	return names
}

func extractDefineStmt(statement spec.Statement, stmt *pg_query.DefineStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "unknown", "postgresql define statement payload is missing")
	}
	switch stmt.GetKind() {
	case pg_query.ObjectType_OBJECT_COLLATION:
		objectName := lastStringFromNodes(stmt.GetDefnames())
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationCreateCollation,
			ObjectName: objectName,
			ObjectType: "collation",
		}
		return statement
	default:
		return unsupportedStatement(statement, "unknown", fmt.Sprintf("postgresql define statement for %s is deferred", stmt.GetKind()))
	}
}

func extractCreateStatsStmt(statement spec.Statement, stmt *pg_query.CreateStatsStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "create_statistics", "postgresql create statistics statement payload is missing")
	}
	objectName := lastStringFromNodes(stmt.GetDefnames())
	options := map[string]string{}
	for _, rel := range stmt.GetRelations() {
		if rv := rel.GetRangeVar(); rv != nil {
			options["target_table"] = rv.GetRelname()
			break
		}
	}
	statement.DDL = &spec.DDL{
		Operation:  spec.DDLOperationCreateStatistics,
		ObjectName: objectName,
		ObjectType: "statistics",
		Options:    options,
	}
	return statement
}

func extractAlterStatsStmt(statement spec.Statement, stmt *pg_query.AlterStatsStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "alter_statistics", "postgresql alter statistics statement payload is missing")
	}
	objectName := lastStringFromNodes(stmt.GetDefnames())
	statement.DDL = &spec.DDL{
		Operation:  spec.DDLOperationAlterStatistics,
		ObjectName: objectName,
		ObjectType: "statistics",
		Options:    map[string]string{"action": "set_statistics_target"},
	}
	return statement
}

func renameObjectCollationName(stmt *pg_query.RenameStmt) string {
	obj := stmt.GetObject()
	if obj == nil {
		return ""
	}
	if list := obj.GetList(); list != nil {
		return lastStringFromNodes(list.GetItems())
	}
	return firstStringFromNodes([]*pg_query.Node{obj})
}

func alterObjectSchemaObjectName(stmt *pg_query.AlterObjectSchemaStmt) string {
	if obj := stmt.GetObject(); obj != nil {
		if list := obj.GetList(); list != nil {
			return lastStringFromNodes(list.GetItems())
		}
		return firstStringFromNodes([]*pg_query.Node{obj})
	}
	if stmt.GetRelation() != nil {
		return stmt.GetRelation().GetRelname()
	}
	return ""
}
