//go:build postgresql

package postgresql

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestExtractCreateTextSearchConfiguration(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "CREATE TEXT SEARCH CONFIGURATION english_copy ( COPY = pg_catalog.english )")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationCreateTextSearchConfiguration {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationCreateTextSearchConfiguration, s.DDL.Operation)
	}
	if s.DDL.ObjectName != "english_copy" {
		t.Errorf("expected object_name english_copy, got %q", s.DDL.ObjectName)
	}
	if s.DDL.ObjectType != "text_search_configuration" {
		t.Errorf("expected object_type text_search_configuration, got %q", s.DDL.ObjectType)
	}
	assertNoLeakedTextSearchPayload(t, s.DDL.Options)
}

func TestExtractAlterTextSearchConfigurationRename(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "ALTER TEXT SEARCH CONFIGURATION english_copy RENAME TO english_copy_v2")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationAlterTextSearchConfiguration {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationAlterTextSearchConfiguration, s.DDL.Operation)
	}
	if s.DDL.ObjectType != "text_search_configuration" {
		t.Errorf("expected object_type text_search_configuration, got %q", s.DDL.ObjectType)
	}
	if s.DDL.Options["action"] != "rename" {
		t.Errorf("expected action=rename, got %q", s.DDL.Options["action"])
	}
	if s.DDL.Options["new_name"] != "english_copy_v2" {
		t.Errorf("expected new_name=english_copy_v2, got %q", s.DDL.Options["new_name"])
	}
}

func TestExtractAlterTextSearchConfigurationOwner(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "ALTER TEXT SEARCH CONFIGURATION english_copy OWNER TO app_owner")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationAlterTextSearchConfiguration {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationAlterTextSearchConfiguration, s.DDL.Operation)
	}
	if s.DDL.Options["action"] != "set_owner" {
		t.Errorf("expected action=set_owner, got %q", s.DDL.Options["action"])
	}
	if s.DDL.Options["owner"] != "app_owner" {
		t.Errorf("expected owner=app_owner, got %q", s.DDL.Options["owner"])
	}
}

func TestExtractAlterTextSearchConfigurationSetSchema(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "ALTER TEXT SEARCH CONFIGURATION english_copy SET SCHEMA app")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationAlterTextSearchConfiguration {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationAlterTextSearchConfiguration, s.DDL.Operation)
	}
	if s.DDL.Options["action"] != "set_schema" {
		t.Errorf("expected action=set_schema, got %q", s.DDL.Options["action"])
	}
	if s.DDL.Options["new_schema"] != "app" {
		t.Errorf("expected new_schema=app, got %q", s.DDL.Options["new_schema"])
	}
}

func TestExtractDropTextSearchConfiguration(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "DROP TEXT SEARCH CONFIGURATION english_copy")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationDropTextSearchConfiguration {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationDropTextSearchConfiguration, s.DDL.Operation)
	}
	if s.DDL.ObjectName != "english_copy" {
		t.Errorf("expected object_name english_copy, got %q", s.DDL.ObjectName)
	}
}

func TestExtractCreateTextSearchDictionary(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "CREATE TEXT SEARCH DICTIONARY simple_dict (TEMPLATE = simple)")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationCreateTextSearchDictionary {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationCreateTextSearchDictionary, s.DDL.Operation)
	}
	if s.DDL.ObjectName != "simple_dict" {
		t.Errorf("expected object_name simple_dict, got %q", s.DDL.ObjectName)
	}
	if s.DDL.ObjectType != "text_search_dictionary" {
		t.Errorf("expected object_type text_search_dictionary, got %q", s.DDL.ObjectType)
	}
	assertNoLeakedTextSearchPayload(t, s.DDL.Options)
}

func TestExtractAlterTextSearchDictionaryRename(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "ALTER TEXT SEARCH DICTIONARY simple_dict RENAME TO simple_dict_v2")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationAlterTextSearchDictionary {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationAlterTextSearchDictionary, s.DDL.Operation)
	}
	if s.DDL.Options["action"] != "rename" {
		t.Errorf("expected action=rename, got %q", s.DDL.Options["action"])
	}
	if s.DDL.Options["new_name"] != "simple_dict_v2" {
		t.Errorf("expected new_name=simple_dict_v2, got %q", s.DDL.Options["new_name"])
	}
}

func TestExtractAlterTextSearchDictionaryOwner(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "ALTER TEXT SEARCH DICTIONARY simple_dict OWNER TO app_owner")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationAlterTextSearchDictionary {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationAlterTextSearchDictionary, s.DDL.Operation)
	}
	if s.DDL.Options["action"] != "set_owner" {
		t.Errorf("expected action=set_owner, got %q", s.DDL.Options["action"])
	}
	if s.DDL.Options["owner"] != "app_owner" {
		t.Errorf("expected owner=app_owner, got %q", s.DDL.Options["owner"])
	}
}

func TestExtractAlterTextSearchDictionarySetSchema(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "ALTER TEXT SEARCH DICTIONARY simple_dict SET SCHEMA app")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationAlterTextSearchDictionary {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationAlterTextSearchDictionary, s.DDL.Operation)
	}
	if s.DDL.Options["action"] != "set_schema" {
		t.Errorf("expected action=set_schema, got %q", s.DDL.Options["action"])
	}
	if s.DDL.Options["new_schema"] != "app" {
		t.Errorf("expected new_schema=app, got %q", s.DDL.Options["new_schema"])
	}
}

