// Package ddl verifies object metadata projection into lifecycle rule findings.
// input: synthetic statements with ObjectSnapshot metadata attached
// output: coverage for projectObjectMetadata helper and projection across all PG lifecycle rule families
// pos: domain DDL rule test coverage for metadata object validation projection
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// ---------------------------------------------------------------------------
// Helper unit tests
// ---------------------------------------------------------------------------

func TestProjectObjectMetadataNilDDL(t *testing.T) {
	t.Parallel()
	stmt := spec.Statement{Metadata: &spec.Metadata{}}
	if m := projectObjectMetadata(stmt); m != nil {
		t.Fatalf("expected nil for nil DDL, got %v", m)
	}
}

func TestProjectObjectMetadataNilMetadata(t *testing.T) {
	t.Parallel()
	stmt := spec.Statement{DDL: &spec.DDL{ObjectType: "extension", ObjectName: "pg_trgm"}}
	if m := projectObjectMetadata(stmt); m != nil {
		t.Fatalf("expected nil for nil metadata, got %v", m)
	}
}

func TestProjectObjectMetadataEmptyObjectType(t *testing.T) {
	t.Parallel()
	stmt := spec.Statement{
		DDL:      &spec.DDL{ObjectName: "pg_trgm"},
		Metadata: &spec.Metadata{},
	}
	if m := projectObjectMetadata(stmt); m != nil {
		t.Fatalf("expected nil for empty object type, got %v", m)
	}
}

func TestProjectObjectMetadataNoMatch(t *testing.T) {
	t.Parallel()
	stmt := spec.Statement{
		DDL:      &spec.DDL{ObjectType: "extension", ObjectName: "pg_trgm"},
		Metadata: &spec.Metadata{Objects: []spec.ObjectSnapshot{}},
	}
	if m := projectObjectMetadata(stmt); m != nil {
		t.Fatalf("expected nil for no match, got %v", m)
	}
}

func TestProjectObjectMetadataConfirmed(t *testing.T) {
	t.Parallel()
	stmt := spec.Statement{
		DDL: &spec.DDL{ObjectType: "extension", ObjectName: "pg_trgm"},
		Metadata: &spec.Metadata{
			Objects: []spec.ObjectSnapshot{{
				Type:   "extension",
				Name:   "pg_trgm",
				Status: spec.MetadataStatusConfirmed,
				Exists: true,
				Schema: "public",
				Attributes: map[string]string{
					"extension_version": "1.6",
				},
			}},
		},
	}
	m := projectObjectMetadata(stmt)
	if m == nil {
		t.Fatal("expected non-nil projection")
	}
	if m["metadata_status"] != "confirmed" {
		t.Fatalf("expected confirmed, got %v", m["metadata_status"])
	}
	if m["metadata_exists"] != true {
		t.Fatalf("expected true, got %v", m["metadata_exists"])
	}
	if m["metadata_object_type"] != "extension" {
		t.Fatalf("expected extension, got %v", m["metadata_object_type"])
	}
	if m["metadata_object_name"] != "pg_trgm" {
		t.Fatalf("expected pg_trgm, got %v", m["metadata_object_name"])
	}
	if m["metadata_schema"] != "public" {
		t.Fatalf("expected public, got %v", m["metadata_schema"])
	}
	if v, ok := m["metadata_ambiguous_candidates"]; ok {
		t.Fatalf("expected no ambiguous_candidates for confirmed, got %v", v)
	}
	if m["metadata_extension_version"] != "1.6" {
		t.Fatalf("expected 1.6, got %v", m["metadata_extension_version"])
	}
}

