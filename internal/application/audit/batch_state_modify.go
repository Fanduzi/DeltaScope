// Package audit derives the conditional post-state of one ordinary MODIFY.
// input: one ALTER TABLE MODIFY COLUMN, the immutable request-local entries
// captured before the statement, and loaded constraint facts already present
// in this batch
// output: an in-place column replacement, or one conservative invalidation of
// the target and the already-loaded dependents that reference its column
// pos: T05-A4 state supply for ordinary VARCHAR and integer MODIFY; a
// non-precise branch still drops those dependents, an inline PRIMARY KEY is
// read from the parser presence fact, and a TiDB known-primary-key signedness
// change stays conservative. It does not change compatibility-rule meaning
// and does not read the provider again
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// modifyPublication is the write set of one MODIFY, applied only after the
// caller has rechecked cancellation. A replace and an invalidation are
// alternatives for the target; children are invalidated together with it.
type modifyPublication struct {
	invalidate []batchTableKey
	replaceKey batchTableKey
	replace    *batchTableEntry
}

func (p modifyPublication) apply(s *batchState) {
	for _, key := range p.invalidate {
		s.invalidateKey(key)
	}
	if p.replace != nil {
		s.entries[p.replaceKey] = p.replace
	}
}

// applyModifyColumn publishes the frozen ordinary-MODIFY post-state.
// Unknown and contaminated entries stay unknown. A known-absent table becomes
// unknown. A known column set is replaced in place; a withheld column set is
// left withheld. Cancellation before the publication returns the context
// error and leaves every entry unchanged.
func (s *batchState) applyModifyColumn(ctx context.Context, statement spec.Statement, alter *spec.Alter, targets []spec.Table) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	publication := s.prepareModifyColumn(statement, alter, targets)
	if err := ctx.Err(); err != nil {
		return err
	}
	publication.apply(s)
	return nil
}

func (s *batchState) prepareModifyColumn(statement spec.Statement, alter *spec.Alter, targets []spec.Table) modifyPublication {
	var key batchTableKey
	if len(targets) > 0 {
		key = s.keyFor(s.schema, targets[0])
	}
	// Dependents come from the immutable pre-state before any replace-or-
	// invalidate decision. A later return must not skip them.
	dependents := s.modifyDependentKeys(key, modifyColumnNames(statement.DDL, alter))
	invalidate := func(includeTarget bool) modifyPublication {
		keys := append([]batchTableKey{}, dependents...)
		if includeTarget {
			keys = append(keys, key)
		}
		return modifyPublication{invalidate: keys}
	}
	if !modifyColumnTemplate(s, statement, alter, targets) {
		return invalidate(len(targets) > 0)
	}
	entry := s.entries[key]
	if entry == nil || entry.state == tableUnknown || entry.state == tableAbsent {
		return invalidate(true)
	}
	// A withheld column set is not a one-column table. Keep the target's other
	// known members, and still drop loaded dependents of the named column.
	if entry.shape == nil || entry.shape.Columns == nil {
		return invalidate(false)
	}
	index := modifyColumnIndex(entry.shape.Columns, alter.Name)
	if index < 0 {
		return invalidate(true)
	}
	source := entry.shape.Columns[index]
	definition := alter.Column.Definition
	inPK, pkKnown := primaryKeyMembership(entry.shape, source.Name)
	if !modifyTypeTemplate(source, *definition) ||
		modifyDeclaresExtraMember(alter, *definition) ||
		modifyTiDBPrimaryKeySignedness(s.dialect, inPK, source, *definition) ||
		(!pkKnown && !modifyExplicitNotNull(alter, *definition)) ||
		(inPK && modifyExplicitNull(alter, *definition)) ||
		modifyLocalUnsafe(entry.shape, source.Name) {
		return invalidate(true)
	}
	next := cloneColumn(*definition)
	if inPK {
		next.NotNull = true
	}
	applyModifyColumnDefaults(&next, entry.shape)
	shape := cloneTableSnapshot(entry.shape)
	shape.Columns[index] = next
	clearModifyAffectedStats(shape)
	return modifyPublication{
		invalidate: dependents,
		replaceKey: key,
		replace: &batchTableEntry{
			state:         tablePresent,
			shape:         shape,
			displaySchema: entry.displaySchema,
			displayTable:  entry.displayTable,
		},
	}
}

// firstModifyAlter returns the first MODIFY action. A statement with several
// actions is still not a success template; the caller uses it only to reach
// the shared affected-set publication.
func firstModifyAlter(ddl *spec.DDL) *spec.Alter {
	if ddl == nil {
		return nil
	}
	for i := range ddl.Alter {
		if ddl.Alter[i].Action == "modify_column" {
			return &ddl.Alter[i]
		}
	}
	return nil
}

