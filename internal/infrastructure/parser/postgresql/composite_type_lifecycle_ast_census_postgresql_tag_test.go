//go:build postgresql

package postgresql

import (
	"context"
	"fmt"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// compositeTypeLifecycleASTFact captures observed pg_query AST facts for a
// composite type lifecycle DDL candidate. Fields are populated from direct
// pg_query_go inspection, not from DeltaScope's extraction pipeline.
type compositeTypeLifecycleASTFact struct {
	Name             string
	SQL              string
	TopKind          string
	ObjectName       string
	SchemaName       string
	NameNodeShape    string // how names appear: RangeVar, String, List, TypeName, etc.
	AttributeCount   int
	AttributeNames   []string
	AttributeTypes   []string
	HasCollation     bool
	CollationName    string
	DropMissingOK    bool
	DropCascade      bool
	DropRemoveType   string
	AlterTableCmds   []string // for AlterTableStmt-based ALTER TYPE
	AlterAction      string   // summary: add_attribute, drop_attribute, etc.
	AlterTargetName  string   // target attribute name
	AlterNewName     string   // new name for RENAME ATTRIBUTE / RENAME TO
	AlterNewType     string   // new type for ALTER ATTRIBUTE ... TYPE
	RenameObjectType string   // RenameStmt renameType
	SetSchemaTarget  string   // SET SCHEMA target schema
}

// compositeTypeLifecycleBaselineFact captures how the current DeltaScope
// pipeline handles each composite type lifecycle DDL candidate.
type compositeTypeLifecycleBaselineFact struct {
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
	Findings           int // count of findings from existing rules
}

var pgCompositeTypeLifecycleCensusCases = []struct {
	Name string
	SQL  string
}{
	{Name: "create_type_composite_plain", SQL: "CREATE TYPE address AS (street text, city text)"},
	{Name: "create_type_composite_multi_attr", SQL: "CREATE TYPE inventory_item AS (name text, supplier_id integer, price numeric)"},
	{Name: "create_type_composite_qualified", SQL: "CREATE TYPE qualified.address AS (street text, city text)"},
	{Name: "create_type_composite_collate", SQL: "CREATE TYPE address AS (street text COLLATE \"C\", city text)"},
	{Name: "drop_type", SQL: "DROP TYPE address"},
	{Name: "drop_type_if_exists_cascade", SQL: "DROP TYPE IF EXISTS address CASCADE"},
	{Name: "alter_type_add_attribute", SQL: "ALTER TYPE address ADD ATTRIBUTE country text"},
	{Name: "alter_type_drop_attribute", SQL: "ALTER TYPE address DROP ATTRIBUTE city"},
	{Name: "alter_type_alter_attribute_type", SQL: "ALTER TYPE address ALTER ATTRIBUTE street TYPE varchar(255)"},
	{Name: "alter_type_rename_attribute", SQL: "ALTER TYPE address RENAME ATTRIBUTE street TO line1"},
	{Name: "alter_type_rename", SQL: "ALTER TYPE address RENAME TO mailing_address"},
	{Name: "alter_type_set_schema", SQL: "ALTER TYPE address SET SCHEMA archive"},
}

// TestCompositeTypeLifecycleASTCensus inspects raw pg_query_go AST facts for
// all composite type lifecycle DDL candidates. This is a read-only
// characterization test — no production code is modified.
func TestCompositeTypeLifecycleASTCensus(t *testing.T) {
	t.Parallel()
	t.Log("")
	t.Log("=== PostgreSQL Composite Type Lifecycle AST Census ===")
	t.Logf("%-40s | %-26s | %s", "Case", "Node Kind", "AST Facts")
	t.Log(string(make([]byte, 140)))

	for _, tc := range pgCompositeTypeLifecycleCensusCases {
		tree, err := pg_query.Parse(tc.SQL)
		if err != nil {
			t.Fatalf("%s: parse failed: %v", tc.Name, err)
		}
		if len(tree.Stmts) != 1 {
			t.Fatalf("%s: expected 1 statement, got %d", tc.Name, len(tree.Stmts))
		}

		node := tree.Stmts[0].Stmt
		fact := inspectCompositeTypeLifecycleAST(t, tc.Name, tc.SQL, node)
		assertCompositeTypeLifecycleASTFacts(t, fact)
	}
}

func inspectCompositeTypeLifecycleAST(t *testing.T, name, sql string, node *pg_query.Node) compositeTypeLifecycleASTFact {
	t.Helper()
	fact := compositeTypeLifecycleASTFact{Name: name, SQL: sql}

	switch n := node.GetNode().(type) {
	case *pg_query.Node_CompositeTypeStmt:
		stmt := n.CompositeTypeStmt
		fact.TopKind = "CompositeTypeStmt"
		rv := stmt.GetTypevar()
		if rv != nil {
			fact.ObjectName = rv.GetRelname()
			if rv.GetSchemaname() != "" {
				fact.SchemaName = rv.GetSchemaname()
			}
			fact.NameNodeShape = "RangeVar"
		}
		for _, col := range stmt.GetColdeflist() {
			cd := col.GetColumnDef()
			if cd == nil {
				continue
			}
			fact.AttributeNames = append(fact.AttributeNames, cd.GetColname())
			fact.AttributeCount++
			if tn := cd.GetTypeName(); tn != nil {
				fact.AttributeTypes = append(fact.AttributeTypes, typeNameStringFromNodes(tn))
			} else {
				fact.AttributeTypes = append(fact.AttributeTypes, "")
			}
			// Collation is on ColumnDef.coll_clause, not constraints
			if cc := cd.GetCollClause(); cc != nil {
				fact.HasCollation = true
				for _, cn := range cc.GetCollname() {
					if s := cn.GetString_(); s != nil && s.GetSval() != "" {
						fact.CollationName = s.GetSval()
						break
					}
				}
			}
		}
		t.Logf("%-40s | %-26s | object=%q schema=%q attrs=%d names=%v types=%v collation=%v shape=%s",
			name, fact.TopKind, fact.ObjectName, fact.SchemaName,
			fact.AttributeCount, fact.AttributeNames, fact.AttributeTypes,
			fact.HasCollation, fact.NameNodeShape)

	case *pg_query.Node_DropStmt:
		stmt := n.DropStmt
		fact.TopKind = "DropStmt"
		fact.DropRemoveType = stmt.GetRemoveType().String()
		fact.ObjectName = typeNameFromDropObjects(stmt.GetObjects())
		if fact.ObjectName == "" {
			fact.ObjectName = firstStringFromListNodes(stmt.GetObjects())
		}
		fact.NameNodeShape = describeDropObjectsShape(stmt.GetObjects())
		fact.DropMissingOK = stmt.GetMissingOk()
		fact.DropCascade = stmt.GetBehavior() == pg_query.DropBehavior_DROP_CASCADE
		t.Logf("%-40s | %-26s | object=%q remove_type=%s missing_ok=%v cascade=%v obj_shape=%s",
			name, fact.TopKind, fact.ObjectName, fact.DropRemoveType,
			fact.DropMissingOK, fact.DropCascade, fact.NameNodeShape)

	case *pg_query.Node_AlterTableStmt:
		// pg_query maps ALTER TYPE ... ADD/DROP/ALTER ATTRIBUTE to AlterTableStmt
		// with ObjectType_OBJECT_TYPE and AT_AlterColumnType/AT_AddColumn/AT_DropColumn cmds.
		stmt := n.AlterTableStmt
		fact.TopKind = "AlterTableStmt"
		rng := stmt.GetRelation()
		if rng != nil {
			fact.ObjectName = rng.GetRelname()
			if rng.GetSchemaname() != "" {
				fact.SchemaName = rng.GetSchemaname()
			}
			fact.NameNodeShape = "RangeVar"
		}
		for _, cmdNode := range stmt.GetCmds() {
			cmd := cmdNode.GetAlterTableCmd()
			if cmd == nil {
				continue
			}
			subtype := cmd.GetSubtype().String()
			fact.AlterTableCmds = append(fact.AlterTableCmds, subtype)
			switch subtype {
			case "AT_AddColumn":
				fact.AlterAction = "add_attribute"
				if def := cmd.GetDef(); def != nil {
					if cd := def.GetColumnDef(); cd != nil {
						fact.AlterTargetName = cd.GetColname()
						if tn := cd.GetTypeName(); tn != nil {
							fact.AlterNewType = typeNameStringFromNodes(tn)
						}
					}
				}
			case "AT_DropColumn":
				fact.AlterAction = "drop_attribute"
				fact.AlterTargetName = cmd.GetName()
			case "AT_AlterColumnType":
				fact.AlterAction = "alter_attribute_type"
				fact.AlterTargetName = cmd.GetName()
				if def := cmd.GetDef(); def != nil {
					// For ALTER TYPE ... ALTER ATTRIBUTE ... TYPE, pg_query
					// puts the new type in ColumnDef.TypeName, not TypeName directly.
					if cd := def.GetColumnDef(); cd != nil {
						if tn := cd.GetTypeName(); tn != nil {
							fact.AlterNewType = typeNameStringFromNodes(tn)
						}
					} else if tn := def.GetTypeName(); tn != nil {
						fact.AlterNewType = typeNameStringFromNodes(tn)
					}
				}
			}
		}
		t.Logf("%-40s | %-26s | object=%q schema=%q cmds=%v action=%s target=%q new_type=%q new_name=%q shape=%s",
			name, fact.TopKind, fact.ObjectName, fact.SchemaName,
			fact.AlterTableCmds, fact.AlterAction, fact.AlterTargetName,
			fact.AlterNewType, fact.AlterNewName, fact.NameNodeShape)

	case *pg_query.Node_RenameStmt:
		stmt := n.RenameStmt
		fact.TopKind = "RenameStmt"
		fact.RenameObjectType = stmt.GetRenameType().String()
		fact.AlterNewName = stmt.GetNewname()
		// Distinguish RENAME ATTRIBUTE from RENAME TO by renameType.
		// RENAME ATTRIBUTE uses OBJECT_ATTRIBUTE with the type name in
		// stmt.GetRelation() and attribute name in stmt.GetSubname().
		// RENAME TO uses OBJECT_TYPE with the type name in stmt.GetObject().
		if stmt.GetRenameType() == pg_query.ObjectType_OBJECT_ATTRIBUTE {
			fact.AlterAction = "rename_attribute"
			if rng := stmt.GetRelation(); rng != nil {
				fact.ObjectName = rng.GetRelname()
				fact.NameNodeShape = "Relation→RangeVar"
			}
			fact.AlterTargetName = stmt.GetSubname()
		} else {
			fact.AlterAction = "rename"
			obj := stmt.GetObject()
			if obj != nil {
				if tn := obj.GetTypeName(); tn != nil {
					fact.ObjectName = typeNameStringFromNodes(tn)
					fact.NameNodeShape = "Object→TypeName"
				} else if list := obj.GetList(); list != nil {
					fact.ObjectName = firstStringFromNodes(list.GetItems())
					fact.NameNodeShape = "Object→List→String"
				} else {
					fact.ObjectName = firstStringFromNodes([]*pg_query.Node{obj})
					fact.NameNodeShape = fmt.Sprintf("Object→%T", obj.GetNode())
				}
			}
		}
		t.Logf("%-40s | %-26s | object=%q rename_type=%s action=%s target=%q new_name=%q obj_shape=%s",
			name, fact.TopKind, fact.ObjectName, fact.RenameObjectType,
			fact.AlterAction, fact.AlterTargetName, fact.AlterNewName, fact.NameNodeShape)

	case *pg_query.Node_AlterObjectSchemaStmt:
		stmt := n.AlterObjectSchemaStmt
		fact.TopKind = "AlterObjectSchemaStmt"
		fact.SetSchemaTarget = stmt.GetNewschema()
		obj := stmt.GetObject()
		if obj != nil {
			if tn := obj.GetTypeName(); tn != nil {
				fact.ObjectName = typeNameStringFromNodes(tn)
				fact.NameNodeShape = "Object→TypeName"
			} else if list := obj.GetList(); list != nil {
				fact.ObjectName = firstStringFromNodes(list.GetItems())
				fact.NameNodeShape = "Object→List→String"
			}
		}
		fact.AlterAction = "set_schema"
		t.Logf("%-40s | %-26s | object=%q object_type=%s new_schema=%q obj_shape=%s",
			name, fact.TopKind, fact.ObjectName,
			stmt.GetObjectType().String(), fact.SetSchemaTarget, fact.NameNodeShape)

	default:
		t.Fatalf("%s: unexpected node type %T", name, node.GetNode())
	}

	return fact
}

// assertCompositeTypeLifecycleASTFacts validates that all expected AST facts
// are present and stable for decision-making.
func assertCompositeTypeLifecycleASTFacts(t *testing.T, fact compositeTypeLifecycleASTFact) {
	t.Helper()

	if fact.TopKind == "" {
		t.Errorf("%s: expected non-empty top kind", fact.Name)
	}

	switch fact.TopKind {
	case "CompositeTypeStmt":
		if fact.ObjectName == "" {
			t.Errorf("%s: expected non-empty object name", fact.Name)
		}
		if fact.AttributeCount == 0 {
			t.Errorf("%s: expected at least one attribute", fact.Name)
		}
		if fact.NameNodeShape != "RangeVar" {
			t.Errorf("%s: expected RangeVar name shape, got %q", fact.Name, fact.NameNodeShape)
		}
	case "DropStmt":
		if fact.ObjectName == "" {
			t.Errorf("%s: expected non-empty object name", fact.Name)
		}
		if fact.DropRemoveType != "OBJECT_TYPE" {
			t.Errorf("%s: expected OBJECT_TYPE remove type, got %s", fact.Name, fact.DropRemoveType)
		}
	case "AlterTableStmt":
		if fact.ObjectName == "" {
			t.Errorf("%s: expected non-empty object name", fact.Name)
		}
		if len(fact.AlterTableCmds) == 0 {
			t.Errorf("%s: expected at least one alter table cmd", fact.Name)
		}
	case "RenameStmt":
		if fact.ObjectName == "" {
			t.Errorf("%s: expected non-empty object name", fact.Name)
		}
	case "AlterObjectSchemaStmt":
		if fact.ObjectName == "" {
			t.Errorf("%s: expected non-empty object name", fact.Name)
		}
		if fact.SetSchemaTarget == "" {
			t.Errorf("%s: expected non-empty set schema target", fact.Name)
		}
	}
}

// TestCompositeTypeLifecycleDeltaScopeBaseline characterizes how the current
// DeltaScope pipeline classifies and extracts composite type lifecycle DDL
// candidates. This is a read-only characterization test — no production code
// is modified.
func TestCompositeTypeLifecycleDeltaScopeBaseline(t *testing.T) {
	t.Parallel()
	t.Log("")
	t.Log("=== PostgreSQL Composite Type Lifecycle DeltaScope Baseline ===")
	t.Logf("%-40s | %-8s | %-5s | %-12s | %s",
		"Case", "Kind", "DDL?", "Unsupported?", "Detail")
	t.Log(string(make([]byte, 140)))

	for _, tc := range pgCompositeTypeLifecycleCensusCases {
		p := New()
		result, parseErr := p.Parse(context.Background(), tc.SQL)
		if parseErr != nil {
			t.Logf("%-40s | %-8s | %-5s | %-12s | parse error: %v",
				tc.Name, "ERROR", "-", "-", parseErr)
			t.Errorf("%s: unexpected parse error: %v", tc.Name, parseErr)
			continue
		}
		if len(result.Statements) != 1 {
			t.Fatalf("%s: expected 1 statement, got %d", tc.Name, len(result.Statements))
		}

		es := result.Statements[0]
		classifies := es.Kind == spec.KindDDL

		baseline := compositeTypeLifecycleBaselineFact{
			Name:       tc.Name,
			Kind:       es.Kind,
			Classifies: classifies,
		}

		stmt, extractErr := es.Extractor.Extract(spec.DialectPostgreSQL, es.RawSQL)
		if extractErr != nil {
			t.Logf("%-40s | %-8s | %-5v | %-12s | extract error: %v",
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

		t.Logf("%-40s | %-8s | %-5s | %-12s | %s",
			tc.Name, baseline.Kind, classStr, unsupported, detail)

		assertCompositeTypeLifecycleBaseline(t, baseline)
	}
}

// assertCompositeTypeLifecycleBaseline captures current (pre-Task-2) expectations.
// At this point, CREATE TYPE composite is unsupported; DROP TYPE is already
// normalized by v0.55.0; ALTER TYPE composite forms are either unsupported or
// fall through to existing ALTER TABLE extraction paths.
func assertCompositeTypeLifecycleBaseline(t *testing.T, fact compositeTypeLifecycleBaselineFact) {
	t.Helper()

	switch fact.Name {
	case "create_type_composite_plain", "create_type_composite_multi_attr",
		"create_type_composite_qualified", "create_type_composite_collate":
		if fact.Unsupported {
			t.Errorf("%s: expected normalized, got unsupported %s", fact.Name, fact.UnsupportedFeature)
		}
		if fact.DDLOperation != "create_type" {
			t.Errorf("%s: expected create_type, got %q", fact.Name, fact.DDLOperation)
		}
		if fact.DDLObjectType != "type" {
			t.Errorf("%s: expected object type type, got %q", fact.Name, fact.DDLObjectType)
		}
		if fact.DDLOptions["type_kind"] != "composite" {
			t.Errorf("%s: expected type_kind=composite, got %q", fact.Name, fact.DDLOptions["type_kind"])
		}

	case "drop_type":
		if fact.Unsupported {
			t.Errorf("%s: expected normalized, got unsupported %s", fact.Name, fact.UnsupportedFeature)
		}
		if fact.DDLOperation != "drop_type" {
			t.Errorf("%s: expected drop_type, got %q", fact.Name, fact.DDLOperation)
		}

	case "drop_type_if_exists_cascade":
		if fact.Unsupported {
			t.Errorf("%s: expected normalized, got unsupported %s", fact.Name, fact.UnsupportedFeature)
		}
		if fact.DDLOperation != "drop_type" {
			t.Errorf("%s: expected drop_type, got %q", fact.Name, fact.DDLOperation)
		}

	case "alter_type_add_attribute":
		if !fact.Unsupported {
			t.Errorf("%s: expected unsupported", fact.Name)
		}
		if fact.UnsupportedFeature != "alter_type_add_attribute" {
			t.Errorf("%s: expected feature alter_type_add_attribute, got %q", fact.Name, fact.UnsupportedFeature)
		}

	case "alter_type_drop_attribute":
		if !fact.Unsupported {
			t.Errorf("%s: expected unsupported", fact.Name)
		}
		if fact.UnsupportedFeature != "alter_type_drop_attribute" {
			t.Errorf("%s: expected feature alter_type_drop_attribute, got %q", fact.Name, fact.UnsupportedFeature)
		}

	case "alter_type_alter_attribute_type":
		if !fact.Unsupported {
			t.Errorf("%s: expected unsupported", fact.Name)
		}
		if fact.UnsupportedFeature != "alter_type_alter_attribute_type" {
			t.Errorf("%s: expected feature alter_type_alter_attribute_type, got %q", fact.Name, fact.UnsupportedFeature)
		}

	case "alter_type_rename_attribute":
		if !fact.Unsupported {
			t.Errorf("%s: expected unsupported", fact.Name)
		}
		if fact.UnsupportedFeature != "alter_type_rename_attribute" {
			t.Errorf("%s: expected feature alter_type_rename_attribute, got %q", fact.Name, fact.UnsupportedFeature)
		}

	case "alter_type_rename":
		if fact.Unsupported {
			t.Errorf("%s: expected normalized, got unsupported %s", fact.Name, fact.UnsupportedFeature)
		}
		if fact.DDLOperation != "alter_type" {
			t.Errorf("%s: expected alter_type, got %q", fact.Name, fact.DDLOperation)
		}
		if fact.DDLOptions["action"] != "rename" {
			t.Errorf("%s: expected action=rename, got %q", fact.Name, fact.DDLOptions["action"])
		}
		if fact.DDLOptions["new_name"] != "mailing_address" {
			t.Errorf("%s: expected new_name=mailing_address, got %q", fact.Name, fact.DDLOptions["new_name"])
		}

	case "alter_type_set_schema":
		if fact.Unsupported {
			t.Errorf("%s: expected normalized, got unsupported %s", fact.Name, fact.UnsupportedFeature)
		}
		if fact.DDLOperation != "alter_type" {
			t.Errorf("%s: expected alter_type, got %q", fact.Name, fact.DDLOperation)
		}
		if fact.DDLOptions["action"] != "set_schema" {
			t.Errorf("%s: expected action=set_schema, got %q", fact.Name, fact.DDLOptions["action"])
		}
		if fact.DDLOptions["new_schema"] != "archive" {
			t.Errorf("%s: expected new_schema=archive, got %q", fact.Name, fact.DDLOptions["new_schema"])
		}
	}
}
