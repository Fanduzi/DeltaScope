// Package spec resolves the logical primary-key role a unique index can
// hold independently of an explicitly declared PRIMARY KEY.
// input: a table's index members (with member-level key-part facts), column
// nullability, and the explicit-PK state of a snapshot or a declared CREATE
// TABLE shape
// output: a PrimaryRoleResolution — the four-state answer distinguishing an
// explicit primary, a proven implicit holder (a member identity, not a name
// string, so multiple unnamed members stay distinct), a proven no-candidate
// state, and a bounded unresolved answer listing the facts that withheld the
// determination — consumed by the advanced-index compatibility assessment,
// the alter_index existence resolution, and the ordered-state publication
// gate; the TiDB handle answer is the tri-state *bool (nil = unprovable,
// explicit CLUSTERED|NONCLUSTERED overrides the session mode, an unstated
// declaration falls back to tidb_enable_clustered_index — int_only also
// defers to the alter-primary-key config fact — and missing mode/config
// evidence stays unknown rather than guessing a PRIMARY object)
// pos: T10-R2 primary-role identity model (issue #89); upstream anchors —
// MySQL 8.0.46 mysql-test/r/invisible_indexes.result L247-289 and TiDB
// v8.5.0 TableInfo.GetPrimaryKey + checkInvisibleIndexOnPK
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
	if snapshot.PrimaryKeyUnknown || snapshot.ConstraintsUnknown {
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
	if snapshot.PrimaryKeyUnknown || snapshot.ConstraintsUnknown {
		return false, false
	}
	if !snapshot.HasPrimaryKey() {
		return false, true
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

// isIntegerType mirrors TiDB's single-integer-PK handle condition; the
// caller passes the base type name already stripped of parameter spans.
func isIntegerType(baseType string) bool {
	switch strings.ToLower(strings.TrimSpace(baseType)) {
	case "tinyint", "smallint", "mediumint", "int", "integer", "bigint":
		return true
	}
	return false
}
