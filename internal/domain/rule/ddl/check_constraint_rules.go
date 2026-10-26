package ddl

import (
	"context"
	"fmt"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// T12 CHECK-constraint rules. Native anchors (probed):
//   - MySQL 8.0.46/8.4.10: CHECK enforces by default (3819 on violation);
//     ALTER CHECK|CONSTRAINT x NOT ENFORCED lands ENFORCED=NO and is
//     idempotent, ENFORCED is data-conditional (3819 refuses with violating
//     rows); missing/non-check names refuse 3821/3940; a secondary index
//     is outside the constraint namespace under both spellings; the
//     generic keyword binds {check,unique,pk,fk} and refuses a cross-type
//     same-name pair 3939; DROP COLUMN silently drops single-referencing
//     checks and refuses 3959 on multi-column ones.
//   - MySQL 8.0.17 (band anchor): ALTER|DROP CHECK executes; the generic
//     CONSTRAINT alias refuses 1064 — the 8.0.19 boundary applies to the
//     alias spelling only.
//   - MySQL 5.7.44: bare CHECK declarations parse but are silently
//     dropped; an authored ENFORCED/NOT ENFORCED clause and any
//     ALTER|DROP spelling refuse 1064.
//   - TiDB v8.5.0: tidb_enable_check_constraint gates everything — OFF
//     parses and ignores (nothing stored, all ops rc=0 no-ops), ON stores
//     and enforces with MySQL-equivalent semantics (3940 refusal class);
//     the generic keyword binds the CHECK member first without the
//     ambiguity refusal MySQL emits.

// gapReasonUnprovenStatementFact marks a fact owned by the statement text
// itself — the authored CHECK|CONSTRAINT spelling or an authored [NOT]
// ENFORCED clause — that the parser adaptation layer could not recover,
// so version, binding, and ignored-or-refused answers that depend on it
// stay undecidable.
const gapReasonUnprovenStatementFact = "unproven_statement_fact"

// checkArm resolves the statement's CHECK arm from the enriched facts.
func checkArm(statement spec.Statement) spec.CheckArm {
	var instance *spec.InstanceFacts
	var version *spec.VersionIdentity
	if statement.Metadata != nil {
		instance = statement.Metadata.Instance
		version = statement.Metadata.Version
	}
	return spec.ResolveCheckArm(statement.Dialect, version, instance)
}

// checkArmGap is the evidence gap an undecidable arm owes: the missing
// governing fact, attributed to instance or version.
func checkArmGap(statement spec.Statement) []rule.EvidenceGap {
	switch statement.Dialect {
	case spec.DialectTiDB:
		return []rule.EvidenceGap{{
			ReasonCode:    gapReasonMissingInstanceFact,
			RequiredFacts: []string{"instance.tidb_enable_check_constraint"},
		}}
	case spec.DialectMySQL:
		return []rule.EvidenceGap{{
			ReasonCode:    gapReasonMissingTargetVersion,
			RequiredFacts: []string{factTargetVersion},
		}}
	default:
		return nil
	}
}

// declaredNotEnforcedChecks collects the check members a statement declares
// in the not-enforced state: CREATE table constraints and column checks,
// ALTER add_constraint payloads, and in-column checks on column-defining
// actions.
func declaredNotEnforcedChecks(statement spec.Statement) []spec.Constraint {
	if statement.DDL == nil {
		return nil
	}
	var out []spec.Constraint
	collect := func(members []spec.Constraint) {
		for i := range members {
			if members[i].Type == "check" && members[i].Enforced == "not_enforced" {
				out = append(out, members[i])
			}
		}
	}
	if statement.DDL.Operation == spec.DDLOperationCreateTable {
		collect(statement.DDL.Constraints)
	}
	for _, alter := range statement.DDL.Alter {
		if alter.Constraint != nil && alter.Constraint.Type == "check" && alter.Constraint.Enforced == "not_enforced" {
			out = append(out, *alter.Constraint)
		}
		collect(alter.Constraints)
	}
	return out
}

// alterCheckActions returns the alter_check actions on the statement —
// ALTER CHECK and its ALTER CONSTRAINT alias share the normalized action.
func alterCheckActions(statement spec.Statement) []spec.Alter {
	return matchingAlterActions(statement, "alter_check")
}

// checkNotEnforcedNoticeRule reports a CHECK left in the disabled-enforcement
// state — declared NOT ENFORCED or flipped by ALTER CHECK|CONSTRAINT ... NOT
// ENFORCED — on arms where the state is real. The ENFORCED direction, ignored
// arms, and ordinary column changes never fire it.
type checkNotEnforcedNoticeRule struct {
	level rule.Level
}

func newCheckNotEnforcedNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return &checkNotEnforcedNoticeRule{level: configuredLevel(cfg, rule.LevelNotice)}, nil
}

