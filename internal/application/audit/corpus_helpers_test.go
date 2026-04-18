package audit

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// corpusConfigPath writes an optional inline corpus config to a temporary file
// and returns the path used by AuditSQL.
func corpusConfigPath(t *testing.T, tc corpusExpected) string {
	t.Helper()
	if len(tc.Config) == 0 {
		return ""
	}

	raw, err := yaml.Marshal(tc.Config)
	if err != nil {
		t.Fatalf("marshal corpus config: %v", err)
	}

	path := filepath.Join(t.TempDir(), "deltascope-corpus.yaml")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write corpus config: %v", err)
	}
	return path
}

func corpusMetadataFields(tc corpusExpected) (string, MetadataProvider) {
	if tc.Metadata == nil {
		return "", nil
	}
	if tc.Metadata.Instance == nil && len(tc.Metadata.Tables) == 0 && len(tc.Metadata.IndexOwners) == 0 {
		return strings.TrimSpace(tc.Metadata.Schema), nil
	}
	return strings.TrimSpace(tc.Metadata.Schema), newCorpusMetadataProvider(tc.Metadata)
}

type corpusFixtureMetadataProvider struct {
	instance    *spec.InstanceFacts
	tables      map[string]*spec.TableSnapshot
	indexOwners map[string]string
}

func newCorpusMetadataProvider(metadata *corpusMetadata) *corpusFixtureMetadataProvider {
	provider := &corpusFixtureMetadataProvider{
		tables:      make(map[string]*spec.TableSnapshot, len(metadata.Tables)),
		indexOwners: make(map[string]string, len(metadata.IndexOwners)),
	}
	if metadata.Instance != nil {
		provider.instance = &spec.InstanceFacts{
			Version:                   metadata.Instance.Version,
			DefaultCharset:            metadata.Instance.DefaultCharset,
			InnoDBLargePrefixEnabled:  metadata.Instance.InnoDBLargePrefixEnabled,
			InnoDBDefaultRowFormat:    metadata.Instance.InnoDBDefaultRowFormat,
			InnoDBAdaptiveHashEnabled: metadata.Instance.InnoDBAdaptiveHashEnabled,
		}
	}
	for tableName, snapshot := range metadata.Tables {
		converted := corpusTableSnapshotToSpec(tableName, metadata.Schema, snapshot)
		provider.tables[strings.ToLower(strings.TrimSpace(tableName))] = &converted
	}
	for indexName, tableName := range metadata.IndexOwners {
		provider.indexOwners[strings.ToLower(strings.TrimSpace(indexName))] = strings.TrimSpace(tableName)
	}
	return provider
}

func (p *corpusFixtureMetadataProvider) LoadInstanceFacts(context.Context, spec.Dialect, string) (*spec.InstanceFacts, error) {
	return p.instance, nil
}

func (p *corpusFixtureMetadataProvider) LoadTableSnapshot(_ context.Context, _ spec.Dialect, schema string, table string) (*spec.TableSnapshot, error) {
	snapshot, ok := p.tables[strings.ToLower(strings.TrimSpace(table))]
	if !ok {
		return nil, nil
	}
	if snapshot.Schema == "" {
		snapshot.Schema = schema
	}
	if snapshot.Table == nil {
		snapshot.Table = &spec.Table{Schema: snapshot.Schema, Name: table}
	}
	return snapshot, nil
}

func (p *corpusFixtureMetadataProvider) ResolveTableForIndex(_ context.Context, _ spec.Dialect, _ string, index string) (string, error) {
	return p.indexOwners[strings.ToLower(strings.TrimSpace(index))], nil
}

