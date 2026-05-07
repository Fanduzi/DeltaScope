//go:build postgresql

package postgresql

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

var pgAlterTableResidualCensusCases = []struct {
	name string
	sql  string
}{
	{name: "enable_trigger_all", sql: "ALTER TABLE users ENABLE TRIGGER ALL"},
	{name: "enable_trigger_user", sql: "ALTER TABLE users ENABLE TRIGGER USER"},
	{name: "disable_trigger_all", sql: "ALTER TABLE users DISABLE TRIGGER ALL"},
	{name: "disable_trigger_user", sql: "ALTER TABLE users DISABLE TRIGGER USER"},
	{name: "replica_identity_default", sql: "ALTER TABLE users REPLICA IDENTITY DEFAULT"},
	{name: "replica_identity_full", sql: "ALTER TABLE users REPLICA IDENTITY FULL"},
	{name: "replica_identity_nothing", sql: "ALTER TABLE users REPLICA IDENTITY NOTHING"},
	{name: "replica_identity_using_index", sql: "ALTER TABLE users REPLICA IDENTITY USING INDEX users_replica_identity_idx"},
}

type alterTableResidualASTFact struct {
	Name               string
	SQL                string
	TopKind            string
	Table              string
	Subtype            string
	CmdName            string
	HasDef             bool
	DefKind            string
	Unsupported        bool
	UnsupportedFeature string
	UnsupportedReason  string
	DDLOperation       string
	AlterActions       []string
	AlterOptions       []map[string]string
}

func alterTableResidualASTFactsForCase(t *testing.T, name, sql string) alterTableResidualASTFact {
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

	fact := alterTableResidualASTFact{
		Name: name,
		SQL:  sql,
	}

	switch node.GetNode().(type) {
	case *pg_query.Node_AlterTableStmt:
		fact.TopKind = "AlterTableStmt"
	default:
		fact.TopKind = strings.TrimPrefix(fmt.Sprintf("%T", node.GetNode()), "*pg_query.Node_")
		return fact
	}

	atStmt := node.GetNode().(*pg_query.Node_AlterTableStmt).AlterTableStmt
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
	fact.CmdName = cmd.GetName()
	fact.HasDef = cmd.GetDef() != nil

	if fact.HasDef {
		defNode := cmd.GetDef()
		fact.DefKind = strings.TrimPrefix(fmt.Sprintf("%T", defNode.GetNode()), "*pg_query.Node_")
	}

	return fact
}

func alterTableResidualBaselineForCase(t *testing.T, name, sql string) alterTableResidualASTFact {
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

	fact := alterTableResidualASTFact{
		Name: name,
		SQL:  sql,
	}

	if stmt.Unsupported != nil {
		fact.Unsupported = true
		fact.UnsupportedFeature = stmt.Unsupported.Feature
		fact.UnsupportedReason = stmt.Unsupported.Reason
	} else if stmt.DDL != nil {
		fact.DDLOperation = string(stmt.DDL.Operation)
		for _, alter := range stmt.DDL.Alter {
			fact.AlterActions = append(fact.AlterActions, alter.Action)
			if alter.Options != nil {
				fact.AlterOptions = append(fact.AlterOptions, alter.Options)
			} else {
				fact.AlterOptions = append(fact.AlterOptions, nil)
			}
		}
	}

	return fact
}

