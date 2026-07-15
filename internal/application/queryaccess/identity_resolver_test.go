// Package queryaccess tests the effect-identity resolver contract (T6, facts only).
// input: synthetic candidate batches and fake resolvers
// output: bounded statuses, ordinal determinism, fail-closed/no-leak assertions
// pos: contract unit tests; no catalog SQL, no admission promotion
// note: if this file changes, update this header and module README.md.
package queryaccess

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

func sampleCandidates() []EffectCandidate {
	return []EffectCandidate{
		{Kind: EffectCandidateOperator, Ordinal: 0, NamePath: []string{"="}, Arity: 2},
		{Kind: EffectCandidateFunction, Ordinal: 1, NamePath: []string{"count"}, Arity: 0, IsAggregate: true},
		{Kind: EffectCandidateCast, Ordinal: 2, TargetTypePath: []string{"pg_catalog", "text"}, Arity: 1},
	}
}

func TestValidateEffectIdentityRequest_UniqueOrdinals(t *testing.T) {
	t.Parallel()
	req := EffectIdentityRequest{Dialect: "postgresql", Candidates: sampleCandidates()}
	if err := ValidateEffectIdentityRequest(req); err != nil {
		t.Fatalf("valid request: %v", err)
	}

	dup := EffectIdentityRequest{
		Dialect: "postgresql",
		Candidates: []EffectCandidate{
			{Kind: EffectCandidateOperator, Ordinal: 1},
			{Kind: EffectCandidateFunction, Ordinal: 1},
		},
	}
	if err := ValidateEffectIdentityRequest(dup); !errors.Is(err, ErrDuplicateIdentityOrdinal) {
		t.Fatalf("want ErrDuplicateIdentityOrdinal, got %v", err)
	}

	badKind := EffectIdentityRequest{
		Candidates: []EffectCandidate{{Kind: EffectCandidateKind("free_text"), Ordinal: 0}},
	}
	if err := ValidateEffectIdentityRequest(badKind); !errors.Is(err, ErrIdentityRequestInvalid) {
		t.Fatalf("want ErrIdentityRequestInvalid, got %v", err)
	}
}

func TestIdentityStatus_BoundedEnumOnly(t *testing.T) {
	t.Parallel()
	allowed := []domain.IdentityStatus{
		domain.IdentityStatusResolved,
		domain.IdentityStatusUnknown,
		domain.IdentityStatusAmbiguous,
		domain.IdentityStatusCoercionGap,
		domain.IdentityStatusLookupFailed,
		domain.IdentityStatusUnavailable,
	}
	for _, s := range allowed {
		if !domain.ValidIdentityStatus(s) {
			t.Errorf("expected valid status %q", s)
		}
	}
	for _, bad := range []domain.IdentityStatus{
		"",
		"trusted",
		"error", // IdentityFailure uses "error"; status uses lookup_failed
		"postgres://user:secret@host/db",
		"SELECT oid FROM pg_operator WHERE oprname = '='",
		"connection refused",
		domain.IdentityStatus("custom"),
	} {
		if domain.ValidIdentityStatus(bad) {
			t.Errorf("free-text status %q must be rejected", bad)
		}
		if code, ok := domain.ReasonForIdentityStatus(bad); ok {
			t.Errorf("free-text status %q must not map to reason %q", bad, code)
		}
	}
}

func TestIdentityStatus_FailClosedMapping(t *testing.T) {
	t.Parallel()
	cases := []struct {
		status domain.IdentityStatus
		fail   domain.IdentityFailure
		reason domain.ReasonCode
	}{
		{domain.IdentityStatusUnavailable, domain.IdentityFailureUnavailable, domain.ReasonIdentityResolverUnavailable},
		{domain.IdentityStatusUnknown, domain.IdentityFailureUnknown, domain.ReasonIdentityUnknown},
		{domain.IdentityStatusLookupFailed, domain.IdentityFailureError, domain.ReasonIdentityLookupFailed},
		{domain.IdentityStatusAmbiguous, domain.IdentityFailureAmbiguous, domain.ReasonIdentityAmbiguous},
		{domain.IdentityStatusCoercionGap, domain.IdentityFailureCoercionGap, domain.ReasonIdentityCoercionGap},
	}
	for _, tc := range cases {
		if !domain.IdentityStatusIsFailClosed(tc.status) {
			t.Errorf("%q must be fail-closed", tc.status)
		}
		f, ok := domain.IdentityStatusToFailure(tc.status)
		if !ok || f != tc.fail {
			t.Errorf("status %q → failure %q ok=%v, want %q", tc.status, f, ok, tc.fail)
		}
		code, ok := domain.ReasonForIdentityStatus(tc.status)
		if !ok || code != tc.reason {
			t.Errorf("status %q → reason %q ok=%v, want %q", tc.status, code, ok, tc.reason)
		}
	}
	if domain.IdentityStatusIsFailClosed(domain.IdentityStatusResolved) {
		t.Error("resolved must not be fail-closed for status purposes")
	}
	if _, ok := domain.IdentityStatusToFailure(domain.IdentityStatusResolved); ok {
		t.Error("resolved must not map to a failure")
	}
	if _, ok := domain.ReasonForIdentityStatus(domain.IdentityStatusResolved); ok {
		t.Error("resolved must not map to a reason code via this helper")
	}
}

