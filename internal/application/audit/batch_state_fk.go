// Package audit publishes bounded foreign-key ordered-state transitions.
// input: one audited ALTER TABLE action carrying add_constraint with a
// foreign-key payload or drop_foreign_key, plus the request-local entries
// captured before the statement
// output: post-state member/index publications mirroring native semantics —
// a proven add merges the constraint member (a bare name stays unproven)
// and the implicit supporting index only when no member already covers its
// local columns (named → authored name; bare MySQL → first column, bare
// TiDB → unproven server-derived name); a proven drop removes the member
// and keeps indexes; a provable native refusal (duplicate name,
// still-needed index) is a no-op; unprovable premises unvouch or
// tombstone rather than fabricate
// pos: T11 foreign-key ordered-state transitions under batch state
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// applyAddForeignKey publishes a bounded add-foreign-key transition. The
// local-column premise mirrors applyAddIndex: every declared member column
// must exist on a vouched column set, and a withheld column set makes the
// premise unprovable. The referenced-side premise mirrors the local one
// across the table edge: a proven-absent parent or a proven-missing
// referenced column is a native refusal that leaves the child unchanged,
// while an unprovable parent or column set leaves the landing unprovable
// and unvouches the constraint set. A proven duplicate constraint name is
// the native 1826 refusal — a provable no-op — while an unvouched name
// space unvouches the constraint set instead of appending a member the
// statement may never have landed.
func (s *batchState) applyAddForeignKey(ctx context.Context, _ spec.Statement, alter *spec.Alter, targets []spec.Table) error {
	if len(targets) != 1 || alter == nil || alter.Constraint == nil || alter.Constraint.Type != "foreign_key" {
		s.invalidateAll(targets)
		return nil
	}
	constraint := alter.Constraint
	key := s.keyFor(s.schema, targets[0])
	entry := s.entries[key]
	switch {
	case entry == nil || entry.state == tableUnknown:
		s.entries[key] = presentIncompleteEntry(s.effectiveSchema(s.schema, targets[0]), targets[0].Name)
		return nil
	case entry.state == tableAbsent:
		s.invalidateKey(key)
		return nil
	}
	if entry.shape == nil {
		return nil
	}
	if entry.shape.Columns != nil {
		for _, column := range constraint.Columns {
			if entry.shape.FindColumn(column) == nil {
				s.invalidateKey(key)
				return nil
			}
		}
	} else if len(constraint.Columns) > 0 {
		s.invalidateKey(key)
		return nil
	}
	refused, decided, err := s.foreignKeyReferencedPremise(ctx, key, entry, constraint)
	if err != nil {
		return err
	}
	if !decided {
		s.markConstraintsUnknown(key)
		return nil
	}
	if refused {
		return nil
	}
	authored := strings.TrimSpace(constraint.Name)
	if authored != "" {
		if entry.shape.FindForeignKeyConstraint(authored) != nil {
			// A proven member already holds the name — the server refuses
			// the duplicate; the table is provably unchanged.
			return nil
		}
		if entry.shape.ForeignKeyNameSpaceUnvouched() {
			// A hidden member may already hold the name — the outcome is
			// unprovable, so the constraint set unvouches rather than
			// appending a member the statement may never have landed.
			s.markConstraintsUnknown(key)
			return nil
		}
	}
	shape := cloneTableSnapshot(entry.shape)
	member := cloneConstraint(*constraint)
	if authored == "" {
		// The server derives a catalog name for a bare foreign key — never
		// authored, so the stored name stays unproven.
		member.NameUnknown = true
	}
	if shape.Constraints != nil && !shape.ConstraintsUnknown {
		shape.Constraints = append(shape.Constraints, member)
	}
	s.publishForeignKeySupportingIndex(shape, member, authored)
	s.entries[key] = &batchTableEntry{state: tablePresent, shape: shape, displaySchema: entry.displaySchema, displayTable: entry.displayTable}
	return nil
}

