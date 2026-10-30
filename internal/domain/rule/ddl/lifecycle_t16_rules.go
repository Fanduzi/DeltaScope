// T16 lifecycle rules consume the namespace-aware ordered-state bindings
// (Metadata.TargetTables / SourceTable / SourceTables) instead of the
// primary TargetTable alone: multi-target DROP members, RENAME endpoint
// pairs, LIKE/CTAS sources, proven inbound-FK edges, and TiDB's
// local-temporary refusal surface.
package ddl

import (
	"context"
	"fmt"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// boundTargetBindings returns the statement's bound target answers —
// Metadata.TargetTables when the ordered projection published them, else a
// single synthesized binding over the legacy primary snapshot so
// metadata-less evaluations still degrade to the previous contract.
func boundTargetBindings(statement spec.Statement) []spec.TargetBinding {
	if statement.Metadata == nil {
		return nil
	}
	if len(statement.Metadata.TargetTables) > 0 {
		return statement.Metadata.TargetTables
	}
	if statement.Metadata.TargetTable != nil {
		return []spec.TargetBinding{{
			Table:    primaryTarget(statement),
			Snapshot: statement.Metadata.TargetTable,
		}}
	}
	// Metadata exists but carries no projection — the primary target is
	// unvouched, not proven: report a single nil-snapshot binding so the
	// gap path sees the same unknown-state contract it had before
	// multi-target bindings existed.
	if target := primaryTarget(statement); strings.TrimSpace(target.Name) != "" {
		return []spec.TargetBinding{{Table: target}}
	}
	return nil
}

func primaryTarget(statement spec.Statement) spec.Table {
	if statement.DDL == nil {
		return spec.Table{}
	}
	if targets := statement.DDL.TableTargets(); len(targets) > 0 {
		return targets[0]
	}
	if statement.DDL.Table != nil {
		return *statement.DDL.Table
	}
	return spec.Table{}
}

// renameEndpointRoles splits bound bindings into pair sources (even
// positions) and destinations (odd positions), matching the binding
// order boundTargetsForStatement publishes under RenameStatementForm.
// A statement whose target count cannot form pairs reports not-ok so no
// endpoint is misclassified.
func renameEndpointRoles(statement spec.Statement) (sources, destinations []spec.TargetBinding, ok bool) {
	bindings := boundTargetBindings(statement)
	if statement.DDL == nil || !statement.DDL.RenameStatementForm() || len(bindings)%2 != 0 {
		return nil, nil, false
	}
	for i, binding := range bindings {
		if i%2 == 0 {
			sources = append(sources, binding)
		} else {
			destinations = append(destinations, binding)
		}
	}
	return sources, destinations, len(bindings) > 0
}

// renameEndpointVerdict is one endpoint's sequential fate inside the
// atomic multi-pair RENAME: pairs evaluate left-to-right, each consumed
// source frees its slot and each produced destination occupies it.
type renameEndpointVerdict int

const (
	renameEndpointUndecidable renameEndpointVerdict = iota
	renameEndpointPresentNow                        // source exists at pair time
	renameEndpointAbsentNow                         // source missing at pair time — proven 1146
	renameEndpointFreeNow                           // destination free at pair time
	renameEndpointOccupiedNow                       // destination occupied at pair time — proven 1050
	renameEndpointSelfPair                          // ALTER self-pair — the destination IS the source; a no-op premise, never an occupancy
)

// renamePairVerdicts answers each pair's endpoint premises against the
// sequential working set — the same spec.RenamePairClasses adjudication
// applyRenameTable consumes. A pair after a proven-failing or undecidable
// pair is unreachable natively, so verdicts stop at the first pair that
// cannot migrate. An ALTER-side self-pair (RENAME TO the same identity)
// is a proven no-op: its destination premise is the source itself, not
// an occupancy — while a standalone RENAME TABLE self-pair keeps the
// ordinary 1050/1146 endpoint judgments.
func renamePairVerdicts(statement spec.Statement) (srcVerdicts, dstVerdicts []renameEndpointVerdict, ok bool) {
	sources, destinations, ok := renameEndpointRoles(statement)
	if !ok {
		return nil, nil, false
	}
	srcKeys := make([]string, len(sources))
	dstKeys := make([]string, len(sources))
	srcBase := make([]spec.RenameEndpointClass, len(sources))
	dstBase := make([]spec.RenameEndpointClass, len(sources))
	for i := range sources {
		srcKeys[i] = boundIdentityKey(statement, sources[i])
		dstKeys[i] = boundIdentityKey(statement, destinations[i])
		srcBase[i] = boundEndpointClass(sources[i])
		dstBase[i] = boundEndpointClass(destinations[i])
	}
	alterForm := statement.DDL.Operation == spec.DDLOperationAlterTable
	for i, verdict := range spec.RenamePairClasses(srcKeys, dstKeys, srcBase, dstBase) {
		srcV := renameSrcVerdict(verdict.Src)
		dstV := renameDstVerdict(verdict.Dst)
		if alterForm && srcKeys[i] == dstKeys[i] {
			dstV = renameEndpointSelfPair
		}
		srcVerdicts = append(srcVerdicts, srcV)
		dstVerdicts = append(dstVerdicts, dstV)
		if srcV == renameEndpointAbsentNow || dstV == renameEndpointOccupiedNow ||
			srcV == renameEndpointUndecidable || dstV == renameEndpointUndecidable {
			// The statement cannot migrate — pairs after this one are
			// unreachable, so no further verdicts are claimed.
			break
		}
	}
	return srcVerdicts, dstVerdicts, true
}

// boundEndpointClass folds a target binding into the shared
// rename-adjudication class: ambiguous and unvouched bindings are
// unknown, never a guessed absence.
func boundEndpointClass(binding spec.TargetBinding) spec.RenameEndpointClass {
	switch {
	case binding.Ambiguous || binding.Snapshot == nil:
		return spec.RenameEndpointUnknown
	case binding.Snapshot.Exists:
		return spec.RenameEndpointPresent
	default:
		return spec.RenameEndpointAbsent
	}
}

func renameSrcVerdict(class spec.RenameEndpointClass) renameEndpointVerdict {
	switch class {
	case spec.RenameEndpointPresent:
		return renameEndpointPresentNow
	case spec.RenameEndpointAbsent:
		return renameEndpointAbsentNow
	default:
		return renameEndpointUndecidable
	}
}

func renameDstVerdict(class spec.RenameEndpointClass) renameEndpointVerdict {
	switch class {
	case spec.RenameEndpointPresent:
		return renameEndpointOccupiedNow
	case spec.RenameEndpointAbsent:
		return renameEndpointFreeNow
	default:
		return renameEndpointUndecidable
	}
}

// boundIdentityKey normalizes a bound identity for cross-endpoint
// comparison — schema/table casing plus the namespace flag, so a temp
// object and a same-named catalog object never collide.
func boundIdentityKey(statement spec.Statement, binding spec.TargetBinding) string {
	schema := strings.ToLower(strings.TrimSpace(binding.Table.Schema))
	if schema == "" && statement.Metadata != nil {
		schema = strings.ToLower(strings.TrimSpace(statement.Metadata.Schema))
	}
	temporary := ""
	if binding.Snapshot != nil && binding.Snapshot.Temporary {
		temporary = "tmp"
	}
	return schema + "." + strings.ToLower(strings.TrimSpace(binding.Table.Name)) + ":" + temporary
}

// renameSourceExistsRequireRule fires when a RENAME endpoint pair's
// source is proven absent — the native 1146/1017 refusal the engine
// answers before any destination check.
type renameSourceExistsRequireRule struct {
	level rule.Level
}

func newRenameSourceExistsRequireRule(ruleID string, fallbackLevel rule.Level, cfg policy.RulePolicy) (rule.StatementRule, error) {
	return renameSourceExistsRequireRule{level: configuredLevel(cfg, fallbackLevel)}, nil
}

func (r renameSourceExistsRequireRule) ID() string { return ruleIDTableRenameSourceExistsRequire }

func (r renameSourceExistsRequireRule) AppliesTo(statement spec.Statement) bool {
	return statement.Kind == spec.KindDDL && statement.DDL != nil && statement.DDL.RenameStatementForm()
}

func (r renameSourceExistsRequireRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	sources, _, ok := renameEndpointRoles(statement)
	if !ok {
		return nil, nil
	}
	srcVerdicts, _, ok := renamePairVerdicts(statement)
	if !ok {
		return nil, nil
	}
	var findings []rule.Finding
	for i, verdict := range srcVerdicts {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if verdict != renameEndpointAbsentNow {
			continue
		}
		source := sources[i]
		findings = append(findings, rule.Finding{
			RuleID:     ruleIDTableRenameSourceExistsRequire,
			Level:      r.level,
			Message:    fmt.Sprintf("rename source %q does not exist in the bound namespace", source.Table.Name),
			Suggestion: "refresh metadata, correct the source identity, or drop the pair before renaming",
			Metadata: map[string]any{
				"table":  source.Table.Name,
				"schema": source.Table.Schema,
				"exists": false,
			},
		})
	}
	return findings, nil
}

func (r renameSourceExistsRequireRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !orderedGapDialect(statement.Dialect) || !r.AppliesTo(statement) {
		return nil
	}
	srcVerdicts, _, ok := renamePairVerdicts(statement)
	if !ok {
		return nil
	}
	var gaps []rule.EvidenceGap
	for _, verdict := range srcVerdicts {
		if verdict == renameEndpointUndecidable {
			gaps = append(gaps, rule.EvidenceGap{
				ReasonCode:    gapReasonUnknownTableState,
				RequiredFacts: []string{"source_table.existence"},
			})
		}
	}
	return gaps
}

// renameDestExistsForbidRule fires when a RENAME destination is occupied
// at its pair's sequential position — the native 1050 refusal — unless an
// earlier declared pair already vacated that identity (the scratch triple
// is the canonical legal form).
type renameDestExistsForbidRule struct {
	level rule.Level
}

func newRenameDestExistsForbidRule(ruleID string, fallbackLevel rule.Level, cfg policy.RulePolicy) (rule.StatementRule, error) {
	return renameDestExistsForbidRule{level: configuredLevel(cfg, fallbackLevel)}, nil
}

func (r renameDestExistsForbidRule) ID() string { return ruleIDTableRenameDestExistsForbid }

func (r renameDestExistsForbidRule) AppliesTo(statement spec.Statement) bool {
	return statement.Kind == spec.KindDDL && statement.DDL != nil && statement.DDL.RenameStatementForm()
}

func (r renameDestExistsForbidRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	_, destinations, ok := renameEndpointRoles(statement)
	if !ok {
		return nil, nil
	}
	_, dstVerdicts, ok := renamePairVerdicts(statement)
	if !ok {
		return nil, nil
	}
	var findings []rule.Finding
	for i, verdict := range dstVerdicts {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if verdict != renameEndpointOccupiedNow {
			continue
		}
		destination := destinations[i]
		findings = append(findings, rule.Finding{
			RuleID:     ruleIDTableRenameDestExistsForbid,
			Level:      r.level,
			Message:    fmt.Sprintf("rename destination %q already exists in the bound namespace", destination.Table.Name),
			Suggestion: "rename onto a free identity or move the occupying object first",
			Metadata: map[string]any{
				"table":  destination.Table.Name,
				"schema": destination.Table.Schema,
				"exists": true,
			},
		})
	}
	return findings, nil
}

func (r renameDestExistsForbidRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !orderedGapDialect(statement.Dialect) || !r.AppliesTo(statement) {
		return nil
	}
	_, dstVerdicts, ok := renamePairVerdicts(statement)
	if !ok {
		return nil
	}
	var gaps []rule.EvidenceGap
	for _, verdict := range dstVerdicts {
		if verdict == renameEndpointUndecidable {
			gaps = append(gaps, rule.EvidenceGap{
				ReasonCode:    gapReasonUnknownTableState,
				RequiredFacts: []string{"target_table.existence"},
			})
		}
	}
	return gaps
}

// likeSourceExistsRequireRule fires when a CREATE TABLE ... LIKE source
// is proven absent (native 1146); an unvouched source emits a bounded
// existence gap, and an existing source whose members cannot be vouched
// emits a structure gap instead of a fabricated shape claim.
type likeSourceExistsRequireRule struct {
	level rule.Level
}

func newLikeSourceExistsRequireRule(ruleID string, fallbackLevel rule.Level, cfg policy.RulePolicy) (rule.StatementRule, error) {
	return likeSourceExistsRequireRule{level: configuredLevel(cfg, fallbackLevel)}, nil
}

func (r likeSourceExistsRequireRule) ID() string { return ruleIDTableLikeSourceExistsRequire }

func (r likeSourceExistsRequireRule) AppliesTo(statement spec.Statement) bool {
	return appliesToCreateTable(statement) &&
		statement.DDL.ReferTable != nil && strings.TrimSpace(statement.DDL.ReferTable.Name) != ""
}

func (r likeSourceExistsRequireRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	source := likeSourceSnapshot(statement)
	if source == nil || source.Exists {
		return nil, nil
	}
	return []rule.Finding{{
		RuleID:     ruleIDTableLikeSourceExistsRequire,
		Level:      r.level,
		Message:    fmt.Sprintf("LIKE source %q does not exist in the bound namespace", statement.DDL.ReferTable.Name),
		Suggestion: "refresh metadata, correct the source identity, or create the source table first",
		Metadata: map[string]any{
			"table":  statement.DDL.ReferTable.Name,
			"schema": statement.DDL.ReferTable.Schema,
			"exists": false,
		},
	}}, nil
}

func (r likeSourceExistsRequireRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !orderedGapDialect(statement.Dialect) || !r.AppliesTo(statement) {
		return nil
	}
	source := likeSourceSnapshot(statement)
	if source == nil {
		return []rule.EvidenceGap{{
			ReasonCode:    gapReasonUnknownTableState,
			RequiredFacts: []string{"source_table.existence"},
		}}
	}
	if source.Exists && (source.PrimaryKeyUnknown || source.IndexesUnknown || source.ConstraintsUnknown) {
		return []rule.EvidenceGap{{
			ReasonCode:    gapReasonUnknownTableState,
			RequiredFacts: []string{"source_table.structure"},
		}}
	}
	return nil
}

func likeSourceSnapshot(statement spec.Statement) *spec.TableSnapshot {
	if statement.Metadata == nil {
		return nil
	}
	return statement.Metadata.SourceTable
}

// createAsSourceExistsRequireRule fires when a CTAS select source is
// proven absent (native 1146). TiDB's CREATE TABLE ... SELECT is a
// probe-proven vendor boundary, so the rule never applies there — the
// boundary finding owns the statement.
type createAsSourceExistsRequireRule struct {
	level rule.Level
}

func newCreateAsSourceExistsRequireRule(ruleID string, fallbackLevel rule.Level, cfg policy.RulePolicy) (rule.StatementRule, error) {
	return createAsSourceExistsRequireRule{level: configuredLevel(cfg, fallbackLevel)}, nil
}

func (r createAsSourceExistsRequireRule) ID() string { return ruleIDTableCreateAsSourceExistsRequire }

func (r createAsSourceExistsRequireRule) AppliesTo(statement spec.Statement) bool {
	return statement.Dialect != spec.DialectTiDB &&
		appliesToCreateTable(statement) &&
		statement.DDL.HasSelect && len(statement.DDL.SelectSources) > 0
}

func (r createAsSourceExistsRequireRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	var findings []rule.Finding
	for i, source := range selectSourceSnapshots(statement) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if source == nil || source.Exists {
			continue
		}
		findings = append(findings, rule.Finding{
			RuleID:     ruleIDTableCreateAsSourceExistsRequire,
			Level:      r.level,
			Message:    fmt.Sprintf("CTAS source %q does not exist in the bound namespace", statement.DDL.SelectSources[i].Name),
			Suggestion: "refresh metadata or create the source table first",
			Metadata: map[string]any{
				"table":  statement.DDL.SelectSources[i].Name,
				"schema": statement.DDL.SelectSources[i].Schema,
				"exists": false,
			},
		})
	}
	return findings, nil
}

func (r createAsSourceExistsRequireRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !orderedGapDialect(statement.Dialect) || !r.AppliesTo(statement) {
		return nil
	}
	var gaps []rule.EvidenceGap
	for _, source := range selectSourceSnapshots(statement) {
		if source == nil {
			gaps = append(gaps, rule.EvidenceGap{
				ReasonCode:    gapReasonUnknownTableState,
				RequiredFacts: []string{"source_table.existence"},
			})
		}
	}
	return gaps
}

func selectSourceSnapshots(statement spec.Statement) []*spec.TableSnapshot {
	if statement.Metadata == nil {
		return nil
	}
	return statement.Metadata.SourceTables
}

// truncateInboundFKRule fires when a TRUNCATE target carries a proven
// inbound foreign-key edge — the native 1701 refusal. Only batch-derived
// edges are provable; an unvouched inbound fact emits a bounded gap.
type truncateInboundFKRule struct {
	level rule.Level
}

func newTruncateInboundFKRule(ruleID string, fallbackLevel rule.Level, cfg policy.RulePolicy) (rule.StatementRule, error) {
	return truncateInboundFKRule{level: configuredLevel(cfg, fallbackLevel)}, nil
}

func (r truncateInboundFKRule) ID() string { return ruleIDTableTruncateInboundFK }

