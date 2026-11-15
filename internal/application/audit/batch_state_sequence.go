package audit

import (
	"context"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// batch_sequence.go holds the T21/#100 ordered sequence registry: bounded
// identity → definition state for CREATE/ALTER/DROP SEQUENCE. It mirrors the
// table/schema contracts — once-per-key provider reads, schema-scope gates,
// refusal-preserving transitions, owned projections — while deliberately
// carrying no runtime facts: issuance progress, cache consumption, and
// persisted position are A2 scope and never appear here.

type sequenceKey struct {
	schema string
	name   string
}

type sequenceEntryState int

const (
	sequenceUnknown sequenceEntryState = iota
	sequencePresent
	sequenceAbsent
)

// batchSequenceEntry is the registry's owned fact for one sequence identity.
// state unknown means unvouched — never a silent absent. definition is the
// proven (or batch-derived) catalog parameter set, nil when unvouched.
// providerRefs is the catalog-vouched inbound edge set (nil = never read);
// refsComplete marks the provider's completeness vouch. refsDiverged marks
// batch-authored uncertainty — an owner tombstone or an unproven authored
// reference made the set unprovable even if the catalog read was complete.
// resolved distinguishes "answered once" from a lazily created entry so a
// poison marker never suppresses the one real provider read.
type batchSequenceEntry struct {
	state         sequenceEntryState
	displaySchema string
	displayName   string
	definition    *spec.SequenceDefinition
	providerRefs  []spec.SequenceRefEdge
	refsComplete  bool
	refsDiverged  bool
	resolved      bool
	conflictKind  string
}

// sequenceKeyFor resolves a target ref to its registry key under the
// effective schema — authored qualifier wins, request schema falls back,
// mirroring keyFor's contract for tables.
func (s *batchState) sequenceKeyFor(ref spec.SequenceRef) sequenceKey {
	schema := strings.TrimSpace(ref.Schema)
	if schema == "" {
		schema = s.schema
	}
	return sequenceKey{schema: strings.ToLower(schema), name: strings.ToLower(ref.Name)}
}

// effectiveSequenceSchema reports the schema identity a lookup must use —
// the authored qualifier or the request schema.
func (s *batchState) effectiveSequenceSchema(ref spec.SequenceRef) string {
	if schema := strings.TrimSpace(ref.Schema); schema != "" {
		return schema
	}
	return s.schema
}

// resolveSequence returns the owned entry for a key, consulting the
// once-per-request object resolver at most once. The schema namespace gates
// every identity exactly like tables: a proven-absent schema proves its
// sequences absent, a batch-created schema proves untouched members absent,
// and an unvouched or batch-invalidated scope tombstones every identity
// under it. Resolver errors propagate — never cached, never degraded.
func (s *batchState) resolveSequence(ctx context.Context, key sequenceKey, ref spec.SequenceRef) (*batchSequenceEntry, error) {
	if entry, ok := s.sequenceEntries[key]; ok && entry.resolved {
		return entry, nil
	}
	entry := s.sequenceEntries[key]
	if entry == nil {
		entry = &batchSequenceEntry{state: sequenceUnknown}
		s.sequenceEntries[key] = entry
	}
	switch schemaEntry, bound := s.schemaEntries[key.schema]; {
	case bound && schemaEntry.state == schemaAbsent:
		entry.state = sequenceAbsent
		entry.resolved = true
		return entry, nil
	case bound && schemaEntry.state == schemaUnknown:
		entry.resolved = true
		return entry, nil
	case bound && schemaEntry.state == schemaPresent && schemaEntry.batchCreated:
		entry.state = sequenceAbsent
		entry.resolved = true
		return entry, nil
	}
	if _, dropped := s.invalidatedSchemas[key.schema]; s.contaminated || s.provider == nil || dropped {
		entry.resolved = true
		return entry, nil
	}
	// A proven non-sequence object at the same namespace identity is a
	// shared-namespace conflict: TiDB sequences occupy the table namespace.
	if tableEntry, ok := s.entries[batchTableKey{dialect: s.dialect, schema: key.schema, table: key.name}]; ok && tableEntry.state == tablePresent {
		entry.conflictKind = "table"
	}
	var snapshot *spec.ObjectSnapshot
	var err error
	lookup := spec.ObjectLookupRequest{
		Schema: s.effectiveSequenceSchema(ref),
		Type:   "sequence",
		Name:   strings.TrimSpace(ref.Name),
	}
	if s.sequenceResolver != nil {
		snapshot, err = s.sequenceResolver(ctx, lookup)
	} else {
		resolver, ok := s.provider.(ObjectResolver)
		if !ok {
			entry.resolved = true
			return entry, nil
		}
		snapshot, err = resolver.ResolveObject(ctx, s.dialect, lookup)
	}
	if err != nil {
		return nil, err
	}
	entry.resolved = true
	if snapshot == nil {
		return entry, nil
	}
	entry.displaySchema = snapshot.Schema
	entry.displayName = snapshot.Name
	switch {
	case snapshot.IsConfirmed() && snapshot.Exists:
		entry.state = sequencePresent
		entry.definition = snapshot.Sequence.Clone()
		if snapshot.SequenceReferences != nil && !snapshot.SequenceReferencesUnknown {
			entry.providerRefs = spec.CloneSequenceRefEdges(snapshot.SequenceReferences)
			entry.refsComplete = true
		}
		if snapshot.Type != "" && snapshot.Type != "sequence" {
			entry.conflictKind = snapshot.Type
		}
	case snapshot.IsNotFound():
		entry.state = sequenceAbsent
		entry.providerRefs = []spec.SequenceRefEdge{}
		entry.refsComplete = true
	default:
		entry.state = sequenceUnknown
	}
	return entry, nil
}

// projectSequenceEntry renders the statement-visible projection of one
// entry — proven states carry the owned definition and the deduplicated
// inbound edge set, unproven states carry only the unvouched identity.
// Edges come from two sources joined under one completeness flag: the
// provider-vouched set (filtered by the owner's live batch state) and
// edges derived from currently-live table shapes — so a proven owner
// drop/rename retires or rekeys edges automatically, and a tombstoned
// owner degrades completeness instead of keeping stale positives.
// Projections never alias the registry.
func (s *batchState) projectSequenceEntry(key sequenceKey, entry *batchSequenceEntry) *spec.SequenceState {
	displaySchema, displayName := entry.displaySchema, entry.displayName
	if displayName == "" {
		displaySchema, displayName = key.schema, key.name
	}
	state := &spec.SequenceState{
		Schema:       displaySchema,
		Name:         displayName,
		ConflictKind: entry.conflictKind,
	}
	switch entry.state {
	case sequencePresent:
		state.Status = spec.MetadataStatusConfirmed
		state.Exists = true
		state.Definition = entry.definition.Clone()
		edges, unknown := s.sequenceInboundEdges(key, entry)
		state.References = edges
		state.ReferencesUnknown = unknown
	case sequenceAbsent:
		state.Status = spec.MetadataStatusNotFound
		state.Exists = false
		state.References = []spec.SequenceRefEdge{}
	default:
		state.Status = spec.MetadataStatusUnavailable
	}
	return state
}

// sequenceInboundEdges computes the live inbound edge set for one present
// entry: provider-vouched edges whose owner is still admissible (a proven
// owner absence retires the edge, an undecidable owner downgrades the set's
// completeness) plus edges derived from live table shapes whose column
// defaults name this target. The unknown flag is true whenever the edge set
// cannot be proven complete.
func (s *batchState) sequenceInboundEdges(key sequenceKey, entry *batchSequenceEntry) ([]spec.SequenceRefEdge, bool) {
	edges := make([]spec.SequenceRefEdge, 0, len(entry.providerRefs)+4)
	unknown := !entry.refsComplete || entry.refsDiverged
	for _, edge := range entry.providerRefs {
		ownerKey := batchTableKey{dialect: s.dialect, schema: strings.ToLower(edge.OwnerSchema), table: strings.ToLower(edge.OwnerName)}
		owner, tracked := s.entries[ownerKey]
		switch {
		case tracked && owner.state == tableAbsent:
			// The owner is proven gone — the edge provably died with it.
			continue
		case tracked && owner.state != tablePresent:
			// The owner's fate is undecidable: the edge may persist or
			// may not, so it leaves the proven set and unvouches
			// completeness.
			unknown = true
			continue
		default:
			edges = append(edges, edge)
		}
	}
	edges = append(edges, s.derivedSequenceEdges(key)...)
	return spec.DedupeSequenceRefEdges(edges), unknown
}

// derivedSequenceEdges collects inbound edges from live table shapes:
// every present entry's column default that names this sequence identity
// contributes one owner-slot/target edge. The shapes are already the
// ordered post-state — a proven create adds its edges, a proven drop or
// slot rewrite removes them, and a tombstoned owner contributes nothing.
func (s *batchState) derivedSequenceEdges(key sequenceKey) []spec.SequenceRefEdge {
	var edges []spec.SequenceRefEdge
	for _, entries := range []map[batchTableKey]*batchTableEntry{s.entries, s.tempEntries} {
		for ownerKey, owner := range entries {
			if owner.state != tablePresent || owner.shape == nil {
				continue
			}
			displaySchema, displayTable := owner.displaySchema, owner.displayTable
			if displaySchema == "" {
				displaySchema = ownerKey.schema
			}
			if displayTable == "" {
				displayTable = ownerKey.table
			}
			for _, column := range owner.shape.Columns {
				if column.DefaultExpression == nil {
					continue
				}
				for _, ref := range column.DefaultExpression.SequenceRefs {
					if s.sequenceKeyFor(ref) != key {
						continue
					}
					edges = append(edges, spec.SequenceRefEdge{
						OwnerKind:    "table",
						OwnerSchema:  displaySchema,
						OwnerName:    displayTable,
						Slot:         "default:" + column.Name,
						TargetSchema: key.schema,
						TargetName:   key.name,
					})
				}
			}
		}
	}
	return edges
}

// bindSequences resolves every sequence identity the statement touches and
// projects the ordered pre-state into metadata.Sequences keyed `schema.name`.
// Reads go through the same once-per-request cache Metadata.Objects uses.
func (s *batchState) bindSequences(ctx context.Context, statement spec.Statement) (map[string]*spec.SequenceState, error) {
	var refs []spec.SequenceRef
	if declaration := statement.DDL.Sequence; declaration != nil {
		refs = declaration.TargetRefs()
	} else if statement.Dialect == spec.DialectTiDB && statement.DDL.Operation == spec.DDLOperationRenameTable {
		// RENAME TABLE pair endpoints share the sequence namespace — resolve
		// both sides so proven sequence identities bind like sequence targets.
		// Sequences are TiDB-only; MySQL pairs keep their table verdict.
		for _, target := range statement.DDL.Targets {
			refs = append(refs, spec.SequenceRef{Schema: target.Schema, Name: target.Name})
		}
	}
	if len(refs) == 0 {
		return nil, nil
	}
	projection := make(map[string]*spec.SequenceState, len(refs))
	for _, ref := range refs {
		key := s.sequenceKeyFor(ref)
		entry, err := s.resolveSequence(ctx, key, ref)
		if err != nil {
			return nil, err
		}
		name := key.schema + "." + key.name
		projection[name] = s.projectSequenceEntry(key, entry)
	}
	return projection, nil
}

// authoredSequenceDefinition derives the batch-proven definition a CREATE
// publishes — authored options only, with every unwritten member left nil
// (unproven, never defaulted).
func authoredSequenceDefinition(declaration *spec.SequenceDeclaration) *spec.SequenceDefinition {
	definition := &spec.SequenceDefinition{CommentDeclared: declaration.CommentDeclared}
	for _, option := range declaration.Options {
		switch option.Kind {
		case spec.SequenceOptionStartWith:
			definition.Start = option.Value
		case spec.SequenceOptionIncrementBy:
			definition.Increment = option.Value
		case spec.SequenceOptionMinValue:
			definition.MinValue = option.Value
		case spec.SequenceOptionNoMinValue:
			definition.MinValue = nil
		case spec.SequenceOptionMaxValue:
			definition.MaxValue = option.Value
		case spec.SequenceOptionNoMaxValue:
			definition.MaxValue = nil
		case spec.SequenceOptionCache:
			definition.CacheValue = option.Value
			v := true
			definition.Cache = &v
		case spec.SequenceOptionNoCache:
			definition.CacheValue = nil
			v := false
			definition.Cache = &v
		case spec.SequenceOptionCycle:
			v := true
			definition.Cycle = &v
		case spec.SequenceOptionNoCycle:
			v := false
			definition.Cycle = &v
		}
	}
	return definition
}

// mergeSequenceAlter applies authored ALTER members onto a proven definition
// clone — RESTART members affect issuance, never the catalog definition, so
// they are excluded here.
func mergeSequenceAlter(base *spec.SequenceDefinition, declaration *spec.SequenceDeclaration) *spec.SequenceDefinition {
	out := base.Clone()
	if out == nil {
		out = &spec.SequenceDefinition{}
	}
	for _, option := range declaration.Options {
		switch option.Kind {
		case spec.SequenceOptionStartWith:
			out.Start = option.Value
		case spec.SequenceOptionIncrementBy:
			out.Increment = option.Value
		case spec.SequenceOptionMinValue:
			out.MinValue = option.Value
		case spec.SequenceOptionNoMinValue:
			out.MinValue = nil
		case spec.SequenceOptionMaxValue:
			out.MaxValue = option.Value
		case spec.SequenceOptionNoMaxValue:
			out.MaxValue = nil
		case spec.SequenceOptionCache:
			out.CacheValue = option.Value
			v := true
			out.Cache = &v
		case spec.SequenceOptionNoCache:
			out.CacheValue = nil
			v := false
			out.Cache = &v
		case spec.SequenceOptionCycle:
			v := true
			out.Cycle = &v
		case spec.SequenceOptionNoCycle:
			v := false
			out.Cycle = &v
		}
	}
	return out
}

// applySequenceStatement records the conditional post-state of one sequence
// lifecycle statement. Refusals preserve the pre-state — a proven refusal
// publishes nothing. Per-target adjudication means a multi-target DROP can
// apply partial effects; an unbounded aspect tombstones the touched
// identities rather than fabricating a success shape.
func (s *batchState) applySequenceStatement(ctx context.Context, statement spec.Statement) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	declaration := statement.DDL.Sequence
	if declaration == nil {
		return nil
	}
	refs := declaration.TargetRefs()
	if !fullyAuditedStatement(s.dialect, statement) {
		for _, ref := range refs {
			key := s.sequenceKeyFor(ref)
			// An unbounded statement tombstones its touched identities —
			// resolved keeps a later provider read from reviving pre-batch
			// facts over an undecidable mutation.
			s.sequenceEntries[key] = &batchSequenceEntry{state: sequenceUnknown, resolved: true}
		}
		return nil
	}
	for _, ref := range refs {
		key := s.sequenceKeyFor(ref)
		entry, err := s.resolveSequence(ctx, key, ref)
		if err != nil {
			return err
		}
		switch statement.DDL.Operation {
		case spec.DDLOperationCreateSequence:
			s.applyCreateSequence(entry, key, declaration)
		case spec.DDLOperationAlterSequence:
			s.applyAlterSequence(entry, key, declaration)
		case spec.DDLOperationDropSequence:
			s.applyDropSequence(entry, key, declaration)
		}
	}
	return nil
}

