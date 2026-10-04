// Package spec verifies RENAME COLUMN version applicability.
// input: parsed VersionIdentity values, including missing and out-of-series identities
// output: supported, incompatible, or gap classifications shared by the rule and the state machine
// pos: T05-A5 pure version boundary for RENAME COLUMN, including MySQL 8.0.2 versus 8.0.3
// note: if this file changes, update this header and module README.md.
package spec

import "testing"

func TestRenameColumnVersionSupportFor(t *testing.T) {
	t.Parallel()
	mysql := func(version string, major, minor, patch int, validated bool) *VersionIdentity {
		return &VersionIdentity{Product: VersionProductMySQL, Version: version, Major: major, Minor: minor, Patch: patch, ValidatedRange: validated}
	}
	tidb := func(version string, patch int) *VersionIdentity {
		return &VersionIdentity{Product: VersionProductTiDB, Version: version, Major: 8, Minor: 5, Patch: patch, ValidatedRange: true}
	}
	cases := []struct {
		name         string
		version      *VersionIdentity
		supported    bool
		incompatible bool
		gap          string
		fact         string
	}{
		{name: "nil", gap: "missing_target_version", fact: "target.version"},
		{name: "empty product", version: &VersionIdentity{Version: "8.0.3", Major: 8, Patch: 3, ValidatedRange: true}, gap: "missing_target_version", fact: "target.version"},
		{name: "mysql 5.7", version: mysql("5.7.44", 5, 7, 44, true), incompatible: true},
		{name: "mysql 8.0.0", version: mysql("8.0.0", 8, 0, 0, true), incompatible: true},
		{name: "mysql 8.0.2", version: mysql("8.0.2", 8, 0, 2, true), incompatible: true},
		{name: "mysql 8.0.3", version: mysql("8.0.3", 8, 0, 3, true), supported: true},
		{name: "mysql 8.0.46", version: mysql("8.0.46", 8, 0, 46, true), supported: true},
		{name: "mysql 8.4.10", version: mysql("8.4.10", 8, 4, 10, true), supported: true},
		{name: "tidb 8.5.0", version: tidb("8.5.0", 0), supported: true},
		{name: "mysql 9.0.0", version: mysql("9.0.0", 9, 0, 0, false), gap: "target_version_out_of_validated_range", fact: "target.version.validated_range"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := RenameColumnVersionSupportFor(tc.version)
			if got.Supported != tc.supported || got.Incompatible != tc.incompatible || got.GapReason != tc.gap {
				t.Fatalf("support = %+v", got)
			}
			if tc.fact == "" {
				if len(got.GapFacts) != 0 {
					t.Fatalf("facts = %#v", got.GapFacts)
				}
				return
			}
			if len(got.GapFacts) != 1 || got.GapFacts[0] != tc.fact {
				t.Fatalf("facts = %#v, want [%s]", got.GapFacts, tc.fact)
			}
		})
	}
}
