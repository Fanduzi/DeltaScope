//go:build postgresql

// Package postgresqlmeta provides a session-pinned PostgreSQL handle for effect identity.
// input: a single *sql.Conn (never a multi-connection *sql.DB pool)
// output: live EffectIdentityResolutionContext and catalog query surface
// pos: T7 infrastructure; facts only; no trust / admission
// note: if this file changes, update this header and module README.md.
package postgresqlmeta

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	appqa "github.com/Fanduzi/DeltaScope/internal/application/queryaccess"
)

var (
	// ErrSessionNotPinned indicates the handle is not a single controlled session.
	ErrSessionNotPinned = errors.New("effect identity requires a session-pinned connection")
	// ErrSessionClosed indicates the pinned session is unavailable.
	ErrSessionClosed = errors.New("effect identity pinned session is closed")
)

// PinnedSession is a single-backend PostgreSQL session for effect-identity lookup.
//
// It intentionally wraps *sql.Conn, not *sql.DB: pool-style multi-connection
// reuse cannot prove search_path / role / database locality for OID facts.
// Callers must not share one PinnedSession across concurrent ResolveEffectIdentities.
// No resolved-identity cache is retained across calls (only the pinned conn).
type PinnedSession struct {
	conn *sql.Conn
}

// NewPinnedSessionFromConn pins an existing *sql.Conn. conn must be non-nil.
func NewPinnedSessionFromConn(conn *sql.Conn) (*PinnedSession, error) {
	if conn == nil {
		return nil, ErrSessionNotPinned
	}
	return &PinnedSession{conn: conn}, nil
}

// PinSession obtains one connection from db via db.Conn and pins it.
// The returned session owns the conn; Close releases it to the pool.
func PinSession(ctx context.Context, db *sql.DB) (*PinnedSession, error) {
	if db == nil {
		return nil, ErrSessionNotPinned
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: acquire conn", ErrSessionNotPinned)
	}
	return &PinnedSession{conn: conn}, nil
}

// Close releases the pinned connection. Safe to call multiple times.
func (s *PinnedSession) Close() error {
	if s == nil || s.conn == nil {
		return nil
	}
	err := s.conn.Close()
	s.conn = nil
	return err
}

// Conn returns the underlying *sql.Conn for advanced callers. Prefer adapter APIs.
func (s *PinnedSession) Conn() *sql.Conn {
	if s == nil {
		return nil
	}
	return s.conn
}

func (s *PinnedSession) requireConn() (*sql.Conn, error) {
	if s == nil || s.conn == nil {
		return nil, ErrSessionClosed
	}
	return s.conn, nil
}

// CaptureLiveContext reads database OID, role OID, server_version_num, backend
// binding, and expanded ordered search_path namespace OIDs from this session.
// PathEpoch is a stable non-zero constant for a capture pair within one resolve
// (session/db/role/version identity is compared field-wise; search_path is separate).
// Never returns DSN, password, or free-text SQL in the context fields.
func (s *PinnedSession) CaptureLiveContext(ctx context.Context) (appqa.EffectIdentityResolutionContext, error) {
	conn, err := s.requireConn()
	if err != nil {
		return appqa.EffectIdentityResolutionContext{}, err
	}
	if err := ctx.Err(); err != nil {
		return appqa.EffectIdentityResolutionContext{}, err
	}

	var (
		databaseOID      uint32
		roleOID          uint32
		serverVersionNum int
		backendPID       int64
	)

	// Single round-trip for scalar session identity facts.
	err = conn.QueryRowContext(ctx, `
		select
			(select d.oid from pg_catalog.pg_database d where d.datname = current_database()),
			(select r.oid from pg_catalog.pg_roles r where r.rolname = current_user),
			current_setting('server_version_num')::int,
			pg_catalog.pg_backend_pid()
	`).Scan(&databaseOID, &roleOID, &serverVersionNum, &backendPID)
	if err != nil {
		// Scrub: do not wrap driver text into public surfaces; callers map to lookup_failed.
		return appqa.EffectIdentityResolutionContext{}, errSessionCapture
	}
	if databaseOID == 0 || roleOID == 0 || serverVersionNum == 0 || backendPID == 0 {
		return appqa.EffectIdentityResolutionContext{}, errSessionCapture
	}

	pathOIDs, err := s.captureSearchPathOIDs(ctx, conn)
	if err != nil {
		return appqa.EffectIdentityResolutionContext{}, errSessionCapture
	}

	// Opaque binding: backend + database (never host/user/password).
	binding := "b" + strconv.FormatInt(backendPID, 10) + "-d" + strconv.FormatUint(uint64(databaseOID), 10)

	return appqa.EffectIdentityResolutionContext{
		Bound:               true,
		SessionBinding:      binding,
		PathEpoch:           1, // field-wise session compare; path compared separately
		NamespaceSearchOIDs: pathOIDs,
		DatabaseOID:         databaseOID,
		RoleOID:             roleOID,
		ServerVersionNum:    serverVersionNum,
	}, nil
}

