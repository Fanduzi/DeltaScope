// Package ddl defines Tier-1 DDL rules.
// input: normalized statements carrying T10 index visibility declarations
// and advanced-index member marks, plus metadata-enriched ordered snapshots
// output: invisibility-forbid findings, dialect-aware name-keyed index
// existence findings and evidence gaps shared by alter/drop/rename_index
// (member-first `primary` precedence — a TiDB member may own the name;
// else MySQL primary key / TiDB PK presence for drop or real object for
// alter/rename vs PKIsHandle — an unproven handle answer or unproven member
// name space reports the primary_key/indexes identity gap instead of a
// fabricated absence), and product/version compatibility findings and gaps for
// visibility, expression keys, descending keys, multi-valued members,
// prefix length, fulltext, spatial, and global forms — including the
// implicit-primary role refusal on qualifying UNIQUE indexes
// pos: T10 advanced-index and visibility rule implementations
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"context"
	"fmt"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// indexInvisibilitySubject binds one index surface that declares or requests
// invisibility to the action that carries it.
type indexInvisibilitySubject struct {
	action string
	name   string
}

// invisibilitySubjects returns every invisibility event the statement
// declares: create-table index/PK attributes, standalone CREATE INDEX and
// ALTER ADD INDEX options, and ALTER INDEX ... INVISIBLE transitions.
// ALTER INDEX ... VISIBLE is not an invisibility event.
func invisibilitySubjects(statement spec.Statement) []indexInvisibilitySubject {
	var out []indexInvisibilitySubject
	if statement.DDL == nil {
		return out
	}
	add := func(index *spec.Index, action string) {
		if index == nil || index.Visibility != spec.ColumnVisibilityInvisible {
			return
		}
		out = append(out, indexInvisibilitySubject{
			action: action,
			name:   strings.TrimSpace(index.Name),
		})
	}
	switch statement.DDL.Operation {
	case spec.DDLOperationCreateTable:
		add(statement.DDL.PrimaryKey, "create_table")
		for i := range statement.DDL.Indexes {
			add(&statement.DDL.Indexes[i], "create_table")
		}
	case spec.DDLOperationCreateIndex, spec.DDLOperationAlterTable:
		for i := range statement.DDL.Alter {
			alter := &statement.DDL.Alter[i]
			if alter.Index == nil {
				continue
			}
			switch alter.Action {
			case "create_index", "add_index", "add_constraint":
				add(alter.Index.Definition, alter.Action)
			case "alter_index":
				definition, ok := alterIndexDefinition(*alter)
				if !ok || definition.Visibility != spec.ColumnVisibilityInvisible {
					continue
				}
				out = append(out, indexInvisibilitySubject{
					action: alter.Action,
					name:   alterIndexTargetName(*alter),
				})
			}
		}
	}
	return out
}

// indexInvisibleForbidRule rejects any index the statement hides — a declared
// INVISIBLE attribute or an ALTER INDEX ... INVISIBLE transition — with a
// configurable policy level.
type indexInvisibleForbidRule struct {
	ruleID string
	level  rule.Level
}

func newIndexInvisibleForbidRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return indexInvisibleForbidRule{
		ruleID: ruleIDIndexInvisibleForbid,
		level:  configuredLevel(cfg, rule.LevelBlocker),
	}, nil
}

func (r indexInvisibleForbidRule) ID() string { return r.ruleID }

func (r indexInvisibleForbidRule) AppliesTo(statement spec.Statement) bool {
	return len(invisibilitySubjects(statement)) > 0
}

func (r indexInvisibleForbidRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	snapshot, _ := targetTableSnapshot(statement)
	table := metadataTargetTableName(statement, snapshot)
	var findings []rule.Finding
	for _, subject := range invisibilitySubjects(statement) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		findings = append(findings, rule.Finding{
			Level:   r.level,
			Message: fmt.Sprintf("index %q is declared or transitioned invisible on table %q", subject.name, table),
			Metadata: map[string]any{
				"table":      table,
				"index":      subject.name,
				"action":     subject.action,
				"visibility": spec.ColumnVisibilityInvisible,
			},
		})
	}
	return findings, nil
}

