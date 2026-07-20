// Package queryaccess provides corpus-driven tests for query access analysis.
// input: testdata/query-access/{dialect}/*.sql + *.expected.yaml pairs
// output: deterministic corpus gate verifying Service.Analyze() against expected results
// pos: application corpus test layer for query access analysis foundation
// note: if this file changes, update this header and module README.md.
package queryaccess_test

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	appqa "github.com/Fanduzi/DeltaScope/internal/application/queryaccess"
	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

// corpusExpected is the schema each .expected.yaml file must satisfy.
type corpusExpected struct {
	Name          string `yaml:"name"`
	Dialect       string `yaml:"dialect"`
	Mode          string `yaml:"mode"`
	Profile       string `yaml:"profile,omitempty"`
	DefaultSchema string `yaml:"default_schema,omitempty"`
	SessionProof  string `yaml:"session_proof,omitempty"`
	Expect        struct {
		ReadClassification string              `yaml:"read_classification"`
		Admission          string              `yaml:"admission"`
		Relations          []corpusRelation    `yaml:"relations"`
		ReferencedColumns  []corpusColumn      `yaml:"referenced_columns"`
		Outputs            []corpusOutput      `yaml:"outputs"`
		Requirements       []corpusRequirement `yaml:"requirements"`
		Unresolved         []corpusUnresolved  `yaml:"unresolved"`
		Warnings           []string            `yaml:"warnings"`
		ReasonCodes        []string            `yaml:"reason_codes"`
	} `yaml:"expect"`
}

type corpusRelation struct {
	Schema             string `yaml:"schema,omitempty"`
	Name               string `yaml:"name"`
	Alias              string `yaml:"alias,omitempty"`
	Kind               string `yaml:"kind"`
	PermissionRequired bool   `yaml:"permission_required"`
}

type corpusColumn struct {
	Schema string   `yaml:"schema,omitempty"`
	Table  string   `yaml:"table"`
	Column string   `yaml:"column"`
	Usages []string `yaml:"usages"`
}

type corpusOutput struct {
	Name    string   `yaml:"name"`
	Sources []string `yaml:"sources"`
}

type corpusRequirement struct {
	Object    string `yaml:"object"`
	Privilege string `yaml:"privilege"`
}

type corpusUnresolved struct {
	Reference string `yaml:"reference"`
	Reason    string `yaml:"reason"`
}

// queryAccessDialects lists the dialects this runner exercises without build tags.
var queryAccessDialects = []string{"mysql", "tidb"}

