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
	switch stmt.GetObjtype() {
	case pg_query.ObjectType_OBJECT_VIEW:
		return extractAlterViewAction(statement, stmt, spec.DDLOperationAlterView, "view")
	case pg_query.ObjectType_OBJECT_MATVIEW:
		return extractAlterViewAction(statement, stmt, spec.DDLOperationAlterMaterializedView, "materialized_view")
	}
	if stmt.GetObjtype() != pg_query.ObjectType_OBJECT_TYPE_UNDEFINED && stmt.GetObjtype() != pg_query.ObjectType_OBJECT_TABLE {
		return unsupportedStatement(statement, "alter_table", "postgresql alter table object type is unsupported in v1")
	}

	ddl := &spec.DDL{
		Operation:               spec.DDLOperationAlterTable,
		PGTableIfExists:         stmt.GetMissingOk(),
		PGTableCatalogQualified: stmt.GetRelation().GetCatalogname() != "",
		PGTableOnly:             !stmt.GetRelation().GetInh(),
		Table:                   tableFromRangeVar(stmt.GetRelation()),
		Alter:                   make([]spec.Alter, 0, len(stmt.GetCmds())),
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
	case pg_query.ObjectType_OBJECT_POLICY:
		statement.DDL = &spec.DDL{Operation: spec.DDLOperationAlterPolicy, ObjectType: "policy", ObjectName: stmt.GetSubname(), Table: tableFromRangeVar(stmt.GetRelation()),
			PGPolicyChange: pgPolicyChangeDeclaration(stmt.GetRelation(), stmt.GetSubname(), stmt.GetNewname())}
		return statement
	case pg_query.ObjectType_OBJECT_TYPE:
		objectName := ""
		if stmt.GetRelation() != nil {
			objectName = stmt.GetRelation().GetRelname()
		}
		if objectName == "" {
			objectName = renameObjectTypeName(stmt)
		}
		statement.DDL = &spec.DDL{
			PGTypeChange: pgTypeChangeDeclaration(stmt.GetObject().GetList().GetItems(), "rename", stmt.GetNewname()),
			Operation:    spec.DDLOperationAlterType,
			ObjectName:   objectName,
			ObjectType:   "type",
			Options:      map[string]string{"type_kind": "composite", "action": "rename", "new_name": stmt.GetNewname()},
		}
		return statement
	case pg_query.ObjectType_OBJECT_ATTRIBUTE:
		typeName := ""
		if rng := stmt.GetRelation(); rng != nil {
			typeName = rng.GetRelname()
		}
		statement.DDL = &spec.DDL{
			PGComposite: pgCompositeAttributeDeclaration(stmt.GetRelation(), "rename_attribute", stmt.GetSubname(), stmt.GetNewname(), stmt.GetMissingOk()),
			Operation:   spec.DDLOperationAlterType,
			ObjectName:  typeName,
			ObjectType:  "type",
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
			Operation:             spec.DDLOperationAlterView,
			ObjectName:            objectName,
			PGViewNamespaceSource: pgViewNamespaceSource(stmt.GetRelation(), stmt.GetMissingOk()),
			ObjectType:            "view",
			Options:               map[string]string{"action": "rename_view", "new_name": stmt.GetNewname()},
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
			Operation:             spec.DDLOperationAlterMaterializedView,
			ObjectName:            objectName,
			PGViewNamespaceSource: pgViewNamespaceSource(stmt.GetRelation(), stmt.GetMissingOk()),
			ObjectType:            "materialized_view",
			Options:               map[string]string{"action": "rename", "new_name": stmt.GetNewname()},
		}
		return statement
	case pg_query.ObjectType_OBJECT_EVENT_TRIGGER:
		return extractRenameEventTrigger(statement, stmt)
	case pg_query.ObjectType_OBJECT_RULE:
		return extractRenameRule(statement, stmt)
	case pg_query.ObjectType_OBJECT_COLLATION:
		target := pgCollationName(stmt.GetObject())
		if target.Name == "" {
			target.Name = renameObjectCollationName(stmt)
		}
		objectName := target.Name
		statement.DDL = &spec.DDL{
			PGCollationChange: &spec.PGCollationChangeTarget{Source: target, Action: "rename", Destination: stmt.GetNewname(), Owner: spec.PGTableOwnerTarget{}},
			Operation:         spec.DDLOperationAlterCollation,
			ObjectName:        objectName,
			ObjectType:        "collation",
			Options:           map[string]string{"action": "rename", "new_name": stmt.GetNewname()},
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
	case pg_query.ObjectType_OBJECT_AGGREGATE:
		objectName := objectNameFromObjectWithArgsOrList(stmt.GetObject())
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationAlterAggregate,
			ObjectName: objectName,
			ObjectType: "aggregate",
			Options:    map[string]string{"action": "rename", "new_name": stmt.GetNewname()},
		}
		return statement
	case pg_query.ObjectType_OBJECT_CONVERSION:
		objectName := renameObjectCollationName(stmt)
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationAlterConversion,
			ObjectName: objectName,
			ObjectType: "conversion",
			Options:    map[string]string{"action": "rename", "new_name": stmt.GetNewname()},
		}
		return statement
	case pg_query.ObjectType_OBJECT_OPFAMILY:
		objectName, am := opNameAndAccessMethod(stmt.GetObject())
		statement.DDL = &spec.DDL{
			PGOperatorFamilyChange: pgOperatorFamilyChange(stmt.GetObject(), "rename", stmt.GetNewname(), spec.PGTableOwnerTarget{}),
			Operation:              spec.DDLOperationAlterOperatorFamily,
			ObjectName:             objectName,
			ObjectType:             "operator_family",
			Options:                map[string]string{"action": "rename", "new_name": stmt.GetNewname(), "access_method": am},
		}
		return statement
	case pg_query.ObjectType_OBJECT_OPCLASS:
		objectName, am := opNameAndAccessMethod(stmt.GetObject())
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationAlterOperatorClass,
			ObjectName: objectName,
			ObjectType: "operator_class",
			Options:    map[string]string{"action": "rename", "new_name": stmt.GetNewname(), "access_method": am},
		}
		return statement

	case pg_query.ObjectType_OBJECT_TSCONFIGURATION:
		objectName := renameObjectCollationName(stmt)
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationAlterTextSearchConfiguration,
			ObjectName: objectName,
			ObjectType: "text_search_configuration",
			Options:    map[string]string{"action": "rename", "new_name": stmt.GetNewname()},
		}
		return statement
	case pg_query.ObjectType_OBJECT_TSDICTIONARY:
		objectName := renameObjectCollationName(stmt)
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationAlterTextSearchDictionary,
			ObjectName: objectName,
			ObjectType: "text_search_dictionary",
			Options:    map[string]string{"action": "rename", "new_name": stmt.GetNewname()},
		}
		return statement
	case pg_query.ObjectType_OBJECT_TSPARSER:
		objectName := renameObjectCollationName(stmt)
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationAlterTextSearchParser,
			ObjectName: objectName,
			ObjectType: "text_search_parser",
			Options:    map[string]string{"action": "rename", "new_name": stmt.GetNewname()},
		}
		return statement
	case pg_query.ObjectType_OBJECT_TSTEMPLATE:
		objectName := renameObjectCollationName(stmt)
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationAlterTextSearchTemplate,
			ObjectName: objectName,
			ObjectType: "text_search_template",
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
			Operation:               spec.DDLOperationAlterTable,
			PGTableIfExists:         stmt.GetRelationType() == pg_query.ObjectType_OBJECT_TABLE && stmt.GetMissingOk(),
			PGTableCatalogQualified: stmt.GetRelation().GetCatalogname() != "",
			PGTableOnly:             !stmt.GetRelation().GetInh(),
			Table:                   table,
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
			Operation:               spec.DDLOperationAlterTable,
			PGTableIfExists:         stmt.GetMissingOk(),
			PGTableCatalogQualified: stmt.GetRelation().GetCatalogname() != "",
			Table:                   table,
			Alter:                   []spec.Alter{{Action: "rename_table", Name: table.Name, Options: map[string]string{"new_name": stmt.GetNewname()}}},
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
		if table != nil {
			statement.DDL.PGIndexDestination = &spec.PGIndexDestinationTarget{
				Source: spec.PGDestructiveTableTarget{Table: *table, CatalogQualified: stmt.GetRelation().GetCatalogname() != ""},
				Action: "rename", Destination: stmt.GetNewname(), IfExists: stmt.GetMissingOk(),
			}
		}
		return statement
	default:
		return unsupportedStatement(statement, "rename", "postgresql rename target is not in the approved v1 subset")
	}
}

