// Package tidbparser pins the coverage disposition of every statement-node
// candidate in the pinned TiDB parser.
// input: the parser's statement-node candidates (transitive stmtNode embedders
// plus direct statement() definers) and the real extractor/boundary switches
// output: drift detection when the parser adds or renames statement types, and
// a fail-closed check that every ast.StmtNode implementer is extracted,
// explicitly named as unsupported evidence, or deliberately exempt
// pos: coverage-boundary census for issue #82 (statement-type completeness)
// note: if this file changes, update this header and module README.md.
package tidbparser

import (
	goast "go/ast"
	goparser "go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
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
	"ProcedureInfo":             "extracted",
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
	"ProcedureBlock":            "procedure_body",
	"ProcedureIfBlock":          "procedure_body",
	"ProcedureElseIfBlock":      "procedure_body",
	"ProcedureElseBlock":        "procedure_body",
	"ProcedureLabelBlock":       "procedure_body",
	"ProcedureLabelLoop":        "procedure_body",
	"ProcedureJump":             "procedure_body",
	"ProcedureFetchInto":        "procedure_body",
	"ProcedureOpenCur":          "procedure_body",
	"ProcedureCloseCur":         "procedure_body",
	"ProcedureErrorCon":         "procedure_body",
	"ProcedureErrorState":       "procedure_body",
	"ProcedureErrorVal":         "procedure_body",
	"ProcedureIfInfo":           "procedure_body",

	// Exempt: read-only query forms, session state, transaction control.
	// SelectStmt/SetOprStmt gain the "select_into" feature when an INTO OUTFILE
	// clause is present; ExplainStmt gains explain_analyze/explain_explore;
	// SetStmt gains "set_global" when any assignment is server-wide
	// (IsGlobal/@@global) — those field-conditional paths are SQL-verified in
	// the T03 coverage tests.
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
	"ProcedureInfo":             reflect.TypeOf((*ast.ProcedureInfo)(nil)),
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
	"ProcedureBlock":            reflect.TypeOf((*ast.ProcedureBlock)(nil)),
	"ProcedureIfBlock":          reflect.TypeOf((*ast.ProcedureIfBlock)(nil)),
	"ProcedureElseIfBlock":      reflect.TypeOf((*ast.ProcedureElseIfBlock)(nil)),
	"ProcedureElseBlock":        reflect.TypeOf((*ast.ProcedureElseBlock)(nil)),
	"ProcedureLabelBlock":       reflect.TypeOf((*ast.ProcedureLabelBlock)(nil)),
	"ProcedureLabelLoop":        reflect.TypeOf((*ast.ProcedureLabelLoop)(nil)),
	"ProcedureJump":             reflect.TypeOf((*ast.ProcedureJump)(nil)),
	"ProcedureFetchInto":        reflect.TypeOf((*ast.ProcedureFetchInto)(nil)),
	"ProcedureOpenCur":          reflect.TypeOf((*ast.ProcedureOpenCur)(nil)),
	"ProcedureCloseCur":         reflect.TypeOf((*ast.ProcedureCloseCur)(nil)),
	"ProcedureErrorCon":         reflect.TypeOf((*ast.ProcedureErrorCon)(nil)),
	"ProcedureErrorState":       reflect.TypeOf((*ast.ProcedureErrorState)(nil)),
	"ProcedureErrorVal":         reflect.TypeOf((*ast.ProcedureErrorVal)(nil)),
	"ProcedureIfInfo":           reflect.TypeOf((*ast.ProcedureIfInfo)(nil)),
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

	// Structural carriers: embed a statement-node base for Visitor plumbing
	// but do not satisfy ast.StmtNode (no disposition — the dispatch test
	// asserts !Implements for them). They stay in this map so a parser
	// upgrade that promotes one to a real statement forces a disposition.
	"ProcedureErrorCondition":        reflect.TypeOf((*ast.ProcedureErrorCondition)(nil)),
	"SplitIndexOption":               reflect.TypeOf((*ast.SplitIndexOption)(nil)),
	"SplitOption":                    reflect.TypeOf((*ast.SplitOption)(nil)),
	"QueryWatchOption":               reflect.TypeOf((*ast.QueryWatchOption)(nil)),
	"DynamicCalibrateResourceOption": reflect.TypeOf((*ast.DynamicCalibrateResourceOption)(nil)),
}

// stmtNodeInterface is the interface every census candidate is checked
// against — interface satisfaction, not naming, decides the census set.
var stmtNodeInterface = reflect.TypeOf((*ast.StmtNode)(nil)).Elem()

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

