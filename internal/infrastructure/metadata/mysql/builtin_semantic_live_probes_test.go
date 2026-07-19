//go:build integration

package mysqlmeta

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"
)

type semanticProbeOutcome string

const (
	semanticAccepted semanticProbeOutcome = "accepted"
	semanticRejected semanticProbeOutcome = "rejected"
)

type semanticWindowEvidence string

const (
	semanticWindowSupported semanticWindowEvidence = "supported"
	semanticWindowDeferred  semanticWindowEvidence = "deferred"
)

type builtinSemanticEvidenceProfile struct {
	name                 string
	dialect              string
	config               ConnectionConfig
	versionMarker        string
	storedFunctionResult semanticProbeOutcome
	collisionResult      semanticProbeOutcome
	sqlModeCommentResult semanticProbeOutcome
	windowEvidence       semanticWindowEvidence
}

func TestBuiltinSemantic57_Live(t *testing.T) {
	runBuiltinSemanticEvidence(t, builtinSemantic57Profile())
}

func TestBuiltinSemantic80_Live(t *testing.T) {
	runBuiltinSemanticEvidence(t, builtinSemantic80Profile())
}

func TestBuiltinSemantic84_Live(t *testing.T) {
	runBuiltinSemanticEvidence(t, builtinSemantic84Profile())
}

func TestBuiltinSemanticTiDB85_Live(t *testing.T) {
	runBuiltinSemanticEvidence(t, builtinSemanticTiDB85Profile())
}

func builtinSemantic57Profile() builtinSemanticEvidenceProfile {
	return builtinSemanticEvidenceProfile{
		name:                 "mysql57",
		dialect:              "mysql",
		config:               mysql57SemanticEvidenceConfig(),
		versionMarker:        "5.7.",
		storedFunctionResult: semanticAccepted,
		collisionResult:      semanticAccepted,
		sqlModeCommentResult: semanticRejected,
		windowEvidence:       semanticWindowDeferred,
	}
}

func builtinSemantic80Profile() builtinSemanticEvidenceProfile {
	return builtinSemanticEvidenceProfile{
		name:                 "mysql80",
		dialect:              "mysql",
		config:               mysql80SemanticEvidenceConfig(),
		versionMarker:        "8.0.",
		storedFunctionResult: semanticAccepted,
		collisionResult:      semanticAccepted,
		sqlModeCommentResult: semanticRejected,
		windowEvidence:       semanticWindowSupported,
	}
}

func builtinSemantic84Profile() builtinSemanticEvidenceProfile {
	return builtinSemanticEvidenceProfile{
		name:                 "mysql84",
		dialect:              "mysql",
		config:               mysql84SemanticEvidenceConfig(),
		versionMarker:        "8.4.",
		storedFunctionResult: semanticAccepted,
		collisionResult:      semanticAccepted,
		sqlModeCommentResult: semanticRejected,
		windowEvidence:       semanticWindowSupported,
	}
}

func builtinSemanticTiDB85Profile() builtinSemanticEvidenceProfile {
	return builtinSemanticEvidenceProfile{
		name:                 "tidb85",
		dialect:              "tidb",
		config:               tidb85SemanticEvidenceConfig(),
		versionMarker:        "TiDB-v8.5.",
		storedFunctionResult: semanticRejected,
		collisionResult:      semanticRejected,
		sqlModeCommentResult: semanticRejected,
		windowEvidence:       semanticWindowSupported,
	}
}

func mysql57SemanticEvidenceConfig() ConnectionConfig {
	return ConnectionConfig{
		Host:           semanticEnvOr("DELTASCOPE_MYSQL57_HOST", "127.0.0.1"),
		Port:           semanticEnvOrInt("DELTASCOPE_MYSQL57_PORT", 3507),
		User:           semanticEnvOr("DELTASCOPE_MYSQL57_USER", "root"),
		Password:       semanticEnvOr("DELTASCOPE_MYSQL57_PASSWORD", "root"),
		ConnectTimeout: 10 * time.Second,
	}
}

func mysql80SemanticEvidenceConfig() ConnectionConfig {
	return ConnectionConfig{
		Host:           semanticEnvOr("DELTASCOPE_MYSQL80_HOST", "127.0.0.1"),
		Port:           semanticEnvOrInt("DELTASCOPE_MYSQL80_PORT", 3800),
		User:           semanticEnvOr("DELTASCOPE_MYSQL80_USER", "root"),
		Password:       semanticEnvOr("DELTASCOPE_MYSQL80_PASSWORD", "root"),
		ConnectTimeout: 10 * time.Second,
	}
}

func mysql84SemanticEvidenceConfig() ConnectionConfig {
	return ConnectionConfig{
		Host:           semanticEnvOr("DELTASCOPE_MYSQL84_HOST", "127.0.0.1"),
		Port:           semanticEnvOrInt("DELTASCOPE_MYSQL84_PORT", 3840),
		User:           semanticEnvOr("DELTASCOPE_MYSQL84_USER", "root"),
		Password:       semanticEnvOr("DELTASCOPE_MYSQL84_PASSWORD", "root"),
		ConnectTimeout: 10 * time.Second,
	}
}

func tidb85SemanticEvidenceConfig() ConnectionConfig {
	return ConnectionConfig{
		Host:           semanticEnvOr("DELTASCOPE_TIDB85_HOST", "127.0.0.1"),
		Port:           semanticEnvOrInt("DELTASCOPE_TIDB85_PORT", 4850),
		User:           semanticEnvOr("DELTASCOPE_TIDB85_USER", "root"),
		Password:       semanticEnvOr("DELTASCOPE_TIDB85_PASSWORD", ""),
		ConnectTimeout: 10 * time.Second,
	}
}

