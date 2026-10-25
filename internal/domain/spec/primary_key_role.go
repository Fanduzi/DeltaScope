// Package spec resolves the logical primary-key role a unique index can
// hold independently of an explicitly declared PRIMARY KEY.
// input: a table's index members (with key-part facts), column nullability,
// and the explicit-PK state of a snapshot or declared CREATE TABLE shape
// output: PrimaryRoleResolution — the four-state answer (explicit primary,
// proven implicit holder by member identity, proven no-candidate, bounded
// unresolved listing the withheld facts); the TiDB handle tri-state *bool
// (nil unprovable, CLUSTERED|NONCLUSTERED overrides the session mode, else
// tidb_enable_clustered_index — int_only defers to alter-primary-key);
// PrimaryKeyClustered answering the TiDB 8200 drop premise beside the
// handle answer (common-handle PKs are clustered yet addressable); and the
// shared name bindings backing every name-keyed consumer — ordered state,
// rules, assessment: `primary` answers four ways, a target five (member /
// PK object / handle / free / ambiguous) — a proven-absent PK frees names
// despite stale storage unknowns or an unvouched constraint collection
// pos: T10-R2/R4/R5 primary-role/clustered/name-binding model (issue #89);
// anchors — MySQL 8.0.46 invisible_indexes.result L247-289, TiDB v8.5.0
// TableInfo.GetPrimaryKey + checkInvisibleIndexOnPK
// note: if this file changes, update this header and module README.md.
package spec

import "strings"

// PrimaryRoleResolution is the shared four-state answer for "which index
// carries the primary-key role on this table shape": an explicit primary
// key, an ordinary unique index holding the implicit role, a resolved
// absence of any candidate, or a bounded unresolved answer.
type PrimaryRoleResolution struct {
	// Resolved marks a determined answer: ExplicitPrimary, HasHolder with
	// its HolderIndex identity, or neither (no qualifying candidate).
	Resolved bool
	// ExplicitPrimary distinguishes "an explicit primary key holds the
	// role" from "no index holds the role" — the two previous no-holder
	// states behaved identically for an appended candidate, which is
	// wrong: only the no-candidate state lets a newly declared qualifying
	// UNIQUE claim the role.
	ExplicitPrimary bool
	// HasHolder distinguishes "resolved, no ordinary index carries the
	// role" (explicit primary, or no qualifying candidate) from
	// "resolved, the member at HolderIndex holds the role" — including an
	// auto-named member whose Holder is the empty string.
	HasHolder bool
	// Holder is the resolved role-holder index name. It is never a model
	// rewrite — the holder stays an ordinary unique index; only its
	// logical role is answered.
	Holder string
	// HolderIndex points at the resolved holder inside the index slice the
	// caller resolved over (declared ddl.Indexes or snapshot.Indexes) —
	// the stable member identity when two members share the same name,
	// including the unnamed-member "" name products auto-fill at create.
	// nil unless HasHolder.
	HolderIndex *Index
	// GapFacts lists the missing facts when Resolved is false — the bounded
	// evidence gap a rule must surface instead of guessing.
	GapFacts []string
}

// Gap fact names emitted when the role cannot be determined.
const (
	PrimaryRoleGapPrimaryKey      = "target_table.primary_key"
	PrimaryRoleGapIndexes         = "target_table.indexes"
	PrimaryRoleGapIndexOrder      = "target_table.indexes.ordinal"
	PrimaryRoleGapMemberFacts     = "target_table.indexes.key_parts"
	PrimaryRoleGapColumnNullables = "target_table.columns"
)