func TestProjectObjectMetadataNotFound(t *testing.T) {
	t.Parallel()
	stmt := spec.Statement{
		DDL: &spec.DDL{ObjectType: "extension", ObjectName: "missing"},
		Metadata: &spec.Metadata{
			Objects: []spec.ObjectSnapshot{{
				Type:   "extension",
				Name:   "missing",
				Status: spec.MetadataStatusNotFound,
			}},
		},
	}
	m := projectObjectMetadata(stmt)
	if m["metadata_status"] != "not_found" {
		t.Fatalf("expected not_found, got %v", m["metadata_status"])
	}
	if m["metadata_exists"] != false {
		t.Fatalf("expected false, got %v", m["metadata_exists"])
	}
	if _, ok := m["metadata_schema"]; ok {
		t.Fatalf("expected no schema for not_found, got %v", m["metadata_schema"])
	}
}

func TestProjectObjectMetadataAmbiguous(t *testing.T) {
	t.Parallel()
	candidates := []string{"public.address", "app.address"}
	stmt := spec.Statement{
		DDL: &spec.DDL{ObjectType: "type", ObjectName: "address"},
		Metadata: &spec.Metadata{
			Objects: []spec.ObjectSnapshot{{
				Type:                "type",
				Name:                "address",
				Status:              spec.MetadataStatusAmbiguous,
				AmbiguousCandidates: candidates,
			}},
		},
	}
	m := projectObjectMetadata(stmt)
	if m["metadata_status"] != "ambiguous" {
		t.Fatalf("expected ambiguous, got %v", m["metadata_status"])
	}
	got, ok := m["metadata_ambiguous_candidates"].([]string)
	if !ok {
		t.Fatalf("expected []string, got %T", m["metadata_ambiguous_candidates"])
	}
	if len(got) != 2 || got[0] != "public.address" || got[1] != "app.address" {
		t.Fatalf("expected candidates %v, got %v", candidates, got)
	}
}

func TestProjectObjectMetadataUnavailable(t *testing.T) {
	t.Parallel()
	stmt := spec.Statement{
		DDL: &spec.DDL{ObjectType: "event_trigger", ObjectName: "trg_ddl"},
		Metadata: &spec.Metadata{
			Objects: []spec.ObjectSnapshot{{
				Type:   "event_trigger",
				Name:   "trg_ddl",
				Status: spec.MetadataStatusUnavailable,
			}},
		},
	}
	m := projectObjectMetadata(stmt)
	if m["metadata_status"] != "unavailable" {
		t.Fatalf("expected unavailable, got %v", m["metadata_status"])
	}
}

func TestProjectObjectMetadataFiltersSensitiveAttributes(t *testing.T) {
	t.Parallel()
	stmt := spec.Statement{
		DDL: &spec.DDL{ObjectType: "subscription", ObjectName: "sub"},
		Metadata: &spec.Metadata{
			Objects: []spec.ObjectSnapshot{{
				Type:   "subscription",
				Name:   "sub",
				Status: spec.MetadataStatusConfirmed,
				Exists: true,
				Attributes: map[string]string{
					"enabled":    "true",
					"connection": "host=secret port=5432",
					"password":   "s3cret",
				},
			}},
		},
	}
	m := projectObjectMetadata(stmt)
	if m["metadata_enabled"] != "true" {
		t.Fatalf("expected enabled=true, got %v", m["metadata_enabled"])
	}
	if _, ok := m["metadata_connection"]; ok {
		t.Fatal("expected connection to be filtered")
	}
	if _, ok := m["metadata_password"]; ok {
		t.Fatal("expected password to be filtered")
	}
}

func TestProjectObjectMetadataWhitelistAllowsAllProjectableKeys(t *testing.T) {
	t.Parallel()
	stmt := spec.Statement{
		DDL: &spec.DDL{ObjectType: "extension", ObjectName: "ext"},
		Metadata: &spec.Metadata{
			Objects: []spec.ObjectSnapshot{{
				Type:   "extension",
				Name:   "ext",
				Status: spec.MetadataStatusConfirmed,
				Exists: true,
				Attributes: map[string]string{
					"type_kind":            "enum",
					"extension_version":    "1.6",
					"enabled":              "true",
					"server":               "srv",
					"foreign_data_wrapper": "fdw",
					"target_type":          "table",
					"has_options":          "true",
					"table":                "users",
				},
			}},
		},
	}
	m := projectObjectMetadata(stmt)
	expected := map[string]string{
		"metadata_type_kind":            "enum",
		"metadata_extension_version":    "1.6",
		"metadata_enabled":              "true",
		"metadata_server":               "srv",
		"metadata_foreign_data_wrapper": "fdw",
		"metadata_target_type":          "table",
		"metadata_has_options":          "true",
		"metadata_table":                "users",
	}
	for k, want := range expected {
		got, ok := m[k]
		if !ok {
			t.Fatalf("expected key %q to be projected", k)
		}
		if got != want {
			t.Fatalf("expected %s=%q, got %q", k, want, got)
		}
	}
}