// queryAccessWalkDialects walks only the dialect subdirectories.
func queryAccessWalkDialects(corpusRoot string, dialects []string) ([]string, error) {
	var files []string
	for _, dialect := range dialects {
		dialectDir := filepath.Join(corpusRoot, dialect)
		if _, err := os.Stat(dialectDir); os.IsNotExist(err) {
			continue
		}
		err := filepath.WalkDir(dialectDir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && strings.HasSuffix(path, ".expected.yaml") {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return files, nil
}

// TestQueryAccessCorpus runs all query access corpus fixtures.
func TestQueryAccessCorpus(t *testing.T) {
	t.Parallel()
	corpusRoot := filepath.Join("..", "..", "..", "testdata", "query-access")

	entries, err := queryAccessWalkDialects(corpusRoot, queryAccessDialects)
	if err != nil {
		t.Fatalf("walk error: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("no .expected.yaml files found for query access corpus")
	}

	for _, expPath := range entries {
		rel, _ := filepath.Rel(corpusRoot, expPath)
		t.Run(rel, func(t *testing.T) {
			t.Parallel()
			runQueryAccessCorpusCase(t, expPath)
		})
	}
}

func runQueryAccessCorpusCase(t *testing.T, expPath string) {
	t.Helper()

	// Read sibling .sql.
	sqlPath := expPath[:len(expPath)-len(".expected.yaml")] + ".sql"
	sqlBytes, err := os.ReadFile(sqlPath)
	if err != nil {
		t.Fatalf("read sql: %v", err)
	}

	// Parse expected YAML.
	raw, err := os.ReadFile(expPath)
	if err != nil {
		t.Fatalf("read expected: %v", err)
	}
	var tc corpusExpected
	if err := yaml.Unmarshal(raw, &tc); err != nil {
		t.Fatalf("parse expected yaml: %v", err)
	}

	// Validate required fields.
	if tc.Name == "" {
		t.Fatal("missing required field: name")
	}
	if tc.Dialect == "" {
		t.Fatal("missing required field: dialect")
	}

	// Run service.
	svc := newCorpusService(t, tc)
	result, err := svc.Analyze(context.Background(), appqa.QueryAccessRequest{
		SQL:             string(sqlBytes),
		Dialect:         tc.Dialect,
		Mode:            tc.Mode,
		AnalysisProfile: appqa.AnalysisProfile(tc.Profile),
		DefaultSchema:   tc.DefaultSchema,
	})
	if err != nil {
		t.Fatalf("analyze error: %v", err)
	}
	dr := result.DomainResult

	// Assert read_classification.
	if tc.Expect.ReadClassification != "" {
		if string(dr.ReadClassification) != tc.Expect.ReadClassification {
			t.Errorf("read_classification: expected %q, got %q", tc.Expect.ReadClassification, dr.ReadClassification)
		}
	}

	// Assert admission.
	if tc.Expect.Admission != "" {
		if string(dr.Admission) != tc.Expect.Admission {
			t.Errorf("admission: expected %q, got %q", tc.Expect.Admission, dr.Admission)
		}
	}

	// Assert relations.
	assertRelations(t, tc.Expect.Relations, dr.Relations)

	// Assert referenced_columns.
	assertColumns(t, tc.Expect.ReferencedColumns, dr.ReferencedColumns)

	// Assert outputs.
	assertOutputs(t, tc.Expect.Outputs, dr.Outputs)

	// Assert requirements.
	assertRequirements(t, tc.Expect.Requirements, dr.Requirements)

	// Assert unresolved.
	assertUnresolved(t, tc.Expect.Unresolved, dr.Unresolved)

	// Assert warnings.
	assertStringSlices(t, "warnings", tc.Expect.Warnings, warningStrings(dr.Warnings))

	// Assert reason_codes.
	assertStringSlices(t, "reason_codes", tc.Expect.ReasonCodes, reasonStrings(dr.ReasonCodes))

	t.Logf("OK: %s (%s/%s) classification=%s admission=%s",
		tc.Name, tc.Dialect, tc.Mode, dr.ReadClassification, dr.Admission)
}

func assertRelations(t *testing.T, expected []corpusRelation, actual []domain.RelationReference) {
	t.Helper()
	if expected == nil {
		return
	}
	if len(actual) != len(expected) {
		t.Errorf("relations: expected %d, got %d", len(expected), len(actual))
		for i, r := range actual {
			t.Logf("  actual[%d]: schema=%q name=%q alias=%q kind=%q perm=%v", i, r.Schema, r.Name, r.Alias, r.Kind, r.PermissionRequired)
		}
		return
	}
	for i, exp := range expected {
		act := actual[i]
		if act.Schema != exp.Schema {
			t.Errorf("relations[%d].schema: expected %q, got %q", i, exp.Schema, act.Schema)
		}
		if act.Name != exp.Name {
			t.Errorf("relations[%d].name: expected %q, got %q", i, exp.Name, act.Name)
		}
		if act.Alias != exp.Alias {
			t.Errorf("relations[%d].alias: expected %q, got %q", i, exp.Alias, act.Alias)
		}
		if string(act.Kind) != exp.Kind {
			t.Errorf("relations[%d].kind: expected %q, got %q", i, exp.Kind, act.Kind)
		}
		if act.PermissionRequired != exp.PermissionRequired {
			t.Errorf("relations[%d].permission_required: expected %v, got %v", i, exp.PermissionRequired, act.PermissionRequired)
		}
	}
}

func assertColumns(t *testing.T, expected []corpusColumn, actual []domain.ColumnReference) {
	t.Helper()
	if expected == nil {
		return
	}
	if len(actual) != len(expected) {
		t.Errorf("referenced_columns: expected %d, got %d", len(expected), len(actual))
		for i, c := range actual {
			t.Logf("  actual[%d]: schema=%q table=%q column=%q usages=%v", i, c.Schema, c.Table, c.Column, c.Usages)
		}
		return
	}
	for i, exp := range expected {
		act := actual[i]
		if act.Schema != exp.Schema {
			t.Errorf("columns[%d].schema: expected %q, got %q", i, exp.Schema, act.Schema)
		}
		if act.Table != exp.Table {
			t.Errorf("columns[%d].table: expected %q, got %q", i, exp.Table, act.Table)
		}
		if act.Column != exp.Column {
			t.Errorf("columns[%d].column: expected %q, got %q", i, exp.Column, act.Column)
		}
		assertStringSlices(t, "columns["+itoa(i)+"].usages", exp.Usages, usageStrings(act.Usages))
	}
}

func assertOutputs(t *testing.T, expected []corpusOutput, actual []domain.OutputColumn) {
	t.Helper()
	if expected == nil {
		return
	}
	if len(actual) != len(expected) {
		t.Errorf("outputs: expected %d, got %d", len(expected), len(actual))
		for i, o := range actual {
			t.Logf("  actual[%d]: name=%q sources=%v", i, o.Name, o.Sources)
		}
		return
	}
	for i, exp := range expected {
		act := actual[i]
		if act.Name != exp.Name {
			t.Errorf("outputs[%d].name: expected %q, got %q", i, exp.Name, act.Name)
		}
		assertStringSlices(t, "outputs["+itoa(i)+"].sources", exp.Sources, act.Sources)
	}
}

func assertRequirements(t *testing.T, expected []corpusRequirement, actual []domain.Requirement) {
	t.Helper()
	if expected == nil {
		return
	}
	if len(actual) != len(expected) {
		t.Errorf("requirements: expected %d, got %d", len(expected), len(actual))
		for i, r := range actual {
			t.Logf("  actual[%d]: object=%q privilege=%q", i, r.Object, r.Privilege)
		}
		return
	}
	for i, exp := range expected {
		act := actual[i]
		if act.Object != exp.Object {
			t.Errorf("requirements[%d].object: expected %q, got %q", i, exp.Object, act.Object)
		}
		if act.Privilege != exp.Privilege {
			t.Errorf("requirements[%d].privilege: expected %q, got %q", i, exp.Privilege, act.Privilege)
		}
	}
}

func assertUnresolved(t *testing.T, expected []corpusUnresolved, actual []domain.Unresolved) {
	t.Helper()
	if expected == nil {
		expected = []corpusUnresolved{}
	}
	if actual == nil {
		actual = []domain.Unresolved{}
	}
	if len(actual) != len(expected) {
		t.Errorf("unresolved: expected %d, got %d", len(expected), len(actual))
		for i, u := range actual {
			t.Logf("  actual[%d]: reference=%q reason=%q", i, u.Reference, u.Reason)
		}
		return
	}
	// Sort both for deterministic comparison.
	sort.Slice(expected, func(i, j int) bool { return expected[i].Reference < expected[j].Reference })
	sort.Slice(actual, func(i, j int) bool { return actual[i].Reference < actual[j].Reference })
	for i, exp := range expected {
		act := actual[i]
		if act.Reference != exp.Reference {
			t.Errorf("unresolved[%d].reference: expected %q, got %q", i, exp.Reference, act.Reference)
		}
		if string(act.Reason) != exp.Reason {
			t.Errorf("unresolved[%d].reason: expected %q, got %q", i, exp.Reason, act.Reason)
		}
	}
}

func assertStringSlices(t *testing.T, field string, expected, actual []string) {
	t.Helper()
	if expected == nil {
		expected = []string{}
	}
	if actual == nil {
		actual = []string{}
	}
	if len(actual) != len(expected) {
		t.Errorf("%s: expected %v, got %v", field, expected, actual)
		return
	}
	// Sort both for deterministic comparison.
	expSorted := append([]string(nil), expected...)
	actSorted := append([]string(nil), actual...)
	sort.Strings(expSorted)
	sort.Strings(actSorted)
	for i, exp := range expSorted {
		if actSorted[i] != exp {
			t.Errorf("%s: expected %v, got %v", field, expected, actual)
			return
		}
	}
}

func warningStrings(w []domain.WarningCode) []string {
	if len(w) == 0 {
		return nil
	}
	s := make([]string, len(w))
	for i, v := range w {
		s[i] = string(v)
	}
	return s
}

func reasonStrings(r []domain.ReasonCode) []string {
	if len(r) == 0 {
		return nil
	}
	s := make([]string, len(r))
	for i, v := range r {
		s[i] = string(v)
	}
	return s
}

func usageStrings(u []domain.UsageContext) []string {
	if len(u) == 0 {
		return nil
	}
	s := make([]string, len(u))
	for i, v := range u {
		s[i] = string(v)
	}
	return s
}

func itoa(i int) string {
	return string(rune('0' + i))
}
