// Package rulepresence verifies Catalog vs Default Policy vs Loaded vs Suppression.
// input: shipped rule IDs and Default Policy
// output: Loaded matches registration; FK naming is suppressed not missing
// pos: application tests at the Presence seam
// note: if this file changes, update this header and module README.md.
package rulepresence

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
)

func TestOfFKNamingIsSuppressedNotLoaded(t *testing.T) {
	t.Parallel()

	presence, err := Of("ddl.constraint.foreign_key.name.prefix.require", policy.Default())
	if err != nil {
		t.Fatalf("Of: %v", err)
	}
	if !presence.InCatalog || !presence.InDefaultPolicy {
		t.Fatalf("expected catalog and default policy, got %#v", presence)
	}
	if presence.Loaded {
		t.Fatal("expected FK naming not Loaded under fk_forbid")
	}
	if presence.SuppressionReason != policy.ForeignKeyNamingSuppressionReason || presence.SuppressionBy != policy.ForeignKeyForbidRuleID {
		t.Fatalf("suppression: %#v", presence)
	}
}

func TestOfWhereRequireIsLoaded(t *testing.T) {
	t.Parallel()

	presence, err := Of("dml.where.require", policy.Default())
	if err != nil {
		t.Fatalf("Of: %v", err)
	}
	if !presence.Loaded || presence.SuppressionReason != "" {
		t.Fatalf("expected loaded without suppression, got %#v", presence)
	}
}

func TestOfImpactRuleIsCatalogOnly(t *testing.T) {
	t.Parallel()

	presence, err := Of("dml.impact.estimate", policy.Default())
	if err != nil {
		t.Fatalf("Of: %v", err)
	}
	if !presence.InCatalog || presence.InDefaultPolicy || presence.Loaded {
		t.Fatalf("expected catalog-only not loaded, got %#v", presence)
	}
}