func (s *batchState) applyCreateSequence(entry *batchSequenceEntry, key sequenceKey, declaration *spec.SequenceDeclaration) {
	if entry.conflictKind != "" {
		// A proven non-sequence object at the identity refuses the create —
		// the statement never executes, so the sequence state stays absent.
		return
	}
	switch entry.state {
	case sequencePresent:
		// Proven duplicate — refused without IF NOT EXISTS, a no-op with it.
		// Either way the pre-state definition stands.
		return
	case sequenceAbsent:
		entry.state = sequencePresent
		entry.definition = authoredSequenceDefinition(declaration)
		entry.displaySchema, entry.displayName = key.schema, key.name
		// A batch-created sequence carries no catalog-visible inbound
		// references — anything later derives through live table shapes.
		entry.providerRefs = []spec.SequenceRefEdge{}
		entry.refsComplete = true
		entry.refsDiverged = false
	default:
		entry.state = sequenceUnknown
	}
}

func (s *batchState) applyAlterSequence(entry *batchSequenceEntry, key sequenceKey, declaration *spec.SequenceDeclaration) {
	if entry.conflictKind != "" {
		return
	}
	switch entry.state {
	case sequencePresent:
		// Proven target — definition members merge; RESTART changes
		// issuance, never the definition. An empty option list is a no-op
		// and keeps the proven shape.
		entry.definition = mergeSequenceAlter(entry.definition, declaration)
		entry.displaySchema, entry.displayName = key.schema, key.name
	case sequenceAbsent:
		// Proven refusal (or proven IF EXISTS no-op): the absent answer
		// stands — nothing new was created either way.
		return
	default:
		entry.state = sequenceUnknown
	}
}

