package spec

import (
	"strconv"
	"strings"
)

// mysql_resource_group.go models the MySQL CPU resource-group family —
// a statement surface the locked TiDB grammar cannot produce, recovered
// by the parser adapter as typed declarations (issue #103 T24). It is
// deliberately separate from the TiDB RU model: TYPE/VCPU/
// THREAD_PRIORITY/ENABLE|DISABLE/FORCE have no RU counterpart, and the
// families never share a declaration so a boundary stays provable.

// MySQLResourceGroupOptionKind names one authored MySQL resource-group
// option member. MySQL's grammar gives each option a single positional
// slot — the recognizer never produces duplicates, so authored order is
// fidelity, never precedence.
type MySQLResourceGroupOptionKind string

const (
	MySQLResourceGroupOptionType           MySQLResourceGroupOptionKind = "type"
	MySQLResourceGroupOptionVCPU           MySQLResourceGroupOptionKind = "vcpu"
	MySQLResourceGroupOptionThreadPriority MySQLResourceGroupOptionKind = "thread_priority"
	MySQLResourceGroupOptionEnabled        MySQLResourceGroupOptionKind = "enabled"
	MySQLResourceGroupOptionForce          MySQLResourceGroupOptionKind = "force"
)

// MySQLResourceGroupType is the authored group class — fixed at CREATE
// and never alterable. SYSTEM groups hold background threads and accept
// negative priorities; USER groups hold session threads.
type MySQLResourceGroupType string

const (
	MySQLResourceGroupTypeSystem MySQLResourceGroupType = "system"
	MySQLResourceGroupTypeUser   MySQLResourceGroupType = "user"
)

// MySQLResourceGroupVCPUSpec is one authored VCPU member — a single CPU
// id (Lo == Hi) or an inclusive range Lo-Hi. Endpoints keep the authored
// u64 NUM values: upstream truncates each through cpu_id_t (uint32) when
// it builds the range vector, so the effective endpoints ride
// MySQLResourceGroupEffectiveVCPU. An inverted effective range is a
// declaration-provable semantic refusal (ER_INVALID_VCPU_RANGE), never a
// parse failure — the declaration keeps the member verbatim.
type MySQLResourceGroupVCPUSpec struct {
	Lo uint64 `json:"lo"`
	Hi uint64 `json:"hi"`
}

// MySQLResourceGroupEffectiveVCPU projects an authored endpoint onto
// upstream's cpu_id_t domain — mysqld stores each NUM through uint32
// when it builds the vcpu_range_vector, so the semantic checks below
// run on the truncated value exactly as the server computes it.
func MySQLResourceGroupEffectiveVCPU(v uint64) uint32 { return uint32(v) }

// MySQLResourceGroupEffectivePriority projects an authored
// THREAD_PRIORITY onto the stored domain — the parse tree keeps a
// signed int, so out-of-band literals narrow to int32 before the
// capability/type adjudication reads them.
func MySQLResourceGroupEffectivePriority(v int64) int64 { return int64(int32(v)) }

// MySQLResourceGroupOption is one authored MySQL resource-group option
// in declaration order. Exactly one payload member is populated per
// recognized kind; a member the recognizer cannot classify stays
// unclaimed — the statement falls back to the base grammar's
// diagnostics, never a silently accepted half-parse.
type MySQLResourceGroupOption struct {
	Kind MySQLResourceGroupOptionKind `json:"kind"`
	// GroupType carries the CREATE-time TYPE member.
	GroupType MySQLResourceGroupType `json:"group_type,omitempty"`
	// VCPUSpecs carries the authored VCPU list in order.
	VCPUSpecs []MySQLResourceGroupVCPUSpec `json:"vcpu_specs,omitempty"`
	// IntValue carries THREAD_PRIORITY — signed; the authored-type
	// bounds (system: -20..0, user: 0..19) are rule adjudications, not
	// lexical facts.
	IntValue *int64 `json:"int_value,omitempty"`
	// Enabled carries ENABLE (true) / DISABLE (false).
	Enabled *bool `json:"enabled,omitempty"`
}

// Clone returns a deep copy.
func (o MySQLResourceGroupOption) Clone() MySQLResourceGroupOption {
	out := o
	if o.VCPUSpecs != nil {
		out.VCPUSpecs = append([]MySQLResourceGroupVCPUSpec(nil), o.VCPUSpecs...)
	}
	if o.IntValue != nil {
		v := *o.IntValue
		out.IntValue = &v
	}
	if o.Enabled != nil {
		v := *o.Enabled
		out.Enabled = &v
	}
	return out
}

