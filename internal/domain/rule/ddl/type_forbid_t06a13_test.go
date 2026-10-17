// Package ddl verifies the T06-A13 five ordinary CREATE TABLE type-family
// forbid policies on synthetic statements.
// input: the frozen mixed-type and safe column sets through each rule's
// registered constructor and through the real registry seam
// output: per-rule per-column attribution including the JSON and DATETIME
// exclusions, the FLOAT/DOUBLE split finding shapes, forbid=true/false,
// enabled=false, default level/params, invalid-parameter controls, the
// declaration-order control, and the CREATE-only applicability seam
// pos: domain DDL rule regression for issue #85 T06-A13
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// t06A13Columns mirrors the frozen mixed input M as extracted spec facts —
// the rendered types observed through the real extractor at baseline.
func t06A13Columns() []spec.Column {
	return []spec.Column{
		{Name: "c_blob", Type: "blob binary"},
		{Name: "c_text", Type: "text"},
		{Name: "c_json", Type: "json"},
		{Name: "c_bit", Type: "bit(1)"},
		{Name: "c_float", Type: "float"},
		{Name: "c_double", Type: "double"},
		{Name: "c_timestamp", Type: "timestamp"},
		{Name: "c_datetime", Type: "datetime"},
		{Name: "c_int", Type: "int(11)"},
	}
}

func t06A13BlockerParams() map[string]any {
	return map[string]any{"forbid": true}
}

// TestT06A13RegisteredAttribution runs the frozen mixed table through the
// real registration seam with each rule enabled alone at blocker — pinning
// that the registered predicate (not a duplicated lambda) produces exactly
// the frozen per-column attribution.
func TestT06A13RegisteredAttribution(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	statement := createTableWithColumns("t", t06A13Columns()...)
	roles := []struct {
		name        string
		ruleID      string
		wantColumns []string
	}{
		{"blob-text", ruleIDColumnBlobTextForbid, []string{"c_blob", "c_text"}},
		{"json", ruleIDColumnJSONForbid, []string{"c_json"}},
		{"bit", ruleIDColumnBitForbid, []string{"c_bit"}},
		{"float-double", ruleIDColumnFloatDoubleForbid, []string{"c_float", "c_double"}},
		{"timestamp", ruleIDColumnTimestampForbid, []string{"c_timestamp"}},
	}
	for _, role := range roles {
		role := role
		t.Run(role.name, func(t *testing.T) {
			t.Parallel()
			registry := rule.NewRegistry()
			if err := Register(registry, policy.Policy{Rules: map[string]policy.RulePolicy{
				role.ruleID: {Enabled: true, Level: rule.LevelBlocker, Params: t06A13BlockerParams()},
			}}); err != nil {
				t.Fatalf("register: %v", err)
			}
			findings, err := registry.EvaluateStatement(ctx, statement)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			if len(findings) != len(role.wantColumns) {
				t.Fatalf("findings = %+v, want %d on %v", findings, len(role.wantColumns), role.wantColumns)
			}
			for i, want := range role.wantColumns {
				finding := findings[i]
				if finding.RuleID != role.ruleID || finding.Level != rule.LevelBlocker ||
					finding.Metadata["column"] != want || finding.Metadata["table"] != "t" {
					t.Fatalf("finding[%d] = %+v, want %s blocker on %s", i, finding, role.ruleID, want)
				}
			}
		})
	}
}

// TestT06A13FindingShapes pins the two finding shapes: the shared
// columnTypeForbiddenRule emits "must not use the <family> type family"
// with table/column/type/family/charset (charset present even when empty),
// while the independent float/double rule emits "should not use <type>"
// with only table/column/type — the shapes are never unified.
func TestT06A13FindingShapes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	statement := createTableWithColumns("t", t06A13Columns()...)
	run := func(t *testing.T, ruleID string) []rule.Finding {
		t.Helper()
		registry := rule.NewRegistry()
		if err := Register(registry, policy.Policy{Rules: map[string]policy.RulePolicy{
			ruleID: {Enabled: true, Level: rule.LevelBlocker, Params: t06A13BlockerParams()},
		}}); err != nil {
			t.Fatalf("register: %v", err)
		}
		findings, err := registry.EvaluateStatement(ctx, statement)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		return findings
	}

	families := map[string]string{
		ruleIDColumnBlobTextForbid:  "blob/text",
		ruleIDColumnJSONForbid:      "json",
		ruleIDColumnBitForbid:       "bit",
		ruleIDColumnTimestampForbid: "timestamp",
	}
	for ruleID, family := range families {
		for _, finding := range run(t, ruleID) {
			column := finding.Metadata["column"]
			want := `column "` + column.(string) + `" must not use the ` + family + ` type family`
			if finding.Message != want {
				t.Fatalf("%s message = %q, want %q", ruleID, finding.Message, want)
			}
			if finding.Metadata["family"] != family {
				t.Fatalf("%s family = %v, want %s", ruleID, finding.Metadata["family"], family)
			}
			if _, ok := finding.Metadata["charset"]; !ok {
				t.Fatalf("%s metadata %v lacks the charset key (empty string is a real value)",
					ruleID, finding.Metadata)
			}
			if finding.Metadata["type"] == nil || finding.Metadata["table"] != "t" {
				t.Fatalf("%s metadata = %+v, want type/table bound", ruleID, finding.Metadata)
			}
		}
	}
	floats := run(t, ruleIDColumnFloatDoubleForbid)
	wants := []struct{ column, typ string }{{"c_float", "float"}, {"c_double", "double"}}
	for i, want := range wants {
		finding := floats[i]
		if finding.Message != `column "`+want.column+`" should not use `+want.typ ||
			finding.Metadata["type"] != want.typ {
			t.Fatalf("float finding[%d] = %+v, want %s %s", i, finding, want.column, want.typ)
		}
		if _, ok := finding.Metadata["family"]; ok {
			t.Fatalf("float finding[%d] metadata = %+v carries a family key — wrong shape", i, finding.Metadata)
		}
		if _, ok := finding.Metadata["charset"]; ok {
			t.Fatalf("float finding[%d] metadata = %+v carries a charset key — wrong shape", i, finding.Metadata)
		}
		if len(finding.Metadata) != 3 {
			t.Fatalf("float finding[%d] metadata = %+v, want exactly table/column/type", i, finding.Metadata)
		}
	}
}

