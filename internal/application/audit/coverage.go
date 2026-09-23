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
	}
	return false, false
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
		// A dropped placement-policy option is still a vendor boundary under
		// MySQL — MySQL has no placement surface regardless of modeling.
		vendor := dialect == spec.DialectMySQL && option == "placement_policy"
		gaps = append(gaps, aspectGap(
			fmt.Sprintf("%s.option.%s", ddl.Operation, option), vendor,
			map[string]any{"aspect": "option"},
		))
	}
	for _, name := range sortedOptionNames(ddl.Options) {
		if gap, vendor := extractedOptionGap(dialect, ddl.Operation, name); gap {
			feature := fmt.Sprintf("%s.option.%s", ddl.Operation, name)
			if name == "has_options" {
				feature = fmt.Sprintf("%s.options", ddl.Operation)
			}
			gaps = append(gaps, aspectGap(feature, vendor, map[string]any{"aspect": "option"}))
		}
	}
	for _, index := range ddl.Indexes {
		if gap, vendor := indexKindGap(dialect, index.Kind); gap {
			gaps = append(gaps, aspectGap(
				fmt.Sprintf("%s.index.%s", ddl.Operation, index.Kind), vendor,
				map[string]any{"aspect": "index", "index_kind": string(index.Kind)},
			))
		}
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
		}
		if alter.Index != nil && alter.Index.Definition != nil {
			if gap, vendor := indexKindGap(dialect, alter.Index.Definition.Kind); gap {
				gaps = append(gaps, aspectGap(
					fmt.Sprintf("%s.%s.%s", ddl.Operation, alter.Action, alter.Index.Definition.Kind), vendor,
					map[string]any{"aspect": "index", "action": alter.Action, "index_kind": string(alter.Index.Definition.Kind)},
				))
			}
		}
		for _, option := range alter.UnextractedOptions {
			gaps = append(gaps, aspectGap(
				fmt.Sprintf("%s.option.%s", ddl.Operation, option), false,
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
