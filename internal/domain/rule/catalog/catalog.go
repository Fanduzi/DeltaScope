// Package catalog defines explanation-oriented metadata for shipped audit rules.
package catalog

import (
	"fmt"
	"sort"
	"strings"

	domainpolicy "github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
)

// MetadataKind identifies the metadata category a rule explanation depends on.
type MetadataKind string

const (
	MetadataKindSchema      MetadataKind = "schema"
	MetadataKindTargetTable MetadataKind = "target_table"
	MetadataKindInstance    MetadataKind = "instance"
)

// MetadataNotes describes how metadata affects one catalog entry.
type MetadataNotes struct {
	Kinds    []MetadataKind
	Required string
	Missing  string
}

// Entry describes one shipped rule in the CLI-facing catalog.
type Entry struct {
	RuleID          string
	ConfigKey       string
	Summary         string
	Description     string
	StatementKinds  []string
	DefaultEnabled  bool
	DefaultLevel    rule.Level
	DefaultParams   map[string]any
	MetadataAware   bool
	TriggerExample  string
	ValidExample    string
	ConfigExample   string
	RemediationHint string
	Why             string
	Risk            string
	Suggestion      string
	ConfigHints     []string
	MetadataNotes   *MetadataNotes
	Dialects        []string
	Category        string
	Tags            []string
	Source          string
	SearchText      string
}

var entries = buildEntries()

// All returns all shipped catalog entries in stable order.
func All() []Entry {
	out := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		out = append(out, cloneEntry(entry))
	}
	return out
}

// Lookup returns the catalog entry for one rule ID.
func Lookup(ruleID string) (Entry, bool) {
	for _, entry := range entries {
		if entry.RuleID == ruleID {
			return cloneEntry(entry), true
		}
	}
	return Entry{}, false
}

// Search returns entries whose IDs, summaries, descriptions, or kinds match the query.
func Search(query string) []Entry {
	needle := strings.ToLower(strings.TrimSpace(query))
	if needle == "" {
		return All()
	}

	results := make([]Entry, 0)
	for _, entry := range entries {
		if strings.Contains(strings.ToLower(entry.SearchText), needle) {
			results = append(results, cloneEntry(entry))
		}
	}
	return results
}

func buildEntries() []Entry {
	rules := shippedCatalogPolicies()
	ids := make([]string, 0, len(rules))
	for ruleID := range rules {
		ids = append(ids, ruleID)
	}
	sort.Strings(ids)

	items := make([]Entry, 0, len(ids))
	for _, ruleID := range ids {
		policy := rules[ruleID]
		entry := Entry{
			RuleID:          ruleID,
			ConfigKey:       ruleID,
			Summary:         summaryForRule(ruleID),
			Description:     descriptionForRule(ruleID, policy),
			StatementKinds:  statementKindsForRule(ruleID),
			DefaultEnabled:  policy.Enabled,
			DefaultLevel:    policy.Level,
			DefaultParams:   cloneParams(policy.Params),
			MetadataAware:   metadataAwareRuleIDs[ruleID],
			TriggerExample:  triggerExampleForRule(ruleID),
			ValidExample:    validExampleForRule(ruleID),
			ConfigExample:   configExampleForRule(ruleID, policy),
			RemediationHint: remediationForRule(ruleID),
			Why:             whyForRule(ruleID),
			Risk:            riskForRule(ruleID),
			Suggestion:      suggestionForRule(ruleID),
			ConfigHints:     configHintsForRule(ruleID, policy),
			MetadataNotes:   metadataNotesForRule(ruleID),
			Dialects:        dialectsForRule(ruleID),
			Category:        categoryForRule(ruleID),
			Tags:            tagsForRule(ruleID),
			Source:          sourceForRule(),
		}
		entry.SearchText = strings.Join([]string{
			entry.RuleID,
			entry.Summary,
			entry.Description,
			strings.Join(entry.StatementKinds, " "),
		}, " ")
		items = append(items, entry)
	}
	return items
}

// shippedCatalogPolicies is Default Policy plus shipped rules that discovery
// must describe even when Default Policy does not enable them.
func shippedCatalogPolicies() map[string]domainpolicy.RulePolicy {
	defaults := domainpolicy.Default()
	rules := make(map[string]domainpolicy.RulePolicy, len(defaults.Rules)+len(optInCatalogRules))
	for ruleID, policy := range defaults.Rules {
		rules[ruleID] = policy
	}
	for ruleID, policy := range optInCatalogRules {
		if _, exists := rules[ruleID]; exists {
			continue
		}
		rules[ruleID] = policy
	}
	return rules
}

var optInCatalogRules = map[string]domainpolicy.RulePolicy{
	"dml.impact.estimate": {
		Enabled: false,
		Level:   rule.LevelNotice,
	},
	"dml.impact.rows.max_count": {
		Enabled: false,
		Level:   rule.LevelWarning,
		Params: map[string]any{
			"value": 1000,
		},
	},
	"dml.impact.ratio.max_percent": {
		Enabled: false,
		Level:   rule.LevelWarning,
		Params: map[string]any{
			"value": 10,
		},
	},
}

func cloneEntry(in Entry) Entry {
	out := in
	out.StatementKinds = append([]string(nil), in.StatementKinds...)
	out.DefaultParams = cloneParams(in.DefaultParams)
	out.ConfigHints = append([]string(nil), in.ConfigHints...)
	out.Dialects = append([]string(nil), in.Dialects...)
	out.Tags = append([]string(nil), in.Tags...)
	if in.MetadataNotes != nil {
		out.MetadataNotes = &MetadataNotes{
			Kinds:    append([]MetadataKind(nil), in.MetadataNotes.Kinds...),
			Required: in.MetadataNotes.Required,
			Missing:  in.MetadataNotes.Missing,
		}
	}
	return out
}

func cloneParams(in map[string]any) map[string]any {
	if len(in) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = cloneParamValue(value)
	}
	return out
}

func cloneParamValue(value any) any {
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...)
	default:
		return value
	}
}

func summaryForRule(ruleID string) string {
	switch {
	case strings.HasSuffix(ruleID, ".require"):
		return humanize(ruleID, "Require")
	case strings.HasSuffix(ruleID, ".forbid"):
		return humanize(ruleID, "Forbid")
	case strings.Contains(ruleID, ".allowlist"):
		return humanize(ruleID, "Restrict")
	case strings.Contains(ruleID, ".max_length"):
		return humanize(ruleID, "Limit")
	case strings.Contains(ruleID, ".max_count"):
		return humanize(ruleID, "Limit")
	case strings.Contains(ruleID, ".max_bytes"):
		return humanize(ruleID, "Limit")
	case strings.HasSuffix(ruleID, ".warn"):
		return humanize(ruleID, "Warn on")
	default:
		return humanize(ruleID, "Check")
	}
}

