// Package mysqlmeta locks the MySQL/TiDB pure-effect deferral disposition.
// input: dialect feasibility evidence for the Phase 1 proof boundary
// output: explicit deferred status until a separately audited proof model exists
// pos: research evidence only; no production promotion
package mysqlmeta

import "testing"

func TestPureEffectProofDeferDisposition(t *testing.T) {
	// Given: neither dialect has an offline OID-equivalent identity root or a
	// caller-session proof surface that can distinguish builtins from user code.
	cases := map[string]pureEffectFeasibilityEvidence{
		"mysql": {
			StoredFunctionCanBeDeterministic:  true,
			DeterministicFlagIsTrustRoot:      false,
			BuiltinLikeCreateFailed:           true,
			BuiltinLikeNameCanBeAmbiguous:     true,
			OfflineBuiltinOIDBindingAvailable: false,
			SessionBoundProofDesigned:         false,
		},
		"tidb": {
			StoredFunctionCanBeDeterministic:  true,
			DeterministicFlagIsTrustRoot:      false,
			BuiltinLikeNameCanBeAmbiguous:     true,
			OfflineBuiltinOIDBindingAvailable: false,
			SessionBoundProofDesigned:         false,
		},
	}

	for dialect, evidence := range cases {
		t.Run(dialect, func(t *testing.T) {
			// When: the Phase 1 feasibility evidence is evaluated.
			// Then: name or determinism allowlists cannot promote functions.
			if !evidence.StoredFunctionCanBeDeterministic {
				t.Fatal("stored functions must be able to declare DETERMINISTIC")
			}
			if evidence.DeterministicFlagIsTrustRoot {
				t.Fatal("DETERMINISTIC must not be a trust root")
			}
			if !evidence.BuiltinLikeCreateFailed && !evidence.BuiltinLikeNameCanBeAmbiguous {
				t.Fatal("builtin-like names must retain an ambiguity or creation boundary")
			}
			if evidence.OfflineBuiltinOIDBindingAvailable {
				t.Fatal("offline OID-equivalent identity must remain unavailable")
			}
			if evidence.SessionBoundProofDesigned {
				t.Fatal("session-bound proof is outside this deferred disposition")
			}
		})
	}
}
