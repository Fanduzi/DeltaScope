// Package ddl: T26-SERVER-DROP-01 — the audited MySQL identifier-form
// DROP SERVER slice (accepted contract:
// docs/verification/t26-semantics-01, b8a67e7b).
// The rule emits one notice finding for every audited DROP SERVER and
// owes the bounded gap set the adjudication cannot vouch for:
// server.existence (unproven catalog answer), server.batch_effects
// (an earlier batch statement's undischarged unknown effect shadowed
// the catalog), and server.drop_privilege — always owed on the audited
// path because SUPER/grant evidence is not wired and the execution
// premise stays unproven. IF EXISTS does not publish absent without
// that privilege proof: the guard only skips the error, it does not
// license an absent finding. Findings and gaps carry no authored
// server name — the name stays internal to the batch channel.

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

func (r mysqlDropServerRule) missingServerFacts(statement spec.Statement) []string {
	missing := []string{}
	state := serverStateFor(statement)
	proven := state != nil && state.Read && state.Exists
	if !proven {
		if state != nil && state.UnreadReason == spec.MetadataUnreadBatchEffects {
			// The ordered channel's undischarged unknown effect shadowed
			// the catalog — the batch-effects gap replaces the existence
			// gap (same dedup contract as the tablespace family).
			missing = append(missing, "server.batch_effects")
		} else {
			missing = append(missing, "server.existence")
		}
	}
	// DROP SERVER needs SUPER — grant evidence is never wired, so the
	// execution premise stays owed on every audited path, present or
	// unproven alike.
	missing = append(missing, "server.drop_privilege")
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

func (r mysqlDropServerRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	state := serverStateFor(statement)
	message := "DROP SERVER targets a server definition whose existence and execution remain unverified"
	if state != nil && state.Read && state.Exists {
		message = "DROP SERVER targets a catalog entry proven present at bind time; execution remains unverified"
	}
	metadata := map[string]any{
		"operation": string(statement.DDL.Operation),
		"object":    "server",
	}
	if statement.DDL.Server.IfExists {
		metadata["if_exists"] = true
	}
	return []rule.Finding{{
		Level:   r.level,
		Message: message,
		Explanation: &rule.FindingExplanation{
			Why:        "DROP SERVER removes the mysql.servers row defining a foreign data wrapper connection and requires SUPER.",
			Risk:       "Whether the statement actually executed is unproven — the privilege premise is not wired, and batch writes may already shadow the catalog answer. FEDERATED tables created with the connection information are not removed with it, but definitions or code resolving through this server entry could be affected if the removal lands.",
			Suggestion: "Confirm the server entry is no longer needed and that the executing account holds SUPER before relying on the drop.",
		},
		Metadata: metadata,
	}}, nil
}