func TestProjectObjectMetadataWhitelistBlocksUnknownBenignKeys(t *testing.T) {
	t.Parallel()
	stmt := spec.Statement{
		DDL: &spec.DDL{ObjectType: "type", ObjectName: "addr"},
		Metadata: &spec.Metadata{
			Objects: []spec.ObjectSnapshot{{
				Type:   "type",
				Name:   "addr",
				Status: spec.MetadataStatusConfirmed,
				Exists: true,
				Attributes: map[string]string{
					"type_kind":    "composite",
					"owner":        "postgres",
					"function":     "my_func",
					"raw_sql":      "SELECT 1",
					"slot_name":    "my_slot",
					"host":         "10.0.0.1",
					"port":         "5432",
					"conninfo":     "dbname=prod",
					"event_filter": "ddl_command_start",
					"rule_body":    "DO INSTEAD NOTHING",
				},
			}},
		},
	}
	m := projectObjectMetadata(stmt)
	// Whitelisted key should project.
	if m["metadata_type_kind"] != "composite" {
		t.Fatalf("expected metadata_type_kind=composite, got %v", m["metadata_type_kind"])
	}
	// All unknown benign keys must be blocked.
	for _, key := range []string{"owner", "function", "raw_sql", "slot_name", "host", "port", "conninfo", "event_filter", "rule_body"} {
		projected := "metadata_" + key
		if _, ok := m[projected]; ok {
			t.Fatalf("expected whitelist to block %q, but it was projected", projected)
		}
	}
}

func TestProjectObjectMetadataNoAttributes(t *testing.T) {
	t.Parallel()
	stmt := spec.Statement{
		DDL: &spec.DDL{ObjectType: "schema", ObjectName: "app"},
		Metadata: &spec.Metadata{
			Objects: []spec.ObjectSnapshot{{
				Type:   "schema",
				Name:   "app",
				Status: spec.MetadataStatusConfirmed,
				Exists: true,
			}},
		},
	}
	m := projectObjectMetadata(stmt)
	if m["metadata_status"] != "confirmed" {
		t.Fatalf("expected confirmed, got %v", m["metadata_status"])
	}
	// Should have exactly the core fields, no attribute-derived fields.
	coreKeys := 0
	for k := range m {
		if k == "metadata_status" || k == "metadata_exists" || k == "metadata_object_type" || k == "metadata_object_name" {
			coreKeys++
		}
	}
	if coreKeys != 4 {
		t.Fatalf("expected 4 core keys, found %d in %v", coreKeys, m)
	}
}

// ---------------------------------------------------------------------------
// Integration: projection into rule findings across rule families
// ---------------------------------------------------------------------------

func TestExtensionRuleProjectsConfirmedMetadata(t *testing.T) {
	t.Parallel()
	r := mustNewCreateExtensionNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})
	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationCreateExtension,
			ObjectName: "pg_trgm",
			ObjectType: "extension",
		},
		Metadata: &spec.Metadata{
			Objects: []spec.ObjectSnapshot{{
				Type:       "extension",
				Name:       "pg_trgm",
				Status:     spec.MetadataStatusConfirmed,
				Exists:     true,
				Schema:     "public",
				Attributes: map[string]string{"extension_version": "1.6"},
			}},
		},
	}
	findings, err := r.Evaluate(context.Background(), stmt)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	f := findings[0]
	if f.Metadata["metadata_status"] != "confirmed" {
		t.Fatalf("expected confirmed, got %v", f.Metadata["metadata_status"])
	}
	if f.Metadata["metadata_extension_version"] != "1.6" {
		t.Fatalf("expected 1.6, got %v", f.Metadata["metadata_extension_version"])
	}
	// Original fields still present.
	if f.Metadata["object_name"] != "pg_trgm" {
		t.Fatalf("expected original object_name preserved, got %v", f.Metadata["object_name"])
	}
}

