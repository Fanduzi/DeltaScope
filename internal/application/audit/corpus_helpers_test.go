package audit

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// corpusExtractStatement parses and extracts SQL for a given dialect,
// returning the first supported spec.Statement. If all statements are
// unsupported, it returns ok=false.
func corpusExtractStatement(t *testing.T, sql string, dialect spec.Dialect) (spec.Statement, bool) {
	t.Helper()
	parsed, err := Parse(sql, dialect)
	if err != nil {
		t.Fatalf("semantic parse: %v", err)
	}
	statements, err := Extract(parsed)
	if err != nil {
		t.Fatalf("semantic extract: %v", err)
	}
	for i := range statements {
		if statements[i].Unsupported == nil {
			return statements[i], true
		}
	}
	return spec.Statement{}, false
}

// corpusAssertOperation asserts that the operation on the extracted statement
// matches the expected value. It checks DDL.Operation for DDL statements and
// DML.Operation for DML statements.
func corpusAssertOperation(t *testing.T, stmt spec.Statement, expected string) {
	t.Helper()
	var actual string
	switch {
	case stmt.DDL != nil:
		actual = string(stmt.DDL.Operation)
	case stmt.DML != nil:
		actual = string(stmt.DML.Operation)
	default:
		t.Fatal("semantic operation: statement has neither DDL nor DML payload")
	}
	if actual != expected {
		t.Errorf("operation: expected %q, got %q", expected, actual)
	}
}

// corpusAssertConstraintFacts asserts that every expected constraint is present
// in the statement's DDL constraints. Each expected constraint can specify any
// combination of type, name, columns, referenced_schema, referenced_table, and referenced_columns.
// Only non-zero fields in the expected entry are checked.
func corpusAssertConstraintFacts(t *testing.T, stmt spec.Statement, expected []corpusFactConstraint) {
	t.Helper()
	if stmt.DDL == nil {
		t.Fatal("semantic facts: statement has no DDL payload")
	}
	actual := stmt.DDL.Constraints
	for _, exp := range expected {
		found := false
		for _, c := range actual {
			if constraintMatches(c, exp) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("facts.constraints: expected %+v not found in actual constraints %+v", exp, summarizeConstraints(actual))
		}
	}
}

// summarizeConstraints returns a lightweight summary for error messages.
func summarizeConstraints(constraints []spec.Constraint) []map[string]string {
	out := make([]map[string]string, 0, len(constraints))
	for _, c := range constraints {
		m := map[string]string{
			"type": c.Type, "name": c.Name,
			"referenced_schema": c.ReferencedSchema,
			"referenced_table":  c.ReferencedTable,
		}
		out = append(out, m)
	}
	return out
}

// constraintMatches checks whether an actual spec.Constraint matches the
// expected corpusFactConstraint. Only non-zero expected fields are compared.
func constraintMatches(actual spec.Constraint, exp corpusFactConstraint) bool {
	if exp.Type != "" && actual.Type != exp.Type {
		return false
	}
	if exp.Name != "" && actual.Name != exp.Name {
		return false
	}
	if len(exp.Columns) > 0 && !stringSlicesEqual(actual.Columns, exp.Columns) {
		return false
	}
	if exp.ReferencedSchema != "" && actual.ReferencedSchema != exp.ReferencedSchema {
		return false
	}
	if exp.ReferencedTable != "" && actual.ReferencedTable != exp.ReferencedTable {
		return false
	}
	if len(exp.ReferencedColumns) > 0 && !stringSlicesEqual(actual.ReferencedColumns, exp.ReferencedColumns) {
		return false
	}
	return true
}

// stringSlicesEqual compares two string slices for element equality ignoring order.
func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := make(map[string]int, len(a))
	for _, s := range a {
		seen[s]++
	}
	for _, s := range b {
		seen[s]--
		if seen[s] < 0 {
			return false
		}
	}
	return true
}

// corpusAssertUnsupportedMetadata asserts that each expected metadata entry is
// present on a matching unsupported detail in the result. Matching is by feature
// name; metadata values are compared with numeric type coercion (int/int32/int64/float64
// are treated as equivalent when their values match).
func corpusAssertUnsupportedMetadata(t *testing.T, actual []spec.UnsupportedDetail, expected []corpusUnsupported) {
	t.Helper()
	for _, exp := range expected {
		found := false
		for _, u := range actual {
			if u.Feature != exp.Feature {
				continue
			}
			if corpusMetadataMatches(t, exp.Metadata, u.Metadata) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("unsupported.metadata: expected feature=%q metadata=%+v not matched", exp.Feature, exp.Metadata)
		}
	}
}

// corpusMetadataMatches checks whether all expected keys/values are present in
// actual. Numeric types are coerced: int, int32, int64, and float64 are compared
// by value. Other types use direct equality.
func corpusMetadataMatches(t *testing.T, expected, actual map[string]any) bool {
	t.Helper()
	for key, expVal := range expected {
		actVal, ok := actual[key]
		if !ok {
			return false
		}
		if !corpusValueEqual(expVal, actVal) {
			return false
		}
	}
	return true
}

// corpusValueEqual compares two values with numeric type coercion and
// recursive map[string]any support.
func corpusValueEqual(a, b any) bool {
	aFloat, aIsNum := toFloat64(a)
	bFloat, bIsNum := toFloat64(b)
	if aIsNum && bIsNum {
		return aFloat == bFloat
	}
	// Recursive map comparison for nested identity_options etc.
	aMap, aIsMap := a.(map[string]any)
	bMap, bIsMap := b.(map[string]any)
	if aIsMap && bIsMap {
		if len(aMap) != len(bMap) {
			return false
		}
		for k, av := range aMap {
			bv, ok := bMap[k]
			if !ok || !corpusValueEqual(av, bv) {
				return false
			}
		}
		return true
	}
	return a == b
}

// toFloat64 converts numeric types to float64 for comparison.
func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case float64:
		return n, true
	}
	return 0, false
}

// corpusAssertSemantic checks operation and facts against the parsed/extracted
// statement. This is the single entrypoint for all semantic (non-report) assertions.
// Only fields present in the expected YAML are checked.
func corpusAssertSemantic(t *testing.T, sql string, dialect spec.Dialect, tc corpusExpected) {
	t.Helper()
	stmt, ok := corpusExtractStatement(t, sql, dialect)
	if !ok {
		// All statements are unsupported — no semantic checks possible.
		return
	}
	if tc.Expect.Operation != "" {
		corpusAssertOperation(t, stmt, tc.Expect.Operation)
	}
	if tc.Facts != nil && len(tc.Facts.Constraints) > 0 {
		corpusAssertConstraintFacts(t, stmt, tc.Facts.Constraints)
	}
}
