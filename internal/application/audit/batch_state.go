// Package audit orchestrates audit use cases at the application layer.
package audit

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// tableKnowledge is the per-table knowledge class inside one audit request.
// never-touched means the provider was not consulted yet; unknown means facts
// are unusable (provider absent or silent, earlier statement invalidated them,
// or a parse failure earlier in the batch contaminated the batch); absent and
// present are definite existence facts. unknown must never be conflated with
// absent — an unknown table may still exist.
type tableKnowledge int

const (
	tableNeverTouched tableKnowledge = iota
	tableUnknown
	tableAbsent
	tablePresent
)

// batchTableKey identifies one table fact slot inside a request as a
// structured (dialect, schema, table) triple. Schema is the effective
// statement schema — an explicit qualifier wins, the request schema is only
// the fallback — so qualified and unqualified names can never collide, and
// the key always names the fact actually consulted.
type batchTableKey struct {
	dialect spec.Dialect
	schema  string
	table   string
}

// batchTableEntry is the owned knowledge record for one table. shape holds a
// deep copy of the provider snapshot or the derived create shape; it is never
// aliased into statements — rules receive projections (copies) instead.
// A nil shape means "present but every member collection is unknown"
// (derived incomplete); a non-nil shape keeps each collection exactly as
// provided or derived — nil Columns marks a withheld column set, while
// PrimaryKey/Indexes/Constraints keep their provider meaning (loaded,
// absent/empty). state records existence knowledge while shape records
// member knowledge, keeping existence separate from completeness.
// displaySchema/displayTable keep the effective identity's casing for
// projections; the map key lowercases for identity matching.
type batchTableEntry struct {
	state         tableKnowledge
	shape         *spec.TableSnapshot
	displaySchema string
	displayTable  string
}

// batchState owns ordered prospective table facts for one audit request. It is
// function-local to the enrichment pass: entries are built lazily from the
// provider on first touch and updated by conditional-success transitions, so
// request-local facts can never leak between audits and provider reads can
// never overwrite derived state once an entry exists.
type batchState struct {
	provider MetadataProvider
	dialect  spec.Dialect
	schema   string
	entries  map[batchTableKey]*batchTableEntry
	// tempEntries holds the session-local temporary-table namespace. A
	// batch models one session: temp objects exist only when a statement
	// created them, so a never-touched slot is provably empty and a
	// same-named persistent entry is never shadowed into unreachability —
	// both slots coexist and bindings pick the right one per reference.
	tempEntries  map[batchTableKey]*batchTableEntry
	contaminated bool
	// invalidatedSchemas holds schema scopes already dropped or altered by an
	// in-batch statement — a later provider read under them would return
	// stale pre-batch facts, so resolve must answer unknown instead.
	invalidatedSchemas map[string]struct{}
	// resolvedVersion is the request's already-parsed version fact. Rules read
	// the same fact from statement metadata. apply still receives the original
	// statement, so RENAME COLUMN applicability reads this copy. Nil means the
	// request has no verified version. Online enrichment stores the observed
	// identity and does not fall back to a caller target.
	resolvedVersion *spec.VersionIdentity
	// instanceFacts is the request's observed server-level facts — the
	// ordered state reads only the TiDB clustered-index default mode for
	// declared PK handle derivation; nil means unobserved (offline or
	// provider-less). Stored here so a batch-derived shape answers the
	// session mode the statements actually execute under.
	instanceFacts *spec.InstanceFacts
}

func newBatchState(dialect spec.Dialect, schema string, provider MetadataProvider) *batchState {
	return &batchState{
		provider:           provider,
		dialect:            dialect,
		schema:             strings.TrimSpace(schema),
		entries:            make(map[batchTableKey]*batchTableEntry),
		tempEntries:        make(map[batchTableKey]*batchTableEntry),
		invalidatedSchemas: make(map[string]struct{}),
	}
}

// orderedStateDialect reports whether the dialect participates in the ordered
// prospective-state pass. PostgreSQL keeps its existing per-statement metadata
// enrichment unchanged in this slice.
func orderedStateDialect(dialect spec.Dialect) bool {
	return dialect == spec.DialectMySQL || dialect == spec.DialectTiDB
}

// effectiveSchema resolves the identity schema one resolver for every use:
// the explicit target qualifier wins, the request schema is the fallback.
// Reads, keys, post-state writes, and invalidation all go through it, so a
// qualified `a.t` can never inherit facts under the request schema.
func (s *batchState) effectiveSchema(schema string, table spec.Table) string {
	if resolved := strings.TrimSpace(table.Schema); resolved != "" {
		return resolved
	}
	if resolved := strings.TrimSpace(schema); resolved != "" {
		return resolved
	}
	return s.schema
}

// keyFor resolves a table to its state key under the effective schema — the
// same schema the provider read was issued against. Dotted names stay whole:
// the table part is never split on "." and the schema part is never inferred
// from a different table's qualifier.
func (s *batchState) keyFor(schema string, table spec.Table) batchTableKey {
	return batchTableKey{
		dialect: s.dialect,
		schema:  strings.ToLower(s.effectiveSchema(schema, table)),
		table:   strings.ToLower(table.Name),
	}
}

// preState returns the projected per-statement snapshot for one target. The
// provider is consulted at most once per key with the effective identity's
// original (un-lowercased) names; a provider error aborts the audit
// unchanged (errors never degrade into evidence gaps). After contamination
// the projection is always nil (unknown) regardless of any earlier cached
// fact.
func (s *batchState) preState(ctx context.Context, schema string, table spec.Table) (*spec.TableSnapshot, error) {
	effective := s.effectiveSchema(schema, table)
	key := s.keyFor(schema, table)
	entry, err := s.resolve(ctx, key, effective, table.Name)
	if err != nil {
		return nil, err
	}
	if s.contaminated || entry.state == tableUnknown {
		return nil, nil
	}
	return projectEntry(key, entry), nil
}

// resolve returns the owned entry for a key, consulting the provider exactly
// once per key per request with the original caller-supplied names. A nil
// provider or a nil snapshot leaves the entry unknown; contamination and
// schema-scope invalidation suppress provider reads for never-touched keys.
func (s *batchState) resolve(ctx context.Context, key batchTableKey, providerSchema, providerTable string) (*batchTableEntry, error) {
	if entry, ok := s.entries[key]; ok {
		return entry, nil
	}
	entry := &batchTableEntry{state: tableUnknown}
	if _, dropped := s.invalidatedSchemas[key.schema]; s.contaminated || s.provider == nil || dropped {
		s.entries[key] = entry
		return entry, nil
	}
	snapshot, err := s.provider.LoadTableSnapshot(ctx, s.dialect, providerSchema, providerTable)
	if err != nil {
		return nil, err
	}
	if snapshot == nil {
		s.entries[key] = entry
		return entry, nil
	}
	entry.displaySchema = providerSchema
	entry.displayTable = providerTable
	entry.shape = cloneTableSnapshot(snapshot)
	// The provider's catalog load covers outbound foreign keys only — no
	// reverse-edge query exists — so a provider-loaded present snapshot
	// cannot vouch the inbound set: mark it unproven. TRUNCATE's 1701
	// premise then reads batch-proven edges and gaps on the unvouched
	// tail instead of fabricating a proven-empty set. The flag lives on
	// the cloned shape; the provider's own snapshot stays untouched.
	if snapshot.Exists {
		entry.state = tablePresent
		entry.shape.ReferencedByUnknown = true
	} else {
		entry.state = tableAbsent
	}
	s.entries[key] = entry
	return entry, nil
}

// projectEntry renders one statement-visible snapshot from an owned entry.
// Definite entries return fresh copies — a provider-partial shape keeps its
// independently known members (primary key, indexes, options) while a nil
// Columns still marks the withheld column set. A nil shape (derived
// incomplete) projects existence plus unknown markers on every member
// collection, so rule code distinguishes "set not provided" from an audited
// empty set instead of fabricating absent members.
func projectEntry(key batchTableKey, entry *batchTableEntry) *spec.TableSnapshot {
	displaySchema, displayTable := entry.displaySchema, entry.displayTable
	if displayTable == "" {
		displaySchema, displayTable = key.schema, key.table
	}
	if entry.state == tableAbsent {
		return &spec.TableSnapshot{
			Schema:      displaySchema,
			Table:       &spec.Table{Schema: displaySchema, Name: displayTable},
			Exists:      false,
			Columns:     []spec.Column{},
			Indexes:     []spec.Index{},
			Constraints: []spec.Constraint{},
		}
	}
	if entry.state == tablePresent {
		if entry.shape != nil {
			return cloneTableSnapshot(entry.shape)
		}
		return &spec.TableSnapshot{
			Schema:              displaySchema,
			Table:               &spec.Table{Schema: displaySchema, Name: displayTable},
			Exists:              true,
			PrimaryKeyUnknown:   true,
			IndexesUnknown:      true,
			ConstraintsUnknown:  true,
			ReferencedByUnknown: true,
		}
	}
	return nil
}

// apply records the conditional post-state of one statement after its
// pre-state was captured. Unsupported or executable-but-unmodeled effects are
// settled before kind dispatch so an EXECUTE, admin statement, or unbound
// side effect can never pass through as a no-op: bound table targets
// invalidate, bound schema scopes invalidate the scope, and an unresolvable
// effect contaminates the batch. After that gate only the frozen transitions
// derive structure — a fully audited plain CREATE TABLE, an ALTER made of
// exactly one plain ADD COLUMN, an ALTER made of exactly one ordinary
// single-column MODIFY inside the frozen type template, a fully audited
// standalone CREATE INDEX, and a single-pair RENAME bounded by the frozen
// endpoint-existence table, and a single-target DROP bounded by the frozen
// drop state table — plus stat-clearing TRUNCATE/DML pass-through. Everything
// else (multi-action ALTER, unaudited aspects, multi-pair or multi-target
// effects) invalidates its bound targets because the real effect is not
// modeled honestly. A non-template MODIFY or DROP COLUMN invalidates every
// named table identity and already-loaded dependents of those identities. A
// precise replacement is withheld when a loaded foreign key cannot be
// recomputed. A dependency-free DROP COLUMN removes only that column. A statement
// whose structural premise is already known-broken (duplicate plain ADD,
// index on a confirmed-absent table) invalidates its target rather than
// fabricating the success shape. Contaminated batches derive nothing.
// The destination read inside renamePair may fail; provider errors propagate
// before any post-state is published.
func (s *batchState) apply(ctx context.Context, statement spec.Statement) error {
	if s.contaminated {
		return nil
	}
	if procedureLifecyclePreservesTableState(statement) {
		return ctx.Err()
	}
	if statement.DDL != nil && statement.DDL.Operation == spec.DDLOperationDropTable {
		return s.applyDropTable(ctx, statement)
	}
	if statement.Unsupported != nil {
		// A vendor-boundary statement never executes on this dialect (the
		// product lacks the feature — TiDB CTAS 1105, TiDB procedures,
		// MySQL-side TiDB extensions). Its pre-state is provably unchanged
		// and must not tombstone facts a refusal kept.
		if statement.Unsupported.Reason == spec.UnsupportedVendorBoundaryReason &&
			statement.Unsupported.Feature == "create_table_select" {
			// TiDB CREATE TABLE ... SELECT is a probe-proven engine refusal
			// (1105): the statement never runs, so its pre-state stands. Other
			// vendor-boundary surfaces are not whitelisted — fulltext/spatial
			// indexes can land degraded objects, procedures/bindings/admin
			// forms may execute, and none were probed for refusal here.
			return nil
		}
		// A recognized-but-unsupported statement has no bounded effect. Bound
		// table targets invalidate; a bound schema scope invalidates the
		// scope; an unresolvable scope contaminates the batch — it must not
		// pass through as a no-op and let stale facts satisfy later checks.
		// A DROP COLUMN inside that statement still uses the shared affected
		// set, so a loaded dependent is not left behind by the generic path.
		if dropColumnStatement(statement) {
			return s.applyDropColumn(ctx, statement, firstDropColumnAlter(statement.DDL), statement.DDL.TableTargets())
		}
		if targets := boundTargets(statement); len(targets) > 0 {
			if statement.DDL != nil {
				s.invalidateDropIdentities(statement.DDL)
			} else {
				s.invalidateAll(targets)
			}
			return nil
		}
		if scope := schemaScopeName(statement.DDL); scope != "" {
			s.invalidateSchema(scope)
			return nil
		}
		s.contaminated = true
		return nil
	}
	if statement.Kind != spec.KindDDL || statement.DDL == nil {
		s.applyDMLMutation(statement)
		return nil
	}
	ddl := statement.DDL
	// The shared advanced-column assessment gates state publication,
	// independently of policy switches. A provably rejected transition must
	// not keep the prior shape reusable as a success fact, and a structurally
	// ambiguous advanced transition must not publish an ordinary shape: both
	// tombstone the affected identities plus their loaded dependents through
	// the same publication seam as the legacy non-precise paths. Dep-scope
	// uncertainty alone stays on the legacy per-action publications, which
	// already withhold success shapes. The cancellation check precedes any
	// publication from this gate.
	if err := ctx.Err(); err != nil {
		return err
	}
	assessment := spec.AssessAdvancedColumns(statement)
	indexAssessment := spec.AssessAdvancedIndexes(statement)
	if assessment.InvalidTransition || indexAssessment.InvalidTransition ||
		(assessment.UnresolvedTransition && len(assessment.Features) > 0) ||
		(indexAssessment.UnresolvedTransition && len(indexAssessment.Features) > 0) {
		targetKeys := s.modifyTargetKeys(ddl.TableTargets())
		publication := modifyPublication{
			invalidate: append(s.modifyIdentityDependentKeys(targetKeys), targetKeys...),
		}
		// Cancellation is rechecked after the assessment and dependent-set
		// work, immediately before the publication lands.
		if err := ctx.Err(); err != nil {
			return err
		}
		publication.apply(s)
		return nil
	}
	switch ddl.Operation {
	case spec.DDLOperationCreateTable:
		return s.applyCreateTable(ctx, statement)
	case spec.DDLOperationAlterTable:
		return s.applyAlterTable(ctx, statement)
	case spec.DDLOperationCreateIndex:
		s.applyCreateIndex(statement)
	case spec.DDLOperationDropIndex:
		s.applyStandaloneDropIndex(statement)
	case spec.DDLOperationTruncateTable:
		return s.applyTruncateTable(ctx, statement)
	case spec.DDLOperationRenameTable:
		return s.applyRenameTable(ctx, statement)
	default:
		// No modeled transition: schema-scoped operations invalidate their
		// scope, table-bound operations invalidate their targets.
		if scope := schemaScopeName(ddl); scope != "" {
			s.invalidateSchema(scope)
			return nil
		}
		s.invalidateAll(ddl.TableTargets())
	}
	return nil
}

