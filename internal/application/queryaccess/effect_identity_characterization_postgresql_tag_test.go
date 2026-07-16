//go:build postgresql

// Package queryaccess characterizations for pure-read effect identity (T3).
// input: PostgreSQL SQL spanning T2 candidate/rejected ledgers
// output: classification + admission freeze assertions (no production promotion)
// pos: application-level characterization tests for effect identity preconditions
// note: tests only; does not implement resolver, manifest, or proof engine.
package queryaccess

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

// T3 freezes today's PostgreSQL Query Access behavior against the T2 candidate
// and rejected ledgers. Characterization only: no candidate is treated as
// Trusted, and no production effect identity promotion is exercised here.

type effectIdentityFakeResolver struct {
	schemas map[string]RelationSchema
	err     error
}

func (f *effectIdentityFakeResolver) ResolveRelation(_ context.Context, _ string, schema, name string) (RelationSchema, error) {
	if f.err != nil {
		return RelationSchema{}, f.err
	}
	key := name
	if schema != "" {
		key = schema + "." + name
	}
	rs, ok := f.schemas[key]
	if !ok {
		return RelationSchema{}, fmt.Errorf("relation not found: %s", key)
	}
	return rs, nil
}

func effectIdentityUsersResolver() *effectIdentityFakeResolver {
	return &effectIdentityFakeResolver{
		schemas: map[string]RelationSchema{
			"public.users": {
				Schema: "public", Name: "users", Kind: "table",
				Columns: []ColumnSchema{
					{Name: "id", Ordinal: 1},
					{Name: "name", Ordinal: 2},
					{Name: "active", Ordinal: 3},
					{Name: "inactive", Ordinal: 4},
				},
			},
		},
	}
}

func assertIndeterminateClassAndAdmission(t *testing.T, dr domain.Result, label string) {
	t.Helper()
	if dr.ReadClassification != domain.Indeterminate {
		t.Errorf("%s: classification got %q, want %q", label, dr.ReadClassification, domain.Indeterminate)
	}
	if dr.Admission != domain.IndeterminateAdmission {
		t.Errorf("%s: admission got %q, want %q", label, dr.Admission, domain.IndeterminateAdmission)
	}
}

func assertNoLeakOrSeverity(t *testing.T, dr domain.Result, err error, sensitive ...string) {
	t.Helper()
	blobs := []string{fmt.Sprintf("%+v", dr)}
	if err != nil {
		blobs = append(blobs, err.Error())
	}
	for _, blob := range blobs {
		if strings.Contains(blob, "severity") {
			t.Errorf("result/error must not contain severity; got substring in %q", blob)
		}
		for _, s := range sensitive {
			if s == "" {
				continue
			}
			if strings.Contains(blob, s) {
				t.Errorf("result/error must not leak %q", s)
			}
		}
	}
}

func analyzePG(t *testing.T, sql string, resolver SchemaResolver) domain.Result {
	t.Helper()
	svc := &Service{}
	res, err := svc.Analyze(context.Background(), QueryAccessRequest{
		SQL:            sql,
		Dialect:        "postgresql",
		Mode:           "strict",
		DefaultSchema:  "public",
		SchemaResolver: resolver,
	})
	if err != nil {
		t.Fatalf("analyze %q: %v", sql, err)
	}
	return res.DomainResult
}

