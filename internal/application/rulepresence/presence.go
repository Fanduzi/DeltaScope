// Package rulepresence computes Catalog, Default Policy, Loaded, and Suppression for one rule.
// input: a shipped rule ID and the effective policy that registration would use
// output: Presence naming the four facts without collapsing them
// pos: application module that config status and discovery share
// note: if this file changes, update this header and module README.md.
package rulepresence

import (
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule/catalog"
	ddlrules "github.com/Fanduzi/DeltaScope/internal/domain/rule/ddl"
	dmlrules "github.com/Fanduzi/DeltaScope/internal/domain/rule/dml"
)

// Presence is the four named facts for one rule ID.
type Presence struct {
	InCatalog         bool
	InDefaultPolicy   bool
	Loaded            bool
	SuppressionReason string
	SuppressionBy     string
}

// Of answers Catalog, Default Policy, Loaded, and Suppression for one rule.
func Of(ruleID string, cfg policy.Policy) (Presence, error) {
	_, inCatalog := catalog.Lookup(ruleID)
	_, inDefault := policy.Default().Rules[ruleID]
	presence := Presence{
		InCatalog:       inCatalog,
		InDefaultPolicy: inDefault,
	}
	if policy.SuppressesForeignKeyNaming(cfg, ruleID) {
		presence.SuppressionReason = policy.ForeignKeyNamingSuppressionReason
		presence.SuppressionBy = policy.ForeignKeyForbidRuleID
	}
	registry := rule.NewRegistry()
	if err := ddlrules.Register(registry, cfg); err != nil {
		return Presence{}, fmt.Errorf("register ddl rules: %w", err)
	}
	if err := dmlrules.Register(registry, cfg); err != nil {
		return Presence{}, fmt.Errorf("register dml rules: %w", err)
	}
	presence.Loaded = registry.Contains(ruleID)
	return presence, nil
}
