//go:build postgresql

package postgresql

import (
	"fmt"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

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
		viewTable := tableFromObjectName(stmt.GetObjects())
		options := map[string]string{"if_exists": fmt.Sprintf("%t", stmt.GetMissingOk())}
		if stmt.GetBehavior() == pg_query.DropBehavior_DROP_CASCADE {
			options["cascade"] = "true"
		}
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationDropView,
			ObjectName: objectNameFromObjectName(stmt.GetObjects()),
			ObjectType: "view",
			Table:      viewTable,
			Options:    options,
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
	case pg_query.ObjectType_OBJECT_DOMAIN:
		options := map[string]string{"if_exists": fmt.Sprintf("%t", stmt.GetMissingOk())}
		if stmt.GetBehavior() == pg_query.DropBehavior_DROP_CASCADE {
			options["cascade"] = "true"
		}
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationDropDomain,
			ObjectName: dropTypeNameFromObjects(stmt.GetObjects()),
			ObjectType: "domain",
			Options:    options,
		}
	case pg_query.ObjectType_OBJECT_EXTENSION:
		options := map[string]string{"if_exists": fmt.Sprintf("%t", stmt.GetMissingOk())}
		if stmt.GetBehavior() == pg_query.DropBehavior_DROP_CASCADE {
			options["cascade"] = "true"
		}
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationDropExtension,
			ObjectName: dropTargetName(stmt),
			ObjectType: "extension",
			Options:    options,
		}
	case pg_query.ObjectType_OBJECT_POLICY:
		options := map[string]string{"if_exists": fmt.Sprintf("%t", stmt.GetMissingOk())}
		if stmt.GetBehavior() == pg_query.DropBehavior_DROP_CASCADE {
			options["cascade"] = "true"
		}
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationDropPolicy,
			ObjectName: dropPolicyName(stmt),
			ObjectType: "policy",
			Options:    options,
		}
	case pg_query.ObjectType_OBJECT_TRIGGER:
		statement.DDL = extractDropTriggerStmt(statement, stmt).DDL
	case pg_query.ObjectType_OBJECT_FUNCTION:
		options := map[string]string{"if_exists": fmt.Sprintf("%t", stmt.GetMissingOk())}
		if stmt.GetBehavior() == pg_query.DropBehavior_DROP_CASCADE {
			options["cascade"] = "true"
		}
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationDropFunction,
			ObjectName: dropObjectWithArgsName(stmt),
			ObjectType: "function",
			Options:    options,
		}
	case pg_query.ObjectType_OBJECT_PROCEDURE:
		options := map[string]string{"if_exists": fmt.Sprintf("%t", stmt.GetMissingOk())}
		if stmt.GetBehavior() == pg_query.DropBehavior_DROP_CASCADE {
			options["cascade"] = "true"
		}
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationDropProcedure,
			ObjectName: dropObjectWithArgsName(stmt),
			ObjectType: "procedure",
			Options:    options,
		}
	case pg_query.ObjectType_OBJECT_PUBLICATION:
		options := map[string]string{"if_exists": fmt.Sprintf("%t", stmt.GetMissingOk())}
		if stmt.GetBehavior() == pg_query.DropBehavior_DROP_CASCADE {
			options["cascade"] = "true"
		}
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationDropPublication,
			ObjectName: dropTargetName(stmt),
			ObjectType: "publication",
			Options:    options,
		}
	case pg_query.ObjectType_OBJECT_FOREIGN_TABLE:
		options := map[string]string{"if_exists": fmt.Sprintf("%t", stmt.GetMissingOk())}
		if stmt.GetBehavior() == pg_query.DropBehavior_DROP_CASCADE {
			options["cascade"] = "true"
		}
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationDropForeignTable,
			ObjectName: dropTargetName(stmt),
			ObjectType: "foreign_table",
			Options:    options,
		}
	case pg_query.ObjectType_OBJECT_FOREIGN_SERVER:
		options := map[string]string{"if_exists": fmt.Sprintf("%t", stmt.GetMissingOk())}
		if stmt.GetBehavior() == pg_query.DropBehavior_DROP_CASCADE {
			options["cascade"] = "true"
		}
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationDropForeignServer,
			ObjectName: dropTargetName(stmt),
			ObjectType: "foreign_server",
			Options:    options,
		}
	case pg_query.ObjectType_OBJECT_FDW:
		options := map[string]string{"if_exists": fmt.Sprintf("%t", stmt.GetMissingOk())}
		if stmt.GetBehavior() == pg_query.DropBehavior_DROP_CASCADE {
			options["cascade"] = "true"
		}
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationDropForeignDataWrapper,
			ObjectName: dropTargetName(stmt),
			ObjectType: "foreign_data_wrapper",
			Options:    options,
		}
	case pg_query.ObjectType_OBJECT_EVENT_TRIGGER:
		statement.DDL = extractDropEventTrigger(statement, stmt).DDL
	case pg_query.ObjectType_OBJECT_RULE:
		statement.DDL = extractDropRule(statement, stmt).DDL
	case pg_query.ObjectType_OBJECT_COLLATION:
		options := map[string]string{"if_exists": fmt.Sprintf("%t", stmt.GetMissingOk())}
		if stmt.GetBehavior() == pg_query.DropBehavior_DROP_CASCADE {
			options["cascade"] = "true"
		}
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationDropCollation,
			ObjectName: dropTargetName(stmt),
			ObjectType: "collation",
			Options:    options,
		}
	case pg_query.ObjectType_OBJECT_STATISTIC_EXT:
		options := map[string]string{"if_exists": fmt.Sprintf("%t", stmt.GetMissingOk())}
		if stmt.GetBehavior() == pg_query.DropBehavior_DROP_CASCADE {
			options["cascade"] = "true"
		}
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationDropStatistics,
			ObjectName: dropTargetName(stmt),
			ObjectType: "statistics",
			Options:    options,
		}
	case pg_query.ObjectType_OBJECT_AGGREGATE:
		options := map[string]string{"if_exists": fmt.Sprintf("%t", stmt.GetMissingOk())}
		if stmt.GetBehavior() == pg_query.DropBehavior_DROP_CASCADE {
			options["cascade"] = "true"
		}
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationDropAggregate,
			ObjectName: dropObjectWithArgsName(stmt),
			ObjectType: "aggregate",
			Options:    options,
		}
	case pg_query.ObjectType_OBJECT_OPERATOR:
		options := map[string]string{"if_exists": fmt.Sprintf("%t", stmt.GetMissingOk())}
		if stmt.GetBehavior() == pg_query.DropBehavior_DROP_CASCADE {
			options["cascade"] = "true"
		}
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationDropOperator,
			ObjectName: dropObjectWithArgsName(stmt),
			ObjectType: "operator",
			Options:    options,
		}
	case pg_query.ObjectType_OBJECT_CONVERSION:
		options := map[string]string{"if_exists": fmt.Sprintf("%t", stmt.GetMissingOk())}
		if stmt.GetBehavior() == pg_query.DropBehavior_DROP_CASCADE {
			options["cascade"] = "true"
		}
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationDropConversion,
			ObjectName: dropTargetName(stmt),
			ObjectType: "conversion",
			Options:    options,
		}
	case pg_query.ObjectType_OBJECT_OPFAMILY:
		options := map[string]string{"if_exists": fmt.Sprintf("%t", stmt.GetMissingOk())}
		if stmt.GetBehavior() == pg_query.DropBehavior_DROP_CASCADE {
			options["cascade"] = "true"
		}
		objectName, am := opNameAndAccessMethodFromDropObjects(stmt)
		if am != "" {
			options["access_method"] = am
		}
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationDropOperatorFamily,
			ObjectName: objectName,
			ObjectType: "operator_family",
			Options:    options,
		}
	case pg_query.ObjectType_OBJECT_OPCLASS:
		options := map[string]string{"if_exists": fmt.Sprintf("%t", stmt.GetMissingOk())}
		if stmt.GetBehavior() == pg_query.DropBehavior_DROP_CASCADE {
			options["cascade"] = "true"
		}
		objectName, am := opNameAndAccessMethodFromDropObjects(stmt)
		if am != "" {
			options["access_method"] = am
		}
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationDropOperatorClass,
			ObjectName: objectName,
			ObjectType: "operator_class",
			Options:    options,
		}
		return statement
	case pg_query.ObjectType_OBJECT_TSCONFIGURATION:
		options := map[string]string{"if_exists": fmt.Sprintf("%t", stmt.GetMissingOk())}
		if stmt.GetBehavior() == pg_query.DropBehavior_DROP_CASCADE {
			options["cascade"] = "true"
		}
		objectName := dropObjectFirstName(stmt)
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationDropTextSearchConfiguration,
			ObjectName: objectName,
			ObjectType: "text_search_configuration",
			Options:    options,
		}
		return statement
	case pg_query.ObjectType_OBJECT_TSDICTIONARY:
		options := map[string]string{"if_exists": fmt.Sprintf("%t", stmt.GetMissingOk())}
		if stmt.GetBehavior() == pg_query.DropBehavior_DROP_CASCADE {
			options["cascade"] = "true"
		}
		objectName := dropObjectFirstName(stmt)
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationDropTextSearchDictionary,
			ObjectName: objectName,
			ObjectType: "text_search_dictionary",
			Options:    options,
		}
		return statement
	case pg_query.ObjectType_OBJECT_TSPARSER:
		options := map[string]string{"if_exists": fmt.Sprintf("%t", stmt.GetMissingOk())}
		if stmt.GetBehavior() == pg_query.DropBehavior_DROP_CASCADE {
			options["cascade"] = "true"
		}
		objectName := dropObjectFirstName(stmt)
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationDropTextSearchParser,
			ObjectName: objectName,
			ObjectType: "text_search_parser",
			Options:    options,
		}
		return statement
	case pg_query.ObjectType_OBJECT_TSTEMPLATE:
		options := map[string]string{"if_exists": fmt.Sprintf("%t", stmt.GetMissingOk())}
		if stmt.GetBehavior() == pg_query.DropBehavior_DROP_CASCADE {
			options["cascade"] = "true"
		}
		objectName := dropObjectFirstName(stmt)
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationDropTextSearchTemplate,
			ObjectName: objectName,
			ObjectType: "text_search_template",
			Options:    options,
		}
		return statement
	case pg_query.ObjectType_OBJECT_TRANSFORM:
		options := map[string]string{"if_exists": fmt.Sprintf("%t", stmt.GetMissingOk())}
		if stmt.GetBehavior() == pg_query.DropBehavior_DROP_CASCADE {
			options["cascade"] = "true"
		}
		objectName := dropTransformIdentity(stmt)
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationDropTransform,
			ObjectName: objectName,
			ObjectType: "transform",
			Options:    options,
		}
		return statement
	case pg_query.ObjectType_OBJECT_ACCESS_METHOD:
		options := map[string]string{"if_exists": fmt.Sprintf("%t", stmt.GetMissingOk())}
		if stmt.GetBehavior() == pg_query.DropBehavior_DROP_CASCADE {
			options["cascade"] = "true"
		}
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationDropAccessMethod,
			ObjectName: dropTargetName(stmt),
			ObjectType: "access_method",
			Options:    options,
		}
		return statement
	default:
		return unsupportedStatement(statement, "drop", "postgresql drop target is not in the approved v1 subset")
	}
	return statement
}

