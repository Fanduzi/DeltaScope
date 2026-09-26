// Package audit computes statement and aggregate audit-coverage status.
// input: extracted domain statements with parser-attached boundary markers and extraction facts
// output: per-statement coverage status plus bounded unsupported evidence for unaudited aspects
// pos: application coverage classification between extraction and reporting (issue #82)
// note: if this file changes, update this header and module README.md.
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
// key/constraint, rename, and table-option clauses. ALGORITHM/LOCK and ALTER
// INDEX INVISIBLE parse but have no rule consumer, so they stay unaudited.
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
	"drop_foreign_key": {},
	"table_option":     {},
}

// auditedAlterActionsTiDB extends the shared audited set with the TiDB
// placement binding the inventory covers at notice level. DROP CHECK parses but
// has no rule consumer, so it stays unaudited.
var auditedAlterActionsTiDB = func() map[string]struct{} {
	set := make(map[string]struct{}, len(auditedAlterActionsMySQL)+1)
	for action := range auditedAlterActionsMySQL {
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
		return dialect == spec.DialectMySQL, false
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
		// extracted-but-unaudited binding under TiDB.
		return true, dialect == spec.DialectMySQL
	case "charset":
		// Database charset is extracted but unchecked on schema ops; the
		// create-table charset allowlist consumes it there.
		return op == spec.DDLOperationCreateSchema || op == spec.DDLOperationAlterSchema, false
	case "collate":
		// Collation is extracted but unchecked on schema ops and create table.
		switch op {
		case spec.DDLOperationCreateSchema, spec.DDLOperationAlterSchema, spec.DDLOperationCreateTable:
			return true, false
		}
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
	return false, false
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
	for _, option := range column.UnextractedOptions {
		// GLOBAL-qualified inline index options are TiDB-only syntax.
		vendor := dialect == spec.DialectMySQL &&
			(option == "unique_global" || option == "primary_key_global")
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
		// parser accepts but MySQL does not ship.
		vendor := dialect == spec.DialectMySQL && (option == "placement_policy" || option == "partition_update_indexes" || option == "split_index" || option == "resource_group_name" || tidbOnlyTableOption(option, dialect))
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
	if ddl.TemporaryScope != "" {
		// A parsed temporary scope changes object identity and lifetime; no
		// rule audits it yet (owner T16). GLOBAL TEMPORARY is TiDB-only, so it
		// is a vendor boundary under MySQL; the local form exists in both
		// products and stays unaudited analysis.
		feature := fmt.Sprintf("%s.temporary", ddl.Operation)
		vendor := false
		if ddl.TemporaryScope == spec.TemporaryScopeGlobal {
			feature += ".global"
			vendor = dialect == spec.DialectMySQL
		}
		gaps = append(gaps, aspectGap(feature, vendor,
			map[string]any{"aspect": "temporary", "temporary_scope": ddl.TemporaryScope}))
	}
	for _, column := range ddl.Columns {
		gaps = append(gaps, columnAspectGaps(dialect, ddl.Operation, "", column)...)
	}
	indexPartGaps := func(prefix string, index spec.Index) {
		// Expression key parts are official syntax on both dialects — TiDB
		// documents expression indexes (LOWER() among the allowed functions)
		// and MySQL 8.0.13+ ships functional key parts — but the audit model
		// does not inspect expressions, so the aspect is unaudited on both.
		// Prefix lengths and descending parts are likewise merely unaudited.
		if index.HasExpressionKeys {
			gaps = append(gaps, aspectGap(
				fmt.Sprintf("%s.expr", prefix), false,
				map[string]any{"aspect": "index", "index_kind": string(index.Kind), "expression_count": index.ExpressionCount},
			))
		}
		if index.PrefixParts > 0 {
			gaps = append(gaps, aspectGap(
				fmt.Sprintf("%s.prefix", prefix), false,
				map[string]any{"aspect": "index", "index_kind": string(index.Kind), "prefix_parts": index.PrefixParts},
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
		for _, option := range index.UnmodeledOptions {
			// split_opt, secondary_engine_attr, and columnar_replica are TiDB
			// extensions: vendor boundaries under MySQL, unaudited under TiDB.
			vendor := dialect == spec.DialectMySQL && (option == "split_opt" || option == "secondary_engine_attr" || option == "columnar_replica")
			gaps = append(gaps, aspectGap(
				fmt.Sprintf("%s.option.%s", prefix, option), vendor,
				map[string]any{"aspect": "index_option", "index_kind": string(index.Kind)},
			))
		}
		if index.DescParts > 0 {
			gaps = append(gaps, aspectGap(
				fmt.Sprintf("%s.desc", prefix), false,
				map[string]any{"aspect": "index", "index_kind": string(index.Kind), "desc_parts": index.DescParts},
			))
		}
	}
	if ddl.PrimaryKey != nil && ddl.PrimaryKey.Global {
		gaps = append(gaps, aspectGap(
			fmt.Sprintf("%s.index.global", ddl.Operation), dialect == spec.DialectMySQL,
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
		if index.Global {
			// The GLOBAL modifier is TiDB-only syntax (global indexes on
			// partitioned tables); under MySQL it is a vendor boundary.
			gaps = append(gaps, aspectGap(
				fmt.Sprintf("%s.index.global", ddl.Operation), dialect == spec.DialectMySQL,
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
	for _, alter := range ddl.Alter {
		if ddl.Operation == spec.DDLOperationAlterTable && !auditedAlterAction(dialect, alter.Action) {
			// PLACEMENT POLICY is TiDB-only: under MySQL the action is a vendor
			// boundary, not merely unaudited analysis.
			vendor := dialect == spec.DialectMySQL && alter.Action == "placement_policy"
			gaps = append(gaps, aspectGap(
				fmt.Sprintf("%s.%s", ddl.Operation, alter.Action), vendor,
				map[string]any{"aspect": "action", "action": alter.Action},
			))
			continue
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
			if alter.Index.Definition.Global {
				gaps = append(gaps, aspectGap(
					fmt.Sprintf("%s.%s.index.global", ddl.Operation, alter.Action), dialect == spec.DialectMySQL,
					map[string]any{"aspect": "index", "action": alter.Action, "index_kind": string(alter.Index.Definition.Kind)},
				))
			}
			indexPartGaps(fmt.Sprintf("%s.%s.index", ddl.Operation, alter.Action), *alter.Index.Definition)
		}
		if alter.Column != nil && alter.Column.Definition != nil {
			gaps = append(gaps, columnAspectGaps(dialect, ddl.Operation, alter.Action, *alter.Column.Definition)...)
		}
		if alter.HasColumnPosition {
			gaps = append(gaps, aspectGap(
				fmt.Sprintf("%s.%s.column_position", ddl.Operation, alter.Action), false,
				map[string]any{"aspect": "column_position", "action": alter.Action},
			))
		}
		for _, option := range alter.UnextractedOptions {
			gaps = append(gaps, aspectGap(
				fmt.Sprintf("%s.option.%s", ddl.Operation, option), tidbOnlyTableOption(option, dialect),
				map[string]any{"aspect": "option", "action": alter.Action},
			))
		}
	}
	return gaps
}

// statementCoverage resolves the coverage status for one extracted statement:
// parser-marked boundaries and any aspect gap make the statement incomplete;
// everything else is complete.
func statementCoverage(dialect spec.Dialect, statement spec.Statement) (report.Coverage, []spec.UnsupportedDetail) {
	if statement.Unsupported != nil {
		return report.Coverage{Status: report.CoverageIncomplete}, nil
	}
	gaps := statementCoverageAspects(dialect, statement)
	if len(gaps) > 0 {
		return report.Coverage{Status: report.CoverageIncomplete}, gaps
	}
	return report.Coverage{Status: report.CoverageComplete}, nil
}
