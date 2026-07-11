package queryaccess_test

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

func TestNormalizeMode(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   queryaccess.Mode
		want queryaccess.Mode
	}{
		{"empty defaults to strict", "", queryaccess.ModeStrict},
		{"strict stays strict", queryaccess.ModeStrict, queryaccess.ModeStrict},
		{"projection_only stays", queryaccess.ModeProjectionOnly, queryaccess.ModeProjectionOnly},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := queryaccess.NormalizeMode(tt.in)
			if got != tt.want {
				t.Errorf("NormalizeMode(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestValidateMode(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		mode    queryaccess.Mode
		wantErr bool
	}{
		{"strict is valid", queryaccess.ModeStrict, false},
		{"projection_only is valid", queryaccess.ModeProjectionOnly, false},
		{"empty is invalid", "", true},
		{"unknown is invalid", "unknown", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := queryaccess.ValidateMode(tt.mode)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateMode(%q) error = %v, wantErr %v", tt.mode, err, tt.wantErr)
			}
		})
	}
}

func TestFoldReadClassification(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   []queryaccess.ReadClassification
		want queryaccess.ReadClassification
	}{
		{"nil returns indeterminate", nil, queryaccess.Indeterminate},
		{"empty returns indeterminate", []queryaccess.ReadClassification{}, queryaccess.Indeterminate},
		{"single read_only", []queryaccess.ReadClassification{queryaccess.ReadOnly}, queryaccess.ReadOnly},
		{"single not_read_only", []queryaccess.ReadClassification{queryaccess.NotReadOnly}, queryaccess.NotReadOnly},
		{"single indeterminate", []queryaccess.ReadClassification{queryaccess.Indeterminate}, queryaccess.Indeterminate},
		{"all read_only", []queryaccess.ReadClassification{queryaccess.ReadOnly, queryaccess.ReadOnly, queryaccess.ReadOnly}, queryaccess.ReadOnly},
		{"any not_read_only wins", []queryaccess.ReadClassification{queryaccess.ReadOnly, queryaccess.NotReadOnly, queryaccess.ReadOnly}, queryaccess.NotReadOnly},
		{"indeterminate when no not_read_only", []queryaccess.ReadClassification{queryaccess.ReadOnly, queryaccess.Indeterminate, queryaccess.ReadOnly}, queryaccess.Indeterminate},
		{"not_read_only beats indeterminate", []queryaccess.ReadClassification{queryaccess.Indeterminate, queryaccess.NotReadOnly, queryaccess.ReadOnly}, queryaccess.NotReadOnly},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := queryaccess.FoldReadClassification(tt.in)
			if got != tt.want {
				t.Errorf("FoldReadClassification(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestValidateAdmission(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		rc      queryaccess.ReadClassification
		adm     queryaccess.Admission
		wantErr bool
	}{
		{"admissible + read_only is valid", queryaccess.ReadOnly, queryaccess.Admissible, false},
		{"admissible + not_read_only is invalid", queryaccess.NotReadOnly, queryaccess.Admissible, true},
		{"admissible + indeterminate is invalid", queryaccess.Indeterminate, queryaccess.Admissible, true},
		{"rejected + read_only is valid", queryaccess.ReadOnly, queryaccess.Rejected, false},
		{"rejected + not_read_only is valid", queryaccess.NotReadOnly, queryaccess.Rejected, false},
		{"rejected + indeterminate is valid", queryaccess.Indeterminate, queryaccess.Rejected, false},
		{"indeterminate_admission + read_only is valid", queryaccess.ReadOnly, queryaccess.IndeterminateAdmission, false},
		{"indeterminate_admission + not_read_only is valid", queryaccess.NotReadOnly, queryaccess.IndeterminateAdmission, false},
		{"unknown admission is rejected", queryaccess.ReadOnly, queryaccess.Admission("bogus"), true},
		{"empty admission is rejected", queryaccess.ReadOnly, queryaccess.Admission(""), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := queryaccess.ValidateAdmission(tt.rc, tt.adm)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAdmission(%q, %q) error = %v, wantErr %v", tt.rc, tt.adm, err, tt.wantErr)
			}
		})
	}
}