// ResolvePrimaryRole determines which index carries the primary-key role on
// a known table shape — the explicit primary key when one exists, else the
// first qualifying UNIQUE index in the product's selection order (natively
// pinned on MySQL 8.0.46/8.4.10 and TiDB v8.5.0, 2026-10): qualifying means
// every member is a plain column part bound to a column proven NOT NULL;
// prefix members still qualify. An expression member disqualifies the index
// on TiDB (hidden member columns never carry the role) but on MySQL the
// hidden generated column can qualify — its nullability and the mixed
// functional/plain selection order are facts we cannot prove, so the index
// stays unprovable and withholds the answer instead of a guessed pass or
// refusal. A withheld member set or unvouched primary key likewise
// withholds the answer.
//
// candidateOrderCatalog reports whether the index slice's order is the
// product's selection order — true for a declared CREATE TABLE shape and a
// batch-derived snapshot, false for a provider-loaded (name-sorted) set.
func ResolvePrimaryRole(dialect Dialect, snapshot *TableSnapshot, candidateOrderCatalog bool) PrimaryRoleResolution {
	if snapshot == nil || !snapshot.Exists {
		return PrimaryRoleResolution{Resolved: true}
	}
	if snapshot.PrimaryKeyUnknown {
		return PrimaryRoleResolution{GapFacts: []string{PrimaryRoleGapPrimaryKey}}
	}
	if snapshot.HasPrimaryKey() {
		return PrimaryRoleResolution{Resolved: true, ExplicitPrimary: true}
	}
	if snapshot.IndexesUnknown {
		return PrimaryRoleResolution{GapFacts: []string{PrimaryRoleGapIndexes}}
	}
	if candidateOrderCatalog {
		// Declaration/catalog order is the product's selection order: the
		// first qualifying candidate wins outright — unprovable indexes
		// ahead of it withhold the answer, ones behind it cannot displace
		// it and do not matter.
		for i := range snapshot.Indexes {
			index := &snapshot.Indexes[i]
			eligible, provable := primaryRoleEligible(dialect, index, snapshot)
			if eligible {
				return PrimaryRoleResolution{Resolved: true, HasHolder: true, Holder: index.Name, HolderIndex: index}
			}
			if !provable {
				return PrimaryRoleResolution{GapFacts: []string{PrimaryRoleGapMemberFacts}}
			}
		}
		return PrimaryRoleResolution{Resolved: true}
	}
	var candidates []*Index
	var unprovable int
	for i := range snapshot.Indexes {
		index := &snapshot.Indexes[i]
		eligible, provable := primaryRoleEligible(dialect, index, snapshot)
		if eligible {
			candidates = append(candidates, index)
		} else if !provable {
			unprovable++
		}
	}
	// Without catalog order, an eligibility-unprovable unique index can
	// hold the role ahead of every provable candidate, so no determined
	// answer exists while any is around.
	if unprovable > 0 {
		return PrimaryRoleResolution{GapFacts: []string{PrimaryRoleGapMemberFacts}}
	}
	if len(candidates) == 0 {
		return PrimaryRoleResolution{Resolved: true}
	}
	if len(candidates) > 1 {
		return PrimaryRoleResolution{GapFacts: []string{PrimaryRoleGapIndexOrder}}
	}
	return PrimaryRoleResolution{Resolved: true, HasHolder: true, Holder: candidates[0].Name, HolderIndex: candidates[0]}
}

// ResolveDeclaredPrimaryRole is the create-time analogue: the declared table
// shape is evaluated within the statement itself (declaration order is the
// selection order, columns and indexes are the statement's own).
func ResolveDeclaredPrimaryRole(dialect Dialect, ddl *DDL) PrimaryRoleResolution {
	if ddl == nil {
		return PrimaryRoleResolution{Resolved: true}
	}
	if ddl.PrimaryKey != nil {
		return PrimaryRoleResolution{Resolved: true, ExplicitPrimary: true}
	}
	view := TableSnapshot{
		Exists:  true,
		Columns: ddl.Columns,
		Indexes: ddl.Indexes,
	}
	return ResolvePrimaryRole(dialect, &view, true)
}

