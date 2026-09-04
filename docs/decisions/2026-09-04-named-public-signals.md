# Decision: Name Split Public Signals; Do Not Collapse Them

Date: 2026-09-04
Status: Accepted
Related milestone/version: issues #65, #67, #68, #69, #72, #74
Related commits:
Related tests:
- `TestLookupReturnsDefaultDisabledImpactRules`
- `TestSearchFindsDefaultDisabledImpactRules`
- `TestCatalogIncludesShippedRulesOutsideDefaultPolicy`
- `TestDefaultPolicyDoesNotEnableImpactRules`
- `TestRegisterSkipsDefaultDisabledImpactRules`
- `TestDefaultAuditDoesNotEmitImpactRuleFindings`
- `TestEnabledImpactConfigEmitsFindingsFromImpactObject`
- `TestRulesListAndExplainDefaultDisabledImpactRules`
- `TestListAndDescribeDefaultDisabledImpactRules`
- `TestInspect_DefaultDisabledImpactRule`
- `TestInspect_AllowsDefaultDisabledImpactRules`
- `TestAuditCommandRejectsExplicitEmptySQLWithoutReadingStdin`
- `TestExplicitEmptySQLErrorsNameTheirCommands`
- `TestAuditHelpDocumentsExitTable`
- `TestAuditJSONNamesFailThresholdSeparatelyFromVerdict`
- `TestRenderJSONResultNamesFailOnTriggeredBesideVerdict`
- `TestPublicResultJSONOmitsFailOnTriggered`
- `TestHandlerAuditReturnsJSONResult`
- `TestAuditSQLCallTextIncludesFindingSummary`
- `TestQueryAccessAnalyzeRejectsExplicitEmptySQLWithoutReadingStdin`
- `TestQueryAccessAnalyzeHelpShowsConnectionFlags`
- `TestDefaultPolicySuppressesForeignKeyNaming`
- `TestInspect_DefaultPolicyFKNamingIsSuppressedNotMissing`
- `TestInspect_FKNamingLoadsWhenForbidDisabled`
- `TestConfigStatus_FKForbidSuppressedNamingText`
- `TestConfigStatus_FKForbidSuppressedNamingJSON`
- `TestMarkdownRenderNamesFKForbidSkipReason``
Related docs:
- `CONTEXT.md`
- `docs/reference/audit-capability-matrix.md`
- `docs/decisions/2026-08-17-cli-explicit-empty-sql-input-source.md`
- `docs/decisions/2026-08-17-cli-user-input-exit-mapping.md`
- `docs/decisions/2026-08-30-mcp-launcher-upgrade-safe-install.md`

## Context

Five issues reported the same class of operator/agent pain: two public signals
looked like one fact, or a surface stayed silent about a negative capability.

- #65: CLI `--fail-on notice` can exit 1 while JSON `verdict` is `pass`.
- #68: empty `--sql` uses the same error sentence on `audit` (exit 2) and
  `query-access analyze` (exit 3).
- #69: npm `engines` and docs require Node 24+, but the launcher still starts
  on Node 20 with `EBADENGINE`.
- #72: MCP does not expose Query Access (an existing non-goal), and
  `get_capabilities` does not say so.
- #74: `dml.impact.*` rules are registered in code and described as covered,
  but they are absent from the Rule Catalog and Default Policy.
- #67: `rules list`, audit `rule_summary.loaded`, and CLI examples collapsed
  Catalog, Default Policy, Loaded, and FK-forbid suppressions into one count.

Existing accepted records already split some of these facts: Verdict is
`pass` / `review` / `reject` only; Query Access keeps its own exit table;
the npm launcher requires Node 24+. The new pain was naming and discovery,
not a mandate to collapse those splits.

## Decision

Different facts stay different. Public words must not refer to two facts.
A surface must state a negative capability instead of omitting it.

Per issue:

- **#65:** Fail Threshold does not change Verdict. CLI JSON adds a sibling
  field `fail_on_triggered`. SDK, HTTP, and MCP Result do not gain this field.
- **#68:** Keep `audit` empty `--sql` at exit 2 and Query Access empty `--sql`
  at exit 3. Change the error text so the two commands are distinguishable.
  Document the exit tables on `--help`.
- **#69:** Keep Node.js 24+ as required. Below 24, the launcher fails closed
  with a clear reason. Do not lower `engines`.
- **#72:** Do not add a Query Access MCP tool. `get_capabilities` includes
  `query_access: { available: false, surfaces: ["cli", "http"] }`.
- **#74:** Put `dml.impact.estimate`, `dml.impact.rows.max_count`, and
  `dml.impact.ratio.max_percent` in the Rule Catalog with Default Policy
  disabled. Do not enable them in Default Policy. Align capability-matrix
  and reference docs with opt-in status. The statement-level impact object
  remains the default audit payload; it is not a finding.
- **#67:** Name Rule Catalog, Default Policy, Loaded, and suppressions as
  different facts. Do not freeze a Loaded number that will drift. Keep
  FK-forbid naming rules in Catalog and Default Policy; `config status`
  reports reason `fk_forbid` so they read as suppressed, not missing.

The Rule Catalog may list rules that Default Policy does not enable.
Catalog generation must not equal Default Policy keys.
Do not treat Catalog count as Loaded count.

## Rationale

Collapsing the signals would change shipped meaning. Notices do not feed
Verdict; raising Verdict to `review` when Fail Threshold trips would make
Verdict a CI overlay. Unifying empty-SQL exits would occupy Query Access
exit 2, which is already indeterminate admission. Lowering Node engines
would reopen the launcher runtime decision while Node 20 today only works
because the requirement is not enforced.

Silence is the remaining defect. Agents read JSON, `get_capabilities`, and
`rules list`. They do not read `--help` as a primary source. Additive,
named fields and catalog rows state the split without redefining Verdict,
exit classes, or Default Policy.

## Public Contract

- Verdict remains `pass` / `review` / `reject` from blockers and warnings.
  Notices do not change Verdict.
- Fail Threshold remains a CLI process overlay. It does not change Verdict.
- CLI JSON may include `fail_on_triggered` (boolean) beside the audit Result.
  SDK, HTTP, and MCP Result shapes do not include this field.
- `deltascope audit --sql ""` stays exit 2. `deltascope query-access analyze
  --sql ""` stays exit 3. The error texts are no longer identical.
- Official MCP launcher requires Node.js 24 or newer and fails closed below
  that version.
- MCP tool list remains `audit_sql`, `describe_rule`, `list_rules`,
  `get_capabilities`. Query Access stays unavailable on MCP and is declared
  on `get_capabilities`.
- `dml.impact.*` threshold/estimate rules are discoverable, default-disabled,
  and require caller config to emit findings. Default audits do not change
  Verdict because of these rules.
- Rule Catalog, Default Policy, Loaded, and suppressions remain different
  counts. `config status` reports `status.loaded` and, for FK naming rules
  under the shipped baseline, `suppression.reason=fk_forbid`.

## Deferred / Out Of Scope

- Raising Verdict when Fail Threshold trips (#65 option to set `review`)
- Unifying empty-SQL exit codes (#68)
- Lowering npm `engines` to Node 20 (#69)
- Adding a Query Access MCP tool (#72)
- Enabling `dml.impact.*` in Default Policy, or deleting those rules (#74)
- Enabling FK-forbid suppressed naming rules by default (#67)
- Issues #66, #70, #71, #73, #75, #76
- Adding `error` / `unsupported` Verdict values
- Changing HTTP, SDK, or MCP status mapping for Fail Threshold

## Verification Evidence

- #74: Rule Catalog `Lookup`/`Search`, `rules list`, `list_rules`, and
  `describe_rule` return `dml.impact.estimate`, `dml.impact.rows.max_count`,
  and `dml.impact.ratio.max_percent` with `default_enabled: false`. Default
  Policy does not include those keys. Default UPDATE/DELETE audits keep the
  statement impact object and emit no findings from those IDs. Enabling them
  in config still produces findings from that object. Capability matrix and
  CLI reference now call them opt-in.
- #68: CLI `Execute` tests keep `audit --sql ""` at exit 2 and
  `query-access analyze --sql ""` at exit 3 without reading stdin. The
  stderr texts are distinct: `audit: SQL input must not be empty` versus
  `query-access: SQL input must not be empty`. `audit --help` and
  `query-access analyze --help` plus the CLI reference document both exit
  tables.
- #65: CLI JSON `Execute` tests keep notices-only `--fail-on notice
  --format json` at exit 1 with `verdict` `pass` and `fail_on_triggered`
  true; the same audit with default Fail Threshold (blocker) exits 0 with
  `verdict` `pass` and `fail_on_triggered` false. Warnings-only stays
  `review` and blockers stay `reject`. The JSON document seam test names
  `fail_on_triggered` beside Result without changing Verdict. SDK, HTTP,
  and MCP Result JSON omit the field. `audit --help` and the CLI
  reference state that Fail Threshold does not change Verdict.
- #69, #72: pending implementation. Expected evidence includes launcher
  failure below Node 24 and `get_capabilities` Query Access declaration
  tests.
- #67: `config status` on `ddl.constraint.foreign_key.name.*` keeps Default
  Policy `enabled`/`on` and reports `loaded: false` with
  `suppression.reason=fk_forbid` and `suppression.by=ddl.table.foreign_key.forbid`.
  Disabling the forbid rule Loads the naming rule. CLI reference no longer
  freezes `"loaded": 371`. Docs name Catalog, Default Policy, Loaded, and
  suppressions as different facts. Default Policy still does not enable
  `dml.impact.*`.

## Consequences

Do not reuse Verdict for process exit. Do not treat Rule Catalog count as
Loaded or Default Policy count. Future discovery payloads (`get_capabilities`,
`rules list`, `describe_rule`) must state negative or opt-in facts instead of
omitting them. Catalog construction must allow default-disabled shipped rules.

This record does not supersede
`2026-08-17-cli-explicit-empty-sql-input-source`,
`2026-08-17-cli-user-input-exit-mapping`, or
`2026-08-30-mcp-launcher-upgrade-safe-install`. It fills the deferred
Fail Threshold naming gap and the discovery gaps around Query Access and
impact rules.

## Links

- Issues: #65, #67, #68, #69, #72, #74
- Glossary: `CONTEXT.md`
- Prior records:
  - `docs/decisions/2026-08-17-cli-explicit-empty-sql-input-source.md`
  - `docs/decisions/2026-08-17-cli-user-input-exit-mapping.md`
  - `docs/decisions/2026-08-30-mcp-launcher-upgrade-safe-install.md`
