// Package ddl: T14 partition topology and maintenance rules.
//
// Three families, all frozen under CONTRACT.md §6:
//   - policy findings (warn/forbid) fire on the authored action alone and
//     never fire under a dialect that vendor-refuses the action;
//   - applicability rules bind the action to proven partition state —
//     non-partitioned targets, inapplicable methods, refused authored
//     forms, and provably-invalid declared scheme bounds;
//   - target rules bind named members to the observed member directory
//     and prove the REORGANIZE/ADD bound contract.
//
// Every refusal finding carries the dialect-correct native_error from the
// probed anchor matrix; soft-fail cells (status-table "Operation failed"
// at rc=0) record soft_fail=true at warning level instead of claiming a
// hard refusal. Facts the projection never answered stay evidence gaps on
// the owning rule — never fabricated refusals.
package ddl

import (
	"context"
	"fmt"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// Reason codes emitted as finding metadata on proven T14 cells.
const (
	partitionReasonSourcesNonAdjacent  = "partition_sources_non_adjacent"
	partitionReasonDuplicateName       = "partition_name_duplicate"
	partitionReasonBoundViolation      = "partition_bound_violation"
	partitionReasonMaxValueTail        = "partition_maxvalue_tail"
	partitionReasonDuplicateListValue  = "partition_list_value_duplicate"
	partitionReasonCoalesceExhausts    = "partition_remove_all"
	partitionReasonAddFormInapplicable = "partition_add_form_inapplicable"
	partitionReasonMemberBoundOverflow = "partition_member_bound"
	partitionReasonOperationFailed     = "partition_operation_failed"
	partitionReasonActionUnsupported   = "partition_action_unsupported"
)

// tidbRefusedPartitionActions is the probed TiDB v8.5 8200 refusal set —
// every named maintenance op. The warn rules suppress under the TiDB
// dialect and the applicability/target rules never apply there; the
// vendor aspect in coverage carries the boundary evidence.
var tidbRefusedPartitionActions = map[string]bool{
	"rebuild_partition":            true,
	"check_partitions":             true,
	"optimize_partition":           true,
	"repair_partition":             true,
	"import_partition_tablespace":  true,
	"discard_partition_tablespace": true,
}

// partitionRangeFamily / partitionListFamily / partitionHashKeyFamily are
// the method families the gates consume — the shared normalized
// vocabulary (lowercase, with the "linear" spellings under hash|key).
func partitionRangeFamily(method string) bool {
	return method == "range" || method == "range columns"
}

func partitionListFamily(method string) bool {
	return method == "list" || method == "list columns"
}

func partitionHashKeyFamily(method string) bool {
	return method == "hash" || method == "key" ||
		method == "linear hash" || method == "linear key"
}

// statementProduct reports the resolved product identity, or "" when the
// version fact never answered.
func statementProduct(statement spec.Statement) string {
	if statement.Metadata == nil || statement.Metadata.Version == nil {
		return ""
	}
	return statement.Metadata.Version.Product
}

// partitionTopologyWarnRule emits one policy finding per authored action
// — the context-dependent risk the #79 contract routes through warning
// (or the discard action's direct data destruction through forbid).
type partitionTopologyWarnRule struct {
	ruleID     string
	action     string
	label      string
	risk       string
	suggestion string
	enabled    bool
	level      rule.Level
}

func newPartitionTopologyWarnRule(ruleID, action, label, risk, suggestion string, fallback rule.Level, cfg policy.RulePolicy) (rule.StatementRule, error) {
	return partitionTopologyWarnRule{
		ruleID:     ruleID,
		action:     action,
		label:      label,
		risk:       risk,
		suggestion: suggestion,
		enabled:    cfg.Enabled,
		level:      configuredLevel(cfg, fallback),
	}, nil
}

func (r partitionTopologyWarnRule) ID() string { return r.ruleID }

func (r partitionTopologyWarnRule) AppliesTo(statement spec.Statement) bool {
	if !r.enabled || !orderedGapDialect(statement.Dialect) {
		return false
	}
	if statement.Dialect == spec.DialectTiDB && tidbRefusedPartitionActions[r.action] {
		// Vendor refusal is the whole answer on TiDB — no policy noise.
		return false
	}
	return len(matchingAlterActions(statement, r.action)) > 0
}

func (r partitionTopologyWarnRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	findings := make([]rule.Finding, 0)
	for _, alter := range matchingAlterActions(statement, r.action) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		metadata := partitionMetadata(statement, alter, nil)
		if len(alter.Partitions) > 0 {
			metadata["partition"] = strings.Join(alter.Partitions, ",")
		}
		defNames := make([]string, 0, len(alter.PartitionDefs))
		for _, def := range alter.PartitionDefs {
			defNames = append(defNames, def.Name)
		}
		if len(defNames) > 0 {
			metadata["partition_defs"] = strings.Join(defNames, ",")
		}
		findings = append(findings, rule.Finding{
			Level:      r.level,
			Message:    fmt.Sprintf("ALTER TABLE %s %s", r.label, r.risk),
			Suggestion: r.suggestion,
			Metadata:   metadata,
		})
	}
	return findings, nil
}

func newReorganizePartitionWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPartitionTopologyWarnRule(ruleIDAlterReorganizePartitionWarn,
		"reorganize_partition", "reorganize partition",
		"redistributes rows between the source members and the declared targets; a dropped LIST value's rows have nowhere to go",
		"verify the target definitions preserve every source boundary/value and take the redistribution window intentionally", rule.LevelWarning, cfg)
}

func newAddPartitionWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPartitionTopologyWarnRule(ruleIDAlterAddPartitionWarn,
		"add_partitions", "add partition",
		"extends the member set; on HASH/KEY methods every existing row redistributes",
		"verify the method's redistribution cost and the new member bounds before shipping", rule.LevelWarning, cfg)
}

func newCoalescePartitionWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPartitionTopologyWarnRule(ruleIDAlterCoalescePartitionWarn,
		"coalesce_partitions", "coalesce partition",
		"removes members and redistributes their rows across the survivors",
		"verify the retained member count and the redistribution window before shipping", rule.LevelWarning, cfg)
}

func newPartitionByWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPartitionTopologyWarnRule(ruleIDAlterPartitionByWarn,
		"partition", "partition by",
		"repartitions the entire table under a new scheme",
		"verify the new method/members and the full-table rewrite window before shipping", rule.LevelWarning, cfg)
}

func newRemovePartitioningWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPartitionTopologyWarnRule(ruleIDAlterRemovePartitioningWarn,
		"remove_partitioning", "remove partitioning",
		"destroys the partitioning layout; member rows merge into a plain table",
		"verify downstream partitioning assumptions before removing the scheme", rule.LevelWarning, cfg)
}

func newRebuildPartitionWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPartitionTopologyWarnRule(ruleIDAlterRebuildPartitionWarn,
		"rebuild_partition", "rebuild partition",
		"rebuilds member storage in place",
		"verify the rebuild window for the named members", rule.LevelWarning, cfg)
}

func newOptimizePartitionWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPartitionTopologyWarnRule(ruleIDAlterOptimizePartitionWarn,
		"optimize_partition", "optimize partition",
		"rebuilds and re-analyzes the named members",
		"verify the rebuild window and the statistics refresh for the named members", rule.LevelWarning, cfg)
}

func newRepairPartitionWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPartitionTopologyWarnRule(ruleIDAlterRepairPartitionWarn,
		"repair_partition", "repair partition",
		"repairs member storage (a rebuild on InnoDB)",
		"verify the repair window for the named members", rule.LevelWarning, cfg)
}

func newCheckPartitionWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPartitionTopologyWarnRule(ruleIDAlterCheckPartitionWarn,
		"check_partitions", "check partition",
		"runs a read-only consistency check on the named members",
		"no structural effect — informational only", rule.LevelNotice, cfg)
}

func newImportPartitionTablespaceWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPartitionTopologyWarnRule(ruleIDAlterImportPartitionTablespaceWarn,
		"import_partition_tablespace", "import partition tablespace",
		"loads an external tablespace into the named members",
		"verify the exported tablespace artifacts and member states before importing", rule.LevelWarning, cfg)
}

func newDiscardPartitionTablespaceForbidRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPartitionForbidRule(ruleIDAlterDiscardPartitionTablespaceForbid,
		"discard_partition_tablespace", "discard partition tablespace", cfg)
}

// partitionTopologyApplicabilityRule binds one T14 action to the proven
// partition state and authored form — the same gates the ordered-state
// transitions prove natively:
//   - every member-consuming action refuses on a proven non-partitioned
//     target (1505) except PARTITION BY, which is the state-creating
//     action itself;
//   - REORGANIZE refuses hash|key methods on TiDB (8200);
//   - COALESCE refuses range|list methods (1509) and n≥members (1508);
//   - ADD refuses `PARTITIONS n` on range|list (1492), the bare form on
//     MySQL (1492 range|list / 1514 hash|key), and member counts past
//     the 8192 bound (1499);
//   - PARTITION BY refuses provably-invalid declared bounds (1493-class);
//   - check/optimize/repair on a non-partitioned target soft-fail
//     (Operation failed rows at rc=0) rather than refuse hard.
type partitionTopologyApplicabilityRule struct {
	ruleID  string
	action  string
	label   string
	enabled bool
	level   rule.Level
}

func newPartitionTopologyApplicabilityRule(ruleID, action, label string, fallback rule.Level, cfg policy.RulePolicy) (rule.StatementRule, error) {
	return partitionTopologyApplicabilityRule{
		ruleID:  ruleID,
		action:  action,
		label:   label,
		enabled: cfg.Enabled,
		level:   configuredLevel(cfg, fallback),
	}, nil
}

func (r partitionTopologyApplicabilityRule) ID() string { return r.ruleID }

// maintenanceSoftFail reports whether the op answers a bad premise with
// status-table "Operation failed" rows at rc=0 instead of a hard refusal.
func maintenanceSoftFail(action string) bool {
	switch action {
	case "check_partitions", "optimize_partition", "repair_partition":
		return true
	}
	return false
}

func (r partitionTopologyApplicabilityRule) AppliesTo(statement spec.Statement) bool {
	if !r.enabled || !orderedGapDialect(statement.Dialect) {
		return false
	}
	if statement.Dialect == spec.DialectTiDB && tidbRefusedPartitionActions[r.action] {
		return false
	}
	return len(matchingAlterActions(statement, r.action)) > 0
}

func (r partitionTopologyApplicabilityRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	// PARTITION BY is the state-creating action — only its declared bound
	// validity gates, against no catalog facts.
	if r.action == "partition" {
		return r.evaluatePartitionBy(statement)
	}
	snapshot, ok := targetTableSnapshot(statement)
	if !ok || snapshot == nil || !snapshot.Exists || snapshot.Partitioned == nil {
		return nil, nil
	}
	if !*snapshot.Partitioned {
		if maintenanceSoftFail(r.action) {
			return r.softFailFindings(statement,
				fmt.Sprintf("ALTER TABLE %s on a non-partitioned table reports Operation failed", r.label)), nil
		}
		return r.refusalFindings(statement, nil,
			partitionReasonNotPartitioned, "1505",
			fmt.Sprintf("ALTER TABLE %s on a non-partitioned table cannot execute (native 1505)", r.label)), nil
	}
	switch r.action {
	case "reorganize_partition":
		if snapshot.PartitionType != nil &&
			partitionHashKeyFamily(*snapshot.PartitionType) &&
			statementProduct(statement) == spec.VersionProductTiDB {
			return r.refusalFindings(statement, snapshot,
				partitionReasonTypeInapplicable, "8200",
				fmt.Sprintf("ALTER TABLE %s is not supported on %s-partitioned tables under TiDB (native 8200)", r.label, *snapshot.PartitionType)), nil
		}
	case "coalesce_partitions":
		if snapshot.PartitionType != nil &&
			(partitionRangeFamily(*snapshot.PartitionType) || partitionListFamily(*snapshot.PartitionType)) {
			return r.refusalFindings(statement, snapshot,
				partitionReasonTypeInapplicable, "1509",
				fmt.Sprintf("ALTER TABLE %s is not allowed on %s-partitioned tables (native 1509)", r.label, *snapshot.PartitionType)), nil
		}
		if snapshot.PartitionNames != nil {
			for _, alter := range matchingAlterActions(statement, r.action) {
				if alter.PartitionCount != nil && len(snapshot.PartitionNames) > 0 &&
					int(*alter.PartitionCount) >= len(snapshot.PartitionNames) {
					return r.refusalFindings(statement, snapshot,
						partitionReasonCoalesceExhausts, "1508",
						fmt.Sprintf("ALTER TABLE %s cannot remove all partitions (native 1508)", r.label)), nil
				}
			}
		}
	case "add_partitions":
		return r.evaluateAddPartitions(statement, snapshot)
	}
	return nil, ctx.Err()
}

// evaluateAddPartitions proves the ADD form gates: the `PARTITIONS n` and
// bare forms are refused on the range|list methods (1492), the bare form
// on MySQL hash|key (1514), and a member count that would overflow the
// native bound (1499). The defs form passes these gates — its bound
// contract lives on the target rule.
func (r partitionTopologyApplicabilityRule) evaluateAddPartitions(statement spec.Statement, snapshot *spec.TableSnapshot) ([]rule.Finding, error) {
	method := ""
	if snapshot.PartitionType != nil {
		method = *snapshot.PartitionType
	}
	for _, alter := range matchingAlterActions(statement, r.action) {
		if len(alter.PartitionDefs) > 0 {
			continue
		}
		if alter.PartitionCount != nil {
			if partitionRangeFamily(method) || partitionListFamily(method) {
				return r.refusalFindings(statement, snapshot,
					partitionReasonAddFormInapplicable, "1492",
					"ALTER TABLE add partition PARTITIONS n requires explicit definitions on range/list methods (native 1492)"), nil
			}
			if partitionHashKeyFamily(method) && snapshot.PartitionNames != nil &&
				int(*alter.PartitionCount)+len(snapshot.PartitionNames) > maxPartitionMembers {
				return r.refusalFindings(statement, snapshot,
					partitionReasonMemberBoundOverflow, "1499",
					"ALTER TABLE add partition would exceed the 8192 member bound (native 1499)"), nil
			}
			continue
		}
		// Bare `ADD PARTITION`: refused on range|list on both anchors and
		// on MySQL hash|key (1514); TiDB hash|key silently no-ops.
		if partitionRangeFamily(method) || partitionListFamily(method) {
			return r.refusalFindings(statement, snapshot,
				partitionReasonAddFormInapplicable, "1492",
				"ALTER TABLE add partition without definitions refuses on range/list methods (native 1492)"), nil
		}
		if partitionHashKeyFamily(method) && statementProduct(statement) == spec.VersionProductMySQL {
			return r.refusalFindings(statement, snapshot,
				partitionReasonAddFormInapplicable, "1514",
				"ALTER TABLE add partition without definitions refuses on hash/key methods (native 1514)"), nil
		}
	}
	return nil, nil
}

// evaluatePartitionBy proves the declared-scheme bound contract: strictly
// increasing comparable RANGE bounds with MAXVALUE only on the tail
// (1493-class), and no provably-duplicate LIST values.
func (r partitionTopologyApplicabilityRule) evaluatePartitionBy(statement spec.Statement) ([]rule.Finding, error) {
	for _, alter := range matchingAlterActions(statement, r.action) {
		scheme := alter.PartitionScheme
		if scheme == nil || len(scheme.Defs) == 0 {
			continue
		}
		if partitionRangeFamily(scheme.Type) {
			var prevRow []spec.BoundLiteral
			prevMax := false
			for i, def := range scheme.Defs {
				row, maxValue, ok := spec.LessThanBoundUpper(def.Bound)
				if !ok {
					return nil, nil
				}
				if prevMax || (maxValue && i != len(scheme.Defs)-1) {
					return r.schemeBoundFinding(statement, alter, def), nil
				}
				if !maxValue && prevRow != nil {
					less, lessOK := spec.BoundRowLessThan(prevRow, row)
					if !lessOK {
						return nil, nil
					}
					if !less {
						return r.schemeBoundFinding(statement, alter, def), nil
					}
				}
				prevRow, prevMax = row, maxValue
			}
		}
		if partitionListFamily(scheme.Type) {
			for i, def := range scheme.Defs {
				for j := 0; j < i; j++ {
					if listBoundsShareValue(def.Bound, scheme.Defs[j].Bound) {
						return r.schemeBoundFinding(statement, alter, def), nil
					}
				}
			}
		}
	}
	return nil, nil
}