func TestPostgreSQLAlterTableResidualASTCensus(t *testing.T) {
	t.Log("")
	t.Log("=== PostgreSQL ALTER TABLE Residual Candidates AST Census ===")
	t.Log("")
	t.Logf("%-28s | %-15s | %-12s | %-20s | %-6s | %-6s | %s",
		"Name", "TopKind", "Table", "Subtype", "CmdName", "HasDef", "DefKind")
	t.Log(strings.Repeat("-", 160))

	expectedShapes := map[string]struct {
		topKind string
		subtype string
		table   string
	}{
		"enable_trigger_all":           {topKind: "AlterTableStmt", subtype: "AT_EnableTrigAll", table: "users"},
		"enable_trigger_user":          {topKind: "AlterTableStmt", subtype: "AT_EnableTrigUser", table: "users"},
		"disable_trigger_all":          {topKind: "AlterTableStmt", subtype: "AT_DisableTrigAll", table: "users"},
		"disable_trigger_user":         {topKind: "AlterTableStmt", subtype: "AT_DisableTrigUser", table: "users"},
		"replica_identity_default":     {topKind: "AlterTableStmt", subtype: "AT_ReplicaIdentity", table: "users"},
		"replica_identity_full":        {topKind: "AlterTableStmt", subtype: "AT_ReplicaIdentity", table: "users"},
		"replica_identity_nothing":     {topKind: "AlterTableStmt", subtype: "AT_ReplicaIdentity", table: "users"},
		"replica_identity_using_index": {topKind: "AlterTableStmt", subtype: "AT_ReplicaIdentity", table: "users"},
	}

	for _, tc := range pgAlterTableResidualCensusCases {
		fact := alterTableResidualASTFactsForCase(t, tc.name, tc.sql)

		t.Logf("%-28s | %-15s | %-12s | %-20s | %-6q | %-6v | %s",
			fact.Name, fact.TopKind, fact.Table, fact.Subtype, fact.CmdName, fact.HasDef, fact.DefKind)

		exp, ok := expectedShapes[tc.name]
		if !ok {
			t.Errorf("no expected shape for case %q", tc.name)
			continue
		}
		if fact.TopKind != exp.topKind {
			t.Errorf("case %q: top-level kind = %q, want %q", tc.name, fact.TopKind, exp.topKind)
		}
		if fact.Subtype != exp.subtype {
			t.Errorf("case %q: subtype = %q, want %q", tc.name, fact.Subtype, exp.subtype)
		}
		if fact.Table != exp.table {
			t.Errorf("case %q: table = %q, want %q", tc.name, fact.Table, exp.table)
		}
	}
}

func TestPostgreSQLAlterTableResidualTriggerScopeASTDetail(t *testing.T) {
	t.Log("")
	t.Log("=== Trigger Scope AST Detail: ALL vs USER ===")
	t.Log("")

	triggerScopeCases := []struct {
		name        string
		sql         string
		wantSubtype string
		wantCmdName string
		wantHasDef  bool
	}{
		{name: "enable_trigger_all", sql: "ALTER TABLE users ENABLE TRIGGER ALL", wantSubtype: "AT_EnableTrigAll", wantCmdName: "", wantHasDef: false},
		{name: "enable_trigger_user", sql: "ALTER TABLE users ENABLE TRIGGER USER", wantSubtype: "AT_EnableTrigUser", wantCmdName: "", wantHasDef: false},
		{name: "disable_trigger_all", sql: "ALTER TABLE users DISABLE TRIGGER ALL", wantSubtype: "AT_DisableTrigAll", wantCmdName: "", wantHasDef: false},
		{name: "disable_trigger_user", sql: "ALTER TABLE users DISABLE TRIGGER USER", wantSubtype: "AT_DisableTrigUser", wantCmdName: "", wantHasDef: false},
	}

	for _, tc := range triggerScopeCases {
		tree, err := pg_query.Parse(tc.sql)
		if err != nil {
			t.Fatalf("%s: parse failed: %v", tc.name, err)
		}

		atStmt := tree.GetStmts()[0].GetStmt().GetNode().(*pg_query.Node_AlterTableStmt).AlterTableStmt
		cmd := atStmt.GetCmds()[0].GetAlterTableCmd()

		cmdName := cmd.GetName()
		hasDef := cmd.GetDef() != nil
		subtype := cmd.GetSubtype().String()

		t.Logf("%-28s | subtype=%s | cmd.GetName()=%q | HasDef=%v",
			tc.name, subtype, cmdName, hasDef)

		if subtype != tc.wantSubtype {
			t.Errorf("%s: subtype = %q, want %q", tc.name, subtype, tc.wantSubtype)
		}
		// ALL and USER are NOT named triggers — cmd.GetName() must be empty
		if cmdName != tc.wantCmdName {
			t.Errorf("%s: cmd.GetName() = %q, want %q (ALL/USER are not trigger names)",
				tc.name, cmdName, tc.wantCmdName)
		}
		if hasDef != tc.wantHasDef {
			t.Errorf("%s: HasDef = %v, want %v", tc.name, hasDef, tc.wantHasDef)
		}

		// Observe: how do we distinguish ALL from USER?
		// Check if there's any field that carries this distinction.
		// We log the full cmd proto for inspection.
		t.Logf("  -> cmd proto (string): %s", cmd.String())
	}
}