var errSessionCapture = errors.New("session context capture failed")

func (s *PinnedSession) captureSearchPathOIDs(ctx context.Context, conn *sql.Conn) ([]uint32, error) {
	rows, err := conn.QueryContext(ctx, `
		select n.oid
		from unnest(pg_catalog.current_schemas(true)) with ordinality as s(nspname, ord)
		join pg_catalog.pg_namespace n on n.nspname = s.nspname
		order by s.ord
	`)
	if err != nil {
		return nil, errSessionCapture
	}
	defer rows.Close()

	var out []uint32
	for rows.Next() {
		var oid uint32
		if err := rows.Scan(&oid); err != nil {
			return nil, errSessionCapture
		}
		if oid != 0 {
			out = append(out, oid)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, errSessionCapture
	}
	return out, nil
}

// namespaceOIDByName resolves a schema name to OID (exact nspname match).
func (s *PinnedSession) namespaceOIDByName(ctx context.Context, name string) (uint32, error) {
	conn, err := s.requireConn()
	if err != nil {
		return 0, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, sql.ErrNoRows
	}
	var oid uint32
	err = conn.QueryRowContext(ctx, `
		select n.oid from pg_catalog.pg_namespace n where n.nspname = $1
	`, name).Scan(&oid)
	return oid, err
}

// typeOIDByName resolves schema-qualified or path-relative type names.
// When schema is empty, walks namespaceSearchOIDs in order (exact typname match).
func (s *PinnedSession) typeOIDByName(ctx context.Context, schema, typname string, path []uint32) (uint32, error) {
	conn, err := s.requireConn()
	if err != nil {
		return 0, err
	}
	typname = strings.TrimSpace(typname)
	if typname == "" {
		return 0, sql.ErrNoRows
	}
	if schema != "" {
		var oid uint32
		err = conn.QueryRowContext(ctx, `
			select t.oid
			from pg_catalog.pg_type t
			join pg_catalog.pg_namespace n on n.oid = t.typnamespace
			where n.nspname = $1 and t.typname = $2
		`, schema, typname).Scan(&oid)
		return oid, err
	}
	// Unqualified type: first exact match along path.
	var found []uint32
	for _, ns := range path {
		var oid uint32
		err = conn.QueryRowContext(ctx, `
			select t.oid from pg_catalog.pg_type t
			where t.typnamespace = $1 and t.typname = $2
		`, ns, typname).Scan(&oid)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return 0, err
		}
		found = append(found, oid)
		break // first path hit only for type name
	}
	if len(found) == 0 {
		return 0, sql.ErrNoRows
	}
	return found[0], nil
}

type operatorRow struct {
	OID               uint32
	NamespaceOID      uint32
	ImplementationOID uint32
	ResultTypeOID     uint32
	Volatility        string
	SchemaName        string
	OperatorName      string
}

