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
	Name                string
	SQL                 string
	TopKind             string   // GrantStmt, GrantRoleStmt, AlterDefaultPrivilegesStmt
	GrantMode           string   // "grant" or "revoke" (from GrantStmt.IsGrant)
	TargetType          string   // ACL_TARGET_OBJECT, ACL_TARGET_ALL_IN_SCHEMA, etc.
	ObjType             string   // OBJECT_TABLE, OBJECT_SEQUENCE, etc.
	ObjectsShape        string   // how objects are represented: RangeVar, List→String, etc.
	ObjectNames         []string // extracted object names
	ObjectSchemas       []string // extracted schema names (non-empty for qualified names)
	PrivilegesShape     string   // AccessPriv nodes with/without priv_name
	PrivilegeNames      []string // named privileges (SELECT, INSERT, etc.)
	AllPrivileges       bool     // true if AccessPriv with empty priv_name present
	GranteesShape       string   // RoleSpec nodes
	GranteeNames        []string // extracted grantee role names
	GrantOption         bool
	Behavior            string // CASCADE, RESTRICT, or empty
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

// assertTablePrivilegeDCLBaseline captures Task-1 baseline expectations.
//
// Current state (pre-Task-2):
//   - All GRANT/REVOKE table privilege forms: KindUnknown, unsupported via
//     "postgresql statement type is not in the approved v1 subset"
//   - GRANT/REVOKE role membership: same unsupported path
//   - ALTER DEFAULT PRIVILEGES: same unsupported path
func assertTablePrivilegeDCLBaseline(t *testing.T, fact tablePrivilegeDCLBaselineFact) {
	t.Helper()

	// All forms should currently be unsupported (KindUnknown) because the
	// classify() function and Extract() switch have no GrantStmt,
	// GrantRoleStmt, or AlterDefaultPrivilegesStmt branches.
	if fact.Kind != spec.KindUnknown {
		t.Errorf("%s: expected KindUnknown, got %v", fact.Name, fact.Kind)
	}
	if !fact.Unsupported {
		t.Errorf("%s: expected unsupported, got normalized (op=%s)", fact.Name, fact.DDLOperation)
	}
	if fact.UnsupportedReason == "" {
		t.Errorf("%s: expected non-empty unsupported reason", fact.Name)
	}
}

// TestTablePrivilegeDCLRuleCoverageBaseline verifies that no existing rules
// fire on table privilege DCL statements. Since rules operate on normalized
// DDL/DML statements and all privilege DCL forms are currently unsupported,
// no rule evaluation is possible. This is a read-only characterization test.
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

		// All forms are unsupported, so no rules can fire. Confirm that
		// the statement has no normalized DDL (the precondition for rules).
		hasDDL := stmt.DDL != nil
		hasDML := stmt.DML != nil
		isUnsupported := stmt.Unsupported != nil

		t.Logf("%-42s | unsupported=%v | has_ddl=%v | has_dml=%v",
			tc.Name, isUnsupported, hasDDL, hasDML)

		if hasDDL || hasDML {
			t.Errorf("%s: expected no normalized DDL/DML, got ddl=%v dml=%v",
				tc.Name, hasDDL, hasDML)
		}
	}
}

var _ = fmt.Sprintf
