// Package ddl: T13-R1 partition applicability rules.
//
// The AST risk rules (partition_rules.go) fire on the authored action
// alone; the rules here bind the action to proven partition metadata:
// primary partitioned state, partition method, named-member existence,
// and EXCHANGE counterpart identity/compatibility. The verdict table is
// evidence-bounded:
//   - DROP PARTITION executes on range|list types and refuses on
//     hash|key types (native 1512).
//   - Any partition action on a known non-partitioned primary refuses
//     (native 1505).
//   - A named partition member missing from an observed member set
//     refuses (native 1507 DROP, 1735 TRUNCATE/EXCHANGE).
//   - EXCHANGE counterpart missing (1146), partitioned (1732), or
//     definition-mismatched (1736) refuses.
//
// Facts that were never observed stay unknown and surface as evidence
// gaps on the owning rule — never fabricated into a refusal finding.
package ddl

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// Reason codes emitted as finding metadata on proven applicability cells.
const (
	partitionReasonNotPartitioned         = "not_partitioned"
	partitionReasonTypeInapplicable       = "partition_type_inapplicable"
	partitionReasonPartitionAbsent        = "partition_absent"
	partitionReasonCounterpartAbsent      = "counterpart_absent"
	partitionReasonCounterpartPartitioned = "counterpart_partitioned"
	partitionReasonDefinitionMismatch     = "counterpart_definition_mismatch"
	partitionReasonEngineMismatch         = "counterpart_engine_mismatch"
)

// gapReasonPartitionFacts reuses the shared unknown-table-state reason:
// the partition family owes bounded catalog facts the probe never
// produced — the fact names carry the partition-specific identity.
const gapReasonPartitionFacts = gapReasonUnknownTableState

// partitionActionApplicableTypes lists the partition methods that allow
// DROP PARTITION on both anchors (1512 on hash|key families). TRUNCATE and
// EXCHANGE execute on every observed method, so only drop carries the
// gate.
var partitionActionApplicableTypes = map[string]map[string]bool{
	"drop_partition": {
		"range": true, "range columns": true,
		"list": true, "list columns": true,
	},
}

// partitionApplicabilityRule gates one partition action on the primary
// table's observed partition state and method.
type partitionApplicabilityRule struct {
	ruleID  string
	action  string
	label   string
	enabled bool
	level   rule.Level
}

func newPartitionApplicabilityRule(ruleID, action, label string, cfg policy.RulePolicy) (rule.StatementRule, error) {
	return partitionApplicabilityRule{
		ruleID:  ruleID,
		action:  action,
		label:   label,
		enabled: cfg.Enabled,
		level:   configuredLevel(cfg, rule.LevelBlocker),
	}, nil
}

func newDropPartitionApplicabilityRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPartitionApplicabilityRule(ruleIDAlterDropPartitionApplicability, "drop_partition", "drop partition", cfg)
}

func newTruncatePartitionApplicabilityRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPartitionApplicabilityRule(ruleIDAlterTruncatePartitionApplicability, "truncate_partition", "truncate partition", cfg)
}

func newExchangePartitionApplicabilityRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPartitionApplicabilityRule(ruleIDAlterExchangePartitionApplicability, "exchange_partition", "exchange partition", cfg)
}

func (r partitionApplicabilityRule) ID() string { return r.ruleID }

func (r partitionApplicabilityRule) AppliesTo(statement spec.Statement) bool {
	return r.enabled && orderedGapDialect(statement.Dialect) &&
		len(matchingAlterActions(statement, r.action)) > 0
}

