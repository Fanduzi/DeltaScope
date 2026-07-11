package queryaccess_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

func TestResult_JSONRoundTrip(t *testing.T) {
	t.Parallel()
	original := queryaccess.Result{
		Dialect:            "mysql",
		Mode:               queryaccess.ModeStrict,
		ReadClassification: queryaccess.ReadOnly,
		Admission:          queryaccess.Admissible,
		ReasonCodes:        []queryaccess.ReasonCode{queryaccess.ReasonParseFailure},
		Relations: []queryaccess.RelationReference{
			{Schema: "app", Name: "users", Kind: queryaccess.RelationTable, PermissionRequired: true},
			{Schema: "app", Name: "orders", Alias: "o", Kind: queryaccess.RelationTable, PermissionRequired: true},
		},
		ReferencedColumns: []queryaccess.ColumnReference{
			{Schema: "app", Table: "users", Column: "id", Usages: []queryaccess.UsageContext{queryaccess.UsageProjection, queryaccess.UsageFilter}},
		},
		Outputs: []queryaccess.OutputColumn{
			{Name: "user_id", Sources: []string{"app.users.id"}},
		},
		Requirements: []queryaccess.Requirement{
			{Object: "app.users", Privilege: "read_table"},
			{Object: "app.users.id", Privilege: "read_column"},
		},
		Unresolved: []queryaccess.Unresolved{
			{Reference: "unknown_table", Reason: queryaccess.ReasonSchemaUnavailable},
		},
		Warnings: []queryaccess.WarningCode{queryaccess.WarningAmbiguousColumn},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded queryaccess.Result
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Dialect != original.Dialect {
		t.Errorf("dialect: got %q, want %q", decoded.Dialect, original.Dialect)
	}
	if decoded.Mode != original.Mode {
		t.Errorf("mode: got %q, want %q", decoded.Mode, original.Mode)
	}
	if decoded.ReadClassification != original.ReadClassification {
		t.Errorf("read_classification: got %q, want %q", decoded.ReadClassification, original.ReadClassification)
	}
	if decoded.Admission != original.Admission {
		t.Errorf("admission: got %q, want %q", decoded.Admission, original.Admission)
	}
	if len(decoded.Relations) != len(original.Relations) {
		t.Errorf("relations count: got %d, want %d", len(decoded.Relations), len(original.Relations))
	}
	if len(decoded.ReferencedColumns) != len(original.ReferencedColumns) {
		t.Errorf("referenced_columns count: got %d, want %d", len(decoded.ReferencedColumns), len(original.ReferencedColumns))
	}
	if len(decoded.Outputs) != len(original.Outputs) {
		t.Errorf("outputs count: got %d, want %d", len(decoded.Outputs), len(original.Outputs))
	}
	if len(decoded.Requirements) != len(original.Requirements) {
		t.Errorf("requirements count: got %d, want %d", len(decoded.Requirements), len(original.Requirements))
	}
	if len(decoded.Unresolved) != len(original.Unresolved) {
		t.Errorf("unresolved count: got %d, want %d", len(decoded.Unresolved), len(original.Unresolved))
	}
	if len(decoded.Warnings) != len(original.Warnings) {
		t.Errorf("warnings count: got %d, want %d", len(decoded.Warnings), len(original.Warnings))
	}
}

func TestResult_OmittedEmptyFields(t *testing.T) {
	t.Parallel()
	result := queryaccess.Result{
		Dialect:            "postgresql",
		Mode:               queryaccess.ModeStrict,
		ReadClassification: queryaccess.ReadOnly,
		Admission:          queryaccess.Admissible,
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	jsonStr := string(data)
	// Empty slices with omitempty should be omitted
	for _, field := range []string{
		"reason_codes",
		"relations",
		"referenced_columns",
		"outputs",
		"requirements",
		"unresolved",
		"warnings",
	} {
		if strings.Contains(jsonStr, `"`+field+`"`) {
			t.Errorf("empty field %q should be omitted from JSON, got: %s", field, jsonStr)
		}
	}
}

func TestResult_NoForbiddenFields(t *testing.T) {
	t.Parallel()
	result := queryaccess.Result{
		Dialect:            "mysql",
		Mode:               queryaccess.ModeStrict,
		ReadClassification: queryaccess.ReadOnly,
		Admission:          queryaccess.Admissible,
		Relations: []queryaccess.RelationReference{
			{Schema: "app", Name: "users", Kind: queryaccess.RelationTable},
		},
		ReferencedColumns: []queryaccess.ColumnReference{
			{Schema: "app", Table: "users", Column: "id", Usages: []queryaccess.UsageContext{queryaccess.UsageProjection}},
		},
		Outputs: []queryaccess.OutputColumn{
			{Name: "id", Sources: []string{"app.users.id"}},
		},
		Requirements: []queryaccess.Requirement{
			{Object: "app.users", Privilege: "read_table"},
		},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	jsonStr := string(data)
	forbidden := []string{"sql", "raw_sql", "normalized_sql", "severity", "literal", "password", "credential"}
	for _, field := range forbidden {
		// Check for exact field name as a JSON key
		if strings.Contains(jsonStr, `"`+field+`":`) {
			t.Errorf("forbidden field %q found in JSON output: %s", field, jsonStr)
		}
	}
}

func TestRelationReference_JSONRoundTrip(t *testing.T) {
	t.Parallel()
	ref := queryaccess.RelationReference{
		Schema:             "public",
		Name:               "accounts",
		Alias:              "a",
		Kind:               queryaccess.RelationView,
		PermissionRequired: true,
	}

	data, err := json.Marshal(ref)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded queryaccess.RelationReference
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded != ref {
		t.Errorf("round-trip mismatch: got %+v, want %+v", decoded, ref)
	}
}

func TestColumnReference_JSONRoundTrip(t *testing.T) {
	t.Parallel()
	ref := queryaccess.ColumnReference{
		Schema: "app",
		Table:  "users",
		Column: "email",
		Usages: []queryaccess.UsageContext{queryaccess.UsageFilter, queryaccess.UsageProjection},
	}

	data, err := json.Marshal(ref)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded queryaccess.ColumnReference
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Schema != ref.Schema || decoded.Table != ref.Table || decoded.Column != ref.Column {
		t.Errorf("round-trip mismatch: got %+v, want %+v", decoded, ref)
	}
	if len(decoded.Usages) != len(ref.Usages) {
		t.Errorf("usages count: got %d, want %d", len(decoded.Usages), len(ref.Usages))
	}
}

func TestOutputColumn_PreservesDeclarationOrder(t *testing.T) {
	t.Parallel()
	outputs := []queryaccess.OutputColumn{
		{Name: "z_col", Sources: []string{"t.z"}},
		{Name: "a_col", Sources: []string{"t.a"}},
		{Name: "m_col", Sources: []string{"t.m"}},
	}

	data, err := json.Marshal(outputs)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded []queryaccess.OutputColumn
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	for i, out := range decoded {
		if out.Name != outputs[i].Name {
			t.Errorf("output[%d].Name: got %q, want %q", i, out.Name, outputs[i].Name)
		}
	}
}

func TestRequirement_JSONRoundTrip(t *testing.T) {
	t.Parallel()
	req := queryaccess.Requirement{
		Object:    "app.users.email",
		Privilege: "read_column",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded queryaccess.Requirement
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded != req {
		t.Errorf("round-trip mismatch: got %+v, want %+v", decoded, req)
	}
}

func TestUnresolved_JSONRoundTrip(t *testing.T) {
	t.Parallel()
	unres := queryaccess.Unresolved{
		Reference: "unknown_table",
		Reason:    queryaccess.ReasonSchemaUnavailable,
	}

	data, err := json.Marshal(unres)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded queryaccess.Unresolved
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded != unres {
		t.Errorf("round-trip mismatch: got %+v, want %+v", decoded, unres)
	}
}

func TestModeConstants(t *testing.T) {
	t.Parallel()
	if queryaccess.ModeStrict != "strict" {
		t.Errorf("ModeStrict: got %q, want %q", queryaccess.ModeStrict, "strict")
	}
	if queryaccess.ModeProjectionOnly != "projection_only" {
		t.Errorf("ModeProjectionOnly: got %q, want %q", queryaccess.ModeProjectionOnly, "projection_only")
	}
}

func TestReadClassificationConstants(t *testing.T) {
	t.Parallel()
	if queryaccess.ReadOnly != "read_only" {
		t.Errorf("ReadOnly: got %q, want %q", queryaccess.ReadOnly, "read_only")
	}
	if queryaccess.NotReadOnly != "not_read_only" {
		t.Errorf("NotReadOnly: got %q, want %q", queryaccess.NotReadOnly, "not_read_only")
	}
	if queryaccess.Indeterminate != "indeterminate" {
		t.Errorf("Indeterminate: got %q, want %q", queryaccess.Indeterminate, "indeterminate")
	}
}

func TestAdmissionConstants(t *testing.T) {
	t.Parallel()
	if queryaccess.Admissible != "admissible" {
		t.Errorf("Admissible: got %q, want %q", queryaccess.Admissible, "admissible")
	}
	if queryaccess.Rejected != "rejected" {
		t.Errorf("Rejected: got %q, want %q", queryaccess.Rejected, "rejected")
	}
	if queryaccess.IndeterminateAdmission != "indeterminate" {
		t.Errorf("IndeterminateAdmission: got %q, want %q", queryaccess.IndeterminateAdmission, "indeterminate")
	}
}

func TestRelationKindConstants(t *testing.T) {
	t.Parallel()
	if queryaccess.RelationTable != "table" {
		t.Errorf("RelationTable: got %q, want %q", queryaccess.RelationTable, "table")
	}
	if queryaccess.RelationView != "view" {
		t.Errorf("RelationView: got %q, want %q", queryaccess.RelationView, "view")
	}
	if queryaccess.RelationCTE != "cte" {
		t.Errorf("RelationCTE: got %q, want %q", queryaccess.RelationCTE, "cte")
	}
	if queryaccess.RelationDerived != "derived" {
		t.Errorf("RelationDerived: got %q, want %q", queryaccess.RelationDerived, "derived")
	}
}

func TestUsageContextConstants(t *testing.T) {
	t.Parallel()
	expected := map[queryaccess.UsageContext]string{
		queryaccess.UsageProjection: "projection",
		queryaccess.UsageFilter:     "filter",
		queryaccess.UsageJoin:       "join",
		queryaccess.UsageGrouping:   "grouping",
		queryaccess.UsageHaving:     "having",
		queryaccess.UsageOrdering:   "ordering",
		queryaccess.UsageWindow:     "window",
	}
	for uc, want := range expected {
		if string(uc) != want {
			t.Errorf("%T(%q): got %q, want %q", uc, uc, string(uc), want)
		}
	}
}

func TestReasonCodeConstants(t *testing.T) {
	t.Parallel()
	expected := map[queryaccess.ReasonCode]string{
		queryaccess.ReasonParseFailure:       "parse_failure",
		queryaccess.ReasonUnsupportedDialect: "unsupported_dialect",
		queryaccess.ReasonWriteOperation:     "write_operation",
		queryaccess.ReasonMultiStatement:     "multi_statement",
		queryaccess.ReasonSchemaUnavailable:  "schema_unavailable",
		queryaccess.ReasonAmbiguousReference: "ambiguous_reference",
	}
	for rc, want := range expected {
		if string(rc) != want {
			t.Errorf("%T(%q): got %q, want %q", rc, rc, string(rc), want)
		}
	}
}

func TestWarningCodeConstants(t *testing.T) {
	t.Parallel()
	expected := map[queryaccess.WarningCode]string{
		queryaccess.WarningAmbiguousColumn:  "ambiguous_column",
		queryaccess.WarningMissingSchema:    "missing_schema",
		queryaccess.WarningDeprecatedSyntax: "deprecated_syntax",
	}
	for wc, want := range expected {
		if string(wc) != want {
			t.Errorf("%T(%q): got %q, want %q", wc, wc, string(wc), want)
		}
	}
}

func TestResult_EmptyJSONRoundTrip(t *testing.T) {
	t.Parallel()
	var zero queryaccess.Result
	data, err := json.Marshal(zero)
	if err != nil {
		t.Fatalf("marshal zero: %v", err)
	}
	var decoded queryaccess.Result
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal zero: %v", err)
	}
	// Zero value should round-trip cleanly
	if decoded.Dialect != "" {
		t.Errorf("dialect: got %q, want empty", decoded.Dialect)
	}
}
