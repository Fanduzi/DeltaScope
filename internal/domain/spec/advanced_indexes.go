// Package spec defines parser-neutral advanced-index facts and assessments.
// input: normalized statements carrying member-level key-part facts
// (ordinal/kind/prefix/direction/expression identity), index visibility
// facts, bound table snapshots (columns/indexes/PK handle/partition state),
// and version metadata
// output: the shared AssessAdvancedIndexes answer — features, determined
// product/version and kind-aware member-legality issues, the four-state
// implicit-primary role verdict, and bounded version/fact gaps —
// InvalidTransition marks determined negatives and UnresolvedTransition
// marks feature-bearing answers that could not be proven either way (R6:
// a kind-withheld maybe-PK visibility target resolves Unresolved too),
// both consumed by the ordered-state publication gate; `alter_index primary`
// skips the primary-object path only on a proven member binding via
// ResolvePrimaryNameBinding (a TiDB member may own the name) — ambiguous
// bindings still resolve through the PRIMARY-object existence probe
// pos: T10 domain model + T10-R2/R4/R6 primary-role/name-binding answers
// note: if this file changes, update this header and module README.md.
package spec

import "strings"

// Advanced-index feature names — the compatibility features parameter
// vocabulary and the finding metadata values.
const (
	AdvancedIndexFeatureVisibility     = "visibility"
	AdvancedIndexFeatureExpressionKeys = "expression_keys"
	AdvancedIndexFeatureDescendingKeys = "descending_keys"
	AdvancedIndexFeatureMultiValued    = "multi_valued"
	AdvancedIndexFeaturePrefixLength   = "prefix_length"
	AdvancedIndexFeatureFulltext       = "fulltext"
	AdvancedIndexFeatureSpatial        = "spatial"
	AdvancedIndexFeatureGlobal         = "global"
)

// Advanced-index reason codes beyond the shared AdvancedReason* set.
const (
	// AdvancedIndexReasonDescendingIgnored marks a declared descending key
	// part the product accepts but silently stores ascending: MySQL below
	// 8.0 and TiDB keep no ordering fact — the statement succeeds, so this
	// reason never blocks post-state publication.
	AdvancedIndexReasonDescendingIgnored = "descending_ignored"
	// AdvancedIndexReasonPrimaryVisibility marks a determined negative:
	// the product rejects any invisibility transition or declaration on
	// the primary key (3522 on both MySQL 8.x and TiDB).
	AdvancedIndexReasonPrimaryVisibility = "primary_index_visibility"
	// AdvancedIndexReasonPrefixRequired marks a BLOB/TEXT member declared
	// without a prefix length — the product rejects it (1170 on both
	// MySQL and TiDB).
	AdvancedIndexReasonPrefixRequired = "prefix_required"
	// AdvancedIndexReasonFulltextColumnType marks a FULLTEXT member bound
	// to a column outside the character/text family (1283 on MySQL).
	AdvancedIndexReasonFulltextColumnType = "fulltext_column_type"
	// AdvancedIndexReasonSpatialColumnType marks a SPATIAL member bound to
	// a column outside the geometry family (1687 on MySQL).
	AdvancedIndexReasonSpatialColumnType = "spatial_column_type"
	// AdvancedIndexReasonSpatialNullable marks a SPATIAL member bound to a
	// nullable column — every part must be NOT NULL (1252 on MySQL).
	AdvancedIndexReasonSpatialNullable = "spatial_nullable"
	// AdvancedIndexReasonExpressionColumn marks an expression member that
	// references a column absent from the provable column set (1054 on
	// both products).
	AdvancedIndexReasonExpressionColumn = "expression_column_missing"
	// AdvancedIndexReasonGlobalNotUnique marks a GLOBAL index option on a
	// non-unique index — TiDB rejects it (8200) even on partitioned
	// tables.
	AdvancedIndexReasonGlobalNotUnique = "global_not_unique"
	// AdvancedIndexReasonGlobalNotPartitioned marks a GLOBAL index option
	// on a non-partitioned table — TiDB rejects it (8200).
	AdvancedIndexReasonGlobalNotPartitioned = "global_not_partitioned"
	// AdvancedIndexReasonJSONColumn marks a plain column member bound to a
	// JSON column — the product requires a generated-column path (3152 on
	// both products, even with a prefix length).
	AdvancedIndexReasonJSONColumn = "json_column_index"
	// AdvancedIndexReasonPrefixZero marks an authored 0-length key part —
	// the product rejects it on every index kind, FULLTEXT included (1391).
	AdvancedIndexReasonPrefixZero = "prefix_zero"
	// AdvancedIndexReasonPrefixNotString marks an authored prefix length on
	// a member bound to a non-string column (1089 on both products —
	// integer, temporal, bit, and geometry members all reject).
	AdvancedIndexReasonPrefixNotString = "prefix_not_string"
	// AdvancedIndexReasonPrefixOverLength marks an authored prefix length
	// past the member column's declared character length (1089).
	AdvancedIndexReasonPrefixOverLength = "prefix_over_length"
	// AdvancedIndexReasonMultiValuedCastType marks a multi-valued member
	// whose CAST target is outside the product's legal ARRAY family —
	// MySQL accepts signed/unsigned/char(n)/binary(n)/date/time/datetime/
	// decimal while TiDB accepts signed/unsigned/char(n)/binary(n)/date/
	// time/datetime/double; everything else answers 1235.
	AdvancedIndexReasonMultiValuedCastType = "multi_valued_cast_type"
)

