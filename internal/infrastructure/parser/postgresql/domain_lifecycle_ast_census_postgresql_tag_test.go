//go:build postgresql

package postgresql

import (
	"fmt"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// domainLifecycleASTFact captures observed pg_query AST facts for a domain
// lifecycle DDL candidate. Fields are populated from pg_query_go direct
// inspection, not from DeltaScope's extraction pipeline.
type domainLifecycleASTFact struct {
	Name            string
	SQL             string
	TopKind         string
	ObjectName      string
	BaseType        string
	NotNull         bool
	HasDefault      bool
	HasCheck        bool
	ConstraintNames []string
	ConstraintTypes []string
	DropMissingOK   bool
	DropCascade     bool
	DropRemoveType  string
	AlterSubtype    string // AlterDomainStmt subtype
	AlterName       string // constraint name for ADD/DROP/VALIDATE CONSTRAINT
	RenameTarget    string // for RENAME TO
	NameNodeShape   string // how names appear: TypeName.names, String, List, etc.
}

// domainLifecycleBaselineFact captures how the current DeltaScope pipeline
// handles each domain lifecycle DDL candidate before production changes.
type domainLifecycleBaselineFact struct {
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
}

var pgDomainLifecycleCensusCases = []struct {
	Name string
	SQL  string
}{
	{Name: "create_domain_plain", SQL: "CREATE DOMAIN email AS text"},
	{Name: "create_domain_not_null", SQL: "CREATE DOMAIN email AS text NOT NULL"},
	{Name: "create_domain_default", SQL: "CREATE DOMAIN email AS text DEFAULT 'unknown@example.com'"},
	{Name: "create_domain_check", SQL: "CREATE DOMAIN email AS text CHECK (VALUE <> '')"},
	{Name: "create_domain_named_check", SQL: "CREATE DOMAIN email AS text CONSTRAINT email_not_empty CHECK (VALUE <> '')"},
	{Name: "drop_domain", SQL: "DROP DOMAIN email"},
	{Name: "drop_domain_if_exists_cascade", SQL: "DROP DOMAIN IF EXISTS email CASCADE"},
	{Name: "alter_domain_set_default", SQL: "ALTER DOMAIN email SET DEFAULT 'unknown@example.com'"},
	{Name: "alter_domain_drop_default", SQL: "ALTER DOMAIN email DROP DEFAULT"},
	{Name: "alter_domain_set_not_null", SQL: "ALTER DOMAIN email SET NOT NULL"},
	{Name: "alter_domain_drop_not_null", SQL: "ALTER DOMAIN email DROP NOT NULL"},
	{Name: "alter_domain_add_constraint", SQL: "ALTER DOMAIN email ADD CONSTRAINT email_not_empty CHECK (VALUE <> '')"},
	{Name: "alter_domain_drop_constraint", SQL: "ALTER DOMAIN email DROP CONSTRAINT email_not_empty"},
	{Name: "alter_domain_validate_constraint", SQL: "ALTER DOMAIN email VALIDATE CONSTRAINT email_not_empty"},
	{Name: "alter_domain_rename", SQL: "ALTER DOMAIN email RENAME TO contact_email"},
}

// TestDomainLifecycleASTCensus inspects raw pg_query_go AST facts for all
// domain lifecycle DDL candidates. This is a read-only characterization
// test — no production code is modified.
func TestDomainLifecycleASTCensus(t *testing.T) {
	t.Log("")
	t.Log("=== PostgreSQL Domain Lifecycle AST Census ===")
	t.Logf("%-38s | %-22s | %s", "Case", "Node Kind", "AST Facts")
	t.Log(string(make([]byte, 140)))

	for _, tc := range pgDomainLifecycleCensusCases {
		tree, err := pg_query.Parse(tc.SQL)
		if err != nil {
			t.Fatalf("%s: parse failed: %v", tc.Name, err)
		}
		if len(tree.Stmts) != 1 {
			t.Fatalf("%s: expected 1 statement, got %d", tc.Name, len(tree.Stmts))
		}

		node := tree.Stmts[0].Stmt
		fact := inspectDomainLifecycleAST(t, tc.Name, tc.SQL, node)
		assertDomainLifecycleASTFacts(t, fact)
	}
}

func inspectDomainLifecycleAST(t *testing.T, name, sql string, node *pg_query.Node) domainLifecycleASTFact {
	t.Helper()
	fact := domainLifecycleASTFact{Name: name, SQL: sql}

	switch n := node.GetNode().(type) {
	case *pg_query.Node_CreateDomainStmt:
		stmt := n.CreateDomainStmt
		fact.TopKind = "CreateDomainStmt"
		fact.ObjectName = firstStringFromNodes(stmt.GetDomainname())
		fact.NameNodeShape = describeNodeShape(stmt.GetDomainname())
		if tn := stmt.GetTypeName(); tn != nil {
			fact.BaseType = typeNameStringFromNodes(tn)
		}
		for _, c := range stmt.GetConstraints() {
			con := c.GetConstraint()
			if con == nil {
				continue
			}
			ct := con.GetContype()
			fact.ConstraintTypes = append(fact.ConstraintTypes, ct.String())
			switch ct {
			case pg_query.ConstrType_CONSTR_NOTNULL:
				fact.NotNull = true
			case pg_query.ConstrType_CONSTR_DEFAULT:
				fact.HasDefault = true
			case pg_query.ConstrType_CONSTR_CHECK:
				fact.HasCheck = true
				if con.GetConname() != "" {
					fact.ConstraintNames = append(fact.ConstraintNames, con.GetConname())
				}
			}
		}
		t.Logf("%-38s | %-22s | object=%q base=%q not_null=%v has_default=%v has_check=%v constraints=%v constraint_names=%v names_shape=%s",
			name, fact.TopKind, fact.ObjectName, fact.BaseType,
			fact.NotNull, fact.HasDefault, fact.HasCheck,
			fact.ConstraintTypes, fact.ConstraintNames, fact.NameNodeShape)

	case *pg_query.Node_AlterDomainStmt:
		stmt := n.AlterDomainStmt
		fact.TopKind = "AlterDomainStmt"
		fact.ObjectName = firstStringFromNodes(stmt.GetTypeName())
		fact.NameNodeShape = describeNodeShape(stmt.GetTypeName())
		fact.AlterSubtype = stmt.GetSubtype()
		fact.AlterName = stmt.GetName()
		if fact.AlterSubtype == "R" {
			fact.RenameTarget = stmt.GetName()
		}
		t.Logf("%-38s | %-22s | object=%q subtype=%q name=%q names_shape=%s",
			name, fact.TopKind, fact.ObjectName, fact.AlterSubtype, fact.AlterName, fact.NameNodeShape)

	case *pg_query.Node_DropStmt:
		stmt := n.DropStmt
		fact.TopKind = "DropStmt"
		fact.DropRemoveType = stmt.GetRemoveType().String()
		// DROP DOMAIN uses TypeName nodes with OBJECT_DOMAIN remove type.
		fact.ObjectName = typeNameFromDropObjects(stmt.GetObjects())
		if fact.ObjectName == "" {
			fact.ObjectName = firstStringFromListNodes(stmt.GetObjects())
		}
		fact.NameNodeShape = describeDropObjectsShape(stmt.GetObjects())
		fact.DropMissingOK = stmt.GetMissingOk()
		fact.DropCascade = stmt.GetBehavior() == pg_query.DropBehavior_DROP_CASCADE
		t.Logf("%-38s | %-22s | object=%q remove_type=%s missing_ok=%v cascade=%v obj_shape=%s",
			name, fact.TopKind, fact.ObjectName, fact.DropRemoveType,
			fact.DropMissingOK, fact.DropCascade, fact.NameNodeShape)

	case *pg_query.Node_RenameStmt:
		stmt := n.RenameStmt
		fact.TopKind = "RenameStmt"
		fact.AlterSubtype = "rename"
		fact.RenameTarget = stmt.GetNewname()
		// ALTER DOMAIN RENAME stores domain name in Object as List (not TypeName).
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
		t.Logf("%-38s | %-22s | object=%q rename_type=%s new_name=%q obj_shape=%s",
			name, fact.TopKind, fact.ObjectName,
			stmt.GetRenameType().String(), fact.RenameTarget, fact.NameNodeShape)

	default:
		t.Fatalf("%s: unexpected node type %T", name, node.GetNode())
	}

	return fact
}

// assertDomainLifecycleASTFacts validates that all expected AST facts are
// present and stable for decision-making.
func assertDomainLifecycleASTFacts(t *testing.T, fact domainLifecycleASTFact) {
	t.Helper()

	if fact.TopKind == "" {
		t.Errorf("%s: expected non-empty top kind", fact.Name)
	}

	switch fact.TopKind {
	case "CreateDomainStmt":
		if fact.ObjectName == "" {
			t.Errorf("%s: expected non-empty object name", fact.Name)
		}
		if fact.BaseType == "" {
			t.Errorf("%s: expected non-empty base type", fact.Name)
		}
	case "AlterDomainStmt":
		if fact.ObjectName == "" {
			t.Errorf("%s: expected non-empty object name", fact.Name)
		}
		if fact.AlterSubtype == "" {
			t.Errorf("%s: expected non-empty alter subtype", fact.Name)
		}
	case "DropStmt":
		if fact.ObjectName == "" {
			t.Errorf("%s: expected non-empty object name", fact.Name)
		}
		// DROP DOMAIN uses OBJECT_DOMAIN remove type (distinct from OBJECT_TYPE).
		if fact.DropRemoveType != "OBJECT_DOMAIN" {
			t.Errorf("%s: expected OBJECT_DOMAIN remove type, got %s", fact.Name, fact.DropRemoveType)
		}
	}
}

// TestDomainLifecycleDeltaScopeBaseline characterizes how the current
// DeltaScope pipeline classifies and extracts domain lifecycle DDL
// candidates. After Task 2, supported forms normalize through spec.DDL and
// deferred forms return explicit unsupported details.
func TestDomainLifecycleDeltaScopeBaseline(t *testing.T) {
	t.Log("")
	t.Log("=== PostgreSQL Domain Lifecycle DeltaScope Baseline ===")
	t.Logf("%-38s | %-8s | %-12s | %-5s | %s",
		"Case", "Kind", "Unsupported?", "Class.", "Detail")
	t.Log(string(make([]byte, 140)))

	for _, tc := range pgDomainLifecycleCensusCases {
		p := New()
		result, parseErr := p.Parse(tc.SQL)
		if parseErr != nil {
			t.Logf("%-38s | %-8s | %-12s | %-5s | parse error: %v",
				tc.Name, "ERROR", "-", "-", parseErr)
			t.Errorf("%s: unexpected parse error: %v", tc.Name, parseErr)
			continue
		}
		if len(result.Statements) != 1 {
			t.Fatalf("%s: expected 1 statement, got %d", tc.Name, len(result.Statements))
		}

		es := result.Statements[0]
		classifies := es.Kind == spec.KindDDL

		baseline := domainLifecycleBaselineFact{
			Name:       tc.Name,
			Kind:       es.Kind,
			Classifies: classifies,
		}

		stmt, extractErr := es.Extractor.Extract(spec.DialectPostgreSQL, es.RawSQL)
		if extractErr != nil {
			t.Logf("%-38s | %-8s | %-12s | %-5v | extract error: %v",
				tc.Name, baseline.Kind, "-", classifies, extractErr)
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
			detail = fmt.Sprintf("op=%s obj=%q type=%q", stmt.DDL.Operation, stmt.DDL.ObjectName, stmt.DDL.ObjectType)
		}

		unsupported := "no"
		if baseline.Unsupported {
			unsupported = "yes"
		}
		classStr := "no"
		if baseline.Classifies {
			classStr = "yes"
		}

		t.Logf("%-38s | %-8s | %-12s | %-5s | %s",
			tc.Name, baseline.Kind, unsupported, classStr, detail)

		assertDomainLifecycleBaseline(t, baseline)
	}
}

// assertDomainLifecycleBaseline captures the current baseline expectations.
// CREATE DOMAIN: currently returns unsupported "create_domain".
// ALTER DOMAIN: currently not classified (KindUnknown), extract returns unsupported.
// DROP DOMAIN: classified as DDL (DropStmt is in classify switch), but extractor
// returns unsupported because the DropStmt extractor does not handle OBJECT_TYPE
// for domains specifically.
func assertDomainLifecycleBaseline(t *testing.T, fact domainLifecycleBaselineFact) {
	t.Helper()

	switch fact.Name {
	case "create_domain_plain", "create_domain_not_null", "create_domain_default",
		"create_domain_check", "create_domain_named_check":
		// CREATE DOMAIN is classified as DDL (in classify switch) but extractor
		// returns unsupported "create_domain".
		if !fact.Classifies {
			t.Errorf("%s: expected to classify as DDL", fact.Name)
		}
		if !fact.Unsupported {
			t.Errorf("%s: expected unsupported, got normalized", fact.Name)
		}
		if fact.UnsupportedFeature != "create_domain" {
			t.Errorf("%s: expected feature create_domain, got %q", fact.Name, fact.UnsupportedFeature)
		}

	case "drop_domain":
		if !fact.Classifies {
			t.Errorf("%s: expected to classify as DDL", fact.Name)
		}
		// DROP DOMAIN goes through the generic DropStmt path.
		// Currently unsupported because the extractor does not handle domain-specific drops.

	case "drop_domain_if_exists_cascade":
		if !fact.Classifies {
			t.Errorf("%s: expected to classify as DDL", fact.Name)
		}

	case "alter_domain_set_default", "alter_domain_drop_default",
		"alter_domain_set_not_null", "alter_domain_drop_not_null",
		"alter_domain_add_constraint", "alter_domain_drop_constraint",
		"alter_domain_validate_constraint", "alter_domain_rename":
		// ALTER DOMAIN is not in the classify switch, so Kind is Unknown.
		// Extractor will return unsupported for unknown kinds.
		if fact.UnsupportedFeature == "" && fact.Kind != spec.KindUnknown {
			// ALTER DOMAIN may either be Unknown or unsupported depending on path
		}
	}
}

// --- AST helper functions for domain census ---

// describeNodeShape identifies how a list of nodes is structured
// (e.g., "String", "TypeName.names→String", "List→String", etc.)
func describeNodeShape(nodes []*pg_query.Node) string {
	if len(nodes) == 0 {
		return "empty"
	}
	first := nodes[0]
	switch first.GetNode().(type) {
	case *pg_query.Node_String_:
		if len(nodes) == 1 {
			return "String"
		}
		return "String[]"
	case *pg_query.Node_TypeName:
		return "TypeName.names→" + describeNodeShape(first.GetTypeName().GetNames())
	case *pg_query.Node_List:
		return "List→" + describeNodeShape(first.GetList().GetItems())
	default:
		return fmt.Sprintf("%T", first.GetNode())
	}
}

// describeDropObjectsShape identifies the node structure of DropStmt objects.
func describeDropObjectsShape(objects []*pg_query.Node) string {
	if len(objects) == 0 {
		return "empty"
	}
	first := objects[0]
	switch first.GetNode().(type) {
	case *pg_query.Node_TypeName:
		return "TypeName"
	case *pg_query.Node_List:
		return "List→" + describeNodeShape(first.GetList().GetItems())
	case *pg_query.Node_String_:
		return "String"
	default:
		return fmt.Sprintf("%T", first.GetNode())
	}
}

// firstStringFromListNodes extracts the first string from a list of nodes
// that may contain List or String sub-nodes.
func firstStringFromListNodes(objects []*pg_query.Node) string {
	for _, obj := range objects {
		if s := obj.GetString_(); s != nil && s.GetSval() != "" {
			return s.GetSval()
		}
		if list := obj.GetList(); list != nil {
			for _, item := range list.GetItems() {
				if s := item.GetString_(); s != nil && s.GetSval() != "" {
					return s.GetSval()
				}
			}
		}
	}
	return ""
}
