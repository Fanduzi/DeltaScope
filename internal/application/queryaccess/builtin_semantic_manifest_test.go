package queryaccess

import (
	"testing"
)

func TestBuiltinSemanticManifest_DeepCopiesSourceAndAccessors(t *testing.T) {
	source := []BuiltinSemanticEntry{{
		Dialect:      "mysql",
		Profile:      AnalysisProfileMySQL57,
		Name:         "count",
		CallClass:    BuiltinSemanticAggregate,
		Arity:        0,
		OperandKinds: []string{"star"},
	}}

	manifest, err := NewBuiltinSemanticManifest(source)
	if err != nil {
		t.Fatalf("new manifest: %v", err)
	}
	source[0].Name = "forged"
	source[0].OperandKinds[0] = "expr"

	entries := manifest.Entries()
	entries[0].Name = "mutated"
	entries[0].OperandKinds[0] = "expr"

	again := manifest.Entries()
	if again[0].Name != "count" || again[0].OperandKinds[0] != "star" {
		t.Fatalf("manifest was mutable through source/accessor: %+v", again)
	}
}

func TestBuiltinSemanticManifest_RejectsInvalidEntries(t *testing.T) {
	cases := []BuiltinSemanticEntry{
		{Profile: AnalysisProfileMySQL57, Name: "count", CallClass: BuiltinSemanticAggregate},
		{Dialect: "mysql", Name: "count", CallClass: BuiltinSemanticAggregate},
		{Dialect: "tidb", Profile: AnalysisProfileMySQL57, Name: "count", CallClass: BuiltinSemanticAggregate},
		{Dialect: "mysql", Profile: AnalysisProfileMySQL57, CallClass: BuiltinSemanticAggregate},
		{Dialect: "mysql", Profile: AnalysisProfileMySQL57, Name: "count", CallClass: "unknown"},
		{Dialect: "mysql", Profile: AnalysisProfileMySQL57, Name: "count", CallClass: BuiltinSemanticAggregate, Arity: -1},
		{Dialect: "mysql", Profile: AnalysisProfileMySQL57, Name: "count", CallClass: BuiltinSemanticAggregate, OperandKinds: []string{"bogus"}},
		{Dialect: "mysql", Profile: AnalysisProfileMySQL57, Name: "count", CallClass: BuiltinSemanticAggregate, OperandKinds: []string{"unknown"}},
	}
	for i, entry := range cases {
		if _, err := NewBuiltinSemanticManifest([]BuiltinSemanticEntry{entry}); err == nil {
			t.Errorf("case %d accepted invalid entry: %+v", i, entry)
		}
	}
}

func TestBuiltinSemanticManifest_MixedConstEntriesExist(t *testing.T) {
	t.Parallel()
	profiles := []AnalysisProfile{AnalysisProfileMySQL57, AnalysisProfileMySQL80, AnalysisProfileMySQL84, AnalysisProfileTiDB85}
	functions := []struct {
		name     string
		minArity int
		arity    int
		kinds    []string
	}{
		{"coalesce", 2, 0, []string{"column", "const"}},
		{"nullif", 0, 2, []string{"column", "const"}},
		{"ifnull", 0, 2, []string{"column", "const"}},
	}
	for _, profile := range profiles {
		manifest := builtinSemanticProductionRegistry.manifest(profile)
		if manifest == nil {
			t.Fatalf("profile %s: nil manifest", profile)
		}
		entries := manifest.Entries()
		for _, fn := range functions {
			found := false
			for _, entry := range entries {
				if entry.Name == fn.name && entry.CallClass == BuiltinSemanticScalar {
					if fn.minArity > 0 && entry.MinArity == fn.minArity && stringSliceEqual(entry.OperandKinds, fn.kinds) {
						found = true
						break
					}
					if fn.arity > 0 && entry.Arity == fn.arity && stringSliceEqual(entry.OperandKinds, fn.kinds) {
						found = true
						break
					}
				}
			}
			if !found {
				t.Errorf("profile %s: missing mixed entry for %s with kinds %v", profile, fn.name, fn.kinds)
			}
		}
	}
}