func (r partitionTopologyApplicabilityRule) schemeBoundFinding(statement spec.Statement, alter spec.Alter, def spec.PartitionDef) []rule.Finding {
	return []rule.Finding{{
		Level:      r.level,
		Message:    fmt.Sprintf("ALTER TABLE %s declares member %q with a bound the scheme cannot accept (native 1493-class)", r.label, def.Name),
		Suggestion: "verify the declared member bounds are strictly ordered and non-overlapping",
		Metadata: partitionMetadata(statement, alter, map[string]any{
			"partition":   def.Name,
			"reason_code": partitionReasonBoundViolation,
		}),
	}}
}

func (r partitionTopologyApplicabilityRule) refusalFindings(statement spec.Statement, snapshot *spec.TableSnapshot, reasonCode, nativeError, message string) []rule.Finding {
	findings := make([]rule.Finding, 0, len(statement.DDL.Alter))
	for _, alter := range matchingAlterActions(statement, r.action) {
		metadata := partitionMetadata(statement, alter, map[string]any{
			"reason_code":  reasonCode,
			"native_error": nativeError,
		})
		if snapshot != nil && snapshot.Partitioned != nil {
			metadata["partitioned"] = fmt.Sprintf("%t", *snapshot.Partitioned)
		}
		if snapshot != nil && snapshot.PartitionType != nil {
			metadata["partition_type"] = *snapshot.PartitionType
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

// softFailFindings reports the status-table "Operation failed" outcome —
// the statement returns rc=0, so the finding is a warning-class failure
// report, never a fabricated hard refusal.
func (r partitionTopologyApplicabilityRule) softFailFindings(statement spec.Statement, message string) []rule.Finding {
	findings := make([]rule.Finding, 0, len(statement.DDL.Alter))
	for _, alter := range matchingAlterActions(statement, r.action) {
		findings = append(findings, rule.Finding{
			Level:      r.level,
			Message:    message,
			Suggestion: "verify the target actually carries partitioning before shipping",
			Metadata: partitionMetadata(statement, alter, map[string]any{
				"reason_code": partitionReasonNotPartitioned,
				"soft_fail":   "true",
			}),
		})
	}
	return findings
}

// EvidenceGaps reports the partition-state facts each action consumed but
// the projection never answered — mirroring the T13 gap contract: the
// partitioned fact first, then the method fact where the action's gate is
// method-shaped, then the member-set fact where a count/list is consumed,
// and instance.product where a refusal is product-conditional.
func (r partitionTopologyApplicabilityRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(statement) {
		return nil
	}
	if r.action == "partition" {
		return nil // consumes only declared scheme facts
	}
	snapshot, ok := targetTableSnapshot(statement)
	if !ok || snapshot == nil {
		return partitionStateGap("target_table.partitioned")
	}
	if !snapshot.Exists {
		return nil
	}
	if snapshot.Partitioned == nil {
		return partitionStateGap("target_table.partitioned")
	}
	if !*snapshot.Partitioned {
		return nil
	}
	var needed []string
	needType := false
	needMembers := false
	needProduct := false
	switch r.action {
	case "reorganize_partition":
		// The hash|key-on-TiDB refusal consumes method + product.
		needType, needProduct = true, true
	case "coalesce_partitions":
		// The 1509 gate consumes the method; the 1508 gate consumes the
		// member count once the method is hash|key.
		needType = true
		if snapshot.PartitionType != nil && partitionHashKeyFamily(*snapshot.PartitionType) {
			needMembers = true
		}
	case "add_partitions":
		// The form gates consume the method; the overflow check consumes
		// the member count; the bare-form hash|key gate consumes product.
		needType, needProduct = true, true
		if snapshot.PartitionType != nil && partitionHashKeyFamily(*snapshot.PartitionType) {
			for _, alter := range matchingAlterActions(statement, r.action) {
				if len(alter.PartitionDefs) == 0 && alter.PartitionCount != nil {
					needMembers = true
				}
			}
		}
	case "rebuild_partition", "check_partitions", "optimize_partition",
		"repair_partition", "import_partition_tablespace", "discard_partition_tablespace",
		"remove_partitioning":
		// Member-state gates only — partitioned fact already proven.
	}
	if needType && snapshot.PartitionType == nil {
		needed = append(needed, "target_table.partition_type")
	}
	if needMembers && snapshot.PartitionNames == nil {
		needed = append(needed, "target_table.partitions")
	}
	if needProduct && statementProduct(statement) == "" {
		needed = append(needed, "instance.product")
	}
	if len(needed) > 0 {
		return partitionStateGap(needed...)
	}
	return nil
}

func partitionStateGap(facts ...string) []rule.EvidenceGap {
	return []rule.EvidenceGap{{
		ReasonCode:    gapReasonPartitionFacts,
		RequiredFacts: facts,
	}}
}

// partitionTopologyTargetRule binds named members and declared bounds to
// the observed member directory:
//   - REORGANIZE: source presence (1507 mysql / 1567 tidb), consecutive
//     sources (1519 / 8200), no surviving-name collision (1517), and the
//     proven RANGE/LIST bound contract (1520/1493 mysql, 1493 tidb);
//   - ADD defs: name collisions (1517) and provable bound violations
//     (1493/1481/1495);
//   - rebuild/discard/import: missing named member is a hard refusal
//     (1507 / 1735);
//   - check/optimize/repair: missing named member soft-fails (warning).
type partitionTopologyTargetRule struct {
	ruleID      string
	action      string
	label       string
	nativeError string
	enabled     bool
	level       rule.Level
}

func newPartitionTopologyTargetRule(ruleID, action, label, nativeError string, fallback rule.Level, cfg policy.RulePolicy) (rule.StatementRule, error) {
	return partitionTopologyTargetRule{
		ruleID:      ruleID,
		action:      action,
		label:       label,
		nativeError: nativeError,
		enabled:     cfg.Enabled,
		level:       configuredLevel(cfg, fallback),
	}, nil
}

func (r partitionTopologyTargetRule) ID() string { return r.ruleID }

func (r partitionTopologyTargetRule) AppliesTo(statement spec.Statement) bool {
	if !r.enabled || !orderedGapDialect(statement.Dialect) {
		return false
	}
	if statement.Dialect == spec.DialectTiDB && tidbRefusedPartitionActions[r.action] {
		return false
	}
	return len(matchingAlterActions(statement, r.action)) > 0
}

// memberAbsentError resolves the dialect-correct refusal code for a named
// member missing from the observed directory — product-conditional codes
// return "" when the product fact never answered; the refusal itself is
// product-independent.
func memberAbsentError(action string, statement spec.Statement) string {
	product := statementProduct(statement)
	switch action {
	case "reorganize_partition":
		switch product {
		case spec.VersionProductMySQL:
			return "1507"
		case spec.VersionProductTiDB:
			return "1567"
		}
		return ""
	case "rebuild_partition":
		return "1507"
	case "discard_partition_tablespace", "import_partition_tablespace":
		return "1735"
	}
	return ""
}

func (r partitionTopologyTargetRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
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
		switch r.action {
		case "reorganize_partition":
			findings = append(findings, r.reorganizeFindings(statement, snapshot, alter, members)...)
		case "add_partitions":
			findings = append(findings, r.addFindings(statement, snapshot, alter)...)
		default:
			// rebuild/discard/import hard refusals; check/optimize/repair
			// soft-fails — both on proven member absence.
			for _, name := range alter.Partitions {
				if members[strings.ToLower(name)] {
					continue
				}
				findings = append(findings, r.memberAbsentFinding(statement, alter, name))
			}
		}
	}
	return findings, nil
}

func (r partitionTopologyTargetRule) memberAbsentFinding(statement spec.Statement, alter spec.Alter, name string) rule.Finding {
	metadata := partitionMetadata(statement, alter, map[string]any{
		"partition":   name,
		"exists":      "false",
		"reason_code": partitionReasonPartitionAbsent,
	})
	var message string
	if maintenanceSoftFail(r.action) {
		metadata["soft_fail"] = "true"
		message = fmt.Sprintf("ALTER TABLE %s names partition %q, which is not a member of the table — the operation reports failure", r.label, name)
	} else {
		metadata["native_error"] = memberAbsentError(r.action, statement)
		message = fmt.Sprintf("ALTER TABLE %s names partition %q, which is not a member of the table (native %s)", r.label, name, memberAbsentError(r.action, statement))
	}
	return rule.Finding{
		Level:      r.level,
		Message:    message,
		Suggestion: "verify the partition name against the observed member set",
		Metadata:   metadata,
	}
}

// reorganizeFindings proves the REORGANIZE member contract: every named
// source must exist, sit consecutively, and leave a non-colliding
// surviving directory — then the RANGE/LIST bound contract decides.
func (r partitionTopologyTargetRule) reorganizeFindings(statement spec.Statement, snapshot *spec.TableSnapshot, alter spec.Alter, members map[string]bool) []rule.Finding {
	indexes, found := partitionMemberIndexesOf(snapshot.PartitionNames, alter.Partitions)
	if !found {
		missing := firstAbsent(alter.Partitions, members)
		code := memberAbsentError("reorganize_partition", statement)
		message := fmt.Sprintf("ALTER TABLE %s names source partition %q, which is not a member of the table", r.label, missing)
		metadata := partitionMetadata(statement, alter, map[string]any{
			"partition":   missing,
			"exists":      "false",
			"reason_code": partitionReasonPartitionAbsent,
		})
		if code != "" {
			metadata["native_error"] = code
			message += fmt.Sprintf(" (native %s)", code)
		}
		return []rule.Finding{{
			Level:      r.level,
			Message:    message,
			Suggestion: "verify the source partition names against the observed member set",
			Metadata:   metadata,
		}}
	}
	for i := 1; i < len(indexes); i++ {
		if indexes[i] != indexes[i-1]+1 {
			metadata := partitionMetadata(statement, alter, map[string]any{
				"reason_code": partitionReasonSourcesNonAdjacent,
			})
			message := fmt.Sprintf("ALTER TABLE %s sources are not in consecutive order", r.label)
			switch statementProduct(statement) {
			case spec.VersionProductTiDB:
				metadata["native_error"] = "8200"
				message += " (native 8200)"
			case spec.VersionProductMySQL:
				metadata["native_error"] = "1519"
				message += " (native 1519)"
			}
			return []rule.Finding{{
				Level:      r.level,
				Message:    message,
				Suggestion: "reorganize a consecutive span of members or split the change",
				Metadata:   metadata,
			}}
		}
	}
	removed := make(map[int]bool, len(indexes))
	surviving := make(map[string]bool, len(snapshot.PartitionNames))
	for _, name := range snapshot.PartitionNames {
		surviving[strings.ToLower(name)] = true
	}
	for _, i := range indexes {
		removed[i] = true
		delete(surviving, strings.ToLower(snapshot.PartitionNames[i]))
	}
	for _, def := range alter.PartitionDefs {
		if surviving[strings.ToLower(def.Name)] {
			return []rule.Finding{{
				Level:      r.level,
				Message:    fmt.Sprintf("ALTER TABLE %s declares partition %q, which collides with a surviving member (native 1517)", r.label, def.Name),
				Suggestion: "choose member names outside the surviving directory",
				Metadata: partitionMetadata(statement, alter, map[string]any{
					"partition":    def.Name,
					"reason_code":  partitionReasonDuplicateName,
					"native_error": "1517",
				}),
			}}
		}
	}
	if snapshot.PartitionDefs == nil {
		return nil
	}
	method := ""
	if snapshot.PartitionType != nil {
		method = *snapshot.PartitionType
	}
	switch {
	case partitionRangeFamily(method):
		if finding, ok := reorganizeRangeFinding(r, statement, alter, snapshot, indexes); ok {
			return []rule.Finding{finding}
		}
	case partitionListFamily(method):
		for i, def := range alter.PartitionDefs {
			for j := 0; j < i; j++ {
				if listBoundsShareValue(def.Bound, alter.PartitionDefs[j].Bound) {
					return []rule.Finding{r.listDuplicateFinding(statement, alter, def)}
				}
			}
			for m, existing := range snapshot.PartitionDefs {
				if removed[m] {
					continue
				}
				if listBoundsShareValue(def.Bound, existing.Bound) {
					return []rule.Finding{r.listDuplicateFinding(statement, alter, def)}
				}
			}
		}
	}
	return nil
}

// reorganizeRangeFinding proves the RANGE bound contract for one
// reorganization: strictly increasing comparable targets whose last
// member provably preserves the covered span's upper edge — exact
// preservation for a mid-table span, or extension allowed only when the
// source was the terminal member (1520 mysql / 1493 tidb).
func reorganizeRangeFinding(r partitionTopologyTargetRule, statement spec.Statement, alter spec.Alter, snapshot *spec.TableSnapshot, indexes []int) (rule.Finding, bool) {
	targets := alter.PartitionDefs
	defs := snapshot.PartitionDefs
	lastIdx := indexes[len(indexes)-1]
	sourceRow, sourceMax, ok := spec.LessThanBoundUpper(defs[lastIdx].Bound)
	if !ok {
		return rule.Finding{}, false
	}
	var prevRow []spec.BoundLiteral
	prevMax := false
	prevOK := true
	if idx := indexes[0] - 1; idx >= 0 {
		prevRow, prevMax, prevOK = spec.LessThanBoundUpper(defs[idx].Bound)
	}
	for i, def := range targets {
		row, maxValue, rowOK := spec.LessThanBoundUpper(def.Bound)
		if !rowOK {
			return rule.Finding{}, false
		}
		if maxValue && i != len(targets)-1 {
			return r.rangeBoundFinding(statement, alter, def.Name), true
		}
		if maxValue {
			continue
		}
		switch {
		case i == 0 && indexes[0] > 0:
			if !prevOK || prevMax {
				return rule.Finding{}, false
			}
			if less, lessOK := spec.BoundRowLessThan(prevRow, row); !lessOK {
				return rule.Finding{}, false
			} else if !less {
				return r.rangeBoundFinding(statement, alter, def.Name), true
			}
		case i > 0:
			prevTarget := targets[i-1]
			prevTargetRow, prevTargetMax, targetOK := spec.LessThanBoundUpper(prevTarget.Bound)
			if !targetOK || prevTargetMax {
				return rule.Finding{}, false
			}
			if less, lessOK := spec.BoundRowLessThan(prevTargetRow, row); !lessOK {
				return rule.Finding{}, false
			} else if !less {
				return r.rangeBoundFinding(statement, alter, def.Name), true
			}
		}
	}
	lastRow, lastMax, lastOK := spec.LessThanBoundUpper(targets[len(targets)-1].Bound)
	if !lastOK {
		return rule.Finding{}, false
	}
	sourceIsTerminal := lastIdx == len(defs)-1
	switch {
	case sourceMax:
		if !lastMax {
			return r.rangeBoundFinding(statement, alter, targets[len(targets)-1].Name), true
		}
	case lastMax:
		if !sourceIsTerminal {
			return r.rangeBoundFinding(statement, alter, targets[len(targets)-1].Name), true
		}
	default:
		cmp, cmpOK := spec.BoundRowCompare(lastRow, sourceRow)
		if !cmpOK {
			return rule.Finding{}, false
		}
		if (sourceIsTerminal && cmp < 0) || (!sourceIsTerminal && cmp != 0) {
			return r.rangeBoundFinding(statement, alter, targets[len(targets)-1].Name), true
		}
	}
	return rule.Finding{}, false
}

func (r partitionTopologyTargetRule) rangeBoundFinding(statement spec.Statement, alter spec.Alter, name string) rule.Finding {
	metadata := partitionMetadata(statement, alter, map[string]any{
		"partition":   name,
		"reason_code": partitionReasonBoundViolation,
	})
	message := fmt.Sprintf("ALTER TABLE %s declares partition %q with bounds that violate the covered span", r.label, name)
	switch statementProduct(statement) {
	case spec.VersionProductTiDB:
		metadata["native_error"] = "1493"
		message += " (native 1493)"
	case spec.VersionProductMySQL:
		metadata["native_error"] = "1520"
		message += " (native 1520)"
	}
	return rule.Finding{
		Level:      r.level,
		Message:    message,
		Suggestion: "keep strictly increasing bounds preserving the source span's upper edge",
		Metadata:   metadata,
	}
}

func (r partitionTopologyTargetRule) listDuplicateFinding(statement spec.Statement, alter spec.Alter, def spec.PartitionDef) rule.Finding {
	return rule.Finding{
		Level:      r.level,
		Message:    fmt.Sprintf("ALTER TABLE %s declares partition %q with values that overlap an existing member (native 1495)", r.label, def.Name),
		Suggestion: "keep each literal value in exactly one member",
		Metadata: partitionMetadata(statement, alter, map[string]any{
			"partition":    def.Name,
			"reason_code":  partitionReasonDuplicateListValue,
			"native_error": "1495",
		}),
	}
}

// addFindings proves the ADD defs contract: no surviving-name collision
// (1517), and on the range/list methods the declared bounds must satisfy
// the ordering tail contract (1493/1481/1495).
func (r partitionTopologyTargetRule) addFindings(statement spec.Statement, snapshot *spec.TableSnapshot, alter spec.Alter) []rule.Finding {
	if len(alter.PartitionDefs) == 0 {
		return nil
	}
	existing := make(map[string]bool, len(snapshot.PartitionNames))
	for _, name := range snapshot.PartitionNames {
		existing[strings.ToLower(name)] = true
	}
	for _, def := range alter.PartitionDefs {
		if existing[strings.ToLower(def.Name)] {
			return []rule.Finding{{
				Level:      r.level,
				Message:    fmt.Sprintf("ALTER TABLE %s declares partition %q, which already exists (native 1517)", r.label, def.Name),
				Suggestion: "choose a member name outside the existing directory",
				Metadata: partitionMetadata(statement, alter, map[string]any{
					"partition":    def.Name,
					"reason_code":  partitionReasonDuplicateName,
					"native_error": "1517",
				}),
			}}
		}
	}
	if snapshot.PartitionDefs == nil {
		return nil
	}
	method := ""
	if snapshot.PartitionType != nil {
		method = *snapshot.PartitionType
	}
	switch {
	case partitionRangeFamily(method):
		if n := len(snapshot.PartitionDefs); n > 0 {
			if _, tailMax, ok := spec.LessThanBoundUpper(snapshot.PartitionDefs[n-1].Bound); !ok {
				return nil
			} else if tailMax {
				return []rule.Finding{{
					Level:      r.level,
					Message:    fmt.Sprintf("ALTER TABLE %s cannot add members after a MAXVALUE tail (native 1481)", r.label),
					Suggestion: "reorganize the MAXVALUE member instead of adding past it",
					Metadata: partitionMetadata(statement, alter, map[string]any{
						"reason_code":  partitionReasonMaxValueTail,
						"native_error": "1481",
					}),
				}}
			}
		}
		var prevRow []spec.BoundLiteral
		if n := len(snapshot.PartitionDefs); n > 0 {
			row, _, ok := spec.LessThanBoundUpper(snapshot.PartitionDefs[n-1].Bound)
			if !ok {
				return nil
			}
			prevRow = row
		}
		for _, def := range alter.PartitionDefs {
			row, maxValue, ok := spec.LessThanBoundUpper(def.Bound)
			if !ok {
				return nil
			}
			if !maxValue && prevRow != nil {
				less, lessOK := spec.BoundRowLessThan(prevRow, row)
				if !lessOK {
					return nil
				}
				if !less {
					return []rule.Finding{r.rangeBoundFinding(statement, alter, def.Name)}
				}
			}
			prevRow = row
		}
	case partitionListFamily(method):
		for i, def := range alter.PartitionDefs {
			for j := 0; j < i; j++ {
				if listBoundsShareValue(def.Bound, alter.PartitionDefs[j].Bound) {
					return []rule.Finding{r.listDuplicateFinding(statement, alter, def)}
				}
			}
			for _, existing := range snapshot.PartitionDefs {
				if listBoundsShareValue(def.Bound, existing.Bound) {
					return []rule.Finding{r.listDuplicateFinding(statement, alter, def)}
				}
			}
		}
	}
	return nil
}

// EvidenceGaps mirrors the T13 member-target contract: when the authored
// action consumes member names or member bounds, the rule owes the
// partitioned fact, then the member-set fact, then the member-defs fact
// for bound-contract actions.
func (r partitionTopologyTargetRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(statement) {
		return nil
	}
	consumes := false
	consumesDefs := false
	for _, alter := range matchingAlterActions(statement, r.action) {
		switch r.action {
		case "reorganize_partition":
			if len(alter.Partitions) > 0 || len(alter.PartitionDefs) > 0 {
				consumes = true
			}
		case "add_partitions":
			if len(alter.PartitionDefs) > 0 {
				consumesDefs = true
			}
		default:
			if !alter.AllPartitions && len(alter.Partitions) > 0 {
				consumes = true
			}
		}
	}
	if !consumes && !consumesDefs {
		return nil
	}
	snapshot, ok := targetTableSnapshot(statement)
	if !ok || snapshot == nil {
		return partitionStateGap("target_table.partitioned")
	}
	if !snapshot.Exists {
		return nil
	}
	if snapshot.Partitioned == nil {
		return partitionStateGap("target_table.partitioned")
	}
	if !*snapshot.Partitioned {
		return nil
	}
	var needed []string
	if consumes && snapshot.PartitionNames == nil {
		needed = append(needed, "target_table.partitions")
	}
	if consumes || consumesDefs {
		if snapshot.PartitionType == nil && (r.action == "reorganize_partition" || r.action == "add_partitions") {
			// The bound contract's method family is unproven.
			needed = append(needed, "target_table.partition_type")
		}
		if snapshot.PartitionDefs == nil && boundContractAction(r.action, snapshot.PartitionType) {
			needed = append(needed, "target_table.partition_defs")
		}
	}
	if len(needed) > 0 {
		return partitionStateGap(needed...)
	}
	return nil
}

// boundContractAction reports whether the action's member checks consume
// declared/observed bound facts under the method's clause shape.
func boundContractAction(action string, method *string) bool {
	if method == nil {
		return false
	}
	switch action {
	case "reorganize_partition", "add_partitions":
		return partitionRangeFamily(*method) || partitionListFamily(*method)
	}
	return false
}

// partitionMemberIndexesOf locates each named source member in the proven
// directory — the rules-side twin of the audit-package helper (the domain
// layer cannot import application internals).
func partitionMemberIndexesOf(names, sources []string) ([]int, bool) {
	index := make(map[string]int, len(names))
	for i, name := range names {
		index[strings.ToLower(name)] = i
	}
	out := make([]int, 0, len(sources))
	for _, source := range sources {
		i, ok := index[strings.ToLower(source)]
		if !ok {
			return nil, false
		}
		out = append(out, i)
	}
	return out, true
}

// firstAbsent returns the first named member missing from the directory.
func firstAbsent(names []string, members map[string]bool) string {
	for _, name := range names {
		if !members[strings.ToLower(name)] {
			return name
		}
	}
	return ""
}

// listBoundsShareValue reports whether two IN bounds provably share a
// value row — literal-row equality only; expression items never count as
// proven overlap.
func listBoundsShareValue(a, b *spec.PartitionBound) bool {
	if a == nil || b == nil {
		return false
	}
	for _, rowA := range a.Rows {
		for _, rowB := range b.Rows {
			if equal, ok := spec.BoundRowEqual(rowA, rowB); ok && equal {
				return true
			}
		}
	}
	return false
}

const maxPartitionMembers = 8192

// Factory wrappers — one constructor per frozen rule ID.
func newReorganizePartitionApplicabilityRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPartitionTopologyApplicabilityRule(ruleIDAlterReorganizePartitionApplicability, "reorganize_partition", "reorganize partition", rule.LevelBlocker, cfg)
}

func newAddPartitionApplicabilityRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPartitionTopologyApplicabilityRule(ruleIDAlterAddPartitionApplicability, "add_partitions", "add partition", rule.LevelBlocker, cfg)
}

func newCoalescePartitionApplicabilityRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPartitionTopologyApplicabilityRule(ruleIDAlterCoalescePartitionApplicability, "coalesce_partitions", "coalesce partition", rule.LevelBlocker, cfg)
}

func newRemovePartitioningApplicabilityRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPartitionTopologyApplicabilityRule(ruleIDAlterRemovePartitioningApplicability, "remove_partitioning", "remove partitioning", rule.LevelBlocker, cfg)
}

func newPartitionByApplicabilityRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPartitionTopologyApplicabilityRule(ruleIDAlterPartitionByApplicability, "partition", "partition by", rule.LevelBlocker, cfg)
}

func newRebuildPartitionApplicabilityRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPartitionTopologyApplicabilityRule(ruleIDAlterRebuildPartitionApplicability, "rebuild_partition", "rebuild partition", rule.LevelBlocker, cfg)
}

func newOptimizePartitionApplicabilityRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPartitionTopologyApplicabilityRule(ruleIDAlterOptimizePartitionApplicability, "optimize_partition", "optimize partition", rule.LevelWarning, cfg)
}

func newRepairPartitionApplicabilityRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPartitionTopologyApplicabilityRule(ruleIDAlterRepairPartitionApplicability, "repair_partition", "repair partition", rule.LevelWarning, cfg)
}

func newCheckPartitionApplicabilityRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPartitionTopologyApplicabilityRule(ruleIDAlterCheckPartitionApplicability, "check_partitions", "check partition", rule.LevelWarning, cfg)
}

func newImportPartitionTablespaceApplicabilityRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPartitionTopologyApplicabilityRule(ruleIDAlterImportPartitionTablespaceApplicability, "import_partition_tablespace", "import partition tablespace", rule.LevelBlocker, cfg)
}

func newDiscardPartitionTablespaceApplicabilityRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPartitionTopologyApplicabilityRule(ruleIDAlterDiscardPartitionTablespaceApplicability, "discard_partition_tablespace", "discard partition tablespace", rule.LevelBlocker, cfg)
}

func newReorganizePartitionTargetRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPartitionTopologyTargetRule(ruleIDAlterReorganizePartitionTarget, "reorganize_partition", "reorganize partition", "", rule.LevelBlocker, cfg)
}

func newAddPartitionTargetRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPartitionTopologyTargetRule(ruleIDAlterAddPartitionTarget, "add_partitions", "add partition", "", rule.LevelBlocker, cfg)
}

func newRebuildPartitionTargetRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPartitionTopologyTargetRule(ruleIDAlterRebuildPartitionTarget, "rebuild_partition", "rebuild partition", "1507", rule.LevelBlocker, cfg)
}

func newOptimizePartitionTargetRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPartitionTopologyTargetRule(ruleIDAlterOptimizePartitionTarget, "optimize_partition", "optimize partition", "", rule.LevelWarning, cfg)
}

func newRepairPartitionTargetRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPartitionTopologyTargetRule(ruleIDAlterRepairPartitionTarget, "repair_partition", "repair partition", "", rule.LevelWarning, cfg)
}

func newCheckPartitionTargetRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPartitionTopologyTargetRule(ruleIDAlterCheckPartitionTarget, "check_partitions", "check partition", "", rule.LevelWarning, cfg)
}

func newImportPartitionTablespaceTargetRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPartitionTopologyTargetRule(ruleIDAlterImportPartitionTablespaceTarget, "import_partition_tablespace", "import partition tablespace", "1735", rule.LevelBlocker, cfg)
}

func newDiscardPartitionTablespaceTargetRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPartitionTopologyTargetRule(ruleIDAlterDiscardPartitionTablespaceTarget, "discard_partition_tablespace", "discard partition tablespace", "1735", rule.LevelBlocker, cfg)
}