func (r *checkNotEnforcedNoticeRule) ID() string { return ruleIDConstraintCheckNotEnforcedNotice }

func (r *checkNotEnforcedNoticeRule) AppliesTo(statement spec.Statement) bool {
	if statement.Dialect != spec.DialectMySQL && statement.Dialect != spec.DialectTiDB {
		return false
	}
	for _, alter := range alterCheckActions(statement) {
		if alter.Options["check_enforced"] == "false" {
			return true
		}
	}
	return len(declaredNotEnforcedChecks(statement)) > 0
}

func (r *checkNotEnforcedNoticeRule) Evaluate(_ context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) || checkArm(statement) != spec.CheckArmEnforced {
		return nil, nil
	}
	snapshot, _ := targetTableSnapshot(statement)
	tableName := metadataTargetTableName(statement, snapshot)
	findings := make([]rule.Finding, 0)
	for _, alter := range alterCheckActions(statement) {
		if alter.Options["check_enforced"] != "false" {
			continue
		}
		// A provably missing or non-check name means the flip provably
		// refused (MySQL 3821 / TiDB 3940) — no enforcement change can be
		// claimed. An unvouched name space cannot prove refusal, so the
		// declared intent still surfaces.
		if snapshot != nil && snapshot.Exists && !snapshot.CheckNameSpaceUnvouched() &&
			snapshot.FindCheckConstraint(strings.TrimSpace(alter.Name)) == nil {
			continue
		}
		findings = append(findings, rule.Finding{
			Level:      r.level,
			Message:    fmt.Sprintf("check constraint %q on table %q is switched to NOT ENFORCED", alter.Name, tableName),
			Suggestion: "keep check constraints enforced, or confirm the disabled state is intentional",
			Metadata: map[string]any{
				"table":      tableName,
				"constraint": alter.Name,
				"action":     alter.Action,
				"enforced":   "not_enforced",
			},
		})
	}
	for _, member := range declaredNotEnforcedChecks(statement) {
		findings = append(findings, rule.Finding{
			Level:      r.level,
			Message:    fmt.Sprintf("check constraint %q on table %q is declared NOT ENFORCED", member.Name, tableName),
			Suggestion: "keep check constraints enforced, or confirm the disabled state is intentional",
			Metadata: map[string]any{
				"table":      tableName,
				"constraint": member.Name,
				"enforced":   "not_enforced",
			},
		})
	}
	return findings, nil
}

func (r *checkNotEnforcedNoticeRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(statement) || checkArm(statement) != spec.CheckArmUnknown {
		return nil
	}
	return checkArmGap(statement)
}

// alterCheckExistsRule requires an ALTER CHECK|CONSTRAINT target name to
// bind a proven CHECK member: a missing name or a proven non-check name is
// a refused statement (MySQL 3821 / TiDB 3940). Refusing arms (MySQL <8.0.19
// syntax) and no-op arms (TiDB flag OFF) answer nothing; unproven member or
// name space yields a gap, never a fabricated finding.
type alterCheckExistsRule struct {
	level rule.Level
}

func newAlterCheckExistsRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return &alterCheckExistsRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r *alterCheckExistsRule) ID() string { return ruleIDAlterCheckExistsRequire }

func (r *alterCheckExistsRule) AppliesTo(statement spec.Statement) bool {
	return (statement.Dialect == spec.DialectMySQL || statement.Dialect == spec.DialectTiDB) &&
		len(alterCheckActions(statement)) > 0
}

// checkActionOutcome classifies whether one alter_check|drop_check action
// reaches target binding on the resolved arm — the spelling-aware version
// gate lives here so Evaluate, EvidenceGaps, and ordered state share one
// answer. MySQL needs the per-spelling floor (CHECK 8.0.16, generic
// CONSTRAINT 8.0.19, unproven spelling in-band undecidable); TiDB needs
// the flag ON; unknown arms are undecidable.
func checkActionOutcome(statement spec.Statement, alter spec.Alter) (executable, undecidable bool) {
	var version *spec.VersionIdentity
	if statement.Metadata != nil {
		version = statement.Metadata.Version
	}
	switch statement.Dialect {
	case spec.DialectMySQL:
		arm := checkArm(statement)
		if arm == spec.CheckArmUnknown {
			return false, true
		}
		if arm == spec.CheckArmIgnored {
			return false, false
		}
		switch spec.CheckAlterSyntaxOutcome(statement.Dialect, version, alter.Options["check_spelling"]) {
		case spec.CheckSyntaxSupported:
			return true, false
		case spec.CheckSyntaxUnproven:
			return false, true
		default:
			return false, false
		}
	case spec.DialectTiDB:
		switch checkArm(statement) {
		case spec.CheckArmEnforced:
			return true, false
		case spec.CheckArmUnknown:
			return false, true
		default:
			return false, false
		}
	default:
		return false, false
	}
}

// checkDropTargetBinding / checkAlterTargetBinding delegate to the shared
// spec-layer classifier — the ordered-state machine consumes the same
// answers so findings, gaps, and post-state never diverge.
func checkDropTargetBinding(dialect spec.Dialect, snapshot *spec.TableSnapshot, alter spec.Alter, name string) spec.CheckTargetBinding {
	return spec.CheckDropTarget(dialect, snapshot, alter.Options["check_spelling"], name).Binding
}

func checkAlterTargetBinding(dialect spec.Dialect, snapshot *spec.TableSnapshot, alter spec.Alter, name string) spec.CheckTargetBinding {
	return spec.CheckAlterTarget(dialect, snapshot, alter.Options["check_spelling"], name).Binding
}

// checkBindingGapFacts attributes the member/identity facts a
// CheckTargetUnproven binding actually lacked: each unvouched name space
// the spelling consulted, the indexes fact when a proven name carries an
// unproven stored kind, and target_table.primary_key when an unproven PK
// presence may hide a `primary` binding. The CHECK spelling only ever
// consults the check name space. A binding left unproven by a semantic
// boundary alone (TiDB's non-check-only arm) owes the broad pair it
// always did — no member fact is missing there.
func checkBindingGapFacts(snapshot *spec.TableSnapshot, spelling, name string) []string {
	facts := []string{}
	add := func(fact string) {
		for _, existing := range facts {
			if existing == fact {
				return
			}
		}
		facts = append(facts, fact)
	}
	if snapshot.CheckNameSpaceUnvouched() {
		add("target_table.constraints")
	}
	if spelling != spec.CheckSpellingCheck {
		if snapshot.ForeignKeyNameSpaceUnvouched() {
			add("target_table.constraints")
		}
		if snapshot.IndexNameSpaceUnvouched() {
			add("target_table.indexes")
		} else if member := snapshot.FindProvenIndex(name); member != nil {
			switch member.Kind {
			case spec.IndexKindUnique, spec.IndexKindPrimary, spec.IndexKindSecondary:
			default:
				add("target_table.indexes")
			}
		}
		if strings.EqualFold(name, "primary") && snapshot.PrimaryKeyUnknown {
			add("target_table.primary_key")
		}
	}
	if len(facts) == 0 {
		return []string{"target_table.constraints", "target_table.indexes"}
	}
	return facts
}