// MySQLResourceGroupDeclaration carries the authored facts of a MySQL
// CREATE/ALTER/DROP RESOURCE GROUP statement. It is internal evidence
// (never serialized): name and the ordered option list. MySQL's grammar
// has no IF EXISTS / IF NOT EXISTS guards — their absence is a grammar
// fact, and a TiDB-form guard parsed under the mysql dialect stays the
// vendor boundary the T22 contract owns.
type MySQLResourceGroupDeclaration struct {
	// Name is the statement's group identity — resource groups are
	// globally namespaced, never schema-qualified, and case-insensitive
	// under MySQL.
	Name string `json:"-"`
	// Options preserves every authored option in order.
	Options []MySQLResourceGroupOption `json:"-"`
}

// Clone returns a deep copy safe for ordered-state publication.
func (d *MySQLResourceGroupDeclaration) Clone() *MySQLResourceGroupDeclaration {
	if d == nil {
		return nil
	}
	out := *d
	if d.Options != nil {
		out.Options = make([]MySQLResourceGroupOption, len(d.Options))
		for i := range d.Options {
			out.Options[i] = d.Options[i].Clone()
		}
	}
	return &out
}

// declaredOption returns the authored option of a kind — each kind
// occupies a single positional grammar slot so at most one can exist.
func (d *MySQLResourceGroupDeclaration) declaredOption(kind MySQLResourceGroupOptionKind) (MySQLResourceGroupOption, bool) {
	var found MySQLResourceGroupOption
	ok := false
	if d == nil {
		return found, false
	}
	for _, option := range d.Options {
		if option.Kind == kind {
			found = option
			ok = true
		}
	}
	return found, ok
}

// DeclaredType returns the authored TYPE member (CREATE only), or
// ok=false when undeclared.
func (d *MySQLResourceGroupDeclaration) DeclaredType() (MySQLResourceGroupType, bool) {
	option, ok := d.declaredOption(MySQLResourceGroupOptionType)
	return option.GroupType, ok
}

// DeclaredVCPU returns the authored VCPU spec list, or ok=false when
// undeclared (an ALTER omitting VCPU keeps the proven affinity).
func (d *MySQLResourceGroupDeclaration) DeclaredVCPU() ([]MySQLResourceGroupVCPUSpec, bool) {
	option, ok := d.declaredOption(MySQLResourceGroupOptionVCPU)
	return option.VCPUSpecs, ok
}

// DeclaredThreadPriority returns the authored THREAD_PRIORITY value, or
// ok=false when undeclared.
func (d *MySQLResourceGroupDeclaration) DeclaredThreadPriority() (*int64, bool) {
	option, ok := d.declaredOption(MySQLResourceGroupOptionThreadPriority)
	return option.IntValue, ok
}

// DeclaredEnabled returns the authored ENABLE/DISABLE switch, or
// ok=false when undeclared.
func (d *MySQLResourceGroupDeclaration) DeclaredEnabled() (*bool, bool) {
	option, ok := d.declaredOption(MySQLResourceGroupOptionEnabled)
	return option.Enabled, ok
}

// DeclaredForce reports whether the statement authored FORCE.
func (d *MySQLResourceGroupDeclaration) DeclaredForce() bool {
	_, ok := d.declaredOption(MySQLResourceGroupOptionForce)
	return ok
}

// MySQLResourceGroupMaxNameBytes is upstream's NAME_CHAR_LEN — the
// check_resource_group_name_len bound compares the ident's byte length
// (name.length under the connection charset; utf8 here), so multibyte
// names count bytes, not characters.
const MySQLResourceGroupMaxNameBytes = 64

// DeclaredNameTooLong reports the declaration-provable
// ER_TOO_LONG_IDENT outcome — byte length under utf8, matching the
// upstream name.length comparison for an utf8 client.
func (d *MySQLResourceGroupDeclaration) DeclaredNameTooLong() bool {
	return d != nil && len(d.Name) > MySQLResourceGroupMaxNameBytes
}

// DeclaredVCPUInverted reports whether any authored member's effective
// range is inverted — start > end after the cpu_id_t narrowing, the
// declaration-provable ER_INVALID_VCPU_RANGE refusal upstream runs
// before the host-capacity check.
func (d *MySQLResourceGroupDeclaration) DeclaredVCPUInverted() bool {
	specs, ok := d.DeclaredVCPU()
	if !ok {
		return false
	}
	for _, member := range specs {
		if MySQLResourceGroupEffectiveVCPU(member.Lo) > MySQLResourceGroupEffectiveVCPU(member.Hi) {
			return true
		}
	}
	return false
}