func procedureLifecyclePreservesTableState(statement spec.Statement) bool {
	if !orderedStateDialect(statement.Dialect) || statement.Kind != spec.KindDDL || statement.DDL == nil ||
		statement.ResourceLimit != nil || statement.DML != nil || statement.DDL.Table != nil || len(statement.DDL.Targets) != 0 {
		return false
	}
	switch statement.DDL.Operation {
	case spec.DDLOperationCreateProcedure, spec.DDLOperationDropProcedure:
		return true
	default:
		return false
	}
}

// boundTargets returns the table identities a statement's effect is bound to,
// whichever payload carries them — DDL targets, or DML mutation targets for
// execution-capable non-DDL statements.
func boundTargets(statement spec.Statement) []spec.Table {
	if statement.DDL != nil {
		return statement.DDL.TableTargets()
	}
	if statement.DML != nil {
		return statement.DML.MutationTargetTables()
	}
	return nil
}

// schemaScopeName returns the schema identity bound by a schema-scoped DDL
// operation (DROP/ALTER DATABASE or SCHEMA), or "" when the statement is not
// schema-scoped or its scope cannot be named.
func schemaScopeName(ddl *spec.DDL) string {
	if ddl == nil {
		return ""
	}
	switch ddl.Operation {
	case spec.DDLOperationDropSchema, spec.DDLOperationAlterSchema:
		return strings.TrimSpace(ddl.ObjectName)
	}
	return ""
}

// fullyAuditedStatement reports whether every parsed aspect of the statement
// is consumed by the audit model. Unsupported markers and coverage aspect gaps
// both mean the statement's effect is not bounded enough to model.
func fullyAuditedStatement(dialect spec.Dialect, statement spec.Statement) bool {
	if statement.Unsupported != nil {
		return false
	}
	for _, aspect := range statementCoverageAspects(dialect, statement) {
		// A probed vendor refusal is the audit's answer, not missing
		// analysis — the six TiDB-refused maintenance actions execute
		// nothing, so the ordered pre-state stands unchanged. Every
		// other aspect keeps the statement unproven for state purposes.
		if aspect.Reason == spec.UnsupportedVendorBoundaryReason &&
			aspect.Metadata["aspect"] == "action" {
			if _, refused := tidbVendorRefusedAlterActions[fmt.Sprint(aspect.Metadata["action"])]; refused {
				continue
			}
		}
		// A proven vendor-refused table option (TiDB 8200/8233/8232,
		// probed) is the statement's answer — the option transition keeps
		// the pre-state rather than tombstoning it.
		if aspect.Reason == spec.UnsupportedVendorBoundaryReason &&
			aspect.Metadata["aspect"] == "option" {
			continue
		}
		// A vendor-refused temporary clause — GLOBAL TEMPORARY syntax on
		// MySQL, ON COMMIT PRESERVE ROWS on TiDB (8200) — is likewise the
		// statement's answer: it never executes, the pre-state stands.
		if aspect.Reason == spec.UnsupportedVendorBoundaryReason &&
			aspect.Metadata["aspect"] == "temporary" {
			continue
		}
		return false
	}
	return true
}

// partitionStateAdmissible reports whether a single partition-action
// statement's unmodelled aspects stay inside the bounded set its ordered
// transition adjudicates itself: probed vendor-refusal action aspects
// (which prove refusal outright) and partition-bound aspects (which the
// bound outcome evaluates per member, refusing the whole statement on a
// provable violation and degrading only the partition facts when a
// comparison stays undecidable). Unsupported markers and every other
// aspect keep the statement fail-closed.
func partitionStateAdmissible(dialect spec.Dialect, statement spec.Statement) bool {
	if statement.Unsupported != nil {
		return false
	}
	for _, aspect := range statementCoverageAspects(dialect, statement) {
		if aspect.Reason == spec.UnsupportedVendorBoundaryReason &&
			aspect.Metadata["aspect"] == "action" {
			if _, refused := tidbVendorRefusedAlterActions[fmt.Sprint(aspect.Metadata["action"])]; refused {
				continue
			}
			return false
		}
		if aspect.Metadata["aspect"] == "partition_bound" {
			continue
		}
		return false
	}
	return true
}

func (s *batchState) invalidateAll(targets []spec.Table) {
	for _, target := range targets {
		key := s.keyFor(s.schema, target)
		s.entries[key] = &batchTableEntry{state: tableUnknown}
		// An unmodeled effect cannot be proven to have missed the session
		// slot either — the temp identity tombstones with the catalog one.
		s.tempEntries[key] = &batchTableEntry{state: tableUnknown}
	}
}

// invalidateSchema marks an entire schema scope unsafe to trust: existing
// entries under it tombstone to unknown, and resolve must answer unknown for
// later first touches — the pre-batch provider facts no longer hold.
func (s *batchState) invalidateSchema(schema string) {
	lower := strings.ToLower(strings.TrimSpace(schema))
	if lower == "" {
		s.contaminated = true
		return
	}
	s.invalidatedSchemas[lower] = struct{}{}
	for key := range s.entries {
		if key.schema == lower {
			s.entries[key] = &batchTableEntry{state: tableUnknown}
		}
	}
	for key := range s.tempEntries {
		if key.schema == lower {
			s.tempEntries[key] = &batchTableEntry{state: tableUnknown}
		}
	}
}

func (s *batchState) invalidateKey(key batchTableKey) {
	s.entries[key] = &batchTableEntry{state: tableUnknown}
}

// presentIncompleteEntry records a table whose existence was proven but whose
// member collections are entirely unknown (shape nil), under the effective
// identity's casing.
func presentIncompleteEntry(schema, table string) *batchTableEntry {
	return &batchTableEntry{
		state:         tablePresent,
		displaySchema: schema,
		displayTable:  table,
	}
}

// applyCreateTable publishes the namespace-aware create outcome. The
// destination binds by scope: LOCAL TEMPORARY lands in the session
// namespace (where a same-named catalog object is left untouched), plain
// and GLOBAL TEMPORARY creates land in the persistent catalog (a temp
// shadow does not occupy the catalog name — the persistent create
// succeeds under it, probed on all four anchors). The outcome matrix:
//
//   - vendor refusal — GLOBAL TEMPORARY on MySQL, ON COMMIT PRESERVE ROWS
//     on TiDB: the statement never executes, pre-state stands.
//   - proven refusal — bound destination proven-present without
//     IF NOT EXISTS (1050): pre-state stands, never an invalidate.
//   - conditional no-op — bound destination proven-present under
//     IF NOT EXISTS: the fact stands exactly as bound.
//   - proven/conditional create — absent or unvouched destination: LIKE
//     copies the source's member shape (a proven-absent source refuses
//     the whole statement, 1146, and the destination stands; an
//     unvouched source keeps the destination conditional-incomplete);
//     CTAS publishes the declared column prefix plus ColumnsTailUnknown
//     (select-derived members are never inferred); a plain/conditional
//     create derives the declared shape — IF NOT EXISTS on an unvouched
//     destination can only prove existence, so it lands
//     present-incomplete.
//
// LIKE/CTAS source bindings are temp-first: a TiDB local-temp LIKE
// source is a proven 8006 refusal that keeps the destination's
// pre-state.
func (s *batchState) applyCreateTable(ctx context.Context, statement spec.Statement) error {
	ddl := statement.DDL
	targets := ddl.TableTargets()
	if len(targets) == 0 || ddl.Table == nil {
		return nil
	}
	if temporaryClauseVendorRefused(s.dialect, ddl) {
		return nil
	}
	dest := targets[0]
	key := s.keyFor(s.schema, dest)
	displaySchema, displayTable := s.effectiveSchema(s.schema, dest), dest.Name
	ifNotExists := ddl.Options["if_not_exists"] == "true"
	tempDest := ddl.TemporaryScope == spec.TemporaryScopeLocal
	dstMap := s.entries
	if tempDest {
		dstMap = s.tempEntries
	}
	if !createAspectAdmissible(s.dialect, statement) {
		s.invalidateBoundKey(key, destNamespace(tempDest))
		return nil
	}
	var entry *batchTableEntry
	if tempDest {
		// The temp slot is session-local: never-touched is provably empty.
		entry = s.tempEntries[key]
	} else {
		var err error
		entry, err = s.resolve(ctx, key, displaySchema, displayTable)
		if err != nil {
			return err
		}
	}
	entryState := boundEntryState(destNamespace(tempDest), entry)
	// A proven-present destination settles the statement before any
	// premise engages: IF NOT EXISTS no-ops, a bare create refuses 1050 —
	// either way the bound fact stands. An unvouched temp slot stays
	// tombstoned (the session slot cannot be recovered conservatively).
	if entryState == tablePresent {
		return nil
	}
	if tempDest {
		if entryState == tableUnknown {
			return nil
		}
		// Temp destinations cannot carry foreign keys or partitions
		// natively — declaring either makes the create's landing
		// undecidable, so the slot stays unvouched rather than publishing
		// a shape the engine may have refused.
		if hasDeclaredForeignKey(ddl) || ddl.HasPartition {
			s.invalidateTempKey(key)
			return nil
		}
		shape, outcome, err := s.createDestShape(ctx, statement, displaySchema, displayTable)
		if err != nil {
			return err
		}
		switch outcome {
		case createShapeRefused:
			return nil
		case createShapeUnproven:
			s.invalidateTempKey(key)
			return nil
		}
		dstMap[key] = &batchTableEntry{state: tablePresent, shape: shape, displaySchema: displaySchema, displayTable: displayTable}
		return nil
	}
	// Persistent destination: an unvouched pre-state stays conditional —
	// the outcome matrix below decides per source premise, mirroring the
	// plain-create doctrine (unknown + plain create derives the shape;
	// unknown + IF NOT EXISTS proves existence only).
	// Declared foreign keys carry the same landing premise an ALTER-side
	// add does, decided against the pre-state. A proven refusal fails the
	// statement before the table lands (the entry is provably unchanged);
	// an unprovable premise cannot publish the declared shape because the
	// statement may never have landed.
	if entryState == tableAbsent || entryState == tableUnknown {
		refused, decided, err := s.createTableForeignKeyPremise(ctx, key, displaySchema, displayTable, ddl)
		if err != nil {
			return err
		}
		if !decided {
			s.invalidateKey(key)
			return nil
		}
		if refused {
			return nil
		}
	}
	// A declared CHECK carrying an authored enforcement clause is a proven
	// syntax refusal on MySQL's parse-and-ignore arm (5.7 answers 1064, not
	// silent drop) — the statement fails atomically and the entry keeps its
	// pre-state.
	arm := s.checkArmFor()
	for _, member := range ddl.Constraints {
		if spec.CheckExplicitClauseRefused(s.dialect, arm, member) {
			return nil
		}
		if member.Type == "check" && member.EnforcedUnproven &&
			s.dialect == spec.DialectMySQL && arm == spec.CheckArmIgnored {
			// The authored clause could not be attributed: the statement may
			// have refused 1064 or landed the table — existence is
			// undecidable, so the entry is unvouched rather than published.
			s.invalidateKey(key)
			return nil
		}
	}
	// A provably-invalid declared partition scheme refuses the whole
	// statement natively — the same judgment ALTER PARTITION BY consumes
	// gates the publish here; an unproven scheme unvouches the target.
	if entryState == tableAbsent || entryState == tableUnknown {
		if ddl.HasPartition {
			switch partitionSchemeOutcome(ddl.PartitionType, ddl.PartitionDefs, ddl.PartitionCount) {
			case partitionOutcomeRefused:
				return nil
			case partitionOutcomeUnproven:
				s.invalidateKey(key)
				return nil
			}
		}
	}
	shape, outcome, err := s.createDestShape(ctx, statement, displaySchema, displayTable)
	if err != nil {
		return err
	}
	switch outcome {
	case createShapeRefused:
		return nil
	case createShapeUnproven:
		// A withheld member set can still carry conditional existence:
		// IF NOT EXISTS on an unvouched destination proves presence only;
		// a LIKE/CTAS whose source premise stayed unproven leaves the
		// destination tombstoned — the create may never have run.
		if ifNotExists && !ddl.HasReferTable && !ddl.HasSelect {
			s.entries[key] = presentIncompleteEntry(displaySchema, displayTable)
			return nil
		}
		s.invalidateKey(key)
		return nil
	}
	if ifNotExists && entryState == tableUnknown {
		// IF NOT EXISTS on an unvouched destination can only prove
		// existence — either the pre-existing table stood or the create
		// landed; the member set differs between those outcomes.
		s.entries[key] = presentIncompleteEntry(displaySchema, displayTable)
		return nil
	}
	s.entries[key] = &batchTableEntry{
		state:         tablePresent,
		shape:         shape,
		displaySchema: displaySchema,
		displayTable:  displayTable,
	}
	for i := range ddl.Constraints {
		member := &ddl.Constraints[i]
		if member.Type != "foreign_key" {
			continue
		}
		if err := s.recordInboundEdge(ctx, key, dest, *member); err != nil {
			return err
		}
	}
	return nil
}

// firstRenameTableAlter returns the ALTER-side rename_table action when
// the statement carries one, or nil.
func firstRenameTableAlter(ddl *spec.DDL) *spec.Alter {
	for i := range ddl.Alter {
		if ddl.Alter[i].Action == "rename_table" {
			return &ddl.Alter[i]
		}
	}
	return nil
}

// hasDeclaredForeignKeyAction reports whether any ALTER action declares a
// foreign-key constraint (add_constraint carrying a foreign_key member or
// the add_foreign_key action form).
func hasDeclaredForeignKeyAction(ddl *spec.DDL) bool {
	for i := range ddl.Alter {
		alter := &ddl.Alter[i]
		if alter.Action == "add_foreign_key" {
			return true
		}
		if alter.Action == "add_constraint" && alter.Constraint != nil && alter.Constraint.Type == "foreign_key" {
			return true
		}
	}
	return false
}

func destNamespace(tempDest bool) boundNamespace {
	if tempDest {
		return namespaceTemporary
	}
	return namespacePersistent
}

type createShapeOutcome int

const (
	createShapeDerived  createShapeOutcome = iota // publish the returned shape
	createShapeRefused                            // proven refusal — keep pre-state
	createShapeUnproven                           // undecidable — caller decides
)

// temporaryClauseVendorRefused reports the temporary-clause forms the
// target product provably refuses: GLOBAL TEMPORARY syntax on MySQL and
// ON COMMIT PRESERVE ROWS on TiDB (8200). The statement never executes —
// callers keep every pre-state. ON COMMIT is a create-time clause, so the
// TiDB preserve check only fires for CREATE — a DROP GLOBAL TEMPORARY
// has no attribute to refuse and is modeled.
func temporaryClauseVendorRefused(dialect spec.Dialect, ddl *spec.DDL) bool {
	if ddl.TemporaryScope != spec.TemporaryScopeGlobal {
		return false
	}
	if dialect == spec.DialectMySQL {
		return true
	}
	return dialect == spec.DialectTiDB && ddl.Operation == spec.DDLOperationCreateTable && !ddl.OnCommitDelete
}

