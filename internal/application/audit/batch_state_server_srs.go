// Package audit: T26/#105 provider round — the MySQL SERVER and spatial
// reference system fact bindings. Each family resolves its one authored
// identity through the shared object-resolver seam (the once-per-request
// objectCache lookup), then binds the provider's fact projection onto
// statement metadata. There is no ordered registry and no lifecycle
// transition this round — the families keep their named boundaries and
// these facts are provider evidence only, never post-state.
//
// Knownness stays per-fact: a statement naming a family always binds a
// state (the fact was bound), while Read/UnreadReason and the nil
// sub-members keep "not loaded", "identity unproven", "version
// inapplicable", and "read ran, answer unknown" distinct. Resolver
// failures propagate — a failed read is never laundered into an
// unvouched state.
package audit

import (
	"context"
	"strconv"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// objectResolverFor returns the family resolver — the once-per-request
// cached seam when the enrich loop wired it, else the provider's own
// ObjectResolver — or nil when no resolver exists.
func (s *batchState) objectResolverFor(cached func(ctx context.Context, lookup spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error)) func(ctx context.Context, lookup spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if cached != nil {
		return cached
	}
	provider, ok := s.provider.(ObjectResolver)
	if !ok {
		return nil
	}
	return func(ctx context.Context, lookup spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
		snapshot, err := provider.ResolveObject(ctx, s.dialect, lookup)
		if err != nil {
			return nil, metadataObjectReadError{cause: err}
		}
		return snapshot, nil
	}
}

// bindMySQLServer resolves the one server identity a SERVER-family
// statement names and binds the adjudicated existence fact — the
// ordered channel merged over the provider answer (T26-SERVER-DROP-01,
// the accepted six-step order in
// docs/verification/t26-semantics-01): whole-batch contamination and
// channel taint exit first; the relation scan decides every barrier
// before any resolver or cache access; only a clean channel reaches
// the provider seam. The catalog read fires only when the authored
// name is a provable identifier (decl.NameIdent) — a string-literal
// spelling stays an unissued read with the identity-unproven reason,
// never a normalized-identity query and never a cached absence. A
// contaminated batch or a non-MySQL dialect binds the state
// unvouched; resolver failures propagate.
func (s *batchState) bindMySQLServer(ctx context.Context, statement spec.Statement) (*spec.ServerState, error) {
	decl := statement.DDL.Server
	bound := &spec.ServerState{Name: decl.Name}
	if s.dialect != spec.DialectMySQL || s.contaminated {
		return bound, nil
	}
	if s.serverChannelTainted || (decl.NameIdent && s.resolveServerBarrier(decl.Name)) {
		// The ordered overlay outranks the provider seam: an
		// undischarged unknown effect (channel taint, unproven-relation
		// entry or recorded write, an entry already holding a barrier)
		// shadows the audit-time catalog answer — no resolver call on
		// this path.
		bound.UnreadReason = spec.MetadataUnreadBatchEffects
		return bound, nil
	}
	if decl.NameIdent {
		// Step 5 of the accepted order: the ordered channel's proven
		// answers — OrderedAbsent and OrderedPresent — answer before
		// any provider or cache access, only on the provably-same
		// identity, and only while the channel carried no barrier or
		// taint (decided above). A shadowed or stale answer never
		// reaches this point — the barrier path already exited
		// (T26-SERVER-DROP-EXECUTION-01, T26-SERVER-CREATE-EXECUTION-01).
		// Consumers read the ordered fields directly; nothing here
		// fakes catalog Exists/Read/Status.
		for _, entry := range s.serverEntries {
			if serverRel(decl.Name, entry.name) != serverRelSame {
				continue
			}
			if entry.absent {
				return &spec.ServerState{
					Name:                     decl.Name,
					OrderedAbsent:            true,
					OrderedAbsentConditional: entry.publishedArm == spec.ServerDropArmProven || entry.publishedArm == spec.ServerDropArmConvergent,
					OrderedAbsentSourceSet:   entry.publishedIndexSet,
					OrderedAbsentSourceIndex: entry.publishedIndex,
				}, nil
			}
			if entry.present {
				return &spec.ServerState{
					Name:                      decl.Name,
					OrderedPresent:            true,
					OrderedPresentConditional: entry.presentConditional,
				}, nil
			}
		}
	}
	resolver := s.objectResolverFor(s.mysqlServerResolver)
	if resolver == nil {
		return bound, nil
	}
	lookup := spec.ObjectLookupRequest{Type: "mysql_server", Name: decl.Name}
	if decl.NameIdent {
		lookup.Qualifiers = map[string]string{
			spec.ObjectLookupQualifierNameForm: spec.ObjectLookupNameFormIdentifier,
		}
	}
	snapshot, err := resolver(ctx, lookup)
	if err != nil {
		return nil, err
	}
	if snapshot != nil && snapshot.Server != nil {
		return snapshot.Server, nil
	}
	return bound, nil
}

// bindMySQLSRS resolves the one SRID identity a spatial reference
// system statement names and binds the provider's fact set — existence
// plus the NAME/ORGANIZATION conflict witnesses and column dependencies
// the request qualifies. A DROP carries no conflict qualifiers —
// conflict premises are not its contract; the CREATE family qualifies
// its reads with the authored NAME / ORGANIZATION+IDENTIFIED BY
// spellings the statement itself inserts into the unique keys
// (T26-SRS-CREATE-PUBLICATION-01 frozen F-6). The SRID keeps the
// existence and dependency reads independently provable. Resolver
// failures propagate.
func (s *batchState) bindMySQLSRS(ctx context.Context, statement spec.Statement) (*spec.SpatialReferenceSystemState, error) {
	decl := statement.DDL.SpatialReference
	bound := &spec.SpatialReferenceSystemState{SRID: decl.SRID}
	if s.dialect != spec.DialectMySQL {
		return bound, nil
	}
	// Stale-state ordering (T26-SRS-DROP-EXECUTION-01): whole-batch
	// contamination or the same SRID's existence stale flag blocks
	// before any resolver/cache consumption — the ordered channel's
	// unread vocabulary marks the blocked read, and a fresh database
	// read never clears a minted flag. Dependency staleness is not
	// checked here: it invalidates witness consumption inside the
	// adjudication, never a still-valid existence fact, and never
	// suppresses the reads the contract requires.
	if s.contaminated {
		bound.UnreadReason = spec.MetadataUnreadBatchEffects
		return bound, nil
	}
	if _, stale := s.srsExistenceStale[decl.SRID]; stale {
		bound.UnreadReason = spec.MetadataUnreadBatchEffects
		return bound, nil
	}
	// A present mint from an earlier CREATE-arm apply is the batch's
	// proven presence — the write landed (or existed regardless for OR
	// REPLACE), so the bound state reads present without a catalog
	// round-trip (T26-SRS-CREATE-PUBLICATION-01 frozen F-8).
	if s.srsExistencePresent[decl.SRID] {
		bound.Read = true
		bound.Exists = true
		return bound, nil
	}
	// Ordered channel (T26-SRS-DROP-ORDERED-PUBLICATION-EXECUTION-01):
	// an audited drop consumes a batch-proven absence entry only while
	// BOTH the active same-SRID entry and the deps-reader capability
	// hold — the ordered branch commits (marker bound, deps-only read
	// issued) or never happens; a barrier'd entry or a nil reader falls
	// through to the existing full path untouched (the entry is not
	// consumed, no OrderedAbsent is written). The deps-only response is
	// the statement's fresh witness — the skipped existence read is
	// never laundered into Read/Exists/Absence.
	if s.orderedSRSDropEligible(statement) {
		if entry := s.srsOrdered[decl.SRID]; entry != nil && entry.absent && !entry.barrier && s.srsDepsReader != nil {
			deps, err := s.srsDepsReader(ctx, decl.SRID)
			if err != nil {
				return nil, err
			}
			bound.OrderedAbsent = true
			bound.Dependencies = deps
			return bound, nil
		}
	}
	resolver := s.objectResolverFor(s.mysqlSRSResolver)
	if resolver == nil {
		return bound, nil
	}
	lookup := spec.ObjectLookupRequest{
		Type: "mysql_spatial_reference_system",
		Name: decl.SRIDName(),
	}
	// The CREATE family qualifies its conflict reads with the authored
	// NAME / ORGANIZATION+IDENTIFIED BY identities — the spellings the
	// statement itself inserts into the unique keys (T26-SRS-CREATE-
	// PUBLICATION-01 frozen F-6). A DROP still carries no qualifiers:
	// conflict premises are not its contract.
	if decl.Kind == spec.SRSKindCreate || decl.Kind == spec.SRSKindCreateOrReplace {
		lookup.Qualifiers = map[string]string{}
		if decl.Attrs.HasName {
			lookup.Qualifiers[spec.ObjectLookupQualifierSRSName] = decl.Attrs.Name
		}
		if decl.Attrs.HasOrganization && decl.Attrs.HasIdentifiedBy {
			lookup.Qualifiers[spec.ObjectLookupQualifierSRSOrg] = decl.Attrs.OrgName
			lookup.Qualifiers[spec.ObjectLookupQualifierSRSOrgID] = strconv.FormatUint(uint64(decl.Attrs.OrgID), 10)
		}
	}
	snapshot, err := resolver(ctx, lookup)
	if err != nil {
		return nil, err
	}
	if snapshot != nil && snapshot.SpatialReference != nil {
		return snapshot.SpatialReference, nil
	}
	return bound, nil
}

// orderedSRSDropEligible is the ordered channel's admission gate —
// bindMySQLSRS serves every MySQL SRS declaration (audited drops AND
// boundary writes), so ordered consumption is gated by an explicit
// predicate evaluated inside the bind: MySQL dialect, a drop kind, no
// unsupported marker, and the already-bound resolvedVersion inside the
// audited domain. A statement failing any conjunct never reaches the
// ordered lookup and takes the pre-existing resolver path unchanged;
// no new SRID-0 or boundary pre-read short-circuit is introduced
// (T26-SRS-DROP-ORDERED-PUBLICATION-EXECUTION-01 contract §4.1).
func (s *batchState) orderedSRSDropEligible(statement spec.Statement) bool {
	decl := statement.DDL.SpatialReference
	return s.dialect == spec.DialectMySQL &&
		decl.Kind == spec.SRSKindDrop &&
		statement.Unsupported == nil &&
		spec.MySQLSRSDropAuditedDomain(s.resolvedVersion)
}