// indexPrimaryMemberMayOwnName reports whether a stored member literally
// named `primary` could own the name — only TiDB reaches that state (RENAME
// … TO `primary` is legal on PK-less and PKIsHandle tables), and only while
// no real PRIMARY index object provably occupies the name space. When the
// member collection is unvouched the possibility cannot be ruled out, so
// existence answers stay suppressed instead of asserting member-first or
// PK-first resolution.
func indexPrimaryMemberMayOwnName(statement spec.Statement, snapshot *spec.TableSnapshot) bool {
	if snapshot == nil {
		return true
	}
	if statement.Dialect == spec.DialectMySQL {
		// MySQL reserves `primary` for the PK object outright — a member
		// can never own it.
		return false
	}
	if pkName, proven := spec.PrimaryKeyObjectName(snapshot); proven &&
		strings.EqualFold(pkName, "primary") &&
		!snapshot.PrimaryKeyHandleUnknown && !snapshot.PrimaryKeyIsHandle {
		// A real PRIMARY index object provably occupies the name — the
		// 1061 refusal rules out any member `primary` regardless of the
		// member-space vouchers (verified v8.5.0: rename TO `primary`
		// refused while a real object holds it). A synthesized handle
		// row does not occupy it — member `primary` coexists legally.
		return false
	}
	// A member `primary` may own the name exactly while the member name
	// space cannot vouch every stored name — a proven member `primary`
	// is already answered by the member-set check above.
	return snapshot.IndexNameSpaceUnvouched()
}

// alterIndexTargetExists resolves the ALTER INDEX target against the
// ordered snapshot through the shared actual-name binding: a proven member
// or a real PK object under its stored name (a renamed pk2, or `primary`
// itself) is a present, operable target; a handle PK's synthesized
// `primary` fold names no operable object (1176, absent like a missing
// member); a provably free name is absent; and an ambiguous binding
// suppresses the blocker — the rule's EvidenceGaps reports the missing
// fact instead of fabricating either answer.
func alterIndexTargetExists(statement spec.Statement, snapshot *spec.TableSnapshot, name string) bool {
	return operableIndexTargetExists(statement, snapshot, name)
}

// operableIndexTargetExists answers the ALTER/RENAME INDEX target classes —
// both operate on a real index object, so they share one binding answer.
func operableIndexTargetExists(statement spec.Statement, snapshot *spec.TableSnapshot, name string) bool {
	if snapshotHasIndex(statement, snapshot, name) {
		return true
	}
	if statement.Dialect != spec.DialectMySQL && statement.Dialect != spec.DialectTiDB {
		return false
	}
	switch spec.ResolveIndexNameBinding(statement.Dialect, snapshot, name) {
	case spec.IndexNameBindsMember, spec.IndexNameBindsPKObject:
		return true
	case spec.IndexNameBindsAmbiguous:
		return true
	default:
		// IndexNameBindsNone — provably free — and IndexNameBindsPKIdentity
		// — a handle PK's `primary` row owns no operable object (1176).
		return false
	}
}

// dropIndexTargetExists resolves the DROP INDEX target through the shared
// binding. The PK identity counts for drop — `primary` names the primary
// key itself on both anchors (MySQL drops it outright; TiDB routes the
// statement to the drop-primary-key outcome, 8200 clustered) — while a PK
// renamed off `primary` frees the name (1091 on the freed name). An
// ambiguous binding suppresses the blocker instead of fabricating an
// absence.
func dropIndexTargetExists(statement spec.Statement, snapshot *spec.TableSnapshot, name string) bool {
	if snapshotHasIndex(statement, snapshot, name) {
		return true
	}
	if statement.Dialect != spec.DialectMySQL && statement.Dialect != spec.DialectTiDB {
		return false
	}
	switch spec.ResolveIndexNameBinding(statement.Dialect, snapshot, name) {
	case spec.IndexNameBindsNone:
		return false
	default:
		// Member, real PK object, PK identity all exist for a drop;
		// an ambiguous binding suppresses the fabricated-absence blocker.
		return true
	}
}

// renameIndexTargetExists resolves the RENAME INDEX source name through the
// shared binding — same operable-object semantics as ALTER: a handle PK's
// `primary` row names no operable object (1176, absent like a missing
// member), while a renamed PK object's actual stored name binds it.
func renameIndexTargetExists(statement spec.Statement, snapshot *spec.TableSnapshot, name string) bool {
	return operableIndexTargetExists(statement, snapshot, name)
}