func TestDomainRuleProjectsNotFoundMetadata(t *testing.T) {
	t.Parallel()
	r := mustNewDropDomainAdvisoryRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})
	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationDropDomain,
			ObjectName: "missing_domain",
			ObjectType: "domain",
		},
		Metadata: &spec.Metadata{
			Objects: []spec.ObjectSnapshot{{
				Type:   "domain",
				Name:   "missing_domain",
				Status: spec.MetadataStatusNotFound,
			}},
		},
	}
	findings, _ := r.Evaluate(context.Background(), stmt)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Metadata["metadata_status"] != "not_found" {
		t.Fatalf("expected not_found, got %v", findings[0].Metadata["metadata_status"])
	}
	if findings[0].Metadata["metadata_exists"] != false {
		t.Fatalf("expected false, got %v", findings[0].Metadata["metadata_exists"])
	}
}

func TestTypeRuleProjectsAmbiguousMetadata(t *testing.T) {
	t.Parallel()
	r := mustNewDropTypeAdvisoryRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})
	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationDropType,
			ObjectName: "address",
			ObjectType: "type",
		},
		Metadata: &spec.Metadata{
			Objects: []spec.ObjectSnapshot{{
				Type:                "type",
				Name:                "address",
				Status:              spec.MetadataStatusAmbiguous,
				AmbiguousCandidates: []string{"public.address", "app.address"},
			}},
		},
	}
	findings, _ := r.Evaluate(context.Background(), stmt)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Metadata["metadata_status"] != "ambiguous" {
		t.Fatalf("expected ambiguous, got %v", findings[0].Metadata["metadata_status"])
	}
	candidates, ok := findings[0].Metadata["metadata_ambiguous_candidates"].([]string)
	if !ok || len(candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %v", findings[0].Metadata["metadata_ambiguous_candidates"])
	}
}

func TestPublicationRuleProjectsUnavailableMetadata(t *testing.T) {
	t.Parallel()
	r, err := newDropPublicationWarnRule(policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})
	if err != nil {
		t.Fatalf("create rule: %v", err)
	}
	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationDropPublication,
			ObjectName: "pub_all",
			ObjectType: "publication",
		},
		Metadata: &spec.Metadata{
			Objects: []spec.ObjectSnapshot{{
				Type:   "publication",
				Name:   "pub_all",
				Status: spec.MetadataStatusUnavailable,
			}},
		},
	}
	findings, _ := r.Evaluate(context.Background(), stmt)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Metadata["metadata_status"] != "unavailable" {
		t.Fatalf("expected unavailable, got %v", findings[0].Metadata["metadata_status"])
	}
}

func TestSubscriptionRuleFiltersSensitiveMetadata(t *testing.T) {
	t.Parallel()
	r, err := newCreateSubscriptionNoticeRule(policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})
	if err != nil {
		t.Fatalf("create rule: %v", err)
	}
	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationCreateSubscription,
			ObjectName: "sub_prod",
			ObjectType: "subscription",
		},
		Metadata: &spec.Metadata{
			Objects: []spec.ObjectSnapshot{{
				Type:   "subscription",
				Name:   "sub_prod",
				Status: spec.MetadataStatusConfirmed,
				Exists: true,
				Attributes: map[string]string{
					"enabled":    "true",
					"connection": "host=secret",
					"password":   "s3cret",
				},
			}},
		},
	}
	findings, _ := r.Evaluate(context.Background(), stmt)
	f := findings[0]
	if f.Metadata["metadata_enabled"] != "true" {
		t.Fatalf("expected enabled=true, got %v", f.Metadata["metadata_enabled"])
	}
	if _, ok := f.Metadata["metadata_connection"]; ok {
		t.Fatal("expected connection to be filtered")
	}
	if _, ok := f.Metadata["metadata_password"]; ok {
		t.Fatal("expected password to be filtered")
	}
}

