// Package ddl defines Tier-1 DDL rules.
package ddl

import (
	"context"
	"fmt"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// auto_random_alter_rules.go implements the T23-R3 TiDB ALTER-side
// AUTO_RANDOM and layout-option validity rules. The matrices themselves
// live in spec.AlterAutoRandomColumnVerdict /
// spec.AlterLayoutOptionOutcome — the same verdicts the ordered-state
// publisher consumes, so a proven refusal can never publish a successor
// shape and an unproven premise is an evidence gap, never a guess.

const (
	alterAutoRandomFactOldColumn    = "table.columns"
	alterAutoRandomFactBits         = "table.auto_random_bits"
	alterAutoRandomFactOptionsValue = "table.options.value"
	alterAutoRandomFactClustered    = "table.primary_key.clustered"
	alterAutoRandomFactAllocator    = "runtime.auto_random_allocator"
	alterAutoRandomFactRows         = "runtime.row_data"
)

type alterAutoRandomValidRule struct {
	level rule.Level
}

func newAlterAutoRandomValidRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return alterAutoRandomValidRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r alterAutoRandomValidRule) ID() string { return ruleIDTiDBAlterAutoRandomValid }

// alterAutoRandomTargets returns the column actions whose declaration can
// reach checkAutoRandom: ADD (attribute refusal), MODIFY and CHANGE (the
// shared matrix). RENAME COLUMN never enters the matrix.
func alterAutoRandomTargets(statement spec.Statement) []*spec.Alter {
	if statement.DDL == nil {
		return nil
	}
	var targets []*spec.Alter
	for i := range statement.DDL.Alter {
		alter := &statement.DDL.Alter[i]
		switch alter.Action {
		case "add_columns", "modify_column", "change_column":
			targets = append(targets, alter)
		}
	}
	return targets
}

func (r alterAutoRandomValidRule) AppliesTo(statement spec.Statement) bool {
	return statement.Dialect == spec.DialectTiDB &&
		statement.Kind == spec.KindDDL &&
		statement.DDL != nil &&
		statement.DDL.Operation == spec.DDLOperationAlterTable &&
		len(alterAutoRandomTargets(statement)) > 0
}

// alterAutoRandomVerdicts runs the shared verdict for one action against
// the metadata-proven target shape (nil snapshot = every old fact
// unproven). The ADD arm resolves from the declared definition alone.
func alterAutoRandomVerdicts(alter *spec.Alter, snapshot *spec.TableSnapshot) (verdict spec.AutoRandomVerdict, reason string) {
	definition := (*spec.Column)(nil)
	oldName := ""
	if alter.Column != nil {
		definition = alter.Column.Definition
		oldName = alter.Column.OldName
		if oldName == "" {
			oldName = alter.Name
		}
	}
	if alter.Action == "add_columns" {
		return spec.AlterAutoRandomAddColumnVerdict(definition)
	}
	oldType, oldAutoInc := "", false
	oldFound := false
	if snapshot != nil {
		for i := range snapshot.Columns {
			if strings.EqualFold(strings.TrimSpace(snapshot.Columns[i].Name), oldName) {
				oldType = snapshot.Columns[i].Type
				oldAutoInc = snapshot.Columns[i].AutoIncrement
				oldFound = true
				break
			}
		}
	}
	member, memberKnown, shardBits, rangeBits, isPKHandle, hasAuto := spec.AlterAutoRandomOldFacts(snapshot, oldName)
	if member && !memberKnown && !hasAuto && (definition == nil || !definition.AutoRandom) {
		// Vacuous member question: a table provably without AUTO_RANDOM
		// reads old bits of zero on both arms — an untouched declaration
		// needs no clustered premise.
		memberKnown, shardBits, rangeBits = true, 0, 0
	}
	if snapshot == nil || !oldFound {
		memberKnown = false
	}
	return spec.AlterAutoRandomColumnVerdict(shardBits, rangeBits, memberKnown,
		oldType, oldAutoInc, isPKHandle, definition)
}

func alterAutoRandomRefusalText(action, reason string) (message, suggestion string) {
	switch reason {
	case spec.AlterReasonAddColumnAutoRandom:
		return "cannot be added with AUTO_RANDOM: TiDB refuses AUTO_RANDOM on ADD COLUMN",
			"add the column without AUTO_RANDOM, or convert an existing AUTO_INCREMENT primary key with MODIFY COLUMN"
	case spec.AlterReasonRemoveAutoRandom:
		return "cannot drop the AUTO_RANDOM attribute: TiDB refuses auto_random removal",
			"keep AUTO_RANDOM on the column"
	case spec.AlterReasonDecreaseShardBits:
		return "cannot decrease AUTO_RANDOM shard bits",
			"keep or increase the shard bits"
	case spec.AlterReasonChangeFromAutoIncrement:
		return "can gain AUTO_RANDOM only from an AUTO_INCREMENT primary-key handle column",
			"convert the AUTO_INCREMENT primary key instead"
	case spec.AlterReasonModifyColumnType:
		return "cannot change the column type of an AUTO_RANDOM column",
			"keep the column type"
	case spec.AlterReasonNonBigint:
		return "must be BIGINT: TiDB supports AUTO_RANDOM only on BIGINT columns",
			"change the column type to BIGINT"
	case spec.AlterReasonWithAutoIncrement:
		return "cannot combine AUTO_RANDOM with AUTO_INCREMENT",
			"remove AUTO_INCREMENT from the column"
	case spec.AlterReasonWithDefault:
		return "cannot combine AUTO_RANDOM with an explicit DEFAULT",
			"remove the DEFAULT clause from the column"
	case spec.AlterReasonRangeBitsChanged:
		return "cannot change AUTO_RANDOM range bits",
			"keep the range bits unchanged"
	case spec.AlterReasonShardBits:
		return "has shard bits outside the allowed range 1..15",
			"choose AUTO_RANDOM shard bits between 1 and 15"
	case spec.AlterReasonRangeBits:
		return "has range bits outside the allowed range 32..64",
			"choose AUTO_RANDOM range bits between 32 and 64"
	default:
		return "is statically refused by TiDB",
			"adjust the AUTO_RANDOM declaration"
	}
}