// DeclaredVCPUNeedsHostFact reports whether any effective endpoint is
// nonzero — only those can exceed a host CPU count, and a running
// server always has at least one CPU so a zero endpoint provably fits.
func (d *MySQLResourceGroupDeclaration) DeclaredVCPUNeedsHostFact() bool {
	specs, ok := d.DeclaredVCPU()
	if !ok {
		return false
	}
	for _, member := range specs {
		if MySQLResourceGroupEffectiveVCPU(member.Lo) > 0 ||
			MySQLResourceGroupEffectiveVCPU(member.Hi) > 0 {
			return true
		}
	}
	return false
}

// DeclaredVCPUExceeds reports whether any effective endpoint is at or
// past the host's CPU count — upstream's ER_INVALID_VCPU_ID check,
// evaluated only when the count fact is proven.
func (d *MySQLResourceGroupDeclaration) DeclaredVCPUExceeds(vcpuCount uint64) bool {
	specs, ok := d.DeclaredVCPU()
	if !ok {
		return false
	}
	for _, member := range specs {
		if uint64(MySQLResourceGroupEffectiveVCPU(member.Lo)) >= vcpuCount ||
			uint64(MySQLResourceGroupEffectiveVCPU(member.Hi)) >= vcpuCount {
			return true
		}
	}
	return false
}

// DeclaredForceMisuse reports upstream's ER_INVALID_USE_OF_FORCE_OPTION
// shape: FORCE authored without the DISABLE pairing (missing member or
// an authored ENABLE). The caller gates it to ALTER — DROP's FORCE is
// always the legal occupancy modifier.
func (d *MySQLResourceGroupDeclaration) DeclaredForceMisuse() bool {
	if !d.DeclaredForce() {
		return false
	}
	enabled, ok := d.DeclaredEnabled()
	return !ok || enabled == nil || *enabled
}

// IsMySQLResourceGroupReservedName reports whether the name is one of
// the builtin groups — usr_default / sys_default, compared
// case-insensitively exactly as upstream's my_strcasecmp check does.
// Every consistent world refuses a lifecycle statement on one: the
// feature-off deployment refuses the statement itself, and the
// feature-on server finds the group present and refuses reserved-ness
// (or, for CREATE, the proven duplicate).
func IsMySQLResourceGroupReservedName(name string) bool {
	return strings.EqualFold(name, "usr_default") || strings.EqualFold(name, "sys_default")
}

// MySQLResourceGroupDefinition carries catalog-observed configuration
// for one MySQL resource group — the information_schema.RESOURCE_GROUPS
// row shape. Every member is nullable: absent means unobserved, never a
// guessed default or a conflated zero.
type MySQLResourceGroupDefinition struct {
	GroupType      MySQLResourceGroupType `json:"-"`
	Enabled        *bool                  `json:"-"`
	ThreadPriority *int64                 `json:"-"`
	// VCPUIDs carries the catalog's VCPU_IDS column text when observed —
	// bounded evidence; range semantics are not re-adjudicated.
	VCPUIDs *string `json:"-"`
}

// Clone returns a deep copy safe for ordered-state publication.
func (d *MySQLResourceGroupDefinition) Clone() *MySQLResourceGroupDefinition {
	if d == nil {
		return nil
	}
	out := *d
	if d.Enabled != nil {
		v := *d.Enabled
		out.Enabled = &v
	}
	if d.ThreadPriority != nil {
		v := *d.ThreadPriority
		out.ThreadPriority = &v
	}
	if d.VCPUIDs != nil {
		v := *d.VCPUIDs
		out.VCPUIDs = &v
	}
	return &out
}

// MySQLResourceGroupState is the ordered-state projection of one MySQL
// resource group as a statement adjudicates it: existence plus the
// proven definition. Status mirrors the MetadataStatus contract —
// confirmed and not_found are proven answers; anything else is unproven.
type MySQLResourceGroupState struct {
	Name   string         `json:"name,omitempty"`
	Status MetadataStatus `json:"status,omitempty"`
	Exists bool           `json:"exists"`
	// Reserved marks a builtin group (usr_default / sys_default) — a
	// fixed-version fact provable without a catalog read: reserved groups
	// always exist and can be neither altered nor dropped.
	Reserved   bool                          `json:"reserved,omitempty"`
	Definition *MySQLResourceGroupDefinition `json:"-"`
	// Occupied is the ordered answer to upstream's is_bound_to_threads()
	// — the fact DROP consults for ER_RESOURCE_GROUP_BUSY. nil means the
	// binding state was never proven; a batch-derived transition sets it
	// (CREATE lands unoccupied, DISABLE FORCE relocates to the defaults),
	// and an unaudited statement can never leave a guessed value behind.
	Occupied *bool `json:"-"`
}

