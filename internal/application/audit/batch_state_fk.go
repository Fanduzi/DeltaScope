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

// applyDropCheck publishes the bounded post-state of a DROP
// CHECK|CONSTRAINT. The extractor recovers the authored keyword
// (alter.Options["check_spelling"]) so the two spellings bind their own
// namespaces instead of an unrecoverable union guess.
//
// MySQL (probed 8.0.46/8.4.10): DROP CHECK removes only a CHECK member —
// a non-check target or missing name refuses 3821, unchanged. DROP
// CONSTRAINT removes the single member its generic namespace {check,
// unique, pk, fk} binds — each kind probed rc=0 — while zero binds refuse
// 3940 and a cross-type same-name set refuses 3939, both unchanged.
// MySQL gates the keywords separately (CHECK 8.0.16, generic 8.0.19; the
// 8.0.17 band anchor probed CHECK executes while CONSTRAINT refuses
// 1064). An unproven spelling or execution gate diverges member-by-
// member: a lone CHECK member drops under either keyword; a lone
// non-check member drops under generic and survives under CHECK; a
// cross-type set leaves only the CHECK member's fate undecidable.
//
// TiDB v8.5.0 flag-ON binds the CHECK member first under either keyword
// and never detects the cross-type ambiguity (probed); non-check-only
// bindings are unprobed and stay undecidable. Flag OFF is a parse-only
// no-op; an unobserved flag leaves the member fate undecidable.
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
	name := strings.TrimSpace(alter.Name)
	if name == "" {
		return
	}
	spelling := alter.Options["check_spelling"]
	if s.dialect == spec.DialectTiDB {
		switch s.checkArmFor() {
		case spec.CheckArmEnforced:
			res := spec.CheckDropTarget(s.dialect, entry.shape, spelling, name)
			switch res.Binding {
			case spec.CheckTargetProven:
				s.removeCheckMember(key, entry.shape, res.Constraint)
			case spec.CheckTargetMissing, spec.CheckTargetWrongKind, spec.CheckTargetAmbiguous:
				return
			default:
				// Non-check-only or unvouched binding — unprobed fate.
				s.markConstraintsUnknown(key)
			}
		case spec.CheckArmUnknown:
			if entry.shape.FindCheckConstraint(name) != nil {
				s.markConstraintsUnknown(key)
			}
		}
		return
	}
	if s.dialect != spec.DialectMySQL {
		return
	}
	// The syntax gate runs before binding: a refused spelling never
	// reaches the server (provably unchanged); an unproven one leaves
	// every member it could have removed undecidable.
	syntaxExecutable := true
	switch spec.CheckAlterSyntaxOutcome(s.dialect, s.resolvedVersion, spelling) {
	case spec.CheckSyntaxRefused:
		return
	case spec.CheckSyntaxUnproven:
		syntaxExecutable = false
	}
	res := spec.CheckDropTarget(s.dialect, entry.shape, spelling, name)
	if !syntaxExecutable {
		s.dropCheckUndecidable(key, entry, name, spelling)
		return
	}
	switch res.Binding {
	case spec.CheckTargetMissing, spec.CheckTargetWrongKind, spec.CheckTargetAmbiguous:
		// Proven refusal — unchanged (3821/3940/3939).
		return
	case spec.CheckTargetUnproven:
		s.dropCheckUndecidable(key, entry, name, spelling)
		return
	}
	s.publishCheckDropTarget(key, entry, res)
}

// publishCheckDropTarget lands a provably-bound DROP removal: the check or
// foreign-key member leaves Constraints, a unique index member leaves
// Indexes, and the primary-key object or primary index member publishes
// the shared PK-drop post-state. Binding only proves which object the
// name reaches — the same foreign-key dependencies that gate DROP INDEX
// and DROP PRIMARY KEY still refuse a sole-covering target (1553, probed
// under the generic spelling on 8.0.46), and an unanswerable dependency
// keeps the outcome unproven rather than deleting anyway.
func (s *batchState) publishCheckDropTarget(key batchTableKey, entry *batchTableEntry, res spec.CheckTarget) {
	switch res.Kind {
	case spec.CheckTargetKindCheck, spec.CheckTargetKindForeignKey:
		s.removeCheckMember(key, entry.shape, res.Constraint)
	case spec.CheckTargetKindUnique, spec.CheckTargetKindPrimary:
		if res.PKObject {
			needed, provable := s.foreignKeyPKDependency(key, entry.shape)
			if !provable {
				s.invalidateKey(key)
				return
			}
			if needed {
				return
			}
			s.publishPrimaryKeyDrop(key, entry)
			return
		}
		if res.Index == nil {
			return
		}
		indexPos, matches := provenIndexMatch(entry.shape.Indexes, res.Index.Name)
		if matches != 1 || indexPos < 0 {
			s.invalidateKey(key)
			return
		}
		needed, provable := s.foreignKeyMemberDependency(key, entry.shape, indexPos)
		if !provable {
			s.invalidateKey(key)
			return
		}
		if needed {
			return
		}
		s.dropIndexMemberAt(key, entry, indexPos)
	}
}

// removeCheckMember drops a constraint member by identity — the shared
// deterministic-removal publish for CHECK and foreign-key members.
func (s *batchState) removeCheckMember(key batchTableKey, shape *spec.TableSnapshot, member *spec.Constraint) {
	if member == nil {
		return
	}
	idx := memberIndex(shape, member)
	if idx < 0 {
		return
	}
	s.withClonedShape(key, func(cloned *spec.TableSnapshot) {
		cloned.Constraints = append(cloned.Constraints[:idx], cloned.Constraints[idx+1:]...)
	})
}

