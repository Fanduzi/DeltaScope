// Package audit: T26-SERVER-DROP-01 — the MySQL SERVER ordered channel.
// This slice ships the protective layer only: unaudited-write records,
// channel taint, relation-checked barriers, and the six-step resolve
// order whose every barrier decision completes before any provider or
// cache access. There is no OrderedAbsent representation and no proven
// absent publish path — those stay the accepted future premise-supply
// stage contract (docs/verification/t26-semantics-01, accepted
// b8a67e7b), so an audited DROP SERVER with an unproven execution
// premise never promotes absent; it leaves an unproven barrier that
// shadows the audit-time catalog present for later statements.
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

// batchServerEntry is one ordered identity projection. This slice only
// ever holds the barrier flag — an undischarged unknown effect on the
// identity: neither present nor absent is promotable. The verbatim
// authored name is kept for relation checks; the map key is the ASCII
// fold bucket, which two different authored names may share — the
// entry stores the first name bound and relation is evaluated against
// the stored name, so collisions can only produce the same barrier
// outcome this slice.
type batchServerEntry struct {
	name    string
	barrier bool
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
//   - An audited DROP SERVER on a provable identifier leaves the
//     unproven barrier on its own identity: the execution premise is
//     unproven, so the statement may have deleted the object or never
//     run — the audit-time catalog present stays shadowed and no
//     absent publishes (accepted transition rule).
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
			return nil
		}
		s.serverWrites = append(s.serverWrites, serverWrite{name: decl.Name})
		for _, entry := range s.serverEntries {
			if serverRel(decl.Name, entry.name) != serverRelDistinct {
				entry.barrier = true
			}
		}
		return nil
	}
	if decl.Kind == spec.ServerKindDrop && decl.NameIdent {
		s.serverEntryFor(decl.Name).barrier = true
	}
	return ctx.Err()
}