func (r partitionApplicabilityRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	snapshot, ok := targetTableSnapshot(statement)
	// Unvouched or known-absent primaries own no finding here — the gap
	// path reports the missing fact and the existence rules own the
	// absent-table cell.
	if !ok || snapshot == nil || !snapshot.Exists || snapshot.Partitioned == nil {
		return nil, nil
	}
	if !*snapshot.Partitioned {
		return partitionApplicabilityFindings(r, statement, snapshot,
			partitionReasonNotPartitioned, "1505",
			fmt.Sprintf("ALTER TABLE %s on a non-partitioned table cannot execute (native 1505)", r.label)), nil
	}
	allowed, typeGated := partitionActionApplicableTypes[r.action]
	if typeGated {
		if snapshot.PartitionType == nil {
			return nil, nil
		}
		if !allowed[*snapshot.PartitionType] {
			return partitionApplicabilityFindings(r, statement, snapshot,
				partitionReasonTypeInapplicable, "1512",
				fmt.Sprintf("ALTER TABLE %s is not allowed on %s-partitioned tables (native 1512)", r.label, *snapshot.PartitionType)), nil
		}
	}
	return nil, ctx.Err()
}

func partitionApplicabilityFindings(r partitionApplicabilityRule, statement spec.Statement, snapshot *spec.TableSnapshot, reasonCode, nativeError, message string) []rule.Finding {
	findings := make([]rule.Finding, 0, len(statement.DDL.Alter))
	for _, alter := range statement.DDL.Alter {
		if alter.Action != r.action {
			continue
		}
		metadata := partitionMetadata(statement, alter, map[string]any{
			"reason_code":  reasonCode,
			"native_error": nativeError,
			"exists":       "true",
			"partitioned":  fmt.Sprintf("%t", *snapshot.Partitioned),
		})
		if snapshot.PartitionType != nil {
			metadata["partition_type"] = *snapshot.PartitionType
		}
		if len(alter.Partitions) > 0 {
			metadata["partition"] = alter.Partitions[0]
		}
		if alter.ExchangeTable != nil {
			metadata["exchange_table"] = alter.ExchangeTable.Name
			metadata["exchange_schema"] = alter.ExchangeTable.Schema
		}
		findings = append(findings, rule.Finding{
			Level:      r.level,
			Message:    message,
			Suggestion: fmt.Sprintf("verify the target actually carries %s-compatible partitioning before shipping", r.label),
			Metadata:   metadata,
		})
	}
	return findings
}

// EvidenceGaps reports the partition-state facts the rule needed but the
// projection never answered. Unknown primary state reports the partitioned
// fact; a known-partitioned primary that never answered PARTITION_METHOD
// under a type-gated action reports the type fact. Facts the rule does
// not consume (engine, members, counterpart) are never owed here.
func (r partitionApplicabilityRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(statement) {
		return nil
	}
	snapshot, ok := targetTableSnapshot(statement)
	if !ok || snapshot == nil {
		return []rule.EvidenceGap{{
			ReasonCode:    gapReasonPartitionFacts,
			RequiredFacts: []string{"target_table.partitioned"},
		}}
	}
	if !snapshot.Exists {
		return nil
	}
	if snapshot.Partitioned == nil {
		return []rule.EvidenceGap{{
			ReasonCode:    gapReasonPartitionFacts,
			RequiredFacts: []string{"target_table.partitioned"},
		}}
	}
	if _, typeGated := partitionActionApplicableTypes[r.action]; typeGated &&
		*snapshot.Partitioned && snapshot.PartitionType == nil {
		return []rule.EvidenceGap{{
			ReasonCode:    gapReasonPartitionFacts,
			RequiredFacts: []string{"target_table.partition_type"},
		}}
	}
	return nil
}

// partitionTargetRule checks each named partition member against the
// observed member set of a known-partitioned primary.
type partitionTargetRule struct {
	ruleID      string
	action      string
	label       string
	nativeError string
	enabled     bool
	level       rule.Level
}

func newPartitionTargetRule(ruleID, action, label, nativeError string, cfg policy.RulePolicy) (rule.StatementRule, error) {
	return partitionTargetRule{
		ruleID:      ruleID,
		action:      action,
		label:       label,
		nativeError: nativeError,
		enabled:     cfg.Enabled,
		level:       configuredLevel(cfg, rule.LevelBlocker),
	}, nil
}

func newDropPartitionTargetRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPartitionTargetRule(ruleIDAlterDropPartitionTarget, "drop_partition", "drop partition", "1507", cfg)
}

func newTruncatePartitionTargetRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPartitionTargetRule(ruleIDAlterTruncatePartitionTarget, "truncate_partition", "truncate partition", "1735", cfg)
}

func newExchangePartitionTargetRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPartitionTargetRule(ruleIDAlterExchangePartitionTarget, "exchange_partition", "exchange partition", "1735", cfg)
}

// newPartitionTargetExistsRule dispatches the member-existence rule for
// one authored partition action — the shared factory the registration
// table and tests bind by action name.
func newPartitionTargetExistsRule(action string, cfg policy.RulePolicy) (rule.StatementRule, error) {
	switch action {
	case "drop_partition":
		return newDropPartitionTargetRule(cfg)
	case "truncate_partition":
		return newTruncatePartitionTargetRule(cfg)
	case "exchange_partition":
		return newExchangePartitionTargetRule(cfg)
	default:
		return nil, fmt.Errorf("no partition target rule for action %q", action)
	}
}

func (r partitionTargetRule) ID() string { return r.ruleID }

func (r partitionTargetRule) AppliesTo(statement spec.Statement) bool {
	return r.enabled && orderedGapDialect(statement.Dialect) &&
		len(matchingAlterActions(statement, r.action)) > 0
}

func (r partitionTargetRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	snapshot, ok := targetTableSnapshot(statement)
	if !ok || snapshot == nil || !snapshot.Exists ||
		snapshot.Partitioned == nil || !*snapshot.Partitioned ||
		snapshot.PartitionNames == nil {
		return nil, nil
	}
	members := make(map[string]bool, len(snapshot.PartitionNames))
	for _, name := range snapshot.PartitionNames {
		members[strings.ToLower(name)] = true
	}
	findings := make([]rule.Finding, 0)
	for _, alter := range matchingAlterActions(statement, r.action) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if alter.AllPartitions {
			// The ALL form names no member — nothing to check.
			continue
		}
		for _, name := range alter.Partitions {
			if members[strings.ToLower(name)] {
				continue
			}
			findings = append(findings, rule.Finding{
				Level:      r.level,
				Message:    fmt.Sprintf("ALTER TABLE %s names partition %q, which is not a member of the table (native %s)", r.label, name, r.nativeError),
				Suggestion: "verify the partition name against the observed member set",
				Metadata: partitionMetadata(statement, alter, map[string]any{
					"partition":    name,
					"exists":       "false",
					"reason_code":  partitionReasonPartitionAbsent,
					"native_error": r.nativeError,
				}),
			})
		}
	}
	return findings, nil
}

// EvidenceGaps reports every premise the rule consumed but could not
// observe — the target rule never borrows the applicability rule's job:
// an unobserved primary or an unproven partitioned truth owes the
// partitioned fact itself, and a proven-partitioned primary whose member
// directory stayed unobserved or incomplete owes the member-set fact.
// Proven-absent and proven-non-partitioned tables are finding paths owned
// by the family, not gap paths here. The ALL form consumes no member name
// and owes nothing at any state.
func (r partitionTargetRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(statement) {
		return nil
	}
	namesConsumed := false
	for _, alter := range matchingAlterActions(statement, r.action) {
		if !alter.AllPartitions && len(alter.Partitions) > 0 {
			namesConsumed = true
			break
		}
	}
	if !namesConsumed {
		return nil
	}
	snapshot, ok := targetTableSnapshot(statement)
	if !ok || snapshot == nil {
		return []rule.EvidenceGap{{
			ReasonCode:    gapReasonPartitionFacts,
			RequiredFacts: []string{"target_table.partitioned"},
		}}
	}
	if !snapshot.Exists {
		return nil
	}
	if snapshot.Partitioned == nil {
		return []rule.EvidenceGap{{
			ReasonCode:    gapReasonPartitionFacts,
			RequiredFacts: []string{"target_table.partitioned"},
		}}
	}
	if !*snapshot.Partitioned {
		return nil
	}
	if snapshot.PartitionNames == nil {
		return []rule.EvidenceGap{{
			ReasonCode:    gapReasonPartitionFacts,
			RequiredFacts: []string{"target_table.partitions"},
		}}
	}
	return nil
}

