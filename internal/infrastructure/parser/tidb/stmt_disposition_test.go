// Package tidbparser pins the coverage disposition of every statement type in
// the pinned TiDB parser.
// input: the pinned parser's ast.*Stmt type registry plus extractor/boundary switches
// output: drift detection when the parser adds or renames statement types, and
// a fail-closed check that every recognized type is extracted, explicitly named
// as unsupported evidence, or deliberately exempt from the audit surface
// pos: coverage-boundary census for issue #82 (statement-type completeness)
// note: if this file changes, update this header and module README.md.
package tidbparser

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	"github.com/pingcap/tidb/pkg/parser/ast"
)

// stmtDisposition describes how one parsed statement type reaches (or avoids)
// the complete-coverage path:
//   - "extracted": the node maps to a spec model (DDL or DML)
//   - "exempt":    read-only/session/transaction — intentionally outside the
//     audit surface (KindUnknown, no unsupported marker)
//   - "<feature>": unhandledStatementFeature names bounded evidence
var stmtDispositions = map[string]string{
	// Extracted into the normalized model.
	"CreateTableStmt":           "extracted",
	"CreateViewStmt":            "extracted",
	"AlterTableStmt":            "extracted",
	"DropTableStmt":             "extracted",
	"TruncateTableStmt":         "extracted",
	"CreateDatabaseStmt":        "extracted",
	"DropDatabaseStmt":          "extracted",
	"AlterDatabaseStmt":         "extracted",
	"CreateIndexStmt":           "extracted",
	"DropIndexStmt":             "extracted",
	"RenameTableStmt":           "extracted",
	"DropProcedureStmt":         "extracted",
	"CreateUserStmt":            "extracted",
	"AlterUserStmt":             "extracted",
	"DropUserStmt":              "extracted",
	"GrantStmt":                 "extracted",
	"RevokeStmt":                "extracted",
	"DropResourceGroupStmt":     "extracted",
	"CreatePlacementPolicyStmt": "extracted",
	"AlterPlacementPolicyStmt":  "extracted",
	"DropPlacementPolicyStmt":   "extracted",
	"CreateSequenceStmt":        "extracted",
	"AlterSequenceStmt":         "extracted",
	"DropSequenceStmt":          "extracted",
	"InsertStmt":                "extracted",
	"UpdateStmt":                "extracted",
	"DeleteStmt":                "extracted",

	// Unhandled → named bounded evidence.
	"DoStmt":                    "do",
	"BinlogStmt":                "binlog",
	"TraceStmt":                 "trace",
	"ExecuteStmt":               "execute_prepared",
	"CreateResourceGroupStmt":   "create_resource_group",
	"AlterResourceGroupStmt":    "alter_resource_group",
	"SetResourceGroupStmt":      "set_resource_group",
	"AlterRangeStmt":            "alter_range_placement",
	"AlterInstanceStmt":         "alter_instance",
	"AdminStmt":                 "admin",
	"FlashBackTableStmt":        "flashback_table",
	"FlashBackDatabaseStmt":     "flashback_database",
	"FlashBackToTimestampStmt":  "flashback_cluster",
	"RecoverTableStmt":          "recover_table",
	"CreateBindingStmt":         "create_binding",
	"DropBindingStmt":           "drop_binding",
	"SetBindingStmt":            "set_binding",
	"BRIEStmt":                  "brie",
	"CalibrateResourceStmt":     "calibrate_resource",
	"CancelDistributionJobStmt": "cancel_distribution_job",
	"CleanupTableLockStmt":      "cleanup_table_lock",
	"CompactTableStmt":          "compact_table",
	"CreateMaskingPolicyStmt":   "create_masking_policy",
	"CreateStatisticsStmt":      "create_statistics",
	"DropStatisticsStmt":        "drop_statistics",
	"DropStatsStmt":             "drop_stats",
	"LoadStatsStmt":             "load_stats",
	"LockStatsStmt":             "lock_stats",
	"UnlockStatsStmt":           "unlock_stats",
	"RefreshStatsStmt":          "refresh_stats",
	"DistributeTableStmt":       "distribute_table",
	"OptimizeTableStmt":         "optimize_table",
	"RepairTableStmt":           "repair_table",
	"AnalyzeTableStmt":          "analyze_table",
	"LockTablesStmt":            "lock_tables",
	"UnlockTablesStmt":          "unlock_tables",
	"GrantProxyStmt":            "grant_proxy",
	"GrantRoleStmt":             "grant_role",
	"RevokeRoleStmt":            "revoke_role",
	"RenameUserStmt":            "rename_user",
	"SetDefaultRoleStmt":        "set_default_role",
	"SetRoleStmt":               "set_role",
	"SetPwdStmt":                "set_pwd",
	"SetConfigStmt":             "set_config",
	"TrafficStmt":               "traffic",
	"CallStmt":                  "call",
	"LoadDataStmt":              "load_data",
	"ImportIntoStmt":            "import_into",
	"ImportIntoActionStmt":      "import_into_action",
	"KillStmt":                  "kill",
	"ShutdownStmt":              "shutdown",
	"RestartStmt":               "restart",
	"FlushStmt":                 "flush",
	"SplitRegionStmt":           "split_region",
	"PlanReplayerStmt":          "plan_replayer",
	"RecommendIndexStmt":        "recommend_index",
	"NonTransactionalDMLStmt":   "non_transactional_dml",
	"AddQueryWatchStmt":         "add_query_watch",
	"DropQueryWatchStmt":        "drop_query_watch",
	"ProcedureWhileStmt":        "procedure_body",
	"ProcedureRepeatStmt":       "procedure_body",
	"SimpleCaseStmt":            "procedure_body",
	"SimpleWhenThenStmt":        "procedure_body",
	"SearchCaseStmt":            "procedure_body",
	"SearchWhenThenStmt":        "procedure_body",

	// Exempt: read-only query forms, session state, transaction control.
	// SelectStmt/SetOprStmt gain the "select_into" feature when an INTO OUTFILE
	// clause is present; ExplainStmt gains explain_analyze/explain_explore —
	// those paths are SQL-verified in the T03 coverage tests.
	"SelectStmt":           "exempt",
	"SetOprStmt":           "exempt",
	"ExplainStmt":          "exempt",
	"ExplainForStmt":       "exempt",
	"ShowStmt":             "exempt",
	"HelpStmt":             "exempt",
	"UseStmt":              "exempt",
	"SetStmt":              "exempt",
	"SetSessionStatesStmt": "exempt",
	"PrepareStmt":          "exempt",
	"DeallocateStmt":       "exempt",
	"BeginStmt":            "exempt",
	"CommitStmt":           "exempt",
	"RollbackStmt":         "exempt",
	"SavepointStmt":        "exempt",
	"ReleaseSavepointStmt": "exempt",
}