// advancedIndexVersionGatedFeatures marks the features whose checks consult
// the target version. Member-legality features (prefix/FT/SPATIAL/GLOBAL
// member rules) are product rules on every supported anchor — they never
// consult the version, so they must not mint version gaps.
var advancedIndexVersionGatedFeatures = map[string]bool{
	AdvancedIndexFeatureVisibility:     true,
	AdvancedIndexFeatureExpressionKeys: true,
	AdvancedIndexFeatureDescendingKeys: true,
	AdvancedIndexFeatureMultiValued:    true,
}

// Minimum supported versions for MySQL-family advanced index forms. TiDB
// forms are gated on the validated series instead of these minimums.
var (
	advancedIndexMinimumVisibilityMySQL  = [3]int{8, 0, 0}
	advancedIndexMinimumExpressionMySQL  = [3]int{8, 0, 13}
	advancedIndexMinimumMultiValuedMySQL = [3]int{8, 0, 17}
)

const (
	AdvancedIndexMinimumVisibilityMySQL  = "8.0.0"
	AdvancedIndexMinimumExpressionMySQL  = "8.0.13"
	AdvancedIndexMinimumMultiValuedMySQL = "8.0.17"
)

// AdvancedIndexIssue is a determined product/version responsibility for one
// index: a form the target provably rejects (version_unsupported) or one the
// product accepts but changes semantics on (descending_ignored), or a
// primary-key visibility attempt the product always refuses
// (primary_index_visibility). Determined negatives mark the assessment's
// InvalidTransition; descending_ignored does not.
type AdvancedIndexIssue struct {
	Index          string
	Feature        string
	Action         string
	ReasonCode     string
	MinimumVersion string
}

// AdvancedIndexAssessment is the shared spec-owned answer for one
// statement's advanced-index facts, mirroring AdvancedColumnAssessment.
// Features is the set of advanced forms the statement carries; VersionGated
// is the subset that consulted the target version.
type AdvancedIndexAssessment struct {
	Applies           bool
	Issues            []AdvancedIndexIssue
	Gaps              []AdvancedColumnGap
	InvalidTransition bool
	// UnresolvedTransition marks a transition whose outcome or post-state
	// cannot be honestly derived — an unproven role or member-legality
	// fact the statement's success depends on. The ordered-state gate
	// tombstones the affected identities instead of publishing a guessed
	// shape. Mirrors AdvancedColumnAssessment.UnresolvedTransition.
	UnresolvedTransition bool
	Features             map[string]bool
	VersionGated         map[string]bool
}

// HasFeature reports whether the statement carries the named index feature.
func (a AdvancedIndexAssessment) HasFeature(feature string) bool {
	return a.Features[feature]
}

func (a *AdvancedIndexAssessment) appendGap(feature, reason string, facts ...string) {
	for _, gap := range a.Gaps {
		if gap.ReasonCode == reason && gap.Feature == feature && sameStringSet(gap.RequiredFacts, facts) {
			return
		}
	}
	a.Gaps = append(a.Gaps, AdvancedColumnGap{ReasonCode: reason, RequiredFacts: append([]string(nil), facts...), Feature: feature})
}

func (a *AdvancedIndexAssessment) appendIssue(index, feature, action, reason, minimum string) {
	a.Issues = append(a.Issues, AdvancedIndexIssue{
		Index:          index,
		Feature:        feature,
		Action:         action,
		ReasonCode:     reason,
		MinimumVersion: minimum,
	})
	if reason != AdvancedIndexReasonDescendingIgnored {
		// A provably rejected statement never publishes a post-state.
		// descending_ignored succeeds natively — the index exists, merely
		// without the declared ordering.
		a.InvalidTransition = true
	}
}

// advancedIndexFact binds one declared index or visibility-transition
// target to its statement action.
type advancedIndexFact struct {
	index  *Index
	name   string
	action string
}