func TestBuiltinSemanticManifest_LiteralAndReversedEntriesExistForEveryProfile(t *testing.T) {
	t.Parallel()

	profiles := []AnalysisProfile{
		AnalysisProfileMySQL57,
		AnalysisProfileMySQL80,
		AnalysisProfileMySQL84,
		AnalysisProfileTiDB85,
	}
	newEntries := []struct {
		name      string
		callClass BuiltinSemanticCallClass
		arity     int
		kinds     []string
	}{
		{name: "lower", callClass: BuiltinSemanticScalar, arity: 1, kinds: []string{"const"}},
		{name: "upper", callClass: BuiltinSemanticScalar, arity: 1, kinds: []string{"const"}},
		{name: "length", callClass: BuiltinSemanticScalar, arity: 1, kinds: []string{"const"}},
		{name: "char_length", callClass: BuiltinSemanticScalar, arity: 1, kinds: []string{"const"}},
		{name: "abs", callClass: BuiltinSemanticScalar, arity: 1, kinds: []string{"const"}},
		{name: "ceil", callClass: BuiltinSemanticScalar, arity: 1, kinds: []string{"const"}},
		{name: "ceiling", callClass: BuiltinSemanticScalar, arity: 1, kinds: []string{"const"}},
		{name: "floor", callClass: BuiltinSemanticScalar, arity: 1, kinds: []string{"const"}},
		{name: "count", callClass: BuiltinSemanticAggregate, arity: 1, kinds: []string{"const"}},
		{name: "coalesce", callClass: BuiltinSemanticScalar, arity: 2, kinds: []string{"const", "column"}},
		{name: "nullif", callClass: BuiltinSemanticScalar, arity: 2, kinds: []string{"const", "column"}},
		{name: "ifnull", callClass: BuiltinSemanticScalar, arity: 2, kinds: []string{"const", "column"}},
		{name: "coalesce", callClass: BuiltinSemanticScalar, arity: 2, kinds: []string{"const", "const"}},
		{name: "nullif", callClass: BuiltinSemanticScalar, arity: 2, kinds: []string{"const", "const"}},
		{name: "ifnull", callClass: BuiltinSemanticScalar, arity: 2, kinds: []string{"const", "const"}},
	}
	if len(newEntries)*len(profiles) != 60 {
		t.Fatalf("new entry count = %d, want 60", len(newEntries)*len(profiles))
	}

	found := 0
	for _, profile := range profiles {
		manifest := builtinSemanticProductionRegistry.manifest(profile)
		if manifest == nil {
			t.Fatalf("profile %s: nil manifest", profile)
		}
		for _, want := range newEntries {
			matches := 0
			for _, entry := range manifest.Entries() {
				if entry.Name != want.name || entry.CallClass != want.callClass || entry.Arity != want.arity || !stringSliceEqual(entry.OperandKinds, want.kinds) {
					continue
				}
				matches++
				if entry.MinArity != 0 || entry.MaxArity != 0 {
					t.Errorf("profile %s entry %s/%v is variable-arity: %+v", profile, want.name, want.kinds, entry)
				}
				if len(entry.OperandKinds) != entry.Arity {
					t.Errorf("profile %s entry %s/%v has arity %d with %d operand kinds", profile, want.name, want.kinds, entry.Arity, len(entry.OperandKinds))
				}
			}
			if matches != 1 {
				t.Errorf("profile %s entry %s/%v matches = %d, want 1", profile, want.name, want.kinds, matches)
				continue
			}
			found++
		}
	}
	if found != 60 {
		t.Fatalf("fixed-arity new entries found = %d, want 60", found)
	}
}

func TestBuiltinSemanticService_ClonesRegistryAtAssembly(t *testing.T) {
	registry := mustBuiltinTestRegistry(t)
	service, err := newBuiltinSemanticService(&builtinTestResolver{}, registry)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	registry.manifests[AnalysisProfileMySQL57] = nil
	if service.builtinSemantic.registry.manifest(AnalysisProfileMySQL57) == nil {
		t.Fatal("service registry changed after caller mutation")
	}
}