// primaryRoleEligible decides whether one index can carry the implicit
// primary role, and whether the decision is provable from available facts.
// Eligible requires: unique kind, at least one member, every member a plain
// column part bound to a proven NOT NULL column. Expression parts provably
// disqualify (both anchors: the hidden member columns never carry the role);
// a withheld column set, an unresolvable member, or an unmodeled member makes
// the decision unprovable. Prefix lengths do not disqualify — the product
// promotes a prefix-unique the same way.
func primaryRoleEligible(dialect Dialect, index *Index, snapshot *TableSnapshot) (eligible, provable bool) {
	if index == nil || index.Kind != IndexKindUnique {
		return false, true
	}
	if index.ExpressionCount > 0 || index.HasExpressionKeys {
		// TiDB: hidden member columns never carry the role (native rc:0).
		// MySQL promotes a functional unique whose hidden generated column
		// is NOT NULL, and prefers a plain-column candidate over an
		// earlier-declared functional one — neither fact is provable from
		// our member set, so the index is unprovable rather than
		// disqualified.
		return false, dialect != DialectMySQL
	}
	if index.UnmodeledKeyParts > 0 {
		return false, false
	}
	if len(index.Columns) == 0 {
		// A unique index whose member list is entirely unrecorded cannot be
		// proven to qualify or to fail.
		return false, false
	}
	if snapshot.Columns == nil {
		return false, false
	}
	for _, member := range index.Columns {
		column := snapshot.FindColumn(member)
		if column == nil {
			return false, false
		}
		if !column.NotNull {
			return false, true
		}
	}
	return true, true
}

// ResolveIndexPrimaryObject reports whether a name resolves to an operable
// index object for ALTER INDEX … purposes — distinct from the logical
// primary-key role. MySQL resolves `primary` to the vouched primary key.
// TiDB resolves it only when the primary key is a real index object — a
// PKIsHandle table (single-int clustered PK) has none (1176). An unvouched
// primary-key state answers resolved=false so the caller emits a gap rather
// than a fabricated absence.
func ResolveIndexPrimaryObject(snapshot *TableSnapshot, dialect Dialect) (exists, resolved bool) {
	if snapshot == nil {
		return false, false
	}
	if snapshot.PrimaryKeyUnknown {
		return false, false
	}
	if !snapshot.HasPrimaryKey() {
		return false, true
	}
	// The object binds only its actual stored name — a renamed PK frees
	// `primary` (1176 ALTER / 1091 DROP on the freed name), while a
	// present PK whose stored name stayed unproven may still own it.
	pkName, proven := PrimaryKeyObjectName(snapshot)
	if proven && !strings.EqualFold(pkName, "primary") {
		return false, true
	}
	if !proven {
		return false, false
	}
	if dialect == DialectTiDB {
		switch {
		case snapshot.PrimaryKeyHandleUnknown:
			// The stored shape could not prove handle vs real object —
			// neither answer may be asserted.
			return false, false
		case snapshot.PrimaryKeyIsHandle:
			return false, true
		}
	}
	return true, true
}

// PrimaryKeyHandle reports whether a declared primary key becomes the row
// handle (TiDB PKIsHandle): dialect TiDB, exactly one member bound to an
// integer-family column, and no NONCLUSTERED override. The answer is
// tri-state: nil when the stored shape cannot be proven — an unwritten
// CLUSTERED|NONCLUSTERED type defers to the session's
// tidb_enable_clustered_index (upstream anchor: v8.5.0
// pkg/ddl/create_table.go L1582-1593), so the caller passes the observed
// default mode. "on" and "off" decide outright; "int_only" clusters a
// single-int PK only when alter-primary-key is disabled, so it also defers
// to the caller-passed alterPrimaryKey fact (true → real object, false →
// handle, nil/unobserved → unknown); nil/unrecognized modes stay unknown.
// Every other shape — MySQL, composite/non-int/nonclustered TiDB PKs —
// keeps a real index object. Integer family mirrors TiDB isSingleIntPK.
func PrimaryKeyHandle(dialect Dialect, pk *Index, columns []Column, defaultMode *string, alterPrimaryKey *bool) *bool {
	no := new(bool)
	if dialect != DialectTiDB || pk == nil {
		return no
	}
	if strings.EqualFold(pk.PrimaryKeyType, "nonclustered") {
		return no
	}
	if len(pk.Columns) != 1 {
		// Composite PKs are real index objects under every mode
		// (common-handle storage still exposes PRIMARY).
		return no
	}
	if columns == nil {
		// A withheld column set cannot prove the member's type family —
		// the handle answer is unprovable either way.
		return nil
	}
	singleInt := false
	memberFound := false
	for _, column := range columns {
		if strings.EqualFold(column.Name, pk.Columns[0]) {
			memberFound = true
			singleInt = isIntegerType(baseColumnTypeName(column.Type))
			break
		}
	}
	if !memberFound {
		// The PK member is not provable in the column set — the handle
		// answer stays unknown rather than guessing non-integer.
		return nil
	}
	if !singleInt {
		// Composite and non-integer PKs are real index objects under every
		// mode (common-handle storage still exposes PRIMARY).
		return no
	}
	if strings.EqualFold(pk.PrimaryKeyType, "clustered") {
		yes := true
		return &yes
	}
	// An unwritten type follows the session default — the audit can only
	// assert the observed mode, never assume the product's ON default.
	if defaultMode == nil {
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(*defaultMode)) {
	case "on":
		yes := true
		return &yes
	case "int_only":
		// int_only clusters a single-int PK only while alter-primary-key is
		// disabled (upstream: !AlterPrimaryKey && isSingleIntPK). With the
		// config on the PK is a real index object; unobserved → unknown.
		if alterPrimaryKey == nil {
			return nil
		}
		if *alterPrimaryKey {
			return no
		}
		yes := true
		return &yes
	case "off":
		return no
	}
	return nil
}

