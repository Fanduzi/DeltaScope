package deltascope

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// t26PremiseProvider is the public-contract stub for
// T26-SERVER-DROP-EXECUTION-01: it implements the optional
// MySQLServerDropPrivilegeProvider seam plus the object resolver, and
// records the observation it returns so the test can assert verbatim
// forwarding and defensive cloning.
type t26PremiseProvider struct {
	version        string
	servers        map[string]bool
	observation    *MySQLServerDropPrivilegeObservation
	privilegeErr   error
	privilegeCalls int
}

func (p *t26PremiseProvider) LoadInstanceFacts(context.Context, Dialect, string) (*InstanceFacts, error) {
	return &InstanceFacts{Version: p.version}, nil
}

func (p *t26PremiseProvider) LoadTableSnapshot(context.Context, Dialect, string, string) (*TableSnapshot, error) {
	return nil, nil
}

func (p *t26PremiseProvider) ResolveObject(_ context.Context, dialect spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	snapshot := &spec.ObjectSnapshot{Type: req.Type, Name: req.Name, Status: spec.MetadataStatusUnavailable}
	if req.Type != "mysql_server" || dialect != spec.DialectMySQL {
		return snapshot, nil
	}
	state := &spec.ServerState{Name: req.Name, Read: true}
	if p.servers[req.Name] {
		state.Exists, state.Status = true, spec.MetadataStatusConfirmed
		snapshot.Status, snapshot.Exists = spec.MetadataStatusConfirmed, true
	}
	snapshot.Server = state
	return snapshot, nil
}

func (p *t26PremiseProvider) LoadMySQLServerDropPrivilege(_ context.Context, dialect Dialect) (*MySQLServerDropPrivilegeObservation, error) {
	p.privilegeCalls++
	if dialect != DialectMySQL {
		return nil, nil
	}
	if p.privilegeErr != nil {
		return nil, p.privilegeErr
	}
	return p.observation, nil
}

// The public mode field wires through to the application request: the
// registered token reaches the loader once and the consumable witness
// discharges the premise; an unregistered token fails closed at the
// boundary; and an unselected mode never invokes the seam.
func TestT26PrivilegeModePublicSurface(t *testing.T) {
	observation := &MySQLServerDropPrivilegeObservation{
		Account:    "edge@%",
		GrantLines: []string{"GRANT SUPER ON *.* TO 'edge'@'%'"},
	}
	provider := &t26PremiseProvider{version: "8.0.36", servers: map[string]bool{"s1": true}, observation: observation}
	result, err := Audit(context.Background(), Request{
		SQL: "DROP SERVER s1", Dialect: DialectMySQL, Schema: "golden",
		MetadataProvider: provider, PrivilegeAuditMode: PrivilegeAuditModeObservedAccountNewSession,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if provider.privilegeCalls != 1 {
		t.Fatalf("loader calls = %d, want exactly one", provider.privilegeCalls)
	}
	if result.Verdict != VerdictPass {
		t.Fatalf("consumable proven drop must pass: %+v", result.Verdict)
	}
	if len(result.Statements) != 1 || len(result.Statements[0].EvidenceGaps) != 0 {
		t.Fatalf("proven arm owes no gaps: %+v", result.Statements)
	}
	// The caller-owned observation never aliases into request
	// evidence — read the copied result through the provider seam,
	// mutate the original, and assert the copy is unchanged.
	wrapped := publicMetadataProvider{provider: provider}
	copied, err := wrapped.LoadMySQLServerDropPrivilege(context.Background(), spec.DialectMySQL)
	if err != nil || copied == nil {
		t.Fatalf("seam read: %v", err)
	}
	observation.GrantLines[0] = "GRANT USAGE ON *.* TO 'edge'@'%'"
	if copied.GrantLines[0] != "GRANT SUPER ON *.* TO 'edge'@'%'" {
		t.Fatalf("cloned observation aliased the caller slice: %q", copied.GrantLines[0])
	}

	// Mode unset keeps the premise owed and the loader unfired.
	provider = &t26PremiseProvider{version: "8.0.36", servers: map[string]bool{"s1": true}, observation: observation}
	result, err = Audit(context.Background(), Request{
		SQL: "DROP SERVER s1", Dialect: DialectMySQL, Schema: "golden", MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if provider.privilegeCalls != 0 {
		t.Fatalf("unselected mode must never invoke the loader")
	}
	if result.Verdict != VerdictReview {
		t.Fatalf("unselected mode keeps the review floor: %+v", result.Verdict)
	}

	// An unregistered token fails closed.
	_, err = Audit(context.Background(), Request{
		SQL: "DROP SERVER s1", Dialect: DialectMySQL, PrivilegeAuditMode: PrivilegeAuditMode("bogus_mode"),
	})
	if err == nil || !errors.Is(err, ErrInvalidPrivilegeMode) {
		t.Fatalf("unregistered mode must fail closed: %v", err)
	}
}

// A provider failure crosses the public boundary bounded: fixed text
// outside, the original cause reachable through errors.Is — and the
// raw account and grant text never reach the public error.
func TestT26PrivilegePublicErrorBoundary(t *testing.T) {
	marker := errors.New("priv-marker-account-edge-pwd")
	provider := &t26PremiseProvider{version: "8.0.36", servers: map[string]bool{"s1": true}, privilegeErr: marker}
	_, err := Audit(context.Background(), Request{
		SQL: "DROP SERVER s1", Dialect: DialectMySQL, Schema: "golden",
		MetadataProvider: provider, PrivilegeAuditMode: PrivilegeAuditModeObservedAccountNewSession,
	})
	if err == nil {
		t.Fatalf("privilege read failure must still fail the audit")
	}
	if !errors.Is(err, marker) {
		t.Fatalf("public error chain lost the loader cause: %v", err)
	}
	for _, protected := range []string{marker.Error(), "edge@%", "GRANT SUPER", "s1"} {
		if strings.Contains(err.Error(), protected) {
			t.Fatalf("public error text echoes protected content %q: %v", protected, err)
		}
	}
}
