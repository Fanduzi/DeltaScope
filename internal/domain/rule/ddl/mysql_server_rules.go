// Package ddl: the audited MySQL identifier-form DROP SERVER slice —
// T26-SERVER-DROP-01 recognition and the T26-SERVER-DROP-EXECUTION-01
// shared adjudication (accepted contracts:
// docs/verification/t26-semantics-01 b8a67e7b,
// docs/verification/t26-server-drop-premise-01 cfd34782).
// The rule reads one truth: spec.AdjudicateServerDrop's arm plus its
// derived owed-fact set. server.existence, server.batch_effects, and
// server.drop_privilege are owed exactly per arm — the terminal
// refusal owes nothing (the proven absent carries the whole verdict),
// the guarded arm owes the privilege only when it stays unproven, and
// a consumable positive witness under the fixed
// observed_account_new_session conditional scope discharges
// server.drop_privilege without widening identity, clearing barriers,
// or proving absence. Findings and gaps carry no authored server name,
// no account, and no grant text — the evidence stays internal.

package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type mysqlDropServerRule struct {
	id    string
	level rule.Level
}

func newMySQLDropServerRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return mysqlDropServerRule{
		id:    ruleIDMySQLDropServerNotice,
		level: configuredLevel(cfg, rule.LevelNotice),
	}, nil
}

func (r mysqlDropServerRule) ID() string { return r.id }

func (r mysqlDropServerRule) AppliesTo(statement spec.Statement) bool {
	return statement.Dialect == spec.DialectMySQL &&
		statement.Kind == spec.KindDDL &&
		statement.DDL != nil &&
		statement.DDL.Server != nil &&
		statement.DDL.Operation == spec.DDLOperationDropServer &&
		statement.Unsupported == nil
}

// serverStateFor returns the adjudicated existence fact bound for this
// statement — nil means the identity is unbound (no provider, no
// resolver), which is unproven, never absent.
func serverStateFor(statement spec.Statement) *spec.ServerState {
	if statement.Metadata == nil {
		return nil
	}
	return statement.Metadata.Server
}

// serverDropAdjudicationFor returns the shared adjudication the
// enrichment attached — nil outside the audited slice (a legacy
// statement shape or a bypassed enrichment), where the defensive
// fallback keeps the pre-execution contract: every premise owed.
func serverDropAdjudicationFor(statement spec.Statement) *spec.ServerDropAdjudication {
	if statement.Metadata == nil {
		return nil
	}
	return statement.Metadata.ServerDrop
}

func (r mysqlDropServerRule) missingServerFacts(statement spec.Statement) []string {
	if adjudication := serverDropAdjudicationFor(statement); adjudication != nil {
		// The shared adjudication derives the owed set per arm — the
		// rule never recomputes premises on its own.
		return adjudication.Owed
	}
	missing := []string{}
	state := serverStateFor(statement)
	proven := state != nil && state.Read && state.Exists
	if !proven {
		if state != nil && state.UnreadReason == spec.MetadataUnreadBatchEffects {
			// The ordered channel's undischarged unknown effect shadowed
			// the catalog — the batch-effects gap replaces the existence
			// gap (same dedup contract as the tablespace family).
			missing = append(missing, spec.ServerFactBatchEffects)
		} else {
			missing = append(missing, spec.ServerFactExistence)
		}
	}
	missing = append(missing, spec.ServerFactDropPrivilege)
	return missing
}

func (r mysqlDropServerRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(statement) {
		return nil
	}
	missing := r.missingServerFacts(statement)
	if len(missing) == 0 {
		return nil
	}
	return []rule.EvidenceGap{{
		ReasonCode:    "unknown_server_state",
		RequiredFacts: missing,
	}}
}

// serverDropMessage renders the arm's bounded public message — the
// conditional-audit scope is the fixed token, never an account or
// grant detail.
func serverDropMessage(statement spec.Statement, adjudication *spec.ServerDropAdjudication) string {
	state := serverStateFor(statement)
	if adjudication == nil {
		if state != nil && state.Read && state.Exists {
			return "DROP SERVER targets a catalog entry proven present at bind time; execution remains unverified"
		}
		return "DROP SERVER targets a server definition whose existence and execution remain unverified"
	}
	consumable := adjudication.Privilege == spec.ServerDropPrivilegeConsumable
	switch adjudication.Arm {
	case spec.ServerDropArmProven:
		return "DROP SERVER is conditionally proven to drop its catalog entry under the observed_account_new_session conditional audit"
	case spec.ServerDropArmConvergent:
		return "DROP SERVER IF EXISTS is conditionally proven to leave the server entry absent under the observed_account_new_session conditional audit"
	case spec.ServerDropArmTerminalRefusal:
		return "DROP SERVER is refused by the ordered state in force — the audited entry's absent state stands proven in this batch"
	case spec.ServerDropArmGuardedAbsent:
		if consumable {
			return "DROP SERVER IF EXISTS keeps the audited entry's proven absent state under the observed_account_new_session conditional audit"
		}
		return "DROP SERVER IF EXISTS keeps the audited entry's proven absent state in this batch; this statement's execution premise remains unverified"
	case spec.ServerDropArmBarrier:
		if consumable {
			return "DROP SERVER leaves existence shadowed by earlier batch effects under the observed_account_new_session conditional audit"
		}
		return "DROP SERVER targets a server definition whose existence and execution remain unverified"
	default:
		if consumable {
			return "DROP SERVER targets a server definition whose existence remains unverified under the observed_account_new_session conditional audit"
		}
		if state != nil && state.Read && state.Exists {
			return "DROP SERVER targets a catalog entry proven present at bind time; execution remains unverified"
		}
		return "DROP SERVER targets a server definition whose existence and execution remain unverified"
	}
}

