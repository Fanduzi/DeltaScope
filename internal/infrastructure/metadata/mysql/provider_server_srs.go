// Package mysqlmeta: T26/#105 provider round — the MySQL SERVER and
// spatial reference system (SRS) fact readers resolved through the
// official ResolveObject seam. Both families are MySQL-only catalog
// surfaces and stay dialect-gated: under TiDB the resolver never emits
// these queries. The reads are on-demand per fact — an unprovable
// identity, an inapplicable proven version, or a statement that never
// names the family issues no dedicated query.
//
// Zero-row contract (R2 metadata fact scheme): mysql.servers and
// ST_GEOMETRY_COLUMNS stay privilege-scoped surfaces with no approved
// completeness contract — a successful empty answer stays unknown.
// ST_SPATIAL_REFERENCE_SYSTEMS is the registered exception
// (T26-SRS-DROP-PROVEN-ABSENT-EXECUTION-01): the accepted completeness
// proof establishes the view as a bare projection of the DD table, so
// a clean zero-observed-row answer under the same Qo observation's
// D-proof supplies Absence=true. Every other surface's zero-row
// answer and every zero-row answer outside the D-proof stays unknown.
// Query, scan, iteration, and driver errors propagate wrapped on
// every version surface — a query that already issued and failed is
// never laundered into "not loaded", and failures are never cached as
// absence.
//
// Identity contract: the SERVER name read fires only when the request
// marks the name a complete decoded identifier (name_form=identifier) —
// an undecoded string-literal spelling is never a normalized catalog
// identity. The SRS reads key on the numeric SRID, which is always
// provable; the NAME and ORGANIZATION conflict reads fire only when the
// request carries provable qualifiers — never from authored spellings.
// DEFINITION and DESCRIPTION columns are never selected.
package mysqlmeta

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// loadMySQLServerObject resolves one MySQL SERVER identity through
// mysql.servers — the bounded existence read only. Connection columns
// (HOST/DB/USERNAME/PASSWORD/PORT/SOCKET/WRAPPER/OWNER) are never
// selected: the fact is the object's presence, never its connection
// payload. A row proves presence; a clean zero-row answer stays unknown
// — mysql.servers visibility is grant-scoped and no approved
// completeness contract exists — and a missing qualifier marks the
// authored name an unprovable spelling, so the read is never issued.
// Query and scan errors propagate wrapped — never absence — and
// failures are never cached.
func (p *Provider) loadMySQLServerObject(ctx context.Context, request spec.ObjectLookupRequest, snapshot *spec.ObjectSnapshot) (*spec.ObjectSnapshot, error) {
	state := &spec.ServerState{Name: request.Name}
	snapshot.Server = state
	if request.Qualifiers[spec.ObjectLookupQualifierNameForm] != spec.ObjectLookupNameFormIdentifier {
		state.UnreadReason = spec.MetadataUnreadIdentityUnproven
		return snapshot, nil
	}
	state.Read = true
	row := p.db.QueryRowContext(ctx, `
		select server_name
		from mysql.servers
		where server_name = ?
	`, request.Name)
	var name sql.NullString
	if err := row.Scan(&name); err != nil {
		if err == sql.ErrNoRows {
			return snapshot, nil
		}
		return nil, fmt.Errorf("query mysql server snapshot: %w", err)
	}
	state.Exists = true
	state.Status = spec.MetadataStatusConfirmed
	if name.Valid {
		state.Name = name.String
	}
	snapshot.Status = spec.MetadataStatusConfirmed
	snapshot.Exists = true
	snapshot.Name = state.Name
	return snapshot, nil
}

// ErrSRSChannelVersionInconsistent marks the bounded consistency error
// raised when the provider channel's own version observation (O2)
// disagrees with the request-level observed identity (O1) — under a
// proxy or topology split the two reads can land on different
// backends, and a disagreement makes every catalog answer unsafe, so
// the read fails closed instead of silently adjudicating with O2
// (T26-SRS-VERSION-APPLICABILITY-01 EXECUTION-01 R3 frozen contract).
var ErrSRSChannelVersionInconsistent = errors.New("mysql srs channel version inconsistent with the request-observed identity")

