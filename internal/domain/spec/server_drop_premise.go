package spec

import "strings"

// server_drop_premise.go carries the T26-SERVER-DROP-EXECUTION-01
// conditional privilege premise for the audited MySQL identifier-form
// DROP SERVER slice — the approved contract at
// docs/verification/t26-server-drop-premise-01 (cfd34782) and the
// semantics contract at docs/verification/t26-semantics-01 (b8a67e7b).
// The chain is: request-selected mode → provider observation on one
// connection → principal/target/version/context binding → ordered
// scope → per-statement projection → one shared adjudication consumed
// by both the rule output and the ordered post-state.

// PrivilegeAuditMode is the audit request's conditional privilege
// premise mode. The empty value selects the default path — no
// privilege read is issued and server.drop_privilege stays owed.
type PrivilegeAuditMode string

// PrivilegeAuditModeObservedAccountNewSession selects the approved
// conditional mode: this audit takes the provider connection's
// observed CURRENT_USER() account as the expected execution principal,
// with the expected execution context being a new session on the same
// target established after the observation; the result is conditioned
// on the relied-upon direct global grant still applying when that
// session loads privileges. It never claims the future executor was
// verified, that the pooled session holds effective SUPER, or that a
// new execution session was created.
const PrivilegeAuditModeObservedAccountNewSession PrivilegeAuditMode = "observed_account_new_session"

// Bounded gap fact vocabulary for the DROP SERVER adjudication — the
// same tokens the rule's evidence-gap contract already registers.
const (
	// ServerFactExistence is owed while the target's pre-state
	// existence stays unproven and is needed by the arm.
	ServerFactExistence = "server.existence"
	// ServerFactBatchEffects replaces the existence gap when the
	// ordered channel's undischarged unknown effect shadowed the
	// catalog answer (same dedup contract as the tablespace family).
	ServerFactBatchEffects = "server.batch_effects"
	// ServerFactDropPrivilege is the SUPER execution premise — owed on
	// every non-terminal-refusal arm whose premise is not consumable.
	ServerFactDropPrivilege = "server.drop_privilege"
)

// MySQLServerDropPrivilegeObservation is the request-level immutable
// raw evidence the optional privilege loader returns: the observed
// account plus the verbatim SHOW GRANTS FOR CURRENT_USER() lines, both
// read on one actual provider connection. Internal evidence only —
// account names and grant text never reach serialized output,
// findings, gaps, diagnostics, or public errors.
type MySQLServerDropPrivilegeObservation struct {
	// Account is the verbatim CURRENT_USER() answer on the observation
	// connection. An empty value means the principal stayed unbound —
	// the observation can never become a consumable witness.
	Account string `json:"-"`
	// GrantLines carries the verbatim SHOW GRANTS FOR CURRENT_USER()
	// lines on the same connection. A successful empty or
	// unrecognised result keeps the witness unproven — it is never a
	// denial and never a read error.
	GrantLines []string `json:"-"`
}

// ServerDropPrivilege is one statement's derived privilege premise
// projection — a per-statement verdict on the request-level raw
// observation, never a mutable field on shared instance facts.
type ServerDropPrivilege int

const (
	// ServerDropPrivilegeUnproven means the premise is not consumable:
	// no observation, unbound principal, out-of-domain version, no
	// positive witness, or the mode was never selected.
	ServerDropPrivilegeUnproven ServerDropPrivilege = iota
	// ServerDropPrivilegeConsumable means the positive witness and
	// every binding hold for this statement.
	ServerDropPrivilegeConsumable
	// ServerDropPrivilegeSuppressed means the raw positive witness
	// exists but an earlier authored grant/role statement ended the
	// applicable range — the premise stays owed for this statement.
	ServerDropPrivilegeSuppressed
)

// ServerDropArm is the adjudicated arm of one audited DROP SERVER —
// the single classification that drives gaps, message, coverage,
// verdict, and the ordered post-state.
type ServerDropArm int

const (
	// ServerDropArmUnproven covers every path whose premises stay
	// owed: unproven existence, consumable or unproven privilege,
	// plain or guarded form — post-state is the unproven barrier.
	ServerDropArmUnproven ServerDropArm = iota
	// ServerDropArmProven is proven-present + consumable privilege +
	// the arm's other premises: the drop is modeled and OrderedAbsent
	// publishes.
	ServerDropArmProven
	// ServerDropArmConvergent is the approved IF EXISTS convergence:
	// unproven pre-state + consumable privilege + proven identity +
	// clean channel — present→drop→absent and absent→guard
	// no-op→absent both publish the same post-state.
	ServerDropArmConvergent
	// ServerDropArmTerminalRefusal is valid ordered-absent + plain
	// DROP: the refusal is proven by the absent alone and owes no
	// separate privilege premise; absent stands.
	ServerDropArmTerminalRefusal
	// ServerDropArmGuardedAbsent is valid ordered-absent + IF EXISTS:
	// the guard no-op preserves absent; the privilege premise follows
	// the guarded arm's own requirements.
	ServerDropArmGuardedAbsent
	// ServerDropArmBarrier is a bound pre-state carrying the ordered
	// channel's undischarged unknown effect (batch_effects) — the
	// barrier stands regardless of privilege.
	ServerDropArmBarrier
)