func descriptionForRule(ruleID string, policy domainpolicy.RulePolicy) string {
	if ruleID == "ddl.routine.target.valid" {
		return "Checks MySQL stored routine guard availability and observed qualified routine identity."
	}
	switch ruleID {
	case "ddl.pg.alter_foreign_table.target.valid":
		return "Checks qualified PostgreSQL 17 foreign table targets, column guards, options, owner and namespace changes."
	case "ddl.pg.alter_foreign_data_wrapper.target.valid":
		return "Checks exact PostgreSQL 17 wrapper targets, routines, owner and ordered option premises."
	case "ddl.pg.sequence.declaration.valid":
		return "Checks qualified PostgreSQL 17 sequence declarations, guards, stable bounds, ownership and namespace changes."
	case "ddl.pg.alter_policy.target.valid":
		return "Checks exact PostgreSQL 17 policy targets, roles, rename occupancy and bounded USING or WITH CHECK clauses."
	case "ddl.pg.alter_type.enum.valid":
		return "Checks PostgreSQL 17 enum ADD VALUE and RENAME VALUE using exact type and label catalog facts."
	case "ddl.pg.alter_operator.change.target.valid":
		return "Checks qualified PostgreSQL 17 operator signatures, OWNER TO and SET SCHEMA destinations, and qualified RESTRICT and JOIN estimators."
	case "ddl.pg.alter_type.change.target.valid":
		return "Checks exact PostgreSQL 17 type identity, rename and schema destinations, and owner resolution."
	case "ddl.pg.type.composite.valid":
		return "Checks qualified PostgreSQL 17 composite type creation and exact attribute actions."
	case "ddl.pg.type.base.valid":
		return "Checks PostgreSQL 17 shell and base type targets, required I/O declarations and bounded property changes."
	case "ddl.pg.create_type.range.valid":
		return "Checks qualified PostgreSQL 17 range creation destinations, exact subtypes and bounded declarations."
	case "ddl.pg.create_type.enum.valid":
		return "Checks a PostgreSQL 17 enum creation target, repeated labels and the UTF8 label size limit."
	case "ddl.pg.text_search.change.target.valid":
		return "Checks qualified PostgreSQL 17 text search configuration and dictionary targets, mappings, owners and namespace changes."
	case "ddl.pg.alter_publication.target.valid":
		return "Checks local PostgreSQL 17 publication membership, named targets and supported options."
	case "ddl.pg.alter_subscription.target.valid":
		return "Checks PostgreSQL 17 local subscription identity, options, publication names and slot prerequisites."
	case "ddl.pg.create_language.handler.valid":
		return "Checks PostgreSQL 17 handler-form language declarations against exact local callback signatures."
	case "ddl.pg.grant.declaration.valid":
		return "Checks exact PostgreSQL 17 privilege subjects, local objects and columns without resolving effective privileges."
	case "ddl.pg.create_schema.target.valid":
		return "Checks PostgreSQL 17 schema creation identity, authored owner and guarded destination premises."
	case "ddl.pg.alter_collation.refresh.target.valid":
		return "Checks exact PostgreSQL 17 collation refresh target identity in the database encoding."
	case "ddl.pg.alter_collation.change.target.valid":
		return "Checks qualified PostgreSQL 17 collation rename, namespace and owner targets."
	case "ddl.pg.create_index.include.valid":
		return "Checks exact PostgreSQL 17 CREATE INDEX INCLUDE targets, bounded access-method eligibility and user-column names."
	case "ddl.pg.alter_index.destination.valid":
		return "Checks qualified PostgreSQL 17 ALTER INDEX rename and tablespace source and destination premises."
	case "ddl.pg.alter_index.options.valid":
		return "Checks exact PostgreSQL 17 index SET/RESET targets and bounded B-tree and hash declarations."
	case "ddl.pg.alter_index.attach_partition.valid":
		return "Checks exact PostgreSQL 17 index partition attachment identities and catalog relationships."
	case "ddl.pg.create_operator_family.target.valid":
		return "Checks the exact PostgreSQL 17 operator family creation namespace, index method and destination identity."
	case "ddl.pg.alter_operator_family.change.target.valid":
		return "Checks PostgreSQL 17 qualified operator family rename, schema and owner premises."
	case "ddl.pg.alter_operator_family.member.valid":
		return "Checks exact PostgreSQL 17 B-tree search operator family member slots."
	case "ddl.pg.create_table.columns.valid":
		return "Checks direct PostgreSQL 17 CREATE TABLE column declarations and exact qualified type existence and eligibility."
	case "ddl.pg.create_table.inherits.valid":
		return "Checks PostgreSQL 17 CREATE TABLE INHERITS exact parent identities, order and relation kind."
	case "ddl.pg.create_table.foreign_key.valid":
		return "Checks direct PostgreSQL 17 CREATE TABLE foreign key targets, member names and candidate referenced keys."
	case "ddl.pg.create_table.expression.valid":
		return "Checks PostgreSQL 17 CREATE TABLE defaults, CHECK predicates and generated expressions for bounded AST refusals."
	case "ddl.pg.create_table.keys.valid":
		return "Checks direct PostgreSQL 17 CREATE TABLE primary and unique key declarations with exact destination evidence."
	case "ddl.pg.create_table.storage.valid":
		return "Checks qualified PostgreSQL 17 CREATE TABLE access methods, tablespaces and bounded storage declarations."
	case "ddl.pg.create_table.of.valid":
		return "Checks the exact standalone composite source type and column option names for PostgreSQL 17 CREATE TABLE OF."
	case "ddl.pg.create_table.on_commit.valid":
		return "Checks qualified PostgreSQL 17 CREATE TABLE persistence and ON COMMIT premises."
	case "ddl.pg.create_table.like.target.valid":
		return "Checks PostgreSQL 17 qualified CREATE TABLE LIKE sources, destination guard and copied column names."
	case "ddl.pg.create_view.output.valid":
		return "Checks exact PostgreSQL 17 CREATE VIEW targets and literal output compatibility."
	case "ddl.pg.alter_view.column_default.valid":
		return "Checks exact PostgreSQL 17 ordinary-view column default targets and prohibited references."
	case "ddl.pg.alter_materialized_view.extension.valid":
		return "Checks the exact PostgreSQL 17 materialized view and installed extension for DEPENDS ON EXTENSION."
	case "ddl.pg.alter_materialized_view.options.valid":
		return "Checks exact PostgreSQL 17 materialized-view SET and RESET targets and bounded heap and TOAST option declarations."
	case "ddl.pg.alter_view.options.valid":
		return "Checks exact PostgreSQL 17 ALTER VIEW SET and RESET targets, guards and option declarations."
	case "ddl.pg.alter_view.owner.valid":
		return "Checks exact PostgreSQL 17 view OWNER TO targets, guards and role existence."
	case "ddl.pg.alter_view.namespace.valid":
		return "Checks PostgreSQL 17 view and materialized-view rename and schema-move target, guard and destination catalog facts."
	case "ddl.pg.drop_view.target.valid":
		return "Checks exact PostgreSQL 17 DROP VIEW targets, guarded absence and direct dependent views under RESTRICT."
	case "ddl.table.drop.exists.require":
		return "Checks DROP TABLE target existence. PostgreSQL 17 checks each exact member's relation kind, honors IF EXISTS for observed absence and checks direct dependencies for an eligible RESTRICT set."
	case "ddl.table.truncate.exists.require":
		return "Checks TRUNCATE target existence. PostgreSQL 17 checks exact members, relation kind and whether an earlier target already selected a partitioned ONLY member. Foreign-table eligibility needs FDW evidence."
	case "ddl.table.truncate.inbound_fk":
		return "Checks whether a foreign key references a selected TRUNCATE table from outside the target set. PostgreSQL 17 RESTRICT accounts for attached descendants and inherited partition foreign-key references."
	case "ddl.pg.create_view.options.valid":
		return "Checks PostgreSQL 17 CREATE VIEW option names, duplicates and the declared security and CHECK OPTION values."
	case "ddl.pg.alter.move_all_tablespace.target.valid":
		return "Checks observed PostgreSQL 17 bulk move tablespaces and ordered owner-filter role existence. Missing endpoints, pg_global, missing literal roles and PUBLIC block."
	case "ddl.pg.alter.owner.target.valid":
		return "Checks an exact PostgreSQL 17 ordinary-table owner target. An observed missing named role or PUBLIC blocks on ordinary and qualified persistent partitioned tables. Session aliases use the observed session only for ordinary tables; permissions are not checked."
	case "ddl.pg.alter.rename_table.target.valid":
		return "Checks exact PG17 ordinary-table rename destinations and blocks relation and row-type collisions on qualified persistent partition roots. PostgreSQL can displace an observed autogenerated array type."
	case "ddl.pg.refresh_materialized_view.target.valid":
		return "Checks the exact PostgreSQL 17 materialized view, population, option compatibility and usable unique index for a concurrent refresh."
	case "ddl.pg.alter.set_schema.target.valid":
		return "Checks the PostgreSQL 17 destination schema and exact collisions for an ordinary table, its row and array types, attached indexes and column-owned sequences."
	case "ddl.pg.alter.add_foreign_key.targets.valid":
		return "Checks PostgreSQL 17 ADD FOREIGN KEY relation and column identities, referenced key eligibility, column counts, bounded equality for scalar, domain, enum and array types and ON DELETE SET column lists on fresh ordinary tables, including NOT VALID declarations. Rejects single-action ADD FOREIGN KEY NOT VALID on a confirmed qualified partitioned source."
	case "ddl.pg.alter.constraint_using_index.valid":
		return "Checks catalog preconditions for using an exact existing index in a PostgreSQL 17 PRIMARY KEY or UNIQUE constraint. Checks index ownership, shape, key sort, collation, options and default operator classes resolved from exact and binary-compatible types."
	case "ddl.pg.alter.constraint_columns.valid":
		return "Checks exact PostgreSQL 17 PRIMARY KEY and UNIQUE key and INCLUDE columns on ordinary tables."
	case "ddl.pg.alter.add_check.name.available":
		return "Checks PostgreSQL 17 ADD CHECK exact names, unsupported subqueries, bounded literal coercion and simple column null tests on a fresh ordinary table. Also checks bounded unnamed TRUE CHECK traversal refusals on qualified persistent partition roots."
	case "ddl.pg.alter.column_type.destination.valid":
		return "Checks PostgreSQL 17 TYPE and TYPE USING destinations on a fresh exact ordinary table. Checks type identity, supported built-in modifiers, qualified collations and direct view or generated-column dependencies. TYPE without USING checks scalar assignment and whether an ONLY target requires child traversal."
	case "ddl.pg.alter.column_type.source.valid":
		return "Checks exact PostgreSQL 17 TYPE or TYPE USING sources. Missing ordinary user columns, system columns, generated columns with USING, inherited source columns and direct partition keys can block."
	case "ddl.pg.alter.persistence.transition.valid":
		return "Checks PG17 ordinary-table persistence transitions and refuses SET UNLOGGED on qualified persistent partition roots with direct publication membership or inbound foreign keys from other persistent tables. A proven ordinary same-persistence request leaves the files in place."
	case "ddl.pg.alter.access_method.target.valid":
		return "Checks the access method destination for a fresh, exactly bound PostgreSQL 17 ordinary table. An absent named method or an index method also blocks on a qualified persistent partitioned table. Bare DEFAULT uses the effective setting for ordinary tables only."
	case "ddl.pg.alter.tablespace.target.valid":
		return "Checks the named SET TABLESPACE destination for an observed, exactly bound PostgreSQL 17 ordinary table. Missing destinations and pg_global also block on a qualified persistent partitioned table. Other partitioned-table destinations remain unverified."
	case "ddl.pg.alter.reloptions.valid":
		return "Checks bounded SET and RESET declarations for a fresh exact PostgreSQL 17 ordinary heap table. Known duplicate or invalid values block; unknown scope or options remain gaps."
	case "ddl.pg.alter.replica_identity_using_index.valid":
		return "Checks exact PostgreSQL 17 ordinary-table replica-identity index attachment and refuses missing, wrong-kind, wrong-owner and nonunique indexes on qualified persistent partition roots. Unknown evidence remains a gap."
	case "ddl.table.exists.alter.require":
		return "Checks ALTER TABLE target existence. On PostgreSQL 17, IF EXISTS guards observed qualified absence. SET WITHOUT OIDS also checks the exact relation kind. Unknown applicable facts remain evidence gaps."
	case "ddl.alter.add_column.exists.forbid":
		return "Checks ADD COLUMN for exact collisions and observed PostgreSQL 17 parent or partition traversal requirements. IF NOT EXISTS skips an existing eligible user column. An observed system-name conflict or direct addition to a partition child blocks."
	case "ddl.alter.rename_column.exists.require":
		return "Requires an existing RENAME COLUMN source. On observed PG17, fresh names are case- and space-exact, and an observed system column is inapplicable."
	case "ddl.alter.rename_column.target.exists.forbid":
		return "Checks RENAME COLUMN for an occupied destination. On observed PG17, the exact same name also collides, and missing or stale column evidence remains a gap."
	case "ddl.pg.alter.column.storage.valid":
		return "Observed PG17 column STORAGE: rejects invalid case-insensitive modes, absent user columns, proved system targets or non-PLAIN strategies on a non-TOAST-aware type. Actual type typstorage supplies DEFAULT; unknown facts remain gaps."
	case "ddl.pg.alter.column.compression.valid":
		return "Observed PG17 column COMPRESSION: rejects invalid case-sensitive methods, absent user columns, proved system targets, nondefault compression on non-TOAST-aware types or lz4 unavailable in the observed build enum. DEFAULT allows fixed-length types; current STORAGE PLAIN is not a type restriction."
	case "ddl.pg.alter.column.statistics.target.valid":
		return "Observed PG17 statistics declaration: targets below -1, proved missing user columns or inapplicable system targets block; DEFAULT/-1 and normal targets are allowed. Above-maximum targets are clamped, not refused. Unknown facts remain gaps."
	case "ddl.pg.alter.column.statistics.clamp.warn":
		return "Warns when an applicable explicit PG17 statistics target exceeds 10000 and the backend lowers it to 10000. DEFAULT and normal values are silent; unknown or proved-invalid targets do not invent a clamp finding."
	case "ddl.pg.alter.column.options.valid":
		return "Checks known builtin PG17 column SET options n_distinct/n_distinct_inherited for range [-1, DBL_MAX] and duplicate keys, plus exact SET/RESET target existence and system inapplicability. Unknown SET keys/opaque values remain gaps; unknown RESET names are not inherently errors."
	case "ddl.pg.alter.column.generation.transition.valid":
		return "PG17 existing-column generation preconditions: exact fresh ordinary-table identity, generated, type, NOT NULL, DEFAULT and primary-key facts. SET DEFAULT checks column references and subqueries. Identity changes reject a qualified persistent leaf partition or ONLY on a persistent partition root. NOT NULL checks partition traversal. DROP NOT NULL also checks live attached replica identity key slots. Proven conflicts block; guards preserve known no-ops. Data, runtime expression values, permissions and ambiguous facts require separate evidence."
	case "ddl.pg.alter.identity.sequence_options.valid":
		return "PG17 existing-column identity sequence options: rejects proven duplicate modes/options, invalid increment/cache/type/bounds/start/restart. ADD uses a new declaration; SET uses actual dependency-bound stable parameters. Missing facts or narrowed bounds needing unread current state remain gaps."
	case "ddl.pg.alter.identity.restart.warn":
		return "PG17 explicit identity RESTART warns about possible value reuse and concurrent issuance blocking. SET START alone is silent; applicability, target and options must be proven, not simulated."
	case "ddl.pg.alter.identity.cycle.warn":
		return "PG17 ADD IDENTITY / SET options explicit CYCLE warns about value reuse; NO CYCLE and omission are silent. CREATE/ADD COLUMN remain owned by the separate declaration rule."
	}
	if ruleID == "ddl.pg.alter.validate_constraint.target.require" {
		return "Checks PostgreSQL 17 FK and CHECK targets for VALIDATE and checks parent-only requests on an unvalidated CHECK with attached children. Missing applicable observations remain evidence gaps. Existing row validity has separate prerequisites."
	}
	if ruleID == "ddl.pg.alter.constraint_deferrability.target.require" {
		return "Requires a proven existing PostgreSQL FK target for ALTER CONSTRAINT deferrability. Missing and wrong-kind targets block. A fresh qualified partition-derived FK also blocks. Quoted names bind exactly and unknown or stale targets remain evidence gaps."
	}
	if ruleID == "ddl.pg.alter.drop_constraint.target.require" {
		return "Checks exact PostgreSQL 17 DROP CONSTRAINT targets, direct foreign-key blockers for RESTRICT, inherited CHECKs and partition foreign keys, and ONLY drops of CHECKs on partitioned parents with children. IF EXISTS guards absence. Unknown applicable facts remain evidence gaps."
	}
	if ruleID == "ddl.pg.alter.add_foreign_key.not_valid.require" {
		return "PostgreSQL ALTER ADD FOREIGN KEY without NOT VALID warns about initial validation and locks. NOT VALID is silent for this rule, not legality proof; PostgreSQL 17 partitioned-table foreign keys cannot use that option. Existing FK forbids and validation follow-up remain independent."
	}
	if ruleID == "ddl.pg.alter.add_primary_key.concurrent_index.warn" {
		return "PostgreSQL ALTER ADD PRIMARY KEY without USING INDEX warns about index-build and lock impact. Explicit USING INDEX is silent, not index-eligibility or lock-free attachment proof."
	}
	if ruleID == "ddl.pg.column.generation.declaration.valid" {
		return "PostgreSQL CREATE TABLE / ADD COLUMN bounded generated/identity declarations: DEFAULT/NULL conflicts, proven scalar integer type and identity sequence bounds. Repeated or incomplete facts stay evidence gaps. No expression legality, existing-column transition or runtime sequence-state proof."
	}
	if ruleID == "ddl.pg.column.identity.cycle.warn" {
		return "PostgreSQL CREATE TABLE / ADD COLUMN explicit identity CYCLE warns about possible value reuse. NO CYCLE and omission are silent; unresolved declarations stay evidence gaps, not last-wins guesses."
	}
	if ruleID == "ddl.routine.security.definer.warn" {
		return "MySQL stored PROCEDURE/FUNCTION security-context review: explicit SQL SECURITY DEFINER on CREATE/ALTER, or the documented DEFINER default on CREATE only. INVOKER and ALTER omission are silent; no account privilege resolution or body analysis."
	}
	base := summaryForRule(ruleID)
	scope := strings.Join(statementKindsForRule(ruleID), ", ")
	metadata := "offline-safe"
	if metadataAwareRuleIDs[ruleID] {
		metadata = "metadata-aware"
	}
	return fmt.Sprintf("%s. Default level is %s, enabled=%t, scope=%s, and the shipped policy treats it as a %s rule.", base, policy.Level, policy.Enabled, scope, metadata)
}