func modifyColumnNames(ddl *spec.DDL, alter *spec.Alter) []string {
	var names []string
	if ddl != nil {
		for i := range ddl.Alter {
			item := &ddl.Alter[i]
			if item.Action != "modify_column" {
				continue
			}
			if name := strings.TrimSpace(item.Name); name != "" {
				names = append(names, name)
				continue
			}
			if item.Column != nil && item.Column.Definition != nil {
				if name := strings.TrimSpace(item.Column.Definition.Name); name != "" {
					names = append(names, name)
				}
			}
		}
	}
	if len(names) == 0 && alter != nil && alter.Column != nil && alter.Column.Definition != nil {
		if name := strings.TrimSpace(alter.Column.Definition.Name); name != "" {
			names = append(names, name)
		}
	}
	return names
}

// modifyColumnTemplate reports whether this single MODIFY is the frozen
// ordinary replacement: one bounded target, matching identities, no
// conditional or positional modifier, and a complete new column definition.
func modifyColumnTemplate(s *batchState, statement spec.Statement, alter *spec.Alter, targets []spec.Table) bool {
	ddl := statement.DDL
	if ddl == nil || ddl.Table == nil || len(ddl.Alter) != 1 || len(targets) != 1 || alter == nil {
		return false
	}
	if !fullyAuditedStatement(s.dialect, statement) {
		return false
	}
	if s.keyFor(s.schema, targets[0]) != s.keyFor(s.schema, *ddl.Table) {
		return false
	}
	if alter.Action != "modify_column" || alter.HasColumnPosition {
		return false
	}
	if alter.Options["if_exists"] == "true" || alter.Options["if_not_exists"] == "true" {
		return false
	}
	if len(alter.UnextractedOptions) > 0 || !modifyColumnIdentity(alter) {
		return false
	}
	return strings.TrimSpace(alter.Column.Definition.Type) != ""
}

func modifyColumnIdentity(alter *spec.Alter) bool {
	if alter.Column == nil || alter.Column.Definition == nil {
		return false
	}
	name := alter.Column.Definition.Name
	if name == "" || alter.Name == "" || !strings.EqualFold(alter.Name, name) {
		return false
	}
	oldName := strings.TrimSpace(alter.Column.OldName)
	return oldName == "" || strings.EqualFold(oldName, name)
}

// modifyTypeTemplate accepts ordinary VARCHAR and integer columns in the
// same class. A missing source type still accepts a complete new definition;
// a cross-family conversion does not.
func modifyTypeTemplate(source, target spec.Column) bool {
	targetClass := ordinaryModifyClass(target)
	if targetClass == "" || (targetClass == "varchar" && target.Length <= 0) {
		return false
	}
	if strings.TrimSpace(source.Type) == "" {
		return true
	}
	return ordinaryModifyClass(source) == targetClass
}

func ordinaryModifyClass(column spec.Column) string {
	switch modifyBaseType(column) {
	case "varchar":
		return "varchar"
	case "tinyint", "smallint", "mediumint", "int", "integer", "bigint":
		return "integer"
	default:
		return ""
	}
}

func modifyBaseType(column spec.Column) string {
	tp := strings.ToLower(strings.TrimSpace(column.Type))
	if idx := strings.Index(tp, "("); idx >= 0 {
		tp = tp[:idx]
	}
	if idx := strings.Index(tp, " "); idx >= 0 {
		tp = tp[:idx]
	}
	return tp
}

// modifyDeclaresExtraMember rejects a new declaration that adds a primary
// key, AUTO_INCREMENT, identity, or another unaudited column member. Inline
// PRIMARY KEY is the parser-owned DeclaresPrimaryKey fact, not a combination
// of NotNull and TouchesNullability.
func modifyDeclaresExtraMember(alter *spec.Alter, definition spec.Column) bool {
	if definition.AutoIncrement || definition.AutoRandom || definition.IsIdentity || strings.TrimSpace(definition.GeneratedWhen) != "" {
		return true
	}
	if len(definition.UnextractedOptions) > 0 {
		return true
	}
	return modifyDeclaresPrimaryKey(alter)
}

func modifyDeclaresPrimaryKey(alter *spec.Alter) bool {
	return alter != nil && alter.Column != nil && alter.Column.Change != nil && alter.Column.Change.DeclaresPrimaryKey
}

