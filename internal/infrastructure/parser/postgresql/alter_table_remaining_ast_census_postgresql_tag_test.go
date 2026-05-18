//go:build postgresql

package postgresql

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// pgAlterTableRemainingCensusCases covers ALTER TABLE forms that are candidates
// for normalization in the v0.56.0 remaining grammar pack.
var pgAlterTableRemainingCensusCases = []struct {
	name string
	sql  string
}{
	{name: "alter_column_type_basic", sql: "ALTER TABLE users ALTER COLUMN name TYPE text"},
	{name: "alter_column_type_using", sql: "ALTER TABLE users ALTER COLUMN name TYPE jsonb USING to_jsonb(name)"},
	{name: "set_logged", sql: "ALTER TABLE users SET LOGGED"},
	{name: "set_unlogged", sql: "ALTER TABLE users SET UNLOGGED"},
	{name: "set_tablespace", sql: "ALTER TABLE users SET TABLESPACE fastspace"},
}

// alterTableRemainingASTFact captures stable AST facts for one candidate form.
type alterTableRemainingASTFact struct {
	Name            string
	SQL             string
	TopKind         string
	Table           string
	Subtype         string
	Column          string
	TypeName        string
	HasUsingClause  bool
	UsingNodeKind   string
	LoggedState     string
	Tablespace      string
	HasDef          bool
	DefKind         string
	Unsupported     bool
	UnsupportedFeat string
	UnsupportedWhy  string
	DDLOperation    string
	AlterActions    []string
	AlterOptions    []map[string]string
}

