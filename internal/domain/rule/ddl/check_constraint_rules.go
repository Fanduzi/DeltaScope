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
//     rows); missing/non-check names refuse 3821; both DROP keywords remove
//     a present CHECK; duplicate check names refuse 3822; DROP COLUMN
//     silently drops referencing checks.
//   - MySQL 5.7.44: CHECK declarations parse but are silently dropped
//     (never stored, never enforced); ALTER/DROP CHECK syntax refuses 1064.
//   - TiDB v8.5.0: tidb_enable_check_constraint gates everything — OFF
//     parses and ignores (nothing stored, all ops rc=0 no-ops), ON stores
//     and enforces with MySQL-equivalent semantics (3940 refusal class).

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

// armExecutable reports whether the statement reaches target binding at all:
// MySQL needs the 8.0.19+ syntax gate; TiDB needs the flag ON; unknown arms
// are undecidable.
func (r *alterCheckExistsRule) armExecutable(statement spec.Statement) (executable, undecidable bool) {
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
		if arm == spec.CheckArmIgnored || !spec.CheckAlterSyntaxSupported(statement.Dialect, version) {
			return false, false
		}
		return true, false
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

func (r *alterCheckExistsRule) Evaluate(_ context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	executable, undecidable := r.armExecutable(statement)
	if !executable || undecidable {
		return nil, nil
	}
	snapshot, ok := targetTableSnapshot(statement)
	if !ok || snapshot == nil || !snapshot.Exists {
		return nil, nil
	}
	tableName := metadataTargetTableName(statement, snapshot)
	findings := make([]rule.Finding, 0)
	for _, alter := range alterCheckActions(statement) {
		name := strings.TrimSpace(alter.Name)
		if name == "" {
			continue
		}
		if snapshot.FindCheckConstraint(name) != nil {
			continue
		}
		// An unvouched check name space cannot confirm absence; the gap is
		// owed by EvidenceGaps, not a fabricated finding.
		if snapshot.CheckNameSpaceUnvouched() {
			continue
		}
		findings = append(findings, rule.Finding{
			Level:      r.level,
			Message:    fmt.Sprintf("check constraint %q does not exist on table %q", name, tableName),
			Suggestion: "fix the check constraint name or refresh metadata before auditing this change",
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
	executable, undecidable := r.armExecutable(statement)
	if undecidable {
		return checkArmGap(statement)
	}
	if !executable {
		return nil
	}
	snapshot, ok := targetTableSnapshot(statement)
	if !ok || snapshot == nil || !snapshot.Exists {
		return []rule.EvidenceGap{{
			ReasonCode:    gapReasonUnknownTableState,
			RequiredFacts: []string{"target_table.existence"},
		}}
	}
	for _, alter := range alterCheckActions(statement) {
		name := strings.TrimSpace(alter.Name)
		if name == "" {
			continue
		}
		if snapshot.FindCheckConstraint(name) == nil && snapshot.CheckNameSpaceUnvouched() {
			return []rule.EvidenceGap{{
				ReasonCode:    gapReasonUnknownTableState,
				RequiredFacts: []string{"target_table.constraints"},
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
	executable, undecidable := dropCheckArmExecutable(statement)
	if !executable || undecidable {
		return nil, nil
	}
	snapshot, ok := targetTableSnapshot(statement)
	if !ok || snapshot == nil || !snapshot.Exists {
		return nil, nil
	}
	tableName := metadataTargetTableName(statement, snapshot)
	findings := make([]rule.Finding, 0)
	for _, alter := range matchingAlterActions(statement, "drop_check") {
		name := strings.TrimSpace(alter.Name)
		if name == "" {
			continue
		}
		if dropCheckNameBinds(snapshot, name) {
			continue
		}
		if dropCheckNameSpaceUnvouched(snapshot) {
			continue
		}
		findings = append(findings, rule.Finding{
			Level:      r.level,
			Message:    fmt.Sprintf("constraint %q does not exist on table %q", name, tableName),
			Suggestion: "fix the constraint name or refresh metadata before auditing this change",
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
	executable, undecidable := dropCheckArmExecutable(statement)
	if undecidable {
		return checkArmGap(statement)
	}
	if !executable {
		return nil
	}
	snapshot, ok := targetTableSnapshot(statement)
	if !ok || snapshot == nil || !snapshot.Exists {
		return []rule.EvidenceGap{{
			ReasonCode:    gapReasonUnknownTableState,
			RequiredFacts: []string{"target_table.existence"},
		}}
	}
	for _, alter := range matchingAlterActions(statement, "drop_check") {
		name := strings.TrimSpace(alter.Name)
		if name == "" {
			continue
		}
		if !dropCheckNameBinds(snapshot, name) && dropCheckNameSpaceUnvouched(snapshot) {
			return []rule.EvidenceGap{{
				ReasonCode:    gapReasonUnknownTableState,
				RequiredFacts: dropCheckUnvouchedFacts(snapshot),
			}}
		}
	}
	return nil
}

// dropCheckArmExecutable shares the arm gate with ALTER CHECK: MySQL needs
// the 8.0.19 syntax boundary; TiDB needs flag ON; unknown is undecidable.
func dropCheckArmExecutable(statement spec.Statement) (executable, undecidable bool) {
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
		if arm == spec.CheckArmIgnored || !spec.CheckAlterSyntaxSupported(statement.Dialect, version) {
			return false, false
		}
		return true, false
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

// dropCheckNameBinds answers whether the authored name binds a removable
// member any vouched collection proves: a CHECK member, a foreign key, an
// index member, or the primary-key object under its stored name.
func dropCheckNameBinds(snapshot *spec.TableSnapshot, name string) bool {
	if snapshot.FindCheckConstraint(name) != nil ||
		snapshot.FindForeignKeyConstraint(name) != nil ||
		snapshot.FindProvenIndex(name) != nil {
		return true
	}
	if pkName, proven := spec.PrimaryKeyObjectName(snapshot); proven && strings.EqualFold(pkName, name) {
		return true
	}
	return snapshot.PrimaryKey != nil && strings.EqualFold(name, "primary")
}

// dropCheckNameSpaceUnvouched reports whether any collection the binding
// question consults cannot vouch its name space.
func dropCheckNameSpaceUnvouched(snapshot *spec.TableSnapshot) bool {
	return snapshot.CheckNameSpaceUnvouched() ||
		snapshot.ForeignKeyNameSpaceUnvouched() ||
		snapshot.IndexNameSpaceUnvouched()
}

func dropCheckUnvouchedFacts(snapshot *spec.TableSnapshot) []string {
	facts := make([]string, 0, 3)
	if snapshot.CheckNameSpaceUnvouched() || snapshot.ForeignKeyNameSpaceUnvouched() {
		facts = append(facts, "target_table.constraints")
	}
	if snapshot.IndexNameSpaceUnvouched() {
		facts = append(facts, "target_table.indexes")
	}
	return facts
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
	if !r.AppliesTo(statement) || checkArm(statement) != spec.CheckArmUnknown {
		return nil
	}
	return checkArmGap(statement)
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
	return len(matchingAlterActions(statement, "alter_check", "drop_check")) > 0
}

func (r *alterCheckVersionRule) Evaluate(_ context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	var version *spec.VersionIdentity
	if statement.Metadata != nil {
		version = statement.Metadata.Version
	}
	if spec.CheckAlterSyntaxSupported(statement.Dialect, version) {
		return nil, nil
	}
	if version == nil || !version.ValidatedRange || version.Major == 0 {
		// An unobserved version cannot prove refusal — the gap is owed by
		// EvidenceGaps, not a fabricated incompatible finding.
		return nil, nil
	}
	snapshot, _ := targetTableSnapshot(statement)
	tableName := metadataTargetTableName(statement, snapshot)
	findings := make([]rule.Finding, 0)
	for _, alter := range matchingAlterActions(statement, "alter_check", "drop_check") {
		findings = append(findings, rule.Finding{
			Level:      r.level,
			Message:    fmt.Sprintf("%s on table %q requires MySQL >= %s (check constraint alter/drop syntax)", alter.Action, tableName, spec.CheckMinAlterSyntaxMySQL),
			Suggestion: "run this change on a MySQL version that supports check-constraint alter/drop syntax",
			Metadata: map[string]any{
				"table":            tableName,
				"action":           alter.Action,
				"version":          version.Version,
				"required_version": spec.CheckMinAlterSyntaxMySQL,
			},
		})
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
	return nil
}