// srsChannelQuerier is the query surface one pinned *sql.Conn offers —
// the O2 witness and every catalog read that follows it ride the SAME
// connection so a proxy or multi-backend topology cannot move the
// reads to a backend the witness never observed
// (T26-SRS-VERSION-APPLICABILITY-01 EXECUTION-01 R4).
type srsChannelQuerier interface {
	mysqlVersionQuerier
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// mysqlSRSChannelVersionWitness issues the channel's own
// select version() as a CONSISTENCY WITNESS — never a second
// adjudicator (frozen V1/R3). The request-level identity (O1) already
// decided applicability; O2 may only confirm the channel answers the
// same product+version, or fail closed:
//
//   - query/scan error → propagated wrapped on the bounded path;
//   - empty answer or unparseable/mismatched banner →
//     ErrSRSChannelVersionInconsistent;
//   - matching banner → nil.
//
// O2 never substitutes for O1 and never relaxes the request verdict;
// when O1 is absent the callers issue no query at all. The caller pins
// the connection — the witness only proves this channel's backend.
func mysqlSRSChannelVersionWitness(ctx context.Context, querier mysqlVersionQuerier, o1 *spec.VersionIdentity) error {
	var banner string
	if err := querier.QueryRowContext(ctx, `select version()`).Scan(&banner); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("%w: channel observation unavailable", ErrSRSChannelVersionInconsistent)
		}
		return fmt.Errorf("query server version for srs channels: %w", err)
	}
	o2 := spec.ParseObservedVersion(banner)
	if o2 == nil || o2.Product != o1.Product || o2.Version != o1.Version {
		return fmt.Errorf("%w: request=%s/%s channel=%q",
			ErrSRSChannelVersionInconsistent, o1.Product, o1.Version, banner)
	}
	return nil
}

// loadMySQLSRSObject resolves one MySQL spatial reference system
// identity through information_schema.ST_SPATIAL_REFERENCE_SYSTEMS —
// the bounded SRS_ID/SRS_NAME existence read keyed on the authored
// numeric SRID — plus the optional fact reads the request qualifies:
//
//   - NAME conflict: `where srs_name = ?` — only when the request
//     carries a provable name identity (srs_name qualifier). Never
//     fired from an undecoded spelling.
//   - ORGANIZATION + ORGANIZATION_COORDSYS_ID pair conflict: only when
//     both qualifiers carry provable values — a partially-known pair
//     issues no query.
//   - Column dependencies: `where srs_id = ?` over
//     ST_GEOMETRY_COLUMNS — a privilege-filtered view over COLUMNS, so
//     the visible set is witness evidence only; a clean empty answer is
//     unproven, never an instance-wide "no dependencies" claim.
//
// DEFINITION and DESCRIPTION are never selected — none of the five
// facts reads a payload column. A non-decimal or empty Name is an
// unprovable SRID identity and issues no query. Errors propagate
// wrapped — never converted into empty witness sets.
func (p *Provider) loadMySQLSRSObject(ctx context.Context, request spec.ObjectLookupRequest, snapshot *spec.ObjectSnapshot) (*spec.ObjectSnapshot, error) {
	state := &spec.SpatialReferenceSystemState{}
	snapshot.SpatialReference = state
	srid, err := strconv.ParseUint(strings.TrimSpace(request.Name), 10, 32)
	if err != nil {
		state.UnreadReason = spec.MetadataUnreadIdentityUnproven
		return snapshot, nil
	}
	state.SRID = uint32(srid)
	// Single-authority binding (frozen V1/R3): the request-level observed
	// identity (O1) decides applicability — a missing O1 means no
	// authoritative version exists, so the call closes unproven with
	// zero catalog reads and never falls back to an independent O2
	// adjudication. An off-domain O1 names the exact version reason.
	// Only an in-domain O1 reaches the witness check below, where O2
	// can merely confirm or fail closed.
	o1 := request.RequestVersion
	if o1 == nil {
		state.UnreadReason = spec.MetadataUnreadVersionUnproven
		return snapshot, nil
	}
	if reason := spec.MySQLSRSVersionGateReason(o1, true); reason != "" {
		state.UnreadReason = reason
		return snapshot, nil
	}
	// Pinned channel connection (R4): *sql.DB is a pool — without a
	// pinned *sql.Conn the O2 witness could land on one backend while
	// the catalog reads land on another, making the consistency check
	// vacuous under a proxy/topology split. The gate above already
	// closed, so a nil or off-domain O1 never reaches this line: no
	// connection is even acquired. The witness and every catalog read
	// below run on this one connection; the connection is released on
	// every path and a release failure on a clean read propagates.
	conn, err := p.db.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("pin mysql srs channel connection: %w", err)
	}
	snapshot, err = p.loadMySQLSRSObjectOnConn(ctx, conn, request, snapshot)
	if cerr := conn.Close(); err == nil && cerr != nil {
		return nil, fmt.Errorf("release mysql srs channel connection: %w", cerr)
	}
	return snapshot, err
}