// Clone returns a deep copy safe for statement-metadata publication.
func (s *MySQLResourceGroupState) Clone() *MySQLResourceGroupState {
	if s == nil {
		return nil
	}
	out := *s
	out.Definition = s.Definition.Clone()
	if s.Occupied != nil {
		v := *s.Occupied
		out.Occupied = &v
	}
	return &out
}

// Proven reports whether the state carries a proven existence answer.
func (s *MySQLResourceGroupState) Proven() bool {
	if s == nil {
		return false
	}
	return s.Status == MetadataStatusConfirmed || s.Status == MetadataStatusNotFound
}

// ─────────────────────────────────────────────────────────────────
// Shared statement outcome (T24-R2). One truth table replays upstream's
// prepare→execute layering against the authored declaration, the
// resolved version, the deployment facts, and the ordered pre-state:
// the valid rule renders its refusal reasons and premise gaps, and the
// ordered transition publishes or withholds the conditional post-state
// — so a blocker finding and a withheld fact can never diverge.
//
// Each check reports "refused" only when the proven facts refuse in
// every consistent world and "depends" when an unproven fact gates a
// possible refusal or a divergent post member. Any unconditional
// refusal makes the statement refused; otherwise an open dependency
// makes it undecidable. A refused statement emits no premise gaps —
// the missing facts cannot change the verdict.

// Gap reason codes — the bounded missing-fact contracts.
const (
	MySQLResourceGroupGapUnknownState       = "unknown_resource_group_state"
	MySQLResourceGroupGapUnknownVersion     = "unknown_resource_group_version"
	MySQLResourceGroupGapUnknownFeature     = "unknown_resource_group_feature"
	MySQLResourceGroupGapUnknownHostVCPU    = "unknown_host_vcpu_count"
	MySQLResourceGroupGapUnknownPrivilege   = "unknown_resource_group_privilege"
	MySQLResourceGroupGapUnknownPriorityCap = "unknown_resource_group_priority_capability"
	MySQLResourceGroupGapUnknownType        = "unknown_resource_group_type"
	MySQLResourceGroupGapUnknownOccupancy   = "unknown_resource_group_occupancy"
)

// Required-fact names the gaps name.
const (
	MySQLResourceGroupFactState       = "resource_group.state"
	MySQLResourceGroupFactVersion     = "instance.version"
	MySQLResourceGroupFactFeature     = "resource_group.feature_supported"
	MySQLResourceGroupFactHostVCPU    = "host.vcpu_count"
	MySQLResourceGroupFactPrivilege   = "privilege.resource_group_admin"
	MySQLResourceGroupFactPriorityCap = "resource_group.thread_priority_capable"
	MySQLResourceGroupFactType        = "resource_group.type"
	MySQLResourceGroupFactOccupancy   = "resource_group.bound_threads"
)

// MySQLResourceGroupGap is one missing premise — the rule renders it as
// an EvidenceGap.
type MySQLResourceGroupGap struct {
	ReasonCode    string
	RequiredFacts []string
}

// MySQLResourceGroupPost is the conditional post-state the proceeding
// branches would publish — per-member withheld where those branches
// diverge. The ordered transition merges it against the pre-state for
// undecidable outcomes.
type MySQLResourceGroupPost struct {
	Exists     bool
	Definition *MySQLResourceGroupDefinition
	Occupied   *bool
}

// MySQLResourceGroupOutcome is the shared verdict.
type MySQLResourceGroupOutcome struct {
	// Refused — every consistent world refuses (upstream proven-fact
	// layering; warnings, notices, and state publication all suppress).
	Refused bool
	// Undecidable — a refusing and a proceeding branch coexist, or an
	// unproven fact gates the post-state. The ordered transition merges
	// the pre-state with Post instead of publishing it.
	Undecidable bool
	// Reasons are the proven refusal reasons in upstream check order.
	Reasons []string
	// Gaps are the open premise dependencies — suppressed when Refused.
	Gaps []MySQLResourceGroupGap
	// Post is the proceeding-branch post-state (nil when Refused).
	Post *MySQLResourceGroupPost
}