func pgViewNamespaceSource(relation *pg_query.RangeVar, guard bool) *spec.PGViewNamespaceSource {
	if relation == nil {
		return nil
	}
	return &spec.PGViewNamespaceSource{Schema: relation.GetSchemaname(), Name: relation.GetRelname(), IfExists: guard,
		CatalogQualified: relation.GetCatalogname() != ""}
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
			Operation:             spec.DDLOperationAlterView,
			ObjectName:            objectName,
			PGViewNamespaceSource: pgViewNamespaceSource(stmt.GetRelation(), stmt.GetMissingOk()),
			ObjectType:            "view",
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
			Operation:             spec.DDLOperationAlterMaterializedView,
			ObjectName:            objectName,
			PGViewNamespaceSource: pgViewNamespaceSource(stmt.GetRelation(), stmt.GetMissingOk()),
			ObjectType:            "materialized_view",
			Options: map[string]string{
				"action":     "set_schema",
				"new_schema": stmt.GetNewschema(),
			},
		}
		return statement
	}
	if stmt.GetObjectType() == pg_query.ObjectType_OBJECT_COLLATION {
		target := pgCollationName(stmt.GetObject())
		if target.Name == "" {
			target.Name = alterObjectSchemaObjectName(stmt)
		}
		objectName := target.Name
		statement.DDL = &spec.DDL{
			PGCollationChange: &spec.PGCollationChangeTarget{Source: target, Action: "set_schema", Destination: stmt.GetNewschema(), Owner: spec.PGTableOwnerTarget{}},
			Operation:         spec.DDLOperationAlterCollation,
			ObjectName:        objectName,
			ObjectType:        "collation",
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
	if stmt.GetObjectType() == pg_query.ObjectType_OBJECT_AGGREGATE {
		objectName := objectNameFromObjectWithArgsOrList(stmt.GetObject())
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationAlterAggregate,
			ObjectName: objectName,
			ObjectType: "aggregate",
			Options: map[string]string{
				"action":     "set_schema",
				"new_schema": stmt.GetNewschema(),
			},
		}
		return statement
	}
	if stmt.GetObjectType() == pg_query.ObjectType_OBJECT_OPERATOR {
		objectName := objectNameFromObjectWithArgsOrList(stmt.GetObject())
		statement.DDL = &spec.DDL{
			PGOperatorChange: pgOperatorChangeDeclaration(stmt.GetObject().GetObjectWithArgs(), "set_schema", stmt.GetNewschema(), spec.PGTableOwnerTarget{}),
			Operation:        spec.DDLOperationAlterOperator,
			ObjectName:       objectName,
			ObjectType:       "operator",
			Options: map[string]string{
				"action":     "set_schema",
				"new_schema": stmt.GetNewschema(),
			},
		}
		return statement
	}
	if stmt.GetObjectType() == pg_query.ObjectType_OBJECT_CONVERSION {
		objectName := alterObjectSchemaObjectName(stmt)
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationAlterConversion,
			ObjectName: objectName,
			ObjectType: "conversion",
			Options: map[string]string{
				"action":     "set_schema",
				"new_schema": stmt.GetNewschema(),
			},
		}
		return statement
	}
	if stmt.GetObjectType() == pg_query.ObjectType_OBJECT_OPFAMILY {
		objectName, am := opNameAndAccessMethod(stmt.GetObject())
		statement.DDL = &spec.DDL{
			PGOperatorFamilyChange: pgOperatorFamilyChange(stmt.GetObject(), "set_schema", stmt.GetNewschema(), spec.PGTableOwnerTarget{}),
			Operation:              spec.DDLOperationAlterOperatorFamily,
			ObjectName:             objectName,
			ObjectType:             "operator_family",
			Options: map[string]string{
				"action":        "set_schema",
				"new_schema":    stmt.GetNewschema(),
				"access_method": am,
			},
		}
		return statement
	}
	if stmt.GetObjectType() == pg_query.ObjectType_OBJECT_OPCLASS {
		objectName, am := opNameAndAccessMethod(stmt.GetObject())
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationAlterOperatorClass,
			ObjectName: objectName,
			ObjectType: "operator_class",
			Options: map[string]string{
				"action":        "set_schema",
				"new_schema":    stmt.GetNewschema(),
				"access_method": am,
			},
		}
		return statement
	}
	if stmt.GetObjectType() == pg_query.ObjectType_OBJECT_TSCONFIGURATION {
		objectName := alterObjectSchemaObjectName(stmt)
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationAlterTextSearchConfiguration,
			ObjectName: objectName,
			ObjectType: "text_search_configuration",
			Options: map[string]string{
				"action":     "set_schema",
				"new_schema": stmt.GetNewschema(),
			},
		}
		return statement
	}
	if stmt.GetObjectType() == pg_query.ObjectType_OBJECT_TSDICTIONARY {
		objectName := alterObjectSchemaObjectName(stmt)
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationAlterTextSearchDictionary,
			ObjectName: objectName,
			ObjectType: "text_search_dictionary",
			Options: map[string]string{
				"action":     "set_schema",
				"new_schema": stmt.GetNewschema(),
			},
		}
		return statement
	}
	if stmt.GetObjectType() == pg_query.ObjectType_OBJECT_TSPARSER {
		objectName := alterObjectSchemaObjectName(stmt)
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationAlterTextSearchParser,
			ObjectName: objectName,
			ObjectType: "text_search_parser",
			Options: map[string]string{
				"action":     "set_schema",
				"new_schema": stmt.GetNewschema(),
			},
		}
		return statement
	}
	if stmt.GetObjectType() == pg_query.ObjectType_OBJECT_TSTEMPLATE {
		objectName := alterObjectSchemaObjectName(stmt)
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationAlterTextSearchTemplate,
			ObjectName: objectName,
			ObjectType: "text_search_template",
			Options: map[string]string{
				"action":     "set_schema",
				"new_schema": stmt.GetNewschema(),
			},
		}
		return statement
	}
	statement.DDL = &spec.DDL{
		Operation:               spec.DDLOperationAlterTable,
		PGTableIfExists:         stmt.GetObjectType() == pg_query.ObjectType_OBJECT_TABLE && stmt.GetMissingOk(),
		PGTableCatalogQualified: stmt.GetRelation().GetCatalogname() != "",
		Table:                   tableFromRangeVar(stmt.GetRelation()),
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
	decl := pgAlterEnumDeclaration(stmt, statement.RawSQL)
	options := map[string]string{
		"type_kind": "enum", "action": "add_value", "value": decl.Value,
		"if_not_exists": strconv.FormatBool(decl.IfNotExists),
	}
	if decl.Old != nil {
		options["action"] = "rename_value"
	} else if decl.Neighbor != nil {
		options["placement"] = "before"
		if stmt.GetNewValIsAfter() {
			options["placement"] = "after"
		}
		options["neighbor"] = *decl.Neighbor
	}
	statement.DDL = &spec.DDL{
		PGAlterEnum: decl,
		Operation:   spec.DDLOperationAlterType,
		ObjectName:  objectName,
		ObjectType:  "type",
		Options:     options,
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
func pgCompositeAttributeDeclaration(relation *pg_query.RangeVar, action, name, newName string, ifExists bool) *spec.PGCompositeDeclaration {
	return &spec.PGCompositeDeclaration{
		Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: relation.GetSchemaname(), Name: relation.GetRelname()}, CatalogQualified: relation.GetCatalogname() != ""},
		Action: action, Name: name, NewName: newName, IfExists: ifExists,
	}
}

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

	var action, name string
	switch cmd.GetSubtype() {
	case pg_query.AlterTableType_AT_AddColumn:
		action, name = "add_attribute", cmd.GetDef().GetColumnDef().GetColname()
	case pg_query.AlterTableType_AT_DropColumn:
		action, name = "drop_attribute", cmd.GetName()
	case pg_query.AlterTableType_AT_AlterColumnType:
		action, name = "alter_attribute_type", cmd.GetName()
	}
	if len(stmt.GetCmds()) != 1 {
		action = ""
	}
	decl := pgCompositeAttributeDeclaration(stmt.GetRelation(), action, name, "", cmd.GetMissingOk())

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
			PGComposite: decl,
			Operation:   spec.DDLOperationAlterType,
			ObjectName:  objectName,
			ObjectType:  "type",
			Options:     options,
		}
		return statement
	case pg_query.AlterTableType_AT_DropColumn:
		statement.DDL = &spec.DDL{
			PGComposite: decl,
			Operation:   spec.DDLOperationAlterType,
			ObjectName:  objectName,
			ObjectType:  "type",
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
			PGComposite: decl,
			Operation:   spec.DDLOperationAlterType,
			ObjectName:  objectName,
			ObjectType:  "type",
			Options:     options,
		}
		return statement
	default:
		return unsupportedStatement(statement, "alter_type", fmt.Sprintf("postgresql alter type composite action %s is deferred", cmd.GetSubtype()))
	}
}

