// Package audit: T26-ALTER-INSTANCE-PUBLICATION-01 EXECUTION-01 —
// the audited MySQL ALTER INSTANCE slice's semantic matrix over the
// frozen contract D1..D10: three authored forms, the
// no_rollback_on_error variant member, per-form version gates, the
// shared privilege premise projection, the always-owed instance
// premises, batch contamination, and the TiDB boundaries.
package audit

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// aiProvider seeds version + privilege fixtures through the shared
// t26 harness.
func aiProvider(version string, obs *spec.MySQLServerDropPrivilegeObservation) *t26Provider {
	return &t26Provider{version: version, privilege: obs}
}

func aiAdjudication(t *testing.T, sql string, provider MetadataProvider, mode spec.PrivilegeAuditMode) spec.Statement {
	t.Helper()
	enriched := enrichT26Premise(t, sql, provider, mode)
	if len(enriched) != 1 || enriched[0].Metadata == nil || enriched[0].Metadata.AlterInstance == nil {
		t.Fatalf("%q: expected one audited ALTER INSTANCE, got %d stmts", sql, len(enriched))
	}
	return enriched[0]
}

func aiObservation(lines ...string) *spec.MySQLServerDropPrivilegeObservation {
	return &spec.MySQLServerDropPrivilegeObservation{Account: "app@%", GrantLines: lines}
}

// D1/D2: the three authored forms bind distinct feature identities on
// the audited path; NO ROLLBACK stays a variant member.
func TestT26AlterInstanceAuditedForms(t *testing.T) {
	for _, tc := range []struct {
		sql        string
		form       spec.AlterInstanceForm
		premise    string
		noRollback bool
	}{
		{"ALTER INSTANCE RELOAD TLS", spec.AlterInstanceFormReloadTLS, spec.AlterInstanceFactTLSContext, false},
		{"ALTER INSTANCE RELOAD TLS NO ROLLBACK ON ERROR", spec.AlterInstanceFormReloadTLS, spec.AlterInstanceFactTLSContext, true},
		{"ALTER INSTANCE ROTATE INNODB MASTER KEY", spec.AlterInstanceFormRotateInnodbMasterKey, spec.AlterInstanceFactKeyring, false},
	} {
		stmt := aiAdjudication(t, tc.sql, aiProvider("8.0.46", nil), spec.PrivilegeAuditMode(""))
		decl := stmt.DDL.AlterInstance
		if decl.Form != tc.form || decl.NoRollbackOnError != tc.noRollback {
			t.Fatalf("%q: decl=%+v want form=%d no_rollback=%v", tc.sql, decl, tc.form, tc.noRollback)
		}
		adj := stmt.Metadata.AlterInstance
		if adj.Arm != spec.AlterInstanceArmUnproven {
			t.Fatalf("%q: arm=%v want unproven", tc.sql, adj.Arm)
		}
		want := []string{spec.AlterInstanceFactPrivilege, tc.premise}
		if strings.Join(adj.Owed, ",") != strings.Join(want, ",") {
			t.Fatalf("%q: owed=%v want %v", tc.sql, adj.Owed, want)
		}
		if adj.ConditionalAudit {
			t.Fatalf("%q: conditional_audit true without witness", tc.sql)
		}
	}
}

