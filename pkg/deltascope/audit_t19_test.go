package deltascope

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type t19SnapshotProvider struct {
	snapshot *TableSnapshot
}

func (p t19SnapshotProvider) LoadInstanceFacts(context.Context, Dialect, string) (*InstanceFacts, error) {
	return &InstanceFacts{Version: "8.0.11-TiDB-v8.5.0"}, nil
}

func (p t19SnapshotProvider) LoadTableSnapshot(context.Context, Dialect, string, string) (*TableSnapshot, error) {
	return p.snapshot, nil
}

// T19: provider-owned TTL member state must cross the public adapter by
// value, including the Enabled pointer that carries OFF-vs-unobserved.
func TestT19PublicProviderSnapshotCloneCopiesTTLEnabled(t *testing.T) {
	t.Parallel()
	enabled := false
	source := &TableSnapshot{
		Exists:   true,
		Table:    &Table{Name: "t"},
		TTLKnown: true,
		TTL: &spec.TTLConfig{
			TimeColumn: "created_at",
			Enabled:    &enabled,
		},
	}
	provider := publicMetadataProvider{provider: t19SnapshotProvider{snapshot: source}}
	got, err := provider.LoadTableSnapshot(context.Background(), spec.DialectTiDB, "app", "t")
	if err != nil {
		t.Fatalf("LoadTableSnapshot: %v", err)
	}
	if got == nil || got.TTL == nil || got.TTL.Enabled == nil {
		t.Fatalf("snapshot TTL = %#v, want enabled member", got)
	}
	if got.TTL.Enabled == source.TTL.Enabled {
		t.Fatalf("public snapshot aliases provider Enabled pointer: %p", got.TTL.Enabled)
	}
	*got.TTL.Enabled = true
	if *source.TTL.Enabled {
		t.Fatal("public snapshot mutation leaked into provider-owned TTL state")
	}
}
