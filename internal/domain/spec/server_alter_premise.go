package spec

// server_alter_premise.go carries the T26-SERVER-ALTER-EXECUTION-01
// conditional privilege premise for the audited MySQL identifier-form
// ALTER SERVER slice — the accepted contract at
// docs/verification/t26-server-alter-contract-01 (R5 ACCEPTED,
// 93945c01). The chain mirrors the approved DROP/CREATE siblings: the
// request-level raw observation is shared by the family, and each
// audited ALTER derives its own premise projection feeding one shared
// adjudication that the rule output, the owed-fact set, and the
// ordered post-state all read.

// Bounded gap fact vocabulary for the ALTER SERVER adjudication —
// server.existence and server.batch_effects reuse the family's
// existing tokens; the two ALTER-specific facts are new.
const (
	// ServerFactAlterPrivilege is the SUPER execution premise for
	// ALTER SERVER — owed on every arm whose own premise is not
	// consumable. A satisfied server.drop_privilege or
	// server.create_privilege is never borrowed: each statement
	// projects its own premise.
	ServerFactAlterPrivilege = "server.alter_privilege"
	// ServerFactAlterOptions is owed whenever the authored OPTIONS
	// list is not clean — duplicate keys or a numeric member outside
	// the documented 0–9999 bound. Dirt is an owed fact only: the
	// manual records no refusal class for it, so it never feeds a
	// refusal claim.
	ServerFactAlterOptions = "server.alter_options"
)

// ServerAlterPrivilege is one statement's derived privilege premise
// projection — a per-statement verdict on the request-level raw
// observation, never a mutable field on shared instance facts.
type ServerAlterPrivilege int

const (
	// ServerAlterPrivilegeUnproven means the premise is not
	// consumable: no observation, unbound principal, out-of-domain
	// version, no positive witness, or the mode was never selected.
	ServerAlterPrivilegeUnproven ServerAlterPrivilege = iota
	// ServerAlterPrivilegeConsumable means the positive witness and
	// every binding hold for this statement.
	ServerAlterPrivilegeConsumable
	// ServerAlterPrivilegeSuppressed means the raw positive witness
	// exists but an earlier authored grant/role statement ended the
	// applicable range — the premise stays owed for this statement.
	ServerAlterPrivilegeSuppressed
)

// ServerAlterArm is the adjudicated arm of one audited ALTER SERVER
// — the single classification that drives gaps, message, coverage,
// verdict, and the ordered post-state.
type ServerAlterArm int

const (
	// ServerAlterArmUnproven covers every in-domain path whose
	// premises stay owed: unproven existence, non-consumable
	// privilege, or non-clean options. Its post-state republishes
	// proven presence when a presence source answered, and touches
	// nothing when existence stays unknown — the presence-
	// preservation theorem proves ALTER can change no tracked
	// presence fact, so no barrier or entry is minted.
	ServerAlterArmUnproven ServerAlterArm = iota
	// ServerAlterArmProvenAlter is proven presence on the
	// provably-same identity + consumable premise + clean options:
	// OrderedPresent republishes with the consumed source flag.
	ServerAlterArmProvenAlter
	// ServerAlterArmTerminalRefusal is OrderedAbsent on the
	// provably-same identity — the impossibility is proven by the
	// ordered absent alone and owes nothing; privilege and options
	// never reach the question. The consumed absent stands.
	ServerAlterArmTerminalRefusal
	// ServerAlterArmBarrier is a bound pre-state carrying an
	// undischarged shadowing effect: whole-batch contamination,
	// channel taint, or a relation-scan barrier — a uniform ordered-
	// channel no-op that owes exactly {server.batch_effects} and
	// mints nothing itself.
	ServerAlterArmBarrier
)

// ServerAlterAdjudication is the shared per-statement adjudication —
// the one truth the rule projection and the ordered apply both read.
type ServerAlterAdjudication struct {
	// Privilege is this statement's derived premise projection.
	Privilege ServerAlterPrivilege
	// Arm is the adjudicated arm.
	Arm ServerAlterArm
	// Owed lists the bounded gap facts this adjudication still owes —
	// derived from the arm's actual premises, never a fixed per-row
	// rule.
	Owed []string
	// PublishPresent directs the ordered channel to republish a
	// proven OrderedPresent on the provably-same identity — presence
	// survives the option-field write whether or not the alter ran.
	PublishPresent bool
	// ConditionalSource marks that this adjudication consumed an
	// ordered state a prior statement published under the conditional
	// privilege scope — distinct from this statement's own consumable
	// Privilege. A barrier-shadowed ordered source is never consumed
	// and never sets this field; a catalog presence answer carries
	// no ordered provenance and leaves it false.
	ConditionalSource bool
	// ConditionalAudit marks that the conditional scope actually fed
	// this adjudication's conclusion — either this statement's own
	// consumable premise reached an arm that read it, or an ordered
	// state published under that scope was consumed. It is the one
	// fact finding metadata must project as conditional_audit: a
	// barrier arm never carries it (its premises were shadowed,
	// never evaluated), and a refusal carries it only through a
	// conditional ordered source — a statement's own consumable
	// premise is never read on the refusal arm.
	ConditionalAudit bool
}

