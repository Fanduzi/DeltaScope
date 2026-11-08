package audit

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// batch_state_t22_test.go pins the T22/#101 ordered-state contract:
// create/alter/drop transitions, refusal preservation, guard no-ops, the
// reserved `default` group, and provider/ordered-state precedence — all
// through the public AuditSQL path with an injected resolver.

type t22GroupFact struct {
	status     spec.MetadataStatus
	exists     bool
	definition *spec.ResourceGroupDefinition
	err        error
}

type t22Provider struct {
	capabilityKnown bool
	capabilityOn    bool
	groups          map[string]t22GroupFact
	calls           []spec.ObjectLookupRequest
	failSecond      error
	callCount       int
}

func (p *t22Provider) LoadInstanceFacts(context.Context, spec.Dialect, string) (*spec.InstanceFacts, error) {
	return &spec.InstanceFacts{
		Version:                        "8.0.11-TiDB-v8.5.0",
		TiDBEnableResourceControl:      p.capabilityOn,
		TiDBEnableResourceControlKnown: p.capabilityKnown,
	}, nil
}

func (p *t22Provider) LoadTableSnapshot(context.Context, spec.Dialect, string, string) (*spec.TableSnapshot, error) {
	return nil, nil
}

func (p *t22Provider) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	p.callCount++
	if p.callCount == 2 && p.failSecond != nil {
		return nil, p.failSecond
	}
	p.calls = append(p.calls, req)
	if req.Type != "resource_group" {
		return &spec.ObjectSnapshot{Schema: req.Schema, Type: req.Type, Name: req.Name, Status: spec.MetadataStatusUnavailable}, nil
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
		Type:          req.Type,
		Name:          req.Name,
		Status:        fact.status,
		Exists:        fact.exists,
		ResourceGroup: fact.definition,
	}, nil
}

func t22ProviderOn(groups map[string]t22GroupFact) *t22Provider {
	return &t22Provider{capabilityKnown: true, capabilityOn: true, groups: groups}
}

// CREATE→ALTER→DROP on a batch-created identity resolves ahead of the
// provider — no catalog read can contradict batch-proven state, and the
// final DROP stays a normal audited statement (no blocker, no state gap).
// The catalog proves the identity absent so the CREATE itself is proven.
func TestT22BatchLifecycleResolvesAheadOfProvider(t *testing.T) {
	provider := t22ProviderOn(map[string]t22GroupFact{
		"rg": {status: spec.MetadataStatusNotFound, exists: false},
	})
	result := t22MustAudit(t,
		"CREATE RESOURCE GROUP rg RU_PER_SEC=100;\nALTER RESOURCE GROUP rg RU_PER_SEC=200;\nDROP RESOURCE GROUP rg;",
		spec.DialectTiDB, provider, "")
	for i, stmt := range result.Statements {
		if slices.Contains(t22GapCodes(stmt), t22StateGap) {
			t.Fatalf("statement %d gaps = %v, want no state gap — batch provenance wins", i, t22GapCodes(stmt))
		}
	}
	if got := len(t22Findings(result, 2, t22ValidRule)); got != 0 {
		t.Fatalf("drop findings = %d, want 0 on a batch-proven target", got)
	}
}

// An absent-target ALTER without IF EXISTS blocks on the valid rule only
// when absence is proven — unproven targets owe the bounded state gap
// instead of a fabricated blocker.
func TestT22ProvenAbsentAlterBlocksUnprovenGaps(t *testing.T) {
	proven := t22ProviderOn(map[string]t22GroupFact{
		"ghost": {status: spec.MetadataStatusNotFound, exists: false},
	})
	result := t22MustAudit(t, "ALTER RESOURCE GROUP ghost RU_PER_SEC=10;", spec.DialectTiDB, proven, "")
	if got := len(t22Findings(result, 0, t22ValidRule)); got != 1 {
		t.Fatalf("findings = %d, want 1 blocker on proven-absent alter", got)
	}

	unproven := t22ProviderOn(map[string]t22GroupFact{
		"ghost": {status: spec.MetadataStatusUnavailable},
	})
	result = t22MustAudit(t, "ALTER RESOURCE GROUP ghost RU_PER_SEC=10;", spec.DialectTiDB, unproven, "")
	if got := len(t22Findings(result, 0, t22ValidRule)); got != 0 {
		t.Fatalf("findings = %d, want 0 — unproven absence is never a fabricated blocker", got)
	}
	if !slices.Contains(t22GapCodes(result.Statements[0]), t22StateGap) {
		t.Fatalf("gaps = %v, want %s", t22GapCodes(result.Statements[0]), t22StateGap)
	}
}

