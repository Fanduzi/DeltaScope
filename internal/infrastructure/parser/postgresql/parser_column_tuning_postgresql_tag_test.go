//go:build postgresql

package postgresql

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

func TestPGColumnStatisticsFacts(t *testing.T) {
	for _, tc := range []struct {
		sql       string
		value     *int64
		defaulted bool
	}{
		{"DEFAULT", nil, true}, {"-2", pgTuningInt(-2), false}, {"-1", pgTuningInt(-1), false}, {"0", pgTuningInt(0), false}, {"10000", pgTuningInt(10000), false}, {"10001", pgTuningInt(10001), false},
	} {
		s := extractPostgreSQLStatement(t, `ALTER TABLE app.t ALTER COLUMN "ID" SET STATISTICS `+tc.sql)
		a := s.DDL.Alter[0]
		f := a.PGColumnTuning
		if a.Name != "ID" || f == nil || f.Unmodeled || f.StatisticsDefault != tc.defaulted || (f.StatisticsValue == nil) != (tc.value == nil) || f.StatisticsValue != nil && *f.StatisticsValue != *tc.value {
			t.Fatalf("facts=%+v", f)
		}
	}
}
func pgTuningInt(v int64) *int64 { return &v }

func TestPGColumnTuningOptionsAreLossless(t *testing.T) {
	s := extractPostgreSQLStatement(t, "ALTER TABLE t ALTER COLUMN c SET(n_distinct=-0.5,n_distinct=2,n_distinct_inherited=0,unknown_option=1)")
	f := s.DDL.Alter[0].PGColumnTuning
	if f == nil || f.Unmodeled || len(f.Options) != 4 {
		t.Fatalf("facts=%+v", f)
	}
	for i, want := range []float64{-0.5, 2, 0, 1} {
		if f.Options[i].ValueKind != spec.PGColumnOptionNumeric || f.Options[i].Numeric == nil || *f.Options[i].Numeric != want {
			t.Fatalf("option=%+v", f.Options[i])
		}
	}
	if f.Options[0].Name != "n_distinct" || f.Options[1].Name != "n_distinct" || f.Options[2].Name != "n_distinct_inherited" || f.Options[3].Name != "unknown_option" {
		t.Fatal("keys or duplicate order lost")
	}
	s = extractPostgreSQLStatement(t, "ALTER TABLE t ALTER COLUMN c RESET(n_distinct,unknown_option)")
	f = s.DDL.Alter[0].PGColumnTuning
	if f == nil || len(f.Options) != 2 || f.Options[1].Name != "unknown_option" {
		t.Fatal("RESET key lost")
	}
	for _, o := range f.Options {
		if o.ValueKind != spec.PGColumnOptionAbsent || o.Numeric != nil {
			t.Fatal("RESET absence became numeric zero")
		}
	}
}

func TestPGColumnTuningOpaqueValuesAndPrivacy(t *testing.T) {
	for _, tc := range []struct {
		value string
		kind  spec.PGColumnOptionValueKind
	}{
		{"'private_quoted_payload'", spec.PGColumnOptionQuoted}, {"'NaN'", spec.PGColumnOptionQuoted}, {"1e309", spec.PGColumnOptionUnmodeled}, {"-1e309", spec.PGColumnOptionUnmodeled},
	} {
		s := extractPostgreSQLStatement(t, "ALTER TABLE t ALTER COLUMN c SET(n_distinct="+tc.value+")")
		f := s.DDL.Alter[0].PGColumnTuning
		if f == nil || len(f.Options) != 1 || f.Options[0].ValueKind != tc.kind || f.Options[0].Numeric != nil {
			t.Fatalf("facts=%+v", f)
		}
		raw, _ := json.Marshal(s.DDL.Alter[0])
		if strings.Contains(string(raw), "private_quoted_payload") || strings.Contains(string(raw), "PGColumnTuning") {
			t.Fatal("private declaration leaked")
		}
	}
	s := extractPostgreSQLStatement(t, "ALTER TABLE t ALTER COLUMN c SET(n_distinct=1.7976931348623157e308)")
	f := s.DDL.Alter[0].PGColumnTuning
	if f.Options[0].Numeric == nil || *f.Options[0].Numeric != math.MaxFloat64 {
		t.Fatal("finite DBL_MAX literal lost")
	}
	bad := pgColumnTuningFromCmd(&pg_query.AlterTableCmd{Subtype: pg_query.AlterTableType_AT_SetOptions})
	if !bad.Unmodeled {
		t.Fatal("missing option list became empty valid declaration")
	}
}

func TestPGDropColumnGuardAndCascadeFacts(t *testing.T) {
	for _, tc := range []struct {
		suffix         string
		guard, cascade string
	}{
		{"", "false", "false"}, {" RESTRICT", "false", "false"}, {" CASCADE", "false", "true"}, {" IF EXISTS", "true", "false"}, {" IF EXISTS CASCADE", "true", "true"},
	} {
		guard := ""
		tail := tc.suffix
		if strings.HasPrefix(tail, " IF EXISTS") {
			guard = " IF EXISTS"
			tail = strings.TrimPrefix(tail, " IF EXISTS")
		}
		s := extractPostgreSQLStatement(t, `ALTER TABLE app.t DROP COLUMN`+guard+` "ID"`+tail)
		a := s.DDL.Alter[0]
		if a.Name != "ID" || a.Options["if_exists"] != tc.guard || a.Options["cascade"] != tc.cascade {
			t.Fatalf("alter=%+v", a)
		}
	}
}