// TestEffectIdentity_StructuralBoolExpr_NotCatalogIdentityProof locks the T2
// rule that BoolExpr AND/OR/NOT is structural AST control flow, not a catalog
// operator/function identity. Pure column BoolExpr does not alone force
// indeterminate classification today; that is not OID-based trust.
func TestEffectIdentity_StructuralBoolExpr_NotCatalogIdentityProof(t *testing.T) {
	t.Parallel()

	structural := []struct {
		name string
		sql  string
	}{
		{name: "column_only", sql: "SELECT id FROM users WHERE active"},
		{name: "and", sql: "SELECT id FROM users WHERE active AND inactive"},
		{name: "or", sql: "SELECT id FROM users WHERE active OR inactive"},
		{name: "not", sql: "SELECT id FROM users WHERE NOT active"},
		{name: "nested", sql: "SELECT id FROM users WHERE NOT (active OR inactive)"},
	}

	for _, tc := range structural {
		tc := tc
		t.Run("no_resolver/"+tc.name, func(t *testing.T) {
			t.Parallel()
			res, err := AnalyzePostgreSQL(context.Background(), QueryAccessRequest{
				SQL: tc.sql, Dialect: "postgresql", Mode: "strict",
			})
			if err != nil {
				t.Fatalf("analyze: %v", err)
			}
			dr := res.DomainResult
			// Structural BoolExpr is not A_Expr/TypeCast/FuncCall → not effect presence.
			if dr.ReadClassification != domain.ReadOnly {
				t.Errorf("classification: got %q, want %q (BoolExpr is structural, not catalog identity)",
					dr.ReadClassification, domain.ReadOnly)
			}
			// AnalyzePostgreSQL still seeds admission indeterminate (PG hard-stop path).
			if dr.Admission != domain.IndeterminateAdmission {
				t.Errorf("admission: got %q, want %q without resolver lift",
					dr.Admission, domain.IndeterminateAdmission)
			}
			assertNoLeakOrSeverity(t, dr, nil, "SELECT", "active AND", "password", "postgres://")
		})

		t.Run("with_resolver/"+tc.name, func(t *testing.T) {
			t.Parallel()
			dr := analyzePG(t, tc.sql, effectIdentityUsersResolver())
			// Unqualified relations fail closed even with a resolver (S1 barrier).
			if dr.ReadClassification != domain.Indeterminate {
				t.Errorf("classification: got %q, want %q (unqualified barrier)",
					dr.ReadClassification, domain.Indeterminate)
			}
			if dr.Admission != domain.IndeterminateAdmission {
				t.Errorf("admission: got %q, want %q (unqualified barrier)",
					dr.Admission, domain.IndeterminateAdmission)
			}
			assertNoLeakOrSeverity(t, dr, nil, "password", "credential", "postgres://", "severity")
		})
	}

	// Structural BoolExpr wrapping a comparison candidate still freezes indeterminate.
	t.Run("and_with_comparison_candidate", func(t *testing.T) {
		t.Parallel()
		for _, resolver := range []SchemaResolver{nil, effectIdentityUsersResolver()} {
			dr := analyzePG(t, "SELECT id FROM users WHERE id = 1 AND active", resolver)
			assertIndeterminateClassAndAdmission(t, dr, "structural AND + comparison")
		}
	})
}

// TestEffectIdentity_CandidateComparisonMatrix_StaysIndeterminate freezes the
// T2 closed comparison operator set as unpromoted candidates: same-type
// representative forms for =, <>, <, >, <=, >= remain indeterminate until a
// future identity resolver + manifest + proof engine exist.
func TestEffectIdentity_CandidateComparisonMatrix_StaysIndeterminate(t *testing.T) {
	t.Parallel()

	// Representative cases for the six comparison spellings. Full 54-op × type
	// matrix is a future positive corpus under resolver+manifest; T3 only freezes
	// that presence of these candidates does not promote admission today.
	cases := []struct {
		name string
		sql  string
	}{
		{name: "eq_int", sql: "SELECT id FROM users WHERE id = 1"},
		{name: "ne_int", sql: "SELECT id FROM users WHERE id <> 1"},
		{name: "lt_int", sql: "SELECT id FROM users WHERE id < 1"},
		{name: "gt_int", sql: "SELECT id FROM users WHERE id > 1"},
		{name: "le_int", sql: "SELECT id FROM users WHERE id <= 1"},
		{name: "ge_int", sql: "SELECT id FROM users WHERE id >= 1"},
		{name: "eq_text", sql: "SELECT id FROM users WHERE name = 'alice'"},
		{name: "ne_text", sql: "SELECT id FROM users WHERE name <> 'alice'"},
		{name: "eq_bool", sql: "SELECT id FROM users WHERE active = true"},
		{name: "schema_qualified_eq", sql: "SELECT id FROM users WHERE id OPERATOR(pg_catalog.=) 1"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name+"/no_resolver", func(t *testing.T) {
			t.Parallel()
			res, err := AnalyzePostgreSQL(context.Background(), QueryAccessRequest{
				SQL: tc.sql, Dialect: "postgresql", Mode: "strict",
			})
			if err != nil {
				t.Fatalf("analyze: %v", err)
			}
			assertIndeterminateClassAndAdmission(t, res.DomainResult, tc.name)
			assertNoLeakOrSeverity(t, res.DomainResult, nil, "alice", "password", "severity")
		})
		t.Run(tc.name+"/with_resolver", func(t *testing.T) {
			t.Parallel()
			dr := analyzePG(t, tc.sql, effectIdentityUsersResolver())
			assertIndeterminateClassAndAdmission(t, dr, tc.name+" with relation resolver")
			// Relation SchemaResolver alone must not promote effect-bearing PG queries.
			assertNoLeakOrSeverity(t, dr, nil, "alice", "password", "severity")
		})
	}
}

