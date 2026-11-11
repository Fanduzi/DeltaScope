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
	"strings"

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

// The audited identifier-form CREATE SERVER first slice
// (T26-SERVER-CREATE-EXECUTION-01; accepted contract
// docs/verification/t26-server-create-contract-01, R4 PASS 112beb08).
// The rule reads the same one truth as the DROP sibling —
// spec.AdjudicateServerCreate's arm plus its derived owed-fact set:
// server.existence, server.batch_effects, server.create_privilege, and
// server.create_options are owed exactly per arm. The terminal refusal
// owes nothing (proven presence carries the whole verdict), the
// publish arm owes nothing, and the barrier arm owes exactly
// {server.batch_effects}. Findings and gaps carry no authored server
// name, no wrapper, no OPTIONS content, no account, and no grant text.

type mysqlCreateServerRule struct {
	id    string
	level rule.Level
}

func newMySQLCreateServerRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return mysqlCreateServerRule{
		id:    ruleIDMySQLCreateServerNotice,
		level: configuredLevel(cfg, rule.LevelNotice),
	}, nil
}

func (r mysqlCreateServerRule) ID() string { return r.id }

func (r mysqlCreateServerRule) AppliesTo(statement spec.Statement) bool {
	return statement.Dialect == spec.DialectMySQL &&
		statement.Kind == spec.KindDDL &&
		statement.DDL != nil &&
		statement.DDL.Server != nil &&
		statement.DDL.Operation == spec.DDLOperationCreateServer &&
		statement.Unsupported == nil
}

// serverCreateAdjudicationFor returns the shared adjudication the
// enrichment attached — nil outside the audited slice (a legacy
// statement shape or a bypassed enrichment), where the defensive
// fallback keeps the pre-execution contract: every premise owed.
func serverCreateAdjudicationFor(statement spec.Statement) *spec.ServerCreateAdjudication {
	if statement.Metadata == nil {
		return nil
	}
	return statement.Metadata.ServerCreate
}

func (r mysqlCreateServerRule) missingServerFacts(statement spec.Statement) []string {
	if adjudication := serverCreateAdjudicationFor(statement); adjudication != nil {
		// The shared adjudication derives the owed set per arm — the
		// rule never recomputes premises on its own.
		return adjudication.Owed
	}
	missing := []string{}
	state := serverStateFor(statement)
	if state != nil && state.UnreadReason == spec.MetadataUnreadBatchEffects {
		missing = append(missing, spec.ServerFactBatchEffects)
	} else {
		missing = append(missing, spec.ServerFactExistence)
	}
	missing = append(missing, spec.ServerFactCreatePrivilege, spec.ServerFactCreateOptions)
	return missing
}