func (r *alterCheckExistsRule) Evaluate(_ context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	snapshot, ok := targetTableSnapshot(statement)
	if !ok || snapshot == nil || !snapshot.Exists {
		return nil, nil
	}
	tableName := metadataTargetTableName(statement, snapshot)
	findings := make([]rule.Finding, 0)
	for _, alter := range alterCheckActions(statement) {
		executable, undecidable := checkActionOutcome(statement, alter)
		if !executable || undecidable {
			continue
		}
		name := strings.TrimSpace(alter.Name)
		if name == "" {
			continue
		}
		binding := checkAlterTargetBinding(statement.Dialect, snapshot, alter, name)
		var message string
		switch binding {
		case spec.CheckTargetMissing:
			message = fmt.Sprintf("check constraint %q does not exist on table %q", name, tableName)
		case spec.CheckTargetWrongKind:
			message = fmt.Sprintf("constraint %q on table %q exists but is not a check constraint — the generic spelling cannot alter its enforcement", name, tableName)
		case spec.CheckTargetAmbiguous:
			message = fmt.Sprintf("constraint name %q on table %q is shared by multiple constraint kinds — the generic spelling refuses ambiguous targets", name, tableName)
		default:
			continue
		}
		findings = append(findings, rule.Finding{
			Level:      r.level,
			Message:    message,
			Suggestion: "fix the constraint name or use the CHECK-specific keyword, and refresh metadata before auditing this change",
			Metadata: map[string]any{
				"table":  tableName,
				"action": alter.Action,
				"name":   name,
				"exists": false,
			},
		})
	}
	return findings, nil
}

func (r *alterCheckExistsRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(statement) {
		return nil
	}
	for _, alter := range alterCheckActions(statement) {
		_, undecidable := checkActionOutcome(statement, alter)
		if undecidable {
			return checkArmGap(statement)
		}
	}
	snapshot, ok := targetTableSnapshot(statement)
	if !ok || snapshot == nil || !snapshot.Exists {
		return []rule.EvidenceGap{{
			ReasonCode:    gapReasonUnknownTableState,
			RequiredFacts: []string{"target_table.existence"},
		}}
	}
	for _, alter := range alterCheckActions(statement) {
		executable, _ := checkActionOutcome(statement, alter)
		if !executable {
			continue
		}
		name := strings.TrimSpace(alter.Name)
		if name == "" {
			continue
		}
		if checkAlterTargetBinding(statement.Dialect, snapshot, alter, name) == spec.CheckTargetUnproven {
			return []rule.EvidenceGap{{
				ReasonCode:    gapReasonUnknownTableState,
				RequiredFacts: checkBindingGapFacts(snapshot, alter.Options["check_spelling"], name),
			}}
		}
	}
	return nil
}

// dropCheckExistsRule requires a DROP CHECK|CONSTRAINT target name to bind
// some removable member the proven collections vouch for — a check, foreign
// key, index member, or the primary-key object. Both keywords refuse a
// missing name (MySQL 3821/3940, TiDB 3940); unvouched collections answer
// with a gap, never a confirmed absence.
type dropCheckExistsRule struct {
	level rule.Level
}

func newDropCheckExistsRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return &dropCheckExistsRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r *dropCheckExistsRule) ID() string { return ruleIDAlterDropCheckExistsRequire }

func (r *dropCheckExistsRule) AppliesTo(statement spec.Statement) bool {
	return (statement.Dialect == spec.DialectMySQL || statement.Dialect == spec.DialectTiDB) &&
		len(matchingAlterActions(statement, "drop_check")) > 0
}

