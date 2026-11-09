package audit

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// batch_state_t24_test.go pins the T24/#103-R2 MySQL resource-group
// contract: recognition of the MySQL CPU family (never the TiDB RU
// declaration), the 8.0.3 introduction boundary, the deployment
// feature/thread-priority-capability/host-VCPU/privilege/occupancy
// premise facts as explicit known/unknown evidence, reserved groups,
// existence adjudication, inverted-VCPU declaration refusals, FORCE
// pairing and occupancy, authored-vs-effective priority, and the
// complete-premise first path — all through the public AuditSQL path
// plus the internal ordered-state seam where member-level withholding
// is observable.

type t24GroupFact struct {
	status     spec.MetadataStatus
	exists     bool
	definition *spec.MySQLResourceGroupDefinition
	err        error
}

type t24Provider struct {
	version string
	facts   *spec.InstanceFacts
	groups  map[string]t24GroupFact
	// loadedFacts/loadErr model the optional T24 premise-bundle loader
	// channel — nil means "loader answers nothing" and the merge is a
	// no-op, so existing fixtures keep their behavior.
	loadedFacts *spec.InstanceFacts
	loadErr     error
}

func (p *t24Provider) LoadInstanceFacts(context.Context, spec.Dialect, string) (*spec.InstanceFacts, error) {
	if p.facts != nil {
		facts := *p.facts
		return &facts, nil
	}
	return &spec.InstanceFacts{Version: p.version}, nil
}

func (p *t24Provider) LoadTableSnapshot(context.Context, spec.Dialect, string, string) (*spec.TableSnapshot, error) {
	return nil, nil
}

// LoadMySQLResourceGroupFacts implements the optional T24 premise-bundle
// loader — the channel the real provider wires for read-only variables,
// grants, occupancy, and the feature probe.
func (p *t24Provider) LoadMySQLResourceGroupFacts(context.Context, spec.Dialect) (*spec.InstanceFacts, error) {
	if p.loadErr != nil {
		return nil, p.loadErr
	}
	return p.loadedFacts, nil
}

func (p *t24Provider) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if req.Type != "mysql_resource_group" {
		return &spec.ObjectSnapshot{Type: req.Type, Name: req.Name, Status: spec.MetadataStatusUnavailable}, nil
	}
	key := strings.ToLower(strings.TrimSpace(req.Name))
	fact, ok := p.groups[key]
	if !ok {
		return &spec.ObjectSnapshot{Type: req.Type, Name: req.Name, Status: spec.MetadataStatusUnavailable}, nil
	}
	if fact.err != nil {
		return nil, fact.err
	}
	return &spec.ObjectSnapshot{
		Type:               req.Type,
		Name:               req.Name,
		Status:             fact.status,
		Exists:             fact.exists,
		MySQLResourceGroup: fact.definition,
	}, nil
}

const (
	t24ValidRule  = "ddl.mysql.resource_group.valid"
	t24ForceWarn  = "ddl.mysql.resource_group.force.warn"
	t24AffWarn    = "ddl.mysql.resource_group.affinity.warn"
	t24PrioWarn   = "ddl.mysql.resource_group.priority.warn"
	t24StateGap   = "unknown_resource_group_state"
	t24VersionGap = "unknown_resource_group_version"
	t24FeatureGap = "unknown_resource_group_feature"
	t24HostGap    = "unknown_host_vcpu_count"
	t24PrivGap    = "unknown_resource_group_privilege"
	t24CapGap     = "unknown_resource_group_priority_capability"
	t24TypeGap    = "unknown_resource_group_type"
	t24OccGap     = "unknown_resource_group_occupancy"
	t24ROGap      = "unknown_instance_read_only"
	t24SROGap     = "unknown_instance_super_read_only"
	t24ROExGap    = "unknown_resource_group_readonly_exemption"
	t24CreateNote = "ddl.create_resource_group.notice"
	t24DropNote   = "ddl.drop_resource_group.notice"
)

// t24Facts returns the complete-premise fact bundle — every deployment
// premise the shared outcome consults, proven and permissive:
// feature supported, priority capability available, 8 host CPUs,
// RESOURCE_GROUP_ADMIN granted, no bound threads observed, and a
// writable instance (read_only=OFF answers the gate for every world,
// so super_read_only and the exemption need no facts).
func t24Facts(version string) *spec.InstanceFacts {
	return &spec.InstanceFacts{
		Version:                            version,
		MySQLResourceGroupFeatureKnown:     true,
		MySQLResourceGroupFeatureSupported: true,
		MySQLResourceGroupPriorityCapKnown: true,
		MySQLResourceGroupPriorityCapable:  true,
		MySQLHostVCPUsKnown:                true,
		MySQLHostVCPUs:                     8,
		MySQLResourceGroupAdminKnown:       true,
		MySQLResourceGroupAdmin:            true,
		MySQLResourceGroupBound:            map[string]bool{},
		MySQLReadOnlyKnown:                 true,
		MySQLReadOnly:                      false,
		MySQLSuperReadOnlyKnown:            true,
		MySQLSuperReadOnly:                 false,
	}
}

func t24Groups(m map[string]t24GroupFact) *t24Provider {
	return &t24Provider{version: "8.0.36", facts: t24Facts("8.0.36"), groups: m}
}

// t24AuditRaw mirrors the t22 helper but reports the provider's own
// version as the audit target — the observed/target consistency check
// needs a matching pair under MySQL.
func t24AuditRaw(t *testing.T, sql string, provider *t24Provider) (report.Result, error) {
	t.Helper()
	target := ""
	if provider != nil {
		target = provider.version
	}
	return AuditSQL(context.Background(), Request{
		SQL:              sql,
		Dialect:          spec.DialectMySQL,
		TargetVersion:    target,
		MetadataProvider: provider,
	})
}

func t24MustAudit(t *testing.T, sql string, provider *t24Provider) report.Result {
	t.Helper()
	result, err := t24AuditRaw(t, sql, provider)
	if err != nil {
		t.Fatalf("AuditSQL(%q): %v", sql, err)
	}
	return result
}

// t24AuditEvidence runs the public Audit path with a DeploymentEvidence
// bundle — the formal seam for premises no SQL channel observes.
func t24AuditEvidence(t *testing.T, sql string, provider *t24Provider, evidence *spec.DeploymentEvidence) report.Result {
	t.Helper()
	target := ""
	if provider != nil {
		target = provider.version
	}
	result, err := AuditSQL(context.Background(), Request{
		SQL:                sql,
		Dialect:            spec.DialectMySQL,
		TargetVersion:      target,
		MetadataProvider:   provider,
		DeploymentEvidence: evidence,
	})
	if err != nil {
		t.Fatalf("AuditSQL(%q): %v", sql, err)
	}
	return result
}

