//go:build postgresql && integration

// Package postgresqlmeta runs T7 adapter and trusted-service E2E against a live
// PostgreSQL 17 session when available.
// input: DELTASCOPE_PG_* env or docker compose defaults (localhost:5500)
// output: real catalog facts for = / count(*) under a pinned session;
//
//	real Service.Analyze promotion for count(*) via NewTrustedService
//
// pos: integration evidence only; skipped when Docker/PG unavailable (not claimed via mocks)
// note: if this file changes, update this header and module README.md.
package postgresqlmeta

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"
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
	if testing.Short() {
		t.Skip("skipping PG17 trusted-service E2E in short mode")
	}

	db, cleanup, err := openIntegrationDB(t)
	if err != nil {
		t.Skipf("PG17 integration unavailable (Docker/compose not running): %v", err)
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

func TestEffectIdentityAdapter_PG17PinnedSessionIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping PG17 integration in short mode")
	}

	db, cleanup, err := openIntegrationDB(t)
	if err != nil {
		t.Skipf("PG17 integration unavailable (Docker/compose not running): %v", err)
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

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func envOrInt(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil {
			return n
		}
	}
	return def
}
