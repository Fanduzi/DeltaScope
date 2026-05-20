//go:build postgresql

package postgresql

import (
	"fmt"
	"strings"
	"testing"

	pg_query "github.com/pganalyze/pg_query_go/v6"
)

var pgAlterTableBoundedResidualCensusCases = []struct {
	name string
	sql  string
}{
	{name: "set_statistics_value", sql: "ALTER TABLE users ALTER COLUMN email SET STATISTICS 100"},
	{name: "set_statistics_default", sql: "ALTER TABLE users ALTER COLUMN email SET STATISTICS DEFAULT"},
	{name: "set_column_options", sql: "ALTER TABLE users ALTER COLUMN email SET (n_distinct = -1)"},
	{name: "reset_column_options", sql: "ALTER TABLE users ALTER COLUMN email RESET (n_distinct)"},
	{name: "set_storage_external", sql: "ALTER TABLE users ALTER COLUMN bio SET STORAGE EXTERNAL"},
	{name: "set_storage_default", sql: "ALTER TABLE users ALTER COLUMN bio SET STORAGE DEFAULT"},
	{name: "set_compression_lz4", sql: "ALTER TABLE users ALTER COLUMN bio SET COMPRESSION lz4"},
	{name: "cluster_on", sql: "ALTER TABLE users CLUSTER ON users_email_idx"},
	{name: "set_without_cluster", sql: "ALTER TABLE users SET WITHOUT CLUSTER"},
	{name: "detach_partition_finalize", sql: "ALTER TABLE measurement DETACH PARTITION measurement_y2026m04 FINALIZE"},
}

type boundedResidualASTFact struct {
	Name             string
	SQL              string
	TopKind          string
	Table            string
	Subtype          string
	SubtypeValue     int32
	CmdName          string
	HasDef           bool
	DefKind          string
	HasDefList       bool
	DefListItemCount int
	PayloadNote      string
}

func boundedResidualASTFactsForCase(t *testing.T, name, sql string) boundedResidualASTFact {
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

	fact := boundedResidualASTFact{
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

		if list := defNode.GetList(); list != nil {
			fact.HasDefList = true
			fact.DefListItemCount = len(list.GetItems())
		}
	}

	return fact
}

func TestAlterTableBoundedResidualASTCensus(t *testing.T) {
	t.Parallel()
	t.Log("")
	t.Log("=== PostgreSQL ALTER TABLE Bounded Residual Candidates AST Census ===")
	t.Log("")
	t.Logf("%-28s | %-15s | %-12s | %-25s | %-3s | %-12s | %-6s | %-12s | %-6s | %-3s | %s",
		"Name", "TopKind", "Table", "Subtype", "Val", "CmdName", "HasDef", "DefKind", "IsList", "N", "PayloadNote")
	t.Log(strings.Repeat("-", 200))

	for _, tc := range pgAlterTableBoundedResidualCensusCases {
		fact := boundedResidualASTFactsForCase(t, tc.name, tc.sql)

		t.Logf("%-28s | %-15s | %-12s | %-25s | %-3d | %-12s | %-6v | %-12s | %-6v | %-3d | %s",
			fact.Name, fact.TopKind, fact.Table, fact.Subtype, fact.SubtypeValue,
			fact.CmdName, fact.HasDef, fact.DefKind, fact.HasDefList, fact.DefListItemCount, fact.PayloadNote)
	}
}

