package spec

import "strings"

// PlacementOutcome is the bounded tri-state answer for one placement
// lifecycle or binding transition after TiDB's semantics are applied:
// refused keeps the exact pre-state, applied publishes, unproven withholds.
type PlacementOutcome int

const (
	PlacementRefused PlacementOutcome = iota
	PlacementApplied
	PlacementUnproven
)

// PlacementOption is one authored CREATE/ALTER PLACEMENT POLICY member
// normalized to a bounded key name plus its parsed value. Findings expose
// the key names only — constraint bodies never appear in findings.
type PlacementOption struct {
	Key       string  `json:"key"`
	StrValue  string  `json:"str_value,omitempty"`
	UintValue *uint64 `json:"uint_value,omitempty"`
}

// PlacementBinding is one authored PLACEMENT POLICY assignment. Policy
// carries the bound policy name; Default marks the `= DEFAULT` unbind form.
type PlacementBinding struct {
	Policy  string `json:"policy,omitempty"`
	Default bool   `json:"default,omitempty"`
}

// PlacementPolicyFact is the catalog fact for one placement policy: its
// proven existence plus the bounded bound-object list (the proven inbound
// references TiDB consults for the 8241 drop refusal). nil members never
// appear in the map — presence in the map itself is the existence answer.
type PlacementPolicyFact struct {
	PrimaryRegion string   `json:"primary_region,omitempty"`
	Regions       string   `json:"regions,omitempty"`
	BoundTargets  []string `json:"bound_targets,omitempty"`
}

// PlacementTopologyFacts is the bounded, proven observation of the
// topology prerequisites behind a placement action — a different evidence
// layer from the catalog: where the policy *could physically deploy*.
// A nil Metadata.PlacementTopology means topology was never read; rules
// surface that absence as a structured evidence gap rather than implying
// verified deployability (issue #99: 拓扑未知给gap而非猜测可部署).
//
// The projection is deliberately bounded: label keys/values and scheduling
// state are provider-visible facts, never constraint bodies copied from
// authored SQL, and the whole structure stays off public JSON (json:"-").
type PlacementTopologyFacts struct {
	// StoreLabels is the proven label universe (SHOW PLACEMENT LABELS):
	// label key → observed values across stores. nil means labels were
	// never read.
	StoreLabels map[string][]string `json:"-"`
	// SchedulingState is the bound target's proven SCHEDULING_STATE
	// (PENDING|INPROGRESS|SCHEDULED) when observed. Empty means not
	// observed — and PENDING is an observed pending state, never itself
	// proof of satisfaction or failure.
	SchedulingState string `json:"-"`
}

// PlacementBindOutcome adjudicates `PLACEMENT POLICY = <name|DEFAULT>`
// against policy existence. DEFAULT is the unconditional unbind and always
// applies; a named policy applies only when existence is proven (native
// 8239 refuses a proven-absent name, an unvouched policy set stays
// unproven). MySQL is the vendor boundary — a parsed placement clause is
// refused, never silently applied.
func PlacementBindOutcome(dialect Dialect, binding PlacementBinding, policyExists *bool) PlacementOutcome {
	if dialect != DialectTiDB {
		return PlacementRefused
	}
	if binding.Default {
		return PlacementApplied
	}
	if strings.TrimSpace(binding.Policy) == "" {
		return PlacementUnproven
	}
	if policyExists == nil {
		return PlacementUnproven
	}
	if !*policyExists {
		return PlacementRefused
	}
	return PlacementApplied
}

// PlacementCreateOutcome adjudicates CREATE PLACEMENT POLICY against the
// proven policy set: a proven-present name refuses the bare create and
// no-ops under IF NOT EXISTS or lands under OR REPLACE; an unvouched set
// stays unproven. or_replace semantics are modeled as a replacement land
// (bound objects keep the name — probed), never as a fresh object.
func PlacementCreateOutcome(dialect Dialect, orReplace, ifNotExists bool, policyExists *bool) PlacementOutcome {
	if dialect != DialectTiDB {
		return PlacementRefused
	}
	if orReplace {
		return PlacementApplied
	}
	if policyExists == nil {
		return PlacementUnproven
	}
	if *policyExists {
		if ifNotExists {
			return PlacementApplied
		}
		return PlacementRefused
	}
	return PlacementApplied
}

// PlacementAlterOutcome adjudicates ALTER PLACEMENT POLICY: the target
// policy must provably exist (native refuses an absent name), while an
// unvouched policy set stays unproven.
func PlacementAlterOutcome(dialect Dialect, policyExists *bool) PlacementOutcome {
	if dialect != DialectTiDB {
		return PlacementRefused
	}
	if policyExists == nil {
		return PlacementUnproven
	}
	if !*policyExists {
		return PlacementRefused
	}
	return PlacementApplied
}

// PlacementDropOutcome adjudicates DROP PLACEMENT POLICY: IF EXISTS on a
// proven-absent name is a no-op land, a bare drop of a proven-absent name
// refuses, and a proven in-use policy refuses (8241) while a proven
// bound-empty set lands. Any unvouched bound set stays unproven.
func PlacementDropOutcome(dialect Dialect, ifExists bool, policyExists *bool, boundTargets *[]string) PlacementOutcome {
	if dialect != DialectTiDB {
		return PlacementRefused
	}
	if policyExists == nil || boundTargets == nil {
		return PlacementUnproven
	}
	if !*policyExists {
		if ifExists {
			return PlacementApplied
		}
		return PlacementRefused
	}
	if len(*boundTargets) > 0 {
		return PlacementRefused
	}
	return PlacementApplied
}

// PlacementRangeOutcome resolves the authored ALTER RANGE scope name to
// the bounded set probed on TiDB v8.5.0: `global` and `meta` land while
// `dist`, `region`, and every other spelling refuse natively (1105).
// The bind premise itself is checked by PlacementBindOutcome.
func PlacementRangeOutcome(dialect Dialect, rangeName string) PlacementOutcome {
	if dialect != DialectTiDB {
		return PlacementRefused
	}
	switch strings.ToLower(strings.TrimSpace(rangeName)) {
	case "global", "meta":
		return PlacementApplied
	case "":
		return PlacementUnproven
	default:
		return PlacementRefused
	}
}
