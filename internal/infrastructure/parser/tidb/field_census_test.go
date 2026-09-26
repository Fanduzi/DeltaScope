// Package tidbparser pins a disposition for every field of every census-scope
// AST struct the extractor can reach.
// input: the pinned parser's struct fields on extracted statement types and
// their option/fact carriers, plus the extractor's field reads
// output: a fail-closed check that no exported field on an audited-path struct
// is silently dropped — every field is projected, evidence, carried, subsumed,
// exempt, or explicitly deferred
// pos: coverage-boundary census for issue #82 (field-level completeness)
// note: if this file changes, update this header and module README.md.
package tidbparser

import (
	"reflect"
	"strings"
	"testing"

	"github.com/pingcap/tidb/pkg/parser/ast"
)

// Field disposition vocabulary:
//
//	projected            — extractor reads the field into the normalized model
//	projected_evidence   — read into the model AND feeds bounded gap evidence
//	evidence:<name>      — dropped, but recorded as a bounded option/marker name
//	carried:<parent>     — value field consumed under a projected/evidenced parent
//	subsumed:<reason>    — detail riding an already-flagged unaudited scope
//	exempt:<reason>      — structural/existence/internal flag with no audit fact
//	deferred:<reason>    — consciously outside the coverage contract (ADR part-6)
//
// A field that ends up unlisted fails the census — that is the point: a parser
// upgrade adding a field to a census struct cannot silently become complete.