// TestStatementTypeDisposition asserts every census candidate lands exactly
// where the coverage contract says it should. Candidates are instantiated and
// pushed through the real classify + boundary switches — the table cannot
// drift from the dispatch it describes. Structural carriers that embed a
// statement base without satisfying ast.StmtNode must carry no disposition.
func TestStatementTypeDisposition(t *testing.T) {
	for name, typ := range stmtNodeTypes {
		want, hasDisposition := stmtDispositions[name]
		if !typ.Implements(stmtNodeInterface) {
			// Embedded-but-not-implementer carrier (e.g. SplitIndexOption):
			// cannot be a parse result, so it must not claim a disposition.
			if hasDisposition {
				t.Errorf("%s: disposition %q but the type does not implement ast.StmtNode", name, want)
			}
			continue
		}
		if !hasDisposition {
			t.Errorf("ast.StmtNode implementer %s has no disposition-table entry — decide extracted/feature/exempt", name)
			continue
		}
		node, ok := reflect.New(typ.Elem()).Interface().(ast.StmtNode)
		if !ok {
			t.Fatalf("%s implements the interface but cannot be instantiated as ast.StmtNode", name)
		}
		if got := dispositionOf(node); got != want {
			t.Errorf("%s: want disposition %q, got %q", name, want, got)
		}
		if want == "extracted" && unhandledStatementFeature(node) != "" {
			// An extracted type must not also carry boundary evidence: the
			// classify switch wins, so the boundary case would be dead code
			// describing the wrong contract.
			t.Errorf("%s: extracted but boundary switch names it %q", name, unhandledStatementFeature(node))
		}
	}
	for name := range stmtDispositions {
		if _, ok := stmtNodeTypes[name]; !ok {
			t.Errorf("disposition entry %q has no type reference — dropped rows must leave both tables", name)
		}
	}
}

// TestExtractedTypesReachExtractor closes the census→dispatch connection:
// every type classified "extracted" must have a case in the real Extract type
// switch. classify() and the switch are separate lists, so this test — not
// the disposition table — is what fails if an extractor case is dropped.
func TestExtractedTypesReachExtractor(t *testing.T) {
	handled := scanExtractorCases(t)
	for name, disposition := range stmtDispositions {
		if disposition != "extracted" {
			continue
		}
		if !handled[name] {
			t.Errorf("%s is classified extracted but has no case in tidbExtractor.Extract", name)
		}
	}
	for name := range handled {
		if stmtDispositions[name] != "extracted" {
			t.Errorf("extractor case %s is not marked extracted in the disposition table", name)
		}
	}
}

// TestStatementTypeCensusDrift fails when the pinned parser gains or renames a
// statement-node candidate without a corresponding stmtNodeTypes row — this
// keeps the complete-coverage boundary closed across parser upgrades.
//
// Candidates are enumerated by structure, not naming convention: every struct
// that transitively embeds stmtNode/ddlNode/dmlNode, plus any type defining
// statement() directly, is a candidate. Whether a candidate is a real
// statement is decided by reflect Implements — that is how non-*Stmt
// implementers like ProcedureInfo (the CREATE PROCEDURE statement node) and
// non-statement carriers like SplitIndexOption are both kept inside the
// census boundary.
func TestStatementTypeCensusDrift(t *testing.T) {
	candidates := scanStmtCandidates(t, parserModuleDir(t))
	for name := range candidates {
		if _, ok := stmtNodeTypes[name]; !ok {
			t.Errorf("statement-node candidate %s missing from stmtNodeTypes — classify it as extracted/feature/exempt or a structural carrier", name)
		}
	}
	for name := range stmtNodeTypes {
		if !candidates[name] {
			t.Errorf("stmtNodeTypes entry %s does not exist in the pinned parser — drop or update the row", name)
		}
	}
}

// parserModuleDir resolves the on-disk directory of the pinned parser module.
func parserModuleDir(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "github.com/pingcap/tidb/pkg/parser").Output()
	if err != nil {
		t.Fatalf("locate parser module: %v", err)
	}
	return strings.TrimSpace(string(out))
}

