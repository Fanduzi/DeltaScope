// Package audit: the MySQL SERVER ordered channel — T26-SERVER-DROP-01
// shipped the protective layer (unaudited-write records, channel
// taint, relation-checked barriers, and the six-step resolve order
// whose every barrier decision completes before any provider or
// cache access), and T26-SERVER-DROP-EXECUTION-01 added the shared
// adjudication's post-state: a publish arm records the proven
// OrderedAbsent with the statement's own arm, the refusal and guarded
// arms leave an absent untouched, and every undischarged unknown
// effect still lands the unproven barrier that shadows the audit-time
// catalog present for later statements
// (docs/verification/t26-semantics-01 b8a67e7b,
// docs/verification/t26-server-drop-premise-01 cfd34782).
//
// Identity relation R(A,B) is the accepted minimal conservative
// contract, evaluated in order and mutually exclusive:
//  1. byte-identical -> provably-same;
//  2. both names inside the registered SERVER ASCII case-insensitive
//     subset AND ASCII-fold-equal -> provably-same;
//  3. everything else -> unproven.
//
// provably-distinct stays a registered class with an empty current
// provable domain — nothing derives distinct from fold keys,
// differing characters, lengths, or tails. The collation PAD
// attribute governs trailing-space comparison only, so leading and
// internal space relations stay unproven by the model's own
// capability boundary. The canonical fold key is a bucket key, never
// an identity verdict, and entries keep the verbatim authored name so
// relation checks never read the folded key as an identity.
package audit

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// serverRelation is one outcome of the accepted R(A,B) contract.
type serverRelation int

const (
	serverRelUnproven serverRelation = iota
	serverRelSame
	// serverRelDistinct stays registered with an empty provable domain
	// — a future scoped identity proof may extend it; nothing derives
	// it from fold keys, characters, lengths, or tails.
	serverRelDistinct
)

// serverASCII reports whether every byte is ASCII — the registered
// SERVER case-insensitive subset is scoped to ASCII names.
func serverASCII(name string) bool {
	for i := 0; i < len(name); i++ {
		if name[i] >= 0x80 {
			return false
		}
	}
	return true
}

// serverFold folds ASCII A-Z only — the ordered channel's bucket key.
// Non-ASCII bytes pass through verbatim, so the key never loses
// identity for relation checks (and is still never an identity
// verdict).
func serverFold(name string) string {
	b := []byte(name)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c | 0x20
		}
	}
	return string(b)
}

// serverRel evaluates R(A,B) under the minimal conservative contract.
func serverRel(a, b string) serverRelation {
	if a == b {
		return serverRelSame
	}
	if serverASCII(a) && serverASCII(b) && serverFold(a) == serverFold(b) {
		return serverRelSame
	}
	return serverRelUnproven
}

// batchServerEntry is one ordered identity projection. barrier is the
// undischarged unknown effect — neither present nor absent promotable.
// absent is the proven OrderedAbsent: set only by the shared
// adjudication's publish decision on a provably-same identity (a
// proven drop or the approved convergent arm), never by the catalog
// and never while a barrier stands — the publishing statement's own
// arm rides along in publishedArm. present is the proven
// OrderedPresent: set only by a proven create on the provably-same
// identity, mutually exclusive with absent; presentConditional records
// that the publishing arm rested on the conditional privilege scope
// (T26-SERVER-CREATE-EXECUTION-01). A later unknown effect clears the
// ordered state back to the barrier shape rather than letting a stale
// answer survive a shadowing write. The verbatim authored name is kept
// for relation checks; the map key is the ASCII fold bucket, which two
// different authored names may share — the entry stores the first
// name bound and relation is evaluated against the stored name.
type batchServerEntry struct {
	name               string
	barrier            bool
	absent             bool
	publishedArm       spec.ServerDropArm
	present            bool
	presentConditional bool
}

// serverWrite records an unaudited SERVER write's authored identity —
// an unknown effect, never a consumable fact (T26-SERVER-DROP-01
// R3-A.3).
type serverWrite struct {
	name string
}

// serverEntryFor returns (creating on demand) the entry for the
// canonical fold bucket of name.
func (s *batchState) serverEntryFor(name string) *batchServerEntry {
	if s.serverEntries == nil {
		s.serverEntries = make(map[string]*batchServerEntry)
	}
	key := serverFold(name)
	entry, ok := s.serverEntries[key]
	if !ok {
		entry = &batchServerEntry{name: name}
		s.serverEntries[key] = entry
	}
	return entry
}

// resolveServerBarrier applies the accepted six-step order's barrier
// half: whole-batch contamination is answered by the caller's
// bound-unvouched path; channel taint and the relation scan both end
// here in an unknown effect, before any valid-proof consumption or
// provider access. A provably-same hit never short-circuits the scan —
// later unproven-relation writes may still apply. Returns true when
// the identity's bound state must be the batch-effects unknown shape
// (no resolver call, no catalog read).
func (s *batchState) resolveServerBarrier(name string) bool {
	if s.contaminated {
		return false
	}
	if s.serverChannelTainted {
		return true
	}
	for _, entry := range s.serverEntries {
		switch serverRel(name, entry.name) {
		case serverRelUnproven:
			return true
		case serverRelSame:
			if entry.barrier {
				return true
			}
		}
	}
	for _, write := range s.serverWrites {
		if serverRel(name, write.name) != serverRelDistinct {
			return true
		}
	}
	return false
}

