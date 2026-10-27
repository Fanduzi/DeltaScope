// T13-R1: the public provider boundary deep-copies the partition catalog
// facts — pointer and slice fields on TableSnapshot can never alias
// provider-owned memory, and an observed-empty member set survives the
// clone without collapsing into unobserved.
package deltascope

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestT13R1PublicProviderClonePreservesPartitionFacts(t *testing.T) {
	t.Parallel()
	partitioned := true
	partitionType := "range"
	source := &spec.TableSnapshot{
		Table:          &spec.Table{Schema: "golden", Name: "tp"},
		Partitioned:    &partitioned,
		PartitionType:  &partitionType,
		PartitionNames: []string{"p0", "p1"},
	}
	sourceEmpty := &spec.TableSnapshot{
		Table:          &spec.Table{Schema: "golden", Name: "plain"},
		PartitionNames: []string{},
	}

	provider := publicMetadataProvider{provider: &fakeMetadataProvider{snapshot: source}}
	mapped, err := provider.LoadTableSnapshot(context.Background(), spec.DialectMySQL, "golden", "tp")
	if err != nil {
		t.Fatalf("clone: %v", err)
	}
	if mapped.Partitioned == source.Partitioned {
		t.Fatal("cloned Partitioned aliases the provider-owned pointer")
	}
	if mapped.PartitionType == source.PartitionType {
		t.Fatal("cloned PartitionType aliases the provider-owned pointer")
	}
	mapped.PartitionNames[0] = "mutated"
	if source.PartitionNames[0] != "p0" {
		t.Fatal("cloned PartitionNames aliased the provider-owned member list")
	}

	provider = publicMetadataProvider{provider: &fakeMetadataProvider{snapshot: sourceEmpty}}
	mappedEmpty, err := provider.LoadTableSnapshot(context.Background(), spec.DialectMySQL, "golden", "plain")
	if err != nil {
		t.Fatalf("clone empty: %v", err)
	}
	if mappedEmpty.PartitionNames == nil {
		t.Fatal("observed-empty PartitionNames must survive the clone as non-nil empty, not collapse to nil")
	}
}
