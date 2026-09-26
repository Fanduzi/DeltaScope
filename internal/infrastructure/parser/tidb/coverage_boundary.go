// Package tidbparser marks parser-recognized but unaudited statement boundaries.
// input: parsed TiDB AST nodes, extracted spec statements, and the selected dialect
// output: UnsupportedDetail markers for dialect vendor boundaries and unhandled parsed statements
// pos: infrastructure boundary classification for audit coverage reporting (issue #82)
// note: if this file changes, update this header and module README.md.
package tidbparser

import (
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	"github.com/pingcap/tidb/pkg/parser/ast"
)

// The shared reason vocabulary lives in spec so parser-attached boundaries and
// application aspect gaps carry identical bounded text.

// applyCoverageBoundary marks a statement Unsupported when the parsed node is
// recognized but outside the audited surface for the selected dialect, or when
// the node was classified but never extracted. Marked statements stay in the
// result stream; the application layer turns the marker into incomplete
// coverage and bounded unsupported evidence.
func applyCoverageBoundary(dialect spec.Dialect, statement *spec.Statement, node ast.StmtNode) {
	if statement.Unsupported != nil {
		return
	}
	if statement.DDL != nil {
		if feature, vendor, ok := dialectBoundaryFeature(dialect, statement.DDL); ok {
			detail := &spec.UnsupportedDetail{Feature: feature, Reason: boundaryReason(vendor)}
			if vendor {
				detail.Metadata = map[string]any{"boundary": "vendor"}
			}
			statement.Unsupported = detail
		}
		return
	}
	if statement.Kind == spec.KindUnknown {
		if feature := unhandledStatementFeature(node); feature != "" {
			statement.Unsupported = &spec.UnsupportedDetail{
				Feature: feature,
				Reason:  spec.UnsupportedUnauditedReason,
			}
		}
	}
}

func boundaryReason(vendor bool) string {
	if vendor {
		return spec.UnsupportedVendorBoundaryReason
	}
	return spec.UnsupportedUnauditedReason
}

// dialectBoundaryFeature reports the feature id for a recognized DDL operation
// that the selected dialect does not support (vendor boundary) or that carries
// an unmodeled primary object (parse-only).
func dialectBoundaryFeature(dialect spec.Dialect, ddl *spec.DDL) (feature string, vendor bool, ok bool) {
	switch dialect {
	case spec.DialectMySQL:
		switch ddl.Operation {
		case spec.DDLOperationCreateSequence, spec.DDLOperationAlterSequence, spec.DDLOperationDropSequence,
			spec.DDLOperationCreatePlacementPolicy, spec.DDLOperationAlterPlacementPolicy, spec.DDLOperationDropPlacementPolicy:
			return string(ddl.Operation), true, true
		}
		if ddl.Operation == spec.DDLOperationCreateIndex {
			if kind := ddlIndexKind(ddl); kind == spec.IndexKindVector || kind == spec.IndexKindColumnar {
				return string(ddl.Operation), true, true
			}
		}
	case spec.DialectTiDB:
		switch ddl.Operation {
		case spec.DDLOperationCreateProcedure, spec.DDLOperationDropProcedure:
			// TiDB's parser accepts procedure statements for compatibility, but
			// the product does not support stored procedures.
			return string(ddl.Operation), true, true
		case spec.DDLOperationCreateTable:
			if ddl.HasSelect {
				// TiDB does not document CREATE TABLE ... AS SELECT as supported.
				return "create_table_select", true, true
			}
		case spec.DDLOperationCreateIndex:
			switch kind := ddlIndexKind(ddl); kind {
			case spec.IndexKindFulltext, spec.IndexKindSpatial:
				return string(ddl.Operation), true, true
			case spec.IndexKindVector, spec.IndexKindColumnar:
				return string(ddl.Operation), false, true
			}
		}
	}
	return "", false, false
}

