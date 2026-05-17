//go:build postgresql

package postgresql

import (
	"strings"
	"testing"

	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// nodeStrings extracts the String values from a list of AST Nodes.
// Used to read identity and payload name fields from the pg_query AST.
func nodeStrings(nodes []*pg_query.Node) []string {
	out := make([]string, 0, len(nodes))
	for _, n := range nodes {
		if s := n.GetString_(); s != nil && s.GetSval() != "" {
			out = append(out, s.GetSval())
		}
	}
	return out
}

// TestCreateTransformASTCensus characterizes pg_query_go AST facts for
// CREATE TRANSFORM to determine whether bounded identity (type@language)
// can be safely extracted without leaking FROM/TO function names.
func TestCreateTransformASTCensus(t *testing.T) {
	t.Parallel()

	tree, err := pg_query.Parse(
		"CREATE TRANSFORM FOR jsonb LANGUAGE plpython3u " +
			"(FROM SQL WITH FUNCTION jsonb_to_plpython(jsonb), " +
			"TO SQL WITH FUNCTION plpython_to_jsonb(internal))")
	if err != nil {
		t.Fatalf("parse create transform: %v", err)
	}
	if len(tree.Stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(tree.Stmts))
	}

	node := tree.Stmts[0].Stmt
	transformStmt, ok := node.GetNode().(*pg_query.Node_CreateTransformStmt)
	if !ok {
		t.Fatalf("expected CreateTransformStmt node, got %T", node.GetNode())
	}
	stmt := transformStmt.CreateTransformStmt

	// --- Object identity fields ---
	var typeNameStr string
	var typeNameParts []string
	if stmt.GetTypeName() != nil {
		typeNameParts = nodeStrings(stmt.GetTypeName().GetNames())
		if len(typeNameParts) > 0 {
			typeNameStr = typeNameParts[len(typeNameParts)-1]
		}
	}
	lang := stmt.GetLang()
	replace := stmt.GetReplace()

	t.Logf("CREATE TRANSFORM AST node: CreateTransformStmt")
	t.Logf("  type_name.names: %v (last segment: %q)", typeNameParts, typeNameStr)
	t.Logf("  lang: %q", lang)
	t.Logf("  replace: %v", replace)

	// --- Handler/function payload fields ---
	var fromsqlNames []string
	if stmt.GetFromsql() != nil {
		fromsqlNames = nodeStrings(stmt.GetFromsql().GetObjname())
	}
	var tosqlNames []string
	if stmt.GetTosql() != nil {
		tosqlNames = nodeStrings(stmt.GetTosql().GetObjname())
	}

	t.Logf("  fromsql.objname (function payload): %v", fromsqlNames)
	t.Logf("  tosql.objname (function payload): %v", tosqlNames)

	// --- Assert identity fields are present and stable ---
	if typeNameStr == "" {
		t.Fatal("type_name is empty — cannot form bounded identity")
	}
	if lang == "" {
		t.Fatal("lang is empty — cannot form bounded identity")
	}

	// --- Assert identity matches expected values ---
	if typeNameStr != "jsonb" {
		t.Errorf("expected type_name last segment %q, got %q", "jsonb", typeNameStr)
	}
	if lang != "plpython3u" {
		t.Errorf("expected lang %q, got %q", "plpython3u", lang)
	}

	// --- Assert payload fields are present but separable ---
	identityOK := typeNameStr != "" && lang != ""
	payloadPresent := len(fromsqlNames) > 0 || len(tosqlNames) > 0
	payloadLeakedInIdentity := strings.Contains(typeNameStr, "jsonb_to_plpython") ||
		strings.Contains(typeNameStr, "plpython_to_jsonb") ||
		strings.Contains(lang, "jsonb_to_plpython") ||
		strings.Contains(lang, "plpython_to_jsonb")

	t.Logf("")
	t.Logf("IDENTITY ASSESSMENT:")
	t.Logf("  bounded identity (type@language): %q@%q", typeNameStr, lang)
	t.Logf("  identity fields present: %v", identityOK)
	t.Logf("  function payload present: %v (fromsql=%v, tosql=%v)", payloadPresent, fromsqlNames, tosqlNames)
	t.Logf("  identity separable from payload: %v", identityOK && payloadPresent && !payloadLeakedInIdentity)

	if payloadLeakedInIdentity {
		t.Error("FATAL: function payload leaked into identity fields")
	}
}

// TestCreateAccessMethodASTCensus characterizes pg_query_go AST facts for
// CREATE ACCESS METHOD to determine whether bounded identity (access method name)
// can be safely extracted without leaking handler function names.
func TestCreateAccessMethodASTCensus(t *testing.T) {
	t.Parallel()

	tree, err := pg_query.Parse(
		"CREATE ACCESS METHOD heap2 TYPE TABLE HANDLER heap_tableam_handler")
	if err != nil {
		t.Fatalf("parse create access method: %v", err)
	}
	if len(tree.Stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(tree.Stmts))
	}

	node := tree.Stmts[0].Stmt
	amStmt, ok := node.GetNode().(*pg_query.Node_CreateAmStmt)
	if !ok {
		t.Fatalf("expected CreateAmStmt node, got %T", node.GetNode())
	}
	stmt := amStmt.CreateAmStmt

	// --- Object identity fields ---
	amName := stmt.GetAmname()
	amType := stmt.GetAmtype()

	t.Logf("CREATE ACCESS METHOD AST node: CreateAmStmt")
	t.Logf("  amname: %q", amName)
	t.Logf("  amtype: %q", amType)

	// --- Handler function payload field ---
	handlerNames := nodeStrings(stmt.GetHandlerName())
	t.Logf("  handler_name (function payload): %v", handlerNames)

	// --- Assert identity fields are present and stable ---
	if amName == "" {
		t.Fatal("amname is empty — cannot form bounded identity")
	}

	// --- Assert identity matches expected values ---
	if amName != "heap2" {
		t.Errorf("expected amname %q, got %q", "heap2", amName)
	}

	// amtype should indicate TABLE or INDEX
	t.Logf("  amtype interpretation: %q (expected 't'=TABLE or 'i'=INDEX from PG internals)", amType)

	// --- Assert payload present but separable ---
	identityOK := amName != ""
	payloadPresent := len(handlerNames) > 0
	payloadLeakedInIdentity := strings.Contains(amName, "heap_tableam_handler")

	t.Logf("")
	t.Logf("IDENTITY ASSESSMENT:")
	t.Logf("  bounded identity (name): %q", amName)
	t.Logf("  optional type hint: %q", amType)
	t.Logf("  identity fields present: %v", identityOK)
	t.Logf("  handler payload present: %v (names=%v)", payloadPresent, handlerNames)
	t.Logf("  identity separable from payload: %v", identityOK && payloadPresent && !payloadLeakedInIdentity)

	if payloadLeakedInIdentity {
		t.Error("FATAL: handler payload leaked into identity fields")
	}
}