func TestForeignServerRuleProjectsWithSafeAttributes(t *testing.T) {
	t.Parallel()
	r := mustNewAlterForeignServerNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})
	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationAlterForeignServer,
			ObjectName: "srv_prod",
			ObjectType: "foreign_server",
		},
		Metadata: &spec.Metadata{
			Objects: []spec.ObjectSnapshot{{
				Type:       "foreign_server",
				Name:       "srv_prod",
				Status:     spec.MetadataStatusConfirmed,
				Exists:     true,
				Schema:     "",
				Attributes: map[string]string{"has_options": "true", "host": "should-be-filtered"},
			}},
		},
	}
	findings, _ := r.Evaluate(context.Background(), stmt)
	f := findings[0]
	if f.Metadata["metadata_has_options"] != "true" {
		t.Fatalf("expected has_options=true, got %v", f.Metadata["metadata_has_options"])
	}
}

func TestEventTriggerRuleProjectsUnavailable(t *testing.T) {
	t.Parallel()
	r := mustNewDropEventTriggerWarnRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})
	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationDropEventTrigger,
			ObjectName: "trg_ddl",
			ObjectType: "event_trigger",
		},
		Metadata: &spec.Metadata{
			Objects: []spec.ObjectSnapshot{{
				Type:   "event_trigger",
				Name:   "trg_ddl",
				Status: spec.MetadataStatusUnavailable,
			}},
		},
	}
	findings, _ := r.Evaluate(context.Background(), stmt)
	if findings[0].Metadata["metadata_status"] != "unavailable" {
		t.Fatalf("expected unavailable, got %v", findings[0].Metadata["metadata_status"])
	}
}

func TestSchemaRuleProjectsConfirmed(t *testing.T) {
	t.Parallel()
	r := mustNewDropSchemaAdvisoryRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})
	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationDropSchema,
			ObjectName: "app",
			ObjectType: "schema",
		},
		Metadata: &spec.Metadata{
			Objects: []spec.ObjectSnapshot{{
				Type:   "schema",
				Name:   "app",
				Status: spec.MetadataStatusConfirmed,
				Exists: true,
			}},
		},
	}
	findings, _ := r.Evaluate(context.Background(), stmt)
	if findings[0].Metadata["metadata_status"] != "confirmed" {
		t.Fatalf("expected confirmed, got %v", findings[0].Metadata["metadata_status"])
	}
}

func TestSequenceRuleProjectsConfirmed(t *testing.T) {
	t.Parallel()
	r := mustNewDropSequenceAdvisoryRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})
	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationDropSequence,
			ObjectName: "seq_id",
			ObjectType: "sequence",
		},
		Metadata: &spec.Metadata{
			Objects: []spec.ObjectSnapshot{{
				Type:   "sequence",
				Name:   "seq_id",
				Status: spec.MetadataStatusConfirmed,
				Exists: true,
			}},
		},
	}
	findings, _ := r.Evaluate(context.Background(), stmt)
	if findings[0].Metadata["metadata_status"] != "confirmed" {
		t.Fatalf("expected confirmed, got %v", findings[0].Metadata["metadata_status"])
	}
}

func TestRuleNoProjectionWhenMetadataNil(t *testing.T) {
	t.Parallel()
	r := mustNewCreateExtensionNoticeRule(t, policy.RulePolicy{Enabled: true, Level: rule.LevelNotice})
	stmt := spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectPostgreSQL,
		DDL: &spec.DDL{
			Operation:  spec.DDLOperationCreateExtension,
			ObjectName: "pg_trgm",
			ObjectType: "extension",
		},
	}
	findings, _ := r.Evaluate(context.Background(), stmt)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if _, ok := findings[0].Metadata["metadata_status"]; ok {
		t.Fatal("expected no metadata_status when statement.Metadata is nil")
	}
}

