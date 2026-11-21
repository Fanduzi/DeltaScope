package ddl

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGForeignOptionOrder(t *testing.T) {
	for _, tc := range []struct {
		name         string
		existing     []string
		options      []spec.PGForeignOption
		reason, fact string
	}{
		{name: "drop and add", existing: []string{"alpha"}, options: []spec.PGForeignOption{{Name: "alpha", Action: "drop"}, {Name: "alpha", Action: "add"}}},
		{name: "set after drop", existing: []string{"alpha"}, options: []spec.PGForeignOption{{Name: "alpha", Action: "drop"}, {Name: "alpha", Action: "set"}}, reason: "foreign_option_absent"},
		{name: "quoted name differs", existing: []string{"alpha"}, options: []spec.PGForeignOption{{Name: "Alpha", Action: "add"}}},
		{name: "empty directory", existing: []string{}, options: []spec.PGForeignOption{{Name: "alpha", Action: "drop"}}, reason: "foreign_option_absent"},
		{name: "unknown directory", options: []spec.PGForeignOption{{Name: "alpha", Action: "drop"}}, fact: "foreign_options.directory"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reason, fact := assessPGForeignOptions(tc.options, tc.existing)
			if reason != tc.reason || fact != tc.fact {
				t.Fatalf("assessment = (%q, %q), want (%q, %q)", reason, fact, tc.reason, tc.fact)
			}
		})
	}
}