func (r mysqlCreateServerRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
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

// serverCreateMessage renders the arm's bounded public message — the
// conditional-audit scope is the fixed token, never an account, grant
// detail, server name, wrapper, or OPTIONS content.
func serverCreateMessage(statement spec.Statement, adjudication *spec.ServerCreateAdjudication) string {
	state := serverStateFor(statement)
	if adjudication == nil {
		return "CREATE SERVER targets a server definition whose existence, execution premise, and options remain unverified"
	}
	consumable := adjudication.Privilege == spec.ServerCreatePrivilegeConsumable
	switch adjudication.Arm {
	case spec.ServerCreateArmProven:
		return "CREATE SERVER is conditionally proven to create its catalog entry under the observed_account_new_session conditional audit"
	case spec.ServerCreateArmTerminalRefusal:
		if state != nil && state.OrderedPresent {
			return "CREATE SERVER is refused by the ordered state in force — the audited entry's present state stands proven in this batch"
		}
		return "CREATE SERVER is refused — the catalog read proved the server entry present at bind time"
	case spec.ServerCreateArmBarrier:
		return "CREATE SERVER leaves its premises shadowed by earlier batch effects; this statement's execution premise remains unverified"
	default:
		items := serverCreateOwedItems(adjudication)
		if len(items) == 0 {
			return "CREATE SERVER targets a server definition whose existence remains unverified"
		}
		verb := "remain"
		if len(items) == 1 {
			verb = "remains"
		}
		phrase := serverCreateOwedPhrase(items)
		if consumable {
			return "CREATE SERVER targets a server definition whose " + phrase + " " + verb + " unverified under the observed_account_new_session conditional audit"
		}
		return "CREATE SERVER targets a server definition whose " + phrase + " " + verb + " unverified"
	}
}

// serverCreateOwedItems projects the adjudication's owed-fact set into
// the fixed public vocabulary — the arm's own bounded tokens, never an
// object name or option content. The order follows the owed set's
// canonical existence → privilege → options order.
func serverCreateOwedItems(adjudication *spec.ServerCreateAdjudication) []string {
	var items []string
	for _, fact := range adjudication.Owed {
		switch fact {
		case spec.ServerFactExistence:
			items = append(items, "existence")
		case spec.ServerFactCreatePrivilege:
			items = append(items, "execution premise")
		case spec.ServerFactCreateOptions:
			items = append(items, "options")
		}
	}
	return items
}

// serverCreateOwedPhrase joins owed items into one bounded noun phrase
// — two items join with "or", three with ", ... or".
func serverCreateOwedPhrase(items []string) string {
	switch len(items) {
	case 1:
		return items[0]
	case 2:
		return items[0] + " or " + items[1]
	default:
		return strings.Join(items[:len(items)-1], ", ") + ", or " + items[len(items)-1]
	}
}

// serverCreateOutcome is the bounded outcome token published in
// finding metadata — one fixed vocabulary per arm so public consumers
// read the arm without parsing prose.
func serverCreateOutcome(adjudication *spec.ServerCreateAdjudication) string {
	if adjudication == nil {
		return "unverified"
	}
	switch adjudication.Arm {
	case spec.ServerCreateArmProven:
		return "conditional_create"
	case spec.ServerCreateArmTerminalRefusal:
		return "refused_present"
	case spec.ServerCreateArmBarrier:
		return "blocked_batch_effects"
	default:
		return "unverified"
	}
}

func (r mysqlCreateServerRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	adjudication := serverCreateAdjudicationFor(statement)
	consumable := adjudication != nil && adjudication.Privilege == spec.ServerCreatePrivilegeConsumable

	level := r.level
	explanation := &rule.FindingExplanation{
		Why:        "CREATE SERVER writes a row into mysql.servers defining a foreign data wrapper connection and requires SUPER.",
		Risk:       "Whether the statement actually executed is unproven — the privilege premise is not discharged, and batch writes may already shadow the catalog answer. Remote connectivity, FEDERATED availability, and the stored connection members are not verified by this audit.",
		Suggestion: "Confirm the server entry is needed, the options are complete, and the executing account holds SUPER before relying on the create.",
	}
	if adjudication != nil {
		switch adjudication.Arm {
		case spec.ServerCreateArmTerminalRefusal:
			// A proven refusal is the same finding class the sibling
			// families emit — a refusal blocker, not an advisory
			// notice. Occupancy alone carries the verdict; no specific
			// native response is claimed beyond the refusal itself.
			level = rule.LevelBlocker
			explanation = &rule.FindingExplanation{
				Why:        "CREATE SERVER requires the named mysql.servers entry to be absent; the entry's present state stands proven, so the statement is refused.",
				Risk:       "The statement cannot land — the presence proof is the audit's own bound evidence (an ordered channel answer or the catalog read), never a guess; the specific native response is not asserted beyond the refusal.",
				Suggestion: "Remove the duplicate create or choose a different server name if the existing entry should be kept.",
			}
		case spec.ServerCreateArmProven:
			explanation = &rule.FindingExplanation{
				Why:        "CREATE SERVER writes a row into mysql.servers and requires SUPER; the conditional audit bound the observed account's direct global grant as the expected executor's premise, and the ordered state proved the slot free.",
				Risk:       conditionalRiskTail(true) + " Remote connectivity and FEDERATED availability are not verified by this audit.",
				Suggestion: "Confirm the server entry is needed; the conditional audit only covers a new session on the observed target while the relied-upon grant still applies.",
			}
		case spec.ServerCreateArmBarrier:
			explanation = &rule.FindingExplanation{
				Why:        "CREATE SERVER writes a row into mysql.servers and requires SUPER; an earlier statement's undischarged effect on the server channel shadows this statement's premises.",
				Risk:       "The batch's own effects left this statement's premises undecidable; the shadowed premises never reached evaluation, so whether the create could land cannot be proven in this batch.",
				Suggestion: "Order the CREATE before unknown-effect writes, or split the batch so each statement's premises can be proven.",
			}
		default:
			if consumable {
				phrase := serverCreateOwedPhrase(serverCreateOwedItems(adjudication))
				explanation = &rule.FindingExplanation{
					Why:        "CREATE SERVER writes a row into mysql.servers and requires SUPER; the conditional audit discharged the privilege premise but " + phrase + " stayed unproven.",
					Risk:       conditionalRiskTail(true) + " Whether the create lands is unproven — the stated premises stayed unverified.",
					Suggestion: "Confirm the server entry does not already exist and that the options are complete before relying on the create.",
				}
			}
		}
	}

	metadata := map[string]any{
		"operation": string(statement.DDL.Operation),
		"object":    "server",
		"outcome":   serverCreateOutcome(adjudication),
	}
	if adjudication != nil && adjudication.ConditionalAudit {
		// The marker is the adjudication's own provenance fact — the
		// conditional scope actually fed the conclusion, either through
		// this statement's consumable premise on an arm that read it
		// or through an ordered source a prior statement published
		// under that scope. A barrier never carries it, and a refusal
		// carries it only when the consumed ordered state was
		// conditional — the rule never re-derives it from witness
		// availability.
		metadata["conditional_audit"] = string(spec.PrivilegeAuditModeObservedAccountNewSession)
	}
	findings := []rule.Finding{{
		Level:       level,
		Message:     serverCreateMessage(statement, adjudication),
		Explanation: explanation,
		Metadata:    metadata,
	}}

	// Audit-model completeness notice: omitted documented connection
	// members are an advisory of this audit slice — MySQL stores
	// blank/zero defaults, so it is never a refusal, never an owed
	// fact, and never a claim the engine emitted a warning. The fixed
	// text names no option keys and no authored content.
	if statement.DDL.Server != nil && !statement.DDL.Server.ConnectionMembersComplete() {
		findings = append(findings, rule.Finding{
			Level:   rule.LevelNotice,
			Message: "CREATE SERVER omits one or more documented connection members; connection completeness is advisory only.",
			Explanation: &rule.FindingExplanation{
				Why:        "The statement stores blank or zero defaults for omitted members, which is valid but leaves the defined connection incomplete for actual use.",
				Risk:       "This notice is an audit-model completeness observation only — it does not refuse the statement and does not assert a native MySQL warning.",
				Suggestion: "Review whether the omitted connection members are intentional for this server entry.",
			},
			Metadata: map[string]any{
				"operation": string(statement.DDL.Operation),
				"object":    "server",
				"notice":    "connection_members",
			},
		})
	}
	return findings, nil
}

// The audited identifier-form ALTER SERVER first slice
// (T26-SERVER-ALTER-EXECUTION-01; accepted contract
// docs/verification/t26-server-alter-contract-01, R5 ACCEPTED
// 93945c01). The rule reads the same one truth as the siblings —
// spec.AdjudicateServerAlter's arm plus its derived owed-fact set:
// server.existence, server.batch_effects, server.alter_privilege,
// and server.alter_options are owed exactly per arm. The terminal
// refusal owes nothing (the ordered absent carries the whole
// verdict), the proven-alter arm owes nothing, and every barrier
// source owes exactly {server.batch_effects}. A3/A4 arms emit the
// independent connection_members advisory with the contract's
// fixed text. Findings and gaps carry no authored server name, no
// option keys/values, no account, and no grant text.

type mysqlAlterServerRule struct {
	id    string
	level rule.Level
}

func newMySQLAlterServerRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return mysqlAlterServerRule{
		id:    ruleIDMySQLAlterServerNotice,
		level: configuredLevel(cfg, rule.LevelNotice),
	}, nil
}

