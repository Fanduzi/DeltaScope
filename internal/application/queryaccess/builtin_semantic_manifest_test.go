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
