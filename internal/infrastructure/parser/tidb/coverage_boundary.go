// Package tidbparser marks parser-recognized but unaudited statement boundaries.
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
		if statement.DDL.Tablespace != nil && dialect == spec.DialectMySQL {
			decl := statement.DDL.Tablespace
			if decl.Kind == spec.TablespaceKindUndo && statement.DDL.Operation == spec.DDLOperationAlterTablespace {
				// `ALTER UNDO TABLESPACE ... SET` is recognized MySQL
				// surface but stays unaudited — a bounded boundary
				// marker, never a parser diagnostic nor an InnoDB
				// verdict (T25/#104 deferred scope).
				statement.Unsupported = &spec.UnsupportedDetail{
					Feature:  string(statement.DDL.Operation),
					Reason:   spec.UnsupportedUnauditedReason,
					Metadata: map[string]any{"form": "alter_undo_tablespace"},
				}
				return
			}
			// The LOGFILE GROUP family and every NDB-declared member are
			// NDB-only semantics — an engine boundary under MySQL, never
			// an audited InnoDB-tablespace verdict (T25/#104). NDBCLUSTER
			// is the documented NDB alias and routes identically.
			if decl.Kind == spec.TablespaceKindLogfileGroup || spec.TablespaceEngineNDB(decl.Engine) {
				metadata := map[string]any{"boundary": "engine"}
				if decl.Engine != "" {
					metadata["engine"] = decl.Engine
				}
				statement.Unsupported = &spec.UnsupportedDetail{
					Feature: string(statement.DDL.Operation), Reason: spec.UnsupportedVendorBoundaryReason,
					Metadata: metadata,
				}
				return
			}
		}
		if statement.DDL.Server != nil {
			// The MySQL SERVER family (T26/#105): on MySQL a named
			// unaudited boundary — recognition-level coverage only, no
			// lifecycle semantics; on TiDB the whole family is not TiDB
			// semantics — a vendor boundary, never a MySQL verdict.
			if dialect == spec.DialectTiDB {
				statement.Unsupported = &spec.UnsupportedDetail{
					Feature:  string(statement.DDL.Operation),
					Reason:   spec.UnsupportedVendorBoundaryReason,
					Metadata: map[string]any{"boundary": "vendor"},
				}
			} else {
				statement.Unsupported = &spec.UnsupportedDetail{
					Feature:  string(statement.DDL.Operation),
					Reason:   spec.UnsupportedUnauditedReason,
					Metadata: map[string]any{"boundary": "unaudited", "object": "server"},
				}
			}
			return
		}
		if statement.DDL.SpatialReference != nil {
			// The MySQL spatial reference system family (T26/#105): on
			// MySQL a named implementation boundary — the declaration is
			// recognized but complete SRS semantics stay unimplemented;
			// on TiDB the whole family is not TiDB semantics — a vendor
			// boundary, never a MySQL verdict.
			if dialect == spec.DialectTiDB {
				statement.Unsupported = &spec.UnsupportedDetail{
					Feature:  string(statement.DDL.Operation),
					Reason:   spec.UnsupportedVendorBoundaryReason,
					Metadata: map[string]any{"boundary": "vendor"},
				}
			} else {
				statement.Unsupported = &spec.UnsupportedDetail{
					Feature:  string(statement.DDL.Operation),
					Reason:   spec.UnsupportedUnauditedReason,
					Metadata: map[string]any{"boundary": "implementation", "object": "spatial_reference_system"},
				}
			}
			return
		}
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
			detail := &spec.UnsupportedDetail{
				Feature: feature,
				Reason:  spec.UnsupportedUnauditedReason,
			}
			if rg, ok := node.(*mysqlResourceGroupStatement); ok && rg.setFor && dialect == spec.DialectTiDB {
				// `SET RESOURCE GROUP ... FOR` is MySQL-only binding
				// syntax — vendor boundary under TiDB (T24/#103).
				detail.Reason = spec.UnsupportedVendorBoundaryReason
				detail.Metadata = map[string]any{"boundary": "vendor"}
			}
			if inst, ok := node.(*mysqlAlterInstanceStatement); ok && inst.mysqlOnly && dialect == spec.DialectTiDB {
				// ROTATE INNODB MASTER KEY is MySQL-only grammar — a
				// vendor boundary under TiDB (T26/#105).
				detail.Reason = spec.UnsupportedVendorBoundaryReason
				detail.Metadata = map[string]any{"boundary": "vendor"}
			}
			if inst, ok := node.(*ast.AlterInstanceStmt); ok && inst.NoRollbackOnError {
				// The NO ROLLBACK ON ERROR member is recorded, not
				// merged silently into the plain subaction (T26/#105).
				if detail.Metadata == nil {
					detail.Metadata = map[string]any{}
				}
				detail.Metadata["no_rollback_on_error"] = true
			}
			statement.Unsupported = detail
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
			spec.DDLOperationCreatePlacementPolicy, spec.DDLOperationAlterPlacementPolicy, spec.DDLOperationDropPlacementPolicy,
			spec.DDLOperationAlterRangePlacement,
			spec.DDLOperationCreateResourceGroup, spec.DDLOperationAlterResourceGroup:
			if ddl.MySQLResourceGroup != nil {
				// The MySQL CPU resource-group family is audited on its
				// own dialect — never a vendor boundary (T24/#103).
				return "", false, false
			}
			// TiDB RU resource-group options (RU_PER_SEC / PRIORITY /
			// BURSTABLE / QUERY_LIMIT) are not MySQL semantics — the
			// parsed form is unambiguously TiDB syntax, so MySQL keeps a
			// vendor boundary rather than a silent audited create. MySQL's
			// own CPU resource-group family is a separate surface (T24);
			// DROP is shared syntax and stays audited on both dialects.
			return string(ddl.Operation), true, true
		}
		if ddl.Operation == spec.DDLOperationDropResourceGroup && ddl.ResourceGroup != nil &&
			(ddl.ResourceGroup.IfExists || len(ddl.ResourceGroup.Options) > 0) {
			// A TiDB-path DROP carrying TiDB-only members (the IF EXISTS
			// guard MySQL's grammar lacks) is a vendor boundary under
			// MySQL — the statement is refused, not audited. The bare
			// shared form stays audited (T24/#103).
			return string(ddl.Operation), true, true
		}
		if ddl.Operation == spec.DDLOperationCreateIndex {
			if kind := ddlIndexKind(ddl); kind == spec.IndexKindVector || kind == spec.IndexKindColumnar {
				return string(ddl.Operation), true, true
			}
		}
		switch ddl.Operation {
		case spec.DDLOperationSplitRegion, spec.DDLOperationCompactTable:
			// SPLIT and COMPACT are TiDB physical-layout statements; the
			// shared parser still produces them under the mysql dialect,
			// so MySQL keeps a vendor boundary, never an audited verdict
			// (issue #102 T23-R8).
			return string(ddl.Operation), true, true
		}
	case spec.DialectTiDB:
		if ddl.MySQLResourceGroup != nil {
			// MySQL CPU resource-group syntax is not TiDB semantics —
			// vendor boundary, never an audited create (T24/#103).
			return string(ddl.Operation), true, true
		}
		if ddl.Tablespace != nil {
			// The whole MySQL tablespace family — data files, undo
			// tablespaces, LOGFILE GROUP — is not TiDB semantics:
			// vendor boundary, never an audited verdict (T25/#104).
			return string(ddl.Operation), true, true
		}
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
// family USE/session-scope SET/SET CHARSET/SET SESSION_STATES plus
// session-scope PREPARE/DEALLOCATE. Execution-capable or server-wide forms —
// EXPLAIN ANALYZE/EXPLORE, TRACE, EXECUTE, DO, BINLOG replay, SELECT INTO
// OUTFILE, and SET GLOBAL — are never excluded:
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
	case *ast.SetStmt:
		// Session-scoped assignments are exempt, but SET GLOBAL / @@global
		// assignments change server-wide settings for subsequent connections —
		// mutating administrative effects the audit does not model.
		for _, v := range n.Variables {
			if v != nil && v.IsGlobal {
				return "set_global"
			}
		}
		return ""
	case *ast.TraceStmt:
		return "trace"
	case *ast.ExecuteStmt:
		return "execute_prepared"
	case *ast.SetResourceGroupStmt:
		return "set_resource_group"
	case *mysqlResourceGroupStatement:
		if n.setFor {
			return "set_resource_group"
		}
		return ""
	case *mysqlAlterInstanceStatement:
		// The recognizer assigns the precise feature name at claim time
		// (T26/#105 inventory scope — ROTATE is T26-owned; the EXT
		// tracking label holds only the surplus subactions).
		return n.feature
	case *ast.AlterInstanceStmt:
		// This case only sees subactions the native grammar already
		// produced an AST for; syntax failures keep the parser-error
		// path. The inventory-owned RELOAD TLS subaction gets its
		// precise name; NO ROLLBACK ON ERROR stays the same subaction
		// with the member recorded in boundary metadata. Other
		// successfully-parsed subactions keep the generic name (T26).
		if n.ReloadTLS {
			return "alter_instance.reload_tls"
		}
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
// outer statement node. Parenthesized operands nest as *ast.SetOprSelectList
// rather than *ast.SetOprStmt, so both shapes recurse.
func setOprHasSelectInto(stmt *ast.SetOprStmt) bool {
	if stmt == nil || stmt.SelectList == nil {
		return false
	}
	return setOprListHasSelectInto(stmt.SelectList)
}

func setOprListHasSelectInto(list *ast.SetOprSelectList) bool {
	for _, inner := range list.Selects {
		switch n := inner.(type) {
		case *ast.SelectStmt:
			if n != nil && n.SelectIntoOpt != nil {
				return true
			}
		case *ast.SetOprStmt:
			if setOprHasSelectInto(n) {
				return true
			}
		case *ast.SetOprSelectList:
			if setOprListHasSelectInto(n) {
				return true
			}
		}
	}
	return false
}