// fieldCensusScope lists every AST struct whose fields are dispositioned. The
// reachability test below extends this boundary mechanically: any ast-package
// struct reachable through a field must join the scope or fieldReachExempt.
var fieldCensusTypes = map[string]reflect.Type{
	"CreateTableStmt":           reflect.TypeOf(ast.CreateTableStmt{}),
	"CreateViewStmt":            reflect.TypeOf(ast.CreateViewStmt{}),
	"AlterTableStmt":            reflect.TypeOf(ast.AlterTableStmt{}),
	"DropTableStmt":             reflect.TypeOf(ast.DropTableStmt{}),
	"TruncateTableStmt":         reflect.TypeOf(ast.TruncateTableStmt{}),
	"CreateDatabaseStmt":        reflect.TypeOf(ast.CreateDatabaseStmt{}),
	"DropDatabaseStmt":          reflect.TypeOf(ast.DropDatabaseStmt{}),
	"AlterDatabaseStmt":         reflect.TypeOf(ast.AlterDatabaseStmt{}),
	"CreateIndexStmt":           reflect.TypeOf(ast.CreateIndexStmt{}),
	"DropIndexStmt":             reflect.TypeOf(ast.DropIndexStmt{}),
	"RenameTableStmt":           reflect.TypeOf(ast.RenameTableStmt{}),
	"ProcedureInfo":             reflect.TypeOf(ast.ProcedureInfo{}),
	"DropProcedureStmt":         reflect.TypeOf(ast.DropProcedureStmt{}),
	"CreateUserStmt":            reflect.TypeOf(ast.CreateUserStmt{}),
	"AlterUserStmt":             reflect.TypeOf(ast.AlterUserStmt{}),
	"DropUserStmt":              reflect.TypeOf(ast.DropUserStmt{}),
	"GrantStmt":                 reflect.TypeOf(ast.GrantStmt{}),
	"RevokeStmt":                reflect.TypeOf(ast.RevokeStmt{}),
	"DropResourceGroupStmt":     reflect.TypeOf(ast.DropResourceGroupStmt{}),
	"CreatePlacementPolicyStmt": reflect.TypeOf(ast.CreatePlacementPolicyStmt{}),
	"AlterPlacementPolicyStmt":  reflect.TypeOf(ast.AlterPlacementPolicyStmt{}),
	"DropPlacementPolicyStmt":   reflect.TypeOf(ast.DropPlacementPolicyStmt{}),
	"CreateSequenceStmt":        reflect.TypeOf(ast.CreateSequenceStmt{}),
	"AlterSequenceStmt":         reflect.TypeOf(ast.AlterSequenceStmt{}),
	"DropSequenceStmt":          reflect.TypeOf(ast.DropSequenceStmt{}),
	"InsertStmt":                reflect.TypeOf(ast.InsertStmt{}),
	"UpdateStmt":                reflect.TypeOf(ast.UpdateStmt{}),
	"DeleteStmt":                reflect.TypeOf(ast.DeleteStmt{}),

	// Option/fact carriers the extractor traverses.
	"AlterTableSpec":           reflect.TypeOf(ast.AlterTableSpec{}),
	"Constraint":               reflect.TypeOf(ast.Constraint{}),
	"ReferenceDef":             reflect.TypeOf(ast.ReferenceDef{}),
	"IndexPartSpecification":   reflect.TypeOf(ast.IndexPartSpecification{}),
	"IndexOption":              reflect.TypeOf(ast.IndexOption{}),
	"ColumnOption":             reflect.TypeOf(ast.ColumnOption{}),
	"ColumnDef":                reflect.TypeOf(ast.ColumnDef{}),
	"ColumnPosition":           reflect.TypeOf(ast.ColumnPosition{}),
	"ColumnName":               reflect.TypeOf(ast.ColumnName{}),
	"TableOption":              reflect.TypeOf(ast.TableOption{}),
	"DatabaseOption":           reflect.TypeOf(ast.DatabaseOption{}),
	"PartitionOptions":         reflect.TypeOf(ast.PartitionOptions{}),
	"PartitionMethod":          reflect.TypeOf(ast.PartitionMethod{}),
	"PartitionDefinition":      reflect.TypeOf(ast.PartitionDefinition{}),
	"SubPartitionDefinition":   reflect.TypeOf(ast.SubPartitionDefinition{}),
	"UserSpec":                 reflect.TypeOf(ast.UserSpec{}),
	"AuthOption":               reflect.TypeOf(ast.AuthOption{}),
	"AuthTokenOrTLSOption":     reflect.TypeOf(ast.AuthTokenOrTLSOption{}),
	"PasswordOrLockOption":     reflect.TypeOf(ast.PasswordOrLockOption{}),
	"ResourceOption":           reflect.TypeOf(ast.ResourceOption{}),
	"CommentOrAttributeOption": reflect.TypeOf(ast.CommentOrAttributeOption{}),
	"ResourceGroupNameOption":  reflect.TypeOf(ast.ResourceGroupNameOption{}),
	"PrivElem":                 reflect.TypeOf(ast.PrivElem{}),
	"GrantLevel":               reflect.TypeOf(ast.GrantLevel{}),
	"SequenceOption":           reflect.TypeOf(ast.SequenceOption{}),
	"PlacementOption":          reflect.TypeOf(ast.PlacementOption{}),
	"SplitIndexOption":         reflect.TypeOf(ast.SplitIndexOption{}),
	"SplitOption":              reflect.TypeOf(ast.SplitOption{}),
	"TableToTable":             reflect.TypeOf(ast.TableToTable{}),
	"TableName":                reflect.TypeOf(ast.TableName{}),
	"StoreParameter":           reflect.TypeOf(ast.StoreParameter{}),
	"OnDeleteOpt":              reflect.TypeOf(ast.OnDeleteOpt{}),
	"OnUpdateOpt":              reflect.TypeOf(ast.OnUpdateOpt{}),
	"AlterOrderItem":           reflect.TypeOf(ast.AlterOrderItem{}),
	"TiFlashReplicaSpec":       reflect.TypeOf(ast.TiFlashReplicaSpec{}),
	"StatisticsSpec":           reflect.TypeOf(ast.StatisticsSpec{}),
	"AttributesSpec":           reflect.TypeOf(ast.AttributesSpec{}),
	"StatsOptionsSpec":         reflect.TypeOf(ast.StatsOptionsSpec{}),
	"AutoRandomOption":         reflect.TypeOf(ast.AutoRandomOption{}),
}