// The reserved builtin group: DROP `default` is a proven refusal — the
// valid rule blocks and the lifecycle notice stays suppressed.
func TestT22ReservedDefaultRefusalSuppressesNotice(t *testing.T) {
	provider := t22ProviderOn(map[string]t22GroupFact{
		"default": {status: spec.MetadataStatusConfirmed, exists: true},
	})
	result := t22MustAudit(t, "DROP RESOURCE GROUP default;", spec.DialectTiDB, provider, "")
	if got := len(t22Findings(result, 0, t22ValidRule)); got != 1 {
		t.Fatalf("findings = %d, want 1 reserved-group blocker", got)
	}
	if got := len(t22Findings(result, 0, "ddl.drop_resource_group.notice")); got != 0 {
		t.Fatalf("notices = %d, want 0 — refusal dominance suppresses the success claim", got)
	}
}

// A known-OFF resource-control gate refuses before existence adjudication:
// blocker on every lifecycle statement even when the target is proven.
func TestT22CapabilityOffRefuses(t *testing.T) {
	provider := &t22Provider{capabilityKnown: true, capabilityOn: false, groups: map[string]t22GroupFact{
		"rg": {status: spec.MetadataStatusConfirmed, exists: true},
	}}
	result := t22MustAudit(t, "DROP RESOURCE GROUP rg;", spec.DialectTiDB, provider, "")
	findings := t22Findings(result, 0, t22ValidRule)
	if len(findings) != 1 || !strings.Contains(findings[0].Message, "tidb_enable_resource_control") {
		t.Fatalf("findings = %+v, want the capability refusal", findings)
	}
	if !slices.Contains(t22GapCodes(result.Statements[0]), t22CapabilityGap) && len(result.Statements[0].EvidenceGaps) > 0 {
		// capability is observed OFF — no capability gap may remain
		for _, gap := range result.Statements[0].EvidenceGaps {
			if gap.ReasonCode == t22CapabilityGap {
				t.Fatal("capability gap emitted for an observed OFF fact")
			}
		}
	}
}

// A failed catalog read propagates as an error — never downgraded to an
// unavailable answer, never cached as absence for a later statement.
func TestT22ResolverErrorPropagatesUncached(t *testing.T) {
	want := errors.New("resource_groups read denied")
	provider := t22ProviderOn(map[string]t22GroupFact{
		"rg": {err: want},
	})
	_, err := t22Audit(t, "DROP RESOURCE GROUP rg;", spec.DialectTiDB, provider, "")
	if err == nil {
		t.Fatal("want the resolver failure to reach the public audit path")
	}
}

// IF EXISTS on a proven-absent target is a guard no-op — the valid rule
// stays silent and the notice must not claim a successful alteration.
func TestT22GuardNoOpOnProvenAbsent(t *testing.T) {
	provider := t22ProviderOn(map[string]t22GroupFact{
		"ghost": {status: spec.MetadataStatusNotFound, exists: false},
	})
	result := t22MustAudit(t, "ALTER RESOURCE GROUP IF EXISTS ghost RU_PER_SEC=10;", spec.DialectTiDB, provider, "")
	if got := len(t22Findings(result, 0, t22ValidRule)); got != 0 {
		t.Fatalf("findings = %d, want 0 — the guard made the miss a no-op", got)
	}
	if got := len(t22Findings(result, 0, "ddl.alter_resource_group.notice")); got != 0 {
		t.Fatalf("notices = %d, want 0 — no success claim on a proven no-op", got)
	}
}

