package ddl

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func pgFormatStatement(action, method, storage string) spec.Statement {
	s := pgGenerationTransitionStatement(action, &spec.PGColumnState{TypeStorage: storage})
	s.DDL.Alter[0].PGColumnTransition = nil
	if action == "set_storage" {
		s.DDL.Alter[0].Options = map[string]string{"has_storage_setting": "true", "storage_kind": method}
	} else {
		s.DDL.Alter[0].PGCompressionMethod = &method
	}
	s.Metadata.Instance.PGCompressionLZ4Known = true
	s.Metadata.Instance.PGCompressionLZ4Supported = true
	return s
}

func TestPGColumnFormatDeclarationAndTypeArms(t *testing.T) {
	for _, tc := range []struct{ action, method, storage, reason, fact string }{
		{"set_storage", "PLAIN", "p", "", ""}, {"set_storage", "plain", "", "", ""},
		{"set_storage", "DEFAULT", "p", "", ""}, {"set_storage", "default", "x", "", ""},
		{"set_storage", "default", "", "", "column.type_storage"},
		{"set_storage", "ExTeRnAl", "x", "", ""}, {"set_storage", "external", "e", "", ""},
		{"set_storage", "extended", "m", "", ""}, {"set_storage", "main", "x", "", ""},
		{"set_storage", "external", "p", "type_not_toastable", ""},
		{"set_storage", "extended", "p", "type_not_toastable", ""},
		{"set_storage", "main", "p", "type_not_toastable", ""},
		{"set_storage", "extended", "", "", "column.type_storage"},
		{"set_storage", "main", "future", "", "column.type_storage"},
		{"set_storage", "private_mode_payload", "p", "storage_mode_invalid", ""},
		{"set_storage", "PLAİN", "p", "storage_mode_invalid", ""},
		{"set_storage", "", "x", "", "column.storage_strategy"},
		{"set_compression", "default", "p", "", ""}, {"set_compression", "default", "", "", ""},
		{"set_compression", "pglz", "x", "", ""}, {"set_compression", "pglz", "m", "", ""},
		{"set_compression", "pglz", "e", "", ""}, {"set_compression", "lz4", "x", "", ""},
		{"set_compression", "pglz", "p", "type_not_toastable", ""},
		{"set_compression", "lz4", "p", "type_not_toastable", ""},
		{"set_compression", "pglz", "", "", "column.type_storage"},
		{"set_compression", "pglz", "future", "", "column.type_storage"},
		{"set_compression", "PGLZ", "x", "compression_method_invalid", ""},
		{"set_compression", "DEFAULT", "x", "compression_method_invalid", ""},
		{"set_compression", "private_method_payload", "x", "compression_method_invalid", ""},
		{"set_compression", "", "x", "", "column.compression_method"},
	} {
		t.Run(tc.action+"/"+tc.method+"/"+tc.storage, func(t *testing.T) {
			id := ruleIDPGColumnStorageValid
			if tc.action == "set_compression" {
				id = ruleIDPGColumnCompressionValid
			}
			r := pgColumnFormatRule{id, rule.LevelBlocker}
			s := pgFormatStatement(tc.action, tc.method, tc.storage)
			f, err := r.Evaluate(context.Background(), s)
			gaps := r.EvidenceGaps(s)
			if err != nil || (len(f) == 1) != (tc.reason != "") || (len(gaps) == 1) != (tc.fact != "") {
				t.Fatalf("findings=%+v gaps=%+v err=%v", f, gaps, err)
			}
			if tc.reason != "" && f[0].Metadata["reason_code"] != tc.reason {
				t.Fatal(f)
			}
			if tc.fact != "" && (len(gaps[0].RequiredFacts) != 1 || gaps[0].RequiredFacts[0] != tc.fact) {
				t.Fatal(gaps)
			}
			encoded, _ := json.Marshal(f)
			for _, secret := range []string{"private_mode_payload", "private_method_payload", "private_expression_marker"} {
				if strings.Contains(string(encoded), secret) {
					t.Fatalf("raw payload leaked: %s", encoded)
				}
			}
		})
	}
}

func TestPGColumnFormatUnknownDeclarationDoesNotHideInvalidTarget(t *testing.T) {
	for _, action := range []string{"set_storage", "set_compression"} {
		id := ruleIDPGColumnStorageValid
		if action == "set_compression" {
			id = ruleIDPGColumnCompressionValid
		}
		r := pgColumnFormatRule{id, rule.LevelBlocker}
		s := pgFormatStatement(action, "", "x")
		s.DDL.Alter[0].PGCompressionMethod = nil
		s.Metadata.TargetTable.PGColumns.Columns = []spec.PGColumnState{}
		if reason, missing := r.assess(s, 0); reason != "column_absent" || missing != "" {
			t.Fatal(reason, missing)
		}
		s.DDL.Alter[0].Name = "ctid"
		s.Metadata.TargetTable.PGColumns.SystemColumns = []spec.PGColumnState{{Name: "ctid", System: true, TypeStorage: "p"}}
		if reason, missing := r.assess(s, 0); reason != "system_column_inapplicable" || missing != "" {
			t.Fatal(reason, missing)
		}
	}
}