func (r alterAutoRandomValidRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	snapshot := (*spec.TableSnapshot)(nil)
	if statement.Metadata != nil {
		snapshot = statement.Metadata.TargetTable
	}
	findings := make([]rule.Finding, 0)
	for _, alter := range alterAutoRandomTargets(statement) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		verdict, reason := alterAutoRandomVerdicts(alter, snapshot)
		if verdict != spec.AutoRandomRefused {
			continue
		}
		message, suggestion := alterAutoRandomRefusalText(alter.Action, reason)
		name := ""
		if alter.Column != nil && alter.Column.Definition != nil {
			name = alter.Column.Definition.Name
		}
		findings = append(findings, rule.Finding{
			Level:      r.level,
			Message:    fmt.Sprintf("auto_random column %q %s", name, message),
			Suggestion: suggestion,
			Metadata: map[string]any{
				"table":  statement.DDL.Table.Name,
				"column": name,
				"action": alter.Action,
				"reason": reason,
			},
		})
	}
	return findings, nil
}

// EvidenceGaps surfaces the premises an undecidable ALTER verdict owes:
// the old column/member facts, the runtime allocator overflow check, or
// the authored bit values themselves.
func (r alterAutoRandomValidRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(statement) {
		return nil
	}
	snapshot := (*spec.TableSnapshot)(nil)
	if statement.Metadata != nil {
		snapshot = statement.Metadata.TargetTable
	}
	facts := map[string]bool{}
	for _, alter := range alterAutoRandomTargets(statement) {
		verdict, reason := alterAutoRandomVerdicts(alter, snapshot)
		if verdict != spec.AutoRandomUndecidable {
			continue
		}
		if reason == spec.AlterReasonRuntimeAllocatorUnproven {
			facts[alterAutoRandomFactAllocator] = true
			continue
		}
		facts[alterAutoRandomFactOldColumn] = true
		facts[alterAutoRandomFactBits] = true
	}
	if len(facts) == 0 {
		return nil
	}
	required := make([]string, 0, len(facts))
	for fact := range facts {
		required = append(required, fact)
	}
	return []rule.EvidenceGap{{
		ReasonCode:    gapReasonMissingInstanceFact,
		RequiredFacts: required,
	}}
}

type alterTableOptionsValidRule struct {
	level rule.Level
}

func newAlterTableOptionsValidRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return alterTableOptionsValidRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r alterTableOptionsValidRule) ID() string { return ruleIDTiDBAlterTableOptionsValid }

func (r alterTableOptionsValidRule) AppliesTo(statement spec.Statement) bool {
	if statement.Dialect != spec.DialectTiDB || statement.Kind != spec.KindDDL ||
		statement.DDL == nil || statement.DDL.Operation != spec.DDLOperationAlterTable {
		return false
	}
	for i := range statement.DDL.Alter {
		alter := &statement.DDL.Alter[i]
		if alter.Action != "table_option" {
			continue
		}
		for _, name := range alterLayoutOptionKeys {
			if _, ok := alter.Options[name]; ok {
				return true
			}
		}
	}
	return false
}

var alterLayoutOptionKeys = []string{"auto_random_base", "auto_id_cache", "shard_row_id_bits", "pre_split_regions"}

// alterLayoutFacts builds the verdict facts from the metadata-proven
// target snapshot — nil snapshot leaves every target fact unproven.
func alterLayoutFacts(snapshot *spec.TableSnapshot) spec.AlterLayoutFacts {
	if snapshot == nil {
		return spec.AlterLayoutFacts{}
	}
	facts := spec.AlterLayoutFacts{
		Options:   snapshot.Options,
		Temporary: snapshot.Temporary || snapshot.GlobalTemporary,
	}
	if snapshot.PrimaryKeyIsHandle || snapshot.PrimaryKeyClustered {
		clustered := true
		facts.Clustered = &clustered
	} else if !snapshot.PrimaryKeyClusteredUnknown {
		clustered := false
		facts.Clustered = &clustered
	}
	for _, column := range snapshot.Columns {
		if column.AutoRandom {
			facts.AutoRandomBits, facts.AutoRandomRangeBits = spec.AutoRandomEffectiveBits(column)
			facts.AutoRandomBitsKnown = true
			facts.AutoRandomPKHasUnsigned = column.Unsigned
			break
		}
	}
	facts.HasPKColumn = snapshot.PrimaryKey != nil && len(snapshot.PrimaryKey.Columns) > 0
	return facts
}