// AdjudicateServerAlter computes the shared adjudication for one
// audited identifier-form ALTER SERVER under the accepted
// first-match-wins arm order (contract §D):
//
//   - ALT-A1 Barrier: whole-batch contamination (the flag reaches the
//     arm table directly, never through UnreadReason) or a bound
//     channel taint/relation barrier (UnreadReason =
//     server.batch_effects) — owed exactly {server.batch_effects}, a
//     uniform no-op that creates or modifies nothing.
//   - ALT-A2 TerminalRefusal: OrderedAbsent on the provably-same
//     identity — owed ∅, the absent stands untouched.
//   - ALT-A3 ProvenAlter: proven presence (ordered OrderedPresent or
//     a proven catalog row) + consumable premise + clean options —
//     owed ∅, OrderedPresent republishes.
//   - ALT-A4 Unproven: everything else — the exact owed-set formula
//     over existence/alter_privilege/alter_options; proven presence
//     still republishes, unknown existence mints nothing.
//
// There is no convergent or guarded arm: MySQL ALTER SERVER has no
// IF EXISTS/IF NOT EXISTS form.
func AdjudicateServerAlter(state *ServerState, privilege ServerAlterPrivilege, optionsClean bool, contaminated bool) ServerAlterAdjudication {
	adjudication := ServerAlterAdjudication{Privilege: privilege}
	consumable := privilege == ServerAlterPrivilegeConsumable
	switch {
	case contaminated || (state != nil && state.UnreadReason == MetadataUnreadBatchEffects):
		// The channel's undischarged unknown effect shadows every
		// premise — the row asserts nothing about privilege, options,
		// or existence having been proven. A uniform no-op: nothing
		// publishes, mints, or clears.
		adjudication.Arm = ServerAlterArmBarrier
		adjudication.Owed = append(adjudication.Owed, ServerFactBatchEffects)
		// ConditionalAudit stays unset: the shadowed premises never
		// participated in the conclusion.
		return adjudication
	case state != nil && state.OrderedAbsent:
		// The ordered absent on the provably-same identity refuses
		// outright — privilege and options never reach the question,
		// and no native error class is claimed. A consumed conditional
		// absent keeps its source on the marker.
		adjudication.Arm = ServerAlterArmTerminalRefusal
		adjudication.ConditionalSource = state.OrderedAbsentConditional
		adjudication.ConditionalAudit = adjudication.ConditionalSource
		return adjudication
	case state != nil && (state.OrderedPresent || (state.Read && state.Exists)) && consumable && optionsClean:
		// Proven alter: presence proven, the premise consumable, the
		// authored options clean — OrderedPresent republishes with
		// the consumed source flag (a catalog source carries none).
		adjudication.Arm = ServerAlterArmProvenAlter
		adjudication.PublishPresent = true
		adjudication.ConditionalSource = state.OrderedPresentConditional
		adjudication.ConditionalAudit = true
		return adjudication
	default:
		adjudication.Arm = ServerAlterArmUnproven
		provenPresent := state != nil && (state.OrderedPresent || (state.Read && state.Exists))
		if !provenPresent {
			adjudication.Owed = append(adjudication.Owed, ServerFactExistence)
		}
		if !consumable {
			adjudication.Owed = append(adjudication.Owed, ServerFactAlterPrivilege)
		}
		if !optionsClean {
			adjudication.Owed = append(adjudication.Owed, ServerFactAlterOptions)
		}
		// Proven presence republishes on every arm whose source
		// proves it — the preservation theorem proves the write
		// cannot remove it. The published OrderedPresentConditional
		// equals ConditionalSource, never ConditionalAudit: a
		// statement's own conditional premise marks the audit
		// conclusion conditional, not the presence fact — so a
		// catalog-sourced presence publishes flag=false even while
		// the statement's own audit stays conditional.
		if provenPresent {
			adjudication.PublishPresent = true
			adjudication.ConditionalSource = state.OrderedPresentConditional
		}
		// The conditional marker follows actual use: this statement's
		// own consumable premise shaped the owed set, or a
		// conditional ordered source was consumed.
		adjudication.ConditionalAudit = adjudication.ConditionalSource || consumable
		return adjudication
	}
}
