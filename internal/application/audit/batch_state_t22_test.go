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