func TestNormalizeEffectIdentityBatch_SortDedupeSanitize(t *testing.T) {
	t.Parallel()
	secretFacts := &EffectIdentityFacts{
		Kind:               EffectCandidateOperator,
		ObjectOID:          96,
		CanonicalSignature: "pg_catalog.=(int4,int4)",
	}
	batch := NormalizeEffectIdentityBatch([]EffectIdentityItem{
		{Ordinal: 2, Status: domain.IdentityStatusUnknown, Facts: secretFacts}, // facts must drop
		{Ordinal: 0, Status: domain.IdentityStatusResolved, Facts: secretFacts},
		{Ordinal: 1, Status: domain.IdentityStatus("SELECT * FROM secrets"), Facts: secretFacts},
		{Ordinal: 0, Status: domain.IdentityStatusAmbiguous}, // duplicate ordinal dropped
		{Ordinal: 3, Status: domain.IdentityStatusResolved, Facts: &EffectIdentityFacts{
			Volatility: EffectVolatility("not_a_vol"),
		}},
	})
	if len(batch.Items) != 4 {
		t.Fatalf("items=%d want 4", len(batch.Items))
	}
	// Sorted ordinals: 0,1,2,3
	for i, wantOrd := range []int{0, 1, 2, 3} {
		if batch.Items[i].Ordinal != wantOrd {
			t.Errorf("item %d ordinal=%d want %d", i, batch.Items[i].Ordinal, wantOrd)
		}
	}
	if batch.Items[0].Status != domain.IdentityStatusResolved || batch.Items[0].Facts == nil {
		t.Errorf("ordinal 0: want resolved with facts, got %+v", batch.Items[0])
	}
	if batch.Items[1].Status != domain.IdentityStatusLookupFailed || batch.Items[1].Facts != nil {
		t.Errorf("free-text status must become lookup_failed without facts: %+v", batch.Items[1])
	}
	if batch.Items[2].Status != domain.IdentityStatusUnknown || batch.Items[2].Facts != nil {
		t.Errorf("non-resolved must drop facts: %+v", batch.Items[2])
	}
	if batch.Items[3].Status != domain.IdentityStatusLookupFailed || batch.Items[3].Facts != nil {
		t.Errorf("invalid volatility must fail closed: %+v", batch.Items[3])
	}
}

func TestCompleteEffectIdentityBatch_PartialFailureDeterministic(t *testing.T) {
	t.Parallel()
	req := EffectIdentityRequest{Candidates: sampleCandidates()}
	// Only ordinal 1 present; 0 and 2 missing → unavailable.
	partial := EffectIdentityBatch{Items: []EffectIdentityItem{
		{Ordinal: 1, Status: domain.IdentityStatusAmbiguous},
		{Ordinal: 99, Status: domain.IdentityStatusResolved, Facts: &EffectIdentityFacts{ObjectOID: 1}}, // extra dropped
	}}
	got := CompleteEffectIdentityBatch(req, partial)
	if len(got.Items) != 3 {
		t.Fatalf("len=%d want 3", len(got.Items))
	}
	want := []struct {
		ord    int
		status domain.IdentityStatus
	}{
		{0, domain.IdentityStatusUnavailable},
		{1, domain.IdentityStatusAmbiguous},
		{2, domain.IdentityStatusUnavailable},
	}
	for i, w := range want {
		if got.Items[i].Ordinal != w.ord || got.Items[i].Status != w.status {
			t.Errorf("item %d = {%d %s}, want {%d %s}", i, got.Items[i].Ordinal, got.Items[i].Status, w.ord, w.status)
		}
		if got.Items[i].Facts != nil {
			t.Errorf("item %d must not carry facts", i)
		}
	}
	// Determinism: two completes equal.
	got2 := CompleteEffectIdentityBatch(req, partial)
	if !reflect.DeepEqual(got, got2) {
		t.Error("CompleteEffectIdentityBatch must be deterministic")
	}
}

