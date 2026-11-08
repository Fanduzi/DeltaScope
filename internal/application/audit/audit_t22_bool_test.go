package audit

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// audit_t22_bool_test.go pins the legal fixed-v8.5 BURSTABLE boolean arm:
// `BURSTABLE [EqOpt] Boolean` (TRUE/FALSE/NUM, = optional) is in-scope
// grammar — a parse error is an unimplemented surface, never a
// "repaired" expectation (issue #101 R2-A).

// Legal 8.5 boolean forms must reach kind=ddl — never parser_error.
func TestT22BurstableBooleanArmIsSupported(t *testing.T) {
	for _, sql := range []string{
		"CREATE RESOURCE GROUP rg_bool RU_PER_SEC=100 BURSTABLE=TRUE;",
		"CREATE RESOURCE GROUP rg_bool RU_PER_SEC=100 BURSTABLE FALSE;",
		"ALTER RESOURCE GROUP rg_bool BURSTABLE=FALSE;",
		"ALTER RESOURCE GROUP rg_bool BURSTABLE = 1;",
		"CREATE RESOURCE GROUP rg_bool RU_PER_SEC=100 BURSTABLE=0;",
	} {
		result := t22MustAudit(t, sql, spec.DialectTiDB, nil, "")
		if result.Statements[0].Kind != string(spec.KindDDL) {
			t.Fatalf("AuditSQL(%q) kind = %q, want ddl — legal 8.5 boolean arm must not parse-error", sql, result.Statements[0].Kind)
		}
		if len(result.Unsupported) != 0 {
			t.Fatalf("AuditSQL(%q) unsupported = %+v, want none — boolean arm is modeled", sql, result.Unsupported)
		}
	}
}

// Authored truth stays authored: TRUE/1/bare → enabled warn, FALSE/0 →
// no burst flag. The warn rule only fires on an authored enabled flag.
func TestT22BurstableBooleanTruthStaysAuthored(t *testing.T) {
	for _, tc := range []struct {
		sql      string
		wantWarn bool
	}{
		{"CREATE RESOURCE GROUP rg_t RU_PER_SEC=100 BURSTABLE=TRUE;", true},
		{"CREATE RESOURCE GROUP rg_t RU_PER_SEC=100 BURSTABLE 1;", true},
		{"CREATE RESOURCE GROUP rg_t RU_PER_SEC=100 BURSTABLE;", true},
		{"CREATE RESOURCE GROUP rg_t RU_PER_SEC=100 BURSTABLE=FALSE;", false},
		{"CREATE RESOURCE GROUP rg_t RU_PER_SEC=100 BURSTABLE=0;", false},
	} {
		result := t22MustAudit(t, tc.sql, spec.DialectTiDB, nil, "")
		got := len(t22Findings(result, 0, t22BurstableRule)) > 0
		if got != tc.wantWarn {
			t.Fatalf("AuditSQL(%q) burstable warn fired = %t, want %t", tc.sql, got, tc.wantWarn)
		}
	}
}

// A burstable token inside a string or identifier context must not be
// rewritten: the supplement binds only inside CREATE/ALTER RESOURCE GROUP.
func TestT22BurstableBooleanOutsideResourceGroupUntouched(t *testing.T) {
	result := t22MustAudit(t, "CREATE TABLE burstable (x int);", spec.DialectTiDB, nil, "")
	if result.Statements[0].Kind != string(spec.KindDDL) {
		t.Fatalf("kind = %q, want ddl — table named burstable parses normally", result.Statements[0].Kind)
	}
}
