package spec

// server_create_premise.go carries the T26-SERVER-CREATE-EXECUTION-01
// conditional privilege premise for the audited MySQL identifier-form
// CREATE SERVER slice — the accepted contract at
// docs/verification/t26-server-create-contract-01 (R4 PASS, 112beb08).
// The chain mirrors the approved DROP sibling: the request-level raw
// observation is shared by the family, and each audited CREATE derives
// its own premise projection feeding one shared adjudication that the
// rule output, the owed-fact set, and the ordered post-state all read.

// Bounded gap fact vocabulary for the CREATE SERVER adjudication —
// server.existence and server.batch_effects reuse the family's
// existing tokens; the two CREATE-specific facts are new.
const (
	// ServerFactCreatePrivilege is the SUPER execution premise for
	// CREATE SERVER — owed on every non-terminal arm whose own premise
	// is not consumable. A satisfied server.drop_privilege is never
	// borrowed: each statement projects its own premise.
	ServerFactCreatePrivilege = "server.create_privilege"
	// ServerFactCreateOptions is owed whenever the authored OPTIONS
	// list is not clean — duplicate keys, a numeric member outside the
	// documented 0–9999 bound, or a numeric state that stays unproven.
	ServerFactCreateOptions = "server.create_options"
)

// ServerCreatePrivilege is one statement's derived privilege premise
// projection — a per-statement verdict on the request-level raw
// observation, never a mutable field on shared instance facts.
type ServerCreatePrivilege int

const (
	// ServerCreatePrivilegeUnproven means the premise is not
	// consumable: no observation, unbound principal, out-of-domain
	// version, no positive witness, or the mode was never selected.
	ServerCreatePrivilegeUnproven ServerCreatePrivilege = iota
	// ServerCreatePrivilegeConsumable means the positive witness and
	// every binding hold for this statement.
	ServerCreatePrivilegeConsumable
	// ServerCreatePrivilegeSuppressed means the raw positive witness
	// exists but an earlier authored grant/role statement ended the
	// applicable range — the premise stays owed for this statement.
	ServerCreatePrivilegeSuppressed
)

// ServerCreateArm is the adjudicated arm of one audited CREATE
// SERVER — the single classification that drives gaps, message,
// coverage, verdict, and the ordered post-state.
type ServerCreateArm int

const (
	// ServerCreateArmUnproven covers every in-domain path whose
	// premises stay owed: unproven existence, non-consumable
	// privilege, or non-clean options — post-state is the unproven
	// barrier.
	ServerCreateArmUnproven ServerCreateArm = iota
	// ServerCreateArmProven is OrderedAbsent on the provably-same
	// identity + consumable premise + clean options: the create is
	// modeled and OrderedPresent publishes.
	ServerCreateArmProven
	// ServerCreateArmTerminalRefusal is proven presence on the
	// provably-same identity — an ordered OrderedPresent or a proven
	// catalog row. The impossibility is proven by occupancy alone and
	// owes nothing; privilege and options never reach the question.
	ServerCreateArmTerminalRefusal
	// ServerCreateArmBarrier is a bound pre-state carrying the ordered
	// channel's undischarged unknown effect (batch_effects) — the
	// barrier stands regardless of privilege, options, or existence.
	ServerCreateArmBarrier
)

// ServerCreateAdjudication is the shared per-statement adjudication —
// the one truth the rule projection and the ordered apply both read.
type ServerCreateAdjudication struct {
	// Privilege is this statement's derived premise projection.
	Privilege ServerCreatePrivilege
	// Arm is the adjudicated arm.
	Arm ServerCreateArm
	// Owed lists the bounded gap facts this adjudication still owes —
	// derived from the arm's actual premises, never a fixed per-row
	// rule.
	Owed []string
	// PublishPresent directs the ordered channel to publish a proven
	// OrderedPresent on the provably-same identity — only on the
	// proven arm.
	PublishPresent bool
	// PublishPresentConditional records the conditional provenance of
	// the published OrderedPresent: the conclusion rested on a
	// consumable conditional premise (or an inherited conditional
	// source), so every OrderedPresent published today carries the
	// conditional marker — the unconditional variant stays a
	// field-honest placeholder.
	PublishPresentConditional bool
	// ConditionalSource marks that this adjudication consumed an
	// ordered state a prior statement published under the conditional
	// privilege scope — distinct from this statement's own consumable
	// Privilege. A barrier-shadowed ordered source is never consumed
	// and never sets this field.
	ConditionalSource bool
	// ConditionalAudit marks that the conditional scope actually fed
	// this adjudication's conclusion — either this statement's own
	// consumable premise reached an arm that read it (proven or
	// unproven), or an ordered state published under that scope was
	// consumed. It is the one fact finding metadata must project as
	// conditional_audit: a barrier arm never carries it (its premises
	// were shadowed, never evaluated), and a refusal carries it only
	// through a conditional ordered source — a statement's own
	// consumable premise is never read on the refusal arm.
	ConditionalAudit bool
}

