//go:build postgresql

package postgresql

import (
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

func pgTextSearchIdentity(names []*pg_query.Node) spec.Table {
	var target spec.Table
	if len(names) == 2 {
		target = spec.Table{Schema: stringNodeValue(names[0]), Name: stringNodeValue(names[1])}
	}
	return target
}

func pgTextSearchTarget(object *pg_query.Node, kind, action, destination string, owner spec.PGTableOwnerTarget) *spec.PGTextSearchChange {
	return &spec.PGTextSearchChange{
		Target: spec.PGDestructiveTableTarget{Table: pgTextSearchIdentity(object.GetList().GetItems())},
		Kind:   kind, Action: action, Destination: destination, Owner: owner,
	}
}

func extractAlterTextSearchConfiguration(s spec.Statement, stmt *pg_query.AlterTSConfigurationStmt) spec.Statement {
	d := &spec.PGTextSearchChange{
		Kind: "configuration", Target: spec.PGDestructiveTableTarget{Table: pgTextSearchIdentity(stmt.GetCfgname())},
		Tokens: stringValuesFromNodes(stmt.GetTokentype()), IfExists: stmt.GetMissingOk(),
	}
	switch stmt.GetKind() {
	case pg_query.AlterTSConfigType_ALTER_TSCONFIG_ADD_MAPPING:
		d.Action = "add_mapping"
	case pg_query.AlterTSConfigType_ALTER_TSCONFIG_ALTER_MAPPING_FOR_TOKEN:
		d.Action = "alter_mapping"
	case pg_query.AlterTSConfigType_ALTER_TSCONFIG_REPLACE_DICT, pg_query.AlterTSConfigType_ALTER_TSCONFIG_REPLACE_DICT_FOR_TOKEN:
		d.Action = "replace_mapping"
	case pg_query.AlterTSConfigType_ALTER_TSCONFIG_DROP_MAPPING:
		d.Action = "drop_mapping"
	}
	for _, item := range stmt.GetDicts() {
		d.Dictionaries = append(d.Dictionaries, pgTextSearchIdentity(item.GetList().GetItems()))
	}
	s.DDL = &spec.DDL{Operation: spec.DDLOperationAlterTextSearchConfiguration, ObjectType: "text_search_configuration",
		ObjectName: lastStringFromNodes(stmt.GetCfgname()), PGTextSearchChange: d}
	return s
}

func extractAlterTextSearchDictionary(s spec.Statement, stmt *pg_query.AlterTSDictionaryStmt) spec.Statement {
	d := &spec.PGTextSearchChange{Kind: "dictionary", Action: "set_dictionary_options",
		Target: spec.PGDestructiveTableTarget{Table: pgTextSearchIdentity(stmt.GetDictname())}}
	for _, item := range stmt.GetOptions() {
		d.Options = append(d.Options, pgTextSearchDictionaryOption(item.GetDefElem()))
	}
	s.DDL = &spec.DDL{Operation: spec.DDLOperationAlterTextSearchDictionary, ObjectType: "text_search_dictionary",
		ObjectName: lastStringFromNodes(stmt.GetDictname()), PGTextSearchChange: d}
	return s
}

func pgTextSearchDictionaryOption(node *pg_query.DefElem) spec.PGTextSearchDictionaryOption {
	arg := node.GetArg()
	o := spec.PGTextSearchDictionaryOption{Name: node.GetDefname(), Present: arg != nil}
	if o.Name != "accept" || !o.Present {
		return o
	}
	var text string
	switch {
	case arg.GetInteger() != nil:
		value := arg.GetInteger().GetIval()
		o.BooleanKnown, o.BooleanValid = true, value == 0 || value == 1
		return o
	case arg.GetString_() != nil:
		text = arg.GetString_().GetSval()
	case arg.GetTypeName() != nil && len(arg.GetTypeName().GetNames()) == 1:
		text = stringNodeValue(arg.GetTypeName().GetNames()[0])
	default:
		return o
	}
	o.BooleanKnown = true
	switch strings.ToLower(text) {
	case "true", "false", "on", "off":
		o.BooleanValid = true
	}
	return o
}
