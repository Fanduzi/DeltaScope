package postgresqlmeta

import (
	"context"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGColumnFormatCatalogObservations(t *testing.T) {
	text := pgColumnCatalogTestRow("CTID", "", "", 25, false, false, false, false)
	text[16] = "x"
	system := pgColumnCatalogTestRow("ctid", "", "", 27, true, false, false, false)
	system[15] = int64(-1)
	for _, tc := range []struct {
		name  string
		rows  [][]driver.Value
		users int
	}{
		{"zero-user-columns", [][]driver.Value{system}, 0},
		{"system-and-quoted-user", [][]driver.Value{system, text}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, log := openTestDB(t, map[string]testQueryResult{"as generation_ordinary": {columns: pgColumnCatalogTestColumns, rows: tc.rows}})
			defer db.Close()
			s := &spec.TableSnapshot{Schema: "app", Table: &spec.Table{Name: "t"}}
			if err := NewProvider(db).loadPGColumnCatalog(context.Background(), s); err != nil {
				t.Fatal(err)
			}
			c := s.PGColumns
			if c.Columns == nil || len(c.Columns) != tc.users || len(c.SystemColumns) != 1 {
				t.Fatalf("directory=%+v", c)
			}
			m, known := c.FindColumn("ctid")
			if !known || m == nil || !m.System || m.TypeStorage != "p" {
				t.Fatalf("system=%+v known=%v", m, known)
			}
			m, known = c.FindColumn("CTID")
			if !known || (m != nil) != (tc.users == 1) || m != nil && (m.System || m.TypeStorage != "x") {
				t.Fatalf("user=%+v known=%v", m, known)
			}
			query := strings.Join(log.Queries(), "\n")
			for _, part := range []string{"a.attnum <> 0", "typ.typstorage::text", "pg_catalog.pg_type"} {
				if !strings.Contains(query, part) {
					t.Fatalf("missing %s", part)
				}
			}
			for _, part := range []string{"last_value", "pg_get_expr", "pg_config"} {
				if strings.Contains(query, part) {
					t.Fatalf("unexpected %s", part)
				}
			}
		})
	}
	for _, index := range []int{15, 16} {
		bad := append([]driver.Value(nil), text...)
		bad[index] = nil
		db, _ := openTestDB(t, map[string]testQueryResult{"as generation_ordinary": {columns: pgColumnCatalogTestColumns, rows: [][]driver.Value{text, bad}}})
		s := &spec.TableSnapshot{Schema: "app", Table: &spec.Table{Name: "t"}}
		err := NewProvider(db).loadPGColumnCatalog(context.Background(), s)
		db.Close()
		if err == nil || s.PGColumns != nil {
			t.Fatalf("partial catalog published: %v %+v", err, s.PGColumns)
		}
	}
}

func TestPGCompressionCapabilityObservation(t *testing.T) {
	failure := errors.New("settings unavailable")
	for _, tc := range []struct {
		name                     string
		result                   testQueryResult
		known, supported, failed bool
	}{
		{"compiled-lz4-selected-pglz", testQueryResult{columns: []string{"name", "setting", "lz4"}, rows: [][]driver.Value{{"default_toast_compression", "pglz", true}}}, true, true, false},
		{"compiled-without-lz4", testQueryResult{columns: []string{"name", "setting", "lz4"}, rows: [][]driver.Value{{"default_toast_compression", "pglz", false}}}, true, false, false},
		{"enum-unobserved", testQueryResult{columns: []string{"name", "setting", "lz4"}, rows: [][]driver.Value{{"default_toast_compression", "pglz", nil}}}, false, false, false},
		{"row-unobserved", testQueryResult{columns: []string{"name", "setting", "lz4"}}, false, false, false},
		{"duplicate", testQueryResult{columns: []string{"name", "setting", "lz4"}, rows: [][]driver.Value{{"default_toast_compression", "pglz", true}, {"default_toast_compression", "pglz", false}}}, false, false, true},
		{"query-error", testQueryResult{err: failure}, false, false, true},
		{"scan-error", testQueryResult{columns: []string{"name", "setting", "lz4"}, rows: [][]driver.Value{{"default_toast_compression", "pglz", "not-bool"}}}, false, false, true},
		{"iteration-error", testQueryResult{columns: []string{"name", "setting", "lz4"}, rows: [][]driver.Value{{"default_toast_compression", "pglz", true}}, rowErr: failure}, false, false, true},
		{"close-error", testQueryResult{columns: []string{"name", "setting", "lz4"}, rows: [][]driver.Value{{"default_toast_compression", "pglz", true}}, closeErr: failure}, false, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, log := openTestDB(t, map[string]testQueryResult{"select pg_catalog.version()": {columns: []string{"version"}, rows: [][]driver.Value{{"PostgreSQL 17.7"}}}, "from pg_catalog.pg_settings": tc.result})
			defer db.Close()
			facts, err := NewProvider(db).LoadInstanceFacts(context.Background(), spec.DialectPostgreSQL, "app")
			if (err != nil) != tc.failed || err != nil && facts != nil {
				t.Fatalf("facts=%+v err=%v", facts, err)
			}
			if !tc.failed && (facts.PGCompressionLZ4Known != tc.known || facts.PGCompressionLZ4Supported != tc.supported) {
				t.Fatalf("facts=%+v", facts)
			}
			query := strings.Join(log.Queries(), "\n")
			if !strings.Contains(query, "'lz4' OPERATOR(pg_catalog.=) any(enumvals)") || strings.Contains(query, "pg_config") {
				t.Fatal(query)
			}
		})
	}
}