func TestPostgreSQLAlterTableResidualReplicaIdentityASTDetail(t *testing.T) {
	t.Log("")
	t.Log("=== Replica Identity AST Detail ===")
	t.Log("")

	replicaCases := []struct {
		name       string
		sql        string
		wantHasDef bool
	}{
		{name: "replica_identity_default", sql: "ALTER TABLE users REPLICA IDENTITY DEFAULT", wantHasDef: true},
		{name: "replica_identity_full", sql: "ALTER TABLE users REPLICA IDENTITY FULL", wantHasDef: true},
		{name: "replica_identity_nothing", sql: "ALTER TABLE users REPLICA IDENTITY NOTHING", wantHasDef: true},
		{name: "replica_identity_using_index", sql: "ALTER TABLE users REPLICA IDENTITY USING INDEX users_replica_identity_idx", wantHasDef: true},
	}

	for _, tc := range replicaCases {
		tree, err := pg_query.Parse(tc.sql)
		if err != nil {
			t.Fatalf("%s: parse failed: %v", tc.name, err)
		}

		atStmt := tree.GetStmts()[0].GetStmt().GetNode().(*pg_query.Node_AlterTableStmt).AlterTableStmt
		cmd := atStmt.GetCmds()[0].GetAlterTableCmd()

		hasDef := cmd.GetDef() != nil
		var defKind string
		var replicaIdentityStmt string
		if hasDef {
			defNode := cmd.GetDef()
			defKind = strings.TrimPrefix(fmt.Sprintf("%T", defNode.GetNode()), "*pg_query.Node_")
			// Log the def node for inspection
			replicaIdentityStmt = defNode.String()
		}

		t.Logf("%-28s | Subtype=%s | HasDef=%v | DefKind=%s",
			tc.name, cmd.GetSubtype().String(), hasDef, defKind)

		if hasDef != tc.wantHasDef {
			t.Errorf("%s: HasDef = %v, want %v", tc.name, hasDef, tc.wantHasDef)
		}

		// Log the full def node proto for detailed inspection
		t.Logf("  -> cmd proto: %s", cmd.String())
		if hasDef {
			t.Logf("  -> def proto: %s", replicaIdentityStmt)
		}

		// Assert def node is ReplicaIdentityStmt for all cases
		if hasDef {
			defNode := cmd.GetDef()
			if _, ok := defNode.GetNode().(*pg_query.Node_ReplicaIdentityStmt); !ok {
				t.Errorf("%s: expected def to be ReplicaIdentityStmt, got %T", tc.name, defNode.GetNode())
			}
		}
	}

	// Detailed extraction of ReplicaIdentityStmt fields
	t.Log("")
	t.Log("=== Replica Identity Stmt Field Extraction ===")
	t.Log("")

	// pg_query_go encodes identity_type as a single-char string matching
	// PostgreSQL's RepIdentity enum: 'd'=DEFAULT, 'f'=FULL, 'n'=NOTHING, 'i'=INDEX.
	detailCases := []struct {
		name              string
		sql               string
		expectedIdentity  string
		expectedIndexName string
	}{
		{name: "replica_identity_default", sql: "ALTER TABLE users REPLICA IDENTITY DEFAULT",
			expectedIdentity: "d", expectedIndexName: ""},
		{name: "replica_identity_full", sql: "ALTER TABLE users REPLICA IDENTITY FULL",
			expectedIdentity: "f", expectedIndexName: ""},
		{name: "replica_identity_nothing", sql: "ALTER TABLE users REPLICA IDENTITY NOTHING",
			expectedIdentity: "n", expectedIndexName: ""},
		{name: "replica_identity_using_index", sql: "ALTER TABLE users REPLICA IDENTITY USING INDEX users_replica_identity_idx",
			expectedIdentity: "i", expectedIndexName: "users_replica_identity_idx"},
	}

	for _, tc := range detailCases {
		tree, err := pg_query.Parse(tc.sql)
		if err != nil {
			t.Fatalf("%s: parse failed: %v", tc.name, err)
		}

		atStmt := tree.GetStmts()[0].GetStmt().GetNode().(*pg_query.Node_AlterTableStmt).AlterTableStmt
		cmd := atStmt.GetCmds()[0].GetAlterTableCmd()
		defNode := cmd.GetDef()

		riNode, ok := defNode.GetNode().(*pg_query.Node_ReplicaIdentityStmt)
		if !ok {
			t.Fatalf("%s: expected ReplicaIdentityStmt, got %T", tc.name, defNode.GetNode())
		}

		ri := riNode.ReplicaIdentityStmt
		identityType := ri.GetIdentityType()
		idxName := ri.GetName()

		t.Logf("%-28s | identity_type=%s | name=%q",
			tc.name, identityType, idxName)

		if identityType != tc.expectedIdentity {
			t.Errorf("%s: identity_type = %q, want %q", tc.name, identityType, tc.expectedIdentity)
		}
		if idxName != tc.expectedIndexName {
			t.Errorf("%s: index = %q, want %q", tc.name, idxName, tc.expectedIndexName)
		}
	}
}

