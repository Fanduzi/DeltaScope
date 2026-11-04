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
		if ddl.PlacementBinding != nil {
			return true
		}
		if _, ok := ddl.Options["placement_policy"]; ok {
			return true
		}
		for i := range ddl.Alter {
			if ddl.Alter[i].Action == "placement_policy" || ddl.Alter[i].Placement != nil {
				return true
			}
		}
		for i := range ddl.PartitionDefs {
			if ddl.PartitionDefs[i].Placement != nil {
				return true
			}
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
	// refs is the proven inbound-reference identity set when refsKnown;
	// identities are "table:<schema>.<name>", "partition:<schema>.<name>.<part>",
	// "schema:<name>", "range:<name>". refsKnown=false keeps the set
	// unvouched — never read nil refs as proven-empty.
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
		if entry == nil || entry.state != placementPolicyPresent || !entry.refsKnown {
			continue
		}
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
		return nil
	}
	refs := make([]string, 0, len(entry.refs))
	for ref := range entry.refs {
		refs = append(refs, ref)
	}
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
	if entry == nil || !entry.refsKnown {
		return
	}
	entry.refs[ref] = struct{}{}
}

// placementRemoveRef drops one proven inbound reference. Dropping a
// bound object also drops its reference — the set stays proven when the
// drop is proven.
func (s *batchState) placementRemoveRef(policy, ref string) {
	entry := s.placementPolicies()[policy]
	if entry == nil || !entry.refsKnown {
		return
	}
	delete(entry.refs, ref)
}

// placementRefsUnder removes every proven reference whose identity carries
// the given prefix — the drop of a bound object dissolves its references.
func (s *batchState) placementRefsUnder(prefix string) {
	for _, entry := range s.placements {
		if !entry.refsKnown {
			continue
		}
		for ref := range entry.refs {
			if strings.HasPrefix(ref, prefix) {
				delete(entry.refs, ref)
			}
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
		entry := registry[name]
		if entry == nil {
			entry = &batchPlacementEntry{state: placementPolicyPresent, refs: map[string]struct{}{}, refsKnown: true}
			registry[name] = entry
		}
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
		if binding == nil {
			s.rangePlacements[rangeName] = ""
			return
		}
		if binding.Default {
			old := s.rangePlacements[rangeName]
			s.rangePlacements[rangeName] = ""
			if old != "" {
				s.placementRemoveRef(old, placementRangeRef(rangeName))
			}
			return
		}
		exists := s.placementPolicyExists(binding.Policy)
		switch spec.PlacementBindOutcome(s.dialect, *binding, exists) {
		case spec.PlacementApplied:
			old := s.rangePlacements[rangeName]
			s.rangePlacements[rangeName] = binding.Policy
			if old != "" && old != binding.Policy {
				s.placementRemoveRef(old, placementRangeRef(rangeName))
			}
			s.placementAddRef(binding.Policy, placementRangeRef(rangeName))
		case spec.PlacementUnproven:
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
		old, known := shape.PlacementPolicy, shape.PlacementPolicyKnown
		outcome := spec.PlacementBindOutcome(s.dialect, binding, s.placementPolicyExists(binding.Policy))
		switch outcome {
		case spec.PlacementApplied:
			if binding.Default {
				shape.PlacementPolicy = ""
				shape.PlacementPolicyKnown = true
				if known && old != "" {
					s.placementRemoveRef(old, ref)
				}
			} else {
				shape.PlacementPolicy = binding.Policy
				shape.PlacementPolicyKnown = true
				if known && old != "" && old != binding.Policy {
					s.placementRemoveRef(old, ref)
				}
				s.placementAddRef(binding.Policy, ref)
			}
			return true
		case spec.PlacementUnproven:
			shape.PlacementPolicyKnown = false
			return true
		default:
			return false
		}
	}
	changed := false
	for _, member := range partitions {
		memberName := strings.ToLower(member)
		memberOld, memberKnown := "", false
		if shape.PartitionPlacements != nil {
			memberOld, memberKnown = shape.PartitionPlacements[memberName], true
		}
		partRef := "partition:" + ref + "." + memberName
		outcome := spec.PlacementBindOutcome(s.dialect, binding, s.placementPolicyExists(binding.Policy))
		switch outcome {
		case spec.PlacementApplied:
			if shape.PartitionPlacements == nil {
				shape.PartitionPlacements = map[string]string{}
			}
			if binding.Default {
				delete(shape.PartitionPlacements, memberName)
				if memberKnown && memberOld != "" {
					s.placementRemoveRef(memberOld, partRef)
				}
			} else {
				shape.PartitionPlacements[memberName] = binding.Policy
				if memberKnown && memberOld != "" && memberOld != binding.Policy {
					s.placementRemoveRef(memberOld, partRef)
				}
				s.placementAddRef(binding.Policy, partRef)
			}
			changed = true
		case spec.PlacementUnproven:
			if shape.PartitionPlacements != nil {
				delete(shape.PartitionPlacements, memberName)
				changed = true
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
		if !entry.refsKnown {
			continue
		}
		for ref := range entry.refs {
			if ref == tableRef || strings.HasPrefix(ref, partPrefix) {
				entry.refsKnown = false
				break
			}
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
		if !entry.refsKnown {
			continue
		}
		for ref := range entry.refs {
			if ref == schemaRef || strings.HasPrefix(ref, tablePrefix) || strings.HasPrefix(ref, partPrefix) {
				entry.refsKnown = false
				break
			}
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
		if !entry.refsKnown {
			continue
		}
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
		if name == newPolicy || !entry.refsKnown {
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
	ref := placementSchemaRef(schema)
	oldPolicy = strings.ToLower(strings.TrimSpace(oldPolicy))
	declaredPolicy = strings.ToLower(strings.TrimSpace(declaredPolicy))
	for name, entry := range s.placements {
		if entry == nil || !entry.refsKnown {
			continue
		}
		if _, ok := entry.refs[ref]; ok {
			delete(entry.refs, ref)
			entry.refsKnown = false
			continue
		}
		if name == oldPolicy || (declaredPolicy != "" && declaredPolicy != "default" && name == declaredPolicy) {
			entry.refs[ref] = struct{}{}
			entry.refsKnown = false
		}
	}
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
	unproven := false
	for _, alter := range ddl.Alter {
		if alter.Action != "placement_policy" || alter.Placement == nil {
			continue
		}
		binding := *alter.Placement
		exists := s.placementPolicyExists(binding.Policy)
		switch spec.PlacementBindOutcome(s.dialect, binding, exists) {
		case spec.PlacementRefused:
			return spec.PlacementRefused
		case spec.PlacementUnproven:
			unproven = true
		}
	}
	if unproven {
		return spec.PlacementUnproven
	}
	return spec.PlacementApplied
}
