// Package spec: T26-ALTER-INSTANCE-PUBLICATION-01 — the audited MySQL
// ALTER INSTANCE slice over the reviewer-frozen contract (Phase 0
// evidence candidate accepted; the frozen clause set D1..D10 lives in
// the work-order record). One shared adjudication is the single truth
// the rule projection and the batch own-apply both read — the rule
// never recomputes premises and apply never re-derives an arm.
//
// The slice audits exactly three authored forms (D1):
//
//   - ALTER INSTANCE RELOAD TLS — form reload_tls, MySQL 8.0+.
//   - ALTER INSTANCE RELOAD TLS NO ROLLBACK ON ERROR — the same
//     reload_tls feature plus the no_rollback_on_error variant
//     member: an independent failure-semantics variant, never merged
//     into the plain arm (D7).
//   - ALTER INSTANCE ROTATE INNODB MASTER KEY — form
//     rotate_innodb_master_key, MySQL 5.7.11+.
//
// Every other ALTER INSTANCE form stays outside the slice — parser
// error or a named boundary, exactly as before.
//
// Conservative-by-design: the statement's success return code is never
// proof the operation took effect (D5/D6). RELOAD TLS presumes a usable
// TLS context (native 3888 on absent material); ROTATE presumes a
// loaded keyring backend (native 3185); both presume their privilege
// (native 1227). No provider seam supplies TLS-context or keyring
// state this round (D4), so the instance premise stays owed on every
// adjudicated statement and the verdict stays bounded — unknown is
// never inferred into available or unavailable.
package spec

// Owed-fact vocabulary for the audited ALTER INSTANCE slice.
const (
	// AlterInstanceFactBatchEffects is owed when an earlier batch
	// statement already mutated the instance surface the premise
	// would read — the same blocked-bind contract as
	// srs.batch_effects.
	AlterInstanceFactBatchEffects = "alter_instance.batch_effects"
	// AlterInstanceFactPrivilege is the execution-privilege premise —
	// owed on every arm whose own privilege projection is not
	// consumable (D3).
	AlterInstanceFactPrivilege = "alter_instance.privilege"
	// AlterInstanceFactTLSContext is the reload premise: the server
	// holds a usable TLS context to reload. Owed while unproven —
	// no provider seam supplies it and rc=0 never proves it.
	AlterInstanceFactTLSContext = "alter_instance.tls_context"
	// AlterInstanceFactKeyring is the rotate premise: a keyring
	// backend is loaded and initialized. Owed while unproven.
	AlterInstanceFactKeyring = "alter_instance.keyring"
	// AlterInstanceFactVersionApplicability is the named promotion
	// premise a TiDB boundary entry owes before it can claim audited
	// support (T32/#111): the subaction is real TiDB semantics, but
	// unaudited stays the honest class until the version-applicability
	// domain for a TiDB audit slice is proven. It names the owed fact
	// — the gate itself is not installed until the slice exists.
	AlterInstanceFactVersionApplicability = "alter_instance.version_applicability"
)

// AlterInstanceForm identifies the authored subaction — the
// declaration member the version gate and the premise vocabulary both
// key on.
type AlterInstanceForm int

const (
	// AlterInstanceFormReloadTLS is `ALTER INSTANCE RELOAD TLS`; the
	// NO ROLLBACK ON ERROR modifier is the declaration's variant
	// member on this form, not a separate form or feature id (D1/D7).
	AlterInstanceFormReloadTLS AlterInstanceForm = iota
	// AlterInstanceFormRotateInnodbMasterKey is
	// `ALTER INSTANCE ROTATE INNODB MASTER KEY`.
	AlterInstanceFormRotateInnodbMasterKey
)

// AlterInstanceDeclaration is the typed declaration the extractor
// binds for one authored ALTER INSTANCE form — presence/absence
// members only; it carries no payload text.
type AlterInstanceDeclaration struct {
	// Form is the authored subaction.
	Form AlterInstanceForm
	// NoRollbackOnError records the authored NO ROLLBACK ON ERROR
	// member on the reload_tls form. It is variant metadata with
	// independent failure semantics (D7): a successful return code
	// under it is even less informative than a plain rc 0.
	NoRollbackOnError bool
}

// Feature returns the stable feature identity — the variant member
// never changes it.
func (d *AlterInstanceDeclaration) Feature() string {
	if d.Form == AlterInstanceFormRotateInnodbMasterKey {
		return "alter_instance.rotate_innodb_master_key"
	}
	return "alter_instance.reload_tls"
}

// InstancePremise returns the owed instance-fact token this form's
// outcome needs — TLS context for reload forms, keyring for rotate.
func (d *AlterInstanceDeclaration) InstancePremise() string {
	if d.Form == AlterInstanceFormRotateInnodbMasterKey {
		return AlterInstanceFactKeyring
	}
	return AlterInstanceFactTLSContext
}

// AlterInstancePrivilege is one statement's derived privilege
// premise projection — a per-statement verdict over the shared
// request-level observed-account grant evidence, never a mutable
// field on shared instance facts (D3).
type AlterInstancePrivilege int

