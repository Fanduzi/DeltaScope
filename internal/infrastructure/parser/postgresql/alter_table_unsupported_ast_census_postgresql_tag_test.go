//go:build postgresql

package postgresql

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

var alterTableUnsupportedASTCases = []struct {
	name string
	sql  string
}{
	{name: "set_schema", sql: "ALTER TABLE users SET SCHEMA archive"},
	{name: "owner_to", sql: "ALTER TABLE users OWNER TO app_owner"},
	{name: "enable_trigger_named", sql: "ALTER TABLE users ENABLE TRIGGER trg_users_audit"},
	{name: "disable_trigger_named", sql: "ALTER TABLE users DISABLE TRIGGER trg_users_audit"},
	{name: "attach_partition", sql: "ALTER TABLE measurement ATTACH PARTITION measurement_y2026m04 FOR VALUES FROM ('2026-04-01') TO ('2026-05-01')"},
	{name: "detach_partition", sql: "ALTER TABLE measurement DETACH PARTITION measurement_y2026m04"},
}

// alterUnsupportedNodeFacts captures the AST shape for one unsupported ALTER TABLE form.
type alterUnsupportedNodeFacts struct {
	Name            string
	SQL             string
	TopLevelKind    string // e.g. "AlterObjectSchemaStmt" or "AlterTableStmt"
	TargetTable     string
	TargetSchema    string
	AlterSubtype    string // AlterTableCmd subtype string (empty for AlterObjectSchemaStmt)
	TargetName      string // trigger name / partition table / role / new schema
	HasBoundPayload bool   // true for ATTACH PARTITION (has FOR VALUES)
}

// alterUnsupportedCurrentStatus captures how DeltaScope currently classifies a form.
type alterUnsupportedCurrentStatus struct {
	Name            string
	Unsupported     bool
	UnsupportedFeat string
	UnsupportedWhy  string
}

func alterUnsupportedNodeKind(tree *pg_query.ParseResult, idx int) string {
	stmts := tree.GetStmts()
	if idx >= len(stmts) {
		return ""
	}
	node := stmts[idx].GetStmt()
	if node == nil {
		return ""
	}
	switch node.GetNode().(type) {
	case *pg_query.Node_AlterTableStmt:
		return "AlterTableStmt"
	case *pg_query.Node_AlterObjectSchemaStmt:
		return "AlterObjectSchemaStmt"
	default:
		// Use proto message name as fallback
		return strings.TrimPrefix(fmt.Sprintf("%T", node.GetNode()), "*pg_query.Node_")
	}
}

func alterUnsupportedTargetName(stmt *pg_query.ParseResult, idx int) (tableName, tableSchema string) {
	stmts := stmt.GetStmts()
	if idx >= len(stmts) {
		return "", ""
	}
	node := stmts[idx].GetStmt()
	if node == nil {
		return "", ""
	}

	switch n := node.GetNode().(type) {
	case *pg_query.Node_AlterTableStmt:
		rv := n.AlterTableStmt.GetRelation()
		if rv != nil {
			tableName = rv.GetRelname()
			tableSchema = rv.GetSchemaname()
		}
	case *pg_query.Node_AlterObjectSchemaStmt:
		// ALTER TABLE … SET SCHEMA uses Relation (RangeVar), not Object.
		rv := n.AlterObjectSchemaStmt.GetRelation()
		if rv != nil {
			tableName = rv.GetRelname()
			tableSchema = rv.GetSchemaname()
		}
	}
	return tableName, tableSchema
}

