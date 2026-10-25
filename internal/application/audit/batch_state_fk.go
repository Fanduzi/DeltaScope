// Package audit publishes bounded foreign-key ordered-state transitions.
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
	// Refer-action landing is a statement-level structural premise checked
	// before any reference resolution: a proven refusal fails the statement
	// before the table is touched, while an undecidable one (an unproven
	// version against a version-gated action) may still have landed the
	// member — unvouch, and propagate the possible implicit index.
	refused, provable := s.foreignKeyReferActionPremise(entry.shape.Columns, constraint)
	if refused {
		return nil
	}
	if !provable {
		s.markConstraintsUnknown(key)
		s.propagateForeignKeyIndexUncertainty(key, constraint)
		return nil
	}
	refused, decided, err := s.foreignKeyReferencedPremise(ctx, key, entry, constraint)
	if err != nil {
		return err
	}
	if !decided {
		s.markConstraintsUnknown(key)
		s.propagateForeignKeyIndexUncertainty(key, constraint)
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
			s.propagateForeignKeyIndexUncertainty(key, constraint)
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
	if !shape.ConstraintsUnknown {
		// A nil Constraints is a loaded empty set per the collection
		// contract — appending to it stays vouched.
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
		// One effective-identity resolution serves the cache key, the
		// provider read, and the display identity alike: an explicitly
		// qualified parent keeps its authored schema, an unqualified
		// reference resolves inside the child's effective schema (the
		// server's own resolution rule), never the request schema.
		parentEntry, err = s.resolve(ctx, parentKey, s.effectiveSchema(key.schema, referenced), referenced.Name)
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
	// Column presence alone does not land the key: the parent must carry a
	// usable referenced key — a member or primary object covering the
	// referenced columns leftmost in order and passing the product's
	// eligibility class (a plain btree on every anchor; non-unique only
	// where the product/version allows it: refused natively as 1215 on
	// 5.7, 1822 on 8.0 and TiDB v8.5, 6125 on 8.4-restrict-ON). A proven
	// absence is a refusal; an unprovable answer unvouches rather than
	// publishing a member the statement may never have landed.
	usable, provable := s.referencedKeyPremise(parentEntry.shape, constraint.ReferencedColumns)
	if !provable {
		return false, false, nil
	}
	if !usable {
		return true, true, nil
	}
	return false, true, nil
}

// createTableForeignKeyPremise answers the CREATE TABLE landing premise
// for its declared foreign keys: each member's local columns must exist
// among the authored columns and pass the refer-action gate, and each
// referenced parent must carry a usable key — the same classes the
// ALTER-side premise enforces (missing member/column/key → refused;
// unread facts → undecidable). A self-reference resolves against the
// derived shape being created. refused means the statement fails before
// the table lands; decided=false means the landing is unprovable.
func (s *batchState) createTableForeignKeyPremise(ctx context.Context, key batchTableKey, displaySchema, displayTable string, ddl *spec.DDL) (refused bool, decided bool, err error) {
	var derived *spec.TableSnapshot
	for i := range ddl.Constraints {
		constraint := &ddl.Constraints[i]
		if constraint.Type != "foreign_key" {
			continue
		}
		for _, column := range constraint.Columns {
			found := false
			for j := range ddl.Columns {
				if strings.EqualFold(ddl.Columns[j].Name, column) {
					found = true
					break
				}
			}
			if !found {
				return true, true, nil
			}
		}
		if refused, provable := s.foreignKeyReferActionPremise(ddl.Columns, constraint); refused {
			return true, true, nil
		} else if !provable {
			return false, false, nil
		}
		referenced := spec.Table{Schema: constraint.ReferencedSchema, Name: strings.TrimSpace(constraint.ReferencedTable)}
		if referenced.Name == "" {
			continue
		}
		var parentEntry *batchTableEntry
		if s.keyFor(key.schema, referenced) == key {
			if derived == nil {
				derived = s.derivedCreateShape(displaySchema, displayTable, ddl)
			}
			parentEntry = &batchTableEntry{state: tablePresent, shape: derived}
		} else {
			parentEntry, err = s.resolve(ctx, s.keyFor(key.schema, referenced), s.effectiveSchema(key.schema, referenced), referenced.Name)
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
		usable, provable := s.referencedKeyPremise(parentEntry.shape, constraint.ReferencedColumns)
		if !provable {
			return false, false, nil
		}
		if !usable {
			return true, true, nil
		}
	}
	return false, true, nil
}

// referencedKeyPremise reports whether the parent's member set carries a
// usable referenced key for the columns — one eligible member or the
// primary object covering them leftmost in order. Unknown collections or
// unclassifiable candidates answer unprovable rather than fabricating
// either absence or usability.
func (s *batchState) referencedKeyPremise(shape *spec.TableSnapshot, referencedColumns []string) (usable bool, provable bool) {
	return s.referencedAlternativeCoverer(shape, -1, false, referencedColumns, false)
}

// referencedAlternativeCoverer reports whether the shape still carries an
// eligible referenced-key coverer once the drop target (member indexPos,
// or the primary object when dropPK) is gone. Shared by the add premise
// (indexPos=-1, forBinding=false: does any usable key exist for a NEW
// reference — the creatability question the restrict flag gates) and the
// parent-side drop dependency (forBinding=true: would the surviving set
// still back a loaded child's foreign key — the native 1553 check, which
// is a physical-binding question the current flag cannot retroactively
// change).
func (s *batchState) referencedAlternativeCoverer(shape *spec.TableSnapshot, indexPos int, dropPK bool, referencedColumns []string, forBinding bool) (covered bool, provable bool) {
	if shape == nil {
		return false, false
	}
	unprovable := false
	if !dropPK && shape.PrimaryKey != nil {
		if covered, provable := s.referencedKeyMatch(*shape.PrimaryKey, referencedColumns, forBinding); covered {
			return true, true
		} else if !provable {
			unprovable = true
		}
	}
	for i := range shape.Indexes {
		if i == indexPos {
			continue
		}
		var eligible, eligibleProvable bool
		if forBinding {
			eligible, eligibleProvable = supportingIndexCandidate(shape.Indexes[i].Kind)
		} else {
			eligible, eligibleProvable = s.referencedKeyMemberEligible(shape.Indexes[i].Kind)
		}
		if !eligibleProvable {
			unprovable = true
			continue
		}
		if !eligible {
			continue
		}
		if covered, provable := s.referencedKeyMatch(shape.Indexes[i], referencedColumns, forBinding); covered {
			return true, true
		} else if !provable {
			unprovable = true
		}
	}
	if shape.IndexesUnknown {
		// A hidden member may carry the usable key — absence is unproven.
		unprovable = true
	}
	if !dropPK && shape.PrimaryKeyUnknown && shape.PrimaryKey == nil {
		// An unproven primary object may be the usable key.
		unprovable = true
	}
	return false, !unprovable
}

// referencedKeyMatch reports whether the index can serve the referenced
// columns. An existing binding needs only the physical leftmost-prefix
// coverage (TiDB accepts partial references for new keys too — probed
// v8.5.0). For a NEW reference on MySQL the strict anchor adds the
// complete-key rule: under observed restrict_fk_on_non_standard_key=ON
// the columns must equal the member's entire key — a partial unique or
// partial primary reference refuses with 6125 (probed on 8.4.10). An
// unobserved flag on a version that could run 8.4+ leaves a
// prefix-only match undecidable; an exact match is safe under either
// arm, and a proven pre-8.4 version keeps the plain prefix rule.
func (s *batchState) referencedKeyMatch(index spec.Index, referencedColumns []string, forBinding bool) (covered bool, provable bool) {
	if forBinding || s.dialect != spec.DialectMySQL {
		return indexCoversColumns(index, referencedColumns)
	}
	prefixCovered, prefixProvable := indexCoversColumns(index, referencedColumns)
	if !prefixCovered || !prefixProvable {
		return prefixCovered, prefixProvable
	}
	if s.instanceFacts != nil && s.instanceFacts.RestrictNonStandardKeyKnown {
		if !s.instanceFacts.RestrictNonStandardKey {
			return true, true
		}
		return indexMatchesColumnsExactly(index, referencedColumns)
	}
	if exact, exactProvable := indexMatchesColumnsExactly(index, referencedColumns); exact {
		return true, true
	} else if !exactProvable {
		return false, false
	}
	version := s.resolvedVersion
	if version == nil || version.Major == 0 {
		return false, false
	}
	if version.Major > 8 || (version.Major == 8 && version.Minor >= 4) {
		return false, false
	}
	return true, true
}

// indexMatchesColumnsExactly reports whether the index's plain stored
// columns equal the given columns as a complete ordered key — the
// strict-anchor referenced-key test. Unmodeled key parts leave the
// answer unanswerable, same as coverage.
func indexMatchesColumnsExactly(index spec.Index, columns []string) (matched bool, provable bool) {
	if index.PrefixParts > 0 || index.DescParts > 0 || index.ExpressionCount > 0 ||
		index.HasExpressionKeys || index.UnmodeledKeyParts > 0 {
		return false, false
	}
	if len(columns) == 0 || len(index.Columns) != len(columns) {
		return false, true
	}
	for i, column := range columns {
		if !strings.EqualFold(index.Columns[i], column) {
			return false, true
		}
	}
	return true, true
}

// referencedKeyMemberEligible reports whether a member kind can serve as a
// referenced key. Primary and unique qualify on every anchor; a plain
// secondary member qualifies only where the product allows non-standard
// keys (TiDB and pre-8.4 MySQL accept; MySQL 8.4+ under
// restrict_fk_on_non_standard_key refuses with 6125). Specialized kinds
// provably cannot answer; an unproven kind leaves the answer unprovable.
func (s *batchState) referencedKeyMemberEligible(kind spec.IndexKind) (eligible bool, provable bool) {
	switch kind {
	case spec.IndexKindPrimary, spec.IndexKindUnique:
		return true, true
	case spec.IndexKindSecondary:
		return s.secondaryReferencedKeyEligible()
	case spec.IndexKindUnknown, "":
		return false, false
	default:
		return false, true
	}
}

// secondaryReferencedKeyEligible resolves the non-unique referenced-key
// question. TiDB accepts them (v8.5 probe); MySQL delegates to the
// observed restrict_fk_on_non_standard_key — absent before 8.4 so a
// proven older version means unrestricted, while an unread variable on
// 8.4+ or an unproven version leaves the answer unprovable.
func (s *batchState) secondaryReferencedKeyEligible() (eligible bool, provable bool) {
	if s.dialect == spec.DialectTiDB {
		return true, true
	}
	if s.instanceFacts != nil && s.instanceFacts.RestrictNonStandardKeyKnown {
		return !s.instanceFacts.RestrictNonStandardKey, true
	}
	version := s.resolvedVersion
	if version == nil || version.Major == 0 {
		return false, false
	}
	if version.Major > 8 || (version.Major == 8 && version.Minor >= 4) {
		return false, false
	}
	return true, true
}

// foreignKeyReferActionPremise answers the statement-local refer-action
// premise on the already-proven local column set: SET NULL against a
// proven NOT NULL member column is refused on every anchor (MySQL 1215 on
// 5.7, 1830 on 8.x; TiDB 1830 on v8.5). SET DEFAULT is refused by pre-8.0
// MySQL (1215) and accepted by 8.x and TiDB v8.5, so an unproven MySQL
// version leaves it undecidable. The second return is false when the
// landing cannot be decided — the caller unvouches rather than publishes.
func (s *batchState) foreignKeyReferActionPremise(columns []spec.Column, constraint *spec.Constraint) (refused bool, provable bool) {
	if constraint == nil {
		return false, true
	}
	if constraint.OnDelete == "set_null" || constraint.OnUpdate == "set_null" {
		for _, name := range constraint.Columns {
			for i := range columns {
				if strings.EqualFold(columns[i].Name, name) && columns[i].NotNull {
					return true, true
				}
			}
		}
	}
	if constraint.OnDelete == "set_default" || constraint.OnUpdate == "set_default" {
		if s.dialect == spec.DialectMySQL {
			version := s.resolvedVersion
			if version == nil || version.Major == 0 {
				return false, false
			}
			if version.Major < 8 {
				return true, true
			}
		}
	}
	return false, true
}

// firstDropCheckAlter returns the sole drop_check action when the
// statement is exactly one DROP CHECK|CONSTRAINT — the parsed AST shape is
// shared by both keywords (CheckConstraintKeyword), so the authored
// keyword is never recoverable and the model answers only what the
// proven target type decides.
func firstDropCheckAlter(ddl *spec.DDL) *spec.Alter {
	if ddl == nil || len(ddl.Alter) != 1 {
		return nil
	}
	alter := &ddl.Alter[0]
	if alter.Action != "drop_check" {
		return nil
	}
	return alter
}

// applyDropCheck publishes the bounded post-state of a DROP CHECK|CONSTRAINT
// — an action whose two authored keywords share one AST node, so the model
// separates them by what the proven target can be.
//
// TiDB never removes a foreign key this way (v8.5.0 probe: warned no-op
// under tidb_enable_check_constraint=OFF, error 3940 under ON — the member
// survives either way) and a hidden CHECK it could remove is never a
// modeled member, so the transition has no modeled effect at all.
//
// MySQL 5.7 rejects the syntax outright (1064) — provably unchanged.
// MySQL 8.x removes every constraint kind under DROP CONSTRAINT — a
// foreign-key member, a unique member, the primary-key object under its
// stored PRIMARY name, and a real CHECK — while DROP CHECK removes only
// a CHECK and refuses the rest (3821, 3940 on a secondary member or a
// missing name; all probed on 8.0.46 and 8.4.10). When the authored name
// binds a proven removable target the two keywords diverge and the
// outcome is unprovable, so the affected collection unvouches — a proven
// match would fabricate survival, a vouched absent member would
// fabricate removal. A non-unique member refuses under both keywords
// and keeps its proven state; an unmatched name on a vouched shape
// refuses under both; a hidden CHECK under an unvouched constraint set
// leaves the modeled members unchanged either way.
func (s *batchState) applyDropCheck(_ spec.Statement, alter *spec.Alter, targets []spec.Table) {
	if len(targets) != 1 {
		s.invalidateAll(targets)
		return
	}
	key := s.keyFor(s.schema, targets[0])
	entry := s.entries[key]
	if entry == nil || entry.state != tablePresent || entry.shape == nil {
		// Unknown and absent targets keep their state: the statement's
		// modeled effect is nil on every dialect, and existence was never
		// proven either way.
		return
	}
	if s.dialect != spec.DialectMySQL {
		return
	}
	name := strings.TrimSpace(alter.Name)
	if name == "" {
		return
	}
	version := s.resolvedVersion
	if version != nil && version.Major != 0 && version.Major < 8 {
		// Pre-8.0 rejects the syntax — provably unchanged.
		return
	}
	bound := false
	if pkName, proven := spec.PrimaryKeyObjectName(entry.shape); proven && strings.EqualFold(pkName, name) {
		// The PK object drops under DROP CONSTRAINT and refuses under
		// DROP CHECK — its fate is unprovable.
		s.unvouchPrimaryKeyObject(key)
		bound = true
	} else if entry.shape.PrimaryKey != nil && strings.EqualFold(name, "primary") {
		// The MySQL stored name is always PRIMARY; an unproven
		// attribution still spells the object.
		s.unvouchPrimaryKeyObject(key)
		bound = true
	}
	if member := entry.shape.FindProvenIndex(name); member != nil {
		bound = true
		if member.Kind == spec.IndexKindUnique {
			// A unique member drops under DROP CONSTRAINT, refuses
			// under DROP CHECK — divergent like the FK arm.
			s.unvouchIndexMember(key, name)
		}
		// Any other proven member kind provably survives both keywords.
	}
	if entry.shape.FindForeignKeyConstraint(name) != nil {
		// DROP CONSTRAINT removes this member; DROP CHECK refuses on it.
		// The authored keyword is unrecoverable, so the member's fate is
		// unprovable: record it gone and unvouch the collection — a
		// proven match would fabricate survival, a vouched absent set
		// would fabricate removal.
		s.unvouchForeignKeyMember(key, name)
		return
	}
	// An unvouched member name may hide a removable target — a hidden FK
	// or a hidden unique member the constraint form would remove. The
	// already-unvouched sets stay unvouched; a vouched set with no proven
	// match is a refusal under both keywords and keeps its facts.
	if !bound {
		if entry.shape.ForeignKeyNameSpaceUnvouched() {
			s.markConstraintsUnknown(key)
		}
		if entry.shape.IndexNameSpaceUnvouched() {
			s.markIndexesUnknown(key)
		}
	}
}

// unvouchForeignKeyMember drops the named member's projection and
// unvouches the collection — the shape a transition leaves when the
// member may have been removed or kept and the model cannot decide. Its
// absence is recorded without vouching for it, so name-keyed readers owe
// the collection gap rather than answering either way.
func (s *batchState) unvouchForeignKeyMember(key batchTableKey, name string) {
	s.withClonedShape(key, func(shape *spec.TableSnapshot) {
		kept := shape.Constraints[:0]
		for i := range shape.Constraints {
			constraint := &shape.Constraints[i]
			if constraint.Type == "foreign_key" && !constraint.NameUnknown && strings.EqualFold(constraint.Name, name) {
				continue
			}
			kept = append(kept, *constraint)
		}
		shape.Constraints = kept
		shape.ConstraintsUnknown = true
	})
}

// unvouchIndexMember drops the named member's projection and unvouches
// the index collection — the same undecidable-fate shape
// unvouchForeignKeyMember records for the constraint space. Presence-keyed
// readers owe the set gap rather than answering either way.
func (s *batchState) unvouchIndexMember(key batchTableKey, name string) {
	s.withClonedShape(key, func(shape *spec.TableSnapshot) {
		kept := shape.Indexes[:0]
		for i := range shape.Indexes {
			member := &shape.Indexes[i]
			if !member.NameUnknown && strings.EqualFold(member.Name, name) {
				continue
			}
			kept = append(kept, *member)
		}
		shape.Indexes = kept
		shape.IndexesUnknown = true
	})
}

// unvouchPrimaryKeyObject clears the primary-key projection and marks its
// presence unproven — the shape a possibly-dropped, possibly-kept object
// leaves when the model cannot decide. The stored-name and handle facts
// stay as last-known attributes; presence-keyed consumers answer only
// through PrimaryKeyUnknown.
func (s *batchState) unvouchPrimaryKeyObject(key batchTableKey) {
	s.withClonedShape(key, func(shape *spec.TableSnapshot) {
		shape.PrimaryKey = nil
		shape.PrimaryKeyUnknown = true
	})
}

// propagateForeignKeyIndexUncertainty unvouches the index set when an
// undecidable add may still have landed its implicit supporting index —
// the same possible change markConstraintsUnknown records for the member
// side. A provably covered member set would reuse the existing index and
// stays vouched; facts that already carry IndexesUnknown need nothing.
func (s *batchState) propagateForeignKeyIndexUncertainty(key batchTableKey, constraint *spec.Constraint) {
	entry := s.entries[key]
	if entry == nil || entry.shape == nil || entry.shape.IndexesUnknown || constraint == nil {
		return
	}
	if len(constraint.Columns) == 0 || constraint.UnmodeledParts > 0 {
		s.markIndexesUnknown(key)
		return
	}
	covered, provable := foreignKeyColumnsCovered(entry.shape, constraint.Columns, -1, false)
	if !provable || !covered {
		s.markIndexesUnknown(key)
	}
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
	covered, provable := foreignKeyColumnsCovered(shape, member.Columns, -1, false)
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
// prefix of some eligible index member or the primary object — the native
// supporting index test. excludeIndex skips one member position (the member
// a drop is about to remove); excludePK excludes the primary object (the
// drop-PK dependency asks what survives without it). Only kinds that can
// back a foreign key count — specialized members (fulltext, spatial,
// vector, columnar) provably cannot serve as a supporting index, while an
// unproven kind or unmodeled key parts cannot answer coverage, so the
// second return value reports provability. Unvouched member collections
// leave absence unprovable the same way.
func foreignKeyColumnsCovered(shape *spec.TableSnapshot, columns []string, excludeIndex int, excludePK bool) (covered bool, provable bool) {
	unprovable := false
	for i := range shape.Indexes {
		if i == excludeIndex {
			continue
		}
		eligible, eligibleProvable := supportingIndexCandidate(shape.Indexes[i].Kind)
		if !eligibleProvable {
			unprovable = true
			continue
		}
		if !eligible {
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
	if !excludePK && shape.PrimaryKey != nil {
		if memberCovered, memberProvable := indexCoversColumns(*shape.PrimaryKey, columns); memberCovered {
			return true, true
		} else if !memberProvable {
			unprovable = true
		}
	}
	if shape.IndexesUnknown || (!excludePK && shape.PrimaryKeyUnknown) {
		// A hidden member or unproven primary object may cover — absence
		// is unproven rather than fabricated.
		unprovable = true
	}
	return false, !unprovable
}

// supportingIndexCandidate reports whether a member kind can serve as a
// foreign key's supporting index — plain btree kinds (secondary, unique,
// and the attribution-withheld primary placeholder member) qualify on
// every anchor; specialized kinds provably cannot; an unproven kind
// cannot answer.
func supportingIndexCandidate(kind spec.IndexKind) (eligible bool, provable bool) {
	switch kind {
	case spec.IndexKindSecondary, spec.IndexKindUnique, spec.IndexKindPrimary:
		return true, true
	case spec.IndexKindUnknown, "":
		return false, false
	default:
		return false, true
	}
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
// A member that could never back a foreign key (a specialized kind) is a
// provable non-dependency; one whose eligibility cannot be read is
// unprovable.
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
		// The member covers this key's columns — only its support
		// eligibility decides whether it is the bound index now; a
		// specialized member covers positionally without ever backing a
		// key, and an unproven kind cannot answer.
		eligible, eligibleProvable := supportingIndexCandidate(shape.Indexes[indexPos].Kind)
		if !eligibleProvable {
			return false, false
		}
		if !eligible {
			continue
		}
		other, otherProvable := foreignKeyColumnsCovered(shape, constraint.Columns, indexPos, false)
		if !otherProvable {
			return false, false
		}
		if !other {
			return true, true
		}
	}
	return false, true
}

// foreignKeyPKObjectDependency reports whether the primary object is the
// sole covering index for some proven foreign key — the same native 1553
// refusal class that blocks DROP PRIMARY KEY (verified 8.0.46) and the
// PK-object drop reached under a renamed stored name. The caller invokes
// it only when the object is proven present.
func foreignKeyPKObjectDependency(shape *spec.TableSnapshot) (needed bool, provable bool) {
	if shape == nil || shape.PrimaryKey == nil || shape.ForeignKeyNameSpaceUnvouched() {
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
		memberCovered, memberProvable := indexCoversColumns(*shape.PrimaryKey, constraint.Columns)
		if !memberProvable {
			return false, false
		}
		if !memberCovered {
			continue
		}
		other, otherProvable := foreignKeyColumnsCovered(shape, constraint.Columns, -1, true)
		if !otherProvable {
			return false, false
		}
		if !other {
			return true, true
		}
	}
	return false, true
}

// referencedDropDependency reports whether the drop target on the parent
// (member indexPos, or the primary object when dropPK) is the sole
// eligible referenced key backing a foreign key declared on another
// already-loaded table — the parent-side arm of the native 1553 refusal.
// A loaded child whose constraint space is unvouched, or whose member
// shapes cannot answer coverage, leaves the answer unprovable.
func (s *batchState) referencedDropDependency(key batchTableKey, shape *spec.TableSnapshot, indexPos int, dropPK bool) (needed bool, provable bool) {
	for otherKey, other := range s.entries {
		if other == nil || other.state != tablePresent || other.shape == nil {
			continue
		}
		if other.shape.ConstraintsUnknown {
			// A hidden member may bind this table — its surviving key
			// need cannot be ruled out.
			return false, false
		}
		for i := range other.shape.Constraints {
			constraint := &other.shape.Constraints[i]
			if constraint.Type != "foreign_key" {
				continue
			}
			referenced := spec.Table{Schema: constraint.ReferencedSchema, Name: constraint.ReferencedTable}
			if strings.TrimSpace(referenced.Name) == "" || s.keyFor(otherKey.schema, referenced) != key {
				continue
			}
			if len(constraint.ReferencedColumns) == 0 || constraint.UnmodeledReferencedParts > 0 {
				return false, false
			}
			var targetCovers, targetProvable bool
			if dropPK {
				if shape.PrimaryKey != nil {
					targetCovers, targetProvable = indexCoversColumns(*shape.PrimaryKey, constraint.ReferencedColumns)
				} else {
					targetCovers, targetProvable = false, true
				}
			} else {
				targetCovers, targetProvable = indexCoversColumns(shape.Indexes[indexPos], constraint.ReferencedColumns)
				if targetCovers && targetProvable {
					// Only a member covering this key's referenced
					// columns can be its bound key — physical backing is
					// the binding question (a loaded relationship keeps
					// binding a non-standard member even where a new
					// equivalent reference would refuse now).
					eligible, eligibleProvable := supportingIndexCandidate(shape.Indexes[indexPos].Kind)
					if !eligibleProvable {
						return false, false
					}
					if !eligible {
						targetCovers = false
					}
				}
			}
			if !targetProvable {
				return false, false
			}
			if !targetCovers {
				continue
			}
			remains, remainsProvable := s.referencedAlternativeCoverer(shape, indexPos, dropPK, constraint.ReferencedColumns, true)
			if !remainsProvable {
				return false, false
			}
			if !remains {
				return true, true
			}
		}
	}
	return false, true
}
