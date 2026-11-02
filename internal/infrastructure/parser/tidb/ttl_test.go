package tidbparser

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// T19: source attribution is lexical, not a bare-word scan — table or column
// names that happen to spell ttl/ttl_enable/remove.ttl must never turn an
// executable feature comment into an unproven/native clause.
func TestT19TTLClauseSourceAttribution(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		sql        string
		member     string
		wantSource spec.TTLClauseSource
	}{
		{name: "create native", sql: "CREATE TABLE t (created_at DATETIME) TTL = created_at + INTERVAL 30 DAY", member: "ttl", wantSource: spec.TTLClauseSourceNative},
		{name: "create feature", sql: "CREATE TABLE t (created_at DATETIME) /*T![ttl] TTL=created_at+INTERVAL 3 DAY */", member: "ttl", wantSource: spec.TTLClauseSourceFeatureComment},
		{name: "create feature ttl-named table", sql: "CREATE TABLE ttl (created_at DATETIME) /*T![ttl] TTL=created_at+INTERVAL 3 DAY */", member: "ttl", wantSource: spec.TTLClauseSourceFeatureComment},
		{name: "create feature ttl-named column", sql: "CREATE TABLE t (ttl DATETIME) /*T![ttl] TTL=created_at+INTERVAL 3 DAY */", member: "ttl", wantSource: spec.TTLClauseSourceFeatureComment},
		{name: "alter native", sql: "ALTER TABLE t TTL = created_at + INTERVAL 30 DAY", member: "ttl", wantSource: spec.TTLClauseSourceNative},
		{name: "alter feature", sql: "ALTER TABLE t /*T![ttl] TTL=created_at+INTERVAL 3 DAY */", member: "ttl", wantSource: spec.TTLClauseSourceFeatureComment},
		{name: "alter feature ttl-named table", sql: "ALTER TABLE ttl /*T![ttl] TTL=created_at+INTERVAL 3 DAY */", member: "ttl", wantSource: spec.TTLClauseSourceFeatureComment},
		{name: "native enable", sql: "ALTER TABLE t TTL_ENABLE='OFF'", member: "ttl_enable", wantSource: spec.TTLClauseSourceNative},
		{name: "feature enable ttl_enable-named table", sql: "ALTER TABLE ttl_enable /*T![ttl] TTL_ENABLE='OFF' */", member: "ttl_enable", wantSource: spec.TTLClauseSourceFeatureComment},
		{name: "native job interval", sql: "ALTER TABLE t TTL_JOB_INTERVAL='12h'", member: "ttl_job_interval", wantSource: spec.TTLClauseSourceNative},
		{name: "feature job interval named table", sql: "ALTER TABLE ttl_job_interval /*T![ttl] TTL_JOB_INTERVAL='12h' */", member: "ttl_job_interval", wantSource: spec.TTLClauseSourceFeatureComment},
		{name: "native remove", sql: "ALTER TABLE t REMOVE TTL", member: "remove_ttl", wantSource: spec.TTLClauseSourceNative},
		{name: "feature remove qualified table", sql: "ALTER TABLE remove.ttl /*T![ttl] REMOVE TTL */", member: "remove_ttl", wantSource: spec.TTLClauseSourceFeatureComment},
		{name: "marker inside string is not feature", sql: "ALTER TABLE t TTL=created_at+INTERVAL 30 DAY COMMENT='/*T![ttl] TTL=created_at+INTERVAL 1 DAY */'", member: "ttl", wantSource: spec.TTLClauseSourceNative},
		{name: "marker inside ordinary comment is not feature", sql: "ALTER TABLE t TTL=created_at+INTERVAL 30 DAY /* note /*T![ttl] TTL=created_at+INTERVAL 1 DAY */", member: "ttl", wantSource: spec.TTLClauseSourceNative},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			statement := extractOne(t, spec.DialectTiDB, tc.sql)
			var declarations []spec.TTLDeclaration
			switch {
			case statement.DDL == nil:
				t.Fatalf("statement = %#v, want DDL", statement)
			case len(statement.DDL.TTL) > 0:
				declarations = statement.DDL.TTL
			default:
				for _, alter := range statement.DDL.Alter {
					declarations = append(declarations, alter.TTL...)
				}
			}
			var got *spec.TTLDeclaration
			for i := range declarations {
				if declarations[i].Member == tc.member {
					got = &declarations[i]
					break
				}
			}
			if got == nil || got.Source != tc.wantSource {
				t.Fatalf("%q member %q source = %#v, want %s", tc.sql, tc.member, got, tc.wantSource)
			}
		})
	}
}

func TestT19TTLDeclarationValues(t *testing.T) {
	t.Parallel()
	statement := extractOne(t, spec.DialectTiDB,
		"ALTER TABLE t TTL=created_at+INTERVAL 4 DAY, TTL_ENABLE='ON', TTL_JOB_INTERVAL='5h'")
	if len(statement.DDL.Alter) != 3 {
		t.Fatalf("alter actions = %#v, want three TTL actions", statement.DDL.Alter)
	}
	if got := statement.DDL.Alter[0].TTL; len(got) != 1 || got[0].TimeColumn != "created_at" || got[0].IntervalValue != "4" || got[0].IntervalUnit != "day" || len(got[0].Unmodeled) != 0 {
		t.Fatalf("TTL declaration = %#v", got)
	}
	if got := statement.DDL.Alter[1].TTL; len(got) != 1 || got[0].Enabled == nil || !*got[0].Enabled {
		t.Fatalf("TTL_ENABLE declaration = %#v", got)
	}
	if got := statement.DDL.Alter[2].TTL; len(got) != 1 || got[0].JobInterval != "5h" {
		t.Fatalf("TTL_JOB_INTERVAL declaration = %#v", got)
	}
}
