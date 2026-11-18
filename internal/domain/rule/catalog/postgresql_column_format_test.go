package catalog

import (
	"reflect"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
)

func TestPGColumnFormatDefaultsAndCatalog(t *testing.T) {
	for _, id := range []string{"ddl.pg.alter.column.storage.valid", "ddl.pg.alter.column.compression.valid"} {
		p, ok := policy.Default().Rules[id]
		if !ok || !p.Enabled || p.Level != rule.LevelBlocker || len(p.Params) != 0 {
			t.Fatal("unapproved defaults", id, p)
		}
		e, ok := Lookup(id)
		if !ok || !e.DefaultEnabled || e.DefaultLevel != rule.LevelBlocker || len(e.DefaultParams) != 0 || !e.MetadataAware || !reflect.DeepEqual(e.Dialects, []string{"postgresql"}) || e.MetadataNotes == nil || !reflect.DeepEqual(e.MetadataNotes.Kinds, []MetadataKind{MetadataKindTargetTable, MetadataKindInstance}) {
			t.Fatalf("entry=%+v", e)
		}
		if e.TriggerExample == e.ValidExample || e.MetadataNotes.Required == "" || e.MetadataNotes.Missing == "" {
			t.Fatal("missing exact silent/gap arms", e)
		}
	}
}