// TestEffectIdentity_CountCandidates_StaysIndeterminate freezes count(*) and
// count(column) as T2 candidates that remain unproven without manifest+proof.
func TestEffectIdentity_CountCandidates_StaysIndeterminate(t *testing.T) {
	t.Parallel()

	cases := []string{
		"SELECT COUNT(*) FROM users",
		"SELECT COUNT(id) FROM users",
		"SELECT count(*) FROM users",
		"SELECT count(id) FROM users",
	}
	for _, sql := range cases {
		sql := sql
		t.Run(sql, func(t *testing.T) {
			t.Parallel()
			for _, resolver := range []SchemaResolver{nil, effectIdentityUsersResolver()} {
				dr := analyzePG(t, sql, resolver)
				assertIndeterminateClassAndAdmission(t, dr, sql)
			}
		})
	}
}

// TestEffectIdentity_TypeResolutionRequired_StaysIndeterminate freezes forms
// where unique operator OID cannot be guessed without type resolution / coercion.
func TestEffectIdentity_TypeResolutionRequired_StaysIndeterminate(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		sql  string
	}{
		{name: "int_eq_text_literal", sql: "SELECT id FROM users WHERE id = 'x'"},
		{name: "int_eq_float_literal", sql: "SELECT id FROM users WHERE id = 1.5"},
		{name: "cross_looking_compare", sql: "SELECT id FROM users WHERE id = name"},
		{name: "numeric_text_mix", sql: "SELECT id FROM users WHERE name > 1"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dr := analyzePG(t, tc.sql, effectIdentityUsersResolver())
			assertIndeterminateClassAndAdmission(t, dr, tc.name)
			assertNoLeakOrSeverity(t, dr, nil, "SELECT id", "password")
		})
	}
}

