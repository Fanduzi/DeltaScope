// Package audit: T26-SRS-DROP-EXECUTION-01 — the spatial-touching
// classifier and the SRS family's own apply. spatialTouching decides
// whether a non-SRS statement's modeled effect may have written a
// geometry-column or SRID fact — minting the batch-wide
// srsDepsStale flag — over the normalized statement model only
// (contract D3: first-match, never SQL text). applySRSStatement is the
// own apply for audited SRS lifecycle statements: only the shared
// adjudication's unproven arm mints the authored SRID's existence
// staleness — blocked, refused, and proven-absence arms write nothing
// (T26-SRS-DROP-PROVEN-ABSENT-EXECUTION-01: an absent drop never
// lands, and the arm already carries that truth, so no stale flag is
// minted and no state is cleared).
package audit

import (
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// srsOrderedEntry is the ordered channel's per-SRID proven-absence
// record (T26-SRS-DROP-ORDERED-PUBLICATION-EXECUTION-01): internal
// provenance, never a second catalog — it records that a certified
// absence was established by a specific statement at a specific
// position, and consumers authenticate it by authored SRID before use.
type srsOrderedEntry struct {
	// srid is the authored identity — the binding key.
	srid uint32
	// absent is the certified fact (the only true value carried).
	absent bool
	// sourceArm records which A arm published the entry (provenance).
	sourceArm spec.SRSDropArm
	// ownPrivilege is the producing statement's actual premise state —
	// provenance, not a consumption record; whether that statement
	// consumed its premise is statement-level and never an entry field.
	ownPrivilege spec.SRSDropPrivilege
	// publishedIdx is the producer's statement index within its enrich
	// call — a later enrich's indices restart and never re-key the
	// first producer's provenance (earliest wins).
	publishedIdx int
	// barrier marks the entry shadowed by a same-SRID boundary/write or
	// unproven stale mint — the entry is kept for provenance while the
	// minted srsExistenceStale flag independently blocks consumption.
	barrier bool
}

// applySRSStatement is the SRS family's own apply on the audited path.
// The shared adjudication already classified the arm — the own apply
// never re-derives it: only the unproven arm (a drop whose drop may
// have landed) mints the authored SRID's existence staleness for every
// later statement; blocked, SRID-0, dependency-refused, and every
// proven-absence arm (absent_refused, absent_noop, absent_conditional)
// write no stale flag (T26-SRS-DROP-EXECUTION-01 writer table, A-arm
// extension T26-SRS-DROP-PROVEN-ABSENT-EXECUTION-01).
//
// Ordered publication (T26-SRS-DROP-ORDERED-PUBLICATION-EXECUTION-01):
// an A arm whose bound state carries a catalog-certified absence
// (ValidAbsence — never an ordered-served marker) mints the SRID's
// ordered entry on first publication and keeps the earliest producer's
// provenance on re-publication; an ordered-served A consumes the
// existing entry and never re-mints. The unproven arm's stale mint
// additionally flips an existing same-SRID entry's barrier.
func (s *batchState) applySRSStatement(statement spec.Statement) error {
	decl := statement.DDL.SpatialReference
	adjudication := (*spec.SRSDropAdjudication)(nil)
	bound := (*spec.SpatialReferenceSystemState)(nil)
	if statement.Metadata != nil {
		adjudication = statement.Metadata.SRSDrop
		bound = statement.Metadata.SpatialReference
	}
	if adjudication == nil {
		return nil
	}
	switch adjudication.Arm {
	case spec.SRSDropArmAbsentRefused, spec.SRSDropArmAbsentNoop, spec.SRSDropArmAbsentConditional:
		// Only a catalog-certified absence publishes: an ordered-served
		// A arm consumed the existing entry and is done — no re-mint,
		// no new source attribution.
		if !spec.ValidAbsence(bound, decl.SRID) {
			return nil
		}
		// Earliest wins — an existing entry's first-producer
		// provenance (sourceArm/ownPrivilege/publishedIdx) stands.
		if s.srsOrdered == nil {
			s.srsOrdered = make(map[uint32]*srsOrderedEntry)
		}
		if _, ok := s.srsOrdered[decl.SRID]; !ok {
			s.srsOrdered[decl.SRID] = &srsOrderedEntry{
				srid:         decl.SRID,
				absent:       true,
				sourceArm:    adjudication.Arm,
				ownPrivilege: adjudication.Privilege,
				publishedIdx: s.applyStmtIndex,
			}
		}
	case spec.SRSDropArmUnproven:
		s.srsExistenceStale[decl.SRID] = struct{}{}
		// The stale mint shadows any earlier same-SRID absence entry —
		// barrier flips, provenance kept (barrier ⇒ stale flag holds by
		// construction: the same apply mints both).
		if entry := s.srsOrdered[decl.SRID]; entry != nil {
			entry.barrier = true
		}
	}
	return nil
}

// spatialTouching is the contract's D3 first-match decision over the
// normalized statement model: whether this non-SRS statement's modeled
// effect space may have written a geometry-column or SRID fact and so
// mints the batch-wide dependency staleness. Every condition reads
// bound model fields; anything unprovable resolves toward touching.
// The SRS family itself is out of scope (D3-0) — its own rule governs
// existence staleness.
func (s *batchState) spatialTouching(statement spec.Statement) bool {
	ddl := statement.DDL
	// D3-0: any SRS-family statement (audited or boundary) exits to
	// its own lifecycle handling — never classified here.
	if ddl != nil && ddl.SpatialReference != nil {
		return false
	}
	// D3-1: provably non-table model markers — non-DDL statements,
	// the SERVER and tablespace families, and ALTER INSTANCE boundary
	// features cannot write a geometry-column/SRID fact.
	if ddl == nil || ddl.Server != nil || ddl.Tablespace != nil {
		return false
	}
	if statement.Unsupported != nil && strings.HasPrefix(statement.Unsupported.Feature, "alter_instance") {
		return false
	}
	if _, inert := srsSpatialInertOps[ddl.Operation]; inert {
		return false
	}
	// D3-2: a CREATE TABLE is inert only under the complete
	// inert-CREATE predicate — every clause proven on bound fields.
	if ddl.Operation == spec.DDLOperationCreateTable && s.srsInertCreateTable(statement) {
		return false
	}
	// D3-3: an ALTER TABLE is inert only under the complete
	// inert-ALTER predicate — every member action a table_option
	// whose keys are exactly engine.
	if ddl.Operation == spec.DDLOperationAlterTable && s.srsInertAlterTable(statement) {
		return false
	}
	// D3-4/D3-5: table targets, table-affecting or unknown
	// operations, and every unclassifiable remainder resolve touching.
	return true
}

// srsSpatialInertOps is the contract's inert-op set — spec.DDLOperation
// values whose modeled family never writes schema/table column state.
// Enumerated, never inferred; the SERVER verbs deliberately stay out
// of this set (their family marker names them directly at D3-1).
var srsSpatialInertOps = map[spec.DDLOperation]struct{}{
	spec.DDLOperationCreateView:              {},
	spec.DDLOperationAlterView:               {},
	spec.DDLOperationDropView:                {},
	spec.DDLOperationCreateIndex:             {},
	spec.DDLOperationDropIndex:               {},
	spec.DDLOperationCreateSequence:          {},
	spec.DDLOperationAlterSequence:           {},
	spec.DDLOperationDropSequence:            {},
	spec.DDLOperationCreateMaterializedView:  {},
	spec.DDLOperationDropMaterializedView:    {},
	spec.DDLOperationRefreshMaterializedView: {},
	spec.DDLOperationCreateType:              {},
	spec.DDLOperationAlterType:               {},
	spec.DDLOperationDropType:                {},
	spec.DDLOperationCreateDomain:            {},
	spec.DDLOperationAlterDomain:             {},
	spec.DDLOperationDropDomain:              {},
	spec.DDLOperationCreateExtension:         {},
	spec.DDLOperationAlterExtension:          {},
	spec.DDLOperationDropExtension:           {},
	spec.DDLOperationGrantTable:              {},
	spec.DDLOperationRevokeTable:             {},
	spec.DDLOperationCreatePolicy:            {},
	spec.DDLOperationAlterPolicy:             {},
	spec.DDLOperationDropPolicy:              {},
	spec.DDLOperationCreateTrigger:           {},
	spec.DDLOperationDropTrigger:             {},
	spec.DDLOperationCreateFunction:          {},
	spec.DDLOperationDropFunction:            {},
	spec.DDLOperationCreateProcedure:         {},
	spec.DDLOperationDropProcedure:           {},
	spec.DDLOperationCreateUser:              {},
	spec.DDLOperationAlterUser:               {},
	spec.DDLOperationDropUser:                {},
	spec.DDLOperationCreateRole:              {},
	spec.DDLOperationDropRole:                {},
	spec.DDLOperationGrant:                   {},
	spec.DDLOperationRevoke:                  {},
	spec.DDLOperationCreatePlacementPolicy:   {},
	spec.DDLOperationAlterPlacementPolicy:    {},
	spec.DDLOperationDropPlacementPolicy:     {},
	spec.DDLOperationAlterRangePlacement:     {},
	spec.DDLOperationCreateResourceGroup:     {},
	spec.DDLOperationAlterResourceGroup:      {},
	spec.DDLOperationDropResourceGroup:       {},
	spec.DDLOperationCreateTablespace:        {},
	spec.DDLOperationAlterTablespace:         {},
	spec.DDLOperationDropTablespace:          {},
	spec.DDLOperationCreateLogfileGroup:      {},
	spec.DDLOperationAlterLogfileGroup:       {},
	spec.DDLOperationDropLogfileGroup:        {},
	spec.DDLOperationSplitRegion:             {},
	spec.DDLOperationCreateSchema:            {},
}

// srsInertCreateTable is the contract's complete inert-CREATE
// predicate (D3-2): the statement provably cannot write a
// geometry-column/SRID fact only when every clause holds on bound
// model fields — no LIKE/SELECT discriminators, a non-empty complete
// explicit column list carrying no spatial-family type, no DDL-level
// unextracted options, no per-column unextracted options, and no
// column-aspect coverage gap.
func (s *batchState) srsInertCreateTable(statement spec.Statement) bool {
	ddl := statement.DDL
	if ddl.HasReferTable || ddl.ReferTable != nil || ddl.HasSelect || len(ddl.SelectSources) > 0 {
		return false
	}
	if len(ddl.Columns) == 0 || len(ddl.UnextractedOptions) > 0 {
		return false
	}
	for i := range ddl.Columns {
		column := &ddl.Columns[i]
		if _, spatial := srsSpatialColumnTypes[strings.ToLower(strings.TrimSpace(column.Type))]; spatial {
			return false
		}
		if len(column.UnextractedOptions) > 0 {
			return false
		}
	}
	return !srsHasCoverageAspect(s.dialect, statement, "column")
}

// srsInertAlterTable is the contract's complete inert-ALTER predicate
// (D3-3): a non-empty complete action list where every member is a
// table_option whose option keys are exactly "engine", every member's
// UnextractedOptions is empty, and no action/option/column coverage
// gap exists.
func (s *batchState) srsInertAlterTable(statement spec.Statement) bool {
	ddl := statement.DDL
	if len(ddl.Alter) == 0 {
		return false
	}
	for i := range ddl.Alter {
		alter := &ddl.Alter[i]
		if alter.Action != "table_option" || len(alter.UnextractedOptions) > 0 {
			return false
		}
		for key := range alter.Options {
			if key != "engine" {
				return false
			}
		}
	}
	return !srsHasCoverageAspect(s.dialect, statement, "action", "option", "column")
}

// srsHasCoverageAspect reports whether the statement's computed
// coverage aspects carry any of the named aspect labels — the
// completeness clause both inert predicates bind to.
func srsHasCoverageAspect(dialect spec.Dialect, statement spec.Statement, names ...string) bool {
	for _, detail := range statementCoverageAspects(dialect, statement) {
		if aspect, ok := detail.Metadata["aspect"].(string); ok {
			for _, name := range names {
				if aspect == name {
					return true
				}
			}
		}
	}
	return false
}

// srsSpatialColumnTypes is the normalized spatial-family column type
// vocabulary — the registered set an inert CREATE may never carry.
var srsSpatialColumnTypes = map[string]struct{}{
	"geometry":           {},
	"point":              {},
	"linestring":         {},
	"polygon":            {},
	"multipoint":         {},
	"multilinestring":    {},
	"multipolygon":       {},
	"geometrycollection": {},
}