// D2: version gates are per form — RELOAD TLS needs MySQL 8.0+,
// ROTATE exists from 5.7.11; below the bound or unresolvable the
// statement gates to the unaudited boundary and keeps its precise
// feature identity.
func TestT26AlterInstanceVersionGate(t *testing.T) {
	// Proven 5.7.44: RELOAD forms gate out (native grammar rejects —
	// probed 1064); ROTATE is inside its 5.7.11+ domain and audits.
	for _, sql := range []string{
		"ALTER INSTANCE RELOAD TLS",
		"ALTER INSTANCE RELOAD TLS NO ROLLBACK ON ERROR",
	} {
		enriched := enrichT26Premise(t, sql, aiProvider("5.7.44", nil), spec.PrivilegeAuditMode(""))
		u := enriched[0].Unsupported
		if u == nil || u.Feature != "alter_instance.reload_tls" ||
			u.Metadata["gate"] != "version" || u.Metadata["boundary"] != "unaudited" {
			t.Fatalf("%q@5.7.44: unsupported=%+v want version-gated unaudited", sql, u)
		}
		if sql == "ALTER INSTANCE RELOAD TLS NO ROLLBACK ON ERROR" && u.Metadata["no_rollback_on_error"] != true {
			t.Fatalf("%q@5.7.44: no_rollback member dropped: %+v", sql, u.Metadata)
		}
	}
	stmt := aiAdjudication(t, "ALTER INSTANCE ROTATE INNODB MASTER KEY", aiProvider("5.7.44", nil), spec.PrivilegeAuditMode(""))
	if stmt.Metadata.AlterInstance.Arm != spec.AlterInstanceArmUnproven {
		t.Fatalf("rotate@5.7.44: arm=%v want unproven", stmt.Metadata.AlterInstance.Arm)
	}
	// Below the documented 5.7.11 bound ROTATE gates out too.
	enriched := enrichT26Premise(t, "ALTER INSTANCE ROTATE INNODB MASTER KEY", aiProvider("5.7.10", nil), spec.PrivilegeAuditMode(""))
	u := enriched[0].Unsupported
	if u == nil || u.Metadata["gate"] != "version" {
		t.Fatalf("rotate@5.7.10: unsupported=%+v want version-gated", u)
	}
	// Unresolvable version gates every form — unknown stays bounded.
	for _, sql := range []string{
		"ALTER INSTANCE RELOAD TLS",
		"ALTER INSTANCE ROTATE INNODB MASTER KEY",
	} {
		enriched := enrichT26Premise(t, sql, aiProvider("", nil), spec.PrivilegeAuditMode(""))
		if enriched[0].Unsupported == nil {
			t.Fatalf("%q@unknown-version: audited without proven version", sql)
		}
	}
}

// D3: the privilege premise is the statement's own projection over
// the shared observed-account evidence — SUPER satisfies every form;
// the form's dynamic grant satisfies the matching form only; a
// missing grant is unproven, never denied.
func TestT26AlterInstancePrivilegePremise(t *testing.T) {
	for _, tc := range []struct {
		name string
		sql  string
		obs  *spec.MySQLServerDropPrivilegeObservation
		want spec.AlterInstancePrivilege
	}{
		{"reload SUPER", "ALTER INSTANCE RELOAD TLS", privObservationSUPER(), spec.AlterInstancePrivilegeConsumable},
		{"reload CONNECTION_ADMIN", "ALTER INSTANCE RELOAD TLS", aiObservation("GRANT SELECT, CONNECTION_ADMIN ON *.* TO `app`@`%`"), spec.AlterInstancePrivilegeConsumable},
		{"reload ENCRYPTION_KEY_ADMIN only", "ALTER INSTANCE RELOAD TLS", aiObservation("GRANT ENCRYPTION_KEY_ADMIN ON *.* TO `app`@`%`"), spec.AlterInstancePrivilegeUnproven},
		{"reload no grants", "ALTER INSTANCE RELOAD TLS", aiObservation("GRANT USAGE ON *.* TO `app`@`%`"), spec.AlterInstancePrivilegeUnproven},
		{"reload no observation", "ALTER INSTANCE RELOAD TLS", nil, spec.AlterInstancePrivilegeUnproven},
		{"rotate SUPER", "ALTER INSTANCE ROTATE INNODB MASTER KEY", privObservationSUPER(), spec.AlterInstancePrivilegeConsumable},
		{"rotate ENCRYPTION_KEY_ADMIN", "ALTER INSTANCE ROTATE INNODB MASTER KEY", aiObservation("GRANT ENCRYPTION_KEY_ADMIN ON *.* TO `app`@`%`"), spec.AlterInstancePrivilegeConsumable},
		{"rotate CONNECTION_ADMIN only", "ALTER INSTANCE ROTATE INNODB MASTER KEY", aiObservation("GRANT CONNECTION_ADMIN ON *.* TO `app`@`%`"), spec.AlterInstancePrivilegeUnproven},
		{"rotate unbound principal", "ALTER INSTANCE ROTATE INNODB MASTER KEY", &spec.MySQLServerDropPrivilegeObservation{GrantLines: []string{"GRANT SUPER ON *.* TO `app`@`%`"}}, spec.AlterInstancePrivilegeUnproven},
	} {
		stmt := aiAdjudication(t, tc.sql, aiProvider("8.0.46", tc.obs), spec.PrivilegeAuditModeObservedAccountNewSession)
		adj := stmt.Metadata.AlterInstance
		if adj.Privilege != tc.want {
			t.Fatalf("%s: privilege=%v want %v", tc.name, adj.Privilege, tc.want)
		}
		if tc.want == spec.AlterInstancePrivilegeConsumable {
			if !adj.ConditionalAudit || len(adj.Owed) != 1 || adj.Owed[0] != stmt.DDL.AlterInstance.InstancePremise() {
				t.Fatalf("%s: consumable adj=%+v want conditional audit + instance premise only", tc.name, adj)
			}
		}
	}
	// On 5.7 the dynamic-grant vocabulary does not exist: an
	// ENCRYPTION_KEY_ADMIN-shaped line is never a satisfaction there.
	stmt := aiAdjudication(t, "ALTER INSTANCE ROTATE INNODB MASTER KEY",
		aiProvider("5.7.44", aiObservation("GRANT ENCRYPTION_KEY_ADMIN ON *.* TO `app`@`%`")),
		spec.PrivilegeAuditModeObservedAccountNewSession)
	if stmt.Metadata.AlterInstance.Privilege != spec.AlterInstancePrivilegeUnproven {
		t.Fatalf("rotate@5.7 dynamic grant: privilege=%v want unproven", stmt.Metadata.AlterInstance.Privilege)
	}
	// An earlier authored grant statement ends the applicable range —
	// the satisfied premise reports suppressed, still owed.
	enriched := enrichT26Premise(t,
		"REVOKE SELECT ON *.* FROM 'victim'@'%'; ALTER INSTANCE RELOAD TLS",
		aiProvider("8.0.46", privObservationSUPER()), spec.PrivilegeAuditModeObservedAccountNewSession)
	if len(enriched) != 2 || enriched[1].Metadata == nil || enriched[1].Metadata.AlterInstance == nil {
		t.Fatalf("suppression batch: %+v", enriched)
	}
	adj := enriched[1].Metadata.AlterInstance
	if adj.Privilege != spec.AlterInstancePrivilegeSuppressed || !containsStr(adj.Owed, spec.AlterInstanceFactPrivilege) {
		t.Fatalf("suppressed premise: adj=%+v", adj)
	}
}

