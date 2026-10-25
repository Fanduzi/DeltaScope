// Package policy verifies FK-forbid naming suppression stays a named fact.
package policy

import "testing"

func TestDefaultPolicySuppressesForeignKeyNaming(t *testing.T) {
	t.Parallel()
	p := Default()
	for _, ruleID := range ForeignKeyNamingRuleIDs {
		ruleCfg, ok := p.Rules[ruleID]
		if !ok || !ruleCfg.Enabled {
			t.Fatalf("%s must remain enabled in Default Policy (suppressed, not missing)", ruleID)
		}
		if !SuppressesForeignKeyNaming(p, ruleID) {
			t.Fatalf("Default Policy must suppress %s while %s is enabled", ruleID, ForeignKeyForbidRuleID)
		}
	}
}

func TestSuppressesForeignKeyNamingWhenForbidDisabled(t *testing.T) {
	t.Parallel()
	p := Default()
	forbid := p.Rules[ForeignKeyForbidRuleID]
	forbid.Enabled = false
	p.Rules[ForeignKeyForbidRuleID] = forbid
	for _, ruleID := range ForeignKeyNamingRuleIDs {
		if SuppressesForeignKeyNaming(p, ruleID) {
			t.Fatalf("did not expect suppression of %s when forbid is disabled", ruleID)
		}
	}
}

func TestSuppressesForeignKeyNamingWhenForbidParamFalse(t *testing.T) {
	t.Parallel()
	p := Default()
	forbid := p.Rules[ForeignKeyForbidRuleID]
	forbid.Params = map[string]any{"forbid": false}
	p.Rules[ForeignKeyForbidRuleID] = forbid
	if SuppressesForeignKeyNaming(p, ForeignKeyNamingRuleIDs[0]) {
		t.Fatal("did not expect suppression when forbid param is false")
	}
}

func TestSuppressesForeignKeyNamingIgnoresOtherRules(t *testing.T) {
	t.Parallel()
	if SuppressesForeignKeyNaming(Default(), "dml.where.require") {
		t.Fatal("dml.where.require must not use fk_forbid suppression")
	}
}
