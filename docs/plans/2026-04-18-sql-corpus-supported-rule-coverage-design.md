# SQL Corpus Supported-Rule Coverage Design

## Goal

Turn SQL corpus coverage from an informal “we have some examples” posture into an explicit, testable product contract:

- every **currently supported** `rule_id × dialect` surface must have at least one corpus case
- corpus fixtures must be able to express the configuration and metadata needed to trigger those rules
- CI must fail as soon as a shipped supported-rule surface loses corpus coverage

This is a **support-surface coverage** milestone, not a line-coverage milestone and not a “every policy key on every dialect” milestone.

## Context

DeltaScope already ships a large rule catalog across:

- MySQL
- TiDB
- PostgreSQL

The repository also already has SQL corpus fixtures, but before this work the corpus had three structural problems:

1. coverage was not defined against a stable contract
2. many metadata-aware rules were difficult or impossible to drive from YAML fixtures alone
3. release and smoke lanes could pass while corpus coverage silently drifted

That left a quality gap: the product could ship new or changed rule surfaces without a corresponding corpus proof.

## Problem Statement

“Corpus coverage” is only useful if the repository answers all three questions deterministically:

1. **What exactly counts as covered?**
2. **How does a fixture express the data needed to trigger that rule?**
3. **Where is the failure gate that blocks drift?**

Before this milestone, the answers were incomplete:

- fixture YAML could not express all of the config or metadata needed for broad rule coverage
- no single gate verified supported rule coverage across dialects
- PostgreSQL support-surface boundaries were mixed with theoretical policy presence

The result was weak confidence: fixture count could increase while meaningful rule coverage remained ambiguous.

## Design Decision

Define SQL corpus coverage using the narrow, operationally honest contract:

> **A rule surface is covered when the repository contains at least one SQL corpus case for each currently supported `rule_id × dialect` target.**

This contract deliberately excludes two broader interpretations:

### Not “every policy key on every dialect”

Some rules are:

- MySQL-family only
- PostgreSQL only
- temporarily deferred because the current extractor/rule contract does not yet support a stable corpus trigger path

Treating all policy keys as universally coverable would create false failures and push the corpus toward fictional support claims.

### Not Go code coverage

This work does not attempt to describe:

- line coverage
- branch coverage
- mutation coverage

It is specifically a **rule-surface corpus coverage** contract.

## Coverage Contract

The supported-rule coverage gate must:

1. enumerate shipped rule IDs from `policy.Default()`
2. map each rule ID to its expected dialect coverage targets
3. scan `testdata/sql-corpus/**.expected.yaml`
4. record `expect.findings.include`
5. fail if any expected `rule_id × dialect` target lacks at least one corpus file

This creates a single authoritative answer for:

- what is required
- what is covered
- what is missing

## Fixture Contract

The SQL corpus fixture model must support three inputs:

### Inline Config

Each corpus case may embed a top-level `config:` object. The harness materializes it into a temporary config file and passes that file through the public audit path.

This keeps corpus cases self-contained and avoids external fixture coupling.

### Metadata Fixtures

Each corpus case may embed top-level `metadata:` used to drive metadata-aware rule paths:

- `schema`
- `instance`
- `tables`
- `index_owners`

The harness must translate that YAML into a metadata provider implementation that is consumed through the same audit interfaces used by product code.

### Existing Semantic Expectations

The current YAML expectations remain valid:

- `parse_ok`
- `unsupported`
- `statement_kind`
- `operation`
- `findings`
- `facts`

This work widens the fixture input model without replacing the existing expectation model.

## PostgreSQL Boundary Decision

PostgreSQL needed a stricter honesty rule than MySQL/TiDB.

Some PostgreSQL policy keys are currently present in defaults but should **not** be treated as corpus-required support surfaces yet, because the current extractor/rule contract does not surface the facts needed for stable rule triggering.

Important examples:

- `ddl.alter.add_index.*@postgresql` semantic families that depend on `Alter.Index` payloads the PostgreSQL extractor does not currently build for `ADD CONSTRAINT`
- `ddl.table.primary_key.*@postgresql` semantic families that depend on `DDL.PrimaryKey` facts the PostgreSQL extractor does not currently populate consistently

The design decision is:

- do **not** force fictional PostgreSQL corpus coverage for those surfaces
- do record them as unsupported-by-current-surface, not as missing corpus work

This keeps the corpus contract aligned with the real product support surface.

## CI / Release Integration

The supported-rule coverage contract must not live only in a local test file. It must be attached to the repository verification flow:

- reusable make target for local and CI use
- inclusion in release test gates
- inclusion in release smoke validation

That gives the repository three benefits:

1. developers get a fast local gate
2. release automation blocks support-surface drift
3. the testing docs can point to one canonical command

## Non-Goals

This milestone does not include:

- adding new rule logic just to satisfy corpus coverage
- forcing every policy key to every dialect
- widening PostgreSQL extractor/spec support to close unrelated gaps
- line-coverage reporting
- corpus fuzzing
- SQL generation or random corpus synthesis

## Risks

### Risk: Over-claiming support

If the matrix says a dialect must cover a rule that the extractor cannot really drive, the corpus gate becomes dishonest.

Mitigation:

- explicitly classify MySQL-family-only, PostgreSQL-only, and deferred rule families
- keep the matrix close to current extractor/rule truth

### Risk: Fixture complexity drift

If fixture YAML gains too many ad hoc knobs, corpus cases become hard to reason about.

Mitigation:

- keep fixture inputs limited to config and metadata shapes already meaningful to audit execution
- reuse the public audit path rather than adding corpus-only semantics

### Risk: Silent CI blind spots

If the gate is only run ad hoc, it will not protect releases.

Mitigation:

- wire the coverage gate into reusable Make targets and release smoke/release test automation

## Success Criteria

This design is successful when all of the following are true:

- a single test verifies supported-rule corpus coverage across dialects
- corpus fixtures can express the config and metadata required by metadata-aware rules
- CI and release smoke both run the coverage gate
- testing docs describe the contract accurately
- coverage failures identify missing `rule_id@dialect` targets directly
- PostgreSQL unsupported support surfaces are treated as design deferrals, not fake missing corpus cases
