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

// statementsNeedMySQLServerDropPrivilege reports whether the batch
// names at least one audited identifier-form DROP SERVER — the narrow
// slice the privilege observation exists for. Boundary forms (literal
// names, CREATE/ALTER SERVER, any other family) never trigger the
// read.
func statementsNeedMySQLServerDropPrivilege(statements []spec.Statement) bool {
	for _, statement := range statements {
		if statement.DDL != nil && statement.DDL.Server != nil &&
			statement.DDL.Server.Kind == spec.ServerKindDrop &&
			statement.DDL.Server.NameIdent && statement.Unsupported == nil {
			return true
		}
	}
	return false
}

// serverPrivilegeScopeTerminator reports whether the statement ends
// the privilege observation's applicable range — an authored
// GRANT/REVOKE operation or the registered boundary features
// (set_role, grant_role, revoke_role, grant_proxy). The range ends for
// every LATER statement: the relied-upon direct grant may have been
// revoked or the session's role set changed after the observation.
// The contract's conservative list is exactly these — set_default_role
// and rename_user stay out of scope this round.
func serverPrivilegeScopeTerminator(statement spec.Statement) bool {
	if statement.DDL != nil {
		switch statement.DDL.Operation {
		case spec.DDLOperationGrant, spec.DDLOperationRevoke:
			return true
		}
	}
	if statement.Unsupported != nil {
		switch statement.Unsupported.Feature {
		case "set_role", "grant_role", "revoke_role", "grant_proxy":
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