// loadMySQLSRSObjectOnConn runs the pinned-connection read sequence —
// O2 witness, then existence, then the qualified conflict and
// dependency reads, all on the same conn.
func (p *Provider) loadMySQLSRSObjectOnConn(ctx context.Context, conn srsChannelQuerier, request spec.ObjectLookupRequest, snapshot *spec.ObjectSnapshot) (*spec.ObjectSnapshot, error) {
	state := snapshot.SpatialReference
	o1 := request.RequestVersion
	if err := mysqlSRSChannelVersionWitness(ctx, conn, o1); err != nil {
		return nil, err
	}
	srid := uint64(state.SRID)
	var matchedID sql.NullInt64
	var matchedName sql.NullString
	// The existence read runs on an explicit *sql.Rows so the delivered
	// state can distinguish the answer shapes a QueryRow Scan collapses
	// (T26-SRS-DROP-PROVEN-ABSENT-EXECUTION-01): Absence=true is
	// supplied ONLY when every one of the frozen supply conditions
	// holds — QueryContext succeeded, Next() never returned true for
	// this result (a Next()==false AFTER a positive row is only EOF),
	// rows.Err()==nil after iteration, and the checked Close()==nil —
	// AND the same Qo observation satisfies the registered D-proof
	// (validated MySQL 8.0.4+). Any query/scan/iteration/cancellation/
	// close error propagates on the bounded path; an error whose value
	// equals sql.ErrNoRows never supplies absence.
	rows, err := conn.QueryContext(ctx, `
		select srs_id, srs_name
		from information_schema.st_spatial_reference_systems
		where srs_id = ?
	`, int64(srid))
	if err != nil {
		return nil, fmt.Errorf("query mysql srs snapshot: %w", err)
	}
	observed := false
	for rows.Next() {
		observed = true
		if err := rows.Scan(&matchedID, &matchedName); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan mysql srs snapshot: %w", err)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate mysql srs snapshot: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close mysql srs snapshot: %w", err)
	}
	if observed {
		state.Read = true
		state.Exists = true
		state.Status = spec.MetadataStatusConfirmed
		if matchedName.Valid {
			state.CatalogName = matchedName.String
		}
		snapshot.Status = spec.MetadataStatusConfirmed
		snapshot.Exists = true
	} else {
		state.Read = true
		// Clean zero observed rows — a proven-absence fact only while
		// the request-level identity is inside the proven-absence domain
		// (O1 is the D-proof now — the witness only confirmed the
		// channel); every other zero-row answer stays unknown
		// (Absence=false).
		state.Absence = spec.MySQLSRSDropAuditedDomain(o1)
	}
	// The conflict reads fire only on provable request identities —
	// an authored spelling never reaches them.
	if name := request.Qualifiers[spec.ObjectLookupQualifierSRSName]; strings.TrimSpace(name) != "" {
		fact, err := p.loadMySQLSRSNameConflict(ctx, conn, name)
		if err != nil {
			return nil, err
		}
		state.NameConflict = fact
	}
	if org := request.Qualifiers[spec.ObjectLookupQualifierSRSOrg]; strings.TrimSpace(org) != "" {
		if idText := request.Qualifiers[spec.ObjectLookupQualifierSRSOrgID]; strings.TrimSpace(idText) != "" {
			if orgID, parseErr := strconv.ParseUint(idText, 10, 32); parseErr == nil {
				fact, err := p.loadMySQLSRSOrgConflict(ctx, conn, org, orgID)
				if err != nil {
					return nil, err
				}
				state.OrgConflict = fact
			}
		}
	}
	// The dependency read keys on the same provable SRID — it runs
	// whenever the version gate passed, independent of the existence
	// answer.
	deps, err := p.loadMySQLSRSDependencies(ctx, conn, srid)
	if err != nil {
		return nil, err
	}
	state.Dependencies = deps
	return snapshot, nil
}

// loadMySQLSRSNameConflict reads the SRIDs matching one provable SRS
// name — the NAME uniqueness witness set. The statement's own SRID and
// other SRIDs both appear as raw witnesses; distinguishing them is the
// consumer's premise. Errors propagate wrapped.
func (p *Provider) loadMySQLSRSNameConflict(ctx context.Context, conn srsChannelQuerier, name string) (*spec.SRSConflictFact, error) {
	return p.loadMySQLSRSConflict(ctx, conn, `
		select srs_id
		from information_schema.st_spatial_reference_systems
		where srs_name = ?
	`, name)
}