func (s *batchState) applyDropSequence(entry *batchSequenceEntry, key sequenceKey, _ *spec.SequenceDeclaration) {
	if entry.conflictKind != "" {
		// The identity binds to a proven non-sequence object — a sequence
		// drop cannot remove it and the sequence answer stays absent.
		return
	}
	switch entry.state {
	case sequencePresent:
		entry.state = sequenceAbsent
		entry.definition = nil
		entry.providerRefs = []spec.SequenceRefEdge{}
		entry.refsComplete = true
		entry.refsDiverged = false
		entry.displaySchema, entry.displayName = key.schema, key.name
	case sequenceAbsent:
		// Absent target: refused without IF EXISTS, no-op with it — the
		// absent answer stands either way.
		return
	default:
		entry.state = sequenceUnknown
	}
}

// renameSequenceEntry transfers a sequence entry when a RENAME TABLE pair
// moves a proven sequence identity — TiDB renames sequences through RENAME
// TABLE because they share the table namespace. The old identity's facts are
// revoked (a same-named recreate must not inherit stale state) and the
// definition follows the object to its new name.
func (s *batchState) renameSequenceEntry(source, destination sequenceKey) {
	entry, ok := s.sequenceEntries[source]
	if !ok {
		return
	}
	delete(s.sequenceEntries, source)
	if entry.state == sequencePresent {
		entry.displaySchema, entry.displayName = destination.schema, destination.name
		s.sequenceEntries[destination] = entry
		return
	}
	// Non-present entries do not transfer: a renamed-away identity leaves no
	// proven fact at the destination beyond what its own binding answers.
}

