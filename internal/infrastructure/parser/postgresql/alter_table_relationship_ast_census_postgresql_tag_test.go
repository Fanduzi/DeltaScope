//go:build postgresql

package postgresql

import (
	"fmt"
	"strings"
	"testing"

	pg_query "github.com/pganalyze/pg_query_go/v6"
)

var pgAlterTableRelationshipCensusCases = []struct {
	name string
	sql  string
}{
	{name: "inherit", sql: "ALTER TABLE child_users INHERIT users"},
	{name: "no_inherit", sql: "ALTER TABLE child_users NO INHERIT users"},
	{name: "of_type", sql: "ALTER TABLE users OF user_type"},
	{name: "not_of", sql: "ALTER TABLE users NOT OF"},
}

type relationshipASTFact struct {
	Name         string
	SQL          string
	TopKind      string
	Table        string
	Subtype      string
	SubtypeValue int32
	CmdName      string
	HasDef       bool
	DefKind      string
	PayloadNote  string
}

func relationshipASTFactsForCase(t *testing.T, name, sql string) relationshipASTFact {
	t.Helper()

	tree, err := pg_query.Parse(sql)
	if err != nil {
		t.Fatalf("pg_query.Parse(%q): %v", name, err)
	}
	if len(tree.GetStmts()) != 1 {
		t.Fatalf("%s: expected 1 statement, got %d", name, len(tree.GetStmts()))
	}

	node := tree.GetStmts()[0].GetStmt()
	if node == nil {
		t.Fatalf("%s: nil statement node", name)
	}

	fact := relationshipASTFact{
		Name: name,
		SQL:  sql,
	}

	atNode, ok := node.GetNode().(*pg_query.Node_AlterTableStmt)
	if !ok {
		fact.TopKind = strings.TrimPrefix(fmt.Sprintf("%T", node.GetNode()), "*pg_query.Node_")
		return fact
	}
	fact.TopKind = "AlterTableStmt"

	atStmt := atNode.AlterTableStmt
	if atStmt == nil {
		t.Fatalf("%s: AlterTableStmt is nil", name)
	}

	if rv := atStmt.GetRelation(); rv != nil {
		fact.Table = rv.GetRelname()
	}

	cmds := atStmt.GetCmds()
	if len(cmds) == 0 {
		t.Fatalf("%s: expected at least 1 AlterTableCmd, got 0", name)
	}

	cmd := cmds[0].GetAlterTableCmd()
	if cmd == nil {
		t.Fatalf("%s: first cmd is not an AlterTableCmd", name)
	}

	fact.Subtype = cmd.GetSubtype().String()
	fact.SubtypeValue = int32(cmd.GetSubtype())
	fact.CmdName = cmd.GetName()
	fact.HasDef = cmd.GetDef() != nil

	if fact.HasDef {
		defNode := cmd.GetDef()
		fact.DefKind = strings.TrimPrefix(fmt.Sprintf("%T", defNode.GetNode()), "*pg_query.Node_")
	}

	return fact
}

func TestAlterTableRelationshipASTCensus(t *testing.T) {
	t.Parallel()
	t.Log("")
	t.Log("=== PostgreSQL ALTER TABLE Table Relationship Candidates AST Census ===")
	t.Log("")
	t.Logf("%-12s | %-15s | %-12s | %-20s | %-3s | %-12s | %-6s | %-12s | %s",
		"Name", "TopKind", "Table", "Subtype", "Val", "CmdName", "HasDef", "DefKind", "PayloadNote")
	t.Log(strings.Repeat("-", 160))

	for _, tc := range pgAlterTableRelationshipCensusCases {
		fact := relationshipASTFactsForCase(t, tc.name, tc.sql)

		t.Logf("%-12s | %-15s | %-12s | %-20s | %-3d | %-12s | %-6v | %-12s | %s",
			fact.Name, fact.TopKind, fact.Table, fact.Subtype, fact.SubtypeValue,
			fact.CmdName, fact.HasDef, fact.DefKind, fact.PayloadNote)
	}
}

func TestAlterTableRelationshipStableShapeAssertions(t *testing.T) {
	t.Parallel()

	expectedShapes := map[string]struct {
		topKind string
		table   string
		subtype string
	}{
		"inherit":    {topKind: "AlterTableStmt", table: "child_users", subtype: "AT_AddInherit"},
		"no_inherit": {topKind: "AlterTableStmt", table: "child_users", subtype: "AT_DropInherit"},
		"of_type":    {topKind: "AlterTableStmt", table: "users", subtype: "AT_AddOf"},
		"not_of":     {topKind: "AlterTableStmt", table: "users", subtype: "AT_DropOf"},
	}

	for _, tc := range pgAlterTableRelationshipCensusCases {
		fact := relationshipASTFactsForCase(t, tc.name, tc.sql)

		exp, ok := expectedShapes[tc.name]
		if !ok {
			t.Errorf("no expected shape for case %q", tc.name)
			continue
		}
		if fact.TopKind != exp.topKind {
			t.Errorf("case %q: topKind = %q, want %q", tc.name, fact.TopKind, exp.topKind)
		}
		if fact.Subtype != exp.subtype {
			t.Errorf("case %q: subtype = %q, want %q", tc.name, fact.Subtype, exp.subtype)
		}
		if fact.Table != exp.table {
			t.Errorf("case %q: table = %q, want %q", tc.name, fact.Table, exp.table)
		}
	}
}

