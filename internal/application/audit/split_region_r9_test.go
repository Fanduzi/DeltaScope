package audit

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule/ddl"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// split_region_r9_test.go pins the T23-R9 SPLIT bound-relation matrix
// through the real parse → enrich → rule path (issue #102). Upstream
// anchors (v8.5.0): pkg/executor/split.go calculateIntBoundValue refuses
// upper<=lower and step<minRegionStepValue(1000) on the int-handle arm,
// and pkg/planner/core/planbuilder.go convertValueListToData refuses
// NULL bounds with ErrBadNull on the table path (BETWEEN and BY alike).
// The common-handle/index BETWEEN arms compare encoded keys — a
// capability boundary this product discloses instead of completing.

func r9SplitProvider(shape *spec.TableSnapshot) *t23Provider {
	return &t23Provider{snapshot: shape, facts: &spec.InstanceFacts{
		Version:                    "8.0.11-TiDB-v8.5.0",
		TiDBSplitRegionMaxNum:      1000,
		TiDBSplitRegionMaxNumKnown: true,
	}}
}

func r9SplitRegistry(t *testing.T) *rule.Registry {
	t.Helper()
	registry := rule.NewRegistry()
	err := ddl.Register(registry, policy.Policy{Rules: map[string]policy.RulePolicy{
		"ddl.table.split_region.exists.require": {Enabled: true},
		"ddl.tidb.table.split_region.valid":     {Enabled: true},
	}})
	if err != nil {
		t.Fatalf("register split rules: %v", err)
	}
	return registry
}

func r9Evaluate(t *testing.T, registry *rule.Registry, sql string, provider *t23Provider) (report.CoverageStatus, []rule.Finding, []rule.EvidenceGap) {
	t.Helper()
	_, stmts := t17Enrich(t, spec.DialectTiDB, sql, "req", t17v8046, provider)
	res, err := EvaluateStatements(context.Background(), registry, stmts)
	if err != nil {
		t.Fatalf("evaluate %q: %v", sql, err)
	}
	if len(res.Statements) != 1 {
		t.Fatalf("expected one statement, got %d", len(res.Statements))
	}
	sr := res.Statements[0]
	return sr.Coverage.Status, sr.Findings, sr.EvidenceGaps
}

func r9FindingReasons(findings []rule.Finding) []any {
	out := make([]any, 0, len(findings))
	for _, f := range findings {
		out = append(out, f.Metadata["reason"])
	}
	return out
}

func r9GapCodes(gaps []rule.EvidenceGap) []string {
	out := make([]string, 0, len(gaps))
	for _, g := range gaps {
		if len(g.RequiredFacts) > 0 {
			out = append(out, g.ReasonCode+":"+g.RequiredFacts[0])
			continue
		}
		out = append(out, g.ReasonCode)
	}
	return out
}