func pgTypeChangeDeclaration(names []*pg_query.Node, action, destination string) *spec.PGTypeChange {
	decl := &spec.PGTypeChange{Action: action, Destination: destination}
	if len(names) == 2 {
		decl.Source.Table = spec.Table{Schema: stringNodeValue(names[0]), Name: stringNodeValue(names[1])}
	}
	return decl
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
		PGTypeChange: pgTypeChangeDeclaration(stmt.GetObject().GetList().GetItems(), "set_schema", stmt.GetNewschema()),
		Operation:    spec.DDLOperationAlterType,
		ObjectName:   objectName,
		ObjectType:   "type",
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

func extractAlterTableMoveAllStmt(statement spec.Statement, stmt *pg_query.AlterTableMoveAllStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "alter_table_move_all", "postgresql alter table move all payload is missing")
	}
	if stmt.GetObjtype() != pg_query.ObjectType_OBJECT_TABLE {
		return unsupportedStatement(statement, "alter_table_move_all", "postgresql alter table move all object type is unsupported in v1")
	}
	source := stmt.GetOrigTablespacename()
	target := stmt.GetNewTablespacename()
	if source == "" || target == "" {
		return unsupportedStatement(statement, "alter_table_move_all", "postgresql alter table move all tablespace names are missing")
	}
	move := &spec.PGBulkTablespaceTarget{Source: source, Destination: target, Nowait: stmt.GetNowait()}
	for _, node := range stmt.GetRoles() {
		role := pgRoleSelection(node.GetRoleSpec())
		if !role.Valid() {
			// Keep the existing normalized action when the role cannot be modeled.
			move = nil
			break
		}
		move.Roles = append(move.Roles, role)
	}
	statement.DDL = &spec.DDL{
		Operation: spec.DDLOperationAlterTable,
		Alter: []spec.Alter{{
			Action:           "move_all_tablespace",
			PGBulkTablespace: move,
			Options: map[string]string{
				"object_type":        "table",
				"source_tablespace":  source,
				"target_tablespace":  target,
				"has_table_identity": "false",
			},
		}},
	}
	return statement
}

func extractAlterViewAction(statement spec.Statement, stmt *pg_query.AlterTableStmt, operation spec.DDLOperation, kind string) spec.Statement {
	commands := stmt.GetCmds()
	table := tableFromRangeVar(stmt.GetRelation())
	if table == nil || len(commands) != 1 {
		return unsupportedStatement(statement, "alter_table", "postgresql alter table object type is unsupported in v1")
	}
	cmd := commands[0].GetAlterTableCmd()
	ddl := &spec.DDL{Operation: operation, ObjectType: kind, ObjectName: table.Name, Table: table,
		PGTableIfExists: stmt.GetMissingOk(), PGTableCatalogQualified: stmt.GetRelation().GetCatalogname() != ""}
	switch cmd.GetSubtype() {
	case pg_query.AlterTableType_AT_ChangeOwner:
		owner := pgRoleSelection(cmd.GetNewowner())
		if !owner.Valid() {
			return unsupportedStatement(statement, "alter_table", "postgresql view owner is unsupported in v1")
		}
		ddl.Alter = []spec.Alter{{Action: "change_owner", PGTableOwner: owner}}
	case pg_query.AlterTableType_AT_SetRelOptions, pg_query.AlterTableType_AT_ResetRelOptions:
		if len(cmd.GetDef().GetList().GetItems()) == 0 {
			return unsupportedStatement(statement, "alter_table", "postgresql alter table object type is unsupported in v1")
		}
		reset := cmd.GetSubtype() == pg_query.AlterTableType_AT_ResetRelOptions
		ddl.Alter = []spec.Alter{{Action: "set_options"}}
		if reset {
			ddl.Alter[0].Action = "reset_options"
		}
		items := cmd.GetDef().GetList().GetItems()
		if operation == spec.DDLOperationAlterMaterializedView {
			ddl.Alter[0].PGTableReloptions = pgTableReloptions(items, reset, true)
		} else {
			ddl.PGViewOptions = pgViewOptions(items)
		}
	case pg_query.AlterTableType_AT_ColumnDefault:
		if operation != spec.DDLOperationAlterView || cmd.GetName() == "" {
			return unsupportedStatement(statement, "alter_table", "postgresql alter table object type is unsupported in v1")
		}
		alter, _, _ := alterFromCmd(cmd)
		ddl.Alter = []spec.Alter{alter}
	default:
		return unsupportedStatement(statement, "alter_table", "postgresql alter table object type is unsupported in v1")
	}
	statement.DDL = ddl
	return statement
}

