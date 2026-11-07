package ddl

import (
	"context"
	"fmt"
	"sort"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// sequence_rules.go implements the T21/#100 TiDB sequence lifecycle rules.
// They adjudicate only what the ordered state proves: existence, definition
// parameters, declared restart/cycle intent, and proven inbound references.
// Runtime facts — issued values, cache consumption, persisted position — are
// never modeled; where an enabled rule's premise needs them the rule emits a
// bounded EvidenceGap instead of implying an answer.

const (
	sequenceGapUnknownState      = "unknown_sequence_state"
	sequenceGapUnknownRuntime    = "unknown_sequence_runtime_state"
	sequenceGapUnknownReferences = "unknown_sequence_references"
	sequenceFactState            = "sequence.state"
	sequenceFactRuntime          = "sequence.runtime_state"
	sequenceFactReferences       = "sequence.references"
)

// sequenceBinding is one resolved sequence target as the statement's ordered
// pre-state projects it.
type sequenceBinding struct {
	key   string
	state *spec.SequenceState
}

// sequenceBindings returns the statement's bound sequence targets in
// deterministic key order — every identity carries the effective-schema key
// the ordered state projected under.
func sequenceBindings(statement spec.Statement) []sequenceBinding {
	if statement.DDL == nil || statement.Metadata == nil || len(statement.Metadata.Sequences) == 0 {
		return nil
	}
	keys := make([]string, 0, len(statement.Metadata.Sequences))
	for key := range statement.Metadata.Sequences {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	bindings := make([]sequenceBinding, 0, len(keys))
	for _, key := range keys {
		bindings = append(bindings, sequenceBinding{key: key, state: statement.Metadata.Sequences[key]})
	}
	return bindings
}

func sequenceApplies(statement spec.Statement) bool {
	return statement.Dialect == spec.DialectTiDB &&
		statement.Kind == spec.KindDDL &&
		statement.DDL != nil &&
		statement.DDL.Sequence != nil
}

// sequenceRenameApplies marks a RENAME TABLE pair that binds at least one
// sequence identity — the shared-namespace path TiDB uses for sequence
// rename (T21 native-verified: identity and definition move with the pair).
func sequenceRenameApplies(statement spec.Statement) bool {
	return statement.Dialect == spec.DialectTiDB &&
		statement.Kind == spec.KindDDL &&
		statement.DDL != nil &&
		statement.DDL.Operation == spec.DDLOperationRenameTable &&
		len(sequenceBindings(statement)) > 0
}

func sequenceFinding(level rule.Level, message, suggestion string, binding sequenceBinding, operation string) rule.Finding {
	metadata := map[string]any{
		"schema":    binding.state.Schema,
		"sequence":  binding.state.Name,
		"operation": operation,
	}
	return rule.Finding{Level: level, Message: message, Suggestion: suggestion, Metadata: metadata}
}

// sequenceValidRule adjudicates existence contracts on sequence lifecycle
// statements: proven duplicates refuse CREATE, proven-absent targets refuse
// ALTER/DROP, and shared-namespace conflicts refuse sequence operations.
// Unproven identities emit bounded gaps rather than fabricated answers. It
// also owns the issuance-state premise for non-RESTART definition changes.
type sequenceValidRule struct {
	level rule.Level
}

func newTiDBSequenceValidRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return sequenceValidRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r sequenceValidRule) ID() string { return ruleIDTiDBSequenceValid }

func (r sequenceValidRule) AppliesTo(statement spec.Statement) bool {
	return sequenceApplies(statement) || sequenceRenameApplies(statement)
}

func (r sequenceValidRule) Evaluate(_ context.Context, statement spec.Statement) ([]rule.Finding, error) {
	declaration := statement.DDL.Sequence
	var findings []rule.Finding
	for _, binding := range sequenceBindings(statement) {
		state := binding.state
		if state == nil || !state.Proven() {
			continue
		}
		switch statement.DDL.Operation {
		case spec.DDLOperationCreateSequence:
			if state.ConflictKind != "" && !declaration.IfNotExists {
				findings = append(findings, sequenceFinding(r.level,
					fmt.Sprintf("Sequence %q conflicts with an existing %s object.", qualifiedName(state), state.ConflictKind),
					"choose a different sequence name or drop the conflicting object first",
					binding, "create"))
				continue
			}
			if state.Exists && !declaration.IfNotExists {
				findings = append(findings, sequenceFinding(r.level,
					fmt.Sprintf("Sequence %q already exists.", qualifiedName(state)),
					"use CREATE SEQUENCE IF NOT EXISTS or drop the existing sequence first",
					binding, "create"))
			}
		case spec.DDLOperationAlterSequence:
			// TiDB 8.5 accepts ALTER SEQUENCE ... IF EXISTS syntactically but
			// the guard never rescues an absent or wrong-kind target — the
			// native verdict stays 1146/1347. Proven-absent bindings refuse
			// regardless of the authored guard.
			if state.ConflictKind != "" {
				findings = append(findings, sequenceFinding(r.level,
					fmt.Sprintf("Sequence %q does not exist; the identity is bound to a %s object.", qualifiedName(state), state.ConflictKind),
					"verify the target name and object kind before altering",
					binding, "alter"))
				continue
			}
			if !state.Exists {
				findings = append(findings, sequenceFinding(r.level,
					fmt.Sprintf("Sequence %q does not exist.", qualifiedName(state)),
					"create the sequence first — IF EXISTS does not tolerate an absent ALTER target",
					binding, "alter"))
			}
		case spec.DDLOperationDropSequence:
			if state.ConflictKind != "" && !declaration.IfExists {
				findings = append(findings, sequenceFinding(r.level,
					fmt.Sprintf("Sequence %q does not exist; the identity is bound to a %s object.", qualifiedName(state), state.ConflictKind),
					"verify the target name and object kind before dropping",
					binding, "drop"))
				continue
			}
			if !state.Exists && !declaration.IfExists {
				findings = append(findings, sequenceFinding(r.level,
					fmt.Sprintf("Sequence %q does not exist.", qualifiedName(state)),
					"verify the sequence exists or use DROP SEQUENCE IF EXISTS",
					binding, "drop"))
			}
		}
	}
	return findings, nil
}

// EvidenceGaps reports the bounded missing-fact contracts this rule owns:
// unproven target existence, plus the issuance-state premise an in-place
// definition change on a live sequence requires.
func (r sequenceValidRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	declaration := statement.DDL.Sequence
	var gaps []rule.EvidenceGap
	for _, binding := range sequenceBindings(statement) {
		state := binding.state
		if state == nil {
			continue
		}
		if !state.Proven() {
			gaps = append(gaps, rule.EvidenceGap{
				ReasonCode:    sequenceGapUnknownState,
				RequiredFacts: []string{sequenceFactState},
			})
			continue
		}
		// An in-place definition change on a proven-live sequence revokes
		// issuance assumptions (cache reset, bound changes) — the runtime
		// premise is required and unprovable without issuance facts. A
		// proven-live sequence moved by RENAME TABLE carries the same
		// premise: the persisted issuance position cannot be re-derived.
		if state.Exists && (statement.DDL.Operation == spec.DDLOperationRenameTable ||
			(statement.DDL.Operation == spec.DDLOperationAlterSequence && declaration != nil && declaration.ChangesDefinition())) {
			gaps = append(gaps, rule.EvidenceGap{
				ReasonCode:    sequenceGapUnknownRuntime,
				RequiredFacts: []string{sequenceFactRuntime},
			})
		}
	}
	return gaps
}

// sequenceRestartRule reports the reuse hazard of a declared RESTART on a
// proven-live sequence. The warning claims only what the declaration says —
// never a predicted next value — and the runtime gap marks the issuance fact
// the reuse assessment cannot prove offline.
type sequenceRestartRule struct {
	level rule.Level
}

func newTiDBSequenceRestartWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return sequenceRestartRule{level: configuredLevel(cfg, rule.LevelWarning)}, nil
}