// createAspectAdmissible reports whether every coverage aspect on the
// statement is either modeled or a proven vendor refusal this flow
// adjudicates itself (temporary-clause refusals keep pre-state; partition
// bound aspects are judged inside; the CTAS duplicate-key clause is a
// data-only modifier with no member-shape effect). Everything else is
// fail-closed.
func createAspectAdmissible(dialect spec.Dialect, statement spec.Statement) bool {
	ddl := statement.DDL
	if statement.Unsupported != nil || ddl.OmittedTargets != 0 {
		return false
	}
	for _, aspect := range statementCoverageAspects(dialect, statement) {
		if aspect.Reason == spec.UnsupportedVendorBoundaryReason && aspect.Metadata["aspect"] == "temporary" {
			continue
		}
		if aspect.Metadata["aspect"] == "partition_bound" {
			continue
		}
		if aspect.Metadata["aspect"] == "option" && strings.HasSuffix(aspect.Feature, ".on_duplicate") {
			continue
		}
		return false
	}
	return true
}

// createDestShape derives the destination's post-shape. LIKE clones the
// bound source's member set; CTAS publishes the declared column prefix
// with ColumnsTailUnknown; a plain create derives the declared shape; a
// global temporary lands the same shape plus the GlobalTemporary marker.
// A proven-absent or refused source is createShapeRefused (keep
// pre-state); an unvouched source is createShapeUnproven.
func (s *batchState) createDestShape(ctx context.Context, statement spec.Statement, displaySchema, displayTable string) (*spec.TableSnapshot, createShapeOutcome, error) {
	ddl := statement.DDL
	if ddl.HasReferTable && ddl.ReferTable != nil {
		ns, entry, _, err := s.bindSource(ctx, *ddl.ReferTable)
		if err != nil {
			return nil, createShapeRefused, fmt.Errorf("load table snapshot for %s: %w", ddl.ReferTable.Name, err)
		}
		if ns == namespaceTemporary && s.dialect == spec.DialectTiDB {
			// TiDB refuses LIKE on a local-temporary source (8006) —
			// proven refusal, destination keeps pre-state.
			return nil, createShapeRefused, nil
		}
		if ns == namespaceAmbiguous {
			return nil, createShapeUnproven, nil
		}
		state := boundEntryState(ns, entry)
		switch state {
		case tableAbsent:
			// Missing LIKE source refuses the statement (1146).
			return nil, createShapeRefused, nil
		case tablePresent:
			return likeCopiedShape(entry.shape, displaySchema, displayTable), createShapeDerived, nil
		default:
			return nil, createShapeUnproven, nil
		}
	}
	if ddl.HasSelect {
		for _, source := range ddl.SelectSources {
			ns, entry, _, err := s.bindSource(ctx, source)
			if err != nil {
				return nil, createShapeRefused, fmt.Errorf("load table snapshot for %s: %w", source.Name, err)
			}
			switch boundEntryState(ns, entry) {
			case tableAbsent:
				// A proven-missing select source refuses the statement
				// (1146) — the destination's pre-state stands.
				return nil, createShapeRefused, nil
			case tablePresent:
			default:
				return nil, createShapeUnproven, nil
			}
		}
		return ctasDeclaredShape(displaySchema, displayTable, ddl), createShapeDerived, nil
	}
	shape := s.derivedCreateShape(displaySchema, displayTable, ddl)
	if ddl.TemporaryScope == spec.TemporaryScopeGlobal {
		shape.GlobalTemporary = true
		if shape.Options == nil {
			shape.Options = map[string]string{}
		}
		shape.Options["on_commit"] = "delete"
	}
	return shape, createShapeDerived, nil
}

// likeCopiedShape projects the member set CREATE TABLE ... LIKE provably
// copies, per the mysql80 probe: columns (with generated definitions),
// indexes, primary key, check constraints (server-renamed → NameUnknown),
// partitioning and options — minus the auto_increment counter and any
// foreign_key constraint (an FK's implicit supporting index lands as a
// plain index; the constraint object does not copy). A nil source shape
// (present-incomplete) yields a present-incomplete destination.
func likeCopiedShape(source *spec.TableSnapshot, displaySchema, displayTable string) *spec.TableSnapshot {
	if source == nil {
		return nil
	}
	shape := cloneTableSnapshot(source)
	shape.Schema = displaySchema
	shape.Table = &spec.Table{Schema: displaySchema, Name: displayTable}
	kept := shape.Constraints[:0]
	for _, member := range shape.Constraints {
		switch member.Type {
		case "foreign_key":
			// Foreign keys do not copy; their implicit supporting index
			// members land as plain indexes in the clone's Indexes.
			continue
		case "check":
			// The engine renames CHECK objects to <dest>_chk_N — the
			// member lands but its stored name is unproven.
			member.NameUnknown = true
		}
		kept = append(kept, member)
	}
	shape.Constraints = kept
	if shape.Options != nil {
		delete(shape.Options, "auto_increment")
	}
	shape.ReferencedBy = []spec.Table{}
	shape.ReferencedByUnknown = false
	return shape
}

// ctasDeclaredShape publishes the only provable member facts of a CTAS
// destination: the authored column definitions land as the column prefix
// and every select-appended member stays unproven — the select is never
// executed or result-inferred.
func ctasDeclaredShape(displaySchema, displayTable string, ddl *spec.DDL) *spec.TableSnapshot {
	shape := &spec.TableSnapshot{
		Schema:             displaySchema,
		Table:              &spec.Table{Schema: displaySchema, Name: displayTable, Comment: ddl.Table.Comment},
		Exists:             true,
		Columns:            make([]spec.Column, 0, len(ddl.Columns)),
		ColumnsTailUnknown: true,
		Indexes:            []spec.Index{},
		Constraints:        []spec.Constraint{},
		ReferencedBy:       []spec.Table{},
	}
	for _, column := range ddl.Columns {
		shape.Columns = append(shape.Columns, cloneColumn(column))
	}
	// Declared PK/index members on a CTAS are column-level attributes the
	// engine honors or ignores per the select's landing; the member truth
	// beyond the declared column prefix is unproven.
	shape.PrimaryKeyUnknown = len(ddl.Columns) > 0
	return shape
}

func hasDeclaredForeignKey(ddl *spec.DDL) bool {
	for _, member := range ddl.Constraints {
		if member.Type == "foreign_key" {
			return true
		}
	}
	return false
}

// derivedCreateShape derives the declared post-state and folds in each
// foreign key's implicit supporting index — CREATE and ALTER share the
// native derivation rule through publishForeignKeySupportingIndex, so a
// create-derived member leaves the same index facts an add would.
func (s *batchState) derivedCreateShape(displaySchema, displayTable string, ddl *spec.DDL) *spec.TableSnapshot {
	shape := derivedCreateShape(s.dialect, displaySchema, displayTable, ddl, s.tidbClusteredMode(), s.tidbAlterPrimaryKey())
	// CHECK members land only on enforcing arms: parse+ignore arms (MySQL
	// <8.0.16, TiDB flag OFF) store nothing, and an unobserved arm cannot
	// prove either direction — the declared members leave the collection
	// unvouched rather than fabricating presence or absence.
	switch s.checkArmFor() {
	case spec.CheckArmIgnored:
		kept := shape.Constraints[:0]
		for _, member := range shape.Constraints {
			if member.Type != "check" {
				kept = append(kept, member)
			}
		}
		shape.Constraints = kept
	case spec.CheckArmUnknown:
		hadCheck := false
		kept := shape.Constraints[:0]
		for _, member := range shape.Constraints {
			if member.Type == "check" {
				hadCheck = true
				continue
			}
			kept = append(kept, member)
		}
		shape.Constraints = kept
		if hadCheck {
			shape.ConstraintsUnknown = true
		}
	}
	for i := range shape.Constraints {
		member := &shape.Constraints[i]
		if member.Type != "foreign_key" {
			continue
		}
		authored := ""
		if !member.NameUnknown {
			authored = member.Name
		}
		s.publishForeignKeySupportingIndex(shape, *member, authored)
	}
	return shape
}

func derivedCreateShape(dialect spec.Dialect, schema, table string, ddl *spec.DDL, tidbClusteredDefault *string, tidbAlterPK *bool) *spec.TableSnapshot {
	snapshot := &spec.TableSnapshot{
		Schema:      schema,
		Table:       &spec.Table{Schema: schema, Name: table, Comment: ddl.Table.Comment},
		Exists:      true,
		Columns:     make([]spec.Column, 0, len(ddl.Columns)),
		Indexes:     make([]spec.Index, 0, len(ddl.Indexes)),
		Constraints: make([]spec.Constraint, 0, len(ddl.Constraints)),
	}
	for _, column := range ddl.Columns {
		snapshot.Columns = append(snapshot.Columns, cloneColumn(column))
	}
	for _, index := range ddl.Indexes {
		snapshot.Indexes = append(snapshot.Indexes, cloneIndex(index))
	}
	// Stored names follow the product's auto-naming rule, not the authored
	// declaration — dialect-bound inside StoredIndexNames (MySQL claims
	// first-column names with reserved-`primary` suffixing; TiDB
	// pre-occupies authored names and derives expression_index). When any
	// name cannot be proven the members stay known but the name space is
	// unvouched, so name-based successors report a gap instead of reading
	// an unmatched name as confirmed absence.
	if resolved, provable := spec.StoredIndexNames(dialect, snapshot.Indexes); provable {
		snapshot.Indexes = resolved
	} else {
		snapshot.IndexNamesUnknown = true
		// Cross-member suffix effects make per-member name honesty
		// all-or-nothing here: every member's stored name is unproven,
		// while members merged later by an authored ADD/CREATE carry a
		// provable name.
		for i := range snapshot.Indexes {
			snapshot.Indexes[i].NameUnknown = true
		}
	}
	// A batch-derived index list is the declaration order — the same order
	// the product selects the implicit primary role in.
	snapshot.IndexOrderCatalog = true
	partitioned := ddl.HasPartition
	snapshot.Partitioned = &partitioned
	if partitioned {
		if ddl.PartitionType != "" {
			partitionType := ddl.PartitionType
			snapshot.PartitionType = &partitionType
		}
		// A nil declared list is an unobserved member space (subpartitioned
		// or interval-shaped) — never collapse it into a known-empty set.
		if ddl.PartitionNames != nil {
			snapshot.PartitionNames = append([]string{}, ddl.PartitionNames...)
		}
		snapshot.PartitionDefs = spec.ClonePartitionDefs(ddl.PartitionDefs)
		snapshot.Subpartitioned = ddl.Subpartitioned
	} else {
		// A declared-plain CREATE observes an empty member set, never an
		// unobserved one.
		snapshot.PartitionNames = []string{}
	}
	if ddl.PrimaryKey != nil {
		pk := cloneIndex(*ddl.PrimaryKey)
		// The stored PK index object is always named PRIMARY on both
		// anchors — an authored CONSTRAINT symbol names the constraint
		// declaration (linted by the naming rules), never the object
		// (verified constraint_named_PRIMARY=1 under an explicit symbol).
		pk.Name = "primary"
		snapshot.PrimaryKey = &pk
		if handle := spec.PrimaryKeyHandle(dialect, &pk, snapshot.Columns, tidbClusteredDefault, tidbAlterPK); handle != nil {
			snapshot.PrimaryKeyIsHandle = *handle
		} else {
			// The declared PK cannot prove handle vs real object (unwritten
			// CLUSTERED|NONCLUSTERED with no observed session mode) — the
			// snapshot must not answer either way.
			snapshot.PrimaryKeyHandleUnknown = true
		}
		if clustered := spec.PrimaryKeyClustered(dialect, &pk, snapshot.Columns, tidbClusteredDefault, tidbAlterPK); clustered != nil {
			snapshot.PrimaryKeyClustered = *clustered
		} else {
			// Clustered storage is a separate question from handle identity:
			// a common-handle PK is clustered yet addressable. When the
			// answer is unprovable the snapshot withholds it rather than
			// guessing a droppable drop target.
			snapshot.PrimaryKeyClusteredUnknown = true
		}
	}
	for _, constraint := range ddl.Constraints {
		member := cloneConstraint(constraint)
		if member.Type == "foreign_key" && member.Name == "" {
			// A bare declared foreign key gets a server-derived catalog
			// name (ibfk_N / fk_N) that is never authored — the stored
			// name stays unproven exactly like a bare ALTER-side add.
			member.NameUnknown = true
		}
		snapshot.Constraints = append(snapshot.Constraints, member)
	}
	if len(ddl.Options) > 0 {
		snapshot.Options = make(map[string]string, len(ddl.Options))
		for name, value := range ddl.Options {
			snapshot.Options[name] = value
		}
	}
	return snapshot
}

// tidbClusteredMode returns the request-observed
// tidb_enable_clustered_index mode — nil when unobserved, in which case a
// batch-derived PK with an unwritten type stays unknown rather than
// inheriting the product's ON default as a guess.
func (s *batchState) tidbClusteredMode() *string {
	if s.instanceFacts == nil || s.instanceFacts.TiDBEnableClusteredIndex == "" {
		return nil
	}
	mode := s.instanceFacts.TiDBEnableClusteredIndex
	return &mode
}

// tidbAlterPrimaryKey returns the request-observed alter-primary-key
// configuration fact the int_only clustered mode defers to — nil when
// unobserved, in which case an int_only single-int PK's handle answer stays
// unknown rather than collapsing to either shape.
func (s *batchState) tidbAlterPrimaryKey() *bool {
	if s.instanceFacts == nil || !s.instanceFacts.TiDBAlterPrimaryKeyKnown {
		return nil
	}
	alterPK := s.instanceFacts.TiDBAlterPrimaryKey
	return &alterPK
}

