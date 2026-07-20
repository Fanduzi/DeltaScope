//go:build integration

package mysqlmeta

import (
	"context"
	"database/sql"
	"os"
	"strconv"
	"strings"
	"testing"
)

func assertBuiltinSemanticBoundaries(t *testing.T, ctx context.Context, conn *sql.Conn, profile builtinSemanticEvidenceProfile) {
	t.Helper()
	if _, err := conn.ExecContext(ctx, "USE app"); err != nil {
		t.Fatalf("%s fixture schema selection failed (driver error suppressed)", profile.name)
	}
	assertBuiltinSemanticFunctionBoundaries(t, ctx, conn, profile)
	assertBuiltinSemanticNativeForms(t, ctx, conn, profile)
}

func assertBuiltinSemanticFunctionBoundaries(t *testing.T, ctx context.Context, conn *sql.Conn, profile builtinSemanticEvidenceProfile) {
	t.Helper()
	const storedName = "semantic_probe_sum"
	if profile.dialect == "mysql" {
		dropStoredFunction(t, ctx, conn, storedName)
	}
	storedOutcome := executeOutcome(ctx, conn, "CREATE FUNCTION "+storedName+"(left_value INT, right_value INT) RETURNS INT DETERMINISTIC RETURN left_value + right_value")
	if storedOutcome != profile.storedFunctionResult {
		t.Fatalf("%s stored-function boundary returned an unexpected bounded outcome", profile.name)
	}
	if storedOutcome == semanticAccepted {
		dropStoredFunction(t, ctx, conn, storedName)
	}

	collisionOutcome := executeOutcome(ctx, conn, "CREATE FUNCTION count (value INT) RETURNS INT DETERMINISTIC RETURN value")
	if collisionOutcome != profile.collisionResult {
		t.Fatalf("%s builtin-like stored-function collision returned an unexpected bounded outcome", profile.name)
	}
	if collisionOutcome == semanticAccepted {
		var result int64
		if err := conn.QueryRowContext(ctx, "SELECT app.count(7)").Scan(&result); err != nil || result != 7 {
			t.Fatalf("%s qualified stored-function collision did not return the bounded probe value", profile.name)
		}
		dropStoredFunction(t, ctx, conn, "count")
	}
	udfOutcome := executeOutcome(ctx, conn, "CREATE FUNCTION semantic_probe_count RETURNS STRING SONAME 'deltascope_missing_udf.so'")
	if udfOutcome != semanticRejected {
		dropStoredFunction(t, ctx, conn, "semantic_probe_count")
		t.Fatalf("%s builtin-like UDF creation was not rejected", profile.name)
	}
	t.Logf("%s stored-function and UDF collision boundaries returned bounded outcomes", profile.name)
}

func assertBuiltinSemanticNativeForms(t *testing.T, ctx context.Context, conn *sql.Conn, profile builtinSemanticEvidenceProfile) {
	t.Helper()
	for _, query := range []string{
		"SELECT app.COUNT(*) FROM app.builtin_semantic_facts",
		"SELECT `COUNT`(*) FROM app.builtin_semantic_facts",
	} {
		if outcome := queryOutcome(ctx, conn, query); outcome != semanticRejected {
			t.Fatalf("%s qualified or quoted builtin form was not rejected", profile.name)
		}
	}
	if outcome := scalarQueryOutcome(ctx, conn, "SELECT app.LOWER('TEST')"); outcome != semanticRejected {
		t.Fatalf("%s qualified scalar builtin form was not rejected", profile.name)
	}
	if outcome := scalarQueryOutcome(ctx, conn, "SELECT `LOWER`('TEST')"); outcome != semanticAccepted {
		t.Fatalf("%s quoted scalar builtin boundary returned an unexpected bounded outcome", profile.name)
	}

	if _, err := conn.ExecContext(ctx, "SET SESSION sql_mode = ''"); err != nil {
		t.Fatalf("%s controlled SQL-mode reset failed (driver error suppressed)", profile.name)
	}
	if outcome := queryOutcome(ctx, conn, "SELECT COUNT (*) FROM app.builtin_semantic_facts"); outcome != semanticRejected {
		t.Fatalf("%s spaced builtin form unexpectedly passed without IGNORE_SPACE", profile.name)
	}
	if _, err := conn.ExecContext(ctx, "SET SESSION sql_mode = 'IGNORE_SPACE'"); err != nil {
		t.Fatalf("%s controlled IGNORE_SPACE setup failed (driver error suppressed)", profile.name)
	}
	var mode string
	if err := conn.QueryRowContext(ctx, "SELECT @@SESSION.sql_mode").Scan(&mode); err != nil || !strings.Contains(strings.ToUpper(mode), "IGNORE_SPACE") {
		t.Fatalf("%s controlled IGNORE_SPACE mode was not observed", profile.name)
	}
	if outcome := queryOutcome(ctx, conn, "SELECT COUNT (*) FROM app.builtin_semantic_facts"); outcome != semanticAccepted {
		t.Fatalf("%s IGNORE_SPACE spacing form returned an unexpected bounded outcome", profile.name)
	}
	commentOutcome := queryOutcome(ctx, conn, "SELECT COUNT/**/(*) FROM app.builtin_semantic_facts")
	t.Logf("%s IGNORE_SPACE comment form outcome %q", profile.name, commentOutcome)
	if commentOutcome != profile.sqlModeCommentResult {
		t.Fatalf("%s IGNORE_SPACE comment form returned an unexpected bounded outcome", profile.name)
	}
	t.Logf("%s qualified, quoted, spacing, and comment native-form boundaries returned bounded outcomes", profile.name)
}

func executeOutcome(ctx context.Context, conn *sql.Conn, statement string) semanticProbeOutcome {
	if _, err := conn.ExecContext(ctx, statement); err != nil {
		return semanticRejected
	}
	return semanticAccepted
}

func queryOutcome(ctx context.Context, conn *sql.Conn, query string) semanticProbeOutcome {
	var value int64
	if err := conn.QueryRowContext(ctx, query).Scan(&value); err != nil {
		return semanticRejected
	}
	return semanticAccepted
}

func scalarQueryOutcome(ctx context.Context, conn *sql.Conn, query string) semanticProbeOutcome {
	var value any
	if err := conn.QueryRowContext(ctx, query).Scan(&value); err != nil {
		return semanticRejected
	}
	return semanticAccepted
}

func dropStoredFunction(t *testing.T, ctx context.Context, conn *sql.Conn, name string) {
	t.Helper()
	if _, err := conn.ExecContext(ctx, "DROP FUNCTION IF EXISTS "+name); err != nil {
		t.Fatalf("stored-function cleanup failed (driver error suppressed)")
	}
}

func semanticEnvOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func semanticEnvOrInt(key string, fallback int) int {
	value := os.Getenv(key)
	parsed, err := strconv.Atoi(value)
	if value == "" || err != nil {
		return fallback
	}
	return parsed
}