// applyServerStatement is the SERVER family's ordered post-state
// transition (T26-SERVER-DROP-01). SERVER objects are instance-global —
// the family never enters the tracked schema/table model — and the
// channel's effects never spread to table/schema state.
//
//   - A proven refusal (TiDB vendor boundary) changes nothing: the
//     statement never executes, so the pre-state stands.
//   - An unaudited SERVER write (any boundary form) leaves a write
//     record: provable-identifier names record their identity and
//     taint every related entry; an unprovable string-literal name
//     taints the whole channel — later statements exit at step 2.
//   - An audited DROP SERVER on a provable identifier settles through
//     the shared adjudication attached at bind time: a publish arm
//     (proven drop or the approved IF EXISTS convergence) records the
//     proven OrderedAbsent with the statement's own arm; the terminal
//     refusal and guarded-absent arms never recreate the entry, so a
//     proven absent stands untouched; every other arm is an
//     undischarged unknown effect — it clears any earlier absent back
//     to the barrier shape (T26-SERVER-DROP-EXECUTION-01).
func (s *batchState) applyServerStatement(ctx context.Context, statement spec.Statement) error {
	decl := statement.DDL.Server
	if s.contaminated {
		return nil
	}
	if s.dialect != spec.DialectMySQL {
		return nil
	}
	if statement.Unsupported != nil {
		if !decl.NameIdent {
			s.serverChannelTainted = true
			for _, entry := range s.serverEntries {
				// A literal-name write may have recreated or removed
				// any server — every ordered answer on the channel
				// goes stale.
				entry.absent = false
				entry.publishedArm = 0
				entry.present = false
				entry.presentConditional = false
			}
			return nil
		}
		s.serverWrites = append(s.serverWrites, serverWrite{name: decl.Name})
		for _, entry := range s.serverEntries {
			if serverRel(decl.Name, entry.name) != serverRelDistinct {
				// An unproven-or-same write is an unknown effect on
				// this identity — a proven ordered answer never
				// survives it.
				entry.barrier = true
				entry.absent = false
				entry.publishedArm = 0
				entry.present = false
				entry.presentConditional = false
			}
		}
		return nil
	}
	if decl.Kind == spec.ServerKindDrop && decl.NameIdent {
		entry := s.serverEntryFor(decl.Name)
		adjudication := serverDropAdjudicationOf(statement)
		switch {
		case adjudication != nil && adjudication.PublishAbsent:
			// The shared adjudication proved the post-state absent —
			// record it with the publishing statement's own arm,
			// replacing any ordered present (T26-SERVER-CREATE-01).
			entry.absent = true
			entry.barrier = false
			entry.publishedArm = adjudication.Arm
			entry.present = false
			entry.presentConditional = false
		case adjudication != nil && (adjudication.Arm == spec.ServerDropArmTerminalRefusal ||
			adjudication.Arm == spec.ServerDropArmGuardedAbsent):
			// The refusal and guarded arms never recreate the object —
			// the proven absent stands untouched.
		default:
			// Every other arm is an undischarged unknown effect — a
			// stale ordered answer never survives it.
			entry.absent = false
			entry.publishedArm = 0
			entry.present = false
			entry.presentConditional = false
			entry.barrier = true
		}
	}
	if decl.Kind == spec.ServerKindAlter && decl.NameIdent {
		// Audited ALTER is presence-preserving: the documented write
		// set is the named row's option columns, which the
		// presence-only channel provably cannot change
		// (T26-SERVER-ALTER-EXECUTION-01, contract §D preservation
		// theorem). Every arm other than a publish is a no-op — A1
		// reports pre-existing shadowing state without minting an
		// entry, A2 leaves the consumed absent standing, and A4 on
		// unknown publishes nothing: no entry, no barrier, no
		// serverWrite. A nil adjudication (outside the audited slice
		// or a bypassed enrichment) takes the same no-op — ALTER can
		// never change tracked presence.
		adjudication := serverAlterAdjudicationOf(statement)
		if adjudication != nil && adjudication.PublishPresent {
			entry := s.serverEntryFor(decl.Name)
			entry.present = true
			// The published OrderedPresentConditional equals
			// ConditionalSource — never ConditionalAudit: the
			// statement's own consumable premise marks the audit
			// conclusion conditional, not the presence fact.
			entry.presentConditional = adjudication.ConditionalSource
			entry.absent = false
			entry.publishedArm = 0
			entry.barrier = false
		}
	}
	if decl.Kind == spec.ServerKindCreate && decl.NameIdent {
		adjudication := serverCreateAdjudicationOf(statement)
		switch {
		case adjudication != nil && adjudication.PublishPresent:
			// The shared adjudication proved the post-state present —
			// publish OrderedPresent on the provably-same entry,
			// replacing any ordered absent (T26-SERVER-CREATE-01).
			entry := s.serverEntryFor(decl.Name)
			entry.present = true
			entry.presentConditional = adjudication.PublishPresentConditional
			entry.absent = false
			entry.publishedArm = 0
			entry.barrier = false
		case adjudication != nil && adjudication.Arm == spec.ServerCreateArmTerminalRefusal:
			// The refusal never mutates the channel: an ordered-source
			// refusal leaves the consumed OrderedPresent standing; a
			// catalog-source refusal publishes nothing and records no
			// unknown effect — the objectCache already carries the
			// proven row for any later statement's own bind.
		default:
			// Every other arm — Unproven or a non-contaminated
			// Barrier — is an undischarged unknown effect on this
			// identity: a stale ordered answer never survives it.
			// (Contaminated batches already returned above, so entries
			// are never mutated on that path.)
			entry := s.serverEntryFor(decl.Name)
			entry.absent = false
			entry.publishedArm = 0
			entry.present = false
			entry.presentConditional = false
			entry.barrier = true
		}
	}
	return ctx.Err()
}