func pgRoleSelection(role *pg_query.RoleSpec) spec.PGTableOwnerTarget {
	switch role.GetRoletype() {
	case pg_query.RoleSpecType_ROLESPEC_CSTRING:
		return spec.PGTableOwnerTarget{Kind: "named", Name: role.GetRolename()}
	case pg_query.RoleSpecType_ROLESPEC_CURRENT_USER:
		return spec.PGTableOwnerTarget{Kind: "current_user"}
	case pg_query.RoleSpecType_ROLESPEC_CURRENT_ROLE:
		return spec.PGTableOwnerTarget{Kind: "current_role"}
	case pg_query.RoleSpecType_ROLESPEC_SESSION_USER:
		return spec.PGTableOwnerTarget{Kind: "session_user"}
	case pg_query.RoleSpecType_ROLESPEC_PUBLIC:
		return spec.PGTableOwnerTarget{Kind: "public"}
	default:
		return spec.PGTableOwnerTarget{}
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
		return spec.Alter{
			Action:                 "add_column",
			Name:                   column.GetColname(),
			Column:                 &spec.AlterColumn{Definition: columnPtr(columnFromDef(column))},
			PGAddColumnIfNotExists: cmd.GetMissingOk(),
		}, true, nil
	case pg_query.AlterTableType_AT_DropColumn:
		return spec.Alter{Action: "drop_column", Name: cmd.GetName(), Column: &spec.AlterColumn{OldName: cmd.GetName()}, Options: map[string]string{
			"if_exists": strconv.FormatBool(cmd.GetMissingOk()),
			"cascade":   strconv.FormatBool(cmd.GetBehavior() == pg_query.DropBehavior_DROP_CASCADE),
		}}, true, nil
	case pg_query.AlterTableType_AT_DropExpression:
		return spec.Alter{Action: "drop_expression", Name: cmd.GetName(), Column: &spec.AlterColumn{OldName: cmd.GetName()}, PGColumnTransition: pgColumnTransitionFromCmd(cmd)}, true, nil
	case pg_query.AlterTableType_AT_SetExpression:
		columnName := cmd.GetName()
		if columnName == "" {
			return spec.Alter{}, false, &spec.UnsupportedDetail{Feature: "setexpression", Reason: "postgresql alter table set expression column name is missing"}
		}
		return spec.Alter{
			Action: "set_expression",
			Name:   columnName,
			Column: &spec.AlterColumn{OldName: columnName},
			Options: map[string]string{
				"column":         columnName,
				"has_expression": "true",
			},
		}, true, nil
	case pg_query.AlterTableType_AT_SetIdentity:
		generatedWhen := generatedWhenFromDef(cmd.GetDef())
		return spec.Alter{
			Action:             "set_generated",
			Name:               cmd.GetName(),
			Column:             &spec.AlterColumn{OldName: cmd.GetName()},
			PGColumnTransition: pgColumnTransitionFromCmd(cmd),
			Options: map[string]string{
				"generated_when": generatedWhen,
			},
		}, true, nil
	case pg_query.AlterTableType_AT_DropIdentity:
		return spec.Alter{Action: "drop_identity", Name: cmd.GetName(), Column: &spec.AlterColumn{OldName: cmd.GetName()}, PGColumnTransition: pgColumnTransitionFromCmd(cmd)}, true, nil
	case pg_query.AlterTableType_AT_AddIdentity:
		columnName := cmd.GetName()
		if columnName == "" {
			return spec.Alter{}, false, &spec.UnsupportedDetail{Feature: "addidentity", Reason: "postgresql alter table add identity column name is missing"}
		}
		options := map[string]string{
			"column":   columnName,
			"identity": "true",
		}
		if generatedWhen := identityGeneratedWhenFromConstraintDef(cmd.GetDef()); generatedWhen != "" {
			options["generated_when"] = generatedWhen
		}
		return spec.Alter{
			Action:             "add_identity",
			Name:               columnName,
			Column:             &spec.AlterColumn{OldName: columnName},
			Options:            options,
			PGColumnTransition: pgColumnTransitionFromCmd(cmd),
		}, true, nil
	case pg_query.AlterTableType_AT_AddConstraint:
		constraint := cmd.GetDef().GetConstraint()
		if constraint == nil {
			return spec.Alter{}, false, &spec.UnsupportedDetail{Feature: "alter_table", Reason: "postgresql add constraint payload is missing"}
		}
		if constraint.GetContype() == pg_query.ConstrType_CONSTR_EXCLUSION {
			constraintName := constraint.GetConname()
			if constraintName == "" {
				return spec.Alter{}, false, &spec.UnsupportedDetail{Feature: "add_constraint", Reason: "postgresql exclusion constraint name is missing"}
			}
			options := map[string]string{
				"constraint":      constraintName,
				"constraint_type": "exclusion",
			}
			if accessMethod := constraint.GetAccessMethod(); accessMethod != "" {
				options["access_method"] = accessMethod
			}
			return spec.Alter{Action: "add_exclusion_constraint", Name: constraintName, Options: options}, true, nil
		}
		constraintType, ok := supportedConstraintType(constraint.GetContype())
		if !ok {
			return spec.Alter{}, false, &spec.UnsupportedDetail{Feature: "add_constraint", Reason: "postgresql constraint type is not in the approved v1 subset"}
		}
		options := map[string]string{
			"constraint_type": constraintType,
			"not_valid":       strconv.FormatBool(constraint.GetSkipValidation()),
		}
		var keyColumns, includeColumns []string
		if constraintType == "primary_key" || constraintType == "unique" {
			options["using_index"] = strconv.FormatBool(constraint.GetIndexname() != "")
			keyColumns = pgExactConstraintColumns(constraint.GetKeys())
			includeColumns = pgExactConstraintColumns(constraint.GetIncluding())
		}
		var foreignKey *spec.PGForeignKeySelection
		if constraint.GetContype() == pg_query.ConstrType_CONSTR_FOREIGN {
			reference := constraint.GetPktable()
			columns := stringValuesFromNodes(constraint.GetFkAttrs())
			referencedColumns := stringValuesFromNodes(constraint.GetPkAttrs())
			if table := tableFromRangeVar(reference); table != nil {
				foreignKey = &spec.PGForeignKeySelection{Reference: *table, ReferenceCatalogQualified: reference.GetCatalogname() != "", Columns: columns, ReferencedColumns: referencedColumns, DeleteSetColumns: stringValuesFromNodes(constraint.GetFkDelSetCols())}
			}
			if len(columns) > 0 {
				options["columns"] = strings.Join(columns, ",")
			}
			if refTable := rangeVarName(constraint.GetPktable()); refTable != "" {
				options["referenced_table"] = refTable
			}
			if len(referencedColumns) > 0 {
				options["referenced_columns"] = strings.Join(referencedColumns, ",")
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
		alter := spec.Alter{Action: "add_constraint", Name: constraint.GetConname(), Options: options, PGUsingIndexName: constraint.GetIndexname(), PGForeignKey: foreignKey,
			PGKeyColumns: keyColumns, PGIncludeColumns: includeColumns}
		if constraint.GetContype() == pg_query.ConstrType_CONSTR_CHECK {
			expr := constraint.GetRawExpr()
			alter.PGCheckHasSubquery = pgExpressionHasReference(expr, false)
			alter.PGCheckLiteral = pgCheckLiteral(expr)
			fields := expr.GetNullTest().GetArg().GetColumnRef().GetFields()
			if len(fields) == 1 || len(fields) == 2 {
				names := stringValuesFromNodes(fields)
				if len(names) == len(fields) {
					alter.PGCheckNullTest = names
				}
			}
		}
		return alter, true, nil
	case pg_query.AlterTableType_AT_DropConstraint:
		return spec.Alter{Action: "drop_constraint", Name: cmd.GetName(), Options: map[string]string{
			"if_exists": strconv.FormatBool(cmd.GetMissingOk()),
			"cascade":   strconv.FormatBool(cmd.GetBehavior() == pg_query.DropBehavior_DROP_CASCADE),
		}}, true, nil
	case pg_query.AlterTableType_AT_ValidateConstraint:
		return spec.Alter{Action: "validate_constraint", Name: cmd.GetName()}, true, nil
	case pg_query.AlterTableType_AT_AlterColumnType:
		column := cmd.GetDef().GetColumnDef()
		if column == nil || column.GetTypeName() == nil {
			return spec.Alter{}, false, &spec.UnsupportedDetail{Feature: "alter_column_type", Reason: "postgresql alter column type payload is missing"}
		}
		alter := spec.Alter{Action: "set_data_type", Name: cmd.GetName(), Column: &spec.AlterColumn{OldName: cmd.GetName(), Definition: &spec.Column{Name: cmd.GetName(), Type: typeNameString(column.GetTypeName())}}}
		target := column.GetTypeName()
		alter.PGColumnTypeModifiers = pgColumnTypeModifiers(target.GetTypmods())
		if names := target.GetNames(); len(names) == 2 && !target.GetPctType() {
			alter.PGColumnType = spec.PGColumnTypeTarget{Schema: stringNodeValue(names[0]), Name: stringNodeValue(names[1]), Array: len(target.GetArrayBounds()) > 0}
		}
		if clause := column.GetCollClause(); clause != nil {
			alter.PGColumnCollation = &spec.PGColumnCollation{}
			if names := clause.GetCollname(); len(names) == 2 {
				alter.PGColumnCollation.Schema = stringNodeValue(names[0])
				alter.PGColumnCollation.Name = stringNodeValue(names[1])
			}
		}
		if column.GetRawDefault() != nil {
			alter.Options = map[string]string{"has_using": "true"}
		}
		return alter, true, nil
	case pg_query.AlterTableType_AT_ColumnDefault:
		action := "set_default"
		if cmd.GetDef() == nil {
			action = "drop_default"
		}
		return spec.Alter{Action: action, Name: cmd.GetName(), Column: &spec.AlterColumn{OldName: cmd.GetName(), Change: &spec.AlterColumnChange{TouchesDefault: true}}, PGColumnTransition: pgColumnTransitionFromCmd(cmd)}, true, nil
	case pg_query.AlterTableType_AT_SetNotNull:
		return spec.Alter{Action: "set_not_null", Name: cmd.GetName(), Column: &spec.AlterColumn{OldName: cmd.GetName(), Definition: &spec.Column{Name: cmd.GetName(), NotNull: true}, Change: &spec.AlterColumnChange{TouchesNullability: true}}, PGColumnTransition: pgColumnTransitionFromCmd(cmd)}, true, nil
	case pg_query.AlterTableType_AT_DropNotNull:
		return spec.Alter{Action: "drop_not_null", Name: cmd.GetName(), Column: &spec.AlterColumn{OldName: cmd.GetName(), Definition: &spec.Column{Name: cmd.GetName(), NotNull: false}, Change: &spec.AlterColumnChange{TouchesNullability: true}}, PGColumnTransition: pgColumnTransitionFromCmd(cmd)}, true, nil
	case pg_query.AlterTableType_AT_ChangeOwner:
		target := pgRoleSelection(cmd.GetNewowner())
		if !target.Valid() {
			return spec.Alter{}, false, &spec.UnsupportedDetail{Feature: "changeowner", Reason: "postgresql alter table owner role is missing"}
		}
		return spec.Alter{Action: "change_owner", PGTableOwner: target}, true, nil
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
	case pg_query.AlterTableType_AT_EnableReplicaTrig:
		name := cmd.GetName()
		return spec.Alter{Action: "enable_replica_trigger", Name: name, Options: map[string]string{"trigger": name, "trigger_mode": "replica"}}, true, nil
	case pg_query.AlterTableType_AT_EnableAlwaysTrig:
		name := cmd.GetName()
		return spec.Alter{Action: "enable_always_trigger", Name: name, Options: map[string]string{"trigger": name, "trigger_mode": "always"}}, true, nil
	case pg_query.AlterTableType_AT_EnableRule:
		name := cmd.GetName()
		return spec.Alter{Action: "enable_rule", Name: name, Options: map[string]string{"rule": name}}, true, nil
	case pg_query.AlterTableType_AT_DisableRule:
		name := cmd.GetName()
		return spec.Alter{Action: "disable_rule", Name: name, Options: map[string]string{"rule": name}}, true, nil
	case pg_query.AlterTableType_AT_EnableReplicaRule:
		name := cmd.GetName()
		return spec.Alter{Action: "enable_replica_rule", Name: name, Options: map[string]string{"rule": name, "rule_mode": "replica"}}, true, nil
	case pg_query.AlterTableType_AT_EnableAlwaysRule:
		name := cmd.GetName()
		return spec.Alter{Action: "enable_always_rule", Name: name, Options: map[string]string{"rule": name, "rule_mode": "always"}}, true, nil
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
		tablespace := cmd.GetName()
		return spec.Alter{Action: "set_tablespace", Options: map[string]string{"tablespace": tablespace}}, true, nil
	case pg_query.AlterTableType_AT_SetAccessMethod:
		method := cmd.GetName()
		options := map[string]string{}
		if method == "" {
			options["access_method"] = "default"
			options["is_default"] = "true"
		} else {
			options["access_method"] = method
			options["is_default"] = "false"
		}
		return spec.Alter{Action: "set_access_method", Options: options}, true, nil
	case pg_query.AlterTableType_AT_EnableRowSecurity:
		return spec.Alter{Action: "enable_rls"}, true, nil
	case pg_query.AlterTableType_AT_DisableRowSecurity:
		return spec.Alter{Action: "disable_rls"}, true, nil
	case pg_query.AlterTableType_AT_ForceRowSecurity:
		return spec.Alter{Action: "force_rls"}, true, nil
	case pg_query.AlterTableType_AT_NoForceRowSecurity:
		return spec.Alter{Action: "no_force_rls"}, true, nil
	case pg_query.AlterTableType_AT_SetRelOptions, pg_query.AlterTableType_AT_ResetRelOptions:
		items := cmd.GetDef().GetList().GetItems()
		reset := cmd.GetSubtype() == pg_query.AlterTableType_AT_ResetRelOptions
		action, countKey := "set_reloptions", "option_count"
		if reset {
			action, countKey = "reset_reloptions", "reset_count"
		}
		return spec.Alter{Action: action, Options: map[string]string{countKey: strconv.Itoa(len(items)), "has_reloptions": "true"},
			PGTableReloptions: pgTableReloptions(items, reset, false)}, true, nil
	case pg_query.AlterTableType_AT_SetStatistics:
		kind := "default"
		if cmd.GetDef() != nil {
			kind = "value"
		}
		return spec.Alter{
			Action:         "set_statistics",
			Name:           cmd.GetName(),
			PGColumnTuning: pgColumnTuningFromCmd(cmd),
			Options: map[string]string{
				"column":                 cmd.GetName(),
				"statistics_target_kind": kind,
				"has_statistics_target":  "true",
			},
		}, true, nil
	case pg_query.AlterTableType_AT_SetOptions:
		count := 0
		if defNode := cmd.GetDef(); defNode != nil {
			if list := defNode.GetList(); list != nil {
				count = len(list.GetItems())
			}
		}
		return spec.Alter{
			Action:         "set_column_options",
			Name:           cmd.GetName(),
			PGColumnTuning: pgColumnTuningFromCmd(cmd),
			Options: map[string]string{
				"column":             cmd.GetName(),
				"option_count":       strconv.Itoa(count),
				"has_column_options": "true",
			},
		}, true, nil
	case pg_query.AlterTableType_AT_ResetOptions:
		count := 0
		if defNode := cmd.GetDef(); defNode != nil {
			if list := defNode.GetList(); list != nil {
				count = len(list.GetItems())
			}
		}
		return spec.Alter{
			Action:         "reset_column_options",
			Name:           cmd.GetName(),
			PGColumnTuning: pgColumnTuningFromCmd(cmd),
			Options: map[string]string{
				"column":             cmd.GetName(),
				"reset_count":        strconv.Itoa(count),
				"has_column_options": "true",
			},
		}, true, nil
	case pg_query.AlterTableType_AT_SetStorage:
		storageKind := ""
		if defNode := cmd.GetDef(); defNode != nil {
			if s := defNode.GetString_(); s != nil {
				storageKind = s.GetSval()
			}
		}
		return spec.Alter{
			Action: "set_storage",
			Name:   cmd.GetName(),
			Options: map[string]string{
				"column":              cmd.GetName(),
				"storage_kind":        storageKind,
				"has_storage_setting": "true",
			},
		}, true, nil
	case pg_query.AlterTableType_AT_SetCompression:
		var method *string
		if value := cmd.GetDef().GetString_(); value != nil {
			name := value.GetSval()
			method = &name
		}
		return spec.Alter{
			Action:              "set_compression",
			Name:                cmd.GetName(),
			PGCompressionMethod: method,
			Options: map[string]string{
				"column":          cmd.GetName(),
				"has_compression": "true",
			},
		}, true, nil
	case pg_query.AlterTableType_AT_ClusterOn:
		indexName := cmd.GetName()
		return spec.Alter{
			Action: "cluster_on",
			Name:   indexName,
			Options: map[string]string{
				"index":             indexName,
				"has_cluster_index": "true",
			},
		}, true, nil
	case pg_query.AlterTableType_AT_DropCluster:
		return spec.Alter{
			Action: "set_without_cluster",
			Options: map[string]string{
				"has_cluster_index": "false",
			},
		}, true, nil
	case pg_query.AlterTableType_AT_DetachPartitionFinalize:
		partName := ""
		if def := cmd.GetDef(); def != nil {
			if pc := def.GetPartitionCmd(); pc != nil {
				if rv := pc.GetName(); rv != nil {
					partName = rv.GetRelname()
				}
			}
		}
		return spec.Alter{
			Action: "detach_partition_finalize",
			Name:   partName,
			Options: map[string]string{
				"partition": partName,
				"finalize":  "true",
			},
		}, true, nil
	case pg_query.AlterTableType_AT_AddInherit:
		parentTable := ""
		if def := cmd.GetDef(); def != nil {
			if rv := def.GetRangeVar(); rv != nil {
				parentTable = rv.GetRelname()
			}
		}
		if parentTable == "" {
			return spec.Alter{}, false, &spec.UnsupportedDetail{Feature: "addinherit", Reason: "postgresql alter table add inherit parent table is missing"}
		}
		return spec.Alter{
			Action: "add_inherit",
			Name:   parentTable,
			Options: map[string]string{
				"parent_table": parentTable,
				"relationship": "inheritance",
			},
		}, true, nil
	case pg_query.AlterTableType_AT_DropInherit:
		parentTable := ""
		if def := cmd.GetDef(); def != nil {
			if rv := def.GetRangeVar(); rv != nil {
				parentTable = rv.GetRelname()
			}
		}
		if parentTable == "" {
			return spec.Alter{}, false, &spec.UnsupportedDetail{Feature: "dropinherit", Reason: "postgresql alter table drop inherit parent table is missing"}
		}
		return spec.Alter{
			Action: "drop_inherit",
			Name:   parentTable,
			Options: map[string]string{
				"parent_table": parentTable,
				"relationship": "inheritance",
			},
		}, true, nil
	case pg_query.AlterTableType_AT_AddOf:
		typeName := ""
		if def := cmd.GetDef(); def != nil {
			if tn := def.GetTypeName(); tn != nil {
				typeName = typeNameString(tn)
			}
		}
		if typeName == "" {
			return spec.Alter{}, false, &spec.UnsupportedDetail{Feature: "addof", Reason: "postgresql alter table add of type name is missing"}
		}
		return spec.Alter{
			Action: "add_of_type",
			Name:   typeName,
			Options: map[string]string{
				"type":         typeName,
				"relationship": "typed_table",
			},
		}, true, nil
	case pg_query.AlterTableType_AT_DropOf:
		return spec.Alter{
			Action: "drop_of_type",
			Options: map[string]string{
				"relationship": "typed_table",
			},
		}, true, nil
	case pg_query.AlterTableType_AT_AlterConstraint:
		constraint := cmd.GetDef().GetConstraint()
		if constraint == nil {
			return spec.Alter{}, false, &spec.UnsupportedDetail{Feature: "alterconstraint", Reason: "postgresql alter constraint payload is missing"}
		}
		if constraint.GetContype() != pg_query.ConstrType_CONSTR_FOREIGN {
			return spec.Alter{}, false, &spec.UnsupportedDetail{Feature: "alterconstraint", Reason: "postgresql alter constraint type is not foreign_key"}
		}
		action := "alter_constraint_deferrable"
		if constraint.GetInitdeferred() {
			action = "alter_constraint_initially_deferred"
		}
		return spec.Alter{
			Action: action,
			Name:   constraint.GetConname(),
			Options: map[string]string{
				"constraint_type":    "foreign_key",
				"deferrable":         strconv.FormatBool(constraint.GetDeferrable()),
				"initially_deferred": strconv.FormatBool(constraint.GetInitdeferred()),
			},
		}, true, nil
	case pg_query.AlterTableType_AT_DropOids:
		return spec.Alter{Action: "set_without_oids"}, true, nil
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

// identityGeneratedWhenFromConstraintDef extracts the generated mode from an
// AT_AddIdentity def payload that wraps a CONSTR_IDENTITY constraint.
func identityGeneratedWhenFromConstraintDef(defNode *pg_query.Node) string {
	if defNode == nil {
		return ""
	}
	constraint := defNode.GetConstraint()
	if constraint == nil || constraint.GetContype() != pg_query.ConstrType_CONSTR_IDENTITY {
		return ""
	}
	switch constraint.GetGeneratedWhen() {
	case "d":
		return "by_default"
	case "a":
		return "always"
	default:
		return ""
	}
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
	case pg_query.ObjectType_OBJECT_TYPE:
		names := stmt.GetObject().GetList().GetItems()
		target := pgTypeChangeDeclaration(names, "set_owner", "")
		target.Owner = pgRoleSelection(stmt.GetNewowner())
		statement.DDL = &spec.DDL{
			Operation: spec.DDLOperationAlterType, ObjectName: lastStringFromNodes(names), ObjectType: "type",
			Options: map[string]string{"action": "set_owner", "owner": owner}, PGTypeChange: target,
		}
		return statement
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
		target := pgCollationName(stmt.GetObject())
		if target.Name == "" {
			target.Name = alterOwnerObjectName(stmt)
		}
		objectName := target.Name
		statement.DDL = &spec.DDL{
			PGCollationChange: &spec.PGCollationChangeTarget{Source: target, Action: "set_owner", Destination: "", Owner: pgRoleSelection(stmt.GetNewowner())},
			Operation:         spec.DDLOperationAlterCollation,
			ObjectName:        objectName,
			ObjectType:        "collation",
			Options:           map[string]string{"action": "set_owner", "owner": owner},
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
	case pg_query.ObjectType_OBJECT_AGGREGATE:
		objectName := objectNameFromObjectWithArgsOrList(stmt.GetObject())
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationAlterAggregate,
			ObjectName: objectName,
			ObjectType: "aggregate",
			Options:    map[string]string{"action": "set_owner", "owner": owner},
		}
		return statement
	case pg_query.ObjectType_OBJECT_OPERATOR:
		objectName := objectNameFromObjectWithArgsOrList(stmt.GetObject())
		statement.DDL = &spec.DDL{
			PGOperatorChange: pgOperatorChangeDeclaration(stmt.GetObject().GetObjectWithArgs(), "set_owner", "", pgRoleSelection(stmt.GetNewowner())),
			Operation:        spec.DDLOperationAlterOperator,
			ObjectName:       objectName,
			ObjectType:       "operator",
			Options:          map[string]string{"action": "set_owner", "owner": owner},
		}
		return statement
	case pg_query.ObjectType_OBJECT_CONVERSION:
		objectName := alterOwnerObjectName(stmt)
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationAlterConversion,
			ObjectName: objectName,
			ObjectType: "conversion",
			Options:    map[string]string{"action": "set_owner", "owner": owner},
		}
		return statement
	case pg_query.ObjectType_OBJECT_OPFAMILY:
		objectName, am := opNameAndAccessMethod(stmt.GetObject())
		statement.DDL = &spec.DDL{
			PGOperatorFamilyChange: pgOperatorFamilyChange(stmt.GetObject(), "set_owner", "", pgRoleSelection(stmt.GetNewowner())),
			Operation:              spec.DDLOperationAlterOperatorFamily,
			ObjectName:             objectName,
			ObjectType:             "operator_family",
			Options:                map[string]string{"action": "set_owner", "owner": owner, "access_method": am},
		}
		return statement
	case pg_query.ObjectType_OBJECT_OPCLASS:
		objectName, am := opNameAndAccessMethod(stmt.GetObject())
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationAlterOperatorClass,
			ObjectName: objectName,
			ObjectType: "operator_class",
			Options:    map[string]string{"action": "set_owner", "owner": owner, "access_method": am},
		}
		return statement

	case pg_query.ObjectType_OBJECT_TSCONFIGURATION:
		objectName := alterOwnerObjectName(stmt)
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationAlterTextSearchConfiguration,
			ObjectName: objectName,
			ObjectType: "text_search_configuration",
			Options:    map[string]string{"action": "set_owner", "owner": owner},
		}
		return statement
	case pg_query.ObjectType_OBJECT_TSDICTIONARY:
		objectName := alterOwnerObjectName(stmt)
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationAlterTextSearchDictionary,
			ObjectName: objectName,
			ObjectType: "text_search_dictionary",
			Options:    map[string]string{"action": "set_owner", "owner": owner},
		}
		return statement
	case pg_query.ObjectType_OBJECT_LARGEOBJECT:
		oid := alterOwnerLargeObjectOID(stmt)
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationAlterLargeObject,
			ObjectName: oid,
			ObjectType: "large_object",
			Options:    map[string]string{"action": "set_owner", "owner": owner},
		}
		return statement
	default:
		return unsupportedStatement(statement, "alter_owner", fmt.Sprintf("postgresql alter owner for %s is deferred", stmt.GetObjectType()))
	}
}

