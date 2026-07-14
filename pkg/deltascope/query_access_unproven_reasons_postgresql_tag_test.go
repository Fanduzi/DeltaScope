//go:build postgresql

// Package deltascope verifies SDK surface passthrough of unproven-effect reason codes.
// input: PostgreSQL SQL via public AnalyzeQueryAccess
// output: reason_codes machine ids, no severity, no SQL leakage, admission freeze
// pos: public API coverage for Query Access T4 reason explanation
// note: if this file changes, update this header and module README.md.
package deltascope

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestAnalyzeQueryAccess_PostgreSQLUnprovenReasons(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		sql  string
		want string
	}{
		{name: "operator", sql: "SELECT id FROM users WHERE id = 1", want: "unproven_operator_effect"},
		{name: "function", sql: "SELECT COUNT(*) FROM users", want: "unproven_function_effect"},
		{name: "cast", sql: "SELECT id::text FROM users", want: "unproven_cast_effect"},
		{name: "limit_function", sql: "SELECT id FROM users LIMIT length('a')", want: "unproven_function_effect"},
		{name: "values_function", sql: "VALUES (length('a'))", want: "unproven_function_effect"},
		{name: "window_partition", sql: "SELECT row_number() OVER (PARTITION BY length(name)) FROM users", want: "unproven_function_effect"},
		{name: "agg_filter", sql: "SELECT count(*) FILTER (WHERE length(name) > 0) FROM users", want: "unproven_function_effect"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result, err := AnalyzeQueryAccess(context.Background(), QueryAccessRequest{
				SQL:     tc.sql,
				Dialect: DialectPostgreSQL,
				Mode:    QueryAccessModeStrict,
			})
			if err != nil {
				t.Fatalf("analyze: %v", err)
			}
			if result.ReadClassification != QueryAccessIndeterminate {
				t.Errorf("classification: got %q, want indeterminate", result.ReadClassification)
			}
			if result.Admission != QueryAccessIndeterminateAdmission {
				t.Errorf("admission: got %q, want indeterminate", result.Admission)
			}
			found := false
			for _, rc := range result.ReasonCodes {
				if rc == tc.want {
					found = true
				}
				if strings.ContainsAny(rc, " \t'\"()") {
					t.Errorf("reason not machine-like: %q", rc)
				}
			}
			if !found {
				t.Errorf("expected reason %q in %v", tc.want, result.ReasonCodes)
			}

			data, err := json.Marshal(result)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			raw := string(data)
			for _, bad := range []string{"severity", "password", "postgres://", "COUNT(*)", "id::text"} {
				if strings.Contains(raw, bad) {
					t.Errorf("JSON leaked %q: %s", bad, raw)
				}
			}
		})
	}
}

func TestAnalyzeQueryAccess_PostgreSQLNoCandidateFieldsInJSON(t *testing.T) {
	t.Parallel()

	result, err := AnalyzeQueryAccess(context.Background(), QueryAccessRequest{
		SQL:     "SELECT COUNT(*) FROM users WHERE id = 1 LIMIT length('a')",
		Dialect: DialectPostgreSQL,
		Mode:    QueryAccessModeStrict,
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	raw := string(data)
	for _, bad := range []string{
		"effect_candidates", "EffectCandidates", "NamePath", "name_path",
		"TargetTypePath", "severity", "COUNT", "length", "password",
		"ObjectOID", "object_oid", "NamespaceOID", "CanonicalSignature",
		"EffectIdentityResolver", "identity_facts", "postgres://",
	} {
		if strings.Contains(raw, bad) {
			t.Errorf("public JSON must not contain %q; got %s", bad, raw)
		}
	}
	// T6: public SDK request still has no identity resolver injection field.
	rt := reflect.TypeOf(QueryAccessRequest{})
	if _, ok := rt.FieldByName("EffectIdentityResolver"); ok {
		t.Error("public QueryAccessRequest must not gain EffectIdentityResolver in T6")
	}
	// Public contract: still indeterminate with unproven reasons only.
	if result.Admission != QueryAccessIndeterminateAdmission {
		t.Errorf("admission: got %q", result.Admission)
	}
	if len(result.ReasonCodes) == 0 {
		t.Error("expected unproven reason codes on public result")
	}
}

func TestAnalyzeQueryAccess_PostgreSQLReasonOrderDeterministic(t *testing.T) {
	t.Parallel()

	sql := "SELECT COUNT(id)::text FROM users WHERE id = 1"
	var first []string
	for i := 0; i < 3; i++ {
		result, err := AnalyzeQueryAccess(context.Background(), QueryAccessRequest{
			SQL: sql, Dialect: DialectPostgreSQL, Mode: QueryAccessModeStrict,
		})
		if err != nil {
			t.Fatalf("analyze: %v", err)
		}
		if i == 0 {
			first = append([]string(nil), result.ReasonCodes...)
			continue
		}
		if len(result.ReasonCodes) != len(first) {
			t.Fatalf("len mismatch %v vs %v", result.ReasonCodes, first)
		}
		for j := range first {
			if result.ReasonCodes[j] != first[j] {
				t.Fatalf("order mismatch %v vs %v", result.ReasonCodes, first)
			}
		}
	}
}

// TestAnalyzeQueryAccess_UnqualifiedPostgreSQL_FailClosed verifies that
// unqualified PostgreSQL base relations fail closed when using the public
// SDK with a SchemaResolver (S1 proof — T11.2).
func TestAnalyzeQueryAccess_UnqualifiedPostgreSQL_FailClosed(t *testing.T) {
	t.Parallel()
	resolver := &mockPublicSchemaResolver{
		schemas: map[string]QueryAccessRelationSchema{
			"public.users": {
				Schema: "public", Name: "users", Kind: "table",
				Columns: []QueryAccessColumnSchema{{Name: "id", Ordinal: 1}},
			},
		},
	}
	result, err := AnalyzeQueryAccess(context.Background(), QueryAccessRequest{
		SQL:            "SELECT id FROM users",
		Dialect:        DialectPostgreSQL,
		Mode:           QueryAccessModeStrict,
		DefaultSchema:  "public",
		SchemaResolver: resolver,
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}

	if result.ReadClassification != QueryAccessIndeterminate {
		t.Errorf("classification: got %q, want %q", result.ReadClassification, QueryAccessIndeterminate)
	}
	if result.Admission != QueryAccessIndeterminateAdmission {
		t.Errorf("admission: got %q, want %q", result.Admission, QueryAccessIndeterminateAdmission)
	}

	hasReason := false
	for _, rc := range result.ReasonCodes {
		if rc == "unqualified_relation_blocked" {
			hasReason = true
			break
		}
	}
	if !hasReason {
		t.Errorf("expected unqualified_relation_blocked in reasons: %v", result.ReasonCodes)
	}

	for _, req := range result.Requirements {
		if req.Privilege == "read_table" || req.Privilege == "read_column" {
			t.Errorf("unexpected physical requirement: %+v", req)
		}
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	raw := string(data)
	for _, bad := range []string{"severity", "password", "postgres://", "ObjectOID", "SessionBinding"} {
		if strings.Contains(raw, bad) {
			t.Errorf("JSON leaked %q: %s", bad, raw)
		}
	}
}

type mockPublicSchemaResolver struct {
	schemas map[string]QueryAccessRelationSchema
}

func (m *mockPublicSchemaResolver) ResolveRelation(_ context.Context, _, schema, name string) (QueryAccessRelationSchema, error) {
	key := schema + "." + name
	if s, ok := m.schemas[key]; ok {
		return s, nil
	}
	return QueryAccessRelationSchema{}, fmt.Errorf("not found: %s", key)
}