// Batch contamination: an earlier ALTER INSTANCE mutates the surface
// the next one's premise would read — the second bind is blocked.
func TestT26AlterInstanceBatchEffects(t *testing.T) {
	enriched := enrichT26Premise(t,
		"ALTER INSTANCE RELOAD TLS; ALTER INSTANCE ROTATE INNODB MASTER KEY",
		aiProvider("8.0.46", nil), spec.PrivilegeAuditMode(""))
	if len(enriched) != 2 {
		t.Fatalf("enriched=%d want 2", len(enriched))
	}
	first := enriched[0].Metadata.AlterInstance
	second := enriched[1].Metadata.AlterInstance
	if first == nil || first.Arm != spec.AlterInstanceArmUnproven {
		t.Fatalf("stmt1 arm: %+v want unproven", first)
	}
	if second == nil || second.Arm != spec.AlterInstanceArmBlocked {
		t.Fatalf("stmt2 arm: %+v want blocked", second)
	}
	if len(second.Owed) != 1 || second.Owed[0] != spec.AlterInstanceFactBatchEffects {
		t.Fatalf("stmt2 owed=%v want {alter_instance.batch_effects}", second.Owed)
	}
}

// The audited family is provably non-table — an ALTER INSTANCE in the
// same batch never mints the SRS dependency-stale flag, so a following
// SRS statement's dependency premise stays uncontaminated.
func TestT26AlterInstanceDoesNotTouchSRSChannel(t *testing.T) {
	state := newBatchState(spec.DialectMySQL, "golden", aiProvider("8.0.46", nil))
	state.SeedResolvedVersion(&spec.VersionIdentity{
		Product: spec.VersionProductMySQL, Major: 8, Minor: 0, Patch: 46,
		Source: spec.VersionSourceObserved, ValidatedRange: true,
	})
	stmts, failures := parseT26(t, "ALTER INSTANCE RELOAD TLS; DROP SPATIAL REFERENCE SYSTEM 1234")
	enriched := enrichSlice(t, state, aiProvider("8.0.46", nil), spec.PrivilegeAuditMode(""), stmts, failures)
	if len(enriched) != 2 {
		t.Fatalf("enriched=%d want 2", len(enriched))
	}
	if drop := enriched[1]; drop.Metadata == nil || drop.Metadata.SRSDrop == nil {
		t.Fatalf("drop adjudication missing: %+v", drop.Metadata)
	}
	if state.SRSDepsStale() {
		t.Fatalf("alter instance minted the dependency-stale flag")
	}
}