// MySQLResourceGroupVersionSupported reports the 8.0.3 introduction
// boundary — the feature exists only on MySQL 8.0.3+. A nil or
// unvalidated version is unproven — never a claimed refusal.
func MySQLResourceGroupVersionSupported(version *VersionIdentity) (supported, proven bool) {
	if version == nil || !version.ValidatedRange || version.Product != VersionProductMySQL {
		return false, false
	}
	if version.Major > 8 || (version.Major == 8 && (version.Minor > 0 || version.Patch >= 3)) {
		return true, true
	}
	return false, true
}

// mysqlResourceGroupPriorityOutcome replays upstream's
// validate_resource_group_priority under every consistent capability
// and type branch: capability on bounds the authored value by group
// type (system -20..0, user 0..19); capability off ignores a nonzero
// authored value and stores zero. effective stays nil while proceeding
// branches diverge.
type mysqlResourceGroupPriorityOutcome struct {
	refuseAll  bool
	anyRefuse  bool
	effective  *int64
	ignoredOff bool // a proceeding capability-off branch zeroes the authored value
}

func evalMySQLResourceGroupPriority(declared bool, authored int64, types []MySQLResourceGroupType, capKnown bool, capable bool) mysqlResourceGroupPriorityOutcome {
	var out mysqlResourceGroupPriorityOutcome
	eff := MySQLResourceGroupEffectivePriority(authored)
	if !declared {
		eff = 0
	}
	capBranches := []bool{true, false}
	if capKnown {
		capBranches = []bool{capable}
	}
	effs := map[int64]bool{}
	for _, cap := range capBranches {
		if !cap {
			effs[0] = true
			if declared && eff != 0 {
				out.ignoredOff = true
			}
			continue
		}
		for _, groupType := range types {
			lo, hi := int64(-20), int64(0)
			if groupType == MySQLResourceGroupTypeUser {
				lo, hi = 0, 19
			}
			if eff < lo || eff > hi {
				out.anyRefuse = true
				continue
			}
			effs[eff] = true
		}
	}
	out.refuseAll = out.anyRefuse && len(effs) == 0
	if len(effs) == 1 {
		for v := range effs {
			value := v
			out.effective = &value
		}
	}
	return out
}

// MySQLResourceGroupVCPUListText renders the effective (cpu_id_t-
// narrowed) spec list in the catalog's bounded VCPU_IDS text shape —
// ranges as `M-N`, singles as `N`, comma-separated in authored order.
func MySQLResourceGroupVCPUListText(specs []MySQLResourceGroupVCPUSpec) string {
	parts := make([]string, 0, len(specs))
	for _, member := range specs {
		lo := MySQLResourceGroupEffectiveVCPU(member.Lo)
		hi := MySQLResourceGroupEffectiveVCPU(member.Hi)
		if lo == hi {
			parts = append(parts, strconv.FormatUint(uint64(lo), 10))
			continue
		}
		parts = append(parts, strconv.FormatUint(uint64(lo), 10)+"-"+strconv.FormatUint(uint64(hi), 10))
	}
	return strings.Join(parts, ",")
}

// MySQLResourceGroupDeclaredVCPUIDs renders the declared VCPU member in
// catalog text, or nil when undeclared.
func MySQLResourceGroupDeclaredVCPUIDs(declaration *MySQLResourceGroupDeclaration) *string {
	specs, ok := declaration.DeclaredVCPU()
	if !ok {
		return nil
	}
	text := MySQLResourceGroupVCPUListText(specs)
	return &text
}