func alterUnsupportedCmdFacts(tree *pg_query.ParseResult, idx int) (subtype string, targetName string, hasBounds bool) {
	stmts := tree.GetStmts()
	if idx >= len(stmts) {
		return "", "", false
	}
	node := stmts[idx].GetStmt()
	if node == nil {
		return "", "", false
	}

	alterTable, ok := node.GetNode().(*pg_query.Node_AlterTableStmt)
	if !ok {
		return "", "", false
	}

	cmds := alterTable.AlterTableStmt.GetCmds()
	if len(cmds) == 0 {
		return "", "", false
	}

	cmd := cmds[0].GetAlterTableCmd()
	if cmd == nil {
		return "", "", false
	}

	subtype = cmd.GetSubtype().String()
	targetName = cmd.GetName()
	hasBounds = cmd.GetDef() != nil

	return subtype, targetName, hasBounds
}

func alterUnsupportedNodeFactsForCase(t *testing.T, name, sql string) alterUnsupportedNodeFacts {
	t.Helper()
	tree, err := pg_query.Parse(sql)
	if err != nil {
		t.Fatalf("pg_query.Parse(%q): %v", name, err)
	}

	topKind := alterUnsupportedNodeKind(tree, 0)
	tableName, tableSchema := alterUnsupportedTargetName(tree, 0)
	subtype, targetName, hasBounds := alterUnsupportedCmdFacts(tree, 0)

	// For AlterObjectSchemaStmt, extract the new schema / target name from the node directly
	if topKind == "AlterObjectSchemaStmt" {
		stmts := tree.GetStmts()
		if len(stmts) > 0 && stmts[0].GetStmt() != nil {
			if aos, ok := stmts[0].GetStmt().GetNode().(*pg_query.Node_AlterObjectSchemaStmt); ok {
				targetName = aos.AlterObjectSchemaStmt.GetNewschema()
			}
		}
	}

	return alterUnsupportedNodeFacts{
		Name:            name,
		SQL:             sql,
		TopLevelKind:    topKind,
		TargetTable:     tableName,
		TargetSchema:    tableSchema,
		AlterSubtype:    subtype,
		TargetName:      targetName,
		HasBoundPayload: hasBounds,
	}
}

func alterUnsupportedCurrentStatusForCase(t *testing.T, name, sql string) alterUnsupportedCurrentStatus {
	t.Helper()
	p := New()
	result, err := p.Parse(sql)
	if err != nil {
		t.Fatalf("Parser.Parse(%q): %v", name, err)
	}
	if len(result.Statements) == 0 {
		t.Fatalf("Parser.Parse(%q): no statements returned", name)
	}

	es := result.Statements[0]
	stmt, extractErr := es.Extractor.Extract(spec.DialectPostgreSQL, sql)
	if extractErr != nil {
		t.Fatalf("Extract(%q): %v", name, extractErr)
	}

	status := alterUnsupportedCurrentStatus{
		Name:        name,
		Unsupported: stmt.Unsupported != nil,
	}
	if stmt.Unsupported != nil {
		status.UnsupportedFeat = stmt.Unsupported.Feature
		status.UnsupportedWhy = stmt.Unsupported.Reason
	}
	return status
}