func extractAlterIndexStmt(statement spec.Statement, stmt *pg_query.AlterTableStmt) spec.Statement {
	if len(stmt.GetCmds()) == 1 && stmt.GetCmds()[0].GetAlterTableCmd().GetSubtype() == pg_query.AlterTableType_AT_AttachPartition {
		parent := stmt.GetRelation()
		child := stmt.GetCmds()[0].GetAlterTableCmd().GetDef().GetPartitionCmd().GetName()
		p, c := tableFromRangeVar(parent), tableFromRangeVar(child)
		if p == nil || c == nil {
			return unsupportedStatement(statement, "alter_index", "postgresql index attachment target is missing")
		}
		statement.DDL = &spec.DDL{Operation: spec.DDLOperationAlterIndex, ObjectType: "index", ObjectName: p.Name,
			Alter: []spec.Alter{{Action: "attach_partition"}}, PGIndexAttachment: &spec.PGIndexAttachment{
				Parent: spec.PGDestructiveTableTarget{Table: *p, CatalogQualified: parent.GetCatalogname() != ""},
				Child:  spec.PGDestructiveTableTarget{Table: *c, CatalogQualified: child.GetCatalogname() != ""},
			}}
		return statement
	}
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
			if table != nil && len(stmt.GetCmds()) == 1 {
				statement.DDL.PGIndexDestination = &spec.PGIndexDestinationTarget{
					Source: spec.PGDestructiveTableTarget{Table: *table, CatalogQualified: stmt.GetRelation().GetCatalogname() != ""},
					Action: "set_tablespace", Destination: tablespace, IfExists: stmt.GetMissingOk(),
				}
			}
			return statement
		case pg_query.AlterTableType_AT_SetRelOptions, pg_query.AlterTableType_AT_ResetRelOptions:
			items := cmd.GetDef().GetList().GetItems()
			if table == nil || len(stmt.GetCmds()) != 1 || len(items) == 0 {
				return unsupportedStatement(statement, "alter_index", "postgresql index storage option target or action is unmodeled")
			}
			reset := cmd.GetSubtype() == pg_query.AlterTableType_AT_ResetRelOptions
			alter := spec.Alter{Action: "set_options", PGTableReloptions: pgTableReloptions(items, reset, false)}
			if reset {
				alter.Action = "reset_options"
			} else {
				for i, item := range items {
					option := &alter.PGTableReloptions[i]
					if option.Namespace == "" && option.Name == "deduplicate_items" {
						option.Value = pgBooleanReloptionValue(item.GetDefElem().GetArg())
					}
				}
			}
			statement.DDL = &spec.DDL{Operation: spec.DDLOperationAlterIndex, ObjectType: "index", ObjectName: objectName,
				Table: table, PGTableIfExists: stmt.GetMissingOk(), PGTableCatalogQualified: stmt.GetRelation().GetCatalogname() != "",
				Alter: []spec.Alter{alter}}
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

func alterOwnerLargeObjectOID(stmt *pg_query.AlterOwnerStmt) string {
	if obj := stmt.GetObject(); obj != nil {
		if iv := obj.GetInteger(); iv != nil {
			return fmt.Sprintf("%d", iv.GetIval())
		}
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
	case pg_query.ObjectType_OBJECT_TYPE:
		action := "create_base"
		if len(stmt.GetDefinition()) == 0 {
			action = "create_shell"
		}
		return extractBaseTypeDeclaration(statement, stmt.GetDefnames(), stmt.GetDefinition(), action)
	case pg_query.ObjectType_OBJECT_COLLATION:
		objectName := lastStringFromNodes(stmt.GetDefnames())
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationCreateCollation,
			ObjectName: objectName,
			ObjectType: "collation",
		}
		return statement
	case pg_query.ObjectType_OBJECT_AGGREGATE:
		objectName := lastStringFromNodes(stmt.GetDefnames())
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationCreateAggregate,
			ObjectName: objectName,
			ObjectType: "aggregate",
		}
		return statement
	case pg_query.ObjectType_OBJECT_OPERATOR:
		objectName := lastStringFromNodes(stmt.GetDefnames())
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationCreateOperator,
			ObjectName: objectName,
			ObjectType: "operator",
		}
		return statement
	case pg_query.ObjectType_OBJECT_TSCONFIGURATION:
		objectName := lastStringFromNodes(stmt.GetDefnames())
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationCreateTextSearchConfiguration,
			ObjectName: objectName,
			ObjectType: "text_search_configuration",
		}
		return statement
	case pg_query.ObjectType_OBJECT_TSDICTIONARY:
		objectName := lastStringFromNodes(stmt.GetDefnames())
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationCreateTextSearchDictionary,
			ObjectName: objectName,
			ObjectType: "text_search_dictionary",
		}
		return statement
	case pg_query.ObjectType_OBJECT_TSPARSER:
		objectName := lastStringFromNodes(stmt.GetDefnames())
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationCreateTextSearchParser,
			ObjectName: objectName,
			ObjectType: "text_search_parser",
		}
		return statement
	case pg_query.ObjectType_OBJECT_TSTEMPLATE:
		objectName := lastStringFromNodes(stmt.GetDefnames())
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationCreateTextSearchTemplate,
			ObjectName: objectName,
			ObjectType: "text_search_template",
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