// exchangeCounterpartRule binds the EXCHANGE counterpart identity to its
// observed pre-state: known-absent (1146), known-partitioned (1732), or
// structurally incompatible on the bounded column/PK compare (1736) are
// proven refusals. The rule never claims the exchange validated — a clean
// bounded compare stays silent.
type exchangeCounterpartRule struct {
	ruleID  string
	enabled bool
	level   rule.Level
}

func newExchangeCounterpartCompatibilityRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return exchangeCounterpartRule{
		ruleID:  ruleIDAlterExchangePartitionCounterpart,
		enabled: cfg.Enabled,
		level:   configuredLevel(cfg, rule.LevelBlocker),
	}, nil
}

func (r exchangeCounterpartRule) ID() string { return r.ruleID }

func (r exchangeCounterpartRule) AppliesTo(statement spec.Statement) bool {
	return r.enabled && orderedGapDialect(statement.Dialect) &&
		len(matchingAlterActions(statement, "exchange_partition")) > 0
}

func (r exchangeCounterpartRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	counterpart := counterpartSnapshot(statement)
	if counterpart == nil {
		return nil, nil
	}
	findings := make([]rule.Finding, 0)
	for _, alter := range matchingAlterActions(statement, "exchange_partition") {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !counterpart.Exists {
			findings = append(findings, r.counterpartFinding(statement, alter,
				partitionReasonCounterpartAbsent, "1146",
				fmt.Sprintf("ALTER TABLE exchange partition counterpart %q does not exist (native 1146)", exchangeTableName(alter)), "false"))
			continue
		}
		if counterpart.Partitioned != nil && *counterpart.Partitioned {
			findings = append(findings, r.counterpartFinding(statement, alter,
				partitionReasonCounterpartPartitioned, "1732",
				fmt.Sprintf("ALTER TABLE exchange partition counterpart %q is itself partitioned (native 1732)", exchangeTableName(alter)), "true"))
			continue
		}
		if counterpart.Partitioned == nil {
			continue
		}
		primary, _ := targetTableSnapshot(statement)
		if primary == nil || !primary.Exists {
			continue
		}
		comparison := exchangeComparisonFacts(primary, counterpart, statement.Dialect)
		if comparison.definitionMismatch {
			findings = append(findings, r.counterpartFinding(statement, alter,
				partitionReasonDefinitionMismatch, "1736",
				fmt.Sprintf("ALTER TABLE exchange partition counterpart %q has a different definition (native 1736)", exchangeTableName(alter)), "true"))
			continue
		}
		if comparison.engineMismatch {
			findings = append(findings, r.counterpartFinding(statement, alter,
				partitionReasonEngineMismatch, "1497",
				fmt.Sprintf("ALTER TABLE exchange partition counterpart %q runs a different storage engine (native 1497)", exchangeTableName(alter)), "true"))
		}
	}
	return findings, nil
}

func (r exchangeCounterpartRule) counterpartFinding(statement spec.Statement, alter spec.Alter, reasonCode, nativeError, message, exists string) rule.Finding {
	metadata := partitionMetadata(statement, alter, map[string]any{
		"exists":       exists,
		"reason_code":  reasonCode,
		"native_error": nativeError,
	})
	if len(alter.Partitions) > 0 {
		metadata["partition"] = alter.Partitions[0]
	}
	if alter.ExchangeTable != nil {
		metadata["exchange_table"] = alter.ExchangeTable.Name
		metadata["exchange_schema"] = alter.ExchangeTable.Schema
	}
	return rule.Finding{
		Level:      r.level,
		Message:    message,
		Suggestion: "verify the counterpart is a non-partitioned table with a matching definition",
		Metadata:   metadata,
	}
}