// PrimaryKeyClustered reports whether a declared primary key stores
// clustered (TiDB PKIsHandle row handle or IsCommonHandle common handle)
// — the DROP PRIMARY KEY premise, since TiDB 8.5.0 refuses the drop with
// error 8200 on every clustered primary (upstream anchor: v8.5.0
// pkg/ddl/executor.go IsCommonHandle||PKIsHandle, probed on INT/VARCHAR/
// composite clustered shapes). Tri-state like PrimaryKeyHandle: nil when
// the stored shape cannot be proven. A clustered answer never implies the
// PRIMARY object is unaddressable — common-handle PKs remain real index
// objects. MySQL's InnoDB key is clustered on disk but DROP PRIMARY KEY
// rebuilds it natively, so non-TiDB dialects answer false here — this
// fact names the TiDB refusal class, not generic storage clustering.
func PrimaryKeyClustered(dialect Dialect, pk *Index, columns []Column, defaultMode *string, alterPrimaryKey *bool) *bool {
	no := new(bool)
	if dialect != DialectTiDB || pk == nil {
		return no
	}
	if strings.EqualFold(pk.PrimaryKeyType, "nonclustered") {
		return no
	}
	if strings.EqualFold(pk.PrimaryKeyType, "clustered") {
		// An explicit CLUSTERED declaration clusters every member shape —
		// single-int (PKIsHandle) and common-handle alike.
		yes := true
		return &yes
	}
	if defaultMode == nil {
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(*defaultMode)) {
	case "on":
		// Clustered mode clusters every declared PK shape.
		yes := true
		return &yes
	case "off":
		return no
	case "int_only":
		// int_only clusters only the handle-qualified single-integer
		// shape — the same member premise the handle answer consumes.
		if len(pk.Columns) != 1 {
			return no
		}
		if columns == nil {
			return nil
		}
		singleInt := false
		memberFound := false
		for _, column := range columns {
			if strings.EqualFold(column.Name, pk.Columns[0]) {
				memberFound = true
				singleInt = isIntegerType(baseColumnTypeName(column.Type))
				break
			}
		}
		if !memberFound {
			return nil
		}
		if !singleInt {
			return no
		}
		if alterPrimaryKey == nil {
			return nil
		}
		if *alterPrimaryKey {
			return no
		}
		yes := true
		return &yes
	}
	return nil
}

// isIntegerType mirrors TiDB's single-integer-PK handle condition; the
// caller passes the base type name already stripped of parameter spans.
func isIntegerType(baseType string) bool {
	switch strings.ToLower(strings.TrimSpace(baseType)) {
	case "tinyint", "smallint", "mediumint", "int", "integer", "bigint":
		return true
	}
	return false
}

// PrimaryNameBinding resolves what the index name `primary` binds to
// inside a table shape — member-first precedence, then the primary-key
// identity. It is the single answer shared by the ordered-state
// publishers, the name-keyed existence rules, and the advanced-index
// assessment so all three agree on the same binding order.
type PrimaryNameBinding int

