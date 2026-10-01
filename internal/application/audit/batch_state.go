// Package audit orchestrates audit use cases at the application layer.
// input: ordered statements, optional metadata provider, and parse-failure positions for one audit request
// output: request-local ordered table facts (unknown / known-absent / known-present) feeding per-statement pre-state snapshots
// pos: prospective schema-state ownership for the first migration path — provider facts load once, deterministic conditional transitions update derived facts, unaudited or contaminated operations invalidate them
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
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
// structured (dialect, schema, table) triple. Schema is resolved the same way
// the provider read resolves it (the effective request schema for DDL, or the
// statement-qualified schema for DML), so the key always names the fact
// actually consulted.
type batchTableKey struct {
	dialect spec.Dialect
	schema  string
	table   string
}

// batchTableEntry is the owned knowledge record for one table. shape holds a
// deep copy of the provider snapshot or the derived create shape; it is never
// aliased into statements — rules receive projections (copies) instead.
// shapeComplete=false means the table is known to exist but its column/index
// collections are unknown (e.g. CREATE TABLE IF NOT EXISTS on an
// unknown-existence target). A complete shape always keeps every collection
// non-nil so consumers can distinguish "known empty" from "not provided".
// displaySchema/displayTable keep the caller-supplied casing for projections;
// the map key lowercases for identity matching.
type batchTableEntry struct {
	state         tableKnowledge
	shape         *spec.TableSnapshot
	shapeComplete bool
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
}

func newBatchState(dialect spec.Dialect, schema string, provider MetadataProvider) *batchState {
	return &batchState{
		provider: provider,
		dialect:  dialect,
		schema:   strings.TrimSpace(schema),
		entries:  make(map[batchTableKey]*batchTableEntry),
	}
}

// orderedStateDialect reports whether the dialect participates in the ordered
// prospective-state pass. PostgreSQL keeps its existing per-statement metadata
// enrichment unchanged in this slice.
func orderedStateDialect(dialect spec.Dialect) bool {
	return dialect == spec.DialectMySQL || dialect == spec.DialectTiDB
}

// keyFor resolves a table to its state key under the effective schema — the
// same schema the provider read was issued against. Dotted names stay whole:
// the table part is never split on "." and the schema part is never inferred
// from a different table's qualifier.
func (s *batchState) keyFor(schema string, table spec.Table) batchTableKey {
	resolved := strings.TrimSpace(schema)
	if resolved == "" {
		resolved = s.schema
	}
	return batchTableKey{dialect: s.dialect, schema: strings.ToLower(resolved), table: strings.ToLower(table.Name)}
}

// preState returns the projected per-statement snapshot for one target. The
// provider is consulted at most once per key with the statement's original
// (un-lowercased) names; a provider error aborts the audit unchanged (errors
// never degrade into evidence gaps). After contamination the projection is
// always nil (unknown) regardless of any earlier cached fact.
func (s *batchState) preState(ctx context.Context, schema string, table spec.Table) (*spec.TableSnapshot, error) {
	key := s.keyFor(schema, table)
	providerSchema := strings.TrimSpace(schema)
	if providerSchema == "" {
		providerSchema = s.schema
	}
	entry, err := s.resolve(ctx, key, providerSchema, table.Name)
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
// provider or a nil snapshot leaves the entry unknown; contamination
// suppresses provider reads for never-touched keys.
func (s *batchState) resolve(ctx context.Context, key batchTableKey, providerSchema, providerTable string) (*batchTableEntry, error) {
	if entry, ok := s.entries[key]; ok {
		return entry, nil
	}
	entry := &batchTableEntry{state: tableUnknown}
	if s.contaminated || s.provider == nil {
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
		entry.shapeComplete = snapshot.Columns != nil
	} else {
		entry.state = tableAbsent
		entry.shapeComplete = true
	}
	s.entries[key] = entry
	return entry, nil
}

// projectEntry renders one statement-visible snapshot from an owned entry.
// Definite entries return fresh copies. An incomplete present shape projects
// only the table-level facts that are actually known (existence, identity,
// provider-reported options) with every structural collection nil so rule code
// can distinguish "set not provided" from an audited empty set.
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
		if entry.shapeComplete {
			return cloneTableSnapshot(entry.shape)
		}
		out := &spec.TableSnapshot{Schema: displaySchema, Table: &spec.Table{Schema: displaySchema, Name: displayTable}, Exists: true}
		if entry.shape != nil {
			if entry.shape.Table != nil {
				table := *entry.shape.Table
				out.Table = &table
			}
			out.Options = cloneOptions(entry.shape.Options)
		}
		return out
	}
	return nil
}