// applyAlterTable models the frozen transitions a fully audited ALTER can
// carry: a bounded multi-action successor (dialect-parametrized — MySQL
// accumulated working state vs TiDB pre-statement premises), exactly one
// plain or positioned ADD COLUMN, exactly one ordinary single-column
// MODIFY or CHANGE/RENAME COLUMN (each optionally positioned), or exactly
// one RENAME TO handled by the shared rename pair migration. Anything
// else — unprovable actions, scalar-conditioned adds, or unaudited
// shapes — has no modeled post-state, so its bound targets invalidate
// rather than fabricating a success shape (a duplicate add inside a
// multi-action ALTER must never produce a definite column set).
// Premise-broken adds — a duplicate column without IF NOT EXISTS, or any
// add on a confirmed-absent table — invalidate instead of fabricating a
// success structure. MODIFY never restores an unknown table.
func (s *batchState) applyAlterTable(ctx context.Context, statement spec.Statement) error {
	ddl := statement.DDL
	targets := ddl.TableTargets()
	// TiDB refuses every ALTER TABLE form on a local temporary table
	// (8200) — a proven refusal that keeps the session object's exact
	// pre-state. The check binds the primary target temp-first, matching
	// the reference semantics; a tombstoned temp slot is ambiguous and
	// falls through to the ordinary gates.
	if s.dialect == spec.DialectTiDB && len(targets) > 0 {
		if temp := s.tempEntry(s.keyFor(s.schema, targets[0])); temp != nil && temp.state == tablePresent {
			return nil
		}
	}
	// MySQL supports ALTER TABLE on a local temporary table — the actions
	// bind the session object (probed: ALTER tp ADD COLUMN works on 5.7/
	// 8.0/8.4). The transition helpers all operate on the bound slot map,
	// so a temp-bound primary target swaps the maps for the duration of
	// the dispatch and every helper keeps its shared semantics verbatim.
	// A tombstoned temp slot makes the read ambiguous — tombstone both
	// slots rather than pick an object.
	if len(targets) > 0 {
		primaryKey := s.keyFor(s.schema, targets[0])
		temp := s.tempEntry(primaryKey)
		if temp != nil && temp.state == tableUnknown {
			s.invalidateTempKey(primaryKey)
			s.invalidateKey(primaryKey)
			return nil
		}
		if temp != nil && temp.state == tablePresent {
			if alter := firstRenameTableAlter(ddl); alter != nil {
				// ALTER TABLE temp RENAME TO ... migrates inside the
				// session namespace — applyRenamePair binds endpoints
				// itself and must see the real maps.
				if len(ddl.Alter) != 1 || len(targets) != 2 || !fullyAuditedStatement(s.dialect, statement) {
					s.invalidateTempKey(primaryKey)
					return nil
				}
				return s.applyRenamePair(ctx, statement, targets[0], targets[1])
			}
			// A local temp object cannot carry foreign keys or a
			// partition scheme — declaring either leaves the outcome
			// undecidable rather than publishing into a slot the engine
			// refused or silently rewrote.
			if hasDeclaredForeignKeyAction(ddl) || hasPartitionAlterAction(ddl) {
				s.invalidateTempKey(primaryKey)
				return nil
			}
			s.entries, s.tempEntries = s.tempEntries, s.entries
			defer func() { s.entries, s.tempEntries = s.tempEntries, s.entries }()
		}
	}
	// A partition action sharing a multi-spec ALTER is settled by the
	// probed combination matrix, not by a blanket refusal: structural
	// siblings, table-option siblings, and multiple partition actions are
	// proven refusals (MySQL 1064, TiDB 8200), a single supported
	// ALGORITHM/LOCK modifier is gated by the probed value+version matrix
	// (ALGORITHM=INPLACE+DROP refuses on MySQL 5.x, executes on 8+/TiDB),
	// and every unprobed combination invalidates rather than claiming
	// either outcome.
	if len(ddl.Alter) > 1 && hasPartitionAlterAction(ddl) {
		switch s.partitionMixedOutcome(ddl) {
		case partitionOutcomeRefused:
			return nil
		case partitionOutcomeCombinable:
			var partitionAlter *spec.Alter
			for i := range ddl.Alter {
				if isPartitionAlterAction(ddl.Alter[i].Action) {
					partitionAlter = &ddl.Alter[i]
					break
				}
			}
			s.applyPartitionOperation(targets, partitionAlter)
			return nil
		default:
			s.invalidateAll(targets)
			return nil
		}
	}
	// A multi-action ALTER is one statement: its net successor is the bounded
	// dialect-parametrized model, or one tombstone — never a prefix of
	// applied actions and never an action-by-action dispatch.
	if len(ddl.Alter) > 1 && orderedStateDialect(s.dialect) {
		return s.applyMultiActionAlter(ctx, statement, targets)
	}
	// A non-template MODIFY is not a success shape. Route it through the shared
	// publication so every named table identity, including a RENAME destination
	// that was never loaded, and every already-loaded dependent of those
	// identities, is tombstoned together.
	if modify := firstModifyAlter(ddl); modify != nil && len(targets) > 0 &&
		(!fullyAuditedStatement(s.dialect, statement) || len(ddl.Alter) != 1) {
		return s.applyModifyColumn(ctx, statement, modify, targets)
	}
	// A non-template CHANGE or RENAME COLUMN is not a success shape. Route it
	// through the shared publication so every named table identity and every
	// already-loaded identity dependent is tombstoned together.
	if ident := firstColumnIdentityAlter(ddl); ident != nil && len(targets) > 0 &&
		(!fullyAuditedStatement(s.dialect, statement) || len(ddl.Alter) != 1) {
		return s.applyColumnIdentity(ctx, statement, ident, targets)
	}
	// DROP COLUMN, including a precise single-column removal and a refused
	// multi-action form, publishes through one affected set. The generic
	// invalidate-all branch does not see loaded dependents.
	if drop := firstDropColumnAlter(ddl); drop != nil {
		return s.applyDropColumn(ctx, statement, drop, targets)
	}
	// DROP CHECK|CONSTRAINT carries no rule consumer, so the coverage aspect
	// still reports it — but its post-state is decidable from the proven
	// target type and must not tombstone facts it provably kept.
	if dropCheck := firstDropCheckAlter(ddl); dropCheck != nil && len(targets) > 0 {
		s.applyDropCheck(statement, dropCheck, targets)
		return nil
	}
	// ALTER CHECK|CONSTRAINT [NOT] ENFORCED flips a proven check member's
	// enforcement state — NOT ENFORCED deterministically, ENFORCED only as
	// far as the data-conditional refusal allows — never a tombstone.
	if alterCheck := firstAlterCheckAlter(ddl); alterCheck != nil && len(targets) > 0 {
		s.applyAlterCheck(statement, alterCheck, targets)
		return nil
	}
	// A single partition action runs its own bounded transition before the
	// blanket audit gate: vendor-refusal aspects prove refusal and
	// partition-bound aspects are adjudicated member-by-member inside the
	// transition itself, so neither may tombstone facts the outcome keeps.
	// Every other aspect keeps the statement fail-closed.
	if len(ddl.Alter) == 1 && len(targets) > 0 && isPartitionAlterAction(ddl.Alter[0].Action) &&
		partitionStateAdmissible(s.dialect, statement) {
		s.applyPartitionOperation(targets, &ddl.Alter[0])
		return nil
	}
	if !fullyAuditedStatement(s.dialect, statement) || len(ddl.Alter) != 1 || len(targets) == 0 {
		s.invalidateAll(targets)
		return nil
	}
	alter := &ddl.Alter[0]
	if alter.Action == "rename_table" {
		// ALTER TABLE x RENAME TO y carries the pair positionally: the
		// altered subject is Targets[0], the destination Targets[1].
		if len(targets) != 2 {
			s.invalidateAll(targets)
			return nil
		}
		return s.applyRenamePair(ctx, statement, targets[0], targets[1])
	}
	if alter.Action == "modify_column" {
		return s.applyModifyColumn(ctx, statement, alter, targets)
	}
	if alter.Action == "change_column" || alter.Action == "rename_column" {
		return s.applyColumnIdentity(ctx, statement, alter, targets)
	}
	if alter.Action == "set_column_default" || alter.Action == "drop_column_default" {
		return s.applyColumnDefault(ctx, statement, alter, targets)
	}
	if alter.Action == "set_column_visibility" {
		return s.applyColumnVisibility(ctx, statement, alter, targets)
	}
	if (alter.Action == "add_index" || alter.Action == "add_constraint") && alter.Index != nil && alter.Index.Definition != nil {
		s.applyAddIndex(statement, alter, targets)
		return nil
	}
	if alter.Action == "add_constraint" && alter.Constraint != nil {
		if alter.Constraint.Type == "foreign_key" {
			return s.applyAddForeignKey(ctx, statement, alter, targets)
		}
		if alter.Constraint.Type == "check" {
			s.applyAddCheck(statement, alter, targets)
			return nil
		}
		// Other non-foreign-key constraint adds are unmodeled
		// post-state transitions — tombstone rather than guess the member.
		s.invalidateAll(targets)
		return nil
	}
	if alter.Action == "drop_foreign_key" {
		s.applyDropForeignKey(statement, alter, targets)
		return nil
	}
	if alter.Action == "drop_index" {
		s.applyDropIndex(statement, alter, targets)
		return nil
	}
	if alter.Action == "rename_index" {
		s.applyRenameIndex(statement, alter, targets)
		return nil
	}
	if alter.Action == "alter_index" {
		s.applyAlterIndexVisibility(statement, alter, targets)
		return nil
	}
	if alter.Action == "drop_primary_key" {
		s.applyDropPrimaryKey(statement, targets)
		return nil
	}
	if alter.Action == "algorithm" || alter.Action == "lock" {
		s.applyModifierOnlyAlter(statement, targets)
		return nil
	}
	if alter.Action == "table_option" {
		s.applyTableOptionAlter(alter, targets)
		return nil
	}
	switch alter.Action {
	case "order_by_columns", "force", "enable_keys", "disable_keys":
		s.applyMaintenanceAlter(alter, targets)
		return nil
	}
	if isPartitionAlterAction(alter.Action) {
		s.applyPartitionOperation(targets, alter)
		return nil
	}
	if alter.Action != "add_columns" || alter.Column == nil || alter.Column.Definition == nil {
		s.invalidateAll(targets)
		return nil
	}
	key := s.keyFor(s.schema, targets[0])
	entry := s.entries[key]
	switch {
	case entry == nil || entry.state == tableUnknown:
		// A successful ADD COLUMN proves the table exists; the pre-existing
		// shape is unknown, so only existence is definite afterwards.
		s.entries[key] = presentIncompleteEntry(s.effectiveSchema(s.schema, targets[0]), targets[0].Name)
	case entry.state == tableAbsent:
		// The premise (table exists) is already known-broken.
		s.invalidateKey(key)
	case entry.state == tablePresent && entry.shape != nil && entry.shape.Columns != nil:
		def := alter.Column.Definition
		if entry.shape.FindColumn(def.Name) != nil {
			if alter.Options["if_not_exists"] == "true" {
				// IF NOT EXISTS on an existing column is a no-op.
				return nil
			}
			// Duplicate plain ADD can never succeed — do not fabricate the
			// added column, invalidate the table instead.
			s.invalidateKey(key)
			return nil
		}
		// The column and its inline CHECK members are ONE statement
		// result: a proven name collision or missing referenced column
		// refuses everything (3822/1054) — nothing lands; an undecidable
		// arm or name space leaves the column's own fate unprovable.
		switch s.inColumnChecksOutcome(entry.shape, def.Name, alter.Constraints) {
		case inColumnCheckRefused:
			return nil
		case inColumnCheckUndecidable:
			s.invalidateKey(key)
			return nil
		}
		shape := cloneTableSnapshot(entry.shape)
		next := cloneColumn(*def)
		switch {
		case alter.Position == nil:
			shape.Columns = append(shape.Columns, next)
		case alter.Position.Kind == "first":
			shape.Columns = insertColumnAt(shape.Columns, next, 0)
		case alter.Position.Kind == "after":
			idx, ok := positionResolveIndex(shape.Columns, alter.Position, def.Name)
			if !ok || idx < 0 {
				// AFTER a missing column is a failed premise — never publish
				// a guessed position.
				s.invalidateKey(key)
				return nil
			}
			shape.Columns = insertColumnAt(shape.Columns, next, idx+1)
		default:
			s.invalidateKey(key)
			return nil
		}
		s.entries[key] = &batchTableEntry{state: tablePresent, shape: shape, displaySchema: entry.displaySchema, displayTable: entry.displayTable}
		// CHECK members declared inside the added column's definition land
		// with it — or unvouch the collection when the arm cannot decide.
		s.applyInColumnChecks(key, alter.Constraints)
	}
	// present-incomplete stays incomplete: the conditional add proves the
	// column exists, but the shape model cannot express a partial column set.
	return nil
}

// statementVersion returns the ordered state's already-resolved version
// fact — the same identity rules read through statement.Metadata.
func (s *batchState) statementVersion() *spec.VersionIdentity {
	return s.resolvedVersion
}

// applyModifierOnlyAlter publishes the successor of a single
// ALGORITHM=/LOCK= spec: the modifier owns no state transition. A proven
// refusal keeps the exact pre-state; a combinable declaration only proves
// the table exists; an unproven cell (fact-missing or unprobed) carries
// the bounded tombstone.
func (s *batchState) applyModifierOnlyAlter(statement spec.Statement, targets []spec.Table) {
	key := s.keyFor(s.schema, targets[0])
	entry := s.entries[key]
	var engine string
	if entry != nil && entry.state == tablePresent && entry.shape != nil {
		engine = entry.shape.Options["engine"]
	} else if statement.Metadata != nil && statement.Metadata.TargetTable != nil {
		engine = statement.Metadata.TargetTable.Options["engine"]
	}
	outcome := spec.OnlineDDLOutcomeFor(s.dialect, s.statementVersion(), engine, statement.DDL.Alter)
	switch outcome.Verdict {
	case spec.OnlineDDLRefused:
		return
	case spec.OnlineDDLUnproven:
		s.invalidateAll(targets)
		return
	}
	s.publishExecutedAlter(key, entry, targets)
}

// applyMaintenanceAlter publishes the successor of ORDER BY, FORCE, or
// ENABLE/DISABLE KEYS — accepted forms keep the modeled table shape; the
// TiDB FORCE refusal keeps the exact pre-state.
func (s *batchState) applyMaintenanceAlter(alter *spec.Alter, targets []spec.Table) {
	if s.dialect == spec.DialectTiDB && alter.Action == "force" {
		// Probed 8200 — the statement refuses atomically.
		return
	}
	key := s.keyFor(s.schema, targets[0])
	s.publishExecutedAlter(key, s.entries[key], targets)
}

// publishExecutedAlter publishes the existence-only successor every
// accepted no-op-on-model action shares: a proven-present table keeps its
// shape, an unknown table becomes present-incomplete, and a known-absent
// table tombstones — its premise is already broken.
func (s *batchState) publishExecutedAlter(key batchTableKey, entry *batchTableEntry, targets []spec.Table) {
	switch {
	case entry == nil || entry.state == tableUnknown:
		s.entries[key] = presentIncompleteEntry(s.effectiveSchema(s.schema, targets[0]), targets[0].Name)
	case entry.state == tableAbsent:
		s.invalidateKey(key)
	}
	// tablePresent keeps the shape untouched — the action changed nothing
	// the model tracks.
}

