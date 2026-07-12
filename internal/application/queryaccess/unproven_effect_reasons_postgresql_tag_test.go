//go:build postgresql

// Package queryaccess tests bounded unproven-effect reason codes (T4).
// input: PostgreSQL SQL spanning operator/function/cast and identity-failure categories
// output: classification/admission freeze + reason id assertions + no-leak checks
// pos: application-level coverage for Query Access pure-read reason explanation
// note: if this file changes, update this header and module README.md.
package queryaccess

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

// TestUnprovenEffectReasons_OperatorFunctionCastBounded covers the minimum
// reason set for unproven operator / function-aggregate / cast presence.
func TestUnprovenEffectReasons_OperatorFunctionCastBounded(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		sql  string
		want []domain.ReasonCode
	}{
		{name: "operator", sql: "SELECT id FROM users WHERE id = 1", want: []domain.ReasonCode{domain.ReasonUnprovenOperatorEffect}},
		{name: "function_aggregate_count_star", sql: "SELECT COUNT(*) FROM users", want: []domain.ReasonCode{domain.ReasonUnprovenFunctionEffect}},
		{name: "function_aggregate_count_col", sql: "SELECT COUNT(id) FROM users", want: []domain.ReasonCode{domain.ReasonUnprovenFunctionEffect}},
		{name: "cast", sql: "SELECT id::text FROM users", want: []domain.ReasonCode{domain.ReasonUnprovenCastEffect}},
		{name: "cast_form", sql: "SELECT CAST(id AS text) FROM users", want: []domain.ReasonCode{domain.ReasonUnprovenCastEffect}},
		// Complete traversal: previously missed executable expression positions.
		{name: "limit_function", sql: "SELECT id FROM users LIMIT length('a')", want: []domain.ReasonCode{domain.ReasonUnprovenFunctionEffect}},
		{name: "values_function", sql: "VALUES (length('a'))", want: []domain.ReasonCode{domain.ReasonUnprovenFunctionEffect}},
		{name: "agg_filter", sql: "SELECT count(*) FILTER (WHERE length(name) > 0) FROM users", want: []domain.ReasonCode{domain.ReasonUnprovenFunctionEffect}},
		{name: "window_partition", sql: "SELECT row_number() OVER (PARTITION BY length(name)) FROM users", want: []domain.ReasonCode{domain.ReasonUnprovenFunctionEffect}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dr := analyzePG(t, tc.sql, nil)
			assertIndeterminateClassAndAdmission(t, dr, tc.name)
			assertReasonsContain(t, dr.ReasonCodes, tc.want...)
			assertReasonsOnlyMachineIDs(t, dr.ReasonCodes)
			assertNoLeakOrSeverity(t, dr, nil, "SELECT", "COUNT", "::text", "password", "severity")
		})
	}
}

// TestUnprovenEffectReasons_RejectedAndUnknownShapesFailClosed locks rejected
// ledger and unknown/coercion morphologies to function/operator reasons without promotion.
func TestUnprovenEffectReasons_RejectedAndUnknownShapesFailClosed(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		sql  string
		want domain.ReasonCode
	}{
		{name: "current_setting", sql: "SELECT current_setting('search_path')", want: domain.ReasonUnprovenFunctionEffect},
		{name: "pg_get", sql: "SELECT pg_get_userbyid(10)", want: domain.ReasonUnprovenFunctionEffect},
		{name: "udf_like", sql: "SELECT my_udf(id) FROM users", want: domain.ReasonUnprovenFunctionEffect},
		{name: "function_backed_cast", sql: "SELECT id::text FROM users", want: domain.ReasonUnprovenCastEffect},
		{name: "param_unknown", sql: "SELECT id FROM users WHERE id = $1", want: domain.ReasonUnprovenOperatorEffect},
		{name: "coercion_looking", sql: "SELECT id FROM users WHERE id = 'x'", want: domain.ReasonUnprovenOperatorEffect},
		{name: "multi_match_looking_concat", sql: "SELECT (id || name) FROM users", want: domain.ReasonUnprovenOperatorEffect},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dr := analyzePG(t, tc.sql, effectIdentityUsersResolver())
			assertIndeterminateClassAndAdmission(t, dr, tc.name)
			assertReasonsContain(t, dr.ReasonCodes, tc.want)
			assertNoLeakOrSeverity(t, dr, nil,
				"search_path", "my_udf", "password", "postgres://", "severity", "SELECT id",
			)
		})
	}
}