// t24Entry runs a batch through the internal ordered-state seam and
// returns the owned registry entry for key — the member-level post
// state (withheld members, occupancy) observable only inside the
// registry, never on the public projection.
func t24Entry(t *testing.T, sql, key string, provider *t24Provider) *batchMySQLResourceGroupEntry {
	t.Helper()
	ctx := context.Background()
	parsed, err := Parse(ctx, sql, spec.DialectMySQL)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	statements, err := Extract(ctx, parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	facts, err := provider.LoadInstanceFacts(ctx, spec.DialectMySQL, "")
	if err != nil {
		t.Fatalf("load instance facts: %v", err)
	}
	state := newBatchState(spec.DialectMySQL, "", provider)
	state.resolvedVersion = spec.ParseObservedVersion(facts.Version)
	state.instanceFacts = facts
	if _, err := state.enrichStatements(ctx,
		&MetadataRequest{Schema: "", Provider: provider, TargetVersion: state.resolvedVersion},
		statements, parsed.failures, facts, auditLimits{orderedStatements: len(statements)}); err != nil {
		t.Fatalf("enrich: %v", err)
	}
	return state.mysqlResourceGroupEntries[resourceGroupKey(key)]
}

// ── B1: inverted VCPU range — declaration claim + semantic refusal ──

// An inverted authored range is a declaration fact, never a parse
// diagnostic: upstream's validate_vcpu_range_vector runs
// ER_INVALID_VCPU_RANGE inside execute, so the adapter claims the
// statement and the rule refuses it — before any host-capacity fact is
// consulted, and on every consistent world.
func TestT24InvertedVCPURangeRefusesSemantically(t *testing.T) {
	result := t24MustAudit(t, "CREATE RESOURCE GROUP rg TYPE=USER VCPU=5-3;", t24Groups(map[string]t24GroupFact{
		"rg": {status: spec.MetadataStatusNotFound, exists: false},
	}))
	stmt := result.Statements[0]
	if stmt.Kind != string(spec.KindDDL) {
		t.Fatalf("kind = %q, want ddl — the inverted range is a claimed declaration", stmt.Kind)
	}
	findings := t22Findings(result, 0, t24ValidRule)
	if len(findings) != 1 || !strings.Contains(findings[0].Message, "inverted") {
		t.Fatalf("findings = %+v, want one inverted-range refusal", findings)
	}
	if len(stmt.EvidenceGaps) != 0 {
		t.Fatalf("gaps = %v, want none — a declaration refusal needs no facts", stmt.EvidenceGaps)
	}
}

// The host-CPU fact is independent of the range inversion: an inverted
// range refuses even under a proven huge host count, and the refusal
// never degrades to a host-count gap.
func TestT24InvertedVCPUIgnoresHostFacts(t *testing.T) {
	facts := t24Facts("8.0.36")
	facts.MySQLHostVCPUs = 1024
	result := t24MustAudit(t, "CREATE RESOURCE GROUP rg TYPE=USER VCPU=9-4;", &t24Provider{
		version: "8.0.36", facts: facts,
		groups: map[string]t24GroupFact{"rg": {status: spec.MetadataStatusNotFound}},
	})
	if got := len(t22Findings(result, 0, t24ValidRule)); got != 1 {
		t.Fatalf("findings = %d, want the inverted-range refusal under any host count", got)
	}
	if gaps := t22GapCodes(result.Statements[0]); len(gaps) != 0 {
		t.Fatalf("gaps = %v, want none", gaps)
	}
}

// Ordered controls: M<N proceeds, M=N is a legal single-CPU range, a
// mixed list claims every member, and a valid range under an unproven
// host count owes the host gap instead of a refusal.
func TestT24VCPUOrderedControls(t *testing.T) {
	result := t24MustAudit(t, "CREATE RESOURCE GROUP a TYPE=USER VCPU=3-5;\n"+
		"CREATE RESOURCE GROUP b TYPE=USER VCPU=3-3;\n"+
		"CREATE RESOURCE GROUP c TYPE=USER VCPU=0-3,5;\n"+
		"CREATE RESOURCE GROUP d TYPE=USER VCPU=0 1;",
		t24Groups(map[string]t24GroupFact{}))
	for i := range result.Statements {
		if result.Statements[i].Kind != string(spec.KindDDL) {
			t.Fatalf("statement %d kind = %q, want ddl — valid members claim", i, result.Statements[i].Kind)
		}
		if got := len(t22Findings(result, i, t24ValidRule)); got != 0 {
			t.Fatalf("statement %d findings = %d, want none for valid ranges", i, got)
		}
	}
}

// A valid range under an unproven host count owes the bounded host gap;
// under a proven count an endpoint at or past it refuses
// (ER_INVALID_VCPU_ID); a zero endpoint provably fits every running host
// and never owes the fact.
func TestT24VCPUEndpointHostFact(t *testing.T) {
	noFacts := &t24Provider{version: "8.0.36", groups: map[string]t24GroupFact{}}
	result := t24MustAudit(t, "CREATE RESOURCE GROUP rg TYPE=USER VCPU=0-9;", noFacts)
	gaps := t22GapCodes(result.Statements[0])
	if !slices.Contains(gaps, t24HostGap) {
		t.Fatalf("gaps = %v, want %s for an unproven host count", gaps, t24HostGap)
	}
	if got := len(t22Findings(result, 0, t24ValidRule)); got != 0 {
		t.Fatalf("findings = %d, want none — unproven facts never refuse", got)
	}

	result = t24MustAudit(t, "CREATE RESOURCE GROUP rg TYPE=USER VCPU=0-8;", t24Groups(map[string]t24GroupFact{}))
	findings := t22Findings(result, 0, t24ValidRule)
	if len(findings) != 1 || !strings.Contains(findings[0].Message, "CPU count") {
		t.Fatalf("findings = %+v, want the endpoint refusal", findings)
	}

	// VCPU=0 is a zero endpoint — provably inside any running host's
	// range, so it never owes the host fact even unproven.
	result = t24MustAudit(t, "CREATE RESOURCE GROUP rg TYPE=USER VCPU=0;", noFacts)
	if gaps := t22GapCodes(result.Statements[0]); slices.Contains(gaps, t24HostGap) {
		t.Fatalf("gaps = %v, want no host gap for a zero endpoint", gaps)
	}
}

// ── B2: priority capability — authored vs effective, per-type bounds ──

// The thread-priority capability is a deployment fact distinct from
// resource-group support: on → type-bound validation; off → authored
// nonzero ignored with the effective value stored as zero; unknown →
// the divergence owes the capability gap and the member stays withheld.
func TestT24PriorityCapabilityMatrix(t *testing.T) {
	absent := map[string]t24GroupFact{"rg": {status: spec.MetadataStatusNotFound}}

	// Capability available: bounds adjudicate per authored TYPE.
	facts := t24Facts("8.0.36")
	result := t24MustAudit(t, "CREATE RESOURCE GROUP rg TYPE=USER THREAD_PRIORITY=20;", &t24Provider{
		version: "8.0.36", facts: facts, groups: absent})
	if got := len(t22Findings(result, 0, t24ValidRule)); got != 1 {
		t.Fatalf("user+20 under cap-on: findings = %d, want refusal", got)
	}
	result = t24MustAudit(t, "CREATE RESOURCE GROUP rg TYPE=USER THREAD_PRIORITY=19;", &t24Provider{
		version: "8.0.36", facts: facts, groups: absent})
	if got := len(t22Findings(result, 0, t24ValidRule)); got != 0 {
		t.Fatalf("user+19 under cap-on: findings = %d, want none", got)
	}
	result = t24MustAudit(t, "CREATE RESOURCE GROUP rg TYPE=SYSTEM THREAD_PRIORITY=-20;", &t24Provider{
		version: "8.0.36", facts: facts, groups: absent})
	if got := len(t22Findings(result, 0, t24ValidRule)); got != 0 {
		t.Fatalf("system-20 under cap-on: findings = %d, want none", got)
	}
	result = t24MustAudit(t, "CREATE RESOURCE GROUP rg TYPE=SYSTEM THREAD_PRIORITY=-21;", &t24Provider{
		version: "8.0.36", facts: facts, groups: absent})
	if got := len(t22Findings(result, 0, t24ValidRule)); got != 1 {
		t.Fatalf("system-21 under cap-on: findings = %d, want refusal", got)
	}
	result = t24MustAudit(t, "CREATE RESOURCE GROUP rg TYPE=SYSTEM THREAD_PRIORITY=1;", &t24Provider{
		version: "8.0.36", facts: facts, groups: absent})
	if got := len(t22Findings(result, 0, t24ValidRule)); got != 1 {
		t.Fatalf("system+1 under cap-on: findings = %d, want refusal", got)
	}
}

// Capability off: an authored nonzero priority is ignored — the
// statement proceeds, the ignored-attribute warning fires, and the
// stored effective priority is zero (upstream ER_ATTRIBUTE_IGNORED).
func TestT24PriorityCapabilityOffIgnoresAuthored(t *testing.T) {
	facts := t24Facts("8.0.36")
	facts.MySQLResourceGroupPriorityCapable = false
	provider := &t24Provider{version: "8.0.36", facts: facts,
		groups: map[string]t24GroupFact{"rg": {status: spec.MetadataStatusNotFound}}}
	result := t24MustAudit(t, "CREATE RESOURCE GROUP rg TYPE=USER THREAD_PRIORITY=-5;", provider)
	if got := len(t22Findings(result, 0, t24ValidRule)); got != 0 {
		t.Fatalf("findings = %d, want none — cap-off never range-refuses", got)
	}
	if got := len(t22Findings(result, 0, t24PrioWarn)); got != 1 {
		t.Fatalf("priority-ignored warns = %d, want 1", got)
	}
	entry := t24Entry(t, "CREATE RESOURCE GROUP rg TYPE=USER THREAD_PRIORITY=-5;", "rg", provider)
	if entry == nil || entry.state != resourceGroupPresent || entry.definition == nil ||
		entry.definition.ThreadPriority == nil || *entry.definition.ThreadPriority != 0 {
		t.Fatalf("entry = %+v, want present with effective priority 0", entry)
	}
}

// Capability unknown: branches diverge (cap-on may refuse, cap-off
// zeroes) — the outcome stays undecidable, the capability gap is
// published, and the post-state member is withheld rather than guessed.
func TestT24PriorityCapabilityUnknownOwesGap(t *testing.T) {
	facts := t24Facts("8.0.36")
	facts.MySQLResourceGroupPriorityCapKnown = false
	provider := &t24Provider{version: "8.0.36", facts: facts,
		groups: map[string]t24GroupFact{"rg": {status: spec.MetadataStatusNotFound}}}
	result := t24MustAudit(t, "CREATE RESOURCE GROUP rg TYPE=USER THREAD_PRIORITY=5;", provider)
	gaps := t22GapCodes(result.Statements[0])
	if !slices.Contains(gaps, t24CapGap) {
		t.Fatalf("gaps = %v, want %s", gaps, t24CapGap)
	}
	if got := len(t22Findings(result, 0, t24ValidRule)); got != 0 {
		t.Fatalf("findings = %d, want none — undecidable never fabricates a refusal", got)
	}
	entry := t24Entry(t, "CREATE RESOURCE GROUP rg TYPE=USER THREAD_PRIORITY=5;", "rg", provider)
	// Every capability branch lands the group — only the effective
	// priority diverges (authored 5 vs ignored 0), so existence is
	// proven while the member stays withheld.
	if entry == nil || entry.state != resourceGroupPresent || entry.definition == nil ||
		entry.definition.ThreadPriority != nil {
		t.Fatalf("entry = %+v, want present with withheld ThreadPriority member", entry)
	}
}

// A zero authored priority needs no capability fact: every branch
// stores zero and no branch refuses.
func TestT24PriorityZeroNeedsNoCapability(t *testing.T) {
	facts := t24Facts("8.0.36")
	facts.MySQLResourceGroupPriorityCapKnown = false
	provider := &t24Provider{version: "8.0.36", facts: facts,
		groups: map[string]t24GroupFact{"rg": {status: spec.MetadataStatusNotFound}}}
	result := t24MustAudit(t, "CREATE RESOURCE GROUP rg TYPE=USER THREAD_PRIORITY=0;", provider)
	if gaps := t22GapCodes(result.Statements[0]); slices.Contains(gaps, t24CapGap) {
		t.Fatalf("gaps = %v, want no capability gap for an authored zero", gaps)
	}
}

// ALTER validates the declared priority against the EXISTING group's
// proven type — never the authored or a fabricated type.
func TestT24AlterPriorityUsesCatalogType(t *testing.T) {
	facts := t24Facts("8.0.36")
	systemDef := &spec.MySQLResourceGroupDefinition{GroupType: spec.MySQLResourceGroupTypeSystem}
	provider := &t24Provider{version: "8.0.36", facts: facts, groups: map[string]t24GroupFact{
		"sysg": {status: spec.MetadataStatusConfirmed, exists: true, definition: systemDef},
	}}
	result := t24MustAudit(t, "ALTER RESOURCE GROUP sysg THREAD_PRIORITY=5;", provider)
	if got := len(t22Findings(result, 0, t24ValidRule)); got != 1 {
		t.Fatalf("alter+5 on a SYSTEM group: findings = %d, want refusal — catalog type adjudicates", got)
	}
	result = t24MustAudit(t, "ALTER RESOURCE GROUP sysg THREAD_PRIORITY=-5;", provider)
	if got := len(t22Findings(result, 0, t24ValidRule)); got != 0 {
		t.Fatalf("alter-5 on a SYSTEM group: findings = %d, want none", got)
	}
}

// An ALTER on a present group whose catalog type is unproven owes the
// type gap when a declared nonzero priority diverges between candidate
// types — never a guessed type.
func TestT24AlterPriorityUnknownCatalogType(t *testing.T) {
	facts := t24Facts("8.0.36")
	provider := &t24Provider{version: "8.0.36", facts: facts, groups: map[string]t24GroupFact{
		"rg": {status: spec.MetadataStatusConfirmed, exists: true,
			definition: &spec.MySQLResourceGroupDefinition{}},
	}}
	result := t24MustAudit(t, "ALTER RESOURCE GROUP rg THREAD_PRIORITY=-5;", provider)
	gaps := t22GapCodes(result.Statements[0])
	if !slices.Contains(gaps, t24TypeGap) {
		t.Fatalf("gaps = %v, want %s for an unproven catalog type", gaps, t24TypeGap)
	}
}

// ALTER's undeclared priority resets the stored value to zero exactly
// as upstream's parse-tree default does — never preserves the prior
// priority.
func TestT24BareAlterResetsPriority(t *testing.T) {
	provider := t24Groups(map[string]t24GroupFact{
		"rg": {status: spec.MetadataStatusNotFound},
	})
	entry := t24Entry(t, "CREATE RESOURCE GROUP rg TYPE=USER THREAD_PRIORITY=10;\nALTER RESOURCE GROUP rg ENABLE;", "rg", provider)
	if entry == nil || entry.definition == nil || entry.definition.ThreadPriority == nil ||
		*entry.definition.ThreadPriority != 0 {
		t.Fatalf("entry = %+v, want effective priority reset to 0 by the bare alter", entry)
	}
}

// ── B3: 8.0.3 boundary, dialect forms, name grammar ──

// 8.0.3 is the precise introduction boundary — 8.0.2 and 5.7 refuse,
// 8.0.3 proceeds.
func TestT24VersionBoundary803(t *testing.T) {
	for _, tc := range []struct {
		version string
		refused bool
	}{
		{"5.7.44", true}, {"8.0.0", true}, {"8.0.2", true},
		{"8.0.3", false}, {"8.0.46", false}, {"8.4.10", false},
	} {
		facts := t24Facts(tc.version)
		provider := &t24Provider{version: tc.version, facts: facts,
			groups: map[string]t24GroupFact{"rg": {status: spec.MetadataStatusNotFound}}}
		result := t24MustAudit(t, "CREATE RESOURCE GROUP rg TYPE=USER;", provider)
		got := len(t22Findings(result, 0, t24ValidRule))
		if tc.refused && got != 1 {
			t.Fatalf("version %s: findings = %d, want one refusal", tc.version, got)
		}
		if !tc.refused && got != 0 {
			t.Fatalf("version %s: findings = %d, want none", tc.version, got)
		}
	}
}

// A proven pre-8.0.3 version refuses before the catalog — no resolver
// reads, no premise gaps.
func TestT24VersionPremiseRefusesPre803(t *testing.T) {
	provider := &t24Provider{version: "5.7.44", groups: map[string]t24GroupFact{
		"usr_default": {status: spec.MetadataStatusConfirmed, exists: true},
	}}
	result := t24MustAudit(t, "CREATE RESOURCE GROUP rg TYPE=USER;\nDROP RESOURCE GROUP usr_default;",
		provider)
	for i := range result.Statements {
		findings := t22Findings(result, i, t24ValidRule)
		if len(findings) != 1 || !strings.Contains(findings[0].Message, "8.0.3") {
			t.Fatalf("statement %d findings = %+v, want one version refusal", i, findings)
		}
		if len(result.Statements[i].EvidenceGaps) != 0 {
			t.Fatalf("statement %d gaps = %v, want none under a proven refusal", i, result.Statements[i].EvidenceGaps)
		}
	}
}

// A proven-off deployment feature refuses on its own layer — separate
// from version support — while an unproven one owes the feature gap.
func TestT24FeaturePremise(t *testing.T) {
	facts := t24Facts("8.0.36")
	facts.MySQLResourceGroupFeatureSupported = false
	result := t24MustAudit(t, "CREATE RESOURCE GROUP rg TYPE=USER;", &t24Provider{
		version: "8.0.36", facts: facts, groups: map[string]t24GroupFact{}})
	findings := t22Findings(result, 0, t24ValidRule)
	if len(findings) != 1 || !strings.Contains(findings[0].Message, "does not support resource groups") {
		t.Fatalf("findings = %+v, want the deployment-feature refusal", findings)
	}

	facts = t24Facts("8.0.36")
	facts.MySQLResourceGroupFeatureKnown = false
	result = t24MustAudit(t, "CREATE RESOURCE GROUP rg TYPE=USER;", &t24Provider{
		version: "8.0.36", facts: facts, groups: map[string]t24GroupFact{}})
	if gaps := t22GapCodes(result.Statements[0]); !slices.Contains(gaps, t24FeatureGap) {
		t.Fatalf("gaps = %v, want %s", gaps, t24FeatureGap)
	}
}

// An unproven version owes the bounded version gap — never a guessed
// feature presence, never a fabricated refusal.
func TestT24UnprovenVersionOwesGap(t *testing.T) {
	facts := t24Facts("")
	result := t24MustAudit(t, "CREATE RESOURCE GROUP rg TYPE=USER;",
		&t24Provider{version: "", facts: facts, groups: map[string]t24GroupFact{}})
	gaps := t22GapCodes(result.Statements[0])
	if !slices.Contains(gaps, t24VersionGap) {
		t.Fatalf("gaps = %v, want %s", gaps, t24VersionGap)
	}
}

// The reserved builtin pair refuses ALTER and DROP by name alone — an
// all-branch refusal that holds even with no other premise proven:
// feature-off refuses the statement, feature-on finds the group
// present and refuses reserved-ness.
func TestT24ReservedGroupsRefuse(t *testing.T) {
	result := t24MustAudit(t, "ALTER RESOURCE GROUP usr_default THREAD_PRIORITY=5;\nDROP RESOURCE GROUP sys_default;",
		&t24Provider{version: "8.0.36", facts: t24Facts("8.0.36"), groups: map[string]t24GroupFact{}})
	for i := range result.Statements {
		findings := t22Findings(result, i, t24ValidRule)
		if len(findings) != 1 || !strings.Contains(strings.ToLower(findings[0].Message), "reserved") {
			t.Fatalf("statement %d findings = %+v, want one reserved refusal", i, findings)
		}
	}
	// Case-insensitive exactly as upstream's my_strcasecmp check.
	result = t24MustAudit(t, "DROP RESOURCE GROUP USR_DEFAULT;",
		&t24Provider{version: "8.0.36", facts: t24Facts("8.0.36"), groups: map[string]t24GroupFact{}})
	if got := len(t22Findings(result, 0, t24ValidRule)); got != 1 {
		t.Fatalf("case-varied reserved drop: findings = %d, want 1", got)
	}
	// Even with every premise unproven, every consistent world refuses.
	result = t24MustAudit(t, "DROP RESOURCE GROUP sys_default;",
		&t24Provider{version: "", groups: map[string]t24GroupFact{}})
	if got := len(t22Findings(result, 0, t24ValidRule)); got != 1 {
		t.Fatalf("unproven-everything reserved drop: findings = %d, want 1", got)
	}
}

// Existence adjudication: proven duplicate CREATE, proven-absent ALTER,
// proven-absent DROP — all refusals, all with the notice suppressed.
func TestT24ExistenceAdjudication(t *testing.T) {
	def := &spec.MySQLResourceGroupDefinition{GroupType: spec.MySQLResourceGroupTypeUser}
	result := t24MustAudit(t, "CREATE RESOURCE GROUP dup TYPE=USER;\nALTER RESOURCE GROUP absent ENABLE;\nDROP RESOURCE GROUP absent;",
		t24Groups(map[string]t24GroupFact{
			"dup":    {status: spec.MetadataStatusConfirmed, exists: true, definition: def},
			"absent": {status: spec.MetadataStatusNotFound, exists: false},
		}))
	for i, want := range []int{1, 1, 1} {
		if got := len(t22Findings(result, i, t24ValidRule)); got != want {
			t.Fatalf("statement %d findings = %d, want %d", i, got, want)
		}
	}
	if got := len(t22Findings(result, 0, t24CreateNote)); got != 0 {
		t.Fatalf("duplicate create notice = %d, want suppressed", got)
	}
	if got := len(t22Findings(result, 2, t24DropNote)); got != 0 {
		t.Fatalf("absent drop notice = %d, want suppressed", got)
	}
}

// Keyword-like names follow the fixed MySQL grammar: ident_keyword
// members (user, resource, type, vcpu, thread_priority, enable,
// disable) are legal unquoted names; reserved words (group, system,
// force, for, select, if, exists, not) need backticks; quoted idents
// keep doubled-backtick escapes.
func TestT24KeywordLikeNames(t *testing.T) {
	for _, name := range []string{"user", "resource", "type", "vcpu", "enable", "disable", "action", "view"} {
		result := t24MustAudit(t, "CREATE RESOURCE GROUP "+name+" TYPE=USER;", t24Groups(map[string]t24GroupFact{}))
		if result.Statements[0].Kind != string(spec.KindDDL) {
			t.Fatalf("unquoted %q: kind = %q, want ddl — ident_keyword names claim", name, result.Statements[0].Kind)
		}
	}
	for _, name := range []string{"group", "system", "force", "for", "select", "if", "exists", "not", "table"} {
		res, err := t24AuditRaw(t, "CREATE RESOURCE GROUP "+name+" TYPE=USER;", t24Groups(map[string]t24GroupFact{}))
		if err == nil {
			t.Fatalf("unquoted reserved %q audited, want parse diagnostic", name)
		}
		if len(res.Diagnostics) == 0 || res.Diagnostics[0].Classification != spec.DiagnosticParserError {
			t.Fatalf("unquoted reserved %q: diagnostics = %+v, want parser_error", name, res.Diagnostics)
		}
	}
	for _, sql := range []string{
		"CREATE RESOURCE GROUP `group` TYPE=USER;",
		"CREATE RESOURCE GROUP `select` TYPE=USER;",
		"CREATE RESOURCE GROUP `a``b` TYPE=USER;",
		"CREATE RESOURCE GROUP `system` TYPE=USER;",
	} {
		result := t24MustAudit(t, sql, t24Groups(map[string]t24GroupFact{}))
		if result.Statements[0].Kind != string(spec.KindDDL) {
			t.Fatalf("%q: kind = %q, want ddl — quoted reserved names claim", sql, result.Statements[0].Kind)
		}
	}
}

// The name bound counts utf8 BYTES (upstream name.length), not
// characters: 64 ASCII bytes pass, 65 refuse; a 21-char CJK name is 63
// bytes and passes, 22 chars is 66 bytes and refuses.
func TestT24NameByteBoundaries(t *testing.T) {
	cases := []struct {
		name    string
		refused bool
	}{
		{strings.Repeat("x", 64), false},
		{strings.Repeat("x", 65), true},
		{strings.Repeat("名", 21), false},       // 63 utf8 bytes
		{strings.Repeat("名", 22), true},        // 66 utf8 bytes
		{strings.Repeat("名", 21) + "ab", true}, // 65 utf8 bytes
	}
	for _, tc := range cases {
		result := t24MustAudit(t, "CREATE RESOURCE GROUP `"+tc.name+"` TYPE=USER;", t24Groups(map[string]t24GroupFact{}))
		got := len(t22Findings(result, 0, t24ValidRule))
		if tc.refused && got == 0 {
			t.Fatalf("name %q (%d bytes): want refusal", tc.name, len(tc.name))
		}
		if !tc.refused && got != 0 {
			t.Fatalf("name %q (%d bytes): findings = %d, want none", tc.name, len(tc.name), got)
		}
	}
}

// Malformed tails and out-of-domain literals keep the base grammar's
// diagnostics — the adapter never makes invalid SQL valid by trimming
// tails or loosening numbers.
func TestT24MalformedOperands(t *testing.T) {
	for _, sql := range []string{
		"CREATE RESOURCE GROUP rg TYPE=USER VCPU=0,",
		"CREATE RESOURCE GROUP rg TYPE=USER VCPU=,1",
		"CREATE RESOURCE GROUP rg TYPE=USER VCPU=-1",
		"CREATE RESOURCE GROUP rg TYPE=USER VCPU=1-",
		"CREATE RESOURCE GROUP rg TYPE=USER VCPU=0-",
		"CREATE RESOURCE GROUP rg TYPE=USER VCPU=x",
		"CREATE RESOURCE GROUP rg TYPE=USER THREAD_PRIORITY=+5",
		"CREATE RESOURCE GROUP rg TYPE=USER THREAD_PRIORITY=5.5",
		"CREATE RESOURCE GROUP rg TYPE=USER THREAD_PRIORITY=x",
		"CREATE RESOURCE GROUP rg TYPE=USER VCPU=18446744073709551616",
		"CREATE RESOURCE GROUP rg TYPE=USER VCPU=0-18446744073709551616",
		"SET RESOURCE GROUP rg FOR 1,",
	} {
		res, err := t24AuditRaw(t, sql, t24Groups(map[string]t24GroupFact{}))
		if err == nil {
			t.Fatalf("%q audited = %+v, want parse diagnostic", sql, res.Statements)
		}
		if len(res.Diagnostics) == 0 || res.Diagnostics[0].Classification != spec.DiagnosticParserError {
			t.Fatalf("%q diagnostics = %+v, want parser_error", sql, res.Diagnostics)
		}
	}
}

// Upstream's resource-group productions accept only the NUM token —
// int_token classifies a zero-stripped decimal literal at or below
// 2147483647 (int32 max) as NUM; larger magnitudes lex as LONG_NUM,
// ULONGLONG_NUM, or DECIMAL_NUM, none of which vcpu_num_or_range or
// signed_num accepts. Out-of-NUM literals are grammar refusals — the
// adapter must never claim them, never saturate or truncate them into
// a declaration. (Corrected oracle: an earlier cell claimed
// VCPU=4294967296 → effective endpoint 0 by cpu_id_t truncation; that
// helper arithmetic was real but the SQL never reaches it — the lexer
// emits LONG_NUM first.)
func TestT24NUMTokenDomain(t *testing.T) {
	// Reachable NUM domain — both boundaries claim.
	for _, sql := range []string{
		"CREATE RESOURCE GROUP rg TYPE=USER VCPU=2147483647;",
		"CREATE RESOURCE GROUP rg TYPE=USER VCPU=0-2147483647;",
		"CREATE RESOURCE GROUP rg TYPE=USER VCPU=0000000000000000000000005;",
		"CREATE RESOURCE GROUP rg TYPE=USER THREAD_PRIORITY=2147483647;",
		"CREATE RESOURCE GROUP rg TYPE=USER THREAD_PRIORITY=-2147483647;",
	} {
		result := t24MustAudit(t, sql, t24Groups(map[string]t24GroupFact{}))
		if result.Statements[0].Kind != string(spec.KindDDL) {
			t.Fatalf("%q: kind = %q, want ddl — NUM-domain literals claim", sql, result.Statements[0].Kind)
		}
	}
	// Beyond NUM — LONG_NUM/ULONGLONG_NUM/DECIMAL_NUM all refuse at the
	// grammar, matching upstream's syntax-error shape: no declaration,
	// no notice, no post-state.
	for _, sql := range []string{
		"CREATE RESOURCE GROUP rg TYPE=USER VCPU=2147483648;",
		"CREATE RESOURCE GROUP rg TYPE=USER VCPU=4294967296;",
		"CREATE RESOURCE GROUP rg TYPE=USER VCPU=4294967296-4294967297;",
		"CREATE RESOURCE GROUP rg TYPE=USER VCPU=4294967296-1;",
		"CREATE RESOURCE GROUP rg TYPE=USER VCPU=18446744073709551615;",
		"CREATE RESOURCE GROUP rg TYPE=USER VCPU=18446744073709551616;",
		"CREATE RESOURCE GROUP rg TYPE=USER THREAD_PRIORITY=2147483648;",
		"CREATE RESOURCE GROUP rg TYPE=USER THREAD_PRIORITY=-2147483648;",
		"CREATE RESOURCE GROUP rg TYPE=USER THREAD_PRIORITY=9223372036854775807;",
		"CREATE RESOURCE GROUP rg TYPE=USER THREAD_PRIORITY=-9223372036854775808;",
		"ALTER RESOURCE GROUP rg VCPU=2147483648;",
	} {
		res, err := t24AuditRaw(t, sql, t24Groups(map[string]t24GroupFact{}))
		if err == nil {
			t.Fatalf("%q audited = %+v, want parse diagnostic — the literal lexes out of NUM", sql, res.Statements)
		}
		if len(res.Diagnostics) == 0 || res.Diagnostics[0].Classification != spec.DiagnosticParserError {
			t.Fatalf("%q diagnostics = %+v, want parser_error", sql, res.Diagnostics)
		}
	}
}

// ── B3.5: ordinary-client read-only gate (T24-R4) ──
//
// Upstream check_readonly() opens every lifecycle execute: an ordinary
// client refuses iff (read_only || super_read_only) && (super_read_only
// || !exempt), where the exemption is SUPER_ACL/CONNECTION_ADMIN —
// never RESOURCE_GROUP_ADMIN. The matrix pins each reachable arm plus
// the gap-precision controls: a fact only owed when it flips a world's
// outcome.
func TestT24ReadOnlyGateMatrix(t *testing.T) {
	base := func(apply func(*spec.InstanceFacts)) *t24Provider {
		facts := t24Facts("8.0.36")
		facts.MySQLReadOnlyKnown = false
		facts.MySQLSuperReadOnlyKnown = false
		facts.MySQLReadOnlyExemptKnown = false
		apply(facts)
		return &t24Provider{version: "8.0.36", facts: facts, groups: map[string]t24GroupFact{
			"rg": {status: spec.MetadataStatusNotFound},
		}}
	}
	// 1. Writable instance — ro/sro both OFF answers the gate; the
	// exemption is never owed.
	result := t24MustAudit(t, "CREATE RESOURCE GROUP rg TYPE=USER;", base(func(f *spec.InstanceFacts) {
		f.MySQLReadOnlyKnown, f.MySQLReadOnly = true, false
		f.MySQLSuperReadOnlyKnown, f.MySQLSuperReadOnly = true, false
	}))
	if got := len(t22Findings(result, 0, t24ValidRule)); got != 0 {
		t.Fatalf("writable create findings = %d, want none", got)
	}
	if gaps := t22GapCodes(result.Statements[0]); slices.Contains(gaps, t24ROExGap) {
		t.Fatalf("writable create gaps = %v, want no exemption gap", gaps)
	}
	// 2. read_only ON + proven no exemption → refused.
	result = t24MustAudit(t, "CREATE RESOURCE GROUP rg TYPE=USER;", base(func(f *spec.InstanceFacts) {
		f.MySQLReadOnlyKnown, f.MySQLReadOnly = true, true
		f.MySQLSuperReadOnlyKnown, f.MySQLSuperReadOnly = true, false
		f.MySQLReadOnlyExemptKnown, f.MySQLReadOnlyExempt = true, false
	}))
	if got := len(t22Findings(result, 0, t24ValidRule)); got != 1 {
		t.Fatalf("ro+no-exempt findings = %d, want refusal", got)
	}
	// 3. read_only ON + exemption proven, super OFF → proceeds.
	result = t24MustAudit(t, "CREATE RESOURCE GROUP rg TYPE=USER;", base(func(f *spec.InstanceFacts) {
		f.MySQLReadOnlyKnown, f.MySQLReadOnly = true, true
		f.MySQLSuperReadOnlyKnown, f.MySQLSuperReadOnly = true, false
		f.MySQLReadOnlyExemptKnown, f.MySQLReadOnlyExempt = true, true
	}))
	if got := len(t22Findings(result, 0, t24ValidRule)); got != 0 {
		t.Fatalf("ro+exempt findings = %d, want none", got)
	}
	// 4. read_only ON + exemption unknown → undecidable, exemption gap.
	result = t24MustAudit(t, "CREATE RESOURCE GROUP rg TYPE=USER;", base(func(f *spec.InstanceFacts) {
		f.MySQLReadOnlyKnown, f.MySQLReadOnly = true, true
		f.MySQLSuperReadOnlyKnown, f.MySQLSuperReadOnly = true, false
	}))
	if gaps := t22GapCodes(result.Statements[0]); !slices.Contains(gaps, t24ROExGap) {
		t.Fatalf("ro+unknown-exempt gaps = %v, want %s", gaps, t24ROExGap)
	}
	if got := len(t22Findings(result, 0, t24ValidRule)); got != 0 {
		t.Fatalf("ro+unknown-exempt findings = %d, want none — undecidable never fabricates", got)
	}
	// 5. super_read_only ON refuses an ordinary client even with the
	// exemption proven.
	result = t24MustAudit(t, "CREATE RESOURCE GROUP rg TYPE=USER;", base(func(f *spec.InstanceFacts) {
		f.MySQLSuperReadOnlyKnown, f.MySQLSuperReadOnly = true, true
		f.MySQLReadOnlyExemptKnown, f.MySQLReadOnlyExempt = true, true
	}))
	if got := len(t22Findings(result, 0, t24ValidRule)); got != 1 {
		t.Fatalf("sro+exempt findings = %d, want refusal — super_read_only has no ordinary-client exemption", got)
	}
	// 6. read_only OFF proven + exemption proven → super_read_only still
	// flips the outcome (an sro=ON world refuses), so the sro gap is
	// owed — while the read_only gap is not.
	result = t24MustAudit(t, "CREATE RESOURCE GROUP rg TYPE=USER;", base(func(f *spec.InstanceFacts) {
		f.MySQLReadOnlyKnown, f.MySQLReadOnly = true, false
		f.MySQLReadOnlyExemptKnown, f.MySQLReadOnlyExempt = true, true
	}))
	gaps := t22GapCodes(result.Statements[0])
	if !slices.Contains(gaps, t24SROGap) {
		t.Fatalf("ro-off+exempt gaps = %v, want %s — sro=ON still refuses", gaps, t24SROGap)
	}
	if slices.Contains(gaps, t24ROGap) {
		t.Fatalf("ro-off+exempt gaps = %v, want no read_only gap — it is already proven", gaps)
	}
	// RESOURCE_GROUP_ADMIN is not a read-only exemption: admin proven,
	// exemption unknown under read_only still owes the gap.
	result = t24MustAudit(t, "CREATE RESOURCE GROUP rg TYPE=USER;", base(func(f *spec.InstanceFacts) {
		f.MySQLReadOnlyKnown, f.MySQLReadOnly = true, true
		f.MySQLSuperReadOnlyKnown, f.MySQLSuperReadOnly = true, false
		f.MySQLResourceGroupAdminKnown, f.MySQLResourceGroupAdmin = true, true
	}))
	if gaps := t22GapCodes(result.Statements[0]); !slices.Contains(gaps, t24ROExGap) {
		t.Fatalf("admin≠exempt gaps = %v, want %s", gaps, t24ROExGap)
	}
}

// ── B3.6: T24-R4 fact channels — the lazy provider bundle and the
// sourced deployment-evidence seam (B3.6).

// The lazy premise-bundle loader merges into the request facts before
// the batch adjudicates: the provider's own InstanceFacts never
// carried the read-only answers, the loader supplies them, and the
// gate closes with no gaps.
func TestT24LazyLoaderMergesIntoPremises(t *testing.T) {
	facts := t24Facts("8.0.36")
	facts.MySQLReadOnlyKnown = false
	facts.MySQLSuperReadOnlyKnown = false
	provider := &t24Provider{version: "8.0.36", facts: facts,
		loadedFacts: &spec.InstanceFacts{
			MySQLReadOnlyKnown:      true,
			MySQLReadOnly:           false,
			MySQLSuperReadOnlyKnown: true,
			MySQLSuperReadOnly:      false,
		},
		groups: map[string]t24GroupFact{"rg": {status: spec.MetadataStatusNotFound}},
	}
	result := t24MustAudit(t, "CREATE RESOURCE GROUP rg TYPE=USER;", provider)
	if gaps := t22GapCodes(result.Statements[0]); len(gaps) != 0 {
		t.Fatalf("gaps = %v, want none — the loader answered the read-only premises", gaps)
	}
	if got := len(t22Findings(result, 0, t24ValidRule)); got != 0 {
		t.Fatalf("findings = %d, want none under merged facts", got)
	}
	// A loader failure propagates — never degraded to unknown facts.
	provider.loadErr = errors.New("facts channel denied")
	if _, err := t24AuditRaw(t, "CREATE RESOURCE GROUP rg TYPE=USER;", provider); err == nil {
		t.Fatal("want propagated loader error — a failed read is not an unknown fact")
	}
}

// Deployment evidence fills only the premises no SQL channel observes,
// binds strictly to product+version, and never overwrites a proven
// online answer.
func TestT24DeploymentEvidenceChannel(t *testing.T) {
	facts := t24Facts("8.0.36")
	facts.MySQLHostVCPUsKnown = false
	facts.MySQLResourceGroupPriorityCapKnown = false
	provider := &t24Provider{version: "8.0.36", facts: facts,
		groups: map[string]t24GroupFact{"rg": {status: spec.MetadataStatusNotFound}},
	}
	sql := "CREATE RESOURCE GROUP rg TYPE=USER VCPU=7 THREAD_PRIORITY=5;"

	// A bound, sourced bundle fills the unproven slots — the statement
	// adjudicates completely.
	vcpus := uint64(8)
	capable := true
	evidence := &spec.DeploymentEvidence{
		Source: "capacity-report:2025-11", Product: "mysql", Version: "8.0.36",
		HostVCPUs: &vcpus, PriorityCapable: &capable,
	}
	result := t24AuditEvidence(t, sql, provider, evidence)
	if gaps := t22GapCodes(result.Statements[0]); len(gaps) != 0 {
		t.Fatalf("gaps = %v, want none — bound evidence answered host/priority premises", gaps)
	}

	// A version-mismatched bundle is ignored wholesale — the missing
	// premises stay bounded gaps.
	result = t24AuditEvidence(t, sql, provider, &spec.DeploymentEvidence{
		Source: "capacity-report:2025-11", Product: "mysql", Version: "8.0.35",
		HostVCPUs: &vcpus, PriorityCapable: &capable,
	})
	gaps := t22GapCodes(result.Statements[0])
	if !slices.Contains(gaps, t24HostGap) || !slices.Contains(gaps, t24CapGap) {
		t.Fatalf("gaps = %v, want host+capability gaps — mismatched evidence never binds", gaps)
	}

	// An unsourced bundle is ignored the same way — provenance is
	// required, never a generic trust flag.
	result = t24AuditEvidence(t, sql, provider, &spec.DeploymentEvidence{
		Product: "mysql", Version: "8.0.36",
		HostVCPUs: &vcpus, PriorityCapable: &capable,
	})
	gaps = t22GapCodes(result.Statements[0])
	if !slices.Contains(gaps, t24HostGap) || !slices.Contains(gaps, t24CapGap) {
		t.Fatalf("gaps = %v, want host+capability gaps — unsourced evidence never binds", gaps)
	}

	// Evidence never overwrites a proven online observation: the
	// provider already proved a 4-CPU host, so evidence claiming 64
	// cannot rescue a VCPU=5 declaration — the refusal stands.
	facts4 := t24Facts("8.0.36")
	facts4.MySQLHostVCPUs = 4
	provider4 := &t24Provider{version: "8.0.36", facts: facts4,
		groups: map[string]t24GroupFact{"rg": {status: spec.MetadataStatusNotFound}},
	}
	vcpus64 := uint64(64)
	result = t24AuditEvidence(t, "CREATE RESOURCE GROUP rg TYPE=USER VCPU=5;", provider4, &spec.DeploymentEvidence{
		Source: "capacity-report:2025-11", Product: "mysql", Version: "8.0.36",
		HostVCPUs: &vcpus64,
	})
	if got := len(t22Findings(result, 0, t24ValidRule)); got != 1 {
		t.Fatalf("findings = %d, want the host-range refusal — evidence never overwrites proven facts", got)
	}
}

// A proven-complete bound observation answers occupancy for every
// group — a missing key is a proven "unoccupied", not an unproven one.
func TestT24BoundCompleteSeedsUnoccupied(t *testing.T) {
	facts := t24Facts("8.0.36")
	facts.MySQLResourceGroupBound = map[string]bool{"other": true}
	facts.MySQLResourceGroupBoundComplete = true
	provider := &t24Provider{version: "8.0.36", facts: facts, groups: map[string]t24GroupFact{
		"rg": {status: spec.MetadataStatusConfirmed, exists: true,
			definition: &spec.MySQLResourceGroupDefinition{GroupType: spec.MySQLResourceGroupTypeUser}},
	}}
	result := t24MustAudit(t, "DROP RESOURCE GROUP rg;", provider)
	if gaps := t22GapCodes(result.Statements[0]); slices.Contains(gaps, t24OccGap) {
		t.Fatalf("gaps = %v, want no occupancy gap under a proven-complete bound set", gaps)
	}
	if got := len(t22Findings(result, 0, t24ValidRule)); got != 0 {
		t.Fatalf("drop findings = %d, want none — the group is proven unoccupied", got)
	}
	// An incomplete map still leaves a missing key unproven.
	facts.MySQLResourceGroupBoundComplete = false
	result = t24MustAudit(t, "DROP RESOURCE GROUP rg;", provider)
	if gaps := t22GapCodes(result.Statements[0]); !slices.Contains(gaps, t24OccGap) {
		t.Fatalf("gaps = %v, want %s under a partial bound map", gaps, t24OccGap)
	}
}

// ── B4: FORCE pairing and occupancy ──

// FORCE on ALTER is legal only paired with DISABLE — every other
// pairing refuses (ER_INVALID_USE_OF_FORCE_OPTION); DISABLE alone stays
// legal and occupancy never gates an ALTER.
func TestT24ForcePairing(t *testing.T) {
	facts := t24Facts("8.0.36")
	facts.MySQLResourceGroupBound = map[string]bool{"rg": true}
	provider := &t24Provider{version: "8.0.36", facts: facts, groups: map[string]t24GroupFact{
		"rg": {status: spec.MetadataStatusConfirmed, exists: true,
			definition: &spec.MySQLResourceGroupDefinition{GroupType: spec.MySQLResourceGroupTypeUser}},
	}}
	for _, tc := range []struct {
		sql     string
		refused bool
	}{
		{"ALTER RESOURCE GROUP rg FORCE;", true},
		{"ALTER RESOURCE GROUP rg ENABLE FORCE;", true},
		{"ALTER RESOURCE GROUP rg DISABLE FORCE;", false},
		{"ALTER RESOURCE GROUP rg DISABLE;", false},
		{"ALTER RESOURCE GROUP rg;", false},
		{"ALTER RESOURCE GROUP rg VCPU=0-1;", false},
	} {
		result := t24MustAudit(t, tc.sql, provider)
		got := len(t22Findings(result, 0, t24ValidRule))
		if tc.refused && got != 1 {
			t.Fatalf("%q: findings = %d, want the FORCE-pairing refusal", tc.sql, got)
		}
		if !tc.refused && got != 0 {
			t.Fatalf("%q: findings = %d, want none — occupancy never refuses an ALTER", tc.sql, got)
		}
	}
}

// DROP occupancy: proven-occupied without FORCE refuses
// (ER_RESOURCE_GROUP_BUSY); unproven owes the occupancy gap; proven
// unoccupied proceeds; FORCE relocates to the default groups regardless.
func TestT24DropOccupancy(t *testing.T) {
	def := &spec.MySQLResourceGroupDefinition{GroupType: spec.MySQLResourceGroupTypeUser}
	mkProvider := func(bound map[string]bool) *t24Provider {
		facts := t24Facts("8.0.36")
		facts.MySQLResourceGroupBound = bound
		return &t24Provider{version: "8.0.36", facts: facts, groups: map[string]t24GroupFact{
			"rg": {status: spec.MetadataStatusConfirmed, exists: true, definition: def},
		}}
	}
	// Occupied, no FORCE → refused.
	result := t24MustAudit(t, "DROP RESOURCE GROUP rg;", mkProvider(map[string]bool{"rg": true}))
	findings := t22Findings(result, 0, t24ValidRule)
	if len(findings) != 1 || !strings.Contains(findings[0].Message, "threads are bound") {
		t.Fatalf("occupied drop findings = %+v, want the busy refusal", findings)
	}
	// Occupied, FORCE → proceeds, relocates.
	result = t24MustAudit(t, "DROP RESOURCE GROUP rg FORCE;", mkProvider(map[string]bool{"rg": true}))
	if got := len(t22Findings(result, 0, t24ValidRule)); got != 0 {
		t.Fatalf("force-drop findings = %d, want none", got)
	}
	if got := len(t22Findings(result, 0, t24ForceWarn)); got != 1 {
		t.Fatalf("force warns = %d, want the declared-relocation warning", got)
	}
	// Unknown occupancy → gap, undecidable post.
	result = t24MustAudit(t, "DROP RESOURCE GROUP rg;", mkProvider(map[string]bool{}))
	if gaps := t22GapCodes(result.Statements[0]); !slices.Contains(gaps, t24OccGap) {
		t.Fatalf("gaps = %v, want %s for unproven occupancy", gaps, t24OccGap)
	}
	// Proven unoccupied → proceeds.
	result = t24MustAudit(t, "DROP RESOURCE GROUP rg;", mkProvider(map[string]bool{"rg": false}))
	if got := len(t22Findings(result, 0, t24ValidRule)); got != 0 {
		t.Fatalf("unoccupied drop findings = %d, want none", got)
	}
	// DISABLE FORCE relocates bound threads — the group provably lands
	// unoccupied; a following DROP needs no occupancy fact.
	entry := t24Entry(t, "ALTER RESOURCE GROUP rg DISABLE FORCE;", "rg", mkProvider(map[string]bool{"rg": true}))
	if entry == nil || entry.occupied == nil || *entry.occupied {
		t.Fatalf("entry = %+v, want proven-unoccupied after DISABLE FORCE", entry)
	}
	// ALTER without FORCE preserves binding state — occupancy never
	// changes through it.
	entry = t24Entry(t, "ALTER RESOURCE GROUP rg DISABLE;", "rg", mkProvider(map[string]bool{"rg": true}))
	if entry == nil || entry.occupied == nil || !*entry.occupied {
		t.Fatalf("entry = %+v, want preserved occupied after bare DISABLE", entry)
	}
	// CREATE lands proven-unoccupied — a later DROP needs no fact.
	entry = t24Entry(t, "CREATE RESOURCE GROUP rg TYPE=USER;", "rg",
		t24Groups(map[string]t24GroupFact{"rg": {status: spec.MetadataStatusNotFound}}))
	if entry == nil || entry.occupied == nil || *entry.occupied {
		t.Fatalf("entry = %+v, want proven-unoccupied after CREATE", entry)
	}
}

// A DROP left undecidable by unproven occupancy tombstones the exists
// answer — a following statement cannot lean on either outcome.
func TestT24UndecidableDropTombstones(t *testing.T) {
	def := &spec.MySQLResourceGroupDefinition{GroupType: spec.MySQLResourceGroupTypeUser}
	facts := t24Facts("8.0.36")
	provider := &t24Provider{version: "8.0.36", facts: facts, groups: map[string]t24GroupFact{
		"rg": {status: spec.MetadataStatusConfirmed, exists: true, definition: def},
	}}
	entry := t24Entry(t, "DROP RESOURCE GROUP rg;", "rg", provider)
	if entry == nil || entry.state != resourceGroupUnknown {
		t.Fatalf("entry = %+v, want unknown exists after an undecidable drop", entry)
	}
}

// ── E: the complete-premise first path ──

// Every premise proven: CREATE→ALTER→DROP reaches the public Audit API
// with deterministic findings/state — zero blockers, zero premise gaps.
func TestT24CompletePremiseFirstPath(t *testing.T) {
	result := t24MustAudit(t, "CREATE RESOURCE GROUP rg TYPE=USER VCPU=0-3 THREAD_PRIORITY=10 ENABLE;\n"+
		"ALTER RESOURCE GROUP rg VCPU=0-1;\nDROP RESOURCE GROUP rg;",
		t24Groups(map[string]t24GroupFact{
			"rg": {status: spec.MetadataStatusNotFound, exists: false},
		}))
	for i, stmt := range result.Statements {
		if got := len(t22Findings(result, i, t24ValidRule)); got != 0 {
			t.Fatalf("statement %d findings = %d, want none under complete premises", i, got)
		}
		if gaps := t22GapCodes(stmt); len(gaps) != 0 {
			t.Fatalf("statement %d gaps = %v, want none under complete premises", i, gaps)
		}
	}
}

// Each premise removed one at a time keeps its bounded gap — a missing
// fact never silently downgrades to an assumed default.
func TestT24MissingPremiseControls(t *testing.T) {
	sql := "CREATE RESOURCE GROUP rg TYPE=USER VCPU=0-3 THREAD_PRIORITY=5;\nDROP RESOURCE GROUP rg;"
	mutate := map[string]struct {
		gap   string
		stmts []int
		apply func(*spec.InstanceFacts)
	}{
		"version":    {t24VersionGap, []int{0, 1}, func(f *spec.InstanceFacts) { f.Version = "" }},
		"feature":    {t24FeatureGap, []int{0, 1}, func(f *spec.InstanceFacts) { f.MySQLResourceGroupFeatureKnown = false }},
		"host":       {t24HostGap, []int{0}, func(f *spec.InstanceFacts) { f.MySQLHostVCPUsKnown = false }},
		"privilege":  {t24PrivGap, []int{0, 1}, func(f *spec.InstanceFacts) { f.MySQLResourceGroupAdminKnown = false }},
		"capability": {t24CapGap, []int{0}, func(f *spec.InstanceFacts) { f.MySQLResourceGroupPriorityCapKnown = false }},
		"state":      {t24StateGap, []int{0, 1}, func(f *spec.InstanceFacts) {}}, // state removed via provider below
	}
	for name, tc := range mutate {
		facts := t24Facts("8.0.36")
		tc.apply(facts)
		provider := &t24Provider{version: facts.Version, facts: facts, groups: map[string]t24GroupFact{
			"rg": {status: spec.MetadataStatusNotFound},
		}}
		if name == "state" {
			provider.groups = map[string]t24GroupFact{} // group unresolvable → state unproven
		}
		result := t24MustAudit(t, sql, provider)
		for _, i := range tc.stmts {
			if gaps := t22GapCodes(result.Statements[i]); !slices.Contains(gaps, tc.gap) {
				t.Fatalf("missing %s: statement %d gaps = %v, want %s", name, i, gaps, tc.gap)
			}
		}
	}
	// Occupancy is a separate control: a batch-created group lands
	// proven-unoccupied, so the gap needs a catalog-proven group
	// dropped without FORCE under a nil binding map.
	facts := t24Facts("8.0.36")
	facts.MySQLResourceGroupBound = nil
	result := t24MustAudit(t, "DROP RESOURCE GROUP rg;", &t24Provider{
		version: "8.0.36", facts: facts,
		groups: map[string]t24GroupFact{
			"rg": {status: spec.MetadataStatusConfirmed, exists: true,
				definition: &spec.MySQLResourceGroupDefinition{GroupType: spec.MySQLResourceGroupTypeUser}},
		}})
	if gaps := t22GapCodes(result.Statements[0]); !slices.Contains(gaps, t24OccGap) {
		t.Fatalf("missing occupancy: gaps = %v, want %s", gaps, t24OccGap)
	}
}

// ── F: provider-read behavior through public Audit ──

// A resolver error propagates to the audit — never cached or degraded
// into an absent answer.
func TestT24ResolverErrorPropagates(t *testing.T) {
	want := errors.New("catalog read refused")
	_, err := t24AuditRaw(t, "ALTER RESOURCE GROUP rg ENABLE;", &t24Provider{
		version: "8.0.36", facts: t24Facts("8.0.36"),
		groups: map[string]t24GroupFact{"rg": {err: want}},
	})
	if err == nil || !strings.Contains(err.Error(), want.Error()) {
		t.Fatalf("err = %v, want propagated resolver error", err)
	}
}

// An unavailable catalog answer keeps the state unproven — never a
// fabricated absence.
func TestT24UnavailableCatalogStaysUnknown(t *testing.T) {
	result := t24MustAudit(t, "ALTER RESOURCE GROUP rg ENABLE;", t24Groups(map[string]t24GroupFact{}))
	if gaps := t22GapCodes(result.Statements[0]); !slices.Contains(gaps, t24StateGap) {
		t.Fatalf("gaps = %v, want %s for an unresolvable group", gaps, t24StateGap)
	}
	if got := len(t22Findings(result, 0, t24ValidRule)); got != 0 {
		t.Fatalf("findings = %d, want none — unproven state never fabricates a refusal", got)
	}
}

// ── G: identity isolation ──

// Resource groups are globally namespaced, never schema-bound: an
// unrelated DROP DATABASE cannot invalidate a proven group identity.
func TestT24IdentitySurvivesDropDatabase(t *testing.T) {
	result := t24MustAudit(t, "CREATE RESOURCE GROUP rg TYPE=USER;\nDROP DATABASE app;\nALTER RESOURCE GROUP rg VCPU=0;",
		t24Groups(map[string]t24GroupFact{"rg": {status: spec.MetadataStatusNotFound}}))
	alter := result.Statements[2]
	if gaps := t22GapCodes(alter); slices.Contains(gaps, t24StateGap) {
		t.Fatalf("alter gaps = %v, want none — the batch-proven group survives an unrelated schema drop", gaps)
	}
}

// A same-named TiDB RU group never shares the MySQL slot — the families
// are product-isolated, and TiDB's `DROP RESOURCE GROUP IF EXISTS`
// stays on the TiDB path as a vendor boundary under MySQL and an
// audited drop under TiDB.
func TestT24FamilyIsolation(t *testing.T) {
	// MySQL bare DROP binds the MySQL registry: a batch create→drop
	// resolves ahead of the provider with no state gap.
	result := t24MustAudit(t, "CREATE RESOURCE GROUP rg TYPE=USER;\nDROP RESOURCE GROUP rg;",
		t24Groups(map[string]t24GroupFact{"rg": {status: spec.MetadataStatusNotFound}}))
	if gaps := t22GapCodes(result.Statements[1]); slices.Contains(gaps, t24StateGap) {
		t.Fatalf("drop gaps = %v, want none — the batch-created identity resolves", gaps)
	}
	// TiDB's guard order is `DROP RESOURCE GROUP IF EXISTS <name>` —
	// audited under TiDB, a vendor boundary under MySQL.
	tidb, err := t22Audit(t, "DROP RESOURCE GROUP IF EXISTS rg;", spec.DialectTiDB, nil, "")
	if err != nil {
		t.Fatalf("tidb drop-if-exists err = %v, want audited statement", err)
	}
	if len(tidb.Statements) != 1 || tidb.Statements[0].Kind != string(spec.KindDDL) {
		t.Fatalf("tidb drop-if-exists = %+v, want an audited ddl drop", tidb.Statements)
	}
	mysql, err := t24AuditRaw(t, "DROP RESOURCE GROUP IF EXISTS rg;", t24Groups(map[string]t24GroupFact{}))
	if !errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("mysql drop-if-exists err = %v, want vendor boundary", err)
	}
	u := t24Unsupported(mysql, "drop_resource_group")
	if u == nil || u.Metadata["boundary"] != "vendor" {
		t.Fatalf("mysql drop-if-exists = %+v, want vendor boundary", mysql.Unsupported)
	}
	// `DROP rg IF EXISTS` — neither grammar's word order — stays a
	// parse diagnostic on both dialects.
	for _, d := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		res, err := AuditSQL(context.Background(), Request{SQL: "DROP RESOURCE GROUP rg IF EXISTS;", Dialect: d})
		if err == nil || len(res.Diagnostics) == 0 || res.Diagnostics[0].Classification != spec.DiagnosticParserError {
			t.Fatalf("dialect %s: wrong-order guard = %+v, want parser_error", d, res.Diagnostics)
		}
	}
}