func corpusTableSnapshotToSpec(tableName, defaultSchema string, snapshot corpusTableSnapshot) spec.TableSnapshot {
	out := spec.TableSnapshot{
		Schema:      firstNonEmpty(snapshot.Schema, defaultSchema),
		Exists:      snapshot.Exists,
		Columns:     make([]spec.Column, 0, len(snapshot.Columns)),
		Indexes:     make([]spec.Index, 0, len(snapshot.Indexes)),
		Constraints: make([]spec.Constraint, 0, len(snapshot.Constraints)),
		Options:     cloneStringMap(snapshot.Options),
	}
	if snapshot.Table != nil {
		out.Table = &spec.Table{
			Schema:  snapshot.Table.Schema,
			Name:    snapshot.Table.Name,
			Comment: snapshot.Table.Comment,
		}
	} else if strings.TrimSpace(tableName) != "" {
		out.Table = &spec.Table{Schema: out.Schema, Name: tableName}
	}
	for _, column := range snapshot.Columns {
		out.Columns = append(out.Columns, corpusColumnToSpec(column))
	}
	if snapshot.PrimaryKey != nil {
		index := corpusIndexToSpec(*snapshot.PrimaryKey)
		out.PrimaryKey = &index
	}
	for _, index := range snapshot.Indexes {
		out.Indexes = append(out.Indexes, corpusIndexToSpec(index))
	}
	for _, constraint := range snapshot.Constraints {
		out.Constraints = append(out.Constraints, corpusConstraintToSpec(constraint))
	}
	return out
}

func corpusColumnToSpec(column corpusColumn) spec.Column {
	return spec.Column{
		Name:                      column.Name,
		Type:                      column.Type,
		Length:                    column.Length,
		Charset:                   column.Charset,
		Collation:                 column.Collation,
		Comment:                   column.Comment,
		Unsigned:                  column.Unsigned,
		NotNull:                   column.NotNull,
		AutoIncrement:             column.AutoIncrement,
		HasDefault:                column.HasDefault,
		DefaultValue:              column.DefaultValue,
		DefaultIsNull:             column.DefaultIsNull,
		DefaultIsCurrentTimestamp: column.DefaultIsCurrentTimestamp,
		OnUpdateCurrentTimestamp:  column.OnUpdateCurrentTimestamp,
		GeneratedWhen:             column.GeneratedWhen,
		IsIdentity:                column.IsIdentity,
		IdentityOptions:           column.IdentityOptions,
	}
}

func corpusIndexToSpec(index corpusIndex) spec.Index {
	return spec.Index{
		Name:        index.Name,
		Kind:        spec.IndexKind(index.Kind),
		Columns:     append([]string(nil), index.Columns...),
		Cardinality: index.Cardinality,
	}
}

func corpusConstraintToSpec(constraint corpusConstraint) spec.Constraint {
	return spec.Constraint{
		Type:              constraint.Type,
		Name:              constraint.Name,
		Columns:           append([]string(nil), constraint.Columns...),
		ReferencedSchema:  constraint.ReferencedSchema,
		ReferencedTable:   constraint.ReferencedTable,
		ReferencedColumns: append([]string(nil), constraint.ReferencedColumns...),
	}
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

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

type corpusPostgreSQLAlterFactFile struct {
	Facts *struct {
		Alter []corpusPostgreSQLAlterFact `yaml:"alter"`
	} `yaml:"facts"`
}

type corpusPostgreSQLAlterFact struct {
	Action        string         `yaml:"action"`
	ColumnOldName string         `yaml:"column_old_name,omitempty"`
	Options       map[string]any `yaml:"options,omitempty"`
}

func corpusAssertPostgreSQLAlterFacts(t *testing.T, stmt spec.Statement, rawYAML []byte) {
	t.Helper()
	var expected corpusPostgreSQLAlterFactFile
	if err := yaml.Unmarshal(rawYAML, &expected); err != nil {
		t.Fatalf("parse PostgreSQL alter facts yaml: %v", err)
	}
	if expected.Facts == nil || len(expected.Facts.Alter) == 0 {
		return
	}
	if stmt.DDL == nil {
		t.Fatal("semantic facts: statement has no DDL payload")
	}
	for _, want := range expected.Facts.Alter {
		matched := false
		for i := range stmt.DDL.Alter {
			actual := stmt.DDL.Alter[i]
			if actual.Action != want.Action {
				continue
			}
			if want.ColumnOldName != "" {
				if actual.Column == nil || actual.Column.OldName != want.ColumnOldName {
					continue
				}
			}
			if len(want.Options) > 0 {
				if len(actual.Options) == 0 {
					continue
				}
				optionsMatch := true
				for key, expectedValue := range want.Options {
					actualValue, ok := actual.Options[key]
					if !ok || !corpusValueEqual(expectedValue, actualValue) {
						optionsMatch = false
						break
					}
				}
				if !optionsMatch {
					continue
				}
			}
			matched = true
			break
		}
		if !matched {
			t.Errorf("facts.alter: expected %+v not found in actual alter actions %+v", want, stmt.DDL.Alter)
		}
	}
}
