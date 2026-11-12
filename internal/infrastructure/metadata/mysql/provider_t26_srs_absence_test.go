package mysqlmeta

// Package mysqlmeta: T26-SRS-DROP-PROVEN-ABSENT-EXECUTION-01 provider
// rows — the explicit-Rows existence read's supply-predicate
// discriminant and bounded-error table over the REAL product loader
// path (docs/verification/t26-srs-drop-proven-absent-contract-01
// case-matrix PA-4g/PA-4h/PA-4i and PA-9a/PA-9b/PA-9c/PA-9e/PA-9f,
// plus the in-domain zero-row supply row the unknown-branch twin at
// provider_t26_test.go pairs with). The stub driver records every
// issued query and injects query/iteration/close/cancellation sentinels
// at the named seams; no fake resolver is substituted.

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func t26SRSRequest(srid string) spec.ObjectLookupRequest {
	return spec.ObjectLookupRequest{Type: "mysql_spatial_reference_system", Name: srid}
}

func t26SRSAbsenceFixture(version string, exist testQueryResult) map[string]testQueryResult {
	return map[string]testQueryResult{
		testSRSVersionFragment:  t26VersionRow(version),
		testSRSIdentityFragment: exist,
		testSRSDepFragment:      {columns: []string{"TABLE_SCHEMA", "TABLE_NAME", "COLUMN_NAME"}},
	}
}

func t26ResolveCtx(ctx context.Context, t *testing.T, results map[string]testQueryResult, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	t.Helper()
	db := openTestDB(t, results)
	provider := NewProvider(db)
	return provider.ResolveObject(ctx, spec.DialectMySQL, req)
}

// PA-1 provider half: a clean zero-observed-row answer under an
// in-domain Qo observation supplies Absence=true — Read=true,
// Exists=false, Absence=true, and the dependency read still fires.
func TestT26SRSAbsenceSuppliedInDomain(t *testing.T) {
	snapshot, err := t26Resolve(t, t26SRSAbsenceFixture("8.0.36",
		testQueryResult{columns: []string{"SRS_ID", "SRS_NAME"}}),
		spec.DialectMySQL, t26SRSRequest("100001"))
	if err != nil {
		t.Fatalf("ResolveObject: %v", err)
	}
	state := snapshot.SpatialReference
	if state == nil || !state.Read || state.Exists || !state.Absence {
		t.Fatalf("srs state = %+v, want read + absent (proven absence)", state)
	}
	if state.SRID != 100001 {
		t.Fatalf("state SRID=%d want the authored 100001", state.SRID)
	}
	if calls := t26Calls(t, testSRSDepFragment); len(calls) != 1 {
		t.Fatalf("dependency read issued %d times, want exactly once", len(calls))
	}
}

// t26Attempt asserts the per-seam attempt counts the stub recorded —
// Qo (select version()), Qexist (st_spatial_reference_systems), and
// Qdeps (st_geometry_columns) — including attempts whose query later
// failed. Zero means the read was never reached (R2-A3).
func t26Attempt(t *testing.T, fragment string) int {
	t.Helper()
	return len(t26Calls(t, fragment))
}

// R2-A1: the domain lower bound — Qo=8.0.4 is inside
// MySQLSRSDropAuditedDomain, so a clean zero-observed-row answer
// supplies Absence=true; Qo/Qexist/Qdeps each issued exactly once.
func TestT26SRSAbsenceSuppliedInDomain804(t *testing.T) {
	snapshot, err := t26Resolve(t, t26SRSAbsenceFixture("8.0.4",
		testQueryResult{columns: []string{"SRS_ID", "SRS_NAME"}}),
		spec.DialectMySQL, t26SRSRequest("100001"))
	if err != nil {
		t.Fatalf("ResolveObject: %v", err)
	}
	state := snapshot.SpatialReference
	if state == nil || !state.Read || state.Exists || !state.Absence {
		t.Fatalf("srs state = %+v, want read + absent under Qo=8.0.4", state)
	}
	if got := t26Attempt(t, testSRSVersionFragment); got != 1 {
		t.Fatalf("Qo attempts=%d want 1", got)
	}
	if got := t26Attempt(t, testSRSIdentityFragment); got != 1 {
		t.Fatalf("Qexist attempts=%d want 1", got)
	}
	if got := t26Attempt(t, testSRSDepFragment); got != 1 {
		t.Fatalf("Qdeps attempts=%d want 1", got)
	}
}