// TestT06A13ForbidDisabledAndDefaults pins the three loaded states per rule:
// forbid=true reports, forbid=false keeps the rule loaded but inapplicable,
// enabled=false never registers — and an omitted forbid or level falls back
// to the product defaults (forbid=true, warning), never the isolated
// profiles' blocker.
func TestT06A13ForbidDisabledAndDefaults(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	statement := createTableWithColumns("t", t06A13Columns()...)
	ruleIDs := []string{
		ruleIDColumnBlobTextForbid, ruleIDColumnJSONForbid, ruleIDColumnBitForbid,
		ruleIDColumnFloatDoubleForbid, ruleIDColumnTimestampForbid,
	}
	for _, ruleID := range ruleIDs {
		ruleID := ruleID
		t.Run(ruleID+"/forbid-false", func(t *testing.T) {
			t.Parallel()
			registry := rule.NewRegistry()
			if err := Register(registry, policy.Policy{Rules: map[string]policy.RulePolicy{
				ruleID: {Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{"forbid": false}},
			}}); err != nil {
				t.Fatalf("register: %v", err)
			}
			findings, err := registry.EvaluateStatement(ctx, statement)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			if len(findings) != 0 {
				t.Fatalf("forbid=false findings = %+v, want none — loaded but inapplicable", findings)
			}
			if !registry.Contains(ruleID) {
				t.Fatalf("forbid=false rule %s must still be loaded", ruleID)
			}
		})
		t.Run(ruleID+"/disabled", func(t *testing.T) {
			t.Parallel()
			registry := rule.NewRegistry()
			if err := Register(registry, policy.Policy{Rules: map[string]policy.RulePolicy{
				ruleID: {Enabled: false},
			}}); err != nil {
				t.Fatalf("register: %v", err)
			}
			if registry.Contains(ruleID) {
				t.Fatalf("disabled rule %s must not be loaded", ruleID)
			}
			findings, err := registry.EvaluateStatement(ctx, statement)
			if err != nil || len(findings) != 0 {
				t.Fatalf("disabled findings = %+v err = %v, want none", findings, err)
			}
		})
		t.Run(ruleID+"/defaults", func(t *testing.T) {
			t.Parallel()
			registry := rule.NewRegistry()
			if err := Register(registry, policy.Policy{Rules: map[string]policy.RulePolicy{
				ruleID: {Enabled: true},
			}}); err != nil {
				t.Fatalf("register: %v", err)
			}
			findings, err := registry.EvaluateStatement(ctx, statement)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			// Default forbid=true fires at the default warning level.
			wantCounts := map[string]int{
				ruleIDColumnBlobTextForbid: 2, ruleIDColumnJSONForbid: 1,
				ruleIDColumnBitForbid: 1, ruleIDColumnFloatDoubleForbid: 2,
				ruleIDColumnTimestampForbid: 1,
			}
			if len(findings) != wantCounts[ruleID] {
				t.Fatalf("default findings = %+v, want %d", findings, wantCounts[ruleID])
			}
			for _, finding := range findings {
				if finding.Level != rule.LevelWarning {
					t.Fatalf("default finding level = %s, want warning — never the isolated blocker", finding.Level)
				}
			}
		})
	}
}

