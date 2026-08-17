# Decision: Encode Empty Generated-Config Strings as YAML `""`

Date: 2026-08-17
Status: Accepted
Related milestone/version: issue #20
Related commits:
Related tests:
- `TestConfigInitYAMLLintsClean`
- `TestConfigShowDefaultMatchesInitAndLintsClean`
- `TestShippedExampleYAMLLintsClean`
- `TestHandWrittenFullSpecOverrideStillLintsClean`
- `TestGeneratedConfigPreservesDefaultAuditFindings`
- `TestConfigLintWarnsOnLevelOnlyOverride`
- `TestConfigLintStillRejectsInvalidValues` (YAML-null string param)
Related docs:
- `docs/reference/cli.md`
- `docs/reference/cli.zh-CN.md`
- `docs/reference/config.md`
- `docs/reference/config.zh-CN.md`
- `docs/recipe/inspect-rules-and-config.md`
- `docs/recipe/inspect-rules-and-config.zh-CN.md`

## Context

`deltascope config init` and `config show-default` render the default policy with `%v`. An empty
string param becomes a bare key such as `suffix:`, which YAML reads as null. `config lint` then
fails with `invalid type ... got <nil>, want string`. The shipped `configs/deltascope.example.yaml`
had the same encoding.

The documented CI path is `config init` followed by `config lint --file ... --strict`. That path
was red on a file the tool just wrote. Load/audit still ran because the loader is looser than lint.

## Decision

Generated and example policy YAML encode empty string params as `""`. Omission is not used: a
present-but-incomplete `params` map is a replacement hazard, and the template must keep every
default key visible.

`config lint` type checking is unchanged. Rule-level replacement warnings are unchanged. Default
policy evaluation is unchanged.

## Rationale

Quoting empty strings is the smallest change that makes generated YAML round-trip through lint.
Omitting empty optional params would pass the type check but fire replacement-hazard warnings and
hide the param from the template. Accepting YAML null as a string would hide the encoding bug and
make lint lie about the on-disk type.

## Public Contract

- `deltascope config init > file && deltascope config lint --file file` exits 0 and prints
  `Config OK`.
- `config show-default` is byte-identical to `config init` and also lints clean.
- `configs/deltascope.example.yaml` lints clean. Empty `prefix` / `suffix` values are `""`.
- A hand-written full-spec override still lints clean.
- `level:`-only replacement warnings stay advisory by default and still exit 2 with `--strict`.
- Auditing the same SQL with no `--config` and with the generated file produces the same findings.

## Deferred / Out Of Scope

- Changing rule-level replacement semantics
- Changing default-policy audit evaluation
- Accepting YAML null as a string in `config lint`
- Reordering or regenerating the whole example file to match `config init` byte-for-byte
- Quoting empty strings in `rules explain` / catalog config snippets (`formatYAMLScalar`)
- Encoding slice params as quoted YAML flow lists; current `%v` lists already lint clean
- Metadata or MCP behavior

## Verification Evidence

The release check is `TestConfigInitYAMLLintsClean`: it writes `config init` to a temp file and
asserts `config lint` / `config lint --strict` print `Config OK` and exit 0. Companion tests lint
`config show-default` and `configs/deltascope.example.yaml`, keep the existing full-spec and
`level:`-only contracts, reject a hand-written YAML-null `suffix:`, and compare default vs
generated-config audit findings for a CREATE TABLE + DELETE pair.

## Consequences

Future default-policy renderers must not print empty strings with `%v`. Empty string params in
generated or example YAML must stay quoted. Adding a new empty-string default param requires the
same encoding in `config init` / `show-default` / the example file.

## Links

- Issue: https://github.com/Fanduzi/DeltaScope/issues/20
- Tests: `internal/interfaces/cli/config_init_test.go`, `internal/interfaces/cli/config_lint_test.go`
- Renderer: `internal/interfaces/cli/config_init.go`
- Example: `configs/deltascope.example.yaml`
