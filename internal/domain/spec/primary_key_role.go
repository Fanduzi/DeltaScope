// Package spec resolves the logical primary-key role a unique index can
// hold independently of an explicitly declared PRIMARY KEY.
// input: a table's index members (with member-level key-part facts), column
// nullability, and the explicit-PK state of a snapshot or a declared CREATE
// TABLE shape
// output: the primary-role holder name ("" when none), or a bounded
// unresolved answer listing the facts that withheld the determination —
// consumed by the advanced-index compatibility assessment, the alter_index
// existence resolution, and the ordered-state publication gate
// pos: T10-R1 primary-role identity model (issue #89); upstream anchors —
// MySQL 8.0.46 mysql-test/r/invisible_indexes.result L247-289 and TiDB
// v8.5.0 TableInfo.GetPrimaryKey + checkInvisibleIndexOnPK
// note: if this file changes, update this header and module README.md.
package spec

import "strings"

// PrimaryRoleResolution is the shared three-state answer for "which index
// carries the primary-key role on this table shape".
type PrimaryRoleResolution struct {
	// Resolved marks a determined answer: HasHolder/Holder then name the
	// role-bearing index, or no index holds the role (explicit primary
	// key, or no qualifying candidate).
	Resolved bool
	// HasHolder distinguishes "resolved, no index carries the role" (an
	// explicit primary key exists, or no qualifying candidate) from
	// "resolved, the holder is the member named Holder" — including an
	// auto-named member whose Holder is the empty string.
	HasHolder bool
	// Holder is the resolved role-holder index name. It is never a model
	// rewrite — the holder stays an ordinary unique index; only its
	// logical role is answered.
	Holder string
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
// first qualifying UNIQUE index in the product's selection order. Both
// anchors agree on the semantics (native-pinned on MySQL 8.0.46/8.4.10 and
// TiDB v8.5.0, 2026-10): qualifying means
// every member is a plain column part (prefix members still qualify;
// expression and unmodeled members disqualify the index) bound to a column
// proven NOT NULL. When several candidates qualify and the slice order is
// not the catalog order the holder is unprovable; a withheld member set or
// unvouched primary key likewise withholds the answer.
//
// candidateOrderCatalog reports whether the index slice's order is the
// product's selection order — true for a declared CREATE TABLE shape and a
// batch-derived snapshot, false for a provider-loaded (name-sorted) set.
func ResolvePrimaryRole(snapshot *TableSnapshot, candidateOrderCatalog bool) PrimaryRoleResolution {
	if snapshot == nil || !snapshot.Exists {
		return PrimaryRoleResolution{Resolved: true}
	}
	if snapshot.PrimaryKeyUnknown || snapshot.ConstraintsUnknown {
		return PrimaryRoleResolution{GapFacts: []string{PrimaryRoleGapPrimaryKey}}
	}
	if snapshot.HasPrimaryKey() {
		return PrimaryRoleResolution{Resolved: true}
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
			eligible, provable := primaryRoleEligible(index, snapshot)
			if eligible {
				return PrimaryRoleResolution{Resolved: true, HasHolder: true, Holder: index.Name}
			}
			if !provable {
				return PrimaryRoleResolution{GapFacts: []string{PrimaryRoleGapMemberFacts}}
			}
		}
		return PrimaryRoleResolution{Resolved: true}
	}
	var candidates []string
	var unprovable []string
	for i := range snapshot.Indexes {
		index := &snapshot.Indexes[i]
		eligible, provable := primaryRoleEligible(index, snapshot)
		if eligible {
			candidates = append(candidates, index.Name)
		} else if !provable {
			unprovable = append(unprovable, index.Name)
		}
	}
	// Without catalog order, an eligibility-unprovable unique index can
	// hold the role ahead of every provable candidate, so no determined
	// answer exists while any is around.
	if len(unprovable) > 0 {
		return PrimaryRoleResolution{GapFacts: []string{PrimaryRoleGapMemberFacts}}
	}
	if len(candidates) == 0 {
		return PrimaryRoleResolution{Resolved: true}
	}
	if len(candidates) > 1 {
		return PrimaryRoleResolution{GapFacts: []string{PrimaryRoleGapIndexOrder}}
	}
	return PrimaryRoleResolution{Resolved: true, HasHolder: true, Holder: candidates[0]}
}

// ResolveDeclaredPrimaryRole is the create-time analogue: the declared table
// shape is evaluated within the statement itself (declaration order is the
// selection order, columns and indexes are the statement's own).
func ResolveDeclaredPrimaryRole(ddl *DDL) PrimaryRoleResolution {
	if ddl == nil {
		return PrimaryRoleResolution{Resolved: true}
	}
	if ddl.PrimaryKey != nil {
		return PrimaryRoleResolution{Resolved: true}
	}
	view := TableSnapshot{
		Exists:  true,
		Columns: ddl.Columns,
		Indexes: ddl.Indexes,
	}
	return ResolvePrimaryRole(&view, true)
}

// primaryRoleEligible decides whether one index can carry the implicit
// primary role, and whether the decision is provable from available facts.
// Eligible requires: unique kind, at least one member, every member a plain
// column part bound to a proven NOT NULL column. Expression parts provably
// disqualify (both anchors: the hidden member columns never carry the role);
// a withheld column set, an unresolvable member, or an unmodeled member makes
// the decision unprovable. Prefix lengths do not disqualify — the product
// promotes a prefix-unique the same way.
func primaryRoleEligible(index *Index, snapshot *TableSnapshot) (eligible, provable bool) {
	if index == nil || index.Kind != IndexKindUnique {
		return false, true
	}
	if index.ExpressionCount > 0 || index.HasExpressionKeys {
		return false, true
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
	if dialect == DialectTiDB && snapshot.PrimaryKeyIsHandle {
		return false, true
	}
	return true, true
}

// PrimaryKeyHandle reports whether a declared primary key becomes the row
// handle (TiDB PKIsHandle): dialect TiDB, exactly one member bound to an
// integer-family column, and no NONCLUSTERED override. Every other shape —
// MySQL, composite/non-int/nonclustered TiDB PKs — keeps a real index
// object. Integer family mirrors TiDB isSingleIntPK.
func PrimaryKeyHandle(dialect Dialect, pk *Index, columns []Column) bool {
	if dialect != DialectTiDB || pk == nil || len(pk.Columns) != 1 {
		return false
	}
	if strings.EqualFold(pk.PrimaryKeyType, "nonclustered") {
		return false
	}
	for _, column := range columns {
		if strings.EqualFold(column.Name, pk.Columns[0]) {
			return isIntegerType(baseColumnTypeName(column.Type))
		}
	}
	return false
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