// R2-A1: a parseable, supported MySQL banner OUTSIDE the 8.0.x proven-
// absence domain — Qo=8.4.3 satisfies D-support (Major>=8) so the
// reads still issue, but the D-proof (8.0.4+) fails, so a clean
// zero-observed-row answer stays unknown (Absence=false). This row
// separates the support gate from the completeness domain.
func TestT26SRSAbsenceNotSuppliedSupportedNon80(t *testing.T) {
	snapshot, err := t26Resolve(t, t26SRSAbsenceFixture("8.4.3",
		testQueryResult{columns: []string{"SRS_ID", "SRS_NAME"}}),
		spec.DialectMySQL, t26SRSRequest("100001"))
	if err != nil {
		t.Fatalf("ResolveObject: %v", err)
	}
	state := snapshot.SpatialReference
	if state == nil || !state.Read || state.Exists || state.Absence {
		t.Fatalf("srs state = %+v, want read + unknown — D-support held, D-proof did not", state)
	}
	if got := t26Attempt(t, testSRSVersionFragment); got != 1 {
		t.Fatalf("Qo attempts=%d want 1", got)
	}
	if got := t26Attempt(t, testSRSIdentityFragment); got != 1 {
		t.Fatalf("Qexist attempts=%d want 1 — supported versions still read", got)
	}
	if got := t26Attempt(t, testSRSDepFragment); got != 1 {
		t.Fatalf("Qdeps attempts=%d want 1", got)
	}
}

// PA-9e: a clean zero-observed-row answer under an unparseable Qo
// banner — the D-proof never holds, so Absence stays false and the
// answer remains unknown (the S2 outcome is the api/enrich twin).
func TestT26SRSAbsenceNotSuppliedUnprovenVersion(t *testing.T) {
	snapshot, err := t26Resolve(t, t26SRSAbsenceFixture("unparseable",
		testQueryResult{columns: []string{"SRS_ID", "SRS_NAME"}}),
		spec.DialectMySQL, t26SRSRequest("100001"))
	if err != nil {
		t.Fatalf("ResolveObject: %v", err)
	}
	state := snapshot.SpatialReference
	if state == nil || !state.Read || state.Exists || state.Absence {
		t.Fatalf("srs state = %+v, want {Read:true,Exists:false,Absence:false}", state)
	}
}

// PA-4i: a positive row followed by Next()=false/Err()=nil is EOF
// after presence — never a zero-row answer. Exists=true, Absence
// stays false.
func TestT26SRSAbsenceEOFFterPositiveRow(t *testing.T) {
	snapshot, err := t26Resolve(t, t26SRSAbsenceFixture("8.0.36",
		testQueryResult{columns: []string{"SRS_ID", "SRS_NAME"}, rows: [][]driver.Value{{int64(100001), "ZZTEST"}}}),
		spec.DialectMySQL, t26SRSRequest("100001"))
	if err != nil {
		t.Fatalf("ResolveObject: %v", err)
	}
	state := snapshot.SpatialReference
	if state == nil || !state.Read || !state.Exists || state.Absence {
		t.Fatalf("srs state = %+v, want present — EOF after a positive row is not absence", state)
	}
}

// t26AssertSeams asserts the Qo/Qexist/Qdeps attempt counts — the
// termination position of a failed or bounded read (R2-A3). Provider
// rows carry no request-discovery or privilege-loader dimension —
// those layers are N/A at this seam.
func t26AssertSeams(t *testing.T, wantQo, wantExist, wantDeps int) {
	t.Helper()
	if got := t26Attempt(t, testSRSVersionFragment); got != wantQo {
		t.Fatalf("Qo attempts=%d want %d", got, wantQo)
	}
	if got := t26Attempt(t, testSRSIdentityFragment); got != wantExist {
		t.Fatalf("Qexist attempts=%d want %d", got, wantExist)
	}
	if got := t26Attempt(t, testSRSDepFragment); got != wantDeps {
		t.Fatalf("Qdeps attempts=%d want %d", got, wantDeps)
	}
}