func (r *dropCheckExistsRule) Evaluate(_ context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	snapshot, ok := targetTableSnapshot(statement)
	if !ok || snapshot == nil || !snapshot.Exists {
		return nil, nil
	}
	tableName := metadataTargetTableName(statement, snapshot)
	findings := make([]rule.Finding, 0)
	for _, alter := range matchingAlterActions(statement, "drop_check") {
		executable, undecidable := checkActionOutcome(statement, alter)
		if !executable || undecidable {
			continue
		}
		name := strings.TrimSpace(alter.Name)
		if name == "" {
			continue
		}
		binding := checkDropTargetBinding(statement.Dialect, snapshot, alter, name)
		var message string
		switch binding {
		case spec.CheckTargetMissing:
			message = fmt.Sprintf("constraint %q does not exist on table %q", name, tableName)
		case spec.CheckTargetWrongKind:
			message = fmt.Sprintf("constraint %q on table %q exists but is not a check constraint — the CHECK keyword cannot drop it", name, tableName)
		case spec.CheckTargetAmbiguous:
			message = fmt.Sprintf("constraint name %q on table %q is shared by multiple constraint kinds — the generic spelling refuses ambiguous targets", name, tableName)
		default:
			continue
		}
		findings = append(findings, rule.Finding{
			Level:      r.level,
			Message:    message,
			Suggestion: "fix the constraint name or use the CHECK-specific keyword, and refresh metadata before auditing this change",
			Metadata: map[string]any{
				"table":  tableName,
				"action": alter.Action,
				"name":   name,
				"exists": false,
			},
		})
	}
	return findings, nil
}

func (r *dropCheckExistsRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(statement) {
		return nil
	}
	for _, alter := range matchingAlterActions(statement, "drop_check") {
		_, undecidable := checkActionOutcome(statement, alter)
		if undecidable {
			return checkArmGap(statement)
		}
	}
	snapshot, ok := targetTableSnapshot(statement)
	if !ok || snapshot == nil || !snapshot.Exists {
		return []rule.EvidenceGap{{
			ReasonCode:    gapReasonUnknownTableState,
			RequiredFacts: []string{"target_table.existence"},
		}}
	}
	for _, alter := range matchingAlterActions(statement, "drop_check") {
		executable, _ := checkActionOutcome(statement, alter)
		if !executable {
			continue
		}
		name := strings.TrimSpace(alter.Name)
		if name == "" {
			continue
		}
		if checkDropTargetBinding(statement.Dialect, snapshot, alter, name) == spec.CheckTargetUnproven {
			return []rule.EvidenceGap{{
				ReasonCode:    gapReasonUnknownTableState,
				RequiredFacts: checkBindingGapFacts(snapshot, alter.Options["check_spelling"], name),
			}}
		}
	}
	return nil
}

// checkIgnoredNoticeRule reveals CHECK declarations an arm provably parses
// but ignores — MySQL before the 8.0.16 enforcement boundary and TiDB under
// an observed tidb_enable_check_constraint=OFF — so an accepted-but-ignored
// declaration can never read as effective enforcement.
type checkIgnoredNoticeRule struct {
	level rule.Level
}

func newCheckIgnoredNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return &checkIgnoredNoticeRule{level: configuredLevel(cfg, rule.LevelNotice)}, nil
}

func (r *checkIgnoredNoticeRule) ID() string { return ruleIDConstraintCheckIgnoredNotice }

func (r *checkIgnoredNoticeRule) AppliesTo(statement spec.Statement) bool {
	if statement.Dialect != spec.DialectMySQL && statement.Dialect != spec.DialectTiDB {
		return false
	}
	return len(declaredCheckConstraints(statement)) > 0
}