func (r mysqlAlterServerRule) ID() string { return r.id }

func (r mysqlAlterServerRule) AppliesTo(statement spec.Statement) bool {
	return statement.Dialect == spec.DialectMySQL &&
		statement.Kind == spec.KindDDL &&
		statement.DDL != nil &&
		statement.DDL.Server != nil &&
		statement.DDL.Operation == spec.DDLOperationAlterServer &&
		statement.Unsupported == nil
}

// serverAlterAdjudicationFor returns the shared adjudication the
// enrichment attached — nil outside the audited slice (a legacy
// statement shape or a bypassed enrichment), where the defensive
// fallback keeps the pre-execution contract: every premise owed.
func serverAlterAdjudicationFor(statement spec.Statement) *spec.ServerAlterAdjudication {
	if statement.Metadata == nil {
		return nil
	}
	return statement.Metadata.ServerAlter
}

func (r mysqlAlterServerRule) missingServerFacts(statement spec.Statement) []string {
	if adjudication := serverAlterAdjudicationFor(statement); adjudication != nil {
		// The shared adjudication derives the owed set per arm — the
		// rule never recomputes premises on its own.
		return adjudication.Owed
	}
	missing := []string{}
	state := serverStateFor(statement)
	if state != nil && state.UnreadReason == spec.MetadataUnreadBatchEffects {
		missing = append(missing, spec.ServerFactBatchEffects)
	} else {
		missing = append(missing, spec.ServerFactExistence)
	}
	missing = append(missing, spec.ServerFactAlterPrivilege, spec.ServerFactAlterOptions)
	return missing
}