func TestPostgreSQLAlterTableUnsupportedASTCensus(t *testing.T) {
	var facts []alterUnsupportedNodeFacts

	for _, tc := range alterTableUnsupportedASTCases {
		f := alterUnsupportedNodeFactsForCase(t, tc.name, tc.sql)
		facts = append(facts, f)
	}

	t.Log("")
	t.Log("=== PostgreSQL ALTER TABLE Unsupported Actions AST Census ===")
	t.Log("")
	t.Logf("%-25s | %-25s | %-20s | %-15s | %-25s | %-30s | %-6s",
		"Name", "TopLevelKind", "TargetTable", "AlterSubtype", "TargetName", "TargetSchema", "Bounds")
	t.Log(strings.Repeat("-", 180))

	for _, f := range facts {
		t.Logf("%-25s | %-25s | %-20s | %-15s | %-25s | %-30s | %-6v",
			f.Name, f.TopLevelKind, f.TargetTable, f.AlterSubtype, f.TargetName, f.TargetSchema, f.HasBoundPayload)
	}

	// Assert stable AST shapes.
	expectedShapes := map[string]struct {
		topKind    string
		subtype    string
		targetTbl  string
	}{
		"set_schema":          {topKind: "AlterObjectSchemaStmt", subtype: "", targetTbl: "users"},
		"owner_to":            {topKind: "AlterTableStmt", subtype: "AT_ChangeOwner", targetTbl: "users"},
		"enable_trigger_named": {topKind: "AlterTableStmt", subtype: "AT_EnableTrig", targetTbl: "users"},
		"disable_trigger_named": {topKind: "AlterTableStmt", subtype: "AT_DisableTrig", targetTbl: "users"},
		"attach_partition":    {topKind: "AlterTableStmt", subtype: "AT_AttachPartition", targetTbl: "measurement"},
		"detach_partition":    {topKind: "AlterTableStmt", subtype: "AT_DetachPartition", targetTbl: "measurement"},
	}

	for _, f := range facts {
		exp, ok := expectedShapes[f.Name]
		if !ok {
			t.Errorf("no expected shape for case %q", f.Name)
			continue
		}
		if f.TopLevelKind != exp.topKind {
			t.Errorf("case %q: top-level kind = %q, want %q", f.Name, f.TopLevelKind, exp.topKind)
		}
		if f.AlterSubtype != exp.subtype {
			t.Errorf("case %q: subtype = %q, want %q", f.Name, f.AlterSubtype, exp.subtype)
		}
		if f.TargetTable != exp.targetTbl {
			t.Errorf("case %q: target table = %q, want %q", f.Name, f.TargetTable, exp.targetTbl)
		}
	}

	// Assert attach_partition has a bound payload.
	for _, f := range facts {
		if f.Name == "attach_partition" && !f.HasBoundPayload {
			t.Error("attach_partition: expected hasBoundPayload = true")
		}
	}

	// Assert set_schema has new schema in targetName.
	for _, f := range facts {
		if f.Name == "set_schema" && f.TargetName != "archive" {
			t.Errorf("set_schema: targetName (new schema) = %q, want %q", f.TargetName, "archive")
		}
	}
}

func TestPostgreSQLAlterTableUnsupportedCurrentExtractionBaseline(t *testing.T) {
	var statuses []alterUnsupportedCurrentStatus

	for _, tc := range alterTableUnsupportedASTCases {
		s := alterUnsupportedCurrentStatusForCase(t, tc.name, tc.sql)
		statuses = append(statuses, s)
	}

	t.Log("")
	t.Log("=== PostgreSQL ALTER TABLE Unsupported Actions Current Extraction Baseline ===")
	t.Log("")
	t.Logf("%-25s | %-12s | %-25s | %s",
		"Name", "Unsupported", "Feature", "Reason")
	t.Log(strings.Repeat("-", 120))

	for _, s := range statuses {
		t.Logf("%-25s | %-12v | %-25s | %s",
			s.Name, s.Unsupported, s.UnsupportedFeat, s.UnsupportedWhy)
	}

	// All 6 forms must be unsupported.
	for _, s := range statuses {
		if !s.Unsupported {
			t.Errorf("case %q: expected unsupported = true, got false", s.Name)
		}
	}

	// Assert expected current feature names from v0.51.0.
	expectedFeatures := map[string]string{
		"set_schema":           "unknown",
		"owner_to":             "changeowner",
		"enable_trigger_named": "enabletrig",
		"disable_trigger_named":"disabletrig",
		"attach_partition":     "attachpartition",
		"detach_partition":     "detachpartition",
	}

	for _, s := range statuses {
		exp, ok := expectedFeatures[s.Name]
		if !ok {
			continue
		}
		if s.UnsupportedFeat != exp {
			t.Errorf("case %q: feature = %q, want %q (actual output: feature=%q reason=%q)",
				s.Name, s.UnsupportedFeat, exp, s.UnsupportedFeat, s.UnsupportedWhy)
		}
	}
}
