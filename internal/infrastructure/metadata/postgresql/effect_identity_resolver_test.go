//go:build postgresql

// Package postgresqlmeta tests the T7 facts-only effect identity adapter.
// input: fake session-pinned catalog + synthetic candidates
// output: bounded statuses, OID stamps, fail-closed gates; no public leak
// pos: unit coverage without claiming PG17 integration
// note: if this file changes, update this header and module README.md.
package postgresqlmeta

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"strings"
	"testing"

	appqa "github.com/Fanduzi/DeltaScope/internal/application/queryaccess"
	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

func TestNewEffectIdentityAdapter_RejectsUnpinned(t *testing.T) {
	t.Parallel()
	if _, err := NewEffectIdentityAdapter(nil); !errors.Is(err, ErrSessionNotPinned) {
		t.Fatalf("nil session: %v", err)
	}
	if _, err := NewEffectIdentityAdapter(&PinnedSession{}); !errors.Is(err, ErrSessionNotPinned) {
		t.Fatalf("nil conn: %v", err)
	}
	// No constructor from *sql.DB — type surface check.
	rt := reflect.TypeOf(EffectIdentityAdapter{})
	for i := 0; i < rt.NumField(); i++ {
		if rt.Field(i).Type == reflect.TypeOf((*sql.DB)(nil)) {
			t.Fatal("adapter must not hold *sql.DB pool")
		}
	}
}

func TestPinSession_NilDB(t *testing.T) {
	t.Parallel()
	if _, err := PinSession(context.Background(), nil); !errors.Is(err, ErrSessionNotPinned) {
		t.Fatalf("got %v", err)
	}
}

func completeLive() appqa.EffectIdentityResolutionContext {
	return appqa.EffectIdentityResolutionContext{
		Bound:               true,
		SessionBinding:      "b1-d16384",
		PathEpoch:           1,
		NamespaceSearchOIDs: []uint32{2200, 11}, // public, pg_catalog
		DatabaseOID:         16384,
		RoleOID:             10,
		ServerVersionNum:    170000,
	}
}