func (r truncateInboundFKRule) AppliesTo(statement spec.Statement) bool {
	return statement.Kind == spec.KindDDL && statement.DDL != nil &&
		statement.DDL.Operation == spec.DDLOperationTruncateTable
}

func (r truncateInboundFKRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	snapshot, ok := targetTableSnapshot(statement)
	if !ok || snapshot == nil || !snapshot.Exists || len(snapshot.ReferencedBy) == 0 {
		return nil, nil
	}
	return []rule.Finding{{
		RuleID:     ruleIDTableTruncateInboundFK,
		Level:      r.level,
		Message:    fmt.Sprintf("truncate target %q is referenced by inbound foreign keys", statement.DDL.Table.Name),
		Suggestion: "drop the referencing child tables or their foreign keys first",
		Metadata: map[string]any{
			"table":         statement.DDL.Table.Name,
			"referenced_by": len(snapshot.ReferencedBy),
		},
	}}, nil
}

func (r truncateInboundFKRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !orderedGapDialect(statement.Dialect) || !r.AppliesTo(statement) {
		return nil
	}
	snapshot, ok := targetTableSnapshot(statement)
	// The gap fills the hole that decides the outcome: a proven inbound
	// edge already answers the 1701 premise (the blocker speaks), so the
	// unvouched tail only gaps when no proven edge exists.
	if !ok || snapshot == nil || !snapshot.Exists || !snapshot.ReferencedByUnknown || len(snapshot.ReferencedBy) > 0 {
		return nil
	}
	return []rule.EvidenceGap{{
		ReasonCode:    gapReasonUnknownTableState,
		RequiredFacts: []string{"target_table.inbound_fk"},
	}}
}

