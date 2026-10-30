// Ordered multi-pair RENAME adjudication shared by the batch-state
// transition layer and the rule layer so both judge one pair sequence by
// the same evolving work set.
package spec

// RenameEndpointClass is the existence knowledge one bound rename
// endpoint carries into pair adjudication.
type RenameEndpointClass int

const (
	// RenameEndpointUnknown marks an endpoint whose existence is
	// unvouched (provider-unresolved, ambiguous, or withheld).
	RenameEndpointUnknown RenameEndpointClass = iota
	// RenameEndpointAbsent marks a proven-absent endpoint.
	RenameEndpointAbsent
	// RenameEndpointPresent marks a proven-present endpoint.
	RenameEndpointPresent
)

// RenamePairVerdict reports one pair's effective endpoint classes at its
// own position inside the sequence.
type RenamePairVerdict struct {
	// Src is the source identity's class at pair time: a destination
	// produced by an earlier pair is present even when its bound
	// pre-state was absent, and a source consumed by an earlier pair is
	// absent even when it was produced earlier still.
	Src RenameEndpointClass
	// Dst is the destination identity's class at pair time under the
	// same last-write-wins work set.
	Dst RenameEndpointClass
	// SrcProduced reports that Src's presence came from an earlier
	// pair's produced destination — the produced shape, not the bound
	// pre-state entry, is what would migrate.
	SrcProduced bool
}

// RenamePairClasses evaluates declared rename pairs left-to-right over
// the evolving work set — the same adjudication the ordered-state layer
// and the rule layer must share. Each consumed source frees its slot and
// each produced destination occupies it, as a last-write disposition per
// identity: a name produced then consumed is absent at later positions,
// and a name consumed then produced is present again.
//
// srcKeys/dstKeys carry caller-computed comparable identities (schema,
// table, and namespace folded together); srcBase/dstBase carry each
// endpoint's bound pre-state class. Verdicts stop at the first pair that
// cannot migrate on existence alone — a proven-occupied destination, a
// proven-absent or unvouched source, or an unvouched destination —
// because pairs after it never execute natively.
func RenamePairClasses(srcKeys, dstKeys []string, srcBase, dstBase []RenameEndpointClass) []RenamePairVerdict {
	dispositions := make(map[string]RenameEndpointClass, len(srcKeys)*2)
	verdicts := make([]RenamePairVerdict, 0, len(srcKeys))
	for i := range srcKeys {
		srcNow, srcProduced := srcBase[i], false
		if d, ok := dispositions[srcKeys[i]]; ok {
			srcNow = d
			srcProduced = d == RenameEndpointPresent
		}
		dstNow := dstBase[i]
		if d, ok := dispositions[dstKeys[i]]; ok {
			dstNow = d
		}
		verdicts = append(verdicts, RenamePairVerdict{Src: srcNow, Dst: dstNow, SrcProduced: srcProduced})
		if dstNow == RenameEndpointPresent || srcNow != RenameEndpointPresent || dstNow != RenameEndpointAbsent {
			// A proven-occupied destination (1050), a proven-absent or
			// unvouched source (1146 / undecidable), or an unvouched
			// destination ends adjudication — later pairs never run.
			break
		}
		dispositions[srcKeys[i]] = RenameEndpointAbsent
		dispositions[dstKeys[i]] = RenameEndpointPresent
	}
	return verdicts
}