// foreignKeyReferencedPremise resolves the referenced parent across the
// table edge and reports whether the native reference premise is decided:
// a proven-absent parent or a proven-missing referenced column refuses
// the add (refused=true, decided=true — the child is provably unchanged);
// a proven parent with every declared referenced column present lets the
// publish continue (decided=true, refused=false); anything else — an
// unknown parent, a withheld parent column set, or unmodeled referenced
// parts — leaves the landing unprovable (decided=false). A self-reference
// answers against the child's own pre-transition shape.
func (s *batchState) foreignKeyReferencedPremise(ctx context.Context, key batchTableKey, entry *batchTableEntry, constraint *spec.Constraint) (refused bool, decided bool, err error) {
	referenced := spec.Table{Schema: constraint.ReferencedSchema, Name: strings.TrimSpace(constraint.ReferencedTable)}
	if referenced.Name == "" {
		// No authored parent identity — nothing to check.
		return false, true, nil
	}
	parentKey := s.keyFor(key.schema, referenced)
	var parentEntry *batchTableEntry
	if parentKey == key {
		parentEntry = entry
	} else {
		parentEntry, err = s.resolve(ctx, parentKey, s.effectiveSchema(s.schema, referenced), referenced.Name)
		if err != nil {
			return false, false, err
		}
	}
	switch {
	case parentEntry.state == tableAbsent:
		return true, true, nil
	case parentEntry.state != tablePresent || parentEntry.shape == nil:
		return false, false, nil
	}
	if parentEntry.shape.Columns == nil || constraint.UnmodeledReferencedParts > 0 || len(constraint.ReferencedColumns) == 0 {
		return false, false, nil
	}
	for _, column := range constraint.ReferencedColumns {
		if parentEntry.shape.FindColumn(column) == nil {
			return true, true, nil
		}
	}
	return false, true, nil
}

// publishForeignKeySupportingIndex mirrors the native implicit-index rule:
// an added foreign key gets a supporting index only when no existing member
// or primary object already covers its local columns as a leftmost prefix.
// The derived name follows the dialect's native rule — a named constraint
// lends its name on both anchors; a bare MySQL add takes the first local
// column (a taken name is a server dedup this model does not guess, so the
// member set unvouches); a bare TiDB add pairs the server-derived fk_<n>
// constraint name with an equally server-derived index name, publishing the
// member with its name unproven rather than inventing a numeral. Coverage
// or derived-name provability failing unvouches the member set — a vouched
// member list that may be missing the implicit member is a fabricated
// completeness claim.
func (s *batchState) publishForeignKeySupportingIndex(shape *spec.TableSnapshot, member spec.Constraint, authored string) {
	if shape.IndexesUnknown {
		return
	}
	if len(member.Columns) == 0 || member.UnmodeledParts > 0 {
		shape.IndexesUnknown = true
		return
	}
	covered, provable := foreignKeyColumnsCovered(shape, member.Columns, -1)
	if !provable {
		shape.IndexesUnknown = true
		return
	}
	if covered {
		return
	}
	if authored == "" && s.dialect == spec.DialectTiDB {
		// TiDB derives an fk_<n> name for a bare foreign key and gives the
		// implicit index the same derived name — unprovable either way, so
		// the member publishes with its name unproven instead of a guessed
		// numeral.
		shape.Indexes = append(shape.Indexes, spec.Index{
			Kind:        spec.IndexKindSecondary,
			Columns:     append([]string(nil), member.Columns...),
			NameUnknown: true,
		})
		return
	}
	derived := authored
	if derived == "" {
		derived = member.Columns[0]
	}
	if shape.IndexNameSpaceUnvouched() || shape.FindProvenIndex(derived) != nil {
		// The derived name may collide with a hidden member and the server's
		// fallback naming is unmodeled — membership stays unvouched.
		shape.IndexesUnknown = true
		return
	}
	if pkName, proven := spec.PrimaryKeyObjectName(shape); proven && strings.EqualFold(pkName, derived) {
		shape.IndexesUnknown = true
		return
	}
	shape.Indexes = append(shape.Indexes, spec.Index{
		Name:    derived,
		Kind:    spec.IndexKindSecondary,
		Columns: append([]string(nil), member.Columns...),
	})
}