func (s *PinnedSession) lookupOperators(ctx context.Context, nsOID uint32, name string, left, right uint32) ([]operatorRow, error) {
	conn, err := s.requireConn()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		select o.oid, o.oprnamespace, o.oprcode::pg_catalog.oid, o.oprresult, coalesce(p.provolatile, ''),
		       n.nspname, o.oprname
		from pg_catalog.pg_operator o
		join pg_catalog.pg_namespace n on n.oid = o.oprnamespace
		left join pg_catalog.pg_proc p on p.oid = o.oprcode
		where o.oprnamespace = $1
		  and o.oprname = $2
		  and o.oprleft = $3
		  and o.oprright = $4
	`, nsOID, name, left, right)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []operatorRow
	for rows.Next() {
		var r operatorRow
		if err := rows.Scan(&r.OID, &r.NamespaceOID, &r.ImplementationOID, &r.ResultTypeOID, &r.Volatility, &r.SchemaName, &r.OperatorName); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type functionRow struct {
	OID          uint32
	NamespaceOID uint32
	ResultType   uint32
	Volatility   string
	SchemaName   string
	FuncName     string
	ArgTypeOIDs  []uint32
}

func (s *PinnedSession) lookupFunctions(ctx context.Context, nsOID uint32, name string, argOIDs []uint32) ([]functionRow, error) {
	conn, err := s.requireConn()
	if err != nil {
		return nil, err
	}
	// Exact proargtypes match via oidvector text form ("23 23" or "").
	argVec := oidVectorLiteral(argOIDs)
	rows, err := conn.QueryContext(ctx, `
		select p.oid, p.pronamespace, p.prorettype, p.provolatile, n.nspname, p.proname, p.proargtypes::text
		from pg_catalog.pg_proc p
		join pg_catalog.pg_namespace n on n.oid = p.pronamespace
		where p.pronamespace = $1
		  and p.proname = $2
		  and p.proargtypes = $3::pg_catalog.oidvector
	`, nsOID, name, argVec)
	if err != nil {
		return nil, err
	}
	out, err := scanFunctionRows(rows)
	if err != nil {
		return nil, err
	}
	if len(out) > 0 {
		return out, nil
	}
	// Fallback 1: single-arg anyelement (OID 2276) overload.
	if len(argOIDs) == 1 {
		rows, err = conn.QueryContext(ctx, `
			select p.oid, p.pronamespace, p.prorettype, p.provolatile, n.nspname, p.proname, p.proargtypes::text
			from pg_catalog.pg_proc p
			join pg_catalog.pg_namespace n on n.oid = p.pronamespace
			where p.pronamespace = $1
			  and p.proname = $2
			  and p.proargtypes = '2276'::pg_catalog.oidvector
		`, nsOID, name)
		if err != nil {
			return nil, err
		}
		out, err = scanFunctionRows(rows)
		if err != nil {
			return nil, err
		}
		if len(out) > 0 {
			return out, nil
		}
	}
	// Fallback 2: purely variadic functions (e.g. COALESCE).
	// Match functions where proargtypes is empty and provariadic > 0.
	// Return the variadic element type OID as the argtypes so the resolved
	// facts carry the polymorphic type, matching the manifest entry.
	if len(argOIDs) > 0 {
		rows, err = conn.QueryContext(ctx, `
			select p.oid, p.pronamespace, p.prorettype, p.provolatile, n.nspname, p.proname, p.provariadic::text
			from pg_catalog.pg_proc p
			join pg_catalog.pg_namespace n on n.oid = p.pronamespace
			where p.pronamespace = $1
			  and p.proname = $2
			  and p.proargtypes = ''::pg_catalog.oidvector
			  and p.provariadic > 0
			  and p.pronargs = 0
		`, nsOID, name)
		if err != nil {
			return nil, err
		}
		out, err = scanFunctionRows(rows)
		if err != nil {
			return nil, err
		}
		if len(out) > 0 {
			return out, nil
		}
	}
	// Fallback 3: polymorphic-arg functions (e.g. NULLIF(any, any)).
	// Match functions whose proargtypes contain only polymorphic type OIDs
	// (any=2276, anyelement=2283, anycompatible=5077, etc.) and arity matches.
	if len(argOIDs) > 0 {
		rows, err = conn.QueryContext(ctx, `
			select p.oid, p.pronamespace, p.prorettype, p.provolatile, n.nspname, p.proname, p.proargtypes::text
			from pg_catalog.pg_proc p
			join pg_catalog.pg_namespace n on n.oid = p.pronamespace
			where p.pronamespace = $1
			  and p.proname = $2
			  and p.pronargs = $3
			  and p.proargtypes::text ~ '^(2276|2283|2776|3500|3831|5077|5078|5079|5080)( (2276|2283|2776|3500|3831|5077|5078|5079|5080))*$'
		`, nsOID, name, len(argOIDs))
		if err != nil {
			return nil, err
		}
		return scanFunctionRows(rows)
	}
	return out, nil
}

type castRow struct {
	OID          uint32
	SourceOID    uint32
	TargetOID    uint32
	CastFuncOID  uint32
	CastMethod   string
	SourceSchema string
	SourceName   string
	TargetSchema string
	TargetName   string
}

func (s *PinnedSession) lookupCasts(ctx context.Context, sourceOID, targetOID uint32) ([]castRow, error) {
	conn, err := s.requireConn()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		select c.oid, c.castsource, c.casttarget, coalesce(c.castfunc, 0), c.castmethod::text,
		       ns.nspname, ts.typname, nt.nspname, tt.typname
		from pg_catalog.pg_cast c
		join pg_catalog.pg_type ts on ts.oid = c.castsource
		join pg_catalog.pg_namespace ns on ns.oid = ts.typnamespace
		join pg_catalog.pg_type tt on tt.oid = c.casttarget
		join pg_catalog.pg_namespace nt on nt.oid = tt.typnamespace
		where c.castsource = $1 and c.casttarget = $2
	`, sourceOID, targetOID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []castRow
	for rows.Next() {
		var r castRow
		if err := rows.Scan(&r.OID, &r.SourceOID, &r.TargetOID, &r.CastFuncOID, &r.CastMethod,
			&r.SourceSchema, &r.SourceName, &r.TargetSchema, &r.TargetName); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// beginRepeatableReadTx starts a REPEATABLE READ READ ONLY transaction
// on the pinned connection. All catalog queries in this transaction see
// the same snapshot, preventing concurrent DDL from causing inconsistency.
func (s *PinnedSession) beginRepeatableReadTx(ctx context.Context) (*sql.Tx, error) {
	conn, err := s.requireConn()
	if err != nil {
		return nil, err
	}
	return conn.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelRepeatableRead,
		ReadOnly:  true,
	})
}