// PA-4g: a QueryContext error whose VALUE equals sql.ErrNoRows is an
// error, never a clean zero-row answer — it propagates and no snapshot
// is delivered. The existence read was attempted once; the dependency
// read is never reached.
func TestT26SRSAbsenceQueryErrNoRowsPropagates(t *testing.T) {
	snapshot, err := t26Resolve(t, t26SRSAbsenceFixture("8.0.36",
		testQueryResult{columns: []string{"SRS_ID", "SRS_NAME"}, queryErr: sql.ErrNoRows}),
		spec.DialectMySQL, t26SRSRequest("100001"))
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("err = %v, want errors.Is(sql.ErrNoRows) — an ErrNoRows-valued failure never supplies absence", err)
	}
	if snapshot != nil {
		t.Fatalf("snapshot = %+v, want none delivered on a failed read", snapshot)
	}
	t26AssertSeams(t, 1, 1, 0)
}

// PA-4h: rows.Err() == sql.ErrNoRows after Next()=false with zero
// observed rows propagates — the sentinel-valued iteration error is
// never Absence.
func TestT26SRSAbsenceIterationErrNoRowsPropagates(t *testing.T) {
	snapshot, err := t26Resolve(t, t26SRSAbsenceFixture("8.0.36",
		testQueryResult{columns: []string{"SRS_ID", "SRS_NAME"}, rowErrAt: 0, rowErr: sql.ErrNoRows}),
		spec.DialectMySQL, t26SRSRequest("100001"))
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("err = %v, want errors.Is(sql.ErrNoRows) — iteration sentinel never supplies absence", err)
	}
	if snapshot != nil {
		t.Fatalf("snapshot = %+v, want none delivered on a failed read", snapshot)
	}
	t26AssertSeams(t, 1, 1, 0)
}

// PA-9a: a generic existence query failure propagates wrapped.
func TestT26SRSAbsenceQueryErrorPropagates(t *testing.T) {
	want := errors.New("srs existence read denied")
	snapshot, err := t26Resolve(t, t26SRSAbsenceFixture("8.0.36",
		testQueryResult{columns: []string{"SRS_ID", "SRS_NAME"}, queryErr: want}),
		spec.DialectMySQL, t26SRSRequest("100001"))
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want errors.Is(%v)", err, want)
	}
	if snapshot != nil {
		t.Fatalf("snapshot = %+v, want none delivered", snapshot)
	}
	t26AssertSeams(t, 1, 1, 0)
}

// PA-9b: an iteration error during Next() propagates through
// rows.Err() — no snapshot is delivered.
func TestT26SRSAbsenceIterationErrorPropagates(t *testing.T) {
	want := errors.New("srs existence rowset lost")
	snapshot, err := t26Resolve(t, t26SRSAbsenceFixture("8.0.36",
		testQueryResult{columns: []string{"SRS_ID", "SRS_NAME"}, rowErrAt: 0, rowErr: want}),
		spec.DialectMySQL, t26SRSRequest("100001"))
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want errors.Is(%v)", err, want)
	}
	if snapshot != nil {
		t.Fatalf("snapshot = %+v, want none delivered", snapshot)
	}
	t26AssertSeams(t, 1, 1, 0)
}

// A checked rows.Close() failure on the existence read propagates —
// the answer is never delivered and never becomes absence. In
// database/sql the close error can surface either through the
// iteration path's EOF auto-close (Rows.Err()) or the explicit
// rows.Close() call — both carry the sentinel through the bounded
// path; this row only requires the error reached the caller.
func TestT26SRSAbsenceCloseErrorPropagates(t *testing.T) {
	want := errors.New("srs existence close failed")
	snapshot, err := t26Resolve(t, t26SRSAbsenceFixture("8.0.36",
		testQueryResult{columns: []string{"SRS_ID", "SRS_NAME"}, closeErr: want}),
		spec.DialectMySQL, t26SRSRequest("100001"))
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want errors.Is(%v)", err, want)
	}
	if snapshot != nil {
		t.Fatalf("snapshot = %+v, want none delivered", snapshot)
	}
	t26AssertSeams(t, 1, 1, 0)
}