// AdjudicateServerCreate computes the shared adjudication for one
// audited identifier-form CREATE SERVER under the accepted
// first-match-wins arm order (contract §D):
//
//   - A1 Barrier: whole-batch contamination (contaminated input — the
//     flag reaches the arm table directly, never through UnreadReason)
//     or a bound channel taint/relation barrier
//     (UnreadReason = server.batch_effects) — owed exactly
//     {server.batch_effects}.
//   - A2 TerminalRefusal: proven presence — ordered OrderedPresent or
//     a proven catalog row — owed ∅.
//   - A3 ProvenCreate: OrderedAbsent + consumable premise + clean
//     options — owed ∅, publishes OrderedPresent.
//   - A4 Unproven: everything else — the exact owed-set formula over
//     existence/create_privilege/create_options.
//
// There is no convergent or guarded arm: MySQL CREATE SERVER has no
// IF NOT EXISTS form.
func AdjudicateServerCreate(state *ServerState, privilege ServerCreatePrivilege, optionsClean bool, contaminated bool) ServerCreateAdjudication {
	adjudication := ServerCreateAdjudication{Privilege: privilege}
	consumable := privilege == ServerCreatePrivilegeConsumable
	switch {
	case contaminated || (state != nil && state.UnreadReason == MetadataUnreadBatchEffects):
		// The channel's undischarged unknown effect shadows every
		// premise — the row asserts nothing about privilege, options,
		// or existence having been proven.
		adjudication.Arm = ServerCreateArmBarrier
		adjudication.Owed = append(adjudication.Owed, ServerFactBatchEffects)
		// ConditionalAudit stays unset: the shadowed premises never
		// participated in the conclusion.
		return adjudication
	case state != nil && (state.OrderedPresent || (state.Read && state.Exists)):
		// Proven presence refuses outright — privilege and options
		// never reach the question. A consumed conditional
		// OrderedPresent keeps its source on the marker; a catalog
		// answer carries none.
		adjudication.Arm = ServerCreateArmTerminalRefusal
		adjudication.ConditionalSource = state.OrderedPresentConditional
		adjudication.ConditionalAudit = state.OrderedPresentConditional
		return adjudication
	case state != nil && state.OrderedAbsent && consumable && optionsClean:
		// Proven create: the slot is proven free, the premise
		// consumable, and the options clean — OrderedPresent publishes
		// with the conditional provenance of the conclusion.
		adjudication.Arm = ServerCreateArmProven
		adjudication.PublishPresent = true
		adjudication.ConditionalSource = state.OrderedAbsentConditional
		adjudication.PublishPresentConditional = true
		adjudication.ConditionalAudit = true
		return adjudication
	default:
		adjudication.Arm = ServerCreateArmUnproven
		if !(state != nil && state.OrderedAbsent) {
			adjudication.Owed = append(adjudication.Owed, ServerFactExistence)
		}
		if !consumable {
			adjudication.Owed = append(adjudication.Owed, ServerFactCreatePrivilege)
		}
		if !optionsClean {
			adjudication.Owed = append(adjudication.Owed, ServerFactCreateOptions)
		}
		// An OrderedAbsent that still answered the slot was consumed
		// — its conditional source rides the marker even while the
		// statement's own premise stays owed.
		if state != nil && state.OrderedAbsent {
			adjudication.ConditionalSource = state.OrderedAbsentConditional
		}
		// The conditional marker follows actual use: this statement's
		// own consumable premise shaped the owed set, or a conditional
		// ordered source was consumed.
		adjudication.ConditionalAudit = adjudication.ConditionalSource || consumable
		return adjudication
	}
}