// applyTableOptionAlter publishes the successor of a table-option spec:
// proven vendor refusals keep the exact pre-state, catalog-mapped options
// publish their declared effect, and runtime-dependent options leave the
// modeled facts unchanged (they never touch what the model vouches).
func (s *batchState) applyTableOptionAlter(alter *spec.Alter, targets []spec.Table) {
	key := s.keyFor(s.schema, targets[0])
	entry := s.entries[key]
	for name := range alter.Options {
		if spec.AlterTableOptionOutcome(s.dialect, name) == spec.TableOptionVendorRefused {
			return
		}
	}
	if s.dialect == spec.DialectTiDB {
		if charset := alter.Options["convert_to_charset"]; charset != "" {
			// TiDB's own conversion arm: a refused target keeps the exact
			// pre-state (8200); an unprobed target tombstones — the option
			// name is consumed but this target pair is not.
			switch spec.TiDBCharsetConversionOutcome(charset) {
			case spec.OnlineDDLRefused:
				return
			case spec.OnlineDDLUnproven:
				s.invalidateAll(targets)
				return
			}
		}
	}
	switch {
	case entry == nil || entry.state == tableUnknown:
		s.entries[key] = presentIncompleteEntry(s.effectiveSchema(s.schema, targets[0]), targets[0].Name)
	case entry.state == tableAbsent:
		s.invalidateKey(key)
	case entry.state == tablePresent && entry.shape != nil:
		shape := cloneTableSnapshot(entry.shape)
		applyTableOptionEffects(s.dialect, alter, shape)
		s.entries[key] = &batchTableEntry{state: tablePresent, shape: shape, displaySchema: entry.displaySchema, displayTable: entry.displayTable}
	}
}

// applyTableOptionEffects applies the catalog-mapped option effects of a
// table-option spec onto a working shape — the same function the
// multi-action successor calls after structural actions. Only options
// with a probed catalog effect write; the rest are declared-but-inert.
func applyTableOptionEffects(dialect spec.Dialect, alter *spec.Alter, shape *spec.TableSnapshot) {
	if shape.Options == nil {
		shape.Options = map[string]string{}
	}
	if dialect == spec.DialectMySQL {
		if value, ok := alter.Options["engine"]; ok && value != "" {
			shape.Options["engine"] = value
		}
		if value, ok := alter.Options["row_format"]; ok && value != "" {
			shape.Options["row_format"] = value
		}
		if value, ok := alter.Options["charset"]; ok && value != "" {
			// A bare DEFAULT CHARACTER SET re-derives the collation from
			// the version-dependent charset default — the catalog value is
			// never the declared text, so the collation fact is unvouched.
			shape.Options["charset"] = value
			delete(shape.Options, "collation")
		}
		if value, ok := alter.Options["collate"]; ok && value != "" {
			shape.Options["collation"] = value
			if charset := charsetOfCollation(value); charset != "" {
				shape.Options["charset"] = charset
			}
		}
		if value, ok := alter.Options["auto_increment"]; ok && value != "" {
			applyAutoIncrementSeed(shape, value)
		}
	}
	// CONVERT TO lands on both dialects: MySQL rewrites every string
	// column; TiDB accepts only the utf8mb4 family, which is the same
	// publication (refused targets never reach here — the vendor-refusal
	// gate keeps the pre-state).
	if charset, ok := alter.Options["convert_to_charset"]; ok && charset != "" {
		collate := alter.Options["convert_to_collate"]
		shape.Options["charset"] = charset
		if collate != "" {
			shape.Options["collation"] = collate
		} else {
			delete(shape.Options, "collation")
		}
		for i := range shape.Columns {
			if columnTypeFamilyLocal(shape.Columns[i]) != "string" {
				continue
			}
			shape.Columns[i].Charset = charset
			shape.Columns[i].Collation = collate
		}
	}
	// COMMENT lands on both dialects (probed TiDB accepts and applies).
	if value, ok := alter.Options["comment"]; ok {
		if shape.Table == nil {
			shape.Table = &spec.Table{}
		}
		shape.Table.Comment = value
	}
}

// charsetOfCollation derives the charset a collation name binds — the
// catalog naming convention is <charset>_<rest>, deterministic by
// construction.
func charsetOfCollation(collation string) string {
	idx := strings.IndexByte(collation, '_')
	if idx <= 0 {
		return ""
	}
	return collation[:idx]
}

// applyAutoIncrementSeed publishes a declared AUTO_INCREMENT only when it
// raises the observed seed — the server clamps a lower declaration to the
// current seed, never decreasing it. A missing seed fact publishes the
// declared value only when the table actually carries an auto-increment
// column; otherwise the declaration is a no-op.
func applyAutoIncrementSeed(shape *spec.TableSnapshot, declared string) {
	hasAutoColumn := false
	for _, column := range shape.Columns {
		if column.AutoIncrement {
			hasAutoColumn = true
			break
		}
	}
	if !hasAutoColumn {
		return
	}
	declaredValue, err := strconv.ParseInt(strings.TrimSpace(declared), 10, 64)
	if err != nil {
		return
	}
	current, err := strconv.ParseInt(strings.TrimSpace(shape.Options["auto_increment"]), 10, 64)
	if err == nil && declaredValue <= current {
		return
	}
	shape.Options["auto_increment"] = strconv.FormatInt(declaredValue, 10)
}

// columnTypeFamilyLocal maps a column's declared type onto the charset-
// carrying family CONVERT TO rewrites.
func columnTypeFamilyLocal(column spec.Column) string {
	t := strings.ToLower(strings.TrimSpace(column.Type))
	switch {
	case strings.HasPrefix(t, "char"), strings.HasPrefix(t, "varchar"),
		strings.HasPrefix(t, "text"), strings.HasPrefix(t, "tinytext"),
		strings.HasPrefix(t, "mediumtext"), strings.HasPrefix(t, "longtext"),
		strings.HasPrefix(t, "enum"), strings.HasPrefix(t, "set"):
		return "string"
	default:
		return t
	}
}

// applyRenameTable applies RENAME TABLE's per-pair sequential outcome.
// Every alter payload carries one pair's old/new identities; pairs execute
// in authored order, so a swap (`a→b, b→c`) reads the post-pair state of
// the previous pair — the same sequential binding the engine uses. On
// MySQL the statement reads the persistent catalog only (a temp-only name
// answers 1146); TiDB resolves temp-first and refuses any local-temp
// endpoint with 8200 — a proven refusal that keeps the pair's pre-state.
// Each pair's endpoint table: source proven-present + destination
// proven-absent + movable members migrates; a proven refusal (missing
// source, occupied destination) keeps the pair's exact pre-state; every
// undecidable cell tombstones only that pair's endpoints.
func (s *batchState) applyRenameTable(ctx context.Context, statement spec.Statement) error {
	ddl := statement.DDL
	targets := ddl.TableTargets()
	if !fullyAuditedStatement(s.dialect, statement) || len(ddl.Alter) == 0 || len(targets) != 2*len(ddl.Alter) {
		s.invalidateBound(ctx, statement, targets)
		return nil
	}
	for i := range ddl.Alter {
		alter := ddl.Alter[i]
		source, destination := targets[2*i], targets[2*i+1]
		if alter.Action != "rename_table" ||
			alter.Options["old_table"] != source.Name || alter.Options["old_schema"] != source.Schema ||
			alter.Options["new_table"] != destination.Name || alter.Options["new_schema"] != destination.Schema {
			s.invalidateBound(ctx, statement, targets)
			return nil
		}
	}
	// Multi-pair RENAME TABLE is atomic on both products but adjudicates
	// pairs left-to-right against an evolving working set: each consumed
	// source frees its slot, each produced destination occupies it — the
	// scratch triple (a→tmp, b→a, tmp→b) is the canonical proof. A proven
	// refusal anywhere rolls the whole statement back (probed: a first
	// migrated pair is restored when a later pair fails), and an
	// undecidable cell tombstones every declared endpoint because pairs
	// after it may never have been reached natively.
	type boundPair struct {
		source, destination spec.Table
		srcNS, dstNS        boundNamespace
		srcEntry, dstEntry  *batchTableEntry
		srcKey, dstKey      batchTableKey
	}
	pairs := make([]boundPair, 0, len(ddl.Alter))
	for i := range ddl.Alter {
		source, destination := targets[2*i], targets[2*i+1]
		srcNS, srcEntry, srcKey, err := s.bindRenameEndpoint(ctx, statement, source, true, namespacePersistent)
		if err != nil {
			return fmt.Errorf("load table snapshot for %s: %w", source.Name, err)
		}
		dstNS, dstEntry, dstKey, err := s.bindRenameEndpoint(ctx, statement, destination, false, srcNS)
		if err != nil {
			return fmt.Errorf("load table snapshot for %s: %w", destination.Name, err)
		}
		pairs = append(pairs, boundPair{source: source, destination: destination, srcNS: srcNS, dstNS: dstNS, srcEntry: srcEntry, dstEntry: dstEntry, srcKey: srcKey, dstKey: dstKey})
	}
	// The shared work-set adjudication answers each endpoint's class at
	// its own pair position — produced and consumed dispositions are
	// last-write-wins, so a name produced then consumed is absent at
	// later pairs and a name consumed then produced is present again.
	classKey := func(key batchTableKey, ns boundNamespace) string {
		return key.schema + "\x00" + key.table + "\x00" + strconv.Itoa(int(ns))
	}
	srcKeys := make([]string, len(pairs))
	dstKeys := make([]string, len(pairs))
	srcBase := make([]spec.RenameEndpointClass, len(pairs))
	dstBase := make([]spec.RenameEndpointClass, len(pairs))
	for i, pair := range pairs {
		srcKeys[i] = classKey(pair.srcKey, pair.srcNS)
		dstKeys[i] = classKey(pair.dstKey, pair.dstNS)
		srcBase[i] = renameEndpointClass(boundEntryState(pair.srcNS, pair.srcEntry))
		dstBase[i] = renameEndpointClass(boundEntryState(pair.dstNS, pair.dstEntry))
	}
	pairVerdicts := spec.RenamePairClasses(srcKeys, dstKeys, srcBase, dstBase)
	fate := renamePairMigrate
	producedShape := make(map[batchTableKey]map[boundNamespace]*spec.TableSnapshot, len(pairs))
	pairShapes := make([]*spec.TableSnapshot, len(pairs))
	for i, verdict := range pairVerdicts {
		if err := ctx.Err(); err != nil {
			return err
		}
		pair := &pairs[i]
		// The first refused or undecidable cell ends adjudication — later
		// pairs may never have been reached natively. The layered checks
		// precede the existence matrix: a self-pair is a proven refusal
		// that keeps, TiDB refuses local-temp endpoints outright (8200),
		// and an ambiguous binding leaves the pair undecidable.
		switch {
		case pair.srcKey == pair.dstKey && pair.srcNS == pair.dstNS:
			fate = renamePairKept
		case s.dialect == spec.DialectTiDB && (pair.srcNS == namespaceTemporary || pair.dstNS == namespaceTemporary):
			fate = renamePairKept
		case pair.srcNS == namespaceAmbiguous || pair.dstNS == namespaceAmbiguous:
			fate = renamePairUnproven
		case verdict.Dst == spec.RenameEndpointPresent:
			fate = renamePairKept
		case verdict.Src == spec.RenameEndpointAbsent:
			fate = renamePairKept
		case verdict.Src != spec.RenameEndpointPresent || verdict.Dst != spec.RenameEndpointAbsent:
			fate = renamePairUnproven
		case !verdict.SrcProduced && (pair.srcEntry == nil || !renameShapeMovable(pair.srcEntry.shape)):
			// The existence cell proved but the member set cannot move —
			// a provable migration never ran, and a guessed shape is
			// worse than none. A produced source inherits movability
			// from its producing pair's already-adjudicated shape.
			fate = renamePairUnproven
		}
		if fate != renamePairMigrate {
			break
		}
		// The produced slot carries the pair's effective source shape —
		// chained moves (a→x, x→b) forward it so b lands a's body. The
		// pair's own shape is captured per-position: a later pair may
		// overwrite the same produced slot (d→b after a→b), and the apply
		// phase must see the shape this pair actually carried.
		var effective *spec.TableSnapshot
		if verdict.SrcProduced {
			effective = producedShape[pair.srcKey][pair.srcNS]
		} else {
			effective = pair.srcEntry.shape
		}
		pairShapes[i] = effective
		if producedShape[pair.dstKey] == nil {
			producedShape[pair.dstKey] = map[boundNamespace]*spec.TableSnapshot{}
		}
		producedShape[pair.dstKey][pair.dstNS] = effective
	}
	if fate == renamePairMigrate {
		for i, pair := range pairs {
			dstMap := s.entries
			if pair.dstNS == namespaceTemporary {
				dstMap = s.tempEntries
			}
			srcShape := pairShapes[i]
			dstMap[pair.dstKey] = &batchTableEntry{
				state:         tablePresent,
				shape:         renamedShape(srcShape, s.effectiveSchema(s.schema, pair.destination), pair.destination.Name),
				displaySchema: s.effectiveSchema(s.schema, pair.destination),
				displayTable:  pair.destination.Name,
			}
		}
		for i := range pairs {
			pair := &pairs[i]
			// The consumed slot stays vacated only when no later pair
			// re-occupies the identity — chained moves must clear a
			// produced-then-consumed slot, while a real rotation keeps
			// the re-occupied landing.
			reoccupied := false
			for j := i + 1; j < len(pairs); j++ {
				if pairs[j].dstKey == pair.srcKey && pairs[j].dstNS == pair.srcNS {
					reoccupied = true
					break
				}
			}
			if reoccupied {
				continue
			}
			srcMap := s.entries
			if pair.srcNS == namespaceTemporary {
				srcMap = s.tempEntries
			}
			srcMap[pair.srcKey] = &batchTableEntry{
				state:         tableAbsent,
				displaySchema: s.effectiveSchema(s.schema, pair.source),
				displayTable:  pair.source.Name,
			}
		}
		endpointKeys := make(map[batchTableKey]struct{}, len(pairs)*2)
		for _, pair := range pairs {
			endpointKeys[pair.srcKey] = struct{}{}
			endpointKeys[pair.dstKey] = struct{}{}
		}
		for _, dependent := range s.referencingEntryKeys(endpointKeys) {
			s.invalidateKey(dependent)
		}
		return nil
	}
	if fate == renamePairUnproven {
		// An atomic statement's undecidable fate belongs to every declared
		// endpoint — pairs after the failing one may never have been
		// reached, and the failing pair may have executed — so every named
		// identity tombstones in both slots, plus their loaded dependents.
		keys := make(map[batchTableKey]struct{}, len(targets))
		for _, target := range targets {
			if strings.TrimSpace(target.Name) != "" {
				keys[s.keyFor(s.schema, target)] = struct{}{}
			}
		}
		for _, dependent := range s.referencingEntryKeys(keys) {
			s.invalidateKey(dependent)
		}
		for key := range keys {
			s.invalidateKey(key)
			s.invalidateTempKey(key)
		}
	}
	return nil
}

