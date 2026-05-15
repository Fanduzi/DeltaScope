//go:build postgresql

package audit

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestSemanticObjectNoPayloadLeakInNormalizedOptions(t *testing.T) {
	t.Parallel()
	sqls := map[string]string{
		"aggregate":  "CREATE AGGREGATE sum2(integer) (SFUNC = int4pl, STYPE = integer)",
		"operator":   "CREATE OPERATOR === (LEFTARG = integer, RIGHTARG = integer, PROCEDURE = int4eq)",
		"conversion": "CREATE CONVERSION conv FOR 'UTF8' TO 'LATIN1' FROM utf8_to_latin1",
	}
	forbiddenValues := []string{"int4pl", "int4eq", "utf8_to_latin1", "plpython"}
	forbiddenKeys := []string{"sfunc", "stype", "procedure", "function", "definition", "body", "query", "finalfunc", "combinefunc"}

	for name, sql := range sqls {
		statements := parseExtract(t, sql, spec.DialectPostgreSQL)
		if len(statements) == 0 {
			t.Fatalf("%s: no statements returned", name)
		}
		s := statements[0]
		if s.DDL == nil {
			t.Fatalf("%s: DDL is nil", name)
		}
		for k := range s.DDL.Options {
			kl := strings.ToLower(k)
			for _, fk := range forbiddenKeys {
				if strings.Contains(kl, fk) {
					t.Errorf("%s: forbidden key %q in options (got key=%q)", name, fk, k)
				}
			}
		}
		for k, v := range s.DDL.Options {
			vl := strings.ToLower(v)
			for _, fv := range forbiddenValues {
				if strings.Contains(vl, fv) {
					t.Errorf("%s: forbidden value %q in options[%s]=%q", name, fv, k, v)
				}
			}
		}
		for _, fv := range forbiddenValues {
			if strings.Contains(strings.ToLower(s.DDL.ObjectName), fv) {
				t.Errorf("%s: ObjectName %q contains forbidden value %q", name, s.DDL.ObjectName, fv)
			}
		}
		t.Logf("%s: ObjectName=%q ObjectType=%q Options=%v", name, s.DDL.ObjectName, s.DDL.ObjectType, s.DDL.Options)
	}
}

func TestSemanticObjectNoPayloadLeakInFindings(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		sql  string
	}{
		{"create_aggregate", "CREATE AGGREGATE sum2(integer) (SFUNC = int4pl, STYPE = integer)"},
		{"create_operator", "CREATE OPERATOR === (LEFTARG = integer, RIGHTARG = integer, PROCEDURE = int4eq)"},
		{"create_conversion", "CREATE CONVERSION conv FOR 'UTF8' TO 'LATIN1' FROM utf8_to_latin1"},
	}
	forbidden := []string{"int4pl", "int4eq", "utf8_to_latin1", "sfunc", "stype", "finalfunc", "combinefunc"}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result, err := AuditSQL(context.Background(), Request{SQL: tc.sql, Dialect: spec.DialectPostgreSQL})
			if err != nil {
				t.Fatalf("audit error: %v", err)
			}
			var allFindings []rule.Finding
			for _, sr := range result.Statements {
				allFindings = append(allFindings, sr.Findings...)
			}
			for _, f := range allFindings {
				msgLower := strings.ToLower(f.Message)
				for _, fb := range forbidden {
					if strings.Contains(msgLower, fb) {
						t.Errorf("finding message contains forbidden %q: %s", fb, f.Message)
					}
				}
				for k, v := range f.Metadata {
					vs := fmt.Sprintf("%v", v)
					vsLower := strings.ToLower(vs)
					for _, fb := range forbidden {
						if strings.Contains(vsLower, fb) {
							t.Errorf("finding metadata[%s]=%v contains forbidden %q", k, v, fb)
						}
					}
				}
			}
		})
	}
}

func parseExtract(t *testing.T, sql string, dialect spec.Dialect) []spec.Statement {
	t.Helper()
	parsed, err := Parse(context.Background(), sql, dialect)
	if err != nil {
		t.Fatalf("parse %q: %v", sql, err)
	}
	statements, err := Extract(context.Background(), parsed)
	if err != nil {
		t.Fatalf("extract %q: %v", sql, err)
	}
	return statements
}