// indexExistenceRule reuses the shared existence frame for every
// name-keyed index existence check — ALTER, DROP, and RENAME alike — and
// reports the extra primary_key fact an unvouched PRIMARY target leaves
// behind.
type indexExistenceRule struct {
	inner alterObjectExistenceRule
}

func newIndexExistenceRule(ruleID string, actions []string, cfg policy.RulePolicy,
	checkExists func(spec.Statement, *spec.TableSnapshot, string) bool) (rule.StatementRule, error) {
	inner, err := newAlterObjectExistenceRule(
		ruleID, actions,
		"index", false, rule.LevelBlocker, cfg, alterIndexTargetName, checkExists)
	if err != nil {
		return nil, err
	}
	exists, ok := inner.(alterObjectExistenceRule)
	if !ok {
		return nil, fmt.Errorf("%s: constructor returned %T, want alterObjectExistenceRule", ruleID, inner)
	}
	// Every variant resolves `primary` through the primary-key identity
	// facts — that target needs no member name space.
	exists.primaryResolves = true
	return indexExistenceRule{inner: exists}, nil
}

func newAlterIndexExistsRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newIndexExistenceRule(ruleIDAlterIndexExistsRequire, []string{"alter_index"}, cfg, alterIndexTargetExists)
}

func (r indexExistenceRule) ID() string { return r.inner.ruleID }

func (r indexExistenceRule) AppliesTo(statement spec.Statement) bool {
	return r.inner.AppliesTo(statement)
}

func (r indexExistenceRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	return r.inner.Evaluate(ctx, statement)
}

// EvidenceGaps appends a target_table.primary_key gap when a `primary`
// target cannot be judged because the primary-key identity is unvouched —
// an unknown member collection on either dialect, or an unproven TiDB
// handle answer (PKIsHandle vs real object). The suppressed finding must
// surface as a gap, not disappear. A `primary` target resolves through
// the primary-key identity facts and never consults the member name space
// when the name is provably bound to the PK — always on MySQL, and on
// TiDB exactly while a real PRIMARY index object occupies it — so an
// unvouched index collection emits no spurious target_table.indexes gap
// for an all-`primary` target list it provably could not consult. A
// non-`primary` target keeps its own gap.
func (r indexExistenceRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	gaps := r.inner.EvidenceGaps(statement)
	if statement.Dialect != spec.DialectMySQL && statement.Dialect != spec.DialectTiDB {
		return gaps
	}
	hasPrimary, allPrimary := false, true
	for _, alter := range matchingAlterObjectActions(statement, r.inner.actions...) {
		name := alterIndexTargetName(alter)
		if name == "" {
			continue
		}
		if strings.EqualFold(name, "primary") {
			hasPrimary = true
		} else {
			allPrimary = false
		}
	}
	snapshot, ok := targetTableSnapshot(statement)
	if hasPrimary {
		if !ok || snapshot == nil {
			if !allPrimary {
				// Ordinary targets still owe the member collection fact;
				// the `primary` target owes the primary-key identity fact.
				return append(gaps, rule.EvidenceGap{
					ReasonCode:    gapReasonUnknownTableState,
					RequiredFacts: []string{"target_table.primary_key"},
				})
			}
			// An all-`primary` target list on an unprovable table owes the
			// primary-key identity fact in place of the member collection —
			// MySQL never needs the member set (the reserved name binds only
			// the PK); TiDB keeps it alongside because a member `primary`
			// may own the name while the whole table state is unproven.
			return rewritePrimaryExistenceGapFacts(gaps, statement.Dialect)
		}
		if snapshot.Exists && allPrimary && memberCollectionUnknown(snapshot, "index") &&
			!indexPrimaryMemberMayOwnName(statement, snapshot) {
			gaps = dropRequiredFactGap(gaps, "target_table.indexes")
		}
	}
	if !ok || snapshot == nil || !snapshot.Exists {
		return gaps
	}
	// A name-keyed target owes the primary-key identity fact exactly while
	// the shared binding still needs the PK side: a proven member owning
	// the name answers without consulting the PK at all, and a provably
	// PK-bound or provably free name is already decided — the flag check
	// alone cannot see those because ambiguity also comes from an
	// unvouched member space or a withheld PK object name under a vouched
	// presence. The PK side can only own the target while it may carry a
	// non-`primary` name — a TiDB object renamed off `primary` — or the
	// target is `primary` itself; on MySQL the PK name is always `primary`
	// (rename refused), so member-keyed ambiguity there is purely
	// member-space sourced and stays with the member-collection fact the
	// inner projection already owes.
	for _, alter := range matchingAlterObjectActions(statement, r.inner.actions...) {
		name := alterIndexTargetName(alter)
		if name == "" ||
			spec.ResolveIndexNameBinding(statement.Dialect, snapshot, name) != spec.IndexNameBindsAmbiguous {
			continue
		}
		if !strings.EqualFold(name, "primary") && statement.Dialect != spec.DialectTiDB {
			continue
		}
		if snapshot.PrimaryKeyUnknown ||
			snapshot.PrimaryKeyHandleUnknown ||
			(snapshot.PrimaryKey != nil && snapshot.PrimaryKey.NameUnknown) {
			return append(gaps, rule.EvidenceGap{
				ReasonCode:    gapReasonUnknownTableState,
				RequiredFacts: []string{"target_table.primary_key"},
			})
		}
	}
	return gaps
}