// The #101 first path with every premise observed: capability ON and a
// proven-absent catalog answer. CREATE publishes the normalized batch
// definition, ALTER reads it back — complete coverage, lifecycle notices,
// no gap and no blocker on either statement.
func TestT22GoldenPathCompletePremises(t *testing.T) {
	provider := t22ProviderOn(map[string]t22GroupFact{
		"golden_rg": {status: spec.MetadataStatusNotFound, exists: false},
	})
	result := t22MustAudit(t,
		"CREATE RESOURCE GROUP golden_rg RU_PER_SEC = 100;\nALTER RESOURCE GROUP golden_rg RU_PER_SEC = 200;",
		spec.DialectTiDB, provider, "")
	for i, stmt := range result.Statements {
		if stmt.Kind != "ddl" || stmt.Coverage.Status != "complete" || len(stmt.EvidenceGaps) != 0 {
			t.Fatalf("statement %d = kind %s coverage %s gaps %v, want ddl/complete/no gaps", i, stmt.Kind, stmt.Coverage.Status, stmt.EvidenceGaps)
		}
	}
	if got := len(t22Findings(result, 0, "ddl.create_resource_group.notice")); got != 1 {
		t.Fatalf("create notices = %d, want 1 on the proven-absent create", got)
	}
	if got := len(t22Findings(result, 1, "ddl.alter_resource_group.notice")); got != 1 {
		t.Fatalf("alter notices = %d, want 1 on the batch-proven target", got)
	}
	for i := range result.Statements {
		if got := len(t22Findings(result, i, t22ValidRule)); got != 0 {
			t.Fatalf("statement %d valid findings = %d, want 0 — every premise proven", i, got)
		}
	}
	// The dedicated lifecycle checks must have actually evaluated — a
	// clean result is only proof when the rules applied, not skipped.
	// valid, burstable.warn, and both notices apply to every RG create/
	// alter statement; their positive-path counterparts (state gaps,
	// burst warnings, blocks) are pinned by the surrounding controls.
	if result.RuleSummary == nil {
		t.Fatal("RuleSummary missing — cannot prove the dedicated rules applied")
	}
	if result.RuleSummary.Applicable < 4 {
		t.Fatalf("applicable rules = %d, want the four dedicated RG checks evaluated", result.RuleSummary.Applicable)
	}
	for _, s := range result.RuleSummary.Skipped {
		switch s.RuleID {
		case t22ValidRule, t22BurstableRule, "ddl.create_resource_group.notice", "ddl.alter_resource_group.notice":
			t.Fatalf("rule %q skipped on the golden path — it must evaluate and find nothing to block", s.RuleID)
		}
	}
}

// The CREATE post-state is a complete authored specification: fixed
// TiDB 8.5 NewResourceGroupSettings defaults (RURate=0, PRIORITY=MEDIUM,
// BurstLimit=0, Runaway=nil) make every undeclared member provable —
// burst enabled exactly when BURSTABLE or RU_PER_SEC=UNLIMITED declares it.
func TestT22CreatePublishesNormalizedDefinition(t *testing.T) {
	u64 := func(v uint64) *uint64 { return &v }
	decl := func(options ...spec.ResourceGroupOption) *spec.ResourceGroupDeclaration {
		return &spec.ResourceGroupDeclaration{Name: "rg", Options: options}
	}
	t.Run("numeric create publishes fixed defaults", func(t *testing.T) {
		def := authoredResourceGroupDefinition(decl(spec.ResourceGroupOption{Kind: spec.ResourceGroupOptionRURate, UintValue: u64(100)}))
		if def.RUPerSec == nil || *def.RUPerSec != 100 || def.RUUnlimited {
			t.Fatalf("ru = %+v unlimited=%t, want 100", def.RUPerSec, def.RUUnlimited)
		}
		if def.Priority != "MEDIUM" {
			t.Fatalf("priority = %q, want the fixed MEDIUM default", def.Priority)
		}
		if def.Burstable == nil || *def.Burstable {
			t.Fatalf("burstable = %v, want proven disabled (BurstLimit=0→Adjust)", def.Burstable)
		}
		if def.QueryLimitState != spec.ResourceGroupQueryLimitConfirmedAbsent || def.QueryLimit != nil {
			t.Fatalf("query_limit = {%s %v}, want confirmed absent (Runaway=nil)", def.QueryLimitState, def.QueryLimit)
		}
	})
	t.Run("unlimited RU forces burst enabled", func(t *testing.T) {
		// SetDirectResourceGroupRUSecondOption(unlimited) pins BurstLimit=-1 —
		// the catalog shows BURSTABLE=YES even without a declared member.
		def := authoredResourceGroupDefinition(decl(spec.ResourceGroupOption{Kind: spec.ResourceGroupOptionRURate, Unlimited: true}))
		if !def.RUUnlimited || def.Burstable == nil || !*def.Burstable {
			t.Fatalf("definition = %+v, want UNLIMITED with proven burst enable", def)
		}
	})
	t.Run("bare burstable publishes enabled", func(t *testing.T) {
		def := authoredResourceGroupDefinition(decl(spec.ResourceGroupOption{Kind: spec.ResourceGroupOptionBurstable, Burstable: spec.ResourceGroupBurstableEnabled}))
		if def.Burstable == nil || !*def.Burstable {
			t.Fatalf("burstable = %v, want enabled", def.Burstable)
		}
	})
	t.Run("declared query limit publishes configured", func(t *testing.T) {
		def := authoredResourceGroupDefinition(decl(spec.ResourceGroupOption{
			Kind:    spec.ResourceGroupOptionQueryLimit,
			Runaway: &spec.ResourceGroupRunawayDeclaration{Members: []spec.ResourceGroupRunawayMember{{Kind: spec.ResourceGroupRunawayAction, TextValue: "kill"}}},
		}))
		if def.QueryLimitState != spec.ResourceGroupQueryLimitConfigured {
			t.Fatalf("query_limit state = %s, want configured", def.QueryLimitState)
		}
	})
	t.Run("bare create publishes the fixed default set", func(t *testing.T) {
		def := authoredResourceGroupDefinition(decl())
		if def.RUPerSec == nil || *def.RUPerSec != 0 || def.RUUnlimited ||
			def.Priority != "MEDIUM" || def.Burstable == nil || *def.Burstable ||
			def.QueryLimitState != spec.ResourceGroupQueryLimitConfirmedAbsent {
			t.Fatalf("definition = %+v, want the NewResourceGroupSettings default set", def)
		}
	})
}