// renameEndpointClass maps a bound entry's knowledge to the shared
// rename-adjudication class — never-touched and unknown both count as
// unvouched for pair purposes.
func renameEndpointClass(state tableKnowledge) spec.RenameEndpointClass {
	switch state {
	case tablePresent:
		return spec.RenameEndpointPresent
	case tableAbsent:
		return spec.RenameEndpointAbsent
	default:
		return spec.RenameEndpointUnknown
	}
}

// backupSlot records a slot's pre-mutation entry once so an atomic veto
// can restore it exactly — entries are shared pointers, so a slot that
// was never written keeps its original record.
func backupSlot(originals map[batchTableKey]map[boundNamespace]*batchTableEntry, entries, tempEntries map[batchTableKey]*batchTableEntry, key batchTableKey, ns boundNamespace) {
	if originals[key] == nil {
		originals[key] = map[boundNamespace]*batchTableEntry{}
	}
	if _, done := originals[key][ns]; done {
		return
	}
	if ns == namespaceTemporary {
		originals[key][ns] = tempEntries[key]
		return
	}
	originals[key][ns] = entries[key]
}

func renamedShape(source *spec.TableSnapshot, schema, table string) *spec.TableSnapshot {
	if source == nil {
		return nil
	}
	shape := cloneTableSnapshot(source)
	shape.Schema = schema
	shape.Table = &spec.Table{Schema: schema, Name: table}
	return shape
}

// bindRenameEndpoint resolves one endpoint of a rename pair. The source
// endpoint binds by form: ALTER TABLE ... RENAME reads temp-first on both
// products (MySQL migrates the session object, TiDB then refuses 8200);
// standalone RENAME TABLE reads the persistent catalog only on MySQL (a
// temp-only name answers 1146) while TiDB resolves temp-first to refuse.
// The destination always binds the namespace the source resolved into —
// a temp object renames inside the session namespace, and a catalog
// rename writes the catalog without consulting a same-named temp shadow
// (probed: RENAME x TO temp-name lands under the shadow on both
// products).
func (s *batchState) bindRenameEndpoint(ctx context.Context, statement spec.Statement, table spec.Table, source bool, sourceNS boundNamespace) (boundNamespace, *batchTableEntry, batchTableKey, error) {
	key := s.keyFor(s.schema, table)
	if !source {
		if sourceNS == namespaceTemporary {
			return namespaceTemporary, s.tempEntry(key), key, nil
		}
		entry, err := s.resolve(ctx, key, s.effectiveSchema(s.schema, table), table.Name)
		return namespacePersistent, entry, key, err
	}
	if statement.DDL != nil && statement.DDL.Operation == spec.DDLOperationRenameTable && s.dialect == spec.DialectMySQL {
		entry, err := s.resolve(ctx, key, s.effectiveSchema(s.schema, table), table.Name)
		return namespacePersistent, entry, key, err
	}
	if temp := s.tempEntry(key); temp != nil {
		switch temp.state {
		case tablePresent:
			return namespaceTemporary, temp, key, nil
		case tableUnknown:
			return namespaceAmbiguous, nil, key, nil
		}
	}
	entry, err := s.resolve(ctx, key, s.effectiveSchema(s.schema, table), table.Name)
	return namespacePersistent, entry, key, err
}

// applyRenamePair adjudicates one bound rename pair (the ALTER TABLE ...
// RENAME form, which is a single atomic pair) through the shared
// transition. A kept fate leaves every slot untouched; an unproven fate
// tombstones the bound endpoints plus dependents; a migrate fate moves
// the deep-copied shape under the destination identity.
func (s *batchState) applyRenamePair(ctx context.Context, statement spec.Statement, source, destination spec.Table) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	srcNS, srcEntry, srcKey, err := s.bindRenameEndpoint(ctx, statement, source, true, namespacePersistent)
	if err != nil {
		return fmt.Errorf("load table snapshot for %s: %w", source.Name, err)
	}
	dstNS, dstEntry, dstKey, err := s.bindRenameEndpoint(ctx, statement, destination, false, srcNS)
	if err != nil {
		return fmt.Errorf("load table snapshot for %s: %w", destination.Name, err)
	}
	originals := make(map[batchTableKey]map[boundNamespace]*batchTableEntry)
	fate, err := s.renamePairTransition(ctx, srcNS, srcEntry, srcKey, dstNS, dstEntry, dstKey, source, destination, originals)
	if err != nil {
		return err
	}
	switch fate {
	case renamePairMigrate:
		if err := ctx.Err(); err != nil {
			return err
		}
		dstMap := s.entries
		if dstNS == namespaceTemporary {
			dstMap = s.tempEntries
		}
		dstMap[dstKey] = &batchTableEntry{
			state:         tablePresent,
			shape:         renamedShape(srcEntry.shape, s.effectiveSchema(s.schema, destination), destination.Name),
			displaySchema: s.effectiveSchema(s.schema, destination),
			displayTable:  destination.Name,
		}
		srcMap := s.entries
		if srcNS == namespaceTemporary {
			srcMap = s.tempEntries
		}
		srcMap[srcKey] = &batchTableEntry{
			state:         tableAbsent,
			displaySchema: s.effectiveSchema(s.schema, source),
			displayTable:  source.Name,
		}
	case renamePairUnproven:
		// The undecidable pair tombstones its bound endpoints and every
		// loaded dependent of those identities — an endpoint whose state
		// cannot be vouched cannot keep a foreign-key premise alive.
		for _, dependent := range s.referencingEntryKeys(map[batchTableKey]struct{}{srcKey: {}, dstKey: {}}) {
			s.invalidateKey(dependent)
		}
		s.invalidateBoundKey(srcKey, srcNS)
		s.invalidateBoundKey(dstKey, dstNS)
	}
	return nil
}

// renamePairTransition adjudicates the endpoint-existence cell of one
// bound pair — the single seam both the ALTER single-pair path and the
// atomic multi-pair simulation consume. Proven refusals (self-pair,
// TiDB's 8200 local-temp refusal, absent source 1146, occupied
// destination 1050) answer renamePairKept; a provable cell with movable
// members answers renamePairMigrate and invalidates dependent entries
// (backing them up when originals is non-nil); every undecidable cell —
// ambiguous binding, unvouched endpoint, or immovable members — answers
// renamePairUnproven and leaves endpoint tombstoning to the caller.
func (s *batchState) renamePairTransition(ctx context.Context, srcNS boundNamespace, srcEntry *batchTableEntry, srcKey batchTableKey, dstNS boundNamespace, dstEntry *batchTableEntry, dstKey batchTableKey, source, destination spec.Table, originals map[batchTableKey]map[boundNamespace]*batchTableEntry) (renamePairOutcome, error) {
	if srcKey == dstKey && srcNS == dstNS {
		// RENAME t TO t is a proven no-op natively — the identity and
		// shape stand exactly as bound.
		return renamePairKept, nil
	}
	// TiDB refuses a rename touching a local-temporary endpoint (8200) —
	// a proven refusal, so both endpoints keep their pre-state.
	if s.dialect == spec.DialectTiDB && (srcNS == namespaceTemporary || dstNS == namespaceTemporary) {
		return renamePairKept, nil
	}
	if srcNS == namespaceAmbiguous || dstNS == namespaceAmbiguous {
		return renamePairUnproven, nil
	}
	fate := renamePairFate(boundEntryState(srcNS, srcEntry), boundEntryState(dstNS, dstEntry))
	if fate == renamePairMigrate && (srcEntry == nil || !renameShapeMovable(srcEntry.shape)) {
		// The existence cell proved but the member set cannot move — a
		// provable migration never ran, and a guessed shape is worse than
		// none.
		return renamePairUnproven, nil
	}
	if fate != renamePairMigrate {
		return fate, nil
	}
	dependentKeys := s.renameDependentKeys(srcKey, dstKey)
	if err := ctx.Err(); err != nil {
		return renamePairUnproven, err
	}
	for _, key := range dependentKeys {
		if originals != nil {
			backupSlot(originals, s.entries, s.tempEntries, key, namespacePersistent)
		}
		s.invalidateKey(key)
	}
	return renamePairMigrate, nil
}

type renamePairOutcome int

const (
	renamePairKept     renamePairOutcome = iota // proven refusal → pre-state stands
	renamePairMigrate                           // proven cell → move the shape
	renamePairUnproven                          // undecidable → tombstone endpoints
)

// boundEntryState resolves an entry's knowledge for fate adjudication. A
// nil entry in the session namespace is a proven-empty temp slot — the
// batch is the session — so it answers absent, never unknown.
func boundEntryState(ns boundNamespace, entry *batchTableEntry) tableKnowledge {
	if entry == nil {
		if ns == namespaceTemporary {
			return tableAbsent
		}
		return tableUnknown
	}
	return entry.state
}

// renamePairFate adjudicates the endpoint-existence cell of one pair.
// Source proven-absent or destination proven-present is a proven refusal.
// Source present + destination absent is the single migratable cell —
// the caller still gates migration on a movable member set. Every other
// cell is undecidable.
func renamePairFate(srcState, dstState tableKnowledge) renamePairOutcome {
	if srcState == tableAbsent || dstState == tablePresent {
		return renamePairKept
	}
	if srcState == tablePresent && dstState == tableAbsent {
		return renamePairMigrate
	}
	return renamePairUnproven
}

func (s *batchState) invalidateBoundKey(key batchTableKey, ns boundNamespace) {
	switch ns {
	case namespaceTemporary:
		s.invalidateTempKey(key)
	default:
		s.invalidateKey(key)
	}
}

func (s *batchState) renameDependentKeys(sourceKey, destinationKey batchTableKey) []batchTableKey {
	return s.referencingEntryKeys(map[batchTableKey]struct{}{sourceKey: {}, destinationKey: {}})
}

func (s *batchState) referencingEntryKeys(targets map[batchTableKey]struct{}) []batchTableKey {
	var dependents []batchTableKey
	for key, entry := range s.entries {
		if _, isTarget := targets[key]; isTarget {
			continue
		}
		if entry == nil || entry.state != tablePresent || entry.shape == nil {
			continue
		}
		for _, constraint := range entry.shape.Constraints {
			if strings.TrimSpace(constraint.ReferencedTable) == "" {
				continue
			}
			referenced := spec.Table{Schema: constraint.ReferencedSchema, Name: constraint.ReferencedTable}
			referenceKey := s.keyFor(key.schema, referenced)
			if _, hit := targets[referenceKey]; hit {
				dependents = append(dependents, key)
				break
			}
		}
	}
	return dependents
}

// applyDropTable publishes the namespace-aware multi-target drop outcome.
// Binding decides which slot each name hits — explicit TEMPORARY reads the
// session namespace only, explicit GLOBAL and every other statement read
// temp-first so a shadowing temp object is the bound target. The outcome
// matrix is probed per anchor:
//
//   - MySQL 8.0/8.4 veto on missing names: any bound-absent target aborts
//     the whole statement (1051) before anything drops — every slot keeps
//     its exact pre-state. With all targets resolved, persistent names
//     drop while a temp-bound name inside a multi-target list fails 1146
//     mid-execution and survives; a single temp target drops cleanly.
//   - MySQL 5.7 executes left to right: every bound-present target drops
//     — temp-bound included — and missing names error without veto.
//   - TiDB drops every resolvable persistent target (missing names error
//     without veto); a temp-bound name inside a multi-target list
//     survives, while a single temp target drops.
//   - An undecidable MySQL version cannot choose between vetoed and
//     sequential outcomes — divergent cells tombstone instead of picking.
//
// IF EXISTS turns proven-absent names into no-ops; duplicate names in one
// list are a proven 1066 refusal that keeps every pre-state. Ambiguous
// bindings and unknown entries tombstone their slots — never a guessed
// absence, never a fabricated survival.
// invalidateDropIdentities tombstones every identity a drop statement names
// — the positional targets, the primary table, and every loaded dependent —
// across both namespaces, without resolving anything. It is the guard path
// for statements whose target set is malformed, unaudited, or partially
// extracted: the effect is unbounded, so no fact may survive it.
func (s *batchState) invalidateDropIdentities(ddl *spec.DDL) {
	keys := make(map[batchTableKey]struct{})
	for _, target := range ddl.TableTargets() {
		if strings.TrimSpace(target.Name) != "" {
			keys[s.keyFor(s.schema, target)] = struct{}{}
		}
	}
	if ddl.Table != nil && strings.TrimSpace(ddl.Table.Name) != "" {
		keys[s.keyFor(s.schema, *ddl.Table)] = struct{}{}
	}
	for _, dependent := range s.referencingEntryKeys(keys) {
		s.invalidateKey(dependent)
	}
	for key := range keys {
		s.invalidateKey(key)
		s.invalidateTempKey(key)
	}
}

