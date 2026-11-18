package postgresqlmeta

import (
	"context"
	"database/sql/driver"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

var pgColumnCatalogTestColumns = []string{"ordinary", "name", "identity", "generated", "type", "notnull", "hasdef", "pk", "seqtype", "start", "increment", "min", "max", "cache", "cycle"}

func pgColumnCatalogTestRow(name, identity, generated string, typeOID int64, notNull, hasDefault, pk, sequence bool) []driver.Value {
	row := []driver.Value{true, name, identity, generated, typeOID, notNull, hasDefault, pk, nil, nil, nil, nil, nil, nil, nil}
	if sequence {
		copy(row[8:], []driver.Value{int64(20), int64(2147483648), int64(1), int64(1), int64(math.MaxInt64), int64(1), false})
	}
	return row
}

func TestLoadPGColumnCatalog(t *testing.T) {
	rows := [][]driver.Value{
		pgColumnCatalogTestRow("id", "d", "", 23, true, false, true, true),
		pgColumnCatalogTestRow("ID", "a", "", 21, true, false, false, true),
		pgColumnCatalogTestRow("stored", "", "s", 20, false, true, false, false),
		pgColumnCatalogTestRow("serial", "", "", 23, true, true, false, false),
		pgColumnCatalogTestRow("domain", "", "", 123456, true, false, false, false),
	}
	db, log := openTestDB(t, map[string]testQueryResult{"as generation_ordinary": {columns: pgColumnCatalogTestColumns, rows: rows}})
	defer db.Close()
	s := &spec.TableSnapshot{Schema: `quoted'schema`, Table: &spec.Table{Name: `t'quoted`}}
	if err := NewProvider(db).loadPGColumnCatalog(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if s.PGColumns == nil || !s.PGColumns.Ordinary || len(s.PGColumns.Columns) != 5 {
		t.Fatalf("catalog=%+v", s.PGColumns)
	}
	id, ok := s.PGColumns.FindColumn("id")
	if !ok || id == nil || id.Identity != "d" || id.TypeWidth != 32 || !id.NotNull || id.HasDefault || !id.PrimaryKey {
		t.Fatalf("id=%+v known=%v", id, ok)
	}
	if id.IdentitySequence == nil || id.IdentitySequence.TypeWidth != 64 || *id.IdentitySequence.Definition.Start != 2147483648 || *id.IdentitySequence.Definition.MaxValue != math.MaxInt64 {
		t.Fatalf("sequence=%+v", id.IdentitySequence)
	}
	upper, ok := s.PGColumns.FindColumn("ID")
	if !ok || upper == nil || upper.TypeWidth != 16 || upper.Identity != "a" {
		t.Fatalf("quoted=%+v", upper)
	}
	serial, _ := s.PGColumns.FindColumn("serial")
	if serial.Identity != "" || serial.IdentitySequence != nil || !serial.HasDefault {
		t.Fatal("serial became guessed identity")
	}
	domain, _ := s.PGColumns.FindColumn("domain")
	if domain.TypeWidth != -1 {
		t.Fatal("domain guessed as base type")
	}
	q := log.Queries()
	if len(q) != 1 {
		t.Fatal(q)
	}
	for _, bad := range []string{"pg_get_expr", "adbin", "last_value", "nextval", "currval", "setval", s.Schema, s.Table.Name} {
		if strings.Contains(q[0], bad) {
			t.Fatalf("forbidden query member %q", bad)
		}
	}
	for _, good := range []string{"pg_catalog.pg_depend", "dep.deptype = 'i'", "pg_catalog.pg_sequence", "$1", "$2", "pg_catalog.pg_inherits", "c.relpersistence = 'p'", "a.attnum = any(pk.conkey)"} {
		if !strings.Contains(q[0], good) {
			t.Fatalf("missing query premise %q", good)
		}
	}
}

func TestPGColumnCatalogEmptyUnknownAndAmbiguous(t *testing.T) {
	empty := []driver.Value{true, nil, nil, nil, nil, nil, nil, false, nil, nil, nil, nil, nil, nil, nil}
	duplicate := pgColumnCatalogTestRow("id", "d", "", 20, true, false, false, true)
	unsupported := pgColumnCatalogTestRow("id", "d", "", 20, true, false, false, true)
	unsupported[0] = false
	partial := pgColumnCatalogTestRow("id", "d", "", 20, true, false, false, true)
	partial[10] = nil
	for _, tc := range []struct {
		name                     string
		rows                     [][]driver.Value
		known, present, sequence bool
	}{
		{"zero-column", [][]driver.Value{empty}, true, false, false},
		{"table-disappeared", nil, false, false, false},
		{"duplicate-association", [][]driver.Value{duplicate, duplicate}, false, false, false},
		{"unsupported-scope", [][]driver.Value{unsupported}, false, false, false},
		{"partial-sequence", [][]driver.Value{partial}, true, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, _ := openTestDB(t, map[string]testQueryResult{"as generation_ordinary": {columns: pgColumnCatalogTestColumns, rows: tc.rows}})
			defer db.Close()
			s := &spec.TableSnapshot{Schema: "app", Table: &spec.Table{Name: "t"}}
			if err := NewProvider(db).loadPGColumnCatalog(context.Background(), s); err != nil {
				t.Fatal(err)
			}
			m, known := s.PGColumns.FindColumn("id")
			if known != tc.known || (m != nil) != tc.present || m != nil && (m.IdentitySequence != nil) != tc.sequence {
				t.Fatalf("member=%+v known=%v", m, known)
			}
		})
	}
}

func TestPGColumnCatalogErrorsNeverPublishPartialFacts(t *testing.T) {
	failure := errors.New("catalog unavailable")
	row := pgColumnCatalogTestRow("id", "a", "", 20, true, false, false, true)
	bad := append([]driver.Value(nil), row...)
	bad[5] = "bad-bool"
	missing := append([]driver.Value(nil), row...)
	missing[4] = nil
	for name, result := range map[string]testQueryResult{
		"query":        {err: failure},
		"scan":         {columns: pgColumnCatalogTestColumns, rows: [][]driver.Value{row, bad}},
		"missing-core": {columns: pgColumnCatalogTestColumns, rows: [][]driver.Value{row, missing}},
		"iterate":      {columns: pgColumnCatalogTestColumns, rows: [][]driver.Value{row}, rowErr: failure},
		"close":        {columns: pgColumnCatalogTestColumns, rows: [][]driver.Value{row}, closeErr: failure},
	} {
		t.Run(name, func(t *testing.T) {
			db, _ := openTestDB(t, map[string]testQueryResult{"as generation_ordinary": result})
			defer db.Close()
			s := &spec.TableSnapshot{Schema: "app", Table: &spec.Table{Name: "t"}}
			err := NewProvider(db).loadPGColumnCatalog(context.Background(), s)
			if err == nil || s.PGColumns != nil {
				t.Fatalf("err=%v catalog=%+v", err, s.PGColumns)
			}
			if name != "scan" && name != "missing-core" && !errors.Is(err, failure) {
				t.Fatalf("lost error identity: %v", err)
			}
		})
	}
}