func (r mysqlAlterServerRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
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

// serverAlterMessage renders the arm's bounded public message —
// wrapper-agnostic by contract: no server name, no option keys or
// values, no account, no grant text, and no claim about the stored
// row's wrapper.
func serverAlterMessage(statement spec.Statement, adjudication *spec.ServerAlterAdjudication) string {
	state := serverStateFor(statement)
	if adjudication == nil {
		return "ALTER SERVER targets a server definition whose existence, execution premise, and options remain unverified"
	}
	consumable := adjudication.Privilege == spec.ServerAlterPrivilegeConsumable
	switch adjudication.Arm {
	case spec.ServerAlterArmProvenAlter:
		return "ALTER SERVER is conditionally proven to update its catalog entry's stored options under the observed_account_new_session conditional audit"
	case spec.ServerAlterArmTerminalRefusal:
		return "ALTER SERVER is refused by the ordered state in force — the audited entry's absent state stands proven in this batch"
	case spec.ServerAlterArmBarrier:
		return "ALTER SERVER leaves its premises shadowed by earlier batch effects; this statement's execution premise remains unverified"
	default:
		items := serverAlterOwedItems(adjudication)
		if len(items) == 0 {
			if state != nil && (state.Read && state.Exists || state.OrderedPresent) {
				return "ALTER SERVER targets a catalog entry proven present at bind time; execution remains unverified"
			}
			return "ALTER SERVER targets a server definition whose existence remains unverified"
		}
		verb := "remain"
		if len(items) == 1 {
			verb = "remains"
		}
		phrase := serverCreateOwedPhrase(items)
		if consumable {
			return "ALTER SERVER targets a server definition whose " + phrase + " " + verb + " unverified under the observed_account_new_session conditional audit"
		}
		return "ALTER SERVER targets a server definition whose " + phrase + " " + verb + " unverified"
	}
}

// serverAlterOwedItems projects the adjudication's owed-fact set
// into the fixed public vocabulary — the arm's own bounded tokens,
// never an object name or option content. The order follows the
// owed set's canonical existence → privilege → options order.
func serverAlterOwedItems(adjudication *spec.ServerAlterAdjudication) []string {
	var items []string
	for _, fact := range adjudication.Owed {
		switch fact {
		case spec.ServerFactExistence:
			items = append(items, "existence")
		case spec.ServerFactAlterPrivilege:
			items = append(items, "execution premise")
		case spec.ServerFactAlterOptions:
			items = append(items, "options")
		}
	}
	return items
}

// serverAlterOutcome is the bounded outcome token published in
// finding metadata — one fixed vocabulary per arm so public
// consumers read the arm without parsing prose.
func serverAlterOutcome(adjudication *spec.ServerAlterAdjudication) string {
	if adjudication == nil {
		return "unverified"
	}
	switch adjudication.Arm {
	case spec.ServerAlterArmProvenAlter:
		return "conditional_alter"
	case spec.ServerAlterArmTerminalRefusal:
		return "refused_absent"
	case spec.ServerAlterArmBarrier:
		return "blocked_batch_effects"
	default:
		return "unverified"
	}
}

func (r mysqlAlterServerRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	adjudication := serverAlterAdjudicationFor(statement)
	consumable := adjudication != nil && adjudication.Privilege == spec.ServerAlterPrivilegeConsumable

	level := r.level
	explanation := &rule.FindingExplanation{
		Why:        "ALTER SERVER updates option fields on the named mysql.servers row and requires SUPER; the stored connection members it leaves untouched are never read by this audit.",
		Risk:       "Whether the statement actually executed is unproven — the privilege premise is not discharged, and batch writes may already shadow the catalog answer.",
		Suggestion: "Confirm the server entry exists and the executing account holds SUPER before relying on the alter.",
	}
	if adjudication != nil {
		switch adjudication.Arm {
		case spec.ServerAlterArmTerminalRefusal:
			// A proven refusal is the same finding class the sibling
			// families emit — a refusal blocker, not an advisory
			// notice. The ordered absent alone carries the verdict;
			// no specific native response is claimed beyond the
			// refusal.
			level = rule.LevelBlocker
			explanation = &rule.FindingExplanation{
				Why:        "ALTER SERVER requires the named mysql.servers entry to exist; the audited entry's absent state stands proven in this batch's ordered model, so the statement is refused.",
				Risk:       "The statement cannot land under the ordered state in force — the absent proof is the batch's own ordered state, never a catalog guess; the specific native response is not asserted beyond the refusal.",
				Suggestion: "Remove the duplicate alter or create the server entry first if the update is intended.",
			}
		case spec.ServerAlterArmProvenAlter:
			explanation = &rule.FindingExplanation{
				Why:        "ALTER SERVER updates option fields on the named mysql.servers row and requires SUPER; the conditional audit bound the observed account's direct global grant as the expected executor's premise and presence stands proven.",
				Risk:       conditionalRiskTail(true) + " The stored members the authored OPTIONS list leaves untouched are never read — the update is modeled as partial.",
				Suggestion: "Confirm the authored option set is the complete intended change; members not listed keep their stored values.",
			}
		case spec.ServerAlterArmBarrier:
			explanation = &rule.FindingExplanation{
				Why:        "ALTER SERVER updates option fields on the named mysql.servers row and requires SUPER; an earlier statement's undischarged effect on the server channel shadows this statement's premises.",
				Risk:       "The batch's own effects left this statement's premises undecidable; the shadowed premises never reached evaluation, so whether the alter could land cannot be proven in this batch.",
				Suggestion: "Order the ALTER before unknown-effect writes, or split the batch so each statement's premises can be proven.",
			}
		default:
			if consumable {
				phrase := serverCreateOwedPhrase(serverAlterOwedItems(adjudication))
				explanation = &rule.FindingExplanation{
					Why:        "ALTER SERVER updates option fields on the named mysql.servers row and requires SUPER; the conditional audit discharged the privilege premise but " + phrase + " stayed unproven.",
					Risk:       conditionalRiskTail(true) + " Whether the alter lands is unproven — the stated premises stayed unverified.",
					Suggestion: "Confirm the server entry exists and the authored options are valid before relying on the alter.",
				}
			}
		}
	}

	metadata := map[string]any{
		"operation": string(statement.DDL.Operation),
		"object":    "server",
		"outcome":   serverAlterOutcome(adjudication),
	}
	if adjudication != nil && adjudication.ConditionalAudit {
		// The marker is the adjudication's own provenance fact —
		// projected verbatim from Metadata.ServerAlter.ConditionalAudit:
		// the conditional scope actually fed the conclusion, either
		// through this statement's consumable premise on an arm that
		// read it or through an ordered source a prior statement
		// published under that scope. A barrier never carries it, and
		// a refusal carries it only when the consumed ordered absent
		// was conditional.
		metadata["conditional_audit"] = string(spec.PrivilegeAuditModeObservedAccountNewSession)
	}
	findings := []rule.Finding{{
		Level:       level,
		Message:     serverAlterMessage(statement, adjudication),
		Explanation: explanation,
		Metadata:    metadata,
	}}

	// The decided ALTER-scoped advisory (contract §G P2): the authored
	// OPTIONS list is intentionally partial and the stored set is
	// never read, so the stored and resulting connection member sets
	// stay unverified. Emitted on the A3/A4 arms only — a barrier or
	// refusal arm never emits it — at the rule's configured level,
	// with no outcome, marker, gap, or authored option payload.
	if adjudication == nil || adjudication.Arm == spec.ServerAlterArmProvenAlter || adjudication.Arm == spec.ServerAlterArmUnproven {
		findings = append(findings, rule.Finding{
			Level:   r.level,
			Message: "ALTER SERVER specifies a partial options update; stored and resulting connection member sets are unverified.",
			Explanation: &rule.FindingExplanation{
				Why:        "The authored OPTIONS list updates only the named members; members it leaves untouched keep their stored values, which this audit never reads.",
				Risk:       "This notice is an audit-model scope observation only — it does not refuse the statement and does not assert a native MySQL warning.",
				Suggestion: "Review whether the partial option update is the intended change for this server entry.",
			},
			Metadata: map[string]any{
				"operation": string(statement.DDL.Operation),
				"object":    "server",
				"notice":    "connection_members",
			},
		})
	}
	return findings, nil
}