// EvidenceGaps reports the counterpart and compare facts the rule needed
// but the projection never answered, sharing Evaluate's premise exactly:
// an unresolved counterpart identity reports existence; a resolved one
// reports partitioned state; then the bounded compare owes whichever
// per-side facts were needed — columns, PK identity, and (MySQL only) the
// engine label. Nothing is owed for premises the rule never consumes.
func (r exchangeCounterpartRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(statement) || !hasExchangeCounterpart(statement) {
		return nil
	}
	counterpart := counterpartSnapshot(statement)
	if counterpart == nil {
		return []rule.EvidenceGap{{
			ReasonCode:    gapReasonPartitionFacts,
			RequiredFacts: []string{"exchange_table.existence"},
		}}
	}
	if !counterpart.Exists {
		return nil
	}
	if counterpart.Partitioned == nil {
		return []rule.EvidenceGap{{
			ReasonCode:    gapReasonPartitionFacts,
			RequiredFacts: []string{"exchange_table.partitioned"},
		}}
	}
	if *counterpart.Partitioned {
		return nil
	}
	primary, _ := targetTableSnapshot(statement)
	missing := exchangeComparisonFacts(primary, counterpart, statement.Dialect).missing
	if len(missing) == 0 {
		return nil
	}
	return []rule.EvidenceGap{{
		ReasonCode:    gapReasonPartitionFacts,
		RequiredFacts: missing,
	}}
}

// counterpartSnapshot returns the resolved EXCHANGE counterpart projection,
// or nil when the statement carries no resolvable counterpart identity.
func counterpartSnapshot(statement spec.Statement) *spec.TableSnapshot {
	if statement.Metadata == nil {
		return nil
	}
	return statement.Metadata.ExchangeTable
}

// hasExchangeCounterpart reports whether the statement's exchange clause
// named a counterpart table — the bound identity fact gaps can be owed
// for. An unbound clause owes nothing here.
func hasExchangeCounterpart(statement spec.Statement) bool {
	for _, alter := range matchingAlterActions(statement, "exchange_partition") {
		if alter.ExchangeTable != nil && strings.TrimSpace(alter.ExchangeTable.Name) != "" {
			return true
		}
	}
	return false
}

// exchangeComparison is one shared comparison premise for the counterpart
// rule: Evaluate reads the proven-mismatch flags, EvidenceGaps reads the
// missing-fact list, and both are computed by the same walk so a reported
// refusal and an owed fact can never disagree about what was compared.
type exchangeComparison struct {
	definitionMismatch bool
	engineMismatch     bool
	missing            []string
}

// exchangeComparisonFacts runs the bounded EXCHANGE comparison:
//   - columns: name order + effective type + signedness. A column pair the
//     bounded model cannot equate keeps the comparison unproven and owes
//     that side's columns fact (the fact exists but could not be
//     adjudicated — the gap asks for a decidable answer).
//   - primary key: presence and member identity under the established
//     PK-absence contract — PrimaryKeyUnknown blocks, a vouched absent PK
//     is never reopened by unrelated constraint-set unknowns.
//   - engine (MySQL only): the observed engine labels must match — MySQL
//     refuses the mix natively (1497). TiDB normalizes the ENGINE label in
//     the catalog, so the premise is never consumed there.
//
// A dimension that proves a mismatch reports it independently of the other
// dimensions' knownness; an undecidable dimension owes its own fact.
func exchangeComparisonFacts(primary, counterpart *spec.TableSnapshot, dialect spec.Dialect) exchangeComparison {
	var out exchangeComparison
	primaryObserved := primary != nil
	primaryKnown := primaryObserved && primary.Exists
	if primary == nil {
		// An unvouched primary carries none of its compare facts. A
		// proven-absent one owes nothing here — the existence family owns
		// that refusal path.
		out.missing = append(out.missing,
			"target_table.columns", "target_table.primary_key")
		if dialect == spec.DialectMySQL {
			out.missing = append(out.missing, "target_table.engine")
		}
	}
	if primaryKnown && primary.Columns == nil {
		out.missing = append(out.missing, "target_table.columns")
	}
	if counterpart.Columns == nil {
		out.missing = append(out.missing, "exchange_table.columns")
	}
	if primaryKnown && primary.Columns != nil && counterpart.Columns != nil {
		switch {
		case len(primary.Columns) != len(counterpart.Columns):
			out.definitionMismatch = true
		default:
			for i := range primary.Columns {
				equal, comparable := exchangeColumnComparison(primary.Columns[i], counterpart.Columns[i])
				if !comparable {
					out.missing = append(out.missing, "target_table.columns", "exchange_table.columns")
					break
				}
				if !equal {
					out.definitionMismatch = true
					break
				}
			}
		}
	}
	var pkMembers, cpMembers []string
	var pkPresent, cpPresent, pkMembersOK, cpMembersOK, pkOK, cpOK bool
	if primaryKnown {
		pkPresent, pkMembers, pkMembersOK, pkOK = primaryKeyIdentityFacts(primary)
	}
	cpPresent, cpMembers, cpMembersOK, cpOK = primaryKeyIdentityFacts(counterpart)
	if primaryKnown && !pkOK {
		out.missing = append(out.missing, "target_table.primary_key")
	}
	if !cpOK {
		out.missing = append(out.missing, "exchange_table.primary_key")
	}
	// A vouched presence whose member sequence never resolved owes the
	// same fact — presence answered but members cannot compare.
	if primaryKnown && pkOK && pkPresent && !pkMembersOK {
		out.missing = append(out.missing, "target_table.primary_key")
	}
	if cpOK && cpPresent && !cpMembersOK {
		out.missing = append(out.missing, "exchange_table.primary_key")
	}
	if pkOK && cpOK {
		if pkPresent != cpPresent ||
			(pkPresent && pkMembersOK && cpMembersOK && !equalFoldedNames(pkMembers, cpMembers)) {
			out.definitionMismatch = true
		}
	}
	if dialect == spec.DialectMySQL {
		var primaryEngine string
		primaryOK := false
		if primaryKnown {
			primaryEngine, primaryOK = observedEngine(primary)
			if !primaryOK {
				out.missing = append(out.missing, "target_table.engine")
			}
		}
		counterpartEngine, counterpartOK := observedEngine(counterpart)
		if !counterpartOK {
			out.missing = append(out.missing, "exchange_table.engine")
		}
		if primaryOK && counterpartOK && !strings.EqualFold(primaryEngine, counterpartEngine) {
			out.engineMismatch = true
		}
	}
	return out
}