func alterOptionRefusalText(reason string) (message, suggestion string) {
	switch reason {
	case spec.AlterOptionReasonPreSplitUnsupported:
		return "PRE_SPLIT_REGIONS is not supported on ALTER TABLE; TiDB refuses the option",
			"declare PRE_SPLIT_REGIONS only at CREATE TABLE"
	case spec.AlterOptionReasonAutoIDCacheOverflow:
		return "AUTO_ID_CACHE exceeds int64; TiDB refuses the table option",
			"choose an AUTO_ID_CACHE value at or below 9223372036854775807"
	case spec.AlterOptionReasonAutoIDCacheBoundary:
		return "AUTO_ID_CACHE cannot move between 1 and non-1 values",
			"keep AUTO_ID_CACHE on the same side of the 1 boundary"
	case spec.AlterOptionReasonShardTemporary:
		return "SHARD_ROW_ID_BITS on a temporary table is refused by TiDB",
			"remove SHARD_ROW_ID_BITS or alter a permanent table"
	case spec.AlterOptionReasonShardClustered:
		return "SHARD_ROW_ID_BITS > 0 on a clustered primary key is refused by TiDB",
			"use a nonclustered primary key or drop SHARD_ROW_ID_BITS"
	case spec.AlterOptionReasonRebaseNotApplicable:
		return "AUTO_RANDOM_BASE applies only to a table with AUTO_RANDOM bits and a primary key",
			"rebase only a table that has an AUTO_RANDOM primary key"
	case spec.AlterOptionReasonRebaseOverflow:
		return "AUTO_RANDOM_BASE exceeds the incremental-bits capacity of the table's layout",
			"choose a base below the incremental capacity"
	default:
		return "is statically refused by TiDB",
			"adjust the declared table option"
	}
}

func (r alterTableOptionsValidRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	snapshot := (*spec.TableSnapshot)(nil)
	if statement.Metadata != nil {
		snapshot = statement.Metadata.TargetTable
	}
	facts := alterLayoutFacts(snapshot)
	findings := make([]rule.Finding, 0)
	for i := range statement.DDL.Alter {
		alter := &statement.DDL.Alter[i]
		if alter.Action != "table_option" {
			continue
		}
		for _, name := range alterLayoutOptionKeys {
			value, ok := alter.Options[name]
			if !ok {
				continue
			}
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			verdict, reason := spec.AlterLayoutOptionOutcome(name, value, facts)
			if verdict != spec.AutoRandomRefused {
				continue
			}
			message, suggestion := alterOptionRefusalText(reason)
			findings = append(findings, rule.Finding{
				Level:      r.level,
				Message:    fmt.Sprintf("table %q option %s", statement.DDL.Table.Name, message),
				Suggestion: suggestion,
				Metadata:   map[string]any{"table": statement.DDL.Table.Name, "option": name, "reason": reason},
			})
		}
	}
	return findings, nil
}

// EvidenceGaps surfaces the premises an undecidable option verdict owes:
// the old catalog value, the clustered answer, the AUTO_RANDOM layout
// bits, or the runtime row/allocator facts.
func (r alterTableOptionsValidRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(statement) {
		return nil
	}
	snapshot := (*spec.TableSnapshot)(nil)
	if statement.Metadata != nil {
		snapshot = statement.Metadata.TargetTable
	}
	facts := alterLayoutFacts(snapshot)
	required := map[string]bool{}
	for i := range statement.DDL.Alter {
		alter := &statement.DDL.Alter[i]
		if alter.Action != "table_option" {
			continue
		}
		for _, name := range alterLayoutOptionKeys {
			value, ok := alter.Options[name]
			if !ok {
				continue
			}
			verdict, reason := spec.AlterLayoutOptionOutcome(name, value, facts)
			if verdict != spec.AutoRandomUndecidable {
				continue
			}
			switch reason {
			case spec.AlterOptionReasonOldValueUnproven, spec.AlterOptionReasonValueUnproven:
				required[alterAutoRandomFactOptionsValue] = true
			case spec.AlterOptionReasonClusteredUnproven:
				required[alterAutoRandomFactClustered] = true
			case spec.AlterOptionReasonAutoRandomBitsUnprov:
				required[alterAutoRandomFactBits] = true
			case spec.AlterOptionReasonRuntimeRowsUnproven:
				required[alterAutoRandomFactRows] = true
			default:
				required[alterAutoRandomFactOptionsValue] = true
			}
		}
	}
	if len(required) == 0 {
		return nil
	}
	list := make([]string, 0, len(required))
	for fact := range required {
		list = append(list, fact)
	}
	return []rule.EvidenceGap{{
		ReasonCode:    gapReasonMissingInstanceFact,
		RequiredFacts: list,
	}}
}
