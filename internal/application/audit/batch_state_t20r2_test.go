package audit

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// MySQL has no /*T![feature]*/ syntax: the server parses it as an ordinary
// comment. Probed on mysql:8.0.46 — create/alter/direct-option comment
// forms all land rc=0 with no binding, while the bare form stays 1064.
// The MySQL dialect entry blanks /*T! comments same-width before the shared
// TiDB parser sees them, so nothing in the comment can produce a binding,
// a vendor refusal, or an evidence gap (issue #99 T20-R2).
func TestT20R2MySQLFeatureCommentIgnored(t *testing.T) {
	mk := func() *t10Provider {
		return &t10Provider{
			version:   "8.0.46",
			placement: map[string]*spec.PlacementPolicyFact{},
			tables:    map[string]*spec.TableSnapshot{},
		}
	}
	audit := func(t *testing.T, sql string, dialect spec.Dialect, provider *t10Provider) report.Result {
		t.Helper()
		result, err := AuditSQL(context.Background(), Request{
			SQL:              sql,
			Dialect:          dialect,
			Schema:           "golden",
			MetadataProvider: provider,
		})
		if err != nil {
			t.Fatalf("audit error for %q: %v", sql, err)
		}
		return result
	}
	placementFields := func(t *testing.T, result report.Result, stmt int) {
		t.Helper()
		for _, f := range result.Statements[stmt].Findings {
			if strings.Contains(f.RuleID, "placement") {
				t.Fatalf("comment must not produce placement findings, got %s", f.RuleID)
			}
		}
		for _, g := range result.Statements[stmt].EvidenceGaps {
			if strings.Contains(g.RuleID, "placement") {
				t.Fatalf("comment must not produce placement gaps, got %s", g.RuleID)
			}
		}
	}

	t.Run("policy-reference comment creates a plain table", func(t *testing.T) {
		result := audit(t,
			"CREATE TABLE t20_comment (id INT) /*T![placement] PLACEMENT POLICY=p20_missing */; SELECT 1;",
			spec.DialectMySQL, mk())
		if len(result.Unsupported) != 0 {
			t.Fatalf("comment must not vendor-refuse, got %#v", result.Unsupported)
		}
		if got := result.Statements[0].RawSQL; !strings.Contains(got, "/*T![placement] PLACEMENT POLICY=p20_missing */") {
			t.Fatalf("original comment text must stay in RawSQL, got %q", got)
		}
		placementFields(t, result, 0)
		// Unrelated checks still evaluate — the table is real, not suppressed.
		var sawTableRule bool
		for _, f := range result.Statements[0].Findings {
			if strings.HasPrefix(f.RuleID, "ddl.table.") {
				sawTableRule = true
			}
		}
		if !sawTableRule {
			t.Fatal("ordinary table rules must still evaluate the created table")
		}
	})

	t.Run("comment never queries the named policy", func(t *testing.T) {
		// A provider with NO placement catalog — if the comment reached the
		// binding check the catalog read would still matter; it must not.
		provider := &t10Provider{
			version:      "8.0.46",
			placementErr: errors.New("catalog must not be read"),
			tables:       map[string]*spec.TableSnapshot{},
		}
		result, err := AuditSQL(context.Background(), Request{
			SQL:              "CREATE TABLE t20_np (id INT) /*T![placement] PLACEMENT POLICY=p20_missing */; SELECT 1;",
			Dialect:          spec.DialectMySQL,
			Schema:           "golden",
			MetadataProvider: provider,
		})
		if err != nil {
			t.Fatalf("comment-blanked create must not touch the policy catalog: %v", err)
		}
		placementFields(t, result, 0)
	})

	t.Run("direct-options comment is an ignored comment too", func(t *testing.T) {
		result := audit(t,
			"CREATE TABLE t20_direct (id INT) /*T![placement] PRIMARY_REGION='r1' */; SELECT 1;",
			spec.DialectMySQL, mk())
		if len(result.Unsupported) != 0 {
			t.Fatalf("comment must not vendor-refuse, got %#v", result.Unsupported)
		}
		placementFields(t, result, 0)
	})

	t.Run("comment-only alter is a native no-op", func(t *testing.T) {
		// mysql:8.0.46 accepts `ALTER TABLE t` rc=0 — the comment strips to
		// an empty alter spec list, which is a legal no-op.
		provider := mk()
		provider.tables["t"] = t20PlacementShape("t")
		result := audit(t,
			"ALTER TABLE t /*T![placement] PLACEMENT POLICY=p20_missing */; SELECT 1;",
			spec.DialectMySQL, provider)
		if len(result.Unsupported) != 0 {
			t.Fatalf("comment-only alter must not vendor-refuse, got %#v", result.Unsupported)
		}
		placementFields(t, result, 0)
	})

	t.Run("bare form keeps the vendor boundary", func(t *testing.T) {
		provider := mk()
		provider.tables["t"] = t20PlacementShape("t")
		_, err := AuditSQL(context.Background(), Request{
			SQL:              "ALTER TABLE t PLACEMENT POLICY=p;",
			Dialect:          spec.DialectMySQL,
			Schema:           "golden",
			MetadataProvider: provider,
		})
		if err == nil {
			t.Fatal("bare PLACEMENT POLICY under MySQL must stay a vendor refusal (1064)")
		}
	})

	t.Run("TiDB dialect still binds through the comment", func(t *testing.T) {
		provider := &t10Provider{
			version:   "8.0.11-TiDB-v8.5.0",
			placement: t20Catalog("p"),
			tables:    map[string]*spec.TableSnapshot{},
		}
		state, _ := t17Enrich(t, spec.DialectTiDB,
			"CREATE TABLE tc (id INT) /*T![placement] PLACEMENT POLICY=p */; SELECT 1;",
			"golden", t20v805, provider)
		entry := state.entries[state.keyFor("golden", spec.Table{Name: "tc"})]
		if entry == nil || entry.shape == nil || entry.shape.PlacementPolicy != "p" {
			t.Fatalf("TiDB comment bind must still land, got %#v", entry)
		}
	})

	t.Run("lookalike text in strings is content not a comment", func(t *testing.T) {
		// A column COMMENT string containing /*T! text is a literal — the
		// blanking lexer must not touch quoted content.
		provider := mk()
		state, _ := t17Enrich(t, spec.DialectMySQL,
			"CREATE TABLE ts (id INT COMMENT '/*T![placement] */'); SELECT 1;",
			"golden", t17v8046, provider)
		entry := state.entries[state.keyFor("golden", spec.Table{Name: "ts"})]
		if entry == nil || entry.state != tablePresent {
			t.Fatalf("string-quoted /*T! text must not affect the create, got %#v", entry)
		}
	})

	t.Run("executable and hint comments unchanged", func(t *testing.T) {
		result := audit(t,
			"CREATE TABLE te (id INT) /*!80000 ENGINE=InnoDB */; SELECT /*+ MAX_EXECUTION_TIME(1000) */ 1;",
			spec.DialectMySQL, mk())
		if len(result.Statements) != 2 {
			t.Fatalf("/*! and /*+ comments must not disturb statement splitting, got %d", len(result.Statements))
		}
	})

	t.Run("multi-line comment keeps positions", func(t *testing.T) {
		parsed, err := Parse(context.Background(),
			"CREATE TABLE tm (id INT)\n/*T![placement]\n PLACEMENT POLICY=p\n*/\n; SELECT 1;",
			spec.DialectMySQL)
		if err != nil {
			t.Fatalf("multi-line comment must parse: %v", err)
		}
		if len(parsed.Statements) != 2 || parsed.Statements[1].Line != 5 {
			t.Fatalf("blanked comment must preserve line positions, got %#v", parsed.Statements)
		}
		if !strings.Contains(parsed.Statements[0].RawSQL, "/*T![placement]") {
			t.Fatalf("original comment must stay in RawSQL, got %q", parsed.Statements[0].RawSQL)
		}
	})

	t.Run("invalid remainder keeps the real syntax error", func(t *testing.T) {
		_, err := Parse(context.Background(),
			"THIS IS /*T![placement] anything */ NOT VALID SQL;", spec.DialectMySQL)
		if err == nil {
			t.Fatal("blanked-invalid SQL must keep its real parse error")
		}
	})
}

