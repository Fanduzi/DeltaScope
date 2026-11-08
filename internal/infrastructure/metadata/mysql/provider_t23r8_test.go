package mysqlmeta

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// provider_t23r8_test.go pins the split-region-max-num SHOW CONFIG read
// (issue #102 T23-R8): the same all-rows-verification contract as
// max-index-length — one row per tidb-server instance, consistent
// positive values only, and query/scan/iteration failures propagate as
// provider errors instead of degrading into a guessed 1000 default.

func TestLoadInstanceFactsTiDBSplitRegionMaxNum(t *testing.T) {
	tidbVariables := testQueryResult{
		columns: []string{"Variable_name", "Value"},
		rows: [][]driver.Value{
			{"version", "8.0.11-TiDB-v8.5.0"},
			{"character_set_database", "utf8mb4"},
			{"innodb_page_size", "16384"},
		},
	}
	configRow := func(instance, value string) []driver.Value {
		return []driver.Value{"tidb", instance, "split-region-max-num", value}
	}
	configColumns := []string{"Type", "Instance", "Name", "Value"}

	cases := []struct {
		name      string
		config    testQueryResult
		wantValue int
		wantKnown bool
		wantErr   bool
	}{
		{
			name:      "single row becomes known",
			config:    testQueryResult{columns: configColumns, rows: [][]driver.Value{configRow("n1:4000", "500")}},
			wantValue: 500, wantKnown: true,
		},
		{
			name: "same value across instances becomes known",
			config: testQueryResult{columns: configColumns, rows: [][]driver.Value{
				configRow("n1:4000", "1000"), configRow("n2:4000", "1000"), configRow("n3:4000", "1000"),
			}},
			wantValue: 1000, wantKnown: true,
		},
		{
			name: "disagreeing instances stay unknown",
			config: testQueryResult{columns: configColumns, rows: [][]driver.Value{
				configRow("n1:4000", "500"), configRow("n2:4000", "1000"),
			}},
			wantKnown: false,
		},
		{
			name:      "zero rows stays unknown",
			config:    testQueryResult{columns: configColumns, rows: [][]driver.Value{}},
			wantKnown: false,
		},
		{
			name:      "unparsable value stays unknown",
			config:    testQueryResult{columns: configColumns, rows: [][]driver.Value{configRow("n1:4000", "auto")}},
			wantKnown: false,
		},
		{
			name:      "non-positive value stays unknown",
			config:    testQueryResult{columns: configColumns, rows: [][]driver.Value{configRow("n1:4000", "0")}},
			wantKnown: false,
		},
		{
			name: "different config name rows are ignored",
			config: testQueryResult{columns: configColumns, rows: [][]driver.Value{
				{"tidb", "n1:4000", "max-index-length", "3072"},
			}},
			wantKnown: false,
		},
		{
			name:    "query error propagates as provider error",
			config:  testQueryResult{queryErr: errors.New("SHOW CONFIG denied")},
			wantErr: true,
		},
		{
			name: "scan error propagates as provider error",
			config: testQueryResult{
				columns: []string{"Type", "Instance", "Name"},
				rows:    [][]driver.Value{{"tidb", "n1:4000", "split-region-max-num"}},
			},
			wantErr: true,
		},
		{
			name: "mid-iteration error propagates as provider error",
			config: testQueryResult{
				columns:  configColumns,
				rows:     [][]driver.Value{configRow("n1:4000", "500"), configRow("n2:4000", "500")},
				rowErrAt: 1,
				rowErr:   errors.New("connection lost mid-result"),
			},
			wantErr: true,
		},
		{
			name: "invalid value still surfaces following iteration error",
			config: testQueryResult{
				columns:  configColumns,
				rows:     [][]driver.Value{configRow("n1:4000", "auto"), configRow("n2:4000", "500")},
				rowErrAt: 1,
				rowErr:   errors.New("connection lost mid-result"),
			},
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// The sibling SHOW CONFIG loaders share the "show config
			// where" fragment — the longest-match router sends the
			// split-region query to tc.config while max-index-length and
			// alter-primary-key still get a served row set.
			db := openTestDB(t, map[string]testQueryResult{
				"show variables":       tidbVariables,
				"show config where":    {columns: configColumns, rows: [][]driver.Value{{"tidb", "n1:4000", "max-index-length", "3072"}}},
				"split-region-max-num": tc.config,
			})
			defer db.Close()

			facts, err := NewProvider(db).LoadInstanceFacts(context.Background(), spec.DialectTiDB, "app")
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected provider error, got facts=%#v", facts)
				}
				return
			}
			if err != nil {
				t.Fatalf("LoadInstanceFacts: %v", err)
			}
			if facts.TiDBSplitRegionMaxNumKnown != tc.wantKnown {
				t.Fatalf("expected known=%t, got %t (value=%d)", tc.wantKnown, facts.TiDBSplitRegionMaxNumKnown, facts.TiDBSplitRegionMaxNum)
			}
			if tc.wantKnown && facts.TiDBSplitRegionMaxNum != tc.wantValue {
				t.Fatalf("expected value=%d, got %d", tc.wantValue, facts.TiDBSplitRegionMaxNum)
			}
		})
	}
}

func TestLoadInstanceFactsSplitRegionMaxNumNotProbedOnMySQL(t *testing.T) {
	mysqlVariables := testQueryResult{
		columns: []string{"Variable_name", "Value"},
		rows: [][]driver.Value{
			{"version", "8.0.40"},
			{"character_set_database", "utf8mb4"},
			{"innodb_page_size", "16384"},
		},
	}
	db := openTestDB(t, map[string]testQueryResult{
		"show variables":       mysqlVariables,
		"split-region-max-num": {queryErr: errors.New("must not be probed")},
	})
	defer db.Close()

	facts, err := NewProvider(db).LoadInstanceFacts(context.Background(), spec.DialectMySQL, "app")
	if err != nil {
		t.Fatalf("mysql facts must not probe tidb config: %v", err)
	}
	if facts.TiDBSplitRegionMaxNumKnown || facts.TiDBSplitRegionMaxNum != 0 {
		t.Fatalf("mysql must never carry the tidb split fact: %+v", facts)
	}
}