func statementIndexFacts(ddl *DDL) []advancedIndexFact {
	var out []advancedIndexFact
	if ddl == nil {
		return out
	}
	appendIndex := func(index *Index, action string) {
		if index == nil {
			return
		}
		out = append(out, advancedIndexFact{index: index, name: strings.TrimSpace(index.Name), action: action})
	}
	switch ddl.Operation {
	case DDLOperationCreateTable:
		appendIndex(ddl.PrimaryKey, "create_table")
		for i := range ddl.Indexes {
			appendIndex(&ddl.Indexes[i], "create_table")
		}
	case DDLOperationCreateIndex:
		for i := range ddl.Alter {
			alter := &ddl.Alter[i]
			if alter.Action == "create_index" && alter.Index != nil {
				appendIndex(alter.Index.Definition, "create_index")
			}
		}
	case DDLOperationAlterTable:
		for i := range ddl.Alter {
			alter := &ddl.Alter[i]
			if alter.Index == nil {
				continue
			}
			switch alter.Action {
			case "add_index", "add_constraint":
				appendIndex(alter.Index.Definition, alter.Action)
			case "alter_index":
				if alter.Index.Definition == nil {
					continue
				}
				name := strings.TrimSpace(alter.Name)
				if name == "" {
					name = strings.TrimSpace(alter.Index.OldName)
				}
				out = append(out, advancedIndexFact{index: alter.Index.Definition, name: name, action: alter.Action})
			}
		}
	}
	return out
}

// indexFeatures returns the advanced-index forms one fact carries. ddl and
// snapshot supply the column table a member-level feature may need to prove
// its relevance (a BLOB/TEXT binding is inside the prefix-length contract
// even without an authored length).
func indexFeatures(fact advancedIndexFact, ddl *DDL, snapshot *TableSnapshot) []string {
	index := fact.index
	var features []string
	if fact.action == "alter_index" || index.VisibilityExplicit {
		features = append(features, AdvancedIndexFeatureVisibility)
	}
	if index.ExpressionCount > 0 || index.HasExpressionKeys {
		features = append(features, AdvancedIndexFeatureExpressionKeys)
	}
	if index.MultiValuedParts > 0 {
		features = append(features, AdvancedIndexFeatureMultiValued)
	}
	if index.DescParts > 0 {
		features = append(features, AdvancedIndexFeatureDescendingKeys)
	}
	for _, part := range index.KeyParts {
		if part.PrefixLength != nil {
			features = append(features, AdvancedIndexFeaturePrefixLength)
			break
		}
		if part.Kind == IndexKeyPartColumn {
			// A member bound to a BLOB/TEXT-family column is inside the
			// prefix-length contract even without an authored length, and
			// so is a direct JSON binding — the member check then decides
			// whether the fact is legal.
			if column := factMemberColumn(fact, part.Column, ddl, snapshot); column != nil {
				if base := baseColumnTypeName(column.Type); prefixRequiredFamilyType(base) || base == "json" {
					features = append(features, AdvancedIndexFeaturePrefixLength)
					break
				}
			}
		}
	}
	switch index.Kind {
	case IndexKindFulltext:
		features = append(features, AdvancedIndexFeatureFulltext)
	case IndexKindSpatial:
		features = append(features, AdvancedIndexFeatureSpatial)
	}
	if index.Global {
		features = append(features, AdvancedIndexFeatureGlobal)
	}
	return features
}

// AssessAdvancedIndexes computes the shared T10 assessment for one
// statement: the advanced-index forms it declares, the product/version
// responsibility issues, and the bounded version gaps. It reads bound
// metadata (target version) so the same answer feeds the compatibility
// rule and the ordered-state publication gate.
func AssessAdvancedIndexes(statement Statement) AdvancedIndexAssessment {
	assessment := AdvancedIndexAssessment{Features: map[string]bool{}, VersionGated: map[string]bool{}}
	if statement.Dialect != DialectMySQL && statement.Dialect != DialectTiDB {
		return assessment
	}
	ddl := statement.DDL
	if ddl == nil || (ddl.Operation != DDLOperationCreateTable &&
		ddl.Operation != DDLOperationCreateIndex && ddl.Operation != DDLOperationAlterTable) {
		return assessment
	}
	var version *VersionIdentity
	var snapshot *TableSnapshot
	if statement.Metadata != nil {
		version = statement.Metadata.Version
		snapshot = statement.Metadata.TargetTable
	}

	facts := statementIndexFacts(ddl)
	var gated []string
	for _, fact := range facts {
		for _, feature := range indexFeatures(fact, ddl, snapshot) {
			// TiDB descending is a product property (accepted, stored
			// ascending) — it never consults the version and must not
			// mint a missing-version gap.
			versionGated := advancedIndexVersionGatedFeatures[feature] &&
				(feature != AdvancedIndexFeatureDescendingKeys || statement.Dialect != DialectTiDB)
			if !assessment.Features[feature] && versionGated {
				gated = append(gated, feature)
			}
			assessment.Features[feature] = true
			if versionGated {
				assessment.VersionGated[feature] = true
			}
		}
	}
	// The version-gated features consult the target version on both
	// dialects: MySQL forms gate on their own minimums, TiDB forms gate on
	// the validated series. One bounded version gap per statement.
	if len(gated) > 0 {
		switch {
		case version == nil:
			assessment.appendGap("", AdvancedReasonMissingTargetVersion, AdvancedFactTargetVersion)
		case !version.ValidatedRange:
			assessment.appendGap("", AdvancedReasonOutOfValidatedRange, AdvancedFactTargetVersionRange)
		}
		if version == nil || !version.ValidatedRange {
			// The gated feature's outcome or stored shape is unprovable
			// without the target version — the statement may have been
			// refused outright or normalized into a different member
			// shape, so no post-state may publish a guessed success.
			assessment.UnresolvedTransition = true
		}
	}
	versionKnown := version != nil && version.ValidatedRange

	for _, fact := range facts {
		assessIndexFact(&assessment, fact, version, versionKnown, statement.Dialect, ddl, snapshot)
	}
	assessment.Applies = len(assessment.Features) > 0
	return assessment
}

