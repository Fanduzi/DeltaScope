package audit

import (
	"strconv"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// batch_state_layout_r6.go publishes the ordered-state transitions of
// the T23-R6 TiDB physical-layout actions — CACHE/NOCACHE, ATTRIBUTES
// (table and partition level), and SET TIFLASH REPLICA — against the
// same shared verdicts the rules evaluate. A proven refusal preserves
// the exact pre-state; an undecidable premise unvouches only the facts
// it could have moved (issue #102).

const (
	layoutCacheStatusOption = "cache_status"
	layoutAttrsOption       = "attributes_present"
	layoutPartAttrsPrefix   = "partition_attributes_present."
	layoutTiFlashCountOpt   = "tiflash_replica_count"
	layoutTiFlashLabelsOpt  = "tiflash_replica_labels"
)

// alterLayoutR6Action reports whether the action belongs to the R6
// physical-layout publisher set.
func alterLayoutR6Action(action string) bool {
	switch action {
	case "cache", "nocache", "attributes", "partition_attributes", "set_tiflash_replica":
		return true
	}
	return false
}

// cachedTableGuardBlocks reports the upstream cached-table refusal: once
// a proven cache-enabled table sees an ALTER whose spec list is refused
// by spec.CachedTableGuardRefused, the statement preserves every target
// — no tombstone, no successor.
func (s *batchState) cachedTableGuardBlocks(alters []spec.Alter, targets []spec.Table) bool {
	specCount := spec.CacheGuardSpecCount(alters)
	for _, target := range targets {
		entry := s.entries[s.keyFor(s.schema, target)]
		if entry == nil || entry.state != tablePresent || entry.shape == nil {
			continue
		}
		facts := spec.AlterLayoutR6FactsForSnapshot(entry.shape)
		for i := range alters {
			if spec.CachedTableGuardRefused(alters[i].Action, specCount, facts) {
				return true
			}
		}
	}
	return false
}

// cachedTableGuardWithholds applies the same upstream gate to ALTER
// targets whose cache status is unobserved: the statement may have run
// (Disable) or been refused (non-Disable), so the shape unvouches to
// present-incomplete — presence itself is unaffected either way.
// Returns true when at least one target withheld the transition.
func (s *batchState) cachedTableGuardWithholds(alters []spec.Alter, targets []spec.Table) bool {
	specCount := spec.CacheGuardSpecCount(alters)
	needed := false
	for i := range alters {
		if spec.CacheGuardPremiseNeeded(alters[i].Action, specCount) {
			needed = true
			break
		}
	}
	if !needed {
		return false
	}
	// An ALTER RENAME unvouches the source name — the withheld source may
	// have moved away — plus a destination that was not proven live: a
	// rename onto a live name refuses natively (upstream checks endpoint
	// identity before the cache gate), so a proven-present destination
	// both keeps its identity and proves the whole statement refused —
	// the source's proven state stands then too.
	renameForm := len(targets) == 2 && specCount == 1 &&
		len(alters) > 0 && alters[0].Action == "rename_table"
	if renameForm {
		dst := s.entries[s.keyFor(s.schema, targets[1])]
		if dst != nil && dst.state == tablePresent {
			return false
		}
	}
	withheld := false
	for ti, target := range targets {
		key := s.keyFor(s.schema, target)
		entry := s.entries[key]
		if renameForm {
			if ti == 0 && entry != nil {
				s.invalidateKey(key)
				withheld = true
				continue
			}
			if entry != nil && entry.state != tablePresent {
				s.invalidateKey(key)
			}
			continue
		}
		if entry == nil || entry.state != tablePresent || entry.shape == nil {
			continue
		}
		facts := spec.AlterLayoutR6FactsForSnapshot(entry.shape)
		if facts.CacheStatusKnown {
			continue
		}
		s.entries[key] = presentIncompleteEntry(s.effectiveSchema(s.schema, target), target.Name)
		withheld = true
	}
	return withheld
}

// cacheGuardOutcome resolves the upstream `!= Disable` gate for one
// already-bound non-ALTER target — DROP/TRUNCATE/RENAME and the
// standalone index statements share it with the ALTER guard.
func (s *batchState) cacheGuardOutcome(entry *batchTableEntry) spec.AutoRandomVerdict {
	if s.dialect != spec.DialectTiDB || entry == nil || entry.state != tablePresent || entry.shape == nil {
		return spec.AutoRandomOK
	}
	verdict, _ := spec.CacheGuardOutcome(spec.AlterLayoutR6FactsForSnapshot(entry.shape))
	return verdict
}

// applyLayoutR6Alter adjudicates one R6 action per shared verdict and
// publishes only what the verdict proves.
func (s *batchState) applyLayoutR6Alter(alter *spec.Alter, targets []spec.Table) {
	key := s.keyFor(s.schema, targets[0])
	entry := s.entries[key]
	if entry == nil || entry.state == tableUnknown {
		s.entries[key] = presentIncompleteEntry(s.effectiveSchema(s.schema, targets[0]), targets[0].Name)
		return
	}
	if entry.state != tablePresent {
		s.invalidateKey(key)
		return
	}
	if entry.shape == nil {
		// Present-incomplete: the action's verdict needs shape facts the
		// entry does not carry — the statement can neither prove refusal
		// nor publish a successor, so the incomplete presence stands.
		return
	}
	facts := spec.AlterLayoutR6FactsForSnapshot(entry.shape)
	var verdict spec.AutoRandomVerdict
	switch alter.Action {
	case "cache", "nocache":
		verdict, _ = spec.AlterTableCacheOutcome(alter.Action, facts)
	case "attributes", "partition_attributes":
		verdict, _ = spec.AlterAttributesOutcome(alter.Attributes, alter.Partitions, facts)
	case "set_tiflash_replica":
		verdict, _ = spec.AlterTiFlashReplicaOutcome(alter.TiFlashReplica, facts)
	default:
		return
	}
	if verdict == spec.AutoRandomRefused {
		return
	}
	shape := cloneTableSnapshot(entry.shape)
	if shape.Options == nil {
		shape.Options = map[string]string{}
	}
	switch alter.Action {
	case "cache":
		if verdict == spec.AutoRandomUndecidable {
			delete(shape.Options, layoutCacheStatusOption)
		} else {
			shape.Options[layoutCacheStatusOption] = "enable"
		}
	case "nocache":
		if verdict == spec.AutoRandomUndecidable {
			delete(shape.Options, layoutCacheStatusOption)
		} else {
			shape.Options[layoutCacheStatusOption] = "disable"
		}
	case "attributes":
		// The authored value itself stays out of the published shape —
		// only the presence marker of the level's effect is vouched.
		if alter.Attributes != nil && alter.Attributes.Default {
			shape.Options[layoutAttrsOption] = "false"
		} else {
			shape.Options[layoutAttrsOption] = "true"
		}
	case "partition_attributes":
		present := "true"
		if alter.Attributes != nil && alter.Attributes.Default {
			present = "false"
		}
		for _, name := range alter.Partitions {
			shape.Options[layoutPartAttrsPrefix+strings.ToLower(name)] = present
		}
	case "set_tiflash_replica":
		switch {
		case verdict == spec.AutoRandomUndecidable:
			delete(shape.Options, layoutTiFlashCountOpt)
			delete(shape.Options, layoutTiFlashLabelsOpt)
		case alter.TiFlashReplica != nil && alter.TiFlashReplica.Hypo:
			// Hypothetical replicas mutate session state only — the
			// catalog definition is never touched.
		case alter.TiFlashReplica != nil:
			// count=0 removes the catalog record upstream
			// (TbInfo.TiFlashReplica=nil); "0" here is the proven-absent
			// marker, never a stored zero record.
			shape.Options[layoutTiFlashCountOpt] = strconv.FormatUint(alter.TiFlashReplica.Count, 10)
			if len(alter.TiFlashReplica.Labels) > 0 {
				shape.Options[layoutTiFlashLabelsOpt] = strings.Join(alter.TiFlashReplica.Labels, ",")
			} else {
				delete(shape.Options, layoutTiFlashLabelsOpt)
			}
		}
	}
	s.entries[key] = &batchTableEntry{state: tablePresent, shape: shape, displaySchema: entry.displaySchema, displayTable: entry.displayTable}
}