// modifyTiDBPrimaryKeySignedness reports the TiDB 8.5 primary-key boundary
// documented as ERROR 8200: changing signedness of a known primary-key integer
// is not a meta-only MODIFY. INT to BIGINT, and the same change on MySQL or
// on a non-key column, stays inside the ordinary integer template.
func modifyTiDBPrimaryKeySignedness(dialect spec.Dialect, inPK bool, source, target spec.Column) bool {
	if dialect != spec.DialectTiDB || !inPK {
		return false
	}
	if ordinaryModifyClass(source) != "integer" || ordinaryModifyClass(target) != "integer" {
		return false
	}
	return source.Unsigned != target.Unsigned
}

func modifyTouchesNullability(alter *spec.Alter) bool {
	return alter != nil && alter.Column != nil && alter.Column.Change != nil && alter.Column.Change.TouchesNullability
}

func modifyExplicitNotNull(alter *spec.Alter, definition spec.Column) bool {
	return modifyTouchesNullability(alter) && definition.NotNull
}

func modifyExplicitNull(alter *spec.Alter, definition spec.Column) bool {
	return modifyTouchesNullability(alter) && !definition.NotNull
}

// primaryKeyMembership reads both PrimaryKey.Columns and a primary_key
// constraint. An empty column list or an unknown primary-key collection does
// not claim the column is outside the key.
func primaryKeyMembership(shape *spec.TableSnapshot, name string) (inPK bool, known bool) {
	known = !shape.PrimaryKeyUnknown
	if shape.PrimaryKey != nil {
		if len(shape.PrimaryKey.Columns) == 0 {
			known = false
		} else if columnListContains(shape.PrimaryKey.Columns, name) {
			return true, true
		}
	}
	for _, constraint := range shape.Constraints {
		if !primaryKeyConstraint(constraint.Type) {
			continue
		}
		if len(constraint.Columns) == 0 {
			if !inPK {
				known = false
			}
			continue
		}
		if columnListContains(constraint.Columns, name) {
			return true, true
		}
	}
	return false, known
}

func primaryKeyConstraint(constraintType string) bool {
	switch strings.ToLower(strings.TrimSpace(constraintType)) {
	case "primary_key", "primary":
		return true
	default:
		return false
	}
}

// applyModifyColumnDefaults fills charset and collation only when the new
// declaration omits both and the table options already carry one value.
// A one-sided declaration is not completed from the other side, and
// contradictory table collation keys are left unknown.
func applyModifyColumnDefaults(column *spec.Column, shape *spec.TableSnapshot) {
	if strings.TrimSpace(column.Charset) != "" || strings.TrimSpace(column.Collation) != "" || shape == nil || shape.Options == nil {
		return
	}
	if charset, ok := knownOption(shape.Options, "charset"); ok {
		column.Charset = charset
	}
	if collation, ok := knownTableCollation(shape.Options); ok {
		column.Collation = collation
	}
}

func knownOption(options map[string]string, key string) (string, bool) {
	value, ok := options[key]
	value = strings.TrimSpace(value)
	if !ok || value == "" {
		return "", false
	}
	return value, true
}

func knownTableCollation(options map[string]string) (string, bool) {
	collate, hasCollate := knownOption(options, "collate")
	collation, hasCollation := knownOption(options, "collation")
	if hasCollate && hasCollation && !strings.EqualFold(collate, collation) {
		return "", false
	}
	if hasCollate {
		return collate, true
	}
	return collation, hasCollation
}

// modifyLocalUnsafe reports loaded facts whose effect on the column cannot
// be recomputed: generated or identity columns, CHECK or foreign keys that
// do not prove they ignore the column, and prefix, expression, or other
// special indexes in the same position.
func modifyLocalUnsafe(shape *spec.TableSnapshot, columnName string) bool {
	for _, column := range shape.Columns {
		if modifyColumnGenerated(column) {
			return true
		}
	}
	for _, constraint := range shape.Constraints {
		if primaryKeyConstraint(constraint.Type) {
			continue
		}
		if !columnListProvesUnrelated(constraint.Columns, columnName, constraint.UnmodeledParts) {
			return true
		}
		if modifyReferencesOwnColumn(shape, constraint, columnName) {
			return true
		}
	}
	if shape.PrimaryKey != nil && modifyIndexUnsafe(*shape.PrimaryKey, columnName) {
		return true
	}
	for _, index := range shape.Indexes {
		if modifyIndexUnsafe(index, columnName) {
			return true
		}
	}
	return false
}

