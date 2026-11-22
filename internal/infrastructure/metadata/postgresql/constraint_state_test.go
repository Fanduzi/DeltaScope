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
			rows := [][]driver.Value{{"ck", "c", false, false, true, nil, false}, {"CK", "u", true, false, nil, nil, nil}, {"fk", "f", false, true, true, nil, nil}, {"pk", "p", true, false, nil, nil, nil}, {"ck", "c", false, false, false, true, true}}
			if empty {
				rows = nil
			}
			db, log := openTestDB(t, map[string]testQueryResult{"from pg_catalog.pg_constraint pc": {columns: []string{"conname", "contype", "convalidated", "derived_fk", "inherited", "partition_children", "validate_children"}, rows: rows}})
			defer db.Close()
			s := &spec.TableSnapshot{Schema: "app", Table: &spec.Table{Name: "t"}}
			if err := NewProvider(db).loadPGConstraintStates(context.Background(), s); err != nil {
				t.Fatal(err)
			}
			if s.PGConstraints == nil || len(s.PGConstraints) != len(rows) {
				t.Fatalf("states=%+v", s.PGConstraints)
			}
			if !empty {
				first, foreignKey, last := s.PGConstraints[0], s.PGConstraints[2], s.PGConstraints[4]
				if !foreignKey.DerivedForeignKey || first.DerivedForeignKey {
					t.Fatalf("incorrect derived foreign-key observation: %+v", s.PGConstraints)
				}
				if !foreignKey.InheritanceKnown || !foreignKey.Inherited {
					t.Fatalf("incorrect partition FK inheritance observation: %+v", foreignKey)
				}
				if !first.CheckValidationChildrenKnown || first.CheckValidationHasChildren || foreignKey.CheckValidationChildrenKnown || !last.CheckValidationChildrenKnown || !last.CheckValidationHasChildren {
					t.Fatalf("incorrect validation traversal observations: %+v", s.PGConstraints)
				}
				if !first.InheritanceKnown || !first.Inherited || !last.InheritanceKnown || last.Inherited || s.PGConstraints[1].InheritanceKnown || first.PartitionCheckHasChildrenKnown || !last.PartitionCheckHasChildrenKnown || !last.PartitionCheckHasChildren {
					t.Fatalf("incorrect inheritance observations: %+v", s.PGConstraints)
				}
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
				"pc.coninhcount OPERATOR(pg_catalog.>) 0",
				"pc.conparentid OPERATOR(pg_catalog.<>) 0::pg_catalog.oid",
				"h.inhparent OPERATOR(pg_catalog.=) tbl.oid and not h.inhdetachpending",
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
		"scan":    {columns: []string{"conname", "contype", "convalidated", "derived_fk", "inherited", "partition_children", "validate_children"}, rows: [][]driver.Value{{"ck", "c", "not-a-bool", false, false, nil, nil}}},
		"iterate": {columns: []string{"conname", "contype", "convalidated", "derived_fk", "inherited", "partition_children", "validate_children"}, rows: [][]driver.Value{{"ck", "c", true, false, false, nil, nil}}, rowErr: failure},
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