// loadMySQLSRSOrgConflict reads the SRIDs matching one provable
// ORGANIZATION + ORGANIZATION_COORDSYS_ID pair — the combined identity,
// never a member alone. Errors propagate wrapped.
func (p *Provider) loadMySQLSRSOrgConflict(ctx context.Context, conn srsChannelQuerier, org string, orgID uint64) (*spec.SRSConflictFact, error) {
	return p.loadMySQLSRSConflict(ctx, conn, `
		select srs_id
		from information_schema.st_spatial_reference_systems
		where organization = ? and organization_coordsys_id = ?
	`, org, int64(orgID))
}

// loadMySQLSRSConflict runs one SRID-witness select for a conflict
// predicate — the shared scan/iteration/error contract for the NAME and
// ORGANIZATION reads. Every query, scan, iteration, permission, driver,
// and cancellation error propagates wrapped — an issued read that
// failed is never laundered into a non-error return; a fact with empty
// MatchedSRIDs is "read, zero rows, unknown" — never a proven "no
// conflict".
func (p *Provider) loadMySQLSRSConflict(ctx context.Context, conn srsChannelQuerier, query string, args ...any) (*spec.SRSConflictFact, error) {
	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query mysql srs conflict read: %w", err)
	}
	fact := &spec.SRSConflictFact{}
	for rows.Next() {
		var id sql.NullInt64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan mysql srs conflict witness: %w", err)
		}
		if id.Valid && id.Int64 >= 0 && id.Int64 <= 4294967295 {
			fact.MatchedSRIDs = append(fact.MatchedSRIDs, uint32(id.Int64))
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate mysql srs conflict witnesses: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close mysql srs conflict read: %w", err)
	}
	return fact, nil
}

// ResolveMySQLSRSDependencies is the SRS ordered channel's deps-only
// read (T26-SRS-DROP-ORDERED-PUBLICATION-EXECUTION-01), bound to the
// request-level observed identity (R3 additive binding): a nil or
// off-domain version means no authoritative O1 exists — zero catalog
// reads and no fact rather than an independent provider adjudication.
// An in-domain O1 still runs the channel witness on ONE pinned
// *sql.Conn (R4) — the witness and the dependency read share the same
// backend; a disagreement or observation failure propagates the
// bounded consistency error with zero catalog reads. The
// ST_GEOMETRY_COLUMNS witness read for one authored SRID then runs on
// that same connection — the existence query is never issued on this
// path, and the call is deliberately uncached so every consuming
// statement sees a fresh witness. Query/iteration/close errors
// propagate wrapped.
func (p *Provider) ResolveMySQLSRSDependencies(ctx context.Context, srid uint32, version *spec.VersionIdentity) (*spec.SRSDependencyFact, error) {
	if version == nil || spec.MySQLSRSVersionGateReason(version, true) != "" {
		return nil, nil
	}
	// Pinned channel connection (R4): the O2 witness and the dependency
	// read run on one *sql.Conn — same backend, same read unit; the
	// connection is released on every path.
	conn, err := p.db.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("pin mysql srs channel connection: %w", err)
	}
	fact, err := func() (*spec.SRSDependencyFact, error) {
		if err := mysqlSRSChannelVersionWitness(ctx, conn, version); err != nil {
			return nil, err
		}
		return p.loadMySQLSRSDependencies(ctx, conn, uint64(srid))
	}()
	if cerr := conn.Close(); err == nil && cerr != nil {
		return nil, fmt.Errorf("release mysql srs channel connection: %w", cerr)
	}
	return fact, err
}

// loadMySQLSRSDependencies reads the ST_GEOMETRY_COLUMNS witness set for
// one SRID — the visible schema/table/column identities whose declared
// SRID is the target. The catalog view is privilege-filtered, so
// Complete is never claimed: positive rows prove presence while an
// empty set stays unproven — never instance-wide absence. Errors
// propagate wrapped.
func (p *Provider) loadMySQLSRSDependencies(ctx context.Context, conn srsChannelQuerier, srid uint64) (*spec.SRSDependencyFact, error) {
	rows, err := conn.QueryContext(ctx, `
		select table_schema, table_name, column_name
		from information_schema.st_geometry_columns
		where srs_id = ?
	`, int64(srid))
	if err != nil {
		return nil, fmt.Errorf("query mysql srs column dependencies: %w", err)
	}
	fact := &spec.SRSDependencyFact{}
	for rows.Next() {
		var schemaName, tableName, columnName sql.NullString
		if err := rows.Scan(&schemaName, &tableName, &columnName); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan mysql srs column dependency: %w", err)
		}
		fact.Columns = append(fact.Columns, spec.SRSDependentColumn{
			Schema: schemaName.String,
			Table:  tableName.String,
			Column: columnName.String,
			SRID:   uint32(srid),
		})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate mysql srs column dependencies: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close mysql srs column dependencies: %w", err)
	}
	return fact, nil
}
