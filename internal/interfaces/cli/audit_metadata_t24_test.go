package cli

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// T24-R13 D1 regression: the CLI metadata wrapper must forward the optional
// LoadMySQLResourceGroupFacts channel — the native run proved that without
// it the whole MySQL resource-group fact preload is silently skipped on the
// real CLI path (uuid never observed, online evidence never binds).

type t24FakeClient struct{ withRG bool }

func (t24FakeClient) LoadInstanceFacts(context.Context, spec.Dialect, string) (*spec.InstanceFacts, error) {
	return &spec.InstanceFacts{}, nil
}
func (t24FakeClient) LoadTableSnapshot(context.Context, spec.Dialect, string, string) (*spec.TableSnapshot, error) {
	return nil, nil
}
func (t24FakeClient) DetectDialect(context.Context) (spec.Dialect, error) {
	return spec.DialectMySQL, nil
}
func (t24FakeClient) FindSchemasForTable(context.Context, string) ([]string, error) {
	return nil, nil
}
func (t24FakeClient) Close() error { return nil }

func (c t24FakeClient) LoadMySQLResourceGroupFacts(ctx context.Context, dialect spec.Dialect) (*spec.InstanceFacts, error) {
	if !c.withRG {
		return nil, nil
	}
	return &spec.InstanceFacts{MySQLServerUUID: "srv-uuid-cli", MySQLServerUUIDKnown: true}, nil
}

// t24NoRGClient deliberately lacks the optional channel — a plain struct
// so the interface assertion inside the forwarder actually fails.
type t24NoRGClient struct{}

func (t24NoRGClient) LoadInstanceFacts(context.Context, spec.Dialect, string) (*spec.InstanceFacts, error) {
	return &spec.InstanceFacts{}, nil
}
func (t24NoRGClient) LoadTableSnapshot(context.Context, spec.Dialect, string, string) (*spec.TableSnapshot, error) {
	return nil, nil
}
func (t24NoRGClient) DetectDialect(context.Context) (spec.Dialect, error) {
	return spec.DialectMySQL, nil
}
func (t24NoRGClient) FindSchemasForTable(context.Context, string) ([]string, error) {
	return nil, nil
}
func (t24NoRGClient) Close() error { return nil }

func TestCLIMetadataProviderForwardsResourceGroupFacts(t *testing.T) {
	wrapped := cliMetadataProvider{client: t24FakeClient{withRG: true}}
	facts, err := wrapped.LoadMySQLResourceGroupFacts(context.Background(), spec.DialectMySQL)
	if err != nil {
		t.Fatalf("forward errored: %v", err)
	}
	if facts == nil || !facts.MySQLServerUUIDKnown || facts.MySQLServerUUID != "srv-uuid-cli" {
		t.Fatalf("forwarded facts lost: %+v", facts)
	}
}

func TestCLIMetadataProviderWithoutResourceGroupChannel(t *testing.T) {
	// A client lacking the optional channel degrades cleanly, never errors.
	wrapped := cliMetadataProvider{client: t24NoRGClient{}}
	facts, err := wrapped.LoadMySQLResourceGroupFacts(context.Background(), spec.DialectMySQL)
	if err != nil {
		t.Fatalf("absent channel errored: %v", err)
	}
	if facts != nil {
		t.Fatalf("absent channel must return nil facts, got %+v", facts)
	}
}