func statementKindsForRule(ruleID string) []string {
	switch {
	case strings.HasPrefix(ruleID, "ddl."):
		return []string{"ddl"}
	case strings.HasPrefix(ruleID, "dml."):
		return []string{"dml"}
	default:
		return []string{"unknown"}
	}
}

func triggerExampleForRule(ruleID string) string {
	switch {
	case ruleID == "ddl.pg.alter.column.storage.valid":
		return "ALTER TABLE t ALTER COLUMN fixed_integer SET STORAGE EXTERNAL;"
	case ruleID == "ddl.pg.alter.column.compression.valid":
		return "ALTER TABLE t ALTER COLUMN fixed_integer SET COMPRESSION pglz;"
	case ruleID == "ddl.pg.alter.column.statistics.target.valid":
		return "ALTER TABLE t ALTER COLUMN c SET STATISTICS -2;"
	case ruleID == "ddl.pg.alter.column.statistics.clamp.warn":
		return "ALTER TABLE t ALTER COLUMN c SET STATISTICS 10001;"
	case ruleID == "ddl.pg.alter.column.options.valid":
		return "ALTER TABLE t ALTER COLUMN c SET (n_distinct=-1.1);"
	case ruleID == "ddl.pg.alter.column.generation.transition.valid":
		return "ALTER TABLE t ALTER COLUMN nullable_id ADD GENERATED ALWAYS AS IDENTITY;"
	case ruleID == "ddl.pg.alter.identity.sequence_options.valid":
		return "ALTER TABLE t ALTER COLUMN id SET INCREMENT 0;"
	case ruleID == "ddl.pg.alter.identity.restart.warn":
		return "ALTER TABLE t ALTER COLUMN id RESTART WITH 5;"
	case ruleID == "ddl.pg.alter.identity.cycle.warn":
		return "ALTER TABLE t ALTER COLUMN id SET CYCLE;"
	case ruleID == "ddl.pg.alter.validate_constraint.target.require":
		return "ALTER TABLE t VALIDATE CONSTRAINT missing;"
	case ruleID == "ddl.pg.alter.constraint_deferrability.target.require":
		return "ALTER TABLE t ALTER CONSTRAINT missing DEFERRABLE;"
	case ruleID == "ddl.pg.alter.drop_constraint.target.require":
		return "ALTER TABLE t DROP CONSTRAINT missing;"
	case ruleID == "ddl.pg.alter.add_foreign_key.not_valid.require":
		return "ALTER TABLE child ADD CONSTRAINT fk FOREIGN KEY (pid) REFERENCES parent(id);"
	case ruleID == "ddl.pg.alter.add_primary_key.concurrent_index.warn":
		return "ALTER TABLE t ADD CONSTRAINT pk PRIMARY KEY (id);"
	case ruleID == "ddl.pg.column.generation.declaration.valid":
		return "CREATE TABLE t (id bigint GENERATED ALWAYS AS IDENTITY (INCREMENT 0));"
	case ruleID == "ddl.pg.column.identity.cycle.warn":
		return "CREATE TABLE t (id bigint GENERATED ALWAYS AS IDENTITY (CYCLE));"
	case ruleID == "ddl.routine.security.definer.warn":
		return "CREATE PROCEDURE p() SQL SECURITY DEFINER SET @x=1;"
	case ruleID == "ddl.view.target.valid":
		return "DROP VIEW missing_view;"
	case ruleID == "ddl.view.source.valid":
		return "CREATE VIEW v AS SELECT missing_column FROM t;"
	case ruleID == "ddl.view.security.definer.warn":
		return "CREATE VIEW v AS SELECT id FROM t;"
	case ruleID == "ddl.view.algorithm.warn", ruleID == "ddl.view.options.ignored.warn":
		return "CREATE ALGORITHM=TEMPTABLE VIEW v AS SELECT id FROM t;"
	case ruleID == "ddl.view.check_option.valid":
		return "CREATE ALGORITHM=TEMPTABLE VIEW v AS SELECT id FROM t WITH LOCAL CHECK OPTION;"
	case ruleID == "dml.where.require":
		return "DELETE FROM users;"
	case strings.HasPrefix(ruleID, "dml."):
		return "UPDATE users SET status = 'disabled';"
	case strings.Contains(ruleID, ".drop."):
		return "ALTER TABLE users DROP COLUMN email;"
	case strings.Contains(ruleID, ".rename."):
		return "ALTER TABLE users RENAME COLUMN nickname TO display_name;"
	default:
		return "CREATE TABLE users (id bigint);"
	}
}

