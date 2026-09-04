# Decision: Reject HTTP `connection_ref` as Field-Level `invalid_request`

Date: 2026-09-04
Status: Accepted
Related issue: [#76](https://github.com/Fanduzi/DeltaScope/issues/76)
Related commits:
Related tests:
- `TestHandlerAuditRejectsConnectionRefAsInvalidRequest`
- `TestHandlerAuditMalformedJSONWithConnectionRefRemainsInvalidJSON`
- `TestHandlerAuditReturnsLegacyConnectionRejection`
- `TestHandlerAuditRejectsInvalidJSON`
- `TestHandlerAuditMetadataAwareWithRegistryConnection`
Related docs:
- `docs/reference/http-api.md`
- `docs/reference/http-api.zh-CN.md`
- `internal/interfaces/http/README.md`

## Context

HTTP `POST /v1/audit` accepts metadata-aware requests through `connection_id`. MCP
`audit_sql` uses a different name, `connection_ref`. The HTTP decoder uses
`DisallowUnknownFields`, so a JSON body that includes MCP's `connection_ref` was
classified as opaque `400 invalid_json`. That hid the real mismatch: HTTP uses
`connection_id` and does not accept `connection_ref`.

The removed inline `connection` object already returns field-level
`400 invalid_request`. Truly malformed JSON already returns `invalid_json`.

## Decision

When `/v1/audit` receives otherwise valid JSON that includes `connection_ref`,
return `400 invalid_request` with a message that names both `connection_id` and
`connection_ref`. Do not treat `connection_ref` as an alias for `connection_id`.
Do not change the `connection_id` happy path. Keep the existing bare `connection`
`400 invalid_request`. Keep truly malformed JSON as `invalid_json`. Other unknown
fields remain `invalid_json`.

## Rationale

Opaque `invalid_json` is the wrong signal for a known cross-surface field name.
Agents copying MCP arguments onto HTTP cannot tell a typo from invalid syntax.
Reusing the existing field-level `invalid_request` code matches the removed
`connection` object and avoids a new error code. Accepting `connection_ref` as an
alias would collapse two public names into one fact.

## Public Contract

`POST /v1/audit`:

| Body | Status | Code |
|------|--------|------|
| Valid JSON including `connection_ref` | 400 | `invalid_request` |
| Valid JSON including the removed `connection` object | 400 | `invalid_request` |
| Valid JSON with `connection_id` (and no rejected fields) | unchanged | unchanged |
| Truly malformed JSON | 400 | `invalid_json` |
| Other unknown fields | 400 | `invalid_json` |

The `invalid_request` message for `connection_ref` names `connection_id` and
`connection_ref`. HTTP does not accept `connection_ref` as an alias.

## Deferred / Out Of Scope

- Accepting `connection_ref` as an HTTP alias for `connection_id`
- Query Access HTTP mapping beyond this field
- Advertising `invalid_request` on `/v1/capabilities` (already used for the
  removed `connection` object; this issue does not expand the capability list)
- Changing MCP `connection_ref` semantics

## Verification Evidence

HTTP adapter tests post `/v1/audit` with `connection_ref` alone and together with
`connection_id`, assert 400 `invalid_request` naming both fields, and keep
malformed JSON, bare `connection`, and the `connection_id` metadata-aware path
on their existing codes.

## Consequences

Future HTTP request-body names that collide with MCP or removed fields should
return a named field-level client error instead of opaque `invalid_json`. Do not
silently alias those names.

## Links

- Issue: https://github.com/Fanduzi/DeltaScope/issues/76
- Tests: `internal/interfaces/http/handler_test.go`
- Docs: `docs/reference/http-api.md`, `docs/reference/http-api.zh-CN.md`