// applyDropForeignKey publishes a bounded drop-foreign-key transition. A
// proven member name removes only the constraint row — the supporting
// index, explicit or implicit, survives natively. A proven-missing name is
// the native 1091 refusal and a provable no-op; an unvouched name space
// leaves the outcome unprovable and unvouches the constraint set rather
// than keeping a member that may have been dropped.
func (s *batchState) applyDropForeignKey(_ spec.Statement, alter *spec.Alter, targets []spec.Table) {
	name := ""
	if alter != nil {
		name = strings.TrimSpace(alter.Name)
	}
	if len(targets) != 1 || name == "" {
		s.invalidateAll(targets)
		return
	}
	key := s.keyFor(s.schema, targets[0])
	entry := s.entries[key]
	switch {
	case entry == nil || entry.state == tableUnknown:
		s.entries[key] = presentIncompleteEntry(s.effectiveSchema(s.schema, targets[0]), targets[0].Name)
		return
	case entry.state == tableAbsent:
		s.invalidateKey(key)
		return
	}
	if entry.shape == nil {
		return
	}
	if entry.shape.FindForeignKeyConstraint(name) == nil {
		if entry.shape.ForeignKeyNameSpaceUnvouched() {
			// The dropped name may have bound a hidden member — the
			// surviving member set is unprovable.
			s.markConstraintsUnknown(key)
		}
		// A proven miss is the native refusal: provably unchanged.
		return
	}
	shape := cloneTableSnapshot(entry.shape)
	kept := shape.Constraints[:0]
	for i := range shape.Constraints {
		constraint := &shape.Constraints[i]
		if constraint.Type == "foreign_key" && !constraint.NameUnknown && strings.EqualFold(constraint.Name, name) {
			continue
		}
		kept = append(kept, *constraint)
	}
	shape.Constraints = kept
	s.entries[key] = &batchTableEntry{state: tablePresent, shape: shape, displaySchema: entry.displaySchema, displayTable: entry.displayTable}
}

// foreignKeyColumnsCovered reports whether the given columns are a leftmost
// prefix of some index member or the primary object — the native supporting
// index test. excludeIndex skips one member position (the member a drop is
// about to remove). A member carrying unmodeled key parts cannot answer
// coverage, so the second return value reports provability.
func foreignKeyColumnsCovered(shape *spec.TableSnapshot, columns []string, excludeIndex int) (covered bool, provable bool) {
	unprovable := false
	for i := range shape.Indexes {
		if i == excludeIndex {
			continue
		}
		memberCovered, memberProvable := indexCoversColumns(shape.Indexes[i], columns)
		if memberCovered {
			return true, true
		}
		if !memberProvable {
			unprovable = true
		}
	}
	if shape.PrimaryKey != nil {
		if memberCovered, memberProvable := indexCoversColumns(*shape.PrimaryKey, columns); memberCovered {
			return true, true
		} else if !memberProvable {
			unprovable = true
		}
	}
	return false, !unprovable
}

// indexCoversColumns reports whether the index's plain stored columns have
// the given columns as an in-order leftmost prefix. Unmodeled key parts
// (prefix lengths, expressions, descending parts) leave coverage
// unanswerable.
func indexCoversColumns(index spec.Index, columns []string) (covered bool, provable bool) {
	if index.PrefixParts > 0 || index.DescParts > 0 || index.ExpressionCount > 0 ||
		index.HasExpressionKeys || index.UnmodeledKeyParts > 0 {
		return false, false
	}
	if len(columns) == 0 || len(index.Columns) < len(columns) {
		return false, true
	}
	for i, column := range columns {
		if !strings.EqualFold(index.Columns[i], column) {
			return false, true
		}
	}
	return true, true
}

// foreignKeyIndexDependency reports whether the member at indexPos is the
// sole covering index for some proven foreign key — the native 1553
// refusal. The second return value is false when constraint or member
// facts leave the dependency unanswerable, which the caller tombstones.
func foreignKeyIndexDependency(shape *spec.TableSnapshot, indexPos int) (needed bool, provable bool) {
	if shape == nil || shape.ForeignKeyNameSpaceUnvouched() {
		return false, false
	}
	for i := range shape.Constraints {
		constraint := &shape.Constraints[i]
		if constraint.Type != "foreign_key" {
			continue
		}
		if len(constraint.Columns) == 0 || constraint.UnmodeledParts > 0 {
			return false, false
		}
		memberCovered, memberProvable := indexCoversColumns(shape.Indexes[indexPos], constraint.Columns)
		if !memberProvable {
			return false, false
		}
		if !memberCovered {
			continue
		}
		other, otherProvable := foreignKeyColumnsCovered(shape, constraint.Columns, indexPos)
		if !otherProvable {
			return false, false
		}
		if !other {
			return true, true
		}
	}
	return false, true
}