// assessIndexFact evaluates one index fact against the product/version
// responsibility table. snapshot is the ordered pre-state the statement was
// enriched with (nil offline); ddl supplies the statement's own declared
// shape for create-table role determination.
func assessIndexFact(assessment *AdvancedIndexAssessment, fact advancedIndexFact, version *VersionIdentity, versionKnown bool, dialect Dialect, ddl *DDL, snapshot *TableSnapshot) {
	index := fact.index
	if index == nil {
		return
	}
	features := indexFeatures(fact, ddl, snapshot)
	for _, feature := range features {
		switch feature {
		case AdvancedIndexFeatureVisibility:
			if versionKnown && dialect == DialectMySQL && !versionAtLeast(version, advancedIndexMinimumVisibilityMySQL) {
				assessment.appendIssue(fact.name, feature, fact.action, AdvancedReasonVersionUnsupported, AdvancedIndexMinimumVisibilityMySQL)
			}
		case AdvancedIndexFeatureExpressionKeys:
			if versionKnown && dialect == DialectMySQL && !versionAtLeast(version, advancedIndexMinimumExpressionMySQL) {
				assessment.appendIssue(fact.name, feature, fact.action, AdvancedReasonVersionUnsupported, AdvancedIndexMinimumExpressionMySQL)
			}
		case AdvancedIndexFeatureMultiValued:
			if versionKnown && dialect == DialectMySQL && !versionAtLeast(version, advancedIndexMinimumMultiValuedMySQL) {
				assessment.appendIssue(fact.name, feature, fact.action, AdvancedReasonVersionUnsupported, AdvancedIndexMinimumMultiValuedMySQL)
			}
		case AdvancedIndexFeatureDescendingKeys:
			switch {
			case dialect == DialectTiDB:
				// TiDB accepts DESC syntax but catalogs every member
				// ascending — a product property, never a version gate.
				assessment.appendIssue(fact.name, feature, fact.action, AdvancedIndexReasonDescendingIgnored, "")
			case dialect == DialectMySQL && versionKnown && !versionAtLeast(version, advancedIndexMinimumVisibilityMySQL):
				// MySQL <8.0 accepts the syntax and stores ascending —
				// the declared ordering is silently dropped.
				assessment.appendIssue(fact.name, feature, fact.action, AdvancedIndexReasonDescendingIgnored, "")
			}
		}
	}
	// The primary-key visibility negative: declared PRIMARY KEY invisibility
	// is rejected at 3522 on both products (and on MySQL <8.0 the INVISIBLE
	// keyword fails at parse) — the statement never succeeds anywhere in the
	// supported range, so the refusal is version-free. ALTER INDEX `PRIMARY`
	// on MySQL is the same refusal class on every version that accepts the
	// syntax (3522 whether the request is VISIBLE or INVISIBLE). TiDB
	// resolves `PRIMARY` to a real index object only on non-handle tables;
	// a PKIsHandle table has no such object and the statement fails 1176
	// through the existence rule instead. An authored VISIBLE attribute on
	// the primary key is a natively-accepted no-op, not a refusal — only
	// invisibility fires.
	if index.Kind == IndexKindPrimary && index.Visibility == ColumnVisibilityInvisible {
		assessment.appendIssue(fact.name, AdvancedIndexFeatureVisibility, fact.action, AdvancedIndexReasonPrimaryVisibility, "")
		return
	}
	if fact.action == "alter_index" && strings.EqualFold(fact.name, "primary") &&
		(dialect != DialectTiDB || ResolvePrimaryNameBinding(dialect, snapshot) != PrimaryNameBindsMember) {
		// TiDB member-first precedence: only a member provably owning the
		// name (RENAME … TO `primary` is legal on PK-less and PKIsHandle
		// tables) makes the alter an ordinary member visibility operation
		// — an ambiguous binding can still be the PK object, so it enters
		// this branch and the existence probe decides: a resolved PRIMARY
		// object evaluates the 3522 refusal, an unresolved answer withholds
		// the outcome as a primary_key gap, and a proven-absent object is
		// provably not a PK-object operation at all.
		switch dialect {
		case DialectMySQL:
			if !versionKnown || versionAtLeast(version, advancedIndexMinimumVisibilityMySQL) {
				// 8.0+ refuses outright; <8.0 fails at parse, so the version
				// question only scopes the 3522 attribution, never the
				// refusal itself.
				assessment.appendIssue(fact.name, AdvancedIndexFeatureVisibility, fact.action, AdvancedIndexReasonPrimaryVisibility, "")
			}
		case DialectTiDB:
			exists, resolved := ResolveIndexPrimaryObject(snapshot, dialect)
			switch {
			case !resolved:
				// Whether a PRIMARY object exists decides whether this
				// alter can even name its target — the outcome stays
				// unpublished while the handle answer is unvouched.
				assessment.appendGap(AdvancedIndexFeatureVisibility, AdvancedReasonUnknownTableState, PrimaryRoleGapPrimaryKey)
				assessment.UnresolvedTransition = true
			case exists && index.Visibility == ColumnVisibilityInvisible:
				// A real PRIMARY index object cannot be made invisible —
				// the same 3522 refusal MySQL reports.
				assessment.appendIssue(fact.name, AdvancedIndexFeatureVisibility, fact.action, AdvancedIndexReasonPrimaryVisibility, "")
			}
		}
		return
	}
	if fact.action == "alter_index" && !strings.EqualFold(fact.name, "primary") &&
		ResolveIndexNameBinding(dialect, snapshot, fact.name) == IndexNameBindsPKObject {
		// The PK object under its actual stored name (a renamed pk2) —
		// the visibility operation reaches the real index object, so
		// INVISIBLE is the same 3522 refusal a `primary`-spelled alter
		// reports (verified on the renamed object, v8.5.0); VISIBLE is a
		// proven no-op and needs no issue.
		if index.Visibility == ColumnVisibilityInvisible {
			assessment.appendIssue(fact.name, AdvancedIndexFeatureVisibility, fact.action, AdvancedIndexReasonPrimaryVisibility, "")
		}
		return
	}
	assessImplicitPrimaryVisibility(assessment, fact, dialect, ddl, snapshot)
	assessIndexMembers(assessment, fact, dialect, ddl, snapshot)
}

