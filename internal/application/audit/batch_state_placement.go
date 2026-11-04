package audit

import (
	"sort"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// statementsNeedPlacementCatalog reports whether any statement carries a
// placement fact the catalog could adjudicate — a policy lifecycle
// operation, a statement-level binding, an alter placement action, or a
// member-level declaration.
func statementsNeedPlacementCatalog(statements []spec.Statement) bool {
	for _, statement := range statements {
		ddl := statement.DDL
		if ddl == nil {
			continue
		}
		switch ddl.Operation {
		case spec.DDLOperationCreatePlacementPolicy,
			spec.DDLOperationAlterPlacementPolicy,
			spec.DDLOperationDropPlacementPolicy,
			spec.DDLOperationAlterRangePlacement:
			return true
		}
		if _, ok := ddl.Options["placement_policy"]; ok {
			return true
		}
		// Every authored placement assignment triggers the one-shot
		// catalog load — direct binds, member definitions, alter member
		// defs and partition-scheme defs alike, DEFAULT retained so its
		// effective unbind state stays adjudicable (T20-R4).
		if len(spec.CollectPlacementBindings(ddl)) > 0 {
			return true
		}
	}
	return false
}

// Placement-policy ordered state (issue #99 T20). Three fact dimensions stay
// separate: the policy object's own existence/attrs, the bound-object
// reference set (the 8241 drop premise), and each bound object's own
// projection (table PlacementPolicy, schema attrs, range bindings). A proven
// refusal preserves exact pre-state; an unproven outcome retracts only the
// affected dimension — never fabricates a bind or a drop.

// placementPolicyState is the tri-state existence answer for one policy.
type placementPolicyState int

const (
	placementPolicyUnknown placementPolicyState = iota
	placementPolicyPresent
	placementPolicyAbsent
)

type batchPlacementEntry struct {
	state placementPolicyState
	// attrs carries proven option values keyed by bounded member name.
	attrs map[string]string
	// refs is the set of individually proven inbound-reference identities:
	// "table:<schema>.<name>", "partition:<schema>.<name>.<part>",
	// "schema:<name>", "range:<name>". refsKnown reports whether the set is
	// exhaustive; a non-exhaustive set still retains sufficient positive
	// proof for a refused DROP, while an empty unread set under
	// refsKnown=false authorizes nothing — never read it as proven-empty
	// (issue #99 T20-R4).
	refs      map[string]struct{}
	refsKnown bool
}

// placementPolicies is the batch's ordered registry of policy objects,
// seeded once from the request's catalog facts (nil = never vouched).
func (s *batchState) placementPolicies() map[string]*batchPlacementEntry {
	if s.placements == nil {
		s.placements = map[string]*batchPlacementEntry{}
	}
	return s.placements
}

// seedPlacementCatalog merges the request's proven policy catalog into the
// registry once. It only ever strengthens unknown entries — ordered facts a
// batch already derived are never overwritten by pre-batch reads.
func (s *batchState) seedPlacementCatalog() {
	if s.placementsSeeded {
		return
	}
	s.placementsSeeded = true
	if s.placementCatalog == nil {
		return
	}
	registry := s.placementPolicies()
	for name, fact := range s.placementCatalog {
		entry := registry[strings.ToLower(strings.TrimSpace(name))]
		if entry == nil {
			entry = &batchPlacementEntry{state: placementPolicyUnknown}
			registry[strings.ToLower(strings.TrimSpace(name))] = entry
		}
		if entry.state == placementPolicyUnknown {
			// Map-key presence is itself the proven-existence answer; a nil
			// fact still proves the policy exists, just with unvouched
			// attributes and an unvouched bound set.
			entry.state = placementPolicyPresent
			if fact == nil {
				continue
			}
			if fact.PrimaryRegion != "" {
				if entry.attrs == nil {
					entry.attrs = map[string]string{}
				}
				entry.attrs["primary_region"] = fact.PrimaryRegion
			}
			if fact.Regions != "" {
				if entry.attrs == nil {
					entry.attrs = map[string]string{}
				}
				entry.attrs["regions"] = fact.Regions
			}
			if fact.BoundTargets != nil {
				entry.refs = map[string]struct{}{}
				for _, ref := range fact.BoundTargets {
					entry.refs[ref] = struct{}{}
				}
				entry.refsKnown = true
			}
		}
	}
	// A proven non-nil catalog answers absence for every name it omits:
	// names the registry never saw are provably absent.
	s.placementsKnownComplete = true
}

// schemaBoundPlacementPolicy answers which policy the schema is proven
// bound to, read from the policy registry's inbound-reference sets — the
// source of truth regardless of whether the binding arrived through the
// provider catalog or a batch ALTER DATABASE. A proven bound schema
// stamps its policy onto new tables' catalog rows at CREATE time
// (probed v8.5.0); the stamp is a real bound reference that outlives a
// later schema =DEFAULT and still refuses the policy drop (8241)
// (T20-R1/#99).
func (s *batchState) schemaBoundPlacementPolicy(schema string) (string, bool) {
	ref := placementSchemaRef(schema)
	for name, entry := range s.placements {
		if entry == nil || entry.state != placementPolicyPresent {
			continue
		}
		// An individually proven schema ref answers the binding even
		// while the owning set is non-exhaustive (T20-R4).
		if _, ok := entry.refs[ref]; ok {
			return name, true
		}
	}
	return "", false
}

// placementPoliciesProjection projects the registry's ordered pre-state
// for statement metadata: a non-nil map answers proven existence per key
// (absent keys are proven-absent only while the registry is
// proven-complete — see placementsKnownComplete), a nil map means the
// catalog was never vouched. Unknown-state entries are excluded — their
// existence cannot be claimed either way (T20/#99).
func (s *batchState) placementPoliciesProjection() map[string]*spec.PlacementPolicyFact {
	s.seedPlacementCatalog()
	if !s.placementsKnownComplete {
		// A partially-known registry cannot answer absence: entries the
		// batch proved are present facts, but absent keys are unproven —
		// never proven-absent. Exposing a partial map lets consumers read
		// absent keys as false and fabricate refusals (#99 invariant:
		// unknown never becomes proven), so the catalog reads unvouched.
		return nil
	}
	out := map[string]*spec.PlacementPolicyFact{}
	for name, entry := range s.placements {
		if entry.state != placementPolicyPresent {
			continue
		}
		fact := &spec.PlacementPolicyFact{
			PrimaryRegion: entry.attrs["primary_region"],
			Regions:       entry.attrs["regions"],
		}
		if entry.refsKnown {
			fact.BoundTargets = make([]string, 0, len(entry.refs))
			for ref := range entry.refs {
				fact.BoundTargets = append(fact.BoundTargets, ref)
			}
			sort.Strings(fact.BoundTargets)
		}
		out[name] = fact
	}
	return out
}

// placementPolicyExists answers the tri-state existence question for one
// policy name: nil while the registry is unvouched, otherwise the proven
// answer.
func (s *batchState) placementPolicyExists(name string) *bool {
	if s.placements == nil {
		return nil
	}
	entry := s.placements[strings.ToLower(strings.TrimSpace(name))]
	if entry == nil {
		if !s.placementsKnownComplete {
			return nil
		}
		absent := false
		return &absent
	}
	switch entry.state {
	case placementPolicyPresent:
		present := true
		return &present
	case placementPolicyAbsent:
		absent := false
		return &absent
	default:
		return nil
	}
}

// placementBoundSet answers the proven inbound-reference list for one
// policy name, or nil while the set is unvouched. A proven-absent policy
// has a proven-empty bound set — a nonexistent object binds nothing.
func (s *batchState) placementBoundSet(name string) *[]string {
	entry := s.placementPolicies()[strings.ToLower(strings.TrimSpace(name))]
	if entry == nil {
		if exists := s.placementPolicyExists(name); exists != nil && !*exists {
			empty := []string{}
			return &empty
		}
		return nil
	}
	if !entry.refsKnown {
		if entry.state == placementPolicyAbsent {
			// A proven-absent policy binds nothing — proven-empty, not
			// unvouched (mirrors the nil-entry arm above).
			empty := []string{}
			return &empty
		}
		if len(entry.refs) > 0 {
			// The set is not exhaustive, but each retained identity is
			// individually proven — a nonempty list is still sufficient
			// reference proof for a refused DROP (8241); it is never a
			// catalog enumeration claim (T20-R4).
			refs := make([]string, 0, len(entry.refs))
			for ref := range entry.refs {
				refs = append(refs, ref)
			}
			sort.Strings(refs)
			return &refs
		}
		return nil
	}
	refs := make([]string, 0, len(entry.refs))
	for ref := range entry.refs {
		refs = append(refs, ref)
	}
	sort.Strings(refs)
	return &refs
}

func placementTableRef(schema, table string) string {
	return "table:" + strings.ToLower(schema) + "." + strings.ToLower(table)
}

func placementPartitionRef(schema, table, part string) string {
	return "partition:" + strings.ToLower(schema) + "." + strings.ToLower(table) + "." + strings.ToLower(part)
}

func placementSchemaRef(schema string) string {
	return "schema:" + strings.ToLower(schema)
}

func placementRangeRef(name string) string {
	return "range:" + strings.ToLower(name)
}

// placementAddRef records one proven inbound reference on a present policy.
func (s *batchState) placementAddRef(policy, ref string) {
	entry := s.placementPolicies()[policy]
	if entry == nil || entry.state != placementPolicyPresent {
		return
	}
	if entry.refs == nil {
		entry.refs = map[string]struct{}{}
	}
	// The reference is individually proven — record it even while the set
	// is non-exhaustive. refsKnown is never flipped by an add.
	entry.refs[ref] = struct{}{}
}

// placementRefsUnder removes every proven reference whose identity carries
// the given prefix — the drop of a bound object dissolves its references.
func (s *batchState) placementRefsUnder(prefix string) {
	for _, entry := range s.placements {
		for ref := range entry.refs {
			if strings.HasPrefix(ref, prefix) {
				delete(entry.refs, ref)
			}
		}
	}
}

// placementRemoveTargetRef deletes one exact identity from every
// registry set — proven removal dissolves the proof regardless of the
// owning set's completeness (T20-R4).
func (s *batchState) placementRemoveTargetRef(ref string) {
	for _, entry := range s.placements {
		delete(entry.refs, ref)
	}
}

// placementMayChangeTargetRef revokes one identity's proof and marks the
// candidate policies' completeness unknown — an undecidable bind/unbind
// may have moved the reference to any named policy or dropped it. The
// proof itself is always deleted; never inserted (T20-R4).
func (s *batchState) placementMayChangeTargetRef(ref string, candidates ...string) {
	named := map[string]bool{}
	for _, name := range candidates {
		name = strings.ToLower(strings.TrimSpace(name))
		if name != "" && name != "default" {
			named[name] = true
		}
	}
	for name, entry := range s.placements {
		_, had := entry.refs[ref]
		if had {
			delete(entry.refs, ref)
		}
		if had || named[name] {
			entry.refsKnown = false
		}
	}
}

// placementMayAddBindingRefs marks every named binding policy's reference
// set non-exhaustive — the undecided operation may have bound any of them.
// Known positives are never touched; an empty set under a marked flag
// authorizes nothing (T20-R4).
func (s *batchState) placementMayAddBindingRefs(facts *spec.PlacementFacts) {
	if facts == nil {
		return
	}
	for _, item := range facts.Bindings {
		if item.Target.Binding.Default {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(item.Target.Binding.Policy))
		if entry := s.placementPolicies()[name]; entry != nil {
			entry.refsKnown = false
		}
	}
}

// applyPlacementStatement dispatches the four placement lifecycle
// operations. Policy objects are schema-independent identities.
func (s *batchState) applyPlacementStatement(statement spec.Statement) error {
	ddl := statement.DDL
	s.seedPlacementCatalog()
	switch ddl.Operation {
	case spec.DDLOperationCreatePlacementPolicy:
		return s.applyCreatePlacementPolicy(statement)
	case spec.DDLOperationAlterPlacementPolicy:
		s.applyAlterPlacementPolicy(statement)
	case spec.DDLOperationDropPlacementPolicy:
		s.applyDropPlacementPolicy(statement)
	case spec.DDLOperationAlterRangePlacement:
		s.applyAlterRangePlacement(statement)
	}
	return nil
}

func (s *batchState) applyCreatePlacementPolicy(statement spec.Statement) error {
	ddl := statement.DDL
	name := strings.ToLower(strings.TrimSpace(ddl.ObjectName))
	if name == "" {
		return nil
	}
	registry := s.placementPolicies()
	exists := s.placementPolicyExists(name)
	orReplace := ddl.Options["or_replace"] == "true"
	ifNotExists := ddl.Options["if_not_exists"] == "true"
	switch spec.PlacementCreateOutcome(s.dialect, orReplace, ifNotExists, exists) {
	case spec.PlacementApplied:
		entry := registry[name]
		if entry == nil {
			entry = &batchPlacementEntry{}
			registry[name] = entry
		}
		if entry.state != placementPolicyPresent {
			// A create over a proven-absent policy starts with a
			// proven-empty bound set — nothing could bind to a policy
			// that did not exist. An OR REPLACE under an unvouched
			// catalog may have replaced a bound policy — the bound set
			// stays unvouched (OR REPLACE over a present entry keeps its
			// refs: bound objects keep referencing the name — probed).
			entry.refsKnown = exists != nil && !*exists
			if entry.refsKnown {
				entry.refs = map[string]struct{}{}
			} else {
				entry.refs = nil
			}
		}
		entry.state = placementPolicyPresent
		if orReplace {
			entry.attrs = nil
			// A replace lands with no bound references to dissolve — bound
			// objects keep referencing the name (probed: bindings persist
			// across CREATE OR REPLACE).
		}
		for _, opt := range ddl.PlacementOptions {
			if entry.attrs == nil {
				entry.attrs = map[string]string{}
			}
			if opt.StrValue != "" {
				entry.attrs[opt.Key] = opt.StrValue
			} else if opt.UintValue != nil {
				entry.attrs[opt.Key] = "*"
			}
		}
	case spec.PlacementUnproven:
		if entry := registry[name]; entry == nil {
			registry[name] = &batchPlacementEntry{state: placementPolicyUnknown}
		}
	}
	return nil
}

func (s *batchState) applyAlterPlacementPolicy(statement spec.Statement) {
	ddl := statement.DDL
	name := strings.ToLower(strings.TrimSpace(ddl.ObjectName))
	if name == "" {
		return
	}
	exists := s.placementPolicyExists(name)
	registry := s.placementPolicies()
	switch spec.PlacementAlterOutcome(s.dialect, exists) {
	case spec.PlacementApplied:
		// exists is proven true here, so a present entry always exists.
		entry := registry[name]
		// ALTER PLACEMENT POLICY replaces the definition — probed on
		// v8.5.0: SHOW CREATE after ALTER emits only the newly stated
		// members; unstated prior members do not survive (T20-R1/#99).
		// The policy identity and its bound references are untouched.
		attrs := map[string]string{}
		for _, opt := range ddl.PlacementOptions {
			if opt.StrValue != "" {
				attrs[opt.Key] = opt.StrValue
			} else if opt.UintValue != nil {
				attrs[opt.Key] = "*"
			}
		}
		entry.attrs = attrs
	case spec.PlacementUnproven:
		if entry := registry[name]; entry != nil && entry.state == placementPolicyPresent {
			// The alter may have landed: attrs are no longer provable.
			entry.attrs = nil
		}
	}
}

func (s *batchState) applyDropPlacementPolicy(statement spec.Statement) {
	ddl := statement.DDL
	name := strings.ToLower(strings.TrimSpace(ddl.ObjectName))
	if name == "" {
		return
	}
	exists := s.placementPolicyExists(name)
	bound := s.placementBoundSet(name)
	ifExists := ddl.Options["if_exists"] == "true"
	switch spec.PlacementDropOutcome(s.dialect, ifExists, exists, bound) {
	case spec.PlacementApplied:
		delete(s.placementPolicies(), name)
		entry := &batchPlacementEntry{state: placementPolicyAbsent}
		s.placementPolicies()[name] = entry
		// Bound references can only exist when the drop was proven
		// unbound — nothing to dissolve.
	case spec.PlacementUnproven:
		// The policy may or may not remain — neither existence nor the
		// bound set can be answered.
		s.placementPolicies()[name] = &batchPlacementEntry{state: placementPolicyUnknown}
	}
}

func (s *batchState) applyAlterRangePlacement(statement spec.Statement) {
	ddl := statement.DDL
	if s.dialect != spec.DialectTiDB {
		return
	}
	switch spec.PlacementRangeOutcome(s.dialect, ddl.PlacementRange) {
	case spec.PlacementApplied:
		if s.rangePlacements == nil {
			s.rangePlacements = map[string]string{}
		}
		binding := ddl.PlacementBinding
		rangeName := strings.ToLower(strings.TrimSpace(ddl.PlacementRange))
		rangeRef := placementRangeRef(rangeName)
		if binding == nil {
			s.rangePlacements[rangeName] = ""
			return
		}
		if binding.Default {
			// A proven DEFAULT dissolves the range's exact reference on
			// every policy — including a stale positive in a
			// non-exhaustive set (T20-R4).
			s.rangePlacements[rangeName] = ""
			s.placementRemoveTargetRef(rangeRef)
			return
		}
		var outcome spec.PlacementOutcome
		found := false
		if facts := s.placementFacts(statement); facts != nil {
			for _, item := range facts.Bindings {
				if item.Target.TargetKind == "range" {
					outcome = item.Outcome
					found = true
					break
				}
			}
		}
		if !found {
			exists := s.placementPolicyExists(binding.Policy)
			outcome = spec.PlacementBindOutcome(s.dialect, *binding, exists)
		}
		switch outcome {
		case spec.PlacementApplied:
			// A proven bind moves the exact ref: dissolve it on every
			// policy, then add it to the named one.
			s.placementRemoveTargetRef(rangeRef)
			s.rangePlacements[rangeName] = binding.Policy
			s.placementAddRef(binding.Policy, rangeRef)
		case spec.PlacementUnproven:
			// The bind may have landed or not — revoke the stale ref's
			// proof and mark the candidate's completeness unknown
			// without losing other proven range refs (T20-R4).
			s.placementMayChangeTargetRef(rangeRef, binding.Policy)
			delete(s.rangePlacements, rangeName)
		}
	case spec.PlacementRefused:
		// Unknown range name: native 1105 refuses, pre-state stands.
	case spec.PlacementUnproven:
		if s.rangePlacements != nil {
			delete(s.rangePlacements, strings.ToLower(strings.TrimSpace(ddl.PlacementRange)))
		}
	}
}

// applyPlacementBind applies a proven PLACEMENT POLICY assignment to one
// bound table entry — table-level (partitions empty) or member-level. The
// shape is cloned before mutation; the bound object's identity feeds the
// policy's inbound-reference set.
func (s *batchState) applyPlacementBind(alter *spec.Alter, schema string, target spec.Table) {
	if alter == nil || alter.Placement == nil {
		return
	}
	key := s.keyFor(schema, target)
	entry := s.entries[key]
	if entry == nil || entry.state != tablePresent || entry.shape == nil {
		// Unknown stays unknown; a proven-absent target keeps its refusal
		// pre-state rather than being rewritten.
		return
	}
	binding := *alter.Placement
	displaySchema := entryDisplaySchema(entry, schema, target)
	shape := cloneTableSnapshot(entry.shape)
	changed := false
	if len(alter.Partitions) == 0 {
		changed = s.applyPlacementBindToShape(shape, binding, nil, placementTableRef(displaySchema, target.Name))
	} else {
		changed = s.applyPlacementBindToShape(shape, binding, alter.Partitions, displaySchema+"."+target.Name)
	}
	if changed {
		s.entries[key] = &batchTableEntry{state: tablePresent, shape: shape, displaySchema: entry.displaySchema, displayTable: entry.displayTable}
	}
}

// applyPlacementBindToShape runs the tri-state bind transition on one
// cloned shape. ref is either a full "table:<schema>.<name>" identity
// (table-level) or a "<schema>.<name>" prefix the member loop completes.
// Reports whether the shape changed.
func (s *batchState) applyPlacementBindToShape(shape *spec.TableSnapshot, binding spec.PlacementBinding, partitions []string, ref string) bool {
	if len(partitions) == 0 {
		old := shape.PlacementPolicy
		outcome := spec.PlacementBindOutcome(s.dialect, binding, s.placementPolicyExists(binding.Policy))
		switch outcome {
		case spec.PlacementApplied:
			if binding.Default {
				shape.PlacementPolicy = ""
				shape.PlacementPolicyKnown = true
			} else {
				shape.PlacementPolicy = binding.Policy
				shape.PlacementPolicyKnown = true
			}
			// A proven bind dissolves the old target's reference on every
			// policy — identity-level removal keeps non-exhaustive sets
			// honest instead of trusting a stale recorded name (T20-R4).
			s.placementRemoveTargetRef(ref)
			if !binding.Default {
				s.placementAddRef(binding.Policy, ref)
			}
			return true
		case spec.PlacementUnproven:
			// The bind may have landed on either arm — revoke the old
			// ref's proof and mark both candidates' sets incomplete.
			shape.PlacementPolicyKnown = false
			if old != "" {
				s.placementMayChangeTargetRef(ref, old, binding.Policy)
			} else {
				s.placementMayChangeTargetRef(ref, binding.Policy)
			}
			return true
		default:
			return false
		}
	}
	changed := false
	for _, member := range partitions {
		memberName := strings.ToLower(member)
		var memberOld string
		var memberKnown bool
		if shape.PartitionPlacements != nil {
			memberOld, memberKnown = shape.PartitionPlacements[memberName]
		}
		partRef := "partition:" + ref + "." + memberName
		outcome := spec.PlacementBindOutcome(s.dialect, binding, s.placementPolicyExists(binding.Policy))
		switch outcome {
		case spec.PlacementApplied:
			if shape.PartitionPlacements == nil {
				shape.PartitionPlacements = map[string]string{}
			}
			// A proven member bind dissolves that member's exact ref on
			// every policy; DEFAULT publishes a known-empty override, not
			// an absent key (T20-R4).
			s.placementRemoveTargetRef(partRef)
			if binding.Default {
				shape.PartitionPlacements[memberName] = ""
			} else {
				shape.PartitionPlacements[memberName] = binding.Policy
				s.placementAddRef(binding.Policy, partRef)
			}
			changed = true
		case spec.PlacementUnproven:
			// The member's own override may have changed or stayed —
			// drop the map proof and revoke the ref proof, marking the
			// candidates' completeness unknown.
			if shape.PartitionPlacements != nil {
				if _, had := shape.PartitionPlacements[memberName]; had {
					delete(shape.PartitionPlacements, memberName)
					changed = true
				}
			}
			if memberKnown && memberOld != "" {
				s.placementMayChangeTargetRef(partRef, memberOld, binding.Policy)
			} else {
				s.placementMayChangeTargetRef(partRef, binding.Policy)
			}
		}
	}
	return changed
}

// placementTableDropRefs dissolves every proven reference a proven table
// drop removes — the table's own binding plus all member bindings.
func (s *batchState) placementTableDropRefs(displaySchema, name string) {
	full := strings.ToLower(displaySchema) + "." + strings.ToLower(name)
	s.placementRefsUnder("table:" + full)
	s.placementRefsUnder("partition:" + full + ".")
}

// placementTableMayDropRefs marks bound sets containing a maybe-dropped
// table's references unvouched — the ref may exist or not.
func (s *batchState) placementTableMayDropRefs(displaySchema, name string) {
	full := strings.ToLower(displaySchema) + "." + strings.ToLower(name)
	tableRef := "table:" + full
	partPrefix := "partition:" + full + "."
	for _, entry := range s.placements {
		affected := false
		for ref := range entry.refs {
			if ref == tableRef || strings.HasPrefix(ref, partPrefix) {
				delete(entry.refs, ref)
				affected = true
			}
		}
		if affected {
			entry.refsKnown = false
		}
	}
}

// placementSchemaDropRefs dissolves every proven reference a proven schema
// drop removes — the schema binding plus every object under it.
func (s *batchState) placementSchemaDropRefs(name string) {
	name = strings.ToLower(name)
	schemaRef := "schema:" + name
	tablePrefix := "table:" + name + "."
	partPrefix := "partition:" + name + "."
	for _, entry := range s.placements {
		if !entry.refsKnown {
			continue
		}
		for ref := range entry.refs {
			if ref == schemaRef || strings.HasPrefix(ref, tablePrefix) || strings.HasPrefix(ref, partPrefix) {
				delete(entry.refs, ref)
			}
		}
	}
}

// placementSchemaMayDropRefs marks bound sets unvouched when the schema's
// drop outcome is undecidable and it carried placement references.
func (s *batchState) placementSchemaMayDropRefs(name string) {
	name = strings.ToLower(name)
	schemaRef := "schema:" + name
	tablePrefix := "table:" + name + "."
	partPrefix := "partition:" + name + "."
	for _, entry := range s.placements {
		affected := false
		for ref := range entry.refs {
			if ref == schemaRef || strings.HasPrefix(ref, tablePrefix) || strings.HasPrefix(ref, partPrefix) {
				delete(entry.refs, ref)
				affected = true
			}
		}
		if affected {
			entry.refsKnown = false
		}
	}
}

// placementRekeyTableRef rewrites a proven renamed table's references from
// the old identity to the new — the binding follows the object.
func (s *batchState) placementRekeyTableRef(oldFull, newFull string) {
	oldFull = strings.ToLower(oldFull)
	newFull = strings.ToLower(newFull)
	oldTable := "table:" + oldFull
	oldPart := "partition:" + oldFull + "."
	for _, entry := range s.placements {
		for ref := range entry.refs {
			switch {
			case ref == oldTable:
				delete(entry.refs, ref)
				entry.refs["table:"+newFull] = struct{}{}
			case strings.HasPrefix(ref, oldPart):
				delete(entry.refs, ref)
				entry.refs["partition:"+newFull+"."+strings.TrimPrefix(ref, oldPart)] = struct{}{}
			}
		}
	}
}

// declaredPlacementPolicyExists answers the tri-state existence question
// for the policy a statement's PlacementBinding names; nil binding → nil.
func (s *batchState) declaredPlacementPolicyExists(ddl *spec.DDL) *bool {
	if ddl == nil || ddl.PlacementBinding == nil || ddl.PlacementBinding.Default {
		return nil
	}
	return s.placementPolicyExists(ddl.PlacementBinding.Policy)
}

// placementSchemaRefSync maintains the inbound-reference sets across a
// proven schema bind transition: the statement's landed post-state is
// "this schema binds exactly newPolicy", so every other policy's
// proven set loses the reference (a rebind or =DEFAULT dissolves it
// even when the schema's own attr projection never vouched the old
// name — the registry-side ref is the authoritative record, T20-R1/#99).
func (s *batchState) placementSchemaRefSync(schema, newPolicy string) {
	ref := placementSchemaRef(schema)
	for name, entry := range s.placements {
		if name == newPolicy {
			continue
		}
		delete(entry.refs, ref)
	}
	if newPolicy != "" {
		s.placementAddRef(newPolicy, ref)
	}
}

// placementSchemaRefUnvouch marks every proven set that carries the
// schema's reference unvouched — an undecidable bind/unbind may have
// dissolved it — plus both candidate policies the statement named
// (either arm may have bound it). The registry refs are authoritative:
// a policy can carry the reference even when the schema's own attr
// projection never vouched the name (T20-R1/#99).
func (s *batchState) placementSchemaRefUnvouch(schema, oldPolicy, declaredPolicy string) {
	// The schema's reference may have moved to either candidate policy or
	// vanished — revoke its proof and mark each candidate's completeness
	// unknown. An undecidable arm never inserts an unproven positive ref
	// (T20-R4).
	s.placementMayChangeTargetRef(placementSchemaRef(schema), oldPolicy, declaredPolicy)
}

// entryDisplaySchema resolves the schema a table entry binds under for
// reference identity — the entry's proven display schema wins, then the
// authored target schema, then the request schema.
func entryDisplaySchema(entry *batchTableEntry, fallbackSchema string, target spec.Table) string {
	if entry != nil && entry.displaySchema != "" {
		return entry.displaySchema
	}
	if s := strings.TrimSpace(target.Schema); s != "" {
		return s
	}
	return fallbackSchema
}

// alterStatementHasPlacement reports whether any spec is a placement bind.
func alterStatementHasPlacement(ddl *spec.DDL) bool {
	for _, alter := range ddl.Alter {
		if alter.Action == "placement_policy" {
			return true
		}
	}
	return false
}

// alterPlacementMultiChange reports whether placement shares the ALTER
// with a second schema-changing spec — probed 8200 on v8.5.0
// ("Unsupported multi schema change for alter table placement"): a
// placement spec may only accompany ALGORITHM/LOCK modifiers; a second
// placement spec or any structural/option sibling refuses the whole
// statement (T20-R1/#99).
func alterPlacementMultiChange(ddl *spec.DDL) bool {
	schemaChanging := 0
	for _, alter := range ddl.Alter {
		switch alter.Action {
		case "algorithm", "lock":
			continue
		}
		schemaChanging++
	}
	return schemaChanging > 1
}

// placementPremiseOutcome evaluates the policy-existence premise of every
// placement spec in one ALTER. refused wins, then unproven.
func (s *batchState) placementPremiseOutcome(ddl *spec.DDL) spec.PlacementOutcome {
	s.seedPlacementCatalog()
	facts := spec.AssessPlacementBindings(s.dialect, spec.CollectPlacementBindings(ddl), s.placementPolicyExists)
	return facts.Outcome
}

// isPlacementPolicyLifecycle reports the three policy-object lifecycle
// operations whose target premise is carried per-statement.
func isPlacementPolicyLifecycle(ddl *spec.DDL) bool {
	if ddl == nil {
		return false
	}
	switch ddl.Operation {
	case spec.DDLOperationCreatePlacementPolicy, spec.DDLOperationAlterPlacementPolicy,
		spec.DDLOperationDropPlacementPolicy:
		return true
	}
	return false
}

// copyBoolPtr deep-copies a tri-state answer so metadata never aliases the
// live registry pointer.
func copyBoolPtr(in *bool) *bool {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

// setPlacementMutationOutcome completes the enclosing atomic mutation's
// static outcome on a statement's owned placement facts. facts lists the
// bounded evidence the undecided case owes; it is nil-safe — statements
// without attached placement facts simply carry no mutation annotation.
func setPlacementMutationOutcome(statement spec.Statement, outcome spec.PlacementOutcome, facts ...string) {
	if statement.Metadata == nil || statement.Metadata.Placement == nil {
		return
	}
	copyOutcome := outcome
	statement.Metadata.Placement.MutationOutcome = &copyOutcome
	statement.Metadata.Placement.RequiredFacts = append([]string(nil), facts...)
}

// placementFacts returns the statement's adjudicated placement facts —
// the enrichment-attached owned projection when present, or a fresh
// assessment through the same collector + registry lookup for direct
// state tests that bypass enrichment. The fallback attaches into
// statement metadata when it exists so the mutation outcome lands on the
// same object the rules read (T20-R4).
func (s *batchState) placementFacts(statement spec.Statement) *spec.PlacementFacts {
	if statement.DDL == nil {
		return nil
	}
	if statement.Metadata != nil && statement.Metadata.Placement != nil {
		return statement.Metadata.Placement
	}
	s.seedPlacementCatalog()
	facts := spec.AssessPlacementBindings(s.dialect, spec.CollectPlacementBindings(statement.DDL), s.placementPolicyExists)
	fillPlacementTargetSchema(facts, s.schema)
	if statement.Metadata != nil {
		statement.Metadata.Placement = facts
	}
	return facts
}

// fillPlacementTargetSchema stamps the resolved effective schema onto
// every collected target that left the qualifier implicit, so member
// finding identities carry the same schema the state adjudicated under.
// Explicitly qualified targets are never overwritten; schema-scoped
// targets adopt the resolved name when the operation named none (T20-R4).
func fillPlacementTargetSchema(facts *spec.PlacementFacts, effectiveSchema string) {
	if facts == nil {
		return
	}
	for i := range facts.Bindings {
		target := &facts.Bindings[i].Target
		// RANGE targets are global/meta, never schema-owned.
		if target.Schema == "" && target.TargetKind != "range" {
			target.Schema = effectiveSchema
		}
		if target.TargetKind == "schema" && target.Target == "" {
			target.Target = target.Schema
		}
	}
}

// isTiDB85 gates the execution normalization that models TiDB 8.5's
// probed discard of the subpartitioning axis. Other versions withhold —
// an unprobed ignored behavior is never inferred.
func isTiDB85(version *spec.VersionIdentity) bool {
	return version != nil && version.Product == spec.VersionProductTiDB &&
		version.Major == 8 && version.Minor == 5 && version.Patch == 0
}

// normalizeIgnoredSubpartition returns an owned statement copy whose DDL
// has every subpartition marker cleared — modeling the axis TiDB 8.5.0
// provably discards — and whose primary member names are restored from
// the capability projection the extractor derived. Only the copy is
// normalized: the original statement, markers, and concealed-name
// projection stay immutable, and no child names or refs are fabricated
// (T20-R4).
func normalizeIgnoredSubpartition(statement spec.Statement) spec.Statement {
	normalized := statement
	ddl := *statement.DDL
	normalized.DDL = &ddl
	ddl.Subpartitioned = false
	if ddl.PartitionPrimaryNames != nil {
		ddl.PartitionNames = append([]string(nil), ddl.PartitionPrimaryNames...)
	}
	ddl.PartitionDefs = spec.ClonePartitionDefs(statement.DDL.PartitionDefs)
	for i := range ddl.PartitionDefs {
		ddl.PartitionDefs[i].Subpartitioned = false
	}
	if statement.DDL.Alter != nil {
		ddl.Alter = make([]spec.Alter, len(statement.DDL.Alter))
		for i, alter := range statement.DDL.Alter {
			ddl.Alter[i] = alter
			if alter.PartitionDefs != nil {
				ddl.Alter[i].PartitionDefs = spec.ClonePartitionDefs(alter.PartitionDefs)
				for j := range ddl.Alter[i].PartitionDefs {
					ddl.Alter[i].PartitionDefs[j].Subpartitioned = false
				}
			}
			if alter.PartitionScheme != nil {
				ddl.Alter[i].PartitionScheme = spec.ClonePartitionScheme(alter.PartitionScheme)
				ddl.Alter[i].PartitionScheme.Subpartitioned = false
				ddl.Alter[i].PartitionScheme.SubType = ""
				for j := range ddl.Alter[i].PartitionScheme.Defs {
					ddl.Alter[i].PartitionScheme.Defs[j].Subpartitioned = false
				}
			}
		}
	}
	return normalized
}