// A pre-cancelled request never reaches the catalog — this is the
// request-control row, NOT the PA-9c mid-read cancellation witness
// (TestT26SRSAbsenceMidIterationCancelPropagates owns that).
func TestT26SRSAbsencePreCancelledRequestPropagates(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	snapshot, err := t26ResolveCtx(ctx, t, t26SRSAbsenceFixture("8.0.36",
		testQueryResult{columns: []string{"SRS_ID", "SRS_NAME"}}), t26SRSRequest("100001"))
	if err == nil || !strings.Contains(err.Error(), "context") {
		t.Fatalf("err = %v, want the cancellation propagated", err)
	}
	if snapshot != nil {
		t.Fatalf("snapshot = %+v, want none delivered", snapshot)
	}
}

// PA-9c: a REAL mid-read cancellation — Qo succeeds, the existence
// query is attempted, and the request context is cancelled while the
// existence rowset is mid-iteration. database/sql surfaces
// context.Canceled through rows.Err(); the bounded path propagates it,
// no snapshot is delivered, and the dependency read is never reached.
//
// R3-A: the cancel is synced to the rowset's Next() ENTRY, not merely
// to the recorded query attempt — QueryContext records its attempt
// before the rowset exists, so attempt-polling alone cannot prove the
// cancel landed inside iteration. The driver.Rows.Next handshake
// (enteredNext) closes the first time Next() is actually entered; the
// cancel fires only after that handshake.
func TestT26SRSAbsenceMidIterationCancelPropagates(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	entered := make(chan struct{})
	db := openTestDB(t, map[string]testQueryResult{
		testSRSVersionFragment:  t26VersionRow("8.0.36"),
		testSRSIdentityFragment: {columns: []string{"SRS_ID", "SRS_NAME"}, blockOnCtx: true, enteredNext: entered},
		testSRSDepFragment:      {columns: []string{"TABLE_SCHEMA", "TABLE_NAME", "COLUMN_NAME"}},
	})
	provider := NewProvider(db)
	type outcome struct {
		snapshot *spec.ObjectSnapshot
		err      error
	}
	done := make(chan outcome, 1)
	go func() {
		snapshot, err := provider.ResolveObject(ctx, spec.DialectMySQL, t26SRSRequest("100001"))
		done <- outcome{snapshot, err}
	}()
	// Bounded wait for the actual Next() entry handshake — this is the
	// proof iteration had begun, not merely that the query was issued.
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("rowset Next() was never entered — no mid-iteration point to cancel at")
	}
	// Qo succeeded before the cancel; the existence query was attempted
	// and its rowset is mid-iteration.
	if got := t26Attempt(t, testSRSVersionFragment); got != 1 {
		t.Fatalf("Qo attempts=%d want 1 — Qo succeeded before the cancel", got)
	}
	if got := t26Attempt(t, testSRSIdentityFragment); got != 1 {
		t.Fatalf("Qexist attempts=%d want 1", got)
	}
	cancel()
	got := <-done
	if !errors.Is(got.err, context.Canceled) {
		t.Fatalf("err = %v, want errors.Is(context.Canceled)", got.err)
	}
	if got.snapshot != nil {
		t.Fatalf("snapshot = %+v, want none delivered", got.snapshot)
	}
	// Final-vector assertion AFTER the call returns — the pre-cancel
	// counts above prove where the cancel landed; this excludes any
	// further Qo/existence attempt made after the cancel (R4-A).
	t26AssertSeams(t, 1, 1, 0)
}

// PA-9f: the existence read's clean zero-observed-row answer does not
// publish a partial snapshot — a dependency read failure propagates
// and nothing reaches the caller.
func TestT26SRSAbsenceDepsErrorPropagates(t *testing.T) {
	want := errors.New("geometry columns denied")
	snapshot, err := t26Resolve(t, map[string]testQueryResult{
		testSRSVersionFragment:  t26VersionRow("8.0.36"),
		testSRSIdentityFragment: {columns: []string{"SRS_ID", "SRS_NAME"}},
		testSRSDepFragment:      {columns: []string{"TABLE_SCHEMA", "TABLE_NAME", "COLUMN_NAME"}, queryErr: want},
	}, spec.DialectMySQL, t26SRSRequest("100001"))
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want errors.Is(%v)", err, want)
	}
	if snapshot != nil {
		t.Fatalf("snapshot = %+v, want none delivered — a helper's local field is not a published fact", snapshot)
	}
	t26AssertSeams(t, 1, 1, 1)
}