func validExampleForRule(ruleID string) string {
	switch {
	case ruleID == "ddl.pg.alter.column.storage.valid":
		return "ALTER TABLE t ALTER COLUMN fixed_integer SET STORAGE DEFAULT;"
	case ruleID == "ddl.pg.alter.column.compression.valid":
		return "ALTER TABLE t ALTER COLUMN fixed_integer SET COMPRESSION default;"
	case ruleID == "ddl.pg.alter.column.statistics.target.valid", ruleID == "ddl.pg.alter.column.statistics.clamp.warn":
		return "ALTER TABLE t ALTER COLUMN c SET STATISTICS DEFAULT;"
	case ruleID == "ddl.pg.alter.column.options.valid":
		return "ALTER TABLE t ALTER COLUMN c SET (n_distinct=-0.5);"
	case ruleID == "ddl.pg.alter.column.generation.transition.valid":
		return "ALTER TABLE t ALTER COLUMN plain_not_null_integer ADD GENERATED ALWAYS AS IDENTITY;"
	case ruleID == "ddl.pg.alter.identity.sequence_options.valid":
		return "ALTER TABLE t ALTER COLUMN id SET CACHE 5;"
	case ruleID == "ddl.pg.alter.identity.restart.warn":
		return "ALTER TABLE t ALTER COLUMN id SET START WITH 5;"
	case ruleID == "ddl.pg.alter.identity.cycle.warn":
		return "ALTER TABLE t ALTER COLUMN id SET NO CYCLE;"
	case ruleID == "ddl.pg.alter.validate_constraint.target.require":
		return "ALTER TABLE t VALIDATE CONSTRAINT existing_check;"
	case ruleID == "ddl.pg.alter.constraint_deferrability.target.require":
		return "ALTER TABLE t ALTER CONSTRAINT existing_fk DEFERRABLE;"
	case ruleID == "ddl.pg.alter.drop_constraint.target.require":
		return "ALTER TABLE t DROP CONSTRAINT IF EXISTS missing;"
	case ruleID == "ddl.pg.alter.add_foreign_key.not_valid.require":
		return "ALTER TABLE child ADD CONSTRAINT fk FOREIGN KEY (pid) REFERENCES parent(id) NOT VALID;"
	case ruleID == "ddl.pg.alter.add_primary_key.concurrent_index.warn":
		return "ALTER TABLE t ADD CONSTRAINT pk PRIMARY KEY USING INDEX idx;"
	case ruleID == "ddl.pg.column.generation.declaration.valid", ruleID == "ddl.pg.column.identity.cycle.warn":
		return "CREATE TABLE t (id bigint GENERATED ALWAYS AS IDENTITY (NO CYCLE));"
	case ruleID == "ddl.routine.security.definer.warn":
		return "CREATE PROCEDURE p() SQL SECURITY INVOKER SET @x=1;"
	case ruleID == "ddl.view.target.valid":
		return "DROP VIEW IF EXISTS v;"
	case ruleID == "ddl.view.security.definer.warn":
		return "CREATE SQL SECURITY INVOKER VIEW v AS SELECT id FROM t;"
	case ruleID == "ddl.view.source.valid", ruleID == "ddl.view.algorithm.warn",
		ruleID == "ddl.view.check_option.valid", ruleID == "ddl.view.options.ignored.warn":
		return "CREATE VIEW v AS SELECT id FROM t;"
	case ruleID == "dml.where.require":
		return "DELETE FROM users WHERE id = 1;"
	case strings.HasPrefix(ruleID, "dml."):
		return "UPDATE users SET status = 'disabled' WHERE id = 1;"
	case strings.Contains(ruleID, "comment.require"):
		return "CREATE TABLE users (id bigint PRIMARY KEY) COMMENT='application users';"
	case strings.Contains(ruleID, "primary_key.require"):
		return "CREATE TABLE users (id bigint unsigned NOT NULL AUTO_INCREMENT, PRIMARY KEY (id));"
	case strings.Contains(ruleID, ".drop."):
		return "ALTER TABLE users MODIFY COLUMN email varchar(255) NOT NULL;"
	default:
		return "CREATE TABLE users (id bigint PRIMARY KEY, name varchar(64) NOT NULL);"
	}
}

func configExampleForRule(ruleID string, policy domainpolicy.RulePolicy) string {
	lines := []string{
		"rules:",
		fmt.Sprintf("  %s:", ruleID),
		fmt.Sprintf("    enabled: %t", policy.Enabled),
		fmt.Sprintf("    level: %s", policy.Level),
	}
	if len(policy.Params) == 0 {
		return strings.Join(lines, "\n")
	}
	lines = append(lines, "    params:")
	keys := make([]string, 0, len(policy.Params))
	for key := range policy.Params {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		lines = append(lines, fmt.Sprintf("      %s: %s", key, FormatYAMLScalar(policy.Params[key])))
	}
	return strings.Join(lines, "\n")
}

