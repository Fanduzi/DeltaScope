package audit

// Test-only exports for `package audit_test` rows that must combine
// batchState observation hooks with the REAL mysqlmeta provider.
// `package audit` cannot import mysqlmeta (provider.go asserts
// audit.MetadataProvider — an import cycle), while `package
// audit_test` may import both sides. These wrappers keep the internal
// hooks reachable there without widening the product surface
// (T26-SRS-DROP-PROVEN-ABSENT-EXECUTION-01 R2-B1).

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// TestBatchState aliases the internal batch state so external test
// files can hold it and use the exported methods below.
type TestBatchState = batchState

// NewTestBatchState mirrors newSRSState — the golden-version state
// used by the internal SRS rows.
func NewTestBatchState(provider MetadataProvider) *batchState {
	return newSRSState(provider)
}

// EnrichSRSPrivForTest mirrors enrichSRSPriv — one real
// enrichStatements call under the r3 privilege fixture.
func EnrichSRSPrivForTest(t *testing.T, state *batchState, provider MetadataProvider, sqlText string) []spec.Statement {
	t.Helper()
	return enrichSRSPriv(t, state, provider, sqlText)
}

// EnrichSRSPrivErrForTest is the error-returning twin of
// EnrichSRSPrivForTest — the OP-4c ordered-reader error row asserts
// the propagated sentinel instead of t.Fatalf (R: no adjudication
// runs on the failing statement).
func EnrichSRSPrivErrForTest(t *testing.T, state *batchState, provider MetadataProvider, sqlText string) ([]spec.Statement, error) {
	t.Helper()
	statements, failures := parseT26(t, sqlText)
	return state.enrichStatements(context.Background(),
		&MetadataRequest{Schema: "golden", Provider: provider,
			PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession},
		statements, failures, state.instanceFacts, auditLimits{orderedStatements: defaultOrderedStatementLimit})
}

// EvaluateEnrichedForTest mirrors evaluateEnriched — the real rule
// registry over the default policy.
func EvaluateEnrichedForTest(t *testing.T, stmts []spec.Statement) report.Result {
	t.Helper()
	return evaluateEnriched(t, stmts)
}

// BatchStateStaleSet mirrors staleSet — a copy of the current per-SRID
// existence-stale key set.
func BatchStateStaleSet(s *batchState) map[uint32]bool { return staleSet(s) }

// BatchStatePrivObservationSUPER mirrors privObservationSUPER — the
// positive privilege-observation fixture.
func BatchStatePrivObservationSUPER() *spec.MySQLServerDropPrivilegeObservation {
	return privObservationSUPER()
}

// SeedSRSExistenceStale injects per-SRID existence-stale keys — the
// PA-5 pre-state rows seed before the enrich.
func (s *batchState) SeedSRSExistenceStale(ids ...uint32) {
	for _, id := range ids {
		s.srsExistenceStale[id] = struct{}{}
	}
}

// SetSRSDepsStale seeds the batch dependency-staleness flag.
func (s *batchState) SetSRSDepsStale(v bool) { s.srsDepsStale = v }

// SRSDepsStale reads the batch dependency-staleness flag.
func (s *batchState) SRSDepsStale() bool { return s.srsDepsStale }

// ScopeEnded reads the privilege-observation scope-ended flag.
func (s *batchState) ScopeEnded() bool { return s.serverPrivilegeScopeEnded }

// Contaminated reads the batch-contamination flag.
func (s *batchState) Contaminated() bool { return s.contaminated }

// SetAfterApplyObserve installs the per-statement post-apply hook —
// the three-timepoint snapshot seam (R2-B4).
func (s *batchState) SetAfterApplyObserve(fn func(int, *spec.Statement)) {
	s.afterApplyObserve = fn
}

// SeedResolvedVersion overrides the resolved version identity — the
// PA-6e internal companion pins the out-of-domain identity the
// request layer would have computed from the observed banner
// (R3-C3); the version gate below decides audit applicability,
// never read applicability.
func (s *batchState) SeedResolvedVersion(version *spec.VersionIdentity) {
	s.resolvedVersion = version
}

// SRSOrderedEntryView is a deep-copy read-out of one ordered-channel
// entry's provenance — the OP rows assert mint/keep/barrier through
// this independent view, never the live pointer
// (T26-SRS-DROP-ORDERED-PUBLICATION-EXECUTION-01).
type SRSOrderedEntryView struct {
	SRID         uint32
	Absent       bool
	SourceArm    spec.SRSDropArm
	OwnPrivilege spec.SRSDropPrivilege
	PublishedIdx int
	Barrier      bool
}

// SRSOrderedEntry returns a copy of the entry for srid — (view,false)
// means no entry exists.
func (s *batchState) SRSOrderedEntry(srid uint32) (SRSOrderedEntryView, bool) {
	entry, ok := s.srsOrdered[srid]
	if !ok {
		return SRSOrderedEntryView{}, false
	}
	return SRSOrderedEntryView{
		SRID:         entry.srid,
		Absent:       entry.absent,
		SourceArm:    entry.sourceArm,
		OwnPrivilege: entry.ownPrivilege,
		PublishedIdx: entry.publishedIdx,
		Barrier:      entry.barrier,
	}, true
}

// SRSOrderedSRIDs returns the channel's live key set — exact-set
// sibling-isolation assertions (OP-8c) consume it.
func (s *batchState) SRSOrderedSRIDs() []uint32 {
	ids := make([]uint32, 0, len(s.srsOrdered))
	for id := range s.srsOrdered {
		ids = append(ids, id)
	}
	return ids
}
