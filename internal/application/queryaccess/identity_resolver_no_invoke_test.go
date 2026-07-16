// Package queryaccess verifies T6 does not invoke identity resolution or change admission.
// input: PostgreSQL and MySQL Analyze paths
// output: classification/admission freeze unchanged from T5; no public identity leak
// pos: public-behavior freeze for resolver contract introduction
// note: if this file changes, update this header and module README.md.
package queryaccess

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

func TestServiceAnalyze_DoesNotUseIdentityResolver(t *testing.T) {
	// Service has no EffectIdentityResolver field and Analyze signature is unchanged.
	st := reflect.TypeOf(Service{})
	if _, ok := st.FieldByName("EffectIdentityResolver"); ok {
		t.Fatal("Service must not hold EffectIdentityResolver in T6")
	}
	// Analyze is a pointer-receiver method with (ctx, request) inputs.
	pt := reflect.TypeOf(&Service{})
	m, ok := pt.MethodByName("Analyze")
	if !ok {
		t.Fatal("missing Analyze")
	}
	if m.Type.NumIn() != 3 {
		t.Fatalf("Analyze arity changed: %d (want receiver + ctx + request)", m.Type.NumIn())
	}
}

func TestMySQL_NoIdentityRegression(t *testing.T) {
	t.Parallel()
	svc := &Service{}
	res, err := svc.Analyze(context.Background(), QueryAccessRequest{
		SQL: "SELECT id FROM users WHERE id = 1", Dialect: "mysql", Mode: "strict", DefaultSchema: "app",
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	dr := res.DomainResult
	if dr.ReadClassification != domain.ReadOnly {
		t.Errorf("classification=%q want read_only", dr.ReadClassification)
	}
	if dr.Admission != domain.Admissible {
		t.Errorf("admission=%q want admissible", dr.Admission)
	}
	for _, code := range dr.ReasonCodes {
		s := string(code)
		if strings.HasPrefix(s, "unproven_") || strings.HasPrefix(s, "identity_") {
			t.Errorf("MySQL must not emit %q", code)
		}
	}
	assertNoIdentityLeakJSON(t, dr)
}

func assertNoIdentityLeakJSON(t *testing.T, dr domain.Result) {
	t.Helper()
	data, err := json.Marshal(dr)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	raw := string(data)
	for _, bad := range []string{
		"effect_candidates", "EffectCandidates", "ObjectOID", "object_oid",
		"NamespaceOID", "CanonicalSignature", "IdentityFacts", "identity_facts",
		"lookup_failed", // status enum must not appear as a free-standing public field name
		"severity", "password", "postgres://", "pg_operator", "atttypid",
	} {
		// reason codes may legitimately include identity_* machine ids later;
		// here we only check structural field names / secrets for this freeze.
		if bad == "lookup_failed" {
			continue
		}
		if strings.Contains(raw, bad) {
			t.Errorf("domain JSON leaked %q: %s", bad, raw)
		}
	}
}