func TestT23R9SplitBoundMatrix(t *testing.T) {
	false_ := false
	signedHandle := &spec.TableSnapshot{
		Exists: true, Schema: "req", Partitioned: &false_,
		PrimaryKeyIsHandle: true, PrimaryKey: &spec.Index{Columns: []string{"id"}},
		Columns: []spec.Column{{Name: "id", Type: "bigint"}},
		Options: map[string]string{},
	}
	unsignedHandle := &spec.TableSnapshot{
		Exists: true, Schema: "req", Partitioned: &false_,
		PrimaryKeyIsHandle: true, PrimaryKey: &spec.Index{Columns: []string{"id"}},
		Columns: []spec.Column{{Name: "id", Type: "bigint unsigned"}},
		Options: map[string]string{},
	}
	commonHandle := &spec.TableSnapshot{
		Exists: true, Schema: "req", Partitioned: &false_,
		PrimaryKeyClustered: true, PrimaryKey: &spec.Index{Columns: []string{"a", "b"}},
		Columns: []spec.Column{
			{Name: "a", Type: "bigint"}, {Name: "b", Type: "varchar(10)"},
		},
		Indexes: []spec.Index{{Name: "ix", Columns: []string{"a"}}},
		Options: map[string]string{},
	}
	// PRIMARY KEY(id) CLUSTERED on a single int column is still a common
	// handle upstream — BuildHandleByDatums, never calculateIntBoundValue.
	commonHandleSingle := &spec.TableSnapshot{
		Exists: true, Schema: "req", Partitioned: &false_,
		PrimaryKeyClustered: true, PrimaryKey: &spec.Index{Columns: []string{"id"}},
		Columns: []spec.Column{{Name: "id", Type: "bigint"}},
		Options: map[string]string{},
	}
	registry := r9SplitRegistry(t)
	type want struct {
		coverage report.CoverageStatus
		reasons  []any
		gaps     []string
	}
	cases := []struct {
		name  string
		sql   string
		shape *spec.TableSnapshot
		want  want
	}{
		{"range_ok_step2500", "SPLIT TABLE t BETWEEN (0) AND (10000) REGIONS 4;", signedHandle,
			want{report.CoverageComplete, nil, nil}},
		{"step_too_small", "SPLIT TABLE t BETWEEN (0) AND (100) REGIONS 4;", signedHandle,
			want{report.CoverageComplete, []any{"split_step_too_small"}, nil}},
		{"bounds_equal_refused", "SPLIT TABLE t BETWEEN (100) AND (100) REGIONS 4;", signedHandle,
			want{report.CoverageComplete, []any{"split_bound_not_increasing"}, nil}},
		{"bounds_reversed_refused", "SPLIT TABLE t BETWEEN (100) AND (0) REGIONS 4;", signedHandle,
			want{report.CoverageComplete, []any{"split_bound_not_increasing"}, nil}},
		{"step_boundary_1000_ok", "SPLIT TABLE t BETWEEN (0) AND (4000) REGIONS 4;", signedHandle,
			want{report.CoverageComplete, nil, nil}},
		{"step_999_refused", "SPLIT TABLE t BETWEEN (0) AND (3999) REGIONS 4;", signedHandle,
			want{report.CoverageComplete, []any{"split_step_too_small"}, nil}},
		{"null_lower_refused", "SPLIT TABLE t BETWEEN (NULL) AND (10000) REGIONS 4;", signedHandle,
			want{report.CoverageComplete, []any{"split_null_value"}, nil}},
		{"null_upper_refused", "SPLIT TABLE t BETWEEN (0) AND (NULL) REGIONS 4;", signedHandle,
			want{report.CoverageComplete, []any{"split_null_value"}, nil}},
		{"by_null_refused", "SPLIT TABLE t BY (NULL),(20);", signedHandle,
			want{report.CoverageComplete, []any{"split_null_value"}, nil}},
		{"by_literals_ok", "SPLIT TABLE t BY (10),(20),(30);", signedHandle,
			want{report.CoverageComplete, nil, nil}},
		{"negative_range_step", "SPLIT TABLE t BETWEEN (-10000) AND (10000) REGIONS 4;", signedHandle,
			want{report.CoverageComplete, nil, nil}},
		{"negative_step_small", "SPLIT TABLE t BETWEEN (-100) AND (100) REGIONS 4;", signedHandle,
			want{report.CoverageComplete, []any{"split_step_too_small"}, nil}},
		{"unsigned_wide_ok", "SPLIT TABLE t BETWEEN (0) AND (18446744073709551615) REGIONS 4;", unsignedHandle,
			want{report.CoverageComplete, nil, nil}},
		{"unsigned_step_wrap_refused", "SPLIT TABLE t BETWEEN (0) AND (18446744073709551615) REGIONS 1;", unsignedHandle,
			want{report.CoverageComplete, []any{"split_step_too_small"}, nil}},
		{"index_between_compare_gap", "SPLIT TABLE t INDEX ix BETWEEN (0) AND (100) REGIONS 4;", commonHandle,
			want{report.CoverageUnverified, nil, []string{"unproven_statement_fact:statement.split_bound_relation"}}},
		{"common_handle_compare_gap", "SPLIT TABLE t BETWEEN (0, 'a') AND (100, 'b') REGIONS 4;", commonHandle,
			want{report.CoverageUnverified, nil, []string{"unproven_statement_fact:statement.split_bound_relation"}}},
		{"single_int_common_handle_gap", "SPLIT TABLE t BETWEEN (0) AND (10000) REGIONS 4;", commonHandleSingle,
			want{report.CoverageUnverified, nil, []string{"unproven_statement_fact:statement.split_bound_relation"}}},
		{"expr_bound_gap", "SPLIT TABLE t BETWEEN (abs(0)) AND (10000) REGIONS 4;", signedHandle,
			want{report.CoverageUnverified, nil, []string{"unproven_statement_fact:statement.split_value_foldability"}}},
		{"regions_zero_refused", "SPLIT TABLE t BETWEEN (0) AND (10000) REGIONS 0;", signedHandle,
			want{report.CoverageComplete, []any{"split_num_below_one"}, nil}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			coverage, findings, gaps := r9Evaluate(t, registry, tc.sql, r9SplitProvider(tc.shape))
			if coverage != tc.want.coverage {
				t.Fatalf("coverage %v want %v (findings=%v gaps=%v)",
					coverage, tc.want.coverage, r9FindingReasons(findings), r9GapCodes(gaps))
			}
			gotReasons := r9FindingReasons(findings)
			if len(gotReasons) != len(tc.want.reasons) {
				t.Fatalf("reasons %v want %v", gotReasons, tc.want.reasons)
			}
			for i, reason := range tc.want.reasons {
				if gotReasons[i] != reason {
					t.Fatalf("reasons %v want %v", gotReasons, tc.want.reasons)
				}
			}
			gotGaps := r9GapCodes(gaps)
			if len(gotGaps) != len(tc.want.gaps) {
				t.Fatalf("gaps %v want %v", gotGaps, tc.want.gaps)
			}
			for i, gap := range tc.want.gaps {
				if gotGaps[i] != gap {
					t.Fatalf("gaps %v want %v", gotGaps, tc.want.gaps)
				}
			}
		})
	}
}
