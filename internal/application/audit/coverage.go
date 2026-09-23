// Package audit computes statement and aggregate audit-coverage status.
// input: extracted domain statements with parser-attached boundary markers and extraction facts
// output: per-statement coverage status plus bounded unsupported evidence for unaudited aspects
// pos: application coverage classification between extraction and reporting (issue #82)
// note: if this file changes, update this header and module README.md.
package audit

import (
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// auditedAlterActionsMySQL lists ALTER TABLE action names whose extraction and
// rule semantics are audited. Every other recognized action is an evidence gap
// that keeps the statement but marks coverage incomplete. The set mirrors the
// generic-notice and semantically-checked rows of the official DDL inventory:
// column, index, key/constraint, rename, table-option, and online-DDL clauses.
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
	"algorithm":        {},
	"lock":             {},
	"index_invisible":  {},
}

// auditedAlterActionsTiDB extends the shared audited set with the TiDB
// placement binding and check lifecycle the inventory covers at notice level.
var auditedAlterActionsTiDB = func() map[string]struct{} {
	set := make(map[string]struct{}, len(auditedAlterActionsMySQL)+2)
	for action := range auditedAlterActionsMySQL {
		set[action] = struct{}{}
	}
	set["placement_policy"] = struct{}{}
	set["drop_check"] = struct{}{}
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
// semantics. gap marks an evidence hole; vendor marks a product boundary.
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

// alterConstraintGap classifies constraint types on alter actions that parse
// but lack audited semantics. MySQL covers check constraints only through
// create-table extraction, so check-bearing alter actions stay incomplete
// there; TiDB's fk-check row covers ADD/DROP CHECK at notice level.
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
		gaps = append(gaps, aspectGap(
			fmt.Sprintf("%s.option.%s", ddl.Operation, option), false,
			map[string]any{"aspect": "option"},
		))
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
			gaps = append(gaps, aspectGap(
				fmt.Sprintf("%s.%s", ddl.Operation, alter.Action), false,
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