func (s *batchState) applyDropTable(ctx context.Context, statement spec.Statement) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	ddl := statement.DDL
	targets := ddl.TableTargets()
	if len(targets) == 0 {
		s.contaminated = true
		return nil
	}
	if !fullyAuditedStatement(s.dialect, statement) || ddl.OmittedTargets != 0 ||
		ddl.Table == nil || s.keyFor(s.schema, targets[0]) != s.keyFor(s.schema, *ddl.Table) {
		s.invalidateDropIdentities(ddl)
		return nil
	}
	// Bind every target first — an ambiguous or undecidable binding never
	// publishes a partial fate.
	type boundDrop struct {
		target spec.Table
		ns     boundNamespace
		entry  *batchTableEntry
		key    batchTableKey
	}
	bound := make([]boundDrop, 0, len(targets))
	seen := make(map[batchTableKey]boundNamespace, len(targets))
	duplicate := false
	for _, target := range targets {
		if strings.TrimSpace(target.Name) == "" {
			s.contaminated = true
			return nil
		}
		ns, entry, key, err := s.bindTarget(ctx, statement, target)
		if err != nil {
			return fmt.Errorf("load table snapshot for %s: %w", target.Name, err)
		}
		if prior, ok := seen[key]; ok && prior == ns && s.dialect == spec.DialectMySQL {
			// DROP TABLE t, t is a proven parse-time refusal (1066) on
			// MySQL — the statement executes nothing and every pre-state
			// stands. TiDB instead executes sequentially: the first
			// occurrence drops and the second errors 1051 after it is
			// already gone, so it falls through to the per-target fates.
			duplicate = true
		}
		seen[key] = ns
		bound = append(bound, boundDrop{target: target, ns: ns, entry: entry, key: key})
	}
	if duplicate {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	ifExists := ddl.Options["if_exists"] == "true"
	multi := len(targets) > 1
	anyMissing := false
	anyUnvouched := false
	for _, ref := range bound {
		switch ref.ns {
		case namespacePersistent:
			switch {
			case ref.entry != nil && ref.entry.state == tableAbsent:
				anyMissing = true
			case ref.entry == nil || ref.entry.state == tableUnknown:
				// A member whose existence is unvouched could be the
				// missing name an 8.x veto keys on — its presence makes
				// every sibling's fate undecidable under the veto arm.
				anyUnvouched = true
			}
		case namespaceTemporary:
			if ref.entry == nil || ref.entry.state == tableAbsent || ref.entry.state == tableNeverTouched {
				// An explicit TEMPORARY drop binds the session slot; an
				// empty slot is a missing name for existence purposes.
				anyMissing = true
			} else if ref.entry.state == tableUnknown {
				anyUnvouched = true
			}
		}
	}
	multiDropAtomic := s.multiDropVetoes()
	dropped := make(map[batchTableKey]struct{})
	unproven := make(map[batchTableKey]struct{})
	for _, ref := range bound {
		fate := dropTargetFate(multiDropAtomic, multi, ifExists, anyMissing, anyUnvouched, ref.ns, ref.entry, s.dialect, s.resolvedVersion)
		switch fate {
		case dropFateDropped:
			switch ref.ns {
			case namespaceTemporary:
				s.tempEntries[ref.key] = &batchTableEntry{state: tableAbsent, displaySchema: s.effectiveSchema(s.schema, ref.target), displayTable: ref.target.Name}
			default:
				dropped[ref.key] = struct{}{}
				if ref.entry != nil && ref.entry.shape != nil {
					// The child's own outbound edges die with it — remove
					// the inbound facts its parents recorded.
					if err := s.dropInboundEdges(ctx, ref.key, ref.target, ref.entry.shape); err != nil {
						return err
					}
				}
				s.entries[ref.key] = &batchTableEntry{state: tableAbsent, displaySchema: s.effectiveSchema(s.schema, ref.target), displayTable: ref.target.Name}
			}
		case dropFateUnproven:
			unproven[ref.key] = struct{}{}
			switch ref.ns {
			case namespaceAmbiguous:
				s.invalidateKey(ref.key)
				s.invalidateTempKey(ref.key)
			case namespaceTemporary:
				s.invalidateTempKey(ref.key)
			default:
				s.invalidateKey(ref.key)
			}
		}
	}
	// A parent drop is not probed for the 3730 refusal class — the
	// statement may have executed, so children whose declared FK premise
	// named a dropped or undecidable table cannot be trusted and tombstone
	// with it. A child that was itself a drop target keeps its own fate.
	for _, dependents := range []map[batchTableKey]struct{}{dropped, unproven} {
		for _, dependent := range s.referencingEntryKeys(dependents) {
			if _, isTarget := dropped[dependent]; isTarget {
				continue
			}
			s.invalidateKey(dependent)
		}
	}
	return nil
}

type dropFate int

const (
	dropFateKept     dropFate = iota // provably unchanged
	dropFateDropped                  // provably dropped → settle absent
	dropFateUnproven                 // divergent or unvouched → tombstone
)

// multiDropVetoes reports whether the target product vetoes a multi-target
// DROP on any missing name before execution — the atomic arm observed on
// MySQL 8.0.46 and 8.4.10 (and TiDB's persistent targets are NOT vetoed:
// TiDB drops resolvable members and reports missing ones, per probe).
// Unknown version answers undecidable: the 5.7 sequential arm and the 8.x
// veto arm diverge, so callers treat it as unproven rather than guessing.
func (s *batchState) multiDropVetoes() *bool {
	if s.dialect == spec.DialectTiDB {
		veto := false
		return &veto
	}
	if s.dialect != spec.DialectMySQL {
		return nil
	}
	if s.resolvedVersion == nil || s.resolvedVersion.Major == 0 {
		return nil
	}
	veto := s.resolvedVersion.Major >= 8
	return &veto
}

// dropTargetFate adjudicates one bound target through the probed outcome
// matrix. missing names under IF EXISTS are proven no-ops; under the atomic
// veto the whole statement refuses; under sequential execution present
// targets drop and missing ones keep their (absent) pre-state. Temp-bound
// names in a multi-target list drop on 5.7, survive on 8.x/TiDB — divergent
// under an unknown version.
func dropTargetFate(atomic *bool, multi, ifExists, anyMissing, anyUnvouched bool, ns boundNamespace, entry *batchTableEntry, dialect spec.Dialect, version *spec.VersionIdentity) dropFate {
	state := tableNeverTouched
	if entry != nil {
		state = entry.state
	}
	// IF EXISTS turns missing names into proven no-ops on every anchor —
	// the veto and sequential arms below only engage for a bare DROP.
	vetoPossible := !ifExists && (anyMissing || anyUnvouched)
	switch ns {
	case namespaceAmbiguous:
		return dropFateUnproven
	case namespaceTemporary:
		switch state {
		case tableUnknown:
			return dropFateUnproven
		case tableAbsent, tableNeverTouched:
			// Missing temp name: no-op under IF EXISTS, refused otherwise —
			// either way the slot is provably unchanged.
			return dropFateKept
		}
		// Present temp object: explicit TEMPORARY drops it outright, a
		// single plain DROP drops it, and a multi-target list's fate is
		// version-bound.
		if !multi {
			return dropFateDropped
		}
		// No name in the list can trigger a veto — every resolvable member
		// drops on every anchor (probed: 8.x `DROP tp16,p16_a` removes both,
		// 5.7 drops unconditionally, TiDB's resolvable list drops the temp).
		if !vetoPossible {
			return dropFateDropped
		}
		if dialect == spec.DialectTiDB {
			// TiDB's temp slot is veto-shaped, not sequential: a proven-
			// missing sibling keeps it (probe: tp16x survives
			// p16_a+missing), while an unvouched sibling leaves the
			// veto undecidable — the name may have existed.
			if anyMissing {
				return dropFateKept
			}
			return dropFateUnproven
		}
		// MySQL: the 8.x veto arm survives a proven-missing sibling, may
		// or may not have fired against an unvouched one, and never fires
		// on 5.7's sequential arm; an unvouched version splits both arms.
		switch {
		case atomic == nil:
			return dropFateUnproven
		case *atomic && anyMissing:
			return dropFateKept
		case *atomic:
			return dropFateUnproven
		default:
			// 5.7 sequential — temp names drop in a multi list too.
			return dropFateDropped
		}
	default:
		switch state {
		case tableUnknown:
			return dropFateUnproven
		case tableAbsent:
			return dropFateKept
		}
		if !vetoPossible {
			return dropFateDropped
		}
		switch {
		case atomic == nil:
			// Version unvouched: the 5.7 sequential arm drops resolvable
			// members while the 8.x veto keeps everything — divergent.
			return dropFateUnproven
		case *atomic && anyMissing:
			// A proven-missing sibling vetoes the whole statement on 8.x.
			return dropFateKept
		case *atomic:
			// The veto keys on a missing name this batch cannot vouch for.
			return dropFateUnproven
		default:
			// Sequential execution (5.7, TiDB): resolvable members drop.
			return dropFateDropped
		}
	}
}

// renameShapeMovable reports whether a known-present shape carries only
// members this slice transports verbatim. A nil shape (present-incomplete)
// moves as-is; a supplied primary_key constraint repeats the PrimaryKey fact
// and also moves, while any other recorded constraint does not — MySQL can
// rebind generated FK/CHECK identities during RENAME, so invalidation wins.
// Unknown member flags are not constraints and always migrate.
func renameShapeMovable(shape *spec.TableSnapshot) bool {
	if shape == nil {
		return true
	}
	for _, constraint := range shape.Constraints {
		if constraint.Type != "primary_key" {
			return false
		}
	}
	return true
}

// applyCreateIndex appends a fully audited standalone CREATE INDEX to a
// known-complete shape. Conditional success on an unknown target proves the
// table exists (present-incomplete); on a confirmed-absent target or with a
// premise already broken (duplicate index name, missing referenced column)
// the entry invalidates instead of fabricating an index.
func (s *batchState) applyCreateIndex(statement spec.Statement) {
	ddl := statement.DDL
	targets := ddl.TableTargets()
	if len(targets) == 0 || ddl.Table == nil {
		return
	}
	for _, alter := range ddl.Alter {
		// An authored `primary` member name is natively refused (1280)
		// before the table is touched — the entry keeps whatever state
		// it held rather than publishing present-incomplete over a
		// statement that provably did nothing.
		if alter.Action == "create_index" && alter.Index != nil &&
			alter.Index.Definition != nil && authoredPrimaryMemberName(alter.Index.Definition) {
			return
		}
	}
	key := s.keyFor(s.schema, targets[0])
	entry := s.entries[key]
	if !fullyAuditedStatement(s.dialect, statement) || (entry != nil && entry.state == tableAbsent) {
		s.invalidateKey(key)
		return
	}
	if entry == nil || entry.state == tableUnknown {
		s.entries[key] = presentIncompleteEntry(s.effectiveSchema(s.schema, targets[0]), targets[0].Name)
		return
	}
	if entry.shape == nil || entry.shape.Columns == nil {
		return
	}
	// An unvouched member set can never vouch for the merged collection —
	// keep the entry rather than merge into a shape that cannot be read.
	// An unproven name space is different: the CREATE INDEX member itself
	// is authored, so it merges and preserves member truth; the surviving
	// IndexNamesUnknown flag keeps name-keyed readers honest, while the
	// collision check below still refuses a provable duplicate name.
	if entry.shape.IndexesUnknown {
		return
	}
	for _, alter := range ddl.Alter {
		if alter.Action != "create_index" || alter.Index == nil || alter.Index.Definition == nil {
			continue
		}
		def := alter.Index.Definition
		if entry.shape.FindProvenIndex(def.Name) != nil {
			// A member provably holds the authored name — the create
			// refuses 1061 outright (or IF NOT EXISTS no-ops); either
			// way the table is provably unchanged, so the entry keeps
			// its facts.
			return
		}
		if pkName, proven := spec.PrimaryKeyObjectName(entry.shape); proven &&
			strings.EqualFold(pkName, def.Name) {
			// The authored name collides with the PK object's actual
			// stored name — the create refuses 1061, provably unchanged.
			return
		}
		if _, nameProven := spec.PrimaryKeyObjectName(entry.shape); !nameProven &&
			(entry.shape.HasPrimaryKey() || entry.shape.PrimaryKeyUnknown) {
			// PK presence or its stored name is unproven — a hidden 1061
			// against a maybe-present PK object cannot be ruled out, so
			// membership stays unvouched.
			s.markIndexesUnknown(key)
			return
		}
		if entry.shape.HasUnprovenIndexName() {
			// A member whose real stored name is unproven may already
			// hold the authored name — the create either landed or
			// refused on the hidden 1061, so membership is unprovable;
			// unvouch the member set rather than append a member the
			// statement may never have created.
			s.markIndexesUnknown(key)
			return
		}
		for _, column := range def.Columns {
			if entry.shape.FindColumn(column) == nil {
				s.invalidateKey(key)
				return
			}
		}
	}
	shape := cloneTableSnapshot(entry.shape)
	for _, alter := range ddl.Alter {
		if alter.Action != "create_index" || alter.Index == nil || alter.Index.Definition == nil {
			continue
		}
		index := cloneIndex(*alter.Index.Definition)
		shape.Indexes = append(shape.Indexes, index)
	}
	s.entries[key] = &batchTableEntry{state: tablePresent, shape: shape, displaySchema: entry.displaySchema, displayTable: entry.displayTable}
}

// applyTruncateTable keeps a present table's structure but clears row and
// index statistics — TRUNCATE empties the table, so provider-reported
// table_rows and cardinality must not carry into the post-state. The bound
// namespace is temp-first (a shadowing temp object truncates natively on
// both dialects). A proven-absent target is a proven refusal (1146-class):
// the pre-state stands rather than fabricating existence or downgrading to
// unknown. An unvouched or ambiguous binding tombstones — never a guessed
// success.
func (s *batchState) applyTruncateTable(ctx context.Context, statement spec.Statement) error {
	ddl := statement.DDL
	targets := ddl.TableTargets()
	if !fullyAuditedStatement(s.dialect, statement) || len(targets) == 0 {
		s.invalidateBound(ctx, statement, targets)
		return nil
	}
	ns, entry, key, err := s.bindTarget(ctx, statement, targets[0])
	if err != nil {
		return fmt.Errorf("load table snapshot for %s: %w", targets[0].Name, err)
	}
	switch ns {
	case namespaceAmbiguous:
		s.invalidateKey(key)
		s.invalidateTempKey(key)
		return nil
	case namespaceTemporary:
		if entry == nil || entry.state != tablePresent {
			// Missing/unknown temp slot: proven refusal keeps the empty
			// fact; unknown stays unvouched.
			if entry != nil && entry.state == tableUnknown {
				s.invalidateTempKey(key)
			}
			return nil
		}
		if entry.shape == nil {
			return nil
		}
		shape := cloneTableSnapshot(entry.shape)
		clearMutationStats(shape)
		s.tempEntries[key] = &batchTableEntry{state: tablePresent, shape: shape, displaySchema: entry.displaySchema, displayTable: entry.displayTable}
		return nil
	default:
		if entry == nil || entry.state == tableUnknown {
			s.invalidateKey(key)
			return nil
		}
		if entry.state != tablePresent {
			// Proven refusal — the absent fact is the statement's answer.
			return nil
		}
		if entry.shape == nil {
			return nil
		}
		// A proven inbound foreign-key edge refuses TRUNCATE natively
		// (1701): the child's existence is the edge's own proof, so the
		// parent keeps its exact pre-state. An unvouched edge remainder
		// cannot refuse — it only gates what the inbound_fk rule claims.
		if len(entry.shape.ReferencedBy) > 0 {
			return nil
		}
		shape := cloneTableSnapshot(entry.shape)
		clearMutationStats(shape)
		s.entries[key] = &batchTableEntry{state: tablePresent, shape: shape, displaySchema: entry.displaySchema, displayTable: entry.displayTable}
		return nil
	}
}

// applyDMLMutation clears row and index statistics on mutation targets —
// INSERT/UPDATE/DELETE change row counts without changing structure, so a
// cached table_rows or cardinality must not be blindly reused afterwards.
func (s *batchState) applyDMLMutation(statement spec.Statement) {
	if statement.DML == nil {
		return
	}
	for _, target := range statement.DML.MutationTargetTables() {
		key := s.keyFor(s.schema, target)
		entry, ok := s.entries[key]
		if !ok || entry.state != tablePresent || entry.shape == nil {
			continue
		}
		shape := cloneTableSnapshot(entry.shape)
		clearMutationStats(shape)
		s.entries[key] = &batchTableEntry{state: tablePresent, shape: shape, displaySchema: entry.displaySchema, displayTable: entry.displayTable}
	}
}