// ServerDropAdjudication is the shared per-statement adjudication —
// the one truth the rule projection and the ordered apply both read.
type ServerDropAdjudication struct {
	// Privilege is this statement's derived premise projection.
	Privilege ServerDropPrivilege
	// Arm is the adjudicated arm.
	Arm ServerDropArm
	// Owed lists the bounded gap facts this adjudication still owes —
	// derived from the arm's actual premises, never a fixed per-row
	// rule.
	Owed []string
	// PublishAbsent directs the ordered channel to publish a proven
	// OrderedAbsent on the provably-same identity — only on the proven
	// or convergent arms, and always with the publishing statement's
	// own arm recorded.
	PublishAbsent bool
	// ConditionalSource marks that this adjudication consumed an
	// OrderedAbsent a prior statement published under the conditional
	// privilege scope — distinct from this statement's own consumable
	// Privilege. The finding surface keeps the conditional-audit
	// marker on refusal/guard arms that rest on such an absent
	// (T26-SERVER-DROP-EXECUTION-01 R1-D).
	ConditionalSource bool
}

// AdjudicateServerDrop computes the shared adjudication for one
// audited identifier-form DROP SERVER. The caller guarantees the
// audited slice — MySQL dialect, NameIdent provable identity, no
// unsupported marker — so the convergent arm's identity premise holds
// by construction and a batch_effects-bound state means the channel
// already carried a blocking taint or barrier.
func AdjudicateServerDrop(state *ServerState, privilege ServerDropPrivilege, ifExists bool) ServerDropAdjudication {
	adjudication := ServerDropAdjudication{Privilege: privilege}
	consumable := privilege == ServerDropPrivilegeConsumable
	switch {
	case state != nil && state.OrderedAbsent:
		adjudication.ConditionalSource = state.OrderedAbsentConditional
		if ifExists {
			// The guard no-op preserves absent; the privilege premise
			// follows the guarded arm's own requirements.
			adjudication.Arm = ServerDropArmGuardedAbsent
			if !consumable {
				adjudication.Owed = append(adjudication.Owed, ServerFactDropPrivilege)
			}
			return adjudication
		}
		// Plain DROP on a proven absent is a complete terminal
		// refusal — it owes no separate privilege premise and the
		// absent stands.
		adjudication.Arm = ServerDropArmTerminalRefusal
		return adjudication
	case state != nil && state.UnreadReason == MetadataUnreadBatchEffects:
		// The ordered channel's unknown effect shadowed the catalog —
		// privilege never clears the barrier, widens the relation
		// domain, or proves existence.
		adjudication.Arm = ServerDropArmBarrier
		adjudication.Owed = append(adjudication.Owed, ServerFactBatchEffects)
		if !consumable {
			adjudication.Owed = append(adjudication.Owed, ServerFactDropPrivilege)
		}
		return adjudication
	case state != nil && state.Read && state.Exists:
		if consumable {
			// Proven drop — present → drop → absent publishes.
			adjudication.Arm = ServerDropArmProven
			adjudication.PublishAbsent = true
			return adjudication
		}
		adjudication.Arm = ServerDropArmUnproven
		adjudication.Owed = append(adjudication.Owed, ServerFactDropPrivilege)
		return adjudication
	default:
		// Unproven existence: unbound, bound-unvouched, not-read, or
		// read-unknown. The convergent arm needs the clean channel the
		// barrier branch above already excluded — a proven identity is
		// the audited slice's construction.
		if ifExists && consumable {
			adjudication.Arm = ServerDropArmConvergent
			adjudication.PublishAbsent = true
			return adjudication
		}
		adjudication.Arm = ServerDropArmUnproven
		adjudication.Owed = append(adjudication.Owed, ServerFactExistence)
		if !consumable {
			adjudication.Owed = append(adjudication.Owed, ServerFactDropPrivilege)
		}
		return adjudication
	}
}

// MySQLServerDropPrivilegeDomain is the registered applicability
// domain for the DROP SERVER privilege witness: a resolved MySQL 8.0.x
// target. Out-of-domain products and unresolved or unregistered
// versions stay unproven — no inference from approximate syntax.
func MySQLServerDropPrivilegeDomain(version *VersionIdentity) bool {
	return version != nil &&
		version.Product == VersionProductMySQL &&
		version.Major == 8 && version.Minor == 0
}

// MySQLServerDropAllImpliesSuper is the registered static-set proof:
// on the in-domain MySQL 8.0.x surface the documented static global
// ALL PRIVILEGES list contains SUPER. It is a version-checked
// constant, never a substring heuristic.
func MySQLServerDropAllImpliesSuper(version *VersionIdentity) bool {
	return MySQLServerDropPrivilegeDomain(version)
}

// MySQLServerDropPrivilegeWitness reports whether one raw observation
// carries the positive witness for the DROP SERVER privilege premise:
// a bound observed account, an in-domain resolved version, and an
// exact-token SUPER — or ALL PRIVILEGES under the registered
// static-set proof — on a direct global ON *.* grant line. Successful
// empty or unrecognised representations answer false (unproven, never
// a denial); CONNECTION_ADMIN, near-shaped tokens, non-global scopes,
// and role-assignment lines never substitute.
func MySQLServerDropPrivilegeWitness(observation *MySQLServerDropPrivilegeObservation, version *VersionIdentity) bool {
	if observation == nil || strings.TrimSpace(observation.Account) == "" || !MySQLServerDropPrivilegeDomain(version) {
		return false
	}
	set := map[string]bool{}
	CollectMySQLGlobalPrivileges(set, observation.GrantLines)
	return set["SUPER"] || (set["ALL PRIVILEGES"] && MySQLServerDropAllImpliesSuper(version))
}
