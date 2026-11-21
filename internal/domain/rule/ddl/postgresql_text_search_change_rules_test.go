package ddl

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGTextSearchChangeAssessment(t *testing.T) {
	d := &spec.PGTextSearchChange{Action: "add_mapping", Tokens: []string{"word", "absent"}, Dictionaries: []spec.Table{{Schema: "app", Name: "missing"}}}
	state := &spec.PGTextSearchChangeState{BuiltinParser: true, Mappings: []spec.PGTextSearchMapping{{Token: "word", Mapped: true}},
		Dictionaries: d.Dictionaries, DictionaryExists: []bool{false}}
	if reason, fact := assessPGTextSearchMapping(d, state); reason != "text_search_token_absent" || fact != "" {
		t.Fatalf("token resolution = (%q, %q)", reason, fact)
	}
	d.Tokens = []string{"word"}
	if reason, fact := assessPGTextSearchMapping(d, state); reason != "text_search_dictionary_absent" || fact != "" {
		t.Fatalf("dictionary resolution = (%q, %q)", reason, fact)
	}
	d.Action, d.IfExists, d.Dictionaries = "drop_mapping", true, nil
	state.Dictionaries, state.DictionaryExists = nil, nil
	state.Mappings[0].Mapped = false
	if reason, fact := assessPGTextSearchMapping(d, state); reason != "" || fact != "" {
		t.Fatalf("guarded mapping = (%q, %q)", reason, fact)
	}
	options := []spec.PGTextSearchDictionaryOption{
		{Name: "accept", Present: true, BooleanKnown: true},
		{Name: "accept", Present: true, BooleanKnown: true, BooleanValid: true},
		{Name: "unknown"},
	}
	if reason, fact := assessPGTextSearchDictionary(&spec.PGTextSearchChange{Options: options}, &spec.PGTextSearchChangeState{SimpleTemplate: true}); reason != "" || fact != "" {
		t.Fatalf("effective dictionary options = (%q, %q)", reason, fact)
	}
}