// rewritePrimaryExistenceGapFacts trades the member-collection fact inside
// the unknown-state gap for the primary-key identity fact — the shape an
// all-`primary` target list owes on an unprovable table. MySQL drops the
// member fact outright; TiDB keeps it alongside primary_key because a
// member `primary` may own the name while nothing about the table is
// proven.
func rewritePrimaryExistenceGapFacts(gaps []rule.EvidenceGap, dialect spec.Dialect) []rule.EvidenceGap {
	out := make([]rule.EvidenceGap, 0, len(gaps))
	for _, gap := range gaps {
		facts := make([]string, 0, len(gap.RequiredFacts)+1)
		for _, fact := range gap.RequiredFacts {
			if fact != "target_table.indexes" {
				facts = append(facts, fact)
				continue
			}
			if dialect == spec.DialectTiDB {
				facts = append(facts, fact)
			}
			facts = append(facts, "target_table.primary_key")
		}
		out = append(out, rule.EvidenceGap{ReasonCode: gap.ReasonCode, RequiredFacts: facts})
	}
	return out
}

// dropRequiredFactGap removes the gap whose required_facts list is exactly
// the single named fact — used when a target resolves through an
// independent fact chain, so the collection gap it never needed is
// spurious. A gap naming the fact alongside others stays: it carries
// evidence this rule did consult.
func dropRequiredFactGap(gaps []rule.EvidenceGap, fact string) []rule.EvidenceGap {
	kept := make([]rule.EvidenceGap, 0, len(gaps))
	for _, gap := range gaps {
		if len(gap.RequiredFacts) == 1 && gap.RequiredFacts[0] == fact {
			continue
		}
		kept = append(kept, gap)
	}
	return kept
}

var indexAdvancedCompatibilityFeatures = map[string]bool{
	spec.AdvancedIndexFeatureVisibility:     true,
	spec.AdvancedIndexFeatureExpressionKeys: true,
	spec.AdvancedIndexFeatureDescendingKeys: true,
	spec.AdvancedIndexFeatureMultiValued:    true,
	spec.AdvancedIndexFeaturePrefixLength:   true,
	spec.AdvancedIndexFeatureFulltext:       true,
	spec.AdvancedIndexFeatureSpatial:        true,
	spec.AdvancedIndexFeatureGlobal:         true,
}

// indexAdvancedCompatibilityRule enforces the frozen T10 product/version
// responsibility table for index visibility, expression keys, descending
// keys, multi-valued members, and the member-legality families
// prefix_length, fulltext, spatial, and global (primary-key invisibility
// and TiDB PRIMARY-object identity included). Determined failures become
// findings; missing version/fact evidence becomes a bounded gap, never a
// silent pass.
type indexAdvancedCompatibilityRule struct {
	ruleID   string
	required bool
	level    rule.Level
	features map[string]bool
}

func newIndexAdvancedCompatibilityRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	required, err := boolParam(ruleIDIndexAdvancedCompatibilityRequire, cfg, "required", true)
	if err != nil {
		return nil, err
	}
	features := map[string]bool{
		spec.AdvancedIndexFeatureVisibility:     true,
		spec.AdvancedIndexFeatureExpressionKeys: true,
		spec.AdvancedIndexFeatureDescendingKeys: true,
		spec.AdvancedIndexFeatureMultiValued:    true,
	}
	if raw, ok := cfg.Params["features"]; ok {
		features = map[string]bool{}
		var names []string
		switch list := raw.(type) {
		case []string:
			names = list
		case []any:
			for _, item := range list {
				name, ok := item.(string)
				if !ok {
					return nil, fmt.Errorf("rule %s param %q entries must be strings, got %T", ruleIDIndexAdvancedCompatibilityRequire, "features", item)
				}
				names = append(names, name)
			}
		default:
			return nil, fmt.Errorf("rule %s param %q must be a list of feature names, got %T", ruleIDIndexAdvancedCompatibilityRequire, "features", raw)
		}
		for _, name := range names {
			if !indexAdvancedCompatibilityFeatures[name] {
				return nil, fmt.Errorf("rule %s param %q has unknown feature %q", ruleIDIndexAdvancedCompatibilityRequire, "features", name)
			}
			features[name] = true
		}
	}
	return indexAdvancedCompatibilityRule{
		ruleID:   ruleIDIndexAdvancedCompatibilityRequire,
		required: required,
		level:    configuredLevel(cfg, rule.LevelBlocker),
		features: features,
	}, nil
}

func (r indexAdvancedCompatibilityRule) ID() string { return r.ruleID }

func (r indexAdvancedCompatibilityRule) featureEnabled(feature string) bool {
	return r.features[feature]
}

func (r indexAdvancedCompatibilityRule) AppliesTo(statement spec.Statement) bool {
	if !r.required || len(r.features) == 0 {
		return false
	}
	assessment := spec.AssessAdvancedIndexes(statement)
	for feature := range assessment.Features {
		if r.featureEnabled(feature) {
			return true
		}
	}
	return false
}

func (r indexAdvancedCompatibilityRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	assessment := spec.AssessAdvancedIndexes(statement)
	snapshot, _ := targetTableSnapshot(statement)
	table := metadataTargetTableName(statement, snapshot)
	targetVersion := ""
	if statement.Metadata != nil && statement.Metadata.Version != nil {
		targetVersion = statement.Metadata.Version.Version
	}
	var findings []rule.Finding
	for _, issue := range assessment.Issues {
		if !r.featureEnabled(issue.Feature) {
			continue
		}
		findings = append(findings, rule.Finding{
			Level:   r.level,
			Message: fmt.Sprintf("index %q advanced form is not supported for the target (%s)", issue.Index, issue.ReasonCode),
			Metadata: map[string]any{
				"table":           table,
				"index":           issue.Index,
				"feature":         issue.Feature,
				"action":          issue.Action,
				"reason_code":     issue.ReasonCode,
				"product":         spec.VersionProductForDialect(statement.Dialect),
				"target_version":  targetVersion,
				"minimum_version": issue.MinimumVersion,
			},
		})
	}
	return findings, nil
}

// EvidenceGaps projects feature-scoped compatibility gaps: version gaps apply
// only when an enabled feature consults the version, and feature-specific
// gaps stay inside their enabled family.
func (r indexAdvancedCompatibilityRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(statement) {
		return nil
	}
	assessment := spec.AssessAdvancedIndexes(statement)
	gated := false
	for feature := range assessment.VersionGated {
		if r.featureEnabled(feature) {
			gated = true
			break
		}
	}
	var gaps []rule.EvidenceGap
	for _, gap := range assessment.Gaps {
		versionGap := gap.ReasonCode == spec.AdvancedReasonMissingTargetVersion ||
			gap.ReasonCode == spec.AdvancedReasonOutOfValidatedRange
		if versionGap && !gated {
			continue
		}
		if gap.Feature != "" && !r.featureEnabled(gap.Feature) {
			continue
		}
		gaps = append(gaps, rule.EvidenceGap{
			ReasonCode:    gap.ReasonCode,
			RequiredFacts: append([]string(nil), gap.RequiredFacts...),
		})
	}
	return gaps
}