// An unaudited statement mid-batch contaminates the registry — a
// following group statement can never lean on stale knowledge.
func TestT24UnauditedContaminates(t *testing.T) {
	result, err := t24AuditRaw(t, "CREATE RESOURCE GROUP rg TYPE=USER;\n"+
		"SET RESOURCE GROUP rg FOR 1;\n"+
		"ALTER RESOURCE GROUP rg VCPU=0;",
		t24Groups(map[string]t24GroupFact{"rg": {status: spec.MetadataStatusNotFound}}))
	if !errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("err = %v, want the bounded set-for boundary alongside the audited statements", err)
	}
	alter := result.Statements[2]
	if gaps := t22GapCodes(alter); !slices.Contains(gaps, t24StateGap) {
		t.Fatalf("alter gaps = %v, want %s — the unaudited binding wipes prior knowledge", gaps, t24StateGap)
	}
}

// ── Retained cells ──

// Positional grammar: TYPE is the required first CREATE member (the
// `=` is optional, so `TYPE USER` claims), the optional members sit in
// fixed order, a bare ALTER is a valid no-op, and any member out of
// position or repeated keeps the base grammar's parse diagnostic —
// the same verdict MySQL's own parser gives.
func TestT24PositionalGrammar(t *testing.T) {
	result := t24MustAudit(t, "CREATE RESOURCE GROUP rg TYPE USER VCPU=1 DISABLE;\n"+
		"ALTER RESOURCE GROUP rg;\n"+
		"ALTER RESOURCE GROUP rg FORCE;",
		t24Groups(map[string]t24GroupFact{
			"rg": {status: spec.MetadataStatusNotFound, exists: false},
		}))
	if len(result.Statements) != 3 {
		t.Fatalf("statements = %d, want 3", len(result.Statements))
	}
	for i := range result.Statements {
		if result.Statements[i].Kind != string(spec.KindDDL) {
			t.Fatalf("statement %d kind = %q, want ddl", i, result.Statements[i].Kind)
		}
	}
	if got := len(t22Findings(result, 0, t24AffWarn)); got != 1 {
		t.Fatalf("affinity warn on `TYPE USER` create = %d, want 1", got)
	}
	for _, sql := range []string{
		"CREATE RESOURCE GROUP rg TYPE=USER TYPE=SYSTEM",
		"CREATE RESOURCE GROUP rg VCPU=1 TYPE=USER",
		"CREATE RESOURCE GROUP rg TYPE=USER VCPU=0 VCPU=1",
		"ALTER RESOURCE GROUP rg FORCE VCPU=1",
		"ALTER RESOURCE GROUP rg DISABLE FORCE VCPU=1",
		"CREATE RESOURCE GROUP rg ENABLE TYPE=USER",
		"SET RESOURCE GROUP rg FOR 1,",
	} {
		if res, err := t24AuditRaw(t, sql, t24Groups(map[string]t24GroupFact{})); err == nil {
			t.Fatalf("%q audited = %+v, want parse diagnostic", sql, res.Statements)
		} else if len(res.Diagnostics) == 0 || res.Diagnostics[0].Classification != spec.DiagnosticParserError {
			t.Fatalf("%q diagnostics = %+v, want parser_error", sql, res.Diagnostics)
		}
	}
}

