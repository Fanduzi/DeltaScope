// T26-SERVER-DROP-EXECUTION-01: the shared grant collector, the
// positive-witness derivation, and the one adjudication truth table —
// arm, owed facts, and publish decision all derive here and the rule
// plus the ordered channel both consume them.
package spec

import (
	"testing"
)

func mysqlVersion(major, minor int) *VersionIdentity {
	return &VersionIdentity{
		Product:        VersionProductMySQL,
		Version:        "8.0.36",
		Major:          major,
		Minor:          minor,
		Patch:          36,
		ValidatedRange: true,
	}
}

func tidbVersion() *VersionIdentity {
	return &VersionIdentity{
		Product:        VersionProductTiDB,
		Version:        "8.5.0",
		Major:          8,
		Minor:          5,
		Patch:          0,
		ValidatedRange: true,
	}
}

// The collector accepts exact bounded tokens on direct global grant
// lines only — near-shaped tokens, non-global scopes, role-assignment
// lines, and substrings never count.
func TestCollectMySQLGlobalPrivilegesExactTokens(t *testing.T) {
	set := map[string]bool{}
	CollectMySQLGlobalPrivileges(set, []string{
		"GRANT SELECT, SUPER ON *.* TO 'u'@'%'",
		"GRANT RESOURCE_GROUP_USER ON *.* TO 'u'@'%'",
		"GRANT CONNECTION_ADMIN ON app.* TO 'u'@'%'",
		"GRANT 'app_role'@'%' TO 'u'@'%'",
		"grant ALL PRIVILEGES on *.* to 'u'@'%'",
	})
	for _, want := range []string{"SELECT", "SUPER", "RESOURCE_GROUP_USER", "ALL PRIVILEGES"} {
		if !set[want] {
			t.Fatalf("expected token %s in %v", want, set)
		}
	}
	for _, banned := range []string{"CONNECTION_ADMIN", "RESOURCE_GROUP_ADMIN", "app_role"} {
		if set[banned] {
			t.Fatalf("token %s must never enter the global set: %v", banned, set)
		}
	}
}