// scanStmtCandidates parses every struct declaration in the parser's ast
// package with go/parser — not regex — and returns the census candidate set:
// types that transitively embed a statement-node base (stmtNode, ddlNode,
// dmlNode) or define statement() directly. Embedded fields are identified
// structurally (a field with no name), so comments, one-line declarations,
// pointer embeds, and generic receivers cannot evade the scan. Direct-marker
// types seed the closure before the transitive pass so their embedders count
// too. Some candidates satisfy ast.StmtNode; a few are structural carriers —
// the dispatch test separates the two by reflection.
func scanStmtCandidates(t *testing.T, parserDir string) map[string]bool {
	t.Helper()
	structs := map[string][]string{}
	candidates := map[string]bool{"stmtNode": true, "ddlNode": true, "dmlNode": true}
	entries, err := os.ReadDir(filepath.Join(parserDir, "ast"))
	if err != nil {
		t.Fatalf("read parser ast dir: %v", err)
	}
	fset := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := goparser.ParseFile(fset, filepath.Join(parserDir, "ast", entry.Name()), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", entry.Name(), err)
		}
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *goast.GenDecl:
				for _, specDecl := range d.Specs {
					ts, ok := specDecl.(*goast.TypeSpec)
					if !ok {
						continue
					}
					if ts.Assign.IsValid() {
						// `type A = T` aliases share the target's method set, so
						// an embedder of A is an embedder of T. Record the edge
						// so the closure resolves alias-mediated embeddings.
						if name := embeddedFieldName(ts.Type); name != "" {
							structs[ts.Name.Name] = append(structs[ts.Name.Name], name)
						}
						continue
					}
					st, ok := ts.Type.(*goast.StructType)
					if !ok {
						continue
					}
					for _, field := range st.Fields.List {
						if len(field.Names) != 0 {
							continue
						}
						if name := embeddedFieldName(field.Type); name != "" {
							structs[ts.Name.Name] = append(structs[ts.Name.Name], name)
						}
					}
				}
			case *goast.FuncDecl:
				// A type defining statement() itself is a candidate even
				// without an embedded base; seed it before the closure so
				// its embedders transitively count.
				if d.Recv == nil || len(d.Recv.List) == 0 || d.Name.Name != "statement" {
					continue
				}
				if name := embeddedFieldName(d.Recv.List[0].Type); name != "" &&
					name != "stmtNode" && name != "ddlNode" && name != "dmlNode" {
					candidates[name] = true
				}
			}
		}
	}
	for changed := true; changed; {
		changed = false
		for name, embeds := range structs {
			if candidates[name] {
				continue
			}
			for _, embedded := range embeds {
				if candidates[embedded] {
					candidates[name] = true
					changed = true
					break
				}
			}
		}
	}
	delete(candidates, "stmtNode")
	delete(candidates, "ddlNode")
	delete(candidates, "dmlNode")
	return candidates
}

// TestScanStmtCandidatesShapes pins the declaration forms the census scanner
// must discover: embedded bases with trailing comments, one-line struct
// declarations, unnamed and generic statement() receivers, embedders of
// direct-marker types, and embedders of local type aliases (an alias shares
// its target's method set, so alias-mediated embedding is real embedding).
// A regression here means a parser upgrade could
// introduce a live statement type the census never sees.
func TestScanStmtCandidatesShapes(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "ast"), 0o755); err != nil {
		t.Fatal(err)
	}
	source := `package ast

type stmtNode struct{}
type ddlNode struct{}
type dmlNode struct{}

type PlainStmt struct {
	stmtNode
}

type CommentedStmt struct {
	stmtNode // visitor base
}

type OneLineStmt struct{ ddlNode }

type UnnamedMarker struct{}

func (*UnnamedMarker) statement() {}

type GenericMarker[T any] struct {
	Value T
}

func (n *GenericMarker[T]) statement() {}

type DirectBase struct{}

func (n *DirectBase) statement() {}

type DirectChild struct {
	DirectBase
}

type BaseAlias = ddlNode

type AliasStmt struct {
	BaseAlias
}

type StmtAlias = PlainStmt

type AliasChild struct {
	StmtAlias
}
`
	if err := os.WriteFile(filepath.Join(dir, "ast", "nodes.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	got := scanStmtCandidates(t, dir)
	for _, name := range []string{
		"PlainStmt", "CommentedStmt", "OneLineStmt", "UnnamedMarker",
		"GenericMarker", "DirectBase", "DirectChild",
		"BaseAlias", "AliasStmt", "StmtAlias", "AliasChild",
	} {
		if !got[name] {
			t.Errorf("statement-node candidate %s missed by scanner", name)
		}
	}
}

// embeddedFieldName unwraps an anonymous field or method receiver type to the
// base type name: handles T, *T, pkg.T, *pkg.T, and generic receivers T[P]
// or *T[P].
func embeddedFieldName(expr goast.Expr) string {
	for {
		switch e := expr.(type) {
		case *goast.StarExpr:
			expr = e.X
		case *goast.IndexExpr:
			expr = e.X
		case *goast.IndexListExpr:
			expr = e.X
		case *goast.SelectorExpr:
			return e.Sel.Name
		case *goast.Ident:
			return e.Name
		default:
			return ""
		}
	}
}

// scanExtractorCases parses this package's extractor.go with go/parser and
// returns the AST type names handled by the Extract type switch — the real
// dispatch set that classify must stay aligned with.
func scanExtractorCases(t *testing.T) map[string]bool {
	t.Helper()
	fset := token.NewFileSet()
	file, err := goparser.ParseFile(fset, "extractor.go", nil, 0)
	if err != nil {
		t.Fatalf("parse extractor.go: %v", err)
	}
	handled := map[string]bool{}
	for _, decl := range file.Decls {
		fn, ok := decl.(*goast.FuncDecl)
		if !ok || fn.Name.Name != "Extract" || fn.Body == nil {
			continue
		}
		goast.Inspect(fn.Body, func(n goast.Node) bool {
			sw, ok := n.(*goast.TypeSwitchStmt)
			if !ok {
				return true
			}
			for _, stmt := range sw.Body.List {
				clause, ok := stmt.(*goast.CaseClause)
				if !ok {
					continue
				}
				for _, expr := range clause.List {
					if name := embeddedFieldName(expr); name != "" {
						handled[name] = true
					}
				}
			}
			return false
		})
	}
	return handled
}