func (s *PinnedSession) columnTypeOID(ctx context.Context, schema, table, column string) (uint32, error) {
	conn, err := s.requireConn()
	if err != nil {
		return 0, err
	}
	var oid uint32
	err = conn.QueryRowContext(ctx, `
		SELECT a.atttypid
		FROM pg_catalog.pg_attribute a
		JOIN pg_catalog.pg_class c ON c.oid = a.attrelid
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = $1 AND c.relname = $2 AND a.attname = $3
		  AND a.attnum > 0 AND NOT a.attisdropped
	`, schema, table, column).Scan(&oid)
	if err != nil {
		return 0, err
	}
	return oid, nil
}

func (s *PinnedSession) resolveColumnTypeOIDBySearchPath(ctx context.Context, table, column string, searchPathOIDs []uint32) (uint32, error) {
	conn, err := s.requireConn()
	if err != nil {
		return 0, err
	}
	for _, nsOID := range searchPathOIDs {
		var oid uint32
		err = conn.QueryRowContext(ctx, `
			SELECT a.atttypid
			FROM pg_catalog.pg_attribute a
			JOIN pg_catalog.pg_class c ON c.oid = a.attrelid
			WHERE c.relnamespace = $1 AND c.relname = $2 AND a.attname = $3
			  AND a.attnum > 0 AND NOT a.attisdropped
		`, nsOID, table, column).Scan(&oid)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return 0, err
		}
		if oid != 0 {
			return oid, nil
		}
	}
	return 0, sql.ErrNoRows
}

func oidVectorLiteral(oids []uint32) string {
	if len(oids) == 0 {
		return ""
	}
	parts := make([]string, len(oids))
	for i, o := range oids {
		parts[i] = strconv.FormatUint(uint64(o), 10)
	}
	return strings.Join(parts, " ")
}

func parseOIDVectorText(s string) []uint32 {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	fields := strings.Fields(s)
	out := make([]uint32, 0, len(fields))
	for _, f := range fields {
		v, err := strconv.ParseUint(f, 10, 32)
		if err != nil {
			continue
		}
		out = append(out, uint32(v))
	}
	return out
}