func TestBuildUnavailableBatch_AndFailClosedReasons(t *testing.T) {
	t.Parallel()
	batch := BuildUnavailableBatch(sampleCandidates())
	if len(batch.Items) != 3 {
		t.Fatalf("len=%d", len(batch.Items))
	}
	for i, it := range batch.Items {
		if it.Status != domain.IdentityStatusUnavailable || it.Facts != nil {
			t.Errorf("item %d: %+v", i, it)
		}
	}
	codes := FailClosedReasonCodes(batch)
	if len(codes) != 1 || codes[0] != domain.ReasonIdentityResolverUnavailable {
		t.Errorf("codes=%v want single identity_resolver_unavailable", codes)
	}
}

func TestMapCatalogErrorToStatus_NoLeakOfErrorText(t *testing.T) {
	t.Parallel()
	secret := errors.New(`pq: password authentication failed for user "admin" DSN=postgres://admin:s3cret@db/app SELECT oid FROM pg_operator`)
	st := MapCatalogErrorToStatus(secret)
	if st != domain.IdentityStatusLookupFailed {
		t.Fatalf("status=%q", st)
	}
	if string(st) != "lookup_failed" {
		t.Fatalf("status string must be bounded enum, got %q", st)
	}
	// Status string itself must not contain secret fragments.
	for _, bad := range []string{"password", "s3cret", "postgres://", "SELECT", "pg_operator"} {
		if strings.Contains(string(st), bad) {
			t.Errorf("status leaked %q", bad)
		}
	}
	code, ok := domain.ReasonForIdentityStatus(st)
	if !ok || code != domain.ReasonIdentityLookupFailed {
		t.Fatalf("reason=%q ok=%v", code, ok)
	}
}