// The placement catalog fact carries region lists and bound-target
// identities; public surfaces may only echo authored policy names and
// bounded target kinds. Marshal a real audit result and assert the
// sentinel catalog values never serialize (T20-R2/#99).
func TestT20R2PlacementPrivacySentinel(t *testing.T) {
	provider := &t10Provider{
		version: "8.0.11-TiDB-v8.5.0",
		placement: map[string]*spec.PlacementPolicyFact{
			"p": {
				PrimaryRegion: "sentinel-region-a",
				Regions:       "sentinel-region-a,sentinel-region-b",
				BoundTargets:  []string{"table:golden.t"},
			},
		},
		tables: map[string]*spec.TableSnapshot{"t": t20PlacementShape("t")},
	}
	result, err := AuditSQL(context.Background(), Request{
		SQL:              "ALTER TABLE t PLACEMENT POLICY=p; ALTER TABLE t PLACEMENT POLICY=missing;",
		Dialect:          spec.DialectTiDB,
		Schema:           "golden",
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, sentinel := range []string{"sentinel-region-a", "sentinel-region-b", "primary_region", "bound_targets"} {
		if strings.Contains(string(raw), sentinel) {
			t.Fatalf("catalog sentinel %q must never serialize: %s", sentinel, raw)
		}
	}
	// The authored policy name itself is allowed — it is user input echoed.
	if !strings.Contains(string(raw), "placement") {
		t.Fatalf("expected placement rule output for the proven bind, got %s", raw)
	}
}

// ALTER DATABASE executes charset/collate BEFORE the placement binding —
// probed on v8.5.0 (`AlterSchema`: ModifySchemaCharsetAndCollate then
// ModifySchemaDefaultPlacement): a proven-absent policy fails 8239 but the
// declared charset still lands, and a prior binding survives the failed
// rebind. SQL option order does not change the executor order. CREATE is
// atomic (8239 → no schema row). The comma form is a grammar 1064.
func TestT20R2AlterDatabasePartialApply(t *testing.T) {
	mk := func(schemaAttrs map[string]string, placement map[string]*spec.PlacementPolicyFact) *t20SchemaAwareProvider {
		if schemaAttrs == nil {
			schemaAttrs = map[string]string{}
		}
		return &t20SchemaAwareProvider{
			t10Provider: &t10Provider{
				version:   "8.0.11-TiDB-v8.5.0",
				placement: placement,
				tables:    map[string]*spec.TableSnapshot{},
			},
			schemas: map[string]*spec.ObjectSnapshot{
				"golden": {Status: spec.MetadataStatusConfirmed, Exists: true, Attributes: schemaAttrs},
			},
		}
	}
	schemaEntry := func(t *testing.T, provider *t20SchemaAwareProvider, sql string) *batchSchemaEntry {
		t.Helper()
		state, _ := t17Enrich(t, spec.DialectTiDB, sql, "golden", t20v805, provider)
		entry := state.schemaEntries["golden"]
		if entry == nil {
			t.Fatalf("schema entry missing for %q", sql)
		}
		return entry
	}

	t.Run("charset lands before missing-policy 8239", func(t *testing.T) {
		// Both SQL orders: the executor order (charset first) decides.
		for _, sql := range []string{
			"ALTER DATABASE golden CHARACTER SET latin1 PLACEMENT POLICY=missing; SELECT 1;",
			"ALTER DATABASE golden PLACEMENT POLICY=missing CHARACTER SET latin1; SELECT 1;",
		} {
			entry := schemaEntry(t, mk(
				map[string]string{"charset": "utf8mb4", "collation": "utf8mb4_bin"},
				t20Catalog("existing")), sql)
			if entry.attrs["charset"] != "latin1" || entry.attrs["collation"] != "latin1_bin" {
				t.Fatalf("%q → charset must land before the 8239, got %#v", sql, entry.attrs)
			}
			if _, bound := entry.attrs["placement_policy"]; bound {
				t.Fatalf("%q → failed bind must not publish a binding, got %#v", sql, entry.attrs)
			}
		}
	})

	t.Run("failed rebind keeps the prior binding", func(t *testing.T) {
		entry := schemaEntry(t, mk(
			map[string]string{"charset": "utf8mb4", "collation": "utf8mb4_bin", "placement_policy": "old"},
			map[string]*spec.PlacementPolicyFact{"old": {BoundTargets: []string{"schema:golden"}}}),
			"ALTER DATABASE golden CHARACTER SET latin1 PLACEMENT POLICY=missing; SELECT 1;")
		if entry.attrs["placement_policy"] != "old" {
			t.Fatalf("failed rebind must keep the prior binding, got %#v", entry.attrs)
		}
		if entry.attrs["charset"] != "latin1" {
			t.Fatalf("charset must still land, got %#v", entry.attrs)
		}
	})

	t.Run("unvouched policy invalidates only the binding dimension", func(t *testing.T) {
		// No placement catalog → policy existence unvouched: charset still
		// provably applies; only the binding attr drops from proven state.
		entry := schemaEntry(t, mk(
			map[string]string{"charset": "utf8mb4", "collation": "utf8mb4_bin", "placement_policy": "old"},
			nil),
			"ALTER DATABASE golden CHARACTER SET latin1 PLACEMENT POLICY=maybe; SELECT 1;")
		if entry.attrs["charset"] != "latin1" || entry.attrs["collation"] != "latin1_bin" {
			t.Fatalf("charset must apply regardless of the unknown bind, got %#v", entry.attrs)
		}
		if _, bound := entry.attrs["placement_policy"]; bound {
			t.Fatalf("unproven bind must invalidate the placement attr, got %#v", entry.attrs)
		}
	})

	t.Run("existing policy binds with charset", func(t *testing.T) {
		entry := schemaEntry(t, mk(
			map[string]string{"charset": "utf8mb4", "collation": "utf8mb4_bin"},
			t20Catalog("existing")),
			"ALTER DATABASE golden CHARACTER SET latin1 PLACEMENT POLICY=existing; SELECT 1;")
		if entry.attrs["charset"] != "latin1" || entry.attrs["placement_policy"] != "existing" {
			t.Fatalf("proven bind must publish both dimensions, got %#v", entry.attrs)
		}
	})

	t.Run("create stays atomic on missing policy", func(t *testing.T) {
		state, _ := t17Enrich(t, spec.DialectTiDB,
			"CREATE DATABASE t20_new CHARACTER SET latin1 PLACEMENT POLICY=missing; SELECT 1;",
			"golden", t20v805, &t20SchemaAwareProvider{
				t10Provider: &t10Provider{
					version:   "8.0.11-TiDB-v8.5.0",
					placement: t20Catalog("existing"),
					tables:    map[string]*spec.TableSnapshot{},
				},
				schemas: map[string]*spec.ObjectSnapshot{
					"t20_new": {Status: spec.MetadataStatusConfirmed, Exists: false},
				},
			})
		if entry := state.schemaEntries["t20_new"]; entry != nil && entry.state == schemaPresent {
			t.Fatalf("atomic create must not publish a schema on 8239, got %#v", entry)
		}
	})
}
