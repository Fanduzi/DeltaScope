// Package mysqlmeta records the MySQL/TiDB pure-effect proof kill criterion.
// input: live feasibility probe conclusions for MySQL 8.0 and TiDB
// output: a test-locked defer disposition without promotion code
// pos: research evidence only; shared parser compatibility is not proof-domain compatibility
package mysqlmeta

import "testing"

type pureEffectFeasibilityEvidence struct {
	StoredFunctionCanBeDeterministic  bool
	DeterministicFlagIsTrustRoot      bool
	BuiltinLikeCreateFailed           bool
	BuiltinLikeNameCanBeAmbiguous     bool
	OfflineBuiltinOIDBindingAvailable bool
	SessionBoundProofDesigned         bool
}

func TestPureEffectProofFeasibility(t *testing.T) {
	// The live MySQL 8.0.45 probe observed CREATE FUNCTION my_sum with
	// DETERMINISTIC, while CREATE FUNCTION count(...) failed. Neither result
	// supplies a safe identity root for name-based promotion.
	mysql := pureEffectFeasibilityEvidence{
		StoredFunctionCanBeDeterministic:  true,
		DeterministicFlagIsTrustRoot:      false,
		BuiltinLikeCreateFailed:           true,
		BuiltinLikeNameCanBeAmbiguous:     true,
		OfflineBuiltinOIDBindingAvailable: false,
		SessionBoundProofDesigned:         false,
	}

	// TiDB is a separate proof domain even though it shares parser-adjacent
	// infrastructure with MySQL. No version-scoped, shadowing-safe offline
	// builtin identity was established by this gate.
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
		t.Run(dialect+" remains deferred", func(t *testing.T) {
			if !evidence.StoredFunctionCanBeDeterministic {
				t.Fatal("stored functions must be able to declare DETERMINISTIC")
			}
			if evidence.DeterministicFlagIsTrustRoot {
				t.Fatal("deterministic stored-function metadata must not be a trust root")
			}
			if !evidence.BuiltinLikeCreateFailed && !evidence.BuiltinLikeNameCanBeAmbiguous {
				t.Fatal("builtin-like function names must fail creation or retain ambiguity risk")
			}
			if evidence.OfflineBuiltinOIDBindingAvailable {
				t.Fatal("no offline OID-equivalent builtin binding was established")
			}
			if evidence.SessionBoundProofDesigned {
				t.Fatal("a new caller-session proof surface is outside this milestone")
			}
		})
	}
}