func remediationForRule(ruleID string) string {
	switch {
	case ruleID == "ddl.pg.alter.column.storage.valid", ruleID == "ddl.pg.alter.column.compression.valid":
		return "Verify fresh exact ordinary-table PG17 user/system membership, actual type typstorage and observed lz4 enum capability. Preserve storage/method case ownership, allow compression default on fixed-length types and do not mistake current STORAGE PLAIN for type ineligibility. Unknown/stale facts stay gaps; these declarations do not immediately rewrite or recompress existing data."
	case strings.HasPrefix(ruleID, "ddl.pg.alter.column.statistics."), ruleID == "ddl.pg.alter.column.options.valid":
		return "Resolve exact fresh observed-PG17 column facts; correct known numeric ranges and duplicate SET keys. Statistics above 10000 are clamped, not refused. Preserve unknown/extension/opaque cases as gaps and distinguish RESET removal from SET validation."
	case strings.HasPrefix(ruleID, "ddl.pg.alter.identity."), ruleID == "ddl.pg.alter.column.generation.transition.valid":
		return "Resolve exact fresh PG17 ordinary-table and stable sequence facts; correct incompatible targets/options. Review value reuse and blocking for RESTART/CYCLE; START alone is not RESTART. Do not infer current issuance, data, expressions, permissions or unapplied post-state."
	case ruleID == "ddl.pg.alter.validate_constraint.target.require", ruleID == "ddl.pg.alter.constraint_deferrability.target.require", ruleID == "ddl.pg.alter.drop_constraint.target.require":
		return "Verify the exact target and its action-compatible kind with fresh complete catalog facts; IF EXISTS guards only DROP absence. Resolve unknown/stale facts separately; do not infer data, dependency, permission or execution legality."
	case ruleID == "ddl.pg.alter.add_foreign_key.not_valid.require":
		return "Review validation and locks on both tables; consider NOT VALID then VALIDATE where supported, not for PostgreSQL 17 partitioned-table foreign keys."
	case ruleID == "ddl.pg.alter.add_primary_key.concurrent_index.warn":
		return "Review index-build risk; consider an eligible prebuilt unique index where supported. USING INDEX still requires eligibility and attachment/NOT NULL review."
	case ruleID == "ddl.pg.column.generation.declaration.valid":
		return "Correct generation/default/nullability conflicts and use valid integer sequence bounds; resolve unproven declaration facts."
	case ruleID == "ddl.pg.column.identity.cycle.warn":
		return "Review potential value reuse and uniqueness requirements; use NO CYCLE when wraparound is not intended."
	case ruleID == "ddl.routine.security.definer.warn":
		return "Review whether definer-context execution is intended; use SQL SECURITY INVOKER when caller-context execution is appropriate."
	case strings.HasSuffix(ruleID, ".require"):
		return "Add the required clause, option, or object explicitly so the rule no longer has to infer intent."
	case strings.HasSuffix(ruleID, ".forbid"):
		return "Rewrite the statement to avoid the forbidden construct, or relax the policy if the team accepts the risk."
	case strings.Contains(ruleID, ".allowlist"):
		return "Use one of the shipped allowlisted values or update the policy allowlist deliberately."
	case strings.Contains(ruleID, ".max_"):
		return "Reduce the size/count below the configured limit or raise the threshold explicitly in policy."
	case strings.HasSuffix(ruleID, ".warn"):
		return "Review the metadata-backed caution and proceed only if the operational trade-off is acceptable."
	default:
		return "Review the default params and align the statement or policy with the intended governance rule."
	}
}

func whyForRule(ruleID string) string {
	switch {
	case strings.HasSuffix(ruleID, ".require"):
		return "The statement is missing a clause, option, or object that the shipped policy requires."
	case strings.HasSuffix(ruleID, ".forbid"):
		return "The statement uses a construct that the shipped policy explicitly forbids."
	case strings.Contains(ruleID, ".allowlist"):
		return "The statement uses a value outside the shipped allowlist for this rule."
	case strings.Contains(ruleID, ".max_"):
		return "The statement exceeds a shipped size or count threshold for this rule."
	case strings.HasSuffix(ruleID, ".warn"):
		return "The statement reached a caution rule that highlights an operational trade-off."
	default:
		return "The statement matched the shipped governance rule for this pattern."
	}
}

func riskForRule(ruleID string) string {
	switch {
	case strings.HasPrefix(ruleID, "dml."):
		return "Ignoring this rule can allow high-impact data changes to proceed with less safety review."
	case metadataAwareRuleIDs[ruleID]:
		return "Ignoring this rule can hide live-schema or instance-state risks that only metadata reveals."
	default:
		return "Ignoring this rule can weaken schema-governance guarantees and make changes harder to review safely."
	}
}

func suggestionForRule(ruleID string) string {
	return remediationForRule(ruleID)
}

func configHintsForRule(ruleID string, policy domainpolicy.RulePolicy) []string {
	hints := []string{
		fmt.Sprintf("rules.%s.enabled", ruleID),
		fmt.Sprintf("rules.%s.level", ruleID),
	}
	if len(policy.Params) == 0 {
		return hints
	}
	keys := make([]string, 0, len(policy.Params))
	for key := range policy.Params {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		hints = append(hints, fmt.Sprintf("rules.%s.params.%s", ruleID, key))
	}
	return hints
}

func metadataNotesForRule(ruleID string) *MetadataNotes {
	kinds, ok := metadataKindsForRule(ruleID)
	if !ok {
		return nil
	}
	switch ruleID {
	case "ddl.pg.alter.column.storage.valid", "ddl.pg.alter.column.compression.valid":
		return &MetadataNotes{Kinds: kinds,
			Required: "Observed PG17, exact fresh ordinary persistent non-inheritance/non-partition user/system column directories, actual pg_type.typstorage and complete pg_settings enum evidence for lz4 when needed. Intrinsic method/mode refusals require the observed version, not a fabricated target.",
			Missing:  "Unknown/stale/misbound/ambiguous scope, authored method/strategy, type or build facts retain unproven_column_storage/unproven_column_compression gaps (unverified/review). User-directory absence does not prove system absence; no privilege/data/dependency/execution or post-state certification."}
	case "ddl.pg.alter.column.statistics.target.valid", "ddl.pg.alter.column.statistics.clamp.warn", "ddl.pg.alter.column.options.valid":
		return &MetadataNotes{Kinds: kinds,
			Required: "Observed PG17 and exact fresh ordinary persistent non-inheritance/non-partition column facts. Intrinsic modeled numeric refusals require the observed version but not an invented catalog.",
			Missing:  "Unknown/stale/misbound/ambiguous user/system facts or unmodeled/quoted/nonfinite/extension SET values retain unproven_column_tuning gaps (unverified/review). No data/expression/permission or PG post-state inference."}
	case "ddl.alter.drop_column.exists.require":
		return &MetadataNotes{Kinds: kinds,
			Required: "On PostgreSQL 17, exact fresh ordinary-table facts establish the column and IF EXISTS behavior. A single schema-qualified RESTRICT also uses bound direct view, generated-column and foreign-key dependencies.",
			Missing:  "Unknown or stale identity, guard or applicable dependency facts report unproven_column_target. A bound empty observation satisfies the scoped dependency premise."}
	case "ddl.pg.alter.drop_column.advisory":
		return &MetadataNotes{Kinds: kinds,
			Required: "For PostgreSQL, exact fresh ordinary-table PG17 user/system column/guard facts distinguish absent unguarded targets from proven IF EXISTS no-ops and inapplicable existing system targets; proved no-op/system inapplicability suppress deletion advice. Other dialects retain existing semantics.",
			Missing:  "The PostgreSQL existence check reports unproven_column_target for unknown version/guard/user or system catalog, ambiguity, stale or misbound proof; user-directory absence never proves a system member absent. Conditional advice is not deletion/execution proof; independent forbids stay effective."}
	case "ddl.pg.alter.column.generation.transition.valid", "ddl.pg.alter.identity.sequence_options.valid", "ddl.pg.alter.identity.restart.warn", "ddl.pg.alter.identity.cycle.warn":
		return &MetadataNotes{Kinds: kinds,
			Required: "Observed PG17, exact fresh ordinary persistent non-inheritance/non-partition column facts and actual dependency-bound stable identity sequence parameters. The transition rule also accepts exact traversal evidence for the bounded partition identity refusal. Observed version alone suffices for intrinsic options refusals.",
			Missing:  "Unknown/unmodeled/ambiguous/stale facts retain unproven_column_generation_target or unproven_identity_sequence_options gaps. Nullable SET NOT NULL needs data proof; narrowed bounds without explicit RESTART can need unread current state. No expression/permission analysis or PG post-state simulation."}
	case "ddl.pg.alter.validate_constraint.target.require", "ddl.pg.alter.constraint_deferrability.target.require", "ddl.pg.alter.drop_constraint.target.require":
		return &MetadataNotes{Kinds: kinds,
			Required: "A complete fresh PG constraint directory bound to the exact table/schema and an observed PostgreSQL 17 server banner establish the supported target-check scope; examples assume those facts.",
			Missing:  "Unobserved, out-of-scope version, ambiguous, stale or incorrectly bound facts remain unproven_constraint_target evidence gaps (unverified coverage/review), never guessed absence. Prior related DDL and preceding ALTER actions withhold old facts; no PG post-state is simulated."}
	case "ddl.pg.alter.validate_constraint.advisory", "ddl.pg.alter.drop_constraint.advisory":
		return &MetadataNotes{Kinds: kinds,
			Required: "Optional proven PG17 catalog state and observed version suppress already-validated VALIDATE or absent guarded DROP no-op notices.",
			Missing:  "Keep conditional advice without claiming a scan/removal occurred; the independent target rules own missing-fact gaps."}
	}
	if ruleID == "ddl.table.row_size.max_bytes.require" {
		return &MetadataNotes{
			Kinds:    kinds,
			Required: "Live instance facts are available, so undeclared charset and row_format inputs resolve to the observed instance defaults.",
			Missing:  "Live instance facts are unavailable; the estimate runs on declared inputs only, and any required field left unresolved reports an evidence gap.",
		}
	}
	if strings.HasPrefix(ruleID, "ddl.view.") {
		return &MetadataNotes{
			Kinds:    kinds,
			Required: "Ordered relation and view facts support checks of target, source and inherited attributes where applicable.",
			Missing:  "Unprovided facts remain evidence gaps when the enabled check needs them; declared facts are still assessed offline.",
		}
	}
	return &MetadataNotes{
		Kinds:    kinds,
		Required: "Live metadata is available, so this explanation can use schema or instance facts for a more accurate judgment.",
		Missing:  "Live metadata is unavailable, so this explanation may be less accurate or cover less context than an online audit.",
	}
}