func runBuiltinSemanticEvidence(t *testing.T, profile builtinSemanticEvidenceProfile) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	conn, db := openBuiltinSemanticEvidenceConn(t, ctx, profile)
	defer conn.Close()
	defer db.Close()

	assertBuiltinSemanticVersion(t, ctx, conn, profile)
	assertBuiltinSemanticAggregates(t, ctx, conn, profile)
	assertBuiltinSemanticWindow(t, ctx, conn, profile)
	assertBuiltinSemanticBoundaries(t, ctx, conn, profile)
}

func openBuiltinSemanticEvidenceConn(t *testing.T, ctx context.Context, profile builtinSemanticEvidenceProfile) (*sql.Conn, *sql.DB) {
	t.Helper()
	db, err := OpenDBContext(ctx, profile.config)
	if err != nil {
		t.Fatalf("%s service unavailable at configured endpoint (driver error suppressed)", profile.name)
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		db.Close()
		t.Fatalf("%s service connection unavailable (driver error suppressed)", profile.name)
	}
	return conn, db
}

func assertBuiltinSemanticVersion(t *testing.T, ctx context.Context, conn *sql.Conn, profile builtinSemanticEvidenceProfile) {
	var version string
	if err := conn.QueryRowContext(ctx, "SELECT VERSION()").Scan(&version); err != nil {
		t.Fatalf("%s VERSION probe failed (driver error suppressed)", profile.name)
	}
	t.Logf("%s observed version marker %q", profile.name, profile.versionMarker)
	versionMatches := strings.HasPrefix(version, profile.versionMarker)
	if profile.dialect == "tidb" {
		versionMatches = strings.Contains(version, profile.versionMarker)
	}
	if !versionMatches {
		t.Fatalf("%s VERSION did not match the independent profile prefix", profile.name)
	}
}

func assertBuiltinSemanticAggregates(t *testing.T, ctx context.Context, conn *sql.Conn, profile builtinSemanticEvidenceProfile) {
	t.Helper()
	const table = "app.builtin_semantic_facts"
	var countStar, countAmount, sumAmount, minAmount, maxAmount int64
	var averageAmount float64
	queries := []struct {
		name  string
		query string
		dest  *int64
	}{
		{"COUNT(*)", "SELECT COUNT(*) FROM " + table, &countStar},
		{"COUNT(amount)", "SELECT COUNT(amount) FROM " + table, &countAmount},
		{"SUM(amount)", "SELECT SUM(amount) FROM " + table, &sumAmount},
		{"MIN(amount)", "SELECT MIN(amount) FROM " + table, &minAmount},
		{"MAX(amount)", "SELECT MAX(amount) FROM " + table, &maxAmount},
	}
	for _, probe := range queries {
		if err := conn.QueryRowContext(ctx, probe.query).Scan(probe.dest); err != nil {
			t.Fatalf("%s %s aggregate probe failed (driver error suppressed)", profile.name, probe.name)
		}
	}
	if err := conn.QueryRowContext(ctx, "SELECT AVG(amount) FROM "+table).Scan(&averageAmount); err != nil {
		t.Fatalf("%s AVG(amount) aggregate probe failed (driver error suppressed)", profile.name)
	}
	if countStar != 4 || countAmount != 4 || sumAmount != 250 || averageAmount != 62.5 || minAmount != 20 || maxAmount != 100 {
		t.Fatalf("%s aggregate evidence returned unexpected bounded values", profile.name)
	}
	t.Logf("%s canonical aggregate evidence passed for COUNT, SUM, AVG, MIN, and MAX", profile.name)
}

func assertBuiltinSemanticWindow(t *testing.T, ctx context.Context, conn *sql.Conn, profile builtinSemanticEvidenceProfile) {
	t.Helper()
	if profile.windowEvidence == semanticWindowDeferred {
		t.Logf("%s ranking-window evidence deferred: exact profile is MySQL 5.7, which has no ranking-window support", profile.name)
		return
	}
	rows, err := conn.QueryContext(ctx, "SELECT id, ROW_NUMBER() OVER (PARTITION BY dept ORDER BY amount DESC, id) FROM app.builtin_semantic_facts ORDER BY id")
	if err != nil {
		t.Fatalf("%s canonical ranking-window probe failed (driver error suppressed)", profile.name)
	}
	defer rows.Close()
	want := [][2]int64{{1, 1}, {2, 2}, {3, 1}, {4, 2}}
	for rowIndex, expected := range want {
		if !rows.Next() {
			t.Fatalf("%s ranking-window probe returned too few rows", profile.name)
		}
		var id, rowNumber int64
		if err := rows.Scan(&id, &rowNumber); err != nil {
			t.Fatalf("%s ranking-window result scan failed (driver error suppressed)", profile.name)
		}
		if id != expected[0] || rowNumber != expected[1] {
			t.Fatalf("%s ranking-window result row %d did not match the independent fixture expectation", profile.name, rowIndex)
		}
	}
	if rows.Next() {
		t.Fatalf("%s ranking-window probe returned too many rows", profile.name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("%s ranking-window iteration failed (driver error suppressed)", profile.name)
	}
	t.Logf("%s canonical ranking-window evidence passed", profile.name)
}
