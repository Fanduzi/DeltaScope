//go:build postgresql

package postgresql

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGTableReloptionsDeclaration(t *testing.T) {
	for _, tc := range []struct {
		clause string
		want   []spec.PGTableReloption
	}{
		{"SET (fillfactor=9, fillfactor=80, toast.autovacuum_enabled='opaque_marker')", []spec.PGTableReloption{
			{Name: "fillfactor", HasValue: true, Value: spec.PGTableReloptionInteger, Integer: 9},
			{Name: "fillfactor", HasValue: true, Value: spec.PGTableReloptionInteger, Integer: 80},
			{Name: "autovacuum_enabled", Namespace: "toast", HasValue: true},
		}},
		{"SET (fillfactor=8e1, fillfactor='0x50', fillfactor)", []spec.PGTableReloption{
			{Name: "fillfactor", HasValue: true}, {Name: "fillfactor", HasValue: true}, {Name: "fillfactor"},
		}},
		{`RESET (missing=80, missing, "FillFactor", toast.autovacuum_enabled)`, []spec.PGTableReloption{
			{Name: "missing", HasValue: true}, {Name: "missing"}, {Name: "FillFactor"}, {Name: "autovacuum_enabled", Namespace: "toast"},
		}},
	} {
		t.Run(tc.clause, func(t *testing.T) {
			s := extractPostgreSQLStatement(t, "ALTER TABLE app.t "+tc.clause)
			a := s.DDL.Alter[0]
			if !reflect.DeepEqual(a.PGTableReloptions, tc.want) {
				t.Fatalf("options=%+v, want %+v", a.PGTableReloptions, tc.want)
			}
			b, err := json.Marshal(a)
			if err != nil || strings.Contains(string(b), "opaque_marker") || strings.Contains(string(b), "PGTableReloptions") {
				t.Fatalf("private declarations: %s %v", b, err)
			}
		})
	}
}

func TestPGTableReloptionBooleanForms(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  spec.PGTableReloptionValue
	}{
		{"", spec.PGTableReloptionBoolean}, {"=off", spec.PGTableReloptionBoolean}, {"=TRUE", spec.PGTableReloptionBoolean},
		{"=-0", spec.PGTableReloptionBoolean}, {"=0x1", spec.PGTableReloptionBoolean}, {"='Fa'", spec.PGTableReloptionBoolean},
		{"=' y '", spec.PGTableReloptionInvalidBoolean}, {"='o'", spec.PGTableReloptionInvalidBoolean}, {"=2", spec.PGTableReloptionInvalidBoolean},
		{"=1.0", spec.PGTableReloptionInvalidBoolean}, {"=8e-1", spec.PGTableReloptionUnknown}, {"='不明'", spec.PGTableReloptionUnknown},
	} {
		t.Run(tc.value, func(t *testing.T) {
			s := extractPostgreSQLStatement(t, "ALTER TABLE app.t SET (autovacuum_enabled"+tc.value+")")
			o := s.DDL.Alter[0].PGTableReloptions[0]
			if o.Value != tc.want || o.HasValue != (tc.value != "") {
				t.Fatalf("option=%+v, want %v", o, tc.want)
			}
		})
	}
}
