// Package audit: T26-SRS-DROP-PRIVILEGE-EXECUTION-01 — the DROP
// SPATIAL REFERENCE SYSTEM privilege premise's family slice. The SRS
// premise is an INDEPENDENT projection of the shared request-level
// observation: it reads the same raw account/grant evidence and the
// same ordered-scope flag the SERVER family already owns, never a
// SERVER statement's adjudication, marker, or outcome, and never the
// SRS catalog reads themselves (accepted contract
// docs/verification/t26-srs-drop-privilege-contract-01, R3 candidate
// 2f7231b5).
package audit

import (
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// statementsNeedMySQLSRSPrivilege reports whether the batch names at
// least one audited DROP SPATIAL REFERENCE SYSTEM — the SRS slice the
// shared privilege observation additionally exists for. Only a
// version-qualified statement triggers the read: the phase-2 version
// gate already marked every out-of-domain drop Unsupported before this
// gate evaluates, so the check stays final-qualification-only and a
// boundary statement can never pull the loader (T26-SRS-DROP-
// PRIVILEGE-CONTRACT-01). A blocked-arm statement still qualifies —
// the batch-effects debt is a later adjudication answer, not a loader
// exclusion.
func statementsNeedMySQLSRSPrivilege(statements []spec.Statement) bool {
	for _, statement := range statements {
		if statement.DDL != nil && statement.DDL.SpatialReference != nil &&
			statement.DDL.SpatialReference.Kind == spec.SRSKindDrop &&
			statement.Unsupported == nil {
			return true
		}
	}
	return false
}

// srsDropPrivilegePremise projects the request-level witness onto this
// statement's own SRS premise — the same raw observation and ordered
// scope as every other family member, but an independent projection: a
// satisfied SERVER premise is never borrowed, a SERVER finding's
// conditional marker never discharges the SRS premise, and the SRS
// existence/dependency answers never count as privilege evidence. A
// scope end without an established positive witness keeps the premise
// unproven, never suppressed.
func (s *batchState) srsDropPrivilegePremise() spec.SRSDropPrivilege {
	if !s.serverDropPrivilegeWitness {
		return spec.SRSDropPrivilegeUnproven
	}
	if s.serverPrivilegeScopeEnded {
		return spec.SRSDropPrivilegeSuppressed
	}
	return spec.SRSDropPrivilegeConsumable
}
