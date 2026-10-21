// Package deltascope verifies T07 snapshot ownership at the SDK seam: the
// publicMetadataProvider boundary deep-copies provider-owned snapshots so
// generated/default-expression facts can never alias into request state.
// input: a shared provider-owned TableSnapshot with T07 column facts
// output: proof that audit mutations never reach the provider object across requests and races
// pos: public SDK ownership tests for issue #86 T07
// note: if this file changes, update this header and module README.md.
package deltascope

import (
	"context"
	"sync"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// t07OwnerProvider returns one shared provider-owned snapshot; if the SDK
// boundary aliased it, request-side state mutations would corrupt it.
type t07OwnerProvider struct {
	snapshot *spec.TableSnapshot
}

func (p *t07OwnerProvider) LoadInstanceFacts(context.Context, Dialect, string) (*spec.InstanceFacts, error) {
	return &spec.InstanceFacts{Version: "8.4.10"}, nil
}

func (p *t07OwnerProvider) LoadTableSnapshot(context.Context, Dialect, string, string) (*spec.TableSnapshot, error) {
	return p.snapshot, nil
}

func t07OwnerSnapshot() *spec.TableSnapshot {
	return &spec.TableSnapshot{
		Exists: true,
		Schema: "golden",
		Table:  &spec.Table{Schema: "golden", Name: "t"},
		Columns: []spec.Column{
			{Name: "a", Type: "int", NotNull: true, Visibility: spec.ColumnVisibilityVisible},
			{
				Name: "b", Type: "int",
				Visibility: spec.ColumnVisibilityVisible,
				Generated: &spec.GeneratedColumn{
					Storage: spec.GeneratedStorageStored,
					Expression: spec.ExpressionFacts{
						Known:       true,
						References:  []spec.ColumnReference{{Name: "a"}},
						Fingerprint: "b-fp",
					},
				},
				DefaultExpression: &spec.ExpressionFacts{
					Known:      true,
					References: []spec.ColumnReference{{Name: "a"}},
				},
			},
			{
				Name: "d", Type: "varchar(16)",
				Visibility:     spec.ColumnVisibilityVisible,
				HasDefault:     true,
				DefaultValue:   "'x'",
				DefaultLiteral: &spec.LiteralDefaultFacts{Kind: spec.LiteralDefaultString, Value: "x"},
			},
		},
		Indexes:     []spec.Index{},
		Constraints: []spec.Constraint{},
	}
}

func t07OwnerFingerprint(p *t07OwnerProvider) any {
	b := p.snapshot.Columns[1]
	d := p.snapshot.Columns[2]
	return struct {
		storage    string
		genRef     string
		defaultRef string
		genFP      string
		litKind    string
		litValue   string
		litPtr     *spec.LiteralDefaultFacts
	}{b.Generated.Storage, b.Generated.Expression.References[0].Name,
		b.DefaultExpression.References[0].Name, b.Generated.Expression.Fingerprint,
		d.DefaultLiteral.Kind, d.DefaultLiteral.Value, d.DefaultLiteral}
}

// TestT07ProviderSnapshotOwnership proves request-side ordered-state work on
// T07 facts cannot mutate the provider's snapshot — the setter path replaces
// default facts on a deep clone, and the dependency assessment only reads.
func TestT07ProviderSnapshotOwnership(t *testing.T) {
	t.Parallel()
	provider := &t07OwnerProvider{snapshot: t07OwnerSnapshot()}
	want := t07OwnerFingerprint(provider)

	// Two requests against the same provider object: the first SET DEFAULT
	// + DROP/change activity would corrupt provider-owned slice/pointer
	// fields if the seam shallow-copied them.
	for i := 0; i < 2; i++ {
		result, err := Audit(context.Background(), Request{
			SQL:              "ALTER TABLE t ALTER COLUMN a SET DEFAULT (1 + 2); ALTER TABLE t DROP COLUMN a",
			Dialect:          DialectMySQL,
			Schema:           "golden",
			MetadataProvider: provider,
		})
		if err != nil {
			t.Fatalf("audit %d: %v", i, err)
		}
		if len(result.Statements) != 2 {
			t.Fatalf("audit %d statements = %d", i, len(result.Statements))
		}
		if got := t07OwnerFingerprint(provider); got != want {
			t.Fatalf("audit %d mutated provider-owned facts: %+v", i, provider.snapshot.Columns[1])
		}
	}
}

// TestT07ProviderSnapshotOwnershipConcurrent races audits against the shared
// provider snapshot; -race catches any aliased read/write on the T07 fields.
func TestT07ProviderSnapshotOwnershipConcurrent(t *testing.T) {
	t.Parallel()
	provider := &t07OwnerProvider{snapshot: t07OwnerSnapshot()}
	want := t07OwnerFingerprint(provider)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := Audit(context.Background(), Request{
				SQL:              "ALTER TABLE t ALTER COLUMN a SET DEFAULT (1 + 2)",
				Dialect:          DialectMySQL,
				Schema:           "golden",
				MetadataProvider: provider,
			})
			if err != nil {
				t.Errorf("audit: %v", err)
			}
		}()
	}
	wg.Wait()
	if got := t07OwnerFingerprint(provider); got != want {
		t.Fatalf("concurrent audits mutated provider-owned facts: %+v", provider.snapshot.Columns[1])
	}
}