// renameSequencePairs moves proven sequence identities alongside a RENAME
// TABLE pair list — sources and destinations pair-wise in DDL.Targets order.
// Only entries the registry already resolved transfer; unproven identities
// leave the destination to bind on its own first touch.
func (s *batchState) renameSequencePairs(statement spec.Statement) {
	if statement.DDL == nil || len(statement.DDL.Targets) < 2 {
		return
	}
	targets := statement.DDL.Targets
	for i := 0; i+1 < len(targets); i += 2 {
		source := s.sequenceKeyFor(spec.SequenceRef{Schema: targets[i].Schema, Name: targets[i].Name})
		destination := s.sequenceKeyFor(spec.SequenceRef{Schema: targets[i+1].Schema, Name: targets[i+1].Name})
		s.renameSequenceEntry(source, destination)
	}
}

// sequenceTargetsForStatement returns the ordered-state binding for a
// statement's sequence targets, or nil when the statement has none.
func (s *batchState) sequenceTargetsForStatement(ctx context.Context, statement spec.Statement) (map[string]*spec.SequenceState, error) {
	return s.bindSequences(ctx, statement)
}

// sequenceRefUse is one authored sequence reference inside a table
// statement — the owner key it lands under plus the authored target ref.
type sequenceRefUse struct {
	owner  batchTableKey
	target spec.SequenceRef
}

