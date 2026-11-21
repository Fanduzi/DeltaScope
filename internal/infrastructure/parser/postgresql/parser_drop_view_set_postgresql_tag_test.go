//go:build postgresql

package postgresql

import (
	"slices"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGDropViewSetPreservesExactOrderedTargets(t *testing.T) {
	s := extractPostgreSQLStatement(t, `DROP VIEW IF EXISTS app."First, View", other.second, app."First, View" RESTRICT`)
	lookup := spec.PGDropViewSetLookup(s)
	want := []spec.PGDestructiveTableTarget{
		{Table: spec.Table{Schema: "app", Name: "First, View"}},
		{Table: spec.Table{Schema: "other", Name: "second"}},
		{Table: spec.Table{Schema: "app", Name: "First, View"}},
	}
	if targets := s.DDL.TableTargets(); !slices.Equal(targets, []spec.Table{want[0].Table, want[1].Table, want[2].Table}) {
		t.Fatalf("public targets = %+v", targets)
	}
	if lookup == nil || !lookup.PGDropIfExists || lookup.Qualifiers["cascade"] != "" || !slices.Equal(lookup.PGDestructiveTargets, want) {
		t.Fatalf("lookup = %+v, want ordered targets %+v", lookup, want)
	}
}