// TestEffectIdentity_RejectedLedger_StaysIndeterminate freezes the T2 rejected
// ledger: hidden-context readers, UDF-looking effects, non-pg_catalog schema
// effects, and function-backed cast shapes remain indeterminate.
func TestEffectIdentity_RejectedLedger_StaysIndeterminate(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		sql  string
	}{
		// Session/GUC and catalog helpers
		{name: "current_setting", sql: "SELECT current_setting('search_path')"},
		{name: "set_config", sql: "SELECT set_config('application_name', 'x', true)"},
		{name: "pg_get_userbyid", sql: "SELECT pg_get_userbyid(10)"},
		{name: "pg_get_indexdef", sql: "SELECT pg_get_indexdef(1)"},
		// File-reader style
		{name: "pg_read_file", sql: "SELECT pg_read_file('/tmp/x')"},
		{name: "pg_ls_dir", sql: "SELECT pg_ls_dir('/tmp')"},
		// User-defined-looking / non-pg_catalog schema
		{name: "udf_like", sql: "SELECT my_udf(id) FROM users"},
		{name: "schema_qualified_app", sql: "SELECT app.my_func(id) FROM users"},
		{name: "schema_qualified_public", sql: "SELECT public.custom_op(id) FROM users"},
		// Function-backed / cast shapes (phase-1 omit casts)
		{name: "type_cast_text", sql: "SELECT id::text FROM users"},
		{name: "type_cast_int", sql: "SELECT name::integer FROM users"},
		{name: "cast_function_form", sql: "SELECT CAST(id AS text) FROM users"},
		// Non-manifest catalog-looking builtins (stable-class shape)
		{name: "now", sql: "SELECT now()"},
		{name: "version", sql: "SELECT version()"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, resolver := range []SchemaResolver{nil, effectIdentityUsersResolver()} {
				dr := analyzePG(t, tc.sql, resolver)
				assertIndeterminateClassAndAdmission(t, dr, tc.name)
			}
			// Sensitive path/literal fragments must not appear in structured result dump.
			dr := analyzePG(t, tc.sql, effectIdentityUsersResolver())
			assertNoLeakOrSeverity(t, dr, nil,
				"/tmp/x", "/tmp", "search_path", "application_name",
				"password", "credential", "postgres://", "severity",
			)
		})
	}
}

// TestEffectIdentity_UnknownCoercionMultiMatchShapes_StaysIndeterminate documents
// fail-closed posture for unknown/coercion/multi-match morphologies. Today any
// A_Expr/FuncCall/TypeCast presence already freezes indeterminate; later proof
// engines must keep these fail-closed when identity is non-unique.
func TestEffectIdentity_UnknownCoercionMultiMatchShapes_StaysIndeterminate(t *testing.T) {
	t.Parallel()

	cases := []string{
		"SELECT id FROM users WHERE id = $1", // param / unknown type
		"SELECT id FROM users WHERE id = NULL",
		"SELECT id FROM users WHERE id IS NOT DISTINCT FROM 1",
		"SELECT (id || name) FROM users", // operator requiring type resolution
	}
	for _, sql := range cases {
		sql := sql
		t.Run(sql, func(t *testing.T) {
			t.Parallel()
			dr := analyzePG(t, sql, effectIdentityUsersResolver())
			assertIndeterminateClassAndAdmission(t, dr, sql)
		})
	}
}

// TestEffectIdentity_NoResolverResolverErrorIncompleteMetadata freezes admission
// for effect-bearing and incomplete-metadata paths.
func TestEffectIdentity_NoResolverResolverErrorIncompleteMetadata(t *testing.T) {
	t.Parallel()

	effectSQL := "SELECT id FROM users WHERE id = 1"

	t.Run("no_resolver", func(t *testing.T) {
		t.Parallel()
		res, err := AnalyzePostgreSQL(context.Background(), QueryAccessRequest{
			SQL: effectSQL, Dialect: "postgresql", Mode: "strict",
		})
		if err != nil {
			t.Fatalf("analyze: %v", err)
		}
		assertIndeterminateClassAndAdmission(t, res.DomainResult, "no resolver")
	})

	t.Run("resolver_error", func(t *testing.T) {
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
			// Analyze should not surface driver SQL; if it does, fail leak check.
			assertNoLeakOrSeverity(t, domain.Result{}, err, secret, "s3cretPass", "pg_operator", "SELECT oid")
			t.Fatalf("analyze returned error: %v", err)
		}
		assertIndeterminateClassAndAdmission(t, res.DomainResult, "resolver error")
		assertNoLeakOrSeverity(t, res.DomainResult, nil,
			secret, "s3cretPass", "db.internal", "pg_operator",
			"SELECT oid FROM pg_operator", "SELECT id FROM users", "severity",
		)
	})

	t.Run("metadata_incomplete_missing_relation", func(t *testing.T) {
		t.Parallel()
		// Empty catalog: relation not found → unresolved → indeterminate admission.
		resolver := &effectIdentityFakeResolver{schemas: map[string]RelationSchema{}}
		dr := analyzePG(t, effectSQL, resolver)
		assertIndeterminateClassAndAdmission(t, dr, "missing relation")
		if len(dr.Unresolved) == 0 {
			t.Error("expected unresolved entries when relation metadata missing")
		}
		assertNoLeakOrSeverity(t, dr, nil, "SELECT id FROM users WHERE id = 1", "severity")
	})

	t.Run("metadata_incomplete_missing_column", func(t *testing.T) {
		t.Parallel()
		resolver := &effectIdentityFakeResolver{
			schemas: map[string]RelationSchema{
				"public.users": {
					Schema: "public", Name: "users", Kind: "table",
					// id missing from metadata
					Columns: []ColumnSchema{{Name: "name", Ordinal: 1}},
				},
			},
		}
		dr := analyzePG(t, effectSQL, resolver)
		// Effect presence still keeps classification indeterminate; incomplete
		// column metadata must not promote.
		assertIndeterminateClassAndAdmission(t, dr, "missing column metadata")
	})
}