// MySQLResourceGroupStatementOutcome replays upstream's adjudication
// for one lifecycle statement. version is the resolved version fact
// (nil unproven), facts the deployment premise bundle (nil = all
// unproven), pre the ordered pre-state (nil/unproven = state unknown).
func MySQLResourceGroupStatementOutcome(operation DDLOperation, declaration *MySQLResourceGroupDeclaration, version *VersionIdentity, facts *InstanceFacts, pre *MySQLResourceGroupState) MySQLResourceGroupOutcome {
	var out MySQLResourceGroupOutcome
	depends := false
	dep := func(code string, required ...string) {
		depends = true
		out.Gaps = append(out.Gaps, MySQLResourceGroupGap{ReasonCode: code, RequiredFacts: required})
	}
	// gapOnly reports a missing fact that gates only a post-state member,
	// never the statement verdict — every consistent world still lands
	// the same existence answer while the divergent member stays
	// withheld inside Post.
	gapOnly := func(code string, required ...string) {
		out.Gaps = append(out.Gaps, MySQLResourceGroupGap{ReasonCode: code, RequiredFacts: required})
	}
	// Upstream reports the first error and aborts — the reasons list
	// carries only the earliest proven refusal, the check the statement
	// actually reaches in every consistent world.
	refuse := func(reason string) {
		if len(out.Reasons) == 0 {
			out.Reasons = append(out.Reasons, reason)
		}
	}

	// 1. Version premise — the 8.0.3 introduction boundary.
	if supported, proven := MySQLResourceGroupVersionSupported(version); proven && !supported {
		refuse("the resource-group feature requires MySQL 8.0.3 or later and the observed version predates it")
	} else if !proven {
		dep(MySQLResourceGroupGapUnknownVersion, MySQLResourceGroupFactVersion)
	}

	// 2. Deployment feature premise — check_resource_group_support() is
	// separate from the version gate: a deployment may lack the feature
	// even on a supported version.
	if facts != nil && facts.MySQLResourceGroupFeatureKnown && !facts.MySQLResourceGroupFeatureSupported {
		refuse("this deployment does not support resource groups")
	} else if facts == nil || !facts.MySQLResourceGroupFeatureKnown {
		dep(MySQLResourceGroupGapUnknownFeature, MySQLResourceGroupFactFeature)
	}

	// 3. Name bound — ER_TOO_LONG_IDENT is declaration-provable.
	if declaration.DeclaredNameTooLong() {
		refuse("the group name exceeds the 64-byte identifier bound")
	}

	// 4. Privilege premise — RESOURCE_GROUP_ADMIN gates every lifecycle
	// command at execute.
	if facts != nil && facts.MySQLResourceGroupAdminKnown && !facts.MySQLResourceGroupAdmin {
		refuse("the statement requires the RESOURCE_GROUP_ADMIN privilege")
	} else if facts == nil || !facts.MySQLResourceGroupAdminKnown {
		dep(MySQLResourceGroupGapUnknownPrivilege, MySQLResourceGroupFactPrivilege)
	}

	// 5. Reserved builtin pair — an all-branch refusal by name alone:
	// feature-off refuses the statement, feature-on finds the group
	// present and refuses the reserved target.
	if IsMySQLResourceGroupReservedName(declaration.Name) {
		switch operation {
		case DDLOperationCreateResourceGroup:
			refuse("the reserved builtin group always exists — a CREATE can only duplicate it")
		case DDLOperationAlterResourceGroup:
			refuse("the reserved builtin group cannot be altered")
		case DDLOperationDropResourceGroup:
			refuse("the reserved builtin group cannot be dropped")
		}
	}

	// 6. Ordered pre-state existence adjudication. A proven-occupied
	// fact answers existence by itself — a group with bound threads
	// exists by definition — while the definition members stay
	// unproven.
	stateProven := pre != nil && pre.Proven()
	existsKnown, exists := stateProven, stateProven && pre.Exists
	if !existsKnown && pre != nil && pre.Occupied != nil && *pre.Occupied {
		existsKnown, exists = true, true
	}
	if !existsKnown {
		dep(MySQLResourceGroupGapUnknownState, MySQLResourceGroupFactState)
	} else {
		switch operation {
		case DDLOperationCreateResourceGroup:
			if exists {
				refuse("the group already exists — MySQL has no CREATE RESOURCE GROUP IF NOT EXISTS")
			}
		case DDLOperationAlterResourceGroup, DDLOperationDropResourceGroup:
			if !exists {
				refuse("the group does not exist — MySQL has no IF EXISTS guard on this surface")
			} else if stateProven && pre.Reserved {
				refuse("the reserved builtin group cannot be altered or dropped")
			}
		}
	}

	// 7. ALTER FORCE pairing — ER_INVALID_USE_OF_FORCE_OPTION.
	if operation == DDLOperationAlterResourceGroup && declaration.DeclaredForceMisuse() {
		refuse("FORCE is valid only paired with DISABLE")
	}

	// 8. Declared VCPU range — ER_INVALID_VCPU_RANGE is
	// declaration-provable on the effective (cpu_id_t) endpoints.
	if declaration.DeclaredVCPUInverted() {
		refuse("a declared VCPU range is inverted")
	}

	// 9. Host-capacity premise — ER_INVALID_VCPU_ID needs the target
	// host's CPU count; a zero endpoint provably fits any running host.
	if declaration.DeclaredVCPUNeedsHostFact() {
		if facts != nil && facts.MySQLHostVCPUsKnown {
			if declaration.DeclaredVCPUExceeds(facts.MySQLHostVCPUs) {
				refuse("a declared VCPU id is at or past the host's CPU count")
			}
		} else {
			dep(MySQLResourceGroupGapUnknownHostVCPU, MySQLResourceGroupFactHostVCPU)
		}
	}

	// 10. Declared priority — capability- and type-bound. CREATE uses
	// the authored TYPE; ALTER uses the existing group's proven type.
	var prio mysqlResourceGroupPriorityOutcome
	prioDeclared := false
	if operation == DDLOperationCreateResourceGroup || operation == DDLOperationAlterResourceGroup {
		if value, ok := declaration.DeclaredThreadPriority(); ok && value != nil {
			prioDeclared = true
			types := mysqlResourceGroupPriorityTypes(operation, declaration, pre)
			capKnown := facts != nil && facts.MySQLResourceGroupPriorityCapKnown
			capable := capKnown && facts.MySQLResourceGroupPriorityCapable
			prio = evalMySQLResourceGroupPriority(true, *value, types, capKnown, capable)
			if prio.refuseAll {
				refuse("the declared THREAD_PRIORITY sits outside the group type's permitted range")
			} else {
				if prio.anyRefuse {
					depends = true
				}
				if !capKnown && MySQLResourceGroupEffectivePriority(*value) != 0 {
					// Every capability branch lands the same existence
					// answer when none refuses — the gap withholds only
					// the effective-priority member.
					if prio.anyRefuse {
						dep(MySQLResourceGroupGapUnknownPriorityCap, MySQLResourceGroupFactPriorityCap)
					} else {
						gapOnly(MySQLResourceGroupGapUnknownPriorityCap, MySQLResourceGroupFactPriorityCap)
					}
				}
				// The catalog type gates only refusal — a proceeding
				// type branch stores the same authored value, so the
				// gap matters only when some type branch refuses.
				if len(types) > 1 && prio.anyRefuse && operation == DDLOperationAlterResourceGroup &&
					(stateProven || existsKnown && exists) {
					dep(MySQLResourceGroupGapUnknownType, MySQLResourceGroupFactType)
				}
			}
		}
	}

	// 11. DROP occupancy — ER_RESOURCE_GROUP_BUSY without FORCE.
	// Occupancy is an independent fact (never derived from the state
	// answer): a proven-occupied target refuses regardless — absent
	// refuses anyway, present refuses busy — while FORCE proceeds and
	// relocates to the default groups.
	occupiedKnown, occupied := false, false
	if pre != nil && pre.Occupied != nil {
		occupiedKnown, occupied = true, *pre.Occupied
	}
	if operation == DDLOperationDropResourceGroup && !declaration.DeclaredForce() {
		if occupiedKnown && occupied {
			refuse("threads are bound to the group — DROP without FORCE is refused")
		} else if !occupiedKnown {
			dep(MySQLResourceGroupGapUnknownOccupancy, MySQLResourceGroupFactOccupancy)
		}
	}

	out.Refused = len(out.Reasons) > 0
	out.Undecidable = !out.Refused && depends
	if out.Refused {
		out.Gaps = nil
		return out
	}
	out.Post = mysqlResourceGroupPostState(operation, declaration, pre, prioDeclared, prio, occupiedKnown, occupied)
	return out
}

