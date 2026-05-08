//go:build postgresql

package postgresql

import (
	"context"
	"fmt"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// typeLifecycleASTFact captures observed AST facts for a type lifecycle DDL
// candidate. Fields are populated from pg_query_go direct inspection, not
// from DeltaScope's extraction pipeline.
type typeLifecycleASTFact struct {
	Name          string
	SQL           string
	TopKind       string
	ObjectName    string
	TypeKind      string // enum, composite, domain
	EnumLabels    []string
	AlterAction   string // add_value (AlterEnumStmt only)
	AlterValue    string
	IfNotExists   bool
	Placement     string // before, after, or empty
	Neighbor      string
	DropMissingOK bool
	DropCascade   bool
}

// typeLifecycleBaselineFact captures how the current DeltaScope pipeline
// handles each type lifecycle DDL candidate before production changes.
type typeLifecycleBaselineFact struct {
	Name               string
	Kind               spec.Kind
	Unsupported        bool
	UnsupportedFeature string
	UnsupportedReason  string
	DDLOperation       string
	DDLObjectName      string
	DDLObjectType      string
	DDLOptions         map[string]string
}

var pgTypeLifecycleCensusCases = []struct {
	Name string
	SQL  string
}{
	{Name: "create_type_enum", SQL: "CREATE TYPE color AS ENUM ('red', 'green', 'blue')"},
	{Name: "alter_type_add_value", SQL: "ALTER TYPE color ADD VALUE 'yellow'"},
	{Name: "alter_type_add_value_if_not_exists", SQL: "ALTER TYPE color ADD VALUE IF NOT EXISTS 'yellow'"},
	{Name: "alter_type_add_value_before", SQL: "ALTER TYPE color ADD VALUE 'yellow' BEFORE 'green'"},
	{Name: "alter_type_add_value_after", SQL: "ALTER TYPE color ADD VALUE 'yellow' AFTER 'green'"},
	{Name: "drop_type", SQL: "DROP TYPE color"},
	{Name: "drop_type_if_exists_cascade", SQL: "DROP TYPE IF EXISTS color CASCADE"},
	{Name: "create_type_composite", SQL: "CREATE TYPE address AS (street text, city text)"},
	{Name: "create_domain", SQL: "CREATE DOMAIN email AS text CHECK (VALUE <> '')"},
}

// TestTypeLifecycleASTCensus inspects raw pg_query_go AST facts for all type
// lifecycle DDL candidates. This is a read-only characterization test — no
// production code is modified.
func TestTypeLifecycleASTCensus(t *testing.T) {
	t.Log("")
	t.Log("=== PostgreSQL Type Lifecycle AST Census ===")
	t.Logf("%-38s | %-22s | %s", "Case", "Node Kind", "AST Facts")
	t.Log(string(make([]byte, 120)))

	for _, tc := range pgTypeLifecycleCensusCases {
		tree, err := pg_query.Parse(tc.SQL)
		if err != nil {
			t.Fatalf("%s: parse failed: %v", tc.Name, err)
		}
		if len(tree.Stmts) != 1 {
			t.Fatalf("%s: expected 1 statement, got %d", tc.Name, len(tree.Stmts))
		}

		node := tree.Stmts[0].Stmt
		fact := inspectTypeLifecycleAST(t, tc.Name, node)
		assertTypeLifecycleASTFacts(t, fact)
	}
}

func inspectTypeLifecycleAST(t *testing.T, name string, node *pg_query.Node) typeLifecycleASTFact {
	t.Helper()
	fact := typeLifecycleASTFact{Name: name}

	switch n := node.GetNode().(type) {
	case *pg_query.Node_CreateEnumStmt:
		stmt := n.CreateEnumStmt
		fact.TopKind = "CreateEnumStmt"
		fact.ObjectName = firstStringFromNodes(stmt.GetTypeName())
		fact.TypeKind = "enum"
		for _, v := range stmt.GetVals() {
			if s := v.GetString_(); s != nil {
				fact.EnumLabels = append(fact.EnumLabels, s.GetSval())
			}
		}
		t.Logf("%-38s | %-22s | object=%q type_kind=%s labels=%v",
			name, fact.TopKind, fact.ObjectName, fact.TypeKind, fact.EnumLabels)

	case *pg_query.Node_AlterEnumStmt:
		stmt := n.AlterEnumStmt
		fact.TopKind = "AlterEnumStmt"
		fact.ObjectName = firstStringFromNodes(stmt.GetTypeName())
		fact.TypeKind = "enum"
		fact.AlterAction = "add_value"
		fact.AlterValue = stmt.GetNewVal()
		fact.IfNotExists = stmt.GetSkipIfNewValExists()
		neighbor := stmt.GetNewValNeighbor()
		if neighbor != "" {
			fact.Neighbor = neighbor
			if stmt.GetNewValIsAfter() {
				fact.Placement = "after"
			} else {
				fact.Placement = "before"
			}
		}
		t.Logf("%-38s | %-22s | object=%q action=%s value=%q if_not_exists=%v placement=%q neighbor=%q",
			name, fact.TopKind, fact.ObjectName, fact.AlterAction,
			fact.AlterValue, fact.IfNotExists, fact.Placement, fact.Neighbor)

	case *pg_query.Node_DropStmt:
		stmt := n.DropStmt
		fact.TopKind = "DropStmt"
		fact.ObjectName = typeNameFromDropObjects(stmt.GetObjects())
		fact.DropMissingOK = stmt.GetMissingOk()
		fact.DropCascade = stmt.GetBehavior() == pg_query.DropBehavior_DROP_CASCADE
		t.Logf("%-38s | %-22s | object=%q missing_ok=%v cascade=%v remove_type=%d",
			name, fact.TopKind, fact.ObjectName, fact.DropMissingOK, fact.DropCascade, stmt.GetRemoveType())

	case *pg_query.Node_CompositeTypeStmt:
		stmt := n.CompositeTypeStmt
		fact.TopKind = "CompositeTypeStmt"
		rv := stmt.GetTypevar()
		if rv != nil {
			fact.ObjectName = rv.GetRelname()
		}
		fact.TypeKind = "composite"
		var colNames []string
		for _, col := range stmt.GetColdeflist() {
			cd := col.GetColumnDef()
			if cd != nil {
				colNames = append(colNames, cd.GetColname())
			}
		}
		t.Logf("%-38s | %-22s | object=%q type_kind=%s columns=%v",
			name, fact.TopKind, fact.ObjectName, fact.TypeKind, colNames)

	case *pg_query.Node_CreateDomainStmt:
		stmt := n.CreateDomainStmt
		fact.TopKind = "CreateDomainStmt"
		fact.ObjectName = firstStringFromNodes(stmt.GetDomainname())
		fact.TypeKind = "domain"
		baseType := typeNameStringFromNodes(stmt.GetTypeName())
		constraintTypes := domainConstraintTypes(stmt.GetConstraints())
		t.Logf("%-38s | %-22s | object=%q type_kind=%s base_type=%s constraints=%v",
			name, fact.TopKind, fact.ObjectName, fact.TypeKind, baseType, constraintTypes)

	default:
		t.Fatalf("%s: unexpected node type %T", name, node.GetNode())
	}

	return fact
}

func assertTypeLifecycleASTFacts(t *testing.T, fact typeLifecycleASTFact) {
	t.Helper()

	if fact.ObjectName == "" {
		t.Errorf("%s: expected non-empty object name", fact.Name)
	}
	if fact.TopKind == "" {
		t.Errorf("%s: expected non-empty top kind", fact.Name)
	}

	switch fact.TopKind {
	case "CreateEnumStmt":
		if len(fact.EnumLabels) == 0 {
			t.Errorf("%s: expected at least one enum label", fact.Name)
		}
	case "AlterEnumStmt":
		if fact.AlterValue == "" {
			t.Errorf("%s: expected non-empty alter value", fact.Name)
		}
	case "DropStmt":
		// No additional required facts beyond ObjectName
	case "CompositeTypeStmt":
		// Stable AST — object name extractable
	case "CreateDomainStmt":
		// Stable AST — object name extractable
	}
}

// TestTypeLifecycleDeltaScopeBaseline characterizes how the current DeltaScope
// pipeline classifies and extracts type lifecycle DDL candidates.
// After Task 2, the 7 supported forms normalize through spec.DDL and the 2
// deferred forms (composite type, domain) return explicit unsupported details.
func TestTypeLifecycleDeltaScopeBaseline(t *testing.T) {
	t.Log("")
	t.Log("=== PostgreSQL Type Lifecycle DeltaScope Baseline ===")
	t.Logf("%-38s | %-8s | %-12s | %s",
		"Case", "Kind", "Unsupported?", "Detail")
	t.Log(string(make([]byte, 120)))

	for _, tc := range pgTypeLifecycleCensusCases {
		p := New()
		result, parseErr := p.Parse(context.Background(), tc.SQL)
		if parseErr != nil {
			t.Logf("%-38s | %-8s | %-12s | parse error: %v",
				tc.Name, "ERROR", "-", parseErr)
			t.Errorf("%s: unexpected parse error: %v", tc.Name, parseErr)
			continue
		}
		if len(result.Statements) != 1 {
			t.Fatalf("%s: expected 1 statement, got %d", tc.Name, len(result.Statements))
		}

		es := result.Statements[0]
		stmt, extractErr := es.Extractor.Extract(spec.DialectPostgreSQL, es.RawSQL)
		if extractErr != nil {
			t.Logf("%-38s | %-8s | %-12s | extract error: %v",
				tc.Name, es.Kind, "-", extractErr)
			t.Errorf("%s: unexpected extract error: %v", tc.Name, extractErr)
			continue
		}

		baseline := typeLifecycleBaselineFact{
			Name:        tc.Name,
			Kind:        stmt.Kind,
			Unsupported: stmt.Unsupported != nil,
		}

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

		t.Logf("%-38s | %-8s | %-12s | %s",
			tc.Name, baseline.Kind, unsupported, detail)

		// All candidates must parse successfully via DeltaScope.
		if parseErr != nil {
			t.Errorf("%s: must parse successfully", tc.Name)
		}

		// Task 2 normalization assertions.
		assertTypeLifecycleBaseline(t, baseline)
	}
}

func assertTypeLifecycleBaseline(t *testing.T, fact typeLifecycleBaselineFact) {
	t.Helper()
	switch fact.Name {
	case "create_type_enum":
		if fact.Unsupported {
			t.Errorf("%s: expected normalized, got unsupported %s", fact.Name, fact.UnsupportedFeature)
		}
		if fact.DDLOperation != "create_type" {
			t.Errorf("%s: expected create_type, got %q", fact.Name, fact.DDLOperation)
		}
		if fact.DDLOptions["type_kind"] != "enum" {
			t.Errorf("%s: expected type_kind=enum, got %q", fact.Name, fact.DDLOptions["type_kind"])
		}
	case "alter_type_add_value", "alter_type_add_value_if_not_exists",
		"alter_type_add_value_before", "alter_type_add_value_after":
		if fact.Unsupported {
			t.Errorf("%s: expected normalized, got unsupported %s", fact.Name, fact.UnsupportedFeature)
		}
		if fact.DDLOperation != "alter_type" {
			t.Errorf("%s: expected alter_type, got %q", fact.Name, fact.DDLOperation)
		}
		if fact.DDLOptions["action"] != "add_value" {
			t.Errorf("%s: expected action=add_value, got %q", fact.Name, fact.DDLOptions["action"])
		}
	case "drop_type", "drop_type_if_exists_cascade":
		if fact.Unsupported {
			t.Errorf("%s: expected normalized, got unsupported %s", fact.Name, fact.UnsupportedFeature)
		}
		if fact.DDLOperation != "drop_type" {
			t.Errorf("%s: expected drop_type, got %q", fact.Name, fact.DDLOperation)
		}
	case "create_type_composite":
		if fact.Unsupported {
			t.Errorf("%s: expected normalized, got unsupported %s", fact.Name, fact.UnsupportedFeature)
		}
		if fact.DDLOperation != "create_type" {
			t.Errorf("%s: expected create_type, got %q", fact.Name, fact.DDLOperation)
		}
	case "create_domain":
		if fact.Unsupported {
			t.Errorf("%s: expected normalized, got unsupported %s", fact.Name, fact.UnsupportedFeature)
		}
		if fact.DDLOperation != "create_domain" {
			t.Errorf("%s: expected create_domain, got %q", fact.Name, fact.DDLOperation)
		}
	}
}

// --- AST helper functions ---

// firstStringFromNodes is now provided by extractor.go in the same package.

// typeNameFromDropObjects extracts the type name from a DropStmt's objects list.
// DROP TYPE uses TypeName nodes instead of the simple String/List format used by
// tables, views, and schemas.
func typeNameFromDropObjects(objects []*pg_query.Node) string {
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

// typeNameStringFromNodes builds a type name string from TypeName node names.
func typeNameStringFromNodes(tn *pg_query.TypeName) string {
	if tn == nil {
		return ""
	}
	return firstStringFromNodes(tn.GetNames())
}

// domainConstraintTypes extracts constraint type strings from domain constraints.
func domainConstraintTypes(constraints []*pg_query.Node) []string {
	var types []string
	for _, c := range constraints {
		con := c.GetConstraint()
		if con != nil {
			types = append(types, con.GetContype().String())
		}
	}
	return types
}