// TestT06A13InvalidForbidParameter pins boolParam's strict typing on the
// forbid parameter for all five rules: a non-bool value fails construction,
// never coerces.
func TestT06A13InvalidForbidParameter(t *testing.T) {
	t.Parallel()
	for _, ruleID := range []string{
		ruleIDColumnBlobTextForbid, ruleIDColumnJSONForbid, ruleIDColumnBitForbid,
		ruleIDColumnFloatDoubleForbid, ruleIDColumnTimestampForbid,
	} {
		ruleID := ruleID
		for _, bad := range []any{"yes", 1, 0.0} {
			t.Run(ruleID, func(t *testing.T) {
				registry := rule.NewRegistry()
				err := Register(registry, policy.Policy{Rules: map[string]policy.RulePolicy{
					ruleID: {Enabled: true, Params: map[string]any{"forbid": bad}},
				}})
				if err == nil {
					t.Fatalf("forbid=%v (%T) must fail rule construction", bad, bad)
				}
			})
		}
	}
}

// TestT06A13ExclusionBoundaries pins the two carved-out boundaries the
// forbid sets must not swallow: the BLOB/TEXT family excludes JSON (the
// A12 isBlobTextLike exemption set contains json — this rule's registered
// predicate subtracts it back out), and TIMESTAMP excludes DATETIME.
func TestT06A13ExclusionBoundaries(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	registry := rule.NewRegistry()
	if err := Register(registry, policy.Policy{Rules: map[string]policy.RulePolicy{
		ruleIDColumnBlobTextForbid:  {Enabled: true, Level: rule.LevelBlocker, Params: t06A13BlockerParams()},
		ruleIDColumnTimestampForbid: {Enabled: true, Level: rule.LevelBlocker, Params: t06A13BlockerParams()},
	}}); err != nil {
		t.Fatalf("register: %v", err)
	}
	statement := createTableWithColumns("t",
		spec.Column{Name: "only_json", Type: "json"},
		spec.Column{Name: "only_datetime", Type: "datetime"})
	findings, err := registry.EvaluateStatement(ctx, statement)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("json-only + datetime-only findings = %+v, want none", findings)
	}
}

// TestT06A13ReorderedColumns pins that finding order follows declaration
// order and attribution follows the declared column — moving matching
// columns behind non-matching ones never changes which column owns a
// finding.
func TestT06A13ReorderedColumns(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	roles := []struct {
		ruleID      string
		wantColumns []string
	}{
		{ruleIDColumnBlobTextForbid, []string{"mid_text", "tail_blob"}},
		{ruleIDColumnFloatDoubleForbid, []string{"mid_double", "tail_float"}},
	}
	for _, role := range roles {
		role := role
		t.Run(role.ruleID, func(t *testing.T) {
			t.Parallel()
			registry := rule.NewRegistry()
			if err := Register(registry, policy.Policy{Rules: map[string]policy.RulePolicy{
				role.ruleID: {Enabled: true, Level: rule.LevelBlocker, Params: t06A13BlockerParams()},
			}}); err != nil {
				t.Fatalf("register: %v", err)
			}
			statement := createTableWithColumns("t",
				spec.Column{Name: "lead_int", Type: "int(11)"},
				spec.Column{Name: "mid_text", Type: "text"},
				spec.Column{Name: "mid_double", Type: "double"},
				spec.Column{Name: "tail_blob", Type: "blob binary"},
				spec.Column{Name: "tail_float", Type: "float"})
			findings, err := registry.EvaluateStatement(ctx, statement)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			if len(findings) != len(role.wantColumns) {
				t.Fatalf("findings = %+v, want %d", findings, len(role.wantColumns))
			}
			for i, want := range role.wantColumns {
				if findings[i].Metadata["column"] != want {
					t.Fatalf("finding[%d] column = %v, want %s", i, findings[i].Metadata["column"], want)
				}
			}
		})
	}
}

// TestT06A13CreateTableOnlySeam pins the rules' applicability to CREATE
// TABLE columns: an ALTER statement carrying the same types does not fall
// under these rules — ALTER-side type governance stays out of scope.
func TestT06A13CreateTableOnlySeam(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	alter := spec.Statement{
		Kind: spec.KindDDL,
		DDL: &spec.DDL{
			Operation: spec.DDLOperationAlterTable,
			Table:     &spec.Table{Name: "t"},
			Alter: []spec.Alter{{Action: "modify_column", Name: "c",
				Column: &spec.AlterColumn{Definition: &spec.Column{Name: "c", Type: "text"}}}},
		},
	}
	for _, ruleID := range []string{
		ruleIDColumnBlobTextForbid, ruleIDColumnJSONForbid, ruleIDColumnBitForbid,
		ruleIDColumnFloatDoubleForbid, ruleIDColumnTimestampForbid,
	} {
		registry := rule.NewRegistry()
		if err := Register(registry, policy.Policy{Rules: map[string]policy.RulePolicy{
			ruleID: {Enabled: true, Level: rule.LevelBlocker, Params: t06A13BlockerParams()},
		}}); err != nil {
			t.Fatalf("register %s: %v", ruleID, err)
		}
		findings, err := registry.EvaluateStatement(ctx, alter)
		if err != nil || len(findings) != 0 {
			t.Fatalf("%s alter findings = %+v err = %v, want none", ruleID, findings, err)
		}
	}
}