// fieldReachExempt lists ast-package struct types reachable through census
// fields but deliberately outside the field-level scope, each with its reason.
// Interface-typed fields (ExprNode/StmtNode/ResultSetNode/ValueExpr/
// PartitionDefinitionClause) are declared boundaries, not struct fields, and
// do not participate in reachability.
var fieldReachExempt = map[string]string{
	// Embedded node bases — visitor plumbing, never audit facts.
	"node":     "embedded node base",
	"stmtNode": "embedded statement base",
	"ddlNode":  "embedded statement base",
	"dmlNode":  "embedded statement base",
	// DML-context clause trees — outside the DDL-scoped coverage contract; the
	// statement-level fields that hold them are individually dispositioned.
	"TableRefsClause":    "dml context — mutation-table extraction consumes the join tree",
	"Join":               "dml context — join tree consumed by mutation-table extraction",
	"DeleteTableList":    "dml context — delete target list consumed by mutation-table extraction",
	"Assignment":         "dml context — SET/ON DUPLICATE assignments are DML surface",
	"SelectField":        "dml context — RETURNING field list is DML surface",
	"TableOptimizerHint": "dml context — optimizer hints are DML surface",
	"WithClause":         "dml context — CTE clauses are DML surface",
	"OrderByClause":      "dml context — ORDER BY is DML surface",
	"Limit":              "dml context — LIMIT is DML surface",
	"IndexHint":          "dml context — index hints attach to query-time table references",
	"TableSample":        "dml context — TABLESAMPLE is query syntax",
	"AsOfClause":         "dml context — FOR SYSTEM_TIME AS OF is query syntax",
	// Value carriers consumed under an already-flagged parent.
	"TimeUnitExpr":                   "option value expression — consumed under the parent option name",
	"PartitionKeyAlgorithm":          "subsumed — detail of the flagged partition clause",
	"PartitionInterval":              "subsumed — detail of the flagged partition clause",
	"CIStr":                          "identifier pair (original/lowered) — projected via .L wherever names are read",
	"IndexLockAndAlgorithm":          "subsumed — member detail under the lock_algorithm evidence",
	"MaskingPolicyState":             "subsumed — detail of flagged masking-policy actions",
	"MaskingPolicyRestrictOps":       "subsumed — detail of flagged masking-policy actions",
	"QueryWatchOption":               "structural carrier — embeds stmtNode for visitor plumbing, never a statement",
	"DynamicCalibrateResourceOption": "structural carrier — embeds stmtNode for visitor plumbing, never a statement",
}

