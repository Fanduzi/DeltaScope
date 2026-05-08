//go:build postgresql

package postgresql

import (
	"context"
	"fmt"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// extensionLifecycleASTFact captures observed pg_query AST facts for a
// PostgreSQL extension lifecycle DDL candidate.
type extensionLifecycleASTFact struct {
	Name             string
	SQL              string
	TopKind          string
	ObjectName       string
	NameNodeShape    string // how the extension name appears: string_field, List→String, etc.
	IfNotExists      bool
	IfExists         bool
	Cascade          bool
	DropRemoveType   string // for DropStmt: OBJECT_EXTENSION, etc.
	TargetSchema     string
	TargetVersion    string
	HasVersion       bool
	OptionNames      []string
	AlterAction      string // update, set_schema, add_object, drop_object
	AlterSubtype     string // raw AST action code
	MemberAction     string // add or drop for member mutation
	MemberObjectType string // TABLE, FUNCTION, etc. for ADD/DROP member
	MemberObjectName string // the member object name
	ObjectType       string // object type string for AlterObjectSchemaStmt
}

// extensionLifecycleBaselineFact captures how the current DeltaScope pipeline
// handles each extension lifecycle DDL candidate.
type extensionLifecycleBaselineFact struct {
	Name               string
	Kind               spec.Kind
	Classifies         bool
	Unsupported        bool
	UnsupportedFeature string
	UnsupportedReason  string
	DDLOperation       string
	DDLObjectName      string
	DDLObjectType      string
	DDLOptions         map[string]string
	Findings           int
}

var pgExtensionLifecycleCensusCases = []struct {
	Name string
	SQL  string
}{
	{Name: "create_extension", SQL: "CREATE EXTENSION pg_trgm"},
	{Name: "create_extension_if_not_exists", SQL: "CREATE EXTENSION IF NOT EXISTS pg_trgm"},
	{Name: "create_extension_schema", SQL: "CREATE EXTENSION pg_trgm WITH SCHEMA public"},
	{Name: "create_extension_version", SQL: "CREATE EXTENSION pg_trgm WITH VERSION '1.6'"},
	{Name: "create_extension_cascade", SQL: "CREATE EXTENSION pg_trgm WITH CASCADE"},
	{Name: "drop_extension", SQL: "DROP EXTENSION pg_trgm"},
	{Name: "drop_extension_if_exists_cascade", SQL: "DROP EXTENSION IF EXISTS pg_trgm CASCADE"},
	{Name: "alter_extension_update", SQL: "ALTER EXTENSION pg_trgm UPDATE"},
	{Name: "alter_extension_update_to_version", SQL: "ALTER EXTENSION pg_trgm UPDATE TO '1.6'"},
	{Name: "alter_extension_set_schema", SQL: "ALTER EXTENSION pg_trgm SET SCHEMA extensions"},
	{Name: "alter_extension_add_table", SQL: "ALTER EXTENSION pg_trgm ADD TABLE users"},
	{Name: "alter_extension_drop_table", SQL: "ALTER EXTENSION pg_trgm DROP TABLE users"},
}

// TestExtensionLifecycleASTCensus inspects raw pg_query_go AST facts for all
// PostgreSQL extension lifecycle DDL candidates. This is a read-only
// characterization test — no production code is modified.
func TestExtensionLifecycleASTCensus(t *testing.T) {
	t.Log("")
	t.Log("=== PostgreSQL Extension Lifecycle AST Census ===")
	t.Logf("%-40s | %-30s | %s", "Case", "Node Kind", "AST Facts")
	t.Log(string(make([]byte, 160)))

	for _, tc := range pgExtensionLifecycleCensusCases {
		tree, err := pg_query.Parse(tc.SQL)
		if err != nil {
			t.Fatalf("%s: parse failed: %v", tc.Name, err)
		}
		if len(tree.Stmts) != 1 {
			t.Fatalf("%s: expected 1 statement, got %d", tc.Name, len(tree.Stmts))
		}

		node := tree.Stmts[0].Stmt
		fact := inspectExtensionLifecycleAST(t, tc.Name, tc.SQL, node)
		assertExtensionLifecycleASTFacts(t, fact)
	}
}

func inspectExtensionLifecycleAST(t *testing.T, name, sql string, node *pg_query.Node) extensionLifecycleASTFact {
	t.Helper()
	fact := extensionLifecycleASTFact{Name: name, SQL: sql}

	switch n := node.GetNode().(type) {
	case *pg_query.Node_CreateExtensionStmt:
		stmt := n.CreateExtensionStmt
		fact.TopKind = "CreateExtensionStmt"

		// Extension name is a plain string field on CreateExtensionStmt.
		fact.ObjectName = stmt.GetExtname()
		fact.NameNodeShape = "string_field"
		fact.IfNotExists = stmt.GetIfNotExists()

		// Options are DefElem nodes: schema, new_version, cascade.
		for _, optNode := range stmt.GetOptions() {
			defElem := optNode.GetDefElem()
			if defElem == nil {
				continue
			}
			optName := defElem.GetDefname()
			fact.OptionNames = append(fact.OptionNames, optName)
			switch optName {
			case "schema":
				if arg := defElem.GetArg(); arg != nil {
					if s := arg.GetString_(); s != nil {
						fact.TargetSchema = s.GetSval()
					}
				}
			case "new_version":
				fact.HasVersion = true
				if arg := defElem.GetArg(); arg != nil {
					if s := arg.GetString_(); s != nil {
						fact.TargetVersion = s.GetSval()
					}
				}
			case "cascade":
				fact.Cascade = true
			}
		}

		t.Logf("%-40s | %-30s | ext=%q if_not_exists=%v schema=%q version=%q has_version=%v cascade=%v options=%v shape=%s",
			name, fact.TopKind, fact.ObjectName, fact.IfNotExists,
			fact.TargetSchema, fact.TargetVersion, fact.HasVersion,
			fact.Cascade, fact.OptionNames, fact.NameNodeShape)

	case *pg_query.Node_DropStmt:
		stmt := n.DropStmt
		fact.TopKind = "DropStmt"
		fact.DropRemoveType = stmt.GetRemoveType().String()
		fact.NameNodeShape = describeExtensionDropObjectsShape(stmt.GetObjects())
		fact.ObjectName = extensionNameFromDropObjects(stmt.GetObjects())
		fact.IfExists = stmt.GetMissingOk()
		fact.Cascade = stmt.GetBehavior() == pg_query.DropBehavior_DROP_CASCADE

		t.Logf("%-40s | %-30s | ext=%q remove_type=%s if_exists=%v cascade=%v obj_shape=%s",
			name, fact.TopKind, fact.ObjectName, fact.DropRemoveType,
			fact.IfExists, fact.Cascade, fact.NameNodeShape)

	case *pg_query.Node_AlterExtensionStmt:
		stmt := n.AlterExtensionStmt
		fact.TopKind = "AlterExtensionStmt"
		fact.ObjectName = stmt.GetExtname()
		fact.NameNodeShape = "string_field"

		for _, optNode := range stmt.GetOptions() {
			defElem := optNode.GetDefElem()
			if defElem == nil {
				continue
			}
			optName := defElem.GetDefname()
			fact.OptionNames = append(fact.OptionNames, optName)
			switch optName {
			case "new_version":
				fact.AlterAction = "update"
				fact.AlterSubtype = "update"
				if arg := defElem.GetArg(); arg != nil {
					if s := arg.GetString_(); s != nil {
						fact.TargetVersion = s.GetSval()
						fact.HasVersion = true
					}
				}
			}
		}
		// Plain UPDATE without version still has action=update but no option node.
		if fact.AlterAction == "" && len(fact.OptionNames) == 0 {
			fact.AlterAction = "update"
		}

		t.Logf("%-40s | %-30s | ext=%q action=%s version=%q has_version=%v options=%v shape=%s",
			name, fact.TopKind, fact.ObjectName, fact.AlterAction,
			fact.TargetVersion, fact.HasVersion, fact.OptionNames, fact.NameNodeShape)

	case *pg_query.Node_AlterObjectSchemaStmt:
		// ALTER EXTENSION ... SET SCHEMA is represented as AlterObjectSchemaStmt
		// with object_type=OBJECT_EXTENSION, not AlterExtensionStmt.
		stmt := n.AlterObjectSchemaStmt
		fact.TopKind = "AlterObjectSchemaStmt"
		fact.AlterAction = "set_schema"
		fact.TargetSchema = stmt.GetNewschema()
		fact.ObjectType = stmt.GetObjectType().String()

		// Extension name: for OBJECT_EXTENSION, the name is in the Object field.
		if obj := stmt.GetObject(); obj != nil {
			if list := obj.GetList(); list != nil {
				fact.ObjectName = firstStringFromNodes(list.GetItems())
				fact.NameNodeShape = "object→List→String"
			} else if s := obj.GetString_(); s != nil {
				fact.ObjectName = s.GetSval()
				fact.NameNodeShape = "object→String"
			}
		}

		t.Logf("%-40s | %-30s | ext=%q obj_type=%s new_schema=%q shape=%s",
			name, fact.TopKind, fact.ObjectName, fact.ObjectType,
			fact.TargetSchema, fact.NameNodeShape)

	case *pg_query.Node_AlterExtensionContentsStmt:
		stmt := n.AlterExtensionContentsStmt
		fact.TopKind = "AlterExtensionContentsStmt"
		fact.ObjectName = stmt.GetExtname()
		fact.NameNodeShape = "string_field"

		action := stmt.GetAction()
		switch action {
		case 1:
			fact.MemberAction = "add"
		case -1:
			fact.MemberAction = "drop"
		default:
			fact.MemberAction = fmt.Sprintf("unknown(%d)", action)
		}
		fact.AlterSubtype = fmt.Sprintf("action=%d", action)
		fact.MemberObjectType = stmt.GetObjtype().String()

		if obj := stmt.GetObject(); obj != nil {
			if list := obj.GetList(); list != nil {
				fact.MemberObjectName = firstStringFromNodes(list.GetItems())
			} else {
				fact.MemberObjectName = firstStringFromNodes([]*pg_query.Node{obj})
			}
		}

		t.Logf("%-40s | %-30s | ext=%q member_action=%s obj_type=%s member=%q action_raw=%d shape=%s",
			name, fact.TopKind, fact.ObjectName, fact.MemberAction,
			fact.MemberObjectType, fact.MemberObjectName,
			action, fact.NameNodeShape)

	default:
		t.Fatalf("%s: unexpected node type %T", name, node.GetNode())
	}

	return fact
}

// describeExtensionDropObjectsShape identifies the node structure of DropStmt
// objects for extension drops.
func describeExtensionDropObjectsShape(objects []*pg_query.Node) string {
	if len(objects) == 0 {
		return "empty"
	}
	obj := objects[0]
	if list := obj.GetList(); list != nil {
		return "List→String"
	}
	if s := obj.GetString_(); s != nil {
		return "String"
	}
	return fmt.Sprintf("%T", obj.GetNode())
}

// extensionNameFromDropObjects extracts the extension name from DropStmt
// objects for DROP EXTENSION.
func extensionNameFromDropObjects(objects []*pg_query.Node) string {
	if len(objects) == 0 {
		return ""
	}
	for _, obj := range objects {
		if list := obj.GetList(); list != nil {
			return firstStringFromNodes(list.GetItems())
		}
		if s := obj.GetString_(); s != nil {
			return s.GetSval()
		}
	}
	return ""
}

// assertExtensionLifecycleASTFacts validates that all expected AST facts are
// present and stable for decision-making.
func assertExtensionLifecycleASTFacts(t *testing.T, fact extensionLifecycleASTFact) {
	t.Helper()

	if fact.TopKind == "" {
		t.Errorf("%s: expected non-empty top kind", fact.Name)
	}
	if fact.ObjectName == "" {
		t.Errorf("%s: expected non-empty extension name", fact.Name)
	}

	switch fact.TopKind {
	case "CreateExtensionStmt":
		if fact.NameNodeShape == "" {
			t.Errorf("%s: expected non-empty name shape", fact.Name)
		}
	case "DropStmt":
		if fact.NameNodeShape == "" {
			t.Errorf("%s: expected non-empty name shape", fact.Name)
		}
	case "AlterExtensionStmt":
		if fact.NameNodeShape == "" {
			t.Errorf("%s: expected non-empty name shape", fact.Name)
		}
	case "AlterObjectSchemaStmt":
		if fact.NameNodeShape == "" {
			t.Errorf("%s: expected non-empty name shape", fact.Name)
		}
	case "AlterExtensionContentsStmt":
		if fact.MemberAction == "" {
			t.Errorf("%s: expected non-empty member action", fact.Name)
		}
	}
}

// TestExtensionLifecycleDeltaScopeBaseline characterizes how the current
// DeltaScope pipeline classifies and extracts extension lifecycle DDL
// candidates. This is a read-only characterization test — no production
// code is modified.
func TestExtensionLifecycleDeltaScopeBaseline(t *testing.T) {
	t.Log("")
	t.Log("=== PostgreSQL Extension Lifecycle DeltaScope Baseline ===")
	t.Logf("%-40s | %-10s | %-5s | %-12s | %s",
		"Case", "Kind", "DDL?", "Unsupported?", "Detail")
	t.Log(string(make([]byte, 160)))

	for _, tc := range pgExtensionLifecycleCensusCases {
		p := New()
		result, parseErr := p.Parse(context.Background(), tc.SQL)
		if parseErr != nil {
			t.Logf("%-40s | %-10s | %-5s | %-12s | parse error: %v",
				tc.Name, "ERROR", "-", "-", parseErr)
			t.Errorf("%s: unexpected parse error: %v", tc.Name, parseErr)
			continue
		}
		if len(result.Statements) != 1 {
			t.Fatalf("%s: expected 1 statement, got %d", tc.Name, len(result.Statements))
		}

		es := result.Statements[0]
		classifies := es.Kind == spec.KindDDL

		baseline := extensionLifecycleBaselineFact{
			Name:       tc.Name,
			Kind:       es.Kind,
			Classifies: classifies,
		}

		stmt, extractErr := es.Extractor.Extract(spec.DialectPostgreSQL, es.RawSQL)
		if extractErr != nil {
			t.Logf("%-40s | %-10s | %-5v | %-12s | extract error: %v",
				tc.Name, baseline.Kind, classifies, "-", extractErr)
			t.Errorf("%s: unexpected extract error: %v", tc.Name, extractErr)
			continue
		}

		baseline.Unsupported = stmt.Unsupported != nil
		detail := ""
		if stmt.Unsupported != nil {
			baseline.UnsupportedFeature = stmt.Unsupported.Feature
			baseline.UnsupportedReason = stmt.Unsupported.Reason
			detail = fmt.Sprintf("%s: %s", stmt.Unsupported.Feature, stmt.Unsupported.Reason)
		} else if stmt.DDL != nil {
			baseline.DDLOperation = string(stmt.DDL.Operation)
			baseline.DDLObjectName = stmt.DDL.ObjectName
			baseline.DDLObjectType = stmt.DDL.ObjectType
			if len(stmt.DDL.Options) > 0 {
				baseline.DDLOptions = stmt.DDL.Options
			}
			detail = fmt.Sprintf("op=%s obj=%q type=%q opts=%v",
				stmt.DDL.Operation, stmt.DDL.ObjectName, stmt.DDL.ObjectType, stmt.DDL.Options)
		}

		unsupported := "no"
		if baseline.Unsupported {
			unsupported = "yes"
		}
		classStr := "no"
		if baseline.Classifies {
			classStr = "yes"
		}

		t.Logf("%-40s | %-10s | %-5s | %-12s | %s",
			tc.Name, baseline.Kind, classStr, unsupported, detail)

		assertExtensionLifecycleBaseline(t, baseline)
	}
}

// assertExtensionLifecycleBaseline captures post-Task-2 expectations.
//
// Post-Task-2 state:
//   - CREATE EXTENSION forms: KindDDL, normalized as create_extension
//   - DROP EXTENSION forms: KindDDL, normalized as drop_extension
//   - ALTER EXTENSION UPDATE forms: KindDDL, normalized as alter_extension
//   - ALTER EXTENSION SET SCHEMA: KindDDL, normalized as alter_extension
//   - ALTER EXTENSION ADD/DROP TABLE: KindDDL, unsupported with stable feature names
func assertExtensionLifecycleBaseline(t *testing.T, fact extensionLifecycleBaselineFact) {
	t.Helper()

	switch fact.Name {
	case "create_extension":
		if fact.Kind != spec.KindDDL {
			t.Errorf("%s: expected KindDDL, got %v", fact.Name, fact.Kind)
		}
		if fact.Unsupported {
			t.Errorf("%s: expected normalized, got unsupported %s: %s", fact.Name, fact.UnsupportedFeature, fact.UnsupportedReason)
		}
		if fact.DDLOperation != "create_extension" {
			t.Errorf("%s: expected create_extension, got %q", fact.Name, fact.DDLOperation)
		}
		if fact.DDLObjectName != "pg_trgm" {
			t.Errorf("%s: expected object_name pg_trgm, got %q", fact.Name, fact.DDLObjectName)
		}
		if fact.DDLObjectType != "extension" {
			t.Errorf("%s: expected object_type extension, got %q", fact.Name, fact.DDLObjectType)
		}

	case "create_extension_if_not_exists":
		if fact.Kind != spec.KindDDL {
			t.Errorf("%s: expected KindDDL, got %v", fact.Name, fact.Kind)
		}
		if fact.Unsupported {
			t.Errorf("%s: expected normalized, got unsupported", fact.Name)
		}
		if fact.DDLOperation != "create_extension" {
			t.Errorf("%s: expected create_extension, got %q", fact.Name, fact.DDLOperation)
		}
		if fact.DDLOptions["if_not_exists"] != "true" {
			t.Errorf("%s: expected if_not_exists=true, got %v", fact.Name, fact.DDLOptions)
		}

	case "create_extension_schema":
		if fact.DDLOperation != "create_extension" {
			t.Errorf("%s: expected create_extension, got %q", fact.Name, fact.DDLOperation)
		}
		if fact.DDLOptions["schema"] != "public" {
			t.Errorf("%s: expected schema=public, got %v", fact.Name, fact.DDLOptions)
		}

	case "create_extension_version":
		if fact.DDLOperation != "create_extension" {
			t.Errorf("%s: expected create_extension, got %q", fact.Name, fact.DDLOperation)
		}
		if fact.DDLOptions["version"] != "1.6" {
			t.Errorf("%s: expected version=1.6, got %v", fact.Name, fact.DDLOptions)
		}

	case "create_extension_cascade":
		if fact.DDLOperation != "create_extension" {
			t.Errorf("%s: expected create_extension, got %q", fact.Name, fact.DDLOperation)
		}
		if fact.DDLOptions["cascade"] != "true" {
			t.Errorf("%s: expected cascade=true, got %v", fact.Name, fact.DDLOptions)
		}

	case "drop_extension":
		if fact.Kind != spec.KindDDL {
			t.Errorf("%s: expected KindDDL, got %v", fact.Name, fact.Kind)
		}
		if fact.Unsupported {
			t.Errorf("%s: expected normalized, got unsupported", fact.Name)
		}
		if fact.DDLOperation != "drop_extension" {
			t.Errorf("%s: expected drop_extension, got %q", fact.Name, fact.DDLOperation)
		}
		if fact.DDLObjectName != "pg_trgm" {
			t.Errorf("%s: expected object_name pg_trgm, got %q", fact.Name, fact.DDLObjectName)
		}

	case "drop_extension_if_exists_cascade":
		if fact.DDLOperation != "drop_extension" {
			t.Errorf("%s: expected drop_extension, got %q", fact.Name, fact.DDLOperation)
		}
		if fact.DDLOptions["if_exists"] != "true" {
			t.Errorf("%s: expected if_exists=true, got %v", fact.Name, fact.DDLOptions)
		}
		if fact.DDLOptions["cascade"] != "true" {
			t.Errorf("%s: expected cascade=true, got %v", fact.Name, fact.DDLOptions)
		}

	case "alter_extension_update":
		if fact.Kind != spec.KindDDL {
			t.Errorf("%s: expected KindDDL, got %v", fact.Name, fact.Kind)
		}
		if fact.Unsupported {
			t.Errorf("%s: expected normalized, got unsupported", fact.Name)
		}
		if fact.DDLOperation != "alter_extension" {
			t.Errorf("%s: expected alter_extension, got %q", fact.Name, fact.DDLOperation)
		}
		if fact.DDLOptions["action"] != "update" {
			t.Errorf("%s: expected action=update, got %v", fact.Name, fact.DDLOptions)
		}

	case "alter_extension_update_to_version":
		if fact.DDLOperation != "alter_extension" {
			t.Errorf("%s: expected alter_extension, got %q", fact.Name, fact.DDLOperation)
		}
		if fact.DDLOptions["action"] != "update" {
			t.Errorf("%s: expected action=update, got %v", fact.Name, fact.DDLOptions)
		}
		if fact.DDLOptions["version"] != "1.6" {
			t.Errorf("%s: expected version=1.6, got %v", fact.Name, fact.DDLOptions)
		}

	case "alter_extension_set_schema":
		if fact.Kind != spec.KindDDL {
			t.Errorf("%s: expected KindDDL, got %v", fact.Name, fact.Kind)
		}
		if fact.Unsupported {
			t.Errorf("%s: expected normalized, got unsupported %s: %s", fact.Name, fact.UnsupportedFeature, fact.UnsupportedReason)
		}
		if fact.DDLOperation != "alter_extension" {
			t.Errorf("%s: expected alter_extension, got %q", fact.Name, fact.DDLOperation)
		}
		if fact.DDLOptions["action"] != "set_schema" {
			t.Errorf("%s: expected action=set_schema, got %v", fact.Name, fact.DDLOptions)
		}
		if fact.DDLOptions["new_schema"] != "extensions" {
			t.Errorf("%s: expected new_schema=extensions, got %v", fact.Name, fact.DDLOptions)
		}

	case "alter_extension_add_table":
		if fact.Kind != spec.KindDDL {
			t.Errorf("%s: expected KindDDL, got %v", fact.Name, fact.Kind)
		}
		if !fact.Unsupported {
			t.Errorf("%s: expected unsupported", fact.Name)
		}
		if fact.UnsupportedFeature != "alter_extension_add_member" {
			t.Errorf("%s: expected feature 'alter_extension_add_member', got %q", fact.Name, fact.UnsupportedFeature)
		}

	case "alter_extension_drop_table":
		if fact.Kind != spec.KindDDL {
			t.Errorf("%s: expected KindDDL, got %v", fact.Name, fact.Kind)
		}
		if !fact.Unsupported {
			t.Errorf("%s: expected unsupported", fact.Name)
		}
		if fact.UnsupportedFeature != "alter_extension_drop_member" {
			t.Errorf("%s: expected feature 'alter_extension_drop_member', got %q", fact.Name, fact.UnsupportedFeature)
		}
	}
}