func TestPostgreSQLAlterTableResidualCurrentExtractionBaseline(t *testing.T) {
	t.Log("")
	t.Log("=== PostgreSQL ALTER TABLE Residual Current Extraction Baseline ===")
	t.Log("")
	t.Logf("%-28s | %-8s | %-12s | %-25s | %s",
		"Name", "Kind", "Unsupported?", "Feature", "Reason / Operation")
	t.Log(strings.Repeat("-", 140))

	for _, tc := range pgAlterTableResidualCensusCases {
		baseline := alterTableResidualBaselineForCase(t, tc.name, tc.sql)

		status := "no"
		detail := ""
		if baseline.Unsupported {
			status = "yes"
			detail = fmt.Sprintf("%s: %s", baseline.UnsupportedFeature, baseline.UnsupportedReason)
		} else if baseline.DDLOperation != "" {
			actions := strings.Join(baseline.AlterActions, ",")
			detail = fmt.Sprintf("DDL op=%s actions=[%s]", baseline.DDLOperation, actions)
		}

		t.Logf("%-28s | %-8s | %-12s | %-25s | %s",
			baseline.Name, "kind", status, baseline.UnsupportedFeature, detail)

		// All 8 residual candidates are now normalized.
		if baseline.Unsupported {
			t.Errorf("case %q: expected normalized (unsupported=false), but got unsupported feature=%q reason=%q",
				tc.name, baseline.UnsupportedFeature, baseline.UnsupportedReason)
		}
		if baseline.DDLOperation != string(spec.DDLOperationAlterTable) {
			t.Errorf("case %q: expected DDL operation=%q, got %q", tc.name, spec.DDLOperationAlterTable, baseline.DDLOperation)
		}
		if len(baseline.AlterActions) != 1 {
			t.Errorf("case %q: expected 1 alter action, got %d", tc.name, len(baseline.AlterActions))
		}
	}
}