var fieldDispositions = map[string]map[string]string{
	"CreateTableStmt": {
		"ddlNode":          "exempt:embedded_base",
		"IfNotExists":      "exempt:existence_flag — failure-mode modifier, object-set semantics unchanged",
		"TemporaryKeyword": "projected",
		"OnCommitDelete":   "projected",
		"Table":            "projected",
		"ReferTable":       "projected — HasReferTable feeds ddl.table.create_like.forbid",
		"Cols":             "projected",
		"Constraints":      "projected_evidence — constraint kinds and key parts",
		"SplitIndex":       "evidence:split_index",
		"Options":          "projected_evidence — modeled keys plus unextracted names",
		"Partition":        "projected_evidence — HasPartition feeds the forbid rule; nested option names walked",
		"OnDuplicate":      "exempt:grammar_unreachable — probed: CREATE TABLE ON DUPLICATE is a parser error",
		"Select":           "projected — HasSelect feeds ddl.table.create_as.forbid",
	},
	"CreateViewStmt": {
		"ddlNode":     "exempt:embedded_base",
		"OrReplace":   "evidence:or_replace",
		"ViewName":    "projected",
		"Cols":        "evidence:view_columns",
		"Select":      "projected — HasSelect; body semantics are rule-audited",
		"SchemaCols":  "evidence:view_columns — covered by the same option name",
		"Algorithm":   "evidence:view_algorithm — non-UNDEFINED forms only (parser fills defaults)",
		"Definer":     "evidence:definer — named non-CURRENT_USER identity only",
		"Security":    "evidence:sql_security — non-DEFINER forms only",
		"CheckOption": "evidence:check_option — LOCAL only (CASCADED is the parser default)",
	},
	"AlterTableStmt": {
		"ddlNode": "exempt:embedded_base",
		"Table":   "projected",
		"Specs":   "projected_evidence — per-spec action names plus option payloads",
	},
	"DropTableStmt": {
		"ddlNode":          "exempt:embedded_base",
		"IfExists":         "projected — options.if_exists",
		"Tables":           "projected — multi-target Targets",
		"IsView":           "projected — drop_table vs drop_view operation",
		"TemporaryKeyword": "projected",
	},
	"TruncateTableStmt": {
		"ddlNode": "exempt:embedded_base",
		"Table":   "projected",
	},
	"CreateDatabaseStmt": {
		"ddlNode":     "exempt:embedded_base",
		"IfNotExists": "projected — options.if_not_exists",
		"Name":        "projected",
		"Options":     "projected_evidence — charset/collate modeled, rest named",
	},
	"DropDatabaseStmt": {
		"ddlNode":  "exempt:embedded_base",
		"IfExists": "projected — options.if_exists",
		"Name":     "projected",
	},
	"AlterDatabaseStmt": {
		"ddlNode":              "exempt:embedded_base",
		"Name":                 "projected",
		"AlterDefaultDatabase": "exempt:grammar_unreachable — probed: ALTER DEFAULT DATABASE is a parser error",
		"Options":              "projected_evidence",
	},
	"CreateIndexStmt": {
		"ddlNode":                 "exempt:embedded_base",
		"IfNotExists":             "exempt:existence_flag",
		"IndexName":               "projected",
		"Table":                   "projected",
		"IndexPartSpecifications": "projected_evidence — columns plus expr/prefix/desc counts",
		"IndexOption":             "projected_evidence — global/predicate plus unmodeled member names",
		"KeyType":                 "projected — index kind",
		"LockAlg":                 "evidence:lock_algorithm",
	},
	"DropIndexStmt": {
		"ddlNode":   "exempt:embedded_base",
		"IfExists":  "exempt:existence_flag",
		"IndexName": "projected",
		"Table":     "projected",
		"LockAlg":   "evidence:lock_algorithm",
		"IsHypo":    "exempt:grammar_unreachable — hypothetical-index flag has no production in the pinned grammar",
	},
	"RenameTableStmt": {
		"ddlNode":       "exempt:embedded_base",
		"TableToTables": "projected",
	},
	"ProcedureInfo": {
		"stmtNode":          "exempt:embedded_base",
		"IfNotExists":       "exempt:existence_flag",
		"ProcedureName":     "projected",
		"ProcedureParam":    "evidence:params",
		"ProcedureBody":     "evidence:has_body",
		"ProcedureParamStr": "exempt:raw_source_duplicate — parser-kept restore text, not a fact",
	},
	"DropProcedureStmt": {
		"stmtNode":      "exempt:embedded_base",
		"IfExists":      "exempt:existence_flag",
		"ProcedureName": "projected",
	},
	"CreateUserStmt": {
		"stmtNode":                 "exempt:embedded_base",
		"IsCreateRole":             "projected — role vs user operation",
		"IfNotExists":              "exempt:existence_flag",
		"Specs":                    "projected_evidence — first target plus omitted counts plus per-spec auth evidence",
		"AuthTokenOrTLSOptions":    "evidence:auth_token_or_tls",
		"ResourceOptions":          "evidence:resource",
		"PasswordOrLockOptions":    "evidence:password_or_lock",
		"CommentOrAttributeOption": "evidence:comment_or_attribute",
		"ResourceGroupNameOption":  "evidence:resource_group_name",
	},
	"AlterUserStmt": {
		"stmtNode":                  "exempt:embedded_base",
		"IfExists":                  "exempt:existence_flag",
		"CurrentAuth":               "projected_evidence — USER() target path plus identified evidence",
		"CurrentDualPasswordOption": "projected_evidence — USER() dual-password target carrier",
		"Specs":                     "projected_evidence",
		"AuthTokenOrTLSOptions":     "evidence:auth_token_or_tls",
		"ResourceOptions":           "evidence:resource",
		"PasswordOrLockOptions":     "evidence:password_or_lock",
		"CommentOrAttributeOption":  "evidence:comment_or_attribute",
		"ResourceGroupNameOption":   "evidence:resource_group_name",
	},
	"DropUserStmt": {
		"stmtNode":   "exempt:embedded_base",
		"IfExists":   "exempt:existence_flag",
		"IsDropRole": "projected",
		"UserList":   "projected_evidence — names plus omitted-target count",
	},
	"GrantStmt": {
		"stmtNode":              "exempt:embedded_base",
		"Privs":                 "projected_evidence — privilege names projected, column privileges evidenced",
		"ObjectType":            "evidence:routine_object — FUNCTION|PROCEDURE grants",
		"Level":                 "projected — grant scope",
		"Users":                 "projected_evidence — omitted target count",
		"AuthTokenOrTLSOptions": "evidence:require_tls",
		"WithGrant":             "evidence:with_grant",
	},
	"RevokeStmt": {
		"stmtNode":   "exempt:embedded_base",
		"Privs":      "projected_evidence",
		"ObjectType": "evidence:routine_object",
		"Level":      "projected",
		"Users":      "projected_evidence — omitted target count",
	},
	"DropResourceGroupStmt": {
		"ddlNode":           "exempt:embedded_base",
		"IfExists":          "exempt:existence_flag",
		"ResourceGroupName": "projected",
	},
	"CreatePlacementPolicyStmt": {
		"ddlNode":          "exempt:embedded_base",
		"OrReplace":        "evidence:or_replace",
		"IfNotExists":      "exempt:existence_flag",
		"PolicyName":       "projected",
		"PlacementOptions": "evidence:has_options",
	},
	"AlterPlacementPolicyStmt": {
		"ddlNode":          "exempt:embedded_base",
		"PolicyName":       "projected",
		"IfExists":         "exempt:existence_flag",
		"PlacementOptions": "evidence:has_options",
	},
	"DropPlacementPolicyStmt": {
		"ddlNode":    "exempt:embedded_base",
		"IfExists":   "exempt:existence_flag",
		"PolicyName": "projected",
	},
	"CreateSequenceStmt": {
		"ddlNode":     "exempt:embedded_base",
		"IfNotExists": "exempt:existence_flag",
		"Name":        "projected",
		"SeqOptions":  "evidence:has_options",
		"TblOptions":  "evidence:<tableOptionName> — shared table-option tail named per member",
	},
	"AlterSequenceStmt": {
		"ddlNode":    "exempt:embedded_base",
		"Name":       "projected",
		"IfExists":   "exempt:existence_flag",
		"SeqOptions": "evidence:has_options",
	},
	"DropSequenceStmt": {
		"ddlNode":   "exempt:embedded_base",
		"IfExists":  "exempt:existence_flag",
		"Sequences": "projected_evidence — first target plus omitted count",
	},
	"InsertStmt": {
		"dmlNode":        "exempt:embedded_base",
		"IsReplace":      "projected",
		"IgnoreErr":      "deferred:dml_modifier — INSERT IGNORE is DML surface (rules-audited rows)",
		"Table":          "projected — mutation tables",
		"Columns":        "deferred:dml_surface — column list not in the DML audit contract",
		"Lists":          "projected — InsertRows count",
		"Setlist":        "deferred:dml_surface",
		"Priority":       "deferred:dml_modifier",
		"OnDuplicate":    "projected — HasOnDuplicate",
		"Select":         "projected — IsInsertSelect",
		"TableHints":     "deferred:dml_modifier",
		"PartitionNames": "deferred:dml_modifier",
		"Returning":      "projected — HasReturning",
		"RowAlias":       "deferred:dml_modifier",
		"ColumnAliases":  "deferred:dml_modifier",
	},
	"UpdateStmt": {
		"dmlNode":       "exempt:embedded_base",
		"TableRefs":     "projected — mutation tables",
		"List":          "deferred:dml_surface — assignment list not in the DML audit contract",
		"Where":         "projected — predicate shape",
		"Order":         "projected — HasOrderBy",
		"Limit":         "projected — HasLimit",
		"Priority":      "deferred:dml_modifier",
		"IgnoreErr":     "deferred:dml_modifier",
		"MultipleTable": "deferred:dml_derived — multi-table shape derived from TableRefs",
		"TableHints":    "deferred:dml_modifier",
		"With":          "deferred:dml_modifier",
		"Returning":     "projected — HasReturning",
	},
	"DeleteStmt": {
		"dmlNode":      "exempt:embedded_base",
		"TableRefs":    "projected",
		"Tables":       "projected — delete target list",
		"Where":        "projected — predicate shape",
		"Order":        "projected — HasOrderBy",
		"Limit":        "projected — HasLimit",
		"Priority":     "deferred:dml_modifier",
		"IgnoreErr":    "deferred:dml_modifier",
		"Quick":        "deferred:dml_modifier",
		"IsMultiTable": "deferred:dml_derived",
		"BeforeFrom":   "deferred:dml_modifier",
		"TableHints":   "deferred:dml_modifier",
		"With":         "deferred:dml_modifier",
		"Returning":    "projected — HasReturning",
	},

	// ---- carriers -----------------------------------------------------------

	"AlterTableSpec": {
		"node":                     "exempt:embedded_base",
		"IfExists":                 "exempt:existence_flag",
		"IfNotExists":              "exempt:existence_flag",
		"NoWriteToBinlog":          "subsumed:unaudited_action — attaches to flagged actions only",
		"OnAllPartitions":          "subsumed:unaudited_action",
		"Tp":                       "projected — drives the action name (fail-closed mapping)",
		"Name":                     "projected — alter name extraction",
		"IndexName":                "projected — alter name extraction",
		"Constraint":               "projected_evidence",
		"SplitIndex":               "subsumed:unaudited_action — split_index action flagged by name",
		"Options":                  "projected_evidence — table_option action options",
		"OrderByList":              "subsumed:unaudited_action",
		"NewTable":                 "projected — rename destination",
		"NewColumns":               "projected",
		"NewConstraints":           "projected — expanded into alter actions",
		"OldColumnName":            "projected",
		"NewColumnName":            "projected",
		"Position":                 "projected — HasColumnPosition presence",
		"LockType":                 "subsumed:unaudited_action — lock action flagged by name",
		"Algorithm":                "subsumed:unaudited_action — algorithm action flagged by name",
		"Comment":                  "subsumed:unaudited_action",
		"FromKey":                  "projected — rename_index source",
		"ToKey":                    "projected — rename_index destination",
		"Partition":                "subsumed:unaudited_action",
		"PartitionNames":           "subsumed:unaudited_action",
		"PartDefinitions":          "subsumed:unaudited_action",
		"WithValidation":           "subsumed:unaudited_action",
		"Num":                      "subsumed:unaudited_action",
		"Visibility":               "subsumed:unaudited_action — alter_index flagged by name",
		"TiFlashReplica":           "subsumed:unaudited_action",
		"Writeable":                "subsumed:unaudited_action",
		"Statistics":               "subsumed:unaudited_action",
		"MaskingPolicyName":        "subsumed:unaudited_action",
		"MaskingPolicyColumn":      "subsumed:unaudited_action",
		"MaskingPolicyExpr":        "subsumed:unaudited_action",
		"MaskingPolicyRestrictOps": "subsumed:unaudited_action",
		"MaskingPolicyState":       "subsumed:unaudited_action",
		"AttributesSpec":           "subsumed:unaudited_action",
		"StatsOptionsSpec":         "subsumed:unaudited_action",
	},
	"Constraint": {
		"node":         "exempt:embedded_base",
		"IfNotExists":  "exempt:existence_flag",
		"Tp":           "projected — constraint type",
		"Name":         "projected",
		"Keys":         "projected_evidence — columns plus unmodeled-part counts",
		"Refer":        "projected_evidence — reference target plus counters",
		"Option":       "projected_evidence — index-option facts",
		"Expr":         "subsumed:constraint_kind — check/other kinds carry the gap by type name",
		"Enforced":     "subsumed:constraint_kind",
		"InColumn":     "exempt:internal_flag — column-vs-table placement marker",
		"InColumnName": "exempt:internal_flag",
		"IsEmptyIndex": "exempt:internal_flag — parser bookkeeping",
	},
	"ReferenceDef": {
		"node":                    "exempt:embedded_base",
		"Table":                   "projected — referenced target",
		"IndexPartSpecifications": "projected_evidence — referenced columns plus counters",
		"OnDelete":                "projected_evidence — refer-action counter",
		"OnUpdate":                "projected_evidence — refer-action counter",
		"Match":                   "projected_evidence — match counter",
	},
	"IndexPartSpecification": {
		"node":   "exempt:embedded_base",
		"Column": "projected — column key part",
		"Length": "projected — prefix count via UnspecifiedLength comparison",
		"Desc":   "projected — descending count",
		"Expr":   "projected — expression count",
	},
	"IndexOption": {
		"node":                       "exempt:embedded_base",
		"KeyBlockSize":               "evidence:key_block_size",
		"Tp":                         "evidence:index_type",
		"Comment":                    "evidence:comment",
		"ParserName":                 "evidence:with_parser",
		"Visibility":                 "evidence:visible|invisible",
		"PrimaryKeyTp":               "evidence:primary_key_type",
		"Global":                     "projected — GLOBAL modifier",
		"SplitOpt":                   "evidence:split_opt",
		"SecondaryEngineAttr":        "evidence:secondary_engine_attr",
		"AddColumnarReplicaOnDemand": "evidence:columnar_replica",
		"Condition":                  "projected — HasPredicate feeds index.predicate evidence",
	},
	"ColumnOption": {
		"node":                "exempt:embedded_base",
		"Tp":                  "projected — option type name",
		"Expr":                "carried:option_name — value consumed when Tp is read; unmodeled Tps flag by name",
		"Stored":              "carried:option_name",
		"Refer":               "carried:option_name — reference option flagged by name",
		"StrValue":            "carried:option_name — also drives *_global names",
		"AutoRandOpt":         "carried:option_name — auto_random flagged by name",
		"Enforced":            "carried:option_name",
		"ConstraintName":      "carried:option_name",
		"PrimaryKeyTp":        "carried:option_name",
		"SecondaryEngineAttr": "carried:option_name",
	},
	"ColumnDef": {
		"node":    "exempt:embedded_base",
		"Name":    "projected",
		"Tp":      "projected — type string, flags, charset",
		"Options": "projected_evidence",
	},
	"ColumnPosition": {
		"node":           "exempt:embedded_base",
		"Tp":             "projected — position presence",
		"RelativeColumn": "subsumed:position_presence — which column is detail under the flagged clause",
	},
	"ColumnName": {
		"node":   "exempt:embedded_base",
		"Schema": "projected",
		"Table":  "projected",
		"Name":   "projected",
	},
	"TableOption": {
		"node":          "exempt:embedded_base",
		"Tp":            "projected — option type name",
		"Default":       "carried:option_name",
		"StrValue":      "carried:option_name",
		"UintValue":     "carried:option_name",
		"BoolValue":     "carried:option_name",
		"TimeUnitValue": "carried:option_name",
		"Value":         "carried:option_name",
		"TableNames":    "carried:option_name",
		"ColumnName":    "carried:option_name",
	},
	"DatabaseOption": {
		"Tp":             "projected — option type name",
		"Value":          "carried:option_name",
		"UintValue":      "carried:option_name",
		"TiFlashReplica": "carried:option_name — TiDB member flagged by option name",
	},
	"PartitionOptions": {
		"PartitionMethod": "subsumed:partition_presence — clause audited at presence granularity (HasPartition)",
		"Sub":             "subsumed:partition_presence",
		"Definitions":     "projected_evidence — nested option names walked",
		"UpdateIndexes":   "evidence:partition_update_indexes",
	},
	"PartitionMethod": {
		"node":         "exempt:embedded_base",
		"Tp":           "subsumed:partition_presence",
		"Linear":       "subsumed:partition_presence",
		"Expr":         "subsumed:partition_presence",
		"ColumnNames":  "subsumed:partition_presence",
		"Unit":         "subsumed:partition_presence",
		"Limit":        "subsumed:partition_presence",
		"Num":          "subsumed:partition_presence",
		"KeyAlgorithm": "subsumed:partition_presence",
		"Interval":     "subsumed:partition_presence",
	},
	"PartitionDefinition": {
		"Name":    "carried:partition_presence",
		"Clause":  "subsumed:partition_presence",
		"Options": "projected_evidence — option names walked",
		"Sub":     "projected_evidence — subpartition option names walked",
	},
	"SubPartitionDefinition": {
		"Name":    "carried:partition_presence",
		"Options": "projected_evidence",
	},
	"UserSpec": {
		"User":               "projected — target identity (CurrentUser counted as omitted)",
		"AuthOpt":            "evidence:identified — auth clause presence; credential values never travel",
		"DualPasswordOption": "evidence:dual_password",
		"IsRole":             "subsumed:omitted_target — role flag rides an already-omitted target",
	},
	"AuthOption": {
		"ByAuthString": "carried:identified",
		"AuthString":   "carried:identified — value never leaves the parser",
		"ByHashString": "carried:identified",
		"HashString":   "carried:identified",
		"AuthPlugin":   "carried:identified",
	},
	"AuthTokenOrTLSOption": {
		"Type":  "carried:family_name",
		"Value": "carried:family_name — TLS subject never leaves the parser",
	},
	"PasswordOrLockOption": {
		"Type":  "carried:family_name",
		"Count": "carried:family_name",
	},
	"ResourceOption": {
		"Type":  "carried:family_name",
		"Count": "carried:family_name",
	},
	"CommentOrAttributeOption": {
		"Type":  "carried:family_name",
		"Value": "carried:family_name — comment text never leaves the parser",
	},
	"ResourceGroupNameOption": {
		"Value": "carried:family_name",
	},
	"PrivElem": {
		"node": "exempt:embedded_base",
		"Priv": "projected — privilege name",
		"Cols": "evidence:column_privileges",
		"Name": "carried:priv_name — dynamic privilege identifier rides the projected name",
	},
	"GrantLevel": {
		"Level":     "projected",
		"DBName":    "projected",
		"TableName": "projected",
	},
	"SequenceOption": {
		"Tp":       "subsumed:has_options — option list presence is the flagged fact",
		"IntValue": "subsumed:has_options",
	},
	"PlacementOption": {
		"Tp":        "subsumed:has_options",
		"StrValue":  "subsumed:has_options",
		"UintValue": "subsumed:has_options",
	},
	"SplitIndexOption": {
		"stmtNode":   "exempt:embedded_base — visitor plumbing, not a statement",
		"TableLevel": "subsumed:split_index_evidence",
		"PrimaryKey": "subsumed:split_index_evidence",
		"IndexName":  "subsumed:split_index_evidence",
		"SplitOpt":   "subsumed:split_index_evidence",
	},
	"SplitOption": {
		"stmtNode":   "exempt:embedded_base — visitor plumbing, not a statement",
		"Lower":      "subsumed:split_index_evidence — region bounds never leave the parser",
		"Upper":      "subsumed:split_index_evidence",
		"Num":        "subsumed:split_index_evidence",
		"ValueLists": "subsumed:split_index_evidence",
	},
	"TableToTable": {
		"node":     "exempt:embedded_base",
		"OldTable": "projected",
		"NewTable": "projected",
	},
	"TableName": {
		"node":           "exempt:embedded_base",
		"Schema":         "projected",
		"Name":           "projected",
		"IndexHints":     "deferred:dml_context — unreachable in DDL name positions",
		"PartitionNames": "deferred:dml_context",
		"TableSample":    "deferred:dml_context",
		"AsOf":           "deferred:dml_context",
		"IsAlias":        "exempt:internal_flag",
	},
	"StoreParameter": {
		"node":        "exempt:embedded_base",
		"Paramstatus": "subsumed:params_evidence",
		"ParamType":   "subsumed:params_evidence",
		"ParamName":   "subsumed:params_evidence",
	},
	"OnDeleteOpt": {
		"node":     "exempt:embedded_base",
		"ReferOpt": "projected — nonzero refer action counted",
	},
	"OnUpdateOpt": {
		"node":     "exempt:embedded_base",
		"ReferOpt": "projected",
	},
	"AlterOrderItem": {
		"node":   "exempt:embedded_base",
		"Column": "subsumed:unaudited_action — order_by_columns flagged by name",
		"Desc":   "subsumed:unaudited_action",
	},
	"TiFlashReplicaSpec": {
		"Count":  "subsumed:unaudited_action",
		"Labels": "subsumed:unaudited_action",
		"Hypo":   "subsumed:unaudited_action",
	},
	"StatisticsSpec": {
		"StatsName": "subsumed:unaudited_action",
		"StatsType": "subsumed:unaudited_action",
		"Columns":   "subsumed:unaudited_action",
	},
	"AttributesSpec": {
		"node":       "exempt:embedded_base",
		"Attributes": "subsumed:unaudited_action",
		"Default":    "subsumed:unaudited_action",
	},
	"StatsOptionsSpec": {
		"node":         "exempt:embedded_base",
		"StatsOptions": "subsumed:unaudited_action",
		"Default":      "subsumed:unaudited_action",
	},
	"AutoRandomOption": {
		"ShardBits": "carried:auto_random — detail under the typed AutoRandom flag",
		"RangeBits": "carried:auto_random",
	},
}

