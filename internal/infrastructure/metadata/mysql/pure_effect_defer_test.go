// Package mysqlmeta locks a STATIC Phase-1 pure-effect deferral assumption
// for MySQL/TiDB. This file does NOT open a Docker connection and does NOT
// claim live evidence. The live Docker probes that supersede this static
// deferral assumption live in builtin_effect_identity_live_probes_test.go
// (build tag: integration).
//
// input: static Phase-1 deferral assumption (no live server)
// output: explicit deferred status until a separately audited proof model exists
//
// pos: research assumption only; superseded for MySQL/TiDB by the live probes
//
//	in builtin_effect_identity_live_probes_test.go which established DEFER
//
// note: if this file changes, update this header, the module README.md, and
//
//	the decision record's evidence section. See
//	docs/decisions/2026-07-17-query-access-mysql-tidb-effect-identity-feasibility.md
//	for the live-evidence-backed DEFER disposition that supersedes this static
//	Phase-1 deferral.
package mysqlmeta

import "testing"

// TestPureEffectProofDeferDisposition locks the STATIC Phase-1 deferral
// assumption. It does not prove live behavior — for live evidence see
// builtin_effect_identity_live_probes_test.go. The static deferral is retained
// only to document the Phase-1 reasoning path; the live probes established the
// actual DEFER disposition for MySQL 8.4 and TiDB 8.5.
func TestPureEffectProofDeferDisposition(t *testing.T) {
	t.Parallel()
	// Given: neither dialect has an offline OID-equivalent identity root or a
	// caller-session proof surface that can distinguish builtins from user code.
	// This was the Phase-1 static assumption. Live Docker probes later
	// confirmed the OID-equivalent absence and established the stronger DEFER
	// disposition (see builtin_effect_identity_live_probes_test.go).
	cases := map[string]pureEffectFeasibilityEvidence{
		"mysql": {
			StoredFunctionCanBeDeterministic:  true,
			DeterministicFlagIsTrustRoot:      false,
			BuiltinLikeCreateFailed:           true,
			BuiltinLikeNameCanBeAmbiguous:     true,
			OfflineBuiltinOIDBindingAvailable: false,
			SessionBoundProofDesigned:         false,
		},
		// TiDB static assumption: stored functions can declare DETERMINISTIC.
		// NOTE: SUPERSEDED by live evidence — TiDB 8.5 does NOT support CREATE
		// FUNCTION at all (see TestTiDB85_LiveProbes_StoredFunctionRejected).
		// Retained as documented Phase-1 reasoning, not as a live fact.
		"tidb": {
			StoredFunctionCanBeDeterministic:  true,
			DeterministicFlagIsTrustRoot:      false,
			BuiltinLikeNameCanBeAmbiguous:     true,
			OfflineBuiltinOIDBindingAvailable: false,
			SessionBoundProofDesigned:         false,
		},
	}

	for dialect, evidence := range cases {
		dialect := dialect
		evidence := evidence
		t.Run(dialect+" static phase-1 deferral (superseded by live DEFER)", func(t *testing.T) {
			t.Parallel()
			// When: the Phase-1 feasibility assumption is evaluated.
			// Then: name or determinism allowlists cannot promote functions.
			if !evidence.StoredFunctionCanBeDeterministic {
				t.Fatal("static assumption: stored functions must be able to declare DETERMINISTIC")
			}
			if evidence.DeterministicFlagIsTrustRoot {
				t.Fatal("static assumption: DETERMINISTIC must not be a trust root")
			}
			if !evidence.BuiltinLikeCreateFailed && !evidence.BuiltinLikeNameCanBeAmbiguous {
				t.Fatal("static assumption: builtin-like names must retain an ambiguity or creation boundary")
			}
			if evidence.OfflineBuiltinOIDBindingAvailable {
				t.Fatal("static assumption: offline OID-equivalent identity must remain unavailable")
			}
			if evidence.SessionBoundProofDesigned {
				t.Fatal("static assumption: session-bound proof is outside this deferred disposition")
			}
		})
	}
}
