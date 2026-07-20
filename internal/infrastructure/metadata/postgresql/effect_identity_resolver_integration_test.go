//go:build postgresql && integration

// Package postgresqlmeta runs T7 adapter and trusted-service E2E against a live
// PostgreSQL 17 session when available.
// input: DELTASCOPE_PG_* env or docker compose defaults (localhost:5500)
// output: real catalog facts for operators, aggregates, and scalar effects under a pinned session;
//
//	real Service.Analyze promotion for count(*) via NewTrustedService
//
// pos: non-skippable PostgreSQL 17 integration evidence
// note: if this file changes, update this header and module README.md.
package postgresqlmeta

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	appqa "github.com/Fanduzi/DeltaScope/internal/application/queryaccess"
	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

// TestTrustedService_PG17CountStarE2E proves count(*) promotion through the
// real Service.Analyze path: PinnedSession → EffectIdentityAdapter →
// CaptureExecutionBoundContext → manifest proof → admission.
//
// This is the only test that verifies the full trust chain against a live PG17.
func TestTrustedService_PG17CountStarE2E(t *testing.T) {
	db, cleanup, err := openIntegrationDB(t)
	if err != nil {
		t.Fatal("PG17 integration service unavailable at configured endpoint (driver error suppressed)")
	}
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1) Pin a single session.
	session, err := PinSession(ctx, db)
	if err != nil {
		t.Fatalf("PinSession: %v", err)
	}
	defer session.Close()

	// 2) Build controlled resolver from pinned session.
	adapter, err := NewEffectIdentityAdapter(session)
	if err != nil {
		t.Fatalf("adapter: %v", err)
	}

	// 3) Build schema resolver from the same DB.
	schemaResolver := NewQueryAccessResolver(db)

	// 4) Build trust policy with PG17 manifest.
	policy, err := appqa.NewTrustPolicy(appqa.NewPG17Manifest())
	if err != nil {
		t.Fatalf("NewTrustPolicy: %v", err)
	}

	// 5) Create trusted service with real controlled resolver.
	svc, err := appqa.NewTrustedService(adapter, policy, schemaResolver)
	if err != nil {
		t.Fatalf("NewTrustedService: %v", err)
	}

	// 6) Analyze count(*) — this is the real end-to-end promotion path.
	res, err := svc.Analyze(ctx, appqa.QueryAccessRequest{
		SQL:           "SELECT count(*) FROM app.users",
		Dialect:       "postgresql",
		Mode:          "strict",
		DefaultSchema: "app",
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	t.Logf("classification=%s admission=%s reasons=%v",
		res.DomainResult.ReadClassification, res.DomainResult.Admission, res.DomainResult.ReasonCodes)

	// 7) Assert promotion.
	if res.DomainResult.ReadClassification != domain.ReadOnly {
		t.Errorf("classification = %v, want read_only", res.DomainResult.ReadClassification)
	}
	if res.DomainResult.Admission != domain.Admissible {
		t.Errorf("admission = %v, want admissible", res.DomainResult.Admission)
	}

	// 8) Verify no unproven reasons remain after proof.
	for _, code := range res.DomainResult.ReasonCodes {
		if code == domain.ReasonUnprovenFunctionEffect {
			t.Error("unproven_function_effect should be removed after proof")
		}
	}

	// 9) Verify relations extracted correctly.
	foundUsers := false
	for _, rel := range res.DomainResult.Relations {
		if rel.Name == "users" {
			foundUsers = true
			break
		}
	}
	if !foundUsers {
		t.Errorf("expected users relation in result: %+v", res.DomainResult.Relations)
	}
}

// TestTrustedService_PG17JoinComparisonE2E proves JOIN column comparison promotion
// through the real Service.Analyze path against PG17: pinned session →
// EffectIdentityAdapter → ResolveColumnTypeOIDs → manifest proof → admission.
//
// Uses existing app.users (id int8) JOIN app.orders (user_id int8) so the
// int8=int8 comparison operator (OID 20=20) is manifest-proven.
func TestTrustedService_PG17JoinComparisonE2E(t *testing.T) {
	db, cleanup, err := openIntegrationDB(t)
	if err != nil {
		t.Fatal("PG17 integration service unavailable at configured endpoint (driver error suppressed)")
	}
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1) Pin a single session.
	session, err := PinSession(ctx, db)
	if err != nil {
		t.Fatalf("PinSession: %v", err)
	}
	defer session.Close()

	// 2) Build controlled resolver from pinned session.
	adapter, err := NewEffectIdentityAdapter(session)
	if err != nil {
		t.Fatalf("adapter: %v", err)
	}

	// 3) Build schema resolver from the same DB.
	schemaResolver := NewQueryAccessResolver(db)

	// 4) Build trust policy with PG17 manifest.
	policy, err := appqa.NewTrustPolicy(appqa.NewPG17Manifest())
	if err != nil {
		t.Fatalf("NewTrustPolicy: %v", err)
	}

	// 5) Create trusted service with real controlled resolver.
	svc, err := appqa.NewTrustedService(adapter, policy, schemaResolver)
	if err != nil {
		t.Fatalf("NewTrustedService: %v", err)
	}

	// 6) Analyze JOIN query — this exercises column comparison promotion.
	res, err := svc.Analyze(ctx, appqa.QueryAccessRequest{
		SQL:           "SELECT u.name, o.user_id FROM app.users u JOIN app.orders o ON u.id = o.user_id",
		Dialect:       "postgresql",
		Mode:          "strict",
		DefaultSchema: "app",
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	t.Logf("classification=%s admission=%s reasons=%v relations=%+v",
		res.DomainResult.ReadClassification, res.DomainResult.Admission,
		res.DomainResult.ReasonCodes, res.DomainResult.Relations)

	// 7) Assert promotion: read_only + admissible.
	if res.DomainResult.ReadClassification != domain.ReadOnly {
		t.Errorf("classification = %v, want read_only", res.DomainResult.ReadClassification)
	}
	if res.DomainResult.Admission != domain.Admissible {
		t.Errorf("admission = %v, want admissible", res.DomainResult.Admission)
	}

	// 8) Verify no unproven_operator_effect reason remains after proof.
	for _, code := range res.DomainResult.ReasonCodes {
		if code == domain.ReasonUnprovenOperatorEffect {
			t.Error("unproven_operator_effect should be removed after manifest proof")
		}
	}

	// 9) Verify relations include both users and orders.
	foundUsers := false
	foundOrders := false
	for _, rel := range res.DomainResult.Relations {
		if rel.Name == "users" {
			foundUsers = true
		}
		if rel.Name == "orders" {
			foundOrders = true
		}
	}
	if !foundUsers {
		t.Errorf("expected users relation in result: %+v", res.DomainResult.Relations)
	}
	if !foundOrders {
		t.Errorf("expected orders relation in result: %+v", res.DomainResult.Relations)
	}
}

func TestEffectIdentityAdapter_PG17PinnedSessionIntegration(t *testing.T) {
	db, cleanup, err := openIntegrationDB(t)
	if err != nil {
		t.Fatal("PG17 integration service unavailable at configured endpoint (driver error suppressed)")
	}
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	session, err := PinSession(ctx, db)
	if err != nil {
		t.Fatalf("PinSession: %v", err)
	}
	defer session.Close()

	// Prove we are on a single backend for the resolve call.
	live, err := session.CaptureLiveContext(ctx)
	if err != nil {
		t.Fatalf("CaptureLiveContext: %v", err)
	}
	if !appqa.ResolutionContextSessionComplete(live) {
		t.Fatalf("incomplete live context: %+v", live)
	}
	if live.ServerVersionNum/10000 < 14 {
		t.Fatalf("unexpected server_version_num %d", live.ServerVersionNum)
	}
	// Runtime environment is the repo's PG17 compose image; do not claim 14–17 matrix.
	t.Logf("integration server_version_num=%d database_oid=%d role_oid=%d path_len=%d",
		live.ServerVersionNum, live.DatabaseOID, live.RoleOID, len(live.NamespaceSearchOIDs))

	adapter, err := NewEffectIdentityAdapter(session)
	if err != nil {
		t.Fatalf("adapter: %v", err)
	}

	// int4 = int4 and count(*) with exact type pins.
	const int4OID = 23
	batch, err := adapter.ResolveEffectIdentities(ctx, appqa.EffectIdentityRequest{
		Dialect: "postgresql",
		Candidates: []appqa.EffectCandidate{
			{Kind: appqa.EffectCandidateOperator, Ordinal: 0, NamePath: []string{"="}, Arity: 2, OperandKinds: []string{"column", "const"}},
			{Kind: appqa.EffectCandidateFunction, Ordinal: 1, NamePath: []string{"count"}, Arity: 0, IsAggregate: true},
			{Kind: appqa.EffectCandidateFunction, Ordinal: 2, NamePath: []string{"pg_catalog", "count"}, ExplicitSchema: true, Arity: 0, IsAggregate: true},
		},
		OperandTypeOIDs: map[int][]uint32{
			0: {int4OID, int4OID},
			1: {},
			2: {},
		},
		Resolution: live,
	})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(batch.Items) != 3 {
		t.Fatalf("items=%d", len(batch.Items))
	}
	for i, it := range batch.Items {
		if it.Status != domain.IdentityStatusResolved || it.Facts == nil {
			t.Fatalf("item %d not resolved: %+v", i, it)
		}
		if it.Facts.DatabaseOID != live.DatabaseOID || it.Facts.ServerVersionNum != live.ServerVersionNum {
			t.Fatalf("item %d locality pin mismatch: %+v vs live db=%d ver=%d", i, it.Facts, live.DatabaseOID, live.ServerVersionNum)
		}
		if it.Facts.ObjectOID == 0 {
			t.Fatalf("item %d zero object oid", i)
		}
	}
	// Explicit and unqualified count should both be pg_catalog.count(*) under default path.
	if batch.Items[1].Facts.ObjectOID != batch.Items[2].Facts.ObjectOID {
		t.Fatalf("count OID mismatch unqualified=%d explicit=%d", batch.Items[1].Facts.ObjectOID, batch.Items[2].Facts.ObjectOID)
	}
}

func TestScalarLive_PG17Builtins(t *testing.T) {
	db, cleanup, err := openIntegrationDB(t)
	if err != nil {
		t.Fatal("PG17 integration service unavailable at configured endpoint (driver error suppressed)")
	}
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stringProbes := []struct {
		query string
		want  string
	}{
		{query: "SELECT LOWER('TEST')", want: "test"},
		{query: "SELECT UPPER('test')", want: "TEST"},
		{query: "SELECT COALESCE(NULL, 'fallback')", want: "fallback"},
		{query: "SELECT NULLIF('a', 'b')", want: "a"},
	}
	for _, probe := range stringProbes {
		var got string
		if err := db.QueryRowContext(ctx, probe.query).Scan(&got); err != nil || got != probe.want {
			t.Fatal("PG17 scalar string probe returned an unexpected bounded value")
		}
	}

	numericProbes := []struct {
		query string
		want  float64
	}{
		{query: "SELECT LENGTH('hello')", want: 5},
		{query: "SELECT CHAR_LENGTH('hello')", want: 5},
		{query: "SELECT ABS(-42)", want: 42},
		{query: "SELECT CEIL(3.14)", want: 4},
		{query: "SELECT FLOOR(3.14)", want: 3},
	}
	for _, probe := range numericProbes {
		var got float64
		if err := db.QueryRowContext(ctx, probe.query).Scan(&got); err != nil || got != probe.want {
			t.Fatal("PG17 scalar numeric probe returned an unexpected bounded value")
		}
	}

	rows, err := db.QueryContext(ctx, `
		SELECT oid, proname, pronargs, proargtypes::text, prorettype::text, provolatile
		FROM pg_catalog.pg_proc
		WHERE proname IN ('lower','upper','length','char_length','character_length','abs','ceil','ceiling','floor','coalesce','nullif')
		  AND pronamespace = 'pg_catalog'::regnamespace
		ORDER BY proname, pronargs
	`)
	if err != nil {
		t.Fatal("PG17 scalar catalog probe failed (driver error suppressed)")
	}
	defer rows.Close()
	immutableRows := 0
	for rows.Next() {
		var (
			oid        uint32
			name       string
			arity      int
			argTypes   string
			resultType string
			volatility string
		)
		if err := rows.Scan(&oid, &name, &arity, &argTypes, &resultType, &volatility); err != nil || oid == 0 || name == "" || arity < 0 || argTypes == "" || resultType == "" {
			t.Fatal("PG17 scalar catalog probe returned an unexpected bounded fact")
		}
		if volatility == "i" {
			immutableRows++
		}
	}
	if err := rows.Err(); err != nil || immutableRows < 17 {
		t.Fatal("PG17 scalar catalog probe did not return the required immutable rows")
	}
}

func TestScalarLive_PG17CatalogBoundQueriesPromote(t *testing.T) {
	db, cleanup, err := openIntegrationDB(t)
	if err != nil {
		t.Fatal("PG17 integration service unavailable at configured endpoint (driver error suppressed)")
	}
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	session, err := PinSession(ctx, db)
	if err != nil {
		t.Fatalf("PinSession: %v", err)
	}
	defer session.Close()
	adapter, err := NewEffectIdentityAdapter(session)
	if err != nil {
		t.Fatalf("NewEffectIdentityAdapter: %v", err)
	}
	schemaResolver, err := NewQueryAccessConnResolver(session.Conn())
	if err != nil {
		t.Fatalf("NewQueryAccessConnResolver: %v", err)
	}
	policy, err := appqa.NewTrustPolicy(appqa.NewPG17Manifest())
	if err != nil {
		t.Fatalf("NewTrustPolicy: %v", err)
	}
	svc, err := appqa.NewTrustedService(adapter, policy, schemaResolver)
	if err != nil {
		t.Fatalf("NewTrustedService: %v", err)
	}

	for _, query := range []string{
		"SELECT LOWER(text_value) FROM app.scalar_facts",
		"SELECT UPPER(text_value) FROM app.scalar_facts",
		"SELECT LENGTH(text_value) FROM app.scalar_facts",
		"SELECT CHAR_LENGTH(text_value) FROM app.scalar_facts",
		"SELECT CHARACTER_LENGTH(text_value) FROM app.scalar_facts",
		"SELECT ABS(numeric_value) FROM app.scalar_facts",
		"SELECT CEIL(numeric_value) FROM app.scalar_facts",
		"SELECT CEILING(numeric_value) FROM app.scalar_facts",
		"SELECT FLOOR(numeric_value) FROM app.scalar_facts",
		"SELECT COALESCE(text_value, fallback_text) FROM app.scalar_facts",
		"SELECT NULLIF(text_value, fallback_text) FROM app.scalar_facts",
	} {
		result, err := svc.Analyze(ctx, appqa.QueryAccessRequest{SQL: query, Dialect: "postgresql", Mode: "strict", DefaultSchema: "app"})
		if err != nil {
			t.Fatalf("Analyze: %v", err)
		}
		if result.DomainResult.ReadClassification != domain.ReadOnly || result.DomainResult.Admission != domain.Admissible {
			t.Fatal("PG17 catalog-bound scalar query was not promoted")
		}
	}
}

func openIntegrationDB(t *testing.T) (*sql.DB, func(), error) {
	t.Helper()
	host := envOr("DELTASCOPE_PG_HOST", "127.0.0.1")
	port := envOrInt("DELTASCOPE_PG_PORT", 5500) // docker/pg-e2e-compose.yaml
	user := envOr("DELTASCOPE_PG_USER", "root")
	pass := envOr("DELTASCOPE_PG_PASSWORD", "root")
	database := envOr("DELTASCOPE_PG_DATABASE", "postgres")

	cfg := ConnectionConfig{
		Host:     host,
		Port:     port,
		User:     user,
		Password: pass,
		Database: database,
		SSLMode:  "disable",
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	db, err := OpenDBContext(ctx, cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("open %s:%d: %w", host, port, err)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("ping %s:%d: %w", host, port, err)
	}
	return db, func() { _ = db.Close() }, nil
}