func TestSortRelations(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   []queryaccess.RelationReference
		want []queryaccess.RelationReference
	}{
		{
			"nil returns nil",
			nil,
			nil,
		},
		{
			"empty returns empty",
			[]queryaccess.RelationReference{},
			[]queryaccess.RelationReference{},
		},
		{
			"single element unchanged",
			[]queryaccess.RelationReference{{Name: "users"}},
			[]queryaccess.RelationReference{{Name: "users"}},
		},
		{
			"sorted by schema then name",
			[]queryaccess.RelationReference{
				{Schema: "b", Name: "orders"},
				{Schema: "a", Name: "users"},
				{Schema: "a", Name: "accounts"},
			},
			[]queryaccess.RelationReference{
				{Schema: "a", Name: "accounts"},
				{Schema: "a", Name: "users"},
				{Schema: "b", Name: "orders"},
			},
		},
		{
			"empty schema sorts before non-empty",
			[]queryaccess.RelationReference{
				{Schema: "app", Name: "users"},
				{Name: "temp"},
			},
			[]queryaccess.RelationReference{
				{Name: "temp"},
				{Schema: "app", Name: "users"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := queryaccess.SortRelations(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("len: got %d, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i].Schema != tt.want[i].Schema || got[i].Name != tt.want[i].Name {
					t.Errorf("[%d]: got {Schema:%q, Name:%q}, want {Schema:%q, Name:%q}",
						i, got[i].Schema, got[i].Name, tt.want[i].Schema, tt.want[i].Name)
				}
			}
		})
	}
}

func TestSortRelations_DoesNotMutateInput(t *testing.T) {
	t.Parallel()
	original := []queryaccess.RelationReference{
		{Schema: "b", Name: "z"},
		{Schema: "a", Name: "a"},
	}
	input := append([]queryaccess.RelationReference(nil), original...)
	_ = queryaccess.SortRelations(input)
	// Verify input unchanged
	if input[0].Schema != "b" || input[0].Name != "z" {
		t.Errorf("input was mutated: got %+v", input)
	}
}

func TestSortColumns(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   []queryaccess.ColumnReference
		want []queryaccess.ColumnReference
	}{
		{
			"nil returns nil",
			nil,
			nil,
		},
		{
			"empty returns empty",
			[]queryaccess.ColumnReference{},
			[]queryaccess.ColumnReference{},
		},
		{
			"sorted by schema+table+column",
			[]queryaccess.ColumnReference{
				{Schema: "b", Table: "t", Column: "z"},
				{Schema: "a", Table: "t", Column: "a"},
				{Schema: "a", Table: "t", Column: "m"},
			},
			[]queryaccess.ColumnReference{
				{Schema: "a", Table: "t", Column: "a"},
				{Schema: "a", Table: "t", Column: "m"},
				{Schema: "b", Table: "t", Column: "z"},
			},
		},
		{
			"usage order preserved within column",
			[]queryaccess.ColumnReference{
				{Table: "t", Column: "a", Usages: []queryaccess.UsageContext{queryaccess.UsageFilter, queryaccess.UsageProjection}},
			},
			[]queryaccess.ColumnReference{
				{Table: "t", Column: "a", Usages: []queryaccess.UsageContext{queryaccess.UsageFilter, queryaccess.UsageProjection}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := queryaccess.SortColumns(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("len: got %d, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i].Schema != tt.want[i].Schema || got[i].Table != tt.want[i].Table || got[i].Column != tt.want[i].Column {
					t.Errorf("[%d]: got {Schema:%q, Table:%q, Column:%q}, want {Schema:%q, Table:%q, Column:%q}",
						i, got[i].Schema, got[i].Table, got[i].Column,
						tt.want[i].Schema, tt.want[i].Table, tt.want[i].Column)
				}
			}
		})
	}
}

func TestSortColumns_DoesNotMutateInput(t *testing.T) {
	t.Parallel()
	original := []queryaccess.ColumnReference{
		{Table: "t", Column: "z"},
		{Table: "t", Column: "a"},
	}
	input := append([]queryaccess.ColumnReference(nil), original...)
	_ = queryaccess.SortColumns(input)
	if input[0].Column != "z" {
		t.Errorf("input was mutated: got %+v", input)
	}
}