const (
	// PrimaryNameBindsMember: a member literally named `primary` provably
	// owns the name — reachable only on TiDB, where RENAME … TO `primary`
	// is legal on PK-less and PKIsHandle tables (native-verified v8.5.0:
	// the member then takes DROP/ALTER/RENAME `primary` targets, both
	// casings). MySQL can never reach this state — the name is reserved
	// for the PK index object outright (1280 on add/rename).
	PrimaryNameBindsMember PrimaryNameBinding = iota
	// PrimaryNameBindsPK: `primary` answers the primary-key identity.
	// Always true on MySQL (the reserved name answers the PK or nothing),
	// and true on TiDB exactly while a real PRIMARY index object provably
	// occupies the name space — a member `primary` can never coexist with
	// it (1061 on RENAME … TO `primary`).
	PrimaryNameBindsPK
	// PrimaryNameBindsAmbiguous: the member name space is unvouched and a
	// member `primary` cannot be ruled out — the name may bind a member
	// or the PK identity.
	PrimaryNameBindsAmbiguous
	// PrimaryNameBindsNothing: member space vouched and the PK object's
	// proven stored name is not `primary` — a renamed PK freed the name
	// (verified v8.5.0: 1176 ALTER / 1091 DROP on the freed name, and a
	// member may legally claim it while the PK object stays named pk3).
	// Reachable only on TiDB; MySQL never frees the reserved name.
	PrimaryNameBindsNothing
)

// PrimaryKeyObjectName returns the primary-key object's proven stored
// catalog name. The constraint identity stays PRIMARY across renames but
// the object name moves, so name-keyed consumers must bind this answer —
// not the literal `primary`. proven=false covers both an absent PK and a
// present PK whose attribution/name could not be proven on reload.
func PrimaryKeyObjectName(snapshot *TableSnapshot) (name string, proven bool) {
	if snapshot == nil || snapshot.PrimaryKey == nil || snapshot.PrimaryKeyUnknown ||
		snapshot.PrimaryKey.NameUnknown || strings.TrimSpace(snapshot.PrimaryKey.Name) == "" {
		return "", false
	}
	return snapshot.PrimaryKey.Name, true
}

// IndexNameBinding classifies what an arbitrary catalog name binds inside
// a table shape — the generalized form of PrimaryNameBinding. The ordered
// state publisher uses it to route name-keyed successors; the existence
// rules use it for presence answers. Member claims always win the name
// natively (a member literally holding the target name provably owns it);
// the PK object claims the name only through its actual stored catalog
// name, never through the logical identity.
type IndexNameBinding int

const (
	// IndexNameBindsMember: a member with a proven stored name owns it.
	IndexNameBindsMember IndexNameBinding = iota
	// IndexNameBindsPKObject: a proven operable PK object owns it — the
	// name-keyed statement reaches the real index object (visible in
	// SHOW INDEX; alterable, renamable, droppable under that name).
	IndexNameBindsPKObject
	// IndexNameBindsPKIdentity: the synthesized `primary` catalog row of a
	// handle/common-handle PK owns it — the row exists for existence
	// answers but no operable object stands behind it (1176 on ALTER).
	IndexNameBindsPKIdentity
	// IndexNameBindsNone: the name is provably free — every member name is
	// vouched, no member holds it, and the PK's proven stored name differs
	// (or no PK exists / the name was freed by a rename).
	IndexNameBindsNone
	// IndexNameBindsAmbiguous: the binding cannot be proven — an unvouched
	// member name or an unproven PK-object name may claim the target.
	IndexNameBindsAmbiguous
)