func extractTruncateStmt(statement spec.Statement, stmt *pg_query.TruncateStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "truncate", "postgresql truncate statement payload is missing")
	}
	statement.DDL = &spec.DDL{Operation: spec.DDLOperationTruncateTable, Table: tableFromRelationNodeList(stmt.GetRelations())}
	return statement
}

func dropObjectFirstName(stmt *pg_query.DropStmt) string {
	for _, obj := range stmt.GetObjects() {
		if list := obj.GetList(); list != nil {
			return firstStringFromNodes(list.GetItems())
		}
		if s := obj.GetString_(); s != nil && s.GetSval() != "" {
			return s.GetSval()
		}
	}
	return ""
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

func dropObjectWithArgsName(stmt *pg_query.DropStmt) string {
	for _, obj := range stmt.GetObjects() {
		owa := obj.GetObjectWithArgs()
		if owa == nil {
			continue
		}
		for _, name := range owa.GetObjname() {
			if s := name.GetString_(); s != nil && s.GetSval() != "" {
				return s.GetSval()
			}
		}
	}
	return ""
}

// opNameAndAccessMethodFromDropObjects extracts (name, access_method) from DropStmt objects
// where each element is a list [access_method, object_name].
func opNameAndAccessMethodFromDropObjects(stmt *pg_query.DropStmt) (string, string) {
	for _, obj := range stmt.GetObjects() {
		list := obj.GetList()
		if list == nil {
			continue
		}
		items := list.GetItems()
		name := lastStringFromNodes(items)
		am := ""
		if len(items) > 0 {
			am = firstStringFromNodes([]*pg_query.Node{items[0]})
		}
		return name, am
	}
	return "", ""
}
func dropTypeNameFromObjects(objects []*pg_query.Node) string {
	for _, obj := range objects {
		tn := obj.GetTypeName()
		if tn == nil {
			continue
		}
		names := tn.GetNames()
		for i := len(names) - 1; i >= 0; i-- {
			if s := names[i].GetString_(); s != nil && s.GetSval() != "" {
				return s.GetSval()
			}
		}
	}
	return ""
}

// dropTransformIdentity extracts a bounded identity from DROP TRANSFORM objects.
// Objects = [List([TypeName(type_name), String(language)])] -> identity = "type@language".
func dropTransformIdentity(stmt *pg_query.DropStmt) string {
	var typeName, lang string
	for _, obj := range stmt.GetObjects() {
		if list := obj.GetList(); list != nil {
			for _, item := range list.GetItems() {
				if tn := item.GetTypeName(); tn != nil {
					for _, n := range tn.GetNames() {
						if s := n.GetString_(); s != nil && s.GetSval() != "" {
							typeName = s.GetSval()
						}
					}
				}
				if s := item.GetString_(); s != nil && s.GetSval() != "" {
					lang = s.GetSval()
				}
			}
		}
	}
	if typeName != "" && lang != "" {
		return typeName + "@" + lang
	}
	if typeName != "" {
		return typeName
	}
	return ""
}
