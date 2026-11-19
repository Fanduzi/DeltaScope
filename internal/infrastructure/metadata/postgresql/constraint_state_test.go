package postgresqlmeta

import (
	"context"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestLoadPGConstraintStates(t *testing.T) {
	for _, empty := range []bool{false, true} {
		t.Run(map[bool]string{false: "complete", true: "empty"}[empty], func(t *testing.T) {
			rows := [][]driver.Value{{"ck", "c", false}, {"CK", "u", true}, {"fk", "f", false}, {"pk", "p", true}, {"ck", "c", false}}
			if empty {
				rows = nil
			}
			db, log := openTestDB(t, map[string]testQueryResult{"from pg_catalog.pg_constraint pc": {columns: []string{"conname", "contype", "convalidated"}, rows: rows}})
			defer db.Close()
			s := &spec.TableSnapshot{Schema: "app", Table: &spec.Table{Name: "t"}}
			if err := NewProvider(db).loadPGConstraintStates(context.Background(), s); err != nil {
				t.Fatal(err)
			}
			if s.PGConstraints == nil || len(s.PGConstraints) != len(rows) {
				t.Fatalf("states=%+v", s.PGConstraints)
			}
			if !empty {
				if _, known := s.FindPGConstraint("ck"); known {
					t.Fatal("duplicate became last-wins")
				}
				m, known := s.FindPGConstraint("CK")
				if !known || m == nil || m.Type != "u" || !m.Validated {
					t.Fatalf("quoted=%+v %v", m, known)
				}
			}
			q := log.Queries()
			if len(q) != 1 || containsQueryFragment(q, "conbin") || containsQueryFragment(q, "pg_get_constraintdef") || containsQueryFragment(q, "contype =") {
				t.Fatalf("query=%v", q)
			}
			for _, predicate := range []string{
				"tbl.oid OPERATOR(pg_catalog.=) pc.conrelid",
				"n.oid OPERATOR(pg_catalog.=) tbl.relnamespace",
				"n.nspname OPERATOR(pg_catalog.=) $1::pg_catalog.text",
				"tbl.relname OPERATOR(pg_catalog.=) $2::pg_catalog.text",
			} {
				if !strings.Contains(q[0], predicate) {
					t.Errorf("unbound constraint catalog predicate %q", predicate)
				}
			}
		})
	}
}

func TestLoadPGConstraintStateErrorsStayUnknown(t *testing.T) {
	failure := errors.New("catalog unavailable")
	for name, result := range map[string]testQueryResult{
		"query":   {err: failure},
		"scan":    {columns: []string{"conname", "contype", "convalidated"}, rows: [][]driver.Value{{"ck", "c", "not-a-bool"}}},
		"iterate": {columns: []string{"conname", "contype", "convalidated"}, rows: [][]driver.Value{{"ck", "c", true}}, rowErr: failure},
	} {
		t.Run(name, func(t *testing.T) {
			db, _ := openTestDB(t, map[string]testQueryResult{"from pg_catalog.pg_constraint pc": result})
			defer db.Close()
			s := &spec.TableSnapshot{Schema: "app", Table: &spec.Table{Name: "t"}}
			err := NewProvider(db).loadPGConstraintStates(context.Background(), s)
			if err == nil || s.PGConstraints != nil || name != "scan" && !errors.Is(err, failure) {
				t.Fatalf("err=%v states=%+v", err, s.PGConstraints)
			}
		})
	}
}
