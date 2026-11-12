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

// PA-4g: a QueryContext error whose VALUE equals sql.ErrNoRows is an
// error, never a clean zero-row answer — it propagates and no snapshot
// is delivered.
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
}

// A checked rows.Close() failure on the existence read propagates —
// the answer is never delivered and never becomes absence.
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
}

// PA-9c: a cancelled context surfaces through the bounded error path —
// no snapshot, no partial state.
func TestT26SRSAbsenceCancellationPropagates(t *testing.T) {
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
}
