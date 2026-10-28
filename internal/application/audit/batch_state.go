// Package audit orchestrates audit use cases at the application layer.
package audit

import (
	"context"
	"fmt"
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
	provider     MetadataProvider
	dialect      spec.Dialect
	schema       string
	entries      map[batchTableKey]*batchTableEntry
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
	if snapshot.Exists {
		entry.state = tablePresent
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
			Schema:             displaySchema,
			Table:              &spec.Table{Schema: displaySchema, Name: displayTable},
			Exists:             true,
			PrimaryKeyUnknown:  true,
			IndexesUnknown:     true,
			ConstraintsUnknown: true,
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
			s.invalidateAll(targets)
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
		s.applyTruncateTable(statement)
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
			if _, refused := tidbVendorRefusedPartitionActions[fmt.Sprint(aspect.Metadata["action"])]; refused {
				continue
			}
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
			if _, refused := tidbVendorRefusedPartitionActions[fmt.Sprint(aspect.Metadata["action"])]; refused {
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
		s.entries[s.keyFor(s.schema, target)] = &batchTableEntry{state: tableUnknown}
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

// applyCreateTable derives a known-present shape from a fully audited CREATE
// TABLE. Conditional success semantics: a plain create on an unknown target
// derives the declared shape; on a known-absent target it deterministically
// creates it. CREATE TABLE IF NOT EXISTS on an unknown target cannot prove
// the declared columns landed, so it yields present-incomplete; on a
// known-present target it is a no-op that keeps the real fact. A plain create
// on a known-present target can never have succeeded — the premise is broken
// and the entry invalidates instead of inventing which shape survived.
func (s *batchState) applyCreateTable(ctx context.Context, statement spec.Statement) error {
	ddl := statement.DDL
	targets := ddl.TableTargets()
	if len(targets) == 0 || ddl.Table == nil {
		return nil
	}
	key := s.keyFor(s.schema, targets[0])
	if !fullyAuditedCreateTable(s.dialect, statement) {
		// A CREATE whose only unmodelled premise is partition-bound
		// comparability still resolves existence and conditional no-ops:
		// the declared-scheme gate inside this flow adjudicates the
		// scheme itself — an expression member stays unproven while a
		// proven-present IF NOT EXISTS target never executes the body
		// at all — so the bound aspect must not tombstone the entry.
		if !createPartitionBoundBounded(s.dialect, statement) {
			s.invalidateKey(key)
			return nil
		}
	}
	entry := s.entries[key]
	ifNotExists := ddl.Options["if_not_exists"] == "true"
	displaySchema, displayTable := s.effectiveSchema(s.schema, targets[0]), targets[0].Name
	// Declared foreign keys carry the same landing premise an ALTER-side
	// add does — local member/refer-action facts plus the referenced
	// parent's usable key — decided against the pre-state. A proven
	// refusal fails the statement before the table lands (the entry is
	// provably unchanged); an unprovable premise cannot publish the
	// declared shape because the statement may never have landed. A
	// present target under IF NOT EXISTS skips creation entirely, so the
	// premise never engages there.
	if entry == nil || entry.state == tableUnknown || entry.state == tableAbsent {
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
	// pre-state. A present target under IF NOT EXISTS never executes the
	// body, so the clause cannot refuse there.
	arm := s.checkArmFor()
	if entry == nil || entry.state != tablePresent || !ifNotExists {
		for _, member := range ddl.Constraints {
			if spec.CheckExplicitClauseRefused(s.dialect, arm, member) {
				return nil
			}
			if member.Type == "check" && member.EnforcedUnproven &&
				s.dialect == spec.DialectMySQL && arm == spec.CheckArmIgnored {
				// The authored clause could not be attributed: the
				// statement may have refused 1064 or landed the table —
				// existence is undecidable, so the entry is unvouched
				// rather than published.
				s.invalidateKey(key)
				return nil
			}
		}
	}
	// A provably-invalid declared partition scheme refuses the whole
	// statement natively — duplicate member names (1517), an over-bound
	// or method-mismatched PARTITIONS n (1499/1492-class), or bounds the
	// method cannot accept (1493/1495-class). The same judgment ALTER
	// PARTITION BY consumes gates the publish here, so a refused CREATE
	// never lands the table or a fabricated member directory; an
	// unproven scheme unvouches the target instead. A present target
	// under IF NOT EXISTS never executes the clause, so the premise
	// cannot engage there.
	if entry == nil || entry.state == tableUnknown || entry.state == tableAbsent {
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
	switch {
	case entry == nil || entry.state == tableUnknown:
		if ifNotExists {
			// Conditional create on an unknown target cannot prove the
			// declared shape landed — existence is proven, members are not.
			s.entries[key] = presentIncompleteEntry(displaySchema, displayTable)
			return nil
		}
		s.entries[key] = &batchTableEntry{
			state:         tablePresent,
			shape:         s.derivedCreateShape(displaySchema, displayTable, ddl),
			displaySchema: displaySchema,
			displayTable:  displayTable,
		}
	case entry.state == tableAbsent:
		s.entries[key] = &batchTableEntry{
			state:         tablePresent,
			shape:         s.derivedCreateShape(displaySchema, displayTable, ddl),
			displaySchema: displaySchema,
			displayTable:  displayTable,
		}
	case entry.state == tablePresent:
		if !ifNotExists {
			s.invalidateKey(key)
		}
	}
	return nil
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

// fullyAuditedCreateTable reports whether a CREATE TABLE's post-state shape is
// the extracted column list: no unsupported marker, no aspect gaps, no
// copy-source or select-driven shape, no temporary scope (a session-scoped
// object is not schema state), and no collapsed extra targets.
func fullyAuditedCreateTable(dialect spec.Dialect, statement spec.Statement) bool {
	ddl := statement.DDL
	if statement.Unsupported != nil || len(statementCoverageAspects(dialect, statement)) > 0 {
		return false
	}
	return !ddl.HasReferTable && !ddl.HasSelect && ddl.TemporaryScope == "" && ddl.OmittedTargets == 0
}

// createPartitionBoundBounded reports whether a CREATE's only unmodelled
// premise is partition-bound comparability — no unsupported marker, no
// structural flags, and every coverage aspect is a partition-bound item
// the declared-scheme gate inside applyCreateTable already adjudicates.
func createPartitionBoundBounded(dialect spec.Dialect, statement spec.Statement) bool {
	ddl := statement.DDL
	if statement.Unsupported != nil || ddl.HasReferTable || ddl.HasSelect ||
		ddl.TemporaryScope != "" || ddl.OmittedTargets != 0 {
		return false
	}
	for _, aspect := range statementCoverageAspects(dialect, statement) {
		if aspect.Metadata["aspect"] != "partition_bound" {
			return false
		}
	}
	return true
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
		return s.applyRenamePair(ctx, targets[0], targets[1])
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

// applyRenameTable dispatches a standalone RENAME TABLE. Only a single
// bounded pair enters migration; multi-pair forms invalidate every endpoint
// because partial success is not modeled.
func (s *batchState) applyRenameTable(ctx context.Context, statement spec.Statement) error {
	ddl := statement.DDL
	targets := ddl.TableTargets()
	if !fullyAuditedStatement(s.dialect, statement) || len(ddl.Alter) != 1 || len(targets) != 2 {
		s.invalidateAll(targets)
		return nil
	}
	// Cross-check the positional pair against the alter payload's own
	// old/new identity options so the two normalized views cannot diverge.
	alter := ddl.Alter[0]
	source, destination := targets[0], targets[1]
	if alter.Action != "rename_table" ||
		alter.Options["old_table"] != source.Name || alter.Options["old_schema"] != source.Schema ||
		alter.Options["new_table"] != destination.Name || alter.Options["new_schema"] != destination.Schema {
		s.invalidateAll(targets)
		return nil
	}
	return s.applyRenamePair(ctx, source, destination)
}

// applyRenamePair applies the frozen endpoint-existence table for one
// bounded rename pair. The destination resolves through the shared once-per-
// key seam — a cached answer (including an earlier tombstone) is reused, an
// invalidated schema answers unknown without a provider call, and a provider
// error propagates before either endpoint's post-state is published.
// Migration runs only on the one proven cell — known-present source onto a
// known-absent destination carrying a movable member set: the destination
// receives a fresh deep-copied shape under its own identity and the source
// becomes a settled known-absent entry. Every other cell tombstones both
// ends; unknown inputs never fabricate absent or present facts.
func (s *batchState) applyRenamePair(ctx context.Context, source, destination spec.Table) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	sourceKey := s.keyFor(s.schema, source)
	destinationKey := s.keyFor(s.schema, destination)
	if sourceKey == destinationKey {
		// The endpoints are not distinguishable identities (same key under
		// the effective schema) — conservatively invalidate rather than run
		// a write-delete-write migration.
		dependentKeys := s.renameDependentKeys(sourceKey, destinationKey)
		if err := ctx.Err(); err != nil {
			return err
		}
		for _, key := range dependentKeys {
			s.invalidateKey(key)
		}
		s.invalidateKey(sourceKey)
		return nil
	}
	sourceEntry, err := s.resolve(ctx, sourceKey, s.effectiveSchema(s.schema, source), source.Name)
	if err != nil {
		return fmt.Errorf("load table snapshot for %s: %w", source.Name, err)
	}
	destinationEntry, err := s.resolve(ctx, destinationKey, s.effectiveSchema(s.schema, destination), destination.Name)
	if err != nil {
		return fmt.Errorf("load table snapshot for %s: %w", destination.Name, err)
	}
	dependentKeys := s.renameDependentKeys(sourceKey, destinationKey)
	movable := sourceEntry.state == tablePresent && destinationEntry.state == tableAbsent && renameShapeMovable(sourceEntry.shape)
	destinationSchema := s.effectiveSchema(s.schema, destination)
	var migrated *spec.TableSnapshot
	if movable && sourceEntry.shape != nil {
		migrated = cloneTableSnapshot(sourceEntry.shape)
		migrated.Schema = destinationSchema
		migrated.Table = &spec.Table{Schema: destinationSchema, Name: destination.Name}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	for _, key := range dependentKeys {
		s.invalidateKey(key)
	}
	if movable {
		s.entries[destinationKey] = &batchTableEntry{
			state:         tablePresent,
			shape:         migrated,
			displaySchema: destinationSchema,
			displayTable:  destination.Name,
		}
		s.entries[sourceKey] = &batchTableEntry{
			state:         tableAbsent,
			displaySchema: s.effectiveSchema(s.schema, source),
			displayTable:  source.Name,
		}
		return nil
	}
	s.invalidateKey(sourceKey)
	s.invalidateKey(destinationKey)
	return nil
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

func (s *batchState) applyDropTable(ctx context.Context, statement spec.Statement) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	ddl := statement.DDL
	targets := ddl.TableTargets()
	keys := make(map[batchTableKey]struct{}, len(targets)+1)
	unbound := len(targets) == 0
	for _, target := range targets {
		if strings.TrimSpace(target.Name) == "" {
			unbound = true
			continue
		}
		keys[s.keyFor(s.schema, target)] = struct{}{}
	}
	if ddl.Table != nil && strings.TrimSpace(ddl.Table.Name) != "" {
		keys[s.keyFor(s.schema, *ddl.Table)] = struct{}{}
	}
	dependentKeys := s.referencingEntryKeys(keys)
	precise := orderedStateDialect(s.dialect) && statement.Kind == spec.KindDDL && !unbound && len(targets) == 1 && ddl.Table != nil && strings.TrimSpace(ddl.Table.Name) != "" && ddl.TemporaryScope == "" && ddl.OmittedTargets == 0 && fullyAuditedStatement(s.dialect, statement)
	var absent *batchTableEntry
	var targetKey batchTableKey
	if precise {
		targetKey = s.keyFor(s.schema, targets[0])
		precise = targetKey == s.keyFor(s.schema, *ddl.Table)
		entry := s.entries[targetKey]
		if precise && entry != nil && (entry.state == tablePresent || (entry.state == tableAbsent && ddl.Options["if_exists"] == "true")) {
			absent = &batchTableEntry{state: tableAbsent, displaySchema: s.effectiveSchema(s.schema, targets[0]), displayTable: targets[0].Name}
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if unbound {
		s.contaminated = true
		return nil
	}
	for _, key := range dependentKeys {
		s.invalidateKey(key)
	}
	for key := range keys {
		s.invalidateKey(key)
	}
	if absent != nil {
		s.entries[targetKey] = absent
	}
	return nil
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
// table_rows and cardinality must not carry into the post-state. Any other
// prior state invalidates: a truncate that cannot have succeeded must not
// fabricate existence, and an absent premise is already broken.
func (s *batchState) applyTruncateTable(statement spec.Statement) {
	ddl := statement.DDL
	targets := ddl.TableTargets()
	if !fullyAuditedStatement(s.dialect, statement) || len(targets) == 0 {
		s.invalidateAll(targets)
		return
	}
	key := s.keyFor(s.schema, targets[0])
	entry := s.entries[key]
	if entry == nil || entry.state != tablePresent {
		s.invalidateKey(key)
		return
	}
	if entry.shape == nil {
		return
	}
	shape := cloneTableSnapshot(entry.shape)
	clearMutationStats(shape)
	s.entries[key] = &batchTableEntry{state: tablePresent, shape: shape, displaySchema: entry.displaySchema, displayTable: entry.displayTable}
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
	switch action {
	case "drop_partition", "truncate_partition", "exchange_partition",
		"add_partitions", "reorganize_partition", "coalesce_partitions",
		"partition", "remove_partitioning",
		"rebuild_partition", "check_partitions", "optimize_partition",
		"repair_partition", "import_partition_tablespace", "discard_partition_tablespace":
		return true
	default:
		return false
	}
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