// Disabled-rule coverage: with the valid rule off, declared-effect
// warnings still fire on an applicable statement and a proven refusal
// still silences them — gating reads the refusal facts, not the
// blocker finding.
func TestT24WarningGatingIndependentOfValidRule(t *testing.T) {
	policyPath := t22Policy(t, map[string]policy.RulePolicy{
		t24ValidRule: {Enabled: false},
		t24AffWarn:   {Enabled: true},
		t24ForceWarn: {Enabled: true},
	})
	provider := t24Groups(map[string]t24GroupFact{
		"absent": {status: spec.MetadataStatusNotFound, exists: false},
	})
	result, err := AuditSQL(context.Background(), Request{
		SQL:              "CREATE RESOURCE GROUP rg TYPE=USER VCPU=0 ENABLE;\nALTER RESOURCE GROUP absent FORCE;",
		Dialect:          spec.DialectMySQL,
		TargetVersion:    provider.version,
		ConfigPath:       policyPath,
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("AuditSQL: %v", err)
	}
	for i := range result.Statements {
		if got := len(t22Findings(result, i, t24ValidRule)); got != 0 {
			t.Fatalf("statement %d valid findings = %d, want 0 — the rule is disabled", i, got)
		}
	}
	if got := len(t22Findings(result, 0, t24AffWarn)); got != 1 {
		t.Fatalf("affinity warn on applicable create = %d, want 1", got)
	}
	if got := len(t22Findings(result, 1, t24ForceWarn)); got != 0 {
		t.Fatalf("force warn on refused alter = %d, want 0 — refusal gating does not read the blocker", got)
	}
}

// FORCE on ALTER DISABLE emits the declared-relocation warning; a
// refused FORCE statement stays silent.
func TestT24ForceWarns(t *testing.T) {
	def := &spec.MySQLResourceGroupDefinition{GroupType: spec.MySQLResourceGroupTypeUser}
	result := t24MustAudit(t, "ALTER RESOURCE GROUP rg DISABLE FORCE;\nALTER RESOURCE GROUP absent FORCE;",
		t24Groups(map[string]t24GroupFact{
			"rg":     {status: spec.MetadataStatusConfirmed, exists: true, definition: def},
			"absent": {status: spec.MetadataStatusNotFound, exists: false},
		}))
	if got := len(t22Findings(result, 0, t24ForceWarn)); got != 1 {
		t.Fatalf("force warn on proven target = %d, want 1", got)
	}
	if got := len(t22Findings(result, 1, t24ForceWarn)); got != 0 {
		t.Fatalf("force warn on refused alter = %d, want 0", got)
	}
}

// Boundary cells: the MySQL family on TiDB is a vendor boundary; the
// TiDB RU family on MySQL keeps its vendor boundary; SET ... FOR is
// bounded unaudited on MySQL, vendor on TiDB.
func t24Unsupported(result report.Result, feature string) *spec.UnsupportedDetail {
	for i := range result.Unsupported {
		if result.Unsupported[i].Feature == feature {
			return &result.Unsupported[i]
		}
	}
	return nil
}

func TestT24DialectBoundaries(t *testing.T) {
	mysql := t24MustAudit(t, "CREATE RESOURCE GROUP rg TYPE=USER;", t24Groups(map[string]t24GroupFact{"rg": {status: spec.MetadataStatusNotFound}}))
	if len(mysql.Unsupported) != 0 {
		t.Fatalf("mysql-family create on mysql is unsupported: %+v", mysql.Unsupported)
	}
	tidb, err := t22Audit(t, "CREATE RESOURCE GROUP rg TYPE=USER;", spec.DialectTiDB, nil, "")
	if !errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("tidb err = %v, want ErrUnsupportedStatement", err)
	}
	u := t24Unsupported(tidb, "create_resource_group")
	if u == nil || u.Metadata["boundary"] != "vendor" {
		t.Fatalf("mysql-family create on tidb = %+v, want vendor boundary", tidb.Unsupported)
	}
	mysqlRU, err := t24AuditRaw(t, "CREATE RESOURCE GROUP rg RU_PER_SEC=100;", t24Groups(map[string]t24GroupFact{}))
	if !errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("mysql-ru err = %v, want ErrUnsupportedStatement", err)
	}
	u = t24Unsupported(mysqlRU, "create_resource_group")
	if u == nil || u.Metadata["boundary"] != "vendor" {
		t.Fatalf("tidb-family create on mysql = %+v, want vendor boundary", mysqlRU.Unsupported)
	}
	setFor, err := t24AuditRaw(t, "SET RESOURCE GROUP rg FOR 1,2;", t24Groups(map[string]t24GroupFact{}))
	if !errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("set-for err = %v, want ErrUnsupportedStatement", err)
	}
	u = t24Unsupported(setFor, "set_resource_group")
	if u == nil || u.Metadata["boundary"] == "vendor" {
		t.Fatalf("SET FOR on mysql = %+v, want bounded unaudited set_resource_group", setFor.Unsupported)
	}
}