func cloneOptions(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for name, value := range in {
		out[name] = value
	}
	return out
}

// apply records the conditional post-state of one statement after its
// pre-state was captured. Only the three frozen transitions derive structure:
// a fully audited plain CREATE TABLE, an ALTER made entirely of plain
// ADD COLUMN actions, and a fully audited standalone CREATE INDEX — plus the
// definite-absence DROP TABLE transition and stat-clearing TRUNCATE/DML
// pass-through. Everything else (unsupported statements, unaudited aspects,
// audited-but-unmodeled actions, RENAME TABLE) invalidates its bound targets
// because the real effect is not modeled honestly. A statement whose
// structural premise is already known-broken (duplicate plain ADD, index on a
// confirmed-absent table) invalidates its target rather than fabricating the
// success shape. Contaminated batches derive nothing.
func (s *batchState) apply(statement spec.Statement) {
	if s.contaminated {
		return
	}
	if statement.Kind != spec.KindDDL || statement.DDL == nil {
		s.applyDMLMutation(statement)
		return
	}
	ddl := statement.DDL
	if statement.Unsupported != nil {
		// A recognized-but-unsupported DDL statement has no bounded effect.
		// When its scope resolves to tables, invalidate those; an unresolvable
		// scope contaminates the batch — it must not pass through as a no-op.
		if targets := ddl.TableTargets(); len(targets) > 0 {
			s.invalidateAll(targets)
		} else {
			s.contaminated = true
		}
		return
	}
	switch ddl.Operation {
	case spec.DDLOperationCreateTable:
		s.applyCreateTable(statement)
	case spec.DDLOperationAlterTable:
		s.applyAlterTable(statement)
	case spec.DDLOperationCreateIndex:
		s.applyCreateIndex(statement)
	case spec.DDLOperationDropTable:
		s.applyDropTable(statement)
	case spec.DDLOperationTruncateTable:
		s.applyTruncateTable(statement)
	default:
		s.invalidateAll(ddl.TableTargets())
	}
}

// fullyAuditedStatement reports whether every parsed aspect of the statement
// is consumed by the audit model. Unsupported markers and coverage aspect gaps
// both mean the statement's effect is not bounded enough to model.
func fullyAuditedStatement(dialect spec.Dialect, statement spec.Statement) bool {
	return statement.Unsupported == nil && len(statementCoverageAspects(dialect, statement)) == 0
}

func (s *batchState) invalidateAll(targets []spec.Table) {
	for _, target := range targets {
		key := s.keyFor(s.schema, target)
		s.entries[key] = &batchTableEntry{state: tableUnknown}
	}
}

func (s *batchState) invalidateKey(key batchTableKey) {
	s.entries[key] = &batchTableEntry{state: tableUnknown}
}

func presentIncompleteEntry(schema, table string) *batchTableEntry {
	return &batchTableEntry{
		state:         tablePresent,
		shape:         &spec.TableSnapshot{Schema: schema, Table: &spec.Table{Schema: schema, Name: table}, Exists: true},
		shapeComplete: false,
		displaySchema: schema,
		displayTable:  table,
	}
}

func absentEntryFor(schema, table string) *batchTableEntry {
	return &batchTableEntry{state: tableAbsent, shapeComplete: true, displaySchema: schema, displayTable: table}
}

