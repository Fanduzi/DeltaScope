// T26-SERVER-ALTER-EXECUTION-01: the shared ALTER adjudication truth
// table — arm, owed facts, OrderedPresent republish decision, and the
// two separate conditionality axes (published flag = ConditionalSource,
// marker = ConditionalAudit) all derive here; the rule and the ordered
// channel both consume this one truth.
package spec

import (
	"testing"
)

// First-match-wins arm order (contract §D): ALT-A1 barrier beats every
// other row on any of its three sources; ALT-A2 OrderedAbsent refuses
// before privilege or options are read; ALT-A3 proven presence +
// consumable + clean republishes; ALT-A4 owes the exact per-premise
// formula and still republishes proven presence.
func TestAdjudicateServerAlterTruthTable(t *testing.T) {
	proven := &ServerState{Name: "s1", Read: true, Exists: true, Status: MetadataStatusConfirmed}
	zeroRow := &ServerState{Name: "s1", Read: true, Exists: false}
	barrier := &ServerState{Name: "s1", UnreadReason: MetadataUnreadBatchEffects}
	orderedAbsent := &ServerState{Name: "s1", OrderedAbsent: true}
	orderedPresent := &ServerState{Name: "s1", OrderedPresent: true}
	unbound := &ServerState{Name: "s1"}

	cases := []struct {
		name         string
		state        *ServerState
		privilege    ServerAlterPrivilege
		optionsClean bool
		contaminated bool
		arm          ServerAlterArm
		owed         []string
		publish      bool
		cs           bool
		ca           bool
	}{
		// ALT-A1 — every shadowing source is the same uniform no-op:
		// owed is exactly server.batch_effects, nothing publishes, no
		// conditional provenance.
		{"contaminated batch -> barrier", orderedAbsent, ServerAlterPrivilegeConsumable, true, true, ServerAlterArmBarrier, []string{ServerFactBatchEffects}, false, false, false},
		{"contaminated + proven presence still barrier", proven, ServerAlterPrivilegeUnproven, false, true, ServerAlterArmBarrier, []string{ServerFactBatchEffects}, false, false, false},
		{"channel barrier -> batch_effects only", barrier, ServerAlterPrivilegeConsumable, true, false, ServerAlterArmBarrier, []string{ServerFactBatchEffects}, false, false, false},
		{"barrier + unproven premise still only batch_effects", barrier, ServerAlterPrivilegeUnproven, false, false, ServerAlterArmBarrier, []string{ServerFactBatchEffects}, false, false, false},
		{"barrier wins over ordered present", &ServerState{Name: "s1", OrderedPresent: true, UnreadReason: MetadataUnreadBatchEffects}, ServerAlterPrivilegeConsumable, true, false, ServerAlterArmBarrier, []string{ServerFactBatchEffects}, false, false, false},
		// ALT-A2 — OrderedAbsent on the provably-same identity owns the
		// verdict and owes nothing; privilege and options never reach
		// the question.
		{"ordered absent -> refusal owes nothing", orderedAbsent, ServerAlterPrivilegeUnproven, false, false, ServerAlterArmTerminalRefusal, nil, false, false, false},
		{"ordered absent + consumable still refusal", orderedAbsent, ServerAlterPrivilegeConsumable, true, false, ServerAlterArmTerminalRefusal, nil, false, false, false},
		{"conditional ordered absent -> refusal inherits marker", &ServerState{Name: "s1", OrderedAbsent: true, OrderedAbsentConditional: true}, ServerAlterPrivilegeSuppressed, false, false, ServerAlterArmTerminalRefusal, nil, false, true, true},
		{"AM-1.3 seeded: conditional ordered absent + unproven + clean -> A2", &ServerState{Name: "s1", OrderedAbsent: true, OrderedAbsentConditional: true}, ServerAlterPrivilegeUnproven, true, false, ServerAlterArmTerminalRefusal, nil, false, true, true},
		// ALT-A3 — proven presence + consumable + clean republishes.
		{"ordered present + consumable + clean -> republish", orderedPresent, ServerAlterPrivilegeConsumable, true, false, ServerAlterArmProvenAlter, nil, true, false, true},
		{"conditional ordered present + consumable + clean -> inherited flag", &ServerState{Name: "s1", OrderedPresent: true, OrderedPresentConditional: true}, ServerAlterPrivilegeConsumable, true, false, ServerAlterArmProvenAlter, nil, true, true, true},
		{"catalog present + consumable + clean -> republish unconditional", proven, ServerAlterPrivilegeConsumable, true, false, ServerAlterArmProvenAlter, nil, true, false, true},
		// ALT-A4 — the exact owed formula; proven presence still
		// republishes, unknown existence mints nothing.
		{"ordered present + unproven -> privilege owed, republish", orderedPresent, ServerAlterPrivilegeUnproven, true, false, ServerAlterArmUnproven, []string{ServerFactAlterPrivilege}, true, false, false},
		{"ordered present + suppressed -> privilege owed, republish", orderedPresent, ServerAlterPrivilegeSuppressed, true, false, ServerAlterArmUnproven, []string{ServerFactAlterPrivilege}, true, false, false},
		{"ordered present + consumable + dirty -> options owed, republish", orderedPresent, ServerAlterPrivilegeConsumable, false, false, ServerAlterArmUnproven, []string{ServerFactAlterOptions}, true, false, true},
		{"ordered present + unproven + dirty -> both owed", orderedPresent, ServerAlterPrivilegeUnproven, false, false, ServerAlterArmUnproven, []string{ServerFactAlterPrivilege, ServerFactAlterOptions}, true, false, false},
		{"conditional ordered present + unproven -> inherited flag republish", &ServerState{Name: "s1", OrderedPresent: true, OrderedPresentConditional: true}, ServerAlterPrivilegeSuppressed, true, false, ServerAlterArmUnproven, []string{ServerFactAlterPrivilege}, true, true, true},
		{"AM-2.4 seeded: conditional ordered present + unproven + clean -> A4", &ServerState{Name: "s1", OrderedPresent: true, OrderedPresentConditional: true}, ServerAlterPrivilegeUnproven, true, false, ServerAlterArmUnproven, []string{ServerFactAlterPrivilege}, true, true, true},
		{"catalog present + unproven -> privilege owed, unconditional republish", proven, ServerAlterPrivilegeUnproven, true, false, ServerAlterArmUnproven, []string{ServerFactAlterPrivilege}, true, false, false},
		{"catalog present + consumable + dirty -> options owed, marker own-premise", proven, ServerAlterPrivilegeConsumable, false, false, ServerAlterArmUnproven, []string{ServerFactAlterOptions}, true, false, true},
		{"catalog present + suppressed + dirty -> privilege+options", proven, ServerAlterPrivilegeSuppressed, false, false, ServerAlterArmUnproven, []string{ServerFactAlterPrivilege, ServerFactAlterOptions}, true, false, false},
		{"zero-row stays unknown -> existence owed, no publish", zeroRow, ServerAlterPrivilegeConsumable, true, false, ServerAlterArmUnproven, []string{ServerFactExistence}, false, false, true},
		{"zero-row + unproven -> existence + privilege", zeroRow, ServerAlterPrivilegeUnproven, true, false, ServerAlterArmUnproven, []string{ServerFactExistence, ServerFactAlterPrivilege}, false, false, false},
		{"zero-row + consumable + dirty -> existence + options", zeroRow, ServerAlterPrivilegeConsumable, false, false, ServerAlterArmUnproven, []string{ServerFactExistence, ServerFactAlterOptions}, false, false, true},
		{"zero-row + suppressed + dirty -> all three", zeroRow, ServerAlterPrivilegeSuppressed, false, false, ServerAlterArmUnproven, []string{ServerFactExistence, ServerFactAlterPrivilege, ServerFactAlterOptions}, false, false, false},
		{"unbound state -> all owed by premise", unbound, ServerAlterPrivilegeUnproven, false, false, ServerAlterArmUnproven, []string{ServerFactExistence, ServerFactAlterPrivilege, ServerFactAlterOptions}, false, false, false},
		{"nil state + consumable + clean -> existence owed", nil, ServerAlterPrivilegeConsumable, true, false, ServerAlterArmUnproven, []string{ServerFactExistence}, false, false, true},
		{"nil state + unproven -> existence + privilege", nil, ServerAlterPrivilegeUnproven, true, false, ServerAlterArmUnproven, []string{ServerFactExistence, ServerFactAlterPrivilege}, false, false, false},
	}
	for _, tc := range cases {
		got := AdjudicateServerAlter(tc.state, tc.privilege, tc.optionsClean, tc.contaminated)
		if got.Arm != tc.arm {
			t.Fatalf("%s: arm = %v, want %v", tc.name, got.Arm, tc.arm)
		}
		if got.PublishPresent != tc.publish {
			t.Fatalf("%s: publish = %v, want %v", tc.name, got.PublishPresent, tc.publish)
		}
		if got.ConditionalSource != tc.cs {
			t.Fatalf("%s: conditional_source = %v, want %v", tc.name, got.ConditionalSource, tc.cs)
		}
		if got.ConditionalAudit != tc.ca {
			t.Fatalf("%s: conditional_audit = %v, want %v", tc.name, got.ConditionalAudit, tc.ca)
		}
		if got.Privilege != tc.privilege {
			t.Fatalf("%s: privilege projection = %v, want %v", tc.name, got.Privilege, tc.privilege)
		}
		if len(got.Owed) != len(tc.owed) {
			t.Fatalf("%s: owed = %v, want %v", tc.name, got.Owed, tc.owed)
		}
		set := factSet(got.Owed)
		for _, want := range tc.owed {
			if !set[want] {
				t.Fatalf("%s: owed = %v, missing %s", tc.name, got.Owed, want)
			}
		}
	}
}