func TestAdapter_ExactOperatorAndFunctionFacts(t *testing.T) {
	t.Parallel()
	live := completeLive()
	fake := &fakeCatalog{
		live: live,
		ns:   map[string]uint32{"pg_catalog": 11, "public": 2200},
		ops: map[opKey][]operatorRow{
			{ns: 11, name: "=", left: 23, right: 23}: {{
				OID: 96, NamespaceOID: 11, ImplementationOID: 65, ResultTypeOID: 16,
				Volatility: "i", SchemaName: "pg_catalog", OperatorName: "=",
			}},
		},
		fns: map[fnKey][]functionRow{
			{ns: 11, name: "count", args: ""}: {{
				OID: 2803, NamespaceOID: 11, ResultType: 20, Volatility: "i",
				SchemaName: "pg_catalog", FuncName: "count", ArgTypeOIDs: nil,
			}},
		},
	}
	a := &EffectIdentityAdapter{catalog: fake}
	batch, err := a.ResolveEffectIdentities(context.Background(), appqa.EffectIdentityRequest{
		Dialect: "postgresql",
		Candidates: []appqa.EffectCandidate{
			{Kind: appqa.EffectCandidateOperator, Ordinal: 0, NamePath: []string{"="}, Arity: 2, OperandKinds: []string{"column", "const"}},
			{Kind: appqa.EffectCandidateFunction, Ordinal: 1, NamePath: []string{"count"}, Arity: 0, IsAggregate: true},
		},
		OperandTypeOIDs: map[int][]uint32{0: {23, 23}, 1: {}},
		Resolution:      live,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Items) != 2 {
		t.Fatalf("len=%d", len(batch.Items))
	}
	if batch.Items[0].Status != domain.IdentityStatusResolved || batch.Items[0].Facts == nil {
		t.Fatalf("operator: %+v", batch.Items[0])
	}
	if batch.Items[0].Facts.ObjectOID != 96 || batch.Items[0].Facts.DatabaseOID != live.DatabaseOID {
		t.Fatalf("operator facts: %+v", batch.Items[0].Facts)
	}
	if batch.Items[0].Facts.ServerVersionNum != live.ServerVersionNum {
		t.Fatalf("version pin: %d", batch.Items[0].Facts.ServerVersionNum)
	}
	if batch.Items[1].Status != domain.IdentityStatusResolved || batch.Items[1].Facts.ObjectOID != 2803 {
		t.Fatalf("count: %+v", batch.Items[1])
	}
	if fake.captureCount < 2 {
		t.Fatalf("expected live capture before and after lookup, got %d", fake.captureCount)
	}
}

func TestAdapter_UnqualifiedUsesPathOrder_CustomShadowsCatalog(t *testing.T) {
	t.Parallel()
	live := completeLive()
	fake := &fakeCatalog{
		live: live,
		ns:   map[string]uint32{"public": 2200, "pg_catalog": 11},
		fns: map[fnKey][]functionRow{
			{ns: 2200, name: "count", args: ""}: {{
				OID: 900001, NamespaceOID: 2200, ResultType: 20, Volatility: "v",
				SchemaName: "public", FuncName: "count",
			}},
			{ns: 11, name: "count", args: ""}: {{
				OID: 2803, NamespaceOID: 11, ResultType: 20, Volatility: "i",
				SchemaName: "pg_catalog", FuncName: "count",
			}},
		},
	}
	a := &EffectIdentityAdapter{catalog: fake}
	// Collect-all exact matches across path → ambiguous when both public and pg_catalog match.
	batch, err := a.ResolveEffectIdentities(context.Background(), appqa.EffectIdentityRequest{
		Candidates: []appqa.EffectCandidate{
			{Kind: appqa.EffectCandidateFunction, Ordinal: 0, NamePath: []string{"count"}, Arity: 0},
		},
		Resolution: live,
	})
	if err != nil {
		t.Fatal(err)
	}
	// Both namespaces have exact zero-arg count → ambiguous (must not pick pg_catalog by name).
	if batch.Items[0].Status != domain.IdentityStatusAmbiguous {
		t.Fatalf("expected ambiguous when path has custom+catalog, got %+v", batch.Items[0])
	}

	// Only public has the function → actual public OID facts.
	fake.fns = map[fnKey][]functionRow{
		{ns: 2200, name: "count", args: ""}: {{
			OID: 900001, NamespaceOID: 2200, ResultType: 20, Volatility: "v",
			SchemaName: "public", FuncName: "count",
		}},
	}
	batch, err = a.ResolveEffectIdentities(context.Background(), appqa.EffectIdentityRequest{
		Candidates: []appqa.EffectCandidate{
			{Kind: appqa.EffectCandidateFunction, Ordinal: 0, NamePath: []string{"count"}, Arity: 0},
		},
		Resolution: live,
	})
	if err != nil {
		t.Fatal(err)
	}
	if batch.Items[0].Status != domain.IdentityStatusResolved || batch.Items[0].Facts.ObjectOID != 900001 {
		t.Fatalf("custom public count: %+v", batch.Items[0])
	}
	if batch.Items[0].Facts.NamespaceOID != 2200 {
		t.Fatalf("must not replace with pg_catalog: %+v", batch.Items[0].Facts)
	}
}

func TestAdapter_ExplicitPgCatalogAndPublicReturnFactsOnly(t *testing.T) {
	t.Parallel()
	live := completeLive()
	fake := &fakeCatalog{
		live: live,
		ns:   map[string]uint32{"pg_catalog": 11, "public": 2200},
		fns: map[fnKey][]functionRow{
			{ns: 11, name: "length", args: "25"}: {{
				OID: 1374, NamespaceOID: 11, ResultType: 23, Volatility: "i",
				SchemaName: "pg_catalog", FuncName: "length", ArgTypeOIDs: []uint32{25},
			}},
			{ns: 2200, name: "my_udf", args: "23"}: {{
				OID: 900002, NamespaceOID: 2200, ResultType: 23, Volatility: "v",
				SchemaName: "public", FuncName: "my_udf", ArgTypeOIDs: []uint32{23},
			}},
		},
	}
	a := &EffectIdentityAdapter{catalog: fake}
	batch, err := a.ResolveEffectIdentities(context.Background(), appqa.EffectIdentityRequest{
		Candidates: []appqa.EffectCandidate{
			{Kind: appqa.EffectCandidateFunction, Ordinal: 0, NamePath: []string{"pg_catalog", "length"}, ExplicitSchema: true, Arity: 1},
			{Kind: appqa.EffectCandidateFunction, Ordinal: 1, NamePath: []string{"public", "my_udf"}, ExplicitSchema: true, Arity: 1},
		},
		OperandTypeOIDs: map[int][]uint32{0: {25}, 1: {23}},
		Resolution:      live,
	})
	if err != nil {
		t.Fatal(err)
	}
	if batch.Items[0].Status != domain.IdentityStatusResolved || batch.Items[0].Facts.ObjectOID != 1374 {
		t.Fatalf("explicit pg_catalog: %+v", batch.Items[0])
	}
	if batch.Items[1].Status != domain.IdentityStatusResolved || batch.Items[1].Facts.NamespaceOID != 2200 {
		t.Fatalf("explicit public facts: %+v", batch.Items[1])
	}
	// No Trusted field.
	if _, ok := reflect.TypeOf(*batch.Items[0].Facts).FieldByName("Trusted"); ok {
		t.Fatal("Trusted forbidden")
	}
}

func TestAdapter_MissingTypesAndParamFailClosed(t *testing.T) {
	t.Parallel()
	fake := &fakeCatalog{live: completeLive(), ns: map[string]uint32{"pg_catalog": 11}}
	a := &EffectIdentityAdapter{catalog: fake}
	batch, err := a.ResolveEffectIdentities(context.Background(), appqa.EffectIdentityRequest{
		Candidates: []appqa.EffectCandidate{
			{Kind: appqa.EffectCandidateOperator, Ordinal: 0, NamePath: []string{"="}, Arity: 2}, // no type OIDs
			{Kind: appqa.EffectCandidateFunction, Ordinal: 1, NamePath: []string{"length"}, Arity: 1, OperandKinds: []string{"param"}},
		},
		Resolution: completeLive(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if batch.Items[0].Status != domain.IdentityStatusCoercionGap {
		t.Errorf("missing types: %s", batch.Items[0].Status)
	}
	if batch.Items[1].Status != domain.IdentityStatusCoercionGap {
		t.Errorf("param: %s", batch.Items[1].Status)
	}
}

func TestAdapter_MultiMatchAmbiguous(t *testing.T) {
	t.Parallel()
	live := completeLive()
	fake := &fakeCatalog{
		live: live,
		ns:   map[string]uint32{"pg_catalog": 11},
		ops: map[opKey][]operatorRow{
			{ns: 11, name: "+", left: 23, right: 23}: {
				{OID: 1, NamespaceOID: 11, SchemaName: "pg_catalog", OperatorName: "+"},
				{OID: 2, NamespaceOID: 11, SchemaName: "pg_catalog", OperatorName: "+"},
			},
		},
	}
	a := &EffectIdentityAdapter{catalog: fake}
	batch, err := a.ResolveEffectIdentities(context.Background(), appqa.EffectIdentityRequest{
		Candidates: []appqa.EffectCandidate{
			{Kind: appqa.EffectCandidateOperator, Ordinal: 0, NamePath: []string{"+"}, Arity: 2},
		},
		OperandTypeOIDs: map[int][]uint32{0: {23, 23}},
		Resolution:      live,
	})
	if err != nil {
		t.Fatal(err)
	}
	if batch.Items[0].Status != domain.IdentityStatusAmbiguous || batch.Items[0].Facts != nil {
		t.Fatalf("got %+v", batch.Items[0])
	}
}

func TestAdapter_CastExactAndUnknown(t *testing.T) {
	t.Parallel()
	live := completeLive()
	fake := &fakeCatalog{
		live:  live,
		types: map[string]uint32{"pg_catalog.text": 25},
		casts: map[castKey][]castRow{
			{src: 23, tgt: 25}: {{
				OID: 1, SourceOID: 23, TargetOID: 25, CastFuncOID: 0, CastMethod: "b",
				SourceSchema: "pg_catalog", SourceName: "int4", TargetSchema: "pg_catalog", TargetName: "text",
			}},
		},
	}
	a := &EffectIdentityAdapter{catalog: fake}
	batch, err := a.ResolveEffectIdentities(context.Background(), appqa.EffectIdentityRequest{
		Candidates: []appqa.EffectCandidate{
			{Kind: appqa.EffectCandidateCast, Ordinal: 0, TargetTypePath: []string{"pg_catalog", "text"}, ExplicitSchema: true, Arity: 1, OperandKinds: []string{"column"}},
		},
		OperandTypeOIDs: map[int][]uint32{0: {23}},
		Resolution:      live,
	})
	if err != nil {
		t.Fatal(err)
	}
	if batch.Items[0].Status != domain.IdentityStatusResolved || batch.Items[0].Facts.CastMethod != appqa.EffectCastMethodBinary {
		t.Fatalf("cast: %+v", batch.Items[0])
	}

	// No cast row.
	fake.casts = nil
	batch, err = a.ResolveEffectIdentities(context.Background(), appqa.EffectIdentityRequest{
		Candidates: []appqa.EffectCandidate{
			{Kind: appqa.EffectCandidateCast, Ordinal: 0, TargetTypePath: []string{"pg_catalog", "text"}, ExplicitSchema: true, Arity: 1},
		},
		OperandTypeOIDs: map[int][]uint32{0: {23}},
		Resolution:      live,
	})
	if err != nil {
		t.Fatal(err)
	}
	if batch.Items[0].Status != domain.IdentityStatusUnknown {
		t.Fatalf("missing cast: %s", batch.Items[0].Status)
	}
}

func TestAdapter_CatalogErrorMapsToLookupFailed_NoLeak(t *testing.T) {
	t.Parallel()
	live := completeLive()
	fake := &fakeCatalog{
		live:    live,
		ns:      map[string]uint32{"pg_catalog": 11},
		opErr:   errors.New(`pq: password authentication failed DSN=postgres://u:s@h/db`),
		ops:     map[opKey][]operatorRow{},
		forceOp: true,
	}
	a := &EffectIdentityAdapter{catalog: fake}
	batch, err := a.ResolveEffectIdentities(context.Background(), appqa.EffectIdentityRequest{
		Candidates: []appqa.EffectCandidate{
			{Kind: appqa.EffectCandidateOperator, Ordinal: 0, NamePath: []string{"="}, Arity: 2},
		},
		OperandTypeOIDs: map[int][]uint32{0: {23, 23}},
		Resolution:      live,
	})
	if err != nil {
		t.Fatal(err)
	}
	if batch.Items[0].Status != domain.IdentityStatusLookupFailed {
		t.Fatalf("status=%s", batch.Items[0].Status)
	}
	// Status string must not contain secrets.
	if strings.Contains(string(batch.Items[0].Status), "password") {
		t.Fatal("status leaked error text")
	}
}

func TestAdapter_LiveSessionMismatchStripsAll(t *testing.T) {
	t.Parallel()
	live1 := completeLive()
	live2 := completeLive()
	live2.RoleOID = 99 // role drift after lookup
	fake := &fakeCatalog{
		live:       live1,
		liveSecond: &live2,
		ns:         map[string]uint32{"pg_catalog": 11},
		ops: map[opKey][]operatorRow{
			{ns: 11, name: "=", left: 23, right: 23}: {{
				OID: 96, NamespaceOID: 11, SchemaName: "pg_catalog", OperatorName: "=", Volatility: "i",
			}},
		},
		fns: map[fnKey][]functionRow{
			{ns: 11, name: "length", args: "25"}: {{
				OID: 1374, NamespaceOID: 11, SchemaName: "pg_catalog", FuncName: "length", ArgTypeOIDs: []uint32{25}, Volatility: "i",
			}},
		},
	}
	a := &EffectIdentityAdapter{catalog: fake}
	batch, err := a.ResolveEffectIdentities(context.Background(), appqa.EffectIdentityRequest{
		Candidates: []appqa.EffectCandidate{
			{Kind: appqa.EffectCandidateOperator, Ordinal: 0, NamePath: []string{"="}, Arity: 2},
			{Kind: appqa.EffectCandidateFunction, Ordinal: 1, NamePath: []string{"pg_catalog", "length"}, ExplicitSchema: true, Arity: 1},
		},
		OperandTypeOIDs: map[int][]uint32{0: {23, 23}, 1: {25}},
		Resolution:      live1,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range batch.Items {
		if it.Status != domain.IdentityStatusUnavailable || it.Facts != nil {
			t.Errorf("role drift must strip all including explicit: %+v", it)
		}
	}
}

func TestAdapter_CaptureFailureUnavailable(t *testing.T) {
	t.Parallel()
	a := &EffectIdentityAdapter{catalog: &fakeCatalog{captureErr: errors.New("boom")}}
	batch, err := a.ResolveEffectIdentities(context.Background(), appqa.EffectIdentityRequest{
		Candidates: []appqa.EffectCandidate{
			{Kind: appqa.EffectCandidateFunction, Ordinal: 0, NamePath: []string{"count"}, Arity: 0},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if batch.Items[0].Status != domain.IdentityStatusUnavailable {
		t.Fatalf("got %s", batch.Items[0].Status)
	}
}

func TestAdapter_Cancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	a := &EffectIdentityAdapter{catalog: &fakeCatalog{live: completeLive()}}
	_, err := a.ResolveEffectIdentities(ctx, appqa.EffectIdentityRequest{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}

func TestAdapter_UnboundRequestRejected(t *testing.T) {
	t.Parallel()
	live := completeLive()
	fake := &fakeCatalog{
		live: live,
		ns:   map[string]uint32{"pg_catalog": 11},
		fns: map[fnKey][]functionRow{
			{ns: 11, name: "count", args: ""}: {{
				OID: 2803, NamespaceOID: 11, ResultType: 20, Volatility: "i",
				SchemaName: "pg_catalog", FuncName: "count",
			}},
		},
	}
	a := &EffectIdentityAdapter{catalog: fake}
	batch, err := a.ResolveEffectIdentities(context.Background(), appqa.EffectIdentityRequest{
		Candidates: []appqa.EffectCandidate{
			{Kind: appqa.EffectCandidateFunction, Ordinal: 0, NamePath: []string{"count"}, Arity: 0},
		},
		// No Resolution: unbound request.
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range batch.Items {
		if it.Status != domain.IdentityStatusUnavailable || it.Facts != nil {
			t.Errorf("unbound request must yield unavailable, got: %+v", it)
		}
	}
}

func TestResolveColumnTypeOIDs_ValidColumns(t *testing.T) {
	t.Parallel()
	fake := &fakeCatalog{
		live: completeLive(),
		cols: map[colKey]uint32{
			{schema: "public", table: "users", column: "id"}:   23,
			{schema: "public", table: "users", column: "name"}: 25,
		},
	}
	a := &EffectIdentityAdapter{catalog: fake}
	candidates := []appqa.EffectCandidate{
		{
			Kind:    appqa.EffectCandidateOperator,
			Ordinal: 0,
			Arity:   2,
			OperandColumnRefs: []appqa.OperandColumnRef{
				{Schema: "public", Table: "users", Column: "id"},
				{Schema: "public", Table: "users", Column: "name"},
			},
		},
	}
	result, err := a.ResolveColumnTypeOIDs(context.Background(), candidates)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result))
	}
	oids, ok := result[0]
	if !ok {
		t.Fatal("missing ordinal 0")
	}
	if len(oids) != 2 || oids[0] != 23 || oids[1] != 25 {
		t.Fatalf("unexpected OIDs: %v", oids)
	}
}

func TestResolveColumnTypeOIDs_MissingColumnReturnsEmpty(t *testing.T) {
	t.Parallel()
	fake := &fakeCatalog{
		live: completeLive(),
		cols: map[colKey]uint32{
			{schema: "public", table: "users", column: "id"}: 23,
		},
	}
	a := &EffectIdentityAdapter{catalog: fake}
	candidates := []appqa.EffectCandidate{
		{
			Kind:    appqa.EffectCandidateOperator,
			Ordinal: 0,
			Arity:   2,
			OperandColumnRefs: []appqa.OperandColumnRef{
				{Schema: "public", Table: "users", Column: "id"},
				{Schema: "public", Table: "users", Column: "missing_col"},
			},
		},
	}
	result, err := a.ResolveColumnTypeOIDs(context.Background(), candidates)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 0 {
		t.Fatalf("expected empty map, got %d entries", len(result))
	}
}

func TestResolveColumnTypeOIDs_SkipsNonOperator(t *testing.T) {
	t.Parallel()
	fake := &fakeCatalog{
		live: completeLive(),
		cols: map[colKey]uint32{
			{schema: "public", table: "users", column: "id"}: 23,
		},
	}
	a := &EffectIdentityAdapter{catalog: fake}
	candidates := []appqa.EffectCandidate{
		{
			Kind:    appqa.EffectCandidateFunction,
			Ordinal: 0,
			Arity:   2,
			OperandColumnRefs: []appqa.OperandColumnRef{
				{Schema: "public", Table: "users", Column: "id"},
				{Schema: "public", Table: "users", Column: "id"},
			},
		},
	}
	result, err := a.ResolveColumnTypeOIDs(context.Background(), candidates)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 0 {
		t.Fatalf("expected empty map for non-operator, got %d entries", len(result))
	}
}

func TestResolveColumnTypeOIDs_SkipsNonBinary(t *testing.T) {
	t.Parallel()
	fake := &fakeCatalog{
		live: completeLive(),
		cols: map[colKey]uint32{
			{schema: "public", table: "users", column: "id"}: 23,
		},
	}
	a := &EffectIdentityAdapter{catalog: fake}
	candidates := []appqa.EffectCandidate{
		{
			Kind:    appqa.EffectCandidateOperator,
			Ordinal: 0,
			Arity:   1,
			OperandColumnRefs: []appqa.OperandColumnRef{
				{Schema: "public", Table: "users", Column: "id"},
			},
		},
	}
	result, err := a.ResolveColumnTypeOIDs(context.Background(), candidates)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 0 {
		t.Fatalf("expected empty map for unary operator, got %d entries", len(result))
	}
}

func TestResolveColumnTypeOIDs_SkipsNilOperandColumnRefs(t *testing.T) {
	t.Parallel()
	fake := &fakeCatalog{live: completeLive()}
	a := &EffectIdentityAdapter{catalog: fake}
	candidates := []appqa.EffectCandidate{
		{
			Kind:    appqa.EffectCandidateOperator,
			Ordinal: 0,
			Arity:   2,
		},
	}
	result, err := a.ResolveColumnTypeOIDs(context.Background(), candidates)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 0 {
		t.Fatalf("expected empty map for nil refs, got %d entries", len(result))
	}
}

func TestResolveColumnTypeOIDs_SkipsEmptySchema(t *testing.T) {
	t.Parallel()
	fake := &fakeCatalog{
		live: completeLive(),
		cols: map[colKey]uint32{
			{schema: "public", table: "users", column: "id"}: 23,
		},
		ns: map[string]uint32{"public": 2200, "pg_catalog": 11},
	}
	a := &EffectIdentityAdapter{catalog: fake}
	candidates := []appqa.EffectCandidate{
		{
			Kind:    appqa.EffectCandidateOperator,
			Ordinal: 0,
			Arity:   2,
			OperandColumnRefs: []appqa.OperandColumnRef{
				{Schema: "", Table: "users", Column: "id"},
				{Schema: "public", Table: "users", Column: "id"},
			},
		},
	}
	result, err := a.ResolveColumnTypeOIDs(context.Background(), candidates)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 entry (resolved via search_path), got %d entries", len(result))
	}
	oids, ok := result[0]
	if !ok || len(oids) != 2 || oids[0] != 23 || oids[1] != 23 {
		t.Fatalf("unexpected OIDs: %v", result)
	}
}

func TestResolveColumnTypeOIDs_UnqualifiedResolvesViaSearchPath(t *testing.T) {
	t.Parallel()
	fake := &fakeCatalog{
		live: completeLive(),
		cols: map[colKey]uint32{
			{schema: "public", table: "users", column: "id"}:   23,
			{schema: "public", table: "users", column: "name"}: 25,
		},
		ns: map[string]uint32{"public": 2200, "pg_catalog": 11},
	}
	a := &EffectIdentityAdapter{catalog: fake}

	candidates := []appqa.EffectCandidate{
		{
			Kind:    appqa.EffectCandidateOperator,
			Ordinal: 0,
			Arity:   2,
			OperandColumnRefs: []appqa.OperandColumnRef{
				{Schema: "", Table: "users", Column: "id"},
				{Schema: "", Table: "users", Column: "name"},
			},
		},
	}
	result, err := a.ResolveColumnTypeOIDs(context.Background(), candidates)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result))
	}
	oids, ok := result[0]
	if !ok || len(oids) != 2 || oids[0] != 23 || oids[1] != 25 {
		t.Fatalf("unexpected OIDs: %v", result)
	}
}

func TestResolveColumnTypeOIDs_UnqualifiedNotInSearchPath(t *testing.T) {
	t.Parallel()
	fake := &fakeCatalog{
		live: completeLive(),
		cols: map[colKey]uint32{
			{schema: "custom", table: "users", column: "id"}:   20,
			{schema: "custom", table: "users", column: "name"}: 1042,
		},
		ns: map[string]uint32{"public": 2200, "custom": 16385, "pg_catalog": 11},
	}
	a := &EffectIdentityAdapter{catalog: fake}

	candidates := []appqa.EffectCandidate{
		{
			Kind:    appqa.EffectCandidateOperator,
			Ordinal: 0,
			Arity:   2,
			OperandColumnRefs: []appqa.OperandColumnRef{
				{Schema: "", Table: "users", Column: "id"},
				{Schema: "", Table: "users", Column: "name"},
			},
		},
	}
	result, err := a.ResolveColumnTypeOIDs(context.Background(), candidates)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 0 {
		t.Fatalf("expected empty (custom not in search_path), got %v", result)
	}
}

func TestResolveColumnTypeOIDs_UnqualifiedNotFoundReturnsEmpty(t *testing.T) {
	t.Parallel()
	fake := &fakeCatalog{
		live: completeLive(),
		cols: map[colKey]uint32{},
		ns:   map[string]uint32{"public": 2200, "pg_catalog": 11},
	}
	a := &EffectIdentityAdapter{catalog: fake}

	candidates := []appqa.EffectCandidate{
		{
			Kind:    appqa.EffectCandidateOperator,
			Ordinal: 0,
			Arity:   2,
			OperandColumnRefs: []appqa.OperandColumnRef{
				{Schema: "", Table: "nonexistent", Column: "id"},
				{Schema: "", Table: "nonexistent", Column: "name"},
			},
		},
	}
	result, err := a.ResolveColumnTypeOIDs(context.Background(), candidates)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 0 {
		t.Fatalf("expected empty map for nonexistent table, got %d entries", len(result))
	}
}

func TestResolveColumnTypeOIDs_ContextCancelled(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	a := &EffectIdentityAdapter{catalog: &fakeCatalog{live: completeLive()}}
	_, err := a.ResolveColumnTypeOIDs(ctx, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestResolveColumnTypeOIDs_NilAdapter(t *testing.T) {
	t.Parallel()
	var a *EffectIdentityAdapter
	_, err := a.ResolveColumnTypeOIDs(context.Background(), nil)
	if !errors.Is(err, ErrSessionNotPinned) {
		t.Fatalf("expected ErrSessionNotPinned, got %v", err)
	}
}

func TestResolveColumnTypeOIDs_CatalogErrorSkips(t *testing.T) {
	t.Parallel()
	fake := &fakeCatalog{
		live:   completeLive(),
		colErr: errors.New("connection lost"),
		cols:   map[colKey]uint32{},
	}
	a := &EffectIdentityAdapter{catalog: fake}
	candidates := []appqa.EffectCandidate{
		{
			Kind:    appqa.EffectCandidateOperator,
			Ordinal: 0,
			Arity:   2,
			OperandColumnRefs: []appqa.OperandColumnRef{
				{Schema: "public", Table: "users", Column: "id"},
				{Schema: "public", Table: "users", Column: "name"},
			},
		},
	}
	result, err := a.ResolveColumnTypeOIDs(context.Background(), candidates)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 0 {
		t.Fatalf("expected empty map on catalog error, got %d entries", len(result))
	}
}

func TestResolveColumnTypeOIDs_MultipleCandidatesPartial(t *testing.T) {
	t.Parallel()
	fake := &fakeCatalog{
		live: completeLive(),
		cols: map[colKey]uint32{
			{schema: "public", table: "users", column: "id"}:   23,
			{schema: "public", table: "users", column: "name"}: 25,
		},
	}
	a := &EffectIdentityAdapter{catalog: fake}
	candidates := []appqa.EffectCandidate{
		{
			Kind:    appqa.EffectCandidateOperator,
			Ordinal: 0,
			Arity:   2,
			OperandColumnRefs: []appqa.OperandColumnRef{
				{Schema: "public", Table: "users", Column: "id"},
				{Schema: "public", Table: "users", Column: "name"},
			},
		},
		{
			Kind:    appqa.EffectCandidateOperator,
			Ordinal: 1,
			Arity:   2,
			OperandColumnRefs: []appqa.OperandColumnRef{
				{Schema: "public", Table: "users", Column: "id"},
				{Schema: "public", Table: "missing", Column: "col"},
			},
		},
		{
			Kind:    appqa.EffectCandidateFunction,
			Ordinal: 2,
			Arity:   0,
		},
	}
	result, err := a.ResolveColumnTypeOIDs(context.Background(), candidates)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result))
	}
	oids, ok := result[0]
	if !ok || len(oids) != 2 || oids[0] != 23 || oids[1] != 25 {
		t.Fatalf("unexpected result: %v", result)
	}
}

func TestResolveColumnTypeOIDs_FunctionColumn(t *testing.T) {
	t.Parallel()
	fake := &fakeCatalog{
		live: completeLive(),
		cols: map[colKey]uint32{
			{schema: "app", table: "orders", column: "amount"}: 1700,
		},
	}
	a := &EffectIdentityAdapter{catalog: fake}
	result, err := a.ResolveColumnTypeOIDs(context.Background(), []appqa.EffectCandidate{
		{
			Kind:              appqa.EffectCandidateFunction,
			Ordinal:           7,
			Arity:             1,
			OperandKinds:      []string{"column"},
			OperandColumnRefs: []appqa.OperandColumnRef{{Schema: "app", Table: "orders", Column: "amount"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := result[7]; len(got) != 1 || got[0] != 1700 {
		t.Fatalf("expected function column type OID, got %v", result)
	}
}

func TestAdapter_CountColumnUsesUniqueAnyElementCatalogMatch(t *testing.T) {
	t.Parallel()
	live := completeLive()
	fake := &fakeCatalog{
		live: live,
		ns:   map[string]uint32{"pg_catalog": 11},
		fns: map[fnKey][]functionRow{
			{ns: 11, name: "count", args: "2276"}: {{
				OID: 2147, NamespaceOID: 11, ResultType: 20, Volatility: "i",
				SchemaName: "pg_catalog", FuncName: "count", ArgTypeOIDs: []uint32{2276},
			}},
		},
	}
	a := &EffectIdentityAdapter{catalog: fake}
	batch, err := a.ResolveEffectIdentities(context.Background(), appqa.EffectIdentityRequest{
		Candidates: []appqa.EffectCandidate{{
			Kind:              appqa.EffectCandidateFunction,
			Ordinal:           0,
			NamePath:          []string{"pg_catalog", "count"},
			Arity:             1,
			OperandKinds:      []string{"column"},
			OperandColumnRefs: []appqa.OperandColumnRef{{Schema: "app", Table: "orders", Column: "amount"}},
		}},
		OperandTypeOIDs: map[int][]uint32{0: {23}},
		Resolution:      live,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Items) != 1 || batch.Items[0].Status != domain.IdentityStatusResolved {
		t.Fatalf("expected resolved count(anyelement), got %+v", batch.Items)
	}
	if got := batch.Items[0].Facts.OperandTypeOIDs; !reflect.DeepEqual(got, []uint32{2276}) {
		t.Fatalf("expected catalog polymorphic operand OID, got %v", got)
	}
}

// --- fake catalog ---

type opKey struct {
	ns          uint32
	name        string
	left, right uint32
}

type fnKey struct {
	ns   uint32
	name string
	args string
}

type castKey struct {
	src, tgt uint32
}

type colKey struct {
	schema, table, column string
}

type fakeCatalog struct {
	live         appqa.EffectIdentityResolutionContext
	liveSecond   *appqa.EffectIdentityResolutionContext
	captureErr   error
	captureCount int
	ns           map[string]uint32
	types        map[string]uint32
	ops          map[opKey][]operatorRow
	fns          map[fnKey][]functionRow
	casts        map[castKey][]castRow
	opErr        error
	forceOp      bool
	cols         map[colKey]uint32
	colErr       error
}

func (f *fakeCatalog) CaptureLiveContext(ctx context.Context) (appqa.EffectIdentityResolutionContext, error) {
	f.captureCount++
	if f.captureErr != nil {
		return appqa.EffectIdentityResolutionContext{}, f.captureErr
	}
	if f.captureCount >= 2 && f.liveSecond != nil {
		return *f.liveSecond, nil
	}
	return f.live, nil
}

func (f *fakeCatalog) namespaceOIDByName(_ context.Context, name string) (uint32, error) {
	if oid, ok := f.ns[name]; ok {
		return oid, nil
	}
	return 0, sql.ErrNoRows
}

func (f *fakeCatalog) typeOIDByName(_ context.Context, schema, typname string, _ []uint32) (uint32, error) {
	key := schema + "." + typname
	if schema == "" {
		key = typname
	}
	if oid, ok := f.types[key]; ok {
		return oid, nil
	}
	return 0, sql.ErrNoRows
}

func (f *fakeCatalog) lookupOperators(_ context.Context, nsOID uint32, name string, left, right uint32) ([]operatorRow, error) {
	if f.opErr != nil {
		return nil, f.opErr
	}
	return append([]operatorRow(nil), f.ops[opKey{ns: nsOID, name: name, left: left, right: right}]...), nil
}

func (f *fakeCatalog) lookupFunctions(_ context.Context, nsOID uint32, name string, argOIDs []uint32) ([]functionRow, error) {
	key := fnKey{ns: nsOID, name: name, args: oidVectorLiteral(argOIDs)}
	rows := f.fns[key]
	if len(rows) == 0 && len(argOIDs) == 1 {
		rows = f.fns[fnKey{ns: nsOID, name: name, args: "2276"}]
	}
	return append([]functionRow(nil), rows...), nil
}

func (f *fakeCatalog) lookupCasts(_ context.Context, sourceOID, targetOID uint32) ([]castRow, error) {
	return append([]castRow(nil), f.casts[castKey{src: sourceOID, tgt: targetOID}]...), nil
}

func (f *fakeCatalog) columnTypeOID(_ context.Context, schema, table, column string) (uint32, error) {
	if f.colErr != nil {
		return 0, f.colErr
	}
	oid, ok := f.cols[colKey{schema: schema, table: table, column: column}]
	if !ok {
		return 0, sql.ErrNoRows
	}
	return oid, nil
}

func (f *fakeCatalog) resolveColumnTypeOIDBySearchPath(_ context.Context, table, column string, searchPathOIDs []uint32) (uint32, error) {
	if f.colErr != nil {
		return 0, f.colErr
	}
	for _, nsOID := range searchPathOIDs {
		for k, oid := range f.cols {
			if k.table == table && k.column == column {
				for name, nsO := range f.ns {
					if nsO == nsOID && name == k.schema {
						return oid, nil
					}
				}
			}
		}
	}
	return 0, sql.ErrNoRows
}