// D8: TiDB keeps its boundaries — RELOAD forms stay a named unaudited
// boundary (TiDB genuinely executes them; this slice does not own
// TiDB semantics), ROTATE is a vendor boundary.
func TestT26AlterInstanceTiDBBoundaries(t *testing.T) {
	result, err := t26Audit(t, spec.DialectTiDB, "ALTER INSTANCE RELOAD TLS")
	if err == nil || len(result.Unsupported) != 1 {
		t.Fatalf("tidb reload: result=%+v err=%v", result, err)
	}
	u := result.Unsupported[0]
	if u.Feature != "alter_instance.reload_tls" || u.Metadata["boundary"] != "unaudited" {
		t.Fatalf("tidb reload boundary: %+v", u)
	}
	result, err = t26Audit(t, spec.DialectTiDB, "ALTER INSTANCE RELOAD TLS NO ROLLBACK ON ERROR")
	u = result.Unsupported[0]
	if err == nil || u.Metadata["no_rollback_on_error"] != true || u.Metadata["boundary"] != "unaudited" {
		t.Fatalf("tidb no-rollback boundary: %+v err=%v", u, err)
	}
	result, err = t26Audit(t, spec.DialectTiDB, "ALTER INSTANCE ROTATE INNODB MASTER KEY")
	u = result.Unsupported[0]
	if err == nil || u.Metadata["boundary"] != "vendor" || u.Feature != "alter_instance.rotate_innodb_master_key" {
		t.Fatalf("tidb rotate boundary: %+v err=%v", u, err)
	}
}

// The rule projection: audited forms mint their notice findings with
// the bounded metadata; the owed facts publish as evidence gaps; the
// conditional notice fires only on a consumable premise.
func TestT26AlterInstanceRuleProjection(t *testing.T) {
	evaluate := func(sql string, provider MetadataProvider, mode spec.PrivilegeAuditMode) []string {
		enriched := enrichT26Premise(t, sql, provider, mode)
		result := EvaluateEnrichedForTest(t, enriched)
		ids := []string{}
		for _, s := range result.Statements {
			for _, f := range s.Findings {
				ids = append(ids, f.RuleID)
			}
		}
		return ids
	}
	ids := evaluate("ALTER INSTANCE RELOAD TLS", aiProvider("8.0.46", nil), spec.PrivilegeAuditMode(""))
	if !containsStr(ids, "ddl.mysql.alter_instance.reload_tls") || containsStr(ids, "ddl.mysql.alter_instance.privilege") {
		t.Fatalf("unproven reload findings=%v", ids)
	}
	ids = evaluate("ALTER INSTANCE ROTATE INNODB MASTER KEY", aiProvider("8.0.46", privObservationSUPER()), spec.PrivilegeAuditModeObservedAccountNewSession)
	if !containsStr(ids, "ddl.mysql.alter_instance.rotate_master_key") || !containsStr(ids, "ddl.mysql.alter_instance.privilege") {
		t.Fatalf("consumable rotate findings=%v", ids)
	}
}

// D10: no account names, grant text, or payload survive serialization.
func TestT26AlterInstancePrivacy(t *testing.T) {
	enriched := enrichT26Premise(t, "ALTER INSTANCE RELOAD TLS",
		aiProvider("8.0.46", privObservationSUPER()), spec.PrivilegeAuditModeObservedAccountNewSession)
	result := EvaluateEnrichedForTest(t, enriched)
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, needle := range []string{"app@%", "GRANT", "SUPER ON", "TO `app`"} {
		if strings.Contains(string(raw), needle) {
			t.Fatalf("serialized output leaks %q: %s", needle, raw)
		}
	}
}

// The loader seam fires only under the conditional mode on the MySQL
// dialect — a boundary or non-target batch never triggers the read.
func TestT26AlterInstancePrivilegeSupplyGate(t *testing.T) {
	provider := aiProvider("8.0.46", privObservationSUPER())
	enrichT26Premise(t, "ALTER INSTANCE RELOAD TLS", provider, spec.PrivilegeAuditModeObservedAccountNewSession)
	if provider.privilegeCalls != 1 {
		t.Fatalf("audited form under mode: privilegeCalls=%d want 1", provider.privilegeCalls)
	}
	provider = aiProvider("8.0.46", privObservationSUPER())
	enrichT26Premise(t, "ALTER INSTANCE RELOAD TLS", provider, spec.PrivilegeAuditMode(""))
	if provider.privilegeCalls != 0 {
		t.Fatalf("no mode: privilegeCalls=%d want 0", provider.privilegeCalls)
	}
	provider = aiProvider("8.0.46", privObservationSUPER())
	enrichT26Premise(t, "ALTER TABLE t ADD COLUMN c INT", provider, spec.PrivilegeAuditModeObservedAccountNewSession)
	if provider.privilegeCalls != 0 {
		t.Fatalf("non-target batch: privilegeCalls=%d want 0", provider.privilegeCalls)
	}
	// A version-gated statement never pulls the loader — the phase-2
	// version gate already marked it Unsupported before this gate
	// evaluates (same contract as the SRS family's trigger).
	provider = aiProvider("5.7.44", privObservationSUPER())
	enrichT26Premise(t, "ALTER INSTANCE RELOAD TLS", provider, spec.PrivilegeAuditModeObservedAccountNewSession)
	if provider.privilegeCalls != 0 {
		t.Fatalf("version-gated form under mode: privilegeCalls=%d want 0", provider.privilegeCalls)
	}
}