const (
	// AlterInstancePrivilegeUnproven means the premise is not
	// consumable: no observation, unbound principal, or no positive
	// grant token for this form. A missing dynamic-grant observation
	// is unproven — never a proven denial (D3).
	AlterInstancePrivilegeUnproven AlterInstancePrivilege = iota
	// AlterInstancePrivilegeConsumable means a proven grant witness
	// satisfies this form's requirement: SUPER / ALL PRIVILEGES, or
	// the form's dynamic grant observed in the same request-level
	// evidence (CONNECTION_ADMIN for reload forms,
	// ENCRYPTION_KEY_ADMIN for rotate on MySQL 8.0+).
	AlterInstancePrivilegeConsumable
	// AlterInstancePrivilegeSuppressed means the positive witness
	// exists but an earlier authored grant/role statement ended the
	// applicable range — the premise stays owed for this statement.
	AlterInstancePrivilegeSuppressed
)

// AlterInstanceArm is the adjudicated arm of one audited ALTER
// INSTANCE — the single classification that drives findings, gaps,
// coverage, and verdict.
type AlterInstanceArm int

const (
	// AlterInstanceArmBlocked (B): the bind was blocked before the
	// statement's own premises were read — an earlier batch statement
	// already mutated the instance surface. Owed = {batch_effects}.
	AlterInstanceArmBlocked AlterInstanceArm = iota
	// AlterInstanceArmUnproven (U): every qualified input. No
	// provider seam proves the TLS-context or keyring premise this
	// round and no success return code can discharge it, so the
	// outcome stays bounded. Owed = the instance premise plus the
	// privilege premise while it is not consumable.
	AlterInstanceArmUnproven
)

// AlterInstanceAdjudication is the shared per-statement
// adjudication — the one truth the rule projection and the own apply
// both read.
type AlterInstanceAdjudication struct {
	// Arm is the adjudicated arm.
	Arm AlterInstanceArm
	// Privilege is this statement's own derived premise projection —
	// unproven, consumable, or scope-suppressed.
	Privilege AlterInstancePrivilege
	// Owed lists the bounded gap facts this adjudication still owes —
	// derived from the arm's actual premises, never a fixed rule.
	Owed []string
	// ConditionalAudit marks that the conditional privilege scope
	// actually fed this adjudication's conclusion — the statement's
	// own consumable premise reached the outcome arm.
	ConditionalAudit bool
}

// AdjudicateAlterInstance computes the shared adjudication for one
// audited MySQL ALTER INSTANCE. The caller guarantees the audited
// slice — MySQL dialect, a recognized authored form, no unsupported
// marker, resolved version inside the form's audited domain — so the
// arm table walks the contract's first-match order:
//
//   - B: blocked bind — an earlier batch statement already mutated
//     the instance surface the premise would read (the same
//     contamination contract as the SRS family). Owed =
//     {alter_instance.batch_effects}.
//   - U: unproven — the instance premise (TLS context / keyring) is
//     never supplied this round and cannot be discharged by a return
//     code, so the outcome always stays bounded. Owed = the form's
//     instance premise plus alter_instance.privilege while the
//     statement's own projection is not consumable. NO ROLLBACK ON
//     ERROR changes no arm identity — it changes only how little a
//     success rc means, which this adjudication never consumes.
func AdjudicateAlterInstance(batchContaminated bool, decl *AlterInstanceDeclaration, privilege AlterInstancePrivilege) AlterInstanceAdjudication {
	adjudication := AlterInstanceAdjudication{Privilege: privilege}
	if batchContaminated {
		adjudication.Arm = AlterInstanceArmBlocked
		adjudication.Owed = []string{AlterInstanceFactBatchEffects}
		return adjudication
	}
	adjudication.Arm = AlterInstanceArmUnproven
	if privilege == AlterInstancePrivilegeConsumable {
		adjudication.ConditionalAudit = true
	} else {
		adjudication.Owed = append(adjudication.Owed, AlterInstanceFactPrivilege)
	}
	adjudication.Owed = append(adjudication.Owed, decl.InstancePremise())
	return adjudication
}

// MySQLAlterInstanceAuditedDomain reports whether the resolved server
// version lies inside the audited domain for this form (D2): the
// RELOAD TLS grammar exists on MySQL 8.0+ (5.7 rejects it at grammar
// level — probed ERROR 1064 on 5.7.44), while ROTATE INNODB MASTER
// KEY exists from MySQL 5.7.11 (probed rc 0 on 5.7.44, 8.0.46, and
// 8.4.10). A nil, non-MySQL, or unvalidated version is never in
// domain.
func MySQLAlterInstanceAuditedDomain(version *VersionIdentity, form AlterInstanceForm) bool {
	if version == nil || version.Product != VersionProductMySQL || !version.ValidatedRange {
		return false
	}
	switch form {
	case AlterInstanceFormReloadTLS:
		return version.Major >= 8
	case AlterInstanceFormRotateInnodbMasterKey:
		return version.Major >= 8 ||
			(version.Major == 5 && version.Minor == 7 && version.Patch >= 11)
	}
	return false
}

// MySQLAlterInstanceDynamicGrantDomain reports whether the resolved
// server version lies inside the domain where the dynamic-grant
// tokens this slice consumes (CONNECTION_ADMIN,
// ENCRYPTION_KEY_ADMIN) can exist — MySQL 8.0+ only. An observed
// token outside the domain is never a satisfaction: on 5.7 the
// rotate requirement is SUPER, which the shared SUPER witness path
// already covers.
func MySQLAlterInstanceDynamicGrantDomain(version *VersionIdentity) bool {
	return version != nil &&
		version.Product == VersionProductMySQL &&
		version.ValidatedRange &&
		version.Major >= 8
}