// assessIndexMembers applies the member-level product rules to a declared
// index shape: prefix-required BLOB/TEXT columns (1170), JSON members (3152),
// FULLTEXT member column families (1283), SPATIAL member geometry type and
// NOT NULL (1687/1252), expression-member column dependencies (1054), and
// TiDB GLOBAL's unique-plus-partitioned gate (8200). It reads the declared
// column set on create_table and the bound snapshot on add/create-index
// actions; facts the statement or snapshot cannot vouch for surface as
// bounded gaps, never guessed answers.
func assessIndexMembers(assessment *AdvancedIndexAssessment, fact advancedIndexFact, dialect Dialect, ddl *DDL, snapshot *TableSnapshot) {
	index := fact.index
	if index == nil || fact.action == "alter_index" {
		return
	}
	if index.Global && dialect == DialectTiDB {
		// GLOBAL legality needs no column set — the unique-kind gate is a
		// statement fact and the partition gate answers from the bound shape
		// or stays a bounded gap when partition state is unvouched.
		switch index.Kind {
		case IndexKindUnique, IndexKindPrimary:
		default:
			assessment.appendIssue(index.Name, AdvancedIndexFeatureGlobal, fact.action, AdvancedIndexReasonGlobalNotUnique, "")
		}
		var partitioned *bool
		if fact.action == "create_table" && ddl != nil {
			known := ddl.HasPartition
			partitioned = &known
		} else if snapshot != nil {
			partitioned = snapshot.Partitioned
		}
		switch {
		case partitioned == nil:
			assessment.appendGap(AdvancedIndexFeatureGlobal, AdvancedReasonUnknownTableState, "target_table.partitions")
		case !*partitioned:
			assessment.appendIssue(index.Name, AdvancedIndexFeatureGlobal, fact.action, AdvancedIndexReasonGlobalNotPartitioned, "")
		}
	}
	var columns []Column
	columnsKnown := false
	if fact.action == "create_table" && ddl != nil {
		columns, columnsKnown = ddl.Columns, true
	} else if snapshot != nil && snapshot.Exists {
		if snapshot.Columns != nil {
			columns, columnsKnown = snapshot.Columns, true
		}
	} else {
		// No bound table shape — member checks wait on existence scope.
		return
	}
	// needColumnsFor collects the features whose checks were blocked on an
	// unvouched column set or a member whose column/type is unproven.
	needColumnsFor := map[string]bool{}
	findColumn := func(name string) *Column {
		for i := range columns {
			if strings.EqualFold(columns[i].Name, name) {
				return &columns[i]
			}
		}
		return nil
	}

	for _, part := range index.KeyParts {
		switch part.Kind {
		case IndexKeyPartColumn:
			column := findColumn(part.Column)
			if column == nil {
				if !columnsKnown {
					// Attribute the withheld answer to every member check
					// this part feeds — prefix/JSON legality plus the
					// kind-scoped FULLTEXT/SPATIAL type checks.
					needColumnsFor[AdvancedIndexFeaturePrefixLength] = true
					switch index.Kind {
					case IndexKindFulltext:
						needColumnsFor[AdvancedIndexFeatureFulltext] = true
					case IndexKindSpatial:
						needColumnsFor[AdvancedIndexFeatureSpatial] = true
					}
					continue
				}
				// A provably missing member column is the key-column
				// existence domain (1072), not member-legality — leave it.
				continue
			}
			base := baseColumnTypeName(column.Type)
			if base == "" {
				// The member check that needs the type decides the gap's
				// feature: a FULLTEXT member's withheld answer belongs to
				// fulltext, never to a filtered-out prefix_length gap.
				switch index.Kind {
				case IndexKindFulltext:
					needColumnsFor[AdvancedIndexFeatureFulltext] = true
				case IndexKindSpatial:
					needColumnsFor[AdvancedIndexFeatureSpatial] = true
				default:
					needColumnsFor[AdvancedIndexFeaturePrefixLength] = true
				}
				continue
			}
			// Kind-scoped family legality first — each kind owns its
			// member-type contract (MySQL 1283/1687/1252; TiDB refuses the
			// FULLTEXT/SPATIAL kinds at the product boundary, so these
			// checks only ever apply on MySQL).
			switch index.Kind {
			case IndexKindFulltext:
				if dialect == DialectMySQL && !fulltextMemberFamilyType(base) {
					assessment.appendIssue(index.Name, AdvancedIndexFeatureFulltext, fact.action, AdvancedIndexReasonFulltextColumnType, "")
				}
			case IndexKindSpatial:
				if dialect == DialectMySQL {
					if !spatialMemberFamilyType(base) {
						assessment.appendIssue(index.Name, AdvancedIndexFeatureSpatial, fact.action, AdvancedIndexReasonSpatialColumnType, "")
					} else if !column.NotNull {
						assessment.appendIssue(index.Name, AdvancedIndexFeatureSpatial, fact.action, AdvancedIndexReasonSpatialNullable, "")
					}
				}
			default:
				if base == "json" {
					// A direct JSON binding needs a generated-column path
					// (3152 on both products, even with a prefix length).
					assessment.appendIssue(index.Name, AdvancedIndexFeaturePrefixLength, fact.action, AdvancedIndexReasonJSONColumn, "")
					continue
				}
			}
			// Prefix-length legality: an authored zero is refused on every
			// kind and both products (1391 — pinned on MySQL 5.7/8.0/8.4 and
			// TiDB 8.5 including FULLTEXT); a nonzero prefix is only a
			// string-family property (1089) and must not exceed the column's
			// declared length (1089). FULLTEXT ignores a nonzero prefix on
			// MySQL (native rc:0); SPATIAL always rejects one (its geometry
			// member is not a string family). Expression members reject a
			// prefix at parse (1064), before this check ever runs.
			if part.PrefixLength != nil {
				switch {
				case *part.PrefixLength == 0:
					assessment.appendIssue(index.Name, AdvancedIndexFeaturePrefixLength, fact.action, AdvancedIndexReasonPrefixZero, "")
				case index.Kind == IndexKindFulltext:
					// MySQL accepts and ignores a nonzero FULLTEXT prefix.
				case index.Kind == IndexKindSpatial:
					// A geometry member is never a string — the product
					// reports the same 1089 refusal as a non-string prefix.
					assessment.appendIssue(index.Name, AdvancedIndexFeaturePrefixLength, fact.action, AdvancedIndexReasonPrefixNotString, "")
				case !prefixCapableFamilyType(base):
					assessment.appendIssue(index.Name, AdvancedIndexFeaturePrefixLength, fact.action, AdvancedIndexReasonPrefixNotString, "")
				case column.Length > 0 && *part.PrefixLength > column.Length:
					assessment.appendIssue(index.Name, AdvancedIndexFeaturePrefixLength, fact.action, AdvancedIndexReasonPrefixOverLength, "")
				}
			} else if index.Kind != IndexKindFulltext && index.Kind != IndexKindSpatial && prefixRequiredFamilyType(base) {
				// BLOB/TEXT members need an authored length (1170) — the
				// contract belongs to ordinary index kinds only; a legal
				// FULLTEXT text member never needs one.
				assessment.appendIssue(index.Name, AdvancedIndexFeaturePrefixLength, fact.action, AdvancedIndexReasonPrefixRequired, "")
			}
		case IndexKeyPartExpression:
			if part.MultiValued {
				// A multi-valued member's CAST target has a pinned
				// product matrix (1235): MySQL accepts signed/unsigned,
				// char(n)/binary(n), date/time/datetime, decimal; TiDB
				// accepts double instead of decimal. Pinned negatives
				// refuse; everything outside the matrix is a bounded
				// gap, not a guessed verdict.
				switch part.MultiValuedCastType {
				case "":
					assessment.appendGap(AdvancedIndexFeatureMultiValued, AdvancedReasonUnknownTableState, "target_table.indexes.key_parts.cast_type")
				case "signed", "unsigned", "char", "binary", "date", "time", "datetime":
					if (part.MultiValuedCastType == "char" || part.MultiValuedCastType == "binary") && !part.MultiValuedCastLengthSet {
						assessment.appendIssue(index.Name, AdvancedIndexFeatureMultiValued, fact.action, AdvancedIndexReasonMultiValuedCastType, "")
					}
				case "decimal":
					if dialect != DialectMySQL {
						assessment.appendIssue(index.Name, AdvancedIndexFeatureMultiValued, fact.action, AdvancedIndexReasonMultiValuedCastType, "")
					}
				case "double":
					if dialect != DialectTiDB {
						assessment.appendIssue(index.Name, AdvancedIndexFeatureMultiValued, fact.action, AdvancedIndexReasonMultiValuedCastType, "")
					}
				case "json", "year", "float":
					// Pinned negatives on both products (1235).
					assessment.appendIssue(index.Name, AdvancedIndexFeatureMultiValued, fact.action, AdvancedIndexReasonMultiValuedCastType, "")
				default:
					// A family the pinned matrix never named (vector,
					// timestamp, set, …) is unprovable, not refused.
					assessment.appendGap(AdvancedIndexFeatureMultiValued, AdvancedReasonUnknownTableState, "target_table.indexes.key_parts.cast_type")
				}
			}
			if part.Expression == nil || !part.Expression.Known {
				// Expression identity unprovable — the dependency answer
				// is withheld, not guessed.
				assessment.appendGap(AdvancedIndexFeatureExpressionKeys, AdvancedReasonUnknownTableState, "target_table.indexes.key_parts.expression")
				continue
			}
			for _, ref := range part.Expression.References {
				if ref.Name == "" || ref.Table != "" {
					// Qualified references and non-column shapes stay
					// outside the bounded dependency check.
					continue
				}
				if !columnsKnown {
					needColumnsFor[AdvancedIndexFeatureExpressionKeys] = true
					break
				}
				if findColumn(ref.Name) == nil {
					assessment.appendIssue(index.Name, AdvancedIndexFeatureExpressionKeys, fact.action, AdvancedIndexReasonExpressionColumn, "")
					break
				}
			}
		}
	}
	for feature := range needColumnsFor {
		assessment.appendGap(feature, AdvancedReasonUnknownTableState, PrimaryRoleGapColumnNullables)
	}
}