func TestPGColumnFormatLZ4Knownness(t *testing.T) {
	r := pgColumnFormatRule{ruleIDPGColumnCompressionValid, rule.LevelBlocker}
	for _, tc := range []struct {
		known, supported bool
		reason, fact     string
	}{
		{false, false, "", "instance.pg_compression_lz4"}, {false, true, "", "instance.pg_compression_lz4"},
		{true, false, "compression_method_unavailable", ""}, {true, true, "", ""},
	} {
		s := pgFormatStatement("set_compression", "lz4", "x")
		s.Metadata.Instance.PGCompressionLZ4Known = tc.known
		s.Metadata.Instance.PGCompressionLZ4Supported = tc.supported
		reason, fact := r.assess(s, 0)
		if reason != tc.reason || fact != tc.fact {
			t.Fatal(reason, fact, tc)
		}
		for _, method := range []string{"pglz", "default"} {
			s.DDL.Alter[0].PGCompressionMethod = &method
			if reason, fact := r.assess(s, 0); reason != "" || fact != "" {
				t.Fatal("unneeded lz4 premise", reason, fact)
			}
		}
	}
}

func TestPGColumnFormatTargetScopeAndCancellation(t *testing.T) {
	for _, action := range []string{"set_storage", "set_compression"} {
		id := ruleIDPGColumnStorageValid
		method := "extended"
		if action == "set_compression" {
			id = ruleIDPGColumnCompressionValid
			method = "pglz"
		}
		r := pgColumnFormatRule{id, rule.LevelBlocker}
		makeStatement := func() spec.Statement { return pgFormatStatement(action, method, "x") }
		for _, mutate := range []func(*spec.Statement){
			func(s *spec.Statement) { s.Metadata = nil }, func(s *spec.Statement) { s.Metadata.Instance = nil },
			func(s *spec.Statement) { s.Metadata.Instance.Version = "PostgreSQL 18.7" }, func(s *spec.Statement) { s.Metadata.Instance.Version = "17.7" },
			func(s *spec.Statement) { s.Metadata.TargetTable.PGColumns = nil }, func(s *spec.Statement) { s.Metadata.TargetTable.PGColumns.Ordinary = false },
			func(s *spec.Statement) { s.Metadata.TargetTable.PGColumns.Columns = nil }, func(s *spec.Statement) { s.Metadata.Schema = "other" },
			func(s *spec.Statement) { s.Metadata.TargetTable.Schema = "other" }, func(s *spec.Statement) { s.Metadata.TargetTable.Table.Name = "T" },
			func(s *spec.Statement) {
				s.Metadata.TargetTable.PGColumns.Columns = append(s.Metadata.TargetTable.PGColumns.Columns, s.Metadata.TargetTable.PGColumns.Columns[0])
			},
			func(s *spec.Statement) {
				if action == "set_compression" {
					s.DDL.Alter[0].PGCompressionMethod = nil
				} else {
					delete(s.DDL.Alter[0].Options, "has_storage_setting")
				}
			},
		} {
			s := makeStatement()
			mutate(&s)
			f, err := r.Evaluate(context.Background(), s)
			if err != nil || len(f) != 0 || len(r.EvidenceGaps(s)) != 1 {
				t.Fatalf("unknown proof: %+v %+v %v", f, r.EvidenceGaps(s), err)
			}
		}
		s := makeStatement()
		s.Metadata.TargetTable.PGColumns.Columns = []spec.PGColumnState{}
		if reason, fact := r.assess(s, 0); reason != "column_absent" || fact != "" {
			t.Fatal(reason, fact)
		}
		s = makeStatement()
		s.DDL.Alter[0].Name = "ctid"
		s.Metadata.TargetTable.PGColumns.SystemColumns = []spec.PGColumnState{{Name: "ctid", System: true, TypeStorage: "p"}}
		if reason, fact := r.assess(s, 0); reason != "system_column_inapplicable" || fact != "" {
			t.Fatal(reason, fact)
		}
		s.Metadata.TargetTable.PGColumns.SystemColumns = nil
		if reason, fact := r.assess(s, 0); reason != "" || fact != "target_table.pg_columns" {
			t.Fatal(reason, fact)
		}
		s = makeStatement()
		s.DDL.Alter = append(s.DDL.Alter, s.DDL.Alter[0])
		f, err := r.Evaluate(context.Background(), s)
		if err != nil || len(f) != 0 || len(r.EvidenceGaps(s)) != 1 {
			t.Fatal("reused pre-action catalog", f, err)
		}
		for _, d := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
			s = makeStatement()
			s.Dialect = d
			if r.AppliesTo(s) || len(r.EvidenceGaps(s)) != 0 {
				t.Fatal("cross-dialect applicability")
			}
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := r.Evaluate(ctx, makeStatement()); !errors.Is(err, context.Canceled) {
			t.Fatal("lost cancellation", err)
		}
	}
}
