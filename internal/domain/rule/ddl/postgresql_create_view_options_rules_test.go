package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGCreateViewOptions(t *testing.T) {
	valid := spec.PGViewOption{Name: "security_invoker", ValueKnown: true, ValueValid: true}
	check := spec.PGViewOption{Name: "check_option", ValueKnown: true, ValueValid: true}
	for _, tc := range []struct {
		name, reason, gap string
		options           []spec.PGViewOption
		clause, offline   bool
	}{
		{name: "valid", options: []spec.PGViewOption{valid, check}},
		{name: "duplicate", options: []spec.PGViewOption{valid, valid}, reason: "view_option_duplicate"},
		{name: "unknown parameter", options: []spec.PGViewOption{{Name: "fillfactor"}}, reason: "view_option_unrecognized"},
		{name: "namespace", options: []spec.PGViewOption{{Name: "security_invoker", Namespace: "foreign"}}, reason: "view_option_namespace_unrecognized"},
		{name: "bad value", options: []spec.PGViewOption{{Name: "security_invoker", ValueKnown: true}}, reason: "view_option_invalid_value"},
		{name: "unmodeled value", options: []spec.PGViewOption{{Name: "security_invoker"}}, gap: "view.option_value"},
		{name: "duplicate CHECK OPTION", options: []spec.PGViewOption{check}, clause: true, reason: "view_check_option_duplicate"},
		{name: "offline version", options: []spec.PGViewOption{valid}, offline: true, gap: "instance.version"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := spec.Statement{Dialect: spec.DialectPostgreSQL, Kind: spec.KindDDL,
				DDL: &spec.DDL{Operation: spec.DDLOperationCreateView, ObjectName: "v", PGViewOptions: tc.options, Options: map[string]string{}}}
			if !tc.offline {
				s.Metadata = &spec.Metadata{Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"}}
			}
			if tc.clause {
				s.DDL.Options["check_option"] = "true"
			}
			r := pgCreateViewOptionsRule{rule.LevelBlocker}
			if reason, gap := assessPGCreateViewOptions(s); reason != tc.reason || gap != tc.gap {
				t.Fatalf("reason = %q, gap = %q, want %q/%q", reason, gap, tc.reason, tc.gap)
			}
			findings, err := r.Evaluate(context.Background(), s)
			if err != nil || (len(findings) == 1) != (tc.reason != "") || (len(r.EvidenceGaps(s)) == 1) != (tc.gap != "") {
				t.Fatalf("findings = %+v, gaps = %v, error = %v", findings, r.EvidenceGaps(s), err)
			}
		})
	}
}
