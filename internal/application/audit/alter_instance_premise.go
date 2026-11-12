// Package audit: T26-ALTER-INSTANCE-PUBLICATION-01 — the ALTER
// INSTANCE privilege premise projection. The statement's premise is
// its own projection over the shared request-level observed-account
// grant evidence (frozen D3): a proven SUPER / ALL PRIVILEGES witness
// satisfies every form; a proven form-specific dynamic grant
// (CONNECTION_ADMIN for RELOAD TLS, ENCRYPTION_KEY_ADMIN for ROTATE
// INNODB MASTER KEY on MySQL 8.0+) satisfies the corresponding form.
// A missing dynamic-grant observation is unproven — never a proven
// denial — and the premise never widens into a cross-feature
// privilege framework: it reads the same raw observation the SERVER
// and SRS families already load, nothing more.
package audit

import "github.com/Fanduzi/DeltaScope/internal/domain/spec"

// statementsNeedMySQLAlterInstancePrivilege reports whether the batch
// names at least one audited MySQL ALTER INSTANCE form — the trigger
// that lets the shared request-level observed-account grant loader run
// for this family under the request-selected conditional mode.
func statementsNeedMySQLAlterInstancePrivilege(statements []spec.Statement) bool {
	for _, statement := range statements {
		if statement.Dialect != spec.DialectMySQL ||
			statement.Kind != spec.KindDDL ||
			statement.DDL == nil ||
			statement.DDL.AlterInstance == nil ||
			statement.Unsupported != nil {
			continue
		}
		return true
	}
	return false
}

// alterInstancePrivilegePremise projects the request-level
// observed-account grant evidence onto this ALTER INSTANCE
// statement's own premise (frozen D3):
//
//   - a proven SUPER / ALL PRIVILEGES witness satisfies every form —
//     SUPER covers CONNECTION_ADMIN and ENCRYPTION_KEY_ADMIN natively.
//   - the form's dynamic grant observed in the same raw grant lines
//     satisfies the form: CONNECTION_ADMIN for RELOAD TLS;
//     ENCRYPTION_KEY_ADMIN for ROTATE on a proven MySQL 8.0+ domain
//     (the dynamic-grant vocabulary does not exist on 5.7, where the
//     native requirement is SUPER — covered by the witness path).
//   - an unbound principal, a missing observation, or no matching
//     token leaves the premise unproven — never a proven denial.
//   - an earlier authored grant/role statement ends the applicable
//     range only when a positive satisfaction was established —
//     suppressed stays owed, same contract as the SRS slices.
func (s *batchState) alterInstancePrivilegePremise(form spec.AlterInstanceForm) spec.AlterInstancePrivilege {
	satisfied := s.serverDropPrivilegeWitness || s.alterInstanceDynamicGrantHeld(form)
	if !satisfied {
		return spec.AlterInstancePrivilegeUnproven
	}
	if s.serverPrivilegeScopeEnded {
		return spec.AlterInstancePrivilegeSuppressed
	}
	return spec.AlterInstancePrivilegeConsumable
}

// alterInstanceDynamicGrantHeld reports whether the shared raw
// observation carries this form's dynamic grant on a bound principal
// — exact uppercase tokens folded from the observed global-scope
// grant lines, never substring matches and never fabricated from a
// SUPER-only provider.
func (s *batchState) alterInstanceDynamicGrantHeld(form spec.AlterInstanceForm) bool {
	observation := s.serverDropPrivilege
	if observation == nil || observation.Account == "" || !spec.MySQLAlterInstanceDynamicGrantDomain(s.resolvedVersion) {
		return false
	}
	set := map[string]bool{}
	spec.CollectMySQLGlobalPrivileges(set, observation.GrantLines)
	switch form {
	case spec.AlterInstanceFormReloadTLS:
		return set["CONNECTION_ADMIN"]
	case spec.AlterInstanceFormRotateInnodbMasterKey:
		return set["ENCRYPTION_KEY_ADMIN"]
	}
	return false
}