func TestFakeResolver_CancellationAndFactsOnly(t *testing.T) {
	t.Parallel()
	fake := &fakeEffectIdentityResolver{
		byOrd: map[int]EffectIdentityItem{
			0: {
				Ordinal: 0,
				Status:  domain.IdentityStatusResolved,
				Facts: &EffectIdentityFacts{
					Kind:               EffectCandidateOperator,
					ObjectOID:          96,
					NamespaceOID:       11,
					OperandTypeOIDs:    []uint32{23, 23},
					ResultTypeOID:      16,
					ImplementationOID:  65,
					Volatility:         EffectVolatilityImmutable,
					CanonicalSignature: "pg_catalog.=(int4,int4)",
				},
			},
			1: {Ordinal: 1, Status: domain.IdentityStatusUnknown},
			2: {Ordinal: 2, Status: domain.IdentityStatusCoercionGap},
		},
	}

	// Contract: no Trusted field on facts or items.
	for _, typ := range []reflect.Type{
		reflect.TypeOf(EffectIdentityFacts{}),
		reflect.TypeOf(EffectIdentityItem{}),
		reflect.TypeOf(EffectIdentityRequest{}),
		reflect.TypeOf(EffectIdentityBatch{}),
	} {
		if _, ok := typ.FieldByName("Trusted"); ok {
			t.Errorf("%s must not have Trusted field", typ.Name())
		}
		if _, ok := typ.FieldByName("Admission"); ok {
			t.Errorf("%s must not have Admission field", typ.Name())
		}
		if _, ok := typ.FieldByName("ReasonText"); ok {
			t.Errorf("%s must not have ReasonText field", typ.Name())
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := fake.ResolveEffectIdentities(ctx, EffectIdentityRequest{Candidates: sampleCandidates()})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}

	batch, err := fake.ResolveEffectIdentities(context.Background(), EffectIdentityRequest{
		Dialect:    "postgresql",
		Candidates: sampleCandidates(),
	})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	batch = CompleteEffectIdentityBatch(EffectIdentityRequest{Candidates: sampleCandidates()}, batch)
	if BatchIsFullyResolved(batch) {
		t.Error("mixed batch must not be fully resolved")
	}
	codes := FailClosedReasonCodes(batch)
	// unknown + coercion_gap (resolved has no fail-closed reason)
	wantCodes := []domain.ReasonCode{domain.ReasonIdentityCoercionGap, domain.ReasonIdentityUnknown}
	if !reflect.DeepEqual(codes, wantCodes) {
		t.Errorf("codes=%v want %v", codes, wantCodes)
	}
	if batch.Items[0].Facts == nil || batch.Items[0].Facts.ObjectOID != 96 {
		t.Errorf("resolved facts missing: %+v", batch.Items[0])
	}
	// Facts must not encode trust.
	blob, _ := json.Marshal(batch.Items[0].Facts)
	raw := string(blob)
	for _, bad := range []string{`"Trusted"`, `"trusted"`, "admission", "severity", "password"} {
		if strings.Contains(raw, bad) {
			t.Errorf("facts JSON leaked %q: %s", bad, raw)
		}
	}
}

func TestRequestCannotInjectCandidatesOrResolver(t *testing.T) {
	t.Parallel()
	rt := reflect.TypeOf(QueryAccessRequest{})
	for _, name := range []string{
		"EffectCandidates", "Trusted", "Trust", "Candidates",
		"EffectIdentityResolver", "IdentityResolver", "IdentityFacts",
	} {
		if _, ok := rt.FieldByName(name); ok {
			t.Errorf("QueryAccessRequest must not allow injection field %q", name)
		}
	}
	// domain.Result must not grow identity fact fields.
	drt := reflect.TypeOf(domain.Result{})
	for _, name := range []string{
		"EffectCandidates", "IdentityFacts", "EffectIdentityBatch", "Trusted", "OID", "ObjectOID",
	} {
		if _, ok := drt.FieldByName(name); ok {
			t.Errorf("domain.Result must not export %q", name)
		}
	}
}

// fakeEffectIdentityResolver is a facts-only test double (no catalog SQL).
type fakeEffectIdentityResolver struct {
	byOrd map[int]EffectIdentityItem
}

func (f *fakeEffectIdentityResolver) ResolveEffectIdentities(ctx context.Context, req EffectIdentityRequest) (EffectIdentityBatch, error) {
	if err := ctx.Err(); err != nil {
		return EffectIdentityBatch{}, err
	}
	if err := ValidateEffectIdentityRequest(req); err != nil {
		return EffectIdentityBatch{}, err
	}
	items := make([]EffectIdentityItem, 0, len(req.Candidates))
	for _, c := range req.Candidates {
		if it, ok := f.byOrd[c.Ordinal]; ok {
			it.Ordinal = c.Ordinal
			items = append(items, it)
			continue
		}
		items = append(items, EffectIdentityItem{
			Ordinal: c.Ordinal,
			Status:  domain.IdentityStatusUnknown,
		})
	}
	return NormalizeEffectIdentityBatch(items), nil
}

var _ EffectIdentityResolver = (*fakeEffectIdentityResolver)(nil)

func TestValidateFactOperandTypeBinding_NilTypeMap(t *testing.T) {
	t.Parallel()
	batch := EffectIdentityBatch{
		Items: []EffectIdentityItem{
			{
				Ordinal: 0,
				Status:  domain.IdentityStatusResolved,
				Facts: &EffectIdentityFacts{
					Kind:            EffectCandidateOperator,
					ObjectOID:       96,
					OperandTypeOIDs: []uint32{25, 25},
				},
			},
		},
	}
	candidates := []EffectCandidate{
		{Ordinal: 0, Kind: EffectCandidateOperator, Arity: 2},
	}
	result := ValidateFactOperandTypeBinding(batch, nil, candidates)
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result.Items))
	}
	if result.Items[0].Status != domain.IdentityStatusResolved {
		t.Errorf("nil type map should skip: got %v, want resolved", result.Items[0].Status)
	}
}

func TestValidateFactOperandTypeBinding_EmptyTypeMap(t *testing.T) {
	t.Parallel()
	batch := EffectIdentityBatch{
		Items: []EffectIdentityItem{
			{
				Ordinal: 0,
				Status:  domain.IdentityStatusResolved,
				Facts: &EffectIdentityFacts{
					Kind:            EffectCandidateOperator,
					ObjectOID:       96,
					OperandTypeOIDs: []uint32{25, 25},
				},
			},
		},
	}
	candidates := []EffectCandidate{
		{Ordinal: 0, Kind: EffectCandidateOperator, Arity: 2},
	}
	result := ValidateFactOperandTypeBinding(batch, map[int][]uint32{}, candidates)
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result.Items))
	}
	if result.Items[0].Status != domain.IdentityStatusResolved {
		t.Errorf("empty type map should skip: got %v, want resolved", result.Items[0].Status)
	}
}