// stmtNodeTypes mirrors stmtDispositions with compile-time type references so
// a parser rename breaks the build rather than silently skipping a row.
var stmtNodeTypes = map[string]reflect.Type{
	"CreateTableStmt":           reflect.TypeOf((*ast.CreateTableStmt)(nil)),
	"CreateViewStmt":            reflect.TypeOf((*ast.CreateViewStmt)(nil)),
	"AlterTableStmt":            reflect.TypeOf((*ast.AlterTableStmt)(nil)),
	"DropTableStmt":             reflect.TypeOf((*ast.DropTableStmt)(nil)),
	"TruncateTableStmt":         reflect.TypeOf((*ast.TruncateTableStmt)(nil)),
	"CreateDatabaseStmt":        reflect.TypeOf((*ast.CreateDatabaseStmt)(nil)),
	"DropDatabaseStmt":          reflect.TypeOf((*ast.DropDatabaseStmt)(nil)),
	"AlterDatabaseStmt":         reflect.TypeOf((*ast.AlterDatabaseStmt)(nil)),
	"CreateIndexStmt":           reflect.TypeOf((*ast.CreateIndexStmt)(nil)),
	"DropIndexStmt":             reflect.TypeOf((*ast.DropIndexStmt)(nil)),
	"RenameTableStmt":           reflect.TypeOf((*ast.RenameTableStmt)(nil)),
	"DropProcedureStmt":         reflect.TypeOf((*ast.DropProcedureStmt)(nil)),
	"CreateUserStmt":            reflect.TypeOf((*ast.CreateUserStmt)(nil)),
	"AlterUserStmt":             reflect.TypeOf((*ast.AlterUserStmt)(nil)),
	"DropUserStmt":              reflect.TypeOf((*ast.DropUserStmt)(nil)),
	"GrantStmt":                 reflect.TypeOf((*ast.GrantStmt)(nil)),
	"RevokeStmt":                reflect.TypeOf((*ast.RevokeStmt)(nil)),
	"DropResourceGroupStmt":     reflect.TypeOf((*ast.DropResourceGroupStmt)(nil)),
	"CreatePlacementPolicyStmt": reflect.TypeOf((*ast.CreatePlacementPolicyStmt)(nil)),
	"AlterPlacementPolicyStmt":  reflect.TypeOf((*ast.AlterPlacementPolicyStmt)(nil)),
	"DropPlacementPolicyStmt":   reflect.TypeOf((*ast.DropPlacementPolicyStmt)(nil)),
	"CreateSequenceStmt":        reflect.TypeOf((*ast.CreateSequenceStmt)(nil)),
	"AlterSequenceStmt":         reflect.TypeOf((*ast.AlterSequenceStmt)(nil)),
	"DropSequenceStmt":          reflect.TypeOf((*ast.DropSequenceStmt)(nil)),
	"InsertStmt":                reflect.TypeOf((*ast.InsertStmt)(nil)),
	"UpdateStmt":                reflect.TypeOf((*ast.UpdateStmt)(nil)),
	"DeleteStmt":                reflect.TypeOf((*ast.DeleteStmt)(nil)),
	"DoStmt":                    reflect.TypeOf((*ast.DoStmt)(nil)),
	"BinlogStmt":                reflect.TypeOf((*ast.BinlogStmt)(nil)),
	"TraceStmt":                 reflect.TypeOf((*ast.TraceStmt)(nil)),
	"ExecuteStmt":               reflect.TypeOf((*ast.ExecuteStmt)(nil)),
	"CreateResourceGroupStmt":   reflect.TypeOf((*ast.CreateResourceGroupStmt)(nil)),
	"AlterResourceGroupStmt":    reflect.TypeOf((*ast.AlterResourceGroupStmt)(nil)),
	"SetResourceGroupStmt":      reflect.TypeOf((*ast.SetResourceGroupStmt)(nil)),
	"AlterRangeStmt":            reflect.TypeOf((*ast.AlterRangeStmt)(nil)),
	"AlterInstanceStmt":         reflect.TypeOf((*ast.AlterInstanceStmt)(nil)),
	"AdminStmt":                 reflect.TypeOf((*ast.AdminStmt)(nil)),
	"FlashBackTableStmt":        reflect.TypeOf((*ast.FlashBackTableStmt)(nil)),
	"FlashBackDatabaseStmt":     reflect.TypeOf((*ast.FlashBackDatabaseStmt)(nil)),
	"FlashBackToTimestampStmt":  reflect.TypeOf((*ast.FlashBackToTimestampStmt)(nil)),
	"RecoverTableStmt":          reflect.TypeOf((*ast.RecoverTableStmt)(nil)),
	"CreateBindingStmt":         reflect.TypeOf((*ast.CreateBindingStmt)(nil)),
	"DropBindingStmt":           reflect.TypeOf((*ast.DropBindingStmt)(nil)),
	"SetBindingStmt":            reflect.TypeOf((*ast.SetBindingStmt)(nil)),
	"BRIEStmt":                  reflect.TypeOf((*ast.BRIEStmt)(nil)),
	"CalibrateResourceStmt":     reflect.TypeOf((*ast.CalibrateResourceStmt)(nil)),
	"CancelDistributionJobStmt": reflect.TypeOf((*ast.CancelDistributionJobStmt)(nil)),
	"CleanupTableLockStmt":      reflect.TypeOf((*ast.CleanupTableLockStmt)(nil)),
	"CompactTableStmt":          reflect.TypeOf((*ast.CompactTableStmt)(nil)),
	"CreateMaskingPolicyStmt":   reflect.TypeOf((*ast.CreateMaskingPolicyStmt)(nil)),
	"CreateStatisticsStmt":      reflect.TypeOf((*ast.CreateStatisticsStmt)(nil)),
	"DropStatisticsStmt":        reflect.TypeOf((*ast.DropStatisticsStmt)(nil)),
	"DropStatsStmt":             reflect.TypeOf((*ast.DropStatsStmt)(nil)),
	"LoadStatsStmt":             reflect.TypeOf((*ast.LoadStatsStmt)(nil)),
	"LockStatsStmt":             reflect.TypeOf((*ast.LockStatsStmt)(nil)),
	"UnlockStatsStmt":           reflect.TypeOf((*ast.UnlockStatsStmt)(nil)),
	"RefreshStatsStmt":          reflect.TypeOf((*ast.RefreshStatsStmt)(nil)),
	"DistributeTableStmt":       reflect.TypeOf((*ast.DistributeTableStmt)(nil)),
	"OptimizeTableStmt":         reflect.TypeOf((*ast.OptimizeTableStmt)(nil)),
	"RepairTableStmt":           reflect.TypeOf((*ast.RepairTableStmt)(nil)),
	"AnalyzeTableStmt":          reflect.TypeOf((*ast.AnalyzeTableStmt)(nil)),
	"LockTablesStmt":            reflect.TypeOf((*ast.LockTablesStmt)(nil)),
	"UnlockTablesStmt":          reflect.TypeOf((*ast.UnlockTablesStmt)(nil)),
	"GrantProxyStmt":            reflect.TypeOf((*ast.GrantProxyStmt)(nil)),
	"GrantRoleStmt":             reflect.TypeOf((*ast.GrantRoleStmt)(nil)),
	"RevokeRoleStmt":            reflect.TypeOf((*ast.RevokeRoleStmt)(nil)),
	"RenameUserStmt":            reflect.TypeOf((*ast.RenameUserStmt)(nil)),
	"SetDefaultRoleStmt":        reflect.TypeOf((*ast.SetDefaultRoleStmt)(nil)),
	"SetRoleStmt":               reflect.TypeOf((*ast.SetRoleStmt)(nil)),
	"SetPwdStmt":                reflect.TypeOf((*ast.SetPwdStmt)(nil)),
	"SetConfigStmt":             reflect.TypeOf((*ast.SetConfigStmt)(nil)),
	"TrafficStmt":               reflect.TypeOf((*ast.TrafficStmt)(nil)),
	"CallStmt":                  reflect.TypeOf((*ast.CallStmt)(nil)),
	"LoadDataStmt":              reflect.TypeOf((*ast.LoadDataStmt)(nil)),
	"ImportIntoStmt":            reflect.TypeOf((*ast.ImportIntoStmt)(nil)),
	"ImportIntoActionStmt":      reflect.TypeOf((*ast.ImportIntoActionStmt)(nil)),
	"KillStmt":                  reflect.TypeOf((*ast.KillStmt)(nil)),
	"ShutdownStmt":              reflect.TypeOf((*ast.ShutdownStmt)(nil)),
	"RestartStmt":               reflect.TypeOf((*ast.RestartStmt)(nil)),
	"FlushStmt":                 reflect.TypeOf((*ast.FlushStmt)(nil)),
	"SplitRegionStmt":           reflect.TypeOf((*ast.SplitRegionStmt)(nil)),
	"PlanReplayerStmt":          reflect.TypeOf((*ast.PlanReplayerStmt)(nil)),
	"RecommendIndexStmt":        reflect.TypeOf((*ast.RecommendIndexStmt)(nil)),
	"NonTransactionalDMLStmt":   reflect.TypeOf((*ast.NonTransactionalDMLStmt)(nil)),
	"AddQueryWatchStmt":         reflect.TypeOf((*ast.AddQueryWatchStmt)(nil)),
	"DropQueryWatchStmt":        reflect.TypeOf((*ast.DropQueryWatchStmt)(nil)),
	"ProcedureWhileStmt":        reflect.TypeOf((*ast.ProcedureWhileStmt)(nil)),
	"ProcedureRepeatStmt":       reflect.TypeOf((*ast.ProcedureRepeatStmt)(nil)),
	"SimpleCaseStmt":            reflect.TypeOf((*ast.SimpleCaseStmt)(nil)),
	"SimpleWhenThenStmt":        reflect.TypeOf((*ast.SimpleWhenThenStmt)(nil)),
	"SearchCaseStmt":            reflect.TypeOf((*ast.SearchCaseStmt)(nil)),
	"SearchWhenThenStmt":        reflect.TypeOf((*ast.SearchWhenThenStmt)(nil)),
	"SelectStmt":                reflect.TypeOf((*ast.SelectStmt)(nil)),
	"SetOprStmt":                reflect.TypeOf((*ast.SetOprStmt)(nil)),
	"ExplainStmt":               reflect.TypeOf((*ast.ExplainStmt)(nil)),
	"ExplainForStmt":            reflect.TypeOf((*ast.ExplainForStmt)(nil)),
	"ShowStmt":                  reflect.TypeOf((*ast.ShowStmt)(nil)),
	"HelpStmt":                  reflect.TypeOf((*ast.HelpStmt)(nil)),
	"UseStmt":                   reflect.TypeOf((*ast.UseStmt)(nil)),
	"SetStmt":                   reflect.TypeOf((*ast.SetStmt)(nil)),
	"SetSessionStatesStmt":      reflect.TypeOf((*ast.SetSessionStatesStmt)(nil)),
	"PrepareStmt":               reflect.TypeOf((*ast.PrepareStmt)(nil)),
	"DeallocateStmt":            reflect.TypeOf((*ast.DeallocateStmt)(nil)),
	"BeginStmt":                 reflect.TypeOf((*ast.BeginStmt)(nil)),
	"CommitStmt":                reflect.TypeOf((*ast.CommitStmt)(nil)),
	"RollbackStmt":              reflect.TypeOf((*ast.RollbackStmt)(nil)),
	"SavepointStmt":             reflect.TypeOf((*ast.SavepointStmt)(nil)),
	"ReleaseSavepointStmt":      reflect.TypeOf((*ast.ReleaseSavepointStmt)(nil)),
}