func (r *checkIgnoredNoticeRule) Evaluate(_ context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) || checkArm(statement) != spec.CheckArmIgnored {
		return nil, nil
	}
	snapshot, _ := targetTableSnapshot(statement)
	tableName := metadataTargetTableName(statement, snapshot)
	findings := make([]rule.Finding, 0)
	for _, member := range declaredCheckConstraints(statement) {
		if spec.CheckExplicitClauseRefused(statement.Dialect, spec.CheckArmIgnored, member) {
			// An authored [NOT] ENFORCED clause on MySQL's parse-and-ignore
			// arm is a 1064 refusal — the whole statement fails, so no
			// member here is "parsed but ignored" (probed 5.7.44). TiDB's
			// ignored arm accepts and ignores the clause (probed v8.5.0
			// flag OFF), so the notice stays owed there.
			return nil, nil
		}
	}
	for _, member := range declaredCheckConstraints(statement) {
		if member.EnforcedUnproven && statement.Dialect == spec.DialectMySQL {
			// The member's clause could not be attributed — on MySQL's
			// ignored arm the statement may have refused 1064, so an
			// "ignored" answer is not provable. EvidenceGaps carries the
			// missing fact; TiDB ignores either way.
			continue
		}
		name := member.Name
		if name == "" {
			name = "(unnamed)"
		}
		findings = append(findings, rule.Finding{
			Level:      r.level,
			Message:    fmt.Sprintf("check constraint %q on table %q is parsed but ignored on this product", name, tableName),
			Suggestion: "verify the target product actually enforces check constraints before relying on this declaration",
			Metadata: map[string]any{
				"table":      tableName,
				"constraint": member.Name,
				"enforced":   "ignored",
			},
		})
	}
	return findings, nil
}

func (r *checkIgnoredNoticeRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(statement) {
		return nil
	}
	switch checkArm(statement) {
	case spec.CheckArmUnknown:
		return checkArmGap(statement)
	case spec.CheckArmIgnored:
		if statement.Dialect != spec.DialectMySQL {
			return nil
		}
		for _, member := range declaredCheckConstraints(statement) {
			if member.EnforcedUnproven {
				// The member's enforcement clause could not be attributed:
				// the statement may have refused 1064 instead of landing
				// an ignored member.
				return []rule.EvidenceGap{{
					ReasonCode:    gapReasonUnprovenStatementFact,
					RequiredFacts: []string{"statement.check.enforced_clause"},
				}}
			}
		}
	}
	return nil
}

// declaredCheckConstraints collects every check member a statement declares
// — CREATE table constraints and column checks, ALTER add_constraint
// payloads, and in-column checks — in declaration order.
func declaredCheckConstraints(statement spec.Statement) []spec.Constraint {
	if statement.DDL == nil {
		return nil
	}
	var out []spec.Constraint
	collect := func(members []spec.Constraint) {
		for i := range members {
			if members[i].Type == "check" {
				out = append(out, members[i])
			}
		}
	}
	if statement.DDL.Operation == spec.DDLOperationCreateTable {
		collect(statement.DDL.Constraints)
	}
	for _, alter := range statement.DDL.Alter {
		if alter.Constraint != nil && alter.Constraint.Type == "check" {
			out = append(out, *alter.Constraint)
		}
		collect(alter.Constraints)
	}
	return out
}

// alterCheckVersionRule refuses ALTER CHECK|CONSTRAINT and DROP
// CHECK|CONSTRAINT on MySQL arms whose version predates the 8.0.19 syntax
// boundary (5.7 refuses 1064). TiDB parses the syntax on both flag arms, so
// no version finding applies there — the flag governs effect instead.
type alterCheckVersionRule struct {
	required bool
	level    rule.Level
}

func newAlterCheckVersionRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	required, err := boolParam(ruleIDAlterCheckVersionRequire, cfg, "required", true)
	if err != nil {
		return nil, err
	}
	return &alterCheckVersionRule{required: required, level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r *alterCheckVersionRule) ID() string { return ruleIDAlterCheckVersionRequire }

func (r *alterCheckVersionRule) AppliesTo(statement spec.Statement) bool {
	if !r.required || statement.Dialect != spec.DialectMySQL {
		return false
	}
	if len(matchingAlterActions(statement, "alter_check", "drop_check")) > 0 {
		return true
	}
	for _, member := range declaredCheckConstraints(statement) {
		if member.EnforcedExplicit || member.EnforcedUnproven {
			return true
		}
	}
	return false
}