func pgCollationName(node *pg_query.Node) spec.PGColumnCollation {
	parts := node.GetList().GetItems()
	out := spec.PGColumnCollation{Name: lastStringFromNodes(parts)}
	if len(parts) == 2 {
		out.Schema = parts[0].GetString_().GetSval()
	}
	return out
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

// objectNameFromObjectWithArgsOrList extracts the object name from a Node that
// may contain ObjectWithArgs (name with signature, e.g. aggregates/operators)
// or a simple List (e.g. conversions).
func objectNameFromObjectWithArgsOrList(node *pg_query.Node) string {
	if node == nil {
		return ""
	}
	if owa := node.GetObjectWithArgs(); owa != nil {
		return firstStringFromNodes(owa.GetObjname())
	}
	if list := node.GetList(); list != nil {
		return lastStringFromNodes(list.GetItems())
	}
	return firstStringFromNodes([]*pg_query.Node{node})
}

func extractCreateConversionStmt(statement spec.Statement, stmt *pg_query.CreateConversionStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "create_conversion", "postgresql create conversion statement payload is missing")
	}
	objectName := lastStringFromNodes(stmt.GetConversionName())
	statement.DDL = &spec.DDL{
		Operation:  spec.DDLOperationCreateConversion,
		ObjectName: objectName,
		ObjectType: "conversion",
	}
	return statement
}