// TestEffectIdentity_ReclassifyAfterResolution_NeverLiftsPostgreSQL locks the
// foundation hard-stop used as a safety precondition for later proof work.
func TestEffectIdentity_ReclassifyAfterResolution_NeverLiftsPostgreSQL(t *testing.T) {
	t.Parallel()

	got := reclassifyAfterResolution(
		domain.Indeterminate,
		nil,
		nil,
		true, // hasResolver
		"postgresql",
		nil, // no proof
	)
	if got != domain.Indeterminate {
		t.Errorf("reclassifyAfterResolution(postgresql): got %q, want %q", got, domain.Indeterminate)
	}

	// Even with empty unresolved and empty reason codes, PG must not lift without proof.
	got = reclassifyAfterResolution(
		domain.Indeterminate,
		[]domain.ReasonCode{},
		[]domain.Unresolved{},
		true,
		"postgresql",
		nil, // no proof
	)
	if got != domain.Indeterminate {
		t.Errorf("reclassifyAfterResolution empty unresolved: got %q, want %q", got, domain.Indeterminate)
	}

	// MySQL may lift when safe; ensure we do not accidentally hard-stop MySQL.
	mysqlLifted := reclassifyAfterResolution(
		domain.Indeterminate,
		nil,
		nil,
		true,
		"mysql",
		nil, // no proof (MySQL doesn't use it)
	)
	if mysqlLifted != domain.ReadOnly {
		t.Errorf("reclassifyAfterResolution(mysql) safe lift: got %q, want %q", mysqlLifted, domain.ReadOnly)
	}
}

// TestEffectIdentity_ServiceAnalyze_CandidateNeverAdmissible is an end-to-end
// freeze: Service.Analyze must not return admissible for T2 candidate effects
// on PostgreSQL even when a relation SchemaResolver is present.
func TestEffectIdentity_ServiceAnalyze_CandidateNeverAdmissible(t *testing.T) {
	t.Parallel()

	candidates := []string{
		"SELECT id FROM users WHERE id = 1",
		"SELECT id FROM users WHERE id >= 1",
		"SELECT COUNT(*) FROM users",
		"SELECT COUNT(id) FROM users",
		"SELECT id::text FROM users",
		"SELECT current_setting('search_path')",
	}
	for _, sql := range candidates {
		sql := sql
		t.Run(sql, func(t *testing.T) {
			t.Parallel()
			dr := analyzePG(t, sql, effectIdentityUsersResolver())
			if dr.Admission == domain.Admissible {
				t.Fatalf("candidate/rejected form must not be admissible yet; sql characterization failed")
			}
			if dr.Admission != domain.IndeterminateAdmission {
				t.Errorf("admission: got %q, want %q", dr.Admission, domain.IndeterminateAdmission)
			}
			if dr.ReadClassification == domain.ReadOnly {
				t.Errorf("classification must not be read_only for unproven effect form; got %q", dr.ReadClassification)
			}
		})
	}
}