func TestSortRequirements(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   []queryaccess.Requirement
		want []queryaccess.Requirement
	}{
		{
			"nil returns nil",
			nil,
			nil,
		},
		{
			"empty returns empty",
			[]queryaccess.Requirement{},
			[]queryaccess.Requirement{},
		},
		{
			"sorted by object then privilege",
			[]queryaccess.Requirement{
				{Object: "b.t", Privilege: "read_table"},
				{Object: "a.t", Privilege: "read_column"},
				{Object: "a.t", Privilege: "read_table"},
			},
			[]queryaccess.Requirement{
				{Object: "a.t", Privilege: "read_column"},
				{Object: "a.t", Privilege: "read_table"},
				{Object: "b.t", Privilege: "read_table"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := queryaccess.SortRequirements(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("len: got %d, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("[%d]: got %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestSortRequirements_DoesNotMutateInput(t *testing.T) {
	t.Parallel()
	original := []queryaccess.Requirement{
		{Object: "b", Privilege: "read_table"},
		{Object: "a", Privilege: "read_table"},
	}
	input := append([]queryaccess.Requirement(nil), original...)
	_ = queryaccess.SortRequirements(input)
	if input[0].Object != "b" {
		t.Errorf("input was mutated: got %+v", input)
	}
}

func TestDeduplicateUsages(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   []queryaccess.UsageContext
		want []queryaccess.UsageContext
	}{
		{
			"nil returns nil",
			nil,
			nil,
		},
		{
			"empty returns empty",
			[]queryaccess.UsageContext{},
			[]queryaccess.UsageContext{},
		},
		{
			"single stays single",
			[]queryaccess.UsageContext{queryaccess.UsageProjection},
			[]queryaccess.UsageContext{queryaccess.UsageProjection},
		},
		{
			"duplicates removed preserving order",
			[]queryaccess.UsageContext{
				queryaccess.UsageProjection,
				queryaccess.UsageFilter,
				queryaccess.UsageProjection,
				queryaccess.UsageJoin,
				queryaccess.UsageFilter,
			},
			[]queryaccess.UsageContext{
				queryaccess.UsageProjection,
				queryaccess.UsageFilter,
				queryaccess.UsageJoin,
			},
		},
		{
			"all same collapses to one",
			[]queryaccess.UsageContext{
				queryaccess.UsageFilter,
				queryaccess.UsageFilter,
				queryaccess.UsageFilter,
			},
			[]queryaccess.UsageContext{queryaccess.UsageFilter},
		},
		{
			"all unique preserved",
			[]queryaccess.UsageContext{
				queryaccess.UsageProjection,
				queryaccess.UsageFilter,
				queryaccess.UsageJoin,
				queryaccess.UsageGrouping,
				queryaccess.UsageHaving,
				queryaccess.UsageOrdering,
				queryaccess.UsageWindow,
			},
			[]queryaccess.UsageContext{
				queryaccess.UsageProjection,
				queryaccess.UsageFilter,
				queryaccess.UsageJoin,
				queryaccess.UsageGrouping,
				queryaccess.UsageHaving,
				queryaccess.UsageOrdering,
				queryaccess.UsageWindow,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := queryaccess.DeduplicateUsages(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("len: got %d, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("[%d]: got %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestDeduplicateUsages_DoesNotMutateInput(t *testing.T) {
	t.Parallel()
	original := []queryaccess.UsageContext{
		queryaccess.UsageProjection,
		queryaccess.UsageFilter,
		queryaccess.UsageProjection,
	}
	input := append([]queryaccess.UsageContext(nil), original...)
	_ = queryaccess.DeduplicateUsages(input)
	if len(input) != 3 {
		t.Errorf("input was mutated: len = %d, want 3", len(input))
	}
}

func TestValidateResult(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		result  *queryaccess.Result
		wantErr bool
	}{
		{
			"nil is valid",
			nil,
			false,
		},
		{
			"admissible + read_only is valid",
			&queryaccess.Result{
				Mode:               queryaccess.ModeStrict,
				ReadClassification: queryaccess.ReadOnly,
				Admission:          queryaccess.Admissible,
			},
			false,
		},
		{
			"admissible + not_read_only is invalid",
			&queryaccess.Result{
				Mode:               queryaccess.ModeStrict,
				ReadClassification: queryaccess.NotReadOnly,
				Admission:          queryaccess.Admissible,
			},
			true,
		},
		{
			"rejected + not_read_only is valid",
			&queryaccess.Result{
				Mode:               queryaccess.ModeStrict,
				ReadClassification: queryaccess.NotReadOnly,
				Admission:          queryaccess.Rejected,
			},
			false,
		},
		{
			"invalid mode is rejected",
			&queryaccess.Result{
				Mode:               "bad",
				ReadClassification: queryaccess.ReadOnly,
				Admission:          queryaccess.Admissible,
			},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := queryaccess.ValidateResult(tt.result)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateResult(%+v) error = %v, wantErr %v", tt.result, err, tt.wantErr)
			}
		})
	}
}

func TestFormatRelationKey(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		schema   string
		relation string
		want     string
	}{
		{"with schema", "app", "users", "app.users"},
		{"without schema", "", "users", "users"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := queryaccess.FormatRelationKey(tt.schema, tt.relation)
			if got != tt.want {
				t.Errorf("FormatRelationKey(%q, %q) = %q, want %q", tt.schema, tt.relation, got, tt.want)
			}
		})
	}
}

func TestFormatColumnKey(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		schema string
		table  string
		column string
		want   string
	}{
		{"with schema", "app", "users", "id", "app.users.id"},
		{"without schema", "", "users", "id", "users.id"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := queryaccess.FormatColumnKey(tt.schema, tt.table, tt.column)
			if got != tt.want {
				t.Errorf("FormatColumnKey(%q, %q, %q) = %q, want %q", tt.schema, tt.table, tt.column, got, tt.want)
			}
		})
	}
}
