// Package spec defines parser-neutral advanced-index facts and assessments.
// input: normalized statements carrying index part-kind marks, visibility
// facts, and version metadata
// output: the shared AssessAdvancedIndexes answer — features, determined
// product/version issues, and bounded version gaps — consumed by the index
// compatibility rule and the ordered-state publication gate
// pos: T10 advanced-index domain model consumed by rules and the ordered-state pass
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
)

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
	Features          map[string]bool
	VersionGated      map[string]bool
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

// indexFeatures returns the advanced-index forms one fact carries.
func indexFeatures(fact advancedIndexFact) []string {
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
	if statement.Metadata != nil {
		version = statement.Metadata.Version
	}

	facts := statementIndexFacts(ddl)
	var gated []string
	for _, fact := range facts {
		for _, feature := range indexFeatures(fact) {
			if !assessment.Features[feature] {
				gated = append(gated, feature)
			}
			assessment.Features[feature] = true
			assessment.VersionGated[feature] = true
		}
	}
	// Every advanced-index feature consults the target version on both
	// dialects: MySQL forms gate on their own minimums, TiDB forms gate on
	// the validated series. One bounded version gap per statement.
	if len(gated) > 0 {
		switch {
		case version == nil:
			assessment.appendGap("", AdvancedReasonMissingTargetVersion, AdvancedFactTargetVersion)
		case !version.ValidatedRange:
			assessment.appendGap("", AdvancedReasonOutOfValidatedRange, AdvancedFactTargetVersionRange)
		}
	}
	versionKnown := version != nil && version.ValidatedRange

	for _, fact := range facts {
		assessIndexFact(&assessment, fact, version, versionKnown, statement.Dialect)
	}
	assessment.Applies = len(assessment.Features) > 0
	return assessment
}

// assessIndexFact evaluates one index fact against the product/version
// responsibility table.
func assessIndexFact(assessment *AdvancedIndexAssessment, fact advancedIndexFact, version *VersionIdentity, versionKnown bool, dialect Dialect) {
	index := fact.index
	if index == nil {
		return
	}
	features := indexFeatures(fact)
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
	// is rejected at 3522 on both products; ALTER INDEX `PRIMARY` on MySQL
	// is the same refusal class (TiDB answers it through existence — PRIMARY
	// is not an index there). An authored VISIBLE attribute on the primary
	// key is a natively-accepted no-op, not a refusal — only invisibility
	// fires. The issue only fires where the product answer is proven: an
	// in-range version.
	if index.Kind == IndexKindPrimary && index.Visibility == ColumnVisibilityInvisible {
		if versionKnown {
			assessment.appendIssue(fact.name, AdvancedIndexFeatureVisibility, fact.action, AdvancedIndexReasonPrimaryVisibility, "")
		}
		return
	}
	if fact.action == "alter_index" && strings.EqualFold(fact.name, "primary") {
		if dialect == DialectMySQL && versionKnown && versionAtLeast(version, advancedIndexMinimumVisibilityMySQL) {
			assessment.appendIssue(fact.name, AdvancedIndexFeatureVisibility, fact.action, AdvancedIndexReasonPrimaryVisibility, "")
		}
	}
}