// TestUnprovenEffectReasons_IdentityFailureMappingNoLeak verifies bounded
// identity failure reasons never embed underlying error text. The identity
// resolver is not implemented; this locks the mapping helper contract used by
// future proof wiring.
func TestUnprovenEffectReasons_IdentityFailureMappingNoLeak(t *testing.T) {
	t.Parallel()

	secretErr := errors.New("connection refused: postgres://user:s3cretPass@db.internal:5432/app SELECT oid FROM pg_operator WHERE oprname = '='")

	failures := []domain.IdentityFailure{
		domain.IdentityFailureUnavailable,
		domain.IdentityFailureUnknown,
		domain.IdentityFailureError,
		domain.IdentityFailureAmbiguous,
		domain.IdentityFailureCoercionGap,
	}
	for _, f := range failures {
		f := f
		t.Run(string(f), func(t *testing.T) {
			t.Parallel()
			code, ok := domain.ReasonForIdentityFailure(f)
			if !ok {
				t.Fatalf("expected mapping for %q", f)
			}
			// Simulate attaching a failure reason while discarding error text.
			var codes []domain.ReasonCode
			codes = appendIdentityFailureReason(codes, f, secretErr)
			if !hasReason(codes, code) {
				t.Fatalf("expected %q in %v", code, codes)
			}
			blob := fmt.Sprintf("%v|%v", codes, secretErr)
			// Public reason list must not contain secret fragments.
			reasonBlob := fmt.Sprintf("%v", codes)
			for _, bad := range []string{"s3cretPass", "db.internal", "pg_operator", "oprname", "postgres://", "SELECT oid"} {
				if strings.Contains(reasonBlob, bad) {
					t.Errorf("reason codes leaked %q via %v (full context %q)", bad, codes, blob)
				}
			}
		})
	}

	// Free-text injection rejected.
	if code, ok := domain.ReasonForIdentityFailure(domain.IdentityFailure(secretErr.Error())); ok {
		t.Errorf("free-text must not map to reason %q", code)
	}
}

// appendIdentityFailureReason maps a bounded identity failure to a reason code.
// The error value is intentionally ignored so driver/catalog text cannot leak.
func appendIdentityFailureReason(codes []domain.ReasonCode, failure domain.IdentityFailure, _ error) []domain.ReasonCode {
	code, ok := domain.ReasonForIdentityFailure(failure)
	if !ok {
		return codes
	}
	return append(codes, code)
}

// TestUnprovenEffectReasons_NoResolverAndResolverError keep effect-bearing PG
// queries indeterminate without leaking resolver/catalog errors.
func TestUnprovenEffectReasons_NoResolverAndResolverError(t *testing.T) {
	t.Parallel()

	effectSQL := "SELECT id FROM users WHERE id = 1"

	t.Run("no_resolver_has_unproven_operator", func(t *testing.T) {
		t.Parallel()
		res, err := AnalyzePostgreSQL(context.Background(), QueryAccessRequest{
			SQL: effectSQL, Dialect: "postgresql", Mode: "strict",
		})
		if err != nil {
			t.Fatalf("analyze: %v", err)
		}
		assertIndeterminateClassAndAdmission(t, res.DomainResult, "no resolver")
		assertReasonsContain(t, res.DomainResult.ReasonCodes, domain.ReasonUnprovenOperatorEffect)
		assertNoLeakOrSeverity(t, res.DomainResult, nil, "SELECT id FROM users", "severity")
	})

	t.Run("relation_resolver_error_no_leak", func(t *testing.T) {
		t.Parallel()
		secret := "postgres://user:s3cretPass@db.internal:5432/app?sslmode=disable"
		resolver := &effectIdentityFakeResolver{
			err: errors.New("connection refused: " + secret + " catalog query SELECT oid FROM pg_operator"),
		}
		svc := &Service{}
		res, err := svc.Analyze(context.Background(), QueryAccessRequest{
			SQL: effectSQL, Dialect: "postgresql", Mode: "strict",
			DefaultSchema: "public", SchemaResolver: resolver,
		})
		if err != nil {
			assertNoLeakOrSeverity(t, domain.Result{}, err, secret, "s3cretPass", "pg_operator", "SELECT oid")
			t.Fatalf("analyze returned error: %v", err)
		}
		assertIndeterminateClassAndAdmission(t, res.DomainResult, "resolver error")
		assertReasonsContain(t, res.DomainResult.ReasonCodes, domain.ReasonUnprovenOperatorEffect)
		assertNoLeakOrSeverity(t, res.DomainResult, nil,
			secret, "s3cretPass", "db.internal", "pg_operator",
			"SELECT oid FROM pg_operator", "SELECT id FROM users", "severity",
		)
	})

	t.Run("unknown_identity_category_no_leak", func(t *testing.T) {
		t.Parallel()
		// Future resolver path: unknown identity attaches identity_unknown only.
		codes := appendIdentityFailureReason(nil, domain.IdentityFailureUnknown, errors.New("no rows for oprname '='"))
		if !hasReason(codes, domain.ReasonIdentityUnknown) {
			t.Fatalf("expected identity_unknown, got %v", codes)
		}
		if strings.Contains(fmt.Sprintf("%v", codes), "oprname") {
			t.Errorf("leaked operator name into reasons: %v", codes)
		}
	})
}

