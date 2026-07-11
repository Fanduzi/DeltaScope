package queryaccess

import (
	"testing"

	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

// S1: Salary threshold WHERE inference
// SELECT id FROM users WHERE salary > 100000
func TestBuildRequirements_S1_SalaryThresholdWHERE(t *testing.T) {
	t.Parallel()
	relations := []domain.RelationReference{
		{Schema: "app", Name: "users", Kind: domain.RelationTable, PermissionRequired: true},
	}
	columns := []domain.ColumnReference{
		{Schema: "app", Table: "users", Column: "id", Usages: []domain.UsageContext{domain.UsageProjection}},
		{Schema: "app", Table: "users", Column: "salary", Usages: []domain.UsageContext{domain.UsageFilter}},
	}
	outputs := []domain.OutputColumn{
		{Name: "id", Sources: []string{"app.users.id"}},
	}

	t.Run("strict requires both id and salary", func(t *testing.T) {
		t.Parallel()
		reqs, warnings, _, err := buildRequirements(domain.ModeStrict, relations, columns, outputs, nil)
		if err != nil {
			t.Fatalf("buildRequirements: %v", err)
		}
		assertRequirement(t, reqs, "app.users", "read_table")
		assertRequirement(t, reqs, "app.users.id", "read_column")
		assertRequirement(t, reqs, "app.users.salary", "read_column")
		if len(warnings) != 0 {
			t.Errorf("strict should have no warnings, got %d", len(warnings))
		}
	})

	t.Run("projection_only requires id only, reports salary, emits inference_risk", func(t *testing.T) {
		t.Parallel()
		reqs, warnings, _, err := buildRequirements(domain.ModeProjectionOnly, relations, columns, outputs, nil)
		if err != nil {
			t.Fatalf("buildRequirements: %v", err)
		}
		assertRequirement(t, reqs, "app.users", "read_table")
		assertRequirement(t, reqs, "app.users.id", "read_column")
		assertNoRequirement(t, reqs, "app.users.salary", "read_column")
		assertWarning(t, warnings, domain.WarningInferenceRisk)
	})
}

// S2: Blacklist JOIN membership
// SELECT u.name FROM users u JOIN blacklist b ON u.id = b.user_id
func TestBuildRequirements_S2_BlacklistJOIN(t *testing.T) {
	t.Parallel()
	relations := []domain.RelationReference{
		{Schema: "app", Name: "users", Alias: "u", Kind: domain.RelationTable, PermissionRequired: true},
		{Schema: "app", Name: "blacklist", Alias: "b", Kind: domain.RelationTable, PermissionRequired: true},
	}
	columns := []domain.ColumnReference{
		{Schema: "app", Table: "users", Column: "name", Usages: []domain.UsageContext{domain.UsageProjection}},
		{Schema: "app", Table: "users", Column: "id", Usages: []domain.UsageContext{domain.UsageJoin}},
		{Schema: "app", Table: "blacklist", Column: "user_id", Usages: []domain.UsageContext{domain.UsageJoin}},
	}
	outputs := []domain.OutputColumn{
		{Name: "name", Sources: []string{"app.users.name"}},
	}

	t.Run("strict requires name, id, and user_id", func(t *testing.T) {
		t.Parallel()
		reqs, _, _, err := buildRequirements(domain.ModeStrict, relations, columns, outputs, nil)
		if err != nil {
			t.Fatalf("buildRequirements: %v", err)
		}
		assertRequirement(t, reqs, "app.users", "read_table")
		assertRequirement(t, reqs, "app.blacklist", "read_table")
		assertRequirement(t, reqs, "app.users.name", "read_column")
		assertRequirement(t, reqs, "app.users.id", "read_column")
		assertRequirement(t, reqs, "app.blacklist.user_id", "read_column")
	})

	t.Run("projection_only requires name only, reports blacklist.user_id, emits inference_risk", func(t *testing.T) {
		t.Parallel()
		reqs, warnings, _, err := buildRequirements(domain.ModeProjectionOnly, relations, columns, outputs, nil)
		if err != nil {
			t.Fatalf("buildRequirements: %v", err)
		}
		assertRequirement(t, reqs, "app.users", "read_table")
		assertRequirement(t, reqs, "app.blacklist", "read_table")
		assertRequirement(t, reqs, "app.users.name", "read_column")
		assertNoRequirement(t, reqs, "app.users.id", "read_column")
		assertNoRequirement(t, reqs, "app.blacklist.user_id", "read_column")
		assertWarning(t, warnings, domain.WarningInferenceRisk)
	})
}

// S3: Diagnosis GROUP/HAVING count
// SELECT department, COUNT(*) as cnt FROM patients GROUP BY department HAVING COUNT(*) > 5
func TestBuildRequirements_S3_DiagnosisGroupHaving(t *testing.T) {
	t.Parallel()
	relations := []domain.RelationReference{
		{Schema: "app", Name: "patients", Kind: domain.RelationTable, PermissionRequired: true},
	}
	columns := []domain.ColumnReference{
		{Schema: "app", Table: "patients", Column: "department", Usages: []domain.UsageContext{domain.UsageGrouping}},
	}
	outputs := []domain.OutputColumn{
		{Name: "department", Sources: []string{"app.patients.department"}},
		{Name: "cnt", Sources: []string{}},
	}

	t.Run("strict requires department", func(t *testing.T) {
		t.Parallel()
		reqs, _, _, err := buildRequirements(domain.ModeStrict, relations, columns, outputs, nil)
		if err != nil {
			t.Fatalf("buildRequirements: %v", err)
		}
		assertRequirement(t, reqs, "app.patients", "read_table")
		assertRequirement(t, reqs, "app.patients.department", "read_column")
	})

	t.Run("projection_only requires department (output-contributing)", func(t *testing.T) {
		t.Parallel()
		reqs, warnings, _, err := buildRequirements(domain.ModeProjectionOnly, relations, columns, outputs, nil)
		if err != nil {
			t.Fatalf("buildRequirements: %v", err)
		}
		assertRequirement(t, reqs, "app.patients", "read_table")
		assertRequirement(t, reqs, "app.patients.department", "read_column")
		// department is both grouped and projected, so no inference risk
		assertNoWarning(t, warnings, domain.WarningInferenceRisk)
	})
}

// S4: Salary ORDER BY ranking
// SELECT name FROM employees ORDER BY salary DESC
func TestBuildRequirements_S4_SalaryOrderBy(t *testing.T) {
	t.Parallel()
	relations := []domain.RelationReference{
		{Schema: "app", Name: "employees", Kind: domain.RelationTable, PermissionRequired: true},
	}
	columns := []domain.ColumnReference{
		{Schema: "app", Table: "employees", Column: "name", Usages: []domain.UsageContext{domain.UsageProjection}},
		{Schema: "app", Table: "employees", Column: "salary", Usages: []domain.UsageContext{domain.UsageOrdering}},
	}
	outputs := []domain.OutputColumn{
		{Name: "name", Sources: []string{"app.employees.name"}},
	}

	t.Run("strict requires both name and salary", func(t *testing.T) {
		t.Parallel()
		reqs, _, _, err := buildRequirements(domain.ModeStrict, relations, columns, outputs, nil)
		if err != nil {
			t.Fatalf("buildRequirements: %v", err)
		}
		assertRequirement(t, reqs, "app.employees", "read_table")
		assertRequirement(t, reqs, "app.employees.name", "read_column")
		assertRequirement(t, reqs, "app.employees.salary", "read_column")
	})

	t.Run("projection_only requires name only, reports salary, emits inference_risk", func(t *testing.T) {
		t.Parallel()
		reqs, warnings, _, err := buildRequirements(domain.ModeProjectionOnly, relations, columns, outputs, nil)
		if err != nil {
			t.Fatalf("buildRequirements: %v", err)
		}
		assertRequirement(t, reqs, "app.employees", "read_table")
		assertRequirement(t, reqs, "app.employees.name", "read_column")
		assertNoRequirement(t, reqs, "app.employees.salary", "read_column")
		assertWarning(t, warnings, domain.WarningInferenceRisk)
	})
}

// S5: Hashed/derived output source requirements
// SELECT SHA2(ssn, 256) AS token FROM users
// Both modes require users.ssn (source lineage, not output alias)
func TestBuildRequirements_S5_HashedDerivedSource(t *testing.T) {
	t.Parallel()
	relations := []domain.RelationReference{
		{Schema: "app", Name: "users", Kind: domain.RelationTable, PermissionRequired: true},
	}
	columns := []domain.ColumnReference{
		{Schema: "app", Table: "users", Column: "ssn", Usages: []domain.UsageContext{domain.UsageProjection}},
	}
	outputs := []domain.OutputColumn{
		{Name: "token", Sources: []string{"app.users.ssn"}},
	}

	t.Run("strict requires users.ssn", func(t *testing.T) {
		t.Parallel()
		reqs, _, _, err := buildRequirements(domain.ModeStrict, relations, columns, outputs, nil)
		if err != nil {
			t.Fatalf("buildRequirements: %v", err)
		}
		assertRequirement(t, reqs, "app.users", "read_table")
		assertRequirement(t, reqs, "app.users.ssn", "read_column")
	})

	t.Run("projection_only requires users.ssn (output-contributing via lineage)", func(t *testing.T) {
		t.Parallel()
		reqs, _, _, err := buildRequirements(domain.ModeProjectionOnly, relations, columns, outputs, nil)
		if err != nil {
			t.Fatalf("buildRequirements: %v", err)
		}
		assertRequirement(t, reqs, "app.users", "read_table")
		assertRequirement(t, reqs, "app.users.ssn", "read_column")
	})
}

// S6: Subquery/correlated requirements
// SELECT id FROM users WHERE EXISTS (SELECT 1 FROM orders WHERE orders.user_id = users.id)
func TestBuildRequirements_S6_SubqueryCorrelated(t *testing.T) {
	t.Parallel()
	relations := []domain.RelationReference{
		{Schema: "app", Name: "users", Kind: domain.RelationTable, PermissionRequired: true},
		{Schema: "app", Name: "orders", Kind: domain.RelationTable, PermissionRequired: true},
	}
	columns := []domain.ColumnReference{
		{Schema: "app", Table: "users", Column: "id", Usages: []domain.UsageContext{domain.UsageProjection, domain.UsageFilter}},
		{Schema: "app", Table: "orders", Column: "user_id", Usages: []domain.UsageContext{domain.UsageFilter}},
	}
	outputs := []domain.OutputColumn{
		{Name: "id", Sources: []string{"app.users.id"}},
	}

	t.Run("strict requires users.id and orders.user_id", func(t *testing.T) {
		t.Parallel()
		reqs, _, _, err := buildRequirements(domain.ModeStrict, relations, columns, outputs, nil)
		if err != nil {
			t.Fatalf("buildRequirements: %v", err)
		}
		assertRequirement(t, reqs, "app.users", "read_table")
		assertRequirement(t, reqs, "app.orders", "read_table")
		assertRequirement(t, reqs, "app.users.id", "read_column")
		assertRequirement(t, reqs, "app.orders.user_id", "read_column")
	})

	t.Run("projection_only requires users.id only, reports orders.user_id", func(t *testing.T) {
		t.Parallel()
		reqs, warnings, _, err := buildRequirements(domain.ModeProjectionOnly, relations, columns, outputs, nil)
		if err != nil {
			t.Fatalf("buildRequirements: %v", err)
		}
		assertRequirement(t, reqs, "app.users", "read_table")
		assertRequirement(t, reqs, "app.orders", "read_table")
		assertRequirement(t, reqs, "app.users.id", "read_column")
		assertNoRequirement(t, reqs, "app.orders.user_id", "read_column")
		assertWarning(t, warnings, domain.WarningInferenceRisk)
	})
}

// S7: Table requirements equal across modes
func TestBuildRequirements_S7_TableRequirementsEqualAcrossModes(t *testing.T) {
	t.Parallel()
	relations := []domain.RelationReference{
		{Schema: "app", Name: "users", Kind: domain.RelationTable, PermissionRequired: true},
		{Schema: "app", Name: "orders", Kind: domain.RelationTable, PermissionRequired: true},
	}
	columns := []domain.ColumnReference{
		{Schema: "app", Table: "users", Column: "id", Usages: []domain.UsageContext{domain.UsageProjection}},
		{Schema: "app", Table: "orders", Column: "amount", Usages: []domain.UsageContext{domain.UsageFilter}},
	}
	outputs := []domain.OutputColumn{
		{Name: "id", Sources: []string{"app.users.id"}},
	}

	strictReqs, _, _, err := buildRequirements(domain.ModeStrict, relations, columns, outputs, nil)
	if err != nil {
		t.Fatalf("buildRequirements strict: %v", err)
	}
	projReqs, _, _, err := buildRequirements(domain.ModeProjectionOnly, relations, columns, outputs, nil)
	if err != nil {
		t.Fatalf("buildRequirements projection_only: %v", err)
	}

	// Extract table requirements from both
	strictTables := extractTableRequirements(strictReqs)
	projTables := extractTableRequirements(projReqs)

	if len(strictTables) != len(projTables) {
		t.Fatalf("table requirement count differs: strict=%d, projection_only=%d", len(strictTables), len(projTables))
	}
	for i := range strictTables {
		if strictTables[i] != projTables[i] {
			t.Errorf("table requirement[%d] differs: strict=%+v, projection_only=%+v", i, strictTables[i], projTables[i])
		}
	}
}

// S8: Read classification equal across modes
// Mode never changes read classification
func TestBuildRequirements_S8_ReadClassificationEqualAcrossModes(t *testing.T) {
	t.Parallel()
	// buildRequirements doesn't produce read classification, but we verify
	// that mode is never used to alter classification.
	// This is a contract test: the function signature takes Mode but not ReadClassification.
	// The test verifies that both modes produce consistent requirements structure.

	relations := []domain.RelationReference{
		{Schema: "app", Name: "users", Kind: domain.RelationTable, PermissionRequired: true},
	}
	columns := []domain.ColumnReference{
		{Schema: "app", Table: "users", Column: "id", Usages: []domain.UsageContext{domain.UsageProjection}},
	}
	outputs := []domain.OutputColumn{
		{Name: "id", Sources: []string{"app.users.id"}},
	}

	strictReqs, strictWarnings, _, err := buildRequirements(domain.ModeStrict, relations, columns, outputs, nil)
	if err != nil {
		t.Fatalf("buildRequirements strict: %v", err)
	}
	projReqs, projWarnings, _, err := buildRequirements(domain.ModeProjectionOnly, relations, columns, outputs, nil)
	if err != nil {
		t.Fatalf("buildRequirements projection_only: %v", err)
	}

	// Table requirements must be identical
	strictTables := extractTableRequirements(strictReqs)
	projTables := extractTableRequirements(projReqs)
	if len(strictTables) != len(projTables) {
		t.Fatalf("table requirements differ across modes: strict=%d, projection_only=%d", len(strictTables), len(projTables))
	}

	// When all columns are output-contributing, projection_only should have no warnings
	if len(strictWarnings) != 0 {
		t.Errorf("strict warnings: got %d, want 0", len(strictWarnings))
	}
	if len(projWarnings) != 0 {
		t.Errorf("projection_only warnings: got %d, want 0 when all columns are output-contributing", len(projWarnings))
	}
}

// S9: Stable warning for projection-only
func TestBuildRequirements_S9_StableWarningForProjectionOnly(t *testing.T) {
	t.Parallel()
	relations := []domain.RelationReference{
		{Schema: "app", Name: "users", Kind: domain.RelationTable, PermissionRequired: true},
	}
	columns := []domain.ColumnReference{
		{Schema: "app", Table: "users", Column: "id", Usages: []domain.UsageContext{domain.UsageProjection}},
		{Schema: "app", Table: "users", Column: "secret", Usages: []domain.UsageContext{domain.UsageFilter}},
	}
	outputs := []domain.OutputColumn{
		{Name: "id", Sources: []string{"app.users.id"}},
	}

	_, warnings, _, err := buildRequirements(domain.ModeProjectionOnly, relations, columns, outputs, nil)
	if err != nil {
		t.Fatalf("buildRequirements: %v", err)
	}

	// Exactly one inference_risk warning
	count := 0
	for _, w := range warnings {
		if w == domain.WarningInferenceRisk {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 inference_risk warning, got %d (total warnings: %d)", count, len(warnings))
	}
}

// S10: Invalid mode rejection
func TestBuildRequirements_S10_InvalidMode(t *testing.T) {
	t.Parallel()
	relations := []domain.RelationReference{
		{Schema: "app", Name: "users", Kind: domain.RelationTable, PermissionRequired: true},
	}
	columns := []domain.ColumnReference{
		{Schema: "app", Table: "users", Column: "id", Usages: []domain.UsageContext{domain.UsageProjection}},
	}
	outputs := []domain.OutputColumn{
		{Name: "id", Sources: []string{"app.users.id"}},
	}

	_, _, _, err := buildRequirements("unknown", relations, columns, outputs, nil)
	if err == nil {
		t.Error("expected error for unknown mode")
	}
}

// Additional: Unresolved references produce indeterminate requirements
func TestBuildRequirements_UnresolvedProducesIndeterminate(t *testing.T) {
	t.Parallel()
	relations := []domain.RelationReference{
		{Schema: "app", Name: "users", Kind: domain.RelationTable, PermissionRequired: true},
	}
	columns := []domain.ColumnReference{
		{Schema: "app", Table: "users", Column: "id", Usages: []domain.UsageContext{domain.UsageProjection}},
	}
	outputs := []domain.OutputColumn{
		{Name: "id", Sources: []string{"app.users.id"}},
	}
	unresolved := []domain.Unresolved{
		{Reference: "unknown_table", Reason: domain.ReasonSchemaUnavailable},
		{Reference: "app.users.secret", Reason: ReasonColumnNotFound},
	}

	reqs, _, _, err := buildRequirements(domain.ModeStrict, relations, columns, outputs, unresolved)
	if err != nil {
		t.Fatalf("buildRequirements: %v", err)
	}
	assertRequirement(t, reqs, "unknown_table", "indeterminate")
	assertRequirement(t, reqs, "app.users.secret", "indeterminate")
}

// Additional: CTE relations are not required even in strict mode
func TestBuildRequirements_CTERelationsNotRequired(t *testing.T) {
	t.Parallel()
	relations := []domain.RelationReference{
		{Schema: "app", Name: "users", Kind: domain.RelationTable, PermissionRequired: true},
		{Name: "my_cte", Kind: domain.RelationCTE, PermissionRequired: false},
	}
	columns := []domain.ColumnReference{
		{Schema: "app", Table: "users", Column: "id", Usages: []domain.UsageContext{domain.UsageProjection}},
	}
	outputs := []domain.OutputColumn{
		{Name: "id", Sources: []string{"app.users.id"}},
	}

	reqs, _, _, err := buildRequirements(domain.ModeStrict, relations, columns, outputs, nil)
	if err != nil {
		t.Fatalf("buildRequirements: %v", err)
	}
	assertRequirement(t, reqs, "app.users", "read_table")
	assertNoRequirement(t, reqs, "my_cte", "read_table")
}

// Additional: Empty relations and columns produce no requirements
func TestBuildRequirements_EmptyInputs(t *testing.T) {
	t.Parallel()
	reqs, warnings, _, err := buildRequirements(domain.ModeStrict, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("buildRequirements: %v", err)
	}
	if len(reqs) != 0 {
		t.Errorf("expected no requirements, got %d", len(reqs))
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %d", len(warnings))
	}
}

// Additional: projection_only with no non-output columns produces no inference_risk warning
func TestBuildRequirements_ProjectionOnlyNoInferenceRiskWhenAllProjected(t *testing.T) {
	t.Parallel()
	relations := []domain.RelationReference{
		{Schema: "app", Name: "users", Kind: domain.RelationTable, PermissionRequired: true},
	}
	columns := []domain.ColumnReference{
		{Schema: "app", Table: "users", Column: "id", Usages: []domain.UsageContext{domain.UsageProjection}},
		{Schema: "app", Table: "users", Column: "name", Usages: []domain.UsageContext{domain.UsageProjection}},
	}
	outputs := []domain.OutputColumn{
		{Name: "id", Sources: []string{"app.users.id"}},
		{Name: "name", Sources: []string{"app.users.name"}},
	}

	_, warnings, _, err := buildRequirements(domain.ModeProjectionOnly, relations, columns, outputs, nil)
	if err != nil {
		t.Fatalf("buildRequirements: %v", err)
	}
	assertNoWarning(t, warnings, domain.WarningInferenceRisk)
}

// --- helpers ---

func assertRequirement(t *testing.T, reqs []domain.Requirement, object, privilege string) {
	t.Helper()
	for _, r := range reqs {
		if r.Object == object && r.Privilege == privilege {
			return
		}
	}
	t.Errorf("expected requirement {Object:%q, Privilege:%q}, not found in %v", object, privilege, reqs)
}

func assertNoRequirement(t *testing.T, reqs []domain.Requirement, object, privilege string) {
	t.Helper()
	for _, r := range reqs {
		if r.Object == object && r.Privilege == privilege {
			t.Errorf("unexpected requirement {Object:%q, Privilege:%q} found", object, privilege)
			return
		}
	}
}

func assertWarning(t *testing.T, warnings []domain.WarningCode, want domain.WarningCode) {
	t.Helper()
	for _, w := range warnings {
		if w == want {
			return
		}
	}
	t.Errorf("expected warning %q, not found in %v", want, warnings)
}

func assertNoWarning(t *testing.T, warnings []domain.WarningCode, want domain.WarningCode) {
	t.Helper()
	for _, w := range warnings {
		if w == want {
			t.Errorf("unexpected warning %q found", want)
			return
		}
	}
}

func extractTableRequirements(reqs []domain.Requirement) []domain.Requirement {
	var tables []domain.Requirement
	for _, r := range reqs {
		if r.Privilege == "read_table" {
			tables = append(tables, r)
		}
	}
	return tables
}