func TestAlterTableBoundedResidualStableShapeAssertions(t *testing.T) {
	t.Parallel()

	expectedShapes := map[string]struct {
		topKind string
		table   string
		subtype string
	}{
		"set_statistics_value":      {topKind: "AlterTableStmt", table: "users", subtype: "AT_SetStatistics"},
		"set_statistics_default":    {topKind: "AlterTableStmt", table: "users", subtype: "AT_SetStatistics"},
		"set_column_options":        {topKind: "AlterTableStmt", table: "users", subtype: "AT_SetOptions"},
		"reset_column_options":      {topKind: "AlterTableStmt", table: "users", subtype: "AT_ResetOptions"},
		"set_storage_external":      {topKind: "AlterTableStmt", table: "users", subtype: "AT_SetStorage"},
		"set_storage_default":       {topKind: "AlterTableStmt", table: "users", subtype: "AT_SetStorage"},
		"set_compression_lz4":       {topKind: "AlterTableStmt", table: "users", subtype: "AT_SetCompression"},
		"cluster_on":                {topKind: "AlterTableStmt", table: "users", subtype: "AT_ClusterOn"},
		"set_without_cluster":       {topKind: "AlterTableStmt", table: "users", subtype: "AT_DropCluster"},
		"detach_partition_finalize": {topKind: "AlterTableStmt", table: "measurement", subtype: "AT_DetachPartitionFinalize"},
	}

	for _, tc := range pgAlterTableBoundedResidualCensusCases {
		fact := boundedResidualASTFactsForCase(t, tc.name, tc.sql)

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

func TestAlterTableBoundedResidualDefPayloadDetail(t *testing.T) {
	t.Parallel()
	t.Log("")
	t.Log("=== Def Payload Detail for Bounded Residuals ===")
	t.Log("")

	for _, tc := range pgAlterTableBoundedResidualCensusCases {
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
			case *pg_query.Node_Integer:
				t.Logf("  def.Integer.ivalue=%d", dn.Integer.GetIval())
			case *pg_query.Node_String_:
				t.Logf("  def.String.sval=%q", dn.String_.GetSval())
			case *pg_query.Node_List:
				items := dn.List.GetItems()
				t.Logf("  def.List.items=%d", len(items))
				for i, item := range items {
					itemKind := strings.TrimPrefix(fmt.Sprintf("%T", item.GetNode()), "*pg_query.Node_")
					t.Logf("    item[%d]: kind=%s", i, itemKind)
				}
			case *pg_query.Node_DefElem:
				t.Logf("  def.DefElem.defname=%q", dn.DefElem.GetDefname())
				argKind := "nil"
				if dn.DefElem.GetArg() != nil {
					argKind = strings.TrimPrefix(fmt.Sprintf("%T", dn.DefElem.GetArg().GetNode()), "*pg_query.Node_")
				}
				t.Logf("  def.DefElem.arg_kind=%s", argKind)
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

		t.Logf("  cmd.proto: %s", cmd.String())
		t.Log("")
	}
}

func TestAlterTableBoundedResidualIdentityFieldMapping(t *testing.T) {
	t.Parallel()
	t.Log("")
	t.Log("=== Identity Field Mapping ===")
	t.Log("")

	type identityExpectation struct {
		column            string
		index             string
		partition         string
		hasDef            bool
		defCarriesPayload bool
	}

	expectations := map[string]identityExpectation{
		"set_statistics_value":      {column: "email", hasDef: true, defCarriesPayload: true},
		"set_statistics_default":    {column: "email", hasDef: false, defCarriesPayload: false},
		"set_column_options":        {column: "email", hasDef: true, defCarriesPayload: true},
		"reset_column_options":      {column: "email", hasDef: true, defCarriesPayload: true},
		"set_storage_external":      {column: "bio", hasDef: true, defCarriesPayload: true},
		"set_storage_default":       {column: "bio", hasDef: true, defCarriesPayload: true},
		"set_compression_lz4":       {column: "bio", hasDef: true, defCarriesPayload: true},
		"cluster_on":                {index: "users_email_idx", hasDef: false, defCarriesPayload: false},
		"set_without_cluster":       {hasDef: false, defCarriesPayload: false},
		"detach_partition_finalize": {hasDef: true, defCarriesPayload: true},
	}

	for _, tc := range pgAlterTableBoundedResidualCensusCases {
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

		cmdName := cmd.GetName()
		hasDef := cmd.GetDef() != nil

		if exp.column != "" && cmdName != exp.column {
			t.Errorf("%s: cmdName=%q, want column=%q", tc.name, cmdName, exp.column)
		}
		if exp.index != "" && cmdName != exp.index {
			t.Errorf("%s: cmdName=%q, want index=%q", tc.name, cmdName, exp.index)
		}
		if exp.partition != "" && cmdName != exp.partition {
			t.Errorf("%s: cmdName=%q, want partition=%q", tc.name, cmdName, exp.partition)
		}
		if hasDef != exp.hasDef {
			t.Errorf("%s: hasDef=%v, want %v", tc.name, hasDef, exp.hasDef)
		}

		// For DETACH PARTITION FINALIZE, partition name is in
		// def.PartitionCmd.name.relname, not cmd.GetName().
		partitionOK := true
		if tc.name == "detach_partition_finalize" && hasDef {
			pcNode, ok := cmd.GetDef().GetNode().(*pg_query.Node_PartitionCmd)
			if !ok {
				partitionOK = false
				t.Errorf("%s: expected def to be PartitionCmd", tc.name)
			} else {
				partName := pcNode.PartitionCmd.GetName().GetRelname()
				if partName != "measurement_y2026m04" {
					partitionOK = false
					t.Errorf("%s: partition name=%q, want measurement_y2026m04", tc.name, partName)
				}
				t.Logf("  partition name from def: %q", partName)
			}
		}

		t.Logf("%-28s | cmdName=%q | hasDef=%v | identity_OK=%v",
			tc.name, cmdName, hasDef,
			(exp.column == "" || cmdName == exp.column) &&
				(exp.index == "" || cmdName == exp.index) &&
				hasDef == exp.hasDef && partitionOK)
	}
}

func TestAlterTableBoundedResidualDetachFinalizeFlag(t *testing.T) {
	t.Parallel()
	t.Log("")
	t.Log("=== DETACH PARTITION ... FINALIZE Flag Inspection ===")
	t.Log("")

	cases := []struct {
		name           string
		sql            string
		wantSubtype    string
		wantCmdName    string
		wantHasDef     bool
		wantConcurrent bool
	}{
		{name: "detach_plain", sql: "ALTER TABLE measurement DETACH PARTITION measurement_y2026m04",
			wantSubtype: "AT_DetachPartition", wantCmdName: "", wantHasDef: true, wantConcurrent: false},
		{name: "detach_finalize", sql: "ALTER TABLE measurement DETACH PARTITION measurement_y2026m04 FINALIZE",
			wantSubtype: "AT_DetachPartitionFinalize", wantCmdName: "", wantHasDef: true, wantConcurrent: false},
		{name: "detach_concurrently", sql: "ALTER TABLE measurement DETACH PARTITION measurement_y2026m04 CONCURRENTLY",
			wantSubtype: "AT_DetachPartition", wantCmdName: "", wantHasDef: true, wantConcurrent: true},
	}

	for _, tc := range cases {
		tree, err := pg_query.Parse(tc.sql)
		if err != nil {
			t.Fatalf("%s: parse failed: %v", tc.name, err)
		}

		atStmt := tree.GetStmts()[0].GetStmt().GetNode().(*pg_query.Node_AlterTableStmt).AlterTableStmt
		cmd := atStmt.GetCmds()[0].GetAlterTableCmd()

		subtype := cmd.GetSubtype().String()
		cmdName := cmd.GetName()
		hasDef := cmd.GetDef() != nil

		t.Logf("%-20s | subtype=%s | cmdName=%q | hasDef=%v", tc.name, subtype, cmdName, hasDef)

		if subtype != tc.wantSubtype {
			t.Errorf("%s: subtype = %q, want %q", tc.name, subtype, tc.wantSubtype)
		}
		if cmdName != tc.wantCmdName {
			t.Errorf("%s: cmdName = %q, want %q", tc.name, cmdName, tc.wantCmdName)
		}
		if hasDef != tc.wantHasDef {
			t.Errorf("%s: hasDef = %v, want %v", tc.name, hasDef, tc.wantHasDef)
		}

		if hasDef {
			defNode := cmd.GetDef()
			defKind := strings.TrimPrefix(fmt.Sprintf("%T", defNode.GetNode()), "*pg_query.Node_")
			t.Logf("  defKind=%s | def proto: %s", defKind, defNode.String())

			if pcNode, ok := defNode.GetNode().(*pg_query.Node_PartitionCmd); ok {
				partName := pcNode.PartitionCmd.GetName().GetRelname()
				concurrent := pcNode.PartitionCmd.GetConcurrent()
				t.Logf("  partition=%q | concurrent=%v", partName, concurrent)
				if partName != "measurement_y2026m04" {
					t.Errorf("%s: partition name = %q, want measurement_y2026m04", tc.name, partName)
				}
				if concurrent != tc.wantConcurrent {
					t.Errorf("%s: concurrent = %v, want %v", tc.name, concurrent, tc.wantConcurrent)
				}
			}
		}

		t.Logf("  cmd proto: %s", cmd.String())
	}
}

func TestAlterTableBoundedResidualColumnStatisticsDetail(t *testing.T) {
	t.Parallel()
	t.Log("")
	t.Log("=== SET STATISTICS Payload Detail ===")
	t.Log("")

	cases := []struct {
		name        string
		sql         string
		wantCmdName string
		wantHasDef  bool
	}{
		{name: "set_statistics_value", sql: "ALTER TABLE users ALTER COLUMN email SET STATISTICS 100", wantCmdName: "email", wantHasDef: true},
		{name: "set_statistics_default", sql: "ALTER TABLE users ALTER COLUMN email SET STATISTICS DEFAULT", wantCmdName: "email", wantHasDef: false},
	}

	for _, tc := range cases {
		tree, err := pg_query.Parse(tc.sql)
		if err != nil {
			t.Fatalf("%s: parse failed: %v", tc.name, err)
		}

		atStmt := tree.GetStmts()[0].GetStmt().GetNode().(*pg_query.Node_AlterTableStmt).AlterTableStmt
		cmd := atStmt.GetCmds()[0].GetAlterTableCmd()

		cmdName := cmd.GetName()
		hasDef := cmd.GetDef() != nil

		t.Logf("%-24s | subtype=%s | cmdName=%q | hasDef=%v", tc.name, cmd.GetSubtype().String(), cmdName, hasDef)

		if cmdName != tc.wantCmdName {
			t.Errorf("%s: cmdName = %q, want %q", tc.name, cmdName, tc.wantCmdName)
		}
		if hasDef != tc.wantHasDef {
			t.Errorf("%s: hasDef = %v, want %v", tc.name, hasDef, tc.wantHasDef)
		}

		if hasDef {
			defNode := cmd.GetDef()
			defKind := strings.TrimPrefix(fmt.Sprintf("%T", defNode.GetNode()), "*pg_query.Node_")
			t.Logf("  defKind=%s", defKind)

			switch dn := defNode.GetNode().(type) {
			case *pg_query.Node_Integer:
				t.Logf("  def.Integer.ivalue=%d (this is the statistics target — must be ignored)", dn.Integer.GetIval())
			case *pg_query.Node_TypeName:
				names := dn.TypeName.GetNames()
				var nameStrs []string
				for _, n := range names {
					if s := n.GetString_(); s != nil {
						nameStrs = append(nameStrs, s.GetSval())
					}
				}
				t.Logf("  def.TypeName.names=%v (DEFAULT encoded as TypeName)", nameStrs)
			default:
				t.Logf("  def (unexpected): %s", defNode.String())
			}
		}

		t.Logf("  cmd proto: %s", cmd.String())
		t.Log("")
	}
}

func TestAlterTableBoundedResidualColumnOptionsDetail(t *testing.T) {
	t.Parallel()
	t.Log("")
	t.Log("=== SET/RESET Column Options Payload Detail ===")
	t.Log("")

	cases := []struct {
		name              string
		sql               string
		wantSubtype       string
		wantCmdName       string
		wantListItemCount int
	}{
		{name: "set_column_options", sql: "ALTER TABLE users ALTER COLUMN email SET (n_distinct = -1)", wantSubtype: "AT_SetOptions", wantCmdName: "email", wantListItemCount: 1},
		{name: "reset_column_options", sql: "ALTER TABLE users ALTER COLUMN email RESET (n_distinct)", wantSubtype: "AT_ResetOptions", wantCmdName: "email", wantListItemCount: 1},
	}

	for _, tc := range cases {
		tree, err := pg_query.Parse(tc.sql)
		if err != nil {
			t.Fatalf("%s: parse failed: %v", tc.name, err)
		}

		atStmt := tree.GetStmts()[0].GetStmt().GetNode().(*pg_query.Node_AlterTableStmt).AlterTableStmt
		cmd := atStmt.GetCmds()[0].GetAlterTableCmd()

		subtype := cmd.GetSubtype().String()
		cmdName := cmd.GetName()
		hasDef := cmd.GetDef() != nil

		t.Logf("%-24s | subtype=%s | cmdName=%q | hasDef=%v", tc.name, subtype, cmdName, hasDef)

		if subtype != tc.wantSubtype {
			t.Errorf("%s: subtype = %q, want %q", tc.name, subtype, tc.wantSubtype)
		}
		if cmdName != tc.wantCmdName {
			t.Errorf("%s: cmdName = %q, want %q", tc.name, cmdName, tc.wantCmdName)
		}

		if hasDef {
			defNode := cmd.GetDef()
			if list := defNode.GetList(); list != nil {
				items := list.GetItems()
				t.Logf("  def.List.items=%d (option names/values must be ignored)", len(items))
				if len(items) != tc.wantListItemCount {
					t.Errorf("%s: list item count = %d, want %d", tc.name, len(items), tc.wantListItemCount)
				}
			} else {
				defKind := strings.TrimPrefix(fmt.Sprintf("%T", defNode.GetNode()), "*pg_query.Node_")
				t.Logf("  defKind=%s (expected List)", defKind)
			}
		}

		t.Logf("  cmd proto: %s", cmd.String())
		t.Log("")
	}
}

func TestAlterTableBoundedResidualStorageCompressionDetail(t *testing.T) {
	t.Parallel()
	t.Log("")
	t.Log("=== SET STORAGE / SET COMPRESSION Payload Detail ===")
	t.Log("")

	cases := []struct {
		name        string
		sql         string
		wantSubtype string
		wantCmdName string
	}{
		{name: "set_storage_external", sql: "ALTER TABLE users ALTER COLUMN bio SET STORAGE EXTERNAL", wantSubtype: "AT_SetStorage", wantCmdName: "bio"},
		{name: "set_storage_default", sql: "ALTER TABLE users ALTER COLUMN bio SET STORAGE DEFAULT", wantSubtype: "AT_SetStorage", wantCmdName: "bio"},
		{name: "set_compression_lz4", sql: "ALTER TABLE users ALTER COLUMN bio SET COMPRESSION lz4", wantSubtype: "AT_SetCompression", wantCmdName: "bio"},
	}

	for _, tc := range cases {
		tree, err := pg_query.Parse(tc.sql)
		if err != nil {
			t.Fatalf("%s: parse failed: %v", tc.name, err)
		}

		atStmt := tree.GetStmts()[0].GetStmt().GetNode().(*pg_query.Node_AlterTableStmt).AlterTableStmt
		cmd := atStmt.GetCmds()[0].GetAlterTableCmd()

		subtype := cmd.GetSubtype().String()
		cmdName := cmd.GetName()
		hasDef := cmd.GetDef() != nil

		t.Logf("%-24s | subtype=%s | cmdName=%q | hasDef=%v", tc.name, subtype, cmdName, hasDef)

		if subtype != tc.wantSubtype {
			t.Errorf("%s: subtype = %q, want %q", tc.name, subtype, tc.wantSubtype)
		}
		if cmdName != tc.wantCmdName {
			t.Errorf("%s: cmdName = %q, want %q", tc.name, cmdName, tc.wantCmdName)
		}

		if hasDef {
			defNode := cmd.GetDef()
			defKind := strings.TrimPrefix(fmt.Sprintf("%T", defNode.GetNode()), "*pg_query.Node_")
			t.Logf("  defKind=%s", defKind)

			switch dn := defNode.GetNode().(type) {
			case *pg_query.Node_String_:
				t.Logf("  def.String.sval=%q (storage/compression value — must evaluate if bounded)", dn.String_.GetSval())
			case *pg_query.Node_TypeName:
				names := dn.TypeName.GetNames()
				var nameStrs []string
				for _, n := range names {
					if s := n.GetString_(); s != nil {
						nameStrs = append(nameStrs, s.GetSval())
					}
				}
				t.Logf("  def.TypeName.names=%v (DEFAULT encoded as TypeName)", nameStrs)
			case *pg_query.Node_List:
				t.Logf("  def.List.items=%d", len(dn.List.GetItems()))
			default:
				t.Logf("  def (other): %s", defNode.String())
			}
		}

		t.Logf("  cmd proto: %s", cmd.String())
		t.Log("")
	}
}

func TestAlterTableBoundedResidualClusterDetail(t *testing.T) {
	t.Parallel()
	t.Log("")
	t.Log("=== CLUSTER ON / SET WITHOUT CLUSTER Detail ===")
	t.Log("")

	cases := []struct {
		name        string
		sql         string
		wantSubtype string
		wantCmdName string
		wantHasDef  bool
	}{
		{name: "cluster_on", sql: "ALTER TABLE users CLUSTER ON users_email_idx", wantSubtype: "AT_ClusterOn", wantCmdName: "users_email_idx", wantHasDef: false},
		{name: "set_without_cluster", sql: "ALTER TABLE users SET WITHOUT CLUSTER", wantSubtype: "AT_DropCluster", wantCmdName: "", wantHasDef: false},
	}

	for _, tc := range cases {
		tree, err := pg_query.Parse(tc.sql)
		if err != nil {
			t.Fatalf("%s: parse failed: %v", tc.name, err)
		}

		atStmt := tree.GetStmts()[0].GetStmt().GetNode().(*pg_query.Node_AlterTableStmt).AlterTableStmt
		cmd := atStmt.GetCmds()[0].GetAlterTableCmd()

		subtype := cmd.GetSubtype().String()
		cmdName := cmd.GetName()
		hasDef := cmd.GetDef() != nil

		t.Logf("%-20s | subtype=%s | cmdName=%q | hasDef=%v", tc.name, subtype, cmdName, hasDef)

		if subtype != tc.wantSubtype {
			t.Errorf("%s: subtype = %q, want %q", tc.name, subtype, tc.wantSubtype)
		}
		if cmdName != tc.wantCmdName {
			t.Errorf("%s: cmdName = %q, want %q", tc.name, cmdName, tc.wantCmdName)
		}
		if hasDef != tc.wantHasDef {
			t.Errorf("%s: hasDef = %v, want %v", tc.name, hasDef, tc.wantHasDef)
		}

		t.Logf("  cmd proto: %s", cmd.String())
		t.Log("")
	}
}
