# product-homepage

A reusable skill for generating or revising product homepages from repository facts.

## What it does
- Reads README, docs, and existing landing pages first
- Asks whether the user wants replication or innovation only when that choice is not already explicit
- Preserves validated product narrative constraints and approved sections when applicable
- If replicate or refresh mode has no current homepage or reference file, asks the user to provide a reference target or switch to Fully Custom Homepage
- Uses `frontend-design` for refresh and custom paths by default, with an explicit handoff and artifact return; if unavailable, continue with an inline design-summary path using the same locked constraints, visual dimensions, and variation rules
- If repo facts are insufficient and the user declines external research, limits copy to verified repo facts, omits unsupported positioning or market claims, and adds a short evidence-gap note

## Modes
- Replicate Current Homepage
- Keep Narrative, Refresh Visuals
- Fully Custom Homepage

## Downstream skills
- `frontend-design` is optional for replicate mode
- `frontend-design` is required by default for refresh-visuals mode, but if unavailable the skill should continue with an inline design-summary path using the same locked constraints, visual dimensions, and variation rules
- `frontend-design` is required by default for fully custom mode, but if unavailable the skill should continue with an inline design-summary path using the same locked constraints, visual dimensions, and variation rules
- external research is opt-in, not default

### `frontend-design` handoff
Pass these inputs into `frontend-design`:
- product facts grounded in repo sources
- chosen homepage mode and file strategy
- required narrative skeleton and preserved sections
- style direction mapped to hero composition, section rhythm, card system, and background treatment
- whether `frontend-design` will be invoked and which constraints are locked versus allowed to vary downstream
- explicit constraints such as bilingual needs, accessibility, framework, or target file
- reference page links or file paths when continuity matters

Expect these outputs back:
- a concrete visual direction for the homepage
- the specific visual dimensions that change versus the reference
- which dimensions preserve continuity versus introduce novelty
- how many visual dimensions changed in total
- why the narrative skeleton and preserved sections remain intact
- implementation-ready guidance for hero, section rhythm, card/panel system, CTA treatment, and background treatment
- any required UI tokens, interaction constraints, or content presentation rules

`product-homepage` should use that output to keep repo-grounded messaging and structure intact while applying the returned visual system during implementation or revision.

### External research
Use external research only when repo facts are insufficient or the user explicitly wants broader market or positioning claims.
Research should answer concrete questions such as:
- what comparable products or category patterns users will recognize
- what terminology or framing is common in the category
- what external proof points are safe to cite or paraphrase
- whether a broader market claim needs corroboration beyond repo materials

Before research output is used, convert it into homepage-safe claim boundaries and evidence constraints.

Research may add evidence such as:
- public competitor positioning examples
- category terminology patterns
- third-party benchmarks, reviews, or ecosystem references
- official public docs, release notes, or announcements outside the repo

Repository facts remain authoritative for product capabilities, scope, supported surfaces, and trust claims unless the user explicitly asks for broader external claims.

## DeltaScope example

For a repo like DeltaScope, the skill should:
- read `README.md`
- read `docs/landing/index.html`
- confirm product boundaries such as DDL/DML-only support
- preserve explicitly approved sections when requested
- decide whether to replicate or innovate before changing the page