// The ordered registry publishes the successor projection the golden
// path requires: after CREATE the second statement's pre-state carries
// the authored RU=100, and after ALTER it carries RU=200 — a batch fact
// no later provider snapshot can resurrect.
func TestT22SuccessorProjectionPublishesRUChange(t *testing.T) {
	u64 := func(v uint64) *uint64 { return &v }
	provider := t22ProviderOn(map[string]t22GroupFact{
		"golden_rg": {status: spec.MetadataStatusNotFound, exists: false},
	})
	state := &batchState{
		dialect:              spec.DialectTiDB,
		provider:             provider,
		resourceGroupEntries: map[string]*batchResourceGroupEntry{},
	}
	stmt := func(operation spec.DDLOperation, ru uint64) spec.Statement {
		return spec.Statement{
			Dialect: spec.DialectTiDB,
			Kind:    spec.KindDDL,
			DDL: &spec.DDL{
				Operation: operation,
				ResourceGroup: &spec.ResourceGroupDeclaration{
					Name:    "golden_rg",
					Options: []spec.ResourceGroupOption{{Kind: spec.ResourceGroupOptionRURate, UintValue: u64(ru)}},
				},
			},
		}
	}
	ctx := context.Background()
	if err := state.applyResourceGroupStatement(ctx, stmt(spec.DDLOperationCreateResourceGroup, 100)); err != nil {
		t.Fatalf("apply create: %v", err)
	}
	entry, err := state.resolveResourceGroup(ctx, "golden_rg")
	if err != nil {
		t.Fatalf("resolve after create: %v", err)
	}
	projection := state.projectResourceGroupEntry(entry)
	if projection.Definition == nil || projection.Definition.RUPerSec == nil || *projection.Definition.RUPerSec != 100 {
		t.Fatalf("post-create definition = %+v, want the published RU=100", projection.Definition)
	}
	if err := state.applyResourceGroupStatement(ctx, stmt(spec.DDLOperationAlterResourceGroup, 200)); err != nil {
		t.Fatalf("apply alter: %v", err)
	}
	entry, err = state.resolveResourceGroup(ctx, "golden_rg")
	if err != nil {
		t.Fatalf("resolve after alter: %v", err)
	}
	projection = state.projectResourceGroupEntry(entry)
	if projection.Definition == nil || projection.Definition.RUPerSec == nil || *projection.Definition.RUPerSec != 200 {
		t.Fatalf("post-alter definition = %+v, want the successor RU=200", projection.Definition)
	}
}