// mysqlResourceGroupPriorityTypes returns the group-type candidates the
// priority check evaluates: CREATE resolves to the authored TYPE; ALTER
// resolves to the existing group's proven type and evaluates both when
// it is unproven.
func mysqlResourceGroupPriorityTypes(operation DDLOperation, declaration *MySQLResourceGroupDeclaration, pre *MySQLResourceGroupState) []MySQLResourceGroupType {
	if operation == DDLOperationCreateResourceGroup {
		if groupType, ok := declaration.DeclaredType(); ok && groupType != "" {
			return []MySQLResourceGroupType{groupType}
		}
	}
	if operation == DDLOperationAlterResourceGroup && pre != nil && pre.Proven() && pre.Exists &&
		pre.Definition != nil && pre.Definition.GroupType != "" {
		return []MySQLResourceGroupType{pre.Definition.GroupType}
	}
	return []MySQLResourceGroupType{MySQLResourceGroupTypeUser, MySQLResourceGroupTypeSystem}
}

// mysqlResourceGroupPostState computes the proceeding-branch post
// state: CREATE publishes the full authored definition (the create
// specifies every member), ALTER merges onto the proven definition
// (undeclared VCPU/enabled preserve, an undeclared priority resets to
// zero exactly as upstream's parse-tree default does), and DROP clears
// the identity. Divergent members stay withheld (nil).
func mysqlResourceGroupPostState(operation DDLOperation, declaration *MySQLResourceGroupDeclaration, pre *MySQLResourceGroupState, prioDeclared bool, prio mysqlResourceGroupPriorityOutcome, occupiedKnown, occupied bool) *MySQLResourceGroupPost {
	switch operation {
	case DDLOperationCreateResourceGroup:
		post := &MySQLResourceGroupPost{
			Exists: true,
			Definition: &MySQLResourceGroupDefinition{
				VCPUIDs: MySQLResourceGroupDeclaredVCPUIDs(declaration),
			},
		}
		if groupType, ok := declaration.DeclaredType(); ok {
			post.Definition.GroupType = groupType
		}
		if prioDeclared {
			post.Definition.ThreadPriority = prio.effective
		} else {
			zero := int64(0)
			post.Definition.ThreadPriority = &zero
		}
		enabled := true
		if value, ok := declaration.DeclaredEnabled(); ok && value != nil {
			enabled = *value
		}
		post.Definition.Enabled = &enabled
		unoccupied := false
		post.Occupied = &unoccupied
		return post
	case DDLOperationAlterResourceGroup:
		if pre == nil || !pre.Proven() || !pre.Exists {
			return &MySQLResourceGroupPost{Exists: true}
		}
		definition := pre.Definition.Clone()
		if definition == nil {
			definition = &MySQLResourceGroupDefinition{}
		}
		if ids := MySQLResourceGroupDeclaredVCPUIDs(declaration); ids != nil {
			definition.VCPUIDs = ids
		}
		// Upstream's ALTER parse tree defaults priority to zero even when
		// undeclared — an ALTER without THREAD_PRIORITY resets the stored
		// value, never preserves it.
		if prioDeclared {
			definition.ThreadPriority = prio.effective
		} else {
			zero := int64(0)
			definition.ThreadPriority = &zero
		}
		if value, ok := declaration.DeclaredEnabled(); ok && value != nil {
			enabled := *value
			definition.Enabled = &enabled
		}
		post := &MySQLResourceGroupPost{Exists: true, Definition: definition}
		post.Occupied = pre.Occupied
		// DISABLE FORCE relocates bound threads to the default groups —
		// the group provably lands unoccupied.
		if enabled, ok := declaration.DeclaredEnabled(); ok && enabled != nil && !*enabled && declaration.DeclaredForce() {
			unoccupied := false
			post.Occupied = &unoccupied
		}
		return post
	case DDLOperationDropResourceGroup:
		return &MySQLResourceGroupPost{Exists: false}
	}
	return nil
}