func metadataKindsForRule(ruleID string) ([]MetadataKind, bool) {
	if ruleID == "ddl.routine.target.valid" {
		return []MetadataKind{MetadataKindSchema, MetadataKindInstance}, true
	}
	switch ruleID {
	case "ddl.pg.create_view.options.valid":
		return []MetadataKind{MetadataKindInstance}, true
	case "ddl.pg.alter.constraint_columns.valid", "ddl.pg.create_view.output.valid", "ddl.pg.create_table.like.target.valid", "ddl.pg.create_table.on_commit.valid", "ddl.pg.create_table.columns.valid", "ddl.pg.create_table.inherits.valid", "ddl.pg.create_table.foreign_key.valid", "ddl.pg.create_table.expression.valid", "ddl.pg.create_table.keys.valid", "ddl.pg.create_table.storage.valid", "ddl.pg.create_table.of.valid",
		"ddl.pg.refresh_materialized_view.target.valid", "ddl.pg.drop_view.target.valid", "ddl.pg.alter_view.namespace.valid",
		"ddl.pg.alter_view.owner.valid", "ddl.pg.alter_view.options.valid", "ddl.pg.alter_view.column_default.valid",
		"ddl.pg.alter_materialized_view.options.valid", "ddl.pg.alter_materialized_view.extension.valid",
		"ddl.pg.alter_publication.target.valid", "ddl.pg.alter_subscription.target.valid",
		"ddl.pg.alter_foreign_table.target.valid", "ddl.pg.alter_foreign_data_wrapper.target.valid",
		"ddl.pg.sequence.declaration.valid",
		"ddl.pg.alter_policy.target.valid",
		"ddl.pg.alter_operator.change.target.valid", "ddl.pg.alter_type.change.target.valid", "ddl.pg.type.base.valid", "ddl.pg.type.composite.valid",
		"ddl.pg.alter_type.enum.valid", "ddl.pg.create_type.range.valid", "ddl.pg.create_type.enum.valid",
		"ddl.pg.text_search.change.target.valid", "ddl.pg.create_language.handler.valid", "ddl.pg.grant.declaration.valid", "ddl.pg.create_schema.target.valid", "ddl.pg.alter_collation.refresh.target.valid", "ddl.pg.alter_collation.change.target.valid",
		"ddl.pg.create_index.include.valid", "ddl.pg.alter_index.options.valid", "ddl.pg.alter_index.attach_partition.valid", "ddl.pg.create_operator_family.target.valid",
		"ddl.pg.alter_operator_family.member.valid", "ddl.pg.alter_index.destination.valid", "ddl.pg.alter_operator_family.change.target.valid":
		return []MetadataKind{MetadataKindTargetTable, MetadataKindInstance}, true
	case "ddl.pg.alter.column.storage.valid", "ddl.pg.alter.column.compression.valid", "ddl.pg.alter.column.statistics.target.valid", "ddl.pg.alter.column.statistics.clamp.warn", "ddl.pg.alter.column.options.valid", "ddl.alter.drop_column.exists.require", "ddl.pg.alter.drop_column.advisory":
		return []MetadataKind{MetadataKindTargetTable, MetadataKindInstance}, true
	case "ddl.pg.alter.column.generation.transition.valid", "ddl.pg.alter.identity.sequence_options.valid", "ddl.pg.alter.identity.restart.warn", "ddl.pg.alter.identity.cycle.warn":
		return []MetadataKind{MetadataKindTargetTable, MetadataKindInstance}, true
	case "ddl.pg.alter.validate_constraint.target.require", "ddl.pg.alter.constraint_deferrability.target.require", "ddl.pg.alter.drop_constraint.target.require",
		"ddl.pg.alter.validate_constraint.advisory", "ddl.pg.alter.drop_constraint.advisory":
		return []MetadataKind{MetadataKindTargetTable, MetadataKindInstance}, true
	case "ddl.view.target.valid", "ddl.view.source.valid", "ddl.view.security.definer.warn",
		"ddl.view.check_option.valid":
		return []MetadataKind{MetadataKindTargetTable}, true
	case "dml.table.denylist.forbid", "ddl.table.denylist.forbid":
		return []MetadataKind{MetadataKindSchema}, true
	case "ddl.database.create.exists.forbid", "ddl.database.alter.exists.require",
		"ddl.database.drop.exists.require":
		// T17 schema-existence premises consume the ordered schema binding
		// (SCHEMATA snapshot), not a target-table one.
		return []MetadataKind{MetadataKindSchema}, true
	case "ddl.database.option.encryption.applicability":
		// The applicability answer depends on the resolved instance version
		// (8.0.16+ lands, below refuses, TiDB ignores) — an instance-fact
		// dependency, not a schema snapshot one.
		return []MetadataKind{MetadataKindInstance}, true
	case "ddl.table.drop.adaptive_hash.warn", "ddl.table.truncate.adaptive_hash.warn",
		"ddl.table.row_size.max_bytes.require":
		// The row-size estimate consults optional instance defaults for
		// undeclared charset/row_format inputs; it never requires a target
		// table snapshot, so instance facts — not TargetTable — are its
		// metadata dependency. The flag marks a supplemental source, not a
		// connectivity requirement: declared-complete statements evaluate
		// identically offline.
		return []MetadataKind{MetadataKindInstance}, true
	case "ddl.table.drop.rows.max_count", "ddl.table.truncate.rows.max_count":
		return []MetadataKind{MetadataKindTargetTable}, true
	}
	if metadataAwareRuleIDs[ruleID] {
		return []MetadataKind{MetadataKindTargetTable}, true
	}
	return nil, false
}

func humanize(ruleID string, verb string) string {
	replacer := strings.NewReplacer(".", " ", "_", " ")
	text := replacer.Replace(ruleID)
	text = strings.ReplaceAll(text, "ddl ", "DDL ")
	text = strings.ReplaceAll(text, "dml ", "DML ")
	return strings.TrimSpace(fmt.Sprintf("%s %s", verb, text))
}

// FormatYAMLScalar renders one policy param value as a YAML scalar.
func FormatYAMLScalar(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []string:
		quoted := make([]string, 0, len(typed))
		for _, item := range typed {
			quoted = append(quoted, fmt.Sprintf("%q", item))
		}
		return fmt.Sprintf("[%s]", strings.Join(quoted, ", "))
	default:
		return fmt.Sprintf("%v", typed)
	}
}