func TestAlterTableRelationshipDefPayloadDetail(t *testing.T) {
	t.Parallel()
	t.Log("")
	t.Log("=== Def Payload Detail for Table Relationship Candidates ===")
	t.Log("")

	for _, tc := range pgAlterTableRelationshipCensusCases {
		tree, err := pg_query.Parse(tc.sql)
		if err != nil {
			t.Fatalf("%s: parse failed: %v", tc.name, err)
		}

		atStmt := tree.GetStmts()[0].GetStmt().GetNode().(*pg_query.Node_AlterTableStmt).AlterTableStmt
		cmd := atStmt.GetCmds()[0].GetAlterTableCmd()

		subtype := cmd.GetSubtype().String()
		cmdName := cmd.GetName()
		hasDef := cmd.GetDef() != nil

		t.Logf("--- %s ---", tc.name)
		t.Logf("  subtype=%s | cmdName=%q | hasDef=%v", subtype, cmdName, hasDef)

		if hasDef {
			defNode := cmd.GetDef()
			defKind := strings.TrimPrefix(fmt.Sprintf("%T", defNode.GetNode()), "*pg_query.Node_")
			t.Logf("  defKind=%s", defKind)

			switch dn := defNode.GetNode().(type) {
			case *pg_query.Node_RangeVar:
				rv := dn.RangeVar
				t.Logf("  def.RangeVar.catalog=%q | schema=%q | relname=%q | inh=%v",
					rv.GetCatalogname(), rv.GetSchemaname(), rv.GetRelname(), rv.GetInh())
			case *pg_query.Node_TypeName:
				names := dn.TypeName.GetNames()
				var nameStrs []string
				for _, n := range names {
					if s := n.GetString_(); s != nil {
						nameStrs = append(nameStrs, s.GetSval())
					}
				}
				t.Logf("  def.TypeName.names=%v", nameStrs)
			default:
				t.Logf("  def (other): %s", defNode.String())
			}
		}

		t.Logf("  cmd proto: %s", cmd.String())
		t.Log("")
	}
}

func TestAlterTableRelationshipIdentityFieldMapping(t *testing.T) {
	t.Parallel()
	t.Log("")
	t.Log("=== Identity Field Mapping for Table Relationship Candidates ===")
	t.Log("")

	type identityExpectation struct {
		parentTable string
		typeName    string
		hasDef      bool
	}

	expectations := map[string]identityExpectation{
		"inherit":    {parentTable: "users", hasDef: true},
		"no_inherit": {parentTable: "users", hasDef: true},
		"of_type":    {typeName: "user_type", hasDef: true},
		"not_of":     {hasDef: false},
	}

	for _, tc := range pgAlterTableRelationshipCensusCases {
		tree, err := pg_query.Parse(tc.sql)
		if err != nil {
			t.Fatalf("%s: parse failed: %v", tc.name, err)
		}

		atStmt := tree.GetStmts()[0].GetStmt().GetNode().(*pg_query.Node_AlterTableStmt).AlterTableStmt
		cmd := atStmt.GetCmds()[0].GetAlterTableCmd()

		exp, ok := expectations[tc.name]
		if !ok {
			t.Errorf("no identity expectation for %q", tc.name)
			continue
		}

		hasDef := cmd.GetDef() != nil
		if hasDef != exp.hasDef {
			t.Errorf("%s: hasDef=%v, want %v", tc.name, hasDef, exp.hasDef)
		}

		parentOK := true
		if exp.parentTable != "" && hasDef {
			if rvNode, ok := cmd.GetDef().GetNode().(*pg_query.Node_RangeVar); ok {
				parentName := rvNode.RangeVar.GetRelname()
				if parentName != exp.parentTable {
					parentOK = false
					t.Errorf("%s: parent table=%q, want %q", tc.name, parentName, exp.parentTable)
				}
				t.Logf("  %s: parent_table from def.RangeVar.relname=%q (OK=%v)", tc.name, parentName, parentOK)
			} else {
				parentOK = false
				t.Errorf("%s: expected def to be RangeVar, got %T", tc.name, cmd.GetDef().GetNode())
			}
		}

		typeOK := true
		if exp.typeName != "" && hasDef {
			if tnNode, ok := cmd.GetDef().GetNode().(*pg_query.Node_TypeName); ok {
				names := tnNode.TypeName.GetNames()
				var nameStrs []string
				for _, n := range names {
					if s := n.GetString_(); s != nil {
						nameStrs = append(nameStrs, s.GetSval())
					}
				}
				got := strings.Join(nameStrs, ".")
				if got != exp.typeName {
					typeOK = false
					t.Errorf("%s: type name=%q, want %q", tc.name, got, exp.typeName)
				}
				t.Logf("  %s: type from def.TypeName.names=%v (OK=%v)", tc.name, nameStrs, typeOK)
			} else {
				typeOK = false
				t.Errorf("%s: expected def to be TypeName, got %T", tc.name, cmd.GetDef().GetNode())
			}
		}

		if tc.name == "not_of" {
			t.Logf("  %s: table-only action (NOT OF), no payload (OK=%v)", tc.name, !hasDef)
		}

		t.Logf("%-12s | hasDef=%v | identity_OK=%v",
			tc.name, hasDef,
			hasDef == exp.hasDef && parentOK && typeOK)
	}
}