// The positive witness needs every binding at once: a bound account,
// an in-domain version, and the exact SUPER token — or ALL PRIVILEGES
// under the registered static-set proof — on a direct global grant
// line. Anything else stays unproven, never a denial.
func TestMySQLServerDropPrivilegeWitness(t *testing.T) {
	cases := []struct {
		name string
		obs  *MySQLServerDropPrivilegeObservation
		ver  *VersionIdentity
		want bool
	}{
		{"super on global", &MySQLServerDropPrivilegeObservation{Account: "u@%", GrantLines: []string{"GRANT SELECT, SUPER ON *.* TO 'u'@'%'"}}, mysqlVersion(8, 0), true},
		{"all privileges 8.0", &MySQLServerDropPrivilegeObservation{Account: "u@%", GrantLines: []string{"GRANT ALL PRIVILEGES ON *.* TO 'u'@'%'"}}, mysqlVersion(8, 0), true},
		{"all privileges 8.4 out of domain", &MySQLServerDropPrivilegeObservation{Account: "u@%", GrantLines: []string{"GRANT ALL PRIVILEGES ON *.* TO 'u'@'%'"}}, mysqlVersion(8, 4), false},
		{"all privileges 5.7 out of domain", &MySQLServerDropPrivilegeObservation{Account: "u@%", GrantLines: []string{"GRANT ALL PRIVILEGES ON *.* TO 'u'@'%'"}}, mysqlVersion(5, 7), false},
		{"unbound account", &MySQLServerDropPrivilegeObservation{Account: "", GrantLines: []string{"GRANT SUPER ON *.* TO 'u'@'%'"}}, mysqlVersion(8, 0), false},
		{"whitespace account", &MySQLServerDropPrivilegeObservation{Account: "   ", GrantLines: []string{"GRANT SUPER ON *.* TO 'u'@'%'"}}, mysqlVersion(8, 0), false},
		{"connection_admin is not super", &MySQLServerDropPrivilegeObservation{Account: "u@%", GrantLines: []string{"GRANT CONNECTION_ADMIN ON *.* TO 'u'@'%'"}}, mysqlVersion(8, 0), false},
		{"schema scope is not global", &MySQLServerDropPrivilegeObservation{Account: "u@%", GrantLines: []string{"GRANT SUPER ON app.* TO 'u'@'%'"}}, mysqlVersion(8, 0), false},
		{"role line carries no ON scope", &MySQLServerDropPrivilegeObservation{Account: "u@%", GrantLines: []string{"GRANT 'r'@'%' TO 'u'@'%'"}}, mysqlVersion(8, 0), false},
		{"empty grant read", &MySQLServerDropPrivilegeObservation{Account: "u@%", GrantLines: nil}, mysqlVersion(8, 0), false},
		{"unrecognised lines", &MySQLServerDropPrivilegeObservation{Account: "u@%", GrantLines: []string{"something else entirely"}}, mysqlVersion(8, 0), false},
		{"tidb target", &MySQLServerDropPrivilegeObservation{Account: "u@%", GrantLines: []string{"GRANT SUPER ON *.* TO 'u'@'%'"}}, tidbVersion(), false},
		{"nil version", &MySQLServerDropPrivilegeObservation{Account: "u@%", GrantLines: []string{"GRANT SUPER ON *.* TO 'u'@'%'"}}, nil, false},
		{"nil observation", nil, mysqlVersion(8, 0), false},
	}
	for _, tc := range cases {
		if got := MySQLServerDropPrivilegeWitness(tc.obs, tc.ver); got != tc.want {
			t.Fatalf("%s: witness = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func factSet(facts []string) map[string]bool {
	set := map[string]bool{}
	for _, f := range facts {
		set[f] = true
	}
	return set
}

// The adjudication truth table — every arm's gap set, message class,
// and publish decision derives from the same function the ordered
// channel reads. Refusal owns the whole verdict; guarded absent only
// owes privilege when it stays unproven; a consumable privilege
// discharges the premise without ever clearing barriers or widening
// identity.
func TestAdjudicateServerDropTruthTable(t *testing.T) {
	proven := &ServerState{Name: "s1", Read: true, Exists: true, Status: MetadataStatusConfirmed}
	unprovenRead := &ServerState{Name: "s1", Read: true, Exists: false}
	barrier := &ServerState{Name: "s1", UnreadReason: MetadataUnreadBatchEffects}
	orderedAbsent := &ServerState{Name: "s1", OrderedAbsent: true}
	unbound := &ServerState{Name: "s1"}

	cases := []struct {
		name      string
		state     *ServerState
		privilege ServerDropPrivilege
		ifExists  bool
		arm       ServerDropArm
		owed      []string
		publish   bool
	}{
		{"proven + consumable -> drop publishes", proven, ServerDropPrivilegeConsumable, false, ServerDropArmProven, nil, true},
		{"proven + unproven premise -> privilege owed", proven, ServerDropPrivilegeUnproven, false, ServerDropArmUnproven, []string{ServerFactDropPrivilege}, false},
		{"proven + suppressed premise -> privilege owed", proven, ServerDropPrivilegeSuppressed, false, ServerDropArmUnproven, []string{ServerFactDropPrivilege}, false},
		{"unproven + consumable -> existence owed", unprovenRead, ServerDropPrivilegeConsumable, false, ServerDropArmUnproven, []string{ServerFactExistence}, false},
		{"unproven + unproven premise -> both owed", unprovenRead, ServerDropPrivilegeUnproven, false, ServerDropArmUnproven, []string{ServerFactExistence, ServerFactDropPrivilege}, false},
		{"unproven + consumable + IF EXISTS -> convergent", unprovenRead, ServerDropPrivilegeConsumable, true, ServerDropArmConvergent, nil, true},
		{"unproven + unproven + IF EXISTS -> both owed", unprovenRead, ServerDropPrivilegeUnproven, true, ServerDropArmUnproven, []string{ServerFactExistence, ServerFactDropPrivilege}, false},
		{"unproven + suppressed + IF EXISTS -> both owed", unprovenRead, ServerDropPrivilegeSuppressed, true, ServerDropArmUnproven, []string{ServerFactExistence, ServerFactDropPrivilege}, false},
		{"unbound + consumable + IF EXISTS -> convergent", unbound, ServerDropPrivilegeConsumable, true, ServerDropArmConvergent, nil, true},
		{"nil state + consumable + IF EXISTS -> convergent", nil, ServerDropPrivilegeConsumable, true, ServerDropArmConvergent, nil, true},
		{"absent + consumable -> refusal owns verdict", orderedAbsent, ServerDropPrivilegeConsumable, false, ServerDropArmTerminalRefusal, nil, false},
		{"absent + unproven -> refusal owes nothing", orderedAbsent, ServerDropPrivilegeUnproven, false, ServerDropArmTerminalRefusal, nil, false},
		{"absent + suppressed -> refusal owes nothing", orderedAbsent, ServerDropPrivilegeSuppressed, false, ServerDropArmTerminalRefusal, nil, false},
		{"absent + IF EXISTS + consumable -> guard clean", orderedAbsent, ServerDropPrivilegeConsumable, true, ServerDropArmGuardedAbsent, nil, false},
		{"absent + IF EXISTS + unproven -> guard owes privilege", orderedAbsent, ServerDropPrivilegeUnproven, true, ServerDropArmGuardedAbsent, []string{ServerFactDropPrivilege}, false},
		{"absent + IF EXISTS + suppressed -> guard owes privilege", orderedAbsent, ServerDropPrivilegeSuppressed, true, ServerDropArmGuardedAbsent, []string{ServerFactDropPrivilege}, false},
		{"barrier + consumable -> batch_effects only", barrier, ServerDropPrivilegeConsumable, false, ServerDropArmBarrier, []string{ServerFactBatchEffects}, false},
		{"barrier + unproven -> batch_effects + privilege", barrier, ServerDropPrivilegeUnproven, false, ServerDropArmBarrier, []string{ServerFactBatchEffects, ServerFactDropPrivilege}, false},
		{"barrier + consumable + IF EXISTS -> barrier wins over convergent", barrier, ServerDropPrivilegeConsumable, true, ServerDropArmBarrier, []string{ServerFactBatchEffects}, false},
		{"barrier + suppressed + IF EXISTS -> both owed", barrier, ServerDropPrivilegeSuppressed, true, ServerDropArmBarrier, []string{ServerFactBatchEffects, ServerFactDropPrivilege}, false},
	}
	for _, tc := range cases {
		got := AdjudicateServerDrop(tc.state, tc.privilege, tc.ifExists)
		if got.Arm != tc.arm {
			t.Fatalf("%s: arm = %v, want %v", tc.name, got.Arm, tc.arm)
		}
		if got.PublishAbsent != tc.publish {
			t.Fatalf("%s: publish = %v, want %v", tc.name, got.PublishAbsent, tc.publish)
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

// ConditionalSource follows the OrderedAbsent's publishing arm — a
// refusal or guarded arm resting on an absent a conditional arm
// published stays marked conditional. The plainAbsent fixture below is
// hand-built with the marker field left unset to check the projection
// alone; it is not evidence that any production path publishes an
// unconditional absent (today's only publishers are the conditional
// arms). Both consumption forms check the projection.
func TestAdjudicateServerDropConditionalSource(t *testing.T) {
	conditionalAbsent := &ServerState{Name: "s1", OrderedAbsent: true, OrderedAbsentConditional: true}
	plainAbsent := &ServerState{Name: "s1", OrderedAbsent: true}

	refusal := AdjudicateServerDrop(conditionalAbsent, ServerDropPrivilegeSuppressed, false)
	if refusal.Arm != ServerDropArmTerminalRefusal || !refusal.ConditionalSource {
		t.Fatalf("refusal on conditional absent = %+v, want refusal + conditional source", refusal)
	}
	guard := AdjudicateServerDrop(conditionalAbsent, ServerDropPrivilegeUnproven, true)
	if guard.Arm != ServerDropArmGuardedAbsent || !guard.ConditionalSource {
		t.Fatalf("guard on conditional absent = %+v, want guarded + conditional source", guard)
	}
	if got := AdjudicateServerDrop(plainAbsent, ServerDropPrivilegeSuppressed, false); got.ConditionalSource {
		t.Fatalf("unconditionally published absent must not mark the consumer conditional: %+v", got)
	}
	if got := AdjudicateServerDrop(plainAbsent, ServerDropPrivilegeUnproven, true); got.ConditionalSource {
		t.Fatalf("guard on unconditional absent must not mark conditional: %+v", got)
	}
}