// authoredSequenceRefUses collects the bounded column-default sequence
// references a statement authors: CREATE TABLE columns and ALTER column
// definitions carrying DefaultExpression.SequenceRefs. Only structured
// ast.TableNameExpr identities qualify — never guessed names.
func (s *batchState) authoredSequenceRefUses(statement spec.Statement) []sequenceRefUse {
	if statement.DDL == nil || len(statement.DDL.TableTargets()) == 0 {
		return nil
	}
	var uses []sequenceRefUse
	owner := s.keyFor(s.schema, statement.DDL.TableTargets()[0])
	collect := func(columns []spec.Column) {
		for _, column := range columns {
			if column.DefaultExpression == nil {
				continue
			}
			for _, ref := range column.DefaultExpression.SequenceRefs {
				uses = append(uses, sequenceRefUse{owner: owner, target: ref})
			}
		}
	}
	collect(statement.DDL.Columns)
	for i := range statement.DDL.Alter {
		if column := statement.DDL.Alter[i].Column; column != nil && column.Definition != nil {
			collect([]spec.Column{*column.Definition})
		}
	}
	return uses
}

// reconcileSequenceRefUses poisons target completeness when a statement
// that authored sequence references could not land its owner. A proven
// landing needs nothing — the derived-edge walk already sees the new
// shape; a proven refusal needs nothing either — no edge was ever
// created. Only an undecidable owner leaves the authored edges' fate
// unknown, so the targets lose completeness (T21-R1 B2).
func (s *batchState) reconcileSequenceRefUses(statement spec.Statement) {
	uses := s.authoredSequenceRefUses(statement)
	if len(uses) == 0 {
		return
	}
	for _, use := range uses {
		if entry, ok := s.entries[use.owner]; ok && entry.state == tablePresent {
			continue
		}
		if entry, ok := s.entries[use.owner]; ok && entry.state == tableAbsent {
			// A proven refusal authored nothing — the edge set stands.
			continue
		}
		s.poisonSequenceRefTarget(s.sequenceKeyFor(use.target))
	}
}