func TestExtractDropTextSearchDictionary(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "DROP TEXT SEARCH DICTIONARY simple_dict")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationDropTextSearchDictionary {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationDropTextSearchDictionary, s.DDL.Operation)
	}
	if s.DDL.ObjectName != "simple_dict" {
		t.Errorf("expected object_name simple_dict, got %q", s.DDL.ObjectName)
	}
}

func TestExtractCreateTextSearchParser(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "CREATE TEXT SEARCH PARSER parser_name (START = start_func, GETTOKEN = token_func, END = end_func, LEXTYPES = lextype_func)")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationCreateTextSearchParser {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationCreateTextSearchParser, s.DDL.Operation)
	}
	if s.DDL.ObjectName != "parser_name" {
		t.Errorf("expected object_name parser_name, got %q", s.DDL.ObjectName)
	}
	if s.DDL.ObjectType != "text_search_parser" {
		t.Errorf("expected object_type text_search_parser, got %q", s.DDL.ObjectType)
	}
	assertNoLeakedTextSearchPayload(t, s.DDL.Options)
}

func TestExtractAlterTextSearchParserRename(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "ALTER TEXT SEARCH PARSER parser_name RENAME TO parser_name_v2")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationAlterTextSearchParser {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationAlterTextSearchParser, s.DDL.Operation)
	}
	if s.DDL.Options["action"] != "rename" {
		t.Errorf("expected action=rename, got %q", s.DDL.Options["action"])
	}
	if s.DDL.Options["new_name"] != "parser_name_v2" {
		t.Errorf("expected new_name=parser_name_v2, got %q", s.DDL.Options["new_name"])
	}
}

func TestExtractAlterTextSearchParserSetSchema(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "ALTER TEXT SEARCH PARSER parser_name SET SCHEMA app")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationAlterTextSearchParser {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationAlterTextSearchParser, s.DDL.Operation)
	}
	if s.DDL.Options["action"] != "set_schema" {
		t.Errorf("expected action=set_schema, got %q", s.DDL.Options["action"])
	}
	if s.DDL.Options["new_schema"] != "app" {
		t.Errorf("expected new_schema=app, got %q", s.DDL.Options["new_schema"])
	}
}

func TestExtractDropTextSearchParser(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "DROP TEXT SEARCH PARSER parser_name")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationDropTextSearchParser {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationDropTextSearchParser, s.DDL.Operation)
	}
	if s.DDL.ObjectName != "parser_name" {
		t.Errorf("expected object_name parser_name, got %q", s.DDL.ObjectName)
	}
}

func TestExtractCreateTextSearchTemplate(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "CREATE TEXT SEARCH TEMPLATE template_name (LEXIZE = lexize_func)")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationCreateTextSearchTemplate {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationCreateTextSearchTemplate, s.DDL.Operation)
	}
	if s.DDL.ObjectName != "template_name" {
		t.Errorf("expected object_name template_name, got %q", s.DDL.ObjectName)
	}
	if s.DDL.ObjectType != "text_search_template" {
		t.Errorf("expected object_type text_search_template, got %q", s.DDL.ObjectType)
	}
	assertNoLeakedTextSearchPayload(t, s.DDL.Options)
}

func TestExtractAlterTextSearchTemplateRename(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "ALTER TEXT SEARCH TEMPLATE template_name RENAME TO template_name_v2")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationAlterTextSearchTemplate {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationAlterTextSearchTemplate, s.DDL.Operation)
	}
	if s.DDL.Options["action"] != "rename" {
		t.Errorf("expected action=rename, got %q", s.DDL.Options["action"])
	}
	if s.DDL.Options["new_name"] != "template_name_v2" {
		t.Errorf("expected new_name=template_name_v2, got %q", s.DDL.Options["new_name"])
	}
}

func TestExtractAlterTextSearchTemplateSetSchema(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "ALTER TEXT SEARCH TEMPLATE template_name SET SCHEMA app")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationAlterTextSearchTemplate {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationAlterTextSearchTemplate, s.DDL.Operation)
	}
	if s.DDL.Options["action"] != "set_schema" {
		t.Errorf("expected action=set_schema, got %q", s.DDL.Options["action"])
	}
	if s.DDL.Options["new_schema"] != "app" {
		t.Errorf("expected new_schema=app, got %q", s.DDL.Options["new_schema"])
	}
}

func TestExtractDropTextSearchTemplate(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "DROP TEXT SEARCH TEMPLATE template_name")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationDropTextSearchTemplate {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationDropTextSearchTemplate, s.DDL.Operation)
	}
	if s.DDL.ObjectName != "template_name" {
		t.Errorf("expected object_name template_name, got %q", s.DDL.ObjectName)
	}
}

func TestExtractDropTextSearchConfigurationIfExists(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "DROP TEXT SEARCH CONFIGURATION IF EXISTS english_copy CASCADE")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationDropTextSearchConfiguration {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationDropTextSearchConfiguration, s.DDL.Operation)
	}
	if s.DDL.Options["if_exists"] != "true" {
		t.Errorf("expected if_exists=true, got %q", s.DDL.Options["if_exists"])
	}
	if s.DDL.Options["cascade"] != "true" {
		t.Errorf("expected cascade=true, got %q", s.DDL.Options["cascade"])
	}
}

func assertNoLeakedTextSearchPayload(t *testing.T, options map[string]string) {
	t.Helper()
	forbidden := []string{
		"procedure", "function", "support", "strategy", "definition",
		"body", "query", "options", "password", "secret", "token",
		"copy", "template", "start", "gettoken", "end", "lextypes",
		"lexize", "has_options",
	}
	for _, key := range forbidden {
		if _, ok := options[key]; ok {
			t.Errorf("forbidden key %q found in options", key)
		}
	}
}