func TestValidateFactOperandTypeBinding_Match(t *testing.T) {
	t.Parallel()
	batch := EffectIdentityBatch{
		Items: []EffectIdentityItem{
			{
				Ordinal: 0,
				Status:  domain.IdentityStatusResolved,
				Facts: &EffectIdentityFacts{
					Kind:            EffectCandidateOperator,
					ObjectOID:       96,
					OperandTypeOIDs: []uint32{23, 23},
				},
			},
		},
	}
	candidates := []EffectCandidate{
		{Ordinal: 0, Kind: EffectCandidateOperator, Arity: 2},
	}
	typeMap := map[int][]uint32{0: {23, 23}}
	result := ValidateFactOperandTypeBinding(batch, typeMap, candidates)
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result.Items))
	}
	if result.Items[0].Status != domain.IdentityStatusResolved {
		t.Errorf("matching types should pass: got %v, want resolved", result.Items[0].Status)
	}
}

func TestValidateFactOperandTypeBinding_Mismatch(t *testing.T) {
	t.Parallel()
	batch := EffectIdentityBatch{
		Items: []EffectIdentityItem{
			{
				Ordinal: 0,
				Status:  domain.IdentityStatusResolved,
				Facts: &EffectIdentityFacts{
					Kind:            EffectCandidateOperator,
					ObjectOID:       98,
					OperandTypeOIDs: []uint32{25, 25}, // text types
				},
			},
		},
	}
	candidates := []EffectCandidate{
		{Ordinal: 0, Kind: EffectCandidateOperator, Arity: 2},
	}
	typeMap := map[int][]uint32{0: {23, 23}} // int4 types
	result := ValidateFactOperandTypeBinding(batch, typeMap, candidates)
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result.Items))
	}
	if result.Items[0].Status != domain.IdentityStatusLookupFailed {
		t.Errorf("mismatched types should fail: got %v, want lookup_failed", result.Items[0].Status)
	}
	if result.Items[0].Facts != nil {
		t.Errorf("mismatched types should clear facts")
	}
}

func TestValidateFactOperandTypeBinding_MissingOrdinal(t *testing.T) {
	t.Parallel()
	batch := EffectIdentityBatch{
		Items: []EffectIdentityItem{
			{
				Ordinal: 0,
				Status:  domain.IdentityStatusResolved,
				Facts: &EffectIdentityFacts{
					Kind:            EffectCandidateOperator,
					ObjectOID:       96,
					OperandTypeOIDs: []uint32{23, 23},
				},
			},
		},
	}
	candidates := []EffectCandidate{
		{Ordinal: 0, Kind: EffectCandidateOperator, Arity: 2},
	}
	typeMap := map[int][]uint32{999: {23, 23}} // wrong ordinal
	result := ValidateFactOperandTypeBinding(batch, typeMap, candidates)
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result.Items))
	}
	if result.Items[0].Status != domain.IdentityStatusResolved {
		t.Errorf("missing ordinal should skip: got %v, want resolved", result.Items[0].Status)
	}
}

func TestValidateFactOperandTypeBinding_SkipNonOperator(t *testing.T) {
	t.Parallel()
	batch := EffectIdentityBatch{
		Items: []EffectIdentityItem{
			{
				Ordinal: 0,
				Status:  domain.IdentityStatusResolved,
				Facts: &EffectIdentityFacts{
					Kind:            EffectCandidateFunction,
					ObjectOID:       2803,
					OperandTypeOIDs: []uint32{2276},
				},
			},
		},
	}
	candidates := []EffectCandidate{
		{Ordinal: 0, Kind: EffectCandidateFunction, Arity: 0},
	}
	typeMap := map[int][]uint32{0: {999}} // wrong types
	result := ValidateFactOperandTypeBinding(batch, typeMap, candidates)
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result.Items))
	}
	if result.Items[0].Status != domain.IdentityStatusResolved {
		t.Errorf("non-operator should skip: got %v, want resolved", result.Items[0].Status)
	}
}

func TestValidateFactOperandTypeBinding_SkipUnaryOperator(t *testing.T) {
	t.Parallel()
	batch := EffectIdentityBatch{
		Items: []EffectIdentityItem{
			{
				Ordinal: 0,
				Status:  domain.IdentityStatusResolved,
				Facts: &EffectIdentityFacts{
					Kind:            EffectCandidateOperator,
					ObjectOID:       518,
					OperandTypeOIDs: []uint32{23},
				},
			},
		},
	}
	candidates := []EffectCandidate{
		{Ordinal: 0, Kind: EffectCandidateOperator, Arity: 1}, // unary
	}
	typeMap := map[int][]uint32{0: {999}} // wrong types
	result := ValidateFactOperandTypeBinding(batch, typeMap, candidates)
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result.Items))
	}
	if result.Items[0].Status != domain.IdentityStatusResolved {
		t.Errorf("unary operator should skip: got %v, want resolved", result.Items[0].Status)
	}
}