// The two conditionality axes stay separate (contract §D): the
// published OrderedPresentConditional equals ConditionalSource — never
// ConditionalAudit — so a catalog-sourced republish lands
// unconditional flag=false even while the statement's own consumable
// premise marks its audit conditional.
func TestAdjudicateServerAlterConditionalAxes(t *testing.T) {
	proven := &ServerState{Name: "s1", Read: true, Exists: true, Status: MetadataStatusConfirmed}

	// Catalog presence + consumable + dirty: the own premise marks
	// the audit conditional, but the published presence flag is the
	// catalog's unconditional false.
	got := AdjudicateServerAlter(proven, ServerAlterPrivilegeConsumable, false, false)
	if !got.PublishPresent || got.ConditionalSource || !got.ConditionalAudit {
		t.Fatalf("catalog+consumable+dirty = %+v, want publish + unconditional source + own-premise marker", got)
	}

	// A consumed conditional OrderedPresent marks both the source and
	// the audit; the published flag inherits the consumed flag.
	condPresent := &ServerState{Name: "s1", OrderedPresent: true, OrderedPresentConditional: true}
	got = AdjudicateServerAlter(condPresent, ServerAlterPrivilegeSuppressed, true, false)
	if !got.PublishPresent || !got.ConditionalSource || !got.ConditionalAudit {
		t.Fatalf("conditional present + suppressed = %+v, want inherited-flag republish", got)
	}
	got = AdjudicateServerAlter(condPresent, ServerAlterPrivilegeUnproven, false, false)
	if !got.PublishPresent || !got.ConditionalSource || !got.ConditionalAudit {
		t.Fatalf("conditional present + unproven + dirty = %+v, want inherited-flag republish + marker", got)
	}

	// A barrier-shadowed conditional source is never consumed — no
	// provenance and no marker even under a consumable premise.
	shadowed := &ServerState{Name: "s1", OrderedPresent: true, OrderedPresentConditional: true, UnreadReason: MetadataUnreadBatchEffects}
	got = AdjudicateServerAlter(shadowed, ServerAlterPrivilegeConsumable, true, false)
	if got.Arm != ServerAlterArmBarrier || got.PublishPresent || got.ConditionalSource || got.ConditionalAudit {
		t.Fatalf("barrier-shadowed conditional source = %+v, want barrier with no publish and no provenance", got)
	}

	// Refusal on a conditional ordered absent keeps the inherited
	// marker; refusal on an unconditional absent carries none.
	got = AdjudicateServerAlter(&ServerState{Name: "s1", OrderedAbsent: true, OrderedAbsentConditional: true}, ServerAlterPrivilegeConsumable, true, false)
	if got.Arm != ServerAlterArmTerminalRefusal || !got.ConditionalSource || !got.ConditionalAudit {
		t.Fatalf("refusal on conditional absent = %+v, want inherited marker only", got)
	}
	got = AdjudicateServerAlter(&ServerState{Name: "s1", OrderedAbsent: true}, ServerAlterPrivilegeConsumable, true, false)
	if got.Arm != ServerAlterArmTerminalRefusal || got.ConditionalSource || got.ConditionalAudit {
		t.Fatalf("refusal on unconditional absent + consumable = %+v, want no marker — own premise is never read on A2", got)
	}
}
