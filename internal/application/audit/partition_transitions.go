// Package audit: T14 partition topology and maintenance transitions.
//
// applyPartitionOperation dispatches each partition ALTER action to its
// evidence-bounded successor. Every action resolves to one of three
// outcomes per bound target:
//   - proven refusal (pre-state stands untouched — no publication);
//   - proven execution (the modeled post-state publishes);
//   - unproven outcome (the affected member facts unvouch — the directory
//     goes nil, never a guessed member set — while unrelated structural
//     facts and the stable method/partitioned facts stay attached).
//
// Refusal evidence comes from the probed anchor matrix: TiDB refuses
// reorganize on hash|key methods (8200), all maintenance partition ops
// (8200), and nonconsecutive reorganize sources (8200); both products
// refuse partition ops on non-partitioned tables (1505), coalesce on
// range|list (1509), member-list violations (1507/1567/1517/1493/1495/
// 1508/1481/1492/1514-class by action and product).
package audit

import (
	"fmt"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// maxPartitionMembers is the native member bound (MySQL refuses more than
// 8192 partitions including subpartitions — error 1499). Implicit
// PARTITIONS n expansion never enumerates past it.
const maxPartitionMembers = 8192

// implicitPartitionName names the engine-assigned member pN — the implicit
// naming under `PARTITIONS n` on both products.
func implicitPartitionName(i int) string {
	return fmt.Sprintf("p%d", i)
}

// isTIDB reports the resolved product being TiDB.
func isTIDB(version *spec.VersionIdentity) bool {
	return version != nil && version.Product == spec.VersionProductTiDB
}

func isRangeFamily(method *string) bool {
	return method != nil && (*method == "range" || *method == "range columns")
}

func isListFamily(method *string) bool {
	return method != nil && (*method == "list" || *method == "list columns")
}

func isHashKeyFamily(method *string) bool {
	return method != nil && (*method == "hash" || *method == "key" ||
		*method == "linear hash" || *method == "linear key")
}

// unvouchPartitionMembers clears the member directory facts a proven-or-
// unproven outcome can no longer vouch — names, boundary defs, and catalog
// descriptions together so no consumer can read a stale member set. The
// method and partitioned facts are unaffected and stay.
func unvouchPartitionMembers(shape *spec.TableSnapshot) {
	shape.PartitionNames = nil
	shape.PartitionDefs = nil
	shape.PartitionDescriptions = nil
	shape.Subpartitioned = false
}

// unvouchPartitioning clears every partitioning fact — for actions whose
// unproven outcome could change whether the table is partitioned at all.
func unvouchPartitioning(shape *spec.TableSnapshot) {
	unvouchPartitionMembers(shape)
	shape.Partitioned = nil
	shape.PartitionType = nil
}

// applyPartitionOperation publishes the shared successor for a single-action
// partition ALTER. Member-level schema facts are invariant under every
// outcome — partitions never add/remove/rename columns, indexes, or
// constraints — while the member directory and the data-derived statistics
// follow each action's evidence-bounded transition.
func (s *batchState) applyPartitionOperation(targets []spec.Table, alter *spec.Alter) {
	for _, target := range targets {
		key := s.keyFor(s.schema, target)
		entry, ok := s.entries[key]
		if !ok || entry.state != tablePresent || entry.shape == nil {
			continue
		}
		next := s.applyPartitionAction(entry.shape, alter)
		if next == nil {
			// Proven refusal: the exact pre-state stands.
			continue
		}
		s.entries[key] = &batchTableEntry{state: tablePresent, shape: next, displaySchema: entry.displaySchema, displayTable: entry.displayTable}
	}
}

// applyPartitionAction clones the target shape and applies the action's
// transition, returning nil for a proven refusal.
func (s *batchState) applyPartitionAction(shape *spec.TableSnapshot, alter *spec.Alter) *spec.TableSnapshot {
	switch alter.Action {
	case "drop_partition":
		return s.transitionDropPartition(shape, alter)
	case "truncate_partition", "exchange_partition":
		out := cloneTableSnapshot(shape)
		clearMutationStats(out)
		clearModifyAffectedStats(out)
		return out
	case "add_partitions":
		return s.transitionAddPartitions(shape, alter)
	case "reorganize_partition":
		return s.transitionReorganizePartition(shape, alter)
	case "coalesce_partitions":
		return s.transitionCoalescePartitions(shape, alter)
	case "partition":
		return s.transitionPartitionBy(shape, alter)
	case "remove_partitioning":
		return s.transitionRemovePartitioning(shape)
	case "rebuild_partition", "check_partitions", "optimize_partition",
		"repair_partition", "import_partition_tablespace", "discard_partition_tablespace":
		return s.transitionPartitionMaintenance(shape, alter)
	default:
		return cloneTableSnapshot(shape)
	}
}

// transitionDropPartition keeps the T13 behavior — directory publication
// only when the action is proven applicable and the post-drop member set
// is fully provable — extended to carry the bound/description alignment
// through the removal.
func (s *batchState) transitionDropPartition(shape *spec.TableSnapshot, alter *spec.Alter) *spec.TableSnapshot {
	if shape.PartitionNames == nil {
		out := cloneTableSnapshot(shape)
		clearMutationStats(out)
		clearModifyAffectedStats(out)
		return out
	}
	// The drop is atomic: any named member absent from the proven
	// directory — or a drop consuming the whole member set — is a
	// proven native refusal under every method state (1507 /
	// last-partition 1508-class), and a proven inapplicable
	// method refuses outright (1512). Both keep the exact
	// pre-state: no stats cleared, no directory changed.
	members, provable := dropPartitionMembers(shape.PartitionNames, alter)
	outcome := partitionDropApplicability(shape.PartitionType)
	if !provable || outcome == partitionOutcomeRefused {
		return nil
	}
	out := cloneTableSnapshot(shape)
	if outcome == partitionOutcomeUnproven {
		// The method never answered — the drop may have executed
		// or refused, so the member directory unvouches while
		// the unaffected structural facts stay attached.
		unvouchPartitionMembers(out)
	} else {
		out.PartitionNames = members
		out.PartitionDefs = survivingPartitionDefs(shape.PartitionDefs, alter.Partitions)
		out.PartitionDescriptions = survivingPartitionDescriptions(shape, alter.Partitions)
	}
	clearMutationStats(out)
	clearModifyAffectedStats(out)
	return out
}

// survivingPartitionDefs drops the defs whose member name is in the removed
// set, preserving order and keeping nil == unobserved.
func survivingPartitionDefs(defs []spec.PartitionDef, removed []string) []spec.PartitionDef {
	if defs == nil {
		return nil
	}
	dropped := nameSet(removed)
	out := make([]spec.PartitionDef, 0, len(defs))
	for _, def := range defs {
		if !dropped[strings.ToLower(def.Name)] {
			out = append(out, def)
		}
	}
	return out
}

// survivingPartitionDescriptions keeps the catalog description cells of
// the surviving members, aligned by name through the defs list when
// available and by ordinal position otherwise.
func survivingPartitionDescriptions(shape *spec.TableSnapshot, removed []string) []string {
	if shape.PartitionDescriptions == nil || shape.PartitionNames == nil {
		return nil
	}
	dropped := nameSet(removed)
	out := make([]string, 0, len(shape.PartitionDescriptions))
	for i, name := range shape.PartitionNames {
		if dropped[strings.ToLower(name)] {
			continue
		}
		if i < len(shape.PartitionDescriptions) {
			out = append(out, shape.PartitionDescriptions[i])
		}
	}
	return out
}

func nameSet(names []string) map[string]bool {
	set := make(map[string]bool, len(names))
	for _, name := range names {
		set[strings.ToLower(name)] = true
	}
	return set
}

// partitionMemberIndexes locates each named source member in the proven
// directory, reporting every index plus whether all were found.
func partitionMemberIndexes(names, sources []string) ([]int, bool) {
	index := make(map[string]int, len(names))
	for i, name := range names {
		index[strings.ToLower(name)] = i
	}
	out := make([]int, 0, len(sources))
	for _, source := range sources {
		i, ok := index[strings.ToLower(source)]
		if !ok {
			return nil, false
		}
		out = append(out, i)
	}
	return out, true
}

// transitionReorganizePartition models REORGANIZE: the source span is
// replaced in place by the authored INTO members when every premise is
// provable — sources present and consecutive, no surviving-member name
// collision, and (on the range family) bounds that provably preserve the
// covered span. Proven refusals keep the pre-state; unprovable premises
// unvouch the member directory.
func (s *batchState) transitionReorganizePartition(shape *spec.TableSnapshot, alter *spec.Alter) *spec.TableSnapshot {
	version := s.resolvedVersion
	method := shape.PartitionType
	// Proven non-partitioned target refuses natively (1505).
	if shape.Partitioned != nil && !*shape.Partitioned {
		return nil
	}
	// TiDB refuses reorganization on the hash|key families (8200);
	// MySQL executes it as a name swap on all methods.
	if isTIDB(version) && isHashKeyFamily(method) {
		return nil
	}
	if shape.PartitionNames == nil || shape.PartitionDefs == nil || len(alter.PartitionDefs) == 0 {
		return s.partitionTopologyUnproven(shape, false)
	}
	indexes, found := partitionMemberIndexes(shape.PartitionNames, alter.Partitions)
	if !found {
		// A named source is provably absent — 1507 on MySQL, 1567 on TiDB.
		return nil
	}
	// Sources must sit consecutively — MySQL 1519, TiDB 8200.
	for i := 1; i < len(indexes); i++ {
		if indexes[i] != indexes[i-1]+1 {
			return nil
		}
	}
	surviving := nameSet(shape.PartitionNames)
	for _, idx := range indexes {
		delete(surviving, strings.ToLower(shape.PartitionNames[idx]))
	}
	// A name declared twice inside the INTO list refuses before any
	// surviving-member check — probed 1517 on both anchors.
	if firstDuplicateDefName(alter.PartitionDefs) != "" {
		return nil
	}
	for _, def := range alter.PartitionDefs {
		if surviving[strings.ToLower(def.Name)] {
			// 1517: target name collides with a member that survives.
			return nil
		}
	}
	if isRangeFamily(method) {
		if outcome, targets := reorganizeRangeOutcome(shape, alter, indexes); outcome != partitionOutcomeCombinable {
			if outcome == partitionOutcomeRefused {
				return nil
			}
			return s.partitionTopologyUnproven(shape, false)
		} else {
			return publishReorganizedSpan(shape, indexes[0], indexes[len(indexes)-1], targets)
		}
	}
	if isListFamily(method) {
		switch reorganizeListOutcome(shape, alter, indexes) {
		case partitionOutcomeRefused:
			return nil
		case partitionOutcomeUnproven:
			return s.partitionTopologyUnproven(shape, false)
		}
		return publishReorganizedSpan(shape, indexes[0], indexes[len(indexes)-1], alter.PartitionDefs)
	}
	if isHashKeyFamily(method) {
		// MySQL executes the name-swap reorganization (no bounds to
		// preserve); TiDB refuses (8200, gated above); an unproven product
		// cannot decide between those two answers.
		if version == nil {
			return s.partitionTopologyUnproven(shape, false)
		}
		return publishReorganizedSpan(shape, indexes[0], indexes[len(indexes)-1], alter.PartitionDefs)
	}
	// Method unobserved or outside the modeled vocabulary — the outcome
	// cannot be proven either way.
	return s.partitionTopologyUnproven(shape, false)
}

// reorganizeRangeOutcome proves the RANGE-family bound contract for one
// reorganization: strictly increasing comparable target bounds, the last
// target bound provably reaching the last source's upper edge (or
// extending it only when the source was the terminal member), and no
// MAXVALUE appearing before the last target member.
func reorganizeRangeOutcome(shape *spec.TableSnapshot, alter *spec.Alter, indexes []int) (partitionOutcome, []spec.PartitionDef) {
	targets := alter.PartitionDefs
	if len(targets) == 0 {
		return partitionOutcomeUnproven, nil
	}
	// Every target bound must be a comparable less_than row (or MAXVALUE
	// on the last member); any expression/unprojected bound leaves the
	// span unprovable.
	lastIdx := indexes[len(indexes)-1]
	sourceDefs := shape.PartitionDefs
	sourceLastRow, sourceLastMax, ok := spec.LessThanBoundUpper(sourceDefs[lastIdx].Bound)
	if !ok {
		return partitionOutcomeUnproven, nil
	}
	predecessorIdx := indexes[0] - 1
	var prevRow []spec.BoundLiteral
	prevMax := false
	if predecessorIdx >= 0 {
		prevRow, prevMax, ok = spec.LessThanBoundUpper(sourceDefs[predecessorIdx].Bound)
		if !ok {
			return partitionOutcomeUnproven, nil
		}
	}
	for i, def := range targets {
		row, maxValue, ok := spec.LessThanBoundUpper(def.Bound)
		if !ok {
			return partitionOutcomeUnproven, nil
		}
		lastTarget := i == len(targets)-1
		if maxValue && !lastTarget {
			// MAXVALUE mid-list — 1481-class refusal on both anchors.
			return partitionOutcomeRefused, nil
		}
		if maxValue {
			continue
		}
		// Strict increase against the previous target — or, on the
		// first target, against the surviving predecessor's bound.
		if i == 0 {
			if prevMax {
				// A predecessor ending at MAXVALUE followed by more
				// members is impossible; nothing to compare.
				continue
			}
			if predecessorIdx >= 0 {
				less, ok := spec.BoundRowLessThan(prevRow, row)
				if !ok {
					return partitionOutcomeUnproven, nil
				}
				if !less {
					return partitionOutcomeRefused, nil
				}
			}
			continue
		}
		prevTarget := targets[i-1]
		prevTargetRow, prevTargetMax, ok := spec.LessThanBoundUpper(prevTarget.Bound)
		if !ok || prevTargetMax {
			return partitionOutcomeUnproven, nil
		}
		less, lessOK := spec.BoundRowLessThan(prevTargetRow, row)
		if !lessOK {
			return partitionOutcomeUnproven, nil
		}
		if !less {
			return partitionOutcomeRefused, nil
		}
	}
	// The covered span's upper edge must be preserved — except the last
	// member's bound may extend when the source itself was the terminal
	// member.
	lastTarget := targets[len(targets)-1]
	lastRow, lastMax, ok := spec.LessThanBoundUpper(lastTarget.Bound)
	if !ok {
		return partitionOutcomeUnproven, nil
	}
	lastSourceIsTerminal := lastIdx == len(sourceDefs)-1
	if sourceLastMax {
		// A MAXVALUE source must be covered by a MAXVALUE last target.
		if !lastMax {
			return partitionOutcomeRefused, nil
		}
		return partitionOutcomeCombinable, targets
	}
	if lastMax {
		// Extending a non-MAXVALUE last member is allowed only when the
		// source was the terminal member.
		if lastSourceIsTerminal {
			return partitionOutcomeCombinable, targets
		}
		return partitionOutcomeRefused, nil
	}
	cmp, ok := spec.BoundRowCompare(lastRow, sourceLastRow)
	if !ok {
		return partitionOutcomeUnproven, nil
	}
	if lastSourceIsTerminal {
		if cmp < 0 {
			return partitionOutcomeRefused, nil
		}
		return partitionOutcomeCombinable, targets
	}
	if cmp != 0 {
		return partitionOutcomeRefused, nil
	}
	return partitionOutcomeCombinable, targets
}

// reorganizeListOutcome applies the LIST-family bound contract tri-state:
// a value provably shared with a surviving member (or inside the target
// list) is a proven refusal; every pair provably disjoint executes; any
// undecidable comparison leaves the outcome unproven — never folded into
// "no overlap".
func reorganizeListOutcome(shape *spec.TableSnapshot, alter *spec.Alter, indexes []int) partitionOutcome {
	removed := make(map[int]bool, len(indexes))
	for _, i := range indexes {
		removed[i] = true
	}
	unproven := false
	for i, def := range alter.PartitionDefs {
		// Duplicate values inside the new members refuse natively.
		for j := 0; j < i; j++ {
			if overlap, ok := spec.InBoundsOverlap(def.Bound, alter.PartitionDefs[j].Bound); ok && overlap {
				return partitionOutcomeRefused
			} else if !ok {
				unproven = true
			}
		}
		for m, existing := range shape.PartitionDefs {
			if removed[m] {
				continue
			}
			if overlap, ok := spec.InBoundsOverlap(def.Bound, existing.Bound); ok && overlap {
				return partitionOutcomeRefused
			} else if !ok {
				unproven = true
			}
		}
	}
	if unproven {
		return partitionOutcomeUnproven
	}
	return partitionOutcomeCombinable
}

// publishReorganizedSpan publishes the member directory after a proven
// REORGANIZE: the source span [first..last] is replaced by the authored
// targets at the first source's position, and the redistribution drops
// the data-derived statistics.
func publishReorganizedSpan(shape *spec.TableSnapshot, first, last int, targets []spec.PartitionDef) *spec.TableSnapshot {
	out := cloneTableSnapshot(shape)
	names := make([]string, 0, len(shape.PartitionNames)-(last-first+1)+len(targets))
	names = append(names, shape.PartitionNames[:first]...)
	for _, def := range targets {
		names = append(names, def.Name)
	}
	names = append(names, shape.PartitionNames[last+1:]...)
	out.PartitionNames = names
	defs := make([]spec.PartitionDef, 0, len(shape.PartitionDefs)-(last-first+1)+len(targets))
	defs = append(defs, shape.PartitionDefs[:first]...)
	defs = append(defs, spec.ClonePartitionDefs(targets)...)
	defs = append(defs, shape.PartitionDefs[last+1:]...)
	out.PartitionDefs = defs
	out.PartitionDescriptions = nil
	clearMutationStats(out)
	clearModifyAffectedStats(out)
	return out
}

// transitionAddPartitions models ADD PARTITION under the probed matrix:
//   - defs form executes on every method on both anchors — with the
//     range/list boundary contract (strictly increasing bounds, no add
//     past a MAXVALUE tail, no duplicate member name, no duplicate LIST
//     value) and the hash|key name-collision check;
//   - `PARTITIONS n` executes on hash|key and refuses on range|list
//     (1492); the implicit member names pN..pN+n-1 publish only when the
//     existing names are provably the canonical p0..p(k-1) set — otherwise
//     the engine-assigned names are unproven;
//   - the bare form refuses on MySQL (1492 range|list, 1514 hash|key) and
//     on TiDB range|list (1492) but executes on TiDB hash|key.
func (s *batchState) transitionAddPartitions(shape *spec.TableSnapshot, alter *spec.Alter) *spec.TableSnapshot {
	version := s.resolvedVersion
	if shape.Partitioned != nil && !*shape.Partitioned {
		return nil
	}
	method := shape.PartitionType
	if len(alter.PartitionDefs) > 0 {
		return s.applyAddPartitionDefs(shape, alter, method)
	}
	if alter.PartitionCount != nil {
		return s.applyAddPartitionCount(shape, *alter.PartitionCount, method)
	}
	// Bare `ADD PARTITION` — refused on range|list on both anchors (1492);
	// on hash|key MySQL refuses (1514) while TiDB silently no-ops (rc=0,
	// directory unchanged — probed supp-bare-add-hash).
	if isRangeFamily(method) || isListFamily(method) {
		return nil
	}
	if isHashKeyFamily(method) {
		if isTIDB(version) {
			return cloneTableSnapshot(shape)
		}
		if version != nil {
			return nil
		}
	}
	return s.partitionTopologyUnproven(shape, false)
}

// applyAddPartitionDefs publishes the defs-form ADD successor under the
// proven checks — or nil when a proven refusal applies.
func (s *batchState) applyAddPartitionDefs(shape *spec.TableSnapshot, alter *spec.Alter, method *string) *spec.TableSnapshot {
	if shape.PartitionNames == nil || shape.PartitionDefs == nil {
		return s.partitionTopologyUnproven(shape, false)
	}
	// A name declared twice inside the ADD list refuses natively (1517)
	// before any existing-member check runs.
	if firstDuplicateDefName(alter.PartitionDefs) != "" {
		return nil
	}
	existing := nameSet(shape.PartitionNames)
	for _, def := range alter.PartitionDefs {
		if existing[strings.ToLower(def.Name)] {
			// 1517: duplicate partition name.
			return nil
		}
	}
	if isRangeFamily(method) {
		// Pairwise adjudication under the shared contract: a provable
		// non-increase — including a member after a MAXVALUE edge —
		// refuses the whole statement; pairs the literal domain cannot
		// decide only degrade the outcome to unproven, so a later
		// provable violation still refuses rather than being swallowed
		// by the earlier unprojectable member.
		undecidable := false
		var prev *spec.PartitionBound
		if n := len(shape.PartitionDefs); n > 0 {
			prev = shape.PartitionDefs[n-1].Bound
		}
		for _, def := range alter.PartitionDefs {
			if prev != nil {
				violates, ok := spec.LessThanPairOutcome(prev, def.Bound)
				if !ok {
					undecidable = true
				} else if violates {
					return nil
				}
			}
			prev = def.Bound
		}
		if undecidable {
			return s.partitionTopologyUnproven(shape, false)
		}
	}
	if isListFamily(method) {
		unproven := false
		for i, def := range alter.PartitionDefs {
			for j := 0; j < i; j++ {
				if overlap, ok := spec.InBoundsOverlap(def.Bound, alter.PartitionDefs[j].Bound); ok && overlap {
					return nil
				} else if !ok {
					unproven = true
				}
			}
			for _, existing := range shape.PartitionDefs {
				if overlap, ok := spec.InBoundsOverlap(def.Bound, existing.Bound); ok && overlap {
					return nil
				} else if !ok {
					unproven = true
				}
			}
		}
		if unproven {
			return s.partitionTopologyUnproven(shape, false)
		}
	}
	out := cloneTableSnapshot(shape)
	out.PartitionNames = append(append([]string{}, shape.PartitionNames...), partitionDefNames(alter.PartitionDefs)...)
	out.PartitionDefs = append(spec.ClonePartitionDefs(shape.PartitionDefs), spec.ClonePartitionDefs(alter.PartitionDefs)...)
	out.PartitionDescriptions = nil
	clearMutationStats(out)
	return out
}

// partitionDefNames projects the member names out of a def list.
func partitionDefNames(defs []spec.PartitionDef) []string {
	names := make([]string, 0, len(defs))
	for _, def := range defs {
		names = append(names, def.Name)
	}
	return names
}

// firstDuplicateDefName reports the first member name declared twice in
// one definition list — case-insensitive, matching the native 1517
// same-batch duplicate refusal. "" means the authored names are unique.
// The audit package keeps its own copy because the rules layer cannot
// import application internals; both sides must stay in lockstep.
func firstDuplicateDefName(defs []spec.PartitionDef) string {
	seen := make(map[string]bool, len(defs))
	for _, def := range defs {
		key := strings.ToLower(def.Name)
		if seen[key] {
			return def.Name
		}
		seen[key] = true
	}
	return ""
}

// applyAddPartitionCount publishes the `PARTITIONS n` (or TiDB bare-ADD)
// successor on the hash|key families: the engine appends implicit
// members p{next}.., which is provable only when the observed member set
// is the canonical p0..p(k-1) sequence.
func (s *batchState) applyAddPartitionCount(shape *spec.TableSnapshot, count uint64, method *string) *spec.TableSnapshot {
	if isRangeFamily(method) || isListFamily(method) {
		return nil // 1492 on both anchors.
	}
	if !isHashKeyFamily(method) {
		return s.partitionTopologyUnproven(shape, false)
	}
	if shape.PartitionNames == nil || shape.PartitionDefs == nil {
		return s.partitionTopologyUnproven(shape, false)
	}
	base := len(shape.PartitionNames)
	if base == 0 || base > maxPartitionMembers {
		return s.partitionTopologyUnproven(shape, false)
	}
	// The overflow judgment stays in the unsigned domain — int(count)
	// would wrap past MaxInt64 and smuggle a refusal through as a
	// successful (or silently truncated) append. `count == 0` folds into
	// the bare form at the extractor, so any zero reaching here is an
	// anomaly the refusal also covers.
	if count == 0 || count > uint64(maxPartitionMembers-base) {
		return nil
	}
	for i, name := range shape.PartitionNames {
		if name != implicitPartitionName(i) {
			// The engine's implicit naming is provable only against the
			// canonical sequence — a renamed member space leaves the
			// appended names unproven.
			return s.partitionTopologyUnproven(shape, false)
		}
	}
	out := cloneTableSnapshot(shape)
	for i := 0; i < int(count); i++ {
		name := implicitPartitionName(base + i)
		out.PartitionNames = append(out.PartitionNames, name)
		out.PartitionDefs = append(out.PartitionDefs, spec.PartitionDef{Name: name})
	}
	out.PartitionDescriptions = nil
	clearMutationStats(out)
	return out
}

// transitionCoalescePartitions models COALESCE PARTITION n on the hash|key
// families: the last n members are removed. Range|list refuse (1509),
// n consuming the whole member set refuses (1508), and an unobserved or
// non-countable member space unvouches.
func (s *batchState) transitionCoalescePartitions(shape *spec.TableSnapshot, alter *spec.Alter) *spec.TableSnapshot {
	if shape.Partitioned != nil && !*shape.Partitioned {
		return nil
	}
	method := shape.PartitionType
	if isRangeFamily(method) || isListFamily(method) {
		return nil
	}
	if !isHashKeyFamily(method) {
		return s.partitionTopologyUnproven(shape, false)
	}
	if alter.PartitionCount == nil || shape.PartitionNames == nil {
		return s.partitionTopologyUnproven(shape, false)
	}
	count := *alter.PartitionCount
	if count == 0 {
		// The authored zero survives extraction — a probed 1515 refusal,
		// never an unknown count.
		return nil
	}
	if count >= uint64(len(shape.PartitionNames)) {
		return nil // 1508: cannot remove all partitions.
	}
	out := cloneTableSnapshot(shape)
	keep := len(shape.PartitionNames) - int(count)
	out.PartitionNames = out.PartitionNames[:keep]
	if out.PartitionDefs != nil {
		if len(out.PartitionDefs) > keep {
			out.PartitionDefs = out.PartitionDefs[:keep]
		}
	}
	out.PartitionDescriptions = nil
	clearMutationStats(out)
	return out
}

// transitionPartitionBy models ALTER ... PARTITION BY: the table takes the
// newly declared scheme wholesale (its method, member directory, and
// subpartition mark). The action executes on partitioned and plain tables
// alike; only an unprojectable scheme leaves the partition facts
// unvouched.
func (s *batchState) transitionPartitionBy(shape *spec.TableSnapshot, alter *spec.Alter) *spec.TableSnapshot {
	scheme := alter.PartitionScheme
	if scheme == nil {
		return s.partitionTopologyUnproven(shape, true)
	}
	// The shared scheme-validity judgment — duplicate member names,
	// over-bound/method-mismatched counts, and provably-invalid declared
	// bounds all refuse natively; uncomparable bounds leave the whole
	// scheme outcome unproven.
	switch partitionSchemeOutcome(scheme.Type, scheme.Defs, scheme.Count) {
	case partitionOutcomeRefused:
		return nil
	case partitionOutcomeUnproven:
		return s.partitionTopologyUnproven(shape, true)
	}
	out := cloneTableSnapshot(shape)
	partitioned := true
	out.Partitioned = &partitioned
	out.Subpartitioned = scheme.Subpartitioned
	if scheme.Type != "" {
		method := scheme.Type
		out.PartitionType = &method
	}
	out.PartitionDescriptions = nil
	switch {
	case scheme.Subpartitioned || scheme.Interval:
		// The leaf member space is unprovable under subpartitioning or
		// interval generation — directory stays unobserved.
		out.PartitionNames = nil
		out.PartitionDefs = nil
	case len(scheme.Defs) > 0:
		out.PartitionDefs = spec.ClonePartitionDefs(scheme.Defs)
		out.PartitionNames = partitionDefNames(scheme.Defs)
	case scheme.Count != nil && isHashKeyMethodName(scheme.Type):
		// The count is provably in-range here — the shared outcome
		// already refused >8192 and every non-hash|key method.
		count := int(*scheme.Count)
		out.PartitionNames = make([]string, 0, count)
		out.PartitionDefs = make([]spec.PartitionDef, 0, count)
		for i := 0; i < count; i++ {
			name := implicitPartitionName(i)
			out.PartitionNames = append(out.PartitionNames, name)
			out.PartitionDefs = append(out.PartitionDefs, spec.PartitionDef{Name: name})
		}
	case isHashKeyMethodName(scheme.Type):
		// HASH/KEY without PARTITIONS n defaults to a single member.
		out.PartitionNames = []string{"p0"}
		out.PartitionDefs = []spec.PartitionDef{{Name: "p0"}}
	default:
		// An uncountable scheme shape keeps the member space unobserved.
		out.PartitionNames = nil
		out.PartitionDefs = nil
	}
	clearMutationStats(out)
	clearModifyAffectedStats(out)
	return out
}

// isHashKeyMethodName reports the hash|key family on a normalized method
// string (no pointer indirection for scheme fields).
func isHashKeyMethodName(method string) bool {
	return method == "hash" || method == "key" ||
		method == "linear hash" || method == "linear key"
}

// partitionSchemeOutcome is the shared declared-scheme validity judgment
// ALTER PARTITION BY and CREATE TABLE both consume: same-batch duplicate
// member names refuse (1517), a `PARTITIONS n` past the native bound or
// declared on a bound-carrying method refuses (1499 / 1492-class), and
// provably-invalid declared bounds refuse (1493/1495-class). Uncomparable
// content leaves the scheme unproven — never a guessed verdict.
func partitionSchemeOutcome(method string, defs []spec.PartitionDef, count *uint64) partitionOutcome {
	if firstDuplicateDefName(defs) != "" {
		return partitionOutcomeRefused
	}
	if len(defs) > 0 {
		return declaredSchemeOutcome(method, defs)
	}
	if count != nil {
		if *count > maxPartitionMembers {
			return partitionOutcomeRefused
		}
		if method == "range" || method == "range columns" ||
			method == "list" || method == "list columns" {
			// A count on a bound-carrying method is the parser-refused
			// 1492-class shape; the guard stays honest even where the
			// grammar never produces it.
			return partitionOutcomeRefused
		}
	}
	return partitionOutcomeCombinable
}

// declaredSchemeOutcome proves the declared member bounds of a fresh
// partitioning scheme: the RANGE family requires strictly increasing
// comparable bounds with MAXVALUE only on the last member (1493-class);
// the LIST family refuses provably-duplicate literal values. Uncomparable
// items keep the outcome unproven; hash|key members carry no bounds and
// always pass.
func declaredSchemeOutcome(method string, defs []spec.PartitionDef) partitionOutcome {
	switch method {
	case "range", "range columns":
		// Two premises accumulate separately: (A) every member's own
		// bound must project to comparable literals — a lone member has
		// no comparison partner yet still owes a decidable bound, so nil
		// bounds, empty shells, and expression items are undecidable
		// rather than auto-passing; and (B) pairwise ordering — a
		// provable non-increase, including a member after a MAXVALUE
		// edge, refuses the scheme while undecidable pairs degrade the
		// outcome without swallowing a later provable violation.
		undecidable := false
		var prev *spec.PartitionBound
		for _, def := range defs {
			if !spec.BoundComparable(def.Bound) {
				undecidable = true
			}
			if prev != nil {
				// The pair check runs independently of the current
				// member's own comparability: a proven MAXVALUE
				// predecessor refuses whatever follows, even an
				// unprojectable bound.
				violates, ok := spec.LessThanPairOutcome(prev, def.Bound)
				if !ok {
					undecidable = true
				} else if violates {
					return partitionOutcomeRefused
				}
			}
			prev = def.Bound
		}
		if undecidable {
			return partitionOutcomeUnproven
		}
	case "list", "list columns":
		unproven := false
		for i, def := range defs {
			if !spec.BoundComparable(def.Bound) {
				unproven = true
			}
			for j := 0; j < i; j++ {
				if overlap, ok := spec.InBoundsOverlap(def.Bound, defs[j].Bound); ok && overlap {
					return partitionOutcomeRefused
				} else if !ok {
					unproven = true
				}
			}
		}
		if unproven {
			return partitionOutcomeUnproven
		}
	}
	return partitionOutcomeCombinable
}

// transitionRemovePartitioning models REMOVE PARTITIONING: a proven-
// partitioned table becomes plain (member directory empties, method and
// subpartition marks clear); a proven non-partitioned table refuses
// (1505); an unproven partition state unvouches all partition facts.
func (s *batchState) transitionRemovePartitioning(shape *spec.TableSnapshot) *spec.TableSnapshot {
	if shape.Partitioned == nil {
		return s.partitionTopologyUnproven(shape, true)
	}
	if !*shape.Partitioned {
		return nil
	}
	out := cloneTableSnapshot(shape)
	partitioned := false
	out.Partitioned = &partitioned
	out.PartitionType = nil
	out.PartitionNames = []string{}
	out.PartitionDefs = nil
	out.PartitionDescriptions = nil
	out.Subpartitioned = false
	clearMutationStats(out)
	return out
}

// transitionPartitionMaintenance models the maintenance operations:
//   - TiDB refuses all of them (8200) — the exact pre-state stands.
//   - MySQL executes them without touching the member directory. The op
//     that provably cannot do its work — a named member absent (1507
//     rebuild, 1735 discard/import; soft-fail for check/optimize/repair),
//     or a proven non-partitioned target — changes nothing.
//   - Statistics follow the probed data effect: CHECK keeps everything,
//     REBUILD preserves data (stats kept), OPTIMIZE re-analyzes (stats
//     refreshed → counters unvouch), REPAIR maps to rebuild under InnoDB
//     and clears otherwise, DISCARD/IMPORT move or destroy member data
//     (mutation stats unvouch). An unproven member directory or product
//     unvouches the stats the op could have touched — never the member
//     facts.
func (s *batchState) transitionPartitionMaintenance(shape *spec.TableSnapshot, alter *spec.Alter) *spec.TableSnapshot {
	if isTIDB(s.resolvedVersion) {
		return nil
	}
	if shape.Partitioned != nil && !*shape.Partitioned {
		// Refused (rebuild/discard/import) or operation-failed
		// (check/optimize/repair) — neither changes the shape.
		return nil
	}
	if (alter.Action == "discard_partition_tablespace" || alter.Action == "import_partition_tablespace") &&
		shape.Options != nil {
		// A proven MyISAM table refuses the partition tablespace ops —
		// the storage engine does not have the option (probed 1031 on
		// MySQL 5.7; the refusal is engine-level, not version-level).
		if engine, ok := shape.Options["engine"]; ok && strings.EqualFold(engine, "myisam") {
			return nil
		}
	}
	memberDirProven := shape.PartitionNames != nil
	if !alter.AllPartitions && len(alter.Partitions) > 0 && memberDirProven {
		if _, found := partitionMemberIndexes(shape.PartitionNames, alter.Partitions); !found {
			return nil
		}
	}
	out := cloneTableSnapshot(shape)
	switch alter.Action {
	case "check_partitions":
		// CHECK is read-only — no statistic moves.
	case "rebuild_partition":
		// REBUILD copies the member's data into a fresh segment — the
		// row-bearing facts stay valid; nothing unvouches.
	case "optimize_partition":
		// OPTIMIZE rebuilds and re-analyzes — refreshed statistics can no
		// longer vouch for the observed counters.
		clearModifyAffectedStats(out)
	case "repair_partition":
		// REPAIR maps to a rebuild on InnoDB; any other engine keeps the
		// outcome unproven, so the touched counters unvouch.
		if engine, ok := shape.Options["engine"]; !ok || !strings.EqualFold(engine, "innodb") {
			clearModifyAffectedStats(out)
		}
	case "discard_partition_tablespace", "import_partition_tablespace":
		clearMutationStats(out)
		clearModifyAffectedStats(out)
	}
	if !memberDirProven && len(alter.Partitions) > 0 && !alter.AllPartitions {
		// A member list the op may or may not have matched — the op's
		// own stat effect is unproven, so every counter it could have
		// touched unvouches on top of the action's proven baseline.
		switch alter.Action {
		case "rebuild_partition", "check_partitions":
			// Nothing provable changes either way.
		default:
			clearModifyAffectedStats(out)
		}
	}
	return out
}

// partitionTopologyUnproven publishes the unproven-outcome successor: the
// member directory (and for scheme-changing actions the whole partitioning
// fact set) unvouches while unrelated structural facts stay attached.
func (s *batchState) partitionTopologyUnproven(shape *spec.TableSnapshot, schemeChanging bool) *spec.TableSnapshot {
	out := cloneTableSnapshot(shape)
	if schemeChanging {
		unvouchPartitioning(out)
	} else {
		unvouchPartitionMembers(out)
	}
	clearMutationStats(out)
	clearModifyAffectedStats(out)
	return out
}