func (r sequenceRestartRule) ID() string { return ruleIDTiDBSequenceRestartWarn }

func (r sequenceRestartRule) AppliesTo(statement spec.Statement) bool {
	return sequenceApplies(statement) &&
		statement.DDL.Operation == spec.DDLOperationAlterSequence &&
		statement.DDL.Sequence.RestartRequested()
}

func (r sequenceRestartRule) Evaluate(_ context.Context, statement spec.Statement) ([]rule.Finding, error) {
	var findings []rule.Finding
	for _, binding := range sequenceBindings(statement) {
		state := binding.state
		if state == nil || !state.Proven() || !state.Exists {
			continue
		}
		findings = append(findings, sequenceFinding(r.level,
			"Restarting a sequence can reuse values previously issued.",
			"verify no application depends on the previously issued range before restarting",
			binding, "restart"))
	}
	return findings, nil
}

// EvidenceGaps marks the runtime-issuance fact a live restart requires —
// without it the reuse assessment stays bounded and the statement unverified.
func (r sequenceRestartRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	var gaps []rule.EvidenceGap
	for _, binding := range sequenceBindings(statement) {
		state := binding.state
		if state == nil || !state.Proven() || !state.Exists {
			continue
		}
		gaps = append(gaps, rule.EvidenceGap{
			ReasonCode:    sequenceGapUnknownRuntime,
			RequiredFacts: []string{sequenceFactRuntime},
		})
	}
	return gaps
}