// modifyReferencesOwnColumn reports a self-foreign-key whose referenced side
// names this column. The local column list can exclude the column and still
// leave the reference unrecomputable.
func modifyReferencesOwnColumn(shape *spec.TableSnapshot, constraint spec.Constraint, columnName string) bool {
	if shape == nil || shape.Table == nil || strings.TrimSpace(constraint.ReferencedTable) == "" {
		return false
	}
	if !strings.EqualFold(constraint.ReferencedTable, shape.Table.Name) {
		return false
	}
	if refSchema := strings.TrimSpace(constraint.ReferencedSchema); refSchema != "" &&
		strings.TrimSpace(shape.Schema) != "" && !strings.EqualFold(refSchema, shape.Schema) {
		return false
	}
	return !columnListProvesUnrelated(constraint.ReferencedColumns, columnName, constraint.UnmodeledReferencedParts)
}

func modifyColumnGenerated(column spec.Column) bool {
	if strings.TrimSpace(column.GeneratedWhen) != "" || column.IsIdentity || column.AutoRandom {
		return true
	}
	for _, option := range column.UnextractedOptions {
		switch option {
		case "generated", "reference", "check", "auto_random", "unique", "unique_global":
			return true
		}
	}
	return false
}

func modifyIndexUnsafe(index spec.Index, columnName string) bool {
	if index.HasExpressionKeys || index.ExpressionCount > 0 {
		return true
	}
	if len(index.IncludedColumns) > 0 && !columnListProvesUnrelated(index.IncludedColumns, columnName, 0) {
		return true
	}
	if !modifyIndexSpecial(index) {
		return false
	}
	return !columnListProvesUnrelated(index.Columns, columnName, 0)
}

func modifyIndexSpecial(index spec.Index) bool {
	switch index.Kind {
	case spec.IndexKindFulltext, spec.IndexKindSpatial, spec.IndexKindVector, spec.IndexKindColumnar:
		return true
	}
	return index.PrefixParts > 0 || index.HasPredicate || index.DescParts > 0 || index.Global || len(index.UnmodeledOptions) > 0
}

// modifyDependentKeys collects loaded tables whose foreign key references one
// of the modified columns. The target itself is not included: a self-reference
// is decided with the target. An empty list or a list with unmodeled parts
// cannot prove the child is unrelated. Explicit schema wins; an unqualified
// reference uses the owning entry's schema. Entries not yet loaded are ignored.
func (s *batchState) modifyDependentKeys(target batchTableKey, columnNames []string) []batchTableKey {
	if len(columnNames) == 0 {
		return nil
	}
	var dependents []batchTableKey
	for key, entry := range s.entries {
		if key == target || entry == nil || entry.state != tablePresent || entry.shape == nil {
			continue
		}
		if modifyEntryReferences(s, key, entry, target, columnNames) {
			dependents = append(dependents, key)
		}
	}
	return dependents
}

func modifyEntryReferences(s *batchState, key batchTableKey, entry *batchTableEntry, target batchTableKey, columnNames []string) bool {
	for _, constraint := range entry.shape.Constraints {
		if strings.TrimSpace(constraint.ReferencedTable) == "" {
			continue
		}
		referenced := spec.Table{Schema: constraint.ReferencedSchema, Name: constraint.ReferencedTable}
		if s.keyFor(key.schema, referenced) != target {
			continue
		}
		for _, name := range columnNames {
			if !columnListProvesUnrelated(constraint.ReferencedColumns, name, constraint.UnmodeledReferencedParts) {
				return true
			}
		}
	}
	return false
}

func modifyColumnIndex(columns []spec.Column, name string) int {
	for i := range columns {
		if strings.EqualFold(columns[i].Name, name) {
			return i
		}
	}
	return -1
}

func columnListContains(columns []string, name string) bool {
	for _, column := range columns {
		if strings.EqualFold(strings.TrimSpace(column), name) {
			return true
		}
	}
	return false
}

// columnListProvesUnrelated is true only when the list is complete, every
// entry is a non-empty name, and none of them is the modified column.
// Unmodeled key parts mean the stored names are not the whole key.
func columnListProvesUnrelated(columns []string, name string, unmodeled int) bool {
	if unmodeled > 0 || len(columns) == 0 {
		return false
	}
	for _, column := range columns {
		if strings.TrimSpace(column) == "" || strings.EqualFold(column, name) {
			return false
		}
	}
	return true
}

// clearModifyAffectedStats drops index cardinality and storage counters a
// column redefinition can change. table_rows keeps its previous estimate
// identity and is not rewritten as an exact count.
func clearModifyAffectedStats(shape *spec.TableSnapshot) {
	if shape.Options != nil {
		delete(shape.Options, "data_length")
		delete(shape.Options, "index_length")
		delete(shape.Options, "avg_row_length")
		delete(shape.Options, "auto_increment")
	}
	if shape.PrimaryKey != nil {
		shape.PrimaryKey.Cardinality = nil
	}
	for i := range shape.Indexes {
		shape.Indexes[i].Cardinality = nil
	}
}