// prefixCapableFamilyType reports the column families a prefix length may
// bind — the string families plus the binary/blob families the product's
// 1089 message treats as "a string". Integer, temporal, enum/set, bit, and
// geometry members all reject an authored prefix.
func prefixCapableFamilyType(base string) bool {
	switch base {
	case "char", "varchar", "nchar", "nvarchar",
		"binary", "varbinary",
		"tinytext", "text", "mediumtext", "longtext",
		"tinyblob", "blob", "mediumblob", "longblob":
		return true
	}
	return false
}

// factMemberColumn resolves one member name against the column table the
// fact can vouch for — the statement's own declared columns on
// create_table, else the bound snapshot's catalog columns. nil when the
// column set is absent or the name is unlisted.
func factMemberColumn(fact advancedIndexFact, name string, ddl *DDL, snapshot *TableSnapshot) *Column {
	var columns []Column
	switch {
	case fact.action == "create_table" && ddl != nil:
		columns = ddl.Columns
	case snapshot != nil && snapshot.Exists:
		columns = snapshot.Columns
	}
	for i := range columns {
		if strings.EqualFold(columns[i].Name, name) {
			return &columns[i]
		}
	}
	return nil
}

// prefixRequiredFamilyType reports the BLOB/TEXT family that needs an
// explicit prefix length under MySQL-family rules (1170).
func prefixRequiredFamilyType(base string) bool {
	switch base {
	case "blob", "tinyblob", "mediumblob", "longblob",
		"text", "tinytext", "mediumtext", "longtext":
		return true
	}
	return false
}