// alterTableRemainingASTFactsForCase parses sql with pg_query and extracts
// stable AST facts without guessing field semantics.
func alterTableRemainingASTFactsForCase(t *testing.T, name, sql string) alterTableRemainingASTFact {
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

	fact := alterTableRemainingASTFact{
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
	fact.Column = cmd.GetName()
	fact.HasDef = cmd.GetDef() != nil

	// Extract form-specific facts based on observed AST.
	switch cmd.GetSubtype() {
	case pg_query.AlterTableType_AT_AlterColumnType:
		if cmd.GetDef() != nil {
			colDef := cmd.GetDef().GetColumnDef()
			if colDef != nil {
				fact.DefKind = "ColumnDef"
				if tn := colDef.GetTypeName(); tn != nil {
					parts := make([]string, 0, len(tn.GetNames()))
					for _, n := range tn.GetNames() {
						if s := n.GetString_(); s != nil {
							parts = append(parts, s.GetSval())
						}
					}
					fact.TypeName = strings.ToLower(strings.Join(parts, "."))
				}
				fact.HasUsingClause = colDef.GetRawDefault() != nil
				if fact.HasUsingClause {
					fact.UsingNodeKind = strings.TrimPrefix(
						fmt.Sprintf("%T", colDef.GetRawDefault().GetNode()),
						"*pg_query.Node_")
				}
			}
		}
	case pg_query.AlterTableType_AT_SetLogged:
		fact.LoggedState = "logged"
	case pg_query.AlterTableType_AT_SetUnLogged:
		fact.LoggedState = "unlogged"
	case pg_query.AlterTableType_AT_SetTableSpace:
		fact.Tablespace = cmd.GetName()
	}

	return fact
}

// alterTableRemainingBaselineForCase runs the current DeltaScope Parse+Extract
// pipeline and records the current behavior for each candidate.
func alterTableRemainingBaselineForCase(t *testing.T, name, sql string) alterTableRemainingASTFact {
	t.Helper()

	p := New()
	result, err := p.Parse(context.Background(), sql)
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

	fact := alterTableRemainingASTFact{
		Name: name,
		SQL:  sql,
	}

	if stmt.Unsupported != nil {
		fact.Unsupported = true
		fact.UnsupportedFeat = stmt.Unsupported.Feature
		fact.UnsupportedWhy = stmt.Unsupported.Reason
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

// TestPostgreSQLAlterTableRemainingASTCensus inspects the raw AST shape for each
// candidate form and asserts stable node types and extractable fields.
func TestPostgreSQLAlterTableRemainingASTCensus(t *testing.T) {
	t.Parallel()
	t.Log("")
	t.Log("=== PostgreSQL ALTER TABLE Remaining Candidates AST Census ===")
	t.Log("")
	t.Logf("%-28s | %-15s | %-12s | %-22s | %-6s | %-8s | %-8s | %-12s | %s",
		"Name", "TopKind", "Table", "Subtype", "Column", "HasDef", "TypeName", "HasUsing", "LoggedState/Tablespace")
	t.Log(strings.Repeat("-", 200))

	expectedShapes := map[string]struct {
		topKind       string
		subtype       string
		table         string
		column        string
		typeName      string
		hasUsing      bool
		usingNodeKind string
		loggedState   string
		tablespace    string
	}{
		"alter_column_type_basic": {
			topKind:  "AlterTableStmt",
			subtype:  "AT_AlterColumnType",
			table:    "users",
			column:   "name",
			typeName: "text",
			hasUsing: false,
		},
		"alter_column_type_using": {
			topKind:       "AlterTableStmt",
			subtype:       "AT_AlterColumnType",
			table:         "users",
			column:        "name",
			typeName:      "jsonb",
			hasUsing:      true,
			usingNodeKind: "FuncCall",
		},
		"set_logged": {
			topKind:     "AlterTableStmt",
			subtype:     "AT_SetLogged",
			table:       "users",
			loggedState: "logged",
		},
		"set_unlogged": {
			topKind:     "AlterTableStmt",
			subtype:     "AT_SetUnLogged",
			table:       "users",
			loggedState: "unlogged",
		},
		"set_tablespace": {
			topKind:    "AlterTableStmt",
			subtype:    "AT_SetTableSpace",
			table:      "users",
			column:     "fastspace", // cmd.GetName() carries the tablespace name
			tablespace: "fastspace",
		},
	}

	for _, tc := range pgAlterTableRemainingCensusCases {
		fact := alterTableRemainingASTFactsForCase(t, tc.name, tc.sql)

		extra := fact.LoggedState
		if fact.Tablespace != "" {
			extra = "tablespace=" + fact.Tablespace
		}

		t.Logf("%-28s | %-15s | %-12s | %-22s | %-6q | %-8v | %-8s | %-12v | %s",
			fact.Name, fact.TopKind, fact.Table, fact.Subtype,
			fact.Column, fact.HasDef, fact.TypeName, fact.HasUsingClause, extra)

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
		if fact.Column != exp.column {
			t.Errorf("case %q: column = %q, want %q", tc.name, fact.Column, exp.column)
		}
		if fact.TypeName != exp.typeName {
			t.Errorf("case %q: typeName = %q, want %q", tc.name, fact.TypeName, exp.typeName)
		}
		if fact.HasUsingClause != exp.hasUsing {
			t.Errorf("case %q: hasUsingClause = %v, want %v", tc.name, fact.HasUsingClause, exp.hasUsing)
		}
		if fact.HasUsingClause && fact.UsingNodeKind != exp.usingNodeKind {
			t.Errorf("case %q: usingNodeKind = %q, want %q", tc.name, fact.UsingNodeKind, exp.usingNodeKind)
		}
		if fact.LoggedState != exp.loggedState {
			t.Errorf("case %q: loggedState = %q, want %q", tc.name, fact.LoggedState, exp.loggedState)
		}
		if fact.Tablespace != exp.tablespace {
			t.Errorf("case %q: tablespace = %q, want %q", tc.name, fact.Tablespace, exp.tablespace)
		}
	}
}

// TestPostgreSQLAlterTableRemainingCurrentExtractionBaseline runs the current
// DeltaScope Parse+Extract pipeline and records what each candidate currently
// produces. This establishes the baseline for Task 2 decisions.
func TestPostgreSQLAlterTableRemainingCurrentExtractionBaseline(t *testing.T) {
	t.Parallel()
	t.Log("")
	t.Log("=== PostgreSQL ALTER TABLE Remaining Current Extraction Baseline ===")
	t.Log("")
	t.Logf("%-28s | %-12s | %-25s | %s",
		"Name", "Unsupported", "Feature", "Detail")
	t.Log(strings.Repeat("-", 140))

	type baselineExpectation struct {
		unsupported  bool
		feature      string
		ddlOp        string
		alterActions []string
		alterOpts    []map[string]string
	}

	expected := map[string]baselineExpectation{
		// ALTER COLUMN TYPE is already supported as set_data_type.
		"alter_column_type_basic": {
			unsupported:  false,
			ddlOp:        string(spec.DDLOperationAlterTable),
			alterActions: []string{"set_data_type"},
		},
		"alter_column_type_using": {
			unsupported:  false,
			ddlOp:        string(spec.DDLOperationAlterTable),
			alterActions: []string{"set_data_type"},
			alterOpts:    []map[string]string{{"has_using": "true"}},
		},
		// SET LOGGED / SET UNLOGGED are now normalized.
		"set_logged": {
			unsupported:  false,
			ddlOp:        string(spec.DDLOperationAlterTable),
			alterActions: []string{"set_logged"},
			alterOpts:    []map[string]string{{"logged": "true"}},
		},
		"set_unlogged": {
			unsupported:  false,
			ddlOp:        string(spec.DDLOperationAlterTable),
			alterActions: []string{"set_unlogged"},
			alterOpts:    []map[string]string{{"logged": "false"}},
		},
		// SET TABLESPACE is now finding-covered.
		"set_tablespace": {
			unsupported:  false,
			ddlOp:        string(spec.DDLOperationAlterTable),
			alterActions: []string{"set_tablespace"},
			alterOpts:    []map[string]string{{"tablespace": "fastspace"}},
		},
	}

	for _, tc := range pgAlterTableRemainingCensusCases {
		baseline := alterTableRemainingBaselineForCase(t, tc.name, tc.sql)

		detail := ""
		if baseline.Unsupported {
			detail = fmt.Sprintf("feature=%q reason=%q", baseline.UnsupportedFeat, baseline.UnsupportedWhy)
		} else if baseline.DDLOperation != "" {
			actions := strings.Join(baseline.AlterActions, ",")
			opts := make([]string, 0, len(baseline.AlterOptions))
			for i, o := range baseline.AlterOptions {
				for k, v := range o {
					opts = append(opts, fmt.Sprintf("alter[%d].%s=%s", i, k, v))
				}
			}
			detail = fmt.Sprintf("DDL op=%s actions=[%s] opts=[%s]", baseline.DDLOperation, actions, strings.Join(opts, ", "))
		}

		status := "supported"
		if baseline.Unsupported {
			status = "UNSUPPORTED"
		}

		t.Logf("%-28s | %-12s | %-25s | %s",
			baseline.Name, status, baseline.UnsupportedFeat, detail)

		exp, ok := expected[tc.name]
		if !ok {
			t.Errorf("no baseline expectation for case %q", tc.name)
			continue
		}
		if baseline.Unsupported != exp.unsupported {
			t.Errorf("case %q: unsupported = %v, want %v", tc.name, baseline.Unsupported, exp.unsupported)
		}
		if baseline.Unsupported && exp.feature != "" && baseline.UnsupportedFeat != exp.feature {
			t.Errorf("case %q: unsupported feature = %q, want %q", tc.name, baseline.UnsupportedFeat, exp.feature)
		}
		if !baseline.Unsupported {
			if baseline.DDLOperation != exp.ddlOp {
				t.Errorf("case %q: DDL operation = %q, want %q", tc.name, baseline.DDLOperation, exp.ddlOp)
			}
			if len(baseline.AlterActions) != len(exp.alterActions) {
				t.Errorf("case %q: got %d alter actions, want %d", tc.name, len(baseline.AlterActions), len(exp.alterActions))
			} else {
				for i, got := range baseline.AlterActions {
					if got != exp.alterActions[i] {
						t.Errorf("case %q: alter[%d].action = %q, want %q", tc.name, i, got, exp.alterActions[i])
					}
				}
			}
			if len(exp.alterOpts) > 0 {
				for i, wantOpts := range exp.alterOpts {
					if i >= len(baseline.AlterOptions) {
						t.Errorf("case %q: missing alter options at index %d", tc.name, i)
						continue
					}
					gotOpts := baseline.AlterOptions[i]
					for k, v := range wantOpts {
						if gotOpts == nil {
							t.Errorf("case %q: alter[%d].options nil, want %s=%s", tc.name, i, k, v)
						} else if gotOpts[k] != v {
							t.Errorf("case %q: alter[%d].options[%q] = %q, want %q", tc.name, i, k, gotOpts[k], v)
						}
					}
				}
			}
		}
	}
}

// TestPostgreSQLAlterTableRemainingUsingClauseDetail examines the USING clause
// AST shape for ALTER COLUMN TYPE ... USING to confirm whether it can be
// stably rendered or only detected as a boolean.
func TestPostgreSQLAlterTableRemainingUsingClauseDetail(t *testing.T) {
	t.Parallel()
	t.Log("")
	t.Log("=== USING Clause AST Detail for ALTER COLUMN TYPE ===")
	t.Log("")

	sql := "ALTER TABLE users ALTER COLUMN name TYPE jsonb USING to_jsonb(name)"
	tree, err := pg_query.Parse(sql)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	atStmt := tree.GetStmts()[0].GetStmt().GetNode().(*pg_query.Node_AlterTableStmt).AlterTableStmt
	cmd := atStmt.GetCmds()[0].GetAlterTableCmd()
	colDef := cmd.GetDef().GetColumnDef()

	if colDef == nil {
		t.Fatal("expected ColumnDef def payload")
	}
	if colDef.GetRawDefault() == nil {
		t.Fatal("expected raw_default (USING clause) to be present")
	}

	usingNode := colDef.GetRawDefault()
	usingKind := strings.TrimPrefix(fmt.Sprintf("%T", usingNode.GetNode()), "*pg_query.Node_")

	t.Logf("USING node kind: %s", usingKind)
	t.Logf("USING node proto: %s", usingNode.String())

	// The USING expression is a FuncCall node. We can detect its presence
	// but rendering arbitrary expressions back to SQL is not reliably stable.
	// Task 2 should capture has_using=true as a coarse boolean fact, not attempt
	// to render the expression text.
	if usingKind != "FuncCall" {
		t.Errorf("USING node kind = %q, want FuncCall", usingKind)
	}

	// Confirm that the FuncCall has a function name we can extract.
	fc := usingNode.GetFuncCall()
	if fc == nil {
		t.Fatal("expected FuncCall node")
	}
	var funcNames []string
	for _, n := range fc.GetFuncname() {
		if s := n.GetString_(); s != nil {
			funcNames = append(funcNames, s.GetSval())
		}
	}
	t.Logf("USING function name: %v", funcNames)

	if len(funcNames) == 0 || funcNames[0] != "to_jsonb" {
		t.Errorf("USING function name = %v, want [to_jsonb]", funcNames)
	}
}

// TestPostgreSQLAlterTableRemainingSetLoggedDetail examines SET LOGGED/UNLOGGED
// AST shape to confirm extractability.
func TestPostgreSQLAlterTableRemainingSetLoggedDetail(t *testing.T) {
	t.Parallel()
	t.Log("")
	t.Log("=== SET LOGGED / SET UNLOGGED AST Detail ===")
	t.Log("")

	cases := []struct {
		name        string
		sql         string
		wantSubtype string
		wantName    string
		wantHasDef  bool
	}{
		{name: "set_logged", sql: "ALTER TABLE users SET LOGGED",
			wantSubtype: "AT_SetLogged", wantName: "", wantHasDef: false},
		{name: "set_unlogged", sql: "ALTER TABLE users SET UNLOGGED",
			wantSubtype: "AT_SetUnLogged", wantName: "", wantHasDef: false},
	}

	for _, tc := range cases {
		tree, err := pg_query.Parse(tc.sql)
		if err != nil {
			t.Fatalf("%s: parse failed: %v", tc.name, err)
		}

		atStmt := tree.GetStmts()[0].GetStmt().GetNode().(*pg_query.Node_AlterTableStmt).AlterTableStmt
		cmd := atStmt.GetCmds()[0].GetAlterTableCmd()

		subtype := cmd.GetSubtype().String()
		name := cmd.GetName()
		hasDef := cmd.GetDef() != nil

		t.Logf("%-12s | subtype=%s | name=%q | hasDef=%v | cmd.proto=%s",
			tc.name, subtype, name, hasDef, cmd.String())

		if subtype != tc.wantSubtype {
			t.Errorf("%s: subtype = %q, want %q", tc.name, subtype, tc.wantSubtype)
		}
		if name != tc.wantName {
			t.Errorf("%s: name = %q, want %q", tc.name, name, tc.wantName)
		}
		if hasDef != tc.wantHasDef {
			t.Errorf("%s: hasDef = %v, want %v", tc.name, hasDef, tc.wantHasDef)
		}
	}
}

// TestPostgreSQLAlterTableRemainingSetTablespaceDetail examines SET TABLESPACE
// AST shape to confirm extractability.
func TestPostgreSQLAlterTableRemainingSetTablespaceDetail(t *testing.T) {
	t.Parallel()
	t.Log("")
	t.Log("=== SET TABLESPACE AST Detail ===")
	t.Log("")

	sql := "ALTER TABLE users SET TABLESPACE fastspace"
	tree, err := pg_query.Parse(sql)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	atStmt := tree.GetStmts()[0].GetStmt().GetNode().(*pg_query.Node_AlterTableStmt).AlterTableStmt
	cmd := atStmt.GetCmds()[0].GetAlterTableCmd()

	subtype := cmd.GetSubtype().String()
	name := cmd.GetName()
	hasDef := cmd.GetDef() != nil

	t.Logf("subtype=%s | name=%q | hasDef=%v | cmd.proto=%s",
		subtype, name, hasDef, cmd.String())

	if subtype != "AT_SetTableSpace" {
		t.Errorf("subtype = %q, want AT_SetTableSpace", subtype)
	}
	if name != "fastspace" {
		t.Errorf("name = %q, want %q", name, "fastspace")
	}
	if hasDef {
		t.Error("hasDef = true, want false (tablespace name is in cmd.GetName())")
	}
}

// TestPostgreSQLAlterTableRemainingExistingRuleCoverage confirms that
// ALTER COLUMN TYPE is already covered by existing rules and records which
// rules exist for it. This prevents Task 2 from duplicating support.
func TestPostgreSQLAlterTableRemainingExistingRuleCoverage(t *testing.T) {
	t.Parallel()
	t.Log("")
	t.Log("=== Existing Rule Coverage for ALTER COLUMN TYPE ===")
	t.Log("")

	// The following rules already exist for set_data_type:
	// 1. ddl.alter.set_data_type.forbid - generic cross-dialect forbid rule
	// 2. ddl.pg.alter.set_data_type.rewrite.warn - PG-specific rewrite warning
	//
	// The current extractor already normalizes AT_AlterColumnType as:
	//   Action: "set_data_type", Name: <column>, Column.Type: <new_type>
	//
	// However, the USING clause is NOT currently captured in Options.
	// Task 2 should add "has_using" to Options for set_data_type actions,
	// not create a new action or re-normalize.

	cases := []struct {
		name         string
		sql          string
		wantAction   string
		wantType     string
		wantHasUsing bool
	}{
		{
			name:         "alter_column_type_basic",
			sql:          "ALTER TABLE users ALTER COLUMN name TYPE text",
			wantAction:   "set_data_type",
			wantType:     "text",
			wantHasUsing: false,
		},
		{
			name:         "alter_column_type_using",
			sql:          "ALTER TABLE users ALTER COLUMN name TYPE jsonb USING to_jsonb(name)",
			wantAction:   "set_data_type",
			wantType:     "jsonb",
			wantHasUsing: true,
		},
	}

	for _, tc := range cases {
		p := New()
		result, err := p.Parse(context.Background(), tc.sql)
		if err != nil {
			t.Fatalf("%s: parse failed: %v", tc.name, err)
		}

		stmt, extractErr := result.Statements[0].Extractor.Extract(spec.DialectPostgreSQL, tc.sql)
		if extractErr != nil {
			t.Fatalf("%s: extract failed: %v", tc.name, extractErr)
		}

		if stmt.Unsupported != nil {
			t.Errorf("%s: unexpected unsupported: %s: %s", tc.name, stmt.Unsupported.Feature, stmt.Unsupported.Reason)
			continue
		}
		if stmt.DDL == nil || len(stmt.DDL.Alter) == 0 {
			t.Fatalf("%s: expected DDL with alter actions", tc.name)
		}

		alter := stmt.DDL.Alter[0]
		colType := ""
		if alter.Column != nil && alter.Column.Definition != nil {
			colType = alter.Column.Definition.Type
		}
		t.Logf("%-28s | action=%s | name=%q | type=%q | options=%v",
			tc.name, alter.Action, alter.Name, colType, alter.Options)

		if alter.Action != tc.wantAction {
			t.Errorf("%s: action = %q, want %q", tc.name, alter.Action, tc.wantAction)
		}
		if alter.Column == nil || alter.Column.Definition == nil {
			t.Fatalf("%s: expected Column.Definition with type info", tc.name)
		}
		if alter.Column.Definition.Type != tc.wantType {
			t.Errorf("%s: type = %q, want %q", tc.name, alter.Column.Definition.Type, tc.wantType)
		}

		// Verify has_using option presence
		hasUsingOpt := alter.Options != nil && alter.Options["has_using"] == "true"
		if hasUsingOpt != tc.wantHasUsing {
			t.Errorf("%s: has_using = %v, want %v (options=%v)", tc.name, hasUsingOpt, tc.wantHasUsing, alter.Options)
		}
	}
}