// isPartitionAlterAction reports the partition ALTER actions whose ordered
// post-state the batch state models — T13 destructive/data-moving actions
// and the T14 topology/maintenance set.
func isPartitionAlterAction(action string) bool {
	return spec.IsPartitionAlterAction(action)
}

func hasPartitionAlterAction(ddl *spec.DDL) bool {
	for i := range ddl.Alter {
		if isPartitionAlterAction(ddl.Alter[i].Action) {
			return true
		}
	}
	return false
}

// partitionOutcome is the evidence-bounded verdict of a partition action
// sharing a multi-spec ALTER.
type partitionOutcome int

const (
	// partitionOutcomeRefused is a proven refusal — the statement can never
	// execute, so the exact pre-state stands.
	partitionOutcomeRefused partitionOutcome = iota
	// partitionOutcomeCombinable is a proven-executed form — the partition
	// action's modeled post-state applies.
	partitionOutcomeCombinable
	// partitionOutcomeUnproven covers combinations no anchor probed — the
	// bound identities invalidate rather than claiming refusal or success.
	partitionOutcomeUnproven
)

// partitionMixedOutcome classifies a multi-spec ALTER that carries at
// least one partition action against the probed anchor matrix:
//   - more than one partition action, any structural sibling, or any
//     table-option sibling (ENGINE= et al.) → refused (MySQL 1064
//     grammar-level, TiDB 8200/1064 multi-schema — probed p6/p8).
//   - placement-policy siblings and multiple modifiers are unprobed →
//     unproven.
//   - one supported ALGORITHM/LOCK modifier → the probed value+version
//     gate decides refused/combinable/unproven.
//
// The shared grammar never accepts a spec after a partition clause, so
// partition-first mixes cannot reach this point.
func (s *batchState) partitionMixedOutcome(ddl *spec.DDL) partitionOutcome {
	return partitionMixedOutcome(ddl, s.resolvedVersion)
}

func partitionMixedOutcome(ddl *spec.DDL, version *spec.VersionIdentity) partitionOutcome {
	partitionActions := 0
	partitionAction := ""
	modifierCount := 0
	for i := range ddl.Alter {
		action := ddl.Alter[i].Action
		switch {
		case isPartitionAlterAction(action):
			partitionActions++
			partitionAction = action
		case action == "algorithm" || action == "lock":
			modifierCount++
		case action == "table_option":
			// ENGINE= + partition action: refused on every anchor —
			// MySQL 1064 at the grammar layer (partition spec must be
			// the ALTER tail), TiDB 8200 multi-schema. Probed p6/p8.
			return partitionOutcomeRefused
		case action == "placement_policy":
			// Placement bindings are table-option writes no anchor
			// probed against a partition action.
			return partitionOutcomeUnproven
		default:
			return partitionOutcomeRefused
		}
	}
	if partitionActions != 1 {
		return partitionOutcomeRefused
	}
	if modifierCount != 1 {
		return partitionOutcomeUnproven
	}
	for i := range ddl.Alter {
		alter := &ddl.Alter[i]
		if isPartitionAlterAction(alter.Action) {
			continue
		}
		value := ""
		if alter.Options != nil {
			value = alter.Options[alter.Action]
		}
		if outcome := partitionModifierOutcome(alter.Action, value, partitionAction, version); outcome != partitionOutcomeCombinable {
			return outcome
		}
	}
	return partitionOutcomeCombinable
}

// partitionModifierOutcome applies the probed modifier matrix:
//   - LOCK=DEFAULT / ALGORITHM=DEFAULT combine on all anchors.
//   - PARTITION BY and REMOVE PARTITIONING refuse any authored modifier at
//     the grammar layer (1064 on every probed anchor).
//   - ALGORITHM=INPLACE combines with TRUNCATE/EXCHANGE on all anchors,
//     combines with DROP on MySQL >=8 and TiDB, refuses on MySQL 5.x
//     (1846), and stays unproven when no version fact resolved. Other
//     algorithms are unprobed for the T13 actions.
//   - T14 actions: MySQL 5.x refuses non-default modifiers on the probed
//     ops (1846, or 1064 for scheme-changing actions); MySQL >=8 accepts
//     INPLACE+{REORGANIZE/ADD/COALESCE/REBUILD} and LOCK=NONE+ADD and
//     refuses COPY/LOCK=NONE+REORGANIZE; TiDB accepts INPLACE+{ADD,
//     COALESCE} and LOCK=NONE+ADD, while modifiers before REORGANIZE/
//     maintenance actions are grammar-level or vendor refusals. Unprobed
//     cells stay unproven.
func partitionModifierOutcome(action, value, partitionAction string, version *spec.VersionIdentity) partitionOutcome {
	if action != "algorithm" && action != "lock" {
		return partitionOutcomeUnproven
	}
	if value == "default" {
		return partitionOutcomeCombinable
	}
	// Scheme-changing actions refuse any authored modifier at the grammar
	// layer on every anchor (1064 — probed p13 mix-inplace-remove/-pby).
	if partitionAction == "partition" || partitionAction == "remove_partitioning" {
		return partitionOutcomeRefused
	}
	if version == nil {
		return partitionOutcomeUnproven
	}
	switch version.Product {
	case spec.VersionProductTiDB:
		switch partitionAction {
		case "add_partitions", "coalesce_partitions":
			if (action == "algorithm" && value == "inplace") ||
				(action == "lock" && value == "none") {
				return partitionOutcomeCombinable
			}
			return partitionOutcomeUnproven
		case "drop_partition", "truncate_partition", "exchange_partition":
			if action == "algorithm" && value == "inplace" {
				return partitionOutcomeCombinable
			}
			return partitionOutcomeUnproven
		default:
			// REORGANIZE under a modifier is a grammar refusal (1064);
			// the maintenance actions are vendor refusals (8200).
			return partitionOutcomeRefused
		}
	case spec.VersionProductMySQL:
		if version.Major < 8 {
			switch partitionAction {
			case "truncate_partition", "exchange_partition":
				if action == "algorithm" && value == "inplace" {
					return partitionOutcomeCombinable
				}
				return partitionOutcomeUnproven
			case "check_partitions", "optimize_partition", "repair_partition",
				"import_partition_tablespace", "discard_partition_tablespace":
				// Only REORGANIZE/ADD/COALESCE/REBUILD/DROP modifier
				// refusals were probed on 5.x.
				return partitionOutcomeUnproven
			case "drop_partition":
				if action == "lock" {
					// Non-default LOCK values stayed unprobed under T13.
					return partitionOutcomeUnproven
				}
				return partitionOutcomeRefused
			default:
				// REORGANIZE/ADD/COALESCE/REBUILD + non-default modifier
				// refuse with 1846 (probed p13).
				return partitionOutcomeRefused
			}
		}
		// MySQL >= 8 probed matrix.
		switch partitionAction {
		case "add_partitions":
			if (action == "algorithm" && value == "inplace") ||
				(action == "lock" && value == "none") {
				return partitionOutcomeCombinable
			}
			return partitionOutcomeUnproven
		case "reorganize_partition":
			switch {
			case action == "algorithm" && value == "inplace":
				return partitionOutcomeCombinable
			case action == "algorithm" && value == "copy":
				return partitionOutcomeRefused
			case action == "lock" && value == "none":
				return partitionOutcomeRefused
			}
			return partitionOutcomeUnproven
		case "coalesce_partitions", "rebuild_partition":
			if action == "algorithm" && value == "inplace" {
				return partitionOutcomeCombinable
			}
			return partitionOutcomeUnproven
		case "drop_partition", "truncate_partition", "exchange_partition":
			if action == "algorithm" && value == "inplace" {
				return partitionOutcomeCombinable
			}
			return partitionOutcomeUnproven
		default:
			// check/optimize/repair/import/discard under a modifier —
			// unprobed on MySQL >=8.
			return partitionOutcomeUnproven
		}
	}
	return partitionOutcomeUnproven
}

// applyPartitionOperation's dispatch and per-action transitions live in
// partition_transitions.go. The directory contract: proven refusals keep
// the exact pre-state, proven executions publish the modeled member set,
// and unproven outcomes unvouch the affected facts — never a guessed
// member list, never a blanket tombstone.

// partitionDropApplicability resolves DROP PARTITION against the observed
// partition method — the action gate the directory publication consumes.
// Only the range|list families accept DROP PARTITION natively (hash|key
// refuse with 1512 — probed p6; the same vocabulary
// partitionActionApplicableTypes carries on the rule side). An unobserved
// or unmodeled method leaves the outcome undecidable.
func partitionDropApplicability(method *string) partitionOutcome {
	if method == nil {
		return partitionOutcomeUnproven
	}
	switch *method {
	case "range", "range columns", "list", "list columns":
		return partitionOutcomeCombinable
	case "hash", "key", "linear hash", "linear key":
		return partitionOutcomeRefused
	}
	return partitionOutcomeUnproven
}

// dropPartitionMembers computes the post-DROP member set, reporting
// provable=false whenever the statement could not have executed: a named
// member absent from the proven directory (1507) or a drop consuming the
// whole member set (the engine never removes the last partition). The
// comparison is case-insensitive against the observed member names.
func dropPartitionMembers(members []string, alter *spec.Alter) ([]string, bool) {
	dropped := make(map[string]bool, len(alter.Partitions))
	for _, name := range alter.Partitions {
		dropped[strings.ToLower(name)] = true
	}
	present := make(map[string]bool, len(members))
	remaining := make([]string, 0, len(members))
	for _, name := range members {
		if dropped[strings.ToLower(name)] {
			present[strings.ToLower(name)] = true
			continue
		}
		remaining = append(remaining, name)
	}
	for name := range dropped {
		if !present[name] {
			return nil, false
		}
	}
	if len(remaining) == 0 {
		return nil, false
	}
	return remaining, true
}

// clearMutationStats removes the statistics a mutation can no longer vouch
// for: table row counts, AUTO_INCREMENT counters, and index cardinality.
func clearMutationStats(shape *spec.TableSnapshot) {
	if shape.Options != nil {
		delete(shape.Options, "table_rows")
		delete(shape.Options, "auto_increment")
	}
	if shape.PrimaryKey != nil {
		shape.PrimaryKey.Cardinality = nil
	}
	for i := range shape.Indexes {
		shape.Indexes[i].Cardinality = nil
	}
}

// failureBefore reports whether a parse failure at (line, column) precedes the
// statement's own start position — the boundary where unknown batch content
// may have changed facts no later statement can trust.
func failureBefore(failureLine, failureColumn, statementLine, statementColumn int) bool {
	if failureLine != statementLine {
		return failureLine < statementLine
	}
	return failureColumn <= statementColumn
}

// cloneTableSnapshot deep-copies a provider or derived snapshot so per-
// statement metadata can never alias mutable collections.
func cloneTableSnapshot(in *spec.TableSnapshot) *spec.TableSnapshot {
	if in == nil {
		return nil
	}
	out := *in
	if in.Table != nil {
		table := *in.Table
		out.Table = &table
	}
	if in.Columns != nil {
		out.Columns = make([]spec.Column, len(in.Columns))
		for i, column := range in.Columns {
			out.Columns[i] = cloneColumn(column)
		}
	}
	if in.PrimaryKey != nil {
		pk := cloneIndex(*in.PrimaryKey)
		out.PrimaryKey = &pk
	}
	if in.Indexes != nil {
		out.Indexes = make([]spec.Index, len(in.Indexes))
		for i, index := range in.Indexes {
			out.Indexes[i] = cloneIndex(index)
		}
	}
	if in.Constraints != nil {
		out.Constraints = make([]spec.Constraint, len(in.Constraints))
		for i, constraint := range in.Constraints {
			out.Constraints[i] = cloneConstraint(constraint)
		}
	}
	if in.Options != nil {
		out.Options = make(map[string]string, len(in.Options))
		for name, value := range in.Options {
			out.Options[name] = value
		}
	}
	if in.Partitioned != nil {
		partitioned := *in.Partitioned
		out.Partitioned = &partitioned
	}
	if in.PartitionType != nil {
		partitionType := *in.PartitionType
		out.PartitionType = &partitionType
	}
	if in.PartitionNames != nil {
		// append(nil, ...) would collapse an observed-empty set back to
		// unobserved — keep the non-nil shape through the clone.
		out.PartitionNames = append([]string{}, in.PartitionNames...)
	}
	if in.PartitionDescriptions != nil {
		out.PartitionDescriptions = append([]string{}, in.PartitionDescriptions...)
	}
	if in.PartitionDefs != nil {
		out.PartitionDefs = spec.ClonePartitionDefs(in.PartitionDefs)
	}
	if in.ReferencedBy != nil {
		out.ReferencedBy = append([]spec.Table{}, in.ReferencedBy...)
	}
	return &out
}

func cloneColumn(in spec.Column) spec.Column {
	out := in
	if in.IdentityOptions != nil {
		out.IdentityOptions = make(map[string]any, len(in.IdentityOptions))
		for key, value := range in.IdentityOptions {
			out.IdentityOptions[key] = value
		}
	}
	out.UnextractedOptions = append([]string(nil), in.UnextractedOptions...)
	out.Generated = spec.CloneGeneratedColumn(in.Generated)
	out.DefaultExpression = spec.CloneExpressionFacts(in.DefaultExpression)
	out.DefaultLiteral = spec.CloneLiteralDefaultFacts(in.DefaultLiteral)
	return out
}

func cloneIndex(in spec.Index) spec.Index {
	out := in
	// Post-state members claim the effective visibility fact, never the
	// authored VISIBLE|INVISIBLE attribute — the declaration form belongs
	// to the statement, not the published shape.
	out.VisibilityExplicit = false
	out.Columns = append([]string(nil), in.Columns...)
	out.KeyParts = spec.CloneKeyParts(in.KeyParts)
	out.IncludedColumns = append([]string(nil), in.IncludedColumns...)
	out.UnmodeledOptions = append([]string(nil), in.UnmodeledOptions...)
	if in.Cardinality != nil {
		cardinality := *in.Cardinality
		out.Cardinality = &cardinality
	}
	return out
}

func cloneConstraint(in spec.Constraint) spec.Constraint {
	out := in
	out.Columns = append([]string(nil), in.Columns...)
	out.ReferencedColumns = append([]string(nil), in.ReferencedColumns...)
	out.Expression = spec.CloneExpressionFacts(in.Expression)
	return out
}