// astStructName unwraps pointer/slice/array layers and reports the struct
// type's name when the field lives in the parser's ast package.
func astStructName(t reflect.Type) string {
	for t.Kind() == reflect.Ptr || t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return ""
	}
	if !strings.HasSuffix(t.PkgPath(), "parser/ast") {
		return ""
	}
	return t.Name()
}

// TestFieldDispositionCensus requires every field of every census-scope struct
// to carry a disposition, every disposition to name a live field, and every
// ast struct reachable through census fields to be in scope or exempted.
func TestFieldDispositionCensus(t *testing.T) {
	for typeName, typ := range fieldCensusTypes {
		fields, ok := fieldDispositions[typeName]
		if !ok {
			t.Fatalf("census-scope type %s has no disposition table", typeName)
		}
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			if _, ok := fields[f.Name]; !ok {
				t.Errorf("%s.%s (%s) has no field disposition — project, evidence, subsume, exempt, or defer it", typeName, f.Name, f.Type)
			}
		}
		for name := range fields {
			if _, ok := typ.FieldByName(name); !ok {
				t.Errorf("disposition for %s.%s but the field no longer exists in the pinned parser", typeName, name)
			}
		}
	}
}

// TestFieldReachabilityCensus closes the carrier boundary: an ast-package
// struct reachable through a census field is itself a carrier and must be in
// scope — or carry an explicit reach exemption. Interface-typed fields
// (ExprNode, StmtNode, ResultSetNode) are declared boundaries.
func TestFieldReachabilityCensus(t *testing.T) {
	for typeName, typ := range fieldCensusTypes {
		for i := 0; i < typ.NumField(); i++ {
			name := astStructName(typ.Field(i).Type)
			if name == "" {
				continue
			}
			if _, ok := fieldCensusTypes[name]; ok {
				continue
			}
			if _, ok := fieldReachExempt[name]; ok {
				continue
			}
			t.Errorf("%s.%s reaches ast struct %s which is outside the census scope and unexempted", typeName, typ.Field(i).Name, name)
		}
	}
}
