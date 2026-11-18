package catalog

import (
	"reflect"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
)

func TestPGColumnTuningDefaultsAndCatalog(t *testing.T) {
	for _, tc := range []struct {
		id    string
		level rule.Level
	}{
		{"ddl.pg.alter.column.statistics.target.valid", rule.LevelBlocker},
		{"ddl.pg.alter.column.statistics.clamp.warn", rule.LevelWarning},
		{"ddl.pg.alter.column.options.valid", rule.LevelBlocker},
	} {
		p, ok := policy.Default().Rules[tc.id]
		if !ok || !p.Enabled || p.Level != tc.level || len(p.Params) != 0 {
			t.Fatal("unapproved defaults", tc.id, p)
		}
		e, ok := Lookup(tc.id)
		if !ok || !e.DefaultEnabled || e.DefaultLevel != tc.level || len(e.DefaultParams) != 0 || !e.MetadataAware || !reflect.DeepEqual(e.Dialects, []string{"postgresql"}) || e.MetadataNotes == nil || !reflect.DeepEqual(e.MetadataNotes.Kinds, []MetadataKind{MetadataKindTargetTable, MetadataKindInstance}) {
			t.Fatalf("entry=%+v", e)
		}
		if e.TriggerExample == e.ValidExample || e.MetadataNotes.Required == "" || e.MetadataNotes.Missing == "" {
			t.Fatal("missing bounded/silent-arm explanation", e)
		}
	}
	for _, id := range []string{"ddl.alter.drop_column.exists.require", "ddl.pg.alter.drop_column.advisory"} {
		e, ok := Lookup(id)
		if !ok || !e.MetadataAware || e.MetadataNotes == nil || !reflect.DeepEqual(e.MetadataNotes.Kinds, []MetadataKind{MetadataKindTargetTable, MetadataKindInstance}) {
			t.Fatal("PG guard proof dependency missing", e)
		}
	}
}