// dialectsForRule derives the dialect scope from the rule ID.
// PG-prefixed rules target PostgreSQL only.
// TiDB-prefixed or merge.tidb rules target TiDB only.
// merge.mysql rules target MySQL only.
// The MODIFY unknown-prior-state nullability advisory targets MySQL and TiDB.
// The DML table-existence requirement also targets MySQL and TiDB.
// All other rules (including most DML rules) apply to common dialect scope.
func dialectsForRule(ruleID string) []string {
	switch {
	case ruleID == "ddl.drop_index.foreign_key.warn":
		return []string{"mysql", "tidb"}
	case ruleID == "ddl.mysql.stored_object.target.valid":
		return []string{"mysql"}
	case ruleID == "ddl.routine.target.valid" || ruleID == "ddl.routine.security.definer.warn":
		return []string{"mysql"}
	case ruleID == "ddl.view.target.valid", ruleID == "ddl.view.source.valid",
		ruleID == "ddl.view.security.definer.warn":
		return []string{"mysql", "tidb"}
	case ruleID == "ddl.view.algorithm.warn", ruleID == "ddl.view.check_option.valid":
		return []string{"mysql"}
	case ruleID == "ddl.view.options.ignored.warn":
		return []string{"tidb"}
	case ruleID == "dml.table.exists.require":
		return []string{"mysql", "tidb"}
	case strings.HasPrefix(ruleID, "ddl.pg."):
		return []string{"postgresql"}
	case strings.HasPrefix(ruleID, "ddl.tidb."):
		return []string{"tidb"}
	case strings.Contains(ruleID, ".merge.mysql."):
		return []string{"mysql"}
	case strings.Contains(ruleID, ".merge.tidb."):
		return []string{"tidb"}
	case ruleID == unknownPriorNullabilityAdvisoryRuleID:
		return []string{"mysql", "tidb"}
	case isCheckLifecycleRule(ruleID):
		// CHECK lifecycle semantics are a MySQL-family surface; the shared
		// parser produces these normalized actions only there.
		return []string{"mysql", "tidb"}
	default:
		return []string{"common"}
	}
}

func isCheckLifecycleRule(ruleID string) bool {
	switch ruleID {
	case
		"ddl.alter.alter_check.exists.require",
		"ddl.alter.drop_check.exists.require",
		"ddl.alter.alter_check.version.require",
		"ddl.constraint.check.not_enforced.notice",
		"ddl.constraint.check.ignored.notice":
		return true
	default:
		return false
	}
}

// categoryForRule derives a stable grouping category from the rule ID.
func categoryForRule(ruleID string) string {
	if strings.HasPrefix(ruleID, "dml.") {
		return "dml_safety"
	}
	if strings.HasPrefix(ruleID, "ddl.pg.") {
		return categoryForPGRule(ruleID)
	}
	return categoryForDDLRule(ruleID)
}

// categoryForDDLRule maps non-PG DDL rule IDs to categories.
func categoryForDDLRule(ruleID string) string {
	if ruleID == "ddl.routine.target.valid" {
		return "procedure"
	}
	if ruleID == "ddl.mysql.stored_object.target.valid" {
		return "trigger"
	}
	rest := ruleID[len("ddl."):]

	// Compound verb_noun patterns where the second segment is a verb+noun.
	compoundPrefixes := []struct {
		prefix   string
		category string
	}{
		{"rename_table", "table"},
		{"create_index", "index"},
		{"drop_index", "index"},
		{"alter_database", "database"},
		{"create_procedure", "procedure"},
		{"drop_procedure", "procedure"},
		{"create_user", "user_management"},
		{"alter_user", "user_management"},
		{"drop_user", "user_management"},
		{"create_role", "user_management"},
		{"drop_role", "user_management"},
		{"grant", "privilege"},
		{"revoke", "privilege"},
		{"create_resource_group", "resource_management"},
		{"alter_resource_group", "resource_management"},
		{"drop_resource_group", "resource_management"},
		{"create_placement_policy", "resource_management"},
		{"alter_placement_policy", "resource_management"},
		{"drop_placement_policy", "resource_management"},
		{"create_sequence", "sequence"},
		{"alter_sequence", "sequence"},
		{"drop_sequence", "sequence"},
		{"tidb.alter_table", "alter_table"},
	}
	for _, cp := range compoundPrefixes {
		if strings.HasPrefix(rest, cp.prefix) {
			return cp.category
		}
	}

	// Standard pattern: second segment is the category.
	segment := rest
	if idx := strings.Index(rest, "."); idx >= 0 {
		segment = rest[:idx]
	}
	if segment == "alter" {
		return "alter_table"
	}
	return segment
}

// categoryForPGRule maps PG DDL rule IDs to categories.
func categoryForPGRule(ruleID string) string {
	rest := ruleID[len("ddl.pg."):]

	// PG alter rules: most map to alter_table, except RLS and large_object.
	if strings.HasPrefix(rest, "alter.") {
		switch {
		case strings.Contains(rest, "enable_rls"),
			strings.Contains(rest, "disable_rls"),
			strings.Contains(rest, "force_rls"),
			strings.Contains(rest, "no_force_rls"):
			return "rls"
		case strings.Contains(rest, "large_object"):
			return "large_object"
		default:
			return "alter_table"
		}
	}

	// Map PG verb_object prefixes to categories.
	pgPrefixes := []struct {
		prefix   string
		category string
	}{
		{"create_index", "index"},
		{"drop_index", "index"},
		{"alter_index", "index"},
		{"create_table", "table"},
		{"table", "table"},
		{"column", "column"},
		{"drop_schema", "schema"},
		{"create_schema", "schema"},
		{"alter_schema", "schema"},
		{"sequence.", "sequence"},
		{"create_sequence", "sequence"},
		{"alter_sequence", "sequence"},
		{"drop_sequence", "sequence"},
		{"drop_materialized_view", "materialized_view"},
		{"refresh_materialized_view", "materialized_view"},
		{"alter_materialized_view", "materialized_view"},
		{"type.", "type"},
		{"create_type", "type"},
		{"alter_type", "type"},
		{"drop_type", "type"},
		{"create_domain", "domain"},
		{"alter_domain", "domain"},
		{"drop_domain", "domain"},
		{"grant", "privilege"},
		{"revoke", "privilege"},
		{"create_language", "language"},
		{"create_extension", "extension"},
		{"alter_extension", "extension"},
		{"drop_extension", "extension"},
		{"create_policy", "rls"},
		{"alter_policy", "rls"},
		{"drop_policy", "rls"},
		{"create_trigger", "trigger"},
		{"create_constraint_trigger", "trigger"},
		{"drop_trigger", "trigger"},
		{"create_function", "function"},
		{"create_or_replace_function", "function"},
		{"drop_function", "function"},
		{"alter_function", "function"},
		{"create_procedure", "procedure"},
		{"drop_procedure", "procedure"},
		{"alter_procedure", "procedure"},
		{"create_or_replace_view", "view"},
		{"create_temp_view", "view"},
		{"create_view", "view"},
		{"alter_view", "view"},
		{"drop_view", "view"},
		{"create_publication", "publication"},
		{"alter_publication", "publication"},
		{"drop_publication", "publication"},
		{"create_subscription", "subscription"},
		{"alter_subscription", "subscription"},
		{"drop_subscription", "subscription"},
		{"create_foreign_table", "foreign_table"},
		{"alter_foreign_table", "foreign_table"},
		{"drop_foreign_table", "foreign_table"},
		{"create_foreign_server", "foreign_server"},
		{"alter_foreign_server", "foreign_server"},
		{"drop_foreign_server", "foreign_server"},
		{"create_user_mapping", "user_mapping"},
		{"alter_user_mapping", "user_mapping"},
		{"drop_user_mapping", "user_mapping"},
		{"create_foreign_data_wrapper", "foreign_data_wrapper"},
		{"alter_foreign_data_wrapper", "foreign_data_wrapper"},
		{"drop_foreign_data_wrapper", "foreign_data_wrapper"},
		{"comment_on", "annotation"},
		{"security_label", "annotation"},
		{"create_event_trigger", "event_trigger"},
		{"alter_event_trigger", "event_trigger"},
		{"drop_event_trigger", "event_trigger"},
		{"create_rule", "rewrite_rule"},
		{"alter_rule", "rewrite_rule"},
		{"drop_rule", "rewrite_rule"},
		{"create_collation", "collation"},
		{"alter_collation", "collation"},
		{"drop_collation", "collation"},
		{"create_statistics", "statistics"},
		{"alter_statistics", "statistics"},
		{"drop_statistics", "statistics"},
		{"create_aggregate", "aggregate"},
		{"alter_aggregate", "aggregate"},
		{"drop_aggregate", "aggregate"},
		{"create_operator_family", "operator_family"},
		{"alter_operator_family", "operator_family"},
		{"drop_operator_family", "operator_family"},
		{"create_operator_class", "operator_class"},
		{"alter_operator_class", "operator_class"},
		{"drop_operator_class", "operator_class"},
		{"create_operator", "operator"},
		{"alter_operator", "operator"},
		{"drop_operator", "operator"},
		{"create_conversion", "conversion"},
		{"alter_conversion", "conversion"},
		{"drop_conversion", "conversion"},
		{"text_search.", "text_search"},
		{"create_text_search_configuration", "text_search"},
		{"alter_text_search_configuration", "text_search"},
		{"drop_text_search_configuration", "text_search"},
		{"create_text_search_dictionary", "text_search"},
		{"alter_text_search_dictionary", "text_search"},
		{"drop_text_search_dictionary", "text_search"},
		{"create_text_search_parser", "text_search"},
		{"alter_text_search_parser", "text_search"},
		{"drop_text_search_parser", "text_search"},
		{"create_text_search_template", "text_search"},
		{"alter_text_search_template", "text_search"},
		{"drop_text_search_template", "text_search"},
		{"create_transform", "transform"},
		{"drop_transform", "transform"},
		{"create_access_method", "access_method"},
		{"drop_access_method", "access_method"},
		{"alter_large_object", "large_object"},
	}
	for _, p := range pgPrefixes {
		if strings.HasPrefix(rest, p.prefix) {
			return p.category
		}
	}
	return "other"
}