// opNameAndAccessMethod extracts object name and access method from an object list
// where items are [access_method, object_name] (used by operator family/class).
func pgOperatorFamilyChange(object *pg_query.Node, action, destination string, owner spec.PGTableOwnerTarget) *spec.PGOperatorFamilyChange {
	parts := object.GetList().GetItems()
	name, method := opNameAndAccessMethod(object)
	source := spec.PGOperatorFamilyTarget{Name: name, Method: method}
	if len(parts) == 3 {
		source.Schema = parts[1].GetString_().GetSval()
	}
	return &spec.PGOperatorFamilyChange{Source: source, Action: action, Destination: destination, Owner: owner}
}

func opNameAndAccessMethod(node *pg_query.Node) (string, string) {
	if node == nil {
		return "", ""
	}
	list := node.GetList()
	if list == nil {
		return firstStringFromNodes([]*pg_query.Node{node}), ""
	}
	items := list.GetItems()
	name := lastStringFromNodes(items)
	am := ""
	if len(items) > 0 {
		am = firstStringFromNodes([]*pg_query.Node{items[0]})
	}
	return name, am
}

func extractCreateOpFamilyStmt(statement spec.Statement, stmt *pg_query.CreateOpFamilyStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "create_operator_family", "postgresql create operator family statement payload is missing")
	}
	parts := stmt.GetOpfamilyname()
	objectName := lastStringFromNodes(parts)
	target := &spec.PGOperatorFamilyTarget{Name: objectName, Method: stmt.GetAmname()}
	if len(parts) == 2 {
		target.Schema = parts[0].GetString_().GetSval()
	}
	statement.DDL = &spec.DDL{
		PGOperatorFamily: target,
		Operation:        spec.DDLOperationCreateOperatorFamily,
		ObjectName:       objectName,
		ObjectType:       "operator_family",
		Options:          map[string]string{"access_method": stmt.GetAmname()},
	}
	return statement
}