func (r *alterCheckVersionRule) Evaluate(_ context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	var version *spec.VersionIdentity
	if statement.Metadata != nil {
		version = statement.Metadata.Version
	}
	snapshot, _ := targetTableSnapshot(statement)
	tableName := metadataTargetTableName(statement, snapshot)
	findings := make([]rule.Finding, 0)
	for _, alter := range matchingAlterActions(statement, "alter_check", "drop_check") {
		spelling := alter.Options["check_spelling"]
		if spec.CheckAlterSyntaxOutcome(statement.Dialect, version, spelling) != spec.CheckSyntaxRefused {
			continue
		}
		required := spec.CheckAlterSyntaxMinVersion(spelling)
		findings = append(findings, rule.Finding{
			Level:      r.level,
			Message:    fmt.Sprintf("%s on table %q requires MySQL >= %s (check constraint alter/drop syntax)", alter.Action, tableName, required),
			Suggestion: "run this change on a MySQL version that supports check-constraint alter/drop syntax",
			Metadata: map[string]any{
				"table":            tableName,
				"action":           alter.Action,
				"version":          version.Version,
				"required_version": required,
			},
		})
	}
	// An authored [NOT] ENFORCED clause refuses before 8.0.16 — bare CHECK
	// declarations are the only parsed-and-ignored form (probed 5.7.44).
	if version != nil && version.ValidatedRange && version.Major != 0 &&
		(version.Major < 8 || (version.Major == 8 && version.Minor == 0 && version.Patch < 16)) {
		for _, member := range declaredCheckConstraints(statement) {
			if !member.EnforcedExplicit {
				continue
			}
			name := member.Name
			if name == "" {
				name = "(unnamed)"
			}
			findings = append(findings, rule.Finding{
				Level:      r.level,
				Message:    fmt.Sprintf("check constraint %q on table %q declares [NOT] ENFORCED — requires MySQL >= %s", name, tableName, spec.CheckMinEnforcementMySQL),
				Suggestion: "run this change on a MySQL version that supports check-constraint enforcement syntax",
				Metadata: map[string]any{
					"table":            tableName,
					"action":           "check_constraint_declaration",
					"constraint":       member.Name,
					"version":          version.Version,
					"required_version": spec.CheckMinEnforcementMySQL,
				},
			})
		}
	}
	return findings, nil
}

func (r *alterCheckVersionRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(statement) {
		return nil
	}
	var version *spec.VersionIdentity
	if statement.Metadata != nil {
		version = statement.Metadata.Version
	}
	if version == nil || !version.ValidatedRange || version.Major == 0 {
		return []rule.EvidenceGap{{
			ReasonCode:    gapReasonMissingTargetVersion,
			RequiredFacts: []string{factTargetVersion},
		}}
	}
	for _, alter := range matchingAlterActions(statement, "alter_check", "drop_check") {
		if spec.CheckAlterSyntaxOutcome(statement.Dialect, version, alter.Options["check_spelling"]) == spec.CheckSyntaxUnproven {
			return []rule.EvidenceGap{{
				ReasonCode:    gapReasonUnprovenStatementFact,
				RequiredFacts: []string{"statement.check_spelling"},
			}}
		}
	}
	// Below 8.0.16 the explicit enforcement clause is a syntax refusal —
	// an unattributed clause leaves refused-or-ignored undecidable.
	if version.Major < 8 || (version.Major == 8 && version.Minor == 0 && version.Patch < 16) {
		for _, member := range declaredCheckConstraints(statement) {
			if member.EnforcedUnproven {
				return []rule.EvidenceGap{{
					ReasonCode:    gapReasonUnprovenStatementFact,
					RequiredFacts: []string{"statement.check.enforced_clause"},
				}}
			}
		}
	}
	return nil
}