// ResolveIndexNameBinding answers what an arbitrary index name binds.
// Ordering mirrors the native name lookup: a proven member name wins
// first; the PK object binds only through its proven stored name (a
// renamed PK frees `primary` and claims its new name instead); a handle
// PK's synthetic row binds `primary` as a non-operable identity; whatever
// cannot be proven stays ambiguous rather than guessing.
func ResolveIndexNameBinding(dialect Dialect, snapshot *TableSnapshot, name string) IndexNameBinding {
	if snapshot == nil {
		return IndexNameBindsAmbiguous
	}
	if dialect == DialectMySQL && strings.EqualFold(name, "primary") {
		// MySQL reserves `primary` for the PK object outright — no member
		// may ever own it (1280 on add/rename), so the member name space
		// is irrelevant to the binding.
		pkName, proven := PrimaryKeyObjectName(snapshot)
		if proven && strings.EqualFold(pkName, name) {
			return IndexNameBindsPKObject
		}
		if !proven && !snapshot.HasPrimaryKey() && !snapshot.PrimaryKeyUnknown {
			return IndexNameBindsNone
		}
		return IndexNameBindsAmbiguous
	}
	if !snapshot.IndexesUnknown {
		if member := snapshot.FindProvenIndex(name); member != nil {
			return IndexNameBindsMember
		}
		if !snapshot.IndexNamesUnknown && !snapshot.HasUnprovenIndexName() {
			// The member name space is fully vouched — only the PK
			// answer left.
			pkName, proven := PrimaryKeyObjectName(snapshot)
			if !proven {
				if snapshot.HasPrimaryKey() && !snapshot.PrimaryKeyUnknown {
					// PK present but its stored name is unproven — it
					// may own the target name.
					return IndexNameBindsAmbiguous
				}
				// Only a proven-absent PK makes the name free — an
				// unproven PK presence could still hide a named object.
				// Storage unknowns are attributes of a PK that exists;
				// once presence is proven absent — a dropped object's
				// stale handle/clustered flags — they describe nothing
				// that can own a name.
				if snapshot.PrimaryKeyUnknown {
					return IndexNameBindsAmbiguous
				}
				return IndexNameBindsNone
			}
			if !strings.EqualFold(pkName, name) {
				return IndexNameBindsNone
			}
			if dialect == DialectTiDB && strings.EqualFold(name, "primary") {
				// The handle caveat applies to `primary` only — a handle
				// synth row is always named PRIMARY, so a proven
				// non-`primary` object name is already a real operable
				// object regardless of the undecided handle answer.
				if snapshot.PrimaryKeyHandleUnknown {
					return IndexNameBindsAmbiguous
				}
				if snapshot.PrimaryKeyIsHandle {
					return IndexNameBindsPKIdentity
				}
			}
			return IndexNameBindsPKObject
		}
	}
	// Member name space unvouched — a member may hold the target name.
	// A proven real PK object still rules the member out natively (1061
	// forbids the coexistence); a synthesized handle row does not occupy
	// the `primary` name against a member (verified: member `primary`
	// coexists with the synth PRIMARY row), so a `primary` target stays
	// ambiguous while the handle answer is undecided or proven — a proven
	// non-`primary` object name is already a real operable object.
	if pkName, proven := PrimaryKeyObjectName(snapshot); proven && strings.EqualFold(pkName, name) {
		if dialect == DialectTiDB && strings.EqualFold(name, "primary") &&
			(snapshot.PrimaryKeyHandleUnknown || snapshot.PrimaryKeyIsHandle) {
			return IndexNameBindsAmbiguous
		}
		return IndexNameBindsPKObject
	}
	return IndexNameBindsAmbiguous
}

// ResolvePrimaryNameBinding classifies a `primary` name-keyed target. A
// member whose stored name is provable (!NameUnknown) always wins the
// name; an unvouched member collection leaves the answer ambiguous unless
// the dialect — MySQL reserves `primary` outright — or a provable real
// PRIMARY index object rules the member out. Once a TiDB PK object is
// renamed off `primary`, the freed name binds nothing.
func ResolvePrimaryNameBinding(dialect Dialect, snapshot *TableSnapshot) PrimaryNameBinding {
	if snapshot == nil {
		return PrimaryNameBindsAmbiguous
	}
	switch ResolveIndexNameBinding(dialect, snapshot, "primary") {
	case IndexNameBindsMember:
		return PrimaryNameBindsMember
	case IndexNameBindsPKObject, IndexNameBindsPKIdentity:
		return PrimaryNameBindsPK
	case IndexNameBindsNone:
		if dialect == DialectMySQL {
			return PrimaryNameBindsPK
		}
		return PrimaryNameBindsNothing
	}
	return PrimaryNameBindsAmbiguous
}