// sequenceCycleRule reports a declared-or-proven CYCLE enablement — wraparound
// reissues values, so the warning names only the proven enablement, never a
// predicted collision.
type sequenceCycleRule struct {
	level rule.Level
}

func newTiDBSequenceCycleWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return sequenceCycleRule{level: configuredLevel(cfg, rule.LevelWarning)}, nil
}

func (r sequenceCycleRule) ID() string { return ruleIDTiDBSequenceCycleWarn }

func (r sequenceCycleRule) AppliesTo(statement spec.Statement) bool {
	return sequenceApplies(statement) &&
		(statement.DDL.Operation == spec.DDLOperationCreateSequence ||
			statement.DDL.Operation == spec.DDLOperationAlterSequence)
}

func (r sequenceCycleRule) Evaluate(_ context.Context, statement spec.Statement) ([]rule.Finding, error) {
	declaration := statement.DDL.Sequence
	var findings []rule.Finding
	for _, binding := range sequenceBindings(statement) {
		state := binding.state
		if state == nil {
			continue
		}
		cycle := declaration.DeclaredCycle()
		if cycle == nil && statement.DDL.Operation == spec.DDLOperationAlterSequence &&
			state.Proven() && state.Exists && state.Definition != nil {
			// ALTER without a cycle member keeps the proven catalog flag.
			cycle = state.Definition.Cycle
		}
		if cycle == nil || !*cycle {
			continue
		}
		findings = append(findings, sequenceFinding(r.level,
			"Declaring CYCLE lets a sequence reuse previously issued values after wraparound.",
			"verify consumers tolerate value reuse before enabling cycle",
			binding, "cycle"))
	}
	return findings, nil
}

// sequenceReferencesRule reports proven inbound references on a drop —
// one warning per distinct owner-slot/target edge, bounded to the
// structured identities the catalog or the batch proved. Completeness is
// adjudicated separately: an unvouched or diverged reference set adds a
// sequence.references gap instead of silence, while a proven-empty set
// emits neither. It never claims a native refusal and never fabricates a
// complete reference set (T21-R1 B2).
type sequenceReferencesRule struct {
	level rule.Level
}

func newTiDBSequenceReferencesWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return sequenceReferencesRule{level: configuredLevel(cfg, rule.LevelWarning)}, nil
}

func (r sequenceReferencesRule) ID() string { return ruleIDTiDBSequenceReferencesWarn }

func (r sequenceReferencesRule) AppliesTo(statement spec.Statement) bool {
	return (sequenceApplies(statement) &&
		statement.DDL.Operation == spec.DDLOperationDropSequence) ||
		sequenceRenameApplies(statement)
}

func (r sequenceReferencesRule) Evaluate(_ context.Context, statement spec.Statement) ([]rule.Finding, error) {
	var findings []rule.Finding
	rename := statement.DDL.Operation == spec.DDLOperationRenameTable
	for _, binding := range sequenceBindings(statement) {
		state := binding.state
		if state == nil || !state.Proven() || !state.Exists {
			continue
		}
		for _, edge := range state.References {
			var finding rule.Finding
			if rename {
				// The stored DEFAULT keeps the pre-rename identity — the
				// bound expression dangles once the source name moves.
				finding = sequenceFinding(r.level,
					fmt.Sprintf("Sequence %q is still referenced by %s; renaming it leaves the dependent broken.", qualifiedName(state), sequenceRefEdgeLabel(edge)),
					"remove or rebind the referencing column default before renaming the sequence",
					binding, "rename")
			} else {
				finding = sequenceFinding(r.level,
					fmt.Sprintf("Sequence %q is still referenced by %s; dropping it leaves the dependent broken.", qualifiedName(state), sequenceRefEdgeLabel(edge)),
					"remove the referencing column default before dropping the sequence",
					binding, "drop")
			}
			finding.Metadata["owner_kind"] = edge.OwnerKind
			finding.Metadata["owner_schema"] = edge.OwnerSchema
			finding.Metadata["owner_name"] = edge.OwnerName
			finding.Metadata["slot"] = edge.Slot
			findings = append(findings, finding)
		}
	}
	return findings, nil
}