// tidbLocalTemporaryRule covers TiDB's proven local-temporary refusals:
// ALTER TABLE (8200), RENAME/ALTER RENAME (8200), and CREATE LIKE from a
// local-temp source (8006). The endpoint's Temporary flag on the bound
// snapshot is the shared premise — a binding that cannot prove temp-ness
// never fires.
type tidbLocalTemporaryRule struct {
	ruleID string
	form   string
	label  string
	level  rule.Level
}

func newTiDBLocalTemporaryRule(ruleID, form, label string, fallbackLevel rule.Level, cfg policy.RulePolicy) (rule.StatementRule, error) {
	return tidbLocalTemporaryRule{
		ruleID: ruleID,
		form:   form,
		label:  label,
		level:  configuredLevel(cfg, fallbackLevel),
	}, nil
}

func (r tidbLocalTemporaryRule) ID() string { return r.ruleID }

func (r tidbLocalTemporaryRule) AppliesTo(statement spec.Statement) bool {
	if statement.Dialect != spec.DialectTiDB || statement.Kind != spec.KindDDL || statement.DDL == nil {
		return false
	}
	switch r.form {
	case "alter":
		return statement.DDL.Operation == spec.DDLOperationAlterTable && !statement.DDL.RenameStatementForm()
	case "rename":
		return statement.DDL.RenameStatementForm()
	case "like":
		return appliesToCreateTable(statement) &&
			statement.DDL.ReferTable != nil && strings.TrimSpace(statement.DDL.ReferTable.Name) != ""
	}
	return false
}

func (r tidbLocalTemporaryRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	var bound *spec.TableSnapshot
	switch r.form {
	case "alter":
		if snapshot, ok := targetTableSnapshot(statement); ok {
			bound = snapshot
		}
	case "rename":
		bindings := boundTargetBindings(statement)
		for i := range bindings {
			if bindings[i].Snapshot != nil && bindings[i].Snapshot.Temporary {
				bound = bindings[i].Snapshot
				break
			}
		}
	case "like":
		bound = likeSourceSnapshot(statement)
	}
	if bound == nil || !bound.Temporary {
		return nil, nil
	}
	return []rule.Finding{{
		RuleID:     r.ruleID,
		Level:      r.level,
		Message:    fmt.Sprintf("%s on a TiDB local temporary table is refused (8200)", r.label),
		Suggestion: "drop the local temporary table and recreate it, or target the persistent object",
		Metadata: map[string]any{
			"form": r.form,
		},
	}}, nil
}
