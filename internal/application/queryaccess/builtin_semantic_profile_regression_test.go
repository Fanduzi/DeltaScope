// Package queryaccess verifies profile-specific builtin semantic promotion.
// input: profiled MySQL/TiDB query access requests over the production registry
// output: read_only + admissible for proven entries; indeterminate for every excluded shape
// pos: profile-specific regression coverage for MySQL 5.7/8.0/8.4 and TiDB 8.5
// note: if this file changes, update this header and module README.md.
package queryaccess

import (
	"context"
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
		{"literal_operand", "SELECT COUNT(1) FROM app.users"},
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
		{"literal_operand", "SELECT COUNT(1) FROM app.users"},
		{"unknown_function", "SELECT app_specific_rollup(id) FROM app.users"},
		{"mixed_proven_unknown", "SELECT COUNT(*), app_specific_rollup(id) FROM app.users"},
		{"view_relation", "SELECT COUNT(*) FROM app.users_view"},
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
	}}, nil
}