// observedEngine reads the vouched engine label — an absent Options entry
// means the fact was never observed, never "no engine".
func observedEngine(snapshot *spec.TableSnapshot) (string, bool) {
	if snapshot.Options == nil {
		return "", false
	}
	engine, ok := snapshot.Options["engine"]
	if !ok || strings.TrimSpace(engine) == "" {
		return "", false
	}
	return engine, true
}

// exchangeIntegerBases are the type families whose declared length is
// display width, not storage identity — natively `INT(10)` exchanges
// against `INT(11)` (probed 5.7), and MySQL ≥8.0.19 drops the width from
// the catalog entirely.
var exchangeIntegerBases = map[string]bool{
	"tinyint": true, "smallint": true, "mediumint": true,
	"int": true, "integer": true, "bigint": true, "year": true,
}

// exchangeDefaultLengths are the type families whose unspecified declared
// length folds to a native default: CHAR/BINARY default to 1, DECIMAL to
// 10, BIT to 1.
var exchangeDefaultLengths = map[string]int{
	"char": 1, "binary": 1, "decimal": 10, "numeric": 10, "bit": 1,
}

// exchangeFaithfulBases are the bases whose parameter text lives inside
// the normalized Type string on both channels (the T06-A17 contract) —
// DECIMAL scale, FLOAT/DOUBLE precision, ENUM/SET member literals.
var exchangeFaithfulBases = map[string]bool{
	"decimal": true, "numeric": true, "float": true, "double": true, "enum": true, "set": true,
}

