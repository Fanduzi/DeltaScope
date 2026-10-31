// Package audit computes statement and aggregate audit-coverage status.
package audit

import (
	"fmt"
	"sort"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// auditedAlterActionsMySQL lists ALTER TABLE action names that at least one
// rule actually consumes — extraction alone does not make an action audited.
// Every other recognized action is an unaudited aspect that keeps the statement
// but marks coverage incomplete. The set mirrors the generic-notice and
// semantically-checked rows of the official DDL inventory: column, index,
// key/constraint, rename, and table-option clauses. T10 promoted ALTER
// INDEX VISIBLE|INVISIBLE to the audited set — the invisibility rule,
// index existence, and version applicability consume it on both dialects.
// ALGORITHM/LOCK still parses without a rule consumer and stays unaudited.
var auditedAlterActionsMySQL = map[string]struct{}{
	"add_columns":      {},
	"drop_column":      {},
	"modify_column":    {},
	"change_column":    {},
	"rename_column":    {},
	"rename_table":     {},
	"drop_primary_key": {},
	"drop_index":       {},
	"add_constraint":   {},
	"add_index":        {},
	"rename_index":     {},
	"alter_index":      {},
	"drop_foreign_key": {},
	"alter_check":      {},
	"drop_check":       {},
	"table_option":     {},
	// T07: the canonical MySQL/TiDB SET/DROP DEFAULT actions are audited by
	// the advanced-column facts and ordered-state setter. SET
	// VISIBLE|INVISIBLE is audited under MySQL only; TiDB accepts the syntax
	// at the vendor boundary below.
	"set_column_default":    {},
	"drop_column_default":   {},
	"set_column_visibility": {},
	// T13: destructive/data-moving partition actions are governed by the
	// partition rules and modeled in ordered state on both dialects.
	"drop_partition":     {},
	"truncate_partition": {},
	"exchange_partition": {},
	// T14: partition topology and maintenance actions — each audited by
	// its applicability/target/policy rules and modeled in ordered state.
	// TiDB audits only the five actions that execute there; the six
	// maintenance ops are vendor refusals (8200) on TiDB and stay
	// non-audited aspects with vendor=true.
	"add_partitions":               {},
	"reorganize_partition":         {},
	"coalesce_partitions":          {},
	"partition":                    {},
	"remove_partitioning":          {},
	"rebuild_partition":            {},
	"check_partitions":             {},
	"optimize_partition":           {},
	"repair_partition":             {},
	"import_partition_tablespace":  {},
	"discard_partition_tablespace": {},
	// T15: online-DDL modifiers and the MyISAM/maintenance actions —
	// adjudicated by the shared OnlineDDLOutcome matrix and the
	// keys/rebuild rules on MySQL. On TiDB, algorithm/lock/order_by/keys
	// are accepted compat declarations; force is a proven 8200 refusal
	// and joins the vendor set below.
	"algorithm":        {},
	"lock":             {},
	"order_by_columns": {},
	"force":            {},
	"enable_keys":      {},
	"disable_keys":     {},
}

// tidbVendorRefusedAlterActions is the probed TiDB 8200 refusal set:
// every named maintenance action is a vendor boundary on TiDB v8.5 —
// the six maintenance partition ops plus FORCE — so the actions are not
// audited there and emit vendor aspects instead.
var tidbVendorRefusedAlterActions = map[string]struct{}{
	"rebuild_partition":            {},
	"check_partitions":             {},
	"optimize_partition":           {},
	"repair_partition":             {},
	"import_partition_tablespace":  {},
	"discard_partition_tablespace": {},
	"force":                        {},
}

// auditedAlterActionsTiDB extends the shared audited set with the TiDB
// placement binding the inventory covers at notice level. ALTER|DROP CHECK
// are audited on both dialects — the CHECK lifecycle rules and ordered-state
// transitions model them. SET VISIBLE|INVISIBLE stays in
// the audited set for both dialects: the action itself is modeled, and the
// column aspect below emits the TiDB vendor-boundary evidence.
var auditedAlterActionsTiDB = func() map[string]struct{} {
	set := make(map[string]struct{}, len(auditedAlterActionsMySQL)+1)
	for action := range auditedAlterActionsMySQL {
		if _, refused := tidbVendorRefusedAlterActions[action]; refused {
			continue
		}
		set[action] = struct{}{}
	}
	set["placement_policy"] = struct{}{}
	return set
}()

// auditedAlterAction reports whether the action has audited semantics for the
// dialect. Unaudited actions stay in the statement result but are recorded as
// bounded unsupported evidence.
func auditedAlterAction(dialect spec.Dialect, action string) bool {
	if dialect == spec.DialectTiDB {
		_, ok := auditedAlterActionsTiDB[action]
		return ok
	}
	_, ok := auditedAlterActionsMySQL[action]
	return ok
}

// auditedPositionAction reports whether a parsed FIRST|AFTER position is
// modeled for the action. The grammar only emits positions on these three;
// a position on any other action is an anomaly that keeps the aspect gap.
func auditedPositionAction(action string) bool {
	switch action {
	case "add_columns", "modify_column", "change_column":
		return true
	default:
		return false
	}
}

// extensionTypeVendor reports whether TiDB-extension index/constraint types
// (vector, columnar) are a vendor boundary under the dialect: MySQL has no such
// types, while TiDB ships them without audited semantics yet.
func extensionTypeVendor(dialect spec.Dialect) bool {
	return dialect == spec.DialectMySQL
}

const (
	constraintTypeVector   = "vector"
	constraintTypeColumnar = "columnar"
	constraintTypeCheck    = "check"
)

// indexKindGap classifies an index kind that parses but lacks audited
// semantics. gap marks unaudited analysis; vendor marks a product boundary.
// TiDB treats fulltext/spatial as a vendor boundary and vector/columnar as
// unaudited; MySQL audits fulltext/spatial at notice level but has no
// vector/columnar index surface.
func indexKindGap(dialect spec.Dialect, kind spec.IndexKind) (gap bool, vendor bool) {
	switch kind {
	case spec.IndexKindVector, spec.IndexKindColumnar:
		return true, extensionTypeVendor(dialect)
	case spec.IndexKindFulltext, spec.IndexKindSpatial:
		v := dialect == spec.DialectTiDB
		return v, v
	default:
		return false, false
	}
}

// sortedOptionNames returns ddl.Options keys in deterministic order so
// unsupported evidence ordering never depends on Go map iteration.
func sortedOptionNames(options map[string]string) []string {
	names := make([]string, 0, len(options))
	for name := range options {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// alterConstraintGap classifies constraint types on alter actions that parse
// but lack audited semantics. MySQL covers check constraints only through
// create-table extraction, so check-bearing alter actions stay incomplete
// there; TiDB's fk-check row covers only ADD CHECK at notice level, so
// DROP CHECK is excluded from the audited action set entirely.
func alterConstraintGap(dialect spec.Dialect, constraintType string) (gap bool, vendor bool) {
	switch constraintType {
	case constraintTypeVector, constraintTypeColumnar:
		return true, extensionTypeVendor(dialect)
	case constraintTypeCheck:
		// T12 audits CHECK payloads on both dialects — the lifecycle
		// rules and ordered-state transitions consume them.
		return false, false
	default:
		return false, false
	}
}

// extractedOptionGap classifies an extracted ddl.Options key that no rule
// consumes: presence of the fact is proven, but its semantics are unaudited.
// MySQL-only vendor flags mark TiDB features MySQL does not ship. Every entry
// corresponds to an inventory row documenting the unchecked aspect.
func extractedOptionGap(dialect spec.Dialect, op spec.DDLOperation, name string) (gap bool, vendor bool) {
	switch name {
	case "placement_policy":
		// Placement policies are a TiDB feature: vendor boundary under MySQL,
		// extracted-but-unaudited binding under TiDB — the binding premise
		// (the named policy's existence) stays unvouchable (T17).
		return true, dialect == spec.DialectMySQL
	case "encryption":
		// Schema-level ENCRYPTION is modeled on schema ops (version-gated
		// transitions, T17); on CREATE TABLE it stays the T15 extracted-but-
		// unaudited table option.
		return op == spec.DDLOperationCreateTable, false
	case "charset":
		// Schema-level charset is modeled on schema ops (T17 ordered
		// transitions); the create-table charset allowlist consumes it there.
		return false, false
	case "collate":
		// Schema-level collation is modeled on schema ops (T17); table
		// ops consume it through the existing allowlist.
		return false, false
	case "has_options":
		// Sequence and placement-policy option lists are observed but not
		// modeled (inventory: option/constraint semantics unchecked).
		switch op {
		case spec.DDLOperationCreateSequence, spec.DDLOperationAlterSequence,
			spec.DDLOperationCreatePlacementPolicy, spec.DDLOperationAlterPlacementPolicy:
			return true, false
		}
	case "has_body":
		// A parsed procedure body carries static SQL no rule audits yet
		// (inventory owner T27). Under TiDB the statement itself is already a
		// vendor boundary, so this aspect only ever fires under MySQL.
		return op == spec.DDLOperationCreateProcedure, false
	}
	if t15ExtractedTableOption(name) {
		// T15 extracts the shared MySQL/TiDB table-option vocabulary on
		// CREATE — declarations carry values now, but per-option CREATE
		// semantics stay unaudited, so the bounded option gap remains.
		return op == spec.DDLOperationCreateTable, false
	}
	return false, false
}

// t15ExtractedTableOption lists the option names T15 moved from the
// unextracted bag into ddl.Options — extraction upgraded the evidence
// (authored values are now captured), not the verdict.
func t15ExtractedTableOption(name string) bool {
	switch name {
	case "key_block_size", "compression", "encryption", "tablespace",
		"insert_method", "stats_persistent", "stats_auto_recalc",
		"stats_sample_pages", "pack_keys", "delay_key_write",
		"avg_row_length", "checksum", "table_checksum", "max_rows",
		"min_rows", "data_directory", "index_directory", "connection",
		"password", "storage_media", "secondary_engine", "union":
		return true
	default:
		return false
	}
}

// tidbOnlyTableOption reports whether an unextracted table-option name is a
// TiDB-only feature under the given dialect — i.e. parsed by the shared
// parser but outside the MySQL surface (official MySQL 5.7/8.0/8.4 has no
// AUTO_RANDOM-base/ID-cache/sharding/TTL/placement/stats/affinity options).
// Consumed names such as placement_policy are handled by extractedOptionGap.
func tidbOnlyTableOption(name string, dialect spec.Dialect) bool {
	if dialect != spec.DialectMySQL {
		return false
	}
	switch name {
	case "auto_random_base", "auto_id_cache", "shard_row_id_bits",
		"pre_split_regions", "ttl", "ttl_enable", "ttl_job_interval",
		"stats_buckets", "stats_top_n", "stats_cols_choice", "stats_col_list",
		"stats_sample_rate", "affinity":
		return true
	}
	return false
}

// columnExpressionFacts returns the declared expression records on one
// column — generated expression first, then the expression default.
func columnExpressionFacts(column spec.Column) []spec.ExpressionFacts {
	var exprs []spec.ExpressionFacts
	if column.Generated != nil {
		exprs = append(exprs, column.Generated.Expression)
	}
	if column.DefaultExpression != nil {
		exprs = append(exprs, *column.DefaultExpression)
	}
	return exprs
}

// expressionFactsUnknownReason reports the bounded unaudited boundary marker
// an expression carries: the specific unknown-function boundary, any other
// unknown-expression shape or traversal limit, or empty when the facts are
// fully known.
func expressionFactsUnknownReason(expr spec.ExpressionFacts) string {
	if expr.Known {
		return ""
	}
	for _, feature := range expr.Features {
		if feature == spec.ExpressionFeatureUnknownFunc {
			return "unknown_function"
		}
	}
	return "unknown_expression"
}

// columnAspectGaps emits bounded evidence for parsed column attributes no rule
// audits: the typed AUTO_RANDOM fact (a TiDB-only feature, so a vendor
// boundary under MySQL — owner T23) and every unextracted column option the
// extractor recognized but dropped (generated expressions, inline
// REFERENCES/CHECK/UNIQUE, storage/format attributes).
func columnAspectGaps(dialect spec.Dialect, op spec.DDLOperation, action string, column spec.Column) []spec.UnsupportedDetail {
	prefix := fmt.Sprintf("%s.column", op)
	if action != "" {
		prefix = fmt.Sprintf("%s.%s.column", op, action)
	}
	var gaps []spec.UnsupportedDetail
	if column.AutoRandom {
		gaps = append(gaps, aspectGap(
			prefix+".auto_random", dialect == spec.DialectMySQL,
			map[string]any{"aspect": "column"},
		))
	}
	if column.VisibilityExplicit && dialect == spec.DialectTiDB {
		// Column VISIBLE/INVISIBLE is MySQL-only syntax the shared parser
		// accepts for TiDB: a vendor boundary, not unaudited analysis.
		gaps = append(gaps, aspectGap(
			prefix+".visibility", true,
			map[string]any{"aspect": "column"},
		))
	}
	for _, expr := range columnExpressionFacts(column) {
		if reason := expressionFactsUnknownReason(expr); reason != "" {
			// An unprovable expression is the bounded T07 unaudited boundary:
			// the dependency answer is withheld even when the rules are off.
			gaps = append(gaps, aspectGap(
				prefix+".expression_dependencies", false,
				map[string]any{"aspect": "column", "reason": reason},
			))
			break
		}
	}
	for _, option := range column.UnextractedOptions {
		// GLOBAL-qualified inline index options and the CLUSTERED/
		// NONCLUSTERED primary-key type are TiDB-only syntax.
		vendor := dialect == spec.DialectMySQL &&
			(option == "unique_global" || option == "primary_key_global" || option == "primary_key_type")
		gaps = append(gaps, aspectGap(
			fmt.Sprintf("%s.%s", prefix, option), vendor,
			map[string]any{"aspect": "column"},
		))
	}
	return gaps
}

// aspectGap builds one bounded UnsupportedDetail for a recognized-but-
// unaudited aspect, choosing the vendor-boundary or unaudited reason.
func aspectGap(feature string, vendor bool, metadata map[string]any) spec.UnsupportedDetail {
	reason := spec.UnsupportedUnauditedReason
	if vendor {
		reason = spec.UnsupportedVendorBoundaryReason
	}
	return spec.UnsupportedDetail{Feature: feature, Reason: reason, Metadata: metadata}
}

// statementCoverageAspects returns bounded UnsupportedDetail entries for every
// recognized-but-unaudited aspect inside a supported statement. The details
// carry stable feature identifiers and fixed reasons only — never raw SQL,
// option values, or parser internals. Aspect classification is scoped to the
// MySQL/TiDB extractor's extraction contract; PostgreSQL keeps its own
// statement-level unsupported mechanism.
func statementCoverageAspects(dialect spec.Dialect, statement spec.Statement) []spec.UnsupportedDetail {
	if dialect != spec.DialectMySQL && dialect != spec.DialectTiDB {
		return nil
	}
	if statement.Kind != spec.KindDDL || statement.DDL == nil {
		return nil
	}
	ddl := statement.DDL
	var gaps []spec.UnsupportedDetail
	for _, option := range ddl.UnextractedOptions {
		// A dropped placement-policy or TiDB-only option is still a vendor
		// boundary under MySQL — MySQL has no such feature regardless of
		// modeling. partition_update_indexes, split_index, and the
		// resource_group_name account binding are TiDB extensions the shared
		// parser accepts but MySQL does not ship. The mirror image holds for
		// routine_object: GRANT/REVOKE ON FUNCTION|PROCEDURE binds privileges
		// to routines, which TiDB does not have — a vendor boundary there.
		vendor := dialect == spec.DialectMySQL && (option == "placement_policy" || option == "partition_update_indexes" || option == "split_index" || option == "resource_group_name" || option == "hypo_index" || option == "tiflash_replica" || tidbOnlyTableOption(option, dialect))
		vendor = vendor || (dialect == spec.DialectTiDB && option == "routine_object")
		gaps = append(gaps, aspectGap(
			fmt.Sprintf("%s.option.%s", ddl.Operation, option), vendor,
			map[string]any{"aspect": "option"},
		))
	}
	for _, name := range sortedOptionNames(ddl.Options) {
		if gap, vendor := extractedOptionGap(dialect, ddl.Operation, name); gap {
			feature := fmt.Sprintf("%s.option.%s", ddl.Operation, name)
			switch name {
			case "has_options":
				feature = fmt.Sprintf("%s.options", ddl.Operation)
			case "has_body":
				feature = fmt.Sprintf("%s.body", ddl.Operation)
			}
			gaps = append(gaps, aspectGap(feature, vendor, map[string]any{"aspect": "option"}))
		}
	}
	if ddl.TemporaryScope == spec.TemporaryScopeGlobal {
		// LOCAL temporary objects are fully modeled (T16): session
		// namespace binding, shadowing, and lifecycle transitions all
		// publish. GLOBAL TEMPORARY is TiDB-only — a vendor boundary
		// under MySQL — while TiDB's own global-temporary catalog object
		// with ON COMMIT DELETE ROWS is modeled and carries no aspect.
		// The ON COMMIT PRESERVE ROWS form is the one global clause TiDB
		// refuses (8200): it stays a vendor boundary under TiDB.
		switch {
		case dialect == spec.DialectMySQL:
			gaps = append(gaps, aspectGap(fmt.Sprintf("%s.temporary.global", ddl.Operation), true,
				map[string]any{"aspect": "temporary", "temporary_scope": ddl.TemporaryScope}))
		case ddl.Operation == spec.DDLOperationCreateTable && !ddl.OnCommitDelete:
			// ON COMMIT is a create-time clause only — DROP GLOBAL carries
			// no attribute and is fully modeled. Only the PRESERVE ROWS
			// create form is the TiDB-refused (8200) vendor boundary.
			gaps = append(gaps, aspectGap(fmt.Sprintf("%s.temporary.global.preserve", ddl.Operation), true,
				map[string]any{"aspect": "temporary", "temporary_scope": ddl.TemporaryScope}))
		}
	}
	for _, column := range ddl.Columns {
		gaps = append(gaps, columnAspectGaps(dialect, ddl.Operation, "", column)...)
	}
	indexPartGaps := func(prefix string, index spec.Index) {
		// Expression, prefix, descending, and multi-valued member forms are
		// member-level audited (member legality, version applicability, and
		// ordered post-state fidelity) — no blanket unsupported aspect
		// remains. Only members the model cannot classify at all keep the
		// unaudited boundary.
		if index.UnmodeledKeyParts > 0 {
			gaps = append(gaps, aspectGap(
				fmt.Sprintf("%s.unmodeled_parts", prefix), false,
				map[string]any{"aspect": "index", "index_kind": string(index.Kind), "unmodeled_parts": index.UnmodeledKeyParts},
			))
		}
		if index.HasPredicate {
			// Partial-index WHERE predicates are documented in TiDB's CREATE
			// INDEX grammar but absent from MySQL's index options.
			gaps = append(gaps, aspectGap(
				fmt.Sprintf("%s.predicate", prefix), dialect == spec.DialectMySQL,
				map[string]any{"aspect": "index", "index_kind": string(index.Kind)},
			))
		}
		if index.PrimaryKeyType != "" && dialect == spec.DialectMySQL {
			// CLUSTERED|NONCLUSTERED is TiDB-only index-option syntax; it is
			// modeled (PrimaryKeyType) for TiDB PKIsHandle determination and
			// stays a vendor boundary under MySQL.
			gaps = append(gaps, aspectGap(
				fmt.Sprintf("%s.option.primary_key_type", prefix), true,
				map[string]any{"aspect": "index_option", "index_kind": string(index.Kind)},
			))
		}
		for _, option := range index.UnmodeledOptions {
			// split_opt, secondary_engine_attr, and columnar_replica are
			// TiDB extensions: vendor boundaries under MySQL, unaudited
			// under TiDB.
			vendor := dialect == spec.DialectMySQL && (option == "split_opt" || option == "secondary_engine_attr" || option == "columnar_replica" || option == "primary_key_type")
			gaps = append(gaps, aspectGap(
				fmt.Sprintf("%s.option.%s", prefix, option), vendor,
				map[string]any{"aspect": "index_option", "index_kind": string(index.Kind)},
			))
		}
	}
	if ddl.PrimaryKey != nil && ddl.PrimaryKey.Global && dialect == spec.DialectMySQL {
		// GLOBAL is TiDB-only syntax — a vendor boundary under MySQL; under
		// TiDB the unique-kind plus partitioned-table gate is member-audited.
		gaps = append(gaps, aspectGap(
			fmt.Sprintf("%s.index.global", ddl.Operation), true,
			map[string]any{"aspect": "index", "index_kind": string(spec.IndexKindPrimary)},
		))
	}
	if ddl.PrimaryKey != nil {
		indexPartGaps(fmt.Sprintf("%s.index", ddl.Operation), *ddl.PrimaryKey)
	}
	for _, index := range ddl.Indexes {
		if gap, vendor := indexKindGap(dialect, index.Kind); gap {
			gaps = append(gaps, aspectGap(
				fmt.Sprintf("%s.index.%s", ddl.Operation, index.Kind), vendor,
				map[string]any{"aspect": "index", "index_kind": string(index.Kind)},
			))
		}
		if index.Global && dialect == spec.DialectMySQL {
			// The GLOBAL modifier is TiDB-only syntax — a vendor boundary
			// under MySQL; under TiDB the unique-kind plus partitioned-table
			// gate is member-audited.
			gaps = append(gaps, aspectGap(
				fmt.Sprintf("%s.index.global", ddl.Operation), true,
				map[string]any{"aspect": "index", "index_kind": string(index.Kind)},
			))
		}
		indexPartGaps(fmt.Sprintf("%s.index", ddl.Operation), index)
	}
	if ddl.OmittedTargets > 0 {
		// The normalized model kept only the first (or no) target of a parsed
		// multi-object list; the dropped targets are unaudited evidence, not
		// audited deletions.
		gaps = append(gaps, aspectGap(
			fmt.Sprintf("%s.unaudited_targets", ddl.Operation), false,
			map[string]any{"aspect": "targets", "omitted": ddl.OmittedTargets},
		))
	}
	for _, constraint := range ddl.Constraints {
		// Create-table-level CHECK is already extracted and audited; only the
		// TiDB-extension types are gaps here.
		if constraint.Type == constraintTypeVector || constraint.Type == constraintTypeColumnar {
			gaps = append(gaps, aspectGap(
				fmt.Sprintf("%s.constraint.%s", ddl.Operation, constraint.Type),
				extensionTypeVendor(dialect),
				map[string]any{"aspect": "constraint", "constraint_type": constraint.Type},
			))
		}
		if constraint.UnmodeledParts > 0 || constraint.UnmodeledReferencedParts > 0 || constraint.UnmodeledReferActions > 0 {
			gaps = append(gaps, aspectGap(
				fmt.Sprintf("%s.constraint.%s.parts", ddl.Operation, constraint.Type), false,
				map[string]any{
					"aspect":           "constraint_parts",
					"constraint_type":  constraint.Type,
					"local_parts":      constraint.UnmodeledParts,
					"referenced_parts": constraint.UnmodeledReferencedParts,
					"refer_actions":    constraint.UnmodeledReferActions,
				},
			))
		}
	}
	// A non-partition modifier combination is adjudicated once per
	// statement: a proven verdict (combinable or refused) consumes every
	// algorithm/lock spec; a fact-missing unproven outcome leaves the gap
	// to the owning rule's EvidenceGaps; an unprobed-combination unproven
	// keeps one bounded aspect — the capability boundary is never
	// laundered into pass.
	if ddl.Operation == spec.DDLOperationAlterTable && !hasPartitionAlterAction(ddl) {
		if _, _, actions, _ := spec.OnlineDDLModifierValues(ddl.Alter); len(actions) != len(ddl.Alter) {
			var version *spec.VersionIdentity
			var engine string
			if statement.Metadata != nil {
				version = statement.Metadata.Version
				if statement.Metadata.TargetTable != nil {
					engine = statement.Metadata.TargetTable.Options["engine"]
				}
			}
			outcome := spec.OnlineDDLOutcomeFor(dialect, version, engine, ddl.Alter)
			if outcome.Verdict == spec.OnlineDDLUnproven && len(outcome.GapFacts) == 0 {
				gaps = append(gaps, aspectGap(
					fmt.Sprintf("%s.online_ddl", ddl.Operation), false,
					map[string]any{"aspect": "action", "action": "online_ddl"},
				))
			}
		}
	}
	for _, alter := range ddl.Alter {
		// An ALGORITHM/LOCK modifier accompanying a partition action is
		// consumed by the ordered-state combination classifier — but only
		// when the classifier reached a proven verdict for the whole
		// combination. An unproven shape keeps its aspect on the current
		// statement — the boundary is never laundered through a
		// successor tombstone.
		if (alter.Action == "algorithm" || alter.Action == "lock") && hasPartitionAlterAction(ddl) {
			if !partitionCombinationProven(ddl, statement) {
				gaps = append(gaps, aspectGap(
					fmt.Sprintf("%s.%s", ddl.Operation, alter.Action), false,
					map[string]any{"aspect": "action", "action": alter.Action},
				))
			}
			continue
		}
		if ddl.Operation == spec.DDLOperationAlterTable && !auditedAlterAction(dialect, alter.Action) {
			// PLACEMENT POLICY and the TiDB-only partition attribute/option
			// forms are vendor boundaries under MySQL; under TiDB the six
			// maintenance partition ops are proven vendor refusals (8200)
			// — vendor aspects in both directions, never plain unaudited.
			vendor := unauditedActionVendor(dialect, alter.Action)
			gaps = append(gaps, aspectGap(
				fmt.Sprintf("%s.%s", ddl.Operation, partitionActionFeatureName(alter.Action)), vendor,
				map[string]any{"aspect": "action", "action": alter.Action},
			))
			continue
		}
		// An extracted table-option name is classified per dialect: a
		// consumed name passes silently, a proven TiDB refusal is a
		// vendor aspect, and an extracted-but-unaudited name keeps a
		// bounded gap — the declaration is never silently accepted.
		if ddl.Operation == spec.DDLOperationAlterTable && alter.Action == "table_option" {
			for _, name := range sortedOptionNames(alter.Options) {
				switch spec.AlterTableOptionOutcome(dialect, name) {
				case spec.TableOptionVendorRefused:
					gaps = append(gaps, aspectGap(
						fmt.Sprintf("%s.option.%s", ddl.Operation, name), true,
						map[string]any{"aspect": "option", "action": alter.Action, "option": name},
					))
				case spec.TableOptionUnaudited:
					gaps = append(gaps, aspectGap(
						fmt.Sprintf("%s.option.%s", ddl.Operation, name), false,
						map[string]any{"aspect": "option", "action": alter.Action, "option": name},
					))
				}
			}
		}
		if alter.Constraint != nil {
			if gap, vendor := alterConstraintGap(dialect, alter.Constraint.Type); gap {
				gaps = append(gaps, aspectGap(
					fmt.Sprintf("%s.%s.%s", ddl.Operation, alter.Action, alter.Constraint.Type), vendor,
					map[string]any{"aspect": "constraint", "action": alter.Action, "constraint_type": alter.Constraint.Type},
				))
			}
			if alter.Constraint.UnmodeledParts > 0 || alter.Constraint.UnmodeledReferencedParts > 0 || alter.Constraint.UnmodeledReferActions > 0 {
				gaps = append(gaps, aspectGap(
					fmt.Sprintf("%s.%s.constraint.%s.parts", ddl.Operation, alter.Action, alter.Constraint.Type), false,
					map[string]any{
						"aspect":           "constraint_parts",
						"action":           alter.Action,
						"constraint_type":  alter.Constraint.Type,
						"local_parts":      alter.Constraint.UnmodeledParts,
						"referenced_parts": alter.Constraint.UnmodeledReferencedParts,
						"refer_actions":    alter.Constraint.UnmodeledReferActions,
					},
				))
			}
		}
		if alter.Index != nil && alter.Index.Definition != nil {
			if gap, vendor := indexKindGap(dialect, alter.Index.Definition.Kind); gap {
				gaps = append(gaps, aspectGap(
					fmt.Sprintf("%s.%s.%s", ddl.Operation, alter.Action, alter.Index.Definition.Kind), vendor,
					map[string]any{"aspect": "index", "action": alter.Action, "index_kind": string(alter.Index.Definition.Kind)},
				))
			}
			if alter.Index.Definition.Global && dialect == spec.DialectMySQL {
				gaps = append(gaps, aspectGap(
					fmt.Sprintf("%s.%s.index.global", ddl.Operation, alter.Action), true,
					map[string]any{"aspect": "index", "action": alter.Action, "index_kind": string(alter.Index.Definition.Kind)},
				))
			}
			indexPartGaps(fmt.Sprintf("%s.%s.index", ddl.Operation, alter.Action), *alter.Index.Definition)
		}
		if alter.Column != nil && alter.Column.Definition != nil {
			gaps = append(gaps, columnAspectGaps(dialect, ddl.Operation, alter.Action, *alter.Column.Definition)...)
		}
		if alter.HasColumnPosition && !auditedPositionAction(alter.Action) {
			gaps = append(gaps, aspectGap(
				fmt.Sprintf("%s.%s.column_position", ddl.Operation, alter.Action), false,
				map[string]any{"aspect": "column_position", "action": alter.Action},
			))
		}
		// Partition bound items the fact model could not project (a
		// truncated or subquery-shaped bound expression) are a coverage
		// boundary — comparable-literal bounds the rules could not
		// adjudicate stay evidence gaps on the owning rule instead. The
		// PARTITION BY scheme definitions carry the same boundary through
		// PartitionScheme.Defs rather than PartitionDefs.
		for _, def := range alter.PartitionDefs {
			if partitionBoundUnproven(def.Bound) {
				gaps = append(gaps, aspectGap(
					fmt.Sprintf("%s.%s.partition_bound", ddl.Operation, partitionActionFeatureName(alter.Action)), false,
					map[string]any{"aspect": "partition_bound", "action": alter.Action, "partition": def.Name},
				))
			}
		}
		if alter.PartitionScheme != nil {
			for _, def := range alter.PartitionScheme.Defs {
				if partitionBoundUnproven(def.Bound) {
					gaps = append(gaps, aspectGap(
						fmt.Sprintf("%s.%s.partition_bound", ddl.Operation, partitionActionFeatureName(alter.Action)), false,
						map[string]any{"aspect": "partition_bound", "action": alter.Action, "partition": def.Name},
					))
				}
			}
		}
		if alter.PartitionScheme != nil && alter.PartitionScheme.Expr != nil &&
			!alter.PartitionScheme.Expr.Known {
			gaps = append(gaps, aspectGap(
				fmt.Sprintf("%s.%s.partition_expression", ddl.Operation, alter.Action), false,
				map[string]any{"aspect": "partition_expression", "action": alter.Action},
			))
		}
		for _, option := range alter.UnextractedOptions {
			gaps = append(gaps, aspectGap(
				fmt.Sprintf("%s.option.%s", ddl.Operation, option), tidbOnlyTableOption(option, dialect),
				map[string]any{"aspect": "option", "action": alter.Action},
			))
		}
	}
	// A CREATE-side partitioning clause declares its member definitions on
	// the statement body rather than an alter action; expression-shaped
	// bound items carry the same capability boundary as the alter path.
	if ddl.HasPartition {
		for _, def := range ddl.PartitionDefs {
			if partitionBoundUnproven(def.Bound) {
				gaps = append(gaps, aspectGap(
					fmt.Sprintf("%s.partition_bound", ddl.Operation), false,
					map[string]any{"aspect": "partition_bound", "partition": def.Name},
				))
			}
		}
	}
	return gaps
}

// unauditedActionVendor reports whether a non-audited alter action is a
// vendor boundary rather than merely unaudited analysis:
//   - under MySQL: placement_policy and the TiDB-only partition
//     attribute/option forms (grammar belongs to the other product — the
//     statement refuses natively);
//   - under TiDB: the six maintenance partition ops (proven 8200
//     refusals). TiDB's own attribute/option forms stay plain unaudited —
//     they execute there.
func unauditedActionVendor(dialect spec.Dialect, action string) bool {
	switch dialect {
	case spec.DialectMySQL:
		switch action {
		case "placement_policy", "partition_attributes", "partition_options":
			return true
		}
	case spec.DialectTiDB:
		_, refused := tidbVendorRefusedAlterActions[action]
		return refused
	}
	return false
}

// partitionActionFeatureName maps a partition alter action to the stable
// feature vocabulary the rule IDs and inventory rows share — the
// extractor's plural/shorthand action spellings (`add_partitions`,
// `check_partitions`, `coalesce_partitions`, `partition`) name the same
// actions the public `*_partition` features document.
func partitionActionFeatureName(action string) string {
	switch action {
	case "add_partitions":
		return "add_partition"
	case "check_partitions":
		return "check_partition"
	case "coalesce_partitions":
		return "coalesce_partition"
	case "partition":
		return "partition_by"
	default:
		return action
	}
}

// partitionBoundUnproven reports whether a member bound carries an item
// outside the comparable literal domain — any expression-shaped item,
// evaluated or not. The comparators never adjudicate expression items,
// so a fully-known expression is still an unimplemented capability here,
// not a metadata gap: the aspect carries the capability boundary while a
// provider-side unprojectable bound stays an evidence gap on the owning
// rule.
func partitionBoundUnproven(bound *spec.PartitionBound) bool {
	if bound == nil {
		return false
	}
	for _, row := range bound.Rows {
		for _, item := range row {
			if item.Kind == spec.PartitionBoundExpression {
				return true
			}
		}
	}
	return false
}

// partitionCombinationProven reports whether the ordered-state classifier
// reached a proven verdict — combinable or refused — for a partition ALTER
// carrying modifier specs. The statement's enriched version fact is the
// same premise the ordered-state application consumed.
func partitionCombinationProven(ddl *spec.DDL, statement spec.Statement) bool {
	if !hasPartitionAlterAction(ddl) || len(ddl.Alter) < 2 {
		return false
	}
	var version *spec.VersionIdentity
	if statement.Metadata != nil {
		version = statement.Metadata.Version
	}
	return partitionMixedOutcome(ddl, version) != partitionOutcomeUnproven
}

// statementCoverage resolves the coverage status for one extracted statement:
// parser-marked boundaries and any aspect gap make the statement incomplete;
// everything else is complete.
func statementCoverage(dialect spec.Dialect, statement spec.Statement) (report.Coverage, []spec.UnsupportedDetail) {
	if boundary := statement.ResourceLimit; boundary != nil {
		var details []spec.UnsupportedDetail
		if statement.Unsupported != nil {
			details = append(details, *statement.Unsupported)
		}
		details = append(details, statementCoverageAspects(dialect, statement)...)
		details = append(details, spec.UnsupportedDetail{
			Feature: spec.AuditResourceLimitFeature,
			Reason:  spec.AuditResourceLimitReason,
			Metadata: map[string]any{
				"phase": "ordered_state", "resource": "statements",
				"limit": boundary.Limit, "consumed": boundary.Consumed,
				"line": statement.Line, "column": statement.Column,
			},
		})
		return report.Coverage{Status: report.CoverageIncomplete}, details
	}
	if statement.Unsupported != nil {
		return report.Coverage{Status: report.CoverageIncomplete}, nil
	}
	gaps := statementCoverageAspects(dialect, statement)
	if len(gaps) > 0 {
		return report.Coverage{Status: report.CoverageIncomplete}, gaps
	}
	return report.Coverage{Status: report.CoverageComplete}, nil
}