// MySQLResourceGroupPostMerge unions a pre-state with the proceeding
// branches' post-state for an undecidable outcome — a refusal branch
// keeps the pre-state, so every member provable only in one branch is
// withheld. The returned state keeps whatever the union still proves.
func MySQLResourceGroupPostMerge(pre *MySQLResourceGroupState, post *MySQLResourceGroupPost) *MySQLResourceGroupState {
	out := &MySQLResourceGroupState{}
	if pre != nil {
		out.Name = pre.Name
		out.Reserved = pre.Reserved
	}
	if post == nil {
		if pre != nil {
			out.Status = pre.Status
			out.Exists = pre.Exists
			out.Definition = pre.Definition.Clone()
			if pre.Occupied != nil {
				v := *pre.Occupied
				out.Occupied = &v
			}
		}
		return out
	}
	if pre != nil && pre.Proven() && pre.Exists == post.Exists {
		out.Status = pre.Status
		out.Exists = pre.Exists
		if mysqlResourceGroupDefinitionEqual(pre.Definition, post.Definition) {
			out.Definition = pre.Definition.Clone()
		}
	} else {
		out.Status = MetadataStatusUnavailable
	}
	// Occupancy unions independently of the existence answer: a refusal
	// branch keeps the pre-state binding, a proceeding branch publishes
	// its own — equal outcomes stay proven either way.
	if pre != nil && pre.Occupied != nil && post.Occupied != nil && *pre.Occupied == *post.Occupied {
		v := *pre.Occupied
		out.Occupied = &v
	}
	return out
}

// mysqlResourceGroupDefinitionEqual compares two definitions member by
// member — nil members on both sides count equal (both unproven).
func mysqlResourceGroupDefinitionEqual(a, b *MySQLResourceGroupDefinition) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.GroupType == b.GroupType &&
		mysqlResourceGroupBoolPtrEqual(a.Enabled, b.Enabled) &&
		mysqlResourceGroupInt64PtrEqual(a.ThreadPriority, b.ThreadPriority) &&
		mysqlResourceGroupStringPtrEqual(a.VCPUIDs, b.VCPUIDs)
}

func mysqlResourceGroupBoolPtrEqual(a, b *bool) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func mysqlResourceGroupInt64PtrEqual(a, b *int64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func mysqlResourceGroupStringPtrEqual(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}