// tagsForRule derives searchable tags from the rule ID and its derived metadata.
func tagsForRule(ruleID string) []string {
	tags := make([]string, 0, 4)

	// Add kind tag.
	if strings.HasPrefix(ruleID, "ddl.") {
		tags = append(tags, "ddl")
	} else if strings.HasPrefix(ruleID, "dml.") {
		tags = append(tags, "dml")
	}

	// Add dialect tag.
	dialects := dialectsForRule(ruleID)
	tags = append(tags, dialects...)

	// Add category tag.
	if cat := categoryForRule(ruleID); cat != "" {
		tags = append(tags, cat)
	}

	// Add level-pattern tag from rule ID suffix.
	switch {
	case strings.HasSuffix(ruleID, ".forbid"):
		tags = append(tags, "forbid")
	case strings.HasSuffix(ruleID, ".require"):
		tags = append(tags, "require")
	case strings.HasSuffix(ruleID, ".warn"):
		tags = append(tags, "warn")
	case strings.HasSuffix(ruleID, ".notice"):
		tags = append(tags, "notice")
	case strings.HasSuffix(ruleID, ".advisory"):
		tags = append(tags, "advisory")
	}

	return tags
}

// sourceForRule returns the provenance of catalog metadata.
// Entries derive from shipped Default Policy or catalog-only opt-in rules.
func sourceForRule() string {
	return "policy"
}

const unknownPriorNullabilityAdvisoryRuleID = "ddl.alter.modify_column.explicit_nullability_change.unknown_prior_state.advisory"

var metadataAwareRuleIDs = map[string]bool{
	"ddl.routine.target.valid":                             true,
	"ddl.mysql.stored_object.target.valid":                 true,
	"ddl.drop_index.foreign_key.warn":                      true,
	"ddl.pg.alter.move_all_tablespace.target.valid":        true,
	"ddl.pg.alter.owner.target.valid":                      true,
	"ddl.pg.alter.rename_table.target.valid":               true,
	"ddl.pg.alter.set_schema.target.valid":                 true,
	"ddl.pg.alter.add_foreign_key.targets.valid":           true,
	"ddl.pg.alter.constraint_using_index.valid":            true,
	"ddl.pg.alter.add_check.name.available":                true,
	"ddl.pg.alter.column_type.destination.valid":           true,
	"ddl.pg.alter.column_type.source.valid":                true,
	"ddl.pg.alter.persistence.transition.valid":            true,
	"ddl.pg.alter.access_method.target.valid":              true,
	"ddl.pg.alter.tablespace.target.valid":                 true,
	"ddl.pg.alter.reloptions.valid":                        true,
	"ddl.pg.alter.replica_identity_using_index.valid":      true,
	"ddl.pg.alter.column.storage.valid":                    true,
	"ddl.pg.alter.column.compression.valid":                true,
	"ddl.pg.alter.column.statistics.target.valid":          true,
	"ddl.pg.alter.column.statistics.clamp.warn":            true,
	"ddl.pg.alter.column.options.valid":                    true,
	"ddl.pg.alter.drop_column.advisory":                    true,
	"ddl.pg.alter.column.generation.transition.valid":      true,
	"ddl.pg.alter.identity.sequence_options.valid":         true,
	"ddl.pg.alter.identity.restart.warn":                   true,
	"ddl.pg.alter.identity.cycle.warn":                     true,
	"ddl.pg.alter.validate_constraint.target.require":      true,
	"ddl.pg.alter.constraint_deferrability.target.require": true,
	"ddl.pg.alter.drop_constraint.target.require":          true,
	"ddl.pg.alter.validate_constraint.advisory":            true,
	"ddl.pg.alter.drop_constraint.advisory":                true,
	"ddl.view.target.valid":                                true,
	"ddl.view.source.valid":                                true,
	"ddl.view.security.definer.warn":                       true,
	"ddl.view.check_option.valid":                          true,
	"ddl.alter.change_column.compatibility.require":        true,
	"ddl.alter.change_column.exists.require":               true,
	"ddl.alter.change_column.target.exists.forbid":         true,
	"ddl.alter.drop_column.exists.require":                 true,
	"ddl.alter.drop_index.exists.require":                  true,
	"ddl.alter.drop_foreign_key.exists.require":            true,
	"ddl.alter.drop_primary_key.exists.require":            true,
	"ddl.alter.modify_column.compatibility.require":        true,
	"ddl.alter.modify_column.exists.require":               true,
	unknownPriorNullabilityAdvisoryRuleID:                  true,
	"ddl.alter.rename_column.exists.require":               true,
	"ddl.alter.rename_column.target.exists.forbid":         true,
	"ddl.alter.rename_column.version.require":              true,
	"ddl.alter.rename_index.exists.require":                true,
	"ddl.alter.alter_index.exists.require":                 true,
	// T12 CHECK lifecycle rules read the target table's constraint members
	// and the instance's CHECK arm facts.
	"ddl.alter.alter_check.exists.require":     true,
	"ddl.alter.drop_check.exists.require":      true,
	"ddl.alter.alter_check.version.require":    true,
	"ddl.constraint.check.not_enforced.notice": true,
	"ddl.constraint.check.ignored.notice":      true,
	"ddl.index.advanced.compatibility.require": true,
	"ddl.table.drop.adaptive_hash.warn":        true,
	"ddl.table.drop.exists.require":            true,
	"ddl.table.drop.rows.max_count":            true,
	// T23-R8: the standalone TiDB physical-layout statements read the
	// bound target snapshot for existence and member/type adjudication.
	"ddl.table.split_region.exists.require": true,
	"ddl.table.compact.exists.require":      true,
	"ddl.tidb.table.split_region.valid":     true,
	"ddl.tidb.table.compact.valid":          true,
	"ddl.table.exists.alter.require":        true,
	"ddl.table.exists.create.forbid":        true,
	"ddl.table.row_size.max_bytes.require":  true,
	"ddl.table.truncate.adaptive_hash.warn": true,
	"ddl.table.truncate.exists.require":     true,
	"ddl.table.truncate.inbound_fk":         true,
	// T17 schema-lifecycle rules read the ordered schema binding
	// (Metadata.SchemaTarget). The name-keyed protection rules
	// (denylist, system.modify) evaluate identically offline, so they
	// stay out of this surface.
	"ddl.database.create.exists.forbid": true,
	"ddl.database.alter.exists.require": true,
	"ddl.database.drop.exists.require":  true,
	// The ENCRYPTION applicability answer reads the resolved instance
	// version (Metadata.Version), not the ordered schema binding.
	"ddl.database.option.encryption.applicability": true,
	"ddl.table.truncate.rows.max_count":            true,
	// T16 lifecycle rules consume the namespace-aware ordered bindings:
	// rename endpoint pairs, LIKE/CTAS sources, and TiDB's local-temp
	// refusal surface all read bound snapshot state.
	"ddl.table.rename.dest.exists.forbid":       true,
	"ddl.table.rename.source.exists.require":    true,
	"ddl.table.like.source.exists.require":      true,
	"ddl.table.create_as.source.exists.require": true,
	"ddl.tidb.local_temporary.alter":            true,
	"ddl.tidb.local_temporary.rename":           true,
	"ddl.tidb.local_temporary.like":             true,
	// T19 reads the ordered target's TTL configuration and prerequisites.
	"ddl.tidb.ttl.warn":                      true,
	"dml.table.denylist.forbid":              true,
	"dml.table.exists.require":               true,
	"ddl.table.denylist.forbid":              true,
	"ddl.alter.add_column.exists.forbid":     true,
	"ddl.alter.add_index.exists.forbid":      true,
	"ddl.index.key_length.max_bytes.require": true,
}