// dropCheckUndecidable publishes the honest undecidable post-state when
// the statement's execution or authored spelling cannot be proven: every
// member a surviving reading could remove unvouches at member
// granularity; a consulted-but-unvouched name space unvouches its
// collection. Members that refuse under every reading keep their facts.
func (s *batchState) dropCheckUndecidable(key batchTableKey, entry *batchTableEntry, name, spelling string) {
	shape := entry.shape
	if spelling == spec.CheckSpellingCheck {
		// Only the CHECK name space is consulted.
		if shape.CheckNameSpaceUnvouched() {
			s.markConstraintsUnknown(key)
			return
		}
		if member := shape.FindCheckConstraint(name); member != nil {
			// Executes → removed; refuses → kept.
			s.markConstraintsUnknown(key)
		}
		return
	}
	// Generic or unproven spelling consults every collection.
	unvouched := shape.CheckNameSpaceUnvouched() ||
		shape.ForeignKeyNameSpaceUnvouched() ||
		shape.IndexNameSpaceUnvouched()
	if unvouched {
		s.markConstraintsUnknown(key)
		s.markIndexesUnknown(key)
		return
	}
	checkFound := shape.FindCheckConstraint(name) != nil
	fkFound := shape.FindForeignKeyConstraint(name) != nil
	var indexMember *spec.Index
	indexKindUnproven := false
	if member := shape.FindProvenIndex(name); member != nil {
		switch member.Kind {
		case spec.IndexKindUnique, spec.IndexKindPrimary:
			indexMember = member
		case spec.IndexKindSecondary:
			// Provably outside the constraint namespace.
		default:
			// The stored kind may hide a unique member the generic
			// keyword would remove — its fate is undecidable.
			indexKindUnproven = true
		}
	}
	pkFound := false
	if pkName, proven := spec.PrimaryKeyObjectName(shape); proven && strings.EqualFold(pkName, name) {
		pkFound = true
	}
	if shape.PrimaryKey != nil && !shape.PrimaryKeyUnknown && strings.EqualFold(name, "primary") {
		pkFound = true
	}
	// `primary` with an unproven PK presence may still bind the object —
	// a possible kind, never a definite one.
	pkMayBind := shape.PrimaryKeyUnknown && strings.EqualFold(name, "primary")
	definiteCount := 0
	for _, found := range []bool{checkFound, fkFound, indexMember != nil || pkFound} {
		if found {
			definiteCount++
		}
	}
	unvouchBoundMember := func() {
		switch {
		case checkFound:
			s.unvouchCheckMember(key, name)
		case fkFound:
			s.unvouchForeignKeyMember(key, name)
		case pkFound:
			s.unvouchPrimaryKeyObject(key)
		case indexMember != nil:
			s.unvouchIndexMember(key, name)
		}
	}
	if spelling == spec.CheckSpellingConstraint {
		// Generic executes: exactly one bound kind drops; ambiguity or a
		// missing name refuses. An uncertain kind (unknown index kind, an
		// unproven PK presence) splits the worlds: it either adds the
		// ambiguity that keeps everything, or it is the lone binding that
		// dies — while a definite member set that already refuses stays
		// vouched in every world.
		switch {
		case definiteCount >= 2:
			// Ambiguity is proven with or without the uncertain kind.
		case definiteCount == 1:
			// The sole definite member dies unless the uncertain kind
			// adds the ambiguity refusal — undecidable.
			unvouchBoundMember()
		default:
			if indexKindUnproven {
				// Unique → removed; anything else → missing refusal.
				s.unvouchIndexMember(key, name)
			}
			if pkMayBind {
				// Present → sole binding removed; absent → missing.
				s.unvouchPrimaryKeyObject(key)
			}
		}
		return
	}
	// Unproven spelling: the CHECK member drops under the CHECK keyword;
	// a lone bound member also drops under the generic keyword; any other
	// member survives under both (check-spelling refuses non-check,
	// generic refuses the ambiguous set).
	if checkFound {
		s.unvouchCheckMember(key, name)
	}
	if definiteCount == 1 && !checkFound {
		unvouchBoundMember()
	}
	if definiteCount == 0 {
		if indexKindUnproven {
			s.unvouchIndexMember(key, name)
		}
		if pkMayBind {
			s.unvouchPrimaryKeyObject(key)
		}
	}
}

// unvouchCheckMember drops the named member's projection and unvouches
// the constraint collection — the same undecidable-fate shape
// unvouchForeignKeyMember records, for a CHECK member that may have been
// removed or kept. Name-keyed readers owe the collection gap rather than
// answering either way.
func (s *batchState) unvouchCheckMember(key batchTableKey, name string) {
	s.withClonedShape(key, func(shape *spec.TableSnapshot) {
		kept := shape.Constraints[:0]
		for i := range shape.Constraints {
			constraint := &shape.Constraints[i]
			if constraint.Type == "check" && !constraint.NameUnknown && strings.EqualFold(constraint.Name, name) {
				continue
			}
			kept = append(kept, *constraint)
		}
		shape.Constraints = kept
		shape.ConstraintsUnknown = true
	})
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