// Upstream validates per-range VCPU endpoints at prepare time, ahead of the
// execute-time inversion check — on a proven small host an endpoint-invalid
// AND inverted member reports ER_INVALID_VCPU_ID, never the later
// ER_INVALID_VCPU_RANGE.
func TestT24EndpointPrecedesInversion(t *testing.T) {
	facts := t24Facts("8.0.36")
	facts.MySQLHostVCPUs = 4
	facts.MySQLHostVCPUsKnown = true
	result := t24MustAudit(t, "CREATE RESOURCE GROUP rg TYPE=USER VCPU=99-5;", &t24Provider{
		version: "8.0.36", facts: facts,
		groups: map[string]t24GroupFact{"rg": {status: spec.MetadataStatusNotFound}},
	})
	findings := t22Findings(result, 0, t24ValidRule)
	if len(findings) != 1 || !strings.Contains(findings[0].Message, "host's CPU count") {
		t.Fatalf("findings = %+v, want the endpoint refusal first", findings)
	}
}

// Upstream runs the ALTER FORCE-pairing check last, after the VCPU and
// priority validations — an out-of-bound priority reports its own refusal,
// never the FORCE-pairing one.
func TestT24PriorityPrecedesForcePairing(t *testing.T) {
	facts := t24Facts("8.0.36")
	result := t24MustAudit(t, "ALTER RESOURCE GROUP rg THREAD_PRIORITY=99 FORCE;", &t24Provider{
		version: "8.0.36", facts: facts,
		groups: map[string]t24GroupFact{"rg": {status: spec.MetadataStatusConfirmed, exists: true,
			definition: &spec.MySQLResourceGroupDefinition{GroupType: spec.MySQLResourceGroupTypeUser}}},
	})
	findings := t22Findings(result, 0, t24ValidRule)
	if len(findings) != 1 || !strings.Contains(findings[0].Message, "THREAD_PRIORITY") {
		t.Fatalf("findings = %+v, want the priority refusal first", findings)
	}
}
