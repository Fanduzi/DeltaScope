// Package queryaccess verifies profile-specific builtin semantic promotion.
// input: profiled MySQL/TiDB query access requests over the production registry
// output: read_only + admissible for proven entries; indeterminate for every excluded shape
// pos: profile-specific regression coverage for MySQL 5.7/8.0/8.4 and TiDB 8.5
// note: if this file changes, update this header and module README.md.
package queryaccess

import (
	"context"
	"strings"
	"testing"

	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

// TestMySQL57ProfileAggregatesProvenPromotesAdmitsPositive aggregates on the
// MySQL 5.7 production registry row.
func TestMySQL57ProfileAggregateAdmitsPositive(t *testing.T) {
	t.Parallel()
	probes := []struct {
		name string
		sql  string
	}{
		{"count_star", "SELECT COUNT(*) FROM app.users"},
		{"count_column", "SELECT COUNT(id) FROM app.users"},
		{"sum_column", "SELECT SUM(id) FROM app.users"},
		{"avg_column", "SELECT AVG(id) FROM app.users"},
		{"min_column", "SELECT MIN(id) FROM app.users"},
		{"max_column", "SELECT MAX(id) FROM app.users"},
	}
	for _, probe := range probes {
		t.Run(probe.name, func(t *testing.T) {
			t.Parallel()
			result := analyzeProductionProfile(t, probe.sql, "mysql", "app", AnalysisProfileMySQL57)
			if result.DomainResult.ReadClassification != domain.ReadOnly || result.DomainResult.Admission != domain.Admissible {
				t.Fatalf("classification=%q admission=%q, want read_only/admissible", result.DomainResult.ReadClassification, result.DomainResult.Admission)
			}
		})
	}
}

// TestMySQL57ProfileAggregateRejectsBoundaries covers each fail-closed shape.
func TestMySQL57ProfileAggregateBoundary(t *testing.T) {
	t.Parallel()
	probes := []struct {
		name string
		sql  string
	}{
		{"unqualified_relation", "SELECT COUNT(*) FROM users"},
		{"qualified_call", "SELECT app.COUNT(*) FROM app.users"},
		{"quoted_call", "SELECT `COUNT`(*) FROM app.users"},
		{"noncanonical_spacing", "SELECT COUNT (id) FROM app.users"},
		{"distinct_modifier", "SELECT COUNT(DISTINCT id) FROM app.users"},
		{"filter_clause", "SELECT COUNT(*) FILTER (WHERE id > 0) FROM app.users"},
		{"agg_local_order", "SELECT COUNT(id ORDER BY id) FROM app.users"},
		{"nested_operand", "SELECT COUNT(ABS(id)) FROM app.users"},
		{"unknown_function", "SELECT app_specific_rollup(id) FROM app.users"},
		{"mixed_proven_unknown", "SELECT COUNT(*), app_specific_rollup(id) FROM app.users"},
		{"view_relation", "SELECT COUNT(*) FROM app.users_view"},
	}
	for _, probe := range probes {
		t.Run(probe.name, func(t *testing.T) {
			t.Parallel()
			result := analyzeProductionProfile(t, probe.sql, "mysql", "app", AnalysisProfileMySQL57)
			if result.DomainResult.ReadClassification == domain.ReadOnly && result.DomainResult.Admission == domain.Admissible {
				t.Fatalf("boundary %q was promoted: classification=%q admission=%q", probe.name, result.DomainResult.ReadClassification, result.DomainResult.Admission)
			}
		})
	}
}

// TestMySQL57ProfileWindowDeferred: MySQL 5.7 has no native ranking-window
// support. Every ranking-window form must remain indeterminate under the
// mysql-5.7 profile.
func TestMySQL57ProfileWindowDeferred(t *testing.T) {
	t.Parallel()
	probes := []struct {
		name string
		sql  string
	}{
		{"row_number", "SELECT ROW_NUMBER() OVER (PARTITION BY dept ORDER BY id) FROM app.users"},
		{"rank", "SELECT RANK() OVER (PARTITION BY dept ORDER BY id) FROM app.users"},
		{"dense_rank", "SELECT DENSE_RANK() OVER (PARTITION BY dept ORDER BY id) FROM app.users"},
	}
	for _, probe := range probes {
		t.Run(probe.name, func(t *testing.T) {
			t.Parallel()
			result := analyzeProductionProfile(t, probe.sql, "mysql", "app", AnalysisProfileMySQL57)
			if result.DomainResult.ReadClassification == domain.ReadOnly && result.DomainResult.Admission == domain.Admissible {
				t.Fatalf("MySQL 5.7 ranking window %q was promoted", probe.name)
			}
		})
	}
}

// TestMySQL80ProfileAggregateBoundary mirrors 5.7 boundaries for MySQL 8.0.
func TestMySQL80ProfileAggregateBoundary(t *testing.T) {
	t.Parallel()
	for _, probe := range mysqlAggregateBoundaryProbes() {
		t.Run(probe.name, func(t *testing.T) {
			t.Parallel()
			result := analyzeProductionProfile(t, probe.sql, "mysql", "app", AnalysisProfileMySQL80)
			if result.DomainResult.ReadClassification == domain.ReadOnly && result.DomainResult.Admission == domain.Admissible {
				t.Fatalf("boundary %q was promoted", probe.name)
			}
		})
	}
}

// TestMySQL80ProfileWindowAdmitsPositive proves ranking windows with direct
// partition and order columns on the MySQL 8.0 production registry row.
func TestMySQL80ProfileWindowAdmitsPositive(t *testing.T) {
	t.Parallel()
	probes := []struct {
		name string
		sql  string
	}{
		{"row_number", "SELECT ROW_NUMBER() OVER (PARTITION BY dept ORDER BY id) FROM app.users"},
		{"rank", "SELECT RANK() OVER (PARTITION BY dept ORDER BY id) FROM app.users"},
		{"dense_rank", "SELECT DENSE_RANK() OVER (PARTITION BY dept ORDER BY id) FROM app.users"},
	}
	for _, probe := range probes {
		t.Run(probe.name, func(t *testing.T) {
			t.Parallel()
			result := analyzeProductionProfile(t, probe.sql, "mysql", "app", AnalysisProfileMySQL80)
			if result.DomainResult.ReadClassification != domain.ReadOnly || result.DomainResult.Admission != domain.Admissible {
				t.Fatalf("classification=%q admission=%q, want read_only/admissible", result.DomainResult.ReadClassification, result.DomainResult.Admission)
			}
		})
	}
}

// TestMySQL80ProfileWindowBoundary covers excluded window shapes.
func TestMySQL80ProfileWindowBoundary(t *testing.T) {
	t.Parallel()
	probes := []struct {
		name string
		sql  string
	}{
		{"explicit_frame", "SELECT ROW_NUMBER() OVER (PARTITION BY dept ORDER BY id ROWS BETWEEN 1 PRECEDING AND CURRENT ROW) FROM app.users"},
		{"named_window", "SELECT ROW_NUMBER() OVER w FROM app.users WINDOW w AS (PARTITION BY dept ORDER BY id)"},
		{"nested_operand_partition", "SELECT ROW_NUMBER() OVER (PARTITION BY ABS(dept) ORDER BY id) FROM app.users"},
		{"nested_operand_order", "SELECT ROW_NUMBER() OVER (PARTITION BY dept ORDER BY ABS(id)) FROM app.users"},
		{"literal_partition", "SELECT ROW_NUMBER() OVER (PARTITION BY 1 ORDER BY id) FROM app.users"},
		{"missing_order", "SELECT ROW_NUMBER() OVER (PARTITION BY dept) FROM app.users"},
		{"missing_partition", "SELECT ROW_NUMBER() OVER (ORDER BY id) FROM app.users"},
	}
	for _, probe := range probes {
		t.Run(probe.name, func(t *testing.T) {
			t.Parallel()
			result := analyzeProductionProfile(t, probe.sql, "mysql", "app", AnalysisProfileMySQL80)
			if result.DomainResult.ReadClassification == domain.ReadOnly && result.DomainResult.Admission == domain.Admissible {
				t.Fatalf("window boundary %q was promoted", probe.name)
			}
		})
	}
}

// TestMySQL84ProfileAggregateBoundary mirrors 5.7 boundaries for MySQL 8.4.
func TestMySQL84ProfileAggregateBoundary(t *testing.T) {
	t.Parallel()
	for _, probe := range mysqlAggregateBoundaryProbes() {
		t.Run(probe.name, func(t *testing.T) {
			t.Parallel()
			result := analyzeProductionProfile(t, probe.sql, "mysql", "app", AnalysisProfileMySQL84)
			if result.DomainResult.ReadClassification == domain.ReadOnly && result.DomainResult.Admission == domain.Admissible {
				t.Fatalf("boundary %q was promoted", probe.name)
			}
		})
	}
}

// TestMySQL84ProfileWindowAdmitsPositive proves ranking windows for MySQL 8.4.
func TestMySQL84ProfileWindowAdmitsPositive(t *testing.T) {
	t.Parallel()
	for _, probe := range []struct {
		name string
		sql  string
	}{
		{"row_number", "SELECT ROW_NUMBER() OVER (PARTITION BY dept ORDER BY id) FROM app.users"},
		{"rank", "SELECT RANK() OVER (PARTITION BY dept ORDER BY id) FROM app.users"},
		{"dense_rank", "SELECT DENSE_RANK() OVER (PARTITION BY dept ORDER BY id) FROM app.users"},
	} {
		t.Run(probe.name, func(t *testing.T) {
			t.Parallel()
			result := analyzeProductionProfile(t, probe.sql, "mysql", "app", AnalysisProfileMySQL84)
			if result.DomainResult.ReadClassification != domain.ReadOnly || result.DomainResult.Admission != domain.Admissible {
				t.Fatalf("classification=%q admission=%q, want read_only/admissible", result.DomainResult.ReadClassification, result.DomainResult.Admission)
			}
		})
	}
}

// TestMySQL84ProfileWindowBoundary mirrors MySQL 8.0 window boundaries.
func TestMySQL84ProfileWindowBoundary(t *testing.T) {
	t.Parallel()
	for _, probe := range []struct {
		name string
		sql  string
	}{
		{"explicit_frame", "SELECT ROW_NUMBER() OVER (PARTITION BY dept ORDER BY id ROWS BETWEEN 1 PRECEDING AND CURRENT ROW) FROM app.users"},
		{"named_window", "SELECT ROW_NUMBER() OVER w FROM app.users WINDOW w AS (PARTITION BY dept ORDER BY id)"},
		{"nested_operand_partition", "SELECT ROW_NUMBER() OVER (PARTITION BY ABS(dept) ORDER BY id) FROM app.users"},
		{"missing_order", "SELECT ROW_NUMBER() OVER (PARTITION BY dept) FROM app.users"},
	} {
		t.Run(probe.name, func(t *testing.T) {
			t.Parallel()
			result := analyzeProductionProfile(t, probe.sql, "mysql", "app", AnalysisProfileMySQL84)
			if result.DomainResult.ReadClassification == domain.ReadOnly && result.DomainResult.Admission == domain.Admissible {
				t.Fatalf("window boundary %q was promoted", probe.name)
			}
		})
	}
}

// TestTiDB85ProfileAggregateBoundary mirrors MySQL aggregate boundaries for TiDB 8.5.
func TestTiDB85ProfileAggregateBoundary(t *testing.T) {
	t.Parallel()
	for _, probe := range mysqlAggregateBoundaryProbes() {
		t.Run(probe.name, func(t *testing.T) {
			t.Parallel()
			result := analyzeProductionProfile(t, probe.sql, "tidb", "app", AnalysisProfileTiDB85)
			if result.DomainResult.ReadClassification == domain.ReadOnly && result.DomainResult.Admission == domain.Admissible {
				t.Fatalf("boundary %q was promoted", probe.name)
			}
		})
	}
}

// TestTiDB85ProfileAggregateAdmitsPositive proves TiDB 8.5 aggregates independently.
func TestTiDB85ProfileAggregateAdmitsPositive(t *testing.T) {
	t.Parallel()
	for _, probe := range []struct {
		name string
		sql  string
	}{
		{"count_star", "SELECT COUNT(*) FROM app.users"},
		{"count_column", "SELECT COUNT(id) FROM app.users"},
		{"sum_column", "SELECT SUM(id) FROM app.users"},
		{"avg_column", "SELECT AVG(id) FROM app.users"},
		{"min_column", "SELECT MIN(id) FROM app.users"},
		{"max_column", "SELECT MAX(id) FROM app.users"},
	} {
		t.Run(probe.name, func(t *testing.T) {
			t.Parallel()
			result := analyzeProductionProfile(t, probe.sql, "tidb", "app", AnalysisProfileTiDB85)
			if result.DomainResult.ReadClassification != domain.ReadOnly || result.DomainResult.Admission != domain.Admissible {
				t.Fatalf("classification=%q admission=%q, want read_only/admissible", result.DomainResult.ReadClassification, result.DomainResult.Admission)
			}
		})
	}
}

// TestTiDB85ProfileWindowAdmitsPositive proves TiDB 8.5 ranking windows independently.
func TestTiDB85ProfileWindowAdmitsPositive(t *testing.T) {
	t.Parallel()
	for _, probe := range []struct {
		name string
		sql  string
	}{
		{"row_number", "SELECT ROW_NUMBER() OVER (PARTITION BY dept ORDER BY id) FROM app.users"},
		{"rank", "SELECT RANK() OVER (PARTITION BY dept ORDER BY id) FROM app.users"},
		{"dense_rank", "SELECT DENSE_RANK() OVER (PARTITION BY dept ORDER BY id) FROM app.users"},
	} {
		t.Run(probe.name, func(t *testing.T) {
			t.Parallel()
			result := analyzeProductionProfile(t, probe.sql, "tidb", "app", AnalysisProfileTiDB85)
			if result.DomainResult.ReadClassification != domain.ReadOnly || result.DomainResult.Admission != domain.Admissible {
				t.Fatalf("classification=%q admission=%q, want read_only/admissible", result.DomainResult.ReadClassification, result.DomainResult.Admission)
			}
		})
	}
}

// TestTiDB85ProfileWindowBoundary mirrors MySQL 8.x window boundaries for TiDB 8.5.
func TestTiDB85ProfileWindowBoundary(t *testing.T) {
	t.Parallel()
	for _, probe := range []struct {
		name string
		sql  string
	}{
		{"explicit_frame", "SELECT ROW_NUMBER() OVER (PARTITION BY dept ORDER BY id ROWS BETWEEN 1 PRECEDING AND CURRENT ROW) FROM app.users"},
		{"named_window", "SELECT ROW_NUMBER() OVER w FROM app.users WINDOW w AS (PARTITION BY dept ORDER BY id)"},
		{"nested_operand_partition", "SELECT ROW_NUMBER() OVER (PARTITION BY ABS(dept) ORDER BY id) FROM app.users"},
		{"missing_order", "SELECT ROW_NUMBER() OVER (PARTITION BY dept) FROM app.users"},
	} {
		t.Run(probe.name, func(t *testing.T) {
			t.Parallel()
			result := analyzeProductionProfile(t, probe.sql, "tidb", "app", AnalysisProfileTiDB85)
			if result.DomainResult.ReadClassification == domain.ReadOnly && result.DomainResult.Admission == domain.Admissible {
				t.Fatalf("window boundary %q was promoted", probe.name)
			}
		})
	}
}

func TestBuiltinSemanticProfiles_ScalarDirectColumnsAdmit(t *testing.T) {
	profiles := []struct {
		name    string
		dialect string
		profile AnalysisProfile
	}{
		{name: "mysql57", dialect: "mysql", profile: AnalysisProfileMySQL57},
		{name: "mysql80", dialect: "mysql", profile: AnalysisProfileMySQL80},
		{name: "mysql84", dialect: "mysql", profile: AnalysisProfileMySQL84},
		{name: "tidb85", dialect: "tidb", profile: AnalysisProfileTiDB85},
	}
	probes := []struct {
		name string
		sql  string
	}{
		{name: "lower", sql: "SELECT LOWER(name) FROM app.users"},
		{name: "upper", sql: "SELECT UPPER(email) FROM app.users"},
		{name: "length", sql: "SELECT LENGTH(name) FROM app.users"},
		{name: "char_length", sql: "SELECT CHAR_LENGTH(name) FROM app.users"},
		{name: "abs", sql: "SELECT ABS(amount) FROM app.orders"},
		{name: "ceil", sql: "SELECT CEIL(amount) FROM app.orders"},
		{name: "floor", sql: "SELECT FLOOR(amount) FROM app.orders"},
		{name: "coalesce", sql: "SELECT COALESCE(name, email) FROM app.users"},
		{name: "nullif", sql: "SELECT NULLIF(name, email) FROM app.users"},
		{name: "ifnull", sql: "SELECT IFNULL(name, email) FROM app.users"},
	}

	for _, profile := range profiles {
		for _, probe := range probes {
			t.Run(profile.name+"/"+probe.name, func(t *testing.T) {
				result := analyzeProductionProfile(t, probe.sql, profile.dialect, "app", profile.profile)
				if result.DomainResult.ReadClassification != domain.ReadOnly || result.DomainResult.Admission != domain.Admissible {
					t.Fatalf("classification=%q admission=%q candidates=%+v requirements=%+v reasons=%v", result.DomainResult.ReadClassification, result.DomainResult.Admission, result.EffectCandidates, result.DomainResult.Requirements, result.DomainResult.ReasonCodes)
				}
			})
		}
	}
}

func TestBuiltinSemanticProfileRegression_LiteralAndReversedPositive(t *testing.T) {
	t.Parallel()
	profiles := []struct {
		name    string
		dialect string
		profile AnalysisProfile
	}{
		{name: "mysql57", dialect: "mysql", profile: AnalysisProfileMySQL57},
		{name: "mysql80", dialect: "mysql", profile: AnalysisProfileMySQL80},
		{name: "mysql84", dialect: "mysql", profile: AnalysisProfileMySQL84},
		{name: "tidb85", dialect: "tidb", profile: AnalysisProfileTiDB85},
	}
	probes := []struct {
		name         string
		sql          string
		requirements []domain.Requirement
	}{
		{name: "lower_literal", sql: "SELECT LOWER('x') FROM app.builtin_semantic_facts", requirements: []domain.Requirement{{Object: "app.builtin_semantic_facts", Privilege: "read_table"}}},
		{name: "upper_literal", sql: "SELECT UPPER('x') FROM app.builtin_semantic_facts", requirements: []domain.Requirement{{Object: "app.builtin_semantic_facts", Privilege: "read_table"}}},
		{name: "length_literal", sql: "SELECT LENGTH('x') FROM app.builtin_semantic_facts", requirements: []domain.Requirement{{Object: "app.builtin_semantic_facts", Privilege: "read_table"}}},
		{name: "char_length_literal", sql: "SELECT CHAR_LENGTH('x') FROM app.builtin_semantic_facts", requirements: []domain.Requirement{{Object: "app.builtin_semantic_facts", Privilege: "read_table"}}},
		{name: "abs_literal", sql: "SELECT ABS(42) FROM app.builtin_semantic_facts", requirements: []domain.Requirement{{Object: "app.builtin_semantic_facts", Privilege: "read_table"}}},
		{name: "ceil_literal", sql: "SELECT CEIL(42) FROM app.builtin_semantic_facts", requirements: []domain.Requirement{{Object: "app.builtin_semantic_facts", Privilege: "read_table"}}},
		{name: "ceiling_literal", sql: "SELECT CEILING(42) FROM app.builtin_semantic_facts", requirements: []domain.Requirement{{Object: "app.builtin_semantic_facts", Privilege: "read_table"}}},
		{name: "floor_literal", sql: "SELECT FLOOR(42) FROM app.builtin_semantic_facts", requirements: []domain.Requirement{{Object: "app.builtin_semantic_facts", Privilege: "read_table"}}},
		{name: "count_literal_qualified", sql: "SELECT COUNT(1) FROM app.builtin_semantic_facts", requirements: []domain.Requirement{{Object: "app.builtin_semantic_facts", Privilege: "read_table"}}},
		{name: "coalesce_reversed", sql: "SELECT COALESCE('x', name) FROM app.builtin_semantic_facts", requirements: []domain.Requirement{{Object: "app.builtin_semantic_facts", Privilege: "read_table"}, {Object: "app.builtin_semantic_facts.name", Privilege: "read_column"}}},
		{name: "nullif_reversed", sql: "SELECT NULLIF('x', name) FROM app.builtin_semantic_facts", requirements: []domain.Requirement{{Object: "app.builtin_semantic_facts", Privilege: "read_table"}, {Object: "app.builtin_semantic_facts.name", Privilege: "read_column"}}},
		{name: "ifnull_reversed", sql: "SELECT IFNULL('x', name) FROM app.builtin_semantic_facts", requirements: []domain.Requirement{{Object: "app.builtin_semantic_facts", Privilege: "read_table"}, {Object: "app.builtin_semantic_facts.name", Privilege: "read_column"}}},
		{name: "coalesce_all_constant", sql: "SELECT COALESCE('x', 'y') FROM app.builtin_semantic_facts", requirements: []domain.Requirement{{Object: "app.builtin_semantic_facts", Privilege: "read_table"}}},
		{name: "nullif_all_constant", sql: "SELECT NULLIF('x', 'y') FROM app.builtin_semantic_facts", requirements: []domain.Requirement{{Object: "app.builtin_semantic_facts", Privilege: "read_table"}}},
		{name: "ifnull_all_constant", sql: "SELECT IFNULL('x', 'y') FROM app.builtin_semantic_facts", requirements: []domain.Requirement{{Object: "app.builtin_semantic_facts", Privilege: "read_table"}}},
	}

	for _, profile := range profiles {
		for _, probe := range probes {
			t.Run(profile.name+"/"+probe.name, func(t *testing.T) {
				result := analyzeProductionProfile(t, probe.sql, profile.dialect, "app", profile.profile)
				assertProfileAdmissible(t, result)
				assertProfileRequirements(t, result, probe.requirements)
			})
		}
	}
}

func TestBuiltinSemanticProfileRegression_BoundariesStayIndeterminate(t *testing.T) {
	t.Parallel()
	profiles := []struct {
		name    string
		dialect string
		profile AnalysisProfile
	}{
		{name: "mysql57", dialect: "mysql", profile: AnalysisProfileMySQL57},
		{name: "mysql80", dialect: "mysql", profile: AnalysisProfileMySQL80},
		{name: "mysql84", dialect: "mysql", profile: AnalysisProfileMySQL84},
		{name: "tidb85", dialect: "tidb", profile: AnalysisProfileTiDB85},
	}
	probes := []struct {
		name string
		sql  string
	}{
		{name: "relationless_lower_literal", sql: "SELECT LOWER('x')"},
		{name: "relationless_count_literal", sql: "SELECT COUNT(1)"},
		{name: "coalesce_arity_one", sql: "SELECT COALESCE('x') FROM app.builtin_semantic_facts"},
		{name: "coalesce_const_first_arity_three", sql: "SELECT COALESCE('x', 'y', 'z') FROM app.builtin_semantic_facts"},
		{name: "nested_literal_expression", sql: "SELECT LOWER(UPPER('x')) FROM app.builtin_semantic_facts"},
		{name: "parameter_literal_expression", sql: "SELECT LOWER(?) FROM app.builtin_semantic_facts"},
		{name: "unknown_function", sql: "SELECT unknown_function('x') FROM app.builtin_semantic_facts"},
	}

	for _, profile := range profiles {
		for _, probe := range probes {
			t.Run(profile.name+"/"+probe.name, func(t *testing.T) {
				result := analyzeProductionProfile(t, probe.sql, profile.dialect, "app", profile.profile)
				assertProfileIndeterminate(t, result)
			})
		}
	}
}

func TestBuiltinSemanticProfileRegression_PostgreSQLBoundariesStayIndeterminate(t *testing.T) {
	t.Parallel()
	candidate := EffectCandidate{
		Kind: EffectCandidateFunction, Ordinal: 0,
		NamePath: []string{"lower"}, OriginalNamePath: []string{"LOWER"},
		Canonical: true, ParserClassification: "generic",
		Arity: 1, OperandKinds: []string{"const"},
	}
	result := domain.Result{
		Dialect: "postgresql", Mode: domain.ModeStrict, ReadClassification: domain.Indeterminate,
		Relations:    []domain.RelationReference{{Schema: "app", Name: "builtin_semantic_facts", Kind: domain.RelationTable, PermissionRequired: true}},
		Requirements: []domain.Requirement{{Object: "app.builtin_semantic_facts", Privilege: "read_table"}},
	}
	proof := proveBuiltinSemantics(AnalysisProfileMySQL84, "postgresql", []EffectCandidate{candidate}, result, result.Requirements, builtinSemanticProductionRegistry)
	if proof.decision == builtinSemanticAllProven {
		t.Fatal("PostgreSQL literal-only candidate was proven by a MySQL profile")
	}
}

func TestBuiltinSemanticProfileRegression_MixedConstFunctions(t *testing.T) {
	t.Parallel()
	profiles := []struct {
		profile AnalysisProfile
		dialect string
	}{
		{AnalysisProfileMySQL57, "mysql"},
		{AnalysisProfileMySQL80, "mysql"},
		{AnalysisProfileMySQL84, "mysql"},
		{AnalysisProfileTiDB85, "tidb"},
	}
	for _, p := range profiles {
		t.Run(string(p.profile), func(t *testing.T) {
			t.Parallel()
			functions := []struct {
				name  string
				kinds []string
				arity int
				refs  []OperandColumnRef
			}{
				{"coalesce", []string{"column", "const"}, 2, []OperandColumnRef{{Schema: "app", Table: "users", Column: "name"}}},
				{"nullif", []string{"column", "const"}, 2, []OperandColumnRef{{Schema: "app", Table: "users", Column: "name"}}},
				{"ifnull", []string{"column", "const"}, 2, []OperandColumnRef{{Schema: "app", Table: "users", Column: "name"}}},
			}
			for _, fn := range functions {
				candidate := EffectCandidate{
					Kind:                 EffectCandidateFunction,
					Ordinal:              0,
					NamePath:             []string{fn.name},
					OriginalNamePath:     []string{strings.ToUpper(fn.name)},
					Canonical:            true,
					ParserClassification: "generic",
					Arity:                fn.arity,
					OperandKinds:         fn.kinds,
					OperandColumnRefs:    fn.refs,
				}
				result := domain.Result{
					Dialect: p.dialect, Mode: domain.ModeStrict, ReadClassification: domain.Indeterminate,
					Relations: []domain.RelationReference{{Schema: "app", Name: "users", Kind: domain.RelationTable, PermissionRequired: true}},
					ReferencedColumns: []domain.ColumnReference{
						{Schema: "app", Table: "users", Column: "name"},
					},
					Requirements: []domain.Requirement{
						{Object: "app.users", Privilege: "read_table"},
						{Object: "app.users.name", Privilege: "read_column"},
					},
				}
				proof := proveBuiltinSemantics(p.profile, p.dialect, []EffectCandidate{candidate}, result, result.Requirements, builtinSemanticProductionRegistry)
				if proof.decision != builtinSemanticAllProven {
					t.Errorf("%s/%s(%v): decision = %q, want all_proven", p.profile, fn.name, fn.kinds, proof.decision)
				}
			}
		})
	}
}

func TestBuiltinSemanticProfileRegression_MixedConstNegativeProbes(t *testing.T) {
	t.Parallel()
	registry := builtinSemanticProductionRegistry
	cases := []struct {
		name string
		cand EffectCandidate
	}{
		{
			name: "unknown_function",
			cand: EffectCandidate{
				Kind: EffectCandidateFunction, Ordinal: 0,
				NamePath: []string{"unknown_func"}, OriginalNamePath: []string{"UNKNOWN_FUNC"},
				Canonical: true, ParserClassification: "generic",
				Arity: 1, OperandKinds: []string{"column"},
				OperandColumnRefs: []OperandColumnRef{{Schema: "app", Table: "users", Column: "name"}},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := domain.Result{
				Dialect: "mysql", Mode: domain.ModeStrict, ReadClassification: domain.Indeterminate,
				Relations:         []domain.RelationReference{{Schema: "app", Name: "users", Kind: domain.RelationTable, PermissionRequired: true}},
				ReferencedColumns: []domain.ColumnReference{{Schema: "app", Table: "users", Column: "name"}},
				Requirements:      []domain.Requirement{{Object: "app.users", Privilege: "read_table"}, {Object: "app.users.name", Privilege: "read_column"}},
			}
			proof := proveBuiltinSemantics(AnalysisProfileMySQL57, "mysql", []EffectCandidate{tc.cand}, result, result.Requirements, registry)
			if proof.decision == builtinSemanticAllProven {
				t.Errorf("%s: was proven but should not be", tc.name)
			}
		})
	}
}

func TestBuiltinSemanticProfileRegression_CastNegativeProbes(t *testing.T) {
	t.Parallel()
	profiles := []struct {
		profile AnalysisProfile
		dialect string
	}{
		{AnalysisProfileMySQL57, "mysql"},
		{AnalysisProfileMySQL80, "mysql"},
		{AnalysisProfileMySQL84, "mysql"},
		{AnalysisProfileTiDB85, "tidb"},
	}
	for _, p := range profiles {
		t.Run(string(p.profile), func(t *testing.T) {
			candidate := EffectCandidate{
				Kind: EffectCandidateCast, Ordinal: 0,
				Canonical: false, ParserClassification: "cast",
				Arity: 1, OperandKinds: []string{"const"},
			}
			result := domain.Result{
				Dialect: p.dialect, Mode: domain.ModeStrict, ReadClassification: domain.Indeterminate,
				Relations:    []domain.RelationReference{{Schema: "app", Name: "builtin_semantic_facts", Kind: domain.RelationTable, PermissionRequired: true}},
				Requirements: []domain.Requirement{{Object: "app.builtin_semantic_facts", Privilege: "read_table"}},
			}
			proof := proveBuiltinSemantics(p.profile, p.dialect, []EffectCandidate{candidate}, result, result.Requirements, builtinSemanticProductionRegistry)
			if proof.decision == builtinSemanticAllProven {
				t.Fatalf("cast literal was proven for %s", p.profile)
			}
		})
	}
}

func TestBuiltinSemanticProfiles_ScalarBoundariesStayIndeterminate(t *testing.T) {
	profiles := []struct {
		name    string
		dialect string
		profile AnalysisProfile
	}{
		{name: "mysql57", dialect: "mysql", profile: AnalysisProfileMySQL57},
		{name: "mysql80", dialect: "mysql", profile: AnalysisProfileMySQL80},
		{name: "mysql84", dialect: "mysql", profile: AnalysisProfileMySQL84},
		{name: "tidb85", dialect: "tidb", profile: AnalysisProfileTiDB85},
	}
	probes := []struct {
		name string
		sql  string
	}{
		{name: "nested", sql: "SELECT LOWER(UPPER(name)) FROM app.users"},
		{name: "qualified", sql: "SELECT app.LOWER(name) FROM app.users"},
	}

	for _, profile := range profiles {
		for _, probe := range probes {
			t.Run(profile.name+"/"+probe.name, func(t *testing.T) {
				result := analyzeProductionProfile(t, probe.sql, profile.dialect, "app", profile.profile)
				if result.DomainResult.ReadClassification == domain.ReadOnly || result.DomainResult.Admission == domain.Admissible {
					t.Fatalf("scalar boundary was promoted: classification=%q admission=%q", result.DomainResult.ReadClassification, result.DomainResult.Admission)
				}
			})
		}
	}
}

// TestProfileBoundaryRejectsCrossDialectPromotion verifies a MySQL profile
// cannot affect TiDB and vice versa.
func TestProfileBoundaryRejectsCrossDialectPromotion(t *testing.T) {
	t.Parallel()
	// MySQL profile against TiDB dialect must be a bounded validation error, not a silent promotion.
	if _, err := (NewService()).Analyze(context.Background(), QueryAccessRequest{
		SQL:             "SELECT COUNT(*) FROM app.users",
		Dialect:         "tidb",
		Mode:            "strict",
		DefaultSchema:   "app",
		AnalysisProfile: AnalysisProfileMySQL84,
	}); err == nil {
		t.Fatal("MySQL profile on TiDB dialect was accepted")
	}
	// TiDB profile against MySQL dialect must be a bounded validation error.
	if _, err := (NewService()).Analyze(context.Background(), QueryAccessRequest{
		SQL:             "SELECT COUNT(*) FROM app.users",
		Dialect:         "mysql",
		Mode:            "strict",
		DefaultSchema:   "app",
		AnalysisProfile: AnalysisProfileTiDB85,
	}); err == nil {
		t.Fatal("TiDB profile on MySQL dialect was accepted")
	}
}

// TestProfileBoundaryRejectsEmptyAndUnknownProfiles verifies the fail-closed
// contract for empty and unknown profile values.
func TestProfileBoundaryRejectsEmptyAndUnknownProfiles(t *testing.T) {
	t.Parallel()
	// Empty profile preserves current indeterminate behavior.
	result := analyzeProductionProfile(t, "SELECT COUNT(*) FROM app.users", "mysql", "app", AnalysisProfileEmpty)
	if result.DomainResult.ReadClassification == domain.ReadOnly && result.DomainResult.Admission == domain.Admissible {
		t.Fatal("empty profile promoted a function query")
	}
	// Unknown profile is a bounded validation error.
	if _, err := (NewService()).Analyze(context.Background(), QueryAccessRequest{
		SQL:             "SELECT COUNT(*) FROM app.users",
		Dialect:         "mysql",
		Mode:            "strict",
		DefaultSchema:   "app",
		AnalysisProfile: AnalysisProfile("mysql-9.9"),
	}); err == nil {
		t.Fatal("unknown profile was accepted")
	}
}

func mysqlAggregateBoundaryProbes() []struct{ name, sql string } {
	return []struct{ name, sql string }{
		{"unqualified_relation", "SELECT COUNT(*) FROM users"},
		{"qualified_call", "SELECT app.COUNT(*) FROM app.users"},
		{"quoted_call", "SELECT `COUNT`(*) FROM app.users"},
		{"noncanonical_spacing", "SELECT COUNT (id) FROM app.users"},
		{"distinct_modifier", "SELECT COUNT(DISTINCT id) FROM app.users"},
		{"filter_clause", "SELECT COUNT(*) FILTER (WHERE id > 0) FROM app.users"},
		{"agg_local_order", "SELECT COUNT(id ORDER BY id) FROM app.users"},
		{"nested_operand", "SELECT COUNT(ABS(id)) FROM app.users"},
		{"unknown_function", "SELECT app_specific_rollup(id) FROM app.users"},
		{"mixed_proven_unknown", "SELECT COUNT(*), app_specific_rollup(id) FROM app.users"},
		{"view_relation", "SELECT COUNT(*) FROM app.users_view"},
	}
}

func assertProfileAdmissible(t *testing.T, result QueryAccessResult) {
	t.Helper()
	if result.DomainResult.ReadClassification != domain.ReadOnly || result.DomainResult.Admission != domain.Admissible {
		t.Fatalf("classification=%q admission=%q candidates=%+v requirements=%+v reasons=%v", result.DomainResult.ReadClassification, result.DomainResult.Admission, result.EffectCandidates, result.DomainResult.Requirements, result.DomainResult.ReasonCodes)
	}
}

func assertProfileIndeterminate(t *testing.T, result QueryAccessResult) {
	t.Helper()
	if result.DomainResult.ReadClassification != domain.Indeterminate || result.DomainResult.Admission != domain.IndeterminateAdmission {
		t.Fatalf("classification=%q admission=%q candidates=%+v requirements=%+v reasons=%v", result.DomainResult.ReadClassification, result.DomainResult.Admission, result.EffectCandidates, result.DomainResult.Requirements, result.DomainResult.ReasonCodes)
	}
}

func assertProfileRequirements(t *testing.T, result QueryAccessResult, want []domain.Requirement) {
	t.Helper()
	got := result.DomainResult.Requirements
	if len(got) != len(want) {
		t.Fatalf("requirements=%+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("requirements=%+v, want %+v", got, want)
		}
	}
}

func analyzeProductionProfile(t *testing.T, sql, dialect, defaultSchema string, profile AnalysisProfile) QueryAccessResult {
	t.Helper()
	service, err := newBuiltinSemanticService(&profileTestResolver{}, builtinSemanticProductionRegistry)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	result, err := service.Analyze(context.Background(), QueryAccessRequest{
		SQL:             sql,
		Dialect:         dialect,
		Mode:            "strict",
		DefaultSchema:   defaultSchema,
		AnalysisProfile: profile,
		SchemaResolver:  &profileTestResolver{},
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	return result
}

type profileTestResolver struct{}

func (*profileTestResolver) ResolveRelation(ctx context.Context, dialect, schema, name string) (RelationSchema, error) {
	if name == "users_view" {
		return RelationSchema{Schema: schema, Name: name, Kind: "view", IsView: true, Columns: []ColumnSchema{{Name: "id", Ordinal: 1}}}, nil
	}
	return RelationSchema{Schema: schema, Name: name, Kind: "table", Columns: []ColumnSchema{
		{Name: "id", Ordinal: 1},
		{Name: "dept", Ordinal: 2},
		{Name: "name", Ordinal: 3},
		{Name: "email", Ordinal: 4},
		{Name: "amount", Ordinal: 5},
	}}, nil
}