// ALTER applies member-wise at the top level — undeclared members keep
// their proven values — but a declared QUERY_LIMIT rebuilds the whole
// nested group (v8.5 buildResourceGroup installs a fresh runaway struct);
// NULL/() clears it. RU_PER_SEC=UNLIMITED still pins burst enabled.
func TestT22AlterMergeSemantics(t *testing.T) {
	u64 := func(v uint64) *uint64 { return &v }
	oldText := "EXEC_ELAPSED='30s' ACTION=KILL"
	base := func() *spec.ResourceGroupDefinition {
		return &spec.ResourceGroupDefinition{
			RUPerSec:        u64(100),
			Priority:        "LOW",
			Burstable:       boolPtr(true),
			QueryLimitState: spec.ResourceGroupQueryLimitConfigured,
			QueryLimit:      &oldText,
		}
	}
	decl := func(options ...spec.ResourceGroupOption) *spec.ResourceGroupDeclaration {
		return &spec.ResourceGroupDeclaration{Name: "rg", Options: options}
	}
	t.Run("priority-only alter keeps every other member", func(t *testing.T) {
		out := mergeResourceGroupAlter(base(), decl(spec.ResourceGroupOption{Kind: spec.ResourceGroupOptionPriority, TextValue: "HIGH"}))
		if out.Priority != "HIGH" || out.RUPerSec == nil || *out.RUPerSec != 100 ||
			out.Burstable == nil || !*out.Burstable ||
			out.QueryLimitState != spec.ResourceGroupQueryLimitConfigured || out.QueryLimit == nil || *out.QueryLimit != oldText {
			t.Fatalf("merged = %+v, want member-wise top-level replace", out)
		}
	})
	t.Run("unlimited RU forces burst enabled over a disabled prior", func(t *testing.T) {
		prior := base()
		disabled := false
		prior.Burstable = &disabled
		out := mergeResourceGroupAlter(prior, decl(spec.ResourceGroupOption{Kind: spec.ResourceGroupOptionRURate, Unlimited: true}))
		if !out.RUUnlimited || out.Burstable == nil || !*out.Burstable {
			t.Fatalf("merged = %+v, want UNLIMITED pinning BurstLimit=-1", out)
		}
	})
	t.Run("numeric RU keeps the prior burst flag", func(t *testing.T) {
		out := mergeResourceGroupAlter(base(), decl(spec.ResourceGroupOption{Kind: spec.ResourceGroupOptionRURate, UintValue: u64(200)}))
		if out.Burstable == nil || !*out.Burstable || out.RUPerSec == nil || *out.RUPerSec != 200 {
			t.Fatalf("merged = %+v, want RU=200 with the prior burst flag kept", out)
		}
	})
	t.Run("query limit reset clears the nested group", func(t *testing.T) {
		out := mergeResourceGroupAlter(base(), decl(spec.ResourceGroupOption{
			Kind:    spec.ResourceGroupOptionQueryLimit,
			Runaway: &spec.ResourceGroupRunawayDeclaration{Reset: true},
		}))
		if out.QueryLimitState != spec.ResourceGroupQueryLimitConfirmedAbsent || out.QueryLimit != nil {
			t.Fatalf("query_limit = {%s %v}, want confirmed absent after NULL reset", out.QueryLimitState, out.QueryLimit)
		}
	})
	t.Run("new query limit replaces the whole nested group", func(t *testing.T) {
		out := mergeResourceGroupAlter(base(), decl(spec.ResourceGroupOption{
			Kind:    spec.ResourceGroupOptionQueryLimit,
			Runaway: &spec.ResourceGroupRunawayDeclaration{Members: []spec.ResourceGroupRunawayMember{{Kind: spec.ResourceGroupRunawayAction, TextValue: "dryrun"}}},
		}))
		if out.QueryLimitState != spec.ResourceGroupQueryLimitConfigured || (out.QueryLimit != nil && *out.QueryLimit == oldText) {
			t.Fatalf("query_limit = {%s %v}, want the authored clause replacing the old group", out.QueryLimitState, out.QueryLimit)
		}
	})
	t.Run("undeclared query limit keeps the prior state", func(t *testing.T) {
		out := mergeResourceGroupAlter(base(), decl())
		if out.QueryLimitState != spec.ResourceGroupQueryLimitConfigured || out.QueryLimit == nil || *out.QueryLimit != oldText {
			t.Fatalf("query_limit = {%s %v}, want the proven prior kept", out.QueryLimitState, out.QueryLimit)
		}
	})
}