// dispositionOf resolves the coverage disposition of one parsed node through
// the same switches extraction and boundary marking use.
func dispositionOf(node ast.StmtNode) string {
	if kind := classify(node); kind != spec.KindUnknown {
		return "extracted"
	}
	if feature := unhandledStatementFeature(node); feature != "" {
		return feature
	}
	return "exempt"
}

// TestStatementTypeDisposition asserts every statement type in the pinned
// parser lands exactly where the coverage contract says it should: extracted
// into the model, named as unsupported evidence, or deliberately exempt.
func TestStatementTypeDisposition(t *testing.T) {
	for name, want := range stmtDispositions {
		typ, ok := stmtNodeTypes[name]
		if !ok {
			t.Fatalf("disposition table entry %q missing its type reference", name)
		}
		node, ok := reflect.New(typ.Elem()).Interface().(ast.StmtNode)
		if !ok {
			t.Fatalf("%s does not implement ast.StmtNode", name)
		}
		if got := dispositionOf(node); got != want {
			t.Errorf("%s: want disposition %q, got %q", name, want, got)
		}
	}
}

// TestStatementTypeCensusDrift fails when the pinned parser gains or renames a
// *ast.*Stmt type without a corresponding disposition-table update — this keeps
// the complete-coverage boundary closed across parser upgrades.
func TestStatementTypeCensusDrift(t *testing.T) {
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "github.com/pingcap/tidb/pkg/parser").Output()
	if err != nil {
		t.Fatalf("locate parser module: %v", err)
	}
	parserDir := strings.TrimSpace(string(out))
	pattern := regexp.MustCompile(`^type (\w+Stmt) struct`)
	blockComment := regexp.MustCompile(`(?s)/\*.*?\*/`)
	seen := map[string]bool{}
	entries, err := os.ReadDir(filepath.Join(parserDir, "ast"))
	if err != nil {
		t.Fatalf("read parser ast dir: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(parserDir, "ast", entry.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", entry.Name(), err)
		}
		// Dead types inside block comments (e.g. SetCharsetStmt) must not
		// count as live parser surface.
		source := blockComment.ReplaceAllString(string(data), "")
		for _, line := range strings.Split(source, "\n") {
			if m := pattern.FindStringSubmatch(line); m != nil {
				seen[m[1]] = true
			}
		}
	}
	for name := range seen {
		if _, ok := stmtDispositions[name]; !ok {
			t.Errorf("parser type %s has no disposition-table entry — decide extracted/feature/exempt", name)
		}
	}
	for name := range stmtDispositions {
		if !seen[name] {
			t.Errorf("disposition entry %s no longer exists in the pinned parser — drop or update the row", name)
		}
	}
}
