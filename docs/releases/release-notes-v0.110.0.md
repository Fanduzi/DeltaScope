# DeltaScope v0.110.0 Release Notes

## Summary

v0.110.0 promotes two previously deferred PostgreSQL DDL boundary forms — `CREATE TRANSFORM` and `CREATE ACCESS METHOD` — to supported lifecycle findings with bounded identity. The PostgreSQL DDL long-tail census now reaches 57/57 forms `finding_covered` with zero `unsupported_boundary` cases.

## What Changed

v0.110.0 is a focused promotion release. Two DDL forms that were intentionally deferred in v0.100.0 are now supported:

- **CREATE TRANSFORM** — fires `ddl.pg.create_transform.notice`. Object identity uses `type@language` (e.g., `jsonb@plpython3u`). The `FROM SQL WITH FUNCTION` and `TO SQL WITH FUNCTION` clause function names are not emitted.
- **CREATE ACCESS METHOD** — fires `ddl.pg.create_access_method.notice`. Object identity uses the access method name only (e.g., `heap2`). The `HANDLER` function name is not emitted.

### Long-Tail Census

| Classification | Count |
|----------------|:-----:|
| finding_covered | 57 |
| normalized_silent | 0 |
| unsupported_boundary | 0 |
| parser_error | 0 |

### Payload Safety

No handler, function, body, query, definition, or options payloads are emitted into findings. Object identities use bounded tokens:

- Transform identity: `type@language` (e.g., `jsonb@plpython3u`) — no function names.
- Access method identity: name only (e.g., `heap2`) — no handler function reference.

## Usage

```bash
# Offline audit — no database connection required
deltascope audit --dialect postgresql --sql "CREATE TRANSFORM FOR jsonb LANGUAGE plpython3u (FROM SQL WITH FUNCTION jsonb_to_plpython(jsonb), TO SQL WITH FUNCTION plpython_to_jsonb(internal))"

deltascope audit --dialect postgresql --sql "CREATE ACCESS METHOD heap2 TYPE TABLE HANDLER heap_tableam_handler"
```

Both rules are offline — they do not require a database connection.

## Non-Goals

- No new object families beyond CREATE TRANSFORM and CREATE ACCESS METHOD.
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
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.110.0/install.sh | \
  DELTASCOPE_VERSION=v0.110.0 sh
```

## Upgrade

If you previously installed v0.100.0:

```bash
# Homebrew
brew upgrade --cask deltascope

# Generic installer (re-run with new version)
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.110.0/install.sh | \
  DELTASCOPE_VERSION=v0.110.0 sh
```

## Verification

```bash
deltascope --version
# Should output v0.110.0

# Offline audit — CREATE TRANSFORM
deltascope audit --dialect postgresql --sql "CREATE TRANSFORM FOR jsonb LANGUAGE plpython3u (FROM SQL WITH FUNCTION jsonb_to_plpython(jsonb), TO SQL WITH FUNCTION plpython_to_jsonb(internal))" --format json
# Finding should include rule_id: "ddl.pg.create_transform.notice"

# Offline audit — CREATE ACCESS METHOD
deltascope audit --dialect postgresql --sql "CREATE ACCESS METHOD heap2 TYPE TABLE HANDLER heap_tableam_handler" --format json
# Finding should include rule_id: "ddl.pg.create_access_method.notice"
```
