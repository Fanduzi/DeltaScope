# DeltaScope v0.100.0 Release Notes

## Summary

v0.100.0 extends DeltaScope's PostgreSQL DDL audit coverage with 36 new PostgreSQL-only lifecycle and boundary rules across 6 object families. This milestone covers collation, extended statistics, aggregate/operator/conversion, operator family/class, text search object, and boundary closure (DROP TRANSFORM, DROP ACCESS METHOD, ALTER LARGE OBJECT) lifecycle forms. Two CREATE boundary forms (CREATE TRANSFORM, CREATE ACCESS METHOD) are intentionally deferred because their handler/function names are the object identity, making safe normalization incompatible with payload safety constraints.

## PostgreSQL DDL Long-Tail Coverage

v0.100.0 adds 36 PostgreSQL-only DDL lifecycle and boundary rules:

| Family | Rules | Example SQL |
|--------|:-----:|-------------|
| Collation | 3 | `CREATE COLLATION`, `ALTER COLLATION`, `DROP COLLATION` |
| Extended statistics | 3 | `CREATE STATISTICS`, `ALTER STATISTICS`, `DROP STATISTICS` |
| Aggregate/operator/conversion | 9 | `CREATE/ALTER/DROP AGGREGATE`, `CREATE/ALTER/DROP OPERATOR`, `CREATE/ALTER/DROP CONVERSION` |
| Operator family/class | 6 | `CREATE/ALTER/DROP OPERATOR FAMILY`, `CREATE/ALTER/DROP OPERATOR CLASS` |
| Text search objects | 12 | `CREATE/ALTER/DROP TEXT SEARCH CONFIGURATION/DICTIONARY/PARSER/TEMPLATE` |
| Boundary closure | 3 | `DROP TRANSFORM`, `DROP ACCESS METHOD`, `ALTER LARGE OBJECT ... OWNER TO` |

### Long-Tail Census

| Classification | Count |
|----------------|:-----:|
| finding_covered | 55 |
| normalized_silent | 0 |
| unsupported_boundary | 2 |
| parser_error | 0 |

The two remaining unsupported boundary cases are:

- **CREATE TRANSFORM**: The `FROM SQL WITH FUNCTION` / `TO SQL WITH FUNCTION` clauses embed handler function names as the object identity. Normalizing safely would require discarding the identity, which is incompatible with lifecycle audit.
- **CREATE ACCESS METHOD**: The `HANDLER handler_function` clause embeds the handler function name as the object identity. Same constraint as CREATE TRANSFORM.

### Payload Safety

Normalized findings avoid projecting handler, function, body, query, definition, or options payloads into output. Object identities use bounded tokens:

- Transform identity: `type@language` (e.g., `jsonb@plpython3u`) — no function names.
- Large object identity: OID integer (e.g., `12345`) — owner name stored as `owner` in options.
- Access method identity: name only (e.g., `hash`) — no handler function reference.

## Usage

```bash
# Offline audit — no database connection required
deltascope audit --dialect postgresql --sql "DROP COLLATION app_case_insensitive"

deltascope audit --dialect postgresql --sql "ALTER LARGE OBJECT 42 OWNER TO admin_user"

deltascope audit --dialect postgresql --sql "DROP TEXT SEARCH CONFIGURATION my_config"
```

All 36 rules are offline — they do not require a database connection.

## Non-Goals

- No full PostgreSQL DDL support claim. Selected long-tail object lifecycle families are covered; many forms remain deferred.
- No DCL/permission expansion beyond existing table-level grant/revoke support.
- No live DDL execution or migration outcome validation.
- No v1.0/stable API contract claim.
- DeltaScope does not execute migrations.

## Install

**macOS (recommended):**

```bash
brew tap fanduzi/deltascope
brew install --cask deltascope
```

**Generic installer:**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.100.0/install.sh | \
  DELTASCOPE_VERSION=v0.100.0 sh
```

## Upgrade

If you previously installed v0.90.0:

```bash
# Homebrew
brew upgrade --cask deltascope

# Generic installer (re-run with new version)
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.100.0/install.sh | \
  DELTASCOPE_VERSION=v0.100.0 sh
```

## Verification

```bash
deltascope --version
# Should output v0.100.0

# Offline audit — new collation lifecycle rule
deltascope audit --dialect postgresql --sql "DROP COLLATION app_ci" --format json
# Finding should include rule_id: "ddl.pg.drop_collation.warn"

# Offline audit — new text search lifecycle rule
deltascope audit --dialect postgresql --sql "CREATE TEXT SEARCH CONFIGURATION my_config" --format json
# Finding should include rule_id: "ddl.pg.create_text_search_configuration.notice"

# Offline audit — new boundary rule
deltascope audit --dialect postgresql --sql "DROP TRANSFORM FOR jsonb LANGUAGE plpython3u" --format json
# Finding should include rule_id: "ddl.pg.drop_transform.warn"
```
