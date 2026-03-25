# Explainable Audit Results Design

## Goal

Make DeltaScope's audit results self-explaining across CLI, HTTP, and `pkg/deltascope` so users and AI agents can immediately understand why a rule fired, what risk it represents, and what the most direct remediation path is.

## Context

DeltaScope already has a strong rule engine, stable verdict semantics, a rule catalog foundation, and product surfaces across CLI, HTTP, and library usage. The current remaining gap is not basic audit coverage; it is result usability.

Today, findings are good enough to block or warn, but they still place too much burden on the caller to answer follow-up questions such as:

- why exactly did this rule trigger for this SQL?
- what practical risk is DeltaScope pointing at?
- what is the shortest path to make the SQL acceptable?
- which config entry controls this behavior?
- does this rule depend on metadata-aware context?

That gap affects both humans and machines:

- developers need to jump from output to docs or source code
- CI logs are less actionable than they could be
- AI agents receive a verdict and message, but not enough structured guidance to propose a precise next step

## Non-Goals

This milestone does not:

- add a large new set of DDL or DML rules
- change verdict semantics or `--fail-on` behavior
- add UI work
- add LLM-generated explanations
- add explain-plan-driven SQL analysis
- introduce team-level policy inheritance or presets

## Approaches Considered

### Approach A: CLI-Only Explanation Layer

Improve human-facing CLI output only, leaving HTTP and library result shapes unchanged.

Pros:

- smallest implementation scope
- fastest visible UX gain in the CLI

Cons:

- duplicates explanation logic in transport code
- HTTP and library clients still need custom logic
- weak foundation for AI-agent consumption

### Approach B: Shared Explanation Layer In The Audit Result Path

Attach structured explanation metadata to findings in a shared layer after rule evaluation, then let CLI, HTTP, and library consumers render or expose the same enriched result.

Pros:

- one explanation source of truth
- transport surfaces stay aligned
- strong foundation for future docs and agent tooling
- avoids repeated formatter-specific explanation logic

Cons:

- requires careful result-shape design
- slightly larger milestone than CLI-only output work

### Approach C: Rule Discovery Workspace

Go beyond enriched findings and also build a broad rule-discovery workspace around searching, browsing, and configuring rule behavior.

Pros:

- strongest long-term operator experience
- improves onboarding and self-service exploration

Cons:

- too much scope for one milestone
- risks turning a result-usability milestone into a broad governance/discovery project

## Recommendation

Choose Approach B.

DeltaScope already has the beginnings of an explanation-oriented catalog. The highest-leverage next step is to turn that into a stable, shared explanation layer that enriches actual audit findings instead of building one-off CLI prose. This keeps the milestone focused on making results more actionable without diluting effort into unrelated governance or discovery work.

## Design

### 1. Product Definition

This milestone turns DeltaScope from a tool that can say "this SQL is risky" into a tool that can also say:

- why it believes that
- what concrete risk category it is pointing at
- how to make progress quickly
- which rule and config surface control the behavior
- whether metadata availability influenced the outcome

The milestone should be named `Explainable Audit Results` rather than `Rule Explainability` because the product surface is the enriched audit result, not just static rule metadata.

### 2. Shared Explanation Model

Add a stable explanation structure associated with each finding.

Recommended shape:

- `summary`
- `why`
- `risk`
- `suggestion`
- `rule`
  - `id`
  - `default_level`
  - `metadata_aware`
- `config`
  - relevant params
  - whether level/enablement are configurable
- `links`
  - stable references to rules/config docs
- `metadata_note`
  - clarifies whether the explanation is limited or enhanced by metadata-aware context

This shape should be concise, deterministic, and fully local. It must not depend on external services or model inference.

### 3. Explanation Source Of Truth

Use the shipped rule catalog as the explanation metadata source of truth.

The catalog should grow from simple discovery metadata into explanation-capable metadata for shipped rules, including:

- stable summaries
- risk framing
- remediation hints
- config guidance
- metadata-awareness notes
- optional trigger/valid examples where they materially improve remediation guidance

Rule execution stays separate from rule description. The link between them remains `rule_id`.

### 4. Audit Flow Integration

Keep the current audit flow intact:

`parse -> extract -> enrich -> evaluate`

Add a final shared explanation-enrichment phase after evaluation:

`parse -> extract -> enrich -> evaluate -> explain`

Important constraints:

- explanation enrichment must never change verdicts
- explanation enrichment must never suppress findings
- missing explanation metadata must degrade gracefully to a minimal explanation instead of failing the audit

### 5. Result Surfaces

#### CLI

The CLI should gain a human-readable detailed explanation mode for findings while preserving compact default output for shell and CI use.

Recommended direction:

- keep the current compact default output stable
- add a detailed mode or explain-oriented flag for richer output
- make JSON mode expose the full structured explanation data

#### HTTP

`POST /v1/audit` should return enriched finding data in a stable structured form. Existing verdict and summary semantics should remain unchanged.

#### `pkg/deltascope`

The stable result object should expose the explanation structure so downstream callers, including AI agents, can consume it without scraping CLI text.

### 6. Metadata-Aware Transparency

For metadata-aware rules and enriched runs, explanations should stay honest about what information was or was not available.

Examples:

- if metadata made a rule more precise, say so
- if metadata was unavailable and explanation quality is limited, say so
- do not imply certainty that the engine did not actually have

This is critical to preserving trust in offline-first behavior.

### 7. Documentation Strategy

This milestone should also add a stable user-facing explanation of how to read DeltaScope results.

Recommended docs outcome:

- update reference docs to describe the explanation fields
- add a short recipe or reference section showing how to interpret enriched findings
- keep docs aligned across CLI, HTTP, and library examples

## Acceptance Criteria

This milestone is complete when:

1. DeltaScope has a stable explanation structure attached to findings.
2. The shipped rule catalog provides explanation metadata for the core shipped rule set.
3. CLI output can show explanation details without breaking the current compact default mode.
4. HTTP audit responses expose structured explanation data.
5. `pkg/deltascope` exposes the same explanation data for downstream callers.
6. Missing explanation metadata degrades gracefully and never changes verdict behavior.
7. Metadata-aware rules clearly communicate whether metadata availability affected explanation fidelity.
8. Product docs explain how to interpret enriched audit findings.