// fulltextMemberFamilyType reports the column family a FULLTEXT member may
// bind on MySQL (1283 for anything else).
func fulltextMemberFamilyType(base string) bool {
	switch base {
	case "char", "varchar", "nchar", "nvarchar",
		"text", "tinytext", "mediumtext", "longtext":
		return true
	}
	return false
}

// spatialMemberFamilyType reports the geometry family a SPATIAL member may
// bind on MySQL (1687 for anything else).
func spatialMemberFamilyType(base string) bool {
	switch base {
	case "geometry", "point", "linestring", "polygon",
		"multipoint", "multilinestring", "multipolygon", "geometrycollection":
		return true
	}
	return false
}

// assessImplicitPrimaryVisibility applies the implicit primary-key role to
// invisibility intent on a non-PRIMARY-named fact: when no explicit primary
// key exists, the first qualifying UNIQUE index carries the role and cannot
// go invisible (3522 on both anchors). The determination is shared across
// create_table declarations, add/create-index declarations, and alter_index
// targets. Unproven role facts degrade to bounded gaps — never a fabricated
// refusal and never a fabricated pass.
func assessImplicitPrimaryVisibility(assessment *AdvancedIndexAssessment, fact advancedIndexFact, dialect Dialect, ddl *DDL, snapshot *TableSnapshot) {
	index := fact.index
	if index.Visibility != ColumnVisibilityInvisible {
		return
	}
	// unresolvedRole marks the outcome-blocking state: whether this index
	// lands depends on the withheld role answer, so no post-state may
	// publish the member as created or its invisibility as applied.
	unresolvedRole := func(role PrimaryRoleResolution) {
		assessment.appendGap(AdvancedIndexFeatureVisibility, AdvancedReasonUnknownTableState, role.GapFacts...)
		assessment.UnresolvedTransition = true
	}
	holds := false
	switch fact.action {
	case "create_table":
		role := ResolveDeclaredPrimaryRole(dialect, ddl)
		switch {
		case !role.Resolved:
			unresolvedRole(role)
			return
		case role.HasHolder:
			// Holder identity is the member pointer itself — two unnamed
			// unique members share the empty name but never share the
			// role (products auto-name and only the first qualifies).
			holds = role.HolderIndex == index
		}
	case "add_index", "add_constraint", "create_index":
		// A newly created index appends after every existing member — it can
		// hold the role only when the snapshot proves no earlier qualifying
		// candidate and no explicit primary key. An unbound/absent target is
		// the existence machinery's scope, not a role gap.
		if snapshot == nil || !snapshot.Exists || index.Kind != IndexKindUnique {
			return
		}
		role := ResolvePrimaryRole(dialect, snapshot, snapshot.IndexOrderCatalog)
		if !role.Resolved {
			unresolvedRole(role)
			return
		}
		if !role.ExplicitPrimary && !role.HasHolder {
			// Resolved no-candidate — distinct from both the explicit-PK
			// and the existing-holder states, in which an appended unique
			// can never claim the role: the declared index becomes the
			// holder exactly when it provably qualifies.
			eligible, provable := primaryRoleEligible(dialect, index, snapshot)
			switch {
			case !provable:
				unresolvedRole(PrimaryRoleResolution{GapFacts: []string{PrimaryRoleGapMemberFacts}})
			case eligible:
				holds = true
			}
		}
	case "alter_index":
		// Unbound metadata is offline scope, a confirmed-absent table fails
		// on existence — neither is a role question. Only a vouched set that
		// cannot enumerate candidates emits the member gap here; the plain
		// IndexesUnknown case already surfaces through the existence rule,
		// and a target with no proven name match under an unproven name
		// space leaves the same name lookup unresolved there.
		if snapshot == nil || !snapshot.Exists || snapshot.IndexesUnknown {
			return
		}
		target := snapshot.FindProvenIndex(fact.name)
		if target == nil {
			// A target with no proven match either fails on existence
			// (1176) or hides behind an unproven sibling name — the role
			// question never arises; the existence rule owns the
			// finding/gap either way. A proven target binds even while a
			// sibling's stored name stays unproven.
			return
		}
		if target.Kind == IndexKindUnknown &&
			(snapshot.PrimaryKeyUnknown ||
				(snapshot.PrimaryKey != nil && snapshot.PrimaryKey.NameUnknown)) {
			// An attribution-demoted candidate may BE the PK object —
			// INVISIBLE on the object refuses (3522) while on a member it
			// lands, so the outcome is unprovable while the PK identity is
			// withheld. Never silently pass and never fabricate the
			// refusal: the primary-key identity fact is what's missing.
			unresolvedRole(PrimaryRoleResolution{GapFacts: []string{PrimaryRoleGapPrimaryKey}})
			return
		}
		if target.Kind != IndexKindUnique {
			// The role question only exists for a candidate-shaped target —
			// a plain or fulltext index can never hold it, so a candidate-
			// order gap must not fire on an unrelated invisibility.
			return
		}
		role := ResolvePrimaryRole(dialect, snapshot, snapshot.IndexOrderCatalog)
		if !role.Resolved {
			unresolvedRole(role)
			return
		}
		holds = role.HasHolder && role.HolderIndex == target
	}
	if holds {
		// The refusal holds on every supported version — MySQL <8.0 rejects
		// the INVISIBLE keyword at parse, 8.0+ and TiDB answer 3522 — so the
		// issue and the blocked transition are version-free.
		assessment.appendIssue(fact.name, AdvancedIndexFeatureVisibility, fact.action, AdvancedIndexReasonPrimaryVisibility, "")
	}
}
