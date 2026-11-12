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

// mysqlSRSCatalogSupport observes the server version once and
// adjudicates the SRS catalog reads against it —
// information_schema.ST_SPATIAL_REFERENCE_SYSTEMS and
// ST_GEOMETRY_COLUMNS.SRS_ID are 8.0 surfaces (the SRS feature itself is
// 8.0+; the 5.7 geometry-column surface carried a differently shaped
// SRID member):
//
//   - proven && !supported: the 8.0 catalog cannot hold this object —
//     the dedicated queries are never issued and every fact stays
//     unproven.
//   - proven && supported: the catalog objects must exist — every
//     query, scan, permission, driver, and cancellation error
//     propagates (missing-object codes included).
//   - !proven: the catalog may not exist — the reads are still issued
//     and every returned error propagates; an unproven version never
//     converts a query that already failed into "not loaded".
//
// The parsed provider version is also returned so the caller's
// completeness proof evaluates the SAME Qo observation — one
// select version() serves both the support gate and the proven-absence
// domain proof (T26-SRS-DROP-PROVEN-ABSENT-EXECUTION-01). It is nil
// whenever the banner could not be parsed or fails the product/range
// checks; request-supplied versions never substitute for it.
func mysqlSRSCatalogSupport(ctx context.Context, querier mysqlVersionQuerier) (supported bool, proven bool, version *spec.VersionIdentity, err error) {
	var banner string
	if err := querier.QueryRowContext(ctx, `select version()`).Scan(&banner); err != nil {
		if err == sql.ErrNoRows {
			return false, false, nil, nil
		}
		return false, false, nil, fmt.Errorf("query server version for srs channels: %w", err)
	}
	version = spec.ParseObservedVersion(banner)
	if version == nil || !version.ValidatedRange || version.Product != spec.VersionProductMySQL {
		return false, false, nil, nil
	}
	return version.Major >= 8, true, version, nil
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
	supported, proven, qoVersion, err := mysqlSRSCatalogSupport(ctx, p.db)
	if err != nil {
		return nil, err
	}
	if proven && !supported {
		state.UnreadReason = spec.MetadataUnreadVersionInapplicable
		return snapshot, nil
	}
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
	rows, err := p.db.QueryContext(ctx, `
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
		// the same Qo observation is inside the proven-absence domain;
		// every other zero-row answer stays unknown (Absence=false).
		state.Absence = spec.MySQLSRSDropAuditedDomain(qoVersion)
	}
	// The conflict reads fire only on provable request identities —
	// an authored spelling never reaches them.
	if name := request.Qualifiers[spec.ObjectLookupQualifierSRSName]; strings.TrimSpace(name) != "" {
		fact, err := p.loadMySQLSRSNameConflict(ctx, name)
		if err != nil {
			return nil, err
		}
		state.NameConflict = fact
	}
	if org := request.Qualifiers[spec.ObjectLookupQualifierSRSOrg]; strings.TrimSpace(org) != "" {
		if idText := request.Qualifiers[spec.ObjectLookupQualifierSRSOrgID]; strings.TrimSpace(idText) != "" {
			if orgID, parseErr := strconv.ParseUint(idText, 10, 32); parseErr == nil {
				fact, err := p.loadMySQLSRSOrgConflict(ctx, org, orgID)
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
	deps, err := p.loadMySQLSRSDependencies(ctx, srid)
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
func (p *Provider) loadMySQLSRSNameConflict(ctx context.Context, name string) (*spec.SRSConflictFact, error) {
	return p.loadMySQLSRSConflict(ctx, `
		select srs_id
		from information_schema.st_spatial_reference_systems
		where srs_name = ?
	`, name)
}

// loadMySQLSRSOrgConflict reads the SRIDs matching one provable
// ORGANIZATION + ORGANIZATION_COORDSYS_ID pair — the combined identity,
// never a member alone. Errors propagate wrapped.
func (p *Provider) loadMySQLSRSOrgConflict(ctx context.Context, org string, orgID uint64) (*spec.SRSConflictFact, error) {
	return p.loadMySQLSRSConflict(ctx, `
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
func (p *Provider) loadMySQLSRSConflict(ctx context.Context, query string, args ...any) (*spec.SRSConflictFact, error) {
	rows, err := p.db.QueryContext(ctx, query, args...)
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
// read (T26-SRS-DROP-ORDERED-PUBLICATION-EXECUTION-01): the Qo support
// gate plus the ST_GEOMETRY_COLUMNS witness read for one authored
// SRID — the existence query is never issued on this path, and the
// call is deliberately uncached so every consuming statement sees a
// fresh witness. A proven-unsupported target short-circuits before
// the catalog query and answers nil fact/nil error; an unparseable Qo
// keeps the accepted continue direction (the deps read still issues);
// query/iteration/close errors propagate wrapped.
func (p *Provider) ResolveMySQLSRSDependencies(ctx context.Context, srid uint32) (*spec.SRSDependencyFact, error) {
	supported, proven, _, err := mysqlSRSCatalogSupport(ctx, p.db)
	if err != nil {
		return nil, err
	}
	if proven && !supported {
		return nil, nil
	}
	return p.loadMySQLSRSDependencies(ctx, uint64(srid))
}

// loadMySQLSRSDependencies reads the ST_GEOMETRY_COLUMNS witness set for
// one SRID — the visible schema/table/column identities whose declared
// SRID is the target. The catalog view is privilege-filtered, so
// Complete is never claimed: positive rows prove presence while an
// empty set stays unproven — never instance-wide absence. Errors
// propagate wrapped.
func (p *Provider) loadMySQLSRSDependencies(ctx context.Context, srid uint64) (*spec.SRSDependencyFact, error) {
	rows, err := p.db.QueryContext(ctx, `
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