// exchangeColumnComparison compares one column pair on the bounded
// exchange contract: name, effective type, and signedness. comparable=false
// marks a dimension the observed facts cannot decide — never read as equal.
func exchangeColumnComparison(a, b spec.Column) (equal, comparable bool) {
	if !strings.EqualFold(a.Name, b.Name) || a.Unsigned != b.Unsigned {
		return false, true
	}
	baseA, baseB := baseType(a), baseType(b)
	if baseA != baseB {
		return false, true
	}
	if exchangeFaithfulBases[baseA] {
		// The effective parameter set is the identity: DECIMAL/NUMERIC
		// spellings fold to base(M,D) against native defaults, other
		// faithful bases compare the verbatim parameter text. A spelling
		// the bounded normalizer cannot project keeps the comparison
		// unproven — never read as a difference.
		normalizedA, okA := normalizeFaithfulColumnType(a.Type, baseA)
		normalizedB, okB := normalizeFaithfulColumnType(b.Type, baseB)
		if !okA || !okB {
			return false, false
		}
		return normalizedA == normalizedB, true
	}
	if exchangeIntegerBases[baseA] {
		return true, true
	}
	lengthA, lengthB := a.Length, b.Length
	if def, ok := exchangeDefaultLengths[baseA]; ok {
		if lengthA < 0 {
			lengthA = def
		}
		if lengthB < 0 {
			lengthB = def
		}
	}
	if lengthA < 0 && lengthB < 0 {
		// Both channels declared the same bare form — equal effective type.
		return true, true
	}
	if lengthA < 0 || lengthB < 0 {
		return false, false
	}
	return lengthA == lengthB, true
}

// normalizeFaithfulColumnType folds a faithful-base type spelling to its
// effective form so the declared and catalog channels align. DECIMAL and
// NUMERIC share the fixed-point default-parameter contract — a bare
// spelling is base(10,0) and a single parameter is base(M,0) — while a
// trailing `unsigned` attribute word is consumed by the dedicated
// signedness compare. Other faithful bases keep their verbatim parameter
// text (member literals are the identity). A faithful form the bounded
// normalizer cannot project returns ok=false — undecidable, never a
// difference.
func normalizeFaithfulColumnType(typeText, base string) (string, bool) {
	trimmed := strings.ToLower(strings.TrimSpace(typeText))
	if base != "decimal" && base != "numeric" {
		return trimmed, true
	}
	trimmed = strings.TrimSpace(strings.TrimSuffix(trimmed, " unsigned"))
	if trimmed == base {
		return base + "(10,0)", true
	}
	rest, ok := strings.CutPrefix(trimmed, base+"(")
	if !ok || !strings.HasSuffix(rest, ")") {
		return "", false
	}
	params := strings.Split(rest[:len(rest)-1], ",")
	if len(params) > 2 {
		return "", false
	}
	var normalized strings.Builder
	normalized.WriteString(base)
	normalized.WriteString("(")
	for i, param := range params {
		value, err := strconv.Atoi(strings.TrimSpace(param))
		if err != nil {
			return "", false
		}
		if i > 0 {
			normalized.WriteString(",")
		}
		normalized.WriteString(strconv.Itoa(value))
	}
	if len(params) == 1 {
		normalized.WriteString(",0")
	}
	normalized.WriteString(")")
	return normalized.String(), true
}

// primaryKeyIdentityFacts resolves PK presence and member columns from the
// vouched member facts. ok=false means presence could not be vouched —
// never read as "no PK"; membersOK=false on a proven-present PK means the
// member sequence was never observed (the unattributed-object placeholder
// carries nil Columns — a real key always has members, so nil is unproven,
// never an empty key). PrimaryKeyUnknown outranks any residual pointer the
// ordered-state unvouch leaves attached. The dedicated PrimaryKey channel
// decides presence: a proven-absent PK is not reopened by an unvouched
// CHECK/FK constraint set (ConstraintsUnknown covers the constraint
// collection, not PK presence).
func primaryKeyIdentityFacts(snapshot *spec.TableSnapshot) (present bool, members []string, membersOK, ok bool) {
	if snapshot == nil || snapshot.PrimaryKeyUnknown {
		return false, nil, false, false
	}
	if snapshot.PrimaryKey != nil {
		return true, snapshot.PrimaryKey.Columns, snapshot.PrimaryKey.Columns != nil, true
	}
	if !snapshot.ConstraintsUnknown {
		for _, constraint := range snapshot.Constraints {
			if constraint.Type == "primary_key" {
				return true, constraint.Columns, constraint.Columns != nil, true
			}
		}
	}
	return false, nil, false, true
}

func equalFoldedNames(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !strings.EqualFold(a[i], b[i]) {
			return false
		}
	}
	return true
}