// EvidenceGaps marks the inbound-completeness premise a proven-present
// drop still owes when the reference set is unvouched or diverged — the
// proven positives already warned stand, but the set cannot be claimed
// exhaustive. Proven-absent targets and guard no-ops make the reference
// premise inapplicable; an unproven target is already covered by the
// valid rule's unknown_sequence_state gap.
func (r sequenceReferencesRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	var gaps []rule.EvidenceGap
	for _, binding := range sequenceBindings(statement) {
		state := binding.state
		if state == nil || !state.Proven() || !state.Exists || !state.ReferencesUnknown {
			continue
		}
		gaps = append(gaps, rule.EvidenceGap{
			ReasonCode:    sequenceGapUnknownReferences,
			RequiredFacts: []string{sequenceFactReferences},
		})
	}
	return gaps
}

// sequenceRefEdgeLabel renders one edge's bounded owner identity — kind,
// qualified name, and slot — never raw expression text.
func sequenceRefEdgeLabel(edge spec.SequenceRefEdge) string {
	owner := edge.OwnerName
	if edge.OwnerSchema != "" {
		owner = edge.OwnerSchema + "." + edge.OwnerName
	}
	if edge.Slot == "" {
		return fmt.Sprintf("%s %q", edge.OwnerKind, owner)
	}
	return fmt.Sprintf("%s %q (slot %s)", edge.OwnerKind, owner, edge.Slot)
}

// sequenceNoticeAdjudicator binds the three legacy sequence notices to the
// shared ordered-state judgment (T21-R1 B1): a proven refusal or a proven
// guard no-op suppresses the notice — neither may claim the object was
// created, changed, or removed. An applicable or unproven target keeps the
// declarative intent wording, and multi-target statements adjudicate each
// binding independently so one refused member neither erases nor claims
// another member's outcome. Statements with no sequence bindings — every
// dialect outside the ordered TiDB path — return unhandled and keep the
// generic single-finding surface verbatim.
func sequenceNoticeAdjudicator(operation, message, suggestion string) func(rule.Level, spec.Statement) ([]rule.Finding, bool) {
	return func(level rule.Level, statement spec.Statement) ([]rule.Finding, bool) {
		bindings := sequenceBindings(statement)
		if len(bindings) == 0 {
			return nil, false
		}
		var findings []rule.Finding
		for _, binding := range bindings {
			if sequenceNoticeSuppressed(statement, binding.state) {
				continue
			}
			name := binding.key
			if binding.state != nil && binding.state.Name != "" {
				name = qualifiedName(binding.state)
			}
			findings = append(findings, rule.Finding{
				Level:      level,
				Message:    fmt.Sprintf(message, name),
				Suggestion: suggestion,
				Metadata: map[string]any{
					"operation":   operation,
					"object_type": "sequence",
					"object_name": name,
					"sequence":    name,
				},
			})
		}
		return findings, true
	}
}

// sequenceNoticeSuppressed is the shared refusal/no-op truth table the
// notices consume — the same ordered SequenceState the valid rule reads,
// never a re-derivation of the catalog. A proven-absent create target or a
// proven-present alter/drop target is the only shape where the declared
// intent can hold; every other proven state is a refusal or a guard
// no-op, and unproven states keep the declarative notice.
func sequenceNoticeSuppressed(statement spec.Statement, state *spec.SequenceState) bool {
	if state == nil || !state.Proven() {
		return false
	}
	switch statement.DDL.Operation {
	case spec.DDLOperationCreateSequence:
		return state.ConflictKind != "" || state.Exists
	case spec.DDLOperationAlterSequence, spec.DDLOperationDropSequence:
		return state.ConflictKind != "" || !state.Exists
	}
	return false
}

// qualifiedName renders the bounded display identity of a bound sequence.
func qualifiedName(state *spec.SequenceState) string {
	if state.Schema == "" {
		return state.Name
	}
	return state.Schema + "." + state.Name
}