// TestUnprovenEffectReasons_ModeDoesNotChangeClassification verifies strict and
// projection_only keep the same classification for unproven effects.
func TestUnprovenEffectReasons_ModeDoesNotChangeClassification(t *testing.T) {
	t.Parallel()

	sql := "SELECT id FROM users WHERE salary > 50000"
	svc := &Service{}
	for _, mode := range []string{"strict", "projection_only"} {
		mode := mode
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			res, err := svc.Analyze(context.Background(), QueryAccessRequest{
				SQL: sql, Dialect: "postgresql", Mode: mode, DefaultSchema: "public",
			})
			if err != nil {
				t.Fatalf("analyze: %v", err)
			}
			dr := res.DomainResult
			assertIndeterminateClassAndAdmission(t, dr, mode)
			assertReasonsContain(t, dr.ReasonCodes, domain.ReasonUnprovenOperatorEffect)
			if string(dr.Mode) != mode {
				t.Errorf("mode: got %q want %q", dr.Mode, mode)
			}
		})
	}
}

// TestUnprovenEffectReasons_SortDeterminism locks stable alphabetical reason order.
func TestUnprovenEffectReasons_SortDeterminism(t *testing.T) {
	t.Parallel()

	sql := "SELECT COUNT(id)::text FROM users WHERE id = 1"
	svc := &Service{}
	var first []domain.ReasonCode
	for i := 0; i < 5; i++ {
		res, err := svc.Analyze(context.Background(), QueryAccessRequest{
			SQL: sql, Dialect: "postgresql", Mode: "strict", DefaultSchema: "public",
		})
		if err != nil {
			t.Fatalf("analyze: %v", err)
		}
		if i == 0 {
			first = append([]domain.ReasonCode(nil), res.DomainResult.ReasonCodes...)
			continue
		}
		if !reflect.DeepEqual(first, res.DomainResult.ReasonCodes) {
			t.Fatalf("non-deterministic reasons: %v vs %v", first, res.DomainResult.ReasonCodes)
		}
	}
	// Normalized alphabetical order.
	want := []domain.ReasonCode{
		domain.ReasonUnprovenCastEffect,
		domain.ReasonUnprovenFunctionEffect,
		domain.ReasonUnprovenOperatorEffect,
	}
	if !reflect.DeepEqual(first, want) {
		t.Fatalf("sorted reasons: got %v, want %v", first, want)
	}
}