func extractCreateOpClassStmt(statement spec.Statement, stmt *pg_query.CreateOpClassStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "create_operator_class", "postgresql create operator class statement payload is missing")
	}
	objectName := lastStringFromNodes(stmt.GetOpclassname())
	options := map[string]string{"access_method": stmt.GetAmname()}
	if stmt.GetIsDefault() {
		options["is_default"] = "true"
	}
	statement.DDL = &spec.DDL{
		Operation:  spec.DDLOperationCreateOperatorClass,
		ObjectName: objectName,
		ObjectType: "operator_class",
		Options:    options,
	}
	return statement
}

func extractCreateTransformStmt(statement spec.Statement, stmt *pg_query.CreateTransformStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "create_transform", "postgresql create transform statement payload is missing")
	}
	var typeNameStr string
	if stmt.GetTypeName() != nil {
		names := stmt.GetTypeName().GetNames()
		for _, n := range names {
			if s := n.GetString_(); s != nil && s.GetSval() != "" {
				typeNameStr = s.GetSval()
			}
		}
	}
	lang := stmt.GetLang()
	if typeNameStr == "" || lang == "" {
		return unsupportedStatement(statement, "create_transform", "postgresql create transform identity fields are empty")
	}
	objectName := typeNameStr + "@" + lang
	options := map[string]string{}
	if stmt.GetReplace() {
		options["replace"] = "true"
	}
	statement.DDL = &spec.DDL{
		Operation:  spec.DDLOperationCreateTransform,
		ObjectName: objectName,
		ObjectType: "transform",
		Options:    options,
	}
	return statement
}

func extractCreateAmStmt(statement spec.Statement, stmt *pg_query.CreateAmStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "create_access_method", "postgresql create access method statement payload is missing")
	}
	amName := stmt.GetAmname()
	if amName == "" {
		return unsupportedStatement(statement, "create_access_method", "postgresql create access method name is empty")
	}
	statement.DDL = &spec.DDL{
		Operation:  spec.DDLOperationCreateAccessMethod,
		ObjectName: amName,
		ObjectType: "access_method",
	}
	return statement
}
