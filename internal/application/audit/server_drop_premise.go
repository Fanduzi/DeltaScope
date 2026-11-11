// Package audit: T26-SERVER-DROP-EXECUTION-01 — the DROP SERVER
// privilege premise's ordered-scope helpers. The request-level raw
// observation and its once-derived positive-witness verdict live on
// batchState; this file carries the three projections that ride them:
// the narrow statement gate that decides whether the loader runs at
// all, the ordered-range terminator that suppresses the witness after
// an authored grant/role statement, and the per-statement premise that
// feeds the shared adjudication. The bounded error type wraps a failed
// observation with the same contract the object seam already uses.
package audit

import (
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// statementsNeedMySQLServerPrivilege reports whether the batch names
// at least one audited identifier-form DROP SERVER, a fully
// qualified CREATE SERVER, or an audited identifier-form ALTER
// SERVER — the SERVER family slice the privilege observation exists
// for. Boundary forms (literal names, shape- or version-failed
// members, any other family) never trigger the read:
// statement.Unsupported already encodes both the phase-1 shape gates
// and the phase-2 version gate, so the check stays
// final-qualification-only (T26-SERVER-CREATE-EXECUTION-01,
// T26-SERVER-ALTER-EXECUTION-01).
func statementsNeedMySQLServerPrivilege(statements []spec.Statement) bool {
	for _, statement := range statements {
		if statement.DDL != nil && statement.DDL.Server != nil &&
			statement.DDL.Server.NameIdent && statement.Unsupported == nil {
			switch statement.DDL.Server.Kind {
			case spec.ServerKindDrop, spec.ServerKindCreate, spec.ServerKindAlter:
				return true
			}
		}
	}
	return false
}

// serverPrivilegeScopeTerminator reports whether the statement ends
// the privilege observation's applicable range — an authored
// GRANT/REVOKE operation or the registered boundary features
// (set_role, set_default_role, grant_role, revoke_role, grant_proxy).
// The range ends for every LATER statement: the relied-upon direct
// grant may have been revoked or the session's role set changed after
// the observation. The contract's conservative list is exactly these —
// rename_user stays out of scope this round. A terminator is a
// conservative scope policy, not a claim that the statement
// necessarily revoked a direct SUPER grant.
func serverPrivilegeScopeTerminator(statement spec.Statement) bool {
	if statement.DDL != nil {
		switch statement.DDL.Operation {
		case spec.DDLOperationGrant, spec.DDLOperationRevoke:
			return true
		}
	}
	if statement.Unsupported != nil {
		switch statement.Unsupported.Feature {
		case "set_role", "set_default_role", "grant_role", "revoke_role", "grant_proxy":
			return true
		}
	}
	return false
}

// serverDropPrivilegePremise projects the request-level witness onto
// this statement's premise: the raw positive witness must exist and
// the ordered range must still be open; a suppressed witness stays
// owed exactly like an unproven one — never a denial.
func (s *batchState) serverDropPrivilegePremise() spec.ServerDropPrivilege {
	if !s.serverDropPrivilegeWitness {
		return spec.ServerDropPrivilegeUnproven
	}
	if s.serverPrivilegeScopeEnded {
		return spec.ServerDropPrivilegeSuppressed
	}
	return spec.ServerDropPrivilegeConsumable
}

// serverDropAdjudicationOf returns the shared adjudication attached to
// one audited DROP SERVER — nil outside the audited slice or before
// enrichment ran.
func serverDropAdjudicationOf(statement spec.Statement) *spec.ServerDropAdjudication {
	if statement.Metadata == nil {
		return nil
	}
	return statement.Metadata.ServerDrop
}

// serverCreatePrivilegePremise projects the request-level witness onto
// this statement's CREATE premise — the same raw observation and the
// same ordered scope as the DROP sibling, but an independent
// projection: a satisfied drop premise is never borrowed and a prior
// drop's conditional marker never discharges the create's own premise
// (T26-SERVER-CREATE-EXECUTION-01).
func (s *batchState) serverCreatePrivilegePremise() spec.ServerCreatePrivilege {
	if !s.serverDropPrivilegeWitness {
		return spec.ServerCreatePrivilegeUnproven
	}
	if s.serverPrivilegeScopeEnded {
		return spec.ServerCreatePrivilegeSuppressed
	}
	return spec.ServerCreatePrivilegeConsumable
}

// serverCreateAdjudicationOf returns the shared adjudication attached
// to one audited CREATE SERVER — nil outside the audited slice or
// before enrichment ran.
func serverCreateAdjudicationOf(statement spec.Statement) *spec.ServerCreateAdjudication {
	if statement.Metadata == nil {
		return nil
	}
	return statement.Metadata.ServerCreate
}

// serverAlterPrivilegePremise projects the request-level witness onto
// this statement's ALTER premise — the same raw observation and the
// same ordered scope as the drop/create siblings, but an independent
// projection: a satisfied drop or create premise is never borrowed
// and a sibling's conditional marker never discharges the alter's own
// premise (T26-SERVER-ALTER-EXECUTION-01).
func (s *batchState) serverAlterPrivilegePremise() spec.ServerAlterPrivilege {
	if !s.serverDropPrivilegeWitness {
		return spec.ServerAlterPrivilegeUnproven
	}
	if s.serverPrivilegeScopeEnded {
		return spec.ServerAlterPrivilegeSuppressed
	}
	return spec.ServerAlterPrivilegeConsumable
}

// serverAlterAdjudicationOf returns the shared adjudication attached
// to one audited ALTER SERVER — nil outside the audited slice or
// before enrichment ran.
func serverAlterAdjudicationOf(statement spec.Statement) *spec.ServerAlterAdjudication {
	if statement.Metadata == nil {
		return nil
	}
	return statement.Metadata.ServerAlter
}

// serverPrivilegeReadError bounds the public text of a failed
// privilege observation — the same contract as
// metadataObjectReadError: a fixed cause-free message outside while
// Unwrap keeps the original cause reachable for errors.Is/As and for
// adapter-level cancellation/timeout recognition. A failed observation
// never degrades into an unproven grant, an evidence gap, or a cached
// answer.
type serverPrivilegeReadError struct{ cause error }

func (serverPrivilegeReadError) Error() string {
	return "server drop privilege observation failed"
}

func (e serverPrivilegeReadError) Unwrap() error { return e.cause }