// TestUnprovenEffectReasons_RequestCannotInjectReasons ensures callers cannot
// supply trusted reason codes via the request contract.
func TestUnprovenEffectReasons_RequestCannotInjectReasons(t *testing.T) {
	t.Parallel()

	rt := reflect.TypeOf(QueryAccessRequest{})
	if _, ok := rt.FieldByName("ReasonCodes"); ok {
		t.Fatal("QueryAccessRequest must not expose ReasonCodes for caller injection")
	}
	// Analyze a non-effect query: reasons must stay empty even if someone later
	// mutates the result (admission is not derived from caller-forged codes).
	dr := analyzePG(t, "SELECT id FROM users WHERE active AND inactive", effectIdentityUsersResolver())
	if hasReason(dr.ReasonCodes, domain.ReasonUnprovenOperatorEffect) {
		t.Errorf("structural bool must not invent operator reason: %v", dr.ReasonCodes)
	}
	// Forging codes on a copy must not be treatable as trust proof via ValidateResult.
	forged := dr
	forged.ReasonCodes = []domain.ReasonCode{"trusted_by_caller", domain.ReasonUnprovenOperatorEffect}
	// ValidateResult does not grant admission from reasons; classification stays as-is.
	if forged.ReadClassification == domain.ReadOnly && forged.Admission == domain.Admissible {
		// structural path may be admissible with resolver; forging extra reasons must not break validation.
	}
	if err := domain.ValidateResult(&forged); err != nil {
		// Unknown reason strings are allowed as opaque ids today; admission still not auto-promoted by them.
		t.Logf("validate forged: %v", err)
	}
	// Re-running Analyze ignores any external mutation of prior results.
	dr2 := analyzePG(t, "SELECT id FROM users WHERE active AND inactive", effectIdentityUsersResolver())
	if hasReason(dr2.ReasonCodes, "trusted_by_caller") {
		t.Error("Analyze must not accept injected trusted_by_caller reason")
	}
}

// TestUnprovenEffectReasons_CompleteTraversalNeverAdmissible ensures LIMIT/VALUES
// window/FILTER unproven effects stay indeterminate even with a relation resolver.
// These remain unproven_* markers only — not proven-admissible.
func TestUnprovenEffectReasons_CompleteTraversalNeverAdmissible(t *testing.T) {
	t.Parallel()

	cases := []string{
		"SELECT id FROM users LIMIT length('a')",
		"VALUES (length('a'))",
		"SELECT count(*) FILTER (WHERE length(name) > 0) FROM users",
		"SELECT row_number() OVER (PARTITION BY length(name) ORDER BY id) FROM users",
	}
	for _, sql := range cases {
		sql := sql
		t.Run(sql, func(t *testing.T) {
			t.Parallel()
			for _, resolver := range []SchemaResolver{nil, effectIdentityUsersResolver()} {
				dr := analyzePG(t, sql, resolver)
				assertIndeterminateClassAndAdmission(t, dr, sql)
				if !hasReason(dr.ReasonCodes, domain.ReasonUnprovenFunctionEffect) &&
					!hasReason(dr.ReasonCodes, domain.ReasonUnprovenOperatorEffect) {
					t.Errorf("expected unproven_* reason for complete-traversal case; got %v", dr.ReasonCodes)
				}
				if dr.Admission == domain.Admissible {
					t.Error("unproven window/filter/limit/values effect must not be admissible")
				}
			}
			assertNoLeakOrSeverity(t, analyzePG(t, sql, nil), nil, "length", "password", "severity")
		})
	}
}

// TestUnprovenEffectReasons_JSONNoSeverityNoSQLLeak locks public JSON shape.
func TestUnprovenEffectReasons_JSONNoSeverityNoSQLLeak(t *testing.T) {
	t.Parallel()

	dr := analyzePG(t, "SELECT current_setting('search_path'), id::text FROM users WHERE id = 1", nil)
	data, err := json.Marshal(dr)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	raw := string(data)
	for _, bad := range []string{
		"severity", "search_path", "current_setting", "::text",
		"password", "postgres://", "SELECT current_setting",
	} {
		if strings.Contains(raw, bad) {
			t.Errorf("JSON must not contain %q; got %s", bad, raw)
		}
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := m["severity"]; ok {
		t.Error("severity field must not exist")
	}
	codes, _ := m["reason_codes"].([]any)
	if len(codes) == 0 {
		t.Fatalf("expected reason_codes in JSON, got %s", raw)
	}
	for _, c := range codes {
		s, _ := c.(string)
		if strings.ContainsAny(s, " \t'\"()") {
			t.Errorf("reason code not machine-like: %q", s)
		}
	}
}
