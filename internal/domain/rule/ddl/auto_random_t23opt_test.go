// Package ddl verifies the T23 TiDB create-time table-option validity rule.
package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func t23OptionStatement(options map[string]string, pk *spec.Index, temp spec.TemporaryScope, mode string) spec.Statement {
	statement := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectTiDB,
		DDL: &spec.DDL{
			Operation:      spec.DDLOperationCreateTable,
			Table:          &spec.Table{Name: "t"},
			Columns:        []spec.Column{{Name: "id", Type: "bigint(20)", NotNull: true}},
			PrimaryKey:     pk,
			Options:        options,
			TemporaryScope: temp,
		},
	}
	if mode != "" {
		statement.Metadata = &spec.Metadata{Instance: &spec.InstanceFacts{TiDBEnableClusteredIndex: mode}}
	}
	return statement
}

func t23PK(memberType string, members ...string) *spec.Index {
	return &spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: members, PrimaryKeyType: memberType}
}

func TestT23TableOptionsValidRefusals(t *testing.T) {
	ruleObj, err := newAutoRandomOptionValidRule(policy.RulePolicy{Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{}})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	cases := []struct {
		name string
		stmt spec.Statement
		want bool
	}{
		{"cache_overflow", t23OptionStatement(map[string]string{"auto_id_cache": "9223372036854775808"}, nil, "", "on"), true},
		{"cache_ok", t23OptionStatement(map[string]string{"auto_id_cache": "1"}, nil, "", "on"), false},
		{"shard_nonclustered", t23OptionStatement(map[string]string{"shard_row_id_bits": "8"}, t23PK("nonclustered", "id"), "", "on"), false},
		{"shard_clustered_explicit", t23OptionStatement(map[string]string{"shard_row_id_bits": "8"}, t23PK("clustered", "id"), "", "on"), true},
		{"shard_clustered_by_mode", t23OptionStatement(map[string]string{"shard_row_id_bits": "8"}, t23PK("", "id"), "", "on"), true},
		{"shard_zero_clustered", t23OptionStatement(map[string]string{"shard_row_id_bits": "0"}, t23PK("clustered", "id"), "", "on"), false},
		{"pre_split_temp", t23OptionStatement(map[string]string{"pre_split_regions": "2"}, nil, spec.TemporaryScopeLocal, "on"), true},
		{"pre_split_normal", t23OptionStatement(map[string]string{"pre_split_regions": "2"}, nil, "", "on"), false},
		{"auto_random_base", t23OptionStatement(map[string]string{"auto_random_base": "100"}, nil, "", "on"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings, err := ruleObj.Evaluate(context.Background(), tc.stmt)
			if err != nil {
				t.Fatal(err)
			}
			if tc.want && len(findings) == 0 {
				t.Fatalf("%s: want refusal finding, got none", tc.name)
			}
			if !tc.want && len(findings) != 0 {
				t.Fatalf("%s: want no finding, got %+v", tc.name, findings)
			}
		})
	}
}

func TestT23TableOptionsGaps(t *testing.T) {
	ruleObj, err := newAutoRandomOptionValidRule(policy.RulePolicy{Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{}})
	if err != nil {
		t.Fatalf("new rule: %v", err)
	}
	// Unobserved clustered mode + nonzero shard: owes exactly the
	// clustered-mode fact.
	gaps := ruleObj.(rule.EvidenceReporter).EvidenceGaps(
		t23OptionStatement(map[string]string{"shard_row_id_bits": "5"}, t23PK("", "id"), "", ""))
	if len(gaps) != 1 || len(gaps[0].RequiredFacts) != 1 ||
		gaps[0].RequiredFacts[0] != "instance.tidb_enable_clustered_index" {
		t.Fatalf("want minimal clustered-mode gap, got %+v", gaps)
	}
	// Observed mode + clustered refusal: no gap.
	gaps = ruleObj.(rule.EvidenceReporter).EvidenceGaps(
		t23OptionStatement(map[string]string{"shard_row_id_bits": "5"}, t23PK("clustered", "id"), "", "on"))
	if len(gaps) != 0 {
		t.Fatalf("proven refusal must not gap, got %+v", gaps)
	}
	// Shard=0 under unobserved mode: legal regardless — no gap.
	gaps = ruleObj.(rule.EvidenceReporter).EvidenceGaps(
		t23OptionStatement(map[string]string{"shard_row_id_bits": "0"}, t23PK("", "id"), "", ""))
	if len(gaps) != 0 {
		t.Fatalf("zero shard must not gap, got %+v", gaps)
	}
}