// ddlIndexKind reads the index kind carried by a standalone CREATE INDEX
// statement (modeled as a create_index alter action payload).
func ddlIndexKind(ddl *spec.DDL) spec.IndexKind {
	for _, alter := range ddl.Alter {
		if alter.Index != nil && alter.Index.Definition != nil {
			return alter.Index.Definition.Kind
		}
	}
	return spec.IndexKindUnknown
}

// unhandledStatementFeature names parser-recognized mutating or administrative
// statements that the extractor does not model. Read-only query, session, and
// transaction statements return "" and stay out of the audit surface. Named
// out-of-surface exclusions include SELECT/UNION/TABLE/VALUES query forms
// (unless they carry INTO OUTFILE), SHOW/DESCRIBE/HELP, plain EXPLAIN and
// EXPLAIN FOR CONNECTION (read-only plan inspection), transaction control
// (BEGIN/COMMIT/ROLLBACK/SAVEPOINT/RELEASE SAVEPOINT), and the session-state
// family USE/SET/SET CHARSET/SET SESSION_STATES plus session-scope
// PREPARE/DEALLOCATE. Execution-capable forms — EXPLAIN ANALYZE/EXPLORE, TRACE,
// EXECUTE, DO, BINLOG replay, and SELECT INTO OUTFILE — are never excluded:
// they run the wrapped statement, dynamic SQL, expressions, or file writes,
// so their effects cannot be audited.
func unhandledStatementFeature(node ast.StmtNode) string {
	switch n := node.(type) {
	case *ast.DoStmt:
		return "do"
	case *ast.BinlogStmt:
		// BINLOG 'base64' replays row events — a mutating statement, not a
		// read-only inspection form.
		return "binlog"
	case *ast.SelectStmt:
		if n.SelectIntoOpt != nil {
			return "select_into"
		}
		return ""
	case *ast.SetOprStmt:
		// UNION/INTERSECT/EXCEPT queries stay read-only, but an INTO OUTFILE
		// clause lands on the innermost trailing select of the set list.
		if setOprHasSelectInto(n) {
			return "select_into"
		}
		return ""
	case *ast.ExplainStmt:
		switch {
		case n.Analyze:
			return "explain_analyze"
		case n.Explore:
			return "explain_explore"
		default:
			return ""
		}
	case *ast.TraceStmt:
		return "trace"
	case *ast.ExecuteStmt:
		return "execute_prepared"
	case *ast.CreateResourceGroupStmt:
		return "create_resource_group"
	case *ast.AlterResourceGroupStmt:
		return "alter_resource_group"
	case *ast.SetResourceGroupStmt:
		return "set_resource_group"
	case *ast.AlterRangeStmt:
		return "alter_range_placement"
	case *ast.AlterInstanceStmt:
		return "alter_instance"
	case *ast.AdminStmt:
		return "admin"
	case *ast.FlashBackTableStmt:
		return "flashback_table"
	case *ast.FlashBackDatabaseStmt:
		return "flashback_database"
	case *ast.FlashBackToTimestampStmt:
		return "flashback_cluster"
	case *ast.RecoverTableStmt:
		return "recover_table"
	case *ast.CreateBindingStmt:
		return "create_binding"
	case *ast.DropBindingStmt:
		return "drop_binding"
	case *ast.SetBindingStmt:
		return "set_binding"
	case *ast.BRIEStmt:
		return "brie"
	case *ast.CalibrateResourceStmt:
		return "calibrate_resource"
	case *ast.CancelDistributionJobStmt:
		return "cancel_distribution_job"
	case *ast.CleanupTableLockStmt:
		return "cleanup_table_lock"
	case *ast.CompactTableStmt:
		return "compact_table"
	case *ast.CreateMaskingPolicyStmt:
		return "create_masking_policy"
	case *ast.CreateStatisticsStmt:
		return "create_statistics"
	case *ast.DropStatisticsStmt:
		return "drop_statistics"
	case *ast.DropStatsStmt:
		return "drop_stats"
	case *ast.LoadStatsStmt:
		return "load_stats"
	case *ast.LockStatsStmt:
		return "lock_stats"
	case *ast.UnlockStatsStmt:
		return "unlock_stats"
	case *ast.RefreshStatsStmt:
		return "refresh_stats"
	case *ast.DistributeTableStmt:
		return "distribute_table"
	case *ast.OptimizeTableStmt:
		return "optimize_table"
	case *ast.RepairTableStmt:
		return "repair_table"
	case *ast.AnalyzeTableStmt:
		return "analyze_table"
	case *ast.LockTablesStmt:
		return "lock_tables"
	case *ast.UnlockTablesStmt:
		return "unlock_tables"
	case *ast.GrantProxyStmt:
		return "grant_proxy"
	case *ast.GrantRoleStmt:
		return "grant_role"
	case *ast.RevokeRoleStmt:
		return "revoke_role"
	case *ast.RenameUserStmt:
		return "rename_user"
	case *ast.SetDefaultRoleStmt:
		return "set_default_role"
	case *ast.SetRoleStmt:
		return "set_role"
	case *ast.SetPwdStmt:
		return "set_pwd"
	case *ast.SetConfigStmt:
		return "set_config"
	case *ast.TrafficStmt:
		return "traffic"
	case *ast.CallStmt:
		return "call"
	case *ast.LoadDataStmt:
		return "load_data"
	case *ast.ImportIntoStmt:
		return "import_into"
	case *ast.ImportIntoActionStmt:
		return "import_into_action"
	case *ast.KillStmt:
		return "kill"
	case *ast.ShutdownStmt:
		return "shutdown"
	case *ast.RestartStmt:
		return "restart"
	case *ast.FlushStmt:
		return "flush"
	case *ast.SplitRegionStmt:
		return "split_region"
	case *ast.PlanReplayerStmt:
		return "plan_replayer"
	case *ast.RecommendIndexStmt:
		return "recommend_index"
	case *ast.NonTransactionalDMLStmt:
		return "non_transactional_dml"
	case *ast.AddQueryWatchStmt:
		return "add_query_watch"
	case *ast.DropQueryWatchStmt:
		return "drop_query_watch"
	case *ast.ProcedureWhileStmt, *ast.ProcedureRepeatStmt,
		*ast.SimpleCaseStmt, *ast.SimpleWhenThenStmt,
		*ast.SearchCaseStmt, *ast.SearchWhenThenStmt,
		*ast.ProcedureBlock, *ast.ProcedureIfBlock, *ast.ProcedureElseIfBlock,
		*ast.ProcedureElseBlock, *ast.ProcedureLabelBlock, *ast.ProcedureLabelLoop,
		*ast.ProcedureJump, *ast.ProcedureFetchInto, *ast.ProcedureOpenCur,
		*ast.ProcedureCloseCur, *ast.ProcedureErrorCon,
		*ast.ProcedureErrorState, *ast.ProcedureErrorVal,
		*ast.ProcedureIfInfo:
		return "procedure_body"
	default:
		return ""
	}
}

// setOprHasSelectInto reports whether a UNION/INTERSECT/EXCEPT statement
// carries an INTO OUTFILE clause on any inner select — the parser attaches the
// clause to the trailing select inside the set-operation list, not to the
// outer statement node.
func setOprHasSelectInto(stmt *ast.SetOprStmt) bool {
	if stmt.SelectList == nil {
		return false
	}
	for _, inner := range stmt.SelectList.Selects {
		if sel, ok := inner.(*ast.SelectStmt); ok && sel != nil && sel.SelectIntoOpt != nil {
			return true
		}
		if nested, ok := inner.(*ast.SetOprStmt); ok && setOprHasSelectInto(nested) {
			return true
		}
	}
	return false
}
