// Package mysqlmeta records a STATIC Phase-1 pure-effect feasibility
// assumption for MySQL/TiDB. This file does NOT open a Docker connection and
// does NOT claim live evidence. The live Docker probes that supersede this
// static assumption live in builtin_effect_identity_live_probes_test.go
// (build tag: integration).
//
// input: static Phase-1 assumption (no live server)
// output: a test-locked static assumption that name/determinism allowlists
//
//	cannot promote functions, pending live evidence
//
// pos: research assumption only; superseded for MySQL/TiDB by the live probes
//
//	in builtin_effect_identity_live_probes_test.go which established KILL
//
// note: if this file changes, update this header, the module README.md, and
//
//	the decision record's evidence section. See
//	docs/decisions/2026-07-17-query-access-mysql-tidb-effect-identity-feasibility.md
//	for the live-evidence-backed KILL disposition that supersedes this static
//	Phase-1 assumption.
package mysqlmeta

import "testing"

// pureEffectFeasibilityEvidence is a STATIC assumption about Phase-1
// pure-effect feasibility. It is NOT live Docker evidence. The fields encode
// the Phase-1 hypothesis that motivated the original DEFER disposition; the
// live Docker probes in builtin_effect_identity_live_probes_test.go later
// established the actual MySQL 8.4 / TiDB 8.5 behavior and the KILL
// disposition.
type pureEffectFeasibilityEvidence struct {
	StoredFunctionCanBeDeterministic  bool
	DeterministicFlagIsTrustRoot      bool
	BuiltinLikeCreateFailed           bool
	BuiltinLikeNameCanBeAmbiguous     bool
	OfflineBuiltinOIDBindingAvailable bool
	SessionBoundProofDesigned         bool
}

// TestPureEffectProofFeasibility locks the STATIC Phase-1 assumption. It does
// not prove live behavior — for live evidence see
// builtin_effect_identity_live_probes_test.go. The static assumption is
// retained only to document the Phase-1 reasoning path; the live probes are
// the authoritative evidence.
func TestPureEffectProofFeasibility(t *testing.T) {
	t.Parallel()
	// MySQL static assumption: stored functions can declare DETERMINISTIC
	// (confirmed live in TestMySQL84_LiveProbes_StoredFunctionDeterministic).
	mysql := pureEffectFeasibilityEvidence{
		StoredFunctionCanBeDeterministic:  true,
		DeterministicFlagIsTrustRoot:      false,
		BuiltinLikeCreateFailed:           true,
		BuiltinLikeNameCanBeAmbiguous:     true,
		OfflineBuiltinOIDBindingAvailable: false,
		SessionBoundProofDesigned:         false,
	}

	// TiDB static assumption: stored functions can declare DETERMINISTIC.
	// NOTE: This static assumption is SUPERSEDED by live evidence — TiDB 8.5
	// does NOT support CREATE FUNCTION at all (see
	// TestTiDB85_LiveProbes_StoredFunctionRejected). The field is retained as
	// documented Phase-1 reasoning, not as a live fact.
	tidb := pureEffectFeasibilityEvidence{
		StoredFunctionCanBeDeterministic:  true,
		DeterministicFlagIsTrustRoot:      false,
		BuiltinLikeNameCanBeAmbiguous:     true,
		OfflineBuiltinOIDBindingAvailable: false,
		SessionBoundProofDesigned:         false,
	}

	for dialect, evidence := range map[string]pureEffectFeasibilityEvidence{
		"mysql": mysql,
		"tidb":  tidb,
	} {
		dialect := dialect
		evidence := evidence
		t.Run(dialect+" static phase-1 assumption (superseded by live probes)", func(t *testing.T) {
			t.Parallel()
			if !evidence.StoredFunctionCanBeDeterministic {
				t.Fatal("static assumption: stored functions must be able to declare DETERMINISTIC")
			}
			if evidence.DeterministicFlagIsTrustRoot {
				t.Fatal("static assumption: DETERMINISTIC must not be a trust root")
			}
			if !evidence.BuiltinLikeCreateFailed && !evidence.BuiltinLikeNameCanBeAmbiguous {
				t.Fatal("static assumption: builtin-like names must fail creation or retain ambiguity risk")
			}
			if evidence.OfflineBuiltinOIDBindingAvailable {
				t.Fatal("static assumption: no offline OID-equivalent builtin binding was established")
			}
			if evidence.SessionBoundProofDesigned {
				t.Fatal("static assumption: a new caller-session proof surface is outside this milestone")
			}
		})
	}
}
