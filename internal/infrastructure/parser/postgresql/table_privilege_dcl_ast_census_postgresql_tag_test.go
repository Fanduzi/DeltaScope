//go:build postgresql

package postgresql

import (
	"fmt"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// tablePrivilegeDCLASTFact captures observed pg_query AST facts for a
// PostgreSQL table privilege DCL candidate.
type tablePrivilegeDCLASTFact struct {
	Name            string
	SQL             string
	TopKind         string   // GrantStmt, GrantRoleStmt, AlterDefaultPrivilegesStmt
	GrantMode       string   // "grant" or "revoke" (from GrantStmt.IsGrant)
	TargetType      string   // ACL_TARGET_OBJECT, ACL_TARGET_ALL_IN_SCHEMA, etc.
	ObjType         string   // OBJECT_TABLE, OBJECT_SEQUENCE, etc.
	ObjectsShape    string   // how objects are represented: RangeVar, List→String, etc.
	ObjectNames     []string // extracted object names
	ObjectSchemas   []string // extracted schema names (non-empty for qualified names)
	PrivilegesShape string   // AccessPriv nodes with/without priv_name
	PrivilegeNames  []string // named privileges (SELECT, INSERT, etc.)
	AllPrivileges   bool     // true if AccessPriv with empty priv_name present
	GranteesShape   string   // RoleSpec nodes
	GranteeNames    []string // extracted grantee role names
	GrantOption     bool
	Behavior        string // CASCADE, RESTRICT, or empty
	// GrantRoleStmt fields
	GrantedRolesShape string
	GrantedRoleNames  []string
	// AlterDefaultPrivilegesStmt fields
	ADPOptionsShape string
	ADPOptions      []string // option DefElem names (e.g., "roles", "schemas")
	ADPAction       string   // nested GrantStmt summary
}

// tablePrivilegeDCLBaselineFact captures how the current DeltaScope pipeline
// handles each table privilege DCL candidate.
type tablePrivilegeDCLBaselineFact struct {
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

var pgTablePrivilegeDCLCensusCases = []struct {
	Name string
	SQL  string
}{
	{Name: "grant_select_on_table", SQL: "GRANT SELECT ON TABLE users TO analyst"},
	{Name: "grant_select_insert_on_table", SQL: "GRANT SELECT, INSERT ON TABLE users TO analyst, app_user"},
	{Name: "grant_all_on_table", SQL: "GRANT ALL PRIVILEGES ON TABLE users TO analyst"},
	{Name: "grant_select_on_qualified_table", SQL: "GRANT SELECT ON TABLE public.users TO analyst"},
	{Name: "grant_select_all_tables_in_schema", SQL: "GRANT SELECT ON ALL TABLES IN SCHEMA public TO analyst"},
	{Name: "grant_select_on_sequence", SQL: "GRANT SELECT ON SEQUENCE user_id_seq TO analyst"},
	{Name: "revoke_select_on_table", SQL: "REVOKE SELECT ON TABLE users FROM analyst"},
	{Name: "revoke_select_insert_on_table", SQL: "REVOKE SELECT, INSERT ON TABLE users FROM analyst, app_user"},
	{Name: "revoke_all_on_table_cascade", SQL: "REVOKE ALL PRIVILEGES ON TABLE users FROM analyst CASCADE"},
	{Name: "grant_role_to_role", SQL: "GRANT analyst TO app_user"},
	{Name: "revoke_role_from_role", SQL: "REVOKE analyst FROM app_user"},
	{Name: "alter_default_privileges_grant", SQL: "ALTER DEFAULT PRIVILEGES GRANT SELECT ON TABLES TO analyst"},
}

// TestTablePrivilegeDCLASTCensus inspects raw pg_query_go AST facts for all
// PostgreSQL table privilege DCL candidates. This is a read-only
// characterization test — no production code is modified.
func TestTablePrivilegeDCLASTCensus(t *testing.T) {
	t.Log("")
	t.Log("=== PostgreSQL Table Privilege DCL AST Census ===")
	t.Logf("%-42s | %-30s | %s", "Case", "Node Kind", "AST Facts")
	t.Log(string(make([]byte, 160)))

	for _, tc := range pgTablePrivilegeDCLCensusCases {
		tree, err := pg_query.Parse(tc.SQL)
		if err != nil {
			t.Fatalf("%s: parse failed: %v", tc.Name, err)
		}
		if len(tree.Stmts) != 1 {
			t.Fatalf("%s: expected 1 statement, got %d", tc.Name, len(tree.Stmts))
		}

		node := tree.Stmts[0].Stmt
		fact := inspectTablePrivilegeDCLAST(t, tc.Name, tc.SQL, node)
		assertTablePrivilegeDCLASTFacts(t, fact)
	}
}

func inspectTablePrivilegeDCLAST(t *testing.T, name, sql string, node *pg_query.Node) tablePrivilegeDCLASTFact {
	t.Helper()
	fact := tablePrivilegeDCLASTFact{Name: name, SQL: sql}

	switch n := node.GetNode().(type) {
	case *pg_query.Node_GrantStmt:
		stmt := n.GrantStmt
		fact.TopKind = "GrantStmt"

		if stmt.GetIsGrant() {
			fact.GrantMode = "grant"
		} else {
			fact.GrantMode = "revoke"
		}

		fact.TargetType = stmt.GetTargtype().String()
		fact.ObjType = stmt.GetObjtype().String()

		// Objects: for TABLE targets these are RangeVar nodes; for
		// ALL TABLES IN SCHEMA they may differ.
		fact.ObjectsShape, fact.ObjectNames, fact.ObjectSchemas = describeGrantObjects(stmt.GetObjects())

		// Privileges: AccessPriv nodes (priv_name="" means ALL PRIVILEGES).
		fact.PrivilegesShape, fact.PrivilegeNames, fact.AllPrivileges = describeAccessPrivs(stmt.GetPrivileges())

		// Grantees: RoleSpec nodes.
		fact.GranteesShape, fact.GranteeNames = describeRoleSpecs(stmt.GetGrantees())

		fact.GrantOption = stmt.GetGrantOption()
		fact.Behavior = stmt.GetBehavior().String()

		t.Logf("%-42s | %-30s | mode=%s targtype=%s objtype=%s obj_shape=%s objs=%v schemas=%v priv_shape=%s privs=%v all=%v grantee_shape=%s grantees=%v grant_option=%v behavior=%s",
			name, fact.TopKind, fact.GrantMode, fact.TargetType, fact.ObjType,
			fact.ObjectsShape, fact.ObjectNames, fact.ObjectSchemas,
			fact.PrivilegesShape, fact.PrivilegeNames, fact.AllPrivileges,
			fact.GranteesShape, fact.GranteeNames,
			fact.GrantOption, fact.Behavior)

	case *pg_query.Node_GrantRoleStmt:
		stmt := n.GrantRoleStmt
		fact.TopKind = "GrantRoleStmt"

		if stmt.GetIsGrant() {
			fact.GrantMode = "grant"
		} else {
			fact.GrantMode = "revoke"
		}

		fact.GrantedRolesShape, fact.GrantedRoleNames = describeRoleSpecs(stmt.GetGrantedRoles())
		fact.GranteesShape, fact.GranteeNames = describeRoleSpecs(stmt.GetGranteeRoles())
		fact.Behavior = stmt.GetBehavior().String()

		t.Logf("%-42s | %-30s | mode=%s granted_shape=%s granted=%v grantee_shape=%s grantees=%v behavior=%s",
			name, fact.TopKind, fact.GrantMode,
			fact.GrantedRolesShape, fact.GrantedRoleNames,
			fact.GranteesShape, fact.GranteeNames,
			fact.Behavior)

	case *pg_query.Node_AlterDefaultPrivilegesStmt:
		stmt := n.AlterDefaultPrivilegesStmt
		fact.TopKind = "AlterDefaultPrivilegesStmt"

		// Options: DefElem nodes carrying role/schema filters.
		fact.ADPOptionsShape, fact.ADPOptions = describeADPOptions(stmt.GetOptions())

		// Action: nested GrantStmt.
		if action := stmt.GetAction(); action != nil {
			if action.GetIsGrant() {
				fact.GrantMode = "grant"
			} else {
				fact.GrantMode = "revoke"
			}
			fact.ObjType = action.GetObjtype().String()
			fact.TargetType = action.GetTargtype().String()
			fact.ADPAction = fmt.Sprintf("GrantStmt{is_grant=%v, objtype=%s, targtype=%s}",
				action.GetIsGrant(), action.GetObjtype().String(), action.GetTargtype().String())
		}

		t.Logf("%-42s | %-30s | opts_shape=%s opts=%v action=%s",
			name, fact.TopKind, fact.ADPOptionsShape, fact.ADPOptions, fact.ADPAction)

	default:
		t.Fatalf("%s: unexpected node type %T", name, node.GetNode())
	}

	return fact
}

// describeGrantObjects identifies the node structure of GrantStmt.Objects
// and extracts object names and schemas.
func describeGrantObjects(objects []*pg_query.Node) (shape string, names []string, schemas []string) {
	if len(objects) == 0 {
		return "empty", nil, nil
	}
	// Inspect the first object to determine shape.
	obj := objects[0]
	if rv := obj.GetRangeVar(); rv != nil {
		shape = "RangeVar"
		for _, o := range objects {
			if r := o.GetRangeVar(); r != nil {
				names = append(names, r.GetRelname())
				schemas = append(schemas, r.GetSchemaname())
			}
		}
		return shape, names, schemas
	}
	if list := obj.GetList(); list != nil {
		shape = "List→String"
		for _, o := range objects {
			if l := o.GetList(); l != nil {
				names = append(names, firstStringFromNodes(l.GetItems()))
			}
		}
		return shape, names, schemas
	}
	shape = fmt.Sprintf("%T", obj.GetNode())
	return shape, names, schemas
}

// describeAccessPrivs inspects AccessPriv nodes from GrantStmt.Privileges.
func describeAccessPrivs(nodes []*pg_query.Node) (shape string, names []string, allPrivs bool) {
	if len(nodes) == 0 {
		return "empty", nil, false
	}
	shape = "AccessPriv"
	for _, n := range nodes {
		ap := n.GetAccessPriv()
		if ap == nil {
			continue
		}
		pn := ap.GetPrivName()
		if pn == "" {
			allPrivs = true
		} else {
			names = append(names, pn)
		}
	}
	return shape, names, allPrivs
}

// describeRoleSpecs inspects RoleSpec and AccessPriv nodes from Grantees or
// GrantedRoles. PostgreSQL uses RoleSpec for grantees but AccessPriv for
// granted roles in GRANT role TO role.
func describeRoleSpecs(nodes []*pg_query.Node) (shape string, names []string) {
	if len(nodes) == 0 {
		return "empty", nil
	}
	// Detect predominant node type.
	hasRoleSpec := false
	hasAccessPriv := false
	for _, n := range nodes {
		if n.GetRoleSpec() != nil {
			hasRoleSpec = true
		}
		if n.GetAccessPriv() != nil {
			hasAccessPriv = true
		}
	}
	switch {
	case hasRoleSpec && !hasAccessPriv:
		shape = "RoleSpec"
	case hasAccessPriv && !hasRoleSpec:
		shape = "AccessPriv"
	case hasRoleSpec && hasAccessPriv:
		shape = "RoleSpec+AccessPriv"
	default:
		shape = "unknown"
	}
	for _, n := range nodes {
		if rs := n.GetRoleSpec(); rs != nil {
			names = append(names, rs.GetRolename())
		}
		if ap := n.GetAccessPriv(); ap != nil {
			names = append(names, ap.GetPrivName())
		}
	}
	return shape, names
}

// describeADPOptions inspects DefElem nodes from AlterDefaultPrivilegesStmt.Options.
func describeADPOptions(nodes []*pg_query.Node) (shape string, names []string) {
	if len(nodes) == 0 {
		return "empty", nil
	}
	shape = "DefElem"
	for _, n := range nodes {
		de := n.GetDefElem()
		if de == nil {
			continue
		}
		names = append(names, de.GetDefname())
	}
	return shape, names
}

// assertTablePrivilegeDCLASTFacts validates that all expected AST facts are
// present and stable for decision-making.
func assertTablePrivilegeDCLASTFacts(t *testing.T, fact tablePrivilegeDCLASTFact) {
	t.Helper()

	if fact.TopKind == "" {
		t.Errorf("%s: expected non-empty top kind", fact.Name)
	}
	if fact.GrantMode == "" {
		t.Errorf("%s: expected non-empty grant mode", fact.Name)
	}

	switch fact.TopKind {
	case "GrantStmt":
		if fact.ObjType == "" {
			t.Errorf("%s: expected non-empty objtype", fact.Name)
		}
		if fact.ObjectsShape == "" {
			t.Errorf("%s: expected non-empty objects shape", fact.Name)
		}
		if fact.PrivilegesShape == "" {
			t.Errorf("%s: expected non-empty privileges shape", fact.Name)
		}
		if fact.GranteesShape == "" {
			t.Errorf("%s: expected non-empty grantees shape", fact.Name)
		}
	case "GrantRoleStmt":
		if fact.GrantedRolesShape == "" {
			t.Errorf("%s: expected non-empty granted_roles shape", fact.Name)
		}
		if fact.GranteesShape == "" {
			t.Errorf("%s: expected non-empty grantee_roles shape", fact.Name)
		}
	case "AlterDefaultPrivilegesStmt":
		// Options may be empty; action should be present.
		if fact.ADPAction == "" {
			t.Errorf("%s: expected non-empty ADP action", fact.Name)
		}
	}
}

// TestTablePrivilegeDCLDeltaScopeBaseline characterizes how the current
// DeltaScope pipeline classifies and extracts table privilege DCL
// candidates. This is a read-only characterization test — no production
// code is modified.
func TestTablePrivilegeDCLDeltaScopeBaseline(t *testing.T) {
	t.Log("")
	t.Log("=== PostgreSQL Table Privilege DCL DeltaScope Baseline ===")
	t.Logf("%-42s | %-10s | %-5s | %-12s | %s",
		"Case", "Kind", "DDL?", "Unsupported?", "Detail")
	t.Log(string(make([]byte, 160)))

	for _, tc := range pgTablePrivilegeDCLCensusCases {
		p := New()
		result, parseErr := p.Parse(tc.SQL)
		if parseErr != nil {
			t.Logf("%-42s | %-10s | %-5s | %-12s | parse error: %v",
				tc.Name, "ERROR", "-", "-", parseErr)
			t.Errorf("%s: unexpected parse error: %v", tc.Name, parseErr)
			continue
		}
		if len(result.Statements) != 1 {
			t.Fatalf("%s: expected 1 statement, got %d", tc.Name, len(result.Statements))
		}

		es := result.Statements[0]
		classifies := es.Kind == spec.KindDDL

		baseline := tablePrivilegeDCLBaselineFact{
			Name:       tc.Name,
			Kind:       es.Kind,
			Classifies: classifies,
		}

		stmt, extractErr := es.Extractor.Extract(spec.DialectPostgreSQL, es.RawSQL)
		if extractErr != nil {
			t.Logf("%-42s | %-10s | %-5v | %-12s | extract error: %v",
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

		t.Logf("%-42s | %-10s | %-5s | %-12s | %s",
			tc.Name, baseline.Kind, classStr, unsupported, detail)

		assertTablePrivilegeDCLBaseline(t, baseline)
	}
}

// assertTablePrivilegeDCLBaseline captures post-Task-2 expectations.
//
// Post-Task-2 state:
//   - Ordinary table GRANT forms: KindDDL, normalized as grant_table
//   - Ordinary table REVOKE forms: KindDDL, normalized as revoke_table
//   - ALL TABLES IN SCHEMA: KindDDL, unsupported with feature grant_all_tables_in_schema
//   - Sequence privileges: KindDDL, unsupported with feature grant_table
//   - Role membership: KindDDL, unsupported with feature grant_role
//   - ALTER DEFAULT PRIVILEGES: KindDDL, unsupported with feature alter_default_privileges
func assertTablePrivilegeDCLBaseline(t *testing.T, fact tablePrivilegeDCLBaselineFact) {
	t.Helper()

	// All forms now classify as DDL.
	if fact.Kind != spec.KindDDL {
		t.Errorf("%s: expected KindDDL, got %v", fact.Name, fact.Kind)
	}

	switch fact.Name {
	// Supported: normalized table privilege forms.
	case "grant_select_on_table":
		if fact.Unsupported {
			t.Errorf("%s: expected normalized, got unsupported %s: %s", fact.Name, fact.UnsupportedFeature, fact.UnsupportedReason)
		}
		if fact.DDLOperation != "grant_table" {
			t.Errorf("%s: expected grant_table, got %q", fact.Name, fact.DDLOperation)
		}
		if fact.DDLObjectName != "users" {
			t.Errorf("%s: expected object_name users, got %q", fact.Name, fact.DDLObjectName)
		}
		if fact.DDLObjectType != "table" {
			t.Errorf("%s: expected object_type table, got %q", fact.Name, fact.DDLObjectType)
		}
		if fact.DDLOptions["privileges"] != "select" {
			t.Errorf("%s: expected privileges=select, got %v", fact.Name, fact.DDLOptions)
		}
		if fact.DDLOptions["grantees"] != "analyst" {
			t.Errorf("%s: expected grantees=analyst, got %v", fact.Name, fact.DDLOptions)
		}

	case "grant_select_insert_on_table":
		if fact.Unsupported {
			t.Errorf("%s: expected normalized, got unsupported", fact.Name)
		}
		if fact.DDLOperation != "grant_table" {
			t.Errorf("%s: expected grant_table, got %q", fact.Name, fact.DDLOperation)
		}
		if fact.DDLOptions["privileges"] != "select,insert" {
			t.Errorf("%s: expected privileges=select,insert, got %v", fact.Name, fact.DDLOptions)
		}
		if fact.DDLOptions["grantees"] != "analyst,app_user" {
			t.Errorf("%s: expected grantees=analyst,app_user, got %v", fact.Name, fact.DDLOptions)
		}

	case "grant_all_on_table":
		if fact.Unsupported {
			t.Errorf("%s: expected normalized, got unsupported", fact.Name)
		}
		if fact.DDLOperation != "grant_table" {
			t.Errorf("%s: expected grant_table, got %q", fact.Name, fact.DDLOperation)
		}
		if fact.DDLOptions["all_privileges"] != "true" {
			t.Errorf("%s: expected all_privileges=true, got %v", fact.Name, fact.DDLOptions)
		}

	case "grant_select_on_qualified_table":
		if fact.Unsupported {
			t.Errorf("%s: expected normalized, got unsupported", fact.Name)
		}
		if fact.DDLOperation != "grant_table" {
			t.Errorf("%s: expected grant_table, got %q", fact.Name, fact.DDLOperation)
		}
		if fact.DDLOptions["schema"] != "public" {
			t.Errorf("%s: expected schema=public, got %v", fact.Name, fact.DDLOptions)
		}
		if fact.DDLObjectName != "users" {
			t.Errorf("%s: expected object_name users, got %q", fact.Name, fact.DDLObjectName)
		}

	case "revoke_select_on_table":
		if fact.Unsupported {
			t.Errorf("%s: expected normalized, got unsupported", fact.Name)
		}
		if fact.DDLOperation != "revoke_table" {
			t.Errorf("%s: expected revoke_table, got %q", fact.Name, fact.DDLOperation)
		}
		if fact.DDLOptions["privileges"] != "select" {
			t.Errorf("%s: expected privileges=select, got %v", fact.Name, fact.DDLOptions)
		}

	case "revoke_select_insert_on_table":
		if fact.Unsupported {
			t.Errorf("%s: expected normalized, got unsupported", fact.Name)
		}
		if fact.DDLOperation != "revoke_table" {
			t.Errorf("%s: expected revoke_table, got %q", fact.Name, fact.DDLOperation)
		}
		if fact.DDLOptions["privileges"] != "select,insert" {
			t.Errorf("%s: expected privileges=select,insert, got %v", fact.Name, fact.DDLOptions)
		}
		if fact.DDLOptions["grantees"] != "analyst,app_user" {
			t.Errorf("%s: expected grantees=analyst,app_user, got %v", fact.Name, fact.DDLOptions)
		}

	case "revoke_all_on_table_cascade":
		if fact.Unsupported {
			t.Errorf("%s: expected normalized, got unsupported", fact.Name)
		}
		if fact.DDLOperation != "revoke_table" {
			t.Errorf("%s: expected revoke_table, got %q", fact.Name, fact.DDLOperation)
		}
		if fact.DDLOptions["all_privileges"] != "true" {
			t.Errorf("%s: expected all_privileges=true, got %v", fact.Name, fact.DDLOptions)
		}
		if fact.DDLOptions["cascade"] != "true" {
			t.Errorf("%s: expected cascade=true, got %v", fact.Name, fact.DDLOptions)
		}

	// Deferred: broader scopes with stable feature names.
	case "grant_select_all_tables_in_schema":
		if !fact.Unsupported {
			t.Errorf("%s: expected unsupported", fact.Name)
		}
		if fact.UnsupportedFeature != "grant_all_tables_in_schema" {
			t.Errorf("%s: expected feature 'grant_all_tables_in_schema', got %q", fact.Name, fact.UnsupportedFeature)
		}

	case "grant_select_on_sequence":
		if !fact.Unsupported {
			t.Errorf("%s: expected unsupported", fact.Name)
		}
		if fact.UnsupportedFeature != "grant_table" {
			t.Errorf("%s: expected feature 'grant_table', got %q", fact.Name, fact.UnsupportedFeature)
		}

	case "grant_role_to_role", "revoke_role_from_role":
		if !fact.Unsupported {
			t.Errorf("%s: expected unsupported", fact.Name)
		}
		if fact.UnsupportedFeature != "grant_role" {
			t.Errorf("%s: expected feature 'grant_role', got %q", fact.Name, fact.UnsupportedFeature)
		}

	case "alter_default_privileges_grant":
		if !fact.Unsupported {
			t.Errorf("%s: expected unsupported", fact.Name)
		}
		if fact.UnsupportedFeature != "alter_default_privileges" {
			t.Errorf("%s: expected feature 'alter_default_privileges', got %q", fact.Name, fact.UnsupportedFeature)
		}
	}
}

// TestTablePrivilegeDCLRuleCoverageBaseline verifies the normalized state
// of table privilege DCL statements for rule applicability. Supported forms
// produce DDL suitable for future rules; deferred forms remain unsupported.
// This is a read-only characterization test — no rules fire yet.
func TestTablePrivilegeDCLRuleCoverageBaseline(t *testing.T) {
	t.Log("")
	t.Log("=== PostgreSQL Table Privilege DCL Rule Coverage Baseline ===")

	for _, tc := range pgTablePrivilegeDCLCensusCases {
		p := New()
		result, parseErr := p.Parse(tc.SQL)
		if parseErr != nil {
			t.Fatalf("%s: parse failed: %v", tc.Name, parseErr)
		}
		if len(result.Statements) != 1 {
			t.Fatalf("%s: expected 1 statement, got %d", tc.Name, len(result.Statements))
		}

		es := result.Statements[0]
		stmt, extractErr := es.Extractor.Extract(spec.DialectPostgreSQL, es.RawSQL)
		if extractErr != nil {
			t.Fatalf("%s: extract failed: %v", tc.Name, extractErr)
		}

		hasDDL := stmt.DDL != nil
		hasDML := stmt.DML != nil
		isUnsupported := stmt.Unsupported != nil

		t.Logf("%-42s | unsupported=%v | has_ddl=%v | has_dml=%v",
			tc.Name, isUnsupported, hasDDL, hasDML)

		switch tc.Name {
		case "grant_select_on_table", "grant_select_insert_on_table",
			"grant_all_on_table", "grant_select_on_qualified_table",
			"revoke_select_on_table", "revoke_select_insert_on_table",
			"revoke_all_on_table_cascade":
			// Supported: produces normalized DDL, no unsupported marker.
			if !hasDDL {
				t.Errorf("%s: expected normalized DDL", tc.Name)
			}
			if hasDML {
				t.Errorf("%s: unexpected DML", tc.Name)
			}
			if isUnsupported {
				t.Errorf("%s: unexpected unsupported marker", tc.Name)
			}

		default:
			// Deferred: no normalized DDL/DML, has unsupported marker.
			if hasDDL || hasDML {
				t.Errorf("%s: expected no normalized DDL/DML, got ddl=%v dml=%v",
					tc.Name, hasDDL, hasDML)
			}
			if !isUnsupported {
				t.Errorf("%s: expected unsupported marker", tc.Name)
			}
		}
	}
}

var _ = fmt.Sprintf
