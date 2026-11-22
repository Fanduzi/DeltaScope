package tidbparser

import (
	"context"
	"testing"
)

func TestMySQLProcedureDeterminismDeclaration(t *testing.T) {
	for _, test := range []struct {
		name   string
		clause string
		want   string
	}{
		{"explicit true", " DETERMINISTIC", "true"},
		{"explicit false", " NOT DETERMINISTIC", "false"},
		{"omitted", "", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			sql := "CREATE PROCEDURE d.p()" + test.clause + " SELECT 1"
			parsed, err := New().Parse(context.Background(), sql)
			if err != nil || len(parsed.Statements) != 1 {
				t.Fatalf("parse %q returned %v, %v", sql, parsed, err)
			}
			routine, ok := parsed.Statements[0].(*routineStatement)
			if !ok || routine.declaration == nil {
				t.Fatalf("expected modeled procedure, got %T", parsed.Statements[0])
			}
			got := ""
			for _, characteristic := range routine.declaration.Characteristics {
				if characteristic.Key == "deterministic" {
					got = characteristic.Value
				}
			}
			if got != test.want {
				t.Fatalf("determinism declaration = %q, want %q", got, test.want)
			}
		})
	}
}