// ER1: every form's public prose explains only its own premise and its
// own variant semantics — RELOAD TLS never mentions keyring, ROTATE
// never mentions TLS context or NO ROLLBACK, and only the variant
// member carries the suppression semantics. The audit may stay silent
// about what it cannot prove; it must never mis-describe it.
func TestT26AlterInstanceExplanationIsolation(t *testing.T) {
	publicText := func(sql string, provider MetadataProvider) string {
		enriched := enrichT26Premise(t, sql, provider, spec.PrivilegeAuditMode(""))
		result := EvaluateEnrichedForTest(t, enriched)
		var b strings.Builder
		for _, s := range result.Statements {
			for _, f := range s.Findings {
				b.WriteString(f.Message)
				b.WriteString("\n")
				if f.Explanation != nil {
					b.WriteString(f.Explanation.Why)
					b.WriteString("\n")
					b.WriteString(f.Explanation.Risk)
					b.WriteString("\n")
					b.WriteString(f.Explanation.Suggestion)
					b.WriteString("\n")
				}
			}
		}
		return b.String()
	}
	assertFree := func(sql, text string, banned []string) {
		t.Helper()
		lower := strings.ToLower(text)
		for _, needle := range banned {
			if strings.Contains(lower, strings.ToLower(needle)) {
				t.Fatalf("%q: public prose leaks %q:\n%s", sql, needle, text)
			}
		}
	}

	provider := aiProvider("8.0.46", nil)
	reload := publicText("ALTER INSTANCE RELOAD TLS", provider)
	assertFree("reload", reload, []string{"keyring", "rotate", "no_rollback", "no rollback"})
	if !strings.Contains(reload, "TLS") {
		t.Fatalf("reload prose must explain its TLS-context premise:\n%s", reload)
	}

	variant := publicText("ALTER INSTANCE RELOAD TLS NO ROLLBACK ON ERROR", provider)
	assertFree("reload no-rollback", variant, []string{"keyring", "rotate"})
	if !strings.Contains(variant, "NO ROLLBACK") || !strings.Contains(variant, "TLS") {
		t.Fatalf("variant prose must explain TLS-context plus suppression:\n%s", variant)
	}

	rotate := publicText("ALTER INSTANCE ROTATE INNODB MASTER KEY", provider)
	assertFree("rotate", rotate, []string{"TLS", "reload", "no_rollback", "no rollback"})
	if !strings.Contains(rotate, "keyring") {
		t.Fatalf("rotate prose must explain its keyring premise:\n%s", rotate)
	}

	// The blocked arm is form-scoped too: a ROTATE blocked behind an
	// earlier RELOAD mutation must not claim a TLS premise.
	stmts, failures := parseT26(t,
		"ALTER INSTANCE RELOAD TLS; ALTER INSTANCE ROTATE INNODB MASTER KEY")
	state := newBatchState(spec.DialectMySQL, "golden", aiProvider("8.0.46", nil))
	state.SeedResolvedVersion(&spec.VersionIdentity{
		Product: spec.VersionProductMySQL, Major: 8, Minor: 0, Patch: 46, ValidatedRange: true,
	})
	enriched := enrichSlice(t, state,
		aiProvider("8.0.46", nil), spec.PrivilegeAuditMode(""), stmts, failures)
	result := EvaluateEnrichedForTest(t, enriched)
	var b strings.Builder
	for _, f := range result.Statements[1].Findings {
		b.WriteString(f.Message + "\n")
		if f.Explanation != nil {
			b.WriteString(f.Explanation.Why + "\n" + f.Explanation.Risk + "\n" + f.Explanation.Suggestion + "\n")
		}
	}
	assertFree("blocked rotate", b.String(), []string{"TLS", "reload", "no_rollback", "no rollback"})
}

func containsStr(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

var _ = context.Background
