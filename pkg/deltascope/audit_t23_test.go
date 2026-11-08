package deltascope

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// audit_t23_test.go pins the T23 first path through the public Audit
// seam with a stub metadata provider — the same provider/interface
// contract the real mysql adapter implements.

type t23FactsProvider struct {
	facts *InstanceFacts
}

func (p *t23FactsProvider) LoadInstanceFacts(context.Context, Dialect, string) (*InstanceFacts, error) {
	if p.facts != nil {
		facts := *p.facts
		return &facts, nil
	}
	return &InstanceFacts{Version: "8.0.11-TiDB-v8.5.0"}, nil
}

func (p *t23FactsProvider) LoadTableSnapshot(context.Context, Dialect, string, string) (*TableSnapshot, error) {
	return nil, nil
}

func (p *t23FactsProvider) ResolveObject(_ context.Context, _ Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	return &spec.ObjectSnapshot{Schema: req.Schema, Type: req.Type, Name: req.Name, Status: spec.MetadataStatusNotFound}, nil
}

const t23FirstPathSQL = "CREATE TABLE t (id BIGINT PRIMARY KEY AUTO_RANDOM);"

// Sufficient clustered premises let the canonical first path complete:
// no validity blocker and no evidence gap on the AUTO_RANDOM statement.
func TestT23AuditFirstPathCompletesThroughPublicSeam(t *testing.T) {
	t.Parallel()
	provider := &t23FactsProvider{facts: &InstanceFacts{
		Version:                  "8.0.11-TiDB-v8.5.0",
		TiDBEnableClusteredIndex: "on",
		TiDBAlterPrimaryKey:      true,
		TiDBAlterPrimaryKeyKnown: true,
	}}
	result, err := Audit(context.Background(), Request{
		SQL:              t23FirstPathSQL,
		Dialect:          DialectTiDB,
		Schema:           "app",
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("statements = %d", len(result.Statements))
	}
	stmt := result.Statements[0]
	for _, finding := range stmt.Findings {
		if finding.RuleID == "ddl.tidb.table.auto_random.valid" {
			t.Fatalf("first path must not block, got %+v", finding)
		}
	}
	for _, gap := range stmt.EvidenceGaps {
		if gap.RuleID == "ddl.tidb.table.auto_random.valid" {
			t.Fatalf("sufficient premises must not gap, got %+v", gap)
		}
	}
}

// Without the clustered-mode observation the statement stays unverified
// with the minimal instance-fact gap — never a fabricated verdict.
func TestT23AuditUnobservedClusteredModeStaysUnverified(t *testing.T) {
	t.Parallel()
	result, err := Audit(context.Background(), Request{
		SQL:              t23FirstPathSQL,
		Dialect:          DialectTiDB,
		Schema:           "app",
		MetadataProvider: &t23FactsProvider{},
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	stmt := result.Statements[0]
	var gap *EvidenceGap
	for i := range stmt.EvidenceGaps {
		if stmt.EvidenceGaps[i].RuleID == "ddl.tidb.table.auto_random.valid" {
			gap = &stmt.EvidenceGaps[i]
		}
	}
	if gap == nil || len(gap.RequiredFacts) != 1 ||
		gap.RequiredFacts[0] != "instance.tidb_enable_clustered_index" {
		t.Fatalf("unobserved mode must owe exactly the clustered-mode fact, got %+v", stmt.EvidenceGaps)
	}
}
