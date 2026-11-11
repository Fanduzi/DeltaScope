// T26-SERVER-CREATE-EXECUTION-01: the shared CREATE adjudication
// truth table — arm, owed facts, OrderedPresent publish decision, and
// conditional-source projection all derive here; the rule and the
// ordered channel both consume this one truth.
package spec

import (
	"testing"
)

// First-match-wins arm order (contract §D): A1 contamination/barrier
// beats every other row; A2 proven presence refuses before privilege
// or options are ever read; A3 OrderedAbsent + consumable + clean
// publishes; A4 owes the exact per-premise facts.
func TestAdjudicateServerCreateTruthTable(t *testing.T) {
	proven := &ServerState{Name: "s1", Read: true, Exists: true, Status: MetadataStatusConfirmed}
	zeroRow := &ServerState{Name: "s1", Read: true, Exists: false}
	barrier := &ServerState{Name: "s1", UnreadReason: MetadataUnreadBatchEffects}
	orderedAbsent := &ServerState{Name: "s1", OrderedAbsent: true}
	orderedPresent := &ServerState{Name: "s1", OrderedPresent: true}
	unbound := &ServerState{Name: "s1"}

	cases := []struct {
		name         string
		state        *ServerState
		privilege    ServerCreatePrivilege
		optionsClean bool
		contaminated bool
		arm          ServerCreateArm
		owed         []string
		publish      bool
		ca           bool
	}{
		// A1 barrier — contamination or the channel barrier wins over
		// every other input; owed is exactly server.batch_effects.
		{"contaminated batch -> barrier", orderedAbsent, ServerCreatePrivilegeConsumable, true, true, ServerCreateArmBarrier, []string{ServerFactBatchEffects}, false, false},
		{"contaminated + proven presence still barrier", proven, ServerCreatePrivilegeUnproven, false, true, ServerCreateArmBarrier, []string{ServerFactBatchEffects}, false, false},
		{"channel barrier -> batch_effects only", barrier, ServerCreatePrivilegeConsumable, true, false, ServerCreateArmBarrier, []string{ServerFactBatchEffects}, false, false},
		{"barrier + unproven premise still only batch_effects", barrier, ServerCreatePrivilegeUnproven, false, false, ServerCreateArmBarrier, []string{ServerFactBatchEffects}, false, false},
		{"barrier wins over ordered present", &ServerState{Name: "s1", OrderedPresent: true, UnreadReason: MetadataUnreadBatchEffects}, ServerCreatePrivilegeConsumable, true, false, ServerCreateArmBarrier, []string{ServerFactBatchEffects}, false, false},
		// A2 terminal refusal — proven presence owns the verdict and
		// owes nothing; privilege and options never reach the question.
		{"catalog proven -> refusal owes nothing", proven, ServerCreatePrivilegeUnproven, false, false, ServerCreateArmTerminalRefusal, nil, false, false},
		{"catalog proven + consumable still refusal", proven, ServerCreatePrivilegeConsumable, true, false, ServerCreateArmTerminalRefusal, nil, false, false},
		{"ordered present -> refusal owes nothing", orderedPresent, ServerCreatePrivilegeSuppressed, false, false, ServerCreateArmTerminalRefusal, nil, false, false},
		// A3 proven create — OrderedAbsent + consumable + clean.
		{"ordered absent + consumable + clean -> publish", orderedAbsent, ServerCreatePrivilegeConsumable, true, false, ServerCreateArmProven, nil, true, true},
		{"ordered absent + unproven premise -> privilege owed", orderedAbsent, ServerCreatePrivilegeUnproven, true, false, ServerCreateArmUnproven, []string{ServerFactCreatePrivilege}, false, false},
		{"ordered absent + suppressed premise -> privilege owed", orderedAbsent, ServerCreatePrivilegeSuppressed, true, false, ServerCreateArmUnproven, []string{ServerFactCreatePrivilege}, false, false},
		{"ordered absent + consumable + unclean -> options owed", orderedAbsent, ServerCreatePrivilegeConsumable, false, false, ServerCreateArmUnproven, []string{ServerFactCreateOptions}, false, true},
		{"ordered absent + unproven + unclean -> both owed", orderedAbsent, ServerCreatePrivilegeUnproven, false, false, ServerCreateArmUnproven, []string{ServerFactCreatePrivilege, ServerFactCreateOptions}, false, false},
		// A4 unproven — the exact owed-set formula.
		{"zero-row catalog stays unknown -> existence owed", zeroRow, ServerCreatePrivilegeConsumable, true, false, ServerCreateArmUnproven, []string{ServerFactExistence}, false, true},
		{"zero-row + unproven -> existence + privilege", zeroRow, ServerCreatePrivilegeUnproven, true, false, ServerCreateArmUnproven, []string{ServerFactExistence, ServerFactCreatePrivilege}, false, false},
		{"zero-row + unclean -> existence + options", zeroRow, ServerCreatePrivilegeConsumable, false, false, ServerCreateArmUnproven, []string{ServerFactExistence, ServerFactCreateOptions}, false, true},
		{"zero-row + unproven + unclean -> all three", zeroRow, ServerCreatePrivilegeSuppressed, false, false, ServerCreateArmUnproven, []string{ServerFactExistence, ServerFactCreatePrivilege, ServerFactCreateOptions}, false, false},
		{"unbound state -> all owed by premise", unbound, ServerCreatePrivilegeUnproven, false, false, ServerCreateArmUnproven, []string{ServerFactExistence, ServerFactCreatePrivilege, ServerFactCreateOptions}, false, false},
		{"nil state + consumable + clean -> existence owed", nil, ServerCreatePrivilegeConsumable, true, false, ServerCreateArmUnproven, []string{ServerFactExistence}, false, true},
	}
	for _, tc := range cases {
		got := AdjudicateServerCreate(tc.state, tc.privilege, tc.optionsClean, tc.contaminated)
		if got.Arm != tc.arm {
			t.Fatalf("%s: arm = %v, want %v", tc.name, got.Arm, tc.arm)
		}
		if got.PublishPresent != tc.publish {
			t.Fatalf("%s: publish = %v, want %v", tc.name, got.PublishPresent, tc.publish)
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

// Conditional provenance: an OrderedPresent or OrderedAbsent published
// under the conditional scope marks its consumers; the published
// OrderedPresent always carries the conditional marker because today's
// only publish arm rests on a consumable premise.
func TestAdjudicateServerCreateConditionalSource(t *testing.T) {
	conditionalPresent := &ServerState{Name: "s1", OrderedPresent: true, OrderedPresentConditional: true}
	plainPresent := &ServerState{Name: "s1", OrderedPresent: true}
	conditionalAbsent := &ServerState{Name: "s1", OrderedAbsent: true, OrderedAbsentConditional: true}
	plainAbsent := &ServerState{Name: "s1", OrderedAbsent: true}

	refusal := AdjudicateServerCreate(conditionalPresent, ServerCreatePrivilegeUnproven, false, false)
	if refusal.Arm != ServerCreateArmTerminalRefusal || !refusal.ConditionalSource || !refusal.ConditionalAudit {
		t.Fatalf("refusal on conditional present = %+v, want refusal + conditional source + audit marker", refusal)
	}
	if got := AdjudicateServerCreate(plainPresent, ServerCreatePrivilegeUnproven, false, false); got.ConditionalSource || got.ConditionalAudit {
		t.Fatalf("unconditionally published present must not mark the consumer conditional: %+v", got)
	}
	// A catalog answer carries no conditional source even when the
	// statement's own premise is consumable — own privilege is never
	// read on the refusal arm.
	catalog := AdjudicateServerCreate(&ServerState{Name: "s1", Read: true, Exists: true, Status: MetadataStatusConfirmed}, ServerCreatePrivilegeConsumable, true, false)
	if catalog.Arm != ServerCreateArmTerminalRefusal || catalog.ConditionalAudit {
		t.Fatalf("catalog refusal with consumable premise = %+v, want refusal without conditional marker", catalog)
	}
	proven := AdjudicateServerCreate(conditionalAbsent, ServerCreatePrivilegeConsumable, true, false)
	if proven.Arm != ServerCreateArmProven || !proven.PublishPresent || !proven.PublishPresentConditional || !proven.ConditionalAudit {
		t.Fatalf("proven create on conditional absent = %+v, want publish + conditional marker", proven)
	}
	unproven := AdjudicateServerCreate(conditionalAbsent, ServerCreatePrivilegeUnproven, true, false)
	if unproven.Arm != ServerCreateArmUnproven || !unproven.ConditionalSource || !unproven.ConditionalAudit {
		t.Fatalf("unproven on conditional absent = %+v, want unproven + conditional source + audit marker", unproven)
	}
	if got := AdjudicateServerCreate(plainAbsent, ServerCreatePrivilegeUnproven, true, false); got.ConditionalSource || got.ConditionalAudit {
		t.Fatalf("unconditional absent must not mark the consumer conditional: %+v", got)
	}
	// A barrier shadows the ordered source entirely — even a
	// conditional OrderedPresent behind the barrier contributes
	// nothing, and a consumable own premise never reaches the arm.
	shadowed := AdjudicateServerCreate(&ServerState{Name: "s1", OrderedPresent: true, OrderedPresentConditional: true, UnreadReason: MetadataUnreadBatchEffects}, ServerCreatePrivilegeConsumable, true, false)
	if shadowed.Arm != ServerCreateArmBarrier || shadowed.ConditionalSource || shadowed.ConditionalAudit {
		t.Fatalf("barrier-shadowed conditional source = %+v, want barrier with no conditional provenance", shadowed)
	}
}

// OptionsClean is the shape gate for the options premise: documented
// keys, every numeric member inside the documented 0–9999 bound, and
// no duplicate keys. A conservative or unproven range state is never
// clean.
func TestServerDeclarationOptionsClean(t *testing.T) {
	clean := &ServerDeclaration{Options: []ServerOption{
		{Key: "host", ValueKind: "char"},
		{Key: "port", ValueKind: "numeric", RangeState: ServerOptionRangeInRange},
	}}
	if !clean.OptionsClean() {
		t.Fatalf("documented keys with in-range numeric must be clean")
	}
	for _, tc := range []struct {
		name string
		decl *ServerDeclaration
	}{
		{"duplicate keys", &ServerDeclaration{Options: []ServerOption{{Key: "host", ValueKind: "char"}}, DuplicateKeys: []string{"host"}}},
		{"out-of-range numeric", &ServerDeclaration{Options: []ServerOption{{Key: "port", ValueKind: "numeric", RangeState: ServerOptionRangeOutOfRange}}}},
		{"unproven numeric", &ServerDeclaration{Options: []ServerOption{{Key: "port", ValueKind: "numeric", RangeState: ServerOptionRangeUnproven}}}},
		{"empty range state stays unproven", &ServerDeclaration{Options: []ServerOption{{Key: "port", ValueKind: "numeric"}}}},
	} {
		if tc.decl.OptionsClean() {
			t.Fatalf("%s: must never be clean", tc.name)
		}
	}
}

// ConnectionMembersComplete is the audit-model completeness premise —
// USER, HOST, and DATABASE are the documented members this slice
// checks; PASSWORD/SOCKET/OWNER/PORT never factor into the notice.
func TestServerDeclarationConnectionMembersComplete(t *testing.T) {
	full := &ServerDeclaration{Options: []ServerOption{{Key: "user", ValueKind: "char"}, {Key: "host", ValueKind: "char"}, {Key: "database", ValueKind: "char"}}}
	if !full.ConnectionMembersComplete() {
		t.Fatalf("user+host+database must read complete")
	}
	for _, tc := range []struct {
		name string
		keys []string
	}{
		{"missing user", []string{"host", "database"}},
		{"missing host", []string{"user", "database"}},
		{"missing database", []string{"user", "host"}},
		{"password alone never completes", []string{"password", "port", "socket", "owner"}},
		{"empty options", nil},
	} {
		opts := make([]ServerOption, 0, len(tc.keys))
		for _, k := range tc.keys {
			opts = append(opts, ServerOption{Key: k, ValueKind: "char"})
		}
		if (&ServerDeclaration{Options: opts}).ConnectionMembersComplete() {
			t.Fatalf("%s: must read incomplete", tc.name)
		}
	}
}
