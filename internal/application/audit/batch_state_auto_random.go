// Package audit orchestrates audit use cases at the application layer.
package audit

import (
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// batch_state_auto_random.go wires the shared ALTER-side AUTO_RANDOM and
// layout-option verdicts (spec.AlterAutoRandomColumnVerdict,
// spec.AlterLayoutOptionOutcome) into the ordered-state publisher: a
// proven refusal keeps the exact pre-state, an unproven premise poisons
// or unvouches only what it cannot prove, and an applicable declaration
// projects the catalog-effective post-state.

// alterAutoRandomGate runs the shared MODIFY/CHANGE verdict on the
// resolved old column. A proven refusal preserves the publication's
// pre-state; an undecidable premise — an unproven clustered member, an
// unobserved old column, or the runtime allocator overflow check —
// poisons the target; an untouched declaration falls through to the
// ordinary transition.
func (s *batchState) alterAutoRandomGate(shape *spec.TableSnapshot, oldName string, definition *spec.Column) spec.AutoRandomVerdict {
	if s.dialect != spec.DialectTiDB || definition == nil {
		return spec.AutoRandomOK
	}
	oldType, oldAutoInc := "", false
	oldFound := false
	for i := range shape.Columns {
		if strings.EqualFold(strings.TrimSpace(shape.Columns[i].Name), oldName) {
			oldType = shape.Columns[i].Type
			oldAutoInc = shape.Columns[i].AutoIncrement
			oldFound = true
			break
		}
	}
	member, memberKnown, shardBits, rangeBits, isPKHandle, hasAuto := spec.AlterAutoRandomOldFacts(shape, oldName)
	if member && !memberKnown && !hasAuto && !definition.AutoRandom {
		// Vacuous member question: the table provably carries no
		// AUTO_RANDOM, so the old bits are zero on both arms and the
		// untouched declaration needs no premise.
		memberKnown, shardBits, rangeBits = true, 0, 0
	}
	if !oldFound {
		// The old column itself is unproven — bits and flags stay
		// unknown unless the declaration is vacuous as above.
		memberKnown = memberKnown && oldFound
	}
	verdict, _ := spec.AlterAutoRandomColumnVerdict(shardBits, rangeBits, memberKnown,
		oldType, oldAutoInc, isPKHandle, definition)
	return verdict
}

// alterLayoutFacts builds the shared ALTER layout-option facts from the
// proven entry shape.
func (s *batchState) alterLayoutFacts(shape *spec.TableSnapshot) spec.AlterLayoutFacts {
	facts := spec.AlterLayoutFacts{
		Options:   shape.Options,
		Temporary: shape.Temporary || shape.GlobalTemporary,
	}
	if shape.PrimaryKeyIsHandle || shape.PrimaryKeyClustered {
		clustered := true
		facts.Clustered = &clustered
	} else if !shape.PrimaryKeyClusteredUnknown {
		// No clustered index: a proven-nonclustered PK or a proven
		// no-primary-key table both answer HasClusteredIndex=false.
		clustered := false
		facts.Clustered = &clustered
	}
	for _, column := range shape.Columns {
		if column.AutoRandom {
			facts.AutoRandomBits, facts.AutoRandomRangeBits = spec.AutoRandomEffectiveBits(column)
			facts.AutoRandomBitsKnown = true
			facts.AutoRandomPKHasUnsigned = column.Unsigned
			facts.AutoRandomPKUnsignedKnown = true
			break
		}
	}
	if !facts.AutoRandomBitsKnown {
		// The table-level catalog record survives unattributable member
		// shapes — the carrier's signedness then stays unproven.
		if bits, rng, ok := spec.AutoRandomBitsFromOptions(shape.Options); ok {
			facts.AutoRandomBits, facts.AutoRandomRangeBits = bits, rng
			facts.AutoRandomBitsKnown = true
		}
	}
	facts.HasPKColumn = shape.PrimaryKey != nil && len(shape.PrimaryKey.Columns) > 0
	return facts
}

// t23AlterLayoutOptionKeys are the extracted ALTER option names the
// shared layout verdict consumes (pre_split_regions included — its
// refusal is part of the same dispatch).
var t23AlterLayoutOptionKeys = []string{"auto_random_base", "auto_id_cache", "shard_row_id_bits", "pre_split_regions"}

// t23AlterHasLayoutOption reports whether the alter carries any of the
// modeled layout options.
func t23AlterHasLayoutOption(alter *spec.Alter) bool {
	for _, name := range t23AlterLayoutOptionKeys {
		if _, ok := alter.Options[name]; ok {
			return true
		}
	}
	return false
}