func TestRuleFindingCountUnchangedWithMetadata(t *testing.T) {
	t.Parallel()
	type ruleFactory func(policy.RulePolicy) (rule.StatementRule, error)
	cases := []struct {
		name   string
		create ruleFactory
		op     spec.DDLOperation
		otype  string
		oname  string
	}{
		{"extension", newCreateExtensionNoticeRule, spec.DDLOperationCreateExtension, "extension", "pg_trgm"},
		{"type", newDropTypeAdvisoryRule, spec.DDLOperationDropType, "type", "my_type"},
		{"domain", newDropDomainAdvisoryRule, spec.DDLOperationDropDomain, "domain", "email"},
		{"publication", newDropPublicationWarnRule, spec.DDLOperationDropPublication, "publication", "pub"},
		{"subscription", newDropSubscriptionWarnRule, spec.DDLOperationDropSubscription, "subscription", "sub"},
		{"foreign_table", newDropForeignTableWarnRule, spec.DDLOperationDropForeignTable, "foreign_table", "ft"},
		{"event_trigger", newDropEventTriggerWarnRule, spec.DDLOperationDropEventTrigger, "event_trigger", "trg"},
		{"schema", newDropSchemaAdvisoryRule, spec.DDLOperationDropSchema, "schema", "app"},
		{"sequence", newDropSequenceAdvisoryRule, spec.DDLOperationDropSequence, "sequence", "seq"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r, err := tc.create(policy.RulePolicy{Enabled: true, Level: rule.LevelWarning})
			if err != nil {
				t.Fatalf("create rule: %v", err)
			}

			// Without metadata: 1 finding.
			plain := spec.Statement{
				Kind:    spec.KindDDL,
				Dialect: spec.DialectPostgreSQL,
				DDL:     &spec.DDL{Operation: tc.op, ObjectName: tc.oname, ObjectType: tc.otype},
			}
			plainFindings, _ := r.Evaluate(context.Background(), plain)

			// With metadata: still 1 finding.
			withMeta := plain
			withMeta.Metadata = &spec.Metadata{
				Objects: []spec.ObjectSnapshot{{
					Type:   tc.otype,
					Name:   tc.oname,
					Status: spec.MetadataStatusConfirmed,
					Exists: true,
				}},
			}
			metaFindings, _ := r.Evaluate(context.Background(), withMeta)

			if len(plainFindings) != 1 {
				t.Fatalf("plain: expected 1 finding, got %d", len(plainFindings))
			}
			if len(metaFindings) != 1 {
				t.Fatalf("with meta: expected 1 finding, got %d", len(metaFindings))
			}
		})
	}
}

// Rule constructor helpers are defined in the existing lifecycle test files:
// - mustNewCreateExtensionNoticeRule → postgresql_extension_lifecycle_rules_test.go
// - mustNewDropDomainAdvisoryRule → postgresql_domain_lifecycle_rules_test.go
// - mustNewDropTypeAdvisoryRule → postgresql_type_lifecycle_rules_test.go
// - mustNewDropPublicationWarnRule → postgresql_replication_lifecycle_rules_test.go
// - mustNewCreateSubscriptionNoticeRule → postgresql_replication_lifecycle_rules_test.go
// - mustNewDropSubscriptionWarnRule → postgresql_replication_lifecycle_rules_test.go
// - mustNewAlterForeignServerNoticeRule → postgresql_foreign_object_lifecycle_rules_test.go
// - mustNewDropForeignTableWarnRule → postgresql_foreign_object_lifecycle_rules_test.go
// - mustNewDropEventTriggerWarnRule → postgresql_event_rule_lifecycle_rules_test.go
// - mustNewDropSchemaAdvisoryRule → postgresql_object_lifecycle_rules_test.go
// - mustNewDropSequenceAdvisoryRule → postgresql_object_lifecycle_rules_test.go