// applyCreateTable derives a known-present shape from a fully audited CREATE
// TABLE. Conditional success semantics: a plain create on an unknown target
// derives the declared shape; on a known-absent target it deterministically
// creates it. CREATE TABLE IF NOT EXISTS on an unknown target cannot prove
// the declared columns landed, so it yields present-incomplete; on a
// known-present target it is a no-op that keeps the real fact. A plain create
// on a known-present target can never have succeeded — the premise is broken
// and the entry invalidates instead of inventing which shape survived.
func (s *batchState) applyCreateTable(statement spec.Statement) {
	ddl := statement.DDL
	targets := ddl.TableTargets()
	if len(targets) == 0 || ddl.Table == nil {
		return
	}
	key := s.keyFor(s.schema, targets[0])
	if !fullyAuditedCreateTable(s.dialect, statement) {
		s.invalidateKey(key)
		return
	}
	entry := s.entries[key]
	ifNotExists := ddl.Options["if_not_exists"] == "true"
	displaySchema, displayTable := s.schema, targets[0].Name
	switch {
	case entry == nil || entry.state == tableUnknown:
		s.entries[key] = &batchTableEntry{
			state:         tablePresent,
			shape:         derivedCreateShape(displaySchema, displayTable, ddl),
			shapeComplete: !ifNotExists,
			displaySchema: displaySchema,
			displayTable:  displayTable,
		}
	case entry.state == tableAbsent:
		s.entries[key] = &batchTableEntry{
			state:         tablePresent,
			shape:         derivedCreateShape(displaySchema, displayTable, ddl),
			shapeComplete: true,
			displaySchema: displaySchema,
			displayTable:  displayTable,
		}
	case entry.state == tablePresent:
		if !ifNotExists {
			s.invalidateKey(key)
		}
	}
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

func derivedCreateShape(schema, table string, ddl *spec.DDL) *spec.TableSnapshot {
	snapshot := &spec.TableSnapshot{
		Schema:      schema,
		Table:       &spec.Table{Schema: schema, Name: table},
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
	if ddl.PrimaryKey != nil {
		pk := cloneIndex(*ddl.PrimaryKey)
		snapshot.PrimaryKey = &pk
	}
	for _, constraint := range ddl.Constraints {
		snapshot.Constraints = append(snapshot.Constraints, cloneConstraint(constraint))
	}
	if len(ddl.Options) > 0 {
		snapshot.Options = make(map[string]string, len(ddl.Options))
		for name, value := range ddl.Options {
			snapshot.Options[name] = value
		}
	}
	return snapshot
}

// applyAlterTable models a fully audited ALTER made entirely of plain
// ADD COLUMN actions. rename_table invalidates both ends of the rename
// (precise identity migration is a later slice); every other audited action
// has no modeled effect this slice, so its bound targets invalidate rather
// than passing stale facts through. Premise-broken adds — a duplicate column
// without IF NOT EXISTS, or any add on a confirmed-absent table — invalidate
// instead of fabricating a success structure.
func (s *batchState) applyAlterTable(statement spec.Statement) {
	ddl := statement.DDL
	if !fullyAuditedStatement(s.dialect, statement) {
		s.invalidateAll(ddl.TableTargets())
		return
	}
	adds := make([]*spec.Alter, 0, len(ddl.Alter))
	modeledOnly := true
	for i := range ddl.Alter {
		alter := &ddl.Alter[i]
		if alter.Action == "add_columns" && alter.Column != nil && alter.Column.Definition != nil {
			adds = append(adds, alter)
			continue
		}
		modeledOnly = false
		break
	}
	if !modeledOnly {
		s.invalidateAll(ddl.TableTargets())
		return
	}
	targets := ddl.TableTargets()
	if len(targets) == 0 {
		return
	}
	key := s.keyFor(s.schema, targets[0])
	entry := s.entries[key]
	switch {
	case entry == nil || entry.state == tableUnknown:
		// A successful ADD COLUMN proves the table exists; the pre-existing
		// shape is unknown, so only existence is definite afterwards.
		s.entries[key] = presentIncompleteEntry(s.schema, targets[0].Name)
	case entry.state == tableAbsent:
		// The premise (table exists) is already known-broken.
		s.invalidateKey(key)
	case entry.state == tablePresent && entry.shapeComplete && entry.shape != nil:
		for _, alter := range adds {
			def := alter.Column.Definition
			if entry.shape.FindColumn(def.Name) != nil && alter.Options["if_not_exists"] != "true" {
				// Duplicate plain ADD can never succeed — do not fabricate
				// the added column, invalidate the table instead.
				s.invalidateKey(key)
				return
			}
		}
		shape := cloneTableSnapshot(entry.shape)
		for _, alter := range adds {
			def := alter.Column.Definition
			// IF NOT EXISTS keeps the existing definition on a duplicate;
			// a fresh column appends the declared definition.
			if shape.FindColumn(def.Name) == nil {
				shape.Columns = append(shape.Columns, cloneColumn(*def))
			}
		}
		s.entries[key] = &batchTableEntry{state: tablePresent, shape: shape, shapeComplete: true, displaySchema: entry.displaySchema, displayTable: entry.displayTable}
	}
	// present-incomplete stays incomplete: the conditional add proves the
	// column exists, but the shape model cannot express a partial column set.
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
	key := s.keyFor(s.schema, targets[0])
	entry := s.entries[key]
	if !fullyAuditedStatement(s.dialect, statement) || (entry != nil && entry.state == tableAbsent) {
		s.invalidateKey(key)
		return
	}
	if entry == nil || entry.state == tableUnknown {
		s.entries[key] = presentIncompleteEntry(s.schema, targets[0].Name)
		return
	}
	if !entry.shapeComplete || entry.shape == nil {
		return
	}
	for _, alter := range ddl.Alter {
		if alter.Action != "create_index" || alter.Index == nil || alter.Index.Definition == nil {
			continue
		}
		def := alter.Index.Definition
		if entry.shape.HasIndex(def.Name) {
			s.invalidateKey(key)
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
	s.entries[key] = &batchTableEntry{state: tablePresent, shape: shape, shapeComplete: true, displaySchema: entry.displaySchema, displayTable: entry.displayTable}
}

// applyDropTable derives definite absence for a fully audited DROP TABLE.
// IF EXISTS succeeds on any prior state; a plain drop on a confirmed-absent
// target can never succeed, so the premise is broken and the entry
// invalidates rather than silently keeping the stale absence.
func (s *batchState) applyDropTable(statement spec.Statement) {
	ddl := statement.DDL
	if !fullyAuditedStatement(s.dialect, statement) {
		s.invalidateAll(ddl.TableTargets())
		return
	}
	ifExists := ddl.Options["if_exists"] == "true"
	for _, target := range ddl.TableTargets() {
		key := s.keyFor(s.schema, target)
		entry := s.entries[key]
		switch {
		case ifExists:
			s.entries[key] = absentEntryFor(s.schema, target.Name)
		case entry != nil && entry.state == tableAbsent:
			s.invalidateKey(key)
		default:
			s.entries[key] = absentEntryFor(s.schema, target.Name)
		}
	}
}

// applyTruncateTable keeps a present table's structure but clears row and
// index statistics — TRUNCATE empties the table, so provider-reported
// table_rows and cardinality must not carry into the post-state. Conditional
// success on an unknown target proves existence; on a confirmed-absent target
// the premise is broken and the entry invalidates.
func (s *batchState) applyTruncateTable(statement spec.Statement) {
	ddl := statement.DDL
	targets := ddl.TableTargets()
	if !fullyAuditedStatement(s.dialect, statement) || len(targets) == 0 {
		s.invalidateAll(targets)
		return
	}
	key := s.keyFor(s.schema, targets[0])
	entry := s.entries[key]
	switch {
	case entry == nil || entry.state == tableUnknown:
		s.entries[key] = presentIncompleteEntry(s.schema, targets[0].Name)
	case entry.state == tableAbsent:
		s.invalidateKey(key)
	default:
		shape := cloneTableSnapshot(entry.shape)
		clearMutationStats(shape)
		s.entries[key] = &batchTableEntry{state: tablePresent, shape: shape, shapeComplete: entry.shapeComplete, displaySchema: entry.displaySchema, displayTable: entry.displayTable}
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
		key := s.keyFor(target.Schema, target)
		entry, ok := s.entries[key]
		if !ok || entry.state != tablePresent || entry.shape == nil {
			continue
		}
		shape := cloneTableSnapshot(entry.shape)
		clearMutationStats(shape)
		s.entries[key] = &batchTableEntry{state: tablePresent, shape: shape, shapeComplete: entry.shapeComplete}
	}
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
	return out
}

func cloneIndex(in spec.Index) spec.Index {
	out := in
	out.Columns = append([]string(nil), in.Columns...)
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
	return out
}