// serverDropOutcome is the bounded outcome token published in finding
// metadata — one fixed vocabulary per arm so public consumers read the
// arm without parsing prose.
func serverDropOutcome(adjudication *spec.ServerDropAdjudication) string {
	if adjudication == nil {
		return "unverified"
	}
	switch adjudication.Arm {
	case spec.ServerDropArmProven:
		return "conditional_drop"
	case spec.ServerDropArmConvergent:
		return "conditional_absent"
	case spec.ServerDropArmTerminalRefusal:
		return "refused_absent"
	case spec.ServerDropArmGuardedAbsent:
		return "guarded_absent"
	case spec.ServerDropArmBarrier:
		return "blocked_batch_effects"
	default:
		return "unverified"
	}
}

func (r mysqlDropServerRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	adjudication := serverDropAdjudicationFor(statement)
	consumable := adjudication != nil && adjudication.Privilege == spec.ServerDropPrivilegeConsumable

	level := r.level
	explanation := &rule.FindingExplanation{
		Why:        "DROP SERVER removes the mysql.servers row defining a foreign data wrapper connection and requires SUPER.",
		Risk:       "Whether the statement actually executed is unproven — the privilege premise is not discharged, and batch writes may already shadow the catalog answer. FEDERATED tables created with the connection information are not removed with it, but definitions or code resolving through this server entry could be affected if the removal lands.",
		Suggestion: "Confirm the server entry is no longer needed and that the executing account holds SUPER before relying on the drop.",
	}
	if adjudication != nil {
		switch adjudication.Arm {
		case spec.ServerDropArmTerminalRefusal:
			// A proven refusal is the same finding class the sibling
			// families emit — a refusal blocker, not an advisory
			// notice. The absent proof is the batch's ordered state;
			// no specific native response is claimed beyond the
			// refusal itself.
			level = rule.LevelBlocker
			explanation = &rule.FindingExplanation{
				Why:        "DROP SERVER requires the named mysql.servers entry to exist; the audited entry's absent state stands proven in this batch's ordered model, so the statement is refused.",
				Risk:       "The statement cannot land under the ordered state in force — the absent proof is the batch's own ordered state, never a catalog guess; the specific native response is not asserted beyond the refusal.",
				Suggestion: "Remove the duplicate drop or guard it with IF EXISTS if absence is acceptable.",
			}
		case spec.ServerDropArmGuardedAbsent:
			explanation = &rule.FindingExplanation{
				Why:        "DROP SERVER IF EXISTS guards the native not-found error; the audited entry's absent state stands proven in this batch's ordered model and is preserved.",
				Risk:       "The absent state is a proven model projection, not an observed deletion. " + conditionalRiskTail(consumable),
				Suggestion: "Confirm the absence is the expected state; the guard hides the not-found error by design.",
			}
		case spec.ServerDropArmProven, spec.ServerDropArmConvergent:
			explanation = &rule.FindingExplanation{
				Why:        "DROP SERVER removes the mysql.servers row defining a foreign data wrapper connection and requires SUPER; the conditional audit bound the observed account's direct global grant as the expected executor's premise.",
				Risk:       conditionalRiskTail(true) + " FEDERATED tables created with the connection information are not removed with it.",
				Suggestion: "Confirm the server entry is no longer needed; the conditional audit only covers a new session on the observed target while the relied-upon grant still applies.",
			}
		default:
			if consumable {
				explanation = &rule.FindingExplanation{
					Why:        "DROP SERVER removes the mysql.servers row defining a foreign data wrapper connection and requires SUPER; the conditional audit discharged the privilege premise but existence stayed unproven.",
					Risk:       conditionalRiskTail(true) + " The entry's pre-state is still unverified, so whether anything was dropped is unproven.",
					Suggestion: "Confirm the server entry exists and is no longer needed before relying on the drop.",
				}
			}
		}
	}

	metadata := map[string]any{
		"operation": string(statement.DDL.Operation),
		"object":    "server",
		"outcome":   serverDropOutcome(adjudication),
	}
	if statement.DDL.Server.IfExists {
		metadata["if_exists"] = true
	}
	if consumable || (adjudication != nil && adjudication.ConditionalSource) {
		// The conditional scope follows either this statement's own
		// consumable premise or an OrderedAbsent a prior statement
		// published under that scope — a refusal/guard resting on a
		// conditional absent stays conditional (R1-D).
		metadata["conditional_audit"] = string(spec.PrivilegeAuditModeObservedAccountNewSession)
	}
	return []rule.Finding{{
		Level:       level,
		Message:     serverDropMessage(statement, adjudication),
		Explanation: explanation,
		Metadata:    metadata,
	}}, nil
}

// conditionalRiskTail is the fixed conditional-audit caveat — the
// premise holds only under the declared scope, never as a live grant
// proof.
func conditionalRiskTail(consumable bool) string {
	if !consumable {
		return "The execution premise stays unproven — no consumable grant evidence satisfied the observed_account_new_session scope for this statement."
	}
	return "The audit is conditional — it describes a new execution session on the observed target established after the grant observation, while the relied-upon direct global grant still applies; it does not verify the future executor or that any current session holds SUPER."
}
